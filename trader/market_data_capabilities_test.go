package trader

import "testing"

func TestGetMarketDataCapabilities_Binance(t *testing.T) {
	at := &AutoTrader{exchange: "binance"}
	caps := at.GetMarketDataCapabilities()

	if !caps.InstrumentTickSize || !caps.InstrumentPricePrecision || !caps.InstrumentQtyStep || !caps.InstrumentQtyPrecision {
		t.Fatalf("expected binance instrument precision/step capabilities, got %+v", caps)
	}
	if !caps.QuoteLastPrice || !caps.QuoteMarkPrice {
		t.Fatalf("expected binance last/mark price capabilities, got %+v", caps)
	}
	if caps.InstrumentMinNotional || caps.QuoteBestBid || caps.QuoteBestAsk || caps.QuoteSpread {
		t.Fatalf("expected conservative false values for unsupported/not-yet-wired binance fields, got %+v", caps)
	}
}

func TestGetMarketDataCapabilities_OKX(t *testing.T) {
	at := &AutoTrader{exchange: "okx"}
	caps := at.GetMarketDataCapabilities()

	if !caps.InstrumentTickSize || !caps.InstrumentPricePrecision || !caps.InstrumentQtyStep || !caps.InstrumentQtyPrecision {
		t.Fatalf("expected okx precision/step capabilities, got %+v", caps)
	}
	if !caps.InstrumentMinQty || !caps.InstrumentContractValue {
		t.Fatalf("expected okx min size + contract value capabilities, got %+v", caps)
	}
	if !caps.QuoteLastPrice {
		t.Fatalf("expected okx last price capability, got %+v", caps)
	}
	if caps.QuoteMarkPrice {
		t.Fatalf("expected conservative false value for not-yet-wired okx mark price, got %+v", caps)
	}
	if !caps.QuoteBestBid || !caps.QuoteBestAsk || !caps.QuoteSpread {
		t.Fatalf("expected okx top-of-book quote capabilities, got %+v", caps)
	}
}

func TestGetMarketDataCapabilities_DegradedFallback(t *testing.T) {
	at := &AutoTrader{exchange: "bybit"}
	caps := at.GetMarketDataCapabilities()
	if !caps.DegradedProfile || !caps.QuoteLastPrice || !caps.QuoteMarkPrice || !caps.FeeFallback {
		t.Fatalf("expected degraded fallback profile, got %+v", caps)
	}
}
