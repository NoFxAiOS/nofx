package api

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nofx/trader"
)

func newTestOrdersCache(clock *time.Time) *openOrdersCache {
	return &openOrdersCache{
		entries: map[string]openOrdersEntry{},
		keyMu:   map[string]*sync.Mutex{},
		now:     func() time.Time { return *clock },
	}
}

func TestOpenOrdersCacheHitMissTTL(t *testing.T) {
	clock := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	c := newTestOrdersCache(&clock)
	var calls int32
	fetch := func() ([]trader.OpenOrder, error) {
		atomic.AddInt32(&calls, 1)
		return []trader.OpenOrder{{OrderID: "1", Type: "STOP_MARKET"}}, nil
	}
	for i := 0; i < 5; i++ {
		got, err := c.get("t1|BTCUSDT", fetch)
		if err != nil || len(got) != 1 {
			t.Fatalf("get %d: %v %v", i, got, err)
		}
		got[0].Type = "MUTATED" // callers get copies
	}
	if calls != 1 {
		t.Fatalf("5 polls inside the TTL must fetch once, fetched %d", calls)
	}
	if got, _ := c.get("t1|BTCUSDT", fetch); got[0].Type != "STOP_MARKET" {
		t.Fatal("cached entry was corrupted by a caller")
	}
	// Other symbol / other trader are separate entries.
	c.get("t1|ETHUSDT", fetch)
	c.get("t2|BTCUSDT", fetch)
	if calls != 3 {
		t.Fatalf("distinct keys must fetch: %d", calls)
	}
	clock = clock.Add(openOrdersUITTL - time.Second)
	c.get("t1|BTCUSDT", fetch)
	if calls != 3 {
		t.Fatalf("inside TTL: %d", calls)
	}
	clock = clock.Add(2 * time.Second)
	c.get("t1|BTCUSDT", fetch)
	if calls != 4 {
		t.Fatalf("past TTL must refetch: %d", calls)
	}
}

func TestOpenOrdersCacheDoesNotCacheErrors(t *testing.T) {
	clock := time.Now()
	c := newTestOrdersCache(&clock)
	var calls int32
	fail := true
	fetch := func() ([]trader.OpenOrder, error) {
		atomic.AddInt32(&calls, 1)
		if fail {
			return nil, errors.New("exchange down")
		}
		return nil, nil
	}
	if _, err := c.get("k", fetch); err == nil {
		t.Fatal("error must propagate")
	}
	fail = false
	if _, err := c.get("k", fetch); err != nil || calls != 2 {
		t.Fatalf("error was cached: calls=%d err=%v", calls, err)
	}
}

func TestOpenOrdersCacheCoalescesConcurrentMisses(t *testing.T) {
	clock := time.Now()
	c := newTestOrdersCache(&clock)
	var calls int32
	release := make(chan struct{})
	fetch := func() ([]trader.OpenOrder, error) {
		atomic.AddInt32(&calls, 1)
		<-release
		return []trader.OpenOrder{{OrderID: "1"}}, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.get("k", fetch) }()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls != 1 {
		t.Fatalf("a burst of identical UI polls must fetch once, fetched %d", calls)
	}
}
