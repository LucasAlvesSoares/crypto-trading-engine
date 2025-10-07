-- name: CreateLog :one
INSERT INTO logs (
    level,
    component,
    message,
    metadata
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: ListLogs :many
SELECT * FROM logs
ORDER BY timestamp DESC
LIMIT $1 OFFSET $2;

-- name: ListLogsByLevel :many
SELECT * FROM logs
WHERE level = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;

-- name: ListLogsByComponent :many
SELECT * FROM logs
WHERE component = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;

-- name: GetRecentLogs :many
SELECT * FROM logs
WHERE timestamp >= $1
ORDER BY timestamp DESC;

-- name: DeleteOldLogs :exec
DELETE FROM logs
WHERE timestamp < $1;

