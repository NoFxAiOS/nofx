package api

import (
	"log"
	"net/http"
	"nofx/analytics"
	"nofx/logger"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGetCorrelationMatrix 获取相关性矩阵
// GET /api/analytics/correlation?trader_id=xxx&symbols=BTC,ETH,SOL&timeframe=1h
func (s *Server) handleGetCorrelationMatrix(c *gin.Context) {
	traderID := c.Query("trader_id")
	symbolsStr := c.Query("symbols")
	timeframe := c.DefaultQuery("timeframe", "1h")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id required"})
		return
	}

	if symbolsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbols required (comma-separated)"})
		return
	}

	symbols := strings.Split(symbolsStr, ",")
	if len(symbols) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要2个交易对"})
		return
	}

	// 从历史数据获取价格序列
	histories, err := analytics.GetHistoricalPrices(traderID, symbols, 60) // 60分钟lookback
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 计算相关性矩阵
	matrix, err := analytics.CalculateCorrelationMatrix(histories, timeframe)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, matrix)
}

// handleGetDrawdownAnalysis 获取回撤分析
// GET /api/analytics/drawdown?trader_id=xxx
func (s *Server) handleGetDrawdownAnalysis(c *gin.Context) {
	traderID := c.Query("trader_id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id required"})
		return
	}

	// 获取交易员
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 获取净值历史
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取净值历史失败: " + err.Error()})
		return
	}

	if len(records) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "暂无历史数据",
			"data":    nil,
		})
		return
	}

	// 转换为EquityPoint格式
	equityPoints := make([]analytics.EquityPoint, len(records))
	for i, record := range records {
		equityPoints[i] = analytics.EquityPoint{
			Timestamp:   record.Timestamp,
			Equity:      record.AccountState.TotalBalance,
			CycleNumber: record.CycleNumber,
		}
	}

	// 计算回撤分析
	drawdown, err := analytics.CalculateDrawdown(equityPoints)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, drawdown)
}

