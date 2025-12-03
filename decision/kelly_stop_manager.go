package decision

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// KellyStopManager 凯利公式止盈止损管理器
// 基于凯利公式动态计算最优止盈止损点
type KellyStopManager struct {
	historicalStats map[string]*HistoricalStats
	statsMutex     sync.RWMutex
}

// HistoricalStats 历史交易统计
type HistoricalStats struct {
	Symbol           string                 `json:"symbol"`           // 交易对
	TotalTrades      int                    `json:"total_trades"`     // 总交易次数
	ProfitableTrades int                    `json:"profitable_trades"`// 盈利交易次数
	TotalProfitPct   float64                `json:"total_profit_pct"` // 总盈利百分比
	TotalLossPct     float64                `json:"total_loss_pct"`   // 总亏损百分比
	WinRate          float64                `json:"win_rate"`         // 胜率
	AvgWinPct        float64                `json:"avg_win_pct"`      // 平均盈利百分比
	AvgLossPct       float64                `json:"avg_loss_pct"`     // 平均亏损百分比
	MaxProfitPct     float64                `json:"max_profit_pct"`   // 最大单次盈利百分比
	MaxDrawdownPct   float64                `json:"max_drawdown_pct"` // 最大回撤百分比
	UpdatedAt        int64                  `json:"updated_at"`       // 更新时间戳
}

// NewKellyStopManager 创建凯利公式管理器
func NewKellyStopManager() *KellyStopManager {
	return &KellyStopManager{
		historicalStats: make(map[string]*HistoricalStats),
	}
}

// UpdateHistoricalStats 更新历史统计数据
// isWin: 是否盈利
// profitPct: 盈利百分比（正数为盈利，负数为亏损）
func (ksm *KellyStopManager) UpdateHistoricalStats(symbol string, isWin bool, profitPct float64) {
	ksm.statsMutex.Lock()
	defer ksm.statsMutex.Unlock()

	stats, exists := ksm.historicalStats[symbol]
	if !exists {
		// 首次交易，创建统计记录
		stats = &HistoricalStats{
			Symbol:           symbol,
			TotalTrades:      0,
			ProfitableTrades: 0,
			TotalProfitPct:   0,
			TotalLossPct:     0,
			MaxProfitPct:     0,
			MaxDrawdownPct:   0,
			UpdatedAt:        time.Now().Unix(),
		}
		ksm.historicalStats[symbol] = stats
	}

	// 更新交易次数
	stats.TotalTrades++
	stats.UpdatedAt = time.Now().Unix()

	if isWin {
		// 盈利交易
		stats.ProfitableTrades++
		stats.TotalProfitPct += profitPct
		stats.AvgWinPct = stats.TotalProfitPct / float64(stats.ProfitableTrades)

		// 更新最大盈利
		if profitPct > stats.MaxProfitPct {
			stats.MaxProfitPct = profitPct
		}
	} else {
		// 亏损交易
		stats.TotalLossPct += math.Abs(profitPct)
		profitTrades := stats.TotalTrades - stats.ProfitableTrades
		if profitTrades > 0 {
			stats.AvgLossPct = stats.TotalLossPct / float64(profitTrades)
		}

		// 更新最大回撤（基于绝对值）
		if math.Abs(profitPct) > stats.MaxDrawdownPct {
			stats.MaxDrawdownPct = math.Abs(profitPct)
		}
	}

	// 计算胜率
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.ProfitableTrades) / float64(stats.TotalTrades)
	}

	log.Printf("📊 [%s] 更新统计数据: 总交易=%d, 盈利=%d, 胜率=%.2f%%, 平均盈利=%.2f%%, 平均亏损=%.2f%%",
		symbol, stats.TotalTrades, stats.ProfitableTrades, stats.WinRate*100, stats.AvgWinPct, stats.AvgLossPct)
}

// GetHistoricalStats 获取历史统计数据
func (ksm *KellyStopManager) GetHistoricalStats(symbol string) *HistoricalStats {
	ksm.statsMutex.RLock()
	defer ksm.statsMutex.RUnlock()

	if stats, exists := ksm.historicalStats[symbol]; exists {
		return stats
	}
	return nil
}

