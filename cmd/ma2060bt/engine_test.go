package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"nofx/market/usstock"
)

func synthetic(n int, price func(int) float64) []usstock.Bar {
	bars := make([]usstock.Bar, n)
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range bars {
		c := price(i)
		bars[i] = usstock.Bar{OpenTime: start.AddDate(0, 0, i), Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 100}
	}
	return bars
}

func replace(bars []usstock.Bar, i int, open, high, low, close float64) {
	bars[i].Open, bars[i].High, bars[i].Low, bars[i].Close = open, high, low, close
}

func near(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-8*math.Max(1, math.Abs(want)) {
		t.Fatalf("%s: got %.12f, want %.12f", label, got, want)
	}
}

func rising(n int) []usstock.Bar {
	return synthetic(n, func(i int) float64 { return 100 + 0.2*float64(i) })
}

func TestConstructedCrossesAndATR(t *testing.T) {
	b := synthetic(130, func(i int) float64 {
		if i >= 100 {
			return 80
		}
		if i >= 80 {
			return 120
		}
		return 100
	})
	v := indicators(b)
	near(t, "flat SMA20", v[79].Fast, 100)
	near(t, "flat SMA60", v[79].Slow, 100)
	near(t, "ATR seed", v[13].ATR, 2)
	near(t, "Wilder ATR", v[80].ATR, (2.0*13+21)/14)
	if !golden(v, 80) || golden(v, 79) || golden(v, 81) {
		t.Fatal("golden cross must occur only at first strict crossing")
	}
	deaths := 0
	for i := range v {
		if death(v, i) {
			deaths++
			if i <= 100 || i >= 120 {
				t.Fatalf("unexpected death cross index %d", i)
			}
		}
	}
	if deaths != 1 {
		t.Fatalf("got %d death crosses, want one", deaths)
	}
	if finite(v[12].ATR) || finite(v[18].Fast) || finite(v[58].Slow) {
		t.Fatal("indicators must remain unavailable during warmup")
	}
}

func TestAllEntryRulesOnOHLCScenarios(t *testing.T) {
	e1 := synthetic(81, func(i int) float64 {
		if i == 80 {
			return 110
		}
		return 100
	})
	e2 := rising(90)
	i := len(e2) - 1
	a := indicators(e2)[i]
	replace(e2, i, e2[i].Close-0.5, e2[i].Close+1, a.Fast*1.001, e2[i].Close)
	e3 := rising(90)
	replace(e3, 88, 115, 115.5, 113.5, 114)
	// Bearish today's body avoids E2, while price has recovered SMA20.
	replace(e3, 89, 118, 119, 117, 117.8)
	e4 := synthetic(82, func(int) float64 { return 100 })
	replace(e4, 80, 76, 77, 74, 75)
	replace(e4, 81, 78, 81, 77, 80)
	for _, tc := range []struct {
		entry string
		bars  []usstock.Bar
	}{{"E1", e1}, {"E2", e2}, {"E3", e3}, {"E4", e4}} {
		t.Run(tc.entry, func(t *testing.T) {
			v := indicators(tc.bars)
			if got := entryRule(tc.bars, v, len(v)-1); got != tc.entry {
				t.Fatalf("got %q, want %s; final indicators %+v", got, tc.entry, v[len(v)-1])
			}
		})
	}
	// An engulfing body can trigger E4 without breaking the previous wick high.
	replace(e4, 80, 76, 90, 74, 75)
	replace(e4, 81, 74.5, 81, 74, 80)
	if !bullishKey(e4, 81) || e4[81].Close > e4[80].High || entryRule(e4, indicators(e4), 81) != "E4" {
		t.Fatal("bullish engulfing alternative should trigger E4")
	}
}

