package exchange

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/crypto-trading-bot/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// PaperExchange simulates an exchange for paper trading
type PaperExchange struct {
	name             string
	balances         map[string]*Balance
	orders           map[string]*OrderResponse
	currentPrices    map[string]decimal.Decimal
	slippagePercent  decimal.Decimal
	takerFeePercent  decimal.Decimal
	makerFeePercent  decimal.Decimal
	mu               sync.RWMutex
	logger           *logrus.Logger
	priceCallbacks   []func(*PriceUpdate)
	priceCallbacksMu sync.RWMutex
}

// NewPaperExchange creates a new paper trading exchange
func NewPaperExchange(name string, initialBalance decimal.Decimal, logger *logrus.Logger) *PaperExchange {
	return &PaperExchange{
		name: name,
		balances: map[string]*Balance{
			"USD": {
				Currency:  "USD",
				Available: initialBalance,
				Locked:    decimal.Zero,
				Total:     initialBalance,
			},
		},
		orders:          make(map[string]*OrderResponse),
		currentPrices:   make(map[string]decimal.Decimal),
		slippagePercent: decimal.NewFromFloat(0.05), // 0.05% slippage
		takerFeePercent: decimal.NewFromFloat(0.4),  // 0.4% taker fee
		makerFeePercent: decimal.NewFromFloat(0.25), // 0.25% maker fee
		logger:          logger,
		priceCallbacks:  make([]func(*PriceUpdate), 0),
	}
}

// Name returns the exchange name
func (pe *PaperExchange) Name() string {
	return pe.name
}

// PlaceOrder places a simulated order
func (pe *PaperExchange) PlaceOrder(ctx context.Context, req *OrderRequest) (*OrderResponse, error) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Get current price
	currentPrice, exists := pe.currentPrices[req.Symbol]
	if !exists {
		return nil, fmt.Errorf("no price available for symbol %s", req.Symbol)
	}

	// Calculate execution price with slippage
	executionPrice := pe.calculateExecutionPrice(currentPrice, req.Side, req.Type)

	// Calculate required funds
	totalCost := executionPrice.Mul(req.Quantity)

	// Calculate fees (taker fee for market orders, maker fee for limit orders)
	feePercent := pe.takerFeePercent
	if req.Type == models.OrderTypeLimit {
		feePercent = pe.makerFeePercent
	}
	fees := totalCost.Mul(feePercent).Div(decimal.NewFromInt(100))

	// Check if we have sufficient balance
	if req.Side == models.OrderSideBuy {
		totalRequired := totalCost.Add(fees)
		if pe.balances["USD"].Available.LessThan(totalRequired) {
			return nil, fmt.Errorf("insufficient balance: need %s, have %s",
				totalRequired.String(), pe.balances["USD"].Available.String())
		}

		// Deduct from USD balance
		pe.balances["USD"].Available = pe.balances["USD"].Available.Sub(totalRequired)
		pe.balances["USD"].Total = pe.balances["USD"].Available.Add(pe.balances["USD"].Locked)

		// Add to asset balance
		baseCurrency := pe.getBaseCurrency(req.Symbol)
		if _, exists := pe.balances[baseCurrency]; !exists {
			pe.balances[baseCurrency] = &Balance{
				Currency:  baseCurrency,
				Available: decimal.Zero,
				Locked:    decimal.Zero,
				Total:     decimal.Zero,
			}
		}
		pe.balances[baseCurrency].Available = pe.balances[baseCurrency].Available.Add(req.Quantity)
		pe.balances[baseCurrency].Total = pe.balances[baseCurrency].Available.Add(pe.balances[baseCurrency].Locked)

	} else { // SELL
		baseCurrency := pe.getBaseCurrency(req.Symbol)
		if pe.balances[baseCurrency].Available.LessThan(req.Quantity) {
			return nil, fmt.Errorf("insufficient %s balance: need %s, have %s",
				baseCurrency, req.Quantity.String(), pe.balances[baseCurrency].Available.String())
		}

		// Deduct from asset balance
		pe.balances[baseCurrency].Available = pe.balances[baseCurrency].Available.Sub(req.Quantity)
		pe.balances[baseCurrency].Total = pe.balances[baseCurrency].Available.Add(pe.balances[baseCurrency].Locked)

		// Add to USD balance (minus fees)
		usdReceived := totalCost.Sub(fees)
		pe.balances["USD"].Available = pe.balances["USD"].Available.Add(usdReceived)
		pe.balances["USD"].Total = pe.balances["USD"].Available.Add(pe.balances["USD"].Locked)
	}

	// Create order response
	orderID := uuid.New().String()
	clientOrderID := uuid.New().String()
	now := time.Now()

	order := &OrderResponse{
		ID:               orderID,
		ClientOrderID:    clientOrderID,
		ExchangeOrderID:  orderID,
		Symbol:           req.Symbol,
		Side:             req.Side,
		Type:             req.Type,
		Status:           models.OrderStatusFilled, // Paper orders fill immediately
		Quantity:         req.Quantity,
		Price:            &executionPrice,
		FilledQuantity:   req.Quantity,
		AverageFillPrice: &executionPrice,
		Fees:             fees,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	pe.orders[orderID] = order

	pe.logger.WithFields(logrus.Fields{
		"order_id":        orderID,
		"symbol":          req.Symbol,
		"side":            req.Side,
		"quantity":        req.Quantity.String(),
		"execution_price": executionPrice.String(),
		"fees":            fees.String(),
	}).Info("Paper order executed")

	return order, nil
}

