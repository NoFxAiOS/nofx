package trader

import (
	"math"
	"testing"
	"time"
)

// testTime returns a deterministic timestamp offset by n minutes.
func testTime(n int) time.Time {
	return time.Date(2026, 1, 2, 10, n, 0, 0, time.UTC)
}

func floatsClose(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestRebuildPositionsFromTrades_EmptyInput(t *testing.T) {
	if got := RebuildPositionsFromTrades(nil); got != nil {
		t.Errorf("RebuildPositionsFromTrades(nil) = %v, want nil", got)
	}
	if got := RebuildPositionsFromTrades([]TradeRecord{}); got != nil {
		t.Errorf("RebuildPositionsFromTrades([]) = %v, want nil", got)
	}
}

func TestRebuildPositionsFromTrades_SimpleLongOpenClose(t *testing.T) {
	trades := []TradeRecord{
		{
			TradeID:  "t1",
			Symbol:   "BTCUSDT",
			Side:     "BUY",
			Price:    100.0,
			Quantity: 1.0,
			Fee:      0.1,
			Time:     testTime(0),
		},
		{
			TradeID:     "t2",
			Symbol:      "BTCUSDT",
			Side:        "SELL",
			Price:       110.0,
			Quantity:    1.0,
			RealizedPnL: 10.0,
			Fee:         0.2,
			Time:        testTime(1),
		},
	}

	records := RebuildPositionsFromTrades(trades)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	if r.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %q, want BTCUSDT", r.Symbol)
	}
	if r.Side != "long" {
		t.Errorf("Side = %q, want long", r.Side)
	}
	if !floatsClose(r.EntryPrice, 100.0) {
		t.Errorf("EntryPrice = %v, want 100", r.EntryPrice)
	}
	if !floatsClose(r.ExitPrice, 110.0) {
		t.Errorf("ExitPrice = %v, want 110", r.ExitPrice)
	}
	if !floatsClose(r.Quantity, 1.0) {
		t.Errorf("Quantity = %v, want 1", r.Quantity)
	}
	if !floatsClose(r.RealizedPnL, 10.0) {
		t.Errorf("RealizedPnL = %v, want 10", r.RealizedPnL)
	}
	// Fee should be entry fee + exit fee.
	if !floatsClose(r.Fee, 0.3) {
		t.Errorf("Fee = %v, want 0.3 (entry 0.1 + exit 0.2)", r.Fee)
	}
	if !r.EntryTime.Equal(testTime(0)) {
		t.Errorf("EntryTime = %v, want %v", r.EntryTime, testTime(0))
	}
	if !r.ExitTime.Equal(testTime(1)) {
		t.Errorf("ExitTime = %v, want %v", r.ExitTime, testTime(1))
	}
	if r.OrderID != "t2" || r.ExchangeID != "t2" {
		t.Errorf("OrderID/ExchangeID = %q/%q, want t2/t2", r.OrderID, r.ExchangeID)
	}
	if r.CloseType != "unknown" {
		t.Errorf("CloseType = %q, want unknown", r.CloseType)
	}
}

func TestRebuildPositionsFromTrades_PartialClose(t *testing.T) {
	trades := []TradeRecord{
		{
			TradeID:  "open1",
			Symbol:   "ETHUSDT",
			Side:     "BUY",
			Price:    100.0,
			Quantity: 2.0,
			Fee:      0.4,
			Time:     testTime(0),
		},
		{
			TradeID:     "close1",
			Symbol:      "ETHUSDT",
			Side:        "SELL",
			Price:       110.0,
			Quantity:    1.0,
			RealizedPnL: 10.0,
			Fee:         0.1,
			Time:        testTime(1),
		},
		{
			TradeID:     "close2",
			Symbol:      "ETHUSDT",
			Side:        "SELL",
			Price:       120.0,
			Quantity:    1.0,
			RealizedPnL: 20.0,
			Fee:         0.1,
			Time:        testTime(2),
		},
	}

	records := RebuildPositionsFromTrades(trades)
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	for i, r := range records {
		if r.Side != "long" {
			t.Errorf("records[%d].Side = %q, want long", i, r.Side)
		}
		// FIFO: both partial closes consume the single open at 100.
		if !floatsClose(r.EntryPrice, 100.0) {
			t.Errorf("records[%d].EntryPrice = %v, want 100", i, r.EntryPrice)
		}
		if !floatsClose(r.Quantity, 1.0) {
			t.Errorf("records[%d].Quantity = %v, want 1", i, r.Quantity)
		}
		if !r.EntryTime.Equal(testTime(0)) {
			t.Errorf("records[%d].EntryTime = %v, want %v", i, r.EntryTime, testTime(0))
		}
	}

	if !floatsClose(records[0].ExitPrice, 110.0) {
		t.Errorf("records[0].ExitPrice = %v, want 110", records[0].ExitPrice)
	}
	if !floatsClose(records[1].ExitPrice, 120.0) {
		t.Errorf("records[1].ExitPrice = %v, want 120", records[1].ExitPrice)
	}

	// First partial close: exit fee 0.1 + proportional entry fee 0.4*(1/2) = 0.3.
	if !floatsClose(records[0].Fee, 0.3) {
		t.Errorf("records[0].Fee = %v, want 0.3", records[0].Fee)
	}
	// Second partial close consumes the remaining half of the open trade:
	// exit fee 0.1 + remaining entry fee 0.2 = 0.3. Total entry fee attributed
	// across both closes must equal the 0.4 actually paid.
	if !floatsClose(records[1].Fee, 0.3) {
		t.Errorf("records[1].Fee = %v, want 0.3", records[1].Fee)
	}
	totalEntryFee := records[0].Fee + records[1].Fee - 0.2 // subtract the two exit fees
	if !floatsClose(totalEntryFee, 0.4) {
		t.Errorf("total attributed entry fee = %v, want 0.4 (fee actually paid)", totalEntryFee)
	}
}

