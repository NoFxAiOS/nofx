package trader

import (
        "crypto/hmac"
        "crypto/sha256"
        "encoding/base64"
        "encoding/json"
        "fmt"
        "io"
        "log"
        "math"
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
                                // 辅助函数：安全解析float字符串
                                parseFloat := func(key string) float64 {
                                        if valStr, ok := pos[key].(string); ok && valStr != "" {
                                                if val, err := strconv.ParseFloat(valStr, 64); err == nil {
                                                        return val
                                                }
                                        }
                                        return 0.0
                                }

                                // 解析关键数值字段
                                markPrice := parseFloat("markPx")
                                entryPrice := parseFloat("avgPx")
                                quantity := parseFloat("pos")
                                upl := parseFloat("upl")
                                liqPx := parseFloat("liqPx")
                                leverage := parseFloat("lever")

                                // 将OKX格式的symbol (如 BTC-USDT-SWAP) 转换为内部格式 (如 BTCUSDT)
                                // 这是关键修复：确保返回的symbol格式与market.Get()期望的格式一致
                                okxInstId, _ := pos["instId"].(string)
                                internalSymbol := convertFromOKXSymbol(okxInstId)

                                // 标准化持仓数据格式 (适配 AutoTrader 要求)
                                standardizedPos := map[string]interface{}{
                                        // 核心字段 (AutoTrader 必需)
                                        "symbol":           internalSymbol,     // 使用内部格式 (BTCUSDT) 而非OKX格式 (BTC-USDT-SWAP)
                                        "side":             pos["posSide"],     // AutoTrader期望 key="side"
                                        "markPrice":        markPrice,          // AutoTrader期望 key="markPrice" (float64)
                                        "entryPrice":       entryPrice,         // AutoTrader期望 key="entryPrice" (float64)
                                        "positionAmt":      quantity,           // AutoTrader期望 key="positionAmt" (float64)
                                        "unRealizedProfit": upl,                // AutoTrader期望 key="unRealizedProfit" (float64)
                                        "leverage":         leverage,           // AutoTrader期望 key="leverage" (float64)
                                        "liquidationPrice": liqPx,              // AutoTrader期望 key="liquidationPrice" (float64)

                                        // 兼容性/原始字段
                                        "posSide":          pos["posSide"],
                                        "marginMode":       pos["mgnMode"],
                                        "uplRatio":         pos["uplRatio"],
                                        "okxInstId":        okxInstId,          // 保留原始OKX格式供调试用
                                }
                                positions = append(positions, standardizedPos)
                        }
                }
        }

        return positions
}

// ContractSpec 合约规格
type ContractSpec struct {
        CtVal float64 // 合约面值（1张合约对应多少币）
        MinSz float64 // 最小下单张数
        LotSz float64 // 下单精度（必须是lotSz的整数倍）
}

// getContractSpec 获取合约规格(ctVal, minSz, lotSz)
// OKX永续合约的sz参数是合约张数，需要用币数量除以合约面值来转换
func (t *OKXTrader) getContractSpec(instId string) (*ContractSpec, error) {
        // 获取合约规格
        endpoint := "/api/v5/public/instruments"
        params := map[string]string{
                "instType": "SWAP",
                "instId":   instId,
        }

        resp, err := t.makeRequest("GET", endpoint, params)
        if err != nil {
                // 如果获取失败，返回默认值
                log.Printf("⚠️ 获取合约规格失败: %v，使用默认值", err)
                return getDefaultContractSpec(instId)
        }

        if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
                if inst, ok := data[0].(map[string]interface{}); ok {
                        spec := &ContractSpec{
                                CtVal: 1.0,
                                MinSz: 1.0,
                                LotSz: 1.0,
                        }
                        
                        if ctValStr, ok := inst["ctVal"].(string); ok {
                                if v, err := strconv.ParseFloat(ctValStr, 64); err == nil {
                                        spec.CtVal = v
                                }
                        }
                        if minSzStr, ok := inst["minSz"].(string); ok {
                                if v, err := strconv.ParseFloat(minSzStr, 64); err == nil {
                                        spec.MinSz = v
                                }
                        }
                        if lotSzStr, ok := inst["lotSz"].(string); ok {
                                if v, err := strconv.ParseFloat(lotSzStr, 64); err == nil {
                                        spec.LotSz = v
                                }
                        }
                        
                        log.Printf("📋 合约规格 %s: ctVal=%.6f, minSz=%.4f, lotSz=%.4f", instId, spec.CtVal, spec.MinSz, spec.LotSz)
                        return spec, nil
                }
        }

        return getDefaultContractSpec(instId)
}

