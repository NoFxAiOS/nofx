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

// OKXFuturesTrader OKX合约交易器
type OKXFuturesTrader struct {
	apiKey     string
	secretKey  string
	passphrase string
	baseURL    string
	client     *http.Client

	// 余额缓存
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// 持仓缓存
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// 缓存有效期（15秒）
	cacheDuration time.Duration
}

// OKX API响应格式
type okxResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// NewOKXFuturesTrader 创建OKX合约交易器
func NewOKXFuturesTrader(apiKey, secretKey, passphrase string, testnet bool) *OKXFuturesTrader {
	baseURL := "https://www.okx.com"
	if testnet {
		baseURL = "https://www.okx.com" // OKX不提供公开测试网，使用模拟交易需要在UI设置
		log.Printf("⚠️ OKX模拟交易需要在账户设置中启用")
	}

	trader := &OKXFuturesTrader{
		apiKey:        apiKey,
		secretKey:     secretKey,
		passphrase:    passphrase,
		baseURL:       baseURL,
		client:        &http.Client{Timeout: 30 * time.Second},
		cacheDuration: 15 * time.Second,
	}

	log.Printf("🏦 OKX合约交易器已初始化")
	return trader
}

// sign 生成OKX API签名
func (t *OKXFuturesTrader) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// request 发送HTTP请求到OKX
func (t *OKXFuturesTrader) request(method, path string, body interface{}) ([]byte, error) {
	var bodyStr string
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyStr = string(bodyBytes)
	}

	url := t.baseURL + path
	req, err := http.NewRequest(method, url, strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}

	// OKX需要的时间戳格式: ISO 8601 (2024-01-01T00:00:00.000Z)
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.999Z")
	signature := t.sign(timestamp, method, path, bodyStr)

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", t.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", t.passphrase)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OKX API错误: HTTP %d, Body: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var okxResp okxResponse
	if err := json.Unmarshal(respBody, &okxResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if okxResp.Code != "0" {
		return nil, fmt.Errorf("OKX API错误: %s - %s", okxResp.Code, okxResp.Msg)
	}

	return []byte(okxResp.Data), nil
}

// GetBalance 获取账户余额（带缓存）
func (t *OKXFuturesTrader) GetBalance() (map[string]interface{}, error) {
	// 检查缓存
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的账户余额（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// 缓存过期，调用API
	log.Printf("🔄 缓存过期，正在调用OKX API获取账户余额...")

	// GET /api/v5/account/balance?ccy=USDT
	data, err := t.request("GET", "/api/v5/account/balance?ccy=USDT", nil)
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 解析余额数据
	var balanceData []struct {
		TotalEq  string `json:"totalEq"`  // 总权益
		Details  []struct {
			AvailBal string `json:"availBal"` // 可用余额
			Ccy      string `json:"ccy"`      // 币种
		} `json:"details"`
	}

	if err := json.Unmarshal(data, &balanceData); err != nil {
		return nil, fmt.Errorf("解析余额数据失败: %w", err)
	}

	if len(balanceData) == 0 {
		return nil, fmt.Errorf("未找到账户余额数据")
	}

	// 提取USDT余额
	totalEquity, _ := strconv.ParseFloat(balanceData[0].TotalEq, 64)
	availableBalance := 0.0

	for _, detail := range balanceData[0].Details {
		if detail.Ccy == "USDT" {
			availableBalance, _ = strconv.ParseFloat(detail.AvailBal, 64)
			break
		}
	}

	result := map[string]interface{}{
		"totalWalletBalance":   totalEquity,
		"availableBalance":     availableBalance,
		"totalUnrealizedProfit": 0.0, // 需要从持仓中计算
	}

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	log.Printf("✓ OKX API返回: 总权益=%.2f, 可用=%.2f", totalEquity, availableBalance)
	return result, nil
}

