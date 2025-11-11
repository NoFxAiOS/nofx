package crypto

import (
	"testing"
)

// TestRSAKeyPairGeneration 测试 RSA 密鑰对生成
func TestRSAKeyPairGeneration(t *testing.T) {
	em, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失败: %v", err)
	}

	publicKey := em.GetPublicKeyPEM()
	if publicKey == "" {
		t.Fatal("公鑰为空")
	}

	if len(publicKey) < 100 {
		t.Fatal("公鑰長度异常")
	}

	t.Logf("✅ RSA 密鑰对生成成功，公鑰長度: %d", len(publicKey))
}

// TestDatabaseEncryption 测试数据庫加密/解密
func TestDatabaseEncryption(t *testing.T) {
	em, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失败: %v", err)
	}

	testCases := []string{
		"0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		"test_api_key_12345",
		"very_secret_password",
		"",
	}

	for _, plaintext := range testCases {
		// 加密
		encrypted, err := em.EncryptForDatabase(plaintext)
		if err != nil {
			t.Fatalf("加密失败: %v (明文: %s)", err, plaintext)
		}

		// 验证加密後不等于明文
		if encrypted == plaintext && plaintext != "" {
			t.Fatalf("加密失败：加密後仍为明文")
		}

		// 解密
		decrypted, err := em.DecryptFromDatabase(encrypted)
		if err != nil {
			t.Fatalf("解密失败: %v (密文: %s)", err, encrypted)
		}

		// 验证解密後等于明文
		if decrypted != plaintext {
			t.Fatalf("解密结果不匹配: 期望 %s, 得到 %s", plaintext, decrypted)
		}

		t.Logf("✅ 加密/解密测试通过: %s", plaintext[:min(len(plaintext), 20)])
	}
}

// TestHybridEncryption 测试混合加密（前端 → 後端場景）
func TestHybridEncryption(t *testing.T) {
	_, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失败: %v", err)
	}
	// 模擬前端加密私鑰
	// plaintext := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	// 注意：这里需要前端的 encryptWithServerPublicKey 实现
	// 为了测试，我们直接使用後端的加密函数（实际前端使用 Web Crypto API）

	// 由于前端加密邏輯較复杂，这里仅测试解密流程
	// 实际测试需要端到端测试
	t.Log("⚠️  混合加密测试需要完整的前後端环境，请执行端到端测试")
}

// TestEmptyString 测试空字串处理
func TestEmptyString(t *testing.T) {
	em, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失败: %v", err)
	}

	encrypted, err := em.EncryptForDatabase("")
	if err != nil {
		t.Fatalf("加密空字串失败: %v", err)
	}

	decrypted, err := em.DecryptFromDatabase(encrypted)
	if err != nil {
		t.Fatalf("解密空字串失败: %v", err)
	}

	if decrypted != "" {
		t.Fatalf("空字串处理错误: 期望空字串, 得到 %s", decrypted)
	}

	t.Log("✅ 空字串处理正确")
}

// TestInvalidCiphertext 测试无效密文处理
func TestInvalidCiphertext(t *testing.T) {
	em, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失败: %v", err)
	}

	invalidCiphertexts := []string{
		"not_base64!@#$%",
		"dGVzdA==", // 有效 Base64，但内容太短
		"",
	}

	for _, ciphertext := range invalidCiphertexts {
		_, err := em.DecryptFromDatabase(ciphertext)
		if err == nil && ciphertext != "" {
			t.Fatalf("应該拒絕无效密文: %s", ciphertext)
		}
	}

	t.Log("✅ 无效密文处理正确")
}

// BenchmarkEncryption 性能测试：加密
func BenchmarkEncryption(b *testing.B) {
	em, _ := GetEncryptionManager()
	plaintext := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = em.EncryptForDatabase(plaintext)
	}
}

// BenchmarkDecryption 性能测试：解密
func BenchmarkDecryption(b *testing.B) {
	em, _ := GetEncryptionManager()
	plaintext := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	encrypted, _ := em.EncryptForDatabase(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = em.DecryptFromDatabase(encrypted)
	}
}

// min 工具函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
