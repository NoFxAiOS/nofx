package trader

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
	"nofx/trader/binance"
	"nofx/trader/types"
)

// review 2026-10-11: a position/price fetch in a fast tick is a test failure.
type realtimeMock struct {
	batch1RiskMock
	positionsCalls, priceCalls, invalidations, failStops int
	actions                                              []string
	cached                                               []map[string]interface{}
	cacheAge                                             time.Duration
	cacheOK                                              bool
}

func (*realtimeMock) NewUserStream() *binance.UserStream { panic("network stream in unit test") }
func (*realtimeMock) NewMarkStream() *binance.MarkStream { panic("network stream in unit test") }
func (m *realtimeMock) CachedPositions() ([]map[string]interface{}, time.Duration, bool) {
	return m.cached, m.cacheAge, m.cacheOK
}
func (m *realtimeMock) InvalidatePositionCache() { m.invalidations++ }
func (m *realtimeMock) GetPositions() ([]map[string]interface{}, error) {
	m.positionsCalls++
	return m.fix06Mock.GetPositions()
}
func (m *realtimeMock) GetMarketPrice(string) (float64, error) {
	m.priceCalls++
	return 0, errors.New("no REST prices")
}
func (m *realtimeMock) SetStopLoss(symbol, side string, qty, price float64) error {
	m.slCalls++
	m.actions = append(m.actions, fmt.Sprintf("stop:%g", price))
	if m.slCalls <= m.failStops {
		return errors.New("stop rejected")
	}
	m.stopPrices = append(m.stopPrices, price)
	m.orders = append(m.orders, types.OpenOrder{OrderID: fmt.Sprint(m.slCalls), Symbol: symbol, PositionSide: side, Type: "STOP_MARKET", Quantity: qty, StopPrice: price, ClosePosition: true})
	return nil
}
func (m *realtimeMock) SetTakeProfit(symbol, side string, qty, price float64) error {
	m.orders = append(m.orders, types.OpenOrder{OrderID: "tp", Symbol: symbol, PositionSide: side, Type: "TAKE_PROFIT_MARKET", Quantity: qty, StopPrice: price, ClosePosition: true})
	return nil
}
func (m *realtimeMock) CloseLong(_ string, qty float64) (map[string]interface{}, error) {
	m.closeCalls++
	m.closedQty = qty
	m.actions = append(m.actions, fmt.Sprintf("trim:%g", qty))
	if m.closeErr != nil {
		return nil, m.closeErr
	}
	m.qty -= qty
	return map[string]interface{}{}, nil
}
func (m *realtimeMock) CloseShort(_ string, qty float64) (map[string]interface{}, error) {
	m.closeShorts++
	m.closedQty = qty
	m.actions = append(m.actions, fmt.Sprintf("trim:%g", qty))
	if m.closeErr != nil {
		return nil, m.closeErr
	}
	m.qty += qty
	return map[string]interface{}{}, nil
}
func newRealtimeTestTrader(rc store.RiskControlConfig) (*AutoTrader, *realtimeMock, *realtimeProtection) {
	at, base := newBatch1RiskTrader("long", 1, 100, rc)
	m := &realtimeMock{batch1RiskMock: *base}
	at.trader = m
	at.peakPnLCache = map[string]float64{}
	at.positionFirstSeenTime = map[string]int64{}
	at.positionExitMode = map[string]string{}
	r := newRealtimeProtection()
	at.realtime = r
	r.retryWait = func(time.Duration) {}
	return at, m, r
}

type fakeRealtimeMarks struct {
	price   float64
	age     time.Duration
	ok      bool
	symbols []string
}

