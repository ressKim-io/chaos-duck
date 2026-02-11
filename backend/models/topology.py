from enum import Enum
from typing import Optional
from pydantic import BaseModel, Field


class ResourceType(str, Enum):
    POD = "pod"
    SERVICE = "service"
    DEPLOYMENT = "deployment"
    NODE = "node"
    NAMESPACE = "namespace"
    EC2 = "ec2"
    RDS = "rds"
    VPC = "vpc"
    SUBNET = "subnet"


class HealthStatus(str, Enum):
    HEALTHY = "healthy"
    DEGRADED = "degraded"
    UNHEALTHY = "unhealthy"
    UNKNOWN = "unknown"


class TopologyNode(BaseModel):
    id: str
    name: str
    resource_type: ResourceType
    namespace: Optional[str] = None
    labels: dict[str, str] = Field(default_factory=dict)
    health: HealthStatus = HealthStatus.UNKNOWN
    metadata: dict = Field(default_factory=dict)


class TopologyEdge(BaseModel):
    source: str
    target: str
    relation: str = "connects_to"
    metadata: dict = Field(default_factory=dict)


class InfraTopology(BaseModel):
    nodes: list[TopologyNode] = Field(default_factory=list)
    edges: list[TopologyEdge] = Field(default_factory=list)
    timestamp: Optional[str] = None


class ResilienceScore(BaseModel):
    overall: float = Field(ge=0.0, le=100.0)
    categories: dict[str, float] = Field(default_factory=dict)
    recommendations: list[str] = Field(default_factory=list)
    details: Optional[str] = None
