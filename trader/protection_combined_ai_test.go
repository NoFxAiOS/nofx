package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestBuildAIProtectionPlanCombinedLadderAndDrawdown(t *testing.T) {
	plan, err := buildAIProtectionPlan(100, "open_long", &kernel.AIProtectionPlan{
		Mode: "combined",
		LadderRules: []kernel.AIProtectionLadderRule{
			{StopLossPct: 1, StopLossCloseRatioPct: 50, StructuralAnchor: "15m support"},
			{StopLossPct: 2, StopLossCloseRatioPct: 50, StructuralAnchor: "1h support"},
		},
		DrawdownRules: []kernel.AIProtectionDrawdownRule{
			{MinProfitPct: 1.2, MaxDrawdownPct: 40, CloseRatioPct: 50, ReasonAnchor: "first target"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected combined plan")
	}
	if len(plan.StopLossOrders) != 2 {
		t.Fatalf("expected 2 AI ladder stop orders, got %+v", plan.StopLossOrders)
	}
	if plan.StopLossOrders[0].Price != 99 || plan.StopLossOrders[1].Price != 98 {
		t.Fatalf("unexpected ladder prices: %+v", plan.StopLossOrders)
	}
	if len(plan.DrawdownRules) != 1 || plan.DrawdownRules[0].MinProfitPct != 1.2 {
		t.Fatalf("expected AI drawdown rule, got %+v", plan.DrawdownRules)
	}
}

func TestBuildAIProtectionPlanCombinedLadderDrawdownAndBreakEven(t *testing.T) {
	plan, err := buildAIProtectionPlan(21.38826087, "open_long", &kernel.AIProtectionPlan{
		Mode: "combined",
		LadderRules: []kernel.AIProtectionLadderRule{
			{StopLossPrice: 21.14, StopLossCloseRatioPct: 50, StructuralAnchor: "15m support"},
		},
		DrawdownRules: []kernel.AIProtectionDrawdownRule{
			{MinProfitPct: 0.56, MaxDrawdownPct: 62, CloseRatioPct: 35, ReasonAnchor: "first target"},
		},
		BreakEvenTrigger: "profit_pct",
		BreakEvenValue:   0.55,
		BreakEvenOffset:  0.18,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected combined plan")
	}
	if plan.BreakEvenConfig == nil {
		t.Fatalf("expected combined plan to preserve AI break-even config, got %+v", plan)
	}
	if plan.BreakEvenConfig.TriggerValue != 0.55 || plan.BreakEvenConfig.OffsetPct != 0.18 {
		t.Fatalf("unexpected break-even config: %+v", plan.BreakEvenConfig)
	}
	if len(plan.DrawdownRules) != 1 || len(plan.StopLossOrders) != 1 {
		t.Fatalf("expected ladder and drawdown legs to remain, got %+v", plan)
	}
}

func TestPreferDecisionProtectionPlanDropsConfiguredPercentLadderButKeepsFallback(t *testing.T) {
	configured := &ProtectionPlan{
		Mode:                 "ai",
		RequiresNativeOrders: true,
		RequiresPartialClose: true,
		StopLossOrders:       []ProtectionOrder{{Price: 2279.31, CloseRatioPct: 50}, {Price: 2265.51, CloseRatioPct: 50}},
		NeedsStopLoss:        true,
		FallbackMaxLossPrice: 2240,
	}
	decision := &ProtectionPlan{
		Mode:                 "ai",
		RequiresNativeOrders: true,
		RequiresPartialClose: true,
		StopLossOrders:       []ProtectionOrder{{Price: 2281.55, CloseRatioPct: 50}},
		DrawdownRules:        []store.DrawdownTakeProfitRule{{MinProfitPct: 0.34, MaxDrawdownPct: 62, CloseRatioPct: 35}},
		BreakEvenConfig:      &store.BreakEvenStopConfig{Enabled: true, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 0.34, OffsetPct: 0.12},
	}

	plan := preferDecisionProtectionPlan(configured, decision)
	if len(plan.StopLossOrders) != 1 || plan.StopLossOrders[0].Price != 2281.55 {
		t.Fatalf("expected AI decision ladder to replace configured percent ladder, got %+v", plan.StopLossOrders)
	}
	if plan.FallbackMaxLossPrice != 2240 {
		t.Fatalf("expected configured fallback to be preserved, got %.2f", plan.FallbackMaxLossPrice)
	}
	if plan.BreakEvenConfig == nil || plan.BreakEvenConfig.TriggerValue != 0.34 {
		t.Fatalf("expected decision break-even config, got %+v", plan.BreakEvenConfig)
	}
}

func TestBuildAIProtectionPlanClampsEarlyFullDrawdownClose(t *testing.T) {
	plan, err := buildAIProtectionPlan(2300.01, "open_long", &kernel.AIProtectionPlan{
		Mode: "combined",
		LadderRules: []kernel.AIProtectionLadderRule{
			{StopLossPrice: 2281.55, StopLossCloseRatioPct: 50},
		},
		DrawdownRules: []kernel.AIProtectionDrawdownRule{
			{MinProfitPct: 0.28, MaxDrawdownPct: 55, CloseRatioPct: 100, ReasonAnchor: "first 15m structure", StageName: "outer_exit"},
			{MinProfitPct: 0.98, MaxDrawdownPct: 42, CloseRatioPct: 100, ReasonAnchor: "final resistance", StageName: "runner", RunnerKeepPct: 30},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.DrawdownRules) != 2 {
		t.Fatalf("expected drawdown rules, got %+v", plan.DrawdownRules)
	}
	if plan.DrawdownRules[0].CloseRatioPct != 60 {
		t.Fatalf("expected early/full drawdown close to be clamped to 60%%, got %.2f", plan.DrawdownRules[0].CloseRatioPct)
	}
	if plan.DrawdownRules[1].CloseRatioPct != 70 {
		t.Fatalf("expected runner drawdown to preserve 30%% runner, got %.2f", plan.DrawdownRules[1].CloseRatioPct)
	}
}

func TestBuildConfiguredProtectionPlanSkipsAIOwnedPercentLadder(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.LadderTPSL = store.LadderTPSLConfig{
		Enabled:           true,
		Mode:              store.ProtectionModeAI,
		StopLossEnabled:   true,
		TakeProfitEnabled: true,
		Rules: []store.LadderTPSLRule{
			{StopLossPct: 0.9, StopLossCloseRatioPct: 50, TakeProfitPct: 1, TakeProfitCloseRatioPct: 50},
			{StopLossPct: 1.5, StopLossCloseRatioPct: 50, TakeProfitPct: 2, TakeProfitCloseRatioPct: 50},
		},
	}

	plan, err := at.BuildConfiguredProtectionPlan(2300, "open_long")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan != nil {
		t.Fatalf("expected AI-owned configured ladder reference to be skipped, got %+v", plan)
	}
}

func TestBreakEvenConfigModeSelectsManualUnlessAI(t *testing.T) {
	manual := &ProtectionPlan{BreakEvenConfig: &store.BreakEvenStopConfig{Enabled: true, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 0.34, OffsetPct: 0.12}}
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.BreakEvenStop = store.BreakEvenStopConfig{Enabled: true, Mode: store.ProtectionModeManual, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 0.7, OffsetPct: 0.3}
	if got := at.getActiveBreakEvenConfigForPlan(manual); got == nil || got.TriggerValue != 0.7 || got.OffsetPct != 0.3 {
		t.Fatalf("expected manual strategy BE to win in manual mode, got %+v", got)
	}
	at.config.StrategyConfig.Protection.BreakEvenStop.Mode = store.ProtectionModeAI
	if got := at.getActiveBreakEvenConfigForPlan(manual); got == nil || got.TriggerValue != 0.34 || got.OffsetPct != 0.12 {
		t.Fatalf("expected AI plan BE to win in AI mode, got %+v", got)
	}
}

func TestAIDecisionLadderAbsolutePriceNotDoubleBuffered(t *testing.T) {
	plan, err := buildAIProtectionPlan(2300.01, "open_long", &kernel.AIProtectionPlan{
		Mode: "ladder",
		LadderRules: []kernel.AIProtectionLadderRule{
			{StopLossPrice: 2281.55, StopLossCloseRatioPct: 50, VolatilityBufferPct: 0.7},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.StopLossOrders) != 1 || plan.StopLossOrders[0].Price != 2281.55 {
		t.Fatalf("expected absolute AI ladder price unchanged, got %+v", plan.StopLossOrders)
	}
}
