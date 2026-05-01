package trader

import (
	"fmt"
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

type drawdownTierAnchor struct {
	StageName   string  `json:"stage_name,omitempty"`
	Timeframe   string  `json:"timeframe,omitempty"`
	AnchorType  string  `json:"anchor_type,omitempty"`
	Price       float64 `json:"price,omitempty"`
	Reason      string  `json:"reason,omitempty"`
	Source      string  `json:"source,omitempty"`
	UsedFor     string  `json:"used_for,omitempty"`
	DistancePct float64 `json:"distance_pct,omitempty"`
	Reference   string  `json:"reference,omitempty"`
}

func (ctx *drawdownStructureContext) higherTimeframeSet() map[string]struct{} {
	out := make(map[string]struct{})
	if ctx == nil {
		return out
	}
	for _, tf := range ctx.HigherTimeframes {
		if tf = strings.TrimSpace(tf); tf != "" {
			out[tf] = struct{}{}
		}
	}
	return out
}

func (ctx *drawdownStructureContext) selectTierAnchor(side string, rule store.DrawdownTakeProfitRule, entryPrice float64) *drawdownTierAnchor {
	if ctx == nil {
		return nil
	}
	rule = normalizeDrawdownRule(rule)
	targetPrice := calculateProfitBasedTrailingTriggerPrice(entryPrice, side, rule.MinProfitPct)
	if targetPrice <= 0 {
		targetPrice = ctx.FirstTarget
	}
	higher := ctx.higherTimeframeSet()
	preferHigher := isHigherTimeframeRunnerRule(rule)
	preferredTf := strings.TrimSpace(rule.Timeframe)
	var best *drawdownTierAnchor
	bestScore := 0.0
	for _, anchor := range ctx.Anchors {
		if anchor.Price <= 0 {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(anchor.Type))
		if strings.EqualFold(side, "long") {
			if !strings.Contains(kind, "resistance") && !strings.Contains(kind, "target") && !strings.Contains(kind, "high") && !strings.Contains(kind, "fib") {
				continue
			}
		} else if !strings.Contains(kind, "support") && !strings.Contains(kind, "target") && !strings.Contains(kind, "low") && !strings.Contains(kind, "fib") {
			continue
		}
		distance := math.Abs(anchor.Price - targetPrice)
		distancePct := 0.0
		if targetPrice > 0 {
			distancePct = distance / targetPrice * 100
		}
		score := distance
		if preferredTf != "" && strings.EqualFold(anchor.Timeframe, preferredTf) {
			score *= 0.01
		} else if _, ok := higher[anchor.Timeframe]; ok && preferHigher {
			score *= 0.01
		} else if _, ok := higher[anchor.Timeframe]; ok {
			score *= 0.75
		}
		if best == nil || score < bestScore {
			bestScore = score
			usedFor := "drawdown_profit_lock"
			if preferHigher {
				usedFor = "higher_timeframe_runner"
			}
			best = &drawdownTierAnchor{StageName: rule.StageName, Timeframe: anchor.Timeframe, AnchorType: anchor.Type, Price: anchor.Price, Reason: anchor.Reason, Source: "entry_structure_anchor", UsedFor: usedFor, DistancePct: distancePct, Reference: fmt.Sprintf("min_profit=%.4f", rule.MinProfitPct)}
		}
	}
	return best
}

func isHigherTimeframeRunnerRule(rule store.DrawdownTakeProfitRule) bool {
	stage := strings.ToLower(strings.TrimSpace(rule.StageName))
	tf := strings.ToLower(strings.TrimSpace(rule.Timeframe))
	return strings.Contains(stage, "runner") || strings.Contains(stage, "outer") || strings.Contains(stage, "higher") || strings.Contains(stage, "trend") || strings.Contains(tf, "h") || strings.Contains(strings.ToLower(rule.RunnerTargetSource), "higher") || strings.Contains(strings.ToLower(rule.RunnerStopSource), "higher")
}

func inferDrawdownStageName(rule store.DrawdownTakeProfitRule) string {
	if strings.TrimSpace(rule.StageName) != "" {
		return strings.TrimSpace(rule.StageName)
	}
	if rule.CloseRatioPct >= 99.999 {
		return "outer_exit"
	}
	if isHigherTimeframeRunnerRule(rule) || rule.RunnerKeepPct > 0 && rule.CloseRatioPct >= 70 {
		return "higher_timeframe_runner"
	}
	if rule.CloseRatioPct > 0 && rule.CloseRatioPct < 100 {
		return "partial_profit_lock"
	}
	return "profit_stage"
}

func normalizeDrawdownRule(rule store.DrawdownTakeProfitRule) store.DrawdownTakeProfitRule {
	if strings.TrimSpace(rule.StageName) == "" {
		rule.StageName = inferDrawdownStageName(rule)
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
		if currentPnLPct < rule.MinProfitPct || !isDrawdownThresholdMet(currentPnLPct, drawdownPct, rule) {
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
	if rule.StageName == "" || rule.StageName == "profit_stage" || rule.StageName == "partial_profit_lock" || rule.StageName == "higher_timeframe_runner" || rule.StageName == "outer_exit" {
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
		if progress >= 1.0 && hasHigherTimeframeAnchor(structure) {
			return "higher_timeframe_runner", "higher_timeframe_structure_trail", "higher_timeframe_runner_target"
		}
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

func hasHigherTimeframeAnchor(structure *drawdownStructureContext) bool {
	if structure == nil {
		return false
	}
	higher := structure.higherTimeframeSet()
	if len(higher) == 0 {
		return false
	}
	for _, anchor := range structure.Anchors {
		if _, ok := higher[strings.TrimSpace(anchor.Timeframe)]; ok && anchor.Price > 0 {
			return true
		}
	}
	return false
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

func summarizeDrawdownStructureEvidence(structure *drawdownStructureContext, side string) []string {
	if structure == nil {
		return nil
	}
	evidence := make([]string, 0, 6)
	if structure.PrimaryTimeframe != "" {
		evidence = append(evidence, "tf:"+structure.PrimaryTimeframe)
	}
	if structure.FirstTarget > 0 {
		evidence = append(evidence, "first_target")
	}
	if strings.EqualFold(side, "long") {
		if len(structure.Resistance) > 0 {
			evidence = append(evidence, "resistance")
		}
		if len(structure.Support) > 0 {
			evidence = append(evidence, "support_stop")
		}
	} else {
		if len(structure.Support) > 0 {
			evidence = append(evidence, "support_target")
		}
		if len(structure.Resistance) > 0 {
			evidence = append(evidence, "resistance_stop")
		}
	}
	if len(structure.FibLevels) > 0 {
		evidence = append(evidence, "fibonacci")
	}
	for _, anchor := range structure.Anchors {
		if anchor.Type == "" {
			continue
		}
		evidence = append(evidence, "anchor:"+strings.ToLower(strings.TrimSpace(anchor.Type)))
		if len(evidence) >= 6 {
			break
		}
	}
	return evidence
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
