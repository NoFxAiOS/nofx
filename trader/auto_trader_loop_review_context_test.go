package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func TestBuildRuntimePolicyControlOutcomeAcceptedSummary(t *testing.T) {
	out := buildRuntimePolicyControlOutcome(runtimePolicyResult{
		OriginalAction:     "open_long",
		FinalAction:        "open_long",
		ConstraintsMerged:  true,
		RRRecomputed:       true,
		AIGrossRR:          2,
		AINetRR:            1.8,
		RuntimeGrossRR:     1.95,
		RuntimeNetRR:       1.72,
		EffectiveRR:        1.72,
		EffectiveRRSource:  "runtime_net",
		ConstraintsSources: []string{"binance:instrument", "binance:ticker"},
	})
	if out == nil {
		t.Fatal("expected control outcome")
	}
	if out.Decision != "accepted" || !out.ConstraintsMerged || !out.RuntimeRRRecomputed {
		t.Fatalf("unexpected control outcome header: %+v", out)
	}
	if out.OriginalAction != "open_long" || out.FinalAction != "open_long" {
		t.Fatalf("expected original/final action audit on accepted outcome, got %+v", out)
	}
	if out.RuntimeNetRR != 1.72 || out.EffectiveRRSource != "runtime_net" {
		t.Fatalf("unexpected rr summary: %+v", out)
	}
	if len(out.ExecutionConstraintSources) != 2 {
		t.Fatalf("unexpected constraint sources: %+v", out)
	}
}

func TestBuildRuntimePolicyControlOutcomeRejectedSummary(t *testing.T) {
	out := buildRuntimePolicyControlOutcome(runtimePolicyResult{
		Blocked:        true,
		Decision:       "rejected",
		OriginalAction: "open_long",
		FinalAction:    "open_long",
		Reason:         "runtime RR policy blocked open_long BTCUSDT: execution-aware rr 1.20 below min 1.50",
		ReasonCode:     "runtime_rr_below_min",
		EffectiveRR:    1.2,
	})
	if out == nil {
		t.Fatal("expected control outcome")
	}
	if out.Decision != "rejected" || !out.NoOrderPlaced {
		t.Fatalf("expected rejected/no_order_placed outcome, got %+v", out)
	}
	if out.OriginalAction != "open_long" || out.FinalAction != "open_long" {
		t.Fatalf("expected strict reject to retain original/final action, got %+v", out)
	}
	if len(out.FailedChecks) != 1 || out.FailedChecks[0] != "runtime_rr_below_min" {
		t.Fatalf("unexpected failed checks: %+v", out)
	}
}

func TestBuildDecisionActionReviewContextMapsEntryProtectionCompactly(t *testing.T) {
	decision := &kernel.Decision{
		Symbol: "BTCUSDT",
		Action: "open_long",
		EntryProtection: &kernel.AIEntryProtectionRationale{
			TimeframeContext: kernel.AIEntryTimeframeContext{Primary: "15m"},
			RiskReward: kernel.AIRiskRewardRationale{
				Entry:            100,
				Invalidation:     95,
				FirstTarget:      110,
				GrossEstimatedRR: 2.0,
				NetEstimatedRR:   1.8,
			},
			KeyLevels: kernel.AIEntryKeyLevels{
				Support:    []float64{99, 95, 91},
				Resistance: []float64{110, 115, 120},
			},
			Anchors: []kernel.AIEntryProtectionAnchor{
				{Type: "support", Timeframe: "15m", Price: 99, Reason: "retest"},
				{Type: "resistance", Timeframe: "1h", Price: 110, Reason: "first target"},
				{Type: "volume", Timeframe: "5m", Price: 101, Reason: "breakout"},
				{Type: "extra", Timeframe: "1d", Price: 120, Reason: "ignored"},
			},
			AlignmentNotes: []string{"full stop remains beyond invalidation", "break-even before target"},
		},
	}

	ctx := buildDecisionActionReviewContext(decision, 1.5, &store.ProtectionSnapshot{
		FullTPSL: &store.ProtectionSnapshotFullTPSL{
			Enabled:    true,
			Mode:       "full",
			StopLoss:   store.ProtectionSnapshotValueSource{Mode: "percent", Value: 5},
			TakeProfit: store.ProtectionSnapshotValueSource{Mode: "percent", Value: 10},
		},
		BreakEven: &store.ProtectionSnapshotBreakEven{Enabled: true, TriggerMode: "profit_pct", TriggerValue: 4},
	})
	if ctx == nil {
		t.Fatal("expected review context")
	}
	if ctx.PrimaryTimeframe != "15m" || ctx.MinRiskReward != 1.5 {
		t.Fatalf("unexpected header context: %+v", ctx)
	}
	if ctx.RiskReward == nil || ctx.RiskReward.NetEstimatedRR != 1.8 || !ctx.RiskReward.Passed {
		t.Fatalf("unexpected risk reward context: %+v", ctx.RiskReward)
	}
	if ctx.KeyLevels == nil || len(ctx.KeyLevels.Support) != 2 || len(ctx.KeyLevels.Resistance) != 2 {
		t.Fatalf("expected compact top support/resistance only, got %+v", ctx.KeyLevels)
	}
	if len(ctx.Anchors) != 3 || ctx.Anchors[0].Reason != "retest" {
		t.Fatalf("expected compact anchors, got %+v", ctx.Anchors)
	}
	if ctx.Protection == nil || !ctx.Protection.StopBeyondInvalidation || !ctx.Protection.TargetAligned || !ctx.Protection.BreakEvenBeforeTarget {
		t.Fatalf("expected protection alignment summary, got %+v", ctx.Protection)
	}
}

func TestBuildDecisionActionReviewContextKeepsWaitCompact(t *testing.T) {
	ctx := buildDecisionActionReviewContext(&kernel.Decision{Symbol: "BTCUSDT", Action: "wait"}, 0, nil)
	if ctx != nil {
		t.Fatalf("wait without rationale should not emit action review context, got %+v", ctx)
	}
}
