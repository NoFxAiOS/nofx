package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	storagePrefix    = "ENC:v1:"
	storageDelimiter = ":"
)

// 环境变量名称
const (
	EnvDataEncryptionKey = "DATA_ENCRYPTION_KEY" // AES 数据加密密钥 (Base64)
	EnvRSAPrivateKey     = "RSA_PRIVATE_KEY"     // RSA 私钥 (PEM 格式，换行用 \n)
)

type EncryptedPayload struct {
	WrappedKey string `json:"wrappedKey"`
	IV         string `json:"iv"`
	Ciphertext string `json:"ciphertext"`
	AAD        string `json:"aad,omitempty"`
	KID        string `json:"kid,omitempty"`
	TS         int64  `json:"ts,omitempty"`
}

type AADData struct {
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId"`
	TS        int64  `json:"ts"`
	Purpose   string `json:"purpose"`
}

type CryptoService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	dataKey    []byte
}

// NewCryptoService 创建加密服务（从环境变量加载密钥）
func NewCryptoService() (*CryptoService, error) {
	// 1. 加载 RSA 私钥
	privateKey, err := loadRSAPrivateKeyFromEnv()
	if err != nil {
		return nil, fmt.Errorf("RSA 私钥加载失败: %w", err)
	}

	// 2. 加载 AES 数据加密密钥
	dataKey, err := loadDataKeyFromEnv()
	if err != nil {
		return nil, fmt.Errorf("数据加密密钥加载失败: %w", err)
	}

	return &CryptoService{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		dataKey:    dataKey,
	}, nil
}

// loadRSAPrivateKeyFromEnv 从环境变量加载 RSA 私钥
func loadRSAPrivateKeyFromEnv() (*rsa.PrivateKey, error) {
	keyPEM := os.Getenv(EnvRSAPrivateKey)
	if keyPEM == "" {
		return nil, fmt.Errorf("环境变量 %s 未设置，请在 .env 中配置 RSA 私钥", EnvRSAPrivateKey)
	}

	// 处理环境变量中的换行符（\n -> 实际换行）
	keyPEM = strings.ReplaceAll(keyPEM, "\\n", "\n")

	return ParseRSAPrivateKeyFromPEM([]byte(keyPEM))
}

// loadDataKeyFromEnv 从环境变量加载 AES 数据加密密钥
func loadDataKeyFromEnv() ([]byte, error) {
	keyStr := strings.TrimSpace(os.Getenv(EnvDataEncryptionKey))
	if keyStr == "" {
		return nil, fmt.Errorf("环境变量 %s 未设置，请在 .env 中配置数据加密密钥", EnvDataEncryptionKey)
	}

	// 尝试解码
	if key, ok := decodePossibleKey(keyStr); ok {
		return key, nil
	}

	// 如果无法解码，使用 SHA256 哈希作为密钥
	sum := sha256.Sum256([]byte(keyStr))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key, nil
}

// ParseRSAPrivateKeyFromPEM 解析 PEM 格式的 RSA 私钥
func ParseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("无效的 PEM 格式")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("不是 RSA 密钥")
		}
		return rsaKey, nil
	default:
		return nil, errors.New("不支持的密钥类型: " + block.Type)
	}
}

// decodePossibleKey 尝试用多种编码方式解码密钥
func decodePossibleKey(value string) ([]byte, bool) {
	decoders := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		func(s string) ([]byte, error) { return hex.DecodeString(s) },
	}

	for _, decoder := range decoders {
		if decoded, err := decoder(value); err == nil {
			if key, ok := normalizeAESKey(decoded); ok {
				return key, true
			}
		}
	}

	return nil, false
}

// normalizeAESKey 标准化 AES 密钥长度
func normalizeAESKey(raw []byte) ([]byte, bool) {
	switch len(raw) {
	case 16, 24, 32:
		return raw, true
	case 0:
		return nil, false
	default:
		sum := sha256.Sum256(raw)
		key := make([]byte, len(sum))
		copy(key, sum[:])
		return key, true
	}
}

func (cs *CryptoService) HasDataKey() bool {
	return len(cs.dataKey) > 0
}

