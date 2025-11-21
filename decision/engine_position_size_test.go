package decision

import (
	"testing"
)

// TestCalculateMinPositionSize 測試動態最小開倉金額計算
func TestCalculateMinPositionSize(t *testing.T) {
	tests := []struct {
		name          string
		symbol        string
		accountEquity float64
		expected      float64
		description   string
	}{
		// 山寨幣測試用例（始終返回絕對最小值）
		{
			name:          "山寨幣_極小賬戶",
			symbol:        "ADAUSDT",
			accountEquity: 5.0,
			expected:      12.0,
			description:   "山寨幣不論賬戶大小都返回 12 USDT",
		},
		{
			name:          "山寨幣_小賬戶",
			symbol:        "SOLUSDT",
			accountEquity: 50.0,
			expected:      12.0,
			description:   "山寨幣不論賬戶大小都返回 12 USDT",
		},
		{
			name:          "山寨幣_大賬戶",
			symbol:        "DOGEUSDT",
			accountEquity: 500.0,
			expected:      12.0,
			description:   "山寨幣不論賬戶大小都返回 12 USDT",
		},

		// BTC/ETH 測試用例（動態調整）
		// 分支 1: 極小賬戶 (< 20U)
		{
			name:          "BTC_極小賬戶_實際案例",
			symbol:        "BTCUSDT",
			accountEquity: 6.97,
			expected:      12.0,
			description:   "用戶實際報告的案例，應返回 12 USDT 降低門檻",
		},
		{
			name:          "ETH_極小賬戶",
			symbol:        "ETHUSDT",
			accountEquity: 15.0,
			expected:      12.0,
			description:   "< 20U 賬戶返回絕對最小值",
		},
		{
			name:          "BTC_邊界_19.99U",
			symbol:        "BTCUSDT",
			accountEquity: 19.99,
			expected:      12.0,
			description:   "邊界測試：剛好低於 20U 閾值",
		},

		// 分支 2: 中型賬戶 (20-100U) - 線性插值
		{
			name:          "BTC_邊界_20U",
			symbol:        "BTCUSDT",
			accountEquity: 20.0,
			expected:      12.0,
			description:   "線性插值起點：20U → 12 USDT",
		},
		{
			name:          "BTC_中型賬戶_60U",
			symbol:        "BTCUSDT",
			accountEquity: 60.0,
			expected:      36.0, // 12 + (60-12) * (60-20)/80 = 12 + 48*0.5 = 36
			description:   "線性插值中點：60U → 36 USDT",
		},
		{
			name:          "ETH_中型賬戶_80U",
			symbol:        "ETHUSDT",
			accountEquity: 80.0,
			expected:      48.0, // 12 + (60-12) * (80-20)/80 = 12 + 48*0.75 = 48
			description:   "線性插值 75%：80U → 48 USDT",
		},
		{
			name:          "BTC_邊界_99.99U",
			symbol:        "BTCUSDT",
			accountEquity: 99.99,
			expected:      59.994, // 12 + (60-12) * (99.99-20)/80
			description:   "邊界測試：剛好低於 100U 閾值",
		},

		// 分支 3: 大賬戶 (≥ 100U)
		{
			name:          "BTC_邊界_100U",
			symbol:        "BTCUSDT",
			accountEquity: 100.0,
			expected:      60.0,
			description:   "標準門檻起點：100U → 60 USDT",
		},
		{
			name:          "ETH_大賬戶_500U",
			symbol:        "ETHUSDT",
			accountEquity: 500.0,
			expected:      60.0,
			description:   "大賬戶返回標準值",
		},
		{
			name:          "BTC_超大賬戶_10000U",
			symbol:        "BTCUSDT",
			accountEquity: 10000.0,
			expected:      60.0,
			description:   "超大賬戶同樣返回標準值",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMinPositionSize(tt.symbol, tt.accountEquity)

			// 使用浮點數容差比較（0.01 USDT 精度）
			tolerance := 0.01
			if result < tt.expected-tolerance || result > tt.expected+tolerance {
				t.Errorf("%s 失敗:\n  期望: %.2f USDT\n  實際: %.2f USDT\n  說明: %s",
					tt.name, tt.expected, result, tt.description)
			}
		})
	}
}

