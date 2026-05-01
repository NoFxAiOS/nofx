package market

func RerankHotCoins(coins []HotCoin) []HotCoin {
	var maxVol, maxOI, maxChg, maxActivity float64
	activity := make([]float64, len(coins))
	for i, c := range coins {
		if c.QuoteVolume24h > maxVol {
			maxVol = c.QuoteVolume24h
		}
		if c.OpenInterestUSD > maxOI {
			maxOI = c.OpenInterestUSD
		}
		chg := c.PriceChangePct
		if chg < 0 {
			chg = -chg
		}
		if chg > maxChg {
			maxChg = chg
		}
		if c.OpenInterestUSD > 0 {
			activity[i] = c.QuoteVolume24h / c.OpenInterestUSD * 100
		}
		if activity[i] > maxActivity {
			maxActivity = activity[i]
		}
	}
	out := make([]HotCoin, 0, len(coins))
	for i, c := range coins {
		q := scoreCandidateQuality(c.QuoteVolume24h, c.OpenInterestUSD, c.PriceChangePct, activity[i], maxVol, maxOI, maxChg, maxActivity)
		if !q.Passed {
			continue
		}
		c.Quality = q
		c.HotScore = compositeHotScore(q)
		out = append(out, c)
	}
	return out
}

func RerankOICoins(coins []HotCoin, top bool) []HotCoin {
	var maxVol, maxOI, maxChg, maxActivity float64
	activity := make([]float64, len(coins))
	for i, c := range coins {
		if c.QuoteVolume24h > maxVol {
			maxVol = c.QuoteVolume24h
		}
		if c.OpenInterestUSD > maxOI {
			maxOI = c.OpenInterestUSD
		}
		chg := c.PriceChangePct
		if chg < 0 {
			chg = -chg
		}
		if chg > maxChg {
			maxChg = chg
		}
		if c.OpenInterestUSD > 0 {
			activity[i] = c.QuoteVolume24h / c.OpenInterestUSD * 100
		}
		if activity[i] > maxActivity {
			maxActivity = activity[i]
		}
	}
	direction := 1.0
	if !top {
		direction = -1
	}
	out := make([]HotCoin, 0, len(coins))
	for i, c := range coins {
		q := scoreCandidateQuality(c.QuoteVolume24h, c.OpenInterestUSD, c.PriceChangePct, activity[i], maxVol, maxOI, maxChg, maxActivity)
		if !q.Passed {
			continue
		}
		c.Quality = q
		c.HotScore = compositeOIRankScore(q, activity[i], maxActivity, direction)
		out = append(out, c)
	}
	return out
}
