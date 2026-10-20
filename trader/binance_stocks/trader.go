package binance_stocks

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nofx/logger"
	"nofx/trader/types"
)

// StocksTrader trades U.S.-listed equities via the Binance Stocks API.
// It satisfies the common trader interface used by AutoTrader:
//
//	open_long  → BUY  MARKET (by notional, computed from quantity×price)
//	close_long → SELL MARKET (by quantity)
//	short actions → unsupported (US equities on Binance are long-only)
type StocksTrader struct {
	client *Client

	// symbol rules cache (stepSize/minQty/minNotional)
	rulesMu   sync.RWMutex
	rules     map[string]EquitySymbol
	rulesTime time.Time

	// balance cache
	balMu    sync.RWMutex
	balCache map[string]interface{}
	balTime  time.Time

	// positions cache
	posMu    sync.RWMutex
	posCache []map[string]interface{}
	posTime  time.Time
}

// NewStocksTrader creates a stocks trader.
func NewStocksTrader(apiKey, secretKey string) *StocksTrader {
	return &StocksTrader{client: NewClient(apiKey, secretKey)}
}

// EnsureDisclaimer signs the US-equity disclaimer (idempotent on Binance side).
// Call once after creating the trader; safe to call repeatedly.
func (t *StocksTrader) EnsureDisclaimer() error {
	if err := t.client.SignDisclaimer(context.Background()); err != nil {
		return fmt.Errorf("failed to sign US equity disclaimer: %w", err)
	}
	logger.Info("✓ US equity disclaimer signed")
	return nil
}

// GetBalance returns the spot wallet equity (USD + USDC) as the account value.
func (t *StocksTrader) GetBalance() (map[string]interface{}, error) {
	t.balMu.RLock()
	if t.balCache != nil && time.Since(t.balTime) < 15*time.Second {
		cached := t.balCache
		t.balMu.RUnlock()
		return cached, nil
	}
	t.balMu.RUnlock()

	balances, err := t.client.GetSpotBalances(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get spot balances: %w", err)
	}

	total, available := 0.0, 0.0
	for _, b := range balances {
		asset := strings.ToUpper(b.Asset)
		if asset != "USDC" && asset != "USD" && asset != "USDT" {
			continue
		}
		free, _ := strconv.ParseFloat(b.Free, 64)
		locked, _ := strconv.ParseFloat(b.Locked, 64)
		available += free
		total += free + locked
	}

	result := map[string]interface{}{
		"totalEquity":           total,
		"totalWalletBalance":    total,
		"availableBalance":      available,
		"totalUnrealizedProfit": 0.0, // unrealized equity PnL lives in the position value
	}
	logger.Infof("✓ Stocks account balance: total=%.2f, available=%.2f (USD/USDC/USDT)", total, available)

	t.balMu.Lock()
	t.balCache = result
	t.balTime = time.Now()
	t.balMu.Unlock()
	return result, nil
}

// position is an internally aggregated equity position.
type position struct {
	Symbol   string
	Quantity float64
	AvgEntry float64
	Realized float64
}

// aggregatePositions derives net positions from equity trade history.
func (t *StocksTrader) aggregatePositions(ctx context.Context) ([]position, error) {
	symbols, err := t.client.GetExchangeInfo(ctx, "")
	if err != nil {
		return nil, err
	}
	// Aggregate over traded symbols only: query history per tradable symbol is
	// too chatty, so we rely on the caller knowing its symbols. Binance exposes
	// no cross-symbol fills listing, so we scan symbols the strategy trades via
	// GetTradesForSymbol; here we return per-symbol aggregation lazily.
	_ = symbols
	return nil, nil
}

// GetPositions returns current equity positions derived from recent trade
// history of the symbols the strategy trades. positionsCache short-circuits
// repeated calls within the cache window.
func (t *StocksTrader) GetPositions() ([]map[string]interface{}, error) {
	t.posMu.RLock()
	if t.posCache != nil && time.Since(t.posTime) < 15*time.Second {
		cached := t.posCache
		t.posMu.RUnlock()
		return cached, nil
	}
	t.posMu.RUnlock()

	// The equity API has no cross-symbol position endpoint and nothing ever
	// fills posCache, so the account's holdings are UNKNOWN here. Returning a
	// successful empty list would make every consumer (exposure checks, program
	// stops, equity) believe the account is flat; fail loudly instead.
	return nil, fmt.Errorf("binance_stocks: positions are not available (no account-wide position snapshot); refusing to report an empty account")
}

