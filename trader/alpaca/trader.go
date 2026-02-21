package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/provider/alpaca"
	"strconv"
	"strings"
	"time"

	"nofx/trader/types"
)

const (
	PaperAPIURL = "https://paper-api.alpaca.markets"
	LiveAPIURL  = "https://api.alpaca.markets"
	APIVersion  = "v2"
)

// AlpacaTrader implements types.Trader interface for Alpaca
type AlpacaTrader struct {
	apiKey     string
	secretKey  string
	paperMode  bool
	baseURL    string
	httpClient *http.Client
	userID     string
}

// NewAlpacaTrader creates a new Alpaca trader
func NewAlpacaTrader(apiKey, secretKey string, paperMode bool, userID string) *AlpacaTrader {
	baseURL := LiveAPIURL
	if paperMode {
		baseURL = PaperAPIURL
	}

	return &AlpacaTrader{
		apiKey:     apiKey,
		secretKey:  secretKey,
		paperMode:  paperMode,
		baseURL:    baseURL,
		userID:     userID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *AlpacaTrader) makeRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s/%s", t.baseURL, APIVersion, endpoint)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("APCA-API-KEY-ID", t.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", t.secretKey)
	req.Header.Set("Content-Type", "application/json")

	return t.httpClient.Do(req)
}

// GetBalance implements types.Trader
func (t *AlpacaTrader) GetBalance() (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := t.makeRequest(ctx, "GET", "account", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca API error (status %d): %s", resp.StatusCode, string(body))
	}

	var account map[string]interface{}
	if err := json.Unmarshal(body, &account); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := map[string]interface{}{
		"totalEquity":    account["equity"],
		"availableCash":  account["cash"],
		"buyingPower":    account["buying_power"],
		"portfolioValue": account["portfolio_value"],
	}

	return result, nil
}

// GetPositions implements types.Trader
func (t *AlpacaTrader) GetPositions() ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := t.makeRequest(ctx, "GET", "positions", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca API error (status %d): %s", resp.StatusCode, string(body))
	}

	var positions []map[string]interface{}
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to NOFX format
	result := make([]map[string]interface{}, len(positions))
	for i, pos := range positions {
		qty, _ := strconv.ParseFloat(pos["qty"].(string), 64)
		avgEntry, _ := strconv.ParseFloat(pos["avg_entry_price"].(string), 64)
		currentPrice, _ := strconv.ParseFloat(pos["current_price"].(string), 64)
		unrealizedPL, _ := strconv.ParseFloat(pos["unrealized_pl"].(string), 64)
		marketValue, _ := strconv.ParseFloat(pos["market_value"].(string), 64)

		result[i] = map[string]interface{}{
			"symbol":        pos["symbol"],
			"qty":           qty,
			"avgEntryPrice": avgEntry,
			"currentPrice":  currentPrice,
			"unrealizedPL":  unrealizedPL,
			"marketValue":   marketValue,
			"positionSide":  strings.ToUpper(pos["side"].(string)),
		}
	}

	return result, nil
}

// OpenLong implements types.Trader
func (t *AlpacaTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.placeOrder(symbol, "buy", quantity, "market", 0, 0)
}

// OpenShort implements types.Trader
func (t *AlpacaTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return t.placeOrder(symbol, "sell", quantity, "market", 0, 0)
}

// CloseLong implements types.Trader
func (t *AlpacaTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.placeOrder(symbol, "sell", quantity, "market", 0, 0)
}

// CloseShort implements types.Trader
func (t *AlpacaTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.placeOrder(symbol, "buy", quantity, "market", 0, 0)
}

// SetLeverage implements types.Trader
// Note: Alpaca crypto does not support leverage
func (t *AlpacaTrader) SetLeverage(symbol string, leverage int) error {
	return fmt.Errorf("leverage not supported for Alpaca crypto")
}

// SetMarginMode implements types.Trader
// Note: Alpaca uses a different margin model
func (t *AlpacaTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return fmt.Errorf("margin mode not supported for Alpaca")
}

