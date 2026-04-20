package trader

import (
	"fmt"
	"math"

	"nofx/kernel"
	"nofx/store"
)

func isManualValue(src store.ProtectionValueSource) bool {
	return src.Mode == store.ProtectionValueModeManual && src.Value > 0
}

func isAIValue(src store.ProtectionValueSource) bool {
	return src.Mode == store.ProtectionValueModeAI
}

func isDisabledValue(src store.ProtectionValueSource) bool {
	return src.Mode == "" || src.Mode == store.ProtectionValueModeDisabled
}

func resolveFullTakeProfit(full store.FullTPSLConfig, aiValue float64) (pricePct float64, ok bool) {
	if !full.Enabled || full.Mode == store.ProtectionModeDisabled || isDisabledValue(full.TakeProfit) {
		return 0, false
	}
	switch full.TakeProfit.Mode {
	case store.ProtectionValueModeManual:
		pricePct = full.TakeProfit.Value
	case store.ProtectionValueModeAI:
		pricePct = aiValue
	}
	if pricePct <= 0 {
		return 0, false
	}
	return pricePct, true
}

func resolveFullStopLoss(full store.FullTPSLConfig, aiValue float64) (pricePct float64, ok bool) {
	if !full.Enabled || full.Mode == store.ProtectionModeDisabled || isDisabledValue(full.StopLoss) {
		return 0, false
	}
	switch full.StopLoss.Mode {
	case store.ProtectionValueModeManual:
		pricePct = full.StopLoss.Value
	case store.ProtectionValueModeAI:
		pricePct = aiValue
	}
	if pricePct <= 0 {
		return 0, false
	}
	return pricePct, true
}

func resolveFallbackMaxLoss(full store.FullTPSLConfig) (pricePct float64, ok bool) {
	if full.FallbackMaxLoss.Mode != store.ProtectionValueModeManual || full.FallbackMaxLoss.Value <= 0 {
		return 0, false
	}
	return full.FallbackMaxLoss.Value, true
}

func enabledManualLadderTakeProfit(ladder store.LadderTPSLConfig) bool {
	return ladder.Enabled && ladder.Mode == store.ProtectionModeManual && ladder.TakeProfitEnabled && !isDisabledValue(ladder.TakeProfitPrice) && !isDisabledValue(ladder.TakeProfitSize)
}

func enabledManualLadderStopLoss(ladder store.LadderTPSLConfig) bool {
	return ladder.Enabled && ladder.Mode == store.ProtectionModeManual && ladder.StopLossEnabled && !isDisabledValue(ladder.StopLossPrice) && !isDisabledValue(ladder.StopLossSize)
}

func resolveLadderTakeProfitRule(rule store.LadderTPSLRule, ladder store.LadderTPSLConfig, aiRule *kernel.AIProtectionLadderRule) (pricePct float64, closeRatioPct float64, ok bool) {
	if !enabledManualLadderTakeProfit(ladder) {
		return 0, 0, false
	}

	switch ladder.TakeProfitPrice.Mode {
	case store.ProtectionValueModeManual:
		pricePct = rule.TakeProfitPct
	case store.ProtectionValueModeAI:
		if aiRule != nil {
			pricePct = aiRule.TakeProfitPct
		}
	}

	switch ladder.TakeProfitSize.Mode {
	case store.ProtectionValueModeManual:
		closeRatioPct = rule.TakeProfitCloseRatioPct
	case store.ProtectionValueModeAI:
		if aiRule != nil {
			closeRatioPct = aiRule.TakeProfitCloseRatioPct
		}
	}

	if pricePct <= 0 || closeRatioPct <= 0 {
		return 0, 0, false
	}
	return pricePct, closeRatioPct, true
}

func resolveLadderStopLossRule(rule store.LadderTPSLRule, ladder store.LadderTPSLConfig, aiRule *kernel.AIProtectionLadderRule) (pricePct float64, closeRatioPct float64, ok bool) {
	if !enabledManualLadderStopLoss(ladder) {
		return 0, 0, false
	}

	switch ladder.StopLossPrice.Mode {
	case store.ProtectionValueModeManual:
		pricePct = rule.StopLossPct
	case store.ProtectionValueModeAI:
		if aiRule != nil {
			pricePct = aiRule.StopLossPct
		}
	}

	switch ladder.StopLossSize.Mode {
	case store.ProtectionValueModeManual:
		closeRatioPct = rule.StopLossCloseRatioPct
	case store.ProtectionValueModeAI:
		if aiRule != nil {
			closeRatioPct = aiRule.StopLossCloseRatioPct
		}
	}

	if pricePct <= 0 || closeRatioPct <= 0 {
		return 0, 0, false
	}
	return pricePct, closeRatioPct, true
}

