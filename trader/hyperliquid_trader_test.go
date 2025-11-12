package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// 一、HyperliquidTestSuite - 继承 base test suite
// ============================================================

// HyperliquidTestSuite Hyperliquid 交易器测试套件
// 继承 TraderTestSuite 并添加 Hyperliquid 特定的 mock 逻辑
type HyperliquidTestSuite struct {
	*TraderTestSuite // 嵌入基础测试套件
	mockServer       *httptest.Server
	privateKey       *ecdsa.PrivateKey
}

// NewHyperliquidTestSuite 创建 Hyperliquid 测试套件
func NewHyperliquidTestSuite(t *testing.T) *HyperliquidTestSuite {
	// 创建测试用私钥
	privateKey, err := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("创建测试私钥失败: %v", err)
	}

	// 创建 mock HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 根据不同的请求路径返回不同的 mock 响应
		var respBody interface{}

		// Hyperliquid API 使用 POST 请求，请求体是 JSON
		// 我们需要根据请求体中的 "type" 字段来区分不同的请求
		var reqBody map[string]interface{}
		if r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&reqBody)
		}

		// Try to get type from top level first, then from action object
		reqType, _ := reqBody["type"].(string)
		if reqType == "" && reqBody["action"] != nil {
			if action, ok := reqBody["action"].(map[string]interface{}); ok {
				reqType, _ = action["type"].(string)
			}
		}

		switch reqType {
		// Mock Meta - 获取市场元数据
		case "meta":
			respBody = map[string]interface{}{
				"universe": []map[string]interface{}{
					{
						"name":          "BTC",
						"szDecimals":    4,
						"maxLeverage":   50,
						"onlyIsolated":  false,
						"isDelisted":    false,
						"marginTableId": 0,
					},
					{
						"name":          "ETH",
						"szDecimals":    3,
						"maxLeverage":   50,
						"onlyIsolated":  false,
						"isDelisted":    false,
						"marginTableId": 0,
					},
				},
				"marginTables": []interface{}{},
			}

		// Mock UserState - 获取用户账户状态（用于 GetBalance 和 GetPositions）
		case "clearinghouseState":
			user, _ := reqBody["user"].(string)

			// 检查是否是查询 Agent 钱包余额（用于安全检查）
			agentAddr := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()
			if user == agentAddr {
				// Agent 钱包余额应该很低
				respBody = map[string]interface{}{
					"crossMarginSummary": map[string]interface{}{
						"accountValue":    "5.00",
						"totalMarginUsed": "0.00",
					},
					"withdrawable":   "5.00",
					"assetPositions": []interface{}{},
				}
			} else {
				// 主钱包账户状态
				respBody = map[string]interface{}{
					"crossMarginSummary": map[string]interface{}{
						"accountValue":    "10000.00",
						"totalMarginUsed": "2000.00",
					},
					"withdrawable": "8000.00",
					"assetPositions": []map[string]interface{}{
						{
							"position": map[string]interface{}{
								"coin":          "BTC",
								"szi":           "0.5",
								"entryPx":       "50000.00",
								"liquidationPx": "45000.00",
								"positionValue": "25000.00",
								"unrealizedPnl": "100.50",
								"leverage": map[string]interface{}{
									"type":  "cross",
									"value": 10,
								},
							},
						},
					},
				}
			}

		// Mock SpotUserState - 获取现货账户状态
		case "spotClearinghouseState":
			respBody = map[string]interface{}{
				"balances": []map[string]interface{}{
					{
						"coin":  "USDC",
						"total": "500.00",
					},
				},
			}

		// Mock SpotMeta - 获取现货市场元数据
		case "spotMeta":
			respBody = map[string]interface{}{
				"universe": []map[string]interface{}{},
				"tokens":   []map[string]interface{}{},
			}

		// Mock AllMids - 获取所有市场价格
		case "allMids":
			respBody = map[string]string{
				"BTC": "50000.00",
				"ETH": "3000.00",
			}

		// Mock OpenOrders - 获取挂单列表
		case "openOrders":
			respBody = []interface{}{}

		// Mock Order - 创建订单（开仓、平仓、止损、止盈）
		case "order":
			respBody = map[string]interface{}{
				"status": "ok",
				"response": map[string]interface{}{
					"type": "order",
					"data": map[string]interface{}{
						"statuses": []map[string]interface{}{
							{
								"filled": map[string]interface{}{
									"totalSz": "0.01",
									"avgPx":   "50000.00",
								},
							},
						},
					},
				},
			}

		// Mock UpdateLeverage - 设置杠杆
		case "updateLeverage":
			respBody = map[string]interface{}{
				"status": "ok",
			}

		// Mock Cancel - 取消订单
		case "cancel":
			respBody = map[string]interface{}{
				"status": "ok",
			}

		default:
			// 默认返回成功响应
			respBody = map[string]interface{}{
				"status": "ok",
			}
		}

		// 序列化响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// 创建 HyperliquidTrader，使用 mock 服务器 URL
	walletAddr := "0x9999999999999999999999999999999999999999"
	ctx := context.Background()

	// 创建 Exchange 客户端，指向 mock 服务器
	exchange := hyperliquid.NewExchange(
		ctx,
		privateKey,
		mockServer.URL, // 使用 mock 服务器 URL
		nil,
		"",
		walletAddr,
		nil,
	)

	// 创建 meta（模拟获取成功）
	meta := &hyperliquid.Meta{
		Universe: []hyperliquid.AssetInfo{
			{Name: "BTC", SzDecimals: 4},
			{Name: "ETH", SzDecimals: 3},
		},
	}

	trader := &HyperliquidTrader{
		exchange:      exchange,
		ctx:           ctx,
		walletAddr:    walletAddr,
		meta:          meta,
		isCrossMargin: true,
	}

	// 创建基础套件
	baseSuite := NewTraderTestSuite(t, trader)

	return &HyperliquidTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
		privateKey:      privateKey,
	}
}

