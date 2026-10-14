package stockengine

import (
	"math"
	"reflect"
	"testing"
	"time"

	"nofx/market/usstock"
)

// Gaps are allowed: each synthetic daily session opens at its close, with a
// configurable volume and a fixed intraday half-range of 0.4.
func cycleTestBars(path []float64, volumes map[int]float64) []usstock.Bar {
	bars := make([]usstock.Bar, len(path))
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for j, price := range path {
		volume := 100.0
		if v, ok := volumes[j]; ok {
			volume = v
		}
		bars[j] = usstock.Bar{OpenTime: start.AddDate(0, 0, j), Open: price, High: price + 0.4, Low: price - 0.4, Close: price, Volume: volume}
	}
	return bars
}

func cycleTestPath(n int, start, step float64) []float64 {
	path := make([]float64, n)
	for j := range path {
		path[j] = start + step*float64(j)
	}
	return path
}

func cycleTestPop() []usstock.Bar {
	path := append(cycleTestPath(80, 100, 0), cycleTestPath(10, 99, 0)...)
	path = append(path, 100.4)
	return cycleTestBars(path, map[int]float64{len(path) - 1: 200})
}

func cycleTestCrossback() []usstock.Bar {
	path := closes(cycleTestPop())
	path = append(path, 100.9, 101.2, 101.1, 101.2, 101.3, 101.0, 100.3)
	return cycleTestBars(path, map[int]float64{90: 200, len(path) - 1: 150})
}

func cycleTestBreak() []usstock.Bar {
	path := append(cycleTestPath(220, 90, 0.05), cycleTestPath(30, 101, 0)...)
	path = append(path, 101.8)
	return cycleTestBars(path, map[int]float64{len(path) - 1: 200})
}

func cycleTestReversal() []usstock.Bar {
	path := append(cycleTestPath(80, 100, 0), cycleTestPath(20, 100.3, 0.3)...)
	path = append(path, 96, 102.9)
	return cycleTestBars(path, map[int]float64{len(path) - 2: 200})
}

func TestClassifyCyclePhases(t *testing.T) {
	exhaustion := append(cycleTestPath(80, 100, 0), 104, 107, 110)
	drop := append(cycleTestPath(80, 100, 0), 101, 101.5, 101.7, 101.6, 100)
	down := append(cycleTestPath(80, 120, 0), cycleTestPath(40, 119.75, -0.25)...)
	forming := append(cycleTestPath(220, 90, 0.05), cycleTestPath(10, 101, 0)...)
	zeroATR := cycleTestBars(cycleTestPath(80, 100, 0), nil)
	for j := range zeroATR {
		zeroATR[j].High, zeroATR[j].Low = 100, 100
	}
	cases := []struct {
		name  string
		bars  []usstock.Bar
		phase CyclePhase
		long  bool
	}{
		{"wedge pop", cycleTestPop(), PhaseWedgePop, true},
		{"EMA crossback", cycleTestCrossback(), PhaseEMACrossback, true},
		{"base n break", cycleTestBreak(), PhaseBaseNBreak, true},
		{"reversal extension", cycleTestReversal(), PhaseReversalExtension, true},
		{"exhaustion extension", cycleTestBars(exhaustion, nil), PhaseExhaustionExtension, false},
		{"wedge drop", cycleTestBars(drop, map[int]float64{len(drop) - 1: 200}), PhaseWedgeDrop, false},
		{"downtrend", cycleTestBars(down, nil), PhaseDowntrend, false},
		{"base forming", cycleTestBars(forming, nil), PhaseBaseForming, true},
		{"uptrend", cycleTestBars(cycleTestPath(100, 100, 0.2), nil), PhaseUptrend, false},
		{"neutral", cycleTestBars(cycleTestPath(80, 100, 0), nil), PhaseNeutral, false},
		{"short", cycleTestBars(cycleTestPath(59, 100, 0), nil), PhaseInsufficient, false},
		{"empty", nil, PhaseInsufficient, false},
		{"zero ATR", zeroATR, PhaseInsufficient, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := append([]usstock.Bar(nil), tc.bars...)
			got := ClassifyCycle(tc.bars, nil)
			if got.Phase != tc.phase || got.LongSetup != tc.long {
				t.Fatalf("got %+v; want phase %s long=%v", got, tc.phase, tc.long)
			}
			if !tc.long && got.Score != 0 {
				t.Fatalf("non-entry has score %.1f", got.Score)
			}
			if got.Bars != len(tc.bars) || got.Score < 0 || got.Score > 100 || len(got.Notes) < 1 || len(got.Notes) > 3 {
				t.Fatalf("invalid assessment: %+v", got)
			}
			if !reflect.DeepEqual(original, tc.bars) {
				t.Fatal("classification mutated input")
			}
		})
	}
}

