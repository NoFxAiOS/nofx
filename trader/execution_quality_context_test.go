package trader

import "testing"

func TestBuildExecutionQualityContextGradesPartialFeasibility(t *testing.T) {
	ctx := buildExecutionQualityContext(&ExecutionConstraintsSnapshot{MinQty: 0.01, LastPrice: 100, SpreadBps: 5}, 3, 3)
	if ctx == nil || ctx.Grade != "B" || !ctx.PartialCloseFeasible || ctx.LadderTiersFeasible != 3 {
		t.Fatalf("unexpected execution quality: %+v", ctx)
	}
	ctx = buildExecutionQualityContext(&ExecutionConstraintsSnapshot{MinQty: 1, LastPrice: 100, SpreadBps: 25}, 50, 3)
	if ctx.Grade != "D" || ctx.PartialCloseFeasible {
		t.Fatalf("expected poor/non-partial feasible execution quality, got %+v", ctx)
	}
}
