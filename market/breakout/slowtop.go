package breakout

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"nofx/logger"
)

// Slow-top short universe: the 24h-gainer screen only catches pumps that
// already went vertical — the other classic short family, a multi-day grind
// sideways at the highs that slowly rolls over, never enters that list. This
// universe screens ALL liquid USDT perps for symbols sitting within 5% of
// their 90-day high, runs the same AnalyzeShort scoring, and keeps only
// those printing a 4h bearish divergence (the user-specified entry rule).
//
// Cost model: one fapi ticker call (all symbols) + one 1d-klines call per
// liquid symbol for the high-proximity prefilter + full AnalyzeShort
// (~6 calls) only for near-high survivors. Refreshed every 30 minutes by the
// scheduler — roughly 12k requests/day, a rounding error against the fapi
// weight budget.

const (
	slowTopCacheTTL = 30 * time.Minute
	// slowTopRefreshTTL gates refreshSlowTops and must stay BELOW the 30-minute
	// scheduler tick: slowTopAt is stamped when a run finishes, so at the next
	// tick only (30m minus runtime) has elapsed and a 30m guard skipped every
	// other tick (review 2026-10-11). slowTopCacheTTL keeps sizing the snapshot
	// max-age (2×TTL) and is deliberately unchanged.
	slowTopRefreshTTL   = 25 * time.Minute
	slowTopMinQuoteVol  = 30_000_000 // $30M/day liquidity floor for the prefilter
	slowTopNearHighPct  = 5.0        // live price within 5% of the 90-day high
	slowTopConcurrent   = 6
	slowTopMaxKeep      = 15 // cap merged-list growth
	slowTopPrefilterCap = 60 // analyze at most this many near-high survivors per run
)

var (
	slowTopMu       sync.Mutex
	slowTopCache    []ShortSignal
	slowTopAt       time.Time
	slowTopInflight bool
)

// slowTopSnapshot returns the last computed slow-top results (may be empty
// before the first refresh; ScanShorts merges whatever is available). A cache
// older than 2×TTL is dropped entirely (2026-10-03 review): the refresher
// keeps the previous list on failures — merging an arbitrarily old near_high
// board would feed the fetch-side OI exemption (which trusts the scan's
// recency) with rotted symbols.
func slowTopSnapshot() []ShortSignal {
	slowTopMu.Lock()
	defer slowTopMu.Unlock()
	if !slowTopAt.IsZero() && time.Since(slowTopAt) > 2*slowTopCacheTTL {
		return nil
	}
	out := make([]ShortSignal, len(slowTopCache))
	copy(out, slowTopCache)
	return out
}

// refreshSlowTops recomputes the slow-top universe if the cache is stale.
// Best-effort: failures keep the previous cache; concurrent callers share
// one in-flight run. Recovers from panics — a market-data hiccup must never
// take down trading.
func refreshSlowTops() {
	slowTopMu.Lock()
	if time.Now().Before(slowTopAt.Add(slowTopRefreshTTL)) || slowTopInflight {
		slowTopMu.Unlock()
		return
	}
	slowTopInflight = true
	slowTopMu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("🩸 Slow-top scan panicked (recovered): %v", r)
		}
		slowTopMu.Lock()
		slowTopInflight = false
		slowTopMu.Unlock()
	}()

	start := time.Now()
	candidates, err := slowTopUniverse()
	if err != nil {
		logger.Warnf("⚠️ Slow-top universe failed: %v", err)
		return
	}
	btc4h, _ := NewBinanceDS("BTCUSDT").Klines("4h", 84)

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		sem  = make(chan struct{}, slowTopConcurrent)
		keep = make([]ShortSignal, 0, 8)
	)
	for _, c := range candidates {
		wg.Add(1)
		go func(symbol, chgStr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			chg, _ := strconv.ParseFloat(chgStr, 64)
			sig, err := AnalyzeShort(symbol, chg, btc4h, NewBinanceDS(symbol))
			if err != nil || sig == nil {
				return
			}
			// Same new-listing rule as the gainer path: < 7 days of history
			// is un-scoreable structure.
			if sig.ListingDays > 0 && sig.ListingDays < 7 {
				return
			}
			// User-specified rule: 90d-high proximity AND 4h bearish divergence.
			if !sig.BearishDiv4h {
				return
			}
			sig.Universe = "near_high"
			mu.Lock()
			keep = append(keep, *sig)
			mu.Unlock()
		}(c.Symbol, c.ChgPct)
	}
	wg.Wait()

	sort.SliceStable(keep, func(i, j int) bool { return keep[i].Score > keep[j].Score })
	if len(keep) > slowTopMaxKeep {
		keep = keep[:slowTopMaxKeep]
	}

	slowTopMu.Lock()
	slowTopCache = keep
	slowTopAt = time.Now()
	slowTopMu.Unlock()
	logger.Infof("🩸 Slow-top scan: %d/%d near-high symbols with 4h divergence in %v",
		len(keep), len(candidates), time.Since(start).Round(time.Millisecond))
}

