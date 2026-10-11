package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	notify "nofx/telegram/notify"
	"nofx/trader/binance"
	"nofx/trader/types"
	"strings"
	"sync"
	"time"
)

// gateNotifyState tracks consecutive hard-gate blocks per symbol so repeated
// bars (the AI retrying every cycle) don't spam Telegram — the first push
// goes out, follow-ups within the window are logged with a running counter.
// (Rebuilt 09-13 after the risk.go corruption — window approximate.)
type gateNotifyState struct {
	Streak int
	Last   time.Time
}

// gateNotifyRecord returns the running streak for a gate key and whether a
// Telegram push should go out (first block, or the first block after the
// dedup window).
func (at *AutoTrader) gateNotifyRecord(key string, now time.Time) (int, bool) {
	at.gateNotifyMu.Lock()
	defer at.gateNotifyMu.Unlock()
	if at.gateNotify == nil {
		at.gateNotify = make(map[string]*gateNotifyState)
	}
	const dedupWindow = 30 * time.Minute // pinned by TestGateNotifyDedup
	if st, ok := at.gateNotify[key]; ok && now.Sub(st.Last) < dedupWindow {
		// Follow-up inside the window: silent, streak climbs, Last unchanged —
		// pushes stay at most once per window per key.
		st.Streak++
		return st.Streak, false
	}
	at.gateNotify[key] = &gateNotifyState{Streak: 1, Last: now}
	return 1, true
}

// drawdownProtectThresholds resolves the drawdown-protect pair: profit above
// minProfit% that then retraces maxDD% of the peak triggers the protective
// close. 0/unset = 5/55 defaults.
func (at *AutoTrader) drawdownProtectThresholds() (minProfit, maxDD float64) {
	minProfit, maxDD = 5.0, 55.0
	if at.config.StrategyConfig == nil {
		return minProfit, maxDD
	}
	rc := at.config.StrategyConfig.RiskControl
	if rc.PeakDrawdownMinProfitPct > 0 {
		minProfit = rc.PeakDrawdownMinProfitPct
	}
	if rc.PeakDrawdownMaxDDPct > 0 {
		maxDD = rc.PeakDrawdownMaxDDPct
	}
	return minProfit, maxDD
}

// protectionMonitorInterval: the between-cycles protection tick (F6,
// 2026-10-01 review). The decision cycle remains the primary pass; this
// ticker bounds how long a just-filled limit entry waits for its protective
// orders when the fill lands right after a cycle (previously: up to a full
// ScanInterval plus AI latency).
const protectionMonitorInterval = 30 * time.Second

// startProtectionMonitor runs fill finalization (processPendingEntries) and
// the protection watchdog on a dedicated ticker so a fill that lands between
// decision cycles is protected within seconds, not on the next cycle
// boundary. The shared execution mutex serializes order state changes; AI
// waits release it so monitoring continues during slow external requests.
func (at *AutoTrader) startProtectionMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()
		ticker := time.NewTicker(protectionMonitorInterval)
		defer ticker.Stop()
		logger.Infof("🛡️ [%s] Protection monitor started (fill/watchdog check every %s between cycles)", at.name, protectionMonitorInterval)
		for {
			select {
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped protection monitor")
				return
			case <-ticker.C:
				at.safeProtectionPass()
			}
		}
	}()
}

// safeProtectionPass guards one monitor pass with a recover backstop — the
// monitor must never die silently on an unexpected panic.
func (at *AutoTrader) safeProtectionPass() {
	mu := at.executionMutex()
	mu.Lock()
	defer mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("🛡️ [%s] protection monitor pass panicked: %v", at.name, r)
		}
	}()
	at.processPendingEntries(false) // 30s pass: protection & fills only — no direction wobble cancels
	at.processProtectionWatchdog()
	if at.gridState != nil && at.config.StrategyConfig != nil && at.config.StrategyConfig.GridConfig != nil {
		at.syncGridState()
		at.checkAndExecuteStopLoss()
	}
}

// startDrawdownMonitor starts drawdown monitoring
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // Check every minute
		defer ticker.Stop()

		logger.Info("📊 Started position drawdown monitoring (check every minute)")

		for {
			select {
			case <-ticker.C:
				at.safeCheckPositionDrawdown()
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped position drawdown monitoring")
				return
			}
		}
	}()
}

// safeCheckPositionDrawdown runs one drawdown-check pass with a recover
// backstop — the type-asserted fields above are now guarded, but this monitor
// must never die silently on an unexpected panic (e.g. a future edit that
// reintroduces an unguarded assertion): one bad cycle logs and moves on
// instead of taking down the whole goroutine for the rest of the process.
func (at *AutoTrader) safeCheckPositionDrawdown() {
	mu := at.executionMutex()
	mu.Lock()
	defer mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("❌ Drawdown monitoring: recovered from panic: %v", r)
		}
	}()
	at.checkPositionDrawdown()
}

// checkPositionDrawdown checks position drawdown situation
func (at *AutoTrader) checkPositionDrawdown() {
	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Warnf("❌ Drawdown monitoring: failed to get positions: %v", err)
		return
	}

	for _, pos := range positions {
		symbol, ok := pos["symbol"].(string)
		if !ok || symbol == "" {
			logger.Warnf("⚠️ Drawdown monitoring: position missing/malformed symbol, skipping: %+v", pos)
			continue
		}
		side, ok := pos["side"].(string)
		if !ok || side == "" {
			logger.Warnf("⚠️ Drawdown monitoring: %s missing/malformed side, skipping", symbol)
			continue
		}
		entryPrice, ok := pos["entryPrice"].(float64)
		if !ok {
			logger.Warnf("⚠️ Drawdown monitoring: %s %s missing/malformed entryPrice, skipping", symbol, side)
			continue
		}
		markPrice, ok := pos["markPrice"].(float64)
		if !ok {
			logger.Warnf("⚠️ Drawdown monitoring: %s %s missing/malformed markPrice, skipping", symbol, side)
			continue
		}
		quantity, ok := pos["positionAmt"].(float64)
		if !ok {
			logger.Warnf("⚠️ Drawdown monitoring: %s %s missing/malformed positionAmt, skipping", symbol, side)
			continue
		}
		if quantity < 0 {
			quantity = -quantity // Short position quantity is negative, convert to positive
		}

		// Guard: skip if entry price is zero (prevents division by zero panic)
		if entryPrice <= 0 {
			logger.Warnf("⚠️ Drawdown monitoring: %s %s has zero entry price, skipping", symbol, side)
			continue
		}

		// Calculate current P&L percentage
		// Hands-off rule: drawdown-protect force-closes are automation too —
		// manual positions are the user's to manage.
		if ps, ok := pos["symbol"].(string); ok {
			sd, _ := pos["side"].(string)
			if !at.isAIManaged(ps, sd) {
				continue
			}
		}
		leverage := 10 // Default value
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// R-mode yardstick (2026-09-29 unit unification): the favorable
		// excursion in R against the WRITE-ONCE opening stop — the same anchor
		// the 1R lock prices. Unlike the ROE number above it does not move
		// with the leverage the model happened to report for this trade.
		rcRef := &at.config.StrategyConfig.RiskControl
		ladderR := kernel.TpLadderUsesR(rcRef)
		ddArmR := kernel.PeakDrawdownArmR(rcRef)
		var pnlR float64
		if ladderR || ddArmR > 0 {
			anchor := at.initialStopAnchor(symbol, side, at.GetRecordedStopLoss(symbol, side))
			if anchor > 0 && entryPrice > 0 {
				if riskDist := math.Abs(entryPrice - anchor); riskDist > 0 {
					fav := markPrice - entryPrice
					if side != "long" {
						fav = entryPrice - markPrice
					}
					pnlR = fav / riskDist
				}
			}
		}
		// The value the shared peak cache tracks: R when the drawdown protect
		// runs on R units, leveraged ROE otherwise. The mode is fixed per
		// process (config loads at start), so a cache never mixes units.
		protectValue := currentPnLPct
		if ddArmR > 0 {
			protectValue = pnlR
		}

		// Construct unique position identifier (distinguish long/short)
		posKey := symbol + "_" + side

		// Get historical peak profit for this position
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// If no historical peak record, use current P&L as initial value
			peakPnLPct = protectValue
			at.UpdatePeakPnL(symbol, side, protectValue)
		} else {
			// Update peak cache
			at.UpdatePeakPnL(symbol, side, protectValue)
		}

		// Calculate drawdown (magnitude of decline from peak)
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// ── TP ladder (user 2026-09-11): ≥ full → close the rest; ≥ trim →
		// market-trim 1/3 once. Leveraged PnL%, same basis as the peak cache.
		at.tpTrimMutex.Lock()
		trimDone := at.tpTrimDone[posKey]
		at.tpTrimMutex.Unlock()
		// Restart-persistent marker (09-28 review P1): the in-memory flag dies
		// with the process, and a position still above the ROE trim threshold
		// would re-fire the 1/3 reduction after a restart.
		if !trimDone {
			if at.store != nil {
				if p, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side)); err == nil && p != nil {
					trimDone = p.TPTrimDone
				}
			}
		}
		// Ladder verdict: R units when the R tiers are configured, legacy
		// leveraged ROE otherwise (TpLadderUsesR). Both share the same tier
		// shape and yields-to-lock flags.
		var ladderAction string
		if ladderR {
			ladderAction = kernel.TpTierActionR(pnlR, trimDone, rcRef)
		} else {
			ladderAction = kernel.TpTierAction(currentPnLPct, trimDone, rcRef)
		}
		switch ladderAction {
		case "full":
			fullTxt := fmt.Sprintf("PnL %.2f%% ≥ %.0f%%", currentPnLPct, kernel.TpFullProfitPct(rcRef))
			if ladderR {
				fullTxt = fmt.Sprintf("PnL %.2fR ≥ %.1fR", pnlR, kernel.TpFullAtR(rcRef))
			}
			logger.Infof("🎯 TP ladder FULL close: %s %s | %s", symbol, side, fullTxt)
			notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🎯 止盈全平 %s (%s)</b>\n浮盈 <code>%s</code>,程序全部平仓锁定利润。",
				notify.Escape(symbol), strings.ToUpper(side[:1])+side[1:], fullTxt))
			if err := at.closePositionReasoned(symbol, side, "tp_full"); err != nil {
				logger.Infof("❌ TP full close failed (%s %s): %v — retries next cycle", symbol, side, err)
			} else {
				// Consume the one-shot flags on SUCCESS only: the full tier
				// retries regardless, but a pre-consumed trimDone would mute
				// the ROE trim tier forever if PnL dips back below the full
				// threshold with the close still unfilled (round-4 R4-8).
				at.tpTrimMutex.Lock()
				at.r1TrimDone[posKey] = true
				at.tpTrimDone[posKey] = true
				at.tpTrimMutex.Unlock()
				at.persistTrimFlags(symbol, side)
				logger.Infof("✅ TP full close succeeded: %s %s", symbol, side)
			}
			continue
		case "trim":
			// The legacy ROE-percent tier can coexist with the R-based lock when
			// tp_trim_yields_to_lock=false. Their units are different, so impose
			// an explicit order: protection reaches breakeven before any trim.
			if err := at.ensureBreakevenBeforeAutomatedTrim(symbol, side, entryPrice, markPrice); err != nil {
				logger.Infof("❌ TP trim deferred (%s %s): breakeven stop not secured: %v", symbol, side, err)
				continue
			}
			minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
			if minSize <= 0 {
				minSize = kernel.MinPositionSizeDefaultUSDT
			}
			if remainder := quantity * (2.0 / 3.0) * markPrice; remainder < minSize {
				logger.Infof("🎯 TP ladder TRIM: %s %s remainder %.2f below min %.2f — closing fully instead of leaving dust", symbol, side, remainder, minSize)
				if err := at.closePositionReasoned(symbol, side, "tp_trim"); err != nil {
					logger.Infof("❌ TP dust-safe full close failed (%s %s): %v — retries next cycle", symbol, side, err)
				} else {
					at.tpTrimMutex.Lock()
					at.tpTrimDone[posKey] = true
					at.tpTrimMutex.Unlock()
					at.persistTPTrimDone(symbol, side)
				}
				continue
			}
			trimQty := quantity / 3
			trimTxt := fmt.Sprintf("PnL %.2f%% ≥ %.0f%%", currentPnLPct, kernel.TpTrimProfitPct(rcRef))
			if ladderR {
				trimTxt = fmt.Sprintf("PnL %.2fR ≥ %.1fR", pnlR, kernel.TpTrimAtR(rcRef))
			}
			logger.Infof("🎯 TP ladder TRIM: %s %s | %s — trimming 1/3 (%.6g)", symbol, side, trimTxt, trimQty)
			notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🎯 止盈减仓 1/3 %s (%s)</b>\n浮盈 <code>%s</code>,程序市价减仓 1/3。",
				notify.Escape(symbol), strings.ToUpper(side[:1])+side[1:], trimTxt))
			var err error
			at.markCloseIntent(symbol, side, "tp_trim")
			if side == "long" {
				_, err = at.trader.CloseLong(symbol, trimQty)
			} else {
				_, err = at.trader.CloseShort(symbol, trimQty)
			}
			if err != nil {
				// Flag NOT consumed on failure: trimDone=true here would skip
				// this 1/3 take-profit forever (one-shot gate in
				// TpTierAction) — retry next cycle instead (round-4 R4-8).
				logger.Infof("❌ TP trim failed (%s %s): %v — retries next cycle", symbol, side, err)
			} else {
				at.tpTrimMutex.Lock()
				at.tpTrimDone[posKey] = true
				at.tpTrimMutex.Unlock()
				at.persistTPTrimDone(symbol, side)
				logger.Infof("✅ TP trim succeeded: %s %s qty=%.6g (protective orders kept)", symbol, side, trimQty)
			}
			continue
		}

		// Check close condition. R mode (peak_drawdown_arm_r > 0): the peak
		// (in R) reached the arm and ≥ giveback_r of it was surrendered —
		// leverage-independent, unlike the legacy leveraged-ROE pair
		// (peak_drawdown_min_profit_pct / peak_drawdown_max_dd_pct, 5/55).
		minProfit, maxDD := at.drawdownProtectThresholds()
		var protectClose bool
		var closeDesc string
		if ddArmR > 0 {
			giveback := kernel.PeakDrawdownGivebackR(rcRef)
			gb := 0.0
			if peakPnLPct > 0 {
				gb = (peakPnLPct - protectValue) / peakPnLPct
			}
			protectClose = peakPnLPct >= ddArmR && gb >= giveback
			closeDesc = fmt.Sprintf("峰值 %.2fR → 当前 %.2fR(回吐 %.0f%% ≥ %.0f%%,arm %.1fR)", peakPnLPct, protectValue, gb*100, giveback*100, ddArmR)
		} else {
			protectClose = currentPnLPct > minProfit && drawdownPct >= maxDD
			closeDesc = fmt.Sprintf("Current profit: %.2f%% | Peak profit: %.2f%% | Drawdown: %.2f%% (thresholds: >%.1f%% & ≥%.1f%%)", currentPnLPct, peakPnLPct, drawdownPct, minProfit, maxDD)
		}
		if protectClose {
			logger.Infof("🚨 Drawdown close position condition triggered: %s %s | %s", symbol, side, closeDesc)
			notify.Notify("RISK", at.name, fmt.Sprintf("<b>🚨 回撤保护平仓 %s (%s)</b>\n<i>%s</i>\n触发保护性平仓锁定利润。",
				notify.Escape(symbol), strings.ToUpper(side[:1])+side[1:], closeDesc))

			// Execute close position
			if err := at.closePositionReasoned(symbol, side, "drawdown_protect"); err != nil {
				logger.Infof("❌ Drawdown close position failed (%s %s): %v", symbol, side, err)
				notify.Notify("ALERT", at.name, fmt.Sprintf("<b>❌ 回撤保护平仓失败 %s (%s)</b>\n<code>%s</code>\n请人工检查持仓！",
					notify.Escape(symbol), strings.ToUpper(side[:1])+side[1:], notify.Escape(err.Error())))
			} else {
				logger.Infof("✅ Drawdown close position succeeded: %s %s", symbol, side)
				notify.Notify("ORDER", at.name, fmt.Sprintf("<b>✅ 回撤保护平仓完成 %s (%s)</b>\n浮盈 <code>%+.2f%%</code> 已落袋。", symbol, strings.ToUpper(side[:1])+side[1:], currentPnLPct))
				// Clear cache for this position after closing
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if ddArmR == 0 && currentPnLPct > minProfit {
			// Record situations close to close position condition (for debugging)
			logger.Infof("📊 Drawdown monitoring: %s %s | Profit: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// ensureBreakevenBeforeAutomatedTrim serializes mixed-unit profit tiers: an
// ROE-percent trim may fire before an R tier, but it may never reduce the
// position while the remainder still carries loss-side stop risk.
func (at *AutoTrader) ensureBreakevenBeforeAutomatedTrim(symbol, side string, entryPrice, markPrice float64) error {
	if entryPrice <= 0 || markPrice <= 0 {
		return fmt.Errorf("entry/mark unavailable")
	}
	currentSL := at.GetRecordedStopLoss(symbol, side)
	if (side == "long" && currentSL >= entryPrice) || (side == "short" && currentSL > 0 && currentSL <= entryPrice) {
		return nil
	}
	if (side == "long" && markPrice <= entryPrice) || (side == "short" && markPrice >= entryPrice) {
		return fmt.Errorf("position is not profitable at mark %.6g", markPrice)
	}
	if err := at.moveStopExchange(symbol, side, entryPrice); err != nil {
		return err
	}
	at.SetRecordedStopLoss(symbol, side, entryPrice)
	logger.Infof("🔒 [%s] Pre-trim breakeven secured: %s %s SL → %.6g", at.name, symbol, side, entryPrice)
	return nil
}

// emergencyClosePosition emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close long position succeeded, order ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close short position succeeded, order ID: %v", order["orderId"])
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}

	at.ClearRecordedStopLoss(symbol, side)
	at.ClearInitialStopLoss(symbol, side)
	at.ClearExitMode(symbol, side)
	return nil
}

// GetPeakPnLCache gets peak profit cache
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// Return a copy of the cache
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL updates peak profit cache
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// Update peak (if long, take larger value; if short, currentPnLPct is negative, also compare)
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// First time recording
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache clears peak cache for specified position
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)

	// Trim/ladder state shares the peak-PnL lifecycle: a fresh position
	// starts with its trims unspent.
	at.tpTrimMutex.Lock()
	delete(at.tpTrimDone, posKey)
	delete(at.r1TrimDone, posKey)
	delete(at.partialTrimmed, posKey)
	at.tpTrimMutex.Unlock()
	// The TP-runner flag shares the same lifecycle: without this delete a
	// re-opened symbol_side inherits "runner done" — the fixed TP would
	// never convert to the trend-run again and the watchdog would refuse to
	// re-place a cancelled TP (round-4 review R4-7).
	at.volResizeMu.Lock()
	delete(at.tpRunnerDoneMap, posKey)
	at.volResizeMu.Unlock()
}

