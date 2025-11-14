package decision

import (
	"testing"
)

func TestValidateAndSanitizeDecision(t *testing.T) {
	tests := []struct {
		name            string
		decision        Decision
		accountEquity   float64
		btcLeverage     int
		altcoinLeverage int
		wantAction      string
		wantError       bool
	}{
		{
			name: "有效做多决策",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        5,
				PositionSizeUSD: 1000,
				StopLoss:        50000,
				TakeProfit:      60000,
				Confidence:      80,
			},
			accountEquity:   10000,
			btcLeverage:     10,
			altcoinLeverage: 5,
			wantAction:      "open_long",
			wantError:       false,
		},
		{
			name: "无效杠杆降级为wait",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        0, // 无效杠杆
				PositionSizeUSD: 1000,
				StopLoss:        50000,
				TakeProfit:      60000,
				Reasoning:       "测试决策",
			},
			accountEquity:   10000,
			btcLeverage:     10,
			altcoinLeverage: 5,
			wantAction:      "wait",
			wantError:       true,
		},
		{
			name: "仓位过小降级为wait",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        5,
				PositionSizeUSD: 5, // 小于最小仓位
				StopLoss:        50000,
				TakeProfit:      60000,
			},
			accountEquity:   10000,
			btcLeverage:     10,
			altcoinLeverage: 5,
			wantAction:      "wait",
			wantError:       true,
		},
		{
			name: "风险回报比不足降级为wait",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        5,
				PositionSizeUSD: 1000,
				StopLoss:        55000, // 止损太接近止盈
				TakeProfit:      56000,
			},
			accountEquity:   10000,
			btcLeverage:     10,
			altcoinLeverage: 5,
			wantAction:      "wait",
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 复制决策以避免修改原始测试数据
			decision := tt.decision

			err := validateAndSanitizeDecision(&decision, 0, tt.accountEquity, tt.btcLeverage, tt.altcoinLeverage)

			// 检查action是否正确降级
			if decision.Action != tt.wantAction {
				t.Errorf("validateAndSanitizeDecision() action = %v, want %v", decision.Action, tt.wantAction)
			}

			// 检查错误返回是否符合预期
			if (err != nil) != tt.wantError {
				t.Errorf("validateAndSanitizeDecision() error = %v, wantError %v", err, tt.wantError)
			}

			// 如果决策被降级，检查reasoning是否包含错误信息
			if tt.wantError && decision.Action == "wait" {
				if decision.Reasoning == "" {
					t.Error("validateAndSanitizeDecision() 降级决策应该包含reasoning")
				}
			}
		})
	}
}

func TestValidateDecisions(t *testing.T) {
	tests := []struct {
		name            string
		decisions       []Decision
		accountEquity   float64
		btcLeverage     int
		altcoinLeverage int
		wantError       bool
		wantValidCount  int // 期望的有效决策数量
	}{
		{
			name: "全部有效决策",
			decisions: []Decision{
				{
					Symbol:          "BTCUSDT",
					Action:          "open_long",
					Leverage:        5,
					PositionSizeUSD: 1000,
					StopLoss:        50000,
					TakeProfit:      60000,
				},
				{
					Symbol:    "ETHUSDT",
					Action:    "hold",
					Reasoning: "继续持有",
				},
			},
			accountEquity:   10000,
			btcLeverage:     10,
			altcoinLeverage: 5,
			wantError:       false,
			wantValidCount:  2,
		},
		{
			name: "部分无效决策",
			decisions: []Decision{
				{
					Symbol:          "BTCUSDT",
					Action:          "open_long",
					Leverage:        5,
					PositionSizeUSD: 1000,
					StopLoss:        50000,
					TakeProfit:      60000,
				},
				{
					Symbol:          "INVALID",
					Action:          "open_long",
					Leverage:        0, // 无效
					PositionSizeUSD: 1000,
					StopLoss:        50000,
					TakeProfit:      60000,
				},
			},
			accountEquity:   10000,
			btcLeverage:     10,
			altcoinLeverage: 5,
			wantError:       true, // 有验证错误
			wantValidCount:  1,    // 只有第一个有效
		},
		{
			name:            "空决策列表",
			decisions:       []Decision{},
			accountEquity:   10000,
			btcLeverage:     10,
			altcoinLeverage: 5,
			wantError:       false,
			wantValidCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 复制决策切片以避免修改原始测试数据
			decisions := make([]Decision, len(tt.decisions))
			copy(decisions, tt.decisions)

			err := validateDecisions(decisions, tt.accountEquity, tt.btcLeverage, tt.altcoinLeverage)

			// 检查错误返回
			if (err != nil) != tt.wantError {
				t.Errorf("validateDecisions() error = %v, wantError %v", err, tt.wantError)
			}

			// 检查有效决策数量
			validCount := 0
			for _, d := range decisions {
				if d.Action != "wait" {
					validCount++
				}
			}
			if validCount != tt.wantValidCount {
				t.Errorf("validateDecisions() validCount = %v, want %v", validCount, tt.wantValidCount)
			}
		})
	}
}
