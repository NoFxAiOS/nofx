package market

import (
	"nofx/store"
	"testing"
)

// TestConfigurableEMA tests EMA with custom periods
func TestConfigurableEMA(t *testing.T) {
	// Generate test data
	klines := generateTestKlines(100)

	tests := []struct {
		name       string
		config     *store.IndicatorConfig
		expectEMA1 bool
		expectEMA2 bool
	}{
		{
			name: "Custom EMA periods 30 and 100",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{30, 100},
				RSIPeriods:     []int{7, 14},
				ATRPeriods:     []int{14},
				MACDFastPeriod: 12,
				MACDSlowPeriod: 26,
			},
			expectEMA1: true,
			expectEMA2: true,
		},
		{
			name: "Single EMA period",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{50},
				RSIPeriods:     []int{7, 14},
				ATRPeriods:     []int{14},
				MACDFastPeriod: 12,
				MACDSlowPeriod: 26,
			},
			expectEMA1: true,
			expectEMA2: false,
		},
		{
			name:       "Nil config (use defaults)",
			config:     nil,
			expectEMA1: true,
			expectEMA2: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := calculateTimeframeSeries(klines, "5m", 10, tt.config)

			if data == nil {
				t.Fatal("calculateTimeframeSeries returned nil")
			}

			if tt.expectEMA1 && len(data.EMA20Values) == 0 {
				t.Error("Expected EMA1 values but got none")
			}
			if !tt.expectEMA1 && len(data.EMA20Values) > 0 {
				t.Error("Expected no EMA1 values but got some")
			}

			if tt.expectEMA2 && len(data.EMA50Values) == 0 {
				t.Error("Expected EMA2 values but got none")
			}
			if !tt.expectEMA2 && len(data.EMA50Values) > 0 {
				t.Error("Expected no EMA2 values but got some")
			}
		})
	}
}

// TestConfigurableRSI tests RSI with custom periods
func TestConfigurableRSI(t *testing.T) {
	// Generate test data
	klines := generateTestKlines(100)

	tests := []struct {
		name       string
		config     *store.IndicatorConfig
		expectRSI1 bool
		expectRSI2 bool
	}{
		{
			name: "Custom RSI periods 10 and 20",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{20, 50},
				RSIPeriods:     []int{10, 20},
				ATRPeriods:     []int{14},
				MACDFastPeriod: 12,
				MACDSlowPeriod: 26,
			},
			expectRSI1: true,
			expectRSI2: true,
		},
		{
			name: "Single RSI period",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{20, 50},
				RSIPeriods:     []int{14},
				ATRPeriods:     []int{14},
				MACDFastPeriod: 12,
				MACDSlowPeriod: 26,
			},
			expectRSI1: true,
			expectRSI2: false,
		},
		{
			name:       "Nil config (use defaults)",
			config:     nil,
			expectRSI1: true,
			expectRSI2: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := calculateTimeframeSeries(klines, "5m", 10, tt.config)

			if data == nil {
				t.Fatal("calculateTimeframeSeries returned nil")
			}

			if tt.expectRSI1 && len(data.RSI7Values) == 0 {
				t.Error("Expected RSI1 values but got none")
			}
			if !tt.expectRSI1 && len(data.RSI7Values) > 0 {
				t.Error("Expected no RSI1 values but got some")
			}

			if tt.expectRSI2 && len(data.RSI14Values) == 0 {
				t.Error("Expected RSI2 values but got none")
			}
			if !tt.expectRSI2 && len(data.RSI14Values) > 0 {
				t.Error("Expected no RSI2 values but got some")
			}
		})
	}
}

// TestConfigurableMACD tests MACD with custom periods
func TestConfigurableMACD(t *testing.T) {
	// Generate test data with increasing prices for better MACD calculation
	klines := make([]Kline, 100)
	baseTime := int64(1609459200000) // 2021-01-01
	for i := range klines {
		klines[i] = Kline{
			OpenTime:  baseTime + int64(i)*60000,
			Open:      100.0 + float64(i)*0.5,
			High:      101.0 + float64(i)*0.5,
			Low:       99.0 + float64(i)*0.5,
			Close:     100.0 + float64(i)*0.5,
			Volume:    1000.0,
			CloseTime: baseTime + int64(i+1)*60000,
		}
	}

	tests := []struct {
		name          string
		config        *store.IndicatorConfig
		expectValues  bool
		minValueCount int
	}{
		{
			name: "Custom MACD periods 8 and 21",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{20, 50},
				RSIPeriods:     []int{7, 14},
				ATRPeriods:     []int{14},
				MACDFastPeriod: 8,
				MACDSlowPeriod: 21,
			},
			expectValues:  true,
			minValueCount: 1,
		},
		{
			name: "Default MACD periods 12 and 26",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{20, 50},
				RSIPeriods:     []int{7, 14},
				ATRPeriods:     []int{14},
				MACDFastPeriod: 12,
				MACDSlowPeriod: 26,
			},
			expectValues:  true,
			minValueCount: 1,
		},
		{
			name:          "Nil config (use defaults)",
			config:        nil,
			expectValues:  true,
			minValueCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := calculateTimeframeSeries(klines, "5m", 10, tt.config)

			if data == nil {
				t.Fatal("calculateTimeframeSeries returned nil")
			}

			if tt.expectValues && len(data.MACDValues) < tt.minValueCount {
				t.Errorf("Expected at least %d MACD values but got %d", tt.minValueCount, len(data.MACDValues))
			}
		})
	}
}

