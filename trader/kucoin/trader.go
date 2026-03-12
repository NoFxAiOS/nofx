package kucoin

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KuCoin Futures API endpoints
const (
	kucoinBaseURL          = "https://api-futures.kucoin.com"
	kucoinAccountPath      = "/api/v1/account-overview"
	kucoinPositionPath     = "/api/v1/positions"
	kucoinOrderPath        = "/api/v1/orders"
	kucoinLeveragePath     = "/api/v1/position/margin/leverage"
	kucoinTickerPath       = "/api/v1/ticker"
	kucoinContractsPath    = "/api/v1/contracts/active"
	kucoinCancelOrderPath  = "/api/v1/orders"
	kucoinStopOrderPath    = "/api/v1/stopOrders"
	kucoinCancelStopPath   = "/api/v1/stopOrders"
	kucoinPositionModePath = "/api/v1/position/margin/auto-deposit-status"
	kucoinFillsPath        = "/api/v1/fills"
	kucoinRecentFillsPath  = "/api/v1/recentFills"
)

// API channel configuration
const (
	kcPartnerID  = "NoFxFutures"
	kcPartnerKey = "d7c05b0c-c81b-4630-8fa8-ca6d049d3aae"
)

// KuCoinTrader implements types.Trader interface for KuCoin Futures
type KuCoinTrader struct {
	apiKey     string
	secretKey  string
	passphrase string

	// HTTP client
	httpClient *http.Client

	// Server time offset (local - server) in milliseconds
	serverTimeOffset int64
	serverTimeMutex  sync.RWMutex

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Positions cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Contract info cache
	contractsCache      map[string]*KuCoinContract
	contractsCacheTime  time.Time
	contractsCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
}

// KuCoinContract represents contract info
type KuCoinContract struct {
	Symbol          string  // Symbol
	BaseCurrency    string  // Base currency
	Multiplier      float64 // Contract multiplier
	LotSize         float64 // Minimum order quantity (lot size)
	TickSize        float64 // Minimum price increment
	MaxOrderQty     float64 // Maximum order quantity
	MaxLeverage     float64 // Maximum leverage
	MarkPrice       float64 // Current mark price
	IsInverse       bool    // Is inverse contract
	QuoteCurrency   string  // Quote currency
	IndexPriceScale int     // Index price decimal places
}

// KuCoinResponse represents KuCoin API response
type KuCoinResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// NewKuCoinTrader creates a new KuCoin trader instance
func NewKuCoinTrader(apiKey, secretKey, passphrase string) *KuCoinTrader {
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: http.DefaultTransport,
	}

	trader := &KuCoinTrader{
		apiKey:         apiKey,
		secretKey:      secretKey,
		passphrase:     passphrase,
		httpClient:     httpClient,
		cacheDuration:  15 * time.Second,
		contractsCache: make(map[string]*KuCoinContract),
	}

	// Sync server time on initialization
	if err := trader.syncServerTime(); err != nil {
		logger.Warnf("⚠️ Failed to sync KuCoin server time: %v (will retry on first request)", err)
	}

	logger.Infof("✓ KuCoin Futures trader initialized")
	return trader
}

