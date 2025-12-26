package coinank_api

import (
	"context"
	"encoding/json"
	"nofx/provider/coinank/coinank_enum"
	"testing"
	"time"
)

func TestKline(t *testing.T) {
	resp, err := Kline(context.TODO(), "BTCUSDT", coinank_enum.Binance, time.Now().UnixMilli(), coinank_enum.To, 10, coinank_enum.Hour1)
	if err != nil {
		t.Error(err)
	}
	res, err := json.Marshal(resp)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%s", res)
}
