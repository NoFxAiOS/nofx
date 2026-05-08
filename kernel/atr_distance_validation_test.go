package kernel

import (
	"testing"

	"nofx/store"
)

func TestValidateATRRelativeDistances(t *testing.T) {
	gate := store.EntryGateConfig{
		MinSLDistanceATRMul: 1.2,
		MinRewardATRMul:     1.8,
	}

	tests := []struct {
		name    string
		entry   float64
		sl      float64
		tp      float64
		atrPct  float64
		wantErr bool
	}{
		{
			name:    "sufficient depth — passes",
			entry:   100,
			sl:      97,   // 3% away
			tp:      106,  // 6% away
			atrPct:  1.5,  // ATR = 1.5 => atrAbs = 1.5; SL=3/1.5=2.0x; TP=6/1.5=4.0x
			wantErr: false,
		},
		{
			name:    "SL too shallow — fails",
			entry:   100,
			sl:      99.5, // 0.5% away
			tp:      103,  // 3% away
			atrPct:  1.0,  // ATR = 1.0 => atrAbs = 1.0; SL=0.5/1.0=0.5x < 1.2x
			wantErr: true,
		},
		{
			name:    "target too close — fails",
			entry:   100,
			sl:      97,   // 3% away => 3.0x ATR (passes)
			tp:      101,  // 1% away => 1.0x ATR < 1.8x
			atrPct:  1.0,
			wantErr: true,
		},
		{
			name:    "zero ATR — skipped (no error)",
			entry:   100,
			sl:      99.5,
			tp:      100.5,
			atrPct:  0,
			wantErr: false,
		},
		{
			name:    "short direction — sufficient depth",
			entry:   100,
			sl:      103,  // 3% above
			tp:      94,   // 6% below
			atrPct:  1.5,  // SL=3/1.5=2.0x; TP=6/1.5=4.0x
			wantErr: false,
		},
		{
			name:    "short direction — SL too tight",
			entry:   100,
			sl:      100.8, // 0.8% above
			tp:      96,    // 4% below
			atrPct:  1.0,   // SL=0.8/1.0=0.8x < 1.2x
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rationale := &AIEntryProtectionRationale{
				VolatilityAdjustment: AIEntryVolatilityAdjustment{ATR14Pct: tt.atrPct},
				RiskReward: AIRiskRewardRationale{
					Entry:        tt.entry,
					Invalidation: tt.sl,
					FirstTarget:  tt.tp,
				},
			}
			err := validateATRRelativeDistances(rationale, gate)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateATRRelativeDistances() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
