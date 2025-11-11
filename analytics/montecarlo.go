package analytics

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

// MonteCarloResult Monte Carlo模拟结果
type MonteCarloResult struct {
	Simulations      int                    `json:"simulations"`         // 模拟次数
	TimeHorizon      int                    `json:"time_horizon_days"`   // 时间范围（天）
	InitialBalance   float64                `json:"initial_balance"`
	Percentiles      *PercentileStats       `json:"percentiles"`         // 百分位数统计
	WorstCase        *SimulationPath        `json:"worst_case"`          // 最坏情况
	BestCase         *SimulationPath        `json:"best_case"`           // 最好情况
	MedianCase       *SimulationPath        `json:"median_case"`         // 中位数情况
	ProbabilityStats *ProbabilityStats      `json:"probability_stats"`   // 概率统计
	AllPaths         []SimulationPath       `json:"all_paths,omitempty"` // 所有路径（可选）
	CalculatedAt     time.Time              `json:"calculated_at"`
}

// PercentileStats 百分位数统计
type PercentileStats struct {
	P5    float64 `json:"p5"`    // 5th percentile
	P25   float64 `json:"p25"`   // 25th percentile
	P50   float64 `json:"p50"`   // 50th percentile (median)
	P75   float64 `json:"p75"`   // 75th percentile
	P95   float64 `json:"p95"`   // 95th percentile
	Mean  float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
}

// SimulationPath 模拟路径
type SimulationPath struct {
	FinalBalance   float64   `json:"final_balance"`
	MaxDrawdown    float64   `json:"max_drawdown"`
	PeakBalance    float64   `json:"peak_balance"`
	ReturnPercent  float64   `json:"return_percent"`
	DailyReturns   []float64 `json:"daily_returns,omitempty"`
	BalanceSeries  []float64 `json:"balance_series,omitempty"`
}

// ProbabilityStats 概率统计
type ProbabilityStats struct {
	ProbProfit      float64 `json:"prob_profit"`       // 盈利概率
	ProbLoss        float64 `json:"prob_loss"`         // 亏损概率
	ProbAbove10     float64 `json:"prob_above_10pct"`  // 收益>10%的概率
	ProbAbove20     float64 `json:"prob_above_20pct"`  // 收益>20%的概率
	ProbBelow10     float64 `json:"prob_below_10pct"`  // 亏损>10%的概率
	ProbBelow20     float64 `json:"prob_below_20pct"`  // 亏损>20%的概率
	ExpectedReturn  float64 `json:"expected_return"`   // 期望收益
	RiskOfRuin      float64 `json:"risk_of_ruin"`      // 破产风险（余额<初始的50%）
}

// StrategyParams 策略参数（从历史数据估计）
type StrategyParams struct {
	MeanDailyReturn    float64 `json:"mean_daily_return"`     // 平均日收益率
	StdDevDailyReturn  float64 `json:"std_dev_daily_return"`  // 日收益率标准差
	WinRate            float64 `json:"win_rate"`              // 胜率
	AvgWin             float64 `json:"avg_win"`               // 平均盈利
	AvgLoss            float64 `json:"avg_loss"`              // 平均亏损
	MaxHistDrawdown    float64 `json:"max_hist_drawdown"`     // 历史最大回撤
}

