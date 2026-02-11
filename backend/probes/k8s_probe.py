import logging

from .base import BaseProbe, ProbeMode, ProbeResult

logger = logging.getLogger(__name__)


class K8sProbe(BaseProbe):
    """Kubernetes resource state probe.

    Checks deployment ready replicas, pod phase, or generic resource conditions.
    """

    def __init__(
        self,
        name: str,
        mode: ProbeMode,
        namespace: str = "default",
        resource_kind: str = "deployment",
        resource_name: str = "",
        condition: str = "ready",
        expected_value: str | int | None = None,
    ):
        super().__init__(name, mode)
        self.namespace = namespace
        self.resource_kind = resource_kind.lower()
        self.resource_name = resource_name
        self.condition = condition
        self.expected_value = expected_value
        self._client = None

    @property
    def probe_type(self) -> str:
        return "k8s"

    def _get_client(self):
        if self._client is None:
            try:
                from kubernetes import client, config

                config.load_incluster_config()
            except Exception:
                from kubernetes import client, config

                config.load_kube_config()
            self._client = client
        return self._client

    async def execute(self) -> ProbeResult:
        k8s = self._get_client()

        if self.resource_kind == "deployment":
            return await self._check_deployment(k8s)
        elif self.resource_kind == "pod":
            return await self._check_pod(k8s)
        else:
            return ProbeResult(
                probe_name=self.name,
                probe_type=self.probe_type,
                mode=self.mode,
                passed=False,
                error=f"Unsupported resource kind: {self.resource_kind}",
            )

    async def _check_deployment(self, k8s) -> ProbeResult:
        apps_v1 = k8s.AppsV1Api()
        dep = apps_v1.read_namespaced_deployment(self.resource_name, self.namespace)

        desired = dep.spec.replicas or 0
        ready = dep.status.ready_replicas or 0

        if self.condition == "ready":
            if self.expected_value is not None:
                passed = ready >= int(self.expected_value)
            else:
                passed = ready == desired
        else:
            passed = ready == desired

        detail = {
            "deployment": self.resource_name,
            "namespace": self.namespace,
            "desired_replicas": desired,
            "ready_replicas": ready,
            "condition": self.condition,
        }

        return ProbeResult(
            probe_name=self.name,
            probe_type=self.probe_type,
            mode=self.mode,
            passed=passed,
            detail=detail,
        )

    async def _check_pod(self, k8s) -> ProbeResult:
        v1 = k8s.CoreV1Api()
        pod = v1.read_namespaced_pod(self.resource_name, self.namespace)

        phase = pod.status.phase
        expected = str(self.expected_value) if self.expected_value else "Running"
        passed = phase == expected

        detail = {
            "pod": self.resource_name,
            "namespace": self.namespace,
            "phase": phase,
            "expected_phase": expected,
        }

        return ProbeResult(
            probe_name=self.name,
            probe_type=self.probe_type,
            mode=self.mode,
            passed=passed,
            detail=detail,
        )