// ============================================================================
// Risk Control Helpers
// ============================================================================

// isBTCETH checks if a symbol is BTC or ETH
func isBTCETH(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	return strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH")
}

// enforcePositionValueRatio checks and enforces position value ratio limits (CODE ENFORCED)
// Returns the adjusted position size (capped if necessary) and whether the position was capped
// positionSizeUSD: the original position size in USD
// equity: the account equity
// symbol: the trading symbol
func (at *AutoTrader) enforcePositionValueRatio(positionSizeUSD float64, equity float64, symbol string) (float64, bool) {
	if at.config.StrategyConfig == nil {
		return positionSizeUSD, false
	}

	riskControl := at.config.StrategyConfig.RiskControl

	// Get the appropriate position value ratio limit
	var maxPositionValueRatio float64
	if isBTCETH(symbol) {
		maxPositionValueRatio = riskControl.BTCETHMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 5.0 // Default: 5x for BTC/ETH
		}
	} else {
		maxPositionValueRatio = riskControl.AltcoinMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 1.0 // Default: 1x for altcoins
		}
	}

	// Calculate max allowed position value = equity × ratio
	maxPositionValue := equity * maxPositionValueRatio

	// Check if position size exceeds limit
	if positionSizeUSD > maxPositionValue {
		logger.Infof("  ⚠️ [RISK CONTROL] Position %.2f USDT exceeds limit (equity %.2f × %.1fx = %.2f USDT max for %s), capping",
			positionSizeUSD, equity, maxPositionValueRatio, maxPositionValue, symbol)
		return maxPositionValue, true
	}

	return positionSizeUSD, false
}

// enforceMinPositionSize checks minimum position size (CODE ENFORCED)
func (at *AutoTrader) enforceMinPositionSize(positionSizeUSD float64) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
	if minSize <= 0 {
		minSize = 12 // Default: 12 USDT
	}

	if positionSizeUSD < minSize {
		return fmt.Errorf("❌ [RISK CONTROL] Position %.2f USDT below minimum (%.2f USDT)", positionSizeUSD, minSize)
	}
	return nil
}

// enforceMaxPositions checks maximum positions count (CODE ENFORCED)
// enforceMaxPositions checks maximum positions count (CODE ENFORCED).
// Callers pass the POST-OPEN count (nextSlotCount: open + 1 for this entry +
// resting entries on OTHER symbols), so the cap must compare with > — a
// count == max IS the cap being exactly reached. The old `>=` (from the
// pre-slot-accounting era when callers passed the raw open count) made the
// effective cap max−1: with max=5 and 4 open positions the 5th open was
// rejected as "Already at max positions (5/5)" (PROMUSDT 2026-09-30).
func (at *AutoTrader) enforceMaxPositions(currentPositionCount int) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	maxPositions := at.config.StrategyConfig.RiskControl.MaxPositions
	if maxPositions <= 0 {
		maxPositions = 3 // Default: 3 positions
	}

	if currentPositionCount > maxPositions {
		return fmt.Errorf("❌ [RISK CONTROL] would exceed max positions (%d > %d)", currentPositionCount, maxPositions)
	}
	return nil
}

// getSideFromAction converts order action to side (BUY/SELL)
func getSideFromAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}

// ============================================================================
// Open-risk validation (stop window rides dual yardsticks)
// ============================================================================

// validateOpenRisk enforces entry risk gates on open decisions before any
// order is sent: mandatory stop-loss, SL/TP side sanity, the strategy's
// minimum risk-reward ratio, and an outlier wide-stop cap. Returns an error
// to reject the open. The stop window rides two yardsticks (user directive
// 2026-09-10): the noise FLOOR scales on ATR(1h) — the rhythm the trade
// actually needs to survive — while the wide-stop CAP stays on ATR(4h)
// (max(2×ATR(4h), 8%)): the 1h cap bound at the 8% floor on violent movers
// and pushed structural stops out of the band.
func (at *AutoTrader) validateOpenRisk(decision *kernel.Decision, entryPrice, floorATRPct, capATRPct float64) error {
	if at.config.StrategyConfig == nil || entryPrice <= 0 {
		return nil
	}
	// Bstock risk geometry is deliberately daily-scale. If the 1d series is
	// unavailable, reject instead of degrading to crypto ATRs or the fixed 8%
	// cap; that would make the executor validate a different plan than the
	// prompt-side 4h-1d stop/target scan.
	if market.IsBStockSymbol(decision.Symbol) && (floorATRPct <= 0 || capATRPct <= 0) {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: BSTOCK_DAILY_DATA_UNAVAILABLE (ATR(1d) required)", decision.Action, decision.Symbol)
	}
	minRR := at.config.StrategyConfig.RiskControl.MinRiskRewardRatio
	isLong := strings.HasPrefix(decision.Action, "open_long")

	// 1. Stop loss is mandatory — it is placed on the exchange right after
	// the open, so an entry without one would run unprotected.
	if decision.StopLoss <= 0 {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: missing stop_loss (entry would be unprotected)", decision.Action, decision.Symbol)
	}

	// 1b. Take profit is mandatory too (audit 09-13 #3): the min-RR gate only
	// runs when a TP exists, so an open that simply omits take_profit used to
	// bypass the entire reward-side validation AND place no TP protection on
	// the exchange. The prompt requires TP on every open — enforce it.
	if decision.TakeProfit <= 0 {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: missing take_profit (min-RR gate requires a target; omitting TP does not skip validation)", decision.Action, decision.Symbol)
	}

	// 2. SL/TP must sit on the correct side of the entry price, otherwise the
	// exchange rejects the conditional orders.
	if isLong && decision.StopLoss >= entryPrice {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: stop_loss %.6g must be below entry %.6g", decision.Action, decision.Symbol, decision.StopLoss, entryPrice)
	}
	if !isLong && decision.StopLoss <= entryPrice {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: stop_loss %.6g must be above entry %.6g", decision.Action, decision.Symbol, decision.StopLoss, entryPrice)
	}
	if decision.TakeProfit > 0 {
		if isLong && decision.TakeProfit <= entryPrice {
			return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: take_profit %.6g must be above entry %.6g", decision.Action, decision.Symbol, decision.TakeProfit, entryPrice)
		}
		if !isLong && decision.TakeProfit >= entryPrice {
			return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: take_profit %.6g must be below entry %.6g", decision.Action, decision.Symbol, decision.TakeProfit, entryPrice)
		}
	}

	// 2b. Stop-vs-liquidation sanity (D3, QUANT_REVIEW 2026-09-22): nothing
	// compared the stop distance with the liquidation distance, so a wide
	// stop on high leverage could place the liquidation price BEFORE the
	// stop. Approximation: isolated liq distance ≈ 1/leverage of notional
	// minus a 10% maintenance-margin/safety haircut; reject when the stop
	// sits beyond 80% of that move — inside that band a wick can liquidate
	// (or cascade-slippage the SL) before the stop triggers.
	if decision.Leverage > 0 {
		liqDistPct := 100.0 / float64(decision.Leverage) * 0.9
		stopDistPct := math.Abs(entryPrice-decision.StopLoss) / entryPrice * 100
		if stopDistPct >= liqDistPct*0.8 {
			return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: stop distance %.2f%% ≥ 80%% of the ~%.2f%% liquidation distance at %dx — price can reach liquidation before the stop; widen the stop's budget by cutting leverage or size",
				decision.Action, decision.Symbol, stopDistPct, liqDistPct, decision.Leverage)
		}
	}

	// 2c. Take-profit cap (risk_control.tp_max_r): clamp the planned TP to
	// entry ± N×R BEFORE the RR gates below, so min-RR judges the plan that is
	// actually routed (market recordOpenTakeProfit / limit pendingEntry both
	// read decision.TakeProfit after this returns). A cap that drops the plan
	// under min RR is rejected by the gates below with the cap named.
	tpCapped := at.capOpenTakeProfit(decision, entryPrice)
	capNote := func(err error) error {
		if tpCapped && err != nil {
			return fmt.Errorf("%w (take_profit was capped to %.6g by risk_control.tp_max_r=%.2f)", err, decision.TakeProfit, at.effectiveTPMaxR())
		}
		return err
	}

	// 3. Minimum risk-reward ratio, anchored at BOTH the AI's decision price
	// and the live execution price. Validating only the live ticker let a
	// market-order fill away from the checked price smuggle in a below-floor
	// plan (SOL: decision RR 2.08 vs required 3.0, passed on a lower ticker).
	if minRR > 0 && decision.TakeProfit > 0 {
		if decision.Price > 0 {
			if err := checkNetRR(decision, decision.Price, minRR, at.config.StrategyConfig.RiskControl.EffectiveEntryRoundTripCostBps()); err != nil {
				return capNote(err)
			}
		}
		if err := checkNetRR(decision, entryPrice, minRR, at.config.StrategyConfig.RiskControl.EffectiveEntryRoundTripCostBps()); err != nil {
			return capNote(err)
		}
	}

	// 4. Outlier wide-stop cap: SL distance ≤ max(2×ATR(4h), 8%). Volatile
	// coins get proportionally more room (a stop inside 2×ATR is noise
	// fodder), but never unbounded — an 8.9% stop at 3x leverage is -27%
	// margin on one trade. The CAP yardstick stays 4h (user directive
	// 2026-09-10): on the 1h scale it bound at the 8% floor and rejected
	// structural stops on exactly the violent movers the pool surfaces.
	slDistPct := math.Abs(decision.StopLoss-entryPrice) / entryPrice * 100
	maxSLDist := 8.0
	if capATRPct*2 > maxSLDist {
		maxSLDist = capATRPct * 2
	}
	if slDistPct > maxSLDist {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: stop distance %.2f%% exceeds cap %.2f%% (max(2×ATR(4h) %.2f%%, 8%%))",
			decision.Action, decision.Symbol, slDistPct, maxSLDist, capATRPct)
	}

	// 5. Noise floor: stop closer than SLMinATRMult × ATR(1h) gets swept by
	// normal fluctuation before the trend can develop (the FLOOR yardstick is
	// 1h — the rhythm the trade must survive). Together with the cap above
	// this pins the stop to [floor, cap].
	if floorMult := at.config.StrategyConfig.RiskControl.SLMinATRMult; floorMult > 0 && floorATRPct > 0 {
		floor := floorMult * floorATRPct
		if slDistPct < floor {
			// 09-19 PONS case: the stop that IS the gated stop_plan was
			// rejected because the LIMIT anchor (a BETTER short entry, +1.2%)
			// shrank the plan's percentage distance below the floor. The plan
			// was validated in-band at the gate's snapshot basis, and the
			// pre-computed anchor only shifts the basis in the favorable
			// direction — the stop is still the real structure level, and RR
			// is re-checked at this basis above. Exempt it.
			if !at.stopMatchesGatedPlan(decision) {
				return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: stop distance %.2f%% below noise floor %.2f%% (%.1f×ATR(1h) %.2f%%) — widen to the next structure level",
					decision.Action, decision.Symbol, slDistPct, floor, floorMult, floorATRPct)
			}
			logger.Infof("📐 [%s] stop %.6g = gated stop_plan (d %.2f%% at the limit basis < %.2f%% floor) — in-band at the gate basis, allowed",
				decision.Symbol, decision.StopLoss, slDistPct, floor)
		}
	}
	return nil
}

