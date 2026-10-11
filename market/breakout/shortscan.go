package breakout

import (
	"fmt"
	"math"
	"nofx/market"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/logger"
)

// ShortScan ranks the top 24h gainers among Binance USDT-M perps by how
// suitable they are for a SHORT entry right now — targeting pumps whose
// "quality" is poor (parabolic moves that outrun BTC, price/volume
// divergence), whose contract market structure is overheated (funding
// annualized sky-high, OI piling in, long/short accounts skewed long), and —
// critically — that already print topping CONFIRMATION signals instead of
// asking the AI to catch a vertical knife: 4h bearish divergence, failed
// breakouts above the prior swing high, fresh EMA20 breaks and funding
// rolling over from highs.
//
// All data comes from Binance public endpoints.

const (
	shortScanCacheTTL    = 2 * time.Minute
	shortScanConcurrency = 6
)

// ShortScanUniverse is how many top 24h gainers each short-scan run covers.
const ShortScanUniverse = 50

// shortScanMinGainerChgPct: review 2026-10-07 S4 — the live "gainer" universe
// is top-50 by rank, so a quiet market scored +1-4% coins as pump-fade shorts.
// Require a real pump. The board is still recorded to the history pool as
// before (the pool applies its own peak ranking and the board prefix is the
// durable asset); only the scan work items are filtered.
const shortScanMinGainerChgPct = 5.0

// ShortSignal is one ranked short candidate.
type ShortSignal struct {
	Symbol           string          `json:"symbol"`
	Price            float64         `json:"price"`
	Change24hPct     float64         `json:"change_24h_pct"`     // 24h gainer ranking input
	Change5dPct      float64         `json:"change_5d_pct"`      // 5-day pump magnitude (4h bars)
	BtcOutperform    float64         `json:"btc_outperform_pct"` // 5d return minus BTC's (pump "quality")
	FundingAnnualPct float64         `json:"funding_annualized_pct"`
	LongShortRatio   *float64        `json:"long_short_ratio,omitempty"`
	OIValueMillions  float64         `json:"oi_value_millions"` // latest OI notional (USD millions); 0 = unavailable
	ListingDays      int             `json:"listing_days"`      // approximate, from available 4h bars
	BearishDiv4h     bool            `json:"bearish_divergence_4h"`
	FakeBreakout     bool            `json:"fake_breakout"`
	MABreak          bool            `json:"ma20_break"`
	FundingRollover  bool            `json:"funding_rollover"`
	Confirmed        bool            `json:"confirmed"` // 4h bearish divergence or fake breakout printed (EMA20 break / funding rollover are NOT confirmations)
	Score            float64         `json:"score"`     // 0-100 composite short suitability
	Percentile       float64         `json:"percentile"`
	Grade            string          `json:"grade"`                    // strong / medium / weak / noise
	BtcRegime        string          `json:"btc_regime,omitempty"`     // btc_bull / btc_bear / chop (macro gate context)
	Universe         string          `json:"universe,omitempty"`       // "gainer" (24h涨幅榜) | "near_high" (距90日高点<5%磨顶池)
	NearHighAlso     bool            `json:"near_high_also,omitempty"` // also passed the grinding-top screen (90d-high + 4h div) — pool cut must apply the near_high OI-floor exemption
	Components       ShortComponents `json:"components"`
	Reasons          []string        `json:"reasons,omitempty"`
	GeneratedAt      time.Time       `json:"generated_at"`
}

// ShortComponents carries the per-dimension scores for transparency.
type ShortComponents struct {
	Stretch    float64 `json:"stretch"`
	Overbought float64 `json:"overbought"`
	Rejection  float64 `json:"rejection"`
	VolumeFade float64 `json:"volume_fade"`
	Extension  float64 `json:"extension"`
	Crowding   float64 `json:"crowding"`
	Parabolic  float64 `json:"parabolic"`
	Divergence float64 `json:"divergence"`
	Structure  float64 `json:"structure"`
}

type shortScanCacheEntry struct {
	results   []ShortSignal
	expires   time.Time
	updatedAt time.Time
}

// A4 (QUANT_REVIEW 09-22): one cache slot per resolved config
// (universe|histDays|histMax) instead of one process-global slot — two
// strategies with different short_scan_history values each keep their own
// 2-minute scan instead of re-scanning every cycle and stomping each other.
var (
	shortScanCache    = map[string]*shortScanCacheEntry{}
	shortScanCacheMu  sync.Mutex
	shortScanInflight = map[string]bool{}
)

