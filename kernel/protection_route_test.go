package kernel

import (
	"strings"
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
			KeyLevels:        AIEntryKeyLevels{Support: []float64{95}, Resistance: []float64{110}},
			Anchors:          []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 95, Reason: "invalidation"}, {Type: "resistance", Timeframe: "15m", Price: 110, Reason: "target"}},
			RiskReward:       AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2, NetEstimatedRR: 2, MinRequiredRR: 1.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", DrawdownRules: []AIProtectionDrawdownRule{
			{MinProfitPct: 5, MaxDrawdownPct: 60, CloseRatioPct: 40, ReasonAnchor: "15m target partial lock"},
			{MinProfitPct: 8, MaxDrawdownPct: 40, CloseRatioPct: 60, ReasonAnchor: "1h trend extension runner"},
		}, BreakEvenTrigger: "profit_pct", BreakEvenValue: 3, BreakEvenOffset: 0.1},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected drawdown+full ownership split to pass, got %v", err)
	}
}

func TestValidateAIDecisionsWithStrategyRequiresCombinedPlanForDrawdownPlusLadderAI(t *testing.T) {
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
			KeyLevels:        AIEntryKeyLevels{Support: []float64{95}, Resistance: []float64{110}},
			Anchors:          []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 95, Reason: "invalidation"}, {Type: "resistance", Timeframe: "15m", Price: 110, Reason: "target"}},
			RiskReward:       AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2, NetEstimatedRR: 2, MinRequiredRR: 1.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100, ReasonAnchor: "target"}}, BreakEvenTrigger: "profit_pct", BreakEvenValue: 3, BreakEvenOffset: 0.1},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err == nil {
		t.Fatal("expected drawdown+ladder AI route to reject drawdown-only plan")
	}
}

func TestValidateAIDecisionsWithStrategyAllowsCombinedDrawdownPlusLadderAI(t *testing.T) {
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
			KeyLevels:        AIEntryKeyLevels{Support: []float64{95}, Resistance: []float64{110}},
			Anchors:          []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 95, Reason: "invalidation"}, {Type: "resistance", Timeframe: "15m", Price: 110, Reason: "target"}},
			RiskReward:       AIRiskRewardRationale{Entry: 100, Invalidation: 95, FirstTarget: 110, GrossEstimatedRR: 2, NetEstimatedRR: 2, MinRequiredRR: 1.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{
			Mode: "combined",
			LadderRules: []AIProtectionLadderRule{
				{StopLossPrice: 95, StopLossCloseRatioPct: 50, StructuralAnchor: "15m support", VolatilityBufferReason: "15m ATR buffer"},
				{StopLossPrice: 95, StopLossCloseRatioPct: 50, StructuralAnchor: "1h support", VolatilityBufferReason: "1h ATR buffer"},
			},
			DrawdownRules: []AIProtectionDrawdownRule{
				{MinProfitPct: 5, MaxDrawdownPct: 60, CloseRatioPct: 40, ReasonAnchor: "15m target partial lock"},
				{MinProfitPct: 8, MaxDrawdownPct: 40, CloseRatioPct: 60, ReasonAnchor: "1h trend extension runner"},
			},
			BreakEvenTrigger: "profit_pct", BreakEvenValue: 3, BreakEvenOffset: 0.1,
		},
	}}

	if err := ValidateAIDecisionsWithStrategy(decisions, cfg); err != nil {
		t.Fatalf("expected combined drawdown+ladder AI plan to pass, got %v", err)
	}
}

