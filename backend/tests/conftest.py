from unittest.mock import MagicMock

import pytest
from httpx import ASGITransport, AsyncClient

from models.experiment import ChaosType, ExperimentConfig, SafetyConfig
from safety.guardrails import emergency_stop_manager
from safety.rollback import RollbackManager
from safety.snapshot import SnapshotManager


@pytest.fixture(autouse=True)
def _reset_emergency_stop():
    """Reset the global emergency stop before each test."""
    emergency_stop_manager.reset()
    yield
    emergency_stop_manager.reset()


@pytest.fixture()
def rollback_mgr():
    """Fresh RollbackManager instance per test."""
    return RollbackManager()


@pytest.fixture()
def snapshot_mgr():
    """Fresh SnapshotManager instance per test."""
    return SnapshotManager()


@pytest.fixture()
def sample_config():
    """Sample ExperimentConfig for testing."""
    return ExperimentConfig(
        name="test-experiment",
        chaos_type=ChaosType.POD_DELETE,
        target_namespace="default",
        target_labels={"app": "nginx"},
        parameters={},
        safety=SafetyConfig(timeout_seconds=10, dry_run=True),
    )


@pytest.fixture()
def sample_ec2_config():
    return ExperimentConfig(
        name="test-ec2-stop",
        chaos_type=ChaosType.EC2_STOP,
        parameters={"instance_ids": ["i-123"]},
        safety=SafetyConfig(dry_run=True),
    )


@pytest.fixture()
def mock_k8s_client():
    """Mock kubernetes client module."""
    mock_client = MagicMock()

    mock_pod = MagicMock()
    mock_pod.metadata.name = "nginx-abc123"
    mock_pod.metadata.labels = {"app": "nginx"}
    mock_pod.metadata.owner_references = []
    mock_pod.status.phase = "Running"

    mock_pod_list = MagicMock()
    mock_pod_list.items = [mock_pod]

    mock_v1 = MagicMock()
    mock_v1.list_namespaced_pod.return_value = mock_pod_list

    mock_dep = MagicMock()
    mock_dep.metadata.name = "nginx"
    mock_dep.metadata.labels = {"app": "nginx"}
    mock_dep.status.ready_replicas = 1
    mock_dep.status.replicas = 1

    mock_dep_list = MagicMock()
    mock_dep_list.items = [mock_dep]

    mock_svc = MagicMock()
    mock_svc.metadata.name = "nginx-svc"
    mock_svc.metadata.labels = {"app": "nginx"}

    mock_svc_list = MagicMock()
    mock_svc_list.items = [mock_svc]

    mock_v1.list_namespaced_service.return_value = mock_svc_list

    mock_apps_v1 = MagicMock()
    mock_apps_v1.list_namespaced_deployment.return_value = mock_dep_list

    mock_client.CoreV1Api.return_value = mock_v1
    mock_client.AppsV1Api.return_value = mock_apps_v1

    return mock_client


@pytest.fixture()
def mock_boto3():
    """Mock boto3 clients."""
    mock_ec2 = MagicMock()
    mock_ec2.describe_instances.return_value = {
        "Reservations": [
            {
                "Instances": [
                    {
                        "InstanceId": "i-123",
                        "State": {"Name": "running"},
                        "Tags": [{"Key": "Name", "Value": "web-server"}],
                        "VpcId": "vpc-abc",
                        "InstanceType": "t3.micro",
                    }
                ]
            }
        ]
    }
    mock_ec2.describe_route_tables.return_value = {"RouteTables": [{"Routes": []}]}

    mock_rds = MagicMock()
    mock_rds.describe_db_clusters.return_value = {
        "DBClusters": [
            {
                "DBClusterIdentifier": "my-cluster",
                "Status": "available",
                "Engine": "aurora-mysql",
            }
        ]
    }

    return mock_ec2, mock_rds


@pytest.fixture()
def mock_anthropic():
    """Mock Anthropic client."""
    mock_client = MagicMock()
    mock_message = MagicMock()
    mock_message.content = [
        MagicMock(
            text='{"severity": "SEV3", "root_cause": "test", '
            '"confidence": 0.8, "recommendations": [], "resilience_score": 75.0}'
        )
    ]
    mock_client.messages.create.return_value = mock_message
    return mock_client


@pytest.fixture()
async def client():
    """Async HTTP test client for FastAPI app."""
    from main import app

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as ac:
        yield ac
