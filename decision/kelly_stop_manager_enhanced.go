package decision

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sync"
	"time"
)

// KellyStopManagerEnhanced 增强版凯利公式止盈止损管理器
// 新增功能：数据持久化、时间衰减、实时峰值追踪、参数调优
type KellyStopManagerEnhanced struct {
	historicalStats map[string]*HistoricalStatsEnhanced
	statsMutex      sync.RWMutex
	config          *KellyConfig

	// 持久化相关
	dataFilePath string
	saveInterval time.Duration
	lastSaveTime time.Time

	// 实时追踪
	positionPeaks map[string]float64 // 持仓期间的最高盈利点
	peakMutex     sync.RWMutex
}

// HistoricalStatsEnhanced 增强版历史交易统计
type HistoricalStatsEnhanced struct {
	Symbol           string                     `json:"symbol"`            // 交易对
	TotalTrades      int                        `json:"total_trades"`      // 总交易次数
	ProfitableTrades int                        `json:"profitable_trades"` // 盈利交易次数
	TotalProfitPct   float64                    `json:"total_profit_pct"`  // 总盈利百分比
	TotalLossPct     float64                    `json:"total_loss_pct"`    // 总亏损百分比
	WinRate          float64                    `json:"win_rate"`          // 胜率
	AvgWinPct        float64                    `json:"avg_win_pct"`       // 平均盈利百分比
	AvgLossPct       float64                    `json:"avg_loss_pct"`      // 平均亏损百分比
	MaxProfitPct     float64                    `json:"max_profit_pct"`    // 最大单次盈利百分比
	MaxDrawdownPct   float64                    `json:"max_drawdown_pct"`  // 最大回撤百分比
	UpdatedAt        int64                      `json:"updated_at"`        // 更新时间戳

	// 增强字段
	TradeHistory     []TradeRecord              `json:"trade_history"`     // 详细交易历史
	WeightedWinRate  float64                    `json:"weighted_win_rate"` // 时间加权胜率
	Volatility       float64                    `json:"volatility"`        // 波动率估算
	TimeDecayFactor  float64                    `json:"time_decay_factor"` // 时间衰减因子
}

// TradeRecord 单个交易记录
type TradeRecord struct {
	Timestamp   int64   `json:"timestamp"`    // 交易时间
	ProfitPct   float64 `json:"profit_pct"`   // 盈利百分比
	IsWin       bool    `json:"is_win"`       // 是否盈利
	Weight      float64 `json:"weight"`       // 时间权重
	HoldingTime int64   `json:"holding_time"` // 持仓时间（秒）
}

// KellyConfig 凯利公式配置参数
type KellyConfig struct {
	KellyRatioAdjustment float64 `json:"kelly_ratio_adjustment"` // 凯利比例调整系数（默认0.5）
	MaxTakeProfitMultiplier float64 `json:"max_take_profit_multiplier"` // 最大止盈倍数（默认3.0）
	TimeDecayLambda      float64 `json:"time_decay_lambda"`      // 时间衰减参数（默认0.01）
	MinTradesForKelly    int     `json:"min_trades_for_kelly"`   // Kelly公式最小交易数（默认5）
	VolatilityWindow     int     `json:"volatility_window"`      // 波动率计算窗口（默认20）
	SaveIntervalSeconds  int     `json:"save_interval_seconds"`  // 自动保存间隔（默认300秒）
}

// DefaultKellyConfig 默认配置
func DefaultKellyConfig() *KellyConfig {
	return &KellyConfig{
		KellyRatioAdjustment:    0.5,
		MaxTakeProfitMultiplier: 3.0,
		TimeDecayLambda:         0.01,
		MinTradesForKelly:       5,
		VolatilityWindow:        20,
		SaveIntervalSeconds:     300,
	}
}

