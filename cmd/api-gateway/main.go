package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/crypto-trading-bot/internal/config"
	"github.com/crypto-trading-bot/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	// Initialize logger
	lgr := logger.NewLogger(cfg.Logging.Level, cfg.Logging.Format)
	lgr.Info("Starting API Gateway...")

	// Connect to database
	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		lgr.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Set Gin mode
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"timestamp": time.Now(),
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Get overview/dashboard stats
		v1.GET("/overview", func(c *gin.Context) {
			overview := getOverview(db, lgr)
			c.JSON(200, overview)
		})

		// Get trades
		v1.GET("/trades", func(c *gin.Context) {
			trades := getTrades(db, lgr)
			c.JSON(200, trades)
		})

		// Get orders
		v1.GET("/orders", func(c *gin.Context) {
			orders := getOrders(db, lgr)
			c.JSON(200, orders)
		})

		// Get balances
		v1.GET("/balances", func(c *gin.Context) {
			balances := getBalances(db, lgr)
			c.JSON(200, balances)
		})

		// Get strategy status
		v1.GET("/strategy", func(c *gin.Context) {
			strategy := getStrategy(db, lgr)
			c.JSON(200, strategy)
		})

		// Toggle strategy
		v1.POST("/strategy/toggle", func(c *gin.Context) {
			var req struct {
				Enabled bool `json:"enabled"`
			}
			if err := c.BindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}

			err := toggleStrategy(db, req.Enabled, lgr)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, gin.H{"success": true, "enabled": req.Enabled})
		})

		// Kill switch endpoints
		v1.GET("/kill-switch", func(c *gin.Context) {
			status := getKillSwitchStatus(db, lgr)
			c.JSON(200, status)
		})

		v1.POST("/kill-switch/enable", func(c *gin.Context) {
			var req struct {
				Reason string `json:"reason"`
			}
			if err := c.BindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}

			err := enableKillSwitch(db, req.Reason, lgr)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, gin.H{"success": true})
		})

		v1.POST("/kill-switch/disable", func(c *gin.Context) {
			err := disableKillSwitch(db, lgr)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, gin.H{"success": true})
		})

		// Get risk events
		v1.GET("/risk-events", func(c *gin.Context) {
			events := getRiskEvents(db, lgr)
			c.JSON(200, events)
		})

		// Get logs
		v1.GET("/logs", func(c *gin.Context) {
			logs := getLogs(db, lgr)
			c.JSON(200, logs)
		})
	}

	// Start server
	port := ":" + cfg.API.Port
	lgr.WithField("port", port).Info("API Gateway started")
	
	if err := router.Run(port); err != nil {
		lgr.Fatalf("Failed to start server: %v", err)
	}
}

// Helper functions

func getOverview(db *sql.DB, lgr *logrus.Logger) map[string]interface{} {
	overview := map[string]interface{}{
		"portfolio_value": 10000.0,
		"daily_pnl":       0.0,
		"total_pnl":       0.0,
		"open_positions":  0,
		"total_trades":    0,
		"win_rate":        0.0,
	}

	// Get total balances
	var totalBalance decimal.NullDecimal
	db.QueryRow("SELECT SUM(total) FROM balances").Scan(&totalBalance)
	if totalBalance.Valid {
		overview["portfolio_value"], _ = totalBalance.Decimal.Float64()
	}

	// Get daily P&L
	startOfDay := time.Now().Truncate(24 * time.Hour)
	var dailyPnL decimal.NullDecimal
	db.QueryRow("SELECT COALESCE(SUM(pnl), 0) FROM trades WHERE entry_time >= $1", startOfDay).Scan(&dailyPnL)
	if dailyPnL.Valid {
		overview["daily_pnl"], _ = dailyPnL.Decimal.Float64()
	}

	// Get total P&L
	var totalPnL decimal.NullDecimal
	db.QueryRow("SELECT COALESCE(SUM(pnl), 0) FROM trades WHERE exit_time IS NOT NULL").Scan(&totalPnL)
	if totalPnL.Valid {
		overview["total_pnl"], _ = totalPnL.Decimal.Float64()
	}

	// Get open positions
	var openPositions int
	db.QueryRow("SELECT COUNT(*) FROM trades WHERE exit_time IS NULL").Scan(&openPositions)
	overview["open_positions"] = openPositions

	// Get total trades
	var totalTrades int
	db.QueryRow("SELECT COUNT(*) FROM trades").Scan(&totalTrades)
	overview["total_trades"] = totalTrades

	// Get win rate
	var winningTrades int
	db.QueryRow("SELECT COUNT(*) FROM trades WHERE pnl > 0").Scan(&winningTrades)
	if totalTrades > 0 {
		overview["win_rate"] = float64(winningTrades) / float64(totalTrades) * 100
	}

	return overview
}

