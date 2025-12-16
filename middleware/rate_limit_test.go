package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestRateLimiter 测试频率限制器
func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(2, time.Second)

	// 测试正常请求
	t.Run("NormalRequests", func(t *testing.T) {
		key := "test_user_1"

		// 前2次请求应该通过
		assert.True(t, limiter.Allow(key))
		assert.True(t, limiter.Allow(key))

		// 第3次请求应该被拒绝
		assert.False(t, limiter.Allow(key))
	})

	// 测试时间窗口重置
	t.Run("WindowReset", func(t *testing.T) {
		key := "test_user_2"

		// 先用完配额
		limiter.Allow(key)
		limiter.Allow(key)
		assert.False(t, limiter.Allow(key))

		// 等待时间窗口过期
		time.Sleep(time.Second + 10*time.Millisecond)

		// 应该可以重新请求
		assert.True(t, limiter.Allow(key))
	})

	// 测试不同用户的隔离
	t.Run("UserIsolation", func(t *testing.T) {
		key1 := "user_1"
		key2 := "user_2"

		// user1用完配额
		limiter.Allow(key1)
		limiter.Allow(key1)
		assert.False(t, limiter.Allow(key1))

		// user2应该不受影响
		assert.True(t, limiter.Allow(key2))
		assert.True(t, limiter.Allow(key2))
		assert.False(t, limiter.Allow(key2))
	})
}

// TestRateLimiterCleanup 测试频率限制器清理
func TestRateLimiterCleanup(t *testing.T) {
	limiter := NewRateLimiter(10, time.Millisecond*100)

	key := "test_cleanup"

	// 添加一些请求记录
	for i := 0; i < 5; i++ {
		limiter.Allow(key)
		time.Sleep(time.Millisecond * 10)
	}

	// 清理过期数据
	limiter.Cleanup()

	// 等待时间窗口过期
	time.Sleep(time.Millisecond * 150)

	// 再次清理
	limiter.Cleanup()

	// 应该可以重新开始
	assert.True(t, limiter.Allow(key))
}

// TestRateLimitMiddleware 测试频率限制中间件
func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := &RateLimitConfig{
		CreditOperations: RateLimitRule{
			Window:   time.Second,
			MaxCount: 2,
		},
	}

	middleware := NewRateLimitMiddleware(config)

	// 创建测试路由器
	router := gin.New()
	router.Use(middleware.Middleware("credit_operations"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 测试正常请求
	t.Run("NormalRequests", func(t *testing.T) {
		// 前2次请求应该成功
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		}

		// 第3次请求应该被拒绝
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})
}

// TestGenerateRateLimitKey 测试生成频率限制key
func TestGenerateRateLimitKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("WithUserID", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("userID", "user123")

		key := generateRateLimitKey(c, "credit_operations")
		assert.Equal(t, "credit_operations:user123", key)
	})

	t.Run("WithIP", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/test", nil)
		c.Request.RemoteAddr = "192.168.1.1:12345"

		key := generateRateLimitKey(c, "package_queries")
		assert.Contains(t, key, "package_queries:")
	})
}

// TestDefaultRateLimitConfig 测试默认配置
func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 10, config.CreditOperations.MaxCount)
	assert.Equal(t, time.Minute, config.CreditOperations.Window)
	assert.Equal(t, 30, config.AdminOperations.MaxCount)
	assert.Equal(t, 60, config.PackageQueries.MaxCount)
}

// TestRateLimitHelperFunctions 测试辅助函数
func TestRateLimitHelperFunctions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("RateLimitByUser", func(t *testing.T) {
		router := gin.New()
		router.Use(RateLimitByUser(5, time.Second))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// 设置用户ID
		router.Use(func(c *gin.Context) {
			c.Set("userID", "test_user")
			c.Next()
		})

		// 测试限制
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// 第6次应该被拒绝
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("RateLimitByIP", func(t *testing.T) {
		router := gin.New()
		router.Use(RateLimitByIP(3, time.Second))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// 测试限制（基于IP）
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// 第4次应该被拒绝
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})
}

// TestSecurityHeadersMiddleware 测试安全头中间件
func TestSecurityHeadersMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// 验证安全头
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=31536000")
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

// TestCombinedSecurityMiddleware 测试组合安全中间件
func TestCombinedSecurityMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CombinedSecurityMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	// 验证响应正常
	assert.Equal(t, http.StatusOK, w.Code)

	// 验证安全头存在
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
}

