package coinank_api

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBaseCoinSymbolsNoArgs(t *testing.T) {
	resp, err := BaseCoinSymbols(context.TODO(), "", "", "")
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestBaseCoinSymbolsBTC(t *testing.T) {
	resp, err := BaseCoinSymbols(context.TODO(), "", "", "BTC")
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestBaseCoinSymbolsBTCUSDT(t *testing.T) {
	resp, err := BaseCoinSymbols(context.TODO(), "", "BTCUSDT", "")
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}
