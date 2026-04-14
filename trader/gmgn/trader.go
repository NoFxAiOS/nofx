package gmgn

import (
	"fmt"
	"math"
	"nofx/logger"
	gmgnprovider "nofx/provider/gmgn"
	"nofx/trader/types"
	"strings"
	"time"
)

const (
	defaultSlippage     = 0.03
	stopLossOrderType   = "stop_loss"
	takeProfitOrderType = "take_profit"
)

type Trader struct {
	client        *gmgnprovider.Client
	chain         string
	walletAddress string
	chainConfig   gmgnprovider.ChainConfig
}

func NewTrader(apiKey, privateKey, chain, walletAddress string) (*Trader, error) {
	normalizedChain := gmgnprovider.NormalizeChain(chain)
	cfg, ok := gmgnprovider.GetChainConfig(normalizedChain)
	if !ok {
		return nil, fmt.Errorf("unsupported gmgn chain: %s", chain)
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("gmgn api key is required")
	}
	if strings.TrimSpace(walletAddress) == "" {
		return nil, fmt.Errorf("gmgn wallet address is required")
	}
	client, err := gmgnprovider.NewClient(apiKey, privateKey)
	if err != nil {
		return nil, err
	}
	return &Trader{
		client:        client,
		chain:         normalizedChain,
		walletAddress: strings.TrimSpace(walletAddress),
		chainConfig:   cfg,
	}, nil
}

func (t *Trader) GetBalance() (map[string]interface{}, error) {
	userInfo, err := t.client.GetUserInfo()
	if err != nil {
		return nil, err
	}

	wallet, ok := findWallet(userInfo, t.chain, t.walletAddress)
	if !ok {
		return nil, fmt.Errorf("gmgn wallet not found for chain=%s address=%s", t.chain, t.walletAddress)
	}

	usdcBalance := 0.0
	nativeBalance := 0.0
	for _, balance := range wallet.Balances {
		if strings.EqualFold(balance.TokenAddress, t.chainConfig.USDCAddress) || strings.EqualFold(balance.Symbol, t.chainConfig.USDCSymbol) {
			usdcBalance += gmgnprovider.ParseFloatString(balance.Balance)
		}
		if strings.EqualFold(balance.Symbol, t.chainConfig.NativeTokenSymbol) {
			nativeBalance += gmgnprovider.ParseFloatString(balance.Balance)
		}
	}

	holdings, err := t.client.GetWalletHoldings(t.chain, t.walletAddress, map[string]any{"limit": 200})
	if err != nil {
		return nil, err
	}

	totalEquity := usdcBalance
	totalUnrealized := 0.0
	for _, holding := range holdings.List {
		addr := strings.TrimSpace(holding.Token.TokenAddress)
		if strings.EqualFold(addr, t.chainConfig.USDCAddress) || strings.EqualFold(holding.Token.Symbol, t.chainConfig.USDCSymbol) {
			continue
		}
		totalEquity += gmgnprovider.ParseFloatString(holding.USDValue)
		totalUnrealized += gmgnprovider.ParseFloatString(holding.UnrealizedProfit)
	}

	return map[string]interface{}{
		"availableBalance":        usdcBalance,
		"available_balance":       usdcBalance,
		"totalWalletBalance":      totalEquity,
		"wallet_balance":          totalEquity,
		"totalEquity":             totalEquity,
		"total_equity":            totalEquity,
		"totalUnrealizedProfit":   totalUnrealized,
		"total_unrealized_profit": totalUnrealized,
		"nativeGasBalance":        nativeBalance,
		"nativeGasRequired":       t.chainConfig.MinGasBuffer,
		"nativeGasSufficient":     nativeBalance >= t.chainConfig.MinGasBuffer,
		"chain":                   t.chain,
		"walletAddress":           t.walletAddress,
		"asset":                   t.chainConfig.USDCSymbol,
	}, nil
}

