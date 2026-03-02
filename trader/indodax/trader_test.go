package indodax

import (
	"os"
	"testing"
	"time"

	"nofx/trader/types"
)

// Test credentials - set via environment variables
func getIndodaxTestCredentials(t *testing.T) (string, string) {
	apiKey := os.Getenv("INDODAX_TEST_API_KEY")
	secretKey := os.Getenv("INDODAX_TEST_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		t.Skip("Indodax test credentials not set (INDODAX_TEST_API_KEY, INDODAX_TEST_SECRET_KEY)")
	}

	return apiKey, secretKey
}

func createIndodaxTestTrader(t *testing.T) *IndodaxTrader {
	apiKey, secretKey := getIndodaxTestCredentials(t)
	trader := NewIndodaxTrader(apiKey, secretKey)
	return trader
}

// TestIndodaxTrader_InterfaceCompliance tests that IndodaxTrader implements types.Trader
func TestIndodaxTrader_InterfaceCompliance(t *testing.T) {
	var _ types.Trader = (*IndodaxTrader)(nil)
}

// TestNewIndodaxTrader tests creating Indodax trader instance
func TestNewIndodaxTrader(t *testing.T) {
	trader := NewIndodaxTrader("test_api_key", "test_secret_key")

	if trader == nil {
		t.Fatal("Expected non-nil trader")
	}
	if trader.apiKey != "test_api_key" {
		t.Errorf("Expected apiKey 'test_api_key', got '%s'", trader.apiKey)
	}
	if trader.secretKey != "test_secret_key" {
		t.Errorf("Expected secretKey 'test_secret_key', got '%s'", trader.secretKey)
	}
	if trader.httpClient == nil {
		t.Error("Expected non-nil httpClient")
	}
	if trader.cacheDuration != 15*time.Second {
		t.Errorf("Expected cacheDuration 15s, got %v", trader.cacheDuration)
	}
}

