CREATE TABLE IF NOT EXISTS experiments (
    id VARCHAR(8) PRIMARY KEY,
    config JSONB NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    phase VARCHAR(30) NOT NULL DEFAULT 'steady_state',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    steady_state JSONB,
    hypothesis TEXT,
    injection_result JSONB,
    observations JSONB,
    rollback_result JSONB,
    error TEXT,
    ai_insights JSONB
);

CREATE TABLE IF NOT EXISTS snapshots (
    id SERIAL PRIMARY KEY,
    experiment_id VARCHAR(8) NOT NULL,
    type VARCHAR(10) NOT NULL,
    namespace VARCHAR(255),
    data JSONB NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_snapshots_experiment_id ON snapshots(experiment_id);

CREATE TABLE IF NOT EXISTS probe_results (
    id SERIAL PRIMARY KEY,
    experiment_id VARCHAR(8) NOT NULL,
    probe_type VARCHAR(30) NOT NULL,
    mode VARCHAR(20) NOT NULL,
    result JSONB NOT NULL,
    passed BOOLEAN NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_probe_results_experiment_id ON probe_results(experiment_id);

CREATE TABLE IF NOT EXISTS analysis_results (
    id SERIAL PRIMARY KEY,
    experiment_id VARCHAR(8) NOT NULL,
    severity VARCHAR(10) NOT NULL,
    root_cause TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    recommendations JSONB NOT NULL DEFAULT '[]',
    resilience_score DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_analysis_results_experiment_id ON analysis_results(experiment_id);