// stopMatchesGatedPlan reports whether the decision's stop equals the
// stop_plan_price the hard gate validated for this direction (± the same
// 0.05% echo tolerance as the post-parse snap, which runs before validation
// — plan-compliant stops arrive here already equalized).
func (at *AutoTrader) stopMatchesGatedPlan(decision *kernel.Decision) bool {
	if at.cycleGateStates == nil || decision.StopLoss <= 0 {
		return false
	}
	gs, ok := at.cycleGateStates[market.Normalize(decision.Symbol)]
	if !ok || gs == nil {
		return false
	}
	plan := gs.LongStopPlanPrice
	if strings.HasPrefix(decision.Action, "open_short") {
		plan = gs.ShortStopPlanPrice
	}
	if plan <= 0 {
		return false
	}
	dev := math.Abs(decision.StopLoss-plan) / plan * 100
	return dev <= 0.05
}

// checkRR validates the risk-reward ratio at one price anchor.
func checkRR(decision *kernel.Decision, entryPrice, minRR float64) error {
	isLong := strings.HasPrefix(decision.Action, "open_long")
	risk := entryPrice - decision.StopLoss
	reward := decision.TakeProfit - entryPrice
	if !isLong {
		risk = decision.StopLoss - entryPrice
		reward = entryPrice - decision.TakeProfit
	}
	if risk <= 0 {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: invalid stop distance", decision.Action, decision.Symbol)
	}
	rr := reward / risk
	if rr < minRR {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: risk-reward 1:%.2f below required 1:%.1f (anchor %.6g SL %.6g TP %.6g)",
			decision.Action, decision.Symbol, rr, minRR, entryPrice, decision.StopLoss, decision.TakeProfit)
	}
	return nil
}

// ============================================================================
// ATR yardsticks
// ============================================================================

// atrPctFromTimeframes returns ATR(14) as a percent of price from the first
// timeframe in the priority list with enough bars — deterministic; 0 when
// none qualify. warnSymbol ("" = silent, for prompt-side mirrors) logs when
// the chosen scale is NOT the nominal one: the floor's yardstick silently
// jumping from 1h to 4h changes what the same multiplier means, and the
// divergence was previously invisible (B2, QUANT_REVIEW 09-22 — open paths
// only, so the volume is one line per attempted open).
func atrPctFromTimeframes(data *market.Data, nominal string, warnSymbol string, names ...string) float64 {
	if data == nil {
		return 0
	}
	for _, name := range names {
		if tf, ok := data.TimeframeData[name]; ok && len(tf.Klines) >= 15 {
			if warnSymbol != "" && name != nominal {
				logger.Infof("📏 [%s] ATR yardstick degraded: %s scale unavailable, using %s ATR — the floor/cap multiplier now means something different on this symbol", warnSymbol, nominal, name)
			}
			return atrPercentFromSeries(tf)
		}
	}
	return 0
}

// oneHourATRPct returns ATR(14) as a percent of price on the 1h timeframe.
// The stop-loss noise FLOOR's yardstick (user directive 2026-09-10: buffer
// and floor ride the 1h rhythm the trade must survive) and the vol-target
// rescale / trailing stop yardstick. Falls back toward other TFs
// deterministically; 0 when unavailable.
// dailyATRPct returns exact ATR(14) on the DAILY timeframe. Bstock callers
// fail closed when it is unavailable; falling back to an intraday ATR would
// silently change the configured stop/target rhythm.
func dailyATRPct(data *market.Data) float64 {
	if data == nil || data.TimeframeData == nil {
		return 0
	}
	if tf := data.TimeframeData["1d"]; tf != nil {
		return atrPercentFromSeries(tf)
	}
	return 0
}

// stopBandATRs resolves the (floor, cap) ATR pair for one symbol: equity
// tokens price BOTH off the DAILY scale (floor sl_min×ATR(1d), cap
// 2×ATR(1d) with the 8% floor-cap retained), everything else keeps the
// 1h-floor / 4h-cap asymmetry. The isStock check rides the market package's
// cached classification (same source as the weekend gate).
func (at *AutoTrader) stopBandATRs(symbol string, data *market.Data) (floorATR, capATR float64) {
	if market.IsBStockSymbol(symbol) {
		daily := dailyATRPct(data)
		return daily, daily
	}
	return oneHourATRPct(data), fourHourATRPct(data)
}

func oneHourATRPct(data *market.Data) float64 {
	return atrPctFromTimeframes(data, "1h", "", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "30m", "15m", "5m", "3m")
}

// fourHourATRPct returns ATR(14) as a percent of price on the 4h timeframe —
// the wide-stop CAP yardstick (user directive 2026-09-10: on the 1h scale the
// cap bound at the 8% floor and pushed structural stops out of the band on
// exactly the violent movers the candidate pool surfaces).
func fourHourATRPct(data *market.Data) float64 {
	return atrPctFromTimeframes(data, "4h", "", "4h", "6h", "8h", "12h", "1d", "2h", "1h", "30m", "15m", "5m", "3m")
}

// atrPercentFromSeries computes Wilder ATR(14) as a percent of the last
// close from a timeframe's kline series. CLOSED bars only (09-28 review P2):
// the series tail is the live-patched forming candle and biasing Wilder ATR
// low by ~1.5-3.5% steady-state made the executor's stop-band floor/cap and
// protection levels diverge from the kernel's closed-bars-only ATR — the
// same gate/executor parity class as the 09-19 BTCUSDT loop.
func atrPercentFromSeries(tf *market.TimeframeSeriesData) float64 {
	dur := market.TimeframeDuration(tf.Timeframe)
	now := time.Now().UnixMilli()
	kb := make([]market.Kline, 0, len(tf.Klines))
	for _, b := range tf.Klines {
		if dur > 0 && b.Time+dur.Milliseconds() > now {
			continue // forming candle
		}
		kb = append(kb, market.Kline{OpenTime: b.Time, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume})
	}
	// Wilder ATR(14), percent of the last close.
	n := len(kb)
	if n < 15 {
		return 0
	}
	tr := make([]float64, n)
	tr[0] = kb[0].High - kb[0].Low
	for i := 1; i < n; i++ {
		pc := kb[i-1].Close
		tr[i] = math.Max(kb[i].High-kb[i].Low, math.Max(math.Abs(kb[i].High-pc), math.Abs(kb[i].Low-pc)))
	}
	a := 0.0
	for i := 1; i <= 14; i++ {
		a += tr[i]
	}
	a /= 14
	for i := 15; i < n; i++ {
		a = (a*13 + tr[i]) / 14
	}
	lastClose := kb[n-1].Close
	if lastClose <= 0 {
		return 0
	}
	return a / lastClose * 100
}

// clampSizeToRisk caps the position value at equity × RiskPerTradePct% ÷
// stop-distance% — sizing derives from the stop, never the other way round
// (framework: 单笔风险金额固定,止损越远仓位越小). Returns the capped size.
func (at *AutoTrader) clampSizeToRisk(decision *kernel.Decision, sizeUSD, equity, livePrice float64) float64 {
	riskPct := at.config.StrategyConfig.RiskControl.EffectiveRiskPerTradePct()
	if equity <= 0 || livePrice <= 0 || decision.StopLoss <= 0 || sizeUSD <= 0 {
		return sizeUSD
	}
	distPct := math.Abs(livePrice-decision.StopLoss) / livePrice * 100
	// Same cost caliber as checkNetRR (single resolver).
	distPct += at.config.StrategyConfig.RiskControl.EffectiveEntryRoundTripCostBps() / 100
	if distPct <= 0 {
		return sizeUSD
	}
	maxNotional := equity * riskPct / distPct
	if sizeUSD <= maxNotional {
		return sizeUSD
	}
	logger.Infof("  ⚠️ [RISK CONTROL] Position %.2f USDT exceeds risk budget (equity %.2f × %.1f%% ÷ stop %.2f%% = %.2f USDT), clamping",
		sizeUSD, equity, riskPct, distPct, maxNotional)
	return maxNotional
}

// ============================================================================
// Spread gate
// ============================================================================

// topOfBookSpreadPct returns the order-book spread as a percent of mid:
// (bestAsk − bestBid) / ((bestAsk+bestBid)/2) × 100. 0 on malformed input —
// the spread gate fails open, never blocks on unusable data.
func topOfBookSpreadPct(bids, asks [][]float64) float64 {
	if len(bids) == 0 || len(asks) == 0 {
		return 0
	}
	bid, ask := bids[0][0], asks[0][0]
	if bid <= 0 || ask <= 0 || ask < bid {
		return 0 // crossed/garbage book — treat as unusable, fail-open
	}
	mid := (ask + bid) / 2
	return (ask - bid) / mid * 100
}

// spreadBlocksOpen rejects opens on symbols whose live order-book spread
// exceeds the threshold (user directive 2026-09-11, the liquidity gap from
// the original spec): a wide spread eats the limit-order edge and directly
// taxes market fills on thin candidates. Threshold from
// risk_control.max_spread_pct (0 = 0.5% default, negative = disabled).
// Fail-open: no book interface or unusable book never blocks.
func (at *AutoTrader) spreadBlocksOpen(symbol string) (bool, string) {
	if at.config.StrategyConfig == nil {
		return false, ""
	}
	threshold := kernel.MaxSpreadPct(&at.config.StrategyConfig.RiskControl)
	if threshold <= 0 {
		return false, "" // disabled
	}
	book, ok := at.trader.(interface {
		GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error)
	})
	if !ok {
		return false, ""
	}
	bids, asks, err := book.GetOrderBook(symbol, 5)
	if err != nil {
		// Fail-open, but never silently: an unmeasurable book means the gate
		// is blind for this symbol this cycle (audit 09-13 #5).
		logger.Infof("⚠️ [RISK CONTROL] spread gate: order book unavailable for %s (%v) — gate skipped for this cycle", symbol, err)
		return false, ""
	}
	spread := topOfBookSpreadPct(bids, asks)
	if spread <= 0 {
		return false, "" // fail-open
	}
	if spread <= threshold {
		return false, ""
	}
	return true, fmt.Sprintf(
		"spread gate: order-book spread %.3f%% > threshold %.2f%% of mid — thin book, the anchor edge would be eaten by the spread",
		spread, threshold)
}

// ============================================================================
// Margin budget gate
// ============================================================================

// usedMarginOf sums per-position margin (notional ÷ leverage) from open
// positions. A position with a nonzero size but unusable mark/leverage is
// SKIPPED (fail-open) — that silently undercounts used margin, so it logs a
// warning instead of disappearing (audit 09-13 #5).
func usedMarginOf(positions []map[string]interface{}) float64 {
	total := 0.0
	for _, pos := range positions {
		qty, _ := pos["positionAmt"].(float64)
		if qty == 0 {
			continue
		}
		symbol, _ := pos["symbol"].(string)
		mark, _ := pos["markPrice"].(float64)
		lev, _ := pos["leverage"].(float64)
		if mark <= 0 || lev <= 0 {
			logger.Warnf("⚠️ [RISK CONTROL] margin budget: %s position excluded from used-margin (mark=%.6g lev=%.1f) — used margin is UNDERCOUNTED, check exchange data",
				symbol, mark, lev)
			continue
		}
		if qty < 0 {
			qty = -qty
		}
		total += qty * mark / lev
	}
	return total
}

// marginExceedsBudget: (used + new) margin vs budget fraction × equity.
func marginExceedsBudget(usedMargin, newMargin, equity, budgetFrac float64) bool {
	if equity <= 0 || budgetFrac <= 0 {
		return false
	}
	return usedMargin+newMargin > budgetFrac*equity
}

// pendingMarginReserved sums the margin that resting AI limit-entry orders
// on other symbol+side keys would consume if they all filled.
// Each placement passed the margin gate against the used margin AT PLACEMENT
// TIME — none of them sees the others — so without this reservation N
// pending limits checked individually could all fill and jointly blow the
// budget. excludeKey is the same-side entry being replaced; an opposite-side
// order on the same symbol still reserves margin.
func (at *AutoTrader) pendingMarginReserved(excludeKey string) float64 {
	total := 0.0
	for key, pe := range at.accountPendingEntries() {
		if key == "unknown" {
			return math.Inf(1)
		}
		if key == excludeKey || pe == nil || pe.Price <= 0 || pe.Quantity <= 0 {
			continue
		}
		lev := float64(pe.Leverage)
		if lev <= 0 {
			lev = 1 // assume worst case: no leverage → full notional as margin
		}
		remaining := math.Max(0, pe.Quantity-pe.ProtectedQty)
		total += pe.Price * remaining / lev
	}
	return total
}

// marginBudgetBlocksOpen rejects an open when the resulting total margin
// usage would exceed risk_control.max_margin_usage (0 = 90% default). The
// prompt states this budget but until now nothing enforced it — a max-size
// BTC/ETH position alone could cross the 90% line (audit 09-13 #2).
// Resting limit entries count as reserved margin (audit 09-13 #4: N pending
// limits each checked in isolation can jointly exceed the budget once they
// all fill). Fail-open on data errors.
func (at *AutoTrader) marginBudgetBlocksOpen(symbol, side string, newSizeUSD, newLeverage, equity float64) (bool, string) {
	if at.config.StrategyConfig == nil || newSizeUSD <= 0 || newLeverage <= 0 || equity <= 0 {
		return false, ""
	}
	budget := at.config.StrategyConfig.RiskControl.MaxMarginUsage
	if budget <= 0 {
		budget = 0.9
	}
	positions, err := at.trader.GetPositions()
	if err != nil {
		return true, "margin budget: account positions unknown"
	}
	newMargin := newSizeUSD / newLeverage
	pending := at.pendingMarginReserved(pendingEntryKey(symbol, side))
	used := usedMarginOf(positions) + pending
	if !marginExceedsBudget(used, newMargin, equity, budget) {
		return false, ""
	}
	return true, fmt.Sprintf(
		"margin budget: used %.2f (incl. %.2f reserved by resting limit entries) + new %.2f USDT (size %.2f @ %.0fx) would exceed %.0f%% × equity %.2f (%.2f USDT) — reduce size/leverage or close a position first",
		used, pending, newMargin, newSizeUSD, newLeverage, budget*100, equity, budget*equity)
}

// ============================================================================
// Early-close lock
// ============================================================================

