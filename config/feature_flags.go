package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// FeatureFlagType 功能标志类型
type FeatureFlagType string

const (
	// 新闻相关功能标志
	NewsAutoFetchEnabled         FeatureFlagType = "news.auto_fetch_enabled"
	NewsPromptInjectionProtection FeatureFlagType = "news.prompt_injection_protection"
	NewsCircuitBreakerEnabled     FeatureFlagType = "news.circuit_breaker_enabled"
	NewsCacheEnabled              FeatureFlagType = "news.cache_enabled"

	// 其他功能标志
	BetaModeEnabled FeatureFlagType = "beta_mode"
	AdminModeEnabled FeatureFlagType = "admin_mode"
)

// FeatureFlag 功能标志结构
type FeatureFlag struct {
	Name        FeatureFlagType `json:"name"`
	Description string          `json:"description"`
	Enabled     bool            `json:"enabled"`
	Percentage  int             `json:"percentage"` // 灰度发布百分比 (0-100)
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // 自定义元数据
}

// FeatureFlagManager 功能标志管理器
type FeatureFlagManager struct {
	flags map[FeatureFlagType]*FeatureFlag
	mu    sync.RWMutex
}

// NewFeatureFlagManager 创建功能标志管理器
func NewFeatureFlagManager() *FeatureFlagManager {
	manager := &FeatureFlagManager{
		flags: make(map[FeatureFlagType]*FeatureFlag),
	}

	// 初始化默认功能标志
	manager.initializeDefaultFlags()

	return manager
}

// initializeDefaultFlags 初始化默认功能标志
func (fm *FeatureFlagManager) initializeDefaultFlags() {
	defaultFlags := []FeatureFlag{
		{
			Name:        NewsAutoFetchEnabled,
			Description: "启用新闻自动抓取功能",
			Enabled:     true,
			Percentage:  100,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        NewsPromptInjectionProtection,
			Description: "启用新闻提示词注入防护",
			Enabled:     true,
			Percentage:  100,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        NewsCircuitBreakerEnabled,
			Description: "启用新闻API熔断器保护",
			Enabled:     true,
			Percentage:  100,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        NewsCacheEnabled,
			Description: "启用新闻缓存功能",
			Enabled:     true,
			Percentage:  100,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        BetaModeEnabled,
			Description: "启用测试版本功能",
			Enabled:     false,
			Percentage:  0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        AdminModeEnabled,
			Description: "启用管理员模式",
			Enabled:     false,
			Percentage:  0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for i := range defaultFlags {
		fm.flags[defaultFlags[i].Name] = &defaultFlags[i]
	}
}

// IsEnabled 检查功能标志是否启用
func (fm *FeatureFlagManager) IsEnabled(name FeatureFlagType) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	flag, exists := fm.flags[name]
	if !exists {
		return false
	}

	return flag.Enabled
}

// IsEnabledForUser 检查功能标志是否对特定用户启用（用于灰度发布）
func (fm *FeatureFlagManager) IsEnabledForUser(name FeatureFlagType, userID string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	flag, exists := fm.flags[name]
	if !exists {
		return false
	}

	if !flag.Enabled {
		return false
	}

	// 如果百分比是100，则对所有用户启用
	if flag.Percentage >= 100 {
		return true
	}

	// 如果百分比是0，则对任何用户都不启用
	if flag.Percentage <= 0 {
		return false
	}

	// 根据userID的哈希值决定灰度发布
	hashValue := hashUserID(userID)
	return (hashValue % 100) < flag.Percentage
}

// SetEnabled 设置功能标志的启用状态
func (fm *FeatureFlagManager) SetEnabled(name FeatureFlagType, enabled bool) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	flag, exists := fm.flags[name]
	if !exists {
		return fmt.Errorf("功能标志不存在: %s", name)
	}

	flag.Enabled = enabled
	flag.UpdatedAt = time.Now()

	return nil
}

// SetPercentage 设置功能标志的灰度发布百分比
func (fm *FeatureFlagManager) SetPercentage(name FeatureFlagType, percentage int) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	flag, exists := fm.flags[name]
	if !exists {
		return fmt.Errorf("功能标志不存在: %s", name)
	}

	if percentage < 0 || percentage > 100 {
		return fmt.Errorf("百分比必须在0-100之间: %d", percentage)
	}

	flag.Percentage = percentage
	flag.UpdatedAt = time.Now()

	return nil
}

// SetMetadata 设置功能标志的元数据
func (fm *FeatureFlagManager) SetMetadata(name FeatureFlagType, key string, value interface{}) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	flag, exists := fm.flags[name]
	if !exists {
		return fmt.Errorf("功能标志不存在: %s", name)
	}

	if flag.Metadata == nil {
		flag.Metadata = make(map[string]interface{})
	}

	flag.Metadata[key] = value
	flag.UpdatedAt = time.Now()

	return nil
}

// GetFlag 获取功能标志详情
func (fm *FeatureFlagManager) GetFlag(name FeatureFlagType) (*FeatureFlag, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	flag, exists := fm.flags[name]
	if !exists {
		return nil, fmt.Errorf("功能标志不存在: %s", name)
	}

	// 返回副本，避免外部修改
	flagCopy := *flag
	return &flagCopy, nil
}

// ListAllFlags 列出所有功能标志
func (fm *FeatureFlagManager) ListAllFlags() []FeatureFlag {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	flags := make([]FeatureFlag, 0, len(fm.flags))
	for _, flag := range fm.flags {
		flags = append(flags, *flag)
	}

	return flags
}

// SaveToFile 保存功能标志配置到文件
func (fm *FeatureFlagManager) SaveToFile(filePath string) error {
	fm.mu.RLock()
	flags := make([]FeatureFlag, 0, len(fm.flags))
	for _, flag := range fm.flags {
		flags = append(flags, *flag)
	}
	fm.mu.RUnlock()

	data, err := json.MarshalIndent(flags, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// LoadFromFile 从文件加载功能标志配置
func (fm *FeatureFlagManager) LoadFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	var flags []FeatureFlag
	if err := json.Unmarshal(data, &flags); err != nil {
		return fmt.Errorf("JSON反序列化失败: %w", err)
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	for i := range flags {
		fm.flags[flags[i].Name] = &flags[i]
	}

	return nil
}

// ===== 辅助函数 =====

// hashUserID 对userID进行哈希处理
func hashUserID(userID string) int {
	hash := 0
	for _, ch := range userID {
		hash = ((hash << 5) - hash) + int(ch)
		hash = hash & hash // 保持为32位整数
	}
	return abs(hash) % 100
}

// abs 返回绝对值
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
