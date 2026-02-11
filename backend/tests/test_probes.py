from unittest.mock import AsyncMock, MagicMock, patch

from probes.base import ProbeMode, ProbeResult
from probes.cmd_probe import CmdProbe
from probes.http_probe import HttpProbe
from probes.k8s_probe import K8sProbe
from probes.prom_probe import PromProbe


class TestProbeResult:
    def test_creates_with_defaults(self):
        r = ProbeResult(
            probe_name="test",
            probe_type="http",
            mode=ProbeMode.SOT,
            passed=True,
        )
        assert r.passed is True
        assert r.executed_at is not None
        assert r.error is None

    def test_creates_with_error(self):
        r = ProbeResult(
            probe_name="test",
            probe_type="http",
            mode=ProbeMode.EOT,
            passed=False,
            error="connection refused",
        )
        assert r.passed is False
        assert r.error == "connection refused"


class TestHttpProbe:
    async def test_success(self):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = '{"status": "healthy"}'
        mock_response.elapsed.total_seconds.return_value = 0.05

        with patch("probes.http_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.request = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = HttpProbe(
                name="health-check",
                mode=ProbeMode.SOT,
                url="http://localhost:8000/health",
                expected_status=200,
            )
            result = await probe.execute()

        assert result.passed is True
        assert result.detail["status_code"] == 200

    async def test_wrong_status(self):
        mock_response = MagicMock()
        mock_response.status_code = 500
        mock_response.text = "error"
        mock_response.elapsed.total_seconds.return_value = 0.1

        with patch("probes.http_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.request = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = HttpProbe(
                name="health-check",
                mode=ProbeMode.SOT,
                url="http://localhost:8000/health",
            )
            result = await probe.execute()

        assert result.passed is False

    async def test_body_pattern_match(self):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = '{"status": "healthy"}'
        mock_response.elapsed.total_seconds.return_value = 0.05

        with patch("probes.http_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.request = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = HttpProbe(
                name="health-check",
                mode=ProbeMode.SOT,
                url="http://localhost:8000/health",
                body_pattern="healthy",
            )
            result = await probe.execute()

        assert result.passed is True
        assert result.detail["body_match"] is True

    async def test_body_pattern_no_match(self):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.text = '{"status": "degraded"}'
        mock_response.elapsed.total_seconds.return_value = 0.05

        with patch("probes.http_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.request = AsyncMock(return_value=mock_response)
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = HttpProbe(
                name="health-check",
                mode=ProbeMode.SOT,
                url="http://localhost:8000/health",
                body_pattern="healthy",
            )
            result = await probe.execute()

        assert result.passed is False

    async def test_safe_execute_on_error(self):
        with patch("probes.http_probe.httpx.AsyncClient") as mock_client_cls:
            mock_client = AsyncMock()
            mock_client.request = AsyncMock(side_effect=Exception("connection refused"))
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client_cls.return_value = mock_client

            probe = HttpProbe(
                name="health-check",
                mode=ProbeMode.SOT,
                url="http://localhost:8000/health",
            )
            result = await probe.safe_execute()

        assert result.passed is False
        assert "connection refused" in result.error


class TestCmdProbe:
    async def test_success(self):
        probe = CmdProbe(
            name="check-echo",
            mode=ProbeMode.SOT,
            command="echo hello",
            expected_exit_code=0,
        )
        result = await probe.execute()
        assert result.passed is True
        assert result.detail["exit_code"] == 0

    async def test_output_contains(self):
        probe = CmdProbe(
            name="check-hello",
            mode=ProbeMode.SOT,
            command="echo hello world",
            output_contains="hello",
        )
        result = await probe.execute()
        assert result.passed is True
        assert result.detail["output_match"] is True

    async def test_output_not_contains(self):
        probe = CmdProbe(
            name="check-missing",
            mode=ProbeMode.SOT,
            command="echo hello",
            output_contains="goodbye",
        )
        result = await probe.execute()
        assert result.passed is False

    async def test_wrong_exit_code(self):
        probe = CmdProbe(
            name="check-fail",
            mode=ProbeMode.SOT,
            command="false",
            expected_exit_code=0,
        )
        result = await probe.execute()
        assert result.passed is False
        assert result.detail["exit_code"] != 0

    async def test_timeout(self):
        probe = CmdProbe(
            name="check-timeout",
            mode=ProbeMode.SOT,
            command="sleep 10",
            timeout_seconds=0.1,
        )
        result = await probe.execute()
        assert result.passed is False
        assert "timed out" in result.error


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


class TestProbeConfig:
    def test_probe_config_model(self):
        from models.experiment import ProbeConfig, ProbeMode, ProbeType

        pc = ProbeConfig(
            name="health",
            type=ProbeType.HTTP,
            mode=ProbeMode.SOT,
            properties={"url": "http://localhost/health"},
        )
        assert pc.name == "health"
        assert pc.type == ProbeType.HTTP
        assert pc.mode == ProbeMode.SOT

    def test_experiment_config_with_probes(self):
        from models.experiment import (
            ChaosType,
            ExperimentConfig,
            ProbeConfig,
            ProbeMode,
            ProbeType,
        )

        config = ExperimentConfig(
            name="test",
            chaos_type=ChaosType.POD_DELETE,
            probes=[
                ProbeConfig(
                    name="health",
                    type=ProbeType.HTTP,
                    mode=ProbeMode.SOT,
                    properties={"url": "http://localhost/health"},
                )
            ],
        )
        assert len(config.probes) == 1
        assert config.probes[0].name == "health"
