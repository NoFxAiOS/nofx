package kernel

import (
	"math"
	"strings"
	"testing"
	"time"

	"nofx/market"
	"nofx/store"
)

// The short-scan funding crowding threshold configured in the strategy UI
// must be injected into the user prompt (both percent and decimal forms) so
// the AI's entry rules follow the config instead of hard-coded numbers.
func TestBuildUserPromptInjectsFundingThreshold(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.CoinSource.SourceType = "short_scan"
	cfg.CoinSource.UseShortScan = true
	cfg.CoinSource.ShortScanFundingRatePct = 0.05

	engine := NewStrategyEngine(cfg)

	prompt := engine.BuildSystemPrompt(100, "")
	i := strings.Index(prompt, "Strategy Parameters")
	if i < 0 {
		t.Fatalf("Strategy Parameters block missing from prompt:\n%s", prompt)
	}
	seg := prompt[i:]
	// Annualized figure (0.05% × 3×365 = 54.8%), the per-settlement form, the
	// either/or structure, and the near_high exemption must all render.
	for _, want := range []string{"0.0500%", "54.8%", "funding_rollover", "near_high"} {
		if !strings.Contains(seg, want) {
			t.Fatalf("prompt block missing %q:\n%s", want, seg[:400])
		}
	}
}

// Unset threshold falls back to the built-in default (0.03%).
func TestBuildUserPromptFundingThresholdDefault(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.CoinSource.SourceType = "short_scan"

	engine := NewStrategyEngine(cfg)
	prompt := engine.BuildSystemPrompt(100, "")
	if !strings.Contains(prompt, "0.0300%") {
		t.Fatalf("default funding threshold 0.0300%% not injected:\n%s", prompt)
	}
}

// The block must not appear for strategies that don't use the short scan.
func TestBuildUserPromptNoFundingBlockWithoutShortScan(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.CoinSource.SourceType = "ai500"

	engine := NewStrategyEngine(cfg)
	if strings.Contains(engine.BuildSystemPrompt(100, ""), "做空资金费率拥挤(二选一") {
		t.Fatalf("funding block injected for a non-short-scan strategy")
	}
}

// The per-coin prompt is JSON-only: no Summary narrative, no Quantitative
// Data text block — the structured fields carry everything.
func TestPerCoinPromptIsJSONOnly(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Indicators.EnableQuantOI = true
	engine := NewStrategyEngine(cfg)

	now := time.Now()
	bars := 40
	tfData := &market.TimeframeSeriesData{Timeframe: "1h"}
	p := 5.0
	for i := 0; i < bars; i++ {
		p *= 1.002
		tfData.Klines = append(tfData.Klines, market.KlineBar{
			Time: now.Add(time.Duration(i-bars) * time.Hour).UnixMilli(),
			Open: p * 0.999, High: p * 1.002, Low: p * 0.998, Close: p, Volume: 1000 + float64(i),
		})
	}
	data := &market.Data{
		Symbol: "TESTUSDT", CurrentPrice: p,
		TimeframeData: map[string]*market.TimeframeSeriesData{"1h": tfData},
	}
	out := engine.formatMarketData(data, &QuantData{
		Symbol:      "TESTUSDT",
		PriceChange: map[string]float64{"24h": 0.09},
		OI:          map[string]*OIData{"binance": {CurrentOI: 1234.5}},
	}, nil, nil)

	if strings.Contains(out, "## Summary") || strings.Contains(out, "Quantitative Data") {
		t.Fatalf("Summary/Quant text must be gone:\n%s", out)
	}
	if !strings.Contains(out, `"volume":`) || !strings.Contains(out, `"volume_change_pct":`) {
		t.Fatal("volume fields missing from JSON")
	}
	if !strings.Contains(out, `"price_change_24h_live_pct":`) {
		t.Fatal("24h fold into derivatives missing")
	}
	// Round-4 review R4-2 regression: QuantData.PriceChange is a DECIMAL
	// (0.09 = 9%) and this field is a _pct JSON key — the raw pass-through
	// shipped 100×-too-small values the model read as ~0.0x%.
	if !strings.Contains(out, `"price_change_24h_live_pct":9`) {
		t.Fatal(`24h live change must render as a PERCENT (0.09 → 9), not the raw decimal`)
	}
	if !strings.Contains(out, `"oi_current_base":1234.5`) {
		t.Fatal("OI current fold missing")
	}
	// The conflict-resolution instruction lives in the static legend, which is
	// now rendered ONCE per prompt (not per coin) — the per-coin block must be
	// JSON-only. (Legend presence is asserted in TestPromptProgramTruthGateWording.)
	if strings.Contains(out, "MUST resolve the conflict explicitly") {
		t.Fatal("per-coin block must not repeat the legend (deduped to once-per-prompt)")
	}
	if strings.Contains(out, `"directional_conflict":true`) {
		t.Fatal("all-up structure must not flag a conflict")
	}
}

