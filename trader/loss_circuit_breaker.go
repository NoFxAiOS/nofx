package trader

import (
	"fmt"
	"log"
	"nofx/database"
	"sync"
	"time"
)

// LossCircuitBreaker 损失断路器 - 防止灾难性亏损
// 实现硬性限制：连续亏损、日亏损、周亏损、最大回撤
type LossCircuitBreaker struct {
	traderID string
	db       *database.Database

	// Hard limits configuration
	MaxConsecutiveLosses   int     // 最多连续亏损笔数（默认：5）
	MaxDailyLossPercent    float64 // 日亏损上限百分比（默认：12%）
	MaxWeeklyLossPercent   float64 // 周亏损上限百分比（默认：20%）
	MaxDrawdownPercent     float64 // 最大回撤上限（默认：15%）

	// Current state tracking
	consecutiveLosses      int
	todayPnLPercent        float64
	weeklyPnLPercent       float64
	currentDrawdownPercent float64
	accountPeak            float64
	lastAccountValue       float64

	// Thread safety
	mu sync.RWMutex

	// Event tracking
	breachedAt        time.Time
	breachType        string
	breachReason      string
	isBroken          bool
	recoveryAttempt   int
}

// NewLossCircuitBreaker 创建新的断路器
func NewLossCircuitBreaker(traderID string, db *database.Database) *LossCircuitBreaker {
	return &LossCircuitBreaker{
		traderID:               traderID,
		db:                     db,
		MaxConsecutiveLosses:   5,
		MaxDailyLossPercent:    12.0,
		MaxWeeklyLossPercent:   20.0,
		MaxDrawdownPercent:     15.0,
		accountPeak:            100.0,
		lastAccountValue:       100.0,
	}
}

// CanTrade 检查是否允许交易
// 返回 (允许, 原因)
func (lcb *LossCircuitBreaker) CanTrade() (bool, string) {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()

	// 检查断路器是否已触发
	if lcb.isBroken {
		return false, fmt.Sprintf(
			"🚨 断路器已触发 (%s): %s | 触发时间: %s ago",
			lcb.breachType, lcb.breachReason,
			time.Since(lcb.breachedAt).String())
	}

	// 检查连续亏损
	if lcb.consecutiveLosses >= lcb.MaxConsecutiveLosses {
		return false, fmt.Sprintf(
			"⚠️ 硬性限制: %d笔连续亏损 (上限: %d)",
			lcb.consecutiveLosses, lcb.MaxConsecutiveLosses)
	}

	// 检查日亏损限制
	if lcb.todayPnLPercent < -lcb.MaxDailyLossPercent {
		return false, fmt.Sprintf(
			"⚠️ 硬性限制: 日亏损 %.2f%% 超过上限 %.2f%%",
			lcb.todayPnLPercent, lcb.MaxDailyLossPercent)
	}

	// 检查周亏损限制
	if lcb.weeklyPnLPercent < -lcb.MaxWeeklyLossPercent {
		return false, fmt.Sprintf(
			"⚠️ 硬性限制: 周亏损 %.2f%% 超过上限 %.2f%%",
			lcb.weeklyPnLPercent, lcb.MaxWeeklyLossPercent)
	}

	// 检查回撤限制
	if lcb.currentDrawdownPercent > lcb.MaxDrawdownPercent {
		return false, fmt.Sprintf(
			"⚠️ 硬性限制: 回撤 %.2f%% 超过上限 %.2f%%",
			lcb.currentDrawdownPercent, lcb.MaxDrawdownPercent)
	}

	return true, ""
}

// UpdateAfterTrade 在交易后更新断路器状态
func (lcb *LossCircuitBreaker) UpdateAfterTrade(
	isWin bool,
	pnlPercent float64,
	currentAccountValue float64,
) {
	lcb.mu.Lock()
	defer lcb.mu.Unlock()

	if lcb.isBroken {
		return // 一旦断路器触发，停止更新
	}

	// 更新连续亏损
	if !isWin {
		lcb.consecutiveLosses++

		if lcb.consecutiveLosses >= lcb.MaxConsecutiveLosses {
			lcb.triggerBreaker("consecutive_loss",
				fmt.Sprintf("%d笔连续亏损", lcb.consecutiveLosses))
			log.Printf("🚨 [%s] 断路器触发: 连续%d笔亏损 (上限: %d)",
				lcb.traderID, lcb.consecutiveLosses, lcb.MaxConsecutiveLosses)
		}
	} else {
		lcb.consecutiveLosses = 0
	}

	// 更新账户价值
	lcb.lastAccountValue = currentAccountValue

	// 更新账户峰值和回撤
	if currentAccountValue > lcb.accountPeak {
		lcb.accountPeak = currentAccountValue
	}

	if lcb.accountPeak > 0 {
		lcb.currentDrawdownPercent = ((lcb.accountPeak - currentAccountValue) / lcb.accountPeak) * 100

		if lcb.currentDrawdownPercent > lcb.MaxDrawdownPercent {
			lcb.triggerBreaker("max_drawdown",
				fmt.Sprintf("回撤 %.2f%% 超过上限 %.2f%%",
					lcb.currentDrawdownPercent, lcb.MaxDrawdownPercent))
			log.Printf("🚨 [%s] 断路器触发: 回撤 %.2f%% (上限: %.2f%%)",
				lcb.traderID, lcb.currentDrawdownPercent, lcb.MaxDrawdownPercent)
		}
	}

	// 更新日亏损（需要查询今日交易）
	if lcb.shouldCheckDailyLimit() {
		lcb.updateDailyPnL()

		if lcb.todayPnLPercent < -lcb.MaxDailyLossPercent {
			lcb.triggerBreaker("daily_loss",
				fmt.Sprintf("日亏损 %.2f%% 超过上限 %.2f%%",
					lcb.todayPnLPercent, lcb.MaxDailyLossPercent))
			log.Printf("🚨 [%s] 断路器触发: 日亏损 %.2f%% (上限: %.2f%%)",
				lcb.traderID, lcb.todayPnLPercent, lcb.MaxDailyLossPercent)
		}
	}

	// 更新周亏损（需要查询本周交易）
	if lcb.shouldCheckWeeklyLimit() {
		lcb.updateWeeklyPnL()

		if lcb.weeklyPnLPercent < -lcb.MaxWeeklyLossPercent {
			lcb.triggerBreaker("weekly_loss",
				fmt.Sprintf("周亏损 %.2f%% 超过上限 %.2f%%",
					lcb.weeklyPnLPercent, lcb.MaxWeeklyLossPercent))
			log.Printf("🚨 [%s] 断路器触发: 周亏损 %.2f%% (上限: %.2f%%)",
				lcb.traderID, lcb.weeklyPnLPercent, lcb.MaxWeeklyLossPercent)
		}
	}
}

