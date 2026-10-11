package kernel

import (
	"testing"

	"nofx/market/breakout"
	"nofx/store"
)

// The two short-side gate defaults (2026-10-04 batch-2): top-confirmation
// ON (the playbook's 默认姿态), BTC strong-bull OFF (opt-in hard pause).
func TestShortGateConfigDefaults(t *testing.T) {
	var rc store.RiskControlConfig
	if !rc.EffectiveShortTopConfirmGate() {
		t.Fatal("short_top_confirm_gate nil must default ON (playbook 默认姿态)")
	}
	if rc.EffectiveBTCFilterShort() {
		t.Fatal("btc_filter_short nil must default OFF (opt-in hard pause)")
	}
	yes, no := true, false
	if (store.RiskControlConfig{ShortTopConfirmGate: &no}).EffectiveShortTopConfirmGate() {
		t.Fatal("explicit false must disable the confirmation gate")
	}
	if !(store.RiskControlConfig{BTCFilterShort: &yes}).EffectiveBTCFilterShort() {
		t.Fatal("explicit true must enable the BTC short filter")
	}
}

// ── review 2026-10-04 #6 + #4: the mixed piggy fan-in ──
//
// The mixed branch used `piggyCoins, piggyErr := …` inside the if-block: :=
// redeclared BOTH names in the inner scope, the outer piggyCoins stayed nil,
// piggyMeta stayed empty and ScannerDirection never survived the collapse —
// SCANNER_VS_SCANNER was structurally dead in the production主力 source.
// This test pins that the direction (and the quality floor) survive.

func TestMixedPiggyDirectionSurvivesCollapse(t *testing.T) {
	breakout.DefaultScheduler().SetSnapshotForTest([]breakout.ScanResult{
		{Symbol: "AAAUSDT", Direction: breakout.DirUp, Score: 75, Grade: "strong", Pattern: breakout.PatternBreakout},
		{Symbol: "BBBUSDT", Direction: breakout.DirDown, Score: 70, Grade: "strong", Pattern: breakout.PatternRetestHold},
	})
	cfg := &store.StrategyConfig{CoinSource: store.CoinSourceConfig{
		SourceType:     "mixed",
		UsePiggyDash:   true,
		PiggyDashLimit: 5,
	}}
	e := NewStrategyEngine(cfg)
	coins, err := e.GetCandidateCoins()
	if err != nil {
		t.Fatalf("GetCandidateCoins: %v", err)
	}
	bySym := map[string]CandidateCoin{}
	for _, c := range coins {
		bySym[c.Symbol] = c
	}
	a, ok := bySym["AAAUSDT"]
	if !ok {
		t.Fatalf("AAAUSDT missing from the mixed pool: %v", coins)
	}
	if a.ScannerDirection != breakout.DirUp {
		t.Fatalf("AAA ScannerDirection = %q, want %q — the mixed := shadow is back (review #6)", a.ScannerDirection, breakout.DirUp)
	}
	if b := bySym["BBBUSDT"]; !ok || b.ScannerDirection != breakout.DirDown {
		t.Fatalf("BBB ScannerDirection = %q, want breakdown", b.ScannerDirection)
	}
	// hasScannerConflict is still direction-gated: no short_scan source here,
	// so no conflict flag — but the INPUT it needs must be non-empty.
	if a.ScannerConflict {
		t.Fatal("no short_scan source → no conflict expected")
	}
}

// The quality floor (review #4): noise-graded and approach-pattern rows must
// not fill pool slots, and an all-weak board is an ERROR (source halts, the
// kernel synthesizes a wait) — never a quiet backfill.
func TestMixedPiggyQualityFloor(t *testing.T) {
	breakout.DefaultScheduler().SetSnapshotForTest([]breakout.ScanResult{
		{Symbol: "AAAUSDT", Direction: breakout.DirUp, Score: 75, Grade: "strong", Pattern: breakout.PatternBreakout},
		{Symbol: "NOISEUSDT", Direction: breakout.DirUp, Score: 30, Grade: "noise", Pattern: breakout.PatternBreakout},
		{Symbol: "APPUSDT", Direction: breakout.DirUp, Score: 65, Grade: "medium", Pattern: breakout.PatternApproach},
		{Symbol: "EMPTYPAT", Direction: breakout.DirUp, Score: 65, Grade: "medium", Pattern: ""},
	})
	cfg := &store.StrategyConfig{CoinSource: store.CoinSourceConfig{
		SourceType:     "mixed",
		UsePiggyDash:   true,
		PiggyDashLimit: 5,
	}}
	e := NewStrategyEngine(cfg)
	coins, err := e.GetCandidateCoins()
	if err != nil {
		t.Fatalf("GetCandidateCoins: %v", err)
	}
	if len(coins) != 1 || coins[0].Symbol != "AAAUSDT" {
		t.Fatalf("pool = %v, want only AAAUSDT above the weak+cross floor", coins)
	}

	// All rows below the floor → the piggy SOURCE halts with an error (a
	// mixed pool degrades to its other sources; the dedicated piggy_dash
	// source type propagates the error and the kernel synthesizes a wait).
	breakout.DefaultScheduler().SetSnapshotForTest([]breakout.ScanResult{
		{Symbol: "WEAKUSDT", Direction: breakout.DirUp, Score: 35, Grade: "weak", Pattern: breakout.PatternApproach},
	})
	if _, err := e.getPiggyDashCoins(5, "", store.DefaultPiggyDashMinGrade); err == nil {
		t.Fatal("an all-below-floor board must error at the source, not return an empty pool")
	}
}
