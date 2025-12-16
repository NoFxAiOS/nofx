package credits

import (
	"context"
	"errors"
	"nofx/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMockServiceOnly 仅测试模拟服务，不依赖数据库
func TestMockServiceOnly(t *testing.T) {
	ctx := context.Background()
	service := NewMockService()

	// 添加测试套餐
	pkg := &config.CreditPackage{
		ID:        "test_pkg",
		Name:      "测试套餐",
		PriceUSDT: 99.99,
		Credits:   1000,
	}
	err := service.CreatePackage(ctx, pkg)
	assert.NoError(t, err)

	// 测试获取套餐
	packages, err := service.GetActivePackages(ctx)
	assert.NoError(t, err)
	assert.Len(t, packages, 1)

	// 测试获取用户积分（自动创建）
	credits, err := service.GetUserCredits(ctx, "test_user")
	assert.NoError(t, err)
	assert.Equal(t, "test_user", credits.UserID)
	assert.Equal(t, 0, credits.AvailableCredits)

	// 测试增加积分
	err = service.AddCredits(ctx, "test_user", 500, "purchase", "购买测试", "ref123")
	assert.NoError(t, err)

	credits, err = service.GetUserCredits(ctx, "test_user")
	assert.NoError(t, err)
	assert.Equal(t, 500, credits.AvailableCredits)

	// 测试检查积分是否足够
	enough := service.HasEnoughCredits(ctx, "test_user", 300)
	assert.True(t, enough)

	enough = service.HasEnoughCredits(ctx, "test_user", 600)
	assert.False(t, enough)

	// 测试消费积分
	err = service.DeductCredits(ctx, "test_user", 200, "consumption", "消费测试", "ref456")
	assert.NoError(t, err)

	credits, err = service.GetUserCredits(ctx, "test_user")
	assert.NoError(t, err)
	assert.Equal(t, 300, credits.AvailableCredits)
	assert.Equal(t, 200, credits.UsedCredits)

	// 测试获取交易记录
	txns, total, err := service.GetUserTransactions(ctx, "test_user", 1, 10)
	assert.NoError(t, err)
	assert.Len(t, txns, 2)
	assert.Equal(t, 2, total)

	// 测试获取积分摘要
	summary, err := service.GetUserCreditSummary(ctx, "test_user")
	assert.NoError(t, err)
	assert.Equal(t, 500, summary["total_credits"])
	assert.Equal(t, 300, summary["available_credits"])
	assert.Equal(t, 200, summary["used_credits"])
	assert.Equal(t, 2, summary["transaction_count"])

	// 测试管理员调整积分
	err = service.AdjustUserCredits(ctx, "admin1", "test_user", 100, "补偿", "127.0.0.1")
	assert.NoError(t, err)

	credits, err = service.GetUserCredits(ctx, "test_user")
	assert.NoError(t, err)
	assert.Equal(t, 400, credits.AvailableCredits)

	// 测试错误情况
	service.SetShouldFail(true, errors.New("模拟错误"))
	_, err = service.GetActivePackages(ctx)
	assert.Error(t, err)
	assert.Equal(t, "模拟错误", err.Error())
}

// TestMockServiceErrorCases 测试模拟服务的错误情况
func TestMockServiceErrorCases(t *testing.T) {
	ctx := context.Background()
	service := NewMockService()

	// 测试增加负积分
	err := service.AddCredits(ctx, "user1", -100, "purchase", "测试", "ref")
	assert.Error(t, err)
	assert.Equal(t, "积分数量必须大于0", err.Error())

	// 测试消费负积分
	err = service.DeductCredits(ctx, "user1", -50, "consumption", "测试", "ref")
	assert.Error(t, err)
	assert.Equal(t, "积分数量必须大于0", err.Error())

	// 测试积分不足
	err = service.AddCredits(ctx, "user1", 100, "purchase", "测试", "ref")
	assert.NoError(t, err)

	err = service.DeductCredits(ctx, "user1", 150, "consumption", "测试", "ref")
	assert.Error(t, err)
	assert.Equal(t, "积分余额不足", err.Error())

	// 测试创建无效套餐
	invalidPkg := &config.CreditPackage{}
	err = service.CreatePackage(ctx, invalidPkg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "套餐ID不能为空")

	invalidPkg2 := &config.CreditPackage{
		ID:        "pkg2",
		Name:      "测试套餐",
		PriceUSDT: -10,
		Credits:   100,
	}
	err = service.CreatePackage(ctx, invalidPkg2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "价格必须大于0")

	// 测试更新不存在的套餐
	notExistPkg := &config.CreditPackage{
		ID:   "not_exist",
		Name: "不存在的套餐",
	}
	err = service.UpdatePackage(ctx, notExistPkg)
	assert.Error(t, err)
	assert.Equal(t, "套餐不存在", err.Error())

	// 测试删除不存在的套餐
	err = service.DeletePackage(ctx, "not_exist")
	assert.Error(t, err)
	assert.Equal(t, "套餐不存在", err.Error())

	// 测试管理员调整积分的错误情况
	err = service.AdjustUserCredits(ctx, "", "user1", 100, "测试", "127.0.0.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "管理员ID不能为空")

	err = service.AdjustUserCredits(ctx, "admin1", "", 100, "测试", "127.0.0.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "用户ID不能为空")

	err = service.AdjustUserCredits(ctx, "admin1", "user1", 0, "测试", "127.0.0.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "调整数量不能为0")

	err = service.AdjustUserCredits(ctx, "admin1", "user1", 100, "", "127.0.0.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "调整原因不能为空")
}

// TestMockServicePagination 测试分页功能
func TestMockServicePagination(t *testing.T) {
	ctx := context.Background()
	service := NewMockService()

	// 添加大量交易记录
	for i := 1; i <= 25; i++ {
		err := service.AddCredits(ctx, "user1", i*10, "purchase", "测试", "ref")
		assert.NoError(t, err)
	}

	// 测试第一页
	txns, total, err := service.GetUserTransactions(ctx, "user1", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, txns, 10)

	// 测试第二页
	txns, total, err = service.GetUserTransactions(ctx, "user1", 2, 10)
	assert.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, txns, 10)

	// 测试第三页（剩余5条）
	txns, total, err = service.GetUserTransactions(ctx, "user1", 3, 10)
	assert.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, txns, 5)

	// 测试超出页数
	txns, total, err = service.GetUserTransactions(ctx, "user1", 4, 10)
	assert.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, txns, 0)

	// 测试边界情况
	txns, total, err = service.GetUserTransactions(ctx, "user1", 0, 10)
	assert.NoError(t, err)
	assert.Equal(t, 25, total)
	assert.Len(t, txns, 0)
}

// TestMockServiceEdgeCases 测试边界情况
func TestMockServiceEdgeCases(t *testing.T) {
	ctx := context.Background()
	service := NewMockService()

	// 测试不存在的用户
	credits, err := service.GetUserCredits(ctx, "non_existent")
	assert.NoError(t, err)
	assert.NotNil(t, credits)
	assert.Equal(t, "non_existent", credits.UserID)
	assert.Equal(t, 0, credits.AvailableCredits)

	// 测试不存在的套餐
	pkg, err := service.GetPackageByID(ctx, "non_existent")
	assert.Error(t, err)
	assert.Nil(t, pkg)
	assert.Equal(t, "package not found", err.Error())

	// 测试空套餐列表
	packages, err := service.GetActivePackages(ctx)
	assert.NoError(t, err)
	assert.Len(t, packages, 0)

	// 测试零积分检查
	enough := service.HasEnoughCredits(ctx, "user1", 0)
	assert.True(t, enough)

	// 测试新用户的交易记录
	txns, total, err := service.GetUserTransactions(ctx, "new_user", 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Len(t, txns, 0)

	// 测试新用户的积分摘要
	summary, err := service.GetUserCreditSummary(ctx, "new_user")
	assert.NoError(t, err)
	assert.Equal(t, 0, summary["total_credits"])
	assert.Equal(t, 0, summary["available_credits"])
	assert.Equal(t, 0, summary["used_credits"])
	assert.Equal(t, 0, summary["transaction_count"])
}
