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

// pendingEntry tracks one placed limit-entry order through its lifecycle:
// placed → filled (SL/TP placed at the exact limit price) | expired (N cycles
// unfilled → cancelled) | invalidated (price crossed the SL before entry) |
// replaced (a newer decision for the same symbol and side).
// pendingEntry is the in-memory limit-entry plan. Concurrency contract:
// MAP MEMBERSHIP is guarded by pendingEntriesMu; FIELD MUTATIONS (Cycles,
// ExecutedQty/ProtectedQty watermark) happen only under the account
// execution mutex (cycle, protection monitor, Stop sweep, reconcile).
type pendingEntry struct {
	RecoveryReason string // durable abort state; cleared only after terminal order and flat position
	Symbol         string
	Side           string // "long" / "short"
	Price          float64
	Quantity       float64
	StopLoss       float64
	TakeProfit     float64
	Leverage       int
	OrderID        string
	PlacedAt       time.Time
	Cycles         int
	ExecutedQty    float64 // cumulative observed fill, independent of verified coverage
	ProtectedQty   float64 // executed size already carrying SL/TP (partial-fill watermark)
	ExitMode       string  // exit template carried to the position on fill (trend|range|quick; '' = trend)
}

func pendingEntryKey(symbol, side string) string { return symbol + "|" + side }

func (at *AutoTrader) setPendingEntry(pe *pendingEntry) {
	registerAccountTrader(at)
	at.pendingEntriesMu.Lock()
	defer at.pendingEntriesMu.Unlock()
	if at.pendingEntries == nil {
		at.pendingEntries = make(map[string]*pendingEntry)
	}
	at.pendingEntries[pendingEntryKey(pe.Symbol, pe.Side)] = pe
	at.persistPendingEntry(pe)
}

func (at *AutoTrader) dropPendingEntry(symbol, side string) {
	at.pendingEntriesMu.Lock()
	delete(at.pendingEntries, pendingEntryKey(symbol, side))
	at.pendingEntriesMu.Unlock()
	// The shadow row must die with the map state — a surviving row would make
	// the next startup re-claim an order that is already gone.
	if at.store != nil {
		if err := at.store.PendingEntry().Delete(at.id, symbol, side); err != nil {
			logger.Infof("⚠️ [%s] drop pending entry %s: shadow row delete failed: %v", at.name, symbol, err)
		}
	}
}

// persistPendingEntry is the write-through half of the pending state: the row
// is what a restart uses to re-claim the resting order (expiry management +
// protective placement on fill). A failed write means the entry could rest
// unowned after a restart — the startup tag-scan catches it as a fallback.
func (at *AutoTrader) persistPendingEntry(pe *pendingEntry) {
	if at.store == nil {
		return
	}
	err := at.store.PendingEntry().Upsert(&store.PendingEntryDB{
		TraderID:       at.id,
		RecoveryReason: pe.RecoveryReason,
		Symbol:         pe.Symbol,
		Side:           pe.Side,
		Price:          pe.Price,
		Quantity:       pe.Quantity,
		StopLoss:       pe.StopLoss,
		TakeProfit:     pe.TakeProfit,
		Leverage:       pe.Leverage,
		OrderID:        pe.OrderID,
		PlacedAt:       pe.PlacedAt,
		ExitMode:       pe.ExitMode,
		ProtectedQty:   pe.ProtectedQty,
		ExecutedQty:    pe.ExecutedQty,
	})
	if err != nil {
		logger.Infof("⚠️ [%s] persist pending entry %s (order %s) failed: %v — restart would orphan it until the tag-scan drops it",
			at.name, pe.Symbol, pe.OrderID, err)
	}
}

func (at *AutoTrader) getPendingEntry(symbol, side string) *pendingEntry {
	at.pendingEntriesMu.RLock()
	defer at.pendingEntriesMu.RUnlock()
	return at.pendingEntries[pendingEntryKey(symbol, side)]
}

func (at *AutoTrader) cancelPendingSide(symbol, side string) error {
	if pe := at.getPendingEntry(symbol, side); pe != nil {
		return at.cancelPending(pe)
	}
	return nil
}

// validateLimitEntryPrice sanity-checks the trigger price against the live
// market: buy limits sit below (pullback), sell limits above (rally), within a
// 0.1%-5% band — outside it the order is either a disguised market order or a
// stale far-away one.
func validateLimitEntryPrice(action string, limitPrice, livePrice float64) error {
	if limitPrice <= 0 || livePrice <= 0 {
		return fmt.Errorf("invalid prices: limit %.6g live %.6g", limitPrice, livePrice)
	}
	var distPct float64
	if action == "open_long_limit" {
		distPct = (livePrice - limitPrice) / livePrice * 100
	} else {
		distPct = (limitPrice - livePrice) / livePrice * 100
	}
	if distPct < 0.1 {
		return fmt.Errorf("trigger price %.6g is at/above market %.6g — use the market action for immediate entries", limitPrice, livePrice)
	}
	if distPct > 5 {
		return fmt.Errorf("trigger price %.6g is %.1f%% away from market %.6g — too far, re-evaluate when price approaches", limitPrice, distPct, livePrice)
	}
	return nil
}