// CalculateOptimalTakeProfit 计算最优止盈点（基于凯利公式）
// symbol: 交易对
// entryPrice: 开仓价格
// currentPrice: 当前价格
// positionSide: 持仓方向（"long" 或 "short"）
func (ksm *KellyStopManager) CalculateOptimalTakeProfit(
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

	// 如果是亏损状态，使用固定止盈目标
	if currentProfitPct <= 0 {
		// 默认止盈目标：15-20%
		fixedTakeProfitPct := 0.18
		if positionSide == "long" {
			return entryPrice * (1 + fixedTakeProfitPct), nil
		}
		return entryPrice * (1 - fixedTakeProfitPct), nil
	}

	// 获取历史统计数据
	stats := ksm.GetHistoricalStats(symbol)

	// 如果没有历史数据，使用经验值
	if stats == nil || stats.TotalTrades < 5 {
		log.Printf("📊 [%s] 无足够历史数据，使用默认止盈策略", symbol)
		return ksm.calculateDefaultTakeProfit(entryPrice, currentPrice, positionSide, currentProfitPct)
	}

	// 计算赔率（平均盈利/平均亏损）
	winRate := stats.WinRate
	avgWinPct := stats.AvgWinPct
	avgLossPct := stats.AvgLossPct

	if avgLossPct <= 0 {
		avgLossPct = 0.08 // 默认8%亏损
	}

	// 凯利公式：f* = (bp - q) / b
	// 其中：b=赔率, p=胜率, q=败率
	b := avgWinPct / avgLossPct  // 赔率
	q := 1 - winRate            // 败率

	// 最优下注比例（调整后，避免过度风险）
	kellyRatio := (b*winRate - q) / b

	// 凯利比例安全调整（保守策略，只使用50%的凯利比例）
	// 避免过度下注导致风险过大
	adjustedKellyRatio := kellyRatio * 0.5

	// 如果凯利比例为负，使用保守策略
	if adjustedKellyRatio <= 0 {
		log.Printf("📊 [%s] 凯利比例为负(%.3f)，使用保守止盈策略", symbol, adjustedKellyRatio)
		return ksm.calculateConservativeTakeProfit(entryPrice, currentPrice, positionSide, currentProfitPct, winRate)
	}

	// 根据凯利比例和当前盈利，计算最优止盈点
	// 思路：将当前已有盈利视为"本金"，用凯利比例计算最优"下注"收益
	optimalTakeProfitPct := currentProfitPct * (1 + adjustedKellyRatio*2)

	// 限制最大止盈倍数（防止过度贪心）
	maxMultiplier := 3.0
	if optimalTakeProfitPct > currentProfitPct*maxMultiplier {
		optimalTakeProfitPct = currentProfitPct * maxMultiplier
		log.Printf("📊 [%s] 止盈点被限制为最大倍数: %.2f%%", symbol, optimalTakeProfitPct*100)
	}

	// 根据持仓方向计算目标价格
	var optimalTakeProfitPrice float64
	if positionSide == "long" {
		optimalTakeProfitPrice = entryPrice * (1 + optimalTakeProfitPct)
	} else {
		optimalTakeProfitPrice = entryPrice * (1 - optimalTakeProfitPct)
	}

	log.Printf("🎯 [%s] 凯利止盈计算: 胜率=%.2f%%, 赔率=%.2f, 凯利比例=%.3f, 当前盈利=%.2f%%, 目标盈利=%.2f%%, 目标价格=%.6f",
		symbol, winRate*100, b, adjustedKellyRatio, currentProfitPct*100, optimalTakeProfitPct*100, optimalTakeProfitPrice)

	return optimalTakeProfitPrice, nil
}

// calculateDefaultTakeProfit 计算默认止盈点（无历史数据时）
func (ksm *KellyStopManager) calculateDefaultTakeProfit(
	entryPrice, currentPrice float64,
	positionSide string,
	currentProfitPct float64,
) (float64, error) {
	// 默认策略：基于当前盈利设置止盈
	// 盈利10%以下，目标再涨10%
	// 盈利10-20%，目标再涨8%
	// 盈利20%以上，目标再涨5%

	targetMultiplier := 1.0
	if currentProfitPct < 0.10 {
		targetMultiplier = 1.10
	} else if currentProfitPct < 0.20 {
		targetMultiplier = 1.08
	} else {
		targetMultiplier = 1.05
	}

	// 设置止盈为当前价格的适度提升
	if positionSide == "long" {
		return currentPrice * targetMultiplier, nil
	}
	return currentPrice / targetMultiplier, nil
}

