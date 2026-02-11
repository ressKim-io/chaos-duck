import asyncio
import fnmatch
import functools
import logging

from models.experiment import ExperimentConfig
from safety.health_check import HealthCheckLoop
from safety.rollback import rollback_manager
from safety.snapshot import snapshot_manager

logger = logging.getLogger(__name__)


class EmergencyStopManager:
    """Manages emergency stop state using asyncio.Event."""

    def __init__(self):
        self._event = asyncio.Event()

    def trigger(self) -> None:
        """Trigger emergency stop."""
        logger.critical("EMERGENCY STOP TRIGGERED")
        self._event.set()

    def reset(self) -> None:
        """Reset emergency stop (allow new experiments)."""
        self._event.clear()
        logger.info("Emergency stop reset")

    def is_triggered(self) -> bool:
        return self._event.is_set()

    async def wait(self) -> None:
        """Block until emergency stop is triggered."""
        await self._event.wait()


# Global singleton
emergency_stop_manager = EmergencyStopManager()


def with_timeout(seconds: int = 30):
    """Decorator to enforce timeout on async functions.

    Max allowed timeout is 120 seconds.
    """
    clamped = min(max(seconds, 1), 120)

    def decorator(func):
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            try:
                return await asyncio.wait_for(func(*args, **kwargs), timeout=clamped)
            except TimeoutError:
                logger.error("Timeout after %ds in %s", clamped, func.__name__)
                raise TimeoutError(f"Operation {func.__name__} timed out after {clamped}s")

        return wrapper

    return decorator


def require_confirmation(namespace_pattern: str = "prod*"):
    """Decorator that requires confirmation for matching namespaces.

    In non-interactive contexts, the confirmation state is managed
    via the experiment's safety config.
    """

    def decorator(func):
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            config: ExperimentConfig | None = kwargs.get("config")
            if config and config.target_namespace:
                if fnmatch.fnmatch(config.target_namespace, namespace_pattern):
                    if not config.safety.require_confirmation:
                        raise PermissionError(
                            f"Namespace '{config.target_namespace}' matches "
                            f"pattern '{namespace_pattern}'. "
                            f"Set safety.require_confirmation=True to proceed."
                        )
            return await func(*args, **kwargs)

        return wrapper

    return decorator


def validate_blast_radius(affected_count: int, total_count: int, max_ratio: float = 0.3) -> bool:
    """Check that blast radius does not exceed the allowed ratio."""
    if total_count == 0:
        return True
    ratio = affected_count / total_count
    if ratio > max_ratio:
        logger.warning(
            "Blast radius %.1f%% exceeds max %.1f%%",
            ratio * 100,
            max_ratio * 100,
        )
        return False
    return True


class ExperimentContext:
    """Context manager for safe experiment execution.

    Automatically captures a snapshot before the experiment,
    runs health check probes during injection, and triggers
    rollback on exception.
    """

    def __init__(
        self,
        experiment_id: str,
        config: ExperimentConfig,
        probes: list | None = None,
    ):
        self.experiment_id = experiment_id
        self.config = config
        self.probes = probes or []
        self._health_loop: HealthCheckLoop | None = None

    async def __aenter__(self):
        if emergency_stop_manager.is_triggered():
            raise RuntimeError("Emergency stop is active. Cannot start experiment.")

        # Capture snapshot before mutation
        if self.config.target_namespace:
            await snapshot_manager.capture_k8s_snapshot(
                self.experiment_id,
                self.config.target_namespace,
                self.config.target_labels,
            )

        # Start health check loop if continuous probes are configured
        continuous_probes = [p for p in self.probes if hasattr(p, "mode") and p.mode == "continuous"]
        if continuous_probes:
            self._health_loop = HealthCheckLoop(
                experiment_id=self.experiment_id,
                probes=continuous_probes,
                interval=self.config.safety.health_check_interval,
                failure_threshold=self.config.safety.health_check_failure_threshold,
            )
            self._health_loop.start()

        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        # Stop health check loop
        if self._health_loop is not None:
            await self._health_loop.stop()

        if exc_type is not None:
            logger.error(
                "Experiment %s failed: %s. Triggering rollback.",
                self.experiment_id,
                exc_val,
            )
            await rollback_manager.rollback(self.experiment_id)
        return False

    def start_health_checks(self, probes: list) -> None:
        """Manually start health check loop with given probes."""
        if self._health_loop is not None:
            return
        self._health_loop = HealthCheckLoop(
            experiment_id=self.experiment_id,
            probes=probes,
            interval=self.config.safety.health_check_interval,
            failure_threshold=self.config.safety.health_check_failure_threshold,
        )
        self._health_loop.start()
