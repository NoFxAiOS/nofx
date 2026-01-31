package lighter

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tradertypes "nofx/trader/types"
)

// Test configuration - uses environment variables for security
// Run with:
//   LIGHTER_TEST=1 LIGHTER_WALLET=0x... LIGHTER_API_KEY=... LIGHTER_API_KEY_INDEX=2 go test -v ./trader -run TestLighter -timeout 300s
// Run with trading:
//   LIGHTER_TEST=1 LIGHTER_TRADE_TEST=1 LIGHTER_WALLET=0x... LIGHTER_API_KEY=... go test -v ./trader -run TestLighter -timeout 300s

// getTestConfig returns test configuration from environment variables
func getTestConfig() (walletAddr, apiKey string, apiKeyIndex int) {
	walletAddr = os.Getenv("LIGHTER_WALLET")
	apiKey = os.Getenv("LIGHTER_API_KEY")
	// All credentials must be provided via environment variables for security
	apiKeyIndex = 2 // Default to index 2 (more stable than index 0)
	if idx := os.Getenv("LIGHTER_API_KEY_INDEX"); idx != "" {
		fmt.Sscanf(idx, "%d", &apiKeyIndex)
	}
	return
}

func skipIfNoEnv(t *testing.T) {
	if os.Getenv("LIGHTER_TEST") != "1" {
		t.Skip("Skipping Lighter integration test. Set LIGHTER_TEST=1 to run")
	}
	if os.Getenv("LIGHTER_WALLET") == "" {
		t.Skip("Skipping: LIGHTER_WALLET environment variable not set")
	}
	if os.Getenv("LIGHTER_API_KEY") == "" {
		t.Skip("Skipping: LIGHTER_API_KEY environment variable not set")
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
	walletAddr, apiKey, apiKeyIndex := getTestConfig()
	trader, err := NewLighterTraderV2(walletAddr, apiKey, apiKeyIndex, false)
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

	// Verify account index is valid (non-zero)
	if trader.accountIndex <= 0 {
		t.Errorf("Expected valid account index, got %d", trader.accountIndex)
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

	orderID, _ := result["orderId"].(string)
	t.Logf("✅ Order created: %s", orderID)

	if orderID == "" {
		t.Fatal("Expected orderId in response")
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
	if os.Getenv("LIGHTER_TEST") != "1" || os.Getenv("LIGHTER_API_KEY") == "" {
		b.Skip("Skipping benchmark. Set LIGHTER_TEST=1 and LIGHTER_API_KEY to run")
	}

	walletAddr, apiKey, apiKeyIndex := getTestConfig()
	trader, err := NewLighterTraderV2(walletAddr, apiKey, apiKeyIndex, false)
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
	if os.Getenv("LIGHTER_TEST") != "1" || os.Getenv("LIGHTER_API_KEY") == "" {
		b.Skip("Skipping benchmark. Set LIGHTER_TEST=1 and LIGHTER_API_KEY to run")
	}

	walletAddr, apiKey, apiKeyIndex := getTestConfig()
	trader, err := NewLighterTraderV2(walletAddr, apiKey, apiKeyIndex, false)
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

// ==================== GetOpenOrders Tests ====================

func TestLighterGetOpenOrders(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Test GetOpenOrders
	orders, err := trader.GetOpenOrders("ETH")
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("GetOpenOrders failed: %v", err)
	}

	t.Logf("✅ GetOpenOrders: found %d open orders", len(orders))
	for i, order := range orders {
		if i >= 5 {
			t.Logf("   ... and %d more", len(orders)-5)
			break
		}
		t.Logf("   [%d] %s %s %s: qty=%.4f @ %.2f, status=%s",
			i+1, order.Symbol, order.Side, order.Type, order.Quantity, order.Price, order.Status)
	}
}

func TestLighterGetActiveOrders(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Test GetActiveOrders (internal API)
	orders, err := trader.GetActiveOrders("ETH")
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("GetActiveOrders failed: %v", err)
	}

	t.Logf("✅ GetActiveOrders: found %d active orders", len(orders))
	for i, order := range orders {
		if i >= 5 {
			t.Logf("   ... and %d more", len(orders)-5)
			break
		}
		t.Logf("   [%d] OrderID=%s, Type=%s, Price=%s, RemainingAmount=%s",
			i+1, order.OrderID, order.Type, order.Price, order.RemainingBaseAmount)
	}
}

// ==================== OrderBook Tests ====================

func TestLighterGetOrderBook(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Test GetOrderBook
	bids, asks, err := trader.GetOrderBook("ETH", 10)
	if err != nil {
		// OrderBook API may not be available in all regions or require special permissions
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "restricted") {
			t.Skipf("Skipping: OrderBook API not available: %v", err)
		}
		t.Fatalf("GetOrderBook failed: %v", err)
	}

	t.Logf("✅ GetOrderBook: %d bids, %d asks", len(bids), len(asks))

	if len(bids) > 0 {
		t.Logf("   Best Bid: %.2f @ %.4f", bids[0][0], bids[0][1])
	}
	if len(asks) > 0 {
		t.Logf("   Best Ask: %.2f @ %.4f", asks[0][0], asks[0][1])
	}

	// Verify spread makes sense
	if len(bids) > 0 && len(asks) > 0 {
		spread := asks[0][0] - bids[0][0]
		spreadPct := spread / bids[0][0] * 100
		t.Logf("   Spread: %.2f (%.4f%%)", spread, spreadPct)

		if spread < 0 {
			t.Error("Invalid spread: ask < bid")
		}
	}
}

// ==================== PlaceLimitOrder (GridTrader) Tests ====================

func TestLighterPlaceLimitOrder(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Get current market price
	marketPrice, err := trader.GetMarketPrice("ETH")
	if err != nil {
		t.Fatalf("Failed to get market price: %v", err)
	}
	t.Logf("Current ETH price: %.2f", marketPrice)

	// Create a limit order using PlaceLimitOrder (GridTrader interface)
	// Buy order at 75% of market price (won't fill)
	limitPrice := marketPrice * 0.75
	quantity := 0.01

	req := &tradertypes.LimitOrderRequest{
		Symbol:       "ETH",
		Side:         "BUY",
		PositionSide: "LONG",
		Price:        limitPrice,
		Quantity:     quantity,
		Leverage:     10,
		ClientID:     "test-order-001",
		ReduceOnly:   false,
	}

	t.Logf("Placing limit order via PlaceLimitOrder: %s %.4f @ %.2f", req.Side, req.Quantity, req.Price)

	result, err := trader.PlaceLimitOrder(req)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("PlaceLimitOrder failed: %v", err)
	}

	t.Logf("✅ PlaceLimitOrder result: OrderID=%s, Status=%s", result.OrderID, result.Status)

	if result.OrderID == "" {
		t.Fatal("Expected OrderID in result")
	}

	// Wait and cancel
	time.Sleep(3 * time.Second)

	// Cancel the order
	err = trader.CancelOrder("ETH", result.OrderID)
	if err != nil {
		t.Logf("⚠️ Failed to cancel order: %v", err)
	} else {
		t.Log("✅ Order cancelled successfully")
	}
}

