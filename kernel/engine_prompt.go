package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"nofx/market"
	"nofx/market/breakout"
	"nofx/provider/openbb"
	"nofx/store"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// Prompt Building - System Prompt
// ============================================================================

// BuildSystemPrompt builds System Prompt according to strategy configuration
func (e *StrategyEngine) BuildSystemPrompt(accountEquity float64, variant string) string {
	var sb strings.Builder
	riskControl := e.config.RiskControl
	promptSections := e.config.PromptSections

	// The live equity belongs to the per-cycle user prompt. Keep the system
	// prompt byte-stable so provider prefix caches survive account PnL changes;
	// examples below use a fixed, clearly illustrative equity instead.
	const exampleEquity = 100.0

	// 0. Data Dictionary & Schema (ensure AI understands all fields)
	lang := e.GetLanguage()
	schemaPrompt := GetSchemaPrompt(lang)
	sb.WriteString(schemaPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("---\n\n")

	// 1. Role definition (editable)
	if promptSections.RoleDefinition != "" {
		sb.WriteString(promptSections.RoleDefinition)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# You are a professional cryptocurrency trading AI\n\n")
		sb.WriteString("Your task is to make trading decisions based on provided market data.\n\n")
	}

	// 2. Trading mode variant
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥ 70\n- Allow higher positions, but must strictly set stop-loss and explain risk-reward ratio\n\n")
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open positions when multiple signals resonate\n- Prioritize cash preservation, must pause for multiple periods after consecutive losses\n\n")
	case "scalping":
		sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick action\n- If price doesn't move as expected within two bars, immediately reduce position or stop-loss\n\n")
	}

	// 3. Hard constraints (risk control)
	btcEthPosValueRatio := riskControl.BTCETHMaxPositionValueRatio
	if btcEthPosValueRatio <= 0 {
		btcEthPosValueRatio = 5.0
	}
	altcoinPosValueRatio := riskControl.AltcoinMaxPositionValueRatio
	if altcoinPosValueRatio <= 0 {
		altcoinPosValueRatio = 1.0
	}

	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("## CODE ENFORCED (Backend validation, cannot be bypassed):\n")
	// ≤0 fallbacks mirror the executor's own defaults (enforceMaxPositions=3,
	// max_margin_usage→0.9, EffectiveMinPositionSize→12) — rendering the raw
	// zero ("0 coins", "≤0%", "≥0 USDT") declared constraints the executor
	// does not have (round-4 review R4-12).
	maxPositions := riskControl.MaxPositions
	if maxPositions <= 0 {
		maxPositions = 3
	}
	sb.WriteString(fmt.Sprintf("- Max Positions: %d coins simultaneously\n", maxPositions))
	if altcoinPosValueRatio == btcEthPosValueRatio {
		// Identical multipliers: one line — two identical rows read as if
		// there were differentiated handling when there is none (09-16 audit).
		sb.WriteString(fmt.Sprintf("- Position Value Limit (all symbols): equity × %.1fx\n", altcoinPosValueRatio))
	} else {
		sb.WriteString(fmt.Sprintf("- Position Value Limit (Altcoins): equity × %.1fx\n", altcoinPosValueRatio))
		sb.WriteString(fmt.Sprintf("- Position Value Limit (BTC/ETH): equity × %.1fx\n", btcEthPosValueRatio))
	}
	maxMarginPct := riskControl.MaxMarginUsage * 100
	if riskControl.MaxMarginUsage <= 0 {
		maxMarginPct = 90
	}
	sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", maxMarginPct))
	minPosSize := riskControl.MinPositionSize
	if minPosSize <= 0 {
		minPosSize = 12
	}
	sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT\n", minPosSize))
	// 09-19 audit: min RR is double-enforced (rr_scan gate + executor
	// checkRR) — it was mis-filed under AI GUIDED and read as relaxable.
	sb.WriteString(fmt.Sprintf("- Min Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss — 程序双重校验,不可放宽)\n", riskControl.MinRiskRewardRatio))
	if capR := riskControl.EffectiveTPMaxR(); capR > 0 {
		sb.WriteString(fmt.Sprintf("- Take-Profit Cap: ≤%.2g×初始风险 (程序强制:take_profit 超过入场价±%.2g×|入场−止损| 时被截断到该价位)\n", capR, capR))
	}
	sb.WriteString(fmt.Sprintf("- Max Leverage: Altcoins %dx | BTC/ETH %dx (超限值由后端强制下调)\n",
		riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
	if riskControl.MinConfidence > 0 {
		sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d (低于门槛的开仓由后端拒绝)\n", riskControl.MinConfidence))
	}
	// Margin-budget reality check (audit 09-13): the value-ratio limits and
	// the margin budget bind at different points — state the binding one.
	// 09-19 audit: "holds about 0 full-size positions" read as "opening is
	// banned this cycle". State the FORMULA and point at the live numbers in
	// the account line instead of a misleading count.
	sb.WriteString("- Margin-budget reality: 开仓名义余量 ≈ 账户行 Available × 杠杆;使用当前 user prompt 的实时数字校验,Max Margin Usage 先于 Max Positions 约束并发\n\n")

	// Position sizing guidance — ONE formula, matching the 程序强制缩仓 rule in
	// the params block (risk budget ÷ stop distance, then clamped). The old
	// confidence-tier percentages of the Position Value Limit contradicted it
	// (review 2026-09-07: two "mandatory" sizing methods → model picks either).
	riskPctDefault := riskControl.EffectiveRiskPerTradePct()
	sb.WriteString("## Position Sizing (single formula, CODE ENFORCED)\n")
	sb.WriteString(fmt.Sprintf("`position_size_usd` = notional value = equity × %.1f%% (risk budget) ÷ (stop_distance%% + %.2f%% round-trip cost estimate), then clamped:\n", riskPctDefault, riskControl.EffectiveEntryRoundTripCostBps()/100))
	sb.WriteString(fmt.Sprintf("- Lower bound: Min Position Size (%.0f USDT) — below it, skip the setup\n", minPosSize))
	sb.WriteString("- Upper bound: the Position Value Limit above\n")
	// Fixed-equity example keeps the prompt cacheable while retaining the
	// configured risk percentage as the single source of sizing truth.
	exampleCostPct := riskControl.EffectiveEntryRoundTripCostBps() / 100
	sb.WriteString(fmt.Sprintf("- Example only: equity %.0f, stop distance 6.48%% + cost %.2f%% → %.0f × %.1f ÷ %.2f ≈ %.1f USDT notional (stop loss plus estimated costs ≈ %.2f USDT);actual sizing uses current user-prompt equity\n",
		exampleEquity, exampleCostPct, exampleEquity, riskPctDefault, 6.48+exampleCostPct, exampleEquity*riskPctDefault/(6.48+exampleCostPct), exampleEquity*riskPctDefault/100))
	sb.WriteString("- Wider stop → smaller position. `confidence` decides WHETHER to open, never a multiplier on position value — do NOT size from Position Value Limit percentages\n")
	sb.WriteString(fmt.Sprintf("- Binance Hedge Mode: at most one LONG and one SHORT per symbol. Their combined gross stop-risk (including resting entries) must stay within the SAME %.1f%% equity risk budget; split that budget across sides, never count opposite risks as offsetting. Same-side orders merge on the exchange and are not separate isolated positions.\n", riskPctDefault))
	sb.WriteString("- **DO NOT** just use available_balance as position_size_usd\n\n")

	// 4. Trading frequency (editable). When the personalized strategy already
	// defines a frequency policy, it is the sole rendered policy; showing a
	// second generic rule forces the model to arbitrate contradictory prose.
	customControlsFrequency := promptDefinesFrequency(e.config.CustomPrompt)
	if !customControlsFrequency {
		if promptSections.TradingFrequency != "" {
			sb.WriteString(promptSections.TradingFrequency)
			sb.WriteString("\n\n")
		} else {
			sb.WriteString("# Trading Frequency\n\n")
			sb.WriteString("- No frequency cap: trade as often as independent, evidence-backed setups appear. Every decision is judged on its own quality (trend alignment, RR, confirmation) — never on how many trades already happened this hour or this cycle.\n")
			sb.WriteString("- Multiple symbols per cycle are independent decisions — judge each on its own evidence.\n\n")
		}
	}

	// 5. Entry standards (editable)
	if promptSections.EntryStandards != "" {
		sb.WriteString(promptSections.EntryStandards)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# 🎯 Entry Standards (Strict)\n\n")
		sb.WriteString("Only open positions when multiple signals resonate. Feel free to use any effective analysis method, but avoid low-quality behaviors such as single indicators, contradictory signals, sideways consolidation, reopening immediately after closing, etc.\n\n")
	}
	// Data-shape pointer (09-16 audit): the old per-indicator checklist was a
	// stale parallel document — the real payload is the Structured Signal
	// with program-precomputed fields, documented once in the user prompt.
	sb.WriteString("📊 行情与衍生数据一律以每周期输入里的 Structured Signal 快照为准——hard_entry_gate / rr_scan / bias / bb_ride / short_ride / funding_rollover 等关键字段均已由程序预计算,直接采用,禁止从原始 K 线自行重算 RR/止损;字段结构以输入中的「Structured Signal 字段说明」为唯一权威,本提示不再维护第二份字段清单。\n\n")

	// 6. Decision process (editable)
	if promptSections.DecisionProcess != "" {
		sb.WriteString(promptSections.DecisionProcess)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# 📋 Decision Process\n\n")
		sb.WriteString("1. Check positions → Should we take profit/stop-loss\n")
		sb.WriteString("2. Scan candidate coins + multi-timeframe → Are there strong signals\n")
		sb.WriteString("3. Write a SHORT decision_summary first, then output the strict JSON\n\n")
	}

	// 7. Output contract. Keep one canonical framing: XML separates the audit
	// summary from the payload, while the content INSIDE <decision> is a strict
	// JSON array. The old "raw JSON only" line contradicted this contract and
	// caused models to alternate formats even though the parser accepts both.
	sb.WriteString("# Output Contract (Strict)\n\n")
	sb.WriteString("Output exactly two XML blocks and nothing else. `<decision>` must contain one strict JSON array without Markdown fences, comments, placeholders, or trailing commas.\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("decision_summary — 简短、可审计,不写隐藏推理链:\n")
	sb.WriteString("- 每个决策最多一行: action + 采用的程序字段/阻断码 + 关键价格;不要重复 JSON 中的完整理由数组\n")
	sb.WriteString("- 已被 hard_entry_gate 拦截的方向只引用 failed,不要继续展开市场分析\n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("[\n")
	// 09-19 audit: the old example taught FOUR violations — risk_usd 300
	// exceeding notional 225, a market order against the limit-default,
	// stale BTC price levels, and missing required self-assessment fields.
	// LLMs weight examples over prose, so every number here is COMPUTED from
	// the live config and the sizing formula (equity × risk%% ÷ stop%%);
	// price levels are illustrative round numbers, internally consistent.
	exRiskUSD := exampleEquity * riskPctDefault / 100
	exNotional := exRiskUSD / 0.03 // 3% stop distance in the example
	// SHORT-limit geometry (the audit 四 case): SL ABOVE entry (+3%),
	// TP BELOW entry (−4.8%, RR 1.6), risk_usd = notional × stop%;
	// Derived fields stay out of the model contract: risk_usd is unused model
	// output; opening entry_quality is backfilled from confidence; hard-gate
	// blockers are mapped by the backend from failed codes.
	exEntry := 150.0
	exSL := exEntry * 1.03  // 154.50 — above entry for a short
	exTP := exEntry * 0.952 // 142.80 — below entry for a short
	// Market-entry strategies (limit_entry_enabled=false) get a market example
	// without "price": the model copies the example's action, so a limit example
	// would push it to open_*_limit even though the strategy default is market.
	exAction, exPriceField := "open_short_limit", fmt.Sprintf("\"price\": %.2f, ", exEntry)
	if !riskControl.LimitEntryEnabled {
		exAction, exPriceField = "open_short", ""
	}
	if TPMenuEnabled(&riskControl) {
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"SOLUSDT\", \"action\": \"%s\", %s\"leverage\": %d, \"position_size_usd\": %.1f, \"stop_loss\": %.2f, \"take_profit\": %.2f, \"tp_option\": 2, \"exit_mode\": \"range\", \"confidence\": 85},\n",
			exAction, exPriceField, riskControl.BTCETHMaxLeverage, exNotional, exSL, exTP))
	} else {
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"SOLUSDT\", \"action\": \"%s\", %s\"leverage\": %d, \"position_size_usd\": %.1f, \"stop_loss\": %.2f, \"take_profit\": %.2f, \"confidence\": 85},\n",
			exAction, exPriceField, riskControl.BTCETHMaxLeverage, exNotional, exSL, exTP))
	}
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"wait\", \"wait_bias\": \"short\", \"entry_quality\": 55, \"no_trade_reason\": [\"LIMIT_ANCHOR_SUPPRESSED\", \"MICRO_TREND_NOT_SHORT\"], \"next_trigger\": \"15m 转 down + RECHECK_ALL_HARD_GATES\"},\n")
	sb.WriteString("  {\"symbol\": \"ARUSDT\", \"action\": \"hold\", \"no_trade_reason\": [\"浮亏未达提前平仓条件\", \"15m 结构未破\"], \"management_quality\": 62, \"management_flags\": [\"STRUCTURE_WEAKENING\"]}\n")
	sb.WriteString("]\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Decision Rules\n\n")
	actions := "open_long_limit | open_short_limit | open_long | open_short | close_long | close_short | adjust_stop_loss | partial_close_long | partial_close_short | hold | wait"
	if riskControl.LimitEntryEnabled {
		actions += "(本策略默认限价入场: open_long/open_short 仅三类例外成立时可用)"
	} else {
		actions += "(本策略默认市价入场: 开仓只用 open_long/open_short,不要输出 open_*_limit)"
	}
	sb.WriteString("- `action`: " + actions + "\n")
	sb.WriteString(fmt.Sprintf("- 开仓必填: leverage, position_size_usd, stop_loss, take_profit, confidence(0-100且≥%d);限价开仓另需 price。后端会拒绝低 confidence、下调超限 leverage;`risk_usd`、开仓的 `entry_quality` 和硬门 `blocking_factors` 由后端计算/回填,不要输出。\n", riskControl.MinConfidence))
	if TPMenuEnabled(&riskControl) {
		sb.WriteString("- 开仓另填 `tp_option`(rr_scan.tp_options 的编号,1起,缺省=①;take_profit 复制所选 level)+ `exit_mode`(trend|range|quick,缺省 trend)。二者只能改变程序预计算方案的取舍,不能自造价格;偏离 tp_option=① 或选非常规 exit_mode 时必须在 reasoning 说明 regime 依据。\n")
	}
	sb.WriteString("- 所有数值必须是数字,禁止公式、占位符或单位字符串。\n")
	if riskControl.LimitEntryEnabled {
		sb.WriteString("- 开仓路径只看 hard_entry_gate: `allowed=false` 必须 wait;`allowed=true && limit_allowed=true` 默认输出对应 open_*_limit 并逐字复制 entry_price;`allowed=true && limit_allowed=false && market_exception=true` 才可输出市价 open_*,且 confidence≥80。不存在其他例外。\n")
	} else {
		sb.WriteString("- 开仓路径只看 hard_entry_gate: `allowed=false` 必须 wait;`allowed=true` 直接输出市价 open_long/open_short(不填 price),止损/止盈/仓位按 entry_price(=实时价)计算的计划复制。limit_allowed/limit_buy_price/limit_sell_price 在本策略不使用。\n")
	}
	sb.WriteString("- wait: 无方向优势时 wait_bias 省略;方向明确但暂不可执行时填 long/short。hard_entry_gate.failed 存在时 no_trade_reason 逐项复制阻断码(去重后最多 4 项,按风险优先级取最重者),不要改写成反向观点;程序会映射 blocking_factors。方向性 wait 的 next_trigger 必须是`触发事件 + RECHECK_ALL_HARD_GATES`;每周期自动重评,禁止按天/周搁置。\n")
	sb.WriteString("- **持仓管理动作(浮盈/结构变化时用,优先于全平)**:\n")
	sb.WriteString("  - `adjust_stop_loss`: 只允许收紧到保本或更好;做多新 SL 必须高于旧 SL、低于现价且≥开仓价,做空镜像。\n")
	sb.WriteString("  - `partial_close_long` / `partial_close_short`(部分平仓): 输出 `close_fraction`(0<frac≤0.5)平掉当前剩余仓位的对应比例。后端按初始数量统计程序自动减仓+LLM减仓,总减仓不得超过75%;若剩余名义价值低于 min size 则拒绝部分平仓,全平用 close_*。\n")
	sb.WriteString("- hold 必填 no_trade_reason、management_quality(0-100)和 management_flags;枚举: BREAKEVEN_WARRANTED|PARTIAL_WARRANTED|TRAIL_SUFFICIENT|TREND_INTACT|STRUCTURE_WEAKENING|CHOP_RISK|VOL_SPIKE|EVENT_RISK。若已需要保本或减仓,直接输出 adjust_stop_loss/partial_close_*,不要 hold+flag。\n")
	sb.WriteString("- wait 的 entry_quality 表示偏好方向当前质量;open 只填 confidence,后端会复制为 entry_quality。\n")
	sb.WriteString("- price/stop_loss/take_profit 为0表示不可交易或被抑制,绝不是占位符;未知时省略并 wait。\n\n")

	// ⑦ Static per-strategy blocks (09-18 token audit): the field legend,
	// Strategy Parameters and the scanner/candidate boundary rules are
	// byte-identical every cycle — they live HERE in the system prompt
	// (head of the cached prefix) instead of being re-sent inside the user
	// prompt every cycle. The user prompt then carries per-cycle data only.
	sb.WriteString("# Structured Signal 字段说明（适用于每个 Structured Signal 块）\n")
	sb.WriteString(signalBlockLegendFor(riskControl.LimitEntryEnabled))
	if TPMenuEnabled(&e.config.RiskControl) {
		sb.WriteString("- stop_loss 复制 stop_plan_price;take_profit 从 rr_scan.tp_options 菜单选编号填 tp_option(并把所选 level 复制为 take_profit):level 全部是程序按 stop_plan 同口径算出的结构位,touch_count 只是近30日触及该价位的次数(中性证据,非概率承诺),beyond_structure=超出全部周期结构极值(历史无参考)。usable=false 或菜单为空→引用 MAX_STRUCTURAL_RR=best_rr 并 wait。\n")
	} else {
		sb.WriteString("- stop_loss 复制 stop_plan_price;take_profit 复制 rr_scan.first_rr_ge_target。rr_scan.usable=false 时引用 MAX_STRUCTURAL_RR=best_rr 并 wait,不得改用更远目标。\n")
	}
	sb.WriteString("\n")
	if paramsText := e.strategyParamsText(); paramsText != "" {
		sb.WriteString(paramsText)
		sb.WriteString("\n")
	}
	sb.WriteString("> scanner_hint / 扫描评分 / patterns 均为程序化扫描的辅助证据,不是交易结论,且为扫描时刻的快照(见 generated_at_utc)。方向、时机、是否交易由你综合全部数据独立判断——可以采信、质疑或推翻扫描结果,但必须在推理中给出自己的依据。资金费率尤其如此:暴涨币的 funding 可能在几分钟内漂移数倍,当前状态以各币 Structured Signal 的 derivatives.funding_annualized_pct 为准(程序已按该币真实结算间隔 funding_settle_hours 年化,无需自行换算;与 hint 数字冲突时以 Structured Signal 为准)。\n")
	sb.WriteString("> **short_scan 候选的默认姿态(稳定规则,勿逐次重判)**: short_scan 按涨幅大入选,候选的 1h/4h 结构天然还是多头——scanner 说可空、结构说多头不是偶发冲突,是该引擎的常态。默认姿态: 顶部确认信号(顶背离/假突破/费率回落——后者只认 derivatives.funding_rollover.detected;破 1h EMA20 不算确认,2026-10-07 复盘:仅破 EMA20 的信号胜率 47%、均值为负)之外,**还必须 execution_filter.short_allowed=true(15m 微趋势已转)才允许做空**;仅凭确认信号而 15m 仍 up → 输出 wait + wait_bias=short(wait_state 由程序按 wait_bias 派生,勿输出),触发事件写\"15m 微趋势转 down + RECHECK_ALL_HARD_GATES\"(转 down 只是重评条件,届时 RR/锚点/资金费率等一切硬门重新全过)。entry_timing_gate 开启时这同时是硬规则(15m 逆势 open_short 会被程序拒单);顶部确认部分同样已被程序强制——short_scan 候选缺确认时 hard_entry_gate.failed 给出 SHORT_TOP_CONFIRM_MISSING,直接引用,不要试图用 hint 里的确认字样推翻(2026-10-04)\n\n")

	// 8. Custom Prompt
	if e.config.CustomPrompt != "" {
		sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		sb.WriteString(e.config.CustomPrompt)
		sb.WriteString("\n\n")
		if customControlsFrequency {
			sb.WriteString("Note: 上述个性化策略是本 prompt 唯一交易频率/仓位节奏规则;风险硬门仍不可违反。\n")
		} else {
			sb.WriteString("Note: The above personalized strategy supplements the base rules and cannot violate program-enforced risk controls.\n")
		}
	}

	return sb.String()
}

