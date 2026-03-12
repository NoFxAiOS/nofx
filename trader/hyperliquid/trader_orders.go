package hyperliquid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
	"time"

	"github.com/sonirico/go-hyperliquid"
)

// OpenLong opens a long position (supports both crypto and xyz dex)
func (t *HyperliquidTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old pending orders: %v", err)
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	isXyz := strings.HasPrefix(coin, "xyz:")

	// Set leverage (skip for xyz dex as it may not support leverage adjustment)
	if !isXyz {
		if err := t.SetLeverage(symbol, leverage); err != nil {
			return nil, err
		}
	} else {
		logger.Infof("  ℹ xyz dex asset %s - using default leverage", coin)
	}

	// Get current price (for market order)
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Price needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Handle xyz dex assets differently
	if isXyz {
		// xyz dex order
		if err := t.placeXyzOrder(coin, true, quantity, aggressivePrice, false); err != nil {
			return nil, fmt.Errorf("failed to open long position on xyz dex: %w", err)
		}
	} else {
		// Standard crypto order
		roundedQuantity := t.roundToSzDecimals(coin, quantity)
		logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: true,
			Size:  roundedQuantity,
			Price: aggressivePrice,
			OrderType: hyperliquid.OrderType{
				Limit: &hyperliquid.LimitOrderType{
					Tif: hyperliquid.TifIoc,
				},
			},
			ReduceOnly: false,
		}

		_, err = t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return nil, fmt.Errorf("failed to open long position: %w", err)
		}
	}

	logger.Infof("✓ Long position opened successfully: %s quantity: %.4f", symbol, quantity)

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// OpenShort opens a short position (supports both crypto and xyz dex)
func (t *HyperliquidTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// First cancel all pending orders for this coin
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel old pending orders: %v", err)
	}

	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	isXyz := strings.HasPrefix(coin, "xyz:")

	// Set leverage (skip for xyz dex)
	if !isXyz {
		if err := t.SetLeverage(symbol, leverage); err != nil {
			return nil, err
		}
	} else {
		logger.Infof("  ℹ xyz dex asset %s - using default leverage", coin)
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Price needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Handle xyz dex assets differently
	if isXyz {
		// xyz dex order
		if err := t.placeXyzOrder(coin, false, quantity, aggressivePrice, false); err != nil {
			return nil, fmt.Errorf("failed to open short position on xyz dex: %w", err)
		}
	} else {
		// Standard crypto order
		roundedQuantity := t.roundToSzDecimals(coin, quantity)
		logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: false,
			Size:  roundedQuantity,
			Price: aggressivePrice,
			OrderType: hyperliquid.OrderType{
				Limit: &hyperliquid.LimitOrderType{
					Tif: hyperliquid.TifIoc,
				},
			},
			ReduceOnly: false,
		}

		_, err = t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return nil, fmt.Errorf("failed to open short position: %w", err)
		}
	}

	logger.Infof("✓ Short position opened successfully: %s quantity: %.4f", symbol, quantity)

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseLong closes a long position (supports both crypto and xyz dex)
func (t *HyperliquidTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)
	isXyz := strings.HasPrefix(coin, "xyz:")

	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		// For xyz dex, also check xyz: prefixed symbols
		searchSymbol := symbol
		if isXyz {
			searchSymbol = coin // Use xyz:SYMBOL format for comparison
		}

		for _, pos := range positions {
			posSymbol := pos["symbol"].(string)
			if (posSymbol == symbol || posSymbol == searchSymbol) && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Price needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 0.99)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*0.99, aggressivePrice)

	// Handle xyz dex assets differently
	if isXyz {
		// xyz dex close order
		if err := t.placeXyzOrder(coin, false, quantity, aggressivePrice, true); err != nil {
			return nil, fmt.Errorf("failed to close long position on xyz dex: %w", err)
		}
	} else {
		// Standard crypto close order
		roundedQuantity := t.roundToSzDecimals(coin, quantity)
		logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: false,
			Size:  roundedQuantity,
			Price: aggressivePrice,
			OrderType: hyperliquid.OrderType{
				Limit: &hyperliquid.LimitOrderType{
					Tif: hyperliquid.TifIoc,
				},
			},
			ReduceOnly: true,
		}

		_, err = t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return nil, fmt.Errorf("failed to close long position: %w", err)
		}
	}

	logger.Infof("✓ Long position closed successfully: %s quantity: %.4f", symbol, quantity)

	// Cancel all pending orders for this coin after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CloseShort closes a short position (supports both crypto and xyz dex)
