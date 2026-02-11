from unittest.mock import AsyncMock, patch

from observability.metrics import METRICS
from observability.middleware import PrometheusMiddleware


class TestMetrics:
    def test_record_experiment_start(self):
        # Should not raise
        METRICS.record_experiment_start()

    def test_record_experiment_end(self):
        METRICS.record_experiment_end("pod_delete", "completed", 5.0)

    def test_record_probe_result(self):
        METRICS.record_probe_result("http", True)
        METRICS.record_probe_result("http", False)

    def test_record_rollback(self):
        METRICS.record_rollback("success")
        METRICS.record_rollback("failed")


class TestPrometheusMiddleware:
    def test_normalize_path_static(self):
        assert PrometheusMiddleware._normalize_path("/health") == "/health"
        assert PrometheusMiddleware._normalize_path("/api/chaos/experiments") == "/api/chaos/experiments"

    def test_normalize_path_with_id(self):
        # 8-char hex IDs get replaced
        assert PrometheusMiddleware._normalize_path("/api/chaos/experiments/a1b2c3d4") == "/api/chaos/experiments/{id}"

    def test_normalize_path_with_dry_prefix(self):
        assert PrometheusMiddleware._normalize_path("/api/chaos/experiments/dry-1234") == "/api/chaos/experiments/{id}"


class TestMetricsEndpoint:
    async def test_metrics_endpoint(self, client):
        resp = await client.get("/metrics")
        assert resp.status_code == 200
        body = resp.text
        assert "chaosduck_http_requests_total" in body

    async def test_metrics_after_experiment(self, client):
        """Run an experiment and verify metrics are updated."""
        with (
            patch("routers.chaos.k8s_engine") as mock_k8s,
            patch("safety.guardrails.snapshot_manager") as mock_snap,
        ):
            mock_k8s.get_steady_state = AsyncMock(return_value={"pods_total": 1})
            mock_k8s.pod_delete = AsyncMock(
                return_value=({"action": "pod_delete", "pods": []}, None)
            )
            mock_snap.capture_k8s_snapshot = AsyncMock()

            await client.post(
                "/api/chaos/experiments",
                json={
                    "name": "metric-test",
                    "chaos_type": "pod_delete",
                    "target_namespace": "default",
                },
            )

        resp = await client.get("/metrics")
        body = resp.text
        assert "chaosduck_experiments_total" in body
        assert "chaosduck_experiment_duration_seconds" in body
