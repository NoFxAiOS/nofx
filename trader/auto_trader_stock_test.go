package trader

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"nofx/kernel/stockengine"
	"nofx/market/usstock"
	"nofx/mcp"
	"nofx/store"
	"nofx/trader/types"
)

// us_stock (design 2026-10-09) tests: fake executor, fake AI, injected data.

type stockFakeAI struct {
	respond func(call int) string
	calls   int
	system  string // prompts of the latest call
	user    string
}

func (f *stockFakeAI) SetAPIKey(string, string, string) {}
func (f *stockFakeAI) SetTimeout(time.Duration)         {}
func (f *stockFakeAI) CallWithMessages(system, user string) (string, error) {
	f.calls++
	f.system, f.user = system, user
	return f.respond(f.calls), nil
}
func (f *stockFakeAI) CallWithRequest(*mcp.Request) (string, error) { return "", nil }
func (f *stockFakeAI) CallWithRequestStream(*mcp.Request, func(string)) (string, error) {
	return "", nil
}
func (f *stockFakeAI) CallWithRequestFull(*mcp.Request) (*mcp.LLMResponse, error) { return nil, nil }

type sellCall struct {
	Symbol string
	Qty    float64
}
type protCall struct {
	Symbol                   string
	Qty, Stop, Limit, Profit float64
}

// fakeSpot is an in-memory SpotStockTrader. Unimplemented Trader methods panic
// (nil embedded interface), proving the stock path does not use them.
type fakeSpot struct {
	types.Trader
	mu         sync.Mutex
	usdt       float64
	hold       map[string]float64
	price      float64
	prot       map[string]*types.ProtectionOrders
	buys       []float64
	limitBuys  int
	sells      []sellCall
	zeroSells  int
	setProt    []protCall
	cancelAll  int
	cancelProt int
	cancelled  []string
	preflights int
	seq        int
}

func newFakeSpot(price float64) *fakeSpot {
	return &fakeSpot{usdt: 10000, hold: map[string]float64{}, price: price, prot: map[string]*types.ProtectionOrders{}}
}

func (f *fakeSpot) GetBalance() (map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	eq := f.usdt
	for _, q := range f.hold {
		eq += q * f.price
	}
	return map[string]interface{}{"totalEquity": eq, "availableBalance": f.usdt, "totalWalletBalance": eq, "totalUnrealizedProfit": 0.0}, nil
}
func (f *fakeSpot) GetPositions() ([]map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []map[string]interface{}{}
	for sym, q := range f.hold {
		if q > 0.0009 {
			out = append(out, map[string]interface{}{"symbol": sym, "side": "long", "positionAmt": q, "entryPrice": f.price, "markPrice": f.price,
				"unRealizedProfit": 0.0, "leverage": 1.0, "liquidationPrice": 0.0})
		}
	}
	return out, nil
}
func (f *fakeSpot) GetMarketPrice(string) (float64, error) { return f.price, nil }
func (f *fakeSpot) FormatQuantity(_ string, q float64) (string, error) {
	return fmt.Sprint(floorStep(q, 0.001)), nil
}
func (f *fakeSpot) Preflight(string) error { f.preflights++; return nil }
func (f *fakeSpot) nextID() string         { f.seq++; return fmt.Sprint(1000 + f.seq) }
func (f *fakeSpot) BuyMarketNotional(sym string, notional float64) (*types.SpotOrderResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buys = append(f.buys, notional)
	qty := floorStep(notional/f.price, 0.001)
	f.hold[sym] += qty
	f.usdt -= qty * f.price
	return &types.SpotOrderResult{Symbol: sym, OrderID: f.nextID(), Side: "BUY", Type: "MARKET", Status: "FILLED", ExecutedQty: qty,
		AvgPrice: f.price, QuoteQty: qty * f.price}, nil
}
func (f *fakeSpot) BuyLimit(sym string, qty, price float64) (*types.SpotOrderResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.limitBuys++
	return &types.SpotOrderResult{Symbol: sym, OrderID: f.nextID(), Side: "BUY", Type: "LIMIT", Status: "NEW", OrigQty: qty, Price: price}, nil
}
func (f *fakeSpot) SellMarket(sym string, qty float64) (*types.SpotOrderResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sells = append(f.sells, sellCall{sym, qty})
	if qty == 0 {
		f.zeroSells++
		qty = f.hold[sym]
	}
	locked := 0.0
	if p := f.prot[sym]; p != nil {
		locked = p.Quantity
	}
	if qty > f.hold[sym]-locked+1e-9 {
		return nil, fmt.Errorf("sell quantity %v exceeds free balance %v", qty, f.hold[sym]-locked)
	}
	f.hold[sym] -= qty
	f.usdt += qty * f.price
	return &types.SpotOrderResult{Symbol: sym, OrderID: f.nextID(), Side: "SELL", Type: "MARKET", Status: "FILLED", ExecutedQty: qty,
		AvgPrice: f.price, QuoteQty: qty * f.price}, nil
}
func (f *fakeSpot) SetProtection(sym string, qty, stop, limit, tp float64) (*types.ProtectionOrders, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setProt = append(f.setProt, protCall{sym, qty, stop, limit, tp})
	if stop >= f.price {
		return nil, fmt.Errorf("invalid protection")
	}
	if qty > f.hold[sym]+1e-9 {
		return nil, fmt.Errorf("protection quantity exceeds held balance")
	}
	f.prot[sym] = &types.ProtectionOrders{Symbol: sym, Quantity: qty, StopPrice: stop, StopLimitPrice: limit, TakeProfit: tp, StopOrderID: f.nextID()}
	return f.prot[sym], nil
}
func (f *fakeSpot) GetProtection(sym string) (*types.ProtectionOrders, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p := f.prot[sym]; p != nil {
		c := *p
		return &c, nil
	}
	return nil, nil
}
func (f *fakeSpot) CancelProtection(sym string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelProt++
	delete(f.prot, sym)
	return nil
}
func (f *fakeSpot) CancelAllOrders(string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelAll++
	return nil
}
func (f *fakeSpot) CostBasis(string) (float64, float64, error) { return f.price, 0, nil }
func (f *fakeSpot) CancelOrder(symbol, orderID string) error {
	f.cancelled = append(f.cancelled, symbol+":"+orderID)
	return nil
}
func (f *fakeSpot) GetOrderStatus(string, string) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "NEW", "executedQty": 0.0, "avgPrice": 0.0, "commission": 0.0}, nil
}

// ------------------------------------------------------------------ fixture

var stockTestNow = time.Date(2026, 10, 14, 10, 0, 0, 0, stockET) // Wednesday, regular session

type stockFixture struct {
	at    *AutoTrader
	spot  *fakeSpot
	ai    *stockFakeAI
	price float64
	alert []string
}

const openAAPL = `Reasoning.
[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"market","stop_loss":115,"take_profit":130,"confidence":80,"reasoning":"trend up"}]`
const closeAAPL = `[{"symbol":"AAPLBUSDT","action":"close_long","confidence":70,"reasoning":"exit"}]`
const holdAAPL = `[{"symbol":"AAPLBUSDT","action":"hold","confidence":70,"reasoning":"keep"}]`

func stockTestBars(n int, step time.Duration, last float64, drift float64) []usstock.Bar {
	bars := make([]usstock.Bar, n)
	start := stockTestNow.Add(-time.Duration(n) * step)
	for i := range bars {
		c := last - float64(n-1-i)*drift
		bars[i] = usstock.Bar{OpenTime: start.Add(time.Duration(i) * step), Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 1000}
	}
	return bars
}

func newStockFixture(t *testing.T, paper bool, now time.Time, respond func(call int) string) *stockFixture {
	t.Helper()
	return newStockFixtureCfg(t, paper, now, respond, nil)
}

