package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"nofx/debate"
	"nofx/logger"
	"nofx/provider/nofxos"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

// DebateHandler handles debate-related API requests
type DebateHandler struct {
	debateStore   *store.DebateStore
	strategyStore *store.StrategyStore
	aiModelStore  *store.AIModelStore
	engine        *debate.DebateEngine

	// Trader manager for execution
	traderManager DebateTraderManager

	// SSE subscribers
	subscribers   map[string]map[chan []byte]bool // sessionID -> channels
	subscribersMu sync.RWMutex
}

// DebateTraderManager interface for getting trader executors
type DebateTraderManager interface {
	GetTraderExecutor(traderID string) (debate.TraderExecutor, error)
}

// NewDebateHandler creates a new DebateHandler
func NewDebateHandler(debateStore *store.DebateStore, strategyStore *store.StrategyStore, aiModelStore *store.AIModelStore) *DebateHandler {
	handler := &DebateHandler{
		debateStore:   debateStore,
		strategyStore: strategyStore,
		aiModelStore:  aiModelStore,
		subscribers:   make(map[string]map[chan []byte]bool),
	}

	// Create debate engine with event callbacks
	handler.engine = debate.NewDebateEngine(debateStore, strategyStore, aiModelStore)
	handler.engine.OnRoundStart = handler.broadcastRoundStart
	handler.engine.OnMessage = handler.broadcastMessage
	handler.engine.OnRoundEnd = handler.broadcastRoundEnd
	handler.engine.OnVote = handler.broadcastVote
	handler.engine.OnConsensus = handler.broadcastConsensus
	handler.engine.OnError = handler.broadcastError

	return handler
}

// CreateDebateRequest represents a request to create a new debate
type CreateDebateRequest struct {
	Name            string              `json:"name" binding:"required"`
	StrategyID      string              `json:"strategy_id" binding:"required"`
	Symbol          string              `json:"symbol"` // Optional: auto-selected based on strategy if empty
	MaxRounds       int                 `json:"max_rounds"`
	IntervalMinutes int                 `json:"interval_minutes"`
	PromptVariant   string              `json:"prompt_variant"`
	AutoExecute     bool                `json:"auto_execute"`
	TraderID        string              `json:"trader_id"`
	Participants    []ParticipantConfig `json:"participants" binding:"required,min=2"`
	// OI Ranking data options
	EnableOIRanking bool   `json:"enable_oi_ranking"` // Whether to include OI ranking data
	OIRankingLimit  int    `json:"oi_ranking_limit"`  // Number of OI ranking entries (default 10)
	OIDuration      string `json:"oi_duration"`       // Duration for OI data (1h, 4h, 24h, etc.)
}

// ParticipantConfig represents a participant configuration
type ParticipantConfig struct {
	AIModelID   string `json:"ai_model_id" binding:"required"`
	Personality string `json:"personality" binding:"required"`
}

// HandleListDebates lists all debates for a user
func (h *DebateHandler) HandleListDebates(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	sessions, err := h.debateStore.GetSessionsByUser(userID)
	if err != nil {
		logger.Errorf("Failed to get debates for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get debates"})
		return
	}

	// Return empty array instead of null
	if sessions == nil {
		sessions = []*store.DebateSession{}
	}

	c.JSON(http.StatusOK, sessions)
}

