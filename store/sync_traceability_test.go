package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAttachSyncedOrderToPositionForOpenAndClose(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "sync-traceability.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	traderID := "trader-1"
	exchangeID := "exchange-1"
	positionStore := st.Position()
	orderStore := st.Order()
	pos := &TraderPosition{
		TraderID:           traderID,
		ExchangeID:         exchangeID,
		ExchangeType:       "test",
		ExchangePositionID: "sync_BTCUSDT_LONG_1",
		Symbol:             "BTCUSDT",
		Side:               "LONG",
		Quantity:           1,
		EntryQuantity:      1,
		EntryPrice:         100,
		Status:             "OPEN",
		Source:             "sync",
		EntryTime:          time.Now().UTC().UnixMilli(),
	}
	if err := positionStore.CreateOpenPosition(pos); err != nil {
		t.Fatalf("create open position: %v", err)
	}
	openOrder := &TraderOrder{TraderID: traderID, ExchangeID: exchangeID, ExchangeType: "test", ExchangeOrderID: "open-1", Symbol: "BTCUSDT", Side: "BUY", PositionSide: "LONG", Type: "MARKET", OrderAction: "open_long", Quantity: 1, Status: "FILLED"}
	if err := orderStore.CreateOrder(openOrder); err != nil {
		t.Fatalf("create open order: %v", err)
	}
	openFill := &TraderFill{TraderID: traderID, ExchangeID: exchangeID, ExchangeType: "test", OrderID: openOrder.ID, ExchangeOrderID: "open-1", ExchangeTradeID: "open-fill-1", Symbol: "BTCUSDT", Side: "BUY", Price: 100, Quantity: 1, QuoteQuantity: 100, CommissionAsset: "USDT"}
	if err := orderStore.CreateFill(openFill); err != nil {
		t.Fatalf("create open fill: %v", err)
	}
	AttachSyncedOrderToPosition(st, orderStore, positionStore, openOrder, traderID, "BTCUSDT", "LONG", "open_long", "open-1")
	if openOrder.RelatedPositionID != pos.ID {
		t.Fatalf("expected open order attached to position %d, got %d", pos.ID, openOrder.RelatedPositionID)
	}
	var gotOpenFill TraderFill
	if err := st.GormDB().Where("order_id = ?", openOrder.ID).First(&gotOpenFill).Error; err != nil {
		t.Fatalf("query open fill: %v", err)
	}
	if gotOpenFill.RelatedPositionID != pos.ID {
		t.Fatalf("expected open fill attached to position %d, got %d", pos.ID, gotOpenFill.RelatedPositionID)
	}

	closeOrderID := "close-1"
	closeOrder := &TraderOrder{TraderID: traderID, ExchangeID: exchangeID, ExchangeType: "test", ExchangeOrderID: closeOrderID, Symbol: "BTCUSDT", Side: "SELL", PositionSide: "LONG", Type: "MARKET", OrderAction: "close_long", Quantity: 1, Status: "FILLED"}
	if err := orderStore.CreateOrder(closeOrder); err != nil {
		t.Fatalf("create close order: %v", err)
	}
	closeFill := &TraderFill{TraderID: traderID, ExchangeID: exchangeID, ExchangeType: "test", OrderID: closeOrder.ID, ExchangeOrderID: closeOrderID, ExchangeTradeID: "close-fill-1", Symbol: "BTCUSDT", Side: "SELL", Price: 105, Quantity: 1, QuoteQuantity: 105, CommissionAsset: "USDT"}
	if err := orderStore.CreateFill(closeFill); err != nil {
		t.Fatalf("create close fill: %v", err)
	}
	if err := positionStore.ClosePositionFully(pos.ID, 105, closeOrderID, time.Now().UTC().UnixMilli(), 5, 0.1, "close_long", "close_long", "MARKET"); err != nil {
		t.Fatalf("close position: %v", err)
	}
	AttachSyncedOrderToPosition(st, orderStore, positionStore, closeOrder, traderID, "BTCUSDT", "LONG", "close_long", closeOrderID)
	if closeOrder.RelatedPositionID != pos.ID {
		t.Fatalf("expected close order attached to position %d, got %d", pos.ID, closeOrder.RelatedPositionID)
	}
	var gotCloseFill TraderFill
	if err := st.GormDB().Where("order_id = ?", closeOrder.ID).First(&gotCloseFill).Error; err != nil {
		t.Fatalf("query close fill: %v", err)
	}
	if gotCloseFill.RelatedPositionID != pos.ID {
		t.Fatalf("expected close fill attached to position %d, got %d", pos.ID, gotCloseFill.RelatedPositionID)
	}
}
