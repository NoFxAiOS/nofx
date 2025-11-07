package trader

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sonirico/go-hyperliquid"
)

// HyperliquidTrader Hyperliquid交易器
type HyperliquidTrader struct {
	exchange      *hyperliquid.Exchange
	ctx           context.Context
	walletAddr    string
	meta          *hyperliquid.Meta // 缓存meta信息（包含精度等）
	isCrossMargin bool              // 是否为全仓模式
}

// NewHyperliquidTrader 创建Hyperliquid交易器
func NewHyperliquidTrader(privateKeyHex string, walletAddr string, testnet bool) (*HyperliquidTrader, error) {
	// 去掉私钥的 0x 前缀（如果有，不区分大小写）
	privateKeyHex = strings.TrimPrefix(strings.ToLower(privateKeyHex), "0x")

	// 解析私钥
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// 选择API URL
	apiURL := hyperliquid.MainnetAPIURL
	if testnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// 从私钥生成钱包地址（如果未提供）
	if walletAddr == "" {
		pubKey := privateKey.Public()
		publicKeyECDSA, ok := pubKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("无法转换公钥")
		}
		walletAddr = crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
		log.Printf("✓ 从私钥自动生成钱包地址: %s", walletAddr)
	} else {
		log.Printf("✓ 使用提供的钱包地址: %s", walletAddr)
	}

	ctx := context.Background()

	// 创建Exchange客户端（Exchange包含Info功能）
	exchange := hyperliquid.NewExchange(
		ctx,
		privateKey,
		apiURL,
		nil,        // Meta will be fetched automatically
		"",         // vault address (empty for personal account)
		walletAddr, // wallet address
		nil,        // SpotMeta will be fetched automatically
	)

	log.Printf("✓ Hyperliquid交易器初始化成功 (testnet=%v, wallet=%s)", testnet, walletAddr)

	// 获取meta信息（包含精度等配置）
	meta, err := exchange.Info().Meta(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取meta信息失败: %w", err)
	}

	return &HyperliquidTrader{
		exchange:      exchange,
		ctx:           ctx,
		walletAddr:    walletAddr,
		meta:          meta,
		isCrossMargin: true, // 默认使用全仓模式
	}, nil
}

