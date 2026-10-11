// Package bnweight is the single choke point for Binance USDT-M futures REST
// traffic. Several independent http.Clients (scanners, market data, the
// go-binance trader client, API handlers) talk to fapi.binance.com from one IP;
// Binance charges all of them to one 2400/min request-weight pool and answers
// overload with 429 (-1003) and, if the caller keeps hammering, a 418 IP ban.
//
// Wrap() installs an http.RoundTripper on each client that
//   - counts every futures request per endpoint with its documented weight,
//   - reads the authoritative X-MBX-USED-WEIGHT-1M header from every response,
//   - holds low-priority (scanner) traffic back before the pool saturates so
//     order placement and position reads keep their headroom, and
//   - honours 429/418 Retry-After with a global cooldown during which only
//     Critical requests are sent.
//
// Non-futures hosts (spot, Yahoo, CoinAnk, other exchanges) pass through
// untouched.
package bnweight

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/logger"
)

// Priority decides how a request behaves under load.
type Priority int

const (
	// Critical: order placement/cancel/algo orders and the position/account
	// reads protection depends on. Never delayed, never dropped (even during
	// a 429/418 cooldown).
	Critical Priority = iota
	// Normal: trader market data, order sync, API handlers. Delayed at high
	// weight, rejected fast during a cooldown.
	Normal
	// Low: scanners, backtests, tuners, history. First to be delayed.
	Low
)

func (p Priority) String() string {
	switch p {
	case Critical:
		return "critical"
	case Normal:
		return "normal"
	case Low:
		return "low"
	}
	return "unknown"
}

const (
	// HeaderUsedWeight1M carries Binance's authoritative per-IP used weight for
	// the current minute.
	HeaderUsedWeight1M = "X-MBX-USED-WEIGHT-1M"

	// DefaultLowThreshold / DefaultNormalThreshold: observed used weight (of
	// Binance's 2400/min) at which Low / Normal requests start waiting.
	DefaultLowThreshold    = 1500
	DefaultNormalThreshold = 2000

	maxGuardWait   = 60 * time.Second
	eventRetention = 5 * time.Minute // data/fundingRate pools are 5-minute windows
	summaryEvery   = 60 * time.Second
)

type prioKey struct{}

// WithPriority returns a context whose requests are metered at prio instead of
// the priority of the client they go through. It lets one shared client (e.g.
// the go-binance trader client, Critical by default) mark its non-urgent calls
// (order-history sync, statistics) as Normal/Low.
func WithPriority(ctx context.Context, prio Priority) context.Context {
	return context.WithValue(ctx, prioKey{}, prio)
}

func priorityFrom(ctx context.Context, def Priority) Priority {
	if p, ok := ctx.Value(prioKey{}).(Priority); ok {
		return p
	}
	return def
}

// ErrCooldown matches (errors.Is) the error returned for Low/Normal requests
// rejected while a 429/418 cooldown is active.
var ErrCooldown = errors.New("bnweight: binance rate-limit cooldown active")

// CooldownError is the concrete error behind ErrCooldown.
type CooldownError struct {
	Until    time.Time
	Status   int // 429 or 418 that started the cooldown
	Priority Priority
}

func (e *CooldownError) Error() string {
	return fmt.Sprintf("bnweight: binance rate-limit cooldown active until %s (HTTP %d), %s request rejected",
		e.Until.Format(time.RFC3339), e.Status, e.Priority)
}

func (e *CooldownError) Is(target error) bool { return target == ErrCooldown }

type event struct {
	t        time.Time
	endpoint string
	weight   int
	prio     Priority
	bucket   Bucket
}

// Meter holds the shared accounting state. Most code uses the package-level
// Default meter through Wrap(); tests construct isolated meters with NewMeter.
type Meter struct {
	LowThreshold    int
	NormalThreshold int

	now    func() time.Time
	sleep  func(ctx context.Context, d time.Duration) error
	jitter func() time.Duration

	mu         sync.Mutex
	extraHosts map[string]bool

	usedWeight int       // last header value observed in usedAt's minute
	usedAt     time.Time // when it was observed
	ownAtObs   int       // own estimated weight this minute when it was observed
	ownMinute  time.Time
	ownWeight  int // own estimated weight sent in ownMinute

	cooldownUntil  time.Time
	cooldownStatus int

	events []event
	total  [3]PriorityTotals

	holdLogged [3]time.Time

	reporterOnce sync.Once
	autoReport   bool
}

// NewMeter returns an isolated meter (no background reporter).
func NewMeter() *Meter {
	return &Meter{
		LowThreshold:    DefaultLowThreshold,
		NormalThreshold: DefaultNormalThreshold,
		now:             time.Now,
		sleep:           sleepCtx,
		jitter:          func() time.Duration { return time.Duration(rand.Int63n(int64(2 * time.Second))) },
		extraHosts:      map[string]bool{},
	}
}

