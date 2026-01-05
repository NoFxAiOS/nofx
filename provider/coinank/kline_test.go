package coinank

import (
	"context"
	"encoding/json"
	"fmt"
	"nofx/provider/coinank/coinank_enum"
	"testing"
	"time"
)

func TestKline(t *testing.T) {
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.Kline(context.TODO(), "BTCUSDT", coinank_enum.Binance, 0, time.Now().UnixMilli(), 10, coinank_enum.Hour1)
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
	client := NewCoinankClient(coinank_enum.MainUrl, TestApikey)
	resp, err := client.Kline(context.TODO(), "BTCUSDT", coinank_enum.Binance, 0, time.Now().UnixMilli(), 5, coinank_enum.Day1)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== BTCUSDT 日线 K线数据 ===")
	for i, k := range resp {
		startTime := time.UnixMilli(k.StartTime).Format("2006-01-02 15:04:05")
		t.Logf("\n[%d] 时间: %s", i, startTime)
		t.Logf("    Open:     %.2f", k.Open)
		t.Logf("    High:     %.2f", k.High)
		t.Logf("    Low:      %.2f", k.Low)
		t.Logf("    Close:    %.2f", k.Close)
		t.Logf("    Volume:   %.2f (k[6])", k.Volume)
		t.Logf("    Quantity: %.2f (k[7])", k.Quantity)
		t.Logf("    Count:    %.0f (k[8])", k.Count)

		// 计算验证
		if k.Close > 0 {
			calcQuote := k.Volume * k.Close
			t.Logf("    --- 验证 ---")
			t.Logf("    Volume × Close = %.2f", calcQuote)
			t.Logf("    Quantity / Close = %.2f", k.Quantity/k.Close)
		}
	}

	// 打印原始 JSON
	res, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Printf("\n原始 JSON:\n%s\n", res)
}