func (t *Trader) GetPositions() ([]map[string]interface{}, error) {
	holdings, err := t.client.GetWalletHoldings(t.chain, t.walletAddress, map[string]any{"limit": 200})
	if err != nil {
		return nil, err
	}

	positions := make([]map[string]interface{}, 0, len(holdings.List))
	for _, holding := range holdings.List {
		addr := strings.TrimSpace(holding.Token.TokenAddress)
		if addr == "" {
			continue
		}
		if strings.EqualFold(addr, t.chainConfig.USDCAddress) || strings.EqualFold(holding.Token.Symbol, t.chainConfig.USDCSymbol) {
			continue
		}

		quantity := gmgnprovider.ParseFloatString(holding.Balance)
		if quantity <= 0 {
			continue
		}

		entryPrice := avgCostPrice(holding)
		markPrice := gmgnprovider.ParseFloatString(holding.Token.Price)
		if markPrice <= 0 {
			markPrice = entryPrice
		}
		unrealized := gmgnprovider.ParseFloatString(holding.UnrealizedProfit)

		positions = append(positions, map[string]interface{}{
			"symbol":           gmgnprovider.FormatSymbol(t.chain, addr),
			"side":             "long",
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"positionAmt":      quantity,
			"unRealizedProfit": unrealized,
			"liquidationPrice": 0.0,
			"leverage":         float64(1),
			"createdTime":      holding.StartHoldingAt,
		})
	}

	return positions, nil
}