// oneHAgainstCandles counts the trailing consecutive CLOSED 1h candles that
// run AGAINST the position direction (long → bearish candles, short →
// bullish candles). The forming bar is excluded. This is the trend-change
// evidence the early-close gate wants: the 1h rhythm itself turning.
func oneHAgainstCandles(data *market.Data, side string) int {
	if data == nil {
		return 0
	}
	tf, ok := data.TimeframeData["1h"]
	if !ok || tf == nil || len(tf.Klines) < 2 {
		return 0
	}
	bars := tf.Klines[:len(tf.Klines)-1] // drop the forming bar — closed candles only
	n := 0
	for i := len(bars) - 1; i >= 0; i-- {
		b := bars[i]
		if b.Close == b.Open {
			break // doji: no direction, breaks the streak
		}
		against := (side == "long" && b.Close < b.Open) || (side == "short" && b.Close > b.Open)
		if !against {
			break
		}
		n++
	}
	return n
}

// earlyCloseBlocksClose gates AI-initiated closes that happen BEFORE the
// position has ridden its intended swing (user directive 2026-09-11): a close
// earlier than EarlyCloseMinHours (default 4h) is only allowed when the 1h
// timeframe shows ≥2 closed candles against the position direction — real
// trend-change evidence, not a wobble. Exempt by construction: exchange
// SL/TP triggers and the drawdown-protect close are program paths, not AI
// decisions; a mark at/beyond the recorded stop also passes (the position is
// stopping out regardless). Fail-open on missing data — a gate must never
// trap a position it cannot see.
func (at *AutoTrader) earlyCloseBlocksClose(symbol, side string, markPrice float64, data *market.Data) (bool, string) {
	if at.config.StrategyConfig == nil {
		return false, ""
	}
	hours := kernel.EarlyCloseHours(&at.config.StrategyConfig.RiskControl)
	if hours <= 0 {
		return false, ""
	}
	posKey := symbol + "_" + side
	openMs, exists := at.positionFirstSeenTime[posKey]
	if !exists || openMs <= 0 {
		return false, "" // age unknown — don't block
	}
	held := time.Since(time.UnixMilli(openMs))
	if held >= time.Duration(hours)*time.Hour {
		return false, ""
	}
	// Hard-exit bypass: the mark is already at/beyond the recorded stop.
	if sl := at.GetRecordedStopLoss(symbol, side); sl > 0 && markPrice > 0 {
		if (side == "long" && markPrice <= sl) || (side == "short" && markPrice >= sl) {
			return false, ""
		}
	}
	evidence := oneHAgainstCandles(data, side)
	if evidence >= 2 {
		return false, "" // 1h trend-change candles present — exit allowed
	}
	return true, fmt.Sprintf(
		"early-close lock: held %.1fh < %dh and 1h shows only %d against-direction closed candle(s) (need ≥2 trend-change candles); SL/TP and drawdown-protect paths unaffected",
		held.Hours(), hours, evidence)
}

// minHoldBlocksClose reports whether an AI-initiated close must be blocked by
// the minimum holding period lock. Hard exits are never blocked: the price
// already crossed the recorded stop-loss (exchange stop handles the exit).
func (at *AutoTrader) minHoldBlocksClose(symbol, side string, markPrice float64) (bool, string) {
	if at.config.StrategyConfig == nil {
		return false, ""
	}
	minHoldMinutes := at.config.StrategyConfig.RiskControl.MinHoldMinutes
	if minHoldMinutes <= 0 {
		return false, ""
	}

	posKey := symbol + "_" + side
	openMs, exists := at.positionFirstSeenTime[posKey]
	if !exists || openMs <= 0 {
		return false, "" // age unknown — don't block
	}
	held := time.Since(time.UnixMilli(openMs))
	if held >= time.Duration(minHoldMinutes)*time.Minute {
		return false, ""
	}

	// Hard-exit bypass: stop-loss already hit.
	if sl := at.GetRecordedStopLoss(symbol, side); sl > 0 && markPrice > 0 {
		if (side == "long" && markPrice <= sl) || (side == "short" && markPrice >= sl) {
			return false, ""
		}
	}
	return true, fmt.Sprintf("min-hold lock: held %.1fmin < %dmin and stop-loss not hit", held.Minutes(), minHoldMinutes)
}

// ============================================================================
// Position-management executors (adjust SL / partial close)
// ============================================================================

// stopMoveTightens validates an AI stop-loss move: tighten-only, never widen,
// never cross the mark (audit-proof guard for the adjust_stop_loss action).
// currentSL <= 0 means NO known stop (recorded state lost and the exchange
// holds none — the LITEUSDT 09-17 case): a correctly-sided new stop then
// ADDS protection where none exists and is allowed through; the breakeven
// gate still applies, and moveStopExchange cancels-then-places so a hidden
// stop can never duplicate.
func stopMoveTightens(side string, currentSL, newSL, markPrice float64) bool {
	if newSL <= 0 || markPrice <= 0 {
		return false
	}
	if currentSL <= 0 {
		if side == "long" {
			return newSL < markPrice
		}
		return newSL > markPrice
	}
	if side == "long" {
		return newSL > currentSL && newSL < markPrice
	}
	return newSL < currentSL && newSL > markPrice
}

// stopMoveLocksProfit enforces the breakeven-or-better rule (user directive
// 09-16, NEARUSDT case): an AI tighten may only fire once it locks in
// breakeven or better — long newSL >= entry, short newSL <= entry. A
// "tighten" that still locks a loss just squeezes the escape room into
// short-timeframe noise: the NEAR stop parked under a 5m support cluster was
// swept by a single 5m wick (2.438, wick 2.432) fifteen minutes before price
// broke the very structure high the tighten claimed to keep room for, and the
// pre-tighten stop (2.4) was never touched. Cutting risk on a LOSING position
// is the close path (early-close gate), not a loss-locking stop.
func stopMoveLocksProfit(side string, entryPrice, newSL float64) bool {
	if entryPrice <= 0 || newSL <= 0 {
		return false
	}
	if side == "long" {
		return newSL >= entryPrice
	}
	return newSL <= entryPrice
}

// stopPriceForSide returns the stop trigger of the position-side's stop
// order from the open-order list (0 when none).
func stopPriceForSide(orders []types.OpenOrder, side string) float64 {
	return protectionPrice(orders, side, "SL")
}

// exchangeStopPrice reads the position's current stop trigger from the
// exchange — the durable fallback for the in-memory recorded stop, which a
// restart wipes (pre-restart positions lose SetRecordedStopLoss state).
func (at *AutoTrader) exchangeStopPrice(symbol, side string) float64 {
	orders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return 0
	}
	return stopPriceForSide(orders, side)
}

// executeAdjustStopLossWithRecord moves one open position's exchange stop to
// the AI's new price — TIGHTEN-ONLY (guard enforced): structure-based profit
// protection the mechanical 1R/ATR ladders cannot express.
func (at *AutoTrader) executeAdjustStopLossWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	// Hands-off rule (user directive 2026-09-25, R7 fix 2026-09-26): the
	// action string carries NO side ("adjust_stop_loss"), so ownership is
	// validated against the side ACTUALLY MATCHED below — checking a
	// hardcoded "long" let a mixed AI-long/manual-short account pass the
	// long check and then modify the manual SHORT. Positions that fail the
	// tighten test are not candidates; among the remaining candidates the
	// trade must be AI-managed on EVERY matched side, else ambiguous →
	// reject (never guess by stop price).
	positions, err := at.trader.GetPositions()
	if err != nil {
		return err
	}
	var side string
	var markPrice, currentSL, entryPrice float64
	for _, pos := range positions {
		if pos["symbol"] != decision.Symbol {
			continue
		}
		pside, _ := pos["side"].(string)
		mp, _ := pos["markPrice"].(float64)
		ep, _ := pos["entryPrice"].(float64)
		sl := at.GetRecordedStopLoss(decision.Symbol, pside)
		if sl <= 0 {
			// Recorded stop lost (restart, pre-upgrade position): the
			// exchange's own stop order is the durable truth — seed it back.
			if sp := at.exchangeStopPrice(decision.Symbol, pside); sp > 0 {
				at.SetRecordedStopLoss(decision.Symbol, pside, sp)
				sl = sp
				logger.Infof("🔧 [%s] Seeded recorded SL for %s %s from exchange stop %.6g", at.name, decision.Symbol, pside, sp)
			}
		}
		if stopMoveTightens(pside, sl, decision.StopLoss, mp) {
			side, markPrice, currentSL, entryPrice = pside, mp, sl, ep
			break
		}
		// Remember the first position for an accurate error message.
		if side == "" {
			side, markPrice, currentSL, entryPrice = pside, mp, sl, ep
		}
	}
	if side == "" {
		// No position on the exchange — the normal AI-latency race, not an
		// error: the decision was made on the cycle-start context snapshot,
		// and during the 2-10min AI call the position legitimately closed
		// (SL/TP trigger, manual close). There is nothing left to protect,
		// so skip silently instead of firing the failed-execution alert
		// (user 09-14: 没有仓位就停止 stop loss 操作).
		logger.Infof("ℹ️ [%s] adjust_stop_loss skipped: %s has no open position (closed during the AI-call window — context snapshot was stale)", at.name, decision.Symbol)
		return nil
	}
	// R7 ownership: the matched side is now known — reject manual there.
	if !at.isAIManaged(decision.Symbol, side) {
		return fmt.Errorf("❌ [HANDS-OFF] %s %s was not opened by the AI — adjust_stop_loss rejected", decision.Symbol, side)
	}
	if !stopMoveTightens(side, currentSL, decision.StopLoss, markPrice) {
		return fmt.Errorf("❌ [RISK CONTROL] adjust_stop_loss %s rejected: new SL %.6g must TIGHTEN (current %.6g, mark %.6g) — never widen, never cross the mark",
			decision.Symbol, decision.StopLoss, currentSL, markPrice)
	}
	// Breakeven-or-better (user directive 09-16, NEARUSDT case): a tighten
	// that still locks a loss is rejected — with no profit there is nothing
	// to "protect", and parking the stop inside short-timeframe noise (below
	// a 5m support cluster) converts intact higher-TF theses into realized
	// losses on routine wicks. Risk reduction on a losing position belongs to
	// the close path (early-close gate), not a loss-locking stop.
	if entryPrice > 0 && !stopMoveLocksProfit(side, entryPrice, decision.StopLoss) {
		return fmt.Errorf("❌ [RISK CONTROL] adjust_stop_loss %s rejected: new SL %.6g still locks a LOSS (entry %.6g, current SL %.6g) — tighten is only allowed to breakeven or better; to cut risk on a losing position use close (early-close gate), never a short-timeframe noise stop",
			decision.Symbol, decision.StopLoss, entryPrice, currentSL)
	}
	if entryPrice <= 0 {
		logger.Infof("⚠️ [%s] adjust_stop_loss %s: entry price unavailable from exchange data — breakeven gate skipped (tighten-only still enforced)", at.name, decision.Symbol)
	}
	if err := at.moveStopExchange(decision.Symbol, side, decision.StopLoss); err != nil {
		return err
	}
	at.SetRecordedStopLoss(decision.Symbol, side, decision.StopLoss)
	logger.Infof("🔧 [%s] AI stop-loss adjusted: %s %s SL → %.6g (was %.6g, mark %.6g)", at.name, decision.Symbol, side, decision.StopLoss, currentSL, markPrice)
	notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🔧 止损调整 %s</b>\nSL %.6g → <code>%.6g</code>(AI 结构性收紧)", notify.Escape(decision.Symbol), currentSL, decision.StopLoss))
	return nil
}

// executePartialCloseWithRecord market-closes close_fraction of a position
// (user 2026-09-12: 分批止盈/减仓). Guards: cumulative partial ≤75% per
// position (the rest must run or exit via close_*); close gates (min-hold /
// early-close / breakout-hold) apply in applyHardRiskGates before this runs.
func (at *AutoTrader) executePartialCloseWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, side string) error {
	// Hands-off rule (user directive 2026-09-25): manual positions are never
	// partially closed by the program.
	if decision.Symbol != "" && !at.isAIManaged(decision.Symbol, side) {
		return fmt.Errorf("❌ [HANDS-OFF] %s was not opened by the AI — partial_close rejected", decision.Symbol)
	}
	positions, err := at.trader.GetPositions()
	if err != nil {
		return err
	}
	var qty, markPrice float64
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == side {
			qty, _ = pos["positionAmt"].(float64)
			markPrice, _ = pos["markPrice"].(float64)
			break
		}
	}
	// review 2026-10-07 B1-1: Binance shorts retain a signed quantity;
	// reduction accounting and order quantities always use its finite magnitude.
	if math.IsNaN(qty) || math.IsInf(qty, 0) {
		return fmt.Errorf("❌ %s has invalid %s position quantity: %g", decision.Symbol, side, qty)
	}
	qty = math.Abs(qty)
	if qty <= 0 {
		return fmt.Errorf("❌ %s has no open %s position", decision.Symbol, side)
	}
	posKey := decision.Symbol + "_" + side
	// partialTrimmed is shared with the drawdown monitor goroutine (its
	// ClearPeakPnLCache deletes under tpTrimMutex) — this cycle-goroutine
	// writer MUST take the same lock or the concurrent map read/write is a
	// runtime-fatal crash (round-4 review R4-4).
	at.tpTrimMutex.Lock()
	if at.partialTrimmed == nil {
		at.partialTrimmed = make(map[string]float64)
	}
	already := at.partialTrimmed[posKey]
	at.tpTrimMutex.Unlock()
	// EntryQuantity persists across restarts and includes every reduction
	// source (R-lock, ROE ladder, structure TP, and AI partial close). It is
	// authoritative over the in-memory fallback counter.
	if at.store != nil {
		if p, err := at.store.Position().GetOpenPositionBySymbol(at.id, decision.Symbol, strings.ToUpper(side)); err == nil && p != nil && p.EntryQuantity > 0 {
			if derived := 1 - qty/p.EntryQuantity; derived > already {
				already = derived
			}
		}
	}
	projected := cumulativeReductionAfter(already, decision.CloseFraction)
	if projected > 0.75+1e-9 {
		return fmt.Errorf("❌ [RISK CONTROL] partial_close %s rejected: total reduction would rise from %.0f%% to %.0f%% of original size (>75%%) — use close_* for a full exit",
			decision.Symbol, already*100, projected*100)
	}
	trimQty := qty * decision.CloseFraction
	minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
	if minSize <= 0 {
		minSize = kernel.MinPositionSizeDefaultUSDT
	}
	if markPrice > 0 && (qty-trimQty)*markPrice < minSize {
		return fmt.Errorf("❌ [RISK CONTROL] partial_close %s rejected: remainder %.2f USDT would be below min position size %.2f USDT; use close_*",
			decision.Symbol, (qty-trimQty)*markPrice, minSize)
	}
	// The final slice auto-closes the row in the fill sync ('sync' stamp) —
	// mark the intent so the classifier attributes it to the AI, not "external".
	at.markCloseIntent(decision.Symbol, side, "ai_close")
	if _, err := at.reducePosition(decision.Symbol, side, trimQty); err != nil {
		return err
	}
	at.tpTrimMutex.Lock()
	at.partialTrimmed[posKey] = projected
	at.tpTrimMutex.Unlock()
	actionRecord.Quantity = trimQty
	logger.Infof("🎯 [%s] AI partial close: %s %s %.0f%% of remainder (%.6g) — %.0f%% of original reduced", at.name, decision.Symbol, side, decision.CloseFraction*100, trimQty, projected*100)
	notify.Notify("ORDER", at.name, fmt.Sprintf("<b>🎯 部分平仓 %s</b>\n<i>%s 平当前仓位 %.0f%%(初始仓位累计已减 %.0f%%),剩余仓位继续持有</i>", notify.Escape(decision.Symbol), side, decision.CloseFraction*100, projected*100))
	return nil
}

