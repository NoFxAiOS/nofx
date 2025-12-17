package coinank

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
	"testing"
	"time"
)

func TestOpenInterestAll(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.OpenInterestAll(context.TODO(), "BTC")
	if err != nil {
		t.Error(err)
	}
	if resp[0].ExchangeName != "ALL" {
		t.Error("exchange name is empty")
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestOpenInterestChartV2(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.OpenInterestChartV2(context.TODO(), "BTC", coinank_enum.Binance, coinank_enum.Hour1, 10)
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestOpenInterestSymbolChart(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.OpenInterestSymbolChart(context.TODO(), coinank_enum.Binance, "BTCUSDT", coinank_enum.Hour1, time.Now().UnixMilli(), 10)
	if err != nil {
		t.Error(err)
	}
	if resp[0].BaseCoin != "BTC" {
		t.Error("baseCoin is error")
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestOpenInterestKline(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.OpenInterestKline(context.TODO(), coinank_enum.Binance, "BTCUSDT", coinank_enum.Hour1, time.Now().UnixMilli(), 10)
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestOpenInterestAggKline(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.OpenInterestAggKline(context.TODO(), "BTC", coinank_enum.Hour1, time.Now().UnixMilli(), 10)
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestTickersTopOIByEx(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.TickersTopOIByEx(context.TODO(), "BTC")
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}

func TestInstrumentsOiVsMc(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.InstrumentsOiVsMc(context.TODO(), "BTC", coinank_enum.Hour1, time.Now().UnixMilli(), 10)
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}
