from unittest.mock import MagicMock, patch

from safety.snapshot import SnapshotManager


class TestK8sSnapshotCapture:
    async def test_captures_pods_deployments_services(self):
        """Verify K8s snapshot captures real resource data."""
        mgr = SnapshotManager()

        # Mock K8s client
        mock_client = MagicMock()

        mock_container = MagicMock()
        mock_container.name = "nginx"
        mock_container.image = "nginx:1.25"

        mock_pod = MagicMock()
        mock_pod.metadata.name = "nginx-abc"
        mock_pod.metadata.namespace = "default"
        mock_pod.metadata.labels = {"app": "nginx"}
        mock_pod.status.phase = "Running"
        mock_pod.spec.containers = [mock_container]
        mock_pod.spec.node_name = "node-1"

        mock_pod_list = MagicMock()
        mock_pod_list.items = [mock_pod]

        mock_dep = MagicMock()
        mock_dep.metadata.name = "nginx"
        mock_dep.metadata.namespace = "default"
        mock_dep.metadata.labels = {"app": "nginx"}
        mock_dep.spec.replicas = 3
        mock_dep.status.ready_replicas = 3
        mock_dep.spec.selector.match_labels = {"app": "nginx"}

        mock_dep_list = MagicMock()
        mock_dep_list.items = [mock_dep]

        mock_port = MagicMock()
        mock_port.port = 80
        mock_port.target_port = 8080
        mock_port.protocol = "TCP"

        mock_svc = MagicMock()
        mock_svc.metadata.name = "nginx-svc"
        mock_svc.metadata.namespace = "default"
        mock_svc.metadata.labels = {"app": "nginx"}
        mock_svc.spec.type = "ClusterIP"
        mock_svc.spec.cluster_ip = "10.96.0.1"
        mock_svc.spec.ports = [mock_port]

        mock_svc_list = MagicMock()
        mock_svc_list.items = [mock_svc]

        mock_v1 = MagicMock()
        mock_v1.list_namespaced_pod.return_value = mock_pod_list
        mock_v1.list_namespaced_service.return_value = mock_svc_list

        mock_apps_v1 = MagicMock()
        mock_apps_v1.list_namespaced_deployment.return_value = mock_dep_list

        mock_client.CoreV1Api.return_value = mock_v1
        mock_client.AppsV1Api.return_value = mock_apps_v1
        mgr._k8s_client = mock_client

        with patch.object(mgr, "_persist_snapshot"):
            snap = await mgr.capture_k8s_snapshot("exp1", "default", {"app": "nginx"})

        assert snap["type"] == "k8s"
        assert len(snap["resources"]["pods"]) == 1
        assert snap["resources"]["pods"][0]["name"] == "nginx-abc"
        assert snap["resources"]["pods"][0]["containers"][0]["image"] == "nginx:1.25"
        assert len(snap["resources"]["deployments"]) == 1
        assert snap["resources"]["deployments"][0]["replicas"] == 3
        assert len(snap["resources"]["services"]) == 1
        assert snap["resources"]["services"][0]["ports"][0]["port"] == 80

    async def test_fallback_on_k8s_unavailable(self):
        """Falls back to empty resources when K8s is not reachable."""
        mgr = SnapshotManager()
        mgr._k8s_client = "force_error"  # Will cause attribute error

        with patch.object(mgr, "_persist_snapshot"):
            snap = await mgr.capture_k8s_snapshot("exp1", "default")

        assert snap["type"] == "k8s"
        assert snap["resources"]["pods"] == []
        assert snap["resources"]["deployments"] == []
        assert snap["resources"]["services"] == []


