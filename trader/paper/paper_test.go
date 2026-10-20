package paper

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"nofx/store"
	"nofx/trader/types"
)

// The entire suite uses injected observations; no HTTP servers or sockets.
type fakeMarket struct {
	mu                      sync.Mutex
	at                      time.Time
	mark                    float64
	bars                    []Candle
	err                     error
	rule                    QuantityRule
	priceCalls, candleCalls int
	block, entered          chan struct{}
}

func newFake() *fakeMarket {
	return &fakeMarket{at: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), mark: 100, rule: QuantityRule{StepSize: "0.001", MinQty: .001, MinNotional: 10}}
}
func (f *fakeMarket) now() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.at }
func (f *fakeMarket) Price(string) (float64, error) {
	f.mu.Lock()
	f.priceCalls++
	v, err, block, entered := f.mark, f.err, f.block, f.entered
	f.mu.Unlock()
	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if block != nil {
		<-block
	}
	return v, err
}
func (f *fakeMarket) Candles1m(_ string, _ time.Time) ([]Candle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.candleCalls++
	return append([]Candle(nil), f.bars...), f.err
}
func (f *fakeMarket) QuantityRules(string) (QuantityRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rule, nil
}
func (f *fakeMarket) OrderBook(_ string, _ int) ([][]float64, [][]float64, error) {
	return [][]float64{{99.9, 10}}, [][]float64{{100.1, 10}}, nil
}
func (f *fakeMarket) step(d time.Duration, mark float64, bars ...Candle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.at = f.at.Add(d)
	f.mark = mark
	f.bars = append(f.bars, bars...)
}
func bar(at time.Time, low, high, close float64) Candle {
	return Candle{Time: at, Low: low, High: high, Close: close}
}

func setup(t *testing.T, cfg Config) (*PaperFuturesTrader, *store.Store, *fakeMarket) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "paper.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	f := newFake()
	cfg.Now = f.now
	if cfg.ExchangeID == "" {
		cfg.ExchangeID = "acct"
	}
	p := New(cfg, st, f)
	if p.initErr != nil {
		t.Fatal(p.initErr)
	}
	t.Cleanup(p.StopOrderSync)
	return p, st, f
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-8 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}
func summary(t *testing.T, p *PaperFuturesTrader) AccountSummary {
	t.Helper()
	s, e := p.Summary()
	must(t, e)
	return s
}
func count(t *testing.T, st *store.Store, model interface{}) int64 {
	t.Helper()
	var n int64
	must(t, st.GormDB().Model(model).Count(&n).Error)
	return n
}
func keys(m map[string]interface{}) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func limit(t *testing.T, p *PaperFuturesTrader, side, ps string, qty, price float64, reduce bool) *types.LimitOrderResult {
	t.Helper()
	r, e := p.PlaceLimitOrder(&types.LimitOrderRequest{Symbol: "BTCUSDT", Side: side, PositionSide: ps, Quantity: qty, Price: price, Leverage: 10, ReduceOnly: reduce, ClientID: fmt.Sprintf("lim-%s-%.3f-%.2f", side, qty, price)})
	must(t, e)
	return r
}

