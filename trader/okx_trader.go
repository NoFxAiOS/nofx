package trader

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OKXTrader OKX USDT 永續合約交易器
type OKXTrader struct {
	apiKey     string
	secretKey  string
	passphrase string
	baseURL    string
	httpClient *http.Client
	testnet    bool

	// 餘額緩存
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// 持倉緩存
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// 緩存有效期（15秒）
	cacheDuration time.Duration
}

// NewOKXTrader 創建 OKX 交易器
func NewOKXTrader(apiKey, secretKey, passphrase string, testnet bool) *OKXTrader {
	baseURL := "https://www.okx.com"

	trader := &OKXTrader{
		apiKey:        apiKey,
		secretKey:     secretKey,
		passphrase:    passphrase,
		baseURL:       baseURL,
		testnet:       testnet,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		cacheDuration: 15 * time.Second,
	}

	log.Printf("🟠 [OKX] 交易器已初始化 (testnet=%v)", testnet)
	return trader
}

// sign 生成 OKX API v5 簽名
// 簽名算法：Base64(HMAC-SHA256(timestamp + method + requestPath + body, SecretKey))
func (t *OKXTrader) sign(timestamp, method, requestPath, body string) string {
	// 構建待簽名字符串：timestamp + method + requestPath + body
	message := timestamp + method + requestPath + body

	// HMAC-SHA256 簽名
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return signature
}

// request 發送 HTTP 請求到 OKX API
func (t *OKXTrader) request(method, path string, params map[string]interface{}) (map[string]interface{}, error) {
	// 生成 ISO 8601 時間戳（含毫秒）
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// 構建請求體
	var bodyBytes []byte
	var bodyStr string
	if params != nil && len(params) > 0 {
		var err error
		bodyBytes, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("序列化請求體失敗: %w", err)
		}
		bodyStr = string(bodyBytes)
	} else {
		bodyStr = ""
	}

	// 構建完整 URL
	url := t.baseURL + path

	// 生成簽名
	signature := t.sign(timestamp, method, path, bodyStr)

	// 創建請求
	var req *http.Request
	var err error
	if bodyStr != "" {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(bodyBytes))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("創建請求失敗: %w", err)
	}

	// 設置請求頭
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", t.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", t.passphrase)

	// Demo 交易模式
	if t.testnet {
		req.Header.Set("x-simulated-trading", "1")
	}

	// 發送請求
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("發送請求失敗: %w", err)
	}
	defer resp.Body.Close()

	// 讀取響應
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	// 解析 JSON
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w, body: %s", err, string(respBody))
	}

	// 檢查錯誤
	if code, ok := result["code"].(string); ok && code != "0" {
		msg := result["msg"].(string)
		return nil, fmt.Errorf("OKX API 錯誤 [%s]: %s", code, msg)
	}

	return result, nil
}

// GetBalance 獲取賬戶餘額
func (t *OKXTrader) GetBalance() (map[string]interface{}, error) {
	// 檢查緩存
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		balance := t.cachedBalance
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ 使用緩存的賬戶餘額（緩存時間: %.1f秒前）", time.Since(t.balanceCacheTime).Seconds())
		return balance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// 調用 API：GET /api/v5/account/balance
	log.Printf("🔄 緩存過期，正在調用 OKX API 獲取賬戶餘額...")
	result, err := t.request("GET", "/api/v5/account/balance", nil)
	if err != nil {
		return nil, fmt.Errorf("獲取 OKX 餘額失敗: %w", err)
	}

	// 解析響應
	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("OKX API 返回數據格式錯誤")
	}

	accountData := data[0].(map[string]interface{})
	details := accountData["details"].([]interface{})

	// 計算 USDT 餘額
	var totalEq, availEq, upl float64
	for _, detail := range details {
		d := detail.(map[string]interface{})
		if d["ccy"].(string) == "USDT" {
			totalEq, _ = strconv.ParseFloat(d["eq"].(string), 64)
			availEq, _ = strconv.ParseFloat(d["availEq"].(string), 64)
			uplStr, ok := d["upl"].(string)
			if ok {
				upl, _ = strconv.ParseFloat(uplStr, 64)
			}
			break
		}
	}

	balance := map[string]interface{}{
		"totalWalletBalance":     totalEq,
		"availableBalance":       availEq,
		"totalUnrealizedProfit":  upl,
		"wallet_balance":         totalEq,
		"available_balance":      availEq,
		"unrealized_profit":      upl,
		"balance":                totalEq,
	}

	// 更新緩存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = balance
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	log.Printf("✓ OKX API 返回: 總餘額=%.2f, 可用=%.2f, 未實現盈虧=%.2f",
		totalEq, availEq, upl)

	return balance, nil
}

