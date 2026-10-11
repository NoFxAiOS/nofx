package binance

import (
	"context"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/trader/types"
	"strconv"
	"strings"

	"github.com/adshao/go-binance/v2/futures"
)

// OpenLong opens a long position
func (t *FuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// R4 (2026-09-26 review): cancel ONLY this side's leftover protection —
	// CancelAllOrders wiped the opposite side's SL/TP (and any manual
	// positions' protection the hands-off rule will never restore).
	if err := t.CancelProtectiveOrdersForSide(symbol, string(futures.PositionSideTypeLong)); err != nil {
		logger.Infof("  ⚠ Failed to cancel old LONG protection (may not have any): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by the caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("position size too small, rounded to 0 (original: %.8f → formatted: %s). Suggest increasing position amount or selecting a lower-priced coin", quantity, quantityStr)
	}

	// Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	// Create market buy order (using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to open long position: %w", WrapAuthError("OpenLong "+symbol, err))
	}

	logger.Infof("✓ Opened long position successfully: %s quantity: %s", symbol, quantityStr)
	logger.Infof("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// OpenShort opens a short position
func (t *FuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// R4: only this side's leftover protection (see OpenLong).
	if err := t.CancelProtectiveOrdersForSide(symbol, string(futures.PositionSideTypeShort)); err != nil {
		logger.Infof("  ⚠ Failed to cancel old SHORT protection (may not have any): %v", err)
	}

	// Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// Note: Margin mode should be set by the caller (AutoTrader) before opening position via SetMarginMode

	// Format quantity to correct precision
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Check if formatted quantity is 0 (prevent rounding errors)
	quantityFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil || quantityFloat <= 0 {
		return nil, fmt.Errorf("position size too small, rounded to 0 (original: %.8f → formatted: %s). Suggest increasing position amount or selecting a lower-priced coin", quantity, quantityStr)
	}

	// Check minimum notional value (Binance requires at least 10 USDT)
	if err := t.CheckMinNotional(symbol, quantityFloat); err != nil {
		return nil, err
	}

	// Create market sell order (using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to open short position: %w", WrapAuthError("OpenShort "+symbol, err))
	}

	logger.Infof("✓ Opened short position successfully: %s quantity: %s", symbol, quantityStr)
	logger.Infof("  Order ID: %d", order.OrderID)

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CloseLong closes a long position
func (t *FuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
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
			return nil, fmt.Errorf("no long position found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market sell order (close long, using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeSell).
		PositionSide(futures.PositionSideTypeLong).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close long position: %w", WrapAuthError("CloseLong "+symbol, err))
	}

	logger.Infof("✓ Closed long position successfully: %s quantity: %s", symbol, quantityStr)

	// Full closes cancel the remaining SL/TP orders; PARTIAL closes (explicit
	// quantity, e.g. the TP-ladder 1/3 trim) must keep them — the remaining
	// position would otherwise ride unprotected.
	if quantity == 0 {
		// R4 (2026-09-26 review): full close cancels THIS side's protection
		// only — CancelAllOrders wiped the opposite side's SL/TP too.
		if err := t.CancelProtectiveOrdersForSide(symbol, string(futures.PositionSideTypeLong)); err != nil {
			logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
		}
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CloseShort closes a short position
func (t *FuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// If quantity is 0, get current position quantity
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = -pos["positionAmt"].(float64) // Short position quantity is negative, take absolute value
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("no short position found for %s", symbol)
		}
	}

	// Format quantity
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// Create market buy order (close short, using br ID)
	order, err := t.client.NewCreateOrderService().
		Symbol(symbol).
		Side(futures.SideTypeBuy).
		PositionSide(futures.PositionSideTypeShort).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr).
		NewClientOrderID(getBrOrderID()).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to close short position: %w", WrapAuthError("CloseShort "+symbol, err))
	}

	logger.Infof("✓ Closed short position successfully: %s quantity: %s", symbol, quantityStr)

	// Full closes cancel the remaining SL/TP orders; partial closes keep
	// them (same rule as CloseLong).
	if quantity == 0 {
		// R4 (2026-09-26 review): full close cancels THIS side's protection
		// only — CancelAllOrders wiped the opposite side's SL/TP too.
		if err := t.CancelProtectiveOrdersForSide(symbol, string(futures.PositionSideTypeShort)); err != nil {
			logger.Infof("  ⚠ Failed to cancel pending orders: %v", err)
		}
	}

	result := make(map[string]interface{})
	result["orderId"] = order.OrderID
	result["symbol"] = order.Symbol
	result["status"] = order.Status
	return result, nil
}

