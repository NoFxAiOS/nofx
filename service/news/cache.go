package news

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrCacheMiss   = errors.New("cache miss")
	ErrCacheExpired = errors.New("cache expired")
)

// NewsCache 新闻缓存接口
type NewsCache interface {
	// Get 获取缓存的文章（如果缓存有效）
	// 返回文章列表和错误（如果缓存无效或不存在）
	Get(category string) ([]Article, error)

	// Set 设置缓存的文章
	// ttlMinutes: 缓存有效期（分钟）
	Set(category string, articles []Article, ttlMinutes int)

	// IsStale 检查缓存是否过期
	IsStale(category string, maxAgeMinutes int) bool

	// Clear 清除缓存
	Clear(category string)
}

// InMemoryCache 内存中的新闻缓存实现
type InMemoryCache struct {
	cache map[string]*cachedArticles
	mu    sync.RWMutex
}

// cachedArticles 缓存的文章及其元数据
type cachedArticles struct {
	articles  []Article
	cachedAt  time.Time
	ttlMinutes int
}

// NewInMemoryCache 创建一个新的内存缓存
func NewInMemoryCache(defaultTTLMinutes int) NewsCache {
	if defaultTTLMinutes <= 0 {
		defaultTTLMinutes = 5
	}

	return &InMemoryCache{
		cache: make(map[string]*cachedArticles),
	}
}

// Get 获取缓存的文章
func (ic *InMemoryCache) Get(category string) ([]Article, error) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	cached, exists := ic.cache[category]
	if !exists {
		return nil, ErrCacheMiss
	}

	// 检查是否过期
	if time.Since(cached.cachedAt) > time.Duration(cached.ttlMinutes)*time.Minute {
		return nil, ErrCacheExpired
	}

	// 返回副本以防止外部修改
	articles := make([]Article, len(cached.articles))
	copy(articles, cached.articles)

	return articles, nil
}

// Set 设置缓存
func (ic *InMemoryCache) Set(category string, articles []Article, ttlMinutes int) {
	if ttlMinutes <= 0 {
		ttlMinutes = 5
	}

	ic.mu.Lock()
	defer ic.mu.Unlock()

	// 创建副本以防止外部修改
	articlesCopy := make([]Article, len(articles))
	copy(articlesCopy, articles)

	ic.cache[category] = &cachedArticles{
		articles:   articlesCopy,
		cachedAt:   time.Now(),
		ttlMinutes: ttlMinutes,
	}
}

// IsStale 检查缓存是否过期
func (ic *InMemoryCache) IsStale(category string, maxAgeMinutes int) bool {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	cached, exists := ic.cache[category]
	if !exists {
		return true
	}

	age := time.Since(cached.cachedAt)
	return age > time.Duration(maxAgeMinutes)*time.Minute
}

// Clear 清除缓存
func (ic *InMemoryCache) Clear(category string) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	delete(ic.cache, category)
}

// ClearAll 清除所有缓存
func (ic *InMemoryCache) ClearAll() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.cache = make(map[string]*cachedArticles)
}

// GetSize 获取缓存大小（用于监控）
func (ic *InMemoryCache) GetSize() int {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	return len(ic.cache)
}

// GetCacheInfo 获取缓存信息（用于调试）
func (ic *InMemoryCache) GetCacheInfo(category string) map[string]interface{} {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	cached, exists := ic.cache[category]
	if !exists {
		return map[string]interface{}{
			"exists": false,
		}
	}

	age := time.Since(cached.cachedAt)
	return map[string]interface{}{
		"exists":      true,
		"articles":    len(cached.articles),
		"cached_at":   cached.cachedAt.Unix(),
		"age_seconds": int(age.Seconds()),
		"ttl_minutes": cached.ttlMinutes,
		"is_expired":  age > time.Duration(cached.ttlMinutes)*time.Minute,
	}
}
