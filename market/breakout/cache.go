package breakout

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Shared response caches for the scanners.
//
// Several scanners (breakout, short scan, slow-top, trend-short, tuners) run on
// 5-30 minute cadences over overlapping symbol sets and used to re-download
// identical Binance payloads every pass — the bulk of the bot's request weight
// (klines 15m/865 = 5, depth = 5, ticker/24hr without a symbol = 40, ...) on
// the single IP-wide 2400/min budget. Two caches remove the redundancy:
//
//   - klines: fetchKlines already drops the still-forming candle, so its result
//     can only change when the next bar CLOSES. An entry is therefore keyed by
//     the full request URL and is valid until the forming bar's close time
//     (+ klineCacheSlack): results are bit-identical to an uncached fetch.
//   - JSON bodies with an explicit TTL (ticker/24hr, premiumIndex, fundingRate,
//     futures/data ratios) — slow-moving or snapshot data where a bounded
//     staleness (documented at each call site) is fine.

const (
	klineCacheSlack = 2 * time.Second
	cacheMaxEntries = 2000

	// tickerCacheTTL: ticker/24hr (weight 40, full-universe snapshot) is read
	// by the volume universe, the board lists, the short scan and the slow-top
	// prefilter within one scheduler pass; a 2-minute snapshot serves them all
	// (the short scan already caches its own result for 2 minutes).
	tickerCacheTTL = 2 * time.Minute
	// fundingCacheTTL: fundingRate history changes only at settlement (every
	// 1-8 h) and has its own 500 req / 5 min / IP limit.
	fundingCacheTTL = 10 * time.Minute
	// premiumCacheTTL: only lastFundingRate is read from premiumIndex. That
	// value drifts continuously between settlements (market.getFundingRate
	// documents a 1 h cache serving figures 2x off), so keep the staleness
	// well inside that path's 5-minute contract: 2 minutes, enough for the
	// short scan's own 2-minute result window and the tuners to share it.
	premiumCacheTTL = 2 * time.Minute
	// oiHistCacheTTL: openInterestHist(period=5m) gains a sample every 5 min;
	// 4 min lets the breakout and short scans of one pass share a download but
	// never serves the previous pass.
	oiHistCacheTTL = 4 * time.Minute
	// ratioCacheTTL: global long/short account ratio, period=1h.
	ratioCacheTTL = 10 * time.Minute
)

// cacheEnabled is switched off by the package tests (they swap binanceHTTP per
// test and assert on fresh requests); cache behaviour has its own tests.
var cacheEnabled = true

// cacheNow is time.Now, replaceable in tests.
var cacheNow = time.Now

type klineCacheEntry struct {
	bars    []Kline
	expires time.Time
}

type bodyCacheEntry struct {
	body    []byte
	expires time.Time
}

var (
	respCacheMu sync.Mutex
	klineCache  = map[string]klineCacheEntry{}
	bodyCache   = map[string]bodyCacheEntry{}
)

func klineCacheGet(key string) ([]Kline, bool) {
	if !cacheEnabled {
		return nil, false
	}
	now := cacheNow()
	respCacheMu.Lock()
	defer respCacheMu.Unlock()
	e, ok := klineCache[key]
	if !ok || !now.Before(e.expires) {
		return nil, false
	}
	return append([]Kline(nil), e.bars...), true
}

func klineCachePut(key string, bars []Kline, expires time.Time) {
	if !cacheEnabled || len(bars) == 0 {
		return
	}
	respCacheMu.Lock()
	defer respCacheMu.Unlock()
	sweepLocked(cacheNow())
	klineCache[key] = klineCacheEntry{bars: append([]Kline(nil), bars...), expires: expires}
}

func bodyCacheGet(key string) ([]byte, bool) {
	if !cacheEnabled {
		return nil, false
	}
	now := cacheNow()
	respCacheMu.Lock()
	defer respCacheMu.Unlock()
	e, ok := bodyCache[key]
	if !ok || !now.Before(e.expires) {
		return nil, false
	}
	return e.body, true
}

func bodyCachePut(key string, body []byte, ttl time.Duration) {
	if !cacheEnabled || ttl <= 0 {
		return
	}
	now := cacheNow()
	respCacheMu.Lock()
	defer respCacheMu.Unlock()
	sweepLocked(now)
	bodyCache[key] = bodyCacheEntry{body: body, expires: now.Add(ttl)}
}

// sweepLocked bounds memory: drop expired entries once the maps grow large,
// and everything if that is not enough.
func sweepLocked(now time.Time) {
	if len(klineCache)+len(bodyCache) < cacheMaxEntries {
		return
	}
	for k, e := range klineCache {
		if !now.Before(e.expires) {
			delete(klineCache, k)
		}
	}
	for k, e := range bodyCache {
		if !now.Before(e.expires) {
			delete(bodyCache, k)
		}
	}
	if len(klineCache)+len(bodyCache) >= cacheMaxEntries {
		klineCache = map[string]klineCacheEntry{}
		bodyCache = map[string]bodyCacheEntry{}
	}
}

// resetResponseCaches empties both caches (tests).
func resetResponseCaches() {
	respCacheMu.Lock()
	klineCache = map[string]klineCacheEntry{}
	bodyCache = map[string]bodyCacheEntry{}
	respCacheMu.Unlock()
}

// fetchJSONCached is fetchJSON with a TTL'd raw-body cache keyed by URL. Only
// successful (HTTP 200, decodable) responses are cached; errors always retry.
func fetchJSONCached(rawURL string, ttl time.Duration, out interface{}) error {
	if body, ok := bodyCacheGet(rawURL); ok {
		if err := json.Unmarshal(body, out); err == nil {
			return nil
		}
	}
	body, err := fetchBody(rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode failed: %w", err)
	}
	bodyCachePut(rawURL, body, ttl)
	return nil
}
