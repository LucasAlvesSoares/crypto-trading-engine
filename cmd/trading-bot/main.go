package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crypto-trading-bot/internal/config"
	"github.com/crypto-trading-bot/internal/events"
	"github.com/crypto-trading-bot/internal/exchange"
	"github.com/crypto-trading-bot/internal/logger"
	"github.com/crypto-trading-bot/internal/models"
	"github.com/crypto-trading-bot/internal/order"
	"github.com/crypto-trading-bot/internal/risk"
	"github.com/crypto-trading-bot/internal/strategy"
	"github.com/google/uuid"
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
	lgr.Info("Starting Trading Bot...")

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

	// Create or get strategy
	strategyID, err := getOrCreateStrategy(db, cfg, lgr)
	if err != nil {
		lgr.Fatalf("Failed to get/create strategy: %v", err)
	}

	lgr.WithField("strategy_id", strategyID).Info("Strategy loaded")

	// Create exchange connector
	var exch exchange.Exchange
	if cfg.IsPaperTrading() {
		lgr.Info("Using Paper Trading exchange")
		exch = exchange.NewPaperExchange(
			"paper",
			decimal.NewFromFloat(10000), // $10,000 initial balance
			lgr,
		)

		// Initialize paper exchange balance in database
		initializePaperBalance(db, cfg, lgr)
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

	// Create components
	riskManager := risk.NewRiskManager(&cfg.Risk, db, natsClient, lgr)
	orderManager := order.NewOrderManager(db, exch, natsClient, lgr)
	meanReversionStrategy := strategy.NewMeanReversionStrategy(
		strategyID,
		cfg.Strategy.Symbol,
		db,
		natsClient,
		cfg,
		lgr,
	)

	// Load price history
	if err := meanReversionStrategy.LoadPriceHistory(context.Background(), 100); err != nil {
		lgr.WithError(err).Warn("Failed to load price history, will build as prices arrive")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe to price updates
	_, err = natsClient.Subscribe(string(events.EventTypePriceUpdate), func(event *events.Event) error {
		var priceUpdate events.PriceUpdateEvent
		if err := json.Unmarshal(event.Data, &priceUpdate); err != nil {
			lgr.WithError(err).Error("Failed to unmarshal price update")
			return err
		}

		// Only process if strategy is enabled
		if !cfg.Strategy.Enabled {
			return nil
		}

		// Pass to strategy
		return meanReversionStrategy.OnPriceUpdate(ctx, &priceUpdate)
	})
	if err != nil {
		lgr.Fatalf("Failed to subscribe to price updates: %v", err)
	}

	// Subscribe to trade signals
	_, err = natsClient.QueueSubscribe(
		string(events.EventTypeTradeSignal),
		"trading-bot",
		func(event *events.Event) error {
			var signal events.TradeSignalEvent
			if err := json.Unmarshal(event.Data, &signal); err != nil {
				lgr.WithError(err).Error("Failed to unmarshal trade signal")
				return err
			}

			lgr.WithFields(logrus.Fields{
				"signal_id": signal.ID,
				"symbol":    signal.Symbol,
				"side":      signal.Side,
				"reason":    signal.Reason,
			}).Info("Received trade signal")

			// Parse strategy ID
			strategyID, err := uuid.Parse(signal.StrategyID)
			if err != nil {
				lgr.WithError(err).Error("Invalid strategy ID")
				return err
			}

			// Build models.TradeSignal for risk validation
			signalModel := &models.TradeSignal{
				StrategyID:    strategyID,
				Symbol:        signal.Symbol,
				Side:          models.OrderSide(signal.Side),
				Quantity:      decimal.NewFromFloat(signal.Quantity),
				StopLossPrice: decimal.NewFromFloat(signal.StopLossPrice),
				Indicators:    signal.Indicators,
			}

			// Validate with risk manager
			if err := riskManager.ValidateTradeSignal(ctx, signalModel); err != nil {
				lgr.WithError(err).Warn("Trade signal rejected by risk manager")
				return nil // Don't return error - just skip the trade
			}

			// Place order
			if err := orderManager.PlaceOrder(ctx, &signal); err != nil {
				lgr.WithError(err).Error("Failed to place order")
				return err
			}

			return nil
		},
	)
	if err != nil {
		lgr.Fatalf("Failed to subscribe to trade signals: %v", err)
	}

	// Subscribe to kill switch events
	_, err = natsClient.Subscribe(string(events.EventTypeKillSwitch), func(event *events.Event) error {
		var killSwitch events.KillSwitchEvent
		if err := json.Unmarshal(event.Data, &killSwitch); err != nil {
			lgr.WithError(err).Error("Failed to unmarshal kill switch event")
			return err
		}

		if killSwitch.Enabled {
			lgr.WithField("reason", killSwitch.Reason).Warn("Kill switch activated!")
		} else {
			lgr.Info("Kill switch deactivated")
		}

		return nil
	})
	if err != nil {
		lgr.Fatalf("Failed to subscribe to kill switch events: %v", err)
	}

	// Start risk monitoring goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := riskManager.CheckOpenTrades(ctx); err != nil {
					lgr.WithError(err).Error("Failed to check open trades")
				}
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if cfg.Strategy.Enabled {
		lgr.Info("Trading Bot is running with strategy ENABLED. Press Ctrl+C to stop.")
	} else {
		lgr.Warn("Trading Bot is running with strategy DISABLED. Enable in config to start trading.")
	}

	lgr.WithFields(logrus.Fields{
		"mode":   cfg.Trading.Mode,
		"symbol": cfg.Strategy.Symbol,
	}).Info("Bot configuration")

	<-sigChan

	lgr.Info("Shutting down Trading Bot...")
	cancel()
	exch.Close()

	lgr.Info("Trading Bot stopped")
}

