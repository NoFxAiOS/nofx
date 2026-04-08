package trader

import (
	"errors"
	"math"
	"testing"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
	"nofx/trader/testutil"
)

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

// --- Regime filter tests ---

func TestAllowDecisionByRegimeBlocksHighFunding(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.RegimeFilter = store.RegimeFilterConfig{
		Enabled:           true,
		AllowedRegimes:    []string{"standard", "wide", "narrow", "trending"},
		BlockHighFunding:  true,
		MaxFundingRateAbs: 0.01,
	}

	decision := &kernel.Decision{Symbol: "BTCUSDT", Action: "open_long"}
	data := &market.Data{FundingRate: 0.02}
	if at.allowDecisionByRegime(decision, data) {
		t.Fatal("expected decision to be blocked by funding filter")
	}
}

func TestAllowDecisionByRegimeBlocksTrendMismatch(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.RegimeFilter = store.RegimeFilterConfig{
		Enabled:               true,
		AllowedRegimes:        []string{"standard", "wide", "narrow", "trending"},
		RequireTrendAlignment: true,
	}

	decision := &kernel.Decision{Symbol: "BTCUSDT", Action: "open_long"}
	data := &market.Data{CurrentPrice: 90, CurrentEMA20: 100, PriceChange4h: -2}
	if at.allowDecisionByRegime(decision, data) {
		t.Fatal("expected decision to be blocked by trend alignment")
	}
}

func TestAllowDecisionByRegimeAllowsAlignedLong(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.RegimeFilter = store.RegimeFilterConfig{
		Enabled:               true,
		AllowedRegimes:        []string{"narrow", "standard", "trending"},
		RequireTrendAlignment: true,
	}

	decision := &kernel.Decision{Symbol: "BTCUSDT", Action: "open_long"}
	data := &market.Data{CurrentPrice: 105, CurrentEMA20: 100, PriceChange4h: 2}
	if !at.allowDecisionByRegime(decision, data) {
		t.Fatal("expected aligned long to be allowed")
	}
}

func TestAllowDecisionByRegimeAllowsAlignedShort(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.RegimeFilter = store.RegimeFilterConfig{
		Enabled:               true,
		AllowedRegimes:        []string{"narrow", "standard", "trending"},
		RequireTrendAlignment: true,
	}

	decision := &kernel.Decision{Symbol: "ETHUSDT", Action: "open_short"}
	data := &market.Data{CurrentPrice: 95, CurrentEMA20: 100, PriceChange4h: -3}
	if !at.allowDecisionByRegime(decision, data) {
		t.Fatal("expected aligned short to be allowed")
	}
}

func TestAllowDecisionByRegimePassesCloseActionsTrendCheck(t *testing.T) {
	// Close actions should not be blocked by trend alignment (isTrendAligned returns true for close)
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.RegimeFilter = store.RegimeFilterConfig{
		Enabled:               true,
		AllowedRegimes:        []string{"narrow", "standard", "trending"},
		RequireTrendAlignment: true,
	}

	for _, action := range []string{"close_long", "close_short"} {
		decision := &kernel.Decision{Symbol: "BTCUSDT", Action: action}
		// Trend is misaligned for opens, but close should still pass
		data := &market.Data{CurrentPrice: 50, CurrentEMA20: 100, PriceChange4h: -20}
		if !at.allowDecisionByRegime(decision, data) {
			t.Fatalf("expected %s to pass trend alignment check", action)
		}
	}
}

// --- Protection lifecycle tests ---

func TestFakeTraderProtectionLifecycle(t *testing.T) {
	fake := testutil.NewFakeTrader()
	at := &AutoTrader{trader: fake, exchange: "binance"}
	plan := &ProtectionPlan{
		Mode:            "manual",
		NeedsStopLoss:   true,
		NeedsTakeProfit: true,
		StopLossPrice:   98,
		TakeProfitPrice: 105,
	}

	if err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 1, plan); err != nil {
		t.Fatalf("expected fake trader protection lifecycle success, got %v", err)
	}
	orders, err := fake.GetOpenOrders("BTCUSDT")
	if err != nil {
		t.Fatalf("expected open orders, got %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected 2 protection orders, got %d", len(orders))
	}
}

func TestProtectionVerifyFailureTriggersError(t *testing.T) {
	fake := testutil.NewFakeTrader()
	fake.GetOpenOrdersErr = errors.New("exchange unavailable")
	at := &AutoTrader{trader: fake, exchange: "binance"}
	plan := &ProtectionPlan{
		Mode:            "manual",
		NeedsStopLoss:   true,
		NeedsTakeProfit: true,
		StopLossPrice:   98,
		TakeProfitPrice: 105,
	}

	err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 1, plan)
	if err == nil {
		t.Fatal("expected error when open orders verification fails")
	}
}

