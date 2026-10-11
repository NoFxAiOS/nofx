// Volatility-targeted position management and rule-based trailing stop.
//
// 规则1/2: every cycle, each open position's notional is compared against the
// volatility target (equity × risk% ÷ ATR%) inside an 80/120 hysteresis band —
// inside the band nothing happens, above 120% the excess is reduced (adds are
// left to the AI's judgment and only surfaced in logs).
//
// 规则3/4: exits are structural and one-way. The trailing stop arms at 1.5×
// the initial stop distance and trails 2×ATR(1h), monotonic tighten-only; once
// price travels the full TP distance the fixed TP order is converted into the
// trend-run (cancelled, the trailing stop takes over). SL is never widened.
package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	notify "nofx/telegram/notify"
	"nofx/trader/types"
	"strings"
	"time"
)

const (
	volBandLow  = 0.80 // hysteresis band
	volBandHigh = 1.20
	// Minimum interval between volatility resizes of one position — the band
	// bounds the trigger, this bounds the churn (fees/slippage erosion).
	volResizeCooldown = time.Hour
	trailArmMult      = 1.5 // arm the trail at 1.5× the initial stop distance
	trailATRMult      = 2.0 // trail distance: 2×ATR(1h)
	trailMinImprove   = 0.1 // % improvement required before moving the SL
)

// volTargetNotional: equity × risk% ÷ ATR% — the volatility-targeted position
// value. atrPct is in percent (1.5 = 1.5%).
func volTargetNotional(equity, riskPct, atrPct float64) float64 {
	if equity <= 0 || riskPct <= 0 || atrPct <= 0 {
		return 0
	}
	return equity * (riskPct / 100) / (atrPct / 100)
}

// resizeAction classifies actual vs target notional through the hysteresis band.
func resizeAction(actual, target float64) string {
	if target <= 0 || actual <= 0 {
		return "none"
	}
	ratio := actual / target
	switch {
	case ratio > volBandHigh:
		return "reduce"
	case ratio < volBandLow:
		return "underweight" // surfaced, never auto-added
	default:
		return "none"
	}
}

