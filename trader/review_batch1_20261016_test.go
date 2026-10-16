package trader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

// ── 1. new traders never adopt existing positions (review A) ────────────────

// A freshly created trader (no flag file, empty registry) over an account that
// already holds a manual position must not mark it AI-managed anywhere on the
// decision path. The old seedAIManagedOnce adopted every open position.
func TestFreshTraderDoesNotAdoptExistingPositions(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil { // seed flag files lived under ./data
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	at, _ := newBatch1RiskTrader("long", 1, 100, store.RiskControlConfig{})
	at.store = profitLockTestStore(t)
	at.id = "fresh-trader"
	at.name = "fresh"

	// The decision cycle's gate stage (where the seeding used to run) with a
	// non-empty batch, against an account holding a manual XUSDT long.
	decisions := []kernel.Decision{{Symbol: "XUSDT", Action: "hold"}}
	_ = at.applyHardRiskGates(decisions, &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}})

	if at.isAIManaged("XUSDT", "long") {
		t.Fatal("pre-existing exchange position was adopted as AI-managed by a fresh trader")
	}
	marks, err := at.store.AIManaged().List(at.id)
	if err != nil || len(marks) != 0 {
		t.Fatalf("registry must stay empty: marks=%v err=%v", marks, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "data")); err == nil {
		t.Fatal("no seed flag/data directory may be created")
	}
}

// Structural guard: the adoption helper and its flag file must not come back.
func TestSeedAIManagedOnceRemoved(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, banned := range []string{"seedAIManagedOnce()", "ai_marks_seeded_"} {
			if strings.Contains(src, banned) {
				t.Fatalf("%s still contains %q — new traders would adopt manual positions", f, banned)
			}
		}
	}
}

// ── 2. TP-runner conversion cancels only its own side (review B) ───────────

type tpCancelMock struct {
	batch1RiskMock
	symbolWide int
	sideCalls  []string
}

func (m *tpCancelMock) CancelTakeProfitOrders(string) error { m.symbolWide++; return nil }

// sideTPCancelMock additionally implements the side-scoped primitive (Binance).
type sideTPCancelMock struct{ tpCancelMock }

func (m *sideTPCancelMock) CancelTakeProfitOrdersForSide(symbol, positionSide string) error {
	m.sideCalls = append(m.sideCalls, symbol+"|"+positionSide)
	return nil
}

func tpRunnerTrader(m Trader, side string) *AutoTrader {
	at := riskTestTrader(store.RiskControlConfig{TrailingStopEnabled: true})
	at.trader = m
	at.positionInitialStopLoss = map[string]float64{}
	at.positionExitMode = map[string]string{}
	at.SetRecordedStopLoss("XUSDT", side, map[string]float64{"long": 95, "short": 105}[side])
	at.recordOpenTakeProfit("XUSDT", side, map[string]float64{"long": 107, "short": 93}[side])
	return at
}

func TestTPRunnerConversionCancelsOnlyOwnSide(t *testing.T) {
	for _, side := range []string{"long", "short"} {
		t.Run(side, func(t *testing.T) {
			m := &sideTPCancelMock{}
			m.mark = 100
			at := tpRunnerTrader(m, side)
			mark, qty := 108.0, 1.0
			if side == "short" {
				mark, qty = 92.0, -1.0
			}
			pos := map[string]interface{}{"symbol": "XUSDT", "side": side, "positionAmt": qty, "entryPrice": 100.0}
			at.processTrailingForPosition(pos, mark, 1.0)
			if !at.tpRunnerDone("XUSDT_" + side) {
				t.Fatal("precondition: TP runner conversion should have fired")
			}
			want := "XUSDT|" + strings.ToUpper(side)
			if len(m.sideCalls) != 1 || m.sideCalls[0] != want {
				t.Fatalf("side-scoped cancel calls=%v, want [%s]", m.sideCalls, want)
			}
			if m.symbolWide != 0 {
				t.Fatalf("symbol-wide CancelTakeProfitOrders must not run (kills the opposite hedge leg's TP), calls=%d", m.symbolWide)
			}
		})
	}
}

