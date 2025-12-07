package pool

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// defaultMainstreamCoins default mainstream coin pool (read from config file)
var defaultMainstreamCoins = []string{
	"BTCUSDT",
	"ETHUSDT",
	"SOLUSDT",
	"BNBUSDT",
	"XRPUSDT",
	"DOGEUSDT",
	"ADAUSDT",
	"HYPEUSDT",
}

// CoinPoolConfig coin pool configuration
type CoinPoolConfig struct {
	APIURL          string
	Timeout         time.Duration
	CacheDir        string
	UseDefaultCoins bool // Whether to use default mainstream coins
}

var coinPoolConfig = CoinPoolConfig{
	APIURL:          "",
	Timeout:         30 * time.Second, // Increased to 30 seconds
	CacheDir:        "coin_pool_cache",
	UseDefaultCoins: false, // Default is not to use
}

// CoinPoolCache coin pool cache
type CoinPoolCache struct {
	Coins      []CoinInfo `json:"coins"`
	FetchedAt  time.Time  `json:"fetched_at"`
	SourceType string     `json:"source_type"` // "api" or "cache"
}

// CoinInfo coin information
type CoinInfo struct {
	Pair            string  `json:"pair"`             // Trading pair symbol (e.g.: BTCUSDT)
	Score           float64 `json:"score"`            // Current score
	StartTime       int64   `json:"start_time"`       // Start time (Unix timestamp)
	StartPrice      float64 `json:"start_price"`      // Start price
	LastScore       float64 `json:"last_score"`       // Latest score
	MaxScore        float64 `json:"max_score"`        // Highest score
	MaxPrice        float64 `json:"max_price"`        // Highest price
	IncreasePercent float64 `json:"increase_percent"` // Increase percentage
	IsAvailable     bool    `json:"-"`                // Whether tradable (internal use)
}

// CoinPoolAPIResponse raw data structure returned by API
type CoinPoolAPIResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Coins []CoinInfo `json:"coins"`
		Count int        `json:"count"`
	} `json:"data"`
}

// SetCoinPoolAPI sets coin pool API
func SetCoinPoolAPI(apiURL string) {
	coinPoolConfig.APIURL = apiURL
}

// SetOITopAPI sets OI Top API
func SetOITopAPI(apiURL string) {
	oiTopConfig.APIURL = apiURL
}

// SetUseDefaultCoins sets whether to use default mainstream coins
func SetUseDefaultCoins(useDefault bool) {
	coinPoolConfig.UseDefaultCoins = useDefault
}

// SetDefaultCoins sets default mainstream coin list
func SetDefaultCoins(coins []string) {
	if len(coins) > 0 {
		defaultMainstreamCoins = coins
		log.Printf("✓ Default coin pool set (%d coins): %v", len(coins), coins)
	}
}

// GetCoinPool retrieves coin pool list (with retry and cache mechanism)
func GetCoinPool() ([]CoinInfo, error) {
	// First check if default coin list is enabled
	if coinPoolConfig.UseDefaultCoins {
		log.Printf("✓ Default mainstream coin list enabled")
		return convertSymbolsToCoins(defaultMainstreamCoins), nil
	}

	// Check if API URL is configured
	if strings.TrimSpace(coinPoolConfig.APIURL) == "" {
		log.Printf("⚠️  Coin pool API URL not configured, using default mainstream coin list")
		return convertSymbolsToCoins(defaultMainstreamCoins), nil
	}

	maxRetries := 3
	var lastErr error

	// Try to fetch from API
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("⚠️  Retry attempt %d of %d to fetch coin pool...", attempt, maxRetries)
			time.Sleep(2 * time.Second) // Wait 2 seconds before retry
		}

		coins, err := fetchCoinPool()
		if err == nil {
			if attempt > 1 {
				log.Printf("✓ Retry attempt %d succeeded", attempt)
			}
			// Save to cache after successful fetch
			if err := saveCoinPoolCache(coins); err != nil {
				log.Printf("⚠️  Failed to save coin pool cache: %v", err)
			}
			return coins, nil
		}

		lastErr = err
		log.Printf("❌ Request attempt %d failed: %v", attempt, err)
	}

	// API fetch failed, try to use cache
	log.Printf("⚠️  All API requests failed, trying to use historical cache data...")
	cachedCoins, err := loadCoinPoolCache()
	if err == nil {
		log.Printf("✓ Using historical cache data (%d coins)", len(cachedCoins))
		return cachedCoins, nil
	}

	// Cache also failed, use default mainstream coins
	log.Printf("⚠️  Unable to load cache data (last error: %v), using default mainstream coin list", lastErr)
	return convertSymbolsToCoins(defaultMainstreamCoins), nil
}

