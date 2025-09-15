-- name: CreateExchange :one
INSERT INTO exchanges (
    name,
    api_key_encrypted,
    api_secret_encrypted,
    api_passphrase_encrypted,
    is_paper_trading,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetExchange :one
SELECT * FROM exchanges
WHERE id = $1;

-- name: GetActiveExchanges :many
SELECT * FROM exchanges
WHERE is_active = true
ORDER BY created_at DESC;

-- name: UpdateExchange :one
UPDATE exchanges
SET 
    name = COALESCE($2, name),
    api_key_encrypted = COALESCE($3, api_key_encrypted),
    api_secret_encrypted = COALESCE($4, api_secret_encrypted),
    is_paper_trading = COALESCE($5, is_paper_trading),
    is_active = COALESCE($6, is_active)
WHERE id = $1
RETURNING *;

-- name: DeleteExchange :exec
DELETE FROM exchanges
WHERE id = $1;

