package stockengine

import (
	"math"
	"testing"
	"time"

	"nofx/market/usstock"
	"nofx/store"
)

func testContext() *Context {
	c := &Context{Now: time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC), Session: usstock.SessionRegular,
		Config:  &store.StockConfig{Symbols: []string{"AAPLBUSDT", "MSFTBUSDT", "QQQBUSDT"}},
		Account: Account{Equity: 10000, Available: 10000}}
	for _, symbol := range c.Config.Symbols {
		c.Snapshots = append(c.Snapshots, &SymbolSnapshot{Symbol: symbol, Underlying: symbol, Price: 100, ATR1d: 2,
			Quote: &usstock.Quote{BStockPrice: 100, RefFresh: true}, Sources: map[string]string{usstock.TF1d: usstock.SourceYahoo}})
	}
	return c
}
func buy(symbol string) Decision {
	return Decision{Symbol: symbol, Action: ActionOpenLong, EntryType: EntryMarket, StopLoss: 96, Confidence: 85, Thesis: "weeks-long trend continuation", Invalidation: "daily close below 96"}
}
func holding(c *Context) {
	c.Positions = []Position{{Symbol: "AAPLBUSDT", Quantity: 5, AvgPrice: 90, Price: 100, InitialStop: 85, StopPrice: 94, OpenedAt: c.Now.Add(-48 * time.Hour)}}
	c.Account.Exposure = 500
}
func assertCode(t *testing.T, v Verdict, code string) {
	t.Helper()
	if code == "" {
		if !v.Accepted || len(v.Codes) != 0 {
			t.Fatalf("expected accepted: %+v", v)
		}
		return
	}
	if v.Accepted || len(v.Codes) != 1 || v.Codes[0] != code {
		t.Fatalf("want %s: %+v", code, v)
	}
}

