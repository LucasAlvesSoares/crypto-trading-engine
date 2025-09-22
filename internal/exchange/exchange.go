package exchange

import (
	"context"
	"time"

	"github.com/crypto-trading-bot/internal/models"
	"github.com/shopspring/decimal"
)

// Exchange defines the interface for exchange connectors
type Exchange interface {
	// Name returns the exchange name
	Name() string

	// PlaceOrder places an order on the exchange
	PlaceOrder(ctx context.Context, order *OrderRequest) (*OrderResponse, error)

	// CancelOrder cancels an order
	CancelOrder(ctx context.Context, orderID string) error

	// GetOrder gets an order by ID
	GetOrder(ctx context.Context, orderID string) (*OrderResponse, error)

	// GetBalance gets account balances
	GetBalance(ctx context.Context) (map[string]*Balance, error)

	// GetPrice gets the current price for a symbol
	GetPrice(ctx context.Context, symbol string) (decimal.Decimal, error)

	// SubscribePriceUpdates subscribes to price updates
	SubscribePriceUpdates(ctx context.Context, symbols []string, callback func(*PriceUpdate)) error

	// Close closes the exchange connection
	Close() error
}

// OrderRequest represents a request to place an order
type OrderRequest struct {
	Symbol        string
	Side          models.OrderSide
	Type          models.OrderType
	Quantity      decimal.Decimal
	Price         *decimal.Decimal // For limit orders
	StopLossPrice *decimal.Decimal
}

// OrderResponse represents the response from placing an order
type OrderResponse struct {
	ID               string
	ClientOrderID    string
	ExchangeOrderID  string
	Symbol           string
	Side             models.OrderSide
	Type             models.OrderType
	Status           models.OrderStatus
	Quantity         decimal.Decimal
	Price            *decimal.Decimal
	FilledQuantity   decimal.Decimal
	AverageFillPrice *decimal.Decimal
	Fees             decimal.Decimal
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Balance represents account balance
type Balance struct {
	Currency  string
	Available decimal.Decimal
	Locked    decimal.Decimal
	Total     decimal.Decimal
}

// PriceUpdate represents a real-time price update
type PriceUpdate struct {
	Exchange  string
	Symbol    string
	Price     decimal.Decimal
	Volume    decimal.Decimal
	Timestamp time.Time
}

// Trade represents a trade execution
type Trade struct {
	ID        string
	Symbol    string
	Price     decimal.Decimal
	Quantity  decimal.Decimal
	Side      models.OrderSide
	Timestamp time.Time
}

// Candle represents OHLCV candle data
type Candle struct {
	Time   time.Time
	Open   decimal.Decimal
	High   decimal.Decimal
	Low    decimal.Decimal
	Close  decimal.Decimal
	Volume decimal.Decimal
}

