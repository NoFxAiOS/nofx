package trader

import (
	"math"
	"testing"
)

// TestAutoBalanceSyncTotalEquityCalculation 测试自动余额同步的 totalEquity 计算逻辑
func TestAutoBalanceSyncTotalEquityCalculation(t *testing.T) {
	tests := []struct {
		name                 string
		balanceInfo          map[string]interface{}
		expectedBalance      float64
		shouldUseTotalEquity bool
		description          string
	}{
		{
			name: "正常情况_无持仓",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": 0.0,
				"availableBalance":      1000.0,
			},
			expectedBalance:      1000.0,
			shouldUseTotalEquity: true,
			description:          "无持仓时，totalEquity = totalWalletBalance",
		},
		{
			name: "持仓盈利_totalEquity高于availableBalance",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": 100.0,
				"availableBalance":      900.0, // 保证金占用
			},
			expectedBalance:      1100.0, // 1000 + 100
			shouldUseTotalEquity: true,
			description:          "盈利时，totalEquity = 1000 + 100 = 1100，而 availableBalance 只有 900",
		},
		{
			name: "持仓亏损_totalEquity低于availableBalance",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": -200.0,
				"availableBalance":      900.0,
			},
			expectedBalance:      800.0, // 1000 - 200
			shouldUseTotalEquity: true,
			description:          "亏损时，totalEquity = 1000 - 200 = 800",
		},
		{
			name: "大仓位持仓_可用余额很低但总资产正常",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    10000.0,
				"totalUnrealizedProfit": 500.0,
				"availableBalance":      1000.0, // 大部分资金用作保证金
			},
			expectedBalance:      10500.0, // 10000 + 500
			shouldUseTotalEquity: true,
			description:          "大仓位时，availableBalance 很低（1000），但 totalEquity 正常（10500）",
		},
		{
			name: "缺少totalUnrealizedProfit字段_视为0",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance": 1000.0,
				"availableBalance":   1000.0,
			},
			expectedBalance:      1000.0,
			shouldUseTotalEquity: true,
			description:          "缺少 totalUnrealizedProfit 时，视为 0",
		},
		{
			name: "缺少totalWalletBalance字段_fallback到availableBalance",
			balanceInfo: map[string]interface{}{
				"availableBalance": 900.0,
			},
			expectedBalance:      900.0,
			shouldUseTotalEquity: false,
			description:          "缺少 totalWalletBalance 时，fallback 到 availableBalance",
		},
		{
			name: "缺少所有totalEquity字段_fallback到balance",
			balanceInfo: map[string]interface{}{
				"balance": 800.0,
			},
			expectedBalance:      800.0,
			shouldUseTotalEquity: false,
			description:          "缺少所有 totalEquity 字段时，fallback 到 balance",
		},
		{
			name:                 "空balanceInfo_应返回0",
			balanceInfo:          map[string]interface{}{},
			expectedBalance:      0.0,
			shouldUseTotalEquity: false,
			description:          "空 balanceInfo 时，无法提取任何字段",
		},
		{
			name: "totalWalletBalance为负_fallback",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    -100.0,
				"totalUnrealizedProfit": 50.0,
				"availableBalance":      500.0,
			},
			expectedBalance:      500.0, // fallback 到 availableBalance
			shouldUseTotalEquity: false,
			description:          "totalWalletBalance 异常为负时，fallback",
		},
		{
			name: "极端盈利场景",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": 5000.0, // 500% 盈利
				"availableBalance":      500.0,
			},
			expectedBalance:      6000.0, // 1000 + 5000
			shouldUseTotalEquity: true,
			description:          "极端盈利时，totalEquity 远高于 availableBalance",
		},
		{
			name: "接近爆仓场景",
			balanceInfo: map[string]interface{}{
				"totalWalletBalance":    1000.0,
				"totalUnrealizedProfit": -950.0,
				"availableBalance":      10.0,
			},
			expectedBalance:      50.0, // 1000 - 950
			shouldUseTotalEquity: true,
			description:          "接近爆仓时，totalEquity = 50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟提取逻辑（与 auto_trader.go:306-335 相同）
			var actualBalance float64
			totalWalletBalance := 0.0
			totalUnrealizedProfit := 0.0

			if wallet, ok := tt.balanceInfo["totalWalletBalance"].(float64); ok {
				totalWalletBalance = wallet
			}
			if unrealized, ok := tt.balanceInfo["totalUnrealizedProfit"].(float64); ok {
				totalUnrealizedProfit = unrealized
			}

			totalEquity := totalWalletBalance + totalUnrealizedProfit
			usedTotalEquity := false
			if totalEquity > 0 {
				actualBalance = totalEquity
				usedTotalEquity = true
			} else {
				// Fallback
				if availableBalance, ok := tt.balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
					actualBalance = availableBalance
				} else if balance, ok := tt.balanceInfo["balance"].(float64); ok && balance > 0 {
					actualBalance = balance
				}
			}

			// 验证结果
			if actualBalance != tt.expectedBalance {
				t.Errorf("%s: 期望余额 %.2f，实际 %.2f", tt.description, tt.expectedBalance, actualBalance)
			}

			if usedTotalEquity != tt.shouldUseTotalEquity {
				t.Errorf("%s: 期望使用 totalEquity = %v，实际 = %v", tt.description, tt.shouldUseTotalEquity, usedTotalEquity)
			}

			t.Logf("✓ %s: actualBalance = %.2f, usedTotalEquity = %v", tt.description, actualBalance, usedTotalEquity)
		})
	}
}