func (m *fakeRealtimeMarks) MarkPrice(string) (float64, time.Duration, bool) {
	return m.price, m.age, m.ok
}
func (m *fakeRealtimeMarks) SetSymbols(s []string) { m.symbols = append([]string{}, s...) }
func realtimePos(side string, qty float64) map[string]interface{} {
	return map[string]interface{}{"symbol": "XUSDT", "side": side, "positionAmt": qty, "entryPrice": 100.0, "markPrice": 100.0}
}
func primeRealtimePosition(at *AutoTrader, r *realtimeProtection, side string, now time.Time) *fakeRealtimeMarks {
	qty := 1.0
	sl := 90.0
	if side == "short" {
		qty = -1
		sl = 110
	}
	at.SetRecordedStopLoss("XUSDT", side, sl)
	at.SetInitialStopLoss("XUSDT", side, sl)
	marks := &fakeRealtimeMarks{price: 100, ok: true}
	r.marks = marks
	at.cacheRealtimePositions(r, []map[string]interface{}{realtimePos(side, qty)}, now)
	return marks
}
func TestRealtimeTrackedFillDebounceAndFinalization(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{RiskPerTradePct: 3})
	pe := &pendingEntry{Symbol: "XUSDT", Side: "long", OrderID: "42", Price: 100, StopLoss: 90, TakeProfit: 130, PlacedAt: time.Now()}
	at.setPendingEntry(pe)
	now := time.Now()
	u := binance.OrderUpdate{Symbol: "XUSDT", OrderID: "other", Status: "FILLED", FilledQuantity: 1, AveragePrice: 100}
	at.queueRealtimeFill(r, u, now)
	at.safeRealtimePass(r, m, false, now)
	if m.slCalls != 0 {
		t.Fatal("unrelated fill finalized")
	}
	u.OrderID = "42"
	u.Status = "PARTIALLY_FILLED"
	u.FilledQuantity = 0.4
	for i := 0; i < 10; i++ {
		at.queueRealtimeFill(r, u, now)
	}
	at.safeRealtimePass(r, m, false, now)
	if m.slCalls != 1 || pe.ProtectedQty != 0.4 {
		t.Fatalf("calls=%d protected=%g", m.slCalls, pe.ProtectedQty)
	}
	// New partial fills during the one-second debounce are retained, coalesced.
	u.FilledQuantity = 0.6
	at.queueRealtimeFill(r, u, now.Add(100*time.Millisecond))
	u.FilledQuantity = 1
	u.Status = "FILLED"
	at.queueRealtimeFill(r, u, now.Add(200*time.Millisecond))
	at.safeRealtimePass(r, m, false, now.Add(999*time.Millisecond))
	if at.getPendingEntry("XUSDT", "long") == nil {
		t.Fatal("debounce ignored")
	}
	at.safeRealtimePass(r, m, false, r.lastFill["42"].Add(time.Second))
	if at.getPendingEntry("XUSDT", "long") != nil || pe.ProtectedQty != 1 || m.slCalls != 1 {
		t.Fatalf("entry=%v protected=%g calls=%d", at.getPendingEntry("XUSDT", "long"), pe.ProtectedQty, m.slCalls)
	}
	at.queueRealtimeFill(r, u, now.Add(2*time.Second))
	at.safeRealtimePass(r, m, false, now.Add(2*time.Second))
	if len(r.fills) != 0 || m.slCalls != 1 {
		t.Fatal("duplicate terminal receipt finalized")
	}
}
func TestRealtimeFillReaderDoesNotWaitForExecutionLock(t *testing.T) {
	at, _, r := newRealtimeTestTrader(store.RiskControlConfig{})
	at.setPendingEntry(&pendingEntry{Symbol: "XUSDT", Side: "long", OrderID: "42"})
	mu := at.executionMutex()
	mu.Lock()
	defer mu.Unlock()
	done := make(chan struct{})
	go func() {
		at.queueRealtimeFill(r, binance.OrderUpdate{Symbol: "XUSDT", OrderID: "42", Status: "FILLED", FilledQuantity: 1, AveragePrice: 100}, time.Now())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WS reader blocked on execution lock")
	}
}
func TestRealtimeInitialStopRetriesAndEmergencySlice(t *testing.T) {
	for _, tc := range []struct {
		name          string
		fail          int
		calls, closes int
	}{{"retry_success", 2, 3, 0}, {"exhausted", 99, 4, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			at, m, r := newRealtimeTestTrader(store.RiskControlConfig{})
			m.failStops = tc.fail
			var waits []time.Duration
			r.retryWait = func(d time.Duration) { waits = append(waits, d) }
			at.setPendingEntry(&pendingEntry{Symbol: "XUSDT", Side: "long", OrderID: "42"})
			sl, _ := at.placeProtectiveOrders(&kernel.Decision{Symbol: "XUSDT", StopLoss: 90, TakeProfit: 130}, "LONG", 0.4, 100, 100)
			if m.slCalls != tc.calls || m.closeCalls != tc.closes {
				t.Fatalf("stops=%d closes=%d sl=%v", m.slCalls, m.closeCalls, sl)
			}
			if tc.closes == 0 && sl != nil {
				t.Fatal(sl)
			}
			if tc.closes > 0 && (sl == nil || m.closedQty != 0.4 || m.cancelCalls != 1 || at.getPendingEntry("XUSDT", "long").RecoveryReason == "") {
				t.Fatalf("sl=%v close qty=%g cancel=%d", sl, m.closedQty, m.cancelCalls)
			}
			want := []time.Duration{time.Second, 2 * time.Second}
			if tc.closes > 0 {
				want = append(want, 4*time.Second)
			}
			if !reflect.DeepEqual(waits, want) {
				t.Fatalf("waits=%v", waits)
			}
		})
	}
}
func TestRealtimeInitialStopOnlyClosesMissingCoverage(t *testing.T) {
	at, m, _ := newRealtimeTestTrader(store.RiskControlConfig{})
	m.failStops = 99
	m.orders = []types.OpenOrder{{Type: "STOP_MARKET", PositionSide: "LONG", Quantity: 0.3, StopPrice: 90}}
	if err, _ := at.placeProtectiveOrders(&kernel.Decision{Symbol: "XUSDT", StopLoss: 90}, "LONG", 1, 100, 100); err == nil || !nearly(m.closedQty, 0.7) {
		t.Fatalf("err=%v close=%g", err, m.closedQty)
	}
}
func TestRealtimeFastProfitRulesNoRESTAndIdempotency(t *testing.T) {
	for _, side := range []string{"long", "short"} {
		t.Run(side, func(t *testing.T) {
			at, m, r := newRealtimeTestTrader(store.RiskControlConfig{BreakevenArmR: 0.8, ProfitLockAtR: 1.5, MinPositionSize: 1})
			now := time.Now()
			marks := primeRealtimePosition(at, r, side, now)
			m.side = side
			if side == "short" {
				m.qty = -1
				marks.price = 92
			} else {
				marks.price = 108
			}
			at.safeRealtimePass(r, m, true, now)
			at.safeRealtimePass(r, m, true, now)
			wantStop := 102.0
			if side == "short" {
				wantStop = 98
			}
			if m.slCalls != 1 || m.closeCalls+m.closeShorts != 0 || at.GetRecordedStopLoss("XUSDT", side) != wantStop {
				t.Fatalf("early arm stops=%d closes=%d", m.slCalls, m.closeCalls+m.closeShorts)
			}
			if side == "short" {
				marks.price = 84
			} else {
				marks.price = 116
			}
			at.safeRealtimePass(r, m, true, now)
			// The cycle's shared evaluator sees the same marker under the same mutex.
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					mu := at.executionMutex()
					mu.Lock()
					defer mu.Unlock()
					at.processRProfitRules(realtimePos(side, 1), marks.price)
				}()
			}
			wg.Wait()
			at.safeRealtimePass(r, m, true, now)
			if m.slCalls != 1 || m.closeCalls+m.closeShorts != 1 || m.closedQty != 0.5 || !at.r1TrimDone["XUSDT_"+side] || !at.tpTrimDone["XUSDT_"+side] {
				t.Fatalf("stops=%d closes=%d qty=%g markers=%v", m.slCalls, m.closeCalls+m.closeShorts, m.closedQty, at.r1TrimDone)
			}
			if m.positionsCalls != 0 || m.priceCalls != 0 {
				t.Fatalf("REST polling positions=%d price=%d", m.positionsCalls, m.priceCalls)
			}
		})
	}
}
func TestRealtimeLockConfirmsStopBeforeTrim(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: 1.5, MinPositionSize: 1})
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now)
	marks.price = 116
	m.failStops = 1
	at.safeRealtimePass(r, m, true, now)
	if m.closeCalls != 0 || at.r1TrimDone["XUSDT_long"] {
		t.Fatal("trimmed before stop confirmed")
	}
	at.safeRealtimePass(r, m, true, now)
	if m.closeCalls != 1 || !reflect.DeepEqual(m.actions, []string{"stop:102", "stop:102", "trim:0.5"}) {
		t.Fatalf("actions=%v", m.actions)
	}
}
func TestRealtimeStaleInputsSkip(t *testing.T) {
	for _, tc := range []struct {
		name            string
		posAge, markAge time.Duration
		ok              bool
	}{{"positions", 121 * time.Second, 0, true}, {"mark", 0, 11 * time.Second, true}, {"missing_mark", 0, 0, false}} {
		t.Run(tc.name, func(t *testing.T) {
			at, m, r := newRealtimeTestTrader(store.RiskControlConfig{BreakevenArmR: 0.8, ProfitLockAtR: 1.5})
			now := time.Now()
			marks := primeRealtimePosition(at, r, "long", now.Add(-tc.posAge))
			marks.price = 116
			marks.age = tc.markAge
			marks.ok = tc.ok
			at.safeRealtimePass(r, m, true, now)
			if m.slCalls+m.closeCalls != 0 || m.positionsCalls != 0 || m.priceCalls != 0 {
				t.Fatalf("actions=%v REST=%d/%d", m.actions, m.positionsCalls, m.priceCalls)
			}
		})
	}
}
func TestRealtimePeakDrawdownSharesCyclePeak(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: -1, PeakDrawdownArmR: 1.5, PeakDrawdownGivebackR: 0.5})
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now)
	marks.price = 116
	at.safeRealtimePass(r, m, true, now)
	if at.GetPeakPnLCache()["XUSDT_long"] != 1.6 {
		t.Fatal("fast peak not shared")
	}
	// The cycle observes the later giveback and closes using that shared peak.
	at.processRProfitRules(realtimePos("long", 1), 104)
	if m.closeCalls != 1 || m.closedQty != 1 || m.positionsCalls != 0 || m.priceCalls != 0 {
		t.Fatalf("close=%d qty=%g REST=%d/%d", m.closeCalls, m.closedQty, m.positionsCalls, m.priceCalls)
	}
	at.safeRealtimePass(r, m, true, now)
	if m.closeCalls != 1 {
		t.Fatal("closed twice")
	}
}
func TestRealtimeFastTrailingUsesCachedATR(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: -1, TrailingStopEnabled: true})
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now)
	marks.price = 120
	at.safeRealtimePass(r, m, true, now)
	if m.slCalls != 0 {
		t.Fatal("trailed without ATR")
	}
	r.atr["XUSDT"] = 1
	at.safeRealtimePass(r, m, true, now)
	at.safeRealtimePass(r, m, true, now)
	if m.slCalls != 1 || !nearly(at.GetRecordedStopLoss("XUSDT", "long"), 117.6) || m.positionsCalls != 0 || m.priceCalls != 0 {
		t.Fatalf("stops=%d price=%g REST=%d/%d", m.slCalls, at.GetRecordedStopLoss("XUSDT", "long"), m.positionsCalls, m.priceCalls)
	}
}
func TestRealtimeCycleLocksSurviveMarketDataFailure(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: 1.5, MinPositionSize: 1})
	now := time.Now()
	primeRealtimePosition(at, r, "long", now)
	m.mark = 116
	at.marketDataFn = func(string) (*market.Data, error) { return nil, errors.New("ATR source down") }
	at.processVolTargetAndTrailing()
	if m.closeCalls != 1 || m.slCalls != 1 {
		t.Fatalf("stops=%d trim=%d", m.slCalls, m.closeCalls)
	}
}
func TestRealtimeAccountDeltasAndCacheInvalidation(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{})
	now := time.Now()
	primeRealtimePosition(at, r, "long", now.Add(-time.Second))
	r.positions["XUSDT_short"] = realtimePosition{realtimePos("short", -2), now.Add(-time.Second)}
	r.accounts["XUSDT_long"] = binance.AccountPosition{Symbol: "XUSDT", PositionSide: "LONG", Amount: 0}
	r.accountTime["XUSDT_long"] = now
	at.safeRealtimePass(r, m, false, now)
	if m.invalidations != 1 || r.positions["XUSDT_long"].pos["positionAmt"] != 0.0 || r.positions["XUSDT_short"].pos["positionAmt"] != -2.0 {
		t.Fatalf("invalidation=%d positions=%v", m.invalidations, r.positions)
	}
	// A pre-close REST cache cannot resurrect the side just removed by a delta.
	at.cacheRealtimePositions(r, []map[string]interface{}{realtimePos("long", 1)}, now.Add(-500*time.Millisecond))
	if r.positions["XUSDT_long"].pos["positionAmt"] != 0.0 {
		t.Fatal("stale snapshot resurrected closed side")
	}
}
func TestRealtimeKillSwitchAndOtherExchanges(t *testing.T) {
	t.Setenv("NOFX_REALTIME_PROTECT", "0")
	at, m, _ := newRealtimeTestTrader(store.RiskControlConfig{})
	at.realtime = nil
	at.isRunning = true
	at.startRealtimeProtection()
	if at.realtime != nil {
		t.Fatal("kill switch ignored")
	}
	t.Setenv("NOFX_REALTIME_PROTECT", "1")
	at.trader = &m.batch1RiskMock
	at.startRealtimeProtection()
	if at.realtime != nil {
		t.Fatal("non-Binance attached streams")
	}
}
func TestRealtimeHandsOffManualPositions(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: 1.5})
	at.store = profitLockTestStore(t)
	at.id = "manual-test"
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now)
	marks.price = 116
	at.safeRealtimePass(r, m, true, now)
	if m.slCalls+m.closeCalls != 0 || len(marks.symbols) != 0 {
		t.Fatal("manual position touched/subscribed")
	}
}

