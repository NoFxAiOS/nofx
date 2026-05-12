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
	Source    string  `json:"source"`    // "swing_point", "volume_cluster", "fibonacci", or "+" combos

	// Enhanced confidence scoring
	VolumeScore   float64 `json:"volume_score,omitempty"`    // 0-1: high-volume candle ratio at this level
	RecencyScore  float64 `json:"recency_score,omitempty"`   // 0-1: how recently this level was touched
	TouchCount    int     `json:"touch_count,omitempty"`     // raw touch count (before strength capping)
	MultiTFCount  int     `json:"multi_tf_count,omitempty"`  // how many timeframes confirm this level
	Confidence    float64 `json:"confidence,omitempty"`      // 0-100: composite confidence score
	AvgBounceVol  float64 `json:"avg_bounce_vol,omitempty"`  // average volume of candles that bounced here
	LastTouchBars int     `json:"last_touch_bars,omitempty"` // how many bars ago the last touch occurred
}

// DetectStructuralLevels detects support and resistance levels from kline data
func DetectStructuralLevels(klines []Kline, currentPrice float64, timeframe string) []StructuralLevel {
	if len(klines) < 10 {
		return nil
	}

	var levels []StructuralLevel
	tolerancePct := 0.005 // 0.5% cluster tolerance

	// 1. Swing point detection — dynamic lookback based on timeframe
	lookback := swingLookbackForTimeframe(timeframe)
	swingHighs := findSwingHighs(klines, lookback)
	swingLows := findSwingLows(klines, lookback)

	// Collect swing high prices with indices for recency
	var highPricesIdx []priceWithIndex
	for _, idx := range swingHighs {
		highPricesIdx = append(highPricesIdx, priceWithIndex{klines[idx].High, idx})
	}
	var lowPricesIdx []priceWithIndex
	for _, idx := range swingLows {
		lowPricesIdx = append(lowPricesIdx, priceWithIndex{klines[idx].Low, idx})
	}

	totalBars := len(klines)

	// Cluster high prices → resistance levels
	highClusters := clusterLevelsEnhanced(highPricesIdx, tolerancePct, klines, totalBars)
	for _, c := range highClusters {
		strength := c.touchCount
		if strength > 5 {
			strength = 5
		}
		levelType := "resistance"
		if c.price < currentPrice {
			levelType = "support"
		}
		levels = append(levels, StructuralLevel{
			Price:         c.price,
			Type:          levelType,
			Timeframe:     timeframe,
			Strength:      strength,
			Source:        "swing_point",
			TouchCount:    c.touchCount,
			LastTouchBars: c.lastTouchBars,
			RecencyScore:  computeRecencyScore(c.lastTouchBars, totalBars),
			VolumeScore:   computeVolumeScoreAtLevel(klines, c.price, tolerancePct),
			AvgBounceVol:  computeAvgBounceVolume(klines, c.price, tolerancePct),
		})
	}

	// Cluster low prices → support levels
	lowClusters := clusterLevelsEnhanced(lowPricesIdx, tolerancePct, klines, totalBars)
	for _, c := range lowClusters {
		strength := c.touchCount
		if strength > 5 {
			strength = 5
		}
		levelType := "support"
		if c.price > currentPrice {
			levelType = "resistance"
		}
		levels = append(levels, StructuralLevel{
			Price:         c.price,
			Type:          levelType,
			Timeframe:     timeframe,
			Strength:      strength,
			Source:        "swing_point",
			TouchCount:    c.touchCount,
			LastTouchBars: c.lastTouchBars,
			RecencyScore:  computeRecencyScore(c.lastTouchBars, totalBars),
			VolumeScore:   computeVolumeScoreAtLevel(klines, c.price, tolerancePct),
			AvgBounceVol:  computeAvgBounceVolume(klines, c.price, tolerancePct),
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
				Strength:  2,
				Source:    "fibonacci",
			})
		}
	}

	// Deduplicate and merge nearby levels
	levels = mergeLevels(levels, tolerancePct)

	// Compute composite confidence for all levels
	for i := range levels {
		levels[i].Confidence = computeCompositeConfidence(&levels[i])
	}

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

// priceWithIndex pairs a price level with its kline index for recency tracking
type priceWithIndex struct {
	price float64
	index int
}

// enhancedCluster holds clustered level data with touch/recency metadata
type enhancedCluster struct {
	price         float64
	touchCount    int
	lastTouchBars int
}

