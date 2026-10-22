package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"nofx/planning"
	"nofx/planning/core"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type testMarket struct {
	book core.Snapshot
	fail bool
}

func (m *testMarket) Snapshot(context.Context, string, string) (core.Snapshot, error) {
	if m.fail {
		return core.Snapshot{}, errors.New("offline")
	}
	b := m.book
	b.Observed = time.Now().UTC()
	return b, nil
}

type testBroker struct {
	mu                     sync.Mutex
	orders                 map[string]Order
	placed                 []Order
	cancelled              []string
	lookedUp               []string
	account                Account
	prepareErr, errorPlace error
	lookupErr              bool
	fillEntryImmediately   bool
	invalidExpiryReceipt   bool
}

func (b *testBroker) Venue() string { return "binance" }
func (b *testBroker) Account(context.Context, string) (Account, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.account, nil
}
func (b *testBroker) Prepare(context.Context, string) error { return b.prepareErr }
func (b *testBroker) Place(_ context.Context, o Order) (Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.placed = append(b.placed, o)
	o.OrderID = fmt.Sprint(len(b.placed))
	o.Status = "NEW"
	o.Filled = "0"
	if b.fillEntryImmediately && o.Role == "entry" {
		o.Status, o.Filled = "FILLED", o.Quantity
	}
	b.orders[o.ClientID] = o
	if b.invalidExpiryReceipt {
		o.Status, o.OrderID = "EXPIRED", ""
	}
	return o, b.errorPlace
}

func TestImmediateFillWaitsForPositionBeforeCompletion(t *testing.T) {
	for _, kind := range []string{"limit", "market"} {
		t.Run(kind, func(t *testing.T) {
			c, b, _, p := setup(t, kind)
			v := started(t, c, p)
			b.fillEntryImmediately = true
			for range 2 {
				if c.step(context.Background(), "alice", v.ID) {
					t.Fatal("monitor stopped before position confirmation")
				}
			}
			got, _ := c.Get("alice", v.ID)
			var leases int64
			c.DB.Model(&Lease{}).Count(&leases)
			if got.State == "completed" || leases != 1 || len(b.placed) != 1 {
				t.Fatal("filled entry lost its reservation or was resubmitted", got.State, leases, len(b.placed))
			}
			b.account.Position = got.Orders[0].Filled
			for range 2 {
				c.step(context.Background(), "alice", v.ID)
			}
			if len(b.placed) != 3 || b.placed[1].Role != "sl" || b.placed[2].Role != "tp" {
				t.Fatal("confirmed position did not receive exactly one protection pair", b.placed)
			}
			// Only a confirmed exit fill and flat position can finish automatically.
			tp := b.orders[b.placed[2].ClientID]
			tp.Status, tp.Filled = "FILLED", tp.Quantity
			b.orders[tp.ClientID] = tp
			b.account.Position = "0"
			if !c.step(context.Background(), "alice", v.ID) {
				t.Fatal("confirmed closed trade did not finish")
			}
			got, _ = c.Get("alice", v.ID)
			c.DB.Model(&Lease{}).Count(&leases)
			if got.State != "completed" || leases != 0 {
				t.Fatal("closed trade retained reservation")
			}
		})
	}
}

func (b *testBroker) Lookup(_ context.Context, o Order) (Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lookedUp = append(b.lookedUp, o.ClientID)
	if b.lookupErr {
		return o, errors.New("unknown")
	}
	r, ok := b.orders[o.ClientID]
	if !ok {
		return o, errors.New("unknown")
	}
	return r, nil
}
func (b *testBroker) Cancel(_ context.Context, o Order) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cancelled = append(b.cancelled, o.ClientID)
	r := b.orders[o.ClientID]
	r.Status = "CANCELED"
	b.orders[o.ClientID] = r
	return nil
}

// Exercise the production NOFX bridge in scheduler/approval/recovery tests.
type testNOFXTrader struct{ *testBroker }

func (b testNOFXTrader) AccountSnapshot(c context.Context, s string) (Account, error) {
	return b.Account(c, s)
}
func (b testNOFXTrader) PrepareExecution(c context.Context, s string) error    { return b.Prepare(c, s) }
func (b testNOFXTrader) SubmitOrder(c context.Context, o Order) (Order, error) { return b.Place(c, o) }
func (b testNOFXTrader) LookupOrder(c context.Context, o Order) (Order, error) { return b.Lookup(c, o) }
func (b testNOFXTrader) CancelExecutionOrder(c context.Context, o Order) error { return b.Cancel(c, o) }

