package store

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestPositionStore(t *testing.T) *PositionStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&TraderOrder{}, &TraderPosition{}, &PositionCloseEvent{}); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}
	return NewPositionStore(db)
}

func seedTestPosition(t *testing.T, s *PositionStore, exchangeID string, qty, entryPrice float64) *TraderPosition {
	t.Helper()
	pos := &TraderPosition{
		TraderID:      "trader-1",
		ExchangeID:    exchangeID,
		ExchangeType:  "okx",
		Symbol:        "ADAUSDT",
		Side:          "LONG",
		EntryQuantity: qty,
		Quantity:      qty,
		EntryPrice:    entryPrice,
		Status:        "OPEN",
	}
	if err := s.db.Create(pos).Error; err != nil {
		t.Fatalf("failed to seed position: %v", err)
	}
	return pos
}

func seedTestOrder(t *testing.T, s *PositionStore, exchangeID, exchangeOrderID, clientOrderID, orderType, orderAction string) {
	t.Helper()
	ord := &TraderOrder{
		TraderID:        "trader-1",
		ExchangeID:      exchangeID,
		ExchangeType:    "okx",
		ExchangeOrderID: exchangeOrderID,
		ClientOrderID:   clientOrderID,
		Symbol:          "ADAUSDT",
		Side:            "SELL",
		PositionSide:    "LONG",
		Type:            orderType,
		OrderAction:     orderAction,
		Status:          "FILLED",
		Quantity:        10,
		FilledQuantity:  10,
	}
	if err := s.db.Create(ord).Error; err != nil {
		t.Fatalf("failed to seed order: %v", err)
	}
}

func TestDeriveCloseReasonManagedDrawdownPrefersSpecificProtectionReason(t *testing.T) {
	s := newTestPositionStore(t)
	pos := seedTestPosition(t, s, "ex-1", 100, 1.0)
	seedTestOrder(t, s, "ex-1", "ord-1", "close_long_managed_drawdown", "MARKET", "close_long_managed_drawdown")

	reason, source, executionType := s.deriveCloseReason(pos, "ord-1", "close_long", 70, 1.01)
	if reason != "managed_drawdown" || source != "managed_drawdown" {
		t.Fatalf("expected managed_drawdown, got reason=%q source=%q", reason, source)
	}
	if executionType != "MARKET" {
		t.Fatalf("expected MARKET execution type, got %q", executionType)
	}
}

func TestDeriveCloseReasonBreakEvenFromActionWhenTagMissing(t *testing.T) {
	s := newTestPositionStore(t)
	pos := seedTestPosition(t, s, "ex-2", 100, 100)
	seedTestOrder(t, s, "ex-2", "ord-2", "", "MARKET", "break_even_stop")

	reason, source, _ := s.deriveCloseReason(pos, "ord-2", "close_long", 100, 100.1)
	if reason != "break_even_stop" || source != "break_even_stop" {
		t.Fatalf("expected break_even_stop, got reason=%q source=%q", reason, source)
	}
}

func TestDeriveCloseReasonFallbackMaxLossMapsToFullSL(t *testing.T) {
	s := newTestPositionStore(t)
	pos := seedTestPosition(t, s, "ex-3", 100, 100)
	seedTestOrder(t, s, "ex-3", "ord-3", "fallback_maxloss_sl", "STOP_MARKET", "fallback_maxloss_sl")

	reason, source, executionType := s.deriveCloseReason(pos, "ord-3", "close_long", 100, 95)
	if reason != "full_sl" || source != "full_sl" {
		t.Fatalf("expected full_sl from fallback max loss, got reason=%q source=%q", reason, source)
	}
	if executionType != "STOP_MARKET" {
		t.Fatalf("expected STOP_MARKET execution type, got %q", executionType)
	}
}