func TestProtectionStopLossSetupFailure(t *testing.T) {
	fake := testutil.NewFakeTrader()
	fake.SetStopLossErr = errors.New("stop loss rejected")
	at := &AutoTrader{trader: fake, exchange: "binance"}
	plan := &ProtectionPlan{
		Mode:          "manual",
		NeedsStopLoss: true,
		StopLossPrice: 98,
	}

	err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 1, plan)
	if err == nil {
		t.Fatal("expected error when stop loss setup fails")
	}
}

func TestProtectionTakeProfitSetupFailure(t *testing.T) {
	fake := testutil.NewFakeTrader()
	fake.SetTakeProfitErr = errors.New("take profit rejected")
	at := &AutoTrader{trader: fake, exchange: "binance"}
	plan := &ProtectionPlan{
		Mode:            "manual",
		NeedsTakeProfit: true,
		TakeProfitPrice: 105,
	}

	err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 1, plan)
	if err == nil {
		t.Fatal("expected error when take profit setup fails")
	}
}

func TestProtectionRetryRecovery(t *testing.T) {
	fake := testutil.NewFakeTrader()
	callCount := 0
	originalSetSL := fake.SetStopLossErr
	_ = originalSetSL

	// Simulate first attempt fails, second succeeds
	fake.SetStopLossErr = errors.New("transient failure")
	at := &AutoTrader{trader: fake, exchange: "binance"}
	plan := &ProtectionPlan{
		Mode:          "manual",
		NeedsStopLoss: true,
		StopLossPrice: 98,
	}

	// First call should fail
	err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 1, plan)
	if err == nil {
		t.Fatal("expected first attempt to fail")
	}
	callCount++

	// Clear error for retry
	fake.SetStopLossErr = nil
	err = at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 1, plan)
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	callCount++

	if callCount != 2 {
		t.Fatalf("expected 2 attempts, got %d", callCount)
	}
}

func TestLadderProtectionLifecycle(t *testing.T) {
	fake := testutil.NewFakeTrader()
	at := &AutoTrader{trader: fake, exchange: "binance"}
	plan := &ProtectionPlan{
		Mode:                 "manual",
		NeedsStopLoss:        true,
		NeedsTakeProfit:      true,
		RequiresPartialClose: true,
		StopLossOrders: []ProtectionOrder{
			{Price: 95, CloseRatioPct: 50},
			{Price: 90, CloseRatioPct: 50},
		},
		TakeProfitOrders: []ProtectionOrder{
			{Price: 110, CloseRatioPct: 30},
			{Price: 120, CloseRatioPct: 70},
		},
	}

	if err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 2, plan); err != nil {
		t.Fatalf("expected ladder protection lifecycle success, got %v", err)
	}
	orders, err := fake.GetOpenOrders("BTCUSDT")
	if err != nil {
		t.Fatalf("expected open orders, got %v", err)
	}
	// 2 SL + 2 TP = 4 orders
	if len(orders) != 4 {
		t.Fatalf("expected 4 ladder protection orders, got %d", len(orders))
	}

	// Verify order types
	slCount, tpCount := 0, 0
	for _, o := range orders {
		if o.Type == "STOP_MARKET" {
			slCount++
		}
		if o.Type == "TAKE_PROFIT_MARKET" {
			tpCount++
		}
	}
	if slCount != 2 {
		t.Fatalf("expected 2 stop loss orders, got %d", slCount)
	}
	if tpCount != 2 {
		t.Fatalf("expected 2 take profit orders, got %d", tpCount)
	}
}