// TestIndodaxTrader_SymbolConversion tests symbol format conversion
func TestIndodaxTrader_SymbolConversion(t *testing.T) {
	trader := NewIndodaxTrader("test", "test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"BTCIDR to btc_idr", "BTCIDR", "btc_idr"},
		{"ETHIDR to eth_idr", "ETHIDR", "eth_idr"},
		{"SOLIDR to sol_idr", "SOLIDR", "sol_idr"},
		{"Already converted", "btc_idr", "btc_idr"},
		{"BTC pair", "ETHBTC", "eth_btc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trader.convertSymbol(tt.input)
			if result != tt.expected {
				t.Errorf("convertSymbol(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIndodaxTrader_SymbolConversionBack tests symbol reversion
func TestIndodaxTrader_SymbolConversionBack(t *testing.T) {
	trader := NewIndodaxTrader("test", "test")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"btc_idr to BTCIDR", "btc_idr", "BTCIDR"},
		{"eth_idr to ETHIDR", "eth_idr", "ETHIDR"},
		{"Already standard", "BTCIDR", "BTCIDR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trader.convertSymbolBack(tt.input)
			if result != tt.expected {
				t.Errorf("convertSymbolBack(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIndodaxTrader_GetCoinFromSymbol tests coin extraction
func TestIndodaxTrader_GetCoinFromSymbol(t *testing.T) {
	trader := NewIndodaxTrader("test", "test")

	tests := []struct {
		input    string
		expected string
	}{
		{"BTCIDR", "btc"},
		{"ETHIDR", "eth"},
		{"btc_idr", "btc"},
		{"eth_idr", "eth"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := trader.getCoinFromSymbol(tt.input)
			if result != tt.expected {
				t.Errorf("getCoinFromSymbol(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIndodaxTrader_Sign tests HMAC-SHA512 signature generation
func TestIndodaxTrader_Sign(t *testing.T) {
	trader := NewIndodaxTrader("api_key", "secret_key")

	body := "method=getInfo&nonce=1000"
	signature := trader.sign(body)

	if signature == "" {
		t.Error("Expected non-empty signature")
	}
	if len(signature) != 128 { // SHA-512 hex = 128 chars
		t.Errorf("Expected signature length 128, got %d", len(signature))
	}

	// Same input should produce same signature
	signature2 := trader.sign(body)
	if signature != signature2 {
		t.Error("Signature should be deterministic")
	}

	// Different input should produce different signature
	signature3 := trader.sign("method=getInfo&nonce=1001")
	if signature == signature3 {
		t.Error("Different input should produce different signature")
	}
}

// TestIndodaxTrader_Nonce tests nonce incrementation
func TestIndodaxTrader_Nonce(t *testing.T) {
	trader := NewIndodaxTrader("test", "test")

	nonce1 := trader.getNonce()
	nonce2 := trader.getNonce()
	nonce3 := trader.getNonce()

	if nonce2 <= nonce1 {
		t.Errorf("Nonce should be increasing: %d <= %d", nonce2, nonce1)
	}
	if nonce3 <= nonce2 {
		t.Errorf("Nonce should be increasing: %d <= %d", nonce3, nonce2)
	}
}

// TestIndodaxTrader_SpotOnlyRestrictions tests that futures-only methods return errors
func TestIndodaxTrader_SpotOnlyRestrictions(t *testing.T) {
	trader := NewIndodaxTrader("test", "test")

	// OpenShort should fail
	_, err := trader.OpenShort("BTCIDR", 0.001, 1)
	if err == nil {
		t.Error("OpenShort should return error on spot exchange")
	}

	// CloseShort should fail
	_, err = trader.CloseShort("BTCIDR", 0.001)
	if err == nil {
		t.Error("CloseShort should return error on spot exchange")
	}

	// SetStopLoss should fail
	err = trader.SetStopLoss("BTCIDR", "LONG", 0.001, 500000000)
	if err == nil {
		t.Error("SetStopLoss should return error on spot exchange")
	}

	// SetTakeProfit should fail
	err = trader.SetTakeProfit("BTCIDR", "LONG", 0.001, 600000000)
	if err == nil {
		t.Error("SetTakeProfit should return error on spot exchange")
	}

	// SetLeverage should NOT fail (no-op)
	err = trader.SetLeverage("BTCIDR", 10)
	if err != nil {
		t.Errorf("SetLeverage should not fail (no-op): %v", err)
	}

	// SetMarginMode should NOT fail (no-op)
	err = trader.SetMarginMode("BTCIDR", true)
	if err != nil {
		t.Errorf("SetMarginMode should not fail (no-op): %v", err)
	}
}

// TestIndodaxTrader_ParseFloat tests parseFloat helper
func TestIndodaxTrader_ParseFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"float64", 123.45, 123.45},
		{"string", "123.45", 123.45},
		{"int", 123, 123.0},
		{"int64", int64(123), 123.0},
		{"nil", nil, 0.0},
		{"zero string", "0", 0.0},
		{"empty string", "", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFloat(tt.input)
			if result != tt.expected {
				t.Errorf("parseFloat(%v) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIndodaxTrader_ClearCache tests cache clearing
func TestIndodaxTrader_ClearCache(t *testing.T) {
	trader := NewIndodaxTrader("test", "test")

	// Set some cached data
	trader.cachedBalance = map[string]interface{}{"test": "data"}
	trader.cachedPositions = []map[string]interface{}{{"test": "data"}}

	// Clear cache
	trader.clearCache()

	if trader.cachedBalance != nil {
		t.Error("Cache should be cleared")
	}
	if trader.cachedPositions != nil {
		t.Error("Position cache should be cleared")
	}
}

// ============================================================
// Integration tests (require INDODAX_TEST_API_KEY env vars)
// ============================================================

// TestIndodaxConnection tests basic API connectivity
func TestIndodaxConnection(t *testing.T) {
	trader := createIndodaxTestTrader(t)

	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}

	t.Logf("✅ Connection OK")
	t.Logf("  totalWalletBalance: %v", balance["totalWalletBalance"])
	t.Logf("  availableBalance: %v", balance["availableBalance"])
	t.Logf("  totalEquity: %v", balance["totalEquity"])
	t.Logf("  currency: %v", balance["currency"])
	t.Logf("  user_id: %v", balance["user_id"])
}

// TestIndodaxGetPositions tests position retrieval
func TestIndodaxGetPositions(t *testing.T) {
	trader := createIndodaxTestTrader(t)

	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("Failed to get positions: %v", err)
	}

	t.Logf("📊 Found %d positions (crypto balances):", len(positions))
	for i, pos := range positions {
		t.Logf("  [%d] %s: qty=%.8f markPrice=%.0f value=%.0f IDR",
			i+1,
			pos["symbol"],
			pos["positionAmt"],
			pos["markPrice"],
			pos["notionalValue"],
		)
	}
}

// TestIndodaxGetMarketPrice tests market price retrieval
func TestIndodaxGetMarketPrice(t *testing.T) {
	trader := createIndodaxTestTrader(t)

	pairs := []string{"BTCIDR", "ETHIDR"}

	for _, pair := range pairs {
		price, err := trader.GetMarketPrice(pair)
		if err != nil {
			t.Errorf("Failed to get price for %s: %v", pair, err)
			continue
		}
		t.Logf("  %s: %.0f IDR", pair, price)
	}
}

// TestIndodaxGetOpenOrders tests open orders retrieval
func TestIndodaxGetOpenOrders(t *testing.T) {
	trader := createIndodaxTestTrader(t)

	orders, err := trader.GetOpenOrders("BTCIDR")
	if err != nil {
		t.Fatalf("Failed to get open orders: %v", err)
	}

	t.Logf("📋 Found %d open orders:", len(orders))
	for i, order := range orders {
		t.Logf("  [%d] %s %s: price=%.0f orderID=%s",
			i+1, order.Symbol, order.Side, order.Price, order.OrderID)
	}
}

// TestIndodaxGetClosedPnL tests trade history retrieval
func TestIndodaxGetClosedPnL(t *testing.T) {
	trader := createIndodaxTestTrader(t)

	startTime := time.Now().Add(-7 * 24 * time.Hour)
	records, err := trader.GetClosedPnL(startTime, 10)
	if err != nil {
		t.Fatalf("Failed to get closed PnL: %v", err)
	}

	t.Logf("📋 Found %d trade records:", len(records))
	for i, record := range records {
		t.Logf("  [%d] %s %s: price=%.0f fee=%.4f time=%s",
			i+1, record.Symbol, record.Side, record.ExitPrice, record.Fee,
			record.ExitTime.Format("2006-01-02 15:04:05"))
	}
}

// TestIndodaxLoadPairs tests loading trading pairs
func TestIndodaxLoadPairs(t *testing.T) {
	trader := createIndodaxTestTrader(t)

	err := trader.loadPairs()
	if err != nil {
		t.Fatalf("Failed to load pairs: %v", err)
	}

	trader.pairCacheMutex.RLock()
	defer trader.pairCacheMutex.RUnlock()

	t.Logf("📊 Loaded %d pairs", len(trader.pairCache))

	// Check some known pairs
	knownPairs := []string{"btc_idr", "eth_idr"}
	for _, pairID := range knownPairs {
		if pair, ok := trader.pairCache[pairID]; ok {
			t.Logf("  %s: min_base=%v, min_traded=%v, precision=%d",
				pair.Description, pair.TradeMinBaseCurrency, pair.TradeMinTradedCurrency, pair.PriceRound)
		} else {
			t.Errorf("Expected pair %s not found", pairID)
		}
	}
}