func TestMarketLongShortAccountingAndBinanceMaps(t *testing.T) {
	for _, side := range []string{"LONG", "SHORT"} {
		t.Run(side, func(t *testing.T) {
			p, st, f := setup(t, Config{InitialBalance: 10000})
			var open, close map[string]interface{}
			var err error
			if side == "LONG" {
				open, err = p.OpenLong("BTCUSDT", 2, 10)
			} else {
				open, err = p.OpenShort("BTCUSDT", 2, 10)
			}
			must(t, err)
			entry, exit, mark := 100.03, 109.967, 110.0
			if side == "SHORT" {
				entry, exit, mark = 99.97, 90.027, 90
			}
			near(t, open["avgPrice"].(float64), entry)
			if _, ok := open["orderId"].(int64); !ok {
				t.Fatal("orderId must match Binance int64")
			}
			for _, k := range []string{"symbol", "status", "clientOrderId"} {
				if _, ok := open[k]; !ok {
					t.Fatal(k)
				}
			}
			positions, err := p.GetPositions()
			must(t, err)
			wantKeys := []string{"entryPrice", "leverage", "liquidationPrice", "markPrice", "positionAmt", "side", "symbol", "unRealizedProfit"}
			if len(positions) != 1 || !reflect.DeepEqual(keys(positions[0]), wantKeys) {
				t.Fatalf("positions: %+v", positions)
			}
			amt := 2.0
			if side == "SHORT" {
				amt = -2
			}
			near(t, positions[0]["positionAmt"].(float64), amt)
			near(t, positions[0]["leverage"].(float64), 10)
			balance, err := p.GetBalance()
			must(t, err)
			if !reflect.DeepEqual(keys(balance), []string{"availableBalance", "totalEquity", "totalUnrealizedProfit", "totalWalletBalance"}) {
				t.Fatal(keys(balance))
			}
			near(t, balance["totalWalletBalance"].(float64), 10000-entry*2*.0005)
			near(t, balance["availableBalance"].(float64), 10000-entry*2*.0005-20)
			f.step(time.Second, mark)
			if side == "LONG" {
				close, err = p.CloseLong("BTCUSDT", 0)
			} else {
				close, err = p.CloseShort("BTCUSDT", 0)
			}
			must(t, err)
			near(t, close["avgPrice"].(float64), exit)
			pnl := (exit - entry) * 2
			if side == "SHORT" {
				pnl = -pnl
			}
			fees := (entry + exit) * 2 * .0005
			s := summary(t, p)
			near(t, s.RealizedPnL, pnl)
			near(t, s.TotalFees, fees)
			near(t, s.Equity, 10000+pnl-fees)
			if s.TradeCount != 2 {
				t.Fatal(s)
			}
			closed, err := p.GetClosedPnL(time.Time{}, 10)
			must(t, err)
			if len(closed) != 1 || closed[0].CloseType != "manual" || closed[0].Leverage != 10 {
				t.Fatal(closed)
			}
			near(t, closed[0].EntryPrice, entry)
			near(t, closed[0].ExitPrice, exit)
			near(t, closed[0].Fee, exit*2*.0005)
			if count(t, st, &store.TraderOrder{}) != 2 || count(t, st, &store.TraderFill{}) != 2 || count(t, st, &store.TraderPosition{}) != 1 {
				t.Fatal("wrong accounting counts")
			}
		})
	}
}

func TestLimitTouchesCancellationAndMakerFees(t *testing.T) {
	for _, side := range []string{"BUY", "SELL"} {
		t.Run(side, func(t *testing.T) {
			p, _, f := setup(t, Config{})
			price, ps := 95.0, "LONG"
			if side == "SELL" {
				price, ps = 105, "SHORT"
			}
			r := limit(t, p, side, ps, 1, price, false)
			orders, err := p.GetOpenOrders("BTCUSDT")
			must(t, err)
			if len(orders) != 1 || orders[0].OrderID != r.OrderID || orders[0].ClientID != r.ClientID {
				t.Fatal(orders)
			}
			at := f.now()
			f.step(time.Minute, 100, bar(at, 96, 104, 100))
			orders, err = p.GetOpenOrders("BTCUSDT")
			must(t, err)
			if len(orders) != 1 {
				t.Fatal("untouched limit filled")
			}
			at = f.now()
			f.step(time.Minute, 100, bar(at, 94, 106, 100))
			orders, err = p.GetOpenOrders("BTCUSDT")
			must(t, err)
			if len(orders) != 0 {
				t.Fatal("touched limit did not fill")
			}
			status, err := p.GetOrderStatus("BTCUSDT", r.OrderID)
			must(t, err)
			near(t, status["avgPrice"].(float64), price)
			near(t, status["commission"].(float64), price*.0002)
			if status["status"] != "FILLED" {
				t.Fatal(status)
			}
			cancel := limit(t, p, side, ps, 1, price, false)
			must(t, p.CancelOrder("BTCUSDT", cancel.OrderID))
			at = f.now()
			f.step(time.Minute, 100, bar(at, 80, 120, 100))
			status, err = p.GetOrderStatus("BTCUSDT", cancel.OrderID)
			must(t, err)
			if status["status"] != "CANCELED" || summary(t, p).TradeCount != 1 {
				t.Fatal(status)
			}
		})
	}
}

