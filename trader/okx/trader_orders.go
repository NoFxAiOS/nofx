package okx

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"
)

// OpenLong opens long position
func (t *OKXTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel old orders
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	instId := t.convertSymbol(symbol)

	// Get instrument info and calculate contract size
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// OKX uses contract count, need to convert quantity (in base asset) to contract count
	// sz = quantity / ctVal (number of contracts = asset amount / asset per contract)
	sz := quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	logger.Infof("  📊 OKX OpenLong: quantity=%.6f, ctVal=%.6f, contracts=%.2f", quantity, inst.CtVal, sz)

	// Check max market order size limit
	if inst.MaxMktSz > 0 && sz > inst.MaxMktSz {
		logger.Infof("  ⚠️ OKX market order size %.2f exceeds max %.2f, reducing to max", sz, inst.MaxMktSz)
		sz = inst.MaxMktSz
		szStr = t.formatSize(sz, inst)
	}

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
		return nil, fmt.Errorf("failed to open long position: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "unknown error"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("failed to open long position: %s", msg)
	}

	logger.Infof("✓ OKX opened long position successfully: %s size: %s", symbol, szStr)
	logger.Infof("  Order ID: %s", orders[0].OrdId)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// OpenShort opens short position
func (t *OKXTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// Cancel old orders
	t.CancelAllOrders(symbol)

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("  ⚠️ Failed to set leverage: %v", err)
	}

	instId := t.convertSymbol(symbol)

	// Get instrument info and calculate contract size
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// OKX uses contract count, need to convert quantity (in base asset) to contract count
	// sz = quantity / ctVal (number of contracts = asset amount / asset per contract)
	sz := quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	logger.Infof("  📊 OKX OpenShort: quantity=%.6f, ctVal=%.6f, contracts=%.2f", quantity, inst.CtVal, sz)

	// Check max market order size limit
	if inst.MaxMktSz > 0 && sz > inst.MaxMktSz {
		logger.Infof("  ⚠️ OKX market order size %.2f exceeds max %.2f, reducing to max", sz, inst.MaxMktSz)
		sz = inst.MaxMktSz
		szStr = t.formatSize(sz, inst)
	}

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
		return nil, fmt.Errorf("failed to open short position: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "unknown error"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("failed to open short position: %s", msg)
	}

	logger.Infof("✓ OKX opened short position successfully: %s size: %s", symbol, szStr)
	logger.Infof("  Order ID: %s", orders[0].OrdId)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseLong closes long position
func (t *OKXTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)

	// Get instrument info for contract conversion
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Invalidate position cache and get fresh positions
	t.InvalidatePositionCache()
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	// Find actual position from exchange
	var actualQty float64
	var posFound bool
	var posMgnMode string = "cross" // Default to cross margin
	logger.Infof("🔍 OKX CloseLong: searching for symbol=%s in %d positions", symbol, len(positions))
	for _, pos := range positions {
		logger.Infof("🔍 OKX position: symbol=%v, side=%v, positionAmt=%v, mgnMode=%v", pos["symbol"], pos["side"], pos["positionAmt"], pos["mgnMode"])
		if pos["symbol"] == symbol {
			side := pos["side"].(string)
			// In net_mode, "long" means positive position
			// In dual mode, check explicit "long" side
			if side == "long" || (t.positionMode == "net_mode" && side == "long") {
				actualQty = pos["positionAmt"].(float64)
				posFound = true
				if mgnMode, ok := pos["mgnMode"].(string); ok && mgnMode != "" {
					posMgnMode = mgnMode
				}
				logger.Infof("🔍 OKX CloseLong: found matching position! qty=%.6f, mgnMode=%s", actualQty, posMgnMode)
				break
			}
		}
	}

	if !posFound || actualQty == 0 {
		logger.Infof("🔍 OKX CloseLong: NO position found for %s LONG", symbol)
		return map[string]interface{}{
			"status":  "NO_POSITION",
			"message": fmt.Sprintf("No long position found for %s on OKX", symbol),
		}, nil
	}

	// Use actual quantity from exchange (more accurate than passed quantity)
	if quantity == 0 || quantity > actualQty {
		quantity = actualQty
	}

	// Convert quantity (base asset) to contract count
	// contracts = quantity / ctVal
	contracts := quantity / inst.CtVal
	szStr := t.formatSize(contracts, inst)

	logger.Infof("🔻 OKX close long: symbol=%s, instId=%s, quantity=%.6f, ctVal=%.6f, contracts=%.2f, szStr=%s, posMode=%s, mgnMode=%s",
		symbol, instId, quantity, inst.CtVal, contracts, szStr, t.positionMode, posMgnMode)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  posMgnMode, // Use position's actual margin mode (cross or isolated)
		"side":    "sell",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	// Only add posSide in dual mode (long_short_mode)
	if t.positionMode == "long_short_mode" {
		body["posSide"] = "long"
	}

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", err)
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
		msg := "unknown error"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("failed to close long position: %s", msg)
	}

	logger.Infof("✓ OKX closed long position successfully: %s", symbol)

	// Cancel pending orders after closing position
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort closes short position
func (t *OKXTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)

	// Get instrument info for contract conversion
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Invalidate position cache and get fresh positions
	t.InvalidatePositionCache()
	positions, err := t.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	// Find actual position from exchange
	var actualQty float64
	var posFound bool
	var posMgnMode string = "cross" // Default to cross margin
	logger.Infof("🔍 OKX CloseShort searching positions: symbol=%s, current position count=%d", symbol, len(positions))
	for _, pos := range positions {
		logger.Infof("🔍 OKX position: symbol=%v, side=%v, positionAmt=%v, mgnMode=%v",
			pos["symbol"], pos["side"], pos["positionAmt"], pos["mgnMode"])
		if pos["symbol"] == symbol && pos["side"] == "short" {
			actualQty = pos["positionAmt"].(float64)
			posFound = true
			if mgnMode, ok := pos["mgnMode"].(string); ok && mgnMode != "" {
				posMgnMode = mgnMode
			}
			logger.Infof("🔍 OKX found short position: quantity=%f (base asset), mgnMode=%s", actualQty, posMgnMode)
			break
		}
	}

	if !posFound || actualQty == 0 {
		return map[string]interface{}{
			"status":  "NO_POSITION",
			"message": fmt.Sprintf("No short position found for %s on OKX", symbol),
		}, nil
	}

	// Use actual quantity from exchange (more accurate than passed quantity)
	if quantity == 0 || quantity > actualQty {
		quantity = actualQty
	}

	// Ensure quantity is positive (OKX sz parameter must be positive)
	if quantity < 0 {
		quantity = -quantity
	}

	// Convert quantity (base asset) to contract count
	// contracts = quantity / ctVal
	contracts := quantity / inst.CtVal
	szStr := t.formatSize(contracts, inst)

	logger.Infof("🔻 OKX close short: symbol=%s, quantity=%.6f, ctVal=%.6f, contracts=%.2f, szStr=%s, posMode=%s, mgnMode=%s",
		symbol, quantity, inst.CtVal, contracts, szStr, t.positionMode, posMgnMode)

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  posMgnMode, // Use position's actual margin mode (cross or isolated)
		"side":    "buy",
		"ordType": "market",
		"sz":      szStr,
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	// Only add posSide in dual mode (long_short_mode)
	if t.positionMode == "long_short_mode" {
		body["posSide"] = "short"
	}

	logger.Infof("🔻 OKX close short request body: %+v", body)

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", err)
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
		msg := "unknown error"
		if len(orders) > 0 {
			msg = fmt.Sprintf("sCode=%s, sMsg=%s", orders[0].SCode, orders[0].SMsg)
		}
		logger.Infof("❌ OKX failed to close short position: %s, response: %s", msg, string(data))
		return nil, fmt.Errorf("failed to close short position: %s", msg)
	}

	logger.Infof("✓ OKX closed short position successfully: %s, ordId=%s", symbol, orders[0].OrdId)

	// Cancel pending orders after closing position
	t.CancelAllOrders(symbol)

	return map[string]interface{}{
		"orderId": orders[0].OrdId,
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// SetStopLoss sets stop loss order
func (t *OKXTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	instId := t.convertSymbol(symbol)

	// Get instrument info
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Calculate contract size: quantity (in base asset) / ctVal (asset per contract)
	sz := quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// Determine direction
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
		"slOrdPx":     "-1", // Market price
		"tag":         okxTag,
	}

	_, err = t.doRequest("POST", okxAlgoOrderPath, body)
	if err != nil {
		return fmt.Errorf("failed to set stop loss: %w", err)
	}

	logger.Infof("  Stop loss price set: %.4f", stopPrice)
	return nil
}