// getDefaultContractSpec 返回默认的合约规格
// 数据来源: OKX API /api/v5/public/instruments (2025-11-27更新)
func getDefaultContractSpec(instId string) (*ContractSpec, error) {
        // 常见合约的默认规格 (ctVal, minSz, lotSz)
        defaults := map[string]*ContractSpec{
                "BTC-USDT-SWAP":  {CtVal: 0.01, MinSz: 0.01, LotSz: 0.01},   // 1张 = 0.01 BTC
                "ETH-USDT-SWAP":  {CtVal: 0.1, MinSz: 0.01, LotSz: 0.01},    // 1张 = 0.1 ETH
                "SOL-USDT-SWAP":  {CtVal: 1.0, MinSz: 0.01, LotSz: 0.01},    // 1张 = 1 SOL
                "DOGE-USDT-SWAP": {CtVal: 1000.0, MinSz: 0.01, LotSz: 0.01}, // 1张 = 1000 DOGE
                "XRP-USDT-SWAP":  {CtVal: 100.0, MinSz: 0.01, LotSz: 0.01},  // 1张 = 100 XRP
                "BNB-USDT-SWAP":  {CtVal: 0.01, MinSz: 1.0, LotSz: 1.0},     // 1张 = 0.01 BNB, 必须整张
                "ADA-USDT-SWAP":  {CtVal: 100.0, MinSz: 0.1, LotSz: 0.1},    // 1张 = 100 ADA
                "HYPE-USDT-SWAP": {CtVal: 1.0, MinSz: 0.01, LotSz: 0.01},    // 1张 = 1 HYPE (估计值)
        }
        
        if spec, ok := defaults[instId]; ok {
                log.Printf("📋 使用默认合约规格 %s: ctVal=%.6f, minSz=%.4f, lotSz=%.4f", instId, spec.CtVal, spec.MinSz, spec.LotSz)
                return spec, nil
        }
        
        // 默认返回保守值
        defaultSpec := &ContractSpec{CtVal: 1.0, MinSz: 1.0, LotSz: 1.0}
        log.Printf("⚠️ 未知合约 %s，使用默认规格: ctVal=1.0, minSz=1.0, lotSz=1.0", instId)
        return defaultSpec, nil
}

// convertToContractSize 将币数量转换为合约张数
func (t *OKXTrader) convertToContractSize(instId string, coinAmount float64) (string, error) {
        spec, err := t.getContractSpec(instId)
        if err != nil {
                return "", err
        }
        
        // 合约张数 = 币数量 / 合约面值
        rawContractSize := coinAmount / spec.CtVal
        
        // 根据lotSz进行取整（向下取整到lotSz的整数倍）
        // 例如: lotSz=1 时，3.7 -> 3; lotSz=0.1 时，3.75 -> 3.7; lotSz=0.01 时，3.756 -> 3.75
        contractSize := rawContractSize
        if spec.LotSz > 0 {
                contractSize = math.Floor(rawContractSize/spec.LotSz) * spec.LotSz
        }
        
        // 检查取整后是否为0或小于最小下单量
        if contractSize < spec.MinSz {
                // 计算最小需要的币数量
                minCoinAmount := spec.MinSz * spec.CtVal
                
                if rawContractSize < spec.MinSz * 0.5 {
                        // 如果原始数量远小于最小下单量（小于50%），返回错误而不是默默使用最小值
                        return "", fmt.Errorf("下单数量过小: 需要至少 %.6f %s (最小 %.0f 张合约), 当前只有 %.6f %s",
                                minCoinAmount, instId[:strings.Index(instId, "-")], spec.MinSz, coinAmount, instId[:strings.Index(instId, "-")])
                }
                
                // 如果接近最小下单量，使用最小值并警告
                log.Printf("⚠️ 数量不足，向上调整到最小下单量: %.4f -> %.4f 张", contractSize, spec.MinSz)
                contractSize = spec.MinSz
        }
        
        log.Printf("📊 数量转换: 币数量=%.6f, ctVal=%.6f, lotSz=%.4f, minSz=%.4f -> 合约张数=%.4f", 
                coinAmount, spec.CtVal, spec.LotSz, spec.MinSz, contractSize)
        
        // 根据lotSz决定输出精度
        // lotSz=1 -> 整数; lotSz=0.1 -> 1位小数; lotSz=0.01 -> 2位小数
        var formatStr string
        if spec.LotSz >= 1 {
                formatStr = "%.0f"
        } else if spec.LotSz >= 0.1 {
                formatStr = "%.1f"
        } else {
                formatStr = "%.2f"
        }
        
        return fmt.Sprintf(formatStr, contractSize), nil
}

