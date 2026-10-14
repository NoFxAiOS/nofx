package trader

import (
	"errors"
	"math"
	"nofx/kernel"
	"nofx/market"
	"nofx/store"
	"nofx/trader/types"
	"path/filepath"
	"testing"
	"time"
)

func TestAudit05AdverseSlippageDirection(t *testing.T) {
	for _, c := range []struct {
		side       string
		fill, want float64
	}{{"long", 102, 200}, {"long", 98, 0}, {"short", 98, 200}, {"short", 102, 0}} {
		if got := adverseSlippageBps(100, c.fill, c.side); got != c.want {
			t.Errorf("side=%s fill=%.0f got=%.0fbps want=%.0fbps", c.side, c.fill, got, c.want)
		}
	}
}

func TestAudit05ShortBansMustReachMarketExecution(t *testing.T) {
	for _, code := range []string{"BTC_4H_STRONGBULL", "SHORT_TOP_CONFIRM_MISSING"} {
		at := riskTestTrader(store.RiskControlConfig{})
		at.cycleGateStates = map[string]*kernel.GateState{"XUSDT": {ShortAllowed: false, ShortFailed: []string{code}}}
		ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}}
		d := kernel.Decision{Symbol: "XUSDT", Action: "open_short", Price: 100, StopLoss: 105, TakeProfit: 90, PositionSizeUSD: 100, Confidence: 90}
		if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 0 {
			t.Errorf("market short passed ban %s", code)
		}
	}
}

func TestAudit05DailyUptrendBanMustCoverLimitShort(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{BlockShort1dUptrend: true})
	at.cycleGateStates = map[string]*kernel.GateState{"XUSDT": {ShortAllowed: true}}
	ctx := &kernel.Context{Account: kernel.AccountInfo{TotalEquity: 1000}, MarketDataMap: map[string]*market.Data{"XUSDT": uptrend1dData()}}
	if trend := kernel.TimeframeTrend(ctx.MarketDataMap["XUSDT"], "1d"); trend != "up" {
		t.Fatalf("fixture trend=%s", trend)
	}
	d := kernel.Decision{Symbol: "XUSDT", Action: "open_short_limit", Price: 100, StopLoss: 105, TakeProfit: 90, PositionSizeUSD: 100, Confidence: 90}
	if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 0 {
		t.Error("limit short bypassed enabled 1d uptrend ban")
	}
}

type audit05Mock struct {
	flat, flatOnClose, cancelTerminal bool
	closeErr                          error
	status                            map[string]interface{}
	Trader
	orders                           []types.OpenOrder
	mark                             float64
	cancelCalls, closeCalls, slCalls int
}

func (m *audit05Mock) GetBalance() (map[string]interface{}, error) {
	return map[string]interface{}{"totalEquity": 900.0}, nil
}
func (m *audit05Mock) GetPositions() ([]map[string]interface{}, error) {
	if m.flat {
		return nil, nil
	}
	return []map[string]interface{}{{"symbol": "XUSDT", "side": "long", "positionAmt": 1.0, "entryPrice": 100.0, "markPrice": m.mark, "leverage": 2.0}}, nil
}
func (m *audit05Mock) GetOpenOrders(string) ([]types.OpenOrder, error)    { return m.orders, nil }
func (m *audit05Mock) SetStopLoss(string, string, float64, float64) error { m.slCalls++; return nil }
func (m *audit05Mock) CloseLong(string, float64) (map[string]interface{}, error) {
	m.closeCalls++
	if m.closeErr != nil {
		return nil, m.closeErr
	}
	if m.flatOnClose {
		m.flat = true
	}
	return map[string]interface{}{}, nil
}
func (m *audit05Mock) CancelOrder(string, string) error {
	m.cancelCalls++
	if m.cancelTerminal && m.status != nil {
		m.status["status"] = "CANCELED"
	}
	return nil
}
func (m *audit05Mock) GetOrderStatus(string, string) (map[string]interface{}, error) {
	if m.status != nil {
		return m.status, nil
	}
	return map[string]interface{}{"status": "NEW", "executedQty": 0.0, "avgPrice": 0.0}, nil
}
func (m *audit05Mock) GetMarketPrice(string) (float64, error) {
	return 0, errors.New("no network in audit mock")
}