var defaultMeter = func() *Meter {
	m := NewMeter()
	m.autoReport = true
	return m
}()

// Default returns the process-wide meter.
func Default() *Meter { return defaultMeter }

// Wrap installs the default meter on base (nil = http.DefaultTransport) at the
// given priority. Wrapping an already-wrapped transport re-wraps the inner one
// so requests are never counted twice.
func Wrap(base http.RoundTripper, prio Priority) http.RoundTripper {
	return defaultMeter.Wrap(base, prio)
}

// RegisterHost adds an extra host (e.g. a configured mirror) that the default
// meter treats as a Binance futures endpoint.
func RegisterHost(host string) { defaultMeter.RegisterHost(host) }

// Snapshot returns the default meter's current state.
func Snapshot() SnapshotData { return defaultMeter.Snapshot() }

// InCooldown reports whether the default meter is inside a 429/418 cooldown,
// so optional work can be skipped instead of failing.
func InCooldown() (time.Time, bool) { return defaultMeter.InCooldown() }

// Wrap returns a RoundTripper that meters/guards futures traffic through m.
func (m *Meter) Wrap(base http.RoundTripper, prio Priority) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if t, ok := base.(*transport); ok && t.m == m {
		base = t.base
	}
	return &transport{m: m, base: base, prio: prio}
}

// RegisterHost marks host (with or without port) as a futures endpoint.
func (m *Meter) RegisterHost(host string) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return
	}
	m.mu.Lock()
	m.extraHosts[host] = true
	m.extraHosts[stripPort(host)] = true
	m.mu.Unlock()
}

func stripPort(h string) string {
	if i := strings.LastIndexByte(h, ':'); i >= 0 && !strings.Contains(h[i:], "]") {
		return h[:i]
	}
	return h
}

func (m *Meter) isFuturesHost(host string) bool {
	host = strings.ToLower(host)
	bare := stripPort(host)
	if bare == "fapi.binance.com" || strings.HasSuffix(bare, "binancefuture.com") ||
		(strings.HasPrefix(bare, "fapi") && strings.HasSuffix(bare, ".binance.com")) {
		return true
	}
	if v := os.Getenv("BINANCE_FAPI_BASE"); v != "" {
		if u, err := url.Parse(v); err == nil && u.Host != "" && (strings.EqualFold(u.Host, host) || strings.EqualFold(stripPort(u.Host), bare)) {
			return true
		}
	}
	m.mu.Lock()
	ok := m.extraHosts[host] || m.extraHosts[bare]
	m.mu.Unlock()
	return ok
}

type transport struct {
	m    *Meter
	base http.RoundTripper
	prio Priority
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL == nil || !t.m.isFuturesHost(req.URL.Host) {
		return t.base.RoundTrip(req)
	}
	if t.m.autoReport {
		t.m.reporterOnce.Do(func() { go t.m.reportLoop() })
	}
	prio := priorityFrom(req.Context(), t.prio)
	endpoint, weight, bucket := Classify(req.Method, req.URL.Path, req.URL.Query())
	if err := t.m.admit(req.Context(), prio); err != nil {
		if req.Body != nil {
			req.Body.Close()
		}
		return nil, err
	}
	t.m.record(prio, endpoint, weight, bucket)
	resp, err := t.base.RoundTrip(req)
	if err == nil {
		t.m.observe(resp, prio, endpoint)
	}
	return resp, err
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (m *Meter) rollOwnLocked(now time.Time) {
	minute := now.Truncate(time.Minute)
	if !minute.Equal(m.ownMinute) {
		m.ownMinute = minute
		m.ownWeight = 0
	}
}

// effectiveUsedLocked estimates the IP weight used in the current minute: the
// last header observed this minute plus what we have sent since, or our own
// estimate when no header was seen yet this minute.
func (m *Meter) effectiveUsedLocked(now time.Time) int {
	m.rollOwnLocked(now)
	if !m.usedAt.IsZero() && m.usedAt.Truncate(time.Minute).Equal(now.Truncate(time.Minute)) {
		d := m.ownWeight - m.ownAtObs
		if d < 0 {
			d = 0
		}
		return m.usedWeight + d
	}
	return m.ownWeight
}

func (m *Meter) threshold(p Priority) int {
	if p == Low {
		return m.LowThreshold
	}
	return m.NormalThreshold
}

// admit implements the guard. Critical always passes. Low/Normal fail fast
// with ErrCooldown during a cooldown and otherwise wait (at most maxGuardWait
// in total) until the next minute boundary when the minute's weight is high.
func (m *Meter) admit(ctx context.Context, prio Priority) error {
	if prio == Critical {
		return nil
	}
	waited := time.Duration(0)
	for {
		now := m.now()
		m.mu.Lock()
		if now.Before(m.cooldownUntil) {
			err := &CooldownError{Until: m.cooldownUntil, Status: m.cooldownStatus, Priority: prio}
			m.mu.Unlock()
			return err
		}
		used := m.effectiveUsedLocked(now)
		thr := m.threshold(prio)
		if used < thr {
			m.mu.Unlock()
			return nil
		}
		logIt := now.Sub(m.holdLogged[prio]) >= 30*time.Second
		if logIt {
			m.holdLogged[prio] = now
		}
		m.mu.Unlock()

		if waited >= maxGuardWait {
			return nil // never starve a caller forever; Binance still enforces the hard limit
		}
		wait := now.Truncate(time.Minute).Add(time.Minute).Sub(now) + m.jitter()
		if wait > maxGuardWait-waited {
			wait = maxGuardWait - waited
		}
		if logIt {
			logger.Infof("⏳ Binance weight guard: used≈%d ≥ %d, holding %s requests %s until the next minute", used, thr, prio, wait.Round(100*time.Millisecond))
		}
		if err := m.sleep(ctx, wait); err != nil {
			return err
		}
		waited += wait
	}
}

func (m *Meter) record(prio Priority, endpoint string, weight int, bucket Bucket) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollOwnLocked(now)
	if bucket == BucketIP1M {
		m.ownWeight += weight
	}
	m.events = append(m.events, event{t: now, endpoint: endpoint, weight: weight, prio: prio, bucket: bucket})
	m.trimLocked(now)
	if int(prio) >= 0 && int(prio) < len(m.total) {
		tp := &m.total[prio]
		tp.Requests++
		if bucket == BucketIP1M {
			tp.Weight += weight
		}
	}
}

