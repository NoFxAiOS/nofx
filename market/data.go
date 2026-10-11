package market

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/security"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FundingRateCache is the funding rate cache structure
// premiumIndex.lastFundingRate is the CONTINUOUSLY DRIFTING forward estimate
// for the next settlement — only the settlement event is 8h-bound, the rate
// value itself can halve within minutes on pumping symbols. A 1-hour cache
// fed the AI prompt stale figures (observed: scanner 117% vs signal 52%).
type FundingRateCache struct {
	Rate      float64
	UpdatedAt time.Time
}

var (
	fundingRateMap sync.Map // map[string]*FundingRateCache
	frCacheTTL     = 5 * time.Minute
)

// FundingIntervalCache holds the measured settlement interval per symbol.
type FundingIntervalCache struct {
	Hours     float64
	UpdatedAt time.Time
}

var (
	fundingIntervalMap sync.Map // map[string]*FundingIntervalCache
	fundingIntervalTTL = 24 * time.Hour
)

// fundingIntervalHours returns the symbol's real funding settlement interval
// in hours, measured from recent settlement timestamps (0 = unknown).
// The interval is fixed per listing (8h default; some new listings 4h/1h)
// and changes rarely, so it is cached for a day. The measurement rides on
// the same /fapi/v1/fundingRate call that feeds the rollover history
// (funding_history.go) — one endpoint call per 5-minute cache window serves
// both.
func fundingIntervalHours(symbol string) float64 {
	if cached, ok := fundingIntervalMap.Load(symbol); ok {
		c := cached.(*FundingIntervalCache)
		if time.Since(c.UpdatedAt) < fundingIntervalTTL {
			return c.Hours
		}
	}
	_, times, ok := fundingHistory(symbol)
	if !ok || len(times) < 2 {
		return 0
	}
	hours := medianFundingIntervalHours(times)
	fundingIntervalMap.Store(symbol, &FundingIntervalCache{Hours: hours, UpdatedAt: time.Now()})
	return hours
}

