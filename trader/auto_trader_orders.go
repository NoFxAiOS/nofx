package trader

import (
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	notify "nofx/telegram/notify"
	"nofx/trader/types"
	"strconv"
	"strings"
	"time"
)

// forceMarketEntries rewrites open_long_limit/open_short_limit into market
// opens (Price cleared) when the strategy is market-entry
// (risk_control.limit_entry_enabled=false). The prompt already teaches the
// market path; this keeps a stray limit decision from creating a pending order
// on a strategy that never opted into limit entries. It runs before any
// risk/safe-mode filtering so every later stage sees the final action. The
// input slice is not mutated.
func (at *AutoTrader) forceMarketEntries(ds []kernel.Decision) []kernel.Decision {
	if at.config.StrategyConfig == nil || at.config.StrategyConfig.RiskControl.LimitEntryEnabled {
		return ds
	}
	out := make([]kernel.Decision, len(ds))
	copy(out, ds)
	for i := range out {
		var to string
		switch out[i].Action {
		case "open_long_limit":
			to = "open_long"
		case "open_short_limit":
			to = "open_short"
		default:
			continue
		}
		logger.Infof("🔁 [%s] %s %s → %s: strategy is market-entry (limit_entry_enabled=false)", at.name, out[i].Symbol, out[i].Action, to)
		out[i].Action = to
		out[i].Price = 0
	}
	return out
}

// executeDecisionWithRecord executes AI decision and records detailed information
func (at *AutoTrader) executeDecisionWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		decision, err := at.marketExceptionGate(decision, actionRecord, true)
		if err != nil {
			return err
		}
		if decision.Action == "open_long_limit" {
			// Degraded: no program evidence for a market open but a live
			// anchor exists — take the default limit path instead.
			return at.executeOpenLimitLongWithRecord(decision, actionRecord)
		}
		if err := at.cancelPendingSide(decision.Symbol, "long"); err != nil {
			return fmt.Errorf("cannot replace pending long entry: %w", err)
		}
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		decision, err := at.marketExceptionGate(decision, actionRecord, false)
		if err != nil {
			return err
		}
		if decision.Action == "open_short_limit" {
			return at.executeOpenLimitShortWithRecord(decision, actionRecord)
		}
		if err := at.cancelPendingSide(decision.Symbol, "short"); err != nil {
			return fmt.Errorf("cannot replace pending short entry: %w", err)
		}
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "open_long_limit":
		return at.executeOpenLimitLongWithRecord(decision, actionRecord)
	case "open_short_limit":
		return at.executeOpenLimitShortWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "adjust_stop_loss":
		return at.executeAdjustStopLossWithRecord(decision, actionRecord)
	case "partial_close_long":
		return at.executePartialCloseWithRecord(decision, actionRecord, "long")
	case "partial_close_short":
		return at.executePartialCloseWithRecord(decision, actionRecord, "short")
	case "hold", "wait":
		// No execution needed, just record
		return nil
	default:
		return fmt.Errorf("unknown action: %s", decision.Action)
	}
}

// stampEntryPath tags the entry context (timing-TF trend) for path
// attribution stats — rally-window shorts vs down-trend shorts vs future
// bb_ride market entries must be separately measurable (user 09-13 #2).
func (at *AutoTrader) stampEntryPath(record *store.DecisionAction, data *market.Data) {
	tf, trend := finestSubHourTrend(data)
	if tf == "" {
		return
	}
	record.EntryPath = tf + ":" + trend
}

// marketExceptionGate enforces the market-order exception with program teeth
// (B1, QUANT_REVIEW 2026-09-22). In limit-entry mode a market open is the
// EXCEPTION path, and until now the exception existed only as prompt prose —
// executeOpenLong/Short had no evidence check, so any market decision that
// survived the timing gates went straight to the exchange.
//
// The kernel prices the exception per cycle (DirectionGate.MarketException —
// bb_ride for longs / short_ride for shorts, or a confirmed breakout with
// volume+OI and |directional_score| ≥ MarketExceptionMinScore) and ships it
// in GateState. Enforcement here:
//   - evidence present AND decision confidence ≥ MarketExceptionMinScore
//     (the prompt's bb_ride clause, now code) → market open as decided;
//   - no evidence, live anchor → decision is REWRITTEN to the anchor limit
//     order — the default path the strategy contract always promised;
//   - no evidence, anchor suppressed → rejected fail-closed (mirrors
//     LIMIT_ANCHOR_SUPPRESSED: the direction is unexecutable).
//
// Deliberately NOT gated here: the crossed-anchor market fallback
// (auto_trader_pending.go) — it converts an ALREADY-PLACED limit entry whose
// anchor the price ran through, a documented market path that never passes
// through this dispatch. Market-default strategies (limit_entry_enabled=
// false) are exempt: market IS their default path.
func (at *AutoTrader) marketExceptionGate(d *kernel.Decision, record *store.DecisionAction, isLong bool) (*kernel.Decision, error) {
	if at.config.StrategyConfig == nil {
		return d, nil
	}
	rc := at.config.StrategyConfig.RiskControl
	if !rc.LimitEntryEnabled {
		return d, nil
	}
	gs := at.cycleGateStates[market.Normalize(d.Symbol)]
	if gs == nil {
		return d, fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: no hard-gate state this cycle — market open unverifiable (fail-closed)", d.Action, d.Symbol)
	}
	exception := gs.LongMarketException
	anchorLive := gs.LongLimitAllowed
	anchor := gs.LongEntryPrice
	if !isLong {
		exception = gs.ShortMarketException
		anchorLive = gs.ShortLimitAllowed
		anchor = gs.ShortEntryPrice
	}
	if exception && d.Confidence >= kernel.MarketExceptionMinScore {
		return d, nil
	}
	if anchorLive && anchor > 0 {
		limitAction := "open_long_limit"
		if !isLong {
			limitAction = "open_short_limit"
		}
		if code := limitEntryBlockCode(gs, !isLong); code != "" {
			reason := "kernel direction gate: " + code + " (market-to-limit conversion)"
			at.setFilterReason(*d, reason)
			return d, fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %s", d.Action, d.Symbol, reason)
		}
		streak, push := at.gateNotifyRecord("mktgate:"+d.Symbol, time.Now())
		logger.Warnf("🛡️ [%s] MARKET→LIMIT %s %s: no market-exception evidence (or confidence %d < %d) — degraded to anchor limit %.6g (streak %d)",
			at.name, d.Action, d.Symbol, d.Confidence, kernel.MarketExceptionMinScore, anchor, streak)
		if push {
			notify.Notify("ALERT", at.name, fmt.Sprintf(
				"<b>🛡️ 市价降级限价 %s</b>\n%s 无市价例外证据(或 confidence %d < %d),已按锚点 <code>%.6g</code> 改挂限价\n\n<i>%s</i>",
				notify.Escape(d.Symbol), d.Action, d.Confidence, kernel.MarketExceptionMinScore, anchor, notify.Escape(d.Reasoning)))
		}
		d.Action = limitAction
		d.Price = anchor
		d.MarketDegraded = true
		return d, nil
	}
	return d, fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: no market-exception evidence and no live anchor — direction unexecutable this cycle", d.Action, d.Symbol)
}

