package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"strings"
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

	// Get exchange info for history sync
	config, _ := m.getTraderConfig(traderID)
	exchangeID := ""
	exchangeType := ""
	if config != nil {
		exchangeID = config.Exchange.ID           // UUID for database association
		exchangeType = config.Exchange.ExchangeType // "binance", "bybit" etc for trader creation
	}

	// Maybe run periodic history sync
	if exchangeID != "" && exchangeType != "" {
		m.maybeRunHistorySync(traderID, exchangeID, exchangeType, trader)
	}

	// Get current exchange positions
	exchangePositions, err := trader.GetPositions()
	if err != nil {
		logger.Infof("⚠️  Failed to get exchange positions (ID: %s): %v", traderID, err)
		return
	}

	// Build exchange position map: symbol_side -> position
	// Note: Exchange returns side as "long"/"short" (lowercase), database stores "LONG"/"SHORT" (uppercase)
	exchangeMap := make(map[string]map[string]interface{})
	for _, pos := range exchangePositions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string) // Note: use "side" not "positionSide"
		if symbol == "" || side == "" {
			continue
		}
		// Normalize side to uppercase for matching with database
		normalizedSide := strings.ToUpper(side)
		key := fmt.Sprintf("%s_%s", symbol, normalizedSide)
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
// For Binance, directly query trades for the specific symbol (more reliable than Income API)
func (m *PositionSyncManager) findClosedPnLRecord(trader Trader, pos *store.TraderPosition) *ClosedPnLRecord {
	// Try to get trades directly for this symbol (Binance-specific, more reliable)
	if binanceTrader, ok := trader.(*FuturesTrader); ok {
		return m.findClosedPnLFromBinanceTrades(binanceTrader, pos)
	}

	// Fallback: use GetClosedPnL for other exchanges
	startTime := time.Now().Add(-24 * time.Hour)
	records, err := trader.GetClosedPnL(startTime, 100)
	if err != nil {
		logger.Infof("⚠️  Failed to get closed PnL records: %v", err)
		return nil
	}

	return m.aggregateClosedRecords(records, pos)
}

// findClosedPnLFromBinanceTrades queries Binance directly for trades of a specific symbol
func (m *PositionSyncManager) findClosedPnLFromBinanceTrades(trader *FuturesTrader, pos *store.TraderPosition) *ClosedPnLRecord {
	// Query trades for this specific symbol from the last hour
	startTime := time.Now().Add(-1 * time.Hour)
	trades, err := trader.GetTradesForSymbol(pos.Symbol, startTime, 100)
	if err != nil {
		logger.Infof("⚠️  Failed to get trades for %s: %v", pos.Symbol, err)
		return nil
	}

	if len(trades) == 0 {
		logger.Infof("⚠️  No trades found for %s in the last hour", pos.Symbol)
		return nil
	}

	// Find all closing trades (realizedPnl != 0) that match this position
	var totalQty, totalPnL, totalFee float64
	var weightedExitPrice float64
	var latestExitTime time.Time
	var latestTradeID string
	matchCount := 0

	posSide := strings.ToLower(pos.Side)

	for _, trade := range trades {
		// Skip opening trades
		if trade.RealizedPnL == 0 {
			continue
		}

		// Determine if this trade closes our position
		// For LONG position: SELL closes it
		// For SHORT position: BUY closes it
		isClosingTrade := false
		tradeSide := strings.ToUpper(trade.Side)
		positionSide := strings.ToUpper(trade.PositionSide)

		if positionSide == "LONG" && posSide == "long" {
			isClosingTrade = true
		} else if positionSide == "SHORT" && posSide == "short" {
			isClosingTrade = true
		} else if positionSide == "BOTH" || positionSide == "" {
			// One-way mode
			if tradeSide == "SELL" && posSide == "long" {
				isClosingTrade = true
			} else if tradeSide == "BUY" && posSide == "short" {
				isClosingTrade = true
			}
		}

		if !isClosingTrade {
			continue
		}

		// Aggregate this trade
		totalQty += trade.Quantity
		totalPnL += trade.RealizedPnL
		totalFee += trade.Fee
		weightedExitPrice += trade.Price * trade.Quantity
		matchCount++

		if trade.Time.After(latestExitTime) {
			latestExitTime = trade.Time
			latestTradeID = trade.TradeID
		}
	}

	if matchCount == 0 {
		logger.Infof("⚠️  No closing trades found for %s %s", pos.Symbol, pos.Side)
		return nil
	}

	avgExitPrice := weightedExitPrice / totalQty

	logger.Infof("📊 Found %d closing trades for %s %s: qty=%.4f, exitPrice=%.6f, pnl=%.4f, fee=%.4f",
		matchCount, pos.Symbol, pos.Side, totalQty, avgExitPrice, totalPnL, totalFee)

	return &ClosedPnLRecord{
		Symbol:      pos.Symbol,
		Side:        posSide,
		EntryPrice:  pos.EntryPrice,
		ExitPrice:   avgExitPrice,
		Quantity:    totalQty,
		RealizedPnL: totalPnL,
		Fee:         totalFee,
		ExitTime:    latestExitTime,
		EntryTime:   pos.EntryTime,
		OrderID:     latestTradeID,
		ExchangeID:  latestTradeID,
		CloseType:   "unknown",
	}
}

