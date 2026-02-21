package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nofx/config"
	"strings"
	"sync"
	"time"
)

const (
	DataAPIURL    = "https://data.alpaca.markets/v2"
	CryptoDataURL = "https://data.alpaca.markets/v1beta3"
	CryptoLoc     = "us"
)

var (
	cryptoAssetsCache struct {
		sync.RWMutex
		Assets    []CryptoAsset
		ExpiresAt time.Time
	}
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

// CryptoAsset represents a crypto asset available on Alpaca
type CryptoAsset struct {
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Tradable     bool   `json:"tradable"`
	Shortable    bool   `json:"shortable"`
	EasyToBorrow bool   `json:"easy_to_borrow"`
}

// CryptoAssetsResponse represents the response from Alpaca crypto assets API
type CryptoAssetsResponse struct {
	Assets []CryptoAsset `json:"assets"`
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
	return MapTimeframeForAsset(interval, "crypto")
}

// MapTimeframeForAsset maps timeframe strings to Alpaca format for specific asset type
func MapTimeframeForAsset(interval string, assetType string) string {
	// Crypto uses different timeframe format
	if assetType == "crypto" {
		switch interval {
		case "1m":
			return "1Min"
		case "3m":
			return "1Min"
		case "5m":
			return "5Min"
		case "10m":
			return "15Min"
		case "15m":
			return "15Min"
		case "30m":
			return "30Min"
		case "1h":
			return "1Hour"
		case "2h":
			return "1Hour"
		case "4h":
			return "4Hour"
		case "6h":
			return "4Hour"
		case "8h":
			return "4Hour"
		case "12h":
			return "4Hour"
		case "1d":
			return "1Day"
		case "1w":
			return "1Week"
		default:
			return "5Min"
		}
	}

	// Stock timeframe mapping (original)
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

// GetCryptoBars fetches historical bars for a crypto symbol
// symbol: e.g., "BTC/USD", "ETH/USD"
// timeframe: 1Min, 5Min, 15Min, 30Min, 1Hour, 4Hour, 1Day
func (c *Client) GetCryptoBars(ctx context.Context, symbol string, timeframe string, limit int) ([]Bar, error) {
	if c.apiKey == "" || c.secretKey == "" {
		return nil, fmt.Errorf("alpaca API keys not configured")
	}

	// Build URL
	encodedSymbol := url.PathEscape(symbol)
	endpoint := fmt.Sprintf("%s/crypto/%s/%s/bars", CryptoDataURL, CryptoLoc, encodedSymbol)
	params := url.Values{}
	params.Set("timeframe", timeframe)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("adjustment", "raw")

	// Set time range: last 30 days for intraday, last 2 years for daily
	now := time.Now()
	var start time.Time
	switch timeframe {
	case "1Day", "1Week":
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
		return nil, fmt.Errorf("alpaca crypto API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result BarsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Bars, nil
}

// GetCryptoAssets fetches available crypto assets from Alpaca
func (c *Client) GetCryptoAssets(ctx context.Context) ([]CryptoAsset, error) {
	if c.apiKey == "" || c.secretKey == "" {
		return nil, fmt.Errorf("alpaca API keys not configured")
	}

	endpoint := fmt.Sprintf("%s/crypto", CryptoDataURL)
	fullURL := endpoint

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("APCA-API-KEY-ID", c.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.secretKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca crypto assets API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result CryptoAssetsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Assets, nil
}

// IsCryptoSymbolSupported checks if a symbol is available on Alpaca
func (c *Client) IsCryptoSymbolSupported(ctx context.Context, symbol string) (bool, error) {
	cryptoAssetsCache.RLock()
	if len(cryptoAssetsCache.Assets) > 0 && time.Now().Before(cryptoAssetsCache.ExpiresAt) {
		for _, asset := range cryptoAssetsCache.Assets {
			if asset.Symbol == symbol && asset.Tradable {
				cryptoAssetsCache.RUnlock()
				return true, nil
			}
		}
		cryptoAssetsCache.RUnlock()
	} else {
		cryptoAssetsCache.RUnlock()
	}

	assets, err := c.GetCryptoAssets(ctx)
	if err != nil {
		return false, err
	}

	cryptoAssetsCache.Lock()
	cryptoAssetsCache.Assets = assets
	cryptoAssetsCache.ExpiresAt = time.Now().Add(5 * time.Minute)
	cryptoAssetsCache.Unlock()

	for _, asset := range assets {
		if asset.Symbol == symbol && asset.Tradable {
			return true, nil
		}
	}

	return false, nil
}

// ConvertSymbolToAlpacaFormat converts standard crypto symbol to Alpaca format
// e.g., "BTCUSDT" -> "BTC/USD", "ETHUSDT" -> "ETH/USD"
func ConvertSymbolToAlpacaFormat(symbol string) string {
	// Common USDT pairs
	usdtPairs := map[string]string{
		"BTCUSDT":   "BTC/USD",
		"ETHUSDT":   "ETH/USD",
		"BNBUSDT":   "BNB/USD",
		"SOLUSDT":   "SOL/USD",
		"XRPUSDT":   "XRP/USD",
		"ADAUSDT":   "ADA/USD",
		"DOGEUSDT":  "DOGE/USD",
		"AVAXUSDT":  "AVAX/USD",
		"DOTUSDT":   "DOT/USD",
		"MATICUSDT": "MATIC/USD",
		"LINKUSDT":  "LINK/USD",
		"UNIUSDT":   "UNI/USD",
		"ATOMUSDT":  "ATOM/USD",
		"LTCUSDT":   "LTC/USD",
		"BCHUSDT":   "BCH/USD",
	}

	upperSymbol := strings.ToUpper(symbol)
	if val, ok := usdtPairs[upperSymbol]; ok {
		return val
	}

	// If already in correct format, return as is
	if strings.Contains(symbol, "/") {
		return symbol
	}

	// Default: assume last 4 chars are USDT and replace with /USD
	if len(symbol) > 4 {
		return symbol[:len(symbol)-4] + "/" + symbol[len(symbol)-3:]
	}

	return symbol
}
