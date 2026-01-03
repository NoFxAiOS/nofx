package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"nofx/backtest"
	"nofx/logger"
	"nofx/market"
	"nofx/provider/nofxos"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerBacktestRoutes(router *gin.RouterGroup) {
	router.POST("/start", s.handleBacktestStart)
	router.POST("/pause", s.handleBacktestPause)
	router.POST("/resume", s.handleBacktestResume)
	router.POST("/stop", s.handleBacktestStop)
	router.POST("/label", s.handleBacktestLabel)
	router.POST("/delete", s.handleBacktestDelete)
	router.GET("/status", s.handleBacktestStatus)
	router.GET("/runs", s.handleBacktestRuns)
	router.GET("/equity", s.handleBacktestEquity)
	router.GET("/trades", s.handleBacktestTrades)
	router.GET("/metrics", s.handleBacktestMetrics)
	router.GET("/trace", s.handleBacktestTrace)
	router.GET("/decisions", s.handleBacktestDecisions)
	router.GET("/export", s.handleBacktestExport)
	router.GET("/klines", s.handleBacktestKlines)
}

type backtestStartRequest struct {
	Config backtest.BacktestConfig `json:"config"`
}

type runIDRequest struct {
	RunID string `json:"run_id"`
}

type labelRequest struct {
	RunID string `json:"run_id"`
	Label string `json:"label"`
}

func (s *Server) handleBacktestStart(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}

	var req backtestStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	cfg := req.Config
	if cfg.RunID == "" {
		cfg.RunID = "bt_" + time.Now().UTC().Format("20060102_150405")
	}
	cfg.CustomPrompt = strings.TrimSpace(cfg.CustomPrompt)
	cfg.UserID = normalizeUserID(c.GetString("user_id"))

	logger.Infof("📊 Backtest request - symbols from request: %v (count=%d), strategyID: %s",
		cfg.Symbols, len(cfg.Symbols), cfg.StrategyID)

	// Load strategy config if strategy_id is provided
	if cfg.StrategyID != "" {
		strategy, err := s.store.Strategy().Get(cfg.UserID, cfg.StrategyID)
		if err != nil {
			SafeBadRequest(c, "Failed to load strategy")
			return
		}
		if strategy == nil {
			SafeBadRequest(c, "Strategy not found")
			return
		}
		var strategyConfig store.StrategyConfig
		if err := json.Unmarshal([]byte(strategy.Config), &strategyConfig); err != nil {
			SafeBadRequest(c, "Failed to parse strategy config")
			return
		}
		cfg.SetLoadedStrategy(&strategyConfig)
		logger.Infof("📊 Backtest using saved strategy: %s (%s)", strategy.Name, strategy.ID)
		logger.Infof("📊 Strategy coin source: type=%s, use_ai500=%v, use_oi_top=%v, static_coins=%v",
			strategyConfig.CoinSource.SourceType,
			strategyConfig.CoinSource.UseAI500,
			strategyConfig.CoinSource.UseOITop,
			strategyConfig.CoinSource.StaticCoins)

		// If no symbols provided, fetch from strategy's coin source
		if len(cfg.Symbols) == 0 {
			symbols, err := s.resolveStrategyCoins(&strategyConfig)
			if err != nil {
				SafeBadRequest(c, "Failed to resolve coins from strategy")
				return
			}
			cfg.Symbols = symbols
			logger.Infof("📊 Resolved %d coins from strategy: %v", len(symbols), symbols)
		}
	}

	if err := s.hydrateBacktestAIConfig(&cfg); err != nil {
		SafeBadRequest(c, "Failed to configure AI model")
		return
	}

	logger.Infof("📊 Starting backtest with final config: runID=%s, symbols=%v (count=%d), strategyID=%s",
		cfg.RunID, cfg.Symbols, len(cfg.Symbols), cfg.StrategyID)

	runner, err := s.backtestManager.Start(context.Background(), cfg)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Failed to start backtest", err)
		return
	}

	meta := runner.CurrentMetadata()
	c.JSON(http.StatusOK, meta)
}

func (s *Server) handleBacktestPause(c *gin.Context) {
	s.handleBacktestControl(c, s.backtestManager.Pause)
}

func (s *Server) handleBacktestResume(c *gin.Context) {
	s.handleBacktestControl(c, s.backtestManager.Resume)
}

