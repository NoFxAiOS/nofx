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
	"net/http"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
	"nofx/trader/types"
)

// OKX API endpoints
const (
	okxBaseURL           = "https://www.okx.com"
	okxAccountPath       = "/api/v5/account/balance"
	okxPositionPath      = "/api/v5/account/positions"
	okxOrderPath         = "/api/v5/trade/order"
	okxLeveragePath      = "/api/v5/account/set-leverage"
	okxTickerPath        = "/api/v5/market/ticker"
	okxInstrumentsPath   = "/api/v5/public/instruments"
	okxCancelOrderPath   = "/api/v5/trade/cancel-order"
	okxPendingOrdersPath = "/api/v5/trade/orders-pending"
	okxAlgoOrderPath     = "/api/v5/trade/order-algo"
	okxCancelAlgoPath    = "/api/v5/trade/cancel-algos"
	okxAlgoPendingPath   = "/api/v5/trade/orders-algo-pending"
	okxPositionModePath  = "/api/v5/account/set-position-mode"
	okxAccountConfigPath = "/api/v5/account/config"
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
	// Use default transport which respects system proxy settings
	// OKX requires proxy in China due to DNS pollution
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: http.DefaultTransport,
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

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
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

// GetBalance gets account balance
func (t *OKXTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		logger.Infof("✓ Using cached OKX account balance")
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	logger.Infof("🔄 Calling OKX API to get account balance...")
	data, err := t.doRequest("GET", okxAccountPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	var balances []struct {
		TotalEq string `json:"totalEq"`
		AdjEq   string `json:"adjEq"`
		IsoEq   string `json:"isoEq"`
		OrdFroz string `json:"ordFroz"`
		Details []struct {
			Ccy      string `json:"ccy"`
			Eq       string `json:"eq"`
			CashBal  string `json:"cashBal"`
			AvailBal string `json:"availBal"`
			UPL      string `json:"upl"`
		} `json:"details"`
	}

	if err := json.Unmarshal(data, &balances); err != nil {
		return nil, fmt.Errorf("failed to parse balance data: %w", err)
	}

	if len(balances) == 0 {
		return nil, fmt.Errorf("no balance data received")
	}

	balance := balances[0]

	// Find USDT balance
	var usdtAvail, usdtUPL float64
	for _, detail := range balance.Details {
		if detail.Ccy == "USDT" {
			usdtAvail, _ = strconv.ParseFloat(detail.AvailBal, 64)
			usdtUPL, _ = strconv.ParseFloat(detail.UPL, 64)
			break
		}
	}

	totalEq, _ := strconv.ParseFloat(balance.TotalEq, 64)

	result := map[string]interface{}{
		"totalWalletBalance":    totalEq,
		"availableBalance":      usdtAvail,
		"totalUnrealizedProfit": usdtUPL,
	}

	logger.Infof("✓ OKX balance: Total equity=%.2f, Available=%.2f, Unrealized PnL=%.2f", totalEq, usdtAvail, usdtUPL)

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions gets all positions
func (t *OKXTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		logger.Infof("✓ Using cached OKX positions")
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	logger.Infof("🔄 Calling OKX API to get positions...")
	data, err := t.doRequest("GET", okxPositionPath+"?instType=SWAP", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positions []struct {
		InstId  string `json:"instId"`
		PosSide string `json:"posSide"`
		Pos     string `json:"pos"`
		AvgPx   string `json:"avgPx"`
		MarkPx  string `json:"markPx"`
		Upl     string `json:"upl"`
		Lever   string `json:"lever"`
		LiqPx   string `json:"liqPx"`
		Margin  string `json:"margin"`
		MgnMode string `json:"mgnMode"` // Margin mode: "cross" or "isolated"
		CTime   string `json:"cTime"`   // Position created time (ms)
		UTime   string `json:"uTime"`   // Position last update time (ms)
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse position data: %w", err)
	}

	logger.Infof("🔍 OKX raw positions response: %d positions", len(positions))
	var result []map[string]interface{}
	for _, pos := range positions {
		logger.Infof("🔍 OKX raw position: instId=%s, posSide=%s, pos=%s, mgnMode=%s", pos.InstId, pos.PosSide, pos.Pos, pos.MgnMode)
		contractCount, _ := strconv.ParseFloat(pos.Pos, 64)
		if contractCount == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(pos.AvgPx, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPx, 64)
		upl, _ := strconv.ParseFloat(pos.Upl, 64)
		leverage, _ := strconv.ParseFloat(pos.Lever, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiqPx, 64)

		// Convert symbol format
		symbol := t.convertSymbolBack(pos.InstId)
		logger.Infof("🔍 OKX symbol conversion: %s → %s", pos.InstId, symbol)

		// Determine direction and ensure contractCount is positive
		side := "long"
		if pos.PosSide == "short" {
			side = "short"
		}
		// OKX short position's pos is negative, need to take absolute value
		if contractCount < 0 {
			contractCount = -contractCount
		}

		// Convert contract count to actual position amount (in base asset)
		// positionAmt = contractCount * ctVal
		inst, err := t.getInstrument(symbol)
		posAmt := contractCount
		if err == nil && inst.CtVal > 0 {
			posAmt = contractCount * inst.CtVal
			logger.Debugf("  📊 OKX position %s: contracts=%.4f, ctVal=%.6f, posAmt=%.6f", symbol, contractCount, inst.CtVal, posAmt)
		}

		// Parse timestamps
		cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)

		// Default to cross margin mode if not specified
		mgnMode := pos.MgnMode
		if mgnMode == "" {
			mgnMode = "cross"
		}

		posMap := map[string]interface{}{
			"symbol":           symbol,
			"positionAmt":      posAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": upl,
			"leverage":         leverage,
			"liquidationPrice": liqPrice,
			"side":             side,
			"mgnMode":          mgnMode, // Margin mode: "cross" or "isolated"
			"createdTime":      cTime,   // Position open time (ms)
			"updatedTime":      uTime,   // Position last update time (ms)
		}
		result = append(result, posMap)
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// InvalidatePositionCache clears the position cache to force fresh data on next call
func (t *OKXTrader) InvalidatePositionCache() {
	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()
}

// getInstrument gets instrument info
func (t *OKXTrader) getInstrument(symbol string) (*OKXInstrument, error) {
	instId := t.convertSymbol(symbol)

	// Check cache
	t.instrumentsCacheMutex.RLock()
	if inst, ok := t.instrumentsCache[instId]; ok && time.Since(t.instrumentsCacheTime) < 5*time.Minute {
		t.instrumentsCacheMutex.RUnlock()
		return inst, nil
	}
	t.instrumentsCacheMutex.RUnlock()

	// Get instrument info
	path := fmt.Sprintf("%s?instType=SWAP&instId=%s", okxInstrumentsPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var instruments []struct {
		InstId   string `json:"instId"`
		CtVal    string `json:"ctVal"`
		CtMult   string `json:"ctMult"`
		LotSz    string `json:"lotSz"`
		MinSz    string `json:"minSz"`
		MaxMktSz string `json:"maxMktSz"` // Maximum market order size
		TickSz   string `json:"tickSz"`
		CtType   string `json:"ctType"`
	}

	if err := json.Unmarshal(data, &instruments); err != nil {
		return nil, err
	}

	if len(instruments) == 0 {
		return nil, fmt.Errorf("instrument info not found: %s", instId)
	}

	inst := instruments[0]
	ctVal, _ := strconv.ParseFloat(inst.CtVal, 64)
	ctMult, _ := strconv.ParseFloat(inst.CtMult, 64)
	lotSz, _ := strconv.ParseFloat(inst.LotSz, 64)
	minSz, _ := strconv.ParseFloat(inst.MinSz, 64)
	maxMktSz, _ := strconv.ParseFloat(inst.MaxMktSz, 64)
	tickSz, _ := strconv.ParseFloat(inst.TickSz, 64)

	instrument := &OKXInstrument{
		InstID:   inst.InstId,
		CtVal:    ctVal,
		CtMult:   ctMult,
		LotSz:    lotSz,
		MinSz:    minSz,
		MaxMktSz: maxMktSz,
		TickSz:   tickSz,
		CtType:   inst.CtType,
	}

	// Update cache
	t.instrumentsCacheMutex.Lock()
	t.instrumentsCache[instId] = instrument
	t.instrumentsCacheTime = time.Now()
	t.instrumentsCacheMutex.Unlock()

	return instrument, nil
}

// SetMarginMode sets margin mode
func (t *OKXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	instId := t.convertSymbol(symbol)

	mgnMode := "isolated"
	if isCrossMargin {
		mgnMode = "cross"
	}

	body := map[string]interface{}{
		"instId":  instId,
		"mgnMode": mgnMode,
	}

	_, err := t.doRequest("POST", "/api/v5/account/set-isolated-mode", body)
	if err != nil {
		// Ignore error if already in target mode
		if strings.Contains(err.Error(), "already") {
			logger.Infof("  ✓ %s margin mode is already %s", symbol, mgnMode)
			return nil
		}
		// Cannot change when there are positions
		if strings.Contains(err.Error(), "position") {
			logger.Infof("  ⚠️ %s has positions, cannot change margin mode", symbol)
			return nil
		}
		return err
	}

	logger.Infof("  ✓ %s margin mode set to %s", symbol, mgnMode)
	return nil
}

// SetLeverage sets leverage
func (t *OKXTrader) SetLeverage(symbol string, leverage int) error {
	instId := t.convertSymbol(symbol)

	// Set leverage for both long and short
	for _, posSide := range []string{"long", "short"} {
		body := map[string]interface{}{
			"instId":  instId,
			"lever":   strconv.Itoa(leverage),
			"mgnMode": "cross",
			"posSide": posSide,
		}

		_, err := t.doRequest("POST", okxLeveragePath, body)
		if err != nil {
			// Ignore if already at target leverage
			if strings.Contains(err.Error(), "same") {
				continue
			}
			logger.Infof("  ⚠️ Failed to set %s %s leverage: %v", symbol, posSide, err)
		}
	}

	logger.Infof("  ✓ %s leverage set to %dx", symbol, leverage)
	return nil
}

// OpenLong opens long position
func (t *OKXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel old orders
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	instId := t.convertSymbol(symbol)

	// Get instrument info and calculate contract size
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// OKX uses contract count, need to convert quantity (in base asset) to contract count
	// sz = quantity / ctVal (number of contracts = asset amount / asset per contract)
	sz := quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	logger.Infof("  📊 OKX OpenLong: quantity=%.6f, ctVal=%.6f, contracts=%.2f", quantity, inst.CtVal, sz)

	// Check max market order size limit
	if inst.MaxMktSz > 0 && sz > inst.MaxMktSz {
		logger.Infof("  ⚠️ OKX market order size %.2f exceeds max %.2f, reducing to max", sz, inst.MaxMktSz)
		sz = inst.MaxMktSz
		szStr = t.formatSize(sz, inst)
	}

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    "buy",
		"posSide": "long",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "unknown error"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("failed to open long position: %s", msg)
	}

	logger.Infof("✓ OKX opened long position successfully: %s size: %s", symbol, szStr)
	logger.Infof("  Order ID: %s", orders[0].OrdId)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort opens short position
func (t *OKXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel old orders
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	instId := t.convertSymbol(symbol)

	// Get instrument info and calculate contract size
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// OKX uses contract count, need to convert quantity (in base asset) to contract count
	// sz = quantity / ctVal (number of contracts = asset amount / asset per contract)
	sz := quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	logger.Infof("  📊 OKX OpenShort: quantity=%.6f, ctVal=%.6f, contracts=%.2f", quantity, inst.CtVal, sz)

	// Check max market order size limit
	if inst.MaxMktSz > 0 && sz > inst.MaxMktSz {
		logger.Infof("  ⚠️ OKX market order size %.2f exceeds max %.2f, reducing to max", sz, inst.MaxMktSz)
		sz = inst.MaxMktSz
		szStr = t.formatSize(sz, inst)
	}

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    "sell",
		"posSide": "short",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "unknown error"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("failed to open short position: %s", msg)
	}

	logger.Infof("✓ OKX opened short position successfully: %s size: %s", symbol, szStr)
	logger.Infof("  Order ID: %s", orders[0].OrdId)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong closes long position
func (t *OKXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)

	// Get instrument info for contract conversion
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Invalidate position cache and get fresh positions
	t.InvalidatePositionCache()
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	// Find actual position from exchange
	var actualQty float64
	var posFound bool
	var posMgnMode string = "cross" // Default to cross margin
	logger.Infof("🔍 OKX CloseLong: searching for symbol=%s in %d positions", symbol, len(positions))
	for _, pos := range positions {
		logger.Infof("🔍 OKX position: symbol=%v, side=%v, positionAmt=%v, mgnMode=%v", pos["symbol"], pos["side"], pos["positionAmt"], pos["mgnMode"])
		if pos["symbol"] == symbol {
			side := pos["side"].(string)
			// In net_mode, "long" means positive position
			// In dual mode, check explicit "long" side
			if side == "long" || (t.positionMode == "net_mode" && side == "long") {
				actualQty = pos["positionAmt"].(float64)
				posFound = true
				if mgnMode, ok := pos["mgnMode"].(string); ok && mgnMode != "" {
					posMgnMode = mgnMode
				}
				logger.Infof("🔍 OKX CloseLong: found matching position! qty=%.6f, mgnMode=%s", actualQty, posMgnMode)
				break
			}
		}
	}

	if !posFound || actualQty == 0 {
		logger.Infof("🔍 OKX CloseLong: NO position found for %s LONG", symbol)
		return map[string]interface{}{
			"status":  "NO_POSITION",
			"message": fmt.Sprintf("No long position found for %s on OKX", symbol),
		}, nil
	}

	// Use actual quantity from exchange (more accurate than passed quantity)
	if quantity == 0 || quantity > actualQty {
		quantity = actualQty
	}

	// Convert quantity (base asset) to contract count
	// contracts = quantity / ctVal
	contracts := quantity / inst.CtVal
	szStr := t.formatSize(contracts, inst)

	logger.Infof("🔻 OKX close long: symbol=%s, instId=%s, quantity=%.6f, ctVal=%.6f, contracts=%.2f, szStr=%s, posMode=%s, mgnMode=%s",
		symbol, instId, quantity, inst.CtVal, contracts, szStr, t.positionMode, posMgnMode)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  posMgnMode, // Use position's actual margin mode (cross or isolated)
		"side":    "sell",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	// Only add posSide in dual mode (long_short_mode)
	if t.positionMode == "long_short_mode" {
		body["posSide"] = "long"
	}

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
	}

	var orders []struct {
		OrdId string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "unknown error"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("failed to close long position: %s", msg)
	}

	logger.Infof("✓ OKX closed long position successfully: %s", symbol)

	// Cancel pending orders after closing position
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort closes short position
func (t *OKXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)

	// Get instrument info for contract conversion
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Invalidate position cache and get fresh positions
	t.InvalidatePositionCache()
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	// Find actual position from exchange
	var actualQty float64
	var posFound bool
	var posMgnMode string = "cross" // Default to cross margin
	logger.Infof("🔍 OKX CloseShort searching positions: symbol=%s, current position count=%d", symbol, len(positions))
	for _, pos := range positions {
		logger.Infof("🔍 OKX position: symbol=%v, side=%v, positionAmt=%v, mgnMode=%v",
			pos["symbol"], pos["side"], pos["positionAmt"], pos["mgnMode"])
		if pos["symbol"] == symbol && pos["side"] == "short" {
			actualQty = pos["positionAmt"].(float64)
			posFound = true
			if mgnMode, ok := pos["mgnMode"].(string); ok && mgnMode != "" {
				posMgnMode = mgnMode
			}
			logger.Infof("🔍 OKX found short position: quantity=%f (base asset), mgnMode=%s", actualQty, posMgnMode)
			break
		}
	}

	if !posFound || actualQty == 0 {
		return map[string]interface{}{
			"status":  "NO_POSITION",
			"message": fmt.Sprintf("No short position found for %s on OKX", symbol),
		}, nil
	}

	// Use actual quantity from exchange (more accurate than passed quantity)
	if quantity == 0 || quantity > actualQty {
		quantity = actualQty
	}

	// Ensure quantity is positive (OKX sz parameter must be positive)
	if quantity < 0 {
		quantity = -quantity
	}

	// Convert quantity (base asset) to contract count
	// contracts = quantity / ctVal
	contracts := quantity / inst.CtVal
	szStr := t.formatSize(contracts, inst)

	logger.Infof("🔻 OKX close short: symbol=%s, quantity=%.6f, ctVal=%.6f, contracts=%.2f, szStr=%s, posMode=%s, mgnMode=%s",
		symbol, quantity, inst.CtVal, contracts, szStr, t.positionMode, posMgnMode)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  posMgnMode, // Use position's actual margin mode (cross or isolated)
		"side":    "buy",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	// Only add posSide in dual mode (long_short_mode)
	if t.positionMode == "long_short_mode" {
		body["posSide"] = "short"
	}

	logger.Infof("🔻 OKX close short request body: %+v", body)

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
	}

	var orders []struct {
		OrdId string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "unknown error"
		if len(orders) > 0 {
			msg = fmt.Sprintf("sCode=%s, sMsg=%s", orders[0].SCode, orders[0].SMsg)
		}
		logger.Infof("❌ OKX failed to close short position: %s, response: %s", msg, string(data))
		return nil, fmt.Errorf("failed to close short position: %s", msg)
	}

	logger.Infof("✓ OKX closed short position successfully: %s, ordId=%s", symbol, orders[0].OrdId)

	// Cancel pending orders after closing position
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// GetMarketPrice gets market price
func (t *OKXTrader) GetMarketPrice(symbol string) (float64, error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("%s?instId=%s", okxTickerPath, instId)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	var tickers []struct {
		Last string `json:"last"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, err
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("no price data received")
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// SetStopLoss sets stop loss order
func (t *OKXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	instId := t.convertSymbol(symbol)

	// Get instrument info
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Calculate contract size: quantity (in base asset) / ctVal (asset per contract)
	sz := quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// Determine direction
	side := "sell"
	posSide := "long"
	if strings.ToUpper(positionSide) == "SHORT" {
		side = "buy"
		posSide = "short"
	}

	body := map[string]interface{}{
		"instId":      instId,
		"tdMode":      "cross",
		"side":        side,
		"posSide":     posSide,
		"ordType":     "conditional",
		"sz":          szStr,
		"slTriggerPx": fmt.Sprintf("%.8f", stopPrice),
		"slOrdPx":     "-1", // Market price
		"tag":         okxTag,
	}

	_, err = t.doRequest("POST", okxAlgoOrderPath, body)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	logger.Infof("  Stop loss price set: %.4f", stopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *OKXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	instId := t.convertSymbol(symbol)

	// Get instrument info
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Calculate contract size: quantity (in base asset) / ctVal (asset per contract)
	sz := quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// Determine direction
	side := "sell"
	posSide := "long"
	if strings.ToUpper(positionSide) == "SHORT" {
		side = "buy"
		posSide = "short"
	}

	body := map[string]interface{}{
		"instId":      instId,
		"tdMode":      "cross",
		"side":        side,
		"posSide":     posSide,
		"ordType":     "conditional",
		"sz":          szStr,
		"tpTriggerPx": fmt.Sprintf("%.8f", takeProfitPrice),
		"tpOrdPx":     "-1", // Market price
		"tag":         okxTag,
	}

	_, err = t.doRequest("POST", okxAlgoOrderPath, body)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	logger.Infof("  Take profit price set: %.4f", takeProfitPrice)
	return nil
}

// CancelStopLossOrders cancels stop loss orders
func (t *OKXTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "sl")
}

// CancelTakeProfitOrders cancels take profit orders
func (t *OKXTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "tp")
}

// cancelAlgoOrders cancels algo orders
func (t *OKXTrader) cancelAlgoOrders(symbol string, orderType string) error {
	instId := t.convertSymbol(symbol)

	// Get pending algo orders
	path := fmt.Sprintf("%s?instType=SWAP&instId=%s&ordType=conditional", okxAlgoPendingPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return err
	}

	var orders []struct {
		AlgoId string `json:"algoId"`
		InstId string `json:"instId"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	canceledCount := 0
	for _, order := range orders {
		body := []map[string]interface{}{
			{
				"algoId": order.AlgoId,
				"instId": order.InstId,
			},
		}

		_, err := t.doRequest("POST", okxCancelAlgoPath, body)
		if err != nil {
			logger.Infof("  ⚠️ Failed to cancel algo order: %v", err)
			continue
		}
		canceledCount++
	}

	if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d algo orders for %s", canceledCount, symbol)
	}

	return nil
}

// CancelAllOrders cancels all pending orders
func (t *OKXTrader) CancelAllOrders(symbol string) error {
	instId := t.convertSymbol(symbol)

	// Get pending orders
	path := fmt.Sprintf("%s?instType=SWAP&instId=%s", okxPendingOrdersPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return err
	}

	var orders []struct {
		OrdId  string `json:"ordId"`
		InstId string `json:"instId"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return err
	}

	// Batch cancel
	for _, order := range orders {
		body := map[string]interface{}{
			"instId": order.InstId,
			"ordId":  order.OrdId,
		}
		t.doRequest("POST", okxCancelOrderPath, body)
	}

	// Also cancel algo orders
	t.cancelAlgoOrders(symbol, "")

	if len(orders) > 0 {
		logger.Infof("  ✓ Canceled all pending orders for %s", symbol)
	}

	return nil
}

// CancelStopOrders cancels stop loss and take profit orders
func (t *OKXTrader) CancelStopOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "")
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

// GetOrderStatus gets order status
func (t *OKXTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("/api/v5/trade/order?instId=%s&ordId=%s", instId, orderID)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	var orders []struct {
		OrdId     string `json:"ordId"`
		State     string `json:"state"`
		AvgPx     string `json:"avgPx"`
		AccFillSz string `json:"accFillSz"`
		Fee       string `json:"fee"`
		Side      string `json:"side"`
		OrdType   string `json:"ordType"`
		CTime     string `json:"cTime"`
		UTime     string `json:"uTime"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return nil, fmt.Errorf("order not found")
	}

	order := orders[0]
	avgPrice, _ := strconv.ParseFloat(order.AvgPx, 64)
	fillSz, _ := strconv.ParseFloat(order.AccFillSz, 64) // This is in contracts
	fee, _ := strconv.ParseFloat(order.Fee, 64)
	cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
	uTime, _ := strconv.ParseInt(order.UTime, 10, 64)

	// Convert contract count to base asset quantity
	// executedQty = contracts * ctVal
	executedQty := fillSz
	inst, err := t.getInstrument(symbol)
	if err == nil && inst.CtVal > 0 {
		executedQty = fillSz * inst.CtVal
		logger.Debugf("  📊 OKX order %s: fillSz(contracts)=%.4f, ctVal=%.6f, executedQty=%.6f", orderID, fillSz, inst.CtVal, executedQty)
	}

	// Status mapping
	statusMap := map[string]string{
		"filled":           "FILLED",
		"live":             "NEW",
		"partially_filled": "PARTIALLY_FILLED",
		"canceled":         "CANCELED",
	}

	status := statusMap[order.State]
	if status == "" {
		status = order.State
	}

	return map[string]interface{}{
		"orderId":     order.OrdId,
		"symbol":      symbol,
		"status":      status,
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"side":        order.Side,
		"type":        order.OrdType,
		"time":        cTime,
		"updateTime":  uTime,
		"commission":  -fee, // OKX returns negative value
	}, nil
}

// OKX order tag
var okxTag = func() string {
	b, _ := base64.StdEncoding.DecodeString("NGMzNjNjODFlZGM1QkNERQ==")
	return string(b)
}()

// GetClosedPnL retrieves closed position PnL records from OKX
// OKX API: /api/v5/account/positions-history
func (t *OKXTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	// Build query path with parameters
	path := fmt.Sprintf("/api/v5/account/positions-history?instType=SWAP&limit=%d", limit)
	if !startTime.IsZero() {
		path += fmt.Sprintf("&after=%d", startTime.UnixMilli())
	}

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions history: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			InstID      string `json:"instId"`      // Instrument ID (e.g., "BTC-USDT-SWAP")
			Direction   string `json:"direction"`   // Position direction: "long" or "short"
			OpenAvgPx   string `json:"openAvgPx"`   // Average open price
			CloseAvgPx  string `json:"closeAvgPx"`  // Average close price
			CloseTotalPos string `json:"closeTotalPos"` // Closed position quantity
			RealizedPnl string `json:"realizedPnl"` // Realized PnL
			Fee         string `json:"fee"`         // Total fee
			FundingFee  string `json:"fundingFee"`  // Funding fee
			Lever       string `json:"lever"`       // Leverage
			CTime       string `json:"cTime"`       // Position open time
			UTime       string `json:"uTime"`       // Position close time
			Type        string `json:"type"`        // Close type: 1=close position, 2=partial close, 3=liquidation, 4=partial liquidation
			PosId       string `json:"posId"`       // Position ID
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Code != "0" {
		return nil, fmt.Errorf("OKX API error: %s - %s", resp.Code, resp.Msg)
	}

	records := make([]types.ClosedPnLRecord, 0, len(resp.Data))

	for _, pos := range resp.Data {
		record := types.ClosedPnLRecord{}

		// Convert instrument ID to standard format (BTC-USDT-SWAP -> BTCUSDT)
		parts := strings.Split(pos.InstID, "-")
		if len(parts) >= 2 {
			record.Symbol = parts[0] + parts[1]
		} else {
			record.Symbol = pos.InstID
		}

		// Side
		record.Side = pos.Direction // OKX already returns "long" or "short"

		// Prices
		record.EntryPrice, _ = strconv.ParseFloat(pos.OpenAvgPx, 64)
		record.ExitPrice, _ = strconv.ParseFloat(pos.CloseAvgPx, 64)

		// Quantity
		record.Quantity, _ = strconv.ParseFloat(pos.CloseTotalPos, 64)

		// PnL
		record.RealizedPnL, _ = strconv.ParseFloat(pos.RealizedPnl, 64)

		// Fee
		fee, _ := strconv.ParseFloat(pos.Fee, 64)
		fundingFee, _ := strconv.ParseFloat(pos.FundingFee, 64)
		record.Fee = -fee + fundingFee // Fee is negative in OKX

		// Leverage
		lev, _ := strconv.ParseFloat(pos.Lever, 64)
		record.Leverage = int(lev)

		// Times
		cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)
		record.EntryTime = time.UnixMilli(cTime).UTC()
		record.ExitTime = time.UnixMilli(uTime).UTC()

		// Close type
		switch pos.Type {
		case "1", "2":
			record.CloseType = "unknown" // Could be manual or AI, need to cross-reference
		case "3", "4":
			record.CloseType = "liquidation"
		default:
			record.CloseType = "unknown"
		}

		// Exchange ID
		record.ExchangeID = pos.PosId

		records = append(records, record)
	}

	return records, nil
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *OKXTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	instId := t.convertSymbol(symbol)
	var result []types.OpenOrder

	// 1. Get pending limit orders
	path := fmt.Sprintf("%s?instId=%s&instType=SWAP", okxPendingOrdersPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		logger.Warnf("[OKX] Failed to get pending orders: %v", err)
	}
	if err == nil && data != nil {
		var orders []struct {
			OrdId   string `json:"ordId"`
			InstId  string `json:"instId"`
			Side    string `json:"side"`    // buy/sell
			PosSide string `json:"posSide"` // long/short/net
			OrdType string `json:"ordType"` // limit/market/post_only
			Px      string `json:"px"`      // price
			Sz      string `json:"sz"`      // size
			State   string `json:"state"`   // live/partially_filled
		}
		if err := json.Unmarshal(data, &orders); err == nil {
			for _, order := range orders {
				price, _ := strconv.ParseFloat(order.Px, 64)
				quantity, _ := strconv.ParseFloat(order.Sz, 64)

				// Convert OKX side to standard format
				side := strings.ToUpper(order.Side)
				positionSide := strings.ToUpper(order.PosSide)
				if positionSide == "NET" {
					positionSide = "BOTH"
				}

				result = append(result, types.OpenOrder{
					OrderID:      order.OrdId,
					Symbol:       symbol,
					Side:         side,
					PositionSide: positionSide,
					Type:         strings.ToUpper(order.OrdType),
					Price:        price,
					StopPrice:    0,
					Quantity:     quantity,
					Status:       "NEW",
				})
			}
		}
	}

	// 2. Get pending algo orders (stop-loss/take-profit)
	// OKX requires ordType parameter for algo orders API
	algoPath := fmt.Sprintf("%s?instId=%s&instType=SWAP&ordType=conditional", okxAlgoPendingPath, instId)
	algoData, err := t.doRequest("GET", algoPath, nil)
	if err != nil {
		logger.Warnf("[OKX] Failed to get algo orders: %v", err)
	}
	if err == nil && algoData != nil {
		var algoOrders []struct {
			AlgoId      string `json:"algoId"`
			InstId      string `json:"instId"`
			Side        string `json:"side"`
			PosSide     string `json:"posSide"`
			OrdType     string `json:"ordType"` // conditional/oco/trigger
			TriggerPx   string `json:"triggerPx"`
			SlTriggerPx string `json:"slTriggerPx"` // Stop loss trigger price
			TpTriggerPx string `json:"tpTriggerPx"` // Take profit trigger price
			Sz          string `json:"sz"`
			State       string `json:"state"`
		}
		if err := json.Unmarshal(algoData, &algoOrders); err == nil {
			for _, order := range algoOrders {
				quantity, _ := strconv.ParseFloat(order.Sz, 64)

				side := strings.ToUpper(order.Side)
				positionSide := strings.ToUpper(order.PosSide)
				if positionSide == "NET" {
					positionSide = "BOTH"
				}

				// Check for stop loss order (slTriggerPx is set)
				if order.SlTriggerPx != "" {
					slPrice, _ := strconv.ParseFloat(order.SlTriggerPx, 64)
					if slPrice > 0 {
						result = append(result, types.OpenOrder{
							OrderID:      order.AlgoId + "_sl",
							Symbol:       symbol,
							Side:         side,
							PositionSide: positionSide,
							Type:         "STOP_MARKET",
							Price:        0,
							StopPrice:    slPrice,
							Quantity:     quantity,
							Status:       "NEW",
						})
					}
				}

				// Check for take profit order (tpTriggerPx is set)
				if order.TpTriggerPx != "" {
					tpPrice, _ := strconv.ParseFloat(order.TpTriggerPx, 64)
					if tpPrice > 0 {
						result = append(result, types.OpenOrder{
							OrderID:      order.AlgoId + "_tp",
							Symbol:       symbol,
							Side:         side,
							PositionSide: positionSide,
							Type:         "TAKE_PROFIT_MARKET",
							Price:        0,
							StopPrice:    tpPrice,
							Quantity:     quantity,
							Status:       "NEW",
						})
					}
				}

				// Fallback for trigger orders (triggerPx is set)
				if order.TriggerPx != "" && order.SlTriggerPx == "" && order.TpTriggerPx == "" {
					triggerPrice, _ := strconv.ParseFloat(order.TriggerPx, 64)
					if triggerPrice > 0 {
						result = append(result, types.OpenOrder{
							OrderID:      order.AlgoId,
							Symbol:       symbol,
							Side:         side,
							PositionSide: positionSide,
							Type:         "STOP_MARKET",
							Price:        0,
							StopPrice:    triggerPrice,
							Quantity:     quantity,
							Status:       "NEW",
						})
					}
				}
			}
		}
	}

	logger.Infof("✓ OKX GetOpenOrders: found %d open orders for %s", len(result), symbol)
	return result, nil
}

// PlaceLimitOrder places a limit order for grid trading
// Implements GridTrader interface
func (t *OKXTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	instId := t.convertSymbol(req.Symbol)

	// Get instrument info
	inst, err := t.getInstrument(req.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Set leverage if specified
	if req.Leverage > 0 {
		if err := t.SetLeverage(req.Symbol, req.Leverage); err != nil {
			logger.Warnf("[OKX] Failed to set leverage: %v", err)
		}
	}

	// Convert quantity to contract size
	sz := req.Quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// Determine side and position side
	side := "buy"
	posSide := "long"
	if req.Side == "SELL" {
		side = "sell"
		posSide = "short"
	}

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    side,
		"posSide": posSide,
		"ordType": "limit",
		"sz":      szStr,
		"px":      fmt.Sprintf("%.8f", req.Price),
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	// Add reduce only if specified
	if req.ReduceOnly {
		body["reduceOnly"] = true
	}

	logger.Infof("[OKX] PlaceLimitOrder: %s %s @ %.4f, sz=%s", instId, side, req.Price, szStr)

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	if len(orders) == 0 {
		return nil, fmt.Errorf("empty order response")
	}

	if orders[0].SCode != "0" {
		return nil, fmt.Errorf("OKX order failed: %s", orders[0].SMsg)
	}

	logger.Infof("✓ [OKX] Limit order placed: %s %s @ %.4f, orderID=%s",
		instId, side, req.Price, orders[0].OrdId)

	return &types.LimitOrderResult{
		OrderID:      orders[0].OrdId,
		ClientID:     orders[0].ClOrdId,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Price:        req.Price,
		Quantity:     req.Quantity,
		Status:       "NEW",
	}, nil
}

// CancelOrder cancels a specific order by ID
// Implements GridTrader interface
func (t *OKXTrader) CancelOrder(symbol, orderID string) error {
	instId := t.convertSymbol(symbol)

	body := map[string]interface{}{
		"instId": instId,
		"ordId":  orderID,
	}

	_, err := t.doRequest("POST", "/api/v5/trade/cancel-order", body)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	logger.Infof("✓ [OKX] Order cancelled: %s %s", symbol, orderID)
	return nil
}

// GetOrderBook gets the order book for a symbol
// Implements GridTrader interface
func (t *OKXTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("/api/v5/market/books?instId=%s&sz=%d", instId, depth)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}

	var result []struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	if len(result) == 0 {
		return nil, nil, nil
	}

	// Parse bids
	for _, b := range result[0].Bids {
		if len(b) >= 2 {
			price, _ := strconv.ParseFloat(b[0], 64)
			qty, _ := strconv.ParseFloat(b[1], 64)
			bids = append(bids, []float64{price, qty})
		}
	}

	// Parse asks
	for _, a := range result[0].Asks {
		if len(a) >= 2 {
			price, _ := strconv.ParseFloat(a[0], 64)
			qty, _ := strconv.ParseFloat(a[1], 64)
			asks = append(asks, []float64{price, qty})
		}
	}

	return bids, asks, nil
}
