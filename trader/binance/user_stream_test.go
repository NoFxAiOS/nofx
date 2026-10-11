package binance

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// review 2026-10-11: all stream tests use SDK event decoding and injected IO.
func TestRealtimeUserEventParsingAndSubscriberRecovery(t *testing.T) {
	s := newUserStream(userStreamDeps{})
	var orders []OrderUpdate
	var accounts []AccountUpdate
	s.OnOrderUpdate(func(OrderUpdate) { panic("subscriber") })
	s.OnOrderUpdate(func(u OrderUpdate) { orders = append(orders, u) })
	s.OnAccountUpdate(func(AccountUpdate) { panic("subscriber") })
	s.OnAccountUpdate(func(u AccountUpdate) { accounts = append(accounts, u) })
	for _, raw := range []string{
		`{"e":"ORDER_TRADE_UPDATE","E":1000,"o":{"s":"ETHUSDT","S":"BUY","ps":"LONG","i":42,"c":"lim-test","X":"PARTIALLY_FILLED","z":"0.4","ap":"2500.5","o":"LIMIT"}}`,
		`{"e":"ACCOUNT_UPDATE","E":1001,"a":{"P":[{"s":"ETHUSDT","ps":"LONG","pa":"0.4","ep":"2500.5"},{"s":"ETHUSDT","ps":"SHORT","pa":"-0.2","ep":"2600"}]}}`,
		`{"e":"listenKeyExpired","E":1002}`,
	} {
		var e futures.WsUserDataEvent
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			t.Fatal(err)
		}
		s.handleEvent(&e)
	}
	if len(orders) != 1 || orders[0] != (OrderUpdate{Symbol: "ETHUSDT", Side: "BUY", PositionSide: "LONG", OrderID: "42", ClientOrderID: "lim-test", Status: "PARTIALLY_FILLED", OrderType: "LIMIT", FilledQuantity: 0.4, AveragePrice: 2500.5, EventTime: 1000}) {
		t.Fatalf("orders=%+v", orders)
	}
	if len(accounts) != 1 || len(accounts[0].Positions) != 2 || accounts[0].Positions[1].Amount != -0.2 || accounts[0].Positions[1].EntryPrice != 2600 || accounts[0].EventTime != 1001 {
		t.Fatalf("accounts=%+v", accounts)
	}
	var bad futures.WsUserDataEvent
	if err := json.Unmarshal([]byte(`{"e":"ORDER_TRADE_UPDATE","o":{"z":"NaN","ap":"100"}}`), &bad); err != nil {
		t.Fatal(err)
	}
	s.handleEvent(&bad)
	s.handleEvent(nil)
	if len(orders) != 1 {
		t.Fatal("nonfinite event delivered")
	}
}
func TestRealtimeUserReconnectBackoffAndExpiry(t *testing.T) {
	var starts, dials int
	var keys, closed []string
	var waits []time.Duration
	ctx := context.Background()
	s := newUserStream(userStreamDeps{
		start:    func(context.Context) (string, error) { starts++; return string(rune('a' + starts - 1)), nil },
		closeKey: func(_ context.Context, k string) error { closed = append(closed, k); return nil },
		dial: func(k string, h futures.WsUserDataHandler, _ futures.ErrHandler) (chan struct{}, chan struct{}, error) {
			dials++
			keys = append(keys, k)
			if dials == 1 {
				h(&futures.WsUserDataEvent{Event: futures.UserDataEventTypeListenKeyExpired})
				done := make(chan struct{})
				stop := make(chan struct{})
				go func() { <-stop; close(done) }()
				return done, stop, nil
			}
			return nil, nil, errors.New("dial failed")
		},
		wait: func(_ context.Context, d time.Duration) bool { waits = append(waits, d); return len(waits) < 8 },
	})
	s.run(ctx)
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, 32 * time.Second, 32 * time.Second}
	if !reflect.DeepEqual(waits, want) {
		t.Fatalf("waits=%v", waits)
	}
	if starts != 2 || keys[0] != "a" || keys[1] != "b" || !reflect.DeepEqual(closed, []string{"a", "b"}) {
		t.Fatalf("starts=%d keys=%v closed=%v", starts, keys, closed)
	}
}
func TestRealtimeUserReconnectOnCloseAndError(t *testing.T) {
	for _, failure := range []string{"close", "error"} {
		t.Run(failure, func(t *testing.T) {
			var dials int
			var waits []time.Duration
			s := newUserStream(userStreamDeps{
				start: func(context.Context) (string, error) { return "key", nil }, closeKey: func(context.Context, string) error { return nil },
				dial: func(_ string, _ futures.WsUserDataHandler, fail futures.ErrHandler) (chan struct{}, chan struct{}, error) {
					dials++
					done := make(chan struct{})
					stop := make(chan struct{})
					if failure == "close" {
						close(done)
					} else {
						fail(errors.New("broken"))
						go func() { <-stop; close(done) }()
					}
					return done, stop, nil
				},
				wait: func(_ context.Context, d time.Duration) bool { waits = append(waits, d); return len(waits) < 3 },
			})
			s.run(context.Background())
			if dials != 3 || !reflect.DeepEqual(waits, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}) {
				t.Fatalf("dials=%d waits=%v", dials, waits)
			}
		})
	}
}
func TestRealtimeUserKeepaliveAndStop(t *testing.T) {
	alive := make(chan struct{}, 1)
	var mu sync.Mutex
	kept, closed := 0, 0
	s := newUserStream(userStreamDeps{
		start:     func(context.Context) (string, error) { return "key", nil },
		keepalive: func(context.Context, string) error { mu.Lock(); kept++; mu.Unlock(); signalStream(alive); return nil },
		closeKey:  func(context.Context, string) error { mu.Lock(); closed++; mu.Unlock(); return nil },
		dial: func(string, futures.WsUserDataHandler, futures.ErrHandler) (chan struct{}, chan struct{}, error) {
			done := make(chan struct{})
			stop := make(chan struct{})
			go func() { <-stop; close(done) }()
			return done, stop, nil
		},
		keepaliveInterval: time.Millisecond,
	})
	s.Start()
	select {
	case <-alive:
	case <-time.After(time.Second):
		t.Fatal("no keepalive")
	}
	s.Stop()
	s.Stop()
	mu.Lock()
	defer mu.Unlock()
	if kept < 1 || closed != 1 {
		t.Fatalf("keepalive=%d close=%d", kept, closed)
	}
}
func TestRealtimeUserStartFailureBackoffCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int
	s := newUserStream(userStreamDeps{start: func(context.Context) (string, error) { calls++; return "", errors.New("listen key") }, wait: func(_ context.Context, d time.Duration) bool {
		if d != time.Second {
			t.Fatalf("delay=%v", d)
		}
		cancel()
		return waitStream(ctx, d)
	}})
	s.run(ctx)
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}
func TestRealtimeCachedPositionsAndInvalidation(t *testing.T) {
	tr := &FuturesTrader{cachedPositions: []map[string]interface{}{{"symbol": "XUSDT", "positionAmt": 1.0}}, positionsCacheTime: time.Now().Add(-time.Minute)}
	p, age, ok := tr.CachedPositions()
	if !ok || age < time.Minute || age > 2*time.Minute {
		t.Fatalf("age=%v ok=%v", age, ok)
	}
	p[0]["positionAmt"] = 99.0
	if tr.cachedPositions[0]["positionAmt"] != 1.0 {
		t.Fatal("snapshot aliases cache")
	}
	tr.InvalidatePositionCache()
	if _, _, ok := tr.CachedPositions(); ok {
		t.Fatal("invalidated cache remained readable")
	}
}

