package stockengine

import (
	"math"
	"testing"
	"time"

	"nofx/store"
)

// regimeDates returns n consecutive calendar dates starting at start.
func regimeDates(start string, n int) []string {
	t0, _ := time.Parse("2006-01-02", start)
	out := make([]string, n)
	for i := range out {
		out[i] = t0.AddDate(0, 0, i).Format("2006-01-02")
	}
	return out
}

// altCloses builds closes whose daily returns alternate +x, −x, +x, … so the 21
// returns (11 up, 10 down) have mean x/21 and sample variance x²·22/21, i.e.
// σ = x·√(22/21)·√252·100 = x·100·√264 = x·1624.8077 percent annualized.
func altCloses(dates []string, x float64) []DatedClose {
	out := make([]DatedClose, len(dates))
	c := 100.0
	for i, d := range dates {
		if i > 0 {
			if i%2 == 1 {
				c *= 1 + x
			} else {
				c *= 1 - x
			}
		}
		out[i] = DatedClose{Date: d, Close: c}
	}
	return out
}

func flatCloses(dates []string) []DatedClose {
	out := make([]DatedClose, len(dates))
	for i, d := range dates {
		out[i] = DatedClose{Date: d, Close: 100}
	}
	return out
}

func regimeRes(sym string, mom float64, closes []DatedClose) ScanResult {
	return ScanResult{Symbol: sym, Momentum: MomentumAssessment{Valid: true, Mom12_1Pct: mom, AboveSMA200: true}, RecentCloses: closes}
}

func regimeCfg() *store.StockConfig {
	return &store.StockConfig{RegimeScaler: store.StockRegimeBasketVol}
}

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s: got %.8f want %.8f", name, got, want)
	}
}

const sigma1Pct = 16.248076809271922 // √264, σ in percent for x = 1%

func TestComputeRegimeDisabled(t *testing.T) {
	dates := regimeDates("2026-09-01", 22)
	report := &ScanReport{Results: []ScanResult{regimeRes("AAABUSDT", 50, altCloses(dates, 0.05))}}
	for _, cfg := range []*store.StockConfig{nil, {}, {RegimeScaler: store.StockRegimeOff}} {
		got := ComputeRegime(cfg, report)
		if got.Enabled || got.Scale != 1 || got.BasketVolPct != 0 || len(got.Basket) != 0 {
			t.Fatalf("disabled regime must be inert: %+v", got)
		}
	}
}

func TestComputeRegimeSigmaAndClamp(t *testing.T) {
	dates := regimeDates("2026-09-01", 22)
	mk := func(x float64) *ScanReport {
		return &ScanReport{Results: []ScanResult{regimeRes("AAABUSDT", 50, altCloses(dates, x))}}
	}
	// x = 4% → σ = 64.9923%; default target 40 → s = 40/64.9923 = 0.615457.
	got := ComputeRegime(regimeCfg(), mk(0.04))
	if !got.Enabled || got.Reason != "" || len(got.Basket) != 1 || got.Basket[0] != "AAABUSDT" {
		t.Fatalf("%+v", got)
	}
	approx(t, "sigma", got.BasketVolPct, 4*sigma1Pct, 1e-6)
	approx(t, "scale", got.Scale, 40/(4*sigma1Pct), 1e-9)
	approx(t, "scale hand value", got.Scale, 0.615457, 1e-5)
	if got.TargetVolPct != 40 || got.Floor != 0.25 {
		t.Fatalf("defaults %+v", got)
	}
	// Calm basket: σ = 16.25% < 40 → clamped to 1.
	got = ComputeRegime(regimeCfg(), mk(0.01))
	approx(t, "calm sigma", got.BasketVolPct, sigma1Pct, 1e-6)
	if got.Scale != 1 {
		t.Fatalf("calm scale %v", got.Scale)
	}
	// Violent basket: x = 10% → σ = 162.48%, 40/162.48 = 0.2462 < floor 0.25.
	if got = ComputeRegime(regimeCfg(), mk(0.10)); got.Scale != 0.25 {
		t.Fatalf("floor not applied: %+v", got)
	}
	// Custom target and floor.
	cfg := regimeCfg()
	cfg.RegimeTargetVolPct, cfg.RegimeFloor = 100, 0.7
	got = ComputeRegime(cfg, mk(0.10)) // 100/162.48 = 0.6155 < 0.7
	if got.Scale != 0.7 || got.TargetVolPct != 100 || got.Floor != 0.7 {
		t.Fatalf("custom floor: %+v", got)
	}
	cfg.RegimeFloor = 0.05
	approx(t, "custom target", ComputeRegime(cfg, mk(0.10)).Scale, 100/(10*sigma1Pct), 1e-9)
}

