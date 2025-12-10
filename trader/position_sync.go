package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"sync"
	"time"
)

// PositionSyncManager Position status synchronization manager
// Responsible for periodically synchronizing exchange positions, detecting manual closures and other changes
type PositionSyncManager struct {
	store                *store.Store
	interval             time.Duration
	historySyncInterval  time.Duration        // Interval for full history sync
	stopCh               chan struct{}
	wg                   sync.WaitGroup
	traderCache          map[string]Trader                    // trader_id -> Trader instance cache
	configCache          map[string]*store.TraderFullConfig   // trader_id -> config cache
	cacheMutex           sync.RWMutex
	lastHistorySync      map[string]time.Time // trader_id -> last history sync time
	lastHistorySyncMutex sync.RWMutex
}

// NewPositionSyncManager Create position synchronization manager
func NewPositionSyncManager(st *store.Store, interval time.Duration) *PositionSyncManager {
	if interval == 0 {
		interval = 10 * time.Second
	}
	return &PositionSyncManager{
		store:               st,
		interval:            interval,
		historySyncInterval: 5 * time.Minute, // Sync closed positions every 5 minutes
		stopCh:              make(chan struct{}),
		traderCache:         make(map[string]Trader),
		configCache:         make(map[string]*store.TraderFullConfig),
		lastHistorySync:     make(map[string]time.Time),
	}
}

// Start Start position synchronization service
func (m *PositionSyncManager) Start() {
	m.wg.Add(1)
	go m.run()
	logger.Info("📊 Position sync manager started")

	// Run startup sync in background
	go m.startupSync()
}

// Stop Stop position synchronization service
func (m *PositionSyncManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	// Clear cache
	m.cacheMutex.Lock()
	m.traderCache = make(map[string]Trader)
	m.configCache = make(map[string]*store.TraderFullConfig)
	m.cacheMutex.Unlock()

	logger.Info("📊 Position sync manager stopped")
}

// run Main loop
func (m *PositionSyncManager) run() {
	defer m.wg.Done()

	// Execute immediately on startup
	m.syncPositions()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.syncPositions()
		}
	}
}

// syncPositions Synchronize all position statuses
func (m *PositionSyncManager) syncPositions() {
	// Get all OPEN status positions
	localPositions, err := m.store.Position().GetAllOpenPositions()
	if err != nil {
		logger.Infof("⚠️  Failed to get local positions: %v", err)
		return
	}

	if len(localPositions) == 0 {
		return
	}

	// Group by trader_id
	positionsByTrader := make(map[string][]*store.TraderPosition)
	for _, pos := range localPositions {
		positionsByTrader[pos.TraderID] = append(positionsByTrader[pos.TraderID], pos)
	}

	// Process each trader
	for traderID, traderPositions := range positionsByTrader {
		m.syncTraderPositions(traderID, traderPositions)
	}
}

// syncTraderPositions Synchronize positions for a single trader
func (m *PositionSyncManager) syncTraderPositions(traderID string, localPositions []*store.TraderPosition) {
	// Get or create trader instance
	trader, err := m.getOrCreateTrader(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get trader instance (ID: %s): %v", traderID, err)
		return
	}

	// Get exchange ID for history sync
	config, _ := m.getTraderConfig(traderID)
	exchangeID := ""
	if config != nil {
		exchangeID = config.Exchange.ID
	}

	// Maybe run periodic history sync
	if exchangeID != "" {
		m.maybeRunHistorySync(traderID, exchangeID, trader)
	}

	// Get current exchange positions
	exchangePositions, err := trader.GetPositions()
	if err != nil {
		logger.Infof("⚠️  Failed to get exchange positions (ID: %s): %v", traderID, err)
		return
	}

	// Build exchange position map: symbol_side -> position
	exchangeMap := make(map[string]map[string]interface{})
	for _, pos := range exchangePositions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["positionSide"].(string)
		if symbol == "" || side == "" {
			continue
		}
		key := fmt.Sprintf("%s_%s", symbol, side)
		exchangeMap[key] = pos
	}

	// Compare local and exchange positions
	for _, localPos := range localPositions {
		key := fmt.Sprintf("%s_%s", localPos.Symbol, localPos.Side)
		exchangePos, exists := exchangeMap[key]

		if !exists {
			// Exchange doesn't have this position → it has been closed
			m.closeLocalPosition(localPos, trader, "manual")
			continue
		}

		// Check if quantity is 0 or very small
		qty := getFloatFromMap(exchangePos, "positionAmt")
		if qty < 0 {
			qty = -qty // Short position quantity is negative
		}

		if qty < 0.0000001 {
			// Quantity is 0, position closed
			m.closeLocalPosition(localPos, trader, "manual")
		}
	}
}