// SetTakeProfit sets take profit order
func (t *OKXTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	instId := t.convertSymbol(symbol)

	// Get instrument info
	inst, err := t.getInstrument(symbol)
	if err != nil {
		return fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Calculate contract size: quantity (in base asset) / ctVal (asset per contract)
	sz := quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// Determine direction
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
		"tpOrdPx":     "-1", // Market price
		"tag":         okxTag,
	}

	_, err = t.doRequest("POST", okxAlgoOrderPath, body)
	if err != nil {
		return fmt.Errorf("failed to set take profit: %w", err)
	}

	logger.Infof("  Take profit price set: %.4f", takeProfitPrice)
	return nil
}

// CancelStopLossOrders cancels stop loss orders
func (t *OKXTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "sl")
}

// CancelTakeProfitOrders cancels take profit orders
func (t *OKXTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "tp")
}

// cancelAlgoOrders cancels algo orders
func (t *OKXTrader) cancelAlgoOrders(symbol string, orderType string) error {
	instId := t.convertSymbol(symbol)

	// Get pending algo orders
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
			logger.Infof("  ⚠️ Failed to cancel algo order: %v", err)
			continue
		}
		canceledCount++
	}

	if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d algo orders for %s", canceledCount, symbol)
	}

	return nil
}

// CancelAllOrders cancels all pending orders
func (t *OKXTrader) CancelAllOrders(symbol string) error {
	instId := t.convertSymbol(symbol)

	// Get pending orders
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

	// Batch cancel
	for _, order := range orders {
		body := map[string]interface{}{
			"instId": order.InstId,
			"ordId":  order.OrdId,
		}
		t.doRequest("POST", okxCancelOrderPath, body)
	}

	// Also cancel algo orders
	t.cancelAlgoOrders(symbol, "")

	if len(orders) > 0 {
		logger.Infof("  ✓ Canceled all pending orders for %s", symbol)
	}

	return nil
}

