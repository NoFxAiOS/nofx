package store

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestPositionBuilderBackfillsLateCloseAfterSyncAbsentClosure(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "late-close.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	positionStore := st.Position()
	builder := NewPositionBuilder(positionStore)

	base := time.Now().UTC().UnixMilli()
	pos := &TraderPosition{
		TraderID:           "t",
		ExchangeID:         "ex",
		ExchangeType:       "okx",
		ExchangePositionID: "sync_HYPEUSDT_LONG_1",
		Symbol:             "HYPEUSDT",
		Side:               "LONG",
		Quantity:           1.3,
		EntryQuantity:      1.3,
		EntryPrice:         39.12,
		Status:             "OPEN",
		Source:             "sync",
		EntryTime:          base,
	}
	if err := positionStore.CreateOpenPosition(pos); err != nil {
		t.Fatalf("create position: %v", err)
	}
	if err := positionStore.ReducePositionQuantity(pos.ID, 0.6, 39.28, 0.001, 0.11, "close_long", "close_long", "MARKET", "close-early", base+1000); err != nil {
		t.Fatalf("partial close: %v", err)
	}
	updated, err := positionStore.MarkOpenPositionsAbsentFromExchangeClosed("t", nil, "sync_absent_from_exchange")
	if err != nil {
		t.Fatalf("mark absent closed: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 position to be sync-closed, got %d", updated)
	}

	if err := builder.ProcessTrade("t", "ex", "okx", "HYPEUSDT", "LONG", "close_long", 0.7, 39.28, 0.002, 0.21, base+3000, "late-close"); err != nil {
		t.Fatalf("process late close: %v", err)
	}

	closed, err := positionStore.GetRecentlyClosedSyncAbsentPosition("t", "HYPEUSDT", "LONG", base+3000, 5*time.Second)
	if err != nil {
		t.Fatalf("get recently closed position: %v", err)
	}
	if closed == nil {
		t.Fatal("expected closed position")
	}
	if math.Abs(closed.RealizedPnL-0.32) > 1e-9 {
		t.Fatalf("expected realized pnl 0.32, got %.8f", closed.RealizedPnL)
	}
	if math.Abs(closed.Fee-0.003) > 1e-9 {
		t.Fatalf("expected fee 0.003, got %.8f", closed.Fee)
	}
	if math.Abs(closed.ExitPrice-39.28) > 1e-9 {
		t.Fatalf("expected exit price 39.28, got %.8f", closed.ExitPrice)
	}

	events, err := st.PositionClose().ListByPositionID(closed.ID)
	if err != nil {
		t.Fatalf("list close events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 close events, got %d", len(events))
	}
	if math.Abs(events[1].CloseQuantity-0.7) > 1e-9 {
		t.Fatalf("expected late close qty 0.7, got %.8f", events[1].CloseQuantity)
	}
	if events[1].ExchangeOrderID != "late-close" {
		t.Fatalf("expected late close order id recorded, got %q", events[1].ExchangeOrderID)
	}
}