func TestClassifyCycleScoreOrdering(t *testing.T) {
	strong := cycleTestPop()
	weak := append([]usstock.Bar(nil), strong...)
	weak[len(weak)-1].Volume = 100
	strongSPY := cycleTestBars(cycleTestPath(len(strong), 200, -1), nil)
	weakSPY := cycleTestBars(cycleTestPath(len(weak), 100, 1), nil)
	a, b := ClassifyCycle(strong, strongSPY), ClassifyCycle(weak, weakSPY)
	if a.Phase != PhaseWedgePop || b.Phase != PhaseWedgePop || a.Score <= b.Score || a.RS63Pct <= b.RS63Pct {
		t.Fatalf("strong=%+v weak=%+v", a, b)
	}
	// A 2x event must stay 2x; including the spike in its own average would
	// produce a smaller ratio and could lose a volume score at a threshold.
	near(t, a.VolRatio, 2)
	near(t, b.VolRatio, 1)
	if math.Abs(a.Score*10-math.Round(a.Score*10)) > 1e-9 {
		t.Fatal("score is not rounded to one decimal")
	}
}

func TestClassifyCycleRSDateAlignment(t *testing.T) {
	stock := cycleTestBars(cycleTestPath(90, 100, 0.2), nil)
	spy := cycleTestBars(cycleTestPath(80, 200, 0.5), nil)
	for j := range spy {
		spy[j].OpenTime = stock[j+5].OpenTime.Add(14*time.Hour + 30*time.Minute)
	}
	// Missing date 25 makes the 64 matched sessions run from date 20 to 84.
	spy = append(spy[:20:20], spy[21:]...)
	want := (stock[84].Close/stock[20].Close - 239.5/207.5) * 100
	got := ClassifyCycle(stock, spy)
	near(t, got.RS63Pct, want)
	near(t, ClassifyCycle(stock, nil).RS63Pct, 0)
	near(t, ClassifyCycle(stock[:63], spy).RS63Pct, 0)
	near(t, ClassifyCycle(stock, spy[:63]).RS63Pct, 0)
	for j := range spy {
		spy[j].OpenTime = spy[j].OpenTime.AddDate(2, 0, 0)
	}
	near(t, ClassifyCycle(stock, spy).RS63Pct, 0)
}

func TestClassifyCycleHistoricalEventVolume(t *testing.T) {
	pop := cycleTestPop()
	path := append(closes(pop), 100.5, 100.6)
	bars := cycleTestBars(path, map[int]float64{len(pop) - 1: 200, len(path) - 1: 50})
	got := ClassifyCycle(bars, nil)
	if got.Phase != PhaseWedgePop {
		t.Fatalf("recent pop: %+v", got)
	}
	near(t, got.VolRatio, 2)

	// A reversal can be confirmed by an adjacent volume bar. Its reported
	// ratio still belongs to the capitulation event itself.
	reversal := cycleTestReversal()
	reversal[len(reversal)-2].Volume = 100
	reversal[len(reversal)-1].Volume = 200
	got = ClassifyCycle(reversal, nil)
	if got.Phase != PhaseReversalExtension {
		t.Fatalf("adjacent capitulation volume: %+v", got)
	}
	near(t, got.VolRatio, 1)
}

func TestClassifyCycleBearishPriority(t *testing.T) {
	// A recent blow-off still vetoes the recovering bar, even when that bar
	// sits near its EMA support and otherwise resembles a long entry.
	path := append(cycleTestPath(80, 100, 0), 106, 104, 103)
	got := ClassifyCycle(cycleTestBars(path, nil), nil)
	if got.Phase != PhaseExhaustionExtension || got.LongSetup || got.Score != 0 {
		t.Fatalf("exhaustion priority: %+v", got)
	}
	// Selling confirmation belongs to the first loss bar, not the latest.
	path = append(cycleTestPath(80, 100, 0), 101, 101.5, 101.7, 101.6, 100, 100)
	bars := cycleTestBars(path, map[int]float64{len(path) - 2: 200, len(path) - 1: 50})
	got = ClassifyCycle(bars, nil)
	if got.Phase != PhaseWedgeDrop || got.LongSetup || got.Score != 0 {
		t.Fatalf("drop priority: %+v", got)
	}
	near(t, got.VolRatio, 2)
}