// Adapters without the side-scoped primitive keep the legacy symbol-wide call.
func TestTPRunnerConversionLegacyFallback(t *testing.T) {
	m := &tpCancelMock{}
	m.mark = 100
	at := tpRunnerTrader(m, "long")
	pos := map[string]interface{}{"symbol": "XUSDT", "side": "long", "positionAmt": 1.0, "entryPrice": 100.0}
	at.processTrailingForPosition(pos, 108, 1.0)
	if m.symbolWide != 1 || len(m.sideCalls) != 0 {
		t.Fatalf("legacy adapter: symbolWide=%d sideCalls=%v, want 1 / none", m.symbolWide, m.sideCalls)
	}
}

// ── 5. daily-loss halt latches until the next UTC day (review A) ───────────

func dailyHaltTestTrader(t *testing.T, st *store.Store, id string) *AutoTrader {
	t.Helper()
	at := riskTestTrader(store.RiskControlConfig{DailyMaxLossPct: 10})
	at.store = st
	at.id = id
	at.exchangeID = "acct-latch" // the halt is keyed per exchange account
	return at
}

func withDailyClock(t *testing.T, now time.Time) {
	t.Helper()
	old := dailyRiskNow
	dailyRiskNow = func() time.Time { return now }
	t.Cleanup(func() { dailyRiskNow = old })
}

func TestDailyLossHaltLatchesUntilNextUTCDay(t *testing.T) {
	ResetDailyBaselineRegistryForTest()
	t.Cleanup(ResetDailyBaselineRegistryForTest)
	day1 := time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC)
	withDailyClock(t, day1)
	st := profitLockTestStore(t)
	rc := store.RiskControlConfig{DailyMaxLossPct: 10}
	at := dailyHaltTestTrader(t, st, "latch")

	at.anchorDailyBaseline(1000)
	if h := at.dailyLossHaltBlocks(rc, 950); h != "" {
		t.Fatalf("-5%% must not halt: %s", h)
	}
	breach := at.dailyLossHaltBlocks(rc, 890) // -11%
	if breach == "" {
		t.Fatal("-11% must halt")
	}
	// Equity recovers above the threshold the same day: still halted, same
	// recorded loss %.
	if h := at.dailyLossHaltBlocks(rc, 990); h != breach {
		t.Fatalf("halt must stay latched after recovery: got %q want %q", h, breach)
	}
	if !strings.Contains(breach, "11.00%") {
		t.Fatalf("latched reason must carry the loss%% at trigger time: %q", breach)
	}

	// Next UTC day: cleared and re-anchored.
	withDailyClock(t, day1.Add(24*time.Hour))
	at.anchorDailyBaseline(990)
	if h := at.dailyLossHaltBlocks(rc, 990); h != "" {
		t.Fatalf("next UTC day must clear the halt: %s", h)
	}
}

func TestDailyLossHaltLatchSurvivesNewInstance(t *testing.T) {
	ResetDailyBaselineRegistryForTest()
	t.Cleanup(ResetDailyBaselineRegistryForTest)
	day1 := time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC)
	withDailyClock(t, day1)
	st := profitLockTestStore(t)
	rc := store.RiskControlConfig{DailyMaxLossPct: 10}

	old := dailyHaltTestTrader(t, st, "latch-old")
	old.anchorDailyBaseline(1000)
	if old.dailyLossHaltBlocks(rc, 880) == "" {
		t.Fatal("precondition: -12% must halt")
	}
	// A strategy save / restart: brand-new instance, process registry wiped,
	// equity already recovered above the threshold — the durable latch holds.
	ResetDailyBaselineRegistryForTest()
	reloaded := dailyHaltTestTrader(t, st, "latch-new")
	reloaded.anchorDailyBaseline(995)
	h := reloaded.dailyLossHaltBlocks(rc, 995)
	if h == "" || !strings.Contains(h, "latched") {
		t.Fatalf("new instance must inherit the latch, got %q", h)
	}
	// And it too releases on the next UTC day.
	withDailyClock(t, day1.Add(24*time.Hour))
	reloaded.anchorDailyBaseline(995)
	if h := reloaded.dailyLossHaltBlocks(rc, 995); h != "" {
		t.Fatalf("next UTC day must clear the inherited latch: %s", h)
	}
}

