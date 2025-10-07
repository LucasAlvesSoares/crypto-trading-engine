-- name: InsertPriceData :one
INSERT INTO price_data (
    time,
    exchange,
    symbol,
    open,
    high,
    low,
    close,
    volume,
    interval
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) ON CONFLICT (time, exchange, symbol, interval) DO UPDATE
SET 
    open = EXCLUDED.open,
    high = EXCLUDED.high,
    low = EXCLUDED.low,
    close = EXCLUDED.close,
    volume = EXCLUDED.volume
RETURNING *;

-- name: GetLatestPrice :one
SELECT * FROM price_data
WHERE exchange = $1 AND symbol = $2 AND interval = $3
ORDER BY time DESC
LIMIT 1;

-- name: GetPriceDataByTimeRange :many
SELECT * FROM price_data
WHERE exchange = $1 
  AND symbol = $2 
  AND interval = $3
  AND time >= $4 
  AND time <= $5
ORDER BY time ASC;

-- name: GetOHLCVData :many
SELECT 
    time,
    open,
    high,
    low,
    close,
    volume
FROM price_data
WHERE exchange = $1 
  AND symbol = $2 
  AND interval = $3
  AND time >= $4
ORDER BY time ASC;

-- name: DeleteOldPriceData :exec
DELETE FROM price_data
WHERE time < $1;

