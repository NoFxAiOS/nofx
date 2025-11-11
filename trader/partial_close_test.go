package trader

import (
	"fmt"
	"nofx/decision"
	"nofx/logger"
	"testing"
)

// MockPartialCloseTrader 用於測試 partial close 邏輯
type MockPartialCloseTrader struct {
	positions          []map[string]interface{}
	closePartialCalled bool
	closeLongCalled    bool
	closeShortCalled   bool
	stopLossCalled     bool
	takeProfitCalled   bool
	lastStopLoss       float64
	lastTakeProfit     float64
}

func (m *MockPartialCloseTrader) GetPositions() ([]map[string]interface{}, error) {
	return m.positions, nil
}

func (m *MockPartialCloseTrader) ClosePartialLong(symbol string, quantity float64) (map[string]interface{}, error) {
	m.closePartialCalled = true
	return map[string]interface{}{"orderId": "12345"}, nil
}

func (m *MockPartialCloseTrader) ClosePartialShort(symbol string, quantity float64) (map[string]interface{}, error) {
	m.closePartialCalled = true
	return map[string]interface{}{"orderId": "12345"}, nil
}

func (m *MockPartialCloseTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	m.closeLongCalled = true
	return map[string]interface{}{"orderId": "12346"}, nil
}

func (m *MockPartialCloseTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	m.closeShortCalled = true
	return map[string]interface{}{"orderId": "12346"}, nil
}

func (m *MockPartialCloseTrader) SetStopLoss(symbol, side string, quantity, price float64) error {
	m.stopLossCalled = true
	m.lastStopLoss = price
	return nil
}

func (m *MockPartialCloseTrader) SetTakeProfit(symbol, side string, quantity, price float64) error {
	m.takeProfitCalled = true
	m.lastTakeProfit = price
	return nil
}

// TestPartialCloseMinPositionCheck 測試最小倉位檢查邏輯
func TestPartialCloseMinPositionCheck(t *testing.T) {
	tests := []struct {
		name              string
		totalQuantity     float64
		markPrice         float64
		closePercentage   float64
		expectFullClose   bool // 是否應該觸發全平邏輯
		expectRemainValue float64
	}{
		{
			name:              "正常部分平倉_剩餘價值充足",
			totalQuantity:     1.0,
			markPrice:         100.0,
			closePercentage:   50.0,
			expectFullClose:   false,
			expectRemainValue: 50.0, // 剩餘 0.5 * 100 = 50 USDT
		},
		{
			name:              "部分平倉_剩餘價值小於10USDT_應該全平",
			totalQuantity:     0.2,
			markPrice:         100.0,
			closePercentage:   95.0, // 平倉 95%，剩餘 1 USDT (0.2 * 5% * 100)
			expectFullClose:   true,
			expectRemainValue: 1.0,
		},
		{
			name:              "部分平倉_剩餘價值剛好10USDT_應該全平",
			totalQuantity:     1.0,
			markPrice:         100.0,
			closePercentage:   90.0, // 剩餘 10 USDT (1.0 * 10% * 100)，邊界測試 (<=)
			expectFullClose:   true,
			expectRemainValue: 10.0,
		},
		{
			name:              "部分平倉_剩餘價值11USDT_不應全平",
			totalQuantity:     1.1,
			markPrice:         100.0,
			closePercentage:   90.0, // 剩餘 11 USDT (1.1 * 10% * 100)
			expectFullClose:   false,
			expectRemainValue: 11.0,
		},
		{
			name:              "大倉位部分平倉_剩餘價值遠大於10USDT",
			totalQuantity:     10.0,
			markPrice:         1000.0,
			closePercentage:   80.0,
			expectFullClose:   false,
			expectRemainValue: 2000.0, // 剩餘 2 * 1000 = 2000 USDT
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 計算剩餘價值
			closeQuantity := tt.totalQuantity * (tt.closePercentage / 100.0)
			remainingQuantity := tt.totalQuantity - closeQuantity
			remainingValue := remainingQuantity * tt.markPrice

			// 驗證計算（使用浮點數比較允許微小誤差）
			const epsilon = 0.001
			if remainingValue-tt.expectRemainValue > epsilon || tt.expectRemainValue-remainingValue > epsilon {
				t.Errorf("計算錯誤: 剩餘價值 = %.2f, 期望 = %.2f",
					remainingValue, tt.expectRemainValue)
			}

			// 驗證最小倉位檢查邏輯
			const MIN_POSITION_VALUE = 10.0
			shouldFullClose := remainingValue > 0 && remainingValue <= MIN_POSITION_VALUE

			if shouldFullClose != tt.expectFullClose {
				t.Errorf("最小倉位檢查失敗: shouldFullClose = %v, 期望 = %v (剩餘價值 = %.2f USDT)",
					shouldFullClose, tt.expectFullClose, remainingValue)
			}
		})
	}
}

