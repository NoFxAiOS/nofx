package decision

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"math"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"regexp"
	"strings"
	"time"
)

// Pre-compiled regular expressions (performance optimization: avoid recompiling on each call)
var (
	// Safe regex: precisely match ```json code blocks
	// Use backtick + concatenation to avoid escape issues
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// XML tag extraction (supports any characters in reasoning chain)
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// PositionInfo position information
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // Historical peak profit percentage
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // Position update timestamp (milliseconds)
}

// AccountInfo account information
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // Account equity
	AvailableBalance float64 `json:"available_balance"` // Available balance
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized profit/loss
	TotalPnL         float64 `json:"total_pnl"`         // Total profit/loss
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total profit/loss percentage
	MarginUsed       float64 `json:"margin_used"`       // Used margin
	MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin usage rate
	PositionCount    int     `json:"position_count"`    // Number of positions
}

// CandidateCoin candidate coin (from coin pool)
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // Sources: "ai500" and/or "oi_top"
}

// OITopData open interest growth top data (for AI decision reference)
type OITopData struct {
	Rank              int     // OI Top ranking
	OIDeltaPercent    float64 // Open interest change percentage (1 hour)
	OIDeltaValue      float64 // Open interest change value
	PriceDeltaPercent float64 // Price change percentage
	NetLong           float64 // Net long positions
	NetShort          float64 // Net short positions
}

// TradingStats trading statistics (for AI input)
type TradingStats struct {
	TotalTrades    int     `json:"total_trades"`     // Total number of trades (closed)
	WinRate        float64 `json:"win_rate"`         // Win rate (%)
	ProfitFactor   float64 `json:"profit_factor"`    // Profit factor
	SharpeRatio    float64 `json:"sharpe_ratio"`     // Sharpe ratio
	TotalPnL       float64 `json:"total_pnl"`        // Total profit/loss
	AvgWin         float64 `json:"avg_win"`          // Average win
	AvgLoss        float64 `json:"avg_loss"`         // Average loss
	MaxDrawdownPct float64 `json:"max_drawdown_pct"` // Maximum drawdown (%)
}

// RecentOrder recently completed order (for AI input)
type RecentOrder struct {
	Symbol      string  `json:"symbol"`       // Trading pair
	Side        string  `json:"side"`         // long/short
	EntryPrice  float64 `json:"entry_price"`  // Entry price
	ExitPrice   float64 `json:"exit_price"`   // Exit price
	RealizedPnL float64 `json:"realized_pnl"` // Realized profit/loss
	PnLPct      float64 `json:"pnl_pct"`      // Profit/loss percentage
	FilledAt    string  `json:"filled_at"`    // Fill time
}

// Context trading context (complete information passed to AI)
type Context struct {
	CurrentTime     string                             `json:"current_time"`
	RuntimeMinutes  int                                `json:"runtime_minutes"`
	CallCount       int                                `json:"call_count"`
	Account         AccountInfo                        `json:"account"`
	Positions       []PositionInfo                     `json:"positions"`
	CandidateCoins  []CandidateCoin                    `json:"candidate_coins"`
	PromptVariant   string                             `json:"prompt_variant,omitempty"`
	TradingStats    *TradingStats                      `json:"trading_stats,omitempty"`  // Trading statistics
	RecentOrders    []RecentOrder                      `json:"recent_orders,omitempty"`  // Recently completed orders (10)
	MarketDataMap   map[string]*market.Data            `json:"-"`                        // Not serialized, but used internally
	MultiTFMarket   map[string]map[string]*market.Data `json:"-"`
	OITopDataMap    map[string]*OITopData              `json:"-"` // OI Top data mapping
	QuantDataMap    map[string]*QuantData              `json:"-"` // Quantitative data mapping (fund flow, position changes)
	BTCETHLeverage  int                                `json:"-"` // BTC/ETH leverage multiplier (read from config)
	AltcoinLeverage int                                `json:"-"` // Altcoin leverage multiplier (read from config)
}

