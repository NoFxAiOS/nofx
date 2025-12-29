package twelvedata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nofx/config"
	"strconv"
	"time"
)

const (
	BaseURL = "https://api.twelvedata.com"
)

// Bar represents a single OHLCV bar from Twelve Data
type Bar struct {
	Datetime string  `json:"datetime"`
	Open     string  `json:"open"`
	High     string  `json:"high"`
	Low      string  `json:"low"`
	Close    string  `json:"close"`
	Volume   string  `json:"volume,omitempty"`
}

// TimeSeriesResponse represents the response from Twelve Data time_series API
type TimeSeriesResponse struct {
	Meta   Meta   `json:"meta"`
	Values []Bar  `json:"values"`
	Status string `json:"status"`
	Code   int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// Meta contains metadata about the time series
type Meta struct {
	Symbol           string `json:"symbol"`
	Interval         string `json:"interval"`
	CurrencyBase     string `json:"currency_base,omitempty"`
	CurrencyQuote    string `json:"currency_quote,omitempty"`
	Type             string `json:"type,omitempty"`
	Exchange         string `json:"exchange,omitempty"`
	ExchangeTimezone string `json:"exchange_timezone,omitempty"`
}

// QuoteResponse represents the response from Twelve Data quote API
type QuoteResponse struct {
	Symbol           string `json:"symbol"`
	Name             string `json:"name"`
	Exchange         string `json:"exchange"`
	Open             string `json:"open"`
	High             string `json:"high"`
	Low              string `json:"low"`
	Close            string `json:"close"`
	PreviousClose    string `json:"previous_close"`
	Volume           string `json:"volume,omitempty"`
	Change           string `json:"change"`
	PercentChange    string `json:"percent_change"`
	AverageVolume    string `json:"average_volume,omitempty"`
	FiftyTwoWeekHigh string `json:"fifty_two_week_high,omitempty"`
	FiftyTwoWeekLow  string `json:"fifty_two_week_low,omitempty"`
	Datetime         string `json:"datetime"`
	Status           string `json:"status,omitempty"`
	Code             int    `json:"code,omitempty"`
	Message          string `json:"message,omitempty"`
}

// Client is the Twelve Data API client
type Client struct {
	apiKey string
	client *http.Client
}

// NewClient creates a new Twelve Data client from config
func NewClient() *Client {
	return &Client{
		apiKey: config.Get().TwelveDataKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithKey creates a new Twelve Data client with provided key
func NewClientWithKey(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetTimeSeries fetches historical bars for a symbol
// interval: 1min, 5min, 15min, 30min, 45min, 1h, 2h, 4h, 1day, 1week, 1month
func (c *Client) GetTimeSeries(ctx context.Context, symbol string, interval string, limit int) (*TimeSeriesResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("twelve data API key not configured")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/time_series", BaseURL)
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	params.Set("outputsize", fmt.Sprintf("%d", limit))
	params.Set("apikey", c.apiKey)

	fullURL := endpoint + "?" + params.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

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

	// Parse response
	var result TimeSeriesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if result.Status == "error" {
		return nil, fmt.Errorf("twelve data API error: %s", result.Message)
	}

	return &result, nil
}

// GetQuote fetches real-time quote for a symbol
func (c *Client) GetQuote(ctx context.Context, symbol string) (*QuoteResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("twelve data API key not configured")
	}

	// Build URL
	endpoint := fmt.Sprintf("%s/quote", BaseURL)
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("apikey", c.apiKey)

	fullURL := endpoint + "?" + params.Encode()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

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

	// Parse response
	var result QuoteResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if result.Status == "error" {
		return nil, fmt.Errorf("twelve data API error: %s", result.Message)
	}

	return &result, nil
}

// MapTimeframe maps common timeframe strings to Twelve Data format
func MapTimeframe(interval string) string {
	switch interval {
	case "1m":
		return "1min"
	case "3m":
		return "5min" // Twelve Data doesn't have 3m, use 5m
	case "5m":
		return "5min"
	case "10m":
		return "15min" // Twelve Data doesn't have 10m, use 15m
	case "15m":
		return "15min"
	case "30m":
		return "30min"
	case "1h":
		return "1h"
	case "2h":
		return "2h"
	case "4h":
		return "4h"
	case "6h":
		return "4h" // Twelve Data doesn't have 6h, use 4h
	case "8h":
		return "4h" // Twelve Data doesn't have 8h, use 4h
	case "12h":
		return "4h" // Twelve Data doesn't have 12h, use 4h
	case "1d":
		return "1day"
	case "3d":
		return "1day" // Twelve Data doesn't have 3d, use 1d
	case "1w":
		return "1week"
	case "1M":
		return "1month"
	default:
		return "5min" // Default to 5 minutes
	}
}

// ParseBar converts a Twelve Data bar to numeric values
func ParseBar(bar Bar) (open, high, low, close, volume float64, timestamp int64, err error) {
	open, err = strconv.ParseFloat(bar.Open, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("failed to parse open: %w", err)
	}
	high, err = strconv.ParseFloat(bar.High, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("failed to parse high: %w", err)
	}
	low, err = strconv.ParseFloat(bar.Low, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("failed to parse low: %w", err)
	}
	close, err = strconv.ParseFloat(bar.Close, 64)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("failed to parse close: %w", err)
	}

	// Volume might be empty for forex
	if bar.Volume != "" {
		volume, _ = strconv.ParseFloat(bar.Volume, 64)
	}

	// Parse datetime - format is "2024-01-15 09:30:00" or "2024-01-15"
	var t time.Time
	if len(bar.Datetime) > 10 {
		t, err = time.Parse("2006-01-02 15:04:05", bar.Datetime)
	} else {
		t, err = time.Parse("2006-01-02", bar.Datetime)
	}
	if err != nil {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("failed to parse datetime: %w", err)
	}
	timestamp = t.UnixMilli()

	return open, high, low, close, volume, timestamp, nil
}
