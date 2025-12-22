package database

import (
	"database/sql"
	"testing"
	"time"
)

func TestUserNewsConfigRepository_CreateAndGet(t *testing.T) {
	// 使用内存数据库进行测试
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserNewsConfigRepository(db)

	// 创建配置
	config := &UserNewsConfig{
		UserID:                  "test-user-123",
		Enabled:                 true,
		NewsSources:             "mlion,twitter",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:     10,
		SentimentThreshold:      0.5,
	}

	err := repo.Create(config)
	if err != nil {
		t.Errorf("创建配置失败: %v", err)
	}

	// 获取配置
	retrieved, err := repo.GetByUserID("test-user-123")
	if err != nil {
		t.Errorf("获取配置失败: %v", err)
	}

	// 验证字段
	if retrieved.UserID != "test-user-123" {
		t.Errorf("期望UserID为test-user-123，得到%s", retrieved.UserID)
	}
	if !retrieved.Enabled {
		t.Error("期望启用为true")
	}
	if retrieved.NewsSources != "mlion,twitter" {
		t.Errorf("期望NewsSources为mlion,twitter，得到%s", retrieved.NewsSources)
	}
}

func TestUserNewsConfigRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserNewsConfigRepository(db)

	// 创建配置
	config := &UserNewsConfig{
		UserID:                  "test-user-456",
		Enabled:                 true,
		NewsSources:             "mlion",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:     10,
		SentimentThreshold:      0.0,
	}

	err := repo.Create(config)
	if err != nil {
		t.Fatalf("创建配置失败: %v", err)
	}

	// 更新配置
	config.Enabled = false
	config.NewsSources = "twitter,reddit"
	config.AutoFetchIntervalMinutes = 10
	config.MaxArticlesPerFetch = 20

	err = repo.Update(config)
	if err != nil {
		t.Errorf("更新配置失败: %v", err)
	}

	// 验证更新
	retrieved, err := repo.GetByUserID("test-user-456")
	if err != nil {
		t.Fatalf("获取配置失败: %v", err)
	}

	if retrieved.Enabled {
		t.Error("期望启用为false")
	}
	if retrieved.NewsSources != "twitter,reddit" {
		t.Errorf("期望NewsSources为twitter,reddit，得到%s", retrieved.NewsSources)
	}
	if retrieved.AutoFetchIntervalMinutes != 10 {
		t.Errorf("期望AutoFetchIntervalMinutes为10，得到%d", retrieved.AutoFetchIntervalMinutes)
	}
}

func TestUserNewsConfigRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserNewsConfigRepository(db)

	// 创建配置
	config := &UserNewsConfig{
		UserID:                  "test-user-789",
		Enabled:                 true,
		NewsSources:             "mlion",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:     10,
		SentimentThreshold:      0.0,
	}

	err := repo.Create(config)
	if err != nil {
		t.Fatalf("创建配置失败: %v", err)
	}

	// 删除配置
	err = repo.Delete("test-user-789")
	if err != nil {
		t.Errorf("删除配置失败: %v", err)
	}

	// 验证删除
	_, err = repo.GetByUserID("test-user-789")
	if err == nil {
		t.Error("期望配置已删除，但仍然存在")
	}
}

func TestUserNewsConfigRepository_GetEnabledNewsSources(t *testing.T) {
	config := &UserNewsConfig{
		UserID:      "test-user",
		NewsSources: "mlion, twitter , reddit",
	}

	sources := config.GetEnabledNewsSources()
	if len(sources) != 3 {
		t.Errorf("期望3个新闻源，得到%d个", len(sources))
	}

	expected := []string{"mlion", "twitter", "reddit"}
	for i, source := range sources {
		if source != expected[i] {
			t.Errorf("期望%s，得到%s", expected[i], source)
		}
	}
}

func TestUserNewsConfigRepository_SetNewsSources(t *testing.T) {
	config := &UserNewsConfig{
		UserID: "test-user",
	}

	sources := []string{"mlion", "twitter", "reddit"}
	config.SetNewsSources(sources)

	if config.NewsSources != "mlion,twitter,reddit" {
		t.Errorf("期望mlion,twitter,reddit，得到%s", config.NewsSources)
	}
}

func TestUserNewsConfigRepository_GetOrCreateDefault(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserNewsConfigRepository(db)

	// 第一次调用应该创建默认配置
	config, err := repo.GetOrCreateDefault("new-user-001")
	if err != nil {
		t.Errorf("获取或创建默认配置失败: %v", err)
	}

	if !config.Enabled {
		t.Error("期望默认配置为启用")
	}
	if config.NewsSources != "mlion" {
		t.Errorf("期望默认新闻源为mlion，得到%s", config.NewsSources)
	}
	if config.AutoFetchIntervalMinutes != 5 {
		t.Errorf("期望默认抓取间隔为5分钟，得到%d分钟", config.AutoFetchIntervalMinutes)
	}

	// 第二次调用应该返回相同的配置
	config2, err := repo.GetOrCreateDefault("new-user-001")
	if err != nil {
		t.Errorf("第二次获取配置失败: %v", err)
	}

	if config.ID != config2.ID {
		t.Error("期望返回相同的配置ID")
	}
}

func TestUserNewsConfigRepository_ListAllEnabled(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserNewsConfigRepository(db)

	// 创建多个配置
	configs := []UserNewsConfig{
		{
			UserID:                  "user-1",
			Enabled:                 true,
			NewsSources:             "mlion",
			AutoFetchIntervalMinutes: 5,
			MaxArticlesPerFetch:     10,
			SentimentThreshold:      0.0,
		},
		{
			UserID:                  "user-2",
			Enabled:                 false,
			NewsSources:             "mlion",
			AutoFetchIntervalMinutes: 5,
			MaxArticlesPerFetch:     10,
			SentimentThreshold:      0.0,
		},
		{
			UserID:                  "user-3",
			Enabled:                 true,
			NewsSources:             "twitter",
			AutoFetchIntervalMinutes: 10,
			MaxArticlesPerFetch:     20,
			SentimentThreshold:      0.5,
		},
	}

	for i := range configs {
		err := repo.Create(&configs[i])
		if err != nil {
			t.Fatalf("创建配置失败: %v", err)
		}
	}

	// 列出启用的配置
	enabled, err := repo.ListAllEnabled()
	if err != nil {
		t.Errorf("列出启用的配置失败: %v", err)
	}

	// 应该有2个启用的配置
	if len(enabled) != 2 {
		t.Errorf("期望2个启用的配置，得到%d个", len(enabled))
	}

	// 验证只有启用的配置被返回
	for _, config := range enabled {
		if !config.Enabled {
			t.Error("列出的配置应该都是启用的")
		}
	}
}

// 辅助函数：设置测试数据库
func setupTestDB(t *testing.T) *sql.DB {
	// 这里需要根据项目的实际数据库设置来实现
	// 通常是创建一个临时的SQLite数据库并运行迁移脚本
	// 为了简化，这里返回nil，实际项目中需要补充实现
	t.Skip("需要根据项目的数据库配置补充实现")
	return nil
}
