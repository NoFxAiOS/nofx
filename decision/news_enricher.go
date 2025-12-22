package decision

import (
	"fmt"
	"log"
	"nofx/service/news"
	"time"
)

// NewsEnricher 实现 ContextEnricher 接口，将新闻数据添加到决策上下文
type NewsEnricher struct {
	cache      news.NewsCache
	breaker    *news.CircuitBreaker
	mlionAPI   *news.MlionFetcher
	logger     *log.Logger
	enabled    bool
}

// NewNewsEnricher 创建一个新的新闻增强器
func NewNewsEnricher(mlionAPI *news.MlionFetcher) *NewsEnricher {
	return &NewsEnricher{
		cache: news.NewInMemoryCache(5), // 5分钟缓存
		breaker: news.NewCircuitBreaker(3, 60*time.Second), // 3次失败后打开，60秒冷却
		mlionAPI: mlionAPI,
		logger: log.New(log.Writer(), "[NewsEnricher] ", log.LstdFlags),
		enabled: true,
	}
}

// Name 返回增强器的名称
func (ne *NewsEnricher) Name() string {
	return "news"
}

// IsEnabled 检查新闻增强器是否启用
func (ne *NewsEnricher) IsEnabled(ctx *Context) bool {
	if !ne.enabled || ctx == nil {
		return false
	}
	// 这里可以添加更多的启用检查逻辑
	// 例如：检查全局feature flag、用户配置等
	return true
}

// Enrich 增强上下文，将新闻数据添加到Extensions中
// 遵循fail-safe原则：如果失败，返回禁用的NewsContext
func (ne *NewsEnricher) Enrich(ctx *Context) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}

	startTime := time.Now()

	// 通过断路器获取新闻（快速失败保护）
	var newsCtx *NewsContext
	err := ne.breaker.Call(func() error {
		articles, err := ne.fetchNewsWithCache()
		if err != nil {
			return err
		}

		// 创建新闻上下文
		newsCtx = NewNewsContext(articles)
		return nil
	})

	fetchDuration := time.Since(startTime)

	if err != nil {
		// 失败时返回禁用的新闻上下文（graceful degradation）
		ne.logger.Printf("⚠️  Failed to fetch news: %v (duration: %v, CB state: %s)",
			err, fetchDuration, ne.breaker.State())

		// 创建禁用的新闻上下文
		newsCtx = NewEmptyNewsContext()
		newsCtx.FetchError = err.Error()
	} else {
		ne.logger.Printf("✅ News fetched successfully: %d articles (duration: %v, CB state: %s)",
			len(newsCtx.Articles), fetchDuration, ne.breaker.State())
	}

	// 清洁新闻数据（防止prompt injection）
	SanitizeNewsContext(newsCtx)

	// 将新闻上下文添加到扩展
	ctx.SetExtension("news", newsCtx)

	return nil
}

// fetchNewsWithCache 从缓存或API获取新闻
func (ne *NewsEnricher) fetchNewsWithCache() ([]Article, error) {
	if ne.mlionAPI == nil {
		return nil, fmt.Errorf("mlion API not configured")
	}

	// 尝试从缓存获取（快速路径）
	cachedArticles, err := ne.cache.Get("crypto")
	if err == nil && len(cachedArticles) > 0 {
		ne.logger.Printf("📦 Cache hit: %d articles", len(cachedArticles))
		// 转换news.Article为decision.Article
		return convertNewsArticles(cachedArticles), nil
	}

	// 缓存miss：从API获取
	ne.logger.Printf("🔄 Cache miss, fetching from Mlion API...")
	apiArticles, err := ne.mlionAPI.FetchNews("crypto")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Mlion: %w", err)
	}

	// 存储到缓存
	if len(apiArticles) > 0 {
		ne.cache.Set("crypto", apiArticles, 5) // 5分钟TTL
	}

	ne.logger.Printf("✅ Fetched from API: %d articles", len(apiArticles))
	return convertNewsArticles(apiArticles), nil
}

// convertNewsArticles 将news.Article转换为decision.Article
func convertNewsArticles(newsArticles []news.Article) []Article {
	articles := make([]Article, len(newsArticles))
	for i, na := range newsArticles {
		// 将Sentiment字符串转换为整数
		sentiment := 0
		switch na.Sentiment {
		case "POSITIVE":
			sentiment = 1
		case "NEGATIVE":
			sentiment = -1
		default:
			sentiment = 0
		}

		articles[i] = Article{
			ID:       na.ID,
			Headline: na.Headline,
			Summary:  na.Summary,
			URL:      na.URL,
			Datetime: na.Datetime,
			Source:   na.Source,
			Category: na.Category,
			Symbol:   "", // news.Article没有Symbol字段，使用空字符串
			Sentiment: sentiment,
		}
	}
	return articles
}

// SetEnabled 设置新闻增强器的启用状态
func (ne *NewsEnricher) SetEnabled(enabled bool) {
	ne.enabled = enabled
	if !enabled {
		ne.logger.Printf("⏭️  News enricher disabled")
	} else {
		ne.logger.Printf("✅ News enricher enabled")
	}
}

// GetCircuitBreakerState 获取断路器状态（用于监控）
func (ne *NewsEnricher) GetCircuitBreakerState() string {
	return ne.breaker.State()
}

// GetMetrics 获取增强器的性能指标
func (ne *NewsEnricher) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"enabled":           ne.enabled,
		"circuit_breaker":   ne.breaker.GetMetrics(),
	}
}

// Reset 重置增强器状态（用于测试或恢复）
func (ne *NewsEnricher) Reset() {
	ne.breaker.Reset()
	ne.logger.Printf("🔵 News enricher reset")
}