// Decision AI trading decision
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "hold", "wait"

	// Opening position parameters
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// Common parameters
	Confidence int     `json:"confidence,omitempty"` // Confidence level (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // Maximum USD risk
	Reasoning  string  `json:"reasoning"`
}

// FullDecision AI's complete decision (including chain of thought)
type FullDecision struct {
	SystemPrompt string     `json:"system_prompt"` // System prompt (system prompt sent to AI)
	UserPrompt   string     `json:"user_prompt"`   // Input prompt sent to AI
	CoTTrace     string     `json:"cot_trace"`     // Chain of thought analysis (AI output)
	Decisions    []Decision `json:"decisions"`     // Specific decision list
	RawResponse  string     `json:"raw_response"`  // Raw AI response (for debugging when parsing fails)
	Timestamp    time.Time  `json:"timestamp"`
	// AIRequestDurationMs records AI API call duration (milliseconds) for troubleshooting latency issues
	AIRequestDurationMs int64 `json:"ai_request_duration_ms,omitempty"`
}

// GetFullDecision gets AI's complete trading decision (batch analysis of all coins and positions)
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
}

// GetFullDecisionWithStrategy uses StrategyEngine to get AI decision (new version: strategy-driven)
// Key: uses strategy-configured timeframes to fetch market data, consistent with api/strategy.go test run logic
func GetFullDecisionWithStrategy(ctx *Context, mcpClient mcp.AIClient, engine *StrategyEngine, variant string) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if engine == nil {
		// If no strategy engine, fallback to default behavior
		return GetFullDecisionWithCustomPrompt(ctx, mcpClient, "", false, "")
	}

	// 1. Fetch market data using strategy config (key: use multiple timeframes)
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataWithStrategy(ctx, engine); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	}

	// Ensure OITopDataMap is initialized
	if ctx.OITopDataMap == nil {
		ctx.OITopDataMap = make(map[string]*OITopData)
		// Load OI Top data
		oiPositions, err := pool.GetOITopPositions()
		if err == nil {
			for _, pos := range oiPositions {
				ctx.OITopDataMap[pos.Symbol] = &OITopData{
					Rank:              pos.Rank,
					OIDeltaPercent:    pos.OIDeltaPercent,
					OIDeltaValue:      pos.OIDeltaValue,
					PriceDeltaPercent: pos.PriceDeltaPercent,
					NetLong:           pos.NetLong,
					NetShort:          pos.NetShort,
				}
			}
		}
	}

	// 2. Build System Prompt using strategy engine
	riskConfig := engine.GetRiskControlConfig()
	systemPrompt := engine.BuildSystemPrompt(ctx.Account.TotalEquity, variant)

	// 3. Build User Prompt using strategy engine (including multi-timeframe data)
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
	)

	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt
		decision.UserPrompt = userPrompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
		decision.RawResponse = aiResponse // Save raw response for debugging
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

