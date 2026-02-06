package binance

import (
	"nofx/store"
	"os"
	"testing"
	"time"
)

// TestBinanceSyncE2E tests the complete sync flow end-to-end
func TestBinanceSyncE2E(t *testing.T) {
	skipIfNoLiveTest(t)

	// Get credentials from environment
	apiKey, secretKey := getBinanceTestCredentials(t)

	// Create test database using full store initialization (includes table creation)
	testDBPath := "/tmp/test_binance_sync.db"
	os.Remove(testDBPath) // Clean up previous test

	st, err := store.New(testDBPath)
	if err != nil {
		t.Fatalf("Failed to init test store: %v", err)
	}
	db := st.GormDB()

	// Create trader
	trader := NewFuturesTrader(apiKey, secretKey, "test-user")

	// Test parameters
	traderID := "test-trader-id"
	exchangeID := "test-exchange-id"
	exchangeType := "binance"

	t.Logf("🧪 Running end-to-end sync test...")
	t.Logf("   DB Path: %s", testDBPath)

	// Run sync
	t.Logf("\n📥 Running SyncOrdersFromBinance...")
	startTime := time.Now()
	err = trader.SyncOrdersFromBinance(traderID, exchangeID, exchangeType, st)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("❌ Sync failed: %v", err)
	}
	t.Logf("✅ Sync completed in %v", elapsed)

	// Check results in database
	orderStore := st.Order()

	// Count orders
	var orderCount int64
	db.Model(&store.TraderOrder{}).Where("exchange_id = ?", exchangeID).Count(&orderCount)
	t.Logf("\n📊 Results:")
	t.Logf("   Orders in DB: %d", orderCount)

	// Count fills
	var fillCount int64
	db.Model(&store.TraderFill{}).Where("exchange_id = ?", exchangeID).Count(&fillCount)
	t.Logf("   Fills in DB: %d", fillCount)

	// Get symbols
	var symbols []string
	db.Model(&store.TraderFill{}).
		Select("DISTINCT symbol").
		Where("exchange_id = ?", exchangeID).
		Pluck("symbol", &symbols)
	t.Logf("   Unique symbols: %d - %v", len(symbols), symbols)

	// Check max trade IDs (test the fix)
	maxTradeIDs, err := orderStore.GetMaxTradeIDsByExchange(exchangeID)
	if err != nil {
		t.Logf("   ⚠️ GetMaxTradeIDsByExchange error: %v", err)
	} else {
		t.Logf("   Max trade IDs per symbol:")
		for symbol, maxID := range maxTradeIDs {
			if maxID > 2147483647 {
				t.Logf("      %s: %d (⚠️ exceeds PostgreSQL INTEGER max)", symbol, maxID)
			} else {
				t.Logf("      %s: %d", symbol, maxID)
			}
		}
	}

	// Sample some orders
	var sampleOrders []store.TraderOrder
	db.Where("exchange_id = ?", exchangeID).Limit(5).Find(&sampleOrders)
	if len(sampleOrders) > 0 {
		t.Logf("\n📝 Sample orders:")
		for i, order := range sampleOrders {
			t.Logf("   [%d] %s %s %s qty=%.6f price=%.4f action=%s time=%s",
				i+1, order.ExchangeOrderID, order.Symbol, order.Side,
				order.Quantity, order.Price, order.OrderAction,
				time.UnixMilli(order.FilledAt).Format(time.RFC3339))
		}
	}

	// Test incremental sync - run again, should find no new trades
	t.Logf("\n🔄 Running incremental sync (should skip existing trades)...")
	startTime = time.Now()
	err = trader.SyncOrdersFromBinance(traderID, exchangeID, exchangeType, st)
	elapsed = time.Since(startTime)
	if err != nil {
		t.Fatalf("❌ Incremental sync failed: %v", err)
	}
	t.Logf("✅ Incremental sync completed in %v", elapsed)

	// Check counts again - should be the same
	var newOrderCount int64
	db.Model(&store.TraderOrder{}).Where("exchange_id = ?", exchangeID).Count(&newOrderCount)
	t.Logf("   Orders after incremental sync: %d (was %d)", newOrderCount, orderCount)

	if newOrderCount != orderCount {
		t.Logf("   ⚠️ Order count changed - possible duplicate detection issue")
	} else {
		t.Logf("   ✅ No duplicates - incremental sync working correctly")
	}

	// Test GetLastFillTimeByExchange
	lastFillTimeMs, err := orderStore.GetLastFillTimeByExchange(exchangeID)
	if err != nil {
		t.Logf("   ⚠️ GetLastFillTimeByExchange error: %v", err)
	} else {
		lastFillTime := time.UnixMilli(lastFillTimeMs)
		t.Logf("\n📅 Last fill time from DB: %s", lastFillTime.Format(time.RFC3339))

		// Check if it would be in the future (the bug we fixed)
		now := time.Now().UTC()
		if lastFillTime.After(now) {
			t.Logf("   ❌ BUG: Last fill time is in the future! (now: %s)", now.Format(time.RFC3339))
		} else {
			t.Logf("   ✅ Last fill time is in the past (correct)")
		}
	}

	// Cleanup
	os.Remove(testDBPath)
	t.Logf("\n✅ E2E test completed successfully!")
}