// A data-incomplete symbol carries the DO-NOT bar — its quantitative numbers
// must be hidden too, so a flashy "+56% 24h" cannot tempt an override.
func TestIncompleteSymbolHidesQuantData(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.Indicators.EnableQuantOI = true
	engine := NewStrategyEngine(cfg)

	now := time.Now()
	// Only 3 bars → data incomplete for this timeframe.
	var bars []market.KlineBar
	for i := 0; i < 3; i++ {
		c := 1.0 + float64(i)*0.001
		bars = append(bars, market.KlineBar{
			Time: now.Add(time.Duration(i-3) * time.Hour).UnixMilli(),
			Open: c, High: c * 1.001, Low: c * 0.999, Close: c, Volume: 1000,
		})
	}
	data := &market.Data{
		Symbol: "MARSCOINUSDT", CurrentPrice: 1.002,
		TimeframeData: map[string]*market.TimeframeSeriesData{
			"1h": {Timeframe: "1h", Klines: bars},
		},
	}
	out := engine.formatMarketData(data, &QuantData{
		Symbol:      "MARSCOINUSDT",
		PriceChange: map[string]float64{"1h": 0.068688, "24h": 0.565920},
	}, nil, nil)

	if !strings.Contains(out, "DO NOT trade this symbol this cycle") {
		t.Fatalf("expected the DO-NOT bar, got: %q", out)
	}
	if strings.Contains(out, "Quantitative Data") {
		t.Fatalf("quant numbers must be hidden for barred symbols, got: %q", out)
	}
}

// The stop band wording must match validateOpenRisk EXACTLY: the cap is
// max(2×ATR(4h), 8%) — wide-of-the-two, never min — and the noise floor
// clause only renders when the strategy actually enables one (SLMinATRMult>0;
// the executor enforces no floor at mult<=0). The old wording listed
// "且 d ≤ 8%" AND "上限取 max(...)" simultaneously — mathematically
// contradictory (min vs max) and it flipped VTHOUSDT's verdict depending on
// which sentence the model believed.
func TestStopBandWordingMatchesExecutor(t *testing.T) {
	// Strategy WITH a noise floor (1.5×ATR(1h), e.g. Balanced/做空): the
	// prompt teaches ADOPTION of the precomputed stop plan (09-19 RR audit) —
	// gate, plan and executor's checkRR price the same stop.
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.SLMinATRMult = 1.5
	engine := NewStrategyEngine(cfg)
	prompt := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{
		"止损(程序预计算,逐字采用)",
		"stop_plan_price",
		"≥1.5×ATR(1h)",
		"max(2×ATR(4h),8%)",
		"STOP_PLAN_NO_STRUCTURE", "STOP_PLAN_OUT_OF_BAND",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("floor variant missing %q:\n%s", want, extractStopLine(prompt))
		}
	}
	for _, gone := range []string{"且 d ≤ 8%", "严禁照抄为 stop_loss"} {
		if strings.Contains(prompt, gone) {
			t.Fatalf("obsolete wording still present: %q:\n%s", gone, extractStopLine(prompt))
		}
	}

	// Strategy WITHOUT a floor (SLMinATRMult=0, e.g. Conservative): no
	// rr_scan/stop_plan — manual methodology, no phantom 1.5 default.
	cfg0 := &store.StrategyConfig{}
	engine0 := NewStrategyEngine(cfg0)
	prompt0 := engine0.BuildSystemPrompt(100, "")
	if !strings.Contains(prompt0, "止损(手工方法论)") || !strings.Contains(prompt0, "未启用噪声下限") {
		t.Fatalf("floor-less variant wrong:\n%s", extractStopLine(prompt0))
	}
	if strings.Contains(prompt0, "d ≥ 1.5×ATR(1h)") {
		t.Fatalf("floor-less strategy must not show a phantom 1.5×ATR floor:\n%s", extractStopLine(prompt0))
	}
}