func (t *Trader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if leverage > 1 {
		return nil, fmt.Errorf("gmgn is spot-only: leverage must be 1")
	}
	chain, tokenAddress, err := gmgnprovider.ParseSymbol(symbol)
	if err != nil {
		return nil, err
	}
	if chain != t.chain {
		return nil, fmt.Errorf("gmgn trader chain mismatch: trader=%s symbol=%s", t.chain, symbol)
	}
	if err := t.ensureGasBuffer(true); err != nil {
		return nil, err
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	if quantity <= 0 || price <= 0 {
		return nil, fmt.Errorf("invalid quantity or price for gmgn open long")
	}

	inputAmount := gmgnprovider.RawAmountFromDecimal(quantity*price, gmgnprovider.DefaultUSDCQuoteDecimals)
	return t.swap(symbol, t.chainConfig.USDCAddress, tokenAddress, inputAmount)
}

func (t *Trader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("gmgn is spot-only: open_short is unsupported")
}

func (t *Trader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	chain, tokenAddress, err := gmgnprovider.ParseSymbol(symbol)
	if err != nil {
		return nil, err
	}
	if chain != t.chain {
		return nil, fmt.Errorf("gmgn trader chain mismatch: trader=%s symbol=%s", t.chain, symbol)
	}

	tokenInfo, err := t.client.GetTokenInfo(t.chain, tokenAddress)
	if err != nil {
		return nil, err
	}

	sellQty := quantity
	if sellQty <= 0 {
		balance, err := t.client.GetWalletTokenBalance(t.chain, t.walletAddress, tokenAddress)
		if err != nil {
			return nil, err
		}
		sellQty = parseWalletTokenBalance(balance)
	}
	if sellQty <= 0 {
		return nil, fmt.Errorf("no token balance available to close %s", symbol)
	}

	inputAmount := gmgnprovider.RawAmountFromDecimal(sellQty, tokenInfo.Decimals)
	return t.swap(symbol, tokenAddress, t.chainConfig.USDCAddress, inputAmount)
}

func (t *Trader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, fmt.Errorf("gmgn is spot-only: close_short is unsupported")
}

func (t *Trader) SetLeverage(symbol string, leverage int) error {
	if leverage > 1 {
		return fmt.Errorf("gmgn is spot-only: leverage > 1 is unsupported")
	}
	logger.Infof("ℹ️ GMGN SetLeverage no-op for %s (spot-only)", symbol)
	return nil
}

func (t *Trader) SetMarginMode(symbol string, isCrossMargin bool) error {
	logger.Infof("ℹ️ GMGN SetMarginMode no-op for %s (spot-only)", symbol)
	return nil
}

func (t *Trader) GetMarketPrice(symbol string) (float64, error) {
	chain, tokenAddress, err := gmgnprovider.ParseSymbol(symbol)
	if err != nil {
		return 0, err
	}
	tokenInfo, err := t.client.GetTokenInfo(chain, tokenAddress)
	if err != nil {
		return 0, err
	}
	if tokenInfo.Price == nil {
		return 0, fmt.Errorf("gmgn token price unavailable for %s", symbol)
	}
	price := gmgnprovider.ParseFloatString(tokenInfo.Price.Price)
	if price <= 0 {
		return 0, fmt.Errorf("gmgn token price invalid for %s", symbol)
	}
	return price, nil
}

func (t *Trader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return t.createStrategyOrder(symbol, positionSide, quantity, stopPrice, stopLossOrderType)
}

func (t *Trader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return t.createStrategyOrder(symbol, positionSide, quantity, takeProfitPrice, takeProfitOrderType)
}

func (t *Trader) CancelStopLossOrders(symbol string) error {
	return t.cancelStrategyOrders(symbol, stopLossOrderType)
}

func (t *Trader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelStrategyOrders(symbol, takeProfitOrderType)
}

func (t *Trader) CancelAllOrders(symbol string) error {
	return t.cancelStrategyOrders(symbol, "")
}

func (t *Trader) CancelStopOrders(symbol string) error {
	return t.CancelAllOrders(symbol)
}

func (t *Trader) FormatQuantity(symbol string, quantity float64) (string, error) {
	_, tokenAddress, err := gmgnprovider.ParseSymbol(symbol)
	if err != nil {
		return "", err
	}
	tokenInfo, err := t.client.GetTokenInfo(t.chain, tokenAddress)
	if err != nil {
		return "", err
	}
	raw := gmgnprovider.RawAmountFromDecimal(quantity, tokenInfo.Decimals)
	formatted := gmgnprovider.DecimalAmountFromRaw(raw, tokenInfo.Decimals)
	return formatAmount(formatted), nil
}

func (t *Trader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	resp, err := t.client.QueryOrder(t.chain, orderID)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"orderId":     orderID,
		"status":      normalizeOrderStatus(resp.Status),
		"avgPrice":    0.0,
		"executedQty": 0.0,
		"commission":  0.0,
	}
	if resp.Report != nil {
		price := gmgnprovider.ParseFloatString(resp.Report.PriceUSD)
		if price <= 0 {
			price = gmgnprovider.ParseFloatString(resp.Report.Price)
		}
		result["avgPrice"] = price
		executedQty := gmgnprovider.DecimalAmountFromRaw(resp.Report.OutputAmount, resp.Report.OutputTokenDecimals)
		if strings.EqualFold(resp.Report.OutputToken, t.chainConfig.USDCAddress) || strings.EqualFold(resp.Report.OutputToken, t.chainConfig.USDCSymbol) {
			executedQty = gmgnprovider.DecimalAmountFromRaw(resp.Report.InputAmount, resp.Report.InputTokenDecimals)
		}
		result["executedQty"] = executedQty
		result["commission"] = gmgnprovider.ParseFloatString(resp.Report.GasUSD)
	}
	return result, nil
}

func (t *Trader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	return nil, nil
}