func TestAudit05WatchdogMustRepairInsufficientSLCoverage(t *testing.T) {
	m := &audit05Mock{mark: 100, orders: []types.OpenOrder{{Type: "STOP_MARKET", PositionSide: "LONG", Side: "SELL", StopPrice: 95, Quantity: 0.1}, {Type: "TAKE_PROFIT_MARKET", PositionSide: "LONG", Side: "SELL", StopPrice: 110, Quantity: 1}}}
	at := riskTestTrader(store.RiskControlConfig{})
	at.trader = m
	at.positionInitialStopLoss = map[string]float64{}
	at.processProtectionWatchdog()
	if m.slCalls == 0 {
		t.Error("1-unit position has only 0.1-unit stop, watchdog did not repair")
	}
}

func TestAudit05CrossedMissingStopMustExit(t *testing.T) {
	m := &audit05Mock{mark: 90, orders: []types.OpenOrder{{Type: "TAKE_PROFIT_MARKET", PositionSide: "LONG", StopPrice: 110, Quantity: 1}}}
	at := riskTestTrader(store.RiskControlConfig{})
	at.trader = m
	at.positionInitialStopLoss = map[string]float64{}
	at.SetRecordedStopLoss("XUSDT", "long", 95)
	at.processProtectionWatchdog()
	if m.closeCalls == 0 {
		t.Error("mark 90 crossed recorded long SL 95, watchdog neither restores a valid SL nor exits")
	}
}

func TestAudit05DailyHaltMustCancelPendingRisk(t *testing.T) {
	m := &audit05Mock{}
	at := riskTestTrader(store.RiskControlConfig{DailyMaxLossPct: 5})
	at.trader = m
	at.anchorDailyBaseline(1000)
	if msg := at.dailyLossHaltBlocks(at.config.StrategyConfig.RiskControl, 900); msg == "" {
		t.Fatal("fixture did not halt")
	}
	at.pendingEntries = map[string]*pendingEntry{"XUSDT|long": {Symbol: "XUSDT", Side: "long", Price: 100, Quantity: 1, StopLoss: 95, TakeProfit: 110, OrderID: "pending", PlacedAt: time.Now()}}
	at.processPendingEntries(true)
	if m.cancelCalls == 0 {
		t.Error("daily halt active, existing NEW entry remains live")
	}
}

func (m *audit05Mock) SetTakeProfit(string, string, float64, float64) error { return nil }

func TestFix05ActualFillMustNotMoveStructureOrHideRisk(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.5, RiskPerTradePct: 1.5})
	d := &kernel.Decision{Symbol: "XUSDT", Action: "open_long", StopLoss: 95, TakeProfit: 110}
	if err := at.actualFillRisk(d, 100, 1, 1000); err != nil {
		t.Fatal(err)
	}
	if err := at.actualFillRisk(d, 104, 1, 1000); err == nil {
		t.Fatal("adverse actual fill hid degraded net RR")
	}
	if d.StopLoss != 95 || d.TakeProfit != 110 {
		t.Fatal("structural levels shifted to disguise slippage")
	}
	if err := at.actualFillRisk(d, 100, 4, 1000); err == nil {
		t.Fatal("actual risk above budget passed")
	}
	if err := at.actualFillRisk(d, 94, 1, 1000); err == nil {
		t.Fatal("fill below original stop accepted")
	}
}

// Small drift past the budget (sizing spends 100% of it before the fill) is
// tolerated; anything beyond fillRiskBudgetTolerance is still rejected.
func TestActualFillRiskBudgetTolerance(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 1.5, RiskPerTradePct: 1.5})
	d := &kernel.Decision{Symbol: "XUSDT", Action: "open_long", StopLoss: 95, TakeProfit: 110}
	perUnit := 5 + 100*at.config.StrategyConfig.RiskControl.EffectiveEntryRoundTripCostBps()/10000
	full := 15 / perUnit // equity 1000 × 1.5%
	if err := at.actualFillRisk(d, 100, full*1.03, 1000); err != nil {
		t.Fatalf("3%% over budget rejected: %v", err)
	}
	if err := at.actualFillRisk(d, 100, full*1.07, 1000); err == nil {
		t.Fatal("7% over budget accepted")
	}
}

