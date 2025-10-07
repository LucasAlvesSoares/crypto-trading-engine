package strategy

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/crypto-trading-bot/internal/config"
	"github.com/crypto-trading-bot/internal/events"
	"github.com/crypto-trading-bot/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// MeanReversionStrategy implements a mean reversion trading strategy
type MeanReversionStrategy struct {
	strategyID uuid.UUID
	symbol     string
	db         *sql.DB
	nats       *events.NATSClient
	logger     *logrus.Entry
	config     *config.Config

	// Strategy parameters
	smaPeriod     int
	rsiPeriod     int
	bbPeriod      int
	bbStdDev      float64
	rsiOversold   float64
	rsiOverbought float64

	// Price history
	priceHistory   []decimal.Decimal
	maxHistorySize int
}

// NewMeanReversionStrategy creates a new mean reversion strategy
func NewMeanReversionStrategy(
	strategyID uuid.UUID,
	symbol string,
	db *sql.DB,
	natsClient *events.NATSClient,
	cfg *config.Config,
	logger *logrus.Logger,
) *MeanReversionStrategy {
	return &MeanReversionStrategy{
		strategyID:     strategyID,
		symbol:         symbol,
		db:             db,
		nats:           natsClient,
		logger:         logger.WithField("component", "mean-reversion-strategy"),
		config:         cfg,
		smaPeriod:      20,
		rsiPeriod:      14,
		bbPeriod:       20,
		bbStdDev:       2.0,
		rsiOversold:    30.0,
		rsiOverbought:  70.0,
		priceHistory:   make([]decimal.Decimal, 0, 100),
		maxHistorySize: 100,
	}
}

// OnPriceUpdate handles price updates and generates signals
func (mrs *MeanReversionStrategy) OnPriceUpdate(ctx context.Context, update *events.PriceUpdateEvent) error {
	if update.Symbol != mrs.symbol {
		return nil // Ignore other symbols
	}

	// Add price to history
	price := decimal.NewFromFloat(update.Price)
	mrs.priceHistory = append(mrs.priceHistory, price)

	// Keep history size manageable
	if len(mrs.priceHistory) > mrs.maxHistorySize {
		mrs.priceHistory = mrs.priceHistory[1:]
	}

	// Need enough data to calculate indicators
	if len(mrs.priceHistory) < mrs.bbPeriod {
		mrs.logger.Debug("Not enough price history yet")
		return nil
	}

	// Calculate indicators
	sma := SMA(mrs.priceHistory, mrs.smaPeriod)
	rsi := RSI(mrs.priceHistory, mrs.rsiPeriod)
	upperBB, _, lowerBB := BollingerBands(mrs.priceHistory, mrs.bbPeriod, mrs.bbStdDev)

	currentPrice := mrs.priceHistory[len(mrs.priceHistory)-1]

	mrs.logger.WithFields(logrus.Fields{
		"price":    currentPrice.String(),
		"sma":      sma.String(),
		"rsi":      rsi,
		"upper_bb": upperBB.String(),
		"lower_bb": lowerBB.String(),
	}).Debug("Indicators calculated")

	// Check if we have an open position
	hasOpenPosition, err := mrs.hasOpenPosition(ctx)
	if err != nil {
		return fmt.Errorf("failed to check open position: %w", err)
	}

	// Generate entry signals
	if !hasOpenPosition {
		// LONG signal: RSI < 30 AND price < lower Bollinger Band
		if rsi < mrs.rsiOversold && currentPrice.LessThan(lowerBB) {
			return mrs.generateLongSignal(ctx, currentPrice, sma, rsi, lowerBB, update.Price)
		}

		// SHORT signal: RSI > 70 AND price > upper Bollinger Band
		// Note: For crypto spot trading, we typically don't short
		// This would be used for futures/margin trading
		if rsi > mrs.rsiOverbought && currentPrice.GreaterThan(upperBB) {
			mrs.logger.WithFields(logrus.Fields{
				"rsi":      rsi,
				"price":    currentPrice.String(),
				"upper_bb": upperBB.String(),
			}).Info("SHORT signal detected (skipping - spot trading only)")
		}
	} else {
		// Check exit conditions for open position
		return mrs.checkExitConditions(ctx, currentPrice, sma)
	}

	return nil
}

