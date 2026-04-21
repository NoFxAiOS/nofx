package trader

import (
	"fmt"
	"nofx/store"
	tradertypes "nofx/trader/types"
	"strings"
	"testing"
	"time"
)

type fakeProtectionTrader struct {
	cancelStopLossCalls int
	setStopLossCalls    int
	lastSymbol          string
	lastPositionSide    string
	lastQuantity        float64
	lastStopPrice       float64
	cancelErr           error
	setStopLossErr      error
	trailingCalls       int
	cancelTrailingCalls int
	trailingSymbol      string
	trailingSide        string
	trailingActivation  float64
	trailingCallback    float64
	openOrders          []tradertypes.OpenOrder
	positions           []map[string]interface{}
	closeLongCalls      int
	closeShortCalls     int
	closeLongQtys       []float64
	closeShortQtys      []float64
}

func (f *fakeProtectionTrader) GetBalance() (map[string]interface{}, error) { return nil, nil }
func (f *fakeProtectionTrader) GetPositions() ([]map[string]interface{}, error) {
	if f.positions != nil {
		return f.positions, nil
	}
	return nil, nil
}
func (f *fakeProtectionTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	f.closeLongCalls++
	f.closeLongQtys = append(f.closeLongQtys, quantity)
	return map[string]interface{}{"orderId": fmt.Sprintf("close-long-%d", f.closeLongCalls)}, nil
}
func (f *fakeProtectionTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	f.closeShortCalls++
	f.closeShortQtys = append(f.closeShortQtys, quantity)
	return map[string]interface{}{"orderId": fmt.Sprintf("close-short-%d", f.closeShortCalls)}, nil
}
func (f *fakeProtectionTrader) SetLeverage(symbol string, leverage int) error         { return nil }
func (f *fakeProtectionTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (f *fakeProtectionTrader) GetMarketPrice(symbol string) (float64, error)         { return 0, nil }
func (f *fakeProtectionTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	f.setStopLossCalls++
	f.lastSymbol = symbol
	f.lastPositionSide = positionSide
	f.lastQuantity = quantity
	f.lastStopPrice = stopPrice
	if f.setStopLossErr != nil {
		return f.setStopLossErr
	}
	f.openOrders = append(f.openOrders, tradertypes.OpenOrder{
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "STOP_MARKET",
		StopPrice:    stopPrice,
		Quantity:     quantity,
	})
	return nil
}
func (f *fakeProtectionTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}
func (f *fakeProtectionTrader) CancelStopLossOrders(symbol string) error {
	f.cancelStopLossCalls++
	f.openOrders = nil
	return f.cancelErr
}
func (f *fakeProtectionTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (f *fakeProtectionTrader) CancelAllOrders(symbol string) error        { return nil }
func (f *fakeProtectionTrader) CancelStopOrders(symbol string) error       { return nil }
func (f *fakeProtectionTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}
func (f *fakeProtectionTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	return nil, nil
}
func (f *fakeProtectionTrader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	return f.openOrders, nil
}
func (f *fakeProtectionTrader) SetTrailingStopLoss(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64) error {
	f.trailingCalls++
	f.trailingSymbol = symbol
	f.trailingSide = positionSide
	f.trailingActivation = activationPrice
	f.trailingCallback = callbackRate
	f.openOrders = append(f.openOrders, tradertypes.OpenOrder{
		OrderID:      fmt.Sprintf("new-tier-%d", f.trailingCalls),
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "TRAILING_STOP_MARKET",
		StopPrice:    activationPrice,
		CallbackRate: callbackRate,
		Quantity:     quantity,
		Status:       "NEW",
	})
	return nil
}
func (f *fakeProtectionTrader) SetTrailingStopLossTagged(symbol string, positionSide string, activationPrice float64, callbackRate float64, quantity float64, reasonTag string) error {
	return f.SetTrailingStopLoss(symbol, positionSide, activationPrice, callbackRate, quantity)
}
func (f *fakeProtectionTrader) CancelTrailingStopOrders(symbol string) error {
	removed := 0
	filtered := make([]tradertypes.OpenOrder, 0, len(f.openOrders))
	for _, order := range f.openOrders {
		if strings.Contains(strings.ToUpper(order.Type), "TRAILING") {
			removed++
			continue
		}
		filtered = append(filtered, order)
	}
	f.cancelTrailingCalls += removed
	f.openOrders = filtered
	return nil
}
func (f *fakeProtectionTrader) CancelTrailingStopOrdersByIDs(symbol string, orderIDs []string) error {
	f.cancelTrailingCalls += len(orderIDs)
	filtered := make([]tradertypes.OpenOrder, 0, len(f.openOrders))
	set := make(map[string]struct{}, len(orderIDs))
	for _, id := range orderIDs {
		set[id] = struct{}{}
	}
	for _, order := range f.openOrders {
		if _, ok := set[order.OrderID]; ok {
			continue
		}
		filtered = append(filtered, order)
	}
	f.openOrders = filtered
	return nil
}

func TestCheckPositionDrawdownSkipsDuplicateManagedPartialCloseForSameRule(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "ADAUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   101.0,
			"positionAmt": 170.0,
		}},
	}
	at := &AutoTrader{
		exchange: "paper",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					DrawdownTakeProfit: store.DrawdownTakeProfitConfig{Enabled: true, Rules: []store.DrawdownTakeProfitRule{{MinProfitPct: 0.7, MaxDrawdownPct: 55, CloseRatioPct: 70}}},
				},
			},
		},
		protectionState: make(map[string]string),
		drawdownState:   make(map[string]string),
		peakPnLCache:    map[string]float64{"ADAUSDT_long": 3.0},
	}

	at.checkPositionDrawdown()
	if fake.closeLongCalls != 1 {
		t.Fatalf("expected first drawdown pass to close once, got %d", fake.closeLongCalls)
	}
	if len(fake.closeLongQtys) != 1 || fake.closeLongQtys[0] != 119 {
		t.Fatalf("expected close qty 119 on first pass, got %+v", fake.closeLongQtys)
	}

	at.checkPositionDrawdown()
	if fake.closeLongCalls != 1 {
		t.Fatalf("expected duplicate drawdown pass to be skipped, got %d closes", fake.closeLongCalls)
	}
}