// executeOpenLongWithRecord executes open long position and records detailed information
func (at *AutoTrader) executeOpenLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	if err := at.entryExecutionBlocked(decision.Symbol, "long"); err != nil {
		return err
	}
	if err := at.unknownSubmitBlocks(decision.Symbol, "long"); err != nil {
		return err
	}
	logger.Infof("  📈 Open long: %s", decision.Symbol)

	if marketData, err := at.getMarketData(decision.Symbol); err == nil {
		at.stampEntryPath(actionRecord, marketData)
	}

	// Spread gate: thin books eat the entry edge (market fills pay the spread).
	if blocked, reason := at.spreadBlocksOpen(decision.Symbol); blocked {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %s", decision.Action, decision.Symbol, reason)
	}

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit. Resting limit entries on
	// OTHER symbols occupy slots too (same accounting as the limit path) —
	// counting only open positions let a market open push open+pending past
	// the cap when the limit path had already been refused.
	if err := at.enforceMaxPositions(nextSlotCount(len(positions), at.snapshotPendingKeys(), pendingEntryKey(decision.Symbol, "long"))); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
			return fmt.Errorf("❌ %s already has long position, close it first", decision.Symbol)
		}
	}

	// Get current price
	marketData, err := at.getMarketData(decision.Symbol)
	if err != nil {
		return err
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Get equity for position value ratio check
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else {
		equity = availableBalance // Fallback to available balance
	}

	// [CODE ENFORCED] Entry risk gates: mandatory SL, SL/TP side sanity,
	// min RR (dual-anchor), stop-distance window [ATR floor, wide cap].
	floorATR, capATR := at.stopBandATRs(decision.Symbol, marketData)
	if err := at.validateOpenRisk(decision, marketData.CurrentPrice, floorATR, capATR); err != nil {
		return err
	}

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// [CODE ENFORCED] Risk-based sizing: position value derives from the stop
	// distance (equity × risk% ÷ dist%), clamped down when the AI oversizes.
	decision.PositionSizeUSD = at.clampSizeToRisk(decision, decision.PositionSizeUSD, equity, marketData.CurrentPrice)

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Margin-budget gate: (used + new) margin ≤ max_margin_usage × equity —
	// the prompt states the budget, this enforces it (audit 09-13 #2).
	if blocked, reason := at.marginBudgetBlocksOpen(decision.Symbol, "long", decision.PositionSizeUSD, float64(decision.Leverage), equity); blocked {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %s", decision.Action, decision.Symbol, reason)
	}
	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.entryUsesCrossMargin()); err != nil {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: margin mode not verified: %w", decision.Action, decision.Symbol, err)
	}

	// Open position
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return at.trackUnknownSubmit(err, "long", pendingEntry{
			Symbol: decision.Symbol, Price: marketData.CurrentPrice, Quantity: quantity,
			StopLoss: decision.StopLoss, TakeProfit: decision.TakeProfit,
			Leverage: decision.Leverage, ExitMode: decision.ExitMode,
		})
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		actionRecord.EntryOrderID = strconv.FormatInt(orderID, 10) // review 2026-10-08 H
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f", order["orderId"], quantity)

	// Record order to database and poll for confirmation
	entryID, _ := orderIDString(order)
	at.markAIManaged(decision.Symbol, "long", entryID)
	at.recordAndConfirmOrder(order, decision.Symbol, "open_long", quantity, marketData.CurrentPrice, decision.Leverage, 0)

	fillPrice := orderFloat(order, "avgPrice")
	if fillPrice <= 0 {
		fillPrice = orderFloat(order, "fillPrice")
	}
	if fillPrice <= 0 {
		// R3 (2026-09-26 review) — LONG side. The raw order response carries
		// no avgPrice on Binance; without polling, the slippage reanchor
		// silently no-opped on longs while shorts got it (the 09-26 review's
		// P0 asymmetry — my earlier edit landed only the short side).
		// F21d (2026-10-01 review): poll for ANY adapter's orderId shape
		// (int64/float64/string), not just int64.
		if id, ok := orderIDString(order); ok {
			if avg, confirmed := at.confirmedFillPrice(decision.Symbol, id, marketData.CurrentPrice); confirmed {
				fillPrice = avg
			}
		}
	}
	// F21c: configurable hard cap on adverse entry slippage (0 = disabled).
	at.invalidateExecutionCache()
	if filledQty, ok := at.positionQty(decision.Symbol, "long"); ok && filledQty > 0 {
		quantity = filledQty
		actionRecord.Quantity = filledQty
	}
	at.SetRecordedStopLoss(decision.Symbol, "long", decision.StopLoss)
	at.SetInitialStopLoss(decision.Symbol, "long", decision.StopLoss)
	at.recordOpenTakeProfit(decision.Symbol, "long", decision.TakeProfit)
	if err := at.enforceEntrySlippageCap(decision, "long", marketData.CurrentPrice, fillPrice); err != nil {
		return err
	}
	if err := at.actualFillRisk(decision, fillPrice, quantity, equity); err != nil {
		return at.rejectMarketFill(decision, "long", quantity, err)
	}
	// F21d: the recorded action price is the ACTUAL fill, not the pre-fill
	// ticker — plan/fill/protection prices must not blur into one number.
	if fillPrice > 0 {
		actionRecord.Price = fillPrice
	}
	// Preserve absolute structure levels; fill risk has been revalidated.

	// Record position opening time and stop-loss (drives the min-hold gate)
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
	at.SetRecordedStopLoss(decision.Symbol, "long", decision.StopLoss)
	at.SetInitialStopLoss(decision.Symbol, "long", decision.StopLoss) // 1R anchor — write-once, immune to later tighten
	at.SetExitMode(decision.Symbol, "long", decision.ExitMode)        // exit template — write-once, drives split-TP/trailing/time-stop
	// Peak PnL is per-position state — a re-opened symbol must not inherit
	// the previous trade's peak (stale peaks poison the drawdown monitors).
	at.ClearPeakPnLCache(decision.Symbol, "long")

	slErr, tpErr := at.placeProtectiveOrders(decision, "LONG", quantity, marketData.CurrentPrice, fillPrice)
	at.reportFillSlippage(decision, marketData.CurrentPrice, fillPrice)
	// F01 (2026-10-01 review): a missing protective leg must never be
	// reported as a successful entry — surface it as an execution failure
	// naming the exact leg. review 2026-10-11: Binance initial SL failures
	// have already exhausted quick retries and submitted a slice exit here;
	// recovery stays visible until a fresh snapshot confirms coverage/exit.
	if slErr != nil {
		at.setProtectionFault(decision.Symbol+"_long", "entry SL coverage unverified")
		return fmt.Errorf("❌ [PROTECTION] %s %s opened @ %.6g but SL placement FAILED: %w — initial protection failed; recovery state retained", decision.Action, decision.Symbol, fillPrice, slErr)
	}
	if tpErr != nil {
		return fmt.Errorf("❌ [PROTECTION] %s %s opened @ %.6g, SL verified, TP placement FAILED: %w — watchdog will retry", decision.Action, decision.Symbol, fillPrice, tpErr)
	}
	return nil
}