// fetchMarketDataWithStrategy fetches market data using strategy config (multiple timeframes)
// Fully implemented according to api/strategy.go handleStrategyTestRun logic
func fetchMarketDataWithStrategy(ctx *Context, engine *StrategyEngine) error {
	config := engine.GetConfig()
	ctx.MarketDataMap = make(map[string]*market.Data)

	// Get timeframe configuration (fully consistent with api/strategy.go logic)
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

	// 2. Fetch data for all candidate coins (fully consistent with api/strategy.go, no quantity limit)
	// Position coin set (used to determine whether to skip OI check)
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	// OI liquidity filter threshold (million USD)
	const minOIThresholdMillions = 15.0 // 15M USD minimum open interest value

	for _, coin := range ctx.CandidateCoins {
		// Skip already fetched position coins
		if _, exists := ctx.MarketDataMap[coin.Symbol]; exists {
			continue
		}

		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for %s: %v", coin.Symbol, err)
			continue
		}

		// Liquidity filter: skip coins with OI value below threshold (both long and short)
		// But existing positions must be retained (need to decide whether to close)
		isExistingPosition := positionSymbols[coin.Symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// Calculate OI value (USD) = OI quantity × current price
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // Convert to million USD
			if oiValueInMillions < minOIThresholdMillions {
				logger.Infof("⚠️  %s OI value too low (%.2fM USD < %.1fM), skipping coin [OI:%.0f × Price:%.4f]",
					coin.Symbol, oiValueInMillions, minOIThresholdMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[coin.Symbol] = data
	}

	logger.Infof("📊 Successfully fetched multi-timeframe market data for %d coins (low liquidity coins filtered)", len(ctx.MarketDataMap))
	return nil
}

// GetFullDecisionWithCustomPrompt gets AI's complete trading decision (supports custom prompt and template selection)
func GetFullDecisionWithCustomPrompt(ctx *Context, mcpClient mcp.AIClient, customPrompt string, overrideBase bool, templateName string) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	// 1. Fetch market data for all coins (if already provided by upper layer, no need to re-fetch)
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataForContext(ctx); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	} else if ctx.OITopDataMap == nil {
		// Ensure OI data mapping is initialized to avoid null pointer access later
		ctx.OITopDataMap = make(map[string]*OITopData)
	}

	// 2. Build System Prompt (fixed rules) and User Prompt (dynamic data)
	systemPrompt := buildSystemPromptWithCustom(
		ctx.Account.TotalEquity,
		ctx.BTCETHLeverage,
		ctx.AltcoinLeverage,
		customPrompt,
		overrideBase,
		templateName,
		ctx.PromptVariant,
	)
	userPrompt := buildUserPrompt(ctx)

	// 3. Call AI API (using system + user prompt)
	aiCallStart := time.Now()
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		return nil, fmt.Errorf("AI API call failed: %w", err)
	}

	// 4. Parse AI response
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage)

	// Save SystemPrompt and UserPrompt regardless of error (for debugging and troubleshooting unexecuted decisions)
	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt // Save system prompt
		decision.UserPrompt = userPrompt     // Save input prompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
		decision.RawResponse = aiResponse // Save raw response for debugging
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

