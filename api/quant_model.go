package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleListQuantModels lists all quant models for the user
func (s *Server) handleListQuantModels(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	models, err := s.store.QuantModel().List(userID)
	if err != nil {
		SafeInternalError(c, "Failed to list quant models", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// handleListPublicQuantModels lists all public quant models
func (s *Server) handleListPublicQuantModels(c *gin.Context) {
	models, err := s.store.QuantModel().ListPublic()
	if err != nil {
		SafeInternalError(c, "Failed to list public quant models", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// handleGetQuantModel gets a single quant model
func (s *Server) handleGetQuantModel(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modelID := c.Param("id")

	model, err := s.store.QuantModel().Get(userID, modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	c.JSON(http.StatusOK, model)
}

// handleCreateQuantModel creates a new quant model
func (s *Server) handleCreateQuantModel(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name        string                  `json:"name" binding:"required"`
		Description string                  `json:"description"`
		ModelType   string                  `json:"model_type" binding:"required"`
		IsPublic    bool                    `json:"is_public"`
		Config      *store.QuantModelConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Use default config if not provided
	config := req.Config
	if config == nil {
		config = store.GetDefaultQuantModelConfig()
	}

	configData, err := json.Marshal(config)
	if err != nil {
		SafeInternalError(c, "Failed to serialize config", err)
		return
	}

	model := &store.QuantModel{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		ModelType:   req.ModelType,
		IsPublic:    req.IsPublic,
		IsActive:    true,
		Config:      string(configData),
	}

	if err := s.store.QuantModel().Create(model); err != nil {
		SafeInternalError(c, "Failed to create quant model", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      model.ID,
		"message": "Quant model created successfully",
	})
}

// handleUpdateQuantModel updates an existing quant model
func (s *Server) handleUpdateQuantModel(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modelID := c.Param("id")

	// Verify model exists and belongs to user
	existing, err := s.store.QuantModel().Get(userID, modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	var req struct {
		Name        string                  `json:"name"`
		Description string                  `json:"description"`
		ModelType   string                  `json:"model_type"`
		IsPublic    bool                    `json:"is_public"`
		IsActive    bool                    `json:"is_active"`
		Config      *store.QuantModelConfig `json:"config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	// Update fields
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.ModelType != "" {
		existing.ModelType = req.ModelType
	}
	existing.IsPublic = req.IsPublic
	existing.IsActive = req.IsActive

	// Update config if provided
	if req.Config != nil {
		configData, err := json.Marshal(req.Config)
		if err != nil {
			SafeInternalError(c, "Failed to serialize config", err)
			return
		}
		existing.Config = string(configData)
	}

	if err := s.store.QuantModel().Update(existing); err != nil {
		SafeInternalError(c, "Failed to update quant model", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Quant model updated successfully",
	})
}

// handleDeleteQuantModel deletes a quant model
func (s *Server) handleDeleteQuantModel(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modelID := c.Param("id")

	if err := s.store.QuantModel().Delete(userID, modelID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": SanitizeError(err, "Failed to delete model")})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Quant model deleted successfully",
	})
}

// handleExportQuantModel exports a quant model to JSON
func (s *Server) handleExportQuantModel(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modelID := c.Param("id")

	model, err := s.store.QuantModel().Get(userID, modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	exportData, err := model.ToExportFormat()
	if err != nil {
		SafeInternalError(c, "Failed to export model", err)
		return
	}

	c.JSON(http.StatusOK, exportData)
}

// handleImportQuantModel imports a quant model from JSON
func (s *Server) handleImportQuantModel(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var importData map[string]interface{}
	if err := c.ShouldBindJSON(&importData); err != nil {
		SafeBadRequest(c, "Invalid import data")
		return
	}

	// Validate version
	version, ok := importData["version"].(string)
	if !ok || version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid export format: missing version"})
		return
	}

	if version != "1.0" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       fmt.Sprintf("Unsupported export version: %s", version),
			"supported":   []string{"1.0"},
		})
		return
	}

	model, err := store.ImportFromExport(importData, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set a default name if empty
	if model.Name == "" || model.Name == "Imported Model" {
		model.Name = fmt.Sprintf("Imported Model %s", uuid.New().String()[:8])
	}

	if err := s.store.QuantModel().Create(model); err != nil {
		SafeInternalError(c, "Failed to import model", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      model.ID,
		"name":    model.Name,
		"message": "Model imported successfully",
	})
}

// handleCloneQuantModel clones an existing model (public or user's own)
func (s *Server) handleCloneQuantModel(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modelID := c.Param("id")

	var req struct {
		Name string `json:"name"`
	}
	c.ShouldBindJSON(&req) // Name is optional

	// Get the source model
	source, err := s.store.QuantModel().Get(userID, modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	// Create a new model based on the source
	newName := req.Name
	if newName == "" {
		newName = fmt.Sprintf("%s (Clone)", source.Name)
	}

	// Parse and re-serialize config
	config, err := source.ParseConfig()
	if err != nil {
		SafeInternalError(c, "Failed to parse source config", err)
		return
	}

	newModel := &store.QuantModel{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        newName,
		Description: fmt.Sprintf("Cloned from %s", source.Name),
		ModelType:   source.ModelType,
		IsPublic:    false,
		IsActive:    true,
	}

	if err := newModel.SetConfig(config); err != nil {
		SafeInternalError(c, "Failed to set config", err)
		return
	}

	if err := s.store.QuantModel().Create(newModel); err != nil {
		SafeInternalError(c, "Failed to clone model", err)
		return
	}

	// Increment usage count on the original model
	s.store.QuantModel().IncrementUsage(modelID)

	c.JSON(http.StatusOK, gin.H{
		"id":      newModel.ID,
		"name":    newModel.Name,
		"message": "Model cloned successfully",
	})
}

// handleGetQuantModelTemplates returns predefined model templates
func (s *Server) handleGetQuantModelTemplates(c *gin.Context) {
	templates := []gin.H{
		{
			"id":          "template_indicator_based",
			"name":        "Indicator-Based Model",
			"description": "Uses weighted combination of technical indicators (RSI, EMA, MACD)",
			"model_type":  "indicator_based",
			"config":      store.GetDefaultQuantModelConfig(),
		},
		{
			"id":          "template_rule_based",
			"name":        "Rule-Based Model",
			"description": "Uses explicit trading rules with conditions and actions",
			"model_type":  "rule_based",
			"config":      store.GetExampleRuleBasedConfig(),
		},
	}

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// handleUpdateBacktestStats updates backtest statistics for a model
func (s *Server) handleUpdateBacktestStats(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	modelID := c.Param("id")

	// Verify model ownership
	_, err := s.store.QuantModel().Get(userID, modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	var req struct {
		WinRate         float64 `json:"win_rate"`
		AvgProfitPct    float64 `json:"avg_profit_pct"`
		MaxDrawdownPct  float64 `json:"max_drawdown_pct"`
		SharpeRatio     float64 `json:"sharpe_ratio"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}

	stats := store.BacktestStats{
		WinRate:        req.WinRate,
		AvgProfitPct:   req.AvgProfitPct,
		MaxDrawdownPct: req.MaxDrawdownPct,
		SharpeRatio:    req.SharpeRatio,
	}

	if err := s.store.QuantModel().UpdateBacktestStats(modelID, stats); err != nil {
		SafeInternalError(c, "Failed to update backtest stats", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Backtest statistics updated successfully",
	})
}
