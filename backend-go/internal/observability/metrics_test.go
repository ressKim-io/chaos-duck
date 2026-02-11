package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/stretchr/testify/assert"
)

func newTestMetrics(reg *prometheus.Registry) *Metrics {
	f := promauto.With(reg)
	return &Metrics{
		ExperimentsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "chaosduck_experiments_total",
			Help: "Total number of chaos experiments",
		}, []string{"chaos_type", "status"}),

		ExperimentDurationSeconds: f.NewHistogram(prometheus.HistogramOpts{
			Name:    "chaosduck_experiment_duration_seconds",
			Help:    "Duration of chaos experiments in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120},
		}),

		ActiveExperiments: f.NewGauge(prometheus.GaugeOpts{
			Name: "chaosduck_active_experiments",
			Help: "Number of currently running experiments",
		}),

		ProbeResultsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "chaosduck_probe_results",
			Help: "Total probe execution results",
		}, []string{"probe_type", "passed"}),

		RollbackTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "chaosduck_rollback_total",
			Help: "Total number of rollbacks",
		}, []string{"status"}),

		HTTPRequestsTotal: f.NewCounterVec(prometheus.CounterOpts{
			Name: "chaosduck_http_requests_total",
			Help: "Total HTTP requests",
		}, []string{"method", "path", "status_code"}),

		HTTPRequestDuration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "chaosduck_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0},
		}, []string{"method", "path"}),
	}
}

func TestNewMetricsFields(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newTestMetrics(reg)

	assert.NotNil(t, m.ExperimentsTotal)
	assert.NotNil(t, m.ExperimentDurationSeconds)
	assert.NotNil(t, m.ActiveExperiments)
	assert.NotNil(t, m.ProbeResultsTotal)
	assert.NotNil(t, m.RollbackTotal)
	assert.NotNil(t, m.HTTPRequestsTotal)
	assert.NotNil(t, m.HTTPRequestDuration)
}

func TestRecordExperimentLifecycle(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newTestMetrics(reg)

	// Should not panic
	m.RecordExperimentStart()
	m.RecordExperimentEnd("pod_delete", "completed", 5.0)
}

func TestRecordRollback(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newTestMetrics(reg)

	// Should not panic
	m.RecordRollback("success")
	m.RecordRollback("failed")
}
