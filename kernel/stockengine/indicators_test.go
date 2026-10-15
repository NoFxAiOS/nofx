package stockengine

import (
	"math"
	"testing"
	"time"

	"nofx/market/usstock"
)

func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.12f want %.12f", got, want)
	}
}

func TestEMA(t *testing.T) {
	got := ema([]float64{1, 2, 3, 4, 5}, 3)
	for i, want := range []float64{0, 0, 2, 3, 4} {
		near(t, got[i], want)
	}
	near(t, last(ema([]float64{10, 20, 30, 10}, 3)), 15)
	near(t, last(ema([]float64{1, 2}, 3)), 0)
	near(t, last(ema([]float64{1, 2}, 0)), 0)
}

func TestATRWilder(t *testing.T) {
	// TRs: 2,3,4,6,3. SMA seed=3, then Wilder values=4, 11/3.
	bars := []usstock.Bar{
		{High: 11, Low: 9, Close: 10}, {High: 13, Low: 11, Close: 12},
		{High: 16, Low: 14, Close: 15}, {High: 21, Low: 18, Close: 20},
		{High: 20, Low: 17, Close: 18},
	}
	near(t, atr(bars[:3], 3), 3)
	near(t, atr(bars[:4], 3), 4)
	near(t, atr(bars, 3), 11.0/3)
	near(t, atr(bars[:2], 3), 0)
	near(t, atr(bars, 0), 0)
}

func TestConfirmedPivots(t *testing.T) {
	bars := make([]usstock.Bar, 10)
	for i := range bars {
		bars[i] = usstock.Bar{Low: 10, High: 20}
	}
	bars[3].Low, bars[3].High = 5, 25
	bars[8].Low, bars[8].High = 1, 30 // not confirmed: only one bar to the right
	lo, hi := pivots(bars, 3)
	near(t, lo, 5)
	near(t, hi, 25)
	bars[2].Low, bars[2].High = 5, 25 // ties are not pivots
	lo, hi = pivots(bars, 3)
	near(t, lo, 0)
	near(t, hi, 0)
	lo, hi = pivots(bars[:3], 3)
	near(t, lo, 0)
	near(t, hi, 0)
}

func risingSeries(tf, source string, n int) *usstock.Series {
	s := &usstock.Series{Symbol: "AAPLBUSDT", Underlying: "AAPL", Timeframe: tf, Source: source}
	for i := 0; i < n; i++ {
		price := 100 + float64(i)
		s.Bars = append(s.Bars, usstock.Bar{OpenTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i), Open: price, High: price + 1, Low: price - 1, Close: price, Volume: 100})
	}
	return s
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func TestBuildSnapshot(t *testing.T) {
	series := map[string]*usstock.Series{
		usstock.TF1d: risingSeries(usstock.TF1d, usstock.SourceYahoo, 260),
		usstock.TF1w: risingSeries(usstock.TF1w, usstock.SourceYahoo, 30),
		usstock.TF1h: risingSeries(usstock.TF1h, usstock.SourceBStock, 60),
	}
	quote := &usstock.Quote{BStockPrice: 400, RefFresh: true}
	s := BuildSnapshot("AAPLBUSDT", "AAPL", series, quote, time.Time{})
	if s.TrendDaily != "up" || s.TrendWeekly != "up" || s.TrendEntry != "up" {
		t.Fatalf("trends: %+v", s)
	}
	near(t, s.Price, 400)
	near(t, s.EMA20, 349.5)
	near(t, s.EMA50, 334.5)
	near(t, s.EMA200, 259.5)
	near(t, s.ATR1d, 2)
	near(t, s.ATR1dPct, 2.0/359*100)
	near(t, s.High52w, 360)
	near(t, s.Low52w, 107)
	near(t, s.PctFrom52wHigh, (359.0/360-1)*100)
	near(t, s.VolumeRatio20_50, 1)
	near(t, s.Return20dPct, (359.0/339-1)*100)
	if s.Sources[usstock.TF1d] != usstock.SourceYahoo || s.Sources[usstock.TF1h] != usstock.SourceBStock {
		t.Fatal(s.Sources)
	}
	if contains(s.MissingTF, usstock.TF1d) || contains(s.MissingTF, usstock.TF1h) || contains(s.MissingTF, usstock.TF1w) {
		t.Fatal(s.MissingTF)
	}
	if !contains(s.MissingTF, usstock.TF15m) || !contains(s.MissingTF, usstock.TF4h) {
		t.Fatal(s.MissingTF)
	}
	near(t, BuildSnapshot("AAPLBUSDT", "AAPL", series, nil, time.Time{}).Price, 159)
}