class TestAwsSnapshotCapture:
    async def test_captures_ec2_state(self):
        """Verify AWS snapshot captures EC2 instance details."""
        mgr = SnapshotManager()

        mock_ec2 = MagicMock()
        mock_ec2.describe_instances.return_value = {
            "Reservations": [
                {
                    "Instances": [
                        {
                            "InstanceId": "i-123",
                            "InstanceType": "t3.micro",
                            "State": {"Name": "running"},
                            "VpcId": "vpc-abc",
                            "SubnetId": "subnet-xyz",
                            "SecurityGroups": [{"GroupId": "sg-123"}],
                            "Tags": [{"Key": "Name", "Value": "web"}],
                        }
                    ]
                }
            ]
        }
        mgr._boto3_ec2 = mock_ec2
        mgr._boto3_rds = MagicMock()

        with patch.object(mgr, "_persist_snapshot"):
            snap = await mgr.capture_aws_snapshot("exp1", "ec2", "i-123")

        assert snap["type"] == "aws"
        assert snap["state"]["instance_id"] == "i-123"
        assert snap["state"]["state"] == "running"
        assert snap["state"]["vpc_id"] == "vpc-abc"
        assert snap["state"]["security_groups"] == ["sg-123"]
        assert snap["state"]["tags"]["Name"] == "web"

    async def test_captures_rds_state(self):
        """Verify AWS snapshot captures RDS cluster details."""
        mgr = SnapshotManager()

        mock_rds = MagicMock()
        mock_rds.describe_db_clusters.return_value = {
            "DBClusters": [
                {
                    "DBClusterIdentifier": "my-cluster",
                    "Status": "available",
                    "Engine": "aurora-mysql",
                    "EngineVersion": "8.0.28",
                    "Endpoint": "my-cluster.abc.us-east-1.rds.amazonaws.com",
                    "ReaderEndpoint": "my-cluster-ro.abc.us-east-1.rds.amazonaws.com",
                    "DBClusterMembers": [
                        {"DBInstanceIdentifier": "inst-1", "IsClusterWriter": True},
                        {"DBInstanceIdentifier": "inst-2", "IsClusterWriter": False},
                    ],
                }
            ]
        }
        mgr._boto3_ec2 = MagicMock()
        mgr._boto3_rds = mock_rds

        with patch.object(mgr, "_persist_snapshot"):
            snap = await mgr.capture_aws_snapshot("exp1", "rds", "my-cluster")

        assert snap["state"]["cluster_id"] == "my-cluster"
        assert snap["state"]["status"] == "available"
        assert len(snap["state"]["members"]) == 2
        assert snap["state"]["members"][0]["is_writer"] is True

    async def test_fallback_on_aws_unavailable(self):
        """Falls back to empty state when AWS is not reachable."""
        mgr = SnapshotManager()
        mgr._boto3_ec2 = "force_error"
        mgr._boto3_rds = "force_error"

        with patch.object(mgr, "_persist_snapshot"):
            snap = await mgr.capture_aws_snapshot("exp1", "ec2", "i-123")

        assert snap["type"] == "aws"
        assert snap["state"] == {}


class TestSnapshotRestore:
    async def test_restore_detects_missing_pod(self):
        """Restore detects pods that existed in snapshot but are gone."""
        mgr = SnapshotManager()

        # Set up snapshot with a pod
        mgr._snapshots["exp1"] = {
            "type": "k8s",
            "namespace": "default",
            "resources": {
                "pods": [{"name": "nginx-abc", "phase": "Running"}],
                "deployments": [],
                "services": [],
            },
        }

        # Mock current state: no pods
        mock_client = MagicMock()
        mock_pod_list = MagicMock()
        mock_pod_list.items = []
        mock_client.CoreV1Api.return_value.list_namespaced_pod.return_value = mock_pod_list
        mgr._k8s_client = mock_client

        result = await mgr.restore_from_snapshot("exp1")
        assert result is not None
        assert len(result["actions"]) == 1
        assert result["actions"][0]["action"] == "pod_missing"
        assert result["actions"][0]["name"] == "nginx-abc"

    async def test_restore_no_snapshot(self):
        """Returns None when no snapshot exists."""
        mgr = SnapshotManager()
        result = await mgr.restore_from_snapshot("nonexistent")
        assert result is None

    async def test_restore_aws_detects_state_drift(self):
        """Restore detects EC2 state drift."""
        mgr = SnapshotManager()

        mgr._snapshots["exp1"] = {
            "type": "aws",
            "resource_type": "ec2",
            "state": {
                "instance_id": "i-123",
                "state": "running",
            },
        }

        mock_ec2 = MagicMock()
        mock_ec2.describe_instances.return_value = {
            "Reservations": [
                {
                    "Instances": [
                        {
                            "InstanceId": "i-123",
                            "State": {"Name": "stopped"},
                        }
                    ]
                }
            ]
        }
        mgr._boto3_ec2 = mock_ec2
        mgr._boto3_rds = MagicMock()

        result = await mgr.restore_from_snapshot("exp1")
        assert len(result["actions"]) == 1
        assert result["actions"][0]["action"] == "state_drift"
        assert result["actions"][0]["snapshot_state"] == "running"
        assert result["actions"][0]["current_state"] == "stopped"