func extractStopLine(prompt string) string {
	i := strings.Index(prompt, "止损(")
	if i < 0 {
		return "(stop-loss line missing)"
	}
	j := strings.Index(prompt[i:], "\n")
	if j < 0 {
		return prompt[i:]
	}
	return prompt[i : i+j]
}

// TP rule must force a FULL scan of the opposing-structure arrays across all
// role timeframes before any "no usable structure" verdict: on BZUSDT the
// model read resistance[0] (103.49), declared "再远无任何历史结构" while
// resistance[1] (104.297) sat right there in the 15m block — the skip was
// right by luck (RR 1.14 < 1.5), not by process. Review 2026-09-15 point 1:
// the scan is now PROGRAM-DELIVERED via hard_entry_gate.rr_scan — the rule
// must pin (a) the full-array scan semantics of the field, (b) adoption of
// first_rr_ge_target, (c) MAX_STRUCTURAL_RR as the failure verdict wording,
// and (d) the manual fallback for symbols without the field.
func TestBuildUserPromptTPRequiresFullArrayScan(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	engine := NewStrategyEngine(cfg)
	prompt := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{
		// 09-29 menu directive: the TP contract is the discrete menu pick;
		// the levels themselves stay program-precomputed.
		"rr_scan.tp_options",
		"MAX_STRUCTURAL_RR=best_rr",
		"usable=false",
		"tp_option",
		"exit_mode",
		"最低净RR=1.5",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("TP rule missing %q", want)
		}
	}
}

// Market entries are ONLY permitted as the breakout-chase exception with all
// six conditions spelled out (user spec 09-11): breakout.status confirmed,
// volume+OI confirmation, directional_score>=80, no directional conflict, no
// loss-streak ban. Renders with limit-entry enabled; absent when disabled
// (market is then unrestricted by prompt).
func TestBreakoutChaseExceptionWording(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.LimitEntryEnabled = true
	engine := NewStrategyEngine(cfg)
	prompt := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{
		"hard_entry_gate",
		"`allowed=false` 必须 wait",
		"`allowed=true && limit_allowed=true`",
		"`allowed=true && limit_allowed=false && market_exception=true`",
		"confidence≥80",
		"不存在其他例外",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("breakout-chase exception missing %q", want)
		}
	}

	cfgOff := &store.StrategyConfig{}
	cfgOff.RiskControl.LimitEntryEnabled = false
	engineOff := NewStrategyEngine(cfgOff)
	promptOff := engineOff.BuildSystemPrompt(100, "")
	if strings.Contains(promptOff, "仅限突破追入例外") {
		t.Fatal("breakout-chase exception must not render when limit entry is disabled")
	}
}