// CancelOrder cancels an order (no-op for paper trading)
func (pe *PaperExchange) CancelOrder(ctx context.Context, orderID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	order, exists := pe.orders[orderID]
	if !exists {
		return fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status == models.OrderStatusFilled {
		return fmt.Errorf("cannot cancel filled order")
	}

	order.Status = models.OrderStatusCancelled
	order.UpdatedAt = time.Now()

	return nil
}

// GetOrder gets an order by ID
func (pe *PaperExchange) GetOrder(ctx context.Context, orderID string) (*OrderResponse, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	order, exists := pe.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return order, nil
}

// GetBalance gets account balances
func (pe *PaperExchange) GetBalance(ctx context.Context) (map[string]*Balance, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// Return a copy to avoid external modification
	balances := make(map[string]*Balance)
	for k, v := range pe.balances {
		balances[k] = &Balance{
			Currency:  v.Currency,
			Available: v.Available,
			Locked:    v.Locked,
			Total:     v.Total,
		}
	}

	return balances, nil
}

// GetPrice gets the current price for a symbol
func (pe *PaperExchange) GetPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	price, exists := pe.currentPrices[symbol]
	if !exists {
		return decimal.Zero, fmt.Errorf("no price available for symbol %s", symbol)
	}

	return price, nil
}

// UpdatePrice updates the current price for a symbol (used by market data service)
func (pe *PaperExchange) UpdatePrice(symbol string, price decimal.Decimal) {
	pe.mu.Lock()
	pe.currentPrices[symbol] = price
	pe.mu.Unlock()

	// Notify callbacks
	pe.priceCallbacksMu.RLock()
	callbacks := make([]func(*PriceUpdate), len(pe.priceCallbacks))
	copy(callbacks, pe.priceCallbacks)
	pe.priceCallbacksMu.RUnlock()

	update := &PriceUpdate{
		Exchange:  pe.name,
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now(),
	}

	for _, callback := range callbacks {
		go callback(update)
	}
}

// SubscribePriceUpdates subscribes to price updates
func (pe *PaperExchange) SubscribePriceUpdates(ctx context.Context, symbols []string, callback func(*PriceUpdate)) error {
	pe.priceCallbacksMu.Lock()
	defer pe.priceCallbacksMu.Unlock()

	pe.priceCallbacks = append(pe.priceCallbacks, callback)

	pe.logger.WithField("symbols", symbols).Info("Subscribed to price updates")

	return nil
}

// Close closes the exchange connection (no-op for paper trading)
func (pe *PaperExchange) Close() error {
	pe.logger.Info("Paper exchange closed")
	return nil
}

// Helper methods

func (pe *PaperExchange) calculateExecutionPrice(price decimal.Decimal, side models.OrderSide, orderType models.OrderType) decimal.Decimal {
	if orderType == models.OrderTypeLimit {
		// Limit orders execute at the limit price (no slippage)
		return price
	}

	// Market orders have slippage
	slippage := price.Mul(pe.slippagePercent).Div(decimal.NewFromInt(100))
	if side == models.OrderSideBuy {
		// Buying costs more (slippage against us)
		return price.Add(slippage)
	}
	// Selling gets less (slippage against us)
	return price.Sub(slippage)
}

func (pe *PaperExchange) getBaseCurrency(symbol string) string {
	// Simple implementation - extract base currency from symbol
	// e.g., "BTC-USD" -> "BTC"
	for i := 0; i < len(symbol); i++ {
		if symbol[i] == '-' || symbol[i] == '/' {
			return symbol[:i]
		}
	}
	return symbol
}
