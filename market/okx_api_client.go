package market

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const okxBaseURL = "https://www.okx.com"

// OKXAPIClient provides access to OKX public market data endpoints
type OKXAPIClient struct {
	client *APIClient // reuse HTTP client from APIClient
}

// NewOKXAPIClient creates a new OKX API client
func NewOKXAPIClient() *OKXAPIClient {
	return &OKXAPIClient{client: NewAPIClient()}
}

// OKX response wrapper
type okxResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (o *OKXAPIClient) doGet(path string, params map[string]string) (json.RawMessage, error) {
	body, err := o.client.doGet(okxBaseURL+path, params)
	if err != nil {
		return nil, err
	}
	var resp okxResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("OKX API error: code=%s msg=%s", resp.Code, resp.Msg)
	}
	return resp.Data, nil
}

// Symbol conversion helpers
// Binance: BTCUSDT → OKX: BTC-USDT-SWAP
func binanceToOKXSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		base := strings.TrimSuffix(symbol, "USDT")
		return base + "-USDT-SWAP"
	}
	return symbol
}

// OKX: BTC-USDT-SWAP → Binance: BTCUSDT
func okxToBinanceSymbol(symbol string) string {
	parts := strings.Split(symbol, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return symbol
}

// GetKlines returns kline data from OKX
func (o *OKXAPIClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	okxSymbol := binanceToOKXSymbol(symbol)
	okxInterval := convertInterval(interval)

	data, err := o.doGet("/api/v5/market/candles", map[string]string{
		"instId": okxSymbol,
		"bar":    okxInterval,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}

	var rawKlines [][]string
	if err := json.Unmarshal(data, &rawKlines); err != nil {
		return nil, err
	}

	// OKX returns newest first, reverse to match Binance order
	var klines []Kline
	for i := len(rawKlines) - 1; i >= 0; i-- {
		k := rawKlines[i]
		if len(k) < 7 {
			continue
		}
		openTime, _ := strconv.ParseInt(k[0], 10, 64)
		open, _ := strconv.ParseFloat(k[1], 64)
		high, _ := strconv.ParseFloat(k[2], 64)
		low, _ := strconv.ParseFloat(k[3], 64)
		close_, _ := strconv.ParseFloat(k[4], 64)
		vol, _ := strconv.ParseFloat(k[5], 64)   // volume in contracts
		qvol, _ := strconv.ParseFloat(k[6], 64)  // volume in quote currency

		klines = append(klines, Kline{
			OpenTime:    openTime,
			Open:        open,
			High:        high,
			Low:         low,
			Close:       close_,
			Volume:      vol,
			QuoteVolume: qvol,
		})
	}
	return klines, nil
}

// GetTicker returns 24h ticker for a symbol
func (o *OKXAPIClient) GetTicker(symbol string) (*Ticker24hResponse, error) {
	okxSymbol := binanceToOKXSymbol(symbol)
	data, err := o.doGet("/api/v5/market/ticker", map[string]string{
		"instId": okxSymbol,
	})
	if err != nil {
		return nil, err
	}

	var tickers []struct {
		InstId    string `json:"instId"`
		Last      string `json:"last"`
		Open24h   string `json:"open24h"`
		High24h   string `json:"high24h"`
		Low24h    string `json:"low24h"`
		Vol24h    string `json:"vol24h"`
		VolCcy24h string `json:"volCcy24h"`
	}
	if err := json.Unmarshal(data, &tickers); err != nil {
		return nil, err
	}
	if len(tickers) == 0 {
		return nil, fmt.Errorf("no ticker data for %s", symbol)
	}
	t := tickers[0]
	last, _ := strconv.ParseFloat(t.Last, 64)
	open24h, _ := strconv.ParseFloat(t.Open24h, 64)
	var pctChange float64
	if open24h > 0 {
		pctChange = (last - open24h) / open24h * 100
	}

	return &Ticker24hResponse{
		Symbol:             okxToBinanceSymbol(t.InstId),
		LastPrice:          t.Last,
		QuoteVolume:        t.VolCcy24h,
		Volume:             t.Vol24h,
		PriceChangePercent: fmt.Sprintf("%.4f", pctChange),
	}, nil
}

// GetOpenInterest returns open interest for a symbol
func (o *OKXAPIClient) GetOpenInterest(symbol string) (*OIData, error) {
	okxSymbol := binanceToOKXSymbol(symbol)
	data, err := o.doGet("/api/v5/public/open-interest", map[string]string{
		"instType": "SWAP",
		"instId":   okxSymbol,
	})
	if err != nil {
		return nil, err
	}

	var results []struct {
		Oi    string `json:"oi"`
		OiCcy string `json:"oiCcy"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no OI data for %s", symbol)
	}
	oi, _ := strconv.ParseFloat(results[0].Oi, 64)
	return &OIData{Latest: oi, Average: oi * 0.999}, nil
}

// GetFundingRate returns the current funding rate
func (o *OKXAPIClient) GetFundingRate(symbol string) (float64, error) {
	okxSymbol := binanceToOKXSymbol(symbol)
	data, err := o.doGet("/api/v5/public/funding-rate", map[string]string{
		"instId": okxSymbol,
	})
	if err != nil {
		return 0, err
	}

	var results []struct {
		FundingRate string `json:"fundingRate"`
	}
	if err := json.Unmarshal(data, &results); err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, fmt.Errorf("no funding rate for %s", symbol)
	}
	rate, _ := strconv.ParseFloat(results[0].FundingRate, 64)
	return rate, nil
}

// GetOrderBookDepth returns orderbook depth
func (o *OKXAPIClient) GetOrderBookDepth(symbol string, limit int) (*OrderBookDepthResponse, error) {
	okxSymbol := binanceToOKXSymbol(symbol)
	data, err := o.doGet("/api/v5/market/books", map[string]string{
		"instId": okxSymbol,
		"sz":     strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}

	var books []struct {
		Asks [][]string `json:"asks"`
		Bids [][]string `json:"bids"`
	}
	if err := json.Unmarshal(data, &books); err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return nil, fmt.Errorf("no depth data for %s", symbol)
	}
	// OKX depth format: [price, qty, liquidated_orders, num_orders]
	// Convert to [price, qty] format
	resp := &OrderBookDepthResponse{}
	for _, b := range books[0].Bids {
		if len(b) >= 2 {
			resp.Bids = append(resp.Bids, []string{b[0], b[1]})
		}
	}
	for _, a := range books[0].Asks {
		if len(a) >= 2 {
			resp.Asks = append(resp.Asks, []string{a[0], a[1]})
		}
	}
	return resp, nil
}

// GetLongShortRatio returns account long/short ratio from OKX
func (o *OKXAPIClient) GetLongShortRatio(symbol, period string) ([]LongShortRatioResponse, error) {
	// OKX uses ccy (currency) not symbol
	base := strings.TrimSuffix(strings.ToUpper(symbol), "USDT")
	data, err := o.doGet("/api/v5/rubik/stat/contracts-long-short-account-ratio", map[string]string{
		"ccy":    base,
		"period": period,
	})
	if err != nil {
		return nil, err
	}

	var rawData [][]string // [timestamp, ratio]
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, err
	}

	var results []LongShortRatioResponse
	for _, r := range rawData {
		if len(r) < 2 {
			continue
		}
		ts, _ := strconv.ParseInt(r[0], 10, 64)
		results = append(results, LongShortRatioResponse{
			Symbol:         symbol,
			LongShortRatio: r[1],
			Timestamp:      ts,
		})
	}
	return results, nil
}

// GetTakerVolume returns taker buy/sell volume from OKX
func (o *OKXAPIClient) GetTakerVolume(symbol, period string) ([]TakerBuySellRatioResponse, error) {
	base := strings.TrimSuffix(strings.ToUpper(symbol), "USDT")
	data, err := o.doGet("/api/v5/rubik/stat/taker-volume", map[string]string{
		"ccy":      base,
		"instType": "SWAP",
		"period":   period,
	})
	if err != nil {
		return nil, err
	}

	var rawData [][]string // [timestamp, sellVol, buyVol]
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, err
	}

	var results []TakerBuySellRatioResponse
	for _, r := range rawData {
		if len(r) < 3 {
			continue
		}
		ts, _ := strconv.ParseInt(r[0], 10, 64)
		sellVol, _ := strconv.ParseFloat(r[1], 64)
		buyVol, _ := strconv.ParseFloat(r[2], 64)
		var ratio float64
		if sellVol > 0 {
			ratio = buyVol / sellVol
		}
		results = append(results, TakerBuySellRatioResponse{
			BuySellRatio: fmt.Sprintf("%.6f", ratio),
			BuyVol:       r[2],
			SellVol:      r[1],
			Timestamp:    ts,
		})
	}
	return results, nil
}

// convertInterval converts Binance-style interval to OKX format
func convertInterval(interval string) string {
	// Most intervals are the same, but OKX uses slightly different naming
	mapping := map[string]string{
		"1m": "1m", "3m": "3m", "5m": "5m", "15m": "15m", "30m": "30m",
		"1h": "1H", "2h": "2H", "4h": "4H", "6h": "6H", "12h": "12H",
		"1d": "1D", "3d": "3D", "1w": "1W",
	}
	if v, ok := mapping[strings.ToLower(interval)]; ok {
		return v
	}
	return interval
}
