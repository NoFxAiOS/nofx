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
	return formatStructuralLevels(mdata, true)
}

// formatStructuralLevelsEN formats structural levels (English)
func formatStructuralLevelsEN(mdata *market.Data) string {
	return formatStructuralLevels(mdata, false)
}

func formatStructuralLevels(mdata *market.Data, zh bool) string {
	if len(mdata.StructuralLevels) == 0 && mdata.FibonacciLevels == nil {
		return ""
	}

	var sb strings.Builder

	if zh {
		sb.WriteString("**关键结构性价位** (自动检测, 机器可读摘要; 请结合自身分析验证):\n")
	} else {
		sb.WriteString("**Key Structural Levels** (auto-detected, machine-readable summary; verify with your analysis):\n")
	}

	if len(mdata.StructuralLevels) > 0 {
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
			label := "support_levels"
			if zh {
				label = "support_levels_支撑"
			}
			sb.WriteString(formatStructuralLevelRows(label, supports, zh))
		}

		if len(resistances) > 0 {
			sort.Slice(resistances, func(i, j int) bool { return resistances[i].Price < resistances[j].Price })
			label := "resistance_levels"
			if zh {
				label = "resistance_levels_阻力"
			}
			sb.WriteString(formatStructuralLevelRows(label, resistances, zh))
		}
	}

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

	if zh {
		sb.WriteString("⚠️ 以上价位为自动计算、无千分位逗号的机器可读格式；请交叉验证，不要把相邻价位合并成一个数字。\n\n")
	} else {
		sb.WriteString("⚠️ These levels are auto-calculated in machine-readable format without thousands separators; cross-validate and never merge adjacent levels into one number.\n\n")
	}
	return sb.String()
}

func formatStructuralLevelRows(label string, levels []market.StructuralLevel, zh bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("- %s:\n", label))
	for i, level := range levels {
		source := level.Source
		if zh {
			source = translateSource(level.Source, true)
		}
		sb.WriteString(fmt.Sprintf("  - level_%d_price=%s timeframe=%s source=%s strength=%d\n", i+1, formatAIFloat(level.Price), level.Timeframe, source, level.Strength))
	}
	return sb.String()
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