func (t *HyperliquidTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// Hyperliquid symbol format
	coin := convertSymbolToHyperliquid(symbol)
	isXyz := strings.HasPrefix(coin, "xyz:")

	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		// For xyz dex, also check xyz: prefixed symbols
		searchSymbol := symbol
		if isXyz {
			searchSymbol = coin
		}

		for _, pos := range positions {
			posSymbol := pos["symbol"].(string)
			if (posSymbol == symbol || posSymbol == searchSymbol) && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Get current price
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Price needs to be processed to 5 significant figures
	aggressivePrice := t.roundPriceToSigfigs(price * 1.01)
	logger.Infof("  💰 Price precision handling: %.8f -> %.8f (5 significant figures)", price*1.01, aggressivePrice)

	// Handle xyz dex assets differently
	if isXyz {
		// xyz dex close order
		if err := t.placeXyzOrder(coin, true, quantity, aggressivePrice, true); err != nil {
			return nil, fmt.Errorf("failed to close short position on xyz dex: %w", err)
		}
	} else {
		// Standard crypto close order
		roundedQuantity := t.roundToSzDecimals(coin, quantity)
		logger.Infof("  📏 Quantity precision handling: %.8f -> %.8f (szDecimals=%d)", quantity, roundedQuantity, t.getSzDecimals(coin))

		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: true,
			Size:  roundedQuantity,
			Price: aggressivePrice,
			OrderType: hyperliquid.OrderType{
				Limit: &hyperliquid.LimitOrderType{
					Tif: hyperliquid.TifIoc,
				},
			},
			ReduceOnly: true,
		}

		_, err = t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return nil, fmt.Errorf("failed to close short position: %w", err)
		}
	}

	logger.Infof("✓ Short position closed successfully: %s quantity: %.4f", symbol, quantity)

	// Cancel all pending orders for this coin after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = 0
	result["symbol"] = symbol
	result["status"] = "FILLED"

	return result, nil
}

// CancelStopLossOrders only cancels stop loss orders (Hyperliquid cannot distinguish stop loss and take profit, cancel all)
func (t *HyperliquidTrader) CancelStopLossOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Cannot distinguish stop loss and take profit orders, so cancel all pending orders for this coin
	logger.Infof("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelTakeProfitOrders only cancels take profit orders (Hyperliquid cannot distinguish stop loss and take profit, cancel all)
func (t *HyperliquidTrader) CancelTakeProfitOrders(symbol string) error {
	// Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Cannot distinguish stop loss and take profit orders, so cancel all pending orders for this coin
	logger.Infof("  ⚠️ Hyperliquid cannot distinguish stop loss/take profit orders, will cancel all pending orders")
	return t.CancelStopOrders(symbol)
}

// CancelAllOrders cancels all pending orders for this coin
func (t *HyperliquidTrader) CancelAllOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	isXyz := strings.HasPrefix(coin, "xyz:")

	if isXyz {
		// xyz dex orders - use direct API call
		return t.cancelXyzOrders(coin)
	}

	// Standard crypto orders
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Cancel all pending orders for this coin
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				logger.Infof("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
			}
		}
	}

	logger.Infof("  ✓ Cancelled all pending orders for %s", symbol)
	return nil
}