// GetPositions 获取所有持仓（带缓存）
func (t *OKXFuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	// 检查缓存
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的持仓信息（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// 缓存过期，调用API
	log.Printf("🔄 缓存过期，正在调用OKX API获取持仓信息...")

	// GET /api/v5/account/positions?instType=SWAP
	data, err := t.request("GET", "/api/v5/account/positions?instType=SWAP", nil)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	// 解析持仓数据
	var positions []struct {
		InstID    string `json:"instId"`    // 交易对 (如 BTC-USDT-SWAP)
		Pos       string `json:"pos"`       // 持仓数量（正数=多，负数=空）
		AvgPx     string `json:"avgPx"`     // 开仓均价
		MarkPx    string `json:"markPx"`    // 标记价格
		Upl       string `json:"upl"`       // 未实现盈亏
		Lever     string `json:"lever"`     // 杠杆倍数
		LiqPx     string `json:"liqPx"`     // 预估强平价
		PosSide   string `json:"posSide"`   // 持仓方向 (long/short/net)
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("解析持仓数据失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		posAmt, _ := strconv.ParseFloat(pos.Pos, 64)
		if posAmt == 0 {
			continue // 跳过无持仓的
		}

		// 将 BTC-USDT-SWAP 转换为 BTCUSDT 格式（兼容Binance格式）
		symbol := strings.Replace(pos.InstID, "-USDT-SWAP", "USDT", 1)

		posMap := map[string]interface{}{
			"symbol":            symbol,
			"positionAmt":       posAmt,
			"entryPrice":        parseFloat(pos.AvgPx),
			"markPrice":         parseFloat(pos.MarkPx),
			"unRealizedProfit":  parseFloat(pos.Upl),
			"leverage":          parseFloat(pos.Lever),
			"liquidationPrice":  parseFloat(pos.LiqPx),
		}

		// 判断方向
		if pos.PosSide == "long" || posAmt > 0 {
			posMap["side"] = "long"
		} else {
			posMap["side"] = "short"
		}

		result = append(result, posMap)
	}

	// 更新缓存
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// SetMarginMode 设置仓位模式
func (t *OKXFuturesTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// OKX设置杠杆模式: POST /api/v5/account/set-leverage
	// mgnMode: cross=全仓, isolated=逐仓
	mgnMode := "cross"
	if !isCrossMargin {
		mgnMode = "isolated"
	}

	instID := formatOKXSymbol(symbol)
	body := map[string]interface{}{
		"instId":  instID,
		"lever":   "1", // 临时设置杠杆为1
		"mgnMode": mgnMode,
	}

	_, err := t.request("POST", "/api/v5/account/set-leverage", body)
	if err != nil {
		// OKX可能返回错误，如果已经是目标模式则忽略
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "No need") {
			log.Printf("  ✓ %s 仓位模式已是 %s", symbol, map[bool]string{true: "全仓", false: "逐仓"}[isCrossMargin])
			return nil
		}
		return fmt.Errorf("设置仓位模式失败: %w", err)
	}

	log.Printf("  ✓ %s 仓位模式已设置为 %s", symbol, map[bool]string{true: "全仓", false: "逐仓"}[isCrossMargin])
	return nil
}

// SetLeverage 设置杠杆
func (t *OKXFuturesTrader) SetLeverage(symbol string, leverage int) error {
	instID := formatOKXSymbol(symbol)

	body := map[string]interface{}{
		"instId":  instID,
		"lever":   fmt.Sprintf("%d", leverage),
		"mgnMode": "cross", // 默认全仓，实际由SetMarginMode控制
	}

	_, err := t.request("POST", "/api/v5/account/set-leverage", body)
	if err != nil {
		if strings.Contains(err.Error(), "already") {
			log.Printf("  ✓ %s 杠杆已是 %dx", symbol, leverage)
			return nil
		}
		return fmt.Errorf("设置杠杆失败: %w", err)
	}

	log.Printf("  ✓ %s 杠杆已切换为 %dx", symbol, leverage)
	return nil
}

// OpenLong 开多仓
func (t *OKXFuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败: %v", err)
	}

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	instID := formatOKXSymbol(symbol)

	// POST /api/v5/trade/order
	body := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "cross",      // 交易模式: cross=全仓, isolated=逐仓
		"side":    "buy",        // 买入开多
		"ordType": "market",     // 市价单
		"sz":      quantityStr,  // 数量
		"posSide": "long",       // 持仓方向（双向持仓模式）
	}

	data, err := t.request("POST", "/api/v5/trade/order", body)
	if err != nil {
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}

	// 解析订单响应
	var orders []struct {
		OrdID string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "未知错误"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("开多仓失败: %s", msg)
	}

	log.Printf("✓ 开多仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %s", orders[0].OrdID)

	return map[string]interface{}{
		"orderId": orders[0].OrdID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort 开空仓
func (t *OKXFuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败: %v", err)
	}

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	instID := formatOKXSymbol(symbol)

	body := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "cross",
		"side":    "sell",       // 卖出开空
		"ordType": "market",
		"sz":      quantityStr,
		"posSide": "short",      // 持仓方向
	}

	data, err := t.request("POST", "/api/v5/trade/order", body)
	if err != nil {
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	var orders []struct {
		OrdID string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "未知错误"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("开空仓失败: %s", msg)
	}

	log.Printf("✓ 开空仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %s", orders[0].OrdID)

	return map[string]interface{}{
		"orderId": orders[0].OrdID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong 平多仓
func (t *OKXFuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				// 🔒 安全的类型断言，防止 panic
				if amt, ok := pos["positionAmt"].(float64); ok {
					quantity = amt
					break
				}
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的多仓", symbol)
		}
	}

	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	instID := formatOKXSymbol(symbol)

	body := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "cross",
		"side":    "sell",      // 卖出平多
		"ordType": "market",
		"sz":      quantityStr,
		"posSide": "long",
	}

	data, err := t.request("POST", "/api/v5/trade/order", body)
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: %w", err)
	}

	var orders []struct {
		OrdID string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "未知错误"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("平多仓失败: %s", msg)
	}

	log.Printf("✓ 平多仓成功: %s 数量: %s", symbol, quantityStr)

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	return map[string]interface{}{
		"orderId": orders[0].OrdID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort 平空仓
func (t *OKXFuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = -pos["positionAmt"].(float64) // 空仓数量是负的，取绝对值
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
		}
	}

	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	instID := formatOKXSymbol(symbol)

	body := map[string]interface{}{
		"instId":  instID,
		"tdMode":  "cross",
		"side":    "buy",       // 买入平空
		"ordType": "market",
		"sz":      quantityStr,
		"posSide": "short",
	}

	data, err := t.request("POST", "/api/v5/trade/order", body)
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: %w", err)
	}

	var orders []struct {
		OrdID string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "未知错误"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("平空仓失败: %s", msg)
	}

	log.Printf("✓ 平空仓成功: %s 数量: %s", symbol, quantityStr)

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	return map[string]interface{}{
		"orderId": orders[0].OrdID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// GetMarketPrice 获取市场价格
func (t *OKXFuturesTrader) GetMarketPrice(symbol string) (float64, error) {
	instID := formatOKXSymbol(symbol)

	// GET /api/v5/market/ticker?instId=BTC-USDT-SWAP
	data, err := t.request("GET", "/api/v5/market/ticker?instId="+instID, nil)
	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	var tickers []struct {
		Last string `json:"last"` // 最新成交价
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, fmt.Errorf("解析价格数据失败: %w", err)
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("未找到价格")
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// SetStopLoss 设置止损单
func (t *OKXFuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	instID := formatOKXSymbol(symbol)
	quantityStr, _ := t.FormatQuantity(symbol, quantity)

	// OKX止损单格式
	side := "sell" // 多仓止损卖出
	if positionSide == "SHORT" {
		side = "buy" // 空仓止损买入
	}

	posSide := "long"
	if positionSide == "SHORT" {
		posSide = "short"
	}

	body := map[string]interface{}{
		"instId":     instID,
		"tdMode":     "cross",
		"side":       side,
		"ordType":    "conditional", // 条件单
		"sz":         quantityStr,
		"posSide":    posSide,
		"slTriggerPx": fmt.Sprintf("%.8f", stopPrice), // 止损触发价
		"slOrdPx":    "-1",                            // -1表示市价
	}

	_, err := t.request("POST", "/api/v5/trade/order-algo", body)
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	log.Printf("  止损价设置: %.4f", stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *OKXFuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	instID := formatOKXSymbol(symbol)
	quantityStr, _ := t.FormatQuantity(symbol, quantity)

	side := "sell"
	if positionSide == "SHORT" {
		side = "buy"
	}

	posSide := "long"
	if positionSide == "SHORT" {
		posSide = "short"
	}

	body := map[string]interface{}{
		"instId":     instID,
		"tdMode":     "cross",
		"side":       side,
		"ordType":    "conditional",
		"sz":         quantityStr,
		"posSide":    posSide,
		"tpTriggerPx": fmt.Sprintf("%.8f", takeProfitPrice), // 止盈触发价
		"tpOrdPx":    "-1",
	}

	_, err := t.request("POST", "/api/v5/trade/order-algo", body)
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	log.Printf("  止盈价设置: %.4f", takeProfitPrice)
	return nil
}

// CancelStopLossOrders 仅取消止损单
func (t *OKXFuturesTrader) CancelStopLossOrders(symbol string) error {
	// OKX需要单独取消条件单
	return t.cancelAlgoOrders(symbol, "stop_loss")
}

// CancelTakeProfitOrders 仅取消止盈单
func (t *OKXFuturesTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "take_profit")
}

// CancelAllOrders 取消该币种的所有挂单
func (t *OKXFuturesTrader) CancelAllOrders(symbol string) error {
	instID := formatOKXSymbol(symbol)

	// 取消普通挂单
	body := map[string]interface{}{
		"instId": instID,
	}

	_, err := t.request("POST", "/api/v5/trade/cancel-all-orders", body)
	if err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	// 取消条件单（止盈止损）
	t.cancelAlgoOrders(symbol, "all")

	log.Printf("  ✓ 已取消 %s 的所有挂单", symbol)
	return nil
}

// CancelStopOrders 取消该币种的止盈/止损单
func (t *OKXFuturesTrader) CancelStopOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "all")
}

// cancelAlgoOrders 取消算法/条件单
func (t *OKXFuturesTrader) cancelAlgoOrders(symbol string, orderType string) error {
	instID := formatOKXSymbol(symbol)

	// 获取所有条件单
	data, err := t.request("GET", "/api/v5/trade/orders-algo-pending?instType=SWAP&instId="+instID, nil)
	if err != nil {
		log.Printf("  ⚠ 获取条件单失败: %v", err)
		return nil
	}

	var algoOrders []struct {
		AlgoID  string `json:"algoId"`
		SlTriggerPx string `json:"slTriggerPx"` // 止损触发价
		TpTriggerPx string `json:"tpTriggerPx"` // 止盈触发价
	}

	if err := json.Unmarshal(data, &algoOrders); err != nil {
		return nil
	}

	canceledCount := 0
	for _, order := range algoOrders {
		shouldCancel := false

		if orderType == "all" {
			shouldCancel = true
		} else if orderType == "stop_loss" && order.SlTriggerPx != "" {
			shouldCancel = true
		} else if orderType == "take_profit" && order.TpTriggerPx != "" {
			shouldCancel = true
		}

		if shouldCancel {
			body := []map[string]interface{}{
				{
					"instId":  instID,
					"algoId":  order.AlgoID,
				},
			}

			_, err := t.request("POST", "/api/v5/trade/cancel-algos", body)
			if err != nil {
				log.Printf("  ⚠ 取消条件单 %s 失败: %v", order.AlgoID, err)
				continue
			}

			canceledCount++
		}
	}

	if canceledCount > 0 {
		log.Printf("  ✓ 已取消 %s 的 %d 个条件单", symbol, canceledCount)
	}

	return nil
}

// FormatQuantity 格式化数量到正确的精度
func (t *OKXFuturesTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// OKX的精度通常是小数点后1位（按张数计算）
	// 实际应该查询交易规则，这里使用默认值
	return fmt.Sprintf("%.0f", quantity), nil
}

// formatOKXSymbol 将Binance格式转换为OKX格式
// BTCUSDT -> BTC-USDT-SWAP
func formatOKXSymbol(symbol string) string {
	// 移除USDT后缀，添加OKX的SWAP格式
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return fmt.Sprintf("%s-USDT-SWAP", base)
	}
	return symbol
}

// parseFloat 辅助函数：解析字符串为float64
func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
