package coinank_api

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestKlineDaily(t *testing.T) {
	resp, err := Kline(context.TODO(), "BTCUSDT", coinank_enum.Binance, time.Now().UnixMilli(), coinank_enum.To, 5, coinank_enum.Day1)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== BTCUSDT 日线 K线数据 (coinank_api 免费接口) ===")
	for i, k := range resp {
		startTime := time.UnixMilli(k.StartTime).Format("2006-01-02 15:04:05")
		t.Logf("\n[%d] 时间: %s", i, startTime)
		t.Logf("    Open:     %.2f", k.Open)
		t.Logf("    High:     %.2f", k.High)
		t.Logf("    Low:      %.2f", k.Low)
		t.Logf("    Close:    %.2f", k.Close)
		t.Logf("    Volume:   %.4f (k[6])", k.Volume)
		t.Logf("    Quantity: %.4f (k[7])", k.Quantity)
		t.Logf("    Count:    %.0f (k[8])", k.Count)

		// 计算验证
		if k.Close > 0 && k.Volume > 0 {
			t.Logf("    --- 验证 ---")
			t.Logf("    Volume × Close = %.2f", k.Volume*k.Close)
			t.Logf("    Quantity / Close = %.4f", k.Quantity/k.Close)
		}
	}

	// 打印原始 JSON
	res, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Printf("\n原始 JSON:\n%s\n", res)
}
