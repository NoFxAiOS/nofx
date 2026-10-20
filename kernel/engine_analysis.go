package kernel

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/provider/nofxos"
	"nofx/store"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ============================================================================
// Pre-compiled regular expressions (performance optimization)
// ============================================================================

var (
	// Safe regex: precisely match ```json code blocks
	// The (?:\{.*?\}\s*,?\s*)* group allows empty decision arrays [] — a valid
	// "no trades this cycle" verdict from the model.
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*(?:\\{.*?\\}\\s*,?\\s*)*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*(?:\{.*?\}\s*,?\s*)*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*(?:\{|\])`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")
	// Placeholder values models emit for unknown numbers (?, ??, ？, N/A, —)
	rePlaceholderVal = regexp.MustCompile(`:\s*("[^"]*")?[?？]+\s*|:\s*"(?:N/A|n/a|NA|—|–|TBD|unknown)"`)
	reTrailingComma  = regexp.MustCompile(`,\s*(\}|\])`)

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

	// Clamp strategy limits to prevent token overflow
	engineConfig := engine.GetConfig()
	engineConfig.ClampLimits()

	// The config estimate is an early advisory only. Dynamic prompt content
	// (raw OHLCV, positions, history and rules) is measured after the prompt is
	// built; it must never be admitted solely from this static estimate.
	estimate := engineConfig.EstimateTokens()

	// Determine context limit for the specific model being used
	contextLimit := 131072 // safe default (strictest common limit)
	outputReserve := 16384
	var providerName string
	if embedder, ok := mcpClient.(mcp.ClientEmbedder); ok {
		base := embedder.BaseClient()
		providerName = base.Provider
		contextLimit = store.GetContextLimitForClient(base.Provider, base.Model)
		if base.Cfg != nil && base.Cfg.MaxContext > 0 {
			contextLimit = base.Cfg.MaxContext
		}
		if base.MaxTokens > 0 {
			outputReserve = base.MaxTokens
		}
	}

	if estimate.Total*100/contextLimit >= 80 {
		logger.Infof("⚠️  Static token estimate %d — approaching %s context limit %d; final prompt will be measured after rendering",
			estimate.Total, providerName, contextLimit)
	}

	// 1. Fetch market data using strategy config
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataWithStrategy(ctx, engine); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
		// Header time must not predate the data it heads: CurrentTime was
		// stamped at context build, BEFORE this fetch loop spent ~1min
		// collecting 9 coins' klines/derivatives (09-18 audit #4: header
		// 15:30:22 vs signal timestamps 15:31:36+ confused the model's
		// freshness reasoning). Re-stamp at fetch completion — every signal
		// timestamp is now ≤ the header.
		ctx.CurrentTime = time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	}

	// Ensure OITopDataMap is initialized
	if ctx.OITopDataMap == nil {
		ctx.OITopDataMap = make(map[string]*OITopData)
		// Vergex only (same source as the data page). NofxOS public keys were
		// deprecated server-side, so there is no fallback.
		var oiPositions []nofxos.OIPosition
		if oiRanking, err := engine.vergexClient.GetOIRanking("1h", 20); err == nil {
			oiPositions = oiRanking.TopPositions
		}
		if len(oiPositions) > 0 {
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
	inputBudget := contextLimit - outputReserve
	if inputBudget <= 0 {
		inputBudget = contextLimit / 2
	}
	actualPromptTokens := estimateRenderedPromptTokens(systemPrompt, userPrompt)
	if actualPromptTokens > inputBudget && engineConfig.Indicators.EnableRawKlines {
		logger.Infof("⚠️  Rendered prompt estimate %d exceeds input budget %d; removing raw OHLCV from the rendered snapshot",
			actualPromptTokens, inputBudget)
		userPrompt = stripRawKlinesFromPrompt(userPrompt)
		actualPromptTokens = estimateRenderedPromptTokens(systemPrompt, userPrompt)
	}
	if actualPromptTokens > inputBudget {
		logger.Errorf("🚫 Rendered prompt estimate %d exceeds %s input budget %d (context %d, output reserve %d)",
			actualPromptTokens, providerName, inputBudget, contextLimit, outputReserve)
		return nil, fmt.Errorf("rendered prompt requires approximately %d tokens but model input budget is %d; reduce coins, timeframes, rules, or output token reserve",
			actualPromptTokens, inputBudget)
	}

	// 3.5 Regime-level skip (09-19 audit): when EVERY candidate is
	// double-blocked by the hard gate (no allowed direction, no
	// exception-eligible path) AND every open position is close-locked this
	// cycle (no legal close/partial path — positionCloseLocked mirrors the
	// trader's gates), the LLM call can only ever return a hold — spending
	// the tokens and the minutes to hear it is pure waste. Synthesize the
	// wait programmatically; the decision record still lands with full
	// prompts. Anything alive — one allowed direction, one
	// exception-eligible coin, one position with a legal action — makes the
	// call as usual.
	if len(ctx.CandidateCoins) > 0 {
		allBlocked := true
		renderedCount := 0
		// Position symbols render in the POSITIONS section, never as
		// candidate blocks — the skip message must count what the prompt
		// table actually renders (user audit 2026-10-01: the skip message
		// said 14 while the table had 13, because a held symbol with market
		// data was counted as "rendered").
		skipPositionSymbols := make(map[string]bool, len(ctx.Positions))
		for _, p := range ctx.Positions {
			skipPositionSymbols[market.Normalize(p.Symbol)] = true
		}
		for _, coin := range ctx.CandidateCoins {
			// The skip message cites the RENDERED count (position-overlap and
			// data-less coins don't render — 09-19 audit: "9 个候选" over 6
			// rendered blocks confused the reader).
			if gs, ok := ctx.GateStates[market.Normalize(coin.Symbol)]; ok && gs != nil && !gs.HardBlocked {
				allBlocked = false
				break
			}
			if skipPositionSymbols[market.Normalize(coin.Symbol)] {
				continue
			}
			if ctx.MarketDataMap[coin.Symbol] != nil {
				renderedCount++
			}
		}
		allLocked := true
		for _, p := range ctx.Positions {
			// Manual positions have NO legal model action this cycle (hold
			// only, user review 2026-09-27 #3) — they must never force the
			// LLM call the way an actionable AI position does.
			if !p.Managed {
				continue
			}
			md := ctx.MarketDataMap[p.Symbol]
			if locked, _ := positionCloseLocked(p, md, &engine.GetConfig().RiskControl); !locked {
				allLocked = false // fail-open: unknown hold age or live data → call the LLM
				break
			}
		}
		allBlocked = allBlocked && allLocked
		if allBlocked {
			positionState := "且无持仓"
			if len(ctx.Positions) > 0 {
				positionState = fmt.Sprintf("且 %d 个现有持仓均不可由本轮模型操作(手工持仓或平仓门锁定)", len(ctx.Positions))
			}
			logger.Infof("⏭️  [Regime Skip] 全部 %d 个候选双向硬门拦截%s — 跳过本次 LLM 调用,程序合成 wait(下一周期快照自动重评)", len(ctx.CandidateCoins), positionState)
			entryQuality := 0
			fd := &FullDecision{
				Decisions: []Decision{{
					Symbol:       "ALL",
					Action:       "wait",
					Reasoning:    fmt.Sprintf("Regime skip: 全部 %d 个渲染候选的开仓硬门双向均为程序拦截(无 allowed 方向、无市价例外路径),%s — 程序直接合成 wait,本轮未调用 LLM;结构变化后下一周期快照自动重评", renderedCount, positionState),
					EntryQuality: &entryQuality,
					// 2026-09-27 data-quality fix: CONFLICT_UNRESOLVED misdescribed
					// regime-skip cycles (the real blockers are per-coin hard-gate
					// codes — RR_MAX/STOP_PLAN/VENDOR_DIVERGENCE/MICRO_TREND/…,
					// preserved in the compressed prompt lines and the
					// gate_shadow_blocks table). Dedicated enum instead.
					BlockingFactors: []string{"ALL_CANDIDATES_HARD_BLOCKED"},
					NoTradeReasons:  []string{"全部候选双向硬门拦截", "下一周期快照自动重评"},
					Stage:           "NO_SETUP",
				}},
				SystemPrompt: systemPrompt,
				UserPrompt:   userPrompt,
				RawResponse:  "program-synthesized wait (regime skip)",
				Timestamp:    time.Now(),
			}
			return fd, nil
		}
	}

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
		engine.EffectiveMinPositionSize(),
		positionSymbolsFromContext(ctx),
		ctx.GateStates,
		ctx.LimitAnchors,
	)

	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt
		decision.UserPrompt = userPrompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
		decision.RawResponse = aiResponse
		// review 2026-10-07 B1-6: the anchor / stop-plan / TP-plan correctors
		// now run inside parseFullDecisionResponse BEFORE per-decision
		// validation, so placeholder SL/TP/price values are repaired instead
		// of rejected, and validation sees the final action. Nothing to redo
		// here.
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

// estimateRenderedPromptTokens counts the content that will actually be sent.
// ASCII-heavy JSON averages about three characters per token; CJK and other
// non-ASCII runes can consume one or more tokens, so count them as two. The
// estimate is deliberately conservative because a truncated decision JSON is
// unusable and model-specific tokenizers are not available in this package.
func estimateRenderedPromptTokens(parts ...string) int {
	total := 0
	for _, part := range parts {
		ascii, nonASCII := 0, 0
		for len(part) > 0 {
			r, size := utf8.DecodeRuneInString(part)
			part = part[size:]
			if r <= 0x7f {
				ascii++
			} else {
				nonASCII++
			}
		}
		total += (ascii+2)/3 + nonASCII*2 + 10
	}
	return total
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
	// Programmatic market regime always needs closed 1h + 4h history. Saved
	// strategies created before the regime field commonly list only
	// 5m/15m/1h; append missing frames at runtime instead of silently emitting
	// UNKNOWN and disabling every market-order exception.
	timeframes = withRequiredRegimeTimeframes(timeframes)
	if primaryTimeframe == "" {
		primaryTimeframe = timeframes[0]
	}
	if klineCount < store.MinKlineCount {
		klineCount = store.MinKlineCount
	}
	if klineCount > store.MaxKlineCount {
		klineCount = store.MaxKlineCount
	}

	logger.Infof("📊 Strategy timeframes: %v, Primary: %s, Kline count: %d", timeframes, primaryTimeframe, klineCount)

	// F10 (2026-10-01 review): analysis data comes from the venue that will
	// EXECUTE the trade — not always Binance.
	venue := ctx.Exchange
	if venue == "" {
		venue = "binance"
	}

	// 1. First fetch data for position coins (must fetch)
	for _, pos := range ctx.Positions {
		symbolTimeframes := withRequiredSymbolTimeframesFor(timeframes, pos.Symbol, config.RiskControl.BlockShort1dUptrend)
		data, err := market.GetWithTimeframesForVenue(pos.Symbol, symbolTimeframes, primaryTimeframe, klineCount, venue)
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

	// Minimum OI value filter — per-strategy config, 0/unset = built-in 15M USD.
	minOIThresholdMillions := config.CoinSource.EffectiveMinOIMillions()

	// F19 (2026-10-01 review): iterate over a snapshot — removeCandidate
	// mutates ctx.CandidateCoins in place, and `range` over the same slice
	// walked the shifted array, skipping the element AFTER each removal
	// (consecutive removals skipped several candidates' fetch entirely).
	ctx.CandidateOrder = ctx.CandidateOrder[:0]
	for _, c := range ctx.CandidateCoins {
		ctx.CandidateOrder = append(ctx.CandidateOrder, c.Symbol)
	}
	for _, coin := range snapshotCandidateCoins(ctx) {
		if _, exists := ctx.MarketDataMap[coin.Symbol]; exists {
			continue
		}

		symbolTimeframes := withRequiredSymbolTimeframesFor(timeframes, coin.Symbol, config.RiskControl.BlockShort1dUptrend)
		data, err := market.GetWithTimeframesForVenue(coin.Symbol, symbolTimeframes, primaryTimeframe, klineCount, venue)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for %s: %v", coin.Symbol, err)
			markFiltered(ctx, coin.Symbol, "market data unavailable/stale")
			continue
		}

		// Liquidity filter (skip for xyz dex assets - they don't have OI data from Binance)
		isExistingPosition := positionSymbols[coin.Symbol]
		isXyzAsset := market.IsXyzDexAsset(coin.Symbol)
		// F19 (2026-10-01 review): the near_high universe slot is granted at
		// scan time with the min-OI floor EXEMPTED (its liquidity guarantee
		// is the scanner's volume bar, engine.go near_high selection) — the
		// old post-fetch filter re-applied the floor anyway and silently
		// evicted exactly those candidates (pool ≠ prompt ≠ execable).
		isNearHighExempt := coin.ShortUniverse == "near_high" || coin.NearHighAlso
		if !isExistingPosition && !isXyzAsset && !isNearHighExempt && data.CurrentPrice > 0 {
			if !data.OpenInterestOK {
				// A failed OI fetch used to masquerade as OI=0 here and the
				// filter silently dropped EVERY candidate for the cycle
				// (round-4 review R4-6). Unknown ≠ zero: skip the level check
				// and say so — the universe-level min-OI gate at scan time
				// already applied.
				logger.Infof("⚠️  %s OI unknown (fetch failed) — liquidity filter skipped this cycle", coin.Symbol)
			} else if data.OpenInterest != nil {
				oiValue := data.OpenInterest.Latest * data.CurrentPrice
				oiValueInMillions := oiValue / 1_000_000
				if oiValueInMillions < minOIThresholdMillions {
					logger.Infof("⚠️  %s OI value too low (%.2fM USD < %.1fM), skipping coin", coin.Symbol, oiValueInMillions, minOIThresholdMillions)
					// Drop it from the candidate pool too (09-29 user report:
					// IOTA/AZTEC showed in the UI's 16-coin pool but never in
					// the prompt's 14-coin render — piggy-dash selects without
					// an OI floor, the fetch pass filters with one). Keeping
					// un-fetched coins in ctx.CandidateCoins desyncs the UI
					// pool, the prompt header count and the regime-skip census.
					markFiltered(ctx, coin.Symbol, fmt.Sprintf("OI %.2fM < %.1fM", oiValueInMillions, minOIThresholdMillions))
					removeCandidate(ctx, coin.Symbol)
					continue
				}
			}
		}

		// XYZ tokenized listings (data.Symbol carries the "NAME:TICKER"
		// exchange form) must never reach the decision layer — the candidate
		// name was normalized (colon stripped) so the choke-point filter
		// missed them; stop them here instead.
		if strings.Contains(strings.ToUpper(data.Symbol), ":") {
			logger.Infof("🚫 Excluded XYZ (tokenized) symbol from market data: %s (candidate %s)", data.Symbol, coin.Symbol)
			markFiltered(ctx, coin.Symbol, "tokenized symbol excluded")
			removeCandidate(ctx, coin.Symbol)
			continue
		}
		ctx.MarketDataMap[coin.Symbol] = data
	}

	logger.Infof("📊 Successfully fetched multi-timeframe market data for %d coins", len(ctx.MarketDataMap))
	return nil
}

// ============================================================================
// AI Response Parsing
// ============================================================================

func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64, minPositionSize float64, positionSymbols map[string]bool, gateStates map[string]*GateState, limitAnchors map[string]*LimitAnchor) (*FullDecision, error) {
	cotTrace := extractCoTTrace(aiResponse)

	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	// review 2026-10-07 B1-6: repair placeholder / drifted prices from the
	// program-computed plans BEFORE validation, so a zero SL/TP/limit price is
	// copied from the gate plan instead of failing the decision.
	correctLimitAnchors(decisions, limitAnchors, LimitAnchorTolerancePct)
	// Stop-plan compliance (round-4 review R4-1): snap stop_loss to the plan.
	correctStopLossToPlan(decisions, gateStates, StopPlanTolerancePct)
	// TP menu (user directive 09-29): point the gate state at the CHOSEN
	// option before the snap.
	resolveTakeProfitOptions(decisions, gateStates)
	// Reward-side symmetry (2026-09-27 external review P0).
	correctTakeProfitToPlan(decisions, gateStates, StopPlanTolerancePct)

	// review 2026-10-07 B1-6: validate per decision. One bad decision (amount
	// too small, invalid action) must not discard valid closes / stop moves
	// for other symbols. A rejected decision is replaced by a non-executing
	// hold (open position) or wait (flat) that carries the rejection reason,
	// so the decision record shows it.
	kept := make([]Decision, 0, len(decisions))
	rejected := 0
	var firstErr error
	for i := range decisions {
		d := decisions[i]
		norm := market.Normalize(d.Symbol)
		hasPos := positionSymbols[norm]
		verr := validateDecision(&d, accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio, minPositionSize, hasPos, gateStates[norm])
		if verr == nil {
			kept = append(kept, d)
			continue
		}
		rejected++
		if firstErr == nil {
			firstErr = fmt.Errorf("decision #%d validation failed: %w", i+1, verr)
		}
		logger.Warnf("⚠️  [ValidationRejected] decision #%d %s %s dropped: %v", i+1, d.Symbol, d.Action, verr)
		repl := Decision{
			Symbol:         decisions[i].Symbol,
			Action:         "wait",
			Reasoning:      decisions[i].Reasoning,
			NoTradeReasons: []string{fmt.Sprintf("VALIDATION_REJECTED: %s %s: %v", decisions[i].Action, decisions[i].Symbol, verr)},
		}
		if hasPos {
			repl.Action = "hold"
		}
		if rerr := validateDecision(&repl, accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio, minPositionSize, hasPos, gateStates[norm]); rerr != nil {
			// Cannot even express the rejection as a wait/hold: drop it silently
			// (already logged above) rather than risk executing it.
			continue
		}
		kept = append(kept, repl)
	}

	if len(decisions) > 0 && rejected == len(decisions) {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", firstErr)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: kept,
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

func extractDecisions(response string) ([]Decision, error) {
	s := removeInvisibleRunes(response)
	s = sanitizePlaceholders(s)
	s = strings.TrimSpace(s)
	s = fixMissingQuotes(s)

	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); match != nil && len(match) > 1 {
		inner := match[1]
		// A reasoning section that literally mentions "<decision>" must not
		// widen the payload: the final block is the one after the LAST open tag.
		if i := strings.LastIndex(inner, "<decision>"); i >= 0 {
			inner = inner[i+len("<decision>"):]
		}
		jsonPart = strings.TrimSpace(inner)
		logger.Infof("✓ Extracted JSON using <decision> tag")
	} else if strings.Contains(s, "<decision>") {
		// review 2026-10-20 area4 #3: once the model opened <decision> the
		// payload boundary is fixed. A missing </decision> means the final
		// block was truncated; falling back to a full-text array search would
		// pick up a draft array abandoned inside <reasoning> (e.g. a discarded
		// close_long) and could execute it. Reject instead.
		// Only salvage completed objects from INSIDE the final payload.
		payload := s[strings.LastIndex(s, "<decision>")+len("<decision>"):]
		if salvaged := salvageTruncatedDecisionArray(payload); len(salvaged) > 0 {
			logger.Infof("⚠️  <decision> not closed; salvaged %d complete decisions from the final payload", len(salvaged))
			return salvaged, nil
		}
		return nil, fmt.Errorf("<decision> tag opened but not closed (truncated or malformed final decision block); refusing to search the full text for a decision array")
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
			return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
		}
		return decisions, nil
	}

	jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
	if jsonContent == "" {
		// Max-token truncation amputates the tail of the decision array: the
		// model has usually already emitted several complete decision objects
		// before the cut. Salvage them instead of discarding the whole
		// response to safe-wait — the earlier coins' decisions are real
		// analysis, and a truncated WAIT-heavy board is still directionally
		// correct (positions beyond the cut just keep their default behavior).
		if salvaged := salvageTruncatedDecisionArray(jsonPart); salvaged != nil {
			logger.Warnf("⚠️  [TruncatedSalvage] Decision array was cut off mid-stream (max_tokens); recovered %d complete decisions, later coins default to no-action", len(salvaged))
			return salvaged, nil
		}

		if strings.TrimSpace(jsonPart) == "" {
			logger.Warnf("⚠️  [SafeFallback] AI response is empty after cleanup (%d raw bytes) — upstream returned blank content (check truncation/limits)", len(response))
		} else {
			logger.Infof("⚠️  [SafeFallback] AI didn't output JSON decision, entering safe wait mode")
		}

		// The model's conclusion sits at the END of its reasoning (the beginning
		// is preamble like "Let me analyze this carefully...") — summarize the
		// tail so users see the actual take-away.
		cotSummary := summarizeTail(jsonPart, 240)

		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("Model didn't output structured JSON decision, entering safe wait; summary: %s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent)

	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
	}

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
	}

	return decisions, nil
}

