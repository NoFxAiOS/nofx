package market

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/logger"
	"nofx/market/bnweight"
	"nofx/security"
)

// ============================================================================
// Market sentiment composite (user directive 2026-09-27): THREE independent
// sentiment sources rendered together at the head of the user prompt, with an
// explicit combine-don't-overindex instruction:
//   1. Alternative.me  — crypto macro fear & greed (daily)
//   2. feargreedchart  — US-stock macro fear & greed proxy (its OWN five-
//                        component model over Yahoo Finance data — NOT CNN's
//                        official index; weights happen to match 25/25/20/15/15)
//   3. Binance         — contract crowding: global long/short account ratio
//                        + premiumIndex last funding (BTC/ETH)
// All sources are KEYLESS and fail-open: a failed source renders as absent,
// never as a fabricated value. Caches match each source's natural cadence.
//
// Time semantics (user review 2026-09-27): the composite is cached up to 30
// minutes, so "as of now" stamped at render time lied about freshness. Every
// source therefore carries the timestamp ITS DATA describes (SourceAt /
// MarketDate / RatioPeriodEnd / FundingAt) and the composite carries the wall
// clock it was FETCHED at — render reads only stored fields, never Now().
// ============================================================================

// CryptoFG is the Alternative.me fear & greed reading.
type CryptoFG struct {
	Value          int    `json:"value"`
	Classification string `json:"classification"`
	YesterdayValue int    `json:"yesterday_value"`
	// SourceAt is the UTC date the reading describes (the API's own
	// timestamp — a daily index, so the value ages up to 24h by design).
	SourceAt string `json:"source_at,omitempty"`
}

// StockFG is the US-stock fear & greed proxy (FearGreedChart's independent
// model — not CNN's official index).
type StockFG struct {
	Score      int                `json:"score"`
	Components []StockFGComponent `json:"components"`
	// MarketDate is the trading day the score describes (latest close —
	// weekends/holders keep showing Friday until Monday's close lands).
	MarketDate string `json:"market_date,omitempty"`
	FetchedAt  string `json:"fetched_at,omitempty"` // API series generation time (UTC)
}

type StockFGComponent struct {
	Name string `json:"name"`
	Val  int    `json:"val"`
	Wt   int    `json:"wt"`
}

// BinanceCrowd is the contract-positioning snapshot for the majors.
type BinanceCrowd struct {
	BTCLS       float64 `json:"btc_ls_ratio"` // global accounts long/short
	ETHLS       float64 `json:"eth_ls_ratio"`
	BTCAFunding float64 `json:"btc_funding"` // raw per-interval rate (decimal)
	ETHFunding  float64 `json:"eth_funding"`
	// RatioPeriodEnd is the close of the 1h stats window the L/S ratios come
	// from (Binance returns one row; its timestamp IS the data's age).
	RatioPeriodEnd string `json:"ratio_period_end,omitempty"`
	FundingAt      string `json:"funding_at,omitempty"` // premiumIndex quote time (UTC)
}

// MarketSentiment is the composite block; nil fields = source unavailable.
type MarketSentiment struct {
	Crypto  *CryptoFG     `json:"crypto_fg,omitempty"`
	Stock   *StockFG      `json:"stock_fg,omitempty"`
	Binance *BinanceCrowd `json:"binance_crowding,omitempty"`
	// FetchedAt is the wall clock the composite was fetched at (UTC) — the
	// honest "how old is this cache" stamp (TTL 30 min).
	FetchedAt string `json:"fetched_at,omitempty"`
	// Regime / TradeEffect are the PROGRAM's combination verdict over the
	// three sources (see classifySentimentRegime) — the model cites the enum
	// instead of re-deriving the three combine rules every cycle.
	Regime      string   `json:"sentiment_regime,omitempty"`
	TradeEffect string   `json:"sentiment_trade_effect,omitempty"`
	Notes       []string `json:"notes,omitempty"` // per-source fetch failures
}