func TestFix05ActualFillDefaultsAndUnknownData(t *testing.T) {
	m := &review03Mock{}
	at := review03Trader(m)
	d := &kernel.Decision{Symbol: "XUSDT", Action: "open_long", StopLoss: 95, TakeProfit: 110}
	if err := at.actualFillRisk(d, 100, 4, 1000); err == nil {
		t.Fatal("unset risk budget bypassed the default 1.5% cap")
	}
	m.equity = 0
	if err := at.actualFillRisk(d, 100, 1, 0); err == nil {
		t.Fatal("unavailable equity passed actual fill risk")
	}
	d.StopLoss = math.NaN()
	if err := at.actualFillRisk(d, 100, 1, 1000); err == nil {
		t.Fatal("nonfinite protection passed actual fill risk")
	}
}

func TestFix05AccountCancelFaultClearsOnlyAfterReconciliation(t *testing.T) {
	m := &audit05Mock{flat: true}
	at := riskTestTrader(store.RiskControlConfig{})
	at.trader = m
	at.setPendingEntry(&pendingEntry{Symbol: "XUSDT", Side: "long", OrderID: "entry", PlacedAt: time.Now()})
	at.cancelAccountPendingRisk("halt")
	if at.protectionFaultReason() == "" || at.getPendingEntry("XUSDT", "long") == nil {
		t.Fatal("nonterminal cancellation did not keep the account blocked and recovery plan")
	}
	m.status = map[string]interface{}{"status": "CANCELED", "executedQty": 0.0}
	at.processPendingEntries(true)
	at.processPendingEntries(true)
	if at.protectionFaultReason() != "" || at.getPendingEntry("XUSDT", "long") != nil {
		t.Fatal("confirmed terminal cancellation left a permanent account fault")
	}
}

func TestFix05InvalidPartialFillMustPersistUntilExitConfirmed(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	m := &audit05Mock{mark: 100, flatOnClose: true, cancelTerminal: true, status: map[string]interface{}{"status": "PARTIALLY_FILLED", "executedQty": 0.4, "avgPrice": 100.0}}
	at := riskTestTrader(store.RiskControlConfig{MinRiskRewardRatio: 3})
	at.id = "fix05"
	at.store = st
	at.trader = m
	pe := &pendingEntry{Symbol: "XUSDT", Side: "long", Price: 100, StopLoss: 95, TakeProfit: 110, OrderID: "entry", PlacedAt: time.Now()}
	at.setPendingEntry(pe)
	at.protectExecutedSlice(pe, m.status)
	rows, err := st.PendingEntry().List(at.id)
	if err != nil || len(rows) != 1 || rows[0].RecoveryReason == "" {
		t.Fatalf("abort plan not durable: rows=%v err=%v", rows, err)
	}
	if m.closeCalls != 1 || m.cancelCalls == 0 || pe.ProtectedQty != 0 {
		t.Fatal("invalid partial fill was not cancelled/exited, or was marked protected")
	}
	at.processPendingEntries(true)
	if at.getPendingEntry("XUSDT", "long") != nil {
		t.Fatal("confirmed terminal and flat order not retired")
	}
}

func TestFix05StopRepairHasBoundedRetryAndBlocksNewRisk(t *testing.T) {
	m := &audit05Mock{mark: 100, orders: []types.OpenOrder{{Type: "TAKE_PROFIT_MARKET", PositionSide: "LONG", Side: "SELL", StopPrice: 110, Quantity: 1}}}
	at := riskTestTrader(store.RiskControlConfig{})
	at.trader = m
	at.positionInitialStopLoss = map[string]float64{}
	at.SetRecordedStopLoss("XUSDT", "long", 95)
	for i := 0; i < 3; i++ {
		at.processProtectionWatchdog()
	}
	if m.closeCalls == 0 || at.protectionFaultReason() == "" {
		t.Fatal("persistent stop failure neither halted account nor submitted exit")
	}
	if err := at.entryExecutionBlocked("YUSDT", "short"); err == nil {
		t.Fatal("unverified exit allowed new account risk")
	}
}

