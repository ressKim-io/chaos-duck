package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelSelectorString(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "empty labels",
			labels:   nil,
			expected: "",
		},
		{
			name:     "single label",
			labels:   map[string]string{"app": "nginx"},
			expected: "app=nginx",
		},
		{
			name:     "empty map",
			labels:   map[string]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LabelSelectorString(tt.labels)
			if len(tt.labels) <= 1 {
				assert.Equal(t, tt.expected, result)
			} else {
				// Multiple labels: order not guaranteed, just check contains
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestLabelSelectorStringMultiple(t *testing.T) {
	labels := map[string]string{"app": "nginx", "env": "prod"}
	result := LabelSelectorString(labels)

	assert.Contains(t, result, "app=nginx")
	assert.Contains(t, result, "env=prod")
	assert.Contains(t, result, ",")
}

func TestDefaultSafetyConfig(t *testing.T) {
	cfg := DefaultSafetyConfig()

	assert.Equal(t, 30, cfg.TimeoutSeconds)
	assert.False(t, cfg.RequireConfirmation)
	assert.Equal(t, 0.3, cfg.MaxBlastRadius)
	assert.False(t, cfg.DryRun)
	assert.Equal(t, 10, cfg.HealthCheckInterval)
	assert.Equal(t, 3, cfg.HealthCheckFailureThreshold)
	assert.Nil(t, cfg.NamespacePattern)
}

func TestExperimentPhaseValues(t *testing.T) {
	assert.Equal(t, ExperimentPhase("steady_state"), PhaseSteadyState)
	assert.Equal(t, ExperimentPhase("hypothesis"), PhaseHypothesis)
	assert.Equal(t, ExperimentPhase("inject"), PhaseInject)
	assert.Equal(t, ExperimentPhase("observe"), PhaseObserve)
	assert.Equal(t, ExperimentPhase("rollback"), PhaseRollback)
}

func TestExperimentStatusValues(t *testing.T) {
	assert.Equal(t, ExperimentStatus("pending"), StatusPending)
	assert.Equal(t, ExperimentStatus("running"), StatusRunning)
	assert.Equal(t, ExperimentStatus("completed"), StatusCompleted)
	assert.Equal(t, ExperimentStatus("failed"), StatusFailed)
	assert.Equal(t, ExperimentStatus("rolled_back"), StatusRolledBack)
	assert.Equal(t, ExperimentStatus("emergency_stopped"), StatusEmergencyStopped)
}

func TestChaosTypeValues(t *testing.T) {
	k8sTypes := []ChaosType{
		ChaosTypePodDelete, ChaosTypeNetworkLatency, ChaosTypeNetworkLoss,
		ChaosTypeCPUStress, ChaosTypeMemoryStress,
	}
	awsTypes := []ChaosType{
		ChaosTypeEC2Stop, ChaosTypeRDSFailover, ChaosTypeRouteBlackhole,
	}

	assert.Len(t, k8sTypes, 5)
	assert.Len(t, awsTypes, 3)

	assert.Equal(t, ChaosType("pod_delete"), ChaosTypePodDelete)
	assert.Equal(t, ChaosType("ec2_stop"), ChaosTypeEC2Stop)
}

func TestProbeTypeAndMode(t *testing.T) {
	assert.Equal(t, ProbeType("http"), ProbeTypeHTTP)
	assert.Equal(t, ProbeType("cmd"), ProbeTypeCmd)
	assert.Equal(t, ProbeType("k8s"), ProbeTypeK8s)
	assert.Equal(t, ProbeType("prometheus"), ProbeTypePrometheus)

	assert.Equal(t, ProbeMode("sot"), ProbeModeSOT)
	assert.Equal(t, ProbeMode("eot"), ProbeModeEOT)
	assert.Equal(t, ProbeMode("continuous"), ProbeModeContinuous)
	assert.Equal(t, ProbeMode("on_chaos"), ProbeModeOnChaos)
}