// newStockFixtureCfg is newStockFixture with a hook to adjust the strategy's
// StockConfig before the runtime is prepared. The scan seams default to "no
// report, no network"; scan tests override them after the fixture is built.
func newStockFixtureCfg(t *testing.T, paper bool, now time.Time, respond func(call int) string, mutate func(*store.StockConfig)) *stockFixture {
	t.Helper()
	fx := &stockFixture{price: 120}
	oldLookup, oldSeries, oldQuote, oldNotify := stockLookupSymbol, stockGetSeries, stockGetQuote, stockNotify
	oldGet, oldWarm, oldCached, oldList := stockGetScanReport, stockWarmScan, stockCachedScan, stockListUniverse
	t.Cleanup(func() {
		stockLookupSymbol, stockGetSeries, stockGetQuote, stockNotify = oldLookup, oldSeries, oldQuote, oldNotify
		stockGetScanReport, stockWarmScan, stockCachedScan, stockListUniverse = oldGet, oldWarm, oldCached, oldList
	})
	stockGetScanReport = func(context.Context, time.Duration) (*stockengine.ScanReport, error) {
		return nil, fmt.Errorf("scan not stubbed")
	}
	stockWarmScan = func(time.Duration) {}
	stockCachedScan = func() *stockengine.ScanReport { return nil }
	stockListUniverse = func(context.Context) ([]usstock.SymbolInfo, error) { return nil, fmt.Errorf("universe not stubbed") }
	stockLookupSymbol = func(_ context.Context, sym string) (usstock.SymbolInfo, bool) {
		switch sym {
		case "AAPLBUSDT", "SPYBUSDT", "MSFTBUSDT", "NVDABUSDT", "TSLABUSDT", "AMDBUSDT", "METABUSDT":
			u := strings.TrimSuffix(sym, "BUSDT")
			return usstock.SymbolInfo{Symbol: sym, BaseAsset: u + "B", Underlying: u, Status: "TRADING", TickSize: 0.01, StepSize: 0.001, MinQty: 0.001, MinNotional: 5}, true
		}
		return usstock.SymbolInfo{}, false
	}
	stockGetSeries = func(_ context.Context, sym, tf string, need int, _ bool) (*usstock.Series, error) {
		step := map[string]time.Duration{usstock.TF1d: 24 * time.Hour, usstock.TF1w: 7 * 24 * time.Hour, usstock.TF1h: time.Hour, usstock.TF4h: 4 * time.Hour}[tf]
		drift := 0.1
		if tf == usstock.TF1h {
			drift = 0.01
		}
		return &usstock.Series{Symbol: sym, Underlying: strings.TrimSuffix(sym, "BUSDT"), Timeframe: tf, Source: usstock.SourceBStock,
			Bars: stockTestBars(need, step, 120, drift)}, nil
	}
	stockGetQuote = func(_ context.Context, sym string) (*usstock.Quote, error) {
		return &usstock.Quote{Symbol: sym, BStockPrice: fx.price, RefPrice: fx.price, RefTime: now.Add(-time.Minute), RefFresh: true, Session: usstock.SessionAt(now)}, nil
	}
	stockNotify = func(kind, title, msg string) { fx.alert = append(fx.alert, kind+"|"+msg) }

	st, err := store.New(filepath.Join(t.TempDir(), "stock.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Trader().Create(&store.Trader{ID: "t1", UserID: "u"}); err != nil {
		t.Fatal(err)
	}
	fx.ai = &stockFakeAI{respond: respond}
	fx.spot = newFakeSpot(120)
	sc := &store.StockConfig{Symbols: []string{"AAPLBUSDT"}, PaperTrading: &paper}
	if mutate != nil {
		mutate(sc)
	}
	at := &AutoTrader{id: "t1", userID: "u", name: "stock-test", store: st, mcpClient: fx.ai, initialBalance: 10000, isRunning: true,
		stopMonitorCh: make(chan struct{}), stockNow: func() time.Time { return now },
		config: AutoTraderConfig{ID: "t1", ScanInterval: time.Minute, StrategyConfig: &store.StrategyConfig{StrategyType: store.StrategyTypeUSStock, StockConfig: sc}}}
	if !paper {
		at.stockTrader = fx.spot
		at.trader = fx.spot
	}
	if _, err := at.prepareStockRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	fx.at = at
	return fx
}

func (fx *stockFixture) cycle(t *testing.T) {
	t.Helper()
	if err := fx.at.runStockCycle(context.Background()); err != nil {
		t.Fatalf("cycle: %v", err)
	}
}

func (fx *stockFixture) lastRecord(t *testing.T) *store.DecisionRecord {
	t.Helper()
	recs, err := fx.at.store.Decision().GetLatestRecords("t1", 20)
	if err != nil || len(recs) == 0 {
		t.Fatalf("decision record: %v %d", err, len(recs))
	}
	last := recs[0]
	for _, r := range recs { // the fixed test clock gives every record one timestamp
		if r.ID > last.ID {
			last = r
		}
	}
	return last
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// ---------------------------------------------------------------- scheduler

func TestNextStockDecisionTime(t *testing.T) {
	swing := []string{"09:45", "12:30", "15:30"}
	et := func(y int, m time.Month, d, h, mi int) time.Time { return time.Date(y, m, d, h, mi, 0, 0, stockET) }
	cases := []struct {
		name  string
		after time.Time
		slots []string
		want  time.Time
	}{
		{"same day next slot", et(2026, 10, 14, 10, 0), swing, et(2026, 10, 14, 12, 30)},
		{"weekend skips to Monday", et(2026, 10, 9, 16, 0), swing, et(2026, 10, 12, 9, 45)},
		{"holiday 2026-11-26 skipped", et(2026, 11, 25, 16, 0), swing, et(2026, 11, 27, 9, 45)},
		{"half day 15:30 slot still fires after the 13:00 close", et(2026, 11, 27, 12, 31), swing, et(2026, 11, 27, 15, 30)},
		{"position preset single slot", et(2026, 10, 14, 15, 30), []string{"15:30"}, et(2026, 10, 15, 15, 30)},
	}
	for _, c := range cases {
		if got := nextStockDecisionTime(c.after, c.slots); !got.Equal(c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	// DST: 09:45 ET is 13:45Z in EDT and 14:45Z in EST.
	if got := nextStockDecisionTime(et(2026, 10, 30, 16, 0), swing); !got.Equal(time.Date(2026, 11, 2, 14, 45, 0, 0, time.UTC)) {
		t.Errorf("EST after fall-back: %v", got.UTC())
	}
	if got := nextStockDecisionTime(et(2026, 3, 6, 16, 0), swing); !got.Equal(time.Date(2026, 3, 9, 13, 45, 0, 0, time.UTC)) {
		t.Errorf("EDT after spring-forward: %v", got.UTC())
	}
}

// -------------------------------------------------------------------- paper

func TestStockPaperCycleOpenThenClose(t *testing.T) {
	fx := newStockFixture(t, true, stockTestNow, func(call int) string {
		if call == 1 {
			return openAAPL
		}
		return closeAAPL
	})
	fx.spot.price = 120
	fx.cycle(t)

	open, err := fx.at.store.StockPaper().ListOpen("t1")
	if err != nil || len(open) != 1 {
		t.Fatalf("paper positions: %v %d", err, len(open))
	}
	p := open[0]
	if p.Symbol != "AAPLBUSDT" || !near(p.Quantity, 16.666) || !near(p.AvgPrice, 120) || p.InitialStop != 115 || p.Stop != 115 || p.TakeProfit != 130 {
		t.Fatalf("unexpected paper position %+v", p)
	}
	rec := fx.lastRecord(t)
	if !rec.Success || len(rec.Decisions) != 1 || !rec.Decisions[0].Success || rec.Decisions[0].Action != "open_long" {
		t.Fatalf("decision record %+v", rec)
	}
	for _, line := range rec.ExecutionLog {
		if !strings.HasPrefix(line, "[PAPER]") {
			t.Fatalf("execution log line without [PAPER] prefix: %q", line)
		}
	}
	if len(fx.spot.buys)+len(fx.spot.sells)+len(fx.spot.setProt)+fx.spot.cancelAll != 0 {
		t.Fatal("paper mode must not call the executor")
	}
	if rows, _ := fx.at.store.Position().GetOpenPositions("t1"); len(rows) != 0 {
		t.Fatal("paper trades must not enter trader_positions")
	}

	fx.price = 126
	fx.cycle(t)
	if open, _ := fx.at.store.StockPaper().ListOpen("t1"); len(open) != 0 {
		t.Fatal("paper position not closed")
	}
	closed, _ := fx.at.store.StockPaper().ListClosed("t1", 5)
	if len(closed) != 1 || closed[0].Status != store.StockPaperClosed || !near(closed[0].ExitPrice, 126) || !near(closed[0].RealizedPnL, 6*16.666) {
		t.Fatalf("closed paper position %+v", closed)
	}
	acct, _ := fx.at.store.StockPaper().GetAccount("t1")
	if !near(acct.Cash, 10000+6*16.666) {
		t.Fatalf("paper cash %v", acct.Cash)
	}
	if rec := fx.lastRecord(t); !rec.Decisions[0].Success || rec.Decisions[0].Action != "close_long" || len(fx.spot.sells) != 0 {
		t.Fatalf("second cycle record %+v", rec)
	}
}

func TestStockPaperTickStopAndTakeProfit(t *testing.T) {
	fx := newStockFixture(t, true, stockTestNow, func(int) string { return openAAPL })
	fx.cycle(t)
	fx.price = 114
	fx.at.stockProtectionTick(context.Background())
	closed, _ := fx.at.store.StockPaper().ListClosed("t1", 5)
	if len(closed) != 1 || closed[0].CloseReason != "stop_loss" || !near(closed[0].ExitPrice, 115) {
		t.Fatalf("paper stop not simulated: %+v", closed)
	}
}

// --------------------------------------------------------------------- live

func TestStockLiveCycleOpenThenClose(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(call int) string {
		if call == 1 {
			return openAAPL
		}
		return closeAAPL
	})
	fx.cycle(t)

	if fx.spot.preflights != 1 {
		t.Fatalf("preflight calls %d", fx.spot.preflights)
	}
	if len(fx.spot.buys) != 1 || !near(fx.spot.buys[0], 1999.92) {
		t.Fatalf("market buy notional %v", fx.spot.buys)
	}
	if len(fx.spot.setProt) != 1 {
		t.Fatalf("protection calls %v", fx.spot.setProt)
	}
	pc := fx.spot.setProt[0]
	if pc.Symbol != "AAPLBUSDT" || !near(pc.Qty, 16.666) || pc.Stop != 115 || !near(pc.Limit, 114.65) || pc.Profit != 130 {
		t.Fatalf("protection %+v", pc)
	}
	rows, _ := fx.at.store.Position().GetOpenPositions("t1")
	if len(rows) != 1 {
		t.Fatalf("rows %d", len(rows))
	}
	r := rows[0]
	if r.Side != "LONG" || !near(r.Quantity, 16.666) || r.InitialStopLoss != 115 || !r.AIManaged || r.Leverage != 1 || r.EntryOrderID == "" || r.ExchangeType != "binance" {
		t.Fatalf("position row %+v", r)
	}
	if rec := fx.lastRecord(t); !rec.Decisions[0].Success || rec.Decisions[0].EntryOrderID != r.EntryOrderID {
		t.Fatalf("record %+v", rec.Decisions)
	}

	fx.price, fx.spot.price = 126, 126
	fx.cycle(t)
	if fx.spot.zeroSells != 0 || len(fx.spot.sells) != 1 || !near(fx.spot.sells[0].Qty, 16.666) {
		t.Fatalf("sells %+v", fx.spot.sells)
	}
	if rows, _ := fx.at.store.Position().GetOpenPositions("t1"); len(rows) != 0 {
		t.Fatal("position row still open")
	}
	closed, _ := fx.at.store.Position().GetClosedPositions("t1", 5)
	if len(closed) != 1 || closed[0].Status != "CLOSED" || !near(closed[0].ExitPrice, 126) || !near(closed[0].RealizedPnL, 6*16.666) {
		t.Fatalf("closed row %+v", closed)
	}
	journal, total, err := fx.at.store.TradeJournal().List("t1", 10, 0, "", "")
	if err != nil || total != 1 || journal[0].AIManaged == nil || !*journal[0].AIManaged || journal[0].PlannedStopLoss != 115 {
		t.Fatalf("journal %v %d %+v", err, total, journal)
	}
	if fx.spot.cancelAll != 0 {
		t.Fatal("CancelAllOrders must never be called from the stock path")
	}
}

func TestStockRejectedVerdictPlacesNoOrders(t *testing.T) {
	saturday := time.Date(2026, 10, 10, 11, 0, 0, 0, stockET)
	fx := newStockFixture(t, false, saturday, func(int) string { return openAAPL })
	fx.cycle(t)
	if len(fx.spot.buys)+len(fx.spot.setProt)+len(fx.spot.sells) != 0 {
		t.Fatal("rejected decision must not place orders")
	}
	rec := fx.lastRecord(t)
	found := false
	for _, l := range rec.ExecutionLog {
		found = found || strings.Contains(l, "SESSION_CLOSED")
	}
	if !found || rec.Decisions[0].Success || !strings.Contains(rec.Decisions[0].Error, "SESSION_CLOSED") {
		t.Fatalf("expected SESSION_CLOSED, log=%v decisions=%+v", rec.ExecutionLog, rec.Decisions)
	}
	rejectedAlert := false
	for _, a := range fx.alert {
		rejectedAlert = rejectedAlert || strings.HasPrefix(a, "ORDER|") && strings.Contains(a, "SESSION_CLOSED")
	}
	if !rejectedAlert {
		t.Fatalf("expected a single rejected-opens summary, got %v", fx.alert)
	}
}

// openLiveOwned seeds a live program position: the executor holds `balance`,
// of which `owned` was bought by the program.
func (fx *stockFixture) openLiveOwned(t *testing.T, owned, balance float64) {
	t.Helper()
	if _, err := fx.at.stockRecordBuy("AAPLBUSDT", owned, 120, 0, "seed1", 115, stockTestNow); err != nil {
		t.Fatal(err)
	}
	fx.spot.hold["AAPLBUSDT"] = balance
}

func TestStockProtectionTickReplacesMissingAndFallsBack(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 16.666, 16.666)
	ctx := context.Background()

	fx.at.stockProtectionTick(ctx) // no protection yet → re-placed from the persisted opening stop
	if len(fx.spot.setProt) != 1 || fx.spot.setProt[0].Stop != 115 || !near(fx.spot.setProt[0].Qty, 16.666) {
		t.Fatalf("protection not re-placed: %+v", fx.spot.setProt)
	}
	alerted := false
	for _, a := range fx.alert {
		alerted = alerted || strings.HasPrefix(a, "ALERT|")
	}
	if !alerted {
		t.Fatal("missing-protection alert expected")
	}

	fx.price, fx.spot.price = 114, 114 // below the stop, stop order still open
	fx.at.stockProtectionTick(ctx)
	if len(fx.spot.sells) != 0 {
		t.Fatal("must wait for a second tick before the program fallback")
	}
	fx.at.stockProtectionTick(ctx)
	if len(fx.spot.sells) != 1 || fx.spot.zeroSells != 0 || !near(fx.spot.sells[0].Qty, 16.666) {
		t.Fatalf("fallback sell %+v", fx.spot.sells)
	}
	if rows, _ := fx.at.store.Position().GetOpenPositions("t1"); len(rows) != 0 {
		t.Fatal("fallback sell must close the position row")
	}
}

func TestStockManualBalanceIsNotSold(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return closeAAPL })
	fx.openLiveOwned(t, 16.666, 21.666) // 5 AAPLB were bought manually
	if _, err := fx.spot.SetProtection("AAPLBUSDT", 16.666, 115, 114.65, 130); err != nil {
		t.Fatal(err)
	}
	fx.cycle(t)
	if len(fx.spot.sells) != 1 || fx.spot.zeroSells != 0 || !near(fx.spot.sells[0].Qty, 16.666) {
		t.Fatalf("close must sell only the program-owned quantity: %+v", fx.spot.sells)
	}
	if !near(fx.spot.hold["AAPLBUSDT"], 5) {
		t.Fatalf("manual holding was touched: %v", fx.spot.hold["AAPLBUSDT"])
	}
	if fx.spot.cancelAll != 0 {
		t.Fatal("CancelAllOrders called")
	}
	rec := fx.lastRecord(t)
	manualLogged := false
	for _, l := range rec.ExecutionLog {
		manualLogged = manualLogged || strings.Contains(l, "manual/external holding")
	}
	if !manualLogged {
		t.Fatalf("manual holding should be logged: %v", rec.ExecutionLog)
	}
}