// medianFundingIntervalHours: the MEDIAN settlement gap, not the mean
// (09-28 review P3) — Binance throttles new listings through 1h→4h→8h
// intervals, and an interval change inside the fetched window poisons the
// unweighted mean (gaps [1,4,4,8,8] → mean 5.0h vs the true current 4h or
// 8h basis), skewing the annualized funding that feeds the crowding math.
// The median is robust to the change boundary. Gaps must already be
// filtered to (0, 24].
func medianFundingIntervalHours(times []int64) float64 {
	var gaps []float64
	for i := 1; i < len(times); i++ {
		h := float64(times[i]-times[i-1]) / 3.6e6
		if h > 0 && h <= 24 {
			gaps = append(gaps, h)
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	sort.Float64s(gaps)
	mid := len(gaps) / 2
	if len(gaps)%2 == 1 {
		return gaps[mid]
	}
	return (gaps[mid-1] + gaps[mid]) / 2
}

// Get retrieves market data for the specified token (uses Binance data by default)
func Get(symbol string) (*Data, error) {
	return GetWithExchange(symbol, "binance")
}

// GetWithExchange retrieves market data for the specified token using exchange-specific data
func GetWithExchange(symbol, exchange string) (*Data, error) {
	return GetWithExchangeAndPrice(symbol, exchange, 0)
}

// GetWithExchangeAndPrice builds trading data using the caller's live price
// from the same exchange that will execute the order. A Binance ticker must
// never silently stand in for another venue's quote.
func GetWithExchangeAndPrice(symbol, exchange string, livePrice float64) (*Data, error) {
	var klines3m, klines4h, klines1h []Kline
	var err error
	// Normalize symbol
	symbol = Normalize(symbol)

	// Check if this is an xyz dex asset (use Hyperliquid API)
	isXyzAsset := IsXyzDexAsset(symbol)

	// For hyperliquid exchange, also use Hyperliquid API
	useHyperliquidAPI := isXyzAsset || strings.ToLower(exchange) == "hyperliquid"

	// Get 3-minute K-line data (or 5-minute for xyz assets as 3m may not be available)
	if useHyperliquidAPI {
		// Use Hyperliquid API for xyz dex assets (use 5m since 3m may not be available)
		klines3m, err = getKlinesFromHyperliquid(symbol, "5m", 100)
		if err != nil {
			return nil, fmt.Errorf("Failed to get 5-minute K-line from Hyperliquid: %v", err)
		}
	} else {
		// Use CoinAnk for regular crypto assets with exchange-specific data
		klines3m, err = getExecutionKlines(symbol, "3m", exchange, 100)
		if err != nil {
			return nil, fmt.Errorf("Failed to get 3-minute K-line from CoinAnk (%s): %v", exchange, err)
		}
	}

	// Data staleness detection: Prevent DOGEUSDT-style price freeze issues
	if isStaleData(klines3m, symbol) {
		logger.Infof("⚠️  WARNING: %s detected stale data (consecutive price freeze), skipping symbol", symbol)
		return nil, fmt.Errorf("%s data is stale, possible cache failure", symbol)
	}

	// Get 4-hour K-line data
	if useHyperliquidAPI {
		klines4h, err = getKlinesFromHyperliquid(symbol, "4h", 100)
		if err != nil {
			return nil, fmt.Errorf("Failed to get 4-hour K-line from Hyperliquid: %v", err)
		}
	} else {
		klines4h, err = getExecutionKlines(symbol, "4h", exchange, 100)
		if err != nil {
			return nil, fmt.Errorf("Failed to get 4-hour K-line from CoinAnk (%s): %v", exchange, err)
		}
	}

	// 1-hour K-lines (best-effort): the stop-band FLOOR yardstick is
	// ATR(1h) and the vol-target/trailing yardsticks read it too — without
	// this series those gates silently skipped (the ATR helpers return 0 on
	// missing data). On failure the floor's fallback chain rides 4h: a
	// wider but still-enforced floor.
	if useHyperliquidAPI {
		klines1h, err = getKlinesFromHyperliquid(symbol, "1h", 100)
	} else {
		klines1h, err = getExecutionKlines(symbol, "1h", exchange, 100)
	}
	if err != nil {
		logger.Infof("⚠️ Failed to get %s 1h K-line (%v) — stop-floor ATR falls back to 4h", symbol, err)
		klines1h = nil
	}

	// 1d K-lines (best-effort, 2026-09-25 user directive): equity tokens
	// (bstock) track their underlying's SESSION — with overnight gaps the
	// daily bar is the honest volatility yardstick, and 1h/4h ATR set
	// systematically too-tight stops on them. Fetched for every symbol so
	// the stock check needs no data-path branch; unused by crypto paths.
	var klines1d []Kline
	if useHyperliquidAPI {
		klines1d, err = getKlinesFromHyperliquid(symbol, "1d", 100)
	} else {
		klines1d, err = getExecutionKlines(symbol, "1d", exchange, 100)
	}
	if err != nil {
		// Crypto paths ignore the absent daily series. Bstock entry validation
		// detects the missing exact 1d ATR and fails closed rather than falling
		// back to an intraday risk scale.
		klines1d = nil
	}

	// Check if data is empty
	if len(klines3m) == 0 {
		return nil, fmt.Errorf("3-minute K-line data is empty")
	}
	if len(klines4h) == 0 {
		return nil, fmt.Errorf("4-hour K-line data is empty")
	}

	// Calculate current indicators (based on 3-minute latest data)
	currentPrice := klines3m[len(klines3m)-1].Close
	// Execution price must be supplied by the exchange adapter. Keep the
	// Binance ticker fallback only for legacy direct Binance callers.
	if livePrice > 0 {
		currentPrice = livePrice
	} else if strings.EqualFold(exchange, "binance") {
		p, priceErr := NewAPIClient().GetCurrentPrice(symbol)
		if priceErr != nil {
			return nil, fmt.Errorf("failed to get Binance execution price for %s: %w", symbol, priceErr)
		}
		if p <= 0 {
			return nil, fmt.Errorf("invalid Binance execution price for %s: %g", symbol, p)
		}
		currentPrice = p
	} else {
		return nil, fmt.Errorf("exchange-native live price required for %s on %s", symbol, exchange)
	}
	// Patch the forming 3m candle with the live price so the intraday series
	// and current indicators aren't anchored to the vendor's frozen close.
	if len(klines3m) > 0 {
		last := &klines3m[len(klines3m)-1]
		bar := KlineBar{Time: last.OpenTime, Open: last.Open, High: last.High, Low: last.Low, Close: last.Close}
		if applyLivePrice(&bar, "3m", currentPrice) {
			last.Open, last.High, last.Low, last.Close = bar.Open, bar.High, bar.Low, bar.Close
		}
	}
	currentEMA20 := calculateEMA(klines3m, 20)
	currentMACD := calculateMACD(klines3m)
	currentRSI7 := calculateRSI(klines3m, 7)

	// Calculate price change percentage
	// 1-hour price change = price from 20 3-minute K-lines ago
	priceChange1h := 0.0
	if len(klines3m) >= 21 { // Need at least 21 K-lines (current + 20 previous)
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 4-hour price change = price from 1 4-hour K-line ago
	priceChange4h := 0.0
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// Get OI data
	oiData, oiOK := fetchOIDataOrZero(symbol)

	// Get Funding Rate + measured settlement interval + settled history
	fundingRate, fundingErr := getFundingRate(symbol)
	fundingSettleHours := fundingIntervalHours(symbol)
	fundingRates, _, histOK := fundingHistory(symbol)
	if !histOK {
		fundingRates = nil
	}

	// Calculate intraday series data
	intradayData := calculateIntradaySeries(klines3m)

	// Calculate longer-term data
	longerTermData := calculateLongerTermData(klines4h)

	// TimeframeData drives every execution-side volatility yardstick: the
	// stop-band floor (1.5×ATR(1h)) and cap (max(2×ATR(4h), 8%)) plus the
	// vol-target/trailing ATR all read it. Leaving it nil silently disabled
	// the whole family — the cap degenerated to the fixed 8% and the floor
	// never enforced (09-16 AKEUSDT: a healthy 11.45% structural stop was
	// rejected by the degenerate 8% cap the prompt never promised).
	TimeframeData := executionTimeframeData(klines3m, klines1h, klines4h, klines1d)

	return &Data{
		Symbol:             symbol,
		CurrentPrice:       currentPrice,
		PriceChange1h:      priceChange1h,
		PriceChange4h:      priceChange4h,
		CurrentEMA20:       currentEMA20,
		CurrentMACD:        currentMACD,
		CurrentRSI7:        currentRSI7,
		OpenInterest:       oiData,
		OpenInterestOK:     oiOK,
		FundingRate:        fundingRate,
		FundingRateOK:      fundingErr == nil,
		FundingSettleHours: fundingSettleHours,
		FundingHistory:     fundingRates,
		IntradaySeries:     intradayData,
		LongerTermContext:  longerTermData,
		TimeframeData:      TimeframeData,
	}, nil
}

// GetWithTimeframes retrieves market data for specified multiple timeframes
// timeframes: list of timeframes, e.g. ["5m", "15m", "1h", "4h"]
// primaryTimeframe: primary timeframe (used for calculating current indicators), defaults to timeframes[0]
// count: number of K-lines for each timeframe
func GetWithTimeframes(symbol string, timeframes []string, primaryTimeframe string, count int) (*Data, error) {
	return getWithTimeframes(symbol, timeframes, primaryTimeframe, count, "binance", 0, false)
}

// GetWithTimeframesForExchange fetches candles and prices for the venue that
// will execute the trade. A positive livePrice must come from that venue's
// exchange adapter; other-venue and generic spot fallbacks are disabled.
func GetWithTimeframesForExchange(symbol string, timeframes []string, primaryTimeframe string, count int, exchange string, livePrice float64) (*Data, error) {
	return getWithTimeframes(symbol, timeframes, primaryTimeframe, count, exchange, livePrice, true)
}

// GetWithTimeframesForVenue fetches ANALYSIS candles for a specific venue
// without requiring an execution live quote (F10, 2026-10-01 review): the
// venue's own kline series is used (CoinAnk per-venue series, no cross-venue
// fallback for non-Binance), and the forming candle prices at the venue's
// last close. The decision layer must analyze the market it will execute on
// — the generic GetWithTimeframes pins Binance, so a Bybit/OKX trader would
// take Binance-derived structures/anchors into another venue's book.
func GetWithTimeframesForVenue(symbol string, timeframes []string, primaryTimeframe string, count int, exchange string) (*Data, error) {
	if exchange == "" {
		exchange = "binance"
	}
	return getWithTimeframes(symbol, timeframes, primaryTimeframe, count, exchange, 0, false)
}

func getWithTimeframes(symbol string, timeframes []string, primaryTimeframe string, count int, exchange string, livePrice float64, executionData bool) (*Data, error) {
	symbol = Normalize(symbol)

	if len(timeframes) == 0 {
		return nil, fmt.Errorf("at least one timeframe is required")
	}

	// If primary timeframe is not specified, use the first one
	if primaryTimeframe == "" {
		primaryTimeframe = timeframes[0]
	}

	// Ensure primary timeframe is in the list
	hasPrimary := false
	for _, tf := range timeframes {
		if tf == primaryTimeframe {
			hasPrimary = true
			break
		}
	}
	if !hasPrimary {
		timeframes = append([]string{primaryTimeframe}, timeframes...)
	}

	// Store data for all timeframes
	timeframeData := make(map[string]*TimeframeSeriesData)
	var primaryKlines []Kline

	// Check if this is an xyz dex asset (use Hyperliquid API)
	isXyzAsset := IsXyzDexAsset(symbol)
	useHyperliquidAPI := isXyzAsset || strings.EqualFold(exchange, "hyperliquid")

	// Get K-line data for each timeframe — request the strategy's configured
	// count (floored at 200 so short configs still have indicator history;
	// capped at 1500, the max most sources return per call).
	fetchCount := count
	if fetchCount < 200 {
		fetchCount = 200
	}
	if fetchCount > 1500 {
		fetchCount = 1500
	}

	for _, tf := range timeframes {
		var klines []Kline
		var err error

		if useHyperliquidAPI {
			// Use Hyperliquid API for xyz dex assets
			klines, err = getKlinesFromHyperliquid(symbol, tf, fetchCount)
			if err != nil {
				logger.Infof("⚠️ Failed to get %s %s K-line from Hyperliquid: %v", symbol, tf, err)
				continue
			}
		} else if executionData {
			klines, err = getExecutionKlines(symbol, tf, exchange, fetchCount)
			if err != nil {
				logger.Infof("⚠️ Failed to get %s %s execution-venue K-line (%s): %v", symbol, tf, exchange, err)
				continue
			}
		} else {
			// Generic analysis and strategy preview data defaults to Binance.
			klines, err = getKlinesWithFallback(symbol, tf, exchange, fetchCount)
			if err != nil {
				logger.Infof("⚠️ Failed to get %s %s K-line from CoinAnk: %v", symbol, tf, err)
				continue
			}
		}

		if len(klines) == 0 {
			logger.Infof("⚠️ %s %s K-line data is empty", symbol, tf)
			continue
		}

		// Save primary timeframe K-lines for calculating base indicators
		if tf == primaryTimeframe {
			primaryKlines = klines
		}

		// Calculate series data for this timeframe (use count from config)
		seriesData := calculateTimeframeSeries(klines, tf, count)
		timeframeData[tf] = seriesData
	}

	// If primary timeframe data is empty, return error
	if len(primaryKlines) == 0 {
		return nil, fmt.Errorf("Primary timeframe %s K-line data is empty", primaryTimeframe)
	}

	// Data staleness detection
	if isStaleData(primaryKlines, symbol) {
		logger.Infof("⚠️  WARNING: %s detected stale data (consecutive price freeze), skipping symbol", symbol)
		return nil, fmt.Errorf("%s data is stale, possible cache failure", symbol)
	}

	// Calculate current indicators (based on primary timeframe latest data)
	currentPrice := primaryKlines[len(primaryKlines)-1].Close

	// Use the same-venue quote provided by the caller for execution analysis.
	// Only the legacy generic Binance analysis path may query Binance here.
	if livePrice > 0 {
		currentPrice = livePrice
	} else if !executionData && strings.EqualFold(exchange, "binance") && !isXyzAsset {
		p, err := NewAPIClient().GetCurrentPrice(symbol)
		if err == nil && p > 0 {
			livePrice = p
			currentPrice = p
		}
	} else if executionData {
		return nil, fmt.Errorf("exchange-native live price required for %s on %s", symbol, exchange)
	}

	// Refresh each timeframe's forming candle with the live price. Without
	// this, consumers reading raw klines (the daily technical narrative, the
	// legacy timeframe dumps, grid Donchian levels) quote the vendor's frozen
	// forming-candle close while the structured signal JSON shows the live
	// price — a systematic lag that misleads the model on trending days.
	// Closed candles are never touched.
	vendorStaleness := 0.0
	vendorStalenessKnown := false
	if livePrice > 0 {
		for tf, sd := range timeframeData {
			if div, patched := refreshFormingCandle(tf, sd, livePrice); patched && tf == primaryTimeframe {
				vendorStaleness = div
				vendorStalenessKnown = true
			}
		}
		// 09-19 audit 六: nothing-to-patch (last candle already closed or
		// already fresh) is itself a MEASUREMENT (~0) — report it, instead
		// of an absent field the AI would read as UNKNOWN.
		if !vendorStalenessKnown {
			if sd := timeframeData[primaryTimeframe]; sd != nil && len(sd.Klines) > 0 {
				if pre := sd.Klines[len(sd.Klines)-1].Close; pre > 0 {
					vendorStaleness = (livePrice - pre) / pre * 100
					vendorStalenessKnown = true
				}
			}
		}
	}
	var vendorStalenessP *float64
	if vendorStalenessKnown {
		v := vendorStaleness
		vendorStalenessP = &v
	}

	currentEMA20 := calculateEMA(primaryKlines, 20)
	currentMACD := calculateMACD(primaryKlines)
	currentRSI7 := calculateRSI(primaryKlines, 7)

	// Calculate price changes
	priceChange1h := calculatePriceChangeByBars(primaryKlines, primaryTimeframe, 60, currentPrice)  // 1 hour
	priceChange4h := calculatePriceChangeByBars(primaryKlines, primaryTimeframe, 240, currentPrice) // 4 hours

	// Get OI data
	oiData, oiOK := fetchOIDataOrZero(symbol)

	// Get Funding Rate + measured settlement interval + settled history
	fundingRate, fundingErr := getFundingRate(symbol)
	fundingSettleHours := fundingIntervalHours(symbol)
	fundingRates, _, histOK := fundingHistory(symbol)
	if !histOK {
		fundingRates = nil
	}

	return &Data{
		Symbol:             symbol,
		CurrentPrice:       currentPrice,
		PriceChange1h:      priceChange1h,
		PriceChange4h:      priceChange4h,
		VendorStalenessPct: vendorStalenessP,
		CurrentEMA20:       currentEMA20,
		CurrentMACD:        currentMACD,
		CurrentRSI7:        currentRSI7,
		OpenInterest:       oiData,
		OpenInterestOK:     oiOK,
		FundingRate:        fundingRate,
		FundingRateOK:      fundingErr == nil,
		FundingSettleHours: fundingSettleHours,
		FundingHistory:     fundingRates,
		TimeframeData:      timeframeData,
	}, nil
}

// TimeframeDuration returns the bar duration of a supported timeframe label,
// or 0 for unknown labels.
func TimeframeDuration(tf string) time.Duration {
	switch tf {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "2h":
		return 2 * time.Hour
	case "4h":
		return 4 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "8h":
		return 8 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "1d":
		return 24 * time.Hour
	// 3d/1w (2026-09-25 P1): the UI offers them and the data source serves
	// them, but the duration table didn't know them — the signal layer's
	// unknown-label fallback (1h) then treated a 3-day/1-week forming candle
	// as settled one hour in, polluting trend/ATR/structure/anchors.
	case "3d":
		return 72 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	}
	return 0
}

// applyLivePrice patches one still-forming candle in place with the live
// ticker price (Close refreshed, High/Low widened). Returns false — leaving
// the candle untouched — when the candle is already closed, the timeframe is
// unknown, or the ticker is implausibly far from the candle (>±80%, which
// indicates broken data rather than volatility; extreme movers are exactly
// where the frozen vendor close is most misleading and must be patched).
func applyLivePrice(bar *KlineBar, tf string, livePrice float64) bool {
	dur := TimeframeDuration(tf)
	if dur <= 0 || bar == nil || bar.Close <= 0 || livePrice <= 0 {
		return false
	}
	if !time.UnixMilli(bar.Time).Add(dur).After(time.Now()) {
		return false // candle already closed
	}
	if livePrice > bar.Close*1.8 || livePrice < bar.Close*0.2 {
		logger.Infof("⚠️ %s forming candle close %.6g vs live price %.6g too far apart, keeping vendor close", tf, bar.Close, livePrice)
		return false
	}
	if livePrice > bar.High {
		bar.High = livePrice
	}
	if livePrice < bar.Low {
		bar.Low = livePrice
	}
	bar.Close = livePrice
	return true
}

// refreshFormingCandle patches the forming candle of a timeframe series with
// the live ticker price so every consumer quoting "the current price" from
// raw klines agrees with Data.CurrentPrice.
func refreshFormingCandle(tf string, sd *TimeframeSeriesData, livePrice float64) (divergencePct float64, patched bool) {
	if sd == nil || len(sd.Klines) == 0 {
		return 0, false
	}
	preClose := sd.Klines[len(sd.Klines)-1].Close
	if applyLivePrice(&sd.Klines[len(sd.Klines)-1], tf, livePrice) {
		last := sd.Klines[len(sd.Klines)-1]
		if len(sd.MidPrices) == len(sd.Klines) {
			sd.MidPrices[len(sd.MidPrices)-1] = (last.High + last.Low) / 2
		}
		if preClose > 0 {
			divergencePct = (livePrice - preClose) / preClose * 100
		}
		return divergencePct, true
	}
	return 0, false
}

// fetchOIDataOrZero wraps getOpenInterestData with the failure convention:
// OpenInterest stays non-nil (zero value) so readers never nil-panic, but the
// ok flag — surfaced as Data.OpenInterestOK — tells level-gating consumers
// (the liquidity filter) that the zero is ABSENCE, not a real reading. A
// silent zero once let one failed endpoint call drop every candidate as "OI
// too low" (round-4 review R4-6).
func fetchOIDataOrZero(symbol string) (*OIData, bool) {
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		logger.Infof("⚠️ %s OI fetch failed (%v) — OI-dependent gates are blind this cycle", symbol, err)
		return &OIData{Latest: 0, Average: 0}, false
	}
	return oiData, true
}

// oiDataTTL: the same symbol is read once per trader per decision cycle (and
// again by the candidate pool); the hourly OI history only gains a bucket per
// hour, so a short cache removes the duplicate /futures/data requests
// (separate 1000 req / 5 min / IP limit) without serving a stale cycle.
const oiDataTTL = 3 * time.Minute

type oiDataEntry struct {
	data *OIData
	at   time.Time
}

var (
	oiDataMu    sync.Mutex
	oiDataCache = map[string]oiDataEntry{}
	// oiNow / oiFetch are test seams.
	oiNow   = time.Now
	oiFetch = fetchOIData
)

// getOpenInterestData retrieves OI data (cached oiDataTTL; failures are never cached).
func getOpenInterestData(symbol string) (*OIData, error) {
	oiDataMu.Lock()
	if e, ok := oiDataCache[symbol]; ok && oiNow().Sub(e.at) < oiDataTTL {
		cp := *e.data
		oiDataMu.Unlock()
		return &cp, nil
	}
	oiDataMu.Unlock()
	d, err := oiFetch(symbol)
	if err != nil {
		return nil, err
	}
	oiDataMu.Lock()
	if len(oiDataCache) > 2000 {
		oiDataCache = map[string]oiDataEntry{}
	}
	cp := *d
	oiDataCache[symbol] = oiDataEntry{data: &cp, at: oiNow()}
	oiDataMu.Unlock()
	return d, nil
}

func fetchOIData(symbol string) (*OIData, error) {
	// Real average from 30 hourly history points — the previous placeholder
	// (Average = Latest × 0.999) made oi_vs_avg_pct a constant ~0.1001% for
	// every symbol, which the model could read as genuine crowding data.
	var hist []struct {
		SumOpenInterest string `json:"sumOpenInterest"`
	}
	if err := binanceGetJSON("/futures/data/openInterestHist?symbol="+symbol+"&period=1h&limit=30", &hist); err != nil {
		return nil, err
	}
	if len(hist) == 0 {
		return nil, fmt.Errorf("empty OI history for %s", symbol)
	}

	latest, _ := strconv.ParseFloat(hist[len(hist)-1].SumOpenInterest, 64)
	var sum float64
	for _, h := range hist {
		v, _ := strconv.ParseFloat(h.SumOpenInterest, 64)
		sum += v
	}
	avg := sum / float64(len(hist))

	return &OIData{
		Latest:  latest,
		Average: avg,
	}, nil
}

// binanceGetJSON GETs a public Binance fapi endpoint and decodes the JSON body.
func binanceGetJSON(path string, out interface{}) error {
	apiClient := NewAPIClient()
	resp, err := apiClient.client.Get(binanceFAPIBase + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

const binanceFAPIBase = "https://fapi.binance.com"

// getFundingRate retrieves funding rate (optimized: uses 1-hour cache)
func getFundingRate(symbol string) (float64, error) {
	// Check cache (1-hour validity)
	// Funding Rate only updates every 8 hours, 1-hour cache is very reasonable
	if cached, ok := fundingRateMap.Load(symbol); ok {
		cache := cached.(*FundingRateCache)
		if time.Since(cache.UpdatedAt) < frCacheTTL {
			// Cache hit, return directly
			return cache.Rate, nil
		}
	}

	// Cache expired or doesn't exist, call API
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	apiClient := NewAPIClient()
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := security.ReadResponseBody(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	if result.Symbol != symbol {
		// A valid-JSON error envelope (e.g. Binance -1121 invalid symbol)
		// unmarshals into empty fields — parsing it would cache a fake 0 for
		// an hour. Only a response that echoes the symbol is real data.
		return 0, fmt.Errorf("premiumIndex response did not echo %s", symbol)
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)

	// Update cache — SUCCESSFUL fetches only; failures must retry next call
	// instead of poisoning the hour.
	fundingRateMap.Store(symbol, &FundingRateCache{
		Rate:      rate,
		UpdatedAt: time.Now(),
	})

	return rate, nil
}

// Format formats and outputs market data
func Format(data *Data) string {
	var sb strings.Builder

	// Format price with dynamic precision
	priceStr := formatPriceWithDynamicPrecision(data.CurrentPrice)
	sb.WriteString(fmt.Sprintf("current_price = %s, current_ema20 = %.3f, current_macd = %.3f, current_rsi (7 period) = %.3f\n\n",
		priceStr, data.CurrentEMA20, data.CurrentMACD, data.CurrentRSI7))

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		// Format OI data with dynamic precision
		oiLatestStr := formatPriceWithDynamicPrecision(data.OpenInterest.Latest)
		oiAverageStr := formatPriceWithDynamicPrecision(data.OpenInterest.Average)
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %s Average: %s\n\n",
			oiLatestStr, oiAverageStr))
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))

	if data.IntradaySeries != nil {
		sb.WriteString("Intraday series (3‑minute intervals, oldest → latest):\n\n")

		if len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
		}

		if len(data.IntradaySeries.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
		}

		if len(data.IntradaySeries.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
		}

		if len(data.IntradaySeries.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
		}

		if len(data.IntradaySeries.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
		}

		if len(data.IntradaySeries.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
		}

		sb.WriteString(fmt.Sprintf("3m ATR (14‑period): %.3f\n\n", data.IntradaySeries.ATR14))
	}

	if data.LongerTermContext != nil {
		sb.WriteString("Longer‑term context (4‑hour timeframe):\n\n")

		sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
			data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))

		sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
			data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))

		sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
			data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))

		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
		}

		if len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
		}
	}

	// Multi-timeframe data (new)
	if len(data.TimeframeData) > 0 {
		// Output sorted by timeframe
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				sb.WriteString(fmt.Sprintf("=== %s Timeframe ===\n\n", strings.ToUpper(tf)))
				formatTimeframeData(&sb, tfData)
			}
		}
	}

	return sb.String()
}

