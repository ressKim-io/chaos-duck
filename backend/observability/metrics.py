from prometheus_client import Counter, Gauge, Histogram

# Experiment lifecycle metrics
experiments_total = Counter(
    "chaosduck_experiments_total",
    "Total number of chaos experiments",
    ["chaos_type", "status"],
)

experiment_duration_seconds = Histogram(
    "chaosduck_experiment_duration_seconds",
    "Duration of chaos experiments in seconds",
    buckets=[1, 5, 10, 30, 60, 120],
)

active_experiments = Gauge(
    "chaosduck_active_experiments",
    "Number of currently running experiments",
)

# Probe metrics
probe_results_total = Counter(
    "chaosduck_probe_results",
    "Total probe execution results",
    ["probe_type", "passed"],
)

# Rollback metrics
rollback_total = Counter(
    "chaosduck_rollback_total",
    "Total number of rollbacks",
    ["status"],
)

# HTTP request metrics (populated by middleware)
http_requests_total = Counter(
    "chaosduck_http_requests_total",
    "Total HTTP requests",
    ["method", "path", "status_code"],
)

http_request_duration_seconds = Histogram(
    "chaosduck_http_request_duration_seconds",
    "HTTP request duration in seconds",
    ["method", "path"],
    buckets=[0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0],
)


class _Metrics:
    """Convenience wrapper for all metrics."""

    experiments_total = experiments_total
    experiment_duration_seconds = experiment_duration_seconds
    active_experiments = active_experiments
    probe_results_total = probe_results_total
    rollback_total = rollback_total
    http_requests_total = http_requests_total
    http_request_duration_seconds = http_request_duration_seconds

    def record_experiment_start(self):
        self.active_experiments.inc()

    def record_experiment_end(self, chaos_type: str, status: str, duration: float):
        self.active_experiments.dec()
        self.experiments_total.labels(chaos_type=chaos_type, status=status).inc()
        self.experiment_duration_seconds.observe(duration)

    def record_probe_result(self, probe_type: str, passed: bool):
        self.probe_results_total.labels(probe_type=probe_type, passed=str(passed)).inc()

    def record_rollback(self, status: str):
        self.rollback_total.labels(status=status).inc()


METRICS = _Metrics()
