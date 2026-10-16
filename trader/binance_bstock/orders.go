package binance_bstock

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	binance "github.com/adshao/go-binance/v2"
	"nofx/trader/types"
)

func (t *BStockTrader) place(svc *binance.CreateOrderService) (*types.SpotOrderResult, error) {
	o, err := svc.NewClientOrderID(clientID()).NewOrderRespType(binance.NewOrderRespTypeFULL).Do(context.Background())
	t.invalidate() // Even a timeout may have executed at the exchange.
	if err != nil {
		return nil, wrap("place spot order", err)
	}
	result := &types.SpotOrderResult{Symbol: o.Symbol, OrderID: id(o.OrderID), ClientOrderID: o.ClientOrderID, Side: string(o.Side), Type: string(o.Type), Status: string(o.Status), Price: number(o.Price), OrigQty: number(o.OrigQuantity), ExecutedQty: number(o.ExecutedQuantity), QuoteQty: number(o.CummulativeQuoteQuantity)}
	if result.ExecutedQty > 0 {
		result.AvgPrice = result.QuoteQty / result.ExecutedQty
	}
	if o.OrderID <= 0 {
		return result, fmt.Errorf("spot order accepted but response lacks order ID; reconcile before retrying")
	}
	// Preserve the native fee asset when all fills use the same asset; mixed
	// assets are converted to USDT to avoid adding unlike currency quantities.
	for _, f := range o.Fills {
		if result.CommissionAsset == "" {
			result.CommissionAsset = f.CommissionAsset
		}
		if result.CommissionAsset != f.CommissionAsset {
			result.CommissionAsset = "USDT"
			break
		}
	}
	for _, f := range o.Fills {
		fee := number(f.Commission)
		if result.CommissionAsset != f.CommissionAsset {
			fee, err = t.feeUSDT(o.Symbol, f.CommissionAsset, fee, number(f.Price), o.TransactTime)
			if err != nil {
				return result, fmt.Errorf("order %s placed, commission conversion failed: %w", result.OrderID, err)
			}
		}
		result.Commission += fee
	}
	return result, nil
}

func (t *BStockTrader) BuyMarketNotional(symbol string, quoteUSDT float64) (*types.SpotOrderResult, error) {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	s, err := t.rule(symbol)
	if err != nil {
		return nil, err
	}
	if !s.QuoteOrderQtyMarketAllowed {
		return nil, fmt.Errorf("quoteOrderQty market buying not allowed for %s", symbol)
	}
	if err = notional(s, quoteUSDT, true); err != nil {
		return nil, err
	}
	return t.place(t.client.NewCreateOrderService().Symbol(symbol).Side(binance.SideTypeBuy).Type(binance.OrderTypeMarket).QuoteOrderQty(value(quoteUSDT)))
}

func (t *BStockTrader) BuyLimit(symbol string, qty, p float64) (*types.SpotOrderResult, error) {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	s, err := t.rule(symbol)
	if err != nil {
		return nil, err
	}
	qty, err = quantity(s, qty, false)
	if err != nil {
		return nil, err
	}
	p, err = price(s, p)
	if err != nil {
		return nil, err
	}
	if err = notional(s, qty*p, false); err != nil {
		return nil, err
	}
	return t.place(t.client.NewCreateOrderService().Symbol(symbol).Side(binance.SideTypeBuy).Type(binance.OrderTypeLimit).TimeInForce(binance.TimeInForceTypeGTC).Quantity(value(qty)).Price(value(p)))
}

func (t *BStockTrader) OpenLong(symbol string, qty float64, leverage int) (map[string]interface{}, error) {
	if leverage > 1 {
		return nil, unsupported("OpenLong leverage > 1")
	}
	t.execMu.Lock()
	defer t.execMu.Unlock()
	s, err := t.rule(symbol)
	if err != nil {
		return nil, err
	}
	qty, err = quantity(s, qty, true)
	if err != nil {
		return nil, err
	}
	p, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	if err = notional(s, qty*p, true); err != nil {
		return nil, err
	}
	o, err := t.place(t.client.NewCreateOrderService().Symbol(symbol).Side(binance.SideTypeBuy).Type(binance.OrderTypeMarket).Quantity(value(qty)))
	if err != nil {
		return nil, err
	}
	return orderMap(o), nil
}

func (t *BStockTrader) SellMarket(symbol string, qty float64) (*types.SpotOrderResult, error) {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	s, err := t.rule(symbol)
	if err != nil {
		return nil, err
	}
	if !finite(qty) || qty < 0 {
		return nil, fmt.Errorf("sell quantity must be nonnegative and finite")
	}
	if qty == 0 {
		if err = t.cancelProtection(symbol); err != nil {
			return nil, err
		}
	}
	// Always query after cancellation: locked OCO inventory becomes free.
	a, err := t.account()
	if err != nil {
		return nil, err
	}
	free, _ := holding(a, s.BaseAsset)
	if qty == 0 {
		qty = free
	}
	qty, err = quantity(s, qty, true)
	if err != nil {
		return nil, err
	}
	if qty > free+1e-10 {
		return nil, fmt.Errorf("sell quantity %s exceeds free balance %s", value(qty), value(free))
	}
	p, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	if err = notional(s, qty*p, true); err != nil {
		return nil, err
	}
	return t.place(t.client.NewCreateOrderService().Symbol(symbol).Side(binance.SideTypeSell).Type(binance.OrderTypeMarket).Quantity(value(qty)))
}

