package trader

import "testing"

func TestExecutableMinPositionUSDUsesExchangeConstraints(t *testing.T) {
	snap := &ExecutionConstraintsSnapshot{
		MinNotional: 10,
		MinQty:      0.01,
		QtyStepSize: 0.01,
		LastPrice:   1000,
	}
	got := snap.ExecutableMinPositionUSD(12)
	if got < 19.5 || got > 20.5 {
		t.Fatalf("expected executable minimum near 20 after qty floor, got %.4f", got)
	}
}

func TestExecutableMinPositionUSDFallsBackToDefault(t *testing.T) {
	if got := (*ExecutionConstraintsSnapshot)(nil).ExecutableMinPositionUSD(12); got != 12 {
		t.Fatalf("expected default fallback 12, got %.4f", got)
	}
}
