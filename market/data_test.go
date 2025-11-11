package market

import (
	"math"
	"testing"
)

// generateTestKlines 生成测试用的 K线数据
func generateTestKlines(count int) []Kline {
	klines := make([]Kline, count)
	for i := 0; i < count; i++ {
		// 生成模拟的价格数据，有一定的波动
		basePrice := 100.0
		variance := float64(i%10) * 0.5
		open := basePrice + variance
		high := open + 1.0
		low := open - 0.5
		close := open + 0.3
		volume := 1000.0 + float64(i*100)

		klines[i] = Kline{
			OpenTime:  int64(i * 180000), // 3分钟间隔
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: int64((i+1)*180000 - 1),
		}
	}
	return klines
}

// TestCalculateIntradaySeries_VolumeCollection 测试 Volume 数据收集
func TestCalculateIntradaySeries_VolumeCollection(t *testing.T) {
	tests := []struct {
		name           string
		klineCount     int
		expectedVolLen int
	}{
		{
			name:           "正常情况 - 20个K线",
			klineCount:     20,
			expectedVolLen: 10, // 应该收集最近10个
		},
		{
			name:           "刚好10个K线",
			klineCount:     10,
			expectedVolLen: 10,
		},
		{
			name:           "少于10个K线",
			klineCount:     5,
			expectedVolLen: 5, // 应该返回所有5个
		},
		{
			name:           "超过10个K线",
			klineCount:     30,
			expectedVolLen: 10, // 应该只返回最近10个
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines := generateTestKlines(tt.klineCount)
			data := calculateIntradaySeries(klines)

			if data == nil {
				t.Fatal("calculateIntradaySeries returned nil")
			}

			if len(data.Volume) != tt.expectedVolLen {
				t.Errorf("Volume length = %d, want %d", len(data.Volume), tt.expectedVolLen)
			}

			// 验证 Volume 数据正确性
			if len(data.Volume) > 0 {
				// 计算期望的起始索引
				start := tt.klineCount - 10
				if start < 0 {
					start = 0
				}

				// 验证第一个 Volume 值
				expectedFirstVolume := klines[start].Volume
				if data.Volume[0] != expectedFirstVolume {
					t.Errorf("First volume = %.2f, want %.2f", data.Volume[0], expectedFirstVolume)
				}

				// 验证最后一个 Volume 值
				expectedLastVolume := klines[tt.klineCount-1].Volume
				lastVolume := data.Volume[len(data.Volume)-1]
				if lastVolume != expectedLastVolume {
					t.Errorf("Last volume = %.2f, want %.2f", lastVolume, expectedLastVolume)
				}
			}
		})
	}
}

// TestCalculateIntradaySeries_VolumeValues 测试 Volume 值的正确性
func TestCalculateIntradaySeries_VolumeValues(t *testing.T) {
	klines := []Kline{
		{Close: 100.0, Volume: 1000.0, High: 101.0, Low: 99.0, Open: 100.0},
		{Close: 101.0, Volume: 1100.0, High: 102.0, Low: 100.0, Open: 101.0},
		{Close: 102.0, Volume: 1200.0, High: 103.0, Low: 101.0, Open: 102.0},
		{Close: 103.0, Volume: 1300.0, High: 104.0, Low: 102.0, Open: 103.0},
		{Close: 104.0, Volume: 1400.0, High: 105.0, Low: 103.0, Open: 104.0},
		{Close: 105.0, Volume: 1500.0, High: 106.0, Low: 104.0, Open: 105.0},
		{Close: 106.0, Volume: 1600.0, High: 107.0, Low: 105.0, Open: 106.0},
		{Close: 107.0, Volume: 1700.0, High: 108.0, Low: 106.0, Open: 107.0},
		{Close: 108.0, Volume: 1800.0, High: 109.0, Low: 107.0, Open: 108.0},
		{Close: 109.0, Volume: 1900.0, High: 110.0, Low: 108.0, Open: 109.0},
	}

	data := calculateIntradaySeries(klines)

	expectedVolumes := []float64{1000.0, 1100.0, 1200.0, 1300.0, 1400.0, 1500.0, 1600.0, 1700.0, 1800.0, 1900.0}

	if len(data.Volume) != len(expectedVolumes) {
		t.Fatalf("Volume length = %d, want %d", len(data.Volume), len(expectedVolumes))
	}

	for i, expected := range expectedVolumes {
		if data.Volume[i] != expected {
			t.Errorf("Volume[%d] = %.2f, want %.2f", i, data.Volume[i], expected)
		}
	}
}

