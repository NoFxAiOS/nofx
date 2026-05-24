package kernel

import "testing"

func TestClampHyperRankLimit(t *testing.T) {
	if got := clampHyperRankLimit(0); got != 5 {
		t.Fatalf("clamp 0 = %d, want 5", got)
	}
	if got := clampHyperRankLimit(99); got != 10 {
		t.Fatalf("clamp 99 = %d, want 10", got)
	}
	if got := clampHyperRankLimit(3); got != 3 {
		t.Fatalf("clamp 3 = %d, want 3", got)
	}
}