// handleRunMonteCarlo 运行Monte Carlo模拟
// POST /api/analytics/montecarlo
// Body: {"trader_id": "xxx", "simulations": 1000, "time_horizon_days": 30, "include_paths": false}
func (s *Server) handleRunMonteCarlo(c *gin.Context) {
	var req struct {
		TraderID         string `json:"trader_id"`
		Simulations      int    `json:"simulations"`
		TimeHorizonDays  int    `json:"time_horizon_days"`
		IncludePaths     bool   `json:"include_paths"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TraderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id required"})
		return
	}

	// 默认值
	if req.Simulations == 0 {
		req.Simulations = 1000
	}
	if req.TimeHorizonDays == 0 {
		req.TimeHorizonDays = 30
	}

	// 获取交易员当前余额
	trader, err := s.traderManager.GetTrader(req.TraderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	balance, err := trader.GetBalance()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取余额失败: " + err.Error()})
		return
	}

	// 🔒 安全的类型断言，防止 panic
	initialBalance, ok := balance["totalWalletBalance"].(float64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取余额数据格式错误"})
		return
	}

	// 获取净值历史用于估计参数
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil || len(records) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "历史数据不足，无法进行模拟"})
		return
	}

	// 转换为EquityPoint
	equityPoints := make([]analytics.EquityPoint, len(records))
	for i, record := range records {
		equityPoints[i] = analytics.EquityPoint{
			Timestamp:   record.Timestamp,
			Equity:      record.AccountState.TotalBalance,
			CycleNumber: record.CycleNumber,
		}
	}

	// 估计策略参数
	params, err := analytics.EstimateStrategyParams(equityPoints)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "估计策略参数失败: " + err.Error()})
		return
	}

	// 运行Monte Carlo模拟
	result, err := analytics.RunMonteCarlo(initialBalance, params, req.Simulations, req.TimeHorizonDays, req.IncludePaths)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleGetPerformanceAttribution 获取绩效归因分析
// GET /api/analytics/attribution?trader_id=xxx&lookback_days=30
func (s *Server) handleGetPerformanceAttribution(c *gin.Context) {
	traderID := c.Query("trader_id")
	lookbackDaysStr := c.DefaultQuery("lookback_days", "30")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id required"})
		return
	}

	lookbackDays, err := strconv.Atoi(lookbackDaysStr)
	if err != nil || lookbackDays <= 0 {
		lookbackDays = 30
	}

	// 获取交易员
	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 获取交易历史（从决策日志中提取）
	cutoffTime := time.Now().AddDate(0, 0, -lookbackDays)
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易历史失败: " + err.Error()})
		return
	}

	// 过滤时间范围内的决策
	filteredDecisions := []*logger.DecisionRecord{}
	for _, record := range records {
		if record.Timestamp.After(cutoffTime) {
			filteredDecisions = append(filteredDecisions, record)
		}
	}

	if len(filteredDecisions) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "指定时间范围内无交易记录",
			"data":    nil,
		})
		return
	}

	// 转换为TradeRecord格式（配对开仓和平仓决策）
	trades := extractTradesFromDecisions(filteredDecisions)

	// 如果没有完整交易记录，返回空结果
	if len(trades) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "暂无完整交易记录用于归因分析",
			"data":    nil,
		})
		return
	}

	// 计算绩效归因
	attribution, err := analytics.CalculatePerformanceAttribution(trades)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, attribution)
}

// handleGetOrderBook 获取实时订单簿
// GET /api/analytics/orderbook?symbol=BTCUSDT&depth=20
func (s *Server) handleGetOrderBook(c *gin.Context) {
	symbol := c.Query("symbol")
	depthStr := c.DefaultQuery("depth", "20")

	if symbol == "" {
		log.Printf("❌ Order Book: symbol参数缺失")
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol required"})
		return
	}

	depth, err := strconv.Atoi(depthStr)
	if err != nil || depth <= 0 {
		depth = 20
	}

	log.Printf("📊 获取订单簿: symbol=%s, depth=%d", symbol, depth)

	// 获取订单簿
	orderBook, err := analytics.FetchOrderBook(symbol, depth)
	if err != nil {
		log.Printf("❌ 获取订单簿失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ 订单簿获取成功: symbol=%s, bids=%d, asks=%d", symbol, len(orderBook.Bids), len(orderBook.Asks))
	c.JSON(http.StatusOK, orderBook)
}

// handleGetOrderBookDepthChart 获取订单簿深度图数据
// GET /api/analytics/orderbook/depth-chart?symbol=BTCUSDT&max_levels=50
func (s *Server) handleGetOrderBookDepthChart(c *gin.Context) {
	symbol := c.Query("symbol")
	maxLevelsStr := c.DefaultQuery("max_levels", "50")

	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol required"})
		return
	}

	maxLevels, err := strconv.Atoi(maxLevelsStr)
	if err != nil || maxLevels <= 0 {
		maxLevels = 50
	}

	// 获取订单簿
	orderBook, err := analytics.FetchOrderBook(symbol, maxLevels)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 生成深度图数据
	depthChart := analytics.GenerateDepthChart(orderBook, maxLevels)

	c.JSON(http.StatusOK, depthChart)
}

// handleGetOrderBookImbalance 获取订单簿失衡分析
// GET /api/analytics/orderbook/imbalance?symbol=BTCUSDT&top_n=10
func (s *Server) handleGetOrderBookImbalance(c *gin.Context) {
	symbol := c.Query("symbol")
	topNStr := c.DefaultQuery("top_n", "10")

	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol required"})
		return
	}

	topN, err := strconv.Atoi(topNStr)
	if err != nil || topN <= 0 {
		topN = 10
	}

	// 获取订单簿
	orderBook, err := analytics.FetchOrderBook(symbol, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 分析失衡
	imbalance := analytics.AnalyzeOrderBookImbalance(orderBook, topN)

	c.JSON(http.StatusOK, imbalance)
}

// handleDetectLargeOrders 检测大单
// GET /api/analytics/orderbook/large-orders?symbol=BTCUSDT&multiplier=3.0
func (s *Server) handleDetectLargeOrders(c *gin.Context) {
	symbol := c.Query("symbol")
	multiplierStr := c.DefaultQuery("multiplier", "3.0")

	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol required"})
		return
	}

	multiplier, err := strconv.ParseFloat(multiplierStr, 64)
	if err != nil || multiplier <= 0 {
		multiplier = 3.0
	}

	// 获取订单簿
	orderBook, err := analytics.FetchOrderBook(symbol, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 检测大单
	largeOrders := analytics.DetectLargeOrders(orderBook, multiplier)

	c.JSON(http.StatusOK, largeOrders)
}

// extractTradesFromDecisions 从决策记录中提取完整交易
// 将开仓和平仓决策配对成完整的交易记录
func extractTradesFromDecisions(records []*logger.DecisionRecord) []analytics.TradeRecord {
	trades := []analytics.TradeRecord{}

	// 使用map跟踪每个symbol的开仓位置
	// key: symbol_side (e.g., "BTCUSDT_long")
	openPositions := make(map[string]struct {
		entryTime  time.Time
		entryPrice float64
		side       string
		symbol     string
	})

	for _, record := range records {
		for _, decision := range record.Decisions {
			if !decision.Success {
				continue // 跳过失败的决策
			}

			posKey := decision.Symbol + "_" + getPositionSide(decision.Action)

			switch decision.Action {
			case "open_long", "open_short":
				// 记录开仓
				side := "Long"
				if decision.Action == "open_short" {
					side = "Short"
				}
				openPositions[posKey] = struct {
					entryTime  time.Time
					entryPrice float64
					side       string
					symbol     string
				}{
					entryTime:  decision.Timestamp,
					entryPrice: decision.Price,
					side:       side,
					symbol:     decision.Symbol,
				}

			case "close_long", "close_short":
				// 查找对应的开仓记录
				if openPos, exists := openPositions[posKey]; exists {
					// 计算PnL
					var pnl, pnlPercent float64
					if openPos.side == "Long" {
						pnl = decision.Price - openPos.entryPrice
						pnlPercent = (pnl / openPos.entryPrice) * 100
					} else {
						pnl = openPos.entryPrice - decision.Price
						pnlPercent = (pnl / openPos.entryPrice) * 100
					}

					// 创建交易记录
					trade := analytics.TradeRecord{
						Symbol:     decision.Symbol,
						EntryTime:  openPos.entryTime,
						ExitTime:   decision.Timestamp,
						Side:       openPos.side,
						PnL:        pnl * decision.Quantity, // 考虑数量
						PnLPercent: pnlPercent,
						EntryPrice: openPos.entryPrice,
						ExitPrice:  decision.Price,
					}
					trades = append(trades, trade)

					// 移除已平仓的位置
					delete(openPositions, posKey)
				}
			}
		}
	}

	return trades
}

// getPositionSide 从action中提取持仓方向
func getPositionSide(action string) string {
	if action == "open_long" || action == "close_long" {
		return "long"
	}
	return "short"
}
