package kernel

import (
	"strings"
	"testing"

	"nofx/market"
)

// acSig builds a minimal signal: live price `price`, 1h EMA20 `ema`, 1h RSI
// `rsi`, 4h price change `chg4h`. nil ema/rsi = missing evidence.
func acSig(symbol string, price float64, ema, rsi *float64, chg4h float64) *SymbolSignal {
	return &SymbolSignal{
		Symbol:           symbol,
		Price:            price,
		PriceChange4hPct: chg4h,
		Timeframes: map[string]*TFSignal{
			"1h": {EMAFast: ema, RSI14: rsi},
		},
	}
}

func acf(v float64) *float64 { return &v }

// acPullback attaches a long pullback plan (retest anchor `entry`) to sig.
func acPullback(sig *SymbolSignal, entry float64, active bool) *SymbolSignal {
	sig.LongPullback = &LongPullbackPlan{Active: active, Entry: entry}
	return sig
}

func TestIsAntiChaseCode(t *testing.T) {
	for _, c := range []string{"CHASE_EMA20_1H_7.0_GT_6", "COUNTER_EMA20_1H_5.0_GT_3", "RSI1H_EXHAUSTED_81", "CHASE_MOVE4H_9.5_GT_6"} {
		if !IsAntiChaseCode(c) {
			t.Errorf("%s must be an anti-chase code", c)
		}
	}
	for _, c := range []string{"EMA20_STRETCH_15.8_GT_12", "RR_MAX_0.90", "MICRO_TREND_NOT_LONG", "CHASE", ""} {
		if IsAntiChaseCode(c) {
			t.Errorf("%q must not be an anti-chase code", c)
		}
	}
}

// acOpt sets the four knobs; 0 leaves that gate off so each case isolates
// its code.
func acOpt(chase, counter, rsi, move4h float64) SignalOptions {
	return SignalOptions{
		ChaseMaxEMA20_1hPct:   chase,
		CounterMaxEMA20_1hPct: counter,
		EntryRSI1hMax:         rsi,
		ChaseMaxMove4hPct:     move4h,
	}
}

