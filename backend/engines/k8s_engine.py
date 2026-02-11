import logging
from collections.abc import Callable
from typing import Any

from models.experiment import ExperimentConfig
from models.topology import (
    HealthStatus,
    InfraTopology,
    ResourceType,
    TopologyEdge,
    TopologyNode,
)
from safety.guardrails import emergency_stop_manager, validate_blast_radius

logger = logging.getLogger(__name__)


class K8sEngine:
    """Kubernetes chaos engine.

    All mutation methods return (result, rollback_fn) tuples.
    Supports dry_run mode and emergency stop checks.
    """

    def __init__(self):
        self._client = None

    def _get_client(self):
        """Lazy-load kubernetes client."""
        if self._client is None:
            try:
                from kubernetes import client, config

                config.load_incluster_config()
            except Exception:
                from kubernetes import client, config

                config.load_kube_config()
            self._client = client
        return self._client

    def _check_emergency_stop(self) -> None:
        if emergency_stop_manager.is_triggered():
            raise RuntimeError("Emergency stop is active.")

    async def pod_delete(
        self,
        namespace: str,
        label_selector: str,
        config: ExperimentConfig | None = None,
        dry_run: bool = False,
    ) -> tuple[dict[str, Any], Callable | None]:
        """Delete pods matching the label selector."""
        self._check_emergency_stop()

        k8s = self._get_client()
        v1 = k8s.CoreV1Api()

        pods = v1.list_namespaced_pod(namespace, label_selector=label_selector)
        pod_names = [p.metadata.name for p in pods.items]
        total_pods = len(v1.list_namespaced_pod(namespace).items)

        if not validate_blast_radius(
            len(pod_names),
            total_pods,
            config.safety.max_blast_radius if config else 0.3,
        ):
            raise ValueError(f"Blast radius exceeded: {len(pod_names)}/{total_pods} pods")

        if dry_run:
            return {"action": "pod_delete", "pods": pod_names, "dry_run": True}, None

        # Save pod specs for rollback
        saved_pods = []
        for pod in pods.items:
            saved_pods.append(pod)
            v1.delete_namespaced_pod(pod.metadata.name, namespace)

        logger.info("Deleted %d pods in %s", len(pod_names), namespace)

        async def rollback():
            """Recreate deleted pods."""
            for pod in saved_pods:
                pod.metadata.resource_version = None
                pod.status = None
                v1.create_namespaced_pod(namespace, pod)
            logger.info("Rollback: recreated %d pods in %s", len(saved_pods), namespace)
            return {"recreated": len(saved_pods)}

        return {"action": "pod_delete", "pods": pod_names}, rollback

    async def network_latency(
        self,
        namespace: str,
        label_selector: str,
        latency_ms: int = 100,
        config: ExperimentConfig | None = None,
        dry_run: bool = False,
    ) -> tuple[dict[str, Any], Callable | None]:
        """Inject network latency using tc (traffic control)."""
        self._check_emergency_stop()

        k8s = self._get_client()
        v1 = k8s.CoreV1Api()
        pods = v1.list_namespaced_pod(namespace, label_selector=label_selector)
        pod_names = [p.metadata.name for p in pods.items]

        if dry_run:
            return {
                "action": "network_latency",
                "pods": pod_names,
                "latency_ms": latency_ms,
                "dry_run": True,
            }, None

        # Inject latency via exec tc command in each pod
        for pod in pods.items:
            self._exec_in_pod(
                namespace,
                pod.metadata.name,
                ["tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay", f"{latency_ms}ms"],
            )

        logger.info(
            "Injected %dms latency on %d pods in %s",
            latency_ms,
            len(pod_names),
            namespace,
        )

        async def rollback():
            for pod in pods.items:
                self._exec_in_pod(
                    namespace,
                    pod.metadata.name,
                    ["tc", "qdisc", "del", "dev", "eth0", "root"],
                )
            return {"removed_latency": len(pod_names)}

        return {
            "action": "network_latency",
            "pods": pod_names,
            "latency_ms": latency_ms,
        }, rollback

    async def network_loss(
        self,
        namespace: str,
        label_selector: str,
        loss_percent: int = 10,
        config: ExperimentConfig | None = None,
        dry_run: bool = False,
    ) -> tuple[dict[str, Any], Callable | None]:
        """Inject network packet loss."""
        self._check_emergency_stop()

        k8s = self._get_client()
        v1 = k8s.CoreV1Api()
        pods = v1.list_namespaced_pod(namespace, label_selector=label_selector)
        pod_names = [p.metadata.name for p in pods.items]

        if dry_run:
            return {
                "action": "network_loss",
                "pods": pod_names,
                "loss_percent": loss_percent,
                "dry_run": True,
            }, None

        for pod in pods.items:
            self._exec_in_pod(
                namespace,
                pod.metadata.name,
                ["tc", "qdisc", "add", "dev", "eth0", "root", "netem", "loss", f"{loss_percent}%"],
            )

        logger.info(
            "Injected %d%% packet loss on %d pods in %s",
            loss_percent,
            len(pod_names),
            namespace,
        )

        async def rollback():
            for pod in pods.items:
                self._exec_in_pod(
                    namespace,
                    pod.metadata.name,
                    ["tc", "qdisc", "del", "dev", "eth0", "root"],
                )
            return {"removed_loss": len(pod_names)}

        return {
            "action": "network_loss",
            "pods": pod_names,
            "loss_percent": loss_percent,
        }, rollback

    async def cpu_stress(
        self,
        namespace: str,
        label_selector: str,
        cores: int = 1,
        duration_seconds: int = 30,
        config: ExperimentConfig | None = None,
        dry_run: bool = False,
    ) -> tuple[dict[str, Any], Callable | None]:
        """Inject CPU stress using stress-ng."""
        self._check_emergency_stop()

        k8s = self._get_client()
        v1 = k8s.CoreV1Api()
        pods = v1.list_namespaced_pod(namespace, label_selector=label_selector)
        pod_names = [p.metadata.name for p in pods.items]

        if dry_run:
            return {
                "action": "cpu_stress",
                "pods": pod_names,
                "cores": cores,
                "dry_run": True,
            }, None

        for pod in pods.items:
            self._exec_in_pod(
                namespace,
                pod.metadata.name,
                ["stress-ng", "--cpu", str(cores), "--timeout", f"{duration_seconds}s", "--quiet"],
            )

        logger.info("CPU stress on %d pods in %s", len(pod_names), namespace)

        async def rollback():
            for pod in pods.items:
                self._exec_in_pod(
                    namespace,
                    pod.metadata.name,
                    ["pkill", "-f", "stress-ng"],
                )
            return {"killed_stress": len(pod_names)}

        return {
            "action": "cpu_stress",
            "pods": pod_names,
            "cores": cores,
        }, rollback

    async def memory_stress(
        self,
        namespace: str,
        label_selector: str,
        workers: int = 1,
        memory_bytes: str = "256M",
        duration_seconds: int = 30,
        config: ExperimentConfig | None = None,
        dry_run: bool = False,
    ) -> tuple[dict[str, Any], Callable | None]:
        """Inject memory stress using stress-ng."""
        self._check_emergency_stop()

        k8s = self._get_client()
        v1 = k8s.CoreV1Api()
        pods = v1.list_namespaced_pod(namespace, label_selector=label_selector)
        pod_names = [p.metadata.name for p in pods.items]

        if dry_run:
            return {
                "action": "memory_stress",
                "pods": pod_names,
                "memory_bytes": memory_bytes,
                "dry_run": True,
            }, None

        for pod in pods.items:
            self._exec_in_pod(
                namespace,
                pod.metadata.name,
                [
                    "stress-ng",
                    "--vm",
                    str(workers),
                    "--vm-bytes",
                    memory_bytes,
                    "--timeout",
                    f"{duration_seconds}s",
                    "--quiet",
                ],
            )

        logger.info("Memory stress on %d pods in %s", len(pod_names), namespace)

        async def rollback():
            for pod in pods.items:
                self._exec_in_pod(
                    namespace,
                    pod.metadata.name,
                    ["pkill", "-f", "stress-ng"],
                )
            return {"killed_stress": len(pod_names)}

        return {
            "action": "memory_stress",
            "pods": pod_names,
            "memory_bytes": memory_bytes,
        }, rollback

    async def get_topology(self, namespace: str = "default") -> InfraTopology:
        """Discover K8s resource topology."""
        k8s = self._get_client()
        v1 = k8s.CoreV1Api()
        apps_v1 = k8s.AppsV1Api()

        nodes = []
        edges = []

        # Deployments
        deployments = apps_v1.list_namespaced_deployment(namespace)
        for dep in deployments.items:
            dep_id = f"deploy/{dep.metadata.name}"
            nodes.append(
                TopologyNode(
                    id=dep_id,
                    name=dep.metadata.name,
                    resource_type=ResourceType.DEPLOYMENT,
                    namespace=namespace,
                    labels=dep.metadata.labels or {},
                    health=HealthStatus.HEALTHY
                    if dep.status.ready_replicas == dep.status.replicas
                    else HealthStatus.DEGRADED,
                )
            )

        # Pods
        pods = v1.list_namespaced_pod(namespace)
        for pod in pods.items:
            pod_id = f"pod/{pod.metadata.name}"
            phase = pod.status.phase
            health = (
                HealthStatus.HEALTHY
                if phase == "Running"
                else HealthStatus.UNHEALTHY
                if phase == "Failed"
                else HealthStatus.UNKNOWN
            )
            nodes.append(
                TopologyNode(
                    id=pod_id,
                    name=pod.metadata.name,
                    resource_type=ResourceType.POD,
                    namespace=namespace,
                    labels=pod.metadata.labels or {},
                    health=health,
                )
            )
            # Link pod to its owner deployment
            for owner in pod.metadata.owner_references or []:
                if owner.kind == "ReplicaSet":
                    for dep in deployments.items:
                        if pod.metadata.name.startswith(dep.metadata.name):
                            edges.append(
                                TopologyEdge(
                                    source=f"deploy/{dep.metadata.name}",
                                    target=pod_id,
                                    relation="manages",
                                )
                            )

        # Services
        services = v1.list_namespaced_service(namespace)
        for svc in services.items:
            svc_id = f"svc/{svc.metadata.name}"
            nodes.append(
                TopologyNode(
                    id=svc_id,
                    name=svc.metadata.name,
                    resource_type=ResourceType.SERVICE,
                    namespace=namespace,
                    labels=svc.metadata.labels or {},
                    health=HealthStatus.HEALTHY,
                )
            )

        return InfraTopology(nodes=nodes, edges=edges)

    async def get_steady_state(self, namespace: str = "default") -> dict[str, Any]:
        """Capture current steady state metrics."""
        k8s = self._get_client()
        v1 = k8s.CoreV1Api()

        pods = v1.list_namespaced_pod(namespace)
        running = sum(1 for p in pods.items if p.status.phase == "Running")
        total = len(pods.items)

        return {
            "namespace": namespace,
            "pods_total": total,
            "pods_running": running,
            "pods_healthy_ratio": running / total if total > 0 else 1.0,
        }

    def _exec_in_pod(self, namespace: str, pod_name: str, command: list[str]) -> str:
        """Execute a command in a pod container."""
        from kubernetes.stream import stream

        k8s = self._get_client()
        v1 = k8s.CoreV1Api()
        resp = stream(
            v1.connect_get_namespaced_pod_exec,
            pod_name,
            namespace,
            command=command,
            stderr=True,
            stdout=True,
        )
        return resp