// gainerTickerSnapshot fetches the 24h ticker once and returns the gainer
// board (top `limit`, chg-desc) PLUS a live-quote index over every filtered
// symbol — the history pool needs current quotes for coins that left the
// board without paying a second API call.
func gainerTickerSnapshot(limit int) ([]GainerQuote, []GainerQuote, map[string]GainerQuote, error) {
	if limit <= 0 {
		limit = ShortScanUniverse
	}
	u := fapiBase() + "/fapi/v1/ticker/24hr"
	var raw []struct {
		Symbol             string `json:"symbol"`
		LastPrice          string `json:"lastPrice"`
		PriceChangePercent string `json:"priceChangePercent"`
		QuoteVolume        string `json:"quoteVolume"`
	}
	if err := fetchJSONCached(u, tickerCacheTTL, &raw); err != nil {
		return nil, nil, nil, err
	}
	board := make([]GainerQuote, 0, limit)
	index := make(map[string]GainerQuote, 512)
	for _, t := range raw {
		if !strings.HasSuffix(t.Symbol, "USDT") || strings.Contains(t.Symbol, "_") {
			continue
		}
		if !isASCIIAlnum(t.Symbol[:len(t.Symbol)-4]) {
			continue
		}
		v, _ := strconv.ParseFloat(t.QuoteVolume, 64)
		if v < 10_000_000 { // skip illiquid pairs
			continue
		}
		chg, _ := strconv.ParseFloat(t.PriceChangePercent, 64)
		price, _ := strconv.ParseFloat(t.LastPrice, 64)
		q := GainerQuote{Symbol: t.Symbol, ChgPct: chg, Price: price}
		index[t.Symbol] = q
		board = append(board, q)
	}
	sort.Slice(board, func(i, j int) bool { return board[i].ChgPct > board[j].ChgPct })
	if len(board) > limit {
		board = board[:limit]
	}
	// Loser board (A1, QUANT_REVIEW 09-22): same filtered set, 24h-change
	// ascending, top BreakdownUniverse — the breakdown-continuation universe
	// costs nothing extra (same snapshot) and repairs the short pool's
	// survivorship bias (it used to contain ONLY coins that had pumped).
	losers := make([]GainerQuote, 0, len(index))
	for _, q := range index {
		losers = append(losers, q)
	}
	sort.Slice(losers, func(i, j int) bool { return losers[i].ChgPct < losers[j].ChgPct })
	if len(losers) > BreakdownUniverse {
		losers = losers[:BreakdownUniverse]
	}
	return board, losers, index, nil
}

// TopGainerSymbols returns up to limit USDT-M perps with the highest 24h
// price-change percent (liquidity-filtered the same way as TopVolumeSymbols).
func TopGainerSymbols(limit int) ([]GainerQuote, error) {
	board, _, _, err := gainerTickerSnapshot(limit)
	return board, err
}

// pumpWindows5d returns the recent and prior 30-interval (5d of 4h bars)
// returns in percent; ok=false when fewer than 61 bars are available.
func pumpWindows5d(c4h []float64) (recent, prior float64, ok bool) {
	n := len(c4h)
	if n < 61 {
		return 0, 0, false
	}
	return (c4h[n-1]/c4h[n-31] - 1) * 100, (c4h[n-31]/c4h[n-61] - 1) * 100, true
}