func TestProtectiveTriggersAndStopFirst(t *testing.T) {
	for _, side := range []string{"LONG", "SHORT"} {
		for _, trigger := range []string{"sl", "tp", "both"} {
			t.Run(side+"/"+trigger, func(t *testing.T) {
				p, _, f := setup(t, Config{})
				var err error
				if side == "LONG" {
					_, err = p.OpenLong("BTCUSDT", 2, 10)
				} else {
					_, err = p.OpenShort("BTCUSDT", 2, 10)
				}
				must(t, err)
				sl, tp := 90.0, 110.0
				if side == "SHORT" {
					sl, tp = 110, 90
				}
				must(t, p.SetStopLoss("BTCUSDT", side, .25, sl)) // SL ignores quantity
				must(t, p.SetTakeProfit("BTCUSDT", side, 0, tp))
				low, high := 95.0, 105.0
				if trigger == "sl" || trigger == "both" {
					if side == "LONG" {
						low = 89
					} else {
						high = 111
					}
				}
				if trigger == "tp" || trigger == "both" {
					if side == "LONG" {
						high = 111
					} else {
						low = 89
					}
				}
				at := f.now()
				f.step(time.Minute, 100, bar(at, low, high, 100))
				pos, err := p.GetPositions()
				must(t, err)
				if len(pos) != 0 {
					t.Fatal(pos)
				}
				closed, err := p.GetClosedPnL(time.Time{}, 10)
				must(t, err)
				if len(closed) != 1 {
					t.Fatal(closed)
				}
				exit, reason, feeBps := tp, "take_profit", 2.0
				if trigger != "tp" {
					reason, feeBps = "stop_loss", 5
					exit = sl * .9997
					if side == "SHORT" {
						exit = sl * 1.0003
					}
				}
				if closed[0].CloseType != reason || closed[0].Quantity != 2 {
					t.Fatal(closed)
				}
				near(t, closed[0].ExitPrice, exit)
				near(t, closed[0].Fee, exit*2*feeBps/10000)
				orders, err := p.GetOpenOrders("BTCUSDT")
				must(t, err)
				if len(orders) != 0 {
					t.Fatal(orders)
				}
			})
		}
	}
}

func TestHedgeModePartialTPAndCancelSemantics(t *testing.T) {
	p, _, f := setup(t, Config{})
	_, err := p.OpenLong("BTCUSDT", 2, 10)
	must(t, err)
	_, err = p.OpenShort("BTCUSDT", 2, 10)
	must(t, err)
	for _, side := range []string{"LONG", "SHORT"} {
		sl, tp := 80.0, 110.0
		if side == "SHORT" {
			sl, tp = 120, 90
		}
		must(t, p.SetStopLoss("BTCUSDT", side, 99, sl))
		must(t, p.SetTakeProfit("BTCUSDT", side, .5, tp))
	}
	orders, err := p.GetOpenOrders("BTCUSDT")
	must(t, err)
	for _, o := range orders {
		if o.Type == "STOP_MARKET" && (!o.ClosePosition || o.Quantity != 0 || o.ReduceOnly) {
			t.Fatal(o)
		}
		if o.Type == "TAKE_PROFIT_MARKET" && (o.ClosePosition || o.Quantity != .5 || o.ReduceOnly) {
			t.Fatal(o)
		}
	}
	at := f.now()
	f.step(time.Minute, 100, bar(at, 95, 111, 100))
	pos, err := p.GetPositions()
	must(t, err)
	if len(pos) != 2 {
		t.Fatal(pos)
	}
	for _, v := range pos {
		want := 1.5
		if v["side"] == "short" {
			want = -2
		}
		near(t, v["positionAmt"].(float64), want)
	}
	must(t, p.CancelStopLossOrdersForSide("BTCUSDT", "LONG"))
	orders, err = p.GetOpenOrders("BTCUSDT")
	must(t, err)
	if len(orders) != 2 {
		t.Fatal(orders)
	}
	for _, o := range orders {
		if o.PositionSide != "SHORT" {
			t.Fatal(o)
		}
	}
	must(t, p.CancelTakeProfitOrders("BTCUSDT"))
	orders, err = p.GetOpenOrders("BTCUSDT")
	must(t, err)
	if len(orders) != 1 || orders[0].Type != "STOP_MARKET" {
		t.Fatal(orders)
	}
	r := limit(t, p, "BUY", "LONG", 1, 50, false)
	must(t, p.CancelStopOrders("BTCUSDT"))
	orders, err = p.GetOpenOrders("BTCUSDT")
	must(t, err)
	if len(orders) != 1 || orders[0].OrderID != r.OrderID {
		t.Fatal(orders)
	}
	must(t, p.CancelAllOrders("BTCUSDT"))
	orders, err = p.GetOpenOrders("")
	must(t, err)
	if len(orders) != 0 {
		t.Fatal(orders)
	}
}

