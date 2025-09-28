package exchange

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/crypto-trading-bot/internal/models"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

const (
	coinbaseAPIURL        = "https://api.coinbase.com"
	coinbaseSandboxAPIURL = "https://api-public.sandbox.exchange.coinbase.com"
	coinbaseWSURL         = "wss://ws-feed.exchange.coinbase.com"
	coinbaseSandboxWSURL  = "wss://ws-feed-public.sandbox.exchange.coinbase.com"
)

// CoinbaseExchange implements the Exchange interface for Coinbase Advanced Trade
type CoinbaseExchange struct {
	apiKey      string
	apiSecret   string
	passphrase  string
	baseURL     string
	wsURL       string
	client      *http.Client
	wsConn      *websocket.Conn
	logger      *logrus.Logger
	callbacks   []func(*PriceUpdate)
	callbacksMu sync.RWMutex
}

// NewCoinbaseExchange creates a new Coinbase exchange connector
func NewCoinbaseExchange(apiKey, apiSecret, passphrase string, sandbox bool, logger *logrus.Logger) *CoinbaseExchange {
	baseURL := coinbaseAPIURL
	wsURL := coinbaseWSURL
	
	if sandbox {
		baseURL = coinbaseSandboxAPIURL
		wsURL = coinbaseSandboxWSURL
	}

	return &CoinbaseExchange{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		passphrase: passphrase,
		baseURL:    baseURL,
		wsURL:      wsURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:    logger,
		callbacks: make([]func(*PriceUpdate), 0),
	}
}

// Name returns the exchange name
func (ce *CoinbaseExchange) Name() string {
	return "coinbase"
}

// PlaceOrder places an order on Coinbase
func (ce *CoinbaseExchange) PlaceOrder(ctx context.Context, req *OrderRequest) (*OrderResponse, error) {
	// Build order request
	orderReq := map[string]interface{}{
		"product_id": req.Symbol,
		"side":       string(req.Side),
		"type":       string(req.Type),
	}

	if req.Type == models.OrderTypeMarket {
		// Market order - specify size
		orderReq["size"] = req.Quantity.String()
	} else {
		// Limit order - specify price and size
		if req.Price == nil {
			return nil, fmt.Errorf("price required for limit orders")
		}
		orderReq["price"] = req.Price.String()
		orderReq["size"] = req.Quantity.String()
	}

	// Make API request
	var response map[string]interface{}
	if err := ce.makeRequest(ctx, "POST", "/orders", orderReq, &response); err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	// Parse response
	orderResp := &OrderResponse{
		ID:              response["id"].(string),
		ExchangeOrderID: response["id"].(string),
		ClientOrderID:   response["id"].(string),
		Symbol:          req.Symbol,
		Side:            req.Side,
		Type:            req.Type,
		Status:          ce.parseOrderStatus(response["status"].(string)),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Parse quantity
	if size, ok := response["size"].(string); ok {
		if qty, err := decimal.NewFromString(size); err == nil {
			orderResp.Quantity = qty
		}
	}

	// Parse filled quantity
	if filled, ok := response["filled_size"].(string); ok {
		if qty, err := decimal.NewFromString(filled); err == nil {
			orderResp.FilledQuantity = qty
		}
	}

	// Parse fees
	if fillFees, ok := response["fill_fees"].(string); ok {
		if fees, err := decimal.NewFromString(fillFees); err == nil {
			orderResp.Fees = fees
		}
	}

	ce.logger.WithFields(logrus.Fields{
		"order_id": orderResp.ID,
		"symbol":   req.Symbol,
		"side":     req.Side,
		"type":     req.Type,
	}).Info("Order placed on Coinbase")

	return orderResp, nil
}

// CancelOrder cancels an order
func (ce *CoinbaseExchange) CancelOrder(ctx context.Context, orderID string) error {
	endpoint := fmt.Sprintf("/orders/%s", orderID)
	if err := ce.makeRequest(ctx, "DELETE", endpoint, nil, nil); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	ce.logger.WithField("order_id", orderID).Info("Order cancelled on Coinbase")

	return nil
}

// GetOrder gets an order by ID
func (ce *CoinbaseExchange) GetOrder(ctx context.Context, orderID string) (*OrderResponse, error) {
	endpoint := fmt.Sprintf("/orders/%s", orderID)
	
	var response map[string]interface{}
	if err := ce.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Parse response (similar to PlaceOrder)
	orderResp := &OrderResponse{
		ID:              response["id"].(string),
		ExchangeOrderID: response["id"].(string),
		Symbol:          response["product_id"].(string),
		Status:          ce.parseOrderStatus(response["status"].(string)),
	}

	return orderResp, nil
}

// GetBalance gets account balances
func (ce *CoinbaseExchange) GetBalance(ctx context.Context) (map[string]*Balance, error) {
	var accounts []map[string]interface{}
	if err := ce.makeRequest(ctx, "GET", "/accounts", nil, &accounts); err != nil {
		return nil, fmt.Errorf("failed to get balances: %w", err)
	}

	balances := make(map[string]*Balance)
	for _, acc := range accounts {
		currency := acc["currency"].(string)
		
		available, _ := decimal.NewFromString(acc["available"].(string))
		hold, _ := decimal.NewFromString(acc["hold"].(string))
		
		balances[currency] = &Balance{
			Currency:  currency,
			Available: available,
			Locked:    hold,
			Total:     available.Add(hold),
		}
	}

	return balances, nil
}

// GetPrice gets the current price for a symbol
func (ce *CoinbaseExchange) GetPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	endpoint := fmt.Sprintf("/products/%s/ticker", symbol)
	
	var response map[string]interface{}
	if err := ce.makeRequest(ctx, "GET", endpoint, nil, &response); err != nil {
		return decimal.Zero, fmt.Errorf("failed to get price: %w", err)
	}

	priceStr := response["price"].(string)
	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to parse price: %w", err)
	}

	return price, nil
}