// NewKellyStopManagerEnhanced 创建增强版凯利公式管理器
func NewKellyStopManagerEnhanced(dataFilePath string) *KellyStopManagerEnhanced {
	ksm := &KellyStopManagerEnhanced{
		historicalStats: make(map[string]*HistoricalStatsEnhanced),
		config:          DefaultKellyConfig(),
		dataFilePath:    dataFilePath,
		saveInterval:    time.Duration(DefaultKellyConfig().SaveIntervalSeconds) * time.Second,
		positionPeaks:   make(map[string]float64),
		lastSaveTime:    time.Now(),
	}

	// 尝试加载历史数据
	if err := ksm.LoadStatsFromFile(dataFilePath); err != nil {
		log.Printf("⚠️ 无法加载历史统计数据: %v，将创建新的统计记录", err)
	}

	return ksm
}

// SaveStatsToFile 保存统计数据到文件
func (ksm *KellyStopManagerEnhanced) SaveStatsToFile(filename string) error {
	ksm.statsMutex.RLock()
	defer ksm.statsMutex.RUnlock()

	data, err := json.MarshalIndent(ksm.historicalStats, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化统计数据失败: %w", err)
	}

	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	log.Printf("💾 成功保存统计数据到文件: %s", filename)
	ksm.lastSaveTime = time.Now()
	return nil
}

// LoadStatsFromFile 从文件加载统计数据
func (ksm *KellyStopManagerEnhanced) LoadStatsFromFile(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", filename)
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	ksm.statsMutex.Lock()
	defer ksm.statsMutex.Unlock()

	if err := json.Unmarshal(data, &ksm.historicalStats); err != nil {
		return fmt.Errorf("反序列化失败: %w", err)
	}

	log.Printf("📂 成功从文件加载统计数据: %s", filename)
	return nil
}

// AutoSave 自动保存（如果到了保存间隔）
func (ksm *KellyStopManagerEnhanced) AutoSave() error {
	if time.Since(ksm.lastSaveTime) >= ksm.saveInterval {
		return ksm.SaveStatsToFile(ksm.dataFilePath)
	}
	return nil
}

// UpdatePositionPeak 更新持仓期间的峰值盈利
func (ksm *KellyStopManagerEnhanced) UpdatePositionPeak(symbol string, currentProfitPct float64) {
	ksm.peakMutex.Lock()
	defer ksm.peakMutex.Unlock()

	if currentProfitPct > 0 {
		if peak, exists := ksm.positionPeaks[symbol]; !exists || currentProfitPct > peak {
			ksm.positionPeaks[symbol] = currentProfitPct
			log.Printf("🎯 [%s] 更新持仓峰值盈利: %.2f%%", symbol, currentProfitPct*100)
		}
	}
}

// GetPositionPeak 获取持仓期间的峰值盈利
func (ksm *KellyStopManagerEnhanced) GetPositionPeak(symbol string) float64 {
	ksm.peakMutex.RLock()
	defer ksm.peakMutex.RUnlock()

	if peak, exists := ksm.positionPeaks[symbol]; exists {
		return peak
	}
	return 0
}

// ClearPositionPeak 清除持仓峰值记录（平仓时调用）
func (ksm *KellyStopManagerEnhanced) ClearPositionPeak(symbol string) {
	ksm.peakMutex.Lock()
	defer ksm.peakMutex.Unlock()

	delete(ksm.positionPeaks, symbol)
}

// CalculateTimeWeight 计算时间权重（指数衰减）
func (ksm *KellyStopManagerEnhanced) CalculateTimeWeight(tradeTime int64) float64 {
	now := time.Now().Unix()
	timeDiff := float64(now-tradeTime) / (24 * 3600) // 转换为天数

	// 指数衰减权重：weight = e^(-lambda * t)
	weight := math.Exp(-ksm.config.TimeDecayLambda * timeDiff)
	return math.Max(weight, 0.01) // 最小权重为0.01
}