func TestCheckPositionDrawdownAllowsNextCloseAfterPositionFingerprintChanges(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "ADAUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   101.0,
			"positionAmt": 170.0,
		}},
	}
	at := &AutoTrader{
		exchange: "paper",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					DrawdownTakeProfit: store.DrawdownTakeProfitConfig{Enabled: true, Rules: []store.DrawdownTakeProfitRule{{MinProfitPct: 0.7, MaxDrawdownPct: 55, CloseRatioPct: 70}}},
				},
			},
		},
		protectionState: make(map[string]string),
		drawdownState:   make(map[string]string),
		peakPnLCache:    map[string]float64{"ADAUSDT_long": 3.0},
	}

	at.checkPositionDrawdown()
	fake.positions = []map[string]interface{}{{
		"symbol":      "ADAUSDT",
		"side":        "long",
		"entryPrice":  100.0,
		"markPrice":   101.0,
		"positionAmt": 51.0,
	}}
	at.checkPositionDrawdown()
	if fake.closeLongCalls != 2 {
		t.Fatalf("expected second close after quantity changed, got %d", fake.closeLongCalls)
	}
	if len(fake.closeLongQtys) != 2 || fake.closeLongQtys[1] != 35.7 {
		t.Fatalf("expected second close qty 35.7 after quantity changed, got %+v", fake.closeLongQtys)
	}
}

