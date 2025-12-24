package mem0

import (
	"strings"
	"fmt"
	"log"
	"sync"
	"time"
)

// RiskAwareFormatter P0修复#4: Kelly杠杆冲突解决
// 作用: 按交易者阶段过滤Mem0建议,防止高杠杆推荐给新手
// 防止: Kelly安全准则冲突,过高杠杆导致爆仓
// 策略: infant(0-5%) → child(5-25%) → mature(25-50%)
type RiskAwareFormatter struct {
	maxLeverageByStage map[KellyStage]float64 // 各阶段最大杠杆倍数
	filterRules        map[KellyStage]FilterRule
	metrics            *RiskMetrics
	mu                 sync.RWMutex
}

// KellyStage 交易者Kelly学习阶段
type KellyStage string

const (
	StageInfant  KellyStage = "infant"  // 初生: 0-5% Kelly
	StageChild   KellyStage = "child"   // 儿童: 5-25% Kelly
	StageMature  KellyStage = "mature"  // 成熟: 25-50% Kelly
)

// FilterRule 过滤规则
type FilterRule struct {
	Stage              KellyStage
	MaxKellyFraction   float64
	MaxPositionSize    float64
	RequiredQualityMin float64
	RequiredWinRate    float64
	AllowedMemoryTypes []string
	AllowedStrategies  []string
}

// RiskMetrics 风险过滤指标
type RiskMetrics struct {
	FilterRuns      int64
	FilteredOut     int64
	KeptCount       int64
	RiskViolations  int64
	AvgFilterRate   float64
	LastFilterAt    *time.Time
	mu              sync.RWMutex
}

// FilteredResult 过滤结果
type FilteredResult struct {
	Memories        []Memory       // 过滤后的安全记忆
	RemovedCount    int            // 因风险被移除的记忆数
	RiskViolations  []RiskViolation
	CurrentStage    KellyStage
	SafetyScore     float64        // 综合安全评分 (0-1)
	Timestamp       time.Time
}

// RiskViolation 风险违规
type RiskViolation struct {
	MemoryID   string
	Reason     string
	RiskScore  float64
	MaxAllowed float64
	ActualValue float64
}

// NewRiskAwareFormatter 创建风险感知格式化器
func NewRiskAwareFormatter() *RiskAwareFormatter {
	raf := &RiskAwareFormatter{
		maxLeverageByStage: map[KellyStage]float64{
			StageInfant: 1.05,   // 5%杠杆 = 1.05倍
			StageChild:  1.25,   // 25%杠杆 = 1.25倍
			StageMature: 1.50,   // 50%杠杆 = 1.50倍
		},
		filterRules: make(map[KellyStage]FilterRule),
		metrics:     &RiskMetrics{},
	}

	// 初始化过滤规则
	raf.initializeFilterRules()

	return raf
}

// initializeFilterRules 初始化各阶段的过滤规则
func (raf *RiskAwareFormatter) initializeFilterRules() {
	raf.filterRules[StageInfant] = FilterRule{
		Stage:              StageInfant,
		MaxKellyFraction:   0.05,
		MaxPositionSize:    0.05,
		RequiredQualityMin: 0.95, // 新手需要最高质量
		RequiredWinRate:    0.85,
		AllowedMemoryTypes: []string{"decision", "outcome"},
		AllowedStrategies:  []string{"conservative", "trend_following"},
	}

	raf.filterRules[StageChild] = FilterRule{
		Stage:              StageChild,
		MaxKellyFraction:   0.25,
		MaxPositionSize:    0.15,
		RequiredQualityMin: 0.80,
		RequiredWinRate:    0.70,
		AllowedMemoryTypes: []string{"decision", "outcome", "reflection"},
		AllowedStrategies:  []string{"conservative", "trend_following", "mean_reversion"},
	}

	raf.filterRules[StageMature] = FilterRule{
		Stage:              StageMature,
		MaxKellyFraction:   0.50,
		MaxPositionSize:    0.40,
		RequiredQualityMin: 0.70,
		RequiredWinRate:    0.55,
		AllowedMemoryTypes: []string{"decision", "outcome", "reflection", "pattern"},
		AllowedStrategies:  []string{"conservative", "trend_following", "mean_reversion", "breakout"},
	}
}

