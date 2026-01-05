package trader

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Test configuration - uses real account
// Run with: LIGHTER_TEST=1 go test -v ./trader -run TestLighter -timeout 120s
const (
	testWalletAddr       = ""
	testAPIKeyPrivateKey = ""
	testAPIKeyIndex      = 0
	testAccountIndex     = int64(681514)
)

func skipIfNoEnv(t *testing.T) {
	if os.Getenv("LIGHTER_TEST") != "1" {
		t.Skip("Skipping Lighter integration test. Set LIGHTER_TEST=1 to run")
	}
}

// skipIfJurisdictionRestricted checks if error is due to geographic restriction
// and skips the test if so (this is expected when running from restricted regions)
func skipIfJurisdictionRestricted(t *testing.T, err error) {
	if err != nil && strings.Contains(err.Error(), "restricted jurisdiction") {
		t.Skip("Skipping: API blocked due to geographic restriction (IP-based). Use VPN to allowed region.")
	}
}

func createTestTrader(t *testing.T) *LighterTraderV2 {
	trader, err := NewLighterTraderV2(testWalletAddr, testAPIKeyPrivateKey, testAPIKeyIndex, false)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}
	return trader
}

// ==================== Account Tests ====================

func TestLighterAccountInit(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Verify account index
	if trader.accountIndex != testAccountIndex {
		t.Errorf("Expected account index %d, got %d", testAccountIndex, trader.accountIndex)
	}

	t.Logf("✅ Account initialized: index=%d", trader.accountIndex)
}

func TestLighterAPIKeyVerification(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Verify API key
	err := trader.checkClient()
	if err != nil {
		t.Errorf("API key verification failed: %v", err)
	} else {
		t.Log("✅ API key verified successfully")
	}
}

func TestLighterGetBalance(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("GetBalance failed: %v", err)
	}

	t.Logf("✅ Balance retrieved:")
	if te, ok := balance["total_equity"].(float64); ok {
		t.Logf("   Total Equity: %.2f", te)
	}
	if ab, ok := balance["available_balance"].(float64); ok {
		t.Logf("   Available Balance: %.2f", ab)
	}
	if mu, ok := balance["margin_used"].(float64); ok {
		t.Logf("   Margin Used: %.2f", mu)
	}
	if up, ok := balance["unrealized_pnl"].(float64); ok {
		t.Logf("   Unrealized PnL: %.2f", up)
	}

	if len(balance) == 0 {
		t.Error("Expected balance data")
	}
}

// ==================== Position Tests ====================

func TestLighterGetPositions(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("GetPositions failed: %v", err)
	}

	t.Logf("✅ Positions retrieved: %d positions", len(positions))
	for i, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		size, _ := pos["size"].(float64)
		entryPrice, _ := pos["entry_price"].(float64)
		unrealizedPnl, _ := pos["unrealized_pnl"].(float64)

		t.Logf("   [%d] %s %s: size=%.4f, entry=%.2f, pnl=%.2f",
			i+1, symbol, side, size, entryPrice, unrealizedPnl)
	}
}

// ==================== Market Data Tests ====================

func TestLighterGetMarketPrice(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	symbols := []string{"ETH", "BTC", "SOL"}

	for _, symbol := range symbols {
		price, err := trader.GetMarketPrice(symbol)
		if err != nil {
			t.Errorf("GetMarketPrice(%s) failed: %v", symbol, err)
			continue
		}
		t.Logf("✅ %s price: %.2f", symbol, price)

		if price <= 0 {
			t.Errorf("Expected positive price for %s, got %.2f", symbol, price)
		}
	}
}

func TestLighterFetchMarketList(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	markets, err := trader.fetchMarketList()
	if err != nil {
		t.Fatalf("fetchMarketList failed: %v", err)
	}

	t.Logf("✅ Markets retrieved: %d markets", len(markets))
	for i, m := range markets {
		if i >= 10 {
			t.Logf("   ... and %d more", len(markets)-10)
			break
		}
		t.Logf("   [%d] %s (market_id=%d, size_decimals=%d, price_decimals=%d)",
			m.MarketID, m.Symbol, m.MarketID, m.SizeDecimals, m.PriceDecimals)
	}

	if len(markets) == 0 {
		t.Error("Expected at least one market")
	}
}

// ==================== Trades API Tests ====================

func TestLighterGetTrades(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Get trades from last 7 days
	startTime := time.Now().Add(-7 * 24 * time.Hour)
	trades, err := trader.GetTrades(startTime, 100)
	if err != nil {
		t.Fatalf("GetTrades failed: %v", err)
	}

	t.Logf("✅ Trades retrieved: %d trades", len(trades))
	for i, trade := range trades {
		if i >= 5 {
			t.Logf("   ... and %d more", len(trades)-5)
			break
		}
		t.Logf("   [%d] %s %s: qty=%.4f @ %.2f, fee=%.6f, time=%s",
			i+1, trade.Symbol, trade.Side, trade.Quantity, trade.Price, trade.Fee,
			trade.Time.Format("2006-01-02 15:04:05"))
	}
}