// HandleGetDebate gets a specific debate with all details
func (h *DebateHandler) HandleGetDebate(c *gin.Context) {
	debateID := c.Param("id")
	userID := c.GetString("user_id")

	session, err := h.debateStore.GetSessionWithDetails(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debate not found"})
		return
	}

	// Check ownership
	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// HandleCreateDebate creates a new debate
func (h *DebateHandler) HandleCreateDebate(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req CreateDebateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Validate strategy exists
	strategy, err := h.strategyStore.Get(userID, req.StrategyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "strategy not found"})
		return
	}

	// Validate strategy belongs to user or is default
	if strategy.UserID != userID && !strategy.IsDefault {
		c.JSON(http.StatusForbidden, gin.H{"error": "strategy access denied"})
		return
	}

	// Auto-select symbol based on strategy if not provided
	if req.Symbol == "" {
		req.Symbol = "BTCUSDT" // default fallback
		if strategyConfig, err := strategy.ParseConfig(); err == nil {
			coinSource := strategyConfig.CoinSource
			switch coinSource.SourceType {
			case "static":
				if len(coinSource.StaticCoins) > 0 {
					req.Symbol = coinSource.StaticCoins[0]
				}
			case "ai500":
				// Fetch from AI500 API
				if coins, err := nofxos.DefaultClient().GetTopRatedCoins(1); err == nil && len(coins) > 0 {
					req.Symbol = coins[0]
					logger.Infof("Fetched coin from AI500 API: %s", req.Symbol)
				}
			case "oi_top":
				// Fetch from OI top API
				if coins, err := nofxos.DefaultClient().GetOITopSymbols(); err == nil && len(coins) > 0 {
					req.Symbol = coins[0]
					logger.Infof("Fetched coin from OI Top API: %s", req.Symbol)
				}
			case "mixed":
				// Try AI500 first, then OI top
				if coinSource.UseAI500 {
					if coins, err := nofxos.DefaultClient().GetTopRatedCoins(1); err == nil && len(coins) > 0 {
						req.Symbol = coins[0]
						logger.Infof("Fetched coin from AI500 API (mixed): %s", req.Symbol)
					}
				} else if coinSource.UseOITop {
					if coins, err := nofxos.DefaultClient().GetOITopSymbols(); err == nil && len(coins) > 0 {
						req.Symbol = coins[0]
						logger.Infof("Fetched coin from OI Top API (mixed): %s", req.Symbol)
					}
				}
			}
			logger.Infof("Auto-selected symbol %s for debate based on strategy %s (source_type=%s)",
				req.Symbol, strategy.Name, coinSource.SourceType)
		}
	}

	// Set defaults
	if req.MaxRounds <= 0 || req.MaxRounds > 5 {
		req.MaxRounds = 3
	}
	if req.IntervalMinutes <= 0 {
		req.IntervalMinutes = 5
	}
	if req.PromptVariant == "" {
		req.PromptVariant = "balanced"
	}

	// Create session
	session := &store.DebateSession{
		UserID:          userID,
		Name:            req.Name,
		StrategyID:      req.StrategyID,
		Symbol:          req.Symbol,
		MaxRounds:       req.MaxRounds,
		IntervalMinutes: req.IntervalMinutes,
		PromptVariant:   req.PromptVariant,
		AutoExecute:     req.AutoExecute,
		TraderID:        req.TraderID,
		EnableOIRanking: req.EnableOIRanking,
		OIRankingLimit:  req.OIRankingLimit,
		OIDuration:      req.OIDuration,
	}

	if err := h.debateStore.CreateSession(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create debate"})
		return
	}

	// Add participants
	for i, p := range req.Participants {
		// Validate AI model exists and belongs to user
		aiModel, err := h.aiModelStore.GetByID(p.AIModelID)
		if err != nil {
			logger.Warnf("AI model not found: %s", p.AIModelID)
			continue
		}
		if aiModel.UserID != userID {
			logger.Warnf("AI model %s does not belong to user", p.AIModelID)
			continue
		}

		// Validate personality
		personality := store.DebatePersonality(p.Personality)
		if _, ok := store.PersonalityColors[personality]; !ok {
			personality = store.PersonalityAnalyst
		}

		participant := &store.DebateParticipant{
			SessionID:   session.ID,
			AIModelID:   p.AIModelID,
			AIModelName: aiModel.Name,
			Provider:    aiModel.Provider,
			Personality: personality,
			Color:       store.PersonalityColors[personality],
			SpeakOrder:  i,
		}

		if err := h.debateStore.AddParticipant(participant); err != nil {
			logger.Errorf("Failed to add participant: %v", err)
		}
	}

	// Get full session with participants
	fullSession, _ := h.debateStore.GetSessionWithDetails(session.ID)

	c.JSON(http.StatusCreated, fullSession)
}

