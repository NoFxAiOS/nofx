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
	if plan.DrawdownRules[0].CloseRatioPct != 65 {
		t.Fatalf("expected early/full drawdown close to be clamped to 65%%, got %.2f", plan.DrawdownRules[0].CloseRatioPct)
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

func TestClampAIDrawdownTierCeilingsMinProfitPct(t *testing.T) {
	cfg := store.DrawdownTakeProfitConfig{
		Enabled:    true,
		EngineMode: store.DrawdownEngineModeAI,
		Rules: []store.DrawdownTakeProfitRule{
			{MinProfitPct: 0.8, MaxDrawdownPct: 60, CloseRatioPct: 50},
			{MinProfitPct: 1.5, MaxDrawdownPct: 55, CloseRatioPct: 80},
			{MinProfitPct: 2.5, MaxDrawdownPct: 45, CloseRatioPct: 100},
		},
	}
	// AI outputs min_profit_pct above strategy ceilings
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 1.38, MaxDrawdownPct: 62, CloseRatioPct: 65, RunnerKeepPct: 35, StageName: "first"},
		{MinProfitPct: 2.99, MaxDrawdownPct: 55, CloseRatioPct: 45, RunnerKeepPct: 20, StageName: "second"},
		{MinProfitPct: 3.91, MaxDrawdownPct: 48, CloseRatioPct: 100, StageName: "third"},
	}
	clamped := clampAIDrawdownTierCeilings(rules, cfg)
	if clamped[0].MinProfitPct > 0.8 {
		t.Errorf("tier 1 min_profit_pct should be clamped to <=0.8, got %.4f", clamped[0].MinProfitPct)
	}
	if clamped[1].MinProfitPct > 1.5 {
		t.Errorf("tier 2 min_profit_pct should be clamped to <=1.5, got %.4f", clamped[1].MinProfitPct)
	}
	if clamped[2].MinProfitPct > 2.5 {
		t.Errorf("tier 3 min_profit_pct should be clamped to <=2.5, got %.4f", clamped[2].MinProfitPct)
	}
}

func TestClampAIDrawdownTierCeilingsFirstTierAllocation(t *testing.T) {
	cfg := store.DrawdownTakeProfitConfig{
		Enabled:    true,
		EngineMode: store.DrawdownEngineModeAI,
		Rules: []store.DrawdownTakeProfitRule{
			{MinProfitPct: 0.8, MaxDrawdownPct: 60, CloseRatioPct: 50},
		},
	}
	// AI outputs first tier with only 35% close (too low)
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 0.7, MaxDrawdownPct: 60, CloseRatioPct: 35, RunnerKeepPct: 65, StageName: "first"},
		{MinProfitPct: 1.2, MaxDrawdownPct: 55, CloseRatioPct: 80, StageName: "second"},
	}
	clamped := clampAIDrawdownTierCeilings(rules, cfg)
	if clamped[0].CloseRatioPct < 50 {
		t.Errorf("tier 1 close_ratio_pct should be clamped up to >=50, got %.1f", clamped[0].CloseRatioPct)
	}
	if clamped[0].RunnerKeepPct > 35 {
		t.Errorf("tier 1 runner_keep_pct should be clamped to <=35, got %.1f", clamped[0].RunnerKeepPct)
	}
}

func TestClampAIDrawdownTierCeilingsAlreadyCompliant(t *testing.T) {
	cfg := store.DrawdownTakeProfitConfig{
		Enabled:    true,
		EngineMode: store.DrawdownEngineModeAI,
		Rules: []store.DrawdownTakeProfitRule{
			{MinProfitPct: 0.8, MaxDrawdownPct: 60, CloseRatioPct: 50},
			{MinProfitPct: 1.5, MaxDrawdownPct: 55, CloseRatioPct: 80},
		},
	}
	// AI outputs compliant values
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 0.75, MaxDrawdownPct: 60, CloseRatioPct: 65, RunnerKeepPct: 35, StageName: "first"},
		{MinProfitPct: 1.4, MaxDrawdownPct: 55, CloseRatioPct: 80, StageName: "second"},
	}
	clamped := clampAIDrawdownTierCeilings(rules, cfg)
	if clamped[0].MinProfitPct != 0.75 {
		t.Errorf("compliant tier 1 should not be changed, got %.4f", clamped[0].MinProfitPct)
	}
	if clamped[1].MinProfitPct != 1.4 {
		t.Errorf("compliant tier 2 should not be changed, got %.4f", clamped[1].MinProfitPct)
	}
}