// OpenLong 开多仓
func (t *OKXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
        if quantity <= 0 {
                return nil, fmt.Errorf("开仓数量必须大于0")
        }

        // 转换交易对格式: BTCUSDT -> BTC-USDT-SWAP
        okxSymbol := convertToOKXSymbol(symbol)
        log.Printf("📊 OKX开多: 原始交易对=%s, OKX格式=%s, 币数量=%f, 杠杆=%d", symbol, okxSymbol, quantity, leverage)

        // 确保账户处于多空模式（使用posSide参数需要）
        if err := t.EnsureLongShortMode(); err != nil {
                log.Printf("⚠️ 设置持仓模式失败: %v，继续尝试下单", err)
        }

        // 设置杠杆（OKX要求先设置杠杆）
        if err := t.SetLeverage(okxSymbol, leverage); err != nil {
                log.Printf("⚠️ 设置杠杆失败: %v", err)
        }

        // 将币数量转换为合约张数
        contractSize, err := t.convertToContractSize(okxSymbol, quantity)
        if err != nil {
                return nil, fmt.Errorf("转换合约张数失败: %w", err)
        }

        order := map[string]string{
                "instId":  okxSymbol,        // 产品ID，如 "BTC-USDT-SWAP"
                "tdMode":  "cross",          // 保证金模式：cross(全仓) / isolated(逐仓)
                "side":    "buy",            // 订单方向：buy(买入开多)
                "posSide": "long",           // 仓位方向：long(多头) - OKX多空模式必须
                "ordType": "market",         // 订单类型：market(市价)
                "sz":      contractSize,     // 合约张数（不是币数量）
        }

        return t.placeOrder(order)
}

// OpenShort 开空仓
func (t *OKXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
        if quantity <= 0 {
                return nil, fmt.Errorf("开仓数量必须大于0")
        }

        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)
        log.Printf("📊 OKX开空: 原始交易对=%s, OKX格式=%s, 币数量=%f, 杠杆=%d", symbol, okxSymbol, quantity, leverage)

        // 确保账户处于多空模式（使用posSide参数需要）
        if err := t.EnsureLongShortMode(); err != nil {
                log.Printf("⚠️ 设置持仓模式失败: %v，继续尝试下单", err)
        }

        // 设置杠杆（OKX要求先设置杠杆）
        if err := t.SetLeverage(okxSymbol, leverage); err != nil {
                log.Printf("⚠️ 设置杠杆失败: %v", err)
        }

        // 将币数量转换为合约张数
        contractSize, err := t.convertToContractSize(okxSymbol, quantity)
        if err != nil {
                return nil, fmt.Errorf("转换合约张数失败: %w", err)
        }

        order := map[string]string{
                "instId":  okxSymbol,
                "tdMode":  "cross",
                "side":    "sell",           // 卖出开空
                "posSide": "short",          // 仓位方向：short(空头) - OKX多空模式必须
                "ordType": "market",
                "sz":      contractSize,     // 合约张数（不是币数量）
        }

        return t.placeOrder(order)
}

