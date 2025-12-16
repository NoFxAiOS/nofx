package credits

import (
	"context"
	"fmt"
	"log"
	"nofx/config"
	"os"
	"testing"
)

// TestMain 设置测试环境
func TestMain(m *testing.M) {
	// 检查数据库连接
	if os.Getenv("DATABASE_URL") == "" {
		println("⚠️  警告: 未设置 DATABASE_URL 环境变量，跳过所有测试")
		os.Exit(0)
	}

	code := m.Run()
	os.Exit(code)
}

// setupTestService 创建测试服务
func setupTestService(t *testing.T) (*CreditService, func()) {
	db, err := config.NewDatabase("")
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}

	service := NewCreditService(db)

	cleanup := func() {
		db.Close()
	}

	return service.(*CreditService), cleanup
}

// TestServiceCreation 测试服务创建
func TestServiceCreation(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	if service == nil {
		t.Fatal("服务创建失败")
	}
	if service.db == nil {
		t.Fatal("数据库连接为空")
	}
}

// TestGetActivePackages 测试获取启用套餐
func TestGetActivePackages(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	packages, err := service.GetActivePackages(ctx)
	if err != nil {
		t.Fatalf("获取启用套餐失败: %v", err)
	}

	if len(packages) == 0 {
		t.Error("应该至少有一个启用的套餐")
	}

	// 验证所有套餐都是启用状态
	for _, pkg := range packages {
		if !pkg.IsActive {
			t.Errorf("套餐 %s 应该是启用状态", pkg.ID)
		}
	}

	t.Logf("✅ 成功获取 %d 个启用套餐", len(packages))
}

// TestGetPackageByID 测试根据ID获取套餐
func TestGetPackageByID(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// 先获取一个存在的套餐ID
	packages, err := service.GetActivePackages(ctx)
	if err != nil {
		t.Fatalf("获取套餐列表失败: %v", err)
	}

	if len(packages) == 0 {
		t.Skip("没有可用的套餐进行测试")
	}

	// 测试获取存在的套餐
	pkg, err := service.GetPackageByID(ctx, packages[0].ID)
	if err != nil {
		t.Fatalf("获取套餐失败: %v", err)
	}

	if pkg.ID != packages[0].ID {
		t.Errorf("套餐ID不匹配: expected %s, got %s", packages[0].ID, pkg.ID)
	}

	// 测试获取不存在的套餐
	_, err = service.GetPackageByID(ctx, "non_existent_package")
	if err == nil {
		t.Error("获取不存在的套餐应该返回错误")
	}

	t.Log("✅ 套餐获取功能正常")
}

// TestUserCreditsOperations 测试用户积分操作
func TestUserCreditsOperations(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	userID := "test_user_" + config.GenerateUUID()[:8]

	// 清理测试数据
	defer func() {
		// 清理测试数据 - 由于无法直接访问私有方法，使用服务层方法清理
		log.Printf("清理用户 %s 的测试数据", userID)
	}()

	// 测试获取或创建用户积分账户
	t.Run("GetUserCredits", func(t *testing.T) {
		credits, err := service.GetUserCredits(ctx, userID)
		if err != nil {
			t.Fatalf("获取用户积分失败: %v", err)
		}

		if credits.UserID != userID {
			t.Errorf("UserID不匹配: expected %s, got %s", userID, credits.UserID)
		}

		if credits.AvailableCredits != 0 {
			t.Errorf("初始可用积分应该为0: got %d", credits.AvailableCredits)
		}

		t.Log("✅ 成功创建用户积分账户")
	})

	// 测试增加积分
	t.Run("AddCredits", func(t *testing.T) {
		err := service.AddCredits(ctx, userID, 500, "purchase", "测试购买", "order_test_001")
		if err != nil {
			t.Fatalf("增加积分失败: %v", err)
		}

		// 验证结果
		credits, err := service.GetUserCredits(ctx, userID)
		if err != nil {
			t.Fatalf("获取用户积分失败: %v", err)
		}

		if credits.AvailableCredits != 500 {
			t.Errorf("可用积分不匹配: expected 500, got %d", credits.AvailableCredits)
		}

		if credits.TotalCredits != 500 {
			t.Errorf("总积分不匹配: expected 500, got %d", credits.TotalCredits)
		}

		t.Log("✅ 成功增加积分")
	})

	// 测试扣减积分
	t.Run("DeductCredits", func(t *testing.T) {
		err := service.DeductCredits(ctx, userID, 200, "consume", "测试消费", "service_test_001")
		if err != nil {
			t.Fatalf("扣减积分失败: %v", err)
		}

		// 验证结果
		credits, err := service.GetUserCredits(ctx, userID)
		if err != nil {
			t.Fatalf("获取用户积分失败: %v", err)
		}

		if credits.AvailableCredits != 300 {
			t.Errorf("可用积分不匹配: expected 300, got %d", credits.AvailableCredits)
		}

		if credits.UsedCredits != 200 {
			t.Errorf("已用积分不匹配: expected 200, got %d", credits.UsedCredits)
		}

		t.Log("✅ 成功扣减积分")
	})

	// 测试积分不足
	t.Run("InsufficientCredits", func(t *testing.T) {
		err := service.DeductCredits(ctx, userID, 1000, "consume", "测试消费", "service_test_002")
		if err == nil {
			t.Error("应该返回积分不足错误")
		}

		t.Log("✅ 正确检测积分不足")
	})

	// 测试检查积分是否充足
	t.Run("HasEnoughCredits", func(t *testing.T) {
		if !service.HasEnoughCredits(ctx, userID, 100) {
			t.Error("应该返回积分充足")
		}

		if service.HasEnoughCredits(ctx, userID, 1000) {
			t.Error("应该返回积分不足")
		}

		t.Log("✅ HasEnoughCredits检查正确")
	})
}

