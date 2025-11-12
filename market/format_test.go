package market

import (
	"strings"
	"testing"
)

// TestGetSortedTimeframes 测试时间框架排序功能
func TestGetSortedTimeframes(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]*TimeframeData
		expected []string
	}{
		{
			name: "默认配置-3m和4h",
			input: map[string]*TimeframeData{
				"4h": {Timeframe: "4h"},
				"3m": {Timeframe: "3m"},
			},
			expected: []string{"3m", "4h"},
		},
		{
			name: "单个时间框架-1h",
			input: map[string]*TimeframeData{
				"1h": {Timeframe: "1h"},
			},
			expected: []string{"1h"},
		},
		{
			name: "多个时间框架-乱序输入",
			input: map[string]*TimeframeData{
				"1d":  {Timeframe: "1d"},
				"15m": {Timeframe: "15m"},
				"1h":  {Timeframe: "1h"},
				"3m":  {Timeframe: "3m"},
			},
			expected: []string{"3m", "15m", "1h", "1d"},
		},
		{
			name: "所有11个时间框架",
			input: map[string]*TimeframeData{
				"1m":  {Timeframe: "1m"},
				"3m":  {Timeframe: "3m"},
				"5m":  {Timeframe: "5m"},
				"15m": {Timeframe: "15m"},
				"30m": {Timeframe: "30m"},
				"1h":  {Timeframe: "1h"},
				"2h":  {Timeframe: "2h"},
				"4h":  {Timeframe: "4h"},
				"6h":  {Timeframe: "6h"},
				"12h": {Timeframe: "12h"},
				"1d":  {Timeframe: "1d"},
			},
			expected: []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "12h", "1d"},
		},
		{
			name:     "空map",
			input:    map[string]*TimeframeData{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSortedTimeframes(tt.input)

			// 检查长度
			if len(result) != len(tt.expected) {
				t.Errorf("长度不匹配: got %d, want %d", len(result), len(tt.expected))
				return
			}

			// 检查顺序
			for i, tf := range result {
				if tf != tt.expected[i] {
					t.Errorf("索引 %d: got %s, want %s", i, tf, tt.expected[i])
				}
			}
		})
	}
}

// TestFormatTimeframeData 测试单个时间框架格式化
func TestFormatTimeframeData(t *testing.T) {
	tests := []struct {
		name        string
		input       *TimeframeData
		contains    []string // 应该包含的字符串
		notContains []string // 不应该包含的字符串
	}{
		{
			name: "完整的3m数据",
			input: &TimeframeData{
				Timeframe:      "3m",
				DataPoints:     40,
				MidPrices:      []float64{100.0, 101.0, 102.0},
				EMA20Values:    []float64{100.5, 101.5, 102.5},
				MACDValues:     []float64{0.1, 0.2, 0.3},
				RSI7Values:     []float64{50.0, 55.0, 60.0},
				RSI14Values:    []float64{52.0, 57.0, 62.0},
				BollingerUpper: []float64{105.0, 106.0, 107.0},
				BollingerMid:   []float64{100.0, 101.0, 102.0},
				BollingerLower: []float64{95.0, 96.0, 97.0},
				Volume:         []float64{1000.0, 1100.0, 1200.0},
				ATR14:          2.5,
			},
			contains: []string{
				"3-minute timeframe",
				"Mid prices:",
				"EMA indicators (20‑period):",
				"MACD indicators:",
				"RSI indicators (7‑Period):",
				"RSI indicators (14‑Period):",
				"Bollinger Bands (20‑period, 2σ):",
				"Volume:",
				"3m ATR (14‑period): 2.500",
			},
			notContains: []string{
				"4-hour",
				"1-hour",
			},
		},
		{
			name: "1h数据-仅基础指标",
			input: &TimeframeData{
				Timeframe:   "1h",
				DataPoints:  30,
				MidPrices:   []float64{100.0, 101.0},
				EMA20Values: []float64{100.5, 101.5},
				ATR14:       3.2,
			},
			contains: []string{
				"1-hour timeframe",
				"Mid prices:",
				"EMA indicators (20‑period):",
				"1h ATR (14‑period): 3.200",
			},
			notContains: []string{
				"MACD indicators:",
				"RSI indicators",
				"Bollinger Bands",
				"Volume:",
			},
		},
		{
			name:        "nil数据",
			input:       nil,
			contains:    []string{},
			notContains: []string{"timeframe", "Mid prices", "ATR"},
		},
		{
			name: "空数组-只有时间框架",
			input: &TimeframeData{
				Timeframe:  "4h",
				DataPoints: 25,
				ATR14:      1.8,
			},
			contains: []string{
				"4-hour timeframe",
				"4h ATR (14‑period): 1.800",
			},
			notContains: []string{
				"Mid prices:",
				"MACD",
				"RSI",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimeframeData(tt.input)

			// 检查应该包含的字符串
			for _, str := range tt.contains {
				if !strings.Contains(result, str) {
					t.Errorf("输出应该包含 '%s', 但没有找到\n输出:\n%s", str, result)
				}
			}

			// 检查不应该包含的字符串
			for _, str := range tt.notContains {
				if strings.Contains(result, str) {
					t.Errorf("输出不应该包含 '%s', 但找到了\n输出:\n%s", str, result)
				}
			}
		})
	}
}

