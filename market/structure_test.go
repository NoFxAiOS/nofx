package market

import (
	"testing"
)

func TestDetectStructuralLevels_Basic(t *testing.T) {
	// Create wave pattern with clear S/R levels
	prices := []float64{
		100, 102, 105, 108, 110, 108, 105, 102, 100, 98,
		100, 103, 106, 109, 111, 109, 106, 103, 100, 97,
		99, 102, 105, 108, 110, 108, 105, 103, 101, 99,
	}
	klines := makeKlines(prices)
	currentPrice := 105.0

	levels := DetectStructuralLevels(klines, currentPrice, "1h")
	if len(levels) == 0 {
		t.Fatal("expected at least one structural level")
	}

	// Check that we have both support and resistance
	hasSupport := false
	hasResistance := false
	for _, l := range levels {
		if l.Type == "support" {
			hasSupport = true
		}
		if l.Type == "resistance" {
			hasResistance = true
		}
	}
	if !hasSupport {
		t.Error("expected at least one support level")
	}
	if !hasResistance {
		t.Error("expected at least one resistance level")
	}
}

func TestDetectStructuralLevels_Sorted(t *testing.T) {
	prices := make([]float64, 50)
	for i := range prices {
		// Oscillating pattern
		if i%10 < 5 {
			prices[i] = 100 + float64(i%10)*2
		} else {
			prices[i] = 110 - float64(i%10-5)*2
		}
	}
	klines := makeKlines(prices)
	currentPrice := 105.0

	levels := DetectStructuralLevels(klines, currentPrice, "1h")
	// Verify sorted by distance from current price
	for i := 1; i < len(levels); i++ {
		distPrev := abs(levels[i-1].Price - currentPrice)
		distCurr := abs(levels[i].Price - currentPrice)
		if distCurr < distPrev {
			t.Errorf("levels not sorted by distance: [%d] dist=%.2f > [%d] dist=%.2f",
				i-1, distPrev, i, distCurr)
		}
	}
}

func TestDetectStructuralLevels_TooFewKlines(t *testing.T) {
	klines := makeKlines([]float64{100, 101, 102})
	levels := DetectStructuralLevels(klines, 101, "1h")
	if levels != nil {
		t.Error("expected nil for too few klines")
	}
}

