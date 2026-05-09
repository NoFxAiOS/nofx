package trader

import (
	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

const marketContextV2ReviewLimit = 12

func (at *AutoTrader) attachMarketContextV2Review(record *store.DecisionRecord, ctx *kernel.Context) {
	if record == nil || ctx == nil || len(ctx.MarketDataMap) == 0 {
		return
	}
	if record.ReviewContext == nil {
		record.ReviewContext = map[string]interface{}{}
	}

	expectedTFs := ctx.Timeframes
	if len(expectedTFs) == 0 && at != nil && at.config.StrategyConfig != nil {
		expectedTFs = at.config.StrategyConfig.Indicators.Klines.SelectedTimeframes
	}
	primaryTF := ""
	if at != nil && at.config.StrategyConfig != nil {
		primaryTF = at.config.StrategyConfig.Indicators.Klines.PrimaryTimeframe
	}
	if primaryTF == "" && len(expectedTFs) > 0 {
		primaryTF = expectedTFs[0]
	}

	contexts := make(map[string]*market.MarketContextV2, len(ctx.MarketDataMap))
	count := 0
	for symbol, data := range ctx.MarketDataMap {
		if count >= marketContextV2ReviewLimit {
			break
		}
		contexts[symbol] = market.BuildMarketContextV2(symbol, data, expectedTFs, primaryTF)
		count++
	}
	if len(contexts) > 0 {
		record.ReviewContext["market_context_v2"] = contexts
		record.ReviewContext["market_context_v2_record_only"] = true
	}
}