// AnalyzeShort computes the short-suitability score for one symbol. btc4h is
// BTC's 4h close series (may be nil) used to judge whether the pump actually
// outperforms the market — pumps that merely track BTC are lower quality.
func AnalyzeShort(symbol string, chg24 float64, btc4h []Kline, ds DataSource) (*ShortSignal, error) {
	k1h, err := ds.Klines("1h", 60)
	if err != nil {
		return nil, fmt.Errorf("1h klines: %w", err)
	}
	k4h, err := ds.Klines("4h", 84) // 14 days
	if err != nil {
		return nil, fmt.Errorf("4h klines: %w", err)
	}
	if len(k1h) < 30 || len(k4h) < 12 {
		return nil, fmt.Errorf("insufficient klines for %s (1h=%d 4h=%d)", symbol, len(k1h), len(k4h))
	}

	c1h := closes(k1h)
	c4h := closes(k4h)
	sig := &ShortSignal{
		Symbol:       symbol,
		Price:        c1h[len(c1h)-1],
		Change24hPct: chg24,
		ListingDays:  len(k4h) * 4 / 24,
		GeneratedAt:  time.Now().UTC(),
	}

	// ── 1. Stretch: 24h move vs its own RECENT-MONTH volatility ──
	// Baseline = 24h-window returns sampled every 4h bar over the last 14
	// days — NOT the last ~36h of 1h bars: a live pump inflates its own
	// denominator and "normalizes" exactly the extremes this dimension must
	// flag (audit 2026-09-12 #3, self-referential vol).
	if len(c4h) >= 31 {
		var rolls []float64
		for i := 6; i < len(c4h); i++ {
			if c4h[i-6] > 0 {
				rolls = append(rolls, (c4h[i]/c4h[i-6]-1)*100)
			}
		}
		if len(rolls) >= 10 {
			m := mean(rolls)
			sd := math.Sqrt(mean2(rolls, m))
			if sd > 0 {
				z := (chg24 - m) / sd
				sig.Components.Stretch = clamp100(sigmoidScore(z, 2.0, 1.0))
			}
		}
	}

	// ── 2. Overbought: RSI exhaustion band on both timeframes ──
	rsi1h := rsiLast(c1h, 14)
	rsi4h := rsiLast(c4h, 14)
	// 09-28 review P2: the negative widths were silently coerced to 1 by
	// sigmoidScore — a near-step ramp (RSI 80→12, 85→95) instead of the
	// intended smooth ±7/±8 slope (85→61), over-punishing the 80-90 band.
	ob1h := clamp100(sigmoidScore(rsi1h, 82, 7)) * bandFade(rsi1h, 65, 95)
	ob4h := clamp100(sigmoidScore(rsi4h, 78, 8)) * bandFade(rsi4h, 60, 95)
	sig.Components.Overbought = 0.6*ob1h + 0.4*ob4h

	// ── 3. Rejection: upper wick share on the last 3 closed 1h candles ──
	rej := 0.0
	n3 := 3
	if len(k1h) < n3 {
		n3 = len(k1h)
	}
	for _, k := range k1h[len(k1h)-n3:] {
		r := k.High - k.Low
		if r > 0 {
			rej += (k.High - math.Max(k.Open, k.Close)) / r
		}
	}
	rej /= float64(n3)
	sig.Components.Rejection = clamp100(sigmoidScore(rej, 0.35, 0.15))

	// ── 4. Volume fade / price-volume divergence ──
	if len(k1h) >= 24 {
		var vols []float64
		for _, k := range k1h[len(k1h)-24:] {
			vols = append(vols, k.QuoteVolume)
		}
		recent := mean(vols[len(vols)-3:])
		peak := 0.0
		for _, v := range vols {
			if v > peak {
				peak = v
			}
		}
		if peak > 0 {
			ratio := recent / peak
			// 09-28 review P1: width was passed negative (-0.2), which
			// sigmoidScore coerces to 1 — an INCREASING curve that scored
			// volume PERSISTENCE (ratio→1) higher than a heavy fade
			// (ratio→0), inverting the dimension and deadening the ×0.8
			// momentum-trap discount. Decreasing in ratio via input
			// negation (the package's convention, cf. breakdownscan.go).
			sig.Components.VolumeFade = clamp100(sigmoidScore(-ratio, -0.45, 0.2))
			// Price printing new 24-bar highs while volume runs well below its
			// peak = classic price/volume divergence on the pump.
			if c1h[len(c1h)-1] >= maxOf(c1h[len(c1h)-24:]) && ratio < 0.7 {
				sig.Components.VolumeFade = math.Max(sig.Components.VolumeFade, 70)
			}
		}
	}

	// ── 5. Extension: distance above EMA20/EMA50 (1h weighted, 4h bonus) ──
	e20 := ema(c1h, 20)
	e50 := ema(c1h, 50)
	last := c1h[len(c1h)-1]
	ext := 0.0
	if e50[len(e50)-1] > 0 {
		ext += (last/e50[len(e50)-1] - 1) * 100
	}
	if e20[len(e20)-1] > 0 {
		ext += (last/e20[len(e20)-1] - 1) * 100 * 0.5
	}
	e20_4h := ema(c4h, 20)
	if e20_4h[len(e20_4h)-1] > 0 {
		ext += (c4h[len(c4h)-1]/e20_4h[len(e20_4h)-1] - 1) * 100 * 0.8
	}
	sig.Components.Extension = clamp100(sigmoidScore(ext, 8, 5))

	// ── 6. Crowding: funding + OI build-up + long/short account skew ──
	// Annualization uses the FORWARD rate (what the next settlement charges,
	// same figure the signal block shows) and the symbol's REAL settlement
	// interval — a fixed 3×/day assumption understates 1h/4h-interval listings
	// by 3-8×. The crowding fundPart scores the SAME annualized figure
	// (review 2026-10-04 #9: it used to score the raw per-settlement rate, so
	// a 1h-settlement coin at 0.01%/h ≈ 876%/yr annualized read as mild).
	fInfo, fiErr := ds.FundingInfo()
	fundKnown := false
	var cur float64
	if fiErr == nil && fInfo != nil {
		cur = fInfo.NextRate
		settlePerDay := 24.0 / fInfo.IntervalHours
		if settlePerDay < 1 {
			settlePerDay = 1
		}
		sig.FundingAnnualPct = round2(cur * settlePerDay * 365 * 100)
		fundKnown = true
	}
	if f, ferr := ds.FundingHistory(6); ferr == nil && len(f) > 0 {
		if !fundKnown {
			cur = f[len(f)-1].Rate
			sig.FundingAnnualPct = round2(cur * 3 * 365 * 100)
			fundKnown = true
		}
		if len(f) >= 4 {
			prev := f[len(f)-4].Rate
			// Single shared definition of "rate rolled over from a high"
			// (review 09-15 point 6): the scanner keeps its looser snapshot
			// threshold (0.01%/period), while the structured signal applies
			// the configured crowding threshold — same predicate, same
			// 3-settlement lookback, no divergent wording per source.
			sig.FundingRollover = market.FundingRolloverDetected(prev, cur, 0.0001)
		}
	}
	// The three parts are fetched independently — a funding-history failure
	// used to zero the WHOLE component, discarding working OI and L/S
	// evidence with it (review 2026-10-04 #9). Missing data scores 0 (no
	// crowding points awarded on absent evidence, the fail-closed side);
	// available-part weights renormalize to keep the component on scale.
	fundPart := 0.0
	if fundKnown {
		fundPart = clamp100(sigmoidScore(sig.FundingAnnualPct, 50, 40))
	}
	oiPart, oiKnown := 0.0, false
	if oi, oerr := ds.OIHistory("5m", 288); oerr == nil && len(oi) >= 49 {
		vals := oiValueSeries(oi)
		sig.OIValueMillions = round2(vals[len(vals)-1] / 1_000_000)
		// review 2026-10-07 S6: 4h build-up is measured on contract QUANTITY
		// (OIPoint.OI). Notional Value rises with price even while contracts
		// are being closed, which read a pump as fresh long crowding.
		if base := oi[len(oi)-49].OI; base > 0 {
			oiChg := (oi[len(oi)-1].OI - base) / base * 100
			oiPart = clamp100(sigmoidScore(oiChg, 5, 4))
			oiKnown = true
		} else {
			oiKnown = true // measured, unusable baseline — still "present"
		}
	}
	lsPart, lsKnown := 0.0, false
	if ls, lerr := ds.LongShortRatio("1h", 3); lerr == nil && len(ls) > 0 {
		v := ls[len(ls)-1].Ratio
		sig.LongShortRatio = &v
		// 1.0 = balanced; ≥2 = accounts heavily skewed long.
		lsPart = clamp100(sigmoidScore(v-1, 0.5, 0.35))
		lsKnown = true
	}
	wFund, wOI, wLS := 0.55, 0.25, 0.20
	if !lsKnown {
		wFund, wOI, wLS = 0.70, 0.30, 0
	}
	wSum := 0.0
	if fundKnown {
		wSum += wFund
	}
	if oiKnown {
		wSum += wOI
	}
	if lsKnown {
		wSum += wLS
	}
	if wSum > 0 {
		crowd := 0.0
		if fundKnown {
			crowd += wFund / wSum * fundPart
		}
		if oiKnown {
			crowd += wOI / wSum * oiPart
		}
		if lsKnown {
			crowd += wLS / wSum * lsPart
		}
		sig.Components.Crowding = clamp100(crowd)
	}

	// ── 7. Parabolic pump: 5d magnitude + acceleration + BTC outperformance ──
	// review 2026-10-07 S5: both windows span 30 intervals (recent 1..31 back,
	// prior 31..61 back); the prior one used to span only 25, biasing accel.
	if recent5d, prior5d, ok := pumpWindows5d(c4h); ok {
		accel := recent5d - prior5d
		parabolic := 0.6*clamp100(sigmoidScore(recent5d, 40, 25)) +
			0.4*clamp100(sigmoidScore(accel, 15, 12))
		sig.Change5dPct = round2(recent5d)
		if len(btc4h) >= 56 {
			bc := closes(btc4h)
			btc5d := (bc[len(bc)-1]/bc[len(bc)-31] - 1) * 100
			sig.BtcOutperform = round2(recent5d - btc5d)
			outperform := clamp100(sigmoidScore(sig.BtcOutperform, 20, 15))
			parabolic = 0.7*parabolic + 0.3*outperform
		}
		sig.Components.Parabolic = clamp100(parabolic)
	}

	// ── 8. Bearish divergence on 4h: price higher high, RSI lower high ──
	if len(k4h) >= 30 {
		highs4h := highs(k4h)
		rsi4hSeries := rsiSeries(c4h, 14)
		if len(rsi4hSeries) == len(c4h) {
			n := len(c4h)
			recentHi, recentIdx := math.Inf(-1), -1
			for i := n - 10; i < n; i++ {
				if highs4h[i] > recentHi {
					recentHi, recentIdx = highs4h[i], i
				}
			}
			priorHi, priorIdx := math.Inf(-1), -1
			for i := n - 30; i < n-10; i++ {
				if highs4h[i] > priorHi {
					priorHi, priorIdx = highs4h[i], i
				}
			}
			if recentIdx >= 0 && priorIdx >= 0 && recentHi > priorHi &&
				rsi4hSeries[recentIdx] < rsi4hSeries[priorIdx]-2 {
				sig.BearishDiv4h = true
			}
			sig.Components.Divergence = divScore(sig.BearishDiv4h, recentHi, priorHi)
		}
	}

	// ── 9. Structure: failed breakout above the prior swing high / fresh EMA20 break ──
	if len(k1h) >= 53 {
		lastClose := c1h[len(c1h)-1]
		for i := len(k1h) - 5; i < len(k1h); i++ {
			swing := 0.0
			for j := i - 48; j < i; j++ {
				if k1h[j].High > swing {
					swing = k1h[j].High
				}
			}
			// The CURRENT close must still sit below the broken swing: a fake
			// from 4 hours ago that has since been reclaimed and held is trend
			// continuation, not a top (review 2026-10-04 #8 — the flag used to
			// stick to any recent spike-and-recover regardless of where price
			// was now).
			if swing > 0 && k1h[i].High > swing && k1h[i].Close < swing && lastClose < swing {
				sig.FakeBreakout = true
				break
			}
		}
	}
	if len(e20) >= 5 {
		brokeNow := c1h[len(c1h)-1] < e20[len(e20)-1]
		wasAbove := c1h[len(c1h)-5] > e20[len(e20)-5]
		if brokeNow && wasAbove {
			sig.MABreak = true
		}
	}
	structure := 0.0
	if sig.FakeBreakout {
		structure = 100
	} else if sig.MABreak {
		structure = 80
	}
	if sig.FundingRollover {
		// Rollover is an ADDITIVE bonus on top of the assigned value, clamped
		// to the 0-100 scale (audit 2026-09-12 #4): fake_breakout(100) stays
		// 100, ma_break(80) → 100, none(0) → 20.
		structure = math.Min(100, structure+20)
	}
	sig.Components.Structure = structure
	// review 2026-10-07 S2/S3: topping confirmation = 4h bearish divergence or
	// fake breakout ONLY. A bare 1h EMA20 break (structure=80) had a 47% win
	// rate / -0.99% mean in the journal, so MABreak stays a flag + Structure
	// score input but is not a confirmation. FundingRollover here uses a loose
	// fixed threshold (0.01%/period); the kernel gate accepts the live
	// strategy-threshold funding_rollover.detected instead.
	sig.Confirmed = sig.BearishDiv4h || sig.FakeBreakout

	w := shortWeights()
	sig.Score = round2(w["stretch"]*sig.Components.Stretch +
		w["overbought"]*sig.Components.Overbought +
		w["rejection"]*sig.Components.Rejection +
		w["volume_fade"]*sig.Components.VolumeFade +
		w["extension"]*sig.Components.Extension +
		w["crowding"]*sig.Components.Crowding +
		w["parabolic"]*sig.Components.Parabolic +
		w["divergence"]*sig.Components.Divergence +
		w["structure"]*sig.Components.Structure)

	// Vertical blow-offs without any confirmation are momentum traps: a
	// still-expanding parabolic move punishes early shorts. GRADING of these
	// is already hard-capped at medium (unconfirmed) — this discount's
	// remaining job is RANKING: an unconfirmed vertical pump must not
	// out-rank confirmed medium setups in the top-N slot cut.
	if !sig.Confirmed && sig.Components.VolumeFade < 40 && sig.Change24hPct > 20 {
		sig.Score = round2(sig.Score * 0.8)
	}

	sig.Grade = finalizeShortGrade(sig.Score, sig.Confirmed)
	sig.Reasons = shortReasons(sig, rsi1h, rsi4h, ext)
	return sig, nil
}