// roundProtectionPrice normalizes protection prices to a stable decimal scale
// before they are compared, logged, or passed into exchange-specific formatting.
// Exchange adapters with tick-size awareness should still apply their own final
// formatting on submission.
func roundProtectionPrice(price float64) float64 {
	if price <= 0 {
		return 0
	}
	return math.Round(price*1e8) / 1e8
}

// ProtectionOrder represents one protection leg in a full or ladder protection plan.

// ProtectionOrder represents one protection leg in a full or ladder protection plan.
type ProtectionOrder struct {
	Price         float64
	CloseRatioPct float64
}

// ProtectionPlan is the normalized execution representation produced from strategy config
// (and later AI protection plans) before hitting exchange adapters.
type ProtectionPlan struct {
	Mode                 string
	NeedsStopLoss        bool
	NeedsTakeProfit      bool
	StopLossPrice        float64
	TakeProfitPrice      float64
	FallbackMaxLossPrice float64
	StopLossOrders       []ProtectionOrder
	TakeProfitOrders     []ProtectionOrder
	DrawdownRules        []store.DrawdownTakeProfitRule
	BreakEvenConfig      *store.BreakEvenStopConfig
	RequiresNativeOrders bool
	RequiresPartialClose bool
}

// mergeProtectionPlans combines multiple protection plans into a single target exchange protection set.
func mergeProtectionPlans(plans ...*ProtectionPlan) *ProtectionPlan {
	merged := &ProtectionPlan{}
	for _, plan := range plans {
		if plan == nil {
			continue
		}
		if merged.Mode == "" {
			merged.Mode = plan.Mode
		} else if plan.Mode != "" && merged.Mode != plan.Mode {
			merged.Mode = merged.Mode + "+" + plan.Mode
		}
		merged.NeedsStopLoss = merged.NeedsStopLoss || plan.NeedsStopLoss
		merged.NeedsTakeProfit = merged.NeedsTakeProfit || plan.NeedsTakeProfit
		merged.StopLossOrders = append(merged.StopLossOrders, plan.StopLossOrders...)
		merged.TakeProfitOrders = append(merged.TakeProfitOrders, plan.TakeProfitOrders...)
		if len(plan.DrawdownRules) > 0 {
			merged.DrawdownRules = append(merged.DrawdownRules, plan.DrawdownRules...)
		}
		if merged.BreakEvenConfig == nil && plan.BreakEvenConfig != nil {
			cfg := *plan.BreakEvenConfig
			merged.BreakEvenConfig = &cfg
		}
		merged.RequiresNativeOrders = merged.RequiresNativeOrders || plan.RequiresNativeOrders
		merged.RequiresPartialClose = merged.RequiresPartialClose || plan.RequiresPartialClose
		if merged.StopLossPrice == 0 && plan.StopLossPrice > 0 {
			merged.StopLossPrice = plan.StopLossPrice
		}
		if merged.TakeProfitPrice == 0 && plan.TakeProfitPrice > 0 {
			merged.TakeProfitPrice = plan.TakeProfitPrice
		}
		if merged.FallbackMaxLossPrice == 0 && plan.FallbackMaxLossPrice > 0 {
			merged.FallbackMaxLossPrice = plan.FallbackMaxLossPrice
		}
	}

	// Ladder orders win for their direction; keep full-position prices only when no ladder orders exist for that side.
	if len(merged.StopLossOrders) > 0 {
		merged.StopLossPrice = 0
	}
	if len(merged.TakeProfitOrders) > 0 {
		merged.TakeProfitPrice = 0
	}

	if !merged.NeedsStopLoss && !merged.NeedsTakeProfit && len(merged.StopLossOrders) == 0 && len(merged.TakeProfitOrders) == 0 && merged.FallbackMaxLossPrice == 0 && len(merged.DrawdownRules) == 0 {
		return nil
	}
	return merged
}

