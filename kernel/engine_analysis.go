package kernel

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// Pre-compiled regular expressions (performance optimization)
// ============================================================================

var (
	// Safe regex: precisely match ```json code blocks
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// XML tag extraction (supports any characters in reasoning chain)
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// ============================================================================
// Entry Functions - Main API
// ============================================================================

// GetFullDecision gets AI's complete trading decision (batch analysis of all coins and positions)
// Uses default strategy configuration - for production use GetFullDecisionWithStrategy with explicit config
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	defaultConfig := store.GetDefaultStrategyConfig("en")
	engine := NewStrategyEngine(&defaultConfig)
	return GetFullDecisionWithStrategy(ctx, mcpClient, engine, "")
}

// GetFullDecisionWithStrategy uses StrategyEngine to get AI decision (unified prompt generation)
func GetFullDecisionWithStrategy(ctx *Context, mcpClient mcp.AIClient, engine *StrategyEngine, variant string) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if engine == nil {
		defaultConfig := store.GetDefaultStrategyConfig("en")
		engine = NewStrategyEngine(&defaultConfig)
	}

	// 1. Fetch market data using strategy config
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataWithStrategy(ctx, engine); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	}

	// Ensure OITopDataMap is initialized
	if ctx.OITopDataMap == nil {
		ctx.OITopDataMap = make(map[string]*OITopData)
		oiPositions, err := engine.nofxosClient.GetOITopPositions()
		if err == nil {
			for _, pos := range oiPositions {
				ctx.OITopDataMap[pos.Symbol] = &OITopData{
					Rank:              pos.Rank,
					OIDeltaPercent:    pos.OIDeltaPercent,
					OIDeltaValue:      pos.OIDeltaValue,
					PriceDeltaPercent: pos.PriceDeltaPercent,
				}
			}
		}
	}

	// 2. Build System Prompt using strategy engine
	riskConfig := engine.GetRiskControlConfig()
	systemPrompt := engine.BuildSystemPrompt(ctx.Account.TotalEquity, variant)

	// 3. Build User Prompt using strategy engine
	userPrompt := engine.BuildUserPrompt(ctx)

	// 4. Call AI API
	aiCallStart := time.Now()
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		return nil, fmt.Errorf("AI API call failed: %w", err)
	}

	// 5. Parse AI response
	decision, err := parseFullDecisionResponse(
		aiResponse,
		ctx.Account.TotalEquity,
		riskConfig.BTCETHMaxLeverage,
		riskConfig.AltcoinMaxLeverage,
		riskConfig.BTCETHMaxPositionValueRatio,
		riskConfig.AltcoinMaxPositionValueRatio,
	)

	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt
		decision.UserPrompt = userPrompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
		decision.RawResponse = aiResponse
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// 6. Validate protection routes against strategy config (drawdown/ladder/break-even AI mode).
	// Route validation failures are per-decision quality failures: reject only the bad
	// open proposal and keep other valid candidates from the same AI response.
	config := engine.GetConfig()
	if decision != nil && len(decision.Decisions) > 0 {
		filtered, rejected := FilterInvalidAIDecisionsWithStrategyAndCoT(decision.Decisions, config, decision.CoTTrace)
		if len(rejected) > 0 {
			for _, rej := range rejected {
				if rej.Index >= 0 {
					logger.Warnf("🚫 Protection route validation rejected decision #%d %s %s: %v", rej.Index+1, rej.Decision.Symbol, rej.Decision.Action, rej.Err)
				} else {
					logger.Warnf("🚫 Protection route validation rejected AI response package: %v", rej.Err)
				}
			}
			decision.Decisions = filtered
			if len(decision.Decisions) == 0 {
				logger.Warnf("🚫 Protection route validation rejected all open decisions; continuing with empty no-trade decision set")
			}
		}
	}

	return decision, nil
}

// ============================================================================
// Market Data Fetching
// ============================================================================

// fetchMarketDataWithStrategy fetches market data using strategy config (multiple timeframes)
func fetchMarketDataWithStrategy(ctx *Context, engine *StrategyEngine) error {
	config := engine.GetConfig()
	ctx.MarketDataMap = make(map[string]*market.Data)

	timeframes := config.Indicators.Klines.SelectedTimeframes
	primaryTimeframe := config.Indicators.Klines.PrimaryTimeframe
	klineCount := config.Indicators.Klines.PrimaryCount

	// Compatible with old configuration
	if len(timeframes) == 0 {
		if primaryTimeframe != "" {
			timeframes = append(timeframes, primaryTimeframe)
		} else {
			timeframes = append(timeframes, "3m")
		}
		if config.Indicators.Klines.LongerTimeframe != "" {
			timeframes = append(timeframes, config.Indicators.Klines.LongerTimeframe)
		}
	}
	if primaryTimeframe == "" {
		primaryTimeframe = timeframes[0]
	}
	if klineCount <= 0 {
		klineCount = 30
	}

	logger.Infof("📊 Strategy timeframes: %v, Primary: %s, Kline count: %d", timeframes, primaryTimeframe, klineCount)

	// Resolve exchange source for K-line data
	exchangeSrc := config.CoinSource.ExchangeSource
	if exchangeSrc == "" {
		exchangeSrc = "binance"
	}

	// 1. First fetch data for position coins (must fetch)
	for _, pos := range ctx.Positions {
		data, err := market.GetWithTimeframesExchange(pos.Symbol, timeframes, primaryTimeframe, klineCount, exchangeSrc)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for position %s: %v", pos.Symbol, err)
			continue
		}
		ctx.MarketDataMap[pos.Symbol] = data
	}

	// 2. Fetch data for all candidate coins
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	const minOIThresholdMillions = 15.0 // 15M USD minimum open interest value

	for _, coin := range ctx.CandidateCoins {
		if _, exists := ctx.MarketDataMap[coin.Symbol]; exists {
			continue
		}

		data, err := market.GetWithTimeframesExchange(coin.Symbol, timeframes, primaryTimeframe, klineCount, exchangeSrc)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for %s: %v", coin.Symbol, err)
			continue
		}

		// Liquidity filter (skip for xyz dex assets - they don't have OI data from Binance)
		isExistingPosition := positionSymbols[coin.Symbol]
		isXyzAsset := market.IsXyzDexAsset(coin.Symbol)
		if !isExistingPosition && !isXyzAsset {
			if data.OpenInterest == nil || data.CurrentPrice <= 0 || data.OpenInterest.Latest <= 0 {
				logger.Infof("⚠️  %s OI data unavailable or invalid, skipping OI hard filter and keeping candidate coin", coin.Symbol)
			} else {
				oiValue := data.OpenInterest.Latest * data.CurrentPrice
				oiValueInMillions := oiValue / 1_000_000
				if oiValueInMillions < minOIThresholdMillions {
					logger.Infof("⚠️  %s OI value too low (%.2fM USD < %.1fM), skipping coin",
						coin.Symbol, oiValueInMillions, minOIThresholdMillions)
					continue
				}
			}
		}

		ctx.MarketDataMap[coin.Symbol] = data
	}

	logger.Infof("📊 Successfully fetched multi-timeframe market data for %d coins", len(ctx.MarketDataMap))
	return nil
}

// ============================================================================
// AI Response Parsing
// ============================================================================

func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) (*FullDecision, error) {
	cotTrace := extractCoTTrace(aiResponse)

	decisions, fallbackReason, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	normalizeAndRepairOpenDecisions(decisions)

	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", err)
	}

	return &FullDecision{
		CoTTrace:            cotTrace,
		Decisions:           decisions,
		ParseFallback:       fallbackReason != "",
		ParseFallbackReason: fallbackReason,
	}, nil
}

