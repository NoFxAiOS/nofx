package trader

import (
	"math"
	"testing"
	"nofx/store"
)

func TestBuildConfiguredProtectionPlanSupportsAIMode(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					LadderTPSL: store.LadderTPSLConfig{
						Enabled:           true,
						Mode:              store.ProtectionModeAI,
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
		t.Fatal("expected plan from AI-mode strategy config")
	}
	if plan.Mode != string(store.ProtectionModeAI) {
		t.Fatalf("expected ai mode, got %q", plan.Mode)
	}
	if len(plan.TakeProfitOrders) != 1 || len(plan.StopLossOrders) != 1 {
		t.Fatalf("expected 1 tp and 1 sl ladder order, got tp=%d sl=%d", len(plan.TakeProfitOrders), len(plan.StopLossOrders))
	}
}

func TestBuildManualProtectionPlanFallsBackToFullTPSL(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
	}
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
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
	}
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
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
	}
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

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