var (
	sentimentMu       sync.Mutex
	sentimentCache    *MarketSentiment
	sentimentCachedAt time.Time
	// sentimentCacheTTLCur is the TTL that applies to the CURRENT cache entry:
	// the full TTL when every source succeeded (or was reused), a short one
	// when any source is missing so the next call retries soon.
	sentimentCacheTTLCur = sentimentCacheTTL
	// sentimentInflight is non-nil while a refresh runs (single-flight).
	sentimentInflight chan struct{}

	// Last successful value per source + when it was fetched, for the
	// per-source fallback (review 2026-10-07 B2-4).
	lastCrypto    *CryptoFG
	lastCryptoAt  time.Time
	lastStock     *StockFG
	lastStockAt   time.Time
	lastBinance   *BinanceCrowd
	lastBinanceAt time.Time
)

const sentimentCacheTTL = 30 * time.Minute

// review 2026-10-07 B2-4: a refresh with a missing source used to be cached
// for the full 30 min (one alternative.me timeout silently disabled the
// greed down-weight that long). Now a failed source reuses its last good
// value up to sentimentReuseMaxAge, and a composite that still has a hole is
// cached only sentimentFailTTL.
const (
	sentimentReuseMaxAge = 2 * time.Hour
	sentimentFailTTL     = 2 * time.Minute
)

// Injectable for tests; production defaults unchanged.
var (
	sentimentFetchCrypto  = fetchCryptoFG
	sentimentFetchStock   = fetchStockFG
	sentimentFetchBinance = fetchBinanceCrowd
	sentimentNow          = time.Now
)

