package binance

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"nofx/logger"
)

// review 2026-10-11: fills must wake protection without waiting for REST polling.
type OrderUpdate struct {
	Symbol, Side, PositionSide, OrderID, ClientOrderID, Status, OrderType string
	FilledQuantity, AveragePrice                                          float64
	EventTime                                                             int64
}
type AccountPosition struct {
	Symbol, PositionSide string
	Amount, EntryPrice   float64
}
type AccountUpdate struct {
	Positions []AccountPosition
	EventTime int64
}

// review 2026-10-11: Binance returns the active listen key for an API key.
// Stopping one trader must not delete the key while an account peer still uses
// it. Lease changes and the final DELETE serialize with new stream starts.
type userStreamLease struct {
	sync.Mutex
	users int
}

var userStreamLeases sync.Map

func (l *userStreamLease) retain()  { l.Lock(); l.users++; l.Unlock() }
func (l *userStreamLease) release() { l.Lock(); l.users--; l.Unlock() }

type userStreamDeps struct {
	retain, release   func()
	start             func(context.Context) (string, error)
	keepalive         func(context.Context, string) error
	closeKey          func(context.Context, string) error
	dial              func(string, futures.WsUserDataHandler, futures.ErrHandler) (chan struct{}, chan struct{}, error)
	wait              func(context.Context, time.Duration) bool
	now               func() time.Time
	keepaliveInterval time.Duration
}

type UserStream struct {
	deps                userStreamDeps
	mu                  sync.RWMutex
	orderHandlers       []func(OrderUpdate)
	accountHandlers     []func(AccountUpdate)
	startOnce, stopOnce sync.Once
	cancel              context.CancelFunc
	done                chan struct{}
}