// CancelStopLossOrders cancels only stop-loss orders (doesn't affect take-profit orders)
// Now uses both legacy API and new Algo Order API
// CancelProtectiveOrdersForSide cancels BOTH protective legs (SL + TP) for
// ONE position side — R4 (2026-09-26 review): OpenLong/OpenShort and full
// closes used CancelAllOrders(symbol), which wiped the OPPOSITE side's
// protection (including manually opened positions' stops, which the
// hands-off watchdog will never restore). Only this side's STOP*/TAKE_PROFIT*
// algos and legacy stops are cancelled.
func (t *FuturesTrader) CancelProtectiveOrdersForSide(symbol, positionSide string) error {
	cancelErrors := []error{}

	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, order := range orders {
			orderType := strings.ToUpper(string(order.Type))
			isProtective := strings.Contains(orderType, "STOP") || strings.Contains(orderType, "TAKE_PROFIT")
			if !isProtective || string(order.PositionSide) != positionSide {
				continue
			}
			if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(order.OrderID).Do(context.Background()); err != nil {
				cancelErrors = append(cancelErrors, fmt.Errorf("order %d: %v", order.OrderID, err))
			}
		}
	}

	algoOrders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, algo := range algoOrders {
			algoType := strings.ToUpper(string(algo.OrderType))
			isProtective := strings.Contains(algoType, "STOP") || strings.Contains(algoType, "TAKE_PROFIT")
			if !isProtective || string(algo.PositionSide) != positionSide {
				continue
			}
			if _, err := t.client.NewCancelAlgoOrderService().AlgoID(algo.AlgoId).Do(context.Background()); err != nil {
				cancelErrors = append(cancelErrors, fmt.Errorf("algo %d: %v", algo.AlgoId, err))
			}
		}
	}

	if len(cancelErrors) > 0 {
		return fmt.Errorf("failed to cancel protective orders (%s %s): %v", symbol, positionSide, cancelErrors)
	}
	return nil
}

// CancelStopLossOrdersForSide cancels the symbol's stop-loss orders for ONE
// position side only — the hedge-mode-safe primitive (2026-09-25 P0: the
// symbol-wide cancel killed the OPPOSITE side's stop too, and the side-blind
// watchdog then judged the naked position protected). Empty positionSide
// cancels all sides (legacy behavior).
func (t *FuturesTrader) CancelStopLossOrdersForSide(symbol, positionSide string) error {
	canceledCount := 0
	var cancelErrors []error
	match := func(ps string) bool {
		return positionSide == "" || ps == "" || ps == positionSide ||
			ps == string(futures.PositionSideTypeBoth)
	}

	// 1. Legacy stop-loss orders
	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)
			if (orderType == "STOP_MARKET" || orderType == "STOP") && match(string(order.PositionSide)) {
				if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(order.OrderID).Do(context.Background()); err != nil {
					cancelErrors = append(cancelErrors, fmt.Errorf("order %d: %v", order.OrderID, err))
					continue
				}
				canceledCount++
			}
		}
	}

	// 2. Algo stop-loss orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, algoOrder := range algoOrders {
			if (algoOrder.OrderType == futures.AlgoOrderTypeStopMarket || algoOrder.OrderType == futures.AlgoOrderTypeStop) && match(string(algoOrder.PositionSide)) {
				if _, err := t.client.NewCancelAlgoOrderService().AlgoID(algoOrder.AlgoId).Do(context.Background()); err != nil {
					cancelErrors = append(cancelErrors, fmt.Errorf("algo %d: %v", algoOrder.AlgoId, err))
					continue
				}
				canceledCount++
			}
		}
	}

	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel stop-loss orders (%s): %v", positionSide, cancelErrors)
	}
	return nil
}