// formatTimeframeData formats data for a single timeframe
func formatTimeframeData(sb *strings.Builder, data *TimeframeSeriesData) {
	// Use OHLCV table format if kline data is available
	if len(data.Klines) > 0 {
		sb.WriteString("Time(UTC)      Open      High      Low       Close     Volume\n")
		for i, k := range data.Klines {
			t := time.Unix(k.Time/1000, 0).UTC()
			timeStr := t.Format("01-02 15:04")
			marker := ""
			if i == len(data.Klines)-1 {
				marker = "  <- current"
			}
			sb.WriteString(fmt.Sprintf("%-14s %-9.4f %-9.4f %-9.4f %-9.4f %-12.2f%s\n",
				timeStr, k.Open, k.High, k.Low, k.Close, k.Volume, marker))
		}
		sb.WriteString("\n")
	} else if len(data.MidPrices) > 0 {
		// Fallback to old format for backward compatibility
		sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.MidPrices)))
		if len(data.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.Volume)))
		}
	}

	// Technical indicators
	if len(data.EMA20Values) > 0 {
		sb.WriteString(fmt.Sprintf("EMA20: %s\n", formatFloatSlice(data.EMA20Values)))
	}

	if len(data.EMA50Values) > 0 {
		sb.WriteString(fmt.Sprintf("EMA50: %s\n", formatFloatSlice(data.EMA50Values)))
	}

	if len(data.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD: %s\n", formatFloatSlice(data.MACDValues)))
	}

	if len(data.RSI7Values) > 0 {
		sb.WriteString(fmt.Sprintf("RSI7: %s\n", formatFloatSlice(data.RSI7Values)))
	}

	if len(data.RSI14Values) > 0 {
		sb.WriteString(fmt.Sprintf("RSI14: %s\n", formatFloatSlice(data.RSI14Values)))
	}

	if data.ATR14 > 0 {
		sb.WriteString(fmt.Sprintf("ATR14: %.4f\n", data.ATR14))
	}

	sb.WriteString("\n")
}

