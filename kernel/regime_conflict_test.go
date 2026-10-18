package kernel

import (
	"strings"
	"testing"
	"time"

	"nofx/market"
	"nofx/mcp"
	"nofx/store"
)

// ── 09-19 audit: EXECUTION_VS_STRUCTURE conflict ────────────────────────────
// ZEC 09-18: 1h/4h both up (directional_score +100, 4 bull / 0 bear) while
// the 15m micro window permitted shorts only — the model was asked to short
// a +100 consensus and the conflict machinery stayed silent.

func TestExecutionVsStructureConflict(t *testing.T) {
	now := time.Now()
	data := &market.Data{
		Symbol: "ZECUSDT", CurrentPrice: 1483.31,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": buildDecliningTF("15m", now, 80, 1483.0), // micro down → shorts only
			"1h":  buildTF("1h", now, 80, 1400.0, false),    // up
			"4h":  buildTF("4h", now, 80, 1300.0, false),    // up
		},
	}
	sig, err := ComputeSymbolSignals("ZECUSDT", data, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if sig.ExecutionFilter == nil || !sig.ExecutionFilter.ShortAllowed || sig.ExecutionFilter.LongAllowed {
		t.Fatalf("fixture drift: execution filter = %+v, want shorts-only", sig.ExecutionFilter)
	}
	c := sig.SignalConflict
	if c == nil || !c.DirectionalConflict {
		t.Fatalf("directional_conflict=false — execution↔structure opposition still silent: %+v", c)
	}
	found := false
	for _, ty := range c.Types {
		if ty == "EXECUTION_VS_STRUCTURE" {
			found = true
		}
	}
	if !found {
		t.Errorf("EXECUTION_VS_STRUCTURE missing from types %v", c.Types)
	}
	if !strings.Contains(c.Note, "allows only short") || !strings.Contains(c.Note, "reads long") {
		t.Errorf("note should name both sides, got %q", c.Note)
	}

	// The clean shape (KORU: structure long AND execution long-only) must
	// NOT flag the conflict.
	data2 := &market.Data{
		Symbol: "KORUUSDT", CurrentPrice: 100.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": buildTF("15m", now, 80, 95.0, false), // up
			"1h":  buildTF("1h", now, 80, 90.0, false),  // up
			"4h":  buildTF("4h", now, 80, 85.0, false),  // up
		},
	}
	sig2, err := ComputeSymbolSignals("KORUUSDT", data2, SignalOptions{Now: now, PrimaryTF: "15m"})
	if err != nil {
		t.Fatalf("compute2: %v", err)
	}
	if sig2.SignalConflict.DirectionalConflict {
		t.Errorf("aligned structure+execution flagged as conflict: %+v", sig2.SignalConflict)
	}
}

// ── 09-19 audit: regime-level skip ──────────────────────────────────────────
// Every candidate double-blocked by the hard gate + no positions ⇒ the LLM
// call can only return a hold; the engine synthesizes the wait for free.

type recordingAIClient struct {
	calls  int
	deaths map[string]bool // method name → panic marker
}

func (c *recordingAIClient) SetAPIKey(string, string, string) {}
func (c *recordingAIClient) SetTimeout(time.Duration)         {}
func (c *recordingAIClient) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	c.calls++
	return `[{"action":"wait","symbol":"OKUSDT","no_trade_reason":["test"]}]`, nil
}
func (c *recordingAIClient) CallWithRequest(req *mcp.Request) (string, error) {
	c.calls++
	return `[{"action":"wait","symbol":"OKUSDT","no_trade_reason":["test"]}]`, nil
}
func (c *recordingAIClient) CallWithRequestStream(req *mcp.Request, onChunk func(string)) (string, error) {
	c.calls++
	return `[{"action":"wait","symbol":"OKUSDT","no_trade_reason":["test"]}]`, nil
}
func (c *recordingAIClient) CallWithRequestFull(req *mcp.Request) (*mcp.LLMResponse, error) {
	c.calls++
	return &mcp.LLMResponse{Content: `[{"action":"wait","symbol":"OKUSDT","no_trade_reason":["test"]}]`}, nil
}

func regimeCtx(t *testing.T, engine *StrategyEngine, coins []string) *Context {
	t.Helper()
	ctx := &Context{MarketDataMap: map[string]*market.Data{}}
	for _, sym := range coins {
		ctx.CandidateCoins = append(ctx.CandidateCoins, CandidateCoin{Symbol: sym, Sources: []string{"ai500"}})
		// Data-less market data → DATA_INSUFFICIENT on both directions →
		// hard-blocked (no allowed direction, no exception path).
		ctx.MarketDataMap[sym] = &market.Data{Symbol: sym, CurrentPrice: 1.0}
	}
	return ctx
}

