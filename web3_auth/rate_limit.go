package web3_auth

import (
	"sync"
	"time"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	mu        sync.RWMutex
	requests  map[string][]time.Time // key -> 请求时间列表
	limit     int                    // 限制次数
	window    time.Duration          // 时间窗口
	cleanupCh chan struct{}          // 清理通道
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	// 启动定期清理
	rl.cleanupCh = make(chan struct{})
	go rl.cleanup()

	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 清理过期的请求记录
	rl.cleanupKeyLocked(key, now)

	// 获取当前请求列表
	requests := rl.requests[key]

	// 检查是否超过限制
	if len(requests) >= rl.limit {
		return false
	}

	// 记录新请求
	rl.requests[key] = append(requests, now)
	return true
}

// GetRemaining 获取剩余请求次数
func (rl *RateLimiter) GetRemaining(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	now := time.Now()
	rl.cleanupKeyLocked(key, now)

	remaining := rl.limit - len(rl.requests[key])
	if remaining < 0 {
		return 0
	}

	return remaining
}

// GetResetTime 获取重置时间
func (rl *RateLimiter) GetResetTime(key string) time.Time {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	requests := rl.requests[key]
	if len(requests) == 0 {
		return time.Now()
	}

	// 返回最早的请求过期时间
	return requests[0].Add(rl.window)
}

// cleanup 定期清理过期记录
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key := range rl.requests {
				rl.cleanupKeyLocked(key, now)
				// 如果没有请求记录了，删除该key
				if len(rl.requests[key]) == 0 {
					delete(rl.requests, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.cleanupCh:
			return
		}
	}
}

// cleanupKeyLocked 清理指定key的过期记录（需要锁保护）
func (rl *RateLimiter) cleanupKeyLocked(key string, now time.Time) {
	if requests, ok := rl.requests[key]; ok {
		// 过滤出在时间窗口内的请求
		validRequests := make([]time.Time, 0, len(requests))
		for _, reqTime := range requests {
			if now.Sub(reqTime) < rl.window {
				validRequests = append(validRequests, reqTime)
			}
		}
		rl.requests[key] = validRequests
	}
}

// Stop 停止速率限制器
func (rl *RateLimiter) Stop() {
	close(rl.cleanupCh)
}

// 预定义的速率限制器
var (
	// IP速率限制：每分钟10次
	IPRateLimiter = NewRateLimiter(10, time.Minute)

	// 地址速率限制：每分钟5次
	AddressRateLimiter = NewRateLimiter(5, time.Minute)

	// 用户速率限制：每分钟100次
	UserRateLimiter = NewRateLimiter(100, time.Minute)
)