func TestAllExitRulesOnOHLCScenarios(t *testing.T) {
	x1 := synthetic(81, func(i int) float64 {
		if i == 80 {
			return 90
		}
		return 100
	})
	x2 := synthetic(81, func(int) float64 { return 100 })
	replace(x2, 70, 120, 121, 119, 120)
	replace(x2, 80, 96, 97, 94, 95)
	x3 := synthetic(90, func(i int) float64 { return 200 - 0.5*float64(i) })
	x4 := rising(90)
	for _, tc := range []struct {
		exit       string
		bars       []usstock.Bar
		price, atr float64
	}{{"X1", x1, 100, 10}, {"X2", x2, 100, 10}, {"X3", x3, 100, 10}, {"X4", x4, 130, 2}} {
		t.Run(tc.exit, func(t *testing.T) {
			v := indicators(tc.bars)
			if got := exitRule(tc.bars, v, len(v)-1, tc.price, tc.atr, false); got != tc.exit {
				t.Fatalf("got %q, want %s", got, tc.exit)
			}
		})
	}
	target := synthetic(81, func(int) float64 { return 100 })
	if got := exitRule(target, indicators(target), 80, 75, 5, true); got != "TARGET" {
		t.Fatalf("E4 target: got %q", got)
	}
	if got := exitRule(target, indicators(target), 80, 75, 5, false); got != "" {
		t.Fatalf("ordinary position should not have a target: %q", got)
	}
	if got := exitRule(x1, indicators(x1), 80, 100, 1, true); got != "X1" {
		t.Fatalf("X1 must take priority over stop and target, got %s", got)
	}
}

func fixture() (map[string][]usstock.Bar, []usstock.Bar) {
	spy := rising(360)
	stock := synthetic(360, func(i int) float64 {
		if i >= 300 {
			return 80
		}
		if i >= 270 {
			return 120
		}
		return 100
	})
	replace(stock, 271, 137, 138, 119, 120)
	return map[string][]usstock.Bar{"AAABUSDT": stock}, spy
}