// HandleStartDebate starts a debate
func (h *DebateHandler) HandleStartDebate(c *gin.Context) {
	debateID := c.Param("id")
	userID := c.GetString("user_id")

	session, err := h.debateStore.GetSession(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debate not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if session.Status != store.DebateStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "debate is not in pending status"})
		return
	}

	// Start debate asynchronously
	if err := h.engine.StartDebate(debateID); err != nil {
		SafeInternalError(c, "Start debate", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "debate started", "id": debateID})
}

// HandleCancelDebate cancels a running debate
func (h *DebateHandler) HandleCancelDebate(c *gin.Context) {
	debateID := c.Param("id")
	userID := c.GetString("user_id")

	session, err := h.debateStore.GetSession(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debate not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if err := h.engine.CancelDebate(debateID); err != nil {
		SafeInternalError(c, "Cancel debate", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "debate cancelled"})
}

// HandleDeleteDebate deletes a debate
func (h *DebateHandler) HandleDeleteDebate(c *gin.Context) {
	debateID := c.Param("id")
	userID := c.GetString("user_id")

	session, err := h.debateStore.GetSession(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debate not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Don't allow deleting running debates
	if session.Status == store.DebateStatusRunning || session.Status == store.DebateStatusVoting {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete running debate"})
		return
	}

	if err := h.debateStore.DeleteSession(debateID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete debate"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "debate deleted"})
}

// HandleGetMessages gets all messages for a debate
func (h *DebateHandler) HandleGetMessages(c *gin.Context) {
	debateID := c.Param("id")
	userID := c.GetString("user_id")

	session, err := h.debateStore.GetSession(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debate not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	messages, err := h.debateStore.GetMessages(debateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get messages"})
		return
	}

	c.JSON(http.StatusOK, messages)
}

// HandleGetVotes gets all votes for a debate
func (h *DebateHandler) HandleGetVotes(c *gin.Context) {
	debateID := c.Param("id")
	userID := c.GetString("user_id")

	session, err := h.debateStore.GetSession(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debate not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	votes, err := h.debateStore.GetVotes(debateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get votes"})
		return
	}

	c.JSON(http.StatusOK, votes)
}

// HandleDebateStream handles SSE streaming for live debate updates
func (h *DebateHandler) HandleDebateStream(c *gin.Context) {
	debateID := c.Param("id")
	userID := c.GetString("user_id")

	session, err := h.debateStore.GetSession(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debate not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// Create channel for this subscriber
	ch := make(chan []byte, 100)
	h.addSubscriber(debateID, ch)
	defer h.removeSubscriber(debateID, ch)

	// Send initial state
	initialState, _ := h.debateStore.GetSessionWithDetails(debateID)
	initialData, _ := json.Marshal(map[string]interface{}{
		"event": "initial",
		"data":  initialState,
	})
	c.Writer.Write([]byte(fmt.Sprintf("event: initial\ndata: %s\n\n", initialData)))
	c.Writer.Flush()

	// Stream updates
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case msg := <-ch:
			c.Writer.Write(msg)
			c.Writer.Flush()
		}
	}
}

// SetTraderManager sets the trader manager for executing trades
func (h *DebateHandler) SetTraderManager(tm DebateTraderManager) {
	h.traderManager = tm
}

// ExecuteDebateRequest represents a request to execute a debate's consensus
type ExecuteDebateRequest struct {
	TraderID string `json:"trader_id" binding:"required"`
}

// HandleExecuteDebate executes the consensus decision from a completed debate
func (h *DebateHandler) HandleExecuteDebate(c *gin.Context) {
	debateID := c.Param("id")
	userID := c.GetString("user_id")

	// Check trader manager is available
	if h.traderManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "trading service not available"})
		return
	}

	// Get debate session
	session, err := h.debateStore.GetSession(debateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debate not found"})
		return
	}

	// Check ownership
	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Check status
	if session.Status != store.DebateStatusCompleted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "debate is not completed"})
		return
	}

	// Parse request
	var req ExecuteDebateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Get trader executor
	executor, err := h.traderManager.GetTraderExecutor(req.TraderID)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Trader not available", err)
		return
	}

	// Execute consensus
	if err := h.engine.ExecuteConsensus(debateID, executor); err != nil {
		SafeInternalError(c, "Execute consensus", err)
		return
	}

	// Get updated session
	updatedSession, _ := h.debateStore.GetSessionWithDetails(debateID)

	c.JSON(http.StatusOK, gin.H{
		"message": "consensus executed successfully",
		"session": updatedSession,
	})
}