// getOrCreateStrategy gets or creates a strategy in the database
func getOrCreateStrategy(db *sql.DB, cfg *config.Config, lgr *logrus.Logger) (uuid.UUID, error) {
	// Try to get existing strategy
	var strategyID uuid.UUID
	err := db.QueryRow(`
		SELECT id FROM strategies WHERE name = $1
	`, "mean-reversion").Scan(&strategyID)

	if err == sql.ErrNoRows {
		// Create new strategy
		strategyID = uuid.New()
		configJSON, _ := json.Marshal(map[string]interface{}{
			"sma_period":    20,
			"rsi_period":    14,
			"bb_period":     20,
			"bb_std_dev":    2.0,
			"rsi_oversold":  30.0,
			"rsi_overbought": 70.0,
		})

		_, err = db.Exec(`
			INSERT INTO strategies (id, name, type, config, is_active)
			VALUES ($1, $2, $3, $4, $5)
		`, strategyID, "mean-reversion", "mean_reversion", configJSON, cfg.Strategy.Enabled)

		if err != nil {
			return uuid.Nil, err
		}

		lgr.Info("Created new mean reversion strategy")
	} else if err != nil {
		return uuid.Nil, err
	}

	return strategyID, nil
}

// initializePaperBalance initializes paper trading balance
func initializePaperBalance(db *sql.DB, cfg *config.Config, lgr *logrus.Logger) {
	// Get or create exchange
	var exchangeID uuid.UUID
	err := db.QueryRow(`
		SELECT id FROM exchanges WHERE name = 'paper' AND is_paper_trading = true
	`).Scan(&exchangeID)

	if err == sql.ErrNoRows {
		exchangeID = uuid.New()
		_, err = db.Exec(`
			INSERT INTO exchanges (id, name, api_key_encrypted, api_secret_encrypted, is_paper_trading, is_active)
			VALUES ($1, 'paper', 'none', 'none', true, true)
		`, exchangeID)

		if err != nil {
			lgr.WithError(err).Error("Failed to create paper exchange")
			return
		}
	}

	// Initialize USD balance
	_, err = db.Exec(`
		INSERT INTO balances (exchange_id, currency, available, locked)
		VALUES ($1, 'USD', 10000.00, 0)
		ON CONFLICT (exchange_id, currency) DO NOTHING
	`, exchangeID)

	if err != nil {
		lgr.WithError(err).Error("Failed to initialize paper balance")
	} else {
		lgr.Info("Paper trading balance initialized: $10,000 USD")
	}
}