// formatPriceWithDynamicPrecision dynamically selects precision based on price range
// This perfectly supports all coins from ultra-low price meme coins (< 0.0001) to BTC/ETH
func formatPriceWithDynamicPrecision(price float64) string {
	switch {
	case price < 0.0001:
		// Ultra-low price meme coins: 1000SATS, 1000WHY, DOGS
		// 0.00002070 → "0.00002070" (8 decimal places)
		return fmt.Sprintf("%.8f", price)
	case price < 0.001:
		// Low price meme coins: NEIRO, HMSTR, HOT, NOT
		// 0.00015060 → "0.000151" (6 decimal places)
		return fmt.Sprintf("%.6f", price)
	case price < 0.01:
		// Mid-low price coins: PEPE, SHIB, MEME
		// 0.00556800 → "0.005568" (6 decimal places)
		return fmt.Sprintf("%.6f", price)
	case price < 1.0:
		// Low price coins: ASTER, DOGE, ADA, TRX
		// 0.9954 → "0.9954" (4 decimal places)
		return fmt.Sprintf("%.4f", price)
	case price < 100:
		// Mid price coins: SOL, AVAX, LINK, MATIC
		// 23.4567 → "23.4567" (4 decimal places)
		return fmt.Sprintf("%.4f", price)
	default:
		// High price coins: BTC, ETH (save tokens)
		// 45678.9123 → "45678.91" (2 decimal places)
		return fmt.Sprintf("%.2f", price)
	}
}

