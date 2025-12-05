package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"nofx/backtest"
	"nofx/decision"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := req.Config
	if cfg.RunID == "" {
		cfg.RunID = "bt_" + time.Now().UTC().Format("20060102_150405")
	}
	cfg.PromptTemplate = strings.TrimSpace(cfg.PromptTemplate)
	if cfg.PromptTemplate == "" {
		cfg.PromptTemplate = "default"
	}
	if _, err := decision.GetPromptTemplate(cfg.PromptTemplate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("提示词模板不存在: %s", cfg.PromptTemplate)})
		return
	}
	cfg.CustomPrompt = strings.TrimSpace(cfg.CustomPrompt)
	cfg.UserID = normalizeUserID(c.GetString("user_id"))
	if err := s.hydrateBacktestAIConfig(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	runner, err := s.backtestManager.Start(context.Background(), cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RunID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}

	if _, err := s.ensureBacktestRunOwnership(req.RunID, userID); writeBacktestAccessError(c, err) {
		return
	}

	if err := fn(req.RunID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))
	if _, err := s.ensureBacktestRunOwnership(req.RunID, userID); writeBacktestAccessError(c, err) {
		return
	}
	meta, err := s.backtestManager.UpdateLabel(req.RunID, req.Label)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.RunID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run_id is required"})
		return
	}
	userID := normalizeUserID(c.GetString("user_id"))
	if _, err := s.ensureBacktestRunOwnership(req.RunID, userID); writeBacktestAccessError(c, err) {
		return
	}
	if err := s.backtestManager.Delete(req.RunID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer os.Remove(path)
	filename := fmt.Sprintf("%s_export.zip", runID)
	c.FileAttachment(path, filename)
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
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该回测任务"})
	case errors.Is(err, os.ErrNotExist), errors.Is(err, sql.ErrNoRows):
		c.JSON(http.StatusNotFound, gin.H{"error": "回测任务不存在"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	return true
}

func (s *Server) resolveBacktestAIConfig(cfg *backtest.BacktestConfig, userID string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if s.store == nil {
		return fmt.Errorf("系统数据库未就绪，无法加载AI模型配置")
	}

	cfg.UserID = normalizeUserID(userID)

	return s.hydrateBacktestAIConfig(cfg)
}

func (s *Server) hydrateBacktestAIConfig(cfg *backtest.BacktestConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if s.store == nil {
		return fmt.Errorf("系统数据库未就绪，无法加载AI模型配置")
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
			return fmt.Errorf("加载AI模型失败: %w", err)
		}
	} else {
		model, err = s.store.AIModel().GetDefault(cfg.UserID)
		if err != nil {
			return fmt.Errorf("未找到可用的AI模型: %w", err)
		}
		cfg.AIModelID = model.ID
	}

	if !model.Enabled {
		return fmt.Errorf("AI模型 %s 尚未启用", model.Name)
	}

	apiKey := strings.TrimSpace(model.APIKey)
	if apiKey == "" {
		return fmt.Errorf("AI模型 %s 缺少API Key，请先在系统中配置", model.Name)
	}

	cfg.AICfg.Provider = strings.ToLower(model.Provider)
	cfg.AICfg.APIKey = apiKey
	cfg.AICfg.BaseURL = strings.TrimSpace(model.CustomAPIURL)
	modelName := strings.TrimSpace(model.CustomModelName)
	if cfg.AICfg.Model == "" {
		cfg.AICfg.Model = modelName
	}
	cfg.AICfg.Model = strings.TrimSpace(cfg.AICfg.Model)

	if cfg.AICfg.Provider == "custom" {
		if cfg.AICfg.BaseURL == "" {
			return fmt.Errorf("自定义AI模型需要配置 API 地址")
		}
		if cfg.AICfg.Model == "" {
			return fmt.Errorf("自定义AI模型需要配置模型名称")
		}
	}

	return nil
}
