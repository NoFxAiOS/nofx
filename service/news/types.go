package news

// Article 代表一条新闻
type Article struct {
	ID       int64  `json:"id"`
	Headline string `json:"headline"`
	Summary  string `json:"summary"`
	URL      string `json:"url"`
	Datetime int64  `json:"datetime"` // Unix timestamp
	Source   string `json:"source"`
	Category string `json:"category"`

	// AI 增强字段 (保留以兼容旧代码，但不再使用)
	TranslatedHeadline string `json:"translated_headline"`
	TranslatedSummary  string `json:"translated_summary"`
	AIProcessed        bool   `json:"ai_processed"`
	Sentiment          string `json:"sentiment"` // POSITIVE, NEGATIVE, NEUTRAL
}

// Fetcher 定义新闻抓取接口
type Fetcher interface {
	FetchNews(category string) ([]Article, error)
	Name() string
}

// Notifier 定义消息发送接口
type Notifier interface {
	Send(msg string, messageThreadID int) error
}

// StateStore 定义状态存储接口
type StateStore interface {
	GetNewsState(category string) (int64, int64, error)
	UpdateNewsState(category string, id int64, timestamp int64) error
	GetSystemConfig(key string) (string, error)
}