// GetPositionsForSymbol derives the net position for one symbol from its fill
// history (BUY adds, SELL reduces) over the recent window.
func (t *StocksTrader) GetPositionsForSymbol(symbol string, since time.Time) (map[string]interface{}, error) {
	symbol = ticker(symbol)
	fills, err := t.client.GetTradeHistory(context.Background(), symbol, since, 500)
	if err != nil {
		return nil, err
	}

	var pos position
	for _, f := range fills {
		qty, _ := strconv.ParseFloat(f.Quantity, 64)
		price, _ := strconv.ParseFloat(f.Price, 64)
		if qty <= 0 || price <= 0 {
			continue
		}
		if f.Side == "BUY" {
			totalCost := pos.AvgEntry*pos.Quantity + price*qty
			pos.Quantity += qty
			if pos.Quantity > 0 {
				pos.AvgEntry = totalCost / pos.Quantity
			}
		} else {
			if pos.Quantity > 0 {
				pos.Realized += (price - pos.AvgEntry) * math.Min(qty, pos.Quantity)
			}
			pos.Quantity -= qty
			if pos.Quantity <= 0.0000001 {
				pos.Quantity = 0
				pos.AvgEntry = 0
			}
		}
	}

	out := map[string]interface{}{
		"symbol":       strings.ToUpper(symbol),
		"position_amt": pos.Quantity,
		"entry_price":  pos.AvgEntry,
		"side":         map[bool]string{true: "LONG", false: "NONE"}[pos.Quantity > 0],
		"leverage":     1,
	}
	return out, nil
}

// OpenLong buys the equity by MARKET notional (quantity × current price).
func (t *StocksTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	symbol = ticker(symbol)
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get price for notional sizing: %w", err)
	}
	notional := quantity * price
	order, err := t.client.PlaceOrder(context.Background(), OrderRequest{
		Symbol:    symbol,
		Side:      "BUY",
		OrderType: "MARKET",
		Notional:  notional,
	})
	if err != nil {
		return nil, err
	}
	logger.Infof("✓ Stocks BUY (open long) placed: %s notional=%.2f (%.6g @ ~%.4f), orderID=%d",
		symbol, notional, quantity, price, order.OrderID)
	return map[string]interface{}{
		"orderId": order.OrderID,
		"symbol":  order.Symbol,
		"status":  order.Status,
	}, nil
}

// OpenShort is unsupported: Binance US equities are long-only cash positions.
func (t *StocksTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, fmt.Errorf("short selling is not supported on Binance Stocks (long-only product)")
}

// CloseLong sells the equity by MARKET quantity (0 = sell entire position).
func (t *StocksTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	symbol = ticker(symbol)
	if quantity <= 0 {
		pos, err := t.GetPositionsForSymbol(symbol, time.Now().Add(-30*24*time.Hour))
		if err != nil {
			return nil, fmt.Errorf("failed to derive position for close-all: %w", err)
		}
		q, _ := pos["position_amt"].(float64)
		quantity = q
		if quantity <= 0 {
			return nil, fmt.Errorf("no long position to close for %s", symbol)
		}
	}
	order, err := t.client.PlaceOrder(context.Background(), OrderRequest{
		Symbol:    symbol,
		Side:      "SELL",
		OrderType: "MARKET",
		Quantity:  quantity,
	})
	if err != nil {
		return nil, err
	}
	logger.Infof("✓ Stocks SELL (close long) placed: %s qty=%.6g, orderID=%d", symbol, quantity, order.OrderID)
	return map[string]interface{}{
		"orderId": order.OrderID,
		"symbol":  order.Symbol,
		"status":  order.Status,
	}, nil
}

// CloseShort is unsupported (long-only product).
func (t *StocksTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, fmt.Errorf("short selling is not supported on Binance Stocks (long-only product)")
}

