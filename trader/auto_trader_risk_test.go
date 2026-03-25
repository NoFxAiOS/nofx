package trader

import (
	"testing"
	"nofx/store"
)

func TestGetDrawdownMonitorInterval(t *testing.T) {
	at := &AutoTrader{}
	if got := at.getDrawdownMonitorInterval(); got.Seconds() != 60 {
		t.Fatalf("expected default 60s interval, got %s", got)
	}

	at.config.StrategyConfig = &store.StrategyConfig{}
	at.config.StrategyConfig.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{
		Enabled: true,
		Rules: []store.DrawdownTakeProfitRule{
			{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100, PollIntervalSeconds: 45},
			{MinProfitPct: 8, MaxDrawdownPct: 20, CloseRatioPct: 50, PollIntervalSeconds: 15},
		},
	}

	if got := at.getDrawdownMonitorInterval(); got.Seconds() != 15 {
		t.Fatalf("expected 15s interval, got %s", got)
	}
}

func TestMatchDrawdownRule(t *testing.T) {
	at := &AutoTrader{}
	rules := []store.DrawdownTakeProfitRule{
		{MinProfitPct: 5, MaxDrawdownPct: 40, CloseRatioPct: 100},
		{MinProfitPct: 10, MaxDrawdownPct: 20, CloseRatioPct: 50},
	}

	matched := at.matchDrawdownRule(12, 25, rules)
	if matched == nil {
		t.Fatal("expected a matched rule")
	}
	if matched.MinProfitPct != 10 || matched.CloseRatioPct != 50 {
		t.Fatalf("expected higher-priority rule, got %+v", *matched)
	}

	if matched := at.matchDrawdownRule(4, 60, rules); matched != nil {
		t.Fatalf("expected no rule below min profit, got %+v", *matched)
	}
}

func TestGetActiveDrawdownRulesFiltersInvalidRules(t *testing.T) {
	at := &AutoTrader{
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{},
		},
	}
	at.config.StrategyConfig.Protection.DrawdownTakeProfit = store.DrawdownTakeProfitConfig{
		Enabled: true,
		Rules: []store.DrawdownTakeProfitRule{
			{MinProfitPct: 0, MaxDrawdownPct: 20, CloseRatioPct: 50},
			{MinProfitPct: 5, MaxDrawdownPct: 30, CloseRatioPct: 150},
		},
	}

	rules := at.getActiveDrawdownRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 valid rule, got %d", len(rules))
	}
	if rules[0].CloseRatioPct != 100 {
		t.Fatalf("expected close ratio clamped to 100, got %.2f", rules[0].CloseRatioPct)
	}
}