func extractCoTTrace(response string) string {
	if match := reReasoningTag.FindStringSubmatch(response); match != nil && len(match) > 1 {
		logger.Infof("✓ Extracted reasoning chain using <reasoning> tag")
		return strings.TrimSpace(match[1])
	}

	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		logger.Infof("✓ Extracted content before <decision> tag as reasoning chain")
		return strings.TrimSpace(response[:decisionIdx])
	}

	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		logger.Infof("⚠️  Extracted reasoning chain using old format ([ character separator)")
		return strings.TrimSpace(response[:jsonStart])
	}

	return strings.TrimSpace(response)
}

func extractDecisions(response string) ([]Decision, string, error) {
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)
	s = fixMissingQuotes(s)

	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); match != nil && len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		logger.Infof("✓ Extracted JSON using <decision> tag")
	} else {
		jsonPart = s
		logger.Infof("⚠️  <decision> tag not found, searching JSON in full text")
	}

	jsonPart = fixMissingQuotes(jsonPart)

	if m := reJSONFence.FindStringSubmatch(jsonPart); m != nil && len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent)
		jsonContent = fixMissingQuotes(jsonContent)
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, "", fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, "", fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
		}
		return decisions, "", nil
	}

	jsonContent := strings.TrimSpace(extractTopLevelJSONArray(jsonPart))
	if jsonContent == "" {
		logger.Infof("⚠️  [SafeFallback] AI didn't output JSON decision, entering safe wait mode")

		cotSummary := jsonPart
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("Model didn't output structured JSON decision, entering safe wait; summary: %s", cotSummary),
		}

		return []Decision{fallbackDecision}, "missing_json_decision_array", nil
	}

	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent)

	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, "", fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
	}

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, "", fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
	}

	return decisions, "", nil
}

func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")

	jsonStr = normalizePunctuationOutsideStrings(jsonStr)

	return jsonStr
}