func TestRealtimeUserHealthyReconnectResetsBackoff(t *testing.T) {
	now := time.Now()
	clockCalls, dials := 0, 0
	var waits []time.Duration
	s := newUserStream(userStreamDeps{
		start: func(context.Context) (string, error) { return "key", nil }, closeKey: func(context.Context, string) error { return nil },
		now: func() time.Time {
			clockCalls++
			if clockCalls == 1 {
				return now
			}
			return now.Add(time.Minute)
		},
		dial: func(string, futures.WsUserDataHandler, futures.ErrHandler) (chan struct{}, chan struct{}, error) {
			dials++
			if dials != 3 {
				return nil, nil, errors.New("dial")
			}
			done := make(chan struct{})
			close(done)
			return done, make(chan struct{}), nil
		},
		wait: func(_ context.Context, d time.Duration) bool { waits = append(waits, d); return len(waits) < 4 },
	})
	s.run(context.Background())
	if !reflect.DeepEqual(waits, []time.Duration{time.Second, 2 * time.Second, time.Second, 2 * time.Second}) {
		t.Fatalf("waits=%v", waits)
	}
}

type realtimeRoundTripper func(*http.Request) (*http.Response, error)

func (f realtimeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRealtimeUserSharedListenKeyLease(t *testing.T) {
	calls := 0
	client := futures.NewClient("realtime-lease-test", "secret")
	client.HTTPClient = &http.Client{Transport: realtimeRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	trader := &FuturesTrader{client: client}
	a, b := trader.NewUserStream(), trader.NewUserStream()
	a.deps.retain()
	b.deps.retain()
	a.deps.release()
	if err := a.deps.closeKey(context.Background(), "key"); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("stopping a peer deleted a shared listen key")
	}
	b.deps.release()
	if err := b.deps.closeKey(context.Background(), "key"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("last owner close calls=%d", calls)
	}
}
