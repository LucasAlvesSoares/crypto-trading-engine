package main

import (
	"context"
	"database/sql"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crypto-trading-bot/internal/config"
	"github.com/crypto-trading-bot/internal/events"
	"github.com/crypto-trading-bot/internal/exchange"
	"github.com/crypto-trading-bot/internal/logger"
	"github.com/crypto-trading-bot/internal/marketdata"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	lgr := logger.NewLogger(cfg.Logging.Level, cfg.Logging.Format)
	lgr.Info("Starting Market Data Service...")

	// Connect to database
	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		lgr.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		lgr.Fatalf("Failed to ping database: %v", err)
	}

	// Connect to NATS
	natsClient, err := events.NewNATSClient(cfg.NATS.URL, lgr)
	if err != nil {
		lgr.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer natsClient.Close()

	// Create exchange connector
	var exch exchange.Exchange
	if cfg.IsPaperTrading() {
		lgr.Info("Using Paper Trading exchange")
		exch = exchange.NewPaperExchange(
			"paper",
			decimal.NewFromFloat(10000), // $10,000 initial balance
			lgr,
		)
	} else {
		lgr.Info("Using Coinbase exchange")
		exch = exchange.NewCoinbaseExchange(
			cfg.Coinbase.APIKey,
			cfg.Coinbase.APISecret,
			cfg.Coinbase.APIPassphrase,
			cfg.Coinbase.UseSandbox,
			lgr,
		)
	}

	// Create market data service
	symbols := []string{cfg.Strategy.Symbol}
	mds := marketdata.NewMarketDataService(db, exch, natsClient, symbols, lgr)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start market data service
	if err := mds.Start(ctx); err != nil {
		lgr.Fatalf("Failed to start market data service: %v", err)
	}

	// If using paper exchange, simulate price updates
	if cfg.IsPaperTrading() {
		go simulatePriceUpdates(ctx, exch.(*exchange.PaperExchange), symbols[0], lgr)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	lgr.Info("Market Data Service is running. Press Ctrl+C to stop.")

	<-sigChan

	lgr.Info("Shutting down Market Data Service...")
	cancel()
	exch.Close()

	lgr.Info("Market Data Service stopped")
}

// simulatePriceUpdates simulates price updates for paper trading
func simulatePriceUpdates(ctx context.Context, paperExch *exchange.PaperExchange, symbol string, lgr *logrus.Logger) {
	// Start with a base price (e.g., BTC at $45,000)
	currentPrice := decimal.NewFromFloat(45000.0)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Simulate price movement (random walk with small changes)
			change := (rand.Float64() - 0.5) * 50 // +/- $25
			currentPrice = currentPrice.Add(decimal.NewFromFloat(change))

			// Update paper exchange
			paperExch.UpdatePrice(symbol, currentPrice)
		}
	}
}