func setup(t *testing.T, kind string) (*Controller, *testBroker, *testMarket, core.Plan) {
	t.Helper()
	db, e := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "planning.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	now := time.Now().UTC()
	m := &testMarket{book: core.Snapshot{Venue: "binance", Symbol: "BTCUSDT", Kind: "perpetual", Bid: "99999", Ask: "100000", BidSize: "100", AskSize: "100", Tick: "1", Step: "0.00001", MinQty: "0.00001", MinNotional: "10", Observed: now}}
	idea := core.Idea{Name: "plan", Side: "long", Entry: "100000", Stop: "99000", Target: "104000"}
	if kind == "market" {
		idea.Execution = &core.Execution{Kind: "market"}
	}
	if kind == "twap" {
		idea.Execution = &core.Execution{Kind: "twap", Slices: 2, IntervalSeconds: 30}
	}
	if kind == "ladder" {
		idea.Execution = &core.Execution{Kind: "ladder", Levels: []core.Level{{Price: "100000", Weight: "0.5"}, {Price: "99800", Weight: "0.5"}}}
	}
	p, e := core.Compile(m.book, core.Constraints{Equity: "10000", MaxRisk: "100", MaxNotional: "1000", MinRewardRisk: "2", FeeBPS: "10", SlippageBPS: "10"}, idea, now)
	if e != nil {
		t.Fatal(e)
	}
	b := &testBroker{orders: map[string]Order{}, account: Account{Equity: "10000", Available: "10000", FeeBPS: "5", Position: "0"}}
	c, e := New(db, func(context.Context, string, string) (Broker, string, error) {
		return NOFXBroker{Trader: testNOFXTrader{b}}, "account", nil
	}, m)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(c.Shutdown)
	ds := planning.DraftStore{DB: db, UserID: "alice", ExchangeID: "exchange", Venue: "binance"}
	if e = ds.Add([]core.Plan{p}); e != nil {
		t.Fatal(e)
	}
	return c, b, m, p
}
func approved(t *testing.T, c *Controller, p core.Plan) View {
	t.Helper()
	v, e := c.Approve(context.Background(), "alice", p.ID, p.Hash, "APPROVE LIVE")
	if e != nil {
		t.Fatal(e)
	}
	return v
}

