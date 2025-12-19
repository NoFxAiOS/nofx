package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
	"testing"
)

func TestVisualScreener(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.VisualScreener(context.TODO(), coinank_enum.Minute15)
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestOiRank(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.OiRank(context.TODO(), coinank_enum.OpenInterest, coinank_enum.Desc, 1, 10)
	if err != nil {
		t.Error(err)
	}
	if resp[0].BaseCoin != "BTC" {
		t.Error("oi first not BTC")
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestLongShortRank(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.LongShortRank(context.TODO(), coinank_enum.LongShortRatio, coinank_enum.Desc, 1, 10)
	if err != nil {
		t.Error(err)
	}
	if resp[0].BaseCoin == "" {
		t.Error("baseCoin is empty")
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestLiquidationRank(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.LiquidationRank(context.TODO(), coinank_enum.LiquidationH1, coinank_enum.Desc, 1, 10)
	if err != nil {
		t.Error(err)
	}
	if resp[0].BaseCoin == "" {
		t.Error("baseCoin is empty")
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestPriceRank(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.PriceRank(context.TODO(), coinank_enum.Price, coinank_enum.Desc, 1, 10)
	if err != nil {
		t.Error(err)
	}
	if resp[0].BaseCoin == "" {
		t.Error("baseCoin is empty")
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestVolumeRank(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.VolumeRank(context.TODO(), coinank_enum.Turnover24h, coinank_enum.Desc, 1, 10)
	if err != nil {
		t.Error(err)
	}
	if resp[0].BaseCoin == "" {
		t.Error("baseCoin is empty")
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}
