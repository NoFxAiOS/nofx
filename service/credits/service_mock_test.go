package credits

import (
	"context"
	"errors"
	"nofx/config"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockService 是一个模拟的积分服务实现，用于测试
type MockService struct {
	packages     []*config.CreditPackage
	userCredits  map[string]*config.UserCredits
	transactions map[string][]*config.CreditTransaction
	shouldFail   bool
	failError    error
}

// NewMockService 创建模拟服务
func NewMockService() *MockService {
	return &MockService{
		packages:     make([]*config.CreditPackage, 0),
		userCredits:  make(map[string]*config.UserCredits),
		transactions: make(map[string][]*config.CreditTransaction),
	}
}

func (m *MockService) SetShouldFail(shouldFail bool, err error) {
	m.shouldFail = shouldFail
	m.failError = err
}

func (m *MockService) GetActivePackages(ctx context.Context) ([]*config.CreditPackage, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	return m.packages, nil
}

func (m *MockService) GetPackageByID(ctx context.Context, id string) (*config.CreditPackage, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	for _, pkg := range m.packages {
		if pkg.ID == id {
			return pkg, nil
		}
	}
	return nil, errors.New("package not found")
}

func (m *MockService) GetUserCredits(ctx context.Context, userID string) (*config.UserCredits, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	if credits, exists := m.userCredits[userID]; exists {
		return credits, nil
	}
	// 自动创建用户积分记录
	credits := &config.UserCredits{
		UserID:           userID,
		AvailableCredits: 0,
		TotalCredits:     0,
		UsedCredits:      0,
	}
	m.userCredits[userID] = credits
	return credits, nil
}

func (m *MockService) AddCredits(ctx context.Context, userID string, amount int, category, description, refID string) error {
	if m.shouldFail {
		return m.failError
	}
	if amount <= 0 {
		return errors.New("积分数量必须大于0")
	}

	credits, err := m.GetUserCredits(ctx, userID)
	if err != nil {
		return err
	}

	credits.AvailableCredits += amount
	credits.TotalCredits += amount

	// 添加交易记录
	transaction := &config.CreditTransaction{
		UserID:        userID,
		Type:          "credit",
		Amount:        amount,
		BalanceBefore: credits.AvailableCredits - amount,
		BalanceAfter:  credits.AvailableCredits,
		Category:      category,
		Description:   description,
		ReferenceID:   refID,
	}

	m.transactions[userID] = append(m.transactions[userID], transaction)
	return nil
}

func (m *MockService) DeductCredits(ctx context.Context, userID string, amount int, category, description, refID string) error {
	if m.shouldFail {
		return m.failError
	}
	if amount <= 0 {
		return errors.New("积分数量必须大于0")
	}

	credits, err := m.GetUserCredits(ctx, userID)
	if err != nil {
		return err
	}

	if credits.AvailableCredits < amount {
		return errors.New("积分余额不足")
	}

	credits.AvailableCredits -= amount
	credits.UsedCredits += amount

	// 添加交易记录
	transaction := &config.CreditTransaction{
		UserID:        userID,
		Type:          "debit",
		Amount:        amount,
		BalanceBefore: credits.AvailableCredits + amount,
		BalanceAfter:  credits.AvailableCredits,
		Category:      category,
		Description:   description,
		ReferenceID:   refID,
	}

	m.transactions[userID] = append(m.transactions[userID], transaction)
	return nil
}

func (m *MockService) HasEnoughCredits(ctx context.Context, userID string, amount int) bool {
	credits, err := m.GetUserCredits(ctx, userID)
	if err != nil {
		return false
	}
	return credits.AvailableCredits >= amount
}

func (m *MockService) GetUserTransactions(ctx context.Context, userID string, page, limit int) ([]*config.CreditTransaction, int, error) {
	if m.shouldFail {
		return nil, 0, m.failError
	}

	allTxns := m.transactions[userID]
	total := len(allTxns)

	// 处理边界情况
	if page <= 0 || limit <= 0 {
		return []*config.CreditTransaction{}, total, nil
	}

	// 分页逻辑
	start := (page - 1) * limit
	if start >= total {
		return []*config.CreditTransaction{}, total, nil
	}

	end := start + limit
	if end > total {
		end = total
	}

	return allTxns[start:end], total, nil
}

func (m *MockService) GetUserCreditSummary(ctx context.Context, userID string) (map[string]interface{}, error) {
	if m.shouldFail {
		return nil, m.failError
	}

	credits, err := m.GetUserCredits(ctx, userID)
	if err != nil {
		return nil, err
	}

	txns := m.transactions[userID]

	return map[string]interface{}{
		"total_credits":     credits.TotalCredits,
		"available_credits": credits.AvailableCredits,
		"used_credits":      credits.UsedCredits,
		"transaction_count": len(txns),
	}, nil
}

func (m *MockService) CreatePackage(ctx context.Context, pkg *config.CreditPackage) error {
	if m.shouldFail {
		return m.failError
	}
	if pkg.ID == "" {
		return errors.New("套餐ID不能为空")
	}
	if pkg.Name == "" {
		return errors.New("套餐名称不能为空")
	}
	if pkg.PriceUSDT <= 0 {
		return errors.New("价格必须大于0")
	}
	if pkg.Credits <= 0 {
		return errors.New("积分数必须大于0")
	}

	m.packages = append(m.packages, pkg)
	return nil
}

func (m *MockService) UpdatePackage(ctx context.Context, pkg *config.CreditPackage) error {
	if m.shouldFail {
		return m.failError
	}
	if pkg.ID == "" {
		return errors.New("套餐ID不能为空")
	}

	for i, existing := range m.packages {
		if existing.ID == pkg.ID {
			m.packages[i] = pkg
			return nil
		}
	}
	return errors.New("套餐不存在")
}

func (m *MockService) DeletePackage(ctx context.Context, id string) error {
	if m.shouldFail {
		return m.failError
	}

	for i, pkg := range m.packages {
		if pkg.ID == id {
			// 从切片中删除
			m.packages = append(m.packages[:i], m.packages[i+1:]...)
			return nil
		}
	}
	return errors.New("套餐不存在")
}

func (m *MockService) AdjustUserCredits(ctx context.Context, adminID, userID string, amount int, reason, ipAddress string) error {
	if m.shouldFail {
		return m.failError
	}
	if adminID == "" {
		return errors.New("管理员ID不能为空")
	}
	if userID == "" {
		return errors.New("用户ID不能为空")
	}
	if amount == 0 {
		return errors.New("调整数量不能为0")
	}
	if reason == "" {
		return errors.New("调整原因不能为空")
	}

	// 执行积分调整
	if amount > 0 {
		return m.AddCredits(ctx, userID, amount, "admin_adjust", reason, "admin_"+adminID)
	} else {
		return m.DeductCredits(ctx, userID, -amount, "admin_adjust", reason, "admin_"+adminID)
	}
}

// TestMockService 测试模拟服务
func TestMockService(t *testing.T) {
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
