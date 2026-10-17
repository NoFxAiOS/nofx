package stockengine

import (
	"fmt"
	"math"

	"nofx/store"
)

// Regime position scaler "basket_vol" (backtest: docs/research/2026-10-17-
// regime-overlay-backtrader-1d.md, scripts/backtest/stocks_regime.py
// BASKET_VOL): s = clip(target_vol / σ_basket, floor, 1), where σ_basket is the
// annualized stdev of the equal-weight daily simple return of the current top
// momentum basket over the last regimeVolWindow sessions.
const (
	regimeBasketSize = 10
	regimeVolWindow  = 21
	// regimeBand: held positions are trimmed once s has fallen at least this far
	// below the scale last applied to them (hysteresis against churn).
	regimeBand = 0.20

	regimeQQQSymbol = "QQQBUSDT"
)

// RegimeScale is one cycle's market-state position scale. Scale multiplies the
// risk budget, per-symbol cap and total exposure cap of new entries. A disabled
// or data-starved regime has Scale 1 and never blocks trading.
type RegimeScale struct {
	Enabled      bool     `json:"enabled"`
	Scale        float64  `json:"scale"`
	BasketVolPct float64  `json:"basket_vol_pct"` // annualized, percent (0 = not computed)
	TargetVolPct float64  `json:"target_vol_pct"`
	Floor        float64  `json:"floor"`
	Basket       []string `json:"basket,omitempty"` // names the volatility was computed on
	Breadth      int      `json:"breadth"`          // valid-momentum names above their SMA200
	BreadthTotal int      `json:"breadth_total"`    // valid-momentum names
	Reason       string   `json:"reason,omitempty"` // why Scale is 1 without a computation
}

// ComputeRegime resolves the regime scale from the scan report. The basket is
// the top regimeBasketSize momentum names (scan-excluded names skipped, held
// names NOT excluded — the same basket the backtest used). Missing or
// misaligned data yields Scale 1 with an explanatory Reason.
func ComputeRegime(cfg *store.StockConfig, report *ScanReport) RegimeScale {
	if !cfg.RegimeEnabled() {
		return RegimeScale{Scale: 1}
	}
	out := RegimeScale{Enabled: true, Scale: 1,
		TargetVolPct: cfg.EffectiveRegimeTargetVolPct(), Floor: cfg.EffectiveRegimeFloor()}
	if report == nil {
		out.Reason = "no scan report — scale 1"
		return out
	}
	exclude := make(map[string]bool, len(cfg.ScanExclude))
	for _, sym := range cfg.ScanExclude {
		exclude[sym] = true
	}
	for _, r := range report.Results {
		if exclude[r.Symbol] || r.Symbol == scanBenchmarkSymbol || r.Symbol == regimeQQQSymbol || !r.Momentum.Valid {
			continue
		}
		out.BreadthTotal++
		if r.Momentum.AboveSMA200 {
			out.Breadth++
		}
	}
	basket := report.TopMomentum(regimeBasketSize, exclude)
	if len(basket) == 0 {
		out.Reason = "empty momentum basket — scale 1"
		return out
	}
	// Reference calendar: SPY's closes when it has a full window, else the first
	// basket name that does.
	var dates []string
	if spy, ok := report.Lookup(scanBenchmarkSymbol); ok && len(spy.RecentCloses) == regimeVolWindow+1 {
		dates = closeDates(spy.RecentCloses)
	} else {
		for _, r := range basket {
			if len(r.RecentCloses) == regimeVolWindow+1 {
				dates = closeDates(r.RecentCloses)
				break
			}
		}
	}
	if dates == nil {
		out.Reason = fmt.Sprintf("no name with %d daily closes — scale 1", regimeVolWindow+1)
		return out
	}
	sum := make([]float64, regimeVolWindow)
	for _, r := range basket {
		rets, ok := alignedReturns(r.RecentCloses, dates)
		if !ok {
			continue
		}
		for i, v := range rets {
			sum[i] += v
		}
		out.Basket = append(out.Basket, r.Symbol)
	}
	if len(out.Basket) == 0 {
		out.Reason = "no basket name aligned with the reference calendar — scale 1"
		return out
	}
	n := float64(len(out.Basket))
	mean := 0.0
	for i := range sum {
		sum[i] /= n
		mean += sum[i]
	}
	mean /= regimeVolWindow
	ss := 0.0
	for _, v := range sum {
		ss += (v - mean) * (v - mean)
	}
	sigma := math.Sqrt(ss/(regimeVolWindow-1)) * math.Sqrt(252) * 100
	out.BasketVolPct = sigma
	if sigma > 0 && finite(sigma) {
		out.Scale = math.Min(math.Max(out.TargetVolPct/sigma, out.Floor), 1)
	} else {
		out.BasketVolPct = 0
		out.Reason = "basket volatility not positive — scale 1"
	}
	return out
}

// RegimeTrimFraction reports whether a holding last sized at applied should be
// trimmed now that the scale is scale: it must have fallen at least regimeBand
// below applied, and the returned fraction 1 − scale/applied is the share of
// the owned quantity to sell.
func RegimeTrimFraction(scale, applied float64) (float64, bool) {
	if !positive(applied) || !finite(scale) || scale > applied-regimeBand+1e-9 {
		return 0, false
	}
	return 1 - scale/applied, true
}

func closeDates(cs []DatedClose) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Date
	}
	return out
}

// alignedReturns returns the simple daily returns of cs over dates (one fewer
// than dates), or false when cs lacks a usable close on any of them.
func alignedReturns(cs []DatedClose, dates []string) ([]float64, bool) {
	by := make(map[string]float64, len(cs))
	for _, c := range cs {
		by[c.Date] = c.Close
	}
	rets := make([]float64, 0, len(dates)-1)
	prev := 0.0
	for i, d := range dates {
		c, ok := by[d]
		if !ok || !positive(c) {
			return nil, false
		}
		if i > 0 {
			rets = append(rets, c/prev-1)
		}
		prev = c
	}
	return rets, true
}
