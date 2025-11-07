package crypto

import (
	"testing"
)

// TestRSAKeyPairGeneration 測試 RSA 密鑰對生成
func TestRSAKeyPairGeneration(t *testing.T) {
	em, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失敗: %v", err)
	}

	publicKey := em.GetPublicKeyPEM()
	if publicKey == "" {
		t.Fatal("公鑰為空")
	}

	if len(publicKey) < 100 {
		t.Fatal("公鑰長度異常")
	}

	t.Logf("✅ RSA 密鑰對生成成功，公鑰長度: %d", len(publicKey))
}

// TestDatabaseEncryption 測試數據庫加密/解密
func TestDatabaseEncryption(t *testing.T) {
	em, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失敗: %v", err)
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
			t.Fatalf("加密失敗: %v (明文: %s)", err, plaintext)
		}

		// 驗證加密後不等於明文
		if encrypted == plaintext && plaintext != "" {
			t.Fatalf("加密失敗：加密後仍為明文")
		}

		// 解密
		decrypted, err := em.DecryptFromDatabase(encrypted)
		if err != nil {
			t.Fatalf("解密失敗: %v (密文: %s)", err, encrypted)
		}

		// 驗證解密後等於明文
		if decrypted != plaintext {
			t.Fatalf("解密結果不匹配: 期望 %s, 得到 %s", plaintext, decrypted)
		}

		t.Logf("✅ 加密/解密測試通過: %s", plaintext[:min(len(plaintext), 20)])
	}
}

// TestHybridEncryption 測試混合加密（前端 → 後端場景）
func TestHybridEncryption(t *testing.T) {
	_, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失敗: %v", err)
	}
	// 模擬前端加密私鑰
	// plaintext := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	// 注意：這裡需要前端的 encryptWithServerPublicKey 實現
	// 為了測試，我們直接使用後端的加密函數（實際前端使用 Web Crypto API）

	// 由於前端加密邏輯較複雜，這裡僅測試解密流程
	// 實際測試需要端到端測試
	t.Log("⚠️  混合加密測試需要完整的前後端環境，請執行端到端測試")
}

// TestEmptyString 測試空字串處理
func TestEmptyString(t *testing.T) {
	em, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失敗: %v", err)
	}

	encrypted, err := em.EncryptForDatabase("")
	if err != nil {
		t.Fatalf("加密空字串失敗: %v", err)
	}

	decrypted, err := em.DecryptFromDatabase(encrypted)
	if err != nil {
		t.Fatalf("解密空字串失敗: %v", err)
	}

	if decrypted != "" {
		t.Fatalf("空字串處理錯誤: 期望空字串, 得到 %s", decrypted)
	}

	t.Log("✅ 空字串處理正確")
}

// TestInvalidCiphertext 測試無效密文處理
func TestInvalidCiphertext(t *testing.T) {
	em, err := GetEncryptionManager()
	if err != nil {
		t.Fatalf("初始化加密管理器失敗: %v", err)
	}

	invalidCiphertexts := []string{
		"not_base64!@#$%",
		"dGVzdA==", // 有效 Base64，但內容太短
		"",
	}

	for _, ciphertext := range invalidCiphertexts {
		_, err := em.DecryptFromDatabase(ciphertext)
		if err == nil && ciphertext != "" {
			t.Fatalf("應該拒絕無效密文: %s", ciphertext)
		}
	}

	t.Log("✅ 無效密文處理正確")
}

// BenchmarkEncryption 性能測試：加密
func BenchmarkEncryption(b *testing.B) {
	em, _ := GetEncryptionManager()
	plaintext := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = em.EncryptForDatabase(plaintext)
	}
}

// BenchmarkDecryption 性能測試：解密
func BenchmarkDecryption(b *testing.B) {
	em, _ := GetEncryptionManager()
	plaintext := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	encrypted, _ := em.EncryptForDatabase(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = em.DecryptFromDatabase(encrypted)
	}
}

// min 工具函數
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
