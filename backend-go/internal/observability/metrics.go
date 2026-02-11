package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metric instruments
type Metrics struct {
	ExperimentsTotal          *prometheus.CounterVec
	ExperimentDurationSeconds prometheus.Histogram
	ActiveExperiments         prometheus.Gauge
	ProbeResultsTotal         *prometheus.CounterVec
	RollbackTotal             *prometheus.CounterVec
	HTTPRequestsTotal         *prometheus.CounterVec
	HTTPRequestDuration       *prometheus.HistogramVec
}

// NewMetrics registers and returns all metrics
func NewMetrics() *Metrics {
	return &Metrics{
		ExperimentsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "chaosduck_experiments_total",
			Help: "Total number of chaos experiments",
		}, []string{"chaos_type", "status"}),

		ExperimentDurationSeconds: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "chaosduck_experiment_duration_seconds",
			Help:    "Duration of chaos experiments in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120},
		}),

		ActiveExperiments: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "chaosduck_active_experiments",
			Help: "Number of currently running experiments",
		}),

		ProbeResultsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "chaosduck_probe_results",
			Help: "Total probe execution results",
		}, []string{"probe_type", "passed"}),

		RollbackTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "chaosduck_rollback_total",
			Help: "Total number of rollbacks",
		}, []string{"status"}),

		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "chaosduck_http_requests_total",
			Help: "Total HTTP requests",
		}, []string{"method", "path", "status_code"}),

		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "chaosduck_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0},
		}, []string{"method", "path"}),
	}
}

// RecordExperimentStart increments the active experiments gauge
func (m *Metrics) RecordExperimentStart() {
	m.ActiveExperiments.Inc()
}

// RecordExperimentEnd records experiment completion
func (m *Metrics) RecordExperimentEnd(chaosType, status string, duration float64) {
	m.ActiveExperiments.Dec()
	m.ExperimentsTotal.WithLabelValues(chaosType, status).Inc()
	m.ExperimentDurationSeconds.Observe(duration)
}

// RecordRollback records a rollback event
func (m *Metrics) RecordRollback(status string) {
	m.RollbackTotal.WithLabelValues(status).Inc()
}
