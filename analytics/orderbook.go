package analytics

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
	"time"
)

// OrderBook 订单簿数据
type OrderBook struct {
	Symbol       string           `json:"symbol"`
	Bids         []OrderBookLevel `json:"bids"` // 买单（按价格降序）
	Asks         []OrderBookLevel `json:"asks"` // 卖单（按价格升序）
	LastUpdateId int64            `json:"last_update_id"`
	Timestamp    time.Time        `json:"timestamp"`
	Stats        *OrderBookStats  `json:"stats"`
}

// OrderBookLevel 订单簿层级
type OrderBookLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Total    float64 `json:"total"` // 累计量
}

// OrderBookStats 订单簿统计
type OrderBookStats struct {
	BestBid           float64 `json:"best_bid"`
	BestAsk           float64 `json:"best_ask"`
	Spread            float64 `json:"spread"`
	SpreadPercent     float64 `json:"spread_percent"`
	MidPrice          float64 `json:"mid_price"`
	BidDepth10        float64 `json:"bid_depth_10"`   // 10档买单深度（USDT）
	AskDepth10        float64 `json:"ask_depth_10"`   // 10档卖单深度（USDT）
	TotalBidVolume    float64 `json:"total_bid_volume"`
	TotalAskVolume    float64 `json:"total_ask_volume"`
	VolumeImbalance   float64 `json:"volume_imbalance"`   // (Bid-Ask)/(Bid+Ask)
	LiquidityScore    float64 `json:"liquidity_score"`    // 流动性评分
	SupportLevel      float64 `json:"support_level"`      // 支撑位（买单最厚的价位）
	ResistanceLevel   float64 `json:"resistance_level"`   // 阻力位（卖单最厚的价位）
}

// OrderBookDepthChart 订单簿深度图数据
type OrderBookDepthChart struct {
	BidLevels []DepthChartPoint `json:"bid_levels"`
	AskLevels []DepthChartPoint `json:"ask_levels"`
}

// DepthChartPoint 深度图点
type DepthChartPoint struct {
	Price          float64 `json:"price"`
	Quantity       float64 `json:"quantity"`
	CumulativeQty  float64 `json:"cumulative_qty"`
	CumulativeValue float64 `json:"cumulative_value"`
}

