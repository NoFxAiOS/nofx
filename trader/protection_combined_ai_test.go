package trader

import (
	"testing"

	"nofx/kernel"
)

func TestBuildAIProtectionPlanCombinedLadderAndDrawdown(t *testing.T) {
	plan, err := buildAIProtectionPlan(100, "open_long", &kernel.AIProtectionPlan{
		Mode: "combined",
		LadderRules: []kernel.AIProtectionLadderRule{
			{StopLossPct: 1, StopLossCloseRatioPct: 50, StructuralAnchor: "15m support"},
			{StopLossPct: 2, StopLossCloseRatioPct: 50, StructuralAnchor: "1h support"},
		},
		DrawdownRules: []kernel.AIProtectionDrawdownRule{
			{MinProfitPct: 1.2, MaxDrawdownPct: 40, CloseRatioPct: 50, ReasonAnchor: "first target"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil {
		t.Fatal("expected combined plan")
	}
	if len(plan.StopLossOrders) != 2 {
		t.Fatalf("expected 2 AI ladder stop orders, got %+v", plan.StopLossOrders)
	}
	if plan.StopLossOrders[0].Price != 99 || plan.StopLossOrders[1].Price != 98 {
		t.Fatalf("unexpected ladder prices: %+v", plan.StopLossOrders)
	}
	if len(plan.DrawdownRules) != 1 || plan.DrawdownRules[0].MinProfitPct != 1.2 {
		t.Fatalf("expected AI drawdown rule, got %+v", plan.DrawdownRules)
	}
}