func TestValidationCodes(t *testing.T) {
	tests := []struct {
		name, code string
		setup      func(*Context, *Decision)
	}{
		{"open accepted", "", func(c *Context, d *Decision) {}},
		{"unknown symbol", "UNKNOWN_SYMBOL", func(c *Context, d *Decision) { d.Symbol = "OTHER" }},
		{"already held", "ALREADY_HELD", func(c *Context, d *Decision) { holding(c) }},
		{"not held", "NOT_HELD", func(c *Context, d *Decision) { d.Action = ActionAddLong }},
		{"pre disabled", "SESSION_NOT_ALLOWED", func(c *Context, d *Decision) { c.Session = usstock.SessionPre }},
		{"closed", "SESSION_CLOSED", func(c *Context, d *Decision) { c.Session = usstock.SessionClosed }},
		{"missing daily", "DATA_INSUFFICIENT", func(c *Context, d *Decision) { c.Snapshots[0].MissingTF = []string{usstock.TF1d} }},
		{"missing entry", "DATA_INSUFFICIENT", func(c *Context, d *Decision) { c.Snapshots[0].MissingTF = []string{usstock.TF1h} }},
		{"missing weekly position", "DATA_INSUFFICIENT", func(c *Context, d *Decision) {
			c.Config.Preset = store.StockPresetPosition
			c.Snapshots[0].MissingTF = []string{usstock.TF1w}
		}},
		{"missing snapshot", "DATA_INSUFFICIENT", func(c *Context, d *Decision) { c.Snapshots = nil }},
		{"missing ATR", "DATA_INSUFFICIENT", func(c *Context, d *Decision) { c.Snapshots[0].ATR1d = 0 }},
		{"divergence positive", "BSTOCK_DIVERGENCE", func(c *Context, d *Decision) { c.Snapshots[0].Quote.DivergencePct = 1.01 }},
		{"divergence negative", "BSTOCK_DIVERGENCE", func(c *Context, d *Decision) { c.Snapshots[0].Quote.DivergencePct = -1.01 }},
		{"divergence disabled", "", func(c *Context, d *Decision) { c.Config.MaxDivergencePct = -1; c.Snapshots[0].Quote.DivergencePct = 99 }},
		{"divergence threshold allowed", "", func(c *Context, d *Decision) { c.Snapshots[0].Quote.DivergencePct = 1 }},
		{"stale reference", "BSTOCK_REF_STALE", func(c *Context, d *Decision) { c.Snapshots[0].Quote.RefFresh = false }},
		{"missing reference", "BSTOCK_REF_STALE", func(c *Context, d *Decision) { c.Snapshots[0].Quote = nil }},
		{"disabled divergence still requires freshness", "BSTOCK_REF_STALE", func(c *Context, d *Decision) { c.Config.MaxDivergencePct = -1; c.Snapshots[0].Quote.RefFresh = false }},
		{"zero stop", "STOP_INVALID", func(c *Context, d *Decision) { d.StopLoss = 0 }},
		{"stop at entry", "STOP_INVALID", func(c *Context, d *Decision) { d.StopLoss = 100 }},
		{"stop too close", "STOP_OUT_OF_BAND", func(c *Context, d *Decision) { d.StopLoss = 98 }},
		{"stop too far", "STOP_OUT_OF_BAND", func(c *Context, d *Decision) { d.StopLoss = 93 }},
		{"minimum band allowed", "", func(c *Context, d *Decision) { d.StopLoss = 97 }},
		{"maximum band allowed", "", func(c *Context, d *Decision) { d.StopLoss = 94 }},
		{"TP invalid", "TP_INVALID", func(c *Context, d *Decision) { d.TakeProfit = 100 }},
		{"TP positive", "", func(c *Context, d *Decision) { d.TakeProfit = 110 }},
		{"limit zero", "LIMIT_INVALID", func(c *Context, d *Decision) { d.EntryType = EntryLimit }},
		{"limit chasing", "LIMIT_INVALID", func(c *Context, d *Decision) { d.EntryType = EntryLimit; d.LimitPrice = 101.01 }},
		{"limit at cap", "", func(c *Context, d *Decision) { d.EntryType = EntryLimit; d.LimitPrice = 101; d.StopLoss = 97 }},
		{"limit below market", "", func(c *Context, d *Decision) { d.EntryType = EntryLimit; d.LimitPrice = 98; d.StopLoss = 94 }},
		{"held position count", "MAX_POSITIONS", func(c *Context, d *Decision) { holding(c); d.Symbol = "MSFTBUSDT"; c.Config.MaxPositions = 1 }},
		{"add losing", "ADD_WHILE_LOSING", func(c *Context, d *Decision) { holding(c); d.Action = ActionAddLong; c.Positions[0].AvgPrice = 101 }},
		{"add breakeven", "ADD_WHILE_LOSING", func(c *Context, d *Decision) { holding(c); d.Action = ActionAddLong; c.Positions[0].AvgPrice = 100 }},
		{"add below initial stop", "ADD_WHILE_LOSING", func(c *Context, d *Decision) { holding(c); d.Action = ActionAddLong; c.Positions[0].InitialStop = 101 }},
		{"add winning", "", func(c *Context, d *Decision) { holding(c); d.Action = ActionAddLong }},
		{"reduce zero", "REDUCE_INVALID", func(c *Context, d *Decision) { holding(c); d.Action = ActionReduceLong }},
		{"reduce full", "REDUCE_INVALID", func(c *Context, d *Decision) { holding(c); d.Action = ActionReduceLong; d.ReduceFraction = 1 }},
		{"reduce half", "", func(c *Context, d *Decision) { holding(c); d.Action = ActionReduceLong; d.ReduceFraction = 0.5 }},
		{"loosen stop", "STOP_LOOSEN", func(c *Context, d *Decision) { holding(c); d.Action = ActionAdjustStop; d.StopLoss = 93 }},
		{"stop unchanged", "", func(c *Context, d *Decision) { holding(c); d.Action = ActionAdjustStop; d.StopLoss = 94 }},
		{"tighten stop closed", "", func(c *Context, d *Decision) {
			holding(c)
			d.Action = ActionAdjustStop
			d.StopLoss = 96
			c.Session = usstock.SessionClosed
		}},
		{"adjust invalid stop", "STOP_INVALID", func(c *Context, d *Decision) { holding(c); d.Action = ActionAdjustStop; d.StopLoss = 100 }},
		{"close closed", "", func(c *Context, d *Decision) {
			holding(c)
			d.Action = ActionCloseLong
			c.Session = usstock.SessionClosed
			c.Snapshots = nil
		}},
		{"hold closed", "", func(c *Context, d *Decision) {
			holding(c)
			d.Action = ActionHold
			c.Session = usstock.SessionClosed
			c.Snapshots = nil
		}},
		{"wait closed", "", func(c *Context, d *Decision) {
			d.Action = ActionWait
			c.Session = usstock.SessionClosed
			c.Snapshots = nil
		}},
		{"min notional", "MIN_NOTIONAL", func(c *Context, d *Decision) { c.Account.Equity = 20 }},
		{"no cash", "NO_CAPACITY", func(c *Context, d *Decision) { c.Account.Available = 0 }},
		{"unknown action", "UNKNOWN_ACTION", func(c *Context, d *Decision) { d.Action = "short" }},
		{"unknown entry", "ENTRY_INVALID", func(c *Context, d *Decision) { d.EntryType = "stop_limit" }},
		{"NaN stop", "STOP_INVALID", func(c *Context, d *Decision) { d.StopLoss = math.NaN() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testContext()
			d := buy("AAPLBUSDT")
			tt.setup(c, &d)
			v := Validate(c, []Decision{d})[0]
			assertCode(t, v, tt.code)
			if tt.code == "ALREADY_HELD" && v.Note == "" {
				t.Fatal("missing add_long suggestion")
			}
		})
	}
}