// RunMonteCarlo 运行Monte Carlo模拟
func RunMonteCarlo(initialBalance float64, params *StrategyParams, simulations int, timeHorizonDays int, includePaths bool) (*MonteCarloResult, error) {
	if simulations <= 0 || simulations > 10000 {
		return nil, fmt.Errorf("模拟次数必须在1-10000之间")
	}

	if timeHorizonDays <= 0 || timeHorizonDays > 365 {
		return nil, fmt.Errorf("时间范围必须在1-365天之间")
	}

	// 初始化随机数生成器
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 运行所有模拟
	allPaths := make([]SimulationPath, simulations)
	finalBalances := make([]float64, simulations)

	for i := 0; i < simulations; i++ {
		path := simulatePath(initialBalance, params, timeHorizonDays, rng, includePaths)
		allPaths[i] = path
		finalBalances[i] = path.FinalBalance
	}

	// 排序以计算百分位数
	sortedBalances := make([]float64, simulations)
	copy(sortedBalances, finalBalances)
	sort.Float64s(sortedBalances)

	// 计算统计
	percentiles := calculatePercentiles(sortedBalances)
	probStats := calculateProbabilities(allPaths, initialBalance)

	// 找出最好、最坏、中位数情况
	worstCase := &allPaths[0]
	bestCase := &allPaths[0]
	var medianCase *SimulationPath

	for i := range allPaths {
		if allPaths[i].FinalBalance < worstCase.FinalBalance {
			worstCase = &allPaths[i]
		}
		if allPaths[i].FinalBalance > bestCase.FinalBalance {
			bestCase = &allPaths[i]
		}
	}

	// 找中位数路径（最接近p50的路径）
	medianTarget := percentiles.P50
	minDiff := math.Abs(allPaths[0].FinalBalance - medianTarget)
	medianCase = &allPaths[0]
	for i := range allPaths {
		diff := math.Abs(allPaths[i].FinalBalance - medianTarget)
		if diff < minDiff {
			minDiff = diff
			medianCase = &allPaths[i]
		}
	}

	result := &MonteCarloResult{
		Simulations:      simulations,
		TimeHorizon:      timeHorizonDays,
		InitialBalance:   initialBalance,
		Percentiles:      percentiles,
		WorstCase:        worstCase,
		BestCase:         bestCase,
		MedianCase:       medianCase,
		ProbabilityStats: probStats,
		CalculatedAt:     time.Now(),
	}

	// 如果需要，包含所有路径
	if includePaths {
		result.AllPaths = allPaths
	}

	return result, nil
}

// simulatePath 模拟单个路径
func simulatePath(initialBalance float64, params *StrategyParams, days int, rng *rand.Rand, includeDetails bool) SimulationPath {
	balance := initialBalance
	peak := initialBalance
	maxDrawdown := 0.0

	var dailyReturns []float64
	var balanceSeries []float64

	if includeDetails {
		dailyReturns = make([]float64, 0, days)
		balanceSeries = make([]float64, 0, days+1)
		balanceSeries = append(balanceSeries, initialBalance)
	}

	// 使用几何布朗运动 (Geometric Brownian Motion)
	// S(t+1) = S(t) * exp((μ - σ²/2)dt + σ√dt * Z)
	// 其中 Z ~ N(0,1)

	dt := 1.0 // 日步长
	drift := params.MeanDailyReturn - (params.StdDevDailyReturn * params.StdDevDailyReturn / 2)
	diffusion := params.StdDevDailyReturn * math.Sqrt(dt)

	for day := 0; day < days; day++ {
		// 生成标准正态分布随机数
		z := rng.NormFloat64()

		// 计算收益率
		returnPct := drift*dt + diffusion*z

		// 应用到余额
		balance *= math.Exp(returnPct)

		// 更新峰值
		if balance > peak {
			peak = balance
		}

		// 计算回撤
		if peak > 0 {
			currentDrawdown := ((peak - balance) / peak) * 100
			if currentDrawdown > maxDrawdown {
				maxDrawdown = currentDrawdown
			}
		}

		if includeDetails {
			dailyReturns = append(dailyReturns, returnPct*100)
			balanceSeries = append(balanceSeries, balance)
		}
	}

	returnPercent := ((balance - initialBalance) / initialBalance) * 100

	return SimulationPath{
		FinalBalance:  balance,
		MaxDrawdown:   maxDrawdown,
		PeakBalance:   peak,
		ReturnPercent: returnPercent,
		DailyReturns:  dailyReturns,
		BalanceSeries: balanceSeries,
	}
}

// calculatePercentiles 计算百分位数
func calculatePercentiles(sortedValues []float64) *PercentileStats {
	n := len(sortedValues)
	if n == 0 {
		return &PercentileStats{}
	}

	getPercentile := func(p float64) float64 {
		index := int(math.Floor(p * float64(n-1)))
		if index >= n {
			index = n - 1
		}
		return sortedValues[index]
	}

	// 计算均值
	var sum float64
	for _, v := range sortedValues {
		sum += v
	}
	mean := sum / float64(n)

	// 计算标准差
	var variance float64
	for _, v := range sortedValues {
		diff := v - mean
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(n))

	return &PercentileStats{
		P5:     getPercentile(0.05),
		P25:    getPercentile(0.25),
		P50:    getPercentile(0.50),
		P75:    getPercentile(0.75),
		P95:    getPercentile(0.95),
		Mean:   mean,
		StdDev: stdDev,
	}
}

