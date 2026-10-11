package trader

import (
	"math"
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

func TestCapTakeProfit(t *testing.T) {
	tests := []struct {
		name        string
		isLong      bool
		entry, stop float64
		tp, capR    float64
		want        float64
		changed     bool
	}{
		{"long capped", true, 100, 98, 108, 2, 104, true},
		{"short capped", false, 100, 102, 92, 2, 96, true},
		{"long tp already closer", true, 100, 98, 103, 2, 103, false},
		{"short tp already closer", false, 100, 102, 97, 2, 97, false},
		{"long tp exactly at cap", true, 100, 98, 104, 2, 104, false},
		{"cap 0 = none", true, 100, 98, 108, 0, 108, false},
		{"cap negative = none", true, 100, 98, 108, -2, 108, false},
		{"cap below 1R = none", true, 100, 98, 108, 0.5, 108, false},
		{"cap exactly 1R", true, 100, 98, 108, 1, 102, true},
		{"cap NaN = none", true, 100, 98, 108, math.NaN(), 108, false},
		{"stop == entry", true, 100, 100, 108, 2, 108, false},
		{"long stop above entry", true, 100, 101, 108, 2, 108, false},
		{"short stop below entry", false, 100, 99, 92, 2, 92, false},
		{"zero entry", true, 0, 98, 108, 2, 108, false},
		{"zero stop", true, 100, 0, 108, 2, 108, false},
		{"zero tp", true, 100, 98, 0, 2, 0, false},
		{"short cap would go non-positive", false, 1, 3, 0.5, 2, 0.5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := CapTakeProfit(tc.isLong, tc.entry, tc.stop, tc.tp, tc.capR)
			if changed != tc.changed || math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("CapTakeProfit = (%v, %v), want (%v, %v)", got, changed, tc.want, tc.changed)
			}
		})
	}
}

func TestEffectiveTPMaxR(t *testing.T) {
	for in, want := range map[float64]float64{0: 0, -1: 0, 0.5: 0, 0.99: 0, 1: 1, 2: 2, 2.5: 2.5} {
		if got := (store.RiskControlConfig{TPMaxR: in}).EffectiveTPMaxR(); got != want {
			t.Errorf("EffectiveTPMaxR(%v) = %v, want %v", in, got, want)
		}
	}
}

// SL 2% away, TP 8% away (4R): the open path (market and limit share
// validateOpenRisk, and read decision.TakeProfit afterwards) must end up with
// TP 4% away under tp_max_r=2, untouched otherwise.
func TestValidateOpenRiskCapsTakeProfit(t *testing.T) {
	cases := []struct {
		name   string
		action string
		entry  float64
		sl, tp float64
		wantTP float64
	}{
		{"market long", "open_long", 100, 98, 108, 104},
		{"market short", "open_short", 100, 102, 92, 96},
		{"limit long", "open_long_limit", 100, 98, 108, 104},
		{"limit short", "open_short_limit", 100, 102, 92, 96},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.2, TPMaxR: 2})
			d := &kernel.Decision{Action: tc.action, Symbol: "BTCUSDT", Price: tc.entry, StopLoss: tc.sl, TakeProfit: tc.tp}
			if err := at.validateOpenRisk(d, tc.entry, 0, 3.5); err != nil {
				t.Fatalf("capped open rejected: %v", err)
			}
			if math.Abs(d.TakeProfit-tc.wantTP) > 1e-9 {
				t.Fatalf("TP = %v, want capped %v", d.TakeProfit, tc.wantTP)
			}
			// The market path records decision.TakeProfit after this gate.
			side := "long"
			if strings.Contains(tc.action, "short") {
				side = "short"
			}
			at.recordOpenTakeProfit(d.Symbol, side, d.TakeProfit)
			if got := at.getOpenTakeProfit(d.Symbol, side); math.Abs(got-tc.wantTP) > 1e-9 {
				t.Fatalf("recorded TP = %v, want %v", got, tc.wantTP)
			}

			for _, off := range []float64{0, -1, 0.5} {
				at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.2, TPMaxR: off})
				d := &kernel.Decision{Action: tc.action, Symbol: "BTCUSDT", Price: tc.entry, StopLoss: tc.sl, TakeProfit: tc.tp}
				if err := at.validateOpenRisk(d, tc.entry, 0, 3.5); err != nil {
					t.Fatalf("uncapped open (tp_max_r=%v) rejected: %v", off, err)
				}
				if d.TakeProfit != tc.tp {
					t.Fatalf("tp_max_r=%v must not change TP: got %v want %v", off, d.TakeProfit, tc.tp)
				}
			}
		})
	}
}

// A cap tighter than the strategy's minimum RR rejects the open and names the cap.
func TestValidateOpenRiskCapBelowMinRRRejects(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 3, TPMaxR: 2})
	d := &kernel.Decision{Action: "open_long", Symbol: "BTCUSDT", Price: 100, StopLoss: 98, TakeProfit: 108}
	err := at.validateOpenRisk(d, 100, 0, 3.5)
	if err == nil || !strings.Contains(err.Error(), "tp_max_r") {
		t.Fatalf("expected RR rejection naming tp_max_r, got %v", err)
	}
}

// A plan whose TP is already inside the cap keeps its own TP.
func TestValidateOpenRiskCapKeepsCloserTakeProfit(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.2, TPMaxR: 2})
	d := &kernel.Decision{Action: "open_long", Symbol: "BTCUSDT", Price: 100, StopLoss: 98, TakeProfit: 103}
	if err := at.validateOpenRisk(d, 100, 0, 3.5); err != nil {
		t.Fatal(err)
	}
	if d.TakeProfit != 103 {
		t.Fatalf("TP inside cap changed to %v", d.TakeProfit)
	}
}

func TestValidateOpenRiskCapSkipsBStock(t *testing.T) {
	market.SetEquityClassificationForTesting(
		map[string]bool{"AAPLUSDT": true},
		map[string]bool{"AAPLUSDT": true},
	)
	defer market.SetEquityClassificationForTesting(nil, nil)
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.2, TPMaxR: 2})
	d := &kernel.Decision{Action: "open_long", Symbol: "AAPLUSDT", Price: 100, StopLoss: 98, TakeProfit: 108}
	if err := at.validateOpenRisk(d, 100, 2, 2); err != nil {
		t.Fatal(err)
	}
	if d.TakeProfit != 108 {
		t.Fatalf("bstock TP must not be capped, got %v", d.TakeProfit)
	}
}