func (cs *CryptoService) GetPublicKeyPEM() string {
	publicKeyDER, err := x509.MarshalPKIXPublicKey(cs.publicKey)
	if err != nil {
		return ""
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	return string(publicKeyPEM)
}

func (cs *CryptoService) EncryptForStorage(plaintext string, aadParts ...string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if !cs.HasDataKey() {
		return "", errors.New("数据加密密钥未配置")
	}
	if isEncryptedStorageValue(plaintext) {
		return plaintext, nil
	}

	block, err := aes.NewCipher(cs.dataKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	aad := composeAAD(aadParts)
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), aad)

	return storagePrefix +
		base64.StdEncoding.EncodeToString(nonce) + storageDelimiter +
		base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (cs *CryptoService) DecryptFromStorage(value string, aadParts ...string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !cs.HasDataKey() {
		return "", errors.New("数据加密密钥未配置")
	}
	if !isEncryptedStorageValue(value) {
		return "", errors.New("数据未加密")
	}

	payload := strings.TrimPrefix(value, storagePrefix)
	parts := strings.SplitN(payload, storageDelimiter, 2)
	if len(parts) != 2 {
		return "", errors.New("无效的加密数据格式")
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("解码 nonce 失败: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("解码密文失败: %w", err)
	}

	block, err := aes.NewCipher(cs.dataKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(nonce) != gcm.NonceSize() {
		return "", fmt.Errorf("无效的 nonce 长度: 期望 %d, 实际 %d", gcm.NonceSize(), len(nonce))
	}

	aad := composeAAD(aadParts)
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

func (cs *CryptoService) IsEncryptedStorageValue(value string) bool {
	return isEncryptedStorageValue(value)
}

func composeAAD(parts []string) []byte {
	if len(parts) == 0 {
		return nil
	}
	return []byte(strings.Join(parts, "|"))
}

func isEncryptedStorageValue(value string) bool {
	return strings.HasPrefix(value, storagePrefix)
}

func (cs *CryptoService) DecryptPayload(payload *EncryptedPayload) ([]byte, error) {
	// 1. 验证时间戳（防止重放攻击）
	if payload.TS != 0 {
		elapsed := time.Since(time.Unix(payload.TS, 0))
		if elapsed > 5*time.Minute || elapsed < -1*time.Minute {
			return nil, errors.New("时间戳无效或已过期")
		}
	}

	// 2. 解码 base64url
	wrappedKey, err := base64.RawURLEncoding.DecodeString(payload.WrappedKey)
	if err != nil {
		return nil, fmt.Errorf("解码 wrapped key 失败: %w", err)
	}

	iv, err := base64.RawURLEncoding.DecodeString(payload.IV)
	if err != nil {
		return nil, fmt.Errorf("解码 IV 失败: %w", err)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("解码密文失败: %w", err)
	}

	var aad []byte
	if payload.AAD != "" {
		aad, err = base64.RawURLEncoding.DecodeString(payload.AAD)
		if err != nil {
			return nil, fmt.Errorf("解码 AAD 失败: %w", err)
		}

		var aadData AADData
		if err := json.Unmarshal(aad, &aadData); err == nil {
			// 可以在这里添加额外的验证逻辑
		}
	}

	// 3. 使用 RSA-OAEP 解密 AES 密钥
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, cs.privateKey, wrappedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA 解密失败: %w", err)
	}

	// 4. 使用 AES-GCM 解密数据
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	if len(iv) != gcm.NonceSize() {
		return nil, fmt.Errorf("无效的 IV 长度: 期望 %d, 实际 %d", gcm.NonceSize(), len(iv))
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("解密验证失败: %w", err)
	}

	return plaintext, nil
}

func (cs *CryptoService) DecryptSensitiveData(payload *EncryptedPayload) (string, error) {
	plaintext, err := cs.DecryptPayload(payload)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// GenerateKeyPair 生成 RSA 密钥对（用于初始化时生成密钥）
// 返回 PEM 格式的私钥和公钥
func GenerateKeyPair() (privateKeyPEM, publicKeyPEM string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// 编码私钥
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// 编码公钥
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	return string(privPEM), string(pubPEM), nil
}

// GenerateDataKey 生成 AES 数据加密密钥
// 返回 Base64 编码的 32 字节密钥
func GenerateDataKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