func TestFilterInvalidAIDecisionsWithStrategyDropsOnlyInvalidOpenDecision(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 2.5
	cfg.EntryStructure.Enabled = true
	cfg.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	invalidZEC := Decision{
		Symbol:          "ZECUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 100,
		StopLoss:        328.7,
		TakeProfit:      342,
		Reasoning:       "bad anchor should be rejected without poisoning ADA",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext: AIEntryTimeframeContext{Primary: "15m", Lower: []string{"3m"}, Higher: []string{"1h"}},
			KeyLevels:        AIEntryKeyLevels{Support: []float64{329.5361, 326}, Resistance: []float64{340}},
			Anchors:          []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 329.5361, Reason: "invalidation"}, {Type: "resistance", Timeframe: "15m", Price: 340, Reason: "target"}},
			RiskReward:       AIRiskRewardRationale{Entry: 330, Invalidation: 328.7, FirstTarget: 340, GrossEstimatedRR: 7.69, NetEstimatedRR: 7.5, MinRequiredRR: 2.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 1, MaxDrawdownPct: 0.5, CloseRatioPct: 50, ReasonAnchor: "15m target"}, {MinProfitPct: 2, MaxDrawdownPct: 1, CloseRatioPct: 50, ReasonAnchor: "1h runner"}}},
	}
	validADA := Decision{
		Symbol:          "ADAUSDT",
		Action:          "open_short",
		Leverage:        3,
		PositionSizeUSD: 100,
		StopLoss:        0.71,
		TakeProfit:      0.65,
		Reasoning:       "valid sibling proposal",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext: AIEntryTimeframeContext{Primary: "15m", Lower: []string{"3m"}, Higher: []string{"1h"}},
			KeyLevels:        AIEntryKeyLevels{Support: []float64{0.66}, Resistance: []float64{0.71}},
			Anchors:          []AIEntryProtectionAnchor{{Type: "resistance", Timeframe: "15m", Price: 0.71, Reason: "invalidation"}, {Type: "support", Timeframe: "15m", Price: 0.66, Reason: "target"}},
			RiskReward:       AIRiskRewardRationale{Entry: 0.70, Invalidation: 0.71, FirstTarget: 0.66, GrossEstimatedRR: 4, NetEstimatedRR: 3.8, MinRequiredRR: 2.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 1, MaxDrawdownPct: 0.5, CloseRatioPct: 50, ReasonAnchor: "15m support"}, {MinProfitPct: 2, MaxDrawdownPct: 1, CloseRatioPct: 50, ReasonAnchor: "1h runner"}}},
	}

	filtered, rejected := FilterInvalidAIDecisionsWithStrategy([]Decision{invalidZEC, validADA}, cfg)
	if len(rejected) != 1 {
		t.Fatalf("expected one rejected decision, got %d", len(rejected))
	}
	if rejected[0].Index != 0 || rejected[0].Decision.Symbol != "ZECUSDT" {
		t.Fatalf("expected ZEC decision #1 rejected, got index=%d symbol=%s", rejected[0].Index, rejected[0].Decision.Symbol)
	}
	if len(filtered) != 1 || filtered[0].Symbol != "ADAUSDT" {
		t.Fatalf("expected only ADA to remain, got %#v", filtered)
	}
}

func TestFilterInvalidAIDecisionsWithStrategyRejectsOnlyBadOpenProposal(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.EntryStructure.Enabled = true
	cfg.EntryStructure.RequireSupportResistance = true
	cfg.EntryStructure.RequireStructuralAnchors = true
	cfg.RiskControl.MinRiskRewardRatio = 2.5
	cfg.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI}
	cfg.Protection.FullTPSL = store.FullTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeDisabled}

	badZEC := Decision{
		Symbol:          "ZECUSDT",
		Action:          "open_long",
		Leverage:        1,
		PositionSizeUSD: 40,
		StopLoss:        328.7,
		TakeProfit:      336.410625,
		Reasoning:       "invalid buffered stop not present as structural reference",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext: AIEntryTimeframeContext{Primary: "15m", Lower: []string{"3m"}, Higher: []string{"1h"}},
			KeyLevels:        AIEntryKeyLevels{Support: []float64{329.53614292}, Resistance: []float64{333.24487643}},
			Anchors:          []AIEntryProtectionAnchor{{Type: "support", Timeframe: "15m", Price: 329.53614292, Reason: "support"}, {Type: "resistance", Timeframe: "15m", Price: 333.24487643, Reason: "target"}},
			RiskReward:       AIRiskRewardRationale{Entry: 329.84, Invalidation: 328.7, FirstTarget: 333.24487643, GrossEstimatedRR: 2.99, NetEstimatedRR: 2.75, MinRequiredRR: 2.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 0.95, MaxDrawdownPct: 68, CloseRatioPct: 50, ReasonAnchor: "tp1"}, {MinProfitPct: 2.0, MaxDrawdownPct: 55, CloseRatioPct: 50, ReasonAnchor: "runner"}}},
	}
	validADA := Decision{
		Symbol:          "ADAUSDT",
		Action:          "open_short",
		Leverage:        1,
		PositionSizeUSD: 40,
		StopLoss:        0.24575,
		TakeProfit:      0.24351537,
		Reasoning:       "valid short proposal",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext:    AIEntryTimeframeContext{Primary: "15m", Lower: []string{"3m"}, Higher: []string{"1h"}},
			KeyLevels:           AIEntryKeyLevels{Support: []float64{0.24351537}, Resistance: []float64{0.24575}},
			StructuralKeyLevels: []AIStructuralKeyLevel{{Price: 0.24575, Type: "resistance", Timeframe: "15m", Source: "ATR_buffered_structural_stop", UsedFor: "stop_loss"}, {Price: 0.24351537, Type: "support", Timeframe: "15m", Source: "structure", UsedFor: "tp1"}},
			Anchors:             []AIEntryProtectionAnchor{{Type: "resistance", Timeframe: "15m", Price: 0.24531653, Reason: "resistance"}, {Type: "support", Timeframe: "15m", Price: 0.24351537, Reason: "target"}},
			RiskReward:          AIRiskRewardRationale{Entry: 0.2452, Invalidation: 0.24575, FirstTarget: 0.24351537, GrossEstimatedRR: 3.06, NetEstimatedRR: 2.85, MinRequiredRR: 2.5, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "drawdown", DrawdownRules: []AIProtectionDrawdownRule{{MinProfitPct: 0.68, MaxDrawdownPct: 70, CloseRatioPct: 50, ReasonAnchor: "tp1"}, {MinProfitPct: 1.4, MaxDrawdownPct: 55, CloseRatioPct: 50, ReasonAnchor: "runner"}}},
	}

	filtered, rejected := FilterInvalidAIDecisionsWithStrategy([]Decision{badZEC, validADA}, cfg)
	if len(rejected) != 1 || rejected[0].Index != 0 || rejected[0].Decision.Symbol != "ZECUSDT" {
		t.Fatalf("expected only ZEC rejected, got %#v", rejected)
	}
	if len(filtered) != 1 || filtered[0].Symbol != "ADAUSDT" {
		t.Fatalf("expected ADA to remain executable, got %#v", filtered)
	}
}

