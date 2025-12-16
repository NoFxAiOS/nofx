package config

import (
	"os"
	"strconv"
	"time"
)

// TransactionConfig 事务配置
type TransactionConfig struct {
	// 事务超时时间（默认5秒）
	Timeout time.Duration
	// 最大重试次数
	MaxRetries int
	// 重试间隔
	RetryInterval time.Duration
}

// DefaultTransactionConfig 默认事务配置
func DefaultTransactionConfig() *TransactionConfig {
	return &TransactionConfig{
		Timeout:       5 * time.Second,
		MaxRetries:    3,
		RetryInterval: 1 * time.Second,
	}
}

// LoadTransactionConfigFromEnv 从环境变量加载事务配置
func LoadTransactionConfigFromEnv() *TransactionConfig {
	config := DefaultTransactionConfig()

	// 读取事务超时时间（秒）
	if timeoutStr := os.Getenv("TRANSACTION_TIMEOUT_SECONDS"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			config.Timeout = time.Duration(timeout) * time.Second
		}
	}

	// 读取最大重试次数
	if retriesStr := os.Getenv("TRANSACTION_MAX_RETRIES"); retriesStr != "" {
		if retries, err := strconv.Atoi(retriesStr); err == nil && retries >= 0 {
			config.MaxRetries = retries
		}
	}

	// 读取重试间隔（秒）
	if intervalStr := os.Getenv("TRANSACTION_RETRY_INTERVAL_SECONDS"); intervalStr != "" {
		if interval, err := strconv.Atoi(intervalStr); err == nil && interval > 0 {
			config.RetryInterval = time.Duration(interval) * time.Second
		}
	}

	return config
}

// GetTransactionTimeout 获取事务超时时间
func (c *TransactionConfig) GetTransactionTimeout() time.Duration {
	if c.Timeout <= 0 {
		return 5 * time.Second // 默认5秒
	}
	return c.Timeout
}

// GetMaxRetries 获取最大重试次数
func (c *TransactionConfig) GetMaxRetries() int {
	if c.MaxRetries < 0 {
		return 0
	}
	return c.MaxRetries
}

// GetRetryInterval 获取重试间隔
func (c *TransactionConfig) GetRetryInterval() time.Duration {
	if c.RetryInterval <= 0 {
		return 1 * time.Second // 默认1秒
	}
	return c.RetryInterval
}
