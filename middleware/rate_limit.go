// Package middleware 安全中间件
// 设计哲学：防御式编程，多层保护，性能优先
package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 频率限制器
type RateLimiter struct {
	requests map[string][]time.Time
	mutex    sync.RWMutex
	limit    int
	window   time.Duration
}

// NewRateLimiter 创建频率限制器
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow 检查是否允许请求
func (r *RateLimiter) Allow(key string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// 清理过期的请求记录
	if requests, exists := r.requests[key]; exists {
		validRequests := make([]time.Time, 0)
		for _, reqTime := range requests {
			if reqTime.After(cutoff) {
				validRequests = append(validRequests, reqTime)
			}
		}
		r.requests[key] = validRequests
	}

	// 检查当前请求数量
	if len(r.requests[key]) >= r.limit {
		return false
	}

	// 记录当前请求
	r.requests[key] = append(r.requests[key], now)
	return true
}

// Cleanup 清理过期数据
func (r *RateLimiter) Cleanup() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	cutoff := time.Now().Add(-r.window)
	for key, requests := range r.requests {
		validRequests := make([]time.Time, 0)
		for _, reqTime := range requests {
			if reqTime.After(cutoff) {
				validRequests = append(validRequests, reqTime)
			}
		}
		if len(validRequests) == 0 {
			delete(r.requests, key)
		} else {
			r.requests[key] = validRequests
		}
	}
}

// RateLimitConfig 频率限制配置
type RateLimitConfig struct {
	// 积分操作限制
	CreditOperations RateLimitRule
	// 管理员操作限制
	AdminOperations RateLimitRule
	// 套餐查询限制（公开接口）
	PackageQueries RateLimitRule
}

// RateLimitRule 频率限制规则
type RateLimitRule struct {
	Window   time.Duration
	MaxCount int
}

// DefaultRateLimitConfig 默认频率限制配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		CreditOperations: RateLimitRule{
			Window:   time.Minute,
			MaxCount: 10, // 每分钟最多10次积分操作
		},
		AdminOperations: RateLimitRule{
			Window:   time.Minute,
			MaxCount: 30, // 管理员每分钟最多30次操作
		},
		PackageQueries: RateLimitRule{
			Window:   time.Minute,
			MaxCount: 60, // 公开查询每分钟最多60次
		},
	}
}

// RateLimitMiddleware 频率限制中间件
type RateLimitMiddleware struct {
	limiters map[string]*RateLimiter
	config   *RateLimitConfig
	mutex    sync.RWMutex
}

// NewRateLimitMiddleware 创建频率限制中间件
func NewRateLimitMiddleware(config *RateLimitConfig) *RateLimitMiddleware {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	return &RateLimitMiddleware{
		limiters: make(map[string]*RateLimiter),
		config:   config,
	}
}

// getLimiter 获取指定规则的频率限制器
func (m *RateLimitMiddleware) getLimiter(rule string) *RateLimiter {
	m.mutex.RLock()
	limiter, exists := m.limiters[rule]
	m.mutex.RUnlock()

	if exists {
		return limiter
	}

	// 如果不存在，创建新的限制器
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 双重检查
	if limiter, exists := m.limiters[rule]; exists {
		return limiter
	}

	var limit int
	var window time.Duration

	switch rule {
	case "credit_operations":
		limit = m.config.CreditOperations.MaxCount
		window = m.config.CreditOperations.Window
	case "admin_operations":
		limit = m.config.AdminOperations.MaxCount
		window = m.config.AdminOperations.Window
	case "package_queries":
		limit = m.config.PackageQueries.MaxCount
		window = m.config.PackageQueries.Window
	default:
		limit = 100
		window = time.Minute
	}

	limiter = NewRateLimiter(limit, window)
	m.limiters[rule] = limiter
	return limiter
}