func TestInsufficientMarginDoesNotMutate(t *testing.T) {
	p, st, _ := setup(t, Config{InitialBalance: 100})
	_, err := p.OpenLong("BTCUSDT", 1, 1)
	if err == nil || !strings.Contains(err.Error(), "Margin is insufficient") {
		t.Fatal(err)
	}
	s := summary(t, p)
	near(t, s.Equity, 100)
	if s.TradeCount != 0 || count(t, st, &store.TraderFill{}) != 0 {
		t.Fatal(s)
	}
	pos, e := p.GetPositions()
	must(t, e)
	if len(pos) != 0 {
		t.Fatal("failed entry survived memory rollback", pos)
	}
	_, err = p.PlaceLimitOrder(&types.LimitOrderRequest{Symbol: "BTCUSDT", Side: "BUY", Quantity: 2, Price: 100, Leverage: 1})
	if err == nil || !strings.Contains(err.Error(), "Margin is insufficient") {
		t.Fatal(err)
	}
}

func TestLiquidationClosesEntireAccountAtMarks(t *testing.T) {
	p, st, f := setup(t, Config{InitialBalance: 100})
	_, err := p.OpenLong("BTCUSDT", 5, 10)
	must(t, err)
	_, err = p.OpenLong("ETHUSDT", 4, 10)
	must(t, err)
	limit(t, p, "BUY", "LONG", 1, 10, false)
	f.step(time.Second, 89)
	pos, err := p.GetPositions()
	must(t, err)
	if len(pos) != 0 {
		t.Fatal(pos)
	}
	closed, err := p.GetClosedPnL(time.Time{}, 10)
	must(t, err)
	if len(closed) != 2 {
		t.Fatal(closed)
	}
	for _, c := range closed {
		if c.CloseType != "liquidation" {
			t.Fatal(c)
		}
		near(t, c.ExitPrice, 89)
	}
	s := summary(t, p)
	near(t, s.RealizedPnL, (89-100.03)*9)
	near(t, s.TotalFees, (100.03+89)*9*.0005)
	near(t, s.Equity, 100+s.RealizedPnL-s.TotalFees)
	orders, err := p.GetOpenOrders("")
	must(t, err)
	if len(orders) != 0 {
		t.Fatal(orders)
	}
	var rows []store.TraderPosition
	must(t, st.GormDB().Find(&rows).Error)
	for _, r := range rows {
		if r.Status != "CLOSED" || r.CloseReason != "liquidation" {
			t.Fatal(r)
		}
	}
}