func TestRegimeSkipSynthesizesWaitWithoutLLM(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{})
	mock := &recordingAIClient{}
	ctx := regimeCtx(t, engine, []string{"BAD1USDT", "BAD2USDT"})

	fd, err := GetFullDecisionWithStrategy(ctx, mock, engine, "balanced")
	if err != nil {
		t.Fatalf("skip path returned error: %v", err)
	}
	if mock.calls != 0 {
		t.Fatalf("LLM was called %d times — the regime skip did not fire", mock.calls)
	}
	if len(fd.Decisions) != 1 || fd.Decisions[0].Action != "wait" {
		t.Fatalf("synthesized decision = %+v, want a single wait", fd.Decisions)
	}
	if !strings.Contains(fd.Decisions[0].Reasoning, "Regime skip") {
		t.Errorf("synthesized reasoning should explain the skip: %q", fd.Decisions[0].Reasoning)
	}
	if fd.UserPrompt == "" {
		t.Error("decision record should still carry the prompts for the audit trail")
	}
}

func TestRegimeSkipKeepsCallForAllowedDirection(t *testing.T) {
	cfg := &store.StrategyConfig{}
	// This test needs an allowed direction; the monotone fixture otherwise
	// hits the unrelated RSI exhaustion and counter-trend distance caps.
	cfg.RiskControl.CounterMaxEMA20_1hPct = -1
	cfg.RiskControl.EntryRSI1hMax = -1
	engine := NewStrategyEngine(cfg)
	mock := &recordingAIClient{}
	ctx := &Context{MarketDataMap: map[string]*market.Data{}}
	now := time.Now()
	ctx.CandidateCoins = append(ctx.CandidateCoins,
		CandidateCoin{Symbol: "BADUSDT", Sources: []string{"ai500"}},
		CandidateCoin{Symbol: "OKUSDT", Sources: []string{"ai500"}})
	ctx.MarketDataMap["BADUSDT"] = &market.Data{Symbol: "BADUSDT", CurrentPrice: 1.0}
	ctx.MarketDataMap["OKUSDT"] = withVendor(&market.Data{Symbol: "OKUSDT", CurrentPrice: 1.0,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"15m": buildTF("15m", now, 80, 1.0, false),
			"1h":  buildTF("1h", now, 80, 1.0, false),
			"4h":  buildTF("4h", now, 80, 1.0, false),
		}}, 0.05)

	if _, err := GetFullDecisionWithStrategy(ctx, mock, engine, "balanced"); err != nil {
		t.Logf("parse error tolerated (the assertion is the call itself): %v", err)
	}
	if mock.calls == 0 {
		t.Fatal("a candidate with an allowed direction must still reach the LLM")
	}
}

func TestRegimeSkipKeepsCallWhenPositionsOpen(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{})
	mock := &recordingAIClient{}
	ctx := regimeCtx(t, engine, []string{"BADUSDT"})
	ctx.Positions = []PositionInfo{{
		Symbol: "HELdUSDT", Side: "long", EntryPrice: 1, MarkPrice: 1, Quantity: 1, Leverage: 1,
		Managed: true,
	}}

	if _, err := GetFullDecisionWithStrategy(ctx, mock, engine, "balanced"); err != nil {
		t.Logf("parse error tolerated: %v", err)
	}
	if mock.calls == 0 {
		t.Fatal("open AI positions always warrant an LLM call (hold/adjust/close management)")
	}

	// 09-27 review #3: a MANUAL position has no legal model action (hold
	// only) — it must not force the LLM call the way an actionable AI
	// position does.
	mock2 := &recordingAIClient{}
	ctx2 := regimeCtx(t, engine, []string{"BADUSDT"})
	ctx2.Positions = []PositionInfo{{
		Symbol: "MANUALUSDT", Side: "long", EntryPrice: 1, MarkPrice: 1, Quantity: 1, Leverage: 1,
	}}
	fd, err := GetFullDecisionWithStrategy(ctx2, mock2, engine, "balanced")
	if err != nil {
		t.Logf("parse error tolerated: %v", err)
	}
	if mock2.calls != 0 {
		t.Fatal("manual-only positions must not force the LLM call (hold-only, automation skipped)")
	}
	if fd == nil || len(fd.Decisions) == 0 || !strings.Contains(fd.Decisions[0].Reasoning, "1 个现有持仓") || strings.Contains(fd.Decisions[0].Reasoning, "且无持仓") {
		t.Fatalf("manual position must be reported as present but non-actionable: %+v", fd)
	}
}
