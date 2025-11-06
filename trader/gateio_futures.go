package trader

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antihax/optional"
	gateapi "github.com/gateio/gateapi-go/v7"
)

// GateIOFuturesTrader Gate.io 合约交易器
type GateIOFuturesTrader struct {
	client *gateapi.APIClient
	ctx    context.Context

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

	// 交易对精度缓存
	symbolPrecision map[string]SymbolPrecision
	precisionMutex  sync.RWMutex
}

// NewGateIOFuturesTrader 创建 Gate.io 合约交易器
func NewGateIOFuturesTrader(apiKey, secretKey string, testnet bool) *GateIOFuturesTrader {
	cfg := gateapi.NewConfiguration()
	if testnet {
		cfg.BasePath = "https://api-testnet.gateapi.io/api/v4"
	}
	client := gateapi.NewAPIClient(cfg)

	ctx := context.WithValue(context.Background(), gateapi.ContextGateAPIV4, gateapi.GateAPIV4{
		Key:    apiKey,
		Secret: secretKey,
	})

	return &GateIOFuturesTrader{
		client:          client,
		ctx:             ctx,
		cacheDuration:   15 * time.Second,
		symbolPrecision: make(map[string]SymbolPrecision),
	}
}

// convertSymbolToGateIO 将 ETHUSDT 格式转换为 ETH_USDT 格式
func convertSymbolToGateIO(symbol string) string {
	// 如果已经包含下划线，直接返回
	if strings.Contains(symbol, "_") {
		return symbol
	}

	// 如果以 USDT 结尾，在 USDT 前添加下划线
	if strings.HasSuffix(symbol, "USDT") {
		base := symbol[:len(symbol)-4] // 去掉 USDT
		return base + "_USDT"
	}

	// 其他情况直接返回
	return symbol
}

// convertSymbolFromGateIO 将 ETH_USDT 格式转换为 ETHUSDT 格式
func convertSymbolFromGateIO(symbol string) string {
	// 如果不包含下划线，直接返回
	if !strings.Contains(symbol, "_") {
		return symbol
	}

	// 如果包含下划线，将下划线去掉
	return strings.ReplaceAll(symbol, "_", "")
}