// formatFloatSlice formats float64 slice to string (using dynamic precision)
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = formatPriceWithDynamicPrecision(v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// xyz dex assets that should NOT get USDT suffix
var xyzDexAssets = map[string]bool{
	// Stocks
	"TSLA": true, "NVDA": true, "AAPL": true, "MSFT": true, "META": true,
	"AMZN": true, "GOOGL": true, "AMD": true, "COIN": true, "NFLX": true,
	"PLTR": true, "HOOD": true, "INTC": true, "MSTR": true, "TSM": true,
	"ORCL": true, "MU": true, "RIVN": true, "COST": true, "LLY": true,
	"CRCL": true, "SKHX": true, "SNDK": true,
	// Forex
	"EUR": true, "JPY": true,
	// Commodities
	"GOLD": true, "SILVER": true,
	// Index
	"XYZ100": true,
}

// IsXyzDexAsset checks if a symbol is an xyz dex asset — i.e. it needs the
// Hyperliquid data path because Binance does NOT list it natively. The
// hand-maintained list went stale after Binance launched native equity
// perps (PLTRUSDT/SNDKUSDT/TSLAUSDT… are real Binance symbols now):
// routing them to xyz: failed their OI fetch every cycle and the
// colon-choke-point filter then excluded them entirely (2026-09-24). A
// natively-listed base is NOT an xyz asset — it takes the full Binance
// data path (OI/funding/klines) like any other perp. Unloaded cache →
// legacy behavior (fail-open).
func IsXyzDexAsset(symbol string) bool {
	base := strings.ToUpper(symbol)
	// Remove any prefix/suffix
	base = strings.TrimPrefix(base, "XYZ:")
	for _, suffix := range []string{"USDT", "USD", "-USDC"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	if !xyzDexAssets[base] {
		return false
	}
	return !binanceListsNative(base)
}

// Normalize normalizes symbol
// For crypto: ensures it's a USDT trading pair
// For xyz dex assets (stocks, forex, commodities): uses xyz: prefix without USDT suffix
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)

	// Check if this is an xyz dex asset
	if IsXyzDexAsset(symbol) {
		// Remove any xyz: prefix (case-insensitive) and USDT suffix, then add xyz: prefix
		base := symbol
		// Handle both lowercase and uppercase xyz: prefix
		if strings.HasPrefix(strings.ToLower(base), "xyz:") {
			base = base[4:] // Remove first 4 characters ("xyz:")
		}
		for _, suffix := range []string{"USDT", "USD", "-USDC"} {
			if strings.HasSuffix(base, suffix) {
				base = strings.TrimSuffix(base, suffix)
				break
			}
		}
		return "xyz:" + base
	}

	// Remove exchange-specific separators (Gate uses BTC_USDT, OKX uses BTC-USDT-SWAP)
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "-SWAP", "")
	symbol = strings.ReplaceAll(symbol, "-", "")

	// For regular crypto assets
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat parses float value
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// BuildDataFromKlines constructs market data snapshot from preloaded K-line series.
func BuildDataFromKlines(symbol string, primary []Kline, longer []Kline) (*Data, error) {
	if len(primary) == 0 {
		return nil, fmt.Errorf("primary series is empty")
	}

	symbol = Normalize(symbol)
	current := primary[len(primary)-1]
	currentPrice := current.Close

	data := &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		CurrentEMA20:      calculateEMA(primary, 20),
		CurrentMACD:       calculateMACD(primary),
		CurrentRSI7:       calculateRSI(primary, 7),
		PriceChange1h:     priceChangeFromSeries(primary, time.Hour),
		PriceChange4h:     priceChangeFromSeries(primary, 4*time.Hour),
		OpenInterest:      &OIData{Latest: 0, Average: 0},
		FundingRate:       0,
		IntradaySeries:    calculateIntradaySeries(primary),
		LongerTermContext: nil,
	}

	if len(longer) > 0 {
		data.LongerTermContext = calculateLongerTermData(longer)
	}

	return data, nil
}

