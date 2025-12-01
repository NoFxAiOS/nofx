package trader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 一、OKXTraderTestSuite - 继承 base test suite
// ============================================================

// OKXTraderTestSuite OKX交易器测试套件
// 继承 TraderTestSuite 并添加 OKX 特定的 mock 逻辑
type OKXTraderTestSuite struct {
	*TraderTestSuite // 嵌入基础测试套件
	mockServer       *httptest.Server
}

// NewOKXTraderTestSuite 创建 OKX 测试套件
func NewOKXTraderTestSuite(t *testing.T) *OKXTraderTestSuite {
	// 创建 mock HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		var respBody interface{}

		switch {
		// Mock GetBalance - /api/v5/account/balance
		case path == "/api/v5/account/balance":
			respBody = map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": []map[string]interface{}{
					{
						"totalEq": "10100.50",
						"details": []map[string]interface{}{
							{
								"ccy":        "USDT",
								"eq":         "10000.00",
								"availEq":    "8000.00",
								"frozenBal":  "2000.00",
								"upl":        "100.50",
								"cashBal":    "10000.00",
								"ordFrozen":  "0",
								"liab":       "0",
								"uTime":      "1609459200000",
								"crossLiab":  "0",
								"isoLiab":    "0",
								"mgnRatio":   "",
								"interest":   "0",
								"twap":       "0",
								"maxLoan":    "",
								"eqUsd":      "10000.00",
								"notionalLever": "",
								"stgyEq":     "0",
								"isoEq":      "0",
							},
						},
					},
				},
			}

		// Mock GetPositions - /api/v5/account/positions
		case path == "/api/v5/account/positions":
			respBody = map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": []map[string]interface{}{
					{
						"instId":     "BTC-USDT-SWAP",
						"pos":        "0.5",
						"posSide":    "long",
						"avgPx":      "50000.00",
						"markPx":     "50500.00",
						"upl":        "250.00",
						"uplRatio":   "0.01",
						"lever":      "10",
						"liqPx":      "45000.00",
						"notionalUsd": "25250.00",
						"instType":   "SWAP",
						"mgnMode":    "cross",
						"cTime":      "1609459200000",
						"uTime":      "1609459200000",
					},
				},
			}

		// Mock GetMarketPrice - /api/v5/market/ticker
		case path == "/api/v5/market/ticker":
			instId := r.URL.Query().Get("instId")
			if instId == "" {
				instId = "BTC-USDT-SWAP"
			}

			price := "50000.00"
			if instId == "ETH-USDT-SWAP" {
				price = "3000.00"
			} else if instId == "INVALID-USDT-SWAP" {
				// 返回错误
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": "51001",
					"msg":  "Instrument ID does not exist",
					"data": []interface{}{},
				})
				return
			}

			respBody = map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": []map[string]interface{}{
					{
						"instId":   instId,
						"last":     price,
						"lastSz":   "1",
						"askPx":    price,
						"askSz":    "10",
						"bidPx":    price,
						"bidSz":    "10",
						"open24h":  price,
						"high24h":  price,
						"low24h":   price,
						"volCcy24h": "1000000",
						"vol24h":   "20",
						"ts":       "1609459200000",
						"sodUtc0":  price,
						"sodUtc8":  price,
					},
				},
			}

		// Mock CreateOrder - /api/v5/trade/order (POST)
		case path == "/api/v5/trade/order" && r.Method == "POST":
			respBody = map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": []map[string]interface{}{
					{
						"ordId":  "123456789",
						"clOrdId": "test_order_123",
						"tag":    "",
						"sCode":  "0",
						"sMsg":   "",
					},
				},
			}

		// Mock SetLeverage - /api/v5/account/set-leverage (POST)
		case path == "/api/v5/account/set-leverage" && r.Method == "POST":
			respBody = map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": []map[string]interface{}{
					{
						"instId":  "BTC-USDT-SWAP",
						"lever":   "10",
						"mgnMode": "cross",
						"posSide": "long",
					},
				},
			}

		// Mock SetMarginMode - /api/v5/account/set-position-mode (POST)
		case path == "/api/v5/account/set-position-mode" && r.Method == "POST":
			respBody = map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": []map[string]interface{}{
					{
						"posMode": "net_mode",
					},
				},
			}

		// Default: empty success response
		default:
			respBody = map[string]interface{}{
				"code": "0",
				"msg":  "",
				"data": []interface{}{},
			}
		}

		// 序列化响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// 创建 OKXTrader 并设置为使用 mock 服务器
	trader := &OKXTrader{
		apiKey:        "test_api_key",
		secretKey:     "test_secret_key",
		passphrase:    "test_passphrase",
		baseURL:       mockServer.URL,
		httpClient:    mockServer.Client(),
		testnet:       false,
		cacheDuration: 0, // 禁用缓存以便测试
	}

	// 创建基础套件
	baseSuite := NewTraderTestSuite(t, trader)

	return &OKXTraderTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
	}
}

