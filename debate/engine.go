package debate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
)

// TraderExecutor interface for executing trades
type TraderExecutor interface {
	ExecuteDecision(decision *kernel.Decision) error
	GetBalance() (map[string]interface{}, error)
}

// DebateEngine orchestrates AI debates using strategy-based market context
type DebateEngine struct {
	debateStore   *store.DebateStore
	strategyStore *store.StrategyStore
	aiModelStore  *store.AIModelStore
	clients       map[string]mcp.AIClient
	clientsMu     sync.RWMutex

	// Event callbacks for SSE streaming
	OnRoundStart func(sessionID string, round int)
	OnMessage    func(sessionID string, msg *store.DebateMessage)
	OnRoundEnd   func(sessionID string, round int)
	OnVote       func(sessionID string, vote *store.DebateVote)
	OnConsensus  func(sessionID string, decision *store.DebateDecision)
	OnError      func(sessionID string, err error)
}

// NewDebateEngine creates a new debate engine
func NewDebateEngine(debateStore *store.DebateStore, strategyStore *store.StrategyStore, aiModelStore *store.AIModelStore) *DebateEngine {
	engine := &DebateEngine{
		debateStore:   debateStore,
		strategyStore: strategyStore,
		aiModelStore:  aiModelStore,
		clients:       make(map[string]mcp.AIClient),
	}

	// Cleanup stale running/voting debates on startup
	engine.cleanupStaleDebates()

	return engine
}

// cleanupStaleDebates marks any running/voting debates as cancelled on startup
func (e *DebateEngine) cleanupStaleDebates() {
	sessions, err := e.debateStore.ListAllSessions()
	if err != nil {
		logger.Warnf("[Debate] Failed to list sessions for cleanup: %v", err)
		return
	}

	for _, session := range sessions {
		if session.Status == store.DebateStatusRunning || session.Status == store.DebateStatusVoting {
			logger.Infof("[Debate] Cancelling stale debate: %s (was %s)", session.ID, session.Status)
			e.debateStore.UpdateSessionStatus(session.ID, store.DebateStatusCancelled)
		}
	}
}

// InitializeClients initializes AI clients for all participants
func (e *DebateEngine) InitializeClients(participants []*store.DebateParticipant) error {
	e.clientsMu.Lock()
	defer e.clientsMu.Unlock()

	for _, p := range participants {
		aiModel, err := e.aiModelStore.GetByID(p.AIModelID)
		if err != nil {
			return fmt.Errorf("failed to get AI model %s: %w", p.AIModelID, err)
		}

		var client mcp.AIClient
		switch aiModel.Provider {
		case "deepseek":
			client = mcp.NewDeepSeekClient()
		case "qwen":
			client = mcp.NewQwenClient()
		case "openai":
			client = mcp.NewOpenAIClient()
		case "claude":
			client = mcp.NewClaudeClient()
		case "gemini":
			client = mcp.NewGeminiClient()
		case "grok":
			client = mcp.NewGrokClient()
		case "kimi":
			client = mcp.NewKimiClient()
		default:
			client = mcp.New()
		}

		// Configure client (convert EncryptedString to string)
		client.SetAPIKey(string(aiModel.APIKey), aiModel.CustomAPIURL, aiModel.CustomModelName)

		e.clients[p.AIModelID] = client
	}

	return nil
}