// Replace only the pre-existing order IDs, after confirming the new stop.
func (at *AutoTrader) moveStopExchange(symbol, side string, newSL float64) error {
	// review 2026-10-07 B1-2: all stop-move paths must preserve the
	// latest recorded protection, including moves within the same cycle.
	if newSL <= 0 || math.IsNaN(newSL) || math.IsInf(newSL, 0) {
		return fmt.Errorf("invalid stop price: %g", newSL)
	}
	currentSL := at.GetRecordedStopLoss(symbol, side)
	eps := math.Max(1e-9, math.Abs(currentSL)*1e-9)
	if currentSL > 0 && ((side == "long" && newSL < currentSL-eps) || (side == "short" && newSL > currentSL+eps)) {
		return fmt.Errorf("stop move would loosen %s %s protection: %.6g -> %.6g", symbol, side, currentSL, newSL)
	}
	qty, ok := at.positionQty(symbol, side)
	if !ok {
		return fmt.Errorf("live quantity unreadable")
	}
	old, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return err
	}
	if err = at.trader.SetStopLoss(symbol, strings.ToUpper(side), qty, newSL); err != nil {
		return err
	}
	slErr, _ := at.verifyProtectiveLegs(&kernel.Decision{Symbol: symbol, StopLoss: newSL}, strings.ToUpper(side), true, false, qty, 0)
	if slErr != nil {
		return slErr
	}
	retiredAny := false
	for _, o := range old {
		typ := strings.ToUpper(o.Type)
		if !strings.Contains(typ, "STOP") || strings.Contains(typ, "TAKE_PROFIT") || o.OrderID == "" {
			continue
		}
		// One-way mode reports PositionSide "BOTH" — it IS this side (2026-10-03
		// review P1: the strict-equality filter skipped BOTH rows, so every
		// stop move on one-way adapters left the old leg armed; its trigger
		// could later fire into an opposite position on qty-sized adapters).
		if o.PositionSide != "" && !strings.EqualFold(o.PositionSide, side) && !strings.EqualFold(o.PositionSide, "BOTH") {
			continue
		}
		if o.StopPrice > 0 && math.Abs(o.StopPrice-newSL)/newSL < 0.001 {
			continue
		}
		var cancelErr error
		var hadCapability = false
		if c, ok := at.trader.(interface {
			CancelProtectiveOrder(string, types.OpenOrder) error
		}); ok {
			hadCapability = true
			cancelErr = c.CancelProtectiveOrder(symbol, o)
		} else if c, ok := at.trader.(interface{ CancelOrder(string, string) error }); ok {
			hadCapability = true
			cancelErr = c.CancelOrder(symbol, o.OrderID)
		}
		// 2026-10-03 review P1: Gate/KuCoin implement NEITHER interface — the
		// retirement was a SILENT no-op and every stop move stacked another
		// armed reduce-only leg at the old price. Surface it (deduped alert).
		if !hadCapability {
			at.gateNotifyRecord("stopretire:"+symbol, time.Now())
			logger.Warnf("⚠️ [%s] %s %s: adapter cannot retire old stop leg %s @ %.6g — armed legs will accumulate at stale prices on this exchange, manual cleanup required", at.name, symbol, side, o.OrderID, o.StopPrice)
			continue
		}
		if cancelErr != nil {
			retiredAny = true
			// 2026-10-03 review P2: the Telegram alert on retirement failure
			// was downgraded to a log — a stuck duplicate leg is Telegram-
			// silent again. Restore via the deduped channel.
			streak, push := at.gateNotifyRecord("stopretirefail:"+symbol+":"+o.OrderID, time.Now())
			logger.Warnf("old stop %s retirement failed; new stop retained: %v (streak %d)", o.OrderID, cancelErr, streak)
			if push {
				notify.Notify("ALERT", at.name, fmt.Sprintf(
					"<b>⚠️ 止损新旧并存 %s</b>\n<i>新止损 %.6g 已生效,旧单 %s 撤除失败(%v)——冗余挂单请人工清理</i>",
					notify.Escape(symbol), newSL, o.OrderID, notify.Escape(cancelErr.Error())))
			}
		}
	}
	_ = retiredAny
	return nil
}

// trailingDecision computes the rule-based stop move for one position.
// Returns (newSL, move, trailingActive). The trail arms at 1.5× the initial
// stop distance and candidates 2×ATR from the mark price; a move only happens
// when it TIGHTENS the stop by ≥ trailMinImprove% of price. Never widens.
func trailingDecision(side string, entry, initialSL, markPrice, atrPct, currentSL float64) (float64, bool, bool) {
	initialDist := math.Abs(entry - initialSL)
	if initialDist <= 0 || markPrice <= 0 || atrPct <= 0 {
		return currentSL, false, false
	}
	mark := markPrice
	profitDist := 0.0
	if side == "long" {
		profitDist = (mark - entry) / entry * 100
	} else {
		profitDist = (entry - mark) / entry * 100
	}
	armed := profitDist >= trailArmMult*(initialDist/entry*100)
	if !armed {
		return currentSL, false, false
	}
	atrPrice := atrPct / 100 * mark
	var candidate float64
	if side == "long" {
		candidate = mark - trailATRMult*atrPrice
		if candidate <= currentSL*(1+trailMinImprove/100) {
			return currentSL, false, true // armed but no meaningful tightening yet
		}
		return math.Max(currentSL, candidate), candidate > currentSL, true
	}
	candidate = mark + trailATRMult*atrPrice
	if candidate >= currentSL*(1-trailMinImprove/100) {
		return currentSL, false, true
	}
	return math.Min(currentSL, candidate), candidate < currentSL, true
}