// fetchCoinPool actually executes coin pool request
func fetchCoinPool() ([]CoinInfo, error) {
	log.Printf("🔄 Requesting AI500 coin pool...")

	client := &http.Client{
		Timeout: coinPoolConfig.Timeout,
	}

	resp, err := client.Get(coinPoolConfig.APIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to request coin pool API: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse API response
	var response CoinPoolAPIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API returned failure status")
	}

	if len(response.Data.Coins) == 0 {
		return nil, fmt.Errorf("coin list is empty")
	}

	// Set IsAvailable flag
	coins := response.Data.Coins
	for i := range coins {
		coins[i].IsAvailable = true
	}

	log.Printf("✓ Successfully fetched %d coins", len(coins))
	return coins, nil
}

// saveCoinPoolCache saves coin pool to cache file
func saveCoinPoolCache(coins []CoinInfo) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(coinPoolConfig.CacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := CoinPoolCache{
		Coins:      coins,
		FetchedAt:  time.Now(),
		SourceType: "api",
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize cache data: %w", err)
	}

	cachePath := filepath.Join(coinPoolConfig.CacheDir, "latest.json")
	if err := ioutil.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	log.Printf("💾 Coin pool cache saved (%d coins)", len(coins))
	return nil
}

// loadCoinPoolCache loads coin pool from cache file
func loadCoinPoolCache() ([]CoinInfo, error) {
	cachePath := filepath.Join(coinPoolConfig.CacheDir, "latest.json")

	// Check if file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cache file does not exist")
	}

	data, err := ioutil.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache CoinPoolCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse cache data: %w", err)
	}

	// Check cache age
	cacheAge := time.Since(cache.FetchedAt)
	if cacheAge > 24*time.Hour {
		log.Printf("⚠️  Cache data is old (%.1f hours ago), but still usable", cacheAge.Hours())
	} else {
		log.Printf("📂 Cache data timestamp: %s (%.1f minutes ago)",
			cache.FetchedAt.Format("2006-01-02 15:04:05"),
			cacheAge.Minutes())
	}

	return cache.Coins, nil
}

// GetAvailableCoins retrieves available coin list (filters out unavailable ones)
func GetAvailableCoins() ([]string, error) {
	coins, err := GetCoinPool()
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, coin := range coins {
		if coin.IsAvailable {
			// Ensure symbol format is correct (convert to uppercase USDT pair)
			symbol := normalizeSymbol(coin.Pair)
			symbols = append(symbols, symbol)
		}
	}

	if len(symbols) == 0 {
		return nil, fmt.Errorf("no available coins")
	}

	return symbols, nil
}

// GetTopRatedCoins retrieves top N coins by score (sorted by score descending)
func GetTopRatedCoins(limit int) ([]string, error) {
	coins, err := GetCoinPool()
	if err != nil {
		return nil, err
	}

	// Filter available coins
	var availableCoins []CoinInfo
	for _, coin := range coins {
		if coin.IsAvailable {
			availableCoins = append(availableCoins, coin)
		}
	}

	if len(availableCoins) == 0 {
		return nil, fmt.Errorf("no available coins")
	}

	// Sort by Score descending (bubble sort)
	for i := 0; i < len(availableCoins); i++ {
		for j := i + 1; j < len(availableCoins); j++ {
			if availableCoins[i].Score < availableCoins[j].Score {
				availableCoins[i], availableCoins[j] = availableCoins[j], availableCoins[i]
			}
		}
	}

	// Take top N
	maxCount := limit
	if len(availableCoins) < maxCount {
		maxCount = len(availableCoins)
	}

	var symbols []string
	for i := 0; i < maxCount; i++ {
		symbol := normalizeSymbol(availableCoins[i].Pair)
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// normalizeSymbol normalizes coin symbol
func normalizeSymbol(symbol string) string {
	// Remove spaces
	symbol = trimSpaces(symbol)

	// Convert to uppercase
	symbol = toUpper(symbol)

	// Ensure ends with USDT
	if !endsWith(symbol, "USDT") {
		symbol = symbol + "USDT"
	}

	return symbol
}

// Helper functions
func trimSpaces(s string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			result += string(s[i])
		}
	}
	return result
}

