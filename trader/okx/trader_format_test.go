package okx

import "testing"

func TestFormatPriceRoundsToTickSizeForTrailingOrders(t *testing.T) {
	trader := &OKXTrader{}
	inst := &OKXInstrument{TickSz: 0.0001}

	if got := trader.formatPrice(0.24137, inst); got != "0.2414" {
		t.Fatalf("expected rounded activePx 0.2414, got %s", got)
	}
}

func TestFormatPriceHandlesIntegerTickSize(t *testing.T) {
	trader := &OKXTrader{}
	inst := &OKXInstrument{TickSz: 1}

	if got := trader.formatPrice(123.6, inst); got != "124" {
		t.Fatalf("expected rounded integer price 124, got %s", got)
	}
}
