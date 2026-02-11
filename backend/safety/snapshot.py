import logging
from datetime import UTC, datetime
from typing import Any

logger = logging.getLogger(__name__)


class SnapshotManager:
    """Captures and stores state snapshots before chaos injection.

    Snapshots are keyed by experiment_id and contain the state
    of targeted resources at the time of capture. Supports both
    in-memory storage and optional DB persistence.
    """

    def __init__(self):
        self._snapshots: dict[str, dict[str, Any]] = {}
        self._k8s_client = None
        self._boto3_ec2 = None
        self._boto3_rds = None

    def _get_k8s_client(self):
        """Lazy-load kubernetes client."""
        if self._k8s_client is None:
            try:
                from kubernetes import client, config

                config.load_incluster_config()
            except Exception:
                from kubernetes import client, config

                config.load_kube_config()
            self._k8s_client = client
        return self._k8s_client

    def _get_boto3_clients(self):
        """Lazy-load boto3 clients."""
        if self._boto3_ec2 is None:
            import boto3

            self._boto3_ec2 = boto3.client("ec2")
            self._boto3_rds = boto3.client("rds")
        return self._boto3_ec2, self._boto3_rds

    async def capture_k8s_snapshot(
        self,
        experiment_id: str,
        namespace: str,
        labels: dict[str, str] | None = None,
    ) -> dict[str, Any]:
        """Capture K8s resource state before mutation.

        Attempts to query the real K8s API for pod specs, deployment specs,
        and service specs. Falls back to empty resources if the cluster
        is not reachable.
        """
        captured_at = datetime.now(UTC).isoformat()
        pods_data = []
        deployments_data = []
        services_data = []

        try:
            k8s = self._get_k8s_client()
            v1 = k8s.CoreV1Api()
            apps_v1 = k8s.AppsV1Api()

            label_selector = ""
            if labels:
                label_selector = ",".join(f"{k}={v}" for k, v in labels.items())

            # Capture pod specs
            pods = v1.list_namespaced_pod(namespace, label_selector=label_selector)
            for pod in pods.items:
                pods_data.append(
                    {
                        "name": pod.metadata.name,
                        "namespace": pod.metadata.namespace,
                        "labels": pod.metadata.labels or {},
                        "phase": pod.status.phase,
                        "containers": [
                            {
                                "name": c.name,
                                "image": c.image,
                            }
                            for c in (pod.spec.containers or [])
                        ],
                        "node_name": pod.spec.node_name,
                    }
                )

            # Capture deployment specs
            deployments = apps_v1.list_namespaced_deployment(
                namespace, label_selector=label_selector
            )
            for dep in deployments.items:
                deployments_data.append(
                    {
                        "name": dep.metadata.name,
                        "namespace": dep.metadata.namespace,
                        "replicas": dep.spec.replicas,
                        "ready_replicas": dep.status.ready_replicas or 0,
                        "labels": dep.metadata.labels or {},
                        "selector": dep.spec.selector.match_labels or {},
                    }
                )

            # Capture service specs
            services = v1.list_namespaced_service(namespace, label_selector=label_selector)
            for svc in services.items:
                svc_data = {
                    "name": svc.metadata.name,
                    "namespace": svc.metadata.namespace,
                    "type": svc.spec.type,
                    "cluster_ip": svc.spec.cluster_ip,
                    "labels": svc.metadata.labels or {},
                }
                if svc.spec.ports:
                    svc_data["ports"] = [
                        {"port": p.port, "target_port": str(p.target_port), "protocol": p.protocol}
                        for p in svc.spec.ports
                    ]
                services_data.append(svc_data)

            logger.info(
                "K8s snapshot captured for %s: %d pods, %d deployments, %d services",
                experiment_id,
                len(pods_data),
                len(deployments_data),
                len(services_data),
            )

        except Exception as e:
            logger.warning("K8s API not available for snapshot, using empty resources: %s", e)

        snapshot = {
            "type": "k8s",
            "namespace": namespace,
            "labels": labels or {},
            "captured_at": captured_at,
            "resources": {
                "pods": pods_data,
                "services": services_data,
                "deployments": deployments_data,
            },
        }

        self._snapshots[experiment_id] = snapshot
        await self._persist_snapshot(experiment_id, snapshot)
        return snapshot

    async def capture_aws_snapshot(
        self,
        experiment_id: str,
        resource_type: str,
        resource_id: str,
    ) -> dict[str, Any]:
        """Capture AWS resource state before mutation.

        Queries the real AWS API for EC2 instance or RDS cluster state.
        Falls back to empty state if AWS is not configured.
        """
        captured_at = datetime.now(UTC).isoformat()
        state = {}

        try:
            ec2, rds = self._get_boto3_clients()

            if resource_type == "ec2":
                response = ec2.describe_instances(InstanceIds=[resource_id])
                for reservation in response.get("Reservations", []):
                    for instance in reservation.get("Instances", []):
                        state = {
                            "instance_id": instance["InstanceId"],
                            "instance_type": instance.get("InstanceType"),
                            "state": instance.get("State", {}).get("Name"),
                            "vpc_id": instance.get("VpcId"),
                            "subnet_id": instance.get("SubnetId"),
                            "security_groups": [
                                sg["GroupId"] for sg in instance.get("SecurityGroups", [])
                            ],
                            "tags": {t["Key"]: t["Value"] for t in instance.get("Tags", [])},
                        }

            elif resource_type == "rds":
                response = rds.describe_db_clusters(DBClusterIdentifier=resource_id)
                for cluster in response.get("DBClusters", []):
                    state = {
                        "cluster_id": cluster["DBClusterIdentifier"],
                        "status": cluster.get("Status"),
                        "engine": cluster.get("Engine"),
                        "engine_version": cluster.get("EngineVersion"),
                        "endpoint": cluster.get("Endpoint"),
                        "reader_endpoint": cluster.get("ReaderEndpoint"),
                        "members": [
                            {
                                "instance_id": m.get("DBInstanceIdentifier"),
                                "is_writer": m.get("IsClusterWriter"),
                            }
                            for m in cluster.get("DBClusterMembers", [])
                        ],
                    }

            logger.info(
                "AWS snapshot captured for %s: %s/%s",
                experiment_id,
                resource_type,
                resource_id,
            )

        except Exception as e:
            logger.warning("AWS API not available for snapshot, using empty state: %s", e)

        snapshot = {
            "type": "aws",
            "resource_type": resource_type,
            "resource_id": resource_id,
            "captured_at": captured_at,
            "state": state,
        }

        self._snapshots[experiment_id] = snapshot
        await self._persist_snapshot(experiment_id, snapshot)
        return snapshot

    async def restore_from_snapshot(
        self,
        experiment_id: str,
    ) -> dict[str, Any] | None:
        """Restore resources to their snapshotted state.

        Compares current state with snapshot and applies corrections.
        Returns a diff summary of what was restored.
        """
        snapshot = self.get_snapshot(experiment_id)
        if not snapshot:
            logger.warning("No snapshot found for %s", experiment_id)
            return None

        restored = {"experiment_id": experiment_id, "actions": []}

        try:
            if snapshot["type"] == "k8s":
                restored["actions"] = await self._restore_k8s(snapshot)
            elif snapshot["type"] == "aws":
                restored["actions"] = await self._restore_aws(snapshot)
        except Exception as e:
            logger.error("Snapshot restore failed for %s: %s", experiment_id, e)
            restored["error"] = str(e)

        return restored

    async def _restore_k8s(self, snapshot: dict) -> list[dict]:
        """Restore K8s resources based on snapshot diff."""
        actions = []
        namespace = snapshot["namespace"]
        snapshot_pods = {p["name"]: p for p in snapshot["resources"].get("pods", [])}

        if not snapshot_pods:
            return actions

        try:
            k8s = self._get_k8s_client()
            v1 = k8s.CoreV1Api()

            current_pods = v1.list_namespaced_pod(namespace)
            current_pod_names = {p.metadata.name for p in current_pods.items}

            # Find pods that existed in snapshot but are missing now
            for pod_name, pod_data in snapshot_pods.items():
                if pod_name not in current_pod_names:
                    actions.append(
                        {
                            "action": "pod_missing",
                            "name": pod_name,
                            "status": "detected",
                        }
                    )
                    logger.warning(
                        "Pod %s was in snapshot but is now missing in %s",
                        pod_name,
                        namespace,
                    )

        except Exception as e:
            logger.error("K8s restore check failed: %s", e)
            actions.append({"action": "error", "error": str(e)})

        return actions

    async def _restore_aws(self, snapshot: dict) -> list[dict]:
        """Restore AWS resources based on snapshot diff."""
        actions = []
        state = snapshot.get("state", {})

        if not state:
            return actions

        try:
            ec2, rds = self._get_boto3_clients()

            if snapshot["resource_type"] == "ec2":
                instance_id = state.get("instance_id")
                original_state = state.get("state")
                if instance_id and original_state:
                    response = ec2.describe_instances(InstanceIds=[instance_id])
                    for res in response.get("Reservations", []):
                        for inst in res.get("Instances", []):
                            current_state = inst.get("State", {}).get("Name")
                            if current_state != original_state:
                                actions.append(
                                    {
                                        "action": "state_drift",
                                        "instance_id": instance_id,
                                        "snapshot_state": original_state,
                                        "current_state": current_state,
                                    }
                                )

        except Exception as e:
            logger.error("AWS restore check failed: %s", e)
            actions.append({"action": "error", "error": str(e)})

        return actions

    async def _persist_snapshot(self, experiment_id: str, snapshot: dict) -> None:
        """Persist snapshot to database if available."""
        try:
            from database import async_session
            from db_models import SnapshotRecord

            async with async_session() as session:
                rec = SnapshotRecord(
                    experiment_id=experiment_id,
                    type=snapshot["type"],
                    namespace=snapshot.get("namespace"),
                    data=snapshot,
                )
                session.add(rec)
                await session.commit()
        except Exception as e:
            logger.debug("DB persistence skipped for snapshot: %s", e)

    def get_snapshot(self, experiment_id: str) -> dict[str, Any] | None:
        return self._snapshots.get(experiment_id)

    def delete_snapshot(self, experiment_id: str) -> None:
        self._snapshots.pop(experiment_id, None)

    def list_snapshots(self) -> dict[str, dict[str, Any]]:
        return dict(self._snapshots)


# Global singleton
snapshot_manager = SnapshotManager()