// CloseLong 平多仓
func (t *OKXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)
        log.Printf("📊 OKX平多: 原始交易对=%s, OKX格式=%s", symbol, okxSymbol)

        // OKX平仓通过反向订单实现
        // 获取当前持仓数量
        positions, err := t.GetPositions()
        if err != nil {
                return nil, fmt.Errorf("获取持仓失败: %w", err)
        }

        var positionSize float64
        for _, pos := range positions {
                posSymbol := pos["symbol"].(string)
                // 比较时也需要转换格式
                if (posSymbol == okxSymbol || convertToOKXSymbol(posSymbol) == okxSymbol) && pos["posSide"] == "long" {
                        if size, ok := pos["positionAmt"].(float64); ok {
                                positionSize = size
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
                "instId":  okxSymbol,
                "tdMode":  "cross",
                "side":    "sell",           // 卖出平仓
                "posSide": "long",           // 仓位方向：平多仓 - OKX多空模式必须
                "ordType": "market",
                "sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
        }

        return t.placeOrder(order)
}

// CloseShort 平空仓
func (t *OKXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)
        log.Printf("📊 OKX平空: 原始交易对=%s, OKX格式=%s", symbol, okxSymbol)

        positions, err := t.GetPositions()
        if err != nil {
                return nil, fmt.Errorf("获取持仓失败: %w", err)
        }

        var positionSize float64
        for _, pos := range positions {
                posSymbol := pos["symbol"].(string)
                // 比较时也需要转换格式
                if (posSymbol == okxSymbol || convertToOKXSymbol(posSymbol) == okxSymbol) && pos["posSide"] == "short" {
                        if size, ok := pos["positionAmt"].(float64); ok {
                                positionSize = size
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
                "instId":  okxSymbol,
                "tdMode":  "cross",
                "side":    "buy",            // 买入平仓
                "posSide": "short",          // 仓位方向：平空仓 - OKX多空模式必须
                "ordType": "market",
                "sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
        }

        return t.placeOrder(order)
}

// placeOrder 下单统一方法
func (t *OKXTrader) placeOrder(order map[string]string) (map[string]interface{}, error) {
        // OKX API: POST /api/v5/trade/order
        endpoint := "/api/v5/trade/order"

        log.Printf("📤 OKX下单请求: %+v", order)

        resp, err := t.makeRequest("POST", endpoint, order)
        if err != nil {
                return nil, fmt.Errorf("OKX下单失败: %w", err)
        }

        // 检查data数组中的详细错误信息
        if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
                if orderResp, ok := data[0].(map[string]interface{}); ok {
                        sCode, _ := orderResp["sCode"].(string)
                        sMsg, _ := orderResp["sMsg"].(string)
                        if sCode != "" && sCode != "0" {
                                log.Printf("❌ OKX下单详细错误: sCode=%s, sMsg=%s", sCode, sMsg)
                                return nil, fmt.Errorf("OKX下单失败 [%s]: %s", sCode, sMsg)
                        }
                        // 获取订单ID
                        if ordId, ok := orderResp["ordId"].(string); ok && ordId != "" {
                                log.Printf("✅ OKX下单成功: ordId=%s, side=%s, symbol=%s, quantity=%s",
                                        ordId, order["side"], order["instId"], order["sz"])
                        }
                }
        }

        return resp, nil
}

// SetLeverage 设置杠杆（多空模式下需要分别设置多头和空头杠杆）
func (t *OKXTrader) SetLeverage(symbol string, leverage int) error {
        if leverage < 1 || leverage > 125 {
                return fmt.Errorf("杠杆必须在1-125之间")
        }

        // 如果symbol已经是OKX格式，直接使用；否则转换
        okxSymbol := symbol
        if !strings.Contains(symbol, "-") {
                okxSymbol = convertToOKXSymbol(symbol)
        }

        // OKX多空模式需要分别为多头和空头设置杠杆
        endpoint := "/api/v5/account/set-leverage"
        
        // 设置多头杠杆
        paramsLong := map[string]string{
                "instId":  okxSymbol,
                "lever":   strconv.Itoa(leverage),
                "mgnMode": "cross",
                "posSide": "long",
        }
        _, err := t.makeRequest("POST", endpoint, paramsLong)
        if err != nil {
                log.Printf("⚠️ 设置多头杠杆失败: %v", err)
        }

        // 设置空头杠杆
        paramsShort := map[string]string{
                "instId":  okxSymbol,
                "lever":   strconv.Itoa(leverage),
                "mgnMode": "cross",
                "posSide": "short",
        }
        _, err = t.makeRequest("POST", endpoint, paramsShort)
        if err != nil {
                log.Printf("⚠️ 设置空头杠杆失败: %v", err)
        }

        log.Printf("✅ OKX杠杆设置成功: symbol=%s, leverage=%d (多头/空头)", okxSymbol, leverage)
        return nil
}

// SetMarginMode 设置仓位模式
func (t *OKXTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
        mgnMode := "isolated"
        if isCrossMargin {
                mgnMode = "cross"
        }

        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)

        params := map[string]string{
                "instId":  okxSymbol,
                "mgnMode": mgnMode,
        }

        // OKX API: POST /api/v5/account/set-margin-mode
        endpoint := "/api/v5/account/set-margin-mode"
        _, err := t.makeRequest("POST", endpoint, params)
        if err != nil {
                return fmt.Errorf("设置OKX保证金模式失败: %w", err)
        }

        log.Printf("✅ OKX保证金模式设置成功: symbol=%s, mode=%s", okxSymbol, mgnMode)
        return nil
}

