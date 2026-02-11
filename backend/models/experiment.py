from datetime import datetime
from enum import Enum

from pydantic import BaseModel, Field


class ExperimentPhase(str, Enum):
    STEADY_STATE = "steady_state"
    HYPOTHESIS = "hypothesis"
    INJECT = "inject"
    OBSERVE = "observe"
    ROLLBACK = "rollback"


class ExperimentStatus(str, Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    ROLLED_BACK = "rolled_back"
    EMERGENCY_STOPPED = "emergency_stopped"


class ChaosType(str, Enum):
    # Kubernetes
    POD_DELETE = "pod_delete"
    NETWORK_LATENCY = "network_latency"
    NETWORK_LOSS = "network_loss"
    CPU_STRESS = "cpu_stress"
    MEMORY_STRESS = "memory_stress"
    # AWS
    EC2_STOP = "ec2_stop"
    RDS_FAILOVER = "rds_failover"
    ROUTE_BLACKHOLE = "route_blackhole"


class ProbeType(str, Enum):
    HTTP = "http"
    CMD = "cmd"
    K8S = "k8s"
    PROMETHEUS = "prometheus"


class ProbeMode(str, Enum):
    SOT = "sot"
    EOT = "eot"
    CONTINUOUS = "continuous"
    ON_CHAOS = "on_chaos"


class ProbeConfig(BaseModel):
    name: str
    type: ProbeType
    mode: ProbeMode
    properties: dict = Field(default_factory=dict)


class SafetyConfig(BaseModel):
    timeout_seconds: int = Field(default=30, ge=1, le=120)
    require_confirmation: bool = Field(default=False)
    max_blast_radius: float = Field(default=0.3, ge=0.0, le=1.0)
    dry_run: bool = Field(default=False)
    namespace_pattern: str | None = Field(default=None)
    health_check_interval: int = Field(default=10, ge=1, le=60)
    health_check_failure_threshold: int = Field(default=3, ge=1, le=10)


class ExperimentConfig(BaseModel):
    name: str
    chaos_type: ChaosType
    target_namespace: str | None = None
    target_labels: dict[str, str] | None = None
    target_resource: str | None = None
    parameters: dict = Field(default_factory=dict)
    safety: SafetyConfig = Field(default_factory=SafetyConfig)
    probes: list[ProbeConfig] = Field(default_factory=list)
    description: str | None = None
    ai_enabled: bool = Field(default=False)


class ExperimentResult(BaseModel):
    experiment_id: str
    config: ExperimentConfig
    status: ExperimentStatus = ExperimentStatus.PENDING
    phase: ExperimentPhase = ExperimentPhase.STEADY_STATE
    started_at: datetime | None = None
    completed_at: datetime | None = None
    steady_state: dict | None = None
    hypothesis: str | None = None
    injection_result: dict | None = None
    observations: dict | None = None
    rollback_result: dict | None = None
    error: str | None = None
    ai_insights: dict | None = None
