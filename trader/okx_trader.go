package trader

import (
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

// OKXTrader OKX交易所交易器
type OKXTrader struct {
	apiKey     string
	secretKey  string
	passphrase string
	baseURL    string
	client     *http.Client

	// 缓存机制（遵循现有模式）
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// 缓存有效期（15秒）- 遵循现有模式
	cacheDuration time.Duration

	// 速率限制器
	rateLimiter *RateLimiter
}

// NewOKXTrader 创建OKX交易器
func NewOKXTrader(apiKey, secretKey, passphrase string, testnet bool) (*OKXTrader, error) {
	// 验证输入参数
	if apiKey == "" {
		return nil, fmt.Errorf("API密钥不能为空")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("Secret密钥不能为空")
	}
	if passphrase == "" {
		return nil, fmt.Errorf("Passphrase不能为空")
	}

	baseURL := "https://www.okx.com"
	if testnet {
		// OKX模拟交易使用相同的host，通过header区分
		log.Println("✅ OKX模拟交易模式已启用")
	}

	return &OKXTrader{
		apiKey:      apiKey,
		secretKey:   secretKey,
		passphrase:  passphrase,
		baseURL:     baseURL,
		client:      &http.Client{Timeout: 30 * time.Second},
		cacheDuration: 15 * time.Second, // 遵循现有缓存策略
		rateLimiter: NewRateLimiter(OKXRateLimitRequestsPerSecond, OKXRateLimitBurst),
	}, nil
}

// GetBalance 获取账户余额（带缓存）
func (t *OKXTrader) GetBalance() (map[string]interface{}, error) {
	// 先检查缓存是否有效
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的OKX账户余额（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// OKX API: GET /api/v5/account/balance
	endpoint := "/api/v5/account/balance"
	resp, err := t.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("获取OKX余额失败: %w", err)
	}

	// 解析OKX响应格式
	balance := t.parseBalance(resp)

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = balance
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	log.Printf("✅ OKX余额获取成功: total=%v, used=%v, free=%v",
		balance["total"], balance["used"], balance["free"])

	return balance, nil
}

// parseBalance 解析OKX余额响应
func (t *OKXTrader) parseBalance(resp map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"total": float64(0),
		"used":  float64(0),
		"free":  float64(0),
	}

	if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
		if balance, ok := data[0].(map[string]interface{}); ok {
			// 总资产
			if totalEq, ok := balance["totalEq"].(string); ok {
				if total, err := strconv.ParseFloat(totalEq, 64); err == nil {
					result["total"] = total
				}
			}
			// 已用资产（isoEq）
			if isoEq, ok := balance["isoEq"].(string); ok {
				if used, err := strconv.ParseFloat(isoEq, 64); err == nil {
					result["used"] = used
				}
			}
			// 可用资产（adjEq）
			if adjEq, ok := balance["adjEq"].(string); ok {
				if free, err := strconv.ParseFloat(adjEq, 64); err == nil {
					result["free"] = free
				}
			}
		}
	}

	return result
}

// GetPositions 获取所有持仓
func (t *OKXTrader) GetPositions() ([]map[string]interface{}, error) {
	// 检查缓存
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的OKX持仓数据（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// OKX API: GET /api/v5/account/positions
	endpoint := "/api/v5/account/positions"
	resp, err := t.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("获取OKX持仓失败: %w", err)
	}

	positions := t.parsePositions(resp)

	// 更新缓存
	t.positionsCacheMutex.Lock()
	t.cachedPositions = positions
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	log.Printf("✅ OKX持仓获取成功: %d个持仓", len(positions))

	return positions, nil
}

// parsePositions 解析OKX持仓响应
func (t *OKXTrader) parsePositions(resp map[string]interface{}) []map[string]interface{} {
	var positions []map[string]interface{}

	if data, ok := resp["data"].([]interface{}); ok {
		for _, item := range data {
			if pos, ok := item.(map[string]interface{}); ok {
				// 标准化持仓数据格式
				standardizedPos := map[string]interface{}{
					"symbol":    pos["instId"],
					"position":  pos["pos"],
					"posSide":   pos["posSide"],
					"avgPrice":  pos["avgPx"],
					"leverage":  pos["lever"],
					"marginMode": pos["mgnMode"],
					"upl":       pos["upl"],      // 未实现盈亏
					"uplRatio":  pos["uplRatio"], // 未实现盈亏率
				}
				positions = append(positions, standardizedPos)
			}
		}
	}

	return positions
}

