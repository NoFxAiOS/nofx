package kernel

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"nofx/logger"
	"nofx/mcp"
	"nofx/market"
	"nofx/provider/nofxos"
	"nofx/store"
)

// MacroSymbolEntry holds per-symbol metadata from the macro pass (bias, risk, conviction).
type MacroSymbolEntry struct {
	Symbol     string  `json:"symbol"`     // e.g. BTCUSDT
	Bias       string  `json:"bias"`       // bullish, bearish, neutral
	Risk       string  `json:"risk"`       // low, medium, high
	Conviction float64 `json:"conviction"` // 0-1, strength vs overall market
}

// macroSymbolsForDeepDive supports both new format ([]MacroSymbolEntry) and legacy ([]string) in JSON.
type macroSymbolsForDeepDive []MacroSymbolEntry

func (m *macroSymbolsForDeepDive) UnmarshalJSON(data []byte) error {
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	result := make([]MacroSymbolEntry, 0, len(raw))
	for _, item := range raw {
		switch v := item.(type) {
		case string:
			result = append(result, MacroSymbolEntry{
				Symbol:     market.Normalize(v),
				Bias:       "neutral",
				Risk:       "medium",
				Conviction: 0.5,
			})
		case map[string]interface{}:
			sym, _ := v["symbol"].(string)
			bias, _ := v["bias"].(string)
			risk, _ := v["risk"].(string)
			conv := 0.5
			if c, ok := v["conviction"].(float64); ok && c >= 0 && c <= 1 {
				conv = c
			}
			if sym == "" {
				continue
			}
			result = append(result, MacroSymbolEntry{
				Symbol:     market.Normalize(sym),
				Bias:       coalesceBias(bias),
				Risk:       coalesceRisk(risk),
				Conviction: conv,
			})
		}
	}
	*m = result
	return nil
}

func coalesceBias(s string) string {
	switch s {
	case "bullish", "bearish", "neutral":
		return s
	default:
		return "neutral"
	}
}

func coalesceRisk(s string) string {
	switch s {
	case "low", "medium", "high":
		return s
	default:
		return "medium"
	}
}

// SymbolStrings returns the symbol strings from a macro symbols slice (for use with APIs that expect []string).
func SymbolStrings(entries macroSymbolsForDeepDive) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Symbol
	}
	return out
}

// NewMacroSymbolsFromStrings creates macro symbol entries with default bias/risk/conviction.
func NewMacroSymbolsFromStrings(syms []string) macroSymbolsForDeepDive {
	out := make(macroSymbolsForDeepDive, len(syms))
	for i, s := range syms {
		out[i] = MacroSymbolEntry{Symbol: market.Normalize(s), Bias: "neutral", Risk: "medium", Conviction: 0.5}
	}
	return out
}

// MacroOutput is the structured output from the macro AI pass.
type MacroOutput struct {
	Trend              string                 `json:"trend"`                // market overview: bullish, bearish, neutral (fallback)
	RiskLevel          string                 `json:"risk_level"`           // market-level risk (fallback)
	FocusReason        string                 `json:"focus_reason"`         // 1-2 sentences
	SymbolsForDeepDive macroSymbolsForDeepDive `json:"symbols_for_deep_dive"` // per-symbol: symbol, bias, risk, conviction
	CheckPositions     bool                   `json:"check_positions"`
}

