package order

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/crypto-trading-bot/internal/events"
	"github.com/crypto-trading-bot/internal/exchange"
	"github.com/crypto-trading-bot/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// OrderManager handles order placement and tracking
type OrderManager struct {
	db       *sql.DB
	exchange exchange.Exchange
	nats     *events.NATSClient
	logger   *logrus.Entry
}

// NewOrderManager creates a new order manager
func NewOrderManager(
	db *sql.DB,
	exch exchange.Exchange,
	natsClient *events.NATSClient,
	logger *logrus.Logger,
) *OrderManager {
	return &OrderManager{
		db:       db,
		exchange: exch,
		nats:     natsClient,
		logger:   logger.WithField("component", "order-manager"),
	}
}

// PlaceOrder places an order with idempotency guarantee
func (om *OrderManager) PlaceOrder(ctx context.Context, signal *events.TradeSignalEvent) error {
	// Generate deterministic client order ID for idempotency
	clientOrderID := om.generateClientOrderID(signal)

	// Check if order already exists
	var existingOrderID string
	err := om.db.QueryRowContext(ctx, `
		SELECT id FROM orders WHERE client_order_id = $1
	`, clientOrderID).Scan(&existingOrderID)

	if err == nil {
		// Order already exists
		om.logger.WithFields(logrus.Fields{
			"client_order_id": clientOrderID,
			"order_id":        existingOrderID,
		}).Info("Order already exists (idempotent)")
		return nil
	}

	if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing order: %w", err)
	}

	// Parse strategy ID
	strategyID, err := uuid.Parse(signal.StrategyID)
	if err != nil {
		return fmt.Errorf("invalid strategy ID: %w", err)
	}

	// Start transaction
	tx, err := om.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get exchange ID (first active exchange for now)
	var exchangeID uuid.UUID
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM exchanges WHERE is_active = true LIMIT 1
	`).Scan(&exchangeID)
	if err != nil {
		return fmt.Errorf("failed to get exchange ID: %w", err)
	}

	// Insert order with PENDING status
	orderID := uuid.New()
	quantity := decimal.NewFromFloat(signal.Quantity)
	var price *decimal.Decimal
	if signal.Price != nil {
		p := decimal.NewFromFloat(*signal.Price)
		price = &p
	}

	var stopLossPrice *decimal.Decimal
	if signal.StopLossPrice > 0 {
		slp := decimal.NewFromFloat(signal.StopLossPrice)
		stopLossPrice = &slp
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders (
			id, client_order_id, exchange_id, strategy_id, symbol, side, type,
			quantity, price, stop_loss_price, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'PENDING')
	`, orderID, clientOrderID, exchangeID, strategyID, signal.Symbol,
		signal.Side, signal.Type, quantity, price, stopLossPrice)

	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	om.logger.WithFields(logrus.Fields{
		"order_id":        orderID,
		"client_order_id": clientOrderID,
		"symbol":          signal.Symbol,
		"side":            signal.Side,
		"quantity":        quantity.String(),
	}).Info("Order created with PENDING status")

	// Place order on exchange
	go om.executeOrder(context.Background(), orderID, signal)

	return nil
}