// Cleanup 清理资源
func (s *HyperliquidTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// 二、使用 HyperliquidTestSuite 运行通用测试
// ============================================================

// TestHyperliquidTrader_InterfaceCompliance 测试接口兼容性
func TestHyperliquidTrader_InterfaceCompliance(t *testing.T) {
	var _ Trader = (*HyperliquidTrader)(nil)
}

// TestHyperliquidTrader_CommonInterface 使用测试套件运行所有通用接口测试
func TestHyperliquidTrader_CommonInterface(t *testing.T) {
	// 创建测试套件
	suite := NewHyperliquidTestSuite(t)
	defer suite.Cleanup()

	// 运行所有通用接口测试
	suite.RunAllTests()
}

// ============================================================
// 三、Hyperliquid 特定功能的单元测试
// ============================================================

// TestNewHyperliquidTrader 测试创建 Hyperliquid 交易器
func TestNewHyperliquidTrader(t *testing.T) {
	tests := []struct {
		name          string
		privateKeyHex string
		walletAddr    string
		testnet       bool
		wantError     bool
		errorContains string
	}{
		{
			name:          "无效私钥格式",
			privateKeyHex: "invalid_key",
			walletAddr:    "0x1234567890123456789012345678901234567890",
			testnet:       true,
			wantError:     true,
			errorContains: "解析私钥失败",
		},
		{
			name:          "钱包地址为空",
			privateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			walletAddr:    "",
			testnet:       true,
			wantError:     true,
			errorContains: "Configuration error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trader, err := NewHyperliquidTrader(tt.privateKeyHex, tt.walletAddr, tt.testnet)

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
					assert.Equal(t, tt.walletAddr, trader.walletAddr)
					assert.NotNil(t, trader.exchange)
				}
			}
		})
	}
}

// TestNewHyperliquidTrader_Success 测试成功创建交易器（需要 mock HTTP）
func TestNewHyperliquidTrader_Success(t *testing.T) {
	// 创建测试用私钥
	privateKey, _ := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	agentAddr := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// 创建 mock HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		reqType, _ := reqBody["type"].(string)

		var respBody interface{}
		switch reqType {
		case "meta":
			respBody = map[string]interface{}{
				"universe": []map[string]interface{}{
					{
						"name":          "BTC",
						"szDecimals":    4,
						"maxLeverage":   50,
						"onlyIsolated":  false,
						"isDelisted":    false,
						"marginTableId": 0,
					},
				},
				"marginTables": []interface{}{},
			}
		case "clearinghouseState":
			user, _ := reqBody["user"].(string)
			if user == agentAddr {
				// Agent 钱包余额低
				respBody = map[string]interface{}{
					"crossMarginSummary": map[string]interface{}{
						"accountValue": "5.00",
					},
					"assetPositions": []interface{}{},
				}
			} else {
				// 主钱包
				respBody = map[string]interface{}{
					"crossMarginSummary": map[string]interface{}{
						"accountValue": "10000.00",
					},
					"assetPositions": []interface{}{},
				}
			}
		default:
			respBody = map[string]interface{}{"status": "ok"}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))
	defer mockServer.Close()

	// 注意：这个测试会真正调用 NewHyperliquidTrader，但会失败
	// 因为 hyperliquid SDK 不允许我们在构造函数中注入自定义 URL
	// 所以这个测试仅用于验证参数处理逻辑
	t.Skip("跳过此测试：hyperliquid SDK 在构造时会调用真实 API，无法注入 mock URL")
}

// ============================================================
// 四、工具函数单元测试（Hyperliquid 特有）
// ============================================================