// limitEntryTooFar reports the stale far-away half of the trigger band: the
// anchor sits more than 5% from market on the entry side. An uncrossed
// far-away anchor must be rejected, not parked — it can only fill after an
// un-modeled 5%+ move in the entry direction, i.e. the plan is already wrong.
func limitEntryTooFar(action string, limitPrice, livePrice float64) bool {
	if limitPrice <= 0 || livePrice <= 0 {
		return true // invalid prices: nothing placeable to park
	}
	var distPct float64
	if action == "open_long_limit" {
		distPct = (livePrice - limitPrice) / livePrice * 100
	} else {
		distPct = (limitPrice - livePrice) / livePrice * 100
	}
	return distPct > 5
}

// limitEntryMarketFallbackEnabled reports whether a limit entry whose anchor
// has been crossed by the live price converts to a market entry. Default on:
// the anchor is pre-computed at prompt-build time and the AI latency window
// (minutes) lets price drift through it — a crossed anchor means the planned
// pullback/rally already arrived, which is exactly when the order was meant
// to fill, not a reason to reject.
func (at *AutoTrader) limitEntryMarketFallbackEnabled() bool {
	if at.config.StrategyConfig == nil || at.config.StrategyConfig.RiskControl.LimitEntryMarketFallback == nil {
		return true
	}
	return *at.config.StrategyConfig.RiskControl.LimitEntryMarketFallback
}

// anchorCrossedByMarket reports whether the live price has crossed the limit
// anchor from the entry side: long anchor at/above live (the pullback came in
// deeper than planned), short anchor at/below live. slCrossed additionally
// flags the setup as dead — the market traded past the planned stop before
// the entry could fill. A zero SL can't be crossed; the missing-SL rule is
// validateOpenRisk's, not this check's.
func anchorCrossedByMarket(side string, anchorPrice, livePrice, stopLoss float64) (crossed, slCrossed bool) {
	if stopLoss <= 0 {
		return false, false
	}
	if side == "long" {
		crossed = anchorPrice >= livePrice
		slCrossed = crossed && livePrice <= stopLoss
	} else {
		crossed = anchorPrice <= livePrice
		slCrossed = crossed && livePrice >= stopLoss
	}
	return crossed, slCrossed
}

// executeOpenLimitLongWithRecord places a GTC limit entry and registers the
// pending state. All risk gates anchor at the LIMIT price — the exact price
// the order fills at, eliminating market-order RR skew.
func (at *AutoTrader) executeOpenLimitLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	return at.executeOpenLimit(decision, actionRecord, "long")
}

func (at *AutoTrader) executeOpenLimitShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	return at.executeOpenLimit(decision, actionRecord, "short")
}

