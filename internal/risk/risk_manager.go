package risk

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/crypto-trading-bot/internal/config"
	"github.com/crypto-trading-bot/internal/events"
	"github.com/crypto-trading-bot/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// RiskManager handles risk validation and kill switch functionality
type RiskManager struct {
	config     *config.RiskConfig
	db         *sql.DB
	nats       *events.NATSClient
	logger     *logrus.Entry
	killSwitch *KillSwitch
}

// KillSwitch manages the emergency stop functionality
type KillSwitch struct {
	enabled   bool
	reason    string
	timestamp time.Time
}

// NewRiskManager creates a new risk manager
func NewRiskManager(cfg *config.RiskConfig, db *sql.DB, natsClient *events.NATSClient, logger *logrus.Logger) *RiskManager {
	return &RiskManager{
		config: cfg,
		db:     db,
		nats:   natsClient,
		logger: logger.WithField("component", "risk-manager"),
		killSwitch: &KillSwitch{
			enabled: false,
		},
	}
}

// ValidateTradeSignal validates a trade signal against risk parameters
func (rm *RiskManager) ValidateTradeSignal(ctx context.Context, signal *models.TradeSignal) error {
	// Check kill switch first
	if rm.killSwitch.enabled {
		rm.logger.Warn("Trade rejected: kill switch is enabled")
		return fmt.Errorf("kill switch is enabled")
	}

	// Check daily loss limit
	if err := rm.checkDailyLossLimit(ctx, signal.StrategyID); err != nil {
		rm.logRiskEvent(ctx, signal.StrategyID, "DAILY_LOSS_LIMIT", err.Error(), "Trade rejected")
		return err
	}

	// Check max open positions
	if err := rm.checkMaxOpenPositions(ctx, signal.StrategyID); err != nil {
		rm.logRiskEvent(ctx, signal.StrategyID, "MAX_POSITIONS", err.Error(), "Trade rejected")
		return err
	}

	// Validate position size
	positionValue := signal.Quantity.Mul(decimal.NewFromFloat(signal.Indicators["price"]))
	if positionValue.GreaterThan(decimal.NewFromFloat(rm.config.MaxPositionSizeUSD)) {
		err := fmt.Errorf("position size %.2f exceeds limit %.2f",
			positionValue.InexactFloat64(), rm.config.MaxPositionSizeUSD)
		rm.logRiskEvent(ctx, signal.StrategyID, "POSITION_SIZE", err.Error(), "Trade rejected")
		return err
	}

	// Validate stop-loss is set
	if signal.StopLossPrice.IsZero() {
		err := fmt.Errorf("stop-loss price is required")
		rm.logRiskEvent(ctx, signal.StrategyID, "STOP_LOSS_MISSING", err.Error(), "Trade rejected")
		return err
	}

	// Validate stop-loss percentage
	entryPrice := decimal.NewFromFloat(signal.Indicators["price"])
	stopLossDiff := entryPrice.Sub(signal.StopLossPrice).Abs()
	stopLossPercent := stopLossDiff.Div(entryPrice).Mul(decimal.NewFromInt(100))

	if stopLossPercent.GreaterThan(decimal.NewFromFloat(rm.config.StopLossPercent * 2)) {
		err := fmt.Errorf("stop-loss %.2f%% is too wide (max %.2f%%)",
			stopLossPercent.InexactFloat64(), rm.config.StopLossPercent*2)
		rm.logRiskEvent(ctx, signal.StrategyID, "STOP_LOSS_TOO_WIDE", err.Error(), "Trade rejected")
		return err
	}

	rm.logger.WithFields(logrus.Fields{
		"strategy_id":    signal.StrategyID,
		"symbol":         signal.Symbol,
		"position_value": positionValue.String(),
		"stop_loss":      signal.StopLossPrice.String(),
	}).Info("Trade signal validated")

	return nil
}

