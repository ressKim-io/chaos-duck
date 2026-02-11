-- name: GetExperiment :one
SELECT * FROM experiments WHERE id = $1;

-- name: ListExperiments :many
SELECT * FROM experiments ORDER BY started_at DESC;

-- name: CreateExperiment :one
INSERT INTO experiments (id, config, status, phase, started_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateExperiment :exec
UPDATE experiments
SET status = $2,
    phase = $3,
    completed_at = $4,
    steady_state = $5,
    hypothesis = $6,
    injection_result = $7,
    observations = $8,
    rollback_result = $9,
    error = $10,
    ai_insights = $11
WHERE id = $1;

-- name: UpdateExperimentStatus :exec
UPDATE experiments SET status = $2 WHERE id = $1;