// generateLongSignal generates a long entry signal
func (mrs *MeanReversionStrategy) generateLongSignal(
	ctx context.Context,
	currentPrice decimal.Decimal,
	sma decimal.Decimal,
	rsi float64,
	lowerBB decimal.Decimal,
	priceFloat float64,
) error {
	// Calculate position size
	positionSizeUSD := decimal.NewFromFloat(mrs.config.Risk.MaxPositionSizeUSD)
	quantity := positionSizeUSD.Div(currentPrice)

	// Calculate stop-loss (2% below entry)
	stopLossPrice := currentPrice.Mul(
		decimal.NewFromFloat(1.0 - mrs.config.Risk.StopLossPercent/100.0),
	)

	// Create trade signal
	signal := &models.TradeSignal{
		ID:            uuid.New(),
		StrategyID:    mrs.strategyID,
		Symbol:        mrs.symbol,
		Side:          models.OrderSideBuy,
		Type:          models.OrderTypeMarket,
		Quantity:      quantity,
		StopLossPrice: stopLossPrice,
		Reason: fmt.Sprintf("Mean reversion LONG: RSI=%.2f (< %.0f), Price=%.2f < LowerBB=%.2f",
			rsi, mrs.rsiOversold, currentPrice.InexactFloat64(), lowerBB.InexactFloat64()),
		Indicators: map[string]float64{
			"price":    priceFloat,
			"sma":      sma.InexactFloat64(),
			"rsi":      rsi,
			"upper_bb": 0, // Not needed for long
			"lower_bb": lowerBB.InexactFloat64(),
		},
		Timestamp: time.Now(),
	}

	// Publish signal event
	signalEvent := &events.TradeSignalEvent{
		ID:            signal.ID.String(),
		StrategyID:    signal.StrategyID.String(),
		Symbol:        signal.Symbol,
		Side:          string(signal.Side),
		Type:          string(signal.Type),
		Quantity:      quantity.InexactFloat64(),
		StopLossPrice: stopLossPrice.InexactFloat64(),
		Reason:        signal.Reason,
		Indicators:    signal.Indicators,
	}

	if err := mrs.nats.Publish(events.EventTypeTradeSignal, signalEvent); err != nil {
		return fmt.Errorf("failed to publish signal: %w", err)
	}

	mrs.logger.WithFields(logrus.Fields{
		"signal_id": signal.ID,
		"side":      signal.Side,
		"quantity":  quantity.String(),
		"stop_loss": stopLossPrice.String(),
		"rsi":       rsi,
		"price":     currentPrice.String(),
		"lower_bb":  lowerBB.String(),
	}).Info("LONG signal generated")

	return nil
}

// checkExitConditions checks if we should exit an open position
func (mrs *MeanReversionStrategy) checkExitConditions(
	ctx context.Context,
	currentPrice decimal.Decimal,
	sma decimal.Decimal,
) error {
	// Get open trade
	var trade models.Trade
	var metadataJSON []byte

	err := mrs.db.QueryRowContext(ctx, `
		SELECT id, strategy_id, symbol, entry_price, quantity, side, entry_time, metadata
		FROM trades
		WHERE strategy_id = $1 AND exit_time IS NULL
		LIMIT 1
	`, mrs.strategyID).Scan(
		&trade.ID,
		&trade.StrategyID,
		&trade.Symbol,
		&trade.EntryPrice,
		&trade.Quantity,
		&trade.Side,
		&trade.EntryTime,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil // No open position
	}
	if err != nil {
		return fmt.Errorf("failed to get open trade: %w", err)
	}

	// Exit condition: Price crosses SMA
	if currentPrice.GreaterThan(sma) {
		mrs.logger.WithFields(logrus.Fields{
			"trade_id":      trade.ID,
			"entry_price":   trade.EntryPrice.String(),
			"current_price": currentPrice.String(),
			"sma":           sma.String(),
		}).Info("EXIT signal: Price crossed SMA")

		// Generate exit signal
		return mrs.generateExitSignal(ctx, &trade, currentPrice, "Price crossed SMA")
	}

	// Stop-loss check is handled by Risk Manager
	// Max hold time check is handled by Risk Manager

	return nil
}

