package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
)

// RSAKeyPair RSA密钥对
type RSAKeyPair struct {
	PrivateKey string `json:"private_key"` // PEM格式的私钥
	PublicKey  string `json:"public_key"`  // PEM格式的公钥
}

// HybridEncryptResult 混合加密结果
type HybridEncryptResult struct {
	EncryptedAesKey string `json:"encryptedAesKey"` // RSA加密的AES密钥（Base64）
	IV              string `json:"iv"`              // AES加密的初始化向量（Base64）
	EncryptedData   string `json:"encryptedData"`   // AES加密的数据（Base64）
}

// GenerateRSAKeyPair 生成RSA密钥对（2048位）
func GenerateRSAKeyPair() (*RSAKeyPair, error) {
	// 生成2048位RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成RSA密钥失败: %w", err)
	}

	// 编码私钥为PEM格式
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// 编码公钥为PEM格式
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("编码公钥失败: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return &RSAKeyPair{
		PrivateKey: string(privateKeyPEM),
		PublicKey:  string(publicKeyPEM),
	}, nil
}

// ParseRSAPrivateKey 解析PEM格式的私钥
func ParseRSAPrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("解析私钥PEM失败")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	return privateKey, nil
}

// ParseRSAPublicKey 解析PEM格式的公钥
func ParseRSAPublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("解析公钥PEM失败")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析公钥失败: %w", err)
	}

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("公钥类型不正确")
	}

	return rsaPubKey, nil
}

// RSAEncrypt 使用公钥加密数据
func RSAEncrypt(publicKeyPEM string, plaintext string) (string, error) {
	publicKey, err := ParseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return "", err
	}

	// 使用OAEP填充模式加密
	ciphertext, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		publicKey,
		[]byte(plaintext),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("RSA加密失败: %w", err)
	}

	// 返回Base64编码的密文
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// RSADecrypt 使用私钥解密数据
func RSADecrypt(privateKeyPEM string, ciphertextBase64 string) (string, error) {
	privateKey, err := ParseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}

	// Base64解码密文
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", fmt.Errorf("Base64解码失败: %w", err)
	}

	// 使用OAEP填充模式解密
	plaintext, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		privateKey,
		ciphertext,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("RSA解密失败: %w", err)
	}

	return string(plaintext), nil
}

// HybridDecrypt 混合解密（RSA + AES）
func HybridDecrypt(privateKeyPEM string, hybridCiphertextJSON string) (string, error) {
	// 1. 解析JSON结构
	var result HybridEncryptResult
	if err := json.Unmarshal([]byte(hybridCiphertextJSON), &result); err != nil {
		return "", fmt.Errorf("解析混合加密JSON失败: %w", err)
	}

	// 2. 解析私钥
	privateKey, err := ParseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}

	// 3. Base64解码RSA加密的AES密钥
	encryptedAesKey, err := base64.StdEncoding.DecodeString(result.EncryptedAesKey)
	if err != nil {
		return "", fmt.Errorf("Base64解码AES密钥失败: %w", err)
	}

	// 4. 使用RSA解密AES密钥
	aesKey, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		privateKey,
		encryptedAesKey,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("RSA解密AES密钥失败: %w", err)
	}

	// 5. Base64解码IV
	iv, err := base64.StdEncoding.DecodeString(result.IV)
	if err != nil {
		return "", fmt.Errorf("Base64解码IV失败: %w", err)
	}

	// 6. Base64解码加密数据
	encryptedData, err := base64.StdEncoding.DecodeString(result.EncryptedData)
	if err != nil {
		return "", fmt.Errorf("Base64解码数据失败: %w", err)
	}

	// 7. 创建AES解密器
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("创建AES解密器失败: %w", err)
	}

	// 8. 使用AES-GCM解密
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
	}

	// 9. 解密数据
	plaintext, err := aesgcm.Open(nil, iv, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("AES-GCM解密失败: %w", err)
	}

	return string(plaintext), nil
}

// SmartDecrypt 智能解密（自动识别纯RSA或混合加密）
func SmartDecrypt(privateKeyPEM string, ciphertext string) (string, error) {
	// 尝试解析为JSON（混合加密格式）
	var result HybridEncryptResult
	err := json.Unmarshal([]byte(ciphertext), &result)

	// 如果能解析为JSON且包含必要字段，则使用混合解密
	if err == nil && result.EncryptedAesKey != "" && result.IV != "" && result.EncryptedData != "" {
		return HybridDecrypt(privateKeyPEM, ciphertext)
	}

	// 否则使用纯RSA解密
	return RSADecrypt(privateKeyPEM, ciphertext)
}