func TestRiskStateLatchDayHalt(t *testing.T) {
	st := profitLockTestStore(t)
	rs := st.RiskState()
	if err := rs.LatchDayHalt("acct", "2026-10-16", time.Now()); err == nil {
		t.Fatal("latching a day without a baseline row must error (nothing stored)")
	}
	if _, err := rs.AnchorDayBaseline("acct", "2026-10-16", 1000); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := rs.DayHalt("acct", "2026-10-16"); found {
		t.Fatal("no halt yet")
	}
	first := time.Date(2026, 10, 16, 4, 0, 0, 0, time.UTC)
	if err := rs.LatchDayHalt("acct", "2026-10-16", first); err != nil {
		t.Fatal(err)
	}
	if err := rs.LatchDayHalt("acct", "2026-10-16", first.Add(time.Hour)); err != nil {
		t.Fatalf("second latch must be a no-op, got %v", err)
	}
	at, found, err := rs.DayHalt("acct", "2026-10-16")
	if err != nil || !found || !at.UTC().Equal(first) {
		t.Fatalf("DayHalt=%v found=%v err=%v, want first trigger %v", at, found, err, first)
	}
	if _, found, _ := rs.DayHalt("acct", "2026-10-17"); found {
		t.Fatal("latch must be per day")
	}
}

// ── 6. AI close never depends on full market data (review D) ───────────────

type noQuoteMock struct{ batch1RiskMock }

func (m *noQuoteMock) GetMarketPrice(string) (float64, error) { return 0, errors.New("quote down") }

func TestAICloseSurvivesMarketDataFailure(t *testing.T) {
	failing := func(string) (*market.Data, error) { return nil, errors.New("klines unavailable / stale") }
	t.Run("long_quote_ok_marketdata_down", func(t *testing.T) {
		at, m := newBatch1RiskTrader("long", 1, 105, store.RiskControlConfig{})
		at.marketDataFn = failing
		rec := &store.DecisionAction{}
		if err := at.executeCloseLongWithRecord(&kernel.Decision{Symbol: "XUSDT"}, rec); err != nil {
			t.Fatalf("close must not depend on market data: %v", err)
		}
		if m.closeCalls != 1 || rec.Price != 105 {
			t.Fatalf("closeCalls=%d recorded price=%g, want 1 / 105 (lightweight quote)", m.closeCalls, rec.Price)
		}
	})
	t.Run("short_quote_ok_marketdata_down", func(t *testing.T) {
		at, m := newBatch1RiskTrader("short", -1, 95, store.RiskControlConfig{})
		at.marketDataFn = failing
		rec := &store.DecisionAction{}
		if err := at.executeCloseShortWithRecord(&kernel.Decision{Symbol: "XUSDT"}, rec); err != nil {
			t.Fatalf("close must not depend on market data: %v", err)
		}
		if m.closeShorts != 1 || rec.Price != 95 {
			t.Fatalf("closeShorts=%d recorded price=%g, want 1 / 95", m.closeShorts, rec.Price)
		}
	})
	t.Run("quote_down_fallback_to_marketdata", func(t *testing.T) {
		m := &noQuoteMock{}
		m.mark, m.symbol, m.side, m.qty = 100, "XUSDT", "long", 1
		at := riskTestTrader(store.RiskControlConfig{})
		at.trader = m
		at.marketDataFn = func(string) (*market.Data, error) { return &market.Data{Symbol: "XUSDT", CurrentPrice: 101.5}, nil }
		rec := &store.DecisionAction{}
		if err := at.executeCloseLongWithRecord(&kernel.Decision{Symbol: "XUSDT"}, rec); err != nil || rec.Price != 101.5 {
			t.Fatalf("err=%v price=%g, want fallback 101.5", err, rec.Price)
		}
	})
	t.Run("both_down_still_closes_price_zero", func(t *testing.T) {
		m := &noQuoteMock{}
		m.mark, m.symbol, m.side, m.qty = 100, "XUSDT", "short", -1
		at := riskTestTrader(store.RiskControlConfig{})
		at.trader = m
		at.marketDataFn = failing
		rec := &store.DecisionAction{Price: 7}
		if err := at.executeCloseShortWithRecord(&kernel.Decision{Symbol: "XUSDT"}, rec); err != nil {
			t.Fatalf("close must proceed with no price at all: %v", err)
		}
		if m.closeShorts != 1 || rec.Price != 0 {
			t.Fatalf("closeShorts=%d price=%g, want 1 / 0", m.closeShorts, rec.Price)
		}
	})
}