// generateExitSignal generates an exit signal
func (mrs *MeanReversionStrategy) generateExitSignal(
	ctx context.Context,
	trade *models.Trade,
	currentPrice decimal.Decimal,
	reason string,
) error {
	// Determine exit side (opposite of entry)
	exitSide := models.OrderSideSell
	if trade.Side == models.TradeSideLong {
		exitSide = models.OrderSideSell
	} else {
		exitSide = models.OrderSideBuy
	}

	signal := &models.TradeSignal{
		ID:         uuid.New(),
		StrategyID: mrs.strategyID,
		Symbol:     mrs.symbol,
		Side:       exitSide,
		Type:       models.OrderTypeMarket,
		Quantity:   trade.Quantity,
		Reason:     reason,
		Indicators: map[string]float64{
			"price": currentPrice.InexactFloat64(),
		},
		Timestamp: time.Now(),
	}

	// Publish signal event
	signalEvent := &events.TradeSignalEvent{
		ID:         signal.ID.String(),
		StrategyID: signal.StrategyID.String(),
		Symbol:     signal.Symbol,
		Side:       string(signal.Side),
		Type:       string(signal.Type),
		Quantity:   signal.Quantity.InexactFloat64(),
		Reason:     signal.Reason,
		Indicators: signal.Indicators,
	}

	if err := mrs.nats.Publish(events.EventTypeTradeSignal, signalEvent); err != nil {
		return fmt.Errorf("failed to publish exit signal: %w", err)
	}

	mrs.logger.WithFields(logrus.Fields{
		"signal_id": signal.ID,
		"trade_id":  trade.ID,
		"exit_side": signal.Side,
		"quantity":  signal.Quantity.String(),
		"reason":    reason,
	}).Info("EXIT signal generated")

	return nil
}

// hasOpenPosition checks if there's an open position for this strategy
func (mrs *MeanReversionStrategy) hasOpenPosition(ctx context.Context) (bool, error) {
	var count int
	err := mrs.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM trades 
		WHERE strategy_id = $1 AND exit_time IS NULL
	`, mrs.strategyID).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// LoadPriceHistory loads historical price data from database
func (mrs *MeanReversionStrategy) LoadPriceHistory(ctx context.Context, limit int) error {
	rows, err := mrs.db.QueryContext(ctx, `
		SELECT close FROM price_data
		WHERE symbol = $1 AND interval = '1m'
		ORDER BY time DESC
		LIMIT $2
	`, mrs.symbol, limit)

	if err != nil {
		return fmt.Errorf("failed to load price history: %w", err)
	}
	defer rows.Close()

	prices := make([]decimal.Decimal, 0, limit)
	for rows.Next() {
		var price decimal.Decimal
		if err := rows.Scan(&price); err != nil {
			return fmt.Errorf("failed to scan price: %w", err)
		}
		prices = append(prices, price)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Reverse to get chronological order
	for i, j := 0, len(prices)-1; i < j; i, j = i+1, j-1 {
		prices[i], prices[j] = prices[j], prices[i]
	}

	mrs.priceHistory = prices

	mrs.logger.WithField("count", len(prices)).Info("Loaded price history")

	return nil
}