func TestVolatilityAutoWidenSL(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.EntryStructure.EntryGate = store.EntryGateConfig{
		Enabled:             true,
		MinSLDistanceATRMul: 1.2,
		MinATR14Pct:         1.0,
	}
	cfg.EntryStructure.EntryGate.VolatilityBufferATRMul = 0.5

	// Entry=100, ATR14=1%, so ATR absolute = 1.0
	// Effective min SL = (1.2 + 0.5) * 1.0 = 1.7
	// Ladder SL at 99.5 = 0.5 away from entry → should be widened to 98.3
	plan, err := buildAIDecisionLadderProtectionPlan(100, "open_long", []kernel.AIProtectionLadderRule{
		{StopLossPrice: 99.5, StopLossCloseRatioPct: 100, VolatilityBufferPct: 0.35},
	}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil || len(plan.StopLossOrders) != 1 {
		t.Fatal("expected 1 SL order")
	}
	sl := plan.StopLossOrders[0].Price
	if sl >= 99.5 {
		t.Errorf("SL should be widened below 99.5, got %.4f", sl)
	}
	if sl > 98.4 {
		t.Errorf("SL should be widened to ~98.3, got %.4f", sl)
	}
}

func TestVolatilityAutoWidenSLShort(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.EntryStructure.EntryGate = store.EntryGateConfig{
		Enabled:             true,
		MinSLDistanceATRMul: 1.2,
		MinATR14Pct:         1.0,
	}
	cfg.EntryStructure.EntryGate.VolatilityBufferATRMul = 0.5

	// Entry=100, ATR14=1%, ATR abs=1.0
	// SL at 100.5 = 0.5 away → too tight → should widen to ~101.7
	plan, err := buildAIDecisionLadderProtectionPlan(100, "open_short", []kernel.AIProtectionLadderRule{
		{StopLossPrice: 100.5, StopLossCloseRatioPct: 100, VolatilityBufferPct: 0.35},
	}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil || len(plan.StopLossOrders) != 1 {
		t.Fatal("expected 1 SL order")
	}
	sl := plan.StopLossOrders[0].Price
	if sl <= 100.5 {
		t.Errorf("short SL should be widened above 100.5, got %.4f", sl)
	}
	if sl < 101.6 {
		t.Errorf("short SL should be widened to ~101.7, got %.4f", sl)
	}
}

func TestVolatilityNoWidenWhenSufficientDistance(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.EntryStructure.EntryGate = store.EntryGateConfig{
		Enabled:             true,
		MinSLDistanceATRMul: 1.2,
		MinATR14Pct:         1.0,
	}
	cfg.EntryStructure.EntryGate.VolatilityBufferATRMul = 0.3

	// Entry=100, VolatilityBufferATRMul=0.3, bufferMul=0.3*0.7=0.21
	// VolatilityBufferPct should be ATR14Pct*bufferMul=1.0*0.21=0.21
	// atr14Pct reverse = 0.21/0.21 = 1.0, ATR abs = 1.0
	// effective min = (1.2+0.3)*1.0 = 1.5
	// SL at 97.0 = 3.0 away → already sufficient → no widening
	plan, err := buildAIDecisionLadderProtectionPlan(100, "open_long", []kernel.AIProtectionLadderRule{
		{StopLossPrice: 97.0, StopLossCloseRatioPct: 100, VolatilityBufferPct: 0.21},
	}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil || len(plan.StopLossOrders) != 1 {
		t.Fatal("expected 1 SL order")
	}
	if plan.StopLossOrders[0].Price != 97.0 {
		t.Errorf("SL should not be widened, expected 97.0 got %.4f", plan.StopLossOrders[0].Price)
	}
}

func TestVolatilityNoWidenWhenBufferDisabled(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.EntryStructure.EntryGate = store.EntryGateConfig{
		Enabled:             true,
		MinSLDistanceATRMul: 1.2,
		MinATR14Pct:         1.0,
	}
	cfg.EntryStructure.EntryGate.VolatilityBufferATRMul = -1 // disabled

	// With buffer disabled (clamped to 0), no widening should happen
	plan, err := buildAIDecisionLadderProtectionPlan(100, "open_long", []kernel.AIProtectionLadderRule{
		{StopLossPrice: 99.5, StopLossCloseRatioPct: 100, VolatilityBufferPct: 0.35},
	}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil || len(plan.StopLossOrders) != 1 {
		t.Fatal("expected 1 SL order")
	}
	if plan.StopLossOrders[0].Price != 99.5 {
		t.Errorf("SL should NOT be widened when buffer disabled, expected 99.5 got %.4f", plan.StopLossOrders[0].Price)
	}
}
