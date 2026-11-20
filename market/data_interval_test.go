package market

import (
	"strings"
	"testing"
)

func seriesWithCloses(n int, close func(i int) float64) []Kline {
	klines := make([]Kline, n)
	for i := range klines {
		klines[i] = Kline{Close: close(i)}
	}
	return klines
}

func TestIntradayPriceChange1hUsesIntervalBarCount(t *testing.T) {
	// Close price equals the bar index, so the lookback is visible in the result.
	klines := seriesWithCloses(100, func(i int) float64 { return float64(i + 1) })
	last := 100.0

	// 3m: one hour is 20 bars back -> close 80.
	want3m := (last - 80) / 80 * 100
	if got := intradayPriceChange1h(klines, "3m"); got != want3m {
		t.Fatalf("3m 1h change = %v, want %v", got, want3m)
	}

	// 5m: one hour is 12 bars back -> close 88 (not 20 bars / 100 minutes).
	want5m := (last - 88) / 88 * 100
	if got := intradayPriceChange1h(klines, "5m"); got != want5m {
		t.Fatalf("5m 1h change = %v, want %v", got, want5m)
	}
}

func TestIntradayPriceChange1hNeedsAFullHour(t *testing.T) {
	klines := seriesWithCloses(12, func(i int) float64 { return float64(i + 1) })
	if got := intradayPriceChange1h(klines, "5m"); got != 0 {
		t.Fatalf("12 bars of 5m cannot cover an hour, got %v", got)
	}
	if got := intradayPriceChange1h(klines, "bogus"); got != 0 {
		t.Fatalf("unknown interval should yield 0, got %v", got)
	}
}

func TestFormatLabelsIntradayInterval(t *testing.T) {
	series := &IntradayData{MidPrices: []float64{1, 2}, ATR14: 1.5}

	series.Interval = "5m"
	out := Format(&Data{Symbol: "BTC", IntradaySeries: series})
	if !strings.Contains(out, "5‑minute intervals") || !strings.Contains(out, "5m ATR") {
		t.Fatalf("5m series must be labelled as 5-minute:\n%s", out)
	}
	if strings.Contains(out, "3‑minute") || strings.Contains(out, "3m ATR") {
		t.Fatalf("5m series must not be labelled as 3-minute:\n%s", out)
	}

	// Legacy data without an interval keeps the 3m label.
	series.Interval = ""
	out = Format(&Data{Symbol: "BTC", IntradaySeries: series})
	if !strings.Contains(out, "3‑minute intervals") || !strings.Contains(out, "3m ATR") {
		t.Fatalf("default series must keep the 3-minute label:\n%s", out)
	}
}
