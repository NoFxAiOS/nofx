package market

import "math"

// calculateEMA calculates EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// Calculate SMA as initial EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// Calculate EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD calculates MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	// Calculate 12-period and 26-period EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12 - ema26
}

// calculateRSI calculates RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// Calculate initial average gain/loss
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// Use Wilder smoothing method to calculate subsequent RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR calculates ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// Calculate initial ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder smoothing
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateBOLL calculates Bollinger Bands (upper, middle, lower)
// period: typically 20, multiplier: typically 2
func calculateBOLL(klines []Kline, period int, multiplier float64) (upper, middle, lower float64) {
	if len(klines) < period {
		return 0, 0, 0
	}

	// Calculate SMA (middle band)
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	sma := sum / float64(period)

	// Calculate standard deviation
	variance := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		diff := klines[i].Close - sma
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(period))

	// Calculate bands
	middle = sma
	upper = sma + multiplier*stdDev
	lower = sma - multiplier*stdDev

	return upper, middle, lower
}

// calculateDonchian calculates Donchian channel (highest high, lowest low) for given period
func calculateDonchian(klines []Kline, period int) (upper, lower float64) {
	if len(klines) == 0 || period <= 0 {
		return 0, 0
	}

	// Use all available klines if period > len(klines)
	start := len(klines) - period
	if start < 0 {
		start = 0
	}

	upper = klines[start].High
	lower = klines[start].Low

	for i := start + 1; i < len(klines); i++ {
		if klines[i].High > upper {
			upper = klines[i].High
		}
		if klines[i].Low < lower {
			lower = klines[i].Low
		}
	}

	return upper, lower
}

// Box period constants (in 1h candles)
const (
	ShortBoxPeriod = 72  // 3 days of 1h candles
	MidBoxPeriod   = 240 // 10 days of 1h candles
	LongBoxPeriod  = 500 // ~21 days of 1h candles
)

// calculateBoxData calculates multi-period box data from klines
func calculateBoxData(klines []Kline, currentPrice float64) *BoxData {
	box := &BoxData{
		CurrentPrice: currentPrice,
	}

	if len(klines) == 0 {
		return box
	}

	box.ShortUpper, box.ShortLower = calculateDonchian(klines, ShortBoxPeriod)
	box.MidUpper, box.MidLower = calculateDonchian(klines, MidBoxPeriod)
	box.LongUpper, box.LongLower = calculateDonchian(klines, LongBoxPeriod)

	return box
}

// ========== Exported indicator calculation functions (for testing) ==========

// ExportCalculateEMA exports calculateEMA for testing
func ExportCalculateEMA(klines []Kline, period int) float64 {
	return calculateEMA(klines, period)
}

// ExportCalculateMACD exports calculateMACD for testing
func ExportCalculateMACD(klines []Kline) float64 {
	return calculateMACD(klines)
}

// ExportCalculateRSI exports calculateRSI for testing
func ExportCalculateRSI(klines []Kline, period int) float64 {
	return calculateRSI(klines, period)
}

// ExportCalculateATR exports calculateATR for testing
func ExportCalculateATR(klines []Kline, period int) float64 {
	return calculateATR(klines, period)
}

// ExportCalculateBOLL exports calculateBOLL for testing
func ExportCalculateBOLL(klines []Kline, period int, multiplier float64) (upper, middle, lower float64) {
	return calculateBOLL(klines, period, multiplier)
}

// ExportCalculateDonchian exports calculateDonchian for testing
func ExportCalculateDonchian(klines []Kline, period int) (float64, float64) {
	return calculateDonchian(klines, period)
}

// ExportCalculateBoxData exports calculateBoxData for testing
func ExportCalculateBoxData(klines []Kline, currentPrice float64) *BoxData {
	return calculateBoxData(klines, currentPrice)
}

// CalculateCVD computes Cumulative Volume Delta from kline data over the last
// `periods` candles. For each candle we approximate taker flow from price action:
// if close > open → buyer dominant, if close < open → seller dominant.
func CalculateCVD(klines []Kline, periods int) float64 {
	if len(klines) == 0 || periods <= 0 {
		return 0
	}
	start := len(klines) - periods
	if start < 0 {
		start = 0
	}
	cvd := 0.0
	for _, k := range klines[start:] {
		hl := k.High - k.Low
		if hl == 0 {
			continue
		}
		var buyVol float64
		if k.Close >= k.Open {
			buyVol = k.Volume * (k.Close - k.Low) / hl
		} else {
			buyVol = k.Volume * (1 - (k.High-k.Close)/hl)
		}
		sellVol := k.Volume - buyVol
		cvd += buyVol - sellVol
	}
	return cvd
}

// CalculateVWAP computes Volume-Weighted Average Price from kline data.
// typical_price = (high + low + close) / 3
func CalculateVWAP(klines []Kline) float64 {
	if len(klines) == 0 {
		return 0
	}
	sumPV := 0.0
	sumV := 0.0
	for _, k := range klines {
		typical := (k.High + k.Low + k.Close) / 3
		sumPV += typical * k.Volume
		sumV += k.Volume
	}
	if sumV == 0 {
		return 0
	}
	return sumPV / sumV
}

// CalculateOIGrowthRate computes OI growth rate percentage.
// Returns 0 when pastOI is zero to avoid division by zero.
func CalculateOIGrowthRate(currentOI, pastOI float64) float64 {
	if pastOI == 0 {
		return 0
	}
	return (currentOI - pastOI) / pastOI * 100
}

// ClassifyFundingTrend analyzes funding rate history to determine trend.
// Thresholds: |rate| >= 0.001 -> extreme; rising/falling from first vs last half averages.
func ClassifyFundingTrend(history []float64) string {
	if len(history) == 0 {
		return "stable"
	}
	last := history[len(history)-1]
	if last >= 0.001 {
		return "extreme_positive"
	}
	if last <= -0.001 {
		return "extreme_negative"
	}
	if len(history) < 2 {
		return "stable"
	}
	mid := len(history) / 2
	var early, late float64
	for _, v := range history[:mid] {
		early += v
	}
	for _, v := range history[mid:] {
		late += v
	}
	early /= float64(mid)
	late /= float64(len(history) - mid)
	diff := late - early
	const threshold = 0.00005
	if diff > threshold {
		return "rising"
	}
	if diff < -threshold {
		return "falling"
	}
	return "stable"
}

// CalculateTakerDelta computes normalized taker buy/sell delta in [-1, 1].
func CalculateTakerDelta(takerBuyVol, takerSellVol float64) float64 {
	total := takerBuyVol + takerSellVol
	if total == 0 {
		return 0
	}
	return (takerBuyVol - takerSellVol) / total
}

// CalculateDepthChangeRate computes the rate of change in depth imbalance.
// Returns 0 when previousImbalance is zero.
func CalculateDepthChangeRate(currentImbalance, previousImbalance float64) float64 {
	if previousImbalance == 0 {
		return 0
	}
	abs := previousImbalance
	if abs < 0 {
		abs = -abs
	}
	return (currentImbalance - previousImbalance) / abs * 100
}
