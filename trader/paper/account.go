package paper

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"nofx/trader/types"
)

func (p *PaperFuturesTrader) GetBalance() (map[string]interface{}, error) {
	if err := p.Advance(p.cfg.Now()); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	u, margin, _ := p.account()
	return map[string]interface{}{"totalWalletBalance": p.s.Wallet, "availableBalance": p.s.Wallet - margin, "totalUnrealizedProfit": u, "totalEquity": p.s.Wallet + u}, nil
}

func (p *PaperFuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	if err := p.Advance(p.cfg.Now()); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	keys := make([]string, 0, len(p.s.Positions))
	for key := range p.s.Positions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]map[string]interface{}, 0, len(keys))
	u, _, n := p.account()
	excess := p.s.Wallet + u - n*p.cfg.MaintenanceMarginPct/100
	for _, key := range keys {
		v := p.s.Positions[key]
		mark := p.s.Marks[v.Symbol]
		direction := 1.0
		if v.Side == "SHORT" {
			direction = -1
		}
		// Solve account equity == maintenance as this symbol's mark changes,
		// holding other symbols fixed. Hedged sides share a liquidation mark.
		net, gross := 0.0, 0.0
		for _, other := range p.s.Positions {
			if other.Symbol == v.Symbol {
				d := 1.0
				if other.Side == "SHORT" {
					d = -1
				}
				net += d * other.Quantity
				gross += other.Quantity
			}
		}
		slope := net - gross*p.cfg.MaintenanceMarginPct/100
		liq := 0.0
		if math.Abs(slope) > 1e-12 {
			liq = math.Max(0, mark-excess/slope)
		}
		out = append(out, map[string]interface{}{"symbol": v.Symbol, "side": strings.ToLower(v.Side), "positionAmt": direction * v.Quantity, "entryPrice": v.EntryPrice, "markPrice": mark, "unRealizedProfit": direction * (mark - v.EntryPrice) * v.Quantity, "leverage": float64(v.Leverage), "liquidationPrice": liq})
	}
	return out, nil
}

func (p *PaperFuturesTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	if err := p.Advance(p.cfg.Now()); err != nil {
		return nil, err
	}
	symbol = symbolName(symbol)
	p.mu.Lock()
	defer p.mu.Unlock()
	list := make([]*order, 0)
	for _, o := range p.s.Orders {
		if o.Status == "NEW" && (symbol == "" || o.Symbol == symbol) {
			list = append(list, o)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Number < list[j].Number })
	out := make([]types.OpenOrder, 0, len(list))
	for _, o := range list {
		out = append(out, o.OpenOrder)
	}
	return out, nil
}

func (p *PaperFuturesTrader) GetOrderStatus(symbol, id string) (map[string]interface{}, error) {
	if err := p.Advance(p.cfg.Now()); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	o := p.s.Orders[id]
	if o == nil || o.Symbol != symbolName(symbol) {
		return nil, fmt.Errorf("-2013 Order does not exist")
	}
	return map[string]interface{}{"orderId": o.Number, "symbol": o.Symbol, "status": o.Status, "avgPrice": o.AvgPrice, "executedQty": o.ExecutedQty, "side": o.Side, "type": o.Type, "time": o.CreatedAt.UnixMilli(), "updateTime": o.UpdatedAt.UnixMilli(), "commission": o.Commission}, nil
}

func (p *PaperFuturesTrader) GetClosedPnL(start time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	if err := p.Advance(p.cfg.Now()); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []types.ClosedPnLRecord{}
	for _, r := range p.s.Closed {
		if !r.ExitTime.Before(start) {
			out = append(out, r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (p *PaperFuturesTrader) GetMarketPrice(symbol string) (float64, error) {
	p.mu.Lock()
	err := p.initErr
	p.mu.Unlock()
	if err != nil {
		return 0, err
	}
	price, err := p.price.Price(symbolName(symbol))
	if err != nil {
		return 0, err
	}
	if !positive(price) {
		return 0, fmt.Errorf("invalid price for %s", symbol)
	}
	return price, nil
}

func (p *PaperFuturesTrader) GetOrderBook(symbol string, depth int) ([][]float64, [][]float64, error) {
	p.mu.Lock()
	err := p.initErr
	p.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	if book, ok := p.price.(interface {
		OrderBook(string, int) ([][]float64, [][]float64, error)
	}); ok {
		return book.OrderBook(symbolName(symbol), depth)
	}
	return nil, nil, fmt.Errorf("price source does not provide an order book")
}

type AccountSummary struct {
	InitialBalance, WalletBalance, Equity float64
	RealizedPnL, UnrealizedPnL, TotalFees float64
	TradeCount                            int
}

func (p *PaperFuturesTrader) Summary() (AccountSummary, error) {
	if err := p.Advance(p.cfg.Now()); err != nil {
		return AccountSummary{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	u, _, _ := p.account()
	return AccountSummary{InitialBalance: p.row.InitialBalance, WalletBalance: p.s.Wallet, Equity: p.s.Wallet + u, RealizedPnL: p.s.Realized, UnrealizedPnL: u, TotalFees: p.s.Fees, TradeCount: p.s.TradeCount}, nil
}