func TestStockBalanceBelowOwnedShrinksPosition(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 16.666, 10) // user sold 6.666 manually
	if _, err := fx.spot.SetProtection("AAPLBUSDT", 10, 115, 114.65, 130); err != nil {
		t.Fatal(err)
	}
	fx.cycle(t)
	rows, _ := fx.at.store.Position().GetOpenPositions("t1")
	if len(rows) != 1 || !near(rows[0].Quantity, 10) || !near(rows[0].EntryQuantity, 10) {
		t.Fatalf("position not shrunk to the balance: %+v", rows)
	}
	if len(fx.spot.sells) != 0 {
		t.Fatal("shrinking is bookkeeping only")
	}
	alerted := false
	for _, a := range fx.alert {
		alerted = alerted || strings.HasPrefix(a, "ALERT|") && strings.Contains(a, "外部减仓")
	}
	if !alerted {
		t.Fatalf("external-reduction alert expected: %v", fx.alert)
	}
	logged := false
	for _, l := range fx.lastRecord(t).ExecutionLog {
		logged = logged || strings.Contains(l, "external reduction") && strings.Contains(l, "no PnL attributed")
	}
	if !logged {
		t.Fatal("execution log should note the external reduction")
	}
	if closed, _ := fx.at.store.Position().GetClosedPositions("t1", 5); len(closed) != 0 {
		t.Fatal("a partial external reduction must not close the row")
	}
}

