package paper

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"nofx/market"
	"nofx/market/bnweight"
)

// Candle.Time is the UTC opening time of a one-minute candle.
// Open is optional (0 = unknown) and only refines stop fills across gaps.
type Candle struct {
	Time                   time.Time
	Open, High, Low, Close float64
}

// PriceSource must be safe for concurrent calls. Candles1m includes the minute
// containing since; Advance filters out future and pre-order observations.
type PriceSource interface {
	Price(symbol string) (float64, error)
	Candles1m(symbol string, since time.Time) ([]Candle, error)
}

type QuantityRule struct {
	StepSize                    string
	MinQty, MaxQty, MinNotional float64
}

// SymbolRules is injectable through Config or optionally implemented by the
// PriceSource. Tests need neither exchange credentials nor network access.
type SymbolRules interface {
	QuantityRules(symbol string) (QuantityRule, error)
}

type marketPriceSource struct{ client *futures.Client }

// NewMarketPriceSource uses the same public futures ticker endpoint as
// binance.FuturesTrader.GetMarketPrice and market's paginated 1m history reader.
// Do NOT construct a Binance trader: that constructor makes authenticated calls.
func NewMarketPriceSource() PriceSource {
	c := futures.NewClient("", "")
	c.HTTPClient = &http.Client{Timeout: 15 * time.Second, Transport: bnweight.Wrap(nil, bnweight.Normal)}
	return &marketPriceSource{client: c}
}

func (s *marketPriceSource) Price(symbol string) (float64, error) {
	rows, err := s.client.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("price not found: %s", symbol)
	}
	return strconv.ParseFloat(rows[0].Price, 64)
}

func (s *marketPriceSource) Candles1m(symbol string, since time.Time) ([]Candle, error) {
	now := time.Now().UTC()
	start := since.Truncate(time.Minute)
	if !now.After(start) {
		return nil, nil
	}
	rows, err := market.GetKlinesRange(symbol, "1m", start, now)
	if err != nil {
		return nil, err
	}
	out := make([]Candle, 0, len(rows))
	for _, r := range rows {
		out = append(out, Candle{Time: time.UnixMilli(r.OpenTime).UTC(), Open: r.Open, High: r.High, Low: r.Low, Close: r.Close})
	}
	return out, nil
}

// QuantityRules mirrors Binance's LOT_SIZE and MIN_NOTIONAL filter lookup.
// Binance's getSymbolOrderRules is private, so share its public SDK endpoint
// and filter semantics without adding a dependency on an authenticated trader.
func (s *marketPriceSource) QuantityRules(symbol string) (QuantityRule, error) {
	info, err := s.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return QuantityRule{}, err
	}
	for _, sym := range info.Symbols {
		if sym.Symbol != symbol {
			continue
		}
		r := QuantityRule{MinNotional: 10}
		for _, f := range sym.Filters {
			switch f["filterType"] {
			case "LOT_SIZE":
				r.StepSize, _ = f["stepSize"].(string)
				min, _ := f["minQty"].(string)
				r.MinQty, _ = strconv.ParseFloat(min, 64)
				max, _ := f["maxQty"].(string)
				r.MaxQty, _ = strconv.ParseFloat(max, 64)
			case "MIN_NOTIONAL", "NOTIONAL":
				n, _ := f["notional"].(string)
				if v, e := strconv.ParseFloat(n, 64); e == nil && v > 0 {
					r.MinNotional = v
				}
			}
		}
		return r, nil
	}
	return QuantityRule{}, fmt.Errorf("%s trading rules not found", symbol)
}

func (s *marketPriceSource) OrderBook(symbol string, depth int) ([][]float64, [][]float64, error) {
	// Binance permits only these REST depth limits.
	limit := 5
	for _, n := range []int{5, 10, 20, 50, 100, 500, 1000} {
		limit = n
		if n >= depth {
			break
		}
	}
	book, err := s.client.NewDepthService().Symbol(symbol).Limit(limit).Do(context.Background())
	if err != nil {
		return nil, nil, err
	}
	bids, asks := make([][]float64, 0, len(book.Bids)), make([][]float64, 0, len(book.Asks))
	for _, r := range book.Bids {
		price, quantity, e := r.Parse()
		if e != nil {
			return nil, nil, e
		}
		bids = append(bids, []float64{price, quantity})
	}
	for _, r := range book.Asks {
		price, quantity, e := r.Parse()
		if e != nil {
			return nil, nil, e
		}
		asks = append(asks, []float64{price, quantity})
	}
	return bids, asks, nil
}