// StartDebate starts a debate session with strategy-based market data
func (e *DebateEngine) StartDebate(sessionID string) error {
	// Get session with details
	session, err := e.debateStore.GetSessionWithDetails(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session.Status != store.DebateStatusPending {
		return fmt.Errorf("debate is not in pending status")
	}

	if len(session.Participants) < 2 {
		return fmt.Errorf("need at least 2 participants")
	}

	// Initialize AI clients
	if err := e.InitializeClients(session.Participants); err != nil {
		return fmt.Errorf("failed to initialize clients: %w", err)
	}

	// Get strategy config
	strategy, err := e.strategyStore.Get(session.UserID, session.StrategyID)
	if err != nil {
		return fmt.Errorf("failed to get strategy: %w", err)
	}

	strategyConfig, err := strategy.ParseConfig()
	if err != nil {
		return fmt.Errorf("failed to parse strategy config: %w", err)
	}

	// Update status to running
	if err := e.debateStore.UpdateSessionStatus(sessionID, store.DebateStatusRunning); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Run debate asynchronously
	go e.runDebate(session, strategyConfig)

	return nil
}

// runDebate runs the actual debate rounds
func (e *DebateEngine) runDebate(session *store.DebateSessionWithDetails, strategyConfig *store.StrategyConfig) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Debate panic recovered: %v", r)
			e.debateStore.UpdateSessionStatus(session.ID, store.DebateStatusCancelled)
			if e.OnError != nil {
				e.OnError(session.ID, fmt.Errorf("debate panic: %v", r))
			}
		}
	}()

	// Create strategy engine for building context
	strategyEngine := kernel.NewStrategyEngine(strategyConfig)

	// Build market context using strategy config
	ctx, err := e.buildMarketContext(session, strategyEngine)
	if err != nil {
		logger.Errorf("Failed to build market context: %v", err)
		e.debateStore.UpdateSessionStatus(session.ID, store.DebateStatusCancelled)
		if e.OnError != nil {
			e.OnError(session.ID, err)
		}
		return
	}

	// Build system prompt based on strategy (same as AI Test)
	baseSystemPrompt := strategyEngine.BuildSystemPrompt(1000.0, session.PromptVariant)

	// Build user prompt with market data (OI ranking data is included via ctx.OIRankingData)
	userPrompt := strategyEngine.BuildUserPrompt(ctx)

	// Run debate rounds
	var allMessages []*store.DebateMessage
	for round := 1; round <= session.MaxRounds; round++ {
		logger.Infof("Starting debate round %d/%d for session %s", round, session.MaxRounds, session.ID)

		if e.OnRoundStart != nil {
			e.OnRoundStart(session.ID, round)
		}

		e.debateStore.UpdateSessionRound(session.ID, round)

		// Get response from each participant
		for i, participant := range session.Participants {
			logger.Infof("[Debate] Round %d - Getting response from participant %d/%d: %s (%s)",
				round, i+1, len(session.Participants), participant.AIModelName, participant.Provider)

			// Build personality-enhanced system prompt
			systemPrompt := e.buildDebateSystemPrompt(baseSystemPrompt, participant, round, session.MaxRounds)

			// Build debate user prompt with previous messages
			debateUserPrompt := e.buildDebateUserPrompt(userPrompt, allMessages, participant, round)

			// Get AI response
			msg, err := e.getParticipantResponse(session, participant, systemPrompt, debateUserPrompt, round)
			if err != nil {
				logger.Errorf("[Debate] Failed to get response from %s (%s): %v", participant.AIModelName, participant.Provider, err)
				// Send error event to frontend
				if e.OnError != nil {
					e.OnError(session.ID, fmt.Errorf("%s failed: %v", participant.AIModelName, err))
				}
				continue
			}

			logger.Infof("[Debate] Got response from %s: %d chars, action=%s, confidence=%d%%",
				participant.AIModelName, len(msg.Content), msg.Decision.Action, msg.Confidence)

			// Save message
			if err := e.debateStore.AddMessage(msg); err != nil {
				logger.Errorf("Failed to save message: %v", err)
			}

			allMessages = append(allMessages, msg)

			if e.OnMessage != nil {
				e.OnMessage(session.ID, msg)
			}
		}

		if e.OnRoundEnd != nil {
			e.OnRoundEnd(session.ID, round)
		}
	}

	// Voting phase
	logger.Infof("Starting voting phase for session %s", session.ID)
	e.debateStore.UpdateSessionStatus(session.ID, store.DebateStatusVoting)

	votes, err := e.collectVotes(session, strategyEngine, allMessages)
	if err != nil {
		logger.Errorf("Failed to collect votes: %v", err)
	}

	// Determine multi-coin consensus
	allDecisions := e.determineMultiCoinConsensus(votes)

	// For backward compatibility, also set single consensus
	var primaryConsensus *store.DebateDecision
	if len(allDecisions) > 0 {
		primaryConsensus = allDecisions[0]
		// If session has specific symbol, find that decision
		if session.Symbol != "" {
			for _, d := range allDecisions {
				if d.Symbol == session.Symbol {
					primaryConsensus = d
					break
				}
			}
		}
	} else {
		primaryConsensus = &store.DebateDecision{
			Action:     "hold",
			Symbol:     session.Symbol,
			Confidence: 0,
			Reasoning:  "No actionable consensus reached",
		}
	}

	// Store both single and multi-coin decisions
	session.FinalDecision = primaryConsensus
	session.FinalDecisions = allDecisions

	// Update session with final decisions
	e.debateStore.UpdateSessionFinalDecisions(session.ID, primaryConsensus, allDecisions)
	e.debateStore.UpdateSessionStatus(session.ID, store.DebateStatusCompleted)

	if e.OnConsensus != nil {
		e.OnConsensus(session.ID, primaryConsensus)
	}

	logger.Infof("Debate %s completed. %d consensus decisions, primary: %s %s (confidence: %d%%)",
		session.ID, len(allDecisions), primaryConsensus.Action, primaryConsensus.Symbol, primaryConsensus.Confidence)
}

// buildMarketContext builds the market context using strategy engine
func (e *DebateEngine) buildMarketContext(session *store.DebateSessionWithDetails, strategyEngine *kernel.StrategyEngine) (*kernel.Context, error) {
	config := strategyEngine.GetConfig()

	// Get candidate coins
	candidates, err := strategyEngine.GetCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("failed to get candidates: %w", err)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidate coins found")
	}

	// Get timeframe settings
	timeframes := config.Indicators.Klines.SelectedTimeframes
	primaryTimeframe := config.Indicators.Klines.PrimaryTimeframe
	klineCount := config.Indicators.Klines.PrimaryCount
	if klineCount <= 0 {
		klineCount = 50
	}

	// Fetch market data for each candidate
	marketDataMap := make(map[string]*market.Data)
	for _, coin := range candidates {
		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logger.Warnf("Failed to get market data for %s: %v", coin.Symbol, err)
			continue
		}
		marketDataMap[coin.Symbol] = data
	}

	if len(marketDataMap) == 0 {
		return nil, fmt.Errorf("failed to fetch market data for any candidate")
	}

	// Fetch quantitative data (using strategy engine's built-in logic)
	symbols := make([]string, 0, len(candidates))
	for _, c := range candidates {
		symbols = append(symbols, c.Symbol)
	}
	quantDataMap := strategyEngine.FetchQuantDataBatch(symbols)

	// Fetch OI ranking data (market-wide position changes)
	oiRankingData := strategyEngine.FetchOIRankingData()

	// Fetch NetFlow ranking data (market-wide fund flow)
	netFlowRankingData := strategyEngine.FetchNetFlowRankingData()

	// Fetch Price ranking data (market-wide gainers/losers)
	priceRankingData := strategyEngine.FetchPriceRankingData()

	// Build context
	ctx := &kernel.Context{
		CurrentTime:    time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes: 0,
		CallCount:      1,
		Account: kernel.AccountInfo{
			TotalEquity:      1000.0, // Simulated for debate
			AvailableBalance: 1000.0,
			UnrealizedPnL:    0,
			TotalPnL:         0,
			TotalPnLPct:      0,
			MarginUsed:       0,
			MarginUsedPct:    0,
			PositionCount:    0,
		},
		Positions:          []kernel.PositionInfo{},
		CandidateCoins:     candidates,
		PromptVariant:      session.PromptVariant,
		MarketDataMap:      marketDataMap,
		QuantDataMap:       quantDataMap,
		OIRankingData:      oiRankingData,
		NetFlowRankingData: netFlowRankingData,
		PriceRankingData:   priceRankingData,
	}

	return ctx, nil
}

