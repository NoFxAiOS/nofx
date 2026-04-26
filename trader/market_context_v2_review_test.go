package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

func TestAttachMarketContextV2ReviewRecordOnly(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Indicators.Klines.SelectedTimeframes = []string{"3m", "15m", "1h", "4h", "1d"}
	at.config.StrategyConfig.Indicators.Klines.PrimaryTimeframe = "15m"

	record := &store.DecisionRecord{ReviewContext: map[string]interface{}{}}
	ctx := &kernel.Context{MarketDataMap: map[string]*market.Data{
		"BTCUSDT": {
			Symbol:       "BTCUSDT",
			CurrentPrice: 100,
			TimeframeData: map[string]*market.TimeframeSeriesData{
				"3m":  {Timeframe: "3m", Klines: []market.KlineBar{{Close: 100}}},
				"15m": {Timeframe: "15m", Klines: []market.KlineBar{{Close: 100}}},
			},
		},
	}}

	at.attachMarketContextV2Review(record, ctx)
	if got, ok := record.ReviewContext["market_context_v2_record_only"].(bool); !ok || !got {
		t.Fatalf("expected record-only marker, got %+v", record.ReviewContext)
	}
	raw, ok := record.ReviewContext["market_context_v2"].(map[string]*market.MarketContextV2)
	if !ok || raw["BTCUSDT"] == nil {
		t.Fatalf("expected market context map, got %+v", record.ReviewContext["market_context_v2"])
	}
	if raw["BTCUSDT"].PrimaryTF != "15m" || raw["BTCUSDT"].DataQuality != "partial" {
		t.Fatalf("unexpected context: %+v", raw["BTCUSDT"])
	}
}