// ==================== SetMarginMode Tests ====================

func TestLighterSetMarginMode(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Test setting cross margin
	t.Log("Setting margin mode to CROSS...")
	err := trader.SetMarginMode("ETH", true)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Errorf("SetMarginMode(cross) failed: %v", err)
	} else {
		t.Log("✅ SetMarginMode(cross) succeeded")
	}

	time.Sleep(2 * time.Second)

	// Note: Isolated margin may fail if there's an open position
	// Just test cross margin for safety
}

// ==================== Stop-Loss/Take-Profit Tests ====================

func TestLighterStopLossOrder(t *testing.T) {
	skipIfNoEnv(t)

	if os.Getenv("LIGHTER_TRADE_TEST") != "1" {
		t.Skip("Skipping stop-loss test. Set LIGHTER_TRADE_TEST=1 to run")
	}

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Check if we have a position first
	pos, err := trader.GetPosition("ETH")
	if err != nil {
		t.Fatalf("GetPosition failed: %v", err)
	}

	if pos == nil || pos.Size == 0 {
		t.Skip("No ETH position to set stop-loss for")
	}

	// Calculate stop-loss price (5% below entry for long, 5% above for short)
	var stopPrice float64
	if pos.Side == "long" {
		stopPrice = pos.EntryPrice * 0.95
	} else {
		stopPrice = pos.EntryPrice * 1.05
	}

	t.Logf("Position: %s %s, size=%.4f, entry=%.2f", pos.Symbol, pos.Side, pos.Size, pos.EntryPrice)
	t.Logf("Setting stop-loss at %.2f", stopPrice)

	err = trader.SetStopLoss("ETH", strings.ToUpper(pos.Side), pos.Size, stopPrice)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Errorf("SetStopLoss failed: %v", err)
	} else {
		t.Log("✅ SetStopLoss succeeded")
	}
}