// buildDebateSystemPrompt enhances the base strategy prompt with debate-specific instructions
func (e *DebateEngine) buildDebateSystemPrompt(basePrompt string, participant *store.DebateParticipant, round, maxRounds int) string {
	personality := getPersonalityDescription(participant.Personality)
	emoji := store.PersonalityEmojis[participant.Personality]

	debateInstructions := fmt.Sprintf(`
## DEBATE MODE - ROUND %d/%d

You are participating in a multi-AI market debate as %s %s.

### Your Debate Role:
%s

### Debate Rules:
1. Analyze ALL candidate coins provided in the market data
2. Support your arguments with specific data points and indicators
3. If this is round 2 or later, respond to other participants' arguments
4. Be persuasive but data-driven
5. Your personality should influence your analysis bias but not override data
6. You can recommend multiple coins with different actions

### CRITICAL: Output Format (MUST follow exactly)

First write your analysis:
<reasoning>
- Your market analysis for each coin with specific data references
- Your main trading thesis and arguments
- Response to other participants (if round > 1)
</reasoning>

Then output your decisions in STRICT JSON ARRAY format (can include multiple coins):
<decision>
[
  {"symbol": "BTCUSDT", "action": "open_long", "confidence": 75, "leverage": 5, "position_pct": 0.3, "stop_loss": 0.02, "take_profit": 0.04, "reasoning": "BTC showing strength"},
  {"symbol": "ETHUSDT", "action": "open_short", "confidence": 80, "leverage": 3, "position_pct": 0.2, "stop_loss": 0.03, "take_profit": 0.06, "reasoning": "ETH bearish divergence"},
  {"symbol": "SOLUSDT", "action": "wait", "confidence": 60, "reasoning": "SOL needs more confirmation"}
]
</decision>

### IMPORTANT: action field MUST be exactly one of:
- "open_long" (做多/买入)
- "open_short" (做空/卖出)
- "close_long" (平多仓)
- "close_short" (平空仓)
- "hold" (持仓观望)
- "wait" (空仓等待)

### Field Requirements for each coin:
- symbol: REQUIRED, the trading pair
- action: REQUIRED, exactly one of the above values
- confidence: REQUIRED, integer 0-100
- leverage: REQUIRED for open_long/open_short, integer 1-20
- position_pct: REQUIRED for open_long/open_short, float 0.1-1.0
- stop_loss: REQUIRED for open_long/open_short, float 0.01-0.10 (percentage as decimal)
- take_profit: REQUIRED for open_long/open_short, float 0.02-0.20 (percentage as decimal)
- reasoning: REQUIRED, one sentence summary

---

`, round, maxRounds, emoji, participant.Personality, personality)

	return debateInstructions + basePrompt
}

// buildDebateUserPrompt adds debate context to the user prompt
func (e *DebateEngine) buildDebateUserPrompt(baseUserPrompt string, previousMessages []*store.DebateMessage, currentParticipant *store.DebateParticipant, round int) string {
	var sb strings.Builder

	// Add previous debate messages if any
	if len(previousMessages) > 0 && round > 1 {
		sb.WriteString("## Previous Debate Arguments\n\n")
		for _, msg := range previousMessages {
			emoji := store.PersonalityEmojis[msg.Personality]
			sb.WriteString(fmt.Sprintf("### %s %s (%s) - Round %d:\n", emoji, msg.AIModelName, msg.Personality, msg.Round))
			// Extract key points from previous messages
			if msg.Decision != nil {
				sb.WriteString(fmt.Sprintf("**Position:** %s (Confidence: %d%%)\n", msg.Decision.Action, msg.Decision.Confidence))
			}
			// Include a summary of their argument
			if len(msg.Content) > 500 {
				sb.WriteString(msg.Content[:500] + "...\n\n")
			} else {
				sb.WriteString(msg.Content + "\n\n")
			}
		}
		sb.WriteString("---\n\n")
	}

	sb.WriteString("## Current Market Data\n\n")
	sb.WriteString(baseUserPrompt)

	return sb.String()
}

// getParticipantResponse gets a response from a participant with timeout
func (e *DebateEngine) getParticipantResponse(
	session *store.DebateSessionWithDetails,
	participant *store.DebateParticipant,
	systemPrompt, userPrompt string,
	round int,
) (*store.DebateMessage, error) {
	e.clientsMu.RLock()
	client, ok := e.clients[participant.AIModelID]
	e.clientsMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("client not found for %s", participant.AIModelID)
	}

	// Use channel-based timeout (60 seconds per AI call)
	type result struct {
		response string
		err      error
	}
	resultCh := make(chan result, 1)

	go func() {
		resp, err := client.CallWithMessages(systemPrompt, userPrompt)
		resultCh <- result{response: resp, err: err}
	}()

	var response string
	var err error
	select {
	case res := <-resultCh:
		response = res.response
		err = res.err
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("AI call timeout after 60s for %s", participant.AIModelName)
	}

	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	// Parse multiple decisions from response
	decisions, confidence := parseDecisions(response)

	// Validate and fix symbols - if session has a specific symbol, force all decisions to use it
	if session.Symbol != "" {
		for _, d := range decisions {
			if d.Symbol == "" || d.Symbol != session.Symbol {
				logger.Warnf("[Debate] Fixing invalid symbol in message '%s' -> '%s'", d.Symbol, session.Symbol)
				d.Symbol = session.Symbol
			}
		}
	}

	// For backward compatibility, set Decision to first decision
	var primaryDecision *store.DebateDecision
	if len(decisions) > 0 {
		primaryDecision = decisions[0]
	}

	// Determine message type based on round
	messageType := "analysis"
	if round > 1 {
		messageType = "rebuttal"
	}

	msg := &store.DebateMessage{
		SessionID:   session.ID,
		Round:       round,
		AIModelID:   participant.AIModelID,
		AIModelName: participant.AIModelName,
		Provider:    participant.Provider,
		Personality: participant.Personality,
		MessageType: messageType,
		Content:     response,
		Decision:    primaryDecision,
		Decisions:   decisions,
		Confidence:  confidence,
	}

	return msg, nil
}

// collectVotes collects final votes from all participants
func (e *DebateEngine) collectVotes(session *store.DebateSessionWithDetails, strategyEngine *kernel.StrategyEngine, allMessages []*store.DebateMessage) ([]*store.DebateVote, error) {
	var votes []*store.DebateVote

	// Build voting context
	baseSystemPrompt := strategyEngine.BuildSystemPrompt(1000.0, session.PromptVariant)

	for _, participant := range session.Participants {
		vote, err := e.getParticipantVote(session, participant, baseSystemPrompt, allMessages)
		if err != nil {
			logger.Errorf("Failed to get vote from %s: %v", participant.AIModelName, err)
			continue
		}

		if err := e.debateStore.AddVote(vote); err != nil {
			logger.Errorf("Failed to save vote: %v", err)
		}

		votes = append(votes, vote)

		if e.OnVote != nil {
			e.OnVote(session.ID, vote)
		}
	}

	return votes, nil
}

