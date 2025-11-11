package bootstrap

import (
	"context"
	"fmt"
	"nofx/config"
	"sync"
)

// Context 初始化上下文，用于在钩子之间传递数据
type Context struct {
	Config *config.Config
	Data   map[string]interface{} // 存储模块之间共享的数据（如数据库实例）
	ctx    context.Context
	mu     sync.RWMutex
}

// NewContext 创建新的初始化上下文
func NewContext(cfg *config.Config) *Context {
	return &Context{
		Config: cfg,
		Data:   make(map[string]interface{}),
		ctx:    context.Background(),
	}
}

// Set 存储数据到上下文
func (c *Context) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Data[key] = value
}

// Get 从上下文获取数据
func (c *Context) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.Data[key]
	return val, ok
}

// MustGet 从上下文获取数据，不存在则 panic
// ⚠️  警告：此方法会在 key 不存在时 panic，仅在确信 key 必定存在时使用
// 建议：优先使用 Get() 或 GetOrDefault() 以获得更好的错误处理
func (c *Context) MustGet(key string) interface{} {
	val, ok := c.Get(key)
	if !ok {
		// 提供更详细的错误信息以便调试
		availableKeys := make([]string, 0, len(c.Data))
		for k := range c.Data {
			availableKeys = append(availableKeys, k)
		}
		panic(fmt.Sprintf("context key '%s' not found. Available keys: %v", key, availableKeys))
	}
	return val
}

// GetOrDefault 从上下文获取数据，不存在则返回默认值（更安全的替代方案）
func (c *Context) GetOrDefault(key string, defaultValue interface{}) interface{} {
	val, ok := c.Get(key)
	if !ok {
		return defaultValue
	}
	return val
}