// ------------------------------------------------------------ Run() branch

func TestRunStockBranchSkipsFuturesMonitors(t *testing.T) {
	saturday := time.Date(2026, 10, 10, 11, 0, 0, 0, stockET) // session closed: no immediate cycle
	fx := newStockFixture(t, true, saturday, func(int) string { return holdAAPL })
	at := fx.at
	at.isRunning = false
	futures, stockLoops := 0, 0
	started := make(chan struct{}, 1)
	onFuturesMonitorsStart = func(*AutoTrader) { futures++ }
	onStockLoopStart = func(*AutoTrader) { stockLoops++; started <- struct{}{} }
	t.Cleanup(func() { onFuturesMonitorsStart, onStockLoopStart = nil, nil })

	if err := at.beginRun(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- at.run() }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("stock loop did not start")
	}
	at.Stop()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}
	if futures != 0 || stockLoops != 1 {
		t.Fatalf("futures monitors started=%d stock loops=%d", futures, stockLoops)
	}

	// Positive control: a grid trader still reaches the futures monitors.
	grid := lifecycleFixture(t)
	if err := grid.beginRun(); err != nil {
		t.Fatal(err)
	}
	_ = grid.run()
	grid.Stop()
	if futures != 1 {
		t.Fatalf("control: futures monitors hook fired %d times", futures)
	}
}

func TestNewAutoTraderStockRequiresBinanceAndLinkedExecutor(t *testing.T) {
	sc := &store.StrategyConfig{StrategyType: store.StrategyTypeUSStock, StockConfig: &store.StockConfig{Symbols: []string{"AAPLBUSDT"}}}
	if _, err := NewAutoTrader(AutoTraderConfig{ID: "x", Exchange: "bybit", StrategyConfig: sc}, nil, "u"); err == nil || !strings.Contains(err.Error(), "Binance") {
		t.Fatalf("expected exchange error, got %v", err)
	}
	live := false
	sc.StockConfig.PaperTrading = &live
	old := newSpotStockTrader
	t.Cleanup(func() { newSpotStockTrader = old })
	newSpotStockTrader = nil
	if _, err := NewAutoTrader(AutoTraderConfig{ID: "x", Exchange: "binance", StrategyConfig: sc}, nil, "u"); err == nil || !strings.Contains(err.Error(), "not linked") {
		t.Fatalf("expected not-linked error, got %v", err)
	}
	fake := newFakeSpot(100)
	newSpotStockTrader = func(string, string) (types.SpotStockTrader, error) { return fake, nil }
	at, err := NewAutoTrader(AutoTraderConfig{ID: "x", Name: "x", Exchange: "binance", StrategyConfig: sc}, nil, "u")
	if err != nil {
		t.Fatal(err)
	}
	if at.stockTrader == nil || at.initialBalance != 10000 || !at.IsStockStrategy() {
		t.Fatalf("live stock trader not built: %+v", at.initialBalance)
	}
}

func TestStockLimitEntryRestsUntilTheRegularClose(t *testing.T) {
	limit := `[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"limit","limit_price":119,"stop_loss":115,"take_profit":130,"confidence":80,"reasoning":"pullback"}]`
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return limit })
	fx.cycle(t)
	if fx.spot.limitBuys != 1 || len(fx.spot.setProt) != 0 {
		t.Fatalf("limit buys %d, protection %v", fx.spot.limitBuys, fx.spot.setProt)
	}
	pend, _ := fx.at.store.StockPaper().ListPending("t1")
	if len(pend) != 1 || pend[0].Symbol != "AAPLBUSDT" || pend[0].Stop != 115 {
		t.Fatalf("pending not persisted: %+v", pend)
	}
	fx.cycle(t) // the symbol has a resting entry: a second entry is refused
	if fx.spot.limitBuys != 1 {
		t.Fatal("second limit entry placed while one is resting")
	}
	if fx.spot.cancelAll != 0 || fx.spot.cancelProt != 0 || len(fx.spot.cancelled) != 0 {
		t.Fatal("the resting entry must survive within its day")
	}
	// At the regular close the unfilled entry expires: exactly that order is
	// cancelled (never CancelAllOrders), and the row stays until the exchange
	// reports the order terminal.
	fx.at.stockNow = func() time.Time { return time.Date(2026, 10, 14, 16, 1, 0, 0, stockET) }
	fx.at.stockProtectionTick(context.Background())
	if len(fx.spot.cancelled) != 1 || fx.spot.cancelAll != 0 {
		t.Fatalf("expired entry not cancelled exactly once: %v (cancelAll %d)", fx.spot.cancelled, fx.spot.cancelAll)
	}
}

