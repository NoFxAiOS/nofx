package kernel

import (
	"fmt"
	"nofx/market"
	"nofx/store"
	"sort"
	"strings"
)

// formatSentimentDataZH formats market sentiment data (Chinese)
func formatSentimentDataZH(mdata *market.Data, indicators ...store.IndicatorConfig) string {
	// Determine which sentiment fields to show based on indicator config
	showLS := true
	showTT := true
	showTBS := true
	showDepth := true
	if len(indicators) > 0 {
		ind := indicators[0]
		showLS = ind.EnableLongShortRatio
		showTT = ind.EnableTopTraderRatio
		showTBS = ind.EnableTakerBuySellRatio
		showDepth = ind.EnableOrderBookDepth
	}

	hasData := (showLS && mdata.LongShortRatio != nil) ||
		(showTT && mdata.TopTraderRatio != nil) ||
		(showTBS && mdata.TakerBuySellRatio != nil) ||
		(showDepth && mdata.DepthImbalance != nil)
	if !hasData {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("**市场情绪**:\n")

	if showLS && mdata.LongShortRatio != nil {
		bias := "多头偏多"
		if *mdata.LongShortRatio < 1 {
			bias = "空头偏多"
		}
		sb.WriteString(fmt.Sprintf("- 多空比: %.2f (%s)\n", *mdata.LongShortRatio, bias))
	}
	if showTT && mdata.TopTraderRatio != nil {
		bias := "大户偏多"
		if *mdata.TopTraderRatio < 1 {
			bias = "大户偏空"
		}
		sb.WriteString(fmt.Sprintf("- 大户多空比: %.2f (%s)\n", *mdata.TopTraderRatio, bias))
	}
	if showTBS && mdata.TakerBuySellRatio != nil {
		bias := "买方主导"
		if *mdata.TakerBuySellRatio < 1 {
			bias = "卖方主导"
		}
		sb.WriteString(fmt.Sprintf("- 主动买卖比: %.2f (%s)\n", *mdata.TakerBuySellRatio, bias))
	}
	if showDepth && mdata.DepthImbalance != nil {
		bias := "买盘偏重, 支撑倾向"
		if *mdata.DepthImbalance < 0 {
			bias = "卖盘偏重, 压力倾向"
		}
		sb.WriteString(fmt.Sprintf("- 深度失衡: %+.2f (%s)\n", *mdata.DepthImbalance, bias))
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatSentimentDataEN formats market sentiment data (English)
func formatSentimentDataEN(mdata *market.Data, indicators ...store.IndicatorConfig) string {
	// Determine which sentiment fields to show based on indicator config
	showLS := true
	showTT := true
	showTBS := true
	showDepth := true
	if len(indicators) > 0 {
		ind := indicators[0]
		showLS = ind.EnableLongShortRatio
		showTT = ind.EnableTopTraderRatio
		showTBS = ind.EnableTakerBuySellRatio
		showDepth = ind.EnableOrderBookDepth
	}

	hasData := (showLS && mdata.LongShortRatio != nil) ||
		(showTT && mdata.TopTraderRatio != nil) ||
		(showTBS && mdata.TakerBuySellRatio != nil) ||
		(showDepth && mdata.DepthImbalance != nil)
	if !hasData {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("**Market Sentiment**:\n")

	if showLS && mdata.LongShortRatio != nil {
		bias := "more longs"
		if *mdata.LongShortRatio < 1 {
			bias = "more shorts"
		}
		sb.WriteString(fmt.Sprintf("- Long/Short Ratio: %.2f (%s)\n", *mdata.LongShortRatio, bias))
	}
	if showTT && mdata.TopTraderRatio != nil {
		bias := "top traders long-biased"
		if *mdata.TopTraderRatio < 1 {
			bias = "top traders short-biased"
		}
		sb.WriteString(fmt.Sprintf("- Top Trader L/S: %.2f (%s)\n", *mdata.TopTraderRatio, bias))
	}
	if showTBS && mdata.TakerBuySellRatio != nil {
		bias := "buyers dominant"
		if *mdata.TakerBuySellRatio < 1 {
			bias = "sellers dominant"
		}
		sb.WriteString(fmt.Sprintf("- Taker Buy/Sell: %.2f (%s)\n", *mdata.TakerBuySellRatio, bias))
	}
	if showDepth && mdata.DepthImbalance != nil {
		bias := "bid-heavy, support bias"
		if *mdata.DepthImbalance < 0 {
			bias = "ask-heavy, resistance bias"
		}
		sb.WriteString(fmt.Sprintf("- Depth Imbalance: %+.2f (%s)\n", *mdata.DepthImbalance, bias))
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatStructuralLevelsZH formats structural levels (Chinese) — evaluated version
func formatStructuralLevelsZH(mdata *market.Data) string {
	return formatStructuralLevelsEvaluated(mdata, true)
}

// formatStructuralLevelsEN formats structural levels (English) — evaluated version
func formatStructuralLevelsEN(mdata *market.Data) string {
	return formatStructuralLevelsEvaluated(mdata, false)
}

// formatStructuralLevelsEvaluated formats structural levels grouped by trading usage
func formatStructuralLevelsEvaluated(mdata *market.Data, zh bool) string {
	if len(mdata.StructuralLevels) == 0 && mdata.FibonacciLevels == nil {
		return ""
	}

	// Get ATR14 from primary timeframe data
	atr14 := extractPrimaryATR14(mdata)
	currentPrice := mdata.CurrentPrice

	// Evaluate levels for trading context (direction unknown at prompt time)
	evaluated := market.EvaluateForTrading(mdata.StructuralLevels, currentPrice, atr14, "")
	groups := market.GroupByUsage(evaluated)

	var sb strings.Builder

	if zh {
		sb.WriteString("**关键结构性价位** (按交易用途分组评估):\n")
	} else {
		sb.WriteString("**Key Structural Levels** (grouped by trading usage):\n")
	}

	// ATR context line
	if atr14 > 0 {
		atrPct := (atr14 / currentPrice) * 100
		sb.WriteString(fmt.Sprintf("- context: current_price=%s atr14=%s (%.2f%%)\n\n",
			formatAIFloat(currentPrice), formatAIFloat(atr14), atrPct))
	}

	// SL Candidates
	slGroup := groups["sl_anchor"]
	if len(slGroup) > 0 {
		limit := 4
		if len(slGroup) < limit {
			limit = len(slGroup)
		}
		label := "[SL Candidates]"
		if zh {
			label = "[止损锚点候选]"
		}
		sb.WriteString(fmt.Sprintf("- %s:\n", label))
		for _, l := range slGroup[:limit] {
			sb.WriteString(formatEvaluatedLevelRow(l, zh))
		}
	}

	// TP Candidates
	tpGroup := groups["tp_target"]
	if len(tpGroup) > 0 {
		limit := 4
		if len(tpGroup) < limit {
			limit = len(tpGroup)
		}
		label := "[TP Candidates]"
		if zh {
			label = "[止盈目标候选]"
		}
		sb.WriteString(fmt.Sprintf("- %s:\n", label))
		for _, l := range tpGroup[:limit] {
			sb.WriteString(formatEvaluatedLevelRow(l, zh))
		}
	}

	// Entry Triggers
	entryGroup := groups["entry_trigger"]
	if len(entryGroup) > 0 {
		limit := 2
		if len(entryGroup) < limit {
			limit = len(entryGroup)
		}
		label := "[Entry Triggers]"
		if zh {
			label = "[入场触发位]"
		}
		sb.WriteString(fmt.Sprintf("- %s:\n", label))
		for _, l := range entryGroup[:limit] {
			sb.WriteString(formatEvaluatedLevelRow(l, zh))
		}
	}

	// Context Only (max 3, only if there are few actionable levels)
	ctxGroup := groups["context_only"]
	if len(ctxGroup) > 0 && (len(slGroup)+len(tpGroup)+len(entryGroup)) < 4 {
		limit := 3
		if len(ctxGroup) < limit {
			limit = len(ctxGroup)
		}
		label := "[Context Only]"
		if zh {
			label = "[仅供参考]"
		}
		sb.WriteString(fmt.Sprintf("- %s:\n", label))
		for _, l := range ctxGroup[:limit] {
			sb.WriteString(formatEvaluatedLevelRow(l, zh))
		}
	}

	// Fibonacci context
	if mdata.FibonacciLevels != nil {
		fib := mdata.FibonacciLevels
		dir := fib.Direction
		if zh {
			dir = "回撤向下"
			if fib.Direction == "retracement_up" {
				dir = "回撤向上"
			}
		}
		sb.WriteString(fmt.Sprintf("- fibonacci_context: timeframe=%s swing_low=%s swing_high=%s direction=%s\n",
			fib.Timeframe, formatAIFloat(fib.SwingLow), formatAIFloat(fib.SwingHigh), dir))
		keys := sortedFibKeys(fib.Levels)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("  - fib_%s=%s\n", k, formatAIFloat(fib.Levels[k])))
		}
	}

	// Quality advisory
	hasSL := market.HasHighQualitySLCandidates(evaluated)
	hasTP := market.HasHighQualityTPCandidates(evaluated)
	if !hasSL || !hasTP {
		sb.WriteString("\n")
		if zh {
			if !hasSL {
				sb.WriteString("⚠️ 无高质量止损锚点 — 可使用 ATR-based 止损 (建议 1.5-2x ATR)。\n")
			}
			if !hasTP {
				sb.WriteString("⚠️ 无高质量止盈目标 — 可使用 ATR-based 目标或固定 RR 比。\n")
			}
		} else {
			if !hasSL {
				sb.WriteString("⚠️ No high-quality SL anchors — use ATR-based stop (suggested 1.5-2x ATR from entry).\n")
			}
			if !hasTP {
				sb.WriteString("⚠️ No high-quality TP targets — use ATR-based target or fixed RR ratio.\n")
			}
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

func formatEvaluatedLevelRow(l market.EvaluatedLevel, zh bool) string {
	source := l.Source
	if zh {
		source = translateSource(l.Source, true)
	}
	return fmt.Sprintf("  - price=%s tf=%s source=%s conf=%.0f atr_dist=%.1f quality=%s",
		formatAIFloat(l.Price), l.Timeframe, source, l.Confidence, l.ATRDistance, l.QualityGrade) +
		formatEvaluatedLevelExtra(l) + "\n"
}

func formatEvaluatedLevelExtra(l market.EvaluatedLevel) string {
	var parts []string
	if l.MultiTFCount > 0 {
		parts = append(parts, fmt.Sprintf("mtf=%d", l.MultiTFCount))
	}
	if l.TouchCount > 1 {
		parts = append(parts, fmt.Sprintf("touches=%d", l.TouchCount))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

// extractPrimaryATR14 gets ATR14 from the best available timeframe in market data
func extractPrimaryATR14(mdata *market.Data) float64 {
	if mdata.TimeframeData == nil {
		return 0
	}
	// Prefer 15m > 5m > 1h as primary ATR reference
	for _, tf := range []string{"15m", "5m", "1h", "3m", "4h"} {
		if series, ok := mdata.TimeframeData[tf]; ok && series.ATR14 > 0 {
			return series.ATR14
		}
	}
	return 0
}


func formatAIFloat(v float64) string {
	s := fmt.Sprintf("%.8f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

func formatAISignedFloat(v float64) string {
	if v > 0 {
		return "+" + formatAIFloat(v)
	}
	return formatAIFloat(v)
}

func sortedFibKeys(levels map[string]float64) []string {
	keys := make([]string, 0, len(levels))
	for k := range levels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func translateSource(source string, zh bool) string {
	if !zh {
		return source
	}
	switch source {
	case "swing_point":
		return "波段点"
	case "volume_cluster":
		return "成交量集中区"
	case "fibonacci":
		return "斐波那契"
	default:
		return source
	}
}