// fillSlippageAlertBps: slippage worth an alert — 1% of the checked price.
// Below that a fast-market fill is routine and stays out of the alert channel.
const fillSlippageAlertBps = 100

// reportFillSlippage reports adverse execution drift. Structure levels stay
// fixed; actual-fill risk validation decides whether the position can remain.
func (at *AutoTrader) reportFillSlippage(decision *kernel.Decision, checkedPrice, fillPrice float64) (float64, bool) {
	if fillPrice <= 0 || checkedPrice <= 0 || fillPrice == checkedPrice {
		return 0, false
	}
	side := "long"
	if strings.HasPrefix(decision.Action, "open_short") {
		side = "short"
	}
	slippageBps := adverseSlippageBps(checkedPrice, fillPrice, side)
	if slippageBps < fillSlippageAlertBps {
		return slippageBps, false
	}
	logger.Warnf("⚠️ [%s] %s %s filled @ %.6g (checked @ %.6g, %.0fbps away) — adverse entry drift; structural SL/TP unchanged, actual-fill risk rechecked",
		at.name, decision.Action, decision.Symbol, fillPrice, checkedPrice, slippageBps)
	notify.Notify("ALERT", at.name, fmt.Sprintf(
		"<b>⚠️ 市价成交滑点 %s</b>\n校验价 %.6g → 成交价 <code>%.6g</code>(滑点 <code>%.0f</code>bps)\nSL/TP 保留原始结构价位，已复核实际成交风险与净盈亏比",
		notify.Escape(decision.Symbol), checkedPrice, fillPrice, slippageBps))
	return slippageBps, true
}

// adverseSlippageBps returns the ADVERSE slippage of a fill vs the checked
// price in basis points (0 when the fill improved or prices are unusable).
func adverseSlippageBps(checkedPrice, fillPrice float64, side string) float64 {
	if checkedPrice <= 0 || fillPrice <= 0 {
		return 0
	}
	var adverse float64
	if side == "long" {
		adverse = fillPrice - checkedPrice
	} else {
		adverse = checkedPrice - fillPrice
	}
	if adverse <= 0 {
		return 0
	}
	return adverse / checkedPrice * 10000
}

