package trader

import (
	"testing"
)

func TestShouldTriggerDrawdownClose(t *testing.T) {
	tests := []struct {
		name          string
		enable        bool
		currentPnLPct float64
		peakPnLPct    float64
		minProfitPct  float64
		pullbackPct   float64
		expected      bool
	}{
		{
			name:          "disabled returns false",
			enable:        false,
			currentPnLPct: 10,
			peakPnLPct:    20,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      false,
		},
		{
			name:          "current below min profit returns false",
			enable:        true,
			currentPnLPct: 4,
			peakPnLPct:    20,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      false,
		},
		{
			name:          "current equals min profit returns false",
			enable:        true,
			currentPnLPct: 5,
			peakPnLPct:    20,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      false,
		},
		{
			name:          "peak <= 0 returns false",
			enable:        true,
			currentPnLPct: 10,
			peakPnLPct:    0,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      false,
		},
		{
			name:          "current >= peak returns false",
			enable:        true,
			currentPnLPct: 20,
			peakPnLPct:    20,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      false,
		},
		{
			name:          "pullback below threshold returns false",
			enable:        true,
			currentPnLPct: 15,
			peakPnLPct:    20,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      false,
		},
		{
			name:          "pullback at threshold returns true",
			enable:        true,
			currentPnLPct: 12,
			peakPnLPct:    20,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      true,
		},
		{
			name:          "pullback above threshold returns true",
			enable:        true,
			currentPnLPct: 6,
			peakPnLPct:    20,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      true,
		},
		{
			name:          "default thresholds 5% profit and 40% pullback",
			enable:        true,
			currentPnLPct: 6,
			peakPnLPct:    10,
			minProfitPct:  5,
			pullbackPct:   40,
			expected:      true,
		},
		{
			name:          "custom thresholds 10% profit 30% pullback",
			enable:        true,
			currentPnLPct: 14,
			peakPnLPct:    20,
			minProfitPct:  10,
			pullbackPct:   30,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldTriggerDrawdownClose(tt.enable, tt.currentPnLPct, tt.peakPnLPct, tt.minProfitPct, tt.pullbackPct)
			if got != tt.expected {
				t.Errorf("shouldTriggerDrawdownClose(%v, %.2f, %.2f, %.2f, %.2f) = %v, want %v",
					tt.enable, tt.currentPnLPct, tt.peakPnLPct, tt.minProfitPct, tt.pullbackPct, got, tt.expected)
			}
		})
	}
}