func TestStockPendingExpiry(t *testing.T) {
	cases := []struct{ placed, want time.Time }{
		// regular session → same day's close
		{time.Date(2026, 10, 14, 10, 0, 0, 0, stockET), time.Date(2026, 10, 14, 16, 0, 0, 0, stockET)},
		// after hours → next trading day's close
		{time.Date(2026, 10, 14, 17, 0, 0, 0, stockET), time.Date(2026, 10, 15, 16, 0, 0, 0, stockET)},
		// Friday after hours → Monday close
		{time.Date(2026, 10, 16, 18, 0, 0, 0, stockET), time.Date(2026, 10, 19, 16, 0, 0, 0, stockET)},
		// half day (2026-11-27) → 13:00 close
		{time.Date(2026, 11, 27, 10, 0, 0, 0, stockET), time.Date(2026, 11, 27, 13, 0, 0, 0, stockET)},
		// Thanksgiving placement → next trading day (half day) close
		{time.Date(2026, 11, 26, 10, 0, 0, 0, stockET), time.Date(2026, 11, 27, 13, 0, 0, 0, stockET)},
	}
	for _, c := range cases {
		if got := stockPendingExpiry(c.placed); !got.Equal(c.want) {
			t.Errorf("placed %s: expiry %s, want %s", c.placed, got.In(stockET), c.want)
		}
	}
}

func TestStockExchangeSideStopFillIsBookedAtStop(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 16.666, 16.666)
	fx.at.stockProtectionTick(context.Background()) // places protection, remembers stop 115
	fx.spot.hold["AAPLBUSDT"] = 0                   // the exchange stop filled
	delete(fx.spot.prot, "AAPLBUSDT")
	fx.price, fx.spot.price = 114.5, 114.5
	fx.at.stockProtectionTick(context.Background())
	closed, _ := fx.at.store.Position().GetClosedPositions("t1", 5)
	if len(closed) != 1 || closed[0].CloseReason != "stop_loss" || !near(closed[0].ExitPrice, 115) || closed[0].RealizedPnL >= 0 {
		t.Fatalf("exchange stop fill not booked: %+v", closed)
	}
}

// ---------------------------------------------------------------- scan mode

func scanRes(sym, phase string, long bool, score float64) stockengine.ScanResult {
	return stockengine.ScanResult{Symbol: sym, Underlying: strings.TrimSuffix(sym, "BUSDT"), Source: usstock.SourceBStock,
		Cycle: stockengine.CycleAssessment{Phase: stockengine.CyclePhase(phase), LongSetup: long, Score: score, RS63Pct: 5, VolRatio: 1.1}}
}

// momRes is a scan result carrying a momentum read (Valid), with a Kell phase
// that deliberately does not drive the momentum ranking.
func momRes(sym, phase string, long bool, score, mom float64, above bool) stockengine.ScanResult {
	r := scanRes(sym, phase, long, score)
	r.Momentum = stockengine.MomentumAssessment{Valid: true, Mom12_1Pct: mom, AboveSMA200: above}
	return r
}

func scanReportOf(results ...stockengine.ScanResult) *stockengine.ScanReport {
	return &stockengine.ScanReport{GeneratedAt: stockTestNow.Add(-10 * time.Minute), Results: results}
}

func useScanReport(report *stockengine.ScanReport, err error) {
	stockGetScanReport = func(context.Context, time.Duration) (*stockengine.ScanReport, error) { return report, err }
}

func scanMode(topN int, pins, exclude []string) func(*store.StockConfig) {
	return symbolSourceMode(store.StockSymbolSourceScan, topN, pins, exclude)
}

func hybridMode(topN int, pins, exclude []string) func(*store.StockConfig) {
	return symbolSourceMode(store.StockSymbolSourceHybrid, topN, pins, exclude)
}

func symbolSourceMode(source string, topN int, pins, exclude []string) func(*store.StockConfig) {
	return func(sc *store.StockConfig) {
		sc.SymbolSource, sc.ScanTopN, sc.Symbols, sc.ScanExclude = source, topN, pins, exclude
	}
}

const waitAll = `[{"symbol":"AAPLBUSDT","action":"wait"},{"symbol":"NVDABUSDT","action":"wait"},{"symbol":"MSFTBUSDT","action":"wait"}]`

func TestStockScanCycleComposesCandidates(t *testing.T) {
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return waitAll }, hybridMode(2, []string{"AAPLBUSDT"}, []string{"TSLABUSDT"}))
	useScanReport(scanReportOf(
		scanRes("NVDABUSDT", "wedge_pop", true, 90),
		scanRes("TSLABUSDT", "ema_crossback", true, 85), // excluded
		scanRes("AAPLBUSDT", "ema_crossback", true, 80), // already pinned
		scanRes("MSFTBUSDT", "base_n_break", true, 70),
		scanRes("AMDBUSDT", "ema_crossback", true, 60), // beyond top N
		scanRes("METABUSDT", "uptrend", false, 50),     // not a long setup
	), nil)
	fx.cycle(t)

	rec := fx.lastRecord(t)
	if !rec.Success {
		t.Fatalf("cycle failed: %s", rec.ErrorMessage)
	}
	if got := strings.Join(rec.CandidateCoins, ","); got != "AAPLBUSDT,NVDABUSDT,MSFTBUSDT" {
		t.Fatalf("candidates %s", got)
	}
	if fx.ai.calls != 1 {
		t.Fatalf("ai calls %d", fx.ai.calls)
	}
	for _, sym := range []string{"AAPLBUSDT (AAPL)", "NVDABUSDT (NVDA)", "MSFTBUSDT (MSFT)"} {
		if !strings.Contains(fx.ai.user, sym) {
			t.Fatalf("user prompt missing %s:\n%s", sym, fx.ai.user)
		}
	}
	for _, sym := range []string{"TSLABUSDT", "AMDBUSDT", "METABUSDT"} {
		if strings.Contains(fx.ai.user, sym) {
			t.Fatalf("user prompt must not carry %s:\n%s", sym, fx.ai.user)
		}
	}
	// Cycle lines: NVDA / MSFT / AAPL come from the report, SPY is classified locally.
	for _, want := range []string{"价格周期(Kell, 日线): phase=wedge_pop", "phase=base_n_break", "phase=ema_crossback"} {
		if !strings.Contains(fx.ai.user, want) {
			t.Fatalf("user prompt missing %q:\n%s", want, fx.ai.user)
		}
	}
	if n := strings.Count(fx.ai.user, "phase="); n != 4 {
		t.Fatalf("want a cycle line for SPY + 3 candidates, got %d:\n%s", n, fx.ai.user)
	}
	if !strings.Contains(fx.ai.system, "周期扫描器") {
		t.Fatal("scan mode system prompt lacks the scanner note")
	}
	if got := fx.at.stock.cfg.Symbols; len(got) != 1 || got[0] != "AAPLBUSDT" {
		t.Fatalf("runtime config must not be mutated: %v", got)
	}
	logged := strings.Join(rec.ExecutionLog, "\n")
	if !strings.Contains(logged, "scan top 2 rank=kell: NVDABUSDT(wedge_pop 90.0) MSFTBUSDT(base_n_break 70.0)") || !strings.Contains(logged, "report age 10m0s") {
		t.Fatalf("scan log line missing:\n%s", logged)
	}
}

func withScanRank(rank string, inner func(*store.StockConfig)) func(*store.StockConfig) {
	return func(sc *store.StockConfig) {
		inner(sc)
		sc.ScanRank = rank
	}
}