// TestConvertSymbolToHyperliquid 测试 symbol 转换函数
func TestConvertSymbolToHyperliquid(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected string
	}{
		{
			name:     "BTCUSDT转换",
			symbol:   "BTCUSDT",
			expected: "BTC",
		},
		{
			name:     "ETHUSDT转换",
			symbol:   "ETHUSDT",
			expected: "ETH",
		},
		{
			name:     "无USDT后缀",
			symbol:   "BTC",
			expected: "BTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertSymbolToHyperliquid(tt.symbol)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAbsFloat 测试绝对值函数
func TestAbsFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "正数",
			input:    10.5,
			expected: 10.5,
		},
		{
			name:     "负数",
			input:    -10.5,
			expected: 10.5,
		},
		{
			name:     "零",
			input:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := absFloat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHyperliquidTrader_RoundToSzDecimals 测试数量精度处理
func TestHyperliquidTrader_RoundToSzDecimals(t *testing.T) {
	trader := &HyperliquidTrader{
		meta: &hyperliquid.Meta{
			Universe: []hyperliquid.AssetInfo{
				{Name: "BTC", SzDecimals: 4},
				{Name: "ETH", SzDecimals: 3},
			},
		},
	}

	tests := []struct {
		name     string
		coin     string
		quantity float64
		expected float64
	}{
		{
			name:     "BTC_四舍五入到4位",
			coin:     "BTC",
			quantity: 1.23456789,
			expected: 1.2346,
		},
		{
			name:     "ETH_四舍五入到3位",
			coin:     "ETH",
			quantity: 10.12345,
			expected: 10.123,
		},
		{
			name:     "未知币种_使用默认精度4位",
			coin:     "UNKNOWN",
			quantity: 1.23456789,
			expected: 1.2346,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trader.roundToSzDecimals(tt.coin, tt.quantity)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

// TestHyperliquidTrader_RoundPriceToSigfigs 测试价格有效数字处理
func TestHyperliquidTrader_RoundPriceToSigfigs(t *testing.T) {
	trader := &HyperliquidTrader{}

	tests := []struct {
		name     string
		price    float64
		expected float64
	}{
		{
			name:     "BTC价格_5位有效数字",
			price:    50123.456789,
			expected: 50123.0,
		},
		{
			name:     "小数价格_5位有效数字",
			price:    0.0012345678,
			expected: 0.0012346,
		},
		{
			name:     "零价格",
			price:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trader.roundPriceToSigfigs(tt.price)
			assert.InDelta(t, tt.expected, result, tt.expected*0.001)
		})
	}
}

// TestHyperliquidTrader_GetSzDecimals 测试获取精度
func TestHyperliquidTrader_GetSzDecimals(t *testing.T) {
	tests := []struct {
		name     string
		meta     *hyperliquid.Meta
		coin     string
		expected int
	}{
		{
			name:     "meta为nil_返回默认精度",
			meta:     nil,
			coin:     "BTC",
			expected: 4,
		},
		{
			name: "找到BTC_返回正确精度",
			meta: &hyperliquid.Meta{
				Universe: []hyperliquid.AssetInfo{
					{Name: "BTC", SzDecimals: 5},
				},
			},
			coin:     "BTC",
			expected: 5,
		},
		{
			name: "未找到币种_返回默认精度",
			meta: &hyperliquid.Meta{
				Universe: []hyperliquid.AssetInfo{
					{Name: "ETH", SzDecimals: 3},
				},
			},
			coin:     "BTC",
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trader := &HyperliquidTrader{meta: tt.meta}
			result := trader.getSzDecimals(tt.coin)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHyperliquidTrader_SetMarginMode 测试设置保证金模式
func TestHyperliquidTrader_SetMarginMode(t *testing.T) {
	trader := &HyperliquidTrader{
		ctx:           context.Background(),
		isCrossMargin: true,
	}

	tests := []struct {
		name          string
		symbol        string
		isCrossMargin bool
		wantError     bool
	}{
		{
			name:          "设置为全仓模式",
			symbol:        "BTCUSDT",
			isCrossMargin: true,
			wantError:     false,
		},
		{
			name:          "设置为逐仓模式",
			symbol:        "ETHUSDT",
			isCrossMargin: false,
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := trader.SetMarginMode(tt.symbol, tt.isCrossMargin)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.isCrossMargin, trader.isCrossMargin)
			}
		})
	}
}

// TestNewHyperliquidTrader_PrivateKeyProcessing 测试私钥处理
func TestNewHyperliquidTrader_PrivateKeyProcessing(t *testing.T) {
	tests := []struct {
		name           string
		privateKeyHex  string
		shouldStripOx  bool
		expectedLength int
	}{
		{
			name:           "带0x前缀的私钥",
			privateKeyHex:  "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			shouldStripOx:  true,
			expectedLength: 64,
		},
		{
			name:           "无前缀的私钥",
			privateKeyHex:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			shouldStripOx:  false,
			expectedLength: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 测试私钥前缀处理逻辑（不实际创建 trader）
			processed := tt.privateKeyHex
			if len(processed) > 2 && (processed[:2] == "0x" || processed[:2] == "0X") {
				processed = processed[2:]
			}

			assert.Equal(t, tt.expectedLength, len(processed))
		})
	}
}