func TestLighterGetClosedPnL(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	startTime := time.Now().Add(-7 * 24 * time.Hour)
	records, err := trader.GetClosedPnL(startTime, 100)
	if err != nil {
		t.Fatalf("GetClosedPnL failed: %v", err)
	}

	t.Logf("✅ Closed PnL records: %d records", len(records))
	for i, r := range records {
		if i >= 5 {
			t.Logf("   ... and %d more", len(records)-5)
			break
		}
		t.Logf("   [%d] %s %s: qty=%.4f, entry=%.2f, exit=%.2f, pnl=%.2f",
			i+1, r.Symbol, r.Side, r.Quantity, r.EntryPrice, r.ExitPrice, r.RealizedPnL)
	}
}

// ==================== Order Tests ====================

func TestLighterCreateAndCancelLimitOrder(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Get current market price
	marketPrice, err := trader.GetMarketPrice("ETH")
	if err != nil {
		t.Fatalf("Failed to get market price: %v", err)
	}
	t.Logf("Current ETH price: %.2f", marketPrice)

	// Create a limit order far from market (won't fill)
	// Buy order at 80% of market price
	limitPrice := marketPrice * 0.80
	quantity := 0.01 // Minimum quantity

	t.Logf("Creating limit buy order: %.4f ETH @ %.2f", quantity, limitPrice)

	result, err := trader.CreateOrder("ETH", false, quantity, limitPrice, "limit", false)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	orderID, _ := result["order_id"].(string)
	t.Logf("✅ Order created: %s", orderID)

	if orderID == "" {
		t.Fatal("Expected order ID in response")
	}

	// Wait a moment for order to be processed
	time.Sleep(3 * time.Second)

	// Cancel the order
	t.Logf("Cancelling order: %s", orderID)
	err = trader.CancelOrder("ETH", orderID)
	if err != nil {
		t.Errorf("CancelOrder failed: %v", err)
	} else {
		t.Log("✅ Order cancelled successfully")
	}
}

func TestLighterCancelAllOrders(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// First create a few test orders
	marketPrice, err := trader.GetMarketPrice("ETH")
	if err != nil {
		t.Fatalf("Failed to get market price: %v", err)
	}

	// Create 2 limit orders
	for i := 0; i < 2; i++ {
		limitPrice := marketPrice * (0.75 - float64(i)*0.05) // 75%, 70% of market
		_, err := trader.CreateOrder("ETH", false, 0.01, limitPrice, "limit", false)
		skipIfJurisdictionRestricted(t, err)
		if err != nil {
			t.Logf("Failed to create test order %d: %v", i+1, err)
		} else {
			t.Logf("Created test order %d @ %.2f", i+1, limitPrice)
		}
	}

	time.Sleep(3 * time.Second)

	// Cancel all
	err = trader.CancelAllOrders("ETH")
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Errorf("CancelAllOrders failed: %v", err)
	} else {
		t.Log("✅ CancelAllOrders executed")
	}
}

// ==================== Trading Flow Tests ====================

func TestLighterOpenCloseLongFlow(t *testing.T) {
	skipIfNoEnv(t)

	// This test actually trades - be careful!
	if os.Getenv("LIGHTER_TRADE_TEST") != "1" {
		t.Skip("Skipping actual trade test. Set LIGHTER_TRADE_TEST=1 to run")
	}

	trader := createTestTrader(t)
	defer trader.Cleanup()

	symbol := "ETH"
	quantity := 0.01 // Minimum quantity
	leverage := 10

	// Get initial positions
	positionsBefore, _ := trader.GetPositions()
	t.Logf("Positions before: %d", len(positionsBefore))

	// Open long
	t.Logf("Opening long: %s qty=%.4f leverage=%d", symbol, quantity, leverage)
	result, err := trader.OpenLong(symbol, quantity, leverage)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("OpenLong failed: %v", err)
	}
	t.Logf("✅ OpenLong result: %v", result)

	time.Sleep(3 * time.Second)

	// Verify position
	positions, _ := trader.GetPositions()
	t.Logf("Positions after open: %d", len(positions))

	// Close long
	t.Logf("Closing long: %s qty=%.4f", symbol, quantity)
	result, err = trader.CloseLong(symbol, quantity)
	if err != nil {
		t.Errorf("CloseLong failed: %v", err)
	} else {
		t.Logf("✅ CloseLong result: %v", result)
	}

	time.Sleep(3 * time.Second)

	// Verify position closed
	positions, _ = trader.GetPositions()
	t.Logf("Positions after close: %d", len(positions))
}

