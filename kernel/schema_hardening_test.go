package kernel

import (
	"encoding/json"
	"testing"
)

func TestDrawdownRuleAcceptsCloseRatioAlias(t *testing.T) {
	var rule AIProtectionDrawdownRule
	if err := json.Unmarshal([]byte(`{"min_profit_pct":2.5,"max_drawdown_pct":35,"close_ratio":40}`), &rule); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if rule.CloseRatioPct != 40 {
		t.Fatalf("expected close_ratio alias to map to CloseRatioPct=40, got %.2f", rule.CloseRatioPct)
	}
}

func TestBackfillEntryProtectionKeyLevelsFromStructuralLevels(t *testing.T) {
	ep := &AIEntryProtectionRationale{
		StructuralKeyLevels: []AIStructuralKeyLevel{
			{Price: 100, Type: "support", Timeframe: "15m", Source: "auto_detected"},
			{Price: 110, Type: "resistance", Timeframe: "15m", Source: "auto_detected"},
			{Price: 99, Type: "support", Timeframe: "5m", Source: "fibonacci_0.618"},
		},
	}
	backfillEntryProtectionKeyLevels(ep)
	if len(ep.KeyLevels.Support) != 2 {
		t.Fatalf("expected 2 support levels, got %d", len(ep.KeyLevels.Support))
	}
	if len(ep.KeyLevels.Resistance) != 1 {
		t.Fatalf("expected 1 resistance level, got %d", len(ep.KeyLevels.Resistance))
	}
	if ep.KeyLevels.Support[0] != 100 || ep.KeyLevels.Support[1] != 99 {
		t.Fatalf("expected support sorted descending [100,99], got %#v", ep.KeyLevels.Support)
	}
	if ep.KeyLevels.Resistance[0] != 110 {
		t.Fatalf("expected resistance [110], got %#v", ep.KeyLevels.Resistance)
	}
}

func TestNormalizeAndRepairOpenDecisionBackfillsStructuralKeyLevels(t *testing.T) {
	decisions := []Decision{{
		Symbol: " btcusdt ",
		Action: " OPEN_LONG ",
		EntryProtection: &AIEntryProtectionRationale{
			TimeframeContext: AIEntryTimeframeContext{Primary: " 15m "},
			KeyLevels: AIEntryKeyLevels{Support: []float64{100}, Resistance: []float64{110}},
		},
	}}
	normalizeAndRepairOpenDecisions(decisions)
	if decisions[0].Action != "open_long" {
		t.Fatalf("expected normalized action open_long, got %q", decisions[0].Action)
	}
	if decisions[0].Symbol != "BTCUSDT" {
		t.Fatalf("expected normalized symbol BTCUSDT, got %q", decisions[0].Symbol)
	}
	ep := decisions[0].EntryProtection
	if ep == nil || len(ep.StructuralKeyLevels) != 2 {
		t.Fatalf("expected 2 backfilled structural key levels, got %#v", ep)
	}
	if ep.StructuralKeyLevels[0].Timeframe != "15m" {
		t.Fatalf("expected trimmed primary timeframe 15m, got %q", ep.StructuralKeyLevels[0].Timeframe)
	}
}

func TestBackfillEntryProtectionKeyLevelsFromAnchors(t *testing.T) {
	ep := &AIEntryProtectionRationale{
		Anchors: []AIEntryProtectionAnchor{
			{Type: "support_invalidation", Timeframe: "15m", Price: 95, Reason: "base"},
			{Type: "resistance_target", Timeframe: "1h", Price: 120, Reason: "target"},
		},
	}
	backfillEntryProtectionKeyLevels(ep)
	if len(ep.KeyLevels.Support) != 1 || ep.KeyLevels.Support[0] != 95 {
		t.Fatalf("expected support [95], got %#v", ep.KeyLevels.Support)
	}
	if len(ep.KeyLevels.Resistance) != 1 || ep.KeyLevels.Resistance[0] != 120 {
		t.Fatalf("expected resistance [120], got %#v", ep.KeyLevels.Resistance)
	}
}
