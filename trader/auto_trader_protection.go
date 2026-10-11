package trader

import (
	"fmt"
	"math"
	"strings"
	"time"

	"nofx/logger"
	notify "nofx/telegram/notify"
	"nofx/trader/types"
)

// Hedge-mode legs close only their position side. One-way legs must carry
// reduce-only/close-position semantics so a stale trigger cannot add risk.
func orderClosesSide(o types.OpenOrder, side string) bool {
	want := strings.ToUpper(side)
	ps := strings.ToUpper(o.PositionSide)
	if ps != "" && ps != "BOTH" && ps != want {
		return false
	}
	exitSide := "SELL"
	if want == "SHORT" {
		exitSide = "BUY"
	}
	if o.Side != "" && !strings.EqualFold(o.Side, exitSide) {
		return false
	}
	if ps == "" || ps == "BOTH" {
		return o.ReduceOnly || o.ClosePosition
	}
	return true
}

func protectionPrice(orders []types.OpenOrder, side, kind string) float64 {
	price := 0.0
	for _, o := range orders {
		if !protectiveOrderMatches(o, side, kind, o.StopPrice) {
			continue
		}
		// Prefer the tightest stop / nearest target. Only that price's
		// actual coverage is credited by the reconciler.
		higher := strings.EqualFold(side, "LONG") == (kind == "SL")
		if price == 0 || (higher && o.StopPrice > price) || (!higher && o.StopPrice < price) {
			price = o.StopPrice
		}
	}
	return price
}

func enoughProtection(orders []types.OpenOrder, side, kind string, price, quantity float64) bool {
	return quantity > 0 && price > 0 && protectiveCoverage(orders, side, kind, price)+math.Max(1e-9, quantity*1e-6) >= quantity
}

func (at *AutoTrader) setProtectionFault(key, reason string) {
	at.runtimeMu.Lock()
	defer at.runtimeMu.Unlock()
	if at.protectionFaults == nil {
		at.protectionFaults = map[string]string{}
	}
	at.protectionFaults[key] = reason
}

func (at *AutoTrader) clearProtectionFaults(live map[string]bool) {
	at.runtimeMu.Lock()
	defer at.runtimeMu.Unlock()
	for key := range at.protectionFaults {
		if strings.HasPrefix(key, "pending:") {
			continue
		}
		if strings.HasPrefix(key, "recovery:") && live[strings.TrimPrefix(key, "recovery:")] {
			continue
		}
		delete(at.protectionFaults, key)
	}
	for key := range at.protectionFailures {
		if !live[key] {
			delete(at.protectionFailures, key)
		}
	}
}
func (at *AutoTrader) clearPendingProtectionFaults() {
	at.runtimeMu.Lock()
	defer at.runtimeMu.Unlock()
	for key := range at.protectionFaults {
		if strings.HasPrefix(key, "pending:") {
			delete(at.protectionFaults, key)
		}
	}
}

func (at *AutoTrader) protectionFaultReason() string {
	for _, peer := range at.accountPeers() {
		peer.runtimeMu.RLock()
		for key, reason := range peer.protectionFaults {
			peer.runtimeMu.RUnlock()
			return fmt.Sprintf("unverified account protection %s: %s", key, reason)
		}
		peer.runtimeMu.RUnlock()
	}
	return ""
}

// Failed stop repair has a bounded retry budget. Keep the fault until a fresh
// snapshot proves coverage or disappearance; close acknowledgements are not proof.
func (at *AutoTrader) protectionFailure(symbol, side, reason string) {
	key := symbol + "_" + side
	at.setProtectionFault(key, reason)
	at.runtimeMu.Lock()
	if at.protectionFailures == nil {
		at.protectionFailures = map[string]int{}
	}
	at.protectionFailures[key]++
	attempts := at.protectionFailures[key]
	at.runtimeMu.Unlock()
	if attempts >= 3 {
		at.markCloseIntent(symbol, side, "stop_loss_recovery")
		if err := at.emergencyClosePosition(symbol, side); err != nil {
			at.alertUnprotectedPosition(symbol, side, "stop repair exhausted; exit failed: "+err.Error())
		}
	}
}

