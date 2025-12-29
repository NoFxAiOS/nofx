package hyperliquid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	MainnetAPIURL = "https://api.hyperliquid.xyz/info"
	TestnetAPIURL = "https://api.hyperliquid-testnet.xyz/info"
)

// Candle represents a single OHLCV candle from Hyperliquid
type Candle struct {
	OpenTime   int64   `json:"t"`  // Open time in milliseconds
	CloseTime  int64   `json:"T"`  // Close time in milliseconds
	Symbol     string  `json:"s"`  // Coin symbol
	Interval   string  `json:"i"`  // Interval
	Open       string  `json:"o"`  // Open price
	High       string  `json:"h"`  // High price
	Low        string  `json:"l"`  // Low price
	Close      string  `json:"c"`  // Close price
	Volume     string  `json:"v"`  // Volume in base unit
	TradeCount int     `json:"n"`  // Number of trades
}

// CandleRequest represents the request for candleSnapshot
type CandleRequest struct {
	Type string            `json:"type"`
	Req  CandleRequestBody `json:"req"`
}

// CandleRequestBody represents the body of candleSnapshot request
type CandleRequestBody struct {
	Coin      string `json:"coin"`
	Interval  string `json:"interval"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

// Client is the Hyperliquid API client
type Client struct {
	apiURL string
	client *http.Client
}

// NewClient creates a new Hyperliquid client for mainnet
func NewClient() *Client {
	return &Client{
		apiURL: MainnetAPIURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewTestnetClient creates a new Hyperliquid client for testnet
func NewTestnetClient() *Client {
	return &Client{
		apiURL: TestnetAPIURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetCandles fetches historical candlestick data for a symbol
// coin: symbol name (e.g., "BTC", "TSLA", "AAPL", "xyz:TSLA")
// interval: "1m", "5m", "15m", "1h", "4h", "1d"
// limit: number of candles to fetch (max 5000)
func (c *Client) GetCandles(ctx context.Context, coin string, interval string, limit int) ([]Candle, error) {
	// Format coin name for API (stock perps need xyz: prefix)
	coin = FormatCoinForAPI(coin)

	// Calculate time range based on interval and limit
	now := time.Now()
	endTime := now.UnixMilli()

	// Calculate start time based on interval
	intervalDuration := getIntervalDuration(interval)
	startTime := now.Add(-intervalDuration * time.Duration(limit)).UnixMilli()

	// Build request
	reqBody := CandleRequest{
		Type: "candleSnapshot",
		Req: CandleRequestBody{
			Coin:      coin,
			Interval:  interval,
			StartTime: startTime,
			EndTime:   endTime,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("hyperliquid API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var candles []Candle
	if err := json.Unmarshal(body, &candles); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	return candles, nil
}

// GetAllMids fetches current mid prices for all assets (default perp dex)
func (c *Client) GetAllMids(ctx context.Context) (map[string]string, error) {
	return c.GetAllMidsWithDex(ctx, "")
}

// GetAllMidsXYZ fetches current mid prices for xyz dex (stocks, forex, commodities)
func (c *Client) GetAllMidsXYZ(ctx context.Context) (map[string]string, error) {
	return c.GetAllMidsWithDex(ctx, XYZDex)
}

// GetAllMidsWithDex fetches current mid prices for a specific dex
func (c *Client) GetAllMidsWithDex(ctx context.Context, dex string) (map[string]string, error) {
	reqBody := map[string]string{"type": "allMids"}
	if dex != "" {
		reqBody["dex"] = dex
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("hyperliquid API error (status %d): %s", resp.StatusCode, string(body))
	}

	var mids map[string]string
	if err := json.Unmarshal(body, &mids); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return mids, nil
}

// GetMeta fetches metadata for all perpetual assets
func (c *Client) GetMeta(ctx context.Context) (*Meta, error) {
	reqBody := map[string]string{"type": "meta"}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
		return nil, fmt.Errorf("hyperliquid API error (status %d): %s", resp.StatusCode, string(body))
	}

	var meta Meta
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &meta, nil
}

// Meta represents the metadata response
type Meta struct {
	Universe []AssetInfo `json:"universe"`
}

// AssetInfo represents information about a single asset
type AssetInfo struct {
	Name       string `json:"name"`
	SzDecimals int    `json:"szDecimals"`
	MaxLeverage int   `json:"maxLeverage"`
}

// NormalizeCoin normalizes coin name for Hyperliquid API
// Examples:
//   - "BTCUSDT" -> "BTC"
//   - "TSLA-USDC" -> "TSLA"
//   - "xyz:TSLA" -> "TSLA"
//   - "BTC" -> "BTC"
func NormalizeCoin(symbol string) string {
	return NormalizeCoinBase(symbol)
}

// MapTimeframe maps common timeframe strings to Hyperliquid format
func MapTimeframe(interval string) string {
	switch interval {
	case "1m":
		return "1m"
	case "3m":
		return "5m" // Hyperliquid doesn't have 3m, use 5m
	case "5m":
		return "5m"
	case "15m":
		return "15m"
	case "30m":
		return "30m"
	case "1h":
		return "1h"
	case "2h":
		return "1h" // Hyperliquid doesn't have 2h, use 1h
	case "4h":
		return "4h"
	case "6h":
		return "4h" // Hyperliquid doesn't have 6h, use 4h
	case "8h":
		return "8h"
	case "12h":
		return "12h"
	case "1d":
		return "1d"
	case "3d":
		return "1d" // Hyperliquid doesn't have 3d, use 1d
	case "1w":
		return "1w"
	case "1M":
		return "1M"
	default:
		return "5m" // Default to 5 minutes
	}
}

// getIntervalDuration returns the duration for a given interval
func getIntervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "8h":
		return 8 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	case "1M":
		return 30 * 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

// XYZ Dex name for stock perps, forex, and commodities
const XYZDex = "xyz"

// Stock perps symbols available on Hyperliquid xyz dex
// Use xyz:SYMBOL format when calling the API
var StockPerpsSymbols = []string{
	"TSLA",  // Tesla
	"AAPL",  // Apple
	"NVDA",  // Nvidia
	"MSFT",  // Microsoft
	"META",  // Meta
	"AMZN",  // Amazon
	"GOOGL", // Alphabet
	"AMD",   // AMD
	"COIN",  // Coinbase
	"NFLX",  // Netflix
	"PLTR",  // Palantir
	"HOOD",  // Robinhood
	"INTC",  // Intel
	"MSTR",  // MicroStrategy
	"TSM",   // TSMC
	"ORCL",  // Oracle
	"MU",    // Micron
	"RIVN",  // Rivian
	"COST",  // Costco
	"LLY",   // Eli Lilly
	"CRCL",  // Circle (new)
	"SKHX",  // Skyward (new)
	"SNDK",  // Sandisk (new)
}

// Forex and commodities on xyz dex
var XYZOtherSymbols = []string{
	"GOLD",   // Gold
	"SILVER", // Silver
	"EUR",    // EUR/USD
	"JPY",    // USD/JPY
	"XYZ100", // Index
}

// IsStockPerp checks if a symbol is a stock perpetual
func IsStockPerp(symbol string) bool {
	coin := NormalizeCoinBase(symbol)
	for _, s := range StockPerpsSymbols {
		if s == coin {
			return true
		}
	}
	return false
}

// IsXYZAsset checks if a symbol is on the xyz dex (stocks, forex, commodities)
func IsXYZAsset(symbol string) bool {
	coin := NormalizeCoinBase(symbol)
	// Check stock perps
	for _, s := range StockPerpsSymbols {
		if s == coin {
			return true
		}
	}
	// Check other xyz assets
	for _, s := range XYZOtherSymbols {
		if s == coin {
			return true
		}
	}
	return false
}

// NormalizeCoinBase removes common suffixes to get base symbol
func NormalizeCoinBase(symbol string) string {
	// Remove xyz: prefix if present
	if strings.HasPrefix(symbol, "xyz:") {
		return strings.TrimPrefix(symbol, "xyz:")
	}
	// Remove -USDC suffix
	if strings.HasSuffix(symbol, "-USDC") {
		return strings.TrimSuffix(symbol, "-USDC")
	}
	// Remove USDT suffix
	if strings.HasSuffix(symbol, "USDT") {
		return strings.TrimSuffix(symbol, "USDT")
	}
	// Remove USD suffix
	if strings.HasSuffix(symbol, "USD") {
		return strings.TrimSuffix(symbol, "USD")
	}
	return symbol
}

// FormatCoinForAPI formats the coin name for Hyperliquid API
// Stock perps need xyz:SYMBOL format, crypto uses plain symbol
func FormatCoinForAPI(symbol string) string {
	base := NormalizeCoinBase(symbol)
	if IsXYZAsset(base) {
		return "xyz:" + base
	}
	return base
}
