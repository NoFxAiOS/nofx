package testutil

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"nofx/trader/types"
)

// TraderTestSuite Generic Trader interface test suite (base suite)
// Used for black-box testing any trader that implements the Trader interface
//
// Usage:
//  1. Create a concrete test suite struct, embedding TraderTestSuite
//  2. Implement SetupMocks() method to configure gomonkey mocks
//  3. Call RunAllTests() to run all generic tests
type TraderTestSuite struct {
	T       *testing.T
	Trader  types.Trader
	Patches *gomonkey.Patches
}

// NewTraderTestSuite Create new base test suite
func NewTraderTestSuite(t *testing.T, trader types.Trader) *TraderTestSuite {
	return &TraderTestSuite{
		T:       t,
		Trader:  trader,
		Patches: gomonkey.NewPatches(),
	}
}

// Cleanup Clean up mock patches
func (s *TraderTestSuite) Cleanup() {
	if s.Patches != nil {
		s.Patches.Reset()
	}
}

// RunAllTests Run all generic interface tests
// Note: Before calling this method, please set up required mocks via SetupMocks
func (s *TraderTestSuite) RunAllTests() {
	// Basic query methods
	s.T.Run("GetBalance", func(t *testing.T) { s.TestGetBalance() })
	s.T.Run("GetPositions", func(t *testing.T) { s.TestGetPositions() })
	s.T.Run("GetMarketPrice", func(t *testing.T) { s.TestGetMarketPrice() })

	// Configuration methods
	s.T.Run("SetLeverage", func(t *testing.T) { s.TestSetLeverage() })
	s.T.Run("SetMarginMode", func(t *testing.T) { s.TestSetMarginMode() })
	s.T.Run("FormatQuantity", func(t *testing.T) { s.TestFormatQuantity() })

	// Core trading methods
	s.T.Run("OpenLong", func(t *testing.T) { s.TestOpenLong() })
	s.T.Run("OpenShort", func(t *testing.T) { s.TestOpenShort() })
	s.T.Run("CloseLong", func(t *testing.T) { s.TestCloseLong() })
	s.T.Run("CloseShort", func(t *testing.T) { s.TestCloseShort() })

	// Stop-loss and take-profit
	s.T.Run("SetStopLoss", func(t *testing.T) { s.TestSetStopLoss() })
	s.T.Run("SetTakeProfit", func(t *testing.T) { s.TestSetTakeProfit() })

	// Order management
	s.T.Run("CancelAllOrders", func(t *testing.T) { s.TestCancelAllOrders() })
	s.T.Run("CancelStopOrders", func(t *testing.T) { s.TestCancelStopOrders() })
	s.T.Run("CancelStopLossOrders", func(t *testing.T) { s.TestCancelStopLossOrders() })
	s.T.Run("CancelTakeProfitOrders", func(t *testing.T) { s.TestCancelTakeProfitOrders() })
}