func (s *Server) handleBacktestStop(c *gin.Context) {
	s.handleBacktestControl(c, s.backtestManager.Stop)
}

func (s *Server) handleBacktestControl(c *gin.Context, fn func(string) error) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))

	var req runIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}
	if req.RunID == "" {
		SafeBadRequest(c, "run_id is required")
		return
	}

	if _, err := s.ensureBacktestRunOwnership(req.RunID, userID); writeBacktestAccessError(c, err) {
		return
	}

	if err := fn(req.RunID); err != nil {
		SafeError(c, http.StatusBadRequest, "Failed to execute backtest operation", err)
		return
	}

	meta, err := s.backtestManager.LoadMetadata(req.RunID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
		return
	}
	c.JSON(http.StatusOK, meta)
}

func (s *Server) handleBacktestLabel(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}
	var req labelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		SafeBadRequest(c, "run_id is required")
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))
	if _, err := s.ensureBacktestRunOwnership(req.RunID, userID); writeBacktestAccessError(c, err) {
		return
	}
	meta, err := s.backtestManager.UpdateLabel(req.RunID, req.Label)
	if err != nil {
		SafeInternalError(c, "Update backtest label", err)
		return
	}
	c.JSON(http.StatusOK, meta)
}

func (s *Server) handleBacktestDelete(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}
	var req runIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		SafeBadRequest(c, "run_id is required")
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))
	if _, err := s.ensureBacktestRunOwnership(req.RunID, userID); writeBacktestAccessError(c, err) {
		return
	}
	if err := s.backtestManager.Delete(req.RunID); err != nil {
		SafeInternalError(c, "Delete backtest run", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (s *Server) handleBacktestStatus(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}

	userID := normalizeUserID(c.GetString("user_id"))

	runID := c.Query("run_id")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}

	meta, err := s.ensureBacktestRunOwnership(runID, userID)
	if writeBacktestAccessError(c, err) {
		return
	}

	status := s.backtestManager.Status(runID)
	if status != nil {
		c.JSON(http.StatusOK, status)
		return
	}

	payload := backtest.StatusPayload{
		RunID:          meta.RunID,
		State:          meta.State,
		ProgressPct:    meta.Summary.ProgressPct,
		ProcessedBars:  meta.Summary.ProcessedBars,
		CurrentTime:    0,
		DecisionCycle:  meta.Summary.ProcessedBars,
		Equity:         meta.Summary.EquityLast,
		UnrealizedPnL:  0,
		RealizedPnL:    0,
		Note:           meta.Summary.LiquidationNote,
		LastUpdatedIso: meta.UpdatedAt.Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, payload)
}

func (s *Server) handleBacktestRuns(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}
	rawUserID := strings.TrimSpace(c.GetString("user_id"))
	userID := normalizeUserID(rawUserID)
	filterByUser := rawUserID != "" && rawUserID != "admin"

	metas, err := s.backtestManager.ListRuns()
	if err != nil {
		SafeInternalError(c, "List backtest runs", err)
		return
	}
	stateFilter := strings.ToLower(strings.TrimSpace(c.Query("state")))
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filtered := make([]*backtest.RunMetadata, 0, len(metas))
	for _, meta := range metas {
		if stateFilter != "" && !strings.EqualFold(string(meta.State), stateFilter) {
			continue
		}
		if search != "" {
			target := strings.ToLower(meta.RunID + " " + meta.Summary.DecisionTF + " " + meta.Label + " " + meta.LastError)
			if !strings.Contains(target, search) {
				continue
			}
		}
		if filterByUser {
			owner := strings.TrimSpace(meta.UserID)
			if owner != "" && owner != userID {
				continue
			}
		}
		filtered = append(filtered, meta)
	}

	total := len(filtered)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := filtered[start:end]

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"items": page,
	})
}

func (s *Server) handleBacktestEquity(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}

	userID := normalizeUserID(c.GetString("user_id"))

	runID := c.Query("run_id")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	if _, err := s.ensureBacktestRunOwnership(runID, userID); writeBacktestAccessError(c, err) {
		return
	}
	timeframe := c.Query("tf")
	limit := queryInt(c, "limit", 1000)

	points, err := s.backtestManager.LoadEquity(runID, timeframe, limit)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Failed to load equity data", err)
		return
	}
	c.JSON(http.StatusOK, points)
}