func TestLighterTakeProfitOrder(t *testing.T) {
	skipIfNoEnv(t)

	if os.Getenv("LIGHTER_TRADE_TEST") != "1" {
		t.Skip("Skipping take-profit test. Set LIGHTER_TRADE_TEST=1 to run")
	}

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Check if we have a position first
	pos, err := trader.GetPosition("ETH")
	if err != nil {
		t.Fatalf("GetPosition failed: %v", err)
	}

	if pos == nil || pos.Size == 0 {
		t.Skip("No ETH position to set take-profit for")
	}

	// Calculate take-profit price (10% above entry for long, 10% below for short)
	var takeProfitPrice float64
	if pos.Side == "long" {
		takeProfitPrice = pos.EntryPrice * 1.10
	} else {
		takeProfitPrice = pos.EntryPrice * 0.90
	}

	t.Logf("Position: %s %s, size=%.4f, entry=%.2f", pos.Symbol, pos.Side, pos.Size, pos.EntryPrice)
	t.Logf("Setting take-profit at %.2f", takeProfitPrice)

	err = trader.SetTakeProfit("ETH", strings.ToUpper(pos.Side), pos.Size, takeProfitPrice)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Errorf("SetTakeProfit failed: %v", err)
	} else {
		t.Log("✅ SetTakeProfit succeeded")
	}
}

// ==================== Full Trading Flow Tests ====================

func TestLighterFullTradingFlow(t *testing.T) {
	skipIfNoEnv(t)

	if os.Getenv("LIGHTER_TRADE_TEST") != "1" {
		t.Skip("Skipping full trading flow test. Set LIGHTER_TRADE_TEST=1 to run")
	}

	trader := createTestTrader(t)
	defer trader.Cleanup()

	symbol := "ETH"
	quantity := 0.01 // Minimum quantity
	leverage := 10

	// Step 1: Get initial state
	t.Log("=== Step 1: Get Initial State ===")
	balance, _ := trader.GetBalance()
	if equity, ok := balance["total_equity"].(float64); ok {
		t.Logf("   Initial equity: %.2f", equity)
	}

	marketPrice, err := trader.GetMarketPrice(symbol)
	if err != nil {
		t.Fatalf("Failed to get market price: %v", err)
	}
	t.Logf("   Market price: %.2f", marketPrice)

	// Step 2: Set leverage
	t.Log("=== Step 2: Set Leverage ===")
	err = trader.SetLeverage(symbol, leverage)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("SetLeverage failed: %v", err)
	}
	t.Logf("   Leverage set to %dx", leverage)
	time.Sleep(2 * time.Second)

	// Step 3: Open Long Position
	t.Log("=== Step 3: Open Long Position ===")
	result, err := trader.OpenLong(symbol, quantity, leverage)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("OpenLong failed: %v", err)
	}
	t.Logf("   OpenLong result: %v", result)
	time.Sleep(3 * time.Second)

	// Step 4: Verify position
	t.Log("=== Step 4: Verify Position ===")
	pos, err := trader.GetPosition(symbol)
	if err != nil {
		t.Errorf("GetPosition failed: %v", err)
	} else if pos != nil {
		t.Logf("   Position: %s %s, size=%.4f, entry=%.2f, pnl=%.2f",
			pos.Symbol, pos.Side, pos.Size, pos.EntryPrice, pos.UnrealizedPnL)
	}

	// Step 5: Place limit order (sell at higher price)
	t.Log("=== Step 5: Place Limit Sell Order ===")
	limitPrice := marketPrice * 1.05 // 5% above market
	limitResult, err := trader.CreateOrder(symbol, true, quantity, limitPrice, "limit", true)
	if err != nil {
		t.Logf("   Failed to place limit order: %v", err)
	} else {
		t.Logf("   Limit order placed: %v", limitResult)
	}
	time.Sleep(2 * time.Second)

	// Step 6: Get open orders
	t.Log("=== Step 6: Get Open Orders ===")
	orders, err := trader.GetOpenOrders(symbol)
	if err != nil {
		t.Logf("   Failed to get open orders: %v", err)
	} else {
		t.Logf("   Open orders: %d", len(orders))
		for _, o := range orders {
			t.Logf("     - %s %s: qty=%.4f @ %.2f", o.Side, o.Type, o.Quantity, o.Price)
		}
	}

	// Step 7: Cancel all orders
	t.Log("=== Step 7: Cancel All Orders ===")
	err = trader.CancelAllOrders(symbol)
	if err != nil {
		t.Logf("   Failed to cancel orders: %v", err)
	} else {
		t.Log("   All orders cancelled")
	}
	time.Sleep(2 * time.Second)

	// Step 8: Close position
	t.Log("=== Step 8: Close Position ===")
	closeResult, err := trader.CloseLong(symbol, 0) // 0 = close all
	if err != nil {
		t.Errorf("CloseLong failed: %v", err)
	} else {
		t.Logf("   CloseLong result: %v", closeResult)
	}
	time.Sleep(3 * time.Second)

	// Step 9: Verify position closed
	t.Log("=== Step 9: Verify Position Closed ===")
	pos, _ = trader.GetPosition(symbol)
	if pos == nil || pos.Size == 0 {
		t.Log("   ✅ Position closed successfully")
	} else {
		t.Logf("   ⚠️ Position still exists: size=%.4f", pos.Size)
	}

	// Step 10: Get final balance
	t.Log("=== Step 10: Get Final State ===")
	balance, _ = trader.GetBalance()
	if equity, ok := balance["total_equity"].(float64); ok {
		t.Logf("   Final equity: %.2f", equity)
	}

	t.Log("=== Full Trading Flow Completed ===")
}