func strategy(t *testing.T, r *Backtest, name string) StrategyResult {
	t.Helper()
	for _, s := range r.Strategies {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("missing strategy %s", name)
	return StrategyResult{}
}

func runFixture(t *testing.T, input map[string][]usstock.Bar, spy []usstock.Bar, c Config) *Backtest {
	t.Helper()
	r, err := runBacktest(input, input, spy, spy, c)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestNoLookaheadAndNextOpen(t *testing.T) {
	input, spy := fixture()
	c := Config{usstock.TF1d, 1, 10, 0}
	r := runFixture(t, input, spy, c)
	ma := strategy(t, r, "MA_CROSS")
	if len(ma.Trades) != 1 {
		t.Fatalf("got %d closed trades, want one", len(ma.Trades))
	}
	trade := ma.Trades[0]
	if !trade.EntryTime.Equal(spy[271].OpenTime) {
		t.Fatalf("entry time %s must be t+1, %s", trade.EntryTime, spy[271].OpenTime)
	}
	near(t, "next OPEN fill", trade.EntryPrice, 137)
	for _, pt := range ma.Curve {
		if pt.Time.Equal(spy[270].OpenTime) {
			near(t, "no same-close purchase", pt.Equity, initialEquity)
		}
		if pt.Time.Equal(spy[271].OpenTime) {
			near(t, "entry-day mark", pt.Equity, initialEquity/137*120)
		}
	}
	cutoff := spy[270].OpenTime
	changed := map[string][]usstock.Bar{"AAABUSDT": append([]usstock.Bar(nil), input["AAABUSDT"]...)}
	changedSpy := append([]usstock.Bar(nil), spy...)
	for i := 271; i < len(spy); i++ {
		for _, b := range []*usstock.Bar{&changed["AAABUSDT"][i], &changedSpy[i]} {
			b.Open *= 3
			b.High *= 3
			b.Low *= 3
			b.Close *= 3
		}
	}
	r2 := runFixture(t, changed, changedSpy, c)
	// Check every strategy, including scheduled momentum decisions, not just MA.
	for _, s := range r.Strategies {
		s2 := strategy(t, r2, s.Name)
		var before1, before2 []Decision
		for _, d := range s.Decisions {
			if !d.Time.After(cutoff) {
				before1 = append(before1, d)
			}
		}
		for _, d := range s2.Decisions {
			if !d.Time.After(cutoff) {
				before2 = append(before2, d)
			}
		}
		if !reflect.DeepEqual(before1, before2) {
			t.Fatalf("future data changed %s decision at/before t", s.Name)
		}
		for i, pt := range s.Curve {
			if !pt.Time.After(cutoff) && pt != s2.Curve[i] {
				t.Fatalf("future data changed %s equity at %s", s.Name, pt.Time)
			}
		}
	}
}

func TestSlotLimitAndMomentumPriority(t *testing.T) {
	input, spy := fixture()
	b := append([]usstock.Bar(nil), input["AAABUSDT"]...)
	for i := 0; i < 50; i++ {
		replace(b, i, 50, 51, 49, 50)
	}
	input["BBBBUSDT"] = b                                                      // Higher 12-1 momentum than AAA at the same cross.
	input["000BUSDT"] = append([]usstock.Bar(nil), input["AAABUSDT"][200:]...) // No valid daily rank.
	for _, slots := range []int{1, 2} {
		r := runFixture(t, input, spy, Config{usstock.TF1d, slots, 10, 0})
		s := strategy(t, r, "MA_CROSS")
		if len(s.Trades) != slots {
			t.Fatalf("slots=%d, trades=%d", slots, len(s.Trades))
		}
		selected := make(map[string]bool)
		for _, tr := range s.Trades {
			selected[tr.Symbol] = true
		}
		if !selected["BBBBUSDT"] || selected["000BUSDT"] || (slots == 2 && !selected["AAABUSDT"]) {
			t.Fatalf("incorrect priority: %v", selected)
		}
		for _, point := range s.Curve {
			if point.Positions > slots || point.Cash < -1e-7 || point.Exposure > 1+1e-10 {
				t.Fatalf("invalid portfolio: %+v", point)
			}
		}
	}
}

func TestPriorityChosenBeforeNextOpenAvailability(t *testing.T) {
	input, spy := fixture()
	b := append([]usstock.Bar(nil), input["AAABUSDT"]...)
	for i := 0; i < 50; i++ {
		replace(b, i, 50, 51, 49, 50)
	}
	// Stronger BBB signals at 270 but is missing the intended 271 fill bar.
	input["BBBBUSDT"] = append(b[:271:271], b[272:]...)
	r := runFixture(t, input, spy, Config{usstock.TF1d, 1, 10, 0})
	s := strategy(t, r, "MA_CROSS")
	for _, d := range s.Decisions {
		if d.Time.Equal(spy[270].OpenTime) {
			if len(d.Orders) != 1 || d.Orders[0].Symbol != "BBBBUSDT" {
				t.Fatalf("entry slots must be allocated before observing next open: %+v", d)
			}
		}
	}
	for _, pt := range s.Curve {
		if pt.Time.Equal(spy[271].OpenTime) && pt.Positions != 0 {
			t.Fatal("missing priority entry must not promote a lower-ranked signal")
		}
	}
	if s.Unfilled != 1 {
		t.Fatalf("want one missing-open order, got %d", s.Unfilled)
	}
}

func TestBounceEntryUsesSignalATRAndTargetNextOpen(t *testing.T) {
	spy := rising(280)
	b := synthetic(280, func(int) float64 { return 100 })
	replace(b, 270, 76, 77, 74, 75)
	replace(b, 271, 78, 81, 77, 80)
	replace(b, 272, 82, 101, 81, 100)
	replace(b, 273, 99, 101, 98, 100)
	r := runFixture(t, map[string][]usstock.Bar{"A": b}, spy, Config{usstock.TF1d, 1, 10, 10})
	s := strategy(t, r, "MA2060_RULES")
	if len(s.Trades) != 1 || s.Trades[0].EntryType != "E4" || s.Trades[0].ExitType != "TARGET" {
		t.Fatalf("expected one E4 bounce to target, got %+v", s.Trades)
	}
	near(t, "bounce entry open", s.Trades[0].EntryPrice, 82)
	near(t, "target next open", s.Trades[0].ExitPrice, 99)
	if s.Trades[0].EntryTime != b[272].OpenTime || s.Trades[0].ExitTime != b[273].OpenTime {
		t.Fatal("bounce entry and exit must both wait for next open")
	}
	for _, d := range s.Decisions {
		if d.Time == b[271].OpenTime {
			near(t, "entry stop ATR", d.Orders[0].ATR, indicators(b[:272])[271].ATR)
		}
	}
	if s.Entries["E4"].Trades != 1 || s.Exits["TARGET"] != 1 {
		t.Fatal("bounce must appear in entry and exit breakdowns")
	}
}

func TestMomentumTimingExitAndEWFixedUniverse(t *testing.T) {
	input, spy := fixture()
	b := append([]usstock.Bar(nil), input["AAABUSDT"]...)
	input["BBBBUSDT"] = b
	input["IPOBUSDT"] = append([]usstock.Bar(nil), b[260:]...)
	r := runFixture(t, input, spy, Config{usstock.TF1d, 1, 20, 10})
	ew := strategy(t, r, "EW_BH")
	if ew.Curve[1].Positions != 2 {
		t.Fatal("EW must hold all names present at start, independently of maxpos")
	}
	for _, d := range ew.Decisions {
		if len(d.Targets) != 2 {
			t.Fatal("EW must not add names first observed after the initial date")
		}
		for _, target := range d.Targets {
			if target.Symbol == "IPOBUSDT" {
				t.Fatal("EW must freeze its initial universe")
			}
		}
	}
	// A death cross between rebalances closes MOM_X_MA; a new golden cross
	// alone cannot spend that cash before the next scheduled selection.
	ma := strategy(t, r, "MOM_X_MA")
	found := false
	for _, d := range ma.Decisions {
		for _, order := range d.Orders {
			if order.Exit == "X1" && !d.Rebalance {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("expected between-rebalance death-cross exit")
	}
	stocks := map[string]history{"A": makeHistory(b, usstock.TF1d)}
	daily := map[string]history{"A": makeHistory(spy, usstock.TF1d)}
	p := &portfolio{Cash: 1000, Positions: make(map[string]*position), Exits: make(map[string]int)}
	d := decide("MOM_X_MA", b[270].OpenTime, stocks, daily, p, Config{usstock.TF1d, 1, 10, 0}, false, false, nil)
	if len(d.Orders) != 0 || d.Rebalance || len(d.Targets) != 0 {
		t.Fatal("between-rebalance golden cross must not reopen MOM_X_MA")
	}
}

func TestOneTradeCostsHandComputed(t *testing.T) {
	p := &portfolio{Cash: 1000, Positions: make(map[string]*position), Exits: make(map[string]int)}
	c := Config{usstock.TF1d, 1, 10, 100} // 1% per side.
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p.fill(Decision{Orders: []Order{{Symbol: "A", Entry: "E1", ATR: 2}}}, map[string]usstock.Bar{"A": {Open: 100, Close: 100}}, at, c, 1)
	near(t, "entry costs", p.Costs, 1000.0/101)
	near(t, "cash after buy", p.Cash, 0)
	near(t, "entry shares", p.Positions["A"].Quantity, 1000.0/101)
	p.fill(Decision{Orders: []Order{{Symbol: "A", Exit: "X1"}}}, map[string]usstock.Bar{"A": {Open: 110, Close: 110}}, at.AddDate(0, 0, 5), c, 1)
	// Spend 1000 / 1.01 on shares; sell at +10%, retain 99% of proceeds.
	want := 1000 / 1.01 * 1.1 * 0.99
	near(t, "final equity", p.equity(), want)
	near(t, "total costs", p.Costs, 1000/1.01*0.01*(1+1.1))
	if len(p.Trades) != 1 || len(p.Positions) != 0 || p.Exits["X1"] != 1 {
		t.Fatalf("incorrect trade accounting: %+v", p)
	}
	near(t, "net P&L", p.Trades[0].PnL, want-1000)
	near(t, "net trade return", p.Trades[0].Return, want/1000-1)
	near(t, "holding days", p.Trades[0].Holding, 5)
}

func TestFullRebalanceWeightsAndFIFO(t *testing.T) {
	p := &portfolio{Cash: 1000, Positions: make(map[string]*position), Exits: make(map[string]int)}
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	c := Config{usstock.TF1d, 2, 10, 100}
	d := Decision{Rebalance: true, Targets: []Target{{"A", 0.5}, {"B", 0.5}}}
	p.fill(d, map[string]usstock.Bar{"A": {Open: 100}, "B": {Open: 100}}, at, c, 2)
	near(t, "post-cost equity", p.equity(), 1000/1.01)
	near(t, "initial equal weight", p.Positions["A"].Quantity*100/p.equity(), 0.5)
	p.fill(d, map[string]usstock.Bar{"A": {Open: 200}, "B": {Open: 100}}, at.AddDate(0, 0, 10), c, 2)
	near(t, "retained A reweighted", p.Positions["A"].Quantity*200/p.equity(), 0.5)
	near(t, "retained B reweighted", p.Positions["B"].Quantity*100/p.equity(), 0.5)
	if len(p.Trades) != 1 || len(p.Positions["B"].Lots) != 2 {
		t.Fatal("rebalance must trim A and add a separate B lot")
	}
	// Empty target means sell everything, including both B acquisition lots.
	p.fill(Decision{Rebalance: true}, map[string]usstock.Bar{"A": {Open: 200}, "B": {Open: 100}}, at.AddDate(0, 0, 20), c, 2)
	if len(p.Positions) != 0 || len(p.Trades) != 4 {
		t.Fatalf("FIFO closure mismatch: %d positions, %d lots", len(p.Positions), len(p.Trades))
	}
	pnl, costs := 0.0, 0.0
	for _, tr := range p.Trades {
		pnl += tr.PnL
		costs += tr.Costs
	}
	near(t, "cash reconciles realized P&L", p.Cash, 1000+pnl)
	near(t, "all costs allocated", costs, p.Costs)
}

func TestMissingBarsHoldAndCannotTrade(t *testing.T) {
	p := &portfolio{Cash: 1000, Positions: make(map[string]*position), Exits: make(map[string]int)}
	at := time.Now().UTC()
	c := Config{usstock.TF1d, 1, 10, 0}
	p.buy("A", 1000, 100, 0, at, "E1", 2)
	p.mark(map[string]usstock.Bar{}, false)
	near(t, "missing mark holds last price", p.equity(), 1000)
	p.fill(Decision{Orders: []Order{{Symbol: "A", Exit: "X1"}}}, map[string]usstock.Bar{}, at.AddDate(0, 0, 1), c, 1)
	if p.Positions["A"] == nil || p.Unfilled != 1 {
		t.Fatal("missing bar cannot fill an exit")
	}
	p.fill(Decision{Rebalance: true, Targets: []Target{{"B", 1}}}, map[string]usstock.Bar{"B": {Open: 50}}, at.AddDate(0, 0, 2), c, 1)
	if p.Positions["A"] == nil || p.Positions["B"] != nil || len(p.Positions) != 1 {
		t.Fatal("missing dropout must not allow extra position or leverage")
	}
	// An entry missing its intended next open expires, with no delayed fill.
	p2 := &portfolio{Cash: 1000, Positions: make(map[string]*position), Exits: make(map[string]int)}
	p2.fill(Decision{Orders: []Order{{Symbol: "A", Entry: "E1"}}}, nil, at, c, 1)
	p2.fill(Decision{}, map[string]usstock.Bar{"A": {Open: 100}}, at.AddDate(0, 0, 1), c, 1)
	if len(p2.Positions) != 0 {
		t.Fatal("missing entry must not be deferred")
	}
}

func TestMomentumFilterAndTimingNoBackfill(t *testing.T) {
	b := rising(300)
	daily := map[string]history{"A": makeHistory(b, usstock.TF1d), "B": makeHistory(b, usstock.TF1d)}
	// Make A the stronger rank while its execution MA trend is bearish.
	dailyA := append([]usstock.Bar(nil), b...)
	for i := 0; i < 50; i++ {
		replace(dailyA, i, 50, 51, 49, 50)
	}
	daily["A"] = makeHistory(dailyA, usstock.TF1d)
	down := synthetic(300, func(i int) float64 { return 300 - 0.2*float64(i) })
	stocks := map[string]history{"A": makeHistory(down, usstock.TF1d), "B": makeHistory(b, usstock.TF1d)}
	p := &portfolio{Cash: 1000, Positions: make(map[string]*position), Exits: make(map[string]int)}
	c := Config{usstock.TF1d, 1, 10, 0}
	key := b[299].OpenTime
	d := decide("MOM_TOP", key, stocks, daily, p, c, true, false, nil)
	if len(d.Targets) != 1 || d.Targets[0].Symbol != "A" {
		t.Fatalf("wrong momentum basket: %+v", d)
	}
	d = decide("MOM_X_MA", key, stocks, daily, p, c, true, false, nil)
	if !d.Rebalance || len(d.Targets) != 0 {
		t.Fatal("timing veto must leave cash, not promote B")
	}
	// Invalid or below-SMA200 names never enter the momentum basket.
	daily["A"], daily["B"] = makeHistory(down, usstock.TF1d), makeHistory(b[:200], usstock.TF1d)
	d = decide("MOM_TOP", key, stocks, daily, p, c, true, false, nil)
	if len(d.Targets) != 0 {
		t.Fatal("momentum requires valid rank and close above SMA200")
	}
}

func TestHourlyUsesPriorDailyCloseAndDailySchedule(t *testing.T) {
	d := rising(340)
	key := d[299].OpenTime.Add(15 * time.Hour)
	original := rankAt("A", makeHistory(d, usstock.TF1d), key, usstock.TF1h)
	near(t, "prior daily 12-1 momentum", original.Momentum, d[277].Close/d[46].Close-1)
	changed := append([]usstock.Bar(nil), d...)
	for i := 299; i < len(changed); i++ {
		replace(changed, i, 999, 1000, 998, 999)
	}
	if got := rankAt("A", makeHistory(changed, usstock.TF1d), key, usstock.TF1h); got != original {
		t.Fatal("hourly rank leaked today's or future daily close")
	}
	var hours []usstock.Bar
	for day := 260; day < 320; day++ {
		for hour := 0; hour < 3; hour++ {
			bar := d[day]
			bar.OpenTime = bar.OpenTime.Add(time.Duration(14+hour) * time.Hour)
			hours = append(hours, bar)
		}
	}
	r, err := runBacktest(map[string][]usstock.Bar{"A": hours}, map[string][]usstock.Bar{"A": d}, hours, d, Config{usstock.TF1h, 1, 2, 0})
	if err != nil {
		t.Fatal(err)
	}
	decisions := strategy(t, r, "MOM_TOP").Decisions
	if len(decisions) < 5 {
		t.Fatal("fixture must include multiple rebalances")
	}
	for i, decision := range decisions {
		if decision.Time.Hour() != 16 || (i > 0 && decision.Time.Sub(decisions[i-1].Time) != 48*time.Hour) {
			t.Fatalf("hourly momentum must rebalance at last bar every two days: %+v", decision)
		}
	}
	// EW's explicit 63-bar schedule differs from momentum's day schedule.
	ew := strategy(t, r, "EW_BH")
	if len(ew.Decisions) != 2 || ew.Decisions[1].Time.Sub(ew.Decisions[0].Time) != 21*24*time.Hour {
		t.Fatalf("EW must rebalance every 63 execution bars: %+v", ew.Decisions)
	}
	if len(makeHistory(hours, usstock.TF1h).bars) != len(hours) {
		t.Fatal("hourly normalization must not collapse bars onto daily dates")
	}
}

func TestKnownEquityMetrics(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	// Exactly 365.25 days: CAGR is 10%; peak 120 -> trough 90 is 25% DD.
	curve := []EquityPoint{{Time: start, Equity: 100}, {Time: start.Add(24 * time.Hour), Equity: 120, Exposure: 0.5}, {Time: start.Add(48 * time.Hour), Equity: 90, Exposure: 1}, {Time: start.Add(365*24*time.Hour + 6*time.Hour), Equity: 110, Exposure: 0.75}}
	trades := []Trade{{EntryType: "E1", PnL: 20, Return: 0.2, Holding: 10}, {EntryType: "E4", PnL: -10, Return: -0.1, Holding: 2}}
	m := computeMetrics(curve, trades, 3, usstock.TF1d)
	if m.CAGR == nil || m.Calmar == nil || m.WinRate == nil || m.ProfitFactor == nil {
		t.Fatal("expected defined metrics")
	}
	near(t, "CAGR", *m.CAGR, 0.1)
	near(t, "drawdown", m.MaxDrawdown, 0.25)
	near(t, "Calmar", *m.Calmar, 0.4)
	if m.Peak != curve[1].Time || m.Trough != curve[2].Time {
		t.Fatal("wrong peak/trough dates")
	}
	near(t, "exposure", m.Exposure, 0.75)
	near(t, "win rate", *m.WinRate, 0.5)
	near(t, "profit factor", *m.ProfitFactor, 2)
	near(t, "holding", *m.AvgHolding, 6)
	near(t, "2020 return", m.Years["2020"], 0.1)
	if m.Costs != 3 || m.Trades != 2 {
		t.Fatal("wrong cost/trade totals")
	}
	b := entryBreakdown(trades)
	near(t, "E1 win rate", *b["E1"].WinRate, 1)
	near(t, "E4 return", *b["E4"].AvgReturn, -0.1)
	if b["E2"].Trades != 0 || b["E2"].WinRate != nil {
		t.Fatal("empty breakdown should have unavailable ratios")
	}
}

func TestYearReturnsAndLast12Months(t *testing.T) {
	parse := func(s string) time.Time {
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	curve := []EquityPoint{{Time: parse("2022-12-30"), Equity: 100}, {Time: parse("2023-12-29"), Equity: 120}, {Time: parse("2024-06-28"), Equity: 90}, {Time: parse("2024-12-30"), Equity: 132}}
	m := computeMetrics(curve, nil, 0, usstock.TF1d)
	near(t, "2023", m.Years["2023"], 0.2)
	near(t, "2024", m.Years["2024"], 0.1)
	near(t, "12m", m.Last12Months, 0.1)
	if m.Last12Partial {
		t.Fatal("12m should have a complete baseline")
	}
	flat := computeMetrics([]EquityPoint{{Time: curve[0].Time, Equity: 100}, {Time: curve[1].Time, Equity: 100}}, nil, 0, usstock.TF1d)
	if flat.Sharpe != nil || flat.Sortino != nil || flat.Calmar != nil || flat.WinRate != nil {
		t.Fatal("undefined ratios must be null, not infinity or fabricated zero")
	}
}

func TestCacheCompatibilityHourlyAndShortHistory(t *testing.T) {
	dir := t.TempDir()
	bars := rising(300)
	s := &usstock.Series{Symbol: "AAPLBUSDT", Timeframe: usstock.TF1d, Source: usstock.SourceYahoo, Bars: bars}
	data, err := json.Marshal(cachedSeries{RequestedBars: 1300, Series: s})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "AAPLBUSDT.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(path, now.Add(-48*time.Hour), now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	noFetch := func(context.Context, string, string, int, bool) (*usstock.Series, error) {
		t.Fatal("valid cache must not make any network call")
		return nil, nil
	}
	got, cached, warning, err := loadSeries(context.Background(), s.Symbol, usstock.TF1d, 1300, dir, now, noFetch)
	if err != nil || !cached || len(got.Bars) != 300 || !strings.Contains(warning, "older than 24h") {
		t.Fatalf("daily cache: cached=%t, warning=%s, err=%v", cached, warning, err)
	}
	hour := *s
	hour.Timeframe, hour.Bars = usstock.TF1h, bars[:80]
	calls := 0
	fetch := func(_ context.Context, sym, tf string, need int, yahoo bool) (*usstock.Series, error) {
		calls++
		if sym != s.Symbol || tf != usstock.TF1h || need != 3000 || !yahoo {
			t.Fatal("wrong hourly GetSeries arguments")
		}
		return &hour, usstock.ErrInsufficientBars
	}
	got, cached, warning, err = loadSeries(context.Background(), s.Symbol, usstock.TF1h, 3000, dir, now, fetch)
	if err != nil || cached || len(got.Bars) != 80 || warning == "" || calls != 1 {
		t.Fatalf("short hourly fetch: cached=%t, warning=%s, err=%v", cached, warning, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "1h", "AAPLBUSDT.json")); err != nil {
		t.Fatal(err)
	}
	if _, cached, _, err := loadSeries(context.Background(), s.Symbol, usstock.TF1h, 3000, dir, now, noFetch); err != nil || !cached {
		t.Fatal("short hourly cache must be reusable without refetching")
	}
	// A request larger than the saved request can fall back to the old cache.
	failedFetch := func(context.Context, string, string, int, bool) (*usstock.Series, error) {
		return nil, errors.New("offline")
	}
	if _, cached, warning, err := loadSeries(context.Background(), s.Symbol, usstock.TF1d, 1400, dir, now, failedFetch); err != nil || !cached || !strings.Contains(warning, "offline") {
		t.Fatal("expected disclosed short-cache fallback")
	}
	registry, warning, err := loadUniverse(context.Background(), dir, func(context.Context) ([]usstock.SymbolInfo, error) {
		t.Fatal("cycleic cache discovery must work fully offline")
		return nil, nil
	})
	if err != nil || len(registry) != 1 || !strings.Contains(warning, "subset") {
		t.Fatalf("cached universe: %v %q %v", registry, warning, err)
	}
}

func TestUniverseNormalizationAndReportJSON(t *testing.T) {
	var registry []usstock.SymbolInfo
	for _, s := range []string{"AAPLBUSDT", "AAPLBUSDT", "MSFTBUSDT", "SPYBUSDT", "QQQBUSDT", "MUUBUSDT", "KORUBUSDT", "TQQQBUSDT"} {
		registry = append(registry, usstock.SymbolInfo{Symbol: s})
	}
	symbols, _ := selectUniverse(registry, defaultExclude)
	if !reflect.DeepEqual(symbols, []string{"AAPLBUSDT", "MSFTBUSDT"}) {
		t.Fatalf("unexpected universe: %v", symbols)
	}
	input, spy := fixture()
	r := runFixture(t, input, spy, Config{usstock.TF1d, 2, 10, 10})
	if _, err := json.Marshal(r); err != nil {
		t.Fatalf("all metrics/decisions must serialize without NaN/Inf: %v", err)
	}
	report := renderReport(r)
	for _, want := range []string{"Strategy comparison", "Calendar-year returns", "MA2060 rule breakdown", "Coverage", "Survivorship", "LLM", "730 days", "E1", "X4", "SPY_BH", "EW_BH"} {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q", want)
		}
	}
	// Mark-to-market includes the initial SPY buy fee but no end-of-test sale.
	s := strategy(t, r, "SPY_BH")
	want := initialEquity / (1 + 10.0/10000) / spy[253].Open * spy[len(spy)-1].Close
	near(t, "SPY buy and hold", s.Curve[len(s.Curve)-1].Equity, want)
	if s.Metrics.Trades != 0 || s.OpenPositions != 1 {
		t.Fatal("open benchmark position must not be counted as a realized trade")
	}
}
