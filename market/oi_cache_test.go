package market

import (
	"errors"
	"testing"
	"time"
)

func TestGetOpenInterestDataCache(t *testing.T) {
	oldFetch, oldNow := oiFetch, oiNow
	oiDataMu.Lock()
	oiDataCache = map[string]oiDataEntry{}
	oiDataMu.Unlock()
	t.Cleanup(func() {
		oiFetch, oiNow = oldFetch, oldNow
		oiDataMu.Lock()
		oiDataCache = map[string]oiDataEntry{}
		oiDataMu.Unlock()
	})
	clock := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	oiNow = func() time.Time { return clock }
	calls := 0
	fail := false
	oiFetch = func(symbol string) (*OIData, error) {
		calls++
		if fail {
			return nil, errors.New("boom")
		}
		return &OIData{Latest: float64(calls), Average: 1}, nil
	}

	a, err := getOpenInterestData("BTCUSDT")
	if err != nil || a.Latest != 1 {
		t.Fatalf("first: %+v %v", a, err)
	}
	a.Latest = 999 // callers must not be able to corrupt the cached copy
	b, _ := getOpenInterestData("BTCUSDT")
	if calls != 1 || b.Latest != 1 {
		t.Fatalf("second call must hit the cache untouched: calls=%d %+v", calls, b)
	}
	if _, _ = getOpenInterestData("ETHUSDT"); calls != 2 {
		t.Fatalf("different symbol must fetch: calls=%d", calls)
	}
	clock = clock.Add(oiDataTTL - time.Second)
	getOpenInterestData("BTCUSDT")
	if calls != 2 {
		t.Fatalf("inside TTL: calls=%d", calls)
	}
	clock = clock.Add(2 * time.Second)
	c, _ := getOpenInterestData("BTCUSDT")
	if calls != 3 || c.Latest != 3 {
		t.Fatalf("past TTL must refetch: calls=%d %+v", calls, c)
	}
	// Failures are not cached and do not poison the entry.
	fail = true
	clock = clock.Add(time.Hour)
	if _, err := getOpenInterestData("BTCUSDT"); err == nil {
		t.Fatal("error must propagate")
	}
	fail = false
	if _, err := getOpenInterestData("BTCUSDT"); err != nil || calls != 5 {
		t.Fatalf("recovery: calls=%d err=%v", calls, err)
	}
}