func (t *Trader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	ordersResp, err := t.client.GetStrategyOrders(t.chain, map[string]any{
		"wallet_address": t.walletAddress,
		"limit":          100,
	})
	if err != nil {
		return nil, err
	}

	var targetSymbol string
	if strings.TrimSpace(symbol) != "" {
		targetSymbol = strings.TrimSpace(symbol)
	}

	result := make([]types.OpenOrder, 0, len(ordersResp.List))
	for _, item := range ordersResp.List {
		orderSymbol := extractStrategySymbol(t.chain, item)
		if targetSymbol != "" && orderSymbol != targetSymbol {
			continue
		}
		status := strings.ToUpper(fmt.Sprint(item["status"]))
		if status == "" {
			status = "NEW"
		}
		result = append(result, types.OpenOrder{
			OrderID:      firstNonEmpty(item, "strategy_order_id", "order_id", "id"),
			Symbol:       orderSymbol,
			Side:         normalizeOrderSide(fmt.Sprint(item["side"])),
			PositionSide: "LONG",
			Type:         normalizeStrategyType(fmt.Sprint(item["order_type"]), fmt.Sprint(item["sub_order_type"])),
			Price:        gmgnprovider.ParseFloatString(fmt.Sprint(item["check_price"])),
			StopPrice:    gmgnprovider.ParseFloatString(fmt.Sprint(item["check_price"])),
			Quantity:     strategyOrderQuantity(item),
			Status:       status,
		})
	}
	return result, nil
}

