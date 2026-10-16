package binance_bstock

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	binance "github.com/adshao/go-binance/v2"
	"github.com/shopspring/decimal"
	"nofx/trader/types"
)

func protectionID(role string) string {
	return clientPrefix + role + "_" + strings.TrimPrefix(clientID(), clientPrefix)
}
func isProtection(o *binance.Order) bool {
	if o.Side != binance.SideTypeSell || !strings.HasPrefix(o.ClientOrderID, clientPrefix) {
		return false
	}
	return o.Type == binance.OrderTypeStopLossLimit || (o.Type == binance.OrderTypeLimitMaker && (o.OrderListId >= 0 || strings.HasPrefix(o.ClientOrderID, clientPrefix+"tp_")))
}

// Open orders carry orderListId on both OCO legs, so no local protection state
// or second order-list query is required to reconstruct the live set.
func reconstruct(symbol string, orders []*binance.Order) (*types.ProtectionOrders, error) {
	var p *types.ProtectionOrders
	list := int64(-1)
	for _, o := range orders {
		if !isProtection(o) {
			continue
		}
		if p == nil {
			p = &types.ProtectionOrders{Symbol: symbol}
			list = o.OrderListId
		}
		if o.OrderListId != list {
			return nil, fmt.Errorf("multiple protection sets for %s; reconcile before replacing", symbol)
		}
		q := number(o.OrigQuantity) - number(o.ExecutedQuantity)
		if p.Quantity == 0 || q < p.Quantity {
			p.Quantity = q
		}
		if o.OrderListId >= 0 {
			p.OrderListID = id(o.OrderListId)
		}
		switch o.Type {
		case binance.OrderTypeStopLossLimit:
			if p.StopOrderID != "" {
				return nil, fmt.Errorf("multiple stop orders for %s", symbol)
			}
			p.StopOrderID = id(o.OrderID)
			p.StopClientID = o.ClientOrderID
			p.StopPrice = number(o.StopPrice)
			p.StopLimitPrice = number(o.Price)
		case binance.OrderTypeLimitMaker:
			if p.TakeProfitID != "" {
				return nil, fmt.Errorf("multiple take-profit orders for %s", symbol)
			}
			p.TakeProfitID = id(o.OrderID)
			p.TPClientID = o.ClientOrderID
			p.TakeProfit = number(o.Price)
		}
	}
	return p, nil
}

func (t *BStockTrader) GetProtection(symbol string) (*types.ProtectionOrders, error) {
	o, err := t.openOrders(symbol)
	if err != nil {
		return nil, err
	}
	return reconstruct(symbol, o)
}

func (t *BStockTrader) cancelProtection(symbol string) error {
	orders, err := t.openOrders(symbol)
	if err != nil {
		return err
	}
	lists := make(map[int64]bool)
	for _, o := range orders {
		if !isProtection(o) {
			continue
		}
		if o.OrderListId >= 0 {
			if lists[o.OrderListId] {
				continue
			}
			_, err = t.client.NewCancelOCOService().Symbol(symbol).OrderListID(o.OrderListId).NewClientOrderID(clientID()).Do(context.Background())
			lists[o.OrderListId] = true
		} else {
			_, err = t.client.NewCancelOrderService().Symbol(symbol).OrderID(o.OrderID).NewClientOrderID(clientID()).Do(context.Background())
		}
		t.invalidate()
		if err != nil {
			return wrap("cancel protection", err)
		}
	}
	return nil
}

func (t *BStockTrader) CancelProtection(symbol string) error {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	// Also invalidate when openOrders is empty: the set may just have filled
	// externally, without any local place/cancel call touching the caches.
	defer t.invalidate()
	return t.cancelProtection(symbol)
}
func (t *BStockTrader) CancelStopOrders(symbol string) error { return t.CancelProtection(symbol) }

func (t *BStockTrader) SetProtection(symbol string, qty, stop, stopLimit, tp float64) (*types.ProtectionOrders, error) {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	return t.setProtection(symbol, qty, stop, stopLimit, tp)
}