// fetchMarketDataForContext fetches market data and OI data for all coins in context
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// Collect all symbols that need data
	symbolSet := make(map[string]bool)

	// 1. Prioritize fetching position coin data (this is required)
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. Candidate coin count dynamically adjusted based on account status
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// Fetch market data concurrently
	// Position coin set (used to determine whether to skip OI check)
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			// Single coin failure doesn't affect overall, just log error
			continue
		}

		// Liquidity filter: skip coins with OI value below threshold (both long and short)
		// OI value = OI quantity × current price
		// But existing positions must be retained (need to decide whether to close)
		// OI threshold configuration: users can adjust based on risk preference
		const minOIThresholdMillions = 15.0 // Adjustable: 15M(conservative) / 10M(balanced) / 8M(loose) / 5M(aggressive)

		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// Calculate OI value (USD) = OI quantity × current price
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // Convert to million USD
			if oiValueInMillions < minOIThresholdMillions {
				logger.Infof("⚠️  %s OI value too low (%.2fM USD < %.1fM), skipping coin [OI:%.0f × Price:%.4f]",
					symbol, oiValueInMillions, minOIThresholdMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// Load OI Top data (doesn't affect main flow)
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// Normalize symbol matching
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates calculates the number of candidate coins to analyze based on account status
func calculateMaxCandidates(ctx *Context) int {
	// Important: limit candidate coin count to avoid prompt being too large
	// Dynamically adjust based on position count: fewer positions allow analyzing more candidates
	const (
		maxCandidatesWhenEmpty    = 30 // Max 30 candidates when no positions
		maxCandidatesWhenHolding1 = 25 // Max 25 candidates when holding 1 position
		maxCandidatesWhenHolding2 = 20 // Max 20 candidates when holding 2 positions
		maxCandidatesWhenHolding3 = 15 // Max 15 candidates when holding 3 positions (avoid prompt being too large)
	)

	positionCount := len(ctx.Positions)
	var maxCandidates int

	switch positionCount {
	case 0:
		maxCandidates = maxCandidatesWhenEmpty
	case 1:
		maxCandidates = maxCandidatesWhenHolding1
	case 2:
		maxCandidates = maxCandidatesWhenHolding2
	default: // 3+ positions
		maxCandidates = maxCandidatesWhenHolding3
	}

	// Return the smaller value between actual candidate count and max limit
	return min(len(ctx.CandidateCoins), maxCandidates)
}

// buildSystemPromptWithCustom builds System Prompt with custom content
func buildSystemPromptWithCustom(accountEquity float64, btcEthLeverage, altcoinLeverage int, customPrompt string, overrideBase bool, templateName string, variant string) string {
	// If override base prompt and has custom prompt, only use custom prompt
	if overrideBase && customPrompt != "" {
		return customPrompt
	}

	// Get base prompt (using specified template)
	basePrompt := buildSystemPrompt(accountEquity, btcEthLeverage, altcoinLeverage, templateName, variant)

	// If no custom prompt, directly return base prompt
	if customPrompt == "" {
		return basePrompt
	}

	// Add custom prompt section to base prompt
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n")
	sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
	sb.WriteString(customPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("Note: The above personalized strategy is a supplement to basic rules and cannot violate basic risk control principles.\n")

	return sb.String()
}

// buildSystemPrompt builds System Prompt (using template + dynamic parts)
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, templateName string, variant string) string {
	var sb strings.Builder

	// 1. Load prompt template (core trading strategy part)
	if templateName == "" {
		templateName = "default" // Default to using default template
	}

	template, err := GetPromptTemplate(templateName)
	if err != nil {
		// If template doesn't exist, log error and use default
		logger.Infof("⚠️  Prompt template '%s' doesn't exist, using default: %v", templateName, err)
		template, err = GetPromptTemplate("default")
		if err != nil {
			// If even default doesn't exist, use built-in simplified version
			logger.Infof("❌ Cannot load any prompt template, using built-in simplified version")
			sb.WriteString("You are a professional cryptocurrency trading AI. Please make trading decisions based on market data.\n\n")
		} else {
			sb.WriteString(template.Content)
			sb.WriteString("\n\n")
		}
	} else {
		sb.WriteString(template.Content)
		sb.WriteString("\n\n")
	}

	// 2. Trading mode variants
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥70\n- Allow higher positions, but must strictly set stop loss and explain profit/loss ratio\n\n")
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open positions when multiple signals resonate\n- Prioritize holding cash, must pause for multiple periods after consecutive losses\n\n")
	case "scalping":
		sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, target smaller profits but require swift action\n- If price doesn't move as expected within two bars, immediately reduce position or stop loss\n\n")
	}

	// 3. Hard constraints (risk control)
	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("1. Risk/reward ratio: Must be ≥ 1:3 (risk 1% to earn 3%+ profit)\n")
	sb.WriteString("2. Max positions: 3 coins (quality > quantity)\n")
	sb.WriteString(fmt.Sprintf("3. Single coin position: Altcoins %.0f-%.0f U | BTC/ETH %.0f-%.0f U\n",
		accountEquity*0.8, accountEquity*1.5, accountEquity*5, accountEquity*10))
	sb.WriteString(fmt.Sprintf("4. Leverage limit: **Altcoins max %dx leverage** | **BTC/ETH max %dx leverage**\n", altcoinLeverage, btcEthLeverage))
	sb.WriteString("5. Margin usage rate ≤ 90%%\n")
	sb.WriteString("6. Opening amount: Recommended ≥12 USDT (exchange minimum notional value 10 USDT + safety margin)\n\n")

	// 4. Trading frequency and signal quality
	sb.WriteString("# ⏱️ Trading Frequency Awareness\n\n")
	sb.WriteString("- Excellent traders: 2-4 trades/day ≈ 0.1-0.2 trades/hour\n")
	sb.WriteString("- >2 trades/hour = overtrading\n")
	sb.WriteString("- Single position holding time ≥30-60 minutes\n")
	sb.WriteString("If you find yourself trading every period → standards too low; if closing position <30 minutes → too impatient.\n\n")

	sb.WriteString("# 🎯 Opening Standards (Strict)\n\n")
	sb.WriteString("Only open positions when multiple signals resonate. You have:\n")
	sb.WriteString("- 3-minute price series + 4-hour K-line series\n")
	sb.WriteString("- EMA20 / MACD / RSI7 / RSI14 indicator series\n")
	sb.WriteString("- Volume, open interest (OI), funding rate and other fund flow series\n")
	sb.WriteString("- AI500 / OI_Top screening tags (if any)\n\n")
	sb.WriteString("Freely use any effective analysis method, but **confidence ≥75** required to open positions; avoid low-quality behaviors such as single indicators, contradictory signals, sideways consolidation, reopening immediately after closing, etc.\n\n")

	// 5. Decision process tips
	sb.WriteString("# 📋 Decision Process\n\n")
	sb.WriteString("1. Check positions → Should take profit/stop loss?\n")
	sb.WriteString("2. Scan candidate coins + multi-timeframe → Any strong signals?\n")
	sb.WriteString("3. Write reasoning chain first, then output structured JSON\n\n")

	// 7. Output format - dynamically generated
	sb.WriteString("# Output Format (Strictly Follow)\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate reasoning chain and decision JSON, avoid parsing errors**\n\n")
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Your reasoning chain analysis...\n")
	sb.WriteString("- Concisely analyze your thought process \n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("Step 2: JSON decision array\n\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300},\n", btcEthLeverage, accountEquity*5))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\"}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Field Descriptions\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("- `confidence`: 0-100 (opening recommended ≥75)\n")
	sb.WriteString("- Required for opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n")
	sb.WriteString("- **IMPORTANT**: All numeric values must be calculated numbers, NOT formulas/expressions (e.g., use `27.76` not `3000 * 0.01`)\n\n")

	return sb.String()
}

// buildUserPrompt builds User Prompt (dynamic data)
func buildUserPrompt(ctx *Context) string {
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

	// Account
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | PnL %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// Positions (complete market data)
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			// Calculate holding duration
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // Convert to minutes
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | Holding %d mins", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | Holding %dh %dm", durationHour, durationMinRemainder)
				}
			}

			// Calculate position value
			positionValue := math.Abs(pos.Quantity) * pos.MarkPrice

			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Qty %.4f | Value %.2f USDT | PnL %+.2f%% | PnL Amount %+.2f USDT | Peak PnL %.2f%% | Leverage %dx | Margin %.0f | Liq %.4f%s\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

			// Use FormatMarketData to output complete market data
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("Current Positions: None\n\n")
	}

	// Trading statistics (if any)
	if ctx.TradingStats != nil && ctx.TradingStats.TotalTrades > 0 {
		sb.WriteString("## Historical Trading Statistics\n")
		sb.WriteString(fmt.Sprintf("Total Trades: %d | Win Rate: %.1f%% | Profit Factor: %.2f | Sharpe Ratio: %.2f\n",
			ctx.TradingStats.TotalTrades,
			ctx.TradingStats.WinRate,
			ctx.TradingStats.ProfitFactor,
			ctx.TradingStats.SharpeRatio))
		sb.WriteString(fmt.Sprintf("Total PnL: %.2f USDT | Avg Win: %.2f | Avg Loss: %.2f | Max Drawdown: %.1f%%\n\n",
			ctx.TradingStats.TotalPnL,
			ctx.TradingStats.AvgWin,
			ctx.TradingStats.AvgLoss,
			ctx.TradingStats.MaxDrawdownPct))
	}

	// Recently completed orders (if any)
	if len(ctx.RecentOrders) > 0 {
		sb.WriteString("## Recently Completed Trades\n")
		for i, order := range ctx.RecentOrders {
			resultStr := "Profit"
			if order.RealizedPnL < 0 {
				resultStr = "Loss"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | %s: %+.2f USDT (%+.2f%%) | %s\n",
				i+1, order.Symbol, order.Side,
				order.EntryPrice, order.ExitPrice,
				resultStr, order.RealizedPnL, order.PnLPct,
				order.FilledAt))
		}
		sb.WriteString("\n")
	}

	// Candidate coins (complete market data)
	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d coins)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top dual signal)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top growing)"
		}

		// Use FormatMarketData to output complete market data
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("---\n\n")
	sb.WriteString("Now please analyze and output decision (reasoning chain + JSON)\n")

	return sb.String()
}