func sentimentHTTPGet(url string, out interface{}) error {
	client := security.SafeHTTPClient(15 * time.Second)
	// Non-Binance vendors pass through untouched; the two fapi reads are metered.
	client.Transport = bnweight.Wrap(client.Transport, bnweight.Normal)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func fetchCryptoFG() *CryptoFG {
	var payload struct {
		Data []struct {
			Value               string `json:"value"`
			ValueClassification string `json:"value_classification"`
			Timestamp           string `json:"timestamp"`
		} `json:"data"`
	}
	// Proxy rides the environment (SafeHTTPClient), same as every vendor.
	if err := sentimentHTTPGet("https://api.alternative.me/fng/?limit=2", &payload); err != nil || len(payload.Data) < 2 {
		return nil
	}
	today, err1 := strconv.Atoi(payload.Data[0].Value)
	yest, err2 := strconv.Atoi(payload.Data[1].Value)
	if err1 != nil || err2 != nil {
		return nil
	}
	out := &CryptoFG{Value: today, Classification: payload.Data[0].ValueClassification, YesterdayValue: yest}
	// The reading's own date (unix seconds, daily 00:00 UTC) — the honest
	// as-of for a value that is up to 24h old by design.
	if sec, err := strconv.ParseInt(payload.Data[0].Timestamp, 10, 64); err == nil && sec > 0 {
		out.SourceAt = time.Unix(sec, 0).UTC().Format("2006-01-02")
	}
	return out
}

func fetchStockFG() *StockFG {
	var payload struct {
		Score struct {
			Score      int `json:"score"`
			Components []struct {
				Name string `json:"name"`
				Val  int    `json:"val"`
				Wt   int    `json:"wt"`
			} `json:"components"`
		} `json:"score"`
		Recent []struct {
			Date  string `json:"date"`
			Score int    `json:"score"`
		} `json:"recent"`
		Ts int64 `json:"ts"`
	}
	if err := sentimentHTTPGet("https://feargreedchart.com/api", &payload); err != nil {
		return nil
	}
	out := &StockFG{Score: payload.Score.Score}
	for _, c := range payload.Score.Components {
		out.Components = append(out.Components, StockFGComponent{Name: c.Name, Val: c.Val, Wt: c.Wt})
	}
	if out.Score <= 0 {
		return nil
	}
	// The trading day the CURRENT score describes = the latest dated entry
	// (score matches; on weekends that is Friday's close — disclose it).
	if n := len(payload.Recent); n > 0 && payload.Recent[n-1].Date != "" {
		out.MarketDate = payload.Recent[n-1].Date
	}
	if payload.Ts > 0 {
		out.FetchedAt = time.UnixMilli(payload.Ts).UTC().Format("2006-01-02 15:04 UTC")
	}
	return out
}

func fetchBinanceCrowd() *BinanceCrowd {
	out := &BinanceCrowd{}
	ls := func(symbol string) float64 {
		var rows []struct {
			LongShortRatio string `json:"longShortRatio"`
			Timestamp      int64  `json:"timestamp"`
		}
		if err := sentimentHTTPGet("https://fapi.binance.com/futures/data/globalLongShortAccountRatio?symbol="+symbol+"&period=1h&limit=1", &rows); err != nil || len(rows) == 0 {
			return 0
		}
		v, _ := strconv.ParseFloat(rows[0].LongShortRatio, 64)
		if rows[0].Timestamp > 0 {
			out.RatioPeriodEnd = time.UnixMilli(rows[0].Timestamp).UTC().Format("2006-01-02 15:04 UTC")
		}
		return v
	}
	premium := func(symbol string) float64 {
		var p struct {
			LastFundingRate string `json:"lastFundingRate"`
			Time            int64  `json:"time"`
		}
		if err := sentimentHTTPGet("https://fapi.binance.com/fapi/v1/premiumIndex?symbol="+symbol, &p); err != nil {
			return 0
		}
		v, _ := strconv.ParseFloat(p.LastFundingRate, 64)
		if p.Time > 0 {
			out.FundingAt = time.UnixMilli(p.Time).UTC().Format("2006-01-02 15:04 UTC")
		}
		return v
	}
	out.BTCLS = ls("BTCUSDT")
	out.ETHLS = ls("ETHUSDT")
	out.BTCAFunding = premium("BTCUSDT")
	out.ETHFunding = premium("ETHUSDT")
	if out.BTCLS <= 0 && out.ETHLS <= 0 && out.BTCAFunding == 0 && out.ETHFunding == 0 {
		return nil
	}
	return out
}

// GetMarketSentiment returns the composite with per-source cache TTLs. Each
// source refreshes independently so one flaky vendor doesn't age the others.
func GetMarketSentiment() *MarketSentiment {
	for {
		sentimentMu.Lock()
		if sentimentCache != nil && sentimentNow().Sub(sentimentCachedAt) < sentimentCacheTTLCur {
			c := sentimentCache
			sentimentMu.Unlock()
			return c
		}
		if ch := sentimentInflight; ch != nil {
			// Single-flight: someone else is fetching. Serve the (expired)
			// cache meanwhile if there is one, else wait for the result.
			stale := sentimentCache
			sentimentMu.Unlock()
			if stale != nil {
				return stale
			}
			<-ch
			continue
		}
		ch := make(chan struct{})
		sentimentInflight = ch
		sentimentMu.Unlock()

		return publishSentimentRefresh(ch)
	}
}

// publishSentimentRefresh runs the refresh as the single-flight winner and
// ALWAYS releases the in-flight slot: a panic inside the refresh used to
// leave sentimentInflight set and ch open forever, so every waiter blocked
// and no later call could ever refresh again (review 2026-10-07 B2-4).
func publishSentimentRefresh(ch chan struct{}) (out *MarketSentiment) {
	defer func() {
		r := recover()
		sentimentMu.Lock()
		if r == nil && out != nil {
			sentimentCache = out
			sentimentCachedAt = sentimentNow()
			if out.Crypto == nil || out.Stock == nil || out.Binance == nil {
				sentimentCacheTTLCur = sentimentFailTTL
			} else {
				sentimentCacheTTLCur = sentimentCacheTTL
			}
		}
		sentimentInflight = nil
		sentimentMu.Unlock()
		close(ch)
		if r != nil {
			logger.Errorf("⚠️ sentiment refresh panicked (recovered): %v", r)
			out = &MarketSentiment{Notes: []string{"sentiment_refresh_panicked"}}
		}
	}()
	return refreshSentiment() // network I/O without holding sentimentMu
}

// refreshSentiment fetches all sources concurrently and applies the
// per-source stale-reuse fallback. Only the refresh winner calls it.
// sentimentPanicHook is a test-only seam (nil in production).
var sentimentPanicHook func()

func refreshSentiment() *MarketSentiment {
	if sentimentPanicHook != nil {
		sentimentPanicHook()
	}
	out := &MarketSentiment{}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); out.Crypto = sentimentFetchCrypto() }()
	go func() { defer wg.Done(); out.Stock = sentimentFetchStock() }()
	go func() { defer wg.Done(); out.Binance = sentimentFetchBinance() }()
	wg.Wait()

	now := sentimentNow()
	sentimentMu.Lock()
	// reuse reports whether the last good value is young enough. The
	// composite FetchedAt stays the wall clock of THIS refresh; the note says
	// which source is a reused older value (its own SourceAt/MarketDate/...
	// fields still describe the data's real age).
	reuse := func(at time.Time, have bool) bool {
		return have && now.Sub(at) <= sentimentReuseMaxAge
	}
	if out.Crypto != nil {
		lastCrypto, lastCryptoAt = out.Crypto, now
	} else if reuse(lastCryptoAt, lastCrypto != nil) {
		out.Crypto = lastCrypto
		out.Notes = append(out.Notes, "crypto_fg_stale_reused")
	}
	if out.Stock != nil {
		lastStock, lastStockAt = out.Stock, now
	} else if reuse(lastStockAt, lastStock != nil) {
		out.Stock = lastStock
		out.Notes = append(out.Notes, "stock_fg_stale_reused")
	}
	if out.Binance != nil {
		lastBinance, lastBinanceAt = out.Binance, now
	} else if reuse(lastBinanceAt, lastBinance != nil) {
		out.Binance = lastBinance
		out.Notes = append(out.Notes, "binance_crowding_stale_reused")
	}
	sentimentMu.Unlock()

	if out.Crypto == nil {
		out.Notes = append(out.Notes, "crypto_fg_fetch_failed")
	}
	if out.Stock == nil {
		out.Notes = append(out.Notes, "stock_fg_fetch_failed")
	}
	if out.Binance == nil {
		out.Notes = append(out.Notes, "binance_crowding_fetch_failed")
	}
	// Wall clock of the FETCH — the only "now" this package is allowed to
	// stamp (the render path reads stored fields so a 29-minute-old cache
	// can never pose as fresh, user review 2026-09-27).
	out.FetchedAt = now.UTC().Format("2006-01-02 15:04 UTC")
	out.Regime, out.TradeEffect = classifySentimentRegime(out)
	return out
}

