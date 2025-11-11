package analytics

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ArbitrageOpportunity 套利机会
type ArbitrageOpportunity struct {
	Symbol           string    `json:"symbol"`             // 交易对
	BuyExchange      string    `json:"buy_exchange"`       // 买入交易所
	SellExchange     string    `json:"sell_exchange"`      // 卖出交易所
	BuyPrice         float64   `json:"buy_price"`          // 买入价格
	SellPrice        float64   `json:"sell_price"`         // 卖出价格
	PriceSpread      float64   `json:"price_spread"`       // 价差百分比
	ProfitPotential  float64   `json:"profit_potential"`   // 潜在利润（扣除手续费后）
	Volume24h        float64   `json:"volume_24h"`         // 24小时交易量
	Confidence       string    `json:"confidence"`         // 信心等级: high, medium, low
	Timestamp        time.Time `json:"timestamp"`
}

// ExchangePrice 交易所价格
type ExchangePrice struct {
	Exchange  string    `json:"exchange"`
	Symbol    string    `json:"symbol"`
	BidPrice  float64   `json:"bid_price"`  // 买一价
	AskPrice  float64   `json:"ask_price"`  // 卖一价
	MidPrice  float64   `json:"mid_price"`  // 中间价
	Volume24h float64   `json:"volume_24h"` // 24小时交易量
	Timestamp time.Time `json:"timestamp"`
}

// ArbitrageAnalysis 套利分析结果
type ArbitrageAnalysis struct {
	Opportunities  []ArbitrageOpportunity `json:"opportunities"`
	TopSymbols     []string               `json:"top_symbols"`      // 最高套利机会的币种
	TotalChecked   int                    `json:"total_checked"`    // 检查的交易对数量
	TotalFound     int                    `json:"total_found"`      // 发现的机会数量
	AverageSpread  float64                `json:"average_spread"`   // 平均价差
	BestSpread     float64                `json:"best_spread"`      // 最大价差
	CalculatedAt   time.Time              `json:"calculated_at"`
}

// ArbitrageConfig 套利配置
type ArbitrageConfig struct {
	MinSpreadPercent float64   // 最小价差百分比（过滤阈值）
	TradingFeeRate   float64   // 交易手续费率（单边）
	WithdrawalFeeUSD float64   // 提币手续费（USD）
	MinVolume24h     float64   // 最小24小时交易量
	Exchanges        []string  // 要监控的交易所
	Symbols          []string  // 要监控的交易对
	TransferTimeMin  int       // 转账时间（分钟）
}

// DefaultArbitrageConfig 默认套利配置
func DefaultArbitrageConfig() *ArbitrageConfig {
	return &ArbitrageConfig{
		MinSpreadPercent: 0.5,      // 最小0.5%价差
		TradingFeeRate:   0.001,    // 0.1%手续费（Binance Maker/Taker平均）
		WithdrawalFeeUSD: 1.0,      // $1提币费（保守估计）
		MinVolume24h:     100000,   // 最小$100k日交易量
		Exchanges:        []string{"binance", "okx", "bybit"},
		Symbols:          []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"},
		TransferTimeMin:  10,       // 10分钟转账时间
	}
}

