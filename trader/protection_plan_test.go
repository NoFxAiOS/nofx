package trader

import (
	"math"
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestBuildConfiguredProtectionPlanUsesStrategyLevelAIModeConfig(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					FullTPSL: store.FullTPSLConfig{
						Enabled: true,
						Mode:    store.ProtectionModeAI,
						TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: 8},
						StopLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: 1.5},
					},
					LadderTPSL: store.LadderTPSLConfig{
						Enabled:           true,
						Mode:              store.ProtectionModeAI,
						TakeProfitEnabled: true,
						StopLossEnabled:   true,
						TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
						TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
						StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
						StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeAI},
						Rules: []store.LadderTPSLRule{{
							TakeProfitPct:           0.5,
							TakeProfitCloseRatioPct: 30,
							StopLossPct:             0.6,
							StopLossCloseRatioPct:   30,
						}},
					},
				},
			},
		},
	}

	plan, err := at.BuildConfiguredProtectionPlan(100, "open_short")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan when strategy protection is ai-enabled")
	}
	if plan.Mode != string(store.ProtectionModeAI) {
		t.Fatalf("expected ai plan mode, got %+v", plan)
	}
	if len(plan.StopLossOrders) == 0 && !plan.NeedsStopLoss && plan.StopLossPrice == 0 {
		t.Fatalf("expected ai-configured plan to contribute stop-loss protection, got %+v", plan)
	}
}

func TestBuildConfiguredProtectionPlanSupportsManualMode(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					LadderTPSL: store.LadderTPSLConfig{
						Enabled:           true,
						Mode:              store.ProtectionModeManual,
						TakeProfitEnabled: true,
						StopLossEnabled:   true,
						TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						Rules: []store.LadderTPSLRule{{
							TakeProfitPct:           1.2,
							TakeProfitCloseRatioPct: 40,
							StopLossPct:             0.8,
							StopLossCloseRatioPct:   40,
						}},
					},
				},
			},
		},
	}

	plan, err := at.BuildConfiguredProtectionPlan(100, "open_long")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected manual plan from strategy config")
	}
	if plan.Mode != string(store.ProtectionModeManual) {
		t.Fatalf("expected manual mode, got %q", plan.Mode)
	}
	if len(plan.TakeProfitOrders) != 1 || len(plan.StopLossOrders) != 1 {
		t.Fatalf("expected 1 tp and 1 sl ladder order, got tp=%d sl=%d", len(plan.TakeProfitOrders), len(plan.StopLossOrders))
	}
}

func TestBuildManualProtectionPlanFallsBackToFullTPSL(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeManual,
		TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 10},
		StopLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 5},
	}

	plan, err := at.BuildManualProtectionPlan(100, "BTCUSDT", "open_long")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if plan == nil {
		t.Fatal("expected protection plan")
	}
	if !plan.NeedsTakeProfit || !plan.NeedsStopLoss {
		t.Fatalf("expected full tp/sl plan, got %+v", plan)
	}
	if !almostEqual(plan.TakeProfitPrice, 110) || !almostEqual(plan.StopLossPrice, 95) {
		t.Fatalf("unexpected prices: tp=%.8f sl=%.8f", plan.TakeProfitPrice, plan.StopLossPrice)
	}
}

func TestBuildManualProtectionPlanPrefersLadder(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeManual,
		TakeProfitEnabled: true,
		StopLossEnabled:   true,
		TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		Rules: []store.LadderTPSLRule{
			{TakeProfitPct: 5, TakeProfitCloseRatioPct: 30, StopLossPct: 2, StopLossCloseRatioPct: 50},
			{TakeProfitPct: 10, TakeProfitCloseRatioPct: 70, StopLossPct: 4, StopLossCloseRatioPct: 50},
		},
	}
	at.config.StrategyConfig.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeManual,
		TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 20},
		StopLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 10},
	}

	plan, err := at.BuildManualProtectionPlan(100, "BTCUSDT", "open_long")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if plan == nil {
		t.Fatal("expected ladder plan")
	}
	if !plan.RequiresPartialClose {
		t.Fatalf("expected ladder plan to require partial close, got %+v", plan)
	}
	if len(plan.TakeProfitOrders) != 2 || len(plan.StopLossOrders) != 2 {
		t.Fatalf("expected 2 ladder orders each, got tp=%d sl=%d", len(plan.TakeProfitOrders), len(plan.StopLossOrders))
	}
	if !almostEqual(plan.TakeProfitOrders[0].Price, 105) || !almostEqual(plan.TakeProfitOrders[1].Price, 110) {
		t.Fatalf("unexpected ladder take-profit prices: %+v", plan.TakeProfitOrders)
	}
	if !almostEqual(plan.StopLossOrders[0].Price, 98) || !almostEqual(plan.StopLossOrders[1].Price, 96) {
		t.Fatalf("unexpected ladder stop-loss prices: %+v", plan.StopLossOrders)
	}
}