// CancelTakeProfitOrdersForSide cancels the symbol's take-profit orders for
// ONE position side only — the TP twin of CancelStopLossOrdersForSide (the
// -4130 stale-leg replacement needs the same side-scoped primitive; the
// symbol-wide CancelTakeProfitOrders would kill the opposite hedge leg's
// TP, the 09-28 review P3).
func (t *FuturesTrader) CancelTakeProfitOrdersForSide(symbol, positionSide string) error {
	canceledCount := 0
	var cancelErrors []error
	match := func(ps string) bool {
		return positionSide == "" || ps == "" || ps == positionSide ||
			ps == string(futures.PositionSideTypeBoth)
	}

	// 1. Legacy take-profit orders
	orders, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)
			if (orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT") && match(string(order.PositionSide)) {
				if _, err := t.client.NewCancelOrderService().Symbol(symbol).OrderID(order.OrderID).Do(context.Background()); err != nil {
					cancelErrors = append(cancelErrors, fmt.Errorf("order %d: %v", order.OrderID, err))
					continue
				}
				canceledCount++
			}
		}
	}

	// 2. Algo take-profit orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err == nil {
		for _, algoOrder := range algoOrders {
			if (algoOrder.OrderType == futures.AlgoOrderTypeTakeProfitMarket || algoOrder.OrderType == futures.AlgoOrderTypeTakeProfit) && match(string(algoOrder.PositionSide)) {
				if _, err := t.client.NewCancelAlgoOrderService().AlgoID(algoOrder.AlgoId).Do(context.Background()); err != nil {
					cancelErrors = append(cancelErrors, fmt.Errorf("algo %d: %v", algoOrder.AlgoId, err))
					continue
				}
				canceledCount++
			}
		}
	}

	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel take-profit orders (%s): %v", positionSide, cancelErrors)
	}
	return nil
}

func (t *FuturesTrader) CancelStopLossOrders(symbol string) error {
	canceledCount := 0
	var cancelErrors []error

	// 1. Cancel legacy stop-loss orders
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)

			// Only cancel stop-loss orders (don't cancel take-profit orders)
			// Use string comparison since OrderType constants were removed in v2.8.9
			if orderType == "STOP_MARKET" || orderType == "STOP" {
				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Order ID %d: %v", order.OrderID, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					logger.Infof("  ⚠ Failed to cancel legacy stop-loss order: %s", errMsg)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled legacy stop-loss order (Order ID: %d, Type: %s, Side: %s)", order.OrderID, orderType, order.PositionSide)
			}
		}
	}

	// 2. Cancel Algo stop-loss orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, algoOrder := range algoOrders {
			// Only cancel stop-loss orders
			if algoOrder.OrderType == futures.AlgoOrderTypeStopMarket || algoOrder.OrderType == futures.AlgoOrderTypeStop {
				_, err := t.client.NewCancelAlgoOrderService().
					AlgoID(algoOrder.AlgoId).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Algo ID %d: %v", algoOrder.AlgoId, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					logger.Infof("  ⚠ Failed to cancel Algo stop-loss order: %s", errMsg)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled Algo stop-loss order (Algo ID: %d, Type: %s)", algoOrder.AlgoId, algoOrder.OrderType)
			}
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		logger.Infof("  ℹ %s has no stop-loss orders to cancel", symbol)
	} else if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d stop-loss order(s) for %s", canceledCount, symbol)
	}

	// If all cancellations failed, return error
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel stop-loss orders: %v", cancelErrors)
	}

	return nil
}

// CancelTakeProfitOrders cancels only take-profit orders (doesn't affect stop-loss orders)
// Now uses both legacy API and new Algo Order API
func (t *FuturesTrader) CancelTakeProfitOrders(symbol string) error {
	canceledCount := 0
	var cancelErrors []error

	// 1. Cancel legacy take-profit orders
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)

			// Only cancel take-profit orders (don't cancel stop-loss orders)
			// Use string comparison since OrderType constants were removed in v2.8.9
			if orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT" {
				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Order ID %d: %v", order.OrderID, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					logger.Infof("  ⚠ Failed to cancel legacy take-profit order: %s", errMsg)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled legacy take-profit order (Order ID: %d, Type: %s, Side: %s)", order.OrderID, orderType, order.PositionSide)
			}
		}
	}

	// 2. Cancel Algo take-profit orders
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, algoOrder := range algoOrders {
			// Only cancel take-profit orders
			if algoOrder.OrderType == futures.AlgoOrderTypeTakeProfitMarket || algoOrder.OrderType == futures.AlgoOrderTypeTakeProfit {
				_, err := t.client.NewCancelAlgoOrderService().
					AlgoID(algoOrder.AlgoId).
					Do(context.Background())

				if err != nil {
					errMsg := fmt.Sprintf("Algo ID %d: %v", algoOrder.AlgoId, err)
					cancelErrors = append(cancelErrors, fmt.Errorf("%s", errMsg))
					logger.Infof("  ⚠ Failed to cancel Algo take-profit order: %s", errMsg)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled Algo take-profit order (Algo ID: %d, Type: %s)", algoOrder.AlgoId, algoOrder.OrderType)
			}
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		logger.Infof("  ℹ %s has no take-profit orders to cancel", symbol)
	} else if canceledCount > 0 {
		logger.Infof("  ✓ Canceled %d take-profit order(s) for %s", canceledCount, symbol)
	}

	// If all cancellations failed, return error
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("failed to cancel take-profit orders: %v", cancelErrors)
	}

	return nil
}

