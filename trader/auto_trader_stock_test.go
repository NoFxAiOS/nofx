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
	mu           sync.Mutex
	usdt         float64
	hold         map[string]float64
	price        float64
	prot         map[string]*types.ProtectionOrders
	buys         []float64
	limitBuys    int
	sells        []sellCall
	zeroSells    int
	setProt      []protCall
	cancelAll    int
	cancelProt   int
	cancelled    []string
	preflights   int
	seq          int
	statuses     map[string]map[string]interface{}
	statusErr    error
	protErr      error
	protErrors   map[string]error
	statusErrors map[string]error
	balanceErr   error
	positionsErr error
	onCancel     func()
	onPositions  func()
	setProtErr   error   // SetProtection fails with it while set
	sellRatio    float64 // >0: a market sell executes only this share of the quantity
}

func newFakeSpot(price float64) *fakeSpot {
	return &fakeSpot{usdt: 10000, hold: map[string]float64{}, price: price, prot: map[string]*types.ProtectionOrders{}, statuses: map[string]map[string]interface{}{}}
}

func (f *fakeSpot) GetBalance() (map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.balanceErr != nil {
		return nil, f.balanceErr
	}
	eq := f.usdt
	for _, q := range f.hold {
		eq += q * f.price
	}
	return map[string]interface{}{"totalEquity": eq, "availableBalance": f.usdt, "totalWalletBalance": eq, "totalUnrealizedProfit": 0.0}, nil
}
func (f *fakeSpot) GetPositions() ([]map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.positionsErr != nil {
		return nil, f.positionsErr
	}
	if f.onPositions != nil {
		hook := f.onPositions
		f.onPositions = nil
		hook()
	}
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
	if f.sellRatio > 0 {
		qty = floorStep(qty*f.sellRatio, 0.001)
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
	if f.setProtErr != nil {
		return nil, f.setProtErr
	}
	if stop >= f.price {
		return nil, fmt.Errorf("invalid protection")
	}
	if qty > f.hold[sym]+1e-9 {
		return nil, fmt.Errorf("protection quantity exceeds held balance")
	}
	f.cancelProtection(sym)
	p := &types.ProtectionOrders{Symbol: sym, Quantity: qty, StopPrice: stop, StopLimitPrice: limit, TakeProfit: tp, StopOrderID: f.nextID()}
	p.StopClientID = "nxbs_sl_" + p.StopOrderID
	if tp > 0 {
		p.OrderListID, p.TakeProfitID = f.nextID(), f.nextID()
		p.ListClientID, p.TPClientID = "nxbs_list_"+p.OrderListID, "nxbs_tp_"+p.TakeProfitID
	}
	f.prot[sym] = p
	for id, client := range map[string]string{p.StopOrderID: p.StopClientID, p.TakeProfitID: p.TPClientID} {
		if id != "" {
			f.statuses[id] = map[string]interface{}{"status": "NEW", "executedQty": 0.0, "avgPrice": 0.0, "commission": 0.0, "clientOrderId": client}
		}
	}
	return f.prot[sym], nil
}
func (f *fakeSpot) GetProtection(sym string) (*types.ProtectionOrders, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.protErrors[sym]; err != nil {
		return nil, err
	}
	if f.protErr != nil {
		return nil, f.protErr
	}
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
	if f.onCancel != nil {
		hook := f.onCancel
		f.onCancel = nil
		hook()
	}
	f.cancelProtection(sym)
	return nil
}
func (f *fakeSpot) cancelProtection(sym string) {
	if p := f.prot[sym]; p != nil {
		for _, id := range []string{p.StopOrderID, p.TakeProfitID} {
			if s := f.statuses[id]; s != nil && !stockOrderTerminal(fmt.Sprint(s["status"])) {
				s["status"] = "CANCELED"
			}
		}
	}
	delete(f.prot, sym)
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
	if s := f.statuses[orderID]; s != nil && !stockOrderTerminal(fmt.Sprint(s["status"])) {
		s["status"] = "CANCELED"
	}
	return nil
}
func (f *fakeSpot) GetOrderStatus(sym string, id string) (map[string]interface{}, error) {
	if err := f.statusErrors[sym]; err != nil {
		return nil, err
	}
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	if s := f.statuses[id]; s != nil {
		return s, nil
	}
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
[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"market","stop_loss":115,"take_profit":130,"confidence":80,"thesis":"trend continuation over weeks","invalidation":"daily close below 115","reasoning":"trend up"}]`
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
	oldUnderlying := stockGetUnderlyingSeries
	t.Cleanup(func() { stockGetUnderlyingSeries = oldUnderlying })
	// Yahoo-only 4h path delegates to the (faked) series seam so tests stay offline.
	stockGetUnderlyingSeries = func(ctx context.Context, sym, tf string, need int) (*usstock.Series, error) {
		s, err := stockGetSeries(ctx, sym, tf, need, true)
		if s != nil {
			c := *s
			c.Source = usstock.SourceYahoo
			s = &c
		}
		return s, err
	}
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
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 16.666, 115, 0, 120); err != nil {
		t.Fatal(err)
	}
	fx.spot.cancelProtection("AAPLBUSDT") // verified cancellation with no fills
	fx.spot.setProt = nil

	fx.at.stockProtectionTick(ctx) // missing protection → re-placed from its durable prices
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
	limit := `[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"limit","limit_price":119,"stop_loss":115,"take_profit":130,"confidence":80,"thesis":"pullback holds","invalidation":"close below 115","reasoning":"pullback"}]`
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return limit })
	fx.cycle(t)
	if fx.spot.limitBuys != 1 || len(fx.spot.setProt) != 0 {
		t.Fatalf("limit buys %d, protection %v", fx.spot.limitBuys, fx.spot.setProt)
	}
	pend, _ := fx.at.store.StockPaper().ListPending("t1")
	if len(pend) != 1 || pend[0].Symbol != "AAPLBUSDT" || pend[0].Stop != 115 {
		t.Fatalf("pending not persisted: %+v", pend)
	}
	if th, _ := fx.at.store.StockThesis().List("t1"); th["AAPLBUSDT"].Thesis != "pullback holds" || th["AAPLBUSDT"].Invalidation != "close below 115" {
		t.Fatalf("a resting limit entry must persist its thesis: %+v", th)
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

func TestStockExchangeSideStopFillIsBookedAtExecutionPrice(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 16.666, 16.666)
	if err := fx.at.stockPlaceProtection(context.Background(), "AAPLBUSDT", 16.666, 115, 0, 120); err != nil {
		t.Fatal(err)
	}
	id := fx.spot.prot["AAPLBUSDT"].StopOrderID
	fx.spot.statuses[id]["status"], fx.spot.statuses[id]["executedQty"], fx.spot.statuses[id]["avgPrice"] = "FILLED", 16.666, 114.5
	fx.spot.hold["AAPLBUSDT"] = 0 // the exchange stop filled
	delete(fx.spot.prot, "AAPLBUSDT")
	fx.price, fx.spot.price = 114.5, 114.5
	fx.at.stockProtectionTick(context.Background())
	closed, _ := fx.at.store.Position().GetClosedPositions("t1", 5)
	if len(closed) != 1 || closed[0].CloseReason != "stop_loss" || !near(closed[0].ExitPrice, 114.5) || closed[0].RealizedPnL >= 0 {
		t.Fatalf("exchange stop fill not booked: %+v", closed)
	}
}

// The balance after an exchange exit may consist entirely of manual shares.
// Only the recorded order executions decide how much program ownership exits.
func TestStockProtectionFillsLeaveManualSharesUntouched(t *testing.T) {
	for _, manual := range []float64{7, 2} {
		for _, role := range []string{"stop_loss", "take_profit"} {
			for _, cycle := range []bool{false, true} {
				t.Run(fmt.Sprintf("manual_%g/%s/cycle_%v", manual, role, cycle), func(t *testing.T) {
					fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
					fx.openLiveOwned(t, 5, 5+manual)
					ctx := context.Background()
					if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
						t.Fatal(err)
					}
					p := fx.spot.prot["AAPLBUSDT"]
					id, px := p.StopOrderID, 113.25
					if role == "take_profit" {
						id, px = p.TakeProfitID, 131.25
					}
					fx.spot.statuses[id]["status"], fx.spot.statuses[id]["executedQty"], fx.spot.statuses[id]["avgPrice"] = "FILLED", 5.0, px
					// The OCO sibling is canceled by the exchange.
					fx.spot.cancelProtection("AAPLBUSDT")
					fx.spot.hold["AAPLBUSDT"] = manual
					// Restart loses in-memory protection prices, never durable IDs.
					fx.at.stock.lastProtect = map[string]stockProtect{}
					fx.price, fx.spot.price = px, px
					if cycle {
						fx.cycle(t)
					} else {
						fx.at.stockProtectionTick(ctx)
					}
					fx.at.stockProtectionTick(ctx) // no duplicate booking/re-protection
					open, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
					closed, _ := fx.at.store.Position().GetClosedPositions("t1", 5)
					if len(open) != 0 || len(closed) != 1 || closed[0].CloseReason != role || closed[0].ExitOrderID != id || !near(closed[0].ExitPrice, px) || !near(closed[0].RealizedPnL, 5*(px-120)) {
						t.Fatalf("open=%+v closed=%+v", open, closed)
					}
					if len(fx.spot.sells) != 0 || len(fx.spot.setProt) != 1 || fx.spot.hold["AAPLBUSDT"] != manual {
						t.Fatalf("manual shares touched: sells=%v protection=%v balance=%g", fx.spot.sells, fx.spot.setProt, fx.spot.hold["AAPLBUSDT"])
					}
					for _, a := range fx.alert {
						if strings.Contains(a, "外部减仓") {
							t.Fatalf("protective fill misclassified as external: %s", a)
						}
					}
				})
			}
		}
	}
}

func TestStockPartialProtectionFillReplacesOnlyRemainder(t *testing.T) {
	for _, cycle := range []bool{false, true} {
		t.Run(fmt.Sprint(cycle), func(t *testing.T) {
			fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
			fx.openLiveOwned(t, 5, 12)
			ctx := context.Background()
			if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
				t.Fatal(err)
			}
			p := fx.spot.prot["AAPLBUSDT"]
			fx.spot.statuses[p.StopOrderID]["status"], fx.spot.statuses[p.StopOrderID]["executedQty"], fx.spot.statuses[p.StopOrderID]["avgPrice"] = "PARTIALLY_FILLED", 3.0, 114.0
			p.Quantity, fx.spot.hold["AAPLBUSDT"] = 2, 9
			if cycle {
				fx.cycle(t)
			} else {
				fx.at.stockProtectionTick(ctx)
			}
			fx.at.stock.lastProtect = map[string]stockProtect{}
			fx.at.stockProtectionTick(ctx)
			rows, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
			if len(rows) != 1 || !near(rows[0].Quantity, 2) || !near(rows[0].RealizedPnL, -18) || !near(rows[0].ExitPrice, 114) {
				t.Fatalf("partial fill %+v", rows)
			}
			if len(fx.spot.setProt) != 2 || fx.spot.setProt[1].Qty != 2 || len(fx.spot.sells) != 0 || fx.spot.hold["AAPLBUSDT"] != 9 {
				t.Fatalf("sells=%v protection=%v balance=%g", fx.spot.sells, fx.spot.setProt, fx.spot.hold["AAPLBUSDT"])
			}
		})
	}
}

func TestStockProtectionQueryFailureBlocksFallbackAndBalanceShrink(t *testing.T) {
	for _, openQuery := range []bool{false, true} {
		t.Run(fmt.Sprint(openQuery), func(t *testing.T) {
			fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
			fx.openLiveOwned(t, 5, 12)
			ctx := context.Background()
			if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
				t.Fatal(err)
			}
			fx.spot.cancelProtection("AAPLBUSDT")
			fx.spot.hold["AAPLBUSDT"] = 2
			fx.price, fx.spot.price = 110, 110
			if openQuery {
				fx.spot.protErr = fmt.Errorf("open orders unavailable")
			} else {
				fx.spot.statusErr = fmt.Errorf("order status unavailable")
			}
			alerts := len(fx.alert)
			fx.at.stockProtectionTick(ctx)
			rows, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
			if len(fx.spot.sells) != 0 || len(fx.spot.setProt) != 1 || len(rows) != 1 || rows[0].Quantity != 5 || len(fx.alert) <= alerts {
				t.Fatalf("unsafe query failure: sells=%v protection=%v rows=%+v alerts=%v", fx.spot.sells, fx.spot.setProt, rows, fx.alert)
			}
		})
	}
}

func TestStockVerifiedMissingProtectionAllowsBoundedFallback(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 5, 12)
	ctx := context.Background()
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	fx.spot.cancelProtection("AAPLBUSDT") // genuine cancellation, no fills
	fx.price, fx.spot.price = 110, 110
	fx.at.stockProtectionTick(ctx)
	if len(fx.spot.sells) != 1 || fx.spot.sells[0].Qty != 5 || fx.spot.hold["AAPLBUSDT"] != 7 {
		t.Fatalf("bounded fallback: sells=%v balance=%g", fx.spot.sells, fx.spot.hold["AAPLBUSDT"])
	}
}

func TestStockProtectionFillRacingCancelCannotSellManualShares(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
			fx.openLiveOwned(t, 5, 12)
			ctx := context.Background()
			if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
				t.Fatal(err)
			}
			st, err := fx.at.stockGatherLive(ctx, stockTestNow, true, true)
			if err != nil {
				t.Fatal(err)
			}
			id := fx.spot.prot["AAPLBUSDT"].StopOrderID
			fx.spot.onCancel = func() {
				fx.spot.statuses[id]["status"], fx.spot.statuses[id]["executedQty"], fx.spot.statuses[id]["avgPrice"] = "FILLED", 5.0, 114.0
				fx.spot.hold["AAPLBUSDT"] = 7
			}
			if replace {
				_ = fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 116, 130, 120)
			} else if _, err := fx.at.stockSellLive(ctx, st.Holdings["AAPLBUSDT"], 5, true, "ai_close", stockTestNow); err != nil {
				t.Fatal(err)
			}
			rows, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
			if len(rows) != 0 || len(fx.spot.sells) != 0 || len(fx.spot.setProt) != 1 || fx.spot.hold["AAPLBUSDT"] != 7 {
				t.Fatalf("cancel race: rows=%+v sells=%v protection=%v balance=%g", rows, fx.spot.sells, fx.spot.setProt, fx.spot.hold["AAPLBUSDT"])
			}
		})
	}
}

func TestStockUnexplainedFlatBalanceDoesNotGuessStop(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 5, 5)
	ctx := context.Background()
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	fx.spot.cancelProtection("AAPLBUSDT")
	fx.spot.hold["AAPLBUSDT"] = 0
	fx.price, fx.spot.price = 114, 114
	fx.at.stockProtectionTick(ctx)
	fx.at.stockProtectionTick(ctx)
	closed, _ := fx.at.store.Position().GetClosedPositions("t1", 5)
	if len(closed) != 1 || closed[0].CloseReason != "external" || closed[0].RealizedPnL != 0 {
		t.Fatalf("unexplained reduction must not guess an exit: %+v", closed)
	}
	count := 0
	for _, a := range fx.alert {
		if strings.HasPrefix(a, "ALERT|") && strings.Contains(a, "外部减仓") {
			count++
		}
	}
	if count != 1 || len(fx.spot.sells) != 0 {
		t.Fatalf("external alerts=%d sells=%v", count, fx.spot.sells)
	}
}

func TestStockProtectionFillRacingBalanceReadIsNotExternalReduction(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 5, 7)
	ctx := context.Background()
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	id := fx.spot.prot["AAPLBUSDT"].StopOrderID
	fx.spot.onPositions = func() {
		fx.spot.statuses[id]["status"], fx.spot.statuses[id]["executedQty"], fx.spot.statuses[id]["avgPrice"] = "FILLED", 5.0, 114.0
		fx.spot.hold["AAPLBUSDT"] = 2
	}
	fx.at.stockProtectionTick(ctx)
	rows, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
	closed, _ := fx.at.store.Position().GetClosedPositions("t1", 5)
	if len(rows) != 0 || len(closed) != 1 || closed[0].CloseReason != "stop_loss" || closed[0].RealizedPnL != -30 || len(fx.spot.sells) != 0 {
		t.Fatalf("balance race: open=%+v closed=%+v sells=%v", rows, closed, fx.spot.sells)
	}
	for _, a := range fx.alert {
		if strings.Contains(a, "外部减仓") {
			t.Fatalf("protective fill mislabeled: %s", a)
		}
	}
}

func TestStockRestartAfterFillBookingRetiresOCOAndProtectsRemainder(t *testing.T) {
	for _, filled := range []float64{3, 5} {
		t.Run(fmt.Sprint(filled), func(t *testing.T) {
			fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
			fx.openLiveOwned(t, 5, 12)
			ctx := context.Background()
			if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
				t.Fatal(err)
			}
			p := fx.spot.prot["AAPLBUSDT"]
			s := "PARTIALLY_FILLED"
			if filled == 5 {
				s = "FILLED"
			}
			fx.spot.statuses[p.StopOrderID]["status"], fx.spot.statuses[p.StopOrderID]["executedQty"], fx.spot.statuses[p.StopOrderID]["avgPrice"] = s, filled, 114.0
			p.Quantity, fx.spot.hold["AAPLBUSDT"] = 5-filled, 12-filled
			if _, err := fx.at.stockCheckProtection("AAPLBUSDT", stockTestNow); err != nil {
				t.Fatal(err)
			}
			// Crash after the atomic book, before canceling the other OCO leg.
			fx.at.stock.lastProtect = map[string]stockProtect{}
			fx.at.stockProtectionTick(ctx)
			rows, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
			if filled == 3 {
				if len(rows) != 1 || rows[0].Quantity != 2 || rows[0].RealizedPnL != -18 || fx.spot.prot["AAPLBUSDT"].Quantity != 2 {
					t.Fatalf("restart partial: %+v protection=%+v", rows, fx.spot.prot["AAPLBUSDT"])
				}
			} else if len(rows) != 0 || fx.spot.prot["AAPLBUSDT"] != nil || len(fx.spot.setProt) != 1 {
				t.Fatalf("restart full: %+v protection=%v", rows, fx.spot.setProt)
			}
			if len(fx.spot.sells) != 0 || fx.spot.hold["AAPLBUSDT"] != 12-filled {
				t.Fatalf("restart touched manual shares: sells=%v balance=%g", fx.spot.sells, fx.spot.hold["AAPLBUSDT"])
			}
		})
	}
}

func TestStockProtectiveExitPrecedesPendingBuyCostAndBalanceReconciliation(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 5, 7)
	ctx := context.Background()
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	id := fx.spot.prot["AAPLBUSDT"].StopOrderID
	fx.spot.statuses[id]["status"], fx.spot.statuses[id]["executedQty"], fx.spot.statuses[id]["avgPrice"] = "FILLED", 5.0, 114.0
	fx.spot.cancelProtection("AAPLBUSDT")
	// During the same interval, a resting entry buys 2 new PROGRAM shares.
	fx.spot.hold["AAPLBUSDT"] = 4 // 2 manual + 2 newly bought
	p := &stockPending{Symbol: "AAPLBUSDT", OrderID: "pending", Qty: 2, Stop: 90, TakeProfit: 130, PlacedAt: stockTestNow}
	fx.at.stock.pendings[p.Symbol] = p
	fx.at.savePending(p)
	fx.spot.statuses[p.OrderID] = map[string]interface{}{"status": "FILLED", "executedQty": 2.0, "avgPrice": 100.0, "commission": 0.0}
	fx.at.stockProtectionTick(ctx)
	rows, _ := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
	closed, _ := fx.at.store.Position().GetClosedPositions("t1", 5)
	if len(rows) != 1 || rows[0].Quantity != 2 || rows[0].EntryPrice != 100 || len(closed) != 1 || closed[0].RealizedPnL != -30 || closed[0].CloseReason != "stop_loss" {
		t.Fatalf("exit must precede the new cost basis: open=%+v closed=%+v", rows, closed)
	}
	if len(fx.spot.sells) != 0 || fx.spot.prot["AAPLBUSDT"].Quantity != 2 || fx.spot.hold["AAPLBUSDT"] != 4 {
		t.Fatalf("pending reconciliation touched manual shares: sells=%v protection=%+v", fx.spot.sells, fx.spot.prot["AAPLBUSDT"])
	}
}

func TestStockMissingUntrackedProtectionCannotManagePossibleManualShares(t *testing.T) {
	for _, price := range []float64{110, 120} {
		t.Run(fmt.Sprint(price), func(t *testing.T) {
			fx := newStockFixture(t, false, stockTestNow, func(int) string { return closeAAPL })
			// An upgrade cannot recover an already-filled legacy protection
			// from openOrders. These recorded 5 could be a phantom position.
			fx.openLiveOwned(t, 5, 7)
			fx.price, fx.spot.price = price, price
			fx.at.stockProtectionTick(context.Background())
			fx.cycle(t)
			rec := fx.lastRecord(t)
			if len(rec.Decisions) != 1 || rec.Decisions[0].Success || !strings.Contains(rec.Decisions[0].Error, "PROTECTION_UNVERIFIED") {
				t.Fatalf("untracked protection must quarantine the holding: %+v", rec.Decisions)
			}
			if len(fx.spot.sells) != 0 || len(fx.spot.setProt) != 0 || fx.ai.calls != 1 || len(fx.alert) == 0 {
				t.Fatalf("unverified shares managed: sells=%v protection=%v AI=%d alerts=%v", fx.spot.sells, fx.spot.setProt, fx.ai.calls, fx.alert)
			}
		})
	}
}

// A problem on A must leave B's protection tick and decisions fully usable.
func TestStockProtectionQuarantineIsolatesSymbols(t *testing.T) {
	for _, failure := range []string{"get_protection", "check_protection", "save_protection", "missing_records", "missing_open_orders"} {
		t.Run(failure, func(t *testing.T) {
			fx := newStockFixtureCfg(t, false, stockTestNow, func(call int) string {
				if call == 1 {
					return `[{"symbol":"AAPLBUSDT","action":"close_long"},{"symbol":"MSFTBUSDT","action":"close_long"}]`
				}
				if call == 2 {
					return holdAAPL
				}
				return `[{"symbol":"AAPLBUSDT","action":"wait"}]`
			}, func(sc *store.StockConfig) { sc.Symbols = []string{"AAPLBUSDT", "MSFTBUSDT"} })
			ctx := context.Background()
			fx.openLiveOwned(t, 5, 12)
			if _, err := fx.at.stockRecordBuy("MSFTBUSDT", 4, 120, 0, "seedB", 115, stockTestNow); err != nil {
				t.Fatal(err)
			}
			fx.spot.hold["MSFTBUSDT"] = 8
			for _, sym := range []string{"AAPLBUSDT", "MSFTBUSDT"} {
				if sym == "AAPLBUSDT" && failure == "missing_records" {
					continue
				}
				if err := fx.at.stockPlaceProtection(ctx, sym, 4, 115, 130, 120); err != nil {
					t.Fatal(err)
				}
			}
			// B has genuinely canceled, unfilled protection, so its tick must
			// repair it even though A is quarantined in the very same pass.
			fx.spot.cancelProtection("MSFTBUSDT")
			fx.spot.setProt = nil
			switch failure {
			case "get_protection":
				fx.spot.protErrors = map[string]error{"AAPLBUSDT": fmt.Errorf("A open orders unavailable")}
			case "check_protection":
				fx.spot.statusErrors = map[string]error{"AAPLBUSDT": fmt.Errorf("A status unavailable")}
			case "save_protection":
				if err := fx.at.store.GormDB().Exec(`CREATE TRIGGER fail_a_protection BEFORE INSERT ON stock_protection_orders
					WHEN NEW.symbol = 'AAPLBUSDT' BEGIN SELECT RAISE(FAIL, 'A protection save failed'); END`).Error; err != nil {
					t.Fatal(err)
				}
			case "missing_open_orders":
				delete(fx.spot.prot, "AAPLBUSDT") // recorded legs still report NEW
			}
			st, err := fx.at.stockGatherLive(ctx, stockTestNow, true, true)
			if err != nil {
				t.Fatal(err)
			}
			a, b := st.Holdings["AAPLBUSDT"], st.Holdings["MSFTBUSDT"]
			if a == nil || !a.Quarantined || a.QuarantineReason == "" || a.ProtectionVerified || b == nil || b.Quarantined {
				t.Fatalf("A=%+v B=%+v", a, b)
			}
			fx.at.stockProtectionTick(ctx)
			if len(fx.spot.sells) != 0 || len(fx.spot.setProt) != 1 || fx.spot.setProt[0].Symbol != "MSFTBUSDT" {
				t.Fatalf("A affected B's protection tick: sells=%v protection=%v", fx.spot.sells, fx.spot.setProt)
			}
			fx.cycle(t)
			rec := fx.lastRecord(t)
			if !rec.Success || len(rec.Decisions) != 2 || rec.Decisions[0].Success || !strings.Contains(rec.Decisions[0].Error, "PROTECTION_UNVERIFIED") || !rec.Decisions[1].Success {
				t.Fatalf("cycle did not isolate A: %+v", rec)
			}
			positionBlock := strings.Split(fx.ai.user, "大盘背景")[0]
			if !strings.Contains(positionBlock, "AAPLBUSDT qty=5.000000") || !strings.Contains(positionBlock, "PROTECTION UNVERIFIED — only hold/wait allowed:") || !strings.Contains(positionBlock, a.QuarantineReason) {
				t.Fatalf("quarantine absent from position block: %s", positionBlock)
			}
			if len(fx.spot.sells) != 1 || fx.spot.sells[0].Symbol != "MSFTBUSDT" || fx.spot.sells[0].Qty != 4 || fx.spot.hold["AAPLBUSDT"] != 12 {
				t.Fatalf("wrong symbol sold: %v balances=%v", fx.spot.sells, fx.spot.hold)
			}
			for _, action := range []string{"hold", "wait"} {
				fx.cycle(t)
				rec := fx.lastRecord(t)
				if len(rec.Decisions) != 1 || rec.Decisions[0].Action != action || !rec.Decisions[0].Success {
					t.Fatalf("%s on quarantined A rejected: %+v", action, rec.Decisions)
				}
			}
			rows, err := fx.at.store.Position().StockOwnedRows("t1", "AAPLBUSDT")
			if err != nil || len(rows) != 1 || rows[0].Quantity != 5 {
				t.Fatalf("quarantined ownership changed: rows=%+v err=%v", rows, err)
			}
		})
	}
}

func TestStockQuarantineRejectsEveryTradingAction(t *testing.T) {
	action := ""
	fx := newStockFixture(t, false, stockTestNow, func(int) string {
		return fmt.Sprintf(`[{"symbol":"AAPLBUSDT","action":%q,"stop_loss":115,"quantity_pct":50,"confidence":85,"thesis":"trend","invalidation":"close below 115"}]`, action)
	})
	fx.openLiveOwned(t, 5, 12) // no live or recorded protection: possible manual inventory
	for _, action = range []string{"open_long", "add_long", "reduce_long", "close_long", "adjust_stop"} {
		fx.cycle(t)
		rec := fx.lastRecord(t)
		if len(rec.Decisions) != 1 || rec.Decisions[0].Success || rec.Decisions[0].Error != "rejected: PROTECTION_UNVERIFIED" {
			t.Fatalf("%s was not rejected by the cycle: %+v", action, rec.Decisions)
		}
	}
	if len(fx.spot.buys)+len(fx.spot.sells)+len(fx.spot.setProt) != 0 {
		t.Fatalf("quarantined inventory traded: buys=%v sells=%v protection=%v", fx.spot.buys, fx.spot.sells, fx.spot.setProt)
	}
}

func TestStockPendingProtectionFailureOnlyQuarantinesItsSymbol(t *testing.T) {
	fx := newStockFixtureCfg(t, false, stockTestNow, func(int) string { return holdAAPL },
		func(sc *store.StockConfig) { sc.Symbols = []string{"AAPLBUSDT", "MSFTBUSDT"} })
	ctx := context.Background()
	fx.openLiveOwned(t, 5, 12)
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	fx.spot.statusErrors = map[string]error{"AAPLBUSDT": fmt.Errorf("A status unavailable")}
	fx.spot.hold["MSFTBUSDT"] = 2
	p := &stockPending{Symbol: "MSFTBUSDT", OrderID: "pendingB", Qty: 2, Stop: 90, TakeProfit: 130, PlacedAt: stockTestNow}
	fx.at.stock.pendings[p.Symbol] = p
	fx.at.savePending(p)
	fx.spot.statuses[p.OrderID] = map[string]interface{}{"status": "FILLED", "executedQty": 2.0, "avgPrice": 100.0}
	fx.spot.setProt = nil
	fx.at.stockProtectionTick(ctx)
	rows, err := fx.at.store.Position().StockOwnedRows("t1", "MSFTBUSDT")
	if err != nil || len(rows) != 1 || rows[0].Quantity != 2 || fx.at.stock.pendings[p.Symbol] != nil {
		t.Fatalf("B's pending entry was blocked: rows=%+v pending=%+v err=%v", rows, fx.at.stock.pendings[p.Symbol], err)
	}
	if len(fx.spot.setProt) != 1 || fx.spot.setProt[0].Symbol != "MSFTBUSDT" || fx.spot.setProt[0].Qty != 2 || len(fx.spot.sells) != 0 {
		t.Fatalf("B's filled entry unprotected: protection=%v sells=%v", fx.spot.setProt, fx.spot.sells)
	}
	if stockNoticeCount(fx, "ALERT", "AAPLBUSDT") != 1 {
		t.Fatalf("pending/gather duplicated the same alert: %v", fx.alert)
	}
}

func stockNoticeCount(fx *stockFixture, kind, symbol string) int {
	n := 0
	for _, message := range fx.alert {
		if strings.HasPrefix(message, kind+"|") && strings.Contains(message, symbol) {
			n++
		}
	}
	return n
}

func TestStockProtectionAlertDedupAndRecovery(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	ctx := context.Background()
	fx.openLiveOwned(t, 5, 12)
	if err := fx.at.stockPlaceProtection(ctx, "AAPLBUSDT", 5, 115, 130, 120); err != nil {
		t.Fatal(err)
	}
	now := stockTestNow
	fx.at.stockNow = func() time.Time { return now }
	fx.spot.protErrors = map[string]error{"AAPLBUSDT": fmt.Errorf("open orders unavailable, request 1")}
	for i := 0; i < 60; i++ {
		now = stockTestNow.Add(time.Duration(i) * time.Minute)
		// Request-specific details must not defeat reason-class de-dup.
		fx.spot.protErrors["AAPLBUSDT"] = fmt.Errorf("open orders unavailable, request %d", i)
		fx.at.stockProtectionTick(ctx)
	}
	if n := stockNoticeCount(fx, "ALERT", "AAPLBUSDT"); n != 1 {
		t.Fatalf("within window ALERTs=%d: %v", n, fx.alert)
	}
	if n := stockNoticeCount(fx, "INFO", "AAPLBUSDT"); n != 0 {
		t.Fatalf("premature recovery notices=%d: %v", n, fx.alert)
	}
	delete(fx.spot.protErrors, "AAPLBUSDT")
	for i := 0; i < 3; i++ {
		fx.at.stockProtectionTick(ctx)
	}
	if n := stockNoticeCount(fx, "INFO", "AAPLBUSDT"); n != 1 {
		t.Fatalf("recovery notices=%d: %v", n, fx.alert)
	}
	if !strings.Contains(fx.alert[len(fx.alert)-1], "request 59") {
		t.Fatalf("recovery omitted previous reason: %v", fx.alert)
	}
	st, err := fx.at.stockGatherLive(ctx, now, true, true)
	if err != nil || st.Holdings["AAPLBUSDT"].Quarantined || !st.Holdings["AAPLBUSDT"].ProtectionVerified {
		t.Fatalf("recovered holding still quarantined: %+v err=%v", st, err)
	}
	// A recurrence inside the original window stays suppressed, including
	// across a clean pass. At exactly 60 minutes it may alert again.
	fx.spot.protErrors["AAPLBUSDT"] = fmt.Errorf("open orders unavailable again")
	fx.at.stockProtectionTick(ctx)
	if n := stockNoticeCount(fx, "ALERT", "AAPLBUSDT"); n != 1 {
		t.Fatalf("flapping error escaped window: %v", fx.alert)
	}
	now = stockTestNow.Add(time.Hour)
	fx.at.stockProtectionTick(ctx)
	if n := stockNoticeCount(fx, "ALERT", "AAPLBUSDT"); n != 2 {
		t.Fatalf("window did not expire: %v", fx.alert)
	}
}

func TestStockProtectionRecoveryAfterManualReconciliation(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	fx.openLiveOwned(t, 5, 12)
	ctx := context.Background()
	fx.at.stockProtectionTick(ctx)
	if stockNoticeCount(fx, "ALERT", "AAPLBUSDT") != 1 {
		t.Fatalf("unverifiable position not alerted: %v", fx.alert)
	}
	// The operator confirms these shares are manual and clears the stale
	// program record. Account inventory remains completely untouched.
	if err := fx.at.store.GormDB().Exec("UPDATE trader_positions SET status = 'CLOSED' WHERE trader_id = ? AND symbol = ?", "t1", "AAPLBUSDT").Error; err != nil {
		t.Fatal(err)
	}
	fx.at.stockProtectionTick(ctx)
	fx.at.stockProtectionTick(ctx)
	if stockNoticeCount(fx, "INFO", "AAPLBUSDT") != 1 || len(fx.spot.sells)+len(fx.spot.setProt) != 0 || fx.spot.hold["AAPLBUSDT"] != 12 {
		t.Fatalf("manual reconciliation: notices=%v sells=%v protection=%v balance=%g", fx.alert, fx.spot.sells, fx.spot.setProt, fx.spot.hold["AAPLBUSDT"])
	}
}

func TestStockProtectionAlertClassesAreIndependent(t *testing.T) {
	fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
	for i := 0; i < 3; i++ {
		fx.at.stockProtectionAlert("AAPLBUSDT", "get_protection", fmt.Errorf("open orders request %d failed", i))
		fx.at.stockProtectionAlert("AAPLBUSDT", "check_protection", fmt.Errorf("status request %d failed", i))
		fx.at.stockProtectionAlert("MSFTBUSDT", "get_protection", fmt.Errorf("open orders request %d failed", i))
	}
	if len(fx.alert) != 3 || stockNoticeCount(fx, "ALERT", "AAPLBUSDT") != 2 || stockNoticeCount(fx, "ALERT", "MSFTBUSDT") != 1 {
		t.Fatalf("de-dup must use both symbol and reason class: %v", fx.alert)
	}
	fx.at.stockProtectionRecovered("AAPLBUSDT")
	fx.at.stockProtectionRecovered("AAPLBUSDT")
	if stockNoticeCount(fx, "INFO", "AAPLBUSDT") != 1 || stockNoticeCount(fx, "INFO", "MSFTBUSDT") != 0 {
		t.Fatalf("recovery must be once per symbol: %v", fx.alert)
	}
}

func TestStockProtectionPassReadFailuresStillAbort(t *testing.T) {
	for _, failure := range []string{"balance", "positions", "owned_rows", "protection_rows"} {
		t.Run(failure, func(t *testing.T) {
			fx := newStockFixture(t, false, stockTestNow, func(int) string { return holdAAPL })
			fx.openLiveOwned(t, 5, 12)
			switch failure {
			case "balance":
				fx.spot.balanceErr = fmt.Errorf("account unavailable")
			case "positions":
				fx.spot.positionsErr = fmt.Errorf("positions unavailable")
			case "owned_rows":
				if err := fx.at.store.GormDB().Exec("DROP TABLE trader_positions").Error; err != nil {
					t.Fatal(err)
				}
			case "protection_rows":
				if err := fx.at.store.GormDB().Exec("DROP TABLE stock_protection_orders").Error; err != nil {
					t.Fatal(err)
				}
			}
			st, err := fx.at.stockGatherLive(context.Background(), stockTestNow, true, true)
			if err == nil || st != nil {
				t.Fatalf("%s must abort: state=%+v err=%v", failure, st, err)
			}
		})
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
	openNVDA := `[{"symbol":"NVDABUSDT","action":"open_long","entry_type":"market","stop_loss":115,"take_profit":130,"confidence":80,"thesis":"t","invalidation":"i","reasoning":"x"}]`
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
	openNVDA := `[{"symbol":"NVDABUSDT","action":"open_long","entry_type":"market","stop_loss":115,"take_profit":130,"confidence":80,"thesis":"t","invalidation":"i","reasoning":"x"}]`
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
		"MSFTBUSDT|stock|Technology||410|"} {
		if !strings.Contains(user, want) {
			t.Fatalf("user prompt missing %q:\n%s", want, user)
		}
	}
	// Scan-excluded pairs are left out of the universe table.
	if strings.Contains(user, "TSLABUSDT|") {
		t.Fatalf("excluded TSLA must be omitted from the universe table:\n%s", user)
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

// ------------------------------------------------- decision contract (task I)

const objOpenAAPL = `Thinking about it.
{"reasoning":"Risk-on tape; one clean setup.","market_context":{"overall_sentiment":"Bullish","key_factors":["SPY above EMA50","low vol"]},
"decisions":[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"market","stop_loss":115,"take_profit":130,"confidence":85,
"thesis":"Earnings re-rating over weeks","invalidation":"Daily close below 115","reasoning":"trend up"}]}`

func TestStockThesisPersistedCarriedForwardAndDeleted(t *testing.T) {
	step := 0
	fx := newStockFixture(t, true, stockTestNow, func(call int) string {
		step = call
		switch call {
		case 1:
			return objOpenAAPL
		case 2: // hold refines only the thesis; invalidation is kept
			return `{"reasoning":"keep","decisions":[{"symbol":"AAPLBUSDT","action":"hold","thesis":"Re-rating plus buyback"}]}`
		case 3:
			return holdAAPL
		}
		return `{"reasoning":"exit","decisions":[{"symbol":"AAPLBUSDT","action":"close_long","reasoning":"thesis broke"}]}`
	})
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if len(rec.Decisions) != 1 || !rec.Decisions[0].Success {
		t.Fatalf("open record %+v", rec)
	}
	if !strings.Contains(rec.CoTTrace, "Risk-on tape; one clean setup.") || !strings.Contains(rec.CoTTrace, "market_context: bullish; factors: SPY above EMA50; low vol") {
		t.Fatalf("CoTTrace %q", rec.CoTTrace)
	}
	th, err := fx.at.store.StockThesis().List("t1")
	if err != nil || th["AAPLBUSDT"].Thesis != "Earnings re-rating over weeks" || th["AAPLBUSDT"].Invalidation != "Daily close below 115" {
		t.Fatalf("thesis after open: %+v %v", th, err)
	}
	if strings.Contains(fx.ai.user, "交易逻辑") {
		t.Fatalf("the opening prompt has no position yet:\n%s", fx.ai.user)
	}

	// Next cycle: the position's thesis & invalidation are in the prompt.
	fx.cycle(t)
	if !strings.Contains(fx.ai.user, "交易逻辑：Earnings re-rating over weeks | 失效条件：Daily close below 115（止损即失效价）") {
		t.Fatalf("thesis not carried forward:\n%s", fx.ai.user)
	}
	th, _ = fx.at.store.StockThesis().List("t1")
	if th["AAPLBUSDT"].Thesis != "Re-rating plus buyback" || th["AAPLBUSDT"].Invalidation != "Daily close below 115" {
		t.Fatalf("hold refinement: %+v", th)
	}
	fx.cycle(t) // plain hold leaves it alone
	if !strings.Contains(fx.ai.user, "交易逻辑：Re-rating plus buyback | 失效条件：Daily close below 115") {
		t.Fatalf("refined thesis not carried forward:\n%s", fx.ai.user)
	}
	fx.cycle(t) // close
	if step != 4 {
		t.Fatalf("calls %d", step)
	}
	if th, _ = fx.at.store.StockThesis().List("t1"); len(th) != 0 {
		t.Fatalf("thesis must be deleted on close: %+v", th)
	}
	if open, _ := fx.at.store.StockPaper().ListOpen("t1"); len(open) != 0 {
		t.Fatal("position not closed")
	}
}

func TestStockRejectedEntryDoesNotPersistThesis(t *testing.T) {
	// Low confidence: rejected, never repaired, nothing stored.
	low := `{"reasoning":"meh","decisions":[{"symbol":"AAPLBUSDT","action":"open_long","stop_loss":115,"confidence":70,"thesis":"t","invalidation":"i"}]}`
	fx := newStockFixture(t, true, stockTestNow, func(int) string { return low })
	fx.cycle(t)
	if fx.ai.calls != 1 {
		t.Fatalf("LOW_CONFIDENCE must not be repaired, calls=%d", fx.ai.calls)
	}
	rec := fx.lastRecord(t)
	if len(rec.Decisions) != 1 || rec.Decisions[0].Success || !strings.Contains(rec.Decisions[0].Error, "LOW_CONFIDENCE") {
		t.Fatalf("record %+v", rec.Decisions)
	}
	if th, _ := fx.at.store.StockThesis().List("t1"); len(th) != 0 {
		t.Fatalf("rejected entry stored a thesis: %+v", th)
	}
}

func TestStockRepairRoundIsExecutedAndRecorded(t *testing.T) {
	// Price 120, ATR ~2 in the fixture series; stop 119 is far inside the band.
	bad := `{"reasoning":"first","decisions":[{"symbol":"AAPLBUSDT","action":"open_long","stop_loss":119.5,"confidence":85,"thesis":"t1","invalidation":"i1"}]}`
	fixed := `{"reasoning":"fixed","decisions":[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"market","stop_loss":115,"take_profit":130,"confidence":85,"thesis":"t2","invalidation":"i2"}]}`
	fx := newStockFixture(t, true, stockTestNow, func(call int) string {
		if call == 1 {
			return bad
		}
		return fixed
	})
	fx.cycle(t)
	if fx.ai.calls != 2 {
		t.Fatalf("expected exactly one repair call, calls=%d", fx.ai.calls)
	}
	if !strings.Contains(fx.ai.user, "## 修复请求（仅此一次）") || !strings.Contains(fx.ai.user, "- AAPLBUSDT open_long: STOP_OUT_OF_BAND") {
		t.Fatalf("repair prompt:\n%s", fx.ai.user)
	}
	rec := fx.lastRecord(t)
	if !rec.Success || len(rec.Decisions) != 1 || !rec.Decisions[0].Success || rec.Decisions[0].StopLoss != 115 {
		t.Fatalf("repaired entry not executed: %+v", rec.Decisions)
	}
	if rec.RawResponse != bad+"\n\n--- repair ---\n"+fixed {
		t.Fatalf("RawResponse %q", rec.RawResponse)
	}
	logged := false
	for _, l := range rec.ExecutionLog {
		logged = logged || strings.Contains(l, "repair: resubmitted [AAPLBUSDT] → 1 accepted")
	}
	if !logged {
		t.Fatalf("repair log line missing: %v", rec.ExecutionLog)
	}
	if th, _ := fx.at.store.StockThesis().List("t1"); th["AAPLBUSDT"].Thesis != "t2" || th["AAPLBUSDT"].Invalidation != "i2" {
		t.Fatalf("thesis must come from the repaired decision: %+v", th)
	}
	if open, _ := fx.at.store.StockPaper().ListOpen("t1"); len(open) != 1 {
		t.Fatalf("paper positions %d", len(open))
	}
}

func TestStockLegacyArrayResponseStillWorks(t *testing.T) {
	fx := newStockFixture(t, true, stockTestNow, func(int) string { return openAAPL })
	fx.cycle(t)
	rec := fx.lastRecord(t)
	if !rec.Decisions[0].Success || rec.CoTTrace != "" || fx.ai.calls != 1 {
		t.Fatalf("legacy array: %+v cot=%q calls=%d", rec.Decisions, rec.CoTTrace, fx.ai.calls)
	}
}

// Restart keeps resting limit entries (the next start restores them); a plain
// stop still cancels; a restart that comes back in paper mode, or a live start
// whose preflight fails, cancels the kept entries instead of orphaning them.
func TestStockRestingEntriesKeptAcrossRestart(t *testing.T) {
	limit := `[{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"limit","limit_price":119,"stop_loss":115,"take_profit":130,"confidence":80,"thesis":"pullback holds","invalidation":"close below 115","reasoning":"pullback"}]`
	place := func(t *testing.T) *stockFixture {
		fx := newStockFixture(t, false, stockTestNow, func(int) string { return limit })
		fx.cycle(t)
		if pend, _ := fx.at.store.StockPaper().ListPending("t1"); len(pend) != 1 {
			t.Fatalf("pending not placed: %+v", pend)
		}
		return fx
	}

	// 1. StopForRestart keeps the order and the row; a new live start restores it.
	fx := place(t)
	fx.at.StopForRestart()
	if len(fx.spot.cancelled) != 0 {
		t.Fatalf("restart must not cancel resting entries: %v", fx.spot.cancelled)
	}
	if pend, _ := fx.at.store.StockPaper().ListPending("t1"); len(pend) != 1 {
		t.Fatalf("row must survive a restart: %+v", pend)
	}
	fx.at.keepRestingOnStop.Store(false)
	fx.at.isRunning = true
	if _, err := fx.at.prepareStockRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fx.at.stock.pendings["AAPLBUSDT"] == nil {
		t.Fatal("kept entry not restored on the next live start")
	}

	// 2. A plain Stop (user stop / delete) cancels it.
	fx = place(t)
	fx.at.Stop()
	if len(fx.spot.cancelled) != 1 {
		t.Fatalf("plain stop must cancel the resting entry: %v", fx.spot.cancelled)
	}

	// 3. Restart back in paper mode: kept entries are cancelled and forgotten.
	fx = place(t)
	fx.at.StopForRestart()
	paper := true
	fx.at.config.StrategyConfig.StockConfig.PaperTrading = &paper
	// Production paper mode has NO executor (newStockExecutor returns nil); the
	// kept entries are cancelled through a temporary executor built from the
	// trader's own credentials (stockOrphanExecutor seam).
	fx.at.stockTrader = nil
	oldOrphan := stockOrphanExecutor
	t.Cleanup(func() { stockOrphanExecutor = oldOrphan })
	stockOrphanExecutor = func(*AutoTrader) (types.SpotStockTrader, error) { return fx.spot, nil }
	fx.at.isRunning = true
	if _, err := fx.at.prepareStockRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fx.spot.cancelled) != 1 {
		t.Fatalf("paper start must cancel kept live entries: %v", fx.spot.cancelled)
	}
	if pend, _ := fx.at.store.StockPaper().ListPending("t1"); len(pend) != 0 {
		t.Fatalf("cancelled kept entries must be forgotten: %+v", pend)
	}

	// 3b. Paper restart where no executor can be built (no credentials): the
	// entries stay recorded and the user is alerted, nothing is cancelled.
	fx = place(t)
	fx.at.StopForRestart()
	fx.at.config.StrategyConfig.StockConfig.PaperTrading = &paper
	fx.at.stockTrader = nil
	stockOrphanExecutor = func(*AutoTrader) (types.SpotStockTrader, error) { return nil, fmt.Errorf("no credentials") }
	fx.at.isRunning = true
	if _, err := fx.at.prepareStockRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fx.spot.cancelled) != 0 {
		t.Fatalf("no executor: nothing may be cancelled: %v", fx.spot.cancelled)
	}
	if pend, _ := fx.at.store.StockPaper().ListPending("t1"); len(pend) != 1 {
		t.Fatalf("uncancelled entries must stay recorded: %+v", pend)
	}

	// 4. Live start whose preflight fails: kept entries are cancelled.
	fx = place(t)
	fx.at.StopForRestart()
	fx.at.stockTrader = &failingPreflightSpot{fx.spot}
	fx.at.isRunning = true
	if _, err := fx.at.prepareStockRuntime(context.Background()); err == nil {
		t.Fatal("preflight failure must fail the start")
	}
	if len(fx.spot.cancelled) != 1 {
		t.Fatalf("preflight failure must cancel kept entries: %v", fx.spot.cancelled)
	}
}

type failingPreflightSpot struct{ *fakeSpot }

func (f *failingPreflightSpot) Preflight(string) error { return fmt.Errorf("api key revoked") }