// SubscribePriceUpdates subscribes to price updates via WebSocket
func (ce *CoinbaseExchange) SubscribePriceUpdates(ctx context.Context, symbols []string, callback func(*PriceUpdate)) error {
	ce.callbacksMu.Lock()
	ce.callbacks = append(ce.callbacks, callback)
	ce.callbacksMu.Unlock()

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(ce.wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	ce.wsConn = conn

	// Subscribe to ticker channel
	subscribeMsg := map[string]interface{}{
		"type":        "subscribe",
		"product_ids": symbols,
		"channels":    []string{"ticker"},
	}

	if err := conn.WriteJSON(subscribeMsg); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Start listening for updates
	go ce.listenWebSocket(ctx)

	ce.logger.WithField("symbols", symbols).Info("Subscribed to Coinbase price updates")

	return nil
}

// Close closes the exchange connection
func (ce *CoinbaseExchange) Close() error {
	if ce.wsConn != nil {
		ce.wsConn.Close()
	}
	ce.logger.Info("Coinbase exchange closed")
	return nil
}

// Helper methods

func (ce *CoinbaseExchange) makeRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	url := ce.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := timestamp + method + endpoint + string(bodyBytes)
	signature := ce.generateSignature(message)

	req.Header.Set("CB-ACCESS-KEY", ce.apiKey)
	req.Header.Set("CB-ACCESS-SIGN", signature)
	req.Header.Set("CB-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("CB-ACCESS-PASSPHRASE", ce.passphrase)
	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := ce.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

func (ce *CoinbaseExchange) generateSignature(message string) string {
	key, _ := base64.StdEncoding.DecodeString(ce.apiSecret)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (ce *CoinbaseExchange) parseOrderStatus(status string) models.OrderStatus {
	switch status {
	case "pending":
		return models.OrderStatusPending
	case "open", "active":
		return models.OrderStatusOpen
	case "done", "filled":
		return models.OrderStatusFilled
	case "cancelled":
		return models.OrderStatusCancelled
	default:
		return models.OrderStatusFailed
	}
}

func (ce *CoinbaseExchange) listenWebSocket(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg map[string]interface{}
			if err := ce.wsConn.ReadJSON(&msg); err != nil {
				ce.logger.WithError(err).Error("WebSocket read error")
				// Implement reconnection logic here
				time.Sleep(5 * time.Second)
				continue
			}

			// Process ticker messages
			if msg["type"] == "ticker" {
				ce.processTicker(msg)
			}
		}
	}
}

func (ce *CoinbaseExchange) processTicker(msg map[string]interface{}) {
	symbol := msg["product_id"].(string)
	priceStr := msg["price"].(string)
	
	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		ce.logger.WithError(err).Error("Failed to parse price")
		return
	}

	update := &PriceUpdate{
		Exchange:  "coinbase",
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now(),
	}

	// Notify all callbacks
	ce.callbacksMu.RLock()
	callbacks := make([]func(*PriceUpdate), len(ce.callbacks))
	copy(callbacks, ce.callbacks)
	ce.callbacksMu.RUnlock()

	for _, callback := range callbacks {
		go callback(update)
	}
}