func (t *BStockTrader) openOrders(symbol string) ([]*binance.Order, error) {
	if _, err := t.rule(symbol); err != nil {
		return nil, err
	}
	o, err := t.client.NewListOpenOrdersService().Symbol(symbol).Do(context.Background())
	return o, wrap("open orders", err)
}

func (t *BStockTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	orders, err := t.openOrders(symbol)
	if err != nil {
		return nil, err
	}
	out := make([]types.OpenOrder, 0, len(orders))
	for _, o := range orders {
		out = append(out, types.OpenOrder{OrderID: id(o.OrderID), Symbol: o.Symbol, Side: string(o.Side), PositionSide: "LONG", Type: string(o.Type), Price: number(o.Price), StopPrice: number(o.StopPrice), Quantity: number(o.OrigQuantity) - number(o.ExecutedQuantity), Status: string(o.Status), ClientID: o.ClientOrderID})
	}
	return out, nil
}

func (t *BStockTrader) GetOrderStatus(symbol, orderID string) (map[string]interface{}, error) {
	if _, err := t.rule(symbol); err != nil {
		return nil, err
	}
	orderNum, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order ID: %w", err)
	}
	o, err := t.client.NewGetOrderService().Symbol(symbol).OrderID(orderNum).Do(context.Background())
	if err != nil {
		return nil, wrap("order status", err)
	}
	// Exchange-side executions do not pass through place/cancel. The next
	// account read must not reuse inventory cached before this status read.
	t.invalidate()
	trades, err := t.trades(symbol)
	if err != nil {
		return nil, err
	}
	commission := 0.0
	for _, f := range trades {
		if f.OrderID == orderNum {
			fee, e := t.feeUSDT(symbol, f.CommissionAsset, number(f.Commission), number(f.Price), f.Time)
			if e != nil {
				return nil, e
			}
			commission += fee
		}
	}
	executed, avg := number(o.ExecutedQuantity), 0.0
	if executed > 0 {
		avg = number(o.CummulativeQuoteQuantity) / executed
	}
	return map[string]interface{}{"orderId": o.OrderID, "clientOrderId": o.ClientOrderID, "orderListId": o.OrderListId, "symbol": o.Symbol, "status": string(o.Status), "avgPrice": avg, "executedQty": executed, "cummulativeQuoteQty": number(o.CummulativeQuoteQuantity), "commission": commission, "commissionAsset": "USDT", "side": string(o.Side), "type": string(o.Type), "time": o.Time, "updateTime": o.UpdateTime}, nil
}

// CancelOrder cancels one program-owned open order. Orders without the nxbs_
// prefix (the user's manual orders) are refused; already-terminal orders are
// a no-op.
func (t *BStockTrader) CancelOrder(symbol, orderID string) error {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	if _, err := t.rule(symbol); err != nil {
		return err
	}
	orderNum, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order ID: %w", err)
	}
	o, err := t.client.NewGetOrderService().Symbol(symbol).OrderID(orderNum).Do(context.Background())
	if err != nil {
		return wrap("order status", err)
	}
	if !strings.HasPrefix(o.ClientOrderID, clientPrefix) {
		return fmt.Errorf("order %s on %s is not a program order (client id %q); refusing to cancel", orderID, symbol, o.ClientOrderID)
	}
	switch o.Status {
	case binance.OrderStatusTypeFilled, binance.OrderStatusTypeCanceled, binance.OrderStatusTypeExpired, binance.OrderStatusTypeRejected:
		return nil
	}
	_, err = t.client.NewCancelOrderService().Symbol(symbol).OrderID(orderNum).NewClientOrderID(clientID()).Do(context.Background())
	t.invalidate()
	return wrap("cancel order", err)
}

// CancelAllOrders cancels the symbol's PROGRAM-owned open orders (nxbs_
// client IDs) only — the user also trades manually on this account, and a
// symbol-wide cancel would wipe their resting orders (us_stock design
// 2026-10-09). OCO lists are cancelled once per list.
func (t *BStockTrader) CancelAllOrders(symbol string) error {
	t.execMu.Lock()
	defer t.execMu.Unlock()
	if _, err := t.rule(symbol); err != nil {
		return err
	}
	orders, err := t.openOrders(symbol)
	if err != nil {
		return err
	}
	lists := make(map[int64]bool)
	for _, o := range orders {
		if !strings.HasPrefix(o.ClientOrderID, clientPrefix) {
			continue
		}
		if o.OrderListId >= 0 {
			if lists[o.OrderListId] {
				continue
			}
			lists[o.OrderListId] = true
			_, err = t.client.NewCancelOCOService().Symbol(symbol).OrderListID(o.OrderListId).NewClientOrderID(clientID()).Do(context.Background())
		} else {
			_, err = t.client.NewCancelOrderService().Symbol(symbol).OrderID(o.OrderID).NewClientOrderID(clientID()).Do(context.Background())
		}
		t.invalidate()
		if err != nil {
			return wrap("cancel program orders", err)
		}
	}
	return nil
}
