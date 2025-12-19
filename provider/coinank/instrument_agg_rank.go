package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
	"strconv"
)

// VisualScreener Visual Screener
func (c *CoinankClient) VisualScreener(ctx context.Context, interval coinank_enum.Interval) ([]VisualScreenerResponse, error) {
	paramsMap := make(map[string]string, 1)
	paramsMap["interval"] = string(interval)
	resp, err := c.Get(ctx, "/api/instruments/visualScreener", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]VisualScreenerResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return result.Data, nil
}

// OiRank Open Interest Ranking
func (c *CoinankClient) OiRank(ctx context.Context, sortBy coinank_enum.InstrumentAggSortBy,
	sortType coinank_enum.SortType, page int, size int) ([]OiRankResponse, error) {
	resp, err := c.Get(ctx, "/api/instruments/oiRank", c.rankParam(sortBy, sortType, page, size))
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[CoinankResponse[PageData[OiRankResponse]]]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	if !result.Data.Success {
		return nil, HttpError
	}
	return result.Data.Data.List, nil
}

// LongShortRank longShortRatio Ranking
func (c *CoinankClient) LongShortRank(ctx context.Context, sortBy coinank_enum.InstrumentAggSortBy,
	sortType coinank_enum.SortType, page int, size int) ([]LongShortRankResponse, error) {
	resp, err := c.Get(ctx, "/api/instruments/longShortRank", c.rankParam(sortBy, sortType, page, size))
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[CoinankResponse[PageData[LongShortRankResponse]]]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	if !result.Data.Success {
		return nil, HttpError
	}
	return result.Data.Data.List, nil
}

// LiquidationRank Liquidation Ranking
func (c *CoinankClient) LiquidationRank(ctx context.Context, sortBy coinank_enum.InstrumentAggSortBy,
	sortType coinank_enum.SortType, page int, size int) ([]LiquidationRankResponse, error) {
	resp, err := c.Get(ctx, "/api/instruments/liquidationRank", c.rankParam(sortBy, sortType, page, size))
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[CoinankResponse[PageData[LiquidationRankResponse]]]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	if !result.Data.Success {
		return nil, HttpError
	}
	return result.Data.Data.List, nil
}

// PriceRank PriceChg Ranking
func (c *CoinankClient) PriceRank(ctx context.Context, sortBy coinank_enum.InstrumentAggSortBy,
	sortType coinank_enum.SortType, page int, size int) ([]PriceRankResponse, error) {
	resp, err := c.Get(ctx, "/api/instruments/priceRank", c.rankParam(sortBy, sortType, page, size))
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[CoinankResponse[PageData[PriceRankResponse]]]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	if !result.Data.Success {
		return nil, HttpError
	}
	return result.Data.Data.List, nil
}

// VolumeRank VolumeChg Ranking
func (c *CoinankClient) VolumeRank(ctx context.Context, sortBy coinank_enum.InstrumentAggSortBy,
	sortType coinank_enum.SortType, page int, size int) ([]VolumeRankResponse, error) {
	resp, err := c.Get(ctx, "/api/instruments/volumeRank", c.rankParam(sortBy, sortType, page, size))
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[CoinankResponse[PageData[VolumeRankResponse]]]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	if !result.Data.Success {
		return nil, HttpError
	}
	return result.Data.Data.List, nil
}

func (c *CoinankClient) rankParam(sortBy coinank_enum.InstrumentAggSortBy,
	sortType coinank_enum.SortType, page int, size int) map[string]string {
	paramsMap := make(map[string]string, 4)
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if sortBy == "" {
		sortBy = coinank_enum.OpenInterest
	}
	if sortType == "" {
		sortType = coinank_enum.Desc
	}
	paramsMap["page"] = strconv.Itoa(page)
	paramsMap["size"] = strconv.Itoa(size)
	paramsMap["sortBy"] = string(sortBy)
	paramsMap["sortType"] = string(sortType)
	return paramsMap
}

type VisualScreenerResponse struct {
	BaseCoin string  `json:"baseCoin"`
	PriceChg float64 `json:"priceChg"`
	OiChg    float64 `json:"oiChg"`
	VoChg    float64 `json:"voChg"`
}

type LongShortRankResponse struct {
	BaseCoin          string  `json:"baseCoin"`
	CoinImage         string  `json:"coinImage"`
	Price             float64 `json:"price"`
	LongShortPerson   float64 `json:"longShortPerson"`
	LsPersonChg5M     float64 `json:"lsPersonChg5m"`
	LsPersonChg15M    float64 `json:"lsPersonChg15m"`
	LsPersonChg30M    float64 `json:"lsPersonChg30m"`
	LsPersonChg1H     float64 `json:"lsPersonChg1h"`
	LsPersonChg4H     float64 `json:"lsPersonChg4h"`
	CirculatingSupply int     `json:"circulatingSupply"`
	Symbol            string  `json:"symbol"`
	ExchangeName      string  `json:"exchangeName"`
	SupportContract   bool    `json:"supportContract"`
}

