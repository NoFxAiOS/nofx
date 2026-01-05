package market

import (
	"fmt"
	"github.com/gorilla/websocket"
	"nofx/logger"
	"sync"
	"time"
)

// BinanceWebSocketClient implements WebSocketClient for Binance
type BinanceWebSocketClient struct {
	mu                 sync.RWMutex
	conn               *websocket.Conn
	url                string
	isConnected        bool
	subscriptions      map[string]bool // subscription key -> subscribed
	klineChannel       chan KlineUpdate
	orderUpdateChannel chan OrderUpdate
	stopCh             chan struct{}
	reconnectAttempts  int
	reconnectDelay     time.Duration
	lastActivity       time.Time
	heartbeatTicker    *time.Ticker
	testnet            bool
}

// NewBinanceWebSocketClient creates a new Binance WebSocket client
func NewBinanceWebSocketClient(testnet bool) *BinanceWebSocketClient {
	url := "wss://fstream.binance.com/ws"
	if testnet {
		url = "wss://stream.binancefuture.com/ws"
	}

	return &BinanceWebSocketClient{
		url:                url,
		isConnected:        false,
		subscriptions:      make(map[string]bool),
		klineChannel:       make(chan KlineUpdate, 100),
		orderUpdateChannel: make(chan OrderUpdate, 100),
		stopCh:             make(chan struct{}),
		reconnectAttempts:  5,
		reconnectDelay:     5 * time.Second,
		testnet:            testnet,
	}
}

// Connect establishes the WebSocket connection
func (c *BinanceWebSocketClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isConnected {
		return nil
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Binance WebSocket: %w", err)
	}

	c.conn = conn
	c.isConnected = true
	c.lastActivity = time.Now()

	logger.Infof("✓ Binance WebSocket connected (testnet=%v)", c.testnet)

	// Start message reader
	go c.readMessages()

	// Start heartbeat
	go c.heartbeat()

	return nil
}

// Disconnect closes the WebSocket connection
func (c *BinanceWebSocketClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected {
		return nil
	}

	c.isConnected = false
	close(c.stopCh)

	if c.conn != nil {
		c.conn.Close()
	}

	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
	}

	logger.Info("✓ Binance WebSocket disconnected")
	return nil
}

// IsConnected returns whether the connection is active
func (c *BinanceWebSocketClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isConnected && c.conn != nil
}

// SubscribeKlines subscribes to kline updates for a symbol
func (c *BinanceWebSocketClient) SubscribeKlines(symbol, interval string) error {
	if !c.IsConnected() {
		return fmt.Errorf("WebSocket not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Format: btcusdt@kline_1m
	subscriptionKey := fmt.Sprintf("%s@kline_%s", symbol, interval)
	if c.subscriptions[subscriptionKey] {
		return nil // Already subscribed
	}

	// Subscribe message
	msg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": []string{subscriptionKey},
		"id":     time.Now().Unix(),
	}

	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", subscriptionKey, err)
	}

	c.subscriptions[subscriptionKey] = true
	logger.Infof("✓ Subscribed to Binance klines: %s", subscriptionKey)
	return nil
}

