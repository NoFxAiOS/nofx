package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/store"
)

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
