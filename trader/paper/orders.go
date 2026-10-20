package paper

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"nofx/store"
	"nofx/trader/types"
)

func (p *PaperFuturesTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	_, text, _, err := p.quantity(symbolName(symbol), quantity)
	return text, err
}

func (p *PaperFuturesTrader) quantity(symbol string, quantity float64) (float64, string, QuantityRule, error) {
	p.mu.Lock()
	err := p.initErr
	p.mu.Unlock()
	if err != nil {
		return 0, "", QuantityRule{}, err
	}
	if !positive(quantity) {
		return 0, "", QuantityRule{}, fmt.Errorf("quantity must be finite and positive")
	}
	r, err := p.rules.QuantityRules(symbol)
	if err != nil {
		return 0, "", r, err
	}
	step, err := strconv.ParseFloat(r.StepSize, 64)
	if err != nil || !positive(step) {
		return 0, "", r, fmt.Errorf("%s LOT_SIZE trading rule is missing", symbol)
	}
	// Match Binance's step-multiple truncation, including non-power-of-ten lots.
	q := math.Floor(quantity/step+1e-9) * step
	// FormatFloat on the parsed step also handles scientific notation.
	stepText := strconv.FormatFloat(step, 'f', -1, 64)
	precision := 0
	if i := strings.IndexByte(stepText, '.'); i >= 0 {
		precision = len(stepText) - i - 1
	}
	text := strconv.FormatFloat(q, 'f', precision, 64)
	q, _ = strconv.ParseFloat(text, 64)
	if !positive(q) || r.MinQty > 0 && q+step*1e-9 < r.MinQty {
		return 0, "", r, fmt.Errorf("MIN_QTY: %s rounds down to %s", symbol, text)
	}
	if r.MaxQty > 0 && q > r.MaxQty+step*1e-9 {
		return 0, "", r, fmt.Errorf("MAX_QTY: %s exceeds maximum quantity", symbol)
	}
	return q, text, r, nil
}

func (p *PaperFuturesTrader) OpenLong(s string, q float64, l int) (map[string]interface{}, error) {
	return p.marketOrder(s, "LONG", q, l, true)
}
func (p *PaperFuturesTrader) OpenShort(s string, q float64, l int) (map[string]interface{}, error) {
	return p.marketOrder(s, "SHORT", q, l, true)
}
func (p *PaperFuturesTrader) CloseLong(s string, q float64) (map[string]interface{}, error) {
	return p.marketOrder(s, "LONG", q, 0, false)
}
func (p *PaperFuturesTrader) CloseShort(s string, q float64) (map[string]interface{}, error) {
	return p.marketOrder(s, "SHORT", q, 0, false)
}

func (p *PaperFuturesTrader) marketOrder(symbol, ps string, quantity float64, lev int, isOpen bool) (map[string]interface{}, error) {
	symbol = symbolName(symbol)
	now := p.cfg.Now().UTC()
	if symbol == "" || quantity < 0 || math.IsNaN(quantity) || math.IsInf(quantity, 0) || lev < 0 {
		return nil, fmt.Errorf("invalid market order")
	}
	if err := p.Advance(now); err != nil {
		return nil, err
	}
	var rule QuantityRule
	var err error
	if isOpen || quantity != 0 {
		quantity, _, rule, err = p.quantity(symbol, quantity)
		if err != nil {
			return nil, err
		}
	}
	mark, err := p.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var result map[string]interface{}
	err = p.mutateLocked(func() ([]store.PaperLedgerTrade, error) {
		p.s.Marks[symbol] = mark
		if isOpen {
			if rule.MinNotional > 0 && quantity*mark < rule.MinNotional {
				return nil, fmt.Errorf("MIN_NOTIONAL: order amount is below minimum requirement")
			}
			if lev == 0 {
				lev = p.leverage(symbol)
			}
			p.setLeverage(symbol, lev)
			p.cancelMatching(func(o *order) bool { return o.Symbol == symbol && o.PositionSide == ps && o.Type != "LIMIT" }, now)
		} else {
			pos := p.s.Positions[posKey(symbol, ps)]
			if pos == nil {
				return nil, fmt.Errorf("no %s position found for %s", strings.ToLower(ps), symbol)
			}
			if quantity == 0 {
				quantity = pos.Quantity
			}
			if quantity > pos.Quantity+1e-12 {
				return nil, fmt.Errorf("-2022 ReduceOnly Order is rejected: quantity exceeds position")
			}
		}
		side := "BUY"
		if ps == "SHORT" {
			side = "SELL"
		}
		if !isOpen {
			if side == "BUY" {
				side = "SELL"
			} else {
				side = "BUY"
			}
		}
		o := p.newOrder(symbol, side, ps, "MARKET", quantity, 0, now)
		reason := ""
		if !isOpen {
			reason = "manual"
		}
		trade, e := p.execute(o, p.slipped(mark, side), false, reason, now)
		if e != nil {
			return nil, e
		}
		result = map[string]interface{}{"orderId": o.Number, "symbol": symbol, "status": o.Status, "avgPrice": o.AvgPrice, "executedQty": o.ExecutedQty, "commission": o.Commission}
		if isOpen {
			result["clientOrderId"] = o.ClientID
		}
		return append([]store.PaperLedgerTrade{trade}, p.liquidate(now)...), nil
	})
	return result, err
}