// enforceEntrySlippageCap (F21c, 2026-10-01 review): a market open whose
// adverse fill slippage breaches risk_control.max_entry_slippage_bps is
// emergency-closed immediately and reported as a FAILED open. Disabled
// (0/negative) keeps the alert-only behavior. Called BEFORE any recorded
// state is written, so a capped close leaves no stale recorded stop/anchor.
func (at *AutoTrader) enforceEntrySlippageCap(decision *kernel.Decision, side string, checkedPrice, fillPrice float64) error {
	if at.config.StrategyConfig == nil {
		return nil
	}
	rc := at.config.StrategyConfig.RiskControl
	if rc.MaxEntrySlippageBps <= 0 {
		return nil
	}
	bps := adverseSlippageBps(checkedPrice, fillPrice, side)
	if bps < float64(rc.MaxEntrySlippageBps) {
		return nil
	}
	logger.Warnf("🛑 [%s] %s %s adverse slippage %.0fbps ≥ cap %dbps (checked %.6g → filled %.6g) — emergency closing", at.name, decision.Action, decision.Symbol, bps, rc.MaxEntrySlippageBps, checkedPrice, fillPrice)
	at.setProtectionFault("recovery:"+decision.Symbol+"_"+side, "slippage cap exit pending confirmation")
	if closeErr := at.emergencyClosePosition(decision.Symbol, side); closeErr != nil {
		if qty, ok := at.positionQty(decision.Symbol, side); ok {
			at.placeProtectiveOrders(decision, strings.ToUpper(side), qty, checkedPrice, fillPrice)
		}
		return fmt.Errorf("❌ [SLIPPAGE CAP] %s %s filled @ %.6g (%.0fbps adverse ≥ %dbps cap) and EMERGENCY CLOSE FAILED: %v — manual close required", decision.Action, decision.Symbol, fillPrice, bps, rc.MaxEntrySlippageBps, closeErr)
	}
	notify.Notify("ALERT", at.name, fmt.Sprintf(
		"<b>🛑 滑点熔断 %s</b>\n<i>%s 校验价 %.6g → 成交 %.6g(不利 %.0fbps ≥ %dbps)——仓位已立即平掉,本次开仓记为失败</i>",
		notify.Escape(decision.Symbol), side, checkedPrice, fillPrice, bps, rc.MaxEntrySlippageBps))
	return fmt.Errorf("❌ [SLIPPAGE CAP] %s %s filled @ %.6g (%.0fbps adverse ≥ %dbps cap) — position closed immediately", decision.Action, decision.Symbol, fillPrice, bps, rc.MaxEntrySlippageBps)
}

// executeOpenShortWithRecord executes open short position and records detailed information
func (at *AutoTrader) executeOpenShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	if err := at.entryExecutionBlocked(decision.Symbol, "short"); err != nil {
		return err
	}
	if err := at.unknownSubmitBlocks(decision.Symbol, "short"); err != nil {
		return err
	}
	logger.Infof("  📉 Open short: %s", decision.Symbol)

	if marketData, err := at.getMarketData(decision.Symbol); err == nil {
		at.stampEntryPath(actionRecord, marketData)
	}

	// Spread gate: thin books eat the entry edge (market fills pay the spread).
	if blocked, reason := at.spreadBlocksOpen(decision.Symbol); blocked {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %s", decision.Action, decision.Symbol, reason)
	}

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit. Resting limit entries on
	// OTHER symbols occupy slots too (same accounting as the limit path) —
	// counting only open positions let a market open push open+pending past
	// the cap when the limit path had already been refused.
	if err := at.enforceMaxPositions(nextSlotCount(len(positions), at.snapshotPendingKeys(), pendingEntryKey(decision.Symbol, "short"))); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
			return fmt.Errorf("❌ %s already has short position, close it first", decision.Symbol)
		}
	}

	// Get current price
	marketData, err := at.getMarketData(decision.Symbol)
	if err != nil {
		return err
	}

	// Get balance (needed for multiple checks)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Get equity for position value ratio check
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else {
		equity = availableBalance // Fallback to available balance
	}

	// [CODE ENFORCED] Entry risk gates: mandatory SL, SL/TP side sanity,
	// min RR (dual-anchor), stop-distance window [ATR floor, wide cap].
	floorATR, capATR := at.stopBandATRs(decision.Symbol, marketData)
	if err := at.validateOpenRisk(decision, marketData.CurrentPrice, floorATR, capATR); err != nil {
		return err
	}

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// [CODE ENFORCED] Risk-based sizing: position value derives from the stop
	// distance (equity × risk% ÷ dist%), clamped down when the AI oversizes.
	decision.PositionSizeUSD = at.clampSizeToRisk(decision, decision.PositionSizeUSD, equity, marketData.CurrentPrice)

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(decision.Leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Margin-budget gate: (used + new) margin ≤ max_margin_usage × equity —
	// the prompt states the budget, this enforces it (audit 09-13 #2).
	if blocked, reason := at.marginBudgetBlocksOpen(decision.Symbol, "short", decision.PositionSizeUSD, float64(decision.Leverage), equity); blocked {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %s", decision.Action, decision.Symbol, reason)
	}
	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.entryUsesCrossMargin()); err != nil {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: margin mode not verified: %w", decision.Action, decision.Symbol, err)
		// Continue execution, doesn't affect trading
	}

	// Open position
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return at.trackUnknownSubmit(err, "short", pendingEntry{
			Symbol: decision.Symbol, Price: marketData.CurrentPrice, Quantity: quantity,
			StopLoss: decision.StopLoss, TakeProfit: decision.TakeProfit,
			Leverage: decision.Leverage, ExitMode: decision.ExitMode,
		})
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
		actionRecord.EntryOrderID = strconv.FormatInt(orderID, 10) // review 2026-10-08 H
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f", order["orderId"], quantity)

	// Record order to database and poll for confirmation
	entryID, _ := orderIDString(order)
	at.markAIManaged(decision.Symbol, "short", entryID)
	at.recordAndConfirmOrder(order, decision.Symbol, "open_short", quantity, marketData.CurrentPrice, decision.Leverage, 0)

	// Record position opening time and stop-loss (drives the min-hold gate)
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
	fillPrice := orderFloat(order, "avgPrice")
	if fillPrice <= 0 {
		fillPrice = orderFloat(order, "fillPrice")
	}
	if fillPrice <= 0 {
		// F21d: poll for ANY adapter's orderId shape, not just int64.
		if id, ok := orderIDString(order); ok {
			if avg, confirmed := at.confirmedFillPrice(decision.Symbol, id, marketData.CurrentPrice); confirmed {
				fillPrice = avg
			}
		}
	}
	// F21c: configurable hard cap on adverse entry slippage (0 = disabled).
	at.invalidateExecutionCache()
	if filledQty, ok := at.positionQty(decision.Symbol, "short"); ok && filledQty > 0 {
		quantity = filledQty
		actionRecord.Quantity = filledQty
	}
	at.SetRecordedStopLoss(decision.Symbol, "short", decision.StopLoss)
	at.SetInitialStopLoss(decision.Symbol, "short", decision.StopLoss)
	at.recordOpenTakeProfit(decision.Symbol, "short", decision.TakeProfit)
	if err := at.enforceEntrySlippageCap(decision, "short", marketData.CurrentPrice, fillPrice); err != nil {
		return err
	}
	if err := at.actualFillRisk(decision, fillPrice, quantity, equity); err != nil {
		return at.rejectMarketFill(decision, "short", quantity, err)
	}
	// F21d: record the ACTUAL fill price, not the pre-fill ticker.
	if fillPrice > 0 {
		actionRecord.Price = fillPrice
	}

	at.SetRecordedStopLoss(decision.Symbol, "short", decision.StopLoss)
	at.SetInitialStopLoss(decision.Symbol, "short", decision.StopLoss) // 1R anchor — write-once, immune to later tighten
	at.SetExitMode(decision.Symbol, "short", decision.ExitMode)        // exit template — write-once, drives split-TP/trailing/time-stop
	// Peak PnL is per-position state — see the open_long note above.
	at.ClearPeakPnLCache(decision.Symbol, "short")

	slErr, tpErr := at.placeProtectiveOrders(decision, "SHORT", quantity, marketData.CurrentPrice, fillPrice)
	at.reportFillSlippage(decision, marketData.CurrentPrice, fillPrice)
	// F01: same no-false-success contract as the long side above.
	if slErr != nil {
		at.setProtectionFault(decision.Symbol+"_"+strings.TrimPrefix(decision.Action, "open_"), "entry SL coverage unverified")
		return fmt.Errorf("❌ [PROTECTION] %s %s opened @ %.6g but SL placement FAILED: %w — initial protection failed; recovery state retained", decision.Action, decision.Symbol, fillPrice, slErr)
	}
	if tpErr != nil {
		return fmt.Errorf("❌ [PROTECTION] %s %s opened @ %.6g, SL verified, TP placement FAILED: %w — watchdog will retry", decision.Action, decision.Symbol, fillPrice, tpErr)
	}
	return nil
}