func TestRealtimeCycleAndFastLoopCannotDoubleTrim(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: 1.5, MinPositionSize: 1})
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now)
	marks.price = 116
	m.mark = 116
	at.marketDataFn = func(string) (*market.Data, error) { return nil, errors.New("no network") }
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); at.safeRealtimePass(r, m, true, now) }()
	go func() {
		defer wg.Done()
		mu := at.executionMutex()
		mu.Lock()
		defer mu.Unlock()
		at.processVolTargetAndTrailing()
	}()
	wg.Wait()
	if m.closeCalls != 1 || m.slCalls != 1 || !at.r1TrimDone["XUSDT_long"] {
		t.Fatalf("stops=%d trims=%d markers=%v", m.slCalls, m.closeCalls, at.r1TrimDone)
	}
}
func TestRealtimeFastPeakDrawdownClose(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: -1, PeakDrawdownArmR: 1.5, PeakDrawdownGivebackR: 0.5})
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now)
	marks.price = 116
	at.safeRealtimePass(r, m, true, now)
	marks.price = 104
	at.safeRealtimePass(r, m, true, now)
	at.safeRealtimePass(r, m, true, now)
	if m.closeCalls != 1 || m.closedQty != 1 || m.positionsCalls != 0 || m.priceCalls != 0 {
		t.Fatalf("closes=%d qty=%g REST=%d/%d", m.closeCalls, m.closedQty, m.positionsCalls, m.priceCalls)
	}
}
func TestRealtimeChecksFreshnessAfterExecutionLockWait(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{BreakevenArmR: 0.8, ProfitLockAtR: -1})
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now.Add(-2*time.Minute+50*time.Millisecond))
	marks.price = 108
	mu := at.executionMutex()
	mu.Lock()
	done := make(chan struct{})
	go func() { at.safeRealtimePass(r, m, true, now); close(done) }()
	time.Sleep(80 * time.Millisecond)
	mu.Unlock()
	<-done
	if m.slCalls != 0 {
		t.Fatal("used pre-lock position freshness after waiting for execution")
	}
}
func TestRealtimeInitialStopCloseCapsLiveRunner(t *testing.T) {
	at, m, _ := newRealtimeTestTrader(store.RiskControlConfig{})
	m.failStops = 99
	m.qty = 0.2
	if err, _ := at.placeProtectiveOrders(&kernel.Decision{Symbol: "XUSDT", StopLoss: 90}, "LONG", 1, 100, 100); err == nil || m.closedQty != 0.2 {
		t.Fatalf("err=%v close=%g", err, m.closedQty)
	}
}
func TestRealtimeInitialStopFailureHandsOff(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{})
	at.store = profitLockTestStore(t)
	at.id = "manual-initial"
	waited := false
	r.retryWait = func(time.Duration) { waited = true }
	err := at.initialStopFastFailure("XUSDT", "long", 90, 1, errors.New("rejected"))
	if err == nil || waited || m.closeCalls+m.slCalls != 0 {
		t.Fatal("manual position retried or closed")
	}
}
func TestRealtimeWorkerStopAndStoppedPass(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{BreakevenArmR: 0.8})
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now)
	marks.price = 108
	go at.runRealtimeProtection(r, m)
	at.stopRealtimeProtection()
	at.stopRealtimeProtection()
	at.safeRealtimePass(r, m, true, now)
	if m.slCalls != 0 {
		t.Fatal("stopped worker placed order")
	}
}