// CancelStopOrders cancels stop loss and take profit orders
func (t *OKXTrader) CancelStopOrders(symbol string) error {
	return t.cancelAlgoOrders(symbol, "")
}

// GetOrderStatus gets order status
func (t *OKXTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("/api/v5/trade/order?instId=%s&ordId=%s", instId, orderID)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
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
		return nil, fmt.Errorf("order not found")
	}

	order := orders[0]
	avgPrice, _ := strconv.ParseFloat(order.AvgPx, 64)
	fillSz, _ := strconv.ParseFloat(order.AccFillSz, 64) // This is in contracts
	fee, _ := strconv.ParseFloat(order.Fee, 64)
	cTime, _ := strconv.ParseInt(order.CTime, 10, 64)
	uTime, _ := strconv.ParseInt(order.UTime, 10, 64)

	// Convert contract count to base asset quantity
	// executedQty = contracts * ctVal
	executedQty := fillSz
	inst, err := t.getInstrument(symbol)
	if err == nil && inst.CtVal > 0 {
		executedQty = fillSz * inst.CtVal
		logger.Debugf("  📊 OKX order %s: fillSz(contracts)=%.4f, ctVal=%.6f, executedQty=%.6f", orderID, fillSz, inst.CtVal, executedQty)
	}

	// Status mapping
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
		"executedQty": executedQty,
		"side":        order.Side,
		"type":        order.OrdType,
		"time":        cTime,
		"updateTime":  uTime,
		"commission":  -fee, // OKX returns negative value
	}, nil
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *OKXTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	instId := t.convertSymbol(symbol)
	var result []types.OpenOrder

	// 1. Get pending limit orders
	path := fmt.Sprintf("%s?instId=%s&instType=SWAP", okxPendingOrdersPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		logger.Warnf("[OKX] Failed to get pending orders: %v", err)
	}
	if err == nil && data != nil {
		var orders []struct {
			OrdId   string `json:"ordId"`
			InstId  string `json:"instId"`
			Side    string `json:"side"`    // buy/sell
			PosSide string `json:"posSide"` // long/short/net
			OrdType string `json:"ordType"` // limit/market/post_only
			Px      string `json:"px"`      // price
			Sz      string `json:"sz"`      // size
			State   string `json:"state"`   // live/partially_filled
		}
		if err := json.Unmarshal(data, &orders); err == nil {
			for _, order := range orders {
				price, _ := strconv.ParseFloat(order.Px, 64)
				quantity, _ := strconv.ParseFloat(order.Sz, 64)

				// Convert OKX side to standard format
				side := strings.ToUpper(order.Side)
				positionSide := strings.ToUpper(order.PosSide)
				if positionSide == "NET" {
					positionSide = "BOTH"
				}

				result = append(result, types.OpenOrder{
					OrderID:      order.OrdId,
					Symbol:       symbol,
					Side:         side,
					PositionSide: positionSide,
					Type:         strings.ToUpper(order.OrdType),
					Price:        price,
					StopPrice:    0,
					Quantity:     quantity,
					Status:       "NEW",
				})
			}
		}
	}

	// 2. Get pending algo orders (stop-loss/take-profit)
	// OKX requires ordType parameter for algo orders API
	algoPath := fmt.Sprintf("%s?instId=%s&instType=SWAP&ordType=conditional", okxAlgoPendingPath, instId)
	algoData, err := t.doRequest("GET", algoPath, nil)
	if err != nil {
		logger.Warnf("[OKX] Failed to get algo orders: %v", err)
	}
	if err == nil && algoData != nil {
		var algoOrders []struct {
			AlgoId      string `json:"algoId"`
			InstId      string `json:"instId"`
			Side        string `json:"side"`
			PosSide     string `json:"posSide"`
			OrdType     string `json:"ordType"` // conditional/oco/trigger
			TriggerPx   string `json:"triggerPx"`
			SlTriggerPx string `json:"slTriggerPx"` // Stop loss trigger price
			TpTriggerPx string `json:"tpTriggerPx"` // Take profit trigger price
			Sz          string `json:"sz"`
			State       string `json:"state"`
		}
		if err := json.Unmarshal(algoData, &algoOrders); err == nil {
			for _, order := range algoOrders {
				quantity, _ := strconv.ParseFloat(order.Sz, 64)

				side := strings.ToUpper(order.Side)
				positionSide := strings.ToUpper(order.PosSide)
				if positionSide == "NET" {
					positionSide = "BOTH"
				}

				// Check for stop loss order (slTriggerPx is set)
				if order.SlTriggerPx != "" {
					slPrice, _ := strconv.ParseFloat(order.SlTriggerPx, 64)
					if slPrice > 0 {
						result = append(result, types.OpenOrder{
							OrderID:      order.AlgoId + "_sl",
							Symbol:       symbol,
							Side:         side,
							PositionSide: positionSide,
							Type:         "STOP_MARKET",
							Price:        0,
							StopPrice:    slPrice,
							Quantity:     quantity,
							Status:       "NEW",
						})
					}
				}

				// Check for take profit order (tpTriggerPx is set)
				if order.TpTriggerPx != "" {
					tpPrice, _ := strconv.ParseFloat(order.TpTriggerPx, 64)
					if tpPrice > 0 {
						result = append(result, types.OpenOrder{
							OrderID:      order.AlgoId + "_tp",
							Symbol:       symbol,
							Side:         side,
							PositionSide: positionSide,
							Type:         "TAKE_PROFIT_MARKET",
							Price:        0,
							StopPrice:    tpPrice,
							Quantity:     quantity,
							Status:       "NEW",
						})
					}
				}

				// Fallback for trigger orders (triggerPx is set)
				if order.TriggerPx != "" && order.SlTriggerPx == "" && order.TpTriggerPx == "" {
					triggerPrice, _ := strconv.ParseFloat(order.TriggerPx, 64)
					if triggerPrice > 0 {
						result = append(result, types.OpenOrder{
							OrderID:      order.AlgoId,
							Symbol:       symbol,
							Side:         side,
							PositionSide: positionSide,
							Type:         "STOP_MARKET",
							Price:        0,
							StopPrice:    triggerPrice,
							Quantity:     quantity,
							Status:       "NEW",
						})
					}
				}
			}
		}
	}

	logger.Infof("✓ OKX GetOpenOrders: found %d open orders for %s", len(result), symbol)
	return result, nil
}

