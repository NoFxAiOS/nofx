package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"sync"
	"time"
)

// syncState stores the last sync time for MAX
var (
	maxSyncState      = make(map[string]int64) // exchangeID -> lastSyncTimeMs
	maxSyncStateMutex sync.RWMutex
)

// SyncOrdersFromMAX syncs MAX Exchange trade history to local database
// MAX is a spot exchange, so sync logic is simpler than futures
func (t *MAXTrader) SyncOrdersFromMAX(traderID string, exchangeID string, exchangeType string, st *store.Store) error {
	if st == nil {
		return fmt.Errorf("store is nil")
	}

	orderStore := st.Order()

	// Get last sync time
	maxSyncStateMutex.RLock()
	lastSyncTimeMs, exists := maxSyncState[exchangeID]
	maxSyncStateMutex.RUnlock()

	nowMs := time.Now().UTC().UnixMilli()
	if !exists {
		// Try to get last fill time from database
		lastFillTimeMs, err := orderStore.GetLastFillTimeByExchange(exchangeID)
		if err == nil && lastFillTimeMs > 0 {
			if lastFillTimeMs > nowMs {
				lastSyncTimeMs = nowMs - 24*60*60*1000 // 24 hours ago
			} else {
				lastSyncTimeMs = lastFillTimeMs + 1000
			}
		} else {
			// First sync: go back 24 hours
			lastSyncTimeMs = nowMs - 24*60*60*1000
		}
	}

	logger.Infof("🔄 Syncing MAX trades from: %s (UTC)",
		time.UnixMilli(lastSyncTimeMs).UTC().Format("2006-01-02 15:04:05"))

	// Get recent trades from MAX
	startTime := time.UnixMilli(lastSyncTimeMs)
	records, err := t.GetClosedPnL(startTime, 100)
	if err != nil {
		return fmt.Errorf("failed to get MAX trades: %w", err)
	}

	if len(records) == 0 {
		logger.Infof("  ✓ No new trades to sync from MAX")
		return nil
	}

	// Process and save trades
	newFillsCount := 0
	latestFillTime := lastSyncTimeMs

	for _, record := range records {
		// Check if fill already exists
		existing, _ := orderStore.GetFillByExchangeTradeID(exchangeID, record.ExchangeID)
		if existing != nil {
			continue
		}

		// Create fill record
		fill := &store.TraderFill{
			ExchangeID:      exchangeID,
			TraderID:        traderID,
			ExchangeType:    exchangeType,
			OrderID:         0, // MAX sync doesn't track orders
			ExchangeOrderID: "",
			ExchangeTradeID: record.ExchangeID,
			Symbol:          record.Symbol,
			Side:            record.Side,
			Price:           record.EntryPrice,
			Quantity:        record.Quantity,
			QuoteQuantity:   record.EntryPrice * record.Quantity,
			Commission:      record.Fee,
			CommissionAsset: "TWD", // MAX uses TWD for fees
			RealizedPnL:     record.RealizedPnL,
			IsMaker:         false,
			CreatedAt:       record.EntryTime.UnixMilli(),
		}

		// Save fill
		if err := orderStore.CreateFill(fill); err != nil {
			logger.Warnf("  ⚠️ Failed to save fill: %v", err)
			continue
		}

		newFillsCount++

		// Track latest fill time
		fillTimeMs := record.EntryTime.UnixMilli()
		if fillTimeMs > latestFillTime {
			latestFillTime = fillTimeMs
		}
	}

	// Update sync state
	if latestFillTime > lastSyncTimeMs {
		maxSyncStateMutex.Lock()
		maxSyncState[exchangeID] = latestFillTime
		maxSyncStateMutex.Unlock()
	}

	logger.Infof("  ✓ MAX sync complete: %d new fills saved", newFillsCount)
	return nil
}

// ResetMAXSyncState resets the sync state for an exchange
func ResetMAXSyncState(exchangeID string) {
	maxSyncStateMutex.Lock()
	delete(maxSyncState, exchangeID)
	maxSyncStateMutex.Unlock()
}