// finalizeShortGrade is the SINGLE grading exit for short signals: the
// threshold map plus the unconfirmed strong→medium cap. Every score mutation
// after AnalyzeShort (the BTC-bull haircut in ScanShorts) must re-grade
// THROUGH this helper — re-deriving with shortGradeOf alone used to let an
// unconfirmed raw-84 signal discounted to 71.4 flip back to strong, bypassing
// the cap (review 2026-10-04 #5).
func finalizeShortGrade(score float64, confirmed bool) string {
	g := shortGradeOf(score)
	if !confirmed && g == "strong" {
		return "medium"
	}
	return g
}

// applyShortBtcRegime applies the BTC 4h regime haircut to a ranked short
// board and re-grades through the capped helper. Applied AFTER the slow-top
// merge so grinding-top candidates take the same haircut as gainers (they
// used to slip in post-discount and short BTC-strong coins unpenalized).
func applyShortBtcRegime(out []ShortSignal, mult float64, regime string) {
	for i := range out {
		if mult != 1.0 {
			out[i].Score = round2(clamp100(out[i].Score * mult))
			out[i].Grade = finalizeShortGrade(out[i].Score, out[i].Confirmed)
		}
		if regime != "" {
			out[i].BtcRegime = regime
		}
	}
}

// divScore maps the price/RSI high comparison to a component score: strict
// lower-high divergence = 100, marginal (roughly equal highs) = 40, else 0.
func divScore(div bool, recentHi, priorHi float64) float64 {
	if div {
		return 100
	}
	if priorHi > 0 && recentHi >= priorHi*0.995 {
		return 40
	}
	return 0
}

