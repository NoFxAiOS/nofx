package kernel

import (
	"fmt"
	"nofx/market"
	"nofx/provider/nofxos"
	"nofx/provider/vergex"
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
	// System prompts are intentionally English-only. UI copy can be localized,
	// but the model contract should stay language-stable for an international
	// open-source project and for reproducible trading behavior.
	lang := LangEnglish
	zh := false
	singleSymbol, primarySymbol := e.singleSymbolInfo()

	// Configs created in the Chinese-UI era carry legacy stored prompt sections
	// and custom prompts written for a different contract; ignore them wholesale
	// and fall back to the canonical built-in English sections.
	legacyZhConfig := strings.EqualFold(strings.TrimSpace(e.config.Language), "zh")
	if legacyZhConfig {
		promptSections = store.PromptSectionsConfig{}
	}

	if e.usesVergexSignalPrompt() {
		return e.localizeDecisionOutput(e.buildVergexSystemPrompt(accountEquity, variant, lang, zh, singleSymbol, primarySymbol))
	}

	// 0. Data Dictionary & Schema (ensure AI understands all fields)
	sb.WriteString(GetSchemaPrompt(lang))
	sb.WriteString("\n\n")
	sb.WriteString("---\n\n")

	// 1. Role definition (editable; falls back to a generic intro in the
	//    correct language so we don't mix EN headings with ZH custom text).
	roleDefinition := englishOnlyPromptSection(promptSections.RoleDefinition)
	if roleDefinition != "" {
		sb.WriteString(roleDefinition)
		sb.WriteString("\n\n")
	} else if zh {
		sb.WriteString("# You are a professional Hyperliquid USDC multi-asset trading AI\n\n")
		sb.WriteString("Your task is to make trading decisions based on the provided market data.\n\n")
	} else {
		sb.WriteString("# You are a professional Hyperliquid USDC multi-asset trading AI\n\n")
		sb.WriteString("Your task is to make trading decisions based on the provided market data.\n\n")
	}

	// 2. Trading mode variant
	writeModeVariant(&sb, variant, zh)

	// 3. Hard constraints (risk control).
	//
	// `singleSymbol` is true for strategies that deliberately trade just one
	// instrument (the quick-create flow, single-asset templates). For those,
	// the "BTC/ETH vs Altcoin" two-tier categorization is irrelevant and
	// actively misleading — we surface a single position-value limit instead.
	btcEthPosValueRatio := riskControl.BTCETHMaxPositionValueRatio
	if btcEthPosValueRatio <= 0 {
		btcEthPosValueRatio = 5.0
	}
	altcoinPosValueRatio := riskControl.AltcoinMaxPositionValueRatio
	if altcoinPosValueRatio <= 0 {
		altcoinPosValueRatio = 1.0
	}

	writeHardConstraints(&sb, accountEquity, riskControl, btcEthPosValueRatio, altcoinPosValueRatio, singleSymbol, primarySymbol, zh)

	// 4. Trading frequency (editable)
	tradingFrequency := englishOnlyPromptSection(promptSections.TradingFrequency)
	if tradingFrequency != "" {
		sb.WriteString(tradingFrequency)
		sb.WriteString("\n\n")
	} else if zh {
		sb.WriteString("# ⏱️ Trading Frequency Awareness\n\n")
		sb.WriteString("- Excellent traders: 2-4 trades/day ≈ 0.1-0.2 trades/hour\n")
		sb.WriteString("- >2 trades/hour = overtrading\n")
		sb.WriteString("- Single position hold time ≥ 45-90 minutes\n")
		sb.WriteString("If you find yourself trading every cycle → standards too low; if closing positions < 45 minutes → too impulsive.\n\n")
	} else {
		sb.WriteString("# ⏱️ Trading Frequency Awareness\n\n")
		sb.WriteString("- Excellent traders: 2-4 trades/day ≈ 0.1-0.2 trades/hour\n")
		sb.WriteString("- >2 trades/hour = overtrading\n")
		sb.WriteString("- Single position hold time ≥ 45-90 minutes\n")
		sb.WriteString("If you find yourself trading every cycle → standards too low; if closing positions < 45 minutes → too impulsive.\n\n")
	}

	// 5. Entry standards (editable)
	entryStandards := englishOnlyPromptSection(promptSections.EntryStandards)
	if entryStandards != "" {
		sb.WriteString(entryStandards)
		if zh {
			sb.WriteString("\n\nYou have the following indicator data:\n")
		} else {
			sb.WriteString("\n\nYou have the following indicator data:\n")
		}
		e.writeAvailableIndicators(&sb, zh)
		if zh {
			sb.WriteString(fmt.Sprintf("\n**Confidence ≥ %d** required to open positions.\n\n", riskControl.MinConfidence))
		} else {
			sb.WriteString(fmt.Sprintf("\n**Confidence ≥ %d** required to open positions.\n\n", riskControl.MinConfidence))
		}
	} else if zh {
		sb.WriteString("# 🎯 Entry Standards (Strict)\n\n")
		sb.WriteString("Only open positions when multiple signals resonate. You have:\n")
		e.writeAvailableIndicators(&sb, zh)
		sb.WriteString(fmt.Sprintf("\nFeel free to use any effective analysis method, but **confidence ≥ %d** is required to open positions; avoid low-quality behaviors such as single-indicator entries, contradictory signals, sideways chop, or re-entering immediately after a close.\n\n", riskControl.MinConfidence))
	} else {
		sb.WriteString("# 🎯 Entry Standards (Strict)\n\n")
		sb.WriteString("Only open positions when multiple signals resonate. You have:\n")
		e.writeAvailableIndicators(&sb, zh)
		sb.WriteString(fmt.Sprintf("\nFeel free to use any effective analysis method, but **confidence ≥ %d** is required to open positions; avoid low-quality behaviors such as single-indicator entries, contradictory signals, sideways chop, or re-entering immediately after a close.\n\n", riskControl.MinConfidence))
	}

	// 6. Decision process (editable)
	decisionProcess := englishOnlyPromptSection(promptSections.DecisionProcess)
	if decisionProcess != "" {
		sb.WriteString(decisionProcess)
		sb.WriteString("\n\n")
	} else if zh {
		sb.WriteString("# 📋 Decision Process\n\n")
		sb.WriteString("1. Check positions → take profit / stop loss?\n")
		sb.WriteString("2. Scan candidates + multi-timeframe → are there strong signals?\n")
		sb.WriteString("3. Write chain of thought first, then output structured JSON\n\n")
	} else {
		sb.WriteString("# 📋 Decision Process\n\n")
		sb.WriteString("1. Check positions → take profit / stop loss?\n")
		sb.WriteString("2. Scan candidates + multi-timeframe → are there strong signals?\n")
		sb.WriteString("3. Write chain of thought first, then output structured JSON\n\n")
	}

	// 7. Output format — schema spec stays in English (this is a parser
	//    contract; reasoning copy is localized below).
	writeOutputFormat(&sb, accountEquity, btcEthPosValueRatio, riskControl, singleSymbol, primarySymbol, zh)

	// 8. Custom Prompt.
	//
	// For single-symbol Hyperliquid XYZ assets (US equities, commodities,
	// forex), we replace any stored CustomPrompt with a built-in English
	// stock-trader template. This serves two purposes:
	//   1. The auto-generated CustomPrompt from the quick-create flow used
	//      to be Chinese (matching UI language), which produced an
	//      incoherent mixed-language final prompt that confused the LLM.
	//   2. It guarantees a stock-specific, US-equity-tuned briefing
	//      regardless of when the strategy was first created.
	customPrompt := englishOnlyPromptSection(e.config.CustomPrompt)
	if legacyZhConfig {
		customPrompt = ""
	}
	if singleSymbol && market.IsXyzDexAsset(primarySymbol) {
		customPrompt = buildXYZStockCustomPrompt(primarySymbol)
	}

	if customPrompt != "" {
		if zh {
			sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		} else {
			sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		}
		sb.WriteString(customPrompt)
		sb.WriteString("\n\n")
		if zh {
			sb.WriteString("Note: the above personalized strategy supplements the basic rules and may not violate the core risk controls.\n")
		} else {
			sb.WriteString("Note: the above personalized strategy supplements the basic rules and may not violate the core risk controls.\n")
		}
	}

	return e.localizeDecisionOutput(sb.String())
}

