package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
	"strconv"
)

// LiquidationExchangeStatistics Current Exchange Liquidation Statistics
func (c *CoinankClient) LiquidationExchangeStatistics(ctx context.Context, baseCoin string) (*LiquidationExchangeStatisticsResponse, error) {
	paramsMap := make(map[string]string, 3)
	paramsMap["baseCoin"] = baseCoin
	resp, err := c.Get(ctx, "/api/liquidation/allExchange/intervals", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[LiquidationExchangeStatisticsResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return &result.Data, nil
}

// LiquidationCoinAggHistory coin liquidation aggregated history
func (c *CoinankClient) LiquidationCoinAggHistory(ctx context.Context, baseCoin string, interval coinank_enum.Interval,
	endTime int64, size int) ([]LiquidationStatistic, error) {
	paramsMap := make(map[string]string, 4)
	paramsMap["baseCoin"] = baseCoin
	paramsMap["interval"] = string(interval)
	paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	paramsMap["size"] = strconv.Itoa(size)
	resp, err := c.Get(ctx, "/api/liquidation/aggregated-history", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]LiquidationStatistic]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

// LiquidationHistory Trading Pair Liquidation Statistics
func (c *CoinankClient) LiquidationHistory(ctx context.Context, exchange coinank_enum.Exchange, symbol string,
	interval coinank_enum.Interval, endTime int64, size int) ([]LiquidationSymbol, error) {
	paramsMap := make(map[string]string, 5)
	paramsMap["exchange"] = string(exchange)
	paramsMap["symbol"] = symbol
	paramsMap["interval"] = string(interval)
	paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	paramsMap["size"] = strconv.Itoa(size)
	resp, err := c.Get(ctx, "/api/liquidation/history", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]LiquidationSymbol]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

// LiquidationOrders Liquidation order, side:`long` or `short`,amount: order amount greater than amount
func (c *CoinankClient) LiquidationOrders(ctx context.Context, baseCoin string, exchange coinank_enum.Exchange,
	side string, amount int, endTime int64) ([]LiquidationOrdersResponse, error) {
	paramsMap := make(map[string]string, 5)
	if baseCoin != "" {
		paramsMap["baseCoin"] = baseCoin
	}
	if exchange != "" {
		paramsMap["exchange"] = string(exchange)
	}
	if side != "" {
		paramsMap["side"] = string(side)
	}
	if amount != 0 {
		paramsMap["amount"] = strconv.Itoa(amount)
	}
	if endTime != 0 {
		paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	}
	resp, err := c.Get(ctx, "/api/liquidation/orders", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]LiquidationOrdersResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

type LiquidationOrdersResponse struct {
	ExchangeName  string  `json:"exchangeName"`
	BaseCoin      string  `json:"baseCoin"`
	ContractCode  string  `json:"contractCode"` //contract code
	PosSide       string  `json:"posSide"`      // `long`: long ,`short`:short
	Amount        float64 `json:"amount"`       //liquidation amount
	Price         float64 `json:"price"`        //liquidation price
	AvgPrice      float64 `json:"avgPrice"`
	TradeTurnover float64 `json:"tradeTurnover"` // liquidation turnover
	Ts            int64   `json:"ts"`
}

type LiquidationSymbol struct {
	Symbol        string  `json:"symbol"`
	ExchangeName  string  `json:"exchangeName"`
	Ts            int64   `json:"ts"`            // timestamp
	LongTurnover  float64 `json:"longTurnover"`  //long turnover
	ShortTurnover float64 `json:"shortTurnover"` //short turnover
	ShortAmount   float64 `json:"shortAmount"`   //short amount
	LongAmount    float64 `json:"longAmount"`    // long amount
}

type LiquidationStatistic struct {
	All struct {
		LongTurnover  float64 `json:"longTurnover"`
		ShortTurnover float64 `json:"shortTurnover"`
		ShortAmount   float64 `json:"shortAmount"`
		LongAmount    float64 `json:"longAmount"`
	} `json:"all"` //coin liquidation aggregated with all exchanges
	Ts int64 `json:"ts"` // timestamp
}

type LiquidationExchangeStatisticsResponse struct {
	TopOrder struct {
		Symbol        string  `json:"symbol"`
		PosSide       string  `json:"posSide"`       //side
		ExchangeName  string  `json:"exchangeName"`  //exchangeName
		TradeTurnover float64 `json:"tradeTurnover"` //turnover
		BaseCoin      string  `json:"baseCoin"`
		Ts            int64   `json:"ts"`
	} `json:"topOrder"` // 24 hour liquidation top order
	Total int      `json:"total"` // 24 hour total liquidation number
	Two4H struct { // 24 hour liquidation data
		BaseCoin      string  `json:"baseCoin"`
		TotalTurnover float64 `json:"totalTurnover"`
		LongTurnover  float64 `json:"longTurnover"`
		ShortTurnover float64 `json:"shortTurnover"`
		Percentage    float64 `json:"percentage"`
		LongRatio     float64 `json:"longRatio"`
		ShortRatio    float64 `json:"shortRatio"`
		Interval      string  `json:"interval"`
	} `json:"24h"`
	OneH struct { // 1 hour liquidation data
		BaseCoin      string  `json:"baseCoin"`
		TotalTurnover float64 `json:"totalTurnover"`
		LongTurnover  float64 `json:"longTurnover"`
		ShortTurnover float64 `json:"shortTurnover"`
		Percentage    float64 `json:"percentage"`
		LongRatio     float64 `json:"longRatio"`
		ShortRatio    float64 `json:"shortRatio"`
		Interval      string  `json:"interval"`
	} `json:"1h"`
}
