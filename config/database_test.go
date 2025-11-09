package config

import (
	"nofx/crypto"
	"os"
	"testing"
)

// TestUpdateExchange_EmptyValuesShouldNotOverwrite 测试空值不应覆盖现有数据
// 这是 Bug 的核心：当前实现会用空字符串覆盖现有的私钥
func TestUpdateExchange_EmptyValuesShouldNotOverwrite(t *testing.T) {
	// 准备测试数据库
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-001"

	// 步骤 1: 创建初始配置（包含私钥）
	initialAPIKey := "initial-api-key-12345"
	initialSecretKey := "initial-secret-key-67890"

	err := db.UpdateExchange(
		userID,
		"hyperliquid",
		true, // enabled
		initialAPIKey,
		initialSecretKey,
		false, // testnet
		"0xWalletAddress",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 步骤 2: 验证初始数据已保存
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("获取配置失败: %v", err)
	}
	if len(exchanges) == 0 {
		t.Fatal("未找到配置")
	}

	// 解密后应该能看到原始值
	if exchanges[0].APIKey != initialAPIKey {
		t.Errorf("初始 APIKey 不正确，期望 %s，实际 %s", initialAPIKey, exchanges[0].APIKey)
	}

	// 步骤 3: 用空值更新（模拟前端发送空值的场景）
	// 🐛 Bug 重现：这应该 NOT 覆盖现有的私钥，但当前实现会覆盖
	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		false, // 只改变 enabled 状态
		"",    // 空 apiKey - 不应该覆盖
		"",    // 空 secretKey - 不应该覆盖
		true,  // 改变 testnet 状态
		"0xWalletAddress",
		"",
		"",
		"", // 空 aster_private_key - 不应该覆盖
	)
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 步骤 4: 验证私钥没有被空值覆盖
	exchanges, err = db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("获取更新后配置失败: %v", err)
	}

	// 🎯 关键断言：私钥应该保持不变
	if exchanges[0].APIKey != initialAPIKey {
		t.Errorf("❌ Bug 确认：APIKey 被空值覆盖了！期望 %s，实际 %s", initialAPIKey, exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != initialSecretKey {
		t.Errorf("❌ Bug 确认：SecretKey 被空值覆盖了！期望 %s，实际 %s", initialSecretKey, exchanges[0].SecretKey)
	}

	// 验证非敏感字段正常更新
	if exchanges[0].Enabled {
		t.Error("enabled 应该被更新为 false")
	}
	if !exchanges[0].Testnet {
		t.Error("testnet 应该被更新为 true")
	}
}

// TestUpdateExchange_AsterEmptyValuesShouldNotOverwrite 测试 Aster 私钥不被空值覆盖
func TestUpdateExchange_AsterEmptyValuesShouldNotOverwrite(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-002"

	// 步骤 1: 创建 Aster 配置
	initialAsterKey := "aster-private-key-xyz123"

	err := db.UpdateExchange(
		userID,
		"aster",
		true,
		"",
		"",
		false,
		"",
		"0xAsterUser",
		"0xAsterSigner",
		initialAsterKey,
	)
	if err != nil {
		t.Fatalf("初始化 Aster 失败: %v", err)
	}

	// 步骤 2: 用空值更新
	err = db.UpdateExchange(
		userID,
		"aster",
		false, // 只改 enabled
		"",
		"",
		false,
		"",
		"0xAsterUser",
		"0xAsterSigner",
		"", // 空 aster_private_key
	)
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 步骤 3: 验证 aster_private_key 没有被覆盖
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("获取配置失败: %v", err)
	}

	if exchanges[0].AsterPrivateKey != initialAsterKey {
		t.Errorf("❌ Bug 确认：AsterPrivateKey 被空值覆盖了！期望 %s，实际 %s", initialAsterKey, exchanges[0].AsterPrivateKey)
	}
}

