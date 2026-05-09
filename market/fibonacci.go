package market

import (
	"math"
)

// FibonacciLevels represents Fibonacci retracement levels
type FibonacciLevels struct {
	SwingHigh float64            `json:"swing_high"`
	SwingLow  float64            `json:"swing_low"`
	Timeframe string             `json:"timeframe"`
	Levels    map[string]float64 `json:"levels"`    // "0.236", "0.382", "0.5", "0.618", "0.786"
	Direction string             `json:"direction"` // "retracement_up" or "retracement_down"
}

// CalculateFibonacciLevels detects swing high/low and calculates Fibonacci retracement levels
func CalculateFibonacciLevels(klines []Kline, timeframe string) *FibonacciLevels {
	if len(klines) < 10 {
		return nil
	}

	// Use up to 50 recent candles
	lookback := 50
	if len(klines) < lookback {
		lookback = len(klines)
	}
	recent := klines[len(klines)-lookback:]

	// Find swing high and swing low
	swingHigh := recent[0].High
	swingLow := recent[0].Low
	swingHighIdx := 0
	swingLowIdx := 0

	for i, k := range recent {
		if k.High > swingHigh {
			swingHigh = k.High
			swingHighIdx = i
		}
		if k.Low < swingLow {
			swingLow = k.Low
			swingLowIdx = i
		}
	}

	if swingHigh == swingLow {
		return nil
	}

	diff := swingHigh - swingLow
	ratios := []string{"0.236", "0.382", "0.5", "0.618", "0.786"}
	ratioValues := []float64{0.236, 0.382, 0.5, 0.618, 0.786}

	levels := make(map[string]float64, len(ratios))

	// Direction: if swing low came before swing high, price moved up → retracement_down
	// If swing high came before swing low, price moved down → retracement_up
	var direction string
	if swingHighIdx > swingLowIdx {
		// Uptrend → retracement from high downward
		direction = "retracement_down"
		for i, r := range ratioValues {
			levels[ratios[i]] = swingHigh - diff*r
		}
	} else {
		// Downtrend → retracement from low upward
		direction = "retracement_up"
		for i, r := range ratioValues {
			levels[ratios[i]] = swingLow + diff*r
		}
	}

	return &FibonacciLevels{
		SwingHigh: swingHigh,
		SwingLow:  swingLow,
		Timeframe: timeframe,
		Levels:    levels,
		Direction: direction,
	}
}

// findSwingHighs finds local maxima with n-bar lookback
func findSwingHighs(klines []Kline, lookback int) []int {
	var indices []int
	for i := lookback; i < len(klines)-lookback; i++ {
		isHigh := true
		for j := 1; j <= lookback; j++ {
			if klines[i].High <= klines[i-j].High || klines[i].High <= klines[i+j].High {
				isHigh = false
				break
			}
		}
		if isHigh {
			indices = append(indices, i)
		}
	}
	return indices
}

// findSwingLows finds local minima with n-bar lookback
func findSwingLows(klines []Kline, lookback int) []int {
	var indices []int
	for i := lookback; i < len(klines)-lookback; i++ {
		isLow := true
		for j := 1; j <= lookback; j++ {
			if klines[i].Low >= klines[i-j].Low || klines[i].Low >= klines[i+j].Low {
				isLow = false
				break
			}
		}
		if isLow {
			indices = append(indices, i)
		}
	}
	return indices
}

// clusterLevels groups nearby price levels into clusters
// priceCluster represents a cluster of nearby prices
type priceCluster struct {
	Price float64
	Count int
}

func clusterLevels(prices []float64, tolerancePct float64) []priceCluster {
	if len(prices) == 0 {
		return nil
	}

	var clusters []priceCluster
	used := make([]bool, len(prices))

	for i := 0; i < len(prices); i++ {
		if used[i] {
			continue
		}
		sum := prices[i]
		count := 1
		used[i] = true

		for j := i + 1; j < len(prices); j++ {
			if used[j] {
				continue
			}
			if math.Abs(prices[j]-prices[i])/prices[i] < tolerancePct {
				sum += prices[j]
				count++
				used[j] = true
			}
		}
		clusters = append(clusters, priceCluster{Price: sum / float64(count), Count: count})
	}
	return clusters
}
