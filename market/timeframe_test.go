package market

import (
	"testing"
)

// ==================== 工具函数测试 ====================

func TestGetTimeframeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"1分钟", "1m", "1-minute"},
		{"3分钟", "3m", "3-minute"},
		{"5分钟", "5m", "5-minute"},
		{"15分钟", "15m", "15-minute"},
		{"30分钟", "30m", "30-minute"},
		{"1小时", "1h", "1-hour"},
		{"2小时", "2h", "2-hour"},
		{"4小时", "4h", "4-hour"},
		{"6小时", "6h", "6-hour"},
		{"12小时", "12h", "12-hour"},
		{"1天", "1d", "1-day"},
		{"未知时间框架", "invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTimeframeName(tt.input)
			if result != tt.expected {
				t.Errorf("GetTimeframeName(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetDefaultDataPoints(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"1分钟", "1m", 40},
		{"3分钟", "3m", 40},
		{"5分钟", "5m", 40},
		{"15分钟", "15m", 40},
		{"30分钟", "30m", 30},
		{"1小时", "1h", 30},
		{"2小时", "2h", 25},
		{"4小时", "4h", 25},
		{"6小时", "6h", 20},
		{"12小时", "12h", 20},
		{"1天", "1d", 15},
		{"未知时间框架", "invalid", 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDefaultDataPoints(tt.input)
			if result != tt.expected {
				t.Errorf("GetDefaultDataPoints(%s) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateTimeframe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"有效-1m", "1m", true},
		{"有效-3m", "3m", true},
		{"有效-5m", "5m", true},
		{"有效-15m", "15m", true},
		{"有效-30m", "30m", true},
		{"有效-1h", "1h", true},
		{"有效-2h", "2h", true},
		{"有效-4h", "4h", true},
		{"有效-6h", "6h", true},
		{"有效-12h", "12h", true},
		{"有效-1d", "1d", true},
		{"无效-2m", "2m", false},
		{"无效-8h", "8h", false},
		{"无效-1w", "1w", false},
		{"无效-空字符串", "", false},
		{"无效-random", "random", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateTimeframe(tt.input)
			if result != tt.expected {
				t.Errorf("ValidateTimeframe(%s) = %v; want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ==================== TimeframeData 计算测试 ====================

func TestCalculateTimeframeData_BasicFunctionality(t *testing.T) {
	// 生成测试数据
	klines := generateTestKlines(100)

	tests := []struct {
		name           string
		timeframe      string
		dataPoints     int
		expectedPoints int
	}{
		{
			name:           "正常情况-3m-40点",
			timeframe:      "3m",
			dataPoints:     40,
			expectedPoints: 40,
		},
		{
			name:           "正常情况-1h-30点",
			timeframe:      "1h",
			dataPoints:     30,
			expectedPoints: 30,
		},
		{
			name:           "数据点超过K线数量",
			timeframe:      "4h",
			dataPoints:     150,
			expectedPoints: 100, // 应该限制为实际K线数量
		},
		{
			name:           "数据点为0使用默认值",
			timeframe:      "3m",
			dataPoints:     0,
			expectedPoints: 40, // 3m默认40点
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTimeframeData(klines, tt.timeframe, tt.dataPoints)

			if result == nil {
				t.Fatal("calculateTimeframeData returned nil")
			}

			if result.DataPoints != tt.expectedPoints {
				t.Errorf("DataPoints = %d; want %d", result.DataPoints, tt.expectedPoints)
			}

			if result.Timeframe != tt.timeframe {
				t.Errorf("Timeframe = %s; want %s", result.Timeframe, tt.timeframe)
			}
		})
	}
}

func TestCalculateTimeframeData_EmptyKlines(t *testing.T) {
	result := calculateTimeframeData([]Kline{}, "3m", 40)

	if result != nil {
		t.Errorf("calculateTimeframeData with empty klines should return nil, got %v", result)
	}
}

func TestCalculateTimeframeData_IndicatorArrays(t *testing.T) {
	klines := generateTestKlines(100)
	result := calculateTimeframeData(klines, "3m", 40)

	if result == nil {
		t.Fatal("calculateTimeframeData returned nil")
	}

	// 验证所有数组长度一致
	expectedLen := 40

	tests := []struct {
		name   string
		length int
	}{
		{"MidPrices", len(result.MidPrices)},
		{"EMA20Values", len(result.EMA20Values)},
		{"MACDValues", len(result.MACDValues)},
		{"RSI7Values", len(result.RSI7Values)},
		{"RSI14Values", len(result.RSI14Values)},
		{"BollingerUpper", len(result.BollingerUpper)},
		{"BollingerMid", len(result.BollingerMid)},
		{"BollingerLower", len(result.BollingerLower)},
		{"Volume", len(result.Volume)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.length != expectedLen {
				t.Errorf("%s length = %d; want %d", tt.name, tt.length, expectedLen)
			}
		})
	}
}

func TestCalculateTimeframeData_MidPriceCalculation(t *testing.T) {
	// 创建简单的测试数据
	klines := []Kline{
		{High: 110.0, Low: 90.0, Close: 100.0},  // Mid = 100
		{High: 120.0, Low: 100.0, Close: 110.0}, // Mid = 110
		{High: 115.0, Low: 105.0, Close: 110.0}, // Mid = 110
	}

	result := calculateTimeframeData(klines, "3m", 3)

	if result == nil {
		t.Fatal("calculateTimeframeData returned nil")
	}

	expectedMidPrices := []float64{100.0, 110.0, 110.0}

	for i, expected := range expectedMidPrices {
		if result.MidPrices[i] != expected {
			t.Errorf("MidPrices[%d] = %f; want %f", i, result.MidPrices[i], expected)
		}
	}
}

func TestCalculateTimeframeData_ATR14(t *testing.T) {
	klines := generateTestKlines(50)
	result := calculateTimeframeData(klines, "3m", 30)

	if result == nil {
		t.Fatal("calculateTimeframeData returned nil")
	}

	// ATR14应该有值(只要K线数>=14)
	if result.ATR14 <= 0 {
		t.Errorf("ATR14 should be positive, got %f", result.ATR14)
	}
}

func TestCalculateTimeframeData_InsufficientDataForIndicators(t *testing.T) {
	// 只有5个K线,不足以计算某些指标
	klines := generateTestKlines(5)
	result := calculateTimeframeData(klines, "3m", 5)

	if result == nil {
		t.Fatal("calculateTimeframeData returned nil")
	}

	// EMA20需要20个数据点,前面的值应该为0
	if result.EMA20Values[0] != 0 {
		t.Errorf("EMA20Values[0] with insufficient data should be 0, got %f", result.EMA20Values[0])
	}

	// MACD需要34个数据点,所有值应该为0
	for i, val := range result.MACDValues {
		if val != 0 {
			t.Errorf("MACDValues[%d] with insufficient data should be 0, got %f", i, val)
		}
	}
}

// ==================== Data 结构测试 ====================

func TestTimeframeDataSerialization(t *testing.T) {
	klines := generateTestKlines(50)
	tfData := calculateTimeframeData(klines, "3m", 30)

	if tfData == nil {
		t.Fatal("calculateTimeframeData returned nil")
	}

	// 验证JSON序列化字段
	if tfData.Timeframe != "3m" {
		t.Errorf("Timeframe = %s; want 3m", tfData.Timeframe)
	}

	if tfData.DataPoints != 30 {
		t.Errorf("DataPoints = %d; want 30", tfData.DataPoints)
	}

	// 验证所有数组都有数据
	if len(tfData.MidPrices) == 0 {
		t.Error("MidPrices should not be empty")
	}
	if len(tfData.Volume) == 0 {
		t.Error("Volume should not be empty")
	}
}

func TestDataStructBackwardCompatibility(t *testing.T) {
	// 测试Data结构保持向后兼容
	data := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 47000.0,
		TimeframeData: map[string]*TimeframeData{
			"3m": {Timeframe: "3m", DataPoints: 40},
			"1h": {Timeframe: "1h", DataPoints: 30},
		},
		IntradaySeries:    &IntradayData{},
		LongerTermContext: &LongerTermData{},
	}

	// 验证新字段存在
	if data.TimeframeData == nil {
		t.Error("TimeframeData should not be nil")
	}

	if len(data.TimeframeData) != 2 {
		t.Errorf("TimeframeData should have 2 entries, got %d", len(data.TimeframeData))
	}

	// 验证旧字段仍然存在
	if data.IntradaySeries == nil {
		t.Error("IntradaySeries should not be nil (backward compatibility)")
	}

	if data.LongerTermContext == nil {
		t.Error("LongerTermContext should not be nil (backward compatibility)")
	}
}

// ==================== Benchmark 测试 ====================

func BenchmarkCalculateTimeframeData_40Points(b *testing.B) {
	klines := generateTestKlines(100)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		calculateTimeframeData(klines, "3m", 40)
	}
}

func BenchmarkCalculateTimeframeData_100Points(b *testing.B) {
	klines := generateTestKlines(200)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		calculateTimeframeData(klines, "3m", 100)
	}
}

func BenchmarkGetTimeframeName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetTimeframeName("3m")
	}
}

func BenchmarkValidateTimeframe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateTimeframe("3m")
	}
}
