package store

import (
	"math"
	"testing"
)

// insertTestFill writes a fill with the NOT NULL columns the sync always sets.
func insertTestFill(t *testing.T, st *Store, f TraderFill) {
	t.Helper()
	f.OrderID, f.ExchangeOrderID = 1, "order-"+f.ExchangeTradeID
	f.Price, f.Quantity, f.QuoteQuantity, f.CommissionAsset = 1, 1, 1, "USDT"
	if err := st.Order().db.Create(&f).Error; err != nil {
		t.Fatalf("insert fill %s: %v", f.ExchangeTradeID, err)
	}
}

// The row a fill belongs to is the one open at that moment: opened at or before
// the fill, and still OPEN or closed at or after it. Other sides, symbols,
// traders and earlier or later cycles must never match.
func TestGetPositionForFillFindsRowOpenAtFill(t *testing.T) {
	st := newTestStore(t)
	const fill = int64(2_000_000)
	rows := []TraderPosition{
		{TraderID: "T1", Symbol: "XRPUSDT", Side: "SHORT", Status: "CLOSED", EntryTime: 1_000_000, ExitTime: 1_500_000, EntryQuantity: 10},
		{TraderID: "T1", Symbol: "XRPUSDT", Side: "SHORT", Status: "CLOSED", EntryTime: 1_600_000, ExitTime: fill, EntryQuantity: 71.8},
		{TraderID: "T1", Symbol: "XRPUSDT", Side: "LONG", Status: "OPEN", EntryTime: 1_700_000, Quantity: 5, EntryQuantity: 5},
		{TraderID: "T1", Symbol: "XRPUSDT", Side: "SHORT", Status: "OPEN", EntryTime: 2_100_000, Quantity: 3, EntryQuantity: 3},
		{TraderID: "T2", Symbol: "XRPUSDT", Side: "SHORT", Status: "OPEN", EntryTime: 1_800_000, Quantity: 9, EntryQuantity: 9},
	}
	for i := range rows {
		if err := st.Position().db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("insert row %d: %v", i, err)
		}
	}
	// rows[0] closed before the fill, rows[2] is the other side, rows[3] opened
	// after it, rows[4] belongs to another trader: only rows[1] qualifies.
	got, err := st.Position().GetPositionForFill("T1", "XRPUSDT", "SHORT", fill)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != rows[1].ID {
		t.Fatalf("got %+v, want row %d (the one the fill closed)", got, rows[1].ID)
	}
}

// A fill with no row open at that moment (its position was closed earlier, e.g.
// an orphan close the sync skipped) must not borrow an older closed row's PnL.
func TestGetPositionForFillIgnoresRowClosedBeforeFill(t *testing.T) {
	st := newTestStore(t)
	row := TraderPosition{TraderID: "T1", Symbol: "XRPUSDT", Side: "SHORT", Status: "CLOSED", EntryTime: 1_000_000, ExitTime: 1_500_000, RealizedPnL: 0.08, Fee: 0.07, EntryQuantity: 71.8}
	if err := st.Position().db.Create(&row).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got, err := st.Position().GetPositionForFill("T1", "XRPUSDT", "SHORT", 2_000_000); err != nil || got != nil {
		t.Fatalf("row closed before the fill matched: got %+v, err %v", got, err)
	}
}

func TestGetPositionForFillBaseSymbolAndMisses(t *testing.T) {
	st := newTestStore(t)
	// Legacy row stored under the base symbol: 100 entered, 40 left open.
	row := TraderPosition{TraderID: "T1", Symbol: "SOL", Side: "LONG", Status: "OPEN", EntryTime: 1_000, Quantity: 40, EntryQuantity: 100, Fee: 0.0376}
	if err := st.Position().db.Create(&row).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := st.Position().GetPositionForFill("T1", "SOLUSDT", "LONG", 5_000)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != row.ID || got.EntryQuantity != 100 || got.Fee != 0.0376 {
		t.Fatalf("base-symbol row: got %+v", got)
	}
	if got, err := st.Position().GetPositionForFill("T1", "SOLUSDT", "LONG", 500); err != nil || got != nil {
		t.Fatalf("fill before the row was opened: got %+v, err %v", got, err)
	}
	if got, err := st.Position().GetPositionForFill("T1", "SOLUSDT", "SHORT", 5_000); err != nil || got != nil {
		t.Fatalf("wrong side: got %+v, err %v", got, err)
	}
	if got, err := st.Position().GetPositionForFill("T9", "SOLUSDT", "LONG", 5_000); err != nil || got != nil {
		t.Fatalf("other trader: got %+v, err %v", got, err)
	}
}

// Close commission of one position: only closing legs on the closing side,
// realizing PnL, booked at or after the entry, of this trader and symbol.
func TestSumCloseCommissionCountsOnlyThisPositionsCloseLegs(t *testing.T) {
	st := newTestStore(t)
	fills := []TraderFill{
		{TraderID: "T1", ExchangeTradeID: "f1", Symbol: "SOLUSDT", Side: "BUY", Commission: 0.02, CreatedAt: 1_000},
		{TraderID: "T1", ExchangeTradeID: "f2", Symbol: "SOLUSDT", Side: "SELL", RealizedPnL: 1.5, Commission: 0.009, CreatedAt: 2_000},
		{TraderID: "T1", ExchangeTradeID: "f3", Symbol: "SOLUSDT", Side: "SELL", RealizedPnL: 1.2, Commission: 0.0086, CreatedAt: 3_000},
		{TraderID: "T1", ExchangeTradeID: "f4", Symbol: "SOLUSDT", Side: "BUY", RealizedPnL: 0.4, Commission: 0.5, CreatedAt: 4_000},
		{TraderID: "T1", ExchangeTradeID: "f5", Symbol: "SOLUSDT", Side: "SELL", RealizedPnL: 0.7, Commission: 0.3, CreatedAt: 500},
		{TraderID: "T1", ExchangeTradeID: "f6", Symbol: "XRPUSDT", Side: "SELL", RealizedPnL: 2, Commission: 0.2, CreatedAt: 2_500},
		{TraderID: "T2", ExchangeTradeID: "f7", Symbol: "SOLUSDT", Side: "SELL", RealizedPnL: 2, Commission: 0.2, CreatedAt: 2_500},
		{TraderID: "T1", ExchangeTradeID: "f8", Symbol: "SOLUSDT", Side: "SELL", Commission: 0.7, CreatedAt: 1_500}, // SHORT entry: SELL, realizes nothing
	}
	for _, f := range fills {
		insertTestFill(t, st, f)
	}
	// f1 is the entry (no PnL, BUY), f4 closes a SHORT (BUY), f5 predates the
	// entry, f6/f7 are other symbol/trader, f8 is a SHORT entry on the closing
	// side: f2 + f3 remain.
	got, err := st.Order().SumCloseCommission("T1", "SOLUSDT", "SELL", 1_000)
	if err != nil {
		t.Fatal(err)
	}
	if want := 0.009 + 0.0086; math.Abs(got-want) > 1e-12 {
		t.Fatalf("close commission = %v, want %v", got, want)
	}
	if got, err := st.Order().SumCloseCommission("T1", "SOLUSDT", "SELL", 9_000); err != nil || got != 0 {
		t.Fatalf("no close legs: got %v, err %v (want 0)", got, err)
	}
}
