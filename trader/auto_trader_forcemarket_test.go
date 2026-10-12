package trader

import (
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func forceMarketTestTrader(limitEnabled bool, withConfig bool) *AutoTrader {
	at := &AutoTrader{name: "t"}
	if withConfig {
		cfg := &store.StrategyConfig{}
		cfg.RiskControl.LimitEntryEnabled = limitEnabled
		at.config.StrategyConfig = cfg
	}
	return at
}

func forceMarketInput() []kernel.Decision {
	return []kernel.Decision{
		{Symbol: "AAAUSDT", Action: "open_long_limit", Price: 1.5},
		{Symbol: "BBBUSDT", Action: "open_short_limit", Price: 2.5},
		{Symbol: "CCCUSDT", Action: "open_short", Price: 0},
		{Symbol: "DDDUSDT", Action: "close_long"},
		{Symbol: "EEEUSDT", Action: "wait"},
	}
}

func TestForceMarketEntriesRewritesInMarketMode(t *testing.T) {
	in := forceMarketInput()
	out := forceMarketTestTrader(false, true).forceMarketEntries(in)
	want := []struct {
		action string
		price  float64
	}{{"open_long", 0}, {"open_short", 0}, {"open_short", 0}, {"close_long", 0}, {"wait", 0}}
	for i, w := range want {
		if out[i].Action != w.action || out[i].Price != w.price {
			t.Fatalf("[%d] got %s@%v want %s@%v", i, out[i].Action, out[i].Price, w.action, w.price)
		}
	}
	if in[0].Action != "open_long_limit" || in[0].Price != 1.5 {
		t.Fatal("input slice must not be mutated")
	}
}

func TestForceMarketEntriesNoopWhenLimitEnabledOrNoConfig(t *testing.T) {
	for name, at := range map[string]*AutoTrader{
		"limit enabled": forceMarketTestTrader(true, true),
		"nil config":    forceMarketTestTrader(false, false),
	} {
		out := at.forceMarketEntries(forceMarketInput())
		in := forceMarketInput()
		for i := range in {
			if out[i].Action != in[i].Action || out[i].Price != in[i].Price {
				t.Fatalf("%s: [%d] changed to %s@%v", name, i, out[i].Action, out[i].Price)
			}
		}
	}
}
