import logging
from collections.abc import Callable
from typing import Any

from models.topology import (
    HealthStatus,
    InfraTopology,
    ResourceType,
    TopologyEdge,
    TopologyNode,
)
from safety.guardrails import emergency_stop_manager

logger = logging.getLogger(__name__)


class AwsEngine:
    """AWS chaos engine using boto3.

    All mutation methods return (result, rollback_fn) tuples.
    """

    def __init__(self, region: str = "us-east-1"):
        self._region = region
        self._ec2 = None
        self._rds = None

    def _get_ec2(self):
        if self._ec2 is None:
            import boto3

            self._ec2 = boto3.client("ec2", region_name=self._region)
        return self._ec2

    def _get_rds(self):
        if self._rds is None:
            import boto3

            self._rds = boto3.client("rds", region_name=self._region)
        return self._rds

    def _check_emergency_stop(self) -> None:
        if emergency_stop_manager.is_triggered():
            raise RuntimeError("Emergency stop is active.")

    async def stop_ec2(
        self,
        instance_ids: list[str],
        dry_run: bool = False,
    ) -> tuple[dict[str, Any], Callable | None]:
        """Stop EC2 instances."""
        self._check_emergency_stop()

        ec2 = self._get_ec2()

        if dry_run:
            return {
                "action": "stop_ec2",
                "instance_ids": instance_ids,
                "dry_run": True,
            }, None

        ec2.stop_instances(InstanceIds=instance_ids)
        logger.info("Stopped EC2 instances: %s", instance_ids)

        async def rollback():
            ec2.start_instances(InstanceIds=instance_ids)
            logger.info("Rollback: started EC2 instances: %s", instance_ids)
            return {"started": instance_ids}

        return {"action": "stop_ec2", "instance_ids": instance_ids}, rollback

    async def failover_rds(
        self,
        db_cluster_id: str,
        dry_run: bool = False,
    ) -> tuple[dict[str, Any], Callable | None]:
        """Force RDS failover."""
        self._check_emergency_stop()

        rds = self._get_rds()

        if dry_run:
            return {
                "action": "rds_failover",
                "db_cluster_id": db_cluster_id,
                "dry_run": True,
            }, None

        rds.failover_db_cluster(DBClusterIdentifier=db_cluster_id)
        logger.info("Triggered RDS failover: %s", db_cluster_id)

        # RDS failover is self-healing; rollback is a no-op
        async def rollback():
            logger.info("RDS failover rollback: cluster will self-heal")
            return {"note": "RDS failover is self-healing"}

        return {"action": "rds_failover", "db_cluster_id": db_cluster_id}, rollback

    async def blackhole_route(
        self,
        route_table_id: str,
        destination_cidr: str,
        dry_run: bool = False,
    ) -> tuple[dict[str, Any], Callable | None]:
        """Create a blackhole route in a VPC route table."""
        self._check_emergency_stop()

        ec2 = self._get_ec2()

        if dry_run:
            return {
                "action": "route_blackhole",
                "route_table_id": route_table_id,
                "destination_cidr": destination_cidr,
                "dry_run": True,
            }, None

        # Save existing route for rollback
        tables = ec2.describe_route_tables(RouteTableIds=[route_table_id])
        original_route = None
        for route in tables["RouteTables"][0]["Routes"]:
            if route.get("DestinationCidrBlock") == destination_cidr:
                original_route = route
                break

        # Replace or create blackhole route
        ec2.create_route(
            RouteTableId=route_table_id,
            DestinationCidrBlock=destination_cidr,
            DryRun=False,
        )
        logger.info(
            "Created blackhole route: %s -> %s",
            route_table_id,
            destination_cidr,
        )

        async def rollback():
            ec2.delete_route(
                RouteTableId=route_table_id,
                DestinationCidrBlock=destination_cidr,
            )
            if original_route and original_route.get("GatewayId"):
                ec2.create_route(
                    RouteTableId=route_table_id,
                    DestinationCidrBlock=destination_cidr,
                    GatewayId=original_route["GatewayId"],
                )
            logger.info("Rollback: restored route %s", destination_cidr)
            return {"restored": destination_cidr}

        return {
            "action": "route_blackhole",
            "route_table_id": route_table_id,
            "destination_cidr": destination_cidr,
        }, rollback

    async def get_topology(self) -> InfraTopology:
        """Discover AWS resource topology."""
        ec2 = self._get_ec2()
        rds = self._get_rds()

        nodes = []
        edges = []

        # EC2 instances
        reservations = ec2.describe_instances()
        for res in reservations["Reservations"]:
            for inst in res["Instances"]:
                inst_id = inst["InstanceId"]
                tags = {t["Key"]: t["Value"] for t in inst.get("Tags", [])}
                state = inst["State"]["Name"]
                health = (
                    HealthStatus.HEALTHY
                    if state == "running"
                    else HealthStatus.UNHEALTHY
                    if state == "stopped"
                    else HealthStatus.UNKNOWN
                )
                nodes.append(
                    TopologyNode(
                        id=inst_id,
                        name=tags.get("Name", inst_id),
                        resource_type=ResourceType.EC2,
                        labels=tags,
                        health=health,
                        metadata={"state": state, "type": inst.get("InstanceType")},
                    )
                )

                # Link to VPC
                vpc_id = inst.get("VpcId")
                if vpc_id:
                    edges.append(
                        TopologyEdge(
                            source=vpc_id,
                            target=inst_id,
                            relation="contains",
                        )
                    )

        # RDS clusters
        clusters = rds.describe_db_clusters()
        for cluster in clusters["DBClusters"]:
            cluster_id = cluster["DBClusterIdentifier"]
            nodes.append(
                TopologyNode(
                    id=cluster_id,
                    name=cluster_id,
                    resource_type=ResourceType.RDS,
                    health=HealthStatus.HEALTHY
                    if cluster["Status"] == "available"
                    else HealthStatus.DEGRADED,
                    metadata={"engine": cluster["Engine"], "status": cluster["Status"]},
                )
            )

        return InfraTopology(nodes=nodes, edges=edges)
