package perception

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// NewsItem represents a crypto news headline.
type NewsItem struct {
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	URL       string    `json:"url"`
	Sentiment string    `json:"sentiment"` // "bullish", "bearish", "neutral"
	Symbols   []string  `json:"symbols"`   // Related symbols
	Timestamp time.Time `json:"timestamp"`
}

// NewsMonitor fetches crypto news and detects sentiment shifts.
type NewsMonitor struct {
	httpClient *http.Client
	logger     *slog.Logger
	lastCheck  time.Time
	seenURLs   map[string]bool
}

// NewNewsMonitor creates a new news monitor.
func NewNewsMonitor(logger *slog.Logger) *NewsMonitor {
	return &NewsMonitor{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		logger:     logger,
		seenURLs:   make(map[string]bool),
	}
}

// FetchNews gets recent crypto news from CryptoCompare (free, no auth needed).
func (n *NewsMonitor) FetchNews() ([]NewsItem, error) {
	url := "https://min-api.cryptocompare.com/data/v2/news/?lang=EN&sortOrder=latest"
	resp, err := n.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch news: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			Title      string `json:"title"`
			Source     string `json:"source"`
			URL        string `json:"url"`
			Body       string `json:"body"`
			Categories string `json:"categories"`
			PublishedOn int64  `json:"published_on"`
		} `json:"Data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse news: %w", err)
	}

	var items []NewsItem
	for _, d := range result.Data {
		if n.seenURLs[d.URL] {
			continue
		}
		n.seenURLs[d.URL] = true

		item := NewsItem{
			Title:     d.Title,
			Source:    d.Source,
			URL:       d.URL,
			Sentiment: classifySentiment(d.Title + " " + d.Body),
			Symbols:   extractSymbols(d.Title + " " + d.Categories),
			Timestamp: time.Unix(d.PublishedOn, 0),
		}
		items = append(items, item)
	}

	// Keep seen URLs map from growing forever
	if len(n.seenURLs) > 1000 {
		n.seenURLs = make(map[string]bool)
	}

	n.lastCheck = time.Now()
	return items, nil
}

// classifySentiment does basic keyword-based sentiment analysis.
func classifySentiment(text string) string {
	lower := strings.ToLower(text)

	bullish := []string{"surge", "rally", "soar", "bullish", "breakout", "all-time high", "ath",
		"pump", "moon", "gain", "rise", "uptrend", "buy signal", "accumulate", "adoption"}
	bearish := []string{"crash", "dump", "plunge", "bearish", "sell-off", "selloff", "decline",
		"drop", "fall", "liquidat", "hack", "exploit", "ban", "fraud", "scam", "risk"}

	bullCount, bearCount := 0, 0
	for _, w := range bullish {
		if strings.Contains(lower, w) {
			bullCount++
		}
	}
	for _, w := range bearish {
		if strings.Contains(lower, w) {
			bearCount++
		}
	}

	if bullCount > bearCount {
		return "bullish"
	}
	if bearCount > bullCount {
		return "bearish"
	}
	return "neutral"
}

// extractSymbols finds crypto symbols mentioned in text.
func extractSymbols(text string) []string {
	upper := strings.ToUpper(text)
	known := []string{"BTC", "ETH", "SOL", "BNB", "XRP", "DOGE", "ADA", "AVAX", "DOT", "LINK", "MATIC", "UNI", "AAVE"}
	var found []string
	for _, s := range known {
		if strings.Contains(upper, s) {
			found = append(found, s)
		}
	}
	return found
}