func promptDefinesFrequency(prompt string) bool {
	p := strings.ToLower(prompt)
	for _, marker := range []string{"频率", "频繁交易", "过度交易", "交易次数", "frequency", "overtrad", "trades per"} {
		if strings.Contains(p, marker) {
			return true
		}
	}
	return false
}

// ============================================================================
// Prompt Building - User Prompt
// ============================================================================

// strategyParamsText renders the "## Strategy Parameters" block — pure
// strategy-config text, byte-identical every cycle (09-18 token audit ⑦:
// it moved into the system prompt where provider-side prompt caching can
// pin it; the user prompt carries only per-cycle data).
func (e *StrategyEngine) strategyParamsText() string {
	var params strings.Builder
	cs := e.config.CoinSource
	{
		if cs.SourceType == "short_scan" || (cs.SourceType == "mixed" && cs.UseShortScan) {
			frPct := cs.ShortScanFundingRatePct
			if frPct <= 0 {
				frPct = 0.03
			}
			// Annualize with the 8h×3×365 convention; non-8h-settlement coins
			// convert with their real interval (scanner_hint.funding_annualized_pct
			// already carries the properly annualized figure).
			annPct := frPct * 1095
			params.WriteString(fmt.Sprintf("- 做空资金费率拥挤(二选一,均为支撑证据而非必要条件): ① 费率年化 > %.1f%%(8h 结算口径即每期费率 > %.4f%%;非 8h 结算的币按真实结算间隔换算)——多头拥挤进行时,以各币 derivatives.funding_annualized_pct 实时值判断; 或 ② funding_rollover 费率刚从高位回落,以各币 derivatives.funding_rollover.detected=true 为唯一依据(程序按结算历史计算: 前 3 个结算期费率高于该阈值、当前前瞻费率已回落;from_annualized_pct 为回落前高点年化)——**禁止从 scanner patterns 里的\"资金费率回落\"字样认定条件②**,hint 是扫描时刻快照,且策略规定 hint 仅作辅助证据;快照没有 funding_rollover 字段 = 历史拉取失败 = 条件② UNKNOWN,按不满足处理(两者都不满足时不要仅因费率理由做空)。磨顶宇宙(universe=\"near_high\")的候选豁免此条件——缓慢磨顶的币费率通常已正常化,其确认信号是 4h 顶背离\n", annPct, frPct))
			// Universe legend, generated through the same resolution the
			// scanner uses (ResolveShortScanHistoryDays) — never a
			// handwritten default.
			if histDays := breakout.ResolveShortScanHistoryDays(cs.ShortScanHistoryDays); histDays > 0 {
				params.WriteString(fmt.Sprintf("- 做空候选宇宙(universe)图例: gainer=24h涨幅榜;hist_gainer=历史涨幅池(近 %d 天每日涨幅 Top20 快照合并去重,币种可能已从当日 24h 榜淡出、处于冲高回落期——回落是进入扫描视野的原因,是否可做空仍看各维度确认信号);near_high=磨顶池(距90日高点<5%%,费率豁免见上条);breakdown=破位池(24h跌幅榜且 4h 趋势向下,顺势反弹做空——它在跌幅榜是进入视野的原因,不是做空结论,是否可做空仍看结构确认与入场时点). universe 仅标注候选来源,不改变打分规则\n", histDays))
			} else {
				params.WriteString("- 做空候选宇宙(universe)图例: gainer=24h涨幅榜;near_high=磨顶池(距90日高点<5%%,费率豁免见上条);breakdown=破位池(24h跌幅榜且 4h 趋势向下,顺势反弹做空——在跌幅榜是进入视野的原因,不是做空结论). universe 仅标注候选来源,不改变打分规则\n")
			}
		}
		if e.config.RiskControl.EntryTimingGate {
			params.WriteString("- 入场时点(程序强制): 最细子小时周期(15m/30m)趋势必须与方向一致——做多需 up/pullback,做空需 down/rally(下跌趋势中的反弹=空头入场窗);range 无动能,顺势入场同样会被拦截\n")
		}
		if pg := PumpGuard4h(&e.config.RiskControl); pg > 0 {
			params.WriteString(fmt.Sprintf("- 暴涨延伸做多确认门:4h 周期的趋势窗口(最近 5 根已闭合 4h K 线,约 20 小时)累计涨幅 ≥%.0f%% 且回踩未确认时,hard_entry_gate.failed 给出 EXTENDED_PUMP_UNCONFIRMED;这是 4h 周期指标,不是最近 4 小时的涨幅,直接采用程序结论\n", pg))
		}
		params.WriteString("- 做多独立确认(可选证据,非必要;轧空条件,程序预计算): 快照 derivatives.long_squeeze.detected=true = 资金费率年化 ≤ −5%(空头付费)+ long_short_account_ratio < 1(散户净空)+ 机构期货净流入 > 0 三者同时成立——作为做多方向的一条独立确认证据,reasoning 可直接引用;detected=false 或字段缺失 = 条件不成立,勿自行换算 FundingRate/比率\n")
		if e.config.RiskControl.LimitEntryEnabled {
			params.WriteString("- 开仓硬门(唯一权威,禁止重算): hard_entry_gate.allowed 是该方向最终可执行权限;false 时只能 wait 并逐项引用 failed。true 时按 limit_allowed/market_exception 选择限价或市价路径;不得从原始指标推翻程序结论\n")
		} else {
			params.WriteString("- 开仓硬门(唯一权威,禁止重算): hard_entry_gate.allowed 是该方向最终可执行权限;false 时只能 wait 并逐项引用 failed。true 时直接市价开仓(open_long/open_short);不得从原始指标推翻程序结论\n")
		}
		if v := EffectiveMaxVendorDivergencePct(&e.config.RiskControl); v > 0 {
			params.WriteString(fmt.Sprintf("- 数据源偏差门:偏差超过%.1f%%时程序输出 VENDOR_DIVERGENCE 并阻断开仓\n", v))
		}
		// 下单公式(框架五): 结构止损 + 风险反推仓位 + 结构位止盈
		rc := e.config.RiskControl
		// 止损措辞与执行端逐字对齐 (09-19 RR audit): stop_plan 由程序按方法论
		// 预计算(结构位+方向性缓冲,夹带),rr_scan/checkRR/模型采用三者同口径。
		if floorMult := rc.SLMinATRMult; floorMult > 0 {
			params.WriteString(fmt.Sprintf("- 止损(程序预计算,逐字采用): stop_plan_price 已按对侧结构+方向缓冲生成,距离带为≥%.1f×ATR(1h)且≤max(2×ATR(4h),8%%);直接复制,STOP_PLAN_NO_STRUCTURE/STOP_PLAN_OUT_OF_BAND 时 wait\n", floorMult))
			params.WriteString(fmt.Sprintf("- 股票代币风险标尺(程序强制): DELL/AAPL/TSLA 等 bstock 的止损与止盈结构都使用4h–1d;止损噪声下限 ≥%.1f×ATR(1d)、方向性缓冲 ×ATR(1d)、带上限 ≤max(2×ATR(1d), 8%%)。ATR(1d)缺失时硬门返回 BSTOCK_DAILY_DATA_UNAVAILABLE,不降级到日内 ATR;15m 仅继续负责入场时点。\n", floorMult))
		} else {
			params.WriteString("- 止损(手工方法论): 本策略未启用噪声下限,快照无 rr_scan/stop_plan——自行按 结构位(最近 support/resistance)外加 0.3-0.5×ATR(1h) 缓冲(空单取上半段 0.4-0.5,多单取下半段 0.3-0.4)定止损,距离 ≤ max(2×ATR(4h), 8%),结构位落在带外时放弃该设置\n")
		}
		params.WriteString(fmt.Sprintf("- 仓位:使用前文唯一公式;min_size.feasible=false 时 wait。最低净RR=%.1f，净RR=(目标价差−往返成本)/(止损价差+往返成本)，往返成本估计 %.0fbps；菜单与执行端采用同一口径。后端对 TP 与 SL 使用同一 0.05%% 容差强制吸附到计划值\n", rc.MinRiskRewardRatio, rc.EffectiveEntryRoundTripCostBps()))
		if TPMenuEnabled(&e.config.RiskControl) {
			params.WriteString(fmt.Sprintf("- 止盈(菜单选择): rr_scan.tp_options 是程序预计算的止盈方案菜单(近/中/远结构位,各附 rr、touch_count=近30日触及次数、beyond_structure)。开仓时选一个编号填 tp_option(缺省=①最近合格位;偏离默认需在 reasoning 给出依据:趋势 regime、动能、上方结构强度),同时把所选 level 复制为 take_profit。usable=false 时引用 MAX_STRUCTURAL_RR=best_rr 并 wait,菜单为空同理\n"))
			params.WriteString(fmt.Sprintf("- 出场模式(开仓必选,缺省 trend): exit_mode=trend 趋势模式(止盈只平一部分,剩余移动止损跑单,适合顺势延续行情)/exit_mode=range 震荡模式(到目标位全平,不跑单,适合区间震荡)/exit_mode=quick 快进快出(到目标位全平+开仓超过 %.0f 小时仍浮亏则程序时间止损,适合事件驱动/脉冲行情)。regime 定性判断由你做,参数由程序按模式执行;持仓中途不可改模式\n", float64(TimeStopHours(&e.config.RiskControl))))
		} else {
			params.WriteString(fmt.Sprintf("- 止盈:逐字复制 rr_scan.first_rr_ge_target;usable=false 时引用 MAX_STRUCTURAL_RR=best_rr 并 wait,不得改用更远目标。stop_plan_price 与 first_rr_ge_target 必须成对采用\n"))
		}
		var tpParts []string
		armR := BreakevenArmR(&e.config.RiskControl)
		if armR > 0 {
			beOff := ProfitLockBreakevenOffsetR(&e.config.RiskControl)
			tpParts = append(tpParts, fmt.Sprintf("浮盈达 %.1fR 时程序先把止损移至开仓价+%.2fR(不减仓)", armR, beOff))
		}
		if lockR := ProfitLockRMult(&e.config.RiskControl); lockR > 0 {
			// 保本位措辞从配置求值(09-21 实验: +0.2R 锁微利 vs 纯保本)
			beTxt := "开仓价保本"
			if beOff := ProfitLockBreakevenOffsetR(&e.config.RiskControl); beOff > 0 {
				beTxt = fmt.Sprintf("开仓价+%.2fR(锁定一档微利,防噪声扫回平手)", beOff)
			}
			if e.config.RiskControl.TrimYieldsToLock() {
				tpParts = append(tpParts, fmt.Sprintf("浮盈达 %.0fR 时程序自动市价减仓 50%%,并确保止损至%s;若早期保本档已设到同一价格,此档只执行减仓,不重复移止损", lockR, beTxt))
			} else {
				// 分工模式: 减仓档独立生效,锁只管保本(CAPUSDT 09-24)
				// R 档(tp_trim_at_r>0)优先——ROE 档随杠杆漂移,已弃用
				if trimR := TpTrimAtR(&e.config.RiskControl); trimR > 0 {
					tpParts = append(tpParts, fmt.Sprintf("浮盈达 %.1fR(=初始止损距离的 %.1f 倍,与杠杆无关)程序自动市价减仓 1/3(一次);该档执行前必须先将止损收紧到保本或更好", trimR, trimR))
				} else if trimR == 0 {
					if trim := TpTrimProfitPct(&e.config.RiskControl); trim > 0 {
						tpParts = append(tpParts, fmt.Sprintf("浮盈达 %.0f%%(杠杆后 ROE;价格涨幅=该值÷杠杆,折 R=价格涨幅÷初始止损距离,随止损宽度浮动)程序自动市价减仓 1/3(一次);该兼容档执行前必须先将止损收紧到保本或更好", trim))
					}
				}
				// 09-29 review #1a: with the early breakeven arm enabled the
				// stop already sits at the lock's breakeven price, so the 1R
				// tier in division-of-labor mode has NO new action — the old
				// "程序把止损移至…" wording described a no-op as if it moved
				// something.
				if armR > 0 {
					tpParts = append(tpParts, fmt.Sprintf("浮盈达 %.0fR:此档无新增动作——止损已由 %.1fR 档移至%s,本模式不减仓(减仓由 ROE 档负责)", lockR, armR, beTxt))
				} else {
					tpParts = append(tpParts, fmt.Sprintf("浮盈达 %.0fR(1×初始止损距离)程序把止损移至%s——此档只保本不再减仓,剩余仓位奔向结构位止盈", lockR, beTxt))
				}
			}
		} else if trimR := TpTrimAtR(&e.config.RiskControl); trimR > 0 {
			tpParts = append(tpParts, fmt.Sprintf("浮盈达 %.1fR(与杠杆无关)程序自动市价减仓 1/3(一次),减仓前先将止损收紧到保本或更好", trimR))
		} else if trimR == 0 {
			if trim := TpTrimProfitPct(&e.config.RiskControl); trim > 0 {
				tpParts = append(tpParts, fmt.Sprintf("浮盈达 %.0f%%(杠杆后)程序自动市价减仓 1/3(一次),减仓前先将止损收紧到保本或更好", trim))
			}
		}
		// 分批止盈+趋势跑单(09-21 实验): 止盈触发只平一部分,剩余由移动止损
		// 接管——只在移动止损开启时生效(trader 端 effectiveTPCloseFraction
		// 同条件收拢为全平)。
		tpFrac := TPCloseFraction(&e.config.RiskControl)
		if e.config.RiskControl.TrailingStopEnabled && tpFrac < 1.0 {
			tpParts = append(tpParts, fmt.Sprintf("exit_mode=trend(趋势模式,默认)时:结构位止盈触发程序只平当时剩余仓位的 %.0f%%(基数=触发时的仓位,不是初始仓位;若此前已有减仓,按剩余量计),剩余继续由 2×ATR 移动止损接管(趋势跑单,利润奔跑;强趋势冲破止盈位后的延续行情由它捕捉);exit_mode=range/quick 时目标位全平,无跑单", tpFrac*100))
		}
		// R 档优先: >0 按 R 渲染, <0 = 档关闭(不再渲染), 0 = legacy ROE 字段
		if tpFullR := TpFullAtR(&e.config.RiskControl); tpFullR > 0 {
			tpParts = append(tpParts, fmt.Sprintf("≥%.1fR(与杠杆无关)程序自动全部平仓", tpFullR))
		} else if tpFullR < 0 {
			// full 档显式关闭: 全平职责归结构位 TP 算法单 + 移动止损 + 回撤保护
		} else if tpFull := TpFullProfitPct(&e.config.RiskControl); tpFull > 0 {
			tpParts = append(tpParts, fmt.Sprintf("≥%.0f%%(杠杆后 ROE;价格涨幅=该值÷杠杆)程序自动全部平仓", tpFull))
		}
		if len(tpParts) > 0 {
			params.WriteString("- 程序自动止盈阶梯(强制,独立于你的 TP 规划): " + strings.Join(tpParts, ";") + "。这些由程序按周期自动执行,你无需输出 close 来实现;你的止盈规划仍按结构位给出。**75% 累计减仓上限只约束你发起的 partial_close,程序阶梯(TP 算法单/1R/ROE 档)不受其限**\n")
		}
		if sp := MaxSpreadPct(&e.config.RiskControl); sp > 0 {
			params.WriteString(fmt.Sprintf("- 点差门(程序强制): 盘口买卖价差 > %.2f%%(占中间价)的币种,任何 open_*/open_*_limit 都会被程序拒单——薄盘口的点差会吃掉限价优势并抬高市价成本,这类币直接放弃\n", sp))
		}
		params.WriteString("- 注意: 资金费率极端拥挤时禁止逆势扛单;目标位越过 structure_high/low(历史新高新低区)时注明无历史阻力参考、不确定性大\n")
		if e.config.RiskControl.VolTargetEnabled {
			params.WriteString("- 波动率调仓(程序自动): 每周期按 权益×单笔风险%÷ATR(1h)% 重算目标仓位,80/120 滞后带外才调——>120% 程序自动减仓,<80% 时你可在信号仍有效的前提下评估加仓。不要因波动率变化去动 SL/TP\n")
		}
		if e.config.RiskControl.CloseRejectBreakoutPct > 0 {
			params.WriteString(fmt.Sprintf("- 浮亏平仓限制(程序强制): 持仓浮亏时,若最近的反向结构位(做多看上方 resistance/structure_high、做空看下方 support)距离现价 < %.1f%% 且 15m 结构未破坏(未破支撑/未破阻力),\"被阻力拒绝/被支撑拒绝\"不构成平仓理由,该 close 会被程序拦截——给突破留空间;你的合法离场路径是 15m 结构实际破位、止损触发,或浮盈状态下的正常止盈\n", e.config.RiskControl.CloseRejectBreakoutPct))
		}
		params.WriteString("- 保护单看门狗(程序强制): 每周期核对全部持仓的止损/止盈挂单,缺失时按开仓计划价自动补挂(止盈只平部分的剩余趋势跑单、以及止盈距离走完转跟踪止损的仓位除外——它们的出场由移动止损接管)——保护单由程序保障,你只负责按结构规划输出 SL/TP 数值\n")
		if EarlyCloseHours(&e.config.RiskControl) > 0 {
			params.WriteString(fmt.Sprintf("- 提前平仓限制(程序强制): 持仓不足 %dh 时,close 还必须有至少 2 根逆持仓方向的已收盘 1h K线。该门与浮亏结构位门、最短持仓门串联(AND):所有当前适用的门都通过才放行;任一门拦截就 hold。交易所 SL/TP 与回撤保护不走这些 AI close 门。持仓满 %dh 后不再适用本时间门\n", EarlyCloseHours(&e.config.RiskControl), EarlyCloseHours(&e.config.RiskControl)))
		}
		if e.config.RiskControl.MinHoldMinutes > 0 {
			params.WriteString(fmt.Sprintf("- 最短持仓限制(程序强制): 持仓不足 %d 分钟时,close/partial_close 会被程序拦截(现价已触及记录止损的硬退出除外)——不要在时间未到且无 1h 逆势证据时输出平仓动作\n", e.config.RiskControl.MinHoldMinutes))
		}
		if e.config.RiskControl.OpenRejectSupplyPct > 0 {
			if e.config.RiskControl.LimitEntryEnabled {
				params.WriteString("- 锚点呼吸空间已并入 hard_entry_gate;直接采用 entry_price/limit_allowed,不要用 limit_entry_offset_pct 自行重算\n")
			} else {
				params.WriteString("- 锚点呼吸空间已并入 hard_entry_gate;直接采用其结论与 entry_price,不要自行重算\n")
			}
		}
		if e.config.RiskControl.AccountMaxDrawdownPct > 0 {
			params.WriteString(fmt.Sprintf("- 账户级熔断(程序强制): 账户净值自初始值回撤 ≥ %.1f%% 时进入只减仓模式,一切新开仓被程序拦截;此时优先保护本金,减少交易频率\n", e.config.RiskControl.AccountMaxDrawdownPct))
		}
		if v := e.config.RiskControl.EffectiveMaxAccountRiskPct(); v > 0 {
			params.WriteString(fmt.Sprintf("- 账户风险敞口上限(程序强制): 全部持仓的止损风险(数量×|开仓价−止损|,无保护单的仓位按止损带上限 %.0f%% 最坏估计)加上本单风险,合计不得超过权益的 %.1f%%——仓位数量上限看不见相关性,五个同向山寨止损等于一个大仓;超限时 open 被拒,优先平掉浮亏的 AI 仓腾出敞口额度(手动仓程序无法平掉,其风险占用不可腾出)。有效并发上限由敞口决定而非 Max Positions:≈ 敞口上限 ÷ 单笔止损风险,MarginUsage 与仓位价值上限也可能先约束\n", UnprotectedStopWorstCasePct, v))
		}
		if v := e.config.RiskControl.EffectiveMaxNetDirectionalRiskPct(); v > 0 {
			params.WriteString(fmt.Sprintf("- 净方向风险上限(程序强制): |多头止损风险−空头止损风险|不得超过权益的 %.1f%%,包含持仓、挂单和本周期已放行决策\n", v))
		}
		if v := e.config.RiskControl.EffectiveDailyMaxLossPct(); v > 0 {
			params.WriteString(fmt.Sprintf("- 日内亏损熔断(程序强制): 净值较今日开盘基线回撤 ≥ %.1f%% 时,当日所有新开仓被拦截至下一个 UTC 日,平仓/止损不受影响——连亏日强制降频,不是可选建议\n", v))
		}
		if noOpen := e.config.RiskControl.StockWeekendNoOpen; noOpen == nil || *noOpen {
			params.WriteString("- 股票类代币周末禁开新仓(程序强制): DELL/SKHY 等 bstock 仅在美东周六/周日禁止新开仓;美东周一至周五的盘前、正常盘、盘后和夜间均允许按 15m 执行门评估开仓。周末只允许 hold/close,不输出任何 open_*\n")
		}
		if v := e.config.EffectiveUSStockSessionBoostPct(); v > 0 {
			params.WriteString(fmt.Sprintf("- 美股盘中时段股票标的加权(程序施加,美东周一至五 09:30–16:00 生效): 美股股票类代币(AAPL/TSLA/SPY 等 EQUITY 代币)在 short_scan 候选中的得分已按 +%.0f%% 加权后再截断——它们跟随标的正股交易时段,波动相对加密货币更低、历史胜率更高;候选 reasons 里的「美股盘中时段加权」即此标记。注意:加权改变排序不改变闸门,RR/止损带/时点门照常执行;且此类标的流动性薄于主流加密,点差门(max_spread_pct)会自动拦截过宽盘口,滑点预期要按更宽计\n", v))
		}
		params.WriteString("- 共识、历史和连亏限制均已进入 hard_entry_gate.failed(CONSENSUS_OPPOSED/POOR_HISTORY/LOSS_STREAK_BANNED);只引用程序码,不要自行推测\n")
	}
	if params.Len() == 0 {
		return ""
	}
	return "## Strategy Parameters\n" + params.String() + "\n"
}

