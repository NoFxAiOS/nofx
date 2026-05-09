package market

import (
	"math"
	"nofx/logger"
	"sort"
	"strconv"
	"strings"
	"time"
)

// HotCoin represents a hot coin with composite scoring
type HotCoin struct {
	Symbol                string           `json:"symbol"`
	QuoteVolume24h        float64          `json:"quote_volume_24h"`
	PriceChangePct        float64          `json:"price_change_pct"`
	OpenInterestUSD       float64          `json:"open_interest_usd"`
	OpenInterestChangePct float64          `json:"open_interest_change_pct,omitempty"`
	OpenInterestWindowSec int              `json:"open_interest_window_sec,omitempty"`
	OpenInterestSource    string           `json:"open_interest_source,omitempty"`
	FundingRate           float64          `json:"funding_rate"`
	HotScore              float64          `json:"hot_score"`
	Source                string           `json:"source"`
	Quality               CandidateQuality `json:"quality,omitempty"`
}

const (
	hotCoinMinVolume   = 50_000_000 // 50M USDT (tier-1 threshold)
	hotCoinMinOI       = 15_000_000 // 15M USDT (tier-1 threshold)
	hotCoinMaxPriceChg = 30.0       // 30% max abs price change

	// Second-tier thresholds: lower absolute bars but require composite percentile > 0.7.
	hotCoinTier2MinVolume    = 20_000_000 // 20M USDT
	hotCoinTier2MinOI        = 8_000_000  // 8M USDT
	hotCoinTier2MinComposite = 0.70       // minimum composite score to qualify
)

// GetHotCoins returns top hot coins by composite score (auto-selects exchange)
func GetHotCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return GetHotCoinsWithExchange(limit, excludedCoins, "okx")
}

// GetHotCoinsWithExchange returns hot coins from specified exchange
func GetHotCoinsWithExchange(limit int, excludedCoins []string, exchange string) ([]HotCoin, error) {
	return cachedHotCoinList(hotCoinCacheKey("hot", limit, excludedCoins, exchange), 180*time.Second, func() ([]HotCoin, error) {
		switch strings.ToLower(exchange) {
		case "okx":
			return getHotCoinsOKX(limit, excludedCoins)
		case "binance":
			return getHotCoinsBinance(limit, excludedCoins)
		default:
			return getHotCoinsOKX(limit, excludedCoins)
		}
	})
}

// GetOITopCoins returns coins ranked by OI increase (defaults to OKX)
func GetOITopCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return GetOITopCoinsWithExchange(limit, excludedCoins, "okx")
}

// GetOITopCoinsWithExchange returns coins ranked by OI increase from specified exchange
func GetOITopCoinsWithExchange(limit int, excludedCoins []string, exchange string) ([]HotCoin, error) {
	return cachedHotCoinList(hotCoinCacheKey("oi_top", limit, excludedCoins, exchange), 180*time.Second, func() ([]HotCoin, error) {
		switch strings.ToLower(exchange) {
		case "binance":
			return getOIRankedCoinsBinance(limit, excludedCoins, true)
		default:
			return getOIRankedCoinsOKX(limit, excludedCoins, true)
		}
	})
}

// GetOILowCoins returns coins ranked by OI decrease (defaults to OKX)
func GetOILowCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return GetOILowCoinsWithExchange(limit, excludedCoins, "okx")
}

// GetOILowCoinsWithExchange returns coins ranked by OI decrease from specified exchange
func GetOILowCoinsWithExchange(limit int, excludedCoins []string, exchange string) ([]HotCoin, error) {
	return cachedHotCoinList(hotCoinCacheKey("oi_low", limit, excludedCoins, exchange), 180*time.Second, func() ([]HotCoin, error) {
		switch strings.ToLower(exchange) {
		case "binance":
			return getOIRankedCoinsBinance(limit, excludedCoins, false)
		default:
			return getOIRankedCoinsOKX(limit, excludedCoins, false)
		}
	})
}

// ---- OKX implementation ----