func (t *Trader) swap(symbol, inputToken, outputToken, inputAmount string) (map[string]interface{}, error) {
	resp, err := t.client.Swap(gmgnprovider.SwapParams{
		Chain:        t.chain,
		FromAddress:  t.walletAddress,
		InputToken:   inputToken,
		OutputToken:  outputToken,
		InputAmount:  inputAmount,
		Slippage:     defaultSlippage,
		AutoSlippage: true,
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"orderId":         firstNonEmptyMap(resp.OrderID, resp.Hash),
		"status":          normalizeOrderStatus(resp.Status),
		"strategyOrderId": resp.StrategyOrderID,
		"symbol":          symbol,
	}, nil
}

func (t *Trader) createStrategyOrder(symbol, positionSide string, quantity, triggerPrice float64, orderType string) error {
	if strings.EqualFold(positionSide, "SHORT") {
		return fmt.Errorf("gmgn is spot-only: short strategy orders are unsupported")
	}
	chain, tokenAddress, err := gmgnprovider.ParseSymbol(symbol)
	if err != nil {
		return err
	}
	if chain != t.chain {
		return fmt.Errorf("gmgn trader chain mismatch: trader=%s symbol=%s", t.chain, symbol)
	}
	resp, err := t.client.CreateStrategyOrder(gmgnprovider.StrategyCreateParams{
		Chain:        t.chain,
		FromAddress:  t.walletAddress,
		BaseToken:    tokenAddress,
		QuoteToken:   t.chainConfig.USDCAddress,
		OrderType:    orderType,
		SubOrderType: orderType,
		CheckPrice:   formatPrice(triggerPrice),
		AmountIn:     formatAmount(quantity),
		Slippage:     defaultSlippage,
		AutoSlippage: true,
	})
	if err != nil {
		return err
	}
	if normalizeOrderStatus(resp.Status) == "REJECTED" {
		return fmt.Errorf("gmgn strategy order rejected: %s", resp.ErrorCode)
	}
	return nil
}

func (t *Trader) cancelStrategyOrders(symbol, orderType string) error {
	orders, err := t.GetOpenOrders(symbol)
	if err != nil {
		return err
	}
	for _, order := range orders {
		if orderType != "" && !strings.EqualFold(order.Type, normalizeStrategyType(orderType, orderType)) {
			continue
		}
		if strings.TrimSpace(order.OrderID) == "" {
			continue
		}
		_, err := t.client.CancelStrategyOrder(gmgnprovider.StrategyCancelParams{
			Chain:       t.chain,
			FromAddress: t.walletAddress,
			OrderID:     order.OrderID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Trader) ensureGasBuffer(blockOpen bool) error {
	info, err := t.client.GetUserInfo()
	if err != nil {
		return err
	}
	wallet, ok := findWallet(info, t.chain, t.walletAddress)
	if !ok {
		return fmt.Errorf("gmgn wallet not found for gas check")
	}
	nativeBalance := 0.0
	for _, balance := range wallet.Balances {
		if strings.EqualFold(balance.Symbol, t.chainConfig.NativeTokenSymbol) {
			nativeBalance += gmgnprovider.ParseFloatString(balance.Balance)
		}
	}
	if blockOpen && nativeBalance < t.chainConfig.MinGasBuffer {
		return fmt.Errorf("insufficient %s gas balance: %.6f < %.6f", t.chainConfig.NativeTokenSymbol, nativeBalance, t.chainConfig.MinGasBuffer)
	}
	return nil
}

func findWallet(info *gmgnprovider.UserInfoResponse, chain, walletAddress string) (gmgnprovider.WalletEntry, bool) {
	if info == nil {
		return gmgnprovider.WalletEntry{}, false
	}
	for _, wallet := range info.Wallets {
		if gmgnprovider.NormalizeChain(wallet.Chain) == chain && strings.EqualFold(strings.TrimSpace(wallet.Address), strings.TrimSpace(walletAddress)) {
			return wallet, true
		}
	}
	return gmgnprovider.WalletEntry{}, false
}

func avgCostPrice(holding gmgnprovider.WalletHoldingItem) float64 {
	qty := gmgnprovider.ParseFloatString(holding.Balance)
	if qty <= 0 {
		qty = gmgnprovider.ParseFloatString(holding.HistoryBoughtAmount)
	}
	if qty <= 0 {
		return gmgnprovider.ParseFloatString(holding.Token.Price)
	}
	cost := gmgnprovider.ParseFloatString(holding.AccuCost)
	if cost <= 0 {
		cost = gmgnprovider.ParseFloatString(holding.HistoryBoughtCost)
	}
	if cost <= 0 {
		return gmgnprovider.ParseFloatString(holding.Token.Price)
	}
	return cost / qty
}

func parseWalletTokenBalance(balance map[string]interface{}) float64 {
	keys := []string{"balance", "amount", "token_balance"}
	for _, key := range keys {
		if raw, ok := balance[key]; ok {
			switch v := raw.(type) {
			case string:
				return gmgnprovider.ParseFloatString(v)
			case float64:
				return v
			}
		}
	}
	return 0
}

func normalizeOrderStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "completed", "filled", "executed", "done":
		return "FILLED"
	case "new", "open", "pending", "submitted", "processing":
		return "NEW"
	case "canceled", "cancelled":
		return "CANCELED"
	case "failed", "rejected", "error":
		return "REJECTED"
	default:
		if strings.TrimSpace(status) == "" {
			return "NEW"
		}
		return strings.ToUpper(status)
	}
}

func normalizeStrategyType(orderType, subType string) string {
	value := strings.ToLower(strings.TrimSpace(subType))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(orderType))
	}
	switch value {
	case stopLossOrderType:
		return "STOP_MARKET"
	case takeProfitOrderType:
		return "TAKE_PROFIT_MARKET"
	default:
		return strings.ToUpper(strings.TrimSpace(value))
	}
}

func normalizeOrderSide(side string) string {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "sell", "ask":
		return "SELL"
	default:
		return "BUY"
	}
}

func firstNonEmptyMap(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := item[key]; ok {
			value := strings.TrimSpace(fmt.Sprint(raw))
			if value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

func extractStrategySymbol(chain string, item map[string]interface{}) string {
	tokenAddress := firstNonEmpty(item, "base_token", "token_address", "baseToken", "tokenAddress")
	if tokenAddress == "" {
		if nested, ok := item["token"].(map[string]interface{}); ok {
			tokenAddress = firstNonEmpty(nested, "token_address", "address")
		}
	}
	if tokenAddress == "" {
		return ""
	}
	return gmgnprovider.FormatSymbol(chain, tokenAddress)
}

func strategyOrderQuantity(item map[string]interface{}) float64 {
	for _, key := range []string{"amount_in", "amount", "quantity"} {
		if raw, ok := item[key]; ok {
			return gmgnprovider.ParseFloatString(fmt.Sprint(raw))
		}
	}
	return 0
}

func formatPrice(price float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.18f", price), "0"), ".")
}

func formatAmount(amount float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.18f", math.Max(amount, 0)), "0"), ".")
}