func toUpper(s string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		result += string(c)
	}
	return result
}

func endsWith(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}

// convertSymbolsToCoins converts symbol list to CoinInfo list
func convertSymbolsToCoins(symbols []string) []CoinInfo {
	coins := make([]CoinInfo, 0, len(symbols))
	for _, symbol := range symbols {
		coins = append(coins, CoinInfo{
			Pair:        symbol,
			Score:       0,
			IsAvailable: true,
		})
	}
	return coins
}

// ========== OI Top (Open Interest Growth Top 20) Data ==========

// OIPosition open interest data
type OIPosition struct {
	Symbol            string  `json:"symbol"`
	Rank              int     `json:"rank"`
	CurrentOI         float64 `json:"current_oi"`          // Current open interest
	OIDelta           float64 `json:"oi_delta"`            // Open interest change
	OIDeltaPercent    float64 `json:"oi_delta_percent"`    // Open interest change percentage
	OIDeltaValue      float64 `json:"oi_delta_value"`      // Open interest change value
	PriceDeltaPercent float64 `json:"price_delta_percent"` // Price change percentage
	NetLong           float64 `json:"net_long"`            // Net long position
	NetShort          float64 `json:"net_short"`           // Net short position
}

// OITopAPIResponse data structure returned by OI Top API
type OITopAPIResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Positions []OIPosition `json:"positions"`
		Count     int          `json:"count"`
		Exchange  string       `json:"exchange"`
		TimeRange string       `json:"time_range"`
	} `json:"data"`
}

// OITopCache OI Top cache
type OITopCache struct {
	Positions  []OIPosition `json:"positions"`
	FetchedAt  time.Time    `json:"fetched_at"`
	SourceType string       `json:"source_type"`
}

var oiTopConfig = struct {
	APIURL   string
	Timeout  time.Duration
	CacheDir string
}{
	APIURL:   "",
	Timeout:  30 * time.Second,
	CacheDir: "coin_pool_cache",
}

// GetOITopPositions retrieves OI Top 20 data (with retry and cache)
func GetOITopPositions() ([]OIPosition, error) {
	// Check if API URL is configured
	if strings.TrimSpace(oiTopConfig.APIURL) == "" {
		log.Printf("⚠️  OI Top API URL not configured, skipping OI Top data fetch")
		return []OIPosition{}, nil // Return empty list, not an error
	}

	maxRetries := 3
	var lastErr error

	// Try to fetch from API
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("⚠️  Retry attempt %d of %d to fetch OI Top data...", attempt, maxRetries)
			time.Sleep(2 * time.Second)
		}

		positions, err := fetchOITop()
		if err == nil {
			if attempt > 1 {
				log.Printf("✓ Retry attempt %d succeeded", attempt)
			}
			// Save to cache after successful fetch
			if err := saveOITopCache(positions); err != nil {
				log.Printf("⚠️  Failed to save OI Top cache: %v", err)
			}
			return positions, nil
		}

		lastErr = err
		log.Printf("❌ OI Top request attempt %d failed: %v", attempt, err)
	}

	// API fetch failed, try to use cache
	log.Printf("⚠️  All OI Top API requests failed, trying to use historical cache data...")
	cachedPositions, err := loadOITopCache()
	if err == nil {
		log.Printf("✓ Using historical OI Top cache data (%d coins)", len(cachedPositions))
		return cachedPositions, nil
	}

	// Cache also failed, return empty list (OI Top is optional)
	log.Printf("⚠️  Unable to load OI Top cache data (last error: %v), skipping OI Top data", lastErr)
	return []OIPosition{}, nil
}

