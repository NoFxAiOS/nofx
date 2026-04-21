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

type drawdownStructureContext struct {
	PrimaryTimeframe string
	LowerTimeframes  []string
	HigherTimeframes []string
	Entry            float64
	Invalidation     float64
	FirstTarget      float64
	Support          []float64
	Resistance       []float64
	FibLevels        []float64
	Anchors          []store.DecisionActionReasonAnchor
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

func evaluateAIDrawdownRule(cfg store.DrawdownTakeProfitConfig, currentPnLPct, peakPnLPct, drawdownPct float64, rules []store.DrawdownTakeProfitRule, structure *drawdownStructureContext, side string, markPrice float64) *drawdownEvaluation {
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
	resolvedStage, stopSource, targetSource := classifyAIDrawdownStage(currentPnLPct, peakPnLPct, structure, side, markPrice)
	if strings.TrimSpace(rule.StageName) == "" || rule.StageName == "profit_stage" {
		rule.StageName = resolvedStage
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
		rule.RunnerStopSource = stopSource
	}
	if strings.TrimSpace(rule.RunnerTargetMode) == "" && rule.RunnerKeepPct > 0 {
		rule.RunnerTargetMode = "structure"
	}
	if strings.TrimSpace(rule.RunnerTargetSource) == "" && rule.RunnerKeepPct > 0 {
		rule.RunnerTargetSource = targetSource
	}

	return &drawdownEvaluation{Rule: normalizeDrawdownRule(rule)}
}

func classifyAIDrawdownStage(currentPnLPct, peakPnLPct float64, structure *drawdownStructureContext, side string, markPrice float64) (string, string, string) {
	if structure != nil && structure.Entry > 0 && structure.FirstTarget > 0 && markPrice > 0 {
		progress := structuralTargetProgress(side, structure.Entry, structure.FirstTarget, markPrice)
		if progress >= 1.15 || isNearAnyLevel(markPrice, structure.FibLevels, 0.0035) {
			return "extension_exhaustion", "extension_swing_trail", "extension_fibonacci"
		}
		if progress >= 0.85 || touchesPrimaryTargetZone(side, structure, markPrice) {
			return "near_primary_target", "primary_target_pullback", "primary_resistance"
		}
		if hasTrendContinuationAnchor(structure.Anchors) || len(structure.Support) > 0 || len(structure.Resistance) > 0 {
			return "trend_continuation", "adjacent_support_flip", "trend_continuation_structure"
		}
	}

	switch {
	case peakPnLPct >= 12 || currentPnLPct >= 10:
		return "extension_exhaustion", "extension_swing_trail", "extension_fibonacci"
	case peakPnLPct >= 5 || currentPnLPct >= 4:
		return "near_primary_target", "primary_target_pullback", "primary_resistance"
	default:
		return "trend_continuation", "adjacent_support_flip", "trend_continuation_structure"
	}
}

func structuralTargetProgress(side string, entry, firstTarget, markPrice float64) float64 {
	if entry <= 0 || firstTarget <= 0 || markPrice <= 0 {
		return 0
	}
	if strings.EqualFold(side, "long") {
		denom := firstTarget - entry
		if denom <= 0 {
			return 0
		}
		return (markPrice - entry) / denom
	}
	denom := entry - firstTarget
	if denom <= 0 {
		return 0
	}
	return (entry - markPrice) / denom
}

func touchesPrimaryTargetZone(side string, structure *drawdownStructureContext, markPrice float64) bool {
	if structure == nil || markPrice <= 0 {
		return false
	}
	if strings.EqualFold(side, "long") {
		return isNearAnyLevel(markPrice, structure.Resistance, 0.004)
	}
	return isNearAnyLevel(markPrice, structure.Support, 0.004)
}

func isNearAnyLevel(price float64, levels []float64, tolerance float64) bool {
	if price <= 0 || tolerance <= 0 {
		return false
	}
	for _, level := range levels {
		if level <= 0 {
			continue
		}
		if math.Abs(price-level)/price <= tolerance {
			return true
		}
	}
	return false
}

func hasTrendContinuationAnchor(anchors []store.DecisionActionReasonAnchor) bool {
	for _, anchor := range anchors {
		typeLower := strings.ToLower(strings.TrimSpace(anchor.Type))
		reasonLower := strings.ToLower(strings.TrimSpace(anchor.Reason))
		if strings.Contains(typeLower, "support") || strings.Contains(typeLower, "resistance") || strings.Contains(typeLower, "target") {
			return true
		}
		if strings.Contains(reasonLower, "pullback") || strings.Contains(reasonLower, "breakout") || strings.Contains(reasonLower, "continuation") || strings.Contains(reasonLower, "retest") {
			return true
		}
	}
	return false
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