// BuildUserPrompt builds User Prompt based on strategy configuration
// dailyLossState renders the daily-loss halt's remaining buffer (user audit
// 2026-10-01: the −10% day-start baseline was never shown, so the model
// could not see how much of the daily budget was already spent). Empty when
// no day anchor exists yet or the halt is disabled.
func (e *StrategyEngine) dailyLossState(ctx *Context) string {
	if ctx.DayStartEquityUSDT <= 0 {
		return ""
	}
	capPct := e.config.RiskControl.EffectiveDailyMaxLossPct()
	if capPct <= 0 {
		return ""
	}
	dayPct := (ctx.Account.TotalEquity - ctx.DayStartEquityUSDT) / ctx.DayStartEquityUSDT * 100
	return fmt.Sprintf(" | DailyLoss: day-start %.2f (今日 %+.1f%%, 日内熔断线 −%.0f%%)", ctx.DayStartEquityUSDT, dayPct, capPct)
}

func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// Market sentiment composite (user directive 2026-09-27): three
	// independent sources at the HEAD of the user prompt, with an explicit
	// combine-don't-overindex rule. Rendered empty when every source failed —
	// the block disappears rather than showing an empty shell.
	if sentiment := market.GetMarketSentiment(); sentiment != nil {
		if body := sentiment.Render(); body != "" {
			sb.WriteString("## 大盘情绪(组合解读,不要只看单一数字)\n")
			sb.WriteString(body + "\n")
			sb.WriteString("组合解读规则: ①三源同向极端(加密贪婪>80+美股>75+多空账户比>2+资金费年化>50%)=拥挤区,逆向风险最高,新开仓一律保守并在 reasoning 说明;②源间背离时以价格结构与硬门判定为准,情绪只作择时背景;③情绪是慢变量,本身不构成开/平仓理由,也不覆盖任何程序硬门。缺数来源按剩余来源判断。`sentiment_regime`/`sentiment_trade_effect` 是程序按上述规则算好的组合枚举,直接引用即可,不要再自行组合;各来源的[数据日]与拉取时间已单独标注,以它们判断新旧,不要假设数据是刚刚的。\n\n")
		}
	}
	// CFTC is dated market context, so it belongs in the per-cycle user
	// prompt. Keeping it out of the system prompt preserves the stable prefix
	// used by provider prompt caches.
	if openbb.Available() {
		if cot := openbb.CotBitcoinCached(context.Background()); cot != nil {
			sb.WriteString(fmt.Sprintf("## CFTC 比特币期货持仓周报 [%s]\n全部未平仓 %s 张 | 投机类净多 %s 张 | 套保类净多 %s 张。慢变量背景,不构成开/平仓触发,不得覆盖 hard_entry_gate。\n\n",
				cot.ReportDate, openbb.Usd(cot.OpenInterestAll), openbb.Usd(cot.SpeculatorsNetLong), openbb.Usd(cot.HedgersNetLong)))
		}
	}

	// System status
	sb.WriteString(fmt.Sprintf("Time: %s | Period: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC market
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD(12/26线,价格绝对值口径): %.4f | RSI(7): %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// Account information. "Available" is DERIVED (equity − Σposition margin)
	// rather than the exchange availableBalance: Binance withholds margin for
	// things this prompt cannot see (manual orders elsewhere on the account,
	// maintenance buffers), which printed 52.62 "Balance" next to 77.32
	// equity + 13.94 position margin on 09-18 — unreconcilable from every
	// other number here, and it silently shrank any model-side sizing math
	// by a third. The breaker state is program-truth (same formula as the
	// executor gate), so the model never has to guess whether reduce-only
	// mode is active from the stats block's HISTORICAL max-drawdown.
	available := ctx.Account.TotalEquity - ctx.Account.MarginUsed
	availablePct := 0.0
	if ctx.Account.TotalEquity > 0 {
		availablePct = available / ctx.Account.TotalEquity * 100
	}
	breakerState := ""
	if ctx.InitialBalanceUSDT > 0 {
		if acctBreaker := e.config.RiskControl.AccountMaxDrawdownPct; acctBreaker > 0 {
			ddPct := (ctx.InitialBalanceUSDT - ctx.Account.TotalEquity) / ctx.InitialBalanceUSDT * 100
			if ddPct >= acctBreaker {
				breakerState = fmt.Sprintf(" | AccountBreaker ON (drawdown %.1f%% ≥ %.1f%% vs initial — reduce-only, all opens blocked)", ddPct, acctBreaker)
			} else {
				breakerState = fmt.Sprintf(" | AccountBreaker OFF (equity vs initial %+.1f%%)", (ctx.Account.TotalEquity-ctx.InitialBalanceUSDT)/ctx.InitialBalanceUSDT*100)
			}
		}
	}
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance (equity−margin) %.2f (%.1f%%) | PnL %+.2f%% (vs initial %.2f USDT) | MarginUsage %.1f%% | Positions %d%s\n\n",
		ctx.Account.TotalEquity,
		available,
		availablePct,
		ctx.Account.TotalPnLPct,
		ctx.InitialBalanceUSDT,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount,
		breakerState+e.dailyLossState(ctx)))

	// Recently completed orders (placed before positions to ensure visibility)
	if len(ctx.RecentOrders) > 0 {
		sb.WriteString("## Recent Completed Trades\n")
		for i, order := range ctx.RecentOrders {
			resultStr := "Profit"
			if order.RealizedPnL < 0 {
				resultStr = "Loss"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | Notional %.2f USDT | Fee %.3f | %s: %+.2f USDT (%+.2f%%,价格回报口径) | %s→%s (%s)\n",
				i+1, order.Symbol, order.Side,
				order.EntryPrice, order.ExitPrice,
				order.PositionValue, order.Fee,
				resultStr, order.RealizedPnL, order.PnLPct,
				order.EntryTime, order.ExitTime, order.HoldDuration))
		}
		sb.WriteString("\n")
	}

	// Historical trading statistics (helps AI understand past performance)
	if ctx.TradingStats != nil && ctx.TradingStats.TotalTrades > 0 {
		// Get language from strategy config
		lang := e.GetLanguage()

		// Win/Loss ratio
		var winLossRatio float64
		if ctx.TradingStats.AvgLoss > 0 {
			winLossRatio = ctx.TradingStats.AvgWin / ctx.TradingStats.AvgLoss
		}

		// Stats window label: the numbers below only cover trades closed within
		// the window (config stats_window_days); 0 = full history.
		statsWindowDays := 0
		if ctx.TradingStats != nil {
			statsWindowDays = ctx.TradingStats.WindowDays
		}
		windowLabel := "全部历史"
		if ctx.TradingStats.WindowDays > 0 {
			windowLabel = fmt.Sprintf("近%d天", ctx.TradingStats.WindowDays)
		}
		// Strategy health is language-independent program truth. Compute it
		// once, then localize only the prose below so switching the prompt
		// language cannot change the model's risk posture.
		edge := ResolveStrategyEdge(ctx.TradingStats)
		// PF is undefined without a loser; print a marker, not a fake 0.00.
		noLoss := noLosingTrade(ctx.TradingStats)
		pfZH := fmt.Sprintf("%.2f", ctx.TradingStats.ProfitFactor)
		pfEN := pfZH
		thinZH, thinEN := "", ""
		if negativeEdgeSuppressed(ctx.TradingStats) {
			thinZH = fmt.Sprintf("(样本不足: %d<%d 笔,不判负期望)", ctx.TradingStats.TotalTrades, MinNegativeEdgeTrades)
			thinEN = fmt.Sprintf("(thin sample: %d<%d trades, negative-edge not assessed)", ctx.TradingStats.TotalTrades, MinNegativeEdgeTrades)
		}
		if noLoss {
			pfZH, pfEN = "n/a(窗口内无亏损单)", "n/a (no losing trade in window)"
		}
		expR := expectancyR(ctx.TradingStats.WinRate, ctx.TradingStats.AvgWin, ctx.TradingStats.AvgLoss)
		expSourceZH := "估算,亏损按-1R假设"
		expSourceEN := "estimated, losses assumed -1R"
		if ctx.TradingStats.MeasuredRSamples >= MinMeasuredRSamples {
			expR = ctx.TradingStats.MeasuredExpectancyR
			expSourceZH = fmt.Sprintf("实测,%d笔R", ctx.TradingStats.MeasuredRSamples)
			expSourceEN = fmt.Sprintf("measured,%d R samples", ctx.TradingStats.MeasuredRSamples)
		}

		if lang == LangChinese {
			if statsWindowDays > 0 {
				sb.WriteString(fmt.Sprintf("## 历史交易统计(滚动窗口;账户行 PnL 为自启动以来累计,两者口径不同;本表只统计 %s 之后平仓的交易)\n",
					time.Now().UTC().AddDate(0, 0, -statsWindowDays).Format("2006-01-02")))
			} else {
				sb.WriteString("## 历史交易统计(全部历史;账户行 PnL 为自启动以来累计,两者口径不同)\n")
			}
			sb.WriteString(fmt.Sprintf("统计窗口: %s | 总交易: %d 笔 | 胜率: %.1f%% | 盈利因子: %s | 夏普比率: %.2f | 盈亏比: %.2f\n",
				windowLabel,
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.WinRate,
				pfZH,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			// 净口径(user audit 2026-10-01): 总盈亏/平均盈亏已逐笔扣除手续费;
			// 最大回撤来自权益曲线快照,峰谷日期让它可对账(可能是早于初始基线
			// 手动重设的真实历史深回撤)。
			if ctx.TradingStats.MaxDDPeakAt != "" && ctx.TradingStats.MaxDDTroughAt != "" {
				sb.WriteString(fmt.Sprintf("总盈亏(净手续费): %+.2f USDT | 手续费合计: %.2f | 平均盈利: +%.2f | 平均亏损: -%.2f | 最大回撤: %.1f%%(权益曲线峰值 %.2f @ %s → 谷值 %.2f @ %s,窗口内历史;当前净值 vs 初始见账户行 AccountBreaker)\n",
					ctx.TradingStats.TotalPnL,
					ctx.TradingStats.TotalFee,
					ctx.TradingStats.AvgWin,
					ctx.TradingStats.AvgLoss,
					ctx.TradingStats.MaxDrawdownPct,
					ctx.TradingStats.MaxDDPeakEquity,
					ctx.TradingStats.MaxDDPeakAt,
					ctx.TradingStats.MaxDDTroughEquity,
					ctx.TradingStats.MaxDDTroughAt))
			} else {
				sb.WriteString(fmt.Sprintf("总盈亏(净手续费): %+.2f USDT | 手续费合计: %.2f | 平均盈利: +%.2f | 平均亏损: -%.2f | 最大回撤: %.1f%%(窗口内历史序列峰值,非当前净值回撤;当前净值 vs 初始见账户行 AccountBreaker)\n",
					ctx.TradingStats.TotalPnL,
					ctx.TradingStats.TotalFee,
					ctx.TradingStats.AvgWin,
					ctx.TradingStats.AvgLoss,
					ctx.TradingStats.MaxDrawdownPct))
			}

			// expectancy_r: MEASURED from journal R multiples when enough
			// planned-stop trades exist; the old estimate assumed every loser
			// = −1R while the real baseline measured −0.70R — a systematic
			// pessimism the model read every cycle (E1, QUANT_REVIEW 09-22).
			// The label states which caliber rendered.
			sb.WriteString(fmt.Sprintf("strategy_health: %s%s (PF %s, expectancy_r %+.2f [%s], avg_win_r %+.2f, avg_loss_r %.2f, 窗口 %s)\n",
				edge, thinZH, pfZH, expR, expSourceZH, ctx.TradingStats.MeasuredAvgWinR, ctx.TradingStats.MeasuredAvgLossR, windowLabel))
			if ctx.TradingStats.MeasuredRSamples > 0 && ctx.TradingStats.MeasuredRSamples < ctx.TradingStats.TotalTrades {
				// R 样本覆盖率(user audit 2026-10-01: 165 笔 R 对 218 笔 USDT
				// 统计,差值 = 无计划止损的手动/外部仓等,两套数字不可直接互推)
				sb.WriteString(fmt.Sprintf("(R 样本口径: 仅 %d/%d 笔有计划止损的交易计入 R 统计,与上方 USDT 统计样本不同,不可直接互推胜率)\n",
					ctx.TradingStats.MeasuredRSamples, ctx.TradingStats.TotalTrades))
			}
			if edge == "NEGATIVE_EDGE" {
				// The 09-27 review: "证据极强/RR 明显占优" carried no numbers,
				// so open and wait were both arguable (BRUSDT passed the plain
				// min_rr at score −50 with net-losing history). The gate now
				// enforces the conditions — this prose only NAMES them, and
				// always through the same resolvers the gate reads.
				clauses := []string{
					fmt.Sprintf("|directional_score| ≥ %.0f(NEG_EDGE_SCORE_*)", NegativeEdgeMinScore(&e.config.RiskControl)),
					"trend_tf+regime_tf 的 EMA 方向均与交易方向一致(NEG_EDGE_TREND_MISALIGNED)",
				}
				if rr := NegativeEdgeMinRR(&e.config.RiskControl); rr > 0 {
					clauses = append(clauses, fmt.Sprintf("first_target_rr ≥ %.1f(NEG_EDGE_RR_*)", rr))
				}
				if NegativeEdgeBlockLosingSymbol(&e.config.RiskControl) {
					clauses = append(clauses, "该币近 ≥2 笔净亏禁开(NEG_EDGE_LOSING_SYMBOL)")
				}
				sb.WriteString(fmt.Sprintf("⚠️ 当前策略整体无正期望(窗口 %s 内 PF<0.9):NEGATIVE_EDGE 附加开仓门已程序化并入各币 hard_entry_gate 的 failed 列表,不存在例外路径,开仓需全部满足: %s;未全过的设置一律 wait 并在 no_trade_reason 引用对应阻断码,已有持仓按管理规则 hold/close\n", windowLabel, strings.Join(clauses, ";")))
			}
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("表现: 良好 - 保持当前策略\n")
			} else if !noLoss && ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("表现: 需改进 - 提高盈亏比，优化止盈止损\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("表现: 风险偏高 - 减少仓位，控制回撤\n")
			} else {
				sb.WriteString("表现: 正常 - 有优化空间\n")
			}
		} else {
			enWindow := "all history"
			if ctx.TradingStats.WindowDays > 0 {
				enWindow = fmt.Sprintf("last %d days", ctx.TradingStats.WindowDays)
			}
			// enWindow in the heading too — it used to hardcode "30d" while the
			// data line below honored stats_window_days, so an English-model
			// reading a non-30d window saw a mislabeled caliber (E3, QUANT_REVIEW 09-22).
			sb.WriteString(fmt.Sprintf("## Historical Trading Statistics (%s rolling window; the account PnL is a since-start cumulative — different bases)\n", enWindow))
			sb.WriteString(fmt.Sprintf("Window: %s | Total Trades: %d | Profit Factor: %s | Sharpe: %.2f | Win/Loss Ratio: %.2f\n",
				enWindow,
				ctx.TradingStats.TotalTrades,
				pfEN,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("Total PnL: %+.2f USDT | Avg Win: +%.2f | Avg Loss: -%.2f | Max Drawdown: %.1f%% (historical series peak within window, NOT current equity drawdown — see AccountBreaker in the account line)\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))
			sb.WriteString(fmt.Sprintf("strategy_health: %s%s (PF %s, expectancy_r %+.2f [%s], avg_win_r %+.2f, avg_loss_r %.2f, window %s)\n",
				edge, thinEN, pfEN, expR, expSourceEN, ctx.TradingStats.MeasuredAvgWinR, ctx.TradingStats.MeasuredAvgLossR, enWindow))
			if edge == "NEGATIVE_EDGE" {
				clauses := []string{
					fmt.Sprintf("|directional_score| >= %.0f (NEG_EDGE_SCORE_*)", NegativeEdgeMinScore(&e.config.RiskControl)),
					"trend_tf + regime_tf EMA direction both aligned with the trade (NEG_EDGE_TREND_MISALIGNED)",
				}
				if rr := NegativeEdgeMinRR(&e.config.RiskControl); rr > 0 {
					clauses = append(clauses, fmt.Sprintf("first_target_rr >= %.1f (NEG_EDGE_RR_*)", rr))
				}
				if NegativeEdgeBlockLosingSymbol(&e.config.RiskControl) {
					clauses = append(clauses, "symbols with >=2 recent net-losing trades are blocked (NEG_EDGE_LOSING_SYMBOL)")
				}
				sb.WriteString(fmt.Sprintf("⚠️ The strategy has no positive edge in the %s window (PF<0.9): the NEGATIVE_EDGE entry gate is PROGRAM-ENFORCED inside each symbol's hard_entry_gate failed list with no exception path — an open requires ALL of: %s. Setups that fail any condition must wait and cite the codes in no_trade_reason; existing positions follow hold/close management rules.\n", enWindow, strings.Join(clauses, "; ")))
			}

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("Performance: GOOD - maintain current strategy\n")
			} else if !noLoss && ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("Performance: NEEDS IMPROVEMENT - improve win/loss ratio, optimize TP/SL\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("Performance: HIGH RISK - reduce position size, control drawdown\n")
			} else {
				sb.WriteString("Performance: NORMAL - room for optimization\n")
			}
		}
		sb.WriteString("\n")
	}

	// Review-derived rules (hard rules + soft lessons from past trade reviews)
	if ctx.RulesText != "" {
		sb.WriteString(ctx.RulesText)
	}

	// (⑦) The Structured Signal field legend moved to the system prompt —
	// static text, part of the cached prefix.

	// Position information
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(e.formatPositionInfo(i+1, pos, ctx))
		}
	} else {
		sb.WriteString("Current Positions: None\n\n")
	}

	// Candidate coins (exclude coins already in positions to avoid duplicate data)
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		// Normalize symbol to handle both "ETH" and "ETHUSDT" formats
		normalizedSymbol := market.Normalize(pos.Symbol)
		positionSymbols[normalizedSymbol] = true
	}

	// History-demotion: candidates where this trader has been repeatedly
	// losing recently render LAST — the ordering itself is soft evidence.
	ordered := make([]CandidateCoin, 0, len(ctx.CandidateCoins))
	var demoted []CandidateCoin
	for _, coin := range ctx.CandidateCoins {
		if recentHistoryNote(ctx, coin.Symbol) != "" {
			demoted = append(demoted, coin)
		} else {
			ordered = append(ordered, coin)
		}
	}
	ordered = append(ordered, demoted...)
	// Header count must equal the number of blocks actually rendered below:
	// position coins are shown in the positions section and coins without
	// market data render no block (09-18 audit #3: "9 coins" header over 8
	// blocks, because MarketDataMap also held the held-position symbol).
	rendered := make([]CandidateCoin, 0, len(ordered))
	for _, coin := range ordered {
		if positionSymbols[market.Normalize(coin.Symbol)] {
			continue
		}
		if _, hasData := ctx.MarketDataMap[coin.Symbol]; !hasData {
			continue
		}
		rendered = append(rendered, coin)
	}
	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d coins)\n\n", len(rendered)))
	// 09-27 review #5: double-blocked candidates collect into ONE compact
	// table with the mechanical-wait instruction stated once — the per-coin
	// repetition spent tokens re-telling every blocked coin how to wait.
	var blockedCoins []blockedCoinRow
	displayedCount := 0
	for _, coin := range rendered {
		marketData := ctx.MarketDataMap[coin.Symbol]
		var quantData *QuantData
		if ctx.QuantDataMap != nil {
			quantData = ctx.QuantDataMap[coin.Symbol]
		}
		sourceTags := e.formatCoinSourceTag(coin.Sources)
		sig := e.computeCoinSignal(marketData, quantData, ctx, &coin)
		// 09-18 token audit ①: a candidate whose BOTH directions carry a
		// no-exception blocker (RR_MAX / MICRO_TREND / DATA_INSUFFICIENT /
		// BSTOCK_DAILY_DATA_UNAVAILABLE / MIN_SIZE / LOSS_STREAK /
		// STOCK_WEEKEND / VENDOR_DIVERGENCE) can
		// only ever produce a mechanical wait — reading its full 3.5-4.5k
		// chars of JSON adds nothing. Compress to a table row; a direction
		// whose only block is LIMIT_ANCHOR_SUPPRESSED keeps the full JSON
		// (the market-order exception paths read evidence from it).
		// GateStates / RRCeilings bookkeeping already ran inside
		// computeCoinSignal.
		if sig != nil && bothDirectionsHardBlocked(sig) {
			blockedCoins = append(blockedCoins, blockedCoinRow{symbol: coin.Symbol, tags: sourceTags, sig: sig})
			continue
		}
		displayedCount++
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		// Scanner output is EVIDENCE, not a conclusion: neutral structured
		// hint, no prescriptive direction instruction.
		for _, src := range coin.Sources {
			if src != "short_scan" || coin.ShortScore <= 0 {
				continue
			}
			hint := map[string]interface{}{
				"source":                 "short_scanner",
				"direction_bias":         "short",
				"score":                  coin.ShortScore,
				"confidence":             coin.ShortGrade,
				"patterns":               coin.ShortReasons,
				"funding_annualized_pct": coin.ShortFundingAnn,
			}
			if coin.ShortUniverse != "" {
				hint["universe"] = coin.ShortUniverse
			}
			if coin.ShortScanAtMs > 0 {
				hint["generated_at_utc"] = time.UnixMilli(coin.ShortScanAtMs).UTC().Format("2006-01-02T15:04:05Z")
			}
			hintJSON, _ := json.Marshal(hint)
			sb.WriteString("scanner_hint(程序化扫描,仅辅助证据,非交易结论): " + string(hintJSON) + "\n\n")
			break
		}
		// Concentration + own-history annotations — neutral evidence lines,
		// same "可推翻但需给依据" contract as scanner_hint.
		wroteNote := false
		for _, w := range concentrationWarnings(ctx, coin.Symbol) {
			sb.WriteString("⚠️ " + w + "\n")
			wroteNote = true
		}
		if note := recentHistoryNote(ctx, coin.Symbol); note != "" {
			sb.WriteString("⚠️ " + note + "\n")
			wroteNote = true
		}
		if wroteNote {
			sb.WriteString("\n")
		}
		if sig != nil {
			sb.WriteString(e.renderSignalBlock(sig))
		} else {
			sb.WriteString(e.formatMarketData(marketData, quantData, ctx, &coin))
		}
		sb.WriteString("\n")
	}
	// The blocked table renders AFTER the full candidates so numbering stays
	// continuous; loss-heavy coins were already demoted to the tail of
	// `rendered`, and mechanically-blocked coins belong at the very end.
	if len(blockedCoins) > 0 {
		sb.WriteString(renderBlockedCoinTable(blockedCoins, displayedCount))
	}
	sb.WriteString("\n")

	// 09-18 token audit ⑥: the three ranking blocks self-describe as
	// context-only ("严禁参与 entry/SL/TP/RR 精算") — render them as a few
	// regime lines instead of full tables. Full tables available in the
	// web UI; the model never needed them for decisions.
	if line := e.formatMarketContext(ctx); line != "" {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")
	sb.WriteString("Now output your decision: a SHORT decision_summary + the strict JSON (per Output Contract)\n")

	return sb.String()
}