// TestUpdateExchange_NonEmptyValuesShouldUpdate 测试非空值应该正常更新
func TestUpdateExchange_NonEmptyValuesShouldUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-003"

	// 步骤 1: 创建初始配置
	err := db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		"old-api-key",
		"old-secret-key",
		false,
		"0xOldWallet",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 步骤 2: 用非空值更新
	newAPIKey := "new-api-key-456"
	newSecretKey := "new-secret-key-789"

	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		newAPIKey,
		newSecretKey,
		false,
		"0xNewWallet",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 步骤 3: 验证新值已更新
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("获取配置失败: %v", err)
	}

	if exchanges[0].APIKey != newAPIKey {
		t.Errorf("APIKey 未更新，期望 %s，实际 %s", newAPIKey, exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != newSecretKey {
		t.Errorf("SecretKey 未更新，期望 %s，实际 %s", newSecretKey, exchanges[0].SecretKey)
	}
	if exchanges[0].HyperliquidWalletAddr != "0xNewWallet" {
		t.Errorf("WalletAddr 未更新")
	}
}


// TestUpdateExchange_PartialUpdateShouldWork 测试部分字段更新
func TestUpdateExchange_PartialUpdateShouldWork(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-005"

	// 创建初始配置
	err := db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		"api-key-123",
		"secret-key-456",
		false,
		"0xWallet1",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 只更新 enabled 和 testnet，私钥留空
	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		false,
		"", // 留空
		"", // 留空
		true,
		"0xWallet2",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("部分更新失败: %v", err)
	}

	// 验证
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("获取配置失败: %v", err)
	}

	// 私钥应该保持不变
	if exchanges[0].APIKey != "api-key-123" {
		t.Errorf("APIKey 不应改变，期望 api-key-123，实际 %s", exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != "secret-key-456" {
		t.Errorf("SecretKey 不应改变，期望 secret-key-456，实际 %s", exchanges[0].SecretKey)
	}

	// 其他字段应该更新
	if exchanges[0].Enabled {
		t.Error("enabled 应该更新为 false")
	}
	if !exchanges[0].Testnet {
		t.Error("testnet 应该更新为 true")
	}
	if exchanges[0].HyperliquidWalletAddr != "0xWallet2" {
		t.Error("wallet 地址应该更新")
	}
}

// TestUpdateExchange_MultipleExchangeTypes 测试不同交易所类型
func TestUpdateExchange_MultipleExchangeTypes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-006"

	testCases := []struct {
		exchangeID string
		name       string
		typ        string
	}{
		{"binance", "Binance Futures", "cex"},
		{"hyperliquid", "Hyperliquid", "dex"},
		{"aster", "Aster DEX", "dex"},
		{"unknown-exchange", "unknown-exchange Exchange", "cex"},
	}

	for _, tc := range testCases {
		t.Run(tc.exchangeID, func(t *testing.T) {
			err := db.UpdateExchange(
				userID,
				tc.exchangeID,
				true,
				"api-key-"+tc.exchangeID,
				"secret-key-"+tc.exchangeID,
				false,
				"",
				"",
				"",
				"",
			)
			if err != nil {
				t.Fatalf("创建 %s 失败: %v", tc.exchangeID, err)
			}

			// 验证创建成功
			exchanges, err := db.GetExchanges(userID)
			if err != nil {
				t.Fatalf("获取配置失败: %v", err)
			}

			found := false
			for _, ex := range exchanges {
				if ex.ID == tc.exchangeID {
					found = true
					if ex.Name != tc.name {
						t.Errorf("交易所名称不正确，期望 %s，实际 %s", tc.name, ex.Name)
					}
					if ex.Type != tc.typ {
						t.Errorf("交易所类型不正确，期望 %s，实际 %s", tc.typ, ex.Type)
					}
					if ex.APIKey != "api-key-"+tc.exchangeID {
						t.Errorf("APIKey 不正确")
					}
					break
				}
			}

			if !found {
				t.Errorf("未找到交易所 %s", tc.exchangeID)
			}
		})
	}
}

// TestUpdateExchange_MixedSensitiveFields 测试混合更新敏感和非敏感字段
func TestUpdateExchange_MixedSensitiveFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-007"

	// 创建初始配置
	err := db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		"old-api-key",
		"old-secret-key",
		false,
		"0xOldWallet",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 场景1: 只更新 apiKey，secretKey 留空
	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		false,
		"new-api-key",
		"", // 留空
		true,
		"0xNewWallet",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("更新1失败: %v", err)
	}

	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api-key" {
		t.Error("APIKey 应该更新")
	}
	if exchanges[0].SecretKey != "old-secret-key" {
		t.Error("SecretKey 应该保持不变")
	}

	// 场景2: 只更新 secretKey，apiKey 留空
	err = db.UpdateExchange(
		userID,
		"hyperliquid",
		true,
		"", // 留空
		"new-secret-key",
		false,
		"0xFinalWallet",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("更新2失败: %v", err)
	}

	exchanges, _ = db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api-key" {
		t.Error("APIKey 应该保持不变")
	}
	if exchanges[0].SecretKey != "new-secret-key" {
		t.Error("SecretKey 应该更新")
	}
	if exchanges[0].Enabled != true {
		t.Error("Enabled 应该更新为 true")
	}
	if exchanges[0].HyperliquidWalletAddr != "0xFinalWallet" {
		t.Error("WalletAddr 应该更新")
	}
}