func TestApplyNativeTrailingDrawdownForBinance(t *testing.T) {
	fake := &fakeProtectionTrader{}
	at := &AutoTrader{
		exchange: "binance",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
		protectionState: make(map[string]string),
	}

	rule := store.DrawdownTakeProfitRule{
		MinProfitPct:   5,
		MaxDrawdownPct: 2,
		CloseRatioPct:  100,
	}

	ok := at.applyNativeTrailingDrawdown("BTCUSDT", "long", 100, rule)
	if !ok {
		t.Fatal("expected native trailing drawdown to be applied")
	}
	if fake.trailingCalls != 1 {
		t.Fatalf("expected 1 trailing call, got %d", fake.trailingCalls)
	}
	if fake.trailingActivation <= 100 {
		t.Fatalf("expected activation above entry for long, got %.4f", fake.trailingActivation)
	}
	if fake.trailingCallback != 0.1 {
		t.Fatalf("expected callback rate 0.1, got %.4f", fake.trailingCallback)
	}
	if at.getProtectionState("BTCUSDT", "long") != "native_trailing_armed" {
		t.Fatalf("expected protection state native_trailing_armed, got %q", at.getProtectionState("BTCUSDT", "long"))
	}
}

func TestCheckPositionDrawdownActivatesRunnerAndSuppressesBreakEvenAfterPartialClose(t *testing.T) {
	fake := &fakeProtectionTrader{
		positions: []map[string]interface{}{{
			"symbol":      "TAOUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"markPrice":   101.0,
			"positionAmt": 10.0,
		}},
	}
	at := &AutoTrader{
		exchange: "paper",
		trader:   fake,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					DrawdownTakeProfit: store.DrawdownTakeProfitConfig{Enabled: true, Mode: store.ProtectionModeAI, EngineMode: store.DrawdownEngineModeAI, RunnerEnabled: true, MinRunnerKeepPct: 20, MaxFirstReducePct: 60, BreakEvenRunnerPolicy: store.DrawdownBreakEvenRunnerFallbackOnly, Rules: []store.DrawdownTakeProfitRule{{
						MinProfitPct:     0.7,
						MaxDrawdownPct:   55,
						CloseRatioPct:    70,
						StageName:        "lock_first_profit",
						RunnerKeepPct:    30,
						RunnerStopMode:   "structure",
						RunnerStopSource: "adjacent_support_flip",
					}}},
					BreakEvenStop: store.BreakEvenStopConfig{Enabled: true, TriggerMode: store.BreakEvenTriggerProfitPct, TriggerValue: 0.5},
				},
			},
		},
		protectionState:       make(map[string]string),
		breakEvenState:        make(map[string]string),
		breakEvenFingerprints: make(map[string]string),
		drawdownState:         make(map[string]string),
		drawdownRunnerState:   make(map[string]DrawdownRunnerState),
		peakPnLCache:          map[string]float64{"TAOUSDT_long": 3.0},
	}

	at.checkPositionDrawdown()
	if fake.closeLongCalls != 1 {
		t.Fatalf("expected partial drawdown close, got %d", fake.closeLongCalls)
	}
	if len(fake.closeLongQtys) != 1 || fake.closeLongQtys[0] != 6.0 {
		t.Fatalf("expected capped first reduce qty 6.0, got %+v", fake.closeLongQtys)
	}
	if fake.setStopLossCalls != 0 {
		t.Fatalf("expected no break-even placement on unsupported paper exchange, got %d", fake.setStopLossCalls)
	}
	if !at.isBreakEvenSuppressedByRunner("TAOUSDT", "long") {
		t.Fatal("expected runner to suppress subsequent break-even")
	}
	state := at.getDrawdownRunnerState("TAOUSDT", "long")
	if state == nil {
		t.Fatal("expected runner state recorded")
	}
	if state.StageName != "lock_first_profit" {
		t.Fatalf("expected stage name preserved from configured rule, got %q", state.StageName)
	}
	if state.RunnerKeepPct != 40 {
		t.Fatalf("expected runner keep pct 40 after cap, got %.2f", state.RunnerKeepPct)
	}
	if state.RunnerStopSource != "adjacent_support_flip" {
		t.Fatalf("expected runner stop source preserved, got %q", state.RunnerStopSource)
	}

	fake.positions[0]["positionAmt"] = 4.0
	at.checkPositionDrawdown()
	if fake.setStopLossCalls != 0 {
		t.Fatalf("expected no break-even apply after runner activation, got %d", fake.setStopLossCalls)
	}
}