// CancelAllOrders cancels all pending orders for this symbol
// Now uses both legacy API and new Algo Order API
func (t *FuturesTrader) CancelAllOrders(symbol string) error {
	// 1. Cancel all legacy orders
	err := t.client.NewCancelAllOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		logger.Infof("  ⚠ Failed to cancel legacy orders: %v", err)
	} else {
		logger.Infof("  ✓ Canceled all legacy pending orders for %s", symbol)
	}

	// 2. Cancel all Algo orders
	err = t.client.NewCancelAllAlgoOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		// Ignore "no algo orders" error
		if !contains(err.Error(), "no algo") && !contains(err.Error(), "No algo") {
			logger.Infof("  ⚠ Failed to cancel Algo orders: %v", err)
		}
	} else {
		logger.Infof("  ✓ Canceled all Algo orders for %s", symbol)
	}

	return nil
}

// PlaceLimitOrder places a limit order for grid trading
// This implements the GridTrader interface for FuturesTrader
func (t *FuturesTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	// Quantize against the symbol's actual LOT_SIZE and validate before the
	// request reaches Binance. Previously QNT 0.03 became "0.0" and was sent
	// repeatedly, producing -4003 on every otherwise-valid signal.
	rules, err := t.getSymbolOrderRules(req.Symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get quantity rules: %w", err)
	}
	quantityStr, err := formatQuantityWithRules(req.Symbol, req.Quantity, rules)
	if err != nil {
		return nil, fmt.Errorf("failed to format quantity: %w", err)
	}

	// Format price to correct precision
	priceStr, err := t.FormatPrice(req.Symbol, req.Price)
	if err != nil {
		return nil, fmt.Errorf("failed to format price: %w", err)
	}
	quantityValue, _ := strconv.ParseFloat(quantityStr, 64)
	priceValue, _ := strconv.ParseFloat(priceStr, 64)
	if !req.ReduceOnly {
		if err := validateOrderNotional(req.Symbol, quantityValue, priceValue, rules.MinNotional); err != nil {
			return nil, err
		}
	}

	// Set leverage if specified
	if req.Leverage > 0 {
		if err := t.SetLeverage(req.Symbol, req.Leverage); err != nil {
			// review 2026-10-07 B1-3: match market entries; do not submit
			// an order with unconfirmed leverage.
			return nil, err
		}
	}

	// Determine side and position side
	var side futures.SideType
	var positionSide futures.PositionSideType

	if req.Side == "BUY" {
		side = futures.SideTypeBuy
		positionSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeSell
		positionSide = futures.PositionSideTypeShort
	}

	// Client order ID: every order keeps the broker referral prefix (revival
	// attribution, getBrOrderID). A caller-supplied req.ClientID tag is
	// carried after it ("lim…" for AI limit entries, "grid…" for grid levels)
	// so the order class is visible on-exchange — startup reconciliation
	// attributes resting limit entries by this tag and never touches orders
	// without it.
	clientOrderID := getBrOrderID()
	if req.ClientID != "" {
		clientOrderID = getBrOrderIDFor(req.ClientID)
	}
	orderService := t.client.NewCreateOrderService().
		Symbol(req.Symbol).
		Side(side).
		PositionSide(positionSide).
		Type(futures.OrderTypeLimit).
		TimeInForce(futures.TimeInForceTypeGTC).
		Quantity(quantityStr).
		Price(priceStr).
		NewClientOrderID(clientOrderID)
	if req.ReduceOnly {
		// Reduce-only resting limits (1R profit-lock trims) close in the
		// position's own direction and never open new exposure.
		orderService = orderService.ReduceOnly(true)
	}

	// Execute order
	order, err := orderService.Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to place limit order: %w", WrapAuthError("PlaceLimitOrder "+req.Symbol, err))
	}

	logger.Infof("✓ [Grid] Placed limit order: %s %s %s @ %s, qty=%s, orderID=%d",
		req.Symbol, req.Side, positionSide, priceStr, quantityStr, order.OrderID)

	return &types.LimitOrderResult{
		OrderID:  fmt.Sprintf("%d", order.OrderID),
		ClientID: order.ClientOrderID,

		Symbol:       order.Symbol,
		Side:         string(order.Side),
		PositionSide: string(order.PositionSide),
		Price:        req.Price,
		Quantity:     quantityValue,
		Status:       string(order.Status),
	}, nil
}

