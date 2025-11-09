package trader

import (
	"math"
	"testing"
)

func TestCalculatePnLPercentage(t *testing.T) {
	tests := []struct {
		name           string
		unrealizedPnl  float64
		marginUsed     float64
		expected       float64
	}{
		{
			name:           "正常盈利 - 10倍杠杆",
			unrealizedPnl:  100.0,  // 盈利 100 USDT
			marginUsed:     1000.0, // 保证金 1000 USDT
			expected:       10.0,   // 10% 收益率
		},
		{
			name:           "正常亏损 - 10倍杠杆",
			unrealizedPnl:  -50.0,  // 亏损 50 USDT
			marginUsed:     1000.0, // 保证金 1000 USDT
			expected:       -5.0,   // -5% 收益率
		},
		{
			name:           "高杠杆盈利 - 价格上涨1%，20倍杠杆",
			unrealizedPnl:  200.0,  // 盈利 200 USDT
			marginUsed:     1000.0, // 保证金 1000 USDT
			expected:       20.0,   // 20% 收益率
		},
		{
			name:           "保证金为0 - 边界情况",
			unrealizedPnl:  100.0,
			marginUsed:     0.0,
			expected:       0.0, // 应该返回 0 而不是除以零错误
		},
		{
			name:           "负保证金 - 边界情况",
			unrealizedPnl:  100.0,
			marginUsed:     -1000.0,
			expected:       0.0, // 应该返回 0（异常情况）
		},
		{
			name:           "盈亏为0",
			unrealizedPnl:  0.0,
			marginUsed:     1000.0,
			expected:       0.0,
		},
		{
			name:           "小额交易",
			unrealizedPnl:  0.5,
			marginUsed:     10.0,
			expected:       5.0,
		},
		{
			name:           "大额盈利",
			unrealizedPnl:  5000.0,
			marginUsed:     10000.0,
			expected:       50.0,
		},
		{
			name:           "极小保证金",
			unrealizedPnl:  1.0,
			marginUsed:     0.01,
			expected:       10000.0, // 100倍收益率
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePnLPercentage(tt.unrealizedPnl, tt.marginUsed)

			// 使用精度比较，避免浮点数误差
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("calculatePnLPercentage(%v, %v) = %v, want %v",
					tt.unrealizedPnl, tt.marginUsed, result, tt.expected)
			}
		})
	}
}

// TestCalculatePnLPercentage_RealWorldScenarios 真实场景测试
func TestCalculatePnLPercentage_RealWorldScenarios(t *testing.T) {
	t.Run("BTC 10倍杠杆，价格上涨2%", func(t *testing.T) {
		// 开仓：1000 USDT 保证金，10倍杠杆 = 10000 USDT 仓位
		// 价格上涨 2% = 200 USDT 盈利
		// 收益率 = 200 / 1000 = 20%
		result := calculatePnLPercentage(200.0, 1000.0)
		expected := 20.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("BTC场景: got %v, want %v", result, expected)
		}
	})

	t.Run("ETH 5倍杠杆，价格下跌3%", func(t *testing.T) {
		// 开仓：2000 USDT 保证金，5倍杠杆 = 10000 USDT 仓位
		// 价格下跌 3% = -300 USDT 亏损
		// 收益率 = -300 / 2000 = -15%
		result := calculatePnLPercentage(-300.0, 2000.0)
		expected := -15.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("ETH场景: got %v, want %v", result, expected)
		}
	})

	t.Run("SOL 20倍杠杆，价格上涨0.5%", func(t *testing.T) {
		// 开仓：500 USDT 保证金，20倍杠杆 = 10000 USDT 仓位
		// 价格上涨 0.5% = 50 USDT 盈利
		// 收益率 = 50 / 500 = 10%
		result := calculatePnLPercentage(50.0, 500.0)
		expected := 10.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("SOL场景: got %v, want %v", result, expected)
		}
	})
}