func TestEvaluateAIDrawdownRuleRespectsRunnerPolicyAndStageDefaults(t *testing.T) {
	cfg := store.DrawdownTakeProfitConfig{
		Enabled:               true,
		Mode:                  store.ProtectionModeAI,
		EngineMode:            store.DrawdownEngineModeAI,
		RunnerEnabled:         true,
		MinRunnerKeepPct:      25,
		MaxFirstReducePct:     60,
		BreakEvenRunnerPolicy: store.DrawdownBreakEvenRunnerFallbackOnly,
	}
	rules := []store.DrawdownTakeProfitRule{{
		MinProfitPct:   4,
		MaxDrawdownPct: 20,
		CloseRatioPct:  80,
	}}

	eval := evaluateAIDrawdownRule(cfg, 4.2, 6.0, 30.0, rules, nil, "long", 0)
	if eval == nil {
		t.Fatal("expected ai drawdown evaluation")
	}
	if eval.Rule.StageName != "near_primary_target" {
		t.Fatalf("expected near_primary_target stage, got %q", eval.Rule.StageName)
	}
	if eval.Rule.CloseRatioPct != 60 {
		t.Fatalf("expected first reduce capped at 60, got %.2f", eval.Rule.CloseRatioPct)
	}
	if eval.Rule.RunnerKeepPct != 40 {
		t.Fatalf("expected runner keep 40 after cap, got %.2f", eval.Rule.RunnerKeepPct)
	}
	if eval.Rule.RunnerStopMode != "structure" {
		t.Fatalf("expected structure runner stop in fallback_only policy, got %q", eval.Rule.RunnerStopMode)
	}
	if eval.Rule.RunnerStopSource != "primary_target_pullback" {
		t.Fatalf("expected default stop source for near_primary_target, got %q", eval.Rule.RunnerStopSource)
	}
	if eval.Rule.RunnerTargetSource != "primary_resistance" {
		t.Fatalf("expected default target source for near_primary_target, got %q", eval.Rule.RunnerTargetSource)
	}
}

func TestEvaluateAIDrawdownRuleUsesStructureContextForStageAndSources(t *testing.T) {
	cfg := store.DrawdownTakeProfitConfig{
		Enabled:               true,
		Mode:                  store.ProtectionModeAI,
		EngineMode:            store.DrawdownEngineModeAI,
		RunnerEnabled:         true,
		MinRunnerKeepPct:      20,
		MaxFirstReducePct:     60,
		BreakEvenRunnerPolicy: store.DrawdownBreakEvenRunnerFallbackOnly,
	}
	rules := []store.DrawdownTakeProfitRule{{
		MinProfitPct:   4,
		MaxDrawdownPct: 20,
		CloseRatioPct:  70,
	}}
	structure := &drawdownStructureContext{
		Entry:       100,
		FirstTarget: 110,
		Resistance:  []float64{110},
		FibLevels:   []float64{111.8},
		Anchors: []store.DecisionActionReasonAnchor{{
			Type:      "first_target",
			Timeframe: "15m",
			Price:     110,
			Reason:    "primary resistance objective",
		}},
	}

	eval := evaluateAIDrawdownRule(cfg, 9.0, 11.0, 25.0, rules, structure, "long", 111.8)
	if eval == nil {
		t.Fatal("expected ai drawdown evaluation")
	}
	if eval.Rule.StageName != "extension_exhaustion" {
		t.Fatalf("expected structure-driven extension_exhaustion stage, got %q", eval.Rule.StageName)
	}
	if eval.Rule.RunnerStopSource != "extension_swing_trail" {
		t.Fatalf("expected extension stop source, got %q", eval.Rule.RunnerStopSource)
	}
	if eval.Rule.RunnerTargetSource != "extension_fibonacci" {
		t.Fatalf("expected extension fibonacci target source, got %q", eval.Rule.RunnerTargetSource)
	}
}
