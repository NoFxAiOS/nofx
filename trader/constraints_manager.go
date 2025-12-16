package trader

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// LearningStage 学习阶段枚举
type LearningStage int

const (
	StageInfant LearningStage = 1 // 婴儿期: 1-5笔交易
	StageChild  LearningStage = 2 // 学童期: 5-20笔交易
	StageMature LearningStage = 3 // 成熟期: 20+笔交易
)

// Constraints 约束条件结构体
type Constraints struct {
	Stage               LearningStage
	MaxLeverage         int     // 最大杠杆
	MaxDailyLoss        float64 // 最大日亏损
	MaxSingleLoss       float64 // 单笔最大亏损
	MinHoldingMinutes   int     // 最小持仓时间(分钟)
	MaxConcurrentPos    int     // 最大并发仓位数
	AllowExceptionForAI bool    // 是否允许AI例外放权
}

// ConstraintsManager AI决策约束管理器
type ConstraintsManager struct {
	mu                 sync.RWMutex
	currentStage       LearningStage
	totalTrades        int
	consecutiveLosses  int
	dailyTrades        []TradeResult
	dailyResetTime     time.Time
	dailyLossAmount    float64
	currentPositions   int
	decisionRejections int
	lastDecisionTime   time.Time
}

// TradeResult 交易结果记录 (用于统计)
type TradeResult struct {
	Timestamp time.Time
	IsWin     bool
	PnL       float64
	PnLPct    float64
}

// NewConstraintsManager 创建约束管理器
func NewConstraintsManager() *ConstraintsManager {
	return &ConstraintsManager{
		currentStage:   StageInfant,
		totalTrades:    0,
		dailyResetTime: time.Now(),
		dailyTrades:    make([]TradeResult, 0),
	}
}

// GetCurrentConstraints 获取当前阶段的约束条件
func (cm *ConstraintsManager) GetCurrentConstraints() Constraints {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	switch cm.currentStage {
	case StageInfant:
		return Constraints{
			Stage:               StageInfant,
			MaxLeverage:         1,
			MaxDailyLoss:        0.05,  // 日亏损最多5%
			MaxSingleLoss:       0.03,  // 单笔最多3%
			MinHoldingMinutes:   30,    // 最少持30分钟
			MaxConcurrentPos:    1,     // 最多1个仓位
			AllowExceptionForAI: false, // 不允许例外
		}
	case StageChild:
		return Constraints{
			Stage:               StageChild,
			MaxLeverage:         2,
			MaxDailyLoss:        0.08, // 日亏损最多8%
			MaxSingleLoss:       0.04, // 单笔最多4%
			MinHoldingMinutes:   15,   // 最少持15分钟
			MaxConcurrentPos:    2,    // 最多2个仓位
			AllowExceptionForAI: false,
		}
	case StageMature:
		return Constraints{
			Stage:               StageMature,
			MaxLeverage:         5,
			MaxDailyLoss:        0.12, // 日亏损最多12%
			MaxSingleLoss:       0.06, // 单笔最多6%
			MinHoldingMinutes:   0,    // 无最小持仓时间限制
			MaxConcurrentPos:    3,    // 最多3个仓位
			AllowExceptionForAI: true, // 允许AI例外放权
		}
	default:
		return cm.GetCurrentConstraints()
	}
}

// ValidateDecision 验证AI决策是否符合约束
// 返回 (是否通过, 拒绝原因)
func (cm *ConstraintsManager) ValidateDecision(
	leverage int,
	estimatedLoss float64,
	position *Position,
) (bool, string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	constraints := cm.GetCurrentConstraints()

	// 约束1: 杠杆上限
	if leverage > constraints.MaxLeverage {
		return false, fmt.Sprintf(
			"❌ 约束拦截: 杠杆%dx超过阶段%d限制%dx",
			leverage, constraints.Stage, constraints.MaxLeverage,
		)
	}

	// 约束2: 日亏损限制
	if cm.dailyLossAmount+estimatedLoss > 100.0*constraints.MaxDailyLoss {
		return false, fmt.Sprintf(
			"❌ 约束拦截: 日亏损已达%.2f%%,超过限制%.2f%%",
			(cm.dailyLossAmount+estimatedLoss)/100.0*100,
			constraints.MaxDailyLoss*100,
		)
	}

	// 约束3: 单笔亏损限制
	if estimatedLoss > 100.0*constraints.MaxSingleLoss {
		return false, fmt.Sprintf(
			"❌ 约束拦截: 单笔预估亏损%.2f%%超过限制%.2f%%",
			estimatedLoss/100.0*100,
			constraints.MaxSingleLoss*100,
		)
	}

	// 约束4: 并发仓位限制
	if cm.currentPositions >= constraints.MaxConcurrentPos {
		return false, fmt.Sprintf(
			"❌ 约束拦截: 当前仓位%d已达上限%d",
			cm.currentPositions, constraints.MaxConcurrentPos,
		)
	}

	log.Printf("✅ 约束验证通过: 杠杆=%dx, 预估亏损=%.2f%%, 并发仓位=%d/%d",
		leverage, estimatedLoss/100.0*100, cm.currentPositions+1, constraints.MaxConcurrentPos)

	return true, ""
}