// cumulativeReductionAfter converts a fraction of the CURRENT remainder into
// a fraction of the ORIGINAL entry quantity. Example: 50% already reduced,
// then closing 50% of the remainder reaches 75%, not 100%.
func cumulativeReductionAfter(alreadyReduced, fractionOfRemainder float64) float64 {
	return alreadyReduced + (1-alreadyReduced)*fractionOfRemainder
}

// ============================================================================
// Protection watchdog
// ============================================================================

// missingProtection inspects a position's open orders and reports which
// protective legs are absent. STOP* matches stop-loss algos, TAKE_PROFIT*
// matches TP algos; a resting LIMIT entry on the same symbol is noise here.
func missingProtection(orders []types.OpenOrder, positionSide string) (needSL, needTP bool) {
	needSL, needTP = true, true
	for _, o := range orders {
		// Hedge-mode-safe (2026-09-25 P0): match the POSITION side — the
		// opposite side's stop says nothing about this one's. One-way mode
		// books every order as BOTH: accept it as this side's protection.
		ps := strings.ToUpper(o.PositionSide)
		if positionSide != "" && ps != "" && ps != positionSide && ps != "BOTH" {
			continue
		}
		t := strings.ToUpper(o.Type)
		if strings.Contains(t, "STOP") {
			needSL = false
		}
		if strings.Contains(t, "TAKE_PROFIT") {
			needTP = false
		}
	}
	return needSL, needTP
}

// exchangeStopPriceSeen — kept for symmetry with stopPriceForSide above.

// processProtectionWatchdog runs once per decision cycle over all open
// positions: any position whose SL/TP order is missing on the exchange gets
// it re-placed at the RECORDED plan price (user request 2026-09-12). Missing
// legs happen via manual cancels, exchange hiccups, or pre-upgrade positions.
// Guards: the TP is not re-placed once the TP-runner converted it into the
// trend-run (the trail owns the exit); wrong-side plan prices are logged
// instead of placed; every check fails open. The AI has no place-SL/TP
// action — without this watchdog a naked position stays naked.
// orphanProtectiveOrder reports whether one protective order's position is
// GONE: hedge mode matches the exact side; one-way mode books orders as
// BOTH — alive if ANY position exists on the symbol. LIMIT entries are
// never protective and are excluded by the caller's type filter.
func orphanProtectiveOrder(o types.OpenOrder, live map[string]bool) bool {
	ps := strings.ToUpper(o.PositionSide)
	if ps == "LONG" || ps == "SHORT" {
		if !orderClosesSide(o, ps) {
			return false
		}
	} else if !o.ReduceOnly && !o.ClosePosition {
		return false
	}
	switch ps {
	case "LONG":
		return !live[strings.ToUpper(o.Symbol)+"_long"]
	case "SHORT":
		return !live[strings.ToUpper(o.Symbol)+"_short"]
	default: // BOTH / "" — one-way mode
		return !live[strings.ToUpper(o.Symbol)+"_long"] && !live[strings.ToUpper(o.Symbol)+"_short"]
	}
}

// sweepOrphanedProtection cancels SL/TP orders whose position has vanished
// (user directive 2026-09-26): SL/TP triggers and external closes can leave
// the OTHER leg resting forever — it would trigger on a future re-opened
// position at a stale price. Scoped to symbols the system has tracked (AI
// marks, pending entries, first-seen keys); candidates with a live position
// on the matching side are skipped.
func (at *AutoTrader) sweepOrphanedProtection(positions []map[string]interface{}, prefetched map[string][]types.OpenOrder) {
	if at.config.StrategyConfig != nil && at.config.StrategyConfig.StrategyType == "grid_trading" {
		return // grid keeps its own order books
	}
	live := make(map[string]bool, len(positions))
	symbols := make(map[string]bool)
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		if symbol == "" || side == "" {
			continue
		}
		symbol = strings.ToUpper(symbol)
		symbols[symbol] = true
		live[symbol+"_"+strings.ToLower(side)] = true
	}
	// Candidate symbols: everything the system has ever tracked protection
	// for (AI marks, first-seen keys, pending entries) plus current
	// position symbols (mixed partial-mismatch cases).
	for _, m := range func() []store.AIManagedPosition {
		if at.store == nil {
			return nil
		}
		ms, _ := at.store.AIManaged().List(at.id)
		return ms
	}() {
		symbols[strings.ToUpper(m.Symbol)] = true
	}
	for key := range at.positionFirstSeenTime {
		parts := strings.SplitN(key, "_", 2)
		if len(parts) == 2 {
			symbols[strings.ToUpper(parts[0])] = true
		}
	}
	for _, key := range at.snapshotPendingKeys() {
		// key form: symbol_side
		parts := strings.SplitN(key, "_", 2)
		if len(parts) == 2 {
			symbols[strings.ToUpper(parts[0])] = true
		}
	}

	canceller, canSide := at.trader.(interface {
		CancelProtectiveOrdersForSide(symbol, positionSide string) error
	})
	for symbol := range symbols {
		orders, cached := prefetched[symbol]
		if !cached {
			orders, _ = at.trader.GetOpenOrders(symbol)
		}
		if len(orders) == 0 {
			continue
		}
		for _, o := range orders {
			t := strings.ToUpper(o.Type)
			if !strings.Contains(t, "STOP") && !strings.Contains(t, "TAKE_PROFIT") {
				continue // LIMIT entries etc. are not protection
			}
			if !orphanProtectiveOrder(o, live) {
				continue
			}
			ps := strings.ToUpper(o.PositionSide)
			if canSide {
				if err := canceller.CancelProtectiveOrdersForSide(o.Symbol, ps); err != nil {
					logger.Infof("⚠️ [%s] orphan sweep: cancel %s %s failed: %v", at.name, o.Symbol, ps, err)
					continue
				}
			} else {
				// Adapters without the side-scoped cancel: cancel only the
				// SL half is wrong (TP would linger) — skip rather than
				// half-clean; Binance (the live exchange) has the capability.
				continue
			}
			logger.Infof("🧹 [%s] orphan sweep: cancelled %s protection for %s %s — no matching position", at.name, o.Type, o.Symbol, o.PositionSide)
		}
	}
}

// watchdogTPQty sizes the watchdog's TP repair: a split-TP position's repair
// must re-place the FRACTION, not the full position quantity — the old full-
// qty repair silently upgraded a trend runner to "close everything at TP"
// (2026-10-03 review P2).
func (at *AutoTrader) watchdogTPQty(symbol, side string) float64 {
	qty, qtyOK := at.positionQty(symbol, side)
	if !qtyOK {
		return 0
	}
	if mode := at.ExitModeFor(symbol, side); mode != "" {
		if q := qty * tpFractionForMode(mode, at.effectiveTPCloseFraction()); q > 0 {
			return q
		}
	}
	return qty
}

func (at *AutoTrader) processProtectionWatchdog() {
	if at.config.StrategyConfig == nil {
		return
	}
	at.invalidateExecutionCache()
	positions, err := at.trader.GetPositions()
	if err != nil {
		at.setProtectionFault("account", "position snapshot unavailable")
		return
	}
	live := map[string]bool{}
	for _, p := range positions {
		symbol, _ := p["symbol"].(string)
		side, _ := p["side"].(string)
		qty, ok := p["positionAmt"].(float64)
		if symbol == "" || (side != "long" && side != "short") || !ok || math.IsNaN(qty) || math.IsInf(qty, 0) {
			at.setProtectionFault("account", "malformed position snapshot")
			return
		}
		if qty != 0 {
			live[symbol+"_"+side] = true
		}
	}
	at.clearProtectionFaults(live)
	// P2-7: one GetOpenOrders per unique SYMBOL for the whole pass — hedge
	// mode produces LONG+SHORT rows for the same symbol and the old per-row
	// call doubled the request on exactly those. Failed fetches are cached
	// per symbol so the per-position error handling below stays intact.
	ordersBySymbol := make(map[string][]types.OpenOrder, len(positions))
	orderErrs := make(map[string]error, len(positions))
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		if symbol == "" || orderErrs[symbol] != nil {
			continue
		}
		if _, done := ordersBySymbol[symbol]; done {
			continue
		}
		orders, err := at.trader.GetOpenOrders(symbol)
		if err != nil {
			orderErrs[symbol] = err
			continue
		}
		ordersBySymbol[symbol] = orders
	}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		mark, _ := pos["markPrice"].(float64)
		qty, _ := pos["positionAmt"].(float64)
		// Binance keeps SHORT positionAmt NEGATIVE (futures_positions.go) —
		// the drawdown monitor at :190 applies the same abs normalization.
		// Before it, the `qty <= 0` guard below classified EVERY AI-managed
		// short as "quantity unavailable": a permanent per-symbol protection
		// fault that blocked ALL new account risk and skipped the position's
		// own stop repair / crossed-stop exit (review 2026-10-06 P0-1,
		// reproduced by a short mirror test; F05/F08 short paths depend on
		// this normalization too).
		qty = math.Abs(qty)
		if symbol == "" || side == "" || !at.isAIManaged(symbol, side) {
			continue
		}
		key := symbol + "_" + side
		if mark <= 0 || qty == 0 || math.IsNaN(mark) || math.IsInf(mark, 0) || math.IsNaN(qty) || math.IsInf(qty, 0) {
			at.setProtectionFault(key, "live mark or quantity unavailable")
			continue
		}
		orders, ordersErr := ordersBySymbol[symbol], orderErrs[symbol]
		if ordersErr != nil {
			at.protectionFailure(symbol, side, "protective orders unavailable")
			at.alertUnprotectedPosition(symbol, side, ordersErr.Error())
			continue
		}
		positionSide := strings.ToUpper(side)
		sl := at.GetRecordedStopLoss(symbol, side)
		seenSL := protectionPrice(orders, positionSide, "SL")
		if seenSL > 0 && (sl <= 0 || (side == "long" && seenSL > sl) || (side == "short" && seenSL < sl)) {
			sl = seenSL
			at.SetRecordedStopLoss(symbol, side, sl)
			at.SetInitialStopLoss(symbol, side, sl)
		}
		tp := at.getOpenTakeProfit(symbol, side)
		if tp <= 0 {
			tp = protectionPrice(orders, positionSide, "TP")
			if tp > 0 {
				at.recordOpenTakeProfit(symbol, side, tp)
			}
		}
		if sl > 0 && ((side == "long" && mark <= sl) || (side == "short" && mark >= sl)) {
			at.setProtectionFault(key, "stop crossed; emergency exit awaiting reconciliation")
			at.markCloseIntent(symbol, side, "stop_loss_recovery")
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				at.alertUnprotectedPosition(symbol, side, "stop crossed and emergency exit failed: "+err.Error())
			}
			continue
		}
		if sl <= 0 {
			at.placeComputedProtection(symbol, side, positionSide, mark)
			sl = at.GetRecordedStopLoss(symbol, side)
			tp = at.getOpenTakeProfit(symbol, side)
			orders, err = at.trader.GetOpenOrders(symbol)
			if err != nil || sl <= 0 {
				at.protectionFailure(symbol, side, "computed stop recovery failed")
				at.alertUnprotectedPosition(symbol, side, "computed stop recovery failed")
				continue
			}
		}
		tpQty := qty
		if mode := at.ExitModeFor(symbol, side); mode != "" {
			tpQty *= tpFractionForMode(mode, at.effectiveTPCloseFraction())
		}
		wantSL := !enoughProtection(orders, positionSide, "SL", sl, qty)
		wantTP := !at.tpRunnerDone(key) && tp > 0 &&
			((side == "long" && tp > mark) || (side == "short" && tp < mark)) &&
			!enoughProtection(orders, positionSide, "TP", tp, tpQty)
		var slErr error
		if wantSL {
			slErr = ensureProtectiveCoverage(at.trader, symbol, positionSide, "SL", sl, qty)
		}
		if wantTP {
			if err := ensureProtectiveCoverage(at.trader, symbol, positionSide, "TP", tp, tpQty); err != nil {
				logger.Warnf("TP recovery %s %s: %v", symbol, side, err)
			}
		}
		if wantSL || wantTP {
			verifiedSL, _ := at.verifyProtectiveLegs(&kernel.Decision{Symbol: symbol, StopLoss: sl, TakeProfit: tp}, positionSide, wantSL, wantTP, qty, tpQty)
			if slErr == nil {
				slErr = verifiedSL
			}
		}
		if slErr != nil {
			at.protectionFailure(symbol, side, "SL coverage recovery failed: "+slErr.Error())
			at.alertUnprotectedPosition(symbol, side, slErr.Error())
		} else {
			at.protectionVerified(symbol, side)
		}
	}
	at.sweepOrphanedProtection(positions, ordersBySymbol)
}

// placeComputedProtection: the naked-position fallback (09-19 user directive
// — PONS sat unprotected for 8h because the old path could only alert). SL =
// mark ± 1.5×ATR(1h), TP = mark ∓ 2× that distance (1:2 RR), prices rounded
// to the symbol tick when the concrete trader exposes precision. Both legs
// placed, the computed stop recorded (memory + write-once initial anchor) so
// every downstream consumer treats it as the plan. Returns true when BOTH
// legs are on the exchange.
func (at *AutoTrader) placeComputedProtection(symbol, side, positionSide string, markPrice float64) bool {
	data, err := at.getMarketTimeframes(symbol, []string{"1h"}, "1h", 99)
	if err != nil || data == nil {
		return false
	}
	tf := data.TimeframeData["1h"]
	if tf == nil || tf.ATR14 <= 0 {
		return false
	}
	atrPct := tf.ATR14 / markPrice * 100
	sl, tp, ok := computedProtectionLevels(side, markPrice, tf.ATR14)
	if !ok {
		return false
	}
	if pp, ok := at.trader.(interface {
		GetSymbolPricePrecision(string) (int, error)
	}); ok {
		if prec, err := pp.GetSymbolPricePrecision(symbol); err == nil && prec >= 0 {
			scale := math.Pow10(prec)
			sl = math.Round(sl*scale) / scale
			tp = math.Round(tp*scale) / scale
		}
	}

	if !at.reconcileComputedProtection(symbol, side, sl, tp) {
		return false
	}
	logger.Infof("🛡️ [%s] Protection watchdog: %s naked — computed protection placed: SL %.6g / TP %.6g (1:2 RR, SL=1.5×ATR(1h) %.2f%% from mark %.6g)",
		at.name, symbol, sl, tp, atrPct, markPrice)
	notify.Notify("ORDER", at.name, fmt.Sprintf(
		"<b>🛡️ 裸仓自动保护 %s (%s)</b>\n<i>无挂单且无记录止损,已按最新价程序重算并挂出: SL %.6g / TP %.6g (1:2 RR, SL=1.5×ATR(1h)=%.2f%%)</i>",
		notify.Escape(symbol), strings.ToUpper(side[:1])+side[1:], sl, tp, atrPct))
	return true
}

