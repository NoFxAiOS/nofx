package trader

import (
	"nofx/kernel"
	"nofx/store"
	"testing"
)

func TestApplyAutopilotFullSizeOpenEnforcesFiveTimesEquityHardCap(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.CoinSource.SourceType = "vergex_signal"
	cfg.RiskControl.MaxPositions = 1
	cfg.RiskControl.BTCETHMaxLeverage = 10
	cfg.RiskControl.AltcoinMaxLeverage = 10
	cfg.RiskControl.BTCETHMaxPositionValueRatio = 10
	cfg.RiskControl.AltcoinMaxPositionValueRatio = 10

	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &cfg}}
	decision := &kernel.Decision{
		Symbol:          "xyz:INTC",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 12,
	}

	at.applyAutopilotFullSizeOpen(decision, 100)

	if decision.Leverage != 10 {
		t.Fatalf("expected leverage to be forced to 10x, got %dx", decision.Leverage)
	}
	if decision.PositionSizeUSD != 500 {
		t.Fatalf("expected position size to be capped at 5x equity, got %.2f", decision.PositionSizeUSD)
	}
}

func TestDefaultAutopilotFullSizeOpenUsesEightPositionAllocation(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.CoinSource.SourceType = "vergex_signal"

	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &cfg}}
	decision := &kernel.Decision{
		Symbol:          "xyz:INTC",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 12,
	}

	at.applyAutopilotFullSizeOpen(decision, 100)

	if cfg.RiskControl.MaxPositions != 8 {
		t.Fatalf("expected eight default positions, got %d", cfg.RiskControl.MaxPositions)
	}
	if cfg.RiskControl.BTCETHMaxPositionValueRatio != 5 || cfg.RiskControl.AltcoinMaxPositionValueRatio != 5 {
		t.Fatalf("expected a fixed 5x per-position hard cap, got %+v", cfg.RiskControl)
	}
	if decision.Leverage != 10 {
		t.Fatalf("expected default leverage to remain 10x, got %dx", decision.Leverage)
	}
	if decision.PositionSizeUSD != 120 {
		t.Fatalf("expected each default slot to target 1.2x equity notional, got %.2f", decision.PositionSizeUSD)
	}
}

func TestApplyAutopilotFullSizeOpenSkipsNonClaw402Strategies(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.CoinSource.SourceType = "static"
	cfg.RiskControl.BTCETHMaxLeverage = 10
	cfg.RiskControl.AltcoinMaxLeverage = 10

	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &cfg}}
	decision := &kernel.Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 12,
	}

	at.applyAutopilotFullSizeOpen(decision, 29.8)

	if decision.Leverage != 3 || decision.PositionSizeUSD != 12 {
		t.Fatalf("non-Claw402 strategies should not be rewritten, got leverage=%d size=%.2f", decision.Leverage, decision.PositionSizeUSD)
	}
}

func TestEnforcePositionValueRatioCapsAutopilotAtFiveTimesEquity(t *testing.T) {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.RiskControl.BTCETHMaxPositionValueRatio = 10
	cfg.RiskControl.AltcoinMaxPositionValueRatio = 10
	at := &AutoTrader{config: AutoTraderConfig{StrategyConfig: &cfg}}

	adjusted, capped := at.enforcePositionValueRatio(900, 100, "xyz:INTC")
	if !capped || adjusted != 500 {
		t.Fatalf("expected final order guard to cap at 5x equity, got capped=%v adjusted=%.2f", capped, adjusted)
	}
}