func validateOrderNotional(symbol string, quantity, price, minNotional float64) error {
	notional := quantity * price
	if notional < minNotional {
		return fmt.Errorf("MIN_NOTIONAL: %s order amount %.2f USDT is below minimum %.2f USDT (quantity=%.8f, price=%.8f)",
			symbol, notional, minNotional, quantity, price)
	}
	return nil
}

// CancelOrder cancels a specific order by ID
// This implements the GridTrader interface for FuturesTrader
func (t *FuturesTrader) CancelOrder(symbol, orderID string) error {
	// Parse order ID to int64
	orderIDInt, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order ID: %w", err)
	}

	_, err = t.client.NewCancelOrderService().
		Symbol(symbol).
		OrderID(orderIDInt).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	logger.Infof("✓ [Grid] Cancelled order: %s/%s", symbol, orderID)
	return nil
}

// GetOrderBook gets the order book for a symbol
// This implements the GridTrader interface for FuturesTrader
func (t *FuturesTrader) GetOrderBook(symbol string, depth int) (bids, asks [][]float64, err error) {
	book, err := t.client.NewDepthService().
		Symbol(symbol).
		Limit(depth).
		Do(context.Background())

	if err != nil {
		return nil, nil, fmt.Errorf("failed to get order book: %w", err)
	}

	// Convert bids
	bids = make([][]float64, len(book.Bids))
	for i, bid := range book.Bids {
		price, _ := strconv.ParseFloat(bid.Price, 64)
		qty, _ := strconv.ParseFloat(bid.Quantity, 64)
		bids[i] = []float64{price, qty}
	}

	// Convert asks
	asks = make([][]float64, len(book.Asks))
	for i, ask := range book.Asks {
		price, _ := strconv.ParseFloat(ask.Price, 64)
		qty, _ := strconv.ParseFloat(ask.Quantity, 64)
		asks[i] = []float64{price, qty}
	}

	return bids, asks, nil
}

// CancelStopOrders cancels take-profit/stop-loss orders for this symbol (used to adjust TP/SL positions)
// Now uses both legacy API and new Algo Order API (Binance migrated stop orders to Algo system)
func (t *FuturesTrader) CancelStopOrders(symbol string) error {
	canceledCount := 0

	// 1. Cancel legacy stop orders (for backward compatibility)
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err == nil {
		for _, order := range orders {
			orderType := string(order.Type)

			// Only cancel stop-loss and take-profit orders
			// Use string comparison since OrderType constants were removed in v2.8.9
			if orderType == "STOP_MARKET" ||
				orderType == "TAKE_PROFIT_MARKET" ||
				orderType == "STOP" ||
				orderType == "TAKE_PROFIT" {

				_, err := t.client.NewCancelOrderService().
					Symbol(symbol).
					OrderID(order.OrderID).
					Do(context.Background())

				if err != nil {
					logger.Infof("  ⚠ Failed to cancel legacy order %d: %v", order.OrderID, err)
					continue
				}

				canceledCount++
				logger.Infof("  ✓ Canceled legacy stop order for %s (Order ID: %d, Type: %s)",
					symbol, order.OrderID, orderType)
			}
		}
	}

	// 2. Cancel Algo orders (new API)
	err = t.client.NewCancelAllAlgoOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		// Ignore "no algo orders" error
		if !contains(err.Error(), "no algo") && !contains(err.Error(), "No algo") {
			logger.Infof("  ⚠ Failed to cancel Algo orders: %v", err)
		}
	} else {
		logger.Infof("  ✓ Canceled all Algo orders for %s", symbol)
		canceledCount++
	}

	if canceledCount == 0 {
		logger.Infof("  ℹ %s has no take-profit/stop-loss orders to cancel", symbol)
	}

	return nil
}