// stripRawKlinesFromPrompt removes only top-level signal "ohlcv" JSON values
// from an already rendered prompt. This preserves the exact gate/timestamp
// snapshot and avoids recomputing signals or repeating live-data calls during
// context-budget degradation.
func stripRawKlinesFromPrompt(prompt string) string {
	const marker = `,"ohlcv":`
	for {
		markerAt := strings.Index(prompt, marker)
		if markerAt < 0 {
			return prompt
		}
		valueAt := markerAt + len(marker)
		valueEnd, ok := jsonCompositeEnd(prompt, valueAt)
		if !ok {
			return prompt // fail closed: never corrupt the surrounding signal JSON
		}
		prompt = prompt[:markerAt] + prompt[valueEnd:]
	}
}

func jsonCompositeEnd(s string, start int) (int, bool) {
	if start >= len(s) || (s[start] != '{' && s[start] != '[') {
		return 0, false
	}
	open := s[start]
	close := byte('}')
	if open == '[' {
		close = ']'
	}
	depth := 0
	inString, escaped := false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		switch c {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func (e *StrategyEngine) formatPositionInfo(index int, pos PositionInfo, ctx *Context) string {
	var sb strings.Builder

	// Hands-off marker (user directive 2026-09-25): manual positions are
	// visually tagged AND the model is told the program won't act on them.
	ownership := " | AI托管"
	if !pos.Managed {
		ownership = " | 手动仓(程序不干预:不可 close/adjust/partial,自动化跳过;hold 无需管理字段)"
	}

	holdingDuration := ""
	if pos.UpdateTime > 0 {
		durationMs := time.Now().UnixMilli() - pos.UpdateTime
		durationMin := durationMs / (1000 * 60)
		if durationMin < 60 {
			holdingDuration = fmt.Sprintf(" | Holding Duration %d min", durationMin)
		} else {
			durationHour := durationMin / 60
			durationMinRemainder := durationMin % 60
			holdingDuration = fmt.Sprintf(" | Holding Duration %dh %dm", durationHour, durationMinRemainder)
		}
	}

	positionValue := pos.Quantity * pos.MarkPrice
	if positionValue < 0 {
		positionValue = -positionValue
	}

	// Display the ticker last price (same snapshot as the signal block) so
	// positions and signals never quote two different "current" prices; the
	// exchange mark price stays available as Mark.
	displayPrice := pos.MarkPrice
	priceLabel := "Mark"
	marginROI, priceReturn, uPnL := pos.UnrealizedPnLPct, pos.PriceReturnPct, pos.UnrealizedPnL
	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok && marketData.CurrentPrice > 0 {
		displayPrice = marketData.CurrentPrice
		priceLabel = "Last"
	}
	// Same-source display (09-18 audit #1): the exchange position snapshot's
	// uPnL/ROI lag the live ticker by a poll tick — KORU showed Last 19.28
	// next to a PnL priced at 19.255, and "Price Return" didn't match
	// (Last−Entry)/Entry. When the live price is shown, derive the whole
	// trio from IT so one line quotes exactly one current price.
	if priceLabel == "Last" && pos.EntryPrice > 0 && pos.Quantity != 0 {
		if strings.EqualFold(pos.Side, "long") {
			priceReturn = (displayPrice - pos.EntryPrice) / pos.EntryPrice * 100
		} else {
			priceReturn = (pos.EntryPrice - displayPrice) / pos.EntryPrice * 100
		}
		uPnL = priceReturn / 100 * pos.EntryPrice * pos.Quantity
		marginROI = priceReturn * float64(pos.Leverage)
		positionValue = pos.Quantity * displayPrice
		if positionValue < 0 {
			positionValue = -positionValue
		}
	}

	automationStage := pos.AutomationStage
	if automationStage == "" {
		automationStage = "UNKNOWN"
	}
	sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f %s %.4f | Qty %.4f | Position Value %.2f USDT | Margin ROI %+.2f%% | Price Return %+.2f%% | Unrealized PnL %+.2f USDT | Peak PnL %.2f%% (margin basis) | Leverage %dx | MarginUsed %.2f | Liq Price %.4f | AutoMgmt %s | Reduced %.1f%% of original%s%s\n\n",
		index, pos.Symbol, strings.ToUpper(pos.Side),
		pos.EntryPrice, priceLabel, displayPrice, pos.Quantity, positionValue, marginROI, priceReturn, uPnL, pos.PeakPnLPct,
		pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, automationStage, pos.CumulativeReducedPct, holdingDuration, ownership))

	// Manual position (user review 2026-09-27 #3): automation skips it —
	// close/partial/adjust are all rejected executor-side. The close-lock
	// template's "仍可 adjust_stop_loss" line contradicted the ownership
	// header and baited the model into a guaranteed-rejected action (HYPE
	// case), so manual positions branch to hold-only BEFORE any market JSON:
	// nothing here is actionable, rendering 4 TFs of data only spends tokens
	// and invites proposals the program will refuse.
	if !pos.Managed {
		sb.WriteString(fmt.Sprintf("=== %s — 手动仓,自动化跳过 ===\n只能输出 hold;禁止 close/partial_close/adjust_stop_loss(执行端对手动仓一律拒绝)。本仓不参与本轮分析,无需评估其市场数据。手动仓的 hold 不需要 management_quality/management_flags(程序不采纳)。\n\n", pos.Symbol))
		return sb.String()
	}

	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
		var quantData *QuantData
		if ctx.QuantDataMap != nil {
			quantData = ctx.QuantDataMap[pos.Symbol]
		}
		if sig := e.computeCoinSignal(marketData, quantData, ctx, nil); sig != nil {
			// 09-18 token audit ②: when close/partial cannot pass the
			// close gates this cycle, the 4-TF JSON adds nothing the model
			// can act on — render one management line. Near-stop danger
			// (price within 0.5×ATR(1h) of the recorded stop) keeps the
			// full data so the model sees WHY the position is dangerous.
			if locked, reason := positionCloseLocked(pos, marketData, &e.config.RiskControl); locked {
				nearStop := false
				if pos.StopLossPrice > 0 && displayPrice > 0 {
					dist := math.Abs(displayPrice-pos.StopLossPrice) / displayPrice * 100
					if t1h := sig.Timeframes["1h"]; t1h != nil && t1h.ATRPct > 0 && dist <= 0.5*t1h.ATRPct {
						nearStop = true
					}
				}
				if !nearStop {
					sl, tp := "—", "—"
					if pos.StopLossPrice > 0 {
						sl = strconv.FormatFloat(pos.StopLossPrice, 'f', 4, 64)
					}
					if pos.TakeProfitPrice > 0 {
						tp = strconv.FormatFloat(pos.TakeProfitPrice, 'f', 4, 64)
					}
					sb.WriteString(fmt.Sprintf("=== %s — 平仓门锁定,本期结构数据省略 ===\n%s | SL %s | TP %s | 浮亏/浮盈 ROI %+.2f%% — 输出 hold;仍可 adjust_stop_loss(仅收紧到更优,程序校验)\n\n",
						pos.Symbol, reason, sl, tp, marginROI))
					return sb.String()
				}
			}
			sb.WriteString(e.renderSignalBlock(sig))
			sb.WriteString("\n")
			return sb.String()
		}
		sb.WriteString(e.formatMarketData(marketData, quantData, ctx, nil))
		sb.WriteString("\n")
	}

	return sb.String()
}

