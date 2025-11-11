package trader

import (
	"testing"
	"time"
)

func TestPaperTradingBaseURL(t *testing.T) {
	// 创建 paper_trading 配置
	cfg := AutoTraderConfig{
		ID:               "test_paper_trading",
		Name:             "Test Paper Trading",
		Exchange:         "paper_trading",
		BinanceAPIKey:    "test_key",
		BinanceSecretKey: "test_secret",
		ScanInterval:     3 * time.Minute,
		InitialBalance:   1000,
		DeepSeekKey:      "test_deepseek_key",
	}

	// 创建 AutoTrader
	trader, err := NewAutoTrader(cfg, nil, "test_user")
	if err != nil {
		t.Fatalf("创建 trader 失败: %v", err)
	}

	// 验证 trader 的类型是 FuturesTrader
	futuresTrader, ok := trader.trader.(*FuturesTrader)
	if !ok {
		t.Fatal("paper_trading 应该使用 FuturesTrader 实现")
	}

	// 验证 BaseURL 已设置为 Testnet
	expectedURL := "https://testnet.binance.vision"
	if futuresTrader.client.BaseURL != expectedURL {
		t.Errorf("BaseURL 配置错误: 期望 '%s', 实际 '%s'", expectedURL, futuresTrader.client.BaseURL)
	}

	t.Log("✓ Paper Trading BaseURL 验证通过")
	t.Logf("  - BaseURL: %s", futuresTrader.client.BaseURL)
	t.Logf("  - Exchange: %s", trader.exchange)
}