// UpdateHistoricalStatsEnhanced 更新增强版历史统计数据
func (ksm *KellyStopManagerEnhanced) UpdateHistoricalStatsEnhanced(symbol string, isWin bool, profitPct float64, holdingTime int64) {
	ksm.statsMutex.Lock()
	defer ksm.statsMutex.Unlock()

	stats, exists := ksm.historicalStats[symbol]
	if !exists {
		stats = &HistoricalStatsEnhanced{
			Symbol:          symbol,
			TradeHistory:    make([]TradeRecord, 0),
			TimeDecayFactor: ksm.config.TimeDecayLambda,
		}
		ksm.historicalStats[symbol] = stats
	}

	// 更新交易次数
	stats.TotalTrades++
	stats.UpdatedAt = time.Now().Unix()

	// 创建交易记录
	tradeRecord := TradeRecord{
		Timestamp:   time.Now().Unix(),
		ProfitPct:   profitPct,
		IsWin:       isWin,
		Weight:      ksm.CalculateTimeWeight(time.Now().Unix()),
		HoldingTime: holdingTime,
	}

	// 添加到历史记录（保持窗口大小）
	stats.TradeHistory = append(stats.TradeHistory, tradeRecord)
	if len(stats.TradeHistory) > ksm.config.VolatilityWindow*2 {
		stats.TradeHistory = stats.TradeHistory[len(stats.TradeHistory)-ksm.config.VolatilityWindow*2:]
	}

	if isWin {
		stats.ProfitableTrades++
		stats.TotalProfitPct += profitPct
		if profitPct > stats.MaxProfitPct {
			stats.MaxProfitPct = profitPct
		}
	} else {
		stats.TotalLossPct += math.Abs(profitPct)
		if math.Abs(profitPct) > stats.MaxDrawdownPct {
			stats.MaxDrawdownPct = math.Abs(profitPct)
		}
	}

	// 计算加权胜率
	ksm.calculateWeightedStats(stats)

	log.Printf("📊 [%s] 更新增强统计: 总交易=%d, 盈利=%d, 加权胜率=%.2f%%, 平均盈利=%.2f%%, 平均亏损=%.2f%%, 峰值=%.2f%%",
		symbol, stats.TotalTrades, stats.ProfitableTrades, stats.WeightedWinRate*100,
		stats.AvgWinPct, stats.AvgLossPct, ksm.GetPositionPeak(symbol)*100)

	// 自动保存检查
	if err := ksm.AutoSave(); err != nil {
		log.Printf("⚠️ 自动保存失败: %v", err)
	}
}

// calculateWeightedStats 计算加权统计数据
func (ksm *KellyStopManagerEnhanced) calculateWeightedStats(stats *HistoricalStatsEnhanced) {
	if len(stats.TradeHistory) == 0 {
		return
	}

	var weightedWins, weightedLosses float64
	var winCount, lossCount int
	var totalWeight float64
	var winWeightSum float64

	for _, trade := range stats.TradeHistory {
		timeWeight := ksm.CalculateTimeWeight(trade.Timestamp)
		finalWeight := trade.Weight * timeWeight
		totalWeight += finalWeight

		if trade.IsWin {
			weightedWins += finalWeight * trade.ProfitPct
			winWeightSum += finalWeight
			winCount++
		} else {
			weightedLosses += finalWeight * math.Abs(trade.ProfitPct)
			lossCount++
		}
	}

	if totalWeight > 0 {
		if winCount > 0 {
			stats.AvgWinPct = weightedWins / float64(winCount)
		}
		if lossCount > 0 {
			stats.AvgLossPct = weightedLosses / float64(lossCount)
		}
		// 修正加权胜率计算：应该是盈利交易权重和 / 总权重和
		if winCount > 0 {
			stats.WeightedWinRate = winWeightSum / totalWeight
		} else {
			stats.WeightedWinRate = 0.0
		}
		if winCount+lossCount > 0 {
			stats.WinRate = float64(winCount) / float64(winCount+lossCount)
		}
	}

	// 计算波动率
	ksm.calculateVolatility(stats)
}

// calculateVolatility 计算波动率
func (ksm *KellyStopManagerEnhanced) calculateVolatility(stats *HistoricalStatsEnhanced) {
	if len(stats.TradeHistory) < 2 {
		stats.Volatility = 0.08 // 默认8%波动率
		return
	}

	windowSize := ksm.config.VolatilityWindow
	if len(stats.TradeHistory) < windowSize {
		windowSize = len(stats.TradeHistory)
	}

	recentTrades := stats.TradeHistory[len(stats.TradeHistory)-windowSize:]

	var sum, sumSquares float64
	for _, trade := range recentTrades {
		profit := trade.ProfitPct
		sum += profit
		sumSquares += profit * profit
	}

	mean := sum / float64(windowSize)
	variance := (sumSquares / float64(windowSize)) - (mean * mean)

	if variance > 0 {
		stats.Volatility = math.Sqrt(variance)
	} else {
		stats.Volatility = 0.08
	}
}