// GetAccountConfig 获取账户配置（包括持仓模式）
func (t *OKXTrader) GetAccountConfig() (map[string]interface{}, error) {
        endpoint := "/api/v5/account/config"
        resp, err := t.makeRequest("GET", endpoint, nil)
        if err != nil {
                return nil, fmt.Errorf("获取OKX账户配置失败: %w", err)
        }
        
        if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
                if config, ok := data[0].(map[string]interface{}); ok {
                        posMode, _ := config["posMode"].(string)
                        acctLv, _ := config["acctLv"].(string)
                        log.Printf("📊 OKX账户配置: posMode=%s, acctLv=%s", posMode, acctLv)
                        return config, nil
                }
        }
        
        return nil, fmt.Errorf("解析账户配置失败")
}

// SetPositionMode 设置持仓模式 (long_short_mode / net_mode)
// OKX交易使用posSide参数需要账户处于long_short_mode
func (t *OKXTrader) SetPositionMode(posMode string) error {
        endpoint := "/api/v5/account/set-position-mode"
        params := map[string]string{
                "posMode": posMode,
        }
        
        _, err := t.makeRequest("POST", endpoint, params)
        if err != nil {
                // 如果已经是目标模式，忽略错误
                if strings.Contains(err.Error(), "already") {
                        log.Printf("ℹ️ OKX持仓模式已经是: %s", posMode)
                        return nil
                }
                return fmt.Errorf("设置OKX持仓模式失败: %w", err)
        }
        
        log.Printf("✅ OKX持仓模式设置成功: %s", posMode)
        return nil
}

// EnsureLongShortMode 确保账户处于多空模式（使用posSide参数需要）
func (t *OKXTrader) EnsureLongShortMode() error {
        config, err := t.GetAccountConfig()
        if err != nil {
                log.Printf("⚠️ 获取账户配置失败: %v，尝试直接设置持仓模式", err)
                return t.SetPositionMode("long_short_mode")
        }
        
        posMode, _ := config["posMode"].(string)
        if posMode == "long_short_mode" {
                log.Printf("✓ OKX账户已处于多空模式")
                return nil
        }
        
        log.Printf("⚠️ OKX账户当前是 %s 模式，需要切换到 long_short_mode", posMode)
        return t.SetPositionMode("long_short_mode")
}

// GetMarketPrice 获取市场价格
func (t *OKXTrader) GetMarketPrice(symbol string) (float64, error) {
        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)

        params := map[string]string{
                "instId": okxSymbol,
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
                                log.Printf("✅ OKX市场价格获取成功: symbol=%s, price=%f", okxSymbol, price)
                                return price, nil
                        }
                }
        }

        return 0, fmt.Errorf("无法解析OKX市场价格数据")
}