func TestSnapshotShortDailyBlocksDecision(t *testing.T) {
	ctx := testContext()
	series := map[string]*usstock.Series{usstock.TF1d: risingSeries(usstock.TF1d, usstock.SourceBStock, 73), usstock.TF1h: risingSeries(usstock.TF1h, usstock.SourceBStock, 60)}
	s := BuildSnapshot("AAPLBUSDT", "AAPL", series, &usstock.Quote{BStockPrice: 100, RefFresh: true}, ctx.Now)
	if !contains(s.MissingTF, usstock.TF1d) || s.EMA200 != 0 || s.TrendDaily != "unknown" || s.EMA50 == 0 || s.EMA20 == 0 {
		t.Fatalf("%+v", s)
	}
	ctx.Snapshots = []*SymbolSnapshot{s}
	assertCode(t, Validate(ctx, []Decision{buy("AAPLBUSDT")})[0], "DATA_INSUFFICIENT")
}

func TestSnapshotMissingAndPositionEntry(t *testing.T) {
	s := BuildSnapshot("A", "A", nil, nil, time.Time{})
	if len(s.MissingTF) != 5 || s.Price != 0 || s.TrendWeekly != "unknown" {
		t.Fatalf("%+v", s)
	}
	series := map[string]*usstock.Series{usstock.TF1d: risingSeries(usstock.TF1d, usstock.SourceYahoo, 60)}
	s = BuildSnapshot("A", "A", series, nil, time.Time{})
	if s.TrendEntry != "up" {
		t.Fatal(s.TrendEntry)
	}
	near(t, s.Price, 159)
	near(t, s.High52w, 160)
	near(t, s.Low52w, 99)
	series[usstock.TF1w] = risingSeries(usstock.TF1w, usstock.SourceYahoo, 29)
	s = BuildSnapshot("A", "A", series, nil, time.Time{})
	if s.TrendWeekly != "unknown" || !contains(s.MissingTF, usstock.TF1w) {
		t.Fatal(s)
	}
}

func TestSnapshotDownAndRange(t *testing.T) {
	daily := risingSeries(usstock.TF1d, usstock.SourceYahoo, 200)
	for i := range daily.Bars {
		p := 400 - float64(i)
		daily.Bars[i] = usstock.Bar{Close: p, High: p + 1, Low: p - 1}
	}
	series := map[string]*usstock.Series{usstock.TF1d: daily}
	if s := BuildSnapshot("A", "A", series, nil, time.Time{}); s.TrendDaily != "down" {
		t.Fatal(s.TrendDaily)
	}
	for i := range daily.Bars {
		daily.Bars[i].Close = 100
	}
	if s := BuildSnapshot("A", "A", series, nil, time.Time{}); s.TrendDaily != "range" {
		t.Fatal(s.TrendDaily)
	}
}

func TestRSIWilder(t *testing.T) {
	// StockCharts RSI worked example: the first RSI(14) is 70.53, the next 66.32.
	closes := []float64{44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28}
	if got := rsi(closes, 14); math.Abs(got-70.53) > 0.1 {
		t.Fatalf("rsi first = %.2f want 70.53", got)
	}
	closes = append(closes, 46.00)
	if got := rsi(closes, 14); math.Abs(got-66.32) > 0.1 {
		t.Fatalf("rsi second = %.2f want 66.32", got)
	}
	// Undefined below period+1 values; all gains → 100; all losses → 0.
	near(t, rsi(closes[:14], 14), 50)
	near(t, rsi(nil, 14), 50)
	up, down := make([]float64, 20), make([]float64, 20)
	for i := range up {
		up[i], down[i] = float64(10+i), float64(50-i)
	}
	near(t, rsi(up, 14), 100)
	near(t, rsi(down, 14), 0)
	near(t, rsi(up, 0), 50)
}
