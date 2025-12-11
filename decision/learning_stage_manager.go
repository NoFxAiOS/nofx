package decision

import (
	"fmt"
	"log"
	"sync"
)

// LearningStageManager Kelly分阶段学习管理器
// 根据交易数量自动调整Kelly参数,实现从保守到积极的渐进式优化
type LearningStageManager struct {
	mu                sync.RWMutex
	currentStage      TrainingStage
	totalTrades       int
	profitableTrades  int
	recentWinRate     float64
	stageParameters   map[TrainingStage]*StageKellyParams
}

// TrainingStage 训练阶段
type TrainingStage int

const (
	StageInfant  TrainingStage = 1  // 婴儿期: 1-5笔交易
	StageChild   TrainingStage = 2  // 学童期: 5-20笔交易
	StageMature  TrainingStage = 3  // 成熟期: 20+笔交易
)

func (ts TrainingStage) String() string {
	switch ts {
	case StageInfant:
		return "婴儿期(1-5笔)"
	case StageChild:
		return "学童期(5-20笔)"
	case StageMature:
		return "成熟期(20+笔)"
	default:
		return "未知"
	}
}

// StageKellyParams 各阶段的Kelly参数
type StageKellyParams struct {
	Stage                     TrainingStage
	MaxLeverage               int     // 最大杠杆倍数
	MinTradesForKelly         int     // Kelly公式最小交易数(本阶段内)
	KellyRatioAdjustment      float64 // Kelly比例调整系数 (0.2~0.8)
	MaxTakeProfitMultiplier   float64 // 止盈倍数上限
	TargetTakeProfitPct       float64 // 目标止盈百分比
	DefaultStopLossPct        float64 // 默认止损百分比
	ProtectionRatioMin        float64 // 保护比例最小值
	FundingFeeAvoidance       bool    // 是否避开资金费率结算
	AllowVolatilityAdjustment bool    // 是否允许波动率调整
	AllowAIException          bool    // 是否允许AI例外放权
	Description               string  // 阶段描述
}

// DefaultStageParams 获取各阶段的默认参数
func DefaultStageParams() map[TrainingStage]*StageKellyParams {
	return map[TrainingStage]*StageKellyParams{
		StageInfant: {
			Stage:                   StageInfant,
			MaxLeverage:             1,
			MinTradesForKelly:       2, // 2笔就可以用Kelly
			KellyRatioAdjustment:    0.2, // 超保守
			MaxTakeProfitMultiplier: 1.5, // 目标倍数低
			TargetTakeProfitPct:     0.08, // 8%止盈
			DefaultStopLossPct:      0.12, // 12%止损
			ProtectionRatioMin:      0.2, // 保护比例最小20%
			FundingFeeAvoidance:     true, // 避开资金费率
			AllowVolatilityAdjustment: false, // 不做波动率调整
			AllowAIException:        false, // 不允许AI例外
			Description: "💤 保守学习期\n" +
				"- 杠杆: 1x (无杠杆)\n" +
				"- 目标: 积累数据,验证策略\n" +
				"- Kelly: 0.2倍 (极保守)\n" +
				"- 重点: 确保本金安全",
		},
		StageChild: {
			Stage:                   StageChild,
			MaxLeverage:             2,
			MinTradesForKelly:       5, // 5笔就可以用Kelly
			KellyRatioAdjustment:    0.4, // 保守
			MaxTakeProfitMultiplier: 2.0,
			TargetTakeProfitPct:     0.10, // 10%止盈
			DefaultStopLossPct:      0.10, // 10%止损
			ProtectionRatioMin:      0.3, // 保护比例最小30%
			FundingFeeAvoidance:     true, // 避开资金费率
			AllowVolatilityAdjustment: true, // 允许波动率调整
			AllowAIException:        false, // 不允许AI例外
			Description: "👦 逐步学习期\n" +
				"- 杠杆: 2x (低倍)\n" +
				"- 目标: 验证胜率,调整参数\n" +
				"- Kelly: 0.4倍 (中等保守)\n" +
				"- 重点: 基于胜率动态调整",
		},
		StageMature: {
			Stage:                   StageMature,
			MaxLeverage:             5,
			MinTradesForKelly:       10, // 10笔可用Kelly
			KellyRatioAdjustment:    0.6, // 中等
			MaxTakeProfitMultiplier: 3.5,
			TargetTakeProfitPct:     0.15, // 15%止盈
			DefaultStopLossPct:      0.08, // 8%止损
			ProtectionRatioMin:      0.4, // 保护比例最小40%
			FundingFeeAvoidance:     false, // 允许跨资金费率
			AllowVolatilityAdjustment: true, // 允许波动率调整
			AllowAIException:        true, // 允许AI例外放权
			Description: "🦁 成熟交易期\n" +
				"- 杠杆: 5x (标准)\n" +
				"- 目标: 最优化Kelly,追求增长\n" +
				"- Kelly: 0.6倍 (中等)\n" +
				"- 重点: 自适应优化",
		},
	}
}

