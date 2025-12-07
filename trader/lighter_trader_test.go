package trader

import (
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// LIGHTER V1 Test Suite
// ============================================================

// TestLighterTrader_NewTrader Test creating LIGHTER trader
func TestLighterTrader_NewTrader(t *testing.T) {
	t.Run("Invalid private key", func(t *testing.T) {
		trader, err := NewLighterTrader("invalid_key", "", true)
		assert.Error(t, err)
		assert.Nil(t, trader)
		t.Logf("✅ Invalid private key correctly rejected")
	})

	t.Run("Valid private key format verification", func(t *testing.T) {
		// Only verify private key parsing, don't call real API
		testL1Key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		privateKey, err := crypto.HexToECDSA(testL1Key)
		assert.NoError(t, err)
		assert.NotNil(t, privateKey)

		walletAddr := crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey)).Hex()
		assert.NotEmpty(t, walletAddr)
		t.Logf("✅ Valid private key format: wallet=%s", walletAddr)
	})
}

// createMockLighterServer Create mock LIGHTER API server
func createMockLighterServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		var respBody interface{}

		switch path {
		// Mock GetBalance
		case "/api/v1/account":
			respBody = map[string]interface{}{
				"totalBalance":      "10000.00",
				"availableBalance":  "8000.00",
				"marginUsed":        "2000.00",
				"unrealizedPnl":     "100.50",
			}

		// Mock GetPositions
		case "/api/v1/positions":
			respBody = []map[string]interface{}{
				{
					"symbol":          "BTC_USDT",
					"side":            "long",
					"positionSize":    "0.5",
					"entryPrice":      "50000.00",
					"markPrice":       "50500.00",
					"unrealizedPnl":   "250.00",
				},
			}

		// Mock GetMarketPrice
		case "/api/v1/ticker/price":
			symbol := r.URL.Query().Get("symbol")
			respBody = map[string]interface{}{
				"symbol":     symbol,
				"last_price": "50000.00",
			}

		// Mock OrderBooks (for market index)
		case "/api/v1/orderBooks":
			respBody = map[string]interface{}{
				"data": []map[string]interface{}{
					{"symbol": "BTC_USDT", "marketIndex": 0},
					{"symbol": "ETH_USDT", "marketIndex": 1},
				},
			}

		// Mock SendTx (submit/cancel orders)
		case "/api/v1/sendTx":
			respBody = map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"orderId": "12345",
					"status":  "success",
				},
			}

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Unknown endpoint",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))
}

// createMockLighterTrader Create LIGHTER trader with mock server
func createMockLighterTrader(t *testing.T, mockServer *httptest.Server) *LighterTrader {
	testL1Key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	privateKey, err := crypto.HexToECDSA(testL1Key)
	assert.NoError(t, err)

	trader := &LighterTrader{
		privateKey:      privateKey,
		walletAddr:      crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey)).Hex(),
		client:          mockServer.Client(),
		baseURL:         mockServer.URL,
		testnet:         true,
		authToken:       "mock_auth_token",
		symbolPrecision: make(map[string]SymbolPrecision),
	}

	return trader
}

// TestLighterTrader_GetBalance Test getting balance
func TestLighterTrader_GetBalance(t *testing.T) {
	t.Skip("Skipping Lighter tests until mock server endpoints are completed")
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	balance, err := trader.GetBalance()

	assert.NoError(t, err)
	assert.NotNil(t, balance)
	t.Logf("✅ GetBalance: %+v", balance)
}

// TestLighterTrader_GetPositions Test getting positions
func TestLighterTrader_GetPositions(t *testing.T) {
	t.Skip("Skipping Lighter tests until mock server endpoints are completed")
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	positions, err := trader.GetPositions()

	assert.NoError(t, err)
	assert.NotNil(t, positions)
	t.Logf("✅ GetPositions: found %d positions", len(positions))
}

// TestLighterTrader_GetMarketPrice Test getting market price
func TestLighterTrader_GetMarketPrice(t *testing.T) {
	t.Skip("Skipping Lighter tests until mock server endpoints are completed")
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	price, err := trader.GetMarketPrice("BTC")

	assert.NoError(t, err)
	assert.Greater(t, price, 0.0)
	t.Logf("✅ GetMarketPrice(BTC): %.2f", price)
}

// TestLighterTrader_FormatQuantity Test formatting quantity
func TestLighterTrader_FormatQuantity(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	result, err := trader.FormatQuantity("BTC", 0.123456)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	t.Logf("✅ FormatQuantity: %s", result)
}

// TestLighterTrader_GetExchangeType Test getting exchange type
func TestLighterTrader_GetExchangeType(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	exchangeType := trader.GetExchangeType()

	assert.Equal(t, "lighter", exchangeType)
	t.Logf("✅ GetExchangeType: %s", exchangeType)
}

// TestLighterTrader_InvalidQuantity Test invalid quantity validation
func TestLighterTrader_InvalidQuantity(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	// Test zero quantity
	_, err := trader.OpenLong("BTC", 0, 10)
	assert.Error(t, err)

	// Test negative quantity
	_, err = trader.OpenLong("BTC", -0.1, 10)
	assert.Error(t, err)

	t.Logf("✅ Invalid quantity validation working")
}

// TestLighterTrader_InvalidLeverage Test invalid leverage validation
func TestLighterTrader_InvalidLeverage(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	// Test zero leverage
	_, err := trader.OpenLong("BTC", 0.1, 0)
	assert.Error(t, err)

	// Test negative leverage
	_, err = trader.OpenLong("BTC", 0.1, -10)
	assert.Error(t, err)

	t.Logf("✅ Invalid leverage validation working")
}

// TestLighterTrader_HelperFunctions Test helper functions
func TestLighterTrader_HelperFunctions(t *testing.T) {
	// Test SafeFloat64
	data := map[string]interface{}{
		"float_val":  123.45,
		"string_val": "678.90",
		"int_val":    42,
	}

	val, err := SafeFloat64(data, "float_val")
	assert.NoError(t, err)
	assert.Equal(t, 123.45, val)

	val, err = SafeFloat64(data, "string_val")
	assert.NoError(t, err)
	assert.Equal(t, 678.90, val)

	val, err = SafeFloat64(data, "int_val")
	assert.NoError(t, err)
	assert.Equal(t, 42.0, val)

	_, err = SafeFloat64(data, "nonexistent")
	assert.Error(t, err)

	t.Logf("✅ Helper functions working correctly")
}
