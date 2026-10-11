package trader

import (
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"nofx/logger"
	"nofx/trader/binance"
)

// NOFX_REALTIME_PROTECT=0 disables the Binance streams and 5s loop. Default on.
// review 2026-10-11: keep the cycle and 30s monitor as independent backstops.
type binanceRealtimeTrader interface {
	NewUserStream() *binance.UserStream
	NewMarkStream() *binance.MarkStream
	CachedPositions() ([]map[string]interface{}, time.Duration, bool)
	InvalidatePositionCache()
}
type realtimePosition struct {
	pos map[string]interface{}
	at  time.Time
}
type realtimeFill struct {
	update binance.OrderUpdate
	due    time.Time
}
type realtimeProtection struct {
	user  *binance.UserStream
	marks interface {
		MarkPrice(string) (float64, time.Duration, bool)
		SetSymbols([]string)
	}
	markStream       *binance.MarkStream
	stop, done, wake chan struct{}
	stopOnce         sync.Once
	mu               sync.Mutex // only short memory operations: WS readers never acquire executionMutex
	accounts         map[string]binance.AccountPosition
	accountTime      map[string]time.Time
	fills            map[string]realtimeFill
	lastFill         map[string]time.Time
	seenFill         map[string]float64
	seenStatus       map[string]string
	positions        map[string]realtimePosition // guarded by executionMutex, like cycle state
	snapshotAt       time.Time
	atr              map[string]float64 // last decision-cycle ATR%, no market fetch in the fast loop
	// Tests inject the delay; production retries keep the execution lock.
	retryWait func(time.Duration)
}

