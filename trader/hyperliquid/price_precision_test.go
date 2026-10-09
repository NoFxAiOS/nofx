package hyperliquid

import (
	"math"
	"testing"
)

func TestPerpPricePrecision(t *testing.T) {
	for _, tc := range []struct {
		price        float64
		sizeDecimals int
		want         float64
	}{
		{0.006364 * 1.01, 0, 0.006428}, // PUMP: five significant figures alone exceeds six decimals
		{0.006364 * 0.99, 0, 0.006300},
		{0.00625, 0, 0.00625}, // protective stop
		{1234.56, 1, 1234.6},
		{0.012345, 1, 0.01235},
		{0.0012345, 0, 0.001235},
		{123456, 5, 123456},  // integer-price exception
		{193.731, 3, 193.73}, // HIP-3
	} {
		got, err := roundPerpPrice(tc.price, tc.sizeDecimals)
		if err != nil || math.Abs(got-tc.want) > 1e-10 {
			t.Errorf("price=%g sz=%d got=%g err=%v want=%g", tc.price, tc.sizeDecimals, got, err, tc.want)
		}
	}
	for _, price := range []float64{0, -1, math.NaN(), math.Inf(1), 0.000000001} {
		if _, err := roundPerpPrice(price, 0); err == nil {
			t.Errorf("expected invalid price %g", price)
		}
	}
	if _, err := (&HyperliquidTrader{}).roundOrderPrice("UNKNOWN", 100); err == nil {
		t.Fatal("unknown precision must not be guessed")
	}
}