// TestCalculateIntradaySeries_ATR14 测试 ATR14 计算
func TestCalculateIntradaySeries_ATR14(t *testing.T) {
	tests := []struct {
		name         string
		klineCount   int
		expectZero   bool
		expectNonZero bool
	}{
		{
			name:         "足够数据 - 20个K线",
			klineCount:   20,
			expectNonZero: true,
		},
		{
			name:         "刚好15个K线（ATR14需要至少15个）",
			klineCount:   15,
			expectNonZero: true,
		},
		{
			name:       "数据不足 - 14个K线",
			klineCount: 14,
			expectZero: true,
		},
		{
			name:       "数据不足 - 10个K线",
			klineCount: 10,
			expectZero: true,
		},
		{
			name:       "数据不足 - 5个K线",
			klineCount: 5,
			expectZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines := generateTestKlines(tt.klineCount)
			data := calculateIntradaySeries(klines)

			if data == nil {
				t.Fatal("calculateIntradaySeries returned nil")
			}

			if tt.expectZero && data.ATR14 != 0 {
				t.Errorf("ATR14 = %.3f, expected 0 (insufficient data)", data.ATR14)
			}

			if tt.expectNonZero && data.ATR14 <= 0 {
				t.Errorf("ATR14 = %.3f, expected > 0", data.ATR14)
			}
		})
	}
}

// TestCalculateATR 测试 ATR 计算函数
func TestCalculateATR(t *testing.T) {
	tests := []struct {
		name       string
		klines     []Kline
		period     int
		expectZero bool
	}{
		{
			name: "正常计算 - 足够数据",
			klines: []Kline{
				{High: 102.0, Low: 100.0, Close: 101.0},
				{High: 103.0, Low: 101.0, Close: 102.0},
				{High: 104.0, Low: 102.0, Close: 103.0},
				{High: 105.0, Low: 103.0, Close: 104.0},
				{High: 106.0, Low: 104.0, Close: 105.0},
				{High: 107.0, Low: 105.0, Close: 106.0},
				{High: 108.0, Low: 106.0, Close: 107.0},
				{High: 109.0, Low: 107.0, Close: 108.0},
				{High: 110.0, Low: 108.0, Close: 109.0},
				{High: 111.0, Low: 109.0, Close: 110.0},
				{High: 112.0, Low: 110.0, Close: 111.0},
				{High: 113.0, Low: 111.0, Close: 112.0},
				{High: 114.0, Low: 112.0, Close: 113.0},
				{High: 115.0, Low: 113.0, Close: 114.0},
				{High: 116.0, Low: 114.0, Close: 115.0},
			},
			period:     14,
			expectZero: false,
		},
		{
			name: "数据不足 - 等于period",
			klines: []Kline{
				{High: 102.0, Low: 100.0, Close: 101.0},
				{High: 103.0, Low: 101.0, Close: 102.0},
			},
			period:     2,
			expectZero: true,
		},
		{
			name: "数据不足 - 少于period",
			klines: []Kline{
				{High: 102.0, Low: 100.0, Close: 101.0},
			},
			period:     14,
			expectZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atr := calculateATR(tt.klines, tt.period)

			if tt.expectZero {
				if atr != 0 {
					t.Errorf("calculateATR() = %.3f, expected 0 (insufficient data)", atr)
				}
			} else {
				if atr <= 0 {
					t.Errorf("calculateATR() = %.3f, expected > 0", atr)
				}
			}
		})
	}
}