func TestBuildManualProtectionPlanSupportsMixedLadderModes(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeManual,
		TakeProfitEnabled: true,
		StopLossEnabled:   true,
		TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: 0},
		StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: 0},
		StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		Rules: []store.LadderTPSLRule{{TakeProfitPct: 5, TakeProfitCloseRatioPct: 30, StopLossPct: 2, StopLossCloseRatioPct: 50}},
	}

	takeProfitPct, takeProfitSize, ok := resolveLadderTakeProfitRule(at.config.StrategyConfig.Protection.LadderTPSL.Rules[0], at.config.StrategyConfig.Protection.LadderTPSL, &kernel.AIProtectionLadderRule{TakeProfitCloseRatioPct: 45})
	if !ok || !almostEqual(takeProfitPct, 5) || !almostEqual(takeProfitSize, 45) {
		t.Fatalf("expected mixed manual/ai TP resolution, got pct=%.2f size=%.2f ok=%v", takeProfitPct, takeProfitSize, ok)
	}

	stopLossPct, stopLossSize, ok := resolveLadderStopLossRule(at.config.StrategyConfig.Protection.LadderTPSL.Rules[0], at.config.StrategyConfig.Protection.LadderTPSL, &kernel.AIProtectionLadderRule{StopLossPct: 3})
	if !ok || !almostEqual(stopLossPct, 3) || !almostEqual(stopLossSize, 50) {
		t.Fatalf("expected mixed ai/manual SL resolution, got pct=%.2f size=%.2f ok=%v", stopLossPct, stopLossSize, ok)
	}
}

func TestBuildManualProtectionPlanDisablesLadderSideWhenAnyDimensionDisabled(t *testing.T) {
	ladder := store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeManual,
		TakeProfitEnabled: true,
		TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeDisabled, Value: 0},
		TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
		Rules:             []store.LadderTPSLRule{{TakeProfitPct: 5, TakeProfitCloseRatioPct: 50}},
	}

	_, _, ok := resolveLadderTakeProfitRule(ladder.Rules[0], ladder, nil)
	if ok {
		t.Fatal("expected ladder TP resolution to be disabled when price dimension is disabled")
	}
}

func TestResolveFullProtectionSupportsMixedModes(t *testing.T) {
	full := store.FullTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeAI,
		TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeAI, Value: 8},
		StopLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 5},
		FallbackMaxLoss: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 9},
	}

	tp, ok := resolveFullTakeProfit(full, 8)
	if !ok || !almostEqual(tp, 8) {
		t.Fatalf("expected ai full take profit 8, got %.2f ok=%v", tp, ok)
	}
	sl, ok := resolveFullStopLoss(full, 0)
	if !ok || !almostEqual(sl, 5) {
		t.Fatalf("expected manual full stop loss 5, got %.2f ok=%v", sl, ok)
	}
	fallback, ok := resolveFallbackMaxLoss(full)
	if !ok || !almostEqual(fallback, 9) {
		t.Fatalf("expected fallback max loss 9, got %.2f ok=%v", fallback, ok)
	}
}

func TestResolveFullProtectionDisabledDimensionSkipsOutput(t *testing.T) {
	full := store.FullTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeManual,
		TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeDisabled, Value: 0},
		StopLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 5},
	}

	if _, ok := resolveFullTakeProfit(full, 0); ok {
		t.Fatal("expected disabled full take profit to produce no value")
	}
	if sl, ok := resolveFullStopLoss(full, 0); !ok || !almostEqual(sl, 5) {
		t.Fatalf("expected manual full stop loss 5, got %.2f ok=%v", sl, ok)
	}
}

func TestBuildConfiguredProtectionPlanLadderWinsOverFullSameDirection(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					FullTPSL: store.FullTPSLConfig{
						Enabled:    true,
						Mode:       store.ProtectionModeManual,
						StopLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 5},
						TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 10},
					},
					LadderTPSL: store.LadderTPSLConfig{
						Enabled:           true,
						Mode:              store.ProtectionModeManual,
						TakeProfitEnabled: true,
						StopLossEnabled:   true,
						TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						StopLossPrice:     store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						StopLossSize:      store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						Rules: []store.LadderTPSLRule{{
							TakeProfitPct:           3,
							TakeProfitCloseRatioPct: 50,
							StopLossPct:             2,
							StopLossCloseRatioPct:   100,
						}},
					},
				},
			},
		},
	}

	plan, err := at.BuildConfiguredProtectionPlan(100, "open_long")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected plan")
	}
	if len(plan.StopLossOrders) != 1 || len(plan.TakeProfitOrders) != 1 {
		t.Fatalf("expected ladder to win over full, got %+v", plan)
	}
	if plan.StopLossPrice != 0 || plan.TakeProfitPrice != 0 {
		t.Fatalf("expected full tp/sl prices suppressed when ladder covers both, got sl=%.4f tp=%.4f", plan.StopLossPrice, plan.TakeProfitPrice)
	}
}