// TestConfigurableATR tests ATR with custom periods
func TestConfigurableATR(t *testing.T) {
	// Generate test data with volatility
	klines := make([]Kline, 100)
	baseTime := int64(1609459200000) // 2021-01-01
	for i := range klines {
		klines[i] = Kline{
			OpenTime:  baseTime + int64(i)*60000,
			Open:      100.0,
			High:      105.0,
			Low:       95.0,
			Close:     100.0,
			Volume:    1000.0,
			CloseTime: baseTime + int64(i+1)*60000,
		}
	}

	tests := []struct {
		name      string
		config    *store.IndicatorConfig
		expectATR bool
	}{
		{
			name: "Custom ATR period 7",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{20, 50},
				RSIPeriods:     []int{7, 14},
				ATRPeriods:     []int{7},
				MACDFastPeriod: 12,
				MACDSlowPeriod: 26,
			},
			expectATR: true,
		},
		{
			name: "Custom ATR period 21",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{20, 50},
				RSIPeriods:     []int{7, 14},
				ATRPeriods:     []int{21},
				MACDFastPeriod: 12,
				MACDSlowPeriod: 26,
			},
			expectATR: true,
		},
		{
			name:      "Nil config (use defaults)",
			config:    nil,
			expectATR: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := calculateTimeframeSeries(klines, "5m", 10, tt.config)

			if data == nil {
				t.Fatal("calculateTimeframeSeries returned nil")
			}

			if tt.expectATR && data.ATR14 == 0 {
				t.Error("Expected ATR value but got 0")
			}
		})
	}
}

// TestCalculateMACDWithPeriods tests the calculateMACD function directly with custom periods
func TestCalculateMACDWithPeriods(t *testing.T) {
	// Generate test data
	klines := make([]Kline, 100)
	baseTime := int64(1609459200000)
	for i := range klines {
		klines[i] = Kline{
			OpenTime:  baseTime + int64(i)*60000,
			Open:      100.0 + float64(i)*0.1,
			High:      101.0 + float64(i)*0.1,
			Low:       99.0 + float64(i)*0.1,
			Close:     100.0 + float64(i)*0.1,
			Volume:    1000.0,
			CloseTime: baseTime + int64(i+1)*60000,
		}
	}

	tests := []struct {
		name       string
		fastPeriod int
		slowPeriod int
		expectZero bool
	}{
		{
			name:       "Standard MACD (12, 26)",
			fastPeriod: 12,
			slowPeriod: 26,
			expectZero: false,
		},
		{
			name:       "Custom MACD (8, 21)",
			fastPeriod: 8,
			slowPeriod: 21,
			expectZero: false,
		},
		{
			name:       "Default periods (0, 0)",
			fastPeriod: 0,
			slowPeriod: 0,
			expectZero: false,
		},
		{
			name:       "Invalid periods (fast >= slow)",
			fastPeriod: 26,
			slowPeriod: 12,
			expectZero: false, // Should use defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			macd := calculateMACD(klines, tt.fastPeriod, tt.slowPeriod)

			if tt.expectZero && macd != 0 {
				t.Errorf("Expected MACD = 0, got %f", macd)
			}
			if !tt.expectZero && macd == 0 {
				t.Errorf("Expected non-zero MACD, got 0")
			}
		})
	}
}

// TestIntradaySeriesConfigurable tests calculateIntradaySeriesWithCount with custom config
func TestIntradaySeriesConfigurable(t *testing.T) {
	klines := generateTestKlines(100)

	tests := []struct {
		name   string
		config *store.IndicatorConfig
	}{
		{
			name: "Custom indicators",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{30},
				RSIPeriods:     []int{10, 25},
				ATRPeriods:     []int{21},
				MACDFastPeriod: 8,
				MACDSlowPeriod: 21,
			},
		},
		{
			name:   "Nil config (defaults)",
			config: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := calculateIntradaySeriesWithCount(klines, 10, tt.config)

			if data == nil {
				t.Fatal("calculateIntradaySeriesWithCount returned nil")
			}

			if data.Count != 10 {
				t.Errorf("Expected count = 10, got %d", data.Count)
			}

			if data.ATR14 == 0 {
				t.Error("Expected non-zero ATR")
			}
		})
	}
}

// TestLongerTermDataConfigurable tests calculateLongerTermData with custom config
func TestLongerTermDataConfigurable(t *testing.T) {
	klines := generateTestKlines(100)

	tests := []struct {
		name   string
		config *store.IndicatorConfig
	}{
		{
			name: "Custom indicators",
			config: &store.IndicatorConfig{
				EMAPeriods:     []int{30, 100},
				RSIPeriods:     []int{21},
				ATRPeriods:     []int{7, 21},
				MACDFastPeriod: 8,
				MACDSlowPeriod: 21,
			},
		},
		{
			name:   "Nil config (defaults)",
			config: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := calculateLongerTermData(klines, tt.config)

			if data == nil {
				t.Fatal("calculateLongerTermData returned nil")
			}

			if data.EMA20 == 0 {
				t.Error("Expected non-zero EMA20")
			}

			if data.ATR14 == 0 {
				t.Error("Expected non-zero ATR14")
			}
		})
	}
}