// CheckOpenTrades checks if open trades need to be closed (stop-loss, timeout, etc.)
func (rm *RiskManager) CheckOpenTrades(ctx context.Context) error {
	// Get all open trades
	rows, err := rm.db.QueryContext(ctx, `
		SELECT id, strategy_id, symbol, entry_price, quantity, side, entry_time, metadata
		FROM trades
		WHERE exit_time IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to get open trades: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var trade models.Trade
		var metadataJSON []byte

		err := rows.Scan(
			&trade.ID,
			&trade.StrategyID,
			&trade.Symbol,
			&trade.EntryPrice,
			&trade.Quantity,
			&trade.Side,
			&trade.EntryTime,
			&metadataJSON,
		)
		if err != nil {
			rm.logger.WithError(err).Error("Failed to scan trade")
			continue
		}

		// Parse metadata
		if err := json.Unmarshal(metadataJSON, &trade.Metadata); err != nil {
			rm.logger.WithError(err).Error("Failed to unmarshal metadata")
			continue
		}

		// Get current price (would come from market data service)
		// For now, skip price check - this will be implemented in market data integration

		// Check max hold time
		holdDuration := time.Since(trade.EntryTime)
		maxHoldDuration := time.Duration(rm.config.MaxHoldTimeHours) * time.Hour

		if holdDuration > maxHoldDuration {
			rm.logger.WithFields(logrus.Fields{
				"trade_id":      trade.ID,
				"hold_duration": holdDuration,
				"max_duration":  maxHoldDuration,
			}).Warn("Trade exceeded max hold time")

			// Publish event to close trade
			closeSignal := &events.TradeSignalEvent{
				ID:         uuid.New().String(),
				StrategyID: trade.StrategyID.String(),
				Symbol:     trade.Symbol,
				Side:       oppositeOrderSide(trade.Side),
				Type:       "MARKET",
				Quantity:   trade.Quantity.InexactFloat64(),
			}

			if err := rm.nats.Publish(events.EventTypeTradeSignal, closeSignal); err != nil {
				rm.logger.WithError(err).Error("Failed to publish close signal")
			}

			rm.logRiskEvent(ctx, trade.StrategyID, "MAX_HOLD_TIME",
				fmt.Sprintf("Trade held for %s", holdDuration), "Closing trade")
		}
	}

	return rows.Err()
}

// EnableKillSwitch enables the emergency kill switch
func (rm *RiskManager) EnableKillSwitch(ctx context.Context, reason string) error {
	rm.killSwitch.enabled = true
	rm.killSwitch.reason = reason
	rm.killSwitch.timestamp = time.Now()

	// Update database
	_, err := rm.db.ExecContext(ctx, `
		UPDATE system_config
		SET value = jsonb_build_object(
			'enabled', true,
			'reason', $1,
			'timestamp', to_jsonb($2)
		)
		WHERE key = 'kill_switch'
	`, reason, time.Now())
	if err != nil {
		return fmt.Errorf("failed to enable kill switch in database: %w", err)
	}

	// Cancel all open orders
	if _, err := rm.db.ExecContext(ctx, `
		UPDATE orders SET status = 'CANCELLED' WHERE status IN ('PENDING', 'OPEN')
	`); err != nil {
		rm.logger.WithError(err).Error("Failed to cancel open orders")
	}

	// Publish kill switch event
	killSwitchEvent := &events.KillSwitchEvent{
		Enabled: true,
		Reason:  reason,
	}
	if err := rm.nats.Publish(events.EventTypeKillSwitch, killSwitchEvent); err != nil {
		rm.logger.WithError(err).Error("Failed to publish kill switch event")
	}

	// Log risk event
	rm.logRiskEvent(ctx, uuid.Nil, "KILL_SWITCH", reason, "All trading halted")

	rm.logger.WithField("reason", reason).Warn("KILL SWITCH ENABLED")

	return nil
}

// DisableKillSwitch disables the kill switch
func (rm *RiskManager) DisableKillSwitch(ctx context.Context) error {
	rm.killSwitch.enabled = false
	rm.killSwitch.reason = ""

	// Update database
	_, err := rm.db.ExecContext(ctx, `
		UPDATE system_config
		SET value = jsonb_build_object(
			'enabled', false,
			'reason', null,
			'timestamp', to_jsonb($1)
		)
		WHERE key = 'kill_switch'
	`, time.Now())
	if err != nil {
		return fmt.Errorf("failed to disable kill switch in database: %w", err)
	}

	// Publish kill switch event
	killSwitchEvent := &events.KillSwitchEvent{
		Enabled: false,
		Reason:  "",
	}
	if err := rm.nats.Publish(events.EventTypeKillSwitch, killSwitchEvent); err != nil {
		rm.logger.WithError(err).Error("Failed to publish kill switch event")
	}

	rm.logger.Info("Kill switch disabled")

	return nil
}

// IsKillSwitchEnabled returns true if kill switch is enabled
func (rm *RiskManager) IsKillSwitchEnabled() bool {
	return rm.killSwitch.enabled
}

// GetKillSwitchStatus returns the kill switch status
func (rm *RiskManager) GetKillSwitchStatus() *models.KillSwitchStatus {
	if !rm.killSwitch.enabled {
		return &models.KillSwitchStatus{
			Enabled:   false,
			Reason:    nil,
			Timestamp: nil,
		}
	}

	return &models.KillSwitchStatus{
		Enabled:   true,
		Reason:    &rm.killSwitch.reason,
		Timestamp: &rm.killSwitch.timestamp,
	}
}

// Helper methods

func (rm *RiskManager) checkDailyLossLimit(ctx context.Context, strategyID uuid.UUID) error {
	startOfDay := time.Now().Truncate(24 * time.Hour)

	// Get daily P&L
	var dailyPnL decimal.NullDecimal
	err := rm.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(pnl), 0) as daily_pnl
		FROM trades
		WHERE strategy_id = $1 AND entry_time >= $2
	`, strategyID, startOfDay).Scan(&dailyPnL)

	if err != nil {
		return fmt.Errorf("failed to get daily P&L: %w", err)
	}

	// Get portfolio value (simplified - get total balance)
	var portfolioValue decimal.Decimal
	err = rm.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total), 0) FROM balances
	`).Scan(&portfolioValue)

	if err != nil {
		portfolioValue = decimal.NewFromFloat(10000) // Default fallback
	}

	// Calculate loss limit
	lossLimit := portfolioValue.Mul(decimal.NewFromFloat(rm.config.DailyLossLimitPercent)).Div(decimal.NewFromInt(100))

	// Check if daily loss exceeds limit
	if dailyPnL.Valid && dailyPnL.Decimal.LessThan(lossLimit.Neg()) {
		// Auto-enable kill switch
		rm.EnableKillSwitch(ctx, fmt.Sprintf("Daily loss limit exceeded: %.2f", dailyPnL.Decimal.InexactFloat64()))
		return fmt.Errorf("daily loss limit exceeded: %.2f (limit: %.2f)",
			dailyPnL.Decimal.InexactFloat64(), lossLimit.InexactFloat64())
	}

	return nil
}

func (rm *RiskManager) checkMaxOpenPositions(ctx context.Context, strategyID uuid.UUID) error {
	var openPositions int
	err := rm.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM trades WHERE strategy_id = $1 AND exit_time IS NULL
	`, strategyID).Scan(&openPositions)

	if err != nil {
		return fmt.Errorf("failed to get open positions: %w", err)
	}

	if openPositions >= rm.config.MaxOpenPositions {
		return fmt.Errorf("max open positions reached: %d (limit: %d)",
			openPositions, rm.config.MaxOpenPositions)
	}

	return nil
}