// tpRunnerDue: price has travelled the full planned TP distance — the fixed
// TP order converts into the trend-run (cancelled, trailing SL takes over).
func tpRunnerDue(side string, entry, takeProfit, markPrice float64) bool {
	if takeProfit <= 0 || markPrice <= 0 {
		return false
	}
	tpDist := math.Abs(takeProfit-entry) / entry * 100
	if tpDist <= 0 {
		return false
	}
	if side == "long" {
		return (markPrice-entry)/entry*100 >= tpDist
	}
	return (entry-markPrice)/entry*100 >= tpDist
}

// timeStopDue: the quick exit-template's time stop (user menu directive
// 09-29). A position older than stopHours whose price-basis PnL is still ≤0
// is a dead trade — the program closes it. stopHours ≤ 0 = disabled; a
// position already in profit is never touched (the breakeven arm / 1R lock
// own the profitable branch).
func timeStopDue(held time.Duration, pricePnLPct, stopHours float64) bool {
	if stopHours <= 0 {
		return false
	}
	return held >= time.Duration(stopHours*float64(time.Hour)) && pricePnLPct <= 0
}

// processVolTargetAndTrailing runs once per decision cycle over all open
// positions: volatility rescale (reduce-only) and trailing stop management.
func (at *AutoTrader) processVolTargetAndTrailing() {
	if at.config.StrategyConfig == nil ||
		(!at.config.StrategyConfig.RiskControl.VolTargetEnabled && !at.config.StrategyConfig.RiskControl.TrailingStopEnabled && kernel.ProfitLockRMult(&at.config.StrategyConfig.RiskControl) <= 0 && kernel.TimeStopHours(&at.config.StrategyConfig.RiskControl) <= 0) {
		return
	}
	positions, err := at.trader.GetPositions()
	if err != nil {
		return
	}
	equity := 0.0
	if balance, err := at.trader.GetBalance(); err == nil {
		if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
			equity = eq
		} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
			equity = eq
		}
	}
	riskPct := at.config.StrategyConfig.RiskControl.RiskPerTradePct
	if riskPct <= 0 {
		riskPct = 1.5
	}
	volEnabled := at.config.StrategyConfig.RiskControl.VolTargetEnabled
	trailEnabled := at.config.StrategyConfig.RiskControl.TrailingStopEnabled

	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		markPrice, _ := pos["markPrice"].(float64)
		qty, _ := pos["positionAmt"].(float64)
		if symbol == "" || markPrice <= 0 || qty == 0 {
			continue
		}
		if qty < 0 {
			qty = -qty
		}
		posKey := symbol + "_" + side
		// Hands-off rule (user directive 2026-09-25): positions NOT opened by
		// the AI get no vol-target, no trailing, no 1R lock, no breakeven arm.
		if !at.isAIManaged(symbol, side) {
			continue
		}
		mode := at.ExitModeFor(symbol, side)
		// ── quick 模式时间止损 (user menu directive 09-29) ──
		// A dead trade is a dead trade: after timeStopHours the position
		// still sits at/below entry (price basis) — the thesis had its
		// window and nothing came of it. This is a PROGRAM path, not an AI
		// close: the early-close/min-hold gates don't apply, and the intent
		// registry marks it 'time_stop' for exit attribution. A position
		// already in profit is left alone — the breakeven arm / 1R lock own
		// it from there.
		if mode == kernel.ExitModeQuick {
			if entry := posEntryPrice(pos); entry > 0 {
				pnlPct := (markPrice - entry) / entry * 100
				if side == "short" {
					pnlPct = (entry - markPrice) / entry * 100
				}
				stopHours := float64(kernel.TimeStopHours(&at.config.StrategyConfig.RiskControl))
				if openMs, ok := at.positionFirstSeenTime[posKey]; ok && openMs > 0 &&
					timeStopDue(time.Since(time.UnixMilli(openMs)), pnlPct, stopHours) {
					held := time.Since(time.UnixMilli(openMs))
					if err := at.closePositionReasoned(symbol, side, "time_stop"); err == nil {
						logger.Infof("⏱️ [%s] Time stop: %s %s held %.1fh still %.2f%% — closed (exit_mode=quick)", at.name, symbol, side, held.Hours(), pnlPct)
						notify.Notify("ORDER", at.name, fmt.Sprintf("<b>⏱️ 时间止损 %s</b>\n<i>exit_mode=quick:持仓 %.1f 小时仍浮亏(%.2f%%),程序市价平仓</i>", notify.Escape(symbol), held.Hours(), pnlPct))
					} else {
						logger.Infof("⚠️ [%s] Time stop close failed for %s: %v — retried next cycle", at.name, symbol, err)
					}
					continue
				}
			}
		}
		currentSL := at.GetRecordedStopLoss(symbol, side)
		if currentSL <= 0 {
			continue // pre-upgrade position or unknown stop — leave alone
		}
		data, err := at.getMarketData(symbol)
		if err != nil {
			continue
		}
		// review 2026-10-07 B1-2: freeze opening risk separately from the
		// live stop so breakeven cannot lower the trailing arm threshold.
		initialSL := at.initialStopAnchor(symbol, side, currentSL)
		// Yardstick: equity tokens ride the DAILY scale (session gaps make
		// 1h ATR too tight for their trailing band too — same rationale as
		// the stop band, user directive 2026-09-25); crypto keeps 1h.
		var atrPct float64
		if market.IsBStockSymbol(symbol) {
			atrPct = dailyATRPct(data)
		} else {
			atrPct = oneHourATRPct(data)
		}

		// ── 规则1/2: volatility-targeted rescale (reduce-only) ──
		if volEnabled && equity > 0 && atrPct > 0 {
			target := volTargetNotional(equity, riskPct, atrPct)
			actual := qty * markPrice
			if minSize := at.config.StrategyConfig.RiskControl.MinPositionSize; target < minSize {
				// target below tradable size — close the position entirely
				at.markCloseIntent(symbol, side, "vol_target_reduce")
				if _, err := at.reducePosition(symbol, side, qty); err == nil {
					logger.Infof("📉 [%s] Vol-target: %s notional %.2f below min size (target %.2f) — closed", at.name, symbol, actual, target)
					notify.Notify("ORDER", at.name, fmt.Sprintf("<b>📉 波动率调仓 %s</b>\\n目标仓位 %.2f 低于最小交易单位,已清仓", notify.Escape(symbol), target))
				}
				continue
			}
			switch resizeAction(actual, target) {
			case "reduce":
				if at.volResizeAllowed(posKey) {
					at.markCloseIntent(symbol, side, "vol_target_reduce")
					reduceQty := qty - target/markPrice
					if _, err := at.reducePosition(symbol, side, reduceQty); err == nil {
						at.markVolResize(posKey)
						logger.Infof("📉 [%s] Vol-target reduce: %s %.2f → %.2f USDT (ATR %.2f%%, band >120%%)", at.name, symbol, actual, target, atrPct)
						notify.Notify("ORDER", at.name, fmt.Sprintf("<b>📉 波动率减仓 %s</b>\\n敞口 %.2f → %.2f USDT(目标=ATR目标位,滞后带>120%%)", notify.Escape(symbol), actual, target))
					}
				}
			case "underweight":
				logger.Infof("📊 [%s] Vol-target: %s underweight (ratio < 80%%) — add-back is AI's call if the signal still holds", at.name, symbol)
			}
		}

		// ── intermediate breakeven arm (breakeven_arm_r, user 2026-09-24) ──
		// At armR × initial risk the stop moves to entry(+beOffset) EARLY —
		// no trim, the reduction ladder stays with TpTierAction/1R lock.
		// One-shot by the same currentSL gating as the 1R lock: once armed
		// the recorded stop sits at bePrice and the arm condition goes false.
		// Arm-only-when-tightening is built into ProfitLockTargets' final
		// gate (an AI tighten past BE simply never arms this).
		if armR := kernel.BreakevenArmR(&at.config.StrategyConfig.RiskControl); armR > 0 {
			entry := posEntryPrice(pos)
			beOffset := kernel.ProfitLockBreakevenOffsetR(&at.config.StrategyConfig.RiskControl)
			bePrice, _ := kernel.ProfitLockTargets(side, entry, initialSL, currentSL, markPrice, armR, beOffset)
			if bePrice > 0 {
				if err := at.moveStopExchange(symbol, side, bePrice); err != nil {
					logger.Infof("⚠️ [%s] early-breakeven SL place failed for %s: %v", at.name, symbol, err)
				} else {
					at.SetRecordedStopLoss(symbol, side, bePrice)
					currentSL = bePrice // review 2026-10-07 B1-2: later moves use the new floor.
					logger.Infof("🔒 [%s] Early breakeven armed: %s %.2fR reached — SL moved to %.6g (entry %.6g, +%.2fR floor; give-back zone closed)", at.name, symbol, armR, bePrice, entry, beOffset)
					notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🔒 提前保本 %s</b>\n浮盈达 %.1fR,止损已先期移至开仓价+%.1fR <code>%.6g</code>——最差结果保本+微利出局", notify.Escape(symbol), armR, beOffset, bePrice))
				}
			}
		}

		// ── 1R profit lock (user 2026-09-12): breakeven stop + 50% trim ──
		if lockR := kernel.ProfitLockRMult(&at.config.StrategyConfig.RiskControl); lockR > 0 {
			entry := posEntryPrice(pos)
			// The R anchor is the OPENING stop — the plan the position was
			// sized against — deliberately NOT the live recorded stop. Every
			// tighten above entry would otherwise compress the R distance and
			// fire the trim on pocket change (ONDOUSDT 2026-09-17: live SL
			// tightened 0.3428 → 0.3512 made +0.49% read as 1.89R → 50%
			// trimmed at 0.352 instead of true 1R at 0.3578). initialSL is
			// resolved before any stop moves, including the legacy fallback
			// for positions with no persisted anchor.
			//
			// The live stop (passed as currentSL) still gates breakeven: once
			// armed, SetRecordedStopLoss overwrites it to bePrice (entry plus
			// the configured R-offset), so the breakeven condition goes false
			// and arms exactly once — no extra "already armed" bookkeeping.
			// Trim idempotency is r1TrimDone.
			beOffset := kernel.ProfitLockBreakevenOffsetR(&at.config.StrategyConfig.RiskControl)
			bePrice, trim := kernel.ProfitLockTargets(side, entry, initialSL, currentSL, markPrice, lockR, beOffset)
			// review 2026-10-11: a partial trim is safe only with its runner
			// protected at breakeven. Zero means the recorded stop is already
			// at/through the target (including an earlier breakeven arm).
			breakevenConfirmed := bePrice == 0 || (side == "long" && currentSL >= bePrice) || (side == "short" && currentSL <= bePrice)
			if bePrice > 0 {
				if err := at.moveStopExchange(symbol, side, bePrice); err != nil {
					logger.Infof("⚠️ [%s] Breakeven SL place failed for %s: %v", at.name, symbol, err)
				} else {
					at.SetRecordedStopLoss(symbol, side, bePrice)
					currentSL = bePrice // review 2026-10-07 B1-2: trailing must retain breakeven.
					breakevenConfirmed = true
					if beOffset > 0 {
						logger.Infof("🔒 [%s] Breakeven+%.1fR armed: %s %dR reached — SL moved to %.6g (entry %.6g + %.1fR, locks a sliver past flat)", at.name, beOffset, symbol, int(lockR), bePrice, entry, beOffset)
						notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🔒 保本+%.1fR 止损 %s</b>\n浮盈达 %.0fR,止损已移至开仓价+%.1fR <code>%.6g</code>——锁住一档微利,防噪声扫回平手", beOffset, notify.Escape(symbol), lockR, beOffset, bePrice))
					} else {
						logger.Infof("🔒 [%s] Breakeven armed: %s %dR reached — SL moved to entry %.6g", at.name, symbol, int(lockR), entry)
						notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🔒 保本止损 %s</b>\n浮盈达 %.0fR,止损已移至开仓价 <code>%.6g</code>——最差结果保本出局", notify.Escape(symbol), lockR, entry))
					}
				}
			}
			// tp_trim_yields_to_lock=false (division of labor): the ROE trim
			// tier owns reductions (tp_trim_profit_pct, once) and the lock
			// contributes ONLY the breakeven stop above — no 50% reduce, no
			// consuming the trim marker. Default (true) keeps the 09-21
			// lock-supersedes-trim behavior below.
			if trim && at.config.StrategyConfig.RiskControl.TrimYieldsToLock() {
				at.tpTrimMutex.Lock()
				done := at.r1TrimDone[posKey]
				at.tpTrimMutex.Unlock()
				// 09-28 review P1: r1TrimDone is memory-only — after a restart
				// it is empty while the position can still sit ≥1R (the
				// breakeven stop is re-seeded from the exchange, the R anchor
				// is restored from the DB row), and ProfitLockTargets would
				// return trim=true again: a SECOND unrequested 50% reduction.
				// The persisted flag on the OPEN row is the
				// restart-persistent idempotency marker.
				if !done {
					done = at.storeR1TrimDone(symbol, side)
				}
				if !done {
					minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
					remainder := qty * 0.5 * markPrice
					if minSize > 0 && remainder < minSize {
						// remainder would be dust — lock the whole thing instead
						if err := at.closePositionReasoned(symbol, side, "profit_lock_trim"); err == nil {
							at.tpTrimMutex.Lock()
							at.r1TrimDone[posKey] = true
							at.tpTrimDone[posKey] = true
							at.tpTrimMutex.Unlock()
							at.persistTrimFlags(symbol, side)
							logger.Infof("🎯 [%s] 1R lock: %s remainder below min size — closed fully instead of trimming", at.name, symbol)
							notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🎯 1R 止盈 %s</b>\n浮盈达 %.0fR,剩余仓位低于最小单位,已全部平仓锁定利润", notify.Escape(symbol), lockR))
						} else {
							logger.Infof("❌ [%s] 1R full close failed (%s): %v", at.name, symbol, err)
						}
						continue
					}
					if !breakevenConfirmed {
						logger.Infof("⚠️ [%s] 1R trim skipped for %s: breakeven SL not confirmed — retries next cycle", at.name, symbol)
					} else {
						at.markCloseIntent(symbol, side, "profit_lock_trim")
						if _, err := at.reducePosition(symbol, side, qty*0.5); err == nil {
							at.tpTrimMutex.Lock()
							at.r1TrimDone[posKey] = true
							at.tpTrimDone[posKey] = true // the ROE trim tier is consumed by the 1R lock
							at.tpTrimMutex.Unlock()
							at.persistTrimFlags(symbol, side)
							logger.Infof("🎯 [%s] 1R trim: %s 50%% trimmed @ %.6g (breakeven SL, rest rides to structural TP)", at.name, symbol, markPrice)
							notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🎯 1R 止盈减仓 %s</b>\n浮盈达 %.0fR,市价减仓 50%% 锁定利润,剩余仓位止损已保本、继续持有", notify.Escape(symbol), lockR))
						} else {
							logger.Infof("❌ [%s] 1R trim failed (%s) — retries next cycle", at.name, symbol)
						}
					}
				}
			}
		}

		// ── 规则3/4: rule-based trailing stop + TP runner ──
		// Trend-template only (user menu directive 09-29): range/quick chose
		// "the target IS the exit" — their full-size TP algo owns the exit
		// and no runner conversion ever fires (the watchdog's TP repair uses
		// the same recorded level for them).
		if trailEnabled && mode == kernel.ExitModeTrend {
			newSL, move, armed := trailingDecision(side, posEntryPrice(pos), initialSL, markPrice, atrPct, currentSL)
			if armed {
				if move {
					if err := at.moveStopExchange(symbol, side, newSL); err != nil {
						logger.Infof("⚠️ [%s] Trailing SL place failed for %s: %v", at.name, symbol, err)
					} else {
						at.SetRecordedStopLoss(symbol, side, newSL)
						currentSL = newSL // review 2026-10-07 B1-2: retain every successful move.
						logger.Infof("🔒 [%s] Trailing stop moved: %s SL → %.6g (armed %.1f%%, trail 2×ATR)", at.name, symbol, newSL, trailArmMult)
						notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🔒 移动止损 %s</b>\\nSL 推进至 <code>%.6g</code>(2×ATR 跟踪,只紧不松)", notify.Escape(symbol), newSL))
					}
				}
				// 规则5: once price travels the full TP distance, convert the
				// fixed TP into the trend-run (trailing stop takes over).
				var tp float64
				if at.config.StrategyConfig != nil {
					tp = at.getOpenTakeProfit(symbol, side)
				}
				if tp > 0 && tpRunnerDue(side, posEntryPrice(pos), tp, markPrice) && !at.tpRunnerDone(posKey) {
					_ = at.trader.CancelTakeProfitOrders(symbol)
					at.markTPRunner(posKey)
					logger.Infof("🏃 [%s] TP runner: %s reached its TP distance — fixed TP cancelled, 2×ATR trail takes over", at.name, symbol)
					notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🏃 止盈转趋势跟踪 %s</b>\\n到达计划止盈距离,固定止盈单撤除,剩余仓位由移动止损接管", notify.Escape(symbol)))
				}
			}
		}
	}
}