// parseFullDecisionResponse parses AI's complete decision response
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int) (*FullDecision, error) {
	// 1. Extract chain of thought
	cotTrace := extractCoTTrace(aiResponse)

	// 2. Extract JSON decision list
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	// 3. Validate decisions
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", err)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// extractCoTTrace extracts chain of thought analysis
func extractCoTTrace(response string) string {
	// Method 1: Prioritize extracting <reasoning> tag content
	if match := reReasoningTag.FindStringSubmatch(response); match != nil && len(match) > 1 {
		logger.Infof("✓ Extracted reasoning chain using <reasoning> tag")
		return strings.TrimSpace(match[1])
	}

	// Method 2: If no <reasoning> tag but has <decision> tag, extract content before <decision>
	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		logger.Infof("✓ Extracted content before <decision> tag as reasoning chain")
		return strings.TrimSpace(response[:decisionIdx])
	}

	// Method 3: Fallback - find start position of JSON array
	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		logger.Infof("⚠️  Extracted reasoning chain using old format ([ character separator)")
		return strings.TrimSpace(response[:jsonStart])
	}

	// If no markers found, entire response is reasoning chain
	return strings.TrimSpace(response)
}

// extractDecisions extracts JSON decision list
func extractDecisions(response string) ([]Decision, error) {
	// Pre-clean: remove zero-width/BOM
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)

	// Critical Fix: fix full-width characters before regex matching!
	// Otherwise regex \[ cannot match full-width ［
	s = fixMissingQuotes(s)

	// Method 1: Prioritize extracting from <decision> tag
	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); match != nil && len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		logger.Infof("✓ Extracted JSON using <decision> tag")
	} else {
		// Fallback: use entire response
		jsonPart = s
		logger.Infof("⚠️  <decision> tag not found, searching JSON in full text")
	}

	// Fix full-width characters in jsonPart
	jsonPart = fixMissingQuotes(jsonPart)

	// 1) Prioritize extracting from ```json code block
	if m := reJSONFence.FindStringSubmatch(jsonPart); m != nil && len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent) // Normalize "[ {" to "[{"
		jsonContent = fixMissingQuotes(jsonContent) // Second fix (prevent residual full-width after regex extraction)
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
		}
		return decisions, nil
	}

	// 2) Fallback: search for first object array in full text
	// Note: at this point jsonPart has already been processed by fixMissingQuotes(), full-width converted to half-width
	jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
	if jsonContent == "" {
		// Safe Fallback: when AI only outputs reasoning without JSON, generate fallback decision (avoid system crash)
		logger.Infof("⚠️  [SafeFallback] AI didn't output JSON decision, entering safe wait mode")

		// Extract reasoning summary (max 240 characters)
		cotSummary := jsonPart
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		// Generate fallback decision: all coins enter wait state
		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("Model didn't output structured JSON decision, entering safe wait; summary: %s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	// Normalize format (full-width characters already fixed earlier)
	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent) // Second fix (prevent residual full-width after regex extraction)

	// Validate JSON format (detect common errors)
	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
	}

	// Parse JSON
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
	}

	return decisions, nil
}

