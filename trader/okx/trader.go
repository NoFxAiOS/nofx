package okx

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"nofx/logger"
	"strings"
	"sync"
	"time"
)

// OKX API endpoints
const (
	okxBaseURL               = "https://www.okx.com"
	okxAccountPath           = "/api/v5/account/balance"
	okxPositionPath          = "/api/v5/account/positions"
	okxOrderPath             = "/api/v5/trade/order"
	okxLeveragePath          = "/api/v5/account/set-leverage"
	okxTickerPath            = "/api/v5/market/ticker"
	okxInstrumentsPath       = "/api/v5/public/instruments"
	okxCancelOrderPath       = "/api/v5/trade/cancel-order"
	okxPendingOrdersPath     = "/api/v5/trade/orders-pending"
	okxAlgoOrderPath         = "/api/v5/trade/order-algo"
	okxCancelAlgoPath        = "/api/v5/trade/cancel-algos"
	okxAlgoPendingPath       = "/api/v5/trade/orders-algo-pending"
	okxAdvanceAlgoPath       = "/api/v5/trade/order-algo"
	okxCancelAdvanceAlgoPath = "/api/v5/trade/cancel-advance-algos"
	okxPositionModePath      = "/api/v5/account/set-position-mode"
	okxAccountConfigPath     = "/api/v5/account/config"
)

// OKXTrader OKX futures trader
type OKXTrader struct {
	apiKey     string
	secretKey  string
	passphrase string

	// Margin mode setting
	isCrossMargin bool

	// Position mode: "long_short_mode" (hedge) or "net_mode" (one-way)
	positionMode string

	// HTTP client (proxy disabled)
	httpClient *http.Client

	// Balance cache
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// Positions cache
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// Instrument info cache
	instrumentsCache      map[string]*OKXInstrument
	instrumentsCacheTime  time.Time
	instrumentsCacheMutex sync.RWMutex

	// Cache duration
	cacheDuration time.Duration
}

// OKXInstrument OKX instrument info
type OKXInstrument struct {
	InstID   string  // Instrument ID
	CtVal    float64 // Contract value
	CtMult   float64 // Contract multiplier
	LotSz    float64 // Minimum order size
	MinSz    float64 // Minimum order size
	MaxMktSz float64 // Maximum market order size
	TickSz   float64 // Minimum price increment
	CtType   string  // Contract type
}

// OKXResponse OKX API response
type OKXResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// OKX order tag
var okxTag = func() string {
	b, _ := base64.StdEncoding.DecodeString("NGMzNjNjODFlZGM1QkNERQ==")
	return string(b)
}()

func okxReasonTag(reason string) string {
	reason = strings.TrimSpace(strings.ToLower(reason))
	if reason == "" {
		return okxTag
	}
	tag := fmt.Sprintf("%s_%s", okxTag, reason)
	if len(tag) > 16 {
		tag = tag[:16]
	}
	return tag
}

// genOkxClOrdID generates OKX order ID
func genOkxClOrdID() string {
	timestamp := time.Now().UnixNano() % 10000000000000
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	// OKX clOrdId max 32 characters
	orderID := fmt.Sprintf("%s%d%s", okxTag, timestamp, randomHex)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}
	return orderID
}

// NewOKXTrader creates OKX trader
func NewOKXTrader(apiKey, secretKey, passphrase string) *OKXTrader {
	// Use a dedicated transport instead of http.DefaultTransport so we can tolerate
	// transient upstream EOF / reset issues without polluting global client behavior.
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	trader := &OKXTrader{
		apiKey:           apiKey,
		secretKey:        secretKey,
		passphrase:       passphrase,
		httpClient:       httpClient,
		cacheDuration:    15 * time.Second,
		instrumentsCache: make(map[string]*OKXInstrument),
	}

	// Get current position mode first
	if err := trader.detectPositionMode(); err != nil {
		logger.Infof("⚠️ Failed to detect OKX position mode: %v, assuming dual mode", err)
		trader.positionMode = "long_short_mode"
	}

	// Try to set dual position mode (only if not already)
	if trader.positionMode != "long_short_mode" {
		if err := trader.setPositionMode(); err != nil {
			logger.Infof("⚠️ Failed to set OKX position mode: %v (current mode: %s)", err, trader.positionMode)
		}
	}

	logger.Infof("✓ OKX trader initialized with position mode: %s", trader.positionMode)
	return trader
}

// detectPositionMode gets current position mode from account config
func (t *OKXTrader) detectPositionMode() error {
	data, err := t.doRequest("GET", okxAccountConfigPath, nil)
	if err != nil {
		return fmt.Errorf("failed to get account config: %w", err)
	}

	var configs []struct {
		PosMode string `json:"posMode"`
	}

	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("failed to parse account config: %w", err)
	}

	if len(configs) > 0 {
		t.positionMode = configs[0].PosMode
		logger.Infof("✓ Detected OKX position mode: %s", t.positionMode)
	}

	return nil
}

