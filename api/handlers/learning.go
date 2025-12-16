package handlers

import (
	"net/http"
	"nofx/config"
	"nofx/decision/analysis"
	"time"

	"github.com/gin-gonic/gin"
)

type LearningHandler struct {
	BaseHandler
	analyzer *analysis.TradeAnalyzer
}

func NewLearningHandler(db *config.Database) *LearningHandler {
	return &LearningHandler{
		BaseHandler: BaseHandler{Database: db},
		analyzer:    analysis.NewTradeAnalyzer(db),
	}
}

// HandleGetAnalysis handles GET /api/traders/:id/analysis
func (h *LearningHandler) HandleGetAnalysis(c *gin.Context) {
	traderID := c.Param("id")
	periodStr := c.Query("period")

	// Default period: 7d
	duration := 7 * 24 * time.Hour
	if periodStr == "1d" {
		duration = 24 * time.Hour
	} else if periodStr == "30d" {
		duration = 30 * 24 * time.Hour
	} else if periodStr == "90d" {
		duration = 90 * 24 * time.Hour
	}

	endDate := time.Now()
	startDate := endDate.Add(-duration)

	// Analyze
	result, err := h.analyzer.AnalyzeTradesForPeriod(traderID, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleGetReflections handles GET /api/traders/:id/reflections
func (h *LearningHandler) HandleGetReflections(c *gin.Context) {
	traderID := c.Param("id")

	reflections, err := h.Database.GetReflections(traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reflections": reflections,
		"total":       len(reflections),
	})
}