func (at *AutoTrader) executeOpenLimit(decision *kernel.Decision, actionRecord *store.DecisionAction, side string) error {
	if err := at.entryExecutionBlocked(decision.Symbol, side); err != nil {
		return err
	}
	logger.Infof("  📌 Open %s (LIMIT): %s @ %.6g", side, decision.Symbol, decision.Price)

	grid, ok := at.trader.(interface {
		PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error)
		CancelOrder(symbol, orderID string) error
	})
	if !ok {
		return fmt.Errorf("exchange %s does not support limit entries", at.exchange)
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}
	// Slot accounting: open positions + resting limit entries on OTHER
	// symbols + this order. Resting entries must occupy slots too, otherwise
	// several unfilled limits could all fill into more positions than the cap.
	if err := at.enforceMaxPositions(nextSlotCount(len(positions), at.snapshotPendingKeys(), pendingEntryKey(decision.Symbol, side))); err != nil {
		return err
	}
	posSide := "long"
	if side == "short" {
		posSide = "short"
	}
	for _, pos := range positions {
		if pos["symbol"] == decision.Symbol && pos["side"] == posSide {
			return fmt.Errorf("❌ %s already has %s position", decision.Symbol, posSide)
		}
	}

	marketData, err := at.getMarketData(decision.Symbol)
	if err != nil {
		return err
	}
	livePrice := marketData.CurrentPrice
	at.stampEntryPath(actionRecord, marketData)

	// Spread gate: a wide book eats the anchor edge — the limit would sit
	// inside the spread and never fill at value.
	if blocked, reason := at.spreadBlocksOpen(decision.Symbol); blocked {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %s", decision.Action, decision.Symbol, reason)
	}

	// Trigger price sanity: pullback/rally band. When the live price has
	// already crossed the anchor (the AI latency window moved the market
	// through the pre-computed trigger), the pullback/rally the anchor was
	// waiting for has arrived — convert to a market entry instead of
	// rejecting, unless the stop was crossed too (setup dead) or the
	// fallback is disabled. An uncrossed failure only survives when the
	// anchor is a tick from market (the pullback is on its final leg and the
	// GTC fills at once); a far-away anchor is stale and is rejected here —
	// the old fall-through placed it anyway (review 2026-09-09).
	if limitErr := validateLimitEntryPrice("open_"+side+"_limit", decision.Price, livePrice); limitErr != nil {
		crossed, slCrossed := anchorCrossedByMarket(side, decision.Price, livePrice, decision.StopLoss)
		if crossed {
			if slCrossed {
				return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: anchor %.6g already crossed (live %.6g) and price is at/beyond SL %.6g — setup invalidated",
					decision.Action, decision.Symbol, decision.Price, livePrice, decision.StopLoss)
			}
			// A degraded decision (market open rewritten to the anchor limit
			// for lacking exception evidence) must not ride this conversion
			// back into a market order — that would silently restore the
			// market chase the gate just refused (B1, QUANT_REVIEW 09-22).
			// The anchor is already at/beyond the live price, so resting it
			// would fill immediately = a market order by another name.
			if decision.MarketDegraded {
				return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: degraded market open's anchor %.6g already crossed (live %.6g) and no exception evidence — not converting back to market",
					decision.Action, decision.Symbol, decision.Price, livePrice)
			}
			if at.limitEntryMarketFallbackEnabled() {
				logger.Infof("  🔄 %s anchor %.6g crossed by market %.6g — converting to market entry", decision.Symbol, decision.Price, livePrice)
				// The market paths don't manage pending state (only the
				// open_long/open_short dispatch does), so drop a replaced
				// resting order here — otherwise it would survive next to
				// the fresh market position.
				if old := at.getPendingEntry(decision.Symbol, side); old != nil {
					if err := at.cancelPendingForReplacement(old); err != nil {
						return fmt.Errorf("cannot cancel replaced limit entry for %s %s: %w", old.Symbol, old.Side, err)
					}
					logger.Infof("  🔄 Cancelled replaced pending limit entry for %s (@ %.6g)", old.Symbol, old.Price)
				}
				if side == "long" {
					return at.executeOpenLongWithRecord(decision, actionRecord)
				}
				return at.executeOpenShortWithRecord(decision, actionRecord)
			}
			return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %v", decision.Action, decision.Symbol, limitErr)
		}
		if limitEntryTooFar("open_"+side+"_limit", decision.Price, livePrice) {
			return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %v", decision.Action, decision.Symbol, limitErr)
		}
	}

	// Supply-zone gate: an anchor parked right at the opposite-side structure
	// (below resistance for longs / above support for shorts) has no room —
	// reject so the model re-anchors or skips the setup.
	if at.config.StrategyConfig != nil && at.config.StrategyConfig.RiskControl.OpenRejectSupplyPct > 0 {
		rc := at.config.StrategyConfig.RiskControl
		offsetCfg := kernel.AnchorOffsetFromRiskControl(&rc)
		sig, sigErr := kernel.ComputeSymbolSignals(decision.Symbol, marketData, kernel.SignalOptions{
			Now:                     time.Now(),
			PrimaryTF:               "15m",
			CurrentPrice:            livePrice,
			LimitEntryOffsetPct:     rc.LimitEntryOffsetPct,
			LimitEntryOffsetMode:    rc.LimitEntryOffsetMode,
			LimitEntryOffsetATRMult: rc.LimitEntryOffsetATRMult,
			LimitEntryOffsetMinPct:  rc.LimitEntryOffsetMinPct,
			LimitEntryOffsetMaxPct:  rc.LimitEntryOffsetMaxPct,
			PumpGuard4hPct:          kernel.PumpGuard4h(&rc),
		})
		if sigErr == nil && sig != nil {
			// Same execution-TF-scaled breathing threshold the prompt-side
			// suppression used (the fill must not land right under a ceiling)
			// — recomputed on fresh data so the gate and the pre-filter can
			// never disagree about what "no room" means.
			threshold := kernel.AnchorBreathingPct(sig.ExecutionATRPct(), offsetCfg, rc.OpenRejectSupplyPct)
			// Breakout-retest exemption (2026-10-03 review): the decision price
			// at the pullback level must not die on the breakout's own extension
			// pivots — the level comes from the cycle's captured gate state.
			pullbackLevel := 0.0
			if gs := at.cycleGateStates[market.Normalize(decision.Symbol)]; gs != nil && gs.LongPullbackActive {
				pullbackLevel = gs.LongPullbackLevel
			}
			if blocked, reason := entrySupplyZoneBlocks(sig, side, decision.Price, threshold, pullbackLevel); blocked {
				return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %s", decision.Action, decision.Symbol, reason)
			}
		}
	}

	// All risk gates anchor at the LIMIT price — the exact fill price.
	floorATR, capATR := at.stopBandATRs(decision.Symbol, marketData)
	if err := at.validateOpenRisk(decision, decision.Price, floorATR, capATR); err != nil {
		return err
	}

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	equity := 0.0
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		equity = eq
	} else if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		equity = eq
	} else if avail, ok := balance["availableBalance"].(float64); ok {
		equity = avail
	}

	adjusted, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjusted
	}
	decision.PositionSizeUSD = at.clampSizeToRisk(decision, decision.PositionSizeUSD, equity, decision.Price)
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Margin-budget gate: (used + new) margin ≤ max_margin_usage × equity —
	// the prompt states the budget, this enforces it (audit 09-13 #2).
	if blocked, reason := at.marginBudgetBlocksOpen(decision.Symbol, side, decision.PositionSizeUSD, float64(decision.Leverage), equity); blocked {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: %s", decision.Action, decision.Symbol, reason)
	}
	quantity := decision.PositionSizeUSD / decision.Price
	actionRecord.Quantity = quantity
	actionRecord.Price = decision.Price

	if err := at.trader.SetMarginMode(decision.Symbol, at.entryUsesCrossMargin()); err != nil {
		return fmt.Errorf("❌ [RISK CONTROL] %s %s rejected: margin mode not verified: %w", decision.Action, decision.Symbol, err)
	}

	// Replace only this side; an opposite-side limit entry remains owned.
	if old := at.getPendingEntry(decision.Symbol, side); old != nil {
		if err := at.cancelPendingForReplacement(old); err != nil {
			return fmt.Errorf("cannot cancel replaced limit entry for %s %s: %w", old.Symbol, old.Side, err)
		}
		logger.Infof("  🔄 Replaced pending limit entry for %s (@ %.6g)", old.Symbol, old.Price)
	}

	sideStr := "BUY"
	posSideStr := "LONG"
	if side == "short" {
		sideStr, posSideStr = "SELL", "SHORT"
	}
	res, err := grid.PlaceLimitOrder(&types.LimitOrderRequest{
		Symbol:       decision.Symbol,
		Side:         sideStr,
		PositionSide: posSideStr,
		Price:        decision.Price,
		Quantity:     quantity,
		Leverage:     decision.Leverage,
		ClientID:     at.entryClientID(),
	})
	if err != nil {
		return fmt.Errorf("failed to place limit entry: %w", err)
	}
	placedQuantity := res.Quantity
	if placedQuantity <= 0 {
		// Some exchange adapters predate the normalized result quantity. Keep
		// their prior behavior while Binance returns its actual quantized size.
		placedQuantity = quantity
	}
	actionRecord.Quantity = placedQuantity

	at.setPendingEntry(&pendingEntry{
		Symbol: decision.Symbol, Side: side, Price: decision.Price,
		Quantity: placedQuantity, StopLoss: decision.StopLoss, TakeProfit: decision.TakeProfit,
		Leverage: decision.Leverage, OrderID: res.OrderID, PlacedAt: time.Now(),
		ExitMode: decision.ExitMode,
	})
	actionRecord.OrderID = 0                // string order id lives in the pending state
	actionRecord.EntryOrderID = res.OrderID // quality-bucket exact join (review 2026-10-08 H)
	logger.Infof("  ✓ Limit entry placed: %s %s %.6g @ %.6g (order %s), SL %.6g / TP %.6g",
		decision.Symbol, side, placedQuantity, decision.Price, res.OrderID, decision.StopLoss, decision.TakeProfit)
	return nil
}

