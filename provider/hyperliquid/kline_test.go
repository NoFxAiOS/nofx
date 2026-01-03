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

	t.Log("=== BTC 日线数据 (Hyperliquid) ===")
	for i, c := range candles {
		openTime := time.UnixMilli(c.OpenTime).Format("2006-01-02 15:04:05")
		t.Logf("\n[%d] 时间: %s", i, openTime)
		t.Logf("    Symbol:     %s", c.Symbol)
		t.Logf("    Interval:   %s", c.Interval)
		t.Logf("    Open:       %s", c.Open)
		t.Logf("    High:       %s", c.High)
		t.Logf("    Low:        %s", c.Low)
		t.Logf("    Close:      %s", c.Close)
		t.Logf("    Volume:     %s", c.Volume)
		t.Logf("    TradeCount: %d", c.TradeCount)
	}

	// 打印原始 JSON
	res, _ := json.MarshalIndent(candles, "", "  ")
	fmt.Printf("\n原始 JSON:\n%s\n", res)
}

func TestGetCandles_TSLA(t *testing.T) {
	client := NewClient()

	// 测试股票永续合约 - 使用 xyz dex
	candles, err := client.GetCandles(context.TODO(), "TSLA", "1d", 5)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== TSLA 日线数据 (Hyperliquid xyz dex) ===")
	for i, c := range candles {
		openTime := time.UnixMilli(c.OpenTime).Format("2006-01-02 15:04:05")
		t.Logf("\n[%d] 时间: %s", i, openTime)
		t.Logf("    Symbol:     %s", c.Symbol)
		t.Logf("    Interval:   %s", c.Interval)
		t.Logf("    Open:       %s", c.Open)
		t.Logf("    High:       %s", c.High)
		t.Logf("    Low:        %s", c.Low)
		t.Logf("    Close:      %s", c.Close)
		t.Logf("    Volume:     %s", c.Volume)
		t.Logf("    TradeCount: %d", c.TradeCount)
	}

	// 打印原始 JSON
	res, _ := json.MarshalIndent(candles, "", "  ")
	fmt.Printf("\n原始 JSON:\n%s\n", res)
}

func TestGetCandles_StockPerps(t *testing.T) {
	client := NewClient()

	// 测试多个股票永续合约 (xyz dex)
	symbols := []string{"TSLA", "NVDA", "AAPL", "MSFT"}

	for _, symbol := range symbols {
		t.Logf("\n=== %s 日线数据 ===", symbol)
		candles, err := client.GetCandles(context.TODO(), symbol, "1d", 3)
		if err != nil {
			t.Errorf("%s 获取失败: %v", symbol, err)
			continue
		}

		if len(candles) == 0 {
			t.Logf("%s: 无数据", symbol)
			continue
		}

		latest := candles[len(candles)-1]
		openTime := time.UnixMilli(latest.OpenTime).Format("2006-01-02")
		t.Logf("%s 最新: %s Open=%s High=%s Low=%s Close=%s Vol=%s",
			symbol, openTime, latest.Open, latest.High, latest.Low, latest.Close, latest.Volume)
	}
}

func TestGetAllMids(t *testing.T) {
	client := NewClient()

	mids, err := client.GetAllMids(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== 加密货币资产中间价 (默认 dex) ===")

	// 显示一些主要加密货币资产
	cryptoAssets := []string{"BTC", "ETH", "SOL", "DOGE", "XRP"}
	for _, asset := range cryptoAssets {
		if mid, ok := mids[asset]; ok {
			t.Logf("%s: %s", asset, mid)
		} else {
			t.Logf("%s: 不存在", asset)
		}
	}

	t.Logf("\n总共 %d 个加密货币交易对", len(mids))
}

func TestGetAllMidsXYZ(t *testing.T) {
	client := NewClient()

	mids, err := client.GetAllMidsXYZ(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== xyz dex 资产中间价 (股票、外汇、大宗商品) ===")

	// 显示所有 xyz dex 资产
	for symbol, mid := range mids {
		t.Logf("%s: %s", symbol, mid)
	}

	t.Logf("\n总共 %d 个 xyz dex 交易对", len(mids))
}

func TestGetMeta(t *testing.T) {
	client := NewClient()

	meta, err := client.GetMeta(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	t.Log("=== 资产元数据 ===")
	t.Logf("总共 %d 个资产", len(meta.Universe))

	// 显示股票永续合约
	t.Log("\n股票永续合约:")
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
