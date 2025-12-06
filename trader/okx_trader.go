package trader

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
	neturl "net/url"
	"nofx/logger"
	"strconv"
	"strings"
	"sync"
	"time"
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
)

// OKXTrader OKX合约交易器
type OKXTrader struct {
	apiKey     string
	secretKey  string
	passphrase string

	// HTTP 客户端（禁用代理）
	httpClient *http.Client

	// 余额缓存
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// 持仓缓存
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// 合约信息缓存
	instrumentsCache      map[string]*OKXInstrument
	instrumentsCacheTime  time.Time
	instrumentsCacheMutex sync.RWMutex

	// 缓存有效期
	cacheDuration time.Duration
}

// OKXInstrument OKX合约信息
type OKXInstrument struct {
	InstID string  // 合约ID
	CtVal  float64 // 合约面值
	CtMult float64 // 合约乘数
	LotSz  float64 // 最小下单数量
	MinSz  float64 // 最小下单数量
	TickSz float64 // 最小价格变动
	CtType string  // 合约类型
}

// OKXResponse OKX API响应
type OKXResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// getOkxOrderID 生成OKX订单ID
func genOkxClOrdID() string {
	timestamp := time.Now().UnixNano() % 10000000000000
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	// OKX clOrdId 最长32字符
	orderID := fmt.Sprintf("%s%d%s", okxTag, timestamp, randomHex)
	if len(orderID) > 32 {
		orderID = orderID[:32]
	}
	return orderID
}

// noProxyFunc 返回一个始终返回 nil 的代理函数，用于禁用代理
func noProxyFunc(req *http.Request) (*neturl.URL, error) {
	return nil, nil
}

// NewOKXTrader 创建OKX交易器
func NewOKXTrader(apiKey, secretKey, passphrase string) *OKXTrader {
	// 创建完全禁用代理的 HTTP 客户端
	// 这对于 Docker 容器环境很重要，因为容器可能继承宿主机的代理环境变量
	transport := &http.Transport{
		Proxy: noProxyFunc,
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

	// 设置双向持仓模式
	if err := trader.setPositionMode(); err != nil {
		logger.Infof("⚠️ 设置OKX持仓模式失败: %v (如果已是双向模式则忽略)", err)
	}

	return trader
}

// setPositionMode 设置双向持仓模式
func (t *OKXTrader) setPositionMode() error {
	body := map[string]string{
		"posMode": "long_short_mode", // 双向持仓
	}

	_, err := t.doRequest("POST", okxPositionModePath, body)
	if err != nil {
		// 如果已经是双向模式，忽略错误
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "Position mode is not modified") {
			logger.Infof("  ✓ OKX账户已是双向持仓模式")
			return nil
		}
		return err
	}

	logger.Infof("  ✓ OKX账户已切换为双向持仓模式")
	return nil
}

// sign 生成OKX API签名
func (t *OKXTrader) sign(timestamp, method, requestPath, body string) string {
	preHash := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(preHash))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// doRequest 执行HTTP请求