// RecordTradeResult 记录交易结果并更新阶段
func (cm *ConstraintsManager) RecordTradeResult(isWin bool, pnl, pnlPct float64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 更新日结果
	cm.dailyTrades = append(cm.dailyTrades, TradeResult{
		Timestamp: time.Now(),
		IsWin:     isWin,
		PnL:       pnl,
		PnLPct:    pnlPct,
	})

	// 更新交易计数和连续亏损
	cm.totalTrades++
	if !isWin {
		cm.consecutiveLosses++
		cm.dailyLossAmount += pnl
		// 触发警告: 连续5笔亏损
		if cm.consecutiveLosses >= 5 {
			log.Printf("🚨 连续%d笔亏损,建议暂停交易检查策略", cm.consecutiveLosses)
		}
	} else {
		cm.consecutiveLosses = 0
	}

	// 自动更新阶段
	cm.updateStage()

	log.Printf("📊 交易记录: #%d %s (PnL=%.2f, PnLPct=%.2f%%), 阶段=%d, 连续亏损=%d",
		cm.totalTrades,
		map[bool]string{true: "✓", false: "✗"}[isWin],
		pnl, pnlPct*100,
		cm.currentStage, cm.consecutiveLosses,
	)
}

// updateStage 自动更新学习阶段
func (cm *ConstraintsManager) updateStage() {
	oldStage := cm.currentStage

	if cm.totalTrades >= 20 {
		cm.currentStage = StageMature
	} else if cm.totalTrades >= 5 {
		cm.currentStage = StageChild
	} else {
		cm.currentStage = StageInfant
	}

	if oldStage != cm.currentStage {
		log.Printf("🎯 学习阶段更新: %d → %d (交易数: %d)", oldStage, cm.currentStage, cm.totalTrades)
	}
}

// CheckDailyReset 检查是否需要重置日数据
func (cm *ConstraintsManager) CheckDailyReset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	if now.Sub(cm.dailyResetTime) >= 24*time.Hour {
		log.Printf("📅 日度重置: 清除日交易数据, 日亏损=%.2f%%", cm.dailyLossAmount/100.0*100)
		cm.dailyTrades = make([]TradeResult, 0)
		cm.dailyLossAmount = 0
		cm.dailyResetTime = now
	}
}

// GetDailyStats 获取日度统计
func (cm *ConstraintsManager) GetDailyStats() (totalTrades int, wins int, losses int, totalPnL float64) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	totalTrades = len(cm.dailyTrades)
	totalPnL = 0

	for _, trade := range cm.dailyTrades {
		if trade.IsWin {
			wins++
		} else {
			losses++
		}
		totalPnL += trade.PnL
	}

	return totalTrades, wins, losses, totalPnL
}

// GetStageInfo 获取阶段信息
func (cm *ConstraintsManager) GetStageInfo() (stage LearningStage, totalTrades int, rejections int) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.currentStage, cm.totalTrades, cm.decisionRejections
}

// RejectDecision 记录被拒绝的决策
func (cm *ConstraintsManager) RejectDecision(reason string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.decisionRejections++
	log.Printf("⛔ 决策被拦截 (#%d): %s", cm.decisionRejections, reason)

	// 如果拒绝率过高,输出警告
	rejectionRate := float64(cm.decisionRejections) / float64(cm.totalTrades+1)
	if rejectionRate > 0.3 {
		log.Printf("⚠️ 决策拒绝率%.1f%%过高,约束可能过严", rejectionRate*100)
	}
}

// PositionTracker 持仓跟踪
type PositionTracker struct {
	mu        sync.RWMutex
	positions map[string]*Position
}

// Position 持仓信息
type Position struct {
	Symbol        string
	OpenPrice     float64
	OpenTime      time.Time
	Leverage      int
	PositionSize  float64
	UnrealizedPnL float64
}

// NewPositionTracker 创建持仓跟踪器
func NewPositionTracker() *PositionTracker {
	return &PositionTracker{
		positions: make(map[string]*Position),
	}
}

// OpenPosition 打开持仓
func (pt *PositionTracker) OpenPosition(pos *Position) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.positions[pos.Symbol] = pos
	log.Printf("📈 开仓: %s @ %.6f, 杠杆=%dx, 仓位=%.2f", pos.Symbol, pos.OpenPrice, pos.Leverage, pos.PositionSize)
}

// ClosePosition 平仓
func (pt *PositionTracker) ClosePosition(symbol string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pos, ok := pt.positions[symbol]; ok {
		log.Printf("📉 平仓: %s, 未实现盈亏=%.2f", symbol, pos.UnrealizedPnL)
		delete(pt.positions, symbol)
	}
}

// GetPositionCount 获取当前仓位数
func (pt *PositionTracker) GetPositionCount() int {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return len(pt.positions)
}

// GetPosition 获取指定币种的持仓信息
func (pt *PositionTracker) GetPosition(symbol string) *Position {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.positions[symbol]
}

// GetAllPositions 获取所有持仓
func (pt *PositionTracker) GetAllPositions() []*Position {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	positions := make([]*Position, 0, len(pt.positions))
	for _, pos := range pt.positions {
		positions = append(positions, pos)
	}
	return positions
}