// snapshotPendingKeys returns the symbol+side keys currently holding a resting
// limit entry in the pending map (copy under lock).
func (at *AutoTrader) snapshotPendingKeys() []string {
	at.pendingEntriesMu.RLock()
	defer at.pendingEntriesMu.RUnlock()
	syms := make([]string, 0, len(at.pendingEntries))
	for sym := range at.pendingEntries {
		syms = append(syms, sym)
	}
	return syms
}

// nextSlotCount computes the count after placing a new entry, replacing only
// a resting entry for the same symbol+side.
func nextSlotCount(openPositions int, pendingKeys []string, key string) int {
	n := openPositions + 1
	for _, s := range pendingKeys {
		if s != key {
			n++
		}
	}
	return n
}

// processPendingEntries runs once per decision cycle: finalizes fills
// (SL/TP keep their structural prices), cancels expired or invalidated
// orders, and drops externally-cancelled ones.
// processPendingEntries runs the pending-entry lifecycle. directionRecheck:
// the fresh-gate re-validation (micro-trend / consensus / absolute bans) is
// a CYCLE-BOUNDARY concern (user decision 2026-10-08, option ②) — the 30s
// protection pass passes false and keeps only the fast paths (account halt,
// fills, SL-crossed invalidation, lifetime expiry, recovery). Re-validating
// direction at 30s granularity killed placements on the first 15m wobble:
// 6/15 cancelled at 21s-9m in the 10-07/10-08 sample, zero time to fill.
func (at *AutoTrader) processPendingEntries(directionRecheck bool) {
	if len(at.accountPendingEntries()) > 0 {
		if reason := at.pendingAccountHaltReason(); reason != "" {
			at.cancelAccountPendingRisk(reason)
		}
	} else {
		at.clearPendingProtectionFaults()
	}
	at.pendingEntriesMu.Lock()
	symbols := make([]string, 0, len(at.pendingEntries))
	for sym := range at.pendingEntries {
		symbols = append(symbols, sym)
	}
	snapshot := make(map[string]*pendingEntry, len(at.pendingEntries))
	for sym, pe := range at.pendingEntries {
		snapshot[sym] = pe
	}
	at.pendingEntriesMu.Unlock()
	if len(snapshot) == 0 {
		return
	}

	maxCycles := at.limitEntryMaxCycles()
	lifetime := limitEntryLifetime(maxCycles, at.config.ScanInterval)

	for _, sym := range symbols {
		pe := snapshot[sym]
		status, err := at.trader.GetOrderStatus(pe.Symbol, pe.OrderID)
		if err != nil {
			logger.Infof("📌 [%s] Limit entry %s status check failed: %v", at.name, pe.Symbol, err)
			continue
		}
		if pe.RecoveryReason != "" {
			if at.recoverRejectedPendingFill(pe, status) {
				at.dropPendingEntry(pe.Symbol, pe.Side)
			}
			continue
		}
		st, _ := status["status"].(string)
		if directionRecheck && (strings.EqualFold(st, "NEW") || strings.EqualFold(st, "PARTIALLY_FILLED")) {
			if reason := at.pendingDirectionBlocked(pe); reason != "" {
				// Incident 2026-10-07: successful cancels used to be silent —
				// only the FAILURE path logged. Without this line the 29
				// same-second cancellations could not be attributed to their
				// first triggering code per order.
				age := time.Since(pe.PlacedAt).Round(time.Second)
				logger.Infof("🗑️ [%s] Cancelling limit entry %s %s (id %s, age %s): %s",
					at.name, pe.Side, pe.Symbol, pe.OrderID, age, reason)
				if err := at.cancelPending(pe); err != nil {
					logger.Warnf("%s: %v", reason, err)
				}
				continue
			}
		}
		switch strings.ToUpper(st) {
		case "FILLED":
			// Validate the real fill against fixed structural levels. Recovery
			// remains durable until coverage or a completed exit is observed.
			at.protectExecutedSlice(pe, status)
			at.reportFilledRR(pe, statusFloat(status, "avgPrice"))
			at.markAIManaged(pe.Symbol, pe.Side, pe.OrderID)
			// protectExecutedSlice reconciles cumulative coverage once, including
			// partial fills. Never discard an unverified protection/abort plan.
			if executed := statusFloat(status, "executedQty"); !pendingProtectionComplete(pe, status) {
				logger.Infof("⚠️ [%s] Limit entry FILLED %s %s but protection incomplete (%.6g/%.6g protected) — pending row KEPT, legs retried next cycle", at.name, pe.Symbol, pe.Side, pe.ProtectedQty, executed)
				notify.Notify("ALERT", at.name, fmt.Sprintf(
					"<b>⚠️ 限价成交保护未完成 %s</b>\n<i>%s 已成交 %.6g,已保护 %.6g —— 恢复计划保留,下周期重试补挂;若持续失败请人工核查</i>",
					notify.Escape(pe.Symbol), pe.Side, executed, pe.ProtectedQty))
				continue
			}
			at.dropPendingEntry(pe.Symbol, pe.Side)
			logger.Infof("✅ [%s] Limit entry FILLED: %s %s @ %.6g — protective orders anchored", at.name, pe.Symbol, pe.Side, pe.Price)
			notify.Notify("ORDER", at.name, fmt.Sprintf("<b>📌 限价入场成交 %s</b>\n<i>%s @ %.6g,保护单已挂</i>", notify.Escape(pe.Symbol), pe.Side, pe.Price))
		case "CANCELED", "EXPIRED", "REJECTED":
			// Residual protection (P1 2026-09-25): a cancel/expiry after a
			// partial fill must not strand the executed slice on ATR-fallback.
			at.protectExecutedSlice(pe, status)
			if !pendingProtectionComplete(pe, status) {
				continue
			}
			at.dropPendingEntry(pe.Symbol, pe.Side)
			logger.Infof("📌 [%s] Limit entry %s %s externally (%s) — state dropped (executed slice protected if any)", at.name, pe.Symbol, st, st)
		case "PARTIALLY_FILLED":
			// P1 (2026-09-25): protect the executed slice NOW at the actual
			// avg price — a partial used to sit unprotected until the next
			// cycle boundary, and a later cancel degraded it to ATR-fallback.
			pe.Cycles++
			at.protectExecutedSlice(pe, status)
			// Remaining size keeps the normal pending lifecycle (SL-crossed
			// invalidation + lifetime expiry), minus the protected watermark.
			if md, err := at.getMarketData(pe.Symbol); err == nil && md.CurrentPrice > 0 {
				invalid := (pe.Side == "long" && md.CurrentPrice <= pe.StopLoss) ||
					(pe.Side == "short" && md.CurrentPrice >= pe.StopLoss)
				if invalid {
					// R5: only drop the pending state when the exchange order
					// is ACTUALLY cancelled — otherwise the resting order can
					// still fill into an untracked position.
					if cerr := at.cancelPending(pe); cerr != nil {
						logger.Infof("⚠️ [%s] Limit entry %s invalidation cancel FAILED (%v) — kept pending, retried next cycle", at.name, pe.Symbol, cerr)
						continue
					}
					at.dropPendingEntry(pe.Symbol, pe.Side)
					logger.Infof("📌 [%s] Limit entry %s invalidated after partial fill: remaining slice cancelled, executed slice keeps its protection", at.name, pe.Symbol)
					continue
				}
			}
			if age := time.Since(pe.PlacedAt); age >= lifetime {
				if cerr := at.cancelPending(pe); cerr != nil {
					logger.Infof("⚠️ [%s] Limit entry %s expiry cancel FAILED (%v) — kept pending, retried next cycle", at.name, pe.Symbol, cerr)
					continue
				}
				logger.Infof("📌 [%s] Limit entry %s expired after partial fill: remaining slice cancelled, executed slice keeps its protection", at.name, pe.Symbol)
				at.dropPendingEntry(pe.Symbol, pe.Side)
			}
		default: // NEW
			pe.Cycles++
			// Invalidation: price crossed the SL before entry — setup is dead.
			if md, err := at.getMarketData(pe.Symbol); err == nil && md.CurrentPrice > 0 {
				invalid := (pe.Side == "long" && md.CurrentPrice <= pe.StopLoss) ||
					(pe.Side == "short" && md.CurrentPrice >= pe.StopLoss)
				if invalid {
					if cerr := at.cancelPending(pe); cerr != nil {
						logger.Infof("⚠️ [%s] Limit entry %s invalidation cancel FAILED (%v) — kept pending, retried next cycle", at.name, pe.Symbol, cerr)
						continue
					}
					logger.Infof("📌 [%s] Limit entry %s invalidated: price crossed SL before entry", at.name, pe.Symbol)
					continue
				}
			}
			if age := time.Since(pe.PlacedAt); age >= lifetime {
				if cerr := at.cancelPending(pe); cerr != nil {
					logger.Infof("⚠️ [%s] Limit entry %s expiry cancel FAILED (%v) — kept pending, retried next cycle", at.name, pe.Symbol, cerr)
					continue
				}
				logger.Infof("📌 [%s] Limit entry %s expired after %s (lifetime max(30min, %d×%v)) — cancelled for re-evaluation", at.name, pe.Symbol, age.Round(time.Second), maxCycles, at.config.ScanInterval)
			} else {
				logger.Infof("📌 [%s] Limit entry %s pending (%d/%d cycles, age %s of %s)", at.name, pe.Symbol, pe.Cycles, maxCycles, age.Round(time.Second), lifetime)
			}
		}
	}
}

