package store

import (
	"math"
	"testing"
	"time"
)

func TestStockPaperLedgerRoundTrip(t *testing.T) {
	st := newTestStore(t)
	sp := st.StockPaper()
	now := time.Now()
	acct, err := sp.EnsureAccount("tr", 0)
	if err != nil || acct.Cash != DefaultStockPaperCash {
		t.Fatalf("default account %+v %v", acct, err)
	}
	if again, _ := sp.EnsureAccount("tr", 777); again.Cash != DefaultStockPaperCash {
		t.Fatal("EnsureAccount must not reset an existing ledger")
	}
	if _, err := sp.OpenOrAdd("tr", "AAPLBUSDT", 10, 100, 95, 120, now); err != nil {
		t.Fatal(err)
	}
	pos, err := sp.OpenOrAdd("tr", "AAPLBUSDT", 10, 110, 100, 0, now)
	if err != nil || math.Abs(pos.AvgPrice-105) > 1e-9 || pos.Quantity != 20 || pos.InitialStop != 95 || pos.Stop != 100 || pos.TakeProfit != 0 {
		t.Fatalf("merged position %+v %v", pos, err)
	}
	pnl, closed, err := sp.Reduce("tr", "AAPLBUSDT", 5, 115, now, "ai_reduce")
	if err != nil || closed || math.Abs(pnl-50) > 1e-9 {
		t.Fatalf("partial reduce pnl=%v closed=%v err=%v", pnl, closed, err)
	}
	pnl, closed, err = sp.Reduce("tr", "AAPLBUSDT", 0, 125, now, "ai_close")
	if err != nil || !closed || math.Abs(pnl-15*20) > 1e-9 {
		t.Fatalf("full close pnl=%v closed=%v err=%v", pnl, closed, err)
	}
	rows, _ := sp.ListClosed("tr", 10)
	if len(rows) != 1 || rows[0].Status != StockPaperClosed || math.Abs(rows[0].RealizedPnL-350) > 1e-9 || math.Abs(rows[0].ExitPrice-(5*115+15*125)/20.0) > 1e-9 {
		t.Fatalf("closed row %+v", rows)
	}
	if open, _ := sp.ListOpen("tr"); len(open) != 0 {
		t.Fatal("no open rows expected")
	}
	acct, _ = sp.GetAccount("tr")
	if math.Abs(acct.Cash-(10000+350)) > 1e-9 {
		t.Fatalf("cash %v", acct.Cash)
	}
}

func TestApplyExternalReductionKeepsCostBase(t *testing.T) {
	st := newTestStore(t)
	row := &TraderPosition{TraderID: "tr", Symbol: "AAPLBUSDT", Side: "LONG", Quantity: 10, EntryQuantity: 10, EntryPrice: 100, EntryTime: 1, AIManaged: true, Status: "OPEN"}
	if err := st.Position().CreateOpenPosition(row); err != nil {
		t.Fatal(err)
	}
	if err := st.Position().ApplyExternalReduction(row.ID, 6, 101, time.Now()); err != nil {
		t.Fatal(err)
	}
	rows, _ := st.Position().StockOwnedRows("tr", "AAPLBUSDT")
	if len(rows) != 1 || rows[0].Quantity != 6 || rows[0].EntryQuantity != 6 || rows[0].RealizedPnL != 0 {
		t.Fatalf("after shrink %+v", rows)
	}
	if err := st.Position().ApplyExternalReduction(row.ID, 0, 101, time.Now()); err != nil {
		t.Fatal(err)
	}
	if rows, _ = st.Position().StockOwnedRows("tr", "AAPLBUSDT"); len(rows) != 0 {
		t.Fatal("row should be closed")
	}
	closed, _ := st.Position().GetClosedPositions("tr", 5)
	if len(closed) != 1 || closed[0].CloseReason != "external" || closed[0].RealizedPnL != 0 {
		t.Fatalf("closed %+v", closed)
	}
}

func TestStockBookEntryFillIsAtomicAndIdempotent(t *testing.T) {
	st := newTestStore(t)
	sp := st.StockPaper()
	fill := func(qty, price float64) *TraderPosition {
		return &TraderPosition{TraderID: "tr", Symbol: "AAPLBUSDT", Side: "LONG", Quantity: qty, EntryQuantity: qty, EntryPrice: price,
			EntryTime: 1, Leverage: 1, Status: "OPEN", Source: "system", AIManaged: true, InitialStopLoss: 90}
	}
	pend := func(filled float64) *StockPendingEntry {
		return &StockPendingEntry{TraderID: "tr", Symbol: "AAPLBUSDT", OrderID: "9", Quantity: 4, FilledQty: filled, FilledQuote: filled * 100}
	}
	booked, err := sp.BookEntryFill(fill(2, 100), pend(2), true)
	if err != nil || !booked {
		t.Fatalf("first fill booked=%v err=%v", booked, err)
	}
	// Replaying the same watermark books nothing (restart / double delivery).
	booked, err = sp.BookEntryFill(fill(2, 100), pend(2), true)
	if err != nil || booked {
		t.Fatalf("replay booked=%v err=%v", booked, err)
	}
	rows, _ := st.Position().StockOwnedRows("tr", "AAPLBUSDT")
	if len(rows) != 1 || rows[0].Quantity != 2 {
		t.Fatalf("replay changed the position: %+v", rows)
	}
	// A failing watermark write rolls the position increment back.
	if err := st.GormDB().Exec(`CREATE TRIGGER fail_wm BEFORE UPDATE ON stock_pending_entries BEGIN SELECT RAISE(ABORT, 'injected'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := sp.BookEntryFill(fill(2, 80), pend(4), true); err == nil {
		t.Fatal("expected injected failure")
	}
	rows, _ = st.Position().StockOwnedRows("tr", "AAPLBUSDT")
	if len(rows) != 1 || rows[0].Quantity != 2 {
		t.Fatalf("position increment escaped the failed transaction: %+v", rows)
	}
	pending, _ := sp.ListPending("tr")
	if len(pending) != 1 || pending[0].FilledQty != 2 || pending[0].FilledQuote != 200 {
		t.Fatalf("watermark moved despite the rollback: %+v", pending)
	}
	st.GormDB().Exec(`DROP TRIGGER fail_wm`)
	if booked, err := sp.BookEntryFill(fill(2, 80), pend(4), false); err != nil || !booked {
		t.Fatalf("retry booked=%v err=%v", booked, err)
	}
	rows, _ = st.Position().StockOwnedRows("tr", "AAPLBUSDT")
	if len(rows) != 1 || rows[0].Quantity != 4 {
		t.Fatalf("after retry: %+v", rows)
	}
	if in, _ := sp.ListProtectionIntents("tr"); !in["AAPLBUSDT"] {
		t.Fatalf("intent not recorded with the fill: %v", in)
	}
}