// FilterMemories 按阶段过滤记忆
func (raf *RiskAwareFormatter) FilterMemories(memories []Memory, stage KellyStage) FilteredResult {
	startTime := time.Now()

	result := FilteredResult{
		Memories:       make([]Memory, 0),
		RiskViolations: make([]RiskViolation, 0),
		CurrentStage:   stage,
		Timestamp:      startTime,
	}

	if len(memories) == 0 {
		raf.recordMetrics(result)
		return result
	}

	rule, exists := raf.filterRules[stage]
	if !exists {
		log.Printf("❌ 未找到阶段%s的规则", stage)
		result.SafetyScore = 0
		return result
	}

	log.Printf("🔍 按阶段过滤: %s (最大杠杆: %.1f%%)", stage, rule.MaxKellyFraction*100)

	// 过滤记忆
	for _, m := range memories {
		// 检查1: 类型允许
		if !raf.contains(rule.AllowedMemoryTypes, m.Type) {
			result.RiskViolations = append(result.RiskViolations, RiskViolation{
				MemoryID:  m.ID,
				Reason:    fmt.Sprintf("类型%s在%s阶段不允许", m.Type, stage),
				RiskScore: 0.3,
			})
			result.RemovedCount++
			log.Printf("  ❌ 移除: %s (类型%s不允许)", idPrefix(m.ID), m.Type)
			continue
		}

		// 检查2: 质量分要求
		if m.QualityScore < rule.RequiredQualityMin {
			result.RiskViolations = append(result.RiskViolations, RiskViolation{
				MemoryID:   m.ID,
				Reason:    fmt.Sprintf("质量分%.2f低于要求%.2f", m.QualityScore, rule.RequiredQualityMin),
				RiskScore: 0.5,
				MaxAllowed: rule.RequiredQualityMin,
				ActualValue: m.QualityScore,
			})
			result.RemovedCount++
			log.Printf("  ❌ 移除: %s (质量分%.2f < %.2f)", idPrefix(m.ID), m.QualityScore, rule.RequiredQualityMin)
			continue
		}

		// 检查3: Kelly杠杆提取
		kellyFraction := raf.extractKellyFraction(m)
		if kellyFraction > rule.MaxKellyFraction {
			result.RiskViolations = append(result.RiskViolations, RiskViolation{
				MemoryID:   m.ID,
				Reason:    fmt.Sprintf("建议Kelly%.1f%%超过限制%.1f%%", kellyFraction*100, rule.MaxKellyFraction*100),
				RiskScore: 0.8,
				MaxAllowed: rule.MaxKellyFraction,
				ActualValue: kellyFraction,
			})
			result.RemovedCount++
			log.Printf("  ❌ 移除: %s (Kelly%.1f%% > %.1f%%)",
				idPrefix(m.ID), kellyFraction*100, rule.MaxKellyFraction*100)
			continue
		}

		// 检查4: 交易量大小
		positionSize := raf.extractPositionSize(m)
		if positionSize > rule.MaxPositionSize {
			result.RiskViolations = append(result.RiskViolations, RiskViolation{
				MemoryID:   m.ID,
				Reason:    fmt.Sprintf("仓位%.1f%%超过限制%.1f%%", positionSize*100, rule.MaxPositionSize*100),
				RiskScore: 0.7,
				MaxAllowed: rule.MaxPositionSize,
				ActualValue: positionSize,
			})
			result.RemovedCount++
			log.Printf("  ❌ 移除: %s (仓位%.1f%% > %.1f%%)",
				idPrefix(m.ID), positionSize*100, rule.MaxPositionSize*100)
			continue
		}

		// 通过过滤,保留记忆
		result.Memories = append(result.Memories, m)
		log.Printf("  ✅ 保留: %s (Q=%.2f, Kelly=%.1f%%)",
			idPrefix(m.ID), m.QualityScore, kellyFraction*100)
	}

	// 计算安全评分
	result.SafetyScore = raf.calculateSafetyScore(result)

	duration := time.Since(startTime)
	log.Printf("✅ 过滤完成: 保留%d条, 移除%d条, 安全评分%.2f (耗时: %.0fms)",
		len(result.Memories), result.RemovedCount, result.SafetyScore, duration.Seconds()*1000)

	raf.recordMetrics(result)
	return result
}

// extractKellyFraction 从记忆元数据提取Kelly分数
func (raf *RiskAwareFormatter) extractKellyFraction(m Memory) float64 {
	if m.Metadata == nil {
		return 0.10 // 默认10%
	}

	// 尝试从元数据中提取Kelly值
	if kelly, ok := m.Metadata["kelly_fraction"].(float64); ok {
		return kelly
	}

	// 尝试其他可能的字段
	if kelly, ok := m.Metadata["Kelly"].(float64); ok {
		return kelly
	}

	if riskReward, ok := m.Metadata["risk_reward"].(float64); ok {
		// Kelly = (winRate * riskReward - (1 - winRate)) / riskReward
		winRate := 0.55
		if wr, ok := m.Metadata["win_rate"].(float64); ok {
			winRate = wr
		}
		return (winRate*riskReward - (1 - winRate)) / riskReward
	}

	return 0.10
}

// extractPositionSize 从记忆提取仓位大小
func (raf *RiskAwareFormatter) extractPositionSize(m Memory) float64 {
	if m.Metadata == nil {
		return 0.10
	}

	if size, ok := m.Metadata["position_size"].(float64); ok {
		return size
	}

	if size, ok := m.Metadata["position"].(float64); ok {
		return size
	}

	return 0.10
}