// fetchOITop actually executes OI Top request
func fetchOITop() ([]OIPosition, error) {
	log.Printf("🔄 Requesting OI Top data...")

	client := &http.Client{
		Timeout: oiTopConfig.Timeout,
	}

	resp, err := client.Get(oiTopConfig.APIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to request OI Top API: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OI Top response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OI Top API returned error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse API response
	var response OITopAPIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("OI Top JSON parsing failed: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("OI Top API returned failure status")
	}

	if len(response.Data.Positions) == 0 {
		return nil, fmt.Errorf("OI Top position list is empty")
	}

	log.Printf("✓ Successfully fetched %d OI Top coins (time range: %s)",
		len(response.Data.Positions), response.Data.TimeRange)
	return response.Data.Positions, nil
}

// saveOITopCache saves OI Top data to cache
func saveOITopCache(positions []OIPosition) error {
	if err := os.MkdirAll(oiTopConfig.CacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := OITopCache{
		Positions:  positions,
		FetchedAt:  time.Now(),
		SourceType: "api",
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize OI Top cache data: %w", err)
	}

	cachePath := filepath.Join(oiTopConfig.CacheDir, "oi_top_latest.json")
	if err := ioutil.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write OI Top cache file: %w", err)
	}

	log.Printf("💾 OI Top cache saved (%d coins)", len(positions))
	return nil
}

// loadOITopCache loads OI Top data from cache
func loadOITopCache() ([]OIPosition, error) {
	cachePath := filepath.Join(oiTopConfig.CacheDir, "oi_top_latest.json")

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("OI Top cache file does not exist")
	}

	data, err := ioutil.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OI Top cache file: %w", err)
	}

	var cache OITopCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse OI Top cache data: %w", err)
	}

	cacheAge := time.Since(cache.FetchedAt)
	if cacheAge > 24*time.Hour {
		log.Printf("⚠️  OI Top cache data is old (%.1f hours ago), but still usable", cacheAge.Hours())
	} else {
		log.Printf("📂 OI Top cache data timestamp: %s (%.1f minutes ago)",
			cache.FetchedAt.Format("2006-01-02 15:04:05"),
			cacheAge.Minutes())
	}

	return cache.Positions, nil
}

// GetOITopSymbols retrieves OI Top coin symbol list
func GetOITopSymbols() ([]string, error) {
	positions, err := GetOITopPositions()
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, pos := range positions {
		symbol := normalizeSymbol(pos.Symbol)
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// MergedCoinPool merged coin pool (AI500 + OI Top)
type MergedCoinPool struct {
	AI500Coins    []CoinInfo          // AI500 score coins
	OITopCoins    []OIPosition        // Open interest growth Top 20
	AllSymbols    []string            // All unique coin symbols
	SymbolSources map[string][]string // Source of each coin ("ai500"/"oi_top")
}

// GetMergedCoinPool retrieves merged coin pool (AI500 + OI Top, deduplicated)
func GetMergedCoinPool(ai500Limit int) (*MergedCoinPool, error) {
	// 1. Get AI500 data
	ai500TopSymbols, err := GetTopRatedCoins(ai500Limit)
	if err != nil {
		log.Printf("⚠️  Failed to get AI500 data: %v", err)
		ai500TopSymbols = []string{} // Use empty list on failure
	}

	// 2. Get OI Top data
	oiTopSymbols, err := GetOITopSymbols()
	if err != nil {
		log.Printf("⚠️  Failed to get OI Top data: %v", err)
		oiTopSymbols = []string{} // Use empty list on failure
	}

	// 3. Merge and deduplicate
	symbolSet := make(map[string]bool)
	symbolSources := make(map[string][]string)

	// Add AI500 coins
	for _, symbol := range ai500TopSymbols {
		symbolSet[symbol] = true
		symbolSources[symbol] = append(symbolSources[symbol], "ai500")
	}

	// Add OI Top coins
	for _, symbol := range oiTopSymbols {
		if !symbolSet[symbol] {
			symbolSet[symbol] = true
		}
		symbolSources[symbol] = append(symbolSources[symbol], "oi_top")
	}

	// Convert to array
	var allSymbols []string
	for symbol := range symbolSet {
		allSymbols = append(allSymbols, symbol)
	}

	// Get complete data
	ai500Coins, _ := GetCoinPool()
	oiTopPositions, _ := GetOITopPositions()

	merged := &MergedCoinPool{
		AI500Coins:    ai500Coins,
		OITopCoins:    oiTopPositions,
		AllSymbols:    allSymbols,
		SymbolSources: symbolSources,
	}

	log.Printf("📊 Coin pool merge complete: AI500=%d, OI_Top=%d, Total(deduplicated)=%d",
		len(ai500TopSymbols), len(oiTopSymbols), len(allSymbols))

	return merged, nil
}