// TP ladder thresholds resolve: 0/unset → spec defaults (trim 10 / full 25),
// negative → tier off.
func TestTpTierAction(t *testing.T) {
	// Default rc: the 1R profit lock (1.0) is active → the ROE trim tier is
	// superseded; only the full-close tier remains.
	rc := &store.RiskControlConfig{}
	if got := TpTierAction(10, false, rc); got != "" {
		t.Fatalf("lock active: ROE trim tier must yield, got %q", got)
	}
	if got := TpTierAction(25, true, rc); got != "full" {
		t.Fatalf("25%% → full close backstop, got %q", got)
	}
	// Lock disabled (negative) → the ROE trim tier works again.
	offLock := &store.RiskControlConfig{ProfitLockAtR: -1}
	if got := TpTierAction(10, false, offLock); got != "trim" {
		t.Fatalf("lock disabled: 10%% with unspent trim → trim, got %q", got)
	}
	// Both tiers disabled via negative config.
	off := &store.RiskControlConfig{ProfitLockAtR: -1, TpTrimProfitPct: -1, TpFullProfitPct: -1}
	if got := TpTierAction(50, false, off); got != "" {
		t.Fatalf("disabled tiers must no-op, got %q", got)
	}
	// Prompt renders the ladder and drops the frequency-cap wording.
	// Default config → breakeven offset 0.2R active (09-21 experiment):
	// the stop parks at 开仓价+0.20R, not plain entry.
	engine := NewStrategyEngine(&store.StrategyConfig{})
	prompt := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{"程序自动市价减仓 50%", "确保止损至开仓价+0.20R", "程序自动全部平仓"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("TP ladder missing %q", want)
		}
	}
	sp := engine.BuildSystemPrompt(100, "")
	if strings.Contains(sp, "2 trades/hour = overtrading") || strings.Contains(sp, "Trading Frequency Awareness") {
		t.Fatal("frequency-cap wording must be gone")
	}
	if !strings.Contains(sp, "No frequency cap") {
		t.Fatal("no-frequency-cap statement missing")
	}
}

// Hold-on-position decisions must carry the management self-assessment
// contract (quality + flags), mirroring entry_quality for the backtest.
func TestBuildUserPromptManagementFields(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{})
	prompt := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{
		"management_quality", "management_flags",
		"BREAKEVEN_WARRANTED", "PARTIAL_WARRANTED",
		"直接输出 adjust_stop_loss/partial_close_*", "不要 hold+flag",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("management contract missing %q", want)
		}
	}
}

// Rally: a downtrend bounce must classify as rally; the timing-gate doc and
// the direction-aware buffer hint must reflect the symmetric policy.
func TestBuildUserPromptRallyWindow(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.EntryTimingGate = true
	cfg.RiskControl.SLMinATRMult = 1.5 // the stop-plan line renders only with a floor
	engine := NewStrategyEngine(cfg)
	prompt := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{
		"做空需 down/rally(下跌趋势中的反弹=空头入场窗)",
		"止损(程序预计算,逐字采用)",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("rally window prompt missing %q", want)
		}
	}
}