// fixMissingQuotes replaces Chinese quotes and full-width characters with English quotes and half-width characters (avoid parsing failure due to AI outputting full-width JSON characters)
func fixMissingQuotes(jsonStr string) string {
	// Replace Chinese quotes
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '

	// Replace full-width brackets, colons, commas (prevent AI outputting full-width JSON characters)
	jsonStr = strings.ReplaceAll(jsonStr, "［", "[") // U+FF3B full-width left square bracket
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]") // U+FF3D full-width right square bracket
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{") // U+FF5B full-width left curly bracket
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}") // U+FF5D full-width right curly bracket
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":") // U+FF1A full-width colon
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",") // U+FF0C full-width comma

	// Replace CJK punctuation (AI may also output these in Chinese context)
	jsonStr = strings.ReplaceAll(jsonStr, "【", "[") // CJK left corner bracket U+3010
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]") // CJK right corner bracket U+3011
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[") // CJK left tortoise shell bracket U+3014
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]") // CJK right tortoise shell bracket U+3015
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",") // CJK ideographic comma U+3001

	// Replace full-width space with half-width space (JSON shouldn't have full-width spaces)
	jsonStr = strings.ReplaceAll(jsonStr, "　", " ") // U+3000 full-width space

	return jsonStr
}

// validateJSONFormat validates JSON format, detecting common errors
func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	// Allow any whitespace (including zero-width) between [ and {
	if !reArrayHead.MatchString(trimmed) {
		// Check if it's a pure number/range array (common error)
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	// Check if contains range symbol ~ (common LLM error)
	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be precise single values")
	}

	// Check if contains thousand separators (like 98,000)
	// Use simple pattern matching: digit + comma + 3 digits
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

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeInvisibleRunes removes zero-width characters and BOM, avoiding invisible prefixes breaking validation
func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