// TestCalculateMinPositionSize_EdgeCases 測試邊界情況
func TestCalculateMinPositionSize_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		symbol        string
		accountEquity float64
		expected      float64
		description   string
	}{
		{
			name:          "零賬戶淨值",
			symbol:        "BTCUSDT",
			accountEquity: 0.0,
			expected:      12.0,
			description:   "賬戶淨值為 0 時應返回最小值",
		},
		{
			name:          "負賬戶淨值",
			symbol:        "ETHUSDT",
			accountEquity: -10.0,
			expected:      12.0,
			description:   "負淨值（理論不應出現）應返回最小值",
		},
		{
			name:          "空字符串符號",
			symbol:        "",
			accountEquity: 100.0,
			expected:      12.0,
			description:   "非 BTC/ETH 符號返回絕對最小值",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMinPositionSize(tt.symbol, tt.accountEquity)
			if result != tt.expected {
				t.Errorf("%s 失敗: 期望 %.2f, 實際 %.2f (%s)",
					tt.name, tt.expected, result, tt.description)
			}
		})
	}
}

// TestCalculateMinPositionSize_MapConfiguration 測試 map 配置邏輯
func TestCalculateMinPositionSize_MapConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		symbol        string
		accountEquity float64
		expected      float64
		description   string
	}{
		// 測試配置中的 BTC/ETH
		{
			name:          "Map配置_BTC_小賬戶",
			symbol:        "BTCUSDT",
			accountEquity: 10.0,
			expected:      12.0,
			description:   "BTC 在 map 中配置，應使用 btcEthSizeRules",
		},
		{
			name:          "Map配置_ETH_大賬戶",
			symbol:        "ETHUSDT",
			accountEquity: 200.0,
			expected:      60.0,
			description:   "ETH 在 map 中配置，應使用 btcEthSizeRules",
		},
		// 測試未配置的山寨幣（應使用 altcoinSizeRules）
		{
			name:          "未配置_SOL_小賬戶",
			symbol:        "SOLUSDT",
			accountEquity: 10.0,
			expected:      12.0,
			description:   "SOL 未配置，應使用 altcoinSizeRules 默認值",
		},
		{
			name:          "未配置_BNB_大賬戶",
			symbol:        "BNBUSDT",
			accountEquity: 1000.0,
			expected:      12.0,
			description:   "BNB 未配置，大賬戶也應返回山寨幣最小值 12 USDT",
		},
		{
			name:          "未配置_MATIC_中型賬戶",
			symbol:        "MATICUSDT",
			accountEquity: 50.0,
			expected:      12.0,
			description:   "MATIC 未配置，任何賬戶規模都返回 12 USDT",
		},
		{
			name:          "未配置_AVAX",
			symbol:        "AVAXUSDT",
			accountEquity: 100.0,
			expected:      12.0,
			description:   "AVAX 未配置，使用山寨幣規則",
		},
		{
			name:          "未配置_DOT",
			symbol:        "DOTUSDT",
			accountEquity: 75.0,
			expected:      12.0,
			description:   "DOT 未配置，使用山寨幣規則",
		},
		{
			name:          "未配置_LINK",
			symbol:        "LINKUSDT",
			accountEquity: 150.0,
			expected:      12.0,
			description:   "LINK 未配置，使用山寨幣規則",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateMinPositionSize(tt.symbol, tt.accountEquity)
			if result != tt.expected {
				t.Errorf("%s 失敗: 期望 %.2f, 實際 %.2f (%s)",
					tt.name, tt.expected, result, tt.description)
			}
		})
	}
}

// TestCalculateMinPositionSize_LinearInterpolation 測試線性插值準確性
func TestCalculateMinPositionSize_LinearInterpolation(t *testing.T) {
	// 測試多個點驗證線性插值的連續性
	testPoints := []float64{20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0}
	var prevSize float64

	for i, equity := range testPoints {
		size := calculateMinPositionSize("BTCUSDT", equity)

		// 驗證單調遞增
		if i > 0 && size < prevSize {
			t.Errorf("線性插值非單調遞增: %.2fU → %.2f USDT, %.2fU → %.2f USDT",
				testPoints[i-1], prevSize, equity, size)
		}

		// 驗證邊界值
		if equity == 20.0 && size != 12.0 {
			t.Errorf("起點錯誤: 20U 應為 12 USDT，實際 %.2f", size)
		}
		if equity == 100.0 && size != 60.0 {
			t.Errorf("終點錯誤: 100U 應為 60 USDT，實際 %.2f", size)
		}

		prevSize = size
	}
}

// Benchmark 性能測試
func BenchmarkCalculateMinPositionSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		calculateMinPositionSize("BTCUSDT", 50.0)
	}
}
