-- name: CreateTrade :one
INSERT INTO trades (
    entry_order_id,
    strategy_id,
    symbol,
    entry_price,
    quantity,
    side,
    entry_time,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetTrade :one
SELECT * FROM trades
WHERE id = $1;

-- name: ListTrades :many
SELECT * FROM trades
ORDER BY entry_time DESC
LIMIT $1 OFFSET $2;

-- name: ListTradesByStrategy :many
SELECT * FROM trades
WHERE strategy_id = $1
ORDER BY entry_time DESC;

-- name: ListOpenTrades :many
SELECT * FROM trades
WHERE exit_time IS NULL
ORDER BY entry_time DESC;

-- name: GetOpenTradesByStrategy :many
SELECT * FROM trades
WHERE strategy_id = $1 AND exit_time IS NULL
ORDER BY entry_time DESC;

-- name: CloseTrade :one
UPDATE trades
SET 
    exit_order_id = $2,
    exit_price = $3,
    exit_time = $4,
    pnl = $5,
    pnl_percent = $6,
    fees_total = $7,
    hold_duration = $8,
    exit_reason = $9
WHERE id = $1
RETURNING *;

-- name: GetTradeStats :one
SELECT 
    COUNT(*) as total_trades,
    COUNT(*) FILTER (WHERE exit_time IS NOT NULL) as closed_trades,
    COUNT(*) FILTER (WHERE pnl > 0) as winning_trades,
    COUNT(*) FILTER (WHERE pnl < 0) as losing_trades,
    SUM(pnl) as total_pnl,
    AVG(pnl) as average_pnl,
    MAX(pnl) as max_profit,
    MIN(pnl) as max_loss,
    AVG(pnl_percent) as average_return_percent,
    SUM(fees_total) as total_fees
FROM trades
WHERE strategy_id = $1;

-- name: GetDailyPnL :one
SELECT COALESCE(SUM(pnl), 0) as daily_pnl
FROM trades
WHERE strategy_id = $1 
AND entry_time >= $2;

-- name: GetTradesByDateRange :many
SELECT * FROM trades
WHERE entry_time >= $1 AND entry_time <= $2
ORDER BY entry_time DESC;