// BuildConfiguredProtectionPlan creates a normalized protection plan from strategy configuration.
// Manual strategy config materializes directly.
// AI strategy config now also materializes directly from strategy-level percentages / rules so
// selecting AI mode in Strategy Studio actually enables exchange protection execution even when
// the model omits decision.ProtectionPlan.
func (at *AutoTrader) BuildConfiguredProtectionPlan(entryPrice float64, action string) (*ProtectionPlan, error) {
	if at.config.StrategyConfig == nil {
		return nil, nil
	}

	protection := at.config.StrategyConfig.Protection
	drawdownEnabled := protection.DrawdownTakeProfit.Enabled && len(protection.DrawdownTakeProfit.Rules) > 0

	// Build ladder plan first so we know which directions it covers.
	var ladderPlan *ProtectionPlan
	if protection.LadderTPSL.Enabled {
		var err error
		switch protection.LadderTPSL.Mode {
		case store.ProtectionModeManual:
			ladderPlan, err = buildManualLadderProtectionPlan(entryPrice, action, protection.LadderTPSL)
		case store.ProtectionModeAI:
			ladderPlan, err = buildAILadderProtectionPlan(entryPrice, action, protection.LadderTPSL)
		}
		if err != nil {
			return nil, err
		}
		// Drawdown/native trailing owns the profit-taking side. When drawdown is enabled,
		// keep ladder stop-loss legs but suppress ladder take-profit legs to avoid conflict.
		if drawdownEnabled && ladderPlan != nil {
			ladderPlan.TakeProfitOrders = nil
			ladderPlan.NeedsTakeProfit = false
			ladderPlan.TakeProfitPrice = 0
		}
	}

	ladderCoversSL := ladderPlan != nil && len(ladderPlan.StopLossOrders) > 0
	ladderCoversTP := ladderPlan != nil && len(ladderPlan.TakeProfitOrders) > 0

	var plans []*ProtectionPlan
	if ladderPlan != nil {
		plans = append(plans, ladderPlan)
	}

	// Build full plan, but suppress directions already covered by ladder.
	if protection.FullTPSL.Enabled {
		var (
			fullPlan *ProtectionPlan
			err      error
		)
		switch protection.FullTPSL.Mode {
		case store.ProtectionModeManual:
			fullPlan, err = buildManualFullProtectionPlan(entryPrice, action, protection.FullTPSL)
		case store.ProtectionModeAI:
			fullPlan, err = buildAIFullProtectionPlan(entryPrice, action, protection.FullTPSL)
		}
		if err != nil {
			return nil, err
		}
		if fullPlan != nil {
			// Suppress full-position directions that ladder already covers.
			if ladderCoversSL {
				fullPlan.NeedsStopLoss = false
				fullPlan.StopLossPrice = 0
			}
			if ladderCoversTP {
				fullPlan.NeedsTakeProfit = false
				fullPlan.TakeProfitPrice = 0
			}
			// Drawdown/native trailing owns the profit-taking side. Keep SL, suppress TP.
			if drawdownEnabled {
				fullPlan.NeedsTakeProfit = false
				fullPlan.TakeProfitPrice = 0
			}
			// Only merge if full plan still has something to contribute.
			if fullPlan.NeedsStopLoss || fullPlan.NeedsTakeProfit || fullPlan.FallbackMaxLossPrice > 0 {
				plans = append(plans, fullPlan)
			}
		}
	}

	return mergeProtectionPlans(plans...), nil
}

// BuildManualProtectionPlan creates a normalized manual protection plan.
// Phase 2 prefers ladder TP/SL when enabled; otherwise it falls back to full-position TP/SL.
func (at *AutoTrader) BuildManualProtectionPlan(entryPrice float64, decisionSymbol string, action string) (*ProtectionPlan, error) {
	if at.config.StrategyConfig == nil {
		return nil, nil
	}

	protection := at.config.StrategyConfig.Protection
	if plan, err := buildManualLadderProtectionPlan(entryPrice, action, protection.LadderTPSL); err != nil || plan != nil {
		return plan, err
	}

	return buildManualFullProtectionPlan(entryPrice, action, protection.FullTPSL)
}

