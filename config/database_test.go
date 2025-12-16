package config

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabase 测试数据库结构
type TestDatabase struct {
	db *Database
}

// setupTestDB 创建测试数据库实例
func setupTestDB(t *testing.T) *TestDatabase {
	// 使用内存SQLite数据库进行测试
	// 注意：由于当前代码只支持PostgreSQL，我们需要使用测试PostgreSQL
	// 或者修改NewDatabase以支持测试模式

	// 检查是否有TEST_DATABASE_URL环境变量
	testDBURL := os.Getenv("TEST_DATABASE_URL")
	if testDBURL == "" {
		// 如果没有测试数据库，跳过测试
		t.Skip("TEST_DATABASE_URL not set, skipping database tests")
	}

	// 临时设置DATABASE_URL
	originalURL := os.Getenv("DATABASE_URL")
	os.Setenv("DATABASE_URL", testDBURL)
	defer os.Setenv("DATABASE_URL", originalURL)

	db, err := NewDatabase("")
	require.NoError(t, err, "Failed to create test database")
	require.NotNil(t, db, "Database should not be nil")

	// 清理测试数据
	cleanupTestData(t, db)

	return &TestDatabase{db: db}
}

// cleanupTestData 清理测试数据
func cleanupTestData(t *testing.T, db *Database) {
	// 删除测试数据（保留default用户和系统配置）
	tables := []string{
		"DELETE FROM password_resets WHERE user_id LIKE 'test_%'",
		"DELETE FROM login_attempts WHERE email LIKE 'test_%'",
		"DELETE FROM audit_logs WHERE user_id LIKE 'test_%'",
		"DELETE FROM traders WHERE user_id LIKE 'test_%'",
		"DELETE FROM user_signal_sources WHERE user_id LIKE 'test_%'",
		"DELETE FROM exchanges WHERE user_id LIKE 'test_%'",
		"DELETE FROM ai_models WHERE user_id LIKE 'test_%'",
		"DELETE FROM users WHERE id LIKE 'test_%'",
	}

	for _, query := range tables {
		_, err := db.exec(query)
		if err != nil {
			t.Logf("Warning: cleanup query failed: %v", err)
		}
	}
}

// teardownTestDB 清理测试数据库
func (tdb *TestDatabase) teardown(t *testing.T) {
	if tdb.db != nil {
		cleanupTestData(t, tdb.db)
		tdb.db.Close()
	}
}

// TestDatabaseConnection 测试数据库连接
func TestDatabaseConnection(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	assert.NotNil(t, tdb.db, "Database connection should be established")
	assert.NotNil(t, tdb.db.db, "Underlying sql.DB should not be nil")
}

// TestDatabasePing 测试数据库Ping
func TestDatabasePing(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	err := tdb.db.db.Ping()
	assert.NoError(t, err, "Database ping should succeed")
}

