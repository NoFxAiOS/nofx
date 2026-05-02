package kernel

import (
	"fmt"
	"nofx/market"
	"nofx/provider/nofxos"
	"nofx/store"
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
	decisionMode := strings.ToLower(strings.TrimSpace(variant))
	allowAIClose := true
	if strings.Contains(decisionMode, "|no_close") {
		allowAIClose = false
		decisionMode = strings.ReplaceAll(decisionMode, "|no_close", "")
	}

	// 0. Data Dictionary & Schema (ensure AI understands all fields)
	lang := e.GetLanguage()
	schemaPrompt := GetSchemaPrompt(lang)
	sb.WriteString(schemaPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("---\n\n")

	// 0.1 Hard language contract — enforce decision/reasoning language
	if lang == LangChinese {
		sb.WriteString("## ⚠️ 输出语言硬性要求\n")
		sb.WriteString("- 你必须用**中文**输出所有分析、reasoning、entry_protection_rationale、protection_plan 说明性字段、reason_anchor、structural_anchor、notes、alignment_notes 等文本内容\n")
		sb.WriteString("- 决策 JSON 的字段名保持英文 schema 不变，但字段值里的自然语言解释必须是中文\n")
		sb.WriteString("- 不要输出英文分析，不要中英混写，除非是币种、周期、字段名或技术术语缩写\n")
		sb.WriteString("- 如果输出语言不是中文，视为不合格输出\n\n")
	} else {
		sb.WriteString("## ⚠️ Output Language Contract\n")
		sb.WriteString("- You MUST use **English** for all reasoning, explanatory text, entry_protection_rationale, protection_plan explanation fields, reason_anchor, structural_anchor, notes, alignment_notes, etc.\n")
		sb.WriteString("- Keep JSON field names in the schema unchanged, but all natural-language values inside the JSON must be English\n")
		sb.WriteString("- Do not output Chinese analysis or mixed Chinese-English prose unless it is a symbol, timeframe, field name, or technical abbreviation\n")
		sb.WriteString("- If the output language is not English, it is invalid\n\n")
	}

	// 1. Role definition (editable)
	if promptSections.RoleDefinition != "" {
		sb.WriteString(promptSections.RoleDefinition)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# You are a professional cryptocurrency trading AI\n\n")
		sb.WriteString("Your task is to make trading decisions based on provided market data.\n\n")
	}

	// 2. Trading mode variant
	switch decisionMode {
	case "aggressive":
		sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥ 70\n- Allow higher positions, but must strictly set stop-loss and explain risk-reward ratio\n\n")
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open positions when multiple signals resonate\n- Prioritize cash preservation, must pause for multiple periods after consecutive losses\n\n")
	case "balanced", "":
		sb.WriteString("## Mode: Balanced\n- Balance opportunity capture and risk control\n- Prefer clear setups with sufficient confirmation, but do not become overly passive\n\n")
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
	sb.WriteString(fmt.Sprintf("- Max Positions: %d coins simultaneously\n", riskControl.MaxPositions))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (Altcoins): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*altcoinPosValueRatio, accountEquity, altcoinPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (BTC/ETH): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*btcEthPosValueRatio, accountEquity, btcEthPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
	minExecutablePositionSize := riskControl.MinPositionSize
	if minExecutablePositionSize <= 0 {
		minExecutablePositionSize = 12
	}
	btcEthExecutableMin := minExecutablePositionSize
	if accountEquity > 0 {
		if adaptiveMin := accountEquity * 0.9; adaptiveMin > 0 && adaptiveMin < btcEthExecutableMin {
			btcEthExecutableMin = adaptiveMin
		}
	}
	if btcEthExecutableMin < 5 {
		btcEthExecutableMin = 5
	}
	sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT (BTC/ETH on small accounts may use the executable floor around %.0f USDT)\n\n", minExecutablePositionSize, btcEthExecutableMin))

	sb.WriteString("## AI GUIDED (Recommended, you should follow):\n")
	sb.WriteString(fmt.Sprintf("- Trading Leverage: Altcoins max %dx | BTC/ETH max %dx\n",
		riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
	sb.WriteString(fmt.Sprintf("- Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss)\n", riskControl.MinRiskRewardRatio))
	sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d to open position\n\n", riskControl.MinConfidence))

	// Position sizing guidance
	sb.WriteString("## Position Sizing Guidance\n")
	sb.WriteString("Calculate `position_size_usd` based on your confidence and the Position Value Limits above:\n")
	sb.WriteString("- High confidence (≥85): Use 80-100%% of max position value limit\n")
	sb.WriteString("- Medium confidence (70-84): Use 65-85%% of max position value limit\n")
	sb.WriteString("- Low confidence (60-69): Use 50-70%% of max position value limit\n")
	sb.WriteString(fmt.Sprintf("- Example: With equity %.0f and BTC/ETH ratio %.1fx, max is %.0f USDT\n",
		accountEquity, btcEthPosValueRatio, accountEquity*btcEthPosValueRatio))
	sb.WriteString(fmt.Sprintf("- For any open decision, `position_size_usd` must stay above the executable floor. On this account, BTC/ETH opens should generally not be below about %.0f USDT unless venue constraints explicitly allow it. Avoid tiny probe sizes that are likely to fail validation or venue minimums.\n", btcEthExecutableMin))
	sb.WriteString("- **DO NOT** just use available_balance as position_size_usd. Use the Position Value Limits!\n\n")

	// 4. Trading frequency (editable)
	if promptSections.TradingFrequency != "" {
		sb.WriteString(promptSections.TradingFrequency)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# ⏱️ Trading Frequency Awareness\n\n")
		sb.WriteString("- Excellent traders: 2-4 trades/day ≈ 0.1-0.2 trades/hour\n")
		sb.WriteString("- >2 trades/hour = Overtrading\n")
		sb.WriteString("- Single position hold time ≥ 30-60 minutes\n")
		sb.WriteString("If you find yourself trading every period → standards too low; if closing positions < 30 minutes → too impatient.\n\n")
	}

	// 5. Entry standards (editable)
	if promptSections.EntryStandards != "" {
		sb.WriteString(promptSections.EntryStandards)
		sb.WriteString("\n\nYou have the following indicator data:\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\n**Confidence ≥ %d** required to open positions.\n\n", riskControl.MinConfidence))
	} else {
		sb.WriteString("# 🎯 Entry Standards (Strict)\n\n")
		sb.WriteString("Only open positions when multiple signals resonate. You have:\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\nFeel free to use any effective analysis method, but **confidence ≥ %d** required to open positions; avoid low-quality behaviors such as single indicators, contradictory signals, sideways consolidation, reopening immediately after closing, etc.\n\n", riskControl.MinConfidence))
	}

	// Structural analysis requirements
	sb.WriteString("# 🏗️ Structural Analysis Requirements\n\n")
	sb.WriteString("You receive auto-detected support/resistance levels and Fibonacci retracements for EACH timeframe in the market data.\n")
	sb.WriteString("Higher timeframes (1h, 4h) provide stronger structural levels; lower timeframes (5m, 15m) provide precision.\n\n")
	sb.WriteString("## How to use multi-timeframe structural data:\n\n")
	sb.WriteString("1. **Entry positioning**: Open positions near support (long) or resistance (short), not in no-man's land\n")
	sb.WriteString("2. **Protection planning — MUST use multi-timeframe structure**:\n")
	sb.WriteString("   - **Stop Loss**: Place beyond the nearest structural invalidation level on the PRIMARY or HIGHER timeframe. Add ATR-based buffer (0.3-0.5x ATR) to avoid stop-hunts/wicks\n")
	sb.WriteString("   - **Take Profit / Ladder TP**: Align with resistance/fib levels (longs) or support/fib levels (shorts). Use HIGHER timeframe levels for major targets, lower timeframe for partial exits\n")
	sb.WriteString("   - **Drawdown rules**: Each profit stage's min_profit_pct should correspond to a structural level distance from entry. max_drawdown_pct is percentage-of-peak-profit giveback (exchange trailing semantics), e.g. 55 means allow 55% of peak profit to be given back before closing; it is NOT 0.55% absolute price/profit drawdown.\n")
	sb.WriteString("   - **Break-even trigger**: Set trigger_value near the first structural level past entry, with offset beyond the nearest support/resistance\n")
	sb.WriteString("3. **Volatility buffer**: All SL/TP/drawdown thresholds must account for typical wick range. Use ATR14 from the relevant timeframe as the volatility gauge. A stop placed exactly at a structural level WILL get swept — always add buffer\n")
	sb.WriteString("4. **Cross-validation**: Auto-detected levels are hints. Confirm with volume, price action, and multi-timeframe alignment\n")
	sb.WriteString("5. **structural_key_levels in output**: When opening, include a `structural_key_levels` array listing the structural levels that influenced your entry/TP/SL/drawdown decisions, with the timeframe each came from\n")
	sb.WriteString("6. **Higher-timeframe runner context**: If `timeframe_context.higher` is present, include `higher_timeframe_anchors` or `timeframe_structures` with explicit higher-TF price anchors. Outer drawdown/runner stages must cite those higher-TF anchors, not only primary-TF resistance/support text.\n\n")
	sb.WriteString("## Protection Plan Requirements (when mode = ai):\n\n")
	sb.WriteString("### For ladder mode=ai:\n")
	sb.WriteString("- Each ladder TP target MUST correspond to a nearby structural level (support/resistance/fibonacci) from the relevant timeframe\n")
	sb.WriteString("- Include a `structural_anchor` field in each ladder rule explaining which level + timeframe it references\n")
	sb.WriteString("- Position sizing per tier should reflect distance to the structural target and confidence\n")
	sb.WriteString("- SL placement: beyond the nearest invalidation level on primary/higher TF, plus ATR buffer to survive wicks\n")
	sb.WriteString("- DO NOT use arbitrary round percentages (like 1%%, 2%%, 3%%) - use market structure\n\n")
	sb.WriteString("### For drawdown mode=ai:\n")
	sb.WriteString("- Each drawdown rule represents a profit protection stage. Design stages around structural targets:\n")
	sb.WriteString("- You MUST output at least 2 `drawdown_rules` for every drawdown/combined protection plan:\n")
	sb.WriteString("  - Stage 1 = partial profit lock near the first primary-timeframe structure: generous `max_drawdown_pct`, smaller `close_ratio_pct`, preserve runner size\n")
	sb.WriteString("  - Stage 2+ = outer runner/trend protection anchored to higher-timeframe structure: wider profit target, structurally justified drawdown tolerance, closes more only after trend extension or structure failure\n")
	sb.WriteString("- The outer runner stage should use primary/higher timeframe trend structure and ATR, not lower-timeframe noise; allow normal retests/wicks so profitable positions can keep running\n")
	sb.WriteString("- max_drawdown_pct MUST factor in ATR volatility: if ATR14 is 0.5%, a max_drawdown of 0.3% will trigger on normal noise. Use at least 1-2x ATR as minimum drawdown tolerance\n")
	sb.WriteString("- Include `reason_anchor` field referencing the specific structural level + timeframe that justifies each stage\n")
	sb.WriteString("- Use the exact field name `close_ratio_pct` in drawdown_rules; do NOT use `close_ratio`\n")
	sb.WriteString("- DO NOT use arbitrary round percentages — derive from actual structural distances\n\n")

	// Dynamic: inject hard requirement when drawdown is actually in AI mode
	prot := e.config.Protection
	if prot.DrawdownTakeProfit.Enabled && prot.DrawdownTakeProfit.Mode == store.ProtectionModeAI {
		if prot.LadderTPSL.Enabled && prot.LadderTPSL.Mode == store.ProtectionModeAI {
			sb.WriteString("### ⚠️ ACTIVE: Ladder + Drawdown are BOTH in AI mode for this strategy\n")
			sb.WriteString("- You MUST include one `protection_plan` with `mode=\"combined\"` for every open_long/open_short decision\n")
			sb.WriteString("- The combined plan MUST include both non-empty `ladder_rules` and at least 2 `drawdown_rules`; omission or a single drawdown stage rejects the trade\n")
			sb.WriteString("- `ladder_rules` own staged stop-loss / optional staged TP; derive absolute prices, percentages, buffers, and close ratios from structure, not from default round numbers\n")
			sb.WriteString("- Every ladder rule must include a volatility/wick buffer using ATR or recent wick behavior; do not place stops/targets exactly on crowded structure/fib levels\n")
			sb.WriteString("- `drawdown_rules` own profit-protection/trailing stages; derive min_profit_pct/max_drawdown_pct/close_ratio_pct from structural targets and volatility\n")
			sb.WriteString("- Include structural_anchor on every ladder rule and reason_anchor on every drawdown rule\n")
			sb.WriteString(fmt.Sprintf("- Strategy has %d default ladder rule(s) and %d default drawdown rule(s) only as reference; do not copy them unless structure justifies them\n", len(prot.LadderTPSL.Rules), len(prot.DrawdownTakeProfit.Rules)))
			sb.WriteString("\n")
		} else {
			sb.WriteString("### ⚠️ ACTIVE: Drawdown Take Profit is in AI mode for this strategy\n")
			sb.WriteString("- You MUST include `protection_plan` with `mode=\"drawdown\"` and at least 2 `drawdown_rules` for every open_long/open_short decision\n")
			sb.WriteString("- Rule 1 should partially lock profit near the first primary-timeframe structural target; rule 2+ should protect a runner using primary/higher timeframe structure and ATR tolerance\n")
			sb.WriteString("- Omitting `drawdown_rules` or providing only one drawdown stage will cause the trade to be rejected\n")
			if prot.FullTPSL.Enabled && prot.FullTPSL.Mode == store.ProtectionModeAI {
				sb.WriteString("- Combined ownership mode is active: drawdown AI owns profit-taking / profit-protection, while full AI remains strategy-level stop-loss / fallback stop protection\n")
				sb.WriteString("- In this combined mode, DO NOT output `mode=full` in the AI decision. Output only drawdown ownership fields and let strategy-level full stop protection merge at execution time\n")
			}
			sb.WriteString(fmt.Sprintf("- Strategy has %d default drawdown rule(s) as reference; your AI rules should be structurally justified\n", len(prot.DrawdownTakeProfit.Rules)))
			for i, r := range prot.DrawdownTakeProfit.Rules {
				sb.WriteString(fmt.Sprintf("  - Default rule %d: min_profit=%.1f%%, max_drawdown=%.0f%%, close_ratio=%.0f%%\n", i+1, r.MinProfitPct, r.MaxDrawdownPct, r.CloseRatioPct))
			}
			sb.WriteString("\n")
		}
	}
	if prot.BreakEvenStop.Enabled {
		if prot.BreakEvenStop.Mode == store.ProtectionModeAI {
			sb.WriteString("### ⚠️ ACTIVE: Break-even Stop is enabled in AI mode for this strategy\n")
			sb.WriteString("- You MUST include break_even_trigger_mode, break_even_trigger_value, and break_even_offset_pct in protection_plan for every open action\n")
			sb.WriteString(fmt.Sprintf("  - Manual fallback/reference: trigger_mode=%s, trigger_value=%.1f, offset=%.2f%%\n", prot.BreakEvenStop.TriggerMode, prot.BreakEvenStop.TriggerValue, prot.BreakEvenStop.OffsetPct))
		} else {
			sb.WriteString("### ⚠️ ACTIVE: Break-even Stop is enabled in manual mode for this strategy\n")
			sb.WriteString("- Break-even uses the strategy manual trigger/offset; do NOT invent AI break-even values unless another AI protection route needs rationale text\n")
			sb.WriteString(fmt.Sprintf("  - Manual: trigger_mode=%s, trigger_value=%.1f, offset=%.2f%%\n", prot.BreakEvenStop.TriggerMode, prot.BreakEvenStop.TriggerValue, prot.BreakEvenStop.OffsetPct))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("### For break_even mode:\n")
	sb.WriteString("- trigger_value should be set near the first structural level past entry (e.g. first resistance for long, first support for short)\n")
	sb.WriteString("- offset_pct should keep the stop just beyond the nearest support/resistance, plus ATR buffer to avoid wick sweeps\n")
	sb.WriteString("- Reference the specific structural level and timeframe in break_even_reason_anchor\n\n")

	// 6. Decision process (editable)
	if promptSections.DecisionProcess != "" {
		sb.WriteString(promptSections.DecisionProcess)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# 📋 Decision Process\n\n")
		sb.WriteString("1. Check positions → Should we take profit/stop-loss\n")
		sb.WriteString("2. Scan candidate coins + multi-timeframe → Are there strong signals\n")
		sb.WriteString("3. Write chain of thought first, then output structured JSON\n\n")
	}

	// 7. Output format
	if !allowAIClose {
		sb.WriteString("# AI Close Gate\n\n")
		sb.WriteString("- You are NOT allowed to output `close_long` or `close_short`.\n")
		sb.WriteString("- Existing positions may only be closed by code protection and exchange protection orders.\n")
		sb.WriteString("- You must continue analyzing open positions, but if you want a close, output `hold` and explain the risk instead.\n\n")
	}

	// 7. Output format
	sb.WriteString("# Output Format (Strictly Follow)\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON, avoiding parsing errors**\n\n")
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Your chain of thought analysis...\n")
	sb.WriteString("- Briefly analyze your thinking process \n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("Step 2: JSON decision array\n\n")
	sb.WriteString("```json\n[\n")
	// Use the actual configured position value ratio for BTC/ETH in the example
	examplePositionSize := accountEquity * btcEthPosValueRatio
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"entry_protection_rationale\": {\"timeframe_context\": {\"primary\": \"15m\", \"lower\": [\"5m\"], \"higher\": [\"1h\"]}, \"risk_reward\": {\"entry\": 95000, \"invalidation\": 97000, \"first_target\": 91000, \"gross_estimated_rr\": 2.0, \"net_estimated_rr\": 1.8, \"min_required_rr\": %.1f, \"passed\": true}, \"anchors\": [{\"type\": \"resistance\", \"timeframe\": \"15m\", \"price\": 96000, \"reason\": \"primary rejection\"}], \"alignment_notes\": [\"full stop remains beyond invalidation\"]}, \"confidence\": 85, \"risk_usd\": 300},\n", riskControl.BTCETHMaxLeverage, examplePositionSize, riskControl.MinRiskRewardRatio))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\"}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Field Description\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("- Optional reliability fields for every decision: `regime` (trend_up|trend_down|range|squeeze|chop|news_risk|no_trade), `setup_type` (trend_pullback|range_edge|breakout_retest|none), and `quality_score` with total/trend_alignment/structure_location/sr_fib_quality/derivatives_context/trigger_quality/net_rr. These fields are currently audit/shadow fields, but strong opens should include them.\n")
	sb.WriteString("- Default to `wait` unless the setup is one of trend_pullback, range_edge, or breakout_retest with clear multi-timeframe alignment and acceptable derivatives/crowding context.\n")
	sb.WriteString("- `protection_plan`: optional structured protection output for open actions only\n")
	sb.WriteString("- Ladder rule price fields: explicit absolute `take_profit_price` / `stop_loss_price` (or aliases `tp_level` / `sl_level`) are required when structure exists; `take_profit_pct` / `stop_loss_pct` are only equivalent UI/audit percentages, not the source of truth. Include `take_profit_anchor` / `stop_loss_anchor` or `structural_anchor` naming the support/resistance/fibonacci/invalidation level, plus a volatility/wick buffer (`volatility_buffer_pct` or `volatility_buffer_reason`) based on ATR/recent wicks. Do not put stops/targets naked exactly on crowded structural levels: invalidation should require an effective break, not a one-tick touch. Never output generic 0.9% / 1.5% ladder stops unless those exact percentages are back-calculated from explicit structural prices plus buffer.\n")
	sb.WriteString("- `entry_protection_rationale`: required for `open_long` / `open_short`; must include timeframe_context, risk_reward (entry/invalidation/first_target/gross_estimated_rr and preferably net_estimated_rr), and structural anchors when opening\n")
	sb.WriteString("  - If timeframe_context.higher is present, you MUST include at least one higher timeframe structural anchor in `higher_timeframe_anchors` or `timeframe_structures`, with type/timeframe/price/reason\n")
	sb.WriteString("  - Treat structural entry as a compact contract, not a verbose essay: include only the few levels/anchors needed to justify entry, invalidation, and first target\n")
	sb.WriteString("  - When strategy `entry_structure` is enabled, you MUST provide the required structural fields (primary timeframe, adjacent timeframe, support/resistance, anchors, and fibonacci only when explicitly required) or output wait/[] instead of forcing an open\n")
	sb.WriteString("  - Use exchange/runtime market data only to extract the necessary structure for judgment; do not dump every indicator or noisy field\n")
	sb.WriteString("  - Use `mode=full` when one unified TP/SL plan is enough\n")
	sb.WriteString("  - For `mode=full`, output `take_profit_pct` / `stop_loss_pct` only; do not place absolute price fields inside protection_plan\n")
	sb.WriteString("  - Use `mode=ladder` when you want staged TP/SL with multiple ladder_rules\n")
	sb.WriteString("  - For ladder_rules, output absolute structural prices as `take_profit_price` / `stop_loss_price`; include equivalent `take_profit_pct` / `stop_loss_pct` only after calculating them from the absolute prices for display/audit. Include `take_profit_anchor` / `stop_loss_anchor` or `structural_anchor`, and include ATR/recent-wick buffer so stops are beyond invalidation and TP tiers are not exactly on crowded levels. Structural linkage means anchored-with-buffer, not exact equality: a stop below support/above resistance should allow effective breakdown/breakout confirmation, and TP should usually sit slightly before crowded resistance/support for fill realism. Percent-only ladder rules are allowed only when no structural levels exist; otherwise they may be normalized to structure or rejected.\n")
	sb.WriteString("  - Use `mode=drawdown` when the strategy route enables AI drawdown profit protection; then `drawdown_rules` must be non-empty\n")
	sb.WriteString("  - Use `mode=break_even` when the strategy enables Break-even Stop as an AI-required runtime stop layer; include break_even_trigger_mode/value/offset\n")
	sb.WriteString("  - In drawdown/break-even AI mode, reasoning must reference the primary timeframe, adjacent timeframes, and structural anchors such as support/resistance, fibonacci, and volatility\n")
	sb.WriteString("  - If Drawdown Take Profit is enabled in strategy config, your reasoning must explicitly mention drawdown, trailing, or profit-protection ownership\n")
	sb.WriteString("  - If Break-even Stop is enabled in strategy config, your reasoning must explicitly mention break-even or acknowledge that an additional stop layer exists after profit trigger\n")
	sb.WriteString("  - Do NOT output protection_plan for hold/wait/close actions\n")
	sb.WriteString(fmt.Sprintf("- `confidence`: 0-100 (opening recommended ≥ %d)\n", riskControl.MinConfidence))
	sb.WriteString(fmt.Sprintf("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd, entry_protection_rationale; risk_reward must satisfy min RR ≥ %.1f and direction sanity (long: invalidation < entry < first_target, short: invalidation > entry > first_target)\n", riskControl.MinRiskRewardRatio))
	sb.WriteString("- Structural entry fields should be compact and purpose-driven: primary/adjacent timeframe, top support/resistance, one or a few anchors, and fibonacci only when it materially affects invalidation/target planning\n")
	sb.WriteString("- `entry_protection_rationale.key_levels.support` and `entry_protection_rationale.key_levels.resistance` are REQUIRED for open actions when structural entry is enabled; provide only the most decision-relevant structural levels and stay within configured caps (typically support<=3, resistance<=3) instead of dumping every visible level\n")
	sb.WriteString("- `structural_key_levels`: structural levels that influenced protection placement decisions; each must specify price, type (support/resistance), timeframe, source, and what it was used_for (tp1/tp2/stop_loss/invalidation)\n")
	sb.WriteString("- If you provide `structural_key_levels`, make sure they are consistent with key_levels.support/resistance; do not leave support/resistance empty\n")
	sb.WriteString("- **IMPORTANT**: All numeric values must be calculated numbers, NOT formulas/expressions (e.g., use `27.76` not `3000 * 0.01`)\n")
	sb.WriteString("- **STRICT JSON NUMBER RULE**: JSON numeric fields must use plain digits with optional decimal point only. Never use thousands separators, grouping commas, spaces, or localized punctuation in numeric fields. Correct: `97687.05`, `77048.9`, `2293.23`. Wrong: `97,687.05`, `77,048.9`, `2,293.23`, `9,76887.05`. If you want to mention comma-formatted prices, put them only inside quoted natural-language strings, never in numeric fields.\n\n")

	// 8. Custom Prompt
	if e.config.CustomPrompt != "" {
		sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		sb.WriteString(e.config.CustomPrompt)
		sb.WriteString("\n\n")
		sb.WriteString("Note: The above personalized strategy is a supplement to the basic rules and cannot violate the basic risk control principles.\n")
	}

	return sb.String()
}

func (e *StrategyEngine) writeAvailableIndicators(sb *strings.Builder) {
	indicators := e.config.Indicators
	kline := indicators.Klines

	sb.WriteString(fmt.Sprintf("- %s price series", kline.PrimaryTimeframe))
	if kline.EnableMultiTimeframe {
		sb.WriteString(fmt.Sprintf(" + %s K-line series\n", kline.LongerTimeframe))
	} else {
		sb.WriteString("\n")
	}

	if indicators.EnableEMA {
		sb.WriteString("- EMA indicators")
		if len(indicators.EMAPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.EMAPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableMACD {
		sb.WriteString("- MACD indicators\n")
	}

	if indicators.EnableRSI {
		sb.WriteString("- RSI indicators")
		if len(indicators.RSIPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.RSIPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableATR {
		sb.WriteString("- ATR indicators")
		if len(indicators.ATRPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.ATRPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableBOLL {
		sb.WriteString("- Bollinger Bands (BOLL) - Upper/Middle/Lower bands")
		if len(indicators.BOLLPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.BOLLPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableVolume {
		sb.WriteString("- Volume data\n")
	}

	if indicators.EnableOI {
		sb.WriteString("- Open Interest (OI) data\n")
	}

	if indicators.EnableFundingRate {
		sb.WriteString("- Funding rate\n")
	}

	if len(e.config.CoinSource.StaticCoins) > 0 || e.config.CoinSource.UseAI500 || e.config.CoinSource.UseOITop {
		sb.WriteString("- AI500 / OI_Top filter tags (if available)\n")
	}

	if indicators.EnableQuantData {
		sb.WriteString("- Quantitative data (institutional/retail fund flow, position changes, multi-period price changes)\n")
	}
}

// ============================================================================
// Prompt Building - User Prompt
// ============================================================================

// BuildUserPrompt builds User Prompt based on strategy configuration
func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder
	lang := e.GetLanguage()

	// Hard language reminder in user prompt too (reinforces system prompt)
	if lang == LangChinese {
		sb.WriteString("【硬性要求】本次所有分析、解释、结构理由、保护方案说明必须使用中文；JSON 字段名保持英文，但字段值中的自然语言文本必须是中文。\n\n")
	} else {
		sb.WriteString("[HARD REQUIREMENT] All analysis, explanations, structural rationale, and protection-plan explanatory text must be in English. Keep JSON field names in English, but any natural-language values inside JSON must also be English.\n\n")
	}

	// System status
	if lang == LangChinese {
		sb.WriteString(fmt.Sprintf("时间: %s | 周期: #%d | 运行时长: %d 分钟\n\n",
			ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))
	} else {
		sb.WriteString(fmt.Sprintf("Time: %s | Period: #%d | Runtime: %d minutes\n\n",
			ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))
	}

	// BTC market
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("BTC: %s (1h: %s%%, 4h: %s%%) | MACD: %s | RSI: %s\n\n",
				formatAIFloat(btcData.CurrentPrice), formatAISignedFloat(btcData.PriceChange1h), formatAISignedFloat(btcData.PriceChange4h),
				formatAIFloat(btcData.CurrentMACD), formatAIFloat(btcData.CurrentRSI7)))
		} else {
			sb.WriteString(fmt.Sprintf("BTC: %s (1h: %s%%, 4h: %s%%) | MACD: %s | RSI: %s\n\n",
				formatAIFloat(btcData.CurrentPrice), formatAISignedFloat(btcData.PriceChange1h), formatAISignedFloat(btcData.PriceChange4h),
				formatAIFloat(btcData.CurrentMACD), formatAIFloat(btcData.CurrentRSI7)))
		}
	}

	// Account information
	if lang == LangChinese {
		sb.WriteString(fmt.Sprintf("账户: 权益 %s | 可用余额 %s (%s%%) | 盈亏 %s%% | 保证金 %s%% | 持仓 %d\n\n",
			formatAIFloat(ctx.Account.TotalEquity),
			formatAIFloat(ctx.Account.AvailableBalance),
			formatAIFloat((ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100),
			formatAISignedFloat(ctx.Account.TotalPnLPct),
			formatAIFloat(ctx.Account.MarginUsedPct),
			ctx.Account.PositionCount))
	} else {
		sb.WriteString(fmt.Sprintf("Account: Equity %s | Balance %s (%s%%) | PnL %s%% | Margin %s%% | Positions %d\n\n",
			formatAIFloat(ctx.Account.TotalEquity),
			formatAIFloat(ctx.Account.AvailableBalance),
			formatAIFloat((ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100),
			formatAISignedFloat(ctx.Account.TotalPnLPct),
			formatAIFloat(ctx.Account.MarginUsedPct),
			ctx.Account.PositionCount))
	}

	// Recently completed orders (placed before positions to ensure visibility)
	if len(ctx.RecentOrders) > 0 {
		if lang == LangChinese {
			sb.WriteString("## 最近已完成交易\n")
		} else {
			sb.WriteString("## Recent Completed Trades\n")
		}
		for i, order := range ctx.RecentOrders {
			resultStr := "Profit"
			if lang == LangChinese {
				resultStr = "盈利"
			}
			if order.RealizedPnL < 0 {
				if lang == LangChinese {
					resultStr = "亏损"
				} else {
					resultStr = "Loss"
				}
			}
			if lang == LangChinese {
				sb.WriteString(fmt.Sprintf("%d. %s %s | 开仓 %s 平仓 %s | %s: %s USDT (%s%%) | %s→%s (%s)\n",
					i+1, order.Symbol, order.Side,
					formatAIFloat(order.EntryPrice), formatAIFloat(order.ExitPrice),
					resultStr, formatAISignedFloat(order.RealizedPnL), formatAISignedFloat(order.PnLPct),
					order.EntryTime, order.ExitTime, order.HoldDuration))
			} else {
				sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %s Exit %s | %s: %s USDT (%s%%) | %s→%s (%s)\n",
					i+1, order.Symbol, order.Side,
					formatAIFloat(order.EntryPrice), formatAIFloat(order.ExitPrice),
					resultStr, formatAISignedFloat(order.RealizedPnL), formatAISignedFloat(order.PnLPct),
					order.EntryTime, order.ExitTime, order.HoldDuration))
			}
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

		if lang == LangChinese {
			sb.WriteString("## 历史交易统计\n")
			sb.WriteString(fmt.Sprintf("总交易: %d 笔 | 盈利因子: %.2f | 夏普比率: %.2f | 盈亏比: %.2f\n",
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.ProfitFactor,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("总盈亏: %+.2f USDT | 平均盈利: +%.2f | 平均亏损: -%.2f | 最大回撤: %.1f%%\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("表现: 良好 - 保持当前策略\n")
			} else if ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("表现: 需改进 - 提高盈亏比，优化止盈止损\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("表现: 风险偏高 - 减少仓位，控制回撤\n")
			} else {
				sb.WriteString("表现: 正常 - 有优化空间\n")
			}
		} else {
			sb.WriteString("## Historical Trading Statistics\n")
			sb.WriteString(fmt.Sprintf("Total Trades: %d | Profit Factor: %.2f | Sharpe: %.2f | Win/Loss Ratio: %.2f\n",
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.ProfitFactor,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("Total PnL: %+.2f USDT | Avg Win: +%.2f | Avg Loss: -%.2f | Max Drawdown: %.1f%%\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("Performance: GOOD - maintain current strategy\n")
			} else if ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("Performance: NEEDS IMPROVEMENT - improve win/loss ratio, optimize TP/SL\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("Performance: HIGH RISK - reduce position size, control drawdown\n")
			} else {
				sb.WriteString("Performance: NORMAL - room for optimization\n")
			}
		}
		sb.WriteString("\n")
	}

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

	seenCandidateSymbols := make(map[string]bool)
	displayableCount := 0
	for _, coin := range ctx.CandidateCoins {
		normalizedCoinSymbol := market.Normalize(coin.Symbol)
		if positionSymbols[normalizedCoinSymbol] || seenCandidateSymbols[normalizedCoinSymbol] {
			continue
		}
		if _, hasData := ctx.MarketDataMap[coin.Symbol]; !hasData {
			continue
		}
		seenCandidateSymbols[normalizedCoinSymbol] = true
		displayableCount++
	}

	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d coins)\n\n", displayableCount))
	displayedCount := 0
	displayedCandidateSymbols := make(map[string]bool)
	for _, coin := range ctx.CandidateCoins {
		// Skip if this coin is already a position (data already shown in positions section)
		normalizedCoinSymbol := market.Normalize(coin.Symbol)
		if positionSymbols[normalizedCoinSymbol] || displayedCandidateSymbols[normalizedCoinSymbol] {
			continue
		}

		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCandidateSymbols[normalizedCoinSymbol] = true
		displayedCount++

		sourceTags := e.formatCoinSourceTag(coin.Sources)
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(e.formatMarketData(marketData))
		sb.WriteString(e.formatMarketContextV2(coin.Symbol, marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[coin.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Get language for market data formatting
	nofxosLang := nofxos.LangEnglish
	if e.GetLanguage() == LangChinese {
		nofxosLang = nofxos.LangChinese
	}

	// OI Ranking data (market-wide open interest changes)
	if ctx.OIRankingData != nil {
		sb.WriteString(nofxos.FormatOIRankingForAI(ctx.OIRankingData, nofxosLang))
	}

	// NetFlow Ranking data (market-wide fund flow)
	if ctx.NetFlowRankingData != nil {
		sb.WriteString(nofxos.FormatNetFlowRankingForAI(ctx.NetFlowRankingData, nofxosLang))
	}

	// Price Ranking data (market-wide gainers/losers)
	if ctx.PriceRankingData != nil {
		sb.WriteString(nofxos.FormatPriceRankingForAI(ctx.PriceRankingData, nofxosLang))
	}

	// Optional data availability: never fail closed; continue with available exchange/market data.
	if len(ctx.OptionalDataStates) > 0 {
		sb.WriteString("## Optional Data Availability\n")
		for _, state := range ctx.OptionalDataStates {
			status := "missing"
			if state.Available {
				status = "available"
			}
			sb.WriteString(fmt.Sprintf("- source=%s status=%s", state.Source, status))
			if state.Reason != "" {
				sb.WriteString(fmt.Sprintf(" reason=%s", state.Reason))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("Rule: optional data absence is not a no-trade signal by itself; use remaining price/structure/exchange data and mention uncertainty if relevant.\n\n")
	}

	sb.WriteString("---\n\n")
	sb.WriteString("Now please analyze and output your decision (Chain of Thought + JSON)\n")

	return sb.String()
}

func (e *StrategyEngine) formatPositionInfo(index int, pos PositionInfo, ctx *Context) string {
	var sb strings.Builder

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

	sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %s Current %s | Qty %s | Position Value %s USDT | PnL%s%% | PnL Amount%s USDT | Peak PnL%s%% | Leverage %dx | Margin %s | Liq Price %s%s\n\n",
		index, pos.Symbol, strings.ToUpper(pos.Side),
		formatAIFloat(pos.EntryPrice), formatAIFloat(pos.MarkPrice), formatAIFloat(pos.Quantity), formatAIFloat(positionValue), formatAISignedFloat(pos.UnrealizedPnLPct), formatAISignedFloat(pos.UnrealizedPnL), formatAIFloat(pos.PeakPnLPct),
		pos.Leverage, formatAIFloat(pos.MarginUsed), formatAIFloat(pos.LiquidationPrice), holdingDuration))

	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
		sb.WriteString(e.formatMarketData(marketData))
		sb.WriteString(e.formatMarketContextV2(pos.Symbol, marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[pos.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
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
// Market Data Formatting
// ============================================================================

func (e *StrategyEngine) formatMarketData(data *market.Data) string {
	var sb strings.Builder
	indicators := e.config.Indicators

	// Clearly label the coin symbol
	sb.WriteString(fmt.Sprintf("=== %s Market Data ===\n\n", data.Symbol))
	sb.WriteString(fmt.Sprintf("current_price = %s", formatAIFloat(data.CurrentPrice)))

	if indicators.EnableEMA {
		sb.WriteString(fmt.Sprintf(", current_ema20 = %s", formatAIFloat(data.CurrentEMA20)))
	}

	if indicators.EnableMACD {
		sb.WriteString(fmt.Sprintf(", current_macd = %s", formatAIFloat(data.CurrentMACD)))
	}

	if indicators.EnableRSI {
		sb.WriteString(fmt.Sprintf(", current_rsi7 = %s", formatAIFloat(data.CurrentRSI7)))
	}

	sb.WriteString("\n\n")

	if indicators.EnableOI || indicators.EnableFundingRate {
		sb.WriteString(fmt.Sprintf("Additional data for %s:\n\n", data.Symbol))

		if indicators.EnableOI && data.OpenInterest != nil {
			sb.WriteString(fmt.Sprintf("Open Interest: latest=%s average=%s\n\n",
				formatAIFloat(data.OpenInterest.Latest), formatAIFloat(data.OpenInterest.Average)))
		}

		if indicators.EnableFundingRate {
			sb.WriteString(fmt.Sprintf("Funding Rate: %s\n\n", formatAIFloat(data.FundingRate)))
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
				sb.WriteString(fmt.Sprintf("3m ATR (14-period): %s\n\n", formatAIFloat(data.IntradaySeries.ATR14)))
			}
		}

		if data.LongerTermContext != nil && indicators.Klines.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf("Longer-term context (%s timeframe):\n\n", indicators.Klines.LongerTimeframe))

			if indicators.EnableEMA {
				sb.WriteString(fmt.Sprintf("20-Period EMA: %s vs. 50-Period EMA: %s\n\n",
					formatAIFloat(data.LongerTermContext.EMA20), formatAIFloat(data.LongerTermContext.EMA50)))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3-Period ATR: %s vs. 14-Period ATR: %s\n\n",
					formatAIFloat(data.LongerTermContext.ATR3), formatAIFloat(data.LongerTermContext.ATR14)))
			}

			if indicators.EnableVolume {
				sb.WriteString(fmt.Sprintf("Current Volume: %s vs. Average Volume: %s\n\n",
					formatAIFloat(data.LongerTermContext.CurrentVolume), formatAIFloat(data.LongerTermContext.AverageVolume)))
			}

			if indicators.EnableMACD && len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}

			if indicators.EnableRSI && len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}
		}
	}

	// Sentiment and structural data
	sb.WriteString(formatSentimentDataEN(data, indicators))
	sb.WriteString(formatStructuralLevelsEN(data))

	return sb.String()
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
		sb.WriteString(fmt.Sprintf("ATR14: %s\n", formatAIFloat(data.ATR14)))
	}

	if indicators.EnableBOLL && len(data.BOLLUpper) > 0 {
		sb.WriteString(fmt.Sprintf("BOLL Upper: %s\n", formatFloatSlice(data.BOLLUpper)))
		sb.WriteString(fmt.Sprintf("BOLL Middle: %s\n", formatFloatSlice(data.BOLLMiddle)))
		sb.WriteString(fmt.Sprintf("BOLL Lower: %s\n", formatFloatSlice(data.BOLLLower)))
	}

	sb.WriteString("\n")
}

func (e *StrategyEngine) formatMarketContextV2(symbol string, data *market.Data) string {
	ctx := market.BuildMarketContextV2(symbol, data, []string{"3m", "15m", "1h", "4h", "1d"}, "15m")
	if ctx == nil || ctx.RegimeRules == nil {
		return ""
	}
	snapshot := market.BuildCompositeMarketSnapshotFromExistingData("okx", []string{"3m", "15m", "1h", "4h", "1d"}, "15m", 180*time.Second, data)
	if snapshot != nil && snapshot.AICompact != "" {
		return "Composite Market Context (shared human/AI source):\n" + snapshot.AICompact + "  rule: open only when setup_type is compatible with allowed_setups and structural anchors satisfy structure_mode; otherwise wait. For any open with ladder protection, stop_loss_price must be an explicit structural invalidation price beyond support/resistance/fibonacci plus ATR/wick buffer; stop_loss_pct is only a derived display value, never the planning input.\n"
	}
	var sb strings.Builder
	sb.WriteString("Execution Regime Guidance:\n")
	sb.WriteString(fmt.Sprintf("  regime=%s structure_mode=%s fibonacci_mode=%s\n", ctx.RegimeRules.Regime, ctx.RegimeRules.StructureMode, ctx.RegimeRules.FibonacciMode))
	if len(ctx.RegimeRules.AllowedSetups) > 0 {
		sb.WriteString(fmt.Sprintf("  allowed_setups=%s\n", strings.Join(ctx.RegimeRules.AllowedSetups, ",")))
	}
	if len(ctx.RegimeRules.RequiredAnchors) > 0 {
		sb.WriteString(fmt.Sprintf("  required_anchors=%s\n", strings.Join(ctx.RegimeRules.RequiredAnchors, ",")))
	}
	if ctx.RegimeRules.ProtectionGuidance != "" {
		sb.WriteString(fmt.Sprintf("  protection_guidance=%s\n", ctx.RegimeRules.ProtectionGuidance))
	}
	if ctx.Derivatives != nil {
		sb.WriteString(fmt.Sprintf("  derivatives: funding_bias=%s squeeze_risk=%s oi_1h=%.2f%% volume_z=%.2f\n", ctx.Derivatives.FundingBias, ctx.Derivatives.SqueezeRisk, ctx.Derivatives.OIChange1hPct, ctx.Derivatives.VolumeZScore))
	}
	if ctx.Quant != nil && ctx.Quant.DataQuality != "" && ctx.Quant.DataQuality != "missing" {
		sb.WriteString(fmt.Sprintf("  quant: flow_bias=%s crowding=%s inst_future_1h=%s retail_future_1h=%s oi_1h=%.2f%%\n", ctx.Quant.FlowBias, ctx.Quant.CrowdingRisk, formatFlowValue(ctx.Quant.InstitutionFuture1h), formatFlowValue(ctx.Quant.RetailFuture1h), ctx.Quant.OIChange1hPct))
	}
	if ctx.ExchangeFlow != nil && ctx.ExchangeFlow.DataQuality != "" && ctx.ExchangeFlow.DataQuality != "missing" {
		sb.WriteString(fmt.Sprintf("  exchange_flow: funding=%s long_short=%s taker=%s depth=%s crowding=%s depth_total=%s\n", ctx.ExchangeFlow.FundingBias, ctx.ExchangeFlow.LongShortSkew, ctx.ExchangeFlow.TakerFlowBias, ctx.ExchangeFlow.DepthBias, ctx.ExchangeFlow.CrowdingRisk, formatFlowValue(ctx.ExchangeFlow.DepthTotalUSDT)))
	}
	sb.WriteString("  rule: open only when setup_type is compatible with allowed_setups and structural anchors satisfy structure_mode; otherwise wait.\n")
	return sb.String()
}

func (e *StrategyEngine) formatQuantData(data *QuantData) string {
	if data == nil {
		return ""
	}

	indicators := e.config.Indicators
	if !indicators.EnableQuantOI && !indicators.EnableQuantNetflow {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s Quantitative Data:\n", data.Symbol))

	if len(data.PriceChange) > 0 {
		sb.WriteString("Price Change: ")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}
		parts := []string{}
		for _, tf := range timeframes {
			if v, ok := data.PriceChange[tf]; ok {
				parts = append(parts, fmt.Sprintf("%s: %+.4f%%", tf, v*100))
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
	}

	if indicators.EnableQuantNetflow && data.Netflow != nil {
		sb.WriteString("Fund Flow (Netflow):\n")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}

		if data.Netflow.Institution != nil {
			if data.Netflow.Institution.Future != nil && len(data.Netflow.Institution.Future) > 0 {
				sb.WriteString("  Institutional Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if data.Netflow.Institution.Spot != nil && len(data.Netflow.Institution.Spot) > 0 {
				sb.WriteString("  Institutional Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}

		if data.Netflow.Personal != nil {
			if data.Netflow.Personal.Future != nil && len(data.Netflow.Personal.Future) > 0 {
				sb.WriteString("  Retail Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if data.Netflow.Personal.Spot != nil && len(data.Netflow.Personal.Spot) > 0 {
				sb.WriteString("  Retail Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}
	}

	if indicators.EnableQuantOI && len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if len(oiData.Delta) > 0 {
				sb.WriteString(fmt.Sprintf("Open Interest (%s):\n", exchange))
				for _, tf := range []string{"5m", "15m", "1h", "4h", "12h", "24h"} {
					if d, ok := oiData.Delta[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %+.4f%% (%s)\n", tf, d.OIDeltaPercent, formatFlowValue(d.OIDeltaValue)))
					}
				}
			}
		}
	}

	return sb.String()
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
		strValues[i] = formatAIFloat(v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}
