package breakout

import (
	"math"
	"testing"
)

// Synthetic 4h series in a clean downtrend: price below a falling EMA20,
// EMA20 below EMA50.
func downtrend4h(n int, start float64) []Kline {
	k := make([]Kline, n)
	price := start
	for i := 0; i < n; i++ {
		price *= 0.995 // steady bleed
		k[i] = Kline{OpenTime: int64(i) * 4 * 3600 * 1000, Open: price * 1.001, High: price * 1.002, Low: price * 0.998, Close: price}
	}
	return k
}

// Ranging/uptrend series for the gate to reject.
func uptrend4h(n int, start float64) []Kline {
	k := make([]Kline, n)
	price := start
	for i := 0; i < n; i++ {
		price *= 1.004
		k[i] = Kline{OpenTime: int64(i) * 4 * 3600 * 1000, Open: price * 0.999, High: price * 1.002, Low: price * 0.998, Close: price}
	}
	return k
}

type fakeDS struct {
	DataSource
	k1h []Kline
	k4h []Kline
}

func (f fakeDS) Klines(interval string, n int) ([]Kline, error) {
	if interval == "1h" {
		return f.k1h, nil
	}
	return f.k4h, nil
}

func (fakeDS) FundingInfo() (*FundingInfo, error) { return nil, nil }
func (fakeDS) SpotKlines(interval string, limit int) ([]Kline, error) {
	return nil, errNoFake
}
func (fakeDS) Depth1Pct() (*DepthSnapshot, error) { return nil, errNoFake }
func (fakeDS) FundingHistory(n int) ([]FundingPoint, error) { return nil, errNoFake }
func (fakeDS) OIHistory(interval string, n int) ([]OIPoint, error) {
	return nil, errNoFake
}
func (fakeDS) LongShortRatio(interval string, n int) ([]LongShortPoint, error) {
	return nil, errNoFake
}

var errNoFake = errFake{}

type errFake struct{}

func (errFake) Error() string { return "not available in fake" }

// The downtrend gate: an established downtrend passes, an uptrend coin that
// merely printed one bad 24h candle is NOT a breakdown-continuation
// candidate (analyzeBreakdownShort returns nil,nil).
func TestBreakdownDowntrendGate(t *testing.T) {
	down := downtrend4h(84, 100)
	up := uptrend4h(84, 100)
	// 1h series mirrors the 4h shape (AnalyzeShort needs ≥30 bars).
	mk1h := func(src []Kline) []Kline {
		out := make([]Kline, 60)
		for i := range out {
			srcK := src[len(src)-61+i]
			out[i] = Kline{OpenTime: srcK.OpenTime, Open: srcK.Open, High: srcK.High, Low: srcK.Low, Close: srcK.Close}
		}
		return out
	}

	sig, err := analyzeBreakdownShort("DROPUSDT", -12, down, fakeDS{k1h: mk1h(down), k4h: down})
	if err != nil {
		t.Fatalf("downtrend coin must be analyzed: %v", err)
	}
	if sig == nil {
		t.Fatal("downtrend coin must pass the gate")
	}
	if sig.Universe != "breakdown" {
		t.Fatalf("universe = %s, want breakdown", sig.Universe)
	}
	if sig.Score <= 0 || sig.Score > 100 {
		t.Fatalf("score %.2f out of range", sig.Score)
	}

	sigUp, err := analyzeBreakdownShort("PUMPUSDT", -12, up, fakeDS{k1h: mk1h(up), k4h: up})
	if err != nil {
		t.Fatalf("uptrend analysis failed: %v", err)
	}
	if sigUp != nil {
		t.Fatalf("uptrend coin must be gate-skipped, got score %.0f", sigUp.Score)
	}
}

// The composite is transparent: with NO structure evidence (no MA break, no
// fake breakout, no divergence) the score is dominated by room-to-fall and
// MUST NOT grade strong — an unconfirmed weak-looking coin is "looks weak",
// not a confirmed short (same discipline as the pump side).
func TestBreakdownUnconfirmedNotStrong(t *testing.T) {
	down := downtrend4h(84, 100)
	mk1h := func(src []Kline) []Kline {
		out := make([]Kline, 60)
		for i := range out {
			srcK := src[len(src)-61+i]
			out[i] = Kline{OpenTime: srcK.OpenTime, Open: srcK.Open, High: srcK.High, Low: srcK.Low, Close: srcK.Close}
		}
		return out
	}
	sig, err := analyzeBreakdownShort("DROPUSDT", -12, down, fakeDS{k1h: mk1h(down), k4h: down})
	if err != nil || sig == nil {
		t.Fatalf("analysis failed: %v %v", sig, err)
	}
	if !sig.Confirmed {
		if sig.Grade == "strong" {
			t.Fatalf("unconfirmed breakdown must not grade strong (score %.0f)", sig.Score)
		}
	} else if math.IsNaN(sig.Score) {
		t.Fatal("NaN score")
	}
}

func TestBreakdownContinuation(t *testing.T) {
	// 1h: slow bleed, then a bounce that pokes above EMA20 and closes under, then a fresh low.
	k1h := make([]Kline, 60)
	p := 100.0
	for i := range k1h {
		p *= 0.998
		k1h[i] = Kline{Open: p * 1.001, High: p * 1.002, Low: p * 0.998, Close: p}
	}
	// bar 52: spike above EMA20 and close back below it
	k1h[52].High = k1h[52].Close * 1.05
	// last bar closes below every prior low
	k1h[59].Close = k1h[59].Low * 0.97
	k1h[59].Low = k1h[59].Close
	k4h := downtrend4h(84, 100)
	score, printed := breakdownContinuation(k1h, k4h)
	if !printed {
		t.Fatalf("bounce failure + fresh low must print, score %.0f", score)
	}
	if score < 70 {
		t.Fatalf("score %.0f too low for full continuation", score)
	}
	// flat chop: nothing printed
	flat := make([]Kline, 60)
	for i := range flat {
		flat[i] = Kline{Open: 100, High: 100.1, Low: 99.9, Close: 100}
	}
	if _, pr := breakdownContinuation(flat, k4h); pr {
		t.Fatal("flat series must not print continuation")
	}
}