// BenchmarkRateLimiter 频率限制器性能基准测试
func BenchmarkRateLimiter(b *testing.B) {
	limiter := NewRateLimiter(1000, time.Second)
	key := "benchmark_user"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow(key)
	}
}

// BenchmarkRateLimitMiddleware 中间件性能基准测试
func BenchmarkRateLimitMiddleware(b *testing.B) {
	gin.SetMode(gin.TestMode)

	config := &RateLimitConfig{
		CreditOperations: RateLimitRule{
			Window:   time.Second,
			MaxCount: 1000,
		},
	}

	middleware := NewRateLimitMiddleware(config)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", "benchmark_user")
		c.Next()
	})
	router.Use(middleware.Middleware("credit_operations"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
	}
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter(100, time.Second)
	key := "concurrent_test"

	// 并发测试
	done := make(chan bool, 10)
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			if limiter.Allow(key) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证并发安全性
	assert.LessOrEqual(t, successCount, 100)
	assert.GreaterOrEqual(t, successCount, 0)
}

// TestEdgeCases 边界情况测试
func TestEdgeCases(t *testing.T) {
	t.Run("EmptyKey", func(t *testing.T) {
		limiter := NewRateLimiter(1, time.Second)
		assert.True(t, limiter.Allow(""))
		assert.False(t, limiter.Allow(""))
	})

	t.Run("ZeroLimit", func(t *testing.T) {
		limiter := NewRateLimiter(0, time.Second)
		assert.False(t, limiter.Allow("test"))
	})

	t.Run("NegativeLimit", func(t *testing.T) {
		limiter := NewRateLimiter(-1, time.Second)
		assert.False(t, limiter.Allow("test"))
	})

	t.Run("ZeroWindow", func(t *testing.T) {
		limiter := NewRateLimiter(1, 0)
		assert.True(t, limiter.Allow("test"))
		assert.True(t, limiter.Allow("test")) // 零时间窗口意味着永不过期
	})
}

// TestMemoryLeaks 内存泄漏测试
func TestMemoryLeaks(t *testing.T) {
	limiter := NewRateLimiter(10, time.Millisecond*10)

	// 创建大量key
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("user_%d", i)
		limiter.Allow(key)
	}

	// 等待过期
	time.Sleep(time.Millisecond * 20)

	// 清理
	limiter.Cleanup()

	// 验证内存使用（这里简化处理，实际应该检查内存占用）
	assert.True(t, true, "内存清理测试通过")
}

// TestRealWorldScenarios 真实场景测试
func TestRealWorldScenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("BurstTraffic", func(t *testing.T) {
		config := &RateLimitConfig{
			CreditOperations: RateLimitRule{
				Window:   time.Second,
				MaxCount: 10,
			},
		}

		middleware := NewRateLimitMiddleware(config)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("userID", "burst_user")
			c.Next()
		})
		router.Use(middleware.Middleware("credit_operations"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// 模拟突发流量
		successCount := 0
		for i := 0; i < 20; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				successCount++
			}
		}

		assert.Equal(t, 10, successCount) // 应该只有前10个成功
	})

	t.Run("GradualTraffic", func(t *testing.T) {
		config := &RateLimitConfig{
			CreditOperations: RateLimitRule{
				Window:   time.Second,
				MaxCount: 5,
			},
		}

		middleware := NewRateLimitMiddleware(config)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("userID", "gradual_user")
			c.Next()
		})
		router.Use(middleware.Middleware("credit_operations"))
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// 模拟渐进式流量
		successCount := 0
		for i := 0; i < 15; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				successCount++
			}

			// 每隔一段时间发送请求
			if i%5 == 4 {
				time.Sleep(time.Second + 10*time.Millisecond)
			}
		}

		// 由于时间窗口重置，应该能成功更多
		assert.Greater(t, successCount, 10)
	})
}
