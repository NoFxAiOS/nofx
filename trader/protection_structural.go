package trader

import (
	"math"
	"nofx/market"
	"sort"
)

// generateStructuralLadderRules creates ladder protection rules from structural levels.
// For longs: TP tiers at resistance levels above entry, SL at support below entry.
// For shorts: TP tiers at support levels below entry, SL at resistance above entry.
// Uses fibonacci levels as secondary anchors if not enough swing-based levels.
func generateStructuralLadderRules(entryPrice float64, isLong bool, mdata *market.Data) (tpOrders, slOrders []ProtectionOrder) {
	if mdata == nil || entryPrice <= 0 {
		return nil, nil
	}

	var tpLevels, slLevels []float64

	// Collect structural levels
	for _, sl := range mdata.StructuralLevels {
		if isLong {
			if sl.Type == "resistance" && sl.Price > entryPrice*1.001 {
				tpLevels = append(tpLevels, sl.Price)
			} else if sl.Type == "support" && sl.Price < entryPrice*0.999 {
				slLevels = append(slLevels, sl.Price)
			}
		} else {
			if sl.Type == "support" && sl.Price < entryPrice*0.999 {
				tpLevels = append(tpLevels, sl.Price)
			} else if sl.Type == "resistance" && sl.Price > entryPrice*1.001 {
				slLevels = append(slLevels, sl.Price)
			}
		}
	}

	// Use fibonacci levels as secondary anchors
	if mdata.FibonacciLevels != nil && len(tpLevels) < 2 {
		for _, price := range mdata.FibonacciLevels.Levels {
			if isLong && price > entryPrice*1.001 {
				tpLevels = append(tpLevels, price)
			} else if !isLong && price < entryPrice*0.999 {
				tpLevels = append(tpLevels, price)
			}
		}
	}
	if mdata.FibonacciLevels != nil && len(slLevels) < 1 {
		for _, price := range mdata.FibonacciLevels.Levels {
			if isLong && price < entryPrice*0.999 {
				slLevels = append(slLevels, price)
			} else if !isLong && price > entryPrice*1.001 {
				slLevels = append(slLevels, price)
			}
		}
	}

	if len(tpLevels) == 0 {
		return nil, nil
	}

	// Sort TP levels: for longs ascending (nearest first), for shorts descending (nearest first)
	if isLong {
		sort.Float64s(tpLevels)
	} else {
		sort.Sort(sort.Reverse(sort.Float64Slice(tpLevels)))
	}

	// Sort SL levels: for longs descending (nearest first), for shorts ascending (nearest first)
	if isLong {
		sort.Sort(sort.Reverse(sort.Float64Slice(slLevels)))
	} else {
		sort.Float64s(slLevels)
	}

	// Deduplicate close levels (within 0.2%)
	tpLevels = deduplicateLevels(tpLevels, 0.002)
	slLevels = deduplicateLevels(slLevels, 0.002)

	// Cap at 3 TP tiers
	if len(tpLevels) > 3 {
		tpLevels = tpLevels[:3]
	}

	// Generate TP orders with proportional sizing
	ratios := ladderCloseRatios(len(tpLevels))
	for i, price := range tpLevels {
		price := applyStructuralProtectionBuffer(entryPrice, price, isLong, true, mdata)
		tpOrders = append(tpOrders, ProtectionOrder{
			Price:         roundProtectionPrice(price),
			CloseRatioPct: ratios[i],
		})
	}

	// Generate SL ladder around the nearest support/resistance. Keep the same primary
	// structure, but use distinct inside/near and outside/beyond buffers so multiple
	// ladder SL legs are not just the same stop split into several quantities.
	if len(slLevels) > 0 {
		base := slLevels[0]
		for _, leg := range structuralLadderStopLegs(entryPrice, base, isLong, mdata) {
			slOrders = append(slOrders, leg)
		}
	}

	return tpOrders, slOrders
}

func structuralLadderStopLegs(entryPrice, base float64, isLong bool, mdata *market.Data) []ProtectionOrder {
	if entryPrice <= 0 || base <= 0 {
		return nil
	}
	atr := structuralATR(mdata)
	if atr <= 0 {
		atr = entryPrice * 0.003
	}
	insideBuffer := atr * 0.20
	outsideBuffer := atr * 0.55
	farBuffer := atr * 1.20
	prices := make([]float64, 0, 3)
	if isLong {
		prices = append(prices, base-insideBuffer, base-outsideBuffer, base-farBuffer)
	} else {
		prices = append(prices, base+insideBuffer, base+outsideBuffer, base+farBuffer)
	}
	ratios := []float64{35, 40, 25}
	out := make([]ProtectionOrder, 0, len(prices))
	for i, price := range prices {
		price = roundProtectionPrice(price)
		if price <= 0 {
			continue
		}
		if isLong && price >= entryPrice {
			continue
		}
		if !isLong && price <= entryPrice {
			continue
		}
		if len(out) > 0 && approximatelyEqualPrice(out[len(out)-1].Price, price) {
			continue
		}
		out = append(out, ProtectionOrder{Price: price, CloseRatioPct: ratios[i]})
	}
	if len(out) == 0 {
		return nil
	}
	if len(out) == 1 {
		out[0].CloseRatioPct = 100
	}
	return out
}

func structuralATR(mdata *market.Data) float64 {
	if mdata == nil {
		return 0
	}
	if mdata.TimeframeData != nil {
		if tf := mdata.TimeframeData["15m"]; tf != nil && tf.ATR14 > 0 {
			return tf.ATR14
		}
	}
	if mdata.IntradaySeries != nil && mdata.IntradaySeries.ATR14 > 0 {
		return mdata.IntradaySeries.ATR14
	}
	return 0
}

func applyStructuralProtectionBuffer(entryPrice, price float64, isLong bool, takeProfit bool, mdata *market.Data) float64 {
	if entryPrice <= 0 || price <= 0 || mdata == nil {
		return price
	}
	atr := structuralATR(mdata)
	if atr <= 0 {
		return price
	}
	buffer := atr * 0.35
	if takeProfit {
		if isLong {
			return price - buffer
		}
		return price + buffer
	}
	if isLong {
		return price - buffer
	}
	return price + buffer
}

func ladderCloseRatios(n int) []float64 {
	switch n {
	case 1:
		return []float64{100}
	case 2:
		return []float64{50, 50}
	case 3:
		return []float64{30, 40, 30}
	default:
		return []float64{100}
	}
}

func deduplicateLevels(levels []float64, tolerancePct float64) []float64 {
	if len(levels) <= 1 {
		return levels
	}
	result := []float64{levels[0]}
	for i := 1; i < len(levels); i++ {
		if math.Abs(levels[i]-result[len(result)-1])/result[len(result)-1] > tolerancePct {
			result = append(result, levels[i])
		}
	}
	return result
}