// Program-truth gate wording (review 2026-09-15): the per-coin legend must
// introduce hard_entry_gate / rr_scan / bias / funding_rollover with their
// verdict semantics (MAX_STRUCTURAL_RR wording, scanner≠market-short,
// anchor-suppression-is-not-a-direction-ban), carry the data freshness
// priority that bars ranking prices from exact math, and the system prompt
// must teach the wait state machine with the RECHECK_ALL_HARD_GATES
// discipline and the next-snapshot (never weekly) re-evaluation semantics.
func TestPromptProgramTruthGateWording(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.RiskControl.SLMinATRMult = 1.5
	cfg.RiskControl.OpenRejectSupplyPct = 0.5 // renders the anchor-offset/breathing params line
	cfg.CoinSource.SourceType = "short_scan"
	engine := NewStrategyEngine(cfg)

	now := time.Now()
	tfData := &market.TimeframeSeriesData{Timeframe: "1h"}
	p := 5.0
	for i := 0; i < 80; i++ {
		p *= 1.002
		if i%7 == 0 {
			p *= 1.02 // swing high spikes → pivot highs
		}
		if i%11 == 0 {
			p *= 0.97 // deep dips → pivot lows (S/R arrays → stop plans)
		}
		tfData.Klines = append(tfData.Klines, market.KlineBar{
			Time: now.Add(time.Duration(i-80) * time.Hour).UnixMilli(),
			Open: p * 0.999, High: p * 1.002, Low: p * 0.998, Close: p, Volume: 1000 + float64(i),
		})
	}
	data := withVendor(&market.Data{
		Symbol: "TESTUSDT", CurrentPrice: p,
		FundingRate: 0.00005, FundingRateOK: true, FundingSettleHours: 4,
		FundingHistory: []float64{0.0004, 0.0004, 0.0004, 0.0004, 0.00005, 0.00005},
		TimeframeData:  map[string]*market.TimeframeSeriesData{"1h": tfData},
	}, 0.05)
	out := engine.formatMarketData(data, nil, nil, nil)
	for _, want := range []string{
		`"hard_entry_gate"`, `"rr_scan"`, `"bias"`, `"funding_rollover"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("per-coin block missing %q", want)
		}
	}
	// The coin-independent legend must NOT be repeated per coin — it is
	// rendered once before the first signal block. Duplicating it across 8+
	// candidates wasted ~21k chars every cycle.
	for _, gone := range []string{
		"数据新鲜度优先级", "MAX_STRUCTURAL_RR", "不等于市场空头证据强",
		"这只是限价路径被禁≠该方向整体禁止", "严禁参与 entry/SL/TP/RR 精确计算",
		"stop_price 仅是门槛校验口径", "逐字采用 stop_plan_price",
	} {
		if strings.Contains(out, gone) {
			t.Errorf("per-coin block must not repeat legend %q", gone)
		}
	}

	sys1 := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{
		"开仓硬门(唯一权威,禁止重算)",
		"stop_loss 复制 stop_plan_price",
		"derivatives.funding_rollover.detected",
		"禁止从 scanner patterns",
		"RECHECK_ALL_HARD_GATES",
		"数据新鲜度优先级",
		"锚点呼吸空间已并入 hard_entry_gate;直接采用其结论与 entry_price,不要自行重算", // market-entry default config
	} {
		if !strings.Contains(sys1, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
	// Legend appears exactly once in the whole prompt (dedup guard).
	if n := strings.Count(sys1, "数据新鲜度优先级"); n != 1 {
		t.Errorf("legend rendered %d times, want exactly 1", n)
	}

	sys := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{
		"方向性 wait 的 next_trigger 必须是",
		"RECHECK_ALL_HARD_GATES",
		"每周期自动重评", "禁止按天/周搁置",
		"只允许收紧到保本或更好",
	} {
		if !strings.Contains(sys, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
}

// The rr_scan ceiling shown this cycle must be captured per symbol onto the
// Context (09-16 point 2: the wait→fill RR-decay dataset reads this value at
// insert time; no later re-derivation).
func TestFormatMarketDataRecordsRRCeilings(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.SLMinATRMult = 1.5
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	engine := NewStrategyEngine(cfg)

	now := time.Now()
	tfData := buildTF("1h", now, 80, 5.0, false)
	// The RR plan must be backed by real swing pivots, not only the dynamic
	// Bollinger supplements. Add repeated closed-bar highs/lows above/below the
	// live price so both directions have genuine structural stops and targets.
	for i := 16; i+4 < len(tfData.Klines)-1; i += 8 {
		tfData.Klines[i].High = tfData.Klines[i].Close * 1.03
		lowIndex := i + 4
		tfData.Klines[lowIndex].Low = tfData.Klines[lowIndex].Close * 0.97
	}
	data := withVendor(&market.Data{
		Symbol: "TESTUSDT", CurrentPrice: tfData.Klines[len(tfData.Klines)-1].Close,
		TimeframeData: map[string]*market.TimeframeSeriesData{"1h": tfData},
	}, 0.05)
	ctx := &Context{}
	out := engine.formatMarketData(data, nil, ctx, nil)
	c := ctx.RRCeilings["TESTUSDT"]
	if c == nil {
		t.Fatalf("rr ceiling not recorded (keys %v)", ctx.RRCeilings)
	}
	if c.LongRR == 0 && c.ShortRR == 0 {
		t.Fatalf("empty ceiling with a configured noise floor: %+v", c)
	}
	// The recorded value must match the best_rr the model saw in the JSON.
	if !strings.Contains(out, `"hard_entry_gate"`) {
		t.Fatal("gate block not rendered for a complete symbol")
	}

	// No noise floor → no ceiling values (never fabricate one).
	cfg2 := &store.StrategyConfig{}
	engine2 := NewStrategyEngine(cfg2)
	ctx2 := &Context{}
	engine2.formatMarketData(data, nil, ctx2, nil)
	if c2 := ctx2.RRCeilings["TESTUSDT"]; c2 == nil || c2.LongRR != 0 || c2.ShortRR != 0 {
		t.Errorf("ceiling without a floor = %+v, want zeros present (captured but empty)", c2)
	}
}

// 09-19 audit: the sizing prose read the config (mac-nofx/Conservative 3.5%)
// while BOTH examples hardcoded 1.5% — and the model sized from the example
// (BTWUSDT opened at 1.5% risk against the 3.5% config). The configured
// risk budget must be the SINGLE source: prose, examples, min_size and the
// executor clamp all render/compute the same number.
func TestRiskBudgetSingleSource(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.RiskPerTradePct = 3.5
	engine := NewStrategyEngine(cfg)
	sp := engine.BuildSystemPrompt(75, "")
	for _, want := range []string{
		"equity × 3.5% (risk budget)",
		"100 × 3.5 ÷ 6.68 ≈ 52.4 USDT",
		"stop loss plus estimated costs ≈ 3.50 USDT",
		"actual sizing uses current user-prompt equity",
	} {
		if !strings.Contains(sp, want) {
			t.Errorf("system prompt missing %q", want)
		}
	}
	for _, gone := range []string{"× 1.5 ÷ 6.48", "equity 75", "按量化权益"} {
		if strings.Contains(sp, gone) {
			t.Errorf("hardcoded 1.5%% example still present: %q", gone)
		}
	}

	// Unset config falls back to the 1.5 legacy default — consistently, in
	// prose AND examples.
	engine0 := NewStrategyEngine(&store.StrategyConfig{})
	sp0 := engine0.BuildSystemPrompt(100, "")
	if !strings.Contains(sp0, "100 × 1.5 ÷ 6.68") || !strings.Contains(sp0, "stop loss plus estimated costs ≈ 1.50 USDT") {
		t.Error("default 1.5% must render consistently in prose and examples")
	}
	if sp != engine.BuildSystemPrompt(9999.99, "") {
		t.Error("system prompt must remain byte-stable across live equity changes")
	}
}

// The universe legend must render the gainer history pool window through the
// SAME resolution the scanner uses — a handwritten "7" here would drift from
// the config exactly like the prompt-example incidents (nofx-prompt-examples).
func TestBuildSystemPromptRendersUniverseLegend(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.CoinSource.SourceType = "short_scan"

	engine := NewStrategyEngine(cfg)
	if !strings.Contains(engine.BuildSystemPrompt(100, ""), "近 7 天每日涨幅 Top20") {
		t.Fatal("default window (7d) legend missing from system prompt")
	}

	cfg.CoinSource.ShortScanHistoryDays = 14
	engine = NewStrategyEngine(cfg)
	if !strings.Contains(engine.BuildSystemPrompt(100, ""), "近 14 天每日涨幅 Top20") {
		t.Fatal("configured window (14d) legend missing from system prompt")
	}

	// Pool disabled → no hist_gainer legend at all.
	cfg.CoinSource.ShortScanHistoryDays = -1
	engine = NewStrategyEngine(cfg)
	if strings.Contains(engine.BuildSystemPrompt(100, ""), "hist_gainer") {
		t.Fatal("disabled pool must not render the universe legend")
	}
}

// The extended-pump long guard (user directive 2026-09-20) must render its
// params line from the RESOLVED threshold — never a handwritten default —
// and the hard-gate line must list EXTENDED_PUMP among the no-exception
// blocker codes.
func TestBuildSystemPromptRendersPumpGuard(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.CoinSource.SourceType = "static"

	engine := NewStrategyEngine(cfg)
	prompt := engine.BuildSystemPrompt(100, "")
	for _, want := range []string{"暴涨延伸做多确认门", "≥20%", "EXTENDED_PUMP_UNCONFIRMED"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("default-guard prompt missing %q", want)
		}
	}
	if !strings.Contains(prompt, "hard_entry_gate.failed 给出 EXTENDED_PUMP_UNCONFIRMED") {
		t.Fatal("pump guard must delegate its verdict to the hard gate")
	}

	// Configured threshold must flow through verbatim; disabled (negative)
	// must drop the line entirely.
	cfg.RiskControl.PumpGuard4hPct = 35
	engine = NewStrategyEngine(cfg)
	if !strings.Contains(engine.BuildSystemPrompt(100, ""), "≥35%") {
		t.Fatal("configured 35% threshold missing from prompt")
	}
	cfg.RiskControl.PumpGuard4hPct = -1
	engine = NewStrategyEngine(cfg)
	if strings.Contains(engine.BuildSystemPrompt(100, ""), "暴涨延伸做多确认门") {
		t.Fatal("disabled guard must not render its params line")
	}
}

func TestSystemPromptOutputContractIsCompactAndUnambiguous(t *testing.T) {
	cfg := &store.StrategyConfig{}
	cfg.RiskControl.LimitEntryEnabled = true
	cfg.RiskControl.SLMinATRMult = 1.5
	cfg.RiskControl.MinRiskRewardRatio = 1.5
	cfg.RiskControl.MinConfidence = 70
	cfg.RiskControl.AltcoinMaxLeverage = 3
	cfg.RiskControl.BTCETHMaxLeverage = 3
	prompt := NewStrategyEngine(cfg).BuildSystemPrompt(123.45, "")

	for _, want := range []string{
		"Output exactly two XML blocks and nothing else",
		"<reasoning>", "</reasoning>", "<decision>", "</decision>",
		"strict JSON array without Markdown fences",
		"Min Confidence: ≥70 (低于门槛的开仓由后端拒绝)",
		"Max Leverage: Altcoins 3x | BTC/ETH 3x",
		"后端对 TP 与 SL 使用同一 0.05% 容差强制吸附",
		"串联(AND)", "净方向风险上限(程序强制)",
		"BSTOCK_DAILY_DATA_UNAVAILABLE", "盘前、正常盘、盘后和夜间均允许按 15m",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("canonical output contract missing %q", want)
		}
	}
	for _, gone := range []string{
		"Output raw JSON only", "```json", "1R 减仓 50%+保本、1.5R 跟踪止损、25% 全平",
		"CFTC 比特币期货持仓周报",
	} {
		if strings.Contains(prompt, gone) {
			t.Errorf("obsolete or dynamic system-prompt content remains: %q", gone)
		}
	}
	if len(prompt) > 26000 {
		t.Errorf("system prompt grew to %d bytes; compact contract budget is 26000", len(prompt))
	}
}

