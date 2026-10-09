package hyperliquid

import "testing"

func TestIsTakeProfitOrderPreservesProtectiveStops(t *testing.T) {
	tests := []struct {
		name           string
		positionSide   string
		orderSide      string
		orderPrice     float64
		markPrice      float64
		wantTakeProfit bool
	}{
		{name: "long take profit", positionSide: "long", orderSide: "A", orderPrice: 105, markPrice: 100, wantTakeProfit: true},
		{name: "long stop loss", positionSide: "long", orderSide: "A", orderPrice: 95, markPrice: 100},
		{name: "short take profit", positionSide: "short", orderSide: "B", orderPrice: 95, markPrice: 100, wantTakeProfit: true},
		{name: "short stop loss", positionSide: "short", orderSide: "B", orderPrice: 105, markPrice: 100},
		{name: "wrong close side", positionSide: "long", orderSide: "B", orderPrice: 105, markPrice: 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isTakeProfitOrder(test.positionSide, test.orderSide, test.orderPrice, test.markPrice)
			if got != test.wantTakeProfit {
				t.Fatalf("isTakeProfitOrder() = %v, want %v", got, test.wantTakeProfit)
			}
		})
	}
}
