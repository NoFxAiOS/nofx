package indodax

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Indodax API endpoints
const (
	indodaxBaseURL    = "https://indodax.com"
	indodaxPublicAPI  = "/api"
	indodaxPrivateAPI = "/tapi"
)

// IndodaxTrader implements types.Trader interface for Indodax Spot Exchange
// Indodax is Indonesia's largest crypto exchange, supporting IDR (Indonesian Rupiah) pairs.
// Since Indodax is spot-only, futures-specific methods (OpenShort, CloseShort, leverage, etc.)
// are gracefully stubbed.
type IndodaxTrader struct {
	apiKey    string
	secretKey string

	httpClient *http.Client
	nonce      int64
	nonceMutex sync.Mutex

	// Cache for pair info
	pairCache      map[string]*IndodaxPair
	pairCacheMutex sync.RWMutex
	pairCacheTime  time.Time

	// Cache for balance
	cachedBalance     map[string]interface{}
	cachedPositions   []map[string]interface{}
	balanceCacheTime  time.Time
	positionCacheTime time.Time
	cacheDuration     time.Duration
	cacheMutex        sync.RWMutex
}

// IndodaxPair represents a trading pair on Indodax
type IndodaxPair struct {
	ID                     string  `json:"id"`
	Symbol                 string  `json:"symbol"`
	BaseCurrency           string  `json:"base_currency"`
	TradedCurrency         string  `json:"traded_currency"`
	TradedCurrencyUnit     string  `json:"traded_currency_unit"`
	Description            string  `json:"description"`
	TickerID               string  `json:"ticker_id"`
	VolumePrecision        int     `json:"volume_precision"`
	PricePrecision         float64 `json:"price_precision"`
	PriceRound             int     `json:"price_round"`
	Pricescale             float64 `json:"pricescale"`
	TradeMinBaseCurrency   float64 `json:"trade_min_base_currency"`
	TradeMinTradedCurrency float64 `json:"trade_min_traded_currency"`
}

// IndodaxResponse represents the standard Indodax private API response
type IndodaxResponse struct {
	Success   int             `json:"success"`
	Return    json.RawMessage `json:"return,omitempty"`
	Error     string          `json:"error,omitempty"`
	ErrorCode string          `json:"error_code,omitempty"`
}

// IndodaxTicker represents ticker data
type IndodaxTicker struct {
	High       string `json:"high"`
	Low        string `json:"low"`
	Last       string `json:"last"`
	Buy        string `json:"buy"`
	Sell       string `json:"sell"`
	ServerTime int64  `json:"server_time"`
}

// IndodaxTickerResponse wraps ticker response
type IndodaxTickerResponse struct {
	Ticker IndodaxTicker `json:"ticker"`
}

// NewIndodaxTrader creates a new Indodax trader instance
func NewIndodaxTrader(apiKey, secretKey string) *IndodaxTrader {
	return &IndodaxTrader{
		apiKey:        apiKey,
		secretKey:     secretKey,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		nonce:         time.Now().UnixMilli(),
		pairCache:     make(map[string]*IndodaxPair),
		cacheDuration: 15 * time.Second,
	}
}

// getNonce returns a unique incrementing nonce for each request
func (t *IndodaxTrader) getNonce() int64 {
	t.nonceMutex.Lock()
	defer t.nonceMutex.Unlock()
	t.nonce++
	return t.nonce
}

// sign generates HMAC-SHA512 signature for request body
func (t *IndodaxTrader) sign(body string) string {
	mac := hmac.New(sha512.New, []byte(t.secretKey))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// doPublicRequest makes a public API GET request
func (t *IndodaxTrader) doPublicRequest(path string) ([]byte, error) {
	reqURL := indodaxBaseURL + indodaxPublicAPI + path

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// doPrivateRequest makes a signed private API POST request
func (t *IndodaxTrader) doPrivateRequest(params url.Values) ([]byte, error) {
	reqURL := indodaxBaseURL + indodaxPrivateAPI

	// Add nonce
	params.Set("nonce", strconv.FormatInt(t.getNonce(), 10))

	body := params.Encode()
	signature := t.sign(body)

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Key", t.apiKey)
	req.Header.Set("Sign", signature)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded, please try again later")
	}

	// Parse response to check success
	var apiResp IndodaxResponse
	if err := json.Unmarshal(data, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(data))
	}

	if apiResp.Success != 1 {
		return nil, fmt.Errorf("API error: %s (code: %s)", apiResp.Error, apiResp.ErrorCode)
	}

	return apiResp.Return, nil
}