// Middleware 创建频率限制中间件
func (m *RateLimitMiddleware) Middleware(rule string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成唯一key（基于用户ID或IP地址）
		key := generateRateLimitKey(c, rule)

		// 获取对应的限制器
		limiter := m.getLimiter(rule)

		// 检查是否超过限制
		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "请求过于频繁",
				"message": fmt.Sprintf("请在 %v 后重试", limiter.window),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// generateRateLimitKey 生成频率限制key
func generateRateLimitKey(c *gin.Context, rule string) string {
	// 优先使用用户ID
	if userID, exists := c.Get("userID"); exists {
		return fmt.Sprintf("%s:%v", rule, userID)
	}

	// 如果没有用户ID，使用IP地址
	ip := c.ClientIP()
	if ip == "" {
		ip = c.RemoteIP()
	}

	return fmt.Sprintf("%s:%s", rule, ip)
}

// StartCleanup 启动定期清理任务
func (m *RateLimitMiddleware) StartCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			m.mutex.Lock()
			for _, limiter := range m.limiters {
				limiter.Cleanup()
			}
			m.mutex.Unlock()
		}
	}()
}

// InputValidator 输入验证器
type InputValidator struct{}

// NewInputValidator 创建输入验证器
func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidateCreditPackage 验证积分套餐输入
func (v *InputValidator) ValidateCreditPackage(req interface{}) error {
	// TODO: 实现具体的验证逻辑
	return nil
}

// ValidateAdjustCredits 验证调整积分输入
func (v *InputValidator) ValidateAdjustCredits(req interface{}) error {
	// TODO: 实现具体的验证逻辑
	return nil
}

// SecurityHeadersMiddleware 安全头中间件
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置安全头
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}

// AuditLogger 审计日志记录器
type AuditLogger struct {
	// TODO: 实现审计日志记录
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger() *AuditLogger {
	return &AuditLogger{}
}

// LogAdminAction 记录管理员操作
func (a *AuditLogger) LogAdminAction(userID, action, details string) {
	// TODO: 实现审计日志记录
	// 这里可以记录到数据库或日志文件
}

// LogCreditOperation 记录积分操作
func (a *AuditLogger) LogCreditOperation(userID, operation, details string) {
	// TODO: 实现积分操作日志记录
	// 这里可以记录到数据库或日志文件
}

// RateLimitByUser 基于用户的频率限制
func RateLimitByUser(limit int, window time.Duration) gin.HandlerFunc {
	middleware := NewRateLimitMiddleware(&RateLimitConfig{
		CreditOperations: RateLimitRule{
			Window:   window,
			MaxCount: limit,
		},
	})
	return middleware.Middleware("credit_operations")
}

// RateLimitByIP 基于IP的频率限制
func RateLimitByIP(limit int, window time.Duration) gin.HandlerFunc {
	middleware := NewRateLimitMiddleware(&RateLimitConfig{
		PackageQueries: RateLimitRule{
			Window:   window,
			MaxCount: limit,
		},
	})
	return middleware.Middleware("package_queries")
}

// RateLimitAdmin 管理员操作频率限制
func RateLimitAdmin(limit int, window time.Duration) gin.HandlerFunc {
	middleware := NewRateLimitMiddleware(&RateLimitConfig{
		AdminOperations: RateLimitRule{
			Window:   window,
			MaxCount: limit,
		},
	})
	return middleware.Middleware("admin_operations")
}

// CombinedSecurityMiddleware 组合安全中间件
func CombinedSecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 安全头
		SecurityHeadersMiddleware()(c)

		// 2. 基本频率限制（基于IP）
		RateLimitByIP(60, time.Minute)(c)

		// 3. 如果请求被中止，直接返回
		if c.IsAborted() {
			return
		}

		c.Next()
	}
}

// InitSecurityMiddlewares 初始化所有安全中间件
func InitSecurityMiddlewares() {
	// 启动频率限制清理任务
	config := DefaultRateLimitConfig()
	middleware := NewRateLimitMiddleware(config)

	// 每5分钟清理一次过期数据
	middleware.StartCleanup(5 * time.Minute)

	// TODO: 初始化其他安全组件
}
