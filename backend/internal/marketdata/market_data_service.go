package marketdata

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/crypto-trading-bot/internal/events"
	"github.com/crypto-trading-bot/internal/exchange"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// MarketDataService handles market data ingestion and storage
type MarketDataService struct {
	db             *sql.DB
	exchange       exchange.Exchange
	nats           *events.NATSClient
	logger         *logrus.Entry
	symbols        []string
	priceCache     map[string]*PriceCacheEntry
	priceCacheMu   sync.RWMutex
	candleBuffer   map[string]*CandleBuffer
	candleBufferMu sync.Mutex
}

// PriceCacheEntry holds cached price data
type PriceCacheEntry struct {
	Price     decimal.Decimal
	Timestamp time.Time
}

// CandleBuffer holds price data for candle aggregation
type CandleBuffer struct {
	Symbol    string
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
	Volume    decimal.Decimal
	StartTime time.Time
}

// NewMarketDataService creates a new market data service
func NewMarketDataService(
	db *sql.DB,
	exch exchange.Exchange,
	natsClient *events.NATSClient,
	symbols []string,
	logger *logrus.Logger,
) *MarketDataService {
	return &MarketDataService{
		db:           db,
		exchange:     exch,
		nats:         natsClient,
		logger:       logger.WithField("component", "market-data"),
		symbols:      symbols,
		priceCache:   make(map[string]*PriceCacheEntry),
		candleBuffer: make(map[string]*CandleBuffer),
	}
}

// Start starts the market data service
func (mds *MarketDataService) Start(ctx context.Context) error {
	// Subscribe to price updates from exchange
	err := mds.exchange.SubscribePriceUpdates(ctx, mds.symbols, func(update *exchange.PriceUpdate) {
		mds.handlePriceUpdate(ctx, update)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to price updates: %w", err)
	}

	// Start candle aggregation goroutine
	go mds.runCandleAggregation(ctx)

	// Start gap detection goroutine
	go mds.runGapDetection(ctx)

	mds.logger.WithField("symbols", mds.symbols).Info("Market data service started")

	return nil
}

// handlePriceUpdate processes incoming price updates
func (mds *MarketDataService) handlePriceUpdate(ctx context.Context, update *exchange.PriceUpdate) {
	// Update price cache
	mds.priceCacheMu.Lock()
	mds.priceCache[update.Symbol] = &PriceCacheEntry{
		Price:     update.Price,
		Timestamp: update.Timestamp,
	}
	mds.priceCacheMu.Unlock()

	// Add to candle buffer
	mds.addToCandleBuffer(update)

	// Publish price update event
	priceEvent := &events.PriceUpdateEvent{
		Exchange: update.Exchange,
		Symbol:   update.Symbol,
		Price:    update.Price.InexactFloat64(),
		Volume:   update.Volume.InexactFloat64(),
		Time:     update.Timestamp,
	}

	if err := mds.nats.Publish(events.EventTypePriceUpdate, priceEvent); err != nil {
		mds.logger.WithError(err).Error("Failed to publish price update event")
	}

	mds.logger.WithFields(logrus.Fields{
		"symbol": update.Symbol,
		"price":  update.Price.String(),
	}).Debug("Price update processed")
}

// addToCandleBuffer adds a price update to the candle buffer
func (mds *MarketDataService) addToCandleBuffer(update *exchange.PriceUpdate) {
	mds.candleBufferMu.Lock()
	defer mds.candleBufferMu.Unlock()

	// Truncate timestamp to 1-minute intervals
	candleTime := update.Timestamp.Truncate(1 * time.Minute)

	buffer, exists := mds.candleBuffer[update.Symbol]
	if !exists || !buffer.StartTime.Equal(candleTime) {
		// New candle period
		if exists {
			// Save previous candle
			go mds.saveCandle(context.Background(), buffer)
		}

		// Create new buffer
		buffer = &CandleBuffer{
			Symbol:    update.Symbol,
			Open:      update.Price,
			High:      update.Price,
			Low:       update.Price,
			Close:     update.Price,
			Volume:    update.Volume,
			StartTime: candleTime,
		}
		mds.candleBuffer[update.Symbol] = buffer
	} else {
		// Update existing buffer
		if update.Price.GreaterThan(buffer.High) {
			buffer.High = update.Price
		}
		if update.Price.LessThan(buffer.Low) {
			buffer.Low = update.Price
		}
		buffer.Close = update.Price
		buffer.Volume = buffer.Volume.Add(update.Volume)
	}
}

// saveCandle saves a completed candle to the database
func (mds *MarketDataService) saveCandle(ctx context.Context, buffer *CandleBuffer) {
	_, err := mds.db.ExecContext(ctx, `
		INSERT INTO price_data (time, exchange, symbol, open, high, low, close, volume, interval)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '1m')
		ON CONFLICT (time, exchange, symbol, interval) DO UPDATE
		SET open = EXCLUDED.open,
		    high = EXCLUDED.high,
		    low = EXCLUDED.low,
		    close = EXCLUDED.close,
		    volume = EXCLUDED.volume
	`, buffer.StartTime, mds.exchange.Name(), buffer.Symbol,
		buffer.Open, buffer.High, buffer.Low, buffer.Close, buffer.Volume)

	if err != nil {
		mds.logger.WithError(err).WithField("symbol", buffer.Symbol).Error("Failed to save candle")
		return
	}

	mds.logger.WithFields(logrus.Fields{
		"symbol": buffer.Symbol,
		"time":   buffer.StartTime,
		"open":   buffer.Open.String(),
		"high":   buffer.High.String(),
		"low":    buffer.Low.String(),
		"close":  buffer.Close.String(),
	}).Debug("Candle saved")
}

// runCandleAggregation periodically saves pending candles
func (mds *MarketDataService) runCandleAggregation(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mds.flushCandleBuffers(ctx)
		}
	}
}

