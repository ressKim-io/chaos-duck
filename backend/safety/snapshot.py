import logging
from datetime import datetime, timezone
from typing import Any, Optional

logger = logging.getLogger(__name__)


class SnapshotManager:
    """Captures and stores state snapshots before chaos injection.

    Snapshots are keyed by experiment_id and contain the state
    of targeted resources at the time of capture.
    """

    def __init__(self):
        self._snapshots: dict[str, dict[str, Any]] = {}

    async def capture_k8s_snapshot(
        self,
        experiment_id: str,
        namespace: str,
        labels: Optional[dict[str, str]] = None,
    ) -> dict[str, Any]:
        """Capture K8s resource state before mutation."""
        snapshot = {
            "type": "k8s",
            "namespace": namespace,
            "labels": labels or {},
            "captured_at": datetime.now(timezone.utc).isoformat(),
            "resources": {
                "pods": [],
                "services": [],
                "deployments": [],
            },
        }
        # Actual K8s API calls will be added when kubernetes client is configured
        self._snapshots[experiment_id] = snapshot
        logger.info("K8s snapshot captured for experiment %s", experiment_id)
        return snapshot

    async def capture_aws_snapshot(
        self,
        experiment_id: str,
        resource_type: str,
        resource_id: str,
    ) -> dict[str, Any]:
        """Capture AWS resource state before mutation."""
        snapshot = {
            "type": "aws",
            "resource_type": resource_type,
            "resource_id": resource_id,
            "captured_at": datetime.now(timezone.utc).isoformat(),
            "state": {},
        }
        # Actual boto3 calls will be added when AWS is configured
        self._snapshots[experiment_id] = snapshot
        logger.info("AWS snapshot captured for experiment %s", experiment_id)
        return snapshot

    def get_snapshot(self, experiment_id: str) -> Optional[dict[str, Any]]:
        return self._snapshots.get(experiment_id)

    def delete_snapshot(self, experiment_id: str) -> None:
        self._snapshots.pop(experiment_id, None)

    def list_snapshots(self) -> dict[str, dict[str, Any]]:
        return dict(self._snapshots)


# Global singleton
snapshot_manager = SnapshotManager()