// TestUpdateExchange_OnlyNonSensitiveFields 测试只更新非敏感字段
func TestUpdateExchange_OnlyNonSensitiveFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-008"

	// 创建初始配置（包含所有私钥）
	err := db.UpdateExchange(
		userID,
		"aster",
		true,
		"binance-api",
		"binance-secret",
		false,
		"",
		"0xUser1",
		"0xSigner1",
		"aster-private-key-1",
	)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 只更新非敏感字段（所有私钥字段留空）
	err = db.UpdateExchange(
		userID,
		"aster",
		false,
		"",
		"",
		true,
		"",
		"0xUser2",
		"0xSigner2",
		"",
	)
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 验证所有私钥保持不变
	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "binance-api" {
		t.Errorf("APIKey 应该保持不变，实际 %s", exchanges[0].APIKey)
	}
	if exchanges[0].SecretKey != "binance-secret" {
		t.Errorf("SecretKey 应该保持不变，实际 %s", exchanges[0].SecretKey)
	}
	if exchanges[0].AsterPrivateKey != "aster-private-key-1" {
		t.Errorf("AsterPrivateKey 应该保持不变，实际 %s", exchanges[0].AsterPrivateKey)
	}

	// 验证非敏感字段已更新
	if exchanges[0].Enabled != false {
		t.Error("Enabled 应该更新为 false")
	}
	if exchanges[0].Testnet != true {
		t.Error("Testnet 应该更新为 true")
	}
	if exchanges[0].AsterUser != "0xUser2" {
		t.Error("AsterUser 应该更新")
	}
	if exchanges[0].AsterSigner != "0xSigner2" {
		t.Error("AsterSigner 应该更新")
	}
}

// TestUpdateExchange_AllSensitiveFieldsUpdate 测试同时更新所有敏感字段
func TestUpdateExchange_AllSensitiveFieldsUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-009"

	// 创建初始配置
	err := db.UpdateExchange(
		userID,
		"binance",
		true,
		"old-api",
		"old-secret",
		false,
		"",
		"",
		"",
		"old-aster-key",
	)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	// 同时更新所有敏感字段
	err = db.UpdateExchange(
		userID,
		"binance",
		false,
		"new-api",
		"new-secret",
		true,
		"0xWallet",
		"0xUser",
		"0xSigner",
		"new-aster-key",
	)
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 验证所有字段都更新了
	exchanges, _ := db.GetExchanges(userID)
	if exchanges[0].APIKey != "new-api" {
		t.Error("APIKey 应该更新")
	}
	if exchanges[0].SecretKey != "new-secret" {
		t.Error("SecretKey 应该更新")
	}
	if exchanges[0].AsterPrivateKey != "new-aster-key" {
		t.Error("AsterPrivateKey 应该更新")
	}
	if !exchanges[0].Testnet {
		t.Error("Testnet 应该更新为 true")
	}
}

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) (*Database, func()) {
	// 创建临时数据库文件
	tmpFile := t.TempDir() + "/test.db"

	db, err := NewDatabase(tmpFile)
	if err != nil {
		t.Fatalf("创建测试数据库失败: %v", err)
	}

	// 创建测试用户
	testUsers := []string{"test-user-001", "test-user-002", "test-user-003", "test-user-004", "test-user-005", "test-user-006", "test-user-007", "test-user-008", "test-user-009"}
	for _, userID := range testUsers {
		user := &User{
			ID:           userID,
			Email:        userID + "@test.com",
			PasswordHash: "hash",
			OTPSecret:    "",
			OTPVerified:  false,
		}
		_ = db.CreateUser(user)
	}

	// 设置加密服务（用于测试加密功能）
	// 创建临时 RSA 密钥
	rsaKeyPath := t.TempDir() + "/test_rsa_key"
	cryptoService, err := crypto.NewCryptoService(rsaKeyPath)
	if err != nil {
		// 如果创建失败，继续测试但不使用加密
		t.Logf("警告：无法创建加密服务，将在无加密模式下测试: %v", err)
	} else {
		db.SetCryptoService(cryptoService)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpFile)
		os.RemoveAll(rsaKeyPath)
	}

	return db, cleanup
}
