-- name: CreateRiskEvent :one
INSERT INTO risk_events (
    strategy_id,
    event_type,
    description,
    action_taken,
    metadata
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetRiskEvent :one
SELECT * FROM risk_events
WHERE id = $1;

-- name: ListRiskEvents :many
SELECT * FROM risk_events
ORDER BY timestamp DESC
LIMIT $1 OFFSET $2;

-- name: ListRiskEventsByStrategy :many
SELECT * FROM risk_events
WHERE strategy_id = $1
ORDER BY timestamp DESC;

-- name: ListRiskEventsByType :many
SELECT * FROM risk_events
WHERE event_type = $1
ORDER BY timestamp DESC;

-- name: GetRecentRiskEvents :many
SELECT * FROM risk_events
WHERE timestamp >= $1
ORDER BY timestamp DESC;

