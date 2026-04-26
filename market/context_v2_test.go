package market

import "testing"

func TestBuildMarketContextV2SummarizesDataQualityAndStructure(t *testing.T) {
	data := &Data{
		Symbol:        "BTCUSDT",
		CurrentPrice:  100,
		PriceChange1h: 1.2,
		OpenInterest:  &OIData{Latest: 120, Average: 100},
		FundingRate:   0.0006,
		LongerTermContext: &LongerTermData{
			CurrentVolume: 150,
			AverageVolume: 100,
		},
		TimeframeData: map[string]*TimeframeSeriesData{
			"3m":  {Timeframe: "3m", Klines: []KlineBar{{Close: 100}}},
			"15m": {Timeframe: "15m", Klines: []KlineBar{{Close: 100}}},
			"1h":  {Timeframe: "1h", Klines: []KlineBar{{Close: 100}}},
		},
		StructuralLevels: []StructuralLevel{
			{Type: "support", Price: 95},
			{Type: "support", Price: 90},
			{Type: "resistance", Price: 105},
			{Type: "resistance", Price: 110},
		},
		FibonacciLevels: &FibonacciLevels{Levels: map[string]float64{"0.382": 97, "0.5": 100, "0.618": 103}},
	}

	ctx := BuildMarketContextV2("BTCUSDT", data, []string{"3m", "15m", "1h", "4h", "1d"}, "15m")
	if ctx.Symbol != "BTCUSDT" || ctx.PrimaryTF != "15m" || ctx.TriggerTF != "3m" || ctx.BiasTF != "1h" {
		t.Fatalf("unexpected timeframe summary: %+v", ctx)
	}
	if ctx.DataQuality != "partial" || len(ctx.MissingTFs) != 2 {
		t.Fatalf("expected partial data with 4h/1d missing, got %+v", ctx)
	}
	if ctx.Derivatives == nil || ctx.Derivatives.FundingBias != "long_crowded" || ctx.Derivatives.OIChange1hPct != 20 {
		t.Fatalf("unexpected derivatives context: %+v", ctx.Derivatives)
	}
	if ctx.Structure == nil || ctx.Structure.NearestSupport != 95 || ctx.Structure.NearestResist != 105 || ctx.Structure.RangePosition != "middle" {
		t.Fatalf("unexpected structure brief: %+v", ctx.Structure)
	}
}

func TestBuildMarketContextV2MissingData(t *testing.T) {
	ctx := BuildMarketContextV2("ETHUSDT", nil, []string{"15m", "1h"}, "15m")
	if ctx.DataQuality != "missing" || len(ctx.MissingTFs) != 2 || ctx.Derivatives != nil || ctx.Structure != nil {
		t.Fatalf("unexpected missing context: %+v", ctx)
	}
}

func TestClassifySqueezeRisk(t *testing.T) {
	if got := classifySqueezeRisk(12, 0.2, 0); got != "high" {
		t.Fatalf("expected high OI squeeze risk, got %s", got)
	}
	if got := classifySqueezeRisk(4, 0.2, 0); got != "medium" {
		t.Fatalf("expected medium OI squeeze risk, got %s", got)
	}
	if got := classifySqueezeRisk(0.5, 0.1, 0.0001); got != "low" {
		t.Fatalf("expected low squeeze risk, got %s", got)
	}
}
