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

// formatStructuralLevelsZH formats structural levels (Chinese)
func formatStructuralLevelsZH(mdata *market.Data) string {
	if len(mdata.StructuralLevels) == 0 && mdata.FibonacciLevels == nil {
		return ""
	}

	var sb strings.Builder

	if len(mdata.StructuralLevels) > 0 {
		sb.WriteString("**关键结构性价位** (自动检测, 请结合自身分析验证):\n")

		var supports, resistances []market.StructuralLevel
		for _, l := range mdata.StructuralLevels {
			if l.Type == "support" {
				supports = append(supports, l)
			} else {
				resistances = append(resistances, l)
			}
		}

		if len(supports) > 0 {
			sort.Slice(supports, func(i, j int) bool { return supports[i].Price > supports[j].Price })
			parts := make([]string, 0, len(supports))
			for _, s := range supports {
				parts = append(parts, fmt.Sprintf("%.4f (%s %s, 强度 %d)", s.Price, s.Timeframe, translateSource(s.Source, true), s.Strength))
			}
			sb.WriteString(fmt.Sprintf("- 支撑: %s\n", strings.Join(parts, " | ")))
		}

		if len(resistances) > 0 {
			sort.Slice(resistances, func(i, j int) bool { return resistances[i].Price < resistances[j].Price })
			parts := make([]string, 0, len(resistances))
			for _, r := range resistances {
				parts = append(parts, fmt.Sprintf("%.4f (%s %s, 强度 %d)", r.Price, r.Timeframe, translateSource(r.Source, true), r.Strength))
			}
			sb.WriteString(fmt.Sprintf("- 阻力: %s\n", strings.Join(parts, " | ")))
		}

		sb.WriteString("\n")
	}

	if mdata.FibonacciLevels != nil {
		fib := mdata.FibonacciLevels
		dirZH := "回撤向下"
		if fib.Direction == "retracement_up" {
			dirZH = "回撤向上"
		}
		sb.WriteString(fmt.Sprintf("**斐波那契水平** (%s, 波动 %.4f→%.4f, %s):\n", fib.Timeframe, fib.SwingLow, fib.SwingHigh, dirZH))

		keys := sortedFibKeys(fib.Levels)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %.4f", k, fib.Levels[k]))
		}
		sb.WriteString(fmt.Sprintf("- %s\n", strings.Join(parts, " | ")))
		sb.WriteString("\n")
	}

	sb.WriteString("⚠️ 以上价位为自动计算, 仅供参考。请结合自身图表分析交叉验证。\n\n")
	return sb.String()
}

// formatStructuralLevelsEN formats structural levels (English)
func formatStructuralLevelsEN(mdata *market.Data) string {
	if len(mdata.StructuralLevels) == 0 && mdata.FibonacciLevels == nil {
		return ""
	}

	var sb strings.Builder

	if len(mdata.StructuralLevels) > 0 {
		sb.WriteString("**Key Structural Levels** (auto-detected, verify with your analysis):\n")

		var supports, resistances []market.StructuralLevel
		for _, l := range mdata.StructuralLevels {
			if l.Type == "support" {
				supports = append(supports, l)
			} else {
				resistances = append(resistances, l)
			}
		}

		if len(supports) > 0 {
			sort.Slice(supports, func(i, j int) bool { return supports[i].Price > supports[j].Price })
			parts := make([]string, 0, len(supports))
			for _, s := range supports {
				parts = append(parts, fmt.Sprintf("%.4f (%s %s, strength %d)", s.Price, s.Timeframe, s.Source, s.Strength))
			}
			sb.WriteString(fmt.Sprintf("- Support: %s\n", strings.Join(parts, " | ")))
		}

		if len(resistances) > 0 {
			sort.Slice(resistances, func(i, j int) bool { return resistances[i].Price < resistances[j].Price })
			parts := make([]string, 0, len(resistances))
			for _, r := range resistances {
				parts = append(parts, fmt.Sprintf("%.4f (%s %s, strength %d)", r.Price, r.Timeframe, r.Source, r.Strength))
			}
			sb.WriteString(fmt.Sprintf("- Resistance: %s\n", strings.Join(parts, " | ")))
		}

		sb.WriteString("\n")
	}

	if mdata.FibonacciLevels != nil {
		fib := mdata.FibonacciLevels
		sb.WriteString(fmt.Sprintf("**Fibonacci Levels** (%s, swing %.4f→%.4f, %s):\n", fib.Timeframe, fib.SwingLow, fib.SwingHigh, fib.Direction))

		keys := sortedFibKeys(fib.Levels)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %.4f", k, fib.Levels[k]))
		}
		sb.WriteString(fmt.Sprintf("- %s\n", strings.Join(parts, " | ")))
		sb.WriteString("\n")
	}

	sb.WriteString("⚠️ These levels are auto-calculated. Use them as reference anchors for your analysis, not as absolute truth. Cross-validate with your own chart reading.\n\n")
	return sb.String()
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
