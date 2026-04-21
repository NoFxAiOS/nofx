package trader

import (
	"strings"

	"nofx/store"
)

type DrawdownRunnerState struct {
	StageName                   string
	RunnerKeepPct               float64
	RunnerStopMode              string
	RunnerStopSource            string
	RunnerTargetMode            string
	RunnerTargetSource          string
	BreakEvenSuppressedByRunner bool
}

func normalizeDrawdownRule(rule store.DrawdownTakeProfitRule) store.DrawdownTakeProfitRule {
	if strings.TrimSpace(rule.StageName) == "" {
		rule.StageName = "profit_stage"
	}
	if rule.RunnerKeepPct <= 0 && rule.CloseRatioPct > 0 && rule.CloseRatioPct < 100 {
		rule.RunnerKeepPct = 100 - rule.CloseRatioPct
	}
	if rule.RunnerKeepPct < 0 {
		rule.RunnerKeepPct = 0
	}
	if rule.RunnerKeepPct > 100 {
		rule.RunnerKeepPct = 100
	}
	if strings.TrimSpace(rule.RunnerStopMode) == "" {
		rule.RunnerStopMode = "break_even"
	}
	if strings.TrimSpace(rule.RunnerTargetMode) == "" && rule.RunnerKeepPct > 0 {
		rule.RunnerTargetMode = "structure"
	}
	return rule
}

func buildDrawdownRunnerState(rule store.DrawdownTakeProfitRule) *DrawdownRunnerState {
	rule = normalizeDrawdownRule(rule)
	if rule.RunnerKeepPct <= 0 {
		return nil
	}
	state := &DrawdownRunnerState{
		StageName:          rule.StageName,
		RunnerKeepPct:      rule.RunnerKeepPct,
		RunnerStopMode:     rule.RunnerStopMode,
		RunnerStopSource:   rule.RunnerStopSource,
		RunnerTargetMode:   rule.RunnerTargetMode,
		RunnerTargetSource: rule.RunnerTargetSource,
	}
	if strings.EqualFold(rule.RunnerStopMode, "structure") {
		state.BreakEvenSuppressedByRunner = true
	}
	return state
}

// buildManagedPartialDrawdownPlanCandidate converts a partial drawdown rule into a managed
// protection plan representation. This is NOT a native trailing order: it precomputes a fixed
// trigger/take-profit price from the drawdown rule and places a standard TP-style protection order.
func buildManagedPartialDrawdownPlanCandidate(entryPrice float64, action string, rule store.DrawdownTakeProfitRule) *ProtectionPlan {
	if entryPrice <= 0 || rule.MinProfitPct <= 0 || rule.MaxDrawdownPct <= 0 {
		return nil
	}
	if rule.CloseRatioPct <= 0 || rule.CloseRatioPct >= 99.999 {
		return nil
	}

	isLong := action == "open_long"
	isShort := action == "open_short"
	if !isLong && !isShort {
		return nil
	}

	peakMove := rule.MinProfitPct / 100.0
	drawdownMove := rule.MaxDrawdownPct / 100.0
	price := entryPrice

	if isLong {
		price = entryPrice * (1 + peakMove) * (1 - drawdownMove)
	} else {
		price = entryPrice * (1 - peakMove) * (1 + drawdownMove)
	}

	if price <= 0 {
		return nil
	}

	rule = normalizeDrawdownRule(rule)
	runnerState := buildDrawdownRunnerState(rule)

	return &ProtectionPlan{
		Mode:                        "drawdown_partial_managed",
		NeedsTakeProfit:             true,
		TakeProfitPrice:             price,
		TakeProfitOrders:            []ProtectionOrder{{Price: price, CloseRatioPct: rule.CloseRatioPct}},
		RequiresNativeOrders:        true,
		RequiresPartialClose:        true,
		DrawdownRunnerState:         runnerState,
		BreakEvenSuppressedByRunner: runnerState != nil && runnerState.BreakEvenSuppressedByRunner,
	}
}
