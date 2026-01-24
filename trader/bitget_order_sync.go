package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
	"sync"
	"time"
)

// syncState stores the last sync time (Unix ms) for incremental sync
var (
	bitgetSyncState      = make(map[string]int64) // exchangeID -> lastSyncTimeMs (Unix ms)
	bitgetSyncStateMutex sync.RWMutex
)

// SyncOrdersFromBitget syncs Bitget exchange order history to local database
// Uses smart symbol detection + incremental sync for efficiency
// Also creates/updates position records to ensure orders/fills/positions data consistency
// exchangeID: Exchange account UUID (from exchanges.id)
// exchangeType: Exchange type ("bitget")
func (t *BitgetTrader) SyncOrdersFromBitget(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	logger.Infof("🔄 [Bitget] Starting order sync for exchange %s...", exchangeID)

	// Get last sync time from state cache
	bitgetSyncStateMutex.RLock()
	lastSyncTimeMs, exists := bitgetSyncState[exchangeID]
	bitgetSyncStateMutex.RUnlock()

	nowMs := time.Now().UTC().UnixMilli()
	orderStore := st.Order()

	if !exists || lastSyncTimeMs == 0 {
		// Try to get last fill time from database (persist across restarts)
		lastFillTimeMs, err := orderStore.GetLastFillTimeByExchange(exchangeID)
		if err == nil && lastFillTimeMs > 0 {
			// If recovered time is in the future, it's clearly wrong - use default
			if lastFillTimeMs > nowMs {
				logger.Infof("⚠️ [Bitget] DB sync time %d is in the future (now: %d), using default",
					lastFillTimeMs, nowMs)
				lastSyncTimeMs = nowMs - 24*60*60*1000 // 24 hours ago
			} else {
				// Add 1 second buffer to avoid re-fetching the same fill
				lastSyncTimeMs = lastFillTimeMs + 1000
				logger.Infof("📅 [Bitget] Recovered last sync time from DB: %s (UTC)",
					time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"))
			}
		} else {
			// First sync: go back 24 hours
			lastSyncTimeMs = nowMs - 24*60*60*1000
			logger.Infof("📅 [Bitget] First sync, starting from 24 hours ago: %s (UTC)",
				time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"))
		}
	} else {
		logger.Infof("🔄 [Bitget] Syncing trades from: %s (UTC) [ms: %d, now: %d]",
			time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"), lastSyncTimeMs, nowMs)
	}

	// Step 1: Get max trade IDs from local DB for incremental sync
	maxTradeIDs, err := orderStore.GetMaxTradeIDsByExchange(exchangeID)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get max trade IDs: %v, will use time-based query", err)
		maxTradeIDs = make(map[string]int64)
	}

	// Step 2: Detect symbols to sync using multiple methods (like Binance)
	symbolMap := make(map[string]bool)
	lastSyncTime := time.UnixMilli(lastSyncTimeMs)

	// Method 1: Commission/Fee detection (check fills with fees)
	commissionSymbols, err := t.GetCommissionSymbols(lastSyncTime)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get commission symbols: %v", err)
	} else {
		logger.Infof("  📋 Commission symbols found: %d - %v", len(commissionSymbols), commissionSymbols)
		for _, s := range commissionSymbols {
			symbolMap[s] = true
		}
	}

	// Method 2: Always include active positions (catches trades that commission detection missed)
	positionSymbols := t.getPositionSymbols()
	logger.Infof("  📋 Position symbols found: %d - %v", len(positionSymbols), positionSymbols)
	for _, s := range positionSymbols {
		symbolMap[s] = true
	}

	// Method 3: Include symbols from recent fills in DB (in case some were partially synced)
	recentSymbols, _ := orderStore.GetRecentFillSymbolsByExchange(exchangeID, lastSyncTimeMs)
	logger.Infof("  📋 Recent fill symbols found: %d - %v", len(recentSymbols), recentSymbols)
	for _, s := range recentSymbols {
		symbolMap[s] = true
	}

	// Method 4: PnL detection for symbols with closed trades
	pnlSymbols, err := t.GetPnLSymbols(lastSyncTime)
	if err != nil {
		logger.Infof("  ⚠️ Failed to get PnL symbols: %v", err)
	} else {
		logger.Infof("  📋 REALIZED_PNL symbols found: %d - %v", len(pnlSymbols), pnlSymbols)
		for _, s := range pnlSymbols {
			symbolMap[s] = true
		}
	}

	var changedSymbols []string
	for s := range symbolMap {
		changedSymbols = append(changedSymbols, s)
	}

	if len(changedSymbols) == 0 {
		logger.Infof("📭 [Bitget] No symbols with new trades to sync")
		return nil
	}

	logger.Infof("📊 [Bitget] Found %d symbols with new trades: %v", len(changedSymbols), changedSymbols)

	// Step 3: Query trades for changed symbols using incremental sync when possible
	var allTrades []TradeRecord
	var failedSymbols []string
	apiCalls := 0

	for _, symbol := range changedSymbols {
		var trades []TradeRecord
		var queryErr error

		if lastID, ok := maxTradeIDs[symbol]; ok && lastID > 0 {
			// Incremental sync: query from last known trade ID
			trades, queryErr = t.GetTradesForSymbolFromID(symbol, lastID+1, 100)
		} else {
			// New symbol or first sync: query by time
			trades, queryErr = t.GetTradesForSymbol(symbol, lastSyncTime, 100)
		}
		apiCalls++

		if queryErr != nil {
			logger.Infof("  ⚠️ Failed to get trades for %s: %v", symbol, queryErr)
			failedSymbols = append(failedSymbols, symbol)
			continue
		}
		allTrades = append(allTrades, trades...)
	}

	logger.Infof("📥 [Bitget] Received %d trades (%d API calls)", len(allTrades), apiCalls)

	if len(allTrades) == 0 {
		if len(failedSymbols) > 0 {
			logger.Infof("  ⚠️ %d symbols failed: %v", len(failedSymbols), failedSymbols)
		}
		return nil
	}
		// Step 3: Query trades for changed symbols using incremental sync when possible
		var allTrades []TradeRecord
		var failedSymbols []string
		var partiallyFailedSymbols []string
		apiCalls := 0

		for _, symbol := range changedSymbols {
			var trades []TradeRecord
			var queryErr error

			if lastID, ok := maxTradeIDs[symbol]; ok && lastID > 0 {
				// Incremental sync: query from last known trade ID
				trades, queryErr = t.GetTradesForSymbolFromID(symbol, lastID+1, 100)
			} else {
				// New symbol or first sync: query by time
				trades, queryErr = t.GetTradesForSymbol(symbol, lastSyncTime, 100)
			}
			apiCalls++

			if queryErr != nil {
				// Distinguish between "symbol not found" errors (which are OK to skip) 
				// and other API errors (which should be logged as failures)
				errStr := queryErr.Error()
				if strings.Contains(errStr, "40309") || strings.Contains(errStr, "The symbol has been removed") {
					// This is a delisted symbol - safe to skip
					logger.Infof("  ℹ️ Symbol %s appears to be delisted (skipping)", symbol)
					partiallyFailedSymbols = append(partiallyFailedSymbols, symbol)
				} else if strings.Contains(errStr, "Too Many Requests") || strings.Contains(errStr, "429") {
					// Rate limit - might succeed on next attempt
					logger.Warnf("  ⏳ Rate limited while fetching %s: %v", symbol, queryErr)
					failedSymbols = append(failedSymbols, symbol)
				} else {
					// Other errors - log for investigation
					logger.Warnf("  ⚠️ Failed to get trades for %s: %v", symbol, queryErr)
					failedSymbols = append(failedSymbols, symbol)
				}
				continue
			}
			allTrades = append(allTrades, trades...)
		}

		logger.Infof("📥 [Bitget] Received %d trades (%d API calls, %d skipped delisted symbols)", 
			len(allTrades), apiCalls, len(partiallyFailedSymbols))

		if len(allTrades) == 0 {
			if len(failedSymbols) > 0 {
				logger.Warnf("  ⚠️ %d symbols failed: %v", len(failedSymbols), failedSymbols)
			}
			return nil
		}

		// Sort trades by time ASC (oldest first) for proper position building
	sort.Slice(allTrades, func(i, j int) bool {
		return allTrades[i].Time.UnixMilli() < allTrades[j].Time.UnixMilli()
	})

	// Process trades one by one
	positionStore := st.Position()
	posBuilder := store.NewPositionBuilder(positionStore)
	syncedCount := 0
	skippedCount := 0

	for _, trade := range allTrades {
		// Check if trade already exists (use exchangeID which is UUID, not exchange type)
		existing, err := orderStore.GetOrderByExchangeID(exchangeID, trade.TradeID)
		if err == nil && existing != nil {
			skippedCount++
			continue // Order already exists, skip
		}

		// Normalize symbol
		symbol := market.Normalize(trade.Symbol)

		// Determine order action - Bitget provides tradeSide which makes this easier
		orderAction := t.determineOrderAction(trade.Side, trade.PositionSide, trade.RealizedPnL)
		if trade.OrderAction != "" {
			orderAction = trade.OrderAction // Use if already provided
		}

		// Determine position side for position builder
		positionSide := "BOTH" // Bitget uses one-way mode

		// Normalize side for storage
		side := strings.ToUpper(trade.Side)

		// Create order record - use UTC time in milliseconds to avoid timezone issues
		execTimeMs := trade.Time.UTC().UnixMilli()
		orderRecord := &store.TraderOrder{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			ExchangeOrderID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			PositionSide:    positionSide,
			Type:            "MARKET",
			OrderAction:     orderAction,
			Quantity:        trade.Quantity,
			Price:           trade.Price,
			Status:          "FILLED",
			FilledQuantity:  trade.Quantity,
			AvgFillPrice:    trade.Price,
			Commission:      trade.Fee,
			FilledAt:        execTimeMs,
			CreatedAt:       execTimeMs,
			UpdatedAt:       execTimeMs,
		}

		// Insert order record
		if err := orderStore.CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync trade %s: %v", trade.TradeID, err)
			continue
		}

		// Create fill record - use UTC time in milliseconds
		fillRecord := &store.TraderFill{
			TraderID:        traderID,
			ExchangeID:      exchangeID,   // UUID
			ExchangeType:    exchangeType, // Exchange type
			OrderID:         orderRecord.ID,
			ExchangeOrderID: trade.TradeID,
			ExchangeTradeID: trade.TradeID,
			Symbol:          symbol,
			Side:            side,
			Price:           trade.Price,
			Quantity:        trade.Quantity,
			QuoteQuantity:   trade.Price * trade.Quantity,
			Commission:      trade.Fee,
			CommissionAsset: "USDT",
			RealizedPnL:     trade.RealizedPnL,
			IsMaker:         false,
			CreatedAt:       execTimeMs,
		}

		if err := orderStore.CreateFill(fillRecord); err != nil {
			logger.Infof("  ⚠️ Failed to sync fill for trade %s: %v", trade.TradeID, err)
		}

		// Create/update position record using PositionBuilder
		if err := posBuilder.ProcessTrade(
			traderID, exchangeID, exchangeType,
			symbol, positionSide, orderAction,
			trade.Quantity, trade.Price, trade.Fee, trade.RealizedPnL,
			execTimeMs, trade.TradeID,
		); err != nil {
			logger.Infof("  ⚠️ Failed to sync position for trade %s: %v", trade.TradeID, err)
		} else {
			logger.Infof("  📍 Position updated for trade: %s (action: %s, qty: %.6f)", trade.TradeID, orderAction, trade.Quantity)
		}

		syncedCount++
		logger.Infof("  ✅ Synced trade: %s %s %s qty=%.6f price=%.6f pnl=%.2f fee=%.6f action=%s time=%s(UTC)",
			trade.TradeID, symbol, side, trade.Quantity, trade.Price, trade.RealizedPnL, trade.Fee, orderAction,
			trade.Time.UTC().Format("01-02 15:04:05"))
	}

	// Update lastSyncTime to the LATEST trade time (not current time!)
	// This ensures next sync starts from where we left off, not from "now"
	// allTrades is already sorted by time ASC, so last element is the latest
	if len(allTrades) > 0 && len(failedSymbols) == 0 {
		latestTradeTimeMs := allTrades[len(allTrades)-1].Time.UTC().UnixMilli()
		bitgetSyncStateMutex.Lock()
		bitgetSyncState[exchangeID] = latestTradeTimeMs
		bitgetSyncStateMutex.Unlock()
		logger.Infof("📅 [Bitget] Updated lastSyncTime to latest trade: %s (UTC)",
			time.UnixMilli(latestTradeTimeMs).UTC().Format("2006-01-02 15:04:05"))
	} else if len(failedSymbols) > 0 {
		logger.Infof("  ⚠️ %d symbols failed, not updating lastSyncTime to retry next time: %v", len(failedSymbols), failedSymbols)
	}

	logger.Infof("✅ [Bitget] Order sync completed: %d new trades synced, %d skipped (already exist)", syncedCount, skippedCount)
	return nil
}

// StartOrderSync starts background order sync task for Bitget
func (t *BitgetTrader) StartOrderSync(traderID string, exchangeID string, exchangeType string, st *store.Store, interval time.Duration) {
	// Run first sync immediately
	go func() {
		logger.Infof("🔄 [Bitget] Running initial order sync...")
		if err := t.SyncOrdersFromBitget(traderID, exchangeID, exchangeType, st); err != nil {
			logger.Infof("⚠️ [Bitget] Initial order sync failed: %v", err)
		}
	}()

	// Then run periodically
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := t.SyncOrdersFromBitget(traderID, exchangeID, exchangeType, st); err != nil {
				logger.Infof("⚠️ [Bitget] Order sync failed: %v", err)
			}
		}
	}()
	logger.Infof("🔄 [Bitget] Order sync started (interval: %v)", interval)
}