// localizeDecisionOutput keeps the parser-facing prompt contract stable while
// allowing operators to read the model's explanatory text in their UI language.
func (e *StrategyEngine) localizeDecisionOutput(prompt string) string {
	if e == nil || e.config == nil || !strings.EqualFold(strings.TrimSpace(e.config.Language), "ja") {
		return prompt
	}

	return prompt + `

# Output Language

Write every user-facing natural-language value, including reasoning fields, in Japanese. Keep JSON keys, XML tags, action enum values, symbols, numbers, and units exactly as specified above.
`
}

func (e *StrategyEngine) usesVergexSignalPrompt() bool {
	if e == nil || e.config == nil {
		return false
	}
	coinSource := e.config.CoinSource
	sourceType := strings.ToLower(strings.TrimSpace(coinSource.SourceType))
	return sourceType == "vergex_signal" ||
		sourceType == "claw402" ||
		sourceType == "claw402_vergex" ||
		coinSource.VergexMarketType != "" ||
		coinSource.VergexChain != "" ||
		coinSource.VergexLimit > 0
}

func (e *StrategyEngine) buildVergexSystemPrompt(accountEquity float64, variant string, lang Language, zh bool, singleSymbol bool, primarySymbol string) string {
	var sb strings.Builder
	riskControl := e.config.RiskControl

	writeVergexSchemaPrompt(&sb, zh)
	sb.WriteString("\n\n---\n\n")

	if zh {
		sb.WriteString("# You are the NOFX Claw402 auto-trader\n\n")
		sb.WriteString("Trade only Hyperliquid instruments returned by this cycle's Claw402.ai/Vergex board. You may trade only the current candidate symbols and existing positions; never invent tickers or rotate outside the provided universe.\n\n")
		sb.WriteString("# Decision Data Priority\n\n")
		sb.WriteString("1. Claw402.ai Signal Ranking: candidate pool, rank, direction and category.\n")
		sb.WriteString("2. Claw402.ai Signal Lab: trend, momentum, event/model confirmation; this is the core pre-entry confirmation source.\n")
		sb.WriteString("3. Claw402.ai Cost/Liquidation Heatmap: crowded liquidation/cost zones, stop placement and target zones.\n")
		sb.WriteString("4. Raw OHLCV candles: entry timing, trend structure, volatility and risk/reward validation.\n\n")
		sb.WriteString("# Trading Rules\n\n")
		sb.WriteString("- Manage existing positions before opening new ones.\n")
		sb.WriteString("- Open only when Signal Lab, heatmap and raw candles broadly agree; wait when key data is missing or contradictory.\n")
		sb.WriteString("- Ranking alone is not an entry reason; it only defines the candidate pool.\n")
		sb.WriteString("- Every symbol in Candidate Coins is part of the allowed trading universe; missing detail can lower confidence or trigger waiting, but does not make the symbol non-tradable.\n")
		sb.WriteString("- If Signal Lab or heatmap is absent from that symbol's Vergex Claw402 Signals, state it in reasoning; if it is present, never claim the symbol lacks that data.\n")
		sb.WriteString("- Avoid churn: unless stopping out or taking a strong profit, hold new positions for at least 60 minutes; avoid flat/noise closes until roughly 90 minutes; after closing a symbol, wait 90 minutes before re-entry; open at most 1 new position per hour.\n")
		sb.WriteString("- Fees are the main edge killer: a round trip costs roughly 0.1%% of notional (about 1%% of margin at 10x). Only take setups whose expected move to target is at least 3x that cost; fewer, higher-conviction, longer-hold trades beat frequent scalps.\n")
		sb.WriteString("- Stops must sit beyond invalidation; targets should prefer heatmap resistance/liquidation zones or valid risk/reward levels.\n\n")
	} else {
		sb.WriteString("# You are the NOFX Claw402 auto-trader\n\n")
		sb.WriteString("Trade only Hyperliquid instruments returned by this cycle's Claw402.ai/Vergex board. You may trade only the current candidate symbols and existing positions; never invent tickers or rotate outside the provided universe.\n\n")
		sb.WriteString("# Decision Data Priority\n\n")
		sb.WriteString("1. Claw402.ai Signal Ranking: candidate pool, rank, direction and category.\n")
		sb.WriteString("2. Claw402.ai Signal Lab: trend, momentum, event/model confirmation; this is the core pre-entry confirmation source.\n")
		sb.WriteString("3. Claw402.ai Cost/Liquidation Heatmap: crowded liquidation/cost zones, stop placement and target zones.\n")
		sb.WriteString("4. Raw OHLCV candles: entry timing, trend structure, volatility and risk/reward validation.\n\n")
		sb.WriteString("# Trading Rules\n\n")
		sb.WriteString("- Manage existing positions before opening new ones.\n")
		sb.WriteString("- Open only when Signal Lab, heatmap and raw candles broadly agree; wait when key data is missing or contradictory.\n")
		sb.WriteString("- Ranking alone is not an entry reason; it only defines the candidate pool.\n")
		sb.WriteString("- Every symbol in Candidate Coins is part of the allowed trading universe; missing detail can lower confidence or trigger waiting, but does not make the symbol non-tradable.\n")
		sb.WriteString("- If Signal Lab or heatmap is absent from that symbol's Vergex Claw402 Signals, state it in reasoning; if it is present, never claim the symbol lacks that data.\n")
		sb.WriteString("- Avoid churn: unless stopping out or taking a strong profit, hold new positions for at least 60 minutes; avoid flat/noise closes until roughly 90 minutes; after closing a symbol, wait 90 minutes before re-entry; open at most 1 new position per hour.\n")
		sb.WriteString("- Fees are the main edge killer: a round trip costs roughly 0.1%% of notional (about 1%% of margin at 10x). Only take setups whose expected move to target is at least 3x that cost; fewer, higher-conviction, longer-hold trades beat frequent scalps.\n")
		sb.WriteString("- Stops must sit beyond invalidation; targets should prefer heatmap resistance/liquidation zones or valid risk/reward levels.\n\n")
	}

	writeModeVariant(&sb, variant, zh)

	altcoinPosValueRatio := riskControl.AltcoinMaxPositionValueRatio
	if altcoinPosValueRatio <= 0 {
		altcoinPosValueRatio = 1.0
	}
	writeVergexHardConstraints(&sb, accountEquity, riskControl, altcoinPosValueRatio, zh)
	writeVergexOutputFormat(&sb, accountEquity, riskControl, altcoinPosValueRatio, singleSymbol, primarySymbol, zh)

	customPrompt := vergexCustomPromptSection(e.config.CustomPrompt)
	if customPrompt != "" {
		sb.WriteString("# User Preference\n\n")
		sb.WriteString(customPrompt)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// vergexCustomPromptSection returns the user's custom prompt for the vergex
// path, dropping legacy directional overrides ("long only" era) that would
// contradict the data-driven direction rule baked into this prompt.
func vergexCustomPromptSection(section string) string {
	trimmed := englishOnlyPromptSection(section)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	legacyDirectives := []string{
		"long only",
		"long-only",
		"do not short",
		"no shorts",
		"must open a long",
		"short only",
		"short-only",
	}
	for _, directive := range legacyDirectives {
		if strings.Contains(lower, directive) {
			return ""
		}
	}
	return trimmed
}

func englishOnlyPromptSection(section string) string {
	trimmed := strings.TrimSpace(section)
	if trimmed == "" {
		return ""
	}
	if detectLanguage(trimmed) == LangChinese {
		return ""
	}
	return trimmed
}

func writeVergexSchemaPrompt(sb *strings.Builder, zh bool) {
	if zh {
		sb.WriteString("# Claw402.ai TradeFi Data Guide\n\n")
		sb.WriteString("- Equity: total account value including unrealized PnL, in USDT.\n")
		sb.WriteString("- Balance: available balance for new positions, in USDT.\n")
		sb.WriteString("- Margin: current margin usage; higher means more risk.\n")
		sb.WriteString("- Position: current holdings with side, entry, leverage, unrealized PnL and liquidation price.\n")
		sb.WriteString("- Claw402 Ranking: tradable candidate pool, rank, direction and category for this cycle.\n")
		sb.WriteString("- Signal Lab: per-symbol Claw402 deep signal used to confirm trend and quality.\n")
		sb.WriteString("- Cost/Liquidation Heatmap: cost and liquidation clusters used for stops, targets and crowding risk.\n")
		sb.WriteString("- Raw OHLCV Kline: raw candles used for trend structure, entry timing and risk/reward.\n")
	} else {
		sb.WriteString("# Claw402.ai TradeFi Data Guide\n\n")
		sb.WriteString("- Equity: total account value including unrealized PnL, in USDT.\n")
		sb.WriteString("- Balance: available balance for new positions, in USDT.\n")
		sb.WriteString("- Margin: current margin usage; higher means more risk.\n")
		sb.WriteString("- Position: current holdings with side, entry, leverage, unrealized PnL and liquidation price.\n")
		sb.WriteString("- Claw402 Ranking: tradable candidate pool, rank, direction and category for this cycle.\n")
		sb.WriteString("- Signal Lab: per-symbol Claw402 deep signal used to confirm trend and quality.\n")
		sb.WriteString("- Cost/Liquidation Heatmap: cost and liquidation clusters used for stops, targets and crowding risk.\n")
		sb.WriteString("- Raw OHLCV Kline: raw candles used for trend structure, entry timing and risk/reward.\n")
	}
}

func writeVergexHardConstraints(sb *strings.Builder, accountEquity float64, riskControl store.RiskControlConfig, tradeFiPositionValueRatio float64, zh bool) {
	maxPositionValue := accountEquity * tradeFiPositionValueRatio
	if zh {
		sb.WriteString("# Hard Risk Constraints\n\n")
		sb.WriteString("## Backend enforced\n")
		sb.WriteString(fmt.Sprintf("- Max positions: %d Claw402 candidate instruments at the same time\n", riskControl.MaxPositions))
		sb.WriteString(fmt.Sprintf("- Max notional per position: %.0f USDT (= equity %.0f × %.1fx)\n", maxPositionValue, accountEquity, tradeFiPositionValueRatio))
		sb.WriteString(fmt.Sprintf("- Max margin usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
		sb.WriteString(fmt.Sprintf("- Min order size: ≥%.0f USDT\n\n", riskControl.MinPositionSize))
		sb.WriteString("## AI guided\n")
		sb.WriteString(fmt.Sprintf("- Leverage: every open position must use exactly %dx\n", riskControl.AltcoinMaxLeverage))
		sb.WriteString(fmt.Sprintf("- Risk/reward: ≥1:%.1f\n", riskControl.MinRiskRewardRatio))
		sb.WriteString(fmt.Sprintf("- Min confidence to open: ≥%d\n\n", riskControl.MinConfidence))
		sb.WriteString("# Position Sizing\n\n")
		sb.WriteString("For every `open_long` or `open_short`, use the full max notional per position.\n")
		sb.WriteString("- Do not scale position_size_usd down by confidence.\n")
		sb.WriteString("- Do not open small probe positions.\n")
		sb.WriteString("- If the setup is not strong enough for full size, output `wait`.\n")
		sb.WriteString("- Do not use available_balance directly as position_size_usd.\n\n")
	} else {
		sb.WriteString("# Hard Risk Constraints\n\n")
		sb.WriteString("## Backend enforced\n")
		sb.WriteString(fmt.Sprintf("- Max positions: %d Claw402 candidate instruments at the same time\n", riskControl.MaxPositions))
		sb.WriteString(fmt.Sprintf("- Max notional per position: %.0f USDT (= equity %.0f × %.1fx)\n", maxPositionValue, accountEquity, tradeFiPositionValueRatio))
		sb.WriteString(fmt.Sprintf("- Max margin usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
		sb.WriteString(fmt.Sprintf("- Min order size: ≥%.0f USDT\n\n", riskControl.MinPositionSize))
		sb.WriteString("## AI guided\n")
		sb.WriteString(fmt.Sprintf("- Leverage: every open position must use exactly %dx\n", riskControl.AltcoinMaxLeverage))
		sb.WriteString(fmt.Sprintf("- Risk/reward: ≥1:%.1f\n", riskControl.MinRiskRewardRatio))
		sb.WriteString(fmt.Sprintf("- Min confidence to open: ≥%d\n\n", riskControl.MinConfidence))
		sb.WriteString("# Position Sizing\n\n")
		sb.WriteString("For every `open_long` or `open_short`, use the full max notional per position.\n")
		sb.WriteString("- Do not scale position_size_usd down by confidence.\n")
		sb.WriteString("- Do not open small probe positions.\n")
		sb.WriteString("- If the setup is not strong enough for full size, output `wait`.\n")
		sb.WriteString("- Do not use available_balance directly as position_size_usd.\n\n")
	}
}

func writeVergexOutputFormat(sb *strings.Builder, accountEquity float64, riskControl store.RiskControlConfig, tradeFiPositionValueRatio float64, singleSymbol bool, primarySymbol string, zh bool) {
	exampleSymbol := "xyz:NVDA"
	secondSymbol := "xyz:AAPL"
	if singleSymbol && strings.TrimSpace(primarySymbol) != "" {
		exampleSymbol = primarySymbol
		secondSymbol = primarySymbol
	}
	positionSize := accountEquity * tradeFiPositionValueRatio
	leverage := riskControl.AltcoinMaxLeverage
	if leverage <= 0 {
		leverage = 1
	}

	sb.WriteString("# Output Format (Strictly Follow)\n\n")
	if zh {
		sb.WriteString("Use XML tags <reasoning> and <decision> to separate concise analysis from the decision JSON.\n\n")
		sb.WriteString("Direction must be data-driven: use `open_long` for confirmed upside structures and `open_short` for confirmed downside structures; never default to long-only or short-only behavior.\n\n")
		if !singleSymbol {
			sb.WriteString("Evaluate both directions every cycle, but enter a side only when its own signals independently justify it. Never open a position just to balance the book — an unbalanced book beats a forced trade.\n\n")
		}
	} else {
		sb.WriteString("Use XML tags <reasoning> and <decision> to separate concise analysis from the decision JSON.\n\n")
		sb.WriteString("Direction must be data-driven: use `open_long` for confirmed upside structures and `open_short` for confirmed downside structures; never default to long-only or short-only behavior.\n\n")
		if !singleSymbol {
			sb.WriteString("Evaluate both directions every cycle, but enter a side only when its own signals independently justify it. Never open a position just to balance the book — an unbalanced book beats a forced trade.\n\n")
		}
	}
	sb.WriteString("<reasoning>\n")
	if zh {
		sb.WriteString("Briefly state whether Claw402 ranking, Signal Lab, heatmap and candles agree; if data is missing or conflicting, explain why you wait.\n")
	} else {
		sb.WriteString("Briefly state whether Claw402 ranking, Signal Lab, heatmap and candles agree; if data is missing or conflicting, explain why you wait.\n")
	}
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("```json\n[\n")
	if singleSymbol {
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"%s\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 0, \"take_profit\": 0, \"confidence\": 85, \"risk_usd\": 0}\n", exampleSymbol, leverage, positionSize))
	} else {
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"%s\", \"action\": \"open_long\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 0, \"take_profit\": 0, \"confidence\": 85, \"risk_usd\": 0},\n", exampleSymbol, leverage, positionSize))
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"%s\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 0, \"take_profit\": 0, \"confidence\": 85, \"risk_usd\": 0}\n", secondSymbol, leverage, positionSize))
	}
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")

	if zh {
		sb.WriteString("## Field Requirements\n\n")
		sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
		sb.WriteString(fmt.Sprintf("- `confidence`: 0-100; recommended ≥ %d to open\n", riskControl.MinConfidence))
		sb.WriteString("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n")
		sb.WriteString("- All numeric values must be calculated numbers, not formulas.\n")
		if singleSymbol {
			sb.WriteString(fmt.Sprintf("- This strategy trades only `%s`; JSON symbol must match it exactly.\n", exampleSymbol))
		} else {
			sb.WriteString("- JSON symbols must exactly match current candidates or existing positions; keep `xyz:` on XYZ instruments, and do not add `xyz:` or `USDT` to core crypto symbols.\n")
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("## Field Requirements\n\n")
		sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
		sb.WriteString(fmt.Sprintf("- `confidence`: 0-100; recommended ≥ %d to open\n", riskControl.MinConfidence))
		sb.WriteString("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n")
		sb.WriteString("- All numeric values must be calculated numbers, not formulas.\n")
		if singleSymbol {
			sb.WriteString(fmt.Sprintf("- This strategy trades only `%s`; JSON symbol must match it exactly.\n", exampleSymbol))
		} else {
			sb.WriteString("- JSON symbols must exactly match current candidates or existing positions; keep `xyz:` on XYZ instruments, and do not add `xyz:` or `USDT` to core crypto symbols.\n")
		}
		sb.WriteString("\n")
	}
}

// buildXYZStockCustomPrompt returns the canonical English directional stock
// briefing the agent uses for single-symbol Hyperliquid USDC perpetuals on
// the XYZ board. Symbol is inlined for LLM grounding so it never confuses the
// trading instrument.
func buildXYZStockCustomPrompt(symbol string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Trade ONLY the Hyperliquid USDC perpetual %s (US equity / xyz board).\n\n", symbol))
	sb.WriteString("Core stance: DIRECTIONAL, SIGNAL-DRIVEN. You may open long or short; never force a trade when Signal Lab, liquidation structure and candles disagree.\n\n")

	sb.WriteString("## Flat-Account Rule\n")
	sb.WriteString("If `Current Positions` is None / empty, evaluate both directions from scratch.\n")
	sb.WriteString("- Use `open_long` only when upside continuation or bullish reversal is confirmed.\n")
	sb.WriteString("- Use `open_short` only when downside continuation or bearish reversal is confirmed.\n")
	sb.WriteString("- Use `wait` when neither side meets the minimum confidence and risk/reward threshold.\n")
	sb.WriteString("- Do not raise confidence just to force an order; confidence must reflect the evidence.\n\n")

	sb.WriteString("## Long Entry Conditions\n")
	sb.WriteString("- Break of the prior session/intraday high on rising volume.\n")
	sb.WriteString("- Pullback to a clearly held intraday support (prior swing low, VWAP, EMA20/50) with a bullish reaction bar.\n")
	sb.WriteString("- Sector tape strength (broad US-equity bid, sympathy with peers in the same theme).\n")
	sb.WriteString("- Confirmed catalyst: earnings beat, guide up, sector rotation, macro tailwind.\n\n")

	sb.WriteString("## Short Entry Conditions\n")
	sb.WriteString("- Breakdown below intraday support or value area with expanding volume.\n")
	sb.WriteString("- Failed breakout, lower high, or bearish rejection at resistance.\n")
	sb.WriteString("- Signal Lab / liquidation structure shows downside fuel, trapped longs, or weak support below.\n")
	sb.WriteString("- Negative catalyst: earnings miss, guide down, sector weakness, macro headwind.\n\n")

	sb.WriteString("## Risk Guardrails (non-negotiable)\n")
	sb.WriteString("- Per-trade stop-loss: 1.5-3% from entry. ALWAYS set a numeric `stop_loss`.\n")
	sb.WriteString("- Take-profit: target at least R/R 2:1; set a numeric `take_profit`.\n")
	sb.WriteString("- Per-trade notional: <= 25% of account equity (probing 10-15%, full 20-25%).\n")
	sb.WriteString("- Leverage: 2-3x default, never above 5x. Never go all-in.\n")
	sb.WriteString("- Do not flip directly from long to short or short to long in the same cycle. Manage or close the open position first.\n\n")

	sb.WriteString("## Position Management\n")
	sb.WriteString("- Trail stop to breakeven once +1R, take partial profits at +2R if momentum stalls.\n")
	sb.WriteString("- Cut quickly if price breaks the stop or the catalyst thesis fails.\n")
	sb.WriteString("- Holding past 45 minutes is fine; flipping in/out every cycle is not.\n\n")

	sb.WriteString("## Discipline\n")
	sb.WriteString(fmt.Sprintf("- Single-symbol mandate: never rotate into another ticker. The decision JSON `symbol` MUST be exactly \"%s\".\n", symbol))
	sb.WriteString("- Before every decision: check current price vs prior pivot, volume vs 5m/1h average, and the broader US-equity tape.\n")
	sb.WriteString("- If positions are open, prioritize managing them over piling on new ones.")
	return sb.String()
}

// singleSymbolInfo returns (true, "ARM-USDC") for static-coin strategies that
// trade exactly one instrument. Multi-symbol strategies return (false, "").
// The flag is used to drop crypto-specific "BTC/ETH vs Altcoin" labeling and
// to put the actual trading symbol into the JSON example.
func (e *StrategyEngine) singleSymbolInfo() (bool, string) {
	coinSource := e.config.CoinSource
	if (coinSource.SourceType == "static" || coinSource.SourceType == "vergex_signal") && len(coinSource.StaticCoins) == 1 {
		return true, strings.ToUpper(strings.TrimSpace(coinSource.StaticCoins[0]))
	}
	return false, ""
}

func writeModeVariant(sb *strings.Builder, variant string, zh bool) {
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		if zh {
			sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts; may scale in when confidence ≥ 70\n- Allow larger positions, but must strictly set stop-loss and explain the risk-reward ratio\n\n")
		} else {
			sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts; may scale in when confidence ≥ 70\n- Allow larger positions, but must strictly set stop-loss and explain the risk-reward ratio\n\n")
		}
	case "conservative":
		if zh {
			sb.WriteString("## Mode: Conservative\n- Open positions only when multiple signals resonate\n- Prioritize capital preservation; pause for multiple periods after consecutive losses\n\n")
		} else {
			sb.WriteString("## Mode: Conservative\n- Open positions only when multiple signals resonate\n- Prioritize capital preservation; pause for multiple periods after consecutive losses\n\n")
		}
	case "scalping":
		if zh {
			sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick action\n- If price doesn't move as expected within two bars, immediately reduce position or stop-loss\n\n")
		} else {
			sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick action\n- If price doesn't move as expected within two bars, immediately reduce position or stop-loss\n\n")
		}
	}
}

