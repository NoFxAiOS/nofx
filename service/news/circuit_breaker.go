package news

import (
	"errors"
	"log"
	"sync"
	"time"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker 实现断路器模式以防止级联故障
// 当API持续失败时，快速失败而不是等待超时
type CircuitBreaker struct {
	failureCount   int
	successCount   int
	lastFailTime   time.Time
	state          string // "closed", "open", "half-open"
	failureThreshold int
	successThreshold int
	cooldownPeriod time.Duration
	mu             sync.RWMutex
	logger         *log.Logger
}

// NewCircuitBreaker 创建一个新的断路器
// failureThreshold: 多少次连续失败后打开断路器 (推荐: 3)
// cooldownPeriod: 打开多久后转为half-open状态 (推荐: 60s)
func NewCircuitBreaker(failureThreshold int, cooldownPeriod time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 3
	}
	if cooldownPeriod <= 0 {
		cooldownPeriod = 60 * time.Second
	}

	return &CircuitBreaker{
		failureCount:     0,
		successCount:     0,
		lastFailTime:     time.Time{},
		state:            "closed",
		failureThreshold: failureThreshold,
		successThreshold: 2, // 半开状态下需要2次成功才能关闭
		cooldownPeriod:   cooldownPeriod,
		logger:           log.New(log.Writer(), "[CircuitBreaker] ", log.LstdFlags),
	}
}

// State 返回当前断路器状态
func (cb *CircuitBreaker) State() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsClosed 断路器是否关闭（允许请求）
func (cb *CircuitBreaker) IsClosed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == "closed"
}

// IsOpen 断路器是否打开（拒绝所有请求）
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == "open"
}

// Call 执行受断路器保护的函数
// 如果断路器打开，立即返回错误（快速失败）
// 否则执行fn，记录成功/失败，管理状态转换
func (cb *CircuitBreaker) Call(fn func() error) error {
	// 1. 检查是否可以执行（在锁内快速完成）
	cb.mu.Lock()
	canProceed, shouldTransition := cb.canProceedLocked()
	cb.mu.Unlock()

	if !canProceed {
		return ErrCircuitOpen
	}

	// 2. 执行用户函数（无锁）
	err := fn()

	// 3. 根据结果更新状态（原子操作）
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if shouldTransition && cb.state == "half-open" {
		cb.logger.Printf("🟡 Half-open: attempting recovery...")
		cb.state = "half-open"
		cb.successCount = 0
	}

	cb.recordResultLocked(err)
	return err
}

// canProceedLocked 检查是否可以执行（必须在锁内调用）
func (cb *CircuitBreaker) canProceedLocked() (bool, bool) {
	if cb.state == "closed" || cb.state == "half-open" {
		return true, false
	}

	// open状态检查是否可以进入half-open
	if cb.state == "open" && time.Since(cb.lastFailTime) > cb.cooldownPeriod {
		return true, true
	}

	return false, false
}

// recordResultLocked 记录执行结果（必须在锁内调用）
func (cb *CircuitBreaker) recordResultLocked(err error) {
	if err != nil {
		cb.failureCount++
		cb.lastFailTime = time.Now()
		cb.successCount = 0

		cb.logger.Printf("❌ Call failed (count: %d/%d)", cb.failureCount, cb.failureThreshold)

		// 检查是否需要打开断路器
		if cb.failureCount >= cb.failureThreshold {
			cb.state = "open"
			cb.logger.Printf("🔴 Circuit breaker OPENED after %d failures", cb.failureCount)
		}

		return
	}

	// 成功
	cb.failureCount = 0

	if cb.state == "half-open" {
		cb.successCount++
		cb.logger.Printf("✅ Success in half-open (count: %d/%d)", cb.successCount, cb.successThreshold)

		if cb.successCount >= cb.successThreshold {
			cb.state = "closed"
			cb.logger.Printf("🟢 Circuit breaker CLOSED - recovered!")
		}
	} else if cb.state == "closed" {
		cb.logger.Printf("✅ Call succeeded")
	}
}

// Reset 手动重置断路器为关闭状态
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0
	cb.successCount = 0
	cb.state = "closed"
	cb.logger.Printf("🔵 Circuit breaker reset to CLOSED")
}

// GetMetrics 返回断路器的当前指标
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stateValue := 0
	if cb.state == "open" {
		stateValue = 1
	} else if cb.state == "half-open" {
		stateValue = 2
	}

	return map[string]interface{}{
		"state":          cb.state,
		"state_value":    stateValue, // 0: closed, 1: open, 2: half-open
		"failure_count":  cb.failureCount,
		"success_count":  cb.successCount,
		"last_fail_time": cb.lastFailTime.Unix(),
	}
}
