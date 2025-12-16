// Package credits 积分系统服务层
// 设计哲学：单一职责，最小依赖，高内聚低耦合
package credits

import (
	"context"
	"fmt"
	"log"
	"nofx/config"
	"time"
	"unicode/utf8"
)

// Service 积分服务接口
type Service interface {
	// 套餐查询 - 只读操作，无需事务
	GetActivePackages(ctx context.Context) ([]*config.CreditPackage, error)
	GetPackageByID(ctx context.Context, id string) (*config.CreditPackage, error)

	// 用户积分 - 核心业务逻辑
	GetUserCredits(ctx context.Context, userID string) (*config.UserCredits, error)
	AddCredits(ctx context.Context, userID string, amount int, category, description, refID string) error
	DeductCredits(ctx context.Context, userID string, amount int, category, description, refID string) error
	HasEnoughCredits(ctx context.Context, userID string, amount int) bool

	// 流水查询 - 只读操作
	GetUserTransactions(ctx context.Context, userID string, page, limit int) ([]*config.CreditTransaction, int, error)
	GetUserCreditSummary(ctx context.Context, userID string) (map[string]interface{}, error)

	// 管理功能 - 需要审计日志
	CreatePackage(ctx context.Context, pkg *config.CreditPackage) error
	UpdatePackage(ctx context.Context, pkg *config.CreditPackage) error
	DeletePackage(ctx context.Context, id string) error
	AdjustUserCredits(ctx context.Context, adminID, userID string, amount int, reason, ipAddress string) error
}

// CreditService 积分服务实现
type CreditService struct {
	db *config.Database
}

// NewCreditService 创建积分服务
func NewCreditService(db *config.Database) Service {
	return &CreditService{db: db}
}

// GetActivePackages 获取所有启用的套餐
func (s *CreditService) GetActivePackages(ctx context.Context) ([]*config.CreditPackage, error) {
	return s.db.GetActivePackages()
}

// GetPackageByID 根据ID获取套餐
func (s *CreditService) GetPackageByID(ctx context.Context, id string) (*config.CreditPackage, error) {
	return s.db.GetPackageByID(id)
}

// GetUserCredits 获取用户积分
// 如果用户没有积分记录，会自动创建（使用UPSERT避免并发问题）
func (s *CreditService) GetUserCredits(ctx context.Context, userID string) (*config.UserCredits, error) {
	if userID == "" {
		return nil, fmt.Errorf("用户ID不能为空")
	}

	// 使用UPSERT自动创建用户积分记录，避免并发竞态
	return s.db.GetOrCreateUserCredits(userID)
}

// AddCredits 增加用户积分
// 业务规则：
// 1. amount必须大于0
// 2. 自动创建用户积分账户（如果不存在）
// 3. 记录完整的流水信息
func (s *CreditService) AddCredits(ctx context.Context, userID string, amount int, category, description, refID string) error {
	// 参数验证 - 防御式编程
	if userID == "" {
		return fmt.Errorf("用户ID不能为空")
	}
	if amount <= 0 {
		return fmt.Errorf("积分数量必须大于0")
	}
	if category == "" {
		return fmt.Errorf("积分类别不能为空")
	}

	// 记录操作日志（用于调试和审计）
	start := time.Now()
	log.Printf("🔄 开始为用户 %s 增加积分 %d (类别: %s)", userID, amount, category)

	// 调用数据库层（已包含事务处理）
	err := s.db.AddCredits(userID, amount, category, description, refID)

	if err != nil {
		log.Printf("❌ 增加积分失败: %v", err)
		return fmt.Errorf("增加积分失败: %w", err)
	}

	log.Printf("✅ 成功为用户 %s 增加积分 %d (耗时: %v)", userID, amount, time.Since(start))
	return nil
}

// DeductCredits 扣减用户积分
// 业务规则：
// 1. amount必须大于0
// 2. 检查积分余额是否充足
// 3. 积分不足时返回明确错误
func (s *CreditService) DeductCredits(ctx context.Context, userID string, amount int, category, description, refID string) error {
	// 参数验证
	if userID == "" {
		return fmt.Errorf("用户ID不能为空")
	}
	if amount <= 0 {
		return fmt.Errorf("积分数量必须大于0")
	}
	if category == "" {
		return fmt.Errorf("积分类别不能为空")
	}

	// 记录操作日志
	start := time.Now()
	log.Printf("🔄 开始为用户 %s 扣减积分 %d (类别: %s)", userID, amount, category)

	// 调用数据库层（已包含事务和余额检查）
	err := s.db.DeductCredits(userID, amount, category, description, refID)

	if err != nil {
		log.Printf("❌ 扣减积分失败: %v", err)
		return fmt.Errorf("扣减积分失败: %w", err)
	}

	log.Printf("✅ 成功为用户 %s 扣减积分 %d (耗时: %v)", userID, amount, time.Since(start))
	return nil
}