// TestCreateTables 测试表创建
func TestCreateTables(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 验证所有必需的表都存在
	tables := []string{
		"users",
		"ai_models",
		"exchanges",
		"traders",
		"system_config",
		"password_resets",
		"login_attempts",
		"audit_logs",
		"beta_codes",
		"user_signal_sources",
	}

	for _, table := range tables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)`
		err := tdb.db.db.QueryRow(query, table).Scan(&exists)
		assert.NoError(t, err, "Failed to check table existence: %s", table)
		assert.True(t, exists, "Table should exist: %s", table)
	}
}

// TestDefaultUserExists 测试默认用户是否存在
func TestDefaultUserExists(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	user, err := tdb.db.GetUserByID("default")
	assert.NoError(t, err, "Should be able to get default user")
	assert.NotNil(t, user, "Default user should exist")
	assert.Equal(t, "default", user.ID)
	assert.Equal(t, "default@system", user.Email)
}

// TestDefaultAIModelsExist 测试默认AI模型是否存在
func TestDefaultAIModelsExist(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	models, err := tdb.db.GetAIModels("default")
	assert.NoError(t, err, "Should be able to get AI models")
	assert.NotNil(t, models, "AI models should not be nil")
	assert.GreaterOrEqual(t, len(models), 2, "Should have at least 2 default AI models")

	// 验证DeepSeek和Qwen存在
	modelMap := make(map[string]bool)
	for _, model := range models {
		modelMap[model.Provider] = true
	}
	assert.True(t, modelMap["deepseek"], "DeepSeek model should exist")
	assert.True(t, modelMap["qwen"], "Qwen model should exist")
}

// TestDefaultExchangesExist 测试默认交易所是否存在
func TestDefaultExchangesExist(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	exchanges, err := tdb.db.GetExchanges("default")
	assert.NoError(t, err, "Should be able to get exchanges")
	assert.NotNil(t, exchanges, "Exchanges should not be nil")
	assert.GreaterOrEqual(t, len(exchanges), 4, "Should have at least 4 default exchanges")

	// 验证所有交易所存在
	exchangeMap := make(map[string]*ExchangeConfig)
	for _, exchange := range exchanges {
		exchangeMap[exchange.ID] = exchange
	}

	assert.NotNil(t, exchangeMap["binance"], "Binance should exist")
	assert.NotNil(t, exchangeMap["hyperliquid"], "Hyperliquid should exist")
	assert.NotNil(t, exchangeMap["aster"], "Aster should exist")
	assert.NotNil(t, exchangeMap["okx"], "OKX should exist")

	// 验证OKX类型正确
	okx := exchangeMap["okx"]
	assert.Equal(t, "cex", okx.Type, "OKX type should be 'cex'")
}

// TestSystemConfigExists 测试系统配置是否存在
func TestSystemConfigExists(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	configs := []string{
		"admin_mode",
		"beta_mode",
		"default_coins",
		"btc_eth_leverage",
		"altcoin_leverage",
	}

	for _, key := range configs {
		value, err := tdb.db.GetSystemConfig(key)
		assert.NoError(t, err, "Should be able to get system config: %s", key)
		assert.NotEmpty(t, value, "System config should not be empty: %s", key)
	}
}

// TestCreateUser 测试创建用户
func TestCreateUser(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	now := time.Now()
	user := &User{
		ID:             "test_user_1",
		Email:          "test1@example.com",
		PasswordHash:   "hashed_password",
		OTPSecret:      "otp_secret",
		OTPVerified:    false,
		IsActive:       true,
		IsAdmin:        false,
		FailedAttempts: 0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err := tdb.db.CreateUser(user)
	assert.NoError(t, err, "Should be able to create user")

	// 验证用户已创建
	retrievedUser, err := tdb.db.GetUserByEmail("test1@example.com")
	assert.NoError(t, err, "Should be able to get user by email")
	assert.NotNil(t, retrievedUser, "Retrieved user should not be nil")
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Email, retrievedUser.Email)
}

// TestCreateUserDuplicateEmail 测试创建重复邮箱用户
func TestCreateUserDuplicateEmail(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	now := time.Now()
	user1 := &User{
		ID:           "test_user_2",
		Email:        "test2@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err := tdb.db.CreateUser(user1)
	require.NoError(t, err, "First user creation should succeed")

	user2 := &User{
		ID:           "test_user_3",
		Email:        "test2@example.com", // 相同邮箱
		PasswordHash: "hashed_password",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = tdb.db.CreateUser(user2)
	assert.Error(t, err, "Should not be able to create user with duplicate email")
}

// TestGetUserByID 测试通过ID获取用户
func TestGetUserByID(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_4",
		Email:        "test4@example.com",
		PasswordHash: "hashed_password",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 通过ID获取
	retrievedUser, err := tdb.db.GetUserByID("test_user_4")
	assert.NoError(t, err, "Should be able to get user by ID")
	assert.NotNil(t, retrievedUser, "User should exist")
	assert.Equal(t, user.Email, retrievedUser.Email)
}

// TestGetUserByIDNotFound 测试获取不存在的用户
func TestGetUserByIDNotFound(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	user, err := tdb.db.GetUserByID("nonexistent_user")
	assert.Error(t, err, "Should return error for nonexistent user")
	assert.Equal(t, sql.ErrNoRows, err, "Should return sql.ErrNoRows")
	assert.Nil(t, user, "User should be nil")
}

// TestSetAndGetSystemConfig 测试系统配置的设置和获取
func TestSetAndGetSystemConfig(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 设置配置
	err := tdb.db.SetSystemConfig("test_key", "test_value")
	assert.NoError(t, err, "Should be able to set system config")

	// 获取配置
	value, err := tdb.db.GetSystemConfig("test_key")
	assert.NoError(t, err, "Should be able to get system config")
	assert.Equal(t, "test_value", value, "Config value should match")

	// 更新配置
	err = tdb.db.SetSystemConfig("test_key", "updated_value")
	assert.NoError(t, err, "Should be able to update system config")

	// 验证更新
	value, err = tdb.db.GetSystemConfig("test_key")
	assert.NoError(t, err, "Should be able to get updated config")
	assert.Equal(t, "updated_value", value, "Config value should be updated")
}

// TestGetSystemConfigNotFound 测试获取不存在的配置
func TestGetSystemConfigNotFound(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	value, err := tdb.db.GetSystemConfig("nonexistent_key")
	assert.NoError(t, err, "Should not return error for nonexistent key")
	assert.Empty(t, value, "Value should be empty string")
}

// TestPlaceholderConversion 测试SQL占位符转换
func TestPlaceholderConversion(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single placeholder",
			input:    "SELECT * FROM users WHERE id = ?",
			expected: "SELECT * FROM users WHERE id = $1",
		},
		{
			name:     "Multiple placeholders",
			input:    "INSERT INTO users (id, email, password) VALUES (?, ?, ?)",
			expected: "INSERT INTO users (id, email, password) VALUES ($1, $2, $3)",
		},
		{
			name:     "No placeholders",
			input:    "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tdb.db.convertPlaceholders(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDatabaseClose 测试数据库关闭
func TestDatabaseClose(t *testing.T) {
	tdb := setupTestDB(t)

	err := tdb.db.Close()
	assert.NoError(t, err, "Should be able to close database")

	// 验证连接已关闭（尝试ping应该失败）
	err = tdb.db.db.Ping()
	assert.Error(t, err, "Ping should fail after close")
}