// TestFormat_DynamicTimeframes 测试Format函数使用动态TimeframeData
func TestFormat_DynamicTimeframes(t *testing.T) {
	tests := []struct {
		name        string
		input       *Data
		contains    []string
		notContains []string
	}{
		{
			name: "使用TimeframeData-单个3m",
			input: &Data{
				Symbol:       "BTCUSDT",
				CurrentPrice: 50000.0,
				CurrentEMA20: 49500.0,
				CurrentMACD:  100.0,
				CurrentRSI7:  65.0,
				OpenInterest: &OIData{Latest: 1000000.0, Average: 950000.0},
				FundingRate:  0.0001,
				TimeframeData: map[string]*TimeframeData{
					"3m": {
						Timeframe:   "3m",
						DataPoints:  40,
						MidPrices:   []float64{49900.0, 50000.0, 50100.0},
						EMA20Values: []float64{49800.0, 49900.0, 50000.0},
						ATR14:       500.0,
					},
				},
			},
			contains: []string{
				"current_price = 50000.00",
				"BTCUSDT",
				"Open Interest:",
				"Funding Rate:",
				"3-minute timeframe",
				"Mid prices:",
				"3m ATR (14‑period): 500.000",
			},
			notContains: []string{
				"Intraday series (3‑minute intervals",    // 旧格式
				"Longer‑term context (4‑hour timeframe)", // 旧格式
			},
		},
		{
			name: "使用TimeframeData-多个时间框架",
			input: &Data{
				Symbol:       "ETHUSDT",
				CurrentPrice: 3000.0,
				CurrentEMA20: 2950.0,
				CurrentMACD:  50.0,
				CurrentRSI7:  55.0,
				OpenInterest: &OIData{Latest: 500000.0, Average: 480000.0},
				FundingRate:  0.00005,
				TimeframeData: map[string]*TimeframeData{
					"3m": {
						Timeframe: "3m",
						MidPrices: []float64{2990.0, 3000.0},
						ATR14:     30.0,
					},
					"1h": {
						Timeframe: "1h",
						MidPrices: []float64{2900.0, 2950.0, 3000.0},
						ATR14:     50.0,
					},
					"4h": {
						Timeframe: "4h",
						MidPrices: []float64{2800.0, 2900.0, 3000.0},
						ATR14:     80.0,
					},
				},
			},
			contains: []string{
				"current_price = 3000.00",
				"ETHUSDT",
				"3-minute timeframe",
				"3m ATR",
				"1-hour timeframe",
				"1h ATR",
				"4-hour timeframe",
				"4h ATR",
			},
			notContains: []string{
				"Intraday series (3‑minute intervals",
			},
		},
		{
			name: "向后兼容-使用旧IntradaySeries",
			input: &Data{
				Symbol:       "BTCUSDT",
				CurrentPrice: 50000.0,
				CurrentEMA20: 49500.0,
				CurrentMACD:  100.0,
				CurrentRSI7:  65.0,
				FundingRate:  0.0001,
				IntradaySeries: &IntradayData{
					MidPrices:   []float64{49900.0, 50000.0},
					EMA20Values: []float64{49800.0, 49900.0},
					ATR14:       500.0,
				},
				LongerTermContext: &LongerTermData{
					EMA20: 49000.0,
					EMA50: 48500.0,
					ATR3:  200.0,
					ATR14: 600.0,
				},
			},
			contains: []string{
				"current_price = 50000.00",
				"Intraday series (3‑minute intervals",
				"3m ATR (14‑period): 500.000",
				"Longer‑term context (4‑hour timeframe)",
				"20‑Period EMA: 49000.000 vs. 50‑Period EMA: 48500.000",
			},
			notContains: []string{
				"3-minute timeframe", // 新格式
				"4-hour timeframe",   // 新格式
			},
		},
		{
			name: "TimeframeData优先-同时存在新旧字段",
			input: &Data{
				Symbol:       "BTCUSDT",
				CurrentPrice: 50000.0,
				CurrentEMA20: 49500.0,
				CurrentMACD:  100.0,
				CurrentRSI7:  65.0,
				FundingRate:  0.0001,
				// 新字段
				TimeframeData: map[string]*TimeframeData{
					"3m": {
						Timeframe: "3m",
						MidPrices: []float64{50000.0, 50100.0}, // 不同数据
						ATR14:     600.0,
					},
				},
				// 旧字段（应该被忽略）
				IntradaySeries: &IntradayData{
					MidPrices: []float64{49900.0, 50000.0},
					ATR14:     500.0,
				},
			},
			contains: []string{
				"3-minute timeframe",          // 新格式
				"3m ATR (14‑period): 600.000", // 新数据
				"50000.00, 50100.00",          // 新的价格数据
			},
			notContains: []string{
				"Intraday series (3‑minute intervals", // 旧格式应被忽略
				"49900.00",                            // 旧数据的价格应被忽略
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Format(tt.input)

			// 检查应该包含的字符串
			for _, str := range tt.contains {
				if !strings.Contains(result, str) {
					t.Errorf("输出应该包含 '%s', 但没有找到\n完整输出:\n%s", str, result)
				}
			}

			// 检查不应该包含的字符串
			for _, str := range tt.notContains {
				if strings.Contains(result, str) {
					t.Errorf("输出不应该包含 '%s', 但找到了\n完整输出:\n%s", str, result)
				}
			}
		})
	}
}