// calculateProbabilities 计算概率统计
func calculateProbabilities(paths []SimulationPath, initialBalance float64) *ProbabilityStats {
	n := len(paths)
	if n == 0 {
		return &ProbabilityStats{}
	}

	var (
		countProfit   = 0
		countLoss     = 0
		countAbove10  = 0
		countAbove20  = 0
		countBelow10  = 0
		countBelow20  = 0
		countRuin     = 0
		sumReturn     = 0.0
	)

	ruinThreshold := initialBalance * 0.5 // 破产定义：余额<初始的50%

	for _, path := range paths {
		returnPct := path.ReturnPercent

		if returnPct > 0 {
			countProfit++
		} else if returnPct < 0 {
			countLoss++
		}

		if returnPct > 10 {
			countAbove10++
		}
		if returnPct > 20 {
			countAbove20++
		}
		if returnPct < -10 {
			countBelow10++
		}
		if returnPct < -20 {
			countBelow20++
		}

		if path.FinalBalance < ruinThreshold {
			countRuin++
		}

		sumReturn += returnPct
	}

	return &ProbabilityStats{
		ProbProfit:     float64(countProfit) / float64(n) * 100,
		ProbLoss:       float64(countLoss) / float64(n) * 100,
		ProbAbove10:    float64(countAbove10) / float64(n) * 100,
		ProbAbove20:    float64(countAbove20) / float64(n) * 100,
		ProbBelow10:    float64(countBelow10) / float64(n) * 100,
		ProbBelow20:    float64(countBelow20) / float64(n) * 100,
		ExpectedReturn: sumReturn / float64(n),
		RiskOfRuin:     float64(countRuin) / float64(n) * 100,
	}
}

// EstimateStrategyParams 从历史数据估计策略参数
func EstimateStrategyParams(equityPoints []EquityPoint) (*StrategyParams, error) {
	if len(equityPoints) < 2 {
		return nil, fmt.Errorf("数据点不足，至少需要2个点")
	}

	// 计算日收益率序列
	dailyReturns := []float64{}
	wins := []float64{}
	losses := []float64{}

	for i := 1; i < len(equityPoints); i++ {
		prevEquity := equityPoints[i-1].Equity
		currEquity := equityPoints[i].Equity

		if prevEquity > 0 {
			dailyReturn := (currEquity - prevEquity) / prevEquity
			dailyReturns = append(dailyReturns, dailyReturn)

			if dailyReturn > 0 {
				wins = append(wins, dailyReturn)
			} else if dailyReturn < 0 {
				losses = append(losses, dailyReturn)
			}
		}
	}

	// 计算均值和标准差
	meanReturn := calculateMean(dailyReturns)
	stdDevReturn := calculateStdDev(dailyReturns)

	// 计算胜率
	winRate := 0.0
	if len(dailyReturns) > 0 {
		winRate = float64(len(wins)) / float64(len(dailyReturns))
	}

	// 计算平均盈亏
	avgWin := 0.0
	if len(wins) > 0 {
		avgWin = calculateMean(wins)
	}

	avgLoss := 0.0
	if len(losses) > 0 {
		avgLoss = calculateMean(losses)
	}

	// 计算历史最大回撤
	drawdown, err := CalculateDrawdown(equityPoints)
	maxDrawdown := 0.0
	if err == nil {
		maxDrawdown = drawdown.MaxDrawdown
	}

	return &StrategyParams{
		MeanDailyReturn:   meanReturn,
		StdDevDailyReturn: stdDevReturn,
		WinRate:           winRate,
		AvgWin:            avgWin * 100,  // 转为百分比
		AvgLoss:           avgLoss * 100, // 转为百分比
		MaxHistDrawdown:   maxDrawdown,
	}, nil
}

// calculateMean 计算均值
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}