func TestRebuildPositionsFromTrades_MultipleOpensWeightedEntry(t *testing.T) {
	trades := []TradeRecord{
		{
			TradeID:  "open1",
			Symbol:   "BTCUSDT",
			Side:     "BUY",
			Price:    100.0,
			Quantity: 1.0,
			Fee:      0.1,
			Time:     testTime(0),
		},
		{
			TradeID:  "open2",
			Symbol:   "BTCUSDT",
			Side:     "BUY",
			Price:    110.0,
			Quantity: 1.0,
			Fee:      0.1,
			Time:     testTime(1),
		},
		{
			TradeID:     "close1",
			Symbol:      "BTCUSDT",
			Side:        "SELL",
			Price:       120.0,
			Quantity:    2.0,
			RealizedPnL: 30.0,
			Fee:         0.2,
			Time:        testTime(2),
		},
	}

	records := RebuildPositionsFromTrades(trades)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	// Weighted average: (100*1 + 110*1) / 2 = 105.
	if !floatsClose(r.EntryPrice, 105.0) {
		t.Errorf("EntryPrice = %v, want 105", r.EntryPrice)
	}
	if !floatsClose(r.Quantity, 2.0) {
		t.Errorf("Quantity = %v, want 2", r.Quantity)
	}
	// Exit fee 0.2 + both entry fees 0.1 + 0.1.
	if !floatsClose(r.Fee, 0.4) {
		t.Errorf("Fee = %v, want 0.4", r.Fee)
	}
	// EntryTime is the first matched open trade's time.
	if !r.EntryTime.Equal(testTime(0)) {
		t.Errorf("EntryTime = %v, want %v", r.EntryTime, testTime(0))
	}
}

func TestRebuildPositionsFromTrades_HedgeMode(t *testing.T) {
	trades := []TradeRecord{
		{
			TradeID:      "lo",
			Symbol:       "BTCUSDT",
			Side:         "BUY",
			PositionSide: "LONG",
			Price:        100.0,
			Quantity:     1.0,
			Time:         testTime(0),
		},
		{
			TradeID:      "so",
			Symbol:       "BTCUSDT",
			Side:         "SELL",
			PositionSide: "SHORT",
			Price:        100.0,
			Quantity:     1.0,
			Time:         testTime(1),
		},
		{
			TradeID:      "lc",
			Symbol:       "BTCUSDT",
			Side:         "SELL",
			PositionSide: "LONG",
			Price:        110.0,
			Quantity:     1.0,
			RealizedPnL:  10.0,
			Time:         testTime(2),
		},
		{
			TradeID:      "sc",
			Symbol:       "BTCUSDT",
			Side:         "BUY",
			PositionSide: "SHORT",
			Price:        90.0,
			Quantity:     1.0,
			RealizedPnL:  10.0,
			Time:         testTime(3),
		},
	}

	records := RebuildPositionsFromTrades(trades)
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	long := records[0]
	if long.Side != "long" {
		t.Fatalf("records[0].Side = %q, want long", long.Side)
	}
	if !floatsClose(long.EntryPrice, 100.0) || !floatsClose(long.ExitPrice, 110.0) {
		t.Errorf("long entry/exit = %v/%v, want 100/110", long.EntryPrice, long.ExitPrice)
	}

	short := records[1]
	if short.Side != "short" {
		t.Fatalf("records[1].Side = %q, want short", short.Side)
	}
	if !floatsClose(short.EntryPrice, 100.0) || !floatsClose(short.ExitPrice, 90.0) {
		t.Errorf("short entry/exit = %v/%v, want 100/90", short.EntryPrice, short.ExitPrice)
	}
}

