package analytics

import (
	"fmt"
	"math"
	"nofx/logger"
	"sort"
	"time"
)

// CorrelationMatrix 相关性矩阵数据结构
type CorrelationMatrix struct {
	Assets      []string             `json:"assets"`
	Matrix      [][]float64          `json:"matrix"`
	Timeframe   string               `json:"timeframe"`
	CalculatedAt time.Time           `json:"calculated_at"`
	Stats       *CorrelationStats    `json:"stats"`
}

// CorrelationStats 相关性统计信息
type CorrelationStats struct {
	AvgCorrelation    float64            `json:"avg_correlation"`
	MaxCorrelation    float64            `json:"max_correlation"`
	MinCorrelation    float64            `json:"min_correlation"`
	HighlyCorrelated  []CorrelationPair  `json:"highly_correlated"`
	LowCorrelated     []CorrelationPair  `json:"low_correlated"`
}

// CorrelationPair 相关性配对
type CorrelationPair struct {
	Asset1      string  `json:"asset1"`
	Asset2      string  `json:"asset2"`
	Correlation float64 `json:"correlation"`
}

// PriceHistory 价格历史数据（用于计算相关性）
type PriceHistory struct {
	Symbol    string
	Prices    []float64
	Timestamps []time.Time
}

// CalculateCorrelationMatrix 计算多资产相关性矩阵
// 使用皮尔逊相关系数 (Pearson Correlation Coefficient)
func CalculateCorrelationMatrix(histories []*PriceHistory, timeframe string) (*CorrelationMatrix, error) {
	if len(histories) < 2 {
		return nil, fmt.Errorf("至少需要2个资产才能计算相关性")
	}

	n := len(histories)
	assets := make([]string, n)
	for i, h := range histories {
		assets[i] = h.Symbol
	}

	// 初始化相关性矩阵
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
	}

	// 计算每对资产的相关性
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				matrix[i][j] = 1.0 // 自身相关性为1
			} else if j > i {
				// 只计算上三角矩阵，下三角对称
				corr, err := calculatePearsonCorrelation(histories[i].Prices, histories[j].Prices)
				if err != nil {
					corr = 0.0 // 如果计算失败，设为0
				}
				matrix[i][j] = corr
				matrix[j][i] = corr // 对称
			}
		}
	}

	// 计算统计信息
	stats := calculateCorrelationStats(assets, matrix)

	return &CorrelationMatrix{
		Assets:       assets,
		Matrix:       matrix,
		Timeframe:    timeframe,
		CalculatedAt: time.Now(),
		Stats:        stats,
	}, nil
}

// calculatePearsonCorrelation 计算皮尔逊相关系数
// r = Σ[(xi - x̄)(yi - ȳ)] / √[Σ(xi - x̄)² · Σ(yi - ȳ)²]
func calculatePearsonCorrelation(x, y []float64) (float64, error) {
	if len(x) != len(y) {
		return 0, fmt.Errorf("数据长度不匹配: %d != %d", len(x), len(y))
	}

	n := len(x)
	if n < 2 {
		return 0, fmt.Errorf("数据点太少: %d < 2", n)
	}

	// 计算均值
	var sumX, sumY float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	// 计算协方差和标准差
	var covariance, varianceX, varianceY float64
	for i := 0; i < n; i++ {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		covariance += diffX * diffY
		varianceX += diffX * diffX
		varianceY += diffY * diffY
	}

	// 避免除零错误
	if varianceX == 0 || varianceY == 0 {
		return 0, fmt.Errorf("标准差为零，无法计算相关性")
	}

	// 皮尔逊相关系数
	correlation := covariance / math.Sqrt(varianceX*varianceY)

	return correlation, nil
}

// calculateCorrelationStats 计算相关性统计信息
func calculateCorrelationStats(assets []string, matrix [][]float64) *CorrelationStats {
	n := len(assets)
	if n < 2 {
		return &CorrelationStats{}
	}

	var sum, max, min float64
	max = -1.0
	min = 1.0
	count := 0

	highCorr := []CorrelationPair{}
	lowCorr := []CorrelationPair{}

	// 只遍历上三角矩阵（避免重复和自相关）
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			corr := matrix[i][j]
			sum += corr
			count++

			if corr > max {
				max = corr
			}
			if corr < min {
				min = corr
			}

			// 高度相关 (|r| > 0.7)
			if math.Abs(corr) > 0.7 {
				highCorr = append(highCorr, CorrelationPair{
					Asset1:      assets[i],
					Asset2:      assets[j],
					Correlation: corr,
				})
			}

			// 低相关 (|r| < 0.3)
			if math.Abs(corr) < 0.3 {
				lowCorr = append(lowCorr, CorrelationPair{
					Asset1:      assets[i],
					Asset2:      assets[j],
					Correlation: corr,
				})
			}
		}
	}

	// 排序：高相关按绝对值降序，低相关按绝对值升序
	sort.Slice(highCorr, func(i, j int) bool {
		return math.Abs(highCorr[i].Correlation) > math.Abs(highCorr[j].Correlation)
	})
	sort.Slice(lowCorr, func(i, j int) bool {
		return math.Abs(lowCorr[i].Correlation) < math.Abs(lowCorr[j].Correlation)
	})

	// 只保留前5个
	if len(highCorr) > 5 {
		highCorr = highCorr[:5]
	}
	if len(lowCorr) > 5 {
		lowCorr = lowCorr[:5]
	}

	avg := sum / float64(count)

	return &CorrelationStats{
		AvgCorrelation:   avg,
		MaxCorrelation:   max,
		MinCorrelation:   min,
		HighlyCorrelated: highCorr,
		LowCorrelated:    lowCorr,
	}
}

// GetHistoricalPrices 从logger获取历史价格数据
// TODO: Implement proper historical price data fetching from market data API
// Currently returns empty data as placeholder
func GetHistoricalPrices(traderId string, symbols []string, lookbackMinutes int) ([]*PriceHistory, error) {
	histories := make([]*PriceHistory, 0, len(symbols))

	// TODO: Implement real historical price fetching
	// For now, return empty histories to allow compilation
	// Future implementation should:
	// 1. Query market data API (Binance/OKX/Bybit) for historical klines
	// 2. Store and retrieve price data from database
	// 3. Parse decision logs if they contain price snapshots

	_ = traderId
	_ = lookbackMinutes

	// Return empty histories for each symbol as placeholder
	for _, symbol := range symbols {
		histories = append(histories, &PriceHistory{
			Symbol:     symbol,
			Prices:     []float64{},
			Timestamps: []time.Time{},
		})
	}

	return histories, nil
}

// GetReturns 计算收益率序列（用于其他分析）
func GetReturns(prices []float64) []float64 {
	if len(prices) < 2 {
		return []float64{}
	}

	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] != 0 {
			returns[i-1] = (prices[i] - prices[i-1]) / prices[i-1]
		}
	}

	return returns
}

// CalculateVolatility 计算波动率（标准差）
func CalculateVolatility(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}

	returns := GetReturns(prices)
	return calculateStdDev(returns)
}

// calculateStdDev 计算标准差
func calculateStdDev(values []float64) float64 {
	n := len(values)
	if n < 2 {
		return 0
	}

	// 计算均值
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	// 计算方差
	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(n - 1) // 样本标准差

	return math.Sqrt(variance)
}
