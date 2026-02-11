import logging
from collections import defaultdict
from collections.abc import Callable
from typing import Any

from observability.metrics import METRICS

logger = logging.getLogger(__name__)


class RollbackManager:
    """LIFO rollback manager for chaos experiments.

    Collects rollback functions and executes them in reverse order
    to restore system state after experiments or on emergency stop.
    """

    def __init__(self):
        # experiment_id -> list of (description, rollback_fn)
        self._stacks: dict[str, list[tuple[str, Callable]]] = defaultdict(list)

    def push(
        self,
        experiment_id: str,
        rollback_fn: Callable,
        description: str = "",
    ) -> None:
        """Push a rollback function onto the experiment's stack."""
        self._stacks[experiment_id].append((description, rollback_fn))
        logger.info(
            "Rollback pushed for %s: %s (stack size: %d)",
            experiment_id,
            description,
            len(self._stacks[experiment_id]),
        )

    async def rollback(self, experiment_id: str) -> list[dict[str, Any]]:
        """Execute all rollback functions for an experiment in LIFO order."""
        stack = self._stacks.pop(experiment_id, [])
        results = []
        for description, rollback_fn in reversed(stack):
            try:
                result = await rollback_fn()
                results.append(
                    {
                        "description": description,
                        "status": "success",
                        "result": result,
                    }
                )
                METRICS.record_rollback("success")
                logger.info("Rollback success: %s", description)
            except Exception as e:
                results.append(
                    {
                        "description": description,
                        "status": "failed",
                        "error": str(e),
                    }
                )
                METRICS.record_rollback("failed")
                logger.error("Rollback failed: %s - %s", description, e)
        return results

    async def rollback_all(self) -> dict[str, list[dict[str, Any]]]:
        """Rollback ALL active experiments (emergency stop)."""
        all_results = {}
        experiment_ids = list(self._stacks.keys())
        for experiment_id in experiment_ids:
            all_results[experiment_id] = await self.rollback(experiment_id)
        return all_results

    def get_stack_size(self, experiment_id: str) -> int:
        return len(self._stacks.get(experiment_id, []))

    def get_active_experiments(self) -> list[str]:
        return list(self._stacks.keys())


# Global singleton
rollback_manager = RollbackManager()
