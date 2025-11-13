package bootstrap

// Hook 初始化钩子
type Hook struct {
	Name        string               // 钩子名称（模块名）
	Priority    int                  // 优先级（越小越先执行）
	Func        func(*Context) error // 初始化函数
	Enabled     func(*Context) bool  // 条件函数，返回 false 则跳过
	ErrorPolicy ErrorPolicy          // 错误处理策略
}

// HookBuilder 钩子构建器（用于链式调用）
type HookBuilder struct {
	hook *Hook
}

// EnabledIf 设置条件函数（链式调用）
func (b *HookBuilder) EnabledIf(fn func(*Context) bool) *HookBuilder {
	b.hook.Enabled = fn
	return b
}

// OnError 设置错误处理策略（链式调用）
func (b *HookBuilder) OnError(policy ErrorPolicy) *HookBuilder {
	b.hook.ErrorPolicy = policy
	return b
}
