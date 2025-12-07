package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"sync"
	"time"
)

// OrderSyncManager Order status synchronization manager
// Responsible for periodically scanning all NEW status orders and updating their status
type OrderSyncManager struct {
	store        *store.Store
	interval     time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	traderCache  map[string]Trader // trader_id -> Trader instance cache
	configCache  map[string]*store.TraderFullConfig // trader_id -> config cache
	cacheMutex   sync.RWMutex
}

// NewOrderSyncManager Create order synchronization manager
func NewOrderSyncManager(st *store.Store, interval time.Duration) *OrderSyncManager {
	if interval == 0 {
		interval = 10 * time.Second
	}
	return &OrderSyncManager{
		store:       st,
		interval:    interval,
		stopCh:      make(chan struct{}),
		traderCache: make(map[string]Trader),
		configCache: make(map[string]*store.TraderFullConfig),
	}
}

// Start Start order synchronization service
func (m *OrderSyncManager) Start() {
	m.wg.Add(1)
	go m.run()
	logger.Info("📦 Order sync manager started")
}

// Stop Stop order synchronization service
func (m *OrderSyncManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	// Clear cache
	m.cacheMutex.Lock()
	m.traderCache = make(map[string]Trader)
	m.configCache = make(map[string]*store.TraderFullConfig)
	m.cacheMutex.Unlock()

	logger.Info("📦 Order sync manager stopped")
}

// run Main loop
func (m *OrderSyncManager) run() {
	defer m.wg.Done()

	// Execute immediately on startup
	m.syncOrders()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.syncOrders()
		}
	}
}

// syncOrders Synchronize all pending orders
func (m *OrderSyncManager) syncOrders() {
	// Get all NEW status orders
	orders, err := m.store.Order().GetAllPendingOrders()
	if err != nil {
		logger.Infof("⚠️  Failed to get pending orders: %v", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	logger.Infof("📦 Starting to sync %d pending orders...", len(orders))

	// Group by trader_id
	ordersByTrader := make(map[string][]*store.TraderOrder)
	for _, order := range orders {
		ordersByTrader[order.TraderID] = append(ordersByTrader[order.TraderID], order)
	}

	// Process each trader
	for traderID, traderOrders := range ordersByTrader {
		m.syncTraderOrders(traderID, traderOrders)
	}
}

// syncTraderOrders Synchronize orders for a single trader
func (m *OrderSyncManager) syncTraderOrders(traderID string, orders []*store.TraderOrder) {
	// Get or create trader instance
	trader, err := m.getOrCreateTrader(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get trader instance (ID: %s): %v", traderID, err)
		return
	}

	for _, order := range orders {
		m.syncSingleOrder(trader, order)
	}
}

// syncSingleOrder Synchronize single order status
func (m *OrderSyncManager) syncSingleOrder(trader Trader, order *store.TraderOrder) {
	status, err := trader.GetOrderStatus(order.Symbol, order.OrderID)
	if err != nil {
		// Query failed, check order creation time, assume filled after certain time
		if time.Since(order.CreatedAt) > 5*time.Minute {
			logger.Infof("⚠️  Order query timeout, assuming filled (ID: %s)", order.OrderID)
			m.markOrderFilled(order, 0, 0, 0)
		}
		return
	}

	statusStr, _ := status["status"].(string)

	switch statusStr {
	case "FILLED":
		avgPrice, _ := status["avgPrice"].(float64)
		executedQty, _ := status["executedQty"].(float64)
		commission, _ := status["commission"].(float64)

		// If API doesn't return quantity, use original quantity
		if executedQty == 0 {
			executedQty = order.Quantity
		}

		m.markOrderFilled(order, avgPrice, executedQty, commission)

	case "CANCELED", "EXPIRED":
		order.Status = statusStr
		if err := m.store.Order().Update(order); err != nil {
			logger.Infof("⚠️  Failed to update order status: %v", err)
		} else {
			logger.Infof("📦 Order status updated: %s (ID: %s)", statusStr, order.OrderID)
		}
	}
}

// markOrderFilled Mark order as filled
func (m *OrderSyncManager) markOrderFilled(order *store.TraderOrder, avgPrice, executedQty, commission float64) {
	// If avgPrice is 0, use order price
	if avgPrice == 0 {
		avgPrice = order.Price
	}
	if executedQty == 0 {
		executedQty = order.Quantity
	}

	// Calculate realized PnL (only for closing orders)
	var realizedPnL float64
	if (order.Action == "close_long" || order.Action == "close_short") && order.EntryPrice > 0 && avgPrice > 0 {
		if order.Action == "close_long" {
			// Long close PnL = (close price - entry price) * quantity
			realizedPnL = (avgPrice - order.EntryPrice) * executedQty
		} else {
			// Short close PnL = (entry price - close price) * quantity
			realizedPnL = (order.EntryPrice - avgPrice) * executedQty
		}
	}

	order.AvgPrice = avgPrice
	order.ExecutedQty = executedQty
	order.Status = "FILLED"
	order.Fee = commission
	order.RealizedPnL = realizedPnL
	order.FilledAt = time.Now()

	if err := m.store.Order().Update(order); err != nil {
		logger.Infof("⚠️  Failed to update order status: %v", err)
	} else {
		if realizedPnL != 0 {
			logger.Infof("✅ Order filled (ID: %s, avgPrice: %.4f, qty: %.4f, PnL: %.2f)",
				order.OrderID, avgPrice, executedQty, realizedPnL)
		} else {
			logger.Infof("✅ Order filled (ID: %s, avgPrice: %.4f, qty: %.4f)",
				order.OrderID, avgPrice, executedQty)
		}
	}
}

// getOrCreateTrader Get or create trader instance
func (m *OrderSyncManager) getOrCreateTrader(traderID string) (Trader, error) {
	m.cacheMutex.RLock()
	trader, exists := m.traderCache[traderID]
	m.cacheMutex.RUnlock()

	if exists && trader != nil {
		return trader, nil
	}

	// Need to create new trader instance
	// First get trader config
	config, err := m.getTraderConfig(traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trader config: %w", err)
	}

	// Create trader based on exchange type
	trader, err = m.createTrader(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create trader instance: %w", err)
	}

	m.cacheMutex.Lock()
	m.traderCache[traderID] = trader
	m.cacheMutex.Unlock()

	return trader, nil
}

// getTraderConfig Get trader configuration
func (m *OrderSyncManager) getTraderConfig(traderID string) (*store.TraderFullConfig, error) {
	m.cacheMutex.RLock()
	config, exists := m.configCache[traderID]
	m.cacheMutex.RUnlock()

	if exists {
		return config, nil
	}

	// Get from database - need to find trader's corresponding userID
	// First query all traders to find corresponding userID
	traders, err := m.store.Trader().ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get trader list: %w", err)
	}

	var userID string
	for _, t := range traders {
		if t.ID == traderID {
			userID = t.UserID
			break
		}
	}

	if userID == "" {
		return nil, fmt.Errorf("trader not found: %s", traderID)
	}

	config, err = m.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		return nil, err
	}

	m.cacheMutex.Lock()
	m.configCache[traderID] = config
	m.cacheMutex.Unlock()

	return config, nil
}