// TestCalculateATR_TrueRange 测试 ATR 的 True Range 计算正确性
func TestCalculateATR_TrueRange(t *testing.T) {
	// 创建一个简单的测试用例，手动计算期望的 ATR
	klines := []Kline{
		{High: 50.0, Low: 48.0, Close: 49.0},  // TR = 2.0
		{High: 51.0, Low: 49.0, Close: 50.0},  // TR = max(2.0, 2.0, 1.0) = 2.0
		{High: 52.0, Low: 50.0, Close: 51.0},  // TR = max(2.0, 2.0, 1.0) = 2.0
		{High: 53.0, Low: 51.0, Close: 52.0},  // TR = 2.0
		{High: 54.0, Low: 52.0, Close: 53.0},  // TR = 2.0
	}

	atr := calculateATR(klines, 3)

	// 期望的计算：
	// TR[1] = max(51-49, |51-49|, |49-49|) = 2.0
	// TR[2] = max(52-50, |52-50|, |50-50|) = 2.0
	// TR[3] = max(53-51, |53-51|, |51-51|) = 2.0
	// 初始 ATR = (2.0 + 2.0 + 2.0) / 3 = 2.0
	// TR[4] = max(54-52, |54-52|, |52-52|) = 2.0
	// 平滑 ATR = (2.0*2 + 2.0) / 3 = 2.0

	expectedATR := 2.0
	tolerance := 0.01 // 允许小的浮点误差

	if math.Abs(atr-expectedATR) > tolerance {
		t.Errorf("calculateATR() = %.3f, want approximately %.3f", atr, expectedATR)
	}
}

// TestCalculateIntradaySeries_ConsistencyWithOtherIndicators 测试 Volume 和其他指标的一致性
func TestCalculateIntradaySeries_ConsistencyWithOtherIndicators(t *testing.T) {
	klines := generateTestKlines(30)
	data := calculateIntradaySeries(klines)

	// 所有数组应该存在
	if data.MidPrices == nil {
		t.Error("MidPrices should not be nil")
	}
	if data.Volume == nil {
		t.Error("Volume should not be nil")
	}

	// MidPrices 和 Volume 应该有相同的长度（都是最近10个）
	if len(data.MidPrices) != len(data.Volume) {
		t.Errorf("MidPrices length (%d) should equal Volume length (%d)",
			len(data.MidPrices), len(data.Volume))
	}

	// 所有 Volume 值应该大于 0
	for i, vol := range data.Volume {
		if vol <= 0 {
			t.Errorf("Volume[%d] = %.2f, should be > 0", i, vol)
		}
	}
}

// TestCalculateIntradaySeries_EmptyKlines 测试空 K线数据
func TestCalculateIntradaySeries_EmptyKlines(t *testing.T) {
	klines := []Kline{}
	data := calculateIntradaySeries(klines)

	if data == nil {
		t.Fatal("calculateIntradaySeries should not return nil for empty klines")
	}

	// 所有切片应该为空
	if len(data.MidPrices) != 0 {
		t.Errorf("MidPrices length = %d, want 0", len(data.MidPrices))
	}
	if len(data.Volume) != 0 {
		t.Errorf("Volume length = %d, want 0", len(data.Volume))
	}

	// ATR14 应该为 0（数据不足）
	if data.ATR14 != 0 {
		t.Errorf("ATR14 = %.3f, want 0", data.ATR14)
	}
}

// TestCalculateIntradaySeries_VolumePrecision 测试 Volume 精度保持
func TestCalculateIntradaySeries_VolumePrecision(t *testing.T) {
	klines := []Kline{
		{Close: 100.0, Volume: 1234.5678, High: 101.0, Low: 99.0},
		{Close: 101.0, Volume: 9876.5432, High: 102.0, Low: 100.0},
		{Close: 102.0, Volume: 5555.1111, High: 103.0, Low: 101.0},
	}

	data := calculateIntradaySeries(klines)

	expectedVolumes := []float64{1234.5678, 9876.5432, 5555.1111}

	for i, expected := range expectedVolumes {
		if data.Volume[i] != expected {
			t.Errorf("Volume[%d] = %.4f, want %.4f (precision not preserved)",
				i, data.Volume[i], expected)
		}
	}
}