// PlaceLimitOrder places a limit order for grid trading
// Implements GridTrader interface
func (t *OKXTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	instId := t.convertSymbol(req.Symbol)

	// Get instrument info
	inst, err := t.getInstrument(req.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get instrument info: %w", err)
	}

	// Set leverage if specified
	if req.Leverage > 0 {
		if err := t.SetLeverage(req.Symbol, req.Leverage); err != nil {
			logger.Warnf("[OKX] Failed to set leverage: %v", err)
		}
	}

	// Convert quantity to contract size
	sz := req.Quantity / inst.CtVal
	szStr := t.formatSize(sz, inst)

	// Determine side and position side
	side := "buy"
	posSide := "long"
	if req.Side == "SELL" {
		side = "sell"
		posSide = "short"
	}

	body := map[string]interface{}{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    side,
		"posSide": posSide,
		"ordType": "limit",
		"sz":      szStr,
		"px":      fmt.Sprintf("%.8f", req.Price),
		"clOrdId": genOkxClOrdID(),
		"tag":     okxTag,
	}

	// Add reduce only if specified
	if req.ReduceOnly {
		body["reduceOnly"] = true
	}

	logger.Infof("[OKX] PlaceLimitOrder: %s %s @ %.4f, sz=%s", instId, side, req.Price, szStr)

	data, err := t.doRequest("POST", okxOrderPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", err)
	}

	var orders []struct {
		OrdId   string `json:"ordId"`
		ClOrdId string `json:"clOrdId"`
		SCode   string `json:"sCode"`
		SMsg    string `json:"sMsg"`
	}

	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	if len(orders) == 0 {
		return nil, fmt.Errorf("empty order response")
	}

	if orders[0].SCode != "0" {
		return nil, fmt.Errorf("OKX order failed: %s", orders[0].SMsg)
	}

	logger.Infof("✓ [OKX] Limit order placed: %s %s @ %.4f, orderID=%s",
		instId, side, req.Price, orders[0].OrdId)

	return &types.LimitOrderResult{
		OrderID:      orders[0].OrdId,
		ClientID:     orders[0].ClOrdId,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Price:        req.Price,
		Quantity:     req.Quantity,
		Status:       "NEW",
	}, nil
}

