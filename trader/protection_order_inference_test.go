package trader

import "testing"

func TestInferProtectionClientOrderIDFromPlan(t *testing.T) {
	plan := &ProtectionPlan{
		NeedsStopLoss:        true,
		StopLossPrice:        100,
		FallbackMaxLossPrice: 95,
		NeedsTakeProfit:      true,
		TakeProfitPrice:      110,
		StopLossOrders:       []ProtectionOrder{{Price: 99, CloseRatioPct: 100}},
		TakeProfitOrders:     []ProtectionOrder{{Price: 108, CloseRatioPct: 100}},
	}

	cases := []struct {
		name  string
		order OpenOrder
		want  string
	}{
		{name: "fallback", order: OpenOrder{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 95}, want: "fallback_maxloss_sl"},
		{name: "full stop", order: OpenOrder{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 100}, want: "full_sl"},
		{name: "ladder stop", order: OpenOrder{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 99}, want: "ladder_sl"},
		{name: "full tp", order: OpenOrder{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 110}, want: "full_tp"},
		{name: "ladder tp", order: OpenOrder{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 108}, want: "ladder_tp"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferProtectionClientOrderID(tt.order, "LONG", plan); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}
