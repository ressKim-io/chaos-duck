from fastapi import APIRouter

from engines.aws_engine import AwsEngine
from engines.k8s_engine import K8sEngine
from models.topology import InfraTopology

router = APIRouter()
k8s_engine = K8sEngine()
aws_engine = AwsEngine()


@router.get("/k8s", response_model=InfraTopology)
async def get_k8s_topology(namespace: str = "default"):
    """Get Kubernetes resource topology."""
    return await k8s_engine.get_topology(namespace)


@router.get("/aws", response_model=InfraTopology)
async def get_aws_topology():
    """Get AWS resource topology."""
    return await aws_engine.get_topology()


@router.get("/combined", response_model=InfraTopology)
async def get_combined_topology(namespace: str = "default"):
    """Get combined K8s + AWS topology."""
    k8s_topo = await k8s_engine.get_topology(namespace)
    aws_topo = await aws_engine.get_topology()

    return InfraTopology(
        nodes=k8s_topo.nodes + aws_topo.nodes,
        edges=k8s_topo.edges + aws_topo.edges,
    )


@router.get("/steady-state")
async def get_steady_state(namespace: str = "default"):
    """Get current steady state metrics."""
    return await k8s_engine.get_steady_state(namespace)