func TestPersonalizedFrequencyPolicyReplacesGenericPolicy(t *testing.T) {
	cfg := &store.StrategyConfig{CustomPrompt: "# 个性化节奏\n不要频繁交易,只做最强设置。"}
	cfg.PromptSections.TradingFrequency = "# 通用频率\n高信心时频率不设上限。"
	prompt := NewStrategyEngine(cfg).BuildSystemPrompt(100, "")
	if strings.Contains(prompt, "高信心时频率不设上限") {
		t.Fatal("generic frequency policy must be omitted when personalized prompt defines one")
	}
	for _, want := range []string{"不要频繁交易", "唯一交易频率/仓位节奏规则"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("resolved personalized frequency contract missing %q", want)
		}
	}
}

func TestPositionPromptExposesAutomationState(t *testing.T) {
	engine := NewStrategyEngine(&store.StrategyConfig{})
	ctx := &Context{MarketDataMap: map[string]*market.Data{}}
	out := engine.formatPositionInfo(1, PositionInfo{
		Symbol: "BTCUSDT", Side: "long", EntryPrice: 100, MarkPrice: 110,
		Quantity: 0.25, Leverage: 3, Managed: true,
		AutomationStage: "R_LOCK_TRIMMED", CumulativeReducedPct: 50,
	}, ctx)
	for _, want := range []string{"AutoMgmt R_LOCK_TRIMMED", "Reduced 50.0% of original"} {
		if !strings.Contains(out, want) {
			t.Fatalf("position automation state missing %q: %s", want, out)
		}
	}
}