// GetBalance 获取账户余额（同时查询 Spot 和 Perpetuals）
func (t *HyperliquidTrader) GetBalance() (map[string]interface{}, error) {
	log.Printf("🔄 正在调用Hyperliquid API获取账户余额...")

	// ✅ 第一步：查询 Spot 现货余额（用户的 USDC/USDT 可能在这里）
	spotState, err := t.exchange.Info().SpotUserState(t.ctx, t.walletAddr)
	var spotUSDCBalance float64 = 0.0
	if err != nil {
		log.Printf("⚠️ 查询 Spot 余额失败（可能无现货资产）: %v", err)
	} else if spotState != nil && len(spotState.Balances) > 0 {
		// 查找 USDC 余额（Hyperliquid 现货主要使用 USDC）
		for _, balance := range spotState.Balances {
			if balance.Coin == "USDC" {
				spotUSDCBalance, _ = strconv.ParseFloat(balance.Total, 64)
				log.Printf("✓ 发现 Spot 现货余额: %.2f USDC", spotUSDCBalance)
				break
			}
		}
	}

	// ✅ 第二步：查询 Perpetuals 合约余额
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		log.Printf("❌ Hyperliquid Perpetuals API调用失败: %v", err)
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}

	// 解析余额信息（MarginSummary字段都是string）
	result := make(map[string]interface{})

	// ✅ 核心修复：根据保证金模式动态选择正确的摘要（CrossMarginSummary 或 MarginSummary）
	var accountValue, totalMarginUsed float64
	var summaryType string
	var summary interface{}

	if t.isCrossMargin {
		// 全仓模式：使用 CrossMarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.CrossMarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.CrossMarginSummary.TotalMarginUsed, 64)
		summaryType = "CrossMarginSummary (全仓)"
		summary = accountState.CrossMarginSummary
	} else {
		// 逐仓模式：使用 MarginSummary
		accountValue, _ = strconv.ParseFloat(accountState.MarginSummary.AccountValue, 64)
		totalMarginUsed, _ = strconv.ParseFloat(accountState.MarginSummary.TotalMarginUsed, 64)
		summaryType = "MarginSummary (逐仓)"
		summary = accountState.MarginSummary
	}

	// 🔍 调试：打印API返回的完整摘要结构
	summaryJSON, _ := json.MarshalIndent(summary, "  ", "  ")
	log.Printf("🔍 [DEBUG] Hyperliquid API %s 完整数据:", summaryType)
	log.Printf("%s", string(summaryJSON))

	// ⚠️ 关键修复：从所有持仓中累加真正的未实现盈亏
	totalUnrealizedPnl := 0.0
	for _, assetPos := range accountState.AssetPositions {
		unrealizedPnl, _ := strconv.ParseFloat(assetPos.Position.UnrealizedPnl, 64)
		totalUnrealizedPnl += unrealizedPnl
	}

	// ✅ 正确理解Hyperliquid字段：
	// AccountValue = 总账户净值（已包含空闲资金+持仓价值+未实现盈亏）
	// TotalMarginUsed = 持仓占用的保证金（已包含在AccountValue中，仅用于显示）
	//
	// 为了兼容auto_trader.go的计算逻辑（totalEquity = totalWalletBalance + totalUnrealizedProfit）
	// 需要返回"不包含未实现盈亏的钱包余额"
	walletBalanceWithoutUnrealized := accountValue - totalUnrealizedPnl

	// ✅ Step 4: 使用 Withdrawable 欄位（PR #443）
	// Withdrawable 是官方提供的真实可提现余额，比简单计算更可靠
	availableBalance := 0.0
	if accountState.Withdrawable != "" {
		withdrawable, err := strconv.ParseFloat(accountState.Withdrawable, 64)
		if err == nil && withdrawable > 0 {
			availableBalance = withdrawable
			log.Printf("✓ 使用 Withdrawable 作为可用余额: %.2f", availableBalance)
		}
	}

	// 降级方案：如果没有 Withdrawable，使用简单计算
	if availableBalance == 0 && accountState.Withdrawable == "" {
		availableBalance = accountValue - totalMarginUsed
		if availableBalance < 0 {
			log.Printf("⚠️ 计算出的可用余额为负数 (%.2f)，重置为 0", availableBalance)
			availableBalance = 0
		}
	}

	// ✅ Step 5: 正确处理 Spot + Perpetuals 余额
	// 重要：Spot 只加到总资产，不加到可用余额
	//      原因：Spot 和 Perpetuals 是独立帐户，需手动 ClassTransfer 才能转账
	totalWalletBalance := walletBalanceWithoutUnrealized + spotUSDCBalance

	result["totalWalletBalance"] = totalWalletBalance    // 总资产（Perp + Spot）
	result["availableBalance"] = availableBalance        // 可用余额（仅 Perpetuals，不含 Spot）
	result["totalUnrealizedProfit"] = totalUnrealizedPnl // 未实现盈亏（仅来自 Perpetuals）
	result["spotBalance"] = spotUSDCBalance              // Spot 现货余额（单独返回）

	log.Printf("✓ Hyperliquid 完整账户:")
	log.Printf("  • Spot 现货余额: %.2f USDC （需手动转账到 Perpetuals 才能开仓）", spotUSDCBalance)
	log.Printf("  • Perpetuals 合约净值: %.2f USDC (钱包%.2f + 未实现%.2f)",
		accountValue,
		walletBalanceWithoutUnrealized,
		totalUnrealizedPnl)
	log.Printf("  • Perpetuals 可用余额: %.2f USDC （可直接用于开仓）", availableBalance)
	log.Printf("  • 保证金占用: %.2f USDC", totalMarginUsed)
	log.Printf("  • 总资产 (Perp+Spot): %.2f USDC", totalWalletBalance)
	log.Printf("  ⭐ 总资产: %.2f USDC | Perp 可用: %.2f USDC | Spot 余额: %.2f USDC",
		totalWalletBalance, availableBalance, spotUSDCBalance)

	return result, nil
}