// NewLearningStageManager 创建学习阶段管理器
func NewLearningStageManager() *LearningStageManager {
	return &LearningStageManager{
		currentStage:    StageInfant,
		stageParameters: DefaultStageParams(),
	}
}

// UpdateTradeStats 更新交易统计并自动切换阶段
func (lsm *LearningStageManager) UpdateTradeStats(isWin bool) {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	oldStage := lsm.currentStage

	lsm.totalTrades++
	if isWin {
		lsm.profitableTrades++
	}

	// 计算最近胜率 (基于最近10笔交易)
	if lsm.totalTrades > 0 {
		lsm.recentWinRate = float64(lsm.profitableTrades) / float64(lsm.totalTrades)
	}

	// 自动切换阶段
	lsm.updateStage()

	// 如果阶段发生变化,输出日志
	if oldStage != lsm.currentStage {
		log.Printf("🎓 学习阶段晋升: %s → %s (总交易数: %d, 胜率: %.2f%%)",
			oldStage, lsm.currentStage, lsm.totalTrades, lsm.recentWinRate*100)
		log.Println(lsm.stageParameters[lsm.currentStage].Description)
	}
}

// updateStage 更新当前阶段
func (lsm *LearningStageManager) updateStage() {
	if lsm.totalTrades >= 20 {
		lsm.currentStage = StageMature
	} else if lsm.totalTrades >= 5 {
		lsm.currentStage = StageChild
	} else {
		lsm.currentStage = StageInfant
	}
}

// GetCurrentStage 获取当前阶段
func (lsm *LearningStageManager) GetCurrentStage() TrainingStage {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	return lsm.currentStage
}

// GetStageParams 获取当前阶段参数
func (lsm *LearningStageManager) GetStageParams() *StageKellyParams {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	return lsm.stageParameters[lsm.currentStage]
}

// GetCurrentStats 获取当前统计
func (lsm *LearningStageManager) GetCurrentStats() (stage TrainingStage, totalTrades, profitableTrades int, winRate float64) {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	return lsm.currentStage, lsm.totalTrades, lsm.profitableTrades, lsm.recentWinRate
}

// AdjustKellyByWinRate 根据胜率动态调整Kelly参数
// 这是关键: 胜率越高,越允许提升杠杆
func (lsm *LearningStageManager) AdjustKellyByWinRate() float64 {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	params := lsm.stageParameters[lsm.currentStage]
	baseKelly := params.KellyRatioAdjustment

	// 胜率调整系数
	adjustmentFactor := 1.0

	if lsm.recentWinRate >= 0.65 {
		adjustmentFactor = 1.3 // 胜率>65% 允许提升30%
		log.Printf("📈 高胜率调整: %.2f%% → Kelly提升30%%", lsm.recentWinRate*100)
	} else if lsm.recentWinRate >= 0.55 {
		adjustmentFactor = 1.1 // 胜率>55% 允许提升10%
	} else if lsm.recentWinRate <= 0.35 {
		adjustmentFactor = 0.7 // 胜率<35% 降低30%
		log.Printf("⚠️ 低胜率调整: %.2f%% → Kelly降低30%%", lsm.recentWinRate*100)
	}

	return baseKelly * adjustmentFactor
}

// GetMaxLeverageForCurrentStage 获取当前阶段最大杠杆
func (lsm *LearningStageManager) GetMaxLeverageForCurrentStage() int {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	baseLeverage := lsm.stageParameters[lsm.currentStage].MaxLeverage

	// 如果胜率足够高,允许加1倍杠杆
	if lsm.recentWinRate > 0.6 && lsm.stageParameters[lsm.currentStage].AllowAIException {
		baseLeverage += 1
	}

	return baseLeverage
}

