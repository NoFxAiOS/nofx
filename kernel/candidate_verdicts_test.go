package kernel

import (
	"testing"

	"nofx/market"
	"nofx/store"
)

// 2026-10-10 per-candidate block reasons.
func TestBuildCandidateVerdicts(t *testing.T) {
	ctx := &Context{
		CandidateOrder: []string{"AUSDT", "BUSDT", "OIUSDT", "NODATAUSDT"},
		CandidateCoins: []CandidateCoin{{Symbol: "AUSDT"}, {Symbol: "BUSDT"}, {Symbol: "NODATAUSDT"}},
		GateStates: map[string]*GateState{
			market.Normalize("AUSDT"): {LongFailed: []string{"BTC_4H_DOWNTREND"}, ShortFailed: []string{"RR_MAX_0.24"}},
			market.Normalize("BUSDT"): {ShortAllowed: true, LongFailed: []string{"MICRO_TREND_NOT_LONG"}},
		},
	}
	markFiltered(ctx, "OIUSDT", "OI 3.60M < 5.0M")
	got := BuildCandidateVerdicts(ctx)
	if len(got) != 4 {
		t.Fatalf("want 4 verdicts, got %d: %+v", len(got), got)
	}
	if got[0].Status != "blocked" || got[0].LongFailed[0] != "BTC_4H_DOWNTREND" || got[0].ShortFailed[0] != "RR_MAX_0.24" {
		t.Errorf("blocked verdict wrong: %+v", got[0])
	}
	if got[1].Status != "evaluated" || len(got[1].LongFailed) != 1 {
		t.Errorf("evaluated verdict wrong: %+v", got[1])
	}
	if got[2].Symbol != "OIUSDT" || got[2].Status != "filtered" || got[2].Reason != "OI 3.60M < 5.0M" {
		t.Errorf("OI-filtered verdict wrong: %+v", got[2])
	}
	if got[3].Status != "filtered" || got[3].Reason != "not rendered (no market data)" {
		t.Errorf("no-data verdict wrong: %+v", got[3])
	}
}

// A held symbol is rendered in the positions section, not as a candidate
// block: it must not read as "no market data" (2026-10-14 BTWUSDT).
func TestBuildCandidateVerdictsHeldPosition(t *testing.T) {
	ctx := &Context{
		CandidateCoins: []CandidateCoin{{Symbol: "BTWUSDT"}, {Symbol: "NODATAUSDT"}, {Symbol: "OIUSDT"}},
		Positions:      []PositionInfo{{Symbol: "BTWUSDT", Side: "short"}, {Symbol: "OIUSDT", Side: "long"}},
	}
	markFiltered(ctx, "OIUSDT", "OI 3.60M < 5.0M")
	got := BuildCandidateVerdicts(ctx)
	if got[0].Status != "held" || got[0].Reason == "" {
		t.Errorf("held verdict wrong: %+v", got[0])
	}
	if got[1].Status != "filtered" || got[1].Reason != "not rendered (no market data)" {
		t.Errorf("no-data verdict wrong: %+v", got[1])
	}
	// an explicit filter reason still wins over the held fallback
	if got[2].Status != "filtered" || got[2].Reason != "OI 3.60M < 5.0M" {
		t.Errorf("filtered held verdict wrong: %+v", got[2])
	}
}

func TestRegimeSkipProducesVerdictsForAllCandidates(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{})
	mock := &recordingAIClient{}
	ctx := regimeCtx(t, engine, []string{"BAD1USDT", "BAD2USDT"})
	if _, err := GetFullDecisionWithStrategy(ctx, mock, engine, "balanced"); err != nil {
		t.Fatal(err)
	}
	if mock.calls != 0 {
		t.Fatalf("LLM called %d times", mock.calls)
	}
	v := BuildCandidateVerdicts(ctx)
	if len(v) != 2 {
		t.Fatalf("want 2 verdicts, got %+v", v)
	}
	for _, x := range v {
		if x.Status != "blocked" || len(x.LongFailed) == 0 || len(x.ShortFailed) == 0 {
			t.Errorf("regime-skip candidate should be blocked with codes both ways: %+v", x)
		}
	}
}