// GetPositions 获取所有持仓
func (t *HyperliquidTrader) GetPositions() ([]map[string]interface{}, error) {
	// 获取账户状态
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}

	// 遍历所有持仓
	for _, assetPos := range accountState.AssetPositions {
		position := assetPos.Position

		// 持仓数量（string类型）
		posAmt, _ := strconv.ParseFloat(position.Szi, 64)

		if posAmt == 0 {
			continue // 跳过无持仓的
		}

		posMap := make(map[string]interface{})

		// 标准化symbol格式（Hyperliquid使用如"BTC"，我们转换为"BTCUSDT"）
		symbol := position.Coin + "USDT"
		posMap["symbol"] = symbol

		// 持仓数量和方向
		if posAmt > 0 {
			posMap["side"] = "long"
			posMap["positionAmt"] = posAmt
		} else {
			posMap["side"] = "short"
			posMap["positionAmt"] = -posAmt // 转为正数
		}

		// 价格信息（EntryPx和LiquidationPx是指针类型）
		var entryPrice, liquidationPx float64
		if position.EntryPx != nil {
			entryPrice, _ = strconv.ParseFloat(*position.EntryPx, 64)
		}
		if position.LiquidationPx != nil {
			liquidationPx, _ = strconv.ParseFloat(*position.LiquidationPx, 64)
		}

		positionValue, _ := strconv.ParseFloat(position.PositionValue, 64)
		unrealizedPnl, _ := strconv.ParseFloat(position.UnrealizedPnl, 64)

		// 计算mark price（positionValue / abs(posAmt)）
		var markPrice float64
		if posAmt != 0 {
			markPrice = positionValue / absFloat(posAmt)
		}

		posMap["entryPrice"] = entryPrice
		posMap["markPrice"] = markPrice
		posMap["unRealizedProfit"] = unrealizedPnl
		posMap["leverage"] = float64(position.Leverage.Value)
		posMap["liquidationPrice"] = liquidationPx

		result = append(result, posMap)
	}

	return result, nil
}

// SetMarginMode 设置仓位模式 (在SetLeverage时一并设置)
func (t *HyperliquidTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Hyperliquid的仓位模式在SetLeverage时设置，这里只记录
	t.isCrossMargin = isCrossMargin
	marginModeStr := "全仓"
	if !isCrossMargin {
		marginModeStr = "逐仓"
	}
	log.Printf("  ✓ %s 将使用 %s 模式", symbol, marginModeStr)
	return nil
}

// SetLeverage 设置杠杆
func (t *HyperliquidTrader) SetLeverage(symbol string, leverage int) error {
	// Hyperliquid symbol格式（去掉USDT后缀）
	coin := convertSymbolToHyperliquid(symbol)

	// 调用UpdateLeverage (leverage int, name string, isCross bool)
	// 第三个参数: true=全仓模式, false=逐仓模式
	_, err := t.exchange.UpdateLeverage(t.ctx, leverage, coin, t.isCrossMargin)
	if err != nil {
		return fmt.Errorf("设置杠杆失败: %w", err)
	}

	log.Printf("  ✓ %s 杠杆已切换为 %dx", symbol, leverage)
	return nil
}