// Cleanup 清理资源
func (s *OKXTraderTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// 二、使用 OKXTraderTestSuite 运行通用测试
// ============================================================

// TestOKXTrader_InterfaceCompliance 测试接口兼容性
func TestOKXTrader_InterfaceCompliance(t *testing.T) {
	var _ Trader = (*OKXTrader)(nil)
}

// TestOKXTrader_CommonInterface 使用测试套件运行所有通用接口测试
func TestOKXTrader_CommonInterface(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	// 运行所有通用接口测试
	suite.RunAllTests()
}

// ============================================================
// 三、OKX 特定功能的单元测试
// ============================================================

// TestNewOKXTrader 测试创建 OKX 交易器
func TestNewOKXTrader(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		secretKey  string
		passphrase string
		testnet    bool
		wantNil    bool
	}{
		{
			name:       "成功创建（正式环境）",
			apiKey:     "test_api_key",
			secretKey:  "test_secret_key",
			passphrase: "test_passphrase",
			testnet:    false,
			wantNil:    false,
		},
		{
			name:       "成功创建（测试环境）",
			apiKey:     "test_api_key",
			secretKey:  "test_secret_key",
			passphrase: "test_passphrase",
			testnet:    true,
			wantNil:    false,
		},
		{
			name:       "空API Key仍可创建",
			apiKey:     "",
			secretKey:  "test_secret_key",
			passphrase: "test_passphrase",
			testnet:    false,
			wantNil:    false,
		},
		{
			name:       "空Secret Key仍可创建",
			apiKey:     "test_api_key",
			secretKey:  "",
			passphrase: "test_passphrase",
			testnet:    false,
			wantNil:    false,
		},
		{
			name:       "空Passphrase仍可创建",
			apiKey:     "test_api_key",
			secretKey:  "test_secret_key",
			passphrase: "",
			testnet:    false,
			wantNil:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trader := NewOKXTrader(tt.apiKey, tt.secretKey, tt.passphrase, tt.testnet)

			if tt.wantNil {
				assert.Nil(t, trader)
			} else {
				assert.NotNil(t, trader)
				assert.NotNil(t, trader.httpClient)
				assert.Equal(t, tt.apiKey, trader.apiKey)
				assert.Equal(t, tt.secretKey, trader.secretKey)
				assert.Equal(t, tt.passphrase, trader.passphrase)
				assert.Equal(t, tt.testnet, trader.testnet)

				// 检查 baseURL
				if tt.testnet {
					assert.Equal(t, "https://www.okx.com", trader.baseURL)
				} else {
					assert.Equal(t, "https://www.okx.com", trader.baseURL)
				}

				// 检查缓存时间
				assert.Equal(t, 15*time.Second, trader.cacheDuration)
			}
		})
	}
}

