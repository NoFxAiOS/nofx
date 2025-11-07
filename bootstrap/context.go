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
func (c *Context) MustGet(key string) interface{} {
	val, ok := c.Get(key)
	if !ok {
		panic(fmt.Sprintf("context key '%s' not found", key))
	}
	return val
}
