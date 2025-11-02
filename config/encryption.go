package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// EncryptionManager 加密管理器
type EncryptionManager struct {
	masterKey []byte
	gcm       cipher.AEAD
}

// NewEncryptionManager 创建加密管理器
func NewEncryptionManager() (*EncryptionManager, error) {
	// 从环境变量读取主密钥
	masterKeyStr := os.Getenv("NOFX_MASTER_KEY")
	if masterKeyStr == "" {
		// 如果未设置，使用默认密钥（仅用于开发环境，生产环境必须设置）
		masterKeyStr = "default-master-key-change-in-production-environment"
		fmt.Println("⚠️  警告：未设置 NOFX_MASTER_KEY 环境变量，使用默认密钥（不安全！）")
		fmt.Println("   生产环境请设置：export NOFX_MASTER_KEY=\"your-random-32-bytes-key\"")
	}

	// 使用 SHA-256 生成固定长度的密钥
	hash := sha256.Sum256([]byte(masterKeyStr))
	masterKey := hash[:]

	// 创建 AES cipher
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	return &EncryptionManager{
		masterKey: masterKey,
		gcm:       gcm,
	}, nil
}

// Encrypt 加密数据
func (em *EncryptionManager) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// 生成随机 nonce
	nonce := make([]byte, em.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密
	ciphertext := em.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Base64 编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
func (em *EncryptionManager) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Base64 解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %w", err)
	}

	// 检查数据长度
	nonceSize := em.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("密文数据太短")
	}

	// 分离 nonce 和密文
	nonce, cipherBytes := data[:nonceSize], data[nonceSize:]

	// 解密
	plaintext, err := em.gcm.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

// MaskSecret 脱敏显示密钥（保留前4位和后4位）
func MaskSecret(secret string) string {
	if secret == "" {
		return ""
	}

	length := len(secret)
	if length <= 8 {
		// 太短的密钥，只显示星号
		return "****"
	}

	// 保留前4位和后4位
	return secret[:4] + "********" + secret[length-4:]
}

// IsEncrypted 判断字符串是否已加密（简单判断：Base64 编码且长度合理）
func IsEncrypted(text string) bool {
	if text == "" {
		return false
	}

	// 尝试 Base64 解码
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return false
	}

	// 加密后的数据应该至少包含 nonce + 一些密文
	// GCM nonce 默认 12 字节，密文至少有几个字节
	return len(data) >= 16
}