func getHotCoinsOKX(limit int, excludedCoins []string) ([]HotCoin, error) {
	okx := NewOKXAPIClient()
	excluded := toExcludeMap(excludedCoins)

	tickers, err := okx.GetAllSwapTickers()
	if err != nil {
		return nil, err
	}

	type raw struct {
		symbol string
		vol    float64
		chg    float64
		oi     float64
		tier2  bool // true when only second-tier threshold is met
	}
	var raws []raw

	for _, t := range tickers {
		if !strings.HasSuffix(t.InstID, "-USDT-SWAP") {
			continue
		}
		// Convert OKX symbol to standard: BTC-USDT-SWAP → BTCUSDT
		stdSymbol := okxToBinanceSymbol(t.InstID)
		if excluded[stdSymbol] {
			continue
		}

		last, _ := strconv.ParseFloat(t.Last, 64)
		open24h, _ := strconv.ParseFloat(t.Open24h, 64)
		volCcy, _ := strconv.ParseFloat(t.VolCcy24h, 64)

		// volCcy24h is in base currency; convert to USDT
		volUSD := volCcy * last

		var chg float64
		if open24h > 0 {
			chg = (last - open24h) / open24h * 100
		}
		if math.Abs(chg) > hotCoinMaxPriceChg {
			continue
		}

		// Reject coins below tier-2 (lowest) volume floor early.
		if volUSD < hotCoinTier2MinVolume {
			continue
		}

		// Get OI
		oiData, err := okx.GetOpenInterest(stdSymbol)
		if err != nil || oiData == nil {
			continue
		}
		oiUSD := oiData.Latest * last
		if oiUSD < hotCoinTier2MinOI {
			continue
		}

		isTier2 := volUSD < hotCoinMinVolume || oiUSD < hotCoinMinOI
		raws = append(raws, raw{symbol: stdSymbol, vol: volUSD, chg: math.Abs(chg), oi: oiUSD, tier2: isTier2})
	}

	// Build candidateInput slice for batch percentile scoring.
	inputs := make([]candidateInput, len(raws))
	for i, r := range raws {
		activity := 0.0
		if r.oi > 0 {
			activity = r.vol / r.oi * 100
		}
		inputs[i] = candidateInput{
			symbol:      r.symbol,
			volumeUSD:   r.vol,
			oiUSD:       r.oi,
			absChgPct:   r.chg,
			activity:    activity,
			oiGrowthPct: math.NaN(), // not available at this stage
			fundingRate: math.NaN(), // optional; skip per-coin API calls to keep batch fast
		}
	}

	qualities := scoreCandidatesPercentile(inputs)

	var candidates []HotCoin
	for i, r := range raws {
		q := qualities[i]
		if !q.Passed {
			continue
		}
		composite := compositeHotScore(q)

		// Tier-2 coins must clear the composite threshold.
		if r.tier2 && composite < hotCoinTier2MinComposite {
			continue
		}

		logger.Infof("%s", qualityLogLine(r.symbol, q, composite))

		candidates = append(candidates, HotCoin{
			Symbol:          r.symbol,
			QuoteVolume24h:  r.vol,
			PriceChangePct:  r.chg,
			OpenInterestUSD: r.oi,
			HotScore:        composite,
			Source:          "okx_hot",
			Quality:         q,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].HotScore > candidates[j].HotScore
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	logger.Infof("GetHotCoinsOKX: found %d coins (%d raw candidates)", len(candidates), len(raws))
	return candidates, nil
}

func getOIRankedCoinsOKX(limit int, excludedCoins []string, ascending bool) ([]HotCoin, error) {
	okx := NewOKXAPIClient()
	excluded := toExcludeMap(excludedCoins)

	tickers, err := okx.GetAllSwapTickers()
	if err != nil {
		return nil, err
	}

	var rawCoins []HotCoin

	for _, t := range tickers {
		if !strings.HasSuffix(t.InstID, "-USDT-SWAP") {
			continue
		}
		stdSymbol := okxToBinanceSymbol(t.InstID)
		if excluded[stdSymbol] {
			continue
		}

		last, _ := strconv.ParseFloat(t.Last, 64)
		volCcy, _ := strconv.ParseFloat(t.VolCcy24h, 64)
		volUSD := volCcy * last
		if volUSD < hotCoinMinVolume*0.5 { // lower threshold for OI ranking
			continue
		}

		open24h, _ := strconv.ParseFloat(t.Open24h, 64)
		var chg float64
		if open24h > 0 {
			chg = (last - open24h) / open24h * 100
		}

		// Get current OI
		oiData, err := okx.GetOpenInterest(stdSymbol)
		if err != nil || oiData == nil || oiData.Latest == 0 {
			continue
		}
		oiUSD := oiData.Latest * last

		// For OI change, use volume/OI ratio as activity proxy
		oiChange := 0.0
		if oiUSD > 0 {
			oiChange = volUSD / oiUSD * 100
		}

		rawCoins = append(rawCoins, HotCoin{
			Symbol:          stdSymbol,
			QuoteVolume24h:  volUSD,
			PriceChangePct:  chg,
			OpenInterestUSD: oiUSD,
			HotScore:        oiChange,
			Source:          "okx_oi_rank",
		})
	}

	coins := RerankOICoins(rawCoins, ascending)
	if deltaCoins, ok := computeOIDeltaScores("okx", coins, ascending); ok {
		coins = deltaCoins
	} else if ascending {
		sort.Slice(coins, func(i, j int) bool {
			return coins[i].HotScore > coins[j].HotScore
		})
	} else {
		sort.Slice(coins, func(i, j int) bool {
			return coins[i].HotScore < coins[j].HotScore
		})
	}

	if limit > 0 && len(coins) > limit {
		coins = coins[:limit]
	}

	logger.Infof("GetOIRankedCoinsOKX: found %d coins (ascending=%v)", len(coins), ascending)
	return coins, nil
}

// ---- Binance implementation (fallback, may be geo-restricted) ----

func getOIRankedCoinsBinance(limit int, excludedCoins []string, ascending bool) ([]HotCoin, error) {
	client := NewAPIClient()
	excluded := toExcludeMap(excludedCoins)

	tickers, err := client.GetAllTickers24h()
	if err != nil {
		return nil, err
	}

	var rawCoins []HotCoin

	for _, t := range tickers {
		if !strings.HasSuffix(t.Symbol, "USDT") {
			continue
		}
		if excluded[t.Symbol] {
			continue
		}

		vol, _ := strconv.ParseFloat(t.QuoteVolume, 64)
		if vol < hotCoinMinVolume*0.5 {
			continue
		}

		chg, _ := strconv.ParseFloat(t.PriceChangePercent, 64)
		price, _ := strconv.ParseFloat(t.WeightedAvgPrice, 64)

		oiData, err := getOpenInterestData(t.Symbol)
		if err != nil || oiData == nil || oiData.Latest == 0 {
			continue
		}
		oiUSD := oiData.Latest * price

		oiChange := 0.0
		if oiUSD > 0 {
			oiChange = vol / oiUSD * 100
		}

		rawCoins = append(rawCoins, HotCoin{
			Symbol:          t.Symbol,
			QuoteVolume24h:  vol,
			PriceChangePct:  chg,
			OpenInterestUSD: oiUSD,
			HotScore:        oiChange,
			Source:          "binance_oi_rank",
		})
	}

	coins := RerankOICoins(rawCoins, ascending)
	if deltaCoins, ok := computeOIDeltaScores("binance", coins, ascending); ok {
		coins = deltaCoins
	} else if ascending {
		sort.Slice(coins, func(i, j int) bool {
			return coins[i].HotScore > coins[j].HotScore
		})
	} else {
		sort.Slice(coins, func(i, j int) bool {
			return coins[i].HotScore < coins[j].HotScore
		})
	}

	if limit > 0 && len(coins) > limit {
		coins = coins[:limit]
	}

	logger.Infof("getOIRankedCoinsBinance: found %d coins (ascending=%v)", len(coins), ascending)
	return coins, nil
}

func getHotCoinsBinance(limit int, excludedCoins []string) ([]HotCoin, error) {
	client := NewAPIClient()
	excluded := toExcludeMap(excludedCoins)

	tickers, err := client.GetAllTickers24h()
	if err != nil {
		return nil, err
	}

	type raw struct {
		symbol string
		vol    float64
		chg    float64
		oi     float64
		tier2  bool
	}
	var raws []raw

	for _, t := range tickers {
		if !strings.HasSuffix(t.Symbol, "USDT") {
			continue
		}
		if excluded[t.Symbol] {
			continue
		}
		vol, _ := strconv.ParseFloat(t.QuoteVolume, 64)
		chg, _ := strconv.ParseFloat(t.PriceChangePercent, 64)

		if math.Abs(chg) > hotCoinMaxPriceChg {
			continue
		}
		if vol < hotCoinTier2MinVolume {
			continue
		}

		oiData, err := getOpenInterestData(t.Symbol)
		if err != nil || oiData == nil {
			continue
		}
		price, _ := strconv.ParseFloat(t.WeightedAvgPrice, 64)
		oiUSD := oiData.Latest * price
		if oiUSD < hotCoinTier2MinOI {
			continue
		}

		isTier2 := vol < hotCoinMinVolume || oiUSD < hotCoinMinOI
		raws = append(raws, raw{symbol: t.Symbol, vol: vol, chg: math.Abs(chg), oi: oiUSD, tier2: isTier2})
	}

	// Batch percentile scoring.
	inputs := make([]candidateInput, len(raws))
	for i, r := range raws {
		activity := 0.0
		if r.oi > 0 {
			activity = r.vol / r.oi * 100
		}
		inputs[i] = candidateInput{
			symbol:      r.symbol,
			volumeUSD:   r.vol,
			oiUSD:       r.oi,
			absChgPct:   r.chg,
			activity:    activity,
			oiGrowthPct: math.NaN(),
			fundingRate: math.NaN(),
		}
	}

	qualities := scoreCandidatesPercentile(inputs)

	var candidates []HotCoin
	for i, r := range raws {
		q := qualities[i]
		if !q.Passed {
			continue
		}
		composite := compositeHotScore(q)
		if r.tier2 && composite < hotCoinTier2MinComposite {
			continue
		}

		logger.Infof("%s", qualityLogLine(r.symbol, q, composite))

		candidates = append(candidates, HotCoin{
			Symbol:          r.symbol,
			QuoteVolume24h:  r.vol,
			PriceChangePct:  r.chg,
			OpenInterestUSD: r.oi,
			HotScore:        composite,
			Source:          "binance_hot",
			Quality:         q,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].HotScore > candidates[j].HotScore
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

// ---- Helpers ----

func safeNorm(val, max float64) float64 {
	if max == 0 {
		return 0
	}
	return val / max
}

func toExcludeMap(coins []string) map[string]bool {
	m := make(map[string]bool, len(coins))
	for _, c := range coins {
		m[strings.ToUpper(c)] = true
	}
	return m
}
