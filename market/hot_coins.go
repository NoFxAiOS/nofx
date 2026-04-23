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
	HotScore        float64 `json:"hot_score"`
	Source          string  `json:"source"`
}

const (
	hotCoinMinVolume    = 50_000_000  // 50M USDT
	hotCoinMinOI        = 15_000_000  // 15M USDT
	hotCoinMaxPriceChg  = 30.0        // 30% max abs price change
)

// GetHotCoins returns top hot coins by composite score
func GetHotCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return GetHotCoinsWithExchange(limit, excludedCoins, "binance")
}

// GetHotCoinsWithExchange returns hot coins from specified exchange
func GetHotCoinsWithExchange(limit int, excludedCoins []string, exchange string) ([]HotCoin, error) {
	client := NewAPIClient()
	excluded := toExcludeMap(excludedCoins)

	tickers, err := client.GetAllTickers24h()
	if err != nil {
		return nil, err
	}

	var candidates []HotCoin
	var maxVol, maxOI, maxChg float64

	// First pass: filter and collect stats for normalization
	type raw struct {
		symbol string
		vol    float64
		chg    float64
		oi     float64
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

		if vol < hotCoinMinVolume {
			continue
		}
		if math.Abs(chg) > hotCoinMaxPriceChg {
			continue
		}

		// Get OI
		oiData, err := getOpenInterestData(t.Symbol)
		if err != nil || oiData == nil {
			continue
		}
		// OI is in contract units; approximate USDT value using current price
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

	// Second pass: score
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
			Source:          "exchange_hot",
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

// GetOITopCoins returns coins ranked by OI increase (4h comparison)
func GetOITopCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return getOIRankedCoins(limit, excludedCoins, true)
}

// GetOILowCoins returns coins ranked by OI decrease (4h comparison)
func GetOILowCoins(limit int, excludedCoins []string) ([]HotCoin, error) {
	return getOIRankedCoins(limit, excludedCoins, false)
}

func getOIRankedCoins(limit int, excludedCoins []string, ascending bool) ([]HotCoin, error) {
	client := NewAPIClient()
	excluded := toExcludeMap(excludedCoins)

	tickers, err := client.GetAllTickers24h()
	if err != nil {
		return nil, err
	}

	var coins []HotCoin

	for _, t := range tickers {
		if !strings.HasSuffix(t.Symbol, "USDT") {
			continue
		}
		if excluded[t.Symbol] {
			continue
		}
		vol, _ := strconv.ParseFloat(t.QuoteVolume, 64)
		if vol < hotCoinMinVolume {
			continue
		}

		// Get OI history (4h period, 2 data points for comparison)
		hist, err := client.GetOpenInterestHist(t.Symbol, "4h", 2)
		if err != nil || len(hist) < 2 {
			continue
		}

		oiOld, _ := strconv.ParseFloat(hist[0].SumOpenInterestValue, 64)
		oiNew, _ := strconv.ParseFloat(hist[1].SumOpenInterestValue, 64)
		if oiOld == 0 {
			continue
		}

		oiChange := (oiNew - oiOld) / oiOld * 100
		chg, _ := strconv.ParseFloat(t.PriceChangePercent, 64)

		coins = append(coins, HotCoin{
			Symbol:          t.Symbol,
			QuoteVolume24h:  vol,
			PriceChangePct:  chg,
			OpenInterestUSD: oiNew,
			HotScore:        oiChange, // Use OI change % as score
			Source:          "oi_rank",
		})
	}

	if ascending {
		// Top OI increase
		sort.Slice(coins, func(i, j int) bool {
			return coins[i].HotScore > coins[j].HotScore
		})
	} else {
		// Top OI decrease
		sort.Slice(coins, func(i, j int) bool {
			return coins[i].HotScore < coins[j].HotScore
		})
	}

	if limit > 0 && len(coins) > limit {
		coins = coins[:limit]
	}

	logger.Infof("GetOIRankedCoins: found %d coins (ascending=%v)", len(coins), ascending)
	return coins, nil
}

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
