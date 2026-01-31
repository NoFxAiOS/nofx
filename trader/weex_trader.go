package trader

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/hook"
	"nofx/market"
)

type WeexTrader struct {
	apiKey     string
	secretKey  string
	passphrase string
	baseURL    string
	client     *http.Client

	contractMu sync.RWMutex
	contracts  map[string]*weexContractMeta // key: exchange symbol (e.g. cmt_btcusdt)
	aliases    map[string]string            // key: normalized symbol (e.g. BTCUSDT) -> exchange symbol
}

type weexContractMeta struct {
	Symbol         string
	TickSize       float64
	SizeIncrement  float64
	MinOrderSize   float64
	PricePrecision int
	SizePrecision  int
}

// weexContractRecord mirrors the subset of contract metadata fields needed for
// order formatting. The API returns many more fields (some arrays), so keeping
// a dedicated struct prevents json.Unmarshal from trying to coerce everything
// into strings.
type weexContractRecord struct {
	Symbol        string `json:"symbol"`
	TickSize      string `json:"tick_size"`
	SizeIncrement string `json:"size_increment"`
	MinOrderSize  string `json:"minOrderSize"`
}

type weexAPIEnvelope struct {
	Code json.RawMessage `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type accountBalanceItem struct {
	CoinName   string `json:"coinName"`
	Available  string `json:"available"`
	Equity     string `json:"equity"`
	Unrealized string `json:"unrealizePnl"`
}

type placeOrderResponse struct {
	OrderID string `json:"order_id"`
}

type futuresOrder struct {
	OrderID      string `json:"order_id"`
	ClientOid    string `json:"client_oid"`
	Symbol       string `json:"symbol"`
	Side         string `json:"side"`
	PosSide      string `json:"posSide"`
	Type         string `json:"type"`
	Price        string `json:"price"`
	TriggerPrice string `json:"triggerPrice"`
	Size         string `json:"size"`
	State        string `json:"state"`
}

type contractPosition struct {
	ID                         int64  `json:"id"`
	AccountID                  int64  `json:"account_id"`
	CoinID                     int64  `json:"coin_id"`
	ContractID                 int64  `json:"contract_id"`
	Symbol                     string `json:"symbol"`
	Side                       string `json:"side"`
	MarginMode                 string `json:"margin_mode"`
	SeparatedMode              string `json:"separated_mode"`
	SeparatedOpenOrderID       int64  `json:"separated_open_order_id"`
	Leverage                   string `json:"leverage"`
	Size                       string `json:"size"`
	OpenValue                  string `json:"open_value"`
	OpenFee                    string `json:"open_fee"`
	FundingFee                 string `json:"funding_fee"`
	MarginSize                 string `json:"marginSize"`
	IsolatedMargin             string `json:"isolated_margin"`
	IsAutoAppendIsolatedMargin bool   `json:"is_auto_append_isolated_margin"`
	CumOpenSize                string `json:"cum_open_size"`
	CumOpenValue               string `json:"cum_open_value"`
	CumOpenFee                 string `json:"cum_open_fee"`
	CumCloseSize               string `json:"cum_close_size"`
	CumCloseValue              string `json:"cum_close_value"`
	CumCloseFee                string `json:"cum_close_fee"`
	CumFundingFee              string `json:"cum_funding_fee"`
	CumLiquidateFee            string `json:"cum_liquidate_fee"`
	CreatedMatchSequenceID     int64  `json:"created_match_sequence_id"`
	UpdatedMatchSequenceID     int64  `json:"updated_match_sequence_id"`
	CreatedTime                int64  `json:"created_time"`
	UpdatedTime                int64  `json:"updated_time"`
	ContractVal                string `json:"contractVal"`
	UnrealizedPnl              string `json:"unrealizePnl"`
	LiquidatePrice             string `json:"liquidatePrice"`
}

// AiLogPayload represents the request body for /capi/v2/order/uploadAiLog.
// Reference: weex_futures_openapi.json components.schemas.AiLogRequest
type AiLogPayload struct {
	OrderID     interface{}     `json:"orderId,omitempty"`
	Stage       string          `json:"stage"`
	Model       string          `json:"model"`
	Input       json.RawMessage `json:"input"`
	Output      json.RawMessage `json:"output"`
	Explanation string          `json:"explanation,omitempty"`
}

func (p AiLogPayload) validate() error {
	if strings.TrimSpace(p.Stage) == "" {
		return fmt.Errorf("stage is required")
	}
	if strings.TrimSpace(p.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if err := requireJSONObject(p.Input, "input"); err != nil {
		return err
	}
	if err := requireJSONObject(p.Output, "output"); err != nil {
		return err
	}
	return nil
}

const (
	defaultWeexBaseURL       = "https://api-contract.weex.com"
	defaultWeexTestnetURL    = "https://api-contract.weex.com"
	planOrderMatchTypeMarket = "0"
)

type planOrderIntent string

const (
	planIntentTakeProfit planOrderIntent = "TAKE_PROFIT"
	planIntentStopLoss   planOrderIntent = "STOP_LOSS"
)

// NewWeexTrader creates a trader implementation backed by Weex REST API.
func NewWeexTrader(apiKey, secretKey, passphrase string, useTestnet bool) (*WeexTrader, error) {
	if apiKey == "" || secretKey == "" || passphrase == "" {
		return nil, fmt.Errorf("Weex API key/secret/passphrase 不能为空")
	}

	baseURL := resolveWeexBaseURL(useTestnet)
	client := &http.Client{Timeout: 15 * time.Second}
	if hookRes := hook.HookExec[hook.NewWeexTraderResult](hook.NEW_WEEX_TRADER, apiKey, client); hookRes != nil {
		if hookRes.Error() == nil && hookRes.GetResult() != nil {
			client = hookRes.GetResult()
		}
	}

	trader := &WeexTrader{
		apiKey:     apiKey,
		secretKey:  secretKey,
		passphrase: passphrase,
		baseURL:    strings.TrimRight(baseURL, "/"),
		client:     client,
		contracts:  make(map[string]*weexContractMeta),
		aliases:    make(map[string]string),
	}

	if err := trader.loadContracts(context.Background()); err != nil {
		log.Printf("⚠️ 预加载Weex合约元数据失败: %v (将在首次调用时重试)", err)
	}

	return trader, nil
}

func resolveWeexBaseURL(useTestnet bool) string {
	if useTestnet {
		if v := strings.TrimSpace(os.Getenv("WEEX_TESTNET_BASE_URL")); v != "" {
			return v
		}
		return defaultWeexTestnetURL
	}
	if v := strings.TrimSpace(os.Getenv("WEEX_BASE_URL")); v != "" {
		return v
	}
	return defaultWeexBaseURL
}

// GetBalance returns aggregated account balances in USDT terms.
func (t *WeexTrader) GetBalance() (map[string]interface{}, error) {
	var items []accountBalanceItem
	if err := t.signedRequest(context.Background(), http.MethodGet, "/capi/v2/account/assets", nil, nil, &items); err != nil {
		return nil, err
	}

	var totalEquity, available, unrealized float64
	for _, item := range items {
		if strings.ToUpper(item.CoinName) != "USDT" {
			continue
		}
		if v, err := strconv.ParseFloat(item.Equity, 64); err == nil {
			totalEquity = v
		}
		if v, err := strconv.ParseFloat(item.Available, 64); err == nil {
			available = v
		}
		if v, err := strconv.ParseFloat(item.Unrealized, 64); err == nil {
			unrealized = v
		}
	}

	return map[string]interface{}{
		"totalWalletBalance":    totalEquity,
		"availableBalance":      available,
		"totalUnrealizedProfit": unrealized,
	}, nil
}

func (t *WeexTrader) GetPositions() ([]map[string]interface{}, error) {
	var rawPositions []contractPosition
	if err := t.signedRequest(context.Background(), http.MethodGet, "/capi/v2/account/position/allPosition", nil, nil, &rawPositions); err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(rawPositions))
	for _, pos := range rawPositions {
		qty, err := strconv.ParseFloat(pos.Size, 64)
		if err != nil || qty == 0 {
			continue
		}

		normalizedSymbol, err := t.denormalizeSymbol(pos.Symbol)
		if err != nil {
			log.Printf("⚠️ 无法识别Weex交易对 %s: %v", pos.Symbol, err)
			continue
		}

		entryPrice := 0.0
		if pos.OpenValue != "" {
			if openValue, err := strconv.ParseFloat(pos.OpenValue, 64); err == nil && qty != 0 {
				entryPrice = math.Abs(openValue / qty)
			}
		}

		markPrice, err := t.GetMarketPrice(normalizedSymbol)
		if err != nil {
			markPrice = entryPrice
		}

		side := strings.ToLower(pos.Side)
		signedQty := qty
		if side == "short" {
			signedQty = -qty
		}

		leverage, _ := strconv.ParseFloat(pos.Leverage, 64)
		unrealized, _ := strconv.ParseFloat(pos.UnrealizedPnl, 64)
		liquidationPrice, _ := strconv.ParseFloat(pos.LiquidatePrice, 64)

		result = append(result, map[string]interface{}{
			"symbol":           normalizedSymbol,
			"side":             side,
			"positionAmt":      signedQty,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealized,
			"leverage":         leverage,
			"liquidationPrice": liquidationPrice,
		})
	}

	return result, nil
}

func (t *WeexTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.OpenLongWithPreset(symbol, quantity, leverage, OrderPreset{})
}

func (t *WeexTrader) OpenLongWithPreset(symbol string, quantity float64, leverage int, preset OrderPreset) (map[string]interface{}, error) {
	return t.openPosition(symbol, quantity, leverage, 1, preset)
}

func (t *WeexTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.OpenShortWithPreset(symbol, quantity, leverage, OrderPreset{})
}

func (t *WeexTrader) OpenShortWithPreset(symbol string, quantity float64, leverage int, preset OrderPreset) (map[string]interface{}, error) {
	return t.openPosition(symbol, quantity, leverage, 2, preset)
}

func (t *WeexTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
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
	return t.closePosition(symbol, quantity, 3)
}

func (t *WeexTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				qty := pos["positionAmt"].(float64)
				quantity = -qty
				break
			}
		}
		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
		}
	}
	return t.closePosition(symbol, quantity, 4)
}

func (t *WeexTrader) SetLeverage(symbol string, leverage int) error {
	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return err
	}

	payload := map[string]string{
		"symbol":        weexSymbol,
		"marginMode":    "1",
		"longLeverage":  strconv.Itoa(leverage),
		"shortLeverage": strconv.Itoa(leverage),
	}
	return t.signedRequest(context.Background(), http.MethodPost, "/capi/v2/account/leverage", nil, payload, nil)
}

func (t *WeexTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	log.Printf("ℹ️ WeexTrader暂不支持API方式切换仓位模式，已跳过 %s", symbol)
	return nil
}

func (t *WeexTrader) GetMarketPrice(symbol string) (float64, error) {
	// 打印日志，开始获取价格
	log.Printf("开始获取 %s 的价格\n", symbol)
	data, err := market.Get(symbol)
	if err != nil {
		return 0, err
	}
	log.Printf("获取 %s 的价格（来自最近一条3分钟k线的close price）： %.8f\n", symbol, data.CurrentPrice)
	return data.CurrentPrice, nil
}

func (t *WeexTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	// return t.submitPlanOrder(symbol, positionSide, quantity, stopPrice, planIntentStopLoss)
	// 暂不支持止损
	return nil
}

func (t *WeexTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	// return t.submitPlanOrder(symbol, positionSide, quantity, takeProfitPrice, planIntentTakeProfit)
	// 暂不支持止盈
	return nil
}

func (t *WeexTrader) CancelStopLossOrders(symbol string) error {
	// return t.cancelPlanOrders(symbol, planIntentStopLoss)
	// 暂不支持止损
	return nil
}

func (t *WeexTrader) CancelTakeProfitOrders(symbol string) error {
	// return t.cancelPlanOrders(symbol, planIntentTakeProfit)
	// 暂不支持止盈
	return nil
}

func (t *WeexTrader) CancelAllOrders(symbol string) error {
	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return err
	}

	query := url.Values{}
	query.Set("symbol", weexSymbol)

	var orders []futuresOrder
	if err := t.signedRequest(context.Background(), http.MethodGet, "/capi/v2/order/current", query, nil, &orders); err != nil {
		return err
	}
	if len(orders) == 0 {
		return nil
	}

	for _, order := range orders {
		if order.OrderID == "" {
			continue
		}
		payload := map[string]string{
			"symbol":  weexSymbol,
			"orderId": order.OrderID,
		}
		if err := t.signedRequest(context.Background(), http.MethodPost, "/capi/v2/order/cancel_order", nil, payload, nil); err != nil {
			return err
		}
	}

	return nil
}

func (t *WeexTrader) CancelStopOrders(symbol string) error {
	if err := t.cancelPlanOrders(symbol, planIntentStopLoss); err != nil {
		return err
	}
	return t.cancelPlanOrders(symbol, planIntentTakeProfit)
}

func (t *WeexTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	meta, err := t.getContractMeta(symbol)
	if err != nil {
		return "", err
	}

	q := floorToStep(quantity, meta.MinOrderSize)
	fmt.Printf("quantity: %.8f minOrderSize: %.8f q: %.8f\n", quantity, meta.MinOrderSize, q)
	if q < meta.MinOrderSize {
		return "", fmt.Errorf("数量 %.8f (floor: %.8f) 小于Weex最小下单数量 %.8f", quantity, q, meta.MinOrderSize)
	}

	precision := meta.SizePrecision
	if precision < 0 {
		precision = 6
	}

	result := fmt.Sprintf("%.8f", q)
	fmt.Printf("计算得数量： %s\n", result)
	return result, nil
}

// GetOrderStatus best-effort order status lookup.
// Weex current-order API only returns open orders; if not found, assume FILLED.
func (t *WeexTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	if orderID == "" {
		return nil, fmt.Errorf("orderID is required")
	}

	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("symbol", weexSymbol)

	var orders []futuresOrder
	if err := t.signedRequest(context.Background(), http.MethodGet, "/capi/v2/order/current", query, nil, &orders); err != nil {
		return nil, err
	}

	status := "FILLED"
	avgPrice := 0.0
	executedQty := 0.0

	for _, order := range orders {
		if order.OrderID != orderID {
			continue
		}
		status = "NEW"
		if v, err := strconv.ParseFloat(order.Price, 64); err == nil {
			avgPrice = v
		}
		break
	}

	return map[string]interface{}{
		"orderId":     orderID,
		"symbol":      symbol,
		"status":      status,
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"commission":  0.0,
	}, nil
}

// GetClosedPnL is currently not supported by Weex API in this implementation.
// Return empty list to allow position sync to fall back to market price.
func (t *WeexTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	_ = startTime
	_ = limit
	return []ClosedPnLRecord{}, nil
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *WeexTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("symbol", weexSymbol)

	result := make([]OpenOrder, 0)

	resolveSymbol := func(raw string) string {
		if raw == "" {
			return symbol
		}
		if denormalized, err := t.denormalizeSymbol(raw); err == nil {
			return denormalized
		}
		return symbol
	}

	resolveSides := func(orderType, side, posSide string) (string, string) {
		if side != "" || posSide != "" {
			return strings.ToUpper(side), strings.ToUpper(posSide)
		}
		switch strings.TrimSpace(orderType) {
		case "1":
			return "BUY", "LONG"
		case "2":
			return "SELL", "SHORT"
		case "3":
			return "SELL", "LONG"
		case "4":
			return "BUY", "SHORT"
		default:
			return "", ""
		}
	}

	// 1) Current orders (limit/pending)
	var orders []futuresOrder
	if err := t.signedRequest(context.Background(), http.MethodGet, "/capi/v2/order/current", query, nil, &orders); err != nil {
		return nil, err
	}
	for _, order := range orders {
		if order.OrderID == "" {
			continue
		}
		price := parseStringFloat(order.Price)
		qty := parseStringFloat(order.Size)
		orderSymbol := resolveSymbol(order.Symbol)
		side, posSide := resolveSides(order.Type, order.Side, order.PosSide)

		orderType := "LIMIT"
		if price == 0 {
			orderType = "MARKET"
		}

		result = append(result, OpenOrder{
			OrderID:      order.OrderID,
			Symbol:       orderSymbol,
			Side:         side,
			PositionSide: posSide,
			Type:         orderType,
			Price:        price,
			StopPrice:    0,
			Quantity:     qty,
			Status:       "NEW",
		})
	}

	// 2) Plan orders (stop-loss / take-profit)
	var planOrders []futuresOrder
	if err := t.signedRequest(context.Background(), http.MethodGet, "/capi/v2/order/currentPlan", query, nil, &planOrders); err != nil {
		return nil, err
	}
	for _, order := range planOrders {
		if order.OrderID == "" {
			continue
		}
		stopPrice := parseStringFloat(order.TriggerPrice)
		if stopPrice == 0 {
			stopPrice = parseStringFloat(order.Price)
		}
		qty := parseStringFloat(order.Size)
		orderSymbol := resolveSymbol(order.Symbol)
		side, posSide := resolveSides(order.Type, order.Side, order.PosSide)

		orderType := "STOP_MARKET"
		switch strings.TrimSpace(order.Type) {
		case "1", "4":
			orderType = "TAKE_PROFIT_MARKET"
		case "2", "3":
			orderType = "STOP_MARKET"
		}

		result = append(result, OpenOrder{
			OrderID:      order.OrderID,
			Symbol:       orderSymbol,
			Side:         side,
			PositionSide: posSide,
			Type:         orderType,
			Price:        0,
			StopPrice:    stopPrice,
			Quantity:     qty,
			Status:       "NEW",
		})
	}

	return result, nil
}

// UploadAiLog sends AI evaluation logs to Weex for a specific order/stage.
func (t *WeexTrader) UploadAiLog(payload AiLogPayload) error {
	if err := payload.validate(); err != nil {
		return err
	}
	return t.signedRequest(context.Background(), http.MethodPost, "/capi/v2/order/uploadAiLog", nil, payload, nil)
}

func (t *WeexTrader) submitPlanOrder(symbol string, positionSide string, quantity, triggerPrice float64, intent planOrderIntent) error {
	qty := math.Abs(quantity)
	if qty == 0 {
		return fmt.Errorf("止损/止盈数量必须大于0")
	}
	if triggerPrice <= 0 {
		return fmt.Errorf("止损/止盈价格必须大于0")
	}

	orderType, err := planOrderType(positionSide, intent)
	if err != nil {
		return err
	}

	size, err := t.FormatQuantity(symbol, qty)
	if err != nil {
		return err
	}

	priceStr, err := t.formatPrice(symbol, triggerPrice)
	if err != nil {
		return err
	}

	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return err
	}

	payload := map[string]string{
		"symbol":        weexSymbol,
		"client_oid":    t.newClientOID(),
		"size":          size,
		"type":          strconv.Itoa(orderType),
		"match_type":    planOrderMatchTypeMarket,
		"execute_price": priceStr,
		"trigger_price": priceStr,
	}

	return t.signedRequest(context.Background(), http.MethodPost, "/capi/v2/order/plan_order", nil, payload, nil)
}

func planOrderType(positionSide string, intent planOrderIntent) (int, error) {
	side := strings.ToUpper(strings.TrimSpace(positionSide))
	switch intent {
	case planIntentTakeProfit:
		switch side {
		case "LONG":
			return 1, nil
		case "SHORT":
			return 4, nil
		}
	case planIntentStopLoss:
		switch side {
		case "LONG":
			return 3, nil
		case "SHORT":
			return 2, nil
		}
	default:
		return 0, fmt.Errorf("未知的止盈/止损类型: %s", intent)
	}
	return 0, fmt.Errorf("不支持的持仓方向: %s", positionSide)
}

// --- internal helpers ---

func (t *WeexTrader) openPosition(symbol string, quantity float64, leverage int, orderType int, preset OrderPreset) (map[string]interface{}, error) {
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("⚠️ 清理旧委托失败: %v", err)
	}

	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	normalizedQtyStr, err := t.FormatQuantity(symbol, quantity)
	fmt.Printf("quantity: %s normalizedQtyStr: %s\n", strconv.FormatFloat(quantity, 'f', -1, 64), normalizedQtyStr)
	if err != nil {
		return nil, err
	}

	qtyFloat, err := strconv.ParseFloat(normalizedQtyStr, 64)
	if err != nil || qtyFloat <= 0 {
		return nil, fmt.Errorf("下单数量过小: %s", normalizedQtyStr)
	}

	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	priceStr, err := t.formatPrice(symbol, price)
	if err != nil {
		return nil, err
	}

	presetTakeProfit := "0"
	presetStopLoss := "0"
	if preset.TakeProfit > 0 {
		formatted, err := t.formatPrice(symbol, preset.TakeProfit)
		if err != nil {
			return nil, fmt.Errorf("format preset take profit: %w", err)
		}
		presetTakeProfit = formatted
	}
	if preset.StopLoss > 0 {
		formatted, err := t.formatPrice(symbol, preset.StopLoss)
		if err != nil {
			return nil, fmt.Errorf("format preset stop loss: %w", err)
		}
		presetStopLoss = formatted
	}

	payload := map[string]string{
		"symbol":                weexSymbol,
		"client_oid":            t.newClientOID(),
		"size":                  normalizedQtyStr,
		"type":                  strconv.Itoa(orderType),
		"order_type":            "0",
		"match_price":           "1",
		"price":                 priceStr,
		"presetTakeProfitPrice": presetTakeProfit,
		"presetStopLossPrice":   presetStopLoss,
	}

	var resp placeOrderResponse
	if err := t.signedRequest(context.Background(), http.MethodPost, "/capi/v2/order/placeOrder", nil, payload, &resp); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"orderId": resp.OrderID,
		"symbol":  symbol,
	}, nil
}

func (t *WeexTrader) closePosition(symbol string, quantity float64, orderType int) (map[string]interface{}, error) {
	normalizedQtyStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	priceStr, err := t.formatPrice(symbol, price)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{
		"symbol":                weexSymbol,
		"client_oid":            t.newClientOID(),
		"size":                  normalizedQtyStr,
		"type":                  strconv.Itoa(orderType),
		"order_type":            "0",
		"match_price":           "1",
		"price":                 priceStr,
		"presetTakeProfitPrice": "0",
		"presetStopLossPrice":   "0",
	}

	var resp placeOrderResponse
	if err := t.signedRequest(context.Background(), http.MethodPost, "/capi/v2/order/placeOrder", nil, payload, &resp); err != nil {
		return nil, err
	}

	if err := t.CancelStopOrders(symbol); err != nil {
		log.Printf("⚠️ 平仓后取消止盈止损失败: %v", err)
	}

	return map[string]interface{}{
		"orderId": resp.OrderID,
		"symbol":  symbol,
	}, nil
}

func (t *WeexTrader) cancelPlanOrders(symbol string, intent planOrderIntent) error {
	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("symbol", weexSymbol)

	var planOrders []futuresOrder
	if err := t.signedRequest(context.Background(), http.MethodGet, "/capi/v2/order/currentPlan", query, nil, &planOrders); err != nil {
		return err
	}

	if intent != planIntentTakeProfit && intent != planIntentStopLoss {
		return fmt.Errorf("unsupported plan order intent: %s", intent)
	}

	type planCandidate struct {
		order *futuresOrder
		price float64
	}

	grouped := make(map[string][]planCandidate)
	for i := range planOrders {
		order := &planOrders[i]
		if order.OrderID == "" {
			continue
		}
		price := parseStringFloat(order.TriggerPrice)
		if price == 0 {
			price = parseStringFloat(order.Price)
		}
		grouped[order.Type] = append(grouped[order.Type], planCandidate{order: order, price: price})
	}

	selectOrder := func(entries []planCandidate, pickHigh bool) *futuresOrder {
		if len(entries) == 0 {
			return nil
		}
		selected := entries[0]
		for _, entry := range entries[1:] {
			if pickHigh {
				if entry.price > selected.price {
					selected = entry
				}
			} else {
				if entry.price < selected.price {
					selected = entry
				}
			}
		}
		return selected.order
	}

	shouldPickHigh := func(orderType string) (bool, bool) {
		switch orderType {
		case "CLOSE_SHORT":
			// Shorts take profit when price drops; stop loss when price rises.
			return intent == planIntentStopLoss, true
		case "CLOSE_LONG":
			// Longs take profit when price rises; stop loss when price falls.
			return intent == planIntentTakeProfit, true
		default:
			return false, false
		}
	}

	var targets []*futuresOrder
	for orderType, entries := range grouped {
		if len(entries) == 1 {
			continue
		}
		pickHigh, ok := shouldPickHigh(orderType)
		if !ok {
			continue
		}
		order := selectOrder(entries, pickHigh)
		if order != nil {
			targets = append(targets, order)
		}
	}

	for _, order := range targets {
		payload := map[string]string{
			"symbol":  weexSymbol,
			"orderId": order.OrderID,
		}
		// cancel_plan is broken on Weex servers; cancel_order handles plan orders too.
		if err := t.signedRequest(context.Background(), http.MethodPost, "/capi/v2/order/cancel_order", nil, payload, nil); err != nil {
			return err
		}
	}
	return nil
}

func (t *WeexTrader) normalizeSymbol(symbol string) (string, error) {
	normalized := market.Normalize(symbol)

	t.contractMu.RLock()
	if alias, ok := t.aliases[normalized]; ok {
		t.contractMu.RUnlock()
		return alias, nil
	}
	t.contractMu.RUnlock()

	if err := t.loadContracts(context.Background()); err != nil {
		return "", err
	}

	t.contractMu.RLock()
	alias, ok := t.aliases[normalized]
	t.contractMu.RUnlock()
	if !ok {
		return "", fmt.Errorf("Weex暂不支持交易对 %s", normalized)
	}
	return alias, nil
}

func (t *WeexTrader) denormalizeSymbol(symbol string) (string, error) {
	t.contractMu.RLock()
	defer t.contractMu.RUnlock()
	for norm, exch := range t.aliases {
		if exch == symbol {
			return norm, nil
		}
	}
	return "", fmt.Errorf("未知的Weex交易对: %s", symbol)
}

func (t *WeexTrader) getContractMeta(symbol string) (*weexContractMeta, error) {
	weexSymbol, err := t.normalizeSymbol(symbol)
	if err != nil {
		return nil, err
	}

	t.contractMu.RLock()
	meta, ok := t.contracts[weexSymbol]
	t.contractMu.RUnlock()
	if ok {
		return meta, nil
	}

	if err := t.loadContracts(context.Background()); err != nil {
		return nil, err
	}

	t.contractMu.RLock()
	meta, ok = t.contracts[weexSymbol]
	t.contractMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("未找到Weex交易对 %s 的精度信息", symbol)
	}
	return meta, nil
}

func (t *WeexTrader) loadContracts(ctx context.Context) error {
	var raw []weexContractRecord
	if err := t.publicGet(ctx, "/capi/v2/market/contracts", nil, &raw); err != nil {
		return err
	}

	aliases := make(map[string]string, len(raw))
	contracts := make(map[string]*weexContractMeta, len(raw))

	for _, item := range raw {
		symbol := strings.TrimSpace(item.Symbol)
		if symbol == "" {
			continue
		}
		tickSize := parseStringFloat(item.TickSize)
		sizeIncrement := parseStringFloat(item.SizeIncrement)
		minOrder := parseStringFloat(item.MinOrderSize)
		pricePrecision := decimalsFromString(item.TickSize)
		sizePrecision := decimalsFromString(item.SizeIncrement)

		contracts[symbol] = &weexContractMeta{
			Symbol:         symbol,
			TickSize:       tickSize,
			SizeIncrement:  sizeIncrement,
			MinOrderSize:   minOrder,
			PricePrecision: pricePrecision,
			SizePrecision:  sizePrecision,
		}

		upper := strings.ToUpper(symbol)
		normalized := strings.TrimPrefix(upper, "CMT_")
		aliases[normalized] = symbol
	}

	t.contractMu.Lock()
	t.contracts = contracts
	t.aliases = aliases
	t.contractMu.Unlock()
	return nil
}

func (t *WeexTrader) signedRequest(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	reqURL := t.baseURL + path
	if len(query) > 0 {
		reqURL = reqURL + "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	canonical := timestamp + strings.ToUpper(method) + path
	if len(query) > 0 {
		canonical += "?" + query.Encode()
	}
	if len(payload) > 0 {
		canonical += string(payload)
	}

	signature := t.buildSignature(canonical)

	req.Header.Set("ACCESS-KEY", t.apiKey)
	req.Header.Set("ACCESS-SIGN", signature)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("ACCESS-PASSPHRASE", t.passphrase)

	curlCmd := buildCurlCommand(req, payload)
	log.Printf("➡️ Weex curl: %s", curlCmd)

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 打印响应内容用于调试
	log.Printf("⬅️ Weex response [%s %s]: %s", method, path, string(bodyBytes))

	var envelope weexAPIEnvelope
	trimmed := bytes.TrimSpace(bodyBytes)
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		if len(trimmed) == 0 {
			return err
		}
		firstByte := trimmed[0]
		if firstByte == '[' || firstByte == '{' {
			envelope.Data = trimmed
		} else {
			return err
		}
	}

	if !envelopeSuccess(envelope.Code) {
		return fmt.Errorf("Weex API错误: code=%s msg=%s", strings.Trim(string(envelope.Code), `"`), envelope.Msg)
	}

	if len(envelope.Data) == 0 && len(trimmed) > 0 {
		if first := trimmed[0]; first == '{' || first == '[' {
			envelope.Data = trimmed
		}
	}

	if out != nil && envelope.Data != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return err
		}
	}

	return nil
}

