package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// OrderSide represents the side of an order
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// OrderType represents the type of an order
type OrderType string

const (
	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"
)

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "PENDING"
	OrderStatusOpen      OrderStatus = "OPEN"
	OrderStatusFilled    OrderStatus = "FILLED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
	OrderStatusFailed    OrderStatus = "FAILED"
)

// TradeSide represents the side of a trade
type TradeSide string

const (
	TradeSideLong  TradeSide = "LONG"
	TradeSideShort TradeSide = "SHORT"
)

// ExitReason represents why a trade was exited
type ExitReason string

const (
	ExitReasonStopLoss   ExitReason = "STOP_LOSS"
	ExitReasonTakeProfit ExitReason = "TAKE_PROFIT"
	ExitReasonTimeout    ExitReason = "TIMEOUT"
	ExitReasonManual     ExitReason = "MANUAL"
	ExitReasonSignal     ExitReason = "SIGNAL"
	ExitReasonKillSwitch ExitReason = "KILL_SWITCH"
)

// Order represents a trading order
type Order struct {
	ID               uuid.UUID
	ClientOrderID    string
	ExchangeOrderID  string
	ExchangeID       uuid.UUID
	StrategyID       uuid.UUID
	Symbol           string
	Side             OrderSide
	Type             OrderType
	Quantity         decimal.Decimal
	Price            decimal.NullDecimal
	StopLossPrice    decimal.NullDecimal
	Status           OrderStatus
	FilledQuantity   decimal.Decimal
	AverageFillPrice decimal.NullDecimal
	Fees             decimal.Decimal
	CreatedAt        time.Time
	UpdatedAt        time.Time
	FilledAt         *time.Time
}

// Trade represents a completed or open trading position
type Trade struct {
	ID           uuid.UUID
	EntryOrderID uuid.UUID
	ExitOrderID  *uuid.UUID
	StrategyID   uuid.UUID
	Symbol       string
	EntryPrice   decimal.Decimal
	ExitPrice    decimal.NullDecimal
	Quantity     decimal.Decimal
	Side         TradeSide
	EntryTime    time.Time
	ExitTime     *time.Time
	PnL          decimal.NullDecimal
	PnLPercent   decimal.NullDecimal
	FeesTotal    decimal.Decimal
	HoldDuration *time.Duration
	ExitReason   *ExitReason
	Metadata     map[string]interface{}
	CreatedAt    time.Time
}

// IsOpen returns true if the trade is still open
func (t *Trade) IsOpen() bool {
	return t.ExitTime == nil
}

// CalculatePnL calculates the P&L for the trade
func (t *Trade) CalculatePnL(currentPrice decimal.Decimal) decimal.Decimal {
	if !t.ExitPrice.Valid {
		// Use current price for open trades
		if t.Side == TradeSideLong {
			return currentPrice.Sub(t.EntryPrice).Mul(t.Quantity).Sub(t.FeesTotal)
		}
		return t.EntryPrice.Sub(currentPrice).Mul(t.Quantity).Sub(t.FeesTotal)
	}
	// Use exit price for closed trades
	if t.Side == TradeSideLong {
		return t.ExitPrice.Decimal.Sub(t.EntryPrice).Mul(t.Quantity).Sub(t.FeesTotal)
	}
	return t.EntryPrice.Sub(t.ExitPrice.Decimal).Mul(t.Quantity).Sub(t.FeesTotal)
}

// TradeSignal represents a signal to enter or exit a trade
type TradeSignal struct {
	ID            uuid.UUID
	StrategyID    uuid.UUID
	Symbol        string
	Side          OrderSide
	Type          OrderType
	Quantity      decimal.Decimal
	Price         decimal.NullDecimal
	StopLossPrice decimal.Decimal
	Reason        string
	Indicators    map[string]float64
	Timestamp     time.Time
}

// PriceData represents OHLCV price data
type PriceData struct {
	Time     time.Time
	Exchange string
	Symbol   string
	Open     decimal.Decimal
	High     decimal.Decimal
	Low      decimal.Decimal
	Close    decimal.Decimal
	Volume   decimal.Decimal
	Interval string
}

// Balance represents account balance
type Balance struct {
	ExchangeID uuid.UUID
	Currency   string
	Available  decimal.Decimal
	Locked     decimal.Decimal
	Total      decimal.Decimal
	UpdatedAt  time.Time
}

// RiskEvent represents a risk management event
type RiskEvent struct {
	ID          uuid.UUID
	StrategyID  *uuid.UUID
	EventType   string
	Description string
	ActionTaken string
	Metadata    map[string]interface{}
	Timestamp   time.Time
}

// PerformanceSnapshot represents a snapshot of strategy performance
type PerformanceSnapshot struct {
	ID             uuid.UUID
	StrategyID     uuid.UUID
	PortfolioValue decimal.Decimal
	CashBalance    decimal.Decimal
	TotalPnL       decimal.Decimal
	DailyPnL       decimal.Decimal
	OpenPositions  int
	TotalTrades    int
	WinRate        decimal.NullDecimal
	SharpeRatio    decimal.NullDecimal
	MaxDrawdown    decimal.NullDecimal
	Timestamp      time.Time
}

// TradeStats represents aggregated statistics for trades
type TradeStats struct {
	TotalTrades      int
	ClosedTrades     int
	WinningTrades    int
	LosingTrades     int
	TotalPnL         decimal.Decimal
	AveragePnL       decimal.Decimal
	MaxProfit        decimal.Decimal
	MaxLoss          decimal.Decimal
	AvgReturnPercent decimal.Decimal
	TotalFees        decimal.Decimal
	WinRate          float64
}

// CalculateWinRate calculates the win rate
func (ts *TradeStats) CalculateWinRate() float64 {
	if ts.ClosedTrades == 0 {
		return 0
	}
	return float64(ts.WinningTrades) / float64(ts.ClosedTrades) * 100
}

// KillSwitchStatus represents the kill switch state
type KillSwitchStatus struct {
	Enabled   bool       `json:"enabled"`
	Reason    *string    `json:"reason"`
	Timestamp *time.Time `json:"timestamp"`
}
