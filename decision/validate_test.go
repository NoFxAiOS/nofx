package decision

import (
	"testing"
)

// TestLeverageFallback 测试杠杆超限时的自动修正功能
func TestLeverageFallback(t *testing.T) {
	tests := []struct {
		name            string
		decision        Decision
		accountEquity   float64
		btcEthLeverage  int
		altcoinLeverage int
		wantLeverage    int // 期望修正后的杠杆值
		wantError       bool
	}{
		{
			name: "山寨币杠杆超限_自动修正为上限",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        20, // 超过上限
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5, // 上限 5x
			wantLeverage:    5, // 应该修正为 5
			wantError:       false,
		},
		{
			name: "BTC杠杆超限_自动修正为上限",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        20, // 超过上限
				PositionSizeUSD: 1000,
				StopLoss:        90000,
				TakeProfit:      110000,
			},
			accountEquity:   100,
			btcEthLeverage:  10, // 上限 10x
			altcoinLeverage: 5,
			wantLeverage:    10, // 应该修正为 10
			wantError:       false,
		},
		{
			name: "杠杆在上限内_不修正",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "open_short",
				Leverage:        5, // 未超限
				PositionSizeUSD: 500,
				StopLoss:        4000,
				TakeProfit:      3000,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    5, // 保持不变
			wantError:       false,
		},
		{
			name: "杠杆为0_应该报错",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        0, // 无效
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    0,
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, tt.accountEquity, tt.btcEthLeverage, tt.altcoinLeverage)

			// 检查错误状态
			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// 如果不应该报错，检查杠杆是否被正确修正
			if !tt.wantError && tt.decision.Leverage != tt.wantLeverage {
				t.Errorf("Leverage not corrected: got %d, want %d", tt.decision.Leverage, tt.wantLeverage)
			}
		})
	}
}

// TestUpdateStopLossValidation 测试 update_stop_loss 动作的字段验证
func TestUpdateStopLossValidation(t *testing.T) {
	tests := []struct {
		name      string
		decision  Decision
		wantError bool
		errorMsg  string
	}{
		{
			name: "正确使用new_stop_loss字段",
			decision: Decision{
				Symbol:      "SOLUSDT",
				Action:      "update_stop_loss",
				NewStopLoss: 155.5,
				Reasoning:   "移动止损至保本位",
			},
			wantError: false,
		},
		{
			name: "new_stop_loss为0应该报错",
			decision: Decision{
				Symbol:      "SOLUSDT",
				Action:      "update_stop_loss",
				NewStopLoss: 0,
				Reasoning:   "测试错误情况",
			},
			wantError: true,
			errorMsg:  "新止损价格必须大于0",
		},
		{
			name: "new_stop_loss为负数应该报错",
			decision: Decision{
				Symbol:      "SOLUSDT",
				Action:      "update_stop_loss",
				NewStopLoss: -100,
				Reasoning:   "测试错误情况",
			},
			wantError: true,
			errorMsg:  "新止损价格必须大于0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, 1000.0, 10, 5)

			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError && err != nil {
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("错误信息不匹配: got %q, want to contain %q", err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// TestUpdateTakeProfitValidation 测试 update_take_profit 动作的字段验证
func TestUpdateTakeProfitValidation(t *testing.T) {
	tests := []struct {
		name      string
		decision  Decision
		wantError bool
		errorMsg  string
	}{
		{
			name: "正确使用new_take_profit字段",
			decision: Decision{
				Symbol:        "BTCUSDT",
				Action:        "update_take_profit",
				NewTakeProfit: 98000,
				Reasoning:     "调整止盈至关键阻力位",
			},
			wantError: false,
		},
		{
			name: "new_take_profit为0应该报错",
			decision: Decision{
				Symbol:        "BTCUSDT",
				Action:        "update_take_profit",
				NewTakeProfit: 0,
				Reasoning:     "测试错误情况",
			},
			wantError: true,
			errorMsg:  "新止盈价格必须大于0",
		},
		{
			name: "new_take_profit为负数应该报错",
			decision: Decision{
				Symbol:        "BTCUSDT",
				Action:        "update_take_profit",
				NewTakeProfit: -1000,
				Reasoning:     "测试错误情况",
			},
			wantError: true,
			errorMsg:  "新止盈价格必须大于0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, 1000.0, 10, 5)

			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError && err != nil {
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("错误信息不匹配: got %q, want to contain %q", err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// TestPartialCloseValidation 测试 partial_close 动作的字段验证
func TestPartialCloseValidation(t *testing.T) {
	tests := []struct {
		name      string
		decision  Decision
		wantError bool
		errorMsg  string
	}{
		{
			name: "正确使用close_percentage字段",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "partial_close",
				ClosePercentage: 50.0,
				Reasoning:       "锁定一半利润",
			},
			wantError: false,
		},
		{
			name: "close_percentage为0应该报错",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "partial_close",
				ClosePercentage: 0,
				Reasoning:       "测试错误情况",
			},
			wantError: true,
			errorMsg:  "平仓百分比必须在0-100之间",
		},
		{
			name: "close_percentage超过100应该报错",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "partial_close",
				ClosePercentage: 150,
				Reasoning:       "测试错误情况",
			},
			wantError: true,
			errorMsg:  "平仓百分比必须在0-100之间",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, 1000.0, 10, 5)

			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.wantError && err != nil {
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("错误信息不匹配: got %q, want to contain %q", err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

// contains 检查字符串是否包含子串（辅助函数）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