func TestStockScanMomentumRankPicksHighestMomentumAboveSMA(t *testing.T) {
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return waitAll },
		withScanRank(store.StockScanRankMomentum, scanMode(2, nil, []string{"TSLABUSDT"})))
	useScanReport(scanReportOf(
		// Kell order first (long setups, high score): none of it matters for momentum rank.
		momRes("MSFTBUSDT", "wedge_pop", true, 95, 20, true),
		momRes("AAPLBUSDT", "ema_crossback", true, 90, 35, true),
		momRes("AMDBUSDT", "wedge_pop", true, 85, 200, false), // huge momentum but below SMA200
		momRes("TSLABUSDT", "uptrend", false, 10, 150, true),  // excluded
		momRes("NVDABUSDT", "downtrend", false, 5, 60, true),  // no Kell setup, strongest eligible
		momRes("METABUSDT", "uptrend", false, 20, 10, true),   // beyond top N
	), nil)
	fx.cycle(t)

	rec := fx.lastRecord(t)
	if !rec.Success {
		t.Fatalf("cycle failed: %s", rec.ErrorMessage)
	}
	if got := strings.Join(rec.CandidateCoins, ","); got != "NVDABUSDT,AAPLBUSDT" {
		t.Fatalf("candidates %s", got)
	}
	logged := strings.Join(rec.ExecutionLog, "\n")
	if !strings.Contains(logged, "scan top 2 rank=momentum: NVDABUSDT(mom +60.0% downtrend) AAPLBUSDT(mom +35.0% ema_crossback)") {
		t.Fatalf("scan log line missing:\n%s", logged)
	}
	for _, want := range []string{"mom12_1=+60.0% above_sma200=true", "mom12_1=+35.0% above_sma200=true"} {
		if !strings.Contains(fx.ai.user, want) {
			t.Fatalf("user prompt missing %q:\n%s", want, fx.ai.user)
		}
	}
	if strings.Contains(fx.ai.user, "AMDBUSDT") || strings.Contains(fx.ai.user, "MSFTBUSDT") {
		t.Fatalf("user prompt must carry only the momentum picks:\n%s", fx.ai.user)
	}
	if !strings.Contains(fx.ai.system, "12-1 动量") || !strings.Contains(fx.ai.system, "exhaustion_extension 阶段不追高") {
		t.Fatalf("momentum rank system prompt missing:\n%s", fx.ai.system)
	}
}

func TestStockScanMomentumRankIgnoresBelowSMAEvenIfFewerThanN(t *testing.T) {
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string {
		return `[{"symbol":"NVDABUSDT","action":"wait"}]`
	}, withScanRank(store.StockScanRankMomentum, scanMode(5, nil, nil)))
	invalid := scanRes("MSFTBUSDT", "wedge_pop", true, 99) // Momentum zero value: not Valid
	useScanReport(scanReportOf(
		invalid,
		momRes("AMDBUSDT", "wedge_pop", true, 90, 300, false),
		momRes("NVDABUSDT", "ema_crossback", true, 50, 12, true),
	), nil)
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if !rec.Success || strings.Join(rec.CandidateCoins, ",") != "NVDABUSDT" {
		t.Fatalf("only the one above-SMA200 name may be a candidate: %+v", rec.CandidateCoins)
	}

	// Nothing above the SMA200: no candidates, no AI call.
	fx2 := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return waitAll },
		withScanRank(store.StockScanRankMomentum, scanMode(5, nil, nil)))
	useScanReport(scanReportOf(momRes("AMDBUSDT", "wedge_pop", true, 90, 300, false), invalid), nil)
	fx2.cycle(t)
	if fx2.ai.calls != 0 || len(fx2.lastRecord(t).CandidateCoins) != 0 {
		t.Fatalf("calls %d candidates %v", fx2.ai.calls, fx2.lastRecord(t).CandidateCoins)
	}
}

func TestStockScanKellRankUnchangedByMomentum(t *testing.T) {
	report := scanReportOf(
		momRes("NVDABUSDT", "wedge_pop", true, 90, 1, true),
		momRes("MSFTBUSDT", "base_n_break", true, 70, 500, true),
		momRes("AMDBUSDT", "uptrend", false, 60, 900, true), // no long setup
	)
	for _, rank := range []string{"", store.StockScanRankKell} {
		fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return waitAll },
			withScanRank(rank, scanMode(2, nil, nil)))
		useScanReport(report, nil)
		fx.cycle(t)
		rec := fx.lastRecord(t)
		if got := strings.Join(rec.CandidateCoins, ","); got != "NVDABUSDT,MSFTBUSDT" {
			t.Fatalf("rank %q candidates %s", rank, got)
		}
		if !strings.Contains(strings.Join(rec.ExecutionLog, "\n"), "scan top 2 rank=kell:") || strings.Contains(fx.ai.system, "12-1 动量") {
			t.Fatalf("rank %q: kell log/prompt wrong:\n%s", rank, strings.Join(rec.ExecutionLog, "\n"))
		}
	}
}

func TestStockScanMomentumRankHybridComposition(t *testing.T) {
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return waitAll },
		withScanRank(store.StockScanRankMomentum, hybridMode(2, []string{"AAPLBUSDT"}, []string{"TSLABUSDT"})))
	useScanReport(scanReportOf(
		momRes("AAPLBUSDT", "wedge_pop", true, 90, 90, true), // pinned: not repeated
		momRes("TSLABUSDT", "wedge_pop", true, 80, 80, true), // excluded
		momRes("NVDABUSDT", "ema_crossback", true, 70, 70, true),
		momRes("MSFTBUSDT", "uptrend", false, 20, 40, true),
		momRes("AMDBUSDT", "uptrend", false, 20, 30, true), // beyond top N
	), nil)
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if !rec.Success || strings.Join(rec.CandidateCoins, ",") != "AAPLBUSDT,NVDABUSDT,MSFTBUSDT" {
		t.Fatalf("hybrid candidates %v err %q", rec.CandidateCoins, rec.ErrorMessage)
	}
	if !strings.Contains(fx.ai.system, "用户手动指定") || !strings.Contains(fx.ai.system, "12-1 动量") {
		t.Fatalf("hybrid momentum system prompt:\n%s", fx.ai.system)
	}

	// A held name outside the ranking stays in the list; the scan fills N more.
	openNVDA := `[{"symbol":"NVDABUSDT","action":"open_long","entry_type":"market","stop_loss":115,"take_profit":130,"confidence":80,"reasoning":"x"}]`
	fx2 := newStockFixtureCfg(t, true, stockTestNow, func(call int) string {
		if call == 1 {
			return openNVDA
		}
		return `[{"symbol":"NVDABUSDT","action":"hold"},{"symbol":"MSFTBUSDT","action":"wait"}]`
	}, withScanRank(store.StockScanRankMomentum, scanMode(1, nil, nil)))
	useScanReport(scanReportOf(momRes("NVDABUSDT", "wedge_pop", true, 90, 70, true)), nil)
	fx2.cycle(t)
	useScanReport(scanReportOf(
		momRes("NVDABUSDT", "downtrend", false, 5, -20, false), // held, dropped below SMA200
		momRes("MSFTBUSDT", "uptrend", false, 20, 40, true),
	), nil)
	fx2.cycle(t)
	rec = fx2.lastRecord(t)
	if !rec.Success || strings.Join(rec.CandidateCoins, ",") != "NVDABUSDT,MSFTBUSDT" {
		t.Fatalf("held + momentum pick: %v err %q", rec.CandidateCoins, rec.ErrorMessage)
	}
}