func TestRebuildPositionsFromTrades_OneWayModeShort(t *testing.T) {
	trades := []TradeRecord{
		{
			TradeID:  "open1",
			Symbol:   "SOLUSDT",
			Side:     "SELL", // sell with zero PnL opens a short in one-way mode
			Price:    100.0,
			Quantity: 1.0,
			Fee:      0.05,
			Time:     testTime(0),
		},
		{
			TradeID:     "close1",
			Symbol:      "SOLUSDT",
			Side:        "BUY", // buy with non-zero PnL closes the short
			Price:       90.0,
			Quantity:    1.0,
			RealizedPnL: 10.0,
			Fee:         0.05,
			Time:        testTime(1),
		},
	}

	records := RebuildPositionsFromTrades(trades)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	if r.Side != "short" {
		t.Errorf("Side = %q, want short", r.Side)
	}
	if !floatsClose(r.EntryPrice, 100.0) {
		t.Errorf("EntryPrice = %v, want 100", r.EntryPrice)
	}
	if !floatsClose(r.ExitPrice, 90.0) {
		t.Errorf("ExitPrice = %v, want 90", r.ExitPrice)
	}
	if !floatsClose(r.Fee, 0.1) {
		t.Errorf("Fee = %v, want 0.1", r.Fee)
	}
}

func TestRebuildPositionsFromTrades_PnLFallbackEntryPrice(t *testing.T) {
	tests := []struct {
		name      string
		trade     TradeRecord
		wantSide  string
		wantEntry float64
	}{
		{
			name: "long fallback: entry = exit - pnl/qty",
			trade: TradeRecord{
				TradeID:     "lone-long",
				Symbol:      "BTCUSDT",
				Side:        "SELL",
				Price:       110.0,
				Quantity:    2.0,
				RealizedPnL: 20.0,
				Time:        testTime(0),
			},
			wantSide:  "long",
			wantEntry: 100.0, // 110 - 20/2
		},
		{
			name: "short fallback: entry = exit + pnl/qty",
			trade: TradeRecord{
				TradeID:     "lone-short",
				Symbol:      "BTCUSDT",
				Side:        "BUY",
				Price:       95.0,
				Quantity:    1.0,
				RealizedPnL: 5.0,
				Time:        testTime(0),
			},
			wantSide:  "short",
			wantEntry: 100.0, // 95 + 5/1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := RebuildPositionsFromTrades([]TradeRecord{tt.trade})
			if len(records) != 1 {
				t.Fatalf("got %d records, want 1", len(records))
			}
			r := records[0]
			if r.Side != tt.wantSide {
				t.Errorf("Side = %q, want %q", r.Side, tt.wantSide)
			}
			if !floatsClose(r.EntryPrice, tt.wantEntry) {
				t.Errorf("EntryPrice = %v, want %v", r.EntryPrice, tt.wantEntry)
			}
			// Without a matching open trade, entry time falls back to exit time.
			if !r.EntryTime.Equal(r.ExitTime) {
				t.Errorf("EntryTime = %v, want exit time %v", r.EntryTime, r.ExitTime)
			}
		})
	}
}

func TestRebuildPositionsFromTrades_InvalidTrades(t *testing.T) {
	tests := []struct {
		name   string
		trades []TradeRecord
	}{
		{
			name: "closing trade with zero quantity",
			trades: []TradeRecord{
				{
					TradeID:     "zq",
					Symbol:      "BTCUSDT",
					Side:        "SELL",
					Price:       110.0,
					Quantity:    0,
					RealizedPnL: 10.0,
					Time:        testTime(0),
				},
			},
		},
		{
			name: "closing trade with zero price",
			trades: []TradeRecord{
				{
					TradeID:     "zp",
					Symbol:      "BTCUSDT",
					Side:        "SELL",
					Price:       0,
					Quantity:    1.0,
					RealizedPnL: 10.0,
					Time:        testTime(0),
				},
			},
		},
		{
			name: "trade with unrecognized side is skipped",
			trades: []TradeRecord{
				{
					TradeID:  "bad-side",
					Symbol:   "BTCUSDT",
					Side:     "HOLD",
					Price:    100.0,
					Quantity: 1.0,
					Time:     testTime(0),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := RebuildPositionsFromTrades(tt.trades)
			if len(records) != 0 {
				t.Errorf("got %d records, want 0: %+v", len(records), records)
			}
		})
	}
}

func TestRebuildPositionsFromTrades_UnsortedInputUsesChronologicalFIFO(t *testing.T) {
	// Deliberately out of chronological order: close first, opens reversed.
	trades := []TradeRecord{
		{
			TradeID:     "close1",
			Symbol:      "BTCUSDT",
			Side:        "SELL",
			Price:       120.0,
			Quantity:    1.0,
			RealizedPnL: 20.0,
			Time:        testTime(2),
		},
		{
			TradeID:  "open2",
			Symbol:   "BTCUSDT",
			Side:     "BUY",
			Price:    110.0,
			Quantity: 1.0,
			Time:     testTime(1),
		},
		{
			TradeID:  "open1",
			Symbol:   "BTCUSDT",
			Side:     "BUY",
			Price:    100.0,
			Quantity: 1.0,
			Time:     testTime(0),
		},
	}

	records := RebuildPositionsFromTrades(trades)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	// FIFO after time sort: the earliest open (price 100) is matched first.
	if !floatsClose(records[0].EntryPrice, 100.0) {
		t.Errorf("EntryPrice = %v, want 100 (earliest open via FIFO)", records[0].EntryPrice)
	}
	if !records[0].EntryTime.Equal(testTime(0)) {
		t.Errorf("EntryTime = %v, want %v", records[0].EntryTime, testTime(0))
	}
}