// OpenLong 开多仓
func (t *HyperliquidTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败: %v", err)
	}

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Hyperliquid symbol格式
	coin := convertSymbolToHyperliquid(symbol)

	// ✅ 关键检查：验证 Asset ID（防止 Meta 未初始化导致 asset=0 错误）
	assetID := t.exchange.Info().NameToAsset(coin)
	if assetID == 0 {
		return nil, fmt.Errorf("❌ 开多仓失败: 资产 %s 的 Asset ID 为 0，Meta 信息可能未正确初始化。请检查 Hyperliquid API 连接或稍后重试", coin)
	}
	log.Printf("  ✓ Asset ID 检查通过: %s -> %d", coin, assetID)

	// 获取当前价格（用于市价单）
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ 关键：根据币种精度要求，四舍五入数量
	szDecimals := t.getSzDecimals(coin)
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 数量精度处理: %.8f -> %.8f (szDecimals=%d, 币种=%s)",
		quantity, roundedQuantity, szDecimals, coin)

	// ✅ 安全检查：确保数量不为 0
	if roundedQuantity == 0 {
		return nil, fmt.Errorf("❌ 开多仓失败: 计算的数量太小（原始=%.8f, 四舍五入后=0），请增加保证金或降低杠杆",
			quantity)
	}

	// ⚠️ 关键：价格也需要处理为5位有效数字
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	log.Printf("  💰 价格精度处理: %.8f -> %.8f (5位有效数字)", price*1.01, aggressivePrice)

	// 创建市价买入订单（使用IOC limit order with aggressive price）
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity, // 使用四舍五入后的数量
		Price: aggressivePrice, // 使用处理后的价格
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc, // Immediate or Cancel (类似市价单)
			},
		},
		ReduceOnly: false,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}

	log.Printf("✓ 开多仓成功: %s 数量: %.4f", symbol, roundedQuantity)

	result := make(map[string]interface{})
	result["orderId"] = 0 // Hyperliquid没有返回order ID
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// OpenShort 开空仓
func (t *HyperliquidTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败: %v", err)
	}

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Hyperliquid symbol格式
	coin := convertSymbolToHyperliquid(symbol)

	// ✅ 关键检查：验证 Asset ID（防止 Meta 未初始化导致 asset=0 错误）
	assetID := t.exchange.Info().NameToAsset(coin)
	if assetID == 0 {
		return nil, fmt.Errorf("❌ 开空仓失败: 资产 %s 的 Asset ID 为 0，Meta 信息可能未正确初始化。请检查 Hyperliquid API 连接或稍后重试", coin)
	}
	log.Printf("  ✓ Asset ID 检查通过: %s -> %d", coin, assetID)

	// 获取当前价格
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ 关键：根据币种精度要求，四舍五入数量
	szDecimals := t.getSzDecimals(coin)
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 数量精度处理: %.8f -> %.8f (szDecimals=%d, 币种=%s)",
		quantity, roundedQuantity, szDecimals, coin)

	// ✅ 安全检查：确保数量不为 0
	if roundedQuantity == 0 {
		return nil, fmt.Errorf("❌ 开空仓失败: 计算的数量太小（原始=%.8f, 四舍五入后=0），请增加保证金或降低杠杆",
			quantity)
	}

	// ⚠️ 关键：价格也需要处理为5位有效数字
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	log.Printf("  💰 价格精度处理: %.8f -> %.8f (5位有效数字)", price*0.99, aggressivePrice)

	// 创建市价卖出订单
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity, // 使用四舍五入后的数量
		Price: aggressivePrice, // 使用处理后的价格
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: false,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	log.Printf("✓ 开空仓成功: %s 数量: %.4f", symbol, roundedQuantity)

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseLong 平多仓
func (t *HyperliquidTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
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

	// Hyperliquid symbol格式
	coin := convertSymbolToHyperliquid(symbol)

	// 获取当前价格
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ 关键：根据币种精度要求，四舍五入数量
	szDecimals := t.getSzDecimals(coin)
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 数量精度处理: %.8f -> %.8f (szDecimals=%d, 币种=%s)",
		quantity, roundedQuantity, szDecimals, coin)

	// ✅ 安全检查：确保数量不为 0
	if roundedQuantity == 0 {
		return nil, fmt.Errorf("❌ 平多仓失败: 持仓数量太小（原始=%.8f, 四舍五入后=0），无法平仓",
			quantity)
	}

	// ⚠️ 关键：价格也需要处理为5位有效数字
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	log.Printf("  💰 价格精度处理: %.8f -> %.8f (5位有效数字)", price*0.99, aggressivePrice)

	// 创建平仓订单（卖出 + ReduceOnly）
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity, // 使用四舍五入后的数量
		Price: aggressivePrice, // 使用处理后的价格
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true, // 只平仓，不开新仓
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: %w", err)
	}

	log.Printf("✓ 平多仓成功: %s 数量: %.4f", symbol, roundedQuantity)

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseShort 平空仓
func (t *HyperliquidTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
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

	// Hyperliquid symbol格式
	coin := convertSymbolToHyperliquid(symbol)

	// 获取当前价格
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// ⚠️ 关键：根据币种精度要求，四舍五入数量
	szDecimals := t.getSzDecimals(coin)
	roundedQuantity := t.roundToSzDecimals(coin, quantity)
	log.Printf("  📏 数量精度处理: %.8f -> %.8f (szDecimals=%d, 币种=%s)",
		quantity, roundedQuantity, szDecimals, coin)

	// ✅ 安全检查：确保数量不为 0
	if roundedQuantity == 0 {
		return nil, fmt.Errorf("❌ 平空仓失败: 持仓数量太小（原始=%.8f, 四舍五入后=0），无法平仓",
			quantity)
	}

	// ⚠️ 关键：价格也需要处理为5位有效数字
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	log.Printf("  💰 价格精度处理: %.8f -> %.8f (5位有效数字)", price*1.01, aggressivePrice)

	// 创建平仓订单（买入 + ReduceOnly）
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity, // 使用四舍五入后的数量
		Price: aggressivePrice, // 使用处理后的价格
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true,
	}

	_, err = t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: %w", err)
	}

	log.Printf("✓ 平空仓成功: %s 数量: %.4f", symbol, roundedQuantity)

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CancelAllOrders 取消该币种的所有挂单
func (t *HyperliquidTrader) CancelAllOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// 获取所有挂单
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("获取挂单失败: %w", err)
	}

	// 取消该币种的所有挂单
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				log.Printf("  ⚠ 取消订单失败 (oid=%d): %v", order.Oid, err)
			}
		}
	}

	log.Printf("  ✓ 已取消 %s 的所有挂单", symbol)
	return nil
}

