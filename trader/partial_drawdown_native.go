package trader

import (
	"math"
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

type drawdownEvaluation struct {
	Rule store.DrawdownTakeProfitRule
}

func evaluateAIDrawdownRule(cfg store.DrawdownTakeProfitConfig, currentPnLPct, peakPnLPct, drawdownPct float64, rules []store.DrawdownTakeProfitRule) *drawdownEvaluation {
	if len(rules) == 0 {
		return nil
	}

	bestIdx := -1
	bestScore := -1.0
	for i, raw := range rules {
		rule := normalizeDrawdownRule(raw)
		if currentPnLPct < rule.MinProfitPct || drawdownPct < rule.MaxDrawdownPct {
			continue
		}
		score := rule.MinProfitPct*1000 + rule.MaxDrawdownPct
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return nil
	}

	rule := normalizeDrawdownRule(rules[bestIdx])
	if strings.TrimSpace(rule.StageName) == "" || rule.StageName == "profit_stage" {
		rule.StageName = classifyAIDrawdownStage(currentPnLPct, peakPnLPct)
	}

	minRunnerKeep := cfg.MinRunnerKeepPct
	if minRunnerKeep < 0 {
		minRunnerKeep = 0
	}
	if minRunnerKeep > 100 {
		minRunnerKeep = 100
	}
	maxFirstReduce := cfg.MaxFirstReducePct
	if maxFirstReduce <= 0 || maxFirstReduce > 100 {
		maxFirstReduce = 100
	}

	if cfg.RunnerEnabled {
		if rule.RunnerKeepPct < minRunnerKeep {
			rule.RunnerKeepPct = minRunnerKeep
		}
		if rule.RunnerKeepPct > 100 {
			rule.RunnerKeepPct = 100
		}
		closeRatio := 100 - rule.RunnerKeepPct
		if closeRatio > maxFirstReduce {
			closeRatio = maxFirstReduce
			rule.RunnerKeepPct = math.Max(0, 100-closeRatio)
		}
		rule.CloseRatioPct = closeRatio
	} else {
		rule.RunnerKeepPct = 0
	}

	switch cfg.BreakEvenRunnerPolicy {
	case store.DrawdownBreakEvenRunnerDisabled:
		rule.RunnerStopMode = "structure"
	case store.DrawdownBreakEvenRunnerFallbackOnly:
		if rule.RunnerKeepPct > 0 {
			rule.RunnerStopMode = "structure"
		}
	case store.DrawdownBreakEvenRunnerPrimary:
		if strings.TrimSpace(rule.RunnerStopMode) == "" || strings.EqualFold(rule.RunnerStopMode, "structure") {
			rule.RunnerStopMode = "break_even"
		}
	}

	if strings.TrimSpace(rule.RunnerStopSource) == "" && strings.EqualFold(rule.RunnerStopMode, "structure") {
		rule.RunnerStopSource = defaultRunnerStopSource(rule.StageName)
	}
	if strings.TrimSpace(rule.RunnerTargetMode) == "" && rule.RunnerKeepPct > 0 {
		rule.RunnerTargetMode = "structure"
	}
	if strings.TrimSpace(rule.RunnerTargetSource) == "" && rule.RunnerKeepPct > 0 {
		rule.RunnerTargetSource = defaultRunnerTargetSource(rule.StageName)
	}

	return &drawdownEvaluation{Rule: normalizeDrawdownRule(rule)}
}

func classifyAIDrawdownStage(currentPnLPct, peakPnLPct float64) string {
	switch {
	case peakPnLPct >= 12 || currentPnLPct >= 10:
		return "extension_exhaustion"
	case peakPnLPct >= 5 || currentPnLPct >= 4:
		return "near_primary_target"
	default:
		return "trend_continuation"
	}
}

func defaultRunnerStopSource(stage string) string {
	switch strings.TrimSpace(stage) {
	case "extension_exhaustion":
		return "extension_swing_trail"
	case "near_primary_target":
		return "primary_target_pullback"
	default:
		return "adjacent_support_flip"
	}
}

func defaultRunnerTargetSource(stage string) string {
	switch strings.TrimSpace(stage) {
	case "extension_exhaustion":
		return "extension_fibonacci"
	case "near_primary_target":
		return "primary_resistance"
	default:
		return "trend_continuation_structure"
	}
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
