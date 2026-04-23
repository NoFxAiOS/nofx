package market

import (
	"math"
	"nofx/logger"
	"sort"
	"strconv"
	"strings"
)

// HotCoin represents a hot coin with composite scoring
type HotCoin struct {
	Symbol          string  `json:"symbol"`
	QuoteVolume24h  float64 `json:"quote_volume_24h"`
	PriceChangePct  float64 `json:"price_change_pct"`
	OpenInterestUSD float64 `json:"open_interest_usd"`
	FundingRate     float64 `json:"funding_rate"`
	HotScore        float64 `json:"hot_score"`
	Source          string  `json:"source"`
}

const (
	hotCoinMinVolume   = 50_000_000 // 50M USDT
	hotCoinMinOI       = 15_000_000 // 15M USDT
	hotCoinMaxPriceChg = 30.0       // 30% max abs price change
)

// GetHotCoins returns top hot coins by composite score (auto-selects exchange)
func GetHotCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return GetHotCoinsWithExchange(limit, excludedCoins, "okx")
}

// GetHotCoinsWithExchange returns hot coins from specified exchange
func GetHotCoinsWithExchange(limit int, excludedCoins []string, exchange string) ([]HotCoin, error) {
	switch strings.ToLower(exchange) {
	case "okx":
		return getHotCoinsOKX(limit, excludedCoins)
	case "binance":
		return getHotCoinsBinance(limit, excludedCoins)
	default:
		return getHotCoinsOKX(limit, excludedCoins)
	}
}

// GetOITopCoins returns coins ranked by OI increase
func GetOITopCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return getOIRankedCoinsOKX(limit, excludedCoins, true)
}

// GetOILowCoins returns coins ranked by OI decrease
func GetOILowCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return getOIRankedCoinsOKX(limit, excludedCoins, false)
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
		last   float64
	}
	var raws []raw
	var maxVol, maxOI, maxChg float64

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
		if volUSD < hotCoinMinVolume {
			continue
		}

		var chg float64
		if open24h > 0 {
			chg = (last - open24h) / open24h * 100
		}
		if math.Abs(chg) > hotCoinMaxPriceChg {
			continue
		}

		// Get OI
		oiData, err := okx.GetOpenInterest(stdSymbol)
		if err != nil || oiData == nil {
			continue
		}
		oiUSD := oiData.Latest * last
		if oiUSD < hotCoinMinOI {
			continue
		}

		r := raw{symbol: stdSymbol, vol: volUSD, chg: math.Abs(chg), oi: oiUSD, last: last}
		raws = append(raws, r)

		if r.vol > maxVol {
			maxVol = r.vol
		}
		if r.oi > maxOI {
			maxOI = r.oi
		}
		if r.chg > maxChg {
			maxChg = r.chg
		}
	}

	var candidates []HotCoin
	for _, r := range raws {
		normVol := safeNorm(r.vol, maxVol)
		normOI := safeNorm(r.oi, maxOI)
		normChg := safeNorm(r.chg, maxChg)
		score := 0.4*normVol + 0.3*normOI + 0.3*normChg

		candidates = append(candidates, HotCoin{
			Symbol:          r.symbol,
			QuoteVolume24h:  r.vol,
			PriceChangePct:  r.chg,
			OpenInterestUSD: r.oi,
			HotScore:        score,
			Source:          "okx_hot",
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].HotScore > candidates[j].HotScore
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	logger.Infof("GetHotCoinsOKX: found %d coins", len(candidates))
	return candidates, nil
}

func getOIRankedCoinsOKX(limit int, excludedCoins []string, ascending bool) ([]HotCoin, error) {
	okx := NewOKXAPIClient()
	excluded := toExcludeMap(excludedCoins)

	tickers, err := okx.GetAllSwapTickers()
	if err != nil {
		return nil, err
	}

	var coins []HotCoin

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

		coins = append(coins, HotCoin{
			Symbol:          stdSymbol,
			QuoteVolume24h:  volUSD,
			PriceChangePct:  chg,
			OpenInterestUSD: oiUSD,
			HotScore:        oiChange,
			Source:          "okx_oi_rank",
		})
	}

	if ascending {
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
	}
	var raws []raw
	var maxVol, maxOI, maxChg float64

	for _, t := range tickers {
		if !strings.HasSuffix(t.Symbol, "USDT") {
			continue
		}
		if excluded[t.Symbol] {
			continue
		}
		vol, _ := strconv.ParseFloat(t.QuoteVolume, 64)
		chg, _ := strconv.ParseFloat(t.PriceChangePercent, 64)

		if vol < hotCoinMinVolume {
			continue
		}
		if math.Abs(chg) > hotCoinMaxPriceChg {
			continue
		}

		oiData, err := getOpenInterestData(t.Symbol)
		if err != nil || oiData == nil {
			continue
		}
		price, _ := strconv.ParseFloat(t.WeightedAvgPrice, 64)
		oiUSD := oiData.Latest * price
		if oiUSD < hotCoinMinOI {
			continue
		}

		r := raw{symbol: t.Symbol, vol: vol, chg: math.Abs(chg), oi: oiUSD}
		raws = append(raws, r)

		if r.vol > maxVol {
			maxVol = r.vol
		}
		if r.oi > maxOI {
			maxOI = r.oi
		}
		if r.chg > maxChg {
			maxChg = r.chg
		}
	}

	var candidates []HotCoin
	for _, r := range raws {
		normVol := safeNorm(r.vol, maxVol)
		normOI := safeNorm(r.oi, maxOI)
		normChg := safeNorm(r.chg, maxChg)
		score := 0.4*normVol + 0.3*normOI + 0.3*normChg

		candidates = append(candidates, HotCoin{
			Symbol:          r.symbol,
			QuoteVolume24h:  r.vol,
			PriceChangePct:  r.chg,
			OpenInterestUSD: r.oi,
			HotScore:        score,
			Source:          "binance_hot",
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
