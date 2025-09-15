-- name: CreateStrategy :one
INSERT INTO strategies (
    name,
    type,
    config,
    is_active
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetStrategy :one
SELECT * FROM strategies
WHERE id = $1;

-- name: GetStrategyByName :one
SELECT * FROM strategies
WHERE name = $1;

-- name: ListStrategies :many
SELECT * FROM strategies
ORDER BY created_at DESC;

-- name: ListActiveStrategies :many
SELECT * FROM strategies
WHERE is_active = true
ORDER BY created_at DESC;

-- name: UpdateStrategy :one
UPDATE strategies
SET 
    name = COALESCE($2, name),
    config = COALESCE($3, config),
    is_active = COALESCE($4, is_active)
WHERE id = $1
RETURNING *;

-- name: ActivateStrategy :exec
UPDATE strategies
SET is_active = true
WHERE id = $1;

-- name: DeactivateStrategy :exec
UPDATE strategies
SET is_active = false
WHERE id = $1;

-- name: DeleteStrategy :exec
DELETE FROM strategies
WHERE id = $1;

