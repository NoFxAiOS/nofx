package store

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPositionReconcileTestStore(t *testing.T) *PositionStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&TraderPosition{}); err != nil {
		t.Fatalf("migrate positions: %v", err)
	}
	return NewPositionStore(db)
}

func TestMarkOpenPositionsAbsentFromExchangeClosed(t *testing.T) {
	s := newPositionReconcileTestStore(t)
	positions := []*TraderPosition{
		{TraderID: "t", ExchangeID: "ex", Symbol: "BTCUSDT", Side: "LONG", Quantity: 0.0009, EntryQuantity: 0.0009, EntryPrice: 77165.5, EntryTime: 1, Status: "OPEN"},
		{TraderID: "t", ExchangeID: "ex", Symbol: "TAOUSDT", Side: "SHORT", Quantity: 0.02, EntryQuantity: 0.02, EntryPrice: 242.207519, EntryTime: 2, Status: "OPEN"},
		{TraderID: "other", ExchangeID: "ex", Symbol: "SOLUSDT", Side: "SHORT", Quantity: 1, EntryQuantity: 1, EntryPrice: 82.55, EntryTime: 3, Status: "OPEN"},
	}
	for _, pos := range positions {
		if err := s.CreateOpenPosition(pos); err != nil {
			t.Fatalf("create position: %v", err)
		}
	}

	updated, err := s.MarkOpenPositionsAbsentFromExchangeClosed("t", map[string]float64{"BTCUSDT|LONG": 0.0009}, "sync_absent_from_exchange")
	if err != nil {
		t.Fatalf("mark absent closed: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 stale position update, got %d", updated)
	}

	open, err := s.GetOpenPositions("t")
	if err != nil {
		t.Fatalf("get open positions: %v", err)
	}
	if len(open) != 1 || open[0].Symbol != "BTCUSDT" || open[0].Side != "LONG" {
		t.Fatalf("expected only BTC long open, got %+v", open)
	}

	closed, err := s.GetOpenPositionBySymbol("t", "TAOUSDT", "SHORT")
	if err != nil {
		t.Fatalf("query TAO open: %v", err)
	}
	if closed != nil {
		t.Fatalf("expected TAO to be closed, still open: %+v", closed)
	}

	otherOpen, err := s.GetOpenPositions("other")
	if err != nil {
		t.Fatalf("get other open positions: %v", err)
	}
	if len(otherOpen) != 1 || otherOpen[0].Symbol != "SOLUSDT" {
		t.Fatalf("expected other trader position preserved, got %+v", otherOpen)
	}
}

func TestMarkOpenPositionsAbsentFromExchangeClosedWithEmptyLiveClosesAllTraderPositions(t *testing.T) {
	s := newPositionReconcileTestStore(t)
	if err := s.CreateOpenPosition(&TraderPosition{TraderID: "t", ExchangeID: "ex", Symbol: "BTCUSDT", Side: "LONG", Quantity: 0.0009, EntryQuantity: 0.0009, EntryPrice: 77165.5, EntryTime: 1, Status: "OPEN"}); err != nil {
		t.Fatalf("create position: %v", err)
	}

	updated, err := s.MarkOpenPositionsAbsentFromExchangeClosed("t", nil, "sync_absent_from_exchange")
	if err != nil {
		t.Fatalf("mark absent closed: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 stale position update, got %d", updated)
	}
	open, err := s.GetOpenPositions("t")
	if err != nil {
		t.Fatalf("get open positions: %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("expected no open positions, got %+v", open)
	}
}