func (p *PaperFuturesTrader) PlaceLimitOrder(req *types.LimitOrderRequest) (*types.LimitOrderResult, error) {
	if req == nil {
		return nil, fmt.Errorf("nil limit order")
	}
	r := *req
	r.Symbol = symbolName(r.Symbol)
	r.Side = strings.ToUpper(r.Side)
	r.PositionSide = strings.ToUpper(r.PositionSide)
	if r.Symbol == "" || (r.Side != "BUY" && r.Side != "SELL") || !positive(r.Price) || r.Leverage < 0 {
		return nil, fmt.Errorf("invalid limit order")
	}
	if r.PositionSide == "" {
		if (r.Side == "BUY") != r.ReduceOnly {
			r.PositionSide = "LONG"
		} else {
			r.PositionSide = "SHORT"
		}
	}
	if r.PositionSide != "LONG" && r.PositionSide != "SHORT" {
		return nil, fmt.Errorf("hedge mode requires LONG or SHORT position side")
	}
	oTest := &order{OpenOrder: types.OpenOrder{Side: r.Side, PositionSide: r.PositionSide}}
	if r.ReduceOnly && opening(oTest) {
		return nil, fmt.Errorf("-2022 ReduceOnly side would increase position")
	}
	now := p.cfg.Now().UTC()
	if err := p.Advance(now); err != nil {
		return nil, err
	}
	qty, _, rule, err := p.quantity(r.Symbol, r.Quantity)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var out *types.LimitOrderResult
	err = p.mutateLocked(func() ([]store.PaperLedgerTrade, error) {
		if r.ClientID != "" {
			id := clientID(r.ClientID)
			for _, o := range p.s.Orders {
				if o.ClientID == id && o.Status == "NEW" {
					return nil, fmt.Errorf("duplicate client order id")
				}
			}
		}
		lev := r.Leverage
		if lev == 0 {
			lev = p.leverage(r.Symbol)
		}
		p.setLeverage(r.Symbol, lev)
		if opening(oTest) {
			if rule.MinNotional > 0 && qty*r.Price < rule.MinNotional {
				return nil, fmt.Errorf("MIN_NOTIONAL: order amount is below minimum requirement")
			}
			_, margin, _ := p.account()
			if p.s.Wallet-margin+1e-9 < qty*r.Price/float64(lev)+qty*r.Price*p.cfg.MakerFeeBps/10000 {
				return nil, fmt.Errorf("-2019 Margin is insufficient")
			}
		} else if pos := p.s.Positions[posKey(r.Symbol, r.PositionSide)]; pos == nil {
			return nil, fmt.Errorf("-2022 ReduceOnly Order is rejected: no position")
		}
		o := p.newOrder(r.Symbol, r.Side, r.PositionSide, "LIMIT", qty, r.Price, now)
		o.ReduceOnly = r.ReduceOnly
		if r.ClientID != "" {
			o.ClientID = clientID(r.ClientID)
		}
		out = &types.LimitOrderResult{OrderID: o.OrderID, ClientID: o.ClientID, Symbol: o.Symbol, Side: o.Side, PositionSide: o.PositionSide, Price: o.Price, Quantity: o.Quantity, Status: o.Status}
		return nil, nil
	})
	return out, err
}

func clientID(tag string) string {
	const prefix = "x-KzrpZaP9"
	if strings.HasPrefix(tag, prefix) {
		if len(tag) > 32 {
			return tag[:32]
		}
		return tag
	}
	if len(tag) > 22 {
		tag = tag[:22]
	}
	return prefix + tag
}

// Binance SL is always closePosition; partial TP has quantity and neither
// closePosition nor an explicit reduceOnly flag in hedge mode.
func (p *PaperFuturesTrader) SetStopLoss(s, ps string, q, price float64) error {
	return p.protection(s, ps, q, price, "STOP_MARKET")
}
func (p *PaperFuturesTrader) SetTakeProfit(s, ps string, q, price float64) error {
	return p.protection(s, ps, q, price, "TAKE_PROFIT_MARKET")
}