// rsiSeries returns the Wilder RSI series (index-aligned from index=period).
func rsiSeries(c []float64, period int) []float64 {
	n := len(c)
	if n < period+1 {
		return nil
	}
	out := make([]float64, n)
	var gains, losses float64
	for i := 1; i <= period; i++ {
		d := c[i] - c[i-1]
		if d > 0 {
			gains += d
		} else {
			losses -= d
		}
	}
	ag, al := gains/float64(period), losses/float64(period)
	setRSI := func(i int, ag, al float64) {
		if al == 0 {
			out[i] = 100
		} else {
			out[i] = 100 - 100/(1+ag/al)
		}
	}
	setRSI(period, ag, al)
	for i := period + 1; i < n; i++ {
		d := c[i] - c[i-1]
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		ag = (ag*float64(period-1) + g) / float64(period)
		al = (al*float64(period-1) + l) / float64(period)
		setRSI(i, ag, al)
	}
	return out
}

// shortGradeOf maps the composite score to a grade (short-specific thresholds:
// catching tops is harder than riding momentum, so strong is stricter).
func shortGradeOf(score float64) string {
	switch {
	case score >= 70:
		return "strong"
	case score >= 55:
		return "medium"
	case score >= 40:
		return "weak"
	default:
		return "noise"
	}
}

