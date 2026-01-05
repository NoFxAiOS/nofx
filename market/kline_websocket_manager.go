package market

import (
	"fmt"
	"nofx/logger"
	"strings"
	"sync"
	"time"
)

// KlineWebSocketManager manages multiple WebSocket connections for kline data
// to avoid hitting Binance's 1,024 stream limit per connection
type KlineWebSocketManager struct {
	mu                  sync.RWMutex
	connections         []*BinanceWebSocketClient // Pool of connections
	subscriptions       map[string]int            // subscriptionKey -> connectionIndex
	activeSymbols       map[string]bool           // Symbols actually being traded
	symbolTimeframes    map[string][]string       // symbol -> []timeframes to subscribe
	maxStreamsPerConn   int                       // Max streams per connection (default 500)
	testnet             bool
	klineUpdateHandlers []func(update KlineUpdate)
	stopCh              chan struct{}
	healthCheckInterval time.Duration
	lastUpdateTime      map[string]time.Time // subscriptionKey -> last update time
	staleDuration       time.Duration        // How long before data is considered stale
	restAPIFallback     *APIClient           // Fallback to REST when WebSocket fails
}

// NewKlineWebSocketManager creates a new manager with connection pooling
func NewKlineWebSocketManager(testnet bool) *KlineWebSocketManager {
	return &KlineWebSocketManager{
		connections:         make([]*BinanceWebSocketClient, 0),
		subscriptions:       make(map[string]int),
		activeSymbols:       make(map[string]bool),
		symbolTimeframes:    make(map[string][]string),
		maxStreamsPerConn:   500, // Conservative limit (Binance allows 1024)
		testnet:             testnet,
		klineUpdateHandlers: make([]func(update KlineUpdate), 0),
		stopCh:              make(chan struct{}),
		healthCheckInterval: 30 * time.Second,
		lastUpdateTime:      make(map[string]time.Time),
		staleDuration:       2 * time.Minute, // 4H candles should update every 4 hours
		restAPIFallback:     NewAPIClient(),
	}
}

// Start initializes the manager and starts health monitoring
func (m *KlineWebSocketManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create at least one connection
	if len(m.connections) == 0 {
		conn := NewBinanceWebSocketClient(m.testnet)
		if err := conn.Connect(); err != nil {
			return fmt.Errorf("failed to create initial WebSocket connection: %w", err)
		}
		m.connections = append(m.connections, conn)
		logger.Info("✓ KlineWebSocketManager: Initial connection created")
	}

	// Start health monitoring
	go m.monitorHealth()

	logger.Infof("✓ KlineWebSocketManager started (testnet=%v, maxStreamsPerConn=%d)", m.testnet, m.maxStreamsPerConn)
	return nil
}

// Stop gracefully shuts down all connections
func (m *KlineWebSocketManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already stopped
	select {
	case <-m.stopCh:
		return nil // Already stopped
	default:
		close(m.stopCh)
	}

	for i, conn := range m.connections {
		if err := conn.Disconnect(); err != nil {
			logger.Warnf("Error disconnecting connection %d: %v", i, err)
		}
	}

	logger.Info("✓ KlineWebSocketManager stopped")
	return nil
}