func TestStockScanModeIgnoresConfiguredSymbols(t *testing.T) {
	// AAPL is configured but the scanner never picks it: scan mode must not trade it.
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string {
		return `[{"symbol":"NVDABUSDT","action":"wait"},{"symbol":"MSFTBUSDT","action":"wait"}]`
	}, scanMode(2, []string{"AAPLBUSDT"}, nil))
	if got := fx.at.stock.cfg.Symbols; len(got) != 0 {
		t.Fatalf("scan mode must not resolve the configured symbols: %v", got)
	}
	useScanReport(scanReportOf(
		scanRes("NVDABUSDT", "wedge_pop", true, 90),
		scanRes("MSFTBUSDT", "base_n_break", true, 70),
		scanRes("AMDBUSDT", "ema_crossback", true, 60), // beyond top N
	), nil)
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if !rec.Success || strings.Join(rec.CandidateCoins, ",") != "NVDABUSDT,MSFTBUSDT" {
		t.Fatalf("record %+v", rec)
	}
	if strings.Contains(fx.ai.user, "AAPLBUSDT") {
		t.Fatalf("scan mode must not trade configured symbols:\n%s", fx.ai.user)
	}
	// The cycle selector must ignore the configured list on its own too.
	list, _ := fx.at.stockCycleSymbols(context.Background(), &store.StockConfig{SymbolSource: store.StockSymbolSourceScan, ScanTopN: 2, Symbols: []string{"AAPLBUSDT"}},
		&stockState{}, func(string, ...interface{}) {})
	if got := strings.Join(list, ","); got != "NVDABUSDT,MSFTBUSDT" {
		t.Fatalf("scan selector with configured symbols handed in: %s", got)
	}
}

func TestStockScanHeldSymbolAlwaysIncluded(t *testing.T) {
	openNVDA := `[{"symbol":"NVDABUSDT","action":"open_long","entry_type":"market","stop_loss":115,"take_profit":130,"confidence":80,"reasoning":"x"}]`
	fx := newStockFixtureCfg(t, true, stockTestNow, func(call int) string {
		if call == 1 {
			return openNVDA
		}
		return `[{"symbol":"NVDABUSDT","action":"hold"},{"symbol":"MSFTBUSDT","action":"wait"}]`
	}, scanMode(1, nil, nil))
	useScanReport(scanReportOf(scanRes("NVDABUSDT", "wedge_pop", true, 90)), nil)
	fx.cycle(t)
	if open, _ := fx.at.store.StockPaper().ListOpen("t1"); len(open) != 1 || open[0].Symbol != "NVDABUSDT" {
		t.Fatalf("NVDA not opened: %+v", open)
	}

	// Cycle 2: NVDA fell out of the ranking and is now excluded; it is held,
	// so it must still be in the list (otherwise hold/close is UNKNOWN_SYMBOL).
	fx.at.stock.cfg.ScanExclude = []string{"NVDABUSDT"}
	useScanReport(scanReportOf(scanRes("MSFTBUSDT", "ema_crossback", true, 70)), nil)
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if !rec.Success || strings.Join(rec.CandidateCoins, ",") != "NVDABUSDT,MSFTBUSDT" {
		t.Fatalf("record %v candidates %v err %q", rec.Success, rec.CandidateCoins, rec.ErrorMessage)
	}
	for _, d := range rec.Decisions {
		if d.Error != "" {
			t.Fatalf("decision rejected: %+v", d)
		}
	}
	if !strings.Contains(fx.ai.user, "NVDABUSDT qty=") {
		t.Fatalf("held position missing from the prompt:\n%s", fx.ai.user)
	}
}

func TestStockScanNoCandidatesSkipsAI(t *testing.T) {
	for name, tc := range map[string]struct {
		report *stockengine.ScanReport
		err    error
		want   string
	}{
		"scan failed": {nil, fmt.Errorf("binance down"), "scan unavailable, no candidates"},
		"no setups":   {scanReportOf(scanRes("METABUSDT", "uptrend", false, 50)), nil, "scan found no long setups, no candidates"},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return waitAll }, scanMode(5, nil, nil))
			useScanReport(tc.report, tc.err)
			fx.cycle(t)
			if fx.ai.calls != 0 {
				t.Fatalf("AI called %d times without candidates", fx.ai.calls)
			}
			rec := fx.lastRecord(t)
			if !rec.Success || len(rec.Decisions) != 0 || !strings.Contains(strings.Join(rec.ExecutionLog, "\n"), tc.want) {
				t.Fatalf("record %+v", rec)
			}
		})
	}
}

func TestStockScanFailureFallsBackToPins(t *testing.T) {
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return holdAAPL }, hybridMode(5, []string{"AAPLBUSDT"}, nil))
	useScanReport(nil, fmt.Errorf("binance down"))
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if fx.ai.calls != 1 || !rec.Success || strings.Join(rec.CandidateCoins, ",") != "AAPLBUSDT" {
		t.Fatalf("calls %d record %+v", fx.ai.calls, rec)
	}
	// No report: the AAPL cycle line is classified from the cycle's own daily series.
	if !strings.Contains(fx.ai.user, "phase=") {
		t.Fatalf("locally classified cycle line missing:\n%s", fx.ai.user)
	}
}

func TestStockHybridNoSetupsStillTradesPins(t *testing.T) {
	// Hybrid keeps its configured symbols even when the scanner finds no setups.
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string {
		return `[{"symbol":"AAPLBUSDT","action":"wait"}]`
	}, hybridMode(5, []string{"AAPLBUSDT"}, nil))
	useScanReport(scanReportOf(scanRes("METABUSDT", "uptrend", false, 50)), nil)
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if fx.ai.calls != 1 || !rec.Success || strings.Join(rec.CandidateCoins, ",") != "AAPLBUSDT" {
		t.Fatalf("calls %d record %+v", fx.ai.calls, rec)
	}
}

func TestStockManualModeUnchangedByScan(t *testing.T) {
	fx := newStockFixture(t, true, stockTestNow, func(int) string { return holdAAPL })
	stockGetScanReport = func(context.Context, time.Duration) (*stockengine.ScanReport, error) {
		t.Fatal("manual mode must not request a scan")
		return nil, nil
	}
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if !rec.Success || strings.Join(rec.CandidateCoins, ",") != "AAPLBUSDT" {
		t.Fatalf("record %+v", rec)
	}
	if !strings.Contains(fx.ai.user, "价格周期(Kell, 日线): phase=") || strings.Contains(fx.ai.system, "周期扫描器") {
		t.Fatalf("manual prompt: cycle line expected, scanner note not:\n%s", fx.ai.user)
	}
}

func TestStockScanPreflightSymbolAndWarm(t *testing.T) {
	fx := newStockFixtureCfg(t, false, stockTestNow, func(int) string { return waitAll }, scanMode(5, nil, nil))
	if fx.spot.preflights != 1 {
		t.Fatalf("scan mode without pins must still preflight once: %d", fx.spot.preflights)
	}
	ctx := context.Background()
	if got := fx.at.stockPreflightSymbol(ctx, []string{"MSFTBUSDT"}); got != "MSFTBUSDT" {
		t.Fatalf("pin first: %q", got)
	}
	if got := fx.at.stockPreflightSymbol(ctx, nil); got != "SPYBUSDT" {
		t.Fatalf("context symbol next: %q", got)
	}
	stockLookupSymbol = func(context.Context, string) (usstock.SymbolInfo, bool) { return usstock.SymbolInfo{}, false }
	stockListUniverse = func(context.Context) ([]usstock.SymbolInfo, error) {
		return []usstock.SymbolInfo{{Symbol: "AMDBUSDT"}, {Symbol: "METABUSDT"}}, nil
	}
	if got := fx.at.stockPreflightSymbol(ctx, nil); got != "AMDBUSDT" {
		t.Fatalf("universe fallback: %q", got)
	}
	stockListUniverse = func(context.Context) ([]usstock.SymbolInfo, error) { return nil, fmt.Errorf("down") }
	if got := fx.at.stockPreflightSymbol(ctx, nil); got != "" {
		t.Fatalf("nothing listed: %q", got)
	}

	// The 60s tick warms the scan in scan mode only.
	var warmAges []time.Duration
	stockWarmScan = func(age time.Duration) { warmAges = append(warmAges, age) }
	fx.at.runStockTickSafe(ctx)
	if len(warmAges) != 1 || warmAges[0] != stockScanWarmAge {
		t.Fatalf("tick warm calls %v", warmAges)
	}
	manual := newStockFixture(t, true, stockTestNow, func(int) string { return holdAAPL })
	warmAges = nil
	stockWarmScan = func(age time.Duration) { warmAges = append(warmAges, age) }
	manual.at.runStockTickSafe(ctx)
	if len(warmAges) != 0 {
		t.Fatalf("manual mode must not warm the scan: %v", warmAges)
	}
}