// GetMarketPrice implements types.Trader
func (t *AlpacaTrader) GetMarketPrice(symbol string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert symbol to Alpaca format
	alpacaSymbol := alpaca.ConvertSymbolToAlpacaFormat(symbol)

	endpoint := fmt.Sprintf("crypto/%s/latest", alpacaSymbol)
	resp, err := t.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("alpaca API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract price from latest_trade
	if latestTrade, ok := result["latest_trade"].(map[string]interface{}); ok {
		if price, ok := latestTrade["p"].(float64); ok {
			return price, nil
		}
	}

	return 0, fmt.Errorf("price not found in response")
}

// SetStopLoss implements types.Trader
func (t *AlpacaTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	side := "buy"
	if positionSide == "LONG" {
		side = "sell"
	}

	_, err := t.placeOrder(symbol, side, quantity, "stop_limit", stopPrice, stopPrice)
	return err
}

// SetTakeProfit implements types.Trader
func (t *AlpacaTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	side := "buy"
	if positionSide == "LONG" {
		side = "sell"
	}

	_, err := t.placeOrder(symbol, side, quantity, "stop_limit", takeProfitPrice, takeProfitPrice)
	return err
}

// CancelStopLossOrders implements types.Trader
func (t *AlpacaTrader) CancelStopLossOrders(symbol string) error {
	return t.CancelAllOrders(symbol)
}

// CancelTakeProfitOrders implements types.Trader
func (t *AlpacaTrader) CancelTakeProfitOrders(symbol string) error {
	return t.CancelAllOrders(symbol)
}

// CancelAllOrders implements types.Trader
func (t *AlpacaTrader) CancelAllOrders(symbol string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	alpacaSymbol := alpaca.ConvertSymbolToAlpacaFormat(symbol)
	endpoint := fmt.Sprintf("orders?symbols=%s", alpacaSymbol)

	resp, err := t.makeRequest(ctx, "DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to cancel orders: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("alpaca API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// CancelStopOrders implements types.Trader
func (t *AlpacaTrader) CancelStopOrders(symbol string) error {
	return t.CancelAllOrders(symbol)
}

// FormatQuantity implements types.Trader
func (t *AlpacaTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return strconv.FormatFloat(quantity, 'f', 8, 64), nil
}

// GetOrderStatus implements types.Trader
func (t *AlpacaTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("orders/%s", orderID)
	resp, err := t.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca API error (status %d): %s", resp.StatusCode, string(body))
	}

	var order map[string]interface{}
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := map[string]interface{}{
		"orderID":      order["id"],
		"symbol":       order["symbol"],
		"status":       order["status"],
		"filledQty":    order["filled_qty"],
		"avgFillPrice": order["filled_avg_price"],
	}

	return result, nil
}

// GetClosedPnL implements types.Trader
func (t *AlpacaTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	return []types.ClosedPnLRecord{}, nil
}

// GetOpenOrders implements types.Trader
func (t *AlpacaTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	alpacaSymbol := alpaca.ConvertSymbolToAlpacaFormat(symbol)
	endpoint := fmt.Sprintf("orders?status=open&symbols=%s", alpacaSymbol)

	resp, err := t.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca API error (status %d): %s", resp.StatusCode, string(body))
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := make([]types.OpenOrder, len(orders))
	for i, order := range orders {
		qty, _ := strconv.ParseFloat(order["qty"].(string), 64)
		filledQty, _ := strconv.ParseFloat(order["filled_qty"].(string), 64)

		result[i] = types.OpenOrder{
			OrderID:  order["id"].(string),
			Symbol:   order["symbol"].(string),
			Side:     strings.ToUpper(order["side"].(string)),
			Quantity: qty,
			Status:   order["status"].(string),
		}

		if order["type"] == "limit" {
			result[i].Type = "LIMIT"
		} else if order["type"] == "stop_limit" {
			result[i].Type = "STOP_LIMIT"
		}

		if filledQty > 0 && filledQty < qty {
			result[i].Status = "PARTIALLY_FILLED"
		}
	}

	return result, nil
}

// placeOrder is a helper to place an order
func (t *AlpacaTrader) placeOrder(symbol, side string, quantity float64, orderType string, limitPrice, stopPrice float64) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	alpacaSymbol := alpaca.ConvertSymbolToAlpacaFormat(symbol)

	orderReq := map[string]interface{}{
		"symbol":        alpacaSymbol,
		"qty":           strconv.FormatFloat(quantity, 'f', 8, 64),
		"side":          side,
		"type":          orderType,
		"time_in_force": "gtc",
	}

	if orderType == "limit" && limitPrice > 0 {
		orderReq["limit_price"] = strconv.FormatFloat(limitPrice, 'f', 2, 64)
	}

	if orderType == "stop_limit" && stopPrice > 0 {
		orderReq["stop_price"] = strconv.FormatFloat(stopPrice, 'f', 2, 64)
		if limitPrice > 0 {
			orderReq["limit_price"] = strconv.FormatFloat(limitPrice, 'f', 2, 64)
		}
	}

	jsonBody, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order: %w", err)
	}

	resp, err := t.makeRequest(ctx, "POST", "orders", strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("alpaca API error (status %d): %s", resp.StatusCode, string(body))
	}

	var order map[string]interface{}
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := map[string]interface{}{
		"orderID":  order["id"],
		"symbol":   order["symbol"],
		"side":     order["side"],
		"qty":      order["qty"],
		"type":     order["type"],
		"status":   order["status"],
		"clientID": order["client_order_id"],
	}

	return result, nil
}