func TestRestartRestoreSyncIdempotencyAndBinanceAccountingShape(t *testing.T) {
	p, st, f := setup(t, Config{InitialBalance: 1000})
	opened, err := p.OpenShort("BTCUSDT", 2, 10)
	must(t, err)
	must(t, p.SetStopLoss("BTCUSDT", "SHORT", 0, 110))
	r := limit(t, p, "SELL", "SHORT", 1, 130, false)
	must(t, p.SetMarginMode("BTCUSDT", false))
	// Binding after fills transfers provisional rows rather than replaying them.
	must(t, p.SyncOrdersFromBinance("ai", "acct", "paper", st))
	cfg := p.cfg
	cfg.InitialBalance = 999999
	p2 := New(cfg, st, f)
	if p2.initErr != nil {
		t.Fatal(p2.initErr)
	}
	near(t, summary(t, p2).InitialBalance, 1000)
	orders, err := p2.GetOpenOrders("BTCUSDT")
	must(t, err)
	if len(orders) != 2 {
		t.Fatal(orders)
	}
	if p2.s.CrossMargin["BTCUSDT"] || p2.s.Leverage["BTCUSDT"] != 10 {
		t.Fatal("settings not restored")
	}
	must(t, p2.CancelOrder("BTCUSDT", r.OrderID))
	at := f.now()
	f.step(time.Minute, 100, bar(at, 95, 111, 100)) // offline trigger despite mark reverting
	for i := 0; i < 4; i++ {
		must(t, p2.SyncOrdersFromBinance("ai", "acct", "paper", st))
	}
	if count(t, st, &store.TraderFill{}) != 2 || count(t, st, &store.TraderOrder{}) != 2 || count(t, st, &store.TraderPosition{}) != 1 {
		t.Fatal("duplicate accounting")
	}
	var pos store.TraderPosition
	must(t, st.GormDB().First(&pos).Error)
	if pos.TraderID != "ai" || pos.ExchangeID != "acct" || pos.ExchangeType != "paper" || pos.Symbol != "BTCUSDT" || pos.Side != "SHORT" || pos.Status != "CLOSED" || pos.Source != "sync" || pos.CloseReason != "stop_loss" || pos.EntryOrderID != strconv.FormatInt(opened["orderId"].(int64), 10) || pos.ExitOrderID == "" || pos.EntryTime != at.UnixMilli() || pos.ExitTime != f.now().UnixMilli() || pos.Leverage != 10 {
		t.Fatalf("position shape: %+v", pos)
	}
	near(t, pos.EntryQuantity, 2)
	near(t, pos.Quantity, 2)
	near(t, pos.EntryPrice, 99.97)
	near(t, pos.ExitPrice, 110.033)
	near(t, pos.RealizedPnL, (99.97-110.033)*2)
	near(t, pos.Fee, (99.97+110.033)*2*.0005)
	var fills []store.TraderFill
	must(t, st.GormDB().Order("id").Find(&fills).Error)
	for i, fill := range fills {
		if fill.TraderID != "ai" || fill.ExchangeID != "acct" || fill.ExchangeType != "paper" || fill.Symbol != "BTCUSDT" || fill.CommissionAsset != "USDT" || fill.ExchangeTradeID == "" || fill.ExchangeOrderID == "" || fill.OrderID <= 0 || fill.IsMaker {
			t.Fatal(fill)
		}
		order, err := st.Order().GetOrderByExchangeID("acct", fill.ExchangeOrderID)
		must(t, err)
		if order == nil || order.ID != fill.OrderID || order.Status != "FILLED" || order.PositionSide != "SHORT" || order.FilledAt != fill.CreatedAt {
			t.Fatal(order)
		}
		action, side := "open_short", "SELL"
		if i == 1 {
			action, side = "close_short", "BUY"
		}
		if order.OrderAction != action || order.Side != side || fill.Side != side {
			t.Fatal(order, fill)
		}
		near(t, fill.QuoteQuantity, fill.Quantity*fill.Price)
		near(t, order.Commission, fill.Commission)
	}
	p3 := New(cfg, st, f)
	near(t, summary(t, p3).Equity, summary(t, p2).Equity)
	must(t, p3.SyncOrdersFromBinance("ai", "acct", "paper", st))
	if count(t, st, &store.TraderFill{}) != 2 {
		t.Fatal("restart replayed receipts")
	}
}

func TestFullDayShortFallsThenStopsOut(t *testing.T) {
	p, st, f := setup(t, Config{InitialBalance: 5000})
	must(t, p.SyncOrdersFromBinance("day-trader", "acct", "paper", st))
	_, err := p.OpenShort("BTCUSDT", 3, 10)
	must(t, err)
	must(t, p.SetStopLoss("BTCUSDT", "SHORT", 0, 105))
	start := f.now()
	bars := make([]Candle, 0, 1440)
	for minute := 0; minute < 1440; minute++ {
		price := 100 - 15*float64(minute)/720
		if minute >= 720 {
			price = 85 + 25*float64(minute-720)/719
		}
		bars = append(bars, bar(start.Add(time.Duration(minute)*time.Minute), price-.1, price+.1, price))
	}
	// A single offline interval replays the whole day, including the trough.
	f.step(24*time.Hour, 110, bars...)
	must(t, p.SyncOrdersFromBinance("day-trader", "acct", "paper", st))
	s := summary(t, p)
	near(t, s.RealizedPnL, (99.97-105.0315)*3)
	near(t, s.TotalFees, (99.97+105.0315)*3*.0005)
	near(t, s.UnrealizedPnL, 0)
	near(t, s.Equity, 5000+s.RealizedPnL-s.TotalFees)
	if s.TradeCount != 2 {
		t.Fatal(s)
	}
	closed, err := p.GetClosedPnL(start, 10)
	must(t, err)
	if len(closed) != 1 || closed[0].CloseType != "stop_loss" || !closed[0].ExitTime.After(start.Add(12*time.Hour)) {
		t.Fatal(closed)
	}
}