// RegisterActiveSymbols tells the manager which symbols are actively being traded
// This prevents subscribing to all 534 pairs and focuses only on what's needed
func (m *KlineWebSocketManager) RegisterActiveSymbols(symbols []string, timeframes []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Infof("📊 Registering %d active symbols with %d timeframes", len(symbols), len(timeframes))

	// Track which symbols are new or updated
	newSymbols := make([]string, 0)

	for _, symbol := range symbols {
		wasActive := m.activeSymbols[symbol]
		m.activeSymbols[symbol] = true

		// Store timeframes for this symbol
		existingTimeframes := m.symbolTimeframes[symbol]
		m.symbolTimeframes[symbol] = timeframes

		// Check if this is a new symbol or has different timeframes
		if !wasActive || !equalStringSlices(existingTimeframes, timeframes) {
			newSymbols = append(newSymbols, symbol)
		}
	}

	// Remove symbols no longer active
	for symbol := range m.activeSymbols {
		found := false
		for _, s := range symbols {
			if s == symbol {
				found = true
				break
			}
		}
		if !found {
			logger.Infof("🗑️ Removing inactive symbol: %s", symbol)
			delete(m.activeSymbols, symbol)
			delete(m.symbolTimeframes, symbol)
			// Unsubscribe from all timeframes
			if timeframes, ok := m.symbolTimeframes[symbol]; ok {
				for _, tf := range timeframes {
					m.unsubscribeInternal(symbol, tf)
				}
			}
		}
	}

	// Subscribe to new/updated symbols
	if len(newSymbols) > 0 {
		logger.Infof("📈 Subscribing to %d new/updated symbols", len(newSymbols))
		for _, symbol := range newSymbols {
			for _, tf := range timeframes {
				if err := m.subscribeInternal(symbol, tf); err != nil {
					logger.Warnf("Failed to subscribe %s@%s: %v", symbol, tf, err)
				}
			}
		}
	}

	logger.Infof("✓ Total active subscriptions: %d", len(m.subscriptions))
	return nil
}

// UnregisterSymbol removes a symbol from active trading
func (m *KlineWebSocketManager) UnregisterSymbol(symbol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.activeSymbols[symbol] {
		return nil // Already not active
	}

	logger.Infof("🗑️ Unregistering symbol: %s", symbol)

	// Unsubscribe from all timeframes
	if timeframes, ok := m.symbolTimeframes[symbol]; ok {
		for _, tf := range timeframes {
			m.unsubscribeInternal(symbol, tf)
		}
	}

	delete(m.activeSymbols, symbol)
	delete(m.symbolTimeframes, symbol)

	return nil
}

// subscribeInternal handles subscription with connection pooling (must hold lock)
func (m *KlineWebSocketManager) subscribeInternal(symbol, interval string) error {
	subscriptionKey := fmt.Sprintf("%s@kline_%s", symbol, interval)

	// Check if already subscribed
	if _, exists := m.subscriptions[subscriptionKey]; exists {
		return nil // Already subscribed
	}

	// Find a connection with capacity
	connIndex := m.findAvailableConnection()
	if connIndex == -1 {
		// Need to create a new connection
		conn := NewBinanceWebSocketClient(m.testnet)
		if err := conn.Connect(); err != nil {
			return fmt.Errorf("failed to create new WebSocket connection: %w", err)
		}
		m.connections = append(m.connections, conn)
		connIndex = len(m.connections) - 1
		logger.Infof("✓ Created new WebSocket connection #%d (total: %d)", connIndex, len(m.connections))

		// Start listening to this connection's updates
		go m.handleKlineUpdates(conn)
	}

	// Subscribe on the selected connection
	conn := m.connections[connIndex]
	if err := conn.SubscribeKlines(symbol, interval); err != nil {
		return fmt.Errorf("failed to subscribe %s: %w", subscriptionKey, err)
	}

	m.subscriptions[subscriptionKey] = connIndex
	m.lastUpdateTime[subscriptionKey] = time.Now()
	logger.Infof("✓ Subscribed: %s on connection #%d", subscriptionKey, connIndex)

	return nil
}

// unsubscribeInternal handles unsubscription (must hold lock)
func (m *KlineWebSocketManager) unsubscribeInternal(symbol, interval string) error {
	subscriptionKey := fmt.Sprintf("%s@kline_%s", symbol, interval)

	connIndex, exists := m.subscriptions[subscriptionKey]
	if !exists {
		return nil // Not subscribed
	}

	if connIndex < len(m.connections) {
		conn := m.connections[connIndex]
		if err := conn.UnsubscribeKlines(symbol, interval); err != nil {
			logger.Warnf("Error unsubscribing %s: %v", subscriptionKey, err)
		}
	}

	delete(m.subscriptions, subscriptionKey)
	delete(m.lastUpdateTime, subscriptionKey)
	logger.Infof("🗑️ Unsubscribed: %s", subscriptionKey)

	return nil
}

