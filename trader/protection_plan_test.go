package trader

import (
	"math"
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestBuildConfiguredProtectionPlanIgnoresAIModeStrategyConfigWithoutDecisionPlan(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					FullTPSL: store.FullTPSLConfig{
						Enabled: true,
						Mode:    store.ProtectionModeAI,
						TakeProfit: store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 8},
						StopLoss:   store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 1.5},
					},
					LadderTPSL: store.LadderTPSLConfig{
						Enabled:           true,
						Mode:              store.ProtectionModeAI,
						TakeProfitEnabled: true,
						StopLossEnabled:   true,
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
	if plan != nil {
		t.Fatalf("expected nil plan when strategy protection is ai-only without decision plan, got %+v", plan)
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
		TakeProfit: store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 10},
		StopLoss:   store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 5},
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
		Rules: []store.LadderTPSLRule{
			{TakeProfitPct: 5, TakeProfitCloseRatioPct: 30, StopLossPct: 2, StopLossCloseRatioPct: 50},
			{TakeProfitPct: 10, TakeProfitCloseRatioPct: 70, StopLossPct: 4, StopLossCloseRatioPct: 50},
		},
	}
	at.config.StrategyConfig.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeManual,
		TakeProfit: store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 20},
		StopLoss:   store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 10},
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

func TestBuildManualProtectionPlanCapsLadderRatiosAt100(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeManual,
		TakeProfitEnabled: true,
		Rules: []store.LadderTPSLRule{
			{TakeProfitPct: 5, TakeProfitCloseRatioPct: 60},
			{TakeProfitPct: 8, TakeProfitCloseRatioPct: 60},
		},
	}

	plan, err := at.BuildManualProtectionPlan(100, "BTCUSDT", "open_long")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if plan == nil {
		t.Fatal("expected ladder plan")
	}
	if len(plan.TakeProfitOrders) != 2 {
		t.Fatalf("expected 2 take-profit orders, got %d", len(plan.TakeProfitOrders))
	}
	if !almostEqual(plan.TakeProfitOrders[0].CloseRatioPct, 60) || !almostEqual(plan.TakeProfitOrders[1].CloseRatioPct, 40) {
		t.Fatalf("expected close ratios to be capped to 100 total, got %+v", plan.TakeProfitOrders)
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
						StopLoss:   store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 5},
						TakeProfit: store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 10},
					},
					LadderTPSL: store.LadderTPSLConfig{
						Enabled:           true,
						Mode:              store.ProtectionModeManual,
						TakeProfitEnabled: true,
						StopLossEnabled:   true,
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
						StopLoss:   store.ProtectionThresholdRule{Enabled: true, PriceMovePct: 5},
						TakeProfit: store.ProtectionThresholdRule{Enabled: false},
					},
					LadderTPSL: store.LadderTPSLConfig{
						Enabled:           true,
						Mode:              store.ProtectionModeManual,
						TakeProfitEnabled: true,
						StopLossEnabled:   false,
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