func posEntryPrice(pos map[string]interface{}) float64 {
	if v, ok := pos["entryPrice"].(float64); ok {
		return v
	}
	return 0
}

// initialStopAnchor resolves the opening-risk stop for the 1R lock: the stop
// planned at entry, frozen per position (in-memory write-once + set-if-empty
// stamp on the OPEN position row) so stop adjustments and restarts can't pull
// the R bar along. Resolution order: in-memory anchor → persisted row → the
// live recorded stop as a legacy fallback (pre-anchor positions), which then
// freezes deterministically on first sight.
func (at *AutoTrader) initialStopAnchor(symbol, side string, liveSL float64) float64 {
	if sl := at.GetInitialStopLoss(symbol, side); sl > 0 {
		// Re-stamp on every scan: the position row is created by trade sync
		// shortly after open, later than the in-memory anchor — this heals
		// the row cheaply (the UPDATE is a no-op once stamped).
		at.SetInitialStopLoss(symbol, side, sl)
		return sl
	}
	if at.store != nil {
		// Position rows store side as "LONG"/"SHORT"; the exchange feed here
		// is lowercase.
		if pos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side)); err == nil && pos != nil && pos.InitialStopLoss > 0 {
			at.SetInitialStopLoss(symbol, side, pos.InitialStopLoss)
			return pos.InitialStopLoss
		}
	}
	if liveSL > 0 {
		at.SetInitialStopLoss(symbol, side, liveSL)
	}
	return liveSL
}

