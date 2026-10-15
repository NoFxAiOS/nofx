package store

import (
	"testing"
	"time"
)

func TestStockThesisRoundTrip(t *testing.T) {
	st := newTestStore(t)
	th := st.StockThesis()
	now := time.Now()
	if err := th.Upsert("tr", "AAPLBUSDT", "earnings beat drives re-rating", "close below 95", now); err != nil {
		t.Fatal(err)
	}
	if err := th.Upsert("tr", "NVDABUSDT", "AI capex", "loses EMA50", now); err != nil {
		t.Fatal(err)
	}
	if err := th.Upsert("other", "AAPLBUSDT", "x", "y", now); err != nil {
		t.Fatal(err)
	}
	// Upsert replaces.
	if err := th.Upsert("tr", "AAPLBUSDT", "refined thesis", "close below 97", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	m, err := th.List("tr")
	if err != nil || len(m) != 2 {
		t.Fatalf("list %+v %v", m, err)
	}
	if a := m["AAPLBUSDT"]; a.Thesis != "refined thesis" || a.Invalidation != "close below 97" || a.UpdatedAt != now.Add(time.Hour).UTC().UnixMilli() {
		t.Fatalf("AAPL row %+v", a)
	}
	if err := th.Delete("tr", "AAPLBUSDT"); err != nil {
		t.Fatal(err)
	}
	if err := th.Delete("tr", "MISSINGBUSDT"); err != nil {
		t.Fatalf("deleting an absent row must not fail: %v", err)
	}
	m, _ = th.List("tr")
	if len(m) != 1 || m["NVDABUSDT"].Thesis != "AI capex" {
		t.Fatalf("after delete %+v", m)
	}
	if o, _ := th.List("other"); len(o) != 1 {
		t.Fatalf("other trader's rows must be untouched: %+v", o)
	}
}
