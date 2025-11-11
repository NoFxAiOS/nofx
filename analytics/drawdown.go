package analytics

import (
	"fmt"
	"math"
	"time"
)

// DrawdownAnalysis 回撤分析数据结构
type DrawdownAnalysis struct {
	MaxDrawdown         float64             `json:"max_drawdown"`           // 最大回撤百分比
	MaxDrawdownDollar   float64             `json:"max_drawdown_dollar"`    // 最大回撤金额
	CurrentDrawdown     float64             `json:"current_drawdown"`       // 当前回撤
	DrawdownPeriods     []DrawdownPeriod    `json:"drawdown_periods"`       // 回撤周期列表
	RecoveryStats       *RecoveryStats      `json:"recovery_stats"`         // 恢复统计
	DrawdownSeries      []DrawdownPoint     `json:"drawdown_series"`        // 回撤序列（用于图表）
	CalculatedAt        time.Time           `json:"calculated_at"`
}

// DrawdownPeriod 回撤周期
type DrawdownPeriod struct {
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	RecoveryTime    *time.Time `json:"recovery_time,omitempty"` // 恢复时间（如果已恢复）
	PeakEquity      float64   `json:"peak_equity"`
	TroughEquity    float64   `json:"trough_equity"`
	DrawdownPercent float64   `json:"drawdown_percent"`
	DrawdownDollar  float64   `json:"drawdown_dollar"`
	Duration        int       `json:"duration_minutes"`        // 持续时间（分钟）
	RecoveryDuration *int     `json:"recovery_duration_minutes,omitempty"` // 恢复时间（分钟）
	IsRecovered     bool      `json:"is_recovered"`
}

// DrawdownPoint 回撤点（用于图表）
type DrawdownPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	Equity          float64   `json:"equity"`
	PeakEquity      float64   `json:"peak_equity"`
	DrawdownPercent float64   `json:"drawdown_percent"`
	DrawdownDollar  float64   `json:"drawdown_dollar"`
	CycleNumber     int       `json:"cycle_number"`
}

// RecoveryStats 恢复统计
type RecoveryStats struct {
	TotalDrawdowns      int     `json:"total_drawdowns"`
	RecoveredDrawdowns  int     `json:"recovered_drawdowns"`
	RecoveryRate        float64 `json:"recovery_rate"`           // 恢复率
	AvgRecoveryTime     float64 `json:"avg_recovery_time_hours"` // 平均恢复时间（小时）
	LongestRecoveryTime float64 `json:"longest_recovery_time_hours"`
}

// EquityPoint 净值点
type EquityPoint struct {
	Timestamp   time.Time
	Equity      float64
	CycleNumber int
}

// CalculateDrawdown 计算回撤分析
func CalculateDrawdown(equityPoints []EquityPoint) (*DrawdownAnalysis, error) {
	if len(equityPoints) < 2 {
		return nil, fmt.Errorf("数据点不足，至少需要2个点")
	}

	// 按时间排序
	// 假设已排序

	var (
		peak            = equityPoints[0].Equity
		peakTime        = equityPoints[0].Timestamp
		maxDrawdown     = 0.0
		maxDrawdownDollar = 0.0
		currentDrawdown = 0.0

		drawdownPeriods []DrawdownPeriod
		drawdownSeries  []DrawdownPoint

		inDrawdown      = false
		currentPeriod   DrawdownPeriod
	)

	// 遍历所有净值点
	for i, point := range equityPoints {
		// 更新峰值
		if point.Equity > peak {
			// 如果在回撤中且净值创新高，标记恢复
			if inDrawdown {
				currentPeriod.RecoveryTime = &point.Timestamp
				currentPeriod.IsRecovered = true
				recoveryDuration := int(point.Timestamp.Sub(currentPeriod.StartTime).Minutes())
				currentPeriod.RecoveryDuration = &recoveryDuration
				drawdownPeriods = append(drawdownPeriods, currentPeriod)
				inDrawdown = false
			}

			peak = point.Equity
			peakTime = point.Timestamp
		}

		// 计算当前回撤
		drawdownDollar := peak - point.Equity
		drawdownPercent := 0.0
		if peak > 0 {
			drawdownPercent = (drawdownDollar / peak) * 100
		}

		// 记录回撤序列点
		drawdownSeries = append(drawdownSeries, DrawdownPoint{
			Timestamp:       point.Timestamp,
			Equity:          point.Equity,
			PeakEquity:      peak,
			DrawdownPercent: drawdownPercent,
			DrawdownDollar:  drawdownDollar,
			CycleNumber:     point.CycleNumber,
		})

		// 更新最大回撤
		if drawdownPercent > maxDrawdown {
			maxDrawdown = drawdownPercent
			maxDrawdownDollar = drawdownDollar
		}

		// 更新当前回撤
		if i == len(equityPoints)-1 {
			currentDrawdown = drawdownPercent
		}

		// 检测回撤周期（回撤超过1%开始记录）
		if drawdownPercent > 1.0 && !inDrawdown {
			// 开始新的回撤周期
			inDrawdown = true
			currentPeriod = DrawdownPeriod{
				StartTime:       peakTime,
				PeakEquity:      peak,
				TroughEquity:    point.Equity,
				DrawdownPercent: drawdownPercent,
				DrawdownDollar:  drawdownDollar,
			}
		} else if inDrawdown {
			// 更新回撤周期的最低点
			if point.Equity < currentPeriod.TroughEquity {
				currentPeriod.TroughEquity = point.Equity
				currentPeriod.EndTime = point.Timestamp
				currentPeriod.DrawdownPercent = ((currentPeriod.PeakEquity - currentPeriod.TroughEquity) / currentPeriod.PeakEquity) * 100
				currentPeriod.DrawdownDollar = currentPeriod.PeakEquity - currentPeriod.TroughEquity
				currentPeriod.Duration = int(currentPeriod.EndTime.Sub(currentPeriod.StartTime).Minutes())
			}
		}
	}

	// 如果当前仍在回撤中，添加到列表
	if inDrawdown {
		currentPeriod.EndTime = equityPoints[len(equityPoints)-1].Timestamp
		currentPeriod.Duration = int(currentPeriod.EndTime.Sub(currentPeriod.StartTime).Minutes())
		currentPeriod.IsRecovered = false
		drawdownPeriods = append(drawdownPeriods, currentPeriod)
	}

	// 计算恢复统计
	recoveryStats := calculateRecoveryStats(drawdownPeriods)

	return &DrawdownAnalysis{
		MaxDrawdown:       maxDrawdown,
		MaxDrawdownDollar: maxDrawdownDollar,
		CurrentDrawdown:   currentDrawdown,
		DrawdownPeriods:   drawdownPeriods,
		RecoveryStats:     recoveryStats,
		DrawdownSeries:    drawdownSeries,
		CalculatedAt:      time.Now(),
	}, nil
}