func priceChangeFromSeries(series []Kline, duration time.Duration) float64 {
	if len(series) == 0 || duration <= 0 {
		return 0
	}
	last := series[len(series)-1]
	target := last.CloseTime - duration.Milliseconds()
	for i := len(series) - 1; i >= 0; i-- {
		if series[i].CloseTime <= target {
			price := series[i].Close
			if price > 0 {
				return ((last.Close - price) / price) * 100
			}
			break
		}
	}
	return 0
}

// isStaleData detects stale data (consecutive price freeze)
// Fix DOGEUSDT-style issue: consecutive N periods with completely unchanged prices indicate data source anomaly
func isStaleData(klines []Kline, symbol string) bool {
	if len(klines) < 5 {
		return false // Insufficient data to determine
	}

	// Detection threshold: 5 consecutive 3-minute periods with unchanged price (15 minutes without fluctuation)
	const stalePriceThreshold = 5
	const priceTolerancePct = 0.0001 // 0.01% fluctuation tolerance (avoid false positives)

	// Take the last stalePriceThreshold K-lines
	recentKlines := klines[len(klines)-stalePriceThreshold:]
	firstPrice := recentKlines[0].Close

	// Check if all prices are within tolerance
	for i := 1; i < len(recentKlines); i++ {
		priceDiff := math.Abs(recentKlines[i].Close-firstPrice) / firstPrice
		if priceDiff > priceTolerancePct {
			return false // Price fluctuation exists, data is normal
		}
	}

	// Additional check: MACD and volume
	// If price is unchanged but MACD/volume shows normal fluctuation, it might be a real market situation (extremely low volatility)
	// Check if volume is also 0 (data completely frozen)
	allVolumeZero := true
	for _, k := range recentKlines {
		if k.Volume > 0 {
			allVolumeZero = false
			break
		}
	}

	if allVolumeZero {
		logger.Infof("⚠️  %s stale data confirmed: price freeze + zero volume", symbol)
		return true
	}

	// Price frozen but has volume: might be extremely low volatility market, allow but log warning
	logger.Infof("⚠️  %s detected extreme price stability (no fluctuation for %d consecutive periods), but volume is normal", symbol, stalePriceThreshold)
	return false
}