// TestOKXTrader_SymbolFormat 测试符号格式转换
func TestOKXTrader_SymbolFormat(t *testing.T) {
	trader := &OKXTrader{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "BTC USDT Swap",
			input:    "BTCUSDT",
			expected: "BTC-USDT-SWAP",
		},
		{
			name:     "ETH USDT Swap",
			input:    "ETHUSDT",
			expected: "ETH-USDT-SWAP",
		},
		{
			name:     "SOL USDT Swap",
			input:    "SOLUSDT",
			expected: "SOL-USDT-SWAP",
		},
		{
			name:     "小写输入",
			input:    "btcusdt",
			expected: "BTC-USDT-SWAP",
		},
		{
			name:     "混合大小写",
			input:    "BtcUsdT",
			expected: "BTC-USDT-SWAP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trader.formatSymbol(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOKXTrader_Sign 测试签名算法
func TestOKXTrader_Sign(t *testing.T) {
	trader := &OKXTrader{
		secretKey: "test_secret_key",
	}

	// 测试签名一致性
	timestamp := "2024-01-01T00:00:00.000Z"
	method := "GET"
	requestPath := "/api/v5/account/balance"
	body := ""

	// 多次签名应该产生相同结果
	sign1 := trader.sign(timestamp, method, requestPath, body)
	sign2 := trader.sign(timestamp, method, requestPath, body)
	assert.Equal(t, sign1, sign2, "相同输入应产生相同签名")

	// 不同输入应该产生不同签名
	sign3 := trader.sign("2024-01-01T00:00:01.000Z", method, requestPath, body)
	assert.NotEqual(t, sign1, sign3, "不同timestamp应产生不同签名")

	sign4 := trader.sign(timestamp, "POST", requestPath, body)
	assert.NotEqual(t, sign1, sign4, "不同method应产生不同签名")

	sign5 := trader.sign(timestamp, method, "/api/v5/account/positions", body)
	assert.NotEqual(t, sign1, sign5, "不同path应产生不同签名")

	sign6 := trader.sign(timestamp, method, requestPath, `{"instId":"BTC-USDT-SWAP"}`)
	assert.NotEqual(t, sign1, sign6, "不同body应产生不同签名")
}

// TestOKXTrader_GetBalance 测试获取余额
func TestOKXTrader_GetBalance(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 测试获取余额
	balance, err := trader.GetBalance()
	assert.NoError(t, err)
	assert.NotNil(t, balance)

	// 验证返回的标准化余额字段
	assert.Contains(t, balance, "totalWalletBalance")
	assert.Contains(t, balance, "availableBalance")
	assert.Contains(t, balance, "totalUnrealizedProfit")
	assert.Contains(t, balance, "balance")

	// 验证余额值
	totalBalance, ok := balance["totalWalletBalance"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 10000.00, totalBalance)

	availBalance, ok := balance["availableBalance"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 8000.00, availBalance)

	upl, ok := balance["totalUnrealizedProfit"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 100.50, upl)
}

// TestOKXTrader_GetPositions 测试获取持仓
func TestOKXTrader_GetPositions(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 测试获取持仓
	positions, err := trader.GetPositions()
	assert.NoError(t, err)
	assert.NotNil(t, positions)
	assert.GreaterOrEqual(t, len(positions), 1)

	// 验证标准化的持仓字段
	position := positions[0]
	assert.Contains(t, position, "symbol")
	assert.Contains(t, position, "side")
	assert.Contains(t, position, "entry_price")
	assert.Contains(t, position, "mark_price")
	assert.Contains(t, position, "quantity")
	assert.Contains(t, position, "leverage")
	assert.Contains(t, position, "unrealized_pnl")
	assert.Contains(t, position, "unrealized_pnl_pct")
	assert.Contains(t, position, "liquidation_price")
	assert.Contains(t, position, "margin_used")

	// 验证具体值（OKX 的数据被标准化）
	assert.Equal(t, "BTC", position["symbol"]) // BTC-USDT-SWAP → BTC
	assert.Equal(t, "long", position["side"])
	assert.Equal(t, 50000.0, position["entry_price"])
	assert.Equal(t, 50500.0, position["mark_price"])
	assert.Equal(t, 0.5, position["quantity"])
	assert.Equal(t, 10, position["leverage"])
	assert.Equal(t, 250.0, position["unrealized_pnl"])
	assert.Equal(t, 45000.0, position["liquidation_price"])
}

// TestOKXTrader_GetMarketPrice 测试获取市场价格
func TestOKXTrader_GetMarketPrice(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	tests := []struct {
		name      string
		symbol    string
		wantError bool
		wantPrice float64
	}{
		{
			name:      "获取BTC价格",
			symbol:    "BTCUSDT",
			wantError: false,
			wantPrice: 50000.00,
		},
		{
			name:      "获取ETH价格",
			symbol:    "ETHUSDT",
			wantError: false,
			wantPrice: 3000.00,
		},
		{
			name:      "无效符号",
			symbol:    "INVALID",
			wantError: true,
			wantPrice: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, err := trader.GetMarketPrice(tt.symbol)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantPrice, price)
			}
		})
	}
}

