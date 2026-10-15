package stockengine

import (
	"math"
	"testing"

	"nofx/market/usstock"
)

func momentumBars(n int, closeAt func(i int) float64) []usstock.Bar {
	bars := make([]usstock.Bar, n)
	for i := range bars {
		bars[i] = usstock.Bar{Close: closeAt(i)}
	}
	return bars
}

func TestClassifyMomentumFormula(t *testing.T) {
	// close[i] = 100+i over 260 bars: n=260, close[n-22]=337, close[n-253]=107... computed explicitly.
	bars := momentumBars(260, func(i int) float64 { return 100 + float64(i) })
	got := ClassifyMomentum(bars)
	want := (bars[260-22].Close/bars[260-253].Close - 1) * 100
	if !got.Valid || math.Abs(got.Mom12_1Pct-want) > 1e-9 {
		t.Fatalf("mom %+v want %.6f", got, want)
	}
	if !got.AboveSMA200 {
		t.Fatalf("rising series must be above its SMA200: %+v", got)
	}
	// Falling series: below the SMA200, positive-denominator momentum negative.
	down := ClassifyMomentum(momentumBars(260, func(i int) float64 { return 400 - float64(i) }))
	if !down.Valid || down.AboveSMA200 || down.Mom12_1Pct >= 0 {
		t.Fatalf("falling series: %+v", down)
	}
}

func TestClassifyMomentumWindowAndSMA(t *testing.T) {
	// Exactly 253 bars is enough; the anchor is bars[0], the skip-month close is bars[231].
	bars := momentumBars(253, func(i int) float64 { return 100 })
	bars[0].Close = 50
	got := ClassifyMomentum(bars)
	if !got.Valid || math.Abs(got.Mom12_1Pct-100) > 1e-9 || got.AboveSMA200 {
		t.Fatalf("253 bars: %+v (flat price is not above its own mean)", got)
	}
	bars[252].Close = 100.01
	if !ClassifyMomentum(bars).AboveSMA200 {
		t.Fatal("last close above the mean of the last 200 closes must set AboveSMA200")
	}
}

func TestClassifyMomentumInvalid(t *testing.T) {
	flat := func(int) float64 { return 100 }
	if got := ClassifyMomentum(momentumBars(252, flat)); got != (MomentumAssessment{}) {
		t.Fatalf("252 bars must be invalid: %+v", got)
	}
	if got := ClassifyMomentum(nil); got != (MomentumAssessment{}) {
		t.Fatalf("nil bars: %+v", got)
	}
	for name, mutate := range map[string]func(b []usstock.Bar){
		"zero anchor":     func(b []usstock.Bar) { b[len(b)-253].Close = 0 },
		"negative recent": func(b []usstock.Bar) { b[len(b)-22].Close = -1 },
		"nan in sma":      func(b []usstock.Bar) { b[len(b)-5].Close = math.NaN() },
		"inf last":        func(b []usstock.Bar) { b[len(b)-1].Close = math.Inf(1) },
		"zero in sma":     func(b []usstock.Bar) { b[len(b)-100].Close = 0 },
	} {
		bars := momentumBars(260, flat)
		mutate(bars)
		if got := ClassifyMomentum(bars); got != (MomentumAssessment{}) {
			t.Fatalf("%s must be invalid with zeroed fields: %+v", name, got)
		}
	}
}