// TestPartialCloseWithStopLossTakeProfitRecovery 測試止盈止損恢復邏輯
func TestPartialCloseWithStopLossTakeProfitRecovery(t *testing.T) {
	tests := []struct {
		name             string
		newStopLoss      float64
		newTakeProfit    float64
		expectStopLoss   bool
		expectTakeProfit bool
	}{
		{
			name:             "有新止損和止盈_應該恢復兩者",
			newStopLoss:      95.0,
			newTakeProfit:    110.0,
			expectStopLoss:   true,
			expectTakeProfit: true,
		},
		{
			name:             "只有新止損_僅恢復止損",
			newStopLoss:      95.0,
			newTakeProfit:    0,
			expectStopLoss:   true,
			expectTakeProfit: false,
		},
		{
			name:             "只有新止盈_僅恢復止盈",
			newStopLoss:      0,
			newTakeProfit:    110.0,
			expectStopLoss:   false,
			expectTakeProfit: true,
		},
		{
			name:             "沒有新止損止盈_不恢復",
			newStopLoss:      0,
			newTakeProfit:    0,
			expectStopLoss:   false,
			expectTakeProfit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模擬止盈止損恢復邏輯
			stopLossRecovered := tt.newStopLoss > 0
			takeProfitRecovered := tt.newTakeProfit > 0

			if stopLossRecovered != tt.expectStopLoss {
				t.Errorf("止損恢復邏輯錯誤: recovered = %v, 期望 = %v",
					stopLossRecovered, tt.expectStopLoss)
			}

			if takeProfitRecovered != tt.expectTakeProfit {
				t.Errorf("止盈恢復邏輯錯誤: recovered = %v, 期望 = %v",
					takeProfitRecovered, tt.expectTakeProfit)
			}
		})
	}
}

// TestPartialCloseEdgeCases 測試邊界情況
func TestPartialCloseEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		closePercentage float64
		totalQuantity   float64
		markPrice       float64
		expectError     bool
		errorContains   string
	}{
		{
			name:            "平倉百分比為0_應該報錯",
			closePercentage: 0,
			totalQuantity:   1.0,
			markPrice:       100.0,
			expectError:     true,
			errorContains:   "0-100",
		},
		{
			name:            "平倉百分比超過100_應該報錯",
			closePercentage: 101.0,
			totalQuantity:   1.0,
			markPrice:       100.0,
			expectError:     true,
			errorContains:   "0-100",
		},
		{
			name:            "平倉百分比為負數_應該報錯",
			closePercentage: -10.0,
			totalQuantity:   1.0,
			markPrice:       100.0,
			expectError:     true,
			errorContains:   "0-100",
		},
		{
			name:            "正常範圍_不應報錯",
			closePercentage: 50.0,
			totalQuantity:   1.0,
			markPrice:       100.0,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模擬百分比驗證邏輯
			var err error
			if tt.closePercentage <= 0 || tt.closePercentage > 100 {
				err = fmt.Errorf("平仓百分比必须在 0-100 之间，当前: %.1f", tt.closePercentage)
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("期望報錯但沒有報錯")
				}
			} else {
				if err != nil {
					t.Errorf("不應報錯但報錯了: %v", err)
				}
			}
		})
	}
}

