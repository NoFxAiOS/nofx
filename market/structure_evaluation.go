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

// HasHighQualitySLCandidates checks if there are any high/medium quality SL anchors
func HasHighQualitySLCandidates(levels []EvaluatedLevel) bool {
	for _, l := range levels {
		if l.UsageLabel == "sl_anchor" && l.QualityGrade != "low" {
			return true
		}
	}
	return false
}

// HasHighQualityTPCandidates checks if there are any high/medium quality TP targets
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
		// For long: support below = SL anchor, resistance above = TP target
		if l.Type == "support" && isBelow && atrDist >= 0.5 && atrDist <= 3.5 && l.Confidence >= 35 {
			return "sl_anchor"
		}
		if (l.Type == "resistance" || l.Source == "fibonacci") && isAbove && atrDist >= 0.8 && l.Confidence >= 30 {
			return "tp_target"
		}
	case "short":
		// For short: resistance above = SL anchor, support below = TP target
		if l.Type == "resistance" && isAbove && atrDist >= 0.5 && atrDist <= 3.5 && l.Confidence >= 35 {
			return "sl_anchor"
		}
		if (l.Type == "support" || l.Source == "fibonacci") && isBelow && atrDist >= 0.8 && l.Confidence >= 30 {
			return "tp_target"
		}
	default:
		// Unknown direction: classify by type and distance
		if l.Type == "support" && isBelow && atrDist >= 0.5 && atrDist <= 3.5 && l.Confidence >= 35 {
			return "sl_anchor"
		}
		if l.Type == "resistance" && isAbove && atrDist >= 0.5 && atrDist <= 3.5 && l.Confidence >= 35 {
			return "sl_anchor"
		}
		if atrDist >= 1.0 && atrDist <= 6.0 && l.Confidence >= 30 {
			return "tp_target"
		}
	}

	return "context_only"
}

func computeQualityGrade(l StructuralLevel) string {
	if l.Confidence >= 60 && l.MultiTFCount >= 2 {
		return "high"
	}
	if l.Confidence >= 35 || l.MultiTFCount >= 1 {
		return "medium"
	}
	return "low"
}
