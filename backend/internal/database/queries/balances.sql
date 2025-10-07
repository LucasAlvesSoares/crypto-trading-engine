-- name: CreateBalance :one
INSERT INTO balances (
    exchange_id,
    currency,
    available,
    locked
) VALUES (
    $1, $2, $3, $4
) ON CONFLICT (exchange_id, currency) DO UPDATE
SET 
    available = EXCLUDED.available,
    locked = EXCLUDED.locked
RETURNING *;

-- name: GetBalance :one
SELECT * FROM balances
WHERE exchange_id = $1 AND currency = $2;

-- name: ListBalances :many
SELECT * FROM balances
WHERE exchange_id = $1
ORDER BY currency;

-- name: UpdateBalance :one
UPDATE balances
SET 
    available = $3,
    locked = $4
WHERE exchange_id = $1 AND currency = $2
RETURNING *;

-- name: LockBalance :one
UPDATE balances
SET 
    available = available - $3,
    locked = locked + $3
WHERE exchange_id = $1 AND currency = $2
RETURNING *;

-- name: UnlockBalance :one
UPDATE balances
SET 
    available = available + $3,
    locked = locked - $3
WHERE exchange_id = $1 AND currency = $2
RETURNING *;

-- name: GetTotalBalance :one
SELECT SUM(total) as total_balance
FROM balances
WHERE exchange_id = $1 AND currency = $2;

