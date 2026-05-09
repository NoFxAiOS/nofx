package market

import (
	"testing"
)

func makeKlines(prices []float64) []Kline {
	klines := make([]Kline, len(prices))
	for i, p := range prices {
		klines[i] = Kline{
			OpenTime: int64(i * 60000),
			Open:     p * 0.999,
			High:     p * 1.01,
			Low:      p * 0.99,
			Close:    p,
			Volume:   1000,
		}
	}
	return klines
}

func TestCalculateFibonacciLevels_Uptrend(t *testing.T) {
	// Simulate uptrend: prices go from 100 to 200
	prices := make([]float64, 30)
	for i := range prices {
		prices[i] = 100 + float64(i)*3.5
	}
	klines := makeKlines(prices)

	fib := CalculateFibonacciLevels(klines, "1h")
	if fib == nil {
		t.Fatal("expected non-nil fibonacci levels")
	}

	if fib.Direction != "retracement_down" {
		t.Errorf("expected retracement_down for uptrend, got %s", fib.Direction)
	}

	if fib.SwingHigh <= fib.SwingLow {
		t.Errorf("swing high (%f) should be > swing low (%f)", fib.SwingHigh, fib.SwingLow)
	}

	// Check all expected levels exist
	expectedKeys := []string{"0.236", "0.382", "0.5", "0.618", "0.786"}
	for _, k := range expectedKeys {
		if _, ok := fib.Levels[k]; !ok {
			t.Errorf("missing fibonacci level %s", k)
		}
	}

	// In retracement_down, 0.236 level should be closer to high, 0.786 closer to low
	if fib.Levels["0.236"] < fib.Levels["0.786"] {
		t.Error("0.236 level should be above 0.786 in retracement_down")
	}
}

func TestCalculateFibonacciLevels_Downtrend(t *testing.T) {
	prices := make([]float64, 30)
	for i := range prices {
		prices[i] = 200 - float64(i)*3.5
	}
	klines := makeKlines(prices)

	fib := CalculateFibonacciLevels(klines, "4h")
	if fib == nil {
		t.Fatal("expected non-nil fibonacci levels")
	}

	if fib.Direction != "retracement_up" {
		t.Errorf("expected retracement_up for downtrend, got %s", fib.Direction)
	}

	// In retracement_up, 0.236 should be closer to low, 0.786 closer to high
	if fib.Levels["0.236"] > fib.Levels["0.786"] {
		t.Error("0.236 level should be below 0.786 in retracement_up")
	}
}

func TestCalculateFibonacciLevels_TooFewKlines(t *testing.T) {
	klines := makeKlines([]float64{100, 101, 102})
	fib := CalculateFibonacciLevels(klines, "1h")
	if fib != nil {
		t.Error("expected nil for too few klines")
	}
}

func TestFindSwingHighsAndLows(t *testing.T) {
	// Create a wave pattern: up-down-up-down
	prices := []float64{
		100, 102, 105, 108, 110, // up
		108, 105, 102, 100, 98, // down
		100, 103, 106, 109, 112, // up
		110, 107, 104, 101, 99, // down
	}
	klines := makeKlines(prices)

	highs := findSwingHighs(klines, 3)
	lows := findSwingLows(klines, 3)

	if len(highs) == 0 {
		t.Error("expected at least one swing high")
	}
	if len(lows) == 0 {
		t.Error("expected at least one swing low")
	}
}