// UnsubscribeKlines unsubscribes from kline updates
func (c *BinanceWebSocketClient) UnsubscribeKlines(symbol, interval string) error {
	if !c.IsConnected() {
		return fmt.Errorf("WebSocket not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	subscriptionKey := fmt.Sprintf("%s@kline_%s", symbol, interval)
	if !c.subscriptions[subscriptionKey] {
		return nil // Not subscribed
	}

	msg := map[string]interface{}{
		"method": "UNSUBSCRIBE",
		"params": []string{subscriptionKey},
		"id":     time.Now().Unix(),
	}

	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to unsubscribe from %s: %w", subscriptionKey, err)
	}

	delete(c.subscriptions, subscriptionKey)
	logger.Infof("✓ Unsubscribed from Binance klines: %s", subscriptionKey)
	return nil
}

// SubscribeOrderUpdates subscribes to real-time order updates
func (c *BinanceWebSocketClient) SubscribeOrderUpdates() error {
	if !c.IsConnected() {
		return fmt.Errorf("WebSocket not connected")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// For Binance, order updates come via dedicated account stream
	// This requires listen key from REST API
	// For now, we'll return success but note this requires separate implementation
	logger.Info("⚠️ Binance order updates require ListenKey from REST API - not yet implemented")
	return nil
}

// UnsubscribeOrderUpdates unsubscribes from order updates
func (c *BinanceWebSocketClient) UnsubscribeOrderUpdates() error {
	return nil
}

// GetKlineChannel returns a channel that receives kline updates
func (c *BinanceWebSocketClient) GetKlineChannel() <-chan KlineUpdate {
	return c.klineChannel
}

// GetOrderUpdateChannel returns a channel that receives order updates
func (c *BinanceWebSocketClient) GetOrderUpdateChannel() <-chan OrderUpdate {
	return c.orderUpdateChannel
}

// Reconnect attempts to reconnect the WebSocket
func (c *BinanceWebSocketClient) Reconnect() error {
	c.Disconnect()
	time.Sleep(c.reconnectDelay)
	return c.Connect()
}

// readMessages reads messages from the WebSocket and processes them
func (c *BinanceWebSocketClient) readMessages() {
	defer func() {
		c.mu.Lock()
		c.isConnected = false
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		var message map[string]interface{}
		if err := conn.ReadJSON(&message); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warnf("WebSocket error: %v", err)
			}
			return
		}

		c.mu.Lock()
		c.lastActivity = time.Now()
		c.mu.Unlock()

		// Process different message types
		if stream, ok := message["stream"].(string); ok {
			switch stream {
			case "":
				// Response to subscription/unsubscription
				continue
			default:
				// Check if it's a kline message
				if data, ok := message["data"].(map[string]interface{}); ok {
					if kline, ok := data["k"].(map[string]interface{}); ok {
						c.processKlineData(kline, stream)
					}
				}
			}
		}
	}
}

// processKlineData processes a kline data message
func (c *BinanceWebSocketClient) processKlineData(kline map[string]interface{}, stream string) {
	// Parse Binance kline format
	update := KlineUpdate{
		Timestamp: time.Now(),
	}

	// Extract symbol from stream (e.g., "btcusdt@kline_1m")
	if s, ok := kline["s"].(string); ok {
		update.Symbol = s
	}

	// Interval
	if i, ok := kline["i"].(string); ok {
		update.Interval = i
	}

	// Open time (milliseconds)
	if t, ok := kline["t"].(float64); ok {
		update.OpenTime = int64(t)
	}

	// Open price
	if o, ok := kline["o"].(string); ok {
		fmt.Sscanf(o, "%f", &update.Open)
	}

	// High price
	if h, ok := kline["h"].(string); ok {
		fmt.Sscanf(h, "%f", &update.High)
	}

	// Low price
	if l, ok := kline["l"].(string); ok {
		fmt.Sscanf(l, "%f", &update.Low)
	}

	// Close price
	if c, ok := kline["c"].(string); ok {
		fmt.Sscanf(c, "%f", &update.Close)
	}

	// Volume
	if v, ok := kline["v"].(string); ok {
		fmt.Sscanf(v, "%f", &update.Volume)
	}

	// Close time
	if ct, ok := kline["T"].(float64); ok {
		update.CloseTime = int64(ct)
	}

	// Is closed
	if x, ok := kline["x"].(bool); ok {
		update.IsClosed = x
	}

	// Send to channel
	select {
	case c.klineChannel <- update:
	case <-c.stopCh:
	default:
		// Channel full, drop message
	}
}

// heartbeat sends periodic ping to keep connection alive
func (c *BinanceWebSocketClient) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()

			if conn != nil {
				if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					logger.Warnf("Failed to send ping: %v", err)
					return
				}
			}
		case <-c.stopCh:
			return
		}
	}
}
