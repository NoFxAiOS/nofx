package decision

import (
	"log"
)

// ContextEnricher 定义了上下文增强器的接口
// 上下文增强器负责将外部数据源（新闻、社交情绪等）添加到决策上下文中
// 所有实现必须遵循fail-safe原则：失败不应该阻止决策进行
type ContextEnricher interface {
	// Name 返回增强器的名称（用于日志和识别）
	Name() string

	// Enrich 增强给定的上下文
	// 错误处理：非致命错误应该记录但不返回（graceful degradation）
	// 上下文应该通过 ctx.SetExtension() 进行修改
	Enrich(ctx *Context) error

	// IsEnabled 检查增强器是否启用
	// 返回true表示应该运行Enrich()
	IsEnabled(ctx *Context) bool
}

// EnrichmentChain 管理多个上下文增强器的执行链
// 增强器按顺序执行，一个失败不会影响其他增强器
type EnrichmentChain struct {
	enrichers []ContextEnricher
	logger    *log.Logger
}

// NewEnrichmentChain 创建一个新的增强链
func NewEnrichmentChain() *EnrichmentChain {
	return &EnrichmentChain{
		enrichers: make([]ContextEnricher, 0),
		logger:    log.New(log.Writer(), "[EnrichmentChain] ", log.LstdFlags),
	}
}

// AddEnricher 添加一个增强器到链中
func (ec *EnrichmentChain) AddEnricher(enricher ContextEnricher) *EnrichmentChain {
	if enricher != nil {
		ec.enrichers = append(ec.enrichers, enricher)
	}
	return ec // 允许链式调用
}

// ExecuteAll 按顺序执行所有启用的增强器
// 如果任何增强器失败，继续执行其他增强器（fail-safe）
// 返回所有遇到的错误（用于监控，但不会中断流程）
func (ec *EnrichmentChain) ExecuteAll(ctx *Context) []error {
	var errors []error

	for _, enricher := range ec.enrichers {
		if enricher == nil {
			continue
		}

		name := enricher.Name()

		// 检查增强器是否启用
		if !enricher.IsEnabled(ctx) {
			ec.logger.Printf("⏭️  %s enricher disabled", name)
			continue
		}

		// 执行增强器
		ec.logger.Printf("🔄 %s enricher running...", name)
		err := enricher.Enrich(ctx)

		if err != nil {
			// 记录错误但继续（fail-safe）
			ec.logger.Printf("⚠️  %s enricher failed: %v (continuing)", name, err)
			errors = append(errors, err)
		} else {
			ec.logger.Printf("✅ %s enricher succeeded", name)
		}
	}

	return errors
}

// ExecuteAllNonFatal 执行所有增强器，忽略错误（仅用于logging）
// 用于您想让失败无声的场景
func (ec *EnrichmentChain) ExecuteAllNonFatal(ctx *Context) {
	_ = ec.ExecuteAll(ctx)
}
