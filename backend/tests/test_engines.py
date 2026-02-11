from unittest.mock import MagicMock

import pytest

from engines.ai_engine import AiEngine, AnalysisResult
from engines.aws_engine import AwsEngine
from engines.k8s_engine import K8sEngine
from models.experiment import ChaosType, ExperimentConfig, SafetyConfig
from safety.guardrails import emergency_stop_manager


# ──────────────────────────────────────────────
# K8sEngine
# ──────────────────────────────────────────────
class TestK8sEngine:
    def _make_engine(self, mock_client):
        engine = K8sEngine()
        engine._client = mock_client
        return engine

    async def test_pod_delete_dry_run(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        config = ExperimentConfig(
            name="t",
            chaos_type=ChaosType.POD_DELETE,
            safety=SafetyConfig(dry_run=True, max_blast_radius=1.0),
        )
        result, rollback_fn = await engine.pod_delete(
            "default", "app=nginx", config=config, dry_run=True
        )
        assert result["dry_run"] is True
        assert result["action"] == "pod_delete"
        assert rollback_fn is None

    async def test_pod_delete_emergency_stop(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        emergency_stop_manager.trigger()
        with pytest.raises(RuntimeError, match="Emergency stop"):
            await engine.pod_delete("default", "app=nginx")

    async def test_network_latency_dry_run(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        result, rollback_fn = await engine.network_latency(
            "default", "app=nginx", latency_ms=200, dry_run=True
        )
        assert result["dry_run"] is True
        assert result["latency_ms"] == 200
        assert rollback_fn is None

    async def test_network_loss_dry_run(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        result, rollback_fn = await engine.network_loss(
            "default", "app=nginx", loss_percent=25, dry_run=True
        )
        assert result["dry_run"] is True
        assert result["loss_percent"] == 25

    async def test_cpu_stress_dry_run(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        result, rollback_fn = await engine.cpu_stress("default", "app=nginx", cores=2, dry_run=True)
        assert result["dry_run"] is True
        assert result["cores"] == 2

    async def test_memory_stress_dry_run(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        result, rollback_fn = await engine.memory_stress(
            "default", "app=nginx", memory_bytes="512M", dry_run=True
        )
        assert result["dry_run"] is True
        assert result["memory_bytes"] == "512M"

    async def test_get_topology(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        topo = await engine.get_topology("default")
        assert len(topo.nodes) >= 1  # at least deployment, pod, service

    async def test_get_steady_state(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        state = await engine.get_steady_state("default")
        assert state["namespace"] == "default"
        assert state["pods_total"] == 1
        assert state["pods_running"] == 1
        assert state["pods_healthy_ratio"] == 1.0

    async def test_pod_delete_blast_radius_exceeded(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        config = ExperimentConfig(
            name="t",
            chaos_type=ChaosType.POD_DELETE,
            safety=SafetyConfig(max_blast_radius=0.0),  # zero tolerance
        )
        with pytest.raises(ValueError, match="Blast radius exceeded"):
            await engine.pod_delete("default", "app=nginx", config=config)

    async def test_network_latency_emergency_stop(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        emergency_stop_manager.trigger()
        with pytest.raises(RuntimeError, match="Emergency stop"):
            await engine.network_latency("default", "app=nginx")

    async def test_cpu_stress_emergency_stop(self, mock_k8s_client):
        engine = self._make_engine(mock_k8s_client)
        emergency_stop_manager.trigger()
        with pytest.raises(RuntimeError):
            await engine.cpu_stress("default", "app=nginx")


# ──────────────────────────────────────────────
# AwsEngine
# ──────────────────────────────────────────────
class TestAwsEngine:
    def _make_engine(self, mock_ec2, mock_rds):
        engine = AwsEngine()
        engine._ec2 = mock_ec2
        engine._rds = mock_rds
        return engine

    async def test_stop_ec2_dry_run(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        result, rollback_fn = await engine.stop_ec2(["i-123"], dry_run=True)
        assert result["dry_run"] is True
        assert result["action"] == "stop_ec2"
        assert rollback_fn is None

    async def test_stop_ec2_actual(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        result, rollback_fn = await engine.stop_ec2(["i-123"])
        assert result["action"] == "stop_ec2"
        assert rollback_fn is not None
        ec2.stop_instances.assert_called_once_with(InstanceIds=["i-123"])

    async def test_stop_ec2_rollback(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        _, rollback_fn = await engine.stop_ec2(["i-123"])
        result = await rollback_fn()
        ec2.start_instances.assert_called_once_with(InstanceIds=["i-123"])
        assert result == {"started": ["i-123"]}

    async def test_failover_rds_dry_run(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        result, rollback_fn = await engine.failover_rds("my-cluster", dry_run=True)
        assert result["dry_run"] is True

    async def test_failover_rds_actual(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        result, rollback_fn = await engine.failover_rds("my-cluster")
        rds.failover_db_cluster.assert_called_once()
        assert rollback_fn is not None

    async def test_blackhole_route_dry_run(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        result, rollback_fn = await engine.blackhole_route("rtb-1", "10.0.0.0/8", dry_run=True)
        assert result["dry_run"] is True

    async def test_blackhole_route_actual(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        result, rollback_fn = await engine.blackhole_route("rtb-1", "10.0.0.0/8")
        ec2.create_route.assert_called_once()
        assert rollback_fn is not None

    async def test_ec2_emergency_stop(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        emergency_stop_manager.trigger()
        with pytest.raises(RuntimeError, match="Emergency stop"):
            await engine.stop_ec2(["i-123"])

    async def test_rds_emergency_stop(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        emergency_stop_manager.trigger()
        with pytest.raises(RuntimeError):
            await engine.failover_rds("cluster")

    async def test_get_topology(self, mock_boto3):
        ec2, rds = mock_boto3
        engine = self._make_engine(ec2, rds)
        topo = await engine.get_topology()
        assert len(topo.nodes) >= 2  # ec2 + rds


# ──────────────────────────────────────────────
# AiEngine
# ──────────────────────────────────────────────
class TestAiEngine:
    def _make_engine(self, mock_client):
        engine = AiEngine(api_key="test-key")
        engine._client = mock_client
        return engine

    async def test_analyze_experiment(self, mock_anthropic):
        engine = self._make_engine(mock_anthropic)
        result = await engine.analyze_experiment(
            experiment_data={"name": "test"},
            steady_state={"pods_total": 3},
            observations={"pods_total": 2},
        )
        assert isinstance(result, AnalysisResult)
        assert result.severity == "SEV3"
        assert result.confidence == 0.8

    async def test_generate_hypothesis(self, mock_anthropic):
        mock_anthropic.messages.create.return_value.content = [
            MagicMock(text="Deleting pods will cause service disruption.")
        ]
        engine = self._make_engine(mock_anthropic)
        hypothesis = await engine.generate_hypothesis({}, "nginx", "pod_delete")
        assert "pod" in hypothesis.lower() or len(hypothesis) > 0

    async def test_calculate_resilience_score(self, mock_anthropic):
        mock_anthropic.messages.create.return_value.content = [
            MagicMock(
                text='{"overall": 80, "categories": {}, "recommendations": [], "details": "ok"}'
            )
        ]
        engine = self._make_engine(mock_anthropic)
        score = await engine.calculate_resilience_score([])
        assert score["overall"] == 80

    async def test_generate_report(self, mock_anthropic):
        mock_anthropic.messages.create.return_value.content = [
            MagicMock(text="# Report\n\nSummary here.")
        ]
        engine = self._make_engine(mock_anthropic)
        report = await engine.generate_report({"name": "test"})
        assert "Report" in report

    async def test_generate_experiments(self, mock_anthropic):
        mock_anthropic.messages.create.return_value.content = [
            MagicMock(
                text='[{"name": "kill-nginx", "chaos_type": "pod_delete",'
                '"target_namespace": "default", "target_labels": {"app": "nginx"},'
                '"parameters": {}, "description": "Test nginx pod recovery"}]'
            )
        ]
        engine = self._make_engine(mock_anthropic)
        results = await engine.generate_experiments({"nodes": []}, "default", 1)
        assert len(results) == 1
        assert results[0]["name"] == "kill-nginx"
        assert results[0]["chaos_type"] == "pod_delete"

    async def test_generate_experiments_filters_invalid(self, mock_anthropic):
        mock_anthropic.messages.create.return_value.content = [
            MagicMock(
                text='[{"name": "valid", "chaos_type": "pod_delete"},'
                '{"name": "invalid", "chaos_type": "nonexistent_type"}]'
            )
        ]
        engine = self._make_engine(mock_anthropic)
        results = await engine.generate_experiments({"nodes": []}, "default", 2)
        # Only the valid one should pass Pydantic validation
        assert len(results) == 1
        assert results[0]["name"] == "valid"