// GetRecommendedKellyParams 获取推荐的Kelly参数集
func (lsm *LearningStageManager) GetRecommendedKellyParams() KellyRecommendation {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	params := lsm.stageParameters[lsm.currentStage]
	adjustedKelly := params.KellyRatioAdjustment

	// 根据胜率调整
	if lsm.recentWinRate > 0.55 {
		adjustedKelly = params.KellyRatioAdjustment * 1.1
	} else if lsm.recentWinRate < 0.40 {
		adjustedKelly = params.KellyRatioAdjustment * 0.8
	}

	return KellyRecommendation{
		Stage:                  lsm.currentStage,
		KellyAdjustment:        adjustedKelly,
		MaxLeverage:            params.MaxLeverage,
		TargetTakeProfitPct:    params.TargetTakeProfitPct,
		DefaultStopLossPct:     params.DefaultStopLossPct,
		ProtectionRatioMin:     params.ProtectionRatioMin,
		RecentWinRate:          lsm.recentWinRate,
		TotalTrades:            lsm.totalTrades,
		Confidence:             lsm.calculateConfidence(),
		IsStageReadyForUpgrade: lsm.isStageReadyForUpgrade(),
	}
}

// calculateConfidence 计算当前参数的置信度 (0-100)
// 基于交易数量和胜率稳定性
func (lsm *LearningStageManager) calculateConfidence() float64 {
	// 交易数量越多,置信度越高
	tradesConfidence := float64(lsm.totalTrades) / 30.0 // 30笔交易达到100%
	if tradesConfidence > 1.0 {
		tradesConfidence = 1.0
	}

	// 胜率越稳定,置信度越高
	// (简化版: 胜率接近50%时最不确定)
	winRateVariance := 0.5 - (lsm.recentWinRate - 0.5) * (lsm.recentWinRate - 0.5)
	if lsm.recentWinRate < 0.2 || lsm.recentWinRate > 0.8 {
		winRateVariance = 1.0 // 极端胜率时确定性高
	}

	confidence := (tradesConfidence + winRateVariance) / 2.0 * 100
	return confidence
}

// isStageReadyForUpgrade 检查当前阶段是否可以升级
func (lsm *LearningStageManager) isStageReadyForUpgrade() bool {
	// 婴儿期→学童期: 需要5笔交易且胜率>40%
	if lsm.currentStage == StageInfant {
		return lsm.totalTrades >= 5 && lsm.recentWinRate > 0.4
	}

	// 学童期→成熟期: 需要20笔交易且胜率>45%
	if lsm.currentStage == StageChild {
		return lsm.totalTrades >= 20 && lsm.recentWinRate > 0.45
	}

	return false
}

// PrintStageReport 打印阶段报告
func (lsm *LearningStageManager) PrintStageReport() {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	params := lsm.stageParameters[lsm.currentStage]
	recommendation := lsm.GetRecommendedKellyParams()

	fmt.Printf("\n📊 ===== Kelly学习阶段报告 =====\n")
	fmt.Printf("当前阶段: %s\n", lsm.currentStage)
	fmt.Printf("总交易数: %d | 盈利笔数: %d | 胜率: %.2f%%\n",
		lsm.totalTrades, lsm.profitableTrades, lsm.recentWinRate*100)
	fmt.Printf("参数置信度: %.1f%%\n", recommendation.Confidence)
	fmt.Printf("\n📋 当前阶段参数:\n")
	fmt.Printf("  Kelly系数: %.2f (调整后: %.2f)\n", params.KellyRatioAdjustment, recommendation.KellyAdjustment)
	fmt.Printf("  最大杠杆: %dx\n", params.MaxLeverage)
	fmt.Printf("  目标止盈: %.2f%%\n", params.TargetTakeProfitPct*100)
	fmt.Printf("  默认止损: %.2f%%\n", params.DefaultStopLossPct*100)
	fmt.Printf("\n%s\n", params.Description)

	if recommendation.IsStageReadyForUpgrade {
		fmt.Printf("🎉 阶段升级条件满足! 下一阶段获得更多杠杆和灵活性\n")
	}
	fmt.Printf("================================\n\n")
}

// KellyRecommendation Kelly推荐参数
type KellyRecommendation struct {
	Stage                TrainingStage
	KellyAdjustment      float64
	MaxLeverage          int
	TargetTakeProfitPct  float64
	DefaultStopLossPct   float64
	ProtectionRatioMin   float64
	RecentWinRate        float64
	TotalTrades          int
	Confidence           float64 // 0-100
	IsStageReadyForUpgrade bool
}
