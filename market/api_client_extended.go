package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// === Response structs for extended Binance FAPI endpoints ===

type Ticker24hResponse struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	WeightedAvgPrice   string `json:"weightedAvgPrice"`
	LastPrice          string `json:"lastPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	OpenPrice          string `json:"openPrice"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
	Count              int64  `json:"count"`
}

type LongShortRatioResponse struct {
	Symbol         string `json:"symbol"`
	LongShortRatio string `json:"longShortRatio"`
	LongAccount    string `json:"longAccount"`
	ShortAccount   string `json:"shortAccount"`
	Timestamp      int64  `json:"timestamp"`
}

type TakerBuySellRatioResponse struct {
	BuySellRatio string `json:"buySellRatio"`
	BuyVol       string `json:"buyVol"`
	SellVol      string `json:"sellVol"`
	Timestamp    int64  `json:"timestamp"`
}

type OpenInterestHistResponse struct {
	Symbol               string `json:"symbol"`
	SumOpenInterest      string `json:"sumOpenInterest"`
	SumOpenInterestValue string `json:"sumOpenInterestValue"`
	Timestamp            int64  `json:"timestamp"`
}

type OrderBookDepthResponse struct {
	LastUpdateId int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"` // [price, qty]
	Asks         [][]string `json:"asks"`
}

// === API methods ===

func (c *APIClient) doGet(url string, params map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// GetTicker24h returns 24h ticker for a single symbol
func (c *APIClient) GetTicker24h(symbol string) (*Ticker24hResponse, error) {
	body, err := c.doGet(baseURL+"/fapi/v1/ticker/24hr", map[string]string{"symbol": symbol})
	if err != nil {
		return nil, err
	}
	var result Ticker24hResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAllTickers24h returns 24h tickers for all symbols
func (c *APIClient) GetAllTickers24h() ([]Ticker24hResponse, error) {
	body, err := c.doGet(baseURL+"/fapi/v1/ticker/24hr", nil)
	if err != nil {
		return nil, err
	}
	var result []Ticker24hResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetLongShortRatio returns global account long/short ratio
func (c *APIClient) GetLongShortRatio(symbol, period string) ([]LongShortRatioResponse, error) {
	body, err := c.doGet(baseURL+"/futures/data/globalLongShortAccountRatio", map[string]string{
		"symbol": symbol,
		"period": period,
		"limit":  "1",
	})
	if err != nil {
		return nil, err
	}
	var result []LongShortRatioResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetTopTraderLongShortRatio returns top trader long/short position ratio
func (c *APIClient) GetTopTraderLongShortRatio(symbol, period string) ([]LongShortRatioResponse, error) {
	body, err := c.doGet(baseURL+"/futures/data/topLongShortPositionRatio", map[string]string{
		"symbol": symbol,
		"period": period,
		"limit":  "1",
	})
	if err != nil {
		return nil, err
	}
	var result []LongShortRatioResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetTakerBuySellRatio returns taker buy/sell volume ratio
func (c *APIClient) GetTakerBuySellRatio(symbol, period string) ([]TakerBuySellRatioResponse, error) {
	body, err := c.doGet(baseURL+"/futures/data/takerlongshortRatio", map[string]string{
		"symbol": symbol,
		"period": period,
		"limit":  "1",
	})
	if err != nil {
		return nil, err
	}
	var result []TakerBuySellRatioResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetOpenInterestHist returns OI history for trend analysis
func (c *APIClient) GetOpenInterestHist(symbol, period string, limit int) ([]OpenInterestHistResponse, error) {
	body, err := c.doGet(baseURL+"/futures/data/openInterestHist", map[string]string{
		"symbol": symbol,
		"period": period,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}
	var result []OpenInterestHistResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetOrderBookDepth returns orderbook depth
func (c *APIClient) GetOrderBookDepth(symbol string, limit int) (*OrderBookDepthResponse, error) {
	body, err := c.doGet(baseURL+"/fapi/v1/depth", map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	})
	if err != nil {
		return nil, err
	}
	var result OrderBookDepthResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CalculateDepthTotals computes total bid/ask depth in USDT from orderbook
func CalculateDepthTotals(depth *OrderBookDepthResponse) (bidTotal, askTotal float64) {
	for _, bid := range depth.Bids {
		if len(bid) >= 2 {
			price, _ := strconv.ParseFloat(bid[0], 64)
			qty, _ := strconv.ParseFloat(bid[1], 64)
			bidTotal += price * qty
		}
	}
	for _, ask := range depth.Asks {
		if len(ask) >= 2 {
			price, _ := strconv.ParseFloat(ask[0], 64)
			qty, _ := strconv.ParseFloat(ask[1], 64)
			askTotal += price * qty
		}
	}
	return
}
