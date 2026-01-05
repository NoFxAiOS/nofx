package coinank_api

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank"
	"nofx/provider/coinank/coinank_enum"
)

// BaseCoinSymbols get base coin from coinank free open api , all params is optional
func BaseCoinSymbols(ctx context.Context, exchangeName coinank_enum.Exchange, symbol string, baseCoin string) ([]BaseCoinResponse, error) {
	paramsMap := make(map[string]string, 3)
	if symbol != "" {
		paramsMap["symbol"] = symbol
	}
	if baseCoin != "" {
		paramsMap["baseCoin"] = baseCoin
	}
	if exchangeName != "" {
		paramsMap["exchangeName"] = string(exchangeName)
	}
	resp, err := get(ctx, "/api/baseCoin/symbols/open", paramsMap)
	if err != nil {
		return nil, err
	}
	var result coinank.CoinankResponse[[]BaseCoinResponse]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, coinank.HttpError
	}
	return result.Data, nil
}

type BaseCoinResponse struct {
	Symbol         string  `json:"symbol"`
	BaseCoin       string  `json:"baseCoin"`
	ExchangeName   string  `json:"exchangeName"`
	ProductType    string  `json:"productType"`
	SymbolType     string  `json:"symbolType"`
	PricePrecision string  `json:"pricePrecision"`
	DeliveryType   string  `json:"deliveryType"`
	ExpireAt       int     `json:"expireAt"`
	UpdateAt       int     `json:"updateAt"`
	Hot            bool    `json:"hot"`
	Price          float64 `json:"price"`
	PriceChangeH24 float64 `json:"priceChangeH24"`
}