func TestValidateLadderPlanRejectsUnanchoredAbsoluteTarget(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.EntryStructure.Enabled = true
	cfg.EntryStructure.RequireSupportResistance = true
	cfg.EntryStructure.RequireStructuralAnchors = true
	cfg.RiskControl.MinRiskRewardRatio = 2
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}

	decision := Decision{
		Symbol:          "BZUSDT",
		Action:          "open_short",
		Leverage:        1,
		PositionSizeUSD: 50,
		Reasoning:       "ladder structure test",
		StopLoss:        113.14,
		TakeProfit:      111.83,
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext:    AIEntryTimeframeContext{Primary: "15m", Higher: []string{"1h"}},
			KeyLevels:           AIEntryKeyLevels{Support: []float64{111.82698}, Resistance: []float64{113.14}},
			StructuralKeyLevels: []AIStructuralKeyLevel{{Price: 113.14, Type: "resistance", Timeframe: "15m", UsedFor: "stop_loss"}, {Price: 111.82698, Type: "support", Timeframe: "15m", Source: "fibonacci", UsedFor: "tp1"}},
			Anchors:             []AIEntryProtectionAnchor{{Type: "resistance", Timeframe: "15m", Price: 113.14, Reason: "buffered stop"}, {Type: "support", Timeframe: "15m", Price: 111.82698, Reason: "fib target"}},
			RiskReward:          AIRiskRewardRationale{Entry: 112.8, Invalidation: 113.14, FirstTarget: 111.82698, GrossEstimatedRR: 2.86, NetEstimatedRR: 2.6, MinRequiredRR: 2, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "ladder", LadderRules: []AIProtectionLadderRule{{TakeProfitPrice: 109.4, TakeProfitCloseRatioPct: 50, StopLossPrice: 115, StopLossCloseRatioPct: 50, StructuralAnchor: "15m fib support", VolatilityBufferReason: "15m ATR buffer"}, {TakeProfitPrice: 108.8, TakeProfitCloseRatioPct: 50, StopLossPrice: 115.5, StopLossCloseRatioPct: 50, StructuralAnchor: "15m fib support", VolatilityBufferReason: "15m ATR buffer"}}},
	}

	if err := ValidateAIDecisionsWithStrategy([]Decision{decision}, cfg); err == nil || !strings.Contains(err.Error(), "ladder_rules[0] take profit") {
		t.Fatalf("expected ladder structural alignment rejection, got %v", err)
	}
}