func (at *AutoTrader) protectionVerified(symbol, side string) {
	at.runtimeMu.Lock()
	delete(at.protectionFailures, symbol+"_"+side)
	at.runtimeMu.Unlock()
}

func (at *AutoTrader) initialStopRetryWait(d time.Duration) {
	if r := at.realtimeState(); r != nil && r.retryWait != nil {
		r.retryWait(d)
		return
	}
	time.Sleep(d)
}
func (at *AutoTrader) initialStopFastFailure(symbol, side string, price, qty float64, cause error) error {
	if !at.binanceRealtimeCapable() || !at.isAIManaged(symbol, side) {
		return cause
	}
	for _, delay := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
		at.initialStopRetryWait(delay)
		err := ensureProtectiveCoverage(at.trader, symbol, strings.ToUpper(side), "SL", price, qty)
		verified := at.verifyInitialStopCoverage(symbol, side, price, qty)
		if verified == nil {
			return nil
		}
		if err != nil {
			cause = err
		} else {
			cause = verified
		}
	}
	missing := qty
	if orders, err := at.trader.GetOpenOrders(symbol); err == nil {
		missing = math.Max(0, qty-protectiveCoverage(orders, strings.ToUpper(side), "SL", price))
	}
	if missing <= math.Max(1e-9, qty*1e-6) {
		return nil
	}
	// Cancel the remainder before exiting so a later fill cannot recreate an
	// unprotected position. Durable recovery survives cancellation/close failure.
	if pe := at.getPendingEntry(symbol, side); pe != nil {
		pe.RecoveryReason = "initial SL retries exhausted: " + cause.Error()
		at.persistPendingEntry(pe)
		if c, ok := at.trader.(interface{ CancelOrder(string, string) error }); ok {
			if err := c.CancelOrder(symbol, pe.OrderID); err != nil {
				logger.Warnf("initial SL failure entry cancel: %v", err)
			}
		}
	}
	// review 2026-10-11: earlier partial fills may already have been trimmed.
	// Refresh only in this failed-fill action path (never in the 5s profit loop)
	// so an oversized hedge close does not reject and leave the smaller runner naked.
	at.invalidateExecutionCache()
	if liveQty, ok := at.positionQty(symbol, side); ok && liveQty > 0 && !math.IsNaN(liveQty) && !math.IsInf(liveQty, 0) {
		missing = math.Min(missing, liveQty)
	}
	at.setProtectionFault("recovery:"+symbol+"_"+side, "initial SL failed; emergency slice exit awaiting reconciliation")
	at.markCloseIntent(symbol, side, "stop_loss_recovery")
	_, closeErr := at.reducePosition(symbol, side, missing)
	logger.Warnf("🛡️ [%s] Initial SL retries exhausted %s %s; emergency exit %.6g (error: %v)", at.name, symbol, side, missing, closeErr)
	notify.Notify("ALERT", at.name, fmt.Sprintf("<b>🛡️ 初始止损失败 %s</b>\n快速重试已耗尽,无保护仓位 %.6g 已提交紧急平仓(错误: %v)", notify.Escape(symbol), missing, closeErr))
	if closeErr != nil {
		return fmt.Errorf("SL retries exhausted (%v); emergency exit failed: %w", cause, closeErr)
	}
	return fmt.Errorf("SL retries exhausted (%v); emergency exit submitted for %.6g", cause, missing)
}
func (at *AutoTrader) verifyInitialStopCoverage(symbol, side string, price, qty float64) error {
	orders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		return err
	}
	if !enoughProtection(orders, strings.ToUpper(side), "SL", price, qty) {
		return fmt.Errorf("SL coverage unconfirmed")
	}
	return nil
}