// The static token estimate's raw-kline term must mirror the RENDERER's
// per-timeframe bar budget (ResolvePromptKlineBars). The estimate kept
// pricing the pre-09-27 60-bar dumps and fired phantom "approaching context
// limit" advisories (108k vs a real render of ~7-10k tokens — user report
// 2026-10-01). store cannot import kernel, so the resolution is mirrored in
// store.EstimateTokens; this test pins the two together via the raw-kline
// cost delta.
func TestEstimateTokensMirrorPromptKlineBars(t *testing.T) {
	// diff(cfg) = coins × TFs × bars × 60chars / 4 — the coins/TFs factors
	// cancel in ratios against the 20-bar default, so every case asserts an
	// exact multiple of the base raw-kline cost.
	base := func(promptBars, primary int) float64 {
		cfg := store.GetDefaultStrategyConfig("zh")
		cfg.Indicators.EnableRawKlines = true
		cfg.Indicators.Klines.PrimaryCount = primary
		cfg.Indicators.Klines.PromptKlineBars = promptBars
		cfg.ClampLimits() // the advisory path clamps before estimating
		off := cfg
		off.Indicators.EnableRawKlines = false
		d := float64(cfg.EstimateTokens().Total - off.EstimateTokens().Total)
		if d <= 0 {
			t.Fatalf("promptBars=%d primary=%d: raw-kline term must add tokens, got %v", promptBars, primary, d)
		}
		return d
	}
	cases := []struct {
		promptBars, primary int
		wantRatio           float64 // bars / 20
	}{
		{0, 120, 1},           // default 20 bars
		{-1, 120, 120 / 20.0}, // legacy full-PrimaryCount escape hatch
		{30, 120, 30 / 20.0},  // explicit, below primary
		{5, 120, 10 / 20.0},   // floor 10
		{50, 20, 50 / 20.0},   // primary clamps up to 60 first (ClampLimits), so 50 bars
	}
	for _, tc := range cases {
		got := base(tc.promptBars, tc.primary) / base(0, 120)
		if math.Abs(got-tc.wantRatio) > 0.001 {
			t.Fatalf("promptBars=%d primary=%d: estimate priced %.3f× the 20-bar base, want %.3f (renderer ships %d bars)",
				tc.promptBars, tc.primary, got, tc.wantRatio, ResolvePromptKlineBars(tc.promptBars, tc.primary))
		}
	}
}