func (t *BStockTrader) setProtection(symbol string, qty, stop, stopLimit, tp float64) (*types.ProtectionOrders, error) {
	s, err := t.rule(symbol)
	if err != nil {
		return nil, err
	}
	qty, err = quantity(s, qty, false)
	if err != nil {
		return nil, err
	}
	stop, err = price(s, stop)
	if err != nil {
		return nil, err
	}
	stopLimit, err = price(s, stopLimit)
	if err != nil {
		return nil, err
	}
	if !finite(tp) || tp < 0 {
		return nil, fmt.Errorf("takeProfit must be nonnegative and finite")
	}
	if tp > 0 {
		if !s.OcoAllowed {
			return nil, fmt.Errorf("OCO not allowed for %s", symbol)
		}
		tp, err = price(s, tp)
		if err != nil {
			return nil, err
		}
	}
	current, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	if stopLimit > stop || stop >= current || (tp > 0 && tp <= current) {
		return nil, fmt.Errorf("invalid protection: require stopLimitPrice <= stopPrice < current price < takeProfit (if set)")
	}
	if err = notional(s, qty*stopLimit, false); err != nil {
		return nil, err
	}
	if tp > 0 {
		if err = notional(s, qty*tp, false); err != nil {
			return nil, err
		}
	}
	a, err := t.account()
	if err != nil {
		return nil, err
	}
	_, total := holding(a, s.BaseAsset)
	if qty > total+1e-10 {
		return nil, fmt.Errorf("protection quantity exceeds held balance")
	}
	// Validate before cancel; an invalid adjustment must leave the old stop live.
	if err = t.cancelProtection(symbol); err != nil {
		return nil, err
	}
	a, err = t.account()
	if err != nil {
		return nil, fmt.Errorf("protection canceled; balance refresh failed: %w", err)
	}
	free, _ := holding(a, s.BaseAsset)
	if qty > free+1e-10 {
		return nil, fmt.Errorf("protection canceled; quantity exceeds free balance (other orders may lock inventory)")
	}
	p := &types.ProtectionOrders{Symbol: symbol, Quantity: qty, StopPrice: stop, StopLimitPrice: stopLimit, TakeProfit: tp}
	if tp == 0 {
		o, e := t.place(t.client.NewCreateOrderService().Symbol(symbol).Side(binance.SideTypeSell).Type(binance.OrderTypeStopLossLimit).TimeInForce(binance.TimeInForceTypeGTC).Quantity(value(qty)).StopPrice(value(stop)).Price(value(stopLimit)))
		if e != nil {
			return nil, fmt.Errorf("protection canceled; replacement stop failed: %w", e)
		}
		p.StopOrderID = o.OrderID
		p.StopClientID = o.ClientOrderID
		return p, nil
	}
	// v2.8.9 CreateOCOService uses deprecated /api/v3/order/oco. The new
	// endpoint has explicit above/below leg types and client IDs.
	below, above := protectionID("sl"), protectionID("tp")
	p.ListClientID, p.StopClientID, p.TPClientID = clientID(), below, above
	params := url.Values{"symbol": {symbol}, "side": {"SELL"}, "quantity": {value(qty)}, "listClientOrderId": {p.ListClientID}, "aboveType": {"LIMIT_MAKER"}, "abovePrice": {value(tp)}, "aboveClientOrderId": {above}, "belowType": {"STOP_LOSS_LIMIT"}, "belowStopPrice": {value(stop)}, "belowPrice": {value(stopLimit)}, "belowTimeInForce": {"GTC"}, "belowClientOrderId": {below}, "newOrderRespType": {"RESULT"}}
	var result struct {
		OrderListID *int64           `json:"orderListId"`
		Orders      []*binance.Order `json:"orders"`
		Reports     []*binance.Order `json:"orderReports"`
	}
	err = t.signed(http.MethodPost, "/api/v3/orderList/oco", params, &result)
	t.invalidate()
	if err != nil {
		return nil, fmt.Errorf("protection canceled; replacement OCO failed: %w", err)
	}
	for _, o := range append(result.Orders, result.Reports...) {
		if o.ClientOrderID == below && o.OrderID > 0 {
			p.StopOrderID = id(o.OrderID)
		}
		if o.ClientOrderID == above && o.OrderID > 0 {
			p.TakeProfitID = id(o.OrderID)
		}
	}
	if result.OrderListID != nil && *result.OrderListID >= 0 {
		p.OrderListID = id(*result.OrderListID)
	}
	if result.OrderListID == nil || *result.OrderListID < 0 || p.StopOrderID == "" || p.TakeProfitID == "" {
		return p, fmt.Errorf("OCO accepted but response lacks list/leg IDs; reconcile via GetProtection before retrying")
	}
	return p, nil
}