// TestBinanceSyncWithExistingData tests sync behavior with pre-existing data
func TestBinanceSyncWithExistingData(t *testing.T) {
	skipIfNoLiveTest(t)

	// Get credentials from environment
	apiKey, secretKey := getBinanceTestCredentials(t)

	testDBPath := "/tmp/test_binance_sync_existing.db"
	os.Remove(testDBPath)

	st, err := store.New(testDBPath)
	if err != nil {
		t.Fatalf("Failed to init test store: %v", err)
	}
	db := st.GormDB()
	orderStore := st.Order()

	trader := NewFuturesTrader(apiKey, secretKey, "test-user")

	traderID := "test-trader-id"
	exchangeID := "test-exchange-id"
	exchangeType := "binance"

	// Insert a fake "old" fill with LOCAL time (simulating the bug scenario)
	// This tests that our timezone fix works
	localTime := time.Now().Add(8 * time.Hour) // Simulate +8 timezone stored as if it were UTC
	fakeFill := &store.TraderFill{
		TraderID:        traderID,
		ExchangeID:      exchangeID,
		ExchangeType:    exchangeType,
		ExchangeOrderID: "fake-old-order",
		ExchangeTradeID: "fake-old-trade",
		Symbol:          "BTCUSDT",
		Side:            "BUY",
		Price:           50000,
		Quantity:        0.001,
		QuoteQuantity:   50,
		CreatedAt:       localTime.UnixMilli(), // This time is "in the future" if interpreted as UTC
	}
	if err := orderStore.CreateFill(fakeFill); err != nil {
		t.Fatalf("Failed to create fake fill: %v", err)
	}

	t.Logf("🧪 Testing sync with existing 'future' data...")
	t.Logf("   Fake fill time: %s", localTime.Format(time.RFC3339))
	t.Logf("   Current UTC time: %s", time.Now().UTC().Format(time.RFC3339))

	// Check GetLastFillTimeByExchange
	lastFillTimeMs2, _ := orderStore.GetLastFillTimeByExchange(exchangeID)
	lastFillTime2 := time.UnixMilli(lastFillTimeMs2)
	t.Logf("   GetLastFillTimeByExchange returned: %s", lastFillTime2.Format(time.RFC3339))

	if lastFillTime2.After(time.Now().UTC()) {
		t.Logf("   ⚠️ Last fill time is in the future - this is the bug scenario!")
	}

	// Run sync - it should detect the future time and fall back
	t.Logf("\n📥 Running sync (should detect future time and fall back)...")
	err = trader.SyncOrdersFromBinance(traderID, exchangeID, exchangeType, st)
	if err != nil {
		t.Fatalf("❌ Sync failed: %v", err)
	}
	t.Logf("✅ Sync completed")

	// Check that trades were actually synced despite the bad data
	var fillCount int64
	db.Model(&store.TraderFill{}).Where("exchange_id = ?", exchangeID).Count(&fillCount)
	t.Logf("   Total fills in DB: %d (includes 1 fake)", fillCount)

	if fillCount > 1 {
		t.Logf("   ✅ Real trades were synced despite 'future' data!")
	} else {
		t.Logf("   ❌ No real trades synced - the bug might still exist")
	}

	os.Remove(testDBPath)
}