// FetchOrderBook 从Binance获取订单簿数据
func FetchOrderBook(symbol string, depth int) (*OrderBook, error) {
	if depth <= 0 {
		depth = 20 // 默认20档
	}

	// 验证symbol格式（必须大写，例如BTCUSDT）
	if symbol == "" {
		return nil, fmt.Errorf("symbol不能为空")
	}

	// Binance API限制：depth可以是 5, 10, 20, 50, 100, 500, 1000, 5000
	validDepths := []int{5, 10, 20, 50, 100, 500, 1000, 5000}
	actualDepth := 20
	for _, d := range validDepths {
		if depth <= d {
			actualDepth = d
			break
		}
	}

	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/depth?symbol=%s&limit=%d", symbol, actualDepth)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("获取订单簿失败 (网络错误): %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		// 尝试解析Binance错误响应
		var binanceErr struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if json.Unmarshal(body, &binanceErr) == nil && binanceErr.Msg != "" {
			return nil, fmt.Errorf("Binance API错误 (HTTP %d): %s", resp.StatusCode, binanceErr.Msg)
		}
		return nil, fmt.Errorf("Binance API错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		LastUpdateId int64      `json:"lastUpdateId"`
		Bids         [][]string `json:"bids"`
		Asks         [][]string `json:"asks"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析订单簿失败: %w (响应: %s)", err, string(body))
	}

	// 转换为内部格式
	orderBook := &OrderBook{
		Symbol:       symbol,
		LastUpdateId: result.LastUpdateId,
		Timestamp:    time.Now(),
	}

	// 解析Bids（买单）
	orderBook.Bids = make([]OrderBookLevel, 0, len(result.Bids))
	for _, bid := range result.Bids {
		if len(bid) < 2 {
			continue
		}
		price := parseFloat(bid[0])
		qty := parseFloat(bid[1])
		orderBook.Bids = append(orderBook.Bids, OrderBookLevel{
			Price:    price,
			Quantity: qty,
			Total:    price * qty,
		})
	}

	// 解析Asks（卖单）
	orderBook.Asks = make([]OrderBookLevel, 0, len(result.Asks))
	for _, ask := range result.Asks {
		if len(ask) < 2 {
			continue
		}
		price := parseFloat(ask[0])
		qty := parseFloat(ask[1])
		orderBook.Asks = append(orderBook.Asks, OrderBookLevel{
			Price:    price,
			Quantity: qty,
			Total:    price * qty,
		})
	}

	// 计算统计数据
	orderBook.Stats = calculateOrderBookStats(orderBook)

	return orderBook, nil
}

// calculateOrderBookStats 计算订单簿统计
func calculateOrderBookStats(ob *OrderBook) *OrderBookStats {
	if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return &OrderBookStats{}
	}

	stats := &OrderBookStats{
		BestBid: ob.Bids[0].Price,
		BestAsk: ob.Asks[0].Price,
	}

	// 价差
	stats.Spread = stats.BestAsk - stats.BestBid
	stats.MidPrice = (stats.BestBid + stats.BestAsk) / 2

	if stats.MidPrice > 0 {
		stats.SpreadPercent = (stats.Spread / stats.MidPrice) * 100
	}

	// 10档深度
	var bidDepth10, askDepth10 float64
	for i := 0; i < 10 && i < len(ob.Bids); i++ {
		bidDepth10 += ob.Bids[i].Total
	}
	for i := 0; i < 10 && i < len(ob.Asks); i++ {
		askDepth10 += ob.Asks[i].Total
	}
	stats.BidDepth10 = bidDepth10
	stats.AskDepth10 = askDepth10

	// 总买卖量
	var totalBidVol, totalAskVol float64
	maxBidLevel := OrderBookLevel{}
	maxAskLevel := OrderBookLevel{}

	for _, bid := range ob.Bids {
		totalBidVol += bid.Total
		if bid.Total > maxBidLevel.Total {
			maxBidLevel = bid
		}
	}

	for _, ask := range ob.Asks {
		totalAskVol += ask.Total
		if ask.Total > maxAskLevel.Total {
			maxAskLevel = ask
		}
	}

	stats.TotalBidVolume = totalBidVol
	stats.TotalAskVolume = totalAskVol

	// 成交量不平衡指标
	totalVolume := totalBidVol + totalAskVol
	if totalVolume > 0 {
		stats.VolumeImbalance = (totalBidVol - totalAskVol) / totalVolume
	}

	// 流动性评分（简化版：10档深度之和）
	stats.LiquidityScore = bidDepth10 + askDepth10

	// 支撑位和阻力位（最厚的价位）
	stats.SupportLevel = maxBidLevel.Price
	stats.ResistanceLevel = maxAskLevel.Price

	return stats
}

// GenerateDepthChart 生成深度图数据
func GenerateDepthChart(ob *OrderBook, maxLevels int) *OrderBookDepthChart {
	if maxLevels <= 0 {
		maxLevels = 50
	}

	chart := &OrderBookDepthChart{
		BidLevels: make([]DepthChartPoint, 0, maxLevels),
		AskLevels: make([]DepthChartPoint, 0, maxLevels),
	}

	// 计算买单累计深度（从高到低）
	var cumulativeQty, cumulativeValue float64
	for i := 0; i < maxLevels && i < len(ob.Bids); i++ {
		bid := ob.Bids[i]
		cumulativeQty += bid.Quantity
		cumulativeValue += bid.Total

		chart.BidLevels = append(chart.BidLevels, DepthChartPoint{
			Price:           bid.Price,
			Quantity:        bid.Quantity,
			CumulativeQty:   cumulativeQty,
			CumulativeValue: cumulativeValue,
		})
	}

	// 计算卖单累计深度（从低到高）
	cumulativeQty = 0
	cumulativeValue = 0
	for i := 0; i < maxLevels && i < len(ob.Asks); i++ {
		ask := ob.Asks[i]
		cumulativeQty += ask.Quantity
		cumulativeValue += ask.Total

		chart.AskLevels = append(chart.AskLevels, DepthChartPoint{
			Price:           ask.Price,
			Quantity:        ask.Quantity,
			CumulativeQty:   cumulativeQty,
			CumulativeValue: cumulativeValue,
		})
	}

	return chart
}

// AnalyzeOrderBookImbalance 分析订单簿失衡情况
func AnalyzeOrderBookImbalance(ob *OrderBook, topN int) *OrderBookImbalanceAnalysis {
	if topN <= 0 {
		topN = 10
	}

	analysis := &OrderBookImbalanceAnalysis{
		Symbol:    ob.Symbol,
		Timestamp: ob.Timestamp,
	}

	// 只分析前N档
	if topN > len(ob.Bids) {
		topN = len(ob.Bids)
	}
	if topN > len(ob.Asks) {
		topN = len(ob.Asks)
	}

	var bidVolume, askVolume float64
	for i := 0; i < topN; i++ {
		bidVolume += ob.Bids[i].Total
		askVolume += ob.Asks[i].Total
	}

	analysis.BidVolume = bidVolume
	analysis.AskVolume = askVolume
	analysis.TotalVolume = bidVolume + askVolume

	if analysis.TotalVolume > 0 {
		analysis.ImbalanceRatio = (bidVolume - askVolume) / analysis.TotalVolume
		analysis.BidPercentage = (bidVolume / analysis.TotalVolume) * 100
		analysis.AskPercentage = (askVolume / analysis.TotalVolume) * 100
	}

	// 判断信号
	if analysis.ImbalanceRatio > 0.2 {
		analysis.Signal = "强烈看涨（买单压倒性优势）"
		analysis.Strength = "Strong"
	} else if analysis.ImbalanceRatio > 0.1 {
		analysis.Signal = "看涨（买单优势）"
		analysis.Strength = "Moderate"
	} else if analysis.ImbalanceRatio < -0.2 {
		analysis.Signal = "强烈看跌（卖单压倒性优势）"
		analysis.Strength = "Strong"
	} else if analysis.ImbalanceRatio < -0.1 {
		analysis.Signal = "看跌（卖单优势）"
		analysis.Strength = "Moderate"
	} else {
		analysis.Signal = "中性（买卖平衡）"
		analysis.Strength = "Neutral"
	}

	return analysis
}

// OrderBookImbalanceAnalysis 订单簿失衡分析
type OrderBookImbalanceAnalysis struct {
	Symbol         string    `json:"symbol"`
	Timestamp      time.Time `json:"timestamp"`
	BidVolume      float64   `json:"bid_volume"`
	AskVolume      float64   `json:"ask_volume"`
	TotalVolume    float64   `json:"total_volume"`
	ImbalanceRatio float64   `json:"imbalance_ratio"` // (Bid-Ask)/(Bid+Ask)
	BidPercentage  float64   `json:"bid_percentage"`
	AskPercentage  float64   `json:"ask_percentage"`
	Signal         string    `json:"signal"`   // 信号描述
	Strength       string    `json:"strength"` // Strong, Moderate, Neutral
}

// DetectLargeOrders 检测大单（显著高于平均水平的订单）
func DetectLargeOrders(ob *OrderBook, multiplier float64) *LargeOrdersDetection {
	if multiplier <= 0 {
		multiplier = 3.0 // 默认3倍标准差
	}

	detection := &LargeOrdersDetection{
		Symbol:     ob.Symbol,
		Timestamp:  ob.Timestamp,
		Multiplier: multiplier,
	}

	// 计算买单平均值和标准差
	bidSizes := make([]float64, len(ob.Bids))
	for i, bid := range ob.Bids {
		bidSizes[i] = bid.Total
	}
	bidMean := calculateMean(bidSizes)
	bidStdDev := calculateStdDev(bidSizes)
	bidThreshold := bidMean + multiplier*bidStdDev

	// 计算卖单平均值和标准差
	askSizes := make([]float64, len(ob.Asks))
	for i, ask := range ob.Asks {
		askSizes[i] = ask.Total
	}
	askMean := calculateMean(askSizes)
	askStdDev := calculateStdDev(askSizes)
	askThreshold := askMean + multiplier*askStdDev

	// 检测大买单
	for _, bid := range ob.Bids {
		if bid.Total >= bidThreshold {
			detection.LargeBids = append(detection.LargeBids, LargeOrder{
				Price:    bid.Price,
				Quantity: bid.Quantity,
				Total:    bid.Total,
				Ratio:    bid.Total / bidMean,
			})
		}
	}

	// 检测大卖单
	for _, ask := range ob.Asks {
		if ask.Total >= askThreshold {
			detection.LargeAsks = append(detection.LargeAsks, LargeOrder{
				Price:    ask.Price,
				Quantity: ask.Quantity,
				Total:    ask.Total,
				Ratio:    ask.Total / askMean,
			})
		}
	}

	// 排序（按金额降序）
	sort.Slice(detection.LargeBids, func(i, j int) bool {
		return detection.LargeBids[i].Total > detection.LargeBids[j].Total
	})
	sort.Slice(detection.LargeAsks, func(i, j int) bool {
		return detection.LargeAsks[i].Total > detection.LargeAsks[j].Total
	})

	detection.TotalLargeBids = len(detection.LargeBids)
	detection.TotalLargeAsks = len(detection.LargeAsks)

	return detection
}

// LargeOrdersDetection 大单检测结果
type LargeOrdersDetection struct {
	Symbol          string       `json:"symbol"`
	Timestamp       time.Time    `json:"timestamp"`
	Multiplier      float64      `json:"multiplier"`
	LargeBids       []LargeOrder `json:"large_bids"`
	LargeAsks       []LargeOrder `json:"large_asks"`
	TotalLargeBids  int          `json:"total_large_bids"`
	TotalLargeAsks  int          `json:"total_large_asks"`
}

// LargeOrder 大单
type LargeOrder struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Total    float64 `json:"total"`
	Ratio    float64 `json:"ratio"` // 相对平均值的倍数
}

// parseFloat 辅助函数：解析字符串为float64
func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}