func TestNotHeldActions(t *testing.T) {
	for _, action := range []string{ActionAddLong, ActionReduceLong, ActionCloseLong, ActionAdjustStop, ActionHold} {
		t.Run(action, func(t *testing.T) {
			d := Decision{Symbol: "AAPLBUSDT", Action: action}
			assertCode(t, Validate(testContext(), []Decision{d})[0], "NOT_HELD")
		})
	}
}

func TestSessionOverrides(t *testing.T) {
	for _, session := range []usstock.Session{usstock.SessionPre, usstock.SessionAfter} {
		c := testContext()
		c.Session = session
		c.Config.Sessions.PreMarket, c.Config.Sessions.AfterHours = true, true
		assertCode(t, Validate(c, []Decision{buy("AAPLBUSDT")})[0], "")
	}
	c := testContext()
	disabled := false
	c.Config.Sessions.Regular = &disabled
	assertCode(t, Validate(c, []Decision{buy("AAPLBUSDT")})[0], "SESSION_NOT_ALLOWED")
}

func TestDuplicatesKeepFirstEvenIfRejected(t *testing.T) {
	for _, badFirst := range []bool{false, true} {
		c := testContext()
		d := buy("AAPLBUSDT")
		if badFirst {
			d.StopLoss = 100
		}
		v := Validate(c, []Decision{d, buy("AAPLBUSDT")})
		code := ""
		if badFirst {
			code = "STOP_INVALID"
		}
		assertCode(t, v[0], code)
		assertCode(t, v[1], "DUPLICATE")
	}
}

func TestMaxPositionsAcrossCycle(t *testing.T) {
	c := testContext()
	c.Config.MaxPositions = 1
	v := Validate(c, []Decision{buy("AAPLBUSDT"), buy("MSFTBUSDT")})
	assertCode(t, v[0], "")
	assertCode(t, v[1], "MAX_POSITIONS")
	// A rejected open must consume neither a slot nor exposure.
	d := buy("AAPLBUSDT")
	d.StopLoss = 100
	v = Validate(c, []Decision{d, buy("MSFTBUSDT")})
	assertCode(t, v[0], "STOP_INVALID")
	assertCode(t, v[1], "")
}

