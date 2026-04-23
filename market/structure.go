package market

import (
	"math"
	"sort"
)

// StructuralLevel represents a detected support or resistance level
type StructuralLevel struct {
	Price     float64 `json:"price"`
	Type      string  `json:"type"`      // "support" or "resistance"
	Timeframe string  `json:"timeframe"`
	Strength  int     `json:"strength"`  // 1-5 based on touch count
	Source    string  `json:"source"`    // "swing_point", "volume_cluster", "fibonacci"
}

// DetectStructuralLevels detects support and resistance levels from kline data
func DetectStructuralLevels(klines []Kline, currentPrice float64, timeframe string) []StructuralLevel {
	if len(klines) < 10 {
		return nil
	}

	var levels []StructuralLevel
	tolerancePct := 0.005 // 0.5% cluster tolerance

	// 1. Swing point detection
	lookback := 5
	swingHighs := findSwingHighs(klines, lookback)
	swingLows := findSwingLows(klines, lookback)

	// Collect swing high prices
	var highPrices []float64
	for _, idx := range swingHighs {
		highPrices = append(highPrices, klines[idx].High)
	}

	// Collect swing low prices
	var lowPrices []float64
	for _, idx := range swingLows {
		lowPrices = append(lowPrices, klines[idx].Low)
	}

	// Cluster high prices → resistance levels
	highClusters := clusterLevels(highPrices, tolerancePct)
	for _, c := range highClusters {
		strength := c.Count
		if strength > 5 {
			strength = 5
		}
		levelType := "resistance"
		if c.Price < currentPrice {
			levelType = "support"
		}
		levels = append(levels, StructuralLevel{
			Price:     c.Price,
			Type:      levelType,
			Timeframe: timeframe,
			Strength:  strength,
			Source:    "swing_point",
		})
	}

	// Cluster low prices → support levels
	lowClusters := clusterLevels(lowPrices, tolerancePct)
	for _, c := range lowClusters {
		strength := c.Count
		if strength > 5 {
			strength = 5
		}
		levelType := "support"
		if c.Price > currentPrice {
			levelType = "resistance"
		}
		levels = append(levels, StructuralLevel{
			Price:     c.Price,
			Type:      levelType,
			Timeframe: timeframe,
			Strength:  strength,
			Source:    "swing_point",
		})
	}

	// 2. Volume cluster detection - find price zones with concentrated volume
	volumeLevels := detectVolumeClusters(klines, currentPrice, timeframe, tolerancePct)
	levels = append(levels, volumeLevels...)

	// 3. Add fibonacci-based levels
	fib := CalculateFibonacciLevels(klines, timeframe)
	if fib != nil {
		for _, price := range fib.Levels {
			levelType := "support"
			if price > currentPrice {
				levelType = "resistance"
			}
			levels = append(levels, StructuralLevel{
				Price:     price,
				Type:      levelType,
				Timeframe: timeframe,
				Strength:  2, // fibonacci levels get moderate strength
				Source:    "fibonacci",
			})
		}
	}

	// Deduplicate and merge nearby levels
	levels = mergeLevels(levels, tolerancePct)

	// Sort by distance from current price
	sort.Slice(levels, func(i, j int) bool {
		distI := math.Abs(levels[i].Price - currentPrice)
		distJ := math.Abs(levels[j].Price - currentPrice)
		return distI < distJ
	})

	// Limit to top 20 levels
	if len(levels) > 20 {
		levels = levels[:20]
	}

	return levels
}

// detectVolumeClusters finds price zones with high volume concentration
func detectVolumeClusters(klines []Kline, currentPrice float64, timeframe string, tolerancePct float64) []StructuralLevel {
	if len(klines) < 20 {
		return nil
	}

	// Calculate average volume
	var totalVol float64
	for _, k := range klines {
		totalVol += k.Volume
	}
	avgVol := totalVol / float64(len(klines))

	// Collect high-volume price points (VWAP of high-volume candles)
	var hvPrices []float64
	for _, k := range klines {
		if k.Volume > avgVol*1.5 {
			midPrice := (k.High + k.Low) / 2
			hvPrices = append(hvPrices, midPrice)
		}
	}

	clusters := clusterLevels(hvPrices, tolerancePct)
	var levels []StructuralLevel
	for _, c := range clusters {
		if c.Count < 2 {
			continue
		}
		strength := c.Count
		if strength > 5 {
			strength = 5
		}
		levelType := "support"
		if c.Price > currentPrice {
			levelType = "resistance"
		}
		levels = append(levels, StructuralLevel{
			Price:     c.Price,
			Type:      levelType,
			Timeframe: timeframe,
			Strength:  strength,
			Source:    "volume_cluster",
		})
	}
	return levels
}

// mergeLevels deduplicates levels that are within tolerance of each other
func mergeLevels(levels []StructuralLevel, tolerancePct float64) []StructuralLevel {
	if len(levels) <= 1 {
		return levels
	}

	sort.Slice(levels, func(i, j int) bool {
		return levels[i].Price < levels[j].Price
	})

	var merged []StructuralLevel
	merged = append(merged, levels[0])

	for i := 1; i < len(levels); i++ {
		last := &merged[len(merged)-1]
		if math.Abs(levels[i].Price-last.Price)/last.Price < tolerancePct {
			// Merge: keep higher strength, combine sources
			if levels[i].Strength > last.Strength {
				last.Strength = levels[i].Strength
			}
			if levels[i].Source != last.Source {
				last.Source = last.Source + "+" + levels[i].Source
			}
			// Average the price
			last.Price = (last.Price + levels[i].Price) / 2
		} else {
			merged = append(merged, levels[i])
		}
	}
	return merged
}
