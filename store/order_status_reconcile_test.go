package store

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestOrderStore(t *testing.T) *OrderStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&TraderOrder{}); err != nil {
		t.Fatalf("migrate orders: %v", err)
	}
	return NewOrderStore(db)
}

func TestMarkMissingOpenOrdersCanceledPreservesLiveAlgoSuffixes(t *testing.T) {
	s := newTestOrderStore(t)
	orders := []*TraderOrder{
		{TraderID: "t", ExchangeID: "ex", ExchangeOrderID: "a1", Symbol: "DOGEUSDT", Side: "SELL", Type: "ALGO", Quantity: 1, Status: "NEW"},
		{TraderID: "t", ExchangeID: "ex", ExchangeOrderID: "a2", Symbol: "DOGEUSDT", Side: "SELL", Type: "ALGO", Quantity: 1, Status: "NEW"},
		{TraderID: "t", ExchangeID: "ex", ExchangeOrderID: "trail1", Symbol: "DOGEUSDT", Side: "SELL", Type: "TRAILING_STOP_MARKET", Quantity: 1, Status: "NEW"},
		{TraderID: "t", ExchangeID: "ex", ExchangeOrderID: "btc1", Symbol: "BTCUSDT", Side: "SELL", Type: "ALGO", Quantity: 1, Status: "NEW"},
	}
	for _, order := range orders {
		if err := s.CreateOrder(order); err != nil {
			t.Fatalf("create order: %v", err)
		}
	}

	updated, err := s.MarkMissingOpenOrdersCanceled("ex", "DOGEUSDT", []string{"a1_sl", "trail1"})
	if err != nil {
		t.Fatalf("mark missing canceled: %v", err)
	}
	if updated != 1 {
		t.Fatalf("expected 1 stale order update, got %d", updated)
	}

	assertStatus := func(exchangeOrderID, want string) {
		t.Helper()
		got, err := s.GetOrderByExchangeID("ex", exchangeOrderID)
		if err != nil {
			t.Fatalf("get order %s: %v", exchangeOrderID, err)
		}
		if got.Status != want {
			t.Fatalf("order %s status = %s, want %s", exchangeOrderID, got.Status, want)
		}
	}
	assertStatus("a1", "NEW")
	assertStatus("a2", "CANCELED")
	assertStatus("trail1", "NEW")
	assertStatus("btc1", "NEW")
}

func TestMarkSymbolProtectionOrdersCanceledOnlyProtectionOrders(t *testing.T) {
	s := newTestOrderStore(t)
	orders := []*TraderOrder{
		{TraderID: "t", ExchangeID: "ex", ExchangeOrderID: "algo", Symbol: "DOGEUSDT", Side: "SELL", Type: "ALGO", Quantity: 1, Status: "NEW"},
		{TraderID: "t", ExchangeID: "ex", ExchangeOrderID: "trail", Symbol: "DOGEUSDT", Side: "SELL", Type: "TRAILING_STOP_MARKET", Quantity: 1, Status: "NEW"},
		{TraderID: "t", ExchangeID: "ex", ExchangeOrderID: "market", Symbol: "DOGEUSDT", Side: "SELL", Type: "MARKET", Quantity: 1, Status: "NEW"},
	}
	for _, order := range orders {
		if err := s.CreateOrder(order); err != nil {
			t.Fatalf("create order: %v", err)
		}
	}

	updated, err := s.MarkSymbolProtectionOrdersCanceled("ex", "DOGEUSDT")
	if err != nil {
		t.Fatalf("mark symbol canceled: %v", err)
	}
	if updated != 2 {
		t.Fatalf("expected 2 protection order updates, got %d", updated)
	}

	algo, _ := s.GetOrderByExchangeID("ex", "algo")
	trail, _ := s.GetOrderByExchangeID("ex", "trail")
	market, _ := s.GetOrderByExchangeID("ex", "market")
	if algo.Status != "CANCELED" || trail.Status != "CANCELED" || market.Status != "NEW" {
		t.Fatalf("unexpected statuses: algo=%s trail=%s market=%s", algo.Status, trail.Status, market.Status)
	}
}