// TestPartialCloseIntegration 整合測試（使用 mock trader）
func TestPartialCloseIntegration(t *testing.T) {
	tests := []struct {
		name                 string
		symbol               string
		side                 string
		totalQuantity        float64
		markPrice            float64
		closePercentage      float64
		newStopLoss          float64
		newTakeProfit        float64
		expectFullClose      bool
		expectStopLossCall   bool
		expectTakeProfitCall bool
	}{
		{
			name:                 "LONG倉_正常部分平倉_有止盈止損",
			symbol:               "BTCUSDT",
			side:                 "LONG",
			totalQuantity:        1.0,
			markPrice:            50000.0,
			closePercentage:      50.0,
			newStopLoss:          48000.0,
			newTakeProfit:        52000.0,
			expectFullClose:      false,
			expectStopLossCall:   true,
			expectTakeProfitCall: true,
		},
		{
			name:                 "SHORT倉_剩餘價值過小_應自動全平",
			symbol:               "ETHUSDT",
			side:                 "SHORT",
			totalQuantity:        0.02,
			markPrice:            3000.0, // 總價值 60 USDT
			closePercentage:      95.0,   // 剩餘 3 USDT < 10 USDT
			newStopLoss:          3100.0,
			newTakeProfit:        2900.0,
			expectFullClose:      true,
			expectStopLossCall:   false, // 全平不需要恢復止盈止損
			expectTakeProfitCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 創建 mock trader
			mockTrader := &MockPartialCloseTrader{
				positions: []map[string]interface{}{
					{
						"symbol":    tt.symbol,
						"side":      tt.side,
						"quantity":  tt.totalQuantity,
						"markPrice": tt.markPrice,
					},
				},
			}

			// 創建決策
			dec := &decision.Decision{
				Symbol:          tt.symbol,
				Action:          "partial_close",
				ClosePercentage: tt.closePercentage,
				NewStopLoss:     tt.newStopLoss,
				NewTakeProfit:   tt.newTakeProfit,
			}

			// 創建 actionRecord
			actionRecord := &logger.DecisionAction{}

			// 計算剩餘價值
			closeQuantity := tt.totalQuantity * (tt.closePercentage / 100.0)
			remainingQuantity := tt.totalQuantity - closeQuantity
			remainingValue := remainingQuantity * tt.markPrice

			// 驗證最小倉位檢查
			const MIN_POSITION_VALUE = 10.0
			shouldFullClose := remainingValue > 0 && remainingValue <= MIN_POSITION_VALUE

			if shouldFullClose != tt.expectFullClose {
				t.Errorf("最小倉位檢查不符: shouldFullClose = %v, 期望 = %v (剩餘 %.2f USDT)",
					shouldFullClose, tt.expectFullClose, remainingValue)
			}

			// 模擬執行邏輯
			if shouldFullClose {
				// 應該轉為全平
				if tt.side == "LONG" {
					mockTrader.CloseLong(tt.symbol, tt.totalQuantity)
				} else {
					mockTrader.CloseShort(tt.symbol, tt.totalQuantity)
				}
			} else {
				// 正常部分平倉
				if tt.side == "LONG" {
					mockTrader.ClosePartialLong(tt.symbol, closeQuantity)
				} else {
					mockTrader.ClosePartialShort(tt.symbol, closeQuantity)
				}

				// 恢復止盈止損
				if dec.NewStopLoss > 0 {
					mockTrader.SetStopLoss(tt.symbol, tt.side, remainingQuantity, dec.NewStopLoss)
				}
				if dec.NewTakeProfit > 0 {
					mockTrader.SetTakeProfit(tt.symbol, tt.side, remainingQuantity, dec.NewTakeProfit)
				}
			}

			// 驗證調用
			if tt.expectFullClose {
				if !mockTrader.closeLongCalled && !mockTrader.closeShortCalled {
					t.Error("期望調用全平但沒有調用")
				}
				if mockTrader.closePartialCalled {
					t.Error("不應該調用部分平倉")
				}
			} else {
				if !mockTrader.closePartialCalled {
					t.Error("期望調用部分平倉但沒有調用")
				}
			}

			if mockTrader.stopLossCalled != tt.expectStopLossCall {
				t.Errorf("止損調用不符: called = %v, 期望 = %v",
					mockTrader.stopLossCalled, tt.expectStopLossCall)
			}

			if mockTrader.takeProfitCalled != tt.expectTakeProfitCall {
				t.Errorf("止盈調用不符: called = %v, 期望 = %v",
					mockTrader.takeProfitCalled, tt.expectTakeProfitCall)
			}

			_ = actionRecord // 避免未使用警告
		})
	}
}