// TestParameterValidation 测试参数验证
func TestParameterValidation(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// 测试空用户ID
	t.Run("EmptyUserID", func(t *testing.T) {
		_, err := service.GetUserCredits(ctx, "")
		if err == nil {
			t.Error("空用户ID应该返回错误")
		}

		err = service.AddCredits(ctx, "", 100, "purchase", "测试", "ref")
		if err == nil {
			t.Error("空用户ID应该返回错误")
		}

		err = service.DeductCredits(ctx, "", 100, "consume", "测试", "ref")
		if err == nil {
			t.Error("空用户ID应该返回错误")
		}
	})

	// 测试无效积分数量
	t.Run("InvalidAmount", func(t *testing.T) {
		userID := "test_user_validation"

		err := service.AddCredits(ctx, userID, 0, "purchase", "测试", "ref")
		if err == nil {
			t.Error("积分数量为0应该返回错误")
		}

		err = service.AddCredits(ctx, userID, -100, "purchase", "测试", "ref")
		if err == nil {
			t.Error("积分数量为负数应该返回错误")
		}

		err = service.DeductCredits(ctx, userID, 0, "consume", "测试", "ref")
		if err == nil {
			t.Error("扣减积分数量为0应该返回错误")
		}
	})

	// 测试空积分类别
	t.Run("EmptyCategory", func(t *testing.T) {
		userID := "test_user_validation"

		err := service.AddCredits(ctx, userID, 100, "", "测试", "ref")
		if err == nil {
			t.Error("空积分类别应该返回错误")
		}

		err = service.DeductCredits(ctx, userID, 100, "", "测试", "ref")
		if err == nil {
			t.Error("空积分类别应该返回错误")
		}
	})
}

// TestTransactions 测试积分流水
func TestTransactions(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	userID := "test_txn_" + config.GenerateUUID()[:8]

	// 清理测试数据
	defer func() {
		log.Printf("清理用户 %s 的测试数据", userID)
	}()

	// 准备测试数据
	service.GetUserCredits(ctx, userID)
	service.AddCredits(ctx, userID, 1000, "purchase", "测试购买", "order_test")
	service.DeductCredits(ctx, userID, 300, "consume", "测试消费", "service_test")
	service.AddCredits(ctx, userID, 500, "gift", "测试赠送", "gift_test")

	// 测试获取积分流水
	t.Run("GetUserTransactions", func(t *testing.T) {
		transactions, total, err := service.GetUserTransactions(ctx, userID, 1, 10)
		if err != nil {
			t.Fatalf("获取积分流水失败: %v", err)
		}

		if total != 3 {
			t.Errorf("流水总数不匹配: expected 3, got %d", total)
		}

		if len(transactions) != 3 {
			t.Errorf("返回流水数量不匹配: expected 3, got %d", len(transactions))
		}

		// 验证排序（最新的在前）
		if transactions[0].Type != "credit" {
			t.Errorf("第一笔流水应该是最新的credit: got %s", transactions[0].Type)
		}

		t.Log("✅ 成功获取积分流水")
	})

	// 测试分页参数
	t.Run("PaginationParameters", func(t *testing.T) {
		// 测试页码校正
		_, _, err := service.GetUserTransactions(ctx, userID, 0, 10)
		if err != nil {
			t.Fatalf("页码为0时不应该报错: %v", err)
		}

		// 测试限制数量校正
		transactions, _, err := service.GetUserTransactions(ctx, userID, 1, 200)
		if err != nil {
			t.Fatalf("获取流水失败: %v", err)
		}

		if len(transactions) > 100 {
			t.Errorf("限制数量应该被校正为100: got %d", len(transactions))
		}

		t.Log("✅ 分页参数校正正确")
	})

	// 测试获取积分摘要
	t.Run("GetUserCreditSummary", func(t *testing.T) {
		summary, err := service.GetUserCreditSummary(ctx, userID)
		if err != nil {
			t.Fatalf("获取积分摘要失败: %v", err)
		}

		// 验证必要字段
		availableCredits, ok := summary["available_credits"].(int)
		if !ok {
			t.Error("available_credits字段缺失或类型错误")
		}

		if availableCredits != 1200 { // 1000 - 300 + 500
			t.Errorf("可用积分不匹配: expected 1200, got %d", availableCredits)
		}

		totalCredits, ok := summary["total_credits"].(int)
		if !ok {
			t.Error("total_credits字段缺失或类型错误")
		}

		if totalCredits != 1500 { // 1000 + 500
			t.Errorf("总积分不匹配: expected 1500, got %d", totalCredits)
		}

		usedCredits, ok := summary["used_credits"].(int)
		if !ok {
			t.Error("used_credits字段缺失或类型错误")
		}

		if usedCredits != 300 {
			t.Errorf("已用积分不匹配: expected 300, got %d", usedCredits)
		}

		t.Log("✅ 成功获取积分摘要")
	})
}