// triggerBreaker 触发断路器
func (lcb *LossCircuitBreaker) triggerBreaker(breachType, reason string) {
	lcb.isBroken = true
	lcb.breachedAt = time.Now()
	lcb.breachType = breachType
	lcb.breachReason = reason

	// 记录到数据库
	if lcb.db != nil {
		lcb.logLossEvent(breachType, reason)
	}
}

// logLossEvent 记录亏损事件
func (lcb *LossCircuitBreaker) logLossEvent(eventType, reason string) {
	// 这里会在实现数据库持久化时填充
	// 暂时仅记录日志
	log.Printf("📊 [%s] 亏损事件: type=%s, reason=%s, time=%s",
		lcb.traderID, eventType, reason, time.Now().Format("2006-01-02 15:04:05"))
}

// updateDailyPnL 更新日亏损百分比
func (lcb *LossCircuitBreaker) updateDailyPnL() {
	// TODO: 查询数据库获取今天的所有交易
	// 计算日PnL百分比
	// lcb.todayPnLPercent = ...
}

// updateWeeklyPnL 更新周亏损百分比
func (lcb *LossCircuitBreaker) updateWeeklyPnL() {
	// TODO: 查询数据库获取本周的所有交易
	// 计算周PnL百分比
	// lcb.weeklyPnLPercent = ...
}

// shouldCheckDailyLimit 是否应该检查日限制
func (lcb *LossCircuitBreaker) shouldCheckDailyLimit() bool {
	// 在交易时检查
	return true
}

// shouldCheckWeeklyLimit 是否应该检查周限制
func (lcb *LossCircuitBreaker) shouldCheckWeeklyLimit() bool {
	// 在交易时检查
	return true
}

// GetStatus 获取断路器状态
func (lcb *LossCircuitBreaker) GetStatus() map[string]interface{} {
	lcb.mu.RLock()
	defer lcb.mu.RUnlock()

	return map[string]interface{}{
		"is_broken":              lcb.isBroken,
		"breach_type":            lcb.breachType,
		"breach_reason":          lcb.breachReason,
		"consecutive_losses":     lcb.consecutiveLosses,
		"today_pnl_percent":      lcb.todayPnLPercent,
		"weekly_pnl_percent":     lcb.weeklyPnLPercent,
		"current_drawdown":       lcb.currentDrawdownPercent,
		"account_peak":           lcb.accountPeak,
		"last_account_value":     lcb.lastAccountValue,
		"breached_at":            lcb.breachedAt.String(),
		"max_consecutive_limit":  lcb.MaxConsecutiveLosses,
		"max_daily_loss_limit":   lcb.MaxDailyLossPercent,
		"max_weekly_loss_limit":  lcb.MaxWeeklyLossPercent,
		"max_drawdown_limit":     lcb.MaxDrawdownPercent,
	}
}

// Reset 重置断路器（用于每日或每周重置）
func (lcb *LossCircuitBreaker) Reset(reason string) {
	lcb.mu.Lock()
	defer lcb.mu.Unlock()

	log.Printf("🔄 [%s] 断路器重置: %s", lcb.traderID, reason)

	lcb.isBroken = false
	lcb.breachedAt = time.Time{}
	lcb.breachType = ""
	lcb.breachReason = ""
	lcb.consecutiveLosses = 0
	lcb.recoveryAttempt++
}

// SetLimits 设置断路器限制
func (lcb *LossCircuitBreaker) SetLimits(
	maxConsecutiveLosses int,
	maxDailyLossPercent float64,
	maxWeeklyLossPercent float64,
	maxDrawdownPercent float64,
) {
	lcb.mu.Lock()
	defer lcb.mu.Unlock()

	lcb.MaxConsecutiveLosses = maxConsecutiveLosses
	lcb.MaxDailyLossPercent = maxDailyLossPercent
	lcb.MaxWeeklyLossPercent = maxWeeklyLossPercent
	lcb.MaxDrawdownPercent = maxDrawdownPercent

	log.Printf("⚙️ [%s] 断路器限制已更新: consecutive=%d, daily=%.1f%%, weekly=%.1f%%, drawdown=%.1f%%",
		lcb.traderID,
		maxConsecutiveLosses,
		maxDailyLossPercent,
		maxWeeklyLossPercent,
		maxDrawdownPercent)
}
