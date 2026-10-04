package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"nofx/logger"
	"nofx/provider/vergex"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleVergexDirectionChangeLeaderboard(c *gin.Context) {
	client, ok := s.newVergexClientForRequest(c)
	if !ok {
		return
	}
	data, err := client.GetDirectionChangeLeaderboard(c.Request.Context())
	if err != nil {
		logger.Warnf("Vergex direction-change leaderboard failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", data.Raw)
}

func (s *Server) handleVergexDirectionChangeCurrent(c *gin.Context) {
	client, ok := s.newVergexClientForRequest(c)
	if !ok {
		return
	}
	symbol := strings.TrimSpace(c.Query("symbol"))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	body, err := client.GetDirectionChangeCurrent(c.Request.Context(), symbol)
	if err != nil {
		logger.Warnf("Vergex direction-change current failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

func (s *Server) handleVergexDirectionChangeHistory(c *gin.Context) {
	client, ok := s.newVergexClientForRequest(c)
	if !ok {
		return
	}
	symbol := strings.TrimSpace(c.Query("symbol"))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}
	body, err := client.GetDirectionChangeHistory(
		c.Request.Context(), symbol, strings.TrimSpace(c.Query("type")),
		parsePositiveInt(c.Query("page"), 1), parsePositiveInt(c.Query("page_size"), 20),
	)
	if err != nil {
		if strings.Contains(err.Error(), "type must be") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logger.Warnf("Vergex direction-change history failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

func (s *Server) handleVergexCostLiquidationHeatmap(c *gin.Context) {
	client, ok := s.newVergexClientForRequest(c)
	if !ok {
		return
	}
	body, err := client.GetCostLiquidationHeatmap(c.Request.Context(), vergex.Query{
		MarketType: withDefault(strings.TrimSpace(c.Query("marketType")), vergex.DefaultMarketType),
		Symbol:     strings.TrimSpace(c.Query("symbol")),
		Chain:      strings.TrimSpace(c.Query("chain")),
		LiqBand:    strings.TrimSpace(c.Query("liqBand")),
	})
	if err != nil {
		logger.Warnf("Vergex cost-liquidation-heatmap failed: %v", err)
		if isWinrateValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// handleVergexFlowMarkets proxies the Vergex net-flow market ranking (paid x402
// endpoint) using the caller's claw402 wallet. The upstream JSON is passed
// through verbatim: { data: { window, by, inflow: [{ symbol, netFlow,
// buyNotional, sellNotional, trades, latestPrice }, ...] } }.
func (s *Server) handleVergexFlowMarkets(c *gin.Context) {
	client, ok := s.newVergexClientForRequest(c)
	if !ok {
		return
	}
	chain := withDefault(strings.TrimSpace(c.Query("chain")), "mainnet")
	window := withDefault(strings.TrimSpace(c.Query("window")), "1h")
	limit := parsePositiveInt(c.Query("limit"), 25)

	body, err := client.GetFlowMarkets(context.Background(), chain, window, limit)
	if err != nil {
		logger.Warnf("Vergex flow-markets failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// handleVergexHolderWinrateMap proxies the Vergex holder win-rate matrix (paid
// x402 endpoint) using the caller's claw402 wallet. The upstream JSON is
// passed through verbatim: { data: { snapshotId, markPrice, viewport, winBins,
// costBins, costRange, cells: [{ row, column, long, short }], water, included,
// excluded, ... }, meta }. cells are winBins rows × (costBins+2) slots where
// slot 0 = below the cost viewport and slot costBins+1 = above it.
func (s *Server) handleVergexHolderWinrateMap(c *gin.Context) {
	q, err := parseWinrateQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client, ok := s.newVergexClientForRequest(c)
	if !ok {
		return
	}
	body, err := client.GetHolderWinrateMap(c.Request.Context(), q)
	if err != nil {
		if isWinrateValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logger.Warnf("Vergex holder-winrate-map failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// handleVergexHolderWinrateHolders paginates the addresses behind one cell
// rectangle of the win-rate matrix (paid x402 endpoint). Requires snapshotId
// (from the map response) plus the row/column range; side is long|short.
func (s *Server) handleVergexHolderWinrateHolders(c *gin.Context) {
	q, err := parseWinrateHoldersQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	client, ok := s.newVergexClientForRequest(c)
	if !ok {
		return
	}
	body, err := client.GetHolderWinrateHolders(c.Request.Context(), q)
	if err != nil {
		if isWinrateValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logger.Warnf("Vergex holder-winrate-holders failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

func parseWinrateQuery(c *gin.Context) (vergex.WinrateQuery, error) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return vergex.WinrateQuery{}, fmt.Errorf("invalid query encoding")
	}
	q := vergex.WinrateQuery{
		MarketType: strings.TrimSpace(c.Query("marketType")),
		Symbol:     strings.TrimSpace(c.Query("symbol")),
		Chain:      strings.TrimSpace(c.Query("chain")),
	}
	// Reject duplicate values instead of validating one and forwarding another.
	for key, entries := range values {
		if len(entries) != 1 {
			return q, fmt.Errorf("%s must be provided only once", key)
		}
	}
	for _, pair := range [][2]string{{"winMin", "winMax"}, {"costMin", "costMax"}} {
		if values.Has(pair[0]) != values.Has(pair[1]) {
			return q, fmt.Errorf("%s and %s must be supplied together", pair[0], pair[1])
		}
	}
	var costMin, costMax int
	for _, field := range []struct {
		key                string
		target             *int
		fallback, min, max int
	}{
		{"winMin", &q.WinMin, 0, 0, 99}, {"winMax", &q.WinMax, 0, 1, 100},
		{"costMin", &costMin, 0, 1, 9999}, {"costMax", &costMax, 0, 2, 10000},
		{"minRoundTrips", &q.MinRoundTrips, 1, 1, 10000},
	} {
		value, err := parseWinrateInt(c, field.key, field.fallback, field.min, field.max)
		if err != nil {
			return q, err
		}
		*field.target = value
	}
	q.CostMin, q.CostMax = float64(costMin), float64(costMax)
	return q, q.Validate()
}

func parseWinrateHoldersQuery(c *gin.Context) (vergex.WinrateHoldersQuery, error) {
	base, err := parseWinrateQuery(c)
	q := vergex.WinrateHoldersQuery{WinrateQuery: base,
		SnapshotID: strings.TrimSpace(c.Query("snapshotId")), Side: strings.TrimSpace(c.Query("side"))}
	if err != nil {
		return q, err
	}
	for _, field := range []struct {
		key                string
		target             *int
		fallback, min, max int
	}{
		{"row", &q.Row, -1, 0, 19}, {"rowEnd", &q.RowEnd, -1, 0, 19},
		{"column", &q.Column, -1, 0, 17}, {"columnEnd", &q.ColumnEnd, -1, 0, 17},
		{"offset", &q.Offset, 0, 0, 1000000}, {"limit", &q.Limit, 50, 1, 100},
	} {
		value, err := parseWinrateInt(c, field.key, field.fallback, field.min, field.max)
		if err != nil {
			return q, err
		}
		*field.target = value
	}
	return q, q.Validate()
}

func isWinrateValidationError(err error) bool {
	var validation *vergex.WinrateValidationError
	return errors.As(err, &validation)
}

func parseWinrateInt(c *gin.Context, key string, fallback, min, max int) (int, error) {
	raw, present := c.GetQuery(key)
	// GetQuery treats empty as absent; URL.Has distinguishes an invalid empty value.
	if !present && !c.Request.URL.Query().Has(key) {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", key, min, max)
	}
	return n, nil
}

func (s *Server) newVergexClientForRequest(c *gin.Context) (*vergex.Client, bool) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil, false
	}
	walletKey, err := s.resolveStrategyDataWalletKey(userID, c.Query("ai_model_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, false
	}
	if walletKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "claw402 wallet is not configured"})
		return nil, false
	}
	client, err := vergex.NewClient("", walletKey, &logger.MCPLogger{})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, false
	}
	return client, true
}

func parsePositiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	return n
}

func withDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
