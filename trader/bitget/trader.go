package bitget

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Bitget API endpoints (V2)
const (
	bitgetBaseURL          = "https://api.bitget.com"
	bitgetAccountPath      = "/api/v2/mix/account/accounts"
	bitgetPositionPath     = "/api/v2/mix/position/all-position"
	bitgetOrderPath        = "/api/v2/mix/order/place-order"
	bitgetLeveragePath     = "/api/v2/mix/account/set-leverage"
	bitgetTickerPath       = "/api/v2/mix/market/ticker"
	bitgetContractsPath    = "/api/v2/mix/market/contracts"
	bitgetCancelOrderPath  = "/api/v2/mix/order/cancel-order"
	bitgetPendingPath      = "/api/v2/mix/order/orders-pending"
	bitgetHistoryPath      = "/api/v2/mix/order/orders-history"
	bitgetMarginModePath   = "/api/v2/mix/account/set-margin-mode"
	bitgetPositionModePath = "/api/v2/mix/account/set-position-mode"
)

// BitgetTrader Bitget futures trader
type BitgetTrader struct {
	apiKey     string
	secretKey  string
	passphrase string

	// HTTP client
	httpClient *http.Client

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Positions cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Contract info cache
	contractsCache      map[string]*BitgetContract
	contractsCacheTime  time.Time
	contractsCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
}

// BitgetContract Bitget contract info
type BitgetContract struct {
	Symbol         string  // Symbol name
	BaseCoin       string  // Base coin
	QuoteCoin      string  // Quote coin
	MinTradeNum    float64 // Minimum trade amount
	MaxTradeNum    float64 // Maximum trade amount
	SizeMultiplier float64 // Contract size multiplier
	PricePlace     int     // Price decimal places
	VolumePlace    int     // Volume decimal places
}

// BitgetResponse Bitget API response
type BitgetResponse struct {
	Code        string          `json:"code"`
	Msg         string          `json:"msg"`
	Data        json.RawMessage `json:"data"`
	RequestTime int64           `json:"requestTime"`
}

// NewBitgetTrader creates a Bitget trader
func NewBitgetTrader(apiKey, secretKey, passphrase string) *BitgetTrader {
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: http.DefaultTransport,
	}

	trader := &BitgetTrader{
		apiKey:         apiKey,
		secretKey:      secretKey,
		passphrase:     passphrase,
		httpClient:     httpClient,
		cacheDuration:  15 * time.Second,
		contractsCache: make(map[string]*BitgetContract),
	}

	// Set one-way position mode (net mode)
	if err := trader.setPositionMode(); err != nil {
		logger.Infof("⚠️ Failed to set Bitget position mode: %v (ignore if already set)", err)
	}

	logger.Infof("🟢 [Bitget] Trader initialized")

	return trader
}

// setPositionMode sets one-way position mode
func (t *BitgetTrader) setPositionMode() error {
	body := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"posMode":     "one_way_mode",
	}

	_, err := t.doRequest("POST", bitgetPositionModePath, body)
	if err != nil {
		if strings.Contains(err.Error(), "same") || strings.Contains(err.Error(), "already") {
			return nil
		}
		return err
	}

	logger.Infof("  ✓ Bitget account switched to one-way position mode")
	return nil
}

