package trader

import (
	"fmt"
	"strings"
)

// ProtectionOwnershipState is a pure, exchange-agnostic summary of whether an
// active position has the minimum expected protection owners visible.
//
// This is intentionally not wired into live reconciliation yet. It is the testable
// ownership model that should replace scattered reconciler conditionals after the
// matrix is green.
type ProtectionOwnershipState struct {
	StaticOwner       string
	ProfitOwner       string
	StopOwner         string
	State             string
	Verified          bool
	MissingStop       bool
	MissingProfit     bool
	UnexpectedStops   int
	UnexpectedProfits int
	Reasons           []string
}

func evaluateProtectionOwnership(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan, breakEvenArmed bool, nativeTrailingArmed bool) ProtectionOwnershipState {
	state := ProtectionOwnershipState{State: "unprotected"}
	positionSide = strings.ToUpper(positionSide)

	if plan == nil {
		if breakEvenArmed || nativeTrailingArmed {
			state.State = "protected"
			state.Verified = true
			if breakEvenArmed {
				state.StopOwner = "breakeven"
			}
			if nativeTrailingArmed {
				state.ProfitOwner = "drawdown"
			}
			return state
		}
		state.Reasons = append(state.Reasons, "no protection plan and no armed native owner")
		state.MissingStop = true
		return state
	}

	missingSL, missingTP := detectMissingProtection(openOrders, positionSide, plan, false)
	unexpectedSL, unexpectedTP := detectUnexpectedProtectionOrders(openOrders, positionSide, plan, breakEvenArmed, nativeTrailingArmed)
	state.MissingStop = missingSL && !breakEvenArmed
	state.MissingProfit = missingTP && !nativeTrailingArmed
	state.UnexpectedStops = unexpectedSL
	state.UnexpectedProfits = unexpectedTP

	if breakEvenArmed {
		state.StopOwner = "breakeven"
	} else {
		state.StopOwner = visiblePlanStopOwnerFromOrders(openOrders, positionSide, plan)
	}

	if nativeTrailingArmed {
		state.ProfitOwner = "drawdown"
	} else if hasVisiblePlanProfitOwner(openOrders, positionSide, plan) {
		state.ProfitOwner = visiblePlanProfitOwner(plan)
	}

	if len(plan.StopLossOrders) > 0 || (plan.NeedsStopLoss && plan.StopLossPrice > 0) || plan.FallbackMaxLossPrice > 0 {
		state.StaticOwner = state.StopOwner
	}

	if state.StopOwner == "" {
		state.Reasons = append(state.Reasons, "missing stop/fallback owner")
	}
	if planRequiresProfitOwner(plan) && state.ProfitOwner == "" {
		state.Reasons = append(state.Reasons, "missing profit owner")
	}
	if unexpectedSL > 0 || unexpectedTP > 0 {
		state.Reasons = append(state.Reasons, fmt.Sprintf("unexpected protection orders sl=%d tp=%d", unexpectedSL, unexpectedTP))
	}

	state.Verified = state.StopOwner != "" && (!planRequiresProfitOwner(plan) || state.ProfitOwner != "") && unexpectedSL == 0 && unexpectedTP == 0
	if state.Verified {
		state.State = "protected"
	} else if state.StopOwner != "" || state.ProfitOwner != "" {
		state.State = "degraded"
	}
	return state
}

func planRequiresProfitOwner(plan *ProtectionPlan) bool {
	if plan == nil {
		return false
	}
	return len(plan.TakeProfitOrders) > 0 || (plan.NeedsTakeProfit && plan.TakeProfitPrice > 0)
}

func visiblePlanStopOwnerFromOrders(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan) string {
	if plan == nil {
		return ""
	}
	for _, target := range plan.StopLossOrders {
		if hasMatchingProtectionOrder(openOrders, positionSide, false, target.Price) {
			return "ladder_sl"
		}
	}
	if plan.NeedsStopLoss && plan.StopLossPrice > 0 && hasMatchingProtectionOrder(openOrders, positionSide, false, plan.StopLossPrice) {
		return "full_sl"
	}
	if plan.FallbackMaxLossPrice > 0 && hasMatchingProtectionOrder(openOrders, positionSide, false, plan.FallbackMaxLossPrice) {
		return "fallback"
	}
	if visibleFallbackOwnerSatisfied(openOrders, positionSide) {
		return "fallback"
	}
	return ""
}

func hasVisiblePlanProfitOwner(openOrders []OpenOrder, positionSide string, plan *ProtectionPlan) bool {
	if plan == nil {
		return false
	}
	for _, target := range plan.TakeProfitOrders {
		if hasMatchingProtectionOrder(openOrders, positionSide, true, target.Price) {
			return true
		}
	}
	return plan.NeedsTakeProfit && plan.TakeProfitPrice > 0 && hasMatchingProtectionOrder(openOrders, positionSide, true, plan.TakeProfitPrice)
}

func visiblePlanStopOwner(plan *ProtectionPlan) string {
	if plan == nil {
		return ""
	}
	if len(plan.StopLossOrders) > 0 {
		return "ladder_sl"
	}
	if plan.NeedsStopLoss && plan.StopLossPrice > 0 {
		return "full_sl"
	}
	if plan.FallbackMaxLossPrice > 0 {
		return "fallback"
	}
	return ""
}

func visiblePlanProfitOwner(plan *ProtectionPlan) string {
	if plan == nil {
		return ""
	}
	if len(plan.TakeProfitOrders) > 0 {
		return "ladder_tp"
	}
	if plan.NeedsTakeProfit && plan.TakeProfitPrice > 0 {
		return "full_tp"
	}
	return ""
}