// findAvailableConnection finds a connection with available capacity (must hold lock)
func (m *KlineWebSocketManager) findAvailableConnection() int {
	// Count subscriptions per connection
	connCounts := make(map[int]int)
	for _, connIndex := range m.subscriptions {
		connCounts[connIndex]++
	}

	// Find first connection with capacity
	for i := range m.connections {
		if connCounts[i] < m.maxStreamsPerConn {
			return i
		}
	}

	return -1 // All connections are at capacity
}

// handleKlineUpdates listens to a connection's kline channel and forwards updates
func (m *KlineWebSocketManager) handleKlineUpdates(conn *BinanceWebSocketClient) {
	for {
		select {
		case <-m.stopCh:
			return
		case update, ok := <-conn.GetKlineChannel():
			if !ok {
				return // Channel closed
			}

			// Update last received time
			subscriptionKey := fmt.Sprintf("%s@kline_%s", update.Symbol, update.Interval)
			m.mu.Lock()
			m.lastUpdateTime[subscriptionKey] = time.Now()
			m.mu.Unlock()

			// Forward to all handlers
			m.mu.RLock()
			handlers := m.klineUpdateHandlers
			m.mu.RUnlock()

			for _, handler := range handlers {
				go handler(update)
			}
		}
	}
}

// RegisterKlineHandler adds a callback for kline updates
func (m *KlineWebSocketManager) RegisterKlineHandler(handler func(update KlineUpdate)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.klineUpdateHandlers = append(m.klineUpdateHandlers, handler)
}

