package kernel

import (
	"strings"
	"testing"

	"nofx/store"
)

// A market-entry strategy (limit_entry_enabled=false) must not be taught the
// limit path: the live strategy opened almost exclusively via open_*_limit
// because the example decision and legend lines were unconditional.
func TestSystemPromptMarketEntryMode(t *testing.T) {
	for _, tpMenu := range []bool{false, true} {
		cfg := &store.StrategyConfig{}
		cfg.RiskControl.LimitEntryEnabled = false
		cfg.RiskControl.TPMenuEnabled = &tpMenu
		prompt := NewStrategyEngine(cfg).BuildSystemPrompt(100, "")

		for _, want := range []string{
			"\"action\": \"open_short\", \"leverage\"",
			"(本策略默认市价入场: 开仓只用 open_long/open_short,不要输出 open_*_limit)",
			"`allowed=true` 直接输出市价 open_long/open_short(不填 price)",
			"true→市价 open_long/open_short,按 entry_price(实时价)计划复制止损止盈",
			"true 时直接市价开仓(open_long/open_short)",
		} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("tpMenu=%v: market-mode prompt missing %q", tpMenu, want)
			}
		}
		for _, banned := range []string{
			"open_short_limit\", \"price\"",
			"true 且 limit_allowed→限价复制",
			"按 limit_allowed/market_exception 选择限价或市价路径",
			"本策略默认限价入场",
		} {
			if strings.Contains(prompt, banned) {
				t.Fatalf("tpMenu=%v: market-mode prompt still contains %q", tpMenu, banned)
			}
		}
	}
}

func TestSystemPromptLimitEntryModeUnchanged(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.LimitEntryEnabled = true
	prompt := NewStrategyEngine(cfg).BuildSystemPrompt(100, "")
	for _, want := range []string{
		"\"action\": \"open_short_limit\", \"price\": 150.00, \"leverage\"",
		"本策略默认限价入场",
		"true 且 limit_allowed→限价复制 entry_price",
		"`allowed=true && limit_allowed=true` 默认输出对应 open_*_limit",
		"按 limit_allowed/market_exception 选择限价或市价路径",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("limit-mode prompt lost %q", want)
		}
	}
	if strings.Contains(prompt, "本策略默认市价入场") {
		t.Fatal("limit-mode prompt must not carry the market-entry note")
	}
	if signalBlockLegendFor(true) != signalBlockLegend {
		t.Fatal("limit-mode legend must equal signalBlockLegend byte-for-byte")
	}
	if signalBlockLegendFor(false) == signalBlockLegend || !strings.Contains(signalBlockLegendFor(false), signalGateLineMarket) {
		t.Fatal("market-mode legend must swap the hard_entry_gate line")
	}
}