func TestRealtimeMalformedEntryCannotTrail(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: 1.5, TrailingStopEnabled: true})
	now := time.Now()
	marks := primeRealtimePosition(at, r, "long", now)
	marks.price = 120
	r.atr["XUSDT"] = 1
	r.positions["XUSDT_long"].pos["entryPrice"] = 0.0
	at.safeRealtimePass(r, m, true, now)
	if m.slCalls+m.closeCalls != 0 || m.positionsCalls != 0 || m.priceCalls != 0 {
		t.Fatal("malformed position snapshot acted on")
	}
}

func TestRealtimeOtherExchangeCycleBehaviorUnchanged(t *testing.T) {
	at, m, _ := newRealtimeTestTrader(store.RiskControlConfig{ProfitLockAtR: 1.5})
	at.trader = &m.batch1RiskMock
	m.mark = 116
	at.SetRecordedStopLoss("XUSDT", "long", 90)
	at.SetInitialStopLoss("XUSDT", "long", 90)
	at.marketDataFn = func(string) (*market.Data, error) { return nil, errors.New("ATR source down") }
	at.processVolTargetAndTrailing()
	if m.slCalls+m.closeCalls != 0 {
		t.Fatal("non-Binance cycle gained the new market-failure fallback")
	}
	at.config.StrategyConfig.RiskControl = store.RiskControlConfig{ProfitLockAtR: -1, PeakDrawdownArmR: 1.5, PeakDrawdownGivebackR: 0.5}
	at.UpdatePeakPnL("XUSDT", "long", 1.6)
	if at.processRProfitRules(realtimePos("long", 1), 104) || m.closeCalls != 0 {
		t.Fatal("non-Binance cycle gained an extra drawdown close")
	}
}

func TestRealtimeFillBeforeEntryAcknowledgement(t *testing.T) {
	at, m, r := newRealtimeTestTrader(store.RiskControlConfig{RiskPerTradePct: 3})
	now := time.Now()
	// Binance can publish the fill before CreateOrder returns its order ID.
	at.queueRealtimeFill(r, binance.OrderUpdate{Symbol: "XUSDT", OrderID: "42", OrderType: "LIMIT", Status: "FILLED", FilledQuantity: 0.4, AveragePrice: 100}, now)
	at.setPendingEntry(&pendingEntry{Symbol: "XUSDT", Side: "long", OrderID: "42", Price: 100, StopLoss: 90, TakeProfit: 130})
	at.safeRealtimePass(r, m, false, now)
	if m.slCalls != 1 || at.getPendingEntry("XUSDT", "long") != nil {
		t.Fatal("fill preceding pending registration was lost")
	}
}