// DetectArbitrage 检测套利机会
func DetectArbitrage(prices []ExchangePrice, config *ArbitrageConfig) (*ArbitrageAnalysis, error) {
	if len(prices) < 2 {
		return nil, fmt.Errorf("至少需要2个交易所的价格数据")
	}

	// 按交易对分组
	symbolPrices := groupPricesBySymbol(prices)

	var opportunities []ArbitrageOpportunity
	totalSpread := 0.0
	maxSpread := 0.0

	// 遍历每个交易对
	for symbol, exchangePrices := range symbolPrices {
		if len(exchangePrices) < 2 {
			continue // 至少需要2个交易所
		}

		// 找出买入价最低和卖出价最高的交易所
		buyOpportunity, sellOpportunity := findBestArbitragePair(exchangePrices)

		if buyOpportunity == nil || sellOpportunity == nil {
			continue
		}

		// 计算价差
		spread := ((sellOpportunity.BidPrice - buyOpportunity.AskPrice) / buyOpportunity.AskPrice) * 100

		// 检查是否满足最小价差要求
		if spread < config.MinSpreadPercent {
			continue
		}

		// 计算净利润（扣除手续费和提币费）
		buyPrice := buyOpportunity.AskPrice
		sellPrice := sellOpportunity.BidPrice

		// 总手续费：买入 + 卖出
		totalFee := (buyPrice * config.TradingFeeRate) + (sellPrice * config.TradingFeeRate)

		// 毛利
		grossProfit := sellPrice - buyPrice

		// 净利润（扣除手续费和提币费）
		netProfit := grossProfit - totalFee - config.WithdrawalFeeUSD

		// 净利润率
		profitPercent := (netProfit / buyPrice) * 100

		// 如果净利润为负，跳过
		if profitPercent <= 0 {
			continue
		}

		// 置信度评估
		confidence := calculateConfidence(spread, profitPercent, buyOpportunity.Volume24h, config)

		opp := ArbitrageOpportunity{
			Symbol:          symbol,
			BuyExchange:     buyOpportunity.Exchange,
			SellExchange:    sellOpportunity.Exchange,
			BuyPrice:        buyOpportunity.AskPrice,
			SellPrice:       sellOpportunity.BidPrice,
			PriceSpread:     spread,
			ProfitPotential: profitPercent,
			Volume24h:       math.Min(buyOpportunity.Volume24h, sellOpportunity.Volume24h), // 取最小值
			Confidence:      confidence,
			Timestamp:       time.Now(),
		}

		opportunities = append(opportunities, opp)
		totalSpread += spread
		if spread > maxSpread {
			maxSpread = spread
		}
	}

	// 按利润潜力排序（降序）
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].ProfitPotential > opportunities[j].ProfitPotential
	})

	// 提取前10个最佳机会的币种
	topSymbols := extractTopSymbols(opportunities, 10)

	avgSpread := 0.0
	if len(opportunities) > 0 {
		avgSpread = totalSpread / float64(len(opportunities))
	}

	analysis := &ArbitrageAnalysis{
		Opportunities: opportunities,
		TopSymbols:    topSymbols,
		TotalChecked:  len(symbolPrices),
		TotalFound:    len(opportunities),
		AverageSpread: avgSpread,
		BestSpread:    maxSpread,
		CalculatedAt:  time.Now(),
	}

	return analysis, nil
}

// groupPricesBySymbol 按交易对分组价格
func groupPricesBySymbol(prices []ExchangePrice) map[string][]ExchangePrice {
	symbolMap := make(map[string][]ExchangePrice)

	for _, price := range prices {
		symbolMap[price.Symbol] = append(symbolMap[price.Symbol], price)
	}

	return symbolMap
}

// findBestArbitragePair 找出最佳套利对（买入价最低 vs 卖出价最高）
func findBestArbitragePair(prices []ExchangePrice) (*ExchangePrice, *ExchangePrice) {
	if len(prices) < 2 {
		return nil, nil
	}

	var lowestAsk *ExchangePrice
	var highestBid *ExchangePrice

	for i := range prices {
		price := &prices[i]

		// 找买入价最低（ask）
		if lowestAsk == nil || price.AskPrice < lowestAsk.AskPrice {
			if price.AskPrice > 0 { // 确保价格有效
				lowestAsk = price
			}
		}

		// 找卖出价最高（bid）
		if highestBid == nil || price.BidPrice > highestBid.BidPrice {
			if price.BidPrice > 0 {
				highestBid = price
			}
		}
	}

	// 确保买入和卖出不是同一个交易所
	if lowestAsk != nil && highestBid != nil && lowestAsk.Exchange == highestBid.Exchange {
		return nil, nil
	}

	return lowestAsk, highestBid
}

// calculateConfidence 计算套利机会的信心等级
func calculateConfidence(spread, profitPercent, volume24h float64, config *ArbitrageConfig) string {
	score := 0

	// 价差越大，信心越高
	if spread >= 2.0 {
		score += 3
	} else if spread >= 1.0 {
		score += 2
	} else {
		score += 1
	}

	// 利润率越高，信心越高
	if profitPercent >= 1.0 {
		score += 3
	} else if profitPercent >= 0.5 {
		score += 2
	} else {
		score += 1
	}

	// 交易量越大，信心越高
	if volume24h >= 1000000 { // $1M+
		score += 3
	} else if volume24h >= 500000 { // $500k+
		score += 2
	} else if volume24h >= config.MinVolume24h {
		score += 1
	}

	// 评分映射到信心等级
	if score >= 7 {
		return "high"
	} else if score >= 4 {
		return "medium"
	}
	return "low"
}