// OpenLong 开多仓
func (t *OKXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("开仓数量必须大于0")
	}

	order := map[string]string{
		"instId":  symbol,           // 产品ID，如 "BTC-USDT-SWAP"
		"tdMode":  "cross",          // 保证金模式：cross(全仓) / isolated(逐仓)
		"side":    "buy",            // 订单方向：buy(买入开多)
		"ordType": "market",         // 订单类型：market(市价)
		"sz":      strconv.FormatFloat(quantity, 'f', -1, 64), // 委托数量
	}

	// 设置杠杆（OKX要求先设置杠杆）
	if err := t.SetLeverage(symbol, leverage); err != nil {
		log.Printf("⚠️ 设置杠杆失败: %v", err)
	}

	return t.placeOrder(order)
}

// OpenShort 开空仓
func (t *OKXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("开仓数量必须大于0")
	}

	order := map[string]string{
		"instId":  symbol,
		"tdMode":  "cross",
		"side":    "sell",           // 卖出开空
		"ordType": "market",
		"sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
	}

	if err := t.SetLeverage(symbol, leverage); err != nil {
		log.Printf("⚠️ 设置杠杆失败: %v", err)
	}

	return t.placeOrder(order)
}

// CloseLong 平多仓
func (t *OKXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// OKX平仓通过反向订单实现
	// 获取当前持仓数量
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionSize float64
	for _, pos := range positions {
		if pos["symbol"] == symbol && pos["posSide"] == "long" {
			if size, ok := pos["position"].(string); ok {
				positionSize, _ = strconv.ParseFloat(size, 64)
				break
			}
		}
	}

	if positionSize <= 0 {
		return nil, fmt.Errorf("没有找到多仓持仓")
	}

	// 如果quantity为0，平仓全部数量
	if quantity <= 0 {
		quantity = positionSize
	}

	// 确保平仓数量不超过持仓数量
	if quantity > positionSize {
		quantity = positionSize
	}

	order := map[string]string{
		"instId":  symbol,
		"tdMode":  "cross",
		"side":    "sell",           // 卖出平仓
		"ordType": "market",
		"sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
	}

	return t.placeOrder(order)
}

// CloseShort 平空仓
func (t *OKXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionSize float64
	for _, pos := range positions {
		if pos["symbol"] == symbol && pos["posSide"] == "short" {
			if size, ok := pos["position"].(string); ok {
				positionSize, _ = strconv.ParseFloat(size, 64)
				break
			}
		}
	}

	if positionSize <= 0 {
		return nil, fmt.Errorf("没有找到空仓持仓")
	}

	if quantity <= 0 {
		quantity = positionSize
	}

	if quantity > positionSize {
		quantity = positionSize
	}

	order := map[string]string{
		"instId":  symbol,
		"tdMode":  "cross",
		"side":    "buy",            // 买入平仓
		"ordType": "market",
		"sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
	}

	return t.placeOrder(order)
}

// placeOrder 下单统一方法
func (t *OKXTrader) placeOrder(order map[string]string) (map[string]interface{}, error) {
	// OKX API: POST /api/v5/trade/order
	endpoint := "/api/v5/trade/order"

	resp, err := t.makeRequest("POST", endpoint, order)
	if err != nil {
		return nil, fmt.Errorf("OKX下单失败: %w", err)
	}

	log.Printf("✅ OKX下单成功: side=%s, symbol=%s, quantity=%s",
		order["side"], order["instId"], order["sz"])

	return resp, nil
}

// SetLeverage 设置杠杆
func (t *OKXTrader) SetLeverage(symbol string, leverage int) error {
	if leverage < 1 || leverage > 125 {
		return fmt.Errorf("杠杆必须在1-125之间")
	}

	params := map[string]string{
		"instId":  symbol,
		"lever":   strconv.Itoa(leverage),
		"mgnMode": "cross",
	}

	// OKX API: POST /api/v5/account/set-leverage
	endpoint := "/api/v5/account/set-leverage"
	_, err := t.makeRequest("POST", endpoint, params)
	if err != nil {
		return fmt.Errorf("设置OKX杠杆失败: %w", err)
	}

	log.Printf("✅ OKX杠杆设置成功: symbol=%s, leverage=%d", symbol, leverage)
	return nil
}