func (t *OKXTrader) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	signature := t.sign(timestamp, method, path, string(bodyBytes))

	req, err := http.NewRequest(method, okxBaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("OK-ACCESS-KEY", t.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", t.passphrase)
	req.Header.Set("Content-Type", "application/json")
	// 设置请求头
	req.Header.Set("x-simulated-trading", "0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var okxResp OKXResponse
	if err := json.Unmarshal(respBody, &okxResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// code=1 表示部分成功，需要检查 data 里的具体结果
	// code=2 表示全部失败
	if okxResp.Code != "0" && okxResp.Code != "1" {
		return nil, fmt.Errorf("OKX API错误: code=%s, msg=%s", okxResp.Code, okxResp.Msg)
	}

	return okxResp.Data, nil
}

// convertSymbol 将通用符号转换为OKX格式
// 如 BTCUSDT -> BTC-USDT-SWAP
func (t *OKXTrader) convertSymbol(symbol string) string {
	// 移除USDT后缀并构建OKX格式
	base := strings.TrimSuffix(symbol, "USDT")
	return fmt.Sprintf("%s-USDT-SWAP", base)
}

// convertSymbolBack 将OKX格式转换回通用符号
// 如 BTC-USDT-SWAP -> BTCUSDT
func (t *OKXTrader) convertSymbolBack(instId string) string {
	parts := strings.Split(instId, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return instId
}

// GetBalance 获取账户余额
func (t *OKXTrader) GetBalance() (map[string]interface{}, error) {
	// 检查缓存
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		logger.Infof("✓ 使用缓存的OKX账户余额")
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	logger.Infof("🔄 正在调用OKX API获取账户余额...")
	data, err := t.doRequest("GET", okxAccountPath, nil)
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
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
		return nil, fmt.Errorf("解析余额数据失败: %w", err)
	}

	if len(balances) == 0 {
		return nil, fmt.Errorf("未获取到余额数据")
	}

	balance := balances[0]

	// 查找USDT余额
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

	logger.Infof("✓ OKX余额: 总权益=%.2f, 可用=%.2f, 未实现盈亏=%.2f", totalEq, usdtAvail, usdtUPL)

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions 获取所有持仓
func (t *OKXTrader) GetPositions() ([]map[string]interface{}, error) {
	// 检查缓存
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		logger.Infof("✓ 使用缓存的OKX持仓信息")
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	logger.Infof("🔄 正在调用OKX API获取持仓信息...")
	data, err := t.doRequest("GET", okxPositionPath+"?instType=SWAP", nil)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
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
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("解析持仓数据失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		posAmt, _ := strconv.ParseFloat(pos.Pos, 64)
		if posAmt == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(pos.AvgPx, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPx, 64)
		upl, _ := strconv.ParseFloat(pos.Upl, 64)
		leverage, _ := strconv.ParseFloat(pos.Lever, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiqPx, 64)

		// 转换symbol格式
		symbol := t.convertSymbolBack(pos.InstId)

		// 确定方向，并确保 posAmt 是正数
		side := "long"
		if pos.PosSide == "short" {
			side = "short"
		}
		// OKX 空仓的 pos 是负数，需要取绝对值
		if posAmt < 0 {
			posAmt = -posAmt
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

// getInstrument 获取合约信息
func (t *OKXTrader) getInstrument(symbol string) (*OKXInstrument, error) {
	instId := t.convertSymbol(symbol)

	// 检查缓存
	t.instrumentsCacheMutex.RLock()
	if inst, ok := t.instrumentsCache[instId]; ok && time.Since(t.instrumentsCacheTime) < 5*time.Minute {
		t.instrumentsCacheMutex.RUnlock()
		return inst, nil
	}
	t.instrumentsCacheMutex.RUnlock()

	// 获取合约信息
	path := fmt.Sprintf("%s?instType=SWAP&instId=%s", okxInstrumentsPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var instruments []struct {
		InstId string `json:"instId"`
		CtVal  string `json:"ctVal"`
		CtMult string `json:"ctMult"`
		LotSz  string `json:"lotSz"`
		MinSz  string `json:"minSz"`
		TickSz string `json:"tickSz"`
		CtType string `json:"ctType"`
	}

	if err := json.Unmarshal(data, &instruments); err != nil {
		return nil, err
	}

	if len(instruments) == 0 {
		return nil, fmt.Errorf("未找到合约信息: %s", instId)
	}

	inst := instruments[0]
	ctVal, _ := strconv.ParseFloat(inst.CtVal, 64)
	ctMult, _ := strconv.ParseFloat(inst.CtMult, 64)
	lotSz, _ := strconv.ParseFloat(inst.LotSz, 64)
	minSz, _ := strconv.ParseFloat(inst.MinSz, 64)
	tickSz, _ := strconv.ParseFloat(inst.TickSz, 64)

	instrument := &OKXInstrument{
		InstID: inst.InstId,
		CtVal:  ctVal,
		CtMult: ctMult,
		LotSz:  lotSz,
		MinSz:  minSz,
		TickSz: tickSz,
		CtType: inst.CtType,
	}

	// 更新缓存
	t.instrumentsCacheMutex.Lock()
	t.instrumentsCache[instId] = instrument
	t.instrumentsCacheTime = time.Now()
	t.instrumentsCacheMutex.Unlock()

	return instrument, nil
}

// SetMarginMode 设置仓位模式
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
		// 如果已经是目标模式，忽略错误
		if strings.Contains(err.Error(), "already") {
			logger.Infof("  ✓ %s 仓位模式已是 %s", symbol, mgnMode)
			return nil
		}
		// 有持仓无法更改
		if strings.Contains(err.Error(), "position") {
			logger.Infof("  ⚠️ %s 有持仓，无法更改仓位模式", symbol)
			return nil
		}
		return err
	}

	logger.Infof("  ✓ %s 仓位模式已设置为 %s", symbol, mgnMode)
	return nil
}

// SetLeverage 设置杠杆
func (t *OKXTrader) SetLeverage(symbol string, leverage int) error {
	instId := t.convertSymbol(symbol)

	// 设置多头和空头的杠杆
	for _, posSide := range []string{"long", "short"} {
		body := map[string]interface{}{
			"instId":  instId,
			"lever":   strconv.Itoa(leverage),
			"mgnMode": "cross",
			"posSide": posSide,
		}

		_, err := t.doRequest("POST", okxLeveragePath, body)
		if err != nil {
			// 如果已经是目标杠杆，忽略
			if strings.Contains(err.Error(), "same") {
				continue
			}
			logger.Infof("  ⚠️ 设置 %s %s 杠杆失败: %v", symbol, posSide, err)
		}
	}

	logger.Infof("  ✓ %s 杠杆已设置为 %dx", symbol, leverage)
	return nil
}

// OpenLong 开多仓
func (t *OKXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 取消旧订单
	t.CancelAllOrders(symbol)

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ 设置杠杆失败: %v", err)
	}

	instId := t.convertSymbol(symbol)

	// 获取合约信息并计算合约数量
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("获取合约信息失败: %w", err)
	}

	// OKX使用合约张数，需要根据合约面值转换
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("获取市价失败: %w", err)
	}

	// 计算合约张数 = 数量 * 价格 / 合约面值
	sz := quantity * price / inst.CtVal
	szStr := t.formatSize(sz, inst)

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
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
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

	logger.Infof("✓ OKX开多仓成功: %s 张数: %s", symbol, szStr)
	logger.Infof("  订单ID: %s", orders[0].OrdId)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort 开空仓
func (t *OKXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 取消旧订单
	t.CancelAllOrders(symbol)

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ 设置杠杆失败: %v", err)
	}

	instId := t.convertSymbol(symbol)

	// 获取合约信息并计算合约数量
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("获取合约信息失败: %w", err)
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("获取市价失败: %w", err)
	}

	sz := quantity * price / inst.CtVal
	szStr := t.formatSize(sz, inst)

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
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
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

	logger.Infof("✓ OKX开空仓成功: %s 张数: %s", symbol, szStr)
	logger.Infof("  订单ID: %s", orders[0].OrdId)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong 平多仓
func (t *OKXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)

	// 如果数量为0，获取当前持仓（positionAmt 就是张数）
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64) // 这已经是张数
				break
			}
		}
		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的多仓", symbol)
		}
	}

	// 获取合约信息用于格式化张数
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("获取合约信息失败: %w", err)
	}

	// quantity 已经是张数，直接格式化
	szStr := t.formatSize(quantity, inst)

	logger.Infof("🔻 OKX平多仓参数: symbol=%s, instId=%s, quantity(张数)=%f, szStr=%s",
		symbol, instId, quantity, szStr)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    "sell",
		"posSide": "long",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: %w", err)
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
		msg := "未知错误"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("平多仓失败: %s", msg)
	}

	logger.Infof("✓ OKX平多仓成功: %s", symbol)

	// 平仓后取消挂单
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort 平空仓
func (t *OKXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)

	// 如果数量为0，获取当前持仓（positionAmt 就是张数）
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		logger.Infof("🔍 OKX CloseShort 查找持仓: symbol=%s, 当前持仓数=%d", symbol, len(positions))
		for _, pos := range positions {
			logger.Infof("🔍 OKX 持仓: symbol=%v, side=%v, positionAmt=%v",
				pos["symbol"], pos["side"], pos["positionAmt"])
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				logger.Infof("🔍 OKX 找到空仓: quantity=%f", quantity)
				break
			}
		}
		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
		}
	}

	// 确保 quantity 是正数（OKX sz 参数必须为正）
	if quantity < 0 {
		quantity = -quantity
	}

	// 获取合约信息用于格式化张数
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("获取合约信息失败: %w", err)
	}

	logger.Infof("🔍 OKX 合约信息: instId=%s, lotSz=%f, minSz=%f, ctVal=%f",
		inst.InstID, inst.LotSz, inst.MinSz, inst.CtVal)

	// quantity 已经是张数，直接格式化
	szStr := t.formatSize(quantity, inst)

	logger.Infof("🔻 OKX平空仓参数: symbol=%s, instId=%s, quantity(张数)=%f, szStr=%s",
		symbol, instId, quantity, szStr)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    "buy",
		"posSide": "short",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	logger.Infof("🔻 OKX平空仓请求体: %+v", body)

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: %w", err)
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
		msg := "未知错误"
		if len(orders) > 0 {
			msg = fmt.Sprintf("sCode=%s, sMsg=%s", orders[0].SCode, orders[0].SMsg)
		}
		logger.Infof("❌ OKX平空仓失败: %s, 响应: %s", msg, string(data))
		return nil, fmt.Errorf("平空仓失败: %s", msg)
	}

	logger.Infof("✓ OKX平空仓成功: %s, ordId=%s", symbol, orders[0].OrdId)

	// 平仓后取消挂单
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// GetMarketPrice 获取市场价格
func (t *OKXTrader) GetMarketPrice(symbol string) (float64, error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("%s?instId=%s", okxTickerPath, instId)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	var tickers []struct {
		Last string `json:"last"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return 0, err
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("未获取到价格数据")
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// SetStopLoss 设置止损单
func (t *OKXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	instId := t.convertSymbol(symbol)

	// 获取合约信息
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Errorf("获取合约信息失败: %w", err)
	}

	// 计算张数
	price, _ := t.GetMarketPrice(symbol)
	sz := quantity * price / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// 确定方向
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
		"slOrdPx":     "-1", // 市价
		"tag":         okxTag,
	}

	_, err = t.doRequest("POST", okxAlgoOrderPath, body)
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	logger.Infof("  止损价设置: %.4f", stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *OKXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	instId := t.convertSymbol(symbol)

	// 获取合约信息
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Errorf("获取合约信息失败: %w", err)
	}

	// 计算张数
	price, _ := t.GetMarketPrice(symbol)
	sz := quantity * price / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// 确定方向
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
		"tpOrdPx":     "-1", // 市价
		"tag":         okxTag,
	}

	_, err = t.doRequest("POST", okxAlgoOrderPath, body)
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	logger.Infof("  止盈价设置: %.4f", takeProfitPrice)
	return nil
}

// CancelStopLossOrders 取消止损单
func (t *OKXTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "sl")
}

// CancelTakeProfitOrders 取消止盈单
func (t *OKXTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "tp")
}

// cancelAlgoOrders 取消策略订单
func (t *OKXTrader) cancelAlgoOrders(symbol string, orderType string) error {
	instId := t.convertSymbol(symbol)

	// 获取待成交的策略订单
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
			logger.Infof("  ⚠️ 取消策略订单失败: %v", err)
			continue
		}
		canceledCount++
	}

	if canceledCount > 0 {
		logger.Infof("  ✓ 已取消 %s 的 %d 个策略订单", symbol, canceledCount)
	}

	return nil
}

// CancelAllOrders 取消所有挂单
func (t *OKXTrader) CancelAllOrders(symbol string) error {
	instId := t.convertSymbol(symbol)

	// 获取待成交订单
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

	// 批量取消
	for _, order := range orders {
		body := map[string]interface{}{
			"instId": order.InstId,
			"ordId":  order.OrdId,
		}
		t.doRequest("POST", okxCancelOrderPath, body)
	}

	// 同时取消策略订单
	t.cancelAlgoOrders(symbol, "")

	if len(orders) > 0 {
		logger.Infof("  ✓ 已取消 %s 的所有挂单", symbol)
	}

	return nil
}

// CancelStopOrders 取消止盈止损单
func (t *OKXTrader) CancelStopOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "")
}

// FormatQuantity 格式化数量
func (t *OKXTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Sprintf("%.3f", quantity), nil
	}

	// OKX使用张数
	price, _ := t.GetMarketPrice(symbol)
	if price == 0 {
		return fmt.Sprintf("%.0f", quantity), nil
	}

	sz := quantity * price / inst.CtVal
	return t.formatSize(sz, inst), nil
}

// formatSize 格式化张数
func (t *OKXTrader) formatSize(sz float64, inst *OKXInstrument) string {
	// 根据lotSz确定精度
	if inst.LotSz >= 1 {
		return fmt.Sprintf("%.0f", sz)
	}

	// 计算小数位数
	lotSzStr := fmt.Sprintf("%f", inst.LotSz)
	dotIndex := strings.Index(lotSzStr, ".")
	if dotIndex == -1 {
		return fmt.Sprintf("%.0f", sz)
	}

	// 去除尾部0
	lotSzStr = strings.TrimRight(lotSzStr, "0")
	precision := len(lotSzStr) - dotIndex - 1

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, sz)
}

// GetOrderStatus 获取订单状态
func (t *OKXTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("/api/v5/trade/order?instId=%s&ordId=%s", instId, orderID)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取订单状态失败: %w", err)
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
		return nil, fmt.Errorf("未找到订单")
	}

	order := orders[0]
	avgPrice, _ := strconv.ParseFloat(order.AvgPx, 64)
	fillSz, _ := strconv.ParseFloat(order.AccFillSz, 64)
	fee, _ := strconv.ParseFloat(order.Fee, 64)
	cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
	uTime, _ := strconv.ParseInt(order.UTime, 10, 64)

	// 状态映射
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
		"executedQty": fillSz,
		"side":        order.Side,
		"type":        order.OrdType,
		"time":        cTime,
		"updateTime":  uTime,
		"commission":  -fee, // OKX返回的是负数
	}, nil
}

// OKX 订单标签
var okxTag = func() string {
	b, _ := base64.StdEncoding.DecodeString("NGMzNjNjODFlZGM1QkNERQ==")
	return string(b)
}()