// TestOKXTrader_FormatQuantity 测试数量格式化
func TestOKXTrader_FormatQuantity(t *testing.T) {
	trader := &OKXTrader{}

	tests := []struct {
		name     string
		symbol   string
		quantity float64
		expected string
	}{
		{
			name:     "整数数量",
			symbol:   "BTCUSDT",
			quantity: 1.0,
			expected: "1.0000",
		},
		{
			name:     "小数数量",
			symbol:   "BTCUSDT",
			quantity: 0.5,
			expected: "0.5000",
		},
		{
			name:     "多位小数（四舍五入到4位）",
			symbol:   "BTCUSDT",
			quantity: 0.123456,
			expected: "0.1235",
		},
		{
			name:     "零数量",
			symbol:   "BTCUSDT",
			quantity: 0,
			expected: "0.0000",
		},
		{
			name:     "大数量",
			symbol:   "BTCUSDT",
			quantity: 100.123,
			expected: "100.1230",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trader.FormatQuantity(tt.symbol, tt.quantity)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOKXTrader_SetLeverage 测试设置杠杆
func TestOKXTrader_SetLeverage(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 测试设置杠杆
	err := trader.SetLeverage("BTCUSDT", 10)
	assert.NoError(t, err)
}

// TestOKXTrader_SetMarginMode 测试设置保证金模式
func TestOKXTrader_SetMarginMode(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 测试设置保证金模式（cross margin = true）
	err := trader.SetMarginMode("BTCUSDT", true)
	assert.NoError(t, err)

	// 测试设置保证金模式（isolated margin = false）
	err = trader.SetMarginMode("BTCUSDT", false)
	assert.NoError(t, err)
}

// TestOKXTrader_OpenLong 测试开多仓
func TestOKXTrader_OpenLong(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 测试开多仓（OKX 的 OpenLong 接受 leverage 参数）
	result, err := trader.OpenLong("BTCUSDT", 0.01, 10)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestOKXTrader_OpenShort 测试开空仓
func TestOKXTrader_OpenShort(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 测试开空仓（OKX 的 OpenShort 接受 leverage 参数）
	result, err := trader.OpenShort("BTCUSDT", 0.01, 10)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestOKXTrader_CloseLong 测试平多仓
func TestOKXTrader_CloseLong(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 测试平多仓（OKX 的 CloseLong 只接受 symbol 和 quantity）
	result, err := trader.CloseLong("BTCUSDT", 0.01)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestOKXTrader_CloseShort 测试平空仓
func TestOKXTrader_CloseShort(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 测试平空仓（OKX 的 CloseShort 只接受 symbol 和 quantity）
	result, err := trader.CloseShort("BTCUSDT", 0.01)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestOKXTrader_Cache 测试缓存机制
func TestOKXTrader_Cache(t *testing.T) {
	// 创建测试套件
	suite := NewOKXTraderTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*OKXTrader)

	// 启用缓存
	trader.cacheDuration = 5 * time.Second

	// 第一次调用 - 应该访问 API
	balance1, err := trader.GetBalance()
	assert.NoError(t, err)
	assert.NotNil(t, balance1)

	// 第二次调用 - 应该使用缓存
	balance2, err := trader.GetBalance()
	assert.NoError(t, err)
	assert.NotNil(t, balance2)
	assert.Equal(t, balance1, balance2)

	// 清空缓存
	trader.balanceCacheMutex.Lock()
	trader.cachedBalance = nil
	trader.balanceCacheTime = time.Time{}
	trader.balanceCacheMutex.Unlock()

	// 第三次调用 - 应该重新访问 API
	balance3, err := trader.GetBalance()
	assert.NoError(t, err)
	assert.NotNil(t, balance3)
}

// TestOKXTrader_ErrorHandling 测试错误处理
func TestOKXTrader_ErrorHandling(t *testing.T) {
	// 创建错误响应的 mock 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "50000",
			"msg":  "Internal server error",
			"data": []interface{}{},
		})
	}))
	defer mockServer.Close()

	trader := &OKXTrader{
		apiKey:     "test_api_key",
		secretKey:  "test_secret_key",
		passphrase: "test_passphrase",
		baseURL:    mockServer.URL,
		httpClient: mockServer.Client(),
		testnet:    false,
	}

	// 测试各种操作应该返回错误
	_, err := trader.GetBalance()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "50000")

	_, err = trader.GetPositions()
	assert.Error(t, err)

	_, err = trader.GetMarketPrice("BTCUSDT")
	assert.Error(t, err)

	_, err = trader.OpenLong("BTCUSDT", 0.01, 10)
	assert.Error(t, err)

	err = trader.SetLeverage("BTCUSDT", 10)
	assert.Error(t, err)
}

// TestOKXTrader_HTTPRequestError 测试 HTTP 请求错误
func TestOKXTrader_HTTPRequestError(t *testing.T) {
	// 使用无效的 baseURL
	trader := &OKXTrader{
		apiKey:     "test_api_key",
		secretKey:  "test_secret_key",
		passphrase: "test_passphrase",
		baseURL:    "http://invalid-url-that-does-not-exist-12345.com",
		httpClient: &http.Client{Timeout: 1 * time.Second},
		testnet:    false,
	}

	// 测试各种操作应该返回网络错误
	_, err := trader.GetBalance()
	assert.Error(t, err)

	_, err = trader.GetPositions()
	assert.Error(t, err)

	_, err = trader.GetMarketPrice("BTCUSDT")
	assert.Error(t, err)
}

// TestOKXTrader_InvalidJSON 测试无效 JSON 响应
func TestOKXTrader_InvalidJSON(t *testing.T) {
	// 创建返回无效 JSON 的 mock 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "invalid json{{{")
	}))
	defer mockServer.Close()

	trader := &OKXTrader{
		apiKey:     "test_api_key",
		secretKey:  "test_secret_key",
		passphrase: "test_passphrase",
		baseURL:    mockServer.URL,
		httpClient: mockServer.Client(),
		testnet:    false,
	}

	// 测试应该返回 JSON 解析错误
	_, err := trader.GetBalance()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析")
}