// computedProtectionLevels derives the naked-position fallback pair:
// SL = mark ∓ 1.5×ATR(1h), TP = mark ∓ 2× that distance (1:2 RR). ok=false
// when the inputs are unusable (bad side, non-positive mark/ATR) or the
// resulting prices sit on the wrong side of mark — extracted as a pure
// function so the watchdog's core math is unit-testable (round-4 review
// R4-14).
func computedProtectionLevels(side string, markPrice, atrAbs float64) (sl, tp float64, ok bool) {
	if markPrice <= 0 || atrAbs <= 0 {
		return 0, 0, false
	}
	dist := 1.5 * atrAbs
	switch side {
	case "long":
		sl, tp = markPrice-dist, markPrice+2*dist
	case "short":
		sl, tp = markPrice+dist, markPrice-2*dist
	default:
		return 0, 0, false
	}
	if sl <= 0 || tp <= 0 {
		return 0, 0, false
	}
	if side == "long" && (sl >= markPrice || tp <= markPrice) {
		return 0, 0, false
	}
	if side == "short" && (sl <= markPrice || tp >= markPrice) {
		return 0, 0, false
	}
	return sl, tp, true
}

// alertUnprotectedPosition pushes a Telegram alert when the watchdog finds a
// position with NO usable stop-loss protection (missing order, missing
// recorded price, wrong-side price, or a failed re-place) — the SL leg is the
// one leg that must never silently stay naked. Deduped like the other gate
// alerts (gateNotifyRecord) so a stuck position doesn't spam every cycle.
func (at *AutoTrader) alertUnprotectedPosition(symbol, side, reason string) {
	streak, push := at.gateNotifyRecord("unprotected-sl:"+symbol+":"+side, time.Now())
	if !push {
		return
	}
	notify.Notify("ALERT", at.name, fmt.Sprintf(
		"<b>🚨 仓位无止损保护 %s (%s)</b>\n%s\n\n<i>看门狗无法自动补挂,请人工核查该仓位并手动设置止损(streak %d)</i>",
		notify.Escape(symbol), strings.ToUpper(side[:1])+side[1:], notify.Escape(reason), streak))
}

// ============================================================================
// Recorded stop-loss state (drives the min-hold/early-close hard-exit bypass)
// ============================================================================

// SetRecordedStopLoss records the stop-loss price of a freshly opened position
// (used by the min-hold gate to allow hard-exit closes).
func (at *AutoTrader) SetRecordedStopLoss(symbol, side string, price float64) {
	at.positionStopLossMutex.Lock()
	defer at.positionStopLossMutex.Unlock()
	at.positionStopLoss[symbol+"_"+side] = price
}

// GetRecordedStopLoss returns the recorded stop-loss for a position.
func (at *AutoTrader) GetRecordedStopLoss(symbol, side string) float64 {
	at.positionStopLossMutex.RLock()
	defer at.positionStopLossMutex.RUnlock()
	return at.positionStopLoss[symbol+"_"+side]
}

// ClearRecordedStopLoss drops the recorded stop-loss after the position is closed.
func (at *AutoTrader) ClearRecordedStopLoss(symbol, side string) {
	at.positionStopLossMutex.Lock()
	defer at.positionStopLossMutex.Unlock()
	delete(at.positionStopLoss, symbol+"_"+side)
}

// SetInitialStopLoss freezes the OPENING stop of a position — the anchor the
// 1R profit lock measures risk against. Write-once: AI tighten, trailing and
// breakeven move the LIVE stop (SetRecordedStopLoss) and must not pull the R
// bar along (ONDOUSDT 2026-09-17: two tightens compressed the R distance
// 2.14% → 0.26%, trimming 50% at +0.49% "1.89R"). The value is also stamped
// onto the OPEN position row (set-if-empty) so the anchor survives restarts.
func (at *AutoTrader) SetInitialStopLoss(symbol, side string, price float64) {
	if price <= 0 {
		return
	}
	key := symbol + "_" + side
	at.positionStopLossMutex.Lock()
	if at.positionInitialStopLoss[key] <= 0 {
		at.positionInitialStopLoss[key] = price
	}
	at.positionStopLossMutex.Unlock()
	if at.store != nil {
		if _, err := at.store.Position().SetInitialStopLossIfEmpty(at.id, symbol, side, price); err != nil {
			logger.Infof("⚠️ [%s] initial-stop persist failed for %s %s: %v", at.name, symbol, side, err)
		}
	}
}

// GetInitialStopLoss returns the frozen opening-risk stop for a position.
func (at *AutoTrader) GetInitialStopLoss(symbol, side string) float64 {
	at.positionStopLossMutex.RLock()
	defer at.positionStopLossMutex.RUnlock()
	return at.positionInitialStopLoss[symbol+"_"+side]
}

// ClearInitialStopLoss drops the frozen opening-risk stop after close.
func (at *AutoTrader) ClearInitialStopLoss(symbol, side string) {
	at.positionStopLossMutex.Lock()
	defer at.positionStopLossMutex.Unlock()
	delete(at.positionInitialStopLoss, symbol+"_"+side)
}

// SetExitMode records the position's exit template (user menu directive
// 09-29): "trend" | "range" | "quick" — the ladder, the split-TP fraction and
// the quick-mode time stop all read it. Memory is the fast path; the OPEN
// position row (write-once stamp) is the restart authority.
func (at *AutoTrader) SetExitMode(symbol, side, mode string) {
	if mode == "" {
		mode = kernel.ExitModeTrend
	}
	key := symbol + "_" + side
	at.positionStopLossMutex.Lock()
	at.positionExitMode[key] = mode
	at.positionStopLossMutex.Unlock()
	if at.store != nil {
		if err := at.store.Position().SetExitModeIfEmpty(at.id, symbol, side, mode); err != nil {
			logger.Infof("⚠️ [%s] exit-mode persist failed for %s %s: %v", at.name, symbol, side, err)
		}
	}
}

// ExitModeFor resolves the position's exit template: memory → the persisted
// row → "trend" (the legacy default, also what pre-menu positions get).
func (at *AutoTrader) ExitModeFor(symbol, side string) string {
	key := symbol + "_" + side
	at.positionStopLossMutex.RLock()
	if m := at.positionExitMode[key]; m != "" {
		at.positionStopLossMutex.RUnlock()
		return m
	}
	at.positionStopLossMutex.RUnlock()
	if at.store != nil {
		if pos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, strings.ToUpper(side)); err == nil && pos != nil && pos.ExitMode != "" {
			at.positionStopLossMutex.Lock()
			at.positionExitMode[key] = pos.ExitMode
			at.positionStopLossMutex.Unlock()
			return pos.ExitMode
		}
	}
	return kernel.ExitModeTrend
}

// ClearExitMode drops the in-memory exit template after the position closes
// (the row dies with the position lifecycle).
func (at *AutoTrader) ClearExitMode(symbol, side string) {
	at.positionStopLossMutex.Lock()
	defer at.positionStopLossMutex.Unlock()
	delete(at.positionExitMode, symbol+"_"+side)
}

// finestSubHourTrend shares kernel micro timing for gates and entry-path tags.
func finestSubHourTrend(data *market.Data) (string, string) {
	return kernel.ExecutionMicroTrend(data, time.Now())
}

// ============================================================================
// Hard Risk Gates (CODE ENFORCED, strategy risk_control driven)
// ============================================================================

// accountRiskExposureBlocks reports whether opening d would push total open
// stop-risk past the account cap or this symbol's single-trade risk budget.
// Existing risk is
// Σ|qty|×|entry − exchange SL| from the cycle's position snapshot;
// unprotected positions are worst-cased at the stop-band cap
// (kernel.UnprotectedStopWorstCasePct). The new trade's risk uses the
// gate-priced entry (limit anchor when live) and the decision's stop.
func (at *AutoTrader) accountRiskExposureBlocks(d *kernel.Decision, entryPx float64, ctx *kernel.Context) (bool, string, float64) {
	rc := at.config.StrategyConfig.RiskControl
	capPct := rc.EffectiveMaxAccountRiskPct()
	symbolCapPct := rc.RiskPerTradePct
	if symbolCapPct <= 0 {
		symbolCapPct = 1.5
	}
	equity := ctx.Account.TotalEquity
	if equity <= 0 && at.exchange == "binance" {
		return true, "cannot verify Binance symbol stop-risk without positive account equity", 0
	}
	if equity <= 0 || entryPx <= 0 || d.StopLoss <= 0 {
		return false, "", 0 // unpriceable → other gates (mandatory SL) handle it
	}
	totalRisk := 0.0
	symbolRisk := 0.0
	longRisk := 0.0
	shortRisk := 0.0
	unprotected := 0
	for _, p := range ctx.Positions {
		if p.Quantity <= 0 || p.EntryPrice <= 0 {
			continue
		}
		stopDistPct := kernel.UnprotectedStopWorstCasePct
		if p.StopLossPrice > 0 {
			stopDistPct = math.Abs(p.EntryPrice-p.StopLossPrice) / p.EntryPrice * 100
		} else {
			unprotected++
		}
		risk := p.Quantity * p.EntryPrice * stopDistPct / 100
		totalRisk += risk
		if strings.EqualFold(p.Side, "short") {
			shortRisk += risk
		} else {
			longRisk += risk
		}
		if market.Normalize(p.Symbol) == market.Normalize(d.Symbol) {
			symbolRisk += risk
		}
	}
	// Both sides' resting limits reserve risk. Exclude only the same-side
	// order that this decision replaces; opposite-side exposure still counts.
	side := "long"
	if strings.Contains(d.Action, "short") {
		side = "short"
	}
	excludeKey := pendingEntryKey(d.Symbol, side)
	for key, pe := range at.accountPendingEntries() {
		if key == "unknown" {
			return true, "account pending reservations unknown", 0
		}
		if key == excludeKey || pe == nil || pe.Price <= 0 || pe.StopLoss <= 0 {
			continue
		}
		remaining := math.Max(0, pe.Quantity-pe.ProtectedQty)
		risk := remaining * math.Abs(pe.Price-pe.StopLoss)
		totalRisk += risk
		if strings.EqualFold(pe.Side, "short") {
			shortRisk += risk
		} else {
			longRisk += risk
		}
		if market.Normalize(pe.Symbol) == market.Normalize(d.Symbol) {
			symbolRisk += risk
		}
	}
	// R8 (2026-09-26 review): the candidate is priced ALONE — the caller
	// books candidateRisk only when the candidate PASSES, so the reservation
	// accumulates linearly (20→40→60), never compounding (the old shape fed
	// the reservation into the candidate AND re-booked it: 20→60→140), never
	// charges risk for candidates a later gate rejects.
	candidateRisk := d.PositionSizeUSD * math.Abs(entryPx-d.StopLoss) / entryPx
	// 2026-09-26 NILUSDT: the gate ran on the RAW AI proposal while the
	// executor's clampSizeToRisk would have downsized it to the risk budget —
	// a 2.002% proposal got hard-rejected by a 2.00% cap it would never
	// actually run at. Evaluate the CLAMPED candidate (clamp-only sizing:
	// max notional = equity×riskPct ÷ dist ⇒ clamped risk = equity×riskPct),
	// so oversized proposals shrink into compliance instead of dying.
	riskBudgetPct := rc.RiskPerTradePct
	if riskBudgetPct <= 0 {
		riskBudgetPct = 1.5
	}
	if maxRisk := equity * riskBudgetPct / 100; candidateRisk > maxRisk {
		candidateRisk = maxRisk
	}
	reserved := at.cycleRiskReservedUSD
	symbolReserved := at.cycleSymbolRiskReservedUSD[market.Normalize(d.Symbol)]
	existingPct := totalRisk / equity * 100
	candidatePct := candidateRisk / equity * 100
	reservedPct := reserved / equity * 100
	if capPct > 0 && existingPct+candidatePct+reservedPct > capPct {
		return true, fmt.Sprintf("open stop-risk would exceed max_account_risk_pct %.1f%% of equity %.2f: existing %.2f%% + candidate %.2f%% + reserved-this-cycle %.2f%%%s",
			capPct, equity, existingPct, candidatePct, reservedPct, unprotectedSuffix(unprotected)), candidateRisk
	}
	// Gross risk limits the size of a full stop-out; net directional risk
	// separately limits correlated one-way concentration. Opposite sides are
	// allowed to reduce NET concentration, but never reduce the gross cap.
	projectedLong := longRisk + at.cycleLongRiskReservedUSD
	projectedShort := shortRisk + at.cycleShortRiskReservedUSD
	if side == "short" {
		projectedShort += candidateRisk
	} else {
		projectedLong += candidateRisk
	}
	if netCapPct := rc.EffectiveMaxNetDirectionalRiskPct(); netCapPct > 0 {
		netPct := math.Abs(projectedLong-projectedShort) / equity * 100
		if netPct > netCapPct+1e-9 {
			return true, fmt.Sprintf("open would exceed max_net_directional_risk_pct %.1f%% of equity %.2f: projected long %.2f USDT - short %.2f USDT = %.2f%% net",
				netCapPct, equity, projectedLong, projectedShort, netPct), candidateRisk
		}
	}
	if at.exchange == "binance" && (symbolRisk+candidateRisk+symbolReserved)/equity*100 > symbolCapPct+1e-9 {
		return true, fmt.Sprintf("%s gross long+short stop-risk would exceed %.2f%% of equity %.2f: existing %.2f + candidate %.2f + reserved-this-cycle %.2f USDT",
			d.Symbol, symbolCapPct, equity, symbolRisk, candidateRisk, symbolReserved), candidateRisk
	}
	return false, "", candidateRisk
}

func unprotectedSuffix(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d unprotected position(s) worst-cased at %.0f%% stop)", n, kernel.UnprotectedStopWorstCasePct)
}

// dailyBaselineRecord is the process-shared day anchor for one account.
type dailyBaselineRecord struct {
	day    string
	equity float64
}