// reducePosition closes part of a position (market).
func (at *AutoTrader) reducePosition(symbol, side string, qty float64) (map[string]interface{}, error) {
	if side == "long" {
		return at.trader.CloseLong(symbol, qty)
	}
	return at.trader.CloseShort(symbol, qty)
}

// volResizeAllowed / markVolResize: per-position resize cooldown.
func (at *AutoTrader) volResizeAllowed(posKey string) bool {
	at.volResizeMu.Lock()
	defer at.volResizeMu.Unlock()
	if at.volResizeLast == nil {
		at.volResizeLast = make(map[string]time.Time)
	}
	return time.Since(at.volResizeLast[posKey]) >= volResizeCooldown
}

func (at *AutoTrader) markVolResize(posKey string) {
	at.volResizeMu.Lock()
	defer at.volResizeMu.Unlock()
	if at.volResizeLast == nil {
		at.volResizeLast = make(map[string]time.Time)
	}
	at.volResizeLast[posKey] = time.Now()
}

// getOpenTakeProfit reads the recorded TP for an open position (0 if unknown).
func (at *AutoTrader) getOpenTakeProfit(symbol, side string) float64 {
	// The TP lives on the exchange; the recorded decision value is the
	// practical source. Re-deriving it from algo orders costs an API call per
	// position per cycle — use the last known decision TP instead.
	at.openTPMu.RLock()
	defer at.openTPMu.RUnlock()
	return at.openTP[symbol+"_"+side]
}

