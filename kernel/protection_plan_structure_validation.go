package kernel

import (
	"fmt"
	"math"
	"strings"

	"nofx/logger"
)

func validateAIProtectionPlanCompletenessAndStructure(d Decision) error {
	if d.Action != "open_long" && d.Action != "open_short" {
		return nil
	}
	if d.ProtectionPlan == nil {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(d.ProtectionPlan.Mode))
	if mode == "combined" || mode == "ladder" {
		if err := validateAILadderRulesCompleteness(d); err != nil {
			return err
		}
	}
	if mode == "combined" || mode == "drawdown" {
		if err := validateAIDrawdownRulesCompleteness(d); err != nil {
			return err
		}
	}
	return nil
}

func validateAILadderRulesCompleteness(d Decision) error {
	primary := ""
	if d.EntryProtection != nil {
		primary = strings.TrimSpace(d.EntryProtection.TimeframeContext.Primary)
	}
	var stopLegs []AIProtectionLadderRule
	for i, rule := range d.ProtectionPlan.LadderRules {
		anchorText := strings.TrimSpace(rule.StructuralAnchor + " " + rule.StopLossAnchor + " " + rule.TakeProfitAnchor)
		if anchorText == "" {
			return fmt.Errorf("protection_plan.ladder_rules[%d] requires structural_anchor/stop_loss_anchor/take_profit_anchor; AI ladder fallback is not allowed", i)
		}
		if primary != "" && !strings.Contains(anchorText, primary) {
			anchorLower := strings.ToLower(anchorText)
			if strings.Contains(anchorLower, "higher") || strings.Contains(anchorLower, "4h") || strings.Contains(anchorLower, "1d") {
				return fmt.Errorf("protection_plan.ladder_rules[%d] must anchor to primary timeframe %s; ladder must not drift to higher timeframe", i, primary)
			}
		}
		if rule.StopLossCloseRatioPct > 0 && rule.StopLossPrice <= 0 {
			return fmt.Errorf("protection_plan.ladder_rules[%d] requires absolute stop_loss_price; percent-only AI ladder fallback is not allowed", i)
		}
		if rule.TakeProfitCloseRatioPct > 0 && rule.TakeProfitPrice <= 0 {
			return fmt.Errorf("protection_plan.ladder_rules[%d] requires absolute take_profit_price; percent-only AI ladder fallback is not allowed", i)
		}
		if rule.TakeProfitPrice > 0 && rule.TakeProfitCloseRatioPct <= 0 {
			return fmt.Errorf("protection_plan.ladder_rules[%d] take_profit_price requires positive take_profit_close_ratio_pct", i)
		}
		if rule.StopLossPrice > 0 && rule.StopLossCloseRatioPct <= 0 {
			return fmt.Errorf("protection_plan.ladder_rules[%d] stop_loss_price requires positive stop_loss_close_ratio_pct", i)
		}
		if d.Action == "open_long" {
			if rule.TakeProfitPrice > 0 && rule.TakeProfitPrice <= entryPriceForProtectionValidation(d) {
				return fmt.Errorf("protection_plan.ladder_rules[%d] take_profit_price must be above entry for long", i)
			}
			if rule.StopLossPrice > 0 && rule.StopLossPrice >= entryPriceForProtectionValidation(d) {
				return fmt.Errorf("protection_plan.ladder_rules[%d] stop_loss_price must be below entry for long", i)
			}
		}
		if d.Action == "open_short" {
			if rule.TakeProfitPrice > 0 && rule.TakeProfitPrice >= entryPriceForProtectionValidation(d) {
				return fmt.Errorf("protection_plan.ladder_rules[%d] take_profit_price must be below entry for short", i)
			}
			if rule.StopLossPrice > 0 && rule.StopLossPrice <= entryPriceForProtectionValidation(d) {
				return fmt.Errorf("protection_plan.ladder_rules[%d] stop_loss_price must be above entry for short", i)
			}
		}
		if rule.VolatilityBufferPct <= 0 && strings.TrimSpace(rule.VolatilityBufferReason) == "" {
			return fmt.Errorf("protection_plan.ladder_rules[%d] requires volatility_buffer_pct or volatility_buffer_reason", i)
		}
		if rule.StopLossPrice > 0 && rule.StopLossCloseRatioPct > 0 {
			stopLegs = append(stopLegs, rule)
		}
	}
	if err := validateAILadderStopDistribution(d, stopLegs); err != nil {
		return err
	}
	return nil
}

