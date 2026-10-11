package binance

import (
	"reflect"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

func TestRealtimeMarkSymbolsResubscribeAndFreshness(t *testing.T) {
	type connection struct {
		rates   map[string]time.Duration
		handler futures.WsMarkPriceHandler
	}
	connections := make(chan connection, 4)
	now := time.Now()
	s := newMarkStream(markStreamDeps{now: func() time.Time { return now }, dial: func(r map[string]time.Duration, h futures.WsMarkPriceHandler, _ futures.ErrHandler) (chan struct{}, chan struct{}, error) {
		done := make(chan struct{})
		stop := make(chan struct{})
		go func() { <-stop; close(done) }()
		connections <- connection{r, h}
		return done, stop, nil
	}})
	next := func() connection {
		t.Helper()
		select {
		case c := <-connections:
			return c
		case <-time.After(time.Second):
			t.Fatal("no subscription")
			return connection{}
		}
	}
	s.SetSymbols([]string{"ethusdt", "BTCUSDT", "ETHUSDT"})
	s.Start()
	defer s.Stop()
	c := next()
	if !reflect.DeepEqual(c.rates, map[string]time.Duration{"ETHUSDT": time.Second, "BTCUSDT": time.Second}) {
		t.Fatalf("rates=%v", c.rates)
	}
	c.handler(&futures.WsMarkPriceEvent{Symbol: "ETHUSDT", MarkPrice: "2500", Time: now.Add(-11 * time.Second).UnixMilli()})
	price, age, ok := s.MarkPrice("ethusdt")
	if !ok || price != 2500 || age < 11*time.Second {
		t.Fatalf("price=%g age=%v ok=%v", price, age, ok)
	}
	s.SetSymbols([]string{"BTCUSDT", "ETHUSDT"})
	select {
	case <-connections:
		t.Fatal("equal symbol set reconnected")
	default:
	}
	s.SetSymbols([]string{"SOLUSDT"})
	c = next()
	if len(c.rates) != 1 || c.rates["SOLUSDT"] != time.Second {
		t.Fatalf("rates=%v", c.rates)
	}
	if _, _, ok := s.MarkPrice("ETHUSDT"); ok {
		t.Fatal("retired symbol price retained")
	}
	c.handler(&futures.WsMarkPriceEvent{Symbol: "SOLUSDT", MarkPrice: "NaN"})
	if _, _, ok := s.MarkPrice("SOLUSDT"); ok {
		t.Fatal("nonfinite price cached")
	}
	c.handler(&futures.WsMarkPriceEvent{Symbol: "SOLUSDT", MarkPrice: "150", Time: now.UnixMilli()})
	if p, _, ok := s.MarkPrice("SOLUSDT"); !ok || p != 150 {
		t.Fatal("valid price missing")
	}
	c.handler(&futures.WsMarkPriceEvent{Symbol: "SOLUSDT", MarkPrice: "999", Time: now.Add(-time.Second).UnixMilli()})
	if price, _, _ := s.MarkPrice("SOLUSDT"); price != 150 {
		t.Fatal("older mark packet replaced the latest price")
	}
	s.SetSymbols(nil)
	s.Stop()
}