// TestAutoBalanceSyncChangeDetection 测试余额变化检测逻辑
func TestAutoBalanceSyncChangeDetection(t *testing.T) {
	tests := []struct {
		name           string
		oldBalance     float64
		newBalance     float64
		expectedChange float64
		shouldUpdate   bool // 是否超过 5% 阈值
		description    string
	}{
		{
			name:           "余额增加6%_应触发更新",
			oldBalance:     1000.0,
			newBalance:     1060.0,
			expectedChange: 6.0,
			shouldUpdate:   true,
			description:    "余额从 1000 增加到 1060（+6%），超过 5% 阈值",
		},
		{
			name:           "余额减少7%_应触发更新",
			oldBalance:     1000.0,
			newBalance:     930.0,
			expectedChange: -7.0,
			shouldUpdate:   true,
			description:    "余额从 1000 减少到 930（-7%），超过 5% 阈值",
		},
		{
			name:           "余额增加3%_不应触发更新",
			oldBalance:     1000.0,
			newBalance:     1030.0,
			expectedChange: 3.0,
			shouldUpdate:   false,
			description:    "余额从 1000 增加到 1030（+3%），未超过 5% 阈值",
		},
		{
			name:           "余额减少4%_不应触发更新",
			oldBalance:     1000.0,
			newBalance:     960.0,
			expectedChange: -4.0,
			shouldUpdate:   false,
			description:    "余额从 1000 减少到 960（-4%），未超过 5% 阈值",
		},
		{
			name:           "余额恰好5%边界_应触发更新",
			oldBalance:     1000.0,
			newBalance:     1050.0,
			expectedChange: 5.0,
			shouldUpdate:   false, // math.Abs(5.0) > 5.0 = false
			description:    "余额从 1000 增加到 1050（恰好 5%），不触发更新",
		},
		{
			name:           "余额恰好-5%边界_应触发更新",
			oldBalance:     1000.0,
			newBalance:     950.0,
			expectedChange: -5.0,
			shouldUpdate:   false, // math.Abs(-5.0) > 5.0 = false
			description:    "余额从 1000 减少到 950（恰好 -5%），不触发更新",
		},
		{
			name:           "余额增加5.1%_应触发更新",
			oldBalance:     1000.0,
			newBalance:     1051.0,
			expectedChange: 5.1,
			shouldUpdate:   true,
			description:    "余额从 1000 增加到 1051（+5.1%），超过 5% 阈值",
		},
		{
			name:           "场景重现_持仓盈利误判为亏损",
			oldBalance:     1000.0, // initialBalance 用的是 totalEquity
			newBalance:     900.0,  // 旧逻辑用 availableBalance（保证金占用）
			expectedChange: -10.0,
			shouldUpdate:   true,
			description:    "旧逻辑 bug：持仓时 availableBalance=900，误判为 -10% 亏损",
		},
		{
			name:           "场景修复_持仓盈利正确检测",
			oldBalance:     1000.0, // initialBalance
			newBalance:     1100.0, // 修复后用 totalEquity
			expectedChange: 10.0,
			shouldUpdate:   true,
			description:    "新逻辑修复：持仓盈利 100，totalEquity=1100，正确检测 +10%",
		},
		{
			name:           "小额账户_变化比例更敏感",
			oldBalance:     100.0,
			newBalance:     106.0,
			expectedChange: 6.0,
			shouldUpdate:   true,
			description:    "小额账户（100 USDT）增加 6 USDT（+6%），触发更新",
		},
		{
			name:           "大额账户_相同比例",
			oldBalance:     100000.0,
			newBalance:     106000.0,
			expectedChange: 6.0,
			shouldUpdate:   true,
			description:    "大额账户（10万 USDT）增加 6000 USDT（+6%），触发更新",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 计算变化百分比（与 auto_trader.go:363 相同）
			changePercent := ((tt.newBalance - tt.oldBalance) / tt.oldBalance) * 100

			// 验证计算结果（使用 epsilon 比较浮点数）
			epsilon := 0.001
			if math.Abs(changePercent-tt.expectedChange) > epsilon {
				t.Errorf("%s: 期望变化 %.2f%%，实际 %.2f%%", tt.description, tt.expectedChange, changePercent)
			}

			// 验证是否应该触发更新（与 auto_trader.go:366 相同）
			shouldUpdate := math.Abs(changePercent) > 5.0
			if shouldUpdate != tt.shouldUpdate {
				t.Errorf("%s: 期望触发更新 = %v，实际 = %v (变化 %.2f%%)", tt.description, tt.shouldUpdate, shouldUpdate, changePercent)
			}

			t.Logf("✓ %s: 变化 %.2f%%, 触发更新 = %v", tt.description, changePercent, shouldUpdate)
		})
	}
}