// TestGetBalance Test getting account balance
func (s *TraderTestSuite) TestGetBalance() {
	tests := []struct {
		name      string
		wantError bool
		validate  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "Successfully get balance",
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

// TestGetPositions Test getting positions
func (s *TraderTestSuite) TestGetPositions() {
	tests := []struct {
		name      string
		wantError bool
		validate  func(*testing.T, []map[string]interface{})
	}{
		{
			name:      "Successfully get position list",
			wantError: false,
			validate: func(t *testing.T, positions []map[string]interface{}) {
				assert.NotNil(t, positions)
				// Positions can be empty array
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

// TestGetMarketPrice Test getting market price
func (s *TraderTestSuite) TestGetMarketPrice() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
		validate  func(*testing.T, float64)
	}{
		{
			name:      "Successfully get BTC price",
			symbol:    "BTCUSDT",
			wantError: false,
			validate: func(t *testing.T, price float64) {
				assert.Greater(t, price, 0.0)
			},
		},
		{
			name:      "Invalid trading pair returns error",
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

// TestSetLeverage Test setting leverage
func (s *TraderTestSuite) TestSetLeverage() {
	tests := []struct {
		name      string
		symbol    string
		leverage  int
		wantError bool
	}{
		{
			name:      "Set 10x leverage",
			symbol:    "BTCUSDT",
			leverage:  10,
			wantError: false,
		},
		{
			name:      "Set 1x leverage",
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

// TestSetMarginMode Test setting margin mode
func (s *TraderTestSuite) TestSetMarginMode() {
	tests := []struct {
		name          string
		symbol        string
		isCrossMargin bool
		wantError     bool
	}{
		{
			name:          "Set cross margin mode",
			symbol:        "BTCUSDT",
			isCrossMargin: true,
			wantError:     false,
		},
		{
			name:          "Set isolated margin mode",
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

// TestFormatQuantity Test formatting quantity
func (s *TraderTestSuite) TestFormatQuantity() {
	tests := []struct {
		name      string
		symbol    string
		quantity  float64
		wantError bool
		validate  func(*testing.T, string)
	}{
		{
			name:      "Format BTC quantity",
			symbol:    "BTCUSDT",
			quantity:  1.23456789,
			wantError: false,
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
		{
			name:      "Format small quantity",
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

// TestCancelAllOrders Test canceling all orders
func (s *TraderTestSuite) TestCancelAllOrders() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
	}{
		{
			name:      "Cancel all BTC orders",
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
// Core trading method tests
// ============================================================

// TestOpenLong Test opening long position
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
			name:      "Successfully open long",
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
			name:      "Small quantity long",
			symbol:    "ETHUSDT",
			quantity:  0.004, // Increased to 0.004 to meet Binance Futures minimum order value of 10 USDT (0.004 * 3000 = 12 USDT)
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

// TestOpenShort Test opening short position
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
			name:      "Successfully open short",
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
			name:      "Small quantity short",
			symbol:    "ETHUSDT",
			quantity:  0.004, // Increased to 0.004 to meet Binance Futures minimum order value of 10 USDT (0.004 * 3000 = 12 USDT)
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

// TestCloseLong Test closing long position
func (s *TraderTestSuite) TestCloseLong() {
	tests := []struct {
		name      string
		symbol    string
		quantity  float64
		wantError bool
		validate  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "Close specified quantity",
			symbol:    "BTCUSDT",
			quantity:  0.01,
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
				assert.Contains(t, result, "symbol")
			},
		},
		{
			name:      "Close all with quantity=0 returns error when no position",
			symbol:    "ETHUSDT",
			quantity:  0,
			wantError: true, // When no position exists, quantity=0 should return error
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

// TestCloseShort Test closing short position
func (s *TraderTestSuite) TestCloseShort() {
	tests := []struct {
		name      string
		symbol    string
		quantity  float64
		wantError bool
		validate  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "Close specified quantity",
			symbol:    "BTCUSDT",
			quantity:  0.01,
			wantError: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.NotNil(t, result)
				assert.Contains(t, result, "symbol")
			},
		},
		{
			name:      "Close all with quantity=0 returns error when no position",
			symbol:    "ETHUSDT",
			quantity:  0,
			wantError: true, // When no position exists, quantity=0 should return error
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
// Stop-loss and take-profit tests
// ============================================================

// TestSetStopLoss Test setting stop-loss
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
			name:         "Long stop-loss",
			symbol:       "BTCUSDT",
			positionSide: "LONG",
			quantity:     0.01,
			stopPrice:    45000.0,
			wantError:    false,
		},
		{
			name:         "Short stop-loss",
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

// TestSetTakeProfit Test setting take-profit
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
			name:            "Long take-profit",
			symbol:          "BTCUSDT",
			positionSide:    "LONG",
			quantity:        0.01,
			takeProfitPrice: 55000.0,
			wantError:       false,
		},
		{
			name:            "Short take-profit",
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

// TestCancelStopOrders Test canceling stop-loss/take-profit orders
func (s *TraderTestSuite) TestCancelStopOrders() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
	}{
		{
			name:      "Cancel BTC stop orders",
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

// TestCancelStopLossOrders Test canceling stop-loss orders
func (s *TraderTestSuite) TestCancelStopLossOrders() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
	}{
		{
			name:      "Cancel BTC stop-loss orders",
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

// TestCancelTakeProfitOrders Test canceling take-profit orders
func (s *TraderTestSuite) TestCancelTakeProfitOrders() {
	tests := []struct {
		name      string
		symbol    string
		wantError bool
	}{
		{
			name:      "Cancel BTC take-profit orders",
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