// GetPositions 獲取所有持倉
func (t *OKXTrader) GetPositions() ([]map[string]interface{}, error) {
	// 檢查緩存
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		positions := t.cachedPositions
		t.positionsCacheMutex.RUnlock()
		return positions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// 調用 API：GET /api/v5/account/positions
	result, err := t.request("GET", "/api/v5/account/positions", nil)
	if err != nil {
		return nil, fmt.Errorf("獲取 OKX 持倉失敗: %w", err)
	}

	// 解析響應
	data, ok := result["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("OKX API 返回數據格式錯誤")
	}

	positions := make([]map[string]interface{}, 0)
	for _, item := range data {
		pos := item.(map[string]interface{})

		// 跳過空倉位
		posStr := pos["pos"].(string)
		if posStr == "0" {
			continue
		}

		// 解析數據
		quantity, _ := strconv.ParseFloat(posStr, 64)
		entryPrice, _ := strconv.ParseFloat(pos["avgPx"].(string), 64)
		markPrice, _ := strconv.ParseFloat(pos["markPx"].(string), 64)
		upl, _ := strconv.ParseFloat(pos["upl"].(string), 64)
		leverage, _ := strconv.ParseFloat(pos["lever"].(string), 64)
		liqPx, _ := strconv.ParseFloat(pos["liqPx"].(string), 64)

		// 計算保證金
		notionalUsd, _ := strconv.ParseFloat(pos["notionalUsd"].(string), 64)
		marginUsed := notionalUsd / leverage

		// 計算盈虧百分比
		uplPct := 0.0
		if entryPrice > 0 {
			uplPct = (upl / (quantity * entryPrice)) * 100
		}

		// 處理方向
		side := "long"
		if pos["posSide"].(string) == "short" {
			side = "short"
			quantity = -quantity // 空倉顯示負數
		}

		// 標準化 symbol：BTC-USDT-SWAP → BTCUSDT
		instId := pos["instId"].(string)
		symbol := strings.ReplaceAll(strings.ReplaceAll(instId, "-USDT-SWAP", ""), "-", "")

		position := map[string]interface{}{
			"symbol":            symbol,
			"side":              side,
			"entry_price":       entryPrice,
			"mark_price":        markPrice,
			"quantity":          quantity,
			"leverage":          int(leverage),
			"unrealized_pnl":    upl,
			"unrealized_pnl_pct": uplPct,
			"liquidation_price": liqPx,
			"margin_used":       marginUsed,
		}

		positions = append(positions, position)
	}

	// 更新緩存
	t.positionsCacheMutex.Lock()
	t.cachedPositions = positions
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return positions, nil
}

// formatSymbol 將 symbol 轉換為 OKX 格式
// BTCUSDT → BTC-USDT-SWAP
func (t *OKXTrader) formatSymbol(symbol string) string {
	// 移除 USDT 後綴，然後加上 -USDT-SWAP
	base := strings.TrimSuffix(strings.ToUpper(symbol), "USDT")
	return base + "-USDT-SWAP"
}

// OpenLong 開多倉
func (t *OKXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.placeOrder(symbol, "buy", "long", quantity, leverage)
}

// OpenShort 開空倉
func (t *OKXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.placeOrder(symbol, "sell", "short", quantity, leverage)
}

// CloseLong 平多倉
func (t *OKXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.placeOrder(symbol, "sell", "long", quantity, 0)
}

// CloseShort 平空倉
func (t *OKXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.placeOrder(symbol, "buy", "short", quantity, 0)
}

// placeOrder 下單核心邏輯
func (t *OKXTrader) placeOrder(symbol, side, posSide string, quantity float64, leverage int) (map[string]interface{}, error) {
	instId := t.formatSymbol(symbol)

	// 如果指定了槓桿，先設置槓桿
	if leverage > 0 {
		if err := t.SetLeverage(symbol, leverage); err != nil {
			log.Printf("⚠️ 設置槓桿失敗: %v", err)
		}
	}

	// 構建訂單參數
	params := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",                // 全倉模式
		"side":    side,                   // buy/sell
		"posSide": posSide,                // long/short
		"ordType": "market",               // 市價單
		"sz":      fmt.Sprintf("%f", quantity),
	}

	log.Printf("🟠 [OKX] 下單: %s %s %s, 數量=%.4f", instId, side, posSide, quantity)

	// 調用 API：POST /api/v5/trade/order
	result, err := t.request("POST", "/api/v5/trade/order", params)
	if err != nil {
		return nil, fmt.Errorf("OKX 下單失敗: %w", err)
	}

	// 清除緩存
	t.clearCache()

	return result, nil
}

