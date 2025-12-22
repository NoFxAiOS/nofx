package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
	"strconv"
)

// NetPositions Net long & Net short
func (c *CoinankClient) NetPositions(ctx context.Context, exchange coinank_enum.Exchange,
	symbol string, interval coinank_enum.Interval, endTime int64, size int) ([]NetPositionsResponse, error) {
	paramsMap := make(map[string]string, 5)
	paramsMap["exchange"] = string(exchange)
	paramsMap["symbol"] = symbol
	paramsMap["interval"] = string(interval)
	paramsMap["endTime"] = strconv.FormatInt(endTime, 10)
	if size < 1 {
		size = 10
	}
	paramsMap["size"] = strconv.Itoa(size)
	resp, err := c.Get(ctx, "/api/netPositions/getNetPositions", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]NetPositionsResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

type NetPositionsResponse struct {
	Begin          int64  `json:"begin"` // begin timestamp
	Interval       string `json:"interval"`
	NetLongsHigh   int    `json:"netLongsHigh"`   // net long high
	NetLongsClose  int    `json:"netLongsClose"`  // net long close
	NetLongsLow    int    `json:"netLongsLow"`    // net long close
	NetShortsClose int    `json:"netShortsClose"` // net short close
	NetShortsHigh  int    `json:"netShortsHigh"`  // net short high
	NetShortsLow   int    `json:"netShortsLow"`   // net short low
}