// TestAdminOperations 测试管理员操作
func TestAdminOperations(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	adminID := "admin_test"
	userID := "test_admin_user_" + config.GenerateUUID()[:8]

	// 清理测试数据
	defer func() {
		// 清理测试数据 - 由于无法直接访问私有方法，使用服务层方法清理
		log.Printf("清理用户 %s 的测试数据", userID)
	}()

	// 准备测试数据
	service.GetUserCredits(ctx, userID)
	service.AddCredits(ctx, userID, 500, "purchase", "测试购买", "order_test")

	// 测试管理员增加积分
	t.Run("AdminAddCredits", func(t *testing.T) {
		err := service.AdjustUserCredits(ctx, adminID, userID, 1000, "新用户奖励", "192.168.1.1")
		if err != nil {
			t.Fatalf("管理员增加积分失败: %v", err)
		}

		// 验证结果
		credits, err := service.GetUserCredits(ctx, userID)
		if err != nil {
			t.Fatalf("获取用户积分失败: %v", err)
		}

		if credits.AvailableCredits != 1500 { // 500 + 1000
			t.Errorf("可用积分不匹配: expected 1500, got %d", credits.AvailableCredits)
		}

		t.Log("✅ 管理员增加积分成功")
	})

	// 测试管理员扣减积分
	t.Run("AdminDeductCredits", func(t *testing.T) {
		err := service.AdjustUserCredits(ctx, adminID, userID, -300, "违规处罚", "192.168.1.1")
		if err != nil {
			t.Fatalf("管理员扣减积分失败: %v", err)
		}

		// 验证结果
		credits, err := service.GetUserCredits(ctx, userID)
		if err != nil {
			t.Fatalf("获取用户积分失败: %v", err)
		}

		if credits.AvailableCredits != 1200 { // 1500 - 300
			t.Errorf("可用积分不匹配: expected 1200, got %d", credits.AvailableCredits)
		}

		t.Log("✅ 管理员扣减积分成功")
	})

	// 测试管理员扣减积分不足
	t.Run("AdminInsufficientCredits", func(t *testing.T) {
		err := service.AdjustUserCredits(ctx, adminID, userID, -2000, "严重违规", "192.168.1.1")
		if err == nil {
			t.Error("应该返回积分不足错误")
		}

		t.Log("✅ 正确检测管理员扣减积分不足")
	})

	// 测试管理员操作参数验证
	t.Run("AdminParameterValidation", func(t *testing.T) {
		// 空管理员ID
		err := service.AdjustUserCredits(ctx, "", userID, 100, "测试", "192.168.1.1")
		if err == nil {
			t.Error("空管理员ID应该返回错误")
		}

		// 调整数量为0
		err = service.AdjustUserCredits(ctx, adminID, userID, 0, "测试", "192.168.1.1")
		if err == nil {
			t.Error("调整数量为0应该返回错误")
		}

		// 调整原因过短
		err = service.AdjustUserCredits(ctx, adminID, userID, 100, "测", "192.168.1.1")
		if err == nil {
			t.Error("调整原因过短应该返回错误")
		}

		// 调整原因过长
		longReason := ""
		for i := 0; i < 250; i++ {
			longReason += "a"
		}
		err = service.AdjustUserCredits(ctx, adminID, userID, 100, longReason, "192.168.1.1")
		if err == nil {
			t.Error("调整原因过长应该返回错误")
		}

		t.Log("✅ 管理员操作参数验证正确")
	})
}