func TestFix05WrongSideOrEntryTriggerCannotCountAsProtection(t *testing.T) {
	for _, o := range []types.OpenOrder{
		{Type: "STOP_MARKET", PositionSide: "SHORT", Side: "BUY", Quantity: 1, StopPrice: 95},
		{Type: "STOP_MARKET", PositionSide: "BOTH", Side: "SELL", Quantity: 1, StopPrice: 95},
		{Type: "STOP_MARKET", PositionSide: "LONG", Side: "BUY", Quantity: 1, StopPrice: 95},
		{Type: "STOP_MARKET", PositionSide: "LONG", Side: "SELL", Quantity: 1, StopPrice: 95, Status: "CANCELED"},
	} {
		if enoughProtection([]types.OpenOrder{o}, "LONG", "SL", 95, 1) {
			t.Fatalf("incorrect leg credited: %+v", o)
		}
	}
	entry := types.OpenOrder{Symbol: "XUSDT", Type: "STOP_MARKET", PositionSide: "BOTH", Side: "BUY", Quantity: 1, StopPrice: 105}
	if orphanProtectiveOrder(entry, map[string]bool{}) {
		t.Fatal("manual opening trigger was classified as orphaned protection")
	}
	leg := types.OpenOrder{OrderID: "same", Symbol: "XUSDT", Type: "STOP_MARKET", PositionSide: "LONG", Side: "SELL", Quantity: 0.5, StopPrice: 95}
	if enoughProtection([]types.OpenOrder{leg, leg}, "LONG", "SL", 95, 1) {
		t.Fatal("duplicate API rows counted twice toward coverage")
	}
}

func TestFix05ExitFaultSurvivesCoveredPositionUntilFlat(t *testing.T) {
	m := &audit05Mock{mark: 100, orders: []types.OpenOrder{{Type: "STOP_MARKET", PositionSide: "LONG", Side: "SELL", StopPrice: 95, Quantity: 1}, {Type: "TAKE_PROFIT_MARKET", PositionSide: "LONG", Side: "SELL", StopPrice: 110, Quantity: 1}}}
	at := riskTestTrader(store.RiskControlConfig{})
	at.trader = m
	at.positionInitialStopLoss = map[string]float64{}
	at.setProtectionFault("recovery:XUSDT_long", "invalid fill exit not confirmed")
	at.processProtectionWatchdog()
	if at.protectionFaultReason() == "" {
		t.Fatal("valid coverage erased an outstanding emergency exit")
	}
	m.flat = true
	at.processProtectionWatchdog()
	if at.protectionFaultReason() != "" {
		t.Fatal("fresh flat snapshot did not clear exit fault")
	}
}

type fix05ComputedMock struct {
	review03Mock
	failTP bool
	empty  bool
}

func (m *fix05ComputedMock) GetPositions() ([]map[string]interface{}, error) {
	if m.empty {
		return nil, nil
	}
	return m.review03Mock.GetPositions()
}
func (m *fix05ComputedMock) SetTakeProfit(s, p string, q, price float64) error {
	if m.failTP {
		return errors.New("simulated TP failure")
	}
	return m.review03Mock.SetTakeProfit(s, p, q, price)
}
func TestFix05ComputedSLCommitSurvivesTPFailureAndUsesRealQuantity(t *testing.T) {
	m := &fix05ComputedMock{failTP: true}
	at := review03Trader(&m.review03Mock)
	at.trader = m
	if at.reconcileComputedProtection("XUSDT", "long", 95, 110) {
		t.Fatal("TP failure reported complete recovery")
	}
	if at.GetRecordedStopLoss("XUSDT", "long") != 95 || at.GetInitialStopLoss("XUSDT", "long") != 95 || len(m.orders) != 1 || m.orders[0].Quantity != 1 {
		t.Fatalf("SL plan or actual size lost: %+v", m.orders)
	}
	m.failTP = false
	if !at.reconcileComputedProtection("XUSDT", "long", 95, 110) || m.slCalls != 1 {
		t.Fatal("retry duplicated verified SL or failed to complete TP")
	}
	m.empty = true
	if at.reconcileComputedProtection("XUSDT", "long", 95, 110) {
		t.Fatal("missing position allowed a zero-quantity protection order")
	}
}