func TestValidateLadderPlanAcceptsAbsoluteStructuralPriceLevels(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.EntryStructure.Enabled = true
	cfg.EntryStructure.RequireSupportResistance = true
	cfg.EntryStructure.RequireStructuralAnchors = true
	cfg.RiskControl.MinRiskRewardRatio = 2
	cfg.Protection.LadderTPSL = store.LadderTPSLConfig{Enabled: true, Mode: store.ProtectionModeAI}

	decision := Decision{
		Symbol:          "BZUSDT",
		Action:          "open_short",
		Leverage:        1,
		PositionSizeUSD: 50,
		Reasoning:       "ladder structure test",
		StopLoss:        113.14,
		TakeProfit:      111.83,
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext:    AIEntryTimeframeContext{Primary: "15m", Higher: []string{"1h"}},
			KeyLevels:           AIEntryKeyLevels{Support: []float64{111.82698, 111.19208}, Resistance: []float64{113.14}},
			StructuralKeyLevels: []AIStructuralKeyLevel{{Price: 113.14, Type: "resistance", Timeframe: "15m", UsedFor: "stop_loss"}, {Price: 111.82698, Type: "support", Timeframe: "15m", Source: "fibonacci", UsedFor: "tp1"}, {Price: 111.19208, Type: "support", Timeframe: "15m", Source: "fibonacci", UsedFor: "tp2"}},
			Anchors:             []AIEntryProtectionAnchor{{Type: "resistance", Timeframe: "15m", Price: 113.14, Reason: "buffered stop"}, {Type: "support", Timeframe: "15m", Price: 111.82698, Reason: "fib target"}},
			RiskReward:          AIRiskRewardRationale{Entry: 112.8, Invalidation: 113.14, FirstTarget: 111.82698, GrossEstimatedRR: 2.86, NetEstimatedRR: 2.6, MinRequiredRR: 2, Passed: true},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "ladder", LadderRules: []AIProtectionLadderRule{{TakeProfitPrice: 111.82698, TakeProfitCloseRatioPct: 50, StopLossPrice: 113.14, StopLossCloseRatioPct: 50, StructuralAnchor: "15m fib support", VolatilityBufferReason: "15m ATR buffer"}, {TakeProfitPrice: 111.19208, TakeProfitCloseRatioPct: 50, StopLossPrice: 113.14, StopLossCloseRatioPct: 50, StructuralAnchor: "15m fib support", VolatilityBufferReason: "15m ATR buffer"}}},
	}

	if err := ValidateAIDecisionsWithStrategy([]Decision{decision}, cfg); err != nil {
		t.Fatalf("expected absolute structural ladder prices to pass, got %v", err)
	}
}

func TestNormalizeAndRepairLadderPlanSnapsPercentRulesToNearestStructure(t *testing.T) {
	decisions := []Decision{{
		Symbol: "BZUSDT",
		Action: "open_short",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext:    AIEntryTimeframeContext{Primary: "15m"},
			KeyLevels:           AIEntryKeyLevels{Support: []float64{111.82698}, Resistance: []float64{113.14}},
			StructuralKeyLevels: []AIStructuralKeyLevel{{Price: 113.14, Type: "resistance", Timeframe: "15m", UsedFor: "stop_loss"}, {Price: 111.82698, Type: "support", Timeframe: "15m", Source: "fibonacci", UsedFor: "tp1"}},
			RiskReward:          AIRiskRewardRationale{Entry: 112.8, Invalidation: 113.14, FirstTarget: 111.82698},
		},
		ProtectionPlan: &AIProtectionPlan{Mode: "ladder", LadderRules: []AIProtectionLadderRule{{TakeProfitPct: 3, TakeProfitCloseRatioPct: 50, StopLossPct: 1.5, StopLossCloseRatioPct: 50}}},
	}}

	normalizeAndRepairOpenDecisions(decisions)
	got := decisions[0].ProtectionPlan.LadderRules[0]
	if got.TakeProfitPrice != 111.82698 || got.StopLossPrice != 113.14 {
		t.Fatalf("expected ladder percent rules snapped to structure, got %+v", got)
	}
}

func TestFullProtectionPlanAcceptsAbsoluteStructuralPrices(t *testing.T) {
	decisions := []Decision{{
		Symbol:          "HYPEUSDT",
		Action:          "open_long",
		Leverage:        1,
		PositionSizeUSD: 50,
		ProtectionPlan: &AIProtectionPlan{
			Mode:            "full",
			StopLossPrice:   38.953,
			StopLossPct:     0.602,
			TakeProfitPrice: 40.324,
			TakeProfitPct:   2.897,
		},
		Reasoning: "test",
	}}
	if err := ValidateDecisionFormat(decisions); err != nil {
		t.Fatalf("expected full absolute protection prices to validate, got %v", err)
	}
}
