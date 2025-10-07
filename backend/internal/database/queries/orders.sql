-- name: CreateOrder :one
INSERT INTO orders (
    client_order_id,
    exchange_id,
    strategy_id,
    symbol,
    side,
    type,
    quantity,
    price,
    stop_loss_price,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetOrder :one
SELECT * FROM orders
WHERE id = $1;

-- name: GetOrderByClientOrderID :one
SELECT * FROM orders
WHERE client_order_id = $1;

-- name: GetOrderByExchangeOrderID :one
SELECT * FROM orders
WHERE exchange_order_id = $1;

-- name: ListOrders :many
SELECT * FROM orders
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListOrdersByStrategy :many
SELECT * FROM orders
WHERE strategy_id = $1
ORDER BY created_at DESC;

-- name: ListOrdersByStatus :many
SELECT * FROM orders
WHERE status = $1
ORDER BY created_at DESC;

-- name: ListOpenOrders :many
SELECT * FROM orders
WHERE status IN ('PENDING', 'OPEN')
ORDER BY created_at DESC;

-- name: UpdateOrderStatus :one
UPDATE orders
SET 
    status = $2,
    exchange_order_id = COALESCE($3, exchange_order_id),
    filled_quantity = COALESCE($4, filled_quantity),
    average_fill_price = COALESCE($5, average_fill_price),
    fees = COALESCE($6, fees),
    filled_at = CASE WHEN $2 = 'FILLED' THEN NOW() ELSE filled_at END
WHERE id = $1
RETURNING *;

-- name: UpdateOrder :one
UPDATE orders
SET 
    exchange_order_id = COALESCE($2, exchange_order_id),
    status = COALESCE($3, status),
    filled_quantity = COALESCE($4, filled_quantity),
    average_fill_price = COALESCE($5, average_fill_price),
    fees = COALESCE($6, fees)
WHERE id = $1
RETURNING *;

-- name: CancelOrder :one
UPDATE orders
SET status = 'CANCELLED'
WHERE id = $1
RETURNING *;

-- name: CancelAllOpenOrders :exec
UPDATE orders
SET status = 'CANCELLED'
WHERE status IN ('PENDING', 'OPEN');

-- name: GetOrderStats :one
SELECT 
    COUNT(*) as total_orders,
    COUNT(*) FILTER (WHERE status = 'FILLED') as filled_orders,
    COUNT(*) FILTER (WHERE status = 'CANCELLED') as cancelled_orders,
    COUNT(*) FILTER (WHERE status = 'FAILED') as failed_orders,
    SUM(filled_quantity * average_fill_price) FILTER (WHERE status = 'FILLED') as total_volume
FROM orders
WHERE strategy_id = $1;