// CancelOrder cancels a specific order by ID
// Implements GridTrader interface
func (t *OKXTrader) CancelOrder(symbol, orderID string) error {
	instId := t.convertSymbol(symbol)

	body := map[string]interface{}{
		"instId": instId,
		"ordId":  orderID,
	}

	_, err := t.doRequest("POST", "/api/v5/trade/cancel-order", body)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	logger.Infof("✓ [OKX] Order cancelled: %s %s", symbol, orderID)
	return nil
}

// GetOrderBook gets the order book for a symbol
// Implements GridTrader interface
func (t *OKXTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	instId := t.convertSymbol(symbol)
	path := fmt.Sprintf("/api/v5/market/books?instId=%s&sz=%d", instId, depth)

	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}

	var result []struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	if len(result) == 0 {
		return nil, nil, nil
	}

	// Parse bids
	for _, b := range result[0].Bids {
		if len(b) >= 2 {
			price, _ := strconv.ParseFloat(b[0], 64)
			qty, _ := strconv.ParseFloat(b[1], 64)
			bids = append(bids, []float64{price, qty})
		}
	}

	// Parse asks
	for _, a := range result[0].Asks {
		if len(a) >= 2 {
			price, _ := strconv.ParseFloat(a[0], 64)
			qty, _ := strconv.ParseFloat(a[1], 64)
			asks = append(asks, []float64{price, qty})
		}
	}

	return bids, asks, nil
}