func TestQuantityRulesAndConfiguredCosts(t *testing.T) {
	p, _, f := setup(t, Config{TakerFeeBps: 10, MakerFeeBps: 4, SlippageBps: 20, DefaultLeverage: 7, MaintenanceMarginPct: 1})
	f.mu.Lock()
	f.rule = QuantityRule{StepSize: "0.025", MinQty: .025, MaxQty: 10}
	f.mu.Unlock()
	text, err := p.FormatQuantity("BTCUSDT", 1.249)
	must(t, err)
	if text != "1.225" {
		t.Fatal(text)
	}
	for _, q := range []float64{0, -1, .001, 11, math.NaN(), math.Inf(1)} {
		if _, e := p.FormatQuantity("BTCUSDT", q); e == nil {
			t.Fatal(q)
		}
	}
	_, err = p.OpenLong("BTCUSDT", 1.249, 0)
	must(t, err)
	s := summary(t, p)
	near(t, s.TotalFees, 100.2*1.225*.001)
	pos, err := p.GetPositions()
	must(t, err)
	near(t, pos[0]["leverage"].(float64), 7)
	// Underflowed TP falls back to full-position, exactly as Binance.
	must(t, p.SetTakeProfit("BTCUSDT", "LONG", .001, 110))
	orders, err := p.GetOpenOrders("BTCUSDT")
	must(t, err)
	if len(orders) != 1 || !orders[0].ClosePosition {
		t.Fatal(orders)
	}
	at := f.now()
	f.step(time.Minute, 105, bar(at, 100, 111, 105))
	s = summary(t, p)
	near(t, s.TotalFees, 100.2*1.225*.001+110*1.225*.0004)
}

func TestAdvanceFailureRetainsCursorAndRetries(t *testing.T) {
	p, _, f := setup(t, Config{})
	_, err := p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	must(t, p.SetStopLoss("BTCUSDT", "LONG", 0, 90))
	at := f.now()
	f.step(time.Minute, 100, bar(at, 89, 100, 100))
	f.mu.Lock()
	f.err = errors.New("fake outage")
	f.mu.Unlock()
	if err := p.Advance(f.now()); err == nil {
		t.Fatal("expected source error")
	}
	p.mu.Lock()
	cursor := p.s.LastAdvance
	p.mu.Unlock()
	if !cursor.Equal(at) {
		t.Fatal(cursor)
	}
	f.mu.Lock()
	f.err = nil
	f.mu.Unlock()
	must(t, p.Advance(f.now()))
	if summary(t, p).TradeCount != 2 {
		t.Fatal("missed stop after retry")
	}
}

func TestTransactionsRollBackLedgerAndFills(t *testing.T) {
	p, st, _ := setup(t, Config{})
	// Fail after order insertion, exercising rollback of nested ApplyTrade.
	must(t, st.GormDB().Exec(`CREATE TRIGGER reject_paper_fill BEFORE INSERT ON trader_fills BEGIN SELECT RAISE(ABORT, 'injected fill failure'); END`).Error)
	_, err := p.OpenLong("BTCUSDT", 1, 10)
	if err == nil {
		t.Fatal("expected DB failure")
	}
	if count(t, st, &store.TraderOrder{}) != 0 || count(t, st, &store.TraderFill{}) != 0 || count(t, st, &store.TraderPosition{}) != 0 {
		t.Fatal("partial transaction committed")
	}
	row, e := st.PaperLedger().Get("acct")
	must(t, e)
	near(t, row.WalletBalance, 10000)
	if summary(t, p).TradeCount != 0 {
		t.Fatal("memory not rolled back")
	}
	must(t, st.GormDB().Exec(`DROP TRIGGER reject_paper_fill`).Error)
	_, err = p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	if count(t, st, &store.TraderFill{}) != 1 {
		t.Fatal("retry did not account")
	}
}

func TestTwoInstancesCannotOverwriteAccount(t *testing.T) {
	p, st, f := setup(t, Config{})
	p2 := New(p.cfg, st, f)
	_, err := p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	_, err = p2.OpenShort("ETHUSDT", 1, 10)
	if !errors.Is(err, store.ErrPaperLedgerConflict) {
		t.Fatal(err)
	}
	_, err = p2.OpenShort("ETHUSDT", 1, 10)
	must(t, err)
	if count(t, st, &store.TraderFill{}) != 2 {
		t.Fatal("stale ledger duplicated fills")
	}
	p3 := New(p.cfg, st, f)
	if len(p3.s.Positions) != 2 {
		t.Fatal(p3.s.Positions)
	}
}