func shortReasons(sig *ShortSignal, rsi1h, rsi4h, ext float64) []string {
	var rs []string
	c := sig.Components
	if c.Parabolic >= 60 {
		rs = append(rs, fmt.Sprintf("5 日 +%.0f%% 呈抛物线加速，超出 BTC 同期 %.0f 个百分点", sig.Change5dPct, sig.BtcOutperform))
	}
	if rsi1h >= 70 {
		rs = append(rs, fmt.Sprintf("1h RSI %.0f 超买", rsi1h))
	}
	if rsi4h >= 68 {
		rs = append(rs, fmt.Sprintf("4h RSI %.0f 超买", rsi4h))
	}
	if c.Extension >= 60 {
		rs = append(rs, fmt.Sprintf("价格偏离 1h/4h EMA 高达 +%.1f%%，乖离过大", ext))
	}
	if c.Rejection >= 55 {
		rs = append(rs, "近期 1h K 线上影线密集，上方抛压明显")
	}
	if c.VolumeFade >= 60 {
		rs = append(rs, "价格创新高但量能萎缩，价量背离")
	}
	if sig.BearishDiv4h {
		rs = append(rs, "4h 级别看跌背离：价格新高但 RSI 不创新高")
	}
	if sig.FakeBreakout {
		rs = append(rs, "假突破：冲破前高后迅速回落收于前高之下")
	}
	if sig.MABreak {
		rs = append(rs, "跌破 1h EMA20，短线结构转弱")
	}
	if sig.FundingAnnualPct >= 50 {
		rs = append(rs, fmt.Sprintf("资金费率年化约 %.0f%%，多头持有成本极高", sig.FundingAnnualPct))
	}
	if sig.FundingRollover {
		rs = append(rs, "资金费率从高位回落，多头开始撤退")
	}
	if sig.LongShortRatio != nil && *sig.LongShortRatio >= 2 {
		rs = append(rs, fmt.Sprintf("多空账户比 %.2f，散户多头一边倒", *sig.LongShortRatio))
	}
	if len(rs) == 0 {
		rs = append(rs, "各维度暂无显著做空信号")
	}
	return rs
}

// BTC4hRegime is the SINGLE shared BTC 4h trend classifier — it drives the
// short-scan haircut, the scheduler's regime label and the kernel's BTC
// entry filters (review 2026-10-04 #3: the pipeline used to carry three
// divergent definitions — BTC's own piggy score, an EMA+RSI read here, and
// the kernel's EMA-only shape — that could contradict each other on the
// same bar). Predicates: bullStructure = EMA20>EMA50 with the last close
// above EMA20; bearStructure mirrors; strongBull adds RSI(14) ≥ 60 (only
// the BULL side needs the momentum check — that is what makes shorts
// systematically dangerous). known=false below 60 closes: the EMA50 is not
// converged enough to classify on (kernel fetches 300, the scanners 84 —
// both clear it).
func BTC4hRegime(closes []float64) (regime string, strongBull bool, downtrend bool, known bool) {
	n := len(closes)
	if n < 60 {
		return "", false, false, false
	}
	e20 := ema(closes, 20)
	e50 := ema(closes, 50)
	last := closes[n-1]
	bullStruct := e20[n-1] > e50[n-1] && last > e20[n-1]
	bearStruct := e20[n-1] < e50[n-1] && last < e20[n-1]
	rsi := rsiLast(closes, 14)
	switch {
	case bullStruct && rsi >= 60:
		return "btc_bull", true, false, true
	case bearStruct:
		// Regime label mirrors for context; no bear-side score boost.
		return "btc_bear", false, true, true
	default:
		return "chop", false, false, true
	}
}

// btcRegimePenalty classifies BTC's own 4h trend and returns the global
// multiplier for short candidates: in a strong BTC uptrend (EMA20>EMA50,
// close above both, RSI strong) altcoin shorts are systematically lower
// probability — the whole board takes one haircut instead of asking the
// decision layer to discount every signal by hand. Mirror regimes pass
// through at 1.0. Returns (multiplier, regimeLabel).
func btcRegimePenalty(btc4h []Kline) (float64, string) {
	if len(btc4h) < 30 {
		return 1.0, ""
	}
	regime, strongBull, _, known := BTC4hRegime(closes(btc4h))
	if !known {
		return 1.0, ""
	}
	if strongBull {
		return 0.85, regime
	}
	return 1.0, regime
}

