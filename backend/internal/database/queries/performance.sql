-- name: CreatePerformanceSnapshot :one
INSERT INTO performance_snapshots (
    strategy_id,
    portfolio_value,
    cash_balance,
    total_pnl,
    daily_pnl,
    open_positions,
    total_trades,
    win_rate,
    sharpe_ratio,
    max_drawdown
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetLatestPerformanceSnapshot :one
SELECT * FROM performance_snapshots
WHERE strategy_id = $1
ORDER BY timestamp DESC
LIMIT 1;

-- name: ListPerformanceSnapshots :many
SELECT * FROM performance_snapshots
WHERE strategy_id = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;

-- name: GetPerformanceHistory :many
SELECT * FROM performance_snapshots
WHERE strategy_id = $1 
  AND timestamp >= $2 
  AND timestamp <= $3
ORDER BY timestamp ASC;