func writeHardConstraints(sb *strings.Builder, accountEquity float64, riskControl store.RiskControlConfig, btcEthPosValueRatio, altcoinPosValueRatio float64, singleSymbol bool, primarySymbol string, zh bool) {
	if zh {
		sb.WriteString("# Hard Constraints (Risk Control)\n\n")
		sb.WriteString("## CODE ENFORCED (backend validation, cannot be bypassed):\n")
		sb.WriteString(fmt.Sprintf("- Max Positions: %d instruments simultaneously\n", riskControl.MaxPositions))
	} else {
		sb.WriteString("# Hard Constraints (Risk Control)\n\n")
		sb.WriteString("## CODE ENFORCED (backend validation, cannot be bypassed):\n")
		sb.WriteString(fmt.Sprintf("- Max Positions: %d instruments simultaneously\n", riskControl.MaxPositions))
	}

	if singleSymbol {
		// One symbol — pick the higher of the two configured ratios so the
		// limit isn't accidentally clamped to the altcoin cap for a stock.
		ratio := altcoinPosValueRatio
		if btcEthPosValueRatio > ratio {
			ratio = btcEthPosValueRatio
		}
		maxVal := accountEquity * ratio
		symLabel := primarySymbol
		if zh {
			sb.WriteString(fmt.Sprintf("- Position Value Limit (%s): max %.0f USDT (= equity %.0f × %.1fx)\n", symLabel, maxVal, accountEquity, ratio))
		} else {
			sb.WriteString(fmt.Sprintf("- Position Value Limit (%s): max %.0f USDT (= equity %.0f × %.1fx)\n", symLabel, maxVal, accountEquity, ratio))
		}
	} else {
		if zh {
			sb.WriteString(fmt.Sprintf("- Position Value Limit (Altcoin/Stock): max %.0f USDT (= equity %.0f × %.1fx)\n", accountEquity*altcoinPosValueRatio, accountEquity, altcoinPosValueRatio))
			sb.WriteString(fmt.Sprintf("- Position Value Limit (BTC/ETH): max %.0f USDT (= equity %.0f × %.1fx)\n", accountEquity*btcEthPosValueRatio, accountEquity, btcEthPosValueRatio))
		} else {
			sb.WriteString(fmt.Sprintf("- Position Value Limit (Altcoin/Stock): max %.0f USDT (= equity %.0f × %.1fx)\n", accountEquity*altcoinPosValueRatio, accountEquity, altcoinPosValueRatio))
			sb.WriteString(fmt.Sprintf("- Position Value Limit (BTC/ETH): max %.0f USDT (= equity %.0f × %.1fx)\n", accountEquity*btcEthPosValueRatio, accountEquity, btcEthPosValueRatio))
		}
	}

	if zh {
		sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
		sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT\n\n", riskControl.MinPositionSize))
		sb.WriteString("## AI GUIDED (recommended):\n")
	} else {
		sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
		sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT\n\n", riskControl.MinPositionSize))
		sb.WriteString("## AI GUIDED (recommended):\n")
	}

	if singleSymbol {
		lev := riskControl.AltcoinMaxLeverage
		if riskControl.BTCETHMaxLeverage > lev {
			lev = riskControl.BTCETHMaxLeverage
		}
		if zh {
			sb.WriteString(fmt.Sprintf("- Trading Leverage (%s): max %dx\n", primarySymbol, lev))
		} else {
			sb.WriteString(fmt.Sprintf("- Trading Leverage (%s): max %dx\n", primarySymbol, lev))
		}
	} else {
		if zh {
			sb.WriteString(fmt.Sprintf("- Trading Leverage: Altcoin/Stock max %dx | BTC/ETH max %dx\n", riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
		} else {
			sb.WriteString(fmt.Sprintf("- Trading Leverage: Altcoin/Stock max %dx | BTC/ETH max %dx\n", riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
		}
	}
	if zh {
		sb.WriteString(fmt.Sprintf("- Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss)\n", riskControl.MinRiskRewardRatio))
		sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d to open position\n\n", riskControl.MinConfidence))
	} else {
		sb.WriteString(fmt.Sprintf("- Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss)\n", riskControl.MinRiskRewardRatio))
		sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d to open position\n\n", riskControl.MinConfidence))
	}

	// Position sizing guidance
	exampleRatio := btcEthPosValueRatio
	if singleSymbol {
		exampleRatio = altcoinPosValueRatio
		if btcEthPosValueRatio > exampleRatio {
			exampleRatio = btcEthPosValueRatio
		}
	}
	if zh {
		sb.WriteString("## Position Sizing Guidance\n")
		sb.WriteString("Calculate `position_size_usd` from your confidence and the Position Value Limits above:\n")
		sb.WriteString("- High confidence (≥85): use 80-100%% of the position value limit\n")
		sb.WriteString("- Medium confidence (70-84): use 50-80%% of the position value limit\n")
		sb.WriteString("- Low confidence (60-69): use 30-50%% of the position value limit\n")
		sb.WriteString(fmt.Sprintf("- Example: equity %.0f × %.1fx = max %.0f USDT\n", accountEquity, exampleRatio, accountEquity*exampleRatio))
		sb.WriteString("- **DO NOT** just use available_balance as position_size_usd. Use the Position Value Limit!\n\n")
	} else {
		sb.WriteString("## Position Sizing Guidance\n")
		sb.WriteString("Calculate `position_size_usd` from your confidence and the Position Value Limits above:\n")
		sb.WriteString("- High confidence (≥85): use 80-100%% of the position value limit\n")
		sb.WriteString("- Medium confidence (70-84): use 50-80%% of the position value limit\n")
		sb.WriteString("- Low confidence (60-69): use 30-50%% of the position value limit\n")
		sb.WriteString(fmt.Sprintf("- Example: equity %.0f × %.1fx = max %.0f USDT\n", accountEquity, exampleRatio, accountEquity*exampleRatio))
		sb.WriteString("- **DO NOT** just use available_balance as position_size_usd. Use the Position Value Limit!\n\n")
	}
}

func writeOutputFormat(sb *strings.Builder, accountEquity, btcEthPosValueRatio float64, riskControl store.RiskControlConfig, singleSymbol bool, primarySymbol string, zh bool) {
	// Output format schema MUST stay English/structural; parser depends on it.
	sb.WriteString("# Output Format (Strictly Follow)\n\n")
	if zh {
		sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON, avoiding parsing errors**\n\n")
	} else {
		sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON, avoiding parsing errors**\n\n")
	}
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("<reasoning>\n")
	if zh {
		sb.WriteString("Your chain of thought analysis...\n- Briefly analyze your thinking process\n")
	} else {
		sb.WriteString("Your chain of thought analysis...\n- Briefly analyze your thinking process\n")
	}
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	if zh {
		sb.WriteString("Step 2: JSON decision array\n\n")
	} else {
		sb.WriteString("Step 2: JSON decision array\n\n")
	}
	sb.WriteString("```json\n[\n")

	// Build a JSON example using the actual trading symbol when the strategy
	// is single-symbol. Falls back to the legacy BTC/ETH two-line example
	// only for multi-symbol strategies that genuinely have BTC/ETH on tap.
	if singleSymbol {
		lev := riskControl.AltcoinMaxLeverage
		if riskControl.BTCETHMaxLeverage > lev {
			lev = riskControl.BTCETHMaxLeverage
		}
		ratio := btcEthPosValueRatio // already chosen as the larger above when single-symbol
		size := accountEquity * ratio
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"%s\", \"action\": \"open_long\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 0, \"take_profit\": 0, \"confidence\": 85, \"risk_usd\": 0},\n", primarySymbol, lev, size))
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"%s\", \"action\": \"wait\"}\n", primarySymbol))
	} else {
		examplePositionSize := accountEquity * btcEthPosValueRatio
		sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300},\n",
			riskControl.BTCETHMaxLeverage, examplePositionSize))
		sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\"}\n")
	}
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")

	if zh {
		sb.WriteString("## Field Description\n\n")
		sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
		sb.WriteString(fmt.Sprintf("- `confidence`: 0-100 (opening recommended ≥ %d)\n", riskControl.MinConfidence))
		sb.WriteString("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n")
		sb.WriteString("- **IMPORTANT**: all numeric values must be calculated numbers, NOT formulas/expressions (e.g. use `27.76`, not `3000 * 0.01`)\n")
		if singleSymbol {
			sb.WriteString(fmt.Sprintf("- **This strategy trades only %s.** The JSON `symbol` MUST match `%s` exactly — do not write `%s` variants that drop the suffix or add USDT.\n", primarySymbol, primarySymbol, primarySymbol))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("## Field Description\n\n")
		sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
		sb.WriteString(fmt.Sprintf("- `confidence`: 0-100 (opening recommended ≥ %d)\n", riskControl.MinConfidence))
		sb.WriteString("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n")
		sb.WriteString("- **IMPORTANT**: all numeric values must be calculated numbers, NOT formulas/expressions (e.g. use `27.76`, not `3000 * 0.01`)\n")
		if singleSymbol {
			sb.WriteString(fmt.Sprintf("- **This strategy trades only %s.** The JSON `symbol` MUST match `%s` exactly — do not add USDT/USDC suffix variants.\n", primarySymbol, primarySymbol))
		}
		sb.WriteString("\n")
	}
}