func (rm *RiskManager) logRiskEvent(ctx context.Context, strategyID uuid.UUID, eventType, description, actionTaken string) {
	var strategyIDPtr *uuid.UUID
	if strategyID != uuid.Nil {
		strategyIDPtr = &strategyID
	}

	metadataJSON, _ := json.Marshal(map[string]interface{}{
		"timestamp": time.Now(),
	})

	_, err := rm.db.ExecContext(ctx, `
		INSERT INTO risk_events (strategy_id, event_type, description, action_taken, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, strategyIDPtr, eventType, description, actionTaken, metadataJSON)

	if err != nil {
		rm.logger.WithError(err).Error("Failed to log risk event")
	}

	// Publish risk violation event
	riskEvent := &events.RiskViolationEvent{
		StrategyID:  strategyID.String(),
		EventType:   eventType,
		Description: description,
		ActionTaken: actionTaken,
	}
	if err := rm.nats.Publish(events.EventTypeRiskViolation, riskEvent); err != nil {
		rm.logger.WithError(err).Error("Failed to publish risk event")
	}
}

func oppositeOrderSide(tradeSide models.TradeSide) string {
	if tradeSide == models.TradeSideLong {
		return string(models.OrderSideSell)
	}
	return string(models.OrderSideBuy)
}
