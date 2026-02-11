-- name: CreateAnalysisResult :one
INSERT INTO analysis_results (experiment_id, severity, root_cause, confidence, recommendations, resilience_score)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAnalysisResultsByExperiment :many
SELECT * FROM analysis_results WHERE experiment_id = $1 ORDER BY created_at DESC;

-- name: ListAnalysisResultsSince :many
SELECT * FROM analysis_results
WHERE created_at >= $1
ORDER BY created_at ASC;

-- name: ListAnalysisResultsSinceByNamespace :many
SELECT ar.* FROM analysis_results ar
JOIN experiments e ON ar.experiment_id = e.id
WHERE ar.created_at >= $1
  AND e.config->>'target_namespace' = $2
ORDER BY ar.created_at ASC;
