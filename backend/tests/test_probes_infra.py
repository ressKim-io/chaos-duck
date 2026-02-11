from unittest.mock import AsyncMock, MagicMock, patch

from probes.base import ProbeMode
from probes.k8s_probe import K8sProbe
from probes.prom_probe import PromProbe


class TestK8sProbe:
    def _make_probe(self, mock_client, **kwargs):
        probe = K8sProbe(**kwargs)
        probe._client = mock_client
        return probe

    async def test_deployment_ready(self):
        mock_client = MagicMock()
        mock_dep = MagicMock()
        mock_dep.spec.replicas = 3
        mock_dep.status.ready_replicas = 3
        mock_client.AppsV1Api.return_value.read_namespaced_deployment.return_value = mock_dep

        probe = self._make_probe(
            mock_client,
            name="dep-check",
            mode=ProbeMode.SOT,
            resource_kind="deployment",
            resource_name="nginx",
            namespace="default",
        )
        result = await probe.execute()
        assert result.passed is True
        assert result.detail["ready_replicas"] == 3

    async def test_deployment_not_ready(self):
        mock_client = MagicMock()
        mock_dep = MagicMock()
        mock_dep.spec.replicas = 3
        mock_dep.status.ready_replicas = 1
        mock_client.AppsV1Api.return_value.read_namespaced_deployment.return_value = mock_dep

        probe = self._make_probe(
            mock_client,
            name="dep-check",
            mode=ProbeMode.SOT,
            resource_kind="deployment",
            resource_name="nginx",
        )
        result = await probe.execute()
        assert result.passed is False

    async def test_deployment_with_expected_value(self):
        mock_client = MagicMock()
        mock_dep = MagicMock()
        mock_dep.spec.replicas = 3
        mock_dep.status.ready_replicas = 2
        mock_client.AppsV1Api.return_value.read_namespaced_deployment.return_value = mock_dep

        probe = self._make_probe(
            mock_client,
            name="dep-check",
            mode=ProbeMode.SOT,
            resource_kind="deployment",
            resource_name="nginx",
            expected_value=2,
        )
        result = await probe.execute()
        assert result.passed is True

    async def test_pod_running(self):
        mock_client = MagicMock()
        mock_pod = MagicMock()
        mock_pod.status.phase = "Running"
        mock_client.CoreV1Api.return_value.read_namespaced_pod.return_value = mock_pod

        probe = self._make_probe(
            mock_client,
            name="pod-check",
            mode=ProbeMode.SOT,
            resource_kind="pod",
            resource_name="nginx-abc",
        )
        result = await probe.execute()
        assert result.passed is True

    async def test_pod_failed(self):
        mock_client = MagicMock()
        mock_pod = MagicMock()
        mock_pod.status.phase = "Failed"
        mock_client.CoreV1Api.return_value.read_namespaced_pod.return_value = mock_pod

        probe = self._make_probe(
            mock_client,
            name="pod-check",
            mode=ProbeMode.SOT,
            resource_kind="pod",
            resource_name="nginx-abc",
        )
        result = await probe.execute()
        assert result.passed is False

    async def test_unsupported_kind(self):
        mock_client = MagicMock()
        probe = self._make_probe(
            mock_client,
            name="bad-check",
            mode=ProbeMode.SOT,
            resource_kind="statefulset",
            resource_name="db",
        )
        result = await probe.execute()
        assert result.passed is False
        assert "Unsupported" in result.error


class TestPromProbe:
    async def test_success_gt(self):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "data": {"result": [{"value": [1234, "0.95"]}]}
        }

        with patch("probes.prom_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.get = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = PromProbe(
                name="error-rate",
                mode=ProbeMode.CONTINUOUS,
                endpoint="http://prometheus:9090",
                query='rate(http_errors[5m])',
                comparator="<",
                threshold=1.0,
            )
            result = await probe.execute()

        assert result.passed is True
        assert result.detail["value"] == 0.95

    async def test_fail_threshold(self):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "data": {"result": [{"value": [1234, "5.0"]}]}
        }

        with patch("probes.prom_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.get = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = PromProbe(
                name="error-rate",
                mode=ProbeMode.CONTINUOUS,
                endpoint="http://prometheus:9090",
                query='rate(http_errors[5m])',
                comparator="<",
                threshold=1.0,
            )
            result = await probe.execute()

        assert result.passed is False

    async def test_no_results(self):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {"data": {"result": []}}

        with patch("probes.prom_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.get = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = PromProbe(
                name="empty",
                mode=ProbeMode.SOT,
                endpoint="http://prometheus:9090",
                query="up",
            )
            result = await probe.execute()

        assert result.passed is False

    async def test_prometheus_error(self):
        mock_response = MagicMock()
        mock_response.status_code = 500

        with patch("probes.prom_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.get = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = PromProbe(
                name="error",
                mode=ProbeMode.SOT,
                endpoint="http://prometheus:9090",
                query="up",
            )
            result = await probe.execute()

        assert result.passed is False
        assert "500" in result.error

    def test_comparators(self):
        probe = PromProbe(
            name="test",
            mode=ProbeMode.SOT,
            endpoint="http://localhost",
            query="up",
        )
        probe.threshold = 5.0

        probe.comparator = ">"
        assert probe._compare(6.0) is True
        assert probe._compare(4.0) is False

        probe.comparator = ">="
        assert probe._compare(5.0) is True

        probe.comparator = "<"
        assert probe._compare(4.0) is True

        probe.comparator = "<="
        assert probe._compare(5.0) is True

        probe.comparator = "=="
        assert probe._compare(5.0) is True
        assert probe._compare(4.0) is False

        probe.comparator = "!="
        assert probe._compare(4.0) is True
        assert probe._compare(5.0) is False
