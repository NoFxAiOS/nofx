package nofxos

import (
	"strings"
	"testing"
)

func TestFormatOIRankingForAIUsesMachineReadableRows(t *testing.T) {
	out := FormatOIRankingForAI(&OIRankingData{
		Duration: "1h",
		TopPositions: []OIPosition{{
			Rank: 1, Symbol: "BTCUSDT", OIDeltaValue: 123456789.12, OIDeltaPercent: 5.5, PriceDeltaPercent: -1.25,
		}},
		LowPositions: []OIPosition{{
			Rank: 2, Symbol: "ETHUSDT", OIDeltaValue: -2345678.9, OIDeltaPercent: -3.25, PriceDeltaPercent: 0.75,
		}},
	}, LangEnglish)

	for _, want := range []string{
		"rank=1 symbol=BTCUSDT oi_delta_usdt=+123.45678912M oi_delta_pct=+5.5 price_delta_pct=-1.25",
		"rank=2 symbol=ETHUSDT oi_delta_usdt=-2.3456789M oi_delta_pct=-3.25 price_delta_pct=+0.75",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "| Rank |") || strings.Contains(out, "%.") {
		t.Fatalf("expected machine-readable rows, got:\n%s", out)
	}
}

func TestFormatNetFlowRankingForAIUsesMachineReadableRows(t *testing.T) {
	out := FormatNetFlowRankingForAI(&NetFlowRankingData{
		Duration: "1h",
		InstitutionFutureTop: []NetFlowPosition{{
			Rank: 1, Symbol: "BTCUSDT", Amount: 9876543.21, Price: 76754.190000,
		}},
		InstitutionFutureLow: []NetFlowPosition{{
			Rank: 2, Symbol: "ETHUSDT", Amount: -123456.78, Price: 2293.230000,
		}},
	}, LangEnglish)

	for _, want := range []string{
		"rank=1 symbol=BTCUSDT flow_usdt=+9.87654321M price=76754.19",
		"rank=2 symbol=ETHUSDT flow_usdt=-123.45678K price=2293.23",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "| Rank |") {
		t.Fatalf("expected machine-readable rows, got:\n%s", out)
	}
}

func TestFormatPriceRankingForAIUsesMachineReadableRows(t *testing.T) {
	out := FormatPriceRankingForAI(&PriceRankingData{
		Durations: map[string]*PriceRankingDuration{
			"1h": {
				Top: []PriceRankingItem{{Symbol: "BTCUSDT", PriceDelta: 0.0125, Price: 76754.190000, FutureFlow: 9876543.21, OIDeltaValue: 123456789.12}},
				Low: []PriceRankingItem{{Symbol: "ETHUSDT", PriceDelta: -0.0075, Price: 2293.230000, FutureFlow: -123456.78, OIDeltaValue: -2345678.9}},
			},
		},
	}, LangEnglish)

	for _, want := range []string{
		"symbol=BTCUSDT price_delta_pct=+1.25 price=76754.19 future_flow_usdt=+9.87654321M oi_delta_usdt=+123.45678912M",
		"symbol=ETHUSDT price_delta_pct=-0.75 price=2293.23 future_flow_usdt=-123.45678K oi_delta_usdt=-2.3456789M",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "| Symbol |") {
		t.Fatalf("expected machine-readable rows, got:\n%s", out)
	}
}

func TestFormatQuantDataForAIUsesMachineReadableRows(t *testing.T) {
	out := FormatQuantDataForAI("BTCUSDT", &QuantData{
		Price:       76754.190000,
		PriceChange: map[string]float64{"1h": 0.0125},
		OI: map[string]*OIData{
			"binance": {
				CurrentOI: 123456789.12,
				NetLong:   123.45,
				NetShort:  67.89,
				Delta: map[string]*OIDeltaData{
					"1h": {OIDeltaValue: 2345678.9, OIDeltaPercent: 3.25},
				},
			},
		},
	}, LangEnglish)

	for _, want := range []string{
		"price=76754.19",
		"timeframe=1h price_delta_pct=+1.25",
		"current_oi=123456789.12",
		"net_long=123.45 net_short=67.89",
		"timeframe=1h oi_delta_usdt=+2.3456789M oi_delta_pct=+3.25",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}