// sanitizePlaceholders replaces placeholder values models emit for unknown
// numbers (?, ??, full-width ？, "N/A", "—") with null, which json.Unmarshal
// treats as "leave the field at zero" — the decision stays executable instead
// of being rejected wholesale by strict JSON parsing.
func sanitizePlaceholders(s string) string {
	s = rePlaceholderVal.ReplaceAllString(s, ": null")
	s = reTrailingComma.ReplaceAllString(s, "$1")
	return s
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

	// An empty decision array [] is a valid "no trades this cycle" verdict.
	if trimmed == "[]" {
		return nil
	}

	if !reArrayHead.MatchString(trimmed) {
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	// review 2026-10-07 B1-6: only inspect characters OUTSIDE string literals —
	// "~80%" or "1,234" inside a reason string is harmless free text.
	outside := stripJSONStringLiterals(jsonStr)

	if strings.Contains(outside, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be precise single values")
	}

	for i := 0; i < len(outside)-4; i++ {
		if outside[i] >= '0' && outside[i] <= '9' &&
			outside[i+1] == ',' &&
			outside[i+2] >= '0' && outside[i+2] <= '9' &&
			outside[i+3] >= '0' && outside[i+3] <= '9' &&
			outside[i+4] >= '0' && outside[i+4] <= '9' {
			return fmt.Errorf("JSON numbers cannot contain thousand separator comma, found: %s", outside[i:min(i+10, len(outside))])
		}
	}

	return nil
}

// stripJSONStringLiterals returns s with the contents of every "..." literal
// blanked out (quotes kept, escapes honored), so structural checks only see
// the characters in value/key positions.
func stripJSONStringLiterals(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inString, escaped := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
				b.WriteByte(c)
			}
			continue
		}
		if c == '"' {
			inString = true
		}
		b.WriteByte(c)
	}
	return b.String()
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