// GetBalance 获取账户余额（带缓存）
func (t *GateIOFuturesTrader) GetBalance() (map[string]interface{}, error) {
	// 先检查缓存是否有效
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的账户余额（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// 缓存过期或不存在，调用 API
	log.Printf("🔄 缓存过期，正在调用 Gate.io API 获取账户余额...")
	account, _, err := t.client.FuturesApi.ListFuturesAccounts(t.ctx, "usdt")
	if err != nil {
		log.Printf("❌ Gate.io API 调用失败: %v", err)
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}

	result := make(map[string]interface{})
	result["totalWalletBalance"], _ = strconv.ParseFloat(account.Total, 64)
	result["availableBalance"], _ = strconv.ParseFloat(account.Available, 64)
	result["totalUnrealizedProfit"], _ = strconv.ParseFloat(account.UnrealisedPnl, 64)

	log.Printf("✓ Gate.io API 返回: 总余额=%s, 可用=%s, 未实现盈亏=%s", account.Total, account.Available, account.UnrealisedPnl)

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions 获取所有持仓（带缓存）
func (t *GateIOFuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	// 先检查缓存是否有效
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的持仓信息（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// 缓存过期或不存在，调用 API
	log.Printf("🔄 缓存过期，正在调用 Gate.io API 获取持仓信息...")
	positions, _, err := t.client.FuturesApi.ListPositions(t.ctx, "usdt", nil)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		posAmt := float64(pos.Size)
		if posAmt == 0 {
			continue // 跳过无持仓的
		}
		symbol := convertSymbolFromGateIO(pos.Contract)

		posAmtFloat, err := t.FormatQuantityToFloat64(symbol, posAmt)
		if err != nil {
			return nil, fmt.Errorf("格式化数量到正确精度失败: %w", err)
		}
		posMap := make(map[string]interface{})
		posMap["symbol"] = symbol
		posMap["positionAmt"] = posAmtFloat

		entryPrice, _ := strconv.ParseFloat(pos.EntryPrice, 64)
		posMap["entryPrice"] = entryPrice

		markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64)
		posMap["markPrice"] = markPrice

		unRealizedProfit, _ := strconv.ParseFloat(pos.UnrealisedPnl, 64)
		posMap["unRealizedProfit"] = unRealizedProfit

		leverage, _ := strconv.ParseFloat(pos.CrossLeverageLimit, 64)
		posMap["leverage"] = leverage

		liquidationPrice, _ := strconv.ParseFloat(pos.LiqPrice, 64)
		posMap["liquidationPrice"] = liquidationPrice

		// 判断方向
		if posAmt > 0 { // 格式化数量到正确精度
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
func (t *GateIOFuturesTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	marginMode := "CROSS"
	if !isCrossMargin {
		marginMode = "ISOLATED"
	}

	gateioSymbol := convertSymbolToGateIO(symbol)
	marginModeReq := gateapi.FuturesPositionCrossMode{
		Contract: gateioSymbol,
		Mode:     marginMode,
	}

	_, _, err := t.client.FuturesApi.UpdatePositionCrossMode(t.ctx, "usdt", marginModeReq)

	if err != nil {
		// 如果错误表示无需更改，忽略错误
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "No need") {
			marginModeStr := "全仓"
			if !isCrossMargin {
				marginModeStr = "逐仓"
			}
			log.Printf("  ✓ %s 仓位模式已是 %s", symbol, marginModeStr)
			return nil
		}
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		return nil // 不返回错误，让交易继续
	}

	marginModeStr := "全仓"
	if !isCrossMargin {
		marginModeStr = "逐仓"
	}
	log.Printf("  ✓ %s 仓位模式已设置为 (%s) %s", symbol, marginMode, marginModeStr)
	return nil
}

// SetLeverage 设置杠杆（智能判断+冷却期）
func (t *GateIOFuturesTrader) SetLeverage(symbol string, leverage int) error {
	// 先尝试获取当前杠杆（从持仓信息）
	currentLeverage := 0
	positions, err := t.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == symbol {
				if lev, ok := pos["leverage"].(float64); ok {
					currentLeverage = int(lev)
					break
				}
			}
		}
	}

	// 如果当前杠杆已经是目标杠杆，跳过
	if currentLeverage == leverage && currentLeverage > 0 {
		log.Printf("  ✓ %s 杠杆已是 %dx，无需切换", symbol, leverage)
		return nil
	}

	gateioSymbol := convertSymbolToGateIO(symbol)
	_, _, err = t.client.FuturesApi.UpdatePositionLeverage(t.ctx, "usdt", gateioSymbol, strconv.Itoa(leverage), nil)

	if err != nil {
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "No need") {
			log.Printf("  ✓ %s 杠杆已是 %dx", symbol, leverage)
			return nil
		}
		return fmt.Errorf("设置杠杆失败: %w", err)
	}

	log.Printf("  ✓ %s 杠杆已切换为 %dx", symbol, leverage)

	// 切换杠杆后等待5秒（避免冷却期错误）
	log.Printf("  ⏱ 等待5秒冷却期...")
	time.Sleep(5 * time.Second)

	return nil
}

