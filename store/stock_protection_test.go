package store

import (
	"math"
	"testing"
	"time"
)

func TestStockProtectionCumulativeFillsAreAtomicAndIdempotent(t *testing.T) {
	st := newTestStore(t)
	pos := &TraderPosition{TraderID: "tr", Symbol: "AAPLBUSDT", Side: "LONG", Quantity: 5, EntryQuantity: 5, EntryPrice: 120, EntryTime: 1, AIManaged: true, Status: "OPEN"}
	if err := st.Position().CreateOpenPosition(pos); err != nil {
		t.Fatal(err)
	}
	leg := StockProtectionOrder{TraderID: "tr", Symbol: "AAPLBUSDT", OrderID: "10", OrderListID: "9", ClientOrderID: "nxbs_sl_10", Role: "stop_loss", Quantity: 5, StopPrice: 115, StopLimitPrice: 114.65, TakeProfit: 130}
	sp := st.StockPaper()
	if err := sp.SaveProtectionOrders([]StockProtectionOrder{leg}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	q, px, pnl, closed, err := sp.BookProtectionStatus(leg, 1, 114, 0.1, "PARTIALLY_FILLED", now)
	if err != nil || q != 1 || px != 114 || pnl != -6 || closed {
		t.Fatalf("first fill qty=%g price=%g pnl=%g closed=%v err=%v", q, px, pnl, closed, err)
	}
	// Reconstruct the store, then rediscover the same open order. Neither
	// restart nor upsert may reset the durable execution watermark.
	sp = NewStockPaperStore(st.GormDB())
	if err := sp.SaveProtectionOrders([]StockProtectionOrder{leg}); err != nil {
		t.Fatal(err)
	}
	q, _, _, _, err = sp.BookProtectionStatus(leg, 1, 114, 0.1, "PARTIALLY_FILLED", now)
	if err != nil || q != 0 {
		t.Fatalf("duplicate fill qty=%g err=%v", q, err)
	}
	// A canceled order can include further fills during cancellation. The
	// cumulative average changed: price of the NEW 2 shares is 113, not 113.333.
	q, px, pnl, closed, err = sp.BookProtectionStatus(leg, 3, 340, 0.3, "CANCELED", now)
	if err != nil || q != 2 || px != 113 || pnl != -14 || closed {
		t.Fatalf("delta fill qty=%g price=%g pnl=%g closed=%v err=%v", q, px, pnl, closed, err)
	}
	rows, _ := st.Position().StockOwnedRows("tr", "AAPLBUSDT")
	if len(rows) != 1 || rows[0].Quantity != 2 || rows[0].RealizedPnL != -20 || math.Abs(rows[0].ExitPrice-340.0/3) > 1e-9 || math.Abs(rows[0].Fee-0.3) > 1e-9 {
		t.Fatalf("position after deltas: %+v", rows)
	}
	orders, err := sp.ListProtectionOrders("tr", "AAPLBUSDT")
	if err != nil || len(orders) != 1 || orders[0].BookedQty != 3 || orders[0].BookedQuote != 340 || orders[0].OrderListID != "9" || orders[0].ClientOrderID != "nxbs_sl_10" {
		t.Fatalf("durable order %+v err=%v", orders, err)
	}
	if _, _, _, _, err := sp.BookProtectionStatus(leg, 2, 226, 0.2, "CANCELED", now); err == nil {
		t.Fatal("regressing cumulative executions must be rejected")
	}
	if _, _, _, _, err := sp.BookProtectionStatus(leg, 6, 680, 0.6, "FILLED", now); err == nil {
		t.Fatal("a fill larger than recorded ownership must block reconciliation")
	}
}

func TestStockProtectionWatermarkFailureRollsBackPosition(t *testing.T) {
	st := newTestStore(t)
	pos := &TraderPosition{TraderID: "tr", Symbol: "AAPLBUSDT", Side: "LONG", Quantity: 5, EntryQuantity: 5, EntryPrice: 120, EntryTime: 1, AIManaged: true, Status: "OPEN"}
	if err := st.Position().CreateOpenPosition(pos); err != nil {
		t.Fatal(err)
	}
	leg := StockProtectionOrder{TraderID: "tr", Symbol: "AAPLBUSDT", OrderID: "10", Role: "stop_loss", Quantity: 5}
	if err := st.StockPaper().SaveProtectionOrders([]StockProtectionOrder{leg}); err != nil {
		t.Fatal(err)
	}
	if err := st.GormDB().Exec(`CREATE TRIGGER fail_protection_watermark BEFORE UPDATE ON stock_protection_orders BEGIN SELECT RAISE(ABORT, 'injected watermark failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := st.StockPaper().BookProtectionStatus(leg, 5, 570, 0, "FILLED", time.Now()); err == nil {
		t.Fatal("expected injected failure")
	}
	rows, _ := st.Position().StockOwnedRows("tr", "AAPLBUSDT")
	if len(rows) != 1 || rows[0].Quantity != 5 || rows[0].RealizedPnL != 0 {
		t.Fatalf("position update escaped failed transaction: %+v", rows)
	}
	orders, _ := st.StockPaper().ListProtectionOrders("tr", "AAPLBUSDT")
	if len(orders) != 1 || orders[0].BookedQty != 0 || orders[0].Status != "" {
		t.Fatalf("failed watermark persisted: %+v", orders)
	}
}