// ==================== API Key Validation Tests ====================

func TestLighterAPIKeyValid(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Check if API key is valid
	if trader.apiKeyValid {
		t.Log("✅ API key is VALID and matches server")
	} else {
		t.Error("❌ API key is INVALID - does not match server")
	}

	// Verify by checking the actual API key
	err := trader.checkClient()
	if err != nil {
		t.Errorf("API key verification error: %v", err)
	} else {
		t.Log("✅ API key verification passed")
	}
}

// ==================== Market Order Tests ====================

func TestLighterMarketOrderBuy(t *testing.T) {
	skipIfNoEnv(t)

	if os.Getenv("LIGHTER_TRADE_TEST") != "1" {
		t.Skip("Skipping market order test. Set LIGHTER_TRADE_TEST=1 to run")
	}

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Create a small market buy order
	quantity := 0.01
	t.Logf("Creating market buy order: %.4f ETH", quantity)

	result, err := trader.CreateOrder("ETH", false, quantity, 0, "market", false)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("Market buy failed: %v", err)
	}

	t.Logf("✅ Market buy result: %v", result)

	// Wait and close
	time.Sleep(3 * time.Second)

	// Close the position
	_, err = trader.CloseLong("ETH", quantity)
	if err != nil {
		t.Logf("⚠️ Failed to close position: %v", err)
	} else {
		t.Log("✅ Position closed")
	}
}

func TestLighterMarketOrderSell(t *testing.T) {
	skipIfNoEnv(t)

	if os.Getenv("LIGHTER_TRADE_TEST") != "1" {
		t.Skip("Skipping market order test. Set LIGHTER_TRADE_TEST=1 to run")
	}

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Create a small market sell order (short)
	quantity := 0.01
	t.Logf("Creating market sell order (short): %.4f ETH", quantity)

	result, err := trader.CreateOrder("ETH", true, quantity, 0, "market", false)
	skipIfJurisdictionRestricted(t, err)
	if err != nil {
		t.Fatalf("Market sell failed: %v", err)
	}

	t.Logf("✅ Market sell result: %v", result)

	// Wait and close
	time.Sleep(3 * time.Second)

	// Close the position
	_, err = trader.CloseShort("ETH", quantity)
	if err != nil {
		t.Logf("⚠️ Failed to close position: %v", err)
	} else {
		t.Log("✅ Position closed")
	}
}

// ==================== GetPosition Tests ====================

func TestLighterGetPosition(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Test GetPosition for ETH
	pos, err := trader.GetPosition("ETH")
	if err != nil {
		t.Fatalf("GetPosition failed: %v", err)
	}

	if pos == nil {
		t.Log("✅ No ETH position (pos is nil)")
	} else if pos.Size == 0 {
		t.Log("✅ No ETH position (size is 0)")
	} else {
		t.Logf("✅ ETH position found:")
		t.Logf("   Symbol: %s", pos.Symbol)
		t.Logf("   Side: %s", pos.Side)
		t.Logf("   Size: %.4f", pos.Size)
		t.Logf("   Entry Price: %.2f", pos.EntryPrice)
		t.Logf("   Mark Price: %.2f", pos.MarkPrice)
		t.Logf("   Liquidation Price: %.2f", pos.LiquidationPrice)
		t.Logf("   Unrealized PnL: %.2f", pos.UnrealizedPnL)
		t.Logf("   Leverage: %.1fx", pos.Leverage)
	}
}

// ==================== Symbol Normalization Tests ====================

func TestLighterSymbolNormalization(t *testing.T) {
	skipIfNoEnv(t)

	trader := createTestTrader(t)
	defer trader.Cleanup()

	// Test different symbol formats
	testCases := []struct {
		input    string
		expected string
	}{
		{"ETH", "ETH"},
		{"ETH-PERP", "ETH"},
		{"ETHUSDT", "ETH"},
		{"ETH/USDT", "ETH"},
		{"BTC", "BTC"},
		{"BTCUSDT", "BTC"},
	}

	for _, tc := range testCases {
		// Try to get market price with different formats
		price, err := trader.GetMarketPrice(tc.input)
		if err != nil {
			t.Logf("⚠️ GetMarketPrice(%s) failed: %v", tc.input, err)
		} else {
			t.Logf("✅ GetMarketPrice(%s) = %.2f", tc.input, price)
		}
	}
}
