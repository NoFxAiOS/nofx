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
// LIGHTER V1 测试套件
// ============================================================

// TestLighterTrader_NewTrader 测试创建LIGHTER交易器
func TestLighterTrader_NewTrader(t *testing.T) {
	t.Run("无效私钥", func(t *testing.T) {
		trader, err := NewLighterTrader("invalid_key", "", true)
		assert.Error(t, err)
		assert.Nil(t, trader)
		t.Logf("✅ Invalid private key correctly rejected")
	})

	t.Run("有效私钥格式验证", func(t *testing.T) {
		// 只验证私钥解析，不调用真实 API
		testL1Key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		privateKey, err := crypto.HexToECDSA(testL1Key)
		assert.NoError(t, err)
		assert.NotNil(t, privateKey)

		walletAddr := crypto.PubkeyToAddress(*privateKey.Public().(*ecdsa.PublicKey)).Hex()
		assert.NotEmpty(t, walletAddr)
		t.Logf("✅ Valid private key format: wallet=%s", walletAddr)
	})
}

// createMockLighterServer 创建 mock LIGHTER API 服务器
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

// createMockLighterTrader 创建带 mock server 的 LIGHTER trader
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

// TestLighterTrader_GetBalance 测试获取余额
func TestLighterTrader_GetBalance(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	balance, err := trader.GetBalance()

	assert.NoError(t, err)
	assert.NotNil(t, balance)
	t.Logf("✅ GetBalance: %+v", balance)
}

// TestLighterTrader_GetPositions 测试获取持仓
func TestLighterTrader_GetPositions(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	positions, err := trader.GetPositions()

	assert.NoError(t, err)
	assert.NotNil(t, positions)
	t.Logf("✅ GetPositions: found %d positions", len(positions))
}

// TestLighterTrader_GetMarketPrice 测试获取市场价格
func TestLighterTrader_GetMarketPrice(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	price, err := trader.GetMarketPrice("BTC")

	assert.NoError(t, err)
	assert.Greater(t, price, 0.0)
	t.Logf("✅ GetMarketPrice(BTC): %.2f", price)
}

// TestLighterTrader_FormatQuantity 测试格式化数量
func TestLighterTrader_FormatQuantity(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	result, err := trader.FormatQuantity("BTC", 0.123456)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	t.Logf("✅ FormatQuantity: %s", result)
}

// TestLighterTrader_GetExchangeType 测试获取交易所类型
func TestLighterTrader_GetExchangeType(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	exchangeType := trader.GetExchangeType()

	assert.Equal(t, "lighter", exchangeType)
	t.Logf("✅ GetExchangeType: %s", exchangeType)
}

// TestLighterTrader_InvalidQuantity 测试无效数量验证
func TestLighterTrader_InvalidQuantity(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	// 测试零数量
	_, err := trader.OpenLong("BTC", 0, 10)
	assert.Error(t, err)

	// 测试负数量
	_, err = trader.OpenLong("BTC", -0.1, 10)
	assert.Error(t, err)

	t.Logf("✅ Invalid quantity validation working")
}

// TestLighterTrader_InvalidLeverage 测试无效杠杆验证
func TestLighterTrader_InvalidLeverage(t *testing.T) {
	mockServer := createMockLighterServer()
	defer mockServer.Close()

	trader := createMockLighterTrader(t, mockServer)

	// 测试零杠杆
	_, err := trader.OpenLong("BTC", 0.1, 0)
	assert.Error(t, err)

	// 测试负杠杆
	_, err = trader.OpenLong("BTC", 0.1, -10)
	assert.Error(t, err)

	t.Logf("✅ Invalid leverage validation working")
}

// TestLighterTrader_HelperFunctions 测试辅助函数
func TestLighterTrader_HelperFunctions(t *testing.T) {
	// 测试 SafeFloat64
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
