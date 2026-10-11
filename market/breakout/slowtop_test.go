package breakout

import (
	"testing"
	"time"
)

func TestNear90dHigh(t *testing.T) {
	mk := func(high, last float64, n int) []Kline {
		k := make([]Kline, n)
		for i := range k {
			k[i] = Kline{High: high, Close: last}
		}
		return k
	}
	// livePrice 0: no ticker price, so the last closed close is measured.
	if !near90dHigh(mk(100, 96.5, 90), 0, 5.0) {
		t.Fatal("3.5% below the 90d high must qualify")
	}
	if near90dHigh(mk(100, 94.0, 90), 0, 5.0) {
		t.Fatal("6% below the high must not qualify")
	}
	if near90dHigh(mk(100, 96.5, 5), 0, 5.0) {
		t.Fatal("too-short series must not qualify (fail-closed on data)")
	}
	if near90dHigh(nil, 0, 5.0) {
		t.Fatal("nil series must not qualify")
	}
}

// The 90d high comes from closed bars, but the distance is measured from the
// live price: yesterday's close can sit near the high while the coin has
// already dumped (or rallied) today.
func TestSlowTopNear90dHighUsesLivePrice(t *testing.T) {
	mk := func(high, last float64, n int) []Kline {
		k := make([]Kline, n)
		for i := range k {
			k[i] = Kline{High: high, Close: last}
		}
		return k
	}
	// (a) closed bar 1% under the high, live price 10% under it.
	if near90dHigh(mk(100, 99, 90), 90, 5.0) {
		t.Fatal("live 10% below the 90d high must not qualify on a near-high close")
	}
	// (b) closed bar 10% under the high, live price 3.5% under it.
	if !near90dHigh(mk(100, 90, 90), 96.5, 5.0) {
		t.Fatal("live 3.5% below the 90d high must qualify")
	}
	// (c) a non-positive live price falls back to the last closed close.
	for _, live := range []float64{0, -1} {
		if !near90dHigh(mk(100, 96.5, 90), live, 5.0) {
			t.Fatalf("livePrice %v must fall back to the closed close (96.5 qualifies)", live)
		}
		if near90dHigh(mk(100, 94.0, 90), live, 5.0) {
			t.Fatalf("livePrice %v must fall back to the closed close (94.0 must not qualify)", live)
		}
	}
	// (d) a live price above the closed-bar high is at the high: distance 0.
	if !near90dHigh(mk(100, 90, 90), 105, 5.0) {
		t.Fatal("live price above the 90d high must qualify")
	}
	// Short series still fail closed whatever the live price.
	if near90dHigh(mk(100, 96.5, 5), 96.5, 5.0) {
		t.Fatal("too-short series must not qualify even with a live price")
	}
}

// The scheduler ticks every 30 minutes and slowTopAt is stamped when a run
// finishes, so the refresh guard must sit below the tick or every other run is
// skipped (review 2026-10-11). The snapshot max-age base stays at 30m.
func TestSlowTopRefreshGuardBelowTick(t *testing.T) {
	const schedulerTick = 30 * time.Minute
	if slowTopRefreshTTL >= schedulerTick {
		t.Fatalf("refresh guard %v must be shorter than the %v scheduler tick", slowTopRefreshTTL, schedulerTick)
	}
	if slowTopCacheTTL != 30*time.Minute {
		t.Fatalf("snapshot max-age base drifted to %v (2×TTL drop rule must stay at 60m)", slowTopCacheTTL)
	}
}

func TestShortWeightsDefaultsAndNormalization(t *testing.T) {
	w := shortWeights()
	if len(w) != 9 {
		t.Fatalf("want 9 component weights, got %d", len(w))
	}
	total := 0.0
	for _, k := range shortWeightKeys {
		total += w[k]
	}
	if total < 0.9999 || total > 1.0001 {
		t.Fatalf("weights must sum to 1, got %.4f", total)
	}
	// Audit 2026-09-12: overbought yields to structure (RSI-pinned-high is a
	// trend feature, not a reversal signal) — structure is now the 0.15 pole.
	if w["overbought"] != 0.10 || w["structure"] != 0.15 || w["crowding"] != 0.15 {
		t.Fatalf("designed defaults drifted: %v", w)
	}
}

func TestUpdateShortWeights(t *testing.T) {
	base := DefaultShortWeights()
	// Forty alternate-day blocks, each with ten distinct symbols. Structure
	// predicts better shorts within each day; overbought predicts worse shorts.
	samples := reviewRankICSamples(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 40, 10)
	out, ok := updateShortWeights(samples, base, shortTunerEta)
	if !ok {
		t.Fatal("strong correlations must pass the significance gate")
	}
	if out["structure"] <= base["structure"] {
		t.Fatalf("predictive component must gain weight: %.3f → %.3f", base["structure"], out["structure"])
	}
	if out["overbought"] >= base["overbought"] {
		t.Fatalf("counter-predictive component must lose weight: %.3f → %.3f", base["overbought"], out["overbought"])
	}
	// Unseen component keeps its base share (corr 0 → exp(0) = 1 before norm).
	if out["stretch"] < 0.02 {
		t.Fatalf("zero-correlation weight collapsed: %.3f", out["stretch"])
	}
	// Renormalized.
	total := 0.0
	for _, k := range shortWeightKeys {
		total += out[k]
		if out[k] < 0.03-1e-9 || out[k] > 0.30+1e-9 {
			t.Fatalf("weight %s = %.3f outside clamp [0.03, 0.30]", k, out[k])
		}
	}
	if total < 0.999 || total > 1.001 {
		t.Fatalf("weights must renormalize to 1, got %.4f", total)
	}
}
