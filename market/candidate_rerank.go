package market

import "math"

func RerankHotCoins(coins []HotCoin) []HotCoin {
	if len(coins) == 0 {
		return coins
	}
	inputs := make([]candidateInput, len(coins))
	for i, c := range coins {
		activity := 0.0
		if c.OpenInterestUSD > 0 {
			activity = c.QuoteVolume24h / c.OpenInterestUSD * 100
		}
		inputs[i] = candidateInput{
			symbol:      c.Symbol,
			volumeUSD:   c.QuoteVolume24h,
			oiUSD:       c.OpenInterestUSD,
			absChgPct:   math.Abs(c.PriceChangePct),
			activity:    activity,
			oiGrowthPct: math.NaN(),
			fundingRate: fundingOrNaN(c.FundingRate),
		}
	}
	qualities := scoreCandidatesPercentile(inputs)
	out := make([]HotCoin, 0, len(coins))
	for i, c := range coins {
		q := qualities[i]
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
	if len(coins) == 0 {
		return coins
	}
	activity := make([]float64, len(coins))
	inputs := make([]candidateInput, len(coins))
	for i, c := range coins {
		if c.OpenInterestUSD > 0 {
			activity[i] = c.QuoteVolume24h / c.OpenInterestUSD * 100
		}
		inputs[i] = candidateInput{
			symbol:      c.Symbol,
			volumeUSD:   c.QuoteVolume24h,
			oiUSD:       c.OpenInterestUSD,
			absChgPct:   math.Abs(c.PriceChangePct),
			activity:    activity[i],
			oiGrowthPct: math.NaN(),
			fundingRate: fundingOrNaN(c.FundingRate),
		}
	}

	// Compute maxActivity for the OI rank composite.
	var maxActivity float64
	for _, a := range activity {
		if a > maxActivity {
			maxActivity = a
		}
	}

	direction := 1.0
	if !top {
		direction = -1
	}

	qualities := scoreCandidatesPercentile(inputs)
	out := make([]HotCoin, 0, len(coins))
	for i, c := range coins {
		q := qualities[i]
		if !q.Passed {
			continue
		}
		c.Quality = q
		c.HotScore = compositeOIRankScore(q, activity[i], maxActivity, direction)
		out = append(out, c)
	}
	return out
}

// fundingOrNaN returns math.NaN() when the funding rate field is zero (unset),
// otherwise returns the raw value. This avoids treating "not fetched" as 0% funding.
func fundingOrNaN(rate float64) float64 {
	if rate == 0 {
		return math.NaN()
	}
	return rate
}