// Tests drive durable ticks explicitly, without real orders or timers.
func started(t *testing.T, c *Controller, p core.Plan) View {
	v := approved(t, c, p)
	now := time.Now().UTC()
	deadline := now.Add(time.Hour)
	v.StartedAt = &now
	v.Deadline = &deadline
	v.State = "running"
	specs := p.Orders
	if len(specs) == 0 {
		specs = []core.OrderSpec{{Index: 1, Price: p.Idea.Entry, Quantity: p.Quantity}}
	}
	for _, s := range specs {
		v.Orders = append(v.Orders, Order{Type: entryOrderType(p), ClientID: cid(v.ID, s.Index, "entry", ""), Role: "entry", Symbol: p.Snapshot.Symbol, Side: p.Idea.Side, Price: s.Price, Quantity: s.Quantity, Status: "scheduled", Filled: "0"})
	}
	if e := c.save(&v); e != nil {
		t.Fatal(e)
	}
	return v
}
func TestApprovalOwnerHashAndAccountLease(t *testing.T) {
	c, b, _, p := setup(t, "limit")
	for _, args := range [][3]string{{"bob", p.Hash, "APPROVE LIVE"}, {"alice", "wrong", "APPROVE LIVE"}, {"alice", p.Hash, "approve live"}} {
		if _, e := c.Approve(context.Background(), args[0], p.ID, args[1], args[2]); e == nil {
			t.Fatal("invalid approval accepted")
		}
	}
	v := approved(t, c, p)
	if _, e := c.Get("bob", v.ID); e == nil {
		t.Fatal("cross user read")
	}
	if _, e := c.Execute(context.Background(), "alice", v.ID, "bad"); e == nil {
		t.Fatal("bad execution hash")
	}
	if len(b.placed) != 0 {
		t.Fatal("ordered without approval")
	}
	if _, e := c.Approve(context.Background(), "alice", p.ID, p.Hash, "APPROVE LIVE"); e == nil {
		t.Fatal("approval replay")
	}
	ds := planning.DraftStore{DB: c.DB, UserID: "alice", ExchangeID: "exchange", Venue: "binance"}
	if e := ds.Revise(p.ID, []core.Plan{p}, time.Now()); e == nil {
		t.Fatal("AI revised locked plan")
	}
}
func TestExecutePreflightFailuresNeverOrder(t *testing.T) {
	for _, name := range []string{"balance", "fee", "position", "pending", "leverage", "market"} {
		t.Run(name, func(t *testing.T) {
			c, b, m, p := setup(t, "limit")
			v := approved(t, c, p)
			switch name {
			case "balance":
				b.account.Available = "1"
			case "fee":
				b.account.FeeBPS = "20"
			case "position":
				b.account.Position = "1"
			case "pending":
				b.account.OpenOrders = 1
			case "leverage":
				b.prepareErr = errors.New("denied")
			case "market":
				m.fail = true
			}
			if _, e := c.Execute(context.Background(), "alice", v.ID, v.ApprovalHash); e == nil {
				t.Fatal("unsafe preflight accepted")
			}
			if len(b.placed) != 0 {
				t.Fatal("preflight ordered")
			}
			stored, err := c.Get("alice", v.ID)
			var leases int64
			c.DB.Model(&Lease{}).Count(&leases)
			if err != nil || stored.State != "cancelled" || leases != 0 || stored.Error == "" {
				t.Fatalf("failed preflight left an unused account reservation: %+v %v leases=%d", stored.Run, err, leases)
			}
			if _, err = c.Execute(context.Background(), "alice", v.ID, v.ApprovalHash); err == nil {
				t.Fatal("retired approval was accepted again")
			}
		})
	}
}
func TestConcurrentExecutionIsIdempotent(t *testing.T) {
	c, b, _, p := setup(t, "limit")
	v := approved(t, c, p)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := c.Execute(context.Background(), "alice", v.ID, v.ApprovalHash); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		n := len(b.placed)
		b.mu.Unlock()
		if n == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("did not submit exactly one entry")
}
func TestAmbiguousAcknowledgementNeverResubmits(t *testing.T) {
	c, b, _, p := setup(t, "limit")
	v := started(t, c, p)
	b.errorPlace = errors.New("timeout after accepting order")
	c.step(context.Background(), "alice", v.ID)
	got, _ := c.Get("alice", v.ID)
	if got.State != "needs_attention" || got.Orders[0].Status != "UNKNOWN" {
		t.Fatal(got)
	}
	b.errorPlace = nil
	c.step(context.Background(), "alice", v.ID)
	c.step(context.Background(), "alice", v.ID)
	if len(b.placed) != 1 {
		t.Fatal("resent ambiguous entry")
	}
}
func TestPartialFillProtectedBeforeRemainingEntry(t *testing.T) {
	c, b, _, p := setup(t, "twap")
	v := started(t, c, p)
	c.step(context.Background(), "alice", v.ID)
	first := b.orders[v.Orders[0].ClientID]
	first.Status = "PARTIALLY_FILLED"
	first.Filled = "0.001"
	b.orders[first.ClientID] = first
	b.account.Position = "0.001"
	c.step(context.Background(), "alice", v.ID)
	if len(b.placed) != 3 || b.placed[1].Role != "sl" || b.placed[2].Role != "tp" {
		t.Fatal(b.placed)
	}
	for _, o := range b.placed[1:] {
		if o.Quantity != "0.001000000000" || o.ParentID != first.ClientID {
			t.Fatal(o)
		}
	}
	c.step(context.Background(), "alice", v.ID)
	if len(b.placed) != 3 {
		t.Fatal("duplicate protection or premature TWAP")
	}
}
func TestUnknownProtectorRetainsAccountLock(t *testing.T) {
	c, b, _, p := setup(t, "limit")
	v := started(t, c, p)
	c.step(context.Background(), "alice", v.ID)
	first := b.orders[v.Orders[0].ClientID]
	first.Status = "FILLED"
	first.Filled = first.Quantity
	b.orders[first.ClientID] = first
	b.account.Position = first.Quantity
	b.errorPlace = errors.New("lost protection ack")
	c.step(context.Background(), "alice", v.ID)
	b.lookupErr = true
	b.account.Position = "0"
	c.step(context.Background(), "alice", v.ID)
	got, _ := c.Get("alice", v.ID)
	if got.State == "completed" {
		t.Fatal("unknown protector considered completed")
	}
	var leases int64
	c.DB.Model(&Lease{}).Count(&leases)
	if leases != 1 {
		t.Fatal("released unresolved account")
	}
	if len(b.placed) != 2 {
		t.Fatal("retried unknown protector")
	}
}
func TestRestartAndPersistedTamperDoNotReplay(t *testing.T) {
	c, b, m, p := setup(t, "limit")
	v := started(t, c, p)
	c.step(context.Background(), "alice", v.ID)
	next, e := New(c.DB, c.Factory, m)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Shutdown()
	got, _ := next.Get("alice", v.ID)
	if got.State != "needs_attention" {
		t.Fatal(got.State)
	}
	next.step(context.Background(), "alice", v.ID)
	if len(b.placed) != 1 {
		t.Fatal("restart replay")
	}
	got, _ = next.Get("alice", v.ID)
	got.Orders[0].Price = "1"
	raw, _ := json.Marshal(got.Orders)
	c.DB.Model(&Run{}).Where("id = ?", v.ID).Update("orders_json", string(raw))
	if _, e = next.Get("alice", v.ID); e == nil {
		t.Fatal("persisted unapproved intent accepted")
	}
}
func TestCancelledProtectionStopsNewEntries(t *testing.T) {
	c, b, _, p := setup(t, "twap")
	v := started(t, c, p)
	c.step(context.Background(), "alice", v.ID)
	first := b.orders[v.Orders[0].ClientID]
	first.Status = "PARTIALLY_FILLED"
	first.Filled = "0.001"
	b.orders[first.ClientID] = first
	b.account.Position = first.Filled
	c.step(context.Background(), "alice", v.ID)
	sl := b.orders[b.placed[1].ClientID]
	sl.Status = "CANCELED"
	b.orders[sl.ClientID] = sl
	c.step(context.Background(), "alice", v.ID)
	got, _ := c.Get("alice", v.ID)
	if got.State != "needs_attention" || got.Orders[1].Status != "CANCELED" {
		t.Fatal(got)
	}
	if len(b.placed) != 3 {
		t.Fatal("replaced canceled protector without new approval")
	}
}