// extractTopSymbols 提取前N个最佳机会的币种
func extractTopSymbols(opportunities []ArbitrageOpportunity, n int) []string {
	if len(opportunities) == 0 {
		return []string{}
	}

	symbols := make([]string, 0, n)
	seen := make(map[string]bool)

	for _, opp := range opportunities {
		if len(symbols) >= n {
			break
		}

		if !seen[opp.Symbol] {
			symbols = append(symbols, opp.Symbol)
			seen[opp.Symbol] = true
		}
	}

	return symbols
}

// CalculateTriangularArbitrage 三角套利检测（单个交易所内部）
// 例如：BTC/USDT -> ETH/BTC -> ETH/USDT 是否存在套利机会
type TriangularArbitrage struct {
	Exchange        string    `json:"exchange"`
	Path            []string  `json:"path"`            // 如 ["BTC/USDT", "ETH/BTC", "ETH/USDT"]
	StartCurrency   string    `json:"start_currency"`  // 起始货币（如 USDT）
	ProfitPercent   float64   `json:"profit_percent"`  // 利润百分比
	Rate1           float64   `json:"rate1"`           // 第一步汇率
	Rate2           float64   `json:"rate2"`           // 第二步汇率
	Rate3           float64   `json:"rate3"`           // 第三步汇率
	FinalAmount     float64   `json:"final_amount"`    // 最终金额（初始1单位）
	Timestamp       time.Time `json:"timestamp"`
}

// DetectTriangularArbitrage 检测三角套利机会
func DetectTriangularArbitrage(prices []ExchangePrice, exchange string) ([]TriangularArbitrage, error) {
	// 三角套利逻辑较复杂，需要构建交易对关系图
	// 这里提供简化版示例：BTC/USDT, ETH/BTC, ETH/USDT

	var opportunities []TriangularArbitrage

	// 过滤出指定交易所的价格
	exchangePrices := make(map[string]ExchangePrice)
	for _, price := range prices {
		if price.Exchange == exchange {
			exchangePrices[price.Symbol] = price
		}
	}

	// 示例：检查 USDT -> BTC -> ETH -> USDT 循环
	btcUsdt, hasBTC := exchangePrices["BTCUSDT"]
	ethBtc, hasETH1 := exchangePrices["ETHBTC"]
	ethUsdt, hasETH2 := exchangePrices["ETHUSDT"]

	if hasBTC && hasETH1 && hasETH2 {
		// 路径1: USDT -> BTC -> ETH -> USDT
		// 1. 用1000 USDT买BTC (bid价)
		// 2. 用BTC买ETH (ask价)
		// 3. 卖ETH换USDT (bid价)

		startAmount := 1000.0

		// Step 1: USDT -> BTC
		btcAmount := startAmount / btcUsdt.AskPrice

		// Step 2: BTC -> ETH
		ethAmount := btcAmount / ethBtc.AskPrice

		// Step 3: ETH -> USDT
		finalUsdt := ethAmount * ethUsdt.BidPrice

		profitPercent := ((finalUsdt - startAmount) / startAmount) * 100

		// 如果有套利机会（扣除手续费后仍盈利）
		// 假设每步0.1%手续费，总共0.3%
		netProfit := profitPercent - 0.3

		if netProfit > 0.1 { // 至少0.1%利润
			opp := TriangularArbitrage{
				Exchange:      exchange,
				Path:          []string{"USDT", "BTC", "ETH", "USDT"},
				StartCurrency: "USDT",
				ProfitPercent: netProfit,
				Rate1:         btcUsdt.AskPrice,
				Rate2:         ethBtc.AskPrice,
				Rate3:         ethUsdt.BidPrice,
				FinalAmount:   finalUsdt,
				Timestamp:     time.Now(),
			}

			opportunities = append(opportunities, opp)
		}
	}

	// 可以添加更多三角套利路径检测...

	return opportunities, nil
}

// EstimateArbitrageRisk 评估套利风险
type ArbitrageRisk struct {
	PriceSlippage     float64 `json:"price_slippage"`     // 价格滑点风险（%）
	TransferRisk      float64 `json:"transfer_risk"`      // 转账风险评分 (0-10)
	LiquidityRisk     float64 `json:"liquidity_risk"`     // 流动性风险评分 (0-10)
	TimingRisk        float64 `json:"timing_risk"`        // 时机风险评分 (0-10)
	OverallRisk       string  `json:"overall_risk"`       // 总体风险等级: low, medium, high
	Recommendation    string  `json:"recommendation"`     // 建议：execute, monitor, skip
}