// executeOrder executes the order on the exchange (async)
func (om *OrderManager) executeOrder(ctx context.Context, orderID uuid.UUID, signal *events.TradeSignalEvent) {
	// Get order details from database
	var order struct {
		ClientOrderID string
		Symbol        string
		Side          string
		Type          string
		Quantity      decimal.Decimal
		Price         sql.NullString
		StopLossPrice sql.NullString
	}

	err := om.db.QueryRowContext(ctx, `
		SELECT client_order_id, symbol, side, type, quantity, price, stop_loss_price
		FROM orders WHERE id = $1
	`, orderID).Scan(
		&order.ClientOrderID,
		&order.Symbol,
		&order.Side,
		&order.Type,
		&order.Quantity,
		&order.Price,
		&order.StopLossPrice,
	)

	if err != nil {
		om.logger.WithError(err).Error("Failed to get order details")
		om.updateOrderStatus(ctx, orderID, models.OrderStatusFailed, "", decimal.Zero, decimal.Zero, decimal.Zero)
		return
	}

	// Build exchange order request
	req := &exchange.OrderRequest{
		Symbol:   order.Symbol,
		Side:     models.OrderSide(order.Side),
		Type:     models.OrderType(order.Type),
		Quantity: order.Quantity,
	}

	if order.Price.Valid {
		price, _ := decimal.NewFromString(order.Price.String)
		req.Price = &price
	}

	if order.StopLossPrice.Valid {
		stopLoss, _ := decimal.NewFromString(order.StopLossPrice.String)
		req.StopLossPrice = &stopLoss
	}

	// Place order on exchange
	resp, err := om.exchange.PlaceOrder(ctx, req)
	if err != nil {
		om.logger.WithError(err).WithField("order_id", orderID).Error("Failed to place order on exchange")
		om.updateOrderStatus(ctx, orderID, models.OrderStatusFailed, "", decimal.Zero, decimal.Zero, decimal.Zero)

		// Publish failed event
		om.publishOrderEvent(events.EventTypeOrderFailed, orderID, order.ClientOrderID, "", signal)
		return
	}

	// Update order with exchange order ID and status
	om.updateOrderStatus(
		ctx,
		orderID,
		resp.Status,
		resp.ExchangeOrderID,
		resp.FilledQuantity,
		resp.AverageFillPrice,
		&resp.Fees,
	)

	om.logger.WithFields(logrus.Fields{
		"order_id":          orderID,
		"exchange_order_id": resp.ExchangeOrderID,
		"status":            resp.Status,
		"filled_quantity":   resp.FilledQuantity.String(),
	}).Info("Order placed on exchange")

	// Publish order placed event
	om.publishOrderEvent(events.EventTypeOrderPlaced, orderID, order.ClientOrderID, resp.ExchangeOrderID, signal)

	// If order is filled immediately (market order), handle it
	if resp.Status == models.OrderStatusFilled {
		om.handleFilledOrder(ctx, orderID, resp)
	}
}

