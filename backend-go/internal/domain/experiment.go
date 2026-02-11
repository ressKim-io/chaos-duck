package domain

import "time"

// Experiment lifecycle phases
type ExperimentPhase string

const (
	PhaseSteadyState ExperimentPhase = "steady_state"
	PhaseHypothesis  ExperimentPhase = "hypothesis"
	PhaseInject      ExperimentPhase = "inject"
	PhaseObserve     ExperimentPhase = "observe"
	PhaseRollback    ExperimentPhase = "rollback"
)

// Experiment status
type ExperimentStatus string

const (
	StatusPending          ExperimentStatus = "pending"
	StatusRunning          ExperimentStatus = "running"
	StatusCompleted        ExperimentStatus = "completed"
	StatusFailed           ExperimentStatus = "failed"
	StatusRolledBack       ExperimentStatus = "rolled_back"
	StatusEmergencyStopped ExperimentStatus = "emergency_stopped"
)

// Chaos injection types
type ChaosType string

const (
	// Kubernetes
	ChaosTypePodDelete      ChaosType = "pod_delete"
	ChaosTypeNetworkLatency ChaosType = "network_latency"
	ChaosTypeNetworkLoss    ChaosType = "network_loss"
	ChaosTypeCPUStress      ChaosType = "cpu_stress"
	ChaosTypeMemoryStress   ChaosType = "memory_stress"
	// AWS
	ChaosTypeEC2Stop        ChaosType = "ec2_stop"
	ChaosTypeRDSFailover    ChaosType = "rds_failover"
	ChaosTypeRouteBlackhole ChaosType = "route_blackhole"
)

// ProbeType identifies the probe implementation
type ProbeType string

const (
	ProbeTypeHTTP       ProbeType = "http"
	ProbeTypeCmd        ProbeType = "cmd"
	ProbeTypeK8s        ProbeType = "k8s"
	ProbeTypePrometheus ProbeType = "prometheus"
)

// ProbeMode defines when a probe executes during the experiment lifecycle
type ProbeMode string

const (
	ProbeModeSOT        ProbeMode = "sot"        // Start of Test
	ProbeModeEOT        ProbeMode = "eot"        // End of Test
	ProbeModeContinuous ProbeMode = "continuous"  // Polled during experiment
	ProbeModeOnChaos    ProbeMode = "on_chaos"    // After fault injection
)

// ProbeConfig defines probe settings within an experiment
type ProbeConfig struct {
	Name       string            `json:"name" binding:"required"`
	Type       ProbeType         `json:"type" binding:"required"`
	Mode       ProbeMode         `json:"mode" binding:"required"`
	Properties map[string]any    `json:"properties,omitempty"`
}

// SafetyConfig defines safety boundaries for an experiment
type SafetyConfig struct {
	TimeoutSeconds            int     `json:"timeout_seconds" binding:"min=1,max=120"`
	RequireConfirmation       bool    `json:"require_confirmation"`
	MaxBlastRadius            float64 `json:"max_blast_radius" binding:"min=0,max=1"`
	DryRun                    bool    `json:"dry_run"`
	NamespacePattern          *string `json:"namespace_pattern,omitempty"`
	HealthCheckInterval       int     `json:"health_check_interval" binding:"min=1,max=60"`
	HealthCheckFailureThreshold int   `json:"health_check_failure_threshold" binding:"min=1,max=10"`
}

// DefaultSafetyConfig returns safety config with safe defaults
func DefaultSafetyConfig() SafetyConfig {
	return SafetyConfig{
		TimeoutSeconds:              30,
		RequireConfirmation:         false,
		MaxBlastRadius:              0.3,
		DryRun:                      false,
		HealthCheckInterval:         10,
		HealthCheckFailureThreshold: 3,
	}
}

// ExperimentConfig defines a chaos experiment
type ExperimentConfig struct {
	Name            string            `json:"name" binding:"required"`
	ChaosType       ChaosType         `json:"chaos_type" binding:"required"`
	TargetNamespace *string           `json:"target_namespace,omitempty"`
	TargetLabels    map[string]string `json:"target_labels,omitempty"`
	TargetResource  *string           `json:"target_resource,omitempty"`
	Parameters      map[string]any    `json:"parameters,omitempty"`
	Safety          SafetyConfig      `json:"safety"`
	Probes          []ProbeConfig     `json:"probes,omitempty"`
	Description     *string           `json:"description,omitempty"`
	AIEnabled       bool              `json:"ai_enabled"`
}

// ExperimentResult holds the full experiment outcome
type ExperimentResult struct {
	ExperimentID    string           `json:"experiment_id"`
	Config          ExperimentConfig `json:"config"`
	Status          ExperimentStatus `json:"status"`
	Phase           ExperimentPhase  `json:"phase"`
	StartedAt       *time.Time       `json:"started_at,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	SteadyState     map[string]any   `json:"steady_state,omitempty"`
	Hypothesis      *string          `json:"hypothesis,omitempty"`
	InjectionResult map[string]any   `json:"injection_result,omitempty"`
	Observations    map[string]any   `json:"observations,omitempty"`
	RollbackResult  map[string]any   `json:"rollback_result,omitempty"`
	Error           *string          `json:"error,omitempty"`
	AIInsights      map[string]any   `json:"ai_insights,omitempty"`
}

// RollbackFunc is a function that undoes a chaos injection
type RollbackFunc func() (map[string]any, error)

// ChaosResult is returned by chaos engine methods: (result, rollbackFn)
type ChaosResult struct {
	Result     map[string]any
	RollbackFn RollbackFunc
}

// LabelSelectorString builds a comma-separated label selector
func LabelSelectorString(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	s := ""
	for k, v := range labels {
		if s != "" {
			s += ","
		}
		s += k + "=" + v
	}
	return s
}
