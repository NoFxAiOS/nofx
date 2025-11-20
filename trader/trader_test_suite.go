package trader

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
)

// TraderTestSuite 通用的 Trader 接口测试套件（基础套件）
// 用于黑盒测试任何实现了 Trader 接口的交易器
//
// 使用方式：
//  1. 创建具体的测试套件结构体，嵌入 TraderTestSuite
//  2. 实现 SetupMocks() 方法来配置 gomonkey mock
//  3. 调用 RunAllTests() 运行所有通用测试
type TraderTestSuite struct {
	T       *testing.T
	Trader  Trader
	Patches *gomonkey.Patches
}

// NewTraderTestSuite 创建新的基础测试套件
func NewTraderTestSuite(t *testing.T, trader Trader) *TraderTestSuite {
	return &TraderTestSuite{
		T:       t,
		Trader:  trader,
		Patches: gomonkey.NewPatches(),
	}
}

// Cleanup 清理 mock patches
func (s *TraderTestSuite) Cleanup() {
	if s.Patches != nil {
		s.Patches.Reset()
	}
}

// RunAllTests 运行所有通用接口测试
// 注意：调用此方法前，请先通过 SetupMocks 设置好所需的 mock
func (s *TraderTestSuite) RunAllTests() {
	// 基础查询方法
	s.T.Run("GetBalance", func(t *testing.T) { s.TestGetBalance() })
	s.T.Run("GetPositions", func(t *testing.T) { s.TestGetPositions() })
	s.T.Run("GetMarketPrice", func(t *testing.T) { s.TestGetMarketPrice() })

	// 配置方法
	s.T.Run("SetLeverage", func(t *testing.T) { s.TestSetLeverage() })
	s.T.Run("SetMarginMode", func(t *testing.T) { s.TestSetMarginMode() })
	s.T.Run("FormatQuantity", func(t *testing.T) { s.TestFormatQuantity() })

	// 核心交易方法
	s.T.Run("OpenLong", func(t *testing.T) { s.TestOpenLong() })
	s.T.Run("OpenShort", func(t *testing.T) { s.TestOpenShort() })
	s.T.Run("CloseLong", func(t *testing.T) { s.TestCloseLong() })
	s.T.Run("CloseShort", func(t *testing.T) { s.TestCloseShort() })

	// 止损止盈
	s.T.Run("SetStopLoss", func(t *testing.T) { s.TestSetStopLoss() })
	s.T.Run("SetTakeProfit", func(t *testing.T) { s.TestSetTakeProfit() })

	// 订单管理
	s.T.Run("CancelAllOrders", func(t *testing.T) { s.TestCancelAllOrders() })
	s.T.Run("CancelStopOrders", func(t *testing.T) { s.TestCancelStopOrders() })
	s.T.Run("CancelStopLossOrders", func(t *testing.T) { s.TestCancelStopLossOrders() })
	s.T.Run("CancelTakeProfitOrders", func(t *testing.T) { s.TestCancelTakeProfitOrders() })
}

// TestGetBalance 测试获取账户余额
func (s *TraderTestSuite) TestGetBalance() {
	tests := []struct {
		name      string
		wantError bool
		validate  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "成功获取余额",
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
				assert.Contains(t, result, "totalWalletBalance")
				assert.Contains(t, result, "availableBalance")
			},
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			result, err := s.Trader.GetBalance()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestGetPositions 测试获取持仓
func (s *TraderTestSuite) TestGetPositions() {
	tests := []struct {
		name      string
		wantError bool
		validate  func(*testing.T, []map[string]interface{})
	}{
		{
			name:      "成功获取持仓列表",
			wantError: false,
			validate: func(t *testing.T, positions []map[string]interface{}) {
				assert.NotNil(t, positions)
				// 持仓可以为空数组
				for _, pos := range positions {
					assert.Contains(t, pos, "symbol")
					assert.Contains(t, pos, "side")
					assert.Contains(t, pos, "positionAmt")
				}
			},
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			result, err := s.Trader.GetPositions()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestGetMarketPrice 测试获取市场价格
func (s *TraderTestSuite) TestGetMarketPrice() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
		validate  func(*testing.T, float64)
	}{
		{
			name:      "成功获取BTC价格",
			symbol:    "BTCUSDT",
			wantError: false,
			validate: func(t *testing.T, price float64) {
				assert.Greater(t, price, 0.0)
			},
		},
		{
			name:      "无效交易对返回错误",
			symbol:    "INVALIDUSDT",
			wantError: true,
			validate:  nil,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			price, err := s.Trader.GetMarketPrice(tt.symbol)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, price)
				}
			}
		})
	}
}