func TestStockResolveSymbolsBySource(t *testing.T) {
	fx := newStockFixture(t, true, stockTestNow, func(int) string { return holdAAPL })
	ctx := context.Background()
	for name, tc := range map[string]struct {
		sc      store.StockConfig
		want    []string
		wantErr bool
	}{
		"manual keeps listed pairs":    {store.StockConfig{Symbols: []string{"AAPLBUSDT", "NOPEBUSDT"}}, []string{"AAPLBUSDT"}, false},
		"manual needs a listed pair":   {store.StockConfig{Symbols: []string{"NOPEBUSDT"}}, nil, true},
		"hybrid keeps listed pairs":    {store.StockConfig{SymbolSource: store.StockSymbolSourceHybrid, Symbols: []string{"AAPLBUSDT", "NOPEBUSDT"}}, []string{"AAPLBUSDT"}, false},
		"hybrid may resolve to none":   {store.StockConfig{SymbolSource: store.StockSymbolSourceHybrid, Symbols: []string{"NOPEBUSDT"}}, nil, false},
		"scan ignores configured list": {store.StockConfig{SymbolSource: store.StockSymbolSourceScan, Symbols: []string{"AAPLBUSDT"}}, nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := fx.at.resolveStockSymbols(ctx, &tc.sc)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("symbols = %v, want %v", got, tc.want)
			}
		})
	}
}

// The model sometimes answers the SPY/QQQ market-context blocks with wait:
// that is dropped, not recorded as a rejected decision. A non-noop action on
// a context pair stays a visible UNKNOWN_SYMBOL rejection.
func TestStockContextWaitIsDroppedNotRejected(t *testing.T) {
	resp := `[{"symbol":"AAPLBUSDT","action":"wait","confidence":60,"reasoning":"no setup"},
{"symbol":"SPYBUSDT","action":"wait","confidence":75,"reasoning":"market weakening"},
{"symbol":"QQQBUSDT","action":"wait","confidence":78,"reasoning":"tape deteriorating"}]`
	fx := newStockFixture(t, true, stockTestNow, func(int) string { return resp })
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if len(rec.Decisions) != 1 || rec.Decisions[0].Symbol != "AAPLBUSDT" || rec.Decisions[0].Error != "" {
		t.Fatalf("context waits must be dropped: %+v", rec.Decisions)
	}
	if !strings.Contains(fx.ai.user, "仅供参考") && !strings.Contains(fx.ai.user, "reference only") {
		t.Fatalf("market-context header must say reference only:\n%s", fx.ai.user)
	}

	resp = `[{"symbol":"AAPLBUSDT","action":"wait","confidence":60,"reasoning":"no setup"},
{"symbol":"SPYBUSDT","action":"open_long","entry_type":"market","stop_loss":115,"confidence":75,"reasoning":"x"}]`
	fx2 := newStockFixture(t, true, stockTestNow, func(int) string { return resp })
	fx2.cycle(t)
	rec = fx2.lastRecord(t)
	found := false
	for _, d := range rec.Decisions {
		if d.Symbol == "SPYBUSDT" && strings.Contains(d.Error, "UNKNOWN_SYMBOL") {
			found = true
		}
	}
	if !found {
		t.Fatalf("open_long on a context pair must stay a rejection: %+v", rec.Decisions)
	}
}

// uniRes is a scan result carrying a universe-table row.
func uniRes(sym string, price float64) stockengine.ScanResult {
	r := momRes(sym, "uptrend", false, 10, 25, true)
	r.Universe = stockengine.UniverseRow{Symbol: sym, Underlying: r.Underlying, Class: "stock", Sector: "Technology", Price: price,
		Ret1dPct: 1.2, Ret10dPct: 3.4, Ret30dPct: 5.6, Vol30Pct: 30, RSI14: 55, DistFrom30dHighPct: -2, AvgDollarVol20: 12.3e6, DaysToEarnings: 9}
	return r
}

func TestStockPromptCarriesCandlesRSIAndUniverse(t *testing.T) {
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return `[{"symbol":"AAPLBUSDT","action":"wait"}]` }, scanMode(1, nil, []string{"TSLABUSDT"}))
	var fetched = map[string]int{}
	base := stockGetSeries
	stockGetSeries = func(ctx context.Context, sym, tf string, need int, yahoo bool) (*usstock.Series, error) {
		if sym == "AAPLBUSDT" {
			fetched[tf]++
		}
		return base(ctx, sym, tf, need, yahoo)
	}
	aapl := uniRes("AAPLBUSDT", 120)
	aapl.Cycle.LongSetup, aapl.Cycle.Score = true, 90
	useScanReport(scanReportOf(aapl, uniRes("TSLABUSDT", 250), uniRes("MSFTBUSDT", 410)), nil)
	fx.cycle(t)

	if !fx.lastRecord(t).Success {
		t.Fatalf("cycle failed: %s", fx.lastRecord(t).ErrorMessage)
	}
	for _, tf := range []string{usstock.TF1d, usstock.TF1w, usstock.TF4h} {
		if fetched[tf] == 0 {
			t.Fatalf("timeframe %s was not fetched: %v", tf, fetched)
		}
	}
	user := fx.ai.user
	for _, want := range []string{"rsi14: 4h=", "K线 1w", "K线 1d", "K线 4h", "全市场概览",
		"AAPLBUSDT|stock|Technology|Y|120|1.2|3.4|5.6|30.0|55.0|-2.0|$12.3M|9|25.0|Y|uptrend",
		"MSFTBUSDT|stock|Technology||410|", "TSLABUSDT|stock|Technology|x|250|"} {
		if !strings.Contains(user, want) {
			t.Fatalf("user prompt missing %q:\n%s", want, user)
		}
	}
	// SPY / QQQ market context carry candles too.
	if n := strings.Count(user, "K线 1d"); n < 2 {
		t.Fatalf("want candles for the candidate and SPY, got %d", n)
	}
}

func TestStockStaleScanReportHasNoUniverseTable(t *testing.T) {
	fx := newStockFixtureCfg(t, true, stockTestNow, func(int) string { return `[{"symbol":"AAPLBUSDT","action":"wait"}]` }, hybridMode(1, []string{"AAPLBUSDT"}, nil))
	report := scanReportOf(uniRes("AAPLBUSDT", 120), uniRes("MSFTBUSDT", 410))
	report.GeneratedAt = stockTestNow.Add(-stockScanMaxAge - time.Minute)
	useScanReport(report, nil)
	fx.cycle(t)
	if strings.Contains(fx.ai.user, "全市场概览") || strings.Contains(fx.ai.user, "MSFTBUSDT|") {
		t.Fatalf("stale report must not produce a universe table:\n%s", fx.ai.user)
	}
}
