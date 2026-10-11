package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"nofx/market/bnweight"
)

// Binance-derivatives fallback for the net-flow card.
//
// vergex's /api/v1/data-intelligence/flow/markets is unreachable from both
// sides: the backend gets Cloudflare-challenged (datacenter egress) and real
// browsers get a bare 403 on the cross-origin call (their API only serves
// same-origin pages), so the browser relay can never feed this endpoint.
// When the proxy chain fails we approximate net flow with the change in
// open-interest value per Binance USDT-M futures symbol over the selected
// window — OI value growth ≈ net inflow for perpetuals.

const (
	flowFallbackTTL      = 60 * time.Second // result cache per window|limit
	flowFallbackUniverse = 60               // most-active symbols scanned
	flowFallbackWorkers  = 8                // parallel upstream fetches
	flowTickerTTL        = 10 * time.Minute // 24h ticker universe cache
	oiHistTTL            = 4 * time.Minute  // 5m OI samples update every 5m
	flowUpstreamTimeout  = 15 * time.Second // whole fallback computation
)

type binanceCacheEntry struct {
	data    []byte
	expires time.Time
}

var (
	binanceCacheMu sync.Mutex
	binanceCache   = map[string]binanceCacheEntry{}
)

// binanceGetCached GETs a Binance fapi endpoint and caches the raw body for ttl.
func binanceGetCached(ctx context.Context, pathWithQuery string, ttl time.Duration) ([]byte, error) {
	now := time.Now()
	binanceCacheMu.Lock()
	if e, ok := binanceCache[pathWithQuery]; ok && now.Before(e.expires) {
		binanceCacheMu.Unlock()
		return e.data, nil
	}
	binanceCacheMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://fapi.binance.com"+pathWithQuery, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: bnweight.Wrap(nil, bnweight.Normal)}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("binance request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance returned status %d for %s", resp.StatusCode, pathWithQuery)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("binance read body failed: %w", err)
	}

	binanceCacheMu.Lock()
	binanceCache[pathWithQuery] = binanceCacheEntry{data: body, expires: time.Now().Add(ttl)}
	if len(binanceCache) > 512 {
		for k, e := range binanceCache {
			if time.Now().After(e.expires) {
				delete(binanceCache, k)
			}
		}
	}
	binanceCacheMu.Unlock()
	return body, nil
}

type binanceTicker struct {
	Symbol      string `json:"symbol"`
	LastPrice   string `json:"lastPrice"`
	QuoteVolume string `json:"quoteVolume"`
}

// binanceTopVolumeSymbols returns the n most-active USDT-M futures symbols.
func binanceTopVolumeSymbols(ctx context.Context, n int) ([]binanceTicker, error) {
	body, err := binanceGetCached(ctx, "/fapi/v1/ticker/24hr", flowTickerTTL)
	if err != nil {
		return nil, err
	}
	var tickers []binanceTicker
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, fmt.Errorf("binance ticker parse failed: %w", err)
	}
	var usdt []binanceTicker
	for _, t := range tickers {
		if len(t.Symbol) > 4 && t.Symbol[len(t.Symbol)-4:] == "USDT" {
			usdt = append(usdt, t)
		}
	}
	sort.Slice(usdt, func(i, j int) bool {
		vi, _ := strconv.ParseFloat(usdt[i].QuoteVolume, 64)
		vj, _ := strconv.ParseFloat(usdt[j].QuoteVolume, 64)
		return vi > vj
	})
	if len(usdt) > n {
		usdt = usdt[:n]
	}
	return usdt, nil
}

// flowBarsForWindow converts a window label to the number of 5m OI samples
// needed to span it (inclusive of both endpoints).
func flowBarsForWindow(window string) int {
	minutes := map[string]int{
		"5m": 5, "15m": 15, "1h": 60, "4h": 240, "8h": 480, "12h": 720, "24h": 1440,
	}[window]
	bars := minutes/5 + 1
	if bars < 2 {
		bars = 2
	}
	if bars > 500 {
		bars = 500
	}
	return bars
}

type oiHistSample struct {
	SumOpenInterestValue string `json:"sumOpenInterestValue"`
}

// oiValueDelta returns the change in open-interest value (USD) for symbol over
// the last bars 5-minute samples.
func oiValueDelta(ctx context.Context, symbol string, bars int) (float64, error) {
	path := fmt.Sprintf("/futures/data/openInterestHist?symbol=%s&period=5m&limit=%d", symbol, bars)
	body, err := binanceGetCached(ctx, path, oiHistTTL)
	if err != nil {
		return 0, err
	}
	var samples []oiHistSample
	if err := json.Unmarshal(body, &samples); err != nil {
		return 0, fmt.Errorf("oi hist parse failed for %s: %w", symbol, err)
	}
	if len(samples) < 2 {
		return 0, fmt.Errorf("oi hist too short for %s (%d samples)", symbol, len(samples))
	}
	first, err1 := strconv.ParseFloat(samples[0].SumOpenInterestValue, 64)
	last, err2 := strconv.ParseFloat(samples[len(samples)-1].SumOpenInterestValue, 64)
	if err1 != nil || err2 != nil {
		return 0, fmt.Errorf("oi hist value parse failed for %s", symbol)
	}
	return last - first, nil
}