func formatMacroOISummary(data *nofxos.OIRankingData, topN int) string {
	if data == nil || (len(data.TopPositions) == 0 && len(data.LowPositions) == 0) {
		return "OI Flow: unavailable\n"
	}
	if topN <= 0 {
		topN = 5
	}
	var sb strings.Builder
	sb.WriteString("### OI Flow (summary)\n")
	if len(data.TopPositions) > 0 {
		sb.WriteString("- Top OI increase: ")
		for i, pos := range data.TopPositions {
			if i >= topN {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s %+.2f%%", pos.Symbol, pos.OIDeltaPercent))
		}
		sb.WriteString("\n")
	}
	if len(data.LowPositions) > 0 {
		sb.WriteString("- Top OI decrease: ")
		for i, pos := range data.LowPositions {
			if i >= topN {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s %+.2f%%", pos.Symbol, pos.OIDeltaPercent))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatMacroNetFlowSummary(data *nofxos.NetFlowRankingData, topN int) string {
	if data == nil {
		return "NetFlow: unavailable\n"
	}
	if topN <= 0 {
		topN = 5
	}
	var sb strings.Builder
	sb.WriteString("### NetFlow (summary)\n")
	if len(data.InstitutionFutureTop) > 0 {
		sb.WriteString("- Institution inflow: ")
		for i, pos := range data.InstitutionFutureTop {
			if i >= topN {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(pos.Symbol)
		}
		sb.WriteString("\n")
	}
	if len(data.InstitutionFutureLow) > 0 {
		sb.WriteString("- Institution outflow: ")
		for i, pos := range data.InstitutionFutureLow {
			if i >= topN {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(pos.Symbol)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatMacroPriceRankingSummary outputs compact lines: per duration, top 5 gainers and losers.
func formatMacroPriceRankingSummary(data *nofxos.PriceRankingData, topN int) string {
	if data == nil || len(data.Durations) == 0 {
		return "### Price Ranking\n(unavailable)\n"
	}
	if topN <= 0 {
		topN = 5
	}
	order := []string{"1h", "4h", "24h"}
	var sb strings.Builder
	sb.WriteString("### Price Ranking (1h/4h/24h)\n")
	for _, dur := range order {
		d, ok := data.Durations[dur]
		if !ok || d == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s Gainers: ", dur))
		for i, item := range d.Top {
			if i >= topN {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s %+.2f%%", item.Symbol, item.PriceDelta*100))
		}
		sb.WriteString(" | Losers: ")
		for i, item := range d.Low {
			if i >= topN {
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s %.2f%%", item.Symbol, item.PriceDelta*100))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatPositionForMacroBrief returns one line per position with full metadata, no klines. Includes TP/SL hints and holding duration.
func formatPositionForMacroBrief(pos PositionInfo, currentPrice float64) string {
	value := pos.Quantity * pos.MarkPrice
	if value < 0 {
		value = -value
	}
	line := fmt.Sprintf("- %s %s | Entry %.4f Current %.4f | Qty %.4f | Value %.2f USDT | PnL %+.2f%% | PnL USDT %+.2f | Peak %.2f%% | Leverage %dx | Margin %.0f | Liq %.4f",
		pos.Symbol, strings.ToUpper(pos.Side), pos.EntryPrice, pos.MarkPrice, pos.Quantity, value,
		pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct, pos.Leverage, pos.MarginUsed, pos.LiquidationPrice)
	if pos.UpdateTime > 0 {
		line += fmt.Sprintf(" | Entry time: %s", time.UnixMilli(pos.UpdateTime).UTC().Format("2006-01-02 15:04:05 UTC"))
	}
	if currentPrice > 0 {
		line += fmt.Sprintf(" | Price %.4f", currentPrice)
	}
	// Trailing TP hint: PnL dropped >=30% from peak and peak > 2%
	if pos.PeakPnLPct >= 2 && (pos.PeakPnLPct-pos.UnrealizedPnLPct) >= 30 {
		line += " [Hint: consider trailing take-profit]"
	}
	if pos.UnrealizedPnLPct < -4 {
		line += " [Hint: consider stop-loss]"
	}
	return line + "\n"
}

// BuildMacroBrief builds the compact market brief for the macro AI pass (no raw kline tables).
func BuildMacroBrief(ctx *Context, engine *StrategyEngine) (string, error) {
	config := engine.GetConfig()
	indicators := config.Indicators
	oiLimit := indicators.OIRankingLimit
	if oiLimit <= 0 {
		oiLimit = 10
	}
	priceTopN := 5
	if indicators.PriceRankingLimit < priceTopN {
		priceTopN = indicators.PriceRankingLimit
	}

	var sb strings.Builder
	sb.WriteString("## Market Brief\n")
	sb.WriteString(fmt.Sprintf("Time: %s | Cycle #%d | Runtime %d min\n\n", ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// Wallet
	sb.WriteString("### Wallet\n")
	pctAvail := 0.0
	if ctx.Account.TotalEquity > 0 {
		pctAvail = (ctx.Account.AvailableBalance / ctx.Account.TotalEquity) * 100
	}
	sb.WriteString(fmt.Sprintf("Total Equity: %.2f USDT | Available: %.2f (%.1f%%) | Total PnL: %+.2f%% | Margin: %.1f%% | Positions: %d\n",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, pctAvail, ctx.Account.TotalPnLPct, ctx.Account.MarginUsedPct, ctx.Account.PositionCount))
	if ctx.Account.MarginUsedPct > 70 {
		sb.WriteString("(Risk: margin > 70%%)\n")
	} else if ctx.Account.MarginUsedPct > 50 {
		sb.WriteString("(Risk: margin > 50%%)\n")
	}
	sb.WriteString("\n")

	// OI
	if ctx.OIRankingData != nil {
		sb.WriteString(formatMacroOISummary(ctx.OIRankingData, priceTopN))
	} else {
		sb.WriteString("### OI Flow\n(unavailable)\n")
	}

	// NetFlow
	if ctx.NetFlowRankingData != nil {
		sb.WriteString(formatMacroNetFlowSummary(ctx.NetFlowRankingData, priceTopN))
	} else {
		sb.WriteString("### NetFlow\n(unavailable)\n")
	}

	// Price ranking
	if ctx.PriceRankingData != nil {
		sb.WriteString(formatMacroPriceRankingSummary(ctx.PriceRankingData, priceTopN))
	} else {
		sb.WriteString("### Price Ranking\n(unavailable)\n")
	}

	// Optional: BTC funding (one line)
	if btcData, ok := ctx.MarketDataMap["BTCUSDT"]; ok && btcData != nil {
		sb.WriteString(fmt.Sprintf("### Funding\nBTC funding: %.4f%%\n\n", btcData.FundingRate*100))
	}

	// Open positions (full metadata, no klines)
	sb.WriteString("### Open Positions\n")
	if len(ctx.Positions) == 0 {
		sb.WriteString("None\n\n")
	} else {
		for _, pos := range ctx.Positions {
			curPrice := 0.0
			if md, ok := ctx.MarketDataMap[pos.Symbol]; ok && md != nil {
				curPrice = md.CurrentPrice
			} else {
				curPrice = pos.MarkPrice
			}
			sb.WriteString(formatPositionForMacroBrief(pos, curPrice))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

var reMacroJSON = regexp.MustCompile(`(?s)\{[\s\S]*"symbols_for_deep_dive"[\s\S]*\}`)

func getMacroSystemPrompt() string {
	return `You are a macro analyst for crypto markets. Output only valid JSON, no other text.

Output schema:
{
  "trend": "bullish" | "bearish" | "neutral",
  "risk_level": "high" | "medium" | "low",
  "focus_reason": "1-2 sentences summarizing market context and where to focus",
  "symbols_for_deep_dive": [
    {"symbol": "SYM1", "bias": "bullish"|"bearish"|"neutral", "risk": "low"|"medium"|"high", "conviction": 0.0-1.0},
    ...
  ],
  "check_positions": true | false
}

Rules:
- You MUST include every currently open position symbol in symbols_for_deep_dive (for TP/SL/hold). No open position may be omitted.
- Each symbol can have its own bias (bullish/bearish/neutral), risk, and conviction (0-1: strength vs overall market).
- The market can be mixed: some symbols bullish, some bearish. Both long and short setups can appear.
- conviction: 0.5 = average vs market; higher = stronger opportunity; lower = weaker.`
}

// BuildMacroSystemPrompt returns the system prompt for the macro AI pass (for preview or trace).
func BuildMacroSystemPrompt() string {
	return getMacroSystemPrompt()
}

// BuildMacroUserPrompt returns the user prompt for the macro AI pass (brief + instruction + custom).
func BuildMacroUserPrompt(brief string, config *store.StrategyConfig) string {
	limit := clampMacroDeepDiveLimit(config.MacroDeepDiveLimit)
	if limit <= 0 {
		limit = 5
	}
	custom := effectiveMacroCustomPrompt(config)
	return getMacroUserPrompt(brief, limit, custom)
}

// effectiveMacroCustomPrompt returns the macro custom text from sections (if any) or the legacy single field.
func effectiveMacroCustomPrompt(config *store.StrategyConfig) string {
	if config == nil {
		return ""
	}
	if config.MacroPromptSections != nil {
		var parts []string
		if s := strings.TrimSpace(config.MacroPromptSections.RoleContext); s != "" {
			parts = append(parts, s)
		}
		if s := strings.TrimSpace(config.MacroPromptSections.OutputGuidance); s != "" {
			parts = append(parts, s)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n")
		}
	}
	return config.MacroCustomPrompt
}

func getMacroUserPrompt(brief string, opportunityLimit int, customPrompt string) string {
	instruction := fmt.Sprintf("Based on this market brief, output a JSON object with: (1) trend: bullish/bearish/neutral (market overview), (2) risk_level: high/medium/low, (3) focus_reason: 1-2 sentences, (4) symbols_for_deep_dive: array of objects {symbol, bias, risk, conviction} — must include every open position symbol, plus at most %d additional symbols for new opportunities, in priority order. Each symbol gets its own bias (bullish/bearish/neutral), risk (low/medium/high), and conviction (0-1). (5) check_positions: true if open positions exist, else false.", opportunityLimit)
	out := brief + "\n\n" + instruction
	if customPrompt != "" {
		out += "\n\n" + customPrompt
	}
	return out
}

// ParseMacroResponse parses the macro AI response into MacroOutput (for trace or tests).
func ParseMacroResponse(response string) (*MacroOutput, error) {
	return parseMacroResponse(response)
}

func parseMacroResponse(response string) (*MacroOutput, error) {
	s := strings.TrimSpace(response)
	// Strip markdown code fence if present
	if idx := strings.Index(s, "```"); idx >= 0 {
		rest := s[idx+3:]
		if strings.HasPrefix(rest, "json") {
			rest = rest[4:]
		}
		rest = strings.TrimSpace(rest)
		if end := strings.Index(rest, "```"); end >= 0 {
			rest = rest[:end]
		}
		s = strings.TrimSpace(rest)
	}
	if match := reMacroJSON.FindString(s); match != "" {
		s = match
	}
	var out MacroOutput
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func clampMacroDeepDiveLimit(limit int) int {
	if limit < 3 {
		return 3
	}
	if limit > 10 {
		return 10
	}
	return limit
}

// ValidateAndMergeMacroOutput merges position symbols into SymbolsForDeepDive, enforces cap, coerces enums.
func ValidateAndMergeMacroOutput(out *MacroOutput, ctx *Context, config *store.StrategyConfig) *MacroOutput {
	if out == nil {
		out = &MacroOutput{Trend: "neutral", RiskLevel: "medium", FocusReason: "", CheckPositions: len(ctx.Positions) > 0}
	}
	// Coerce enums
	switch out.Trend {
	case "bullish", "bearish", "neutral":
	default:
		out.Trend = "neutral"
	}
	switch out.RiskLevel {
	case "high", "medium", "low":
	default:
		out.RiskLevel = "medium"
	}
	// Build excluded set (strategy excludes these from trading)
	excluded := make(map[string]bool)
	if config != nil && config.CoinSource.ExcludedCoins != nil {
		for _, c := range config.CoinSource.ExcludedCoins {
			excluded[market.Normalize(c)] = true
		}
	}

	// Ensure all position symbols are in the list (keep even if excluded - we need to manage hold/close), then add macro-selected symbols up to cap
	seen := make(map[string]bool)
	var merged macroSymbolsForDeepDive
	for _, pos := range ctx.Positions {
		n := market.Normalize(pos.Symbol)
		if !seen[n] {
			merged = append(merged, MacroSymbolEntry{Symbol: n, Bias: "neutral", Risk: out.RiskLevel, Conviction: 0.5})
			seen[n] = true
		}
	}
	for _, entry := range out.SymbolsForDeepDive {
		n := market.Normalize(entry.Symbol)
		if seen[n] {
			continue
		}
		if excluded[n] {
			logger.Infof("🚫 [macro-micro] Excluded symbol %s skipped from deep-dive", n)
			continue
		}
		merged = append(merged, MacroSymbolEntry{
			Symbol:     n,
			Bias:       coalesceBias(entry.Bias),
			Risk:       coalesceRisk(entry.Risk),
			Conviction: clampConviction(entry.Conviction),
		})
		seen[n] = true
	}
	limit := clampMacroDeepDiveLimit(config.MacroDeepDiveLimit)
	if limit <= 0 {
		limit = 5
	}
	maxTotal := len(ctx.Positions) + limit
	if len(merged) > maxTotal {
		merged = merged[:maxTotal]
	}
	out.SymbolsForDeepDive = merged
	out.CheckPositions = out.CheckPositions || len(ctx.Positions) > 0
	return out
}

func clampConviction(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}

// GetMacroDecision calls the AI with the macro brief and returns structured output.
func GetMacroDecision(ctx *Context, macroBrief string, engine *StrategyEngine, mcpClient mcp.AIClient) (*MacroOutput, error) {
	config := engine.GetConfig()
	limit := clampMacroDeepDiveLimit(config.MacroDeepDiveLimit)
	if limit <= 0 {
		limit = 5
	}
	macroCustomPrompt := effectiveMacroCustomPrompt(config)
	sysPrompt := getMacroSystemPrompt()
	userPrompt := getMacroUserPrompt(macroBrief, limit, macroCustomPrompt)

	response, err := mcpClient.CallWithMessages(sysPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("macro AI call failed: %w", err)
	}

	out, err := ParseMacroResponse(response)
	if err != nil {
		// Fallback: first 3 from AI500 + all position symbols
		logger.Warnf("[macro-micro] failed to parse macro response, using fallback: %v", err)
		out = &MacroOutput{
			Trend:          "neutral",
			RiskLevel:      "medium",
			FocusReason:    "",
			CheckPositions: len(ctx.Positions) > 0,
		}
		coins, _ := engine.nofxosClient.GetAI500List()
		for i, c := range coins {
			if i >= 3 {
				break
			}
			sym := market.Normalize(c.Pair)
			if sym != "BTCUSDT" && sym != "ETHUSDT" {
				out.SymbolsForDeepDive = append(out.SymbolsForDeepDive, MacroSymbolEntry{Symbol: sym, Bias: "neutral", Risk: "medium", Conviction: 0.5})
			}
		}
		for _, pos := range ctx.Positions {
			out.SymbolsForDeepDive = append(out.SymbolsForDeepDive, MacroSymbolEntry{Symbol: market.Normalize(pos.Symbol), Bias: "neutral", Risk: "medium", Conviction: 0.5})
		}
	}

	return ValidateAndMergeMacroOutput(out, ctx, config), nil
}