// convertSymbol converts standard symbol to Indodax format
// e.g. BTCIDR -> btc_idr, ETHIDR -> eth_idr
func (t *IndodaxTrader) convertSymbol(symbol string) string {
	s := strings.ToLower(symbol)

	// Already in Indodax format (contains underscore)
	if strings.Contains(s, "_") {
		return s
	}

	// Try to split by known base currencies
	for _, base := range []string{"idr", "btc", "usdt"} {
		if strings.HasSuffix(s, base) {
			traded := strings.TrimSuffix(s, base)
			if traded != "" {
				return traded + "_" + base
			}
		}
	}

	return s
}

// convertSymbolBack converts Indodax format back to standard
// e.g. btc_idr -> BTCIDR
func (t *IndodaxTrader) convertSymbolBack(indodaxSymbol string) string {
	return strings.ToUpper(strings.ReplaceAll(indodaxSymbol, "_", ""))
}

// getCoinFromSymbol extracts the traded currency from a symbol
// e.g. btc_idr -> btc, eth_idr -> eth
func (t *IndodaxTrader) getCoinFromSymbol(symbol string) string {
	pair := t.convertSymbol(symbol)
	parts := strings.Split(pair, "_")
	if len(parts) >= 1 {
		return parts[0]
	}
	return strings.ToLower(symbol)
}

// loadPairs loads trading pair information from the public API
func (t *IndodaxTrader) loadPairs() error {
	t.pairCacheMutex.RLock()
	if len(t.pairCache) > 0 && time.Since(t.pairCacheTime) < 5*time.Minute {
		t.pairCacheMutex.RUnlock()
		return nil
	}
	t.pairCacheMutex.RUnlock()

	data, err := t.doPublicRequest("/pairs")
	if err != nil {
		return fmt.Errorf("failed to load pairs: %w", err)
	}

	var pairs []IndodaxPair
	if err := json.Unmarshal(data, &pairs); err != nil {
		return fmt.Errorf("failed to parse pairs: %w", err)
	}

	t.pairCacheMutex.Lock()
	defer t.pairCacheMutex.Unlock()

	t.pairCache = make(map[string]*IndodaxPair)
	for i := range pairs {
		p := pairs[i]
		t.pairCache[p.TickerID] = &p
		// Also index by ID (e.g. "btcidr")
		t.pairCache[p.ID] = &p
	}
	t.pairCacheTime = time.Now()

	logger.Infof("[Indodax] Loaded %d trading pairs", len(pairs))
	return nil
}

// getPair gets pair info for a symbol
func (t *IndodaxTrader) getPair(symbol string) (*IndodaxPair, error) {
	if err := t.loadPairs(); err != nil {
		return nil, err
	}

	pairID := t.convertSymbol(symbol)

	t.pairCacheMutex.RLock()
	defer t.pairCacheMutex.RUnlock()

	if pair, ok := t.pairCache[pairID]; ok {
		return pair, nil
	}

	// Try without underscore
	noUnderscore := strings.ReplaceAll(pairID, "_", "")
	if pair, ok := t.pairCache[noUnderscore]; ok {
		return pair, nil
	}

	return nil, fmt.Errorf("pair not found: %s", symbol)
}

// clearCache clears cached data
func (t *IndodaxTrader) clearCache() {
	t.cacheMutex.Lock()
	defer t.cacheMutex.Unlock()
	t.cachedBalance = nil
	t.cachedPositions = nil
}

// parseFloat safely parses a float from interface{}
func parseFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case json.Number:
		f, _ := val.Float64()
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}
