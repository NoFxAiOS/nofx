package kernel

import (
	"testing"

	"nofx/store"
)

func TestValidateAIDecisionsWithStrategyRequiresLadderRoute(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode:          "full",
			TakeProfitPct: 8,
			StopLossPct:   3,
		},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected ladder-only strategy route to reject full protection_plan")
	}
}

func TestValidateAIDecisionsWithStrategyRequiresFullRoute(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode: "ladder",
			LadderRules: []AIProtectionLadderRule{{
				TakeProfitPct:           3,
				TakeProfitCloseRatioPct: 40,
				StopLossPct:             1.5,
				StopLossCloseRatioPct:   25,
			}, {
				TakeProfitPct:           6,
				TakeProfitCloseRatioPct: 60,
				StopLossPct:             3,
				StopLossCloseRatioPct:   75,
			}},
		},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected full-only strategy route to reject ladder protection_plan")
	}
}

func TestValidateAIDecisionsWithStrategyRejectsMissingLadderProtectionPlan(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected ladder-only strategy route to reject missing protection_plan")
	}
}

func TestValidateAIDecisionsWithStrategyRejectsTooManyLadderTiers(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode:        "ladder",
			LadderRules: []AIProtectionLadderRule{{TakeProfitPct: 2, TakeProfitCloseRatioPct: 20, StopLossPct: 1, StopLossCloseRatioPct: 20}, {TakeProfitPct: 4, TakeProfitCloseRatioPct: 30, StopLossPct: 2, StopLossCloseRatioPct: 30}, {TakeProfitPct: 6, TakeProfitCloseRatioPct: 30, StopLossPct: 3, StopLossCloseRatioPct: 30}, {TakeProfitPct: 8, TakeProfitCloseRatioPct: 20, StopLossPct: 4, StopLossCloseRatioPct: 20}},
		},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected ladder-only strategy route to reject ladder with more than 3 tiers")
	}
}

func TestValidateAIDecisionsWithStrategyRejectsMissingFullProtectionPlan(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected full-only strategy route to reject missing protection_plan")
	}
}

func TestValidateAIDecisionsWithStrategyRequiresBreakEvenProtectionOutput(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.BreakEvenStop = store.BreakEvenStopConfig{Enabled: true, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 3, OffsetPct: 0.1}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected break-even enabled route to reject missing break-even protection output")
	}
}

func TestValidateAIDecisionsWithStrategyRequiresDrawdownRoute(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		ProtectionPlan: &AIProtectionPlan{
			Mode:          "full",
			TakeProfitPct: 8,
			StopLossPct:   3,
		},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected drawdown-only strategy route to reject non-drawdown protection_plan")
	}
}

func TestValidateAIDecisionsWithStrategyRejectsMissingDrawdownProtectionPlan(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected drawdown-only strategy route to reject missing protection_plan")
	}
}

func TestValidateAIDecisionsWithStrategyAllowsDrawdownPlusFullOwnershipSplit(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.BreakEvenStop = store.BreakEvenStopConfig{Enabled: true, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 3, OffsetPct: 0.1}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext: AIEntryTimeframeContext{Primary: "15m", Lower: []string{"3m"}, Higher: []string{"1h"}},
			KeyLevels: AIEntryKeyLevels{Support: []float64{95}, Resistance: []float64{110}},
			Anchors: []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 95, Reason: "invalidation"}, {Type: "resistance", Timeframe: "15m", Price: 110, Reason: "target"}},
			RiskReward: AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2, NetEstimatedRR: 2, MinRequiredRR: 1.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100, ReasonAnchor: "target"}}, BreakEvenTrigger: "profit_pct", BreakEvenValue: 3, BreakEvenOffset: 0.1},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected drawdown+full ownership split to pass, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyAllowsDrawdownPlusLadderOwnershipSplit(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.BreakEvenStop = store.BreakEvenStopConfig{Enabled: true, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 3, OffsetPct: 0.1}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext: AIEntryTimeframeContext{Primary: "15m", Lower: []string{"3m"}, Higher: []string{"1h"}},
			KeyLevels: AIEntryKeyLevels{Support: []float64{95}, Resistance: []float64{110}},
			Anchors: []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 95, Reason: "invalidation"}, {Type: "resistance", Timeframe: "15m", Price: 110, Reason: "target"}},
			RiskReward: AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2, NetEstimatedRR: 2, MinRequiredRR: 1.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100, ReasonAnchor: "target"}}, BreakEvenTrigger: "profit_pct", BreakEvenValue: 3, BreakEvenOffset: 0.1},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected drawdown+ladder ownership split to pass, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRejectsEmbeddedLadderWhenDrawdownOwnsProfitTaking(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.BreakEvenStop = store.BreakEvenStopConfig{Enabled: true, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 3, OffsetPct: 0.1}

	decisions := []Decision{{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		Reasoning:       "test",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext: AIEntryTimeframeContext{Primary: "15m", Lower: []string{"3m"}, Higher: []string{"1h"}},
			KeyLevels: AIEntryKeyLevels{Support: []float64{95}, Resistance: []float64{110}},
			Anchors: []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 95, Reason: "invalidation"}, {Type: "resistance", Timeframe: "15m", Price: 110, Reason: "target"}},
			RiskReward: AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2, NetEstimatedRR: 2, MinRequiredRR: 1.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", LadderRules: []AIProtectionLadderRule{{TakeProfitPct: 2, TakeProfitCloseRatioPct: 50, StopLossPct: 1, StopLossCloseRatioPct: 50}}, DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100, ReasonAnchor: "target"}}, BreakEvenTrigger: "profit_pct", BreakEvenValue: 3, BreakEvenOffset: 0.1},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected embedded ladder rules to be rejected when drawdown owns profit-taking")
	}
}
