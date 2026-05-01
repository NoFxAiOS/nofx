package market

import (
	"testing"
	"time"

	"nofx/provider/nofxos"
)

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

func TestClassifyRangePositionUsesRegimeCompatibleLabels(t *testing.T) {
	if got := classifyRangePosition(97, 95, 105); got != "near_support" {
		t.Fatalf("expected near_support, got %s", got)
	}
	if got := classifyRangePosition(103, 95, 105); got != "near_resistance" {
		t.Fatalf("expected near_resistance, got %s", got)
	}
	if got := classifyRangePosition(100, 95, 105); got != "middle" {
		t.Fatalf("expected middle, got %s", got)
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

func TestBuildRegimeEntryGuidanceBindsStructureToRegime(t *testing.T) {
	data := &Data{CurrentPrice: 100, PriceChange1h: 1.1, PriceChange4h: 1.6, FundingRate: 0.0001}
	ctx := BuildMarketContextV2("BTCUSDT", data, nil, "15m")
	if ctx.RegimeRules == nil || ctx.RegimeRules.Regime != "trend_up" {
		t.Fatalf("expected trend_up guidance, got %+v", ctx.RegimeRules)
	}
	if ctx.RegimeRules.FibonacciMode != "prefer_retracement_confluence" {
		t.Fatalf("expected fib prefer in trend, got %+v", ctx.RegimeRules)
	}
}

func TestBuildQuantContextFromNofxOSClassifiesCrowdingAndFlow(t *testing.T) {
	q := &nofxos.QuantData{Netflow: &nofxos.NetflowData{Institution: &nofxos.FlowTypeData{Future: map[string]float64{"1h": 1000}}, Personal: &nofxos.FlowTypeData{Future: map[string]float64{"1h": 500}}}, OI: map[string]*nofxos.OIData{"binance": {Delta: map[string]*nofxos.OIDeltaData{"1h": {OIDeltaPercent: 9, OIDeltaValue: 100000}}}}}
	ctx := BuildQuantContextFromNofxOS(q)
	if ctx == nil || ctx.CrowdingRisk != "high" || ctx.FlowBias != "institution_inflow" {
		t.Fatalf("unexpected quant context: %+v", ctx)
	}
}

func TestBuildExchangeFlowContextClassifiesCrowding(t *testing.T) {
	ls := 1.35
	taker := 1.25
	imb := 0.3
	bid := 100000.0
	ask := 50000.0
	ctx := BuildExchangeFlowContext(&Data{FundingRate: 0.0006, LongShortRatio: &ls, TakerBuySellRatio: &taker, DepthImbalance: &imb, DepthBidTotal: &bid, DepthAskTotal: &ask})
	if ctx.CrowdingRisk != "high" || ctx.LongShortSkew != "long_crowded" || ctx.DepthBias != "bid_heavy" {
		t.Fatalf("unexpected exchange flow context: %+v", ctx)
	}
}

func TestBuildCompositeMarketSnapshotIncludesLinesAndAICompact(t *testing.T) {
	data := &Data{
		Symbol:        "BTCUSDT",
		CurrentPrice:  100,
		PriceChange1h: 1.2,
		PriceChange4h: -0.5,
		FundingRate:   0.0001,
		TimeframeData: map[string]*TimeframeSeriesData{
			"15m": {
				Timeframe:        "15m",
				Klines:           []KlineBar{{Time: 1, Open: 99, High: 101, Low: 98, Close: 100}},
				StructuralLevels: []StructuralLevel{{Price: 98, Type: "support", Timeframe: "15m", Strength: 3, Source: "swing_point"}},
				FibonacciLevels:  &FibonacciLevels{Timeframe: "15m", Direction: "retracement_down", Levels: map[string]float64{"0.5": 100.5}},
			},
		},
		StructuralLevels: []StructuralLevel{{Price: 98, Type: "support", Timeframe: "15m", Strength: 3, Source: "swing_point"}},
		FibonacciLevels:  &FibonacciLevels{Timeframe: "15m", Direction: "retracement_down", Levels: map[string]float64{"0.5": 100.5}},
	}
	s := buildCompositeMarketSnapshotFromData("okx", []string{"15m"}, "15m", 15*time.Second, data)
	if s == nil || len(s.Lines) == 0 || s.AICompact == "" || s.Context == nil {
		t.Fatalf("expected composite snapshot with lines/context/ai compact, got %+v", s)
	}
	if s.Timeframes["15m"].Lines[0].DistancePct == 0 {
		t.Fatalf("expected line distance pct to be populated: %+v", s.Timeframes["15m"].Lines[0])
	}
}

func TestProjectCompositeMarketSnapshotViewsTrimPayload(t *testing.T) {
	klines := make([]KlineBar, 140)
	for i := range klines {
		klines[i] = KlineBar{Time: int64(i), Open: 100, High: 101, Low: 99, Close: 100, Volume: 1}
	}
	lines := make([]CompositeMarketLine, 40)
	for i := range lines {
		lines[i] = CompositeMarketLine{ID: "l", Price: 100 + float64(i), DistancePct: float64(i)}
	}
	s := &CompositeMarketSnapshot{Timeframes: map[string]CompositeMarketTimeframe{"15m": {Klines: klines, Lines: lines}}, Lines: lines, Context: &MarketContextV2{}, AICompact: "ai"}
	chart := ProjectCompositeMarketSnapshot(s, "chart")
	if chart.Context != nil || len(chart.Timeframes["15m"].Klines) != 120 || len(chart.Timeframes["15m"].Lines) != 24 {
		t.Fatalf("unexpected chart projection: %+v", chart)
	}
	ai := ProjectCompositeMarketSnapshot(s, "ai")
	if ai.Timeframes != nil || ai.Context != nil || len(ai.Lines) != 12 || ai.AICompact == "" {
		t.Fatalf("unexpected ai projection: %+v", ai)
	}
}

func TestComputeOIDeltaScoresSeparatesIncreaseAndDecrease(t *testing.T) {
	prev := []HotCoin{{Symbol: "AAAUSDT", OpenInterestUSD: 100, QuoteVolume24h: hotCoinMinVolume, PriceChangePct: 1}, {Symbol: "BBBUSDT", OpenInterestUSD: 100, QuoteVolume24h: hotCoinMinVolume, PriceChangePct: 1}}
	computeOIDeltaScores("test", prev, true)
	cur := []HotCoin{{Symbol: "AAAUSDT", OpenInterestUSD: 110, QuoteVolume24h: hotCoinMinVolume, PriceChangePct: 1}, {Symbol: "BBBUSDT", OpenInterestUSD: 90, QuoteVolume24h: hotCoinMinVolume, PriceChangePct: 1}}
	inc, ok := computeOIDeltaScores("test", cur, true)
	if !ok || len(inc) != 1 || inc[0].Symbol != "AAAUSDT" || inc[0].HotScore <= 0 {
		t.Fatalf("unexpected increase scores: ok=%v %+v", ok, inc)
	}
	// Re-seed previous snapshot because computeOIDeltaScores updates state each call.
	computeOIDeltaScores("test2", prev, true)
	dec, ok := computeOIDeltaScores("test2", cur, false)
	if !ok || len(dec) != 1 || dec[0].Symbol != "BBBUSDT" || dec[0].HotScore >= 0 {
		t.Fatalf("unexpected decrease scores: ok=%v %+v", ok, dec)
	}
}