// DefaultPromptKlineBars is how many closed bars per timeframe the prompt
// ships when the config doesn't say (user review 2026-09-27: 15-20 recent
// bars carry the actionable micro-structure; the 60-bar dump was ~63% of a
// full signal and diluted the program-computed fields the model trades on).
const DefaultPromptKlineBars = 20

// ResolvePromptKlineBars caps the prompt-side OHLCV per timeframe:
// >0 = keep N (clamped into [10, primaryCount]); 0 = default 20;
// negative = full primaryCount (legacy / audit escape hatch).
func ResolvePromptKlineBars(promptBars, primaryCount int) int {
	if promptBars < 0 {
		return primaryCount
	}
	n := promptBars
	if n == 0 {
		n = DefaultPromptKlineBars
	}
	if n < 10 {
		n = 10
	}
	if n > primaryCount {
		n = primaryCount
	}
	return n
}

// StrategyHealthEdge names the profit-factor regime of the rolling stats
// window: NEGATIVE_EDGE < 0.9 ≤ NO_EDGE < 1.1 ≤ POSITIVE_EDGE. The single
// definition shared by the stats block (strategy_health line) and the
// NEGATIVE_EDGE health gate in computeCoinSignal, so the prose, the gate
// codes and the shadow-block dataset can never disagree about the regime.
func StrategyHealthEdge(pf float64) string {
	switch {
	case pf < 0.9:
		return "NEGATIVE_EDGE"
	case pf < 1.1:
		return "NO_EDGE"
	default:
		return "POSITIVE_EDGE"
	}
}

// strategyEdge resolves the cycle's health regime from the context (no
// stats yet = POSITIVE_EDGE — the health gate must never arm on absence of
// data).
func (e *StrategyEngine) strategyEdge(ctx *Context) string {
	if ctx == nil || ctx.TradingStats == nil || ctx.TradingStats.TotalTrades == 0 {
		return "POSITIVE_EDGE"
	}
	return ResolveStrategyEdge(ctx.TradingStats)
}

// noLosingTrade reports a window with trades but no loser: store leaves both
// ProfitFactor and AvgLoss at 0 only then (a real losing window always has
// AvgLoss > 0), so PF=0 here means "undefined", not "worst".
func noLosingTrade(st *TradingStats) bool {
	return st != nil && st.TotalTrades > 0 && st.ProfitFactor == 0 && st.AvgLoss == 0
}

// MinNegativeEdgeTrades: user decision 2026-10-08 — one or two losers in a
// thin window armed the NEGATIVE_EDGE gate; below this sample it is not assessed.
const MinNegativeEdgeTrades = 20

// negativeEdgeSuppressed reports a PF-negative window downgraded for a thin
// sample; the resolver and the stats-block marker share this one condition.
func negativeEdgeSuppressed(st *TradingStats) bool {
	return st != nil && st.TotalTrades > 0 && st.TotalTrades < MinNegativeEdgeTrades &&
		!noLosingTrade(st) && StrategyHealthEdge(st.ProfitFactor) == "NEGATIVE_EDGE"
}

// ResolveStrategyEdge is the single edge resolver for prose and gate (review
// 2026-10-08 A: all-win window read as PF 0 → NEGATIVE_EDGE). No data →
// POSITIVE_EDGE (never arm on absence); no loser → by TotalPnL; else PF.
func ResolveStrategyEdge(st *TradingStats) string {
	if st == nil || st.TotalTrades == 0 {
		return "POSITIVE_EDGE"
	}
	if noLosingTrade(st) {
		if st.TotalPnL > 0 {
			return "POSITIVE_EDGE"
		}
		return "NO_EDGE"
	}
	if negativeEdgeSuppressed(st) {
		return "NO_EDGE"
	}
	return StrategyHealthEdge(st.ProfitFactor)
}