func normalizePunctuationOutsideStrings(jsonStr string) string {
	var b strings.Builder
	b.Grow(len(jsonStr))
	inString := false
	escaped := false
	for _, r := range jsonStr {
		if inString {
			b.WriteRune(r)
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		if r == '"' {
			inString = true
			b.WriteRune(r)
			continue
		}
		switch r {
		case '，', '、':
			b.WriteRune(',')
		case '：':
			b.WriteRune(':')
		case '［', '【', '〔':
			b.WriteRune('[')
		case '］', '】', '〕':
			b.WriteRune(']')
		case '｛':
			b.WriteRune('{')
		case '｝':
			b.WriteRune('}')
		case '　':
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	if trimmed == "[]" {
		return nil
	}

	if !reArrayHead.MatchString(trimmed) {
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be precise single values")
	}

	inString := false
	escaped := false
	for i := 0; i < len(jsonStr)-4; i++ {
		ch := jsonStr[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch >= '0' && ch <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON numbers cannot contain thousand separator comma, found: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// ParseAIDecisions parses structured AI decision JSON from raw model output.
func ParseAIDecisions(response string) ([]Decision, error) {
	decisions, _, err := extractDecisions(response)
	return decisions, err
}

// ValidateAIDecisions validates parsed AI decisions against supported action/schema rules.
func ValidateAIDecisions(decisions []Decision) error {
	return ValidateDecisionFormat(decisions)
}

func ValidateAIDecisionsWithStrategyAndCoT(decisions []Decision, config *store.StrategyConfig, cotTrace string) error {
	if err := ValidateDecisionFormatWithCoT(decisions, cotTrace); err != nil {
		return err
	}
	return validateAIDecisionRoutesWithStrategy(decisions, config)
}

func ValidateAIDecisionsWithStrategy(decisions []Decision, config *store.StrategyConfig) error {
	if err := ValidateDecisionFormat(decisions); err != nil {
		return err
	}
	return validateAIDecisionRoutesWithStrategy(decisions, config)
}

type DecisionRouteRejection struct {
	Index    int
	Decision Decision
	Err      error
}

func FilterInvalidAIDecisionsWithStrategyAndCoT(decisions []Decision, config *store.StrategyConfig, cotTrace string) ([]Decision, []DecisionRouteRejection) {
	if err := ValidateDecisionFormatWithCoT(decisions, cotTrace); err != nil {
		return decisions, []DecisionRouteRejection{{Index: -1, Err: err}}
	}
	return filterInvalidAIDecisionRoutesWithStrategy(decisions, config)
}

func FilterInvalidAIDecisionsWithStrategy(decisions []Decision, config *store.StrategyConfig) ([]Decision, []DecisionRouteRejection) {
	if err := ValidateDecisionFormat(decisions); err != nil {
		return decisions, []DecisionRouteRejection{{Index: -1, Err: err}}
	}
	return filterInvalidAIDecisionRoutesWithStrategy(decisions, config)
}

func filterInvalidAIDecisionRoutesWithStrategy(decisions []Decision, config *store.StrategyConfig) ([]Decision, []DecisionRouteRejection) {
	if config == nil {
		return decisions, nil
	}
	filtered := make([]Decision, 0, len(decisions))
	rejected := make([]DecisionRouteRejection, 0)
	for i, d := range decisions {
		if d.Action != "open_long" && d.Action != "open_short" && d.Action != "OPEN_NEW" {
			filtered = append(filtered, d)
			continue
		}
		if err := validateAIDecisionRoutesWithStrategy([]Decision{d}, config); err != nil {
			rejected = append(rejected, DecisionRouteRejection{Index: i, Decision: d, Err: err})
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered, rejected
}

func validateAIDecisionRoutesWithStrategy(decisions []Decision, config *store.StrategyConfig) error {
	if config == nil {
		return nil
	}
	minRR := config.RiskControl.MinRiskRewardRatio
	if minRR <= 0 {
		minRR = 1.5
	}
	fullAI := config.Protection.FullTPSL.Enabled && config.Protection.FullTPSL.Mode == store.ProtectionModeAI
	ladderAI := config.Protection.LadderTPSL.Enabled && config.Protection.LadderTPSL.Mode == store.ProtectionModeAI
	drawdownAI := config.Protection.DrawdownTakeProfit.Enabled && config.Protection.DrawdownTakeProfit.Mode == store.ProtectionModeAI
	for i, d := range decisions {
		isOpen := d.Action == "open_long" || d.Action == "open_short" || d.Action == "OPEN_NEW"
		if !isOpen {
			continue
		}
		if err := ValidateEntryProtectionRationale(d, minRR, config); err != nil {
			return fmt.Errorf("decision #%d: %w", i+1, err)
		}
		if (fullAI && drawdownAI) || (ladderAI && drawdownAI) || (fullAI && ladderAI) {
			logger.Warnf("strategy has multiple AI protection routes enabled, validating by ownership: full=%t ladder=%t drawdown=%t", fullAI, ladderAI, drawdownAI)
		}
		if config.Protection.BreakEvenStop.Enabled && isOpen {
			if d.ProtectionPlan == nil || d.ProtectionPlan.BreakEvenTrigger == "" || d.ProtectionPlan.BreakEvenValue <= 0 {
				return fmt.Errorf("decision #%d: current strategy route requires break-even protection output for open actions", i+1)
			}
		}
		planMode := ""
		if d.ProtectionPlan != nil {
			planMode = strings.ToLower(strings.TrimSpace(d.ProtectionPlan.Mode))
		}
		if drawdownAI && ladderAI {
			if d.ProtectionPlan == nil || planMode != "combined" {
				return fmt.Errorf("decision #%d: current strategy route requires combined protection_plan with ladder_rules and drawdown_rules for open actions", i+1)
			}
			if n := len(d.ProtectionPlan.LadderRules); n < 2 || n > 3 {
				return fmt.Errorf("decision #%d: combined protection_plan must contain 2~3 ladder_rules under current strategy route", i+1)
			}
			if len(d.ProtectionPlan.DrawdownRules) == 0 {
				return fmt.Errorf("decision #%d: combined protection_plan must contain drawdown_rules under current strategy route", i+1)
			}
			for j := range d.ProtectionPlan.LadderRules {
				rule := &d.ProtectionPlan.LadderRules[j]
				if strings.TrimSpace(rule.StructuralAnchor) == "" {
					logger.Warnf("decision #%d ladder_rule[%d]: missing structural_anchor (structural justification expected)", i+1, j)
				}
				ensureLadderVolatilityBuffer(d, rule, i, j)
			}
			for j, rule := range d.ProtectionPlan.DrawdownRules {
				if strings.TrimSpace(rule.ReasonAnchor) == "" {
					logger.Warnf("decision #%d drawdown_rule[%d]: missing reason_anchor (structural justification expected)", i+1, j)
				}
			}
			continue
		}
		if ladderAI && !drawdownAI && !fullAI {
			if d.ProtectionPlan == nil || planMode != "ladder" {
				return fmt.Errorf("decision #%d: current strategy route requires ladder protection_plan for open actions", i+1)
			}
			if n := len(d.ProtectionPlan.LadderRules); n < 2 || n > 3 {
				return fmt.Errorf("decision #%d: ladder protection_plan must contain 2~3 ladder_rules under current strategy route", i+1)
			}
			for j := range d.ProtectionPlan.LadderRules {
				ensureLadderVolatilityBuffer(d, &d.ProtectionPlan.LadderRules[j], i, j)
			}
		}
		if fullAI && !drawdownAI && !ladderAI {
			if d.ProtectionPlan == nil || planMode != "full" {
				return fmt.Errorf("decision #%d: current strategy route requires full protection_plan for open actions", i+1)
			}
		}
		if drawdownAI {
			if d.ProtectionPlan == nil || planMode != "drawdown" {
				return fmt.Errorf("decision #%d: current strategy route requires drawdown protection_plan for open actions", i+1)
			}
			if len(d.ProtectionPlan.DrawdownRules) == 0 {
				return fmt.Errorf("decision #%d: drawdown protection_plan must contain drawdown_rules under current strategy route", i+1)
			}
			for j, rule := range d.ProtectionPlan.DrawdownRules {
				if strings.TrimSpace(rule.ReasonAnchor) == "" {
					logger.Warnf("decision #%d drawdown_rule[%d]: missing reason_anchor (structural justification expected)", i+1, j)
				}
			}
		}
	}
	return nil
}

func ValidateEntryProtectionRationale(d Decision, minRR float64, config *store.StrategyConfig) error {
	if d.Action != "open_long" && d.Action != "open_short" {
		return nil
	}
	if minRR <= 0 {
		minRR = 1.5
	}
	if d.EntryProtection == nil {
		return fmt.Errorf("open action requires entry_protection_rationale")
	}
	// Backfill key_levels from structural_key_levels/anchors when AI omits support/resistance buckets.
	backfillEntryProtectionKeyLevels(d.EntryProtection)
	if config != nil {
		trimEntryProtectionToConfigLimits(d.EntryProtection, config.EntryStructure)
	}

	if config != nil {
		entryStructure := config.EntryStructure
		if entryStructure.Enabled {
			if entryStructure.RequirePrimaryTimeframe && strings.TrimSpace(d.EntryProtection.TimeframeContext.Primary) == "" {
				return fmt.Errorf("entry_protection_rationale.timeframe_context.primary is required")
			}
			if entryStructure.RequireAdjacentTimeframes && len(d.EntryProtection.TimeframeContext.Lower) == 0 && len(d.EntryProtection.TimeframeContext.Higher) == 0 {
				return fmt.Errorf("entry_protection_rationale.timeframe_context requires at least one adjacent timeframe")
			}
			if entryStructure.RequireSupportResistance && (len(d.EntryProtection.KeyLevels.Support) == 0 || len(d.EntryProtection.KeyLevels.Resistance) == 0) {
				return fmt.Errorf("entry_protection_rationale.key_levels support/resistance are required")
			}
			if entryStructure.MaxSupportLevels > 0 && len(d.EntryProtection.KeyLevels.Support) > entryStructure.MaxSupportLevels {
				return fmt.Errorf("entry_protection_rationale.key_levels support exceeds max %d", entryStructure.MaxSupportLevels)
			}
			if entryStructure.MaxResistanceLevels > 0 && len(d.EntryProtection.KeyLevels.Resistance) > entryStructure.MaxResistanceLevels {
				return fmt.Errorf("entry_protection_rationale.key_levels resistance exceeds max %d", entryStructure.MaxResistanceLevels)
			}
			if entryStructure.RequireStructuralAnchors && len(d.EntryProtection.Anchors) == 0 {
				return fmt.Errorf("entry_protection_rationale.anchors is required")
			}
			if entryStructure.MaxAnchorCount > 0 && len(d.EntryProtection.Anchors) > entryStructure.MaxAnchorCount {
				return fmt.Errorf("entry_protection_rationale.anchors exceeds max %d", entryStructure.MaxAnchorCount)
			}
			if len(d.EntryProtection.Anchors) > 0 {
				allowedTF := map[string]struct{}{}
				if primary := strings.TrimSpace(d.EntryProtection.TimeframeContext.Primary); primary != "" {
					allowedTF[primary] = struct{}{}
				}
				for _, tf := range d.EntryProtection.TimeframeContext.Lower {
					if tf = strings.TrimSpace(tf); tf != "" {
						allowedTF[tf] = struct{}{}
					}
				}
				for _, tf := range d.EntryProtection.TimeframeContext.Higher {
					if tf = strings.TrimSpace(tf); tf != "" {
						allowedTF[tf] = struct{}{}
					}
				}
				for i, anchor := range d.EntryProtection.Anchors {
					if strings.TrimSpace(anchor.Type) == "" || strings.TrimSpace(anchor.Timeframe) == "" || anchor.Price <= 0 || strings.TrimSpace(anchor.Reason) == "" {
						return fmt.Errorf("entry_protection_rationale.anchors[%d] requires type, timeframe, price, and reason", i)
					}
					if len(allowedTF) > 0 {
						if _, ok := allowedTF[anchor.Timeframe]; !ok {
							return fmt.Errorf("entry_protection_rationale.anchors[%d] timeframe %s not in timeframe_context", i, anchor.Timeframe)
						}
					}
				}
			}
			if err := validateHigherTimeframeStructureCoverage(d, entryStructure.RequireAdjacentTimeframes, entryStructure.RequireFibonacci && entryStructure.RequireAdjacentTimeframes); err != nil {
				return err
			}
			if entryStructure.RequireFibonacci {
				fib := d.EntryProtection.KeyLevels.Fibonacci
				if fib == nil || fib.SwingHigh <= 0 || fib.SwingLow <= 0 || len(fib.Levels) == 0 {
					return fmt.Errorf("entry_protection_rationale.key_levels.fibonacci with swing anchors is required")
				}
			}
		}
	}
	rr := d.EntryProtection.RiskReward
	if rr.Entry <= 0 || rr.Invalidation <= 0 || rr.FirstTarget <= 0 || rr.GrossEstimatedRR <= 0 {
		return fmt.Errorf("entry_protection_rationale.risk_reward requires positive entry, invalidation, first_target, and gross_estimated_rr")
	}
	if d.Action == "open_long" && !(rr.Invalidation < rr.Entry && rr.FirstTarget > rr.Entry) {
		return fmt.Errorf("entry_protection_rationale.risk_reward direction mismatch for open_long")
	}
	if d.Action == "open_short" && !(rr.Invalidation > rr.Entry && rr.FirstTarget < rr.Entry) {
		return fmt.Errorf("entry_protection_rationale.risk_reward direction mismatch for open_short")
	}
	if err := validateStructuralPriceAlignment(d.Action, d.EntryProtection, config); err != nil {
		return err
	}

	computedRR := rr.GrossEstimatedRR
	riskDistance := absFloat(rr.Entry - rr.Invalidation)
	rewardDistance := absFloat(rr.FirstTarget - rr.Entry)
	if riskDistance > 0 && rewardDistance > 0 {
		computedRR = rewardDistance / riskDistance
	}
	effectiveRR := rr.GrossEstimatedRR
	if rr.NetEstimatedRR > 0 {
		effectiveRR = rr.NetEstimatedRR
	}
	if hasRiskRewardExecutionConstraints(d.EntryProtection.ExecutionConstraints) {
		if recomputedGross, recomputedNet, ok := recomputeRiskRewardWithExecutionConstraints(d.Action, rr, d.EntryProtection.ExecutionConstraints); ok {
			computedRR = recomputedGross
			if rr.NetEstimatedRR > 0 {
				effectiveRR = recomputedNet
			}
			if rr.NetEstimatedRR > 0 && absFloat(rr.NetEstimatedRR-recomputedNet) > 0.05 {
				return fmt.Errorf("entry_protection_rationale.risk_reward net_estimated_rr %.2f inconsistent with execution constraints %.2f", rr.NetEstimatedRR, recomputedNet)
			}
		}
	}

	if effectiveRR < minRR {
		return fmt.Errorf("entry_protection_rationale.risk_reward %.2f below min %.2f", effectiveRR, minRR)
	}
	if rr.MinRequiredRR > 0 && rr.MinRequiredRR+0.02 < minRR {
		return fmt.Errorf("entry_protection_rationale.risk_reward min_required_rr %.2f below strategy min %.2f", rr.MinRequiredRR, minRR)
	}
	if rr.Passed && effectiveRR+0.02 < minRR {
		return fmt.Errorf("entry_protection_rationale.risk_reward passed=true inconsistent with effective rr %.2f below min %.2f", effectiveRR, minRR)
	}
	if !rr.Passed && effectiveRR >= minRR+0.02 {
		return fmt.Errorf("entry_protection_rationale.risk_reward passed=false inconsistent with effective rr %.2f meeting min %.2f", effectiveRR, minRR)
	}
	if absFloat(rr.GrossEstimatedRR-computedRR) > 0.05 {
		return fmt.Errorf("entry_protection_rationale.risk_reward gross_estimated_rr %.2f inconsistent with entry/invalidation/first_target %.2f", rr.GrossEstimatedRR, computedRR)
	}
	if err := validateProtectionPlanAlignmentSkeleton(d, rr, config); err != nil {
		return err
	}
	return nil
}

func validateHigherTimeframeStructureCoverage(d Decision, requireHigherContext bool, requireFibonacci bool) error {
	if d.EntryProtection == nil || len(d.EntryProtection.TimeframeContext.Higher) == 0 || !requireHigherContext {
		return nil
	}
	higherTF := map[string]struct{}{}
	for _, tf := range d.EntryProtection.TimeframeContext.Higher {
		if tf = strings.TrimSpace(tf); tf != "" {
			higherTF[tf] = struct{}{}
		}
	}
	if len(higherTF) == 0 {
		return nil
	}
	anchors := append([]AIEntryProtectionAnchor{}, d.EntryProtection.HigherAnchors...)
	for _, anchor := range d.EntryProtection.Anchors {
		if _, ok := higherTF[strings.TrimSpace(anchor.Timeframe)]; ok {
			anchors = append(anchors, anchor)
		}
	}
	for _, structure := range d.EntryProtection.TimeframeStructures {
		if _, ok := higherTF[strings.TrimSpace(structure.Timeframe)]; !ok {
			continue
		}
		anchors = append(anchors, structure.Anchors...)
	}
	if len(anchors) == 0 {
		return fmt.Errorf("entry_protection_rationale requires at least one higher timeframe anchor when timeframe_context.higher is provided")
	}
	for i, anchor := range anchors {
		if _, ok := higherTF[strings.TrimSpace(anchor.Timeframe)]; !ok {
			return fmt.Errorf("entry_protection_rationale.higher_timeframe_anchors[%d] timeframe %s not in timeframe_context.higher", i, anchor.Timeframe)
		}
		if strings.TrimSpace(anchor.Type) == "" || anchor.Price <= 0 || strings.TrimSpace(anchor.Reason) == "" {
			return fmt.Errorf("entry_protection_rationale.higher_timeframe_anchors[%d] requires type, timeframe, price, and reason", i)
		}
	}
	if requireFibonacci {
		for _, structure := range d.EntryProtection.TimeframeStructures {
			if _, ok := higherTF[strings.TrimSpace(structure.Timeframe)]; ok && structure.Fibonacci != nil && structure.Fibonacci.SwingHigh > 0 && structure.Fibonacci.SwingLow > 0 && len(structure.Fibonacci.Levels) > 0 {
				return nil
			}
		}
		return fmt.Errorf("entry_protection_rationale requires higher timeframe fibonacci context when fibonacci is required")
	}
	return nil
}

func validateStructuralPriceAlignment(action string, rationale *AIEntryProtectionRationale, config *store.StrategyConfig) error {
	if rationale == nil {
		return nil
	}
	if config == nil || !config.EntryStructure.Enabled {
		return nil
	}
	rr := rationale.RiskReward
	riskDistance := absFloat(rr.Entry - rr.Invalidation)
	rewardDistance := absFloat(rr.FirstTarget - rr.Entry)
	if riskDistance <= 0 || rewardDistance <= 0 {
		return nil
	}

	if err := validateStructuralAnchorCoverage(action, rationale); err != nil {
		return err
	}
	if err := validateStructuralLevelProximity(action, rationale, riskDistance, rewardDistance); err != nil {
		return err
	}
	return nil
}

func validateStructuralAnchorCoverage(action string, rationale *AIEntryProtectionRationale) error {
	anchors := append([]AIEntryProtectionAnchor{}, rationale.Anchors...)
	if len(anchors) == 0 && len(rationale.HigherAnchors) == 0 && len(rationale.TimeframeStructures) == 0 {
		return nil
	}
	var invalidationAnchor, targetAnchor bool
	if len(rationale.HigherAnchors) > 0 {
		anchors = append(anchors, rationale.HigherAnchors...)
	}
	for _, structure := range rationale.TimeframeStructures {
		anchors = append(anchors, structure.Anchors...)
	}
	for _, anchor := range anchors {
		t := strings.ToLower(strings.TrimSpace(anchor.Type))
		switch action {
		case "open_long":
			if t == "support" || t == "swing_low" || t == "fib_support" {
				invalidationAnchor = true
			}
			if t == "resistance" || t == "swing_high" || t == "fib_resistance" || t == "fibonacci" {
				targetAnchor = true
			}
		case "open_short":
			if t == "resistance" || t == "swing_high" || t == "fib_resistance" {
				invalidationAnchor = true
			}
			if t == "support" || t == "swing_low" || t == "fib_support" || t == "fibonacci" {
				targetAnchor = true
			}
		}
	}
	if !invalidationAnchor {
		return fmt.Errorf("entry_protection_rationale.anchors must include a structural invalidation anchor for %s", action)
	}
	if !targetAnchor {
		return fmt.Errorf("entry_protection_rationale.anchors must include a structural first_target anchor for %s", action)
	}
	return nil
}

func validateStructuralLevelProximity(action string, rationale *AIEntryProtectionRationale, riskDistance, rewardDistance float64) error {
	entry := rationale.RiskReward.Entry
	invalidation := rationale.RiskReward.Invalidation
	firstTarget := rationale.RiskReward.FirstTarget
	supportTol := structuralTolerance(riskDistance, rationale.VolatilityAdjustment.ATR14Pct, entry)
	resistanceTol := structuralTolerance(rewardDistance, rationale.VolatilityAdjustment.ATR14Pct, entry)
	fibTol := structuralTolerance(maxFloat(riskDistance, rewardDistance), rationale.VolatilityAdjustment.ATR14Pct, entry)

	supports := filterPositiveLevels(rationale.KeyLevels.Support)
	resistances := filterPositiveLevels(rationale.KeyLevels.Resistance)
	fibLevels := fibonacciLevels(rationale.KeyLevels.Fibonacci)
	invalidationRefs, targetRefs := structuralReferenceLevels(rationale)

	switch action {
	case "open_long":
		longInvalidationLevels := supports
		if len(invalidationRefs) > 0 {
			longInvalidationLevels = invalidationRefs
		}
		if len(longInvalidationLevels) > 0 {
			nearestSupport, supportGap := nearestLevel(invalidation, longInvalidationLevels)
			if supportGap > supportTol {
				return fmt.Errorf("entry_protection_rationale.risk_reward invalidation %.4f too far from structural support %.4f", invalidation, nearestSupport)
			}
			if invalidation > nearestSupport+supportTol {
				return fmt.Errorf("entry_protection_rationale.risk_reward invalidation %.4f must sit near/below support %.4f", invalidation, nearestSupport)
			}
		}
		longTargetLevels := resistances
		if len(targetRefs) > 0 {
			longTargetLevels = targetRefs
		}
		if len(longTargetLevels) > 0 {
			nearestResistance, resistanceGap := nearestLevel(firstTarget, longTargetLevels)
			if resistanceGap > resistanceTol {
				return fmt.Errorf("entry_protection_rationale.risk_reward first_target %.4f too far from structural resistance %.4f", firstTarget, nearestResistance)
			}
		}
		if len(fibLevels) > 0 {
			_, fibGap := nearestLevel(firstTarget, fibLevels)
			if len(resistances) == 0 && fibGap > fibTol {
				return fmt.Errorf("entry_protection_rationale.risk_reward first_target %.4f too far from fibonacci structure", firstTarget)
			}
		}
	case "open_short":
		shortInvalidationLevels := resistances
		if len(invalidationRefs) > 0 {
			shortInvalidationLevels = invalidationRefs
		}
		if len(shortInvalidationLevels) > 0 {
			nearestResistance, resistanceGap := nearestLevel(invalidation, shortInvalidationLevels)
			if resistanceGap > resistanceTol {
				return fmt.Errorf("entry_protection_rationale.risk_reward invalidation %.4f too far from structural resistance %.4f", invalidation, nearestResistance)
			}
			if invalidation < nearestResistance-supportTol {
				return fmt.Errorf("entry_protection_rationale.risk_reward invalidation %.4f must sit near/above resistance %.4f", invalidation, nearestResistance)
			}
		}
		shortTargetLevels := supports
		if len(targetRefs) > 0 {
			shortTargetLevels = targetRefs
		}
		if len(shortTargetLevels) > 0 {
			nearestSupport, supportGap := nearestLevel(firstTarget, shortTargetLevels)
			if supportGap > supportTol {
				return fmt.Errorf("entry_protection_rationale.risk_reward first_target %.4f too far from structural support %.4f", firstTarget, nearestSupport)
			}
		}
		if len(fibLevels) > 0 {
			_, fibGap := nearestLevel(firstTarget, fibLevels)
			if len(supports) == 0 && fibGap > fibTol {
				return fmt.Errorf("entry_protection_rationale.risk_reward first_target %.4f too far from fibonacci structure", firstTarget)
			}
		}
	}
	return nil
}

func structuralReferenceLevels(rationale *AIEntryProtectionRationale) (invalidationRefs []float64, targetRefs []float64) {
	if rationale == nil {
		return nil, nil
	}
	for _, lvl := range rationale.StructuralKeyLevels {
		if lvl.Price <= 0 {
			continue
		}
		usedFor := strings.ToLower(strings.TrimSpace(lvl.UsedFor))
		switch usedFor {
		case "invalidation", "stop_loss", "entry_support", "entry_resistance", "entry":
			invalidationRefs = append(invalidationRefs, lvl.Price)
		case "take_profit", "first_target", "tp1", "tp2", "tp1_drawdown_trigger", "tp2_drawdown_trigger", "profit_protection_stage_1", "profit_protection_stage_2", "profit_protection_stage_3", "tp2_reference", "break_even", "outer_drawdown_runner", "runner_target":
			targetRefs = append(targetRefs, lvl.Price)
		}
	}
	invalidationRefs = filterPositiveLevels(invalidationRefs)
	targetRefs = filterPositiveLevels(targetRefs)
	return invalidationRefs, targetRefs
}

func structuralTolerance(distance, atrPct, entry float64) float64 {
	if distance <= 0 {
		return 0
	}
	tol := distance * 0.35
	if atrPct > 0 && entry > 0 {
		atrDistance := entry * (atrPct / 100)
		if atrDistance > tol {
			tol = atrDistance
		}
	}
	if minTol := distance * 0.10; tol < minTol {
		tol = minTol
	}
	if maxTol := distance * 0.60; tol > maxTol {
		tol = maxTol
	}
	return tol
}

func nearestLevel(price float64, levels []float64) (float64, float64) {
	best := 0.0
	bestGap := 0.0
	for i, level := range levels {
		gap := absFloat(price - level)
		if i == 0 || gap < bestGap {
			best = level
			bestGap = gap
		}
	}
	return best, bestGap
}

func filterPositiveLevels(levels []float64) []float64 {
	out := make([]float64, 0, len(levels))
	for _, level := range levels {
		if level > 0 {
			out = append(out, level)
		}
	}
	return out
}

func fibonacciLevels(fib *AIEntryFibonacci) []float64 {
	if fib == nil {
		return nil
	}
	levels := filterPositiveLevels(fib.Levels)
	if fib.SwingHigh > 0 {
		levels = append(levels, fib.SwingHigh)
	}
	if fib.SwingLow > 0 {
		levels = append(levels, fib.SwingLow)
	}
	return levels
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func validateProtectionPlanAlignmentSkeleton(d Decision, rr AIRiskRewardRationale, config *store.StrategyConfig) error {
	if d.ProtectionPlan == nil && (config == nil || !config.Protection.FullTPSL.FallbackMaxLossEnabled) {
		return nil
	}
	var plan *AIProtectionPlan
	if d.ProtectionPlan != nil {
		plan = d.ProtectionPlan
		mode := strings.ToLower(plan.Mode)
		if mode == "" || mode == "full" {
			if plan.StopLossPct > 0 {
				expectedSL := protectionPctFromPrices(d.Action, rr.Entry, rr.Invalidation)
				if expectedSL > 0 && absFloat(plan.StopLossPct-expectedSL) > 0.05 {
					return fmt.Errorf("protection_plan.stop_loss_pct %.2f inconsistent with rationale invalidation %.2f", plan.StopLossPct, expectedSL)
				}
			}
			if plan.TakeProfitPct > 0 {
				expectedTP := protectionPctFromPrices(d.Action, rr.Entry, rr.FirstTarget)
				if expectedTP > 0 && absFloat(plan.TakeProfitPct-expectedTP) > 0.05 {
					return fmt.Errorf("protection_plan.take_profit_pct %.2f inconsistent with rationale first_target %.2f", plan.TakeProfitPct, expectedTP)
				}
			}
		}
		if mode == "ladder" || mode == "combined" {
			applyLadderVolatilityBuffers(d.Action, rr, plan)
			if err := validateLadderPlanStructuralAlignment(d.Action, rr, d.EntryProtection, plan); err != nil {
				return err
			}
		}
		if err := validateBreakEvenTriggerAlignment(d.Action, rr, plan); err != nil {
			return err
		}
	}
	if err := validateFallbackMaxLossAlignment(d.Action, rr, plan, config); err != nil {
		return err
	}
	return nil
}

func ensureLadderVolatilityBuffer(d Decision, rule *AIProtectionLadderRule, decisionIndex, ruleIndex int) {
	if rule == nil || rule.VolatilityBufferPct > 0 {
		return
	}
	if d.EntryProtection != nil && d.EntryProtection.VolatilityAdjustment.ATR14Pct > 0 {
		rule.VolatilityBufferPct = d.EntryProtection.VolatilityAdjustment.ATR14Pct * 0.35
		rule.VolatilityBufferReason = firstNonEmptyString(rule.VolatilityBufferReason, "auto 0.35x ATR14 buffer from entry_protection_rationale")
		return
	}
	logger.Warnf("decision #%d ladder_rule[%d]: missing volatility buffer; structural alignment fallback will be used", decisionIndex+1, ruleIndex)
}

func firstNonEmptyString(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func applyLadderVolatilityBuffers(action string, rr AIRiskRewardRationale, plan *AIProtectionPlan) {
	if plan == nil || len(plan.LadderRules) == 0 || rr.Entry <= 0 {
		return
	}
	for i := range plan.LadderRules {
		rule := &plan.LadderRules[i]
		if rule.VolatilityBufferPct <= 0 {
			continue
		}
		bufferMove := rr.Entry * rule.VolatilityBufferPct / 100.0
		if rule.StopLossPrice > 0 {
			switch action {
			case "open_long":
				rule.StopLossPrice -= bufferMove
			case "open_short":
				rule.StopLossPrice += bufferMove
			}
		}
		if rule.TakeProfitPrice > 0 {
			switch action {
			case "open_long":
				rule.TakeProfitPrice -= bufferMove
			case "open_short":
				rule.TakeProfitPrice += bufferMove
			}
		}
	}
}

func validateLadderPlanStructuralAlignment(action string, rr AIRiskRewardRationale, rationale *AIEntryProtectionRationale, plan *AIProtectionPlan) error {
	if plan == nil || rationale == nil || len(plan.LadderRules) == 0 {
		return nil
	}
	entry := rr.Entry
	if entry <= 0 {
		return nil
	}

	riskDistance := absFloat(rr.Entry - rr.Invalidation)
	rewardDistance := absFloat(rr.FirstTarget - rr.Entry)
	invalidationRefs, targetRefs := structuralReferenceLevels(rationale)
	if len(targetRefs) == 0 {
		switch action {
		case "open_long":
			targetRefs = filterPositiveLevels(rationale.KeyLevels.Resistance)
		case "open_short":
			targetRefs = filterPositiveLevels(rationale.KeyLevels.Support)
		}
	}
	if len(invalidationRefs) == 0 {
		switch action {
		case "open_long":
			invalidationRefs = filterPositiveLevels(rationale.KeyLevels.Support)
		case "open_short":
			invalidationRefs = filterPositiveLevels(rationale.KeyLevels.Resistance)
		}
	}
	fibLevels := fibonacciLevels(rationale.KeyLevels.Fibonacci)
	targetRefs = append(targetRefs, fibLevels...)

	targetTol := structuralTolerance(maxFloat(rewardDistance, entry*0.005), rationale.VolatilityAdjustment.ATR14Pct, entry)
	stopTol := structuralTolerance(maxFloat(riskDistance, entry*0.005), rationale.VolatilityAdjustment.ATR14Pct, entry)

	for i, rule := range plan.LadderRules {
		if rule.TakeProfitCloseRatioPct > 0 {
			tpPrice := ladderRulePriceFromPctOrAbsolute(action, entry, rule.TakeProfitPct, rule.TakeProfitPrice, true)
			unbufferedTPPrice := ladderRulePriceFromPctOrAbsolute(action, entry, rule.TakeProfitPct, removeLadderBuffer(action, entry, rule.TakeProfitPrice, rule.VolatilityBufferPct, true), true)
			if tpPrice <= 0 {
				return fmt.Errorf("protection_plan.ladder_rules[%d] take profit requires take_profit_price or take_profit_pct", i)
			}
			if !isTakeProfitPriceForAction(action, entry, tpPrice) {
				return fmt.Errorf("protection_plan.ladder_rules[%d] take profit %.4f is not executable for %s entry %.4f", i, tpPrice, action, entry)
			}
			if len(targetRefs) > 0 {
				nearest, gap := nearestLevel(unbufferedTPPrice, targetRefs)
				if gap > targetTol {
					return fmt.Errorf("protection_plan.ladder_rules[%d] take profit %.4f too far from structural/fibonacci target %.4f", i, tpPrice, nearest)
				}
			}
		}
		if rule.StopLossCloseRatioPct > 0 {
			slPrice := ladderRulePriceFromPctOrAbsolute(action, entry, rule.StopLossPct, rule.StopLossPrice, false)
			unbufferedSLPrice := ladderRulePriceFromPctOrAbsolute(action, entry, rule.StopLossPct, removeLadderBuffer(action, entry, rule.StopLossPrice, rule.VolatilityBufferPct, false), false)
			if slPrice <= 0 {
				return fmt.Errorf("protection_plan.ladder_rules[%d] stop loss requires stop_loss_price or stop_loss_pct", i)
			}
			if !isStopLossPriceForAction(action, entry, slPrice) {
				return fmt.Errorf("protection_plan.ladder_rules[%d] stop loss %.4f is not executable for %s entry %.4f", i, slPrice, action, entry)
			}
			if len(invalidationRefs) > 0 {
				nearest, gap := nearestLevel(unbufferedSLPrice, invalidationRefs)
				if gap > stopTol {
					return fmt.Errorf("protection_plan.ladder_rules[%d] stop loss %.4f too far from structural invalidation %.4f", i, slPrice, nearest)
				}
			}
		}
	}
	return nil
}

func ladderRulePriceFromPctOrAbsolute(action string, entry, pct, absolute float64, takeProfit bool) float64 {
	if absolute > 0 {
		return absolute
	}
	if entry <= 0 || pct <= 0 {
		return 0
	}
	move := pct / 100.0
	if takeProfit {
		if action == "open_long" {
			return entry * (1 + move)
		}
		if action == "open_short" {
			return entry * (1 - move)
		}
	} else {
		if action == "open_long" {
			return entry * (1 - move)
		}
		if action == "open_short" {
			return entry * (1 + move)
		}
	}
	return 0
}

func isTakeProfitPriceForAction(action string, entry, price float64) bool {
	return entry > 0 && price > 0 && ((action == "open_long" && price > entry) || (action == "open_short" && price < entry))
}

func isStopLossPriceForAction(action string, entry, price float64) bool {
	return entry > 0 && price > 0 && ((action == "open_long" && price < entry) || (action == "open_short" && price > entry))
}

func removeLadderBuffer(action string, entry, price, bufferPct float64, takeProfit bool) float64 {
	if entry <= 0 || price <= 0 || bufferPct <= 0 {
		return price
	}
	move := entry * bufferPct / 100.0
	if takeProfit {
		switch action {
		case "open_long":
			return price + move
		case "open_short":
			return price - move
		}
	} else {
		switch action {
		case "open_long":
			return price + move
		case "open_short":
			return price - move
		}
	}
	return price
}

func validateBreakEvenTriggerAlignment(action string, rr AIRiskRewardRationale, plan *AIProtectionPlan) error {
	if plan == nil || plan.BreakEvenValue <= 0 || plan.BreakEvenTrigger == "" {
		return nil
	}
	firstTargetPct := protectionPctFromPrices(action, rr.Entry, rr.FirstTarget)
	if firstTargetPct <= 0 {
		return nil
	}
	switch strings.ToLower(plan.BreakEvenTrigger) {
	case "profit_pct":
		if plan.BreakEvenValue-firstTargetPct > 0.05 {
			return fmt.Errorf("protection_plan.break_even_trigger_value %.2f exceeds rationale first_target %.2f", plan.BreakEvenValue, firstTargetPct)
		}
	case "r_multiple":
		if plan.BreakEvenValue-rr.GrossEstimatedRR > 0.05 {
			return fmt.Errorf("protection_plan.break_even_trigger_value %.2f exceeds rationale first_target rr %.2f", plan.BreakEvenValue, rr.GrossEstimatedRR)
		}
	}
	return nil
}

func validateFallbackMaxLossAlignment(action string, rr AIRiskRewardRationale, plan *AIProtectionPlan, cfg *store.StrategyConfig) error {
	if cfg == nil {
		return nil
	}
	full := cfg.Protection.FullTPSL
	if !full.Enabled || full.Mode == store.ProtectionModeDisabled || !full.FallbackMaxLossEnabled {
		return nil
	}
	fallbackPct, ok := resolveManualFallbackMaxLossPct(full)
	if !ok {
		return nil
	}
	invalidationPct := protectionPctFromPrices(action, rr.Entry, rr.Invalidation)
	if invalidationPct <= 0 {
		return nil
	}
	if fallbackPct+0.05 < invalidationPct {
		return fmt.Errorf("strategy full_tp_sl fallback_max_loss %.2f sits inside rationale invalidation %.2f", fallbackPct, invalidationPct)
	}
	return nil
}

func resolveManualFallbackMaxLossPct(full store.FullTPSLConfig) (float64, bool) {
	if full.FallbackMaxLoss.Mode != store.ProtectionValueModeManual || full.FallbackMaxLoss.Value <= 0 {
		return 0, false
	}
	return full.FallbackMaxLoss.Value, true
}

func protectionPctFromPrices(action string, entry, target float64) float64 {
	if entry <= 0 || target <= 0 {
		return 0
	}
	switch action {
	case "open_long":
		if target == entry {
			return 0
		}
		return absFloat((target-entry)/entry) * 100
	case "open_short":
		if target == entry {
			return 0
		}
		return absFloat((entry-target)/entry) * 100
	default:
		return 0
	}
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ParseAndValidateAIDecisions parses decisions and validates them with awareness of XML reasoning blocks.
func ParseAndValidateAIDecisions(response string) ([]Decision, error) {
	decisions, _, err := extractDecisions(response)
	if err != nil {
		return nil, err
	}
	if err := ValidateDecisionFormatWithCoT(decisions, extractCoTTrace(response)); err != nil {
		return decisions, err
	}
	return decisions, nil
}

func ParseAndValidateAIDecisionsWithStrategy(response string, config *store.StrategyConfig) ([]Decision, error) {
	decisions, _, err := extractDecisions(response)
	if err != nil {
		return nil, err
	}
	normalizeAndRepairOpenDecisions(decisions)
	if config != nil {
		for i := range decisions {
			if decisions[i].EntryProtection != nil {
				trimEntryProtectionToConfigLimits(decisions[i].EntryProtection, config.EntryStructure)
			}
		}
	}
	cot := extractCoTTrace(response)
	if err := ValidateAIDecisionsWithStrategyAndCoT(decisions, config, cot); err != nil {
		return decisions, err
	}
	if err := ValidateProtectionReasoningContract(cot, config); err != nil {
		return decisions, err
	}
	return decisions, nil
}

func normalizeAndRepairOpenDecisions(decisions []Decision) {
	for i := range decisions {
		d := &decisions[i]
		d.Action = strings.ToLower(strings.TrimSpace(d.Action))
		d.Symbol = strings.ToUpper(strings.TrimSpace(d.Symbol))
		if d.Action != "open_long" && d.Action != "open_short" {
			continue
		}
		if d.EntryProtection != nil {
			if len(d.EntryProtection.StructuralKeyLevels) == 0 && len(d.StructuralKeyLevels) > 0 {
				d.EntryProtection.StructuralKeyLevels = append([]AIStructuralKeyLevel{}, d.StructuralKeyLevels...)
			}
			normalizeEntryProtectionRationale(d.EntryProtection)
		}
		if d.ProtectionPlan != nil {
			normalizeProtectionPlan(d.ProtectionPlan)
			alignLadderPlanToStructure(d.Action, d.EntryProtection, d.ProtectionPlan)
		}
	}
}

func normalizeEntryProtectionRationale(ep *AIEntryProtectionRationale) {
	if ep == nil {
		return
	}
	ep.TimeframeContext.Primary = strings.TrimSpace(ep.TimeframeContext.Primary)
	for i := range ep.TimeframeContext.Lower {
		ep.TimeframeContext.Lower[i] = strings.TrimSpace(ep.TimeframeContext.Lower[i])
	}
	for i := range ep.TimeframeContext.Higher {
		ep.TimeframeContext.Higher[i] = strings.TrimSpace(ep.TimeframeContext.Higher[i])
	}
	for i := range ep.AlignmentNotes {
		ep.AlignmentNotes[i] = strings.TrimSpace(ep.AlignmentNotes[i])
	}
	for i := range ep.Anchors {
		ep.Anchors[i].Type = strings.ToLower(strings.TrimSpace(ep.Anchors[i].Type))
		ep.Anchors[i].Timeframe = strings.TrimSpace(ep.Anchors[i].Timeframe)
		ep.Anchors[i].Reason = strings.TrimSpace(ep.Anchors[i].Reason)
	}
	for i := range ep.StructuralKeyLevels {
		ep.StructuralKeyLevels[i].Type = strings.ToLower(strings.TrimSpace(ep.StructuralKeyLevels[i].Type))
		ep.StructuralKeyLevels[i].Timeframe = strings.TrimSpace(ep.StructuralKeyLevels[i].Timeframe)
		ep.StructuralKeyLevels[i].Source = strings.TrimSpace(ep.StructuralKeyLevels[i].Source)
		ep.StructuralKeyLevels[i].UsedFor = strings.TrimSpace(ep.StructuralKeyLevels[i].UsedFor)
	}
	backfillEntryProtectionKeyLevels(ep)
	backfillStructuralKeyLevels(ep)
}

func trimEntryProtectionToConfigLimits(ep *AIEntryProtectionRationale, entryStructure store.EntryStructureConfig) {
	if ep == nil {
		return
	}
	trimFloatSlice := func(src []float64, max int) []float64 {
		if len(src) <= max || max <= 0 {
			return src
		}
		return src[:max]
	}
	if entryStructure.MaxSupportLevels > 0 {
		ep.KeyLevels.Support = trimFloatSlice(ep.KeyLevels.Support, entryStructure.MaxSupportLevels)
	}
	if entryStructure.MaxResistanceLevels > 0 {
		ep.KeyLevels.Resistance = trimFloatSlice(ep.KeyLevels.Resistance, entryStructure.MaxResistanceLevels)
	}
	if entryStructure.MaxAnchorCount > 0 && len(ep.Anchors) > entryStructure.MaxAnchorCount {
		priority := func(anchor AIEntryProtectionAnchor) int {
			t := strings.ToLower(strings.TrimSpace(anchor.Type))
			switch t {
			case "support", "resistance", "swing_low", "swing_high", "fib_support", "fib_resistance", "fibonacci", "first_target":
				return 0
			default:
				return 1
			}
		}
		sort.SliceStable(ep.Anchors, func(i, j int) bool {
			return priority(ep.Anchors[i]) < priority(ep.Anchors[j])
		})
		ep.Anchors = ep.Anchors[:entryStructure.MaxAnchorCount]
	}
}

func normalizeProtectionPlan(pp *AIProtectionPlan) {
	if pp == nil {
		return
	}
	pp.Mode = strings.ToLower(strings.TrimSpace(pp.Mode))
	pp.BreakEvenTrigger = strings.TrimSpace(pp.BreakEvenTrigger)
	pp.BreakEvenAnchor = strings.TrimSpace(pp.BreakEvenAnchor)
	for i := range pp.DrawdownRules {
		pp.DrawdownRules[i].Timeframe = strings.TrimSpace(pp.DrawdownRules[i].Timeframe)
		pp.DrawdownRules[i].ReasonAnchor = strings.TrimSpace(pp.DrawdownRules[i].ReasonAnchor)
		pp.DrawdownRules[i].StageName = strings.TrimSpace(pp.DrawdownRules[i].StageName)
		pp.DrawdownRules[i].RunnerStopMode = strings.TrimSpace(pp.DrawdownRules[i].RunnerStopMode)
		pp.DrawdownRules[i].RunnerStopSource = strings.TrimSpace(pp.DrawdownRules[i].RunnerStopSource)
		pp.DrawdownRules[i].RunnerTargetMode = strings.TrimSpace(pp.DrawdownRules[i].RunnerTargetMode)
		pp.DrawdownRules[i].RunnerTargetSource = strings.TrimSpace(pp.DrawdownRules[i].RunnerTargetSource)
	}
}

func alignLadderPlanToStructure(action string, ep *AIEntryProtectionRationale, pp *AIProtectionPlan) {
	if ep == nil || pp == nil || len(pp.LadderRules) == 0 {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(pp.Mode))
	if mode != "ladder" && mode != "combined" {
		return
	}

	_, targetRefs := structuralReferenceLevels(ep)
	if len(targetRefs) == 0 {
		switch action {
		case "open_long":
			targetRefs = filterPositiveLevels(ep.KeyLevels.Resistance)
		case "open_short":
			targetRefs = filterPositiveLevels(ep.KeyLevels.Support)
		}
	}
	targetRefs = append(targetRefs, fibonacciLevels(ep.KeyLevels.Fibonacci)...)
	invalidationRefs, _ := structuralReferenceLevels(ep)
	if len(invalidationRefs) == 0 {
		switch action {
		case "open_long":
			invalidationRefs = filterPositiveLevels(ep.KeyLevels.Support)
		case "open_short":
			invalidationRefs = filterPositiveLevels(ep.KeyLevels.Resistance)
		}
	}

	for i := range pp.LadderRules {
		rule := &pp.LadderRules[i]
		if rule.TakeProfitPrice <= 0 && rule.TakeProfitPct > 0 && len(targetRefs) > 0 {
			pctPrice := ladderRulePriceFromPctOrAbsolute(action, ep.RiskReward.Entry, rule.TakeProfitPct, 0, true)
			nearest, _ := nearestLevel(pctPrice, targetRefs)
			if isTakeProfitPriceForAction(action, ep.RiskReward.Entry, nearest) {
				rule.TakeProfitPrice = nearest
			}
		}
		if rule.StopLossPrice <= 0 && len(invalidationRefs) > 0 {
			nearest := bestStopLossReference(action, ep.RiskReward.Entry, invalidationRefs)
			if isStopLossPriceForAction(action, ep.RiskReward.Entry, nearest) {
				rule.StopLossPrice = nearest
				rule.StopLossAnchor = firstNonEmptyString(rule.StopLossAnchor, "auto-aligned to structural invalidation")
			}
		}
		if rule.TakeProfitPct <= 0 && rule.TakeProfitPrice > 0 {
			rule.TakeProfitPct = protectionPctFromPrices(action, ep.RiskReward.Entry, rule.TakeProfitPrice)
		}
		if rule.StopLossPct <= 0 && rule.StopLossPrice > 0 {
			rule.StopLossPct = protectionPctFromPrices(action, ep.RiskReward.Entry, rule.StopLossPrice)
		}
	}
}

func bestStopLossReference(action string, entry float64, refs []float64) float64 {
	best := 0.0
	for _, ref := range refs {
		if !isStopLossPriceForAction(action, entry, ref) {
			continue
		}
		if best == 0 || absFloat(entry-ref) < absFloat(entry-best) {
			best = ref
		}
	}
	return best
}

func backfillStructuralKeyLevels(ep *AIEntryProtectionRationale) {
	if ep == nil || len(ep.StructuralKeyLevels) > 0 {
		return
	}
	primary := strings.TrimSpace(ep.TimeframeContext.Primary)
	for _, v := range ep.KeyLevels.Support {
		if v > 0 {
			ep.StructuralKeyLevels = append(ep.StructuralKeyLevels, AIStructuralKeyLevel{Price: v, Type: "support", Timeframe: primary, Source: "backfilled_key_levels", UsedFor: "reference"})
		}
	}
	for _, v := range ep.KeyLevels.Resistance {
		if v > 0 {
			ep.StructuralKeyLevels = append(ep.StructuralKeyLevels, AIStructuralKeyLevel{Price: v, Type: "resistance", Timeframe: primary, Source: "backfilled_key_levels", UsedFor: "reference"})
		}
	}
}

func backfillEntryProtectionKeyLevels(ep *AIEntryProtectionRationale) {
	if ep == nil {
		return
	}
	seenSupport := map[string]struct{}{}
	seenResistance := map[string]struct{}{}
	for _, v := range ep.KeyLevels.Support {
		seenSupport[fmt.Sprintf("%.8f", v)] = struct{}{}
	}
	for _, v := range ep.KeyLevels.Resistance {
		seenResistance[fmt.Sprintf("%.8f", v)] = struct{}{}
	}

	metaSupport, _ := schemaMeta("key_levels.support")
	metaResistance, _ := schemaMeta("key_levels.resistance")

	// 1) structural_key_levels → support/resistance buckets
	for _, lvl := range ep.StructuralKeyLevels {
		if lvl.Price <= 0 {
			continue
		}
		key := fmt.Sprintf("%.8f", lvl.Price)
		switch strings.ToLower(strings.TrimSpace(lvl.Type)) {
		case "support":
			if metaSupport.AutoFill {
				if _, ok := seenSupport[key]; !ok {
					ep.KeyLevels.Support = append(ep.KeyLevels.Support, lvl.Price)
					seenSupport[key] = struct{}{}
				}
			}
		case "resistance":
			if metaResistance.AutoFill {
				if _, ok := seenResistance[key]; !ok {
					ep.KeyLevels.Resistance = append(ep.KeyLevels.Resistance, lvl.Price)
					seenResistance[key] = struct{}{}
				}
			}
		}
	}

	// 2) anchors → support/resistance buckets as fallback
	for _, a := range ep.Anchors {
		if a.Price <= 0 {
			continue
		}
		key := fmt.Sprintf("%.8f", a.Price)
		t := strings.ToLower(strings.TrimSpace(a.Type))
		if strings.Contains(t, "support") && metaSupport.AutoFill {
			if _, ok := seenSupport[key]; !ok {
				ep.KeyLevels.Support = append(ep.KeyLevels.Support, a.Price)
				seenSupport[key] = struct{}{}
			}
		}
		if strings.Contains(t, "resistance") && metaResistance.AutoFill {
			if _, ok := seenResistance[key]; !ok {
				ep.KeyLevels.Resistance = append(ep.KeyLevels.Resistance, a.Price)
				seenResistance[key] = struct{}{}
			}
		}
	}

	// Keep support descending and resistance ascending for consistency.
	sort.Slice(ep.KeyLevels.Support, func(i, j int) bool { return ep.KeyLevels.Support[i] > ep.KeyLevels.Support[j] })
	sort.Slice(ep.KeyLevels.Resistance, func(i, j int) bool { return ep.KeyLevels.Resistance[i] < ep.KeyLevels.Resistance[j] })
}

func extractTopLevelJSONArray(s string) string {
	start := strings.Index(s, "[")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