// reportFilledRR logs (and on a badly-degraded setup alerts) the realized
// reward:risk of a just-filled limit entry, priced at the ACTUAL fill price
// against the fixed structural SL/TP (F21b, 2026-10-01 review: the old
// planned-price version could never detect degradation because the entry
// gates validated RR at that same planned price). Alert threshold: realized
// RR below half the configured min_rr — the setup aged badly between
// placement and fill; anything above that is normal aging noise.
func (at *AutoTrader) reportFilledRR(pe *pendingEntry, fillPrice float64) {
	if pe.StopLoss <= 0 || pe.TakeProfit <= 0 || pe.Price <= 0 {
		return
	}
	if fillPrice <= 0 {
		fillPrice = pe.Price // no confirmed fill price — fall back to the plan
	}
	newSL, newTP := pe.StopLoss, pe.TakeProfit
	minRR := 1.5
	if at.config.StrategyConfig != nil {
		if v := at.config.StrategyConfig.RiskControl.MinRiskRewardRatio; v > 0 {
			minRR = v
		}
	}
	risk := math.Abs(fillPrice - newSL)
	reward := math.Abs(newTP - fillPrice)
	if risk <= 0 {
		return
	}
	rr := reward / risk
	if rr < minRR*0.5 {
		logger.Warnf("⚠️ [%s] %s %s filled @ %.6g with realized RR %.2f vs min_rr %.2f — setup aged badly since placement; structural protection remains fixed; invalid fills enter recovery",
			at.name, pe.Symbol, pe.Side, fillPrice, rr, minRR)
		notify.Notify("ALERT", at.name, fmt.Sprintf(
			"<b>⚠️ 限价成交 RR 劣化 %s</b>\n%s @ %.6g,成交 RR <code>%.2f</code>(min_rr %.2f 的一半)——挂单期间 setup 已老化,结构保护价位保持不变，不合格成交会进入退出恢复",
			notify.Escape(pe.Symbol), pe.Side, fillPrice, rr, minRR))
	}
}