func TestComputeRegimeEqualWeightAndCalendar(t *testing.T) {
	dates := regimeDates("2026-09-01", 22)
	shifted := regimeDates("2026-09-02", 22) // one day off: shares only 21 of the 22 dates
	report := &ScanReport{Results: []ScanResult{
		regimeRes("AAABUSDT", 90, altCloses(dates, 0.08)),
		regimeRes("FLATBUSDT", 80, flatCloses(dates)),
		regimeRes("SHIFTBUSDT", 70, altCloses(shifted, 0.30)), // misaligned → excluded
		regimeRes("SHORTBUSDT", 60, altCloses(dates[5:], 0.30)),
		{Symbol: "BELOWBUSDT", Momentum: MomentumAssessment{Valid: true, Mom12_1Pct: 99, AboveSMA200: false}, RecentCloses: altCloses(dates, 0.30)},
	}}
	got := ComputeRegime(regimeCfg(), report)
	// Equal-weight of the 8% alternator and a flat name: mean returns are half of
	// the alternator's, σ = 4·sigma1Pct.
	if len(got.Basket) != 2 || got.Basket[0] != "AAABUSDT" || got.Basket[1] != "FLATBUSDT" {
		t.Fatalf("basket %v", got.Basket)
	}
	approx(t, "equal weight sigma", got.BasketVolPct, 4*sigma1Pct, 1e-6)
	approx(t, "scale", got.Scale, 40/(4*sigma1Pct), 1e-9)

	// With SPY present its dates are the calendar: names on the shifted dates now
	// align with it, the others do not.
	report.Results = append(report.Results, ScanResult{Symbol: "SPYBUSDT", RecentCloses: flatCloses(shifted)})
	got = ComputeRegime(regimeCfg(), report)
	if len(got.Basket) != 1 || got.Basket[0] != "SHIFTBUSDT" {
		t.Fatalf("SPY calendar basket %v", got.Basket)
	}
	approx(t, "shifted sigma", got.BasketVolPct, 30*sigma1Pct, 1e-6)
	if got.Scale != 0.25 {
		t.Fatalf("scale %v", got.Scale)
	}

	// A SPY without a full window falls back to the first full basket name.
	report.Results[len(report.Results)-1].RecentCloses = flatCloses(shifted[:10])
	got = ComputeRegime(regimeCfg(), report)
	if len(got.Basket) != 2 || got.Basket[0] != "AAABUSDT" {
		t.Fatalf("fallback calendar basket %v", got.Basket)
	}
}

func TestComputeRegimeBasketSelection(t *testing.T) {
	dates := regimeDates("2026-09-01", 22)
	var results []ScanResult
	// 12 names with distinct momentum; only the top 10 form the basket and
	// scan-excluded names are skipped without being replaced by held names.
	syms := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"}
	for i, s := range syms {
		results = append(results, regimeRes(s+"BUSDT", float64(100-i), altCloses(dates, 0.01)))
	}
	cfg := regimeCfg()
	cfg.ScanExclude = []string{"ABUSDT"}
	got := ComputeRegime(cfg, &ScanReport{Results: results})
	if len(got.Basket) != 10 || got.Basket[0] != "BBUSDT" || got.Basket[9] != "KBUSDT" {
		t.Fatalf("basket %v", got.Basket)
	}
	approx(t, "sigma", got.BasketVolPct, sigma1Pct, 1e-6) // identical names: same σ as one
}

func TestComputeRegimeMissingDataIsScaleOne(t *testing.T) {
	dates := regimeDates("2026-09-01", 22)
	cases := map[string]*ScanReport{
		"nil report":      nil,
		"empty report":    {},
		"no momentum":     {Results: []ScanResult{{Symbol: "AAABUSDT", RecentCloses: altCloses(dates, 0.05)}}},
		"no closes":       {Results: []ScanResult{regimeRes("AAABUSDT", 10, nil)}},
		"short closes":    {Results: []ScanResult{regimeRes("AAABUSDT", 10, altCloses(dates[:21], 0.05))}},
		"flat (σ = 0)":    {Results: []ScanResult{regimeRes("AAABUSDT", 10, flatCloses(dates))}},
		"non-positive px": {Results: []ScanResult{regimeRes("AAABUSDT", 10, append(altCloses(dates[:21], 0.05), DatedClose{Date: dates[21], Close: 0}))}},
	}
	for name, report := range cases {
		got := ComputeRegime(regimeCfg(), report)
		if !got.Enabled || got.Scale != 1 || got.Reason == "" || got.BasketVolPct != 0 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
}

func TestComputeRegimeBreadth(t *testing.T) {
	dates := regimeDates("2026-09-01", 22)
	above := func(sym string) ScanResult { return regimeRes(sym, 10, altCloses(dates, 0.01)) }
	below := func(sym string) ScanResult {
		r := above(sym)
		r.Momentum.AboveSMA200 = false
		return r
	}
	invalid := ScanResult{Symbol: "NEWBUSDT"}
	cfg := regimeCfg()
	cfg.ScanExclude = []string{"XBUSDT"}
	report := &ScanReport{Results: []ScanResult{
		above("ABUSDT"), above("BBUSDT"), below("CBUSDT"), invalid,
		above("XBUSDT"),   // scan-excluded: not counted
		above("SPYBUSDT"), // benchmarks: not counted
		below("QQQBUSDT"),
	}}
	got := ComputeRegime(cfg, report)
	if got.Breadth != 2 || got.BreadthTotal != 3 {
		t.Fatalf("breadth %d/%d", got.Breadth, got.BreadthTotal)
	}
}

func TestRegimeTrimFraction(t *testing.T) {
	cases := []struct {
		scale, applied, frac float64
		ok                   bool
	}{
		{0.5, 1, 0.5, true},
		{0.8, 1, 0.2, true}, // exactly the band
		{0.9, 1, 0, false},
		{1, 1, 0, false},
		{0.3, 0.5, 0.4, true},
		{0.4, 0.5, 0, false},
		{0.9, 0.5, 0, false}, // scale rose: never trims
		{0.5, 0, 0, false},
	}
	for _, c := range cases {
		frac, ok := RegimeTrimFraction(c.scale, c.applied)
		if ok != c.ok || math.Abs(frac-c.frac) > 1e-9 {
			t.Fatalf("scale %v applied %v: got %v %v want %v %v", c.scale, c.applied, frac, ok, c.frac, c.ok)
		}
	}
}