// CancelStopOrders 取消该币种的止盈/止损单（用于调整止盈止损位置）
func (t *HyperliquidTrader) CancelStopOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// 获取所有挂单
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("获取挂单失败: %w", err)
	}

	// 注意：Hyperliquid SDK 的 OpenOrder 结构不暴露 trigger 字段
	// 因此暂时取消该币种的所有挂单（包括止盈止损单）
	// 这是安全的，因为在设置新的止盈止损之前，应该清理所有旧订单
	canceledCount := 0
	triggeredCount := 0
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				errMsg := err.Error()

				// ============ P3 修复：智能错误分类 ============
				// 判断错误类型：订单已触发 vs 真实错误
				if strings.Contains(errMsg, "Order does not exist") ||
					strings.Contains(errMsg, "already filled") ||
					strings.Contains(errMsg, "already triggered") ||
					strings.Contains(errMsg, "Order not found") {
					// 订单已触发/成交/不存在 → 这是正常情况，不是错误
					log.Printf("  ℹ️  订单 oid=%d 已触发或成交，无需取消", order.Oid)
					triggeredCount++
					continue
				}

				if strings.Contains(errMsg, "permission") ||
					strings.Contains(errMsg, "Unauthorized") ||
					strings.Contains(errMsg, "API key") {
					// 权限错误 → 这是严重问题
					log.Printf("  🚨 权限错误 (oid=%d): %v", order.Oid, err)
					log.Printf("  → 请检查 API Key 配置或权限设置")
					continue
				}

				// 其他未知错误 → 记录但继续
				log.Printf("  ⚠️  取消订单失败 (oid=%d): %v", order.Oid, err)
				// ===================================================
				continue
			}
			canceledCount++
		}
	}

	// ============ P3 修复：详细的操作摘要 ============
	if canceledCount == 0 && triggeredCount == 0 {
		log.Printf("  ℹ️  %s 没有挂单需要取消", symbol)
	} else {
		if canceledCount > 0 && triggeredCount > 0 {
			log.Printf("  ✅ 已取消 %s 的 %d 个挂单，%d 个已触发/成交", symbol, canceledCount, triggeredCount)
		} else if canceledCount > 0 {
			log.Printf("  ✅ 已取消 %s 的 %d 个挂单（包括止盈/止损单）", symbol, canceledCount)
		} else {
			log.Printf("  ℹ️  %s 的 %d 个订单已触发或成交，无需取消", symbol, triggeredCount)
		}
	}
	// ===================================================

	return nil
}