// createTrader Create trader instance based on configuration
func (m *OrderSyncManager) createTrader(config *store.TraderFullConfig) (Trader, error) {
	exchange := config.Exchange

	// Use exchange.ID to determine specific exchange, not exchange.Type (cex/dex)
	switch exchange.ID {
	case "binance":
		return NewFuturesTrader(exchange.APIKey, exchange.SecretKey, config.Trader.UserID), nil

	case "bybit":
		return NewBybitTrader(exchange.APIKey, exchange.SecretKey), nil

	case "okx":
		return NewOKXTrader(exchange.APIKey, exchange.SecretKey, exchange.Passphrase), nil

	case "hyperliquid":
		return NewHyperliquidTrader(exchange.SecretKey, exchange.HyperliquidWalletAddr, exchange.Testnet)

	case "aster":
		return NewAsterTrader(exchange.AsterUser, exchange.AsterSigner, exchange.AsterPrivateKey)

	case "lighter":
		if exchange.LighterAPIKeyPrivateKey != "" {
			return NewLighterTraderV2(
				exchange.LighterPrivateKey,
				exchange.LighterWalletAddr,
				exchange.LighterAPIKeyPrivateKey,
				exchange.Testnet,
			)
		}
		return NewLighterTrader(exchange.LighterPrivateKey, exchange.LighterWalletAddr, exchange.Testnet)

	default:
		return nil, fmt.Errorf("unsupported exchange: %s", exchange.ID)
	}
}

// InvalidateCache Invalidate cache (call when configuration changes)
func (m *OrderSyncManager) InvalidateCache(traderID string) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	delete(m.traderCache, traderID)
	delete(m.configCache, traderID)
}