// CancelStopOrders cancels take profit/stop loss orders for this coin (used to adjust TP/SL positions)
func (t *HyperliquidTrader) CancelStopOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Check if this is an xyz dex asset
	isXyz := strings.HasPrefix(coin, "xyz:")

	if isXyz {
		// xyz dex orders - use direct API call
		return t.cancelXyzOrders(coin)
	}

	// Get all pending orders for standard crypto
	openOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	// Note: Hyperliquid SDK's OpenOrder structure does not expose trigger field
	// Therefore temporarily cancel all pending orders for this coin (including TP/SL orders)
	// This is safe because all old orders should be cleaned up before setting new TP/SL
	canceledCount := 0
	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := t.exchange.Cancel(t.ctx, coin, order.Oid)
			if err != nil {
				logger.Infof("  ⚠ Failed to cancel order (oid=%d): %v", order.Oid, err)
				continue
			}
			canceledCount++
		}
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ No pending orders to cancel for %s", symbol)
	} else {
		logger.Infof("  ✓ Cancelled %d pending orders for %s (including TP/SL orders)", canceledCount, symbol)
	}

	return nil
}

// cancelXyzOrders cancels all pending orders for xyz dex assets (stocks, forex, commodities)
func (t *HyperliquidTrader) cancelXyzOrders(coin string) error {
	// Query xyz dex open orders
	reqBody := map[string]interface{}{
		"type": "openOrders",
		"user": t.walletAddr,
		"dex":  "xyz",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := "https://api.hyperliquid.xyz/info"

	req, err := http.NewRequestWithContext(t.ctx, "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("xyz dex openOrders API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse open orders
	var openOrders []struct {
		Coin string `json:"coin"`
		Oid  int64  `json:"oid"`
	}
	if err := json.Unmarshal(body, &openOrders); err != nil {
		return fmt.Errorf("failed to parse open orders: %w", err)
	}

	// Filter orders for this coin and cancel them
	canceledCount := 0
	for _, order := range openOrders {
		if order.Coin == coin {
			if err := t.cancelXyzOrder(order.Oid); err != nil {
				logger.Infof("  ⚠ Failed to cancel xyz dex order (oid=%d): %v", order.Oid, err)
				continue
			}
			canceledCount++
		}
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ No pending xyz dex orders to cancel for %s", coin)
	} else {
		logger.Infof("  ✓ Cancelled %d xyz dex orders for %s", canceledCount, coin)
	}

	return nil
}

// cancelXyzOrder cancels a single xyz dex order by oid
func (t *HyperliquidTrader) cancelXyzOrder(oid int64) error {
	// Get asset index for this order (we need it for cancel action)
	// For cancel, we construct a cancel action with the oid

	action := map[string]interface{}{
		"type": "cancel",
		"cancels": []map[string]interface{}{
			{
				"a": oid, // asset index not needed for cancel by oid in xyz dex
				"o": oid,
			},
		},
	}

	// Sign the action
	nonce := time.Now().UnixMilli()
	isMainnet := !t.isTestnet
	vaultAddress := ""

	sig, err := hyperliquid.SignL1Action(t.privateKey, action, vaultAddress, nonce, nil, isMainnet)
	if err != nil {
		return fmt.Errorf("failed to sign cancel action: %w", err)
	}

	payload := map[string]any{
		"action":    action,
		"nonce":     nonce,
		"signature": sig,
	}

	apiURL := hyperliquid.MainnetAPIURL
	if t.isTestnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, apiURL+"/exchange", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check response
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "ok" {
		return fmt.Errorf("cancel failed: %s", string(body))
	}

	return nil
}

// floatToWireStr converts a float to wire format string (8 decimal places, trimmed zeros)
// This matches the SDK's floatToWire function
func floatToWireStr(x float64) string {
	// Format to 8 decimal places
	result := fmt.Sprintf("%.8f", x)
	// Remove trailing zeros
	result = strings.TrimRight(result, "0")
	// Remove trailing decimal point if no decimals left
	result = strings.TrimRight(result, ".")
	return result
}

// placeXyzOrder places an order on the xyz dex (stocks, forex, commodities)
// Note: xyz dex orders use builder-deployed perpetuals and require different handling
// xyz dex asset indices start from 10000 (10000 + meta_index)
// This implementation bypasses the SDK's NameToAsset lookup and directly constructs the order
func (t *HyperliquidTrader) placeXyzOrder(coin string, isBuy bool, size float64, price float64, reduceOnly bool) error {
	// Fetch xyz meta if not cached
	t.xyzMetaMutex.RLock()
	hasMeta := t.xyzMeta != nil
	t.xyzMetaMutex.RUnlock()

	if !hasMeta {
		if err := t.fetchXyzMeta(); err != nil {
			return fmt.Errorf("failed to fetch xyz meta: %w", err)
		}
	}

	// Get asset index from xyz meta (returns 0-based index)
	metaIndex := t.getXyzAssetIndex(coin)
	if metaIndex < 0 {
		return fmt.Errorf("xyz asset %s not found in meta", coin)
	}

	// HIP-3 perp dex asset index formula: 100000 + perp_dex_index * 10000 + index_in_meta
	// xyz dex is at perp_dex_index = 1 (verified from perpDexs API: [null, {name:"xyz",...}])
	// So xyz asset index = 100000 + 1 * 10000 + metaIndex = 110000 + metaIndex
	const xyzPerpDexIndex = 1
	assetIndex := 100000 + xyzPerpDexIndex*10000 + metaIndex

	// Round size to correct precision
	szDecimals := t.getXyzSzDecimals(coin)
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}
	roundedSize := float64(int(size*multiplier+0.5)) / multiplier

	// Round price to 5 significant figures
	roundedPrice := t.roundPriceToSigfigs(price)

	logger.Infof("📝 Placing xyz dex order (direct): %s %s size=%.4f price=%.4f metaIndex=%d assetIndex=%d (formula: 100000 + 1*10000 + %d) reduceOnly=%v",
		map[bool]string{true: "BUY", false: "SELL"}[isBuy],
		coin, roundedSize, roundedPrice, metaIndex, assetIndex, metaIndex, reduceOnly)

	// Construct OrderWire directly with correct asset index (bypassing SDK's NameToAsset)
	orderWire := hyperliquid.OrderWire{
		Asset:      assetIndex,
		IsBuy:      isBuy,
		LimitPx:    floatToWireStr(roundedPrice),
		Size:       floatToWireStr(roundedSize),
		ReduceOnly: reduceOnly,
		OrderType: hyperliquid.OrderWireType{
			Limit: &hyperliquid.OrderWireTypeLimit{
				Tif: hyperliquid.TifIoc,
			},
		},
	}

	// Create OrderAction (no builder to avoid requiring builder fee approval)
	action := hyperliquid.OrderAction{
		Type:     "order",
		Orders:   []hyperliquid.OrderWire{orderWire},
		Grouping: "na",
		Builder:  nil,
	}

	// Sign the action
	nonce := time.Now().UnixMilli()
	isMainnet := !t.isTestnet
	vaultAddress := "" // No vault for personal account

	sig, err := hyperliquid.SignL1Action(t.privateKey, action, vaultAddress, nonce, nil, isMainnet)
	if err != nil {
		return fmt.Errorf("failed to sign xyz dex order: %w", err)
	}

	// Construct payload for /exchange endpoint
	payload := map[string]any{
		"action":    action,
		"nonce":     nonce,
		"signature": sig,
	}

	// Determine API URL
	apiURL := hyperliquid.MainnetAPIURL
	if t.isTestnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// POST to /exchange
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	logger.Infof("📤 Sending xyz dex order to %s/exchange", apiURL)

	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, apiURL+"/exchange", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var result struct {
		Status   string `json:"status"`
		Response struct {
			Type string `json:"type"`
			Data struct {
				Statuses []struct {
					Resting *struct {
						Oid int64 `json:"oid"`
					} `json:"resting,omitempty"`
					Filled *struct {
						TotalSz string `json:"totalSz"`
						AvgPx   string `json:"avgPx"`
						Oid     int    `json:"oid"`
					} `json:"filled,omitempty"`
					Error *string `json:"error,omitempty"`
				} `json:"statuses"`
			} `json:"data"`
		} `json:"response"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		// Try to parse as error response
		logger.Infof("⚠️  Failed to parse response as success, raw body: %s", string(body))
		return fmt.Errorf("xyz dex order failed, status=%d, body=%s", resp.StatusCode, string(body))
	}

	// Check for errors in response
	if result.Status != "ok" {
		return fmt.Errorf("xyz dex order failed: status=%s, body=%s", result.Status, string(body))
	}

	// Check order statuses
	if len(result.Response.Data.Statuses) > 0 {
		status := result.Response.Data.Statuses[0]
		if status.Error != nil {
			return fmt.Errorf("xyz dex order error (coin=%s, assetIndex=%d, size=%.4f, price=%.4f): %s", coin, assetIndex, roundedSize, roundedPrice, *status.Error)
		}
		if status.Filled != nil {
			logger.Infof("✅ xyz dex order filled: totalSz=%s avgPx=%s oid=%d",
				status.Filled.TotalSz, status.Filled.AvgPx, status.Filled.Oid)
		} else if status.Resting != nil {
			logger.Infof("✅ xyz dex order resting: oid=%d", status.Resting.Oid)
		}
	}

	logger.Infof("✅ xyz dex order placed successfully: %s (response: %s)", coin, string(body))
	return nil
}

// placeXyzTriggerOrder places a trigger order (stop loss / take profit) on the xyz dex
// tpsl: "sl" for stop loss, "tp" for take profit
func (t *HyperliquidTrader) placeXyzTriggerOrder(coin string, isBuy bool, size float64, triggerPrice float64, tpsl string) error {
	// Fetch xyz meta if not cached
	t.xyzMetaMutex.RLock()
	hasMeta := t.xyzMeta != nil
	t.xyzMetaMutex.RUnlock()

	if !hasMeta {
		if err := t.fetchXyzMeta(); err != nil {
			return fmt.Errorf("failed to fetch xyz meta: %w", err)
		}
	}

	// Get asset index from xyz meta (returns 0-based index)
	metaIndex := t.getXyzAssetIndex(coin)
	if metaIndex < 0 {
		return fmt.Errorf("xyz asset %s not found in meta", coin)
	}

	// HIP-3 perp dex asset index formula: 100000 + perp_dex_index * 10000 + index_in_meta
	// xyz dex is at perp_dex_index = 1
	const xyzPerpDexIndex = 1
	assetIndex := 100000 + xyzPerpDexIndex*10000 + metaIndex

	// Round size to correct precision
	szDecimals := t.getXyzSzDecimals(coin)
	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}
	roundedSize := float64(int(size*multiplier+0.5)) / multiplier

	// Round price to 5 significant figures
	roundedPrice := t.roundPriceToSigfigs(triggerPrice)

	logger.Infof("📝 Placing xyz dex %s order: %s %s size=%.4f triggerPrice=%.4f assetIndex=%d",
		tpsl,
		map[bool]string{true: "BUY", false: "SELL"}[isBuy],
		coin, roundedSize, roundedPrice, assetIndex)

	// Construct OrderWire with trigger type for stop loss / take profit
	orderWire := hyperliquid.OrderWire{
		Asset:      assetIndex,
		IsBuy:      isBuy,
		LimitPx:    floatToWireStr(roundedPrice),
		Size:       floatToWireStr(roundedSize),
		ReduceOnly: true, // TP/SL orders are always reduce-only
		OrderType: hyperliquid.OrderWireType{
			Trigger: &hyperliquid.OrderWireTypeTrigger{
				TriggerPx: floatToWireStr(roundedPrice),
				IsMarket:  true,
				Tpsl:      hyperliquid.Tpsl(tpsl), // "sl" or "tp" - convert string to Tpsl type
			},
		},
	}

	// Create OrderAction (no builder to avoid requiring builder fee approval)
	action := hyperliquid.OrderAction{
		Type:     "order",
		Orders:   []hyperliquid.OrderWire{orderWire},
		Grouping: "na",
		Builder:  nil,
	}

	// Sign the action
	nonce := time.Now().UnixMilli()
	isMainnet := !t.isTestnet
	vaultAddress := ""

	sig, err := hyperliquid.SignL1Action(t.privateKey, action, vaultAddress, nonce, nil, isMainnet)
	if err != nil {
		return fmt.Errorf("failed to sign xyz dex trigger order: %w", err)
	}

	// Construct payload for /exchange endpoint
	payload := map[string]any{
		"action":    action,
		"nonce":     nonce,
		"signature": sig,
	}

	// Determine API URL
	apiURL := hyperliquid.MainnetAPIURL
	if t.isTestnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	// POST to /exchange
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	logger.Infof("📤 Sending xyz dex %s order to %s/exchange", tpsl, apiURL)

	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, apiURL+"/exchange", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var result struct {
		Status   string `json:"status"`
		Response struct {
			Type string `json:"type"`
			Data struct {
				Statuses []struct {
					Resting *struct {
						Oid int64 `json:"oid"`
					} `json:"resting,omitempty"`
					Error *string `json:"error,omitempty"`
				} `json:"statuses"`
			} `json:"data"`
		} `json:"response"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		logger.Infof("⚠️  Failed to parse response, raw body: %s", string(body))
		return fmt.Errorf("xyz dex %s order failed, status=%d, body=%s", tpsl, resp.StatusCode, string(body))
	}

	// Check for errors in response
	if result.Status != "ok" {
		return fmt.Errorf("xyz dex %s order failed: status=%s, body=%s", tpsl, result.Status, string(body))
	}

	// Check order statuses
	if len(result.Response.Data.Statuses) > 0 {
		status := result.Response.Data.Statuses[0]
		if status.Error != nil {
			return fmt.Errorf("xyz dex %s order error: %s", tpsl, *status.Error)
		}
		if status.Resting != nil {
			logger.Infof("✅ xyz dex %s order placed: oid=%d", tpsl, status.Resting.Oid)
		}
	}

	logger.Infof("✅ xyz dex %s order placed successfully: %s", tpsl, coin)
	return nil
}

// SetStopLoss sets stop loss order
func (t *HyperliquidTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position stop loss = buy, long position stop loss = sell

	// Price needs to be processed to 5 significant figures
	roundedStopPrice := t.roundPriceToSigfigs(stopPrice)

	// Check if this is an xyz dex asset (stocks, forex, commodities)
	isXyz := strings.HasPrefix(coin, "xyz:")

	if isXyz {
		// xyz dex stop loss order - use direct API call similar to placeXyzOrder
		if err := t.placeXyzTriggerOrder(coin, isBuy, quantity, roundedStopPrice, "sl"); err != nil {
			return fmt.Errorf("failed to set xyz dex stop loss: %w", err)
		}
	} else {
		// Standard crypto stop loss order
		// Round quantity according to coin precision requirements
		roundedQuantity := t.roundToSzDecimals(coin, quantity)

		// Create stop loss order (Trigger Order)
		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: isBuy,
			Size:  roundedQuantity,  // Use rounded quantity
			Price: roundedStopPrice, // Use processed price
			OrderType: hyperliquid.OrderType{
				Trigger: &hyperliquid.TriggerOrderType{
					TriggerPx: roundedStopPrice,
					IsMarket:  true,
					Tpsl:      "sl", // stop loss
				},
			},
			ReduceOnly: true,
		}

		_, err := t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return fmt.Errorf("failed to set stop loss: %w", err)
		}
	}

	logger.Infof("  Stop loss price set: %.4f", roundedStopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *HyperliquidTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	coin := convertSymbolToHyperliquid(symbol)

	isBuy := positionSide == "SHORT" // Short position take profit = buy, long position take profit = sell

	// Price needs to be processed to 5 significant figures
	roundedTakeProfitPrice := t.roundPriceToSigfigs(takeProfitPrice)

	// Check if this is an xyz dex asset (stocks, forex, commodities)
	isXyz := strings.HasPrefix(coin, "xyz:")

	if isXyz {
		// xyz dex take profit order - use direct API call similar to placeXyzOrder
		if err := t.placeXyzTriggerOrder(coin, isBuy, quantity, roundedTakeProfitPrice, "tp"); err != nil {
			return fmt.Errorf("failed to set xyz dex take profit: %w", err)
		}
	} else {
		// Standard crypto take profit order
		// Round quantity according to coin precision requirements
		roundedQuantity := t.roundToSzDecimals(coin, quantity)

		// Create take profit order (Trigger Order)
		order := hyperliquid.CreateOrderRequest{
			Coin:  coin,
			IsBuy: isBuy,
			Size:  roundedQuantity,        // Use rounded quantity
			Price: roundedTakeProfitPrice, // Use processed price
			OrderType: hyperliquid.OrderType{
				Trigger: &hyperliquid.TriggerOrderType{
					TriggerPx: roundedTakeProfitPrice,
					IsMarket:  true,
					Tpsl:      "tp", // take profit
				},
			},
			ReduceOnly: true,
		}

		_, err := t.exchange.Order(t.ctx, order, defaultBuilder)
		if err != nil {
			return fmt.Errorf("failed to set take profit: %w", err)
		}
	}

	logger.Infof("  Take profit price set: %.4f", roundedTakeProfitPrice)
	return nil
}

