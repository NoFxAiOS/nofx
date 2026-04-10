package market

// IndicatorCalculator provides methods to calculate technical indicators
// Used by the quant model engine for strategy execution
type IndicatorCalculator struct{}

// NewIndicatorCalculator creates a new indicator calculator instance
func NewIndicatorCalculator() *IndicatorCalculator {
	return &IndicatorCalculator{}
}

// RSI calculates the Relative Strength Index for a series of closing prices
func (ic *IndicatorCalculator) RSI(closes []float64, period int) []float64 {
	if len(closes) <= period {
		return []float64{}
	}

	result := make([]float64, 0, len(closes)-period)
	
	// Calculate initial average gain/loss
	gains := 0.0
	losses := 0.0
	
	for i := 1; i <= period; i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}
	
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	
	// Calculate first RSI
	if avgLoss == 0 {
		result = append(result, 100)
	} else {
		rs := avgGain / avgLoss
		rsi := 100 - (100 / (1 + rs))
		result = append(result, rsi)
	}
	
	// Use Wilder smoothing for subsequent values
	for i := period + 1; i < len(closes); i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
		
		if avgLoss == 0 {
			result = append(result, 100)
		} else {
			rs := avgGain / avgLoss
			rsi := 100 - (100 / (1 + rs))
			result = append(result, rsi)
		}
	}
	
	return result
}

// EMA calculates the Exponential Moving Average
func (ic *IndicatorCalculator) EMA(closes []float64, period int) []float64 {
	if len(closes) < period {
		return []float64{}
	}
	
	result := make([]float64, 0, len(closes)-period+1)
	
	// Calculate initial SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += closes[i]
	}
	ema := sum / float64(period)
	result = append(result, ema)
	
	// Calculate EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(closes); i++ {
		ema = (closes[i]-ema)*multiplier + ema
		result = append(result, ema)
	}
	
	return result
}

// SMA calculates the Simple Moving Average
func (ic *IndicatorCalculator) SMA(values []float64, period int) []float64 {
	if len(values) < period {
		return []float64{}
	}
	
	result := make([]float64, 0, len(values)-period+1)
	
	for i := period - 1; i < len(values); i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += values[j]
		}
		result = append(result, sum/float64(period))
	}
	
	return result
}

// MACD calculates MACD, Signal line, and Histogram
func (ic *IndicatorCalculator) MACD(closes []float64, fast, slow, signal int) (macd, signalLine, histogram []float64) {
	if len(closes) < slow {
		return []float64{}, []float64{}, []float64{}
	}
	
	// Calculate fast and slow EMAs
	emaFast := ic.EMA(closes, fast)
	emaSlow := ic.EMA(closes, slow)
	
	if len(emaFast) == 0 || len(emaSlow) == 0 {
		return []float64{}, []float64{}, []float64{}
	}
	
	// Align EMAs (they have different lengths due to different periods)
	offset := len(emaFast) - len(emaSlow)
	
	// Calculate MACD line
	macd = make([]float64, len(emaSlow))
	for i := 0; i < len(emaSlow); i++ {
		macd[i] = emaFast[i+offset] - emaSlow[i]
	}
	
	// Calculate signal line (EMA of MACD)
	signalLine = ic.EMA(macd, signal)
	
	// Calculate histogram
	histogram = make([]float64, len(signalLine))
	signalOffset := len(macd) - len(signalLine)
	for i := 0; i < len(signalLine); i++ {
		histogram[i] = macd[i+signalOffset] - signalLine[i]
	}
	
	return macd, signalLine, histogram
}

// ATR calculates the Average True Range
func (ic *IndicatorCalculator) ATR(highs, lows, closes []float64, period int) []float64 {
	if len(highs) != len(lows) || len(highs) != len(closes) || len(highs) <= period {
		return []float64{}
	}
	
	// Calculate True Ranges
	trs := make([]float64, len(highs))
	trs[0] = highs[0] - lows[0] // First TR is just the range
	
	for i := 1; i < len(highs); i++ {
		hl := highs[i] - lows[i]
		hc := abs(highs[i] - closes[i-1])
		lc := abs(lows[i] - closes[i-1])
		trs[i] = max(hl, max(hc, lc))
	}
	
	// Calculate initial ATR
	result := make([]float64, 0, len(trs)-period+1)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)
	result = append(result, atr)
	
	// Wilder smoothing
	for i := period; i < len(trs); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
		result = append(result, atr)
	}
	
	return result
}

// BOLL calculates Bollinger Bands (upper, middle, lower)
func (ic *IndicatorCalculator) BOLL(closes []float64, period int, multiplier float64) (upper, middle, lower []float64) {
	if len(closes) < period {
		return []float64{}, []float64{}, []float64{}
	}
	
	sma := ic.SMA(closes, period)
	if len(sma) == 0 {
		return []float64{}, []float64{}, []float64{}
	}
	
	upper = make([]float64, len(sma))
	middle = make([]float64, len(sma))
	lower = make([]float64, len(sma))
	
	for i := 0; i < len(sma); i++ {
		// Calculate standard deviation
		startIdx := i
		if len(closes) > len(sma) {
			startIdx = i + (len(closes) - len(sma))
		}
		
		variance := 0.0
		for j := 0; j < period; j++ {
			if startIdx+j < len(closes) {
				diff := closes[startIdx+j] - sma[i]
				variance += diff * diff
			}
		}
		stdDev := sqrt(variance / float64(period))
		
		middle[i] = sma[i]
		upper[i] = sma[i] + multiplier*stdDev
		lower[i] = sma[i] - multiplier*stdDev
	}
	
	return upper, middle, lower
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func sqrt(x float64) float64 {
	// Simple Newton-Raphson square root
	if x < 0 {
		return 0
	}
	if x == 0 {
		return 0
	}
	
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}