// CalculateOptimalTakeProfitEnhanced 增强版最优止盈计算
func (ksm *KellyStopManagerEnhanced) CalculateOptimalTakeProfitEnhanced(
	symbol string,
	entryPrice float64,
	currentPrice float64,
	positionSide string,
) (float64, error) {
	if entryPrice <= 0 || currentPrice <= 0 {
		return 0, fmt.Errorf("价格无效: entryPrice=%.6f, currentPrice=%.6f", entryPrice, currentPrice)
	}

	// 计算当前盈亏百分比
	currentProfitPct := 0.0
	if positionSide == "long" {
		currentProfitPct = (currentPrice - entryPrice) / entryPrice
	} else {
		currentProfitPct = (entryPrice - currentPrice) / entryPrice
	}

	// 更新持仓峰值
	ksm.UpdatePositionPeak(symbol, currentProfitPct)

	// 获取历史统计数据
	stats := ksm.GetHistoricalStats(symbol)

	// 如果没有足够历史数据，使用经验策略
	if stats == nil || stats.TotalTrades < ksm.config.MinTradesForKelly {
		log.Printf("📊 [%s] 无足够历史数据(%d<%d)，使用默认止盈策略",
			symbol, stats.TotalTrades, ksm.config.MinTradesForKelly)
		return ksm.calculateDefaultTakeProfitEnhanced(entryPrice, currentPrice, positionSide, currentProfitPct, stats)
	}

	// 使用加权胜率进行Kelly计算
	winRate := stats.WeightedWinRate
	avgWinPct := stats.AvgWinPct
	avgLossPct := stats.AvgLossPct

	if avgLossPct <= 0 {
		avgLossPct = 0.08 // 默认8%亏损
	}

	// 考虑波动率调整凯利比例
	volatilityAdjustment := 1.0
	if stats.Volatility > 0.15 { // 高波动率市场
		volatilityAdjustment = 0.8 // 降低风险
	} else if stats.Volatility < 0.05 { // 低波动率市场
		volatilityAdjustment = 1.2 // 适当增加风险
	}

	// 凯利公式：f* = (bp - q) / b
	b := avgWinPct / avgLossPct // 赔率
	q := 1 - winRate           // 败率
	kellyRatio := (b*winRate - q) / b

	// 多重安全调整
	adjustedKellyRatio := kellyRatio * ksm.config.KellyRatioAdjustment * volatilityAdjustment

	// 如果凯利比例为负，使用保守策略
	if adjustedKellyRatio <= 0 {
		log.Printf("📊 [%s] 凯利比例为负(%.3f)，使用保守止盈策略", symbol, adjustedKellyRatio)
		return ksm.calculateConservativeTakeProfitEnhanced(entryPrice, currentPrice, positionSide, currentProfitPct, winRate, stats)
	}

	// 根据波动率动态调整目标倍数
	dynamicMultiplier := ksm.config.MaxTakeProfitMultiplier
	if stats.Volatility > 0.2 {
		dynamicMultiplier = 2.0 // 高波动降低目标
	} else if stats.Volatility < 0.08 {
		dynamicMultiplier = 4.0 // 低波动提高目标
	}

	// 考虑持仓峰值调整目标
	peakProfit := ksm.GetPositionPeak(symbol)
	peakAdjustment := 1.0
	if peakProfit > currentProfitPct && peakProfit > 0 {
		// 曾经达到过更高盈利，适当降低目标
		peakAdjustment = 0.9
		log.Printf("🎯 [%s] 检测到峰值回撤: 峰值=%.2f%%, 当前=%.2f%%", symbol, peakProfit*100, currentProfitPct*100)
	}

	// 计算最优止盈点
	optimalTakeProfitPct := currentProfitPct * (1 + adjustedKellyRatio*2) * peakAdjustment

	// 应用动态倍数限制
	if optimalTakeProfitPct > currentProfitPct*dynamicMultiplier {
		optimalTakeProfitPct = currentProfitPct * dynamicMultiplier
		log.Printf("📊 [%s] 止盈点被限制为动态倍数: %.2f倍", symbol, dynamicMultiplier)
	}

	// 根据持仓方向计算目标价格
	var optimalTakeProfitPrice float64
	if positionSide == "long" {
		optimalTakeProfitPrice = entryPrice * (1 + optimalTakeProfitPct)
	} else {
		optimalTakeProfitPrice = entryPrice * (1 - optimalTakeProfitPct)
	}

	log.Printf("🎯 [%s] 增强凯利止盈: 加权胜率=%.2f%%, 赔率=%.2f, 凯利比例=%.3f, 波动率=%.2f%%, 当前盈利=%.2f%%, 目标盈利=%.2f%%, 目标价格=%.6f",
		symbol, winRate*100, b, adjustedKellyRatio, stats.Volatility*100, currentProfitPct*100, optimalTakeProfitPct*100, optimalTakeProfitPrice)

	return optimalTakeProfitPrice, nil
}