// SetLeverage is a no-op: cash equity market has no leverage.
func (t *StocksTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

// SetMarginMode is a no-op: no margin on the equity product.
func (t *StocksTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}

// GetMarketPrice returns the latest quote price.
func (t *StocksTrader) GetMarketPrice(symbol string) (float64, error) {
	q, err := t.client.GetLatestQuote(context.Background(), ticker(symbol))
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(q.Price, 64)
}

// SetStopLoss is unsupported: the stocks API has no stop order type. The AI
// must manage exits via close_long decisions in later cycles.
func (t *StocksTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return fmt.Errorf("exchange-side stop-loss is not supported on Binance Stocks — the AI must manage exits via close decisions")
}

// SetTakeProfit is unsupported for the same reason.
func (t *StocksTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return fmt.Errorf("exchange-side take-profit is not supported on Binance Stocks — the AI must manage exits via close decisions")
}

// CancelStopLossOrders cancels open LIMIT orders (no stop orders exist).
func (t *StocksTrader) CancelStopLossOrders(symbol string) error {
	return nil // no stop orders on the equity product
}

// CancelTakeProfitOrders cancels open LIMIT orders (no take-profit orders exist).
func (t *StocksTrader) CancelTakeProfitOrders(symbol string) error {
	return nil
}

// CancelAllOrders cancels all open orders for the symbol.
func (t *StocksTrader) CancelAllOrders(symbol string) error {
	return t.client.CancelAllOrders(context.Background(), ticker(symbol))
}

// CancelStopOrders is an alias for CancelAllOrders.
func (t *StocksTrader) CancelStopOrders(symbol string) error {
	return t.CancelAllOrders(symbol)
}

// FormatQuantity rounds quantity to the symbol's stepSize.
func (t *StocksTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	step, err := t.stepSizeFor(ticker(symbol))
	if err != nil {
		return strconv.FormatFloat(quantity, 'f', 6, 64), nil // best effort
	}
	s, parseErr := strconv.ParseFloat(step, 64)
	if parseErr != nil || s <= 0 {
		return strconv.FormatFloat(quantity, 'f', 6, 64), nil
	}
	rounded := math.Floor(quantity/s) * s
	return strconv.FormatFloat(rounded, 'f', -1, 64), nil
}

func (t *StocksTrader) stepSizeFor(symbol string) (string, error) {
	t.rulesMu.RLock()
	if t.rules != nil && time.Since(t.rulesTime) < 10*time.Minute {
		r, ok := t.rules[symbol]
		t.rulesMu.RUnlock()
		if ok {
			return r.StepSize, nil
		}
		t.rulesMu.RUnlock()
	} else {
		t.rulesMu.RUnlock()
	}
	symbols, err := t.client.GetExchangeInfo(context.Background(), symbol)
	if err != nil {
		return "", err
	}
	t.rulesMu.Lock()
	if t.rules == nil {
		t.rules = map[string]EquitySymbol{}
	}
	for _, s := range symbols {
		t.rules[s.Symbol] = s
	}
	t.rulesTime = time.Now()
	t.rulesMu.Unlock()

	if r, ok := t.rules[symbol]; ok {
		return r.StepSize, nil
	}
	return "", fmt.Errorf("symbol %s not found in equity exchangeInfo", symbol)
}

// GetOrderStatus returns the order status in the common map shape.
func (t *StocksTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	symbol = ticker(symbol)
	id, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid order id %q: %w", orderID, err)
	}
	o, err := t.client.GetOrderDetail(context.Background(), symbol, id)
	if err != nil {
		return nil, err
	}
	avgPrice, _ := strconv.ParseFloat(o.Price, 64)
	executed, _ := strconv.ParseFloat(o.ExecutedQty, 64)
	return map[string]interface{}{
		"status":      o.Status,
		"avgPrice":    avgPrice,
		"executedQty": executed,
		"commission":  o.Commission,
	}, nil
}

// GetClosedPnL derives realized-PnL records from equity fill history.
func (t *StocksTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	// The equity API lists fills per symbol only; symbols come from order
	// history of the strategy universe — callers with a symbol should prefer
	// GetClosedPnLForSymbol. Without a symbol we return an empty set.
	return []types.ClosedPnLRecord{}, nil
}