func (s *Server) handleBacktestTrades(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}

	userID := normalizeUserID(c.GetString("user_id"))

	runID := c.Query("run_id")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	if _, err := s.ensureBacktestRunOwnership(runID, userID); writeBacktestAccessError(c, err) {
		return
	}
	limit := queryInt(c, "limit", 1000)

	events, err := s.backtestManager.LoadTrades(runID, limit)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Failed to load trades", err)
		return
	}
	c.JSON(http.StatusOK, events)
}

func (s *Server) handleBacktestMetrics(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}

	userID := normalizeUserID(c.GetString("user_id"))

	runID := c.Query("run_id")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	if _, err := s.ensureBacktestRunOwnership(runID, userID); writeBacktestAccessError(c, err) {
		return
	}

	metrics, err := s.backtestManager.GetMetrics(runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusAccepted, gin.H{"error": "metrics not ready yet"})
			return
		}
		SafeError(c, http.StatusBadRequest, "Failed to load metrics", err)
		return
	}
	c.JSON(http.StatusOK, metrics)
}

func (s *Server) handleBacktestTrace(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))
	runID := c.Query("run_id")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	if _, err := s.ensureBacktestRunOwnership(runID, userID); writeBacktestAccessError(c, err) {
		return
	}
	cycle := queryInt(c, "cycle", 0)
	record, err := s.backtestManager.GetTrace(runID, cycle)
	if err != nil {
		SafeNotFound(c, "Trace record")
		return
	}
	c.JSON(http.StatusOK, record)
}

func (s *Server) handleBacktestDecisions(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))
	runID := c.Query("run_id")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	if _, err := s.ensureBacktestRunOwnership(runID, userID); writeBacktestAccessError(c, err) {
		return
	}
	limit := queryInt(c, "limit", 20)
	offset := queryInt(c, "offset", 0)
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	records, err := backtest.LoadDecisionRecords(runID, limit, offset)
	if err != nil {
		SafeInternalError(c, "Load decision records", err)
		return
	}
	c.JSON(http.StatusOK, records)
}

func (s *Server) handleBacktestExport(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))
	runID := c.Query("run_id")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	if _, err := s.ensureBacktestRunOwnership(runID, userID); writeBacktestAccessError(c, err) {
		return
	}
	path, err := s.backtestManager.ExportRun(runID)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Failed to export backtest", err)
		return
	}
	defer os.Remove(path)
	filename := fmt.Sprintf("%s_export.zip", runID)
	c.FileAttachment(path, filename)
}

func (s *Server) handleBacktestKlines(c *gin.Context) {
	if s.backtestManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backtest manager unavailable"})
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))
	runID := c.Query("run_id")
	symbol := c.Query("symbol")
	timeframe := c.Query("timeframe")

	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	meta, err := s.ensureBacktestRunOwnership(runID, userID)
	if writeBacktestAccessError(c, err) {
		return
	}

	// Load config to get time range
	cfg, err := backtest.LoadConfig(runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "failed to load backtest config"})
		return
	}

	// Use decision timeframe if not specified
	if timeframe == "" {
		timeframe = cfg.DecisionTimeframe
		if timeframe == "" {
			timeframe = "15m"
		}
	}

	// Fetch klines for the backtest time range
	startTime := time.Unix(cfg.StartTS, 0)
	endTime := time.Unix(cfg.EndTS, 0)

	klines, err := market.GetKlinesRange(symbol, timeframe, startTime, endTime)
	if err != nil {
		SafeInternalError(c, "Fetch klines", err)
		return
	}

	// Convert to response format
	type KlineResponse struct {
		Time   int64   `json:"time"`
		Open   float64 `json:"open"`
		High   float64 `json:"high"`
		Low    float64 `json:"low"`
		Close  float64 `json:"close"`
		Volume float64 `json:"volume"`
	}

	result := make([]KlineResponse, len(klines))
	for i, k := range klines {
		result[i] = KlineResponse{
			Time:   k.OpenTime / 1000, // Convert to seconds for lightweight-charts
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":    symbol,
		"timeframe": timeframe,
		"start_ts":  cfg.StartTS,
		"end_ts":    cfg.EndTS,
		"count":     len(result),
		"klines":    result,
		"run_id":    meta.RunID,
	})
}