// getParticipantVote gets a final vote from a participant (supports multi-coin)
func (e *DebateEngine) getParticipantVote(
	session *store.DebateSessionWithDetails,
	participant *store.DebateParticipant,
	baseSystemPrompt string,
	allMessages []*store.DebateMessage,
) (*store.DebateVote, error) {
	e.clientsMu.RLock()
	client, ok := e.clients[participant.AIModelID]
	e.clientsMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("client not found for %s", participant.AIModelID)
	}

	systemPrompt := e.buildVotingSystemPrompt(baseSystemPrompt, participant)
	userPrompt := e.buildVotingUserPrompt(allMessages)

	response, err := client.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI call failed: %w", err)
	}

	// Parse multi-coin votes
	decisions, avgConfidence := parseDecisions(response)

	// Validate and fix symbols - if session has a specific symbol, force all decisions to use it
	// This prevents AI from hallucinating random symbols not in the candidate list
	if session.Symbol != "" {
		for _, d := range decisions {
			if d.Symbol == "" || d.Symbol != session.Symbol {
				logger.Warnf("[Debate] Fixing invalid symbol '%s' -> '%s'", d.Symbol, session.Symbol)
				d.Symbol = session.Symbol
			}
		}
	}

	// Find primary decision (for backward compatibility)
	var primaryDecision *store.DebateDecision
	if len(decisions) > 0 {
		primaryDecision = decisions[0]
	}

	// If no valid decisions, create a default one with session symbol
	if primaryDecision == nil && session.Symbol != "" {
		primaryDecision = &store.DebateDecision{
			Action:     "hold",
			Symbol:     session.Symbol,
			Confidence: 50,
			Leverage:   5,
			PositionPct: 0.2,
		}
		decisions = []*store.DebateDecision{primaryDecision}
	}

	vote := &store.DebateVote{
		SessionID:   session.ID,
		AIModelID:   participant.AIModelID,
		AIModelName: participant.AIModelName,
		Decisions:   decisions,
		Confidence:  avgConfidence,
	}

	// Set backward-compatible fields from primary decision
	if primaryDecision != nil {
		vote.Action = primaryDecision.Action
		vote.Symbol = primaryDecision.Symbol
		vote.Leverage = primaryDecision.Leverage
		vote.PositionPct = primaryDecision.PositionPct
		vote.StopLossPct = primaryDecision.StopLoss
		vote.TakeProfitPct = primaryDecision.TakeProfit
		vote.Reasoning = primaryDecision.Reasoning
		vote.Confidence = primaryDecision.Confidence
	}

	logger.Infof("[Debate] Vote from %s: %d decisions", participant.AIModelName, len(decisions))
	for _, d := range decisions {
		logger.Infof("[Debate]   - %s: %s (confidence: %d%%)", d.Symbol, d.Action, d.Confidence)
	}

	return vote, nil
}

// buildVotingSystemPrompt builds the system prompt for voting
func (e *DebateEngine) buildVotingSystemPrompt(basePrompt string, participant *store.DebateParticipant) string {
	personality := getPersonalityDescription(participant.Personality)
	emoji := store.PersonalityEmojis[participant.Personality]

	return fmt.Sprintf(`## FINAL VOTE

You are %s %s. The debate has concluded.

Your personality: %s

Review all the arguments presented and cast your final vote for ALL coins discussed.

Consider:
- The strength of technical arguments
- Data-driven evidence presented
- Risk/reward analysis
- Market timing considerations

You may vote differently from your earlier position if convinced by others' arguments.

### CRITICAL: Output your votes in STRICT JSON ARRAY format (one vote per coin):
<final_vote>
[
  {"symbol": "BTCUSDT", "action": "open_long", "confidence": 75, "leverage": 5, "position_pct": 0.3, "stop_loss": 0.02, "take_profit": 0.04, "reasoning": "BTC final vote reason"},
  {"symbol": "ETHUSDT", "action": "open_short", "confidence": 80, "leverage": 3, "position_pct": 0.2, "stop_loss": 0.03, "take_profit": 0.06, "reasoning": "ETH final vote reason"},
  {"symbol": "SOLUSDT", "action": "wait", "confidence": 60, "reasoning": "SOL not ready"}
]
</final_vote>

### IMPORTANT: action field MUST be exactly one of:
- "open_long" (做多/买入)
- "open_short" (做空/卖出)
- "close_long" (平多仓)
- "close_short" (平空仓)
- "hold" (持仓观望)
- "wait" (空仓等待)

---

%s
`, emoji, participant.Personality, personality, basePrompt)
}