// confirmedFillPrice polls the order status for the ACTUAL average fill
// price (R3, 2026-09-26 review): Binance's OpenLong/OpenShort response map
// carries only orderId/status — avgPrice is always 0 there, so the old code
// silently skipped the slippage reanchor (fillPrice==refPrice ⇒ no-op).
// Bounded polling (5×400ms); an unknown fill cannot pass actualFillRisk.
func (at *AutoTrader) confirmedFillPrice(symbol, orderID string, checkedPrice float64) (float64, bool) {
	if orderID == "" {
		return checkedPrice, false
	}
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(400 * time.Millisecond)
		}
		status, err := at.trader.GetOrderStatus(symbol, orderID)
		if err != nil {
			continue
		}
		if st, _ := status["status"].(string); strings.ToUpper(st) == "NEW" {
			continue
		}
		if avg := orderFloat(status, "avgPrice"); avg > 0 {
			return avg, true
		}
	}
	return checkedPrice, false
}

// orderFloat reads a float64 field from an exchange order response map.
func orderFloat(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

// orderIDString extracts the order ID from an exchange order response map in
// ANY adapter shape (F21d, 2026-10-01 review: the fill-confirmation poll used
// to gate on int64 only, so string- or float64-ID adapters silently skipped
// the avgPrice poll and the slippage reanchor no-opped).
func orderIDString(m map[string]interface{}) (string, bool) {
	if m == nil {
		return "", false
	}
	switch v := m["orderId"].(type) {
	case int64:
		if v != 0 {
			return strconv.FormatInt(v, 10), true
		}
	case float64:
		if v > 0 {
			return strconv.FormatFloat(v, 'f', -1, 64), true
		}
	case string:
		if v != "" {
			return v, true
		}
	}
	return "", false
}

// placeProtectiveOrders places the exchange-side stop-loss/take-profit algo
// orders (closePosition mode on Binance) right after a successful open, so the
// AI's planned SL/TP are enforced by the exchange instead of only living in
// the decision log. The SL/TP are RE-ANCHORED to the actual fill price (the
// RR gate validated distances at a pre-fill ticker — a market order can fill
// away from it, silently compressing the take-profit distance). Zero prices
// are skipped; placement failures raise an alert because the position would
// otherwise run unprotected. quantity is used by exchange implementations that
// place qty-sized trigger orders.
//
// F01 (2026-10-01 review): returns BOTH leg errors — the TP rejection used to
// vanish into a log line, so callers booked fills as protected with no profit
// leg. Callers gate their state advancement on ACTUAL success. Retry-safety:
// a leg already resting at (≈) the intended price is skipped, so a watermark
// retry after one leg's failure cannot stack duplicate stops on qty-sized
// adapters (OKX/Bybit); Binance closePosition dedupes via the -4130 self-heal.
func (at *AutoTrader) placeProtectiveOrders(decision *kernel.Decision, positionSide string, quantity float64, refPrice, fillPrice float64) (slErr, tpErr error) {
	return at.placeProtectiveOrdersTPDone(decision, positionSide, quantity, refPrice, fillPrice, 0)
}

// placeProtectiveOrdersTPDone is placeProtectiveOrders for a cumulative
// quantity of which tpDone has ALREADY been realized by earlier take-profit
// legs (2026-10-20 review area2 #7). The TP target is
// quantity*fraction - tpDone, so a split TP that already filled is not
// re-armed against the runner. tpDone == 0 is the unchanged legacy behavior.
func (at *AutoTrader) placeProtectiveOrdersTPDone(decision *kernel.Decision, positionSide string, quantity float64, refPrice, fillPrice, tpDone float64) (slErr, tpErr error) {
	mode := decision.ExitMode
	if mode == "" {
		mode = kernel.ExitModeTrend
	}
	tpQty := quantity * tpFractionForMode(mode, at.effectiveTPCloseFraction())
	if tpDone > 0 {
		tpQty = math.Max(0, tpQty-tpDone)
	}
	tpNeeded := decision.TakeProfit > 0 && (tpDone <= 0 || tpQty > math.Max(1e-9, quantity*1e-6))
	if decision.StopLoss > 0 {
		slErr = ensureProtectiveCoverage(at.trader, decision.Symbol, positionSide, "SL", decision.StopLoss, quantity)
	}
	// review 2026-10-11: initial Binance SL failure cannot wait for three
	// 30s repairs. Verify now, retry 1/2/4s, then exit the unprotected slice.
	if at.binanceRealtimeCapable() && decision.StopLoss > 0 && at.isAIManaged(decision.Symbol, strings.ToLower(positionSide)) {
		verified := at.verifyInitialStopCoverage(decision.Symbol, positionSide, decision.StopLoss, quantity)
		slErr = verified
		if slErr != nil {
			slErr = at.initialStopFastFailure(decision.Symbol, strings.ToLower(positionSide), decision.StopLoss, quantity, slErr)
			if slErr != nil {
				return slErr, fmt.Errorf("TP deferred after initial SL failure")
			}
		}
	}
	if tpNeeded {
		tpErr = ensureProtectiveCoverage(at.trader, decision.Symbol, positionSide, "TP", decision.TakeProfit, tpQty)
	}
	verifiedSL, verifiedTP := at.verifyProtectiveLegs(decision, positionSide, decision.StopLoss > 0, tpNeeded, quantity, tpQty)
	if slErr == nil {
		slErr = verifiedSL
	}
	if tpErr == nil {
		tpErr = verifiedTP
	}
	if slErr != nil && decision.StopLoss > 0 && at.binanceRealtimeCapable() {
		// review 2026-10-11: coverage can also disappear during final TP
		// verification. That failure gets the same within-pass exit budget.
		slErr = at.initialStopFastFailure(decision.Symbol, strings.ToLower(positionSide), decision.StopLoss, quantity, slErr)
	}
	if tpErr == nil && decision.TakeProfit > 0 {
		if tpQty < quantity {
			at.markTPRunner(decision.Symbol + "_" + strings.ToLower(positionSide))
		}
		at.recordOpenTakeProfit(decision.Symbol, strings.ToLower(positionSide), decision.TakeProfit)
	}
	return
}

func protectiveOrderMatches(o types.OpenOrder, side, kind string, price float64) bool {
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return false
	}
	if o.Status != "" && !strings.EqualFold(o.Status, "NEW") && !strings.EqualFold(o.Status, "PARTIALLY_FILLED") {
		return false
	}
	typ := strings.ToUpper(o.Type)
	match := strings.Contains(typ, "TAKE_PROFIT")
	if kind == "SL" {
		match = strings.Contains(typ, "STOP") && !strings.Contains(typ, "TAKE_PROFIT")
	}
	return match && orderClosesSide(o, side) && o.StopPrice > 0 && math.Abs(o.StopPrice-price)/price < 0.001
}

