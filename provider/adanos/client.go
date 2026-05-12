package adanos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.adanos.org"

	SourceRedditCrypto     = "reddit_crypto"
	SourceRedditStocks     = "reddit_stocks"
	SourceXStocks          = "x_stocks"
	SourceNewsStocks       = "news_stocks"
	SourcePolymarketStocks = "polymarket_stocks"
)

type sourceEndpoint struct {
	Path       string
	QueryParam string
	Crypto     bool
}

var sourceEndpoints = map[string]sourceEndpoint{
	SourceRedditCrypto:     {Path: "/reddit/crypto/v1/compare", QueryParam: "symbols", Crypto: true},
	SourceRedditStocks:     {Path: "/reddit/stocks/v1/compare", QueryParam: "tickers"},
	SourceXStocks:          {Path: "/x/stocks/v1/compare", QueryParam: "tickers"},
	SourceNewsStocks:       {Path: "/news/stocks/v1/compare", QueryParam: "tickers"},
	SourcePolymarketStocks: {Path: "/polymarket/stocks/v1/compare", QueryParam: "tickers"},
}

type Config struct {
	BaseURL    string
	APIKey     string
	Source     string
	Days       int
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	apiKey     string
	source     string
	days       int
	httpClient *http.Client
}

type Sentiment struct {
	Symbol         string
	Source         string
	BuzzScore      *float64
	SentimentScore *float64
	Mentions       *int
	Trend          string
	BullishPct     *float64
	BearishPct     *float64
}

func NewClient(cfg Config) *Client {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	source := NormalizeSource(cfg.Source)
	days := cfg.Days
	if days <= 0 {
		days = 7
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     strings.TrimSpace(cfg.APIKey),
		source:     source,
		days:       days,
		httpClient: httpClient,
	}
}

func NormalizeSource(source string) string {
	normalized := strings.ToLower(strings.TrimSpace(source))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	if _, ok := sourceEndpoints[normalized]; ok {
		return normalized
	}
	return SourceRedditCrypto
}

func (c *Client) Compare(ctx context.Context, symbols []string) (map[string]*Sentiment, error) {
	result := make(map[string]*Sentiment)
	if c.apiKey == "" {
		return result, fmt.Errorf("adanos API key is required")
	}

	endpoint := sourceEndpoints[c.source]
	apiSymbols, aliases := normalizeSymbols(symbols, endpoint.Crypto)
	if len(apiSymbols) == 0 {
		return result, nil
	}

	requestURL, err := c.compareURL(endpoint, apiSymbols)
	if err != nil {
		return result, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("adanos compare request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return result, err
	}

	for _, item := range extractRows(payload) {
		sentiment := parseSentiment(item, c.source)
		if sentiment.Symbol == "" {
			continue
		}

		apiSymbol := normalizeAPISymbol(sentiment.Symbol)
		if original, ok := aliases[apiSymbol]; ok {
			sentiment.Symbol = original
		}
		result[sentiment.Symbol] = sentiment
	}

	return result, nil
}

func (c *Client) compareURL(endpoint sourceEndpoint, symbols []string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	pathBase := strings.TrimRight(base.Path, "/")
	base.Path = pathBase + endpoint.Path

	q := base.Query()
	q.Set(endpoint.QueryParam, strings.Join(symbols, ","))
	q.Set("days", strconv.Itoa(c.days))
	base.RawQuery = q.Encode()
	return base.String(), nil
}

func normalizeSymbols(symbols []string, crypto bool) ([]string, map[string]string) {
	seen := make(map[string]bool)
	aliases := make(map[string]string)
	apiSymbols := make([]string, 0, len(symbols))

	for _, symbol := range symbols {
		original := normalizeAPISymbol(symbol)
		if original == "" {
			continue
		}

		apiSymbol := original
		if crypto {
			apiSymbol = trimQuoteSuffix(apiSymbol)
		}
		if apiSymbol == "" || seen[apiSymbol] {
			continue
		}

		seen[apiSymbol] = true
		aliases[apiSymbol] = original
		apiSymbols = append(apiSymbols, apiSymbol)
	}

	return apiSymbols, aliases
}

func normalizeAPISymbol(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	symbol = strings.TrimPrefix(symbol, "$")
	symbol = strings.ReplaceAll(symbol, "/", "")
	symbol = strings.ReplaceAll(symbol, "-", "")
	return symbol
}

func trimQuoteSuffix(symbol string) string {
	for _, suffix := range []string{"USDT", "USDC", "USD"} {
		if strings.HasSuffix(symbol, suffix) && len(symbol) > len(suffix) {
			return strings.TrimSuffix(symbol, suffix)
		}
	}
	return symbol
}

func extractRows(payload any) []map[string]any {
	switch data := payload.(type) {
	case []any:
		return mapRows(data)
	case map[string]any:
		for _, key := range []string{"data", "results", "items", "stocks", "tokens"} {
			if rows, ok := data[key].([]any); ok {
				return mapRows(rows)
			}
		}
		if row, ok := asMap(data); ok {
			return []map[string]any{row}
		}
	}
	return nil
}

func mapRows(rows []any) []map[string]any {
	mapped := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if item, ok := asMap(row); ok {
			mapped = append(mapped, item)
		}
	}
	return mapped
}

func asMap(value any) (map[string]any, bool) {
	item, ok := value.(map[string]any)
	return item, ok
}

func parseSentiment(item map[string]any, source string) *Sentiment {
	return &Sentiment{
		Symbol:         firstString(item, "symbol", "ticker", "token"),
		Source:         firstStringWithDefault(item, source, "source", "platform"),
		BuzzScore:      firstFloat(item, "buzz_score", "buzz"),
		SentimentScore: firstFloat(item, "sentiment_score", "sentiment"),
		Mentions:       firstInt(item, "mentions", "total_mentions", "mention_count", "trade_count"),
		Trend:          firstString(item, "trend"),
		BullishPct:     firstFloat(item, "bullish_pct", "bullish_percent"),
		BearishPct:     firstFloat(item, "bearish_pct", "bearish_percent"),
	}
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case string:
				if trimmed := strings.TrimSpace(typed); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func firstStringWithDefault(item map[string]any, fallback string, keys ...string) string {
	if value := firstString(item, keys...); value != "" {
		return value
	}
	return fallback
}

func firstFloat(item map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if parsed, ok := parseFloat(value); ok {
				return &parsed
			}
		}
	}
	return nil
}

func firstInt(item map[string]any, keys ...string) *int {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if parsed, ok := parseInt(value); ok {
				return &parsed
			}
		}
	}
	return nil
}

func parseFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func parseInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		return 0, false
	}
}