// buildVotingUserPrompt builds the user prompt for voting
func (e *DebateEngine) buildVotingUserPrompt(allMessages []*store.DebateMessage) string {
	var sb strings.Builder
	sb.WriteString("## Debate Summary\n\n")

	// Group messages by participant
	participantMessages := make(map[string][]*store.DebateMessage)
	for _, msg := range allMessages {
		participantMessages[msg.AIModelName] = append(participantMessages[msg.AIModelName], msg)
	}

	for name, msgs := range participantMessages {
		if len(msgs) == 0 {
			continue
		}
		emoji := store.PersonalityEmojis[msgs[0].Personality]
		sb.WriteString(fmt.Sprintf("### %s %s:\n", emoji, name))
		for _, msg := range msgs {
			if msg.Decision != nil {
				sb.WriteString(fmt.Sprintf("- Round %d: %s (Confidence: %d%%)\n", msg.Round, msg.Decision.Action, msg.Decision.Confidence))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nCast your final vote based on the debate above.\n")
	return sb.String()
}

// determineConsensus determines the final consensus from votes (supports multi-coin)
func (e *DebateEngine) determineConsensus(symbol string, votes []*store.DebateVote) *store.DebateDecision {
	decisions := e.determineMultiCoinConsensus(votes)

	// For backward compatibility, return the first decision or a default
	if len(decisions) == 0 {
		return &store.DebateDecision{
			Action:     "hold",
			Symbol:     symbol,
			Confidence: 0,
			Reasoning:  "No consensus reached",
		}
	}

	// If a specific symbol was requested, find it
	if symbol != "" {
		for _, d := range decisions {
			if d.Symbol == symbol {
				return d
			}
		}
	}

	return decisions[0]
}

// determineMultiCoinConsensus determines consensus for all coins from votes
func (e *DebateEngine) determineMultiCoinConsensus(votes []*store.DebateVote) []*store.DebateDecision {
	if len(votes) == 0 {
		return nil
	}

	// Collect all coin decisions from all votes
	// Map: symbol -> action -> weighted score and decision data
	type actionData struct {
		score         float64
		totalConf     int
		totalLeverage int
		totalPosPct   float64
		totalSLPct    float64
		totalTPPct    float64
		count         int
		reasonings    []string
	}

	symbolActions := make(map[string]map[string]*actionData)

	// Process all votes
	logger.Infof("[Debate] Determining multi-coin consensus from %d votes:", len(votes))
	for _, vote := range votes {
		// Process multi-coin decisions if available
		decisionsProcessed := false
		if len(vote.Decisions) > 0 {
			for _, d := range vote.Decisions {
				// Use vote.Symbol as fallback if decision symbol is empty
				symbol := d.Symbol
				if symbol == "" {
					symbol = vote.Symbol
				}
				if symbol == "" || !isValidAction(d.Action) {
					continue
				}
				decisionsProcessed = true
				if _, ok := symbolActions[symbol]; !ok {
					symbolActions[symbol] = make(map[string]*actionData)
				}
				if _, ok := symbolActions[symbol][d.Action]; !ok {
					symbolActions[symbol][d.Action] = &actionData{}
				}
				ad := symbolActions[symbol][d.Action]
				weight := float64(d.Confidence) / 100.0
				if weight < 0.1 {
					weight = 0.5 // Default weight for low confidence
				}
				ad.score += weight
				ad.totalConf += d.Confidence
				if d.Leverage > 0 {
					ad.totalLeverage += d.Leverage
				} else {
					ad.totalLeverage += 5 // Default leverage
				}
				if d.PositionPct > 0 {
					ad.totalPosPct += d.PositionPct
				} else {
					ad.totalPosPct += 0.2 // Default position pct
				}
				ad.totalSLPct += d.StopLoss
				ad.totalTPPct += d.TakeProfit
				ad.count++
				if d.Reasoning != "" {
					ad.reasonings = append(ad.reasonings, d.Reasoning)
				}
				logger.Infof("[Debate]   %s: %s -> %s (conf: %d%%)", vote.AIModelName, symbol, d.Action, d.Confidence)
			}
		}

		// Fallback to single-coin vote if no decisions were processed
		if !decisionsProcessed && vote.Symbol != "" && isValidAction(vote.Action) {
			if _, ok := symbolActions[vote.Symbol]; !ok {
				symbolActions[vote.Symbol] = make(map[string]*actionData)
			}
			if _, ok := symbolActions[vote.Symbol][vote.Action]; !ok {
				symbolActions[vote.Symbol][vote.Action] = &actionData{}
			}
			ad := symbolActions[vote.Symbol][vote.Action]
			weight := float64(vote.Confidence) / 100.0
			if weight < 0.1 {
				weight = 0.5 // Default weight for low confidence
			}
			ad.score += weight
			ad.totalConf += vote.Confidence
			if vote.Leverage > 0 {
				ad.totalLeverage += vote.Leverage
			} else {
				ad.totalLeverage += 5 // Default leverage
			}
			if vote.PositionPct > 0 {
				ad.totalPosPct += vote.PositionPct
			} else {
				ad.totalPosPct += 0.2 // Default position pct
			}
			ad.totalSLPct += vote.StopLossPct
			ad.totalTPPct += vote.TakeProfitPct
			ad.count++
			if vote.Reasoning != "" {
				ad.reasonings = append(ad.reasonings, vote.Reasoning)
			}
			logger.Infof("[Debate]   %s: %s -> %s (conf: %d%%)", vote.AIModelName, vote.Symbol, vote.Action, vote.Confidence)
		}
	}

	// Determine winning action for each symbol
	var results []*store.DebateDecision
	for symbol, actions := range symbolActions {
		var winningAction string
		var maxScore float64
		for action, ad := range actions {
			if ad.score > maxScore {
				maxScore = ad.score
				winningAction = action
			}
		}

		if winningAction == "" {
			continue
		}

		ad := actions[winningAction]
		if ad.count == 0 {
			continue
		}

		// Calculate averages
		avgConf := ad.totalConf / ad.count
		avgLeverage := ad.totalLeverage / ad.count
		avgPosPct := ad.totalPosPct / float64(ad.count)
		avgSLPct := ad.totalSLPct / float64(ad.count)
		avgTPPct := ad.totalTPPct / float64(ad.count)

		// Apply defaults and limits
		if avgLeverage < 1 {
			avgLeverage = 5
		}
		if avgLeverage > 20 {
			avgLeverage = 20
		}
		if avgPosPct < 0.1 {
			avgPosPct = 0.2
		}
		if avgPosPct > 1.0 {
			avgPosPct = 1.0
		}
		// Apply defaults for SL/TP if not set
		if avgSLPct <= 0 && (winningAction == "open_long" || winningAction == "open_short") {
			avgSLPct = 0.03 // Default 3% stop loss
		}
		if avgTPPct <= 0 && (winningAction == "open_long" || winningAction == "open_short") {
			avgTPPct = 0.06 // Default 6% take profit
		}

		decision := &store.DebateDecision{
			Action:      winningAction,
			Symbol:      symbol,
			Confidence:  avgConf,
			Leverage:    avgLeverage,
			PositionPct: avgPosPct,
			StopLoss:    avgSLPct,
			TakeProfit:  avgTPPct,
			Reasoning:   strings.Join(ad.reasonings, "; "),
		}

		logger.Infof("[Debate] Consensus for %s: %s (score: %.2f, conf: %d%%, leverage: %dx)",
			symbol, winningAction, maxScore, avgConf, avgLeverage)

		results = append(results, decision)
	}

	logger.Infof("[Debate] Total %d consensus decisions", len(results))
	return results
}

// CancelDebate cancels a running debate
func (e *DebateEngine) CancelDebate(sessionID string) error {
	return e.debateStore.UpdateSessionStatus(sessionID, store.DebateStatusCancelled)
}

// ExecuteConsensus executes the consensus decision from a completed debate
func (e *DebateEngine) ExecuteConsensus(sessionID string, executor TraderExecutor) error {
	session, err := e.debateStore.GetSessionWithDetails(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session.Status != store.DebateStatusCompleted {
		return fmt.Errorf("debate is not completed (status: %s)", session.Status)
	}

	if session.FinalDecision == nil {
		return fmt.Errorf("no final decision available")
	}

	if session.FinalDecision.Executed {
		return fmt.Errorf("consensus already executed at %s", session.FinalDecision.ExecutedAt.Format(time.RFC3339))
	}

	action := session.FinalDecision.Action
	if action != "open_long" && action != "open_short" {
		return fmt.Errorf("action '%s' does not require execution", action)
	}

	// Get current market price
	marketData, err := market.Get(session.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get market data: %w", err)
	}

	// Get account balance
	balance, err := executor.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	// Debug log balance keys and values
	logger.Infof("Debate execution - balance data: %+v", balance)

	// Use available_balance for position sizing (not total equity)
	availableBalance := 0.0
	if avail, ok := balance["available_balance"].(float64); ok && avail > 0 {
		availableBalance = avail
		logger.Infof("Using available_balance: %.2f", availableBalance)
	} else if eq, ok := balance["total_equity"].(float64); ok && eq > 0 {
		// Fallback to total_equity if available_balance not found
		availableBalance = eq
		logger.Infof("Fallback to total_equity: %.2f", availableBalance)
	} else if wallet, ok := balance["wallet_balance"].(float64); ok && wallet > 0 {
		availableBalance = wallet
		logger.Infof("Fallback to wallet_balance: %.2f", availableBalance)
	}

	if availableBalance <= 0 {
		// Log all balance keys for debugging
		keys := make([]string, 0, len(balance))
		for k, v := range balance {
			keys = append(keys, fmt.Sprintf("%s=%v", k, v))
		}
		return fmt.Errorf("invalid available balance: %.2f (balance data: %v)", availableBalance, keys)
	}

	// Calculate position size = available_balance × position_pct
	positionSizeUSD := availableBalance * session.FinalDecision.PositionPct
	if positionSizeUSD < 12 {
		positionSizeUSD = 12
	}

	// Calculate stop loss and take profit prices
	currentPrice := marketData.CurrentPrice
	var stopLossPrice, takeProfitPrice float64

	if action == "open_long" {
		stopLossPrice = currentPrice * (1 - session.FinalDecision.StopLoss)
		takeProfitPrice = currentPrice * (1 + session.FinalDecision.TakeProfit)
	} else {
		stopLossPrice = currentPrice * (1 + session.FinalDecision.StopLoss)
		takeProfitPrice = currentPrice * (1 - session.FinalDecision.TakeProfit)
	}

	// Create decision
	tradeDecision := &kernel.Decision{
		Symbol:          session.Symbol,
		Action:          action,
		Leverage:        session.FinalDecision.Leverage,
		PositionSizeUSD: positionSizeUSD,
		StopLoss:        stopLossPrice,
		TakeProfit:      takeProfitPrice,
		Confidence:      session.FinalDecision.Confidence,
		Reasoning:       fmt.Sprintf("Debate consensus: %s", session.FinalDecision.Reasoning),
	}

	logger.Infof("======== EXECUTING DEBATE CONSENSUS ========")
	logger.Infof("Session ID: %s", sessionID)
	logger.Infof("Symbol: %s", session.Symbol)
	logger.Infof("Action: %s (from FinalDecision.Action: %s)", action, session.FinalDecision.Action)
	logger.Infof("Position Size: %.2f USD", positionSizeUSD)
	logger.Infof("Leverage: %dx", tradeDecision.Leverage)
	logger.Infof("StopLoss: %.4f, TakeProfit: %.4f", stopLossPrice, takeProfitPrice)
	logger.Infof("=============================================")
	logger.Infof("Executing debate consensus: %s %s @ %.2f USD, leverage %dx",
		action, session.Symbol, positionSizeUSD, tradeDecision.Leverage)

	// Execute
	err = executor.ExecuteDecision(tradeDecision)

	// Update session
	session.FinalDecision.Executed = err == nil
	session.FinalDecision.ExecutedAt = time.Now()
	session.FinalDecision.PositionSizeUSD = positionSizeUSD
	if err != nil {
		session.FinalDecision.Error = err.Error()
	}

	e.debateStore.UpdateSessionFinalDecision(sessionID, session.FinalDecision)

	if err != nil {
		return fmt.Errorf("trade execution failed: %w", err)
	}

	return nil
}

// Helper functions

func getPersonalityDescription(personality store.DebatePersonality) string {
	switch personality {
	case store.PersonalityBull:
		return "Aggressive Bull - You are optimistic and look for long opportunities. You believe in upward momentum and trend continuation. Focus on bullish signals and support levels."
	case store.PersonalityBear:
		return "Cautious Bear - You are skeptical and focus on risks. You look for short opportunities and warning signs. Question bullish narratives and highlight resistance levels."
	case store.PersonalityAnalyst:
		return "Data Analyst - You are neutral and purely data-driven. Present technical analysis without bias. Let the indicators speak for themselves."
	case store.PersonalityContrarian:
		return "Contrarian - You challenge majority opinions and look for overlooked opportunities. Question consensus views and find alternative interpretations of the data."
	case store.PersonalityRiskManager:
		return "Risk Manager - You focus on position sizing, stop losses, and capital preservation. Evaluate risk/reward ratios and warn about potential downsides."
	default:
		return "Market Analyst - Provide balanced technical analysis."
	}
}

// parseDecisions extracts multiple decisions from AI response using strict JSON parsing
func parseDecisions(response string) ([]*store.DebateDecision, int) {
	avgConfidence := 50

	// Log first 500 chars of response for debugging
	responsePreview := response
	if len(responsePreview) > 500 {
		responsePreview = responsePreview[:500] + "..."
	}
	logger.Infof("[Debate] Parsing response (preview): %s", responsePreview)

	// Try to extract JSON from <decision> or <final_vote> tag
	var jsonContent string
	decisionPattern := regexp.MustCompile(`(?s)<decision>\s*(.*?)\s*</decision>`)
	finalVotePattern := regexp.MustCompile(`(?s)<final_vote>\s*(.*?)\s*</final_vote>`)

	if matches := decisionPattern.FindStringSubmatch(response); len(matches) > 1 {
		jsonContent = strings.TrimSpace(matches[1])
		logger.Infof("[Debate] Found <decision> tag, content length: %d", len(jsonContent))
	} else if matches := finalVotePattern.FindStringSubmatch(response); len(matches) > 1 {
		jsonContent = strings.TrimSpace(matches[1])
		logger.Infof("[Debate] Found <final_vote> tag, content length: %d", len(jsonContent))
	}

	if jsonContent != "" {
		// Intermediate struct to handle both field naming conventions
		type rawDecision struct {
			Action       string  `json:"action"`
			Symbol       string  `json:"symbol"`
			Confidence   int     `json:"confidence"`
			Leverage     int     `json:"leverage"`
			PositionPct  float64 `json:"position_pct"`
			StopLoss     float64 `json:"stop_loss"`
			TakeProfit   float64 `json:"take_profit"`
			StopLossPct  float64 `json:"stop_loss_pct"`  // Alternative field name
			TakeProfitPct float64 `json:"take_profit_pct"` // Alternative field name
			Reasoning    string  `json:"reasoning"`
		}

		convertRawDecision := func(r *rawDecision) *store.DebateDecision {
			d := &store.DebateDecision{
				Action:      normalizeAction(r.Action),
				Symbol:      r.Symbol,
				Confidence:  r.Confidence,
				Leverage:    r.Leverage,
				PositionPct: r.PositionPct,
				Reasoning:   r.Reasoning,
			}
			// Use stop_loss or stop_loss_pct (whichever is set)
			if r.StopLoss > 0 {
				d.StopLoss = r.StopLoss
			} else if r.StopLossPct > 0 {
				d.StopLoss = r.StopLossPct
			}
			// Use take_profit or take_profit_pct (whichever is set)
			if r.TakeProfit > 0 {
				d.TakeProfit = r.TakeProfit
			} else if r.TakeProfitPct > 0 {
				d.TakeProfit = r.TakeProfitPct
			}
			// Apply defaults
			if d.Leverage == 0 {
				d.Leverage = 5
			}
			if d.PositionPct == 0 {
				d.PositionPct = 0.2
			}
			return d
		}

		// Try to parse as JSON array first
		var rawDecisions []*rawDecision
		if err := json.Unmarshal([]byte(jsonContent), &rawDecisions); err == nil && len(rawDecisions) > 0 {
			logger.Infof("[Debate] Parsed %d decisions from JSON array", len(rawDecisions))
			validDecisions := make([]*store.DebateDecision, 0)
			totalConfidence := 0
			for _, r := range rawDecisions {
				d := convertRawDecision(r)
				if isValidAction(d.Action) {
					validDecisions = append(validDecisions, d)
					totalConfidence += d.Confidence
					logger.Infof("[Debate]   - %s: %s (conf: %d%%, sl: %.4f, tp: %.4f)", d.Symbol, d.Action, d.Confidence, d.StopLoss, d.TakeProfit)
				}
			}
			if len(validDecisions) > 0 {
				avgConfidence = totalConfidence / len(validDecisions)
				return validDecisions, avgConfidence
			}
		}

		// Try to parse as single JSON object
		var singleRaw rawDecision
		if err := json.Unmarshal([]byte(jsonContent), &singleRaw); err == nil {
			d := convertRawDecision(&singleRaw)
			if isValidAction(d.Action) {
				logger.Infof("[Debate] Parsed single decision: %s %s (conf: %d%%, sl: %.4f, tp: %.4f)",
					d.Symbol, d.Action, d.Confidence, d.StopLoss, d.TakeProfit)
				return []*store.DebateDecision{d}, d.Confidence
			}
		}

		// Try to find JSON array in content
		jsonArrayPattern := regexp.MustCompile(`\[[\s\S]*\]`)
		if jsonArray := jsonArrayPattern.FindString(jsonContent); jsonArray != "" {
			if err := json.Unmarshal([]byte(jsonArray), &rawDecisions); err == nil && len(rawDecisions) > 0 {
				logger.Infof("[Debate] Parsed %d decisions from embedded JSON array", len(rawDecisions))
				validDecisions := make([]*store.DebateDecision, 0)
				totalConfidence := 0
				for _, r := range rawDecisions {
					d := convertRawDecision(r)
					if isValidAction(d.Action) {
						validDecisions = append(validDecisions, d)
						totalConfidence += d.Confidence
					}
				}
				if len(validDecisions) > 0 {
					avgConfidence = totalConfidence / len(validDecisions)
					return validDecisions, avgConfidence
				}
			}
		}
	} else {
		logger.Warnf("[Debate] No <decision> or <final_vote> tag found in response!")
	}

	// Fallback: create a single decision with fallback action
	logger.Warnf("[Debate] No valid decisions found, using fallback parsing")
	fallbackAction := fallbackParseAction(response)
	fallbackDecision := &store.DebateDecision{
		Action:      fallbackAction,
		Confidence:  50,
		Leverage:    5,
		PositionPct: 0.2,
	}
	logger.Infof("[Debate] Fallback decision: %s", fallbackAction)
	return []*store.DebateDecision{fallbackDecision}, 50
}

// parseDecision extracts single decision (backward compatible wrapper)
func parseDecision(response string) (*store.DebateDecision, int) {
	decisions, confidence := parseDecisions(response)
	if len(decisions) > 0 {
		return decisions[0], confidence
	}
	return &store.DebateDecision{Action: "wait", Confidence: 50}, 50
}

// isValidAction checks if action is one of the valid actions
func isValidAction(action string) bool {
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}
	return validActions[strings.ToLower(strings.TrimSpace(action))]
}

// normalizeAction normalizes action string to standard format
func normalizeAction(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	action = strings.ReplaceAll(action, " ", "_")
	action = strings.ReplaceAll(action, "-", "_")

	// Map common variations
	actionMap := map[string]string{
		"long":       "open_long",
		"openlong":   "open_long",
		"buy":        "open_long",
		"short":      "open_short",
		"openshort":  "open_short",
		"sell":       "open_short",
		"closelong":  "close_long",
		"closeshort": "close_short",
	}

	if mapped, ok := actionMap[action]; ok {
		return mapped
	}
	return action
}

// fallbackParseAction parses action from full response text when <decision> parsing fails
func fallbackParseAction(response string) string {
	responseLower := strings.ToLower(response)

	// Count specific action keywords only
	openLongCount := strings.Count(responseLower, "\"action\": \"open_long\"") +
		strings.Count(responseLower, "\"action\":\"open_long\"") +
		strings.Count(responseLower, "action: open_long")
	openShortCount := strings.Count(responseLower, "\"action\": \"open_short\"") +
		strings.Count(responseLower, "\"action\":\"open_short\"") +
		strings.Count(responseLower, "action: open_short")
	holdCount := strings.Count(responseLower, "\"action\": \"hold\"") +
		strings.Count(responseLower, "\"action\":\"hold\"") +
		strings.Count(responseLower, "action: hold")
	waitCount := strings.Count(responseLower, "\"action\": \"wait\"") +
		strings.Count(responseLower, "\"action\":\"wait\"") +
		strings.Count(responseLower, "action: wait")

	logger.Infof("[Debate] Fallback action counts: long=%d, short=%d, hold=%d, wait=%d",
		openLongCount, openShortCount, holdCount, waitCount)

	// Find max
	maxCount := 0
	action := "wait"
	if openLongCount > maxCount {
		maxCount = openLongCount
		action = "open_long"
	}
	if openShortCount > maxCount {
		maxCount = openShortCount
		action = "open_short"
	}
	if holdCount > maxCount {
		maxCount = holdCount
		action = "hold"
	}
	if waitCount > maxCount {
		action = "wait"
	}

	return action
}

// VoteResult holds the parsed vote details
type VoteResult struct {
	Action        string
	Confidence    int
	Reasoning     string
	Leverage      int
	PositionPct   float64
	StopLossPct   float64
	TakeProfitPct float64
}

// parseVote extracts vote from AI response using strict JSON parsing
func parseVote(response string) *VoteResult {
	result := &VoteResult{
		Confidence:  50,
		Leverage:    5,
		PositionPct: 0.2,
	}

	// Try to extract JSON from <final_vote> tag
	votePattern := regexp.MustCompile(`(?s)<final_vote>\s*(.*?)\s*</final_vote>`)
	if matches := votePattern.FindStringSubmatch(response); len(matches) > 1 {
		jsonContent := strings.TrimSpace(matches[1])

		// Try direct JSON parse first
		if err := json.Unmarshal([]byte(jsonContent), result); err == nil {
			logger.Infof("[Debate] Parsed vote JSON: action=%s, confidence=%d", result.Action, result.Confidence)
			if isValidAction(result.Action) {
				result.Action = normalizeAction(result.Action)
				return result
			}
			logger.Warnf("[Debate] Invalid action in vote JSON: %s", result.Action)
		}

		// Try to find JSON object in content
		jsonObjPattern := regexp.MustCompile(`\{[^}]+\}`)
		if jsonObj := jsonObjPattern.FindString(jsonContent); jsonObj != "" {
			if err := json.Unmarshal([]byte(jsonObj), result); err == nil {
				logger.Infof("[Debate] Parsed vote from JSON object: action=%s, confidence=%d", result.Action, result.Confidence)
				if isValidAction(result.Action) {
					result.Action = normalizeAction(result.Action)
					return result
				}
			}
		}

		// Fallback to key-value parsing
		if action := extractValue(jsonContent, "action"); action != "" {
			result.Action = normalizeAction(action)
		}
		if confStr := extractValue(jsonContent, "confidence"); confStr != "" {
			if c, err := strconv.Atoi(strings.TrimSpace(confStr)); err == nil {
				result.Confidence = c
			}
		}
		result.Reasoning = extractValue(jsonContent, "reasoning")
		if leverageStr := extractValue(jsonContent, "leverage"); leverageStr != "" {
			if lev, err := strconv.Atoi(strings.TrimSpace(leverageStr)); err == nil {
				result.Leverage = lev
			}
		}
		if posPctStr := extractValue(jsonContent, "position_pct"); posPctStr != "" {
			if pct, err := strconv.ParseFloat(strings.TrimSpace(posPctStr), 64); err == nil {
				result.PositionPct = pct
			}
		}
		if slPctStr := extractValue(jsonContent, "stop_loss_pct"); slPctStr != "" {
			if sl, err := strconv.ParseFloat(strings.TrimSpace(slPctStr), 64); err == nil {
				result.StopLossPct = sl
			}
		}
		if tpPctStr := extractValue(jsonContent, "take_profit_pct"); tpPctStr != "" {
			if tp, err := strconv.ParseFloat(strings.TrimSpace(tpPctStr), 64); err == nil {
				result.TakeProfitPct = tp
			}
		}
	}

	// Normalize action if found
	if result.Action != "" {
		result.Action = normalizeAction(result.Action)
	}

	// Only use fallback if no valid action found
	if !isValidAction(result.Action) {
		logger.Warnf("[Debate] No valid action in <final_vote> tag, using fallback parsing")
		result.Action = fallbackParseAction(response)
		logger.Infof("[Debate] Fallback parsed vote action: %s", result.Action)
	}

	return result
}

// extractValue extracts a value from key: value format
func extractValue(content, key string) string {
	patterns := []string{
		fmt.Sprintf(`(?i)%s:\s*([^\n,]+)`, key),
		fmt.Sprintf(`(?i)"%s":\s*"?([^"\n,]+)"?`, key),
		fmt.Sprintf(`(?i)'%s':\s*'?([^'\n,]+)'?`, key),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}