func TestAdvanceNetworkOutsideMutexAndStaleSnapshot(t *testing.T) {
	p, _, f := setup(t, Config{})
	_, err := p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	must(t, p.SetStopLoss("BTCUSDT", "LONG", 0, 90))
	f.mu.Lock()
	f.block = make(chan struct{})
	f.entered = make(chan struct{}, 1)
	block, entered := f.block, f.entered
	f.mu.Unlock()
	f.step(time.Minute, 100)
	done := make(chan error, 1)
	go func() { done <- p.Advance(f.now()) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("source did not start")
	}
	changed := make(chan struct{})
	go func() { p.SetDisplayName("still responsive"); _ = p.SetLeverage("BTCUSDT", 11); close(changed) }()
	select {
	case <-changed:
	case <-time.After(time.Second):
		close(block)
		t.Fatal("network request held state mutex")
	}
	close(block)
	select {
	case e := <-done:
		must(t, e)
	case <-time.After(2 * time.Second):
		t.Fatal("advance did not complete")
	}
	p.mu.Lock()
	lev := p.s.Positions["BTCUSDT:LONG"].Leverage
	p.mu.Unlock()
	if lev != 11 {
		t.Fatal("stale observation overwrote setting")
	}
}

func TestConcurrentPublicMethodsAndSyncLifecycle(t *testing.T) {
	p, st, f := setup(t, Config{})
	must(t, p.SyncOrdersFromBinance("ai", "acct", "paper", st))
	_, err := p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	f.step(time.Second, 101)
	p.StartOrderSync("ai", "acct", "paper", st, time.Millisecond)
	p.StartOrderSync("ai", "acct", "paper", st, time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				var e error
				switch i {
				case 0:
					_, e = p.GetBalance()
				case 1:
					_, e = p.GetPositions()
				case 2:
					_, e = p.GetOpenOrders("")
				case 3:
					_, e = p.Summary()
				case 4:
					e = p.SetLeverage("BTCUSDT", 10+j)
				case 5:
					e = p.SetMarginMode("BTCUSDT", j%2 == 0)
				case 6:
					e = p.SyncOrdersFromBinance("ai", "acct", "paper", st)
				case 7:
					p.SetDisplayName("parallel")
					p.InvalidateAccountCache()
					p.InvalidatePositionCache()
					_, e = p.FormatQuantity("BTCUSDT", 1.2345)
				}
				if e != nil {
					t.Errorf("concurrent operation: %v", e)
				}
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); p.StopOrderSync() }()
	}
	wg.Wait()
	p.StartOrderSync("ai", "acct", "paper", st, time.Millisecond)
	p.StopOrderSync()
	if count(t, st, &store.TraderFill{}) != 1 {
		t.Fatal("sync duplicated fill")
	}
}

func TestPlacementMinuteDoesNotUseOldExtremes(t *testing.T) {
	p, _, f := setup(t, Config{})
	f.step(30*time.Second, 100)
	_, err := p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	must(t, p.SetStopLoss("BTCUSDT", "LONG", 0, 90))
	at := f.now().Truncate(time.Minute)
	f.step(30*time.Second, 100, bar(at, 80, 105, 100))
	if summary(t, p).TradeCount != 1 {
		t.Fatal("pre-order low fired SL")
	}
	f.step(time.Second, 89) // a sampled live quote is valid, even within a minute
	if summary(t, p).TradeCount != 2 {
		t.Fatal("live quote failed to trigger")
	}
}

func TestReduceOnlyLimitAndSmallRemainder(t *testing.T) {
	p, st, f := setup(t, Config{})
	f.mu.Lock()
	f.rule = QuantityRule{StepSize: "0.00001"}
	f.mu.Unlock()
	_, err := p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	limit(t, p, "SELL", "LONG", .99995, 110, true)
	at := f.now()
	f.step(time.Minute, 105, bar(at, 100, 111, 105))
	pos, err := p.GetPositions()
	must(t, err)
	if len(pos) != 1 {
		t.Fatal(pos)
	}
	near(t, pos[0]["positionAmt"].(float64), .00005)
	var row store.TraderPosition
	must(t, st.GormDB().First(&row).Error)
	if row.Status != "OPEN" || row.ExitTime != 0 {
		t.Fatal(row)
	}
	near(t, row.Quantity, .00005)
	_, err = p.CloseLong("BTCUSDT", 0)
	must(t, err)
	pos, err = p.GetPositions()
	must(t, err)
	if len(pos) != 0 {
		t.Fatal(pos)
	}
	if _, err = p.PlaceLimitOrder(&types.LimitOrderRequest{Symbol: "BTCUSDT", Side: "SELL", PositionSide: "LONG", Quantity: 1, Price: 110, ReduceOnly: true}); err == nil {
		t.Fatal("reduce-only opened position")
	}
}