// formatMarketContext compresses the three market-wide ranking blocks (OI
// change / institution netflow / price movers) into a few regime lines
// (09-18 token audit ⑥): the blocks self-describe as context-only, so the
// full tables were ~2k chars of tokens the model is forbidden to price with.
// Candidate/position symbols get their rows; the rest is market flavor.
func (e *StrategyEngine) formatMarketContext(ctx *Context) string {
	if ctx.OIRankingData == nil && ctx.NetFlowRankingData == nil && ctx.PriceRankingData == nil {
		return ""
	}
	interesting := interestingSymbols(ctx)
	var oiParts, flowIn, flowOut, movers []string

	if d := ctx.OIRankingData; d != nil {
		own, other := 0, 0
		for _, p := range d.TopPositions {
			tag := fmt.Sprintf("%s OI%+.1f%%($%+.0fK,价%+.1f%%)", p.Symbol, p.OIDeltaPercent, p.OIDeltaValue/1000, p.PriceDeltaPercent)
			if interesting[p.Symbol] && own < 3 {
				oiParts = append(oiParts, tag)
				own++
			} else if !interesting[p.Symbol] && other < 2 {
				oiParts = append(oiParts, tag)
				other++
			}
			if own >= 3 && other >= 2 {
				break
			}
		}
	}
	if d := ctx.NetFlowRankingData; d != nil {
		own, other := 0, 0
		for _, p := range d.InstitutionFutureTop {
			tag := fmt.Sprintf("%s +%0.1fM", p.Symbol, p.Amount/1e6)
			if interesting[p.Symbol] && own < 3 {
				flowIn = append(flowIn, tag)
				own++
			} else if !interesting[p.Symbol] && other < 2 {
				flowIn = append(flowIn, tag)
				other++
			}
			if own >= 3 && other >= 2 {
				break
			}
		}
		for _, p := range d.InstitutionFutureLow {
			if len(flowOut) < 2 {
				flowOut = append(flowOut, fmt.Sprintf("%s %0.1fM", p.Symbol, p.Amount/1e6))
			}
		}
	}
	if d := ctx.PriceRankingData; d != nil {
		if dur, ok := d.Durations["4h"]; ok {
			for i, it := range dur.Top {
				if i >= 3 {
					break
				}
				movers = append(movers, fmt.Sprintf("%s %+.1f%%", it.Symbol, it.PriceDelta*100))
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("> 市场语境(宏观参考,严禁参与 entry/SL/TP/RR 精算;榜外币不可决策,价格快照与 Structured Signal 冲突时以后者为准)\n")
	if len(oiParts) > 0 {
		sb.WriteString("- 持仓量变化(1h): " + strings.Join(oiParts, " | ") + "\n")
	}
	if len(flowIn) > 0 || len(flowOut) > 0 {
		line := "- 机构资金流(1h):"
		if len(flowIn) > 0 {
			line += " 流入 " + strings.Join(flowIn, " ")
		}
		if len(flowOut) > 0 {
			line += " | 流出 " + strings.Join(flowOut, " ")
		}
		sb.WriteString(line + "\n")
	}
	if len(movers) > 0 {
		sb.WriteString("- 4h涨跌幅前列: " + strings.Join(movers, " ") + "\n")
	}
	return sb.String()
}

// fresh60mForInteresting: this cycle's rolling-60m change computed from each
// interesting symbol's OWN klines — full ranking rows for these symbols use
// the fresh, signal-aligned figure instead of the up-to-5-minutes-stale
// market-wide snapshot (violent movers used to show 4x divergent values).
func fresh60mForInteresting(ctx *Context) map[string]float64 {
	out := make(map[string]float64, len(ctx.MarketDataMap))
	for sym := range interestingSymbols(ctx) {
		if data, ok := ctx.MarketDataMap[sym]; ok {
			out[sym] = rolling1hChangePct(data, time.Now())
		}
	}
	return out
}

// interestingSymbols: candidates + open positions — the only symbols that get
// full rows in the market-wide ranking tables (the rest collapse to headlines
// to keep the context budget for actionable data).
func interestingSymbols(ctx *Context) map[string]bool {
	out := make(map[string]bool, len(ctx.CandidateCoins)+len(ctx.Positions))
	for _, c := range ctx.CandidateCoins {
		out[strings.ToUpper(market.Normalize(c.Symbol))] = true
	}
	for _, p := range ctx.Positions {
		out[strings.ToUpper(market.Normalize(p.Symbol))] = true
	}
	return out
}

func (e *StrategyEngine) formatCoinSourceTag(sources []string) string {
	if len(sources) > 1 {
		// Multiple signal source combination
		hasAI500 := false
		hasOITop := false
		hasOILow := false
		hasHyperAll := false
		hasHyperMain := false
		for _, s := range sources {
			switch s {
			case "ai500":
				hasAI500 = true
			case "oi_top":
				hasOITop = true
			case "oi_low":
				hasOILow = true
			case "hyper_all":
				hasHyperAll = true
			case "hyper_main":
				hasHyperMain = true
			}
		}
		if hasAI500 && hasOITop {
			return " (AI500+OI_Top dual signal)"
		}
		if hasAI500 && hasOILow {
			return " (AI500+OI_Low dual signal)"
		}
		if hasOITop && hasOILow {
			return " (OI_Top+OI_Low)"
		}
		if hasHyperMain && hasAI500 {
			return " (HyperMain+AI500)"
		}
		if hasHyperAll || hasHyperMain {
			return " (Hyperliquid)"
		}
		return " (Multiple sources)"
	} else if len(sources) == 1 {
		switch sources[0] {
		case "ai500":
			return " (AI500)"
		case "oi_top":
			return " (OI_Top OI increase)"
		case "oi_low":
			return " (OI_Low OI decrease)"
		case "piggy_dash":
			return " (Piggy Dash breakout engine)"
		case "static":
			return " (Manual selection)"
		case "hyper_all":
			return " (Hyperliquid All)"
		case "hyper_main":
			return " (Hyperliquid Top20)"
		}
	}
	return ""
}

// ============================================================================
// Technical Narrative (图形化技术面叙事)
// ============================================================================

// formatTechnicalNarrative renders the daily chart of a candidate as a
// human-readable "看图说话" narrative (headline character, dated key moves,
// volume anomalies, MA position, and a pattern judgment with resistance /
// box levels), mirroring how a trader would verbally walk through the chart.
// Returns "" when daily history is insufficient.
func formatTechnicalNarrative(data *market.Data) string {
	tf, ok := data.TimeframeData["1d"]
	if !ok || len(tf.Klines) < 10 {
		return ""
	}
	all := tf.Klines // full history (MA20 needs >= 20 closes)
	bars := all
	if len(bars) > 15 {
		bars = bars[len(bars)-15:]
	}
	n := len(bars)
	day := func(i int) string { return time.UnixMilli(bars[i].Time).Format("1/2") }

	// Period extremes.
	hi, hiIdx := math.Inf(-1), 0
	lo, loIdx := math.Inf(1), 0
	for i, b := range bars {
		if b.High > hi {
			hi, hiIdx = b.High, i
		}
		if b.Low < lo {
			lo, loIdx = b.Low, i
		}
	}
	last := bars[n-1].Close
	amp := (hi - lo) / lo * 100
	mid := (hi + lo) / 2

	// MA5 / MA20 of daily closes (computed on the full history).
	ma5, ma20 := smaLast(closesBars(all), 5), smaLast(closesBars(all), 20)

	var sb strings.Builder

	// ── 标题：波动特征 + 结构定性 ──
	maxDailyAbs := 0.0
	for i := 1; i < n; i++ {
		if bars[i-1].Close > 0 {
			if p := math.Abs(bars[i].Close/bars[i-1].Close-1) * 100; p > maxDailyAbs {
				maxDailyAbs = p
			}
		}
	}
	volWord := "窄幅整理"
	switch {
	case amp >= 40 || maxDailyAbs >= 8:
		volWord = "剧烈波动"
	case amp >= 20:
		volWord = "波动较大"
	}
	structWord := "宽幅震荡"
	highEarly := hiIdx < n-3 // 冲高不在最近 3 根内 = 冲高发生在窗口前段
	switch {
	case last >= hi*0.97:
		structWord = "强势上行"
	case highEarly && last < mid:
		structWord = "高位宽幅震荡"
	case loIdx >= n/2 && last <= lo*1.05:
		structWord = "弱势下行"
	case math.Abs(last-mid) <= (hi-lo)*0.1:
		structWord = "箱体整理"
	}
	sb.WriteString(fmt.Sprintf("技术面：%s、%s\n", volWord, structWord))
	sb.WriteString(fmt.Sprintf("近%d个交易日走势：\n", n))

	// ── 要点 1：窗口内最猛的单日涨/跌 ──
	worstIdx, worstPct, bestIdx, bestPct := -1, 0.0, -1, 0.0
	for i := 1; i < n; i++ {
		if bars[i-1].Close <= 0 {
			continue
		}
		pct := (bars[i].Close/bars[i-1].Close - 1) * 100
		if pct < worstPct {
			worstPct, worstIdx = pct, i
		}
		if pct > bestPct {
			bestPct, bestIdx = pct, i
		}
	}
	bigMove := func(idx, pct int, dir string) {
		if idx <= 0 {
			return
		}
		word := "大涨"
		if dir == "down" {
			word = map[bool]string{true: "暴跌", false: "大跌"}[pct <= -8]
		}
		sb.WriteString(fmt.Sprintf("· %s 单日%s %.1f%%（%s→%s）%s\n",
			day(idx), word, math.Abs(float64(pct)), fmtPx(bars[idx-1].Close), fmtPx(bars[idx].Close),
			volumeWord(bars[idx], bars)))
	}
	if bestIdx > 0 && bestPct >= 5 && (worstIdx < 0 || bestIdx < worstIdx) {
		bigMove(bestIdx, int(bestPct), "up")
	}
	if worstIdx > 0 && worstPct <= -5 {
		bigMove(worstIdx, int(worstPct), "down")
	}
	if bestIdx > 0 && bestPct >= 5 && worstIdx > 0 && bestIdx > worstIdx {
		bigMove(bestIdx, int(bestPct), "up")
	}

	// ── 要点 2：冲高极值与回落 ──
	sb.WriteString(fmt.Sprintf("· %s 冲高 %s 后", day(hiIdx), fmtPx(hi)))
	if loIdx > hiIdx {
		sb.WriteString(fmt.Sprintf("，%s 回落至 %s（较高点 -%.1f%%）", day(loIdx), fmtPx(lo), amp))
	}
	sb.WriteString("\n")

	// ── 要点 3：近期震荡区间与量能 ──
	rangeBars := bars
	if hiIdx > 2 {
		rangeBars = bars[hiIdx:]
	}
	rLo, rHi := math.Inf(1), math.Inf(-1)
	for _, b := range rangeBars {
		if b.Low < rLo {
			rLo = b.Low
		}
		if b.High > rHi {
			rHi = b.High
		}
	}
	rAmp := 0.0
	if rLo > 0 {
		rAmp = (rHi - rLo) / rLo * 100
	}
	if rHi > rLo && len(rangeBars) >= 3 {
		ampWord := map[bool]string{true: "振幅极大", false: "振幅较大"}[rAmp >= 20]
		sb.WriteString(fmt.Sprintf("· 随后在 %s-%s 之间来回拉锯，%s\n", fmtPx(rLo), fmtPx(rHi), ampWord))
	}
	// 放量日（量能 ≥ 窗口均量 1.8 倍且为最放量的一根）。
	volMaxIdx, volAvg := 0, 0.0
	for i, b := range bars {
		volAvg += b.Volume
		if b.Volume > bars[volMaxIdx].Volume {
			volMaxIdx = i
		}
	}
	volAvg /= float64(n)
	if volMaxIdx >= 0 && volAvg > 0 && bars[volMaxIdx].Volume >= volAvg*1.8 {
		trend := "拉升"
		if bars[volMaxIdx].Close < bars[volMaxIdx].Open {
			trend = "下杀"
		}
		sb.WriteString(fmt.Sprintf("· %s 放量%s至 %s（量能为均量 %.1f 倍）\n",
			day(volMaxIdx), trend, fmtPx(bars[volMaxIdx].Close), bars[volMaxIdx].Volume/volAvg))
	}

	// ── 要点 4：今日位置与均线关系 ──
	maPart := "均线数据不足"
	if ma5 > 0 && ma20 > 0 {
		switch {
		case last >= ma5 && last >= ma20:
			maPart = "站上 5 日线和 20 日线"
		case last >= ma5:
			maPart = "站上 5 日线但仍在 20 日线下方"
		case last >= ma20:
			maPart = "5 日线下方但仍站在 20 日线上"
		default:
			maPart = "5 日线和 20 日线下方"
		}
	}
	sb.WriteString(fmt.Sprintf("· 今日报 %s，%s\n", fmtPx(last), maPart))

	// ── 形态判断 ──
	pressureLo, pressureHi := pressureZone(bars, hi, hiIdx, last)
	sb.WriteString("形态判断：")
	switch structWord {
	case "高位宽幅震荡":
		sb.WriteString("高位宽幅震荡+冲高回落结构")
	case "强势上行":
		sb.WriteString("上升趋势加速结构，追多风险与回撤风险同步放大")
	case "弱势下行":
		sb.WriteString("冲高回落+弱势下行结构")
	default:
		sb.WriteString("箱体整理结构")
	}
	if pressureLo > 0 {
		sb.WriteString(fmt.Sprintf("，上方 %s-%s 为压力/套牢区", fmtPx(pressureLo), fmtPx(pressureHi)))
	}
	posWord := "下方"
	if math.Abs(last-mid) <= (hi-lo)*0.1 {
		posWord = "附近"
	} else if last > mid {
		posWord = "上方"
	}
	sb.WriteString(fmt.Sprintf("。当前价位于箱体中枢 %s %s", fmtPx(mid), posWord))
	if ma20 > 0 && last < ma20 {
		sb.WriteString("，短线性质是超跌反弹而非反转")
	} else if ma5 > 0 && last > ma5 && ma20 > 0 && last > ma20 {
		sb.WriteString("，短线趋势仍偏多")
	}
	sb.WriteString("。\n")

	return sb.String()
}

// pressureZone finds the overhead supply zone: the swing highs above the
// current price within the window (falls back to the period high itself).
func pressureZone(bars []market.KlineBar, periodHi float64, hiIdx int, last float64) (lo, hi float64) {
	if periodHi <= last {
		return 0, 0
	}
	second := math.Inf(-1)
	for i, b := range bars {
		if i == hiIdx {
			continue
		}
		if b.High > second && b.High <= periodHi*0.97 {
			second = b.High
		}
	}
	if second > last && second < periodHi {
		return second, periodHi
	}
	return periodHi * 0.97, periodHi
}

func closesBars(bars []market.KlineBar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

func smaLast(xs []float64, period int) float64 {
	if len(xs) < period {
		return 0
	}
	s := 0.0
	for _, v := range xs[len(xs)-period:] {
		s += v
	}
	return s / float64(period)
}

func volumeWord(bar market.KlineBar, bars []market.KlineBar) string {
	sum, cnt := 0.0, 0
	for _, b := range bars {
		sum += b.Volume
		cnt++
	}
	if cnt == 0 || sum/float64(cnt) <= 0 {
		return ""
	}
	avg := sum / float64(cnt)
	switch {
	case bar.Volume >= avg*1.8:
		return "，伴随放量"
	case bar.Volume <= avg*0.6:
		return "，但量能萎缩"
	default:
		return ""
	}
}

// fmtPx formats a price with magnitude-appropriate precision for the
// narrative text.
func fmtPx(v float64) string {
	switch {
	case v >= 1000:
		return strconv.FormatFloat(v, 'f', 1, 64)
	case v >= 100:
		return strconv.FormatFloat(v, 'f', 2, 64)
	case v >= 1:
		return strconv.FormatFloat(v, 'f', 4, 64)
	default:
		return strconv.FormatFloat(v, 'f', 6, 64)
	}
}

// ============================================================================
// Market Data Formatting
// ============================================================================

// computeCoinSignal builds the SignalOptions for one symbol, runs the signal
// layer, and records the program-side bookkeeping the decision validator and
// datasets read (anchors / RR ceilings / gate states). Returns nil when the
// data cannot support a signal — callers fall back to the legacy dump.
// Split from formatMarketData for the 09-18 token audit: the candidate and
// position renderers need the signal VERDICT before choosing how much to
// render (double-blocked candidates and close-locked positions compress to
// one line), while the bookkeeping must still happen every cycle.
func (e *StrategyEngine) computeCoinSignal(data *market.Data, quantData *QuantData, ctx *Context, coin *CandidateCoin) *SymbolSignal {
	// Signal layer: structured, normalized feature block replaces the raw
	// candle dump. Falls back to the legacy text dump only if computation fails.
	opt := SignalOptions{
		Now:                     time.Now(),
		PrimaryTF:               e.config.Indicators.Klines.PrimaryTimeframe,
		CurrentPrice:            data.CurrentPrice,
		LimitEntryOffsetPct:     e.config.RiskControl.LimitEntryOffsetPct,
		LimitEntryOffsetMode:    e.config.RiskControl.LimitEntryOffsetMode,
		LimitEntryOffsetATRMult: e.config.RiskControl.LimitEntryOffsetATRMult,
		LimitEntryOffsetMinPct:  e.config.RiskControl.LimitEntryOffsetMinPct,
		LimitEntryOffsetMaxPct:  e.config.RiskControl.LimitEntryOffsetMaxPct,
		SupplyZonePct:           e.config.RiskControl.OpenRejectSupplyPct,
		PumpGuard4hPct:          PumpGuard4h(&e.config.RiskControl),
		// Hard-entry gate inputs — the program pre-evaluates the strategy's
		// own gates per direction (review 2026-09-15).
		MinRR:             e.config.RiskControl.MinRiskRewardRatio,
		LimitEntryEnabled: e.config.RiskControl.LimitEntryEnabled,
		EntryTimingGate:   e.config.RiskControl.EntryTimingGate,
		// Vendor-vs-live divergence hard gate (09-18 audit): a large
		// vendor gap prices every anchor/SL/TP off the wrong tick.
		MaxVendorDivergencePct: EffectiveMaxVendorDivergencePct(&e.config.RiskControl),
		// NEGATIVE_EDGE health gate (user review 2026-09-27 #4): the same
		// PF regime the stats block prints, resolved through the shared
		// helpers so prompt prose and gate codes never quote different
		// numbers.
		NegativeEdge:            NegativeEdgeGateEnabled(&e.config.RiskControl) && e.strategyEdge(ctx) == "NEGATIVE_EDGE",
		NegativeEdgeMinScore:    NegativeEdgeMinScore(&e.config.RiskControl),
		NegativeEdgeMinRR:       NegativeEdgeMinRR(&e.config.RiskControl),
		NegativeEdgeBlockLosing: NegativeEdgeBlockLosingSymbol(&e.config.RiskControl),
		// Long-side entry discipline (user design 2026-10-01): breakout
		// retest anchors, the EMA20 chase cap, the BTC market filter and the
		// greed deweight — all config-switchable via risk_control.
		LongPullbackEntry:   e.config.RiskControl.EffectiveLongPullbackEntry(),
		LongMaxEMA20DistPct: e.config.RiskControl.EffectiveLongMaxEMA20DistPct(),
		// Anti-chase symmetric gates (Opus analysis 2026-10-11, 273 live trades).
		ChaseMaxEMA20_1hPct:   e.config.RiskControl.EffectiveChaseMaxEMA20_1hPct(),
		CounterMaxEMA20_1hPct: e.config.RiskControl.EffectiveCounterMaxEMA20_1hPct(),
		EntryRSI1hMax:         e.config.RiskControl.EffectiveEntryRSI1hMax(),
		ChaseMaxMove4hPct:     e.config.RiskControl.EffectiveChaseMaxMove4hPct(),
		BTCFilterLong:         e.config.RiskControl.EffectiveBTCFilterLong(),
		// Short-side mirrors (2026-10-04 batch-2): the BTC strong-bull hard
		// pause is OPT-IN (the scan-side ×0.85 haircut off the same shared
		// classifier stays the default treatment); the top-confirmation gate
		// program-enforces the playbook's 默认姿态 for short_scan candidates.
		BTCFilterShort:             e.config.RiskControl.EffectiveBTCFilterShort(),
		ShortTopConfirmGate:        e.config.RiskControl.EffectiveShortTopConfirmGate(),
		SentimentLongDeweightPts:   e.config.RiskControl.EffectiveSentimentLongDeweightPts(),
		SentimentLongDeweightArmed: sentimentGreedy(e.config.RiskControl.EffectiveSentimentLongDeweightFNG()),
		BtcTrendCloses:             binanceBTC4hCloses(300),
		EntryRoundTripCostBps:      e.config.RiskControl.EffectiveEntryRoundTripCostBps(),
		MaxStopDistancePct:         e.config.RiskControl.EffectiveMaxStopDistancePct(),
	}
	// DataQuality's bar requirements must match what fetchMarketDataWithStrategy
	// actually fetches — including its legacy expansion of an empty
	// SelectedTimeframes (round-4 review R4-3: a ["5m","15m","1h"] strategy
	// was pinned to DATA_INSUFFICIENT by the static 4h requirement).
	{
		tfs := e.config.Indicators.Klines.SelectedTimeframes
		if len(tfs) == 0 {
			if p := e.config.Indicators.Klines.PrimaryTimeframe; p != "" {
				tfs = append(tfs, p)
			}
			if l := e.config.Indicators.Klines.LongerTimeframe; l != "" {
				tfs = append(tfs, l)
			}
		}
		opt.ConfiguredTimeframes = withRequiredSymbolTimeframes(tfs, data.Symbol)
	}
	opt.Quant = quantData
	{
		frPct := e.config.CoinSource.ShortScanFundingRatePct
		if frPct <= 0 {
			frPct = 0.03
		}
		opt.ShortFundingCrowdPctP = frPct
		// STOCK_WEEKEND hard block: same predicate as the executor gate
		// (policy enabled && bstock symbol && US weekend) — surfaced in
		// hard_entry_gate.failed instead of only as a prose bullet.
		if noOpen := e.config.RiskControl.StockWeekendNoOpen; (noOpen == nil || *noOpen) &&
			market.IsBStockSymbol(data.Symbol) && market.IsUSMarketWeekend(opt.Now) {
			opt.StockWeekendBlock = true
		}
	}
	// Per-symbol enrichment (all programmatic, no AI judgment):
	if coin != nil {
		for _, src := range coin.Sources {
			if src == "short_scan" && coin.ShortScore > 0 {
				opt.ScannerBias = "short"
				// The top-confirmation gate consumes the scanner's own
				// confirmation verdict (2026-10-04 batch-2).
				conf := coin.ShortConfirmed
				opt.ShortScanConfirmed = &conf
				break
			}
		}
		if coin.ScannerConflict {
			opt.ScannerConflict = true
		}
	}
	opt.QuoteVolume24hUsd = binanceQuoteVolume24h(data.Symbol)
	if sp := binanceOrderBookSpreadPct(data.Symbol); sp > 0 {
		v := sp
		opt.SpreadPct = &v
	}
	// Institution futures net inflow for this symbol (1h ranking) — feeds
	// the long-squeeze mirror condition (09-19 audit 七).
	if ctx != nil && ctx.NetFlowRankingData != nil {
		for _, p := range ctx.NetFlowRankingData.InstitutionFutureTop {
			if p.Symbol == strings.ToUpper(data.Symbol) {
				v := p.Amount
				opt.NetInflowUSDT = &v
				break
			}
		}
	}
	if m, err := binanceLongShortMetrics(data.Symbol); err == nil {
		opt.LongShortAccountRatio = m.AccountRatio
		opt.TopTraderPositionRatio = m.TopPosRatio
		opt.TakerBuySellRatio = m.TakerBuySell
	}
	opt.BtcCloses = binanceBTC1hCloses(72)
	if data.VendorStalenessPct != nil {
		v := *data.VendorStalenessPct
		opt.VendorStalenessPct = &v
	}
	if ctx != nil && len(ctx.SymbolStats) > 0 {
		if st, ok := ctx.SymbolStats[strings.ToUpper(data.Symbol)]; ok {
			opt.TraderHistory = st
		}
	}
	// Program-computed circuit-breaker verdict + minimum-notional feasibility
	// (09-15: the model was self-declaring LOSS_STREAK_BAN on just-won symbols
	// and re-deriving the small-account dead zone every cycle — both now
	// precomputed here).
	if ctx != nil {
		if until, ok := ctx.LossStreakBanned[strings.ToUpper(data.Symbol)]; ok {
			opt.LossStreakBannedUntil = until
		}
		if eq := ctx.Account.TotalEquity; eq > 0 {
			opt.EquityUSDT = eq
			// The binding minimum is the strategy-config min_position_size
			// (web strategy page) — the same value enforceMinPositionSize
			// rejects orders under. Config unset → executor default 12.
			opt.MinPositionSizeUSDT = e.config.RiskControl.MinPositionSize
			if opt.MinPositionSizeUSDT <= 0 {
				opt.MinPositionSizeUSDT = MinPositionSizeDefaultUSDT
			}
			opt.RiskPct = e.config.RiskControl.EffectiveRiskPerTradePct()
		}
	}
	// The stop noise floor is pure strategy config — independent of the
	// equity context above (it also gates the hard-entry gate's rr_scan).
	opt.SLMinATRMult = e.config.RiskControl.SLMinATRMult
	if quantData != nil {
		for _, oi := range quantData.OI {
			if oi == nil || oi.Delta == nil {
				continue
			}
			if d1h, ok := oi.Delta["1h"]; ok && d1h != nil {
				v := d1h.OIDeltaPercent
				opt.OI1hPct = &v
				break
			}
		}
	}
	// exit_rule_triggered is direction-aware (2026-10-09 ONUSDT): thread the
	// held position's side into the signal so the rule evaluates the matching
	// mirror. An empty side (not held) leaves the flag false — the phantom
	// suppression this replaces also covered the held case wrongly, by
	// showing a SHORT the long-side exit semantics.
	if ctx != nil {
		for _, p := range ctx.Positions {
			if market.Normalize(p.Symbol) == market.Normalize(data.Symbol) {
				switch {
				case strings.EqualFold(p.Side, "long"):
					opt.PositionSide = "long"
				case strings.EqualFold(p.Side, "short"):
					opt.PositionSide = "short"
				}
				break
			}
		}
	}
	if sig, err := ComputeSymbolSignals(data.Symbol, data, opt); err == nil {
		// Naming consistency: the JSON symbol MUST match the candidate heading
		// and the decision symbol — market data may carry a prefixed exchange
		// name (e.g. "XYZ:INTC") while the candidate list uses the normalized
		// form. Decisions route through the candidate name.
		if coin != nil && coin.Symbol != "" {
			sig.Symbol = strings.ToUpper(coin.Symbol)
		}
		// Remember the anchors the model is being shown, so post-parse
		// compliance can snap open_*_limit prices back to the pre-computed
		// values when the model does its own math.
		if ctx != nil {
			if ctx.LimitAnchors == nil {
				ctx.LimitAnchors = make(map[string]*LimitAnchor)
			}
			ctx.LimitAnchors[market.Normalize(sig.Symbol)] = &LimitAnchor{
				LimitBuy:  sig.LimitBuyPrice,
				LimitSell: sig.LimitSellPrice,
			}
			// Capture the rr_scan ceilings shown this cycle (09-16 point 2:
			// the wait→fill RR-decay dataset needs the program value from the
			// decision that waited, not a later re-derivation).
			if ctx.RRCeilings == nil {
				ctx.RRCeilings = make(map[string]*RRCeiling)
			}
			rc := &RRCeiling{}
			if sig.HardGate != nil {
				if sig.HardGate.Long != nil && sig.HardGate.Long.RR != nil {
					rc.LongRR, rc.LongUsable = sig.HardGate.Long.RR.BestRR, sig.HardGate.Long.RR.Usable
				}
				if sig.HardGate.Short != nil && sig.HardGate.Short.RR != nil {
					rc.ShortRR, rc.ShortUsable = sig.HardGate.Short.RR.BestRR, sig.HardGate.Short.RR.Usable
				}
			}
			ctx.RRCeilings[market.Normalize(sig.Symbol)] = rc
			// Capture the hard-gate verdicts so wait_state can be derived
			// program-side from (wait_bias, gate) instead of model judgment.
			if ctx.GateStates == nil {
				ctx.GateStates = make(map[string]*GateState)
			}
			gs := &GateState{
				LongAllowed:  sig.HardGate != nil && sig.HardGate.Long != nil && sig.HardGate.Long.Allowed,
				ShortAllowed: sig.HardGate != nil && sig.HardGate.Short != nil && sig.HardGate.Short.Allowed,
				HardBlocked:  bothDirectionsHardBlocked(sig),
			}
			if sig.HardGate != nil {
				if sig.HardGate.Long != nil {
					gs.LongStopPlanPrice = sig.HardGate.Long.StopPlanPrice
					gs.LongEntryPrice = sig.HardGate.Long.EntryPrice
					gs.LongEntryBasis = sig.HardGate.Long.EntryBasis
					gs.LongFailed = sig.HardGate.Long.Failed
					gs.LongMarketException = sig.HardGate.Long.MarketException
					gs.LongLimitAllowed = sig.HardGate.Long.LimitAllowed
					// Breakout-retest exemption plumbing (2026-10-03 review):
					// the executor's supply-zone check skips when the decision
					// price ≈ the pullback level — the pivots above it are the
					// breakout's own extension, not supply.
					if sig.LongPullback != nil && sig.LongPullback.Active {
						gs.LongPullbackActive = true
						gs.LongPullbackLevel = sig.LongPullback.Level
					}
					if sig.HardGate.Long.RR != nil {
						gs.LongTakeProfit = sig.HardGate.Long.RR.FirstRRGeTarget
						gs.LongTPMenu = sig.HardGate.Long.RR.Options
					}
				}
				if sig.HardGate.Short != nil {
					gs.ShortStopPlanPrice = sig.HardGate.Short.StopPlanPrice
					gs.ShortEntryPrice = sig.HardGate.Short.EntryPrice
					gs.ShortEntryBasis = sig.HardGate.Short.EntryBasis
					gs.ShortFailed = sig.HardGate.Short.Failed
					gs.ShortMarketException = sig.HardGate.Short.MarketException
					gs.ShortLimitAllowed = sig.HardGate.Short.LimitAllowed
					if sig.HardGate.Short.RR != nil {
						gs.ShortTakeProfit = sig.HardGate.Short.RR.FirstRRGeTarget
						gs.ShortTPMenu = sig.HardGate.Short.RR.Options
					}
				}
			}
			ctx.GateStates[market.Normalize(sig.Symbol)] = gs
		}
		// Raw OHLCV (user directive 2026-09-25): the strategy config's 市场数据
		// panel promises raw candles alongside the derived metrics. CLOSED bars
		// only (the forming candle is dropped — every derived field above is
		// closed-basis and mixing bases invites lookahead reads), oldest→newest,
		// prompt keeps the most recent ResolvePromptKlineBars per configured
		// timeframe (default 20) + the newest closed bar's timestamp per TF.
		if e.config.Indicators.EnableRawKlines && len(data.TimeframeData) > 0 {
			count := e.config.Indicators.Klines.PrimaryCount
			if count < store.MinKlineCount {
				count = store.MinKlineCount
			}
			if count > store.MaxKlineCount {
				count = store.MaxKlineCount
			}
			keep := ResolvePromptKlineBars(e.config.Indicators.Klines.PromptKlineBars, count)
			ov := map[string][][5]float64{}
			lastClosed := map[string]string{}
			for _, tf := range opt.ConfiguredTimeframes {
				ts := data.TimeframeData[tf]
				if ts == nil || len(ts.Klines) == 0 {
					continue
				}
				dur := tfDuration(tf).Milliseconds()
				bars := make([][5]float64, 0, keep)
				newestClosed := int64(0)
				for _, k := range ts.Klines {
					if k.Time+dur > opt.Now.UnixMilli() {
						continue // forming candle
					}
					bars = append(bars, [5]float64{k.Open, k.High, k.Low, k.Close, k.Volume})
					newestClosed = k.Time
				}
				if len(bars) > keep {
					bars = bars[len(bars)-keep:]
				}
				if len(bars) > 0 {
					ov[tf] = bars
					lastClosed[tf] = time.UnixMilli(newestClosed).UTC().Format("2006-01-02T15:04:05Z")
				}
			}
			if len(ov) > 0 {
				sig.OHLCV = ov
				sig.OHLCVLastClosed = lastClosed
			}
		}
		// Data-incomplete symbols are barred from trading — nothing beyond
		// the DO-NOT block is rendered for them.
		return sig
	}
	return nil
}

// formatMarketData renders the full structured-signal block for one symbol
// (legacy candle dump when the signal layer cannot produce a signal).
func (e *StrategyEngine) formatMarketData(data *market.Data, quantData *QuantData, ctx *Context, coin *CandidateCoin) string {
	if sig := e.computeCoinSignal(data, quantData, ctx, coin); sig != nil {
		return e.renderSignalBlock(sig)
	}
	var sb strings.Builder
	indicators := e.config.Indicators

	// Clearly label the coin symbol
	sb.WriteString(fmt.Sprintf("=== %s Market Data ===\n\n", data.Symbol))
	sb.WriteString(fmt.Sprintf("current_price = %.4f", data.CurrentPrice))

	if indicators.EnableEMA {
		sb.WriteString(fmt.Sprintf(", current_ema20 = %.3f", data.CurrentEMA20))
	}

	if indicators.EnableMACD {
		sb.WriteString(fmt.Sprintf(", current_macd = %.3f", data.CurrentMACD))
	}

	if indicators.EnableRSI {
		sb.WriteString(fmt.Sprintf(", current_rsi7 = %.3f", data.CurrentRSI7))
	}

	sb.WriteString("\n\n")

	if indicators.EnableOI || indicators.EnableFundingRate {
		sb.WriteString(fmt.Sprintf("Additional data for %s:\n\n", data.Symbol))

		// OpenInterestOK guard (round-4 review R4-6): a failed fetch leaves
		// the zero-value struct in place — printing "0 / 0" here would read
		// as a real reading to the model.
		if indicators.EnableOI && data.OpenInterest != nil && data.OpenInterestOK {
			sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n",
				data.OpenInterest.Latest, data.OpenInterest.Average))
		}

		if indicators.EnableFundingRate {
			sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
		}
	}

	if len(data.TimeframeData) > 0 {
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				sb.WriteString(fmt.Sprintf("=== %s Timeframe (oldest → latest) ===\n\n", strings.ToUpper(tf)))
				e.formatTimeframeSeriesData(&sb, tfData, indicators)
			}
		}
	} else {
		// Compatible with old data format
		if data.IntradaySeries != nil {
			klineConfig := indicators.Klines
			sb.WriteString(fmt.Sprintf("Intraday series (%s intervals, oldest → latest):\n\n", klineConfig.PrimaryTimeframe))

			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}

			if indicators.EnableEMA && len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA indicators (20-period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}

			if indicators.EnableMACD && len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}

			if indicators.EnableRSI {
				if len(data.IntradaySeries.RSI7Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (7-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
				}
				if len(data.IntradaySeries.RSI14Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
				}
			}

			if indicators.EnableVolume && len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3m ATR (14-period): %.3f\n\n", data.IntradaySeries.ATR14))
			}
		}

		if data.LongerTermContext != nil && indicators.Klines.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf("Longer-term context (%s timeframe):\n\n", indicators.Klines.LongerTimeframe))

			if indicators.EnableEMA {
				sb.WriteString(fmt.Sprintf("20-Period EMA: %.3f vs. 50-Period EMA: %.3f\n\n",
					data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3-Period ATR: %.3f vs. 14-Period ATR: %.3f\n\n",
					data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))
			}

			if indicators.EnableVolume {
				sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
					data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))
			}

			if indicators.EnableMACD && len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}

			if indicators.EnableRSI && len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}
		}
	}

	return sb.String()
}