func TestExposureAccumulationAcrossCycle(t *testing.T) {
	c := testContext()
	c.Config.MaxTotalExposurePct = 20
	v := Validate(c, []Decision{buy("AAPLBUSDT"), buy("MSFTBUSDT")})
	assertCode(t, v[0], "")
	assertCode(t, v[1], "NO_CAPACITY")
	c.Config.MaxTotalExposurePct = 25
	v = Validate(c, []Decision{buy("AAPLBUSDT"), buy("MSFTBUSDT"), buy("QQQBUSDT")})
	assertCode(t, v[0], "")
	assertCode(t, v[1], "")
	assertCode(t, v[2], "NO_CAPACITY")
	if c.Account.Exposure != 0 || c.Account.Available != 10000 {
		t.Fatal("context mutated")
	}
}

func TestAddsReserveCapacityAndClosesDoNotReleaseIt(t *testing.T) {
	c := testContext()
	holding(c)
	c.Config.MaxTotalExposurePct = 20
	d := buy("AAPLBUSDT")
	d.Action = ActionAddLong
	v := Validate(c, []Decision{d, buy("MSFTBUSDT")})
	assertCode(t, v[0], "")
	assertCode(t, v[1], "NO_CAPACITY")
	c.Config.MaxPositions = 1
	v = Validate(c, []Decision{{Symbol: "AAPLBUSDT", Action: ActionCloseLong}, buy("MSFTBUSDT")})
	assertCode(t, v[0], "")
	assertCode(t, v[1], "MAX_POSITIONS")
}

func TestCashAccumulationAcrossCycle(t *testing.T) {
	c := testContext()
	c.Account.Available = 1000
	v := Validate(c, []Decision{buy("AAPLBUSDT"), buy("MSFTBUSDT")})
	assertCode(t, v[0], "")
	assertCode(t, v[1], "MIN_NOTIONAL") // 995 reserved, remaining order=4.975
}

func TestOptionalMissingTFDoesNotBlock(t *testing.T) {
	c := testContext()
	c.Snapshots[0].MissingTF = []string{usstock.TF15m, usstock.TF4h, usstock.TF1w}
	assertCode(t, Validate(c, []Decision{buy("AAPLBUSDT")})[0], "")
}

func TestConfidenceGateAndThesisRequired(t *testing.T) {
	c := testContext()
	for conf, want := range map[int]string{0: "LOW_CONFIDENCE", 79: "LOW_CONFIDENCE", 80: "", 100: ""} {
		d := buy("AAPLBUSDT")
		d.Confidence = conf
		assertCode(t, Validate(c, []Decision{d})[0], want)
	}
	if MinEntryConfidence != 80 {
		t.Fatal(MinEntryConfidence)
	}
	for name, mutate := range map[string]func(*Decision){
		"no thesis":        func(d *Decision) { d.Thesis = "" },
		"blank thesis":     func(d *Decision) { d.Thesis = " \t\n" },
		"no invalidation":  func(d *Decision) { d.Invalidation = "" },
		"blank invalidate": func(d *Decision) { d.Invalidation = "  " },
	} {
		d := buy("AAPLBUSDT")
		mutate(&d)
		v := Validate(c, []Decision{d})[0]
		assertCode(t, v, "THESIS_REQUIRED")
		if v.Note == "" {
			t.Fatalf("%s: note expected", name)
		}
	}
	// add_long is gated and needs a thesis too.
	h := testContext()
	holding(h)
	add := Decision{Symbol: "AAPLBUSDT", Action: ActionAddLong, EntryType: EntryMarket, StopLoss: 96, Confidence: 85, Thesis: "t", Invalidation: "i"}
	assertCode(t, Validate(h, []Decision{add})[0], "")
	add.Confidence = 70
	assertCode(t, Validate(h, []Decision{add})[0], "LOW_CONFIDENCE")
	add.Confidence, add.Thesis = 85, ""
	assertCode(t, Validate(h, []Decision{add})[0], "THESIS_REQUIRED")
}

