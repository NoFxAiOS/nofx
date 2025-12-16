package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
)

// ListCoin list all support coin from coinank, response is list of coin symbol
func (c *CoinankClient) ListCoin(ctx context.Context, productType coinank_enum.ProductType) (*[]string, error) {
	paramsMap := make(map[string]string, 1)
	paramsMap["productType"] = string(productType)
	resp, err := c.Get(ctx, "/api/baseCoin/list", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]string]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return &result.Data, nil
}

// ListSymbols list all support symbols from coinank
func (c *CoinankClient) ListSymbols(ctx context.Context, exchange coinank_enum.Exchange, productType coinank_enum.ProductType) (*[]SymbolResp, error) {
	paramsMap := make(map[string]string, 2)
	paramsMap["exchange"] = string(exchange)
	paramsMap["productType"] = string(productType)
	resp, err := c.Get(ctx, "/api/baseCoin/symbols", paramsMap)
	if err != nil {
		return nil, err
	}
	var result CoinankResponse[[]SymbolResp]
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(resp), &result)
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, HttpError
	}
	return &result.Data, nil
}

type SymbolResp struct {
	Symbol       string `json:"symbol"`       // symbol,such as:`BTCUSDT`
	BaseCoin     string `json:"baseCoin"`     // baseCoin from symbol,such as `BTC`
	ExchangeName string `json:"exchangeName"` // symbol source ,such as:`Binance`
	ExpireAt     int    `json:"expireAt"`
	UpdateAt     int    `json:"updateAt"`
}