// GetClosedPnLForSymbol derives realized-PnL records for one symbol.
func (t *StocksTrader) GetClosedPnLForSymbol(symbol string, startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	symbol = ticker(symbol)
	fills, err := t.client.GetTradeHistory(context.Background(), symbol, startTime, limit)
	if err != nil {
		return nil, err
	}

	var avgEntry float64
	var qty float64
	var records []types.ClosedPnLRecord
	for _, f := range fills {
		q, _ := strconv.ParseFloat(f.Quantity, 64)
		p, _ := strconv.ParseFloat(f.Price, 64)
		fee, _ := strconv.ParseFloat(f.Commission, 64)
		if q <= 0 || p <= 0 {
			continue
		}
		when := time.UnixMilli(f.Time).UTC()
		if f.Side == "BUY" {
			totalCost := avgEntry*qty + p*q
			qty += q
			if qty > 0 {
				avgEntry = totalCost / qty
			}
			continue
		}
		// SELL against a long position = realized PnL
		if qty > 0 {
			pnl := (p - avgEntry) * math.Min(q, qty)
			records = append(records, types.ClosedPnLRecord{
				Symbol:      symbol,
				Side:        "long",
				EntryPrice:  avgEntry,
				ExitPrice:   p,
				Quantity:    math.Min(q, qty),
				RealizedPnL: pnl,
				Fee:         fee,
				ExitTime:    when,
				EntryTime:   when,
				OrderID:     strconv.FormatInt(f.OrderID, 10),
				ExchangeID:  strconv.FormatInt(f.ID, 10),
				CloseType:   "unknown",
			})
			qty -= q
			if qty <= 0.0000001 {
				qty, avgEntry = 0, 0
			}
		}
	}
	// newest first, matching futures behavior
	sort.Slice(records, func(i, j int) bool { return records[i].ExitTime.After(records[j].ExitTime) })
	return records, nil
}

// GetOpenOrders returns pending equity orders in the common shape.
func (t *StocksTrader) GetOpenOrders(symbol string) ([]types.OpenOrder, error) {
	symbol = ticker(symbol)
	orders, err := t.client.GetOpenOrders(context.Background(), symbol)
	if err != nil {
		return nil, err
	}
	var out []types.OpenOrder
	for _, o := range orders {
		price, _ := strconv.ParseFloat(o.Price, 64)
		qty, _ := strconv.ParseFloat(o.OrigQty, 64)
		out = append(out, types.OpenOrder{
			OrderID:      strconv.FormatInt(o.OrderID, 10),
			Symbol:       o.Symbol,
			Side:         o.Side,
			PositionSide: "LONG",
			Type:         o.Type,
			Price:        price,
			StopPrice:    0,
			Quantity:     qty,
			Status:       o.Status,
		})
	}
	return out, nil
}

// GetTradesForSymbol returns raw equity fills (used by the order sync).
func (t *StocksTrader) GetTradesForSymbol(symbol string, startTime time.Time, limit int) ([]types.TradeRecord, error) {
	fills, err := t.client.GetTradeHistory(context.Background(), symbol, startTime, limit)
	if err != nil {
		return nil, err
	}
	var out []types.TradeRecord
	for _, f := range fills {
		p, _ := strconv.ParseFloat(f.Price, 64)
		q, _ := strconv.ParseFloat(f.Quantity, 64)
		fee, _ := strconv.ParseFloat(f.Commission, 64)
		out = append(out, types.TradeRecord{
			TradeID:      strconv.FormatInt(f.ID, 10),
			OrderID:      strconv.FormatInt(f.OrderID, 10),
			Symbol:       f.Symbol,
			Side:         f.Side,
			PositionSide: "LONG",
			Price:        p,
			Quantity:     q,
			Fee:          fee,
			Time:         time.UnixMilli(f.Time).UTC(),
		})
	}
	return out, nil
}

// ticker strips quote-currency suffixes the strategy engine appends
// (AAPLUSDT → AAPL) and uppercases the result.
func ticker(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	for _, suffix := range []string{"USDT", "USDC", "USD"} {
		if strings.HasSuffix(s, suffix) && len(s) > len(suffix) {
			return strings.TrimSuffix(s, suffix)
		}
	}
	return s
}