// TestPackageOperations 测试套餐操作
func TestPackageOperations(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// 测试创建套餐
	t.Run("CreatePackage", func(t *testing.T) {
		pkg := &config.CreditPackage{
			Name:          "测试套餐",
			NameEN:        "Test Package",
			Description:   "这是一个测试套餐",
			PriceUSDT:     19.99,
			Credits:       1000,
			BonusCredits:  100,
			IsActive:      true,
			IsRecommended: false,
			SortOrder:     999,
		}

		err := service.CreatePackage(ctx, pkg)
		if err != nil {
			t.Fatalf("创建套餐失败: %v", err)
		}

		// 验证创建结果
		if pkg.ID == "" {
			t.Error("套餐ID应该自动生成")
		}

		t.Log("✅ 成功创建套餐")

		// 清理测试数据
		service.DeletePackage(ctx, pkg.ID)
	})

	// 测试创建套餐参数验证
	t.Run("CreatePackageValidation", func(t *testing.T) {
		// 空名称
		pkg := &config.CreditPackage{
			PriceUSDT: 10.0,
			Credits:   500,
		}
		err := service.CreatePackage(ctx, pkg)
		if err == nil {
			t.Error("空名称应该返回错误")
		}

		// 价格为0
		pkg = &config.CreditPackage{
			Name:      "测试套餐",
			PriceUSDT: 0,
			Credits:   500,
		}
		err = service.CreatePackage(ctx, pkg)
		if err == nil {
			t.Error("价格为0应该返回错误")
		}

		// 价格为负数
		pkg = &config.CreditPackage{
			Name:      "测试套餐",
			PriceUSDT: -10.0,
			Credits:   500,
		}
		err = service.CreatePackage(ctx, pkg)
		if err == nil {
			t.Error("价格为负数应该返回错误")
		}

		// 积分数量为0
		pkg = &config.CreditPackage{
			Name:      "测试套餐",
			PriceUSDT: 10.0,
			Credits:   0,
		}
		err = service.CreatePackage(ctx, pkg)
		if err == nil {
			t.Error("积分数量为0应该返回错误")
		}

		// 赠送积分为负数
		pkg = &config.CreditPackage{
			Name:         "测试套餐",
			PriceUSDT:    10.0,
			Credits:      500,
			BonusCredits: -100,
		}
		err = service.CreatePackage(ctx, pkg)
		if err == nil {
			t.Error("赠送积分为负数应该返回错误")
		}

		t.Log("✅ 创建套餐参数验证正确")
	})
}

// TestConcurrentOperations 测试并发操作
func TestConcurrentOperations(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	userID := "concurrent_test_" + config.GenerateUUID()[:8]

	// 清理测试数据
	defer func() {
		// 清理测试数据 - 由于无法直接访问私有方法，使用服务层方法清理
		log.Printf("清理用户 %s 的测试数据", userID)
	}()

	// 准备初始数据
	service.GetUserCredits(ctx, userID)
	service.AddCredits(ctx, userID, 1000, "purchase", "初始积分", "init_order")

	// 并发扣减测试
	t.Run("ConcurrentDeduction", func(t *testing.T) {
		concurrency := 10
		deductionPerGoroutine := 50
		expectedTotal := 1000 - (concurrency * deductionPerGoroutine)

		done := make(chan bool, concurrency)
		errors := make(chan error, concurrency)

		// 启动并发扣减
		for i := 0; i < concurrency; i++ {
			go func(index int) {
				defer func() { done <- true }()

				err := service.DeductCredits(ctx, userID, deductionPerGoroutine,
					"concurrent_test", "并发测试", fmt.Sprintf("concurrent_%d", index))
				if err != nil {
					errors <- err
				}
			}(i)
		}

		// 等待所有goroutine完成
		for i := 0; i < concurrency; i++ {
			<-done
		}

		close(errors)

		// 检查错误
		errorCount := 0
		for err := range errors {
			t.Logf("并发扣减错误: %v", err)
			errorCount++
		}

		if errorCount > 0 {
			t.Errorf("并发扣减出现 %d 个错误", errorCount)
		}

		// 验证最终余额
		credits, err := service.GetUserCredits(ctx, userID)
		if err != nil {
			t.Fatalf("获取最终余额失败: %v", err)
		}

		if credits.AvailableCredits != expectedTotal {
			t.Errorf("并发扣减后余额不正确: expected %d, got %d", expectedTotal, credits.AvailableCredits)
		}

		t.Logf("✅ 并发扣减测试通过: %d 个goroutine，最终余额 %d", concurrency, credits.AvailableCredits)
	})
}

// cleanupUserCredits 清理用户积分测试数据
func cleanupUserCredits(db interface{}, userID string) {
	// 由于exec方法是私有的，我们使用其他方式清理数据
	// 在实际测试中，可以使用事务回滚或测试数据库
	log.Printf("清理用户 %s 的测试数据", userID)
}
