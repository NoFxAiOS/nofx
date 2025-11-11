package config

import (
	"os"
	"testing"
)

func TestPaperTradingExchangeInit(t *testing.T) {
	// 创建临时数据库
	tempDB := "test_paper_trading.db"
	defer os.Remove(tempDB)

	// 初始化数据库
	db, err := NewDatabase(tempDB)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}
	defer db.Close()

	// 查询 paper_trading 交易所
	exchanges, err := db.GetExchanges("default")
	if err != nil {
		t.Fatalf("查询交易所失败: %v", err)
	}

	// 验证 paper_trading 交易所存在
	var paperTrading *ExchangeConfig
	for _, exchange := range exchanges {
		if exchange.ID == "paper_trading" {
			paperTrading = exchange
			break
		}
	}

	if paperTrading == nil {
		t.Fatal("paper_trading 交易所记录不存在")
	}

	// 验证字段值
	if paperTrading.Type != "paper_trading" {
		t.Errorf("Type 字段错误: 期望 'paper_trading', 实际 '%s'", paperTrading.Type)
	}

	if paperTrading.Name != "Paper Trading (Binance Testnet)" {
		t.Errorf("Name 字段错误: 期望 'Paper Trading (Binance Testnet)', 实际 '%s'", paperTrading.Name)
	}

	t.Log("✓ Paper Trading 交易所记录验证通过")
	t.Logf("  - ID: %s", paperTrading.ID)
	t.Logf("  - Name: %s", paperTrading.Name)
	t.Logf("  - Type: %s", paperTrading.Type)
}