// aggregateClosedRecords aggregates closed PnL records for a position
func (m *PositionSyncManager) aggregateClosedRecords(records []ClosedPnLRecord, pos *store.TraderPosition) *ClosedPnLRecord {
	if len(records) == 0 {
		return nil
	}

	posSide := strings.ToLower(pos.Side)
	var matchingRecords []ClosedPnLRecord

	for i := range records {
		record := &records[i]
		if record.Symbol != pos.Symbol {
			continue
		}

		recordSide := strings.ToLower(record.Side)
		if recordSide != posSide {
			continue
		}

		matchingRecords = append(matchingRecords, *record)
	}

	if len(matchingRecords) == 0 {
		return nil
	}

	var totalQty, totalPnL, totalFee float64
	var weightedExitPrice float64
	var latestExitTime time.Time
	var latestOrderID, latestExchangeID string

	for _, rec := range matchingRecords {
		totalQty += rec.Quantity
		totalPnL += rec.RealizedPnL
		totalFee += rec.Fee
		weightedExitPrice += rec.ExitPrice * rec.Quantity

		if rec.ExitTime.After(latestExitTime) {
			latestExitTime = rec.ExitTime
			latestOrderID = rec.OrderID
			latestExchangeID = rec.ExchangeID
		}
	}

	avgExitPrice := weightedExitPrice / totalQty

	logger.Infof("📊 Aggregated %d closing trades for %s %s: qty=%.4f, pnl=%.4f, fee=%.4f",
		len(matchingRecords), pos.Symbol, pos.Side, totalQty, totalPnL, totalFee)

	return &ClosedPnLRecord{
		Symbol:      pos.Symbol,
		Side:        posSide,
		EntryPrice:  pos.EntryPrice,
		ExitPrice:   avgExitPrice,
		Quantity:    totalQty,
		RealizedPnL: totalPnL,
		Fee:         totalFee,
		ExitTime:    latestExitTime,
		EntryTime:   pos.EntryTime,
		OrderID:     latestOrderID,
		ExchangeID:  latestExchangeID,
		CloseType:   "unknown",
	}
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

	// Use exchange.ExchangeType to determine specific exchange, not exchange.ID (UUID) or exchange.Type (cex/dex)
	switch exchange.ExchangeType {
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
		return nil, fmt.Errorf("unsupported exchange type: %s", exchange.ExchangeType)
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

		// Get exchange info
		config, err := m.getTraderConfig(traderID)
		if err != nil {
			logger.Infof("⚠️  Failed to get trader config for startup sync (ID: %s): %v", traderID, err)
			continue
		}
		exchangeID := config.Exchange.ID               // UUID
		exchangeType := config.Exchange.ExchangeType  // "binance", "bybit" etc

		// 1. Sync current open positions from exchange
		m.syncExternalPositions(traderID, exchangeID, exchangeType, trader)

		// 2. Sync closed positions history from exchange
		m.syncClosedPositionsHistory(traderID, exchangeID, exchangeType, trader)
	}

	logger.Info("📊 Startup sync completed")
}