// The day-start equity anchor is shared per ACCOUNT across AutoTrader
// instances in this process (F8, 2026-10-01 review: the anchor lived only on
// the instance, so a strategy save — destroy + reload — re-anchored at the
// CURRENT equity and silently cleared an active daily-loss halt). Keyed by
// exchange account; anonymous keys (bare unit-test instances) share one
// bucket — tests anchoring their own baseline must call
// ResetDailyBaselineRegistryForTest.
var (
	dailyBaselineRegMu sync.Mutex
	dailyBaselineReg   = map[string]dailyBaselineRecord{}
)

// ResetDailyBaselineRegistryForTest clears the process-shared day anchors.
func ResetDailyBaselineRegistryForTest() {
	dailyBaselineRegMu.Lock()
	dailyBaselineReg = map[string]dailyBaselineRecord{}
	dailyBaselineRegMu.Unlock()
}

// dailyBaselineKey identifies the ACCOUNT the halt governs — the exchange
// account when known, else the trader identity, else the anonymous bucket.
func (at *AutoTrader) dailyBaselineKey() string {
	if at.exchangeID != "" {
		return at.exchangeID
	}
	return at.userID + ":" + at.id
}

// inheritDailyBaseline looks up today's anchor in the process registry, then
// the durable store. ok=false → no anchor exists yet today.
func (at *AutoTrader) inheritDailyBaseline(day string) (float64, bool) {
	key := at.dailyBaselineKey()
	dailyBaselineRegMu.Lock()
	rec, ok := dailyBaselineReg[key]
	dailyBaselineRegMu.Unlock()
	if ok && rec.day == day && rec.equity > 0 {
		return rec.equity, true
	}
	if at.store != nil {
		if baseline, found, err := at.store.RiskState().DayBaseline(key, day); err == nil && found && baseline > 0 {
			return baseline, true
		}
	}
	return 0, false
}

// anchorDailyBaseline pins the daily-loss-halt baseline ONCE per UTC day,
// called every cycle right after the equity snapshot (09-28 review P2: the
// anchor used to be captured lazily inside dailyLossHaltBlocks, whose only
// call site is the open_ branch — losses accrued between midnight and the
// day's first open DECISION never made it into the baseline, letting the
// account lose ~1.6× the configured daily cap before the halt fired).
//
// F8 (2026-10-01 review): FIRST anchor per (account, UTC day) wins — shared
// in-process via the registry, durable across restarts via the
// risk_baselines store. A reloaded trader inherits today's anchor instead of
// re-anchoring at the post-loss equity.
func (at *AutoTrader) anchorDailyBaseline(equity float64) {
	if equity <= 0 {
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	key := at.dailyBaselineKey()

	// 2026-10-03 review: consult the DURABLE layer BEFORE publishing to the
	// registry — the old order seeded the registry with the new equity and
	// corrected it after the store read, letting a same-account peer inherit
	// the non-durable anchor for one pass (and leaving a crash window where
	// the registry said "anchored" but the store did not).
	if at.store != nil {
		// review 2026-10-07 B2-D: only a verified durable anchor may be published.
		if anchored, err := at.store.RiskState().AnchorDayBaseline(key, today, equity); err == nil && anchored > 0 {
			equity = anchored
		} else {
			logger.Warnf("⚠️ [%s] day-anchor unavailable: %v — new risk blocked until a verified baseline is available", at.name, err)
			return
		}
	}
	dailyBaselineRegMu.Lock()
	if rec, ok := dailyBaselineReg[key]; ok && rec.day == today && rec.equity > 0 {
		// FIRST anchor for the day wins — a later call NEVER moves the
		// baseline, in either direction (that is the entire halt contract,
		// and the reload-survival tests pin it).
		equity = rec.equity
	}
	dailyBaselineReg[key] = dailyBaselineRecord{day: today, equity: equity}
	dailyBaselineRegMu.Unlock()

	at.dayStartDay, at.dayStartEquity = today, equity
}

// dailyLossHaltBlocks reports the halt reason when equity has retraced ≥
// daily_max_loss_pct from the first equity seen this UTC day. Empty string =
// no halt. The baseline is anchored per-cycle via anchorDailyBaseline; the
// lazy re-anchor here survives only as a fallback for a gate call before the
// first snapshot of the day —
// F8: it now consults the shared/durable anchors FIRST so a reloaded
// instance inherits an active halt instead of clearing it.
func (at *AutoTrader) dailyLossHaltBlocks(rc store.RiskControlConfig, equity float64) string {
	capPct := rc.EffectiveDailyMaxLossPct()
	if capPct <= 0 || equity <= 0 {
		return ""
	}
	today := time.Now().UTC().Format("2006-01-02")
	if at.dayStartDay != today || at.dayStartEquity <= 0 {
		if baseline, ok := at.inheritDailyBaseline(today); ok {
			at.dayStartDay, at.dayStartEquity = today, baseline
		} else {
			at.anchorDailyBaseline(equity)
		}
	}
	// review 2026-10-07 B2-D: use the existing open-only halt gate while the anchor is unknown.
	if at.dayStartDay != today || at.dayStartEquity <= 0 {
		return "day-start equity baseline unknown — opens halted until a verified baseline is available"
	}
	lossPct := (at.dayStartEquity - equity) / at.dayStartEquity * 100
	if lossPct >= capPct {
		return fmt.Sprintf("equity %.2f is %.2f%% below day-start %.2f (≥ %.1f%%) — opens halted until next UTC day",
			equity, lossPct, at.dayStartEquity, capPct)
	}
	return ""
}

// absoluteBanCode reports whether a kernel ban code has NO exception path on
// the MARKET entry style either — the documented absolute contracts. Codes
// with sanctioned exception/mirror paths are deliberately NOT listed here for
// market opens: EXTENDED_PUMP_UNCONFIRMED → marketExceptionGate, MICRO_TREND_*
// → execution timing re-derivation, RR_MAX_*/MIN_SIZE_DEAD_ZONE →
// validateOpenRisk / min-size re-check, STOP_PLAN_* → designed ATR-band
// fallback, LIMIT_ANCHOR_SUPPRESSED → market-anchored fail-closed at the
// executor, VENDOR_DIVERGENCE* → its dedicated block below. On the LIMIT
// style any disallowed direction blocks regardless of code (resting-limit
// entries have no exception path at all).
//
// 2026-10-01 long-side discipline (user design): EMA20_STRETCH_* (chase cap
// vs the 4h EMA20 — the retest-anchor path is the kernel-side exemption, the
// code never coexists with an active pullback anchor) and BTC_4H_DOWNTREND /
// BTC_WEAK_LONG_* (market-level BTC filter) enter the absolute list — chasing
// stretched laggards has no sanctioned market path.
func absoluteBanCode(code string) bool {
	switch code {
	case "DATA_INSUFFICIENT", "POOR_HISTORY", "BSTOCK_DAILY_DATA_UNAVAILABLE",
		"LOSS_STREAK_BANNED", "STOCK_WEEKEND", "BTC_4H_DOWNTREND",
		"BTC_4H_STRONGBULL", "SHORT_TOP_CONFIRM_MISSING", "BTC_REGIME_UNKNOWN", "REGIME_LINE_BROKEN":
		return true
	}
	return strings.HasPrefix(code, "NEG_EDGE_") || strings.HasPrefix(code, "CONSENSUS_OPPOSED_") ||
		strings.HasPrefix(code, "EMA20_STRETCH_") || strings.HasPrefix(code, "BTC_WEAK_LONG_") ||
		strings.HasPrefix(code, "WIDE_STOP_")
}

// applyHardRiskGates enforces program-level gates the AI cannot override:
//  1. 1d-uptrend short block (risk_control.block_short_1d_uptrend)
//  2. Minimum holding period lock on AI-initiated closes (min_hold_minutes)
//  3. Early-close lock (early_close_min_hours, 1h reversal evidence)
//  4. Loss-streak circuit breaker + entry timing gate + regime-line guard
//  5. Stock weekend open block + breakout-hold close gate
//  6. Minimum model confidence for every open
//
// Blocked decisions are dropped with a logged reason, mirroring preTradeRuleCheck.
func (at *AutoTrader) applyHardRiskGates(decisions []kernel.Decision, ctx *kernel.Context) []kernel.Decision {
	if at.config.StrategyConfig == nil || len(decisions) == 0 {
		return decisions
	}
	rc := at.config.StrategyConfig.RiskControl
	filtered := make([]kernel.Decision, 0, len(decisions))
	for _, d := range decisions {
		if kernel.IsOpenDecision(d.Action) {
			if reason := at.protectionFaultReason(); reason != "" {
				at.setFilterReason(d, reason)
				continue
			}
		}
		if strings.HasPrefix(d.Action, "open_") && rc.MinConfidence > 0 && d.Confidence < rc.MinConfidence {
			at.setFilterReason(d, fmt.Sprintf("confidence %d < minimum %d", d.Confidence, rc.MinConfidence))
			logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: confidence %d < configured minimum %d",
				at.name, d.Action, d.Symbol, d.Confidence, rc.MinConfidence)
			continue
		}
		// Account-level circuit breaker (user directive, prompt 09-13): once
		// equity has retraced ≥ account_max_drawdown_pct from the initial
		// balance, ALL new opens are blocked — reduce-only mode. The prompt
		// promises this as CODE ENFORCED; this branch is that enforcement
		// (it was prompt-only until now). Closes/SL/TP are never blocked —
		// the account must be able to de-risk.
		if strings.HasPrefix(d.Action, "open_") && rc.AccountMaxDrawdownPct > 0 && at.initialBalance > 0 {
			equity := ctx.Account.TotalEquity
			if equity > 0 {
				drawdownPct := (at.initialBalance - equity) / at.initialBalance * 100
				if drawdownPct >= rc.AccountMaxDrawdownPct {
					streak, push := at.gateNotifyRecord("acctdd:"+d.Symbol, time.Now())
					logger.Warnf("🛑 [%s] GATE BLOCKED %s %s: account drawdown %.2f%% ≥ %.1f%% (equity %.2f vs initial %.2f) — reduce-only mode (streak %d)",
						at.name, d.Action, d.Symbol, drawdownPct, rc.AccountMaxDrawdownPct, equity, at.initialBalance, streak)
					if push {
						notify.Notify("ALERT", at.name, fmt.Sprintf(
							"<b>🛑 账户级熔断 — 只减仓模式</b>\n净值回撤 <code>%.2f%%</code> ≥ %.1f%%(equity %.2f / initial %.2f)\n一切新开仓被程序拦截,直至回撤修复\n\n<i>%s</i>",
							drawdownPct, rc.AccountMaxDrawdownPct, equity, at.initialBalance, notify.Escape(d.Reasoning)))
					}
					at.setFilterReason(d, "account drawdown ≥ cap — reduce-only mode")
					continue
				}
			}
		}
		// Stock weekend block: only Sat/Sun ET are prohibited. Weekday
		// pre-market, regular session, after-hours, and overnight remain
		// eligible; closes/SL/TP are always unaffected.
		if strings.HasPrefix(d.Action, "open_") {
			if noOpen := rc.StockWeekendNoOpen; noOpen == nil || *noOpen {
				if bt, ok := at.trader.(interface {
					IsStockSymbol(symbol string) bool
				}); ok && bt.IsStockSymbol(d.Symbol) && binance.IsUSMarketWeekend(time.Now()) {
					logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: bstock weekend — US market closed, no new stock positions", at.name, d.Action, d.Symbol)
					at.setFilterReason(d, "US-market weekend (tokenized stock)")
					continue
				}
			}
		}
		// Vendor-divergence executor enforcement (09-28 review P2): the kernel
		// writes VENDOR_DIVERGENCE_* into GateState.Failed, but until now the
		// only consumers were the prompt and the shadow dataset — a market
		// order with market-exception evidence (marketExceptionEvidence does
		// not check divergence) or a resting limit executed anyway, on
		// entry/stop/RR geometry priced off vendor candles diverged beyond
		// the gate. The prompt's contract: this code has NO exception path.
		//
		// F03/F04 (2026-10-01 review): this is now the UNIFIED execution-side
		// authorization for every open decision.
		//   • open_*_limit: resting-limit entries have NO exception path
		//     (marketExceptionGate is market-only), so ANY kernel ban code
		//     with the direction disallowed blocks, and a symbol without a
		//     gate state at all (never kernel-evaluated this cycle — gate
		//     states exist only for pool candidates) is not authorized to
		//     open. The review reproduced EXTENDED_PUMP_UNCONFIRMED /
		//     DATA_INSUFFICIENT / POOR_HISTORY / NEG_EDGE_* limit entries
		//     sailing through on prompt-compliance alone.
		//   • open_* (market): the ABSOLUTE ban codes block here (documented
		//     no-exception contracts). MICRO_TREND_*/RR_MAX_*/MIN_SIZE_DEAD_ZONE
		//     keep their executor-mirrored live checks (timing re-derivation,
		//     validateOpenRisk, min-size re-check); EXTENDED_PUMP_UNCONFIRMED
		//     keeps its sanctioned market-exception path (marketExceptionGate);
		//     STOP_PLAN_* keeps the designed ATR-band fallback; a market open
		//     with NO gate state keeps the existing marketExceptionGate
		//     semantics (missing-gate fail-closed when LimitEntryEnabled).
		if strings.HasPrefix(d.Action, "open_") {
			gs := at.cycleGateStates[market.Normalize(d.Symbol)]
			isLimitOpen := strings.HasSuffix(d.Action, "_limit")
			isShort := strings.Contains(d.Action, "short")
			allowed := true
			var failed []string
			if gs != nil {
				allowed, failed = gs.LongAllowed, gs.LongFailed
				if isShort {
					allowed, failed = gs.ShortAllowed, gs.ShortFailed
				}
			}
			blockCode := ""
			switch {
			case isLimitOpen && gs == nil:
				blockCode = "NO_GATE_STATE"
			case isLimitOpen && !allowed:
				blockCode = "DIRECTION_DISALLOWED"
			case !isLimitOpen && gs != nil && !allowed:
				// Anti-chase codes (CHASE_EMA20_1H_/COUNTER_EMA20_1H_/
				// RSI1H_EXHAUSTED_/CHASE_MOVE4H_) block a MARKET open — it
				// buys at the live price the gate evaluated. They are not
				// absoluteBanCode: the pending recheck would cancel resting
				// pullback limits that buy at their anchor (review 2026-10-11).
				for _, code := range failed {
					if absoluteBanCode(code) || kernel.IsAntiChaseCode(code) {
						blockCode = code
						break
					}
				}
			}
			if blockCode != "" {
				_, push := at.gateNotifyRecord("gatecode:"+d.Symbol, time.Now())
				logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: kernel direction gate (%s, failed: %s)",
					at.name, d.Action, d.Symbol, blockCode, strings.Join(failed, "+"))
				if push {
					notify.Notify("ALERT", at.name, fmt.Sprintf(
						"<b>🛡️ 方向硬门拦截 %s</b>\n<code>%s</code> 被程序阻断(%s):内核禁开码 <code>%s</code>\n<i>限价入场无例外路径;市价绝对禁开码无例外路径</i>",
						notify.Escape(d.Symbol), d.Action, blockCode, notify.Escape(strings.Join(failed, "+"))))
				}
				at.setFilterReason(d, "kernel direction gate: "+blockCode+" ("+strings.Join(failed, "+")+")")
				continue
			}
			if gs != nil {
				// VENDOR_DIVERGENCE_* keeps its dedicated block + notify (no
				// exception path on EITHER entry style).
				vendorBlocked := false
				for _, code := range failed {
					if strings.HasPrefix(code, "VENDOR_DIVERGENCE") {
						vendorBlocked = true
						_, push := at.gateNotifyRecord("vendordiv:"+d.Symbol, time.Now())
						logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: hard-gate code %s — vendor/live divergence has no exception path", at.name, d.Action, d.Symbol, code)
						if push {
							notify.Notify("ALERT", at.name, fmt.Sprintf(
								"<b>🛡️ 数据源偏差拦截 %s</b>\n%s 被程序硬门阻断(<code>%s</code>):K线数据源与实时价偏差超限,入场/止损/RR 全部失真——本码无例外路径,改挂限价与市价均不放行",
								notify.Escape(d.Symbol), d.Action, notify.Escape(code)))
						}
						blockCode = code
						break
					}
				}
				if vendorBlocked {
					at.setFilterReason(d, "vendor divergence beyond gate — no exception path")
					continue
				}
			}
		}
		// Daily-loss halt (QUANT_REVIEW_2026-09-22 D2): once equity is down
		// ≥ daily_max_loss_pct from the day's first-seen equity, new opens
		// are blocked until the next UTC day. The old dailyPnL variable was
		// reset every day but never consumed — this is the missing consumer.
		// Closes/SL/TP unaffected — the account must de-risk, not stop.
		if strings.HasPrefix(d.Action, "open_") {
			if halt := at.dailyLossHaltBlocks(rc, ctx.Account.TotalEquity); halt != "" {
				_, push := at.gateNotifyRecord("dailyhalt:"+time.Now().Format("2006-01-02"), time.Now())
				logger.Warnf("🛑 [%s] GATE BLOCKED %s %s: %s", at.name, d.Action, d.Symbol, halt)
				if push {
					notify.Notify("ALERT", at.name, fmt.Sprintf(
						"<b>🛑 日内亏损熔断 — 今日停开新仓</b>\n%s\n一切新开仓被程序拦截至次日,平仓/止损不受限", halt))
				}
				at.setFilterReason(d, "daily loss halt ("+halt+")")
				continue
			}
		}
		// Account risk-exposure cap (QUANT_REVIEW_2026-09-22 D2): Σ open
		// stop-risk + this trade's stop-risk ≤ max_account_risk_pct × equity.
		// Position-count caps can't see correlation — five same-direction
		// altcoin stops are one big position; this bounds the full-load
		// stop-out (3.5% × 5 correlated positions ≈ 17.5% was one bad cycle
		// from the account breaker).
		if strings.HasPrefix(d.Action, "open_") {
			entryPx := d.Price
			// 2026-10-03 review: an unpriceable decision (Price=0) fell
			// through the account-cap gate with ZERO reservation. Fall back
			// to this cycle's market data before declaring it unpriceable.
			if entryPx <= 0 {
				if md := ctx.MarketDataMap[d.Symbol]; md != nil && md.CurrentPrice > 0 {
					entryPx = md.CurrentPrice
				}
			}
			if gs, ok := at.cycleGateStates[market.Normalize(d.Symbol)]; ok && gs != nil {
				basis := gs.LongEntryPrice
				if strings.Contains(d.Action, "short") {
					basis = gs.ShortEntryPrice
				}
				if basis > 0 {
					entryPx = basis
				}
			}
			if blocked, reason, candUSD := at.accountRiskExposureBlocks(&d, entryPx, ctx); blocked {
				streak, push := at.gateNotifyRecord("exposure:"+d.Symbol, time.Now())
				logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: %s (streak %d)", at.name, d.Action, d.Symbol, reason, streak)
				if push {
					notify.Notify("ALERT", at.name, fmt.Sprintf(
						"<b>🛡️ 账户风险敞口上限 %s</b>\n%s\n\n<i>%s</i>",
						notify.Escape(d.Symbol), reason, notify.Escape(d.Reasoning)))
				}
				continue
			} else {
				// R8: booking DEFERRED — the candidate must survive the whole
				// gate chain first; carried on the decision, booked at the
				// final append.
				d.CycleReservedRiskUSD = candUSD
			}
		}
		// Loss-streak circuit breaker (opens only).
		if strings.HasPrefix(d.Action, "open_") && rc.LossStreakBanEnabled {
			maxLosses := rc.LossStreakMaxLosses
			if maxLosses <= 0 {
				maxLosses = lossStreakDefaultN
			}
			if blocked, reason := at.lossStreakBlocks(d.Symbol, maxLosses); blocked {
				streak, push := at.gateNotifyRecord("lossstreak:"+d.Symbol, time.Now())
				logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s (streak %d): %s", at.name, d.Action, d.Symbol, streak, reason)
				if push {
					notify.Notify("ALERT", at.name, fmt.Sprintf(
						"<b>🛡️ 连亏熔断 %s</b>\n%s\n\n<i>%s</i>",
						notify.Escape(d.Symbol), reason, notify.Escape(d.Reasoning)))
				}
				continue
			}
		}
		if rc.EntryTimingGate {
			if tf, trend := kernel.ExecutionMicroTrend(ctx.MarketDataMap[d.Symbol], time.Now()); tf != "" {
				isLongAction := strings.HasPrefix(d.Action, "open_long")
				isShortAction := strings.HasPrefix(d.Action, "open_short")
				// Symmetric policy (audit 09-13): longs enter on
				// up|pullback, blocked in down|rally|range contexts;
				// shorts enter on down|rally (sell the downtrend bounce),
				// blocked in up|pullback|range.
				bad := (isLongAction && (trend == "down" || trend == "rally" || trend == "range")) ||
					(isShortAction && (trend == "up" || trend == "pullback" || trend == "range"))
				if bad {
					streak, push := at.gateNotifyRecord("timing:"+d.Symbol+":"+d.Action, time.Now())
					logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: %s trend is %s (entry timing gate, streak %d)",
						at.name, d.Action, d.Symbol, tf, trend, streak)
					if push {
						why := map[bool]string{true: "做多", false: "做空"}[strings.HasPrefix(d.Action, "open_long")]
						notify.Notify("ALERT", at.name, fmt.Sprintf(
							"<b>🛡️ 已拦截 %s %s</b>\n\n原因:%s 周期趋势为 <code>%s</code>,入场时点不佳\n\n<i>做多需 up/pullback,做空需 down/rally(反弹做空);range 无动能不放行</i>",
							why, notify.Escape(d.Symbol), tf, trend))
					}
					at.setFilterReason(d, "entry timing gate (sub-hour trend misaligned)")
					continue
				}
				// 2026-10-09 regime line → trend TF (user option B).
				if (isLongAction || isShortAction) && kernel.RegimeLineApplies(isLongAction, trend) {
					if md := ctx.MarketDataMap[d.Symbol]; md != nil {
						lineTF := kernel.ResolveRoleTimeframes(at.config.StrategyConfig).TrendTF
						line := kernel.RegimeLine(md, lineTF, time.Now())
						// Backstop retains fill-site semantics: limit anchor, else live tick.
						px := md.CurrentPrice
						basis := ""
						if strings.HasSuffix(d.Action, "_limit") && d.Price > 0 {
							px = d.Price
							basis = "(锚位)"
						}
						if line > 0 && px > 0 {
							broken := kernel.RegimeLineBreached(isLongAction, px, line)
							if broken {
								streak, push := at.gateNotifyRecord("regimeline:"+d.Symbol+":"+d.Action, time.Now())
								logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: %s regime line (EMA50) %.6g broken by %.6g — possible reversal, not an entry window (streak %d)",
									at.name, d.Action, d.Symbol, lineTF, line, px, streak)
								if push {
									notify.Notify("ALERT", at.name, fmt.Sprintf(
										"<b>🛡️ 已拦截 %s %s</b>\n%s 回踩/反弹入场,但 %s 慢线(EMA50)<code>%.6g</code> 已被 <code>%.6g</code> 跌/升破%s——可能是趋势反转而非入场窗\n\n<i>%s</i>",
										notify.Escape(d.Symbol), map[bool]string{true: "做多", false: "做空"}[isLongAction], tf, lineTF, line, px, basis, notify.Escape(d.Reasoning)))
								}
								at.setFilterReason(d, fmt.Sprintf("%s regime line (EMA50) %.6g breached by %.6g%s — possible reversal", lineTF, line, px, basis))
								continue
							}
						}
					}
				}
			}
		}
		switch d.Action {
		case "open_short", "open_short_limit":
			if rc.BlockShort1dUptrend {
				trend := kernel.TimeframeTrend(ctx.MarketDataMap[d.Symbol], "1d")
				if trend == "up" {
					logger.Warnf("🛡️ [%s] GATE BLOCKED open_short %s: 1d trend is up (counter-trend short protection)", at.name, d.Symbol)
					notify.Notify("ALERT", at.name, fmt.Sprintf("<b>🛡️ 已拦截做空 %s</b>\n1d 趋势向上，禁止逆势做空\n<i>%s</i>", notify.Escape(d.Symbol), notify.Escape(d.Reasoning)))
					// 2026-10-03 review P2: this was a copy-paste of the timing
					// gate's reason — the Telegram CoT mislabeled the block.
					at.setFilterReason(d, "1d trend is up (counter-trend short protection)")
					continue
				}
			}
		case "close_long", "close_short", "partial_close_long", "partial_close_short":
			side := "long"
			if d.Action == "close_short" || d.Action == "partial_close_short" {
				side = "short"
			}
			var markPrice, entryPrice float64
			for _, p := range ctx.Positions {
				if p.Symbol == d.Symbol && p.Side == side {
					markPrice = p.MarkPrice
					entryPrice = p.EntryPrice
					break
				}
			}
			// Resistance-breakout hold: a losing exit into a nearby
			// opposite-side level (with the 15m structure intact) gets
			// blocked — the breakout/breakdown keeps its room.
			if rc.CloseRejectBreakoutPct > 0 && markPrice > 0 && entryPrice > 0 {
				view := buildCloseGateView(ctx.MarketDataMap[d.Symbol], side, markPrice)
				if blocked, reason := closeRejectBreakoutBlocks(side, entryPrice, view, rc.CloseRejectBreakoutPct); blocked {
					streak, push := at.gateNotifyRecord("reshold:"+d.Symbol+":"+d.Action, time.Now())
					logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s (streak %d): %s", at.name, d.Action, d.Symbol, streak, reason)
					if push {
						notify.Notify("ALERT", at.name, fmt.Sprintf(
							"<b>🛡️ 已拦截平仓 %s</b>\n%s\n\n<i>%s</i>",
							notify.Escape(d.Symbol), reason, notify.Escape(d.Reasoning)))
					}
					continue
				}
			}
			if blocked, reason := at.minHoldBlocksClose(d.Symbol, side, markPrice); blocked {
				logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: %s", at.name, d.Action, d.Symbol, reason)
				continue
			}
			// Early-close lock: before EarlyCloseMinHours (default 4h) an AI
			// close needs ≥2 closed 1h candles against the position —
			// SL/TP fills and drawdown-protect never pass through here.
			if blocked, reason := at.earlyCloseBlocksClose(d.Symbol, side, markPrice, ctx.MarketDataMap[d.Symbol]); blocked {
				logger.Warnf("🛡️ [%s] GATE BLOCKED %s %s: %s", at.name, d.Action, d.Symbol, reason)
				continue
			}
		}
		// R8: the decision survived EVERY gate — now book its stop-risk so
		// later decisions in this batch see it.
		at.cycleRiskReservedUSD += d.CycleReservedRiskUSD
		if d.CycleReservedRiskUSD > 0 {
			if strings.Contains(d.Action, "short") {
				at.cycleShortRiskReservedUSD += d.CycleReservedRiskUSD
			} else if strings.Contains(d.Action, "long") {
				at.cycleLongRiskReservedUSD += d.CycleReservedRiskUSD
			}
			if at.cycleSymbolRiskReservedUSD == nil {
				at.cycleSymbolRiskReservedUSD = make(map[string]float64)
			}
			at.cycleSymbolRiskReservedUSD[market.Normalize(d.Symbol)] += d.CycleReservedRiskUSD
		}
		filtered = append(filtered, d)
	}
	return filtered
}

