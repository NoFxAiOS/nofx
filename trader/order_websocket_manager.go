package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"sync"
	"time"
)

// OrderWebSocketManager manages order WebSocket connections and publishes events
type OrderWebSocketManager struct {
	mu                  sync.RWMutex
	binanceOrderWS      *market.BinanceOrderWebSocket
	bybitOrderWS        *market.BybitOrderWebSocket
	okxOrderWS          *market.OKXOrderWebSocket
	activeConnections   map[string]bool
	eventBus            *EventBus
	orderUpdateHandlers []func(order market.OrderUpdate)
}

// NewOrderWebSocketManager creates a new manager
func NewOrderWebSocketManager(eventBus *EventBus) *OrderWebSocketManager {
	return &OrderWebSocketManager{
		activeConnections:   make(map[string]bool),
		eventBus:            eventBus,
		orderUpdateHandlers: make([]func(order market.OrderUpdate), 0),
	}
}

// StartBinanceOrderStream starts the Binance order WebSocket with ListenKey
func (m *OrderWebSocketManager) StartBinanceOrderStream(listenKey string, testnet bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.binanceOrderWS != nil && m.binanceOrderWS.IsConnected() {
		return fmt.Errorf("Binance order stream already running")
	}

	m.binanceOrderWS = market.NewBinanceOrderWebSocket(testnet)
	if err := m.binanceOrderWS.SetListenKey(listenKey); err != nil {
		return fmt.Errorf("failed to set listen key: %w", err)
	}

	if err := m.binanceOrderWS.Connect(); err != nil {
		return fmt.Errorf("failed to connect Binance order stream: %w", err)
	}

	m.activeConnections["binance"] = true

	// Start listening for order updates
	go m.handleBinanceOrderUpdates()

	logger.Infof("✓ Binance order WebSocket stream started")
	return nil
}

// StartBybitOrderStream starts the Bybit order WebSocket
func (m *OrderWebSocketManager) StartBybitOrderStream(apiKey, apiSecret string, testnet bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.bybitOrderWS != nil && m.bybitOrderWS.IsConnected() {
		return fmt.Errorf("Bybit order stream already running")
	}

	m.bybitOrderWS = market.NewBybitOrderWebSocket(testnet, apiKey, apiSecret)
	if err := m.bybitOrderWS.Connect(); err != nil {
		return fmt.Errorf("failed to connect Bybit order stream: %w", err)
	}

	m.activeConnections["bybit"] = true

	// Start listening for order updates
	go m.handleBybitOrderUpdates()

	logger.Infof("✓ Bybit order WebSocket stream started")
	return nil
}

// StartOKXOrderStream starts the OKX order WebSocket
func (m *OrderWebSocketManager) StartOKXOrderStream(apiKey, apiSecret, passphrase string, testnet bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.okxOrderWS != nil && m.okxOrderWS.IsConnected() {
		return fmt.Errorf("OKX order stream already running")
	}

	m.okxOrderWS = market.NewOKXOrderWebSocket(testnet, apiKey, apiSecret, passphrase)
	if err := m.okxOrderWS.Connect(); err != nil {
		return fmt.Errorf("failed to connect OKX order stream: %w", err)
	}

	m.activeConnections["okx"] = true

	// Start listening for order updates
	go m.handleOKXOrderUpdates()

	logger.Infof("✓ OKX order WebSocket stream started")
	return nil
}

// handleBinanceOrderUpdates processes Binance order updates and publishes events
func (m *OrderWebSocketManager) handleBinanceOrderUpdates() {
	m.mu.RLock()
	ws := m.binanceOrderWS
	m.mu.RUnlock()

	if ws == nil {
		return
	}

	ch := ws.GetOrderUpdateChannel()
	for update := range ch {
		m.processOrderUpdate("binance", update)
	}
}

// handleBybitOrderUpdates processes Bybit order updates and publishes events
func (m *OrderWebSocketManager) handleBybitOrderUpdates() {
	m.mu.RLock()
	ws := m.bybitOrderWS
	m.mu.RUnlock()

	if ws == nil {
		return
	}

	ch := ws.GetOrderUpdateChannel()
	for update := range ch {
		m.processOrderUpdate("bybit", update)
	}
}

// handleOKXOrderUpdates processes OKX order updates and publishes events
func (m *OrderWebSocketManager) handleOKXOrderUpdates() {
	m.mu.RLock()
	ws := m.okxOrderWS
	m.mu.RUnlock()

	if ws == nil {
		return
	}

	ch := ws.GetOrderUpdateChannel()
	for update := range ch {
		m.processOrderUpdate("okx", update)
	}
}