func buildManualFullProtectionPlan(entryPrice float64, action string, full store.FullTPSLConfig) (*ProtectionPlan, error) {
	if !full.Enabled || full.Mode != store.ProtectionModeManual {
		return nil, nil
	}

	if entryPrice <= 0 {
		return nil, fmt.Errorf("invalid entry price %.8f for protection plan", entryPrice)
	}

	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil, nil
	}

	plan := &ProtectionPlan{Mode: string(full.Mode), RequiresNativeOrders: true}

	if stopLossPct, ok := resolveFullStopLoss(full, 0); ok {
		move := stopLossPct / 100.0
		if isLong {
			plan.StopLossPrice = entryPrice * (1 - move)
		} else {
			plan.StopLossPrice = entryPrice * (1 + move)
		}
		plan.StopLossPrice = roundProtectionPrice(plan.StopLossPrice)
		plan.NeedsStopLoss = true
	}

	if fallbackPct, ok := resolveFallbackMaxLoss(full); ok {
		move := fallbackPct / 100.0
		if isLong {
			plan.FallbackMaxLossPrice = roundProtectionPrice(entryPrice * (1 - move))
		} else {
			plan.FallbackMaxLossPrice = roundProtectionPrice(entryPrice * (1 + move))
		}
	}

	if takeProfitPct, ok := resolveFullTakeProfit(full, 0); ok {
		move := takeProfitPct / 100.0
		if isLong {
			plan.TakeProfitPrice = entryPrice * (1 + move)
		} else {
			plan.TakeProfitPrice = entryPrice * (1 - move)
		}
		plan.TakeProfitPrice = roundProtectionPrice(plan.TakeProfitPrice)
		plan.NeedsTakeProfit = true
	}

	if !plan.NeedsStopLoss && !plan.NeedsTakeProfit && plan.FallbackMaxLossPrice == 0 {
		return nil, nil
	}

	return plan, nil
}

func buildManualLadderProtectionPlan(entryPrice float64, action string, ladder store.LadderTPSLConfig) (*ProtectionPlan, error) {
	if !ladder.Enabled || ladder.Mode != store.ProtectionModeManual {
		return nil, nil
	}
	if entryPrice <= 0 {
		return nil, fmt.Errorf("invalid entry price %.8f for ladder protection plan", entryPrice)
	}

	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil, nil
	}

	plan := &ProtectionPlan{
		Mode:                 string(ladder.Mode),
		RequiresNativeOrders: true,
		RequiresPartialClose: true,
	}

	remainingTakeProfitRatio := 100.0
	remainingStopLossRatio := 100.0
	for _, rule := range ladder.Rules {
		if pricePct, closeRatioPct, ok := resolveLadderTakeProfitRule(rule, ladder, nil); ok && remainingTakeProfitRatio > 0 {
			closeRatio := minPositive(closeRatioPct, remainingTakeProfitRatio)
			move := pricePct / 100.0
			price := entryPrice
			if isLong {
				price = entryPrice * (1 + move)
			} else {
				price = entryPrice * (1 - move)
			}
			price = roundProtectionPrice(price)
			plan.TakeProfitOrders = append(plan.TakeProfitOrders, ProtectionOrder{
				Price:         price,
				CloseRatioPct: closeRatio,
			})
			remainingTakeProfitRatio -= closeRatio
		}

		if pricePct, closeRatioPct, ok := resolveLadderStopLossRule(rule, ladder, nil); ok && remainingStopLossRatio > 0 {
			closeRatio := minPositive(closeRatioPct, remainingStopLossRatio)
			move := pricePct / 100.0
			price := entryPrice
			if isLong {
				price = entryPrice * (1 - move)
			} else {
				price = entryPrice * (1 + move)
			}
			price = roundProtectionPrice(price)
			plan.StopLossOrders = append(plan.StopLossOrders, ProtectionOrder{
				Price:         price,
				CloseRatioPct: closeRatio,
			})
			remainingStopLossRatio -= closeRatio
		}
	}

	plan.NeedsStopLoss = len(plan.StopLossOrders) > 0
	plan.NeedsTakeProfit = len(plan.TakeProfitOrders) > 0
	if !plan.NeedsStopLoss && !plan.NeedsTakeProfit {
		return nil, nil
	}

	if len(plan.StopLossOrders) == 1 {
		plan.StopLossPrice = plan.StopLossOrders[0].Price
	}
	if len(plan.TakeProfitOrders) == 1 {
		plan.TakeProfitPrice = plan.TakeProfitOrders[0].Price
	}
	return plan, nil
}

func minPositive(a, b float64) float64 {
	switch {
	case a <= 0:
		return 0
	case b <= 0:
		return 0
	case a < b:
		return a
	default:
		return b
	}
}
