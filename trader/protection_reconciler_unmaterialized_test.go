package trader

import "testing"

func TestReconcileResultForUnmaterializedPlanRequiresExchangeStop(t *testing.T) {
	result := reconcileResultForUnmaterializedPlan(nil, "SHORT", true)
	if result.ExchangeVerified {
		t.Fatalf("expected no verification without a stop, got %+v", result)
	}
	if result.Summary != "configured protection plan not materialized and no exchange stop present" {
		t.Fatalf("unexpected summary: %+v", result)
	}
}

func TestReconcileResultForUnmaterializedPlanAcceptsDegradedExchangeStop(t *testing.T) {
	result := reconcileResultForUnmaterializedPlan([]OpenOrder{{PositionSide: "SHORT", Type: "STOP_MARKET", StopPrice: 1.23}}, "SHORT", true)
	if !result.ExchangeVerified || result.Summary != "degraded_exchange_stop_present_without_materialized_plan" {
		t.Fatalf("expected degraded exchange stop verification, got %+v", result)
	}
}

func TestReconcileResultForUnmaterializedPlanDoesNotVerifyProfitOnly(t *testing.T) {
	result := reconcileResultForUnmaterializedPlan([]OpenOrder{{PositionSide: "SHORT", Type: "TAKE_PROFIT_MARKET", StopPrice: 1.11}}, "SHORT", true)
	if result.ExchangeVerified {
		t.Fatalf("profit-only exchange order must not verify protection, got %+v", result)
	}
	if result.Summary != "exchange_profit_present_but_stop_missing_without_materialized_plan" {
		t.Fatalf("unexpected summary: %+v", result)
	}
}
