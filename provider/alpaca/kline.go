package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nofx/config"
	"time"
)

const (
	DataAPIURL = "https://data.alpaca.markets/v2"
)

// Bar represents a single OHLCV bar from Alpaca
type Bar struct {
	Timestamp  time.Time `json:"t"`
	Open       float64   `json:"o"`
	High       float64   `json:"h"`
	Low        float64   `json:"l"`
	Close      float64   `json:"c"`
	Volume     uint64    `json:"v"`
	TradeCount uint64    `json:"n"`
	VWAP       float64   `json:"vw"`
}

// BarsResponse represents the response from Alpaca bars API
type BarsResponse struct {
	Bars          []Bar  `json:"bars"`
	Symbol        string `json:"symbol"`
	NextPageToken string `json:"next_page_token"`
}

// Client is the Alpaca API client
type Client struct {
	apiKey    string
	secretKey string
	client    *http.Client
}

// NewClient creates a new Alpaca client from config
func NewClient() *Client {
	cfg := config.Get()
	return &Client{
		apiKey:    cfg.AlpacaAPIKey,
		secretKey: cfg.AlpacaSecretKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithKeys creates a new Alpaca client with provided keys
func NewClientWithKeys(apiKey, secretKey string) *Client {
	return &Client{
		apiKey:    apiKey,
		secretKey: secretKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetBars fetches historical bars for a symbol
// timeframe: 1Min, 5Min, 15Min, 30Min, 1Hour, 4Hour, 1Day, 1Week, 1Month
func (c *Client) GetBars(ctx context.Context, symbol string, timeframe string, limit int) ([]Bar, error) {
	if c.apiKey == "" || c.secretKey == "" {
		return nil, fmt.Errorf("alpaca API keys not configured")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/stocks/%s/bars", DataAPIURL, symbol)
	params := url.Values{}
	params.Set("timeframe", timeframe)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("adjustment", "raw")
	params.Set("feed", "iex") // Use IEX feed (free tier)

	// Set time range: last 30 days for intraday, last 2 years for daily
	now := time.Now()
	var start time.Time
	switch timeframe {
	case "1Day", "1Week", "1Month":
		start = now.AddDate(-2, 0, 0) // 2 years back
	default:
		start = now.AddDate(0, 0, -30) // 30 days back for intraday
	}
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", now.Format(time.RFC3339))

	fullURL := endpoint + "?" + params.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set auth headers
	req.Header.Set("APCA-API-KEY-ID", c.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.secretKey)

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result BarsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Bars, nil
}

// MapTimeframe maps common timeframe strings to Alpaca format
func MapTimeframe(interval string) string {
	switch interval {
	case "1m":
		return "1Min"
	case "3m":
		return "1Min" // Alpaca doesn't have 3m, use 1m
	case "5m":
		return "5Min"
	case "10m":
		return "15Min" // Alpaca doesn't have 10m, use 15m
	case "15m":
		return "15Min"
	case "30m":
		return "30Min"
	case "1h":
		return "1Hour"
	case "2h":
		return "1Hour" // Alpaca doesn't have 2h, use 1h
	case "4h":
		return "4Hour"
	case "6h":
		return "4Hour" // Alpaca doesn't have 6h, use 4h
	case "8h":
		return "4Hour" // Alpaca doesn't have 8h, use 4h
	case "12h":
		return "4Hour" // Alpaca doesn't have 12h, use 4h
	case "1d":
		return "1Day"
	case "3d":
		return "1Day" // Alpaca doesn't have 3d, use 1d
	case "1w":
		return "1Week"
	case "1M":
		return "1Month"
	default:
		return "5Min" // Default to 5 minutes
	}
}
