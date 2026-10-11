package binance

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// Request-weight budget tests for the periodic order sync: traders sharing an
// account run one sync per interval, discovery probes are throttled, and held
// symbols are re-queried only when their exchange position changed.

type syncCallCounter struct {
	mu        sync.Mutex
	income    int
	userTrade map[string]int
	positions string // current positionRisk JSON
}

func (c *syncCallCounter) transport(trades []map[string]interface{}) batch1Transport {
	base := batch2SyncTransport(trades)
	return func(r *http.Request) (*http.Response, error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		switch r.URL.Path {
		case "/fapi/v1/income":
			c.income++
		case "/fapi/v1/userTrades":
			if c.userTrade == nil {
				c.userTrade = map[string]int{}
			}
			c.userTrade[r.URL.Query().Get("symbol")]++
		case "/fapi/v2/positionRisk":
			return batch1Response(r, http.StatusOK, c.positions), nil
		}
		return base(r)
	}
}

func (c *syncCallCounter) counts() (income int, userTrades map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]int{}
	for k, v := range c.userTrade {
		out[k] = v
	}
	return c.income, out
}

func posJSON(symbol, amt string) string {
	return `[{"symbol":"` + symbol + `","positionAmt":"` + amt + `","entryPrice":"100","markPrice":"101","unRealizedProfit":"1","leverage":"2","liquidationPrice":"0","positionSide":"LONG"}]`
}

// Two traders sharing one exchange account each keep their own probe schedule
// and fingerprint state: one trader's probe must never suppress the other's.
func TestOrderSyncStateIsPerTraderNotPerExchange(t *testing.T) {
	st := batch2SyncStore(t)
	ex := t.Name()
	ctr := &syncCallCounter{positions: posJSON("XUSDT", "1")}
	tr := batch1FuturesTrader(ctr.transport(nil))

	if err := tr.syncOrders("traderA", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	income, ut := ctr.counts()
	if income != 2 || ut["XUSDT"] != 1 {
		t.Fatalf("A first run: income=%d userTrades=%v", income, ut)
	}
	// B, same exchange, right after A's probe: B has never probed, so it must
	// run its own probe pair and query the held symbol itself.
	tr.InvalidateAccountCache()
	if err := tr.syncOrders("traderB", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	income, ut = ctr.counts()
	if income != 4 || ut["XUSDT"] != 2 {
		t.Fatalf("B must get its own probe + query: income=%d userTrades=%v", income, ut)
	}
	// Each trader is now throttled independently.
	for _, id := range []string{"traderA", "traderB"} {
		tr.InvalidateAccountCache()
		if err := tr.syncOrders(id, ex, "binance", st, false); err != nil {
			t.Fatal(err)
		}
	}
	income, ut = ctr.counts()
	if income != 4 || ut["XUSDT"] != 2 {
		t.Fatalf("idle follow-up runs must be free for both: income=%d userTrades=%v", income, ut)
	}
	// Age out only A's probe: A probes again, B stays throttled.
	binanceSyncStateMutex.Lock()
	orderSyncAcctLocked("traderA", ex).lastProbe = time.Now().Add(-orderSyncProbeEvery - time.Second)
	binanceSyncStateMutex.Unlock()
	tr.InvalidateAccountCache()
	tr.syncOrders("traderB", ex, "binance", st, false)
	if income, _ = ctr.counts(); income != 4 {
		t.Fatalf("A's schedule must not drive B: income=%d", income)
	}
	tr.syncOrders("traderA", ex, "binance", st, false)
	if income, _ = ctr.counts(); income != 6 {
		t.Fatalf("A is due and must probe: income=%d", income)
	}
}

func TestPeriodicSyncThrottlesIncomeProbesAndSkipsUnchangedHeldSymbols(t *testing.T) {
	st := batch2SyncStore(t)
	ex := t.Name()
	ctr := &syncCallCounter{positions: posJSON("XUSDT", "1")}
	tr := batch1FuturesTrader(ctr.transport(nil))

	// Run 1: first run is a probe run (2 income calls) and queries the held symbol.
	if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	income, ut := ctr.counts()
	if income != 2 || ut["XUSDT"] != 1 {
		t.Fatalf("run 1: income=%d userTrades=%v, want 2 / XUSDT=1", income, ut)
	}

	// Runs 2-4 (30 s cadence, inside the probe interval): no income probes and
	// the held symbol is not re-queried while its position is unchanged.
	for i := 0; i < 3; i++ {
		tr.InvalidateAccountCache() // fresh positionRisk each run, like the 15 s cache expiring
		if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
			t.Fatal(err)
		}
	}
	income, ut = ctr.counts()
	if income != 2 || ut["XUSDT"] != 1 {
		t.Fatalf("idle runs must be free: income=%d userTrades=%v, want 2 / XUSDT=1", income, ut)
	}

	// Position changes (a fill): queried on the very next run, still no income probe.
	ctr.mu.Lock()
	ctr.positions = posJSON("XUSDT", "2")
	ctr.mu.Unlock()
	tr.InvalidateAccountCache()
	if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	income, ut = ctr.counts()
	if income != 2 || ut["XUSDT"] != 2 {
		t.Fatalf("changed position must be re-queried: income=%d userTrades=%v", income, ut)
	}

	// Unchanged again afterwards -> skipped.
	tr.InvalidateAccountCache()
	if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	if _, ut = ctr.counts(); ut["XUSDT"] != 2 {
		t.Fatalf("unchanged position re-queried: %v", ut)
	}

	// Probe interval elapsed: probes AND every held symbol are covered again.
	binanceSyncStateMutex.Lock()
	orderSyncAcctLocked("t", ex).lastProbe = time.Now().Add(-orderSyncProbeEvery - time.Second)
	binanceSyncStateMutex.Unlock()
	tr.InvalidateAccountCache()
	if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	income, ut = ctr.counts()
	if income != 4 || ut["XUSDT"] != 3 {
		t.Fatalf("probe run must re-run both income probes and query held symbols: income=%d userTrades=%v", income, ut)
	}

	// A direct (non-periodic) caller always probes.
	tr.InvalidateAccountCache()
	if err := tr.SyncOrdersFromBinance("t", ex, "binance", st); err != nil {
		t.Fatal(err)
	}
	if income, _ = ctr.counts(); income != 6 {
		t.Fatalf("SyncOrdersFromBinance must always probe, income=%d", income)
	}
}

func TestPeriodicSyncProbeWindowStartsAtLastProbeNotLastFill(t *testing.T) {
	st := batch2SyncStore(t)
	ex := t.Name()
	var mu sync.Mutex
	var starts []int64
	base := batch2SyncTransport(nil)
	tr := batch1FuturesTrader(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/fapi/v1/income" {
			mu.Lock()
			starts = append(starts, atoi64(r.URL.Query().Get("startTime")))
			mu.Unlock()
		}
		return base(r)
	})
	// An idle account whose last fill was three days ago.
	old := time.Now().Add(-72 * time.Hour).UnixMilli()
	binanceSyncStateMutex.Lock()
	binanceSyncState[ex] = old
	orderSyncAcctLocked("t", ex).lastProbe = time.Now().Add(-orderSyncProbeEvery - time.Second)
	binanceSyncStateMutex.Unlock()
	t.Cleanup(func() {
		binanceSyncStateMutex.Lock()
		delete(binanceSyncState, ex)
		binanceSyncStateMutex.Unlock()
	})

	if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 2 {
		t.Fatalf("expected the two income probes, got %v", starts)
	}
	for _, s := range starts {
		age := time.Since(time.UnixMilli(s))
		if age > orderSyncProbeOverlap+orderSyncProbeEvery+30*time.Second || age < orderSyncProbeOverlap {
			t.Fatalf("probe window must start ~%v+ before now (last probe - overlap), not at the 3-day-old fill: age=%v", orderSyncProbeOverlap, age)
		}
	}
}