func protectiveCoverage(orders []types.OpenOrder, side, kind string, price float64) float64 {
	qty := 0.0
	seen := map[string]bool{}
	for _, o := range orders {
		if !protectiveOrderMatches(o, side, kind, price) {
			continue
		}
		if o.OrderID != "" {
			id := fmt.Sprintf("%t:%s", o.Algo, o.OrderID)
			if seen[id] {
				continue
			}
			seen[id] = true
		}
		if o.ClosePosition {
			return math.Inf(1)
		}
		if !math.IsNaN(o.Quantity) && !math.IsInf(o.Quantity, 0) {
			qty += math.Max(0, o.Quantity)
		}
	}
	return qty
}

func ensureProtectiveCoverage(t Trader, symbol, side, kind string, price, quantity float64) error {
	if price <= 0 || quantity <= 0 || math.IsNaN(price) || math.IsInf(price, 0) || math.IsNaN(quantity) || math.IsInf(quantity, 0) {
		return fmt.Errorf("invalid %s protection price/quantity", kind)
	}
	orders, err := t.GetOpenOrders(symbol)
	if err != nil {
		return fmt.Errorf("%s coverage unknown: %w", kind, err)
	}
	missing := quantity - protectiveCoverage(orders, side, kind, price)
	if missing <= math.Max(1e-9, quantity*1e-6) {
		return nil
	}
	if kind == "SL" {
		return t.SetStopLoss(symbol, side, missing, price)
	}
	return t.SetTakeProfit(symbol, side, missing, price)
}

// protectiveLegAtPrice reports whether a protective leg (kind "SL"/"TP") for
// the position side already rests on the exchange at ≈ the intended price
// (0.1% band — both sides tick-round the same plan price). The retry-safe
// skip in placeProtectiveOrders: exact-price retries must not stack duplicate
// legs on qty-sized adapters, while a STALE leg at a different price (the
// -4130 case) still routes through placement so Binance's self-heal can
// replace it.
func protectiveLegAtPrice(t Trader, symbol, positionSide string, kind string, wantPrice float64) bool {
	if wantPrice <= 0 {
		return false
	}
	orders, err := t.GetOpenOrders(symbol)
	if err != nil {
		return false
	}
	for _, o := range orders {
		oType := strings.ToUpper(o.Type)
		if kind == "SL" {
			if !strings.Contains(oType, "STOP") {
				continue
			}
		} else {
			if !strings.Contains(oType, "TAKE_PROFIT") {
				continue
			}
		}
		if o.PositionSide != "" && !strings.EqualFold(o.PositionSide, positionSide) {
			continue
		}
		if o.StopPrice > 0 && math.Abs(o.StopPrice-wantPrice)/wantPrice < 0.001 {
			return true
		}
	}
	return false
}