func (t *WeexTrader) publicGet(ctx context.Context, path string, query url.Values, out interface{}) error {
	if ctx == nil {
		ctx = context.Background()
	}

	reqURL := t.baseURL + path
	if len(query) > 0 {
		reqURL = reqURL + "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取响应体用于日志和解析
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 打印响应内容用于调试
	log.Printf("⬅️ Weex public response [%s]: %s", path, string(bodyBytes))

	return json.Unmarshal(bodyBytes, out)
}

func (t *WeexTrader) buildSignature(payload string) string {
	mac := hmac.New(sha256.New, []byte(t.secretKey))
	mac.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildCurlCommand(req *http.Request, body []byte) string {
	var builder strings.Builder
	builder.WriteString("curl -X ")
	builder.WriteString(req.Method)
	builder.WriteString(" '")
	builder.WriteString(req.URL.String())
	builder.WriteString("'")

	headerKeys := make([]string, 0, len(req.Header))
	for key := range req.Header {
		headerKeys = append(headerKeys, key)
	}
	sort.Strings(headerKeys)

	for _, key := range headerKeys {
		for _, value := range req.Header[key] {
			builder.WriteString(" -H '")
			builder.WriteString(key)
			builder.WriteString(": ")
			builder.WriteString(escapeSingleQuotes(value))
			builder.WriteString("'")
		}
	}

	if len(body) > 0 {
		builder.WriteString(" --data-raw '")
		builder.WriteString(escapeSingleQuotes(string(body)))
		builder.WriteString("'")
	}

	return builder.String()
}

func escapeSingleQuotes(input string) string {
	if input == "" {
		return input
	}
	return strings.ReplaceAll(input, "'", `'"'"'`)
}

func (t *WeexTrader) newClientOID() string {
	return fmt.Sprintf("go-weex-%d", time.Now().UnixNano())
}

func envelopeSuccess(code json.RawMessage) bool {
	if len(code) == 0 {
		return true
	}
	var asString string
	if err := json.Unmarshal(code, &asString); err == nil {
		return asString == "0" || asString == "200" || asString == "" || asString == "00000"
	}
	var asInt int
	if err := json.Unmarshal(code, &asInt); err == nil {
		return asInt == 0 || asInt == 200
	}
	return false
}

func parseStringFloat(input string) float64 {
	if input == "" {
		return 0
	}
	v, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return 0
	}
	return v
}

func requireJSONObject(raw json.RawMessage, field string) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return fmt.Errorf("%s is required", field)
	}
	if !json.Valid(trimmed) {
		return fmt.Errorf("%s must be valid JSON", field)
	}
	if trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return fmt.Errorf("%s must be a JSON object", field)
	}
	return nil
}

