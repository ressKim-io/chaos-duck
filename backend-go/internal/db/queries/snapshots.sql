-- name: CreateSnapshot :one
INSERT INTO snapshots (experiment_id, type, namespace, data, captured_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSnapshotsByExperiment :many
SELECT * FROM snapshots WHERE experiment_id = $1 ORDER BY captured_at DESC;