// slowTopCandidate is one prefilter survivor (chg kept as raw ticker string —
// only AnalyzeShort consumes it).
type slowTopCandidate struct {
	Symbol      string
	ChgPct      string
	LastPrice   float64 // live 24h-ticker price the 90d-high prefilter measures from
	QuoteVolume float64
}

// slowTopUniverse lists liquid perps whose live price sits within
// slowTopNearHighPct% of their 90-day high. Two data touches per symbol
// (shared ticker batch + one 1d-klines fetch each), concurrency-capped.
func slowTopUniverse() ([]slowTopCandidate, error) {
	tickers, err := allPerpTickers()
	if err != nil {
		return nil, err
	}
	var uni []slowTopCandidate
	for _, t := range tickers {
		if t.QuoteVolume >= slowTopMinQuoteVol {
			// The live ticker price, not yesterday's close, feeds the prefilter.
			// An unusable price falls back to the closed bar (see near90dHigh).
			last, err := strconv.ParseFloat(t.LastPrice, 64)
			if err != nil || math.IsNaN(last) || math.IsInf(last, 0) {
				last = 0
			}
			uni = append(uni, slowTopCandidate{Symbol: t.Symbol, ChgPct: t.ChgPct, LastPrice: last, QuoteVolume: t.QuoteVolume})
		}
	}
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, slowTopConcurrent)
		out = make([]slowTopCandidate, 0, 64)
	)
	for _, c := range uni {
		wg.Add(1)
		go func(c slowTopCandidate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			k, err := NewBinanceDS(c.Symbol).Klines("1d", 90)
			if err != nil {
				return
			}
			if near90dHigh(k, c.LastPrice, slowTopNearHighPct) {
				mu.Lock()
				out = append(out, c)
				mu.Unlock()
			}
		}(c)
	}
	wg.Wait()
	if len(out) > slowTopPrefilterCap {
		// Keep the most liquid survivors — liquidity is execution safety.
		sort.SliceStable(out, func(i, j int) bool { return out[i].QuoteVolume > out[j].QuoteVolume })
		out = out[:slowTopPrefilterCap]
	}
	return out, nil
}

// perpTicker is the minimal slice of fapi ticker/24hr the scanners need.
type perpTicker struct {
	Symbol      string
	ChgPct      string
	LastPrice   string
	QuoteVolume float64
}

// allPerpTickers returns every tradable USDT-M perp (same ASCII/suffix
// filters as the other universe builders, no volume cut here).
func allPerpTickers() ([]perpTicker, error) {
	u := fapiBase() + "/fapi/v1/ticker/24hr"
	var raw []struct {
		Symbol             string `json:"symbol"`
		LastPrice          string `json:"lastPrice"`
		PriceChangePercent string `json:"priceChangePercent"`
		QuoteVolume        string `json:"quoteVolume"`
	}
	if err := fetchJSONCached(u, tickerCacheTTL, &raw); err != nil {
		return nil, err
	}
	out := make([]perpTicker, 0, len(raw))
	for _, t := range raw {
		if !stringsHasSuffixUSDT(t.Symbol) || stringsContainsUnderscore(t.Symbol) {
			continue
		}
		if !isASCIIAlnum(t.Symbol[:len(t.Symbol)-4]) {
			continue
		}
		v, err := strconv.ParseFloat(t.QuoteVolume, 64)
		if err != nil || math.IsNaN(v) {
			continue
		}
		out = append(out, perpTicker{Symbol: t.Symbol, ChgPct: t.PriceChangePercent, LastPrice: t.LastPrice, QuoteVolume: v})
	}
	return out, nil
}

func stringsHasSuffixUSDT(s string) bool {
	return len(s) > 4 && s[len(s)-4:] == "USDT"
}

func stringsContainsUnderscore(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			return true
		}
	}
	return false
}

var _ = fmt.Sprintf // keep fmt if unused after refactors

// near90dHigh reports whether the live price sits within maxDistPct% of the
// series' 90-bar high. The series holds CLOSED daily bars (fetchKlines drops
// the forming candle), so its last close is yesterday's: a coin already down
// 15% today would still look near the high. livePrice (the 24h-ticker last
// price) is therefore used whenever it is positive, else the last closed
// close. A live price above the high sits at the high and qualifies.
// Guarded: short/dirty series return false (fail-open — the symbol just
// doesn't qualify).
func near90dHigh(k []Kline, livePrice, maxDistPct float64) bool {
	if len(k) < 20 {
		return false
	}
	high := 0.0
	for _, bar := range k {
		if bar.High > high {
			high = bar.High
		}
	}
	last := k[len(k)-1].Close
	if livePrice > 0 {
		last = livePrice
	}
	if high <= 0 || last <= 0 {
		return false
	}
	dist := (high - last) / high * 100
	if dist < 0 {
		dist = 0 // above the closed-bar high: at the high
	}
	return dist <= maxDistPct
}