func (at *AutoTrader) recordOpenTakeProfit(symbol, side string, tp float64) {
	at.openTPMu.Lock()
	defer at.openTPMu.Unlock()
	if at.openTP == nil {
		at.openTP = make(map[string]float64)
	}
	at.openTP[symbol+"_"+side] = tp
}

func (at *AutoTrader) tpRunnerDone(posKey string) bool {
	at.volResizeMu.Lock()
	defer at.volResizeMu.Unlock()
	return at.tpRunnerDoneMap[posKey]
}

func (at *AutoTrader) markTPRunner(posKey string) {
	at.volResizeMu.Lock()
	defer at.volResizeMu.Unlock()
	if at.tpRunnerDoneMap == nil {
		at.tpRunnerDoneMap = make(map[string]bool)
	}
	at.tpRunnerDoneMap[posKey] = true
}

// positionQty reads the LIVE exchange quantity for one symbol+side — R1 fix
// (2026-09-26 review): every adapter emits `positionAmt` (short NEGATIVE on
// Bybit/OKX; the old `quantity` key exists nowhere, so this always returned
// 0 and Bybit/OKX rejected the re-placement with the old stop already
// gone). Returns abs(base-asset size). ok=false when unreadable — the
// caller must NOT cancel the old stop on that path (unknown size ⇒ no
// cancel, no placement, watchdog keeps watching).
func (at *AutoTrader) positionQty(symbol, side string) (float64, bool) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return 0, false
	}
	for _, pos := range positions {
		if pos["symbol"] != symbol || pos["side"] != side {
			continue
		}
		amt, ok := pos["positionAmt"].(float64)
		if !ok || amt == 0 {
			return 0, false
		}
		if amt < 0 {
			amt = -amt
		}
		return amt, true
	}
	return 0, false
}

