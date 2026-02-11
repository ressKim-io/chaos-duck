import asyncio
import logging

from safety.rollback import rollback_manager

logger = logging.getLogger(__name__)


class HealthCheckLoop:
    """Background health check loop that monitors probes during experiments.

    Polls probes at a configurable interval and triggers automatic rollback
    when consecutive failures exceed the threshold.
    """

    def __init__(
        self,
        experiment_id: str,
        probes: list,
        interval: int = 10,
        failure_threshold: int = 3,
        on_failure: callable | None = None,
    ):
        self.experiment_id = experiment_id
        self.probes = probes
        self.interval = interval
        self.failure_threshold = failure_threshold
        self.on_failure = on_failure
        self._consecutive_failures = 0
        self._task: asyncio.Task | None = None
        self._stopped = asyncio.Event()
        self._results: list = []

    @property
    def results(self) -> list:
        return list(self._results)

    @property
    def is_running(self) -> bool:
        return self._task is not None and not self._task.done()

    def start(self) -> None:
        """Start the health check loop as a background task."""
        if self._task is not None:
            return
        self._stopped.clear()
        self._task = asyncio.create_task(self._run())
        logger.info(
            "Health check loop started for %s (interval=%ds, threshold=%d)",
            self.experiment_id,
            self.interval,
            self.failure_threshold,
        )

    async def stop(self) -> None:
        """Stop the health check loop."""
        if self._task is None:
            return
        self._stopped.set()
        try:
            await asyncio.wait_for(self._task, timeout=self.interval + 2)
        except (TimeoutError, asyncio.CancelledError):
            self._task.cancel()
        self._task = None
        logger.info("Health check loop stopped for %s", self.experiment_id)

    async def _run(self) -> None:
        """Main polling loop."""
        while not self._stopped.is_set():
            try:
                all_passed = await self._check_probes()
                if all_passed:
                    self._consecutive_failures = 0
                else:
                    self._consecutive_failures += 1
                    logger.warning(
                        "Health check failed for %s (%d/%d)",
                        self.experiment_id,
                        self._consecutive_failures,
                        self.failure_threshold,
                    )

                    if self._consecutive_failures >= self.failure_threshold:
                        logger.critical(
                            "Health check threshold reached for %s. Triggering rollback.",
                            self.experiment_id,
                        )
                        if self.on_failure:
                            await self.on_failure()
                        else:
                            await rollback_manager.rollback(self.experiment_id)
                        self._stopped.set()
                        return

                # Wait for interval or stop signal
                try:
                    await asyncio.wait_for(self._stopped.wait(), timeout=self.interval)
                    return  # Stopped
                except TimeoutError:
                    pass  # Continue polling

            except asyncio.CancelledError:
                return
            except Exception as e:
                logger.error("Health check loop error for %s: %s", self.experiment_id, e)
                self._consecutive_failures += 1

    async def _check_probes(self) -> bool:
        """Execute all probes and return True if all pass."""
        if not self.probes:
            return True

        all_passed = True
        for probe in self.probes:
            result = await probe.safe_execute()
            self._results.append(result)
            if not result.passed:
                all_passed = False
        return all_passed
