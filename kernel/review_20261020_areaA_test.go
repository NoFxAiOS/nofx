package kernel

import (
	"strings"
	"testing"
	"time"

	"nofx/market"
	"nofx/store"
)

// review 2026-10-20 area4 #3: a truncated final block must not fall back to a
// full-text array search that finds the draft abandoned inside <reasoning>.
func TestExtractDecisionsUnclosedDecisionTagRejected(t *testing.T) {
	resp := "<reasoning>弃用草案 [{\"symbol\":\"BTCUSDT\",\"action\":\"close_long\"}]；\n最终选择持有。</reasoning>\n" +
		"<decision>[{\"symbol\":\"BTCUSDT\",\"action\":\"hold\"}"
	decisions, err := extractDecisions(resp)
	if err != nil || len(decisions) != 1 || decisions[0].Action != "hold" {
		t.Fatalf("expected only the final-payload hold, got %+v err=%v", decisions, err)
	}
	for _, d := range decisions {
		if d.Action == "close_long" {
			t.Fatalf("draft close_long leaked from <reasoning>: %+v", d)
		}
	}
}

// Even when the tag is closed, a reasoning section that literally mentions
// "<decision>" must not widen the payload to the draft array before it.
func TestExtractDecisionsReasoningMentioningDecisionTag(t *testing.T) {
	resp := "<reasoning>草案 <decision>[{\"symbol\":\"BTCUSDT\",\"action\":\"close_long\"}] 已弃用</reasoning>\n" +
		"<decision>[{\"symbol\":\"BTCUSDT\",\"action\":\"hold\"}]</decision>"
	decisions, err := extractDecisions(resp)
	if err != nil {
		t.Fatalf("extractDecisions failed: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Action != "hold" {
		t.Fatalf("expected the final hold, got %+v", decisions)
	}
}

func TestExtractDecisionsClosedTagStillParses(t *testing.T) {
	resp := "<reasoning>ok</reasoning><decision>\n[{\"symbol\":\"BTCUSDT\",\"action\":\"wait\"}]\n</decision>"
	decisions, err := extractDecisions(resp)
	if err != nil || len(decisions) != 1 || decisions[0].Action != "wait" {
		t.Fatalf("closed tag regression: %v %+v", err, decisions)
	}
}

// review 2026-10-20 area4 #6: with a hedged symbol (long AND short open) the
// exit signal must follow the leg being rendered, not the first listed leg.
func TestExitSignalFollowsRenderedLegOnHedgedSymbol(t *testing.T) {
	now := time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)
	cfg := store.GetDefaultStrategyConfig("zh")
	e := NewStrategyEngine(&cfg)
	td := buildMixedTrendTF("15m", now, 60, 100, false)
	data := withVendor(&market.Data{
		Symbol: "ONUSDT", CurrentPrice: lastClosedClose(td),
		TimeframeData: map[string]*market.TimeframeSeriesData{"15m": td},
	}, 0.05)
	px := lastClosedClose(td)
	ctx := &Context{
		MarketDataMap: map[string]*market.Data{"ONUSDT": data},
		Account:       AccountInfo{TotalEquity: 200},
		// long listed FIRST: the old lookup picked it for both legs.
		Positions: []PositionInfo{
			{Symbol: "ONUSDT", Side: "long", EntryPrice: px, Quantity: 1},
			{Symbol: "ONUSDT", Side: "short", EntryPrice: px, Quantity: 1},
		},
	}

	// Same data as TestComputeCoinSignalThreadsPositionSide: the long-side
	// invalidation fires, the short side (downtrend working) must not.
	if sig := e.computeCoinSignalForSide(data, nil, ctx, nil, "long"); sig == nil || !sig.ExitTriggered {
		t.Fatalf("long leg must see exit_rule_triggered (sig=%v)", sig != nil)
	}
	if sig := e.computeCoinSignalForSide(data, nil, ctx, nil, "short"); sig == nil || sig.ExitTriggered {
		t.Fatalf("short leg must NOT see the long-side exit flag (sig=%v)", sig != nil)
	}
	// Ambiguous inference (both legs held, no explicit side) stays false.
	if sig := e.computeCoinSignal(data, nil, ctx, &CandidateCoin{Symbol: "ONUSDT"}); sig == nil || sig.ExitTriggered {
		t.Fatalf("hedged symbol without an explicit side must not infer a leg (sig=%v)", sig != nil)
	}
	// The position renderer passes pos.Side: the rendered short block must
	// carry exit_rule_triggered=false.
	out := e.formatPositionInfo(2, ctx.Positions[1], ctx)
	if strings.Contains(out, `"exit_rule_triggered": true`) || strings.Contains(out, `"exit_rule_triggered":true`) {
		t.Fatalf("short leg render shows exit_rule_triggered=true:\n%s", out)
	}
}

// review 2026-10-20 area4 #7: BlockShort1dUptrend requires the 1d frame for
// every symbol, not just bstocks.
func TestRequiredSymbolTimeframesFor1dGate(t *testing.T) {
	base := []string{"15m", "1h", "4h"}
	got := withRequiredSymbolTimeframesFor(base, "BTCUSDT", true)
	want := []string{"15m", "1h", "4h", "1d"}
	if !equalStrings(got, want) {
		t.Fatalf("require1d crypto timeframes = %v, want %v", got, want)
	}
	if off := withRequiredSymbolTimeframesFor(base, "BTCUSDT", false); !equalStrings(off, base) {
		t.Fatalf("require1d=false must leave crypto untouched, got %v", off)
	}
	// already present → no duplicate
	if dup := withRequiredSymbolTimeframesFor([]string{"1h", "4h", "1d"}, "BTCUSDT", true); !equalStrings(dup, []string{"1h", "4h", "1d"}) {
		t.Fatalf("duplicate 1d: %v", dup)
	}
	if len(base) != 3 {
		t.Fatalf("input mutated: %v", base)
	}
}