// classifySentimentRegime is the program's verdict over the three combine
// rules the prompt states: side (extreme needs BOTH macro sources), leverage
// crowding, and the resulting trade effect (crowded extremes → conservative
// opens; diverging sources → structure wins; otherwise context-only).
// Crowding requires SAME-SIDE CONFLUENCE (user review 09-29: a single L/S
// >2 reading with cheap funding is NOT "leverage crowded" — labeling it so
// pushed the model toward over-conservatism):
//
//	long crowd  = funding annualized ≥ +50% (either major) AND L/S > 2
//	short crowd = funding annualized ≤ −50% (either major) AND L/S < 0.5
//
// Zero values = missing fetches and never trigger (L/S > 0 guarded).
// Pure function of the stored values — never of the wall clock.
func classifySentimentRegime(m *MarketSentiment) (regime, effect string) {
	crypto, stock := 50, 50
	haveCrypto, haveStock := false, false
	if m.Crypto != nil {
		crypto, haveCrypto = m.Crypto.Value, true
	}
	if m.Stock != nil {
		stock, haveStock = m.Stock.Score, true
	}
	side := "NEUTRAL"
	bothGreed := haveCrypto && haveStock && crypto >= 75 && stock >= 75
	bothFear := haveCrypto && haveStock && crypto <= 25 && stock <= 25
	split := haveCrypto && haveStock && ((crypto >= 75 && stock <= 25) || (crypto <= 25 && stock >= 75))
	switch {
	case split:
		side = "DIVERGENT"
	case bothGreed:
		side = "EXTREME_GREED"
	case bothFear:
		side = "EXTREME_FEAR"
	case crypto >= 55 || stock >= 55:
		side = "GREED"
	case crypto <= 45 || stock <= 45:
		side = "FEAR"
	}

	crowding := ""
	if m.Binance != nil {
		maxAnn := 0.0
		minAnn := 0.0
		for _, f := range []float64{m.Binance.BTCAFunding, m.Binance.ETHFunding} {
			ann := fundingAnnualized(f)
			if ann > maxAnn {
				maxAnn = ann
			}
			if ann < minAnn {
				minAnn = ann
			}
		}
		maxLS := math.Max(m.Binance.BTCLS, m.Binance.ETHLS)
		longCrowd := maxAnn >= 50 && maxLS > 2
		shortCrowd := minAnn <= -50 && maxLS > 0 && math.Min(lsOr(m.Binance.BTCLS), lsOr(m.Binance.ETHLS)) < 0.5
		if longCrowd || shortCrowd {
			crowding = "_LEVERAGE_CROWDED"
		} else {
			crowding = "_NOT_LEVERAGE_CROWDED"
		}
	} else {
		crowding = "_CROWDING_UNKNOWN"
	}
	if side == "NEUTRAL" {
		return "NEUTRAL", "CONTEXT_ONLY"
	}
	regime = side + crowding
	switch {
	case side == "EXTREME_GREED" && crowding == "_LEVERAGE_CROWDED":
		effect = "CONSERVATIVE_OPENS" // combine rule ①: crowding zone
	case side == "DIVERGENT":
		effect = "STRUCTURE_WINS" // combine rule ②
	default:
		effect = "CONTEXT_ONLY" // combine rule ③
	}
	return regime, effect
}