func TestLadderProtectionPartialCloseQuantities(t *testing.T) {
	fake := testutil.NewFakeTrader()
	at := &AutoTrader{trader: fake, exchange: "binance"}
	plan := &ProtectionPlan{
		Mode:                 "manual",
		NeedsStopLoss:        true,
		NeedsTakeProfit:      true,
		RequiresPartialClose: true,
		StopLossOrders: []ProtectionOrder{
			{Price: 95, CloseRatioPct: 40},
			{Price: 90, CloseRatioPct: 60},
		},
		TakeProfitOrders: []ProtectionOrder{
			{Price: 110, CloseRatioPct: 50},
			{Price: 120, CloseRatioPct: 50},
		},
	}

	totalQty := 10.0
	if err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", totalQty, plan); err != nil {
		t.Fatalf("expected ladder protection success, got %v", err)
	}
	orders, _ := fake.GetOpenOrders("BTCUSDT")

	// Verify quantities match ratios
	for _, o := range orders {
		if o.Type == "STOP_MARKET" && o.StopPrice == 95 {
			expected := totalQty * 40 / 100
			if o.Quantity != expected {
				t.Fatalf("expected SL@95 qty %.2f, got %.2f", expected, o.Quantity)
			}
		}
		if o.Type == "STOP_MARKET" && o.StopPrice == 90 {
			expected := totalQty * 60 / 100
			if o.Quantity != expected {
				t.Fatalf("expected SL@90 qty %.2f, got %.2f", expected, o.Quantity)
			}
		}
		if o.Type == "TAKE_PROFIT_MARKET" && o.StopPrice == 110 {
			expected := totalQty * 50 / 100
			if o.Quantity != expected {
				t.Fatalf("expected TP@110 qty %.2f, got %.2f", expected, o.Quantity)
			}
		}
		if o.Type == "TAKE_PROFIT_MARKET" && o.StopPrice == 120 {
			expected := totalQty * 50 / 100
			if o.Quantity != expected {
				t.Fatalf("expected TP@120 qty %.2f, got %.2f", expected, o.Quantity)
			}
		}
	}
}

func TestProtectionNilPlan(t *testing.T) {
	fake := testutil.NewFakeTrader()
	at := &AutoTrader{trader: fake, exchange: "binance"}
	if err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 1, nil); err != nil {
		t.Fatalf("expected nil plan to be no-op, got %v", err)
	}
}

func TestProtectionNoStopNoTP(t *testing.T) {
	fake := testutil.NewFakeTrader()
	at := &AutoTrader{trader: fake, exchange: "binance"}
	plan := &ProtectionPlan{Mode: "manual"}
	if err := at.placeAndVerifyProtectionPlan("BTCUSDT", "LONG", 1, plan); err != nil {
		t.Fatalf("expected no-op plan to succeed, got %v", err)
	}
	orders, _ := fake.GetOpenOrders("BTCUSDT")
	if len(orders) != 0 {
		t.Fatalf("expected 0 orders for no-op plan, got %d", len(orders))
	}
}

// --- Protection plan builder tests ---

func TestBuildManualFullProtectionPlanLong(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeManual,
		StopLoss: store.ProtectionThresholdRule{
			Enabled:      true,
			PriceMovePct: 5,
		},
		TakeProfit: store.ProtectionThresholdRule{
			Enabled:      true,
			PriceMovePct: 10,
		},
	}

	plan, err := at.BuildManualProtectionPlan(100, "BTCUSDT", "open_long")
	if err != nil {
		t.Fatalf("expected plan build success, got %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if !plan.NeedsStopLoss || !plan.NeedsTakeProfit {
		t.Fatal("expected both SL and TP")
	}
	if !approxEqual(plan.StopLossPrice, 95) {
		t.Fatalf("expected SL at 95, got %.2f", plan.StopLossPrice)
	}
	if !approxEqual(plan.TakeProfitPrice, 110) {
		t.Fatalf("expected TP at 110, got %.2f", plan.TakeProfitPrice)
	}
}

func TestBuildManualFullProtectionPlanShort(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeManual,
		StopLoss: store.ProtectionThresholdRule{
			Enabled:      true,
			PriceMovePct: 5,
		},
		TakeProfit: store.ProtectionThresholdRule{
			Enabled:      true,
			PriceMovePct: 10,
		},
	}

	plan, err := at.BuildManualProtectionPlan(100, "BTCUSDT", "open_short")
	if err != nil {
		t.Fatalf("expected plan build success, got %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if !approxEqual(plan.StopLossPrice, 105) {
		t.Fatalf("expected short SL at 105, got %.2f", plan.StopLossPrice)
	}
	if !approxEqual(plan.TakeProfitPrice, 90) {
		t.Fatalf("expected short TP at 90, got %.2f", plan.TakeProfitPrice)
	}
}

func TestBuildManualProtectionPlanDisabled(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled: false,
	}

	plan, err := at.BuildManualProtectionPlan(100, "BTCUSDT", "open_long")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan != nil {
		t.Fatal("expected nil plan when protection disabled")
	}
}

func TestBuildManualProtectionPlanInvalidEntry(t *testing.T) {
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{}}}
	at.config.StrategyConfig.Protection.FullTPSL = store.FullTPSLConfig{
		Enabled: true,
		Mode:    store.ProtectionModeManual,
		StopLoss: store.ProtectionThresholdRule{
			Enabled:      true,
			PriceMovePct: 5,
		},
	}

	_, err := at.BuildManualProtectionPlan(0, "BTCUSDT", "open_long")
	if err == nil {
		t.Fatal("expected error for zero entry price")
	}
}