func (p *PaperFuturesTrader) protection(symbol, ps string, qty, trigger float64, kind string) error {
	symbol = symbolName(symbol)
	ps = strings.ToUpper(ps)
	if symbol == "" || (ps != "LONG" && ps != "SHORT") || !positive(trigger) || math.IsNaN(qty) || math.IsInf(qty, 0) {
		return fmt.Errorf("invalid protective order")
	}
	now := p.cfg.Now().UTC()
	if err := p.Advance(now); err != nil {
		return err
	}
	closePosition := kind == "STOP_MARKET" || qty <= 0
	if !closePosition {
		var err error
		qty, _, _, err = p.quantity(symbol, qty)
		// Binance falls back to closePosition on TP lot underflow.
		if err != nil {
			if !strings.Contains(err.Error(), "MIN_QTY") {
				return err
			}
			closePosition = true
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mutateLocked(func() ([]store.PaperLedgerTrade, error) {
		if p.s.Positions[posKey(symbol, ps)] == nil {
			return nil, fmt.Errorf("no %s position found for %s", ps, symbol)
		}
		// Mirrors Binance's stale-leg cancel/replace for this kind and side.
		p.cancelMatching(func(o *order) bool { return o.Symbol == symbol && o.PositionSide == ps && o.Type == kind }, now)
		side := "SELL"
		if ps == "SHORT" {
			side = "BUY"
		}
		if closePosition {
			qty = 0
		}
		o := p.newOrder(symbol, side, ps, kind, qty, 0, now)
		o.StopPrice = trigger
		o.ClosePosition = closePosition
		o.Algo = true
		return nil, nil
	})
}

func (p *PaperFuturesTrader) cancel(symbol string, match func(*order) bool) error {
	symbol = symbolName(symbol)
	now := p.cfg.Now().UTC()
	if err := p.Advance(now); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mutateLocked(func() ([]store.PaperLedgerTrade, error) {
		p.cancelMatching(func(o *order) bool { return (symbol == "" || o.Symbol == symbol) && match(o) }, now)
		return nil, nil
	})
}

func (p *PaperFuturesTrader) CancelOrder(symbol, id string) error {
	symbol = symbolName(symbol)
	now := p.cfg.Now().UTC()
	if err := p.Advance(now); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mutateLocked(func() ([]store.PaperLedgerTrade, error) {
		o := p.s.Orders[id]
		if o == nil || o.Symbol != symbol {
			return nil, fmt.Errorf("-2011 Unknown order sent")
		}
		if o.Status != "NEW" {
			return nil, fmt.Errorf("-2011 order is already %s", o.Status)
		}
		o.Status = "CANCELED"
		o.UpdatedAt = now
		return nil, nil
	})
}

func (p *PaperFuturesTrader) CancelProtectiveOrder(s string, o types.OpenOrder) error {
	return p.CancelOrder(s, o.OrderID)
}
func (p *PaperFuturesTrader) CancelAllOrders(s string) error {
	return p.cancel(s, func(*order) bool { return true })
}
func (p *PaperFuturesTrader) CancelStopOrders(s string) error {
	return p.cancel(s, func(o *order) bool { return o.Type != "LIMIT" })
}
func (p *PaperFuturesTrader) CancelStopLossOrders(s string) error {
	return p.CancelStopLossOrdersForSide(s, "")
}
func (p *PaperFuturesTrader) CancelTakeProfitOrders(s string) error {
	return p.CancelTakeProfitOrdersForSide(s, "")
}
func (p *PaperFuturesTrader) CancelStopLossOrdersForSide(s, ps string) error {
	return p.cancel(s, func(o *order) bool {
		return o.Type == "STOP_MARKET" && (ps == "" || o.PositionSide == strings.ToUpper(ps))
	})
}
func (p *PaperFuturesTrader) CancelTakeProfitOrdersForSide(s, ps string) error {
	return p.cancel(s, func(o *order) bool {
		return o.Type == "TAKE_PROFIT_MARKET" && (ps == "" || o.PositionSide == strings.ToUpper(ps))
	})
}
func (p *PaperFuturesTrader) CancelProtectiveOrdersForSide(s, ps string) error {
	ps = strings.ToUpper(ps)
	if ps != "LONG" && ps != "SHORT" {
		return fmt.Errorf("invalid position side")
	}
	return p.cancel(s, func(o *order) bool { return o.Type != "LIMIT" && o.PositionSide == ps })
}
func (p *PaperFuturesTrader) CancelProgramProtectiveOrdersForSide(s, ps string) error {
	return p.CancelProtectiveOrdersForSide(s, ps)
}

func (p *PaperFuturesTrader) SetLeverage(symbol string, leverage int) error {
	symbol = symbolName(symbol)
	if symbol == "" || leverage < 1 {
		return fmt.Errorf("invalid leverage")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mutateLocked(func() ([]store.PaperLedgerTrade, error) { p.setLeverage(symbol, leverage); return nil, nil })
}

func (p *PaperFuturesTrader) setLeverage(symbol string, leverage int) {
	p.s.Leverage[symbol] = leverage
	for _, pos := range p.s.Positions {
		if pos.Symbol == symbol {
			pos.Leverage = leverage
		}
	}
}

func (p *PaperFuturesTrader) SetMarginMode(symbol string, cross bool) error {
	symbol = symbolName(symbol)
	if symbol == "" {
		return fmt.Errorf("invalid symbol")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mutateLocked(func() ([]store.PaperLedgerTrade, error) { p.s.CrossMargin[symbol] = cross; return nil, nil })
}

func (p *PaperFuturesTrader) SetDisplayName(name string) {
	p.mu.Lock()
	p.displayName = name
	p.mu.Unlock()
}

// There is no account/position cache; readers always advance the ledger.
func (p *PaperFuturesTrader) InvalidateAccountCache()  {}
func (p *PaperFuturesTrader) InvalidatePositionCache() {}