func TestSameMinuteHedgesAreMarkedTogether(t *testing.T) {
	p, _, f := setup(t, Config{InitialBalance: 100})
	_, err := p.OpenLong("BTCUSDT", 4.9, 10)
	must(t, err)
	_, err = p.OpenShort("ETHUSDT", 4.9, 10)
	must(t, err)
	before := summary(t, p)
	at := f.now()
	f.step(time.Minute, 70, bar(at, 69, 101, 70))
	pos, err := p.GetPositions()
	must(t, err)
	if len(pos) != 2 || summary(t, p).TradeCount != 2 {
		t.Fatal("mixed timestamps spuriously liquidated a hedge", pos)
	}
	near(t, summary(t, p).Equity, before.Equity)
}

func TestHighLeverageLiquidatesImmediately(t *testing.T) {
	p, _, _ := setup(t, Config{InitialBalance: 100})
	_, err := p.OpenLong("BTCUSDT", 190, 1000)
	must(t, err)
	s := summary(t, p)
	if s.TradeCount != 2 {
		t.Fatal("entry below maintenance must liquidate immediately", s)
	}
	closed, err := p.GetClosedPnL(time.Time{}, 10)
	must(t, err)
	if len(closed) != 1 || closed[0].CloseType != "liquidation" {
		t.Fatal(closed)
	}
	near(t, s.Equity, 100+s.RealizedPnL-s.TotalFees)
}

func TestChangedLeverageUpdatesAccountingRows(t *testing.T) {
	p, st, _ := setup(t, Config{})
	_, err := p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	must(t, p.SetLeverage("BTCUSDT", 7))
	var row store.TraderPosition
	must(t, st.GormDB().First(&row).Error)
	if row.Leverage != 7 {
		t.Fatal(row)
	}
	pos, err := p.GetPositions()
	must(t, err)
	near(t, pos[0]["leverage"].(float64), 7)
}

func TestBackgroundSyncActuallyAdvances(t *testing.T) {
	p, st, f := setup(t, Config{})
	r := limit(t, p, "BUY", "LONG", 1, 95, false)
	at := f.now()
	f.step(time.Minute, 100, bar(at, 94, 100, 100))
	p.StartOrderSync("ai", "acct", "paper", st, time.Millisecond)
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("background loop did not fill limit")
		case <-ticker.C:
			p.mu.Lock()
			status := p.s.Orders[r.OrderID].Status
			p.mu.Unlock()
			if status == "FILLED" {
				p.StopOrderSync()
				if count(t, st, &store.TraderFill{}) != 1 {
					t.Fatal("missing background receipt")
				}
				return
			}
		}
	}
}

func TestReopenDatabaseRestoresPendingTrigger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	st, err := store.New(path)
	must(t, err)
	f := newFake()
	cfg := Config{ExchangeID: "persist", InitialBalance: 1234, Now: f.now}
	p := New(cfg, st, f)
	_, err = p.OpenLong("BTCUSDT", 1, 10)
	must(t, err)
	must(t, p.SetStopLoss("BTCUSDT", "LONG", 0, 90))
	must(t, st.Close())
	at := f.now()
	f.step(time.Hour, 100, bar(at.Add(30*time.Minute), 89, 100, 100))
	st, err = store.New(path)
	must(t, err)
	defer st.Close()
	cfg.InitialBalance = 1
	p = New(cfg, st, f)
	s := summary(t, p)
	if s.InitialBalance != 1234 || s.TradeCount != 2 {
		t.Fatal(s)
	}
	near(t, s.Equity, 1234+(89.973-100.03)-(100.03+89.973)*.0005)
	must(t, p.SyncOrdersFromBinance("ai", "persist", "paper", st))
	if count(t, st, &store.TraderFill{}) != 2 {
		t.Fatal("database reopen replayed fills")
	}
}

func TestConcurrentTradingIsSerialized(t *testing.T) {
	p, st, _ := setup(t, Config{})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				_, err = p.OpenLong("BTCUSDT", .1, 10)
			} else {
				_, err = p.OpenShort("BTCUSDT", .1, 10)
			}
			if err != nil {
				t.Errorf("entry: %v", err)
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				_, err = p.CloseLong("BTCUSDT", .1)
			} else {
				_, err = p.CloseShort("BTCUSDT", .1)
			}
			if err != nil {
				t.Errorf("close: %v", err)
			}
		}(i)
	}
	wg.Wait()
	pos, err := p.GetPositions()
	must(t, err)
	if len(pos) != 2 {
		t.Fatal(pos)
	}
	for _, v := range pos {
		near(t, math.Abs(v["positionAmt"].(float64)), .1)
	}
	if count(t, st, &store.TraderFill{}) != 22 {
		t.Fatal("lost concurrent fills")
	}
}
