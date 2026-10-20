package store

import "testing"

func TestStockRegimeScaleRoundTrip(t *testing.T) {
	st := newTestStore(t)
	rs := st.StockRegimeScales()
	if m, err := rs.List("tr"); err != nil || len(m) != 0 {
		t.Fatalf("empty list %+v %v", m, err)
	}
	for _, c := range []struct {
		trader, sym string
		scale       float64
	}{{"tr", "AAPLBUSDT", 1}, {"tr", "NVDABUSDT", 0.8}, {"other", "AAPLBUSDT", 0.3}} {
		if err := rs.Set(c.trader, c.sym, c.scale); err != nil {
			t.Fatal(err)
		}
	}
	// Set replaces.
	if err := rs.Set("tr", "AAPLBUSDT", 0.5); err != nil {
		t.Fatal(err)
	}
	m, err := rs.List("tr")
	if err != nil || len(m) != 2 || m["AAPLBUSDT"] != 0.5 || m["NVDABUSDT"] != 0.8 {
		t.Fatalf("list %+v %v", m, err)
	}
	if err := rs.Delete("tr", "AAPLBUSDT"); err != nil {
		t.Fatal(err)
	}
	if err := rs.Delete("tr", "MISSINGBUSDT"); err != nil {
		t.Fatalf("deleting an absent row must not fail: %v", err)
	}
	m, _ = rs.List("tr")
	if len(m) != 1 || m["NVDABUSDT"] != 0.8 {
		t.Fatalf("after delete %+v", m)
	}
	if o, _ := rs.List("other"); len(o) != 1 || o["AAPLBUSDT"] != 0.3 {
		t.Fatalf("other trader's rows must be untouched: %+v", o)
	}
}

func TestStockRegimeTrimPlanRoundTrip(t *testing.T) {
	st := newTestStore(t)
	rs := st.StockRegimeScales()
	if err := rs.SetPlan("tr", "AAPLBUSDT", 0.5, 5); err != nil {
		t.Fatal(err)
	}
	// A plan on a fresh row leaves the applied scale at the implicit 1.
	if m, _ := rs.List("tr"); m["AAPLBUSDT"] != 1 {
		t.Fatalf("applied scale %v", m)
	}
	if p, _ := rs.ListPlans("tr"); p["AAPLBUSDT"] != (StockRegimeTrimPlan{TargetScale: 0.5, TargetQty: 5}) {
		t.Fatalf("plans %+v", p)
	}
	if err := rs.Set("tr", "AAPLBUSDT", 0.8); err != nil { // applying the scale ends the plan
		t.Fatal(err)
	}
	if p, _ := rs.ListPlans("tr"); len(p) != 0 {
		t.Fatalf("Set must clear the plan: %+v", p)
	}
	if err := rs.SetPlan("tr", "AAPLBUSDT", 0.4, 3); err != nil { // an existing row keeps its applied scale
		t.Fatal(err)
	}
	if m, _ := rs.List("tr"); m["AAPLBUSDT"] != 0.8 {
		t.Fatalf("SetPlan changed the applied scale: %v", m)
	}
	if err := rs.ClearPlan("tr", "AAPLBUSDT"); err != nil {
		t.Fatal(err)
	}
	if p, _ := rs.ListPlans("tr"); len(p) != 0 {
		t.Fatalf("plans after clear: %+v", p)
	}
	if m, _ := rs.List("tr"); m["AAPLBUSDT"] != 0.8 {
		t.Fatalf("ClearPlan changed the applied scale: %v", m)
	}
}
