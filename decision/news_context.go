package decision

import (
	"time"
)

// Article 新闻文章结构（来自新闻API）
type Article struct {
	ID       int64  `json:"id"`
	Headline string `json:"headline"`
	Summary  string `json:"summary"`
	URL      string `json:"url"`
	Datetime int64  `json:"datetime"` // Unix timestamp
	Source   string `json:"source"`
	Category string `json:"category"`
	Symbol   string `json:"symbol"` // 相关的币种
	Sentiment int    `json:"sentiment"` // -1: negative, 0: neutral, 1: positive
}

// NewsContext 新闻上下文（包含最近的市场新闻和情绪）
// 此结构是不可变的，创建后不应修改
type NewsContext struct {
	Articles     []Article `json:"articles"`       // 最近的新闻文章列表
	SentimentAvg float64   `json:"sentiment_avg"` // 平均情绪 (-1.0 to +1.0)
	TopCategories []string  `json:"top_categories"` // 新闻top分类
	FetchedAt    int64     `json:"fetched_at"`    // Unix timestamp，何时获取的新闻
	Enabled      bool      `json:"enabled"`       // 新闻集成是否启用
	FetchError   string    `json:"fetch_error,omitempty"` // 获取失败时的错误信息（用于日志）
}

// NewEmptyNewsContext 创建一个禁用的空新闻上下文
// 用于在新闻获取失败或禁用时使用
func NewEmptyNewsContext() *NewsContext {
	return &NewsContext{
		Articles:     []Article{},
		SentimentAvg: 0.0,
		TopCategories: []string{},
		FetchedAt:    0,
		Enabled:      false,
		FetchError:   "",
	}
}

// NewNewsContext 创建一个启用的新闻上下文
func NewNewsContext(articles []Article) *NewsContext {
	ctx := &NewsContext{
		Articles:      articles,
		SentimentAvg:  calculateSentiment(articles),
		TopCategories: extractCategories(articles),
		FetchedAt:     time.Now().Unix(),
		Enabled:       true,
		FetchError:    "",
	}
	return ctx
}

// calculateSentiment 计算平均情绪分数
func calculateSentiment(articles []Article) float64 {
	if len(articles) == 0 {
		return 0.0
	}

	var sum int
	for _, a := range articles {
		sum += a.Sentiment
	}

	return float64(sum) / float64(len(articles))
}

// extractCategories 提取top分类
func extractCategories(articles []Article) []string {
	categories := make(map[string]int)

	for _, a := range articles {
		if a.Category != "" {
			categories[a.Category]++
		}
	}

	// 返回出现最多的分类（最多5个）
	result := make([]string, 0)
	if len(categories) > 0 {
		// 简单的排序逻辑：按出现次数降序
		for cat := range categories {
			result = append(result, cat)
			if len(result) >= 5 {
				break
			}
		}
	}

	return result
}

// HasArticles 检查是否有文章
func (nc *NewsContext) HasArticles() bool {
	return nc != nil && len(nc.Articles) > 0
}

// GetTopArticles 获取top N篇文章
func (nc *NewsContext) GetTopArticles(n int) []Article {
	if nc == nil || len(nc.Articles) == 0 {
		return []Article{}
	}

	if n < 0 || n > len(nc.Articles) {
		n = len(nc.Articles)
	}
	return nc.Articles[:n]
}

// SentimentLabel 返回情绪的文字标签
func (nc *NewsContext) SentimentLabel() string {
	if nc == nil {
		return "neutral"
	}
	if nc.SentimentAvg >= 0.2 {  // 改为 >= (修复边界)
		return "positive"
	} else if nc.SentimentAvg <= -0.2 {  // 改为 <= (修复边界)
		return "negative"
	}
	return "neutral"
}