// fundingAnnualized converts a raw per-8h-settlement funding decimal to an
// annualized percent for the prompt line (3 settlements/day default).
func fundingAnnualized(raw float64) float64 {
	return raw * 3 * 365 * 100
}

// Render renders the composite into prompt text — the caller embeds it at
// the head of the user prompt. Empty when every source failed (the block
// disappears rather than showing an empty shell).
func (m *MarketSentiment) Render() string {
	if m == nil {
		return ""
	}
	var sb_ []string
	if m.Crypto != nil {
		dir := "持平"
		if m.Crypto.Value > m.Crypto.YesterdayValue {
			dir = "较昨日上升"
		} else if m.Crypto.Value < m.Crypto.YesterdayValue {
			dir = "较昨日回落"
		}
		line := fmt.Sprintf("加密宏观情绪(Alternative.me): %d %s(%s,昨日 %d)", m.Crypto.Value, m.Crypto.Classification, dir, m.Crypto.YesterdayValue)
		if m.Crypto.SourceAt != "" {
			line += "[数据日 " + m.Crypto.SourceAt + "]"
		}
		sb_ = append(sb_, line)
	}
	if m.Stock != nil {
		comps := make([]string, 0, len(m.Stock.Components))
		for _, c := range m.Stock.Components {
			comps = append(comps, fmt.Sprintf("%s %d(权重%d%%)", c.Name, c.Val, c.Wt))
		}
		sort.Strings(comps)
		// FearGreedChart's own methodology is an independent five-component
		// model over Yahoo Finance data — it is NOT CNN's official index and
		// must not be presented as one (user review 2026-09-27 #2).
		line := fmt.Sprintf("美股情绪代理(FearGreedChart 独立模型,非CNN官方): %d;分量: %s", m.Stock.Score, strings.Join(comps, ", "))
		if m.Stock.MarketDate != "" {
			line += "[数据日 " + m.Stock.MarketDate + " 收盘]"
		}
		sb_ = append(sb_, line)
	}
	if m.Binance != nil {
		parts := ""
		if m.Binance.BTCLS > 0 {
			parts += fmt.Sprintf("BTC 多空账户比 %.2f", m.Binance.BTCLS)
		}
		if m.Binance.ETHLS > 0 {
			if parts != "" {
				parts += "、"
			}
			parts += fmt.Sprintf("ETH %.2f", m.Binance.ETHLS)
		}
		if m.Binance.BTCAFunding != 0 {
			parts += fmt.Sprintf(";资金费 BTC %+.4f%%/期(年化约 %+.1f%%)", m.Binance.BTCAFunding*100, fundingAnnualized(m.Binance.BTCAFunding))
		}
		if m.Binance.ETHFunding != 0 {
			parts += fmt.Sprintf(",ETH %+.4f%%/期(年化约 %+.1f%%)", m.Binance.ETHFunding*100, fundingAnnualized(m.Binance.ETHFunding))
		}
		if parts != "" {
			sb_ = append(sb_, "合约拥挤(Binance): "+parts)
		}
	}
	if len(sb_) == 0 {
		return ""
	}
	body := strings.Join(sb_, "\n")
	var failed []string
	if m.Crypto == nil {
		failed = append(failed, "加密情绪缺数")
	}
	if m.Stock == nil {
		failed = append(failed, "美股情绪缺数")
	}
	if m.Binance == nil {
		failed = append(failed, "合约拥挤缺数")
	}
	if len(failed) > 0 {
		body += "\n(缺数来源: " + strings.Join(failed, "、") + " — 按剩余来源与价格结构判断)"
	}
	// Program's combination verdict as citable enums (user review 2026-09-27):
	// the model references these instead of re-deriving the three combine
	// rules; they never override program hard gates.
	regime, effect := m.Regime, m.TradeEffect
	if regime == "" {
		regime, effect = classifySentimentRegime(m)
	}
	body += "\nsentiment_regime: " + regime + " | sentiment_trade_effect: " + effect + "(程序判定,仅为背景,不覆盖硬门)"
	// Per-source as-of stamps (user review 2026-09-27 #1): the composite is
	// cached ≤30min, so freshness comes from the stored data timestamps —
	// funding drifts per settlement, the daily indexes age by design, and a
	// stale cache must never pose as "just fetched".
	var stamps []string
	if m.Crypto != nil && m.Crypto.SourceAt != "" {
		stamps = append(stamps, "crypto_fng="+m.Crypto.SourceAt)
	}
	if m.Stock != nil && m.Stock.MarketDate != "" {
		stamps = append(stamps, "stock="+m.Stock.MarketDate+"收盘")
	}
	if m.Binance != nil {
		if m.Binance.RatioPeriodEnd != "" {
			stamps = append(stamps, "多空比窗口="+m.Binance.RatioPeriodEnd)
		}
		if m.Binance.FundingAt != "" {
			stamps = append(stamps, "funding="+m.Binance.FundingAt)
		}
	}
	if len(stamps) > 0 {
		body += "\n数据截至(各来源独立): " + strings.Join(stamps, "; ")
	}
	if m.FetchedAt != "" {
		body += " | 快照拉取: " + m.FetchedAt + "(缓存最长30分钟)"
	}
	return body
}

var _ = security.SafeHTTPClient

// lsOr maps a missing (0) L/S reading to +Inf so the short-crowd min()
// ignores missing fetches instead of treating them as extreme short crowding.
func lsOr(v float64) float64 {
	if v <= 0 {
		return math.Inf(1)
	}
	return v
}