func TestConfidenceGateNotAppliedToNonEntries(t *testing.T) {
	c := testContext()
	holding(c)
	for _, d := range []Decision{
		{Symbol: "AAPLBUSDT", Action: ActionHold},
		{Symbol: "AAPLBUSDT", Action: ActionReduceLong, ReduceFraction: 0.5},
		{Symbol: "AAPLBUSDT", Action: ActionCloseLong},
		{Symbol: "AAPLBUSDT", Action: ActionAdjustStop, StopLoss: 97},
	} {
		d.Confidence = 10
		assertCode(t, Validate(c, []Decision{d})[0], "")
	}
	assertCode(t, Validate(c, []Decision{{Symbol: "MSFTBUSDT", Action: ActionWait}})[0], "")
}

func TestThesisRequiredCombinesWithRepairableCodesOnly(t *testing.T) {
	c := testContext()
	// Bad stop AND no thesis → both codes, repairable.
	d := buy("AAPLBUSDT")
	d.StopLoss, d.Thesis = 99, ""
	v := Validate(c, []Decision{d})[0]
	if v.Accepted || len(v.Codes) != 2 || v.Codes[0] != "STOP_OUT_OF_BAND" || v.Codes[1] != "THESIS_REQUIRED" {
		t.Fatalf("%+v", v)
	}
	// Low confidence wins over a bad stop and a missing thesis, and is not repairable.
	d.Confidence = 50
	v = Validate(c, []Decision{d})[0]
	if len(v.Codes) != 1 || v.Codes[0] != "LOW_CONFIDENCE" {
		t.Fatalf("%+v", v)
	}
	// A non-repairable rejection never gains THESIS_REQUIRED.
	c.Session = usstock.SessionClosed
	d = buy("AAPLBUSDT")
	d.Thesis = ""
	assertCode(t, Validate(c, []Decision{d})[0], "SESSION_CLOSED")
}

func TestRepairableClassification(t *testing.T) {
	for _, code := range []string{"STOP_INVALID", "STOP_OUT_OF_BAND", "LIMIT_INVALID", "TP_INVALID", "ENTRY_INVALID", "THESIS_REQUIRED"} {
		if !RepairableCodes[code] {
			t.Fatalf("%s must be repairable", code)
		}
	}
	for _, code := range []string{"LOW_CONFIDENCE", "MAX_POSITIONS", "SESSION_CLOSED", "SESSION_NOT_ALLOWED", "BSTOCK_REF_STALE", "BSTOCK_DIVERGENCE",
		"DATA_INSUFFICIENT", "ALREADY_HELD", "DUPLICATE", "UNKNOWN_SYMBOL", "UNKNOWN_ACTION", "NOT_HELD", "ADD_WHILE_LOSING"} {
		if RepairableCodes[code] {
			t.Fatalf("%s must not be repairable", code)
		}
	}
	verdicts := []Verdict{
		{Decision: Decision{Symbol: "A", Action: ActionOpenLong}, Codes: []string{"STOP_OUT_OF_BAND", "THESIS_REQUIRED"}},
		{Decision: Decision{Symbol: "B", Action: ActionOpenLong}, Codes: []string{"LOW_CONFIDENCE"}},
		{Decision: Decision{Symbol: "C", Action: ActionAddLong}, Codes: []string{"TP_INVALID"}},
		{Decision: Decision{Symbol: "D", Action: ActionOpenLong}, Codes: []string{"STOP_INVALID", "MAX_POSITIONS"}},
		{Decision: Decision{Symbol: "E", Action: ActionAdjustStop}, Codes: []string{"STOP_INVALID"}},
		{Decision: Decision{Symbol: "F", Action: ActionOpenLong}, Accepted: true},
	}
	got := repairable(verdicts)
	if len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("repairable indexes %v", got)
	}
}