// GetOpenOrders gets all open/pending orders for a symbol
func (t *FuturesTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	var result []types.OpenOrder

	// 1. Get legacy open orders
	orders, err := t.client.NewListOpenOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("failed to get open orders: %w", err)
	}

	for _, order := range orders {
		price, _ := strconv.ParseFloat(order.Price, 64)
		stopPrice, _ := strconv.ParseFloat(order.StopPrice, 64)
		quantity, _ := strconv.ParseFloat(order.OrigQuantity, 64)

		result = append(result, types.OpenOrder{
			OrderID:       fmt.Sprintf("%d", order.OrderID),
			Symbol:        order.Symbol,
			Side:          string(order.Side),
			PositionSide:  string(order.PositionSide),
			Type:          string(order.Type),
			Price:         price,
			StopPrice:     stopPrice,
			Quantity:      quantity,
			Status:        string(order.Status),
			ClientID:      order.ClientOrderID,
			ClosePosition: order.ClosePosition, ReduceOnly: order.ReduceOnly,
		})
	}

	// 2. Get Algo orders (new API for stop-loss/take-profit)
	algoOrders, err := t.client.NewListOpenAlgoOrdersService().
		Symbol(symbol).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("algo protection query failed: %w", err)
	}
	{
		for _, algoOrder := range algoOrders {
			triggerPrice, _ := strconv.ParseFloat(algoOrder.TriggerPrice, 64)
			quantity, _ := strconv.ParseFloat(algoOrder.Quantity, 64)

			result = append(result, types.OpenOrder{
				OrderID:       fmt.Sprintf("%d", algoOrder.AlgoId),
				Symbol:        algoOrder.Symbol,
				Side:          string(algoOrder.Side),
				PositionSide:  string(algoOrder.PositionSide),
				Type:          string(algoOrder.OrderType),
				Price:         0, // Algo orders use stop price
				StopPrice:     triggerPrice,
				Quantity:      quantity,
				Status:        "NEW",
				ClosePosition: algoOrder.ClosePosition, ReduceOnly: algoOrder.ReduceOnly, Algo: true,
			})
		}
	}

	return result, nil
}