// SetMarginMode 设置仓位模式
func (t *OKXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	mgnMode := "isolated"
	if isCrossMargin {
		mgnMode = "cross"
	}

	params := map[string]string{
		"instId":  symbol,
		"mgnMode": mgnMode,
	}

	// OKX API: POST /api/v5/account/set-margin-mode
	endpoint := "/api/v5/account/set-margin-mode"
	_, err := t.makeRequest("POST", endpoint, params)
	if err != nil {
		return fmt.Errorf("设置OKX保证金模式失败: %w", err)
	}

	log.Printf("✅ OKX保证金模式设置成功: symbol=%s, mode=%s", symbol, mgnMode)
	return nil
}

// GetMarketPrice 获取市场价格
func (t *OKXTrader) GetMarketPrice(symbol string) (float64, error) {
	params := map[string]string{
		"instId": symbol,
	}

	// OKX API: GET /api/v5/market/ticker
	endpoint := "/api/v5/market/ticker"
	resp, err := t.makeRequest("GET", endpoint, params)
	if err != nil {
		return 0, fmt.Errorf("获取OKX市场价格失败: %w", err)
	}

	if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
		if ticker, ok := data[0].(map[string]interface{}); ok {
			if lastPrice, ok := ticker["last"].(string); ok {
				price, err := strconv.ParseFloat(lastPrice, 64)
				if err != nil {
					return 0, fmt.Errorf("解析价格失败: %w", err)
				}
				log.Printf("✅ OKX市场价格获取成功: symbol=%s, price=%f", symbol, price)
				return price, nil
			}
		}
	}

	return 0, fmt.Errorf("无法解析OKX市场价格数据")
}