func (e *StrategyEngine) writeAvailableIndicators(sb *strings.Builder, zh bool) {
	indicators := e.config.Indicators
	kline := indicators.Klines

	label := func(en, zhStr string) string {
		if zh {
			return zhStr
		}
		return en
	}

	if zh {
		sb.WriteString(fmt.Sprintf("- %s price series", kline.PrimaryTimeframe))
		if kline.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf(" + %s K-line series\n", kline.LongerTimeframe))
		} else {
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("- %s price series", kline.PrimaryTimeframe))
		if kline.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf(" + %s K-line series\n", kline.LongerTimeframe))
		} else {
			sb.WriteString("\n")
		}
	}

	if indicators.EnableEMA {
		sb.WriteString("- " + label("EMA indicators", "EMA indicators"))
		if len(indicators.EMAPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s: %v)", label("periods", "periods"), indicators.EMAPeriods))
		}
		sb.WriteString("\n")
	}
	if indicators.EnableMACD {
		sb.WriteString("- " + label("MACD indicators", "MACD indicators") + "\n")
	}
	if indicators.EnableRSI {
		sb.WriteString("- " + label("RSI indicators", "RSI indicators"))
		if len(indicators.RSIPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s: %v)", label("periods", "periods"), indicators.RSIPeriods))
		}
		sb.WriteString("\n")
	}
	if indicators.EnableATR {
		sb.WriteString("- " + label("ATR indicators", "ATR indicators"))
		if len(indicators.ATRPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s: %v)", label("periods", "periods"), indicators.ATRPeriods))
		}
		sb.WriteString("\n")
	}
	if indicators.EnableBOLL {
		sb.WriteString("- " + label("Bollinger Bands (BOLL) - Upper/Middle/Lower bands", "Bollinger Bands (BOLL) - Upper/Middle/Lower bands"))
		if len(indicators.BOLLPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s: %v)", label("periods", "periods"), indicators.BOLLPeriods))
		}
		sb.WriteString("\n")
	}
	if indicators.EnableVolume {
		sb.WriteString("- " + label("Volume data", "Volume data") + "\n")
	}
	if indicators.EnableOI {
		sb.WriteString("- " + label("Open Interest (OI) data", "Open Interest (OI) data") + "\n")
	}
	if indicators.EnableFundingRate {
		sb.WriteString("- " + label("Funding rate", "Funding rate") + "\n")
	}
	if len(e.config.CoinSource.StaticCoins) > 0 || e.config.CoinSource.UseAI500 || e.config.CoinSource.UseOITop {
		sb.WriteString("- " + label("AI500 / OI_Top filter tags (if available)", "AI500 / OI_Top filter tags (if available)") + "\n")
	}
	if indicators.EnableQuantData {
		sb.WriteString("- " + label("Quantitative data (institutional/retail fund flow, position changes, multi-period price changes)", "Quantitative data (institutional/retail fund flow, position changes, multi-period price changes)") + "\n")
	}
}