// EvaluateArbitrageRisk 评估套利风险
func EvaluateArbitrageRisk(opp *ArbitrageOpportunity, config *ArbitrageConfig) *ArbitrageRisk {
	risk := &ArbitrageRisk{}

	// 1. 价格滑点风险：价差越小，滑点风险越高
	if opp.PriceSpread < 1.0 {
		risk.PriceSlippage = 0.5 // 高滑点风险
	} else if opp.PriceSpread < 2.0 {
		risk.PriceSlippage = 0.3
	} else {
		risk.PriceSlippage = 0.1 // 低滑点风险
	}

	// 2. 转账风险：转账时间越长，价格变化风险越大
	if config.TransferTimeMin > 30 {
		risk.TransferRisk = 8.0 // 高风险
	} else if config.TransferTimeMin > 15 {
		risk.TransferRisk = 5.0 // 中风险
	} else {
		risk.TransferRisk = 2.0 // 低风险
	}

	// 3. 流动性风险：交易量越小，流动性风险越高
	if opp.Volume24h < 100000 {
		risk.LiquidityRisk = 9.0 // 极高风险
	} else if opp.Volume24h < 500000 {
		risk.LiquidityRisk = 6.0 // 高风险
	} else if opp.Volume24h < 1000000 {
		risk.LiquidityRisk = 3.0 // 中风险
	} else {
		risk.LiquidityRisk = 1.0 // 低风险
	}

	// 4. 时机风险：利润空间越小，时机把握越关键
	if opp.ProfitPotential < 0.5 {
		risk.TimingRisk = 8.0 // 高风险
	} else if opp.ProfitPotential < 1.0 {
		risk.TimingRisk = 5.0 // 中风险
	} else {
		risk.TimingRisk = 2.0 // 低风险
	}

	// 综合风险评分
	totalRiskScore := risk.TransferRisk + risk.LiquidityRisk + risk.TimingRisk
	avgRiskScore := totalRiskScore / 3.0

	// 映射到风险等级
	if avgRiskScore <= 3.0 {
		risk.OverallRisk = "low"
		risk.Recommendation = "execute"
	} else if avgRiskScore <= 6.0 {
		risk.OverallRisk = "medium"
		risk.Recommendation = "monitor"
	} else {
		risk.OverallRisk = "high"
		risk.Recommendation = "skip"
	}

	return risk
}

// RealTimeArbitrageMonitor 实时套利监控器
type RealTimeArbitrageMonitor struct {
	Config        *ArbitrageConfig
	PriceFeeds    chan ExchangePrice
	Opportunities chan ArbitrageOpportunity
	Stop          chan struct{}
}

// NewRealTimeArbitrageMonitor 创建实时监控器
func NewRealTimeArbitrageMonitor(config *ArbitrageConfig) *RealTimeArbitrageMonitor {
	return &RealTimeArbitrageMonitor{
		Config:        config,
		PriceFeeds:    make(chan ExchangePrice, 100),
		Opportunities: make(chan ArbitrageOpportunity, 10),
		Stop:          make(chan struct{}),
	}
}

// Start 启动监控（示例框架，实际需要WebSocket连接）
func (m *RealTimeArbitrageMonitor) Start() {
	// 实际实现需要：
	// 1. 连接多个交易所的WebSocket
	// 2. 实时接收价格更新
	// 3. 检测套利机会
	// 4. 发送警报

	// 这里是简化的示例框架
	go func() {
		priceBuffer := make(map[string][]ExchangePrice)

		for {
			select {
			case price := <-m.PriceFeeds:
				// 缓存价格
				priceBuffer[price.Symbol] = append(priceBuffer[price.Symbol], price)

				// 定期检测（每秒检测一次）
				if len(priceBuffer[price.Symbol]) >= len(m.Config.Exchanges) {
					analysis, err := DetectArbitrage(priceBuffer[price.Symbol], m.Config)
					if err == nil && len(analysis.Opportunities) > 0 {
						// 发送最佳机会
						m.Opportunities <- analysis.Opportunities[0]
					}

					// 清空缓存
					priceBuffer[price.Symbol] = []ExchangePrice{}
				}

			case <-m.Stop:
				return
			}
		}
	}()
}

// StopMonitoring 停止监控
func (m *RealTimeArbitrageMonitor) StopMonitoring() {
	close(m.Stop)
}