// monitorHealth periodically checks connection health and data staleness
func (m *KlineWebSocketManager) monitorHealth() {
	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck checks connection status and data freshness
func (m *KlineWebSocketManager) performHealthCheck() {
	m.mu.RLock()
	subscriptions := make(map[string]int)
	for key, connIndex := range m.subscriptions {
		subscriptions[key] = connIndex
	}
	connections := m.connections
	m.mu.RUnlock()

	now := time.Now()

	// Check each subscription for staleness
	for subscriptionKey, connIndex := range subscriptions {
		lastUpdate, exists := m.lastUpdateTime[subscriptionKey]
		if !exists {
			lastUpdate = now // Newly subscribed
		}

		staleDuration := now.Sub(lastUpdate)
		if staleDuration > m.staleDuration {
			logger.Warnf("⚠️ Stale data detected: %s (last update: %v ago)", subscriptionKey, staleDuration)

			// Try reconnecting the connection
			if connIndex < len(connections) {
				conn := connections[connIndex]
				if !conn.IsConnected() {
					logger.Infof("🔄 Reconnecting connection #%d", connIndex)
					if err := conn.Reconnect(); err != nil {
						logger.Errorf("Failed to reconnect connection #%d: %v", connIndex, err)
					} else {
						// Resubscribe all streams on this connection
						m.resubscribeConnection(connIndex)
					}
				}
			}

			// Use REST API fallback for immediate data
			m.fetchViaRestAPI(subscriptionKey)
		}
	}

	// Check connection health
	for i, conn := range connections {
		if !conn.IsConnected() {
			logger.Warnf("⚠️ Connection #%d is disconnected, attempting reconnect", i)
			if err := conn.Reconnect(); err != nil {
				logger.Errorf("Failed to reconnect connection #%d: %v", i, err)
			} else {
				m.resubscribeConnection(i)
			}
		}
	}

	m.mu.RLock()
	activeCount := len(m.activeSymbols)
	subscriptionCount := len(m.subscriptions)
	connectionCount := len(m.connections)
	m.mu.RUnlock()

	logger.Infof("🔍 Health: %d active symbols, %d subscriptions, %d connections",
		activeCount, subscriptionCount, connectionCount)
}

// resubscribeConnection resubscribes all streams on a specific connection after reconnect
func (m *KlineWebSocketManager) resubscribeConnection(connIndex int) {
	m.mu.RLock()
	subscriptionsToRestore := make([]struct {
		key      string
		symbol   string
		interval string
	}, 0)

	for subscriptionKey, idx := range m.subscriptions {
		if idx == connIndex {
			// Parse subscription key: "BTCUSDT@kline_4h"
			parts := strings.Split(subscriptionKey, "@kline_")
			if len(parts) == 2 {
				subscriptionsToRestore = append(subscriptionsToRestore, struct {
					key      string
					symbol   string
					interval string
				}{
					key:      subscriptionKey,
					symbol:   parts[0],
					interval: parts[1],
				})
			}
		}
	}

	conn := m.connections[connIndex]
	m.mu.RUnlock()

	logger.Infof("🔄 Resubscribing %d streams on connection #%d", len(subscriptionsToRestore), connIndex)

	for _, sub := range subscriptionsToRestore {
		if err := conn.SubscribeKlines(sub.symbol, sub.interval); err != nil {
			logger.Warnf("Failed to resubscribe %s: %v", sub.key, err)
		} else {
			logger.Infof("✓ Resubscribed: %s", sub.key)
		}
	}
}

// fetchViaRestAPI fetches kline data via REST API when WebSocket data is stale
func (m *KlineWebSocketManager) fetchViaRestAPI(subscriptionKey string) {
	// Parse subscription key: "BTCUSDT@kline_4h"
	parts := strings.Split(subscriptionKey, "@kline_")
	if len(parts) != 2 {
		logger.Errorf("Invalid subscription key format: %s", subscriptionKey)
		return
	}

	symbol := parts[0]
	interval := parts[1]

	logger.Infof("🔄 Fetching %s %s via REST API (WebSocket data stale)", symbol, interval)

	// Fetch from REST API
	klines, err := m.restAPIFallback.GetKlines(symbol, interval, 1)
	if err != nil {
		logger.Errorf("REST API fallback failed for %s: %v", subscriptionKey, err)
		return
	}

	if len(klines) > 0 {
		// Convert to KlineUpdate and send to handlers
		update := KlineUpdate{
			Symbol:    symbol,
			Interval:  interval,
			OpenTime:  klines[0].OpenTime,
			Open:      klines[0].Open,
			High:      klines[0].High,
			Low:       klines[0].Low,
			Close:     klines[0].Close,
			Volume:    klines[0].Volume,
			Timestamp: time.Now(),
			IsClosed:  true,
		}

		// Update last update time
		m.mu.Lock()
		m.lastUpdateTime[subscriptionKey] = time.Now()
		m.mu.Unlock()

		// Forward to handlers
		m.mu.RLock()
		handlers := m.klineUpdateHandlers
		m.mu.RUnlock()

		for _, handler := range handlers {
			go handler(update)
		}

		logger.Infof("✓ REST API fallback successful for %s", subscriptionKey)
	}
}

// GetStatus returns current manager status
func (m *KlineWebSocketManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Count streams per connection
	connCounts := make(map[int]int)
	for _, connIndex := range m.subscriptions {
		connCounts[connIndex]++
	}

	connectionStatus := make([]map[string]interface{}, len(m.connections))
	for i, conn := range m.connections {
		connectionStatus[i] = map[string]interface{}{
			"index":         i,
			"connected":     conn.IsConnected(),
			"stream_count":  connCounts[i],
			"capacity_used": fmt.Sprintf("%.1f%%", float64(connCounts[i])/float64(m.maxStreamsPerConn)*100),
		}
	}

	return map[string]interface{}{
		"active_symbols":    len(m.activeSymbols),
		"subscriptions":     len(m.subscriptions),
		"connections":       len(m.connections),
		"max_per_conn":      m.maxStreamsPerConn,
		"connection_status": connectionStatus,
	}
}

// equalStringSlices checks if two string slices are equal
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