// flushCandleBuffers saves all pending candle buffers
func (mds *MarketDataService) flushCandleBuffers(ctx context.Context) {
	mds.candleBufferMu.Lock()
	buffers := make([]*CandleBuffer, 0, len(mds.candleBuffer))
	for _, buffer := range mds.candleBuffer {
		buffers = append(buffers, buffer)
	}
	mds.candleBufferMu.Unlock()

	for _, buffer := range buffers {
		mds.saveCandle(ctx, buffer)
	}
}

// runGapDetection periodically checks for gaps in price data
func (mds *MarketDataService) runGapDetection(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mds.detectGaps(ctx)
		}
	}
}

// detectGaps detects gaps in price data
func (mds *MarketDataService) detectGaps(ctx context.Context) {
	for _, symbol := range mds.symbols {
		// Get latest candle time
		var lastTime time.Time
		err := mds.db.QueryRowContext(ctx, `
			SELECT MAX(time) FROM price_data
			WHERE symbol = $1 AND interval = '1m'
		`, symbol).Scan(&lastTime)

		if err != nil {
			mds.logger.WithError(err).WithField("symbol", symbol).Error("Failed to get latest candle time")
			continue
		}

		// Check if there's a gap (more than 5 minutes)
		gap := time.Since(lastTime)
		if gap > 5*time.Minute {
			mds.logger.WithFields(logrus.Fields{
				"symbol":    symbol,
				"last_time": lastTime,
				"gap":       gap,
			}).Warn("Gap detected in price data")

			// TODO: Implement backfill from REST API
			// For now, just log the gap
		}
	}
}

// GetLatestPrice gets the latest price from cache
func (mds *MarketDataService) GetLatestPrice(symbol string) (decimal.Decimal, error) {
	mds.priceCacheMu.RLock()
	defer mds.priceCacheMu.RUnlock()

	entry, exists := mds.priceCache[symbol]
	if !exists {
		return decimal.Zero, fmt.Errorf("no price available for symbol %s", symbol)
	}

	// Check if price is stale (older than 1 minute)
	if time.Since(entry.Timestamp) > 1*time.Minute {
		mds.logger.WithField("symbol", symbol).Warn("Price data is stale")
	}

	return entry.Price, nil
}

// GetHistoricalCandles gets historical candle data
func (mds *MarketDataService) GetHistoricalCandles(
	ctx context.Context,
	symbol string,
	startTime, endTime time.Time,
	interval string,
) ([]exchange.Candle, error) {
	rows, err := mds.db.QueryContext(ctx, `
		SELECT time, open, high, low, close, volume
		FROM price_data
		WHERE symbol = $1 AND interval = $2 AND time >= $3 AND time <= $4
		ORDER BY time ASC
	`, symbol, interval, startTime, endTime)

	if err != nil {
		return nil, fmt.Errorf("failed to get historical candles: %w", err)
	}
	defer rows.Close()

	candles := make([]exchange.Candle, 0)
	for rows.Next() {
		var candle exchange.Candle
		err := rows.Scan(
			&candle.Time,
			&candle.Open,
			&candle.High,
			&candle.Low,
			&candle.Close,
			&candle.Volume,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan candle: %w", err)
		}
		candles = append(candles, candle)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return candles, nil
}

// CleanupOldData removes old price data beyond retention period
func (mds *MarketDataService) CleanupOldData(ctx context.Context, retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	result, err := mds.db.ExecContext(ctx, `
		DELETE FROM price_data WHERE time < $1
	`, cutoffTime)

	if err != nil {
		return fmt.Errorf("failed to cleanup old data: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	mds.logger.WithFields(logrus.Fields{
		"cutoff_time":   cutoffTime,
		"rows_affected": rowsAffected,
	}).Info("Old price data cleaned up")

	return nil
}
