package hyperliquid

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestGetCandles_BTC(t *testing.T) {
	client := NewClient()

	candles, err := client.GetCandles(context.TODO(), "BTC", "1d", 5)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== BTC daily data (Hyperliquid) ===")
	for i, c := range candles {
		openTime := time.UnixMilli(c.OpenTime).Format("2006-01-02 15:04:05")
		t.Logf("\n[%d] Time: %s", i, openTime)
		t.Logf("    Symbol:     %s", c.Symbol)
		t.Logf("    Interval:   %s", c.Interval)
		t.Logf("    Open:       %s", c.Open)
		t.Logf("    High:       %s", c.High)
		t.Logf("    Low:        %s", c.Low)
		t.Logf("    Close:      %s", c.Close)
		t.Logf("    Volume:     %s", c.Volume)
		t.Logf("    TradeCount: %d", c.TradeCount)
	}

	// Print raw JSON
	res, _ := json.MarshalIndent(candles, "", "  ")
	fmt.Printf("\nRaw JSON:\n%s\n", res)
}

func TestGetCandles_TSLA(t *testing.T) {
	client := NewClient()

	// Test stock perpetual contracts - using xyz dex
	candles, err := client.GetCandles(context.TODO(), "TSLA", "1d", 5)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== TSLA daily data (Hyperliquid xyz dex) ===")
	for i, c := range candles {
		openTime := time.UnixMilli(c.OpenTime).Format("2006-01-02 15:04:05")
		t.Logf("\n[%d] Time: %s", i, openTime)
		t.Logf("    Symbol:     %s", c.Symbol)
		t.Logf("    Interval:   %s", c.Interval)
		t.Logf("    Open:       %s", c.Open)
		t.Logf("    High:       %s", c.High)
		t.Logf("    Low:        %s", c.Low)
		t.Logf("    Close:      %s", c.Close)
		t.Logf("    Volume:     %s", c.Volume)
		t.Logf("    TradeCount: %d", c.TradeCount)
	}

	// Print raw JSON
	res, _ := json.MarshalIndent(candles, "", "  ")
	fmt.Printf("\nRaw JSON:\n%s\n", res)
}

func TestGetCandles_StockPerps(t *testing.T) {
	client := NewClient()

	// Test multiple stock perpetual contracts (xyz dex)
	symbols := []string{"TSLA", "NVDA", "AAPL", "MSFT"}

	for _, symbol := range symbols {
		t.Logf("\n=== %s daily data ===", symbol)
		candles, err := client.GetCandles(context.TODO(), symbol, "1d", 3)
		if err != nil {
			t.Errorf("%s fetch failed: %v", symbol, err)
			continue
		}

		if len(candles) == 0 {
			t.Logf("%s: no data", symbol)
			continue
		}

		latest := candles[len(candles)-1]
		openTime := time.UnixMilli(latest.OpenTime).Format("2006-01-02")
		t.Logf("%s latest: %s Open=%s High=%s Low=%s Close=%s Vol=%s",
			symbol, openTime, latest.Open, latest.High, latest.Low, latest.Close, latest.Volume)
	}
}

func TestGetAllMids(t *testing.T) {
	client := NewClient()

	mids, err := client.GetAllMids(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== Crypto asset mid prices (default dex) ===")

	// Show some major crypto assets
	cryptoAssets := []string{"BTC", "ETH", "SOL", "DOGE", "XRP"}
	for _, asset := range cryptoAssets {
		if mid, ok := mids[asset]; ok {
			t.Logf("%s: %s", asset, mid)
		} else {
			t.Logf("%s: not found", asset)
		}
	}

	t.Logf("\nTotal %d crypto trading pairs", len(mids))
}

func TestGetAllMidsXYZ(t *testing.T) {
	client := NewClient()

	mids, err := client.GetAllMidsXYZ(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== xyz dex asset mid prices (stocks, forex, commodities) ===")

	// Show all xyz dex assets
	for symbol, mid := range mids {
		t.Logf("%s: %s", symbol, mid)
	}

	t.Logf("\nTotal %d xyz dex trading pairs", len(mids))
}

func TestGetMeta(t *testing.T) {
	client := NewClient()

	meta, err := client.GetMeta(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== Asset metadata ===")
	t.Logf("Total %d assets", len(meta.Universe))

	// Show stock perpetual contracts
	t.Log("\nStock perpetual contracts:")
	for _, asset := range meta.Universe {
		if IsStockPerp(asset.Name) {
			t.Logf("  %s: szDecimals=%d, maxLeverage=%d", asset.Name, asset.SzDecimals, asset.MaxLeverage)
		}
	}
}

func TestNormalizeCoin(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTC", "BTC"},
		{"BTCUSDT", "BTC"},
		{"BTCUSD", "BTC"},
		{"TSLA-USDC", "TSLA"},
		{"TESLA-USDC", "TSLA"},
		{"SMSN-USDC", "SMSN"},
		{"SAMSUNG-USDC", "SMSN"},
		{"xyz:SMSN", "SMSN"},
		{"AAPL-USDC", "AAPL"},
		{"ETH", "ETH"},
		{"ETHUSDT", "ETH"},
	}

	for _, tt := range tests {
		result := NormalizeCoin(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeCoin(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestIsStockPerp(t *testing.T) {
	tests := []struct {
		symbol   string
		expected bool
	}{
		{"TSLA", true},
		{"TSLA-USDC", true},
		{"xyz:TSLA", true},
		{"AAPL", true},
		{"BTC", false},
		{"BTCUSDT", false},
		{"ETH", false},
	}

	for _, tt := range tests {
		result := IsStockPerp(tt.symbol)
		if result != tt.expected {
			t.Errorf("IsStockPerp(%s) = %v, expected %v", tt.symbol, result, tt.expected)
		}
	}
}

func TestFormatCoinForAPI(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTC", "BTC"},
		{"BTCUSDT", "BTC"},
		{"ETH", "ETH"},
		{"TSLA", "xyz:TSLA"},
		{"TSLA-USDC", "xyz:TSLA"},
		{"TESLA-USDC", "xyz:TSLA"},
		{"SMSN-USDC", "xyz:SMSN"},
		{"SAMSUNG-USDC", "xyz:SMSN"},
		{"xyz:SMSN", "xyz:SMSN"},
		{"xyz:TSLA", "xyz:TSLA"},
		{"NVDA", "xyz:NVDA"},
		{"GOLD", "xyz:GOLD"},
		{"EUR", "xyz:EUR"},
	}

	for _, tt := range tests {
		result := FormatCoinForAPI(tt.input)
		if result != tt.expected {
			t.Errorf("FormatCoinForAPI(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}