// SetStopLoss sets stop-loss order using new Algo Order API
// Binance has migrated stop orders to Algo Order system (error -4120 STOP_ORDER_SWITCH_ALGO)
func (t *FuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	// Use new Algo Order API
	//
	// Trigger price MUST be tick-rounded — a raw %.8f triggers -1111
	// "Precision is over the maximum defined for this asset" on coarse-tick
	// symbols (UNIUSDT tick 0.001 vs stop 8.86573 → the position ran
	// unprotected, 2026-09-20). Quantized to a real tick MULTIPLE (not just
	// decimal places — a 0.025 tick breaks the decimals-only shortcut),
	// falling back to decimal formatting when the tick size is unavailable.
	priceStr, ferr := t.formatAlgoTriggerPrice(symbol, stopPrice)
	if ferr != nil {
		return fmt.Errorf("failed to set stop-loss: %w", ferr)
	}
	_, err := t.client.NewCreateAlgoOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.AlgoOrderTypeStopMarket).
		TriggerPrice(priceStr).
		WorkingType(futures.WorkingTypeContractPrice).
		ClosePosition(true).
		ClientAlgoId(getBrOrderID()).
		Do(context.Background())

	if err != nil && isStaleProtectiveOrder(err) {
		// -4130: an open GTE+closePosition algo in this direction already
		// exists (MUBARAKUSDT 09-29: the watchdog's naked-position fallback
		// fired before the pending-fill path placed the plan stop). The old
		// leg carries a STALE trigger — cancel this side's SL legs once and
		// re-place at the new price; if the cancel itself fails, surface the
		// ORIGINAL error (the position stays protected at the old level).
		previous, readErr := t.GetOpenOrders(symbol)
		if readErr != nil {
			return fmt.Errorf("cannot snapshot original stop: %w", readErr)
		}

		originals := []types.OpenOrder{}
		for _, old := range previous {
			if (old.Type == "STOP_MARKET" || old.Type == "STOP") && strings.EqualFold(old.PositionSide, string(posSide)) && old.OrderID != "" && old.StopPrice > 0 {
				originals = append(originals, old)
			}
		}
		if len(originals) == 0 {
			return fmt.Errorf("stale stop replacement: original protection unknown: %w", err)
		}
		for _, old := range originals {
			if cerr := t.CancelProtectiveOrder(symbol, old); cerr != nil {
				return fmt.Errorf("stale stop %s cancel failed: %w", old.OrderID, cerr)
			}
		}
		if _, err2 := t.client.NewCreateAlgoOrderService().
			Symbol(symbol).
			Side(side).
			PositionSide(posSide).
			Type(futures.AlgoOrderTypeStopMarket).
			TriggerPrice(priceStr).
			WorkingType(futures.WorkingTypeContractPrice).
			ClosePosition(true).
			ClientAlgoId(getBrOrderID()).
			Do(context.Background()); err2 != nil {
			for _, old := range originals {
				_, restoreErr := t.client.NewCreateAlgoOrderService().Symbol(symbol).Side(side).PositionSide(posSide).Type(futures.AlgoOrderTypeStopMarket).TriggerPrice(strconv.FormatFloat(old.StopPrice, 'f', -1, 64)).WorkingType(futures.WorkingTypeContractPrice).ClosePosition(true).ClientAlgoId(getBrOrderID()).Do(context.Background())
				if restoreErr != nil {
					return fmt.Errorf("replacement rejected (%v), original stop restore failed: %w", err2, restoreErr)
				}
				restored, verifyErr := t.GetOpenOrders(symbol)
				confirmed := false
				for _, o := range restored {
					if o.ClosePosition && (o.Type == "STOP_MARKET" || o.Type == "STOP") && strings.EqualFold(o.PositionSide, string(posSide)) && math.Abs(o.StopPrice-old.StopPrice)/old.StopPrice < 0.001 {
						confirmed = true
					}
				}
				if verifyErr != nil || !confirmed {
					return fmt.Errorf("replacement rejected (%v), original stop restoration unconfirmed: %v", err2, verifyErr)
				}
				break
			}
			return fmt.Errorf("failed to set stop-loss: stale algo leg cancelled but re-place failed: %w", err2)
		}
		logger.Infof("  ♻️ Stop-loss replaced: stale -4130 leg cancelled, new trigger %.4f (Algo Order)", stopPrice)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to set stop-loss: %w", err)
	}

	logger.Infof("  Stop-loss price set (Algo Order): %.4f", stopPrice)
	return nil
}