// clusterLevelsEnhanced clusters prices while tracking touch count and recency
func clusterLevelsEnhanced(prices []priceWithIndex, tolerancePct float64, klines []Kline, totalBars int) []enhancedCluster {
	if len(prices) == 0 {
		return nil
	}

	var clusters []enhancedCluster
	used := make([]bool, len(prices))

	for i := 0; i < len(prices); i++ {
		if used[i] {
			continue
		}
		sum := prices[i].price
		count := 1
		maxIdx := prices[i].index
		used[i] = true

		for j := i + 1; j < len(prices); j++ {
			if used[j] {
				continue
			}
			if math.Abs(prices[j].price-prices[i].price)/prices[i].price < tolerancePct {
				sum += prices[j].price
				count++
				if prices[j].index > maxIdx {
					maxIdx = prices[j].index
				}
				used[j] = true
			}
		}

		lastTouchBars := totalBars - 1 - maxIdx
		if lastTouchBars < 0 {
			lastTouchBars = 0
		}
		clusters = append(clusters, enhancedCluster{
			price:         sum / float64(count),
			touchCount:    count,
			lastTouchBars: lastTouchBars,
		})
	}
	return clusters
}

// computeRecencyScore returns 0-1 based on how recently the level was touched.
// Uses exponential decay: score = exp(-decay * barsAgo / totalBars)
func computeRecencyScore(lastTouchBars, totalBars int) float64 {
	if totalBars <= 0 {
		return 0
	}
	ratio := float64(lastTouchBars) / float64(totalBars)
	return math.Exp(-3.0 * ratio)
}

// computeVolumeScoreAtLevel returns 0-1 indicating what fraction of candles
// touching this level had above-average volume (high-conviction touches).
func computeVolumeScoreAtLevel(klines []Kline, levelPrice, tolerancePct float64) float64 {
	if len(klines) == 0 || levelPrice <= 0 {
		return 0
	}

	var totalVol float64
	for _, k := range klines {
		totalVol += k.Volume
	}
	avgVol := totalVol / float64(len(klines))
	if avgVol <= 0 {
		return 0
	}

	touches := 0
	highVolTouches := 0
	for _, k := range klines {
		if touchesLevel(k, levelPrice, tolerancePct) {
			touches++
			if k.Volume > avgVol*1.2 {
				highVolTouches++
			}
		}
	}
	if touches == 0 {
		return 0
	}
	return float64(highVolTouches) / float64(touches)
}

// computeAvgBounceVolume calculates average volume of candles that bounced at this level
func computeAvgBounceVolume(klines []Kline, levelPrice, tolerancePct float64) float64 {
	if len(klines) < 3 || levelPrice <= 0 {
		return 0
	}

	var bounceVols []float64
	for i := 1; i < len(klines)-1; i++ {
		if !touchesLevel(klines[i], levelPrice, tolerancePct) {
			continue
		}
		// A bounce: price touches level then reverses direction
		prevDir := klines[i].Close - klines[i-1].Close
		nextDir := klines[i+1].Close - klines[i].Close
		if (prevDir < 0 && nextDir > 0) || (prevDir > 0 && nextDir < 0) {
			bounceVols = append(bounceVols, klines[i].Volume)
		}
	}
	if len(bounceVols) == 0 {
		return 0
	}
	var sum float64
	for _, v := range bounceVols {
		sum += v
	}
	return sum / float64(len(bounceVols))
}

// touchesLevel returns true if a candle's price range intersects the level within tolerance
func touchesLevel(k Kline, levelPrice, tolerancePct float64) bool {
	tolerance := levelPrice * tolerancePct
	return k.Low <= levelPrice+tolerance && k.High >= levelPrice-tolerance
}

// computeCompositeConfidence calculates a 0-100 confidence score from multiple dimensions.
// Weights: TouchCount 25%, VolumeScore 30%, RecencyScore 25%, MultiTFCount 20%
func computeCompositeConfidence(level *StructuralLevel) float64 {
	touchScore := math.Min(float64(level.TouchCount)/5.0, 1.0)

	volScore := level.VolumeScore

	recScore := level.RecencyScore

	multiTFScore := 0.0
	if level.MultiTFCount > 0 {
		multiTFScore = math.Min(float64(level.MultiTFCount)/3.0, 1.0)
	}

	composite := touchScore*25 + volScore*30 + recScore*25 + multiTFScore*20
	if composite > 100 {
		composite = 100
	}
	return math.Round(composite*10) / 10
}