// ============================================================================
// Prompt Building - User Prompt
// ============================================================================

// BuildUserPrompt builds User Prompt based on strategy configuration
func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// System status
	sb.WriteString(fmt.Sprintf("Time: %s | Period: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC market
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// Account information
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | PnL %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// Recently completed orders (placed before positions to ensure visibility)
	if len(ctx.RecentOrders) > 0 {
		sb.WriteString("## Recent Completed Trades\n")
		for i, order := range ctx.RecentOrders {
			resultStr := "Profit"
			if order.RealizedPnL < 0 {
				resultStr = "Loss"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | %s: %+.2f USDT (%+.2f%%) | %s→%s (%s)\n",
				i+1, order.Symbol, order.Side,
				order.EntryPrice, order.ExitPrice,
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

		if lang == LangChinese {
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

	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d coins)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		// Skip if this coin is already a position (data already shown in positions section)
		normalizedCoinSymbol := market.Normalize(coin.Symbol)
		if positionSymbols[normalizedCoinSymbol] {
			continue
		}

		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := e.formatCoinSourceTag(coin.Sources)
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[coin.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		if ctx.VergexDataMap != nil {
			if vergexData, hasVergex := ctx.VergexDataMap[coin.Symbol]; hasVergex {
				sb.WriteString(e.formatVergexData(vergexData))
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

	sb.WriteString("---\n\n")
	sb.WriteString("Now please analyze briefly and output the decision JSON.\n")

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

	sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Qty %.4f | Position Value %.2f USDT | PnL%+.2f%% | PnL Amount%+.2f USDT | Peak PnL%.2f%% | Leverage %dx | Margin %.0f | Liq Price %.4f%s\n\n",
		index, pos.Symbol, strings.ToUpper(pos.Side),
		pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
		pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[pos.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		if ctx.VergexDataMap != nil {
			if vergexData, hasVergex := ctx.VergexDataMap[pos.Symbol]; hasVergex {
				sb.WriteString(e.formatVergexData(vergexData))
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
		case "vergex_signal":
			return " (Vergex Signal)"
		}
		if strings.HasPrefix(sources[0], "hyper_rank") {
			return " (Hyperliquid Dynamic Rank)"
		}
	}
	return ""
}

func (e *StrategyEngine) formatVergexData(data *vergex.MarketAnalysis) string {
	if data == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\nVergex Claw402 Signals:\n")
	sb.WriteString(vergex.FormatAnalysisForAI(data))
	return sb.String()
}

// ============================================================================
// Market Data Formatting
// ============================================================================

func (e *StrategyEngine) formatMarketData(data *market.Data) string {
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

		if indicators.EnableOI && data.OpenInterest != nil {
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
		strValues[i] = fmt.Sprintf("%.4f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}