// SetTakeProfit sets take-profit order using new Algo Order API
// Binance has migrated stop orders to Algo Order system (error -4120 STOP_ORDER_SWITCH_ALGO)
func (t *FuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	var side futures.SideType
	var posSide futures.PositionSideType

	if positionSide == "LONG" {
		side = futures.SideTypeSell
		posSide = futures.PositionSideTypeLong
	} else {
		side = futures.SideTypeBuy
		posSide = futures.PositionSideTypeShort
	}

	// Use new Algo Order API — tick-rounded trigger, same -1111 rationale
	// as SetStopLoss above.
	//
	// quantity > 0 = PARTIAL TP (tp_close_fraction < 1): only that slice
	// closes at the trigger, the remainder stays on as a trend-runner under
	// the trailing stop (09-21 user experiment — a resting full-size TP
	// structurally sold every spike top: BTCUSDT TP filled 83000, price
	// printed 84275 in the same minute). quantity <= 0 keeps the legacy
	// closePosition mode. If the fraction underflows the symbol's lot step
	// the order falls back to full-close rather than placing a rejected
	// dust order.
	priceStr, ferr := t.formatAlgoTriggerPrice(symbol, takeProfitPrice)
	if ferr != nil {
		return fmt.Errorf("failed to set take-profit: %w", ferr)
	}
	algo := t.client.NewCreateAlgoOrderService().
		Symbol(symbol).
		Side(side).
		PositionSide(posSide).
		Type(futures.AlgoOrderTypeTakeProfitMarket).
		TriggerPrice(priceStr).
		WorkingType(futures.WorkingTypeContractPrice)
	if quantity > 0 {
		qtyStr, qerr := t.FormatQuantity(symbol, quantity)
		qty, _ := strconv.ParseFloat(qtyStr, 64)
		if qerr == nil && qty > 0 {
			// NO ReduceOnly here: with positionSide LONG/SHORT Binance
			// rejects the redundant param (-1106 "Parameter 'reduceonly'
			// sent when not required") and the whole TP leg fails to place
			// (AKEUSDT/PHAUSDT 09-22 — positions ran with SL only). The
			// quantity alone bounds the close; positionSide implies the
			// reduce direction.
			algo = algo.Quantity(qtyStr)
		} else {
			// Fraction underflowed the lot step — full close instead of a
			// rejected order (which would leave the TP leg missing).
			algo = algo.ClosePosition(true)
		}
	} else {
		algo = algo.ClosePosition(true)
	}
	_, err := algo.ClientAlgoId(getBrOrderID()).
		Do(context.Background())

	// -4130 on a TP: an open TAKE_PROFIT* leg exists in this direction. For
	// TPs the -4130 block is unconditional (any GTE+closePosition TP), so
	// retrying without cancelling can never succeed — mirror the SL leg's
	// cancel-and-replace. review 2026-10-11: reuse the first request's shape
	// so a quantity-limited TP cannot silently become a full-position close.
	// Only the client ID changes; quantity/closePosition and side stay intact.
	if err != nil && isStaleProtectiveOrder(err) {
		if cerr := t.CancelTakeProfitOrdersForSide(symbol, string(posSide)); cerr != nil {
			return fmt.Errorf("failed to set take-profit: stale algo leg replacement failed at cancel: %w", cerr)
		}
		if _, err2 := algo.ClientAlgoId(getBrOrderID()).
			Do(context.Background()); err2 != nil {
			return fmt.Errorf("failed to set take-profit: stale algo leg cancelled but re-place failed: %w", err2)
		}
		logger.Infof("  ♻️ Take-profit replaced: stale -4130 leg cancelled, new trigger %.4f (Algo Order, original close shape)", takeProfitPrice)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to set take-profit: %w", err)
	}

	logger.Infof("  Take-profit price set (Algo Order): %.4f", takeProfitPrice)
	return nil
}

// GetOrderStatus gets order status
func (t *FuturesTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	// Convert orderID to int64
	orderIDInt, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %s", orderID)
	}

	order, err := t.client.NewGetOrderService().
		Symbol(symbol).
		OrderID(orderIDInt).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	// Parse execution price
	avgPrice, _ := strconv.ParseFloat(order.AvgPrice, 64)
	executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)

	result := map[string]interface{}{
		"orderId":     order.OrderID,
		"symbol":      order.Symbol,
		"status":      string(order.Status),
		"avgPrice":    avgPrice,
		"executedQty": executedQty,
		"side":        string(order.Side),
		"type":        string(order.Type),
		"time":        order.Time,
		"updateTime":  order.UpdateTime,
	}

	// Binance futures commission fee needs to be obtained through GetUserTrades, not retrieved here for now
	// Can be obtained later through WebSocket or separate query
	result["commission"] = 0.0

	return result, nil
}

// DebugListOpenAlgoOrders returns one summary line per open algo order on
// the symbol (maintenance-CLI helper, cmd/listalgos).
func (t *FuturesTrader) DebugListOpenAlgoOrders(symbol string) []string {
	orders, err := t.client.NewListOpenAlgoOrdersService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return []string{fmt.Sprintf("list failed: %v", err)}
	}
	out := make([]string, 0, len(orders))
	for _, a := range orders {
		out = append(out, fmt.Sprintf("id=%v type=%s side=%s posSide=%s trigger=%s qty=%s status=%s",
			a.AlgoId, a.OrderType, a.Side, a.PositionSide, a.TriggerPrice, a.Quantity, a.AlgoStatus))
	}
	return out
}

func (t *FuturesTrader) CancelProtectiveOrder(symbol string, o types.OpenOrder) error {
	id, err := strconv.ParseInt(o.OrderID, 10, 64)
	if err != nil {
		return err
	}
	if o.Algo {
		_, err = t.client.NewCancelAlgoOrderService().AlgoID(id).Do(context.Background())
		return err
	}
	return t.CancelOrder(symbol, o.OrderID)
}