// symbolPriceChange returns the live price and the % change over one `window`
// bar (live forming close vs previous close) from Binance klines.
func symbolPriceChange(ctx context.Context, symbol, window string) (lastPrice, changePct float64, err error) {
	path := fmt.Sprintf("/fapi/v1/klines?symbol=%s&interval=%s&limit=2", symbol, window)
	body, err := binanceGetCached(ctx, path, 30*time.Second)
	if err != nil {
		return 0, 0, err
	}
	var rows [][]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return 0, 0, fmt.Errorf("klines parse failed for %s: %w", symbol, err)
	}
	if len(rows) < 2 {
		return 0, 0, fmt.Errorf("klines too short for %s", symbol)
	}
	prevClose, _ := strconv.ParseFloat(fmt.Sprint(rows[0][4]), 64)
	lastClose, _ := strconv.ParseFloat(fmt.Sprint(rows[1][4]), 64)
	if prevClose <= 0 {
		return 0, 0, fmt.Errorf("klines bad close for %s", symbol)
	}
	return lastClose, (lastClose - prevClose) / prevClose * 100, nil
}

type flowRow struct {
	Key            string `json:"key"`
	MarketType     string `json:"marketType"`
	Symbol         string `json:"symbol"`
	NetFlow        string `json:"netFlow"`
	LatestPrice    string `json:"latestPrice"`
	PriceChangePct string `json:"priceChangePct"`
	Trades         int    `json:"trades"`
}

// flowFallbackJSON builds a flow/markets-shaped payload from Binance OI
// deltas: top `limit` symbols by OI value growth as inflow, by decline as
// outflow. The result is cached for flowFallbackTTL.
func flowFallbackJSON(window string, limit int) ([]byte, error) {
	cacheKey := "flowfallback|" + window + "|" + strconv.Itoa(limit)
	now := time.Now()
	binanceCacheMu.Lock()
	if e, ok := binanceCache[cacheKey]; ok && now.Before(e.expires) {
		binanceCacheMu.Unlock()
		return e.data, nil
	}
	binanceCacheMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), flowUpstreamTimeout)
	defer cancel()

	tickers, err := binanceTopVolumeSymbols(ctx, flowFallbackUniverse)
	if err != nil {
		return nil, err
	}
	bars := flowBarsForWindow(window)

	type flowResult struct {
		symbol              string
		delta, last, chgPct float64
	}
	results := make([]flowResult, 0, len(tickers))
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, flowFallbackWorkers)
	)
	for _, tk := range tickers {
		wg.Add(1)
		go func(symbol, lastPriceStr string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			delta, err := oiValueDelta(ctx, symbol, bars)
			if err == nil {
				var last, chg float64
				last, chg, err = symbolPriceChange(ctx, symbol, window)
				if err == nil {
					if last == 0 {
						last, _ = strconv.ParseFloat(lastPriceStr, 64)
					}
					mu.Lock()
					results = append(results, flowResult{symbol: symbol, delta: delta, last: last, chgPct: chg})
					mu.Unlock()
				}
			}
		}(tk.Symbol, tk.LastPrice)
	}
	wg.Wait()
	if len(results) == 0 {
		return nil, fmt.Errorf("no OI data available for any symbol")
	}
	sort.Slice(results, func(i, j int) bool { return results[i].delta > results[j].delta })

	buildRow := func(r flowResult) flowRow {
		return flowRow{
			Key:            "binance_perp:" + r.symbol,
			MarketType:     "binance_perp",
			Symbol:         r.symbol,
			NetFlow:        strconv.FormatFloat(r.delta, 'f', 2, 64),
			LatestPrice:    strconv.FormatFloat(r.last, 'f', -1, 64),
			PriceChangePct: strconv.FormatFloat(r.chgPct, 'f', 4, 64),
		}
	}
	var inflow, outflow []flowRow
	for _, r := range results {
		if len(inflow) < limit && r.delta > 0 {
			inflow = append(inflow, buildRow(r))
		}
	}
	for i := len(results) - 1; i >= 0 && len(outflow) < limit; i-- {
		r := results[i]
		if r.delta >= 0 {
			break
		}
		outflow = append(outflow, buildRow(r))
	}

	payload, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"by":      "markets",
			"window":  window,
			"source":  "binance_oi_delta",
			"inflow":  inflow,
			"outflow": outflow,
		},
	})
	if err != nil {
		return nil, err
	}

	binanceCacheMu.Lock()
	binanceCache[cacheKey] = binanceCacheEntry{data: payload, expires: time.Now().Add(flowFallbackTTL)}
	binanceCacheMu.Unlock()
	return payload, nil
}