// calculateDefaultTakeProfitEnhanced 增强版默认止盈计算
func (ksm *KellyStopManagerEnhanced) calculateDefaultTakeProfitEnhanced(
	entryPrice, currentPrice float64,
	positionSide string,
	currentProfitPct float64,
	stats *HistoricalStatsEnhanced,
) (float64, error) {
	// 基于波动率的动态策略
	baseTarget := 0.15 // 基础目标15%

	if stats != nil && stats.Volatility > 0 {
		// 根据波动率调整目标
		if stats.Volatility > 0.2 {
			baseTarget = 0.12 // 高波动降低目标
		} else if stats.Volatility < 0.08 {
			baseTarget = 0.18 // 低波动提高目标
		}
	}

	// 基于当前盈利的分层策略
	targetMultiplier := 1.0
	if currentProfitPct < 0.05 {
		targetMultiplier = 1.0 + baseTarget
	} else if currentProfitPct < 0.15 {
		targetMultiplier = 1.0 + baseTarget*0.8
	} else {
		targetMultiplier = 1.0 + baseTarget*0.6
	}

	if positionSide == "long" {
		return currentPrice * targetMultiplier, nil
	}
	return currentPrice / targetMultiplier, nil
}

// calculateConservativeTakeProfitEnhanced 增强版保守止盈计算
func (ksm *KellyStopManagerEnhanced) calculateConservativeTakeProfitEnhanced(
	entryPrice, currentPrice float64,
	positionSide string,
	currentProfitPct float64,
	winRate float64,
	stats *HistoricalStatsEnhanced,
) (float64, error) {
	// 基于波动率和胜率的保守策略
	baseMultiplier := 1.0

	if winRate >= 0.6 {
		baseMultiplier = 1.15
	} else if winRate >= 0.4 {
		baseMultiplier = 1.10
	} else {
		baseMultiplier = 1.05
	}

	// 波动率调整
	if stats != nil && stats.Volatility > 0.15 {
		baseMultiplier *= 0.9 // 高波动更保守
	}

	if positionSide == "long" {
		return currentPrice * baseMultiplier, nil
	}
	return currentPrice / baseMultiplier, nil
}