// calculateRecoveryStats 计算恢复统计
func calculateRecoveryStats(periods []DrawdownPeriod) *RecoveryStats {
	if len(periods) == 0 {
		return &RecoveryStats{}
	}

	var (
		recovered          = 0
		totalRecoveryTime  = 0.0
		longestRecovery    = 0.0
	)

	for _, period := range periods {
		if period.IsRecovered && period.RecoveryDuration != nil {
			recovered++
			recoveryHours := float64(*period.RecoveryDuration) / 60.0
			totalRecoveryTime += recoveryHours

			if recoveryHours > longestRecovery {
				longestRecovery = recoveryHours
			}
		}
	}

	avgRecoveryTime := 0.0
	if recovered > 0 {
		avgRecoveryTime = totalRecoveryTime / float64(recovered)
	}

	recoveryRate := 0.0
	if len(periods) > 0 {
		recoveryRate = (float64(recovered) / float64(len(periods))) * 100
	}

	return &RecoveryStats{
		TotalDrawdowns:      len(periods),
		RecoveredDrawdowns:  recovered,
		RecoveryRate:        recoveryRate,
		AvgRecoveryTime:     avgRecoveryTime,
		LongestRecoveryTime: longestRecovery,
	}
}

// CalculateCalmarRatio 计算Calmar比率（年化收益率 / 最大回撤）
func CalculateCalmarRatio(annualReturn, maxDrawdown float64) float64 {
	if maxDrawdown == 0 {
		return 0
	}
	return annualReturn / maxDrawdown
}

// CalculateSterlingRatio 计算Sterling比率
func CalculateSterlingRatio(annualReturn float64, drawdownPeriods []DrawdownPeriod) float64 {
	if len(drawdownPeriods) == 0 {
		return 0
	}

	// 计算平均回撤
	var sumDrawdown float64
	for _, period := range drawdownPeriods {
		sumDrawdown += period.DrawdownPercent
	}
	avgDrawdown := sumDrawdown / float64(len(drawdownPeriods))

	if avgDrawdown == 0 {
		return 0
	}

	return annualReturn / avgDrawdown
}

// CalculateUlcerIndex 计算溃疡指数（Ulcer Index）
// 衡量回撤的深度和持续时间
func CalculateUlcerIndex(drawdownSeries []DrawdownPoint) float64 {
	if len(drawdownSeries) == 0 {
		return 0
	}

	var sumSquaredDrawdown float64
	for _, point := range drawdownSeries {
		sumSquaredDrawdown += math.Pow(point.DrawdownPercent, 2)
	}

	meanSquaredDrawdown := sumSquaredDrawdown / float64(len(drawdownSeries))
	return math.Sqrt(meanSquaredDrawdown)
}

// GetWorstDrawdowns 获取最严重的N个回撤周期
func GetWorstDrawdowns(periods []DrawdownPeriod, n int) []DrawdownPeriod {
	if n <= 0 || len(periods) == 0 {
		return []DrawdownPeriod{}
	}

	// 复制并排序
	sorted := make([]DrawdownPeriod, len(periods))
	copy(sorted, periods)

	// 按回撤百分比降序排序
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].DrawdownPercent > sorted[i].DrawdownPercent {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if n > len(sorted) {
		n = len(sorted)
	}

	return sorted[:n]
}

// AnalyzeDrawdownDistribution 分析回撤分布
func AnalyzeDrawdownDistribution(periods []DrawdownPeriod) map[string]int {
	distribution := map[string]int{
		"0-5%":    0,
		"5-10%":   0,
		"10-20%":  0,
		"20-30%":  0,
		"30%+":    0,
	}

	for _, period := range periods {
		dd := period.DrawdownPercent
		switch {
		case dd < 5:
			distribution["0-5%"]++
		case dd < 10:
			distribution["5-10%"]++
		case dd < 20:
			distribution["10-20%"]++
		case dd < 30:
			distribution["20-30%"]++
		default:
			distribution["30%+"]++
		}
	}

	return distribution
}