// cancelPending cancels the exchange order and drops the pending state.
func (at *AutoTrader) cancelPending(pe *pendingEntry) error {
	_, err := at.cancelPendingWithExecuted(pe)
	return err
}

// review 2026-10-11: a cancel can race a fill after the position check;
// that slice now owns the position, so neither limit nor market replacement
// may add another entry. Other cancellation callers keep their prior behavior.
func (at *AutoTrader) cancelPendingForReplacement(pe *pendingEntry) error {
	executed, err := at.cancelPendingWithExecuted(pe)
	if err != nil {
		return err
	}
	if executed > 0 {
		err := fmt.Errorf("old limit entry filled during replacement — new entry aborted; position now owned by the filled slice (%s %s, executed %.6g)", pe.Symbol, pe.Side, executed)
		logger.Warnf("%v", err)
		return err
	}
	return nil
}

func (at *AutoTrader) cancelPendingWithExecuted(pe *pendingEntry) (float64, error) {
	grid, ok := at.trader.(interface {
		CancelOrder(symbol, orderID string) error
	})
	if !ok {
		return pe.ExecutedQty, fmt.Errorf("exchange does not support order cancellation")
	}
	cancelErr := grid.CancelOrder(pe.Symbol, pe.OrderID)
	if cancelErr != nil {
		logger.Warnf("Cancel pending %s: %v", pe.Symbol, cancelErr)
	}
	// F06c (2026-10-01 review): a fill can land between the cycle's last
	// status poll and this cancel succeeding. Re-query once and protect any
	// executed slice BEFORE dropping the plan (protectExecutedSlice is
	// idempotent on the watermark). On Binance a cancel of a fully-filled
	// order can error; the terminal receipt and verified coverage still
	// determine the final cancellation result.
	status, err := at.trader.GetOrderStatus(pe.Symbol, pe.OrderID)
	if err != nil {
		return pe.ExecutedQty, fmt.Errorf("cancel accepted, final fill unknown: %w", err)
	}
	executed := math.Max(pe.ExecutedQty, statusFloat(status, "executedQty"))
	at.protectExecutedSlice(pe, status)
	st, _ := status["status"].(string)
	switch strings.ToUpper(st) {
	case "CANCELED", "EXPIRED", "REJECTED", "FILLED":
	default:
		return executed, fmt.Errorf("cancel not terminal: %s", st)
	}
	if !pendingProtectionComplete(pe, status) {
		return executed, fmt.Errorf("cancel accepted, residual protection incomplete")
	}
	at.dropPendingEntry(pe.Symbol, pe.Side)
	notify.Notify("ORDER", at.name, fmt.Sprintf("<b>限价单终态已确认 %s</b>\n已成交 %.6g，残量保护已核验", notify.Escape(pe.Symbol), statusFloat(status, "executedQty")))
	return executed, nil
}