func getTrades(db *sql.DB, lgr *logrus.Logger) []map[string]interface{} {
	rows, err := db.Query(`
		SELECT id, symbol, side, entry_price, exit_price, quantity, pnl, pnl_percent, 
		       entry_time, exit_time, exit_reason
		FROM trades
		ORDER BY entry_time DESC
		LIMIT 50
	`)
	if err != nil {
		lgr.WithError(err).Error("Failed to get trades")
		return []map[string]interface{}{}
	}
	defer rows.Close()

	trades := []map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var symbol, side string
		var entryPrice, quantity decimal.Decimal
		var exitPrice, pnl, pnlPercent decimal.NullDecimal
		var entryTime time.Time
		var exitTime *time.Time
		var exitReason *string

		rows.Scan(&id, &symbol, &side, &entryPrice, &exitPrice, &quantity, &pnl, &pnlPercent,
			&entryTime, &exitTime, &exitReason)

		trade := map[string]interface{}{
			"id":          id.String(),
			"symbol":      symbol,
			"side":        side,
			"entry_price": entryPrice.String(),
			"quantity":    quantity.String(),
			"entry_time":  entryTime,
			"status":      "open",
		}

		if exitPrice.Valid {
			trade["exit_price"] = exitPrice.Decimal.String()
			trade["status"] = "closed"
		}
		if pnl.Valid {
			pnlFloat, _ := pnl.Decimal.Float64()
			trade["pnl"] = pnlFloat
		}
		if pnlPercent.Valid {
			pnlPctFloat, _ := pnlPercent.Decimal.Float64()
			trade["pnl_percent"] = pnlPctFloat
		}
		if exitTime != nil {
			trade["exit_time"] = *exitTime
		}
		if exitReason != nil {
			trade["exit_reason"] = *exitReason
		}

		trades = append(trades, trade)
	}

	return trades
}

func getOrders(db *sql.DB, lgr *logrus.Logger) []map[string]interface{} {
	rows, err := db.Query(`
		SELECT id, symbol, side, type, quantity, status, created_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT 50
	`)
	if err != nil {
		lgr.WithError(err).Error("Failed to get orders")
		return []map[string]interface{}{}
	}
	defer rows.Close()

	orders := []map[string]interface{}{}
	for rows.Next() {
		var id uuid.UUID
		var symbol, side, orderType, status string
		var quantity decimal.Decimal
		var createdAt time.Time

		rows.Scan(&id, &symbol, &side, &orderType, &quantity, &status, &createdAt)

		orders = append(orders, map[string]interface{}{
			"id":         id.String(),
			"symbol":     symbol,
			"side":       side,
			"type":       orderType,
			"quantity":   quantity.String(),
			"status":     status,
			"created_at": createdAt,
		})
	}

	return orders
}

func getBalances(db *sql.DB, lgr *logrus.Logger) []map[string]interface{} {
	rows, err := db.Query("SELECT currency, available, locked, total FROM balances")
	if err != nil {
		lgr.WithError(err).Error("Failed to get balances")
		return []map[string]interface{}{}
	}
	defer rows.Close()

	balances := []map[string]interface{}{}
	for rows.Next() {
		var currency string
		var available, locked, total decimal.Decimal

		rows.Scan(&currency, &available, &locked, &total)

		avail, _ := available.Float64()
		lock, _ := locked.Float64()
		tot, _ := total.Float64()

		balances = append(balances, map[string]interface{}{
			"currency":  currency,
			"available": avail,
			"locked":    lock,
			"total":     tot,
		})
	}

	return balances
}

