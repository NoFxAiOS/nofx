package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetAIModels 测试获取AI模型配置
func TestGetAIModels(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 获取default用户的AI模型
	models, err := tdb.db.GetAIModels("default")
	assert.NoError(t, err, "Should be able to get AI models")
	assert.NotNil(t, models, "Models should not be nil")
	assert.IsType(t, []*AIModelConfig{}, models, "Should return slice, not nil")
	assert.GreaterOrEqual(t, len(models), 2, "Should have at least 2 models")
}

// TestGetAIModelsEmptyUser 测试获取不存在用户的AI模型
func TestGetAIModelsEmptyUser(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	models, err := tdb.db.GetAIModels("test_nonexistent_user")
	assert.NoError(t, err, "Should not error for nonexistent user")
	assert.NotNil(t, models, "Should return empty slice, not nil")
	assert.Equal(t, 0, len(models), "Should return empty slice")
}

// TestUpdateAIModel 测试更新AI模型配置
func TestUpdateAIModel(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_ai",
		Email:        "testai@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 更新AI模型（应该创建新的）
	err = tdb.db.UpdateAIModel("test_user_ai", "deepseek", true, "test_api_key", "https://api.test.com", "custom-model")
	assert.NoError(t, err, "Should be able to update AI model")

	// 验证已创建
	models, err := tdb.db.GetAIModels("test_user_ai")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(models), "Should have 1 model")
	assert.Equal(t, "deepseek", models[0].Provider)
	assert.True(t, models[0].Enabled)
	assert.Equal(t, "test_api_key", models[0].APIKey)
}

// TestUpdateAIModelExisting 测试更新已存在的AI模型
func TestUpdateAIModelExisting(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_ai2",
		Email:        "testai2@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 第一次创建
	err = tdb.db.UpdateAIModel("test_user_ai2", "qwen", false, "key1", "", "")
	require.NoError(t, err)

	// 第二次更新
	err = tdb.db.UpdateAIModel("test_user_ai2", "qwen", true, "key2", "https://new.api.com", "new-model")
	assert.NoError(t, err, "Should be able to update existing model")

	// 验证已更新
	models, err := tdb.db.GetAIModels("test_user_ai2")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(models), "Should still have 1 model")
	assert.True(t, models[0].Enabled, "Should be enabled")
	assert.Equal(t, "key2", models[0].APIKey, "API key should be updated")
}

// TestGetExchanges 测试获取交易所配置
func TestGetExchanges(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 获取default用户的交易所
	exchanges, err := tdb.db.GetExchanges("default")
	assert.NoError(t, err, "Should be able to get exchanges")
	assert.NotNil(t, exchanges, "Exchanges should not be nil")
	assert.IsType(t, []*ExchangeConfig{}, exchanges, "Should return slice, not nil")
	assert.GreaterOrEqual(t, len(exchanges), 4, "Should have at least 4 exchanges")
}

// TestGetExchangesEmptyUser 测试获取不存在用户的交易所
func TestGetExchangesEmptyUser(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	exchanges, err := tdb.db.GetExchanges("test_nonexistent_user")
	assert.NoError(t, err, "Should not error for nonexistent user")
	assert.NotNil(t, exchanges, "Should return empty slice, not nil")
	assert.Equal(t, 0, len(exchanges), "Should return empty slice")
}

// TestUpdateExchange 测试更新交易所配置
func TestUpdateExchange(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_ex",
		Email:        "testex@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 更新交易所（应该创建新的）
	err = tdb.db.UpdateExchange("test_user_ex", "binance", true, "api_key", "secret_key", false, "", "", "", "", "")
	assert.NoError(t, err, "Should be able to update exchange")

	// 验证已创建
	exchanges, err := tdb.db.GetExchanges("test_user_ex")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(exchanges), "Should have 1 exchange")
	assert.Equal(t, "binance", exchanges[0].ID)
	assert.True(t, exchanges[0].Enabled)
	assert.Equal(t, "api_key", exchanges[0].APIKey)
}

// TestUpdateExchangeOKX 测试更新OKX交易所
func TestUpdateExchangeOKX(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_okx",
		Email:        "testokx@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 更新OKX交易所
	err = tdb.db.UpdateExchange("test_user_okx", "okx", true, "okx_api_key", "okx_secret", false, "", "", "", "", "okx_passphrase")
	assert.NoError(t, err, "Should be able to update OKX exchange")

	// 验证已创建
	exchanges, err := tdb.db.GetExchanges("test_user_okx")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(exchanges), "Should have 1 exchange")

	okx := exchanges[0]
	assert.Equal(t, "okx", okx.ID)
	assert.Equal(t, "cex", okx.Type, "OKX type should be 'cex'")
	assert.Equal(t, "okx_passphrase", okx.OKXPassphrase, "Should have passphrase")
}

// TestUpdateExchangeHyperliquid 测试更新Hyperliquid交易所
func TestUpdateExchangeHyperliquid(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_hl",
		Email:        "testhl@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 更新Hyperliquid交易所
	err = tdb.db.UpdateExchange("test_user_hl", "hyperliquid", true, "private_key", "", true, "0x1234567890", "", "", "", "")
	assert.NoError(t, err, "Should be able to update Hyperliquid exchange")

	// 验证已创建
	exchanges, err := tdb.db.GetExchanges("test_user_hl")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(exchanges), "Should have 1 exchange")

	hl := exchanges[0]
	assert.Equal(t, "hyperliquid", hl.ID)
	assert.Equal(t, "dex", hl.Type, "Hyperliquid type should be 'dex'")
	assert.Equal(t, "0x1234567890", hl.HyperliquidWalletAddr, "Should have wallet address")
	assert.True(t, hl.Testnet, "Should be testnet")
}

// TestCreateAIModel 测试创建AI模型
func TestCreateAIModel(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_create_ai",
		Email:        "testcreateai@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 创建AI模型
	err = tdb.db.CreateAIModel("test_user_create_ai", "custom_model", "Custom Model", "custom", true, "custom_key", "https://custom.api")
	assert.NoError(t, err, "Should be able to create AI model")

	// 验证已创建
	models, err := tdb.db.GetAIModels("test_user_create_ai")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(models), "Should have 1 model")
	assert.Equal(t, "custom_model", models[0].ID)
	assert.Equal(t, "Custom Model", models[0].Name)
}

// TestCreateExchange 测试创建交易所
func TestCreateExchange(t *testing.T) {
	tdb := setupTestDB(t)
	defer tdb.teardown(t)

	// 创建测试用户
	now := time.Now()
	user := &User{
		ID:           "test_user_create_ex",
		Email:        "testcreateex@example.com",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := tdb.db.CreateUser(user)
	require.NoError(t, err)

	// 创建交易所
	err = tdb.db.CreateExchange("test_user_create_ex", "custom_exchange", "Custom Exchange", "cex", true, "api", "secret", false, "", "", "", "")
	assert.NoError(t, err, "Should be able to create exchange")

	// 验证已创建
	exchanges, err := tdb.db.GetExchanges("test_user_create_ex")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(exchanges), "Should have 1 exchange")
	assert.Equal(t, "custom_exchange", exchanges[0].ID)
	assert.Equal(t, "Custom Exchange", exchanges[0].Name)
}