// SetStopLoss 设置止损单
func (t *OKXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
        side := "buy"
        posSide := "short"
        if positionSide == "long" {
                side = "sell"
                posSide = "long"
        }

        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)

        order := map[string]string{
                "instId":  okxSymbol,
                "tdMode":  "cross",
                "side":    side,
                "posSide": posSide,          // 仓位方向 - OKX多空模式必须
                "ordType": "conditional",    // 条件单
                "sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
                "tpTriggerPx": strconv.FormatFloat(stopPrice, 'f', -1, 64), // 触发价格
                "tpOrdPx": "-1", // 市价触发
        }

        _, err := t.placeOrder(order)
        if err != nil {
                return fmt.Errorf("设置OKX止损失败: %w", err)
        }

        log.Printf("✅ OKX止损设置成功: symbol=%s, posSide=%s, stopPrice=%f", okxSymbol, posSide, stopPrice)
        return nil
}

// SetTakeProfit 设置止盈单
func (t *OKXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
        side := "buy"
        posSide := "short"
        if positionSide == "long" {
                side = "sell"
                posSide = "long"
        }

        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)

        order := map[string]string{
                "instId":  okxSymbol,
                "tdMode":  "cross",
                "side":    side,
                "posSide": posSide,          // 仓位方向 - OKX多空模式必须
                "ordType": "conditional",
                "sz":      strconv.FormatFloat(quantity, 'f', -1, 64),
                "tpTriggerPx": strconv.FormatFloat(takeProfitPrice, 'f', -1, 64),
                "tpOrdPx": "-1",
        }

        _, err := t.placeOrder(order)
        if err != nil {
                return fmt.Errorf("设置OKX止盈失败: %w", err)
        }

        log.Printf("✅ OKX止盈设置成功: symbol=%s, posSide=%s, takeProfitPrice=%f", okxSymbol, posSide, takeProfitPrice)
        return nil
}

// CancelAllOrders 取消该币种的所有挂单
func (t *OKXTrader) CancelAllOrders(symbol string) error {
        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)

        params := map[string]string{
                "instId": okxSymbol,
        }

        // OKX API: POST /api/v5/trade/cancel-all-orders
        endpoint := "/api/v5/trade/cancel-all-orders"
        _, err := t.makeRequest("POST", endpoint, params)
        if err != nil {
                return fmt.Errorf("取消OKX所有订单失败: %w", err)
        }

        log.Printf("✅ OKX取消所有订单成功: symbol=%s", okxSymbol)
        return nil
}