func (m *Meter) trimLocked(now time.Time) {
	cut := now.Add(-eventRetention)
	i := 0
	for i < len(m.events) && m.events[i].t.Before(cut) {
		i++
	}
	if i > 0 {
		// Reslice (O(1)); append reallocates only the live tail when the
		// backing array fills, so retired events are not kept forever.
		m.events = m.events[i:]
	}
}

// observe reads the used-weight header and reacts to 429/418.
func (m *Meter) observe(resp *http.Response, prio Priority, endpoint string) {
	now := m.now()
	if v := resp.Header.Get(HeaderUsedWeight1M); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			m.mu.Lock()
			m.rollOwnLocked(now)
			if !m.usedAt.IsZero() && m.usedAt.Truncate(time.Minute).Equal(now.Truncate(time.Minute)) && n < m.usedWeight {
				n = m.usedWeight // monotonic within a minute; a late response must not rewind it
			}
			m.usedWeight = n
			m.usedAt = now
			m.ownAtObs = m.ownWeight
			m.mu.Unlock()
		}
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == 418 {
		m.startCooldown(now, resp.StatusCode, parseRetryAfter(resp.Header.Get("Retry-After"), now), prio, endpoint)
	}
}

// parseRetryAfter accepts delta-seconds or an HTTP date; 0 means "absent/invalid".
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
		return time.Duration(n * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

func (m *Meter) startCooldown(now time.Time, status int, retryAfter time.Duration, prio Priority, endpoint string) {
	if retryAfter <= 0 {
		if status == 418 {
			retryAfter = 5 * time.Minute
		} else {
			retryAfter = now.Truncate(time.Minute).Add(time.Minute).Sub(now) + 2*time.Second
		}
	}
	if retryAfter > 24*time.Hour {
		retryAfter = 24 * time.Hour
	}
	until := now.Add(retryAfter)
	m.mu.Lock()
	fresh := !now.Before(m.cooldownUntil) // previous cooldown (if any) already over
	if until.After(m.cooldownUntil) {
		m.cooldownUntil = until
		m.cooldownStatus = status
	}
	used := m.effectiveUsedLocked(now)
	m.mu.Unlock()
	if fresh {
		logger.Warnf("🚨 BINANCE RATE LIMIT HIT: HTTP %d on %s (%s priority), used weight≈%d. Low/Normal futures requests are rejected until %s (%s); Critical requests still go through. Continued hammering after 429 escalates to a 418 IP ban.",
			status, endpoint, prio, used, until.Format("15:04:05"), retryAfter.Round(time.Second))
	}
}

// InCooldown reports whether a 429/418 cooldown is active.
func (m *Meter) InCooldown() (time.Time, bool) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cooldownUntil, now.Before(m.cooldownUntil)
}

// PriorityTotals are cumulative per-priority counters since process start.
type PriorityTotals struct {
	Requests int
	Weight   int
}

// EndpointStat aggregates the last minute of one endpoint.
type EndpointStat struct {
	Endpoint string
	Count    int
	Weight   int
}