// syncExternalPositions syncs positions that exist on exchange but not locally
// These could be positions opened manually or from other systems
func (m *PositionSyncManager) syncExternalPositions(traderID, exchangeID, exchangeType string, trader Trader) {
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
			ExchangeType:       exchangeType,
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
// IMPORTANT: Only exchanges with position-level history API should sync history:
// - Bybit: /v5/position/closed-pnl (accurate position records)
// - OKX: /api/v5/account/positions-history (accurate position records)
// Other exchanges (Binance, Hyperliquid, Lighter, Aster) only have trade-level data,
// which cannot accurately reconstruct positions. They should NOT sync historical positions.
func (m *PositionSyncManager) syncClosedPositionsHistory(traderID, exchangeID, exchangeType string, trader Trader) {
	// Only sync history for exchanges with position-level API
	// Binance/Hyperliquid/Lighter/Aster only have trade-level data, skip history sync
	switch exchangeType {
	case "bybit", "okx":
		// These exchanges have position-level history API, proceed with sync
	default:
		// Other exchanges don't have accurate position history API
		// Their GetClosedPnL only returns recent trades for closure detection, not for history sync
		return
	}

	// Get last sync time from database
	lastSyncTime, err := m.store.Position().GetLastClosedPositionTime(traderID)
	if err != nil {
		logger.Infof("⚠️  Failed to get last closed position time (ID: %s): %v", traderID, err)
		// First sync: go back 90 days to get more history
		lastSyncTime = time.Now().Add(-90 * 24 * time.Hour)
	}

	// Subtract a small buffer to avoid missing positions at the boundary
	startTime := lastSyncTime.Add(-1 * time.Minute)

	// Pagination loop to get all records
	const batchSize = 500
	totalCreated := 0
	totalSkipped := 0

	for {
		// Get closed positions from exchange
		closedRecords, err := trader.GetClosedPnL(startTime, batchSize)
		if err != nil {
			logger.Infof("⚠️  Failed to get closed PnL records (ID: %s): %v", traderID, err)
			break
		}

		if len(closedRecords) == 0 {
			break
		}

		// Convert to store.ClosedPnLRecord and sync
		storeRecords := make([]store.ClosedPnLRecord, len(closedRecords))
		var latestExitTime time.Time
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
			// Track latest exit time for pagination
			if rec.ExitTime.After(latestExitTime) {
				latestExitTime = rec.ExitTime
			}
		}

		created, skipped, err := m.store.Position().SyncClosedPositions(traderID, exchangeID, exchangeType, storeRecords)
		if err != nil {
			logger.Infof("⚠️  Failed to sync closed positions (ID: %s): %v", traderID, err)
			break
		}

		totalCreated += created
		totalSkipped += skipped

		// If we got fewer records than batch size, we've reached the end
		if len(closedRecords) < batchSize {
			break
		}

		// Move start time forward for next batch (add 1ms to avoid duplicate)
		startTime = latestExitTime.Add(time.Millisecond)
	}

	if totalCreated > 0 {
		logger.Infof("📊 Synced %d new closed positions for trader %s (skipped %d duplicates)",
			totalCreated, traderID[:8], totalSkipped)
	}

	// Update last history sync time
	m.lastHistorySyncMutex.Lock()
	m.lastHistorySync[traderID] = time.Now()
	m.lastHistorySyncMutex.Unlock()
}

// maybeRunHistorySync checks if it's time to run history sync for a trader
func (m *PositionSyncManager) maybeRunHistorySync(traderID, exchangeID, exchangeType string, trader Trader) {
	m.lastHistorySyncMutex.RLock()
	lastSync, exists := m.lastHistorySync[traderID]
	m.lastHistorySyncMutex.RUnlock()

	if !exists || time.Since(lastSync) >= m.historySyncInterval {
		m.syncClosedPositionsHistory(traderID, exchangeID, exchangeType, trader)
	}
}