// review 2026-10-08 D: live and shadow entries share the configured cycle limit.
func (at *AutoTrader) limitEntryMaxCycles() int {
	if at.config.StrategyConfig != nil && at.config.StrategyConfig.RiskControl.LimitEntryMaxCycles > 0 {
		return at.config.StrategyConfig.RiskControl.LimitEntryMaxCycles
	}
	return 3
}

// limitEntryLifetime resolves a limit order's unfilled lifetime: the older
// rule expired orders after N decision cycles, but cycles vary in length
// when the scan interval changes, so "3 cycles" could mean 9 or 60 minutes.
// The binding lifetime is max(30min, N × scan interval) — short-enough
// cycles still get the 30-minute minimum, longer ones scale the window.
func limitEntryLifetime(maxCycles int, scanInterval time.Duration) time.Duration {
	if maxCycles <= 0 {
		maxCycles = 3
	}
	if scanInterval <= 0 {
		scanInterval = 5 * time.Minute
	}
	lifetime := time.Duration(maxCycles) * scanInterval
	if min := 30 * time.Minute; lifetime < min {
		return min
	}
	return lifetime
}

// statusFloat reads a float64 out of an exchange order-status map whose
// numeric fields may arrive as strings.
func statusFloat(status map[string]interface{}, key string) float64 {
	switch v := status[key].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// protectExecutedSlice places SL/TP for the executed portion of a limit
// entry at the ACTUAL average fill price (P1, 2026-09-25): partial fills
// used to sit unprotected until the next full-fill/next-cycle path, and a
// cancel degraded them to ATR-fallback protection. Idempotent on the
// protected-qty watermark in the pending entry. Returns the structural stop
// that was recorded (0 = nothing placed).
func (at *AutoTrader) protectExecutedSlice(pe *pendingEntry, status map[string]interface{}) float64 {
	executed, avg, valid := fillReceipt(status)
	if !valid || executed <= 0 || avg <= 0 || executed+1e-9 < pe.ExecutedQty {
		return 0
	}
	previouslyExecuted := pe.ExecutedQty
	pe.ExecutedQty = executed
	previouslyProtected := pe.ProtectedQty
	// Confirmed coverage is revocable: an exchange-side cancel after an earlier
	// slice must not let the old watermark clean up an unprotected terminal fill.
	pe.ProtectedQty = 0
	defer func() {
		if pe.ProtectedQty == 0 && (previouslyProtected > 0 || pe.ExecutedQty != previouslyExecuted) {
			at.persistPendingEntry(pe)
		}
	}()
	if pe.RecoveryReason == "" {
		d := &kernel.Decision{Symbol: pe.Symbol, Action: "open_" + pe.Side, StopLoss: pe.StopLoss, TakeProfit: pe.TakeProfit, Leverage: pe.Leverage}
		if err := at.actualFillRisk(d, avg, executed, 0); err != nil {
			pe.RecoveryReason = err.Error()
			at.persistPendingEntry(pe)
		}
	}
	if pe.RecoveryReason != "" {
		at.recoverRejectedPendingFill(pe, status)
		return 0
	}
	newSL, newTP := pe.StopLoss, pe.TakeProfit
	recordedSL := at.GetRecordedStopLoss(pe.Symbol, pe.Side)
	// review 2026-10-11: later fills must extend coverage without undoing
	// breakeven/trailing/AI tightening; the opening-risk anchor stays the plan.
	if recordedSL > 0 && !math.IsNaN(recordedSL) && !math.IsInf(recordedSL, 0) &&
		(newSL <= 0 || (pe.Side == "long" && recordedSL > newSL) || (pe.Side == "short" && recordedSL < newSL)) {
		newSL = recordedSL
	}
	if newSL <= 0 || newTP <= 0 {
		return 0
	}
	at.markAIManaged(pe.Symbol, pe.Side, pe.OrderID)
	posKey := pe.Symbol + "_" + pe.Side
	if recordedSL <= 0 {
		// First slice of this entry: the 1R anchor and the recorded stop are
		// the REAL opening risk (actual fill), not the plan's limit price.
		at.SetRecordedStopLoss(pe.Symbol, pe.Side, newSL)
		at.SetInitialStopLoss(pe.Symbol, pe.Side, pe.StopLoss)
		at.SetExitMode(pe.Symbol, pe.Side, pe.ExitMode)
		at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
		at.ClearPeakPnLCache(pe.Symbol, pe.Side)
	}
	positionSide := "LONG"
	if pe.Side == "short" {
		positionSide = "SHORT"
	}
	slice := executed // reconcile cumulative coverage, including prior partial fills
	// R2: the watermark advances ONLY when both legs actually placed — a
	// failed placement must not mark the slice protected (the same size
	// would be skipped forever). F01 (2026-10-01 review): that now includes
	// the TP leg — the old SL-only gate let a TP rejection advance the
	// watermark and report the slice protected with no profit leg. Duplicate
	// placement on retry is prevented by the retry-safe leg skip inside
	// placeProtectiveOrders.
	slErr, tpErr := at.placeProtectiveOrders(&kernel.Decision{
		Symbol: pe.Symbol, Action: "open_" + pe.Side,
		StopLoss: newSL, TakeProfit: newTP, ExitMode: pe.ExitMode,
	}, positionSide, slice, pe.Price, avg)
	if slErr != nil || tpErr != nil {
		logger.Infof("⚠️ [%s] partial-fill protection FAILED for %s %s (SL: %v, TP: %v) — watermark NOT advanced, retried next cycle", at.name, pe.Symbol, pe.Side, slErr, tpErr)
		return 0
	}
	pe.ProtectedQty = executed
	at.persistPendingEntry(pe)
	if executed <= previouslyProtected {
		return newSL
	}
	logger.Infof("📌 [%s] Partial-fill protection: %s %s executed %.6g @ %.6g — SL %.6g / TP %.6g kept at structural levels",
		at.name, pe.Symbol, pe.Side, executed, avg, newSL, newTP)
	notify.Notify("ORDER", at.name, fmt.Sprintf(
		"<b>📌 部分成交保护 %s</b>\n<i>已成交 %.6g @ %.6g,SL/TP 保留原始结构价位(%.6g / %.6g)</i>",
		notify.Escape(pe.Symbol), executed, avg, newSL, newTP))
	return newSL
}

func pendingProtectionComplete(pe *pendingEntry, status map[string]interface{}) bool {
	executed, _, valid := fillReceipt(status)
	if !valid || executed+1e-9 < pe.ExecutedQty {
		return false
	}
	st, _ := status["status"].(string)
	switch strings.ToUpper(st) {
	case "CANCELED", "EXPIRED", "REJECTED", "FILLED":
	default:
		return false
	}
	if executed == 0 {
		return !strings.EqualFold(st, "FILLED")
	}
	return pe.ProtectedQty+1e-9 >= executed
}

// Distinguish a real zero fill from a missing, malformed or non-finite receipt.
func fillReceipt(status map[string]interface{}) (quantity, average float64, valid bool) {
	number := func(key string) (float64, bool) {
		var n float64
		switch v := status[key].(type) {
		case float64:
			n = v
		case string:
			var err error
			n, err = strconv.ParseFloat(v, 64)
			if err != nil {
				return 0, false
			}
		default:
			return 0, false
		}
		return n, !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0
	}
	var ok bool
	quantity, ok = number("executedQty")
	if !ok {
		return 0, 0, false
	}
	if quantity > 0 {
		average, ok = number("avgPrice")
		if !ok || average <= 0 {
			return quantity, 0, false
		}
	}
	return quantity, average, true
}