// detectVolumeClusters finds price zones with high volume concentration
func detectVolumeClusters(klines []Kline, currentPrice float64, timeframe string, tolerancePct float64) []StructuralLevel {
	if len(klines) < 20 {
		return nil
	}

	var totalVol float64
	for _, k := range klines {
		totalVol += k.Volume
	}
	avgVol := totalVol / float64(len(klines))

	var hvPricesIdx []priceWithIndex
	for i, k := range klines {
		if k.Volume > avgVol*1.5 {
			midPrice := (k.High + k.Low) / 2
			hvPricesIdx = append(hvPricesIdx, priceWithIndex{midPrice, i})
		}
	}

	totalBars := len(klines)
	clusters := clusterLevelsEnhanced(hvPricesIdx, tolerancePct, klines, totalBars)
	var levels []StructuralLevel
	for _, c := range clusters {
		if c.touchCount < 2 {
			continue
		}
		strength := c.touchCount
		if strength > 5 {
			strength = 5
		}
		levelType := "support"
		if c.price > currentPrice {
			levelType = "resistance"
		}
		levels = append(levels, StructuralLevel{
			Price:         c.price,
			Type:          levelType,
			Timeframe:     timeframe,
			Strength:      strength,
			Source:        "volume_cluster",
			TouchCount:    c.touchCount,
			LastTouchBars: c.lastTouchBars,
			RecencyScore:  computeRecencyScore(c.lastTouchBars, totalBars),
			VolumeScore:   computeVolumeScoreAtLevel(klines, c.price, tolerancePct),
			AvgBounceVol:  computeAvgBounceVolume(klines, c.price, tolerancePct),
		})
	}
	return levels
}

// mergeLevels deduplicates levels that are within tolerance of each other,
// preserving the best confidence data from merged levels.
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
			if levels[i].Strength > last.Strength {
				last.Strength = levels[i].Strength
			}
			if levels[i].Source != last.Source {
				last.Source = last.Source + "+" + levels[i].Source
			}
			last.Price = (last.Price + levels[i].Price) / 2

			last.TouchCount += levels[i].TouchCount
			if levels[i].VolumeScore > last.VolumeScore {
				last.VolumeScore = levels[i].VolumeScore
			}
			if levels[i].RecencyScore > last.RecencyScore {
				last.RecencyScore = levels[i].RecencyScore
				last.LastTouchBars = levels[i].LastTouchBars
			}
			if levels[i].AvgBounceVol > last.AvgBounceVol {
				last.AvgBounceVol = levels[i].AvgBounceVol
			}
		} else {
			merged = append(merged, levels[i])
		}
	}
	return merged
}

// enrichMultiTFConfirmation iterates all timeframes' structural levels and counts
// how many different timeframes confirm each level (within 0.8% tolerance).
// This mutates the levels in-place and recomputes composite confidence.
func enrichMultiTFConfirmation(timeframeData map[string]*TimeframeSeriesData) {
	if len(timeframeData) < 2 {
		return
	}

	const mtfTolerance = 0.008 // 0.8% tolerance for cross-TF matching

	for tfA, seriesA := range timeframeData {
		if seriesA == nil {
			continue
		}
		for i := range seriesA.StructuralLevels {
			levelA := &seriesA.StructuralLevels[i]
			confirmations := 0
			for tfB, seriesB := range timeframeData {
				if tfB == tfA || seriesB == nil {
					continue
				}
				for _, levelB := range seriesB.StructuralLevels {
					if levelA.Price > 0 && math.Abs(levelB.Price-levelA.Price)/levelA.Price < mtfTolerance {
						confirmations++
						break
					}
				}
			}
			levelA.MultiTFCount = confirmations
			levelA.Confidence = computeCompositeConfidence(levelA)
		}
	}
}

// swingLookbackForTimeframe returns the number of bars to look back on each
// side when detecting swing highs/lows. Shorter timeframes use larger lookback
// to filter out noise and detect more meaningful structural pivots.
func swingLookbackForTimeframe(tf string) int {
	switch tf {
	case "1m", "3m":
		return 10
	case "5m":
		return 8
	case "15m":
		return 7
	case "30m":
		return 6
	case "1h":
		return 5
	case "4h", "1d":
		return 4
	default:
		return 7
	}
}