func queryInt(c *gin.Context, name string, fallback int) int {
	if value := c.Query(name); value != "" {
		if v, err := strconv.Atoi(value); err == nil {
			return v
		}
	}
	return fallback
}

var errBacktestForbidden = errors.New("backtest run forbidden")

func normalizeUserID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "default"
	}
	return id
}

func (s *Server) ensureBacktestRunOwnership(runID, userID string) (*backtest.RunMetadata, error) {
	if s.backtestManager == nil {
		return nil, fmt.Errorf("backtest manager unavailable")
	}
	meta, err := s.backtestManager.LoadMetadata(runID)
	if err != nil {
		return nil, err
	}
	if userID == "" || userID == "admin" {
		return meta, nil
	}
	owner := strings.TrimSpace(meta.UserID)
	if owner == "" {
		return meta, nil
	}
	if owner != userID {
		return nil, errBacktestForbidden
	}
	return meta, nil
}

func writeBacktestAccessError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, errBacktestForbidden):
		SafeForbidden(c, "No permission to access this backtest task")
	case errors.Is(err, os.ErrNotExist), errors.Is(err, sql.ErrNoRows):
		SafeNotFound(c, "Backtest task")
	default:
		SafeInternalError(c, "Access backtest", err)
	}
	return true
}

// resolveStrategyCoins fetches coins based on strategy's coin source configuration
func (s *Server) resolveStrategyCoins(strategyConfig *store.StrategyConfig) ([]string, error) {
	if strategyConfig == nil {
		return nil, fmt.Errorf("strategy config is nil")
	}

	coinSource := strategyConfig.CoinSource
	var symbols []string
	symbolSet := make(map[string]bool)

	// Handle empty source_type - check flags for backward compatibility
	sourceType := coinSource.SourceType
	if sourceType == "" {
		if coinSource.UseAI500 && coinSource.UseOITop {
			sourceType = "mixed"
		} else if coinSource.UseAI500 {
			sourceType = "ai500"
		} else if coinSource.UseOITop {
			sourceType = "oi_top"
		} else if len(coinSource.StaticCoins) > 0 {
			sourceType = "static"
		} else {
			return nil, fmt.Errorf("strategy has no coin source configured")
		}
		logger.Infof("📊 Inferred source_type=%s from flags", sourceType)
	}

	switch sourceType {
	case "static":
		for _, sym := range coinSource.StaticCoins {
			sym = market.Normalize(sym)
			if !symbolSet[sym] {
				symbols = append(symbols, sym)
				symbolSet[sym] = true
			}
		}

	case "ai500":
		limit := coinSource.AI500Limit
		if limit <= 0 {
			limit = 30
		}
		logger.Infof("📊 Fetching AI500 coins with limit=%d", limit)
		coins, err := nofxos.DefaultClient().GetTopRatedCoins(limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get AI500 coins: %w", err)
		}
		logger.Infof("📊 Got %d coins from AI500: %v", len(coins), coins)
		for _, sym := range coins {
			sym = market.Normalize(sym)
			if !symbolSet[sym] {
				symbols = append(symbols, sym)
				symbolSet[sym] = true
			}
		}

	case "oi_top":
		coins, err := nofxos.DefaultClient().GetOITopSymbols()
		if err != nil {
			return nil, fmt.Errorf("failed to get OI Top coins: %w", err)
		}
		limit := coinSource.OITopLimit
		if limit <= 0 || limit > len(coins) {
			limit = len(coins)
		}
		for i, sym := range coins {
			if i >= limit {
				break
			}
			sym = market.Normalize(sym)
			if !symbolSet[sym] {
				symbols = append(symbols, sym)
				symbolSet[sym] = true
			}
		}

	case "mixed":
		// Get from AI500
		if coinSource.UseAI500 {
			limit := coinSource.AI500Limit
			if limit <= 0 {
				limit = 30
			}
			coins, err := nofxos.DefaultClient().GetTopRatedCoins(limit)
			if err != nil {
				logger.Warnf("Failed to get AI500 coins: %v", err)
			} else {
				for _, sym := range coins {
					sym = market.Normalize(sym)
					if !symbolSet[sym] {
						symbols = append(symbols, sym)
						symbolSet[sym] = true
					}
				}
			}
		}

		// Get from OI Top
		if coinSource.UseOITop {
			coins, err := nofxos.DefaultClient().GetOITopSymbols()
			if err != nil {
				logger.Warnf("Failed to get OI Top coins: %v", err)
			} else {
				limit := coinSource.OITopLimit
				if limit <= 0 || limit > len(coins) {
					limit = len(coins)
				}
				for i, sym := range coins {
					if i >= limit {
						break
					}
					sym = market.Normalize(sym)
					if !symbolSet[sym] {
						symbols = append(symbols, sym)
						symbolSet[sym] = true
					}
				}
			}
		}

		// Add static coins
		for _, sym := range coinSource.StaticCoins {
			sym = market.Normalize(sym)
			if !symbolSet[sym] {
				symbols = append(symbols, sym)
				symbolSet[sym] = true
			}
		}

	default:
		return nil, fmt.Errorf("unknown coin source type: %s", sourceType)
	}

	if len(symbols) == 0 {
		return nil, fmt.Errorf("no coins resolved from strategy")
	}

	logger.Infof("📊 Final resolved symbols: %d coins - %v", len(symbols), symbols)
	return symbols, nil
}