// SetLeverage 設置槓桿
func (t *OKXTrader) SetLeverage(symbol string, leverage int) error {
	instId := t.formatSymbol(symbol)

	params := map[string]interface{}{
		"instId":  instId,
		"lever":   strconv.Itoa(leverage),
		"mgnMode": "cross", // 全倉模式
	}

	log.Printf("🟠 [OKX] 設置槓桿: %s, 槓桿=%d", instId, leverage)

	_, err := t.request("POST", "/api/v5/account/set-leverage", params)
	if err != nil {
		// OKX 如果槓桿已經是目標值會返回錯誤，但可以忽略
		if strings.Contains(err.Error(), "Leverage not modified") {
			log.Printf("  ✓ 槓桿已是目標值")
			return nil
		}
		return fmt.Errorf("設置槓桿失敗: %w", err)
	}

	log.Printf("  ✓ 槓桿設置成功")
	return nil
}

// SetMarginMode 設置倉位模式（全倉/逐倉）
func (t *OKXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// OKX 的保證金模式在下單時指定（tdMode: cross/isolated）
	// 這裡僅記錄日誌
	mode := "isolated"
	if isCrossMargin {
		mode = "cross"
	}
	log.Printf("🟠 [OKX] 保證金模式: %s (在下單時指定)", mode)
	return nil
}

// GetMarketPrice 獲取市場價格
func (t *OKXTrader) GetMarketPrice(symbol string) (float64, error) {
	instId := t.formatSymbol(symbol)

	// 調用 API：GET /api/v5/market/ticker?instId=BTC-USDT-SWAP
	path := fmt.Sprintf("/api/v5/market/ticker?instId=%s", instId)
	result, err := t.request("GET", path, nil)
	if err != nil {
		return 0, fmt.Errorf("獲取市場價格失敗: %w", err)
	}

	// 解析響應
	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		return 0, fmt.Errorf("OKX API 返回數據格式錯誤")
	}

	ticker := data[0].(map[string]interface{})
	priceStr := ticker["last"].(string)
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("解析價格失敗: %w", err)
	}

	return price, nil
}

// SetStopLoss 設置止損單
func (t *OKXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	log.Printf("🟠 [OKX] 設置止損: %s %s, 止損價=%.2f", symbol, positionSide, stopPrice)
	// TODO: 實現止損邏輯
	return fmt.Errorf("OKX 止損功能尚未實現")
}

// SetTakeProfit 設置止盈單
func (t *OKXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	log.Printf("🟠 [OKX] 設置止盈: %s %s, 止盈價=%.2f", symbol, positionSide, takeProfitPrice)
	// TODO: 實現止盈邏輯
	return fmt.Errorf("OKX 止盈功能尚未實現")
}

// CancelStopLossOrders 取消止損單
func (t *OKXTrader) CancelStopLossOrders(symbol string) error {
	log.Printf("🟠 [OKX] 取消止損單: %s", symbol)
	// TODO: 實現取消止損邏輯
	return nil
}

// CancelTakeProfitOrders 取消止盈單
func (t *OKXTrader) CancelTakeProfitOrders(symbol string) error {
	log.Printf("🟠 [OKX] 取消止盈單: %s", symbol)
	// TODO: 實現取消止盈邏輯
	return nil
}

// CancelAllOrders 取消所有掛單
func (t *OKXTrader) CancelAllOrders(symbol string) error {
	instId := t.formatSymbol(symbol)
	log.Printf("🟠 [OKX] 取消所有掛單: %s", instId)
	// TODO: 實現取消所有訂單邏輯
	return nil
}

// CancelStopOrders 取消止盈止損單
func (t *OKXTrader) CancelStopOrders(symbol string) error {
	log.Printf("🟠 [OKX] 取消止盈止損單: %s", symbol)
	// TODO: 實現取消止盈止損邏輯
	return nil
}

// FormatQuantity 格式化數量到正確的精度
func (t *OKXTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// OKX 通常使用合約數量（contracts），不同幣種精度不同
	// 這裡暫時返回標準格式
	return fmt.Sprintf("%.4f", quantity), nil
}

// clearCache 清除緩存
func (t *OKXTrader) clearCache() {
	t.balanceCacheMutex.Lock()
	t.cachedBalance = nil
	t.balanceCacheMutex.Unlock()

	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheMutex.Unlock()
}