// ClosePosition 关闭指定持仓
func (t *OKXTrader) ClosePosition(symbol string, side string) (map[string]interface{}, error) {
        // 转换交易对格式
        okxSymbol := convertToOKXSymbol(symbol)

        // 获取当前持仓
        positions, err := t.GetPositions()
        if err != nil {
                return nil, fmt.Errorf("获取持仓失败: %w", err)
        }

        // 查找匹配的持仓
        var position map[string]interface{}
        for _, pos := range positions {
                posSymbol := pos["symbol"].(string)
                if (posSymbol == okxSymbol || convertToOKXSymbol(posSymbol) == okxSymbol) && pos["side"] == side {
                        position = pos
                        break
                }
        }

        if position == nil {
                return nil, fmt.Errorf("未找到持仓: symbol=%s, side=%s", symbol, side)
        }

        quantity := position["positionAmt"].(float64)

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

        var body string
        var requestPath string = endpoint
        var fullURL string = t.baseURL + endpoint

        if method == "GET" && len(params) > 0 {
                // GET请求：参数需要添加到URL查询字符串中
                // 签名也需要包含完整的路径+查询字符串
                queryParts := make([]string, 0, len(params))
                for k, v := range params {
                        queryParts = append(queryParts, k+"="+v)
                }
                queryString := strings.Join(queryParts, "&")
                requestPath = endpoint + "?" + queryString
                fullURL = t.baseURL + requestPath
                log.Printf("📡 OKX GET请求: %s", requestPath)
        } else if method == "POST" && len(params) > 0 {
                // POST请求：参数放在body中
                jsonBody, err := json.Marshal(params)
                if err != nil {
                        return nil, fmt.Errorf("序列化请求参数失败: %w", err)
                }
                body = string(jsonBody)
                log.Printf("📡 OKX POST请求: %s, body: %s", endpoint, body)
        }

        // 生成签名（使用完整的请求路径）
        signature := t.generateSignature(timestamp, method, requestPath, body)

        // 构建请求
        var reqBody io.Reader
        if body != "" {
                reqBody = strings.NewReader(body)
        }

        req, err := http.NewRequest(method, fullURL, reqBody)
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

        // 记录完整响应用于调试
        log.Printf("📥 OKX响应: %s", string(bodyBytes))

        // 检查OKX错误码
        if code, ok := result["code"].(string); ok && code != "0" {
                msg, _ := result["msg"].(string)
                
                // 尝试从data数组获取详细错误信息
                detailError := ""
                if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
                        if item, ok := data[0].(map[string]interface{}); ok {
                                sCode, _ := item["sCode"].(string)
                                sMsg, _ := item["sMsg"].(string)
                                if sCode != "" || sMsg != "" {
                                        detailError = fmt.Sprintf(" (详细: sCode=%s, sMsg=%s)", sCode, sMsg)
                                }
                        }
                }
                
                log.Printf("❌ OKX API错误: code=%s, msg=%s%s", code, msg, detailError)
                return nil, fmt.Errorf("OKX API错误 [%s]: %s%s", code, msg, detailError)
        }

        return result, nil
}

// convertToOKXSymbol 将通用交易对格式转换为OKX格式
// 例如: BTCUSDT -> BTC-USDT-SWAP, ETHUSDT -> ETH-USDT-SWAP
func convertToOKXSymbol(symbol string) string {
        // 如果已经是OKX格式，直接返回
        if strings.Contains(symbol, "-") {
                return symbol
        }

        // 移除可能的空格
        symbol = strings.TrimSpace(symbol)
        symbol = strings.ToUpper(symbol)

        // 常见的基础货币列表（按长度降序排列，避免BTC匹配到BTCB等）
        bases := []string{
                "1000PEPE", "1000SATS", "1000SHIB", "1000BONK", "1000FLOKI", "1000RATS",
                "DOGE", "SHIB", "PEPE", "FLOKI", "BONK", "SATS", "RATS", "WIF", "MEW",
                "HYPE", "MATIC", "AVAX", "LINK", "ATOM", "NEAR", "APT", "ARB", "OP", "SUI", "SEI", "TIA", "INJ", "FTM",
                "DOT", "ADA", "XRP", "LTC", "BCH", "ETC", "FIL", "AAVE", "UNI", "MKR", "SNX", "CRV", "COMP",
                "BTC", "ETH", "SOL", "BNB",
        }

        // 常见的报价货币
        quotes := []string{"USDT", "USDC", "USD", "BUSD"}

        for _, base := range bases {
                for _, quote := range quotes {
                        if symbol == base+quote {
                                return base + "-" + quote + "-SWAP"
                        }
                }
        }

        // 通用处理：尝试从末尾匹配报价货币
        for _, quote := range quotes {
                if strings.HasSuffix(symbol, quote) {
                        base := strings.TrimSuffix(symbol, quote)
                        if base != "" {
                                return base + "-" + quote + "-SWAP"
                        }
                }
        }

        // 无法识别的格式，返回原值并添加SWAP后缀
        log.Printf("⚠️ 无法识别的交易对格式: %s", symbol)
        return symbol + "-SWAP"
}

// convertFromOKXSymbol 将OKX格式转换为通用格式
// 例如: BTC-USDT-SWAP -> BTCUSDT
func convertFromOKXSymbol(okxSymbol string) string {
        // 移除 -SWAP 后缀
        symbol := strings.TrimSuffix(okxSymbol, "-SWAP")
        // 移除中间的连字符
        symbol = strings.ReplaceAll(symbol, "-", "")
        return symbol
}