// closeLocalPosition Mark local position as closed
func (m *PositionSyncManager) closeLocalPosition(pos *store.TraderPosition, trader Trader, reason string) {
	// Try to get accurate closure data from exchange first
	closedPnLRecord := m.findClosedPnLRecord(trader, pos)

	var exitPrice, realizedPnL, fee float64
	var closeReason, exitOrderID string

	if closedPnLRecord != nil {
		// Use accurate data from exchange
		exitPrice = closedPnLRecord.ExitPrice
		realizedPnL = closedPnLRecord.RealizedPnL
		fee = closedPnLRecord.Fee
		closeReason = closedPnLRecord.CloseType
		exitOrderID = closedPnLRecord.OrderID
		logger.Infof("📊 Found accurate closure data from exchange for %s %s", pos.Symbol, pos.Side)
	} else {
		// Fallback: use market price and calculate PnL
		exitPrice = pos.EntryPrice // Default to entry price
		if price, err := trader.GetMarketPrice(pos.Symbol); err == nil && price > 0 {
			exitPrice = price
		}

		// Calculate PnL
		if pos.Side == "LONG" {
			realizedPnL = (exitPrice - pos.EntryPrice) * pos.Quantity
		} else {
			realizedPnL = (pos.EntryPrice - exitPrice) * pos.Quantity
		}
		closeReason = reason
		fee = 0
		exitOrderID = ""
		logger.Infof("⚠️  Using market price for closure (no exchange data): %s %s", pos.Symbol, pos.Side)
	}

	// Update database
	err := m.store.Position().ClosePosition(
		pos.ID,
		exitPrice,
		exitOrderID,
		realizedPnL,
		fee,
		closeReason,
	)

	if err != nil {
		logger.Infof("⚠️  Failed to update position status: %v", err)
	} else {
		logger.Infof("📊 Position closed [%s] %s %s @ %.4f → %.4f, PnL: %.2f, Fee: %.4f (%s)",
			pos.TraderID[:8], pos.Symbol, pos.Side, pos.EntryPrice, exitPrice, realizedPnL, fee, closeReason)
	}
}

// findClosedPnLRecord Try to find matching ClosedPnL record from exchange
func (m *PositionSyncManager) findClosedPnLRecord(trader Trader, pos *store.TraderPosition) *ClosedPnLRecord {
	// Get closed PnL records from the last 24 hours (to cover recent closures)
	startTime := time.Now().Add(-24 * time.Hour)
	records, err := trader.GetClosedPnL(startTime, 50)
	if err != nil {
		logger.Infof("⚠️  Failed to get closed PnL records: %v", err)
		return nil
	}

	if len(records) == 0 {
		return nil
	}

	// Normalize position side for comparison
	posSide := pos.Side
	if posSide == "LONG" {
		posSide = "long"
	} else if posSide == "SHORT" {
		posSide = "short"
	}

	// Find matching record by symbol and side
	// Priority: exact match on symbol and side, closest entry price
	var bestMatch *ClosedPnLRecord
	var bestPriceDiff float64 = -1

	for i := range records {
		record := &records[i]
		if record.Symbol != pos.Symbol {
			continue
		}

		// Match side (case-insensitive)
		recordSide := record.Side
		if recordSide == "LONG" {
			recordSide = "long"
		} else if recordSide == "SHORT" {
			recordSide = "short"
		}

		if recordSide != posSide {
			continue
		}

		// Check if entry price is close (within 2% to account for slippage)
		if record.EntryPrice > 0 {
			priceDiff := abs((record.EntryPrice - pos.EntryPrice) / pos.EntryPrice)
			if priceDiff > 0.02 {
				continue // Entry price too different, probably not the same position
			}

			// Prefer closest entry price match
			if bestMatch == nil || priceDiff < bestPriceDiff {
				bestMatch = record
				bestPriceDiff = priceDiff
			}
		} else {
			// No entry price in record, accept if symbol and side match
			if bestMatch == nil {
				bestMatch = record
			}
		}
	}

	return bestMatch
}

