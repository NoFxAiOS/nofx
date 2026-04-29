package trader

import (
	"testing"

	"nofx/store"
)

func TestBuildManagedPartialDrawdownPlanCandidate_Long(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  50,
	}

	plan := buildManagedPartialDrawdownPlanCandidate(100, "open_long", rule)
	if plan == nil {
		t.Fatal("expected candidate plan")
	}
	if plan.Mode != "drawdown_partial_managed" {
		t.Fatalf("expected managed mode, got %q", plan.Mode)
	}
	if !plan.RequiresPartialClose {
		t.Fatal("expected partial-close requirement")
	}
	if len(plan.TakeProfitOrders) != 1 {
		t.Fatalf("expected one managed partial order, got %d", len(plan.TakeProfitOrders))
	}
	if plan.TakeProfitOrders[0].CloseRatioPct != 50 {
		t.Fatalf("expected close ratio 50, got %.2f", plan.TakeProfitOrders[0].CloseRatioPct)
	}
	if plan.TakeProfitOrders[0].Price <= 0 {
		t.Fatalf("expected positive derived price, got %.4f", plan.TakeProfitOrders[0].Price)
	}
}

func TestBuildManagedPartialDrawdownPlanCandidate_Short(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  40,
	}

	plan := buildManagedPartialDrawdownPlanCandidate(100, "open_short", rule)
	if plan == nil {
		t.Fatal("expected candidate plan")
	}
	if plan.Mode != "drawdown_partial_managed" {
		t.Fatalf("expected managed mode, got %q", plan.Mode)
	}
	if plan.TakeProfitOrders[0].Price <= 0 {
		t.Fatalf("expected positive derived price, got %.4f", plan.TakeProfitOrders[0].Price)
	}
}

func TestBuildManagedPartialDrawdownPlanCandidate_IgnoresFullClose(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   10,
		MaxDrawdownPct: 20,
		CloseRatioPct:  100,
	}
	if plan := buildManagedPartialDrawdownPlanCandidate(100, "open_long", rule); plan != nil {
		t.Fatal("expected nil for full-close rule")
	}
}

func TestBuildManagedPartialDrawdownPlanCandidate_CarriesRunnerStateAndSuppressesBE(t *testing.T) {
	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:       10,
		MaxDrawdownPct:     20,
		CloseRatioPct:      70,
		StageName:          "lock_first_profit",
		RunnerKeepPct:      30,
		RunnerStopMode:     "structure",
		RunnerStopSource:   "adjacent_support_flip",
		RunnerTargetMode:   "structure",
		RunnerTargetSource: "primary_resistance",
	}

	plan := buildManagedPartialDrawdownPlanCandidate(100, "open_long", rule)
	if plan == nil {
		t.Fatal("expected candidate plan")
	}
	if plan.DrawdownRunnerState == nil {
		t.Fatal("expected runner state")
	}
	if !plan.BreakEvenSuppressedByRunner {
		t.Fatal("expected break-even suppression by runner")
	}
	if plan.DrawdownRunnerState.StageName != "lock_first_profit" {
		t.Fatalf("expected stage name carried, got %q", plan.DrawdownRunnerState.StageName)
	}
	if plan.DrawdownRunnerState.RunnerKeepPct != 30 {
		t.Fatalf("expected runner keep 30, got %.2f", plan.DrawdownRunnerState.RunnerKeepPct)
	}
}

func TestDrawdownStructureContextSelectsHigherTimeframeRunnerAnchor(t *testing.T) {
	ctx := &drawdownStructureContext{
		HigherTimeframes: []string{"1h"},
		Anchors: []store.DecisionActionReasonAnchor{
			{Type: "resistance", Timeframe: "15m", Price: 110, Reason: "primary target"},
			{Type: "resistance", Timeframe: "1h", Price: 118, Reason: "higher runner target"},
		},
	}
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 8, MaxDrawdownPct: 50, CloseRatioPct: 50, RunnerKeepPct: 50, StageName: "runner"}
	anchor := ctx.selectTierAnchor("long", rule, 100)
	if anchor == nil {
		t.Fatal("expected anchor")
	}
	if anchor.Timeframe != "1h" || anchor.Price != 118 {
		t.Fatalf("expected higher timeframe runner anchor, got %+v", anchor)
	}
}

func TestDrawdownStructureContextSelectsPrimaryAnchorForFirstStage(t *testing.T) {
	ctx := &drawdownStructureContext{
		HigherTimeframes: []string{"1h"},
		Anchors: []store.DecisionActionReasonAnchor{
			{Type: "resistance", Timeframe: "15m", Price: 110, Reason: "primary target"},
			{Type: "resistance", Timeframe: "1h", Price: 118, Reason: "higher runner target"},
		},
	}
	rule := store.DrawdownTakeProfitRule{MinProfitPct: 10, MaxDrawdownPct: 60, CloseRatioPct: 100, StageName: "partial_lock"}
	anchor := ctx.selectTierAnchor("long", rule, 100)
	if anchor == nil {
		t.Fatal("expected anchor")
	}
	if anchor.Timeframe != "15m" || anchor.Price != 110 {
		t.Fatalf("expected nearest primary anchor, got %+v", anchor)
	}
}

func TestClassifyAIDrawdownStageMigratesPastPrimaryTargetToHigherRunner(t *testing.T) {
	ctx := &drawdownStructureContext{
		Entry:            100,
		FirstTarget:      110,
		HigherTimeframes: []string{"1h"},
		Anchors: []store.DecisionActionReasonAnchor{
			{Type: "resistance", Timeframe: "15m", Price: 110, Reason: "primary target"},
			{Type: "resistance", Timeframe: "1h", Price: 118, Reason: "higher runner target"},
		},
	}
	stage, stopSource, targetSource := classifyAIDrawdownStage(11, 12, ctx, "long", 111)
	if stage != "higher_timeframe_runner" || stopSource != "higher_timeframe_structure_trail" || targetSource != "higher_timeframe_runner_target" {
		t.Fatalf("expected higher timeframe runner stage, got stage=%s stop=%s target=%s", stage, stopSource, targetSource)
	}
}

func TestClassifyAIDrawdownStageKeepsPrimaryBeforeTarget(t *testing.T) {
	ctx := &drawdownStructureContext{
		Entry:            100,
		FirstTarget:      110,
		HigherTimeframes: []string{"1h"},
		Anchors: []store.DecisionActionReasonAnchor{
			{Type: "resistance", Timeframe: "1h", Price: 118, Reason: "higher runner target"},
		},
	}
	stage, stopSource, targetSource := classifyAIDrawdownStage(8, 8, ctx, "long", 108)
	if stage != "trend_continuation" || stopSource != "adjacent_support_flip" || targetSource != "trend_continuation_structure" {
		t.Fatalf("expected trend continuation before primary target zone, got stage=%s stop=%s target=%s", stage, stopSource, targetSource)
	}
}