// executionTimeframeData builds the TimeframeData map the execution-side
// risk gates read (3m/4h always fetched; 1h and 1d best-effort). Missing 1h
// uses the crypto fallback chain; missing 1d makes bstock entries fail closed.
// 15m is
// AGGREGATED locally from the 3m series (5:1, zero extra vendor calls):
// ComputeSymbolSignals treats 15m as the execution TF, and without this key
// ExecutionATRPct silently fell back to the longest available series (4h
// ATR ≈ 8× a quiet 15m's) — the supply-zone gate then rejected anchors the
// prompt-side suppression had blessed on real 15m data, every cycle
// (2026-09-19 BTCUSDT loop).
func executionTimeframeData(klines3m, klines1h, klines4h, klines1d []Kline) map[string]*TimeframeSeriesData {
	tf := map[string]*TimeframeSeriesData{}
	if len(klines3m) > 0 {
		tf["3m"] = calculateTimeframeSeries(klines3m, "3m", len(klines3m))
		// 100×3m bars ≈ 20×15m buckets — enough for ATR14 and the swing-pivot
		// scan; computeTFSignal drops the trailing forming bucket natively.
		if k15 := aggregateKlines(klines3m, 15*time.Minute); len(k15) > 0 {
			tf["15m"] = calculateTimeframeSeries(k15, "15m", len(k15))
		}
	}
	if len(klines1h) > 0 {
		tf["1h"] = calculateTimeframeSeries(klines1h, "1h", len(klines1h))
	}
	if len(klines4h) > 0 {
		tf["4h"] = calculateTimeframeSeries(klines4h, "4h", len(klines4h))
	}
	if len(klines1d) > 0 {
		tf["1d"] = calculateTimeframeSeries(klines1d, "1d", len(klines1d))
	}
	return tf
}

// aggregateKlines buckets klines into fixed-duration bars (OHLC extremes,
// volume summed). Input must be ascending by OpenTime; a partial bucket at
// either edge is kept — the trailing (usually forming) one is dropped
// downstream by the timeframe settlement logic, exactly like a native
// forming candle.
func aggregateKlines(src []Kline, bucket time.Duration) []Kline {
	if len(src) == 0 || bucket <= 0 {
		return nil
	}
	bucketMs := bucket.Milliseconds()
	out := make([]Kline, 0, len(src)/5+1)
	curBucket := int64(math.MinInt64)
	var cur *Kline
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, k := range src {
		b := k.OpenTime / bucketMs * bucketMs
		if b != curBucket {
			flush()
			curBucket = b
			cur = &Kline{
				OpenTime: b, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
				Volume: k.Volume, CloseTime: b + bucketMs - 1,
				QuoteVolume: k.QuoteVolume,
			}
			continue
		}
		cur.High = math.Max(cur.High, k.High)
		cur.Low = math.Min(cur.Low, k.Low)
		cur.Close = k.Close
		cur.Volume += k.Volume
		cur.QuoteVolume += k.QuoteVolume
	}
	flush()
	return out
}
