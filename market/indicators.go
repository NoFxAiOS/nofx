package market

import (
	"github.com/markcheno/go-talib"
)

// CalculateEMA 使用TA-Lib计算EMA
// 返回当前值（最后一个值）
func CalculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算EMA
	emaValues := talib.Ema(closes, period)
	if len(emaValues) == 0 {
		return 0
	}

	// 返回最后一个值（当前值）
	return emaValues[len(emaValues)-1]
}

// CalculateEMASeries 使用TA-Lib计算EMA序列
// 返回整个EMA序列
func CalculateEMASeries(klines []Kline, period int) []float64 {
	if len(klines) < period {
		return []float64{}
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算EMA
	return talib.Ema(closes, period)
}

// CalculateMACD 使用TA-Lib计算MACD
// 返回MACD线、信号线、柱状图的当前值
func CalculateMACD(klines []Kline, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, histogram float64) {
	minPeriod := slowPeriod + signalPeriod
	if len(klines) < minPeriod {
		return 0, 0, 0
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算MACD
	macdValues, signalValues, histogramValues := talib.Macd(closes, fastPeriod, slowPeriod, signalPeriod)

	if len(macdValues) == 0 {
		return 0, 0, 0
	}

	// 返回最后一个值（当前值）
	macd = macdValues[len(macdValues)-1]
	if len(signalValues) > 0 {
		signal = signalValues[len(signalValues)-1]
	}
	if len(histogramValues) > 0 {
		histogram = histogramValues[len(histogramValues)-1]
	}

	return macd, signal, histogram
}

// CalculateMACDSeries 使用TA-Lib计算MACD序列
// 返回MACD线、信号线、柱状图的整个序列
func CalculateMACDSeries(klines []Kline, fastPeriod, slowPeriod, signalPeriod int) (macd, signal, histogram []float64) {
	minPeriod := slowPeriod + signalPeriod
	if len(klines) < minPeriod {
		return []float64{}, []float64{}, []float64{}
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算MACD
	macdValues, signalValues, histogramValues := talib.Macd(closes, fastPeriod, slowPeriod, signalPeriod)
	return macdValues, signalValues, histogramValues
}

// CalculateMACDLine 使用TA-Lib计算MACD线（仅返回MACD线，兼容旧代码）
func CalculateMACDLine(klines []Kline, fastPeriod, slowPeriod int) float64 {
	macd, _, _ := CalculateMACD(klines, fastPeriod, slowPeriod, 9)
	return macd
}

// CalculateRSI 使用TA-Lib计算RSI
// 返回当前值（最后一个值）
func CalculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算RSI
	rsiValues := talib.Rsi(closes, period)
	if len(rsiValues) == 0 {
		return 0
	}

	// 返回最后一个值（当前值）
	return rsiValues[len(rsiValues)-1]
}

// CalculateRSISeries 使用TA-Lib计算RSI序列
// 返回整个RSI序列
func CalculateRSISeries(klines []Kline, period int) []float64 {
	if len(klines) <= period {
		return []float64{}
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算RSI
	return talib.Rsi(closes, period)
}

// CalculateATR 使用TA-Lib计算ATR
// 返回当前值（最后一个值）
func CalculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	// 提取高、低、收盘价数组
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	closes := make([]float64, len(klines))
	for i, k := range klines {
		highs[i] = k.High
		lows[i] = k.Low
		closes[i] = k.Close
	}

	// 使用TA-Lib计算ATR
	atrValues := talib.Atr(highs, lows, closes, period)
	if len(atrValues) == 0 {
		return 0
	}

	// 返回最后一个值（当前值）
	return atrValues[len(atrValues)-1]
}

// CalculateATRSeries 使用TA-Lib计算ATR序列
// 返回整个ATR序列
func CalculateATRSeries(klines []Kline, period int) []float64 {
	if len(klines) <= period {
		return []float64{}
	}

	// 提取高、低、收盘价数组
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	closes := make([]float64, len(klines))
	for i, k := range klines {
		highs[i] = k.High
		lows[i] = k.Low
		closes[i] = k.Close
	}

	// 使用TA-Lib计算ATR
	return talib.Atr(highs, lows, closes, period)
}

// CalculateSMA 使用TA-Lib计算SMA（简单移动平均）
// 返回当前值（最后一个值）
func CalculateSMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算SMA（MaType = 0 表示SMA）
	smaValues := talib.Ma(closes, period, talib.SMA)
	if len(smaValues) == 0 {
		return 0
	}

	// 返回最后一个值（当前值）
	return smaValues[len(smaValues)-1]
}

// CalculateSMASeries 使用TA-Lib计算SMA序列
// 返回整个SMA序列
func CalculateSMASeries(klines []Kline, period int) []float64 {
	if len(klines) < period {
		return []float64{}
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算SMA
	return talib.Ma(closes, period, talib.SMA)
}

// BollingerBandsResult 布林带计算结果
type BollingerBandsResult struct {
	Upper  float64 // 上轨
	Middle float64 // 中轨
	Lower  float64 // 下轨
}

// CalculateBollingerBands 使用TA-Lib计算布林带
// 返回当前值（最后一个值）：上轨、中轨、下轨
func CalculateBollingerBands(klines []Kline, period int, stdDev float64) (upper, middle, lower float64) {
	if len(klines) < period {
		return 0, 0, 0
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算布林带（上轨、中轨、下轨）
	upperBand, middleBand, lowerBand := talib.BBands(closes, period, stdDev, stdDev, talib.SMA)

	if len(upperBand) == 0 || len(middleBand) == 0 || len(lowerBand) == 0 {
		return 0, 0, 0
	}

	// 返回最后一个值（当前值）
	return upperBand[len(upperBand)-1], middleBand[len(middleBand)-1], lowerBand[len(lowerBand)-1]
}

// CalculateBollingerBandsSeries 使用TA-Lib计算布林带序列
// 返回整个布林带序列：上轨、中轨、下轨
func CalculateBollingerBandsSeries(klines []Kline, period int, stdDev float64) (upper, middle, lower []float64) {
	if len(klines) < period {
		return []float64{}, []float64{}, []float64{}
	}

	// 提取收盘价数组
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	// 使用TA-Lib计算布林带
	return talib.BBands(closes, period, stdDev, stdDev, talib.SMA)
}
