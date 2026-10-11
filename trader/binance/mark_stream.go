package binance

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"nofx/logger"
)

type markSample struct {
	price float64
	at    time.Time
}
type markStreamDeps struct {
	dial func(map[string]time.Duration, futures.WsMarkPriceHandler, futures.ErrHandler) (chan struct{}, chan struct{}, error)
	wait func(context.Context, time.Duration) bool
	now  func() time.Time
}

// review 2026-10-11: stream only the AI-managed open-symbol set; no REST prices.
type MarkStream struct {
	deps                markStreamDeps
	mu                  sync.RWMutex
	symbols             []string
	prices              map[string]markSample
	changed             chan struct{}
	done                chan struct{}
	cancel              context.CancelFunc
	startOnce, stopOnce sync.Once
}

func (*FuturesTrader) NewMarkStream() *MarkStream {
	return newMarkStream(markStreamDeps{dial: futures.WsCombinedMarkPriceServeWithRate})
}
func newMarkStream(d markStreamDeps) *MarkStream {
	if d.wait == nil {
		d.wait = waitStream
	}
	if d.now == nil {
		d.now = time.Now
	}
	return &MarkStream{deps: d, prices: map[string]markSample{}, changed: make(chan struct{}, 1), done: make(chan struct{})}
}
func (s *MarkStream) Start() {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.mu.Lock()
		s.cancel = cancel
		s.mu.Unlock()
		go func() { defer close(s.done); defer recoverStream("mark stream"); s.run(ctx) }()
	})
}
func (s *MarkStream) Stop() {
	s.Start()
	s.stopOnce.Do(func() { s.mu.RLock(); c := s.cancel; s.mu.RUnlock(); c() })
	<-s.done
}
func (s *MarkStream) SetSymbols(symbols []string) {
	set := map[string]bool{}
	for _, symbol := range symbols {
		if symbol != "" {
			set[strings.ToUpper(symbol)] = true
		}
	}
	next := make([]string, 0, len(set))
	for symbol := range set {
		next = append(next, symbol)
	}
	sort.Strings(next)
	s.mu.Lock()
	if strings.Join(next, ",") == strings.Join(s.symbols, ",") {
		s.mu.Unlock()
		return
	}
	s.symbols = next
	for symbol := range s.prices {
		if !set[symbol] {
			delete(s.prices, symbol)
		}
	}
	s.mu.Unlock()
	signalStream(s.changed)
}
func (s *MarkStream) MarkPrice(symbol string) (float64, time.Duration, bool) {
	s.mu.RLock()
	p, ok := s.prices[strings.ToUpper(symbol)]
	s.mu.RUnlock()
	return p.price, s.deps.now().Sub(p.at), ok
}
func (s *MarkStream) symbolSnapshot() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.symbols...)
}
func (s *MarkStream) run(ctx context.Context) {
	attempt := 0
	for ctx.Err() == nil {
		// Drain a coalesced notification before taking the latest subscription set.
		select {
		case <-s.changed:
		default:
		}
		symbols := s.symbolSnapshot()
		if len(symbols) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-s.changed:
				continue
			}
		}
		rates := map[string]time.Duration{}
		for _, symbol := range symbols {
			rates[symbol] = time.Second
		}
		failed := make(chan struct{}, 1)
		done, stop, err := s.deps.dial(rates, func(e *futures.WsMarkPriceEvent) {
			defer recoverStream("mark handler")
			if e == nil {
				return
			}
			price, err := streamNumber(e.MarkPrice)
			if err != nil || price <= 0 {
				return
			}
			now := s.deps.now()
			at := time.UnixMilli(e.Time)
			if e.Time <= 0 || at.After(now) {
				at = now
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			// Ignore late packets from the connection being retired.
			symbol := strings.ToUpper(e.Symbol)
			for _, active := range s.symbols {
				if active == symbol {
					if previous := s.prices[symbol]; previous.at.After(at) {
						return
					}
					s.prices[symbol] = markSample{price, at}
					break
				}
			}
		}, func(error) { signalStream(failed) })
		if err != nil {
			logger.Infof("Binance mark stream disconnected: dial failed: %v", err)
			if !s.deps.wait(ctx, streamBackoff(attempt)) {
				return
			}
			attempt++
			continue
		}
		logger.Infof("Binance mark stream connected (%d symbols)", len(symbols))
		connected := s.deps.now()
		changed := false
		select {
		case <-ctx.Done():
		case <-done:
		case <-failed:
		case <-s.changed:
			changed = true
		}
		stopSocket(done, stop)
		logger.Infof("Binance mark stream disconnected")
		if ctx.Err() != nil {
			return
		}
		if changed {
			attempt = 0
			continue
		}
		if s.deps.now().Sub(connected) >= time.Minute {
			attempt = 0
		}
		if !s.deps.wait(ctx, streamBackoff(attempt)) {
			return
		}
		attempt++
	}
}