// storeR1TrimDone reads the restart-persistent 1R-lock trim marker from the
// OPEN trader_positions row (09-28 review P1: the in-memory r1TrimDone map
// dies with the process, and the watchdog/DB restore lets a still-≥1R
// position re-qualify for the trim after a restart). Fail-open: a store
// outage reads as not-trimmed, matching the old behavior.
func (at *AutoTrader) storeR1TrimDone(symbol, side string) bool {
	if at.store == nil {
		return false
	}
	p, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side))
	return err == nil && p != nil && p.R1TrimDone
}

// persistTrimFlags write-through for both one-shot exit-ladder markers —
// called only on SUCCESSFUL reductions (a failed trim must retry next cycle,
// so the flag is deliberately not consumed then).
func (at *AutoTrader) persistTrimFlags(symbol, side string) {
	if at.store == nil {
		return
	}
	if err := at.store.Position().MarkR1TrimDone(at.id, symbol, side); err != nil {
		logger.Warnf("⚠️ [%s] failed to persist r1_trim_done for %s %s: %v", at.name, symbol, side, err)
	}
	if err := at.store.Position().MarkTPTrimDone(at.id, symbol, side); err != nil {
		logger.Warnf("⚠️ [%s] failed to persist tp_trim_done for %s %s: %v", at.name, symbol, side, err)
	}
}

// persistTPTrimDone write-through for the ROE-ladder trim marker only.
func (at *AutoTrader) persistTPTrimDone(symbol, side string) {
	if at.store == nil {
		return
	}
	if err := at.store.Position().MarkTPTrimDone(at.id, symbol, side); err != nil {
		logger.Warnf("⚠️ [%s] failed to persist tp_trim_done for %s %s: %v", at.name, symbol, side, err)
	}
}