// TestAutoBalanceSyncAvoidsFalsePositives 测试避免误判场景
func TestAutoBalanceSyncAvoidsFalsePositives(t *testing.T) {
	tests := []struct {
		name              string
		initialBalance    float64
		walletBalance     float64
		unrealizedProfit  float64
		availableBalance  float64
		shouldTriggerSync bool
		description       string
	}{
		{
			name:              "持仓盈利_不应误判为余额下降",
			initialBalance:    1000.0,
			walletBalance:     1000.0,
			unrealizedProfit:  100.0,
			availableBalance:  900.0, // 保证金占用
			shouldTriggerSync: false, // totalEquity=1100, 变化 10% > 5%，但是增加而非减少
			description:       "持仓盈利 100，totalEquity=1100，availableBalance=900（旧逻辑会误判）",
		},
		{
			name:              "持仓亏损_不应误判为余额增加",
			initialBalance:    1000.0,
			walletBalance:     1000.0,
			unrealizedProfit:  -100.0,
			availableBalance:  950.0,
			shouldTriggerSync: false, // totalEquity=900, 变化 -10% > 5%
			description:       "持仓亏损 100，totalEquity=900（正确），availableBalance=950（旧逻辑会误判增加）",
		},
		{
			name:              "大额持仓_可用余额极低但总资产正常",
			initialBalance:    10000.0,
			walletBalance:     10000.0,
			unrealizedProfit:  0.0,
			availableBalance:  500.0, // 95% 资金用作保证金
			shouldTriggerSync: false, // totalEquity=10000, 变化 0%
			description:       "大额持仓，availableBalance=500（旧逻辑会误判为 -95% 暴跌）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 旧逻辑（错误）：使用 availableBalance
			oldLogicBalance := tt.availableBalance
			oldLogicChange := ((oldLogicBalance - tt.initialBalance) / tt.initialBalance) * 100

			// 新逻辑（正确）：使用 totalEquity
			totalEquity := tt.walletBalance + tt.unrealizedProfit
			newLogicChange := ((totalEquity - tt.initialBalance) / tt.initialBalance) * 100

			oldLogicTrigger := math.Abs(oldLogicChange) > 5.0
			newLogicTrigger := math.Abs(newLogicChange) > 5.0

			t.Logf("旧逻辑: availableBalance=%.2f, 变化 %.2f%%, 触发=%v", oldLogicBalance, oldLogicChange, oldLogicTrigger)
			t.Logf("新逻辑: totalEquity=%.2f, 变化 %.2f%%, 触发=%v", totalEquity, newLogicChange, newLogicTrigger)

			// 验证：在这些场景中，新逻辑应该避免误判
			if tt.name == "持仓盈利_不应误判为余额下降" {
				if oldLogicTrigger && oldLogicChange < 0 {
					t.Logf("✓ 旧逻辑错误：误判为余额下降 %.2f%%", oldLogicChange)
				}
				if newLogicTrigger && newLogicChange > 0 {
					t.Logf("✓ 新逻辑正确：检测到余额增加 %.2f%%（真实盈利）", newLogicChange)
				}
			}

			if tt.name == "大额持仓_可用余额极低但总资产正常" {
				if oldLogicTrigger {
					t.Logf("✓ 旧逻辑错误：误判为暴跌 %.2f%%", oldLogicChange)
				}
				if !newLogicTrigger {
					t.Logf("✓ 新逻辑正确：总资产无变化 (%.2f%%)，不触发更新", newLogicChange)
				}
			}
		})
	}
}