func newRealtimeProtection() *realtimeProtection {
	return &realtimeProtection{
		stop: make(chan struct{}), done: make(chan struct{}), wake: make(chan struct{}, 1),
		accounts: map[string]binance.AccountPosition{}, accountTime: map[string]time.Time{}, fills: map[string]realtimeFill{}, lastFill: map[string]time.Time{}, seenFill: map[string]float64{}, seenStatus: map[string]string{}, positions: map[string]realtimePosition{}, atr: map[string]float64{},
	}
}
func (at *AutoTrader) realtimeState() *realtimeProtection {
	at.realtimeMu.RLock()
	defer at.realtimeMu.RUnlock()
	return at.realtime
}
func (at *AutoTrader) binanceRealtimeCapable() bool {
	_, ok := at.trader.(binanceRealtimeTrader)
	return ok
}
func (at *AutoTrader) startRealtimeProtection() {
	provider, ok := at.trader.(binanceRealtimeTrader)
	if !ok || os.Getenv("NOFX_REALTIME_PROTECT") == "0" {
		return
	}
	at.isRunningMutex.RLock()
	defer at.isRunningMutex.RUnlock()
	if !at.isRunning {
		return
	}
	r := newRealtimeProtection()
	r.user = provider.NewUserStream()
	r.markStream = provider.NewMarkStream()
	r.marks = r.markStream
	at.realtimeMu.Lock()
	at.realtime = r
	at.realtimeMu.Unlock()
	r.user.OnOrderUpdate(func(u binance.OrderUpdate) { at.queueRealtimeFill(r, u, time.Now()) })
	r.user.OnAccountUpdate(func(u binance.AccountUpdate) {
		select {
		case <-r.stop:
			return
		default:
		}
		// ACCOUNT_UPDATE contains deltas, not a complete account snapshot. Coalesce
		// each hedge side independently so a close cannot erase the opposite side.
		now := time.Now()
		if u.EventTime > 0 {
			if eventAt := time.UnixMilli(u.EventTime); eventAt.Before(now) {
				now = eventAt
			}
		}
		r.mu.Lock()
		for _, p := range u.Positions {
			key := p.Symbol + "_" + strings.ToLower(p.PositionSide)
			if now.Before(r.accountTime[key]) {
				continue
			}
			r.accounts[key] = p
			r.accountTime[key] = now
		}
		r.mu.Unlock()
		wakeRealtime(r)
	})
	go at.runRealtimeProtection(r, provider)
	r.markStream.Start()
	r.user.Start()
	logger.Infof("🛡️ [%s] Binance realtime protection enabled", at.name)
}
func (at *AutoTrader) stopRealtimeProtection() {
	r := at.realtimeState()
	if r == nil {
		return
	}
	r.stopOnce.Do(func() { close(r.stop) })
	// Cancel both SDK dials concurrently: one 45s handshake wait should not
	// postpone cancellation of the other stream (review 2026-10-11).
	var streams sync.WaitGroup
	if r.user != nil {
		streams.Add(1)
		go func() { defer streams.Done(); r.user.Stop() }()
	}
	if r.markStream != nil {
		streams.Add(1)
		go func() { defer streams.Done(); r.markStream.Stop() }()
	}
	streams.Wait()
	<-r.done
}
func wakeRealtime(r *realtimeProtection) {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}
func (at *AutoTrader) queueRealtimeFill(r *realtimeProtection, u binance.OrderUpdate, now time.Time) {
	if u.Status != "PARTIALLY_FILLED" && u.Status != "FILLED" {
		return
	}
	if u.FilledQuantity <= 0 || u.AveragePrice <= 0 || math.IsNaN(u.FilledQuantity) || math.IsInf(u.FilledQuantity, 0) || math.IsNaN(u.AveragePrice) || math.IsInf(u.AveragePrice, 0) {
		return
	}
	select {
	case <-r.stop:
		return
	default:
	}
	if u.OrderType != "" && !strings.EqualFold(u.OrderType, "LIMIT") {
		return
	}
	// review 2026-10-11: a fill can beat the REST entry acknowledgement.
	// Match pending order IDs in the worker after acquiring executionMutex,
	// which also guards publishing the new pending row, not on the WS reader.
	r.mu.Lock()
	// Duplicate receipts must not re-finalize. A larger cumulative fill during
	// cooldown is retained for a trailing pass, rather than silently dropped.
	if u.FilledQuantity < r.seenFill[u.OrderID] || (u.FilledQuantity == r.seenFill[u.OrderID] && (u.Status != "FILLED" || r.seenStatus[u.OrderID] == "FILLED")) {
		r.mu.Unlock()
		return
	}
	r.seenFill[u.OrderID] = u.FilledQuantity
	r.seenStatus[u.OrderID] = u.Status
	due := now
	if next := r.lastFill[u.OrderID].Add(time.Second); next.After(due) {
		due = next
	}
	r.fills[u.OrderID] = realtimeFill{u, due}
	r.mu.Unlock()
	wakeRealtime(r)
}
func (at *AutoTrader) runRealtimeProtection(r *realtimeProtection, p binanceRealtimeTrader) {
	defer close(r.done)
	fills := time.NewTicker(100 * time.Millisecond)
	fast := time.NewTicker(5 * time.Second)
	defer fills.Stop()
	defer fast.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-r.wake:
			at.safeRealtimePass(r, p, false, time.Now())
		case <-fills.C:
			at.safeRealtimePass(r, p, false, time.Now())
		case <-fast.C:
			at.safeRealtimePass(r, p, true, time.Now())
		}
	}
}
func (at *AutoTrader) safeRealtimePass(r *realtimeProtection, p binanceRealtimeTrader, fast bool, now time.Time) {
	if !fast {
		r.mu.Lock()
		work := len(r.accounts) > 0
		for _, f := range r.fills {
			if !f.due.After(now) {
				work = true
				break
			}
		}
		r.mu.Unlock()
		if !work {
			return
		}
	}
	// Same account-wide mutex as safeProtectionPass / runCycle: stop moves,
	// cumulative-fill watermarks and trim markers cannot race either caller.
	waitStarted := time.Now()
	mu := at.executionMutex()
	mu.Lock()
	defer mu.Unlock()
	// review 2026-10-11: cycle/AI work can hold this lock for minutes;
	// freshness must be measured when actions execute, not when the tick fired.
	now = now.Add(time.Since(waitStarted))
	passStarted := time.Now()
	defer func() {
		if v := recover(); v != nil {
			logger.Errorf("🛡️ [%s] realtime protection pass recovered: %v", at.name, v)
		}
	}()
	select {
	case <-r.stop:
		return
	default:
	}
	at.applyRealtimeAccounts(r, p)
	r.mu.Lock()
	ready := []binance.OrderUpdate{}
	for id, f := range r.fills {
		if !f.due.After(now) {
			ready = append(ready, f.update)
			delete(r.fills, id)
			r.lastFill[id] = now
		}
	}
	r.mu.Unlock()
	for _, u := range ready {
		if !at.finalizeRealtimeFill(u) {
			r.mu.Lock()
			delete(r.lastFill, u.OrderID)
			delete(r.seenFill, u.OrderID)
			delete(r.seenStatus, u.OrderID)
			r.mu.Unlock()
		}
	}
	if len(ready) > 0 {
		at.updateRealtimeSymbols(r)
	}
	if fast {
		actionTime := now.Add(time.Since(passStarted))
		if liveTime := time.Now(); liveTime.After(actionTime) {
			actionTime = liveTime
		}
		readStarted := time.Now()
		at.refreshRealtimeCachedPositions(r, p, actionTime)
		at.fastProfitPass(r, actionTime.Add(time.Since(readStarted)))
	}
}
func (at *AutoTrader) finalizeRealtimeFill(u binance.OrderUpdate) bool {
	at.pendingEntriesMu.RLock()
	var pe *pendingEntry
	for _, pending := range at.pendingEntries {
		if pending.OrderID == u.OrderID && pending.Symbol == u.Symbol {
			pe = pending
			break
		}
	}
	at.pendingEntriesMu.RUnlock()
	if pe == nil {
		return false
	}
	// review 2026-10-11: use the actual event receipt, including a partial fill,
	// with the very same cumulative-coverage finalizer as the 30s backstop.
	at.invalidateExecutionCache()
	status := map[string]interface{}{"status": u.Status, "executedQty": u.FilledQuantity, "avgPrice": u.AveragePrice}
	if pe.RecoveryReason != "" {
		if at.recoverRejectedPendingFill(pe, status) {
			at.dropPendingEntry(pe.Symbol, pe.Side)
		}
		return true
	}
	at.protectExecutedSlice(pe, status)
	if u.Status == "FILLED" {
		at.reportFilledRR(pe, u.AveragePrice)
		if pendingProtectionComplete(pe, status) {
			at.dropPendingEntry(pe.Symbol, pe.Side)
			logger.Infof("✅ [%s] Realtime limit fill protected: %s %s", at.name, pe.Symbol, pe.Side)
		}
	}
	return true
}
func (at *AutoTrader) applyRealtimeAccounts(r *realtimeProtection, p binanceRealtimeTrader) {
	r.mu.Lock()
	accounts, times := r.accounts, r.accountTime
	r.accounts = map[string]binance.AccountPosition{}
	r.accountTime = map[string]time.Time{}
	r.mu.Unlock()
	if len(accounts) == 0 {
		return
	}
	// Invalidation may wait for an in-flight REST fetch; it runs on this worker,
	// never on go-binance's websocket read loop (review 2026-10-11).
	p.InvalidatePositionCache()
	for key, update := range accounts {
		side := strings.ToLower(update.PositionSide)
		if side != "long" && side != "short" {
			continue
		}
		if times[key].Before(r.positions[key].at) {
			continue
		}
		if update.Amount == 0 {
			r.positions[key] = realtimePosition{map[string]interface{}{"symbol": update.Symbol, "side": side, "positionAmt": 0.0}, times[key]}
			continue
		}
		r.positions[key] = realtimePosition{map[string]interface{}{"symbol": update.Symbol, "side": side, "positionAmt": update.Amount, "entryPrice": update.EntryPrice}, times[key]}
	}
	at.updateRealtimeSymbols(r)
}
func (at *AutoTrader) refreshRealtimeCachedPositions(r *realtimeProtection, p binanceRealtimeTrader, now time.Time) {
	readStarted := time.Now()
	positions, age, ok := p.CachedPositions()
	now = now.Add(time.Since(readStarted))
	if ok && age >= 0 && age <= 2*time.Minute {
		at.cacheRealtimePositions(r, positions, now.Add(-age))
	}
}
func (at *AutoTrader) cacheRealtimePositions(r *realtimeProtection, positions []map[string]interface{}, atTime time.Time) {
	if !atTime.After(r.snapshotAt) {
		return
	}
	r.snapshotAt = atTime
	live := map[string]bool{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["side"].(string)
		key := symbol + "_" + side
		live[key] = true
		if atTime.Before(r.positions[key].at) {
			continue
		}
		copy := map[string]interface{}{}
		for k, v := range pos {
			copy[k] = v
		}
		r.positions[key] = realtimePosition{copy, atTime}
	}
	for key, p := range r.positions {
		if !live[key] && !p.at.After(atTime) {
			delete(r.positions, key)
		}
	}
	at.updateRealtimeSymbols(r)
}
func (at *AutoTrader) updateRealtimeSymbols(r *realtimeProtection) {
	symbols := []string{}
	for _, p := range r.positions {
		symbol, _ := p.pos["symbol"].(string)
		side, _ := p.pos["side"].(string)
		qty, _ := p.pos["positionAmt"].(float64)
		if qty != 0 && at.isAIManaged(symbol, side) {
			symbols = append(symbols, symbol)
		}
	}
	if r.marks != nil {
		r.marks.SetSymbols(symbols)
	}
}
func (at *AutoTrader) fastProfitPass(r *realtimeProtection, now time.Time) {
	if at.config.StrategyConfig == nil || r.marks == nil {
		return
	}
	passStarted := time.Now()
	for key, p := range r.positions {
		if age := now.Add(time.Since(passStarted)).Sub(p.at); age < 0 || age > 2*time.Minute {
			continue
		}
		symbol, _ := p.pos["symbol"].(string)
		mark, age, ok := r.marks.MarkPrice(symbol)
		if !ok || age < 0 || age > 10*time.Second || mark <= 0 || math.IsNaN(mark) || math.IsInf(mark, 0) {
			continue
		}
		if at.processRProfitRules(p.pos, mark) {
			side, _ := p.pos["side"].(string)
			if at.GetRecordedStopLoss(symbol, side) == 0 {
				delete(r.positions, key)
			}
			continue
		}
		at.processTrailingForPosition(p.pos, mark, r.atr[symbol])
	}
}

// Explicit quantity avoids CloseLong/Short(qty=0)'s REST position lookup in the
// fast loop. Binance hedge-side SELL LONG / BUY SHORT only reduce that side;
// Binance forbids the reduceOnly flag in hedge mode (review 2026-10-11).
func (at *AutoTrader) closeProfitSnapshot(symbol, side, reason string, qty float64) error {
	if !at.binanceRealtimeCapable() {
		return at.closePositionReasoned(symbol, side, reason)
	}
	at.markCloseIntent(symbol, side, reason)
	if _, err := at.reducePosition(symbol, side, qty); err != nil {
		return err
	}
	at.ClearRecordedStopLoss(symbol, side)
	at.ClearInitialStopLoss(symbol, side)
	at.ClearExitMode(symbol, side)
	return nil
}