// PlaceLimitOrder places a limit order for grid trading
// Implements GridTrader interface
func (t *HyperliquidTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	coin := convertSymbolToHyperliquid(req.Symbol)

	// Set leverage if specified and not xyz dex
	isXyz := strings.HasPrefix(coin, "xyz:")
	if req.Leverage > 0 && !isXyz {
		if err := t.SetLeverage(req.Symbol, req.Leverage); err != nil {
			logger.Warnf("[Hyperliquid] Failed to set leverage: %v", err)
		}
	}

	// Round quantity to allowed decimals
	roundedQuantity := t.roundToSzDecimals(coin, req.Quantity)

	// Round price to 5 significant figures
	roundedPrice := t.roundPriceToSigfigs(req.Price)

	// Determine if buy or sell
	isBuy := req.Side == "BUY"

	logger.Infof("[Hyperliquid] PlaceLimitOrder: %s %s @ %.4f, qty=%.4f", coin, req.Side, roundedPrice, roundedQuantity)

	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: isBuy,
		Size:  roundedQuantity,
		Price: roundedPrice,
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifGtc, // Good Till Cancel for grid orders
			},
		},
		ReduceOnly: req.ReduceOnly,
	}

	_, err := t.exchange.Order(t.ctx, order, defaultBuilder)
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", err)
	}

	// Note: Hyperliquid's Order response doesn't return the order ID directly
	// We would need to query open orders to get it, but for grid trading
	// we can track orders by price level instead
	orderID := fmt.Sprintf("%d", time.Now().UnixNano())

	logger.Infof("✓ [Hyperliquid] Limit order placed: %s %s @ %.4f",
		coin, req.Side, roundedPrice)

	return &types.LimitOrderResult{
		OrderID:      orderID,
		ClientID:     req.ClientID,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Price:        roundedPrice,
		Quantity:     roundedQuantity,
		Status:       "NEW",
	}, nil
}

// CancelOrder cancels a specific order by ID
// Implements GridTrader interface
func (t *HyperliquidTrader) CancelOrder(symbol, orderID string) error {
	coin := convertSymbolToHyperliquid(symbol)

	// Parse order ID
	oid, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order ID: %w", err)
	}

	_, err = t.exchange.Cancel(t.ctx, coin, oid)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	logger.Infof("✓ [Hyperliquid] Order cancelled: %s %s", symbol, orderID)
	return nil
}
