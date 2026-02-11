import pytest
from pydantic import ValidationError

from models.experiment import (
    ChaosType,
    ExperimentConfig,
    ExperimentPhase,
    ExperimentResult,
    ExperimentStatus,
    SafetyConfig,
)
from models.topology import (
    HealthStatus,
    InfraTopology,
    ResilienceScore,
    ResourceType,
    TopologyEdge,
    TopologyNode,
)


class TestExperimentEnums:
    def test_experiment_phase_values(self):
        assert ExperimentPhase.STEADY_STATE == "steady_state"
        assert ExperimentPhase.HYPOTHESIS == "hypothesis"
        assert ExperimentPhase.INJECT == "inject"
        assert ExperimentPhase.OBSERVE == "observe"
        assert ExperimentPhase.ROLLBACK == "rollback"

    def test_experiment_status_values(self):
        assert ExperimentStatus.PENDING == "pending"
        assert ExperimentStatus.RUNNING == "running"
        assert ExperimentStatus.COMPLETED == "completed"
        assert ExperimentStatus.FAILED == "failed"
        assert ExperimentStatus.ROLLED_BACK == "rolled_back"
        assert ExperimentStatus.EMERGENCY_STOPPED == "emergency_stopped"

    def test_chaos_type_k8s(self):
        k8s_types = [
            ChaosType.POD_DELETE,
            ChaosType.NETWORK_LATENCY,
            ChaosType.NETWORK_LOSS,
            ChaosType.CPU_STRESS,
            ChaosType.MEMORY_STRESS,
        ]
        assert len(k8s_types) == 5

    def test_chaos_type_aws(self):
        aws_types = [
            ChaosType.EC2_STOP,
            ChaosType.RDS_FAILOVER,
            ChaosType.ROUTE_BLACKHOLE,
        ]
        assert len(aws_types) == 3


class TestSafetyConfig:
    def test_defaults(self):
        config = SafetyConfig()
        assert config.timeout_seconds == 30
        assert config.require_confirmation is False
        assert config.max_blast_radius == 0.3
        assert config.dry_run is False
        assert config.namespace_pattern is None

    def test_timeout_min_boundary(self):
        config = SafetyConfig(timeout_seconds=1)
        assert config.timeout_seconds == 1

    def test_timeout_max_boundary(self):
        config = SafetyConfig(timeout_seconds=120)
        assert config.timeout_seconds == 120

    def test_timeout_below_min_fails(self):
        with pytest.raises(ValidationError):
            SafetyConfig(timeout_seconds=0)

    def test_timeout_above_max_fails(self):
        with pytest.raises(ValidationError):
            SafetyConfig(timeout_seconds=121)

    def test_blast_radius_boundary(self):
        SafetyConfig(max_blast_radius=0.0)
        SafetyConfig(max_blast_radius=1.0)

    def test_blast_radius_out_of_range(self):
        with pytest.raises(ValidationError):
            SafetyConfig(max_blast_radius=1.1)
        with pytest.raises(ValidationError):
            SafetyConfig(max_blast_radius=-0.1)


class TestExperimentConfig:
    def test_minimal_config(self):
        config = ExperimentConfig(name="test", chaos_type=ChaosType.POD_DELETE)
        assert config.name == "test"
        assert config.target_namespace is None
        assert config.parameters == {}
        assert isinstance(config.safety, SafetyConfig)

    def test_full_config(self):
        config = ExperimentConfig(
            name="full-test",
            chaos_type=ChaosType.EC2_STOP,
            target_namespace="production",
            target_labels={"app": "web"},
            target_resource="i-123",
            parameters={"instance_ids": ["i-123"]},
            safety=SafetyConfig(dry_run=True, require_confirmation=True),
            description="Stop an EC2 instance",
        )
        assert config.target_resource == "i-123"
        assert config.safety.dry_run is True


class TestExperimentResult:
    def test_defaults(self):
        result = ExperimentResult(
            experiment_id="abc123",
            config=ExperimentConfig(name="t", chaos_type=ChaosType.POD_DELETE),
        )
        assert result.status == ExperimentStatus.PENDING
        assert result.phase == ExperimentPhase.STEADY_STATE
        assert result.started_at is None
        assert result.error is None


class TestTopologyModels:
    def test_resource_type_values(self):
        assert ResourceType.POD == "pod"
        assert ResourceType.EC2 == "ec2"
        assert ResourceType.RDS == "rds"

    def test_health_status_values(self):
        assert HealthStatus.HEALTHY == "healthy"
        assert HealthStatus.UNKNOWN == "unknown"

    def test_topology_node_defaults(self):
        node = TopologyNode(id="n1", name="test", resource_type=ResourceType.POD)
        assert node.labels == {}
        assert node.health == HealthStatus.UNKNOWN
        assert node.metadata == {}

    def test_topology_edge_defaults(self):
        edge = TopologyEdge(source="a", target="b")
        assert edge.relation == "connects_to"

    def test_infra_topology_empty(self):
        topo = InfraTopology()
        assert topo.nodes == []
        assert topo.edges == []

    def test_resilience_score_valid(self):
        score = ResilienceScore(overall=85.0, categories={"k8s": 90.0})
        assert score.overall == 85.0

    def test_resilience_score_out_of_range(self):
        with pytest.raises(ValidationError):
            ResilienceScore(overall=101.0)
        with pytest.raises(ValidationError):
            ResilienceScore(overall=-1.0)