// abs returns absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// getOrCreateTrader Get or create trader instance
func (m *PositionSyncManager) getOrCreateTrader(traderID string) (Trader, error) {
	m.cacheMutex.RLock()
	trader, exists := m.traderCache[traderID]
	m.cacheMutex.RUnlock()

	if exists && trader != nil {
		return trader, nil
	}

	// Need to create new trader instance
	config, err := m.getTraderConfig(traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trader config: %w", err)
	}

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
func (m *PositionSyncManager) getTraderConfig(traderID string) (*store.TraderFullConfig, error) {
	m.cacheMutex.RLock()
	config, exists := m.configCache[traderID]
	m.cacheMutex.RUnlock()

	if exists {
		return config, nil
	}

	// Get from database
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
func (m *PositionSyncManager) createTrader(config *store.TraderFullConfig) (Trader, error) {
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

// InvalidateCache Invalidate cache
func (m *PositionSyncManager) InvalidateCache(traderID string) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	delete(m.traderCache, traderID)
	delete(m.configCache, traderID)
}

// getFloatFromMap Get float64 value from map
func getFloatFromMap(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int64:
			return float64(val)
		case int:
			return float64(val)
		case string:
			var f float64
			fmt.Sscanf(val, "%f", &f)
			return f
		}
	}
	return 0
}

// =============================================================================
// Startup and History Sync Methods
// =============================================================================

// startupSync performs initial sync on startup
// 1. Sync existing positions from exchange (to detect external positions)
// 2. Sync closed positions history from exchange
func (m *PositionSyncManager) startupSync() {
	logger.Info("📊 Starting startup sync...")

	// Get all traders
	traders, err := m.store.Trader().ListAll()
	if err != nil {
		logger.Infof("⚠️  Failed to get traders for startup sync: %v", err)
		return
	}

	for _, traderInfo := range traders {
		traderID := traderInfo.ID

		// Get trader instance
		trader, err := m.getOrCreateTrader(traderID)
		if err != nil {
			logger.Infof("⚠️  Failed to get trader instance for startup sync (ID: %s): %v", traderID, err)
			continue
		}

		// Get exchange ID
		config, err := m.getTraderConfig(traderID)
		if err != nil {
			logger.Infof("⚠️  Failed to get trader config for startup sync (ID: %s): %v", traderID, err)
			continue
		}
		exchangeID := config.Exchange.ID

		// 1. Sync current open positions from exchange
		m.syncExternalPositions(traderID, exchangeID, trader)

		// 2. Sync closed positions history from exchange
		m.syncClosedPositionsHistory(traderID, exchangeID, trader)
	}

	logger.Info("📊 Startup sync completed")
}

// syncExternalPositions syncs positions that exist on exchange but not locally
// These could be positions opened manually or from other systems
func (m *PositionSyncManager) syncExternalPositions(traderID, exchangeID string, trader Trader) {
	// Get current positions from exchange
	exchangePositions, err := trader.GetPositions()
	if err != nil {
		logger.Infof("⚠️  Failed to get exchange positions for external sync (ID: %s): %v", traderID, err)
		return
	}

	// Get local open positions
	localPositions, err := m.store.Position().GetOpenPositions(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get local positions for external sync (ID: %s): %v", traderID, err)
		return
	}

	// Build local position map: symbol_side -> position
	localMap := make(map[string]*store.TraderPosition)
	for _, pos := range localPositions {
		key := fmt.Sprintf("%s_%s", pos.Symbol, pos.Side)
		localMap[key] = pos
	}

	// Find positions that exist on exchange but not locally
	for _, pos := range exchangePositions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		if symbol == "" || side == "" {
			continue
		}

		// Normalize side
		normalizedSide := side
		if side == "Buy" || side == "LONG" || side == "long" {
			normalizedSide = "LONG"
		} else if side == "Sell" || side == "SHORT" || side == "short" {
			normalizedSide = "SHORT"
		}

		key := fmt.Sprintf("%s_%s", symbol, normalizedSide)

		// Check if we already have this position locally
		if _, exists := localMap[key]; exists {
			continue // Already tracking this position
		}

		// This is an external position - create local record
		qty := getFloatFromMap(pos, "positionAmt")
		if qty < 0 {
			qty = -qty
		}
		if qty < 0.0000001 {
			continue // No actual position
		}

		entryPrice := getFloatFromMap(pos, "entryPrice")
		leverage := int(getFloatFromMap(pos, "leverage"))
		if leverage == 0 {
			leverage = 1
		}

		// Get entry time if available
		createdTime := getFloatFromMap(pos, "createdTime")
		var entryTime time.Time
		if createdTime > 0 {
			entryTime = time.UnixMilli(int64(createdTime))
		} else {
			entryTime = time.Now() // Use current time as fallback
		}

		// Generate unique exchange position ID
		exchangePositionID := fmt.Sprintf("%s_%s_%d", symbol, normalizedSide, entryTime.UnixMilli())

		newPos := &store.TraderPosition{
			TraderID:           traderID,
			ExchangeID:         exchangeID,
			ExchangePositionID: exchangePositionID,
			Symbol:             symbol,
			Side:               normalizedSide,
			Quantity:           qty,
			EntryPrice:         entryPrice,
			EntryTime:          entryTime,
			Leverage:           leverage,
			Source:             "sync", // Mark as synced from exchange
		}

		if err := m.store.Position().CreateOpenPosition(newPos); err != nil {
			logger.Infof("⚠️  Failed to create external position record: %v", err)
		} else {
			logger.Infof("📊 Synced external position: [%s] %s %s @ %.4f (qty: %.4f)",
				traderID[:8], symbol, normalizedSide, entryPrice, qty)
		}
	}
}