// ScanShorts ranks the top `limit` 24h gainers by short suitability, plus
// the history pool (histDaysRaw/histMaxRaw: 0 = defaults, negative disables
// the pool — resolved via ResolveShortScanHistory*, single default source)
// and the slow-top universe. Results are cached per resolved config for
// shortScanCacheTTL; concurrent calls with the SAME config share one scan.
// Newly listed symbols (< 7 days of 4h history) are skipped — thin history
// and tiny float make squeeze risk unacceptable.
func ScanShorts(limit, histDaysRaw, histMaxRaw int) ([]ShortSignal, time.Time, error) {
	histDays := ResolveShortScanHistoryDays(histDaysRaw)
	histMax := ResolveShortScanHistoryMax(histMaxRaw)
	fullList := limit <= 0 || limit >= ShortScanUniverse
	if !fullList && limit < 1 {
		limit = 10
	}
	cacheKey := fmt.Sprintf("%d|%d|%d", ShortScanUniverse, histDays, histMax)
	shortScanCacheMu.Lock()
	if ent := shortScanCache[cacheKey]; ent != nil && time.Now().Before(ent.expires) {
		r, up := ent.results, ent.updatedAt
		shortScanCacheMu.Unlock()
		if !fullList && len(r) > limit {
			r = r[:limit]
		}
		return r, up, nil
	}
	if shortScanInflight[cacheKey] {
		shortScanCacheMu.Unlock()
		// Wait for the in-flight scan to populate the cache.
		for i := 0; i < 100; i++ {
			time.Sleep(300 * time.Millisecond)
			shortScanCacheMu.Lock()
			if ent := shortScanCache[cacheKey]; ent != nil && time.Now().Before(ent.expires) {
				r, up := ent.results, ent.updatedAt
				shortScanCacheMu.Unlock()
				if !fullList && len(r) > limit {
					r = r[:limit]
				}
				return r, up, nil
			}
			shortScanCacheMu.Unlock()
		}
		return nil, time.Time{}, fmt.Errorf("short scan in progress, please retry")
	}
	shortScanInflight[cacheKey] = true
	shortScanCacheMu.Unlock()
	defer func() {
		shortScanCacheMu.Lock()
		shortScanInflight[cacheKey] = false
		shortScanCacheMu.Unlock()
	}()

	board, losers, index, err := gainerTickerSnapshot(ShortScanUniverse)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to list gainers: %w", err)
	}
	// Persist today's board into the gainer history pool (历史涨幅池) BEFORE
	// analysis — the record is the durable asset; a failed analysis run must
	// not lose the day's snapshot. Retention is a fixed anti-bloat cap (R3);
	// the per-strategy window applies at consumption time.
	recordGainerHistory(board, time.Now())

	// Work items: the live gainer board plus, when enabled, history-pool
	// symbols that pumped within the window but already fell off today's
	// board (universe=hist_gainer). Live quotes for history symbols come
	// from the same ticker snapshot — analysis inputs stay current.
	type shortScanItem struct {
		symbol   string
		chg      float64
		universe string
	}
	items := make([]shortScanItem, 0, len(board)+histMax)
	covered := make(map[string]bool, len(board))
	for _, g := range board {
		// Still marked covered: a sub-threshold board coin must not re-enter
		// as hist_gainer/breakdown under a different label this cycle.
		covered[g.Symbol] = true
		if g.ChgPct < shortScanMinGainerChgPct {
			continue
		}
		items = append(items, shortScanItem{g.Symbol, g.ChgPct, "gainer"})
	}
	if histDays > 0 {
		extra := historyUniverseCandidates(loadGainerHistory(), time.Now(), histDays, index, covered, histMax)
		for _, q := range extra {
			items = append(items, shortScanItem{q.Symbol, q.ChgPct, "hist_gainer"})
		}
		if len(extra) > 0 {
			logger.Infof("🩸 Gainer history pool: +%d symbols over %dd window (universe=hist_gainer)", len(extra), histDays)
		}
	}
	// Breakdown universe (A1): 24h losers not already covered by the gainer
	// board — a coin on both boards is whipsaw, the gainer label wins.
	bdAdded := 0
	for _, q := range losers {
		if covered[q.Symbol] {
			continue
		}
		covered[q.Symbol] = true
		items = append(items, shortScanItem{q.Symbol, q.ChgPct, "breakdown"})
		bdAdded++
	}
	if bdAdded > 0 {
		logger.Infof("🩸 Breakdown universe: +%d symbols from the 24h loser board (universe=breakdown)", bdAdded)
	}

	btc4h, _ := NewBinanceDS("BTCUSDT").Klines("4h", 84)

	type res struct {
		sig *ShortSignal
	}
	results := make([]res, len(items))
	skipped := 0
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, shortScanConcurrency)
	)
	for i, it := range items {
		wg.Add(1)
		go func(idx int, symbol string, chg float64) {
			defer wg.Done()
			// Same rationale as AnalyzeMany's worker recover (2026-10-06
			// P2-11): ScanShorts runs on caller goroutines that pass through
			// runOnce OR straight into the kernel's candidate path — a panic
			// here killed the process. Recover per worker, keep symbol and
			// stack; results[idx] stays zero and the symbol drops out.
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("🩸 Short scan panicked for %s (worker recovered): %v\n%s", symbol, r, debug.Stack())
				}
			}()
			sem <- struct{}{}
			defer func() { <-sem }()

			var sig *ShortSignal
			var err error
			if it.universe == "breakdown" {
				sig, err = analyzeBreakdownShort(symbol, chg, btc4h, NewBinanceDS(symbol))
				// nil,nil = downtrend gate said no — not a continuation setup.
			} else {
				sig, err = AnalyzeShort(symbol, chg, btc4h, NewBinanceDS(symbol))
			}
			if err == nil && sig != nil {
				mu.Lock()
				if sig.ListingDays > 0 && sig.ListingDays < 7 {
					skipped++
					mu.Unlock()
					return
				}
				results[idx] = res{sig: sig}
				mu.Unlock()
			}
		}(i, it.symbol, it.chg)
	}
	wg.Wait()
	if skipped > 0 {
		logger.Infof("🩸 Short scan skipped %d newly listed symbols (< 7 days)", skipped)
	}

	var out []ShortSignal
	for i, r := range results {
		if r.sig != nil {
			sig := *r.sig
			sig.Universe = items[i].universe
			out = append(out, sig)
		}
	}
	if len(out) == 0 {
		return nil, time.Time{}, fmt.Errorf("short scan produced no analyzable symbols")
	}
	// Merge the slow-top universe FIRST (grinding tops near their 90d high
	// with 4h bearish divergence — invisible to a 24h-gainer screen), then
	// apply the BTC regime haircut to the merged board: the slow-top entries
	// used to slip in after the discount and short coins unpenalized in a
	// strong BTC uptrend (review 2026-10-04 #5). Gainer results win symbol
	// collisions (they ranked on the live 24h board), but a colliding coin
	// carries NearHighAlso: it equally passed the grinding-top screen, so the
	// pool cut must not re-impose the gainer-side OI floor the slow-top
	// universe is exempt from (user audit 2026-09-17 — the exemption used to
	// die silently on exactly this collision).
	out = mergeShortScans(out, slowTopSnapshot())
	// BTC macro gate: in a strong BTC uptrend every altcoin short carries
	// systematic squeeze risk — one global haircut instead of manual vetting.
	// Mirror regimes pass through at 1.0, labelled for context only (the old
	// "breakdown mildly BOOSTS scores" comment never matched the code).
	mult, regime := btcRegimePenalty(btc4h)
	applyShortBtcRegime(out, mult, regime)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	n := len(out)
	for i := range out {
		out[i].Percentile = round2(float64(n-i) / float64(n) * 100)
	}
	now := time.Now()
	shortScanCacheMu.Lock()
	shortScanCache[cacheKey] = &shortScanCacheEntry{
		results:   out,
		expires:   now.Add(shortScanCacheTTL),
		updatedAt: now,
	}
	shortScanCacheMu.Unlock()

	if !fullList && len(out) > limit {
		out = out[:limit]
	}
	return out, now, nil
}

