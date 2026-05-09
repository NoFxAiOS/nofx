package kernel

import (
	"strings"
	"testing"
	"time"

	"nofx/market"
	"nofx/store"
)

func TestFormatStructuralLevelsUsesMachineReadableSeparatedPrices(t *testing.T) {
	mdata := &market.Data{
		StructuralLevels: []market.StructuralLevel{
			{Price: 76754.19, Type: "resistance", Timeframe: "15m", Strength: 3, Source: "fibonacci"},
			{Price: 76887.05, Type: "resistance", Timeframe: "15m", Strength: 2, Source: "swing_point"},
			{Price: 76000.50, Type: "support", Timeframe: "15m", Strength: 4, Source: "swing_point"},
		},
		FibonacciLevels: &market.FibonacciLevels{
			SwingLow:  76000.50,
			SwingHigh: 77209.06,
			Timeframe: "15m",
			Direction: "retracement_down",
			Levels: map[string]float64{
				"0.5":   76604.78,
				"0.618": 76747.39,
			},
		},
	}

	out := formatStructuralLevelsEN(mdata)
	for _, want := range []string{
		"resistance_levels:",
		"level_1_price=76754.19",
		"level_2_price=76887.05",
		"fibonacci_context: timeframe=15m swing_low=76000.5 swing_high=77209.06",
		"fib_0.5=76604.78",
		"fib_0.618=76747.39",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "76754.19,76887.05") || strings.Contains(out, " | ") {
		t.Fatalf("expected separated machine-readable levels, got:\n%s", out)
	}
}

func TestFormatMarketDataUsesPlainMachineReadableNumbers(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.Indicators.EnableEMA = true
	cfg.Indicators.EnableMACD = true
	cfg.Indicators.EnableRSI = true
	cfg.Indicators.EnableOI = true
	cfg.Indicators.EnableFundingRate = true
	engine := NewStrategyEngine(&cfg)
	mdata := &market.Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 76754.190000,
		CurrentEMA20: 76887.050000,
		CurrentMACD:  -12.300000,
		CurrentRSI7:  41.500000,
		OpenInterest: &market.OIData{Latest: 123456789.120000, Average: 123000000.000000},
		FundingRate:  0.00012340,
	}

	out := engine.formatMarketData(mdata)
	for _, want := range []string{
		"current_price = 76754.19",
		"current_ema20 = 76887.05",
		"current_macd = -12.3",
		"current_rsi7 = 41.5",
		"Open Interest: latest=123456789.12 average=123000000",
		"Funding Rate: 0.0001234",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestFormatFloatSliceUsesPlainDecimals(t *testing.T) {
	got := formatFloatSlice([]float64{76754.190000, 76887.050000, 0.00012340})
	want := "[76754.19, 76887.05, 0.0001234]"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestBuildUserPromptUsesPlainNumbersForAccountAndPositions(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	engine := NewStrategyEngine(&cfg)
	ctx := &Context{
		CurrentTime:    "2026-04-28 12:00:00",
		RuntimeMinutes: 42,
		CallCount:      7,
		Account: AccountInfo{
			TotalEquity:      1000.500000,
			AvailableBalance: 250.125000,
			TotalPnLPct:      -1.230000,
			MarginUsedPct:    87.500000,
			PositionCount:    1,
		},
		Positions: []PositionInfo{{
			Symbol:           "BTCUSDT",
			Side:             "short",
			EntryPrice:       76754.190000,
			MarkPrice:        76887.050000,
			Quantity:         0.001000,
			Leverage:         10,
			UnrealizedPnL:    -1.234000,
			UnrealizedPnLPct: -0.160000,
			PeakPnLPct:       0.250000,
			LiquidationPrice: 80000.000000,
			MarginUsed:       7.688705,
			UpdateTime:       time.Now().Add(-10 * time.Minute).UnixMilli(),
		}},
		MarketDataMap: map[string]*market.Data{},
	}

	out := engine.BuildUserPrompt(ctx)
	for _, want := range []string{
		"Account: Equity 1000.5 | Balance 250.125 (25%) | PnL -1.23% | Margin 87.5% | Positions 1",
		"1. BTCUSDT SHORT | Entry 76754.19 Current 76887.05 | Qty 0.001 | Position Value 76.88705 USDT | PnL-0.16% | PnL Amount-1.234 USDT | Peak PnL0.25% | Leverage 10x | Margin 7.688705 | Liq Price 80000",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, out)
		}
	}
}