func (t *FuturesTrader) NewUserStream() *UserStream {
	leaseValue, _ := userStreamLeases.LoadOrStore(t.client.BaseURL+":"+t.client.APIKey, &userStreamLease{})
	lease := leaseValue.(*userStreamLease)
	return newUserStream(userStreamDeps{
		retain: lease.retain, release: lease.release,
		start: func(ctx context.Context) (string, error) { return t.client.NewStartUserStreamService().Do(ctx) },
		keepalive: func(ctx context.Context, key string) error {
			return t.client.NewKeepaliveUserStreamService().ListenKey(key).Do(ctx)
		},
		closeKey: func(ctx context.Context, key string) error {
			lease.Lock()
			defer lease.Unlock()
			if lease.users > 0 {
				return nil
			}
			return t.client.NewCloseUserStreamService().ListenKey(key).Do(ctx)
		},
		// v2.8.9 futures/websocket.go defaults to http.ProxyFromEnvironment;
		// gorilla rewrites wss to https before consulting it, honoring HTTPS_PROXY.
		// Do not mutate SDK-global proxy settings while other adapters are dialing.
		// The SDK's non-context Dial is bounded by its 45s handshake timeout.
		// Stop waits for an in-flight dial, then immediately closes its socket.
		dial: futures.WsUserDataServe,
	})
}
func newUserStream(d userStreamDeps) *UserStream {
	if d.wait == nil {
		d.wait = waitStream
	}
	if d.now == nil {
		d.now = time.Now
	}
	if d.keepaliveInterval == 0 {
		d.keepaliveInterval = 30 * time.Minute
	}
	return &UserStream{deps: d, done: make(chan struct{})}
}
func (s *UserStream) OnOrderUpdate(f func(OrderUpdate)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orderHandlers = append(s.orderHandlers, f)
}
func (s *UserStream) OnAccountUpdate(f func(AccountUpdate)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountHandlers = append(s.accountHandlers, f)
}
func (s *UserStream) Start() {
	s.startOnce.Do(func() {
		if s.deps.retain != nil {
			s.deps.retain()
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.mu.Lock()
		s.cancel = cancel
		s.mu.Unlock()
		go func() { defer close(s.done); defer recoverStream("user stream"); s.run(ctx) }()
	})
}
func (s *UserStream) Stop() {
	s.Start()
	s.stopOnce.Do(func() { s.mu.RLock(); cancel := s.cancel; s.mu.RUnlock(); cancel() })
	<-s.done
}
func recoverStream(where string) {
	if r := recover(); r != nil {
		logger.Errorf("Binance %s recovered panic: %v", where, r)
	}
}
func waitStream(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// review 2026-10-11: flapping connections must not hammer listen-key or WS APIs.
func streamBackoff(attempt int) time.Duration {
	if attempt > 5 {
		attempt = 5
	}
	if attempt < 0 {
		attempt = 0
	}
	return time.Second * time.Duration(1<<attempt) // capped at 32s
}
func signalStream(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
func stopSocket(done, stop chan struct{}) {
	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
}
func (s *UserStream) closeListenKey(key string) {
	if key == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.deps.closeKey(ctx, key); err != nil {
		logger.Infof("Binance user stream listen-key close failed: %v", err)
	}
}
func (s *UserStream) run(ctx context.Context) {
	key := ""
	attempt := 0
	defer func() {
		if s.deps.release != nil {
			s.deps.release()
		}
		s.closeListenKey(key)
	}()
	for ctx.Err() == nil {
		if key == "" {
			var err error
			key, err = s.deps.start(ctx)
			if err != nil || key == "" {
				logger.Infof("Binance user stream disconnected: listen-key creation failed: %v", err)
				if !s.deps.wait(ctx, streamBackoff(attempt)) {
					return
				}
				attempt++
				continue
			}
		}
		expired, failed := make(chan struct{}, 1), make(chan struct{}, 1)
		done, stop, err := s.deps.dial(key, func(e *futures.WsUserDataEvent) {
			defer recoverStream("user event handler")
			if e != nil && e.Event == futures.UserDataEventTypeListenKeyExpired {
				signalStream(expired)
				return
			}
			s.handleEvent(e)
		}, func(error) { signalStream(failed) })
		if err != nil {
			logger.Infof("Binance user stream disconnected: dial failed: %v", err)
			if !s.deps.wait(ctx, streamBackoff(attempt)) {
				return
			}
			attempt++
			continue
		}
		logger.Infof("Binance user stream connected")
		connected := s.deps.now()
		ticker := time.NewTicker(s.deps.keepaliveInterval)
		recreate := false
	connection:
		for {
			select {
			case <-ctx.Done():
				break connection
			case <-done:
				break connection
			case <-failed:
				break connection
			case <-expired:
				recreate = true
				break connection
			case <-ticker.C:
				if err := s.deps.keepalive(ctx, key); err != nil {
					logger.Infof("Binance user stream keepalive failed: %v", err)
					recreate = true
					break connection
				}
			}
		}
		ticker.Stop()
		stopSocket(done, stop)
		logger.Infof("Binance user stream disconnected")
		if recreate {
			s.closeListenKey(key)
			key = ""
		}
		if ctx.Err() != nil {
			return
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
func streamNumber(v string) (float64, error) {
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("invalid stream number")
	}
	return n, nil
}
func (s *UserStream) handleEvent(e *futures.WsUserDataEvent) {
	if e == nil {
		return
	}
	s.mu.RLock()
	orders := append([]func(OrderUpdate){}, s.orderHandlers...)
	accounts := append([]func(AccountUpdate){}, s.accountHandlers...)
	s.mu.RUnlock()
	switch e.Event {
	case futures.UserDataEventTypeOrderTradeUpdate:
		o := e.OrderTradeUpdate
		qty, err := streamNumber(o.AccumulatedFilledQty)
		if err != nil || qty < 0 {
			return
		}
		avg, err := streamNumber(o.AveragePrice)
		if err != nil || avg < 0 {
			return
		}
		update := OrderUpdate{Symbol: o.Symbol, Side: string(o.Side), PositionSide: string(o.PositionSide), OrderID: strconv.FormatInt(o.ID, 10), ClientOrderID: o.ClientOrderID, Status: string(o.Status), OrderType: string(o.Type), FilledQuantity: qty, AveragePrice: avg, EventTime: e.Time}
		for _, f := range orders {
			func() { defer recoverStream("order subscriber"); f(update) }()
		}
	case futures.UserDataEventTypeAccountUpdate:
		update := AccountUpdate{EventTime: e.Time}
		for _, p := range e.AccountUpdate.Positions {
			amt, err := streamNumber(p.Amount)
			if err != nil {
				return
			}
			entry, err := streamNumber(p.EntryPrice)
			if err != nil || entry < 0 {
				return
			}
			update.Positions = append(update.Positions, AccountPosition{Symbol: p.Symbol, PositionSide: string(p.Side), Amount: amt, EntryPrice: entry})
		}
		for _, f := range accounts {
			func() { defer recoverStream("account subscriber"); f(update) }()
		}
	}
}

// CachedPositions is a defensive copy only: the 5s loop must never turn a cache
// miss into a REST poll (review 2026-10-11). Age remains the original REST age.
func (t *FuturesTrader) CachedPositions() ([]map[string]interface{}, time.Duration, bool) {
	t.positionsCacheMutex.RLock()
	defer t.positionsCacheMutex.RUnlock()
	if t.positionsCacheTime.IsZero() || t.cachedPositions == nil {
		return nil, 0, false
	}
	result := make([]map[string]interface{}, 0, len(t.cachedPositions))
	for _, p := range t.cachedPositions {
		copy := make(map[string]interface{}, len(p))
		for k, v := range p {
			copy[k] = v
		}
		result = append(result, copy)
	}
	return result, time.Since(t.positionsCacheTime), true
}
func (t *FuturesTrader) InvalidatePositionCache() {
	t.positionsCacheMutex.Lock()
	defer t.positionsCacheMutex.Unlock()
	t.cachedPositions = nil
	t.positionsCacheTime = time.Time{}
}