type OiRankResponse struct {
	BaseCoin          string  `json:"baseCoin"`
	CoinImage         string  `json:"coinImage"`
	Price             float64 `json:"price"`
	OpenInterest      float64 `json:"openInterest"`
	OpenInterestChM5  float64 `json:"openInterestChM5"`
	OpenInterestChM15 float64 `json:"openInterestChM15"`
	OpenInterestChM30 float64 `json:"openInterestChM30"`
	OpenInterestCh1   float64 `json:"openInterestCh1"`
	OpenInterestCh4   float64 `json:"openInterestCh4"`
	OpenInterestCh24  float64 `json:"openInterestCh24"`
	OpenInterestCh2D  float64 `json:"openInterestCh2D"`
	OpenInterestCh3D  float64 `json:"openInterestCh3D"`
	OpenInterestCh7D  float64 `json:"openInterestCh7D"`
	CirculatingSupply int     `json:"circulatingSupply"`
	Symbol            string  `json:"symbol"`
	ExchangeName      string  `json:"exchangeName"`
	SupportContract   bool    `json:"supportContract"`
	Follow            bool    `json:"follow"`
}

type LiquidationRankResponse struct {
	BaseCoin            string  `json:"baseCoin"`
	CoinImage           string  `json:"coinImage"`
	Price               float64 `json:"price"`
	PriceChangeH24      float64 `json:"priceChangeH24"`
	LiquidationH1       float64 `json:"liquidationH1"`
	LiquidationH1Long   float64 `json:"liquidationH1Long"`
	LiquidationH1Short  float64 `json:"liquidationH1Short"`
	LiquidationH4       float64 `json:"liquidationH4"`
	LiquidationH4Long   float64 `json:"liquidationH4Long"`
	LiquidationH4Short  float64 `json:"liquidationH4Short"`
	LiquidationH12      float64 `json:"liquidationH12"`
	LiquidationH12Long  float64 `json:"liquidationH12Long"`
	LiquidationH12Short float64 `json:"liquidationH12Short"`
	LiquidationH24      float64 `json:"liquidationH24"`
	LiquidationH24Long  float64 `json:"liquidationH24Long"`
	LiquidationH24Short float64 `json:"liquidationH24Short"`
	CirculatingSupply   int     `json:"circulatingSupply"`
	SupportContract     bool    `json:"supportContract"`
}

type PriceRankResponse struct {
	BaseCoin          string  `json:"baseCoin"`
	CoinImage         string  `json:"coinImage"`
	Price             float64 `json:"price"`
	PriceChangeH24    float64 `json:"priceChangeH24"`
	PriceChangeM5     float64 `json:"priceChangeM5"`
	PriceChangeM15    float64 `json:"priceChangeM15"`
	PriceChangeM30    float64 `json:"priceChangeM30"`
	PriceChangeH1     float64 `json:"priceChangeH1"`
	PriceChangeH2     float64 `json:"priceChangeH2"`
	PriceChangeH4     float64 `json:"priceChangeH4"`
	PriceChangeH6     float64 `json:"priceChangeH6"`
	PriceChangeH8     float64 `json:"priceChangeH8"`
	PriceChangeH12    float64 `json:"priceChangeH12"`
	CirculatingSupply int     `json:"circulatingSupply"`
	Symbol            string  `json:"symbol"`
	ExchangeName      string  `json:"exchangeName"`
	SupportContract   bool    `json:"supportContract"`
}

type VolumeRankResponse struct {
	BaseCoin          string  `json:"baseCoin"`
	CoinImage         string  `json:"coinImage"`
	Price             float64 `json:"price"`
	Turnover24H       float64 `json:"turnover24h"`
	TurnoverChg24H    float64 `json:"turnoverChg24h"`
	TurnoverChg4H     float64 `json:"turnoverChg4h"`
	TurnoverChg1H     float64 `json:"turnoverChg1h"`
	TurnoverChg30M    float64 `json:"turnoverChg30m"`
	TurnoverChg15M    float64 `json:"turnoverChg15m"`
	CirculatingSupply int     `json:"circulatingSupply"`
	Symbol            string  `json:"symbol"`
	ExchangeName      string  `json:"exchangeName"`
	SupportContract   bool    `json:"supportContract"`
}
