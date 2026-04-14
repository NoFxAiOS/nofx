package market

import "testing"

func TestNormalizePreservesGMGNSymbol(t *testing.T) {
	got := Normalize("BSC:0xAbCdEf0123456789abcdef0123456789ABCDEF01")
	want := "bsc:0xAbCdEf0123456789abcdef0123456789ABCDEF01"
	if got != want {
		t.Fatalf("Normalize() = %q, want %q", got, want)
	}
}

func TestNormalizeKeepsLegacyUSDTBehavior(t *testing.T) {
	got := Normalize("sol")
	if got != "SOLUSDT" {
		t.Fatalf("Normalize() = %q, want %q", got, "SOLUSDT")
	}
}
