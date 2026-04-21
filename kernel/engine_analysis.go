package kernel

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"regexp"
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

	// 1. First fetch data for position coins (must fetch)
	for _, pos := range ctx.Positions {
		data, err := market.GetWithTimeframes(pos.Symbol, timeframes, primaryTimeframe, klineCount)
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

		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primaryTimeframe, klineCount)
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

	jsonStr = strings.ReplaceAll(jsonStr, "［", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{")
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}")
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":")
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "【", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "　", " ")

	return jsonStr
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

	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
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
			return fmt.Errorf("current strategy route supports only one AI protection route at a time (full, ladder, or drawdown)")
		}
		if config.Protection.BreakEvenStop.Enabled && isOpen {
			if d.ProtectionPlan == nil || d.ProtectionPlan.BreakEvenTrigger == "" || d.ProtectionPlan.BreakEvenValue <= 0 {
				return fmt.Errorf("decision #%d: current strategy route requires break-even protection output for open actions", i+1)
			}
		}
		if ladderAI && !fullAI && !drawdownAI {
			if d.ProtectionPlan == nil || d.ProtectionPlan.Mode != "ladder" {
				return fmt.Errorf("decision #%d: current strategy route requires ladder protection_plan for open actions", i+1)
			}
			if n := len(d.ProtectionPlan.LadderRules); n < 2 || n > 3 {
				return fmt.Errorf("decision #%d: ladder protection_plan must contain 2~3 ladder_rules under current strategy route", i+1)
			}
		}
		if fullAI && !ladderAI && !drawdownAI {
			if d.ProtectionPlan == nil || d.ProtectionPlan.Mode != "full" {
				return fmt.Errorf("decision #%d: current strategy route requires full protection_plan for open actions", i+1)
			}
		}
		if drawdownAI && !fullAI && !ladderAI {
			if d.ProtectionPlan == nil || d.ProtectionPlan.Mode != "drawdown" {
				return fmt.Errorf("decision #%d: current strategy route requires drawdown protection_plan for open actions", i+1)
			}
			if len(d.ProtectionPlan.DrawdownRules) == 0 {
				return fmt.Errorf("decision #%d: drawdown protection_plan must contain drawdown_rules under current strategy route", i+1)
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
			if entryStructure.RequireStructuralAnchors && len(d.EntryProtection.Anchors) == 0 {
				return fmt.Errorf("entry_protection_rationale.anchors is required")
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
	if rr.MinRequiredRR > 0 && absFloat(rr.MinRequiredRR-minRR) > 0.02 {
		return fmt.Errorf("entry_protection_rationale.risk_reward min_required_rr %.2f inconsistent with strategy min %.2f", rr.MinRequiredRR, minRR)
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
		if err := validateBreakEvenTriggerAlignment(d.Action, rr, plan); err != nil {
			return err
		}
	}
	if err := validateFallbackMaxLossAlignment(d.Action, rr, plan, config); err != nil {
		return err
	}
	return nil
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
	cot := extractCoTTrace(response)
	if err := ValidateAIDecisionsWithStrategyAndCoT(decisions, config, cot); err != nil {
		return decisions, err
	}
	if err := ValidateProtectionReasoningContract(cot, config); err != nil {
		return decisions, err
	}
	return decisions, nil
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