// GetPersonalities returns available AI personalities
func (h *DebateHandler) HandleGetPersonalities(c *gin.Context) {
	personalities := []map[string]interface{}{
		{
			"id":          "bull",
			"name":        "Aggressive Bull",
			"emoji":       "🐂",
			"color":       store.PersonalityColors[store.PersonalityBull],
			"description": "Looks for long opportunities, optimistic about market",
		},
		{
			"id":          "bear",
			"name":        "Cautious Bear",
			"emoji":       "🐻",
			"color":       store.PersonalityColors[store.PersonalityBear],
			"description": "Skeptical, focuses on risks and short opportunities",
		},
		{
			"id":          "analyst",
			"name":        "Data Analyst",
			"emoji":       "📊",
			"color":       store.PersonalityColors[store.PersonalityAnalyst],
			"description": "Pure technical analysis, neutral and data-driven",
		},
		{
			"id":          "contrarian",
			"name":        "Contrarian",
			"emoji":       "🔄",
			"color":       store.PersonalityColors[store.PersonalityContrarian],
			"description": "Challenges majority opinion, looks for overlooked opportunities",
		},
		{
			"id":          "risk_manager",
			"name":        "Risk Manager",
			"emoji":       "🛡️",
			"color":       store.PersonalityColors[store.PersonalityRiskManager],
			"description": "Focuses on position sizing, stop losses, and risk control",
		},
	}
	c.JSON(http.StatusOK, personalities)
}

// SSE broadcast helpers
func (h *DebateHandler) addSubscriber(sessionID string, ch chan []byte) {
	h.subscribersMu.Lock()
	defer h.subscribersMu.Unlock()

	if h.subscribers[sessionID] == nil {
		h.subscribers[sessionID] = make(map[chan []byte]bool)
	}
	h.subscribers[sessionID][ch] = true
}

func (h *DebateHandler) removeSubscriber(sessionID string, ch chan []byte) {
	h.subscribersMu.Lock()
	defer h.subscribersMu.Unlock()

	if h.subscribers[sessionID] != nil {
		delete(h.subscribers[sessionID], ch)
		close(ch)
	}
}

func (h *DebateHandler) broadcast(sessionID string, event string, data interface{}) {
	h.subscribersMu.RLock()
	defer h.subscribersMu.RUnlock()

	subs := h.subscribers[sessionID]
	if subs == nil {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, jsonData))
	for ch := range subs {
		select {
		case ch <- msg:
		default:
			// Channel full, skip
		}
	}
}

func (h *DebateHandler) broadcastRoundStart(sessionID string, round int) {
	h.broadcast(sessionID, "round_start", map[string]interface{}{
		"round":  round,
		"status": "running",
	})
}

func (h *DebateHandler) broadcastMessage(sessionID string, msg *store.DebateMessage) {
	h.broadcast(sessionID, "message", msg)
}

func (h *DebateHandler) broadcastRoundEnd(sessionID string, round int) {
	h.broadcast(sessionID, "round_end", map[string]interface{}{
		"round":  round,
		"status": "completed",
	})
}

func (h *DebateHandler) broadcastVote(sessionID string, vote *store.DebateVote) {
	h.broadcast(sessionID, "vote", vote)
}

func (h *DebateHandler) broadcastConsensus(sessionID string, decision *store.DebateDecision) {
	h.broadcast(sessionID, "consensus", decision)
}

func (h *DebateHandler) broadcastError(sessionID string, err error) {
	// Sanitize error message before broadcasting to client
	safeMsg := SanitizeError(err, "An error occurred during debate")
	h.broadcast(sessionID, "error", map[string]interface{}{
		"error": safeMsg,
	})
}