func TestAntiChaseGates(t *testing.T) {
	market.SetEquityClassificationForTesting(map[string]bool{"AAPLUSDT": true}, map[string]bool{"AAPLUSDT": true})
	defer market.SetEquityClassificationForTesting(nil, nil)

	cases := []struct {
		name      string
		sig       *SymbolSignal
		opt       SignalOptions
		code      string // prefix
		longFires bool
		shortFire bool
	}{
		// CHASE_EMA20_1H: entry 107 vs EMA 100 = +7% above (long chase);
		// entry 93 vs EMA 100 = 7% below (short chase).
		{"chase long fires", acSig("TESTUSDT", 107, acf(100), nil, 0), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", true, false},
		{"chase short fires", acSig("TESTUSDT", 93, acf(100), nil, 0), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", false, true},
		{"chase passes inside cap", acSig("TESTUSDT", 105, acf(100), nil, 0), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", false, false},
		{"chase missing EMA fails open", acSig("TESTUSDT", 107, nil, nil, 0), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", false, false},
		{"chase bstock exempt", acSig("AAPLUSDT", 107, acf(100), nil, 0), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", false, false},
		{"chase knob off", acSig("TESTUSDT", 107, acf(100), nil, 0), acOpt(0, 0, 0, 0), "CHASE_EMA20_1H_", false, false},

		// Retest-anchor basis: live 107 is 7% extended, but an active long
		// pullback plan anchors at 103 (3% above the EMA) -> long passes;
		// inactive plan with the same live price fires. Shorts unchanged.
		{"chase long pullback active anchor within cap passes", acPullback(acSig("TESTUSDT", 107, acf(100), nil, 0), 103, true), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", false, false},
		{"chase long pullback active anchor still extended fires", acPullback(acSig("TESTUSDT", 107, acf(100), nil, 0), 106.5, true), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", true, false},
		{"chase long pullback inactive fires", acPullback(acSig("TESTUSDT", 107, acf(100), nil, 0), 103, false), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", true, false},
		{"chase short ignores pullback anchor", acPullback(acSig("TESTUSDT", 93, acf(100), nil, 0), 103, true), acOpt(6, 0, 0, 0), "CHASE_EMA20_1H_", false, true},
		{"counter long keeps plain entry despite pullback", acPullback(acSig("TESTUSDT", 95, acf(100), nil, 0), 90, true), acOpt(0, 3, 0, 0), "COUNTER_EMA20_1H_", true, false},

		// COUNTER_EMA20_1H: long entry 95 vs EMA 100 = 5% below (knife);
		// short entry 105 = 5% above (fading a pump).
		{"counter long fires", acSig("TESTUSDT", 95, acf(100), nil, 0), acOpt(0, 3, 0, 0), "COUNTER_EMA20_1H_", true, false},
		{"counter short fires", acSig("TESTUSDT", 105, acf(100), nil, 0), acOpt(0, 3, 0, 0), "COUNTER_EMA20_1H_", false, true},
		{"counter passes inside cap", acSig("TESTUSDT", 98, acf(100), nil, 0), acOpt(0, 3, 0, 0), "COUNTER_EMA20_1H_", false, false},
		{"counter missing EMA fails open", acSig("TESTUSDT", 95, nil, nil, 0), acOpt(0, 3, 0, 0), "COUNTER_EMA20_1H_", false, false},
		{"counter bstock exempt", acSig("AAPLUSDT", 95, acf(100), nil, 0), acOpt(0, 3, 0, 0), "COUNTER_EMA20_1H_", false, false},
		{"counter knob off", acSig("TESTUSDT", 95, acf(100), nil, 0), acOpt(0, 0, 0, 0), "COUNTER_EMA20_1H_", false, false},

		// RSI1H_EXHAUSTED: long at >=78, short at <=22.
		{"rsi long fires", acSig("TESTUSDT", 100, nil, acf(82), 0), acOpt(0, 0, 78, 0), "RSI1H_EXHAUSTED_", true, false},
		{"rsi long boundary fires", acSig("TESTUSDT", 100, nil, acf(78), 0), acOpt(0, 0, 78, 0), "RSI1H_EXHAUSTED_", true, false},
		{"rsi short fires", acSig("TESTUSDT", 100, nil, acf(18), 0), acOpt(0, 0, 78, 0), "RSI1H_EXHAUSTED_", false, true},
		{"rsi mid passes", acSig("TESTUSDT", 100, nil, acf(50), 0), acOpt(0, 0, 78, 0), "RSI1H_EXHAUSTED_", false, false},
		{"rsi missing fails open", acSig("TESTUSDT", 100, nil, nil, 0), acOpt(0, 0, 78, 0), "RSI1H_EXHAUSTED_", false, false},
		{"rsi bstock exempt", acSig("AAPLUSDT", 100, nil, acf(90), 0), acOpt(0, 0, 78, 0), "RSI1H_EXHAUSTED_", false, false},
		{"rsi knob off", acSig("TESTUSDT", 100, nil, acf(90), 0), acOpt(0, 0, 0, 0), "RSI1H_EXHAUSTED_", false, false},

		// CHASE_MOVE4H: +9% fires long, -9% fires short.
		{"move4h long fires", acSig("TESTUSDT", 100, nil, nil, 9), acOpt(0, 0, 0, 6), "CHASE_MOVE4H_", true, false},
		{"move4h short fires", acSig("TESTUSDT", 100, nil, nil, -9), acOpt(0, 0, 0, 6), "CHASE_MOVE4H_", false, true},
		{"move4h passes inside cap", acSig("TESTUSDT", 100, nil, nil, 4), acOpt(0, 0, 0, 6), "CHASE_MOVE4H_", false, false},
		{"move4h missing (0) fails open", acSig("TESTUSDT", 100, nil, nil, 0), acOpt(0, 0, 0, 6), "CHASE_MOVE4H_", false, false},
		{"move4h bstock exempt", acSig("AAPLUSDT", 100, nil, nil, 9), acOpt(0, 0, 0, 6), "CHASE_MOVE4H_", false, false},
		{"move4h knob off", acSig("TESTUSDT", 100, nil, nil, 9), acOpt(0, 0, 0, 0), "CHASE_MOVE4H_", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := computeHardEntryGate(tc.sig, tc.opt)
			if got := hasCode(g.Long, tc.code); got != tc.longFires {
				t.Errorf("long %s fired=%v want %v (failed=%v)", tc.code, got, tc.longFires, g.Long.Failed)
			}
			if got := hasCode(g.Short, tc.code); got != tc.shortFire {
				t.Errorf("short %s fired=%v want %v (failed=%v)", tc.code, got, tc.shortFire, g.Short.Failed)
			}
		})
	}
}

// Codes carry the measured value and the resolved cap, formatted like the
// neighbouring gates.
func TestAntiChaseGateCodeFormat(t *testing.T) {
	market.SetEquityClassificationForTesting(map[string]bool{"AAPLUSDT": true}, map[string]bool{"AAPLUSDT": true})
	defer market.SetEquityClassificationForTesting(nil, nil)

	sig := acSig("TESTUSDT", 107, acf(100), acf(81), 9.5)
	g := computeHardEntryGate(sig, acOpt(6, 3, 78, 6))
	want := []string{"CHASE_EMA20_1H_7.0_GT_6", "RSI1H_EXHAUSTED_81", "CHASE_MOVE4H_9.5_GT_6"}
	joined := strings.Join(g.Long.Failed, ",")
	for _, w := range want {
		if !strings.Contains(joined, w) {
			t.Errorf("long failed codes missing %q: %v", w, g.Long.Failed)
		}
	}
	gs := computeHardEntryGate(acSig("TESTUSDT", 107, acf(100), acf(50), 0), acOpt(6, 3, 78, 6))
	if !strings.Contains(strings.Join(gs.Short.Failed, ","), "COUNTER_EMA20_1H_7.0_GT_3") {
		t.Errorf("short entry 7%% above the EMA must emit COUNTER_EMA20_1H_7.0_GT_3: %v", gs.Short.Failed)
	}
}

// ComputeSymbolSignals must carry market.Data.PriceChange4h onto the signal
// (the CHASE_MOVE4H input) without serializing it to the model.
func TestPriceChange4hReachesSignal(t *testing.T) {
	sig, err := ComputeSymbolSignals("TESTUSDT", &market.Data{Symbol: "TESTUSDT", CurrentPrice: 100, PriceChange4h: 7.5}, SignalOptions{})
	if err != nil || sig == nil {
		t.Fatalf("ComputeSymbolSignals: sig=%v err=%v", sig, err)
	}
	if sig.PriceChange4hPct != 7.5 {
		t.Fatalf("PriceChange4hPct=%v want 7.5", sig.PriceChange4hPct)
	}
}
