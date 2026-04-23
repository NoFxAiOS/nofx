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