// signalBlockLegend documents the Structured Signal JSON fields. It is coin-independent,
// so it is rendered ONCE before the first signal block (positions/candidates) instead of
// once per coin — duplicating it across 8+ candidates wasted ~21k chars every cycle.
const signalBlockLegend = `时间口径: timeframe 名称是K线粒度;trend_window_return_pct 的窗口看 return_window_hours;price_change_60m/24h_live_pct 使用实时价;prev_hour_close_change_pct 只看最近完整1h收盘。macd_hist 实际为 MACD line(EMA12-EMA26)/price,不是传统 histogram。
程序字段是唯一权威,禁止重算:
- hard_entry_gate.long/short.allowed 是最终方向权限。false→wait并逐项引用 failed;true 且 limit_allowed→限价复制 entry_price;true 且 !limit_allowed 且 market_exception→可走市价例外。不存在其他路径。
- long_pullback 块(突破回踩多头):breakout 确认后 limit_buy_price 即被破前阻力位(旧阻转支撑),止损计划按该位下方结构生成——等回踩、不追延伸;chase_dist_pct 是现价距锚的延伸度,funding_not_overheated / volume_confirmation / oi_confirmation 是对称证据徽章,ema20_4h_dist_pct 是偏离度。回踩失败(收盘跌回位下)状态自动转 fake_break,锚随之消失——不要在无锚时自行猜回踩位。
- market_regime、execution_filter、pump_guard、data_quality、data_freshness、liquidity、funding_rollover、bb_ride(布林上轨骑行)/short_ride(布林下轨骑行) 和 breakout 均为程序结果。funding_rate 是原始小数;funding_annualized_pct 才是年化百分比。pump_guard.return_4h_pct = 最近 5 根已闭合 4h K 线(约 20 小时)的趋势窗口累计涨幅——4h 周期指标,不是最近 4 小时的涨幅。
- bias.scanner 只是候选来源姿态;方向依据 structure/execution/directional_score。仅在 hard_entry_gate.allowed=true 且准备开仓时处理 signal_conflict;已被硬门阻断时直接 wait,不展开冲突分析。
- support/resistance 与距离均按实时价生成;空数组表示对应方向没有结构参考。trend 与窗口收益方向不同可以是合法反弹/回撤,不自动构成冲突。
- rr_scan.first_obstacle / first_obstacle_rr = 交易路径上最近的结构位及该位的净 RR(无论是否达到 min_rr)。止盈目标若在它之后,价格要先穿过它;first_obstacle_rr 很低说明前方很近就有墙——这是证据,不是禁开码,请在推理中说明你如何看待它。
- exit_rule_triggered 按当前持仓方向计算,只在你持有该币时可能为 true:多头=价格跌破主TF结构低/RSI>80/主TF趋势转down且收阴;空头=镜像(升破结构高/RSI<20/趋势转up且收阳);无持仓恒 false。它是该方向反转的程序证据,不是平仓指令——引用时必须写明触发的是哪个条件并与 1h/4h 结构互证;程序值为 true 但你的结构判断不成立时可以继续 hold 并说明理由。
- regime_line 是 trend_tf(默认1h)EMA50 收敛线;last_close_above(最近已收盘K线收盘 vs 线)与 live_price_above(实时价 vs 线)已由程序预计算。判断「升破/跌破」直接引用这两个布尔值,禁止自己比较小数——0.1107 与 0.110776 曾被读成「升破」。
数据新鲜度优先级: Structured Signal timestamp > derivatives/liquidity > scanner/rankings。hint/榜单旧价格禁止参与 entry/SL/TP/RR 精确计算。ohlcv 只用于软证据,不得覆盖上述程序结论。
`

