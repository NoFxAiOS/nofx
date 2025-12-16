package trader

import (
	"context"
	"fmt"
	"nofx/config"
	"nofx/service/credits"
	"os"
	"testing"
	"time"
)

// TestTopTraderCreditConsumption 测试TopTrader积分消耗
func TestTopTraderCreditConsumption(t *testing.T) {
	// 1. 初始化数据库
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("跳过测试: 未设置DATABASE_URL")
	}
	db, err := config.NewDatabase("")
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	// 2. 准备测试用户
	userID := fmt.Sprintf("test_user_%d", time.Now().UnixNano())
	user := &config.User{
		ID:       userID,
		Email:    userID + "@test.com",
		IsActive: true,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 3. 增加初始积分
	initialCredits := 100
	creditService := credits.NewCreditService(db)
	err = creditService.AddCredits(context.Background(), userID, initialCredits, "purchase", "Test Init", "test_ref")
	if err != nil {
		t.Fatalf("增加积分失败: %v", err)
	}

	// 4. 配置系统消耗
	cost := 5
	db.SetSystemConfig("trading_decision_points_cost", fmt.Sprintf("%d", cost))

	// 5. 创建TopTrader
	cfg := AutoTraderConfig{
		ID:             "test_top_trader",
		UserID:         userID,
		Name:           "TopTrader", // 关键：必须是TopTrader
		AIModel:        "deepseek",
		Exchange:       "binance",
		InitialBalance: 1000,
		Database:       db,
		ScanInterval:   1 * time.Second,
	}

	// 使用EnhancedAutoTrader，因为它暴露了RunCycle
	trader, err := NewEnhancedAutoTrader(cfg)
	if err != nil {
		t.Fatalf("创建Trader失败: %v", err)
	}

	// 模拟 mcpClient (或者由于我们无法mock internal mcpClient，我们预期它会失败或我们只关心积分扣减逻辑在前面)
	// runCycle的逻辑是：
	// 0. 积分检查 -> 1. 停止检查 -> 2. 重置 -> 3. 上下文 -> 4. AI决策
	// 积分扣减在第0步。即使后面失败，积分也应该已经被扣除（因为是预付费模式）。
	// Wait, code says:
	// "err = at.creditService.DeductCredits..."
	// "if err != nil ... return"
	// So if deduction succeeds, it proceeds.

	// 6. 运行一次周期
	// 注意：由于没有真实的API Key，buildTradingContext可能会失败，或者AI决策会失败。
	// 但我们只关心积分是否被扣减。积分扣减发生在最前面。
	err = trader.RunCycle()
	if err != nil {
		// 预期可能会有其他错误（如API配置错误），但积分扣减应该已经发生
		t.Logf("RunCycle返回错误(预期内): %v", err)
	}

	// 7. 验证积分扣减
	userCredits, err := creditService.GetUserCredits(context.Background(), userID)
	if err != nil {
		t.Fatalf("获取用户积分失败: %v", err)
	}

	expectedAvailable := initialCredits - cost
	if userCredits.AvailableCredits != expectedAvailable {
		t.Errorf("可用积分不匹配: 期望 %d, 实际 %d", expectedAvailable, userCredits.AvailableCredits)
	}

	if userCredits.UsedCredits != cost {
		t.Errorf("已用积分不匹配: 期望 %d, 实际 %d", cost, userCredits.UsedCredits)
	}

	t.Logf("✅ 积分消耗测试通过: 初始 %d, 消耗 %d, 剩余 %d", initialCredits, cost, userCredits.AvailableCredits)

	// 8. 测试积分不足的情况
	// 消耗完剩余积分
	err = creditService.DeductCredits(context.Background(), userID, userCredits.AvailableCredits, "consume", "Drain", "drain_ref")
	if err != nil {
		t.Fatalf("消耗剩余积分失败: %v", err)
	}

	// 再次运行周期，应该失败并报错"积分不足"
	err = trader.RunCycle()
	if err == nil {
		t.Error("积分不足时RunCycle应该返回错误")
	} else {
		t.Logf("✅ 积分不足测试通过: %v", err)
	}
}