func (s *Server) resolveBacktestAIConfig(cfg *backtest.BacktestConfig, userID string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if s.store == nil {
		return fmt.Errorf("System database not ready, cannot load AI model configuration")
	}

	cfg.UserID = normalizeUserID(userID)

	return s.hydrateBacktestAIConfig(cfg)
}

func (s *Server) hydrateBacktestAIConfig(cfg *backtest.BacktestConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if s.store == nil {
		return fmt.Errorf("System database not ready, cannot load AI model configuration")
	}

	cfg.UserID = normalizeUserID(cfg.UserID)
	modelID := strings.TrimSpace(cfg.AIModelID)

	var (
		model *store.AIModel
		err   error
	)

	if modelID != "" {
		model, err = s.store.AIModel().Get(cfg.UserID, modelID)
		if err != nil {
			return fmt.Errorf("Failed to load AI model: %w", err)
		}
	} else {
		model, err = s.store.AIModel().GetDefault(cfg.UserID)
		if err != nil {
			return fmt.Errorf("No available AI model found: %w", err)
		}
		cfg.AIModelID = model.ID
	}

	if !model.Enabled {
		return fmt.Errorf("AI model %s is not enabled yet", model.Name)
	}

	apiKey := strings.TrimSpace(string(model.APIKey))
	if apiKey == "" {
		return fmt.Errorf("AI model %s is missing API Key, please configure it in the system first", model.Name)
	}

	provider := strings.ToLower(strings.TrimSpace(model.Provider))
	// Ensure provider is never empty or "inherit" - infer from model name if needed
	if provider == "" || provider == "inherit" {
		modelNameLower := strings.ToLower(model.Name)
		if strings.Contains(modelNameLower, "claude") || strings.Contains(modelNameLower, "anthropic") {
			provider = "anthropic"
		} else if strings.Contains(modelNameLower, "gpt") || strings.Contains(modelNameLower, "openai") {
			provider = "openai"
		} else if strings.Contains(modelNameLower, "gemini") || strings.Contains(modelNameLower, "google") {
			provider = "google"
		} else if strings.Contains(modelNameLower, "deepseek") {
			provider = "deepseek"
		} else if model.CustomAPIURL != "" {
			provider = "custom"
		} else {
			provider = "openai" // default fallback
		}
		logger.Infof("📊 Inferred AI provider '%s' from model name '%s'", provider, model.Name)
	}
	cfg.AICfg.Provider = provider
	cfg.AICfg.APIKey = apiKey
	cfg.AICfg.BaseURL = strings.TrimSpace(model.CustomAPIURL)
	modelName := strings.TrimSpace(model.CustomModelName)
	if cfg.AICfg.Model == "" {
		cfg.AICfg.Model = modelName
	}
	cfg.AICfg.Model = strings.TrimSpace(cfg.AICfg.Model)

	if cfg.AICfg.Provider == "custom" {
		if cfg.AICfg.BaseURL == "" {
			return fmt.Errorf("Custom AI model requires API URL configuration")
		}
		if cfg.AICfg.Model == "" {
			return fmt.Errorf("Custom AI model requires model name configuration")
		}
	}

	return nil
}
