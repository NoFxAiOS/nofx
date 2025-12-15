package news

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const mlionBaseURL = "https://api.mlion.ai/v2/api/news/real/time"

// MlionFetcher 实现 Fetcher 接口
type MlionFetcher struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// MlionResponse 对应 API 响应结构
type MlionResponse struct {
	Code int             `json:"code"`
	Data []MlionNewsItem `json:"data"`
}

type MlionNewsItem struct {
	NewsID     int64  `json:"news_id"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	URL        string `json:"url"`
	CreateTime string `json:"createTime"` // Format: 2006-01-02 15:04:05
	Sentiment  int    `json:"sentiment"`
	Symbol     string `json:"symbol"`
}

// NewMlionFetcher 创建 Mlion 抓取器
func NewMlionFetcher(apiKey string) *MlionFetcher {
	return &MlionFetcher{
		apiKey:  apiKey,
		baseURL: mlionBaseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Name 返回抓取器名称
func (m *MlionFetcher) Name() string {
	return "Mlion"
}

// FetchNews 从 Mlion 获取新闻
func (m *MlionFetcher) FetchNews(category string) ([]Article, error) {
	// 动态添加 is_hot=Y 参数
	url := m.baseURL
	if strings.Contains(url, "?") {
		url += "&is_hot=Y"
	} else {
		url += "?is_hot=Y"
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-KEY", m.apiKey)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch news: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mlion api returned status: %d", resp.StatusCode)
	}

	var result MlionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Code != 200 {
		return nil, fmt.Errorf("mlion api returned code: %d", result.Code)
	}

	var articles []Article
	
	// Mlion API uses Beijing Time (UTC+8)
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600) // Fallback to fixed offset if DB missing
	}

	for _, item := range result.Data {
		// Parse Time in Beijing Time
		t, err := time.ParseInLocation("2006-01-02 15:04:05", item.CreateTime, loc)
		if err != nil {
			// Try adding simple recovery or skip
			continue
		}

		article := Article{
			ID:       item.NewsID,
			Headline: item.Title,
			Summary:  item.Content,
			URL:      item.URL,
			Datetime: t.Unix(),
			Source:   "Mlion",
			Category: "crypto", // Defaulting to crypto as Mlion seems crypto-focused
		}
		articles = append(articles, article)
	}

	return articles, nil
}