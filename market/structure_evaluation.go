package market

import (
	"math"
	"sort"
)

// EvaluatedLevel wraps a StructuralLevel with trading-context evaluation
type EvaluatedLevel struct {
	StructuralLevel
	UsageLabel   string  `json:"usage_label"`   // "sl_anchor" | "tp_target" | "entry_trigger" | "context_only"
	ATRDistance   float64 `json:"atr_distance"`  // distance from current price in ATR multiples
	DistancePct   float64 `json:"distance_pct"`  // distance from current price as percentage
	QualityGrade string  `json:"quality_grade"` // "high" | "medium" | "low"
}

// EvaluateForTrading evaluates structural levels for trading context.
// side: "long", "short", or "" (unknown direction — evaluate both)
func EvaluateForTrading(levels []StructuralLevel, currentPrice, atr14 float64, side string) []EvaluatedLevel {
	if len(levels) == 0 || currentPrice <= 0 {
		return nil
	}
	if atr14 <= 0 {
		atr14 = currentPrice * 0.01
	}

	result := make([]EvaluatedLevel, 0, len(levels))
	for _, l := range levels {
		dist := l.Price - currentPrice
		absDist := math.Abs(dist)
		atrDist := absDist / atr14
		distPct := (absDist / currentPrice) * 100

		el := EvaluatedLevel{
			StructuralLevel: l,
			ATRDistance:      math.Round(atrDist*100) / 100,
			DistancePct:      math.Round(distPct*100) / 100,
			QualityGrade:    computeQualityGrade(l),
		}
		el.UsageLabel = classifyUsageLabel(l, currentPrice, atr14, dist, atrDist, side)
		result = append(result, el)
	}

	return result
}

// GroupByUsage groups evaluated levels by their usage label
func GroupByUsage(levels []EvaluatedLevel) map[string][]EvaluatedLevel {
	groups := map[string][]EvaluatedLevel{
		"sl_anchor":     {},
		"tp_target":     {},
		"entry_trigger": {},
		"context_only":  {},
	}
	for _, l := range levels {
		groups[l.UsageLabel] = append(groups[l.UsageLabel], l)
	}
	// Sort each group by proximity (ATR distance ascending)
	for k := range groups {
		sort.Slice(groups[k], func(i, j int) bool {
			return groups[k][i].ATRDistance < groups[k][j].ATRDistance
		})
	}
	return groups
}

// FilterRecommended returns only levels with quality >= medium and a non-context usage
func FilterRecommended(levels []EvaluatedLevel) []EvaluatedLevel {
	var out []EvaluatedLevel
	for _, l := range levels {
		if l.UsageLabel != "context_only" && l.QualityGrade != "low" {
			out = append(out, l)
		}
	}
	return out
}

// HasHighQualitySLCandidates checks if there are any medium+ quality SL anchors
func HasHighQualitySLCandidates(levels []EvaluatedLevel) bool {
	for _, l := range levels {
		if l.UsageLabel == "sl_anchor" && l.QualityGrade != "low" {
			return true
		}
	}
	return false
}

// HasHighQualityTPCandidates checks if there are any medium+ quality TP targets
func HasHighQualityTPCandidates(levels []EvaluatedLevel) bool {
	for _, l := range levels {
		if l.UsageLabel == "tp_target" && l.QualityGrade != "low" {
			return true
		}
	}
	return false
}

func classifyUsageLabel(l StructuralLevel, currentPrice, atr14, dist, atrDist float64, side string) string {
	isBelow := dist < 0
	isAbove := dist > 0

	// Entry trigger: very close to current price with decent confidence
	if atrDist < 0.5 && l.Confidence >= 50 {
		return "entry_trigger"
	}

	// Too far away — context only
	if atrDist > 6 {
		return "context_only"
	}

	// Low confidence without multi-TF confirmation — context only
	if l.Confidence < 25 && l.MultiTFCount == 0 {
		return "context_only"
	}

	switch side {
	case "long":
		if l.Type == "support" && isBelow && atrDist >= 0.3 && atrDist <= 4.0 && l.Confidence >= 30 {
			return "sl_anchor"
		}
		if (l.Type == "resistance" || l.Source == "fibonacci") && isAbove && atrDist >= 0.8 && l.Confidence >= 25 {
			return "tp_target"
		}
	case "short":
		if l.Type == "resistance" && isAbove && atrDist >= 0.3 && atrDist <= 4.0 && l.Confidence >= 30 {
			return "sl_anchor"
		}
		if (l.Type == "support" || l.Source == "fibonacci") && isBelow && atrDist >= 0.8 && l.Confidence >= 25 {
			return "tp_target"
		}
	default:
		// Unknown direction: both support below and resistance above can serve as SL
		if l.Type == "support" && isBelow && atrDist >= 0.3 && atrDist <= 4.0 && l.Confidence >= 30 {
			return "sl_anchor"
		}
		if l.Type == "resistance" && isAbove && atrDist >= 0.3 && atrDist <= 4.0 && l.Confidence >= 30 {
			return "sl_anchor"
		}
		// TP: anything at reasonable distance with decent confidence
		if atrDist >= 0.8 && atrDist <= 6.0 && l.Confidence >= 25 {
			return "tp_target"
		}
	}

	return "context_only"
}

func computeQualityGrade(l StructuralLevel) string {
	if l.Confidence >= 50 && l.MultiTFCount >= 1 {
		return "high"
	}
	if l.Confidence >= 30 || l.MultiTFCount >= 1 {
		return "medium"
	}
	return "low"
}

// GenerateFibExtensionLevels generates fibonacci extension levels below swing low
// (for downtrend TP targets) or above swing high (for uptrend TP targets).
// These provide structural targets when price enters uncharted territory.
func GenerateFibExtensionLevels(fib *FibonacciLevels, currentPrice float64, timeframe string) []StructuralLevel {
	if fib == nil || fib.SwingHigh <= fib.SwingLow {
		return nil
	}

	diff := fib.SwingHigh - fib.SwingLow
	var levels []StructuralLevel

	if fib.Direction == "retracement_up" && currentPrice < fib.SwingLow {
		// Price broke below swing low — generate extension targets below
		extensions := []struct {
			ratio float64
			name  string
		}{
			{1.272, "fib_ext_1.272"},
			{1.618, "fib_ext_1.618"},
			{2.0, "fib_ext_2.0"},
		}
		for _, ext := range extensions {
			price := fib.SwingHigh - diff*ext.ratio
			if price > 0 && price < currentPrice {
				levels = append(levels, StructuralLevel{
					Price:      price,
					Type:       "support",
					Timeframe:  timeframe,
					Strength:   2,
					Source:     "fibonacci_extension",
					Confidence: 40,
				})
			}
		}
	} else if fib.Direction == "retracement_down" && currentPrice > fib.SwingHigh {
		// Price broke above swing high — generate extension targets above
		extensions := []struct {
			ratio float64
			name  string
		}{
			{1.272, "fib_ext_1.272"},
			{1.618, "fib_ext_1.618"},
			{2.0, "fib_ext_2.0"},
		}
		for _, ext := range extensions {
			price := fib.SwingLow + diff*ext.ratio
			if price > currentPrice {
				levels = append(levels, StructuralLevel{
					Price:      price,
					Type:       "resistance",
					Timeframe:  timeframe,
					Strength:   2,
					Source:     "fibonacci_extension",
					Confidence: 40,
				})
			}
		}
	}

	return levels
}
