package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// handleStatisticsFull returns the full set of computed performance metrics for
// a single trader: win rate, profit factor, Sharpe ratio, max drawdown, and the
// average win/loss amounts. These are derived from the trader's CLOSED positions
// via store.Position().GetFullStatsByTraderFilters — the same computation the
// strategy engine feeds to the AI, so the dashboard and the model see identical
// numbers.
//
// The existing GET /statistics endpoint only returns cycle/position counts; this
// endpoint exposes the richer trade-quality metrics the terminal dashboard needs.
func (s *Server) handleStatisticsFull(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		SafeBadRequest(c, "Invalid trader ID")
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		SafeNotFound(c, "Trader")
		return
	}

	store := trader.GetStore()
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Store not available"})
		return
	}

	// Aggregate across the trader's historical IDs exactly like the position
	// history endpoint (handler_order.go). One-click "NOFX Autopilot" relaunches
	// create fresh trader rows, but the closed positions stay under the old
	// generated IDs (which embed userID + "claw402"). Without this, a freshly
	// relaunched Autopilot would report only the current incarnation's trades
	// instead of its real lifetime history.
	userID := c.GetString("user_id")
	traderIDs := []string{trader.GetID()}
	var traderIDPatterns []string
	if strings.EqualFold(strings.TrimSpace(trader.GetName()), "NOFX Autopilot") && strings.TrimSpace(userID) != "" {
		traderIDPatterns = append(traderIDPatterns, "%_"+userID+"_claw402_%")
	}

	stats, err := store.Position().GetFullStatsByTraderFilters(traderIDs, traderIDPatterns, trader.GetInitialBalance())
	if err != nil {
		SafeInternalError(c, "Get full statistics", err)
		return
	}

	c.JSON(http.StatusOK, stats)
}
