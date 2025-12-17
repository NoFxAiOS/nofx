package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
	"strconv"
)

// OpenInterestAll coin holdings list (order by exchange)
func (c *CoinankClient) OpenInterestAll(ctx context.Context, baseCoin string) ([]OpenInterestAllResponse, error) {
	paramsMap := make(map[string]string, 1)
	paramsMap["baseCoin"] = baseCoin
	resp, err := c.Get(ctx, "/api/openInterest/all", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]OpenInterestAllResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

// OpenInterestChartV2 Exchange History Chart
func (c *CoinankClient) OpenInterestChartV2(ctx context.Context,
	baseCoin string, exchange coinank_enum.Exchange,
	interval coinank_enum.Interval, size int) (*OpenInterestChartV2Response, error) {
	paramsMap := make(map[string]string, 4)
	paramsMap["baseCoin"] = baseCoin
	paramsMap["interval"] = string(interval)
	if exchange != "" {
		paramsMap["exchange"] = string(exchange)
	}
	if size > 0 {
		paramsMap["size"] = strconv.Itoa(size)
	}
	resp, err := c.Get(ctx, "/api/openInterest/v2/chart", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[OpenInterestChartV2Response]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return &result.Data, nil
}

// OpenInterestSymbolChart Trading Pair Open Interest , endTime: Returns data before this timestamp ,is millisecond timestamp
func (c *CoinankClient) OpenInterestSymbolChart(ctx context.Context,
	exchange coinank_enum.Exchange, symbol string, interval coinank_enum.Interval,
	endTime int64, size int) ([]OpenInterestSymbolChartResponse, error) {
	paramsMap := make(map[string]string, 5)
	paramsMap["exchange"] = string(exchange)
	paramsMap["symbol"] = symbol
	paramsMap["interval"] = string(interval)
	paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	if size > 0 {
		paramsMap["size"] = strconv.Itoa(size)
	}
	resp, err := c.Get(ctx, "/api/openInterest/symbol/Chart", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]OpenInterestSymbolChartResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

// OpenInterestKline Trading Pair Open Interest K Line
func (c *CoinankClient) OpenInterestKline(ctx context.Context,
	exchange coinank_enum.Exchange, symbol string, interval coinank_enum.Interval,
	endTime int64, size int) ([]OpenInterestKlineResponse, error) {
	paramsMap := make(map[string]string, 5)
	paramsMap["exchange"] = string(exchange)
	paramsMap["symbol"] = symbol
	paramsMap["interval"] = string(interval)
	paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	if size > 0 {
		paramsMap["size"] = strconv.Itoa(size)
	}
	resp, err := c.Get(ctx, "/api/openInterest/kline", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]OpenInterestKlineResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

// OpenInterestAggKline Aggregation Open Interest K Line
func (c *CoinankClient) OpenInterestAggKline(ctx context.Context,
	baseCoin string, interval coinank_enum.Interval,
	endTime int64, size int) ([]OpenInterestAggKlineResponse, error) {
	paramsMap := make(map[string]string, 4)
	paramsMap["baseCoin"] = baseCoin
	paramsMap["interval"] = string(interval)
	paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	if size > 0 {
		paramsMap["size"] = strconv.Itoa(size)
	}
	resp, err := c.Get(ctx, "/api/openInterest/aggKline", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]OpenInterestAggKlineResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

// TickersTopOIByEx Real-Time Open Interest
func (c *CoinankClient) TickersTopOIByEx(ctx context.Context, baseCoin string) (*TickersTopOIByExResponse, error) {
	paramsMap := make(map[string]string, 1)
	paramsMap["baseCoin"] = baseCoin
	resp, err := c.Get(ctx, "/api/tickers/topOIByEx", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[TickersTopOIByExResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return &result.Data, nil
}

// InstrumentsOiVsMc Oi/MarketCap Ratio History
func (c *CoinankClient) InstrumentsOiVsMc(ctx context.Context,
	baseCoin string, interval coinank_enum.Interval, endTime int64, size int) ([]InstrumentsOiVsMcResponse, error) {
	paramsMap := make(map[string]string, 4)
	paramsMap["baseCoin"] = baseCoin
	paramsMap["interval"] = string(interval)
	paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	if size > 0 {
		paramsMap["size"] = strconv.Itoa(size)
	}
	resp, err := c.Get(ctx, "/api/instruments/oiVsMc", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]InstrumentsOiVsMcResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

type OpenInterestAllResponse struct {
	CoinCount    float64 `json:"coinCount"`    //total number of currency held
	CoinValue    float64 `json:"coinValue"`    //total value of currency held
	ExchangeName string  `json:"exchangeName"` //data from what exchange ,if return `ALL`,means all exchange statistics info
	Rate         float64 `json:"rate"`         //the proportion of the total
	Change15M    float64 `json:"change15M"`    //15-minute price change
	Change5M     float64 `json:"change5M"`     //5-minute price change
	Change30M    float64 `json:"change30M"`    //30-minute price change
	Change1H     float64 `json:"change1H"`     //1-hours price change
	Change4H     float64 `json:"change4H"`     //4-hours price change
	Change6H     float64 `json:"change6H"`     //6-hours price change
	Change8H     float64 `json:"change8H"`     //8-hours price change
	Change12H    float64 `json:"change12H"`    //12-hours price change
	Change24H    float64 `json:"change24H"`    //24-hours price change
	Change2D     float64 `json:"change2D"`     //2-day price change
	Change3D     float64 `json:"change3D"`     //3-day price change
	Change7D     float64 `json:"change7D"`     //7-day price change
	Turnover24H  float64 `json:"turnover24h"`  //24-hour turnover
	Ts           int     `json:"ts"`
}

type OpenInterestChartV2Response struct {
	Tss        []int64              `json:"tss"`        //Horizontal axis of the chart , millisecond timestamp
	Prices     []float64            `json:"prices"`     //chart vertical axis, coin price
	DataValues map[string][]float64 `json:"dataValues"` // chart value,key is exchangeName,value is exchange holding amount
}

type OpenInterestSymbolChartResponse struct {
	ExchangeName  string   `json:"exchangeName"` // such as `Binance`
	BaseCoin      string   `json:"baseCoin"`     // such as `BTC`
	Symbol        string   `json:"symbol"`       // such as `BTCUSDT`
	ExchangeType  string   `json:"exchangeType"` // exchange type ,`USDT`: usdt base ,`COIN`: coin base
	ContractType  string   `json:"contractType"` // `SWAP`:Perpetual Contract,`FUTURES` : Delivery Contract
	DeliveryType  string   `json:"deliveryType"` // Delivery type ,`PERPETUAL`: Perpetual Contract
	Ts            int64    `json:"ts"`
	UtcIntervals  []string `json:"utcIntervals"`  //symbol has utc intervals
	Utc8Intervals []string `json:"utc8Intervals"` //symbol has utc+8 intervals
	AtUtc         bool     `json:"atUtc"`         //symbol is in utc intervals
	AtUtc8        bool     `json:"atUtc8"`        //symbol is in utc+8 intervals
	CreateAt      string   `json:"createAt"`
	Volume        float64  `json:"volume"`    // coin volume
	CoinCount     float64  `json:"coinCount"` // coin number
	CoinValue     float64  `json:"coinValue"` // coin all value
}

type OpenInterestKlineResponse struct {
	Begin int64   `json:"begin"` //start time
	Open  float64 `json:"open"`  //open price
	Close float64 `json:"close"` //close price
	Low   float64 `json:"low"`   //low price
	High  float64 `json:"high"`  //high price
	O     float64 `json:"o"`     //open price
	C     float64 `json:"c"`     //close price
	L     float64 `json:"l"`     //low price
	H     float64 `json:"h"`     //high price
}

type OpenInterestAggKlineResponse struct {
	Begin int64   `json:"begin"` //start time
	Open  float64 `json:"open"`  //open price
	Close float64 `json:"close"` //close price
	Low   float64 `json:"low"`   //low price
	High  float64 `json:"high"`  //high price
}

type TickersTopOIByExResponse struct {
	Coins     []float64 `json:"coins"`     // coin number for each exchange
	Exchanges []string  `json:"exchanges"` // exchange hold order (desc)
	Oi        []float64 `json:"oi"`        // coin value for each exchange
}

type InstrumentsOiVsMcResponse struct {
	OiVsMar  float64 `json:"oiVsMar"`
	VolVsMar float64 `json:"volVsMar"`
	OiVsVol  float64 `json:"oiVsVol"`
	Ts       int64   `json:"ts"`
}