// calculateConservativeTakeProfit 计算保守止盈点（凯利比例为负时）
func (ksm *KellyStopManager) calculateConservativeTakeProfit(
	entryPrice, currentPrice float64,
	positionSide string,
	currentProfitPct float64,
	winRate float64,
) (float64, error) {
	// 保守策略：根据胜率调整目标
	// 胜率越高，目标越激进
	// 胜率越低，目标越保守

	baseMultiplier := 1.0
	if winRate >= 0.6 {
		baseMultiplier = 1.15 // 高胜率，目标15%额外收益
	} else if winRate >= 0.4 {
		baseMultiplier = 1.10 // 中等胜率，目标10%额外收益
	} else {
		baseMultiplier = 1.05 // 低胜率，目标5%额外收益
	}

	if positionSide == "long" {
		return currentPrice * baseMultiplier, nil
	}
	return currentPrice / baseMultiplier, nil
}

// CalculateDynamicStopLoss 计算动态止损点
// 核心思路：保护已获利润，同时为后续上涨留空间
func (ksm *KellyStopManager) CalculateDynamicStopLoss(
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

	// 如果是亏损状态，使用固定止损（避免进一步亏损）
	if currentProfitPct <= 0 {
		// 固定止损：亏损8-10%
		stopLossPct := 0.08
		return entryPrice * (1 - stopLossPct), nil
	}

	// 盈利状态：动态保护利润
	// 策略：
	// 1. 盈利初期（<5%）：止损设入场价（保本）
	// 2. 盈利中期（5-15%）：保护60%已获利润
	// 3. 盈利后期（>15%）：保护80%已获利润

	var protectionRatio float64
	if currentProfitPct < 0.05 {
		// 盈利初期：保本
		protectionRatio = 1.0
	} else if currentProfitPct < 0.15 {
		// 盈利中期：保护60%
		protectionRatio = 0.6
	} else {
		// 盈利后期：保护80%
		protectionRatio = 0.8
	}

	// 计算止损点：当前价格 - 已获利润 × 保护比例
	stopDistancePct := currentProfitPct * protectionRatio
	stopLossPct := currentProfitPct - stopDistancePct

	var stopLossPrice float64
	if stopLossPct >= 0 {
		// 有利润保护，止损设为保本或微盈利
		stopLossPrice = entryPrice * (1 + stopLossPct)
	} else {
		// 极端情况：止损设为保本
		stopLossPrice = entryPrice
	}

	// 获取历史统计数据，验证止损点合理性
	stats := ksm.GetHistoricalStats(symbol)
	if stats != nil && stats.TotalTrades >= 5 {
		// 如果止损点距离当前价格太远（>平均亏损的1.5倍），适当收紧
		maxAllowedLossPct := stats.AvgLossPct * 1.5
		if (currentPrice - stopLossPrice) / entryPrice > maxAllowedLossPct {
			stopLossPrice = currentPrice * (1 - maxAllowedLossPct)
			log.Printf("⚠️ [%s] 止损点过于宽松，调整为平均亏损的1.5倍: %.2f%%", symbol, maxAllowedLossPct*100)
		}
	}

	log.Printf("🛡️ [%s] 动态止损计算: 当前盈利=%.2f%%, 保护比例=%.1f%%, 保护后止损=%.2f%%, 止损价格=%.6f",
		symbol, currentProfitPct*100, protectionRatio*100, stopLossPct*100, stopLossPrice)

	return stopLossPrice, nil
}

// CalculateKellyOptimalRatio 计算凯利最优下注比例
// 返回值范围：0-1，表示最优资金使用比例
func (ksm *KellyStopManager) CalculateKellyOptimalRatio(symbol string) float64 {
	stats := ksm.GetHistoricalStats(symbol)
	if stats == nil || stats.TotalTrades < 3 {
		// 经验值：40%仓位
		return 0.4
	}

	winRate := stats.WinRate
	avgWinPct := stats.AvgWinPct
	avgLossPct := stats.AvgLossPct

	if avgLossPct <= 0 {
		return 0.3 // 保守策略
	}

	// 凯利公式
	b := avgWinPct / avgLossPct
	kellyRatio := (b*winRate - (1 - winRate)) / b

	// 安全调整：使用50%凯利比例
	adjustedKellyRatio := kellyRatio * 0.5

	// 限制范围：0.1 - 0.8
	if adjustedKellyRatio < 0.1 {
		adjustedKellyRatio = 0.1
	} else if adjustedKellyRatio > 0.8 {
		adjustedKellyRatio = 0.8
	}

	log.Printf("📊 [%s] 凯利最优比例: 胜率=%.2f%%, 赔率=%.2f, 凯利比例=%.3f, 调整后=%.3f",
		symbol, winRate*100, b, kellyRatio, adjustedKellyRatio)

	return adjustedKellyRatio
}
