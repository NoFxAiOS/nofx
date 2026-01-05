package market

import (
	"fmt"
	"sync"
	"time"
)

// WebSocketClient defines the interface for exchange WebSocket connections
type WebSocketClient interface {
	// Connect establishes the WebSocket connection
	Connect() error

	// Disconnect closes the WebSocket connection
	Disconnect() error

	// IsConnected returns whether the connection is active
	IsConnected() bool

	// SubscribeKlines subscribes to kline updates for a symbol
	SubscribeKlines(symbol, interval string) error

	// UnsubscribeKlines unsubscribes from kline updates
	UnsubscribeKlines(symbol, interval string) error

	// SubscribeOrderUpdates subscribes to real-time order updates
	SubscribeOrderUpdates() error

	// UnsubscribeOrderUpdates unsubscribes from order updates
	UnsubscribeOrderUpdates() error

	// GetKlineChannel returns a channel that receives kline updates
	GetKlineChannel() <-chan KlineUpdate

	// GetOrderUpdateChannel returns a channel that receives order updates
	GetOrderUpdateChannel() <-chan OrderUpdate

	// Reconnect attempts to reconnect the WebSocket
	Reconnect() error
}

// KlineUpdate represents a real-time kline update
type KlineUpdate struct {
	Symbol      string
	Interval    string // "1m", "5m", "15m", "1h", "4h", etc.
	OpenTime    int64  // Unix timestamp in milliseconds
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      float64
	CloseTime   int64
	QuoteVolume float64
	NumTrades   int64
	IsClosed    bool      // Whether this candle is closed
	Timestamp   time.Time // When the update was received
}

// OrderUpdate represents a real-time order or trade update
type OrderUpdate struct {
	Symbol             string
	OrderID            string
	ClientOrderID      string
	Side               string // BUY or SELL
	PositionSide       string // LONG or SHORT (for futures)
	OrderType          string // LIMIT, MARKET, etc.
	TimeInForce        string // GTC, IOC, FOK
	OriginalQuantity   float64
	ExecutedQuantity   float64
	CumulativeQuoteQty float64
	Status             string // NEW, PARTIALLY_FILLED, FILLED, CANCELED, etc.
	ExecutionType      string // NEW, PARTIAL_FILL, FILL, CANCELED, REJECTED, EXPIRED
	RejectReason       string // Error message if rejected
	OrderPrice         float64
	AveragePrice       float64
	Timestamp          time.Time
}

// WebSocketManager manages multiple WebSocket connections with fallback to REST
type WebSocketManager struct {
	mu                sync.RWMutex
	clients           map[string]WebSocketClient  // Exchange → client
	klineBuffer       map[string]chan KlineUpdate // Symbol → channel
	orderBuffer       map[string]chan OrderUpdate // Symbol → channel
	fallbackRestAPI   bool                        // Use REST API if WebSocket fails
	reconnectAttempts int
	reconnectDelay    time.Duration
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients:           make(map[string]WebSocketClient),
		klineBuffer:       make(map[string]chan KlineUpdate),
		orderBuffer:       make(map[string]chan OrderUpdate),
		fallbackRestAPI:   true,
		reconnectAttempts: 5,
		reconnectDelay:    5 * time.Second,
	}
}

// RegisterClient registers a WebSocket client for an exchange
func (wm *WebSocketManager) RegisterClient(exchange string, client WebSocketClient) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if client == nil {
		return fmt.Errorf("cannot register nil client for exchange %s", exchange)
	}

	wm.clients[exchange] = client
	return nil
}

// GetClient returns the WebSocket client for an exchange
func (wm *WebSocketManager) GetClient(exchange string) WebSocketClient {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	return wm.clients[exchange]
}

// SubscribeSymbolKlines subscribes to kline updates for a symbol
func (wm *WebSocketManager) SubscribeSymbolKlines(exchange, symbol, interval string) error {
	client := wm.GetClient(exchange)
	if client == nil {
		if wm.fallbackRestAPI {
			return nil // Use REST API instead
		}
		return fmt.Errorf("no WebSocket client registered for exchange %s", exchange)
	}

	if !client.IsConnected() {
		if err := client.Connect(); err != nil {
			if wm.fallbackRestAPI {
				return nil
			}
			return fmt.Errorf("failed to connect WebSocket for %s: %w", exchange, err)
		}
	}

	return client.SubscribeKlines(symbol, interval)
}

// UnsubscribeSymbolKlines unsubscribes from kline updates
func (wm *WebSocketManager) UnsubscribeSymbolKlines(exchange, symbol, interval string) error {
	client := wm.GetClient(exchange)
	if client == nil {
		return nil
	}

	return client.UnsubscribeKlines(symbol, interval)
}

// SubscribeOrderUpdates subscribes to order updates for an exchange
func (wm *WebSocketManager) SubscribeOrderUpdates(exchange string) error {
	client := wm.GetClient(exchange)
	if client == nil {
		if wm.fallbackRestAPI {
			return nil
		}
		return fmt.Errorf("no WebSocket client registered for exchange %s", exchange)
	}

	if !client.IsConnected() {
		if err := client.Connect(); err != nil {
			if wm.fallbackRestAPI {
				return nil
			}
			return fmt.Errorf("failed to connect WebSocket for %s: %w", exchange, err)
		}
	}

	return client.SubscribeOrderUpdates()
}

// GetKlineUpdates returns a channel for receiving kline updates
func (wm *WebSocketManager) GetKlineUpdates(exchange string) <-chan KlineUpdate {
	client := wm.GetClient(exchange)
	if client == nil {
		return nil
	}

	return client.GetKlineChannel()
}

// GetOrderUpdates returns a channel for receiving order updates
func (wm *WebSocketManager) GetOrderUpdates(exchange string) <-chan OrderUpdate {
	client := wm.GetClient(exchange)
	if client == nil {
		return nil
	}

	return client.GetOrderUpdateChannel()
}

// DisconnectAll closes all WebSocket connections
func (wm *WebSocketManager) DisconnectAll() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	var lastErr error
	for _, client := range wm.clients {
		if err := client.Disconnect(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// SetFallbackRestAPI enables or disables fallback to REST API on WebSocket failure
func (wm *WebSocketManager) SetFallbackRestAPI(enabled bool) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.fallbackRestAPI = enabled
}

// CheckConnections verifies all registered clients are connected, attempting reconnect if not
func (wm *WebSocketManager) CheckConnections() error {
	wm.mu.RLock()
	clients := make(map[string]WebSocketClient)
	for k, v := range wm.clients {
		clients[k] = v
	}
	wm.mu.RUnlock()

	var errors []error
	for exchange, client := range clients {
		if !client.IsConnected() {
			if err := client.Reconnect(); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", exchange, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("WebSocket connection errors: %v", errors)
	}

	return nil
}
