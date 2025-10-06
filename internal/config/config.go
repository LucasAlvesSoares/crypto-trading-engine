package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Database DatabaseConfig
	NATS     NATSConfig
	Coinbase CoinbaseConfig
	Trading  TradingConfig
	Risk     RiskConfig
	Strategy StrategyConfig
	API      APIConfig
	Logging  LoggingConfig
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	URL string
}

// NATSConfig holds NATS connection configuration
type NATSConfig struct {
	URL string
}

// CoinbaseConfig holds Coinbase API configuration
type CoinbaseConfig struct {
	APIKey        string
	APISecret     string
	APIPassphrase string
	UseSandbox    bool
}

// TradingConfig holds trading mode configuration
type TradingConfig struct {
	Mode string // "paper" or "live"
}

// RiskConfig holds risk management parameters
type RiskConfig struct {
	MaxPositionSizeUSD    float64
	MaxOpenPositions      int
	DailyLossLimitPercent float64
	StopLossPercent       float64
	MaxHoldTimeHours      int
	MinBalanceUSD         float64
}

// StrategyConfig holds strategy configuration
type StrategyConfig struct {
	Enabled   bool
	Symbol    string
	Timeframe string
}

// APIConfig holds API server configuration
type APIConfig struct {
	Port      string
	JWTSecret string
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string
	Format string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (optional)
	_ = godotenv.Load()

	config := &Config{
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", "postgresql://trading:trading@localhost:5432/trading_bot?sslmode=disable"),
		},
		NATS: NATSConfig{
			URL: getEnv("NATS_URL", "nats://localhost:4222"),
		},
		Coinbase: CoinbaseConfig{
			APIKey:        getEnv("COINBASE_API_KEY", ""),
			APISecret:     getEnv("COINBASE_API_SECRET", ""),
			APIPassphrase: getEnv("COINBASE_API_PASSPHRASE", ""),
			UseSandbox:    getEnvBool("COINBASE_USE_SANDBOX", true),
		},
		Trading: TradingConfig{
			Mode: getEnv("TRADING_MODE", "paper"),
		},
		Risk: RiskConfig{
			MaxPositionSizeUSD:    getEnvFloat("RISK_MAX_POSITION_SIZE_USD", 100.0),
			MaxOpenPositions:      getEnvInt("RISK_MAX_OPEN_POSITIONS", 1),
			DailyLossLimitPercent: getEnvFloat("RISK_DAILY_LOSS_LIMIT_PERCENT", 2.0),
			StopLossPercent:       getEnvFloat("RISK_STOP_LOSS_PERCENT", 2.0),
			MaxHoldTimeHours:      getEnvInt("RISK_MAX_HOLD_TIME_HOURS", 24),
			MinBalanceUSD:         getEnvFloat("RISK_MIN_BALANCE_USD", 50.0),
		},
		Strategy: StrategyConfig{
			Enabled:   getEnvBool("STRATEGY_ENABLED", false),
			Symbol:    getEnv("STRATEGY_SYMBOL", "BTC-USD"),
			Timeframe: getEnv("STRATEGY_TIMEFRAME", "1m"),
		},
		API: APIConfig{
			Port:      getEnv("API_PORT", "8080"),
			JWTSecret: getEnv("API_JWT_SECRET", "change_me_in_production"),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate trading mode
	if c.Trading.Mode != "paper" && c.Trading.Mode != "live" {
		return fmt.Errorf("invalid trading mode: %s (must be 'paper' or 'live')", c.Trading.Mode)
	}

	// Validate risk parameters
	if c.Risk.MaxPositionSizeUSD <= 0 {
		return fmt.Errorf("max position size must be positive")
	}
	if c.Risk.MaxOpenPositions <= 0 {
		return fmt.Errorf("max open positions must be positive")
	}
	if c.Risk.DailyLossLimitPercent <= 0 || c.Risk.DailyLossLimitPercent > 100 {
		return fmt.Errorf("daily loss limit must be between 0 and 100")
	}
	if c.Risk.StopLossPercent <= 0 || c.Risk.StopLossPercent > 100 {
		return fmt.Errorf("stop loss percent must be between 0 and 100")
	}

	// Validate database URL
	if c.Database.URL == "" {
		return fmt.Errorf("database URL is required")
	}

	return nil
}

// IsPaperTrading returns true if in paper trading mode
func (c *Config) IsPaperTrading() bool {
	return c.Trading.Mode == "paper"
}

// GetMaxHoldDuration returns the maximum hold duration
func (c *Config) GetMaxHoldDuration() time.Duration {
	return time.Duration(c.Risk.MaxHoldTimeHours) * time.Hour
}

// Helper functions to get environment variables with defaults

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolValue
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return floatValue
}