// TestSetLeverage 测试设置杠杆
func (s *TraderTestSuite) TestSetLeverage() {
	tests := []struct {
		name      string
		symbol    string
		leverage  int
		wantError bool
	}{
		{
			name:      "设置10倍杠杆",
			symbol:    "BTCUSDT",
			leverage:  10,
			wantError: false,
		},
		{
			name:      "设置1倍杠杆",
			symbol:    "ETHUSDT",
			leverage:  1,
			wantError: false,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			err := s.Trader.SetLeverage(tt.symbol, tt.leverage)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSetMarginMode 测试设置仓位模式
func (s *TraderTestSuite) TestSetMarginMode() {
	tests := []struct {
		name          string
		symbol        string
		isCrossMargin bool
		wantError     bool
	}{
		{
			name:          "设置全仓模式",
			symbol:        "BTCUSDT",
			isCrossMargin: true,
			wantError:     false,
		},
		{
			name:          "设置逐仓模式",
			symbol:        "ETHUSDT",
			isCrossMargin: false,
			wantError:     false,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			err := s.Trader.SetMarginMode(tt.symbol, tt.isCrossMargin)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFormatQuantity 测试数量格式化
func (s *TraderTestSuite) TestFormatQuantity() {
	tests := []struct {
		name      string
		symbol    string
		quantity  float64
		wantError bool
		validate  func(*testing.T, string)
	}{
		{
			name:      "格式化BTC数量",
			symbol:    "BTCUSDT",
			quantity:  1.23456789,
			wantError: false,
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
		{
			name:      "格式化小数量",
			symbol:    "ETHUSDT",
			quantity:  0.001,
			wantError: false,
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			result, err := s.Trader.FormatQuantity(tt.symbol, tt.quantity)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestCancelAllOrders 测试取消所有订单
func (s *TraderTestSuite) TestCancelAllOrders() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
	}{
		{
			name:      "取消BTC所有订单",
			symbol:    "BTCUSDT",
			wantError: false,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			err := s.Trader.CancelAllOrders(tt.symbol)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============================================================
// 核心交易方法测试
// ============================================================

// TestOpenLong 测试开多仓
func (s *TraderTestSuite) TestOpenLong() {
	tests := []struct {
		name      string
		symbol    string
		quantity  float64
		leverage  int
		wantError bool
		validate  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "成功开多仓",
			symbol:    "BTCUSDT",
			quantity:  0.01,
			leverage:  10,
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
				assert.Contains(t, result, "symbol")
				assert.Equal(t, "BTCUSDT", result["symbol"])
			},
		},
		{
			name:      "小数量开仓",
			symbol:    "ETHUSDT",
			quantity:  0.004, // 增加到 0.004 以满足 Binance Futures 的 10 USDT 最小订单金额要求 (0.004 * 3000 = 12 USDT)
			leverage:  5,
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
			},
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			result, err := s.Trader.OpenLong(tt.symbol, tt.quantity, tt.leverage)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestOpenShort 测试开空仓
func (s *TraderTestSuite) TestOpenShort() {
	tests := []struct {
		name      string
		symbol    string
		quantity  float64
		leverage  int
		wantError bool
		validate  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "成功开空仓",
			symbol:    "BTCUSDT",
			quantity:  0.01,
			leverage:  10,
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
				assert.Contains(t, result, "symbol")
				assert.Equal(t, "BTCUSDT", result["symbol"])
			},
		},
		{
			name:      "小数量开空仓",
			symbol:    "ETHUSDT",
			quantity:  0.004, // 增加到 0.004 以满足 Binance Futures 的 10 USDT 最小订单金额要求 (0.004 * 3000 = 12 USDT)
			leverage:  5,
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
			},
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			result, err := s.Trader.OpenShort(tt.symbol, tt.quantity, tt.leverage)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestCloseLong 测试平多仓
func (s *TraderTestSuite) TestCloseLong() {
	tests := []struct {
		name      string
		symbol    string
		quantity  float64
		wantError bool
		validate  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "平指定数量",
			symbol:    "BTCUSDT",
			quantity:  0.01,
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
				assert.Contains(t, result, "symbol")
			},
		},
		{
			name:      "全部平仓_quantity为0_无持仓返回错误",
			symbol:    "ETHUSDT",
			quantity:  0,
			wantError: true, // 当没有持仓时，quantity=0 应该返回错误
			validate:  nil,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			result, err := s.Trader.CloseLong(tt.symbol, tt.quantity)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestCloseShort 测试平空仓
func (s *TraderTestSuite) TestCloseShort() {
	tests := []struct {
		name      string
		symbol    string
		quantity  float64
		wantError bool
		validate  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "平指定数量",
			symbol:    "BTCUSDT",
			quantity:  0.01,
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
				assert.Contains(t, result, "symbol")
			},
		},
		{
			name:      "全部平仓_quantity为0_无持仓返回错误",
			symbol:    "ETHUSDT",
			quantity:  0,
			wantError: true, // 当没有持仓时，quantity=0 应该返回错误
			validate:  nil,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			result, err := s.Trader.CloseShort(tt.symbol, tt.quantity)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// ============================================================
// 止损止盈测试
// ============================================================

// TestSetStopLoss 测试设置止损
func (s *TraderTestSuite) TestSetStopLoss() {
	tests := []struct {
		name         string
		symbol       string
		positionSide string
		quantity     float64
		stopPrice    float64
		wantError    bool
	}{
		{
			name:         "多头止损",
			symbol:       "BTCUSDT",
			positionSide: "LONG",
			quantity:     0.01,
			stopPrice:    45000.0,
			wantError:    false,
		},
		{
			name:         "空头止损",
			symbol:       "ETHUSDT",
			positionSide: "SHORT",
			quantity:     0.1,
			stopPrice:    3200.0,
			wantError:    false,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			err := s.Trader.SetStopLoss(tt.symbol, tt.positionSide, tt.quantity, tt.stopPrice)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSetTakeProfit 测试设置止盈
func (s *TraderTestSuite) TestSetTakeProfit() {
	tests := []struct {
		name            string
		symbol          string
		positionSide    string
		quantity        float64
		takeProfitPrice float64
		wantError       bool
	}{
		{
			name:            "多头止盈",
			symbol:          "BTCUSDT",
			positionSide:    "LONG",
			quantity:        0.01,
			takeProfitPrice: 55000.0,
			wantError:       false,
		},
		{
			name:            "空头止盈",
			symbol:          "ETHUSDT",
			positionSide:    "SHORT",
			quantity:        0.1,
			takeProfitPrice: 2800.0,
			wantError:       false,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			err := s.Trader.SetTakeProfit(tt.symbol, tt.positionSide, tt.quantity, tt.takeProfitPrice)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCancelStopOrders 测试取消止盈止损单
func (s *TraderTestSuite) TestCancelStopOrders() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
	}{
		{
			name:      "取消BTC止盈止损单",
			symbol:    "BTCUSDT",
			wantError: false,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			err := s.Trader.CancelStopOrders(tt.symbol)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCancelStopLossOrders 测试取消止损单
func (s *TraderTestSuite) TestCancelStopLossOrders() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
	}{
		{
			name:      "取消BTC止损单",
			symbol:    "BTCUSDT",
			wantError: false,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			err := s.Trader.CancelStopLossOrders(tt.symbol)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCancelTakeProfitOrders 测试取消止盈单
func (s *TraderTestSuite) TestCancelTakeProfitOrders() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
	}{
		{
			name:      "取消BTC止盈单",
			symbol:    "BTCUSDT",
			wantError: false,
		},
	}

	for _, tt := range tests {
		s.T.Run(tt.name, func(t *testing.T) {
			err := s.Trader.CancelTakeProfitOrders(tt.symbol)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
