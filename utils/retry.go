package utils

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts uint          // 最大重试次数 (默认3次)
	Delay       time.Duration // 初始延迟 (默认1秒)
	MaxDelay    time.Duration // 最大延迟 (默认10秒)
	Multiplier  float64       // 指数退避倍数 (默认2.0)
	Jitter      bool          // 是否添加随机抖动 (默认true)
	LogRetries  bool          // 是否记录重试日志 (默认true)
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	Delay:       1 * time.Second,
	MaxDelay:    10 * time.Second,
	Multiplier:  2.0,
	Jitter:      true,
	LogRetries:  true,
}

// APIRetryConfig API专用重试配置 (更激进的重试)
var APIRetryConfig = RetryConfig{
	MaxAttempts: 5,
	Delay:       500 * time.Millisecond,
	MaxDelay:    8 * time.Second,
	Multiplier:  2.0,
	Jitter:      true,
	LogRetries:  true,
}

// NetworkRetryConfig 网络请求专用配置 (快速重试)
var NetworkRetryConfig = RetryConfig{
	MaxAttempts: 4,
	Delay:       200 * time.Millisecond,
	MaxDelay:    5 * time.Second,
	Multiplier:  1.5,
	Jitter:      true,
	LogRetries:  true,
}

// RetryableFunc 可重试的函数类型
type RetryableFunc func() error

// RetryWithConfig 使用指定配置进行重试
func RetryWithConfig(config RetryConfig, fn RetryableFunc, context ...string) error {
	contextStr := strings.Join(context, " ")
	if contextStr == "" {
		contextStr = "操作"
	}

	var retryOptions []retry.Option

	// 基础配置
	retryOptions = append(retryOptions, retry.Attempts(config.MaxAttempts))
	retryOptions = append(retryOptions, retry.Delay(config.Delay))

	// 指数退避配置
	if config.Jitter {
		retryOptions = append(retryOptions, retry.DelayType(retry.BackOffDelay))
	} else {
		retryOptions = append(retryOptions, retry.DelayType(retry.FixedDelay))
	}

	// 日志配置
	if config.LogRetries {
		retryOptions = append(retryOptions, retry.OnRetry(func(n uint, err error) {
			log.Printf("⚠️ %s重试 #%d: %v", contextStr, n+1, err)
		}))
	}

	// 可重试错误条件 (默认所有错误都重试，除非是特定的不可重试错误)
	retryOptions = append(retryOptions, retry.RetryIf(func(err error) bool {
		if err == nil {
			return false
		}

		errStr := strings.ToLower(err.Error())

		// 不可重试的错误类型
		nonRetryableErrors := []string{
			"unauthorized",      // 401 认证错误
			"forbidden",         // 403 权限错误
			"not found",         // 404 资源不存在
			"bad request",       // 400 请求格式错误
			"invalid signature", // 签名错误
			"invalid key",       // 密钥错误
			"cancelled",         // 用户取消
		}

		for _, nonRetryable := range nonRetryableErrors {
			if strings.Contains(errStr, nonRetryable) {
				return false // 不重试
			}
		}

		return true // 其他错误都重试
	}))

	// 执行重试
	err := retry.Do(func() error {
		return fn()
	}, retryOptions...)

	if err != nil {
		log.Printf("❌ %s重试%d次后仍然失败: %v", contextStr, config.MaxAttempts, err)
	}

	return err
}

// Retry 使用默认配置进行重试
func Retry(fn RetryableFunc, context ...string) error {
	return RetryWithConfig(DefaultRetryConfig, fn, context...)
}

// RetryAPI 使用API专用配置进行重试
func RetryAPI(fn RetryableFunc, context ...string) error {
	return RetryWithConfig(APIRetryConfig, fn, context...)
}

// RetryNetwork 使用网络专用配置进行重试
func RetryNetwork(fn RetryableFunc, context ...string) error {
	return RetryWithConfig(NetworkRetryConfig, fn, context...)
}

// RetryWithContext 带上下文的重试 (支持取消)
func RetryWithContext(ctx context.Context, config RetryConfig, fn func() error, context ...string) error {
	contextStr := strings.Join(context, " ")
	if contextStr == "" {
		contextStr = "操作"
	}

	return retry.Do(
		func() error { return fn() },
		retry.Context(ctx),
		retry.Attempts(config.MaxAttempts),
		retry.Delay(config.Delay),
		retry.DelayType(retry.BackOffDelay),
		retry.OnRetry(func(n uint, err error) {
			if config.LogRetries {
				log.Printf("⚠️ %s重试 #%d: %v", contextStr, n+1, err)
			}
		}),
	)
}

// QuickRetry 快速重试 (3次，每次500ms)
func QuickRetry(fn RetryableFunc, context ...string) error {
	config := RetryConfig{
		MaxAttempts: 3,
		Delay:       500 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		Multiplier:  1.5,
		Jitter:      false,
		LogRetries:  true,
	}
	return RetryWithConfig(config, fn, context...)
}

// AggressiveRetry 激进重试 (10次，指数退避)
func AggressiveRetry(fn RetryableFunc, context ...string) error {
	config := RetryConfig{
		MaxAttempts: 10,
		Delay:       100 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
		LogRetries:  true,
	}
	return RetryWithConfig(config, fn, context...)
}

// IsRetryableError 检查错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// 常见的可重试错误
	retryableErrors := []string{
		"timeout",
		"connection reset",
		"connection refused",
		"no such host",
		"temporary failure",
		"eof",
		"broken pipe",
		"network is unreachable",
		"service unavailable",
		"internal server error",
		"bad gateway",
		"gateway timeout",
		"too many requests",
		"rate limit",
		"need to retry",
		"try again",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	return false
}
