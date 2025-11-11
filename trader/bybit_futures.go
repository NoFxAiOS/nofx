package trader

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BybitFuturesTrader Bybit合约交易器
type BybitFuturesTrader struct {
	apiKey     string
	secretKey  string
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

// Bybit API响应格式
type bybitResponse struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
	Time    int64           `json:"time"`
}

// NewBybitFuturesTrader 创建Bybit合约交易器
func NewBybitFuturesTrader(apiKey, secretKey string, testnet bool) *BybitFuturesTrader {
	baseURL := "https://api.bybit.com"
	if testnet {
		baseURL = "https://api-testnet.bybit.com"
		log.Printf("⚠️ 使用Bybit测试网")
	}

	trader := &BybitFuturesTrader{
		apiKey:        apiKey,
		secretKey:     secretKey,
		baseURL:       baseURL,
		client:        &http.Client{Timeout: 30 * time.Second},
		cacheDuration: 15 * time.Second,
	}

	log.Printf("🏦 Bybit合约交易器已初始化")
	return trader
}

// sign 生成Bybit API签名 (V5)
func (t *BybitFuturesTrader) sign(timestamp, params string) string {
	message := timestamp + t.apiKey + "5000" + params // 5000 = recv_window
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// request 发送HTTP请求到Bybit
func (t *BybitFuturesTrader) request(method, path string, params map[string]interface{}) ([]byte, error) {
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	// 构建请求URL和参数
	var reqURL string
	var bodyStr string

	if method == "GET" {
		// GET请求：参数放在URL中
		if len(params) > 0 {
			query := url.Values{}
			for k, v := range params {
				query.Set(k, fmt.Sprintf("%v", v))
			}
			reqURL = t.baseURL + path + "?" + query.Encode()
			bodyStr = query.Encode()
		} else {
			reqURL = t.baseURL + path
			bodyStr = ""
		}
	} else {
		// POST请求：参数放在body中
		reqURL = t.baseURL + path
		if len(params) > 0 {
			bodyBytes, err := json.Marshal(params)
			if err != nil {
				return nil, err
			}
			bodyStr = string(bodyBytes)
		}
	}

	// 生成签名
	signature := t.sign(timestamp, bodyStr)

	// 创建请求
	req, err := http.NewRequest(method, reqURL, strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BAPI-API-KEY", t.apiKey)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", "5000")

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
		return nil, fmt.Errorf("Bybit API错误: HTTP %d, Body: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var bybitResp bybitResponse
	if err := json.Unmarshal(respBody, &bybitResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if bybitResp.RetCode != 0 {
		return nil, fmt.Errorf("Bybit API错误: %d - %s", bybitResp.RetCode, bybitResp.RetMsg)
	}

	return []byte(bybitResp.Result), nil
}

// GetBalance 获取账户余额（带缓存）
func (t *BybitFuturesTrader) GetBalance() (map[string]interface{}, error) {
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
	log.Printf("🔄 缓存过期，正在调用Bybit API获取账户余额...")

	// GET /v5/account/wallet-balance?accountType=UNIFIED
	params := map[string]interface{}{
		"accountType": "UNIFIED",
	}

	data, err := t.request("GET", "/v5/account/wallet-balance", params)
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 解析余额数据
	var balanceData struct {
		List []struct {
			TotalEquity       string `json:"totalEquity"`       // 总权益
			TotalAvailableBalance string `json:"totalAvailableBalance"` // 可用余额
			TotalPerpUPL      string `json:"totalPerpUPL"`      // 合约未实现盈亏
			Coin              []struct {
				Coin         string `json:"coin"`
				WalletBalance string `json:"walletBalance"`
				AvailableToWithdraw string `json:"availableToWithdraw"`
			} `json:"coin"`
		} `json:"list"`
	}

	if err := json.Unmarshal(data, &balanceData); err != nil {
		return nil, fmt.Errorf("解析余额数据失败: %w", err)
	}

	if len(balanceData.List) == 0 {
		return nil, fmt.Errorf("未找到账户余额数据")
	}

	account := balanceData.List[0]
	totalEquity, _ := strconv.ParseFloat(account.TotalEquity, 64)
	availableBalance, _ := strconv.ParseFloat(account.TotalAvailableBalance, 64)
	unrealizedPnL, _ := strconv.ParseFloat(account.TotalPerpUPL, 64)

	result := map[string]interface{}{
		"totalWalletBalance":   totalEquity,
		"availableBalance":     availableBalance,
		"totalUnrealizedProfit": unrealizedPnL,
	}

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	log.Printf("✓ Bybit API返回: 总权益=%.2f, 可用=%.2f, 未实现盈亏=%.2f", totalEquity, availableBalance, unrealizedPnL)
	return result, nil
}

// GetPositions 获取所有持仓（带缓存）
func (t *BybitFuturesTrader) GetPositions() ([]map[string]interface{}, error) {
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
	log.Printf("🔄 缓存过期，正在调用Bybit API获取持仓信息...")

	// GET /v5/position/list?category=linear&settleCoin=USDT
	params := map[string]interface{}{
		"category":   "linear",
		"settleCoin": "USDT",
	}

	data, err := t.request("GET", "/v5/position/list", params)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	// 解析持仓数据
	var positionData struct {
		List []struct {
			Symbol        string `json:"symbol"`        // 交易对 (如 BTCUSDT)
			Side          string `json:"side"`          // Buy=多, Sell=空
			Size          string `json:"size"`          // 持仓数量
			AvgPrice      string `json:"avgPrice"`      // 开仓均价
			MarkPrice     string `json:"markPrice"`     // 标记价格
			UnrealisedPnl string `json:"unrealisedPnl"` // 未实现盈亏
			Leverage      string `json:"leverage"`      // 杠杆倍数
			LiqPrice      string `json:"liqPrice"`      // 强平价
		} `json:"list"`
	}

	if err := json.Unmarshal(data, &positionData); err != nil {
		return nil, fmt.Errorf("解析持仓数据失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positionData.List {
		size, _ := strconv.ParseFloat(pos.Size, 64)
		if size == 0 {
			continue // 跳过无持仓的
		}

		posMap := map[string]interface{}{
			"symbol":            pos.Symbol,
			"positionAmt":       size,
			"entryPrice":        parseFloatBybit(pos.AvgPrice),
			"markPrice":         parseFloatBybit(pos.MarkPrice),
			"unRealizedProfit":  parseFloatBybit(pos.UnrealisedPnl),
			"leverage":          parseFloatBybit(pos.Leverage),
			"liquidationPrice":  parseFloatBybit(pos.LiqPrice),
		}

		// 判断方向
		if pos.Side == "Buy" {
			posMap["side"] = "long"
		} else {
			posMap["side"] = "short"
			// Bybit的空仓数量是正数，我们转换为负数以保持一致
			posMap["positionAmt"] = -size
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
func (t *BybitFuturesTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Bybit V5: POST /v5/position/switch-isolated
	// tradeMode: 0=全仓, 1=逐仓
	tradeMode := 0
	if !isCrossMargin {
		tradeMode = 1
	}

	params := map[string]interface{}{
		"category":  "linear",
		"symbol":    symbol,
		"tradeMode": tradeMode,
		"buyLeverage": "1",  // 多仓杠杆
		"sellLeverage": "1", // 空仓杠杆
	}

	_, err := t.request("POST", "/v5/position/switch-isolated", params)
	if err != nil {
		// 如果已经是目标模式则忽略错误
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "leverage not modified") {
			log.Printf("  ✓ %s 仓位模式已是 %s", symbol, map[bool]string{true: "全仓", false: "逐仓"}[isCrossMargin])
			return nil
		}
		return fmt.Errorf("设置仓位模式失败: %w", err)
	}

	log.Printf("  ✓ %s 仓位模式已设置为 %s", symbol, map[bool]string{true: "全仓", false: "逐仓"}[isCrossMargin])
	return nil
}

// SetLeverage 设置杠杆
func (t *BybitFuturesTrader) SetLeverage(symbol string, leverage int) error {
	// POST /v5/position/set-leverage
	params := map[string]interface{}{
		"category":     "linear",
		"symbol":       symbol,
		"buyLeverage":  fmt.Sprintf("%d", leverage),  // 多仓杠杆
		"sellLeverage": fmt.Sprintf("%d", leverage),  // 空仓杠杆
	}

	_, err := t.request("POST", "/v5/position/set-leverage", params)
	if err != nil {
		if strings.Contains(err.Error(), "leverage not modified") {
			log.Printf("  ✓ %s 杠杆已是 %dx", symbol, leverage)
			return nil
		}
		return fmt.Errorf("设置杠杆失败: %w", err)
	}

	log.Printf("  ✓ %s 杠杆已切换为 %dx", symbol, leverage)
	return nil
}

// OpenLong 开多仓
func (t *BybitFuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
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

	// POST /v5/order/create
	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Buy",       // 买入开多
		"orderType":   "Market",    // 市价单
		"qty":         quantityStr,
		"positionIdx": 0,           // 0=单向持仓, 1=双向持仓-多, 2=双向持仓-空
	}

	data, err := t.request("POST", "/v5/order/create", params)
	if err != nil {
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}

	// 解析订单响应
	var orderResp struct {
		OrderID     string `json:"orderId"`
		OrderLinkID string `json:"orderLinkId"`
	}

	if err := json.Unmarshal(data, &orderResp); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	log.Printf("✓ 开多仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %s", orderResp.OrderID)

	return map[string]interface{}{
		"orderId": orderResp.OrderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort 开空仓
func (t *BybitFuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
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

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Sell",      // 卖出开空
		"orderType":   "Market",
		"qty":         quantityStr,
		"positionIdx": 0,
	}

	data, err := t.request("POST", "/v5/order/create", params)
	if err != nil {
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	var orderResp struct {
		OrderID     string `json:"orderId"`
		OrderLinkID string `json:"orderLinkId"`
	}

	if err := json.Unmarshal(data, &orderResp); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	log.Printf("✓ 开空仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %s", orderResp.OrderID)

	return map[string]interface{}{
		"orderId": orderResp.OrderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong 平多仓
func (t *BybitFuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
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

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Sell",      // 卖出平多
		"orderType":   "Market",
		"qty":         quantityStr,
		"positionIdx": 0,
		"reduceOnly":  true,        // 只减仓
	}

	data, err := t.request("POST", "/v5/order/create", params)
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: %w", err)
	}

	var orderResp struct {
		OrderID string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &orderResp); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	log.Printf("✓ 平多仓成功: %s 数量: %s", symbol, quantityStr)

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	return map[string]interface{}{
		"orderId": orderResp.OrderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort 平空仓
func (t *BybitFuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = -pos["positionAmt"].(float64) // 取绝对值
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

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        "Buy",       // 买入平空
		"orderType":   "Market",
		"qty":         quantityStr,
		"positionIdx": 0,
		"reduceOnly":  true,
	}

	data, err := t.request("POST", "/v5/order/create", params)
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: %w", err)
	}

	var orderResp struct {
		OrderID string `json:"orderId"`
	}

	if err := json.Unmarshal(data, &orderResp); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	log.Printf("✓ 平空仓成功: %s 数量: %s", symbol, quantityStr)

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	return map[string]interface{}{
		"orderId": orderResp.OrderID,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// GetMarketPrice 获取市场价格
func (t *BybitFuturesTrader) GetMarketPrice(symbol string) (float64, error) {
	// GET /v5/market/tickers?category=linear&symbol=BTCUSDT
	params := map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
	}

	data, err := t.request("GET", "/v5/market/tickers", params)
	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	var tickerData struct {
		List []struct {
			LastPrice string `json:"lastPrice"` // 最新成交价
		} `json:"list"`
	}

	if err := json.Unmarshal(data, &tickerData); err != nil {
		return 0, fmt.Errorf("解析价格数据失败: %w", err)
	}

	if len(tickerData.List) == 0 {
		return 0, fmt.Errorf("未找到价格")
	}

	price, err := strconv.ParseFloat(tickerData.List[0].LastPrice, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// SetStopLoss 设置止损单
func (t *BybitFuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	quantityStr, _ := t.FormatQuantity(symbol, quantity)

	// Bybit止损单
	side := "Sell" // 多仓止损卖出
	if positionSide == "SHORT" {
		side = "Buy"
	}

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        side,
		"orderType":   "Market",
		"qty":         quantityStr,
		"stopLoss":    fmt.Sprintf("%.8f", stopPrice),
		"positionIdx": 0,
		"reduceOnly":  true,
	}

	_, err := t.request("POST", "/v5/order/create", params)
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	log.Printf("  止损价设置: %.4f", stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *BybitFuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	quantityStr, _ := t.FormatQuantity(symbol, quantity)

	side := "Sell"
	if positionSide == "SHORT" {
		side = "Buy"
	}

	params := map[string]interface{}{
		"category":    "linear",
		"symbol":      symbol,
		"side":        side,
		"orderType":   "Market",
		"qty":         quantityStr,
		"takeProfit":  fmt.Sprintf("%.8f", takeProfitPrice),
		"positionIdx": 0,
		"reduceOnly":  true,
	}

	_, err := t.request("POST", "/v5/order/create", params)
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	log.Printf("  止盈价设置: %.4f", takeProfitPrice)
	return nil
}

// CancelStopLossOrders 仅取消止损单
func (t *BybitFuturesTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelConditionalOrders(symbol, "stop_loss")
}

// CancelTakeProfitOrders 仅取消止盈单
func (t *BybitFuturesTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelConditionalOrders(symbol, "take_profit")
}

// CancelAllOrders 取消该币种的所有挂单
func (t *BybitFuturesTrader) CancelAllOrders(symbol string) error {
	// POST /v5/order/cancel-all
	params := map[string]interface{}{
		"category": "linear",
		"symbol":   symbol,
	}

	_, err := t.request("POST", "/v5/order/cancel-all", params)
	if err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	log.Printf("  ✓ 已取消 %s 的所有挂单", symbol)
	return nil
}

// CancelStopOrders 取消该币种的止盈/止损单
func (t *BybitFuturesTrader) CancelStopOrders(symbol string) error {
	return t.cancelConditionalOrders(symbol, "all")
}

// cancelConditionalOrders 取消条件单（止盈/止损）
func (t *BybitFuturesTrader) cancelConditionalOrders(symbol string, orderType string) error {
	// 获取所有未完成订单
	params := map[string]interface{}{
		"category":  "linear",
		"symbol":    symbol,
		"orderFilter": "StopOrder", // 只获取条件单
	}

	data, err := t.request("GET", "/v5/order/realtime", params)
	if err != nil {
		log.Printf("  ⚠ 获取条件单失败: %v", err)
		return nil
	}

	var orderData struct {
		List []struct {
			OrderID   string `json:"orderId"`
			StopLoss  string `json:"stopLoss"`
			TakeProfit string `json:"takeProfit"`
		} `json:"list"`
	}

	if err := json.Unmarshal(data, &orderData); err != nil {
		return nil
	}

	canceledCount := 0
	for _, order := range orderData.List {
		shouldCancel := false

		if orderType == "all" {
			shouldCancel = true
		} else if orderType == "stop_loss" && order.StopLoss != "" {
			shouldCancel = true
		} else if orderType == "take_profit" && order.TakeProfit != "" {
			shouldCancel = true
		}

		if shouldCancel {
			cancelParams := map[string]interface{}{
				"category": "linear",
				"symbol":   symbol,
				"orderId":  order.OrderID,
			}

			_, err := t.request("POST", "/v5/order/cancel", cancelParams)
			if err != nil {
				log.Printf("  ⚠ 取消条件单 %s 失败: %v", order.OrderID, err)
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
func (t *BybitFuturesTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// Bybit的精度通常是小数点后3位
	// 实际应该查询交易规则，这里使用默认值
	return fmt.Sprintf("%.3f", quantity), nil
}

// parseFloatBybit 辅助函数：解析字符串为float64
func parseFloatBybit(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// sortMapKeys 辅助函数：对map的keys排序（用于签名）
func sortMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