func TestLighterOpenCloseShortFlow(t *testing.T) {
	skipIfNoEnv(t)

	if os.Getenv("LIGHTER_TRADE_TEST") != "1" {
		t.Skip("Skipping actual trade test. Set LIGHTER_TRADE_TEST=1 to run")
	}

	trader := createTestTrader(t)
	defer trader.Cleanup()

	symbol := "ETH"
	quantity := 0.01
	leverage := 10

	// Open short
	t.Logf("Opening short: %s qty=%.4f leverage=%d", symbol, quantity, leverage)
	result, err := trader.OpenShort(symbol, quantity, leverage)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("OpenShort failed: %v", err)
	}
	t.Logf("✅ OpenShort result: %v", result)

	time.Sleep(3 * time.Second)

	// Close short
	t.Logf("Closing short: %s qty=%.4f", symbol, quantity)
	result, err = trader.CloseShort(symbol, quantity)
	if err != nil {
		t.Errorf("CloseShort failed: %v", err)
	} else {
		t.Logf("✅ CloseShort result: %v", result)
	}
}

// ==================== Leverage Tests ====================

func TestLighterSetLeverage(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Test setting leverage
	leverages := []int{5, 10, 20}

	for _, lev := range leverages {
		err := trader.SetLeverage("ETH", lev)
		skipIfJurisdictionRestricted(t, err)
		if err != nil {
			t.Errorf("SetLeverage(%d) failed: %v", lev, err)
		} else {
			t.Logf("✅ SetLeverage(%d) succeeded", lev)
		}
		time.Sleep(1 * time.Second)
	}
}

// ==================== Auth Token Tests ====================

func TestLighterAuthTokenRefresh(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Get initial token
	err := trader.ensureAuthToken()
	if err != nil {
		t.Fatalf("ensureAuthToken failed: %v", err)
	}
	t.Logf("✅ Initial auth token obtained")

	// Force refresh
	err = trader.refreshAuthToken()
	if err != nil {
		t.Errorf("refreshAuthToken failed: %v", err)
	} else {
		t.Log("✅ Auth token refreshed successfully")
	}

	// Verify token works by making API call
	_, err = trader.GetBalance()
	if err != nil {
		t.Errorf("GetBalance after refresh failed: %v", err)
	} else {
		t.Log("✅ Token verified working after refresh")
	}
}

// ==================== Error Handling Tests ====================

func TestLighterInvalidSymbol(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Test with invalid symbol
	_, err := trader.GetMarketPrice("INVALID_SYMBOL_XYZ")
	if err == nil {
		t.Error("Expected error for invalid symbol, got nil")
	} else {
		t.Logf("✅ Got expected error for invalid symbol: %v", err)
	}
}

func TestLighterCancelNonExistentOrder(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Try to cancel non-existent order
	err := trader.CancelOrder("ETH", "999999999999")
	if err == nil {
		t.Log("⚠️ No error for cancelling non-existent order (may be expected)")
	} else {
		t.Logf("✅ Got error for non-existent order: %v", err)
	}
}

// ==================== OrderSync Tests ====================

func TestLighterOrderSync(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Get trades to simulate order sync
	startTime := time.Now().Add(-24 * time.Hour)
	trades, err := trader.GetTrades(startTime, 50)
	if err != nil {
		t.Fatalf("GetTrades failed: %v", err)
	}

	t.Logf("✅ OrderSync simulation: retrieved %d trades", len(trades))

	// Analyze trades
	openTrades := 0
	closeTrades := 0
	for _, trade := range trades {
		if trade.OrderAction == "open_long" || trade.OrderAction == "open_short" {
			openTrades++
		} else if trade.OrderAction == "close_long" || trade.OrderAction == "close_short" {
			closeTrades++
		}
	}

	t.Logf("   Open trades: %d, Close trades: %d", openTrades, closeTrades)
}

// ==================== Benchmark Tests ====================

func BenchmarkLighterGetBalance(b *testing.B) {
	if os.Getenv("LIGHTER_TEST") != "1" {
		b.Skip("Skipping benchmark. Set LIGHTER_TEST=1 to run")
	}

	trader, err := NewLighterTraderV2(testWalletAddr, testAPIKeyPrivateKey, testAPIKeyIndex, false)
	if err != nil {
		b.Fatalf("Failed to create trader: %v", err)
	}
	defer trader.Cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := trader.GetBalance()
		if err != nil {
			b.Fatalf("GetBalance failed: %v", err)
		}
	}
}

func BenchmarkLighterGetMarketPrice(b *testing.B) {
	if os.Getenv("LIGHTER_TEST") != "1" {
		b.Skip("Skipping benchmark. Set LIGHTER_TEST=1 to run")
	}

	trader, err := NewLighterTraderV2(testWalletAddr, testAPIKeyPrivateKey, testAPIKeyIndex, false)
	if err != nil {
		b.Fatalf("Failed to create trader: %v", err)
	}
	defer trader.Cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := trader.GetMarketPrice("ETH")
		if err != nil {
			b.Fatalf("GetMarketPrice failed: %v", err)
		}
	}
}