// updateOrderStatus updates the order status in the database
func (om *OrderManager) updateOrderStatus(
	ctx context.Context,
	orderID uuid.UUID,
	status models.OrderStatus,
	exchangeOrderID string,
	filledQuantity decimal.Decimal,
	avgFillPrice *decimal.Decimal,
	fees *decimal.Decimal,
) error {
	query := `
		UPDATE orders
		SET status = $2,
		    exchange_order_id = COALESCE(NULLIF($3, ''), exchange_order_id),
		    filled_quantity = COALESCE($4, filled_quantity),
		    average_fill_price = COALESCE($5, average_fill_price),
		    fees = COALESCE($6, fees),
		    filled_at = CASE WHEN $2 = 'FILLED' THEN NOW() ELSE filled_at END,
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := om.db.ExecContext(ctx, query, orderID, status, exchangeOrderID, filledQuantity, avgFillPrice, fees)
	if err != nil {
		om.logger.WithError(err).Error("Failed to update order status")
		return err
	}

	return nil
}

// handleFilledOrder handles a filled order (creates trade record)
func (om *OrderManager) handleFilledOrder(ctx context.Context, orderID uuid.UUID, resp *exchange.OrderResponse) {
	// Get order details
	var order struct {
		StrategyID       uuid.UUID
		Symbol           string
		Side             string
		Quantity         decimal.Decimal
		AverageFillPrice decimal.Decimal
		Fees             decimal.Decimal
	}

	err := om.db.QueryRowContext(ctx, `
		SELECT strategy_id, symbol, side, quantity, average_fill_price, fees
		FROM orders WHERE id = $1
	`, orderID).Scan(
		&order.StrategyID,
		&order.Symbol,
		&order.Side,
		&order.Quantity,
		&order.AverageFillPrice,
		&order.Fees,
	)

	if err != nil {
		om.logger.WithError(err).Error("Failed to get order details for filled order")
		return
	}

	// Check if this is opening or closing a trade
	if order.Side == string(models.OrderSideBuy) {
		// Opening a LONG position
		om.createTrade(ctx, orderID, order.StrategyID, order.Symbol, order.AverageFillPrice, order.Quantity, models.TradeSideLong)
	} else {
		// Closing a position
		om.closeTrade(ctx, orderID, order.StrategyID, order.Symbol, order.AverageFillPrice, order.Fees)
	}

	// Publish order filled event
	filledEvent := &events.OrderFilledEvent{
		OrderID:          orderID.String(),
		ClientOrderID:    resp.ClientOrderID,
		ExchangeOrderID:  resp.ExchangeOrderID,
		Symbol:           order.Symbol,
		Side:             order.Side,
		FilledQuantity:   order.Quantity.InexactFloat64(),
		AverageFillPrice: order.AverageFillPrice.InexactFloat64(),
		Fees:             order.Fees.InexactFloat64(),
		FilledAt:         time.Now(),
	}

	if err := om.nats.Publish(events.EventTypeOrderFilled, filledEvent); err != nil {
		om.logger.WithError(err).Error("Failed to publish order filled event")
	}
}

// createTrade creates a new trade record
func (om *OrderManager) createTrade(
	ctx context.Context,
	orderID uuid.UUID,
	strategyID uuid.UUID,
	symbol string,
	entryPrice decimal.Decimal,
	quantity decimal.Decimal,
	side models.TradeSide,
) {
	tradeID := uuid.New()
	metadata := map[string]interface{}{
		"entry_order_id": orderID.String(),
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err := om.db.ExecContext(ctx, `
		INSERT INTO trades (
			id, entry_order_id, strategy_id, symbol, entry_price, quantity, side, entry_time, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8)
	`, tradeID, orderID, strategyID, symbol, entryPrice, quantity, side, metadataJSON)

	if err != nil {
		om.logger.WithError(err).Error("Failed to create trade")
		return
	}

	om.logger.WithFields(logrus.Fields{
		"trade_id":    tradeID,
		"strategy_id": strategyID,
		"symbol":      symbol,
		"side":        side,
		"entry_price": entryPrice.String(),
		"quantity":    quantity.String(),
	}).Info("Trade opened")

	// Publish trade opened event
	tradeEvent := &events.TradeOpenedEvent{
		TradeID:    tradeID.String(),
		StrategyID: strategyID.String(),
		Symbol:     symbol,
		Side:       string(side),
		EntryPrice: entryPrice.InexactFloat64(),
		Quantity:   quantity.InexactFloat64(),
		EntryTime:  time.Now(),
	}

	if err := om.nats.Publish(events.EventTypeTradeOpened, tradeEvent); err != nil {
		om.logger.WithError(err).Error("Failed to publish trade opened event")
	}
}

// closeTrade closes an open trade
func (om *OrderManager) closeTrade(
	ctx context.Context,
	exitOrderID uuid.UUID,
	strategyID uuid.UUID,
	symbol string,
	exitPrice decimal.Decimal,
	exitFees decimal.Decimal,
) {
	// Get open trade
	var trade struct {
		ID         uuid.UUID
		EntryPrice decimal.Decimal
		Quantity   decimal.Decimal
		Side       string
		EntryTime  time.Time
		EntryFees  decimal.Decimal
	}

	err := om.db.QueryRowContext(ctx, `
		SELECT t.id, t.entry_price, t.quantity, t.side, t.entry_time, COALESCE(o.fees, 0) as entry_fees
		FROM trades t
		LEFT JOIN orders o ON t.entry_order_id = o.id
		WHERE t.strategy_id = $1 AND t.symbol = $2 AND t.exit_time IS NULL
		ORDER BY t.entry_time DESC
		LIMIT 1
	`, strategyID, symbol).Scan(
		&trade.ID,
		&trade.EntryPrice,
		&trade.Quantity,
		&trade.Side,
		&trade.EntryTime,
		&trade.EntryFees,
	)

	if err != nil {
		om.logger.WithError(err).Error("Failed to get open trade for closing")
		return
	}

	// Calculate P&L
	totalFees := trade.EntryFees.Add(exitFees)
	var pnl decimal.Decimal

	if trade.Side == string(models.TradeSideLong) {
		// Long: P&L = (exit_price - entry_price) * quantity - fees
		pnl = exitPrice.Sub(trade.EntryPrice).Mul(trade.Quantity).Sub(totalFees)
	} else {
		// Short: P&L = (entry_price - exit_price) * quantity - fees
		pnl = trade.EntryPrice.Sub(exitPrice).Mul(trade.Quantity).Sub(totalFees)
	}

	pnlPercent := pnl.Div(trade.EntryPrice.Mul(trade.Quantity)).Mul(decimal.NewFromInt(100))
	holdDuration := time.Since(trade.EntryTime)

	// Update trade
	_, err = om.db.ExecContext(ctx, `
		UPDATE trades
		SET exit_order_id = $2,
		    exit_price = $3,
		    exit_time = NOW(),
		    pnl = $4,
		    pnl_percent = $5,
		    fees_total = $6,
		    hold_duration = $7,
		    exit_reason = 'SIGNAL'
		WHERE id = $1
	`, trade.ID, exitOrderID, exitPrice, pnl, pnlPercent, totalFees, holdDuration)

	if err != nil {
		om.logger.WithError(err).Error("Failed to update closed trade")
		return
	}

	om.logger.WithFields(logrus.Fields{
		"trade_id":      trade.ID,
		"entry_price":   trade.EntryPrice.String(),
		"exit_price":    exitPrice.String(),
		"pnl":           pnl.String(),
		"pnl_percent":   pnlPercent.String(),
		"hold_duration": holdDuration,
	}).Info("Trade closed")

	// Publish trade closed event
	tradeEvent := &events.TradeClosedEvent{
		TradeID:      trade.ID.String(),
		StrategyID:   strategyID.String(),
		Symbol:       symbol,
		EntryPrice:   trade.EntryPrice.InexactFloat64(),
		ExitPrice:    exitPrice.InexactFloat64(),
		Quantity:     trade.Quantity.InexactFloat64(),
		PnL:          pnl.InexactFloat64(),
		PnLPercent:   pnlPercent.InexactFloat64(),
		ExitReason:   "SIGNAL",
		ExitTime:     time.Now(),
		HoldDuration: holdDuration.String(),
	}

	if err := om.nats.Publish(events.EventTypeTradeClosed, tradeEvent); err != nil {
		om.logger.WithError(err).Error("Failed to publish trade closed event")
	}
}

// Helper methods

func (om *OrderManager) generateClientOrderID(signal *events.TradeSignalEvent) string {
	// Create deterministic ID from signal properties
	data := fmt.Sprintf("%s-%s-%s-%f-%d",
		signal.StrategyID,
		signal.Symbol,
		signal.Side,
		signal.Quantity,
		signal.Indicators["price"],
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes
}

func (om *OrderManager) publishOrderEvent(
	eventType events.EventType,
	orderID uuid.UUID,
	clientOrderID string,
	exchangeOrderID string,
	signal *events.TradeSignalEvent,
) {
	event := &events.OrderPlacedEvent{
		OrderID:         orderID.String(),
		ClientOrderID:   clientOrderID,
		ExchangeOrderID: exchangeOrderID,
		StrategyID:      signal.StrategyID,
		Symbol:          signal.Symbol,
		Side:            signal.Side,
		Type:            signal.Type,
		Quantity:        signal.Quantity,
		Price:           signal.Price,
		StopLossPrice:   &signal.StopLossPrice,
	}

	if err := om.nats.Publish(eventType, event); err != nil {
		om.logger.WithError(err).WithField("event_type", eventType).Error("Failed to publish order event")
	}
}