func decimalsFromString(input string) int {
	if idx := strings.IndexByte(input, '.'); idx >= 0 {
		trimmed := strings.TrimRight(input[idx+1:], "0")
		return len(trimmed)
	}
	return 0
}

func floorToStep(value, step float64) float64 {
	if step <= 0 {
		return value
	}
	steps := math.Floor(value/step + 1e-9)
	result := steps * step
	// 如果向下取整结果为 0，则向上取整到至少一个步长单位
	if result == 0 && value > 0 {
		return step
	}
	return result
}

func (t *WeexTrader) formatPrice(symbol string, price float64) (string, error) {
	meta, err := t.getContractMeta(symbol)
	if err != nil {
		return "", err
	}
	// 转换整数形式的 tickSize 为实际小数值
	// 例如: 5 -> 0.00001, 1 -> 0.1
	tickSize := meta.TickSize
	if tickSize >= 1 {
		tickSize = 1.0 / math.Pow10(int(tickSize))
	}
	rounded := roundToStep(price, tickSize)
	precision := meta.PricePrecision
	if precision <= 0 {
		precision = 4
	}
	return strconv.FormatFloat(rounded, 'f', precision, 64), nil
}

func roundToStep(value, step float64) float64 {
	if step <= 0 {
		return value
	}
	return math.Round(value/step) * step
}