// syncServerTime fetches KuCoin server time and calculates offset
func (t *KuCoinTrader) syncServerTime() error {
	resp, err := t.httpClient.Get(kucoinBaseURL + "/api/v1/timestamp")
	if err != nil {
		return fmt.Errorf("failed to get server time: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Code string `json:"code"`
		Data int64  `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != "200000" {
		return fmt.Errorf("server time API error: %s", result.Code)
	}

	serverTime := result.Data
	localTime := time.Now().UnixMilli()
	offset := localTime - serverTime

	t.serverTimeMutex.Lock()
	t.serverTimeOffset = offset
	t.serverTimeMutex.Unlock()

	logger.Infof("✓ KuCoin time synced: offset=%dms (local %d - server %d)", offset, localTime, serverTime)
	return nil
}

// getTimestamp returns the current timestamp adjusted for server time offset
func (t *KuCoinTrader) getTimestamp() string {
	t.serverTimeMutex.RLock()
	offset := t.serverTimeOffset
	t.serverTimeMutex.RUnlock()

	// Subtract offset to get server time from local time
	timestamp := time.Now().UnixMilli() - offset
	return strconv.FormatInt(timestamp, 10)
}

// sign generates KuCoin API signature
func (t *KuCoinTrader) sign(timestamp, method, requestPath, body string) string {
	// KuCoin signature: base64(HMAC-SHA256(timestamp + method + endpoint + body, secretKey))
	preHash := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// signPassphrase signs the passphrase with API v2
func (t *KuCoinTrader) signPassphrase(passphrase string) string {
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(passphrase))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// signPartner generates partner signature: base64(HMAC-SHA256(timestamp + partner + apiKey, partnerKey))
func (t *KuCoinTrader) signPartner(timestamp string) string {
	preHash := timestamp + kcPartnerID + t.apiKey
	h := hmac.New(sha256.New, []byte(kcPartnerKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest executes HTTP request
func (t *KuCoinTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize request body: %w", err)
		}
	}

	timestamp := t.getTimestamp()
	signature := t.sign(timestamp, method, path, string(bodyBytes))
	signedPassphrase := t.signPassphrase(t.passphrase)

	req, err := http.NewRequest(method, kucoinBaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Authentication headers
	req.Header.Set("KC-API-KEY", t.apiKey)
	req.Header.Set("KC-API-SIGN", signature)
	req.Header.Set("KC-API-TIMESTAMP", timestamp)
	req.Header.Set("KC-API-PASSPHRASE", signedPassphrase)
	req.Header.Set("KC-API-KEY-VERSION", "3")
	req.Header.Set("Content-Type", "application/json")

	// Partner headers
	req.Header.Set("KC-API-PARTNER", kcPartnerID)
	req.Header.Set("KC-API-PARTNER-SIGN", t.signPartner(timestamp))
	req.Header.Set("KC-API-PARTNER-VERIFY", "true")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var kcResp KuCoinResponse
	if err := json.Unmarshal(respBody, &kcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(respBody))
	}

	if kcResp.Code != "200000" {
		// If timestamp error, try to re-sync server time
		if kcResp.Code == "400002" || strings.Contains(kcResp.Msg, "TIMESTAMP") {
			logger.Warnf("⚠️ KuCoin timestamp error, re-syncing server time...")
			if err := t.syncServerTime(); err != nil {
				logger.Warnf("⚠️ Failed to re-sync server time: %v", err)
			}
		}
		return nil, fmt.Errorf("KuCoin API error: code=%s, msg=%s", kcResp.Code, kcResp.Msg)
	}

	return kcResp.Data, nil
}

// convertSymbol converts generic symbol to KuCoin format
// e.g. BTCUSDT -> XBTUSDTM (KuCoin uses XBT for BTC)
func (t *KuCoinTrader) convertSymbol(symbol string) string {
	// Remove USDT suffix
	base := strings.TrimSuffix(symbol, "USDT")
	// KuCoin uses XBT instead of BTC
	if base == "BTC" {
		base = "XBT"
	}
	return fmt.Sprintf("%sUSDTM", base)
}

// convertSymbolBack converts KuCoin format back to generic symbol
// e.g. XBTUSDTM -> BTCUSDT
func (t *KuCoinTrader) convertSymbolBack(kcSymbol string) string {
	// Remove M suffix
	sym := strings.TrimSuffix(kcSymbol, "M")
	// Convert XBT back to BTC
	if strings.HasPrefix(sym, "XBT") {
		sym = "BTC" + strings.TrimPrefix(sym, "XBT")
	}
	return sym
}

// getContract gets contract info
func (t *KuCoinTrader) getContract(symbol string) (*KuCoinContract, error) {
	kcSymbol := t.convertSymbol(symbol)

	// Check cache
	t.contractsCacheMutex.RLock()
	if contract, ok := t.contractsCache[kcSymbol]; ok && time.Since(t.contractsCacheTime) < 5*time.Minute {
		t.contractsCacheMutex.RUnlock()
		return contract, nil
	}
	t.contractsCacheMutex.RUnlock()

	// Get contract info
	data, err := t.doRequest("GET", kucoinContractsPath, nil)
	if err != nil {
		return nil, err
	}

	var contracts []struct {
		Symbol        string  `json:"symbol"`
		BaseCurrency  string  `json:"baseCurrency"`
		Multiplier    float64 `json:"multiplier"`
		LotSize       int64   `json:"lotSize"`
		TickSize      float64 `json:"tickSize"`
		MaxOrderQty   int64   `json:"maxOrderQty"`
		MaxLeverage   int     `json:"maxLeverage"`
		MarkPrice     float64 `json:"markPrice"`
		IsInverse     bool    `json:"isInverse"`
		QuoteCurrency string  `json:"quoteCurrency"`
	}

	if err := json.Unmarshal(data, &contracts); err != nil {
		return nil, err
	}

	// Update cache with all contracts
	t.contractsCacheMutex.Lock()
	for _, c := range contracts {
		t.contractsCache[c.Symbol] = &KuCoinContract{
			Symbol:        c.Symbol,
			BaseCurrency:  c.BaseCurrency,
			Multiplier:    c.Multiplier,
			LotSize:       float64(c.LotSize),
			TickSize:      c.TickSize,
			MaxOrderQty:   float64(c.MaxOrderQty),
			MaxLeverage:   float64(c.MaxLeverage),
			MarkPrice:     c.MarkPrice,
			IsInverse:     c.IsInverse,
			QuoteCurrency: c.QuoteCurrency,
		}
	}
	t.contractsCacheTime = time.Now()
	t.contractsCacheMutex.Unlock()

	// Return requested contract
	t.contractsCacheMutex.RLock()
	contract, ok := t.contractsCache[kcSymbol]
	t.contractsCacheMutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("contract info not found: %s", kcSymbol)
	}

	return contract, nil
}

// quantityToLots converts quantity (in base asset) to lots
func (t *KuCoinTrader) quantityToLots(symbol string, quantity float64) (int64, error) {
	contract, err := t.getContract(symbol)
	if err != nil {
		return 0, err
	}

	// lots = quantity / multiplier
	lots := quantity / contract.Multiplier

	// Round to integer (KuCoin uses integer lots)
	lotsInt := int64(math.Round(lots))

	// Check max order quantity
	if contract.MaxOrderQty > 0 && float64(lotsInt) > contract.MaxOrderQty {
		logger.Infof("⚠️ KuCoin order quantity %d exceeds max %d, reducing to max", lotsInt, int64(contract.MaxOrderQty))
		lotsInt = int64(contract.MaxOrderQty)
	}

	return lotsInt, nil
}
