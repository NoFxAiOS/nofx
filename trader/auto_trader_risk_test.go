package trader

import "testing"

func TestDrawdownCloseArmsOnPriceBasisOnly(t *testing.T) {
	cases := []struct {
		name         string
		pricePnLPct  float64
		drawdownPct  float64
		shouldClose  bool
	}{
		// +0.5% price move (what +5% margin at 10x used to arm on) must NOT
		// arm the monitor, no matter how large the relative drawdown is.
		{"tiny price gain big drawdown", 0.5, 60.0, false},
		// Armed only from a real +5% price move, and still needs the 40% giveback.
		{"real gain small drawdown", 6.0, 20.0, false},
		{"real gain big drawdown", 6.0, 45.0, true},
		{"at threshold not armed", 5.0, 45.0, false},
		{"loss never triggers", -3.0, 80.0, false},
	}
	for _, c := range cases {
		if got := shouldDrawdownClose(c.pricePnLPct, c.drawdownPct); got != c.shouldClose {
			t.Fatalf("%s: shouldDrawdownClose(%.1f, %.1f) = %v, want %v",
				c.name, c.pricePnLPct, c.drawdownPct, got, c.shouldClose)
		}
	}
}