// compactArrayOpen normalizes opening "[ {" → "[{"
func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// validateDecisions validates all decisions (requires account information and leverage configuration)
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
			return fmt.Errorf("decision #%d validation failed: %w", i+1, err)
		}
	}
	return nil
}

// findMatchingBracket finds matching right bracket
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// validateDecision validates the validity of a single decision
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	// Validate action
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("invalid action: %s", d.Action)
	}

	// Opening operations must provide complete parameters
	if d.Action == "open_long" || d.Action == "open_short" {
		// Use configured leverage limit based on coin type
		maxLeverage := altcoinLeverage          // Altcoins use configured leverage
		maxPositionValue := accountEquity * 1.5 // Altcoins max 1.5x account equity
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC and ETH use configured leverage
			maxPositionValue = accountEquity * 10 // BTC/ETH max 10x account equity
		}

		// Fallback mechanism: auto-correct leverage to limit when exceeded (instead of directly rejecting decision)
		if d.Leverage <= 0 {
			return fmt.Errorf("leverage must be greater than 0: %d", d.Leverage)
		}
		if d.Leverage > maxLeverage {
			logger.Infof("⚠️  [Leverage Fallback] %s leverage exceeded (%dx > %dx), auto-adjusting to limit %dx",
				d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage // Auto-correct to limit value
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("position size must be greater than 0: %.2f", d.PositionSizeUSD)
		}

		// Validate minimum opening amount (prevent quantity rounding to 0 error)
		// Binance minimum notional value 10 USDT + safety margin
		const minPositionSizeGeneral = 12.0 // 10 + 20% safety margin
		const minPositionSizeBTCETH = 60.0  // BTC/ETH requires larger amount due to high price and precision limits (more flexible)

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT (due to high price and precision limits, avoid quantity rounding to 0)", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT (Binance minimum notional value requirement)", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		// Validate position value limit (add 1% tolerance to avoid floating point precision issues)
		tolerance := maxPositionValue * 0.01 // 1% tolerance
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH single coin position value cannot exceed %.0f USDT (10x account equity), actual: %.0f", maxPositionValue, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("altcoin single coin position value cannot exceed %.0f USDT (1.5x account equity), actual: %.0f", maxPositionValue, d.PositionSizeUSD)
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("stop loss and take profit must be greater than 0")
		}

		// Validate rationality of stop loss and take profit
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("for long positions, stop loss price must be less than take profit price")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("for short positions, stop loss price must be greater than take profit price")
			}
		}

		// Validate risk/reward ratio (must be ≥1:3)
		// Calculate entry price (assume current market price)
		var entryPrice float64
		if d.Action == "open_long" {
			// Long: entry price between stop loss and take profit
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2 // Assume entry at 20% position
		} else {
			// Short: entry price between stop loss and take profit
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2 // Assume entry at 20% position
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		// Hard constraint: risk/reward ratio must be ≥3.0
		if riskRewardRatio < 3.0 {
			return fmt.Errorf("risk/reward ratio too low (%.2f:1), must be ≥3.0:1 [risk: %.2f%% reward: %.2f%%] [stop loss: %.2f take profit: %.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	return nil
}
