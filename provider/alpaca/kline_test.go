package alpaca

import (
	"context"
	"fmt"
	"testing"
)

func TestGetBars(t *testing.T) {
	client := NewClient()

	resp, err := client.GetBars(context.TODO(), "AAPL", "1Day", 5)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== AAPL 日线数据 (Alpaca IEX feed) ===")
	for i, bar := range resp {
		t.Logf("\n[%d] 时间: %s", i, bar.Timestamp.Format("2006-01-02 15:04:05"))
		t.Logf("    Open:       %.2f", bar.Open)
		t.Logf("    High:       %.2f", bar.High)
		t.Logf("    Low:        %.2f", bar.Low)
		t.Logf("    Close:      %.2f", bar.Close)
		t.Logf("    Volume:     %d (股数)", bar.Volume)
		t.Logf("    TradeCount: %d (成交笔数)", bar.TradeCount)
		t.Logf("    VWAP:       %.2f (成交量加权平均价)", bar.VWAP)

		// 计算成交额
		quoteVolume := float64(bar.Volume) * bar.Close
		t.Logf("    成交额:     %.2f USD (Volume × Close)", quoteVolume)
	}

	fmt.Printf("\n⚠️ 注意：IEX feed 只包含 IEX 交易所的数据，不是完整市场数据\n")
	fmt.Printf("完整市场数据需要使用 SIP feed（付费）\n")
}