func TestExpiredApprovalCanBeCancelledWithoutOrders(t *testing.T) {
	c, b, _, p := setup(t, "limit")
	v := approved(t, c, p)
	if e := c.DB.Model(&Run{}).Where("id = ?", v.ID).Update("state", "expired").Error; e != nil {
		t.Fatal(e)
	}
	out, e := c.Stop("alice", v.ID, v.ApprovalHash)
	if e != nil || out.State != "cancelled" {
		t.Fatal(out, e)
	}
	if len(b.placed) != 0 {
		t.Fatal("cancel expired approval submitted orders")
	}
	var count int64
	c.DB.Model(&Lease{}).Count(&count)
	if count != 0 {
		t.Fatal("unreleased expired lease")
	}
}

type routedTestBroker struct{ *testBroker }

func (*routedTestBroker) RouteClientID(id string) (string, error) { return "test-broker-" + id, nil }
func TestExecutionPersistsBrokerRouteBeforeSubmit(t *testing.T) {
	c, b, _, p := setup(t, "limit")
	c.Factory = func(context.Context, string, string) (Broker, string, error) {
		return &routedTestBroker{b}, "account", nil
	}
	v := approved(t, c, p)
	got, e := c.Execute(context.Background(), "alice", v.ID, v.ApprovalHash)
	if e != nil {
		t.Fatal(e)
	}
	if got.Orders[0].VenueClientID != "test-broker-"+got.Orders[0].ClientID {
		t.Fatal("route was not included in durable intent")
	}
	c.Shutdown()
	durable, e := c.Get("alice", v.ID)
	if e != nil || durable.Orders[0].VenueClientID != got.Orders[0].VenueClientID {
		t.Fatal("route lost across reload", e)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, o := range b.placed {
		if o.VenueClientID != got.Orders[0].VenueClientID {
			t.Fatal("submit did not use persisted route")
		}
	}
}

func TestPersistedExecutionFlagsCannotChangeLockedPlan(t *testing.T) {
	for _, flag := range []string{"post_only", "reduce_only"} {
		t.Run(flag, func(t *testing.T) {
			c, _, _, p := setup(t, "ladder")
			v := started(t, c, p)
			if flag == "post_only" {
				v.Orders[0].PostOnly = true
			} else {
				v.Orders[0].ReduceOnly = true
			}
			if err := validateIntent(v); err == nil {
				t.Fatal("changed execution flags escaped locked plan")
			}
		})
	}
}

func TestMarketIntentAndReceiptCannotBecomeRestingLimit(t *testing.T) {
	c, _, _, p := setup(t, "market")
	v := started(t, c, p)
	if err := validateIntent(v); err != nil {
		t.Fatal(err)
	}
	original := v.Orders[0]
	got := original
	got.OrderID = "123"
	got.Status = "FILLED"
	got.Filled = got.Quantity
	if !validReceipt(original, got) {
		t.Fatal("valid market fill rejected")
	}
	got.Type = ""
	if validReceipt(original, got) {
		t.Fatal("market intent accepted a limit receipt")
	}
	v.Orders[0].Type = ""
	if validateIntent(v) == nil {
		t.Fatal("persisted market intent became a resting limit")
	}
}