func TestPeriodicSyncRetriesFailedSymbolWithoutProbe(t *testing.T) {
	st := batch2SyncStore(t)
	ex := t.Name()
	var mu sync.Mutex
	fail := true
	utCalls := 0
	base := batch2SyncTransport(nil)
	tr := batch1FuturesTrader(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/fapi/v2/positionRisk":
			return batch1Response(r, http.StatusOK, posJSON("XUSDT", "1")), nil
		case "/fapi/v1/userTrades":
			mu.Lock()
			utCalls++
			f := fail
			mu.Unlock()
			if f {
				return batch1Response(r, http.StatusInternalServerError, `{"code":-1000,"msg":"boom"}`), nil
			}
		}
		return base(r)
	})
	if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	// Failed query -> the symbol is retried next run (no probe is due, the
	// position fingerprint is unchanged: only the pending marker brings it back).
	mu.Lock()
	fail = false
	mu.Unlock()
	tr.InvalidateAccountCache()
	if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	n := utCalls
	mu.Unlock()
	if n != 2 {
		t.Fatalf("failed symbol must be retried on the next run, userTrades calls=%d", n)
	}
	// ...and once it succeeded it is not pending anymore.
	tr.InvalidateAccountCache()
	if err := tr.syncOrders("t", ex, "binance", st, false); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	n = utCalls
	mu.Unlock()
	if n != 2 {
		t.Fatalf("recovered symbol re-queried, userTrades calls=%d", n)
	}
}

func atoi64(s string) int64 {
	var n int64
	for _, c := range strings.TrimSpace(s) {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