// signalGateLineLimit / signalGateLineMarket are the hard_entry_gate rule line
// inside signalBlockLegend for limit-entry and market-entry strategies.
const (
	signalGateLineLimit  = "- hard_entry_gate.long/short.allowed 是最终方向权限。false→wait并逐项引用 failed;true 且 limit_allowed→限价复制 entry_price;true 且 !limit_allowed 且 market_exception→可走市价例外。不存在其他路径。\n"
	signalGateLineMarket = "- hard_entry_gate.long/short.allowed 是最终方向权限。false→wait并逐项引用 failed;true→市价 open_long/open_short,按 entry_price(实时价)计划复制止损止盈。不存在其他路径。\n"
)

// signalBlockLegendFor returns the legend for the strategy's entry mode.
// Limit mode is byte-identical to signalBlockLegend.
func signalBlockLegendFor(limitEntryEnabled bool) string {
	if limitEntryEnabled {
		return signalBlockLegend
	}
	return strings.Replace(signalBlockLegend, signalGateLineLimit, signalGateLineMarket, 1)
}

// renderSignalBlock renders the structured signal block: a plain-language
// summary header (structure state, key levels, derivatives, rules) followed by
// the normalized per-timeframe JSON features.
func (e *StrategyEngine) renderSignalBlock(sig *SymbolSignal) string {
	var sb strings.Builder

	if !sig.DataComplete {
		sb.WriteString(fmt.Sprintf("=== %s — DATA INCOMPLETE ===\n", sig.Symbol))
		for _, w := range sig.Warnings {
			sb.WriteString("⚠ " + w + "\n")
		}
		sb.WriteString("DO NOT trade this symbol this cycle.\n\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("=== %s Structured Signal (all times UTC, closed candles only) ===\n", sig.Symbol))
	// External news contains provider-controlled free-form text. Keep it in
	// application data, but remove raw headlines and URLs at the execution
	// prompt boundary: JSON escaping does not prevent semantic prompt injection.
	promptSignal := sig
	if sig.Derivatives != nil && len(sig.Derivatives.News) > 0 {
		copySig := *sig
		copyDerivatives := *sig.Derivatives
		copyDerivatives.News = nil
		copySig.Derivatives = &copyDerivatives
		promptSignal = &copySig
	}
	sb.WriteString(RenderSignalJSON(promptSignal))
	sb.WriteString("\n")
	return sb.String()
}

// bothDirectionsHardBlocked: the candidate can only ever produce a mechanical
// wait — its full JSON need not be rendered (09-18 token audit ①: 7 of 8
// candidates in the audited cycle were double-blocked yet shipped 3.5-4.5k
// chars of JSON each). Since the 09-19 fail-closed change, allowed=false on
// a direction means UNEXECUTABLE (anchor-suppressed without exception
// evidence is blocked, not exception-eligible), so the both-directions
// allowed check is the complete test.
func bothDirectionsHardBlocked(sig *SymbolSignal) bool {
	return sig != nil && sig.HardGate != nil &&
		!sig.HardGate.Long.Allowed && !sig.HardGate.Short.Allowed
}

// blockedCoinRow is one double-blocked candidate waiting for the compact
// table render (symbol + source tags + its already-computed signal).
type blockedCoinRow struct {
	symbol string
	tags   string
	sig    *SymbolSignal
}

// renderBlockedCoinTable: one compact markdown table for every
// double-blocked candidate (09-27 review #5 — the wait instruction used to
// repeat at the end of every per-coin line). The failed codes ARE the
// decision content; the header states the mechanical-wait rule ONCE.
func renderBlockedCoinTable(coins []blockedCoinRow, firstNum int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 双向硬门拦截候选(%d 个,本期一律 wait,细节省略)\n\n", len(coins)))
	sb.WriteString("统一规则(对本表每一行生效,不再逐行重复): 输出 wait;no_trade_reason 从该行两侧 hard_blockers 去重后按风险优先级选 2-4 项(blocking_factors 由后端映射,无需输出);不要展开分析,不要给出表外理由。\n\n")
	sb.WriteString("| # | symbol | directional_score | bias(scanner/structure/execution) | hard_blockers.long | hard_blockers.short | history |\n")
	sb.WriteString("|---|--------|-------------------|-----------------------------------|--------------------|---------------------|---------|\n")
	for i, bc := range coins {
		sig := bc.sig
		score := 0
		if sig.SignalConflict != nil {
			score = sig.SignalConflict.DirectionalScore
		}
		bias := "unknown"
		if sig.Bias != nil {
			bias = fmt.Sprintf("%s/%s/%s", sig.Bias.Scanner, sig.Bias.Structure, sig.Bias.Execution)
		}
		codes := func(g *DirectionGate) string {
			if g == nil {
				return "unknown"
			}
			if len(g.Failed) == 0 {
				return "allowed"
			}
			return strings.Join(g.Failed, "+")
		}
		history := "—"
		if sig.TraderHistory != nil {
			history = fmt.Sprintf("%d trades,win %.0f%%,%+.2fU", sig.TraderHistory.ClosedTrades, sig.TraderHistory.WinRatePct, sig.TraderHistory.RealizedPnL)
		}
		sb.WriteString(fmt.Sprintf("| %d | %s%s | %+d | %s | %s | %s | %s |\n",
			firstNum+i+1, bc.symbol, bc.tags, score, bias, codes(sig.HardGate.Long), codes(sig.HardGate.Short), history))
	}
	sb.WriteString("\n")
	return sb.String()
}

// oneHAgainstCandles counts the trailing consecutive CLOSED 1h candles that
// run AGAINST the position side (long → bearish, short → bullish); the
// forming bar is dropped. Mirror of the trader's early-close evidence
// counter — the prompt-side close-lock compression must never disagree with
// the gate it mirrors.
func oneHAgainstCandles(data *market.Data, side string) int {
	if data == nil {
		return 0
	}
	tf, ok := data.TimeframeData["1h"]
	if !ok || tf == nil || len(tf.Klines) < 2 {
		return 0
	}
	bars := tf.Klines[:len(tf.Klines)-1]
	n := 0
	for i := len(bars) - 1; i >= 0; i-- {
		b := bars[i]
		if b.Close == b.Open {
			break // doji: no direction, breaks the streak
		}
		against := (side == "long" && b.Close < b.Open) || (side == "short" && b.Close > b.Open)
		if !against {
			break
		}
		n++
	}
	return n
}

// positionCloseLocked mirrors the trader's early-close/min-hold close gates:
// when an AI close/partial cannot pass this cycle, the position's full TF
// JSON adds nothing but tokens (09-18 audit ②) — render one management line
// instead. Fail-open: unknown hold age renders full data (a gate must never
// trap a position it cannot see). Exchange SL/TP triggers and the
// drawdown-protect close are program paths, unaffected by either gate.
func positionCloseLocked(pos PositionInfo, data *market.Data, rc *store.RiskControlConfig) (bool, string) {
	if pos.UpdateTime <= 0 {
		return false, "" // age unknown — full data, don't guess
	}
	held := time.Since(time.UnixMilli(pos.UpdateTime))
	// Hard-exit bypass: mark already at/beyond the recorded stop.
	if pos.StopLossPrice > 0 {
		if (strings.EqualFold(pos.Side, "long") && pos.MarkPrice <= pos.StopLossPrice) ||
			(strings.EqualFold(pos.Side, "short") && pos.MarkPrice >= pos.StopLossPrice) {
			return false, ""
		}
	}
	var locks []string
	if hours := EarlyCloseHours(rc); hours > 0 && held < time.Duration(hours)*time.Hour {
		if oneHAgainstCandles(data, strings.ToLower(pos.Side)) < 2 {
			locks = append(locks, fmt.Sprintf("持仓不足%dh且1h逆向收盘<2根", hours))
		}
	}
	if minHold := rc.MinHoldMinutes; minHold > 0 && held < time.Duration(minHold)*time.Minute {
		locks = append(locks, fmt.Sprintf("持仓不足%dmin", minHold))
	}
	if len(locks) == 0 {
		return false, ""
	}
	heldMin := int(held.Minutes())
	heldStr := fmt.Sprintf("%dmin", heldMin)
	if heldMin >= 60 {
		heldStr = fmt.Sprintf("%dh%dm", heldMin/60, heldMin%60)
	}
	return true, fmt.Sprintf("持仓%s | 本期 close/partial 被程序锁定(%s,未触发硬退出)", heldStr, strings.Join(locks, "、"))
}
func boolZhEN(b bool) string {
	if b {
		return "TRIGGERED"
	}
	return "not triggered"
}

func formatLevels(levels []float64) string {
	parts := make([]string, 0, len(levels))
	for _, l := range levels {
		parts = append(parts, fmt.Sprintf("%.5g", l))
	}
	return strings.Join(parts, ", ")
}

func tfLabel(dummy string) string { return "primary" }

// ResistanceIncl / SupportIncl merge per-TF pivot lists with the primary TF's
// Bollinger-supplemented lists (already merged in the TF signal).
// NearestLevels merges S/R candidates from ALL timeframes and returns the
// closest resistance above and closest support below the current price.
func (sig *SymbolSignal) NearestLevels() (resistance, support float64) {
	resistance, support = 0, 0
	for _, tf := range sig.Timeframes {
		for _, r := range tf.Resistance {
			if r > sig.Price && (resistance == 0 || r < resistance) {
				resistance = r
			}
		}
		for _, s := range tf.Support {
			if s < sig.Price && (support == 0 || s > support) {
				support = s
			}
		}
	}
	return resistance, support
}

func (sig *SymbolSignal) ResistanceIncl() []float64 {
	if tf := primaryTFSignal(sig, sig.PrimaryTF); tf != nil {
		return tf.Resistance
	}
	return nil
}

func (sig *SymbolSignal) SupportIncl() []float64 {
	if tf := primaryTFSignal(sig, sig.PrimaryTF); tf != nil {
		return tf.Support
	}
	return nil
}

func (e *StrategyEngine) formatTimeframeSeriesData(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig) {
	if len(data.Klines) > 0 {
		sb.WriteString("Time(UTC)      Open      High      Low       Close     Volume\n")
		for i, k := range data.Klines {
			t := time.Unix(k.Time/1000, 0).UTC()
			timeStr := t.Format("01-02 15:04")
			marker := ""
			if i == len(data.Klines)-1 {
				marker = "  <- current"
			}
			sb.WriteString(fmt.Sprintf("%-14s %-9.4f %-9.4f %-9.4f %-9.4f %-12.2f%s\n",
				timeStr, k.Open, k.High, k.Low, k.Close, k.Volume, marker))
		}
		sb.WriteString("\n")
	} else if len(data.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.MidPrices)))
		if indicators.EnableVolume && len(data.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.Volume)))
		}
	}

	if indicators.EnableEMA {
		if len(data.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA20: %s\n", formatFloatSlice(data.EMA20Values)))
		}
		if len(data.EMA50Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA50: %s\n", formatFloatSlice(data.EMA50Values)))
		}
	}

	if indicators.EnableMACD && len(data.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD: %s\n", formatFloatSlice(data.MACDValues)))
	}

	if indicators.EnableRSI {
		if len(data.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI7: %s\n", formatFloatSlice(data.RSI7Values)))
		}
		if len(data.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI14: %s\n", formatFloatSlice(data.RSI14Values)))
		}
	}

	if indicators.EnableATR && data.ATR14 > 0 {
		sb.WriteString(fmt.Sprintf("ATR14: %.4f\n", data.ATR14))
	}

	if indicators.EnableBOLL && len(data.BOLLUpper) > 0 {
		sb.WriteString(fmt.Sprintf("BOLL Upper: %s\n", formatFloatSlice(data.BOLLUpper)))
		sb.WriteString(fmt.Sprintf("BOLL Middle: %s\n", formatFloatSlice(data.BOLLMiddle)))
		sb.WriteString(fmt.Sprintf("BOLL Lower: %s\n", formatFloatSlice(data.BOLLLower)))
	}

	sb.WriteString("\n")
}

func formatFlowValue(v float64) string {
	sign := ""
	if v >= 0 {
		sign = "+"
	}
	absV := v
	if absV < 0 {
		absV = -absV
	}
	if absV >= 1e9 {
		return fmt.Sprintf("%s%.2fB", sign, v/1e9)
	} else if absV >= 1e6 {
		return fmt.Sprintf("%s%.2fM", sign, v/1e6)
	} else if absV >= 1e3 {
		return fmt.Sprintf("%s%.2fK", sign, v/1e3)
	}
	return fmt.Sprintf("%s%.2f", sign, v)
}

func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.4f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// ============================================================================
// Portfolio-concentration & history annotations (candidate rows)
// ============================================================================

// pearsonOfReturns computes the Pearson correlation of two price series'
// 1-bar percent returns over the aligned tail (up to 28 samples — the same
// window the signal layer's BTC correlation uses). Returns 0 when either
// series is too short or degenerate.
func pearsonOfReturns(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 12 {
		return 0
	}
	a = a[len(a)-n:]
	b = b[len(b)-n:]
	var ra, rb []float64
	for i := 1; i < n; i++ {
		if a[i-1] > 0 && b[i-1] > 0 {
			ra = append(ra, a[i]/a[i-1]-1)
			rb = append(rb, b[i]/b[i-1]-1)
		}
	}
	if len(ra) < 10 {
		return 0
	}
	var sa, sb float64
	for i := range ra {
		sa += ra[i]
		sb += rb[i]
	}
	ma, mb := sa/float64(len(ra)), sb/float64(len(rb))
	var cov, va, vb float64
	for i := range ra {
		da, db := ra[i]-ma, rb[i]-mb
		cov += da * db
		va += da * da
		vb += db * db
	}
	if va == 0 || vb == 0 {
		return 0
	}
	return cov / (sqrtF(va) * sqrtF(vb))
}

func sqrtF(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 40; i++ { // Newton-Raphson, avoids importing math for one call
		x = 0.5 * (x + v/x)
		if x*x-v < 1e-12 && x*x-v > -1e-12 {
			break
		}
	}
	return x
}

// closes1hFromMarketData extracts the 1h close series from the shared market
// snapshot (nil when the symbol has no usable 1h data).
func closes1hFromMarketData(data *market.Data) []float64 {
	if data == nil {
		return nil
	}
	tf, ok := data.TimeframeData["1h"]
	if !ok || tf == nil || len(tf.Klines) < 12 {
		return nil
	}
	out := make([]float64, len(tf.Klines))
	for i, k := range tf.Klines {
		out[i] = k.Close
	}
	return out
}

// concentrationWarnings lists open positions whose 1h returns are highly
// correlated with the candidate's. Candidate direction is not known while
// building the prompt, so this is neutral evidence; the model applies it only
// after selecting long/short. Pure over the shared snapshot: no extra calls.
func concentrationWarnings(ctx *Context, symbol string) []string {
	cand := closes1hFromMarketData(ctx.MarketDataMap[symbol])
	if len(cand) == 0 {
		return nil
	}
	var out []string
	for _, p := range ctx.Positions {
		if p.Symbol == symbol {
			continue
		}
		posCloses := closes1hFromMarketData(ctx.MarketDataMap[p.Symbol])
		if len(posCloses) == 0 {
			continue
		}
		r := pearsonOfReturns(cand, posCloses)
		if r >= 0.75 {
			out = append(out, fmt.Sprintf("与持仓%s的1h收益相关性%.2f(方向确定后再判断是集中敞口还是对冲)", p.Symbol, r))
		}
	}
	return out
}

// recentHistoryNote summarizes this trader's own closed-trade record on the
// symbol when it has been loss-heavy — the "same logic, same place, same
// loss" guard rail rendered as evidence next to the candidate.
func recentHistoryNote(ctx *Context, symbol string) string {
	st, ok := ctx.SymbolStats[market.Normalize(symbol)]
	if !ok || st == nil || st.ClosedTrades < 2 {
		return ""
	}
	if st.WinRatePct < 50 && st.RealizedPnL < 0 {
		return fmt.Sprintf("近期该币%d笔交易%d胜(胜率%.0f%%),净亏%.2fU——同位置重复亏损风险", st.ClosedTrades, st.Wins, st.WinRatePct, st.RealizedPnL)
	}
	return ""
}

// expectancyR estimates the average outcome in R multiples from aggregate
// stats: winners average (AvgWin/AvgLoss) R, losers are assumed ≈ -1R
// (risk-based sizing makes that approximately true). 0 when AvgLoss is 0 or
// no trades yet.
func expectancyR(winRatePct, avgWin, avgLoss float64) float64 {
	if avgLoss <= 0 || winRatePct <= 0 {
		return 0
	}
	w := winRatePct / 100
	return w*(avgWin/avgLoss) - (1 - w)
}
