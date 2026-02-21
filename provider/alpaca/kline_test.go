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

func TestConvertSymbolToAlpacaFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTCUSDT", "BTC/USD"},
		{"ETHUSDT", "ETH/USD"},
		{"BTC/USD", "BTC/USD"},
		{"SOLUSDT", "SOL/USD"},
		{"XRPUSDT", "XRP/USD"},
		{"BNBUSDT", "BNB/USD"},
		{"ADAUSDT", "ADA/USD"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ConvertSymbolToAlpacaFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertSymbolToAlpacaFormat(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapTimeframeForAsset(t *testing.T) {
	tests := []struct {
		interval  string
		assetType string
		expected  string
	}{
		{"1m", "crypto", "1Min"},
		{"5m", "crypto", "5Min"},
		{"15m", "crypto", "15Min"},
		{"30m", "crypto", "30Min"},
		{"1h", "crypto", "1Hour"},
		{"4h", "crypto", "4Hour"},
		{"1d", "crypto", "1Day"},
		{"1w", "crypto", "1Week"},
		{"1m", "stock", "1Min"},
		{"5m", "stock", "5Min"},
		{"1h", "stock", "1Hour"},
		{"4h", "stock", "4Hour"},
		{"1d", "stock", "1Day"},
	}

	for _, tt := range tests {
		t.Run(tt.interval+"_"+tt.assetType, func(t *testing.T) {
			result := MapTimeframeForAsset(tt.interval, tt.assetType)
			if result != tt.expected {
				t.Errorf("MapTimeframeForAsset(%s, %s) = %s; want %s", tt.interval, tt.assetType, result, tt.expected)
			}
		})
	}
}

func TestMapTimeframe(t *testing.T) {
	// MapTimeframe should default to crypto
	result := MapTimeframe("1h")
	if result != "1Hour" {
		t.Errorf("MapTimeframe(1h) = %s; want 1Hour", result)
	}

	result = MapTimeframe("5m")
	if result != "5Min" {
		t.Errorf("MapTimeframe(5m) = %s; want 5Min", result)
	}
}