// CalculateDynamicStopLossEnhanced 增强版动态止损计算
func (ksm *KellyStopManagerEnhanced) CalculateDynamicStopLossEnhanced(
	symbol string,
	entryPrice float64,
	currentPrice float64,
	maxProfitPct float64,
) (float64, error) {
	if entryPrice <= 0 || currentPrice <= 0 {
		return 0, fmt.Errorf("价格无效: entryPrice=%.6f, currentPrice=%.6f", entryPrice, currentPrice)
	}

	// 计算当前盈亏百分比
	currentProfitPct := (currentPrice - entryPrice) / entryPrice

	// 如果是亏损状态，使用基于波动率的止损
	if currentProfitPct <= 0 {
		stats := ksm.GetHistoricalStats(symbol)
		stopLossPct := 0.08 // 默认8%

		if stats != nil && stats.Volatility > 0 {
			// 根据波动率调整止损
			stopLossPct = math.Min(0.12, stats.Volatility*1.5) // 最多12%
		}

		return entryPrice * (1 - stopLossPct), nil
	}

	// 获取统计数据用于动态调整
	stats := ksm.GetHistoricalStats(symbol)

	// 基于波动率和盈利阶段的动态保护策略
	var protectionRatio float64

	if currentProfitPct < 0.05 {
		protectionRatio = 1.0 // 保本
	} else if currentProfitPct < 0.10 {
		protectionRatio = 0.7 // 保护70%
	} else if currentProfitPct < 0.20 {
		protectionRatio = 0.8 // 保护80%
	} else {
		protectionRatio = 0.85 // 保护85%
	}

	// 波动率调整保护比例
	if stats != nil && stats.Volatility > 0 {
		if stats.Volatility > 0.2 {
			protectionRatio *= 0.9 // 高波动更保守
		} else if stats.Volatility < 0.08 {
			protectionRatio *= 1.1 // 低波动可稍微激进
		}
	}

	// 确保保护比例在合理范围内
	protectionRatio = math.Max(0.5, math.Min(1.0, protectionRatio))

	// 计算止损点
	stopDistancePct := currentProfitPct * protectionRatio
	stopLossPct := currentProfitPct - stopDistancePct

	var stopLossPrice float64
	if stopLossPct >= 0 {
		stopLossPrice = entryPrice * (1 + stopLossPct)
	} else {
		stopLossPrice = entryPrice // 保本
	}

	// 基于历史平均亏损进行合理性检查
	if stats != nil && stats.TotalTrades >= ksm.config.MinTradesForKelly && stats.AvgLossPct > 0 {
		maxAllowedLossPct := stats.AvgLossPct * 2.0 // 允许最大2倍平均亏损
		currentLossPct := (currentPrice - stopLossPrice) / entryPrice

		if currentLossPct > maxAllowedLossPct {
			stopLossPrice = currentPrice * (1 - maxAllowedLossPct)
			log.Printf("⚠️ [%s] 止损点过于宽松，调整为最大允许亏损: %.2f%%", symbol, maxAllowedLossPct*100)
		}
	}

	log.Printf("🛡️ [%s] 增强动态止损: 当前盈利=%.2f%%, 保护比例=%.1f%%, 止损价格=%.6f, 波动率=%.2f%%",
		symbol, currentProfitPct*100, protectionRatio*100, stopLossPrice,
		func() float64 { if stats != nil { return stats.Volatility * 100 }; return 0 }())

	return stopLossPrice, nil
}

// GetHistoricalStats 获取历史统计数据
func (ksm *KellyStopManagerEnhanced) GetHistoricalStats(symbol string) *HistoricalStatsEnhanced {
	ksm.statsMutex.RLock()
	defer ksm.statsMutex.RUnlock()

	if stats, exists := ksm.historicalStats[symbol]; exists {
		return stats
	}
	return nil
}

// UpdateConfig 更新配置参数
func (ksm *KellyStopManagerEnhanced) UpdateConfig(config *KellyConfig) {
	ksm.statsMutex.Lock()
	defer ksm.statsMutex.Unlock()

	ksm.config = config
	ksm.saveInterval = time.Duration(config.SaveIntervalSeconds) * time.Second

	log.Printf("⚙️ 更新Kelly配置: 凯利调整=%.2f, 最大倍数=%.1f, 时间衰减=%.3f",
		config.KellyRatioAdjustment, config.MaxTakeProfitMultiplier, config.TimeDecayLambda)
}

// GetConfig 获取当前配置
func (ksm *KellyStopManagerEnhanced) GetConfig() *KellyConfig {
	ksm.statsMutex.RLock()
	defer ksm.statsMutex.RUnlock()

	return ksm.config
}

// Shutdown 优雅关闭，保存数据
func (ksm *KellyStopManagerEnhanced) Shutdown() error {
	log.Println("🔄 正在关闭Kelly管理器，保存数据...")

	if err := ksm.SaveStatsToFile(ksm.dataFilePath); err != nil {
		return fmt.Errorf("关闭时保存数据失败: %w", err)
	}

	log.Println("✅ Kelly管理器已安全关闭")
	return nil
}