// setPositionMode sets dual position mode
func (t *OKXTrader) setPositionMode() error {
	body := map[string]string{
		"posMode": "long_short_mode", // Dual position mode
	}

	_, err := t.doRequest("POST", okxPositionModePath, body)
	if err != nil {
		// Ignore error if already in dual position mode
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "Position mode is not modified") {
			logger.Infof("  ✓ OKX account is already in dual position mode")
			return nil
		}
		return err
	}

	logger.Infof("  ✓ OKX account switched to dual position mode")
	return nil
}

// sign generates OKX API signature
func (t *OKXTrader) sign(timestamp, method, requestPath, body string) string {
	preHash := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest executes HTTP request
func (t *OKXTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		signature := t.sign(timestamp, method, path, string(bodyBytes))

		req, err := http.NewRequest(method, okxBaseURL+path, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("OK-ACCESS-KEY", t.apiKey)
		req.Header.Set("OK-ACCESS-SIGN", signature)
		req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
		req.Header.Set("OK-ACCESS-PASSPHRASE", t.passphrase)
		req.Header.Set("Content-Type", "application/json")
		// Set request header
		req.Header.Set("x-simulated-trading", "0")
		req.Header.Set("Connection", "keep-alive")

		resp, err := t.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if shouldRetryOKXError(err) && attempt < 3 {
				logger.Infof("⚠️ OKX request retry %d/3 for %s %s after error: %v", attempt, method, path, err)
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("request failed: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < 3 {
				logger.Infof("⚠️ OKX response read retry %d/3 for %s %s after error: %v", attempt, method, path, readErr)
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, fmt.Errorf("failed to read response: %w", readErr)
		}

		var okxResp OKXResponse
		if err := json.Unmarshal(respBody, &okxResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// code=1 indicates partial success, need to check specific results in data
		// code=2 indicates complete failure
		if okxResp.Code != "0" && okxResp.Code != "1" {
			return nil, fmt.Errorf("OKX API error: code=%s, msg=%s", okxResp.Code, okxResp.Msg)
		}

		return okxResp.Data, nil
	}

	return nil, fmt.Errorf("request failed after retries: %w", lastErr)
}

func shouldRetryOKXError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "eof") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "tls handshake timeout")
}

// convertSymbol converts generic symbol to OKX format
// e.g. BTCUSDT -> BTC-USDT-SWAP
func (t *OKXTrader) convertSymbol(symbol string) string {
	// Remove USDT suffix and build OKX format
	base := strings.TrimSuffix(symbol, "USDT")
	return fmt.Sprintf("%s-USDT-SWAP", base)
}

// convertSymbolBack converts OKX format back to generic symbol
// e.g. BTC-USDT-SWAP -> BTCUSDT
func (t *OKXTrader) convertSymbolBack(instId string) string {
	parts := strings.Split(instId, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return instId
}

// FormatQuantity formats quantity (converts base asset quantity to contract count)
func (t *OKXTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Sprintf("%.3f", quantity), nil
	}

	// OKX uses contract count: quantity (in base asset) / ctVal (asset per contract)
	sz := quantity / inst.CtVal
	return t.formatSize(sz, inst), nil
}

// formatPrice rounds a price to the instrument's tick size so that the value
// sent to the OKX API matches what OKX will actually store. Without this
// rounding OKX silently truncates the price, which can cause verification
// mismatches when the plan target differs from the exchange order.
func (t *OKXTrader) formatPrice(price float64, inst *OKXInstrument) string {
	if inst.TickSz > 0 {
		// Round to the nearest multiple of tick size
		steps := math.Round(price / inst.TickSz)
		price = steps * inst.TickSz
	}
	// Determine decimal places from tick size string representation
	precision := tickSzPrecision(inst.TickSz)
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, price)
}

// tickSzPrecision returns the number of decimal places implied by a tick size.
func tickSzPrecision(tickSz float64) int {
	if tickSz <= 0 || tickSz >= 1 {
		return 0
	}
	s := fmt.Sprintf("%f", tickSz)
	s = strings.TrimRight(s, "0")
	dot := strings.Index(s, ".")
	if dot == -1 {
		return 0
	}
	return len(s) - dot - 1
}

// formatSize formats contract size
func (t *OKXTrader) formatSize(sz float64, inst *OKXInstrument) string {
	// Determine precision based on lotSz
	if inst.LotSz >= 1 {
		return fmt.Sprintf("%.0f", sz)
	}

	// Calculate decimal places
	lotSzStr := fmt.Sprintf("%f", inst.LotSz)
	dotIndex := strings.Index(lotSzStr, ".")
	if dotIndex == -1 {
		return fmt.Sprintf("%.0f", sz)
	}

	// Remove trailing zeros
	lotSzStr = strings.TrimRight(lotSzStr, "0")
	precision := len(lotSzStr) - dotIndex - 1

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, sz)
}