// TestFormat_TimeframeOrder 测试时间框架输出顺序
func TestFormat_TimeframeOrder(t *testing.T) {
	data := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 50000.0,
		CurrentEMA20: 49500.0,
		CurrentMACD:  100.0,
		CurrentRSI7:  65.0,
		FundingRate:  0.0001,
		TimeframeData: map[string]*TimeframeData{
			"1d":  {Timeframe: "1d", MidPrices: []float64{48000.0}, ATR14: 1000.0},
			"3m":  {Timeframe: "3m", MidPrices: []float64{50000.0}, ATR14: 500.0},
			"1h":  {Timeframe: "1h", MidPrices: []float64{49500.0}, ATR14: 700.0},
			"15m": {Timeframe: "15m", MidPrices: []float64{49800.0}, ATR14: 600.0},
		},
	}

	result := Format(data)

	// 查找各个时间框架在输出中的位置
	pos3m := strings.Index(result, "3-minute timeframe")
	pos15m := strings.Index(result, "15-minute timeframe")
	pos1h := strings.Index(result, "1-hour timeframe")
	pos1d := strings.Index(result, "1-day timeframe")

	// 验证所有时间框架都存在
	if pos3m == -1 || pos15m == -1 || pos1h == -1 || pos1d == -1 {
		t.Fatal("输出应该包含所有4个时间框架")
	}

	// 验证顺序: 3m < 15m < 1h < 1d
	if pos3m > pos15m {
		t.Errorf("3m应该在15m之前，但位置: 3m=%d, 15m=%d", pos3m, pos15m)
	}
	if pos15m > pos1h {
		t.Errorf("15m应该在1h之前，但位置: 15m=%d, 1h=%d", pos15m, pos1h)
	}
	if pos1h > pos1d {
		t.Errorf("1h应该在1d之前，但位置: 1h=%d, 1d=%d", pos1h, pos1d)
	}

	t.Logf("✅ 时间框架顺序正确: 3m(%d) < 15m(%d) < 1h(%d) < 1d(%d)", pos3m, pos15m, pos1h, pos1d)
}

