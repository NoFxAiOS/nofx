// backtest/risk/regime.go
package risk

import "time"

// ValidPrimaryRegimes lists all valid regime combinations.
var ValidPrimaryRegimes = []string{
	"high_trending", "high_ranging",
	"mid_trending", "mid_ranging",
	"low_trending", "low_ranging",
}

// RegimeCalculator calculates market regime from indicators.
type RegimeCalculator struct{}

// NewRegimeCalculator creates a new regime calculator.
func NewRegimeCalculator() *RegimeCalculator {
	return &RegimeCalculator{}
}

// Calculate computes regime from ATR percentile and ADX.
func (rc *RegimeCalculator) Calculate(atrPercentile, adx float64) RegimeSummary {
	volRegime := calcVolRegime(atrPercentile)
	trendRegime := calcTrendRegime(adx)

	return RegimeSummary{
		VolRegime:     volRegime,
		TrendRegime:   trendRegime,
		PrimaryRegime: volRegime + "_" + trendRegime,
		ATRPercentile: atrPercentile,
		ADX:           adx,
		CalculatedAt:  time.Now(),
	}
}

// calcVolRegime determines volatility regime from ATR percentile.
func calcVolRegime(atrPercentile float64) string {
	if atrPercentile >= VolHighPercentile {
		return "high"
	}
	if atrPercentile <= VolLowPercentile {
		return "low"
	}
	return "mid"
}

// calcTrendRegime determines trend regime from ADX.
func calcTrendRegime(adx float64) string {
	if adx >= TrendADXThreshold {
		return "trending"
	}
	return "ranging"
}

// CalculateATRPercentile calculates where current ATR sits in historical distribution.
func CalculateATRPercentile(currentATR float64, historicalATRs []float64) float64 {
	if len(historicalATRs) == 0 {
		return 0.5 // default to mid
	}

	count := 0
	for _, atr := range historicalATRs {
		if atr <= currentATR {
			count++
		}
	}
	return float64(count) / float64(len(historicalATRs))
}
