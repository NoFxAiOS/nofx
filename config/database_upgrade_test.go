package config

import (
	"os"
	"testing"
)

// TestDatabaseUpgrade 测试从旧版本数据库升级到新版本
// 模拟场景：旧数据库只有3个交易所，升级后应该自动添加 paper_trading
func TestDatabaseUpgrade(t *testing.T) {
	// 创建临时数据库
	tempDB := "test_upgrade.db"
	defer os.Remove(tempDB)

	// 步骤1: 创建"旧版本"数据库（只有3个交易所）
	db1, err := NewDatabase(tempDB)
	if err != nil {
		t.Fatalf("创建数据库失败: %v", err)
	}

	// 手动删除 paper_trading（模拟旧数据库）
	_, err = db1.db.Exec(`DELETE FROM exchanges WHERE id = 'paper_trading'`)
	if err != nil {
		t.Fatalf("删除 paper_trading 失败: %v", err)
	}

	// 验证只有3个交易所
	exchanges1, err := db1.GetExchanges("default")
	if err != nil {
		t.Fatalf("查询交易所失败: %v", err)
	}

	if len(exchanges1) != 3 {
		t.Fatalf("期望3个交易所，实际 %d 个", len(exchanges1))
	}

	hasPaperTrading := false
	for _, ex := range exchanges1 {
		if ex.ID == "paper_trading" {
			hasPaperTrading = true
			break
		}
	}
	if hasPaperTrading {
		t.Fatal("旧数据库不应该有 paper_trading")
	}

	t.Log("✓ 旧数据库验证通过（3个交易所）")

	// 关闭数据库
	db1.Close()

	// 步骤2: 重新打开数据库（模拟升级）
	db2, err := NewDatabase(tempDB)
	if err != nil {
		t.Fatalf("重新打开数据库失败: %v", err)
	}
	defer db2.Close()

	// 步骤3: 验证 paper_trading 已自动添加
	exchanges2, err := db2.GetExchanges("default")
	if err != nil {
		t.Fatalf("查询交易所失败: %v", err)
	}

	if len(exchanges2) != 4 {
		t.Fatalf("期望4个交易所，实际 %d 个", len(exchanges2))
	}

	hasPaperTrading = false
	for _, ex := range exchanges2 {
		if ex.ID == "paper_trading" {
			hasPaperTrading = true
			if ex.Name != "Paper Trading (Binance Testnet)" {
				t.Errorf("paper_trading 名称错误: %s", ex.Name)
			}
			if ex.Type != "paper_trading" {
				t.Errorf("paper_trading 类型错误: %s", ex.Type)
			}
			break
		}
	}

	if !hasPaperTrading {
		t.Fatal("升级后应该有 paper_trading 交易所")
	}

	t.Log("✓ 数据库升级成功，paper_trading 已自动添加")
}