// missingLegsReport returns "" when every INTENDED leg (wantSL/wantTP) is
// resting on the exchange; otherwise names what is missing. Pure — the
// retry/notify wrapper around it is verifyProtectiveLegs.
func missingLegsReport(orders []types.OpenOrder, positionSide string, wantSL, wantTP bool) string {
	needSL, needTP := missingProtection(orders, positionSide)
	var missing []string
	if wantSL && needSL {
		missing = append(missing, "SL")
	}
	if wantTP && needTP {
		missing = append(missing, "TP")
	}
	return strings.Join(missing, "+")
}

// verifyProtectiveLegs re-queries the exchange open orders right after the
// placement pass. One retry after a short delay rides out read-after-write
// lag without masking a real rejection; a still-missing leg is an ALERT —
// the position is running unprotected or without its profit leg.
func (at *AutoTrader) verifyProtectiveLegs(decision *kernel.Decision, positionSide string, wantSL, wantTP bool, quantities ...float64) (slErr, tpErr error) {
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		slErr, tpErr = nil, nil
		orders, err := at.trader.GetOpenOrders(decision.Symbol)
		if err != nil {
			if wantSL {
				slErr = err
			}
			if wantTP {
				tpErr = err
			}
			continue
		}
		if len(quantities) >= 2 {
			if wantSL && protectiveCoverage(orders, positionSide, "SL", decision.StopLoss)+math.Max(1e-9, quantities[0]*1e-6) < quantities[0] {
				slErr = fmt.Errorf("SL coverage unconfirmed")
			}
			if wantTP && protectiveCoverage(orders, positionSide, "TP", decision.TakeProfit)+math.Max(1e-9, quantities[1]*1e-6) < quantities[1] {
				tpErr = fmt.Errorf("TP coverage unconfirmed")
			}
		} else {
			missing := missingLegsReport(orders, positionSide, wantSL, wantTP)
			if strings.Contains(missing, "SL") {
				slErr = fmt.Errorf("SL missing")
			}
			if strings.Contains(missing, "TP") {
				tpErr = fmt.Errorf("TP missing")
			}
		}
		if slErr == nil && tpErr == nil {
			return
		}
	}
	// 2026-10-03 review P2 (cosmetic): a nil leg error printed "TP: <nil>".
	legText := func(name string, err error) string {
		if err == nil {
			return name + ": OK"
		}
		return fmt.Sprintf("%s: %v", name, err)
	}
	notify.Notify("ALERT", at.name, fmt.Sprintf("<b>保护核验未完成 %s %s</b>\n%s / %s — 恢复任务保留", notify.Escape(decision.Symbol), positionSide, legText("SL", slErr), legText("TP", tpErr)))
	return
}

// effectiveTPCloseFraction resolves the split-TP fraction for this trader:
// trailing disabled → 1.0 (full close, no runner), otherwise the configured
// kernel.TPCloseFraction (0 = default 0.5, negative = legacy 1.0).
func (at *AutoTrader) effectiveTPCloseFraction() float64 {
	if at.config.StrategyConfig == nil || !at.config.StrategyConfig.RiskControl.TrailingStopEnabled {
		return 1.0
	}
	return kernel.TPCloseFraction(&at.config.StrategyConfig.RiskControl)
}

// tpFractionForMode resolves the TP-algo close fraction for one position's
// exit template (user menu directive 09-29): only the TREND template runs a
// runner, so a trend position keeps the configured split fraction while
// range/quick close the FULL position at the chosen menu level. base is the
// trader's effectiveTPCloseFraction (already 1.0 when trailing is off).
func tpFractionForMode(mode string, base float64) float64 {
	if mode != "" && mode != kernel.ExitModeTrend && base < 1.0 {
		return 1.0
	}
	return base
}

// executeCloseLongWithRecord executes close long position and records detailed information
func (at *AutoTrader) executeCloseLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close long: %s", decision.Symbol)

	// Hands-off rule (user directive 2026-09-25): the AI does not manage
	// manually opened positions — closing them is automation acting on them.
	if !at.isAIManaged(decision.Symbol, "long") {
		return fmt.Errorf("❌ [HANDS-OFF] %s was not opened by the AI — manual positions are never closed/adjusted by the program (close the position manually or take it over via a decision to open it)", decision.Symbol)
	}

	// Reference price for the record only. A close must never be blocked by
	// market-data trouble (2026-10-16 review D): lightweight quote first, then
	// the full market data, else record 0 and close anyway.
	closePrice := at.closeReferencePrice(decision.Symbol)
	actionRecord.Price = closePrice

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity - prioritize local database for accurate quantity
	var entryPrice float64
	var quantity float64

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "LONG"); err == nil && openPos != nil {
			quantity = openPos.Quantity
			entryPrice = openPos.EntryPrice
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Fallback to exchange API if local data not found
	if quantity == 0 {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
					if ep, ok := pos["entryPrice"].(float64); ok {
						entryPrice = ep
					}
					if amt, ok := pos["positionAmt"].(float64); ok && amt > 0 {
						quantity = amt
					}
					break
				}
			}
		}
		logger.Infof("  📊 Using exchange position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	// Close position
	at.markCloseIntent(decision.Symbol, "long", "ai_close")
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = close all
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "close_long", quantity, closePrice, 0, entryPrice)

	// 2026-10-20 review area2 #2: a successful POST is not a closed position
	// (a market close can part-fill and expire). Protection state and AI
	// ownership are released only after a FRESH snapshot shows this side flat;
	// a remainder keeps its SL/TP, recorded levels and AI mark so the watchdog
	// and the next decision keep managing it. The per-cycle "position gone"
	// sweep clears the registry once the remainder really disappears.
	if !at.closeConfirmedFlat(decision.Symbol, "long") {
		at.keepUnconfirmedClose(decision.Symbol, "long")
		return nil
	}
	at.ClearRecordedStopLoss(decision.Symbol, "long")
	at.ClearInitialStopLoss(decision.Symbol, "long")
	at.ClearExitMode(decision.Symbol, "long")
	at.ClearPeakPnLCache(decision.Symbol, "long")
	at.unmarkAIManaged(decision.Symbol, "long") // lifecycle complete — registry clean
	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// executeCloseShortWithRecord executes close short position and records detailed information