func TestBuildConfiguredProtectionPlanFullSLLadderTPCoexist(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					FullTPSL: store.FullTPSLConfig{
						Enabled:    true,
						Mode:       store.ProtectionModeManual,
						StopLoss:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 5},
						TakeProfit: store.ProtectionValueSource{Mode: store.ProtectionValueModeDisabled, Value: 0},
					},
					LadderTPSL: store.LadderTPSLConfig{
						Enabled:           true,
						Mode:              store.ProtectionModeManual,
						TakeProfitEnabled: true,
						StopLossEnabled:   false,
						TakeProfitPrice:   store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						TakeProfitSize:    store.ProtectionValueSource{Mode: store.ProtectionValueModeManual, Value: 1},
						Rules: []store.LadderTPSLRule{{
							TakeProfitPct:           4,
							TakeProfitCloseRatioPct: 100,
							StopLossPct:             2,
							StopLossCloseRatioPct:   100,
						}},
					},
				},
			},
		},
	}

	plan, err := at.BuildConfiguredProtectionPlan(100, "open_long")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected plan")
	}
	if !plan.NeedsStopLoss || len(plan.TakeProfitOrders) != 1 {
		t.Fatalf("expected full SL + ladder TP coexist, got %+v", plan)
	}
	if !almostEqual(plan.StopLossPrice, 95) || !almostEqual(plan.TakeProfitOrders[0].Price, 104) {
		t.Fatalf("unexpected coexist plan: %+v", plan)
	}
}

func TestBuildAIProtectionPlanFullUsesAIDecisionPercentages(t *testing.T) {
	plan, err := buildAIProtectionPlan(100, "open_long", &kernel.AIProtectionPlan{
		Mode:          "full",
		TakeProfitPct: 8,
		StopLossPct:   2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected ai full protection plan")
	}
	if plan.Mode != string(store.ProtectionModeAI) {
		t.Fatalf("expected ai mode, got %q", plan.Mode)
	}
	if !plan.NeedsTakeProfit || !plan.NeedsStopLoss {
		t.Fatalf("expected full tp/sl, got %+v", plan)
	}
	if !almostEqual(plan.TakeProfitPrice, 108) || !almostEqual(plan.StopLossPrice, 98) {
		t.Fatalf("unexpected ai full prices: tp=%.8f sl=%.8f", plan.TakeProfitPrice, plan.StopLossPrice)
	}
}

func TestBuildAIProtectionPlanLadderUsesAIDecisionRules(t *testing.T) {
	plan, err := buildAIProtectionPlan(100, "open_short", &kernel.AIProtectionPlan{
		Mode: "ladder",
		LadderRules: []kernel.AIProtectionLadderRule{
			{TakeProfitPct: 3, TakeProfitCloseRatioPct: 40, StopLossPct: 1.5, StopLossCloseRatioPct: 25},
			{TakeProfitPct: 5, TakeProfitCloseRatioPct: 60, StopLossPct: 2.5, StopLossCloseRatioPct: 75},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected ai ladder protection plan")
	}
	if plan.Mode != string(store.ProtectionModeAI) {
		t.Fatalf("expected ai mode, got %q", plan.Mode)
	}
	if len(plan.TakeProfitOrders) != 2 || len(plan.StopLossOrders) != 2 {
		t.Fatalf("expected 2 ladder tp/sl orders, got tp=%d sl=%d", len(plan.TakeProfitOrders), len(plan.StopLossOrders))
	}
	if !almostEqual(plan.TakeProfitOrders[0].Price, 97) || !almostEqual(plan.TakeProfitOrders[1].Price, 95) {
		t.Fatalf("unexpected ai ladder tp prices: %+v", plan.TakeProfitOrders)
	}
	if !almostEqual(plan.StopLossOrders[0].Price, 101.5) || !almostEqual(plan.StopLossOrders[1].Price, 102.5) {
		t.Fatalf("unexpected ai ladder sl prices: %+v", plan.StopLossOrders)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
