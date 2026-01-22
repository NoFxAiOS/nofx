// backtest/risk/regime_test.go
package risk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegimeCalculator_VolRegime(t *testing.T) {
	tests := []struct {
		name          string
		atrPercentile float64
		expected      string
	}{
		{"high volatility", 0.75, "high"},
		{"mid volatility", 0.50, "mid"},
		{"low volatility", 0.25, "low"},
		{"boundary high", 0.70, "high"},
		{"boundary low", 0.30, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime := calcVolRegime(tt.atrPercentile)
			assert.Equal(t, tt.expected, regime)
		})
	}
}

func TestRegimeCalculator_TrendRegime(t *testing.T) {
	tests := []struct {
		name     string
		adx      float64
		expected string
	}{
		{"trending", 30.0, "trending"},
		{"ranging", 20.0, "ranging"},
		{"boundary", 25.0, "trending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime := calcTrendRegime(tt.adx)
			assert.Equal(t, tt.expected, regime)
		})
	}
}

func TestRegimeCalculator_PrimaryRegime(t *testing.T) {
	rc := NewRegimeCalculator()

	// High vol + trending
	regime := rc.Calculate(0.75, 30.0)
	assert.Equal(t, "high_trending", regime.PrimaryRegime)
	assert.Equal(t, "high", regime.VolRegime)
	assert.Equal(t, "trending", regime.TrendRegime)

	// Low vol + ranging
	regime = rc.Calculate(0.25, 20.0)
	assert.Equal(t, "low_ranging", regime.PrimaryRegime)
}

func TestValidPrimaryRegimes(t *testing.T) {
	expected := []string{
		"high_trending", "high_ranging",
		"mid_trending", "mid_ranging",
		"low_trending", "low_ranging",
	}
	assert.ElementsMatch(t, expected, ValidPrimaryRegimes)
}

func TestCalculateATRPercentile(t *testing.T) {
	tests := []struct {
		name           string
		currentATR     float64
		historicalATRs []float64
		expected       float64
	}{
		{
			name:           "empty historical returns 0.5",
			currentATR:     1.0,
			historicalATRs: []float64{},
			expected:       0.5,
		},
		{
			name:           "current ATR is highest",
			currentATR:     10.0,
			historicalATRs: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			expected:       1.0, // all 5 values <= 10
		},
		{
			name:           "current ATR is lowest",
			currentATR:     0.5,
			historicalATRs: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			expected:       0.0, // no values <= 0.5
		},
		{
			name:           "current ATR is median",
			currentATR:     3.0,
			historicalATRs: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			expected:       0.6, // 3 values <= 3.0 (1, 2, 3) = 3/5
		},
		{
			name:           "single historical value equal",
			currentATR:     5.0,
			historicalATRs: []float64{5.0},
			expected:       1.0,
		},
		{
			name:           "single historical value greater",
			currentATR:     3.0,
			historicalATRs: []float64{5.0},
			expected:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateATRPercentile(tt.currentATR, tt.historicalATRs)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}

func TestRegimeCalculator_AllCombinations(t *testing.T) {
	rc := NewRegimeCalculator()

	// Test all 6 valid combinations
	combinations := []struct {
		atrPercentile float64
		adx           float64
		expected      string
	}{
		{0.80, 30.0, "high_trending"},
		{0.80, 20.0, "high_ranging"},
		{0.50, 30.0, "mid_trending"},
		{0.50, 20.0, "mid_ranging"},
		{0.20, 30.0, "low_trending"},
		{0.20, 20.0, "low_ranging"},
	}

	for _, c := range combinations {
		t.Run(c.expected, func(t *testing.T) {
			regime := rc.Calculate(c.atrPercentile, c.adx)
			assert.Equal(t, c.expected, regime.PrimaryRegime)
			assert.Equal(t, c.atrPercentile, regime.ATRPercentile)
			assert.Equal(t, c.adx, regime.ADX)
			assert.False(t, regime.CalculatedAt.IsZero())
		})
	}
}

func TestRegimeCalculator_FieldsPopulated(t *testing.T) {
	rc := NewRegimeCalculator()
	regime := rc.Calculate(0.55, 22.0)

	assert.Equal(t, "mid", regime.VolRegime)
	assert.Equal(t, "ranging", regime.TrendRegime)
	assert.Equal(t, "mid_ranging", regime.PrimaryRegime)
	assert.Equal(t, 0.55, regime.ATRPercentile)
	assert.Equal(t, 22.0, regime.ADX)
	assert.False(t, regime.CalculatedAt.IsZero())
}