func getStrategy(db *sql.DB, lgr *logrus.Logger) map[string]interface{} {
	var id uuid.UUID
	var name, strategyType string
	var isActive bool
	var config json.RawMessage

	err := db.QueryRow(`
		SELECT id, name, type, is_active, config
		FROM strategies
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&id, &name, &strategyType, &isActive, &config)

	if err != nil {
		lgr.WithError(err).Error("Failed to get strategy")
		return map[string]interface{}{
			"id":        "",
			"name":      "unknown",
			"type":      "unknown",
			"is_active": false,
		}
	}

	return map[string]interface{}{
		"id":        id.String(),
		"name":      name,
		"type":      strategyType,
		"is_active": isActive,
		"config":    config,
	}
}

func toggleStrategy(db *sql.DB, enabled bool, lgr *logrus.Logger) error {
	_, err := db.Exec("UPDATE strategies SET is_active = $1", enabled)
	if err != nil {
		lgr.WithError(err).Error("Failed to toggle strategy")
		return err
	}

	lgr.WithField("enabled", enabled).Info("Strategy toggled")
	return nil
}

func getKillSwitchStatus(db *sql.DB, lgr *logrus.Logger) map[string]interface{} {
	var value json.RawMessage
	err := db.QueryRow("SELECT value FROM system_config WHERE key = 'kill_switch'").Scan(&value)
	if err != nil {
		lgr.WithError(err).Error("Failed to get kill switch status")
		return map[string]interface{}{
			"enabled": false,
		}
	}

	var status map[string]interface{}
	json.Unmarshal(value, &status)
	return status
}

func enableKillSwitch(db *sql.DB, reason string, lgr *logrus.Logger) error {
	_, err := db.Exec(`
		UPDATE system_config
		SET value = jsonb_build_object('enabled', true, 'reason', $1, 'timestamp', to_jsonb(NOW()))
		WHERE key = 'kill_switch'
	`, reason)

	if err != nil {
		lgr.WithError(err).Error("Failed to enable kill switch")
		return err
	}

	lgr.WithField("reason", reason).Warn("Kill switch enabled via API")
	return nil
}

func disableKillSwitch(db *sql.DB, lgr *logrus.Logger) error {
	_, err := db.Exec(`
		UPDATE system_config
		SET value = jsonb_build_object('enabled', false, 'reason', null, 'timestamp', to_jsonb(NOW()))
		WHERE key = 'kill_switch'
	`)

	if err != nil {
		lgr.WithError(err).Error("Failed to disable kill switch")
		return err
	}

	lgr.Info("Kill switch disabled via API")
	return nil
}

func getRiskEvents(db *sql.DB, lgr *logrus.Logger) []map[string]interface{} {
	rows, err := db.Query(`
		SELECT event_type, description, action_taken, timestamp
		FROM risk_events
		ORDER BY timestamp DESC
		LIMIT 50
	`)
	if err != nil {
		lgr.WithError(err).Error("Failed to get risk events")
		return []map[string]interface{}{}
	}
	defer rows.Close()

	events := []map[string]interface{}{}
	for rows.Next() {
		var eventType, description, actionTaken string
		var timestamp time.Time

		rows.Scan(&eventType, &description, &actionTaken, &timestamp)

		events = append(events, map[string]interface{}{
			"event_type":   eventType,
			"description":  description,
			"action_taken": actionTaken,
			"timestamp":    timestamp,
		})
	}

	return events
}

func getLogs(db *sql.DB, lgr *logrus.Logger) []map[string]interface{} {
	rows, err := db.Query(`
		SELECT level, component, message, timestamp
		FROM logs
		ORDER BY timestamp DESC
		LIMIT 100
	`)
	if err != nil {
		lgr.WithError(err).Error("Failed to get logs")
		return []map[string]interface{}{}
	}
	defer rows.Close()

	logs := []map[string]interface{}{}
	for rows.Next() {
		var level, component, message string
		var timestamp time.Time

		rows.Scan(&level, &component, &message, &timestamp)

		logs = append(logs, map[string]interface{}{
			"level":     level,
			"component": component,
			"message":   message,
			"timestamp": timestamp,
		})
	}

	return logs
}

