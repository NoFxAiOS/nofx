package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
)

// GetLastPrice get symbol latest information, param example -> symbol:`BTCUSDT`,exchange:`Binance`,productType:`SWAP`
func (c *CoinankClient) GetLastPrice(ctx context.Context,
	symbol string, exchange coinank_enum.Exchange, productType coinank_enum.ProductType) (*GetLastPriceResponse, error) {
	paramsMap := make(map[string]string, 3)
	paramsMap["symbol"] = symbol
	paramsMap["exchange"] = string(exchange)
	paramsMap["productType"] = string(productType)
	resp, err := c.Get(ctx, "/api/instruments/getLastPrice", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[GetLastPriceResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return &result.Data, nil
}

// GetCoinMarketCap get market cap info for coin ,example -> baseCoin:`BTC`
func (c *CoinankClient) GetCoinMarketCap(ctx context.Context,
	baseCoin string) (*GetCoinMarketResponse, error) {
	paramsMap := make(map[string]string, 1)
	paramsMap["baseCoin"] = baseCoin
	resp, err := c.Get(ctx, "/api/instruments/getCoinMarketCap", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[GetCoinMarketResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return &result.Data, nil
}

type GetLastPriceResponse struct {
	BaseCoin       string  `json:"baseCoin"`       //symbol base_coin
	QuoteCoin      string  `json:"quoteCoin"`      //symbol quote_coin
	Symbol         string  `json:"symbol"`         //symbol name
	ExchangeName   string  `json:"exchangeName"`   //symbol from exchange
	ContractType   string  `json:"contractType"`   //`SWAP`:Perpetual Contracts,`FUTURES`:Delivery Contracts
	LastPrice      float64 `json:"lastPrice"`      //Latest transaction price
	Open24H        float64 `json:"open24h"`        //24-hour opening price
	High24H        float64 `json:"high24h"`        //24-hour highest price
	Low24H         float64 `json:"low24h"`         //24-hour lowest price
	PriceChange24H float64 `json:"priceChange24h"` //24-hour price changes
	VolCcy24H      float64 `json:"volCcy24h"`      //24-hour trading volume
	Turnover24H    float64 `json:"turnover24h"`    //24-hour transaction volume
	TradeTimes     int     `json:"tradeTimes"`     //Number of transactions
	OiUSD          float64 `json:"oiUSD"`          //Open interest(USD)
	OiCcy          float64 `json:"oiCcy"`          //Open interest(ccy)
	OiVol          float64 `json:"oiVol"`          //Open interest(vol)
	FundingRate    float64 `json:"fundingRate"`    //Real-time funding rates
	MarkPrice      float64 `json:"markPrice"`      //mark price
	LiqLong24H     float64 `json:"liqLong24h"`     //24-hour margin call on long positions
	LiqShort24H    float64 `json:"liqShort24h"`    //24-hour margin call on short positions
	Liq24H         float64 `json:"liq24h"`         //24-hour margin call
	OiChg24H       float64 `json:"oiChg24h"`       //24-hour position changes
	BuyTurnover    float64 `json:"buyTurnover"`    //buy turnover
	SellTurnover   float64 `json:"sellTurnover"`   //sell turnover
	Basis          float64 `json:"basis"`
	BasisRate      float64 `json:"basisRate"`
	ExpireAt       int64   `json:"expireAt"` //expire time
	Ts             int     `json:"ts"`
}

type GetCoinMarketResponse struct {
	BaseCoin          string  `json:"baseCoin"`  //coin symbol such as `BTC`
	Price             float64 `json:"price"`     // now price
	MarketCap         float64 `json:"marketCap"` // now market cap
	CirculatingSupply float64 `json:"circulatingSupply"`
	TotalSupply       float64 `json:"totalSupply"`
	SupportContract   bool    `json:"supportContract"`
}
