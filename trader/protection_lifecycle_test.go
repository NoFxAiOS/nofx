package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
	"nofx/trader/testutil"
)

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