// OpenLong 开多仓
func (t *GateIOFuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单（清理旧的止损止盈单）
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败（可能没有委托单）: %v", err)
	}

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}
	if err := t.SetMarginMode(symbol, true); err != nil {
		return nil, err
	}
	// 格式化数量到正确精度
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 将字符串转换为 int64
	quantityInt, err := strconv.ParseInt(strings.Replace(quantityStr, ".", "", -1), 10, 64)
	if err != nil {
		// 如果转换失败，尝试直接转换
		quantityInt = int64(quantity)
	}

	// 创建市价买入订单
	gateioSymbol := convertSymbolToGateIO(symbol)
	order := gateapi.FuturesOrder{
		Text:     "t-my-custom-id",
		StpAct:   "-",
		Contract: gateioSymbol,
		Iceberg:  0,
		Size:     quantityInt,
		Price:    "0",   // 0 表示市价单
		Tif:      "ioc", // Immediate or Cancel
	}

	createdOrder, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", order, nil)
	if err != nil {
		return nil, fmt.Errorf("开多仓失败: symbol: %s quantityStr: %s quantityInt: %d error: %w", gateioSymbol, quantityStr, quantityInt, err)
	}

	log.Printf("✓ 开多仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %d", createdOrder.Id)

	result := make(map[string]interface{})
	result["orderId"] = createdOrder.Id
	result["symbol"] = convertSymbolFromGateIO(createdOrder.Contract)
	result["status"] = createdOrder.Status
	return result, nil
}

// OpenShort 开空仓
func (t *GateIOFuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单（清理旧的止损止盈单）
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败（可能没有委托单）: %v", err)
	}

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	if err := t.SetMarginMode(symbol, true); err != nil {
		return nil, err
	}

	// 格式化数量到正确精度
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 将字符串转换为 int64（负数表示卖出）
	quantityInt, err := strconv.ParseInt(strings.Replace(quantityStr, ".", "", -1), 10, 64)
	if err != nil {
		// 如果转换失败，尝试直接转换
		quantityInt = int64(quantity)
	}
	quantityInt = -quantityInt // 负数表示卖出

	// 创建市价卖出订单（负数表示卖出）
	gateioSymbol := convertSymbolToGateIO(symbol)
	order := gateapi.FuturesOrder{
		Contract: gateioSymbol,
		Size:     quantityInt,
		Price:    "0",   // 0 表示市价单
		Tif:      "ioc", // Immediate or Cancel
		Text:     "t-my-custom-id",
		StpAct:   "-",
		Iceberg:  0,
	}

	createdOrder, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", order, nil)
	if err != nil {
		return nil, fmt.Errorf("开空仓失败: symbol: %s quantityStr: %s quantityInt: %d error: %w", gateioSymbol, quantityStr, quantityInt, err)
	}

	log.Printf("✓ 开空仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %d", createdOrder.Id)

	result := make(map[string]interface{})
	result["orderId"] = createdOrder.Id
	result["symbol"] = convertSymbolFromGateIO(createdOrder.Contract)
	result["status"] = createdOrder.Status
	return result, nil
}

// CloseLong 平多仓
func (t *GateIOFuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的多仓", symbol)
		}
	}

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 将字符串转换为 int64（负数表示卖出）
	quantityInt, err := strconv.ParseInt(strings.Replace(quantityStr, ".", "", -1), 10, 64)
	if err != nil {
		quantityInt = int64(quantity)
	}
	quantityInt = -quantityInt // 负数表示卖出

	// 创建市价卖出订单（平多）
	gateioSymbol := convertSymbolToGateIO(symbol)
	order := gateapi.FuturesOrder{
		Contract: gateioSymbol,
		Size:     0,
		Close:    true,
		Price:    "0",   // 0 表示市价单
		Tif:      "ioc", // Immediate or Cancel
		Text:     "t-my-custom-id",
		StpAct:   "-",
		Iceberg:  0,
	}

	createdOrder, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", order, nil)
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: symbol: %s quantityStr: %s quantityInt: %d error: %w", gateioSymbol, quantityStr, quantityInt, err)
	}

	log.Printf("✓ 平多仓成功: %s 数量: %s", symbol, quantityStr)

	// 平仓后取消该币种的所有挂单（止损止盈单）
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = createdOrder.Id
	result["symbol"] = convertSymbolFromGateIO(createdOrder.Contract)
	result["status"] = createdOrder.Status
	return result, nil
}