// TestFormat_EmptyTimeframeData 测试空TimeframeData的处理
func TestFormat_EmptyTimeframeData(t *testing.T) {
	data := &Data{
		Symbol:        "BTCUSDT",
		CurrentPrice:  50000.0,
		CurrentEMA20:  49500.0,
		CurrentMACD:   100.0,
		CurrentRSI7:   65.0,
		FundingRate:   0.0001,
		TimeframeData: map[string]*TimeframeData{}, // 空map
	}

	result := Format(data)

	// 应该包含基础信息
	if !strings.Contains(result, "current_price = 50000.00") {
		t.Error("应该包含基础价格信息")
	}

	// 不应该包含任何时间框架数据
	timeframeKeywords := []string{
		"timeframe",
		"Mid prices:",
		"ATR",
	}
	for _, keyword := range timeframeKeywords {
		if strings.Contains(result, keyword) {
			t.Errorf("空TimeframeData不应该输出时间框架数据，但找到了: %s", keyword)
		}
	}
}

// BenchmarkFormat_SingleTimeframe 基准测试-单个时间框架
func BenchmarkFormat_SingleTimeframe(b *testing.B) {
	data := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 50000.0,
		CurrentEMA20: 49500.0,
		CurrentMACD:  100.0,
		CurrentRSI7:  65.0,
		FundingRate:  0.0001,
		TimeframeData: map[string]*TimeframeData{
			"3m": {
				Timeframe:      "3m",
				DataPoints:     40,
				MidPrices:      make([]float64, 40),
				EMA20Values:    make([]float64, 40),
				MACDValues:     make([]float64, 40),
				RSI7Values:     make([]float64, 40),
				RSI14Values:    make([]float64, 40),
				BollingerUpper: make([]float64, 40),
				BollingerMid:   make([]float64, 40),
				BollingerLower: make([]float64, 40),
				Volume:         make([]float64, 40),
				ATR14:          500.0,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Format(data)
	}
}

// BenchmarkFormat_MultipleTimeframes 基准测试-多个时间框架
func BenchmarkFormat_MultipleTimeframes(b *testing.B) {
	data := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 50000.0,
		CurrentEMA20: 49500.0,
		CurrentMACD:  100.0,
		CurrentRSI7:  65.0,
		FundingRate:  0.0001,
		TimeframeData: map[string]*TimeframeData{
			"3m": {Timeframe: "3m", MidPrices: make([]float64, 40), ATR14: 500.0},
			"1h": {Timeframe: "1h", MidPrices: make([]float64, 30), ATR14: 700.0},
			"4h": {Timeframe: "4h", MidPrices: make([]float64, 25), ATR14: 900.0},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Format(data)
	}
}

// BenchmarkFormat_AllTimeframes 基准测试-所有11个时间框架
func BenchmarkFormat_AllTimeframes(b *testing.B) {
	timeframes := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "12h", "1d"}
	tfData := make(map[string]*TimeframeData)
	for _, tf := range timeframes {
		tfData[tf] = &TimeframeData{
			Timeframe:   tf,
			MidPrices:   make([]float64, 30),
			EMA20Values: make([]float64, 30),
			ATR14:       500.0,
		}
	}

	data := &Data{
		Symbol:        "BTCUSDT",
		CurrentPrice:  50000.0,
		CurrentEMA20:  49500.0,
		CurrentMACD:   100.0,
		CurrentRSI7:   65.0,
		FundingRate:   0.0001,
		TimeframeData: tfData,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Format(data)
	}
}
