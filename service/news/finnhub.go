package news

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const finnhubBaseURL = "https://finnhub.io/api/v1/news"

// 过滤关键词配置
var (
	// 核心关注关键词 (扩充版)
	policyKeywords = []string{
		// --- 央行/货币政策 ---
		"Fed", "FOMC", "Powell", "interest rate", "rate cut", "rate hike",
		"PBOC", "People's Bank of China", "LPR", "reserve requirement", "China stimulus",
		"ECB", "European Central Bank", "Lagarde", "Eurozone", "monetary policy",
		"monetary easing", "fiscal stimulus", "central bank",

		// --- 经济指标 (中美欧) ---
		"CPI", "PCE", "inflation", "deflation", "non-farm", "payroll", "jobless", "unemployment",
		"GDP", "PMI", "retail sales", "factory orders",
		"Treasury yield", "bond yield", "sovereign debt",
		"export data", "import data", "trade balance",

		// --- 地缘/宏观/政策 ---
		"trade war", "tariff", "sanctions", "geopolitical", "supply chain",
		"Belt and Road", "real estate policy", "tech regulation",
		"Brexit", "EU fiscal", "energy policy", "climate policy",
		"IMF", "WTO", "World Bank", "recession", "economic outlook",
	}

	// 排除关键词 (负面过滤)
	excludedKeywords = []string{
		"sports", "football", "basketball", "soccer", "cricket", "tennis", "olympic",
		"entertainment", "movie", "music", "celebrity", "hollywood",
		"gaming", "console", "video game",
	}

	usKeywords = []string{"fed", "fomc", "powell", "treasury", "sec", "usa", "united states", "america", "dollar", "usd"}
	cnKeywords = []string{"pboc", "pbc", "china", "chinese", "beijing", "yuan", "rmb", "hk", "hong kong"}
	euKeywords = []string{"ecb", "eurozone", "european", "lagarde", "germany", "france", "uk", "britain", "euro", "eur"}

	// 权威媒体白名单 (不区分大小写匹配)
	allowedSources = []string{
		"Reuters",
		"Bloomberg",
		"Financial Times", "FT",
		"Wall Street Journal", "WSJ",
		"CNBC",
		"Caixin",
		"South China Morning Post", "SCMP",
		"Xinhua",
		"BBC",
		"The Economist",
		"IMF",
		"World Bank",
	}
)

// FinnhubFetcher 实现 Fetcher 接口
type FinnhubFetcher struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewFinnhubFetcher 创建 Finnhub 抓取器
func NewFinnhubFetcher(apiKey string) *FinnhubFetcher {
	return &FinnhubFetcher{
		apiKey:  apiKey,
		baseURL: finnhubBaseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// SetBaseURL 设置自定义 BaseURL (用于测试)
func (f *FinnhubFetcher) SetBaseURL(url string) {
	f.baseURL = url
}

// Name 返回抓取器名称
func (f *FinnhubFetcher) Name() string {
	return "Finnhub"
}

// FetchNews 从 Finnhub 获取新闻
func (f *FinnhubFetcher) FetchNews(category string) ([]Article, error) {
	url := fmt.Sprintf("%s?category=%s&token=%s", f.baseURL, category, f.apiKey)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch news: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("finnhub api returned status: %d", resp.StatusCode)
	}

	var articles []Article
	if err := json.NewDecoder(resp.Body).Decode(&articles); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 过滤和处理新闻
	var filteredArticles []Article
	for _, article := range articles {
		// 1. 来源白名单校验
		if !f.isAllowedSource(article.Source) {
			continue
		}

		// 组合标题和摘要进行检查
		text := strings.ToLower(article.Headline + " " + article.Summary)

		// 2. 负面过滤 (排除无关话题)
		if f.containsAny(text, excludedKeywords) {
			continue
		}

		// 3. 检查是否命中任意核心关键词
		matched := false
		for _, kw := range policyKeywords {
			if strings.Contains(text, strings.ToLower(kw)) {
				matched = true
				break
			}
		}

		if !matched {
			continue
		}

		// 4. 确定区域
		region := "全球/其他"
		if f.containsAny(text, usKeywords) {
			region = "美国"
		} else if f.containsAny(text, cnKeywords) {
			region = "中国"
		} else if f.containsAny(text, euKeywords) {
			region = "欧洲"
		}

		// 格式化标题：[区域] 原标题
		article.Headline = fmt.Sprintf("【%s】%s", region, article.Headline)
		article.Category = category

		filteredArticles = append(filteredArticles, article)
	}

	return filteredArticles, nil
}

// isAllowedSource 检查消息源是否在白名单中 (不区分大小写)
func (f *FinnhubFetcher) isAllowedSource(source string) bool {
	if source == "" {
		return false
	}
	lowerSource := strings.ToLower(source)
	for _, allowed := range allowedSources {
		if strings.Contains(lowerSource, strings.ToLower(allowed)) {
			return true
		}
	}
	return false
}

// containsAny 检查文本是否包含任意关键词
func (f *FinnhubFetcher) containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