// CloseShort 平空仓
func (t *GateIOFuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
		}
	}

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 将字符串转换为 int64（正数表示买入）
	quantityInt, err := strconv.ParseInt(strings.Replace(quantityStr, ".", "", -1), 10, 64)
	if err != nil {
		quantityInt = int64(quantity)
	}

	// 创建市价买入订单（平空）
	gateioSymbol := convertSymbolToGateIO(symbol)
	order := gateapi.FuturesOrder{
		Contract: gateioSymbol,
		Size:     0,
		Close:    true,
		Price:    "0",   // 0 表示市价单
		Tif:      "ioc", // Immediate or Cancel
		Text:     "t-my-custom-id",
		StpAct:   "-",
		Iceberg:  0,
	}

	createdOrder, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", order, nil)
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: symbol: %s quantityStr: %s quantityInt: %d error: %w", gateioSymbol, quantityStr, quantityInt, err)
	}

	log.Printf("✓ 平空仓成功: %s 数量: %s", symbol, quantityStr)

	// 平仓后取消该币种的所有挂单（止损止盈单）
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = createdOrder.Id
	result["symbol"] = convertSymbolFromGateIO(createdOrder.Contract)
	result["status"] = createdOrder.Status
	return result, nil
}

// CancelAllOrders 取消该币种的所有挂单
func (t *GateIOFuturesTrader) CancelAllOrders(symbol string) error {
	gateioSymbol := convertSymbolToGateIO(symbol)
	_, _, err := t.client.FuturesApi.CancelFuturesOrders(t.ctx, "usdt", gateioSymbol, nil)

	if err != nil {
		return fmt.Errorf("取消挂单失败: %w", err)
	}

	log.Printf("  ✓ 已取消 %s 的所有挂单", symbol)
	return nil
}

