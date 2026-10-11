package trader

import (
	"math"
	"strings"
	"sync"

	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
)

var tpMaxRInvalidLogOnce sync.Once

// CapTakeProfit clamps a planned take-profit to the CLOSER of itself and
// entry ± capR × R, where R = |entry − stop| is the initial risk (long:
// min(tp, entry + capR×R); short: max(tp, entry − capR×R)). capR below 1
// (including 0 / negative) means no cap, as do degenerate inputs (non-positive
// or non-finite prices, stop == entry, stop on the wrong side).
func CapTakeProfit(isLong bool, entry, stop, tp, capR float64) (capped float64, changed bool) {
	if !finitePositive(entry) || !finitePositive(stop) || !finitePositive(tp) || math.IsNaN(capR) || math.IsInf(capR, 0) || capR < 1 {
		return tp, false
	}
	if isLong {
		risk := entry - stop
		if risk <= 0 {
			return tp, false
		}
		if limit := entry + capR*risk; tp > limit {
			return limit, true
		}
		return tp, false
	}
	risk := stop - entry
	if risk <= 0 {
		return tp, false
	}
	if limit := entry - capR*risk; limit > 0 && tp < limit {
		return limit, true
	}
	return tp, false
}

func finitePositive(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// effectiveTPMaxR resolves the configured take-profit cap (0 = none), logging
// once when the value sits in the invalid (0, 1) range.
func (at *AutoTrader) effectiveTPMaxR() float64 {
	if at.config.StrategyConfig == nil {
		return 0
	}
	rc := at.config.StrategyConfig.RiskControl
	if rc.TPMaxR > 0 && rc.TPMaxR < 1 {
		tpMaxRInvalidLogOnce.Do(func() {
			logger.Warnf("⚠️ [%s] risk_control.tp_max_r=%.2f is below 1R (would contradict min_risk_reward_ratio) — ignored, no take-profit cap", at.name, rc.TPMaxR)
		})
	}
	return rc.EffectiveTPMaxR()
}

// capOpenTakeProfit applies risk_control.tp_max_r to an open decision in place
// (entry = the price validateOpenRisk anchors the plan at). Returns true when
// the TP was moved toward entry. Bstock symbols are not capped.
func (at *AutoTrader) capOpenTakeProfit(decision *kernel.Decision, entryPrice float64) bool {
	capR := at.effectiveTPMaxR()
	if capR <= 0 || market.IsBStockSymbol(decision.Symbol) {
		return false
	}
	isLong := strings.HasPrefix(decision.Action, "open_long")
	capped, changed := CapTakeProfit(isLong, entryPrice, decision.StopLoss, decision.TakeProfit, capR)
	if !changed {
		return false
	}
	side := "short"
	if isLong {
		side = "long"
	}
	logger.Infof("📐 [%s] %s %s take-profit capped at %.2fR: planned TP %.6g -> %.6g (entry %.6g, SL %.6g)",
		at.name, decision.Symbol, side, capR, decision.TakeProfit, capped, entryPrice, decision.StopLoss)
	decision.TakeProfit = capped
	return true
}
