package web3_auth

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============ 测试辅助函数 ============

// generateTestKeyPair 生成测试用的密钥对
func generateTestKeyPair() (*ecdsa.PrivateKey, string, error) {
	privateKey, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return nil, "", err
	}

	publicKey := privateKey.PublicKey
	address := crypto.PubkeyToAddress(publicKey).Hex()

	return privateKey, address, nil
}

// signMessage 使用私钥签名消息
func signMessage(privateKey *ecdsa.PrivateKey, message string) (string, error) {
	// 生成EIP-191兼容的消息哈希
	msgHash := generateMessageHash(message)

	signature, err := crypto.Sign(msgHash, privateKey)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(signature), nil
}

// ============ 签名恢复测试 ============

// TestRecoverAddressFromSignature_Valid 有效的签名测试
func TestRecoverAddressFromSignature_Valid(t *testing.T) {
	// 生成测试密钥对
	privateKey, address, err := generateTestKeyPair()
	require.NoError(t, err)

	// 生成测试nonce和过期时间
	nonce, err := GenerateNonce()
	require.NoError(t, err)

	expiresAt := time.Now().Add(10 * time.Minute)

	// 生成签名消息
	message := GenerateSignatureMessage(address, nonce, expiresAt)

	// 签名消息
	signature, err := signMessage(privateKey, message)
	require.NoError(t, err)

	// 测试恢复地址
	recoveredAddr, err := RecoverAddressFromSignature(message, signature, address)
	require.NoError(t, err)

	// 验证恢复的地址与原始地址匹配（不区分大小写）
	assert.Equal(t, address, recoveredAddr)
}

// TestRecoverAddressFromSignature_InvalidSignature 无效签名测试
func TestRecoverAddressFromSignature_InvalidSignature(t *testing.T) {
	address := "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0"
	message := "Test message"

	// 测试空签名
	_, err := RecoverAddressFromSignature(message, "", address)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的签名格式")

	// 测试无效格式的签名
	_, err = RecoverAddressFromSignature(message, "0x1234", address)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "签名长度无效")

	// 测试错误的签名
	wrongSignature := "0x" + "ff" * 65
	_, err = RecoverAddressFromSignature(message, wrongSignature, address)
	assert.Error(t, err)
}

// TestRecoverAddressFromSignature_AddressMismatch 地址不匹配测试
func TestRecoverAddressFromSignature_AddressMismatch(t *testing.T) {
	// 生成两个不同的密钥对
	privateKey1, address1, err := generateTestKeyPair()
	require.NoError(t, err)

	_, address2, err := generateTestKeyPair()
	require.NoError(t, err)

	nonce, err := GenerateNonce()
	require.NoError(t, err)

	expiresAt := time.Now().Add(10 * time.Minute)
	message := GenerateSignatureMessage(address1, nonce, expiresAt)

	// 使用第一个私钥签名
	signature, err := signMessage(privateKey1, message)
	require.NoError(t, err)

	// 使用第二个地址验证（应该失败）
	_, err = RecoverAddressFromSignature(message, signature, address2)
	assert.Error(t, err)
	assert.Equal(t, ErrAddressMismatch, err)
}

// ============ 验证函数测试 ============