// syncClosedPositionsHistory syncs closed positions from exchange history
func (m *PositionSyncManager) syncClosedPositionsHistory(traderID, exchangeID string, trader Trader) {
	// Get last sync time
	lastSyncTime, err := m.store.Position().GetLastClosedPositionTime(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get last closed position time (ID: %s): %v", traderID, err)
		lastSyncTime = time.Now().Add(-30 * 24 * time.Hour) // Default to 30 days ago
	}

	// Subtract a small buffer to avoid missing positions at the boundary
	startTime := lastSyncTime.Add(-1 * time.Minute)

	// Get closed positions from exchange
	closedRecords, err := trader.GetClosedPnL(startTime, 200) // Get up to 200 records
	if err != nil {
		logger.Infof("⚠️  Failed to get closed PnL records (ID: %s): %v", traderID, err)
		return
	}

	if len(closedRecords) == 0 {
		return
	}

	// Convert to store.ClosedPnLRecord and sync
	storeRecords := make([]store.ClosedPnLRecord, len(closedRecords))
	for i, rec := range closedRecords {
		storeRecords[i] = store.ClosedPnLRecord{
			Symbol:      rec.Symbol,
			Side:        rec.Side,
			EntryPrice:  rec.EntryPrice,
			ExitPrice:   rec.ExitPrice,
			Quantity:    rec.Quantity,
			RealizedPnL: rec.RealizedPnL,
			Fee:         rec.Fee,
			Leverage:    rec.Leverage,
			EntryTime:   rec.EntryTime,
			ExitTime:    rec.ExitTime,
			OrderID:     rec.OrderID,
			CloseType:   rec.CloseType,
			ExchangeID:  rec.ExchangeID,
		}
	}

	created, skipped, err := m.store.Position().SyncClosedPositions(traderID, exchangeID, storeRecords)
	if err != nil {
		logger.Infof("⚠️  Failed to sync closed positions (ID: %s): %v", traderID, err)
		return
	}

	if created > 0 {
		logger.Infof("📊 Synced %d new closed positions for trader %s (skipped %d duplicates)",
			created, traderID[:8], skipped)
	}

	// Update last history sync time
	m.lastHistorySyncMutex.Lock()
	m.lastHistorySync[traderID] = time.Now()
	m.lastHistorySyncMutex.Unlock()
}

// maybeRunHistorySync checks if it's time to run history sync for a trader
func (m *PositionSyncManager) maybeRunHistorySync(traderID, exchangeID string, trader Trader) {
	m.lastHistorySyncMutex.RLock()
	lastSync, exists := m.lastHistorySync[traderID]
	m.lastHistorySyncMutex.RUnlock()

	if !exists || time.Since(lastSync) >= m.historySyncInterval {
		m.syncClosedPositionsHistory(traderID, exchangeID, trader)
	}
}
