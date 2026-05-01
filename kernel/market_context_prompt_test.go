package kernel

import (
	"strings"
	"testing"

	"nofx/market"
)

func TestFormatMarketContextV2UsesCompositeAICompactWithTimestamp(t *testing.T) {
	engine := &StrategyEngine{}
	data := &market.Data{
		Symbol:        "BTCUSDT",
		CurrentPrice:  100,
		PriceChange1h: 1,
		PriceChange4h: 2,
		FundingRate:   0.0001,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": {
				Timeframe:        "15m",
				Klines:           []market.KlineBar{{Time: 1, Open: 99, High: 101, Low: 98, Close: 100}},
				StructuralLevels: []market.StructuralLevel{{Price: 98, Type: "support", Timeframe: "15m", Strength: 3, Source: "swing_point"}},
			},
		},
		StructuralLevels: []market.StructuralLevel{{Price: 98, Type: "support", Timeframe: "15m", Strength: 3, Source: "swing_point"}},
	}
	out := engine.formatMarketContextV2("BTCUSDT", data)
	for _, want := range []string{"Composite Market Context", "updated_at=", "ttl=180s", "point-in-time", "line 15m support"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}