func validateAILadderStopDistribution(d Decision, stopLegs []AIProtectionLadderRule) error {
	if len(stopLegs) <= 1 {
		return nil
	}
	entry := entryPriceForProtectionValidation(d)
	if entry <= 0 {
		return nil
	}
	total := 0.0
	near := 0.0
	far := 0.0
	nearestStop := stopLegs[0].StopLossPrice
	for _, leg := range stopLegs[1:] {
		if math.Abs(leg.StopLossPrice-entry) < math.Abs(nearestStop-entry) {
			nearestStop = leg.StopLossPrice
		}
	}
	for i := range stopLegs {
		leg := stopLegs[i]
		total += leg.StopLossCloseRatioPct
		for j := i + 1; j < len(stopLegs); j++ {
			other := stopLegs[j]
			base := math.Max(math.Abs(leg.StopLossPrice), math.Abs(other.StopLossPrice))
			if base > 0 && math.Abs(leg.StopLossPrice-other.StopLossPrice)/base <= 0.001 {
				return fmt.Errorf("protection_plan.ladder_rules stop_loss_price tiers must be distinct; %.8f and %.8f are near-identical", leg.StopLossPrice, other.StopLossPrice)
			}
		}
		base := math.Max(math.Abs(leg.StopLossPrice), math.Abs(nearestStop))
		if base > 0 && math.Abs(leg.StopLossPrice-nearestStop)/base <= 0.008 {
			near += leg.StopLossCloseRatioPct
		} else {
			far += leg.StopLossCloseRatioPct
		}
	}
	if total <= 0 {
		return nil
	}
	if near < math.Min(75, total) {
		return fmt.Errorf("protection_plan ladder SL must protect at least 75%% near the nearest primary structure; got %.1f%% near", near)
	}
	if far > 25.0001 {
		return fmt.Errorf("protection_plan far ladder SL tier must be <=25%% size; got %.1f%%", far)
	}
	return nil
}

func entryPriceForProtectionValidation(d Decision) float64 {
	if d.EntryProtection != nil && d.EntryProtection.RiskReward.Entry > 0 {
		return d.EntryProtection.RiskReward.Entry
	}
	return 0
}

func validateAIDrawdownRulesCompleteness(d Decision) error {
	allowedTF := allowedDrawdownTimeframes(d.EntryProtection)
	for i, rule := range d.ProtectionPlan.DrawdownRules {
		tf := strings.TrimSpace(rule.Timeframe)
		// Normalize combined timeframe format (e.g., "15m/1h" -> "15m")
		if strings.Contains(tf, "/") {
			parts := strings.Split(tf, "/")
			normalized := strings.TrimSpace(parts[0])
			logger.Warnf("🟡 Drawdown rule[%d] timeframe '%s' contains combined format, normalized to '%s'", i, tf, normalized)
			tf = normalized
		}
		if tf == "" {
			if strings.TrimSpace(rule.ReasonAnchor) == "" {
				return fmt.Errorf("protection_plan.drawdown_rules[%d] requires timeframe or reason_anchor timeframe reference", i)
			}
		} else if len(allowedTF) > 0 {
			if _, ok := allowedTF[tf]; !ok {
				return fmt.Errorf("protection_plan.drawdown_rules[%d] timeframe %s not in entry timeframe context", i, tf)
			}
		}
		if strings.TrimSpace(rule.ReasonAnchor) == "" {
			return fmt.Errorf("protection_plan.drawdown_rules[%d] requires reason_anchor; AI drawdown fallback is not allowed", i)
		}
		if rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 || rule.CloseRatioPct <= 0 {
			return fmt.Errorf("protection_plan.drawdown_rules[%d] requires positive min_profit_pct/max_drawdown_pct/close_ratio_pct", i)
		}
	}
	return nil
}

func allowedDrawdownTimeframes(rationale *AIEntryProtectionRationale) map[string]struct{} {
	if rationale == nil {
		return nil
	}
	out := map[string]struct{}{}
	if tf := strings.TrimSpace(rationale.TimeframeContext.Primary); tf != "" {
		out[tf] = struct{}{}
	}
	for _, tf := range rationale.TimeframeContext.Lower {
		if tf = strings.TrimSpace(tf); tf != "" {
			out[tf] = struct{}{}
		}
	}
	for _, tf := range rationale.TimeframeContext.Higher {
		if tf = strings.TrimSpace(tf); tf != "" {
			out[tf] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
