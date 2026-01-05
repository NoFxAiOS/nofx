package market

import (
	"math"
	"testing"
)

// generateTestKlines generates test K-line data
func generateTestKlines(count int) []Kline {
	klines := make([]Kline, count)
	for i := 0; i < count; i++ {
		// Generate simulated price data with some fluctuation
		basePrice := 100.0
		variance := float64(i%10) * 0.5
		open := basePrice + variance
		high := open + 1.0
		low := open - 0.5
		close := open + 0.3
		volume := 1000.0 + float64(i*100)

		klines[i] = Kline{
			OpenTime:  int64(i * 180000), // 3-minute interval
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

// TestCalculateIntradaySeries_VolumeCollection tests Volume data collection
func TestCalculateIntradaySeries_VolumeCollection(t *testing.T) {
	tests := []struct {
		name           string
		klineCount     int
		expectedVolLen int
	}{
		{
			name:           "Normal case - 20 K-lines",
			klineCount:     20,
			expectedVolLen: 10, // Should collect latest 10
		},
		{
			name:           "Exactly 10 K-lines",
			klineCount:     10,
			expectedVolLen: 10,
		},
		{
			name:           "Less than 10 K-lines",
			klineCount:     5,
			expectedVolLen: 5, // Should return all 5
		},
		{
			name:           "More than 10 K-lines",
			klineCount:     30,
			expectedVolLen: 10, // Should only return latest 10
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

			// Verify Volume data correctness
			if len(data.Volume) > 0 {
				// Calculate expected start index
				start := tt.klineCount - 10
				if start < 0 {
					start = 0
				}

				// Verify first Volume value
				expectedFirstVolume := klines[start].Volume
				if data.Volume[0] != expectedFirstVolume {
					t.Errorf("First volume = %.2f, want %.2f", data.Volume[0], expectedFirstVolume)
				}

				// Verify last Volume value
				expectedLastVolume := klines[tt.klineCount-1].Volume
				lastVolume := data.Volume[len(data.Volume)-1]
				if lastVolume != expectedLastVolume {
					t.Errorf("Last volume = %.2f, want %.2f", lastVolume, expectedLastVolume)
				}
			}
		})
	}
}

// TestCalculateIntradaySeries_VolumeValues tests Volume value correctness
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

// TestCalculateIntradaySeries_ATR14 tests ATR14 calculation
func TestCalculateIntradaySeries_ATR14(t *testing.T) {
	tests := []struct {
		name          string
		klineCount    int
		expectZero    bool
		expectNonZero bool
	}{
		{
			name:          "Sufficient data - 20 K-lines",
			klineCount:    20,
			expectNonZero: true,
		},
		{
			name:          "Exactly 15 K-lines (ATR14 requires at least 15)",
			klineCount:    15,
			expectNonZero: true,
		},
		{
			name:       "Insufficient data - 14 K-lines",
			klineCount: 14,
			expectZero: true,
		},
		{
			name:       "Insufficient data - 10 K-lines",
			klineCount: 10,
			expectZero: true,
		},
		{
			name:       "Insufficient data - 5 K-lines",
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

// TestCalculateATR tests ATR calculation function
func TestCalculateATR(t *testing.T) {
	tests := []struct {
		name       string
		klines     []Kline
		period     int
		expectZero bool
	}{
		{
			name: "Normal calculation - sufficient data",
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
			name: "Insufficient data - equal to period",
			klines: []Kline{
				{High: 102.0, Low: 100.0, Close: 101.0},
				{High: 103.0, Low: 101.0, Close: 102.0},
			},
			period:     2,
			expectZero: true,
		},
		{
			name: "Insufficient data - less than period",
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

// TestCalculateATR_TrueRange tests ATR True Range calculation correctness
func TestCalculateATR_TrueRange(t *testing.T) {
	// Create a simple test case, manually calculate expected ATR
	klines := []Kline{
		{High: 50.0, Low: 48.0, Close: 49.0}, // TR = 2.0
		{High: 51.0, Low: 49.0, Close: 50.0}, // TR = max(2.0, 2.0, 1.0) = 2.0
		{High: 52.0, Low: 50.0, Close: 51.0}, // TR = max(2.0, 2.0, 1.0) = 2.0
		{High: 53.0, Low: 51.0, Close: 52.0}, // TR = 2.0
		{High: 54.0, Low: 52.0, Close: 53.0}, // TR = 2.0
	}

	atr := calculateATR(klines, 3)

	// Expected calculation:
	// TR[1] = max(51-49, |51-49|, |49-49|) = 2.0
	// TR[2] = max(52-50, |52-50|, |50-50|) = 2.0
	// TR[3] = max(53-51, |53-51|, |51-51|) = 2.0
	// Initial ATR = (2.0 + 2.0 + 2.0) / 3 = 2.0
	// TR[4] = max(54-52, |54-52|, |52-52|) = 2.0
	// Smoothed ATR = (2.0*2 + 2.0) / 3 = 2.0

	expectedATR := 2.0
	tolerance := 0.01 // Allow small floating point error

	if math.Abs(atr-expectedATR) > tolerance {
		t.Errorf("calculateATR() = %.3f, want approximately %.3f", atr, expectedATR)
	}
}

// TestCalculateIntradaySeries_ConsistencyWithOtherIndicators tests Volume and other indicators consistency
func TestCalculateIntradaySeries_ConsistencyWithOtherIndicators(t *testing.T) {
	klines := generateTestKlines(30)
	data := calculateIntradaySeries(klines)

	// All arrays should exist
	if data.MidPrices == nil {
		t.Error("MidPrices should not be nil")
	}
	if data.Volume == nil {
		t.Error("Volume should not be nil")
	}

	// MidPrices and Volume should have the same length (both latest 10)
	if len(data.MidPrices) != len(data.Volume) {
		t.Errorf("MidPrices length (%d) should equal Volume length (%d)",
			len(data.MidPrices), len(data.Volume))
	}

	// All Volume values should be > 0
	for i, vol := range data.Volume {
		if vol <= 0 {
			t.Errorf("Volume[%d] = %.2f, should be > 0", i, vol)
		}
	}
}

// TestCalculateIntradaySeries_EmptyKlines tests empty K-line data
func TestCalculateIntradaySeries_EmptyKlines(t *testing.T) {
	klines := []Kline{}
	data := calculateIntradaySeries(klines)

	if data == nil {
		t.Fatal("calculateIntradaySeries should not return nil for empty klines")
	}

	// All slices should be empty
	if len(data.MidPrices) != 0 {
		t.Errorf("MidPrices length = %d, want 0", len(data.MidPrices))
	}
	if len(data.Volume) != 0 {
		t.Errorf("Volume length = %d, want 0", len(data.Volume))
	}

	// ATR14 should be 0 (insufficient data)
	if data.ATR14 != 0 {
		t.Errorf("ATR14 = %.3f, want 0", data.ATR14)
	}
}

// TestCalculateIntradaySeries_VolumePrecision tests Volume precision preservation
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

// TestIsStaleData_NormalData tests that normal fluctuating data returns false
func TestIsStaleData_NormalData(t *testing.T) {
	klines := []Kline{
		{Close: 100.0, Volume: 1000},
		{Close: 100.5, Volume: 1200},
		{Close: 99.8, Volume: 900},
		{Close: 100.2, Volume: 1100},
		{Close: 100.1, Volume: 950},
	}

	result := isStaleData(klines, "BTCUSDT")

	if result {
		t.Error("Expected false for normal fluctuating data, got true")
	}
}

// TestIsStaleData_PriceFreezeWithZeroVolume tests that frozen price + zero volume returns true
func TestIsStaleData_PriceFreezeWithZeroVolume(t *testing.T) {
	klines := []Kline{
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
	}

	result := isStaleData(klines, "DOGEUSDT")

	if !result {
		t.Error("Expected true for frozen price + zero volume, got false")
	}
}

// TestIsStaleData_PriceFreezeWithVolume tests that frozen price but normal volume returns false
func TestIsStaleData_PriceFreezeWithVolume(t *testing.T) {
	klines := []Kline{
		{Close: 100.0, Volume: 1000},
		{Close: 100.0, Volume: 1200},
		{Close: 100.0, Volume: 900},
		{Close: 100.0, Volume: 1100},
		{Close: 100.0, Volume: 950},
	}

	result := isStaleData(klines, "STABLECOIN")

	if result {
		t.Error("Expected false for frozen price but normal volume (low volatility market), got true")
	}
}

// TestIsStaleData_InsufficientData tests that insufficient data (<5 klines) returns false
func TestIsStaleData_InsufficientData(t *testing.T) {
	klines := []Kline{
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
	}

	result := isStaleData(klines, "BTCUSDT")

	if result {
		t.Error("Expected false for insufficient data (<5 klines), got true")
	}
}

// TestIsStaleData_ExactlyFiveKlines tests edge case with exactly 5 klines
func TestIsStaleData_ExactlyFiveKlines(t *testing.T) {
	// Stale case: exactly 5 frozen klines with zero volume
	staleKlines := []Kline{
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
		{Close: 100.0, Volume: 0},
	}

	result := isStaleData(staleKlines, "TESTUSDT")
	if !result {
		t.Error("Expected true for exactly 5 frozen klines with zero volume, got false")
	}

	// Normal case: exactly 5 klines with fluctuation
	normalKlines := []Kline{
		{Close: 100.0, Volume: 1000},
		{Close: 100.1, Volume: 1100},
		{Close: 99.9, Volume: 900},
		{Close: 100.0, Volume: 1000},
		{Close: 100.05, Volume: 950},
	}

	result = isStaleData(normalKlines, "TESTUSDT")
	if result {
		t.Error("Expected false for exactly 5 normal klines, got true")
	}
}

// TestIsStaleData_WithinTolerance tests price changes within tolerance (0.01%)
func TestIsStaleData_WithinTolerance(t *testing.T) {
	// Price changes within 0.01% tolerance should be treated as frozen
	basePrice := 10000.0
	tolerance := 0.0001                        // 0.01%
	smallChange := basePrice * tolerance * 0.5 // Half of tolerance

	klines := []Kline{
		{Close: basePrice, Volume: 1000},
		{Close: basePrice + smallChange, Volume: 1000},
		{Close: basePrice - smallChange, Volume: 1000},
		{Close: basePrice, Volume: 1000},
		{Close: basePrice + smallChange, Volume: 1000},
	}

	result := isStaleData(klines, "BTCUSDT")

	// Should return false because there's normal volume despite tiny price changes
	if result {
		t.Error("Expected false for price within tolerance but with volume, got true")
	}
}

// TestIsStaleData_MixedScenario tests realistic scenario with some history before freeze
func TestIsStaleData_MixedScenario(t *testing.T) {
	// Simulate: normal trading → suddenly freezes
	klines := []Kline{
		{Close: 100.0, Volume: 1000}, // Normal
		{Close: 100.5, Volume: 1200}, // Normal
		{Close: 100.2, Volume: 1100}, // Normal
		{Close: 50.0, Volume: 0},     // Freeze starts
		{Close: 50.0, Volume: 0},     // Frozen
		{Close: 50.0, Volume: 0},     // Frozen
		{Close: 50.0, Volume: 0},     // Frozen
		{Close: 50.0, Volume: 0},     // Frozen (last 5 are all frozen)
	}

	result := isStaleData(klines, "DOGEUSDT")

	// Should detect stale data based on last 5 klines
	if !result {
		t.Error("Expected true for frozen last 5 klines with zero volume, got false")
	}
}

// TestIsStaleData_EmptyKlines tests edge case with empty slice
func TestIsStaleData_EmptyKlines(t *testing.T) {
	klines := []Kline{}

	result := isStaleData(klines, "BTCUSDT")

	if result {
		t.Error("Expected false for empty klines, got true")
	}
}

// TestCalculateIntradaySeriesWithCount tests configurable K-line count functionality
func TestCalculateIntradaySeriesWithCount(t *testing.T) {
	tests := []struct {
		name           string
		klineCount     int
		requestedCount int
		expectedLen    int
	}{
		{
			name:           "Request 5 from 20 K-lines",
			klineCount:     20,
			requestedCount: 5,
			expectedLen:    5,
		},
		{
			name:           "Request 10 from 30 K-lines",
			klineCount:     30,
			requestedCount: 10,
			expectedLen:    10,
		},
		{
			name:           "Request 30 from 30 K-lines",
			klineCount:     30,
			requestedCount: 30,
			expectedLen:    30,
		},
		{
			name:           "Request more than available",
			klineCount:     15,
			requestedCount: 30,
			expectedLen:    15, // Should return all available
		},
		{
			name:           "Request 0 count",
			klineCount:     20,
			requestedCount: 0,
			expectedLen:    0,
		},
		{
			name:           "Request negative count (use default 10)",
			klineCount:     20,
			requestedCount: -5,
			expectedLen:    10, // Should default to 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines := generateTestKlines(tt.klineCount)
			data := calculateIntradaySeriesWithCount(klines, tt.requestedCount, nil)

			if data == nil {
				t.Fatal("calculateIntradaySeriesWithCount returned nil")
			}

			if len(data.Volume) != tt.expectedLen {
				t.Errorf("Volume length = %d, want %d", len(data.Volume), tt.expectedLen)
			}

			if len(data.MidPrices) != tt.expectedLen {
				t.Errorf("MidPrices length = %d, want %d", len(data.MidPrices), tt.expectedLen)
			}

			// Verify Count field matches actual data length
			if data.Count != tt.expectedLen {
				t.Errorf("Count = %d, want %d", data.Count, tt.expectedLen)
			}

			if data.Count != len(data.Volume) {
				t.Errorf("Count = %d, but Volume length = %d", data.Count, len(data.Volume))
			}

			// Verify data correctness if we have data
			if len(data.Volume) > 0 && tt.requestedCount > 0 {
				// Calculate expected start index
				start := tt.klineCount - tt.requestedCount
				if start < 0 {
					start = 0
				}

				// Verify first and last values match expected K-lines
				expectedFirstVolume := klines[start].Volume
				if data.Volume[0] != expectedFirstVolume {
					t.Errorf("First volume = %.2f, want %.2f", data.Volume[0], expectedFirstVolume)
				}

				expectedLastVolume := klines[tt.klineCount-1].Volume
				lastVolume := data.Volume[len(data.Volume)-1]
				if lastVolume != expectedLastVolume {
					t.Errorf("Last volume = %.2f, want %.2f", lastVolume, expectedLastVolume)
				}
			}
		})
	}
}

// TestBuildDataFromKlines tests the updated function with new parameters
func TestBuildDataFromKlines(t *testing.T) {
	tests := []struct {
		name             string
		timeframes       []string
		primaryTimeframe string
		klineCount       int
		expectedTFData   bool // Should populate TimeframeData
		expectedIntraday bool // Should populate IntradaySeries
	}{
		{
			name:             "Standard case with 3m primary",
			timeframes:       []string{"3m", "4h"},
			primaryTimeframe: "3m",
			klineCount:       30,
			expectedTFData:   true,
			expectedIntraday: true,
		},
		{
			name:             "Single timeframe",
			timeframes:       []string{"3m"},
			primaryTimeframe: "3m",
			klineCount:       10,
			expectedTFData:   true,
			expectedIntraday: true,
		},
		{
			name:             "Multiple timeframes with 4h primary",
			timeframes:       []string{"3m", "4h"},
			primaryTimeframe: "4h",
			klineCount:       20,
			expectedTFData:   true,
			expectedIntraday: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test K-line data for each timeframe
			timeframeSeries := make(map[string][]Kline)
			for _, tf := range tt.timeframes {
				timeframeSeries[tf] = generateTestKlines(50) // Generate enough data
			}

			longerSeries := make(map[string][]Kline)
			longerSeries["4h"] = generateTestKlines(100)

			data := BuildDataFromKlines("BTCUSDT", timeframeSeries, longerSeries, tt.timeframes, tt.primaryTimeframe, tt.klineCount)

			if data == nil {
				t.Fatal("BuildDataFromKlines returned nil")
			}

			if tt.expectedTFData {
				if data.TimeframeData == nil {
					t.Error("TimeframeData should not be nil")
				}
				// Verify TimeframeData has entries for requested timeframes
				for _, tf := range tt.timeframes {
					if _, exists := data.TimeframeData[tf]; !exists {
						t.Errorf("TimeframeData missing entry for timeframe %s", tf)
					}
				}
			}

			if tt.expectedIntraday {
				if data.IntradaySeries == nil {
					t.Error("IntradaySeries should not be nil")
				}
				// Verify IntradaySeries has correct length (should use klineCount)
				actualLen := len(data.IntradaySeries.Volume)
				expectedLen := tt.klineCount
				if len(timeframeSeries[tt.primaryTimeframe]) < tt.klineCount {
					expectedLen = len(timeframeSeries[tt.primaryTimeframe])
				}
				if actualLen != expectedLen {
					t.Errorf("IntradaySeries volume length = %d, want %d", actualLen, expectedLen)
				}
			}
		})
	}
}

// TestBuildDataFromKlinesWithConfig tests the new configuration-based function
func TestBuildDataFromKlinesWithConfig(t *testing.T) {
	tests := []struct {
		name             string
		timeframes       []string
		primaryTimeframe string
		klineCount       int
	}{
		{
			name:             "Backtest configuration - 30 K-lines",
			timeframes:       []string{"3m", "4h"},
			primaryTimeframe: "3m",
			klineCount:       30,
		},
		{
			name:             "Live trading configuration - 30 K-lines",
			timeframes:       []string{"3m", "4h"},
			primaryTimeframe: "3m",
			klineCount:       30,
		},
		{
			name:             "Custom configuration - 15 K-lines",
			timeframes:       []string{"3m"},
			primaryTimeframe: "3m",
			klineCount:       15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test K-line data
			timeframeSeries := make(map[string][]Kline)
			for _, tf := range tt.timeframes {
				timeframeSeries[tf] = generateTestKlines(50)
			}

			longerSeries := make(map[string][]Kline)
			longerSeries["4h"] = generateTestKlines(100)

			data := BuildDataFromKlinesWithConfig("BTCUSDT", timeframeSeries, longerSeries, tt.timeframes, tt.primaryTimeframe, tt.klineCount)

			if data == nil {
				t.Fatal("BuildDataFromKlinesWithConfig returned nil")
			}

			// Verify TimeframeData is properly populated
			if data.TimeframeData == nil {
				t.Error("TimeframeData should not be nil")
			}

			for _, tf := range tt.timeframes {
				if seriesData, exists := data.TimeframeData[tf]; !exists {
					t.Errorf("TimeframeData missing entry for timeframe %s", tf)
				} else {
					// Verify the data has correct length
					actualLen := len(seriesData.Volume)
					expectedLen := tt.klineCount
					if len(timeframeSeries[tf]) < tt.klineCount {
						expectedLen = len(timeframeSeries[tf])
					}
					if actualLen != expectedLen {
						t.Errorf("TimeframeData[%s] volume length = %d, want %d", tf, actualLen, expectedLen)
					}
				}
			}

			// Verify IntradaySeries uses configurable count
			if data.IntradaySeries == nil {
				t.Error("IntradaySeries should not be nil")
			}

			actualLen := len(data.IntradaySeries.Volume)
			expectedLen := tt.klineCount
			if len(timeframeSeries[tt.primaryTimeframe]) < tt.klineCount {
				expectedLen = len(timeframeSeries[tt.primaryTimeframe])
			}
			if actualLen != expectedLen {
				t.Errorf("IntradaySeries volume length = %d, want %d", actualLen, expectedLen)
			}
		})
	}
}

// TestKlineConsistency tests that backtest and live trading use same data structures
func TestKlineConsistency(t *testing.T) {
	symbol := "BTCUSDT"
	timeframes := []string{"3m", "4h"}
	primaryTimeframe := "3m"
	klineCount := 30

	// Generate test data
	timeframeSeries := make(map[string][]Kline)
	for _, tf := range timeframes {
		timeframeSeries[tf] = generateTestKlines(50)
	}
	longerSeries := map[string][]Kline{"4h": generateTestKlines(100)}

	// Test BuildDataFromKlinesWithConfig (used by backtest)
	backtestData := BuildDataFromKlinesWithConfig(symbol, timeframeSeries, longerSeries, timeframes, primaryTimeframe, klineCount)

	// Test BuildDataFromKlines with same parameters (used by live trading)
	liveData := BuildDataFromKlines(symbol, timeframeSeries, longerSeries, timeframes, primaryTimeframe, klineCount)

	if backtestData == nil || liveData == nil {
		t.Fatal("Data functions returned nil")
	}

	// Both should have TimeframeData populated
	if backtestData.TimeframeData == nil || liveData.TimeframeData == nil {
		t.Error("TimeframeData should be populated in both backtest and live data")
	}

	// Both should have IntradaySeries with same length
	if backtestData.IntradaySeries == nil || liveData.IntradaySeries == nil {
		t.Error("IntradaySeries should be populated in both backtest and live data")
	}

	backtestLen := len(backtestData.IntradaySeries.Volume)
	liveLen := len(liveData.IntradaySeries.Volume)
	if backtestLen != liveLen {
		t.Errorf("IntradaySeries length mismatch: backtest=%d, live=%d", backtestLen, liveLen)
	}

	if backtestLen != klineCount {
		t.Errorf("IntradaySeries should use klineCount=%d, got %d", klineCount, backtestLen)
	}

	// TimeframeData should have same structure
	for _, tf := range timeframes {
		backtestTF, backtestExists := backtestData.TimeframeData[tf]
		liveTF, liveExists := liveData.TimeframeData[tf]

		if !backtestExists || !liveExists {
			t.Errorf("TimeframeData[%s] should exist in both: backtest=%v, live=%v", tf, backtestExists, liveExists)
			continue
		}

		if len(backtestTF.Volume) != len(liveTF.Volume) {
			t.Errorf("TimeframeData[%s] length mismatch: backtest=%d, live=%d", tf, len(backtestTF.Volume), len(liveTF.Volume))
		}
	}
}

// TestGetCurrentPriceWithFallback tests the new price fetching mechanism
func TestGetCurrentPriceWithFallback(t *testing.T) {
	tests := []struct {
		name               string
		klineCount         int
		expectedPriceSrc   string     // "kline_fallback" since API will likely fail in tests
		expectedPriceRange [2]float64 // min/max range for price validation
	}{
		{
			name:               "Normal K-line data",
			klineCount:         10,
			expectedPriceSrc:   "kline_fallback",        // API call expected to fail in test environment
			expectedPriceRange: [2]float64{99.0, 110.0}, // Should be around 100.3 based on generateTestKlines
		},
		{
			name:               "Single K-line",
			klineCount:         1,
			expectedPriceSrc:   "kline_fallback",
			expectedPriceRange: [2]float64{99.0, 110.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines := generateTestKlines(tt.klineCount)

			price, source := getCurrentPriceWithFallback("BTCUSDT", klines)

			// Validate price source (should be fallback due to API unavailability in tests)
			if source != tt.expectedPriceSrc {
				t.Logf("Price source: got %s, expected %s (this is normal in test environment)", source, tt.expectedPriceSrc)
			}

			// Validate price is within expected range
			if price < tt.expectedPriceRange[0] || price > tt.expectedPriceRange[1] {
				t.Errorf("Price %.4f outside expected range [%.1f, %.1f]",
					price, tt.expectedPriceRange[0], tt.expectedPriceRange[1])
			}

			// Validate price is positive
			if price <= 0 {
				t.Errorf("Price should be positive, got %.4f", price)
			}
		})
	}
}

// TestGetCurrentPriceWithFallback_EmptyKlines tests edge case with no K-line data
func TestGetCurrentPriceWithFallback_EmptyKlines(t *testing.T) {
	price, source := getCurrentPriceWithFallback("BTCUSDT", []Kline{})

	if price != 0 {
		t.Errorf("Expected price 0 for empty klines, got %.4f", price)
	}

	if source != "no_data" {
		t.Errorf("Expected source 'no_data' for empty klines, got %s", source)
	}
}