// salvageTruncatedDecisionArray recovers complete decision objects from a
// response whose closing "]" was amputated by max-token truncation. It scans
// the first JSON-looking array, tracks brace depth (string-aware, escape-safe),
// and cuts after the last object that closed at depth 1 — then re-arms the
// array and parses normally. Returns nil when nothing complete survives.
func salvageTruncatedDecisionArray(s string) []Decision {
	start := strings.Index(s, "[")
	if start < 0 {
		return nil
	}
	depth, lastComplete := 0, -1
	inString, escaped := false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				lastComplete = i
			} else if depth < 0 {
				return nil // stray close brace — not a salvageable array
			}
		}
	}
	if lastComplete < 0 {
		return nil
	}
	candidate := s[start:lastComplete+1] + "]"
	if err := validateJSONFormat(candidate); err != nil {
		return nil
	}
	var decisions []Decision
	if err := json.Unmarshal([]byte(candidate), &decisions); err != nil {
		return nil
	}
	return decisions
}

// summarizeTail returns the last n characters of s (with an ellipsis prefix
// when truncated) — reasoning models put their conclusion at the end.
func summarizeTail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}

// positionSymbolsFromContext builds the open-position symbol set the stage
// derivation reads (hold: IN_POSITION vs NO_SETUP). Symbols are normalized
// so "NEAR" and "NEARUSDT" forms match.
func positionSymbolsFromContext(ctx *Context) map[string]bool {
	set := map[string]bool{}
	if ctx == nil {
		return set
	}
	for _, p := range ctx.Positions {
		if p.Symbol != "" {
			set[market.Normalize(p.Symbol)] = true
		}
	}
	return set
}

// removeCandidate drops one symbol from the cycle's candidate pool (in
// place) — used when the market-data pass skips a coin (low OI) so the
// persisted pool matches what the prompt actually renders (09-29 user
// report: IOTA/AZTEC appeared in the UI pool but not in the prompt).
func removeCandidate(ctx *Context, symbol string) {
	for i, c := range ctx.CandidateCoins {
		if market.Normalize(c.Symbol) == market.Normalize(symbol) {
			ctx.CandidateCoins = append(ctx.CandidateCoins[:i], ctx.CandidateCoins[i+1:]...)
			return
		}
	}
}

// snapshotCandidateCoins copies the candidate slice so callers that mutate
// the pool mid-loop (removeCandidate) can range it safely.
func snapshotCandidateCoins(ctx *Context) []CandidateCoin {
	out := make([]CandidateCoin, len(ctx.CandidateCoins))
	copy(out, ctx.CandidateCoins)
	return out
}