// SnapshotData is a point-in-time view of the meter (tests/debugging).
type SnapshotData struct {
	UsedWeight    int       // last X-MBX-USED-WEIGHT-1M header value (0 if never seen)
	UsedAt        time.Time // when that header was observed
	UsedFresh     bool      // observed within the current minute
	EffectiveUsed int       // header + own weight since, the value the guard compares to its thresholds
	EstLastMinute int       // own estimated weight sent in the last 60 s
	FundingRate5m int       // fundingRate requests in the last 5 min (limit 500)
	Data5m        int       // futures/data/* requests in the last 5 min (limit 1000)
	CooldownUntil time.Time
	InCooldown    bool
	// Endpoints is the last 60 s per endpoint, sorted by estimated weight then count.
	Endpoints []EndpointStat
	// ByPriority is the last 60 s per priority.
	ByPriority [3]PriorityTotals
	// Totals are cumulative since start.
	Totals [3]PriorityTotals
}

// Snapshot returns the meter's current state.
func (m *Meter) Snapshot() SnapshotData {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trimLocked(now)
	s := SnapshotData{
		UsedWeight:    m.usedWeight,
		UsedAt:        m.usedAt,
		UsedFresh:     !m.usedAt.IsZero() && m.usedAt.Truncate(time.Minute).Equal(now.Truncate(time.Minute)),
		EffectiveUsed: m.effectiveUsedLocked(now),
		CooldownUntil: m.cooldownUntil,
		InCooldown:    now.Before(m.cooldownUntil),
		Totals:        m.total,
	}
	minuteAgo := now.Add(-time.Minute)
	agg := map[string]*EndpointStat{}
	for _, e := range m.events {
		switch e.bucket {
		case BucketFundingRate:
			s.FundingRate5m++
		case BucketData:
			s.Data5m++
		}
		if e.t.Before(minuteAgo) {
			continue
		}
		s.EstLastMinute += e.weight
		st := agg[e.endpoint]
		if st == nil {
			st = &EndpointStat{Endpoint: e.endpoint}
			agg[e.endpoint] = st
		}
		st.Count++
		st.Weight += e.weight
		if int(e.prio) >= 0 && int(e.prio) < len(s.ByPriority) {
			s.ByPriority[e.prio].Requests++
			s.ByPriority[e.prio].Weight += e.weight
		}
	}
	for _, st := range agg {
		s.Endpoints = append(s.Endpoints, *st)
	}
	sort.Slice(s.Endpoints, func(i, j int) bool {
		a, b := s.Endpoints[i], s.Endpoints[j]
		if a.Weight != b.Weight {
			return a.Weight > b.Weight
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Endpoint < b.Endpoint
	})
	return s
}

// SummaryLine renders the periodic one-line report (top 8 endpoints).
func (m *Meter) SummaryLine() string {
	s := m.Snapshot()
	now := m.now()
	var sb strings.Builder
	if s.UsedAt.IsZero() {
		sb.WriteString("📊 Binance weight: header=n/a")
	} else {
		fresh := ""
		if !s.UsedFresh {
			fresh = " stale"
		}
		fmt.Fprintf(&sb, "📊 Binance weight: header=%d/2400 (%ds ago%s)", s.UsedWeight, int(now.Sub(s.UsedAt).Seconds()), fresh)
	}
	fmt.Fprintf(&sb, " ownEst1m=%d", s.EstLastMinute)
	if s.InCooldown {
		fmt.Fprintf(&sb, " COOLDOWN until %s", s.CooldownUntil.Format("15:04:05"))
	}
	fmt.Fprintf(&sb, " | prio critical=%dw/%dr normal=%dw/%dr low=%dw/%dr",
		s.ByPriority[Critical].Weight, s.ByPriority[Critical].Requests,
		s.ByPriority[Normal].Weight, s.ByPriority[Normal].Requests,
		s.ByPriority[Low].Weight, s.ByPriority[Low].Requests)
	fmt.Fprintf(&sb, " | fundingRate5m=%d/500 data5m=%d/1000 | top:", s.FundingRate5m, s.Data5m)
	n := len(s.Endpoints)
	if n > 8 {
		n = 8
	}
	for i := 0; i < n; i++ {
		e := s.Endpoints[i]
		fmt.Fprintf(&sb, " %s=%dw/%dx", e.Endpoint, e.Weight, e.Count)
	}
	return sb.String()
}

func (m *Meter) reportLoop() {
	t := time.NewTicker(summaryEvery)
	defer t.Stop()
	for range t.C {
		s := m.Snapshot()
		if len(s.Endpoints) == 0 && s.FundingRate5m == 0 && s.Data5m == 0 && !s.InCooldown {
			continue // idle
		}
		logger.Info(m.SummaryLine())
	}
}
