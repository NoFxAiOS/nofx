package api

import (
	"net/http"
	"nofx/logger"

	"github.com/gin-gonic/gin"
)

// handleGetAICosts returns AI charges for a specific trader
func (s *Server) handleGetAICosts(c *gin.Context) {
	traderID := c.Query("trader_id")
	period := c.DefaultQuery("period", "today")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id is required"})
		return
	}

	charges, total, err := s.store.AICharge().GetCharges(traderID, period)
	if err != nil {
		logger.Infof("❌ Failed to get AI charges for trader %s: %v", traderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve AI cost data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"charges": charges,
		"total":   total,
		"count":   len(charges),
	})
}

// handleGetAICostsSummary returns AI cost summary across all traders
func (s *Server) handleGetAICostsSummary(c *gin.Context) {
	period := c.DefaultQuery("period", "today")

	total, count, byModel := s.store.AICharge().GetSummary(period)

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"count":    count,
		"by_model": byModel,
	})
}
