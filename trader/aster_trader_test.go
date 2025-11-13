package trader

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// 一、AsterTraderTestSuite - 继承 base test suite
// ============================================================

// AsterTraderTestSuite Aster交易器测试套件
// 继承 TraderTestSuite 并添加 Aster 特定的 mock 逻辑
type AsterTraderTestSuite struct {
	*TraderTestSuite // 嵌入基础测试套件
	mockServer       *httptest.Server
}

// NewAsterTraderTestSuite 创建 Aster 测试套件
func NewAsterTraderTestSuite(t *testing.T) *AsterTraderTestSuite {
	// 创建 mock HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 根据不同的 URL 路径返回不同的 mock 响应
		path := r.URL.Path

		var respBody interface{}

		switch {
		// Mock GetBalance - /fapi/v3/balance (返回数组)
		case path == "/fapi/v3/balance":
			respBody = []map[string]interface{}{
				{
					"asset":              "USDT",
					"walletBalance":      "10000.00",
					"unrealizedProfit":   "100.50",
					"marginBalance":      "10100.50",
					"maintMargin":        "200.00",
					"initialMargin":      "2000.00",
					"maxWithdrawAmount":  "8000.00",
					"crossWalletBalance": "10000.00",
					"crossUnPnl":         "100.50",
					"availableBalance":   "8000.00",
				},
			}

		// Mock GetPositions - /fapi/v3/positionRisk
		case path == "/fapi/v3/positionRisk":
			respBody = []map[string]interface{}{
				{
					"symbol":           "BTCUSDT",
					"positionAmt":      "0.5",
					"entryPrice":       "50000.00",
					"markPrice":        "50500.00",
					"unRealizedProfit": "250.00",
					"liquidationPrice": "45000.00",
					"leverage":         "10",
					"positionSide":     "LONG",
				},
			}

		// Mock GetMarketPrice - /fapi/v3/ticker/price (返回单个对象)
		case path == "/fapi/v3/ticker/price":
			// 从查询参数获取symbol
			symbol := r.URL.Query().Get("symbol")
			if symbol == "" {
				symbol = "BTCUSDT"
			}
			// 根据symbol返回不同价格
			price := "50000.00"
			if symbol == "ETHUSDT" {
				price = "3000.00"
			} else if symbol == "INVALIDUSDT" {
				// 返回错误响应
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -1121,
					"msg":  "Invalid symbol",
				})
				return
			}
			respBody = map[string]interface{}{
				"symbol": symbol,
				"price":  price,
			}

		// Mock ExchangeInfo - /fapi/v3/exchangeInfo
		case path == "/fapi/v3/exchangeInfo":
			respBody = map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol":             "BTCUSDT",
						"pricePrecision":     1,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.1",
							},
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
						},
					},
					{
						"symbol":             "ETHUSDT",
						"pricePrecision":     2,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.01",
							},
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
						},
					},
				},
			}

		// Mock CreateOrder - /fapi/v1/order and /fapi/v3/order
		case (path == "/fapi/v1/order" || path == "/fapi/v3/order") && r.Method == "POST":
			// 从请求中解析参数以确定symbol
			bodyBytes, _ := io.ReadAll(r.Body)
			var orderParams map[string]interface{}
			json.Unmarshal(bodyBytes, &orderParams)

			symbol := "BTCUSDT"
			if s, ok := orderParams["symbol"].(string); ok {
				symbol = s
			}

			respBody = map[string]interface{}{
				"orderId": 123456,
				"symbol":  symbol,
				"status":  "FILLED",
				"side":    orderParams["side"],
				"type":    orderParams["type"],
			}

		// Mock CancelOrder - /fapi/v1/order (DELETE)
		case path == "/fapi/v1/order" && r.Method == "DELETE":
			respBody = map[string]interface{}{
				"orderId": 123456,
				"symbol":  "BTCUSDT",
				"status":  "CANCELED",
			}

		// Mock ListOpenOrders - /fapi/v1/openOrders and /fapi/v3/openOrders
		case path == "/fapi/v1/openOrders" || path == "/fapi/v3/openOrders":
			respBody = []map[string]interface{}{}

		// Mock SetLeverage - /fapi/v1/leverage
		case path == "/fapi/v1/leverage":
			respBody = map[string]interface{}{
				"leverage": 10,
				"symbol":   "BTCUSDT",
			}

		// Mock SetMarginMode - /fapi/v1/marginType
		case path == "/fapi/v1/marginType":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "success",
			}

		// Default: empty response
		default:
			respBody = map[string]interface{}{}
		}

		// 序列化响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// 生成一个测试用的私钥
	privateKey, _ := crypto.GenerateKey()

	// 创建 mock trader，使用 mock server 的 URL
	trader := &AsterTrader{
		ctx:             context.Background(),
		user:            "0x1234567890123456789012345678901234567890",
		signer:          "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		privateKey:      privateKey,
		client:          mockServer.Client(),
		baseURL:         mockServer.URL, // 使用 mock server 的 URL
		symbolPrecision: make(map[string]SymbolPrecision),
	}

	// 创建基础套件
	baseSuite := NewTraderTestSuite(t, trader)

	return &AsterTraderTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
	}
}

// Cleanup 清理资源
func (s *AsterTraderTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// 二、使用 AsterTraderTestSuite 运行通用测试
// ============================================================

// TestAsterTrader_InterfaceCompliance 测试接口兼容性
func TestAsterTrader_InterfaceCompliance(t *testing.T) {
	var _ Trader = (*AsterTrader)(nil)
}

// TestAsterTrader_CommonInterface 使用测试套件运行所有通用接口测试
func TestAsterTrader_CommonInterface(t *testing.T) {
	// 创建测试套件
	suite := NewAsterTraderTestSuite(t)
	defer suite.Cleanup()

	// 运行所有通用接口测试
	suite.RunAllTests()
}

// ============================================================
// 三、Aster 特定功能的单元测试
// ============================================================

// TestNewAsterTrader 测试创建 Aster 交易器
func TestNewAsterTrader(t *testing.T) {
	tests := []struct {
		name          string
		user          string
		signer        string
		privateKeyHex string
		wantError     bool
		errorContains string
	}{
		{
			name:          "成功创建",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantError:     false,
		},
		{
			name:          "无效私钥格式",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "invalid_key",
			wantError:     true,
			errorContains: "解析私钥失败",
		},
		{
			name:          "带0x前缀的私钥",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trader, err := NewAsterTrader(tt.user, tt.signer, tt.privateKeyHex)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, trader)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, trader)
				if trader != nil {
					assert.Equal(t, tt.user, trader.user)
					assert.Equal(t, tt.signer, trader.signer)
					assert.NotNil(t, trader.privateKey)
				}
			}
		})
	}
}
