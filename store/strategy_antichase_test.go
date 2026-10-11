package store

import "testing"

func TestEffectiveAntiChaseKnobs(t *testing.T) {
	cases := []struct {
		name string
		get  func(RiskControlConfig) float64
		set  func(*RiskControlConfig, float64)
		def  float64
	}{
		{"ChaseMaxEMA20_1hPct", RiskControlConfig.EffectiveChaseMaxEMA20_1hPct, func(r *RiskControlConfig, v float64) { r.ChaseMaxEMA20_1hPct = v }, 6},
		{"CounterMaxEMA20_1hPct", RiskControlConfig.EffectiveCounterMaxEMA20_1hPct, func(r *RiskControlConfig, v float64) { r.CounterMaxEMA20_1hPct = v }, 3},
		{"EntryRSI1hMax", RiskControlConfig.EffectiveEntryRSI1hMax, func(r *RiskControlConfig, v float64) { r.EntryRSI1hMax = v }, 78},
		{"ChaseMaxMove4hPct", RiskControlConfig.EffectiveChaseMaxMove4hPct, func(r *RiskControlConfig, v float64) { r.ChaseMaxMove4hPct = v }, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rc RiskControlConfig
			if got := tc.get(rc); got != tc.def {
				t.Errorf("unset -> %v want default %v", got, tc.def)
			}
			tc.set(&rc, -1)
			if got := tc.get(rc); got > 0 {
				t.Errorf("negative -> %v want <=0 (gate off)", got)
			}
			tc.set(&rc, 9.5)
			if got := tc.get(rc); got != 9.5 {
				t.Errorf("positive -> %v want 9.5", got)
			}
		})
	}
}