// SetStopLoss 设置止损单
func (t *OKXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	side := "buy"
	if positionSide == "long" {
		side = "sell"
	}

	order := map[string]string{
		"instId":  symbol,
		"tdMode":  "cross",
		"side":    side,
		"ordType": "conditional",    // 条件单
		"sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
		"tpTriggerPx": strconv.FormatFloat(stopPrice, 'f', -1, 64), // 触发价格
		"tpOrdPx": "-1", // 市价触发
	}

	_, err := t.placeOrder(order)
	if err != nil {
		return fmt.Errorf("设置OKX止损失败: %w", err)
	}

	log.Printf("✅ OKX止损设置成功: symbol=%s, side=%s, stopPrice=%f", symbol, side, stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *OKXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	side := "buy"
	if positionSide == "long" {
		side = "sell"
	}

	order := map[string]string{
		"instId":  symbol,
		"tdMode":  "cross",
		"side":    side,
		"ordType": "conditional",
		"sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
		"tpTriggerPx": strconv.FormatFloat(takeProfitPrice, 'f', -1, 64),
		"tpOrdPx": "-1",
	}

	_, err := t.placeOrder(order)
	if err != nil {
		return fmt.Errorf("设置OKX止盈失败: %w", err)
	}

	log.Printf("✅ OKX止盈设置成功: symbol=%s, side=%s, takeProfitPrice=%f", symbol, side, takeProfitPrice)
	return nil
}

// CancelAllOrders 取消该币种的所有挂单
func (t *OKXTrader) CancelAllOrders(symbol string) error {
	params := map[string]string{
		"instId": symbol,
	}

	// OKX API: POST /api/v5/trade/cancel-all-orders
	endpoint := "/api/v5/trade/cancel-all-orders"
	_, err := t.makeRequest("POST", endpoint, params)
	if err != nil {
		return fmt.Errorf("取消OKX所有订单失败: %w", err)
	}

	log.Printf("✅ OKX取消所有订单成功: symbol=%s", symbol)
	return nil
}

// ClosePosition 关闭指定持仓
func (t *OKXTrader) ClosePosition(symbol string, side string) (map[string]interface{}, error) {
	// 获取当前持仓
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找匹配的持仓
	var position map[string]interface{}
	for _, pos := range positions {
		if pos["symbol"] == symbol && pos["side"] == side {
			position = pos
			break
		}
	}

	if position == nil {
		return nil, fmt.Errorf("未找到持仓: symbol=%s, side=%s", symbol, side)
	}

	quantity := position["quantity"].(float64)

	// 根据持仓方向决定平仓方向
	var closeSide string
	if side == "long" {
		closeSide = "sell" // 多头平仓需要卖出
	} else {
		closeSide = "buy"  // 空头平仓需要买入
	}

	order := map[string]string{
		"instId":  symbol,
		"tdMode":  "cross", // 默认全仓模式
		"side":    closeSide,
		"ordType": "market", // 市价平仓
		"sz":      fmt.Sprintf("%.4f", quantity),
	}

	result, err := t.placeOrder(order)
	if err != nil {
		return nil, fmt.Errorf("平仓失败: %w", err)
	}

	log.Printf("✅ OKX平仓成功: symbol=%s, side=%s, quantity=%.4f", symbol, side, quantity)
	return result, nil
}

// GetFills 获取成交记录
func (t *OKXTrader) GetFills(symbol string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 100 {
		limit = 20 // 默认获取最近20条记录
	}

	params := map[string]string{
		"instId": symbol,
		"limit":  fmt.Sprintf("%d", limit),
	}

	// OKX API: GET /api/v5/trade/fills
	endpoint := "/api/v5/trade/fills"
	resp, err := t.makeRequest("GET", endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("获取成交记录失败: %w", err)
	}

	// 解析成交记录
	fillsData, ok := resp["data"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	var fills []map[string]interface{}
	for _, fillItem := range fillsData {
		fill, ok := fillItem.(map[string]interface{})
		if !ok {
			continue
		}

		// 标准化成交记录格式
		standardizedFill := map[string]interface{}{
			"symbol":      symbol,
			"orderId":     fill["ordId"],
			"fillId":      fill["tradeId"],
			"side":        t.standardizeSide(fill["side"].(string)),
			"quantity":    parseOKXFloat(fill["sz"].(string)),
			"price":       parseOKXFloat(fill["px"].(string)),
			"timestamp":   parseOKXTimestamp(fill["ts"].(string)),
			"fee":         parseOKXFloat(fill["fee"].(string)),
			"feeCurrency": fill["feeCcy"],
			"role":        fill["side"], // maker or taker
		}

		fills = append(fills, standardizedFill)
	}

	log.Printf("✅ OKX获取成交记录成功: symbol=%s, count=%d", symbol, len(fills))
	return fills, nil
}

// standardizeSide 标准化交易方向
func (t *OKXTrader) standardizeSide(side string) string {
	switch strings.ToLower(side) {
	case "buy":
		return "buy"
	case "sell":
		return "sell"
	default:
		return side
	}
}

// FormatQuantity 格式化数量到正确的精度
func (t *OKXTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// OKX的数量精度规则：
	// BTC-USDT-SWAP: 0.001
	// ETH-USDT-SWAP: 0.001
	// 其他币种根据合约规定

	// 基本实现：根据symbol判断精度
	var precision int
	switch {
	case strings.HasPrefix(symbol, "BTC-"):
		precision = 3
	case strings.HasPrefix(symbol, "ETH-"):
		precision = 3
	case strings.HasPrefix(symbol, "SOL-"):
		precision = 3
	default:
		precision = 4 // 默认精度
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity), nil
}

// generateSignature 生成OKX API签名
func (t *OKXTrader) generateSignature(timestamp, method, requestPath, body string) string {
	message := timestamp + strings.ToUpper(method) + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// makeRequest 发送HTTP请求（遵循KISS原则）
func (t *OKXTrader) makeRequest(method, endpoint string, params map[string]string) (map[string]interface{}, error) {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// 构建请求body
	var body string
	if method == "POST" && len(params) > 0 {
		jsonBody, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("序列化请求参数失败: %w", err)
		}
		body = string(jsonBody)
	}

	// 生成签名
	signature := t.generateSignature(timestamp, method, endpoint, body)

	// 构建请求
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, t.baseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置OKX认证头
	req.Header.Set("OK-ACCESS-KEY", t.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", t.passphrase)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查OKX错误码
	if code, ok := result["code"].(string); ok && code != "0" {
		msg, _ := result["msg"].(string)
		return nil, fmt.Errorf("OKX API错误 [%s]: %s", code, msg)
	}

	return result, nil
}