// processOrderUpdate converts order update to trading event and publishes
func (m *OrderWebSocketManager) processOrderUpdate(exchange string, order market.OrderUpdate) {
	// Log the update
	logger.Infof("📊 [%s] Order update: %s %s (status: %s, qty: %.2f, filled: %.2f)",
		exchange, order.Symbol, order.Side, order.Status, order.OriginalQuantity, order.ExecutedQuantity)

	// Determine event type based on status
	var eventType EventType
	switch order.Status {
	case "FILLED", "filled", "Filled":
		eventType = EventTypeOrderFilled
	case "PARTIALLY_FILLED", "PartiallyFilled", "partial_fill":
		eventType = EventTypeOrderFilled
	case "CANCELED", "Cancelled", "cancelled":
		eventType = EventTypeRiskEvent // Could extend for cancel events
	case "REJECTED", "Rejected", "rejected":
		eventType = EventTypeRiskEvent
	default:
		// For other states (NEW, etc), skip event publishing
		return
	}

	// Publish event to event bus
	event := TradingEvent{
		Type:      eventType,
		Symbol:    order.Symbol,
		Timestamp: order.Timestamp,
		Source:    exchange + "_websocket",
		Severity:  1.0, // Order fills are always high priority
		Data: map[string]interface{}{
			"order_id":          order.OrderID,
			"client_order_id":   order.ClientOrderID,
			"side":              order.Side,
			"position_side":     order.PositionSide,
			"status":            order.Status,
			"original_quantity": order.OriginalQuantity,
			"executed_quantity": order.ExecutedQuantity,
			"cumulative_quote":  order.CumulativeQuoteQty,
			"average_price":     order.AveragePrice,
			"order_price":       order.OrderPrice,
			"reject_reason":     order.RejectReason,
		},
	}

	m.mu.RLock()
	eventBus := m.eventBus
	m.mu.RUnlock()

	if eventBus != nil {
		eventBus.PublishAsync(event)
	}

	// Call registered handlers
	m.callOrderHandlers(order)
}

// callOrderHandlers calls all registered order handlers
func (m *OrderWebSocketManager) callOrderHandlers(order market.OrderUpdate) {
	m.mu.RLock()
	handlers := make([]func(order market.OrderUpdate), len(m.orderUpdateHandlers))
	copy(handlers, m.orderUpdateHandlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		go func(h func(order market.OrderUpdate)) {
			defer func() {
				if r := recover(); r != nil {
					logger.Warnf("⚠️ Order handler panic: %v", r)
				}
			}()
			h(order)
		}(handler)
	}
}

// RegisterOrderHandler registers a callback for all order updates
func (m *OrderWebSocketManager) RegisterOrderHandler(handler func(order market.OrderUpdate)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.orderUpdateHandlers = append(m.orderUpdateHandlers, handler)
}

// StopAllStreams stops all active WebSocket connections
func (m *OrderWebSocketManager) StopAllStreams() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.binanceOrderWS != nil {
		m.binanceOrderWS.Disconnect()
		m.activeConnections["binance"] = false
	}

	if m.bybitOrderWS != nil {
		m.bybitOrderWS.Disconnect()
		m.activeConnections["bybit"] = false
	}

	if m.okxOrderWS != nil {
		m.okxOrderWS.Disconnect()
		m.activeConnections["okx"] = false
	}

	logger.Info("✓ All order WebSocket streams stopped")
}

// GetActiveStreams returns which exchanges have active order streams
func (m *OrderWebSocketManager) GetActiveStreams() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]bool)
	for k, v := range m.activeConnections {
		result[k] = v
	}

	return result
}

// RefreshBinanceListenKey extends the listen key (required every 60 minutes)
func (m *OrderWebSocketManager) RefreshBinanceListenKey() error {
	m.mu.RLock()
	ws := m.binanceOrderWS
	m.mu.RUnlock()

	if ws == nil {
		return fmt.Errorf("Binance order stream not connected")
	}

	return ws.RefreshListenKey()
}

// CheckConnectionHealth checks if all active streams are still connected
func (m *OrderWebSocketManager) CheckConnectionHealth() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errors []string

	if m.binanceOrderWS != nil && !m.binanceOrderWS.IsConnected() {
		errors = append(errors, "Binance order stream disconnected")
	}

	if m.bybitOrderWS != nil && !m.bybitOrderWS.IsConnected() {
		errors = append(errors, "Bybit order stream disconnected")
	}

	if m.okxOrderWS != nil && !m.okxOrderWS.IsConnected() {
		errors = append(errors, "OKX order stream disconnected")
	}

	if len(errors) > 0 {
		errorMsg := fmt.Sprintf("Order stream health check failed: %v", errors)
		logger.Warnf("⚠️ %s", errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}

	return nil
}

// AutoReconnect attempts to reconnect any disconnected streams
func (m *OrderWebSocketManager) AutoReconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.binanceOrderWS != nil && !m.binanceOrderWS.IsConnected() {
		logger.Warn("🔄 Attempting to reconnect Binance order stream...")
		if err := m.binanceOrderWS.Reconnect(); err != nil {
			logger.Warnf("❌ Failed to reconnect Binance: %v", err)
		}
	}

	if m.bybitOrderWS != nil && !m.bybitOrderWS.IsConnected() {
		logger.Warn("🔄 Attempting to reconnect Bybit order stream...")
		if err := m.bybitOrderWS.Reconnect(); err != nil {
			logger.Warnf("❌ Failed to reconnect Bybit: %v", err)
		}
	}

	if m.okxOrderWS != nil && !m.okxOrderWS.IsConnected() {
		logger.Warn("🔄 Attempting to reconnect OKX order stream...")
		if err := m.okxOrderWS.Reconnect(); err != nil {
			logger.Warnf("❌ Failed to reconnect OKX: %v", err)
		}
	}
}

// StartHealthCheck starts a background health check routine
func (m *OrderWebSocketManager) StartHealthCheck(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := m.CheckConnectionHealth(); err != nil {
				m.AutoReconnect()
			}
		}
	}()

	logger.Infof("✓ Order WebSocket health check started (interval: %v)", interval)
}