// calculateSafetyScore 计算综合安全评分
func (raf *RiskAwareFormatter) calculateSafetyScore(result FilteredResult) float64 {
	if len(result.RiskViolations) == 0 {
		// 没有违规,评分高
		return 0.95
	}

	// 根据违规严重程度降分
	totalRiskScore := 0.0
	for _, v := range result.RiskViolations {
		totalRiskScore += v.RiskScore
	}

	avgRisk := totalRiskScore / float64(len(result.RiskViolations))
	return 1.0 - (avgRisk * 0.5) // 风险影响50%
}

// recordMetrics 记录过滤指标
func (raf *RiskAwareFormatter) recordMetrics(result FilteredResult) {
	raf.metrics.mu.Lock()
	defer raf.metrics.mu.Unlock()

	raf.metrics.FilterRuns++
	raf.metrics.FilteredOut += int64(result.RemovedCount)
	raf.metrics.KeptCount += int64(len(result.Memories))
	raf.metrics.RiskViolations += int64(len(result.RiskViolations))

	if raf.metrics.FilterRuns > 0 {
		raf.metrics.AvgFilterRate = float64(raf.metrics.FilteredOut) / float64(raf.metrics.FilterRuns + raf.metrics.KeptCount)
	}

	now := time.Now()
	raf.metrics.LastFilterAt = &now
}

// GetMetrics 获取风险过滤指标
func (raf *RiskAwareFormatter) GetMetrics() RiskMetrics {
	raf.metrics.mu.RLock()
	defer raf.metrics.mu.RUnlock()

	return *raf.metrics
}

// PrintStats 打印统计信息
func (raf *RiskAwareFormatter) PrintStats() {
	metrics := raf.GetMetrics()

	log.Println("\n🛡️ 风险感知过滤统计:")
	log.Println(strings.Repeat("═", 60))
	log.Printf("  过滤次数: %d\n", metrics.FilterRuns)
	log.Printf("  保留: %d条 | 过滤: %d条\n", metrics.KeptCount, metrics.FilteredOut)

	if metrics.FilterRuns > 0 {
		log.Printf("  平均过滤率: %.1f%%\n", metrics.AvgFilterRate*100)
	}

	log.Printf("  风险违规: %d次\n", metrics.RiskViolations)

	if metrics.LastFilterAt != nil {
		log.Printf("  最后过滤: %s\n", metrics.LastFilterAt.Format("2006-01-02 15:04:05"))
	}

	log.Println(strings.Repeat("═", 60))
}

// contains 检查字符串是否在数组中
func (raf *RiskAwareFormatter) contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

// UpdateStage 更新交易者阶段(用于学习进度跟踪)
type StageManager struct {
	currentStage    KellyStage
	stagedAt        time.Time
	successCount    int64
	totalTradesCount int64
	mu              sync.RWMutex
}

// NewStageManager 创建阶段管理器
func NewStageManager() *StageManager {
	return &StageManager{
		currentStage: StageInfant,
		stagedAt:     time.Now(),
	}
}

// RecordTrade 记录交易(用于阶段升级判断)
func (sm *StageManager) RecordTrade(successful bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.totalTradesCount++
	if successful {
		sm.successCount++
	}

	// 检查是否应该升级
	sm.checkStageUpgrade()
}

// checkStageUpgrade 检查阶段升级条件
func (sm *StageManager) checkStageUpgrade() {
	if sm.totalTradesCount < 50 {
		return // 至少50笔交易才考虑升级
	}

	winRate := float64(sm.successCount) / float64(sm.totalTradesCount)
	stageDuration := time.Since(sm.stagedAt)

	switch sm.currentStage {
	case StageInfant:
		// infant → child: 胜率>70%且交易持续2周
		if winRate > 0.70 && stageDuration > 14*24*time.Hour {
			log.Printf("📈 升级: infant → child (胜率: %.1f%%)", winRate*100)
			sm.currentStage = StageChild
			sm.stagedAt = time.Now()
		}

	case StageChild:
		// child → mature: 胜率>60%且交易持续4周
		if winRate > 0.60 && stageDuration > 28*24*time.Hour {
			log.Printf("📈 升级: child → mature (胜率: %.1f%%)", winRate*100)
			sm.currentStage = StageMature
			sm.stagedAt = time.Now()
		}
	}
}

// GetCurrentStage 获取当前阶段
func (sm *StageManager) GetCurrentStage() KellyStage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.currentStage
}

// GetStats 获取统计信息
func (sm *StageManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	winRate := 0.0
	if sm.totalTradesCount > 0 {
		winRate = float64(sm.successCount) / float64(sm.totalTradesCount)
	}

	return map[string]interface{}{
		"stage":              sm.currentStage,
		"total_trades":       sm.totalTradesCount,
		"successful":         sm.successCount,
		"win_rate":           winRate,
		"stage_duration":     time.Since(sm.stagedAt).String(),
	}
}