// sign generates Bitget API signature
func (t *BitgetTrader) sign(timestamp, method, requestPath, body string) string {
	// Signature = BASE64(HMAC_SHA256(timestamp + method + requestPath + body, secretKey))
	preHash := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest executes HTTP request
func (t *BitgetTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error
	var queryString string

	if body != nil {
		if method == "GET" {
			// For GET requests, body is query parameters
			if params, ok := body.(map[string]interface{}); ok {
				var parts []string
				for k, v := range params {
					parts = append(parts, fmt.Sprintf("%s=%v", k, v))
				}
				queryString = strings.Join(parts, "&")
				if queryString != "" {
					path = path + "?" + queryString
				}
			}
		} else {
			bodyBytes, err = json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize request body: %w", err)
			}
		}
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	// Signature includes body for POST, nothing for GET (query is in path)
	signBody := ""
	if method != "GET" && bodyBytes != nil {
		signBody = string(bodyBytes)
	}
	signature := t.sign(timestamp, method, path, signBody)

	url := bitgetBaseURL + path
	req, err := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("ACCESS-KEY", t.apiKey)
	req.Header.Set("ACCESS-SIGN", signature)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("ACCESS-PASSPHRASE", t.passphrase)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("locale", "en-US")
	// Channel code only for order endpoints
	if strings.Contains(path, "/order/") {
		req.Header.Set("X-CHANNEL-API-CODE", "7fygt")
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var bitgetResp BitgetResponse
	if err := json.Unmarshal(respBody, &bitgetResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(respBody))
	}

	if bitgetResp.Code != "00000" {
		return nil, fmt.Errorf("Bitget API error: code=%s, msg=%s", bitgetResp.Code, bitgetResp.Msg)
	}

	return bitgetResp.Data, nil
}

// convertSymbol converts generic symbol to Bitget format
// e.g., BTCUSDT -> BTCUSDT
func (t *BitgetTrader) convertSymbol(symbol string) string {
	// Bitget uses same format as input, just ensure uppercase
	return strings.ToUpper(symbol)
}

// getContract gets contract info
func (t *BitgetTrader) getContract(symbol string) (*BitgetContract, error) {
	symbol = t.convertSymbol(symbol)

	// Check cache
	t.contractsCacheMutex.RLock()
	if contract, ok := t.contractsCache[symbol]; ok && time.Since(t.contractsCacheTime) < 5*time.Minute {
		t.contractsCacheMutex.RUnlock()
		return contract, nil
	}
	t.contractsCacheMutex.RUnlock()

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"symbol":      symbol,
	}

	data, err := t.doRequest("GET", bitgetContractsPath, params)
	if err != nil {
		return nil, err
	}

	var contracts []struct {
		Symbol         string `json:"symbol"`
		BaseCoin       string `json:"baseCoin"`
		QuoteCoin      string `json:"quoteCoin"`
		MinTradeNum    string `json:"minTradeNum"`
		MaxTradeNum    string `json:"maxTradeNum"`
		SizeMultiplier string `json:"sizeMultiplier"`
		PricePlace     string `json:"pricePlace"`
		VolumePlace    string `json:"volumePlace"`
	}

	if err := json.Unmarshal(data, &contracts); err != nil {
		return nil, err
	}

	// Find matching contract
	for _, c := range contracts {
		if c.Symbol == symbol {
			minTrade, _ := strconv.ParseFloat(c.MinTradeNum, 64)
			maxTrade, _ := strconv.ParseFloat(c.MaxTradeNum, 64)
			sizeMult, _ := strconv.ParseFloat(c.SizeMultiplier, 64)
			pricePlace, _ := strconv.Atoi(c.PricePlace)
			volumePlace, _ := strconv.Atoi(c.VolumePlace)

			contract := &BitgetContract{
				Symbol:         c.Symbol,
				BaseCoin:       c.BaseCoin,
				QuoteCoin:      c.QuoteCoin,
				MinTradeNum:    minTrade,
				MaxTradeNum:    maxTrade,
				SizeMultiplier: sizeMult,
				PricePlace:     pricePlace,
				VolumePlace:    volumePlace,
			}

			// Update cache
			t.contractsCacheMutex.Lock()
			t.contractsCache[symbol] = contract
			t.contractsCacheTime = time.Now()
			t.contractsCacheMutex.Unlock()

			return contract, nil
		}
	}

	return nil, fmt.Errorf("contract info not found: %s", symbol)
}

// FormatQuantity formats quantity
func (t *BitgetTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		return fmt.Sprintf("%.4f", quantity), nil
	}

	// Format according to volume precision
	format := fmt.Sprintf("%%.%df", contract.VolumePlace)
	return fmt.Sprintf(format, quantity), nil
}

// clearCache clears all caches
func (t *BitgetTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheMutex.Unlock()
}

// genBitgetClientOid generates unique client order ID
func genBitgetClientOid() string {
	timestamp := time.Now().UnixNano() % 10000000000000
	rand := time.Now().Nanosecond() % 100000
	return fmt.Sprintf("nofx%d%05d", timestamp, rand)
}
