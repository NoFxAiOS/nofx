package trader

import "testing"

func TestEvaluateProtectionOwnership_FullManualProtected(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 98, NeedsTakeProfit: true, TakeProfitPrice: 110}
	orders := []OpenOrder{
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98},
		{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 110},
	}

	state := evaluateProtectionOwnership(orders, "LONG", plan, false, false)
	if !state.Verified || state.State != "protected" || state.StopOwner != "full_sl" || state.ProfitOwner != "full_tp" {
		t.Fatalf("unexpected ownership state: %+v", state)
	}
}

func TestEvaluateProtectionOwnership_LadderManualProtected(t *testing.T) {
	plan := &ProtectionPlan{
		NeedsStopLoss:    true,
		NeedsTakeProfit:  true,
		StopLossOrders:   []ProtectionOrder{{Price: 98, CloseRatioPct: 50}, {Price: 96, CloseRatioPct: 50}},
		TakeProfitOrders: []ProtectionOrder{{Price: 105, CloseRatioPct: 50}, {Price: 110, CloseRatioPct: 50}},
	}
	orders := []OpenOrder{
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98},
		{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 96},
		{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 105},
		{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 110},
	}

	state := evaluateProtectionOwnership(orders, "LONG", plan, false, false)
	if !state.Verified || state.StopOwner != "ladder_sl" || state.ProfitOwner != "ladder_tp" {
		t.Fatalf("unexpected ownership state: %+v", state)
	}
}

func TestEvaluateProtectionOwnership_ActivePositionWithZeroOrdersIsUnprotected(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 98, NeedsTakeProfit: true, TakeProfitPrice: 110}
	state := evaluateProtectionOwnership(nil, "LONG", plan, false, false)
	if state.Verified || state.State != "unprotected" || !state.MissingStop || !state.MissingProfit {
		t.Fatalf("expected unprotected zero-order state, got %+v", state)
	}
}

func TestEvaluateProtectionOwnership_DrawdownArmedCanOwnProfitButNotStop(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 98, NeedsTakeProfit: true, TakeProfitPrice: 110}
	orders := []OpenOrder{{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98}}

	state := evaluateProtectionOwnership(orders, "LONG", plan, false, true)
	if !state.Verified || state.StopOwner != "full_sl" || state.ProfitOwner != "drawdown" {
		t.Fatalf("expected drawdown to own profit and full SL to own stop, got %+v", state)
	}
}

func TestEvaluateProtectionOwnership_DrawdownArmedWithoutStopIsNotVerified(t *testing.T) {
	plan := &ProtectionPlan{NeedsStopLoss: true, StopLossPrice: 98, NeedsTakeProfit: true, TakeProfitPrice: 110}
	state := evaluateProtectionOwnership(nil, "LONG", plan, false, true)
	if state.Verified || state.ProfitOwner != "drawdown" || state.StopOwner != "" {
		t.Fatalf("expected drawdown profit owner without stop to be degraded/unprotected, got %+v", state)
	}
}

func TestEvaluateProtectionOwnership_BreakEvenOwnsStopAndLadderOwnsProfit(t *testing.T) {
	plan := &ProtectionPlan{
		NeedsStopLoss:    true,
		NeedsTakeProfit:  true,
		StopLossOrders:   []ProtectionOrder{{Price: 98, CloseRatioPct: 100}},
		TakeProfitOrders: []ProtectionOrder{{Price: 110, CloseRatioPct: 100}},
	}
	orders := []OpenOrder{{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 110}}
	state := evaluateProtectionOwnership(orders, "LONG", plan, true, false)
	if !state.Verified || state.StopOwner != "breakeven" || state.ProfitOwner != "ladder_tp" {
		t.Fatalf("expected break-even stop owner and ladder TP owner, got %+v", state)
	}
}

func TestEvaluateProtectionOwnership_FallbackStopOnlyWhenProfitNotRequired(t *testing.T) {
	plan := &ProtectionPlan{FallbackMaxLossPrice: 95}
	orders := []OpenOrder{{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 95}}
	state := evaluateProtectionOwnership(orders, "LONG", plan, false, false)
	if !state.Verified || state.StopOwner != "fallback" || state.ProfitOwner != "" {
		t.Fatalf("expected fallback-only verified when profit owner not required, got %+v", state)
	}
}