// CancelStopOrders 取消该币种的止盈/止损单（用于调整止盈止损位置）
func (t *GateIOFuturesTrader) CancelStopOrders(symbol string) error {
	gateioSymbol := convertSymbolToGateIO(symbol)

	// 获取该币种的所有价格触发订单（状态为 "open" 的订单）
	opts := &gateapi.ListPriceTriggeredOrdersOpts{
		Contract: optional.NewString(gateioSymbol),
	}
	orders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "open", opts)
	if err != nil {
		return fmt.Errorf("获取价格触发订单失败: %w", err)
	}

	// 过滤出止盈止损单并取消
	// Gate.io 的止盈/止损单都是价格触发订单，类型为 "close-long-position" 或 "close-short-position"
	canceledCount := 0
	for _, order := range orders {
		// 只取消止盈/止损订单（close-long-position 和 close-short-position）
		if order.OrderType == "close-long-position" || order.OrderType == "close-short-position" {
			orderIdStr := strconv.FormatInt(order.Id, 10)
			_, _, err := t.client.FuturesApi.CancelPriceTriggeredOrder(t.ctx, "usdt", orderIdStr)
			if err != nil {
				log.Printf("  ⚠ 取消订单 %d 失败: %v", order.Id, err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ 已取消 %s 的止盈/止损单 (订单ID: %d, 类型: %s, 规则: %d)",
				symbol, order.Id, order.OrderType, order.Trigger.Rule)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止盈/止损单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止盈/止损单", symbol, canceledCount)
	}

	return nil
}

// CancelStopLossOrders 仅取消止损单（不影响止盈单）
func (t *GateIOFuturesTrader) CancelStopLossOrders(symbol string) error {
	gateioSymbol := convertSymbolToGateIO(symbol)

	// 获取该币种的所有价格触发订单（状态为 "open" 的订单）
	opts := &gateapi.ListPriceTriggeredOrdersOpts{
		Contract: optional.NewString(gateioSymbol),
	}
	orders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "open", opts)
	if err != nil {
		return fmt.Errorf("获取价格触发订单失败: %w", err)
	}

	// 过滤出止损单并取消
	// 止损单规则：
	// - 多仓止损：order_type = "close-long-position" && rule = 2
	// - 空仓止损：order_type = "close-short-position" && rule = 1
	canceledCount := 0
	for _, order := range orders {
		isStopLoss := false

		// 判断是否为止损单
		if order.OrderType == "close-long-position" && order.Trigger.Rule == 2 {
			// 多仓止损
			isStopLoss = true
		} else if order.OrderType == "close-short-position" && order.Trigger.Rule == 1 {
			// 空仓止损
			isStopLoss = true
		}

		if isStopLoss {
			orderIdStr := strconv.FormatInt(order.Id, 10)
			_, _, err := t.client.FuturesApi.CancelPriceTriggeredOrder(t.ctx, "usdt", orderIdStr)
			if err != nil {
				log.Printf("  ⚠ 取消止损单 %d 失败: %v", order.Id, err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ 已取消止损单 (订单ID: %d, 类型: %s, 规则: %d)", order.Id, order.OrderType, order.Trigger.Rule)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止损单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止损单", symbol, canceledCount)
	}

	return nil
}

// CancelTakeProfitOrders 仅取消止盈单（不影响止损单）
func (t *GateIOFuturesTrader) CancelTakeProfitOrders(symbol string) error {
	gateioSymbol := convertSymbolToGateIO(symbol)

	// 获取该币种的所有价格触发订单（状态为 "open" 的订单）
	opts := &gateapi.ListPriceTriggeredOrdersOpts{
		Contract: optional.NewString(gateioSymbol),
	}
	orders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "open", opts)
	if err != nil {
		return fmt.Errorf("获取价格触发订单失败: %w", err)
	}

	// 过滤出止盈单并取消
	// 止盈单规则：
	// - 多仓止盈：order_type = "close-long-position" && rule = 1
	// - 空仓止盈：order_type = "close-short-position" && rule = 2
	canceledCount := 0
	for _, order := range orders {
		isTakeProfit := false

		// 判断是否为止盈单
		if order.OrderType == "close-long-position" && order.Trigger.Rule == 1 {
			// 多仓止盈
			isTakeProfit = true
		} else if order.OrderType == "close-short-position" && order.Trigger.Rule == 2 {
			// 空仓止盈
			isTakeProfit = true
		}

		if isTakeProfit {
			orderIdStr := strconv.FormatInt(order.Id, 10)
			_, _, err := t.client.FuturesApi.CancelPriceTriggeredOrder(t.ctx, "usdt", orderIdStr)
			if err != nil {
				log.Printf("  ⚠ 取消止盈单 %d 失败: %v", order.Id, err)
				continue
			}

			canceledCount++
			log.Printf("  ✓ 已取消止盈单 (订单ID: %d, 类型: %s, 规则: %d)", order.Id, order.OrderType, order.Trigger.Rule)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止盈单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止盈单", symbol, canceledCount)
	}

	return nil
}

// GetMarketPrice 获取市场价格
func (t *GateIOFuturesTrader) GetMarketPrice(symbol string) (float64, error) {
	gateioSymbol := convertSymbolToGateIO(symbol)
	opts := &gateapi.ListFuturesTickersOpts{
		Contract: optional.NewString(gateioSymbol),
	}
	tickers, _, err := t.client.FuturesApi.ListFuturesTickers(t.ctx, "usdt", opts)

	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("未找到 %s 的价格", symbol)
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// CalculatePositionSize 计算仓位大小
func (t *GateIOFuturesTrader) CalculatePositionSize(balance, riskPercent, price float64, leverage int) float64 {
	riskAmount := balance * (riskPercent / 100.0)
	positionValue := riskAmount * float64(leverage)
	quantity := positionValue / price
	return quantity
}

// SetStopLoss 设置止损单
func (t *GateIOFuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	// 格式化数量和价格
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 将字符串转换为 int64
	quantityInt, err := strconv.ParseInt(strings.Replace(quantityStr, ".", "", -1), 10, 64)
	if err != nil {
		quantityInt = int64(quantity)
	}

	stopPriceStr := fmt.Sprintf("%.8f", stopPrice)

	// Gate.io 使用价格触发订单（Price Triggered Order）
	gateioSymbol := convertSymbolToGateIO(symbol)

	// 根据持仓方向确定订单大小（多仓止损=卖出，空仓止损=买入）
	var orderSize int64
	if positionSide == "LONG" { //  多仓止损=卖出
		orderSize = -quantityInt // 卖出
	} else { // 空仓止损=买入
		orderSize = quantityInt // 买入
	}
	var rule int32
	// 1: Trigger.Price must > last_price
	// 2: Trigger.Price must < last_price
	if positionSide == "LONG" {
		// 多仓止损=卖出
		rule = 2
	} else {
		// 空仓止损=买入
		rule = 1
	}

	var order_type string
	if positionSide == "LONG" {
		// 仓位止盈止损，用于全部平多仓
		order_type = "close-long-position"
	} else {
		// 仓位止盈止损，用于全部平空仓
		order_type = "close-short-position"
	}

	order := gateapi.FuturesPriceTriggeredOrder{
		OrderType: order_type,
		Trigger: gateapi.FuturesPriceTrigger{
			StrategyType: 0, // 0: Price trigger
			PriceType:    0, // 0: Latest trade price
			Price:        stopPriceStr,
			Rule:         rule, // 2: Trigger when price <= Trigger.Price (止损)
			Expiration:   0,    // 0: Never expire
		},
		Initial: gateapi.FuturesInitialOrder{
			Contract:   gateioSymbol,
			Size:       orderSize,
			Price:      "0",
			Tif:        "ioc", // Immediate or Cancel
			ReduceOnly: true,  // 止损单应该是只减仓
		},
	}

	_, _, err = t.client.FuturesApi.CreatePriceTriggeredOrder(t.ctx, "usdt", order)
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	log.Printf("  止损价设置: %.4f", stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *GateIOFuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	// 格式化数量和价格
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 将字符串转换为 int64
	quantityInt, err := strconv.ParseInt(strings.Replace(quantityStr, ".", "", -1), 10, 64)
	if err != nil {
		quantityInt = int64(quantity)
	}

	takeProfitPriceStr := fmt.Sprintf("%.8f", takeProfitPrice)

	// Gate.io 使用价格触发订单（Price Triggered Order）
	gateioSymbol := convertSymbolToGateIO(symbol)

	// 根据持仓方向确定订单大小（多仓止盈=卖出，空仓止盈=买入）
	var orderSize int64
	if positionSide == "LONG" {
		orderSize = -quantityInt // 卖出
	} else {
		orderSize = quantityInt // 买入
	}
	var rule int32

	// 1: Trigger.Price must > last_price
	// 2: Trigger.Price must < last_price
	if positionSide == "LONG" {
		// 多仓止盈=卖出
		rule = 1
	} else {
		// 空仓止盈=买入
		rule = 2
	}

	var order_type string
	if positionSide == "LONG" {
		order_type = "close-long-position" // 1: Trigger when price >= Trigger.Price (止盈)
	} else {
		order_type = "close-short-position" // 2: Trigger when price <= Trigger.Price (止损)
	}
	order := gateapi.FuturesPriceTriggeredOrder{
		OrderType: order_type,
		Trigger: gateapi.FuturesPriceTrigger{
			StrategyType: 0, // 0: Price trigger
			PriceType:    0, // 0: Latest trade price
			Price:        takeProfitPriceStr,
			Rule:         rule, // 1: Trigger when price >= Trigger.Price (止盈)
			Expiration:   0,    // 0: Never expire
		},
		Initial: gateapi.FuturesInitialOrder{
			Contract:   gateioSymbol,
			Size:       orderSize,
			Price:      "0",
			Tif:        "ioc", // Immediate or Cancel
			ReduceOnly: true,  // 止盈单应该是只减仓
		},
	}
	_, _, err = t.client.FuturesApi.CreatePriceTriggeredOrder(t.ctx, "usdt", order)
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	log.Printf("  止盈价设置: %.4f", takeProfitPrice)
	return nil
}

// GetSymbolPrecision 获取交易对的数量精度
func (t *GateIOFuturesTrader) GetSymbolPrecision(symbol string) (int, error) {
	// 先检查缓存
	t.precisionMutex.RLock()
	if prec, ok := t.symbolPrecision[symbol]; ok {
		t.precisionMutex.RUnlock()
		return prec.QuantityPrecision, nil
	}
	t.precisionMutex.RUnlock()

	// 获取交易对信息
	gateioSymbol := convertSymbolToGateIO(symbol)
	contracts, _, err := t.client.FuturesApi.ListFuturesContracts(t.ctx, "usdt", nil)

	// 查找指定的合约
	var contract *gateapi.Contract
	for _, c := range contracts {
		if c.Name == gateioSymbol {
			contract = &c
			break
		}
	}

	if err != nil {
		log.Printf("  ⚠ %s 未找到精度信息，使用默认精度3", symbol)
		return 3, nil // 默认精度为3
	}

	if contract == nil {
		log.Printf("  ⚠ %s 未找到精度信息，使用默认精度3", symbol)
		return 3, nil
	}

	// 从 OrderPriceRound 计算价格精度
	pricePrecision := 2 // 默认精度
	if contract.OrderPriceRound != "" {
		// 从 OrderPriceRound 计算精度（例如 "0.01" -> 2位小数）
		roundValue, _ := strconv.ParseFloat(contract.OrderPriceRound, 64)
		if roundValue > 0 {
			pricePrecision = calculatePrecisionFromStep(roundValue)
		}
	}

	// 从 OrderSizeMin 计算数量精度
	sizePrecision := 3 // 默认精度
	if contract.QuantoMultiplier != "" {
		// 从 OrderSizeMin 计算精度（例如 0.001 -> 3位小数）
		quantoMultiplier, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
		sizePrecision = calculatePrecisionFromStep(quantoMultiplier)
	}

	// 缓存精度信息
	t.precisionMutex.Lock()
	t.symbolPrecision[symbol] = SymbolPrecision{
		PricePrecision:    pricePrecision,
		QuantityPrecision: sizePrecision,
	}
	t.precisionMutex.Unlock()

	log.Printf("  %s 数量精度: %d", symbol, sizePrecision)
	return sizePrecision, nil
}

// calculatePrecisionFromStep 从步进值计算精度
func calculatePrecisionFromStep(step float64) int {
	precision := 0
	for step < 1.0 {
		step *= 10
		precision++
		if precision >= 10 {
			break
		}
	}
	return precision
}

// FormatQuantity 格式化数量到正确的精度
func (t *GateIOFuturesTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	precision, err := t.GetSymbolPrecision(symbol)
	if err != nil {
		// 如果获取失败，使用默认格式
		return fmt.Sprintf("%.3f", quantity), nil
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity), nil
}

// FormatQuantity 格式化数量到正确的精度
func (t *GateIOFuturesTrader) FormatQuantityToFloat64(symbol string, quantity float64) (float64, error) {
	precision, err := t.GetSymbolPrecision(symbol)
	if err != nil {
		// 如果获取失败，使用默认格式
		return quantity, nil
	}
	multiplier := 1.0
	if precision > 0 {
		for i := 0; i < precision; i++ {
			multiplier /= 10.0
		}
	}
	format := fmt.Sprintf("%%.%df", precision)
	return strconv.ParseFloat(fmt.Sprintf(format, quantity*multiplier), 64)
}
