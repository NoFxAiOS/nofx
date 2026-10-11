package breakout

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"nofx/market/bnweight"
	"nofx/security"
)

const (
	defaultFAPIBase = "https://fapi.binance.com"
	defaultSPOTBase = "https://api.binance.com"
)

// binanceHTTP is the scanner client. Every scanner request is Low priority in
// the shared Binance weight meter: it is the first traffic held back when the
// IP's per-minute weight runs high, and it fails fast during a 429/418
// cooldown, so scans never starve order placement or live price reads.
var binanceHTTP = newScannerHTTP()

func newScannerHTTP() *http.Client {
	c := security.SafeHTTPClient(15 * time.Second)
	c.Transport = bnweight.Wrap(c.Transport, bnweight.Low)
	return c
}

func fapiBase() string {
	if v := os.Getenv("BINANCE_FAPI_BASE"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultFAPIBase
}

func spotBase() string {
	if v := os.Getenv("BINANCE_SPOT_BASE"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultSPOTBase
}

// Kline is one candle with taker-flow detail.
type Kline struct {
	OpenTime     int64
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       float64 // base volume
	QuoteVolume  float64 // quote turnover
	Trades       int64
	TakerBuyBase float64
	TakerBuyQty  float64 // quote
}

// LongShortRatio fetches the global long/short account ratio samples.
func (b *binanceDS) LongShortRatio(period string, limit int) ([]LongShortPoint, error) {
	if limit > 500 {
		limit = 500
	}
	u := fmt.Sprintf("%s/futures/data/globalLongShortAccountRatio?symbol=%s&period=%s&limit=%d",
		fapiBase(), b.symbol, period, limit)
	var raw []struct {
		LongShortRatio string `json:"longShortRatio"`
		LongAccount    string `json:"longAccount"`
		ShortAccount   string `json:"shortAccount"`
		Timestamp      int64  `json:"timestamp"`
	}
	if err := fetchJSONCached(u, ratioCacheTTL, &raw); err != nil {
		return nil, err
	}
	out := make([]LongShortPoint, 0, len(raw))
	for _, r := range raw {
		ratio, _ := strconv.ParseFloat(r.LongShortRatio, 64)
		lp, _ := strconv.ParseFloat(r.LongAccount, 64)
		sp, _ := strconv.ParseFloat(r.ShortAccount, 64)
		out = append(out, LongShortPoint{Ratio: ratio, LongPct: lp * 100, ShortPct: sp * 100, TS: r.Timestamp})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty long/short ratio for %s", b.symbol)
	}
	return out, nil
}

// DepthSnapshot carries the ±1% book imbalance around mid price.
type DepthSnapshot struct {
	Mid        float64
	BestBid    float64
	BestAsk    float64
	SpreadPct  float64 // (ask-bid)/mid × 100
	BidQty1Pct float64
	AskQty1Pct float64
	Imbalance  float64 // (bid-ask)/(bid+ask), -1..+1
}

// OIPoint is one open-interest history sample.
type OIPoint struct {
	OI    float64
	Value float64 // notional
	TS    int64
}

// FundingPoint is one funding rate sample.
type FundingPoint struct {
	Rate float64
	TS   int64
}

// FundingInfo is the current (forward-looking) funding state for a symbol:
// the rate the NEXT settlement will pay, and that settlement's interval —
// which is NOT always 8h on Binance (volatile listings settle every 1h/4h).
type FundingInfo struct {
	NextRate      float64 // premiumIndex lastFundingRate (settles at next timestamp)
	IntervalHours float64 // actual settlement interval derived from recent history
}

// LongShortPoint is one global long/short account ratio sample.
type LongShortPoint struct {
	Ratio    float64 // long accounts / short accounts (>1 = long-skewed)
	LongPct  float64 // share of long accounts, 0-100
	ShortPct float64 // share of short accounts, 0-100
	TS       int64
}

// DataSource abstracts the market data needed by the engine (mockable in tests).
type DataSource interface {
	Klines(interval string, limit int) ([]Kline, error)                // USDT-M futures
	SpotKlines(interval string, limit int) ([]Kline, error)            // spot
	Depth1Pct() (*DepthSnapshot, error)                                // ±1% book imbalance
	OIHistory(period string, limit int) ([]OIPoint, error)             // 5m open-interest history
	FundingHistory(limit int) ([]FundingPoint, error)                  // funding rate history
	FundingInfo() (*FundingInfo, error)                                // next-settlement rate + real interval
	LongShortRatio(period string, limit int) ([]LongShortPoint, error) // global account L/S ratio
}

// binanceDS implements DataSource against Binance public endpoints.
type binanceDS struct{ symbol string }

// NewBinanceDS creates a Binance public-market data source for symbol.
func NewBinanceDS(symbol string) DataSource { return &binanceDS{symbol: symbol} }

func (b *binanceDS) Klines(interval string, limit int) ([]Kline, error) {
	return fetchKlines(fapiBase()+"/fapi/v1/klines", b.symbol, interval, limit)
}

func (b *binanceDS) SpotKlines(interval string, limit int) ([]Kline, error) {
	return fetchKlines(spotBase()+"/api/v3/klines", b.symbol, interval, limit)
}

func (b *binanceDS) Depth1Pct() (*DepthSnapshot, error) {
	u := fmt.Sprintf("%s/fapi/v1/depth?symbol=%s&limit=100", fapiBase(), b.symbol)
	var raw struct {
		Bids [][]interface{} `json:"bids"`
		Asks [][]interface{} `json:"asks"`
	}
	if err := fetchJSON(u, &raw); err != nil {
		return nil, err
	}
	if len(raw.Bids) == 0 || len(raw.Asks) == 0 {
		return nil, fmt.Errorf("empty depth book")
	}
	bestBid := toF(raw.Bids[0][0])
	bestAsk := toF(raw.Asks[0][0])
	mid := (bestBid + bestAsk) / 2
	if mid <= 0 {
		return nil, fmt.Errorf("invalid mid price")
	}
	band := mid * 0.01
	var bidQty, askQty float64
	for _, lv := range raw.Bids {
		p, q := toF(lv[0]), toF(lv[1])
		if p >= mid-band {
			bidQty += q
		}
	}
	for _, lv := range raw.Asks {
		p, q := toF(lv[0]), toF(lv[1])
		if p <= mid+band {
			askQty += q
		}
	}
	imb := 0.0
	if bidQty+askQty > 0 {
		imb = (bidQty - askQty) / (bidQty + askQty)
	}
	spreadPct := (bestAsk - bestBid) / mid * 100
	return &DepthSnapshot{
		Mid: mid, BestBid: bestBid, BestAsk: bestAsk, SpreadPct: spreadPct,
		BidQty1Pct: bidQty, AskQty1Pct: askQty, Imbalance: imb,
	}, nil
}

func (b *binanceDS) OIHistory(period string, limit int) ([]OIPoint, error) {
	u := fmt.Sprintf("%s/futures/data/openInterestHist?symbol=%s&period=%s&limit=%d",
		fapiBase(), b.symbol, period, limit)
	var raw []struct {
		SumOpenInterest    string `json:"sumOpenInterest"`
		SumOpenInterestVal string `json:"sumOpenInterestValue"`
		Timestamp          int64  `json:"timestamp"`
	}
	if err := fetchJSONCached(u, oiHistCacheTTL, &raw); err != nil {
		return nil, err
	}
	out := make([]OIPoint, 0, len(raw))
	for _, r := range raw {
		oi, _ := strconv.ParseFloat(r.SumOpenInterest, 64)
		val, _ := strconv.ParseFloat(r.SumOpenInterestVal, 64)
		out = append(out, OIPoint{OI: oi, Value: val, TS: r.Timestamp})
	}
	return out, nil
}

// FundingInfo combines the forward rate (premiumIndex) with the settlement
// interval measured from recent settlement timestamps.
func (b *binanceDS) FundingInfo() (*FundingInfo, error) {
	// Forward-looking rate: what the next settlement charges.
	var px struct {
		LastFundingRate string `json:"lastFundingRate"`
	}
	if err := fetchJSONCached(fapiBase()+"/fapi/v1/premiumIndex?symbol="+b.symbol, premiumCacheTTL, &px); err != nil {
		return nil, err
	}
	rate, _ := strconv.ParseFloat(px.LastFundingRate, 64)

	info := &FundingInfo{NextRate: rate, IntervalHours: 8}
	// Measure the real interval from the last few settlements (some listings
	// settle every 1h or 4h — a fixed ×3/day annualization would understate
	// them by 3-8×).
	var raw []struct {
		FundingTime int64 `json:"fundingTime"`
	}
	if err := fetchJSONCached(fapiBase()+"/fapi/v1/fundingRate?symbol="+b.symbol+"&limit=4", fundingCacheTTL, &raw); err == nil && len(raw) >= 2 {
		gaps := []float64{}
		for i := 1; i < len(raw); i++ {
			h := float64(raw[i].FundingTime-raw[i-1].FundingTime) / 3.6e6
			if h > 0 && h <= 24 {
				gaps = append(gaps, h)
			}
		}
		if len(gaps) > 0 {
			sum := 0.0
			for _, g := range gaps {
				sum += g
			}
			info.IntervalHours = sum / float64(len(gaps))
		}
	}
	return info, nil
}

func (b *binanceDS) FundingHistory(limit int) ([]FundingPoint, error) {
	u := fmt.Sprintf("%s/fapi/v1/fundingRate?symbol=%s&limit=%d", fapiBase(), b.symbol, limit)
	var raw []struct {
		Rate      string `json:"fundingRate"`
		FundingTS int64  `json:"fundingTime"`
	}
	if err := fetchJSONCached(u, fundingCacheTTL, &raw); err != nil {
		return nil, err
	}
	out := make([]FundingPoint, 0, len(raw))
	for _, r := range raw {
		v, _ := strconv.ParseFloat(r.Rate, 64)
		out = append(out, FundingPoint{Rate: v, TS: r.FundingTS})
	}
	return out, nil
}

// fetchKlines downloads and parses a Binance kline payload (futures or spot share the format).
func fetchKlines(baseURL, symbol, interval string, limit int) ([]Kline, error) {
	u := fmt.Sprintf("%s?symbol=%s&interval=%s&limit=%d", baseURL, symbol, interval, limit)
	// Closed-bar series only change when the next bar closes: serve repeat
	// requests (other scanners, the next 5-minute pass) from the cache until
	// then — see cache.go.
	if bars, ok := klineCacheGet(u); ok {
		return bars, nil
	}
	var raw [][]interface{}
	if err := fetchJSON(u, &raw); err != nil {
		return nil, err
	}
	out := make([]Kline, 0, len(raw))
	var formingClose int64 // close time of the forming candle = when this result goes stale
	for _, r := range raw {
		if len(r) < 11 {
			continue
		}
		// Binance includes the forming candle. Confirmation and replay both
		// consume only candles whose exchange close time has passed.
		if closeTime := int64(toF(r[6])); closeTime >= time.Now().UnixMilli() {
			if formingClose == 0 || closeTime < formingClose {
				formingClose = closeTime
			}
			continue
		}
		out = append(out, Kline{
			OpenTime:     int64(toF(r[0])),
			Open:         toF(r[1]),
			High:         toF(r[2]),
			Low:          toF(r[3]),
			Close:        toF(r[4]),
			Volume:       toF(r[5]),
			QuoteVolume:  toF(r[7]),
			Trades:       int64(toF(r[8])),
			TakerBuyBase: toF(r[9]),
			TakerBuyQty:  toF(r[10]),
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no klines returned for %s %s", symbol, interval)
	}
	if formingClose > 0 {
		klineCachePut(u, out, time.UnixMilli(formingClose).Add(klineCacheSlack))
	}
	return out, nil
}

// fetchBody GETs rawURL and returns the body of a 200 response.
func fetchBody(rawURL string) ([]byte, error) {
	resp, err := binanceHTTP.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func fetchJSON(rawURL string, out interface{}) error {
	body, err := fetchBody(rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode failed: %w", err)
	}
	return nil
}

func toF(v interface{}) float64 {
	switch x := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case float64:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}

// isASCIIAlnum reports whether s consists only of ASCII letters and digits.
func isASCIIAlnum(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// TopVolumeSymbols returns the top N USDT-M perps by 24h quote volume
// (filters out non-USDT and delivery contracts, and dead pairs).
func TopVolumeSymbols(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	tickers, err := fetchBoardTickers()
	if err != nil {
		return nil, err
	}
	return topVolumeTickers(tickers, limit), nil
}

// boardTicker is shared by the boards and the shadow scan (one snapshot).
type boardTicker struct {
	Symbol             string `json:"symbol"`
	QuoteVolume        string `json:"quoteVolume"`
	PriceChangePercent string `json:"priceChangePercent"`
	Count              int    `json:"count"`
}

func fetchBoardTickers() ([]boardTicker, error) {
	var raw []boardTicker
	if err := fetchJSONCached(fapiBase()+"/fapi/v1/ticker/24hr", tickerCacheTTL, &raw); err != nil {
		return nil, err
	}
	var out []boardTicker
	for _, t := range raw {
		if !strings.HasSuffix(t.Symbol, "USDT") || strings.Contains(t.Symbol, "_") || !isASCIIAlnum(strings.TrimSuffix(t.Symbol, "USDT")) {
			continue
		}
		if toF(t.QuoteVolume) < 10_000_000 {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func topVolumeTickers(tickers []boardTicker, limit int) []string {
	rows := append([]boardTicker(nil), tickers...)
	sort.Slice(rows, func(i, j int) bool { return toF(rows[i].QuoteVolume) > toF(rows[j].QuoteVolume) })
	out := make([]string, 0, limit)
	for _, r := range rows {
		if len(out) >= limit {
			break
		}
		out = append(out, r.Symbol)
	}
	return out
}

// BoardLists derives the three Binance-app leaderboards from one 24hr ticker
// fetch (user directive 2026-09-12, piggy-dash second universe): 热门榜 =
// top trade-count (popularity proxy — no public hot-list API), 涨幅榜 =
// top 24h gain, 跌幅榜 = top 24h loss. Same USDT-perp/ASCII/liquidity
// filters as the volume universe; each board returns at most its N.
func BoardLists(hotN, gainN, loseN int) (hot, gain, lose []string, err error) {
	if hotN <= 0 {
		hotN = 20
	}
	if gainN <= 0 {
		gainN = 20
	}
	if loseN <= 0 {
		loseN = 20
	}
	tickers, err := fetchBoardTickers()
	if err != nil {
		return nil, nil, nil, err
	}
	hot, gain, lose = boardListsFromTickers(tickers, hotN, gainN, loseN)
	return hot, gain, lose, nil
}

func boardListsFromTickers(tickers []boardTicker, hotN, gainN, loseN int) (hot, gain, lose []string) {
	rows := append([]boardTicker(nil), tickers...)
	collect := func(n int) []string {
		var out []string
		for _, r := range rows {
			if len(out) >= n {
				break
			}
			out = append(out, r.Symbol)
		}
		return out
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
	hot = collect(hotN)
	rows = append([]boardTicker(nil), tickers...)
	sort.Slice(rows, func(i, j int) bool { return toF(rows[i].PriceChangePercent) > toF(rows[j].PriceChangePercent) })
	gain = collect(gainN)
	rows = append([]boardTicker(nil), tickers...)
	sort.Slice(rows, func(i, j int) bool { return toF(rows[i].PriceChangePercent) < toF(rows[j].PriceChangePercent) })
	lose = collect(loseN)
	return
}