// HasEnoughCredits 检查用户积分是否充足
// 返回bool值，简化调用方逻辑
func (s *CreditService) HasEnoughCredits(ctx context.Context, userID string, amount int) bool {
	if userID == "" || amount <= 0 {
		return false
	}
	return s.db.HasEnoughCredits(userID, amount)
}

// GetUserTransactions 获取用户积分流水
// 分页查询，支持大数据量场景
func (s *CreditService) GetUserTransactions(ctx context.Context, userID string, page, limit int) ([]*config.CreditTransaction, int, error) {
	if userID == "" {
		return nil, 0, fmt.Errorf("用户ID不能为空")
	}

	// 参数校正
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20 // 默认20条，最大100条
	}

	return s.db.GetUserTransactions(userID, page, limit)
}

// GetUserCreditSummary 获取用户积分摘要
// 包含统计信息，用于用户界面展示
func (s *CreditService) GetUserCreditSummary(ctx context.Context, userID string) (map[string]interface{}, error) {
	if userID == "" {
		return nil, fmt.Errorf("用户ID不能为空")
	}
	return s.db.GetUserCreditSummary(userID)
}

// CreatePackage 创建积分套餐
// 管理员功能，需要完整数据验证
func (s *CreditService) CreatePackage(ctx context.Context, pkg *config.CreditPackage) error {
	// 数据验证
	if err := validatePackage(pkg); err != nil {
		return err
	}

	// 设置默认值
	if pkg.ID == "" {
		pkg.ID = "pkg_" + generateTimestampID()
	}
	now := time.Now()
	pkg.CreatedAt = now
	pkg.UpdatedAt = now

	return s.db.CreateCreditPackage(pkg)
}

// UpdatePackage 更新积分套餐
func (s *CreditService) UpdatePackage(ctx context.Context, pkg *config.CreditPackage) error {
	// 数据验证
	if pkg.ID == "" {
		return fmt.Errorf("套餐ID不能为空")
	}
	if err := validatePackage(pkg); err != nil {
		return err
	}

	pkg.UpdatedAt = time.Now()
	return s.db.UpdateCreditPackage(pkg)
}

// DeletePackage 删除积分套餐（软删除）
func (s *CreditService) DeletePackage(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("套餐ID不能为空")
	}
	return s.db.DeleteCreditPackage(id)
}

// AdjustUserCredits 管理员调整用户积分
// 安全关键功能，需要完整审计日志
func (s *CreditService) AdjustUserCredits(ctx context.Context, adminID, userID string, amount int, reason, ipAddress string) error {
	// 权限验证
	if adminID == "" {
		return fmt.Errorf("管理员ID不能为空")
	}
	if userID == "" {
		return fmt.Errorf("用户ID不能为空")
	}
	if amount == 0 {
		return fmt.Errorf("调整积分数量不能为0")
	}
	reasonLen := utf8.RuneCountInString(reason)
	if reasonLen < 2 || reasonLen > 200 {
		return fmt.Errorf("调整原因长度必须在2-200字符之间")
	}

	// 记录管理员操作日志
	operation := "增加"
	if amount < 0 {
		operation = "扣减"
	}
	log.Printf("🔧 管理员 %s 正在%s用户 %s 的积分 %d (原因: %s)",
		adminID, operation, userID, amount, reason)

	// 调用数据库层（已包含审计日志）
	err := s.db.AdjustUserCredits(adminID, userID, amount, reason, ipAddress)
	if err != nil {
		log.Printf("❌ 管理员调整积分失败: %v", err)
		return fmt.Errorf("调整积分失败: %w", err)
	}

	log.Printf("✅ 管理员 %s 成功%s用户 %s 的积分 %d",
		adminID, operation, userID, amount)
	return nil
}

// validatePackage 验证积分套餐数据
// 遵循"防御式编程"原则，所有输入都必须验证
func validatePackage(pkg *config.CreditPackage) error {
	if pkg.Name == "" {
		return fmt.Errorf("套餐名称不能为空")
	}
	if pkg.PriceUSDT <= 0 {
		return fmt.Errorf("价格必须大于0")
	}
	if pkg.Credits <= 0 {
		return fmt.Errorf("积分数量必须大于0")
	}
	if pkg.BonusCredits < 0 {
		return fmt.Errorf("赠送积分不能为负数")
	}
	return nil
}

// generateTimestampID 生成基于时间戳的唯一ID
// 简单可靠，避免复杂的UUID依赖
func generateTimestampID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
