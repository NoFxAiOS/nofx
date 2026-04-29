package api

import "testing"

func TestNormalizeAPIPositionSideForStore(t *testing.T) {
	cases := map[string]string{
		"long":  "LONG",
		"LONG":  "LONG",
		"buy":   "LONG",
		"short": "SHORT",
		"SELL":  "SHORT",
		"both":  "BOTH",
	}
	for input, want := range cases {
		if got := normalizeAPIPositionSideForStore(input); got != want {
			t.Fatalf("normalizeAPIPositionSideForStore(%q) = %q, want %q", input, got, want)
		}
	}
}
