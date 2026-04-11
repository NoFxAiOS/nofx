package trader

import (
	"fmt"
	"math"
	"nofx/store"
)

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
	StopLossOrders       []ProtectionOrder
	TakeProfitOrders     []ProtectionOrder
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
		merged.RequiresNativeOrders = merged.RequiresNativeOrders || plan.RequiresNativeOrders
		merged.RequiresPartialClose = merged.RequiresPartialClose || plan.RequiresPartialClose
		if merged.StopLossPrice == 0 && plan.StopLossPrice > 0 {
			merged.StopLossPrice = plan.StopLossPrice
		}
		if merged.TakeProfitPrice == 0 && plan.TakeProfitPrice > 0 {
			merged.TakeProfitPrice = plan.TakeProfitPrice
		}
	}

	// Ladder orders win for their direction; keep full-position prices only when no ladder orders exist for that side.
	if len(merged.StopLossOrders) > 0 {
		merged.StopLossPrice = 0
	}
	if len(merged.TakeProfitOrders) > 0 {
		merged.TakeProfitPrice = 0
	}

	if !merged.NeedsStopLoss && !merged.NeedsTakeProfit && len(merged.StopLossOrders) == 0 && len(merged.TakeProfitOrders) == 0 {
		return nil
	}
	return merged
}

// BuildConfiguredProtectionPlan creates a normalized protection plan from strategy configuration.
// Unlike BuildManualProtectionPlan, it can also materialize AI-mode strategy protection config
// when the runtime decision does not provide a concrete decision.ProtectionPlan payload yet.
func (at *AutoTrader) BuildConfiguredProtectionPlan(entryPrice float64, action string) (*ProtectionPlan, error) {
	if at.config.StrategyConfig == nil {
		return nil, nil
	}

	protection := at.config.StrategyConfig.Protection

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
	}

	ladderCoversSL := ladderPlan != nil && len(ladderPlan.StopLossOrders) > 0
	ladderCoversTP := ladderPlan != nil && len(ladderPlan.TakeProfitOrders) > 0

	var plans []*ProtectionPlan
	if ladderPlan != nil {
		plans = append(plans, ladderPlan)
	}

	// Build full plan, but suppress directions already covered by ladder.
	if protection.FullTPSL.Enabled {
		var fullPlan *ProtectionPlan
		var err error
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
			// Only merge if full plan still has something to contribute.
			if fullPlan.NeedsStopLoss || fullPlan.NeedsTakeProfit {
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

	if full.StopLoss.Enabled && full.StopLoss.PriceMovePct > 0 {
		move := full.StopLoss.PriceMovePct / 100.0
		if isLong {
			plan.StopLossPrice = entryPrice * (1 - move)
		} else {
			plan.StopLossPrice = entryPrice * (1 + move)
		}
		plan.StopLossPrice = roundProtectionPrice(plan.StopLossPrice)
		plan.NeedsStopLoss = true
	}

	if full.TakeProfit.Enabled && full.TakeProfit.PriceMovePct > 0 {
		move := full.TakeProfit.PriceMovePct / 100.0
		if isLong {
			plan.TakeProfitPrice = entryPrice * (1 + move)
		} else {
			plan.TakeProfitPrice = entryPrice * (1 - move)
		}
		plan.TakeProfitPrice = roundProtectionPrice(plan.TakeProfitPrice)
		plan.NeedsTakeProfit = true
	}

	if !plan.NeedsStopLoss && !plan.NeedsTakeProfit {
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
		if ladder.TakeProfitEnabled && rule.TakeProfitPct > 0 && rule.TakeProfitCloseRatioPct > 0 && remainingTakeProfitRatio > 0 {
			closeRatio := minPositive(rule.TakeProfitCloseRatioPct, remainingTakeProfitRatio)
			move := rule.TakeProfitPct / 100.0
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

		if ladder.StopLossEnabled && rule.StopLossPct > 0 && rule.StopLossCloseRatioPct > 0 && remainingStopLossRatio > 0 {
			closeRatio := minPositive(rule.StopLossCloseRatioPct, remainingStopLossRatio)
			move := rule.StopLossPct / 100.0
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