// mergeShortScans merges the slow-top universe into the gainer ranking.
// Symbol collisions keep the gainer entry (same AnalyzeShort score either
// way) but stamp NearHighAlso — see the ScanShorts merge comment.
func mergeShortScans(gainers, slowTops []ShortSignal) []ShortSignal {
	out := make([]ShortSignal, len(gainers), len(gainers)+len(slowTops))
	copy(out, gainers)
	for _, sig := range slowTops {
		dup := false
		for i := range out {
			if out[i].Symbol == sig.Symbol {
				out[i].NearHighAlso = true
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, sig)
		}
	}
	return out
}

// bandFade tapers the score linearly outside [lo, hi]: below lo or above hi
// the edge is gone (oversold already, or vertical blow-off too risky to fade).
func bandFade(v, lo, hi float64) float64 {
	switch {
	case v < lo:
		return math.Max(0, 1-(lo-v)/10)
	case v > hi:
		return math.Max(0, 1-(v-hi)/10)
	default:
		return 1
	}
}

func clamp100(v float64) float64 {
	// NaN fails both comparisons and would flow straight through into the
	// weighted score (review 2026-10-04 #9: a zero OI baseline propagated
	// NaN through the OI delta). Absent/unusable evidence scores 0.
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// mean2 returns the population variance given a precomputed mean.
func mean2(xs []float64, m float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	return s / float64(len(xs))
}
