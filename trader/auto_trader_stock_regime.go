package trader

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nofx/kernel/stockengine"
	"nofx/market/usstock"
	"nofx/store"
)

// us_stock regime position scaler (basket_vol): the cycle's market-state scale
// and the trim of held positions when it falls. Everything here is a no-op, with
// no store access, while the strategy's regime_scaler is off.

// stockRegimeMaxReportAge bounds how old a scan report may be and still drive
// the scale; beyond it the scale falls back to 1 (daily closes only change once
// a session, so a day-old report is fine, a stale outage is not).
const stockRegimeMaxReportAge = 72 * time.Hour

// stockRegimeTrimReason is the close reason booked for a regime trim.
const stockRegimeTrimReason = "regime_trim"

// stockCycleRegime computes this cycle's regime scale from the scan report and
// logs one line. Disabled → the inert scale 1 and no log.
func (at *AutoTrader) stockCycleRegime(sc *store.StockConfig, report *stockengine.ScanReport, now time.Time,
	logf func(string, ...interface{})) stockengine.RegimeScale {
	if !sc.RegimeEnabled() {
		return stockengine.RegimeScale{Scale: 1}
	}
	if report != nil && now.Sub(report.GeneratedAt) > stockRegimeMaxReportAge {
		logf("regime scan report from %s is too old — scale 1", report.GeneratedAt.In(stockET).Format("01-02 15:04 MST"))
		report = nil
	}
	regime := stockengine.ComputeRegime(sc, report)
	if regime.Reason != "" {
		logf("regime %s s=%.2f: %s", sc.EffectiveRegimeScaler(), regime.Scale, regime.Reason)
	} else {
		logf("regime %s s=%.2f σ=%.1f%% basket=[%s] breadth=%d/%d", sc.EffectiveRegimeScaler(), regime.Scale, regime.BasketVolPct,
			strings.Join(regime.Basket, " "), regime.Breadth, regime.BreadthTotal)
	}
	return regime
}

// applyStockRegimeTrim sells down held program-owned positions whose applied
// regime scale has fallen by at least the band, and cleans up the applied-scale
// rows of positions that are gone. Manual (non-owned) quantity is never touched,
// the scale rising never buys. When a trim was attempted the account state is
// gathered again (as the cycle did) so the prompt and sizing see post-trim
// numbers; the (possibly new) state is returned.
func (at *AutoTrader) applyStockRegimeTrim(ctx context.Context, sc *store.StockConfig, regime stockengine.RegimeScale, session usstock.Session,
	state *stockState, snaps []*stockengine.SymbolSnapshot, prices map[string]float64, quarantined map[string]string,
	rec *store.DecisionRecord, now time.Time, logf func(string, ...interface{})) (*stockState, error) {
	if !regime.Enabled {
		return state, nil
	}
	scales := at.store.StockRegimeScales()
	applied, err := scales.List(at.id)
	if err != nil {
		logf("regime scale store read failed: %v — no trim this cycle", err)
		return state, nil
	}
	// Positions that are gone no longer carry an applied scale.
	for sym := range applied {
		h := state.Holdings[sym]
		if (h == nil || (h.Owned <= 0 && !h.Quarantined)) && at.stock.pendings[sym] == nil {
			if err := scales.Delete(at.id, sym); err != nil {
				logf("regime scale delete %s failed: %v", sym, err)
			}
			delete(applied, sym)
		}
	}
	if !sc.RegimeTrimEnabled() || session != usstock.SessionRegular {
		return state, nil
	}
	paper := sc.IsPaper()
	pfx := stockPrefix(paper)
	snapBy := map[string]*stockengine.SymbolSnapshot{}
	for _, s := range snaps {
		snapBy[s.Symbol] = s
	}
	trimmed := false
	for _, h := range state.sortedHoldings() {
		if h.Owned <= 0 || h.Quarantined || at.stock.pendings[h.Symbol] != nil {
			continue
		}
		prev, ok := applied[h.Symbol]
		if !ok {
			prev = 1
		}
		frac, need := stockengine.RegimeTrimFraction(regime.Scale, prev)
		if !need {
			continue
		}
		d := stockengine.Decision{Symbol: h.Symbol, Action: stockengine.ActionReduceLong, ReduceFraction: frac}
		act := store.DecisionAction{Action: d.Action, Symbol: d.Symbol, Leverage: 1, Timestamp: now.UTC(),
			Reasoning: fmt.Sprintf("REGIME_TRIM s=%.2f applied=%.2f σ=%.1f%%", regime.Scale, prev, regime.BasketVolPct)}
		trimmed = true
		out, xerr := at.execStockExitReason(ctx, d, snapBy[h.Symbol], h, paper, now, stockRegimeTrimReason)
		tooSmall := xerr != nil && strings.HasPrefix(xerr.Error(), "REDUCE_TOO_SMALL")
		switch {
		case xerr != nil:
			act.Error = pfx + xerr.Error()
			logf("regime trim %s %.0f%% (s=%.2f applied=%.2f) FAILED: %v", h.Symbol, frac*100, regime.Scale, prev, xerr)
		default:
			act.Success, act.Quantity, act.Price = true, out.Qty, out.Price
			act.OrderID, act.EntryOrderID = orderIDInt(out.OrderID), out.OrderID
			logf("regime trim %s %.0f%% (s=%.2f applied=%.2f) ok: qty=%.6f price=%.4f%s", h.Symbol, frac*100, regime.Scale, prev, out.Qty, out.Price, noteSuffix(out.Note))
			at.notifyStockOutcome(paper, d, out)
		}
		rec.Decisions = append(rec.Decisions, act)
		switch {
		case xerr == nil && out.Closed:
			if err := scales.Delete(at.id, h.Symbol); err != nil {
				logf("regime scale delete %s failed: %v", h.Symbol, err)
			}
		case xerr == nil || tooSmall:
			// A too-small remainder is recorded as applied too: retrying every cycle
			// would only repeat the rejection.
			if err := scales.Set(at.id, h.Symbol, regime.Scale); err != nil {
				logf("regime scale save %s failed: %v", h.Symbol, err)
			}
		}
	}
	if !trimmed {
		return state, nil
	}
	var fresh *stockState
	if paper {
		fresh, err = at.stockGatherPaper(nil)
		if err == nil {
			fresh.repricePaper(prices)
		}
	} else {
		fresh, err = at.stockGatherLiveWithQuarantine(ctx, now, true, true, quarantined)
	}
	if err != nil {
		return state, fmt.Errorf("account state after regime trim: %w", err)
	}
	return fresh, nil
}

// setStockRegimeScale records the scale a position was just sized at (entry).
func (at *AutoTrader) setStockRegimeScale(regime stockengine.RegimeScale, sym string, logf func(string, ...interface{})) {
	if !regime.Enabled {
		return
	}
	if err := at.store.StockRegimeScales().Set(at.id, sym, regime.Scale); err != nil {
		logf("regime scale save %s failed: %v", sym, err)
	}
}

// deleteStockRegimeScale forgets the applied scale of a fully closed position.
func (at *AutoTrader) deleteStockRegimeScale(regime stockengine.RegimeScale, sym string, logf func(string, ...interface{})) {
	if !regime.Enabled {
		return
	}
	if err := at.store.StockRegimeScales().Delete(at.id, sym); err != nil {
		logf("regime scale delete %s failed: %v", sym, err)
	}
}