func checkNetRR(d *kernel.Decision, entry, minRR, costBps float64) error {
	risk, reward := entry-d.StopLoss, d.TakeProfit-entry
	if strings.HasPrefix(d.Action, "open_short") {
		risk, reward = d.StopLoss-entry, entry-d.TakeProfit
	}
	cost := entry * costBps / 10000
	if risk <= 0 || reward <= 0 || (reward-cost)/(risk+cost) < minRR {
		return fmt.Errorf("net RR below %.2f at entry %.6g (SL %.6g TP %.6g; cost %.1fbps)", minRR, entry, d.StopLoss, d.TakeProfit, costBps)
	}
	return nil
}

// Separate recovery-leg reconciliation from indicator acquisition so the
// quantity, independent commits and failed-leg retries can be verified.
func (at *AutoTrader) reconcileComputedProtection(symbol, side string, sl, tp float64) bool {
	qty, qtyOK := at.positionQty(symbol, side)
	if !qtyOK || qty <= 0 {
		return false
	}
	if err := ensureProtectiveCoverage(at.trader, symbol, strings.ToUpper(side), "SL", sl, qty); err != nil {
		logger.Infof("⚠️ [%s] Protection watchdog: computed SL place failed for %s: %v", at.name, symbol, err)
		return false
	}
	// Commit each recovery leg independently: a failed TP must not erase the
	// successfully established SL plan or change the initial-R anchor later.
	at.SetRecordedStopLoss(symbol, side, sl)
	at.SetInitialStopLoss(symbol, side, sl)
	at.recordOpenTakeProfit(symbol, side, tp)
	tpQty := at.watchdogTPQty(symbol, side)
	if err := ensureProtectiveCoverage(at.trader, symbol, strings.ToUpper(side), "TP", tp, tpQty); err != nil {
		logger.Infof("⚠️ [%s] Protection watchdog: computed TP place failed for %s: %v", at.name, symbol, err)
		return false
	}
	slErr, tpErr := at.verifyProtectiveLegs(&kernel.Decision{Symbol: symbol, StopLoss: sl, TakeProfit: tp}, strings.ToUpper(side), true, true, qty, tpQty)
	if slErr != nil || tpErr != nil {
		return false
	}
	return true
}