// TestValidateAddress 验证地址测试
func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{"有效地址", "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0", false},
		{"有效地址(大写)", "0x742D35CC6634C0532925A3B8D4D9F4BF1E68E9E0", false},
		{"无效长度-太短", "0x742d35Cc4", true},
		{"无效长度-太长", "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E000", true},
		{"不含0x前缀", "742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0", false}, // 自动添加前缀
		{"空地址", "", true},
		{"无效字符", "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9ZZ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.address)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateSignature 验证签名测试
func TestValidateSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		wantErr   bool
	}{
		{"有效签名", "0x" + "ff" * 130, false},
		{"有效签名(无前缀)", "ff" * 130, false}, // 自动添加前缀
		{"空签名", "", true},
		{"太短", "0x" + "ff" * 10, true},
		{"太长", "0x" + "ff" * 200, true},
		{"无效字符", "0xgg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSignature(tt.signature)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateWalletType 验证钱包类型测试
func TestValidateWalletType(t *testing.T) {
	tests := []struct {
		name       string
		walletType string
		wantErr    bool
	}{
		{"MetaMask", "metamask", false},
		{"TP钱包", "tp", false},
		{"其他", "other", false},
		{"无效类型", "wallet123", true},
		{"空类型", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWalletType(tt.walletType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============ Nonce测试 ============

// TestGenerateNonce 生成nonce测试
func TestGenerateNonce(t *testing.T) {
	nonce1, err := GenerateNonce()
	require.NoError(t, err)
	assert.Len(t, nonce1, 64) // 32字节hex编码
	assert.NotEmpty(t, nonce1)

	// 确保随机性
	nonce2, err := GenerateNonce()
	require.NoError(t, err)
	assert.NotEqual(t, nonce1, nonce2)

	// 验证格式
	err = ValidateNonce(nonce1)
	assert.NoError(t, err)

	err = ValidateNonce(nonce2)
	assert.NoError(t, err)
}

// TestValidateNonce_Valid 验证有效nonce测试
func TestValidateNonce_Valid(t *testing.T) {
	nonce, err := GenerateNonce()
	require.NoError(t, err)

	err = ValidateNonce(nonce)
	assert.NoError(t, err)
}

// TestValidateNonce_Invalid 验证无效nonce测试
func TestValidateNonce_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		nonce string
		want  bool
	}{
		{"空nonce", "", true},
		{"太短", "123", true},
		{"无效字符", "12345g67890abcdef12345g67890abcdef12345g67890abcdef12345g67890abcdef", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonce(tt.nonce)
			if tt.want {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCheckNonceExpiration 检查nonce过期测试
func TestCheckNonceExpiration(t *testing.T) {
	// 有效的nonce
	validExpiresAt := time.Now().Add(10 * time.Minute)
	err := CheckNonceExpiration(validExpiresAt)
	assert.NoError(t, err)

	// 已过期的nonce
	expiredExpiresAt := time.Now().Add(-1 * time.Minute)
	err = CheckNonceExpiration(expiredExpiresAt)
	assert.Error(t, err)
	assert.Equal(t, ErrNonceExpired, err)
}

// ============ 消息生成测试 ============

// TestGenerateSignatureMessage 生成签名消息测试
func TestGenerateSignatureMessage(t *testing.T) {
	address := "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0"
	nonce := "test_nonce_123"
	expiresAt := time.Now().Add(10 * time.Minute)

	message := GenerateSignatureMessage(address, nonce, expiresAt)

	// 验证消息包含必要信息
	assert.Contains(t, message, address)
	assert.Contains(t, message, nonce)
	assert.Contains(t, message, "Monnaire Trading Agent OS")
	assert.Contains(t, message, "Web3 Authentication")
	assert.Contains(t, message, "安全提醒")
	assert.Contains(t, message, "Signature Expires")
}

// ============ 安全函数测试 ============

// TestHashSignature 生成签名哈希测试
func TestHashSignature(t *testing.T) {
	signature := "0x" + "ff" * 130

	hash := HashSignature(signature)

	// 验证哈希格式
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 32) // 16字节hex编码

	// 相同输入应产生相同哈希
	hash2 := HashSignature(signature)
	assert.Equal(t, hash, hash2)
}

// TestSanitizeAddress 清理地址测试
func TestSanitizeAddress(t *testing.T) {
	tests := []struct {
		address    string
		sanitized  string
	}{
		{"0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0", "0x742d...E9E0"},
		{"0x1111", "0x1111"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			result := SanitizeAddress(tt.address)
			assert.Equal(t, tt.sanitized, result)
		})
	}
}

// ============ 基准测试 ============

// BenchmarkRecoverAddressFromSignature 签名恢复基准测试
func BenchmarkRecoverAddressFromSignature(b *testing.B) {
	privateKey, address, err := generateTestKeyPair()
	require.NoError(b, err)

	nonce, err := GenerateNonce()
	require.NoError(b, err)

	expiresAt := time.Now().Add(10 * time.Minute)
	message := GenerateSignatureMessage(address, nonce, expiresAt)

	signature, err := signMessage(privateKey, message)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := RecoverAddressFromSignature(message, signature, address)
		require.NoError(b, err)
	}
}

// BenchmarkGenerateNonce 生成nonce基准测试
func BenchmarkGenerateNonce(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateNonce()
		require.NoError(b, err)
	}
}