func TestMergeLevels(t *testing.T) {
	levels := []StructuralLevel{
		{Price: 100.0, Type: "support", Strength: 2, Source: "swing_point"},
		{Price: 100.3, Type: "support", Strength: 3, Source: "fibonacci"},
		{Price: 200.0, Type: "resistance", Strength: 1, Source: "swing_point"},
	}
	merged := mergeLevels(levels, 0.005)
	if len(merged) != 2 {
		t.Errorf("expected 2 merged levels, got %d", len(merged))
	}
	// First merged level should have higher strength
	if merged[0].Strength != 3 {
		t.Errorf("expected merged strength 3, got %d", merged[0].Strength)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestComputeRecencyScore(t *testing.T) {
	// Just touched (0 bars ago) → score ~1.0
	score := computeRecencyScore(0, 100)
	if score < 0.99 {
		t.Errorf("expected ~1.0 for just-touched, got %.4f", score)
	}

	// Half-way through → moderate score
	score = computeRecencyScore(50, 100)
	if score < 0.1 || score > 0.5 {
		t.Errorf("expected moderate score for half-way, got %.4f", score)
	}

	// Very old → low score
	score = computeRecencyScore(100, 100)
	if score > 0.1 {
		t.Errorf("expected low score for oldest, got %.4f", score)
	}
}

func TestComputeVolumeScoreAtLevel(t *testing.T) {
	klines := make([]Kline, 20)
	for i := range klines {
		klines[i] = Kline{
			Open: 100, High: 101, Low: 99, Close: 100,
			Volume: 1000,
		}
	}
	// Make candles 5-9 touch level 105 with high volume
	for i := 5; i < 10; i++ {
		klines[i].High = 106
		klines[i].Low = 104
		klines[i].Volume = 5000 // well above avg
	}

	score := computeVolumeScoreAtLevel(klines, 105.0, 0.005)
	if score <= 0 {
		t.Errorf("expected positive volume score, got %.4f", score)
	}
}

func TestComputeAvgBounceVolume(t *testing.T) {
	// Create a bounce pattern at price 100
	klines := []Kline{
		{Open: 102, High: 103, Low: 101, Close: 101, Volume: 500},  // down
		{Open: 101, High: 101, Low: 99.5, Close: 100, Volume: 2000}, // touches 100, reversal
		{Open: 100, High: 103, Low: 100, Close: 102, Volume: 800},  // bounces up
		{Open: 102, High: 104, Low: 102, Close: 103, Volume: 500},
	}
	vol := computeAvgBounceVolume(klines, 100.0, 0.005)
	if vol <= 0 {
		t.Errorf("expected positive bounce volume, got %.4f", vol)
	}
}

func TestComputeCompositeConfidence(t *testing.T) {
	level := StructuralLevel{
		TouchCount:   5,
		VolumeScore:  0.8,
		RecencyScore: 0.9,
		MultiTFCount: 2,
	}
	conf := computeCompositeConfidence(&level)
	if conf <= 0 || conf > 100 {
		t.Errorf("confidence out of range: %.2f", conf)
	}
	// With strong scores across all dimensions, should be high
	if conf < 50 {
		t.Errorf("expected high confidence for strong level, got %.2f", conf)
	}
}

func TestDetectStructuralLevels_ConfidencePopulated(t *testing.T) {
	prices := []float64{
		100, 102, 105, 108, 110, 108, 105, 102, 100, 98,
		100, 103, 106, 109, 111, 109, 106, 103, 100, 97,
		99, 102, 105, 108, 110, 108, 105, 103, 101, 99,
	}
	klines := makeKlines(prices)
	levels := DetectStructuralLevels(klines, 105.0, "1h")

	hasConfidence := false
	for _, l := range levels {
		if l.Confidence > 0 {
			hasConfidence = true
			break
		}
	}
	if !hasConfidence {
		t.Error("expected at least one level with confidence > 0")
	}
}

func TestEnrichMultiTFConfirmation(t *testing.T) {
	tfData := map[string]*TimeframeSeriesData{
		"15m": {
			StructuralLevels: []StructuralLevel{
				{Price: 100.0, Timeframe: "15m"},
				{Price: 200.0, Timeframe: "15m"},
			},
		},
		"1h": {
			StructuralLevels: []StructuralLevel{
				{Price: 100.5, Timeframe: "1h"}, // within 0.8% of 100.0
				{Price: 300.0, Timeframe: "1h"},
			},
		},
		"4h": {
			StructuralLevels: []StructuralLevel{
				{Price: 99.8, Timeframe: "4h"}, // within 0.8% of 100.0
			},
		},
	}

	enrichMultiTFConfirmation(tfData)

	// 15m level at 100.0 should be confirmed by 1h (100.5) and 4h (99.8)
	found := false
	for _, l := range tfData["15m"].StructuralLevels {
		if abs(l.Price-100.0) < 1 {
			if l.MultiTFCount != 2 {
				t.Errorf("expected MultiTFCount=2 for 100.0 on 15m, got %d", l.MultiTFCount)
			}
			found = true
		}
	}
	if !found {
		t.Error("expected to find 15m level at 100.0")
	}

	// 15m level at 200.0 should have no confirmation
	for _, l := range tfData["15m"].StructuralLevels {
		if abs(l.Price-200.0) < 1 && l.MultiTFCount != 0 {
			t.Errorf("expected MultiTFCount=0 for 200.0, got %d", l.MultiTFCount)
		}
	}
}

func TestMergeLevels_PreservesConfidenceData(t *testing.T) {
	levels := []StructuralLevel{
		{Price: 100.0, Strength: 2, Source: "swing_point", TouchCount: 3, VolumeScore: 0.5, RecencyScore: 0.8, LastTouchBars: 5},
		{Price: 100.3, Strength: 3, Source: "volume_cluster", TouchCount: 2, VolumeScore: 0.9, RecencyScore: 0.6, LastTouchBars: 10},
	}
	merged := mergeLevels(levels, 0.005)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged, got %d", len(merged))
	}
	m := merged[0]
	if m.TouchCount != 5 {
		t.Errorf("expected combined TouchCount=5, got %d", m.TouchCount)
	}
	if m.VolumeScore != 0.9 {
		t.Errorf("expected best VolumeScore=0.9, got %.2f", m.VolumeScore)
	}
	if m.RecencyScore != 0.8 {
		t.Errorf("expected best RecencyScore=0.8, got %.2f", m.RecencyScore)
	}
	if m.LastTouchBars != 5 {
		t.Errorf("expected LastTouchBars=5 (from more recent), got %d", m.LastTouchBars)
	}
}