// CancelStopLossOrders 仅取消止损单（Hyperliquid 暂无法区分止损和止盈，取消所有）
func (t *HyperliquidTrader) CancelStopLossOrders(symbol string) error {
	// Hyperliquid SDK 的 OpenOrder 结构不暴露 trigger 字段
	// 无法区分止损和止盈单，因此取消该币种的所有挂单
	log.Printf("  ⚠️ Hyperliquid 无法区分止损/止盈单，将取消所有挂单")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders 仅取消止盈单（Hyperliquid 暂无法区分止损和止盈，取消所有）
func (t *HyperliquidTrader) CancelTakeProfitOrders(symbol string) error {
	// Hyperliquid SDK 的 OpenOrder 结构不暴露 trigger 字段
	// 无法区分止损和止盈单，因此取消该币种的所有挂单
	log.Printf("  ⚠️ Hyperliquid 无法区分止损/止盈单，将取消所有挂单")
	return t.CancelStopOrders(symbol)
}

// GetMarketPrice 获取市场价格
func (t *HyperliquidTrader) GetMarketPrice(symbol string) (float64, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// 获取所有市场价格
	allMids, err := t.exchange.Info().AllMids(t.ctx)
	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	// 查找对应币种的价格（allMids是map[string]string）
	if priceStr, ok := allMids[coin]; ok {
		priceFloat, err := strconv.ParseFloat(priceStr, 64)
		if err == nil {
			return priceFloat, nil
		}
		return 0, fmt.Errorf("价格格式错误: %v", err)
	}

	return 0, fmt.Errorf("未找到 %s 的价格", symbol)
}

// SetStopLoss 设置止损单
func (t *HyperliquidTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // 空仓止损=买入，多仓止损=卖出

	// ⚠️ 关键：根据币种精度要求，四舍五入数量
	roundedQuantity := t.roundToSzDecimals(coin, quantity)

	// ✅ 安全检查：确保数量不为 0
	if roundedQuantity == 0 {
		return fmt.Errorf("❌ 设置止损失败: 持仓数量太小（原始=%.8f, 四舍五入后=0）", quantity)
	}

	// ⚠️ 关键：价格也需要处理为5位有效数字
	roundedStopPrice := t.roundPriceToSigfigs(stopPrice)

	// 创建止损单（Trigger Order）
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,  // 使用四舍五入后的数量
		Price: roundedStopPrice, // 使用处理后的价格
		OrderType: hyperliquid.OrderType{
			Trigger: &hyperliquid.TriggerOrderType{
				TriggerPx: roundedStopPrice,
				IsMarket:  true,
				Tpsl:      "sl", // stop loss
			},
		},
		ReduceOnly: true,
	}

	_, err := t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	log.Printf("  止损价设置: %.4f", roundedStopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *HyperliquidTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // 空仓止盈=买入，多仓止盈=卖出

	// ⚠️ 关键：根据币种精度要求，四舍五入数量
	roundedQuantity := t.roundToSzDecimals(coin, quantity)

	// ✅ 安全检查：确保数量不为 0
	if roundedQuantity == 0 {
		return fmt.Errorf("❌ 设置止盈失败: 持仓数量太小（原始=%.8f, 四舍五入后=0）", quantity)
	}

	// ⚠️ 关键：价格也需要处理为5位有效数字
	roundedTakeProfitPrice := t.roundPriceToSigfigs(takeProfitPrice)

	// 创建止盈单（Trigger Order）
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,        // 使用四舍五入后的数量
		Price: roundedTakeProfitPrice, // 使用处理后的价格
		OrderType: hyperliquid.OrderType{
			Trigger: &hyperliquid.TriggerOrderType{
				TriggerPx: roundedTakeProfitPrice,
				IsMarket:  true,
				Tpsl:      "tp", // take profit
			},
		},
		ReduceOnly: true,
	}

	_, err := t.exchange.Order(t.ctx, order, nil)
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	log.Printf("  止盈价设置: %.4f", roundedTakeProfitPrice)
	return nil
}