func longSide(side string) error {
	if !strings.EqualFold(side, "LONG") {
		return unsupported("short protection")
	}
	return nil
}
func (t *BStockTrader) SetStopLoss(symbol, side string, qty, stop float64) error {
	if err := longSide(side); err != nil {
		return err
	}
	t.execMu.Lock()
	defer t.execMu.Unlock()
	p, err := t.GetProtection(symbol)
	if err != nil {
		return err
	}
	tp := 0.0
	if p != nil {
		tp = p.TakeProfit
	}
	if !finite(stop) || stop <= 0 {
		return fmt.Errorf("PRICE_FILTER: invalid stop price")
	}
	// Multiply in decimal before tick rounding: e.g. 114*0.995 in float64
	// becomes 113.42999999999999 and would incorrectly round down to 113.42.
	stopLimit, _ := decimal.NewFromFloat(stop).Mul(decimal.RequireFromString("0.995")).Float64()
	_, err = t.setProtection(symbol, qty, stop, stopLimit, tp)
	return err
}

// A take-profit without a stop is a tagged lone LIMIT_MAKER. It can later be
// combined with SetStopLoss into an OCO without inventing a stop trigger.
func (t *BStockTrader) loneTP(symbol string, qty, tp float64) error {
	s, err := t.rule(symbol)
	if err != nil {
		return err
	}
	qty, err = quantity(s, qty, false)
	if err != nil {
		return err
	}
	tp, err = price(s, tp)
	if err != nil {
		return err
	}
	current, err := t.GetMarketPrice(symbol)
	if err != nil {
		return err
	}
	if tp <= current {
		return fmt.Errorf("take-profit must exceed current price")
	}
	if err = notional(s, qty*tp, false); err != nil {
		return err
	}
	a, err := t.account()
	if err != nil {
		return err
	}
	_, total := holding(a, s.BaseAsset)
	if qty > total+1e-10 {
		return fmt.Errorf("take-profit quantity exceeds held balance")
	}
	if err = t.cancelProtection(symbol); err != nil {
		return err
	}
	a, err = t.account()
	if err != nil {
		return fmt.Errorf("protection canceled; balance refresh failed: %w", err)
	}
	free, _ := holding(a, s.BaseAsset)
	if qty > free+1e-10 {
		return fmt.Errorf("protection canceled; take-profit quantity exceeds free balance")
	}
	// place normally generates a generic ID; this role is needed to recognize
	// the lone limit as protection. Use the SDK directly with the role tag.
	o, err := t.client.NewCreateOrderService().Symbol(symbol).Side(binance.SideTypeSell).Type(binance.OrderTypeLimitMaker).Quantity(value(qty)).Price(value(tp)).NewClientOrderID(protectionID("tp")).Do(context.Background())
	t.invalidate()
	if err != nil {
		return fmt.Errorf("protection canceled; replacement take-profit failed: %w", wrap("place take-profit", err))
	}
	if o.OrderID <= 0 {
		return fmt.Errorf("take-profit response lacks order ID; reconcile before retrying")
	}
	return nil
}

func (t *BStockTrader) SetTakeProfit(symbol, side string, qty, tp float64) error {
	if err := longSide(side); err != nil {
		return err
	}
	t.execMu.Lock()
	defer t.execMu.Unlock()
	p, err := t.GetProtection(symbol)
	if err != nil {
		return err
	}
	if p == nil || p.StopOrderID == "" {
		return t.loneTP(symbol, qty, tp)
	}
	_, err = t.setProtection(symbol, qty, p.StopPrice, p.StopLimitPrice, tp)
	return err
}

func (t *BStockTrader) CancelTakeProfitOrders(symbol string) error {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	p, err := t.GetProtection(symbol)
	if err != nil {
		return err
	}
	if p == nil || p.TakeProfitID == "" {
		return nil
	}
	if p.StopOrderID == "" {
		return t.cancelProtection(symbol)
	}
	_, err = t.setProtection(symbol, p.Quantity, p.StopPrice, p.StopLimitPrice, 0)
	return err
}

func (t *BStockTrader) CancelStopLossOrders(symbol string) error {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	p, err := t.GetProtection(symbol)
	if err != nil {
		return err
	}
	if p == nil || p.StopOrderID == "" {
		return nil
	}
	if p.TakeProfitID == "" {
		return t.cancelProtection(symbol)
	}
	return t.loneTP(symbol, p.Quantity, p.TakeProfit)
}
