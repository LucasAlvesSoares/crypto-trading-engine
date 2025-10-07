package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event
type EventType string

const (
	// Market data events
	EventTypePriceUpdate EventType = "market.price.update"

	// Order events
	EventTypeOrderPlaced    EventType = "order.placed"
	EventTypeOrderFilled    EventType = "order.filled"
	EventTypeOrderCancelled EventType = "order.cancelled"
	EventTypeOrderFailed    EventType = "order.failed"

	// Trade events
	EventTypeTradeSignal EventType = "strategy.signal"
	EventTypeTradeOpened EventType = "trade.opened"
	EventTypeTradeClosed EventType = "trade.closed"

	// Risk events
	EventTypeRiskViolation EventType = "risk.violation"
	EventTypeKillSwitch    EventType = "risk.kill_switch"

	// System events
	EventTypeSystemError  EventType = "system.error"
	EventTypeSystemHealth EventType = "system.health"
)

// NATS subjects for publishing/subscribing
const (
	SubjectPriceUpdates = "market.price.>"
	SubjectOrders       = "order.>"
	SubjectTradeSignals = "strategy.signal"
	SubjectRiskEvents   = "risk.>"
	SubjectSystemEvents = "system.>"
)

// Event is the base event structure
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// NewEvent creates a new event
func NewEvent(eventType EventType, data interface{}) (*Event, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      dataBytes,
	}, nil
}

// PriceUpdateEvent represents a price update event
type PriceUpdateEvent struct {
	Exchange string    `json:"exchange"`
	Symbol   string    `json:"symbol"`
	Price    float64   `json:"price"`
	Volume   float64   `json:"volume"`
	Time     time.Time `json:"time"`
}

// OrderPlacedEvent represents an order placed event
type OrderPlacedEvent struct {
	OrderID         string   `json:"order_id"`
	ClientOrderID   string   `json:"client_order_id"`
	ExchangeOrderID string   `json:"exchange_order_id"`
	StrategyID      string   `json:"strategy_id"`
	Symbol          string   `json:"symbol"`
	Side            string   `json:"side"`
	Type            string   `json:"type"`
	Quantity        float64  `json:"quantity"`
	Price           *float64 `json:"price,omitempty"`
	StopLossPrice   *float64 `json:"stop_loss_price,omitempty"`
}

// OrderFilledEvent represents an order filled event
type OrderFilledEvent struct {
	OrderID          string    `json:"order_id"`
	ClientOrderID    string    `json:"client_order_id"`
	ExchangeOrderID  string    `json:"exchange_order_id"`
	Symbol           string    `json:"symbol"`
	Side             string    `json:"side"`
	FilledQuantity   float64   `json:"filled_quantity"`
	AverageFillPrice float64   `json:"average_fill_price"`
	Fees             float64   `json:"fees"`
	FilledAt         time.Time `json:"filled_at"`
}

// TradeSignalEvent represents a trade signal event
type TradeSignalEvent struct {
	ID            string             `json:"id"`
	StrategyID    string             `json:"strategy_id"`
	Symbol        string             `json:"symbol"`
	Side          string             `json:"side"`
	Type          string             `json:"type"`
	Quantity      float64            `json:"quantity"`
	Price         *float64           `json:"price,omitempty"`
	StopLossPrice float64            `json:"stop_loss_price"`
	Reason        string             `json:"reason"`
	Indicators    map[string]float64 `json:"indicators"`
}

// TradeOpenedEvent represents a trade opened event
type TradeOpenedEvent struct {
	TradeID    string    `json:"trade_id"`
	StrategyID string    `json:"strategy_id"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	EntryPrice float64   `json:"entry_price"`
	Quantity   float64   `json:"quantity"`
	EntryTime  time.Time `json:"entry_time"`
}

// TradeClosedEvent represents a trade closed event
type TradeClosedEvent struct {
	TradeID      string    `json:"trade_id"`
	StrategyID   string    `json:"strategy_id"`
	Symbol       string    `json:"symbol"`
	EntryPrice   float64   `json:"entry_price"`
	ExitPrice    float64   `json:"exit_price"`
	Quantity     float64   `json:"quantity"`
	PnL          float64   `json:"pnl"`
	PnLPercent   float64   `json:"pnl_percent"`
	ExitReason   string    `json:"exit_reason"`
	ExitTime     time.Time `json:"exit_time"`
	HoldDuration string    `json:"hold_duration"`
}

// RiskViolationEvent represents a risk violation event
type RiskViolationEvent struct {
	StrategyID  string                 `json:"strategy_id"`
	EventType   string                 `json:"event_type"`
	Description string                 `json:"description"`
	ActionTaken string                 `json:"action_taken"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// KillSwitchEvent represents a kill switch event
type KillSwitchEvent struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

// SystemErrorEvent represents a system error event
type SystemErrorEvent struct {
	Component string `json:"component"`
	Error     string `json:"error"`
	Severity  string `json:"severity"`
}

// SystemHealthEvent represents a system health event
type SystemHealthEvent struct {
	Component string                 `json:"component"`
	Status    string                 `json:"status"`
	Metadata  map[string]interface{} `json:"metadata"`
}