// FormatQuantity 格式化数量到正确的精度
func (t *HyperliquidTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	coin := convertSymbolToHyperliquid(symbol)
	szDecimals := t.getSzDecimals(coin)

	// 使用szDecimals格式化数量
	formatStr := fmt.Sprintf("%%.%df", szDecimals)
	return fmt.Sprintf(formatStr, quantity), nil
}

// getSzDecimals 获取币种的数量精度
func (t *HyperliquidTrader) getSzDecimals(coin string) int {
	if t.meta == nil {
		log.Printf("❌ [Meta診斷] meta 信息为空！可能原因：")
		log.Printf("   1. exchange.Info().Meta() 调用失败")
		log.Printf("   2. 网络问题导致 API 请求失败")
		log.Printf("   → 使用默认精度 4，可能导致订单失败")
		return 4 // 默认精度
	}

	// 在meta.Universe中查找对应的币种
	for _, asset := range t.meta.Universe {
		if asset.Name == coin {
			log.Printf("✓ [Meta診斷] 找到 %s 配置：szDecimals=%d", coin, asset.SzDecimals)
			return asset.SzDecimals
		}
	}

	log.Printf("⚠️  [Meta診斷] 未在 meta.Universe 中找到 %s，使用默认精度4", coin)
	log.Printf("   meta.Universe 包含 %d 个币种", len(t.meta.Universe))
	return 4 // 默认精度
}

// roundToSzDecimals 将数量四舍五入到正确的精度
func (t *HyperliquidTrader) roundToSzDecimals(coin string, quantity float64) float64 {
	szDecimals := t.getSzDecimals(coin)

	// 计算倍数（10^szDecimals）
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}

	// 四舍五入
	rounded := float64(int(quantity*multiplier+0.5)) / multiplier

	// ⚠️ 关键修复：检测四舍五入后是否变成 0
	if rounded == 0 && quantity > 0 {
		minUnit := 1.0 / multiplier
		log.Printf("❌ [精度问题] %s 数量 %.8f 四舍五入后变成 0（szDecimals=%d）",
			coin, quantity, szDecimals)
		log.Printf("   建议：")
		log.Printf("   1. 增加保证金（至少需要 %.2f 倍）", minUnit/quantity)
		log.Printf("   2. 降低杠杆")
		log.Printf("   3. 检查 Meta 信息是否正确加载（szDecimals 可能错误）")
		// 注意：不再强制使用最小单位，而是返回 0 让后续检查捕获
		return 0
	}

	return rounded
}

// roundPriceToSigfigs 将价格四舍五入到5位有效数字
// Hyperliquid要求价格使用5位有效数字（significant figures）
func (t *HyperliquidTrader) roundPriceToSigfigs(price float64) float64 {
	if price == 0 {
		return 0
	}

	const sigfigs = 5 // Hyperliquid标准：5位有效数字

	// 计算价格的数量级
	var magnitude float64
	if price < 0 {
		magnitude = -price
	} else {
		magnitude = price
	}

	// 计算需要的倍数
	multiplier := 1.0
	for magnitude >= 10 {
		magnitude /= 10
		multiplier /= 10
	}
	for magnitude < 1 {
		magnitude *= 10
		multiplier *= 10
	}

	// 应用有效数字精度
	for i := 0; i < sigfigs-1; i++ {
		multiplier *= 10
	}

	// 四舍五入
	rounded := float64(int(price*multiplier+0.5)) / multiplier
	return rounded
}

// convertSymbolToHyperliquid 将标准symbol转换为Hyperliquid格式
// 例如: "BTCUSDT" -> "BTC"
func convertSymbolToHyperliquid(symbol string) string {
	// 去掉USDT后缀
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	return symbol
}

// absFloat 返回浮点数的绝对值
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