func (at *AutoTrader) executeCloseShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close short: %s", decision.Symbol)

	// Hands-off rule (user directive 2026-09-25): the AI does not manage
	// manually opened positions — closing them is automation acting on them.
	if !at.isAIManaged(decision.Symbol, "short") {
		return fmt.Errorf("❌ [HANDS-OFF] %s was not opened by the AI — manual positions are never closed/adjusted by the program (close the position manually or take it over via a decision to open it)", decision.Symbol)
	}

	// Reference price for the record only. A close must never be blocked by
	// market-data trouble (2026-10-16 review D): lightweight quote first, then
	// the full market data, else record 0 and close anyway.
	closePrice := at.closeReferencePrice(decision.Symbol)
	actionRecord.Price = closePrice

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity - prioritize local database for accurate quantity
	var entryPrice float64
	var quantity float64

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "SHORT"); err == nil && openPos != nil {
			quantity = openPos.Quantity
			entryPrice = openPos.EntryPrice
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Fallback to exchange API if local data not found
	if quantity == 0 {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
					if ep, ok := pos["entryPrice"].(float64); ok {
						entryPrice = ep
					}
					if amt, ok := pos["positionAmt"].(float64); ok {
						quantity = -amt // positionAmt is negative for short
					}
					break
				}
			}
		}
		logger.Infof("  📊 Using exchange position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	// Close position
	at.markCloseIntent(decision.Symbol, "short", "ai_close")
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = close all
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "close_short", quantity, closePrice, 0, entryPrice)

	// 2026-10-20 review area2 #2: a successful POST is not a closed position
	// (a market close can part-fill and expire). Protection state and AI
	// ownership are released only after a FRESH snapshot shows this side flat;
	// a remainder keeps its SL/TP, recorded levels and AI mark so the watchdog
	// and the next decision keep managing it. The per-cycle "position gone"
	// sweep clears the registry once the remainder really disappears.
	if !at.closeConfirmedFlat(decision.Symbol, "short") {
		at.keepUnconfirmedClose(decision.Symbol, "short")
		return nil
	}
	at.ClearRecordedStopLoss(decision.Symbol, "short")
	at.ClearInitialStopLoss(decision.Symbol, "short")
	at.ClearExitMode(decision.Symbol, "short")
	at.ClearPeakPnLCache(decision.Symbol, "short")
	at.unmarkAIManaged(decision.Symbol, "short") // lifecycle complete — registry clean
	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// closeFlatRetryDelay spaces the post-close position re-reads (test seam).
var closeFlatRetryDelay = 400 * time.Millisecond

// closeConfirmedFlat reports whether a fresh (cache-invalidated) position
// snapshot shows NO position on symbol/side. A failed read is "not confirmed":
// the caller must keep protection and ownership rather than assume success.
func (at *AutoTrader) closeConfirmedFlat(symbol, side string) bool {
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(closeFlatRetryDelay)
		}
		at.invalidateExecutionCache()
		positions, err := at.trader.GetPositions()
		if err != nil {
			continue
		}
		held := false
		for _, pos := range positions {
			if pos["symbol"] != symbol || pos["side"] != side {
				continue
			}
			if amt, ok := pos["positionAmt"].(float64); !ok || amt != 0 {
				held = true
				break
			}
		}
		if !held {
			return true
		}
	}
	return false
}

// keepUnconfirmedClose is the "close submitted, flat NOT confirmed" outcome:
// nothing is cleared, the operator is alerted, and the first-seen time is
// seeded from the DB entry (when known) so the per-cycle position-gone sweep
// can still release the AI mark if the remainder later disappears unseen.
func (at *AutoTrader) keepUnconfirmedClose(symbol, side string) {
	posKey := symbol + "_" + side
	if _, exists := at.positionFirstSeenTime[posKey]; !exists {
		if entryTime, _, ok := at.lookupOpenPositionEntry(symbol, side); ok && entryTime > 0 {
			at.positionFirstSeenTime[posKey] = entryTime
		}
	}
	logger.Warnf("⚠️ [%s] %s %s close submitted but the position is NOT confirmed flat (partial fill / stale snapshot) — keeping SL/TP, recorded stop and AI ownership",
		at.name, symbol, side)
	notify.Notify("ALERT", at.name, fmt.Sprintf(
		"<b>⚠️ 平仓未确认归零 %s %s</b>\n<i>平仓单已提交但持仓未确认清空(可能部分成交),已保留保护单与 AI 归属,下周期继续管理</i>",
		notify.Escape(symbol), side))
}

// closeReferencePrice returns the price recorded on a close action without
// depending on full market data (3m/4h klines + stale check): the adapter's
// lightweight quote first, then getMarketData's CurrentPrice, else 0 (the
// close proceeds and the record carries price 0).
func (at *AutoTrader) closeReferencePrice(symbol string) float64 {
	usable := func(p float64) bool { return p > 0 && !math.IsNaN(p) && !math.IsInf(p, 0) }
	price, quoteErr := at.trader.GetMarketPrice(symbol)
	if quoteErr == nil && usable(price) {
		return price
	}
	md, mdErr := at.getMarketData(symbol)
	if mdErr == nil && md != nil && usable(md.CurrentPrice) {
		return md.CurrentPrice
	}
	logger.Warnf("⚠️ [%s] close %s: no reference price (quote err=%v, market data err=%v) — closing anyway, recording price 0",
		at.name, symbol, quoteErr, mdErr)
	return 0
}
