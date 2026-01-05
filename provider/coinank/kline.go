package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
	"strconv"
)

// Kline get kline data ,startTime and size is optional
func (c *CoinankClient) Kline(ctx context.Context, symbol string, exchange coinank_enum.Exchange,
	startTime int64, endTime int64,
	size int, interval coinank_enum.Interval) ([]KlineResult, error) {
	paramsMap := make(map[string]string, 6)
	paramsMap["symbol"] = symbol
	paramsMap["exchange"] = string(exchange)
	if startTime > 0 {
		paramsMap["startTime"] = strconv.FormatInt(startTime, 10)
	}
	paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	if size <= 0 {
		size = 10
	}
	paramsMap["size"] = strconv.Itoa(size)
	paramsMap["interval"] = string(interval)
	resp, err := c.Get(ctx, "/api/kline/lists", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[][]float64]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	klines := make([]KlineResult, len(result.Data))
	for i, k := range result.Data {
		klines[i].StartTime = int64(k[0] + 0.001)
		klines[i].EndTime = int64(k[1] + 0.001)
		klines[i].Open = k[2]
		klines[i].Close = k[3]
		klines[i].High = k[4]
		klines[i].Low = k[5]
		klines[i].Volume = k[6]
		klines[i].Quantity = k[7]
		klines[i].Count = k[8]
	}
	return klines, nil
}

type KlineResult struct {
	StartTime int64   `json:"startTime"`
	EndTime   int64   `json:"endTime"`
	Open      float64 `json:"open"`
	Close     float64 `json:"close"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Volume    float64 `json:"volume"`
	Quantity  float64 `json:"quantity"`
	Count     float64 `json:"count"`
}
