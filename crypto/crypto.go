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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	storagePrefix    = "ENC:v1:"
	storageDelimiter = ":"
	dataKeyEnvName   = "DATA_ENCRYPTION_KEY"
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

func NewCryptoService(privateKeyPath string) (*CryptoService, error) {
	// 读取私钥文件
	privateKeyPEM, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		// 如果私钥文件不存在，生成新的密钥对
		if err := GenerateRSAKeyPair(privateKeyPath); err != nil {
			return nil, fmt.Errorf("failed to generate RSA key pair: %w", err)
		}
		privateKeyPEM, err = ioutil.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read generated private key: %w", err)
		}
	}

	// 解析私钥
	privateKey, err := ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	dataKey, err := loadDataKeyFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load data encryption key: %w", err)
	}

	return &CryptoService{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		dataKey:    dataKey,
	}, nil
}

func GenerateRSAKeyPair(privateKeyPath string) error {
	// 确保目录存在
	dir := filepath.Dir(privateKeyPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// 生成 RSA 密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// 编码私钥
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// 保存私钥
	if err := ioutil.WriteFile(privateKeyPath, privateKeyPEM, 0600); err != nil {
		return err
	}

	// 编码公钥
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	// 保存公钥
	publicKeyPath := privateKeyPath + ".pub"
	if err := ioutil.WriteFile(publicKeyPath, publicKeyPEM, 0644); err != nil {
		return err
	}

	return nil
}

func ParseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
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
			return nil, errors.New("not an RSA key")
		}
		return rsaKey, nil
	default:
		return nil, errors.New("unsupported key type: " + block.Type)
	}
}

func loadDataKeyFromEnv() ([]byte, error) {
	keyStr := strings.TrimSpace(os.Getenv(dataKeyEnvName))
	if keyStr == "" {
		return nil, fmt.Errorf("%s not set", dataKeyEnvName)
	}

	if key, ok := decodePossibleKey(keyStr); ok {
		return key, nil
	}

	sum := sha256.Sum256([]byte(keyStr))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key, nil
}

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
		return "", errors.New("data encryption key not configured")
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
		return "", errors.New("data encryption key not configured")
	}
	if !isEncryptedStorageValue(value) {
		return "", errors.New("value is not encrypted")
	}

	payload := strings.TrimPrefix(value, storagePrefix)
	parts := strings.SplitN(payload, storageDelimiter, 2)
	if len(parts) != 2 {
		return "", errors.New("invalid encrypted payload format")
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("decode nonce failed: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode ciphertext failed: %w", err)
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
		return "", fmt.Errorf("invalid nonce size: expected %d, got %d", gcm.NonceSize(), len(nonce))
	}

	aad := composeAAD(aadParts)
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
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
			return nil, errors.New("timestamp invalid or expired")
		}
	}

	// 2. 解码 base64url
	wrappedKey, err := base64.RawURLEncoding.DecodeString(payload.WrappedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode wrapped key: %w", err)
	}

	iv, err := base64.RawURLEncoding.DecodeString(payload.IV)
	if err != nil {
		return nil, fmt.Errorf("failed to decode IV: %w", err)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	var aad []byte
	if payload.AAD != "" {
		aad, err = base64.RawURLEncoding.DecodeString(payload.AAD)
		if err != nil {
			return nil, fmt.Errorf("failed to decode AAD: %w", err)
		}

		// 验证 AAD
		var aadData AADData
		if err := json.Unmarshal(aad, &aadData); err == nil {
			// 可以在这里添加额外的验证逻辑
			// 例如：验证 sessionID、userID 等
		}
	}

	// 3. 使用 RSA-OAEP 解密 AES 密钥
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, cs.privateKey, wrappedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap AES key: %w", err)
	}

	// 4. 使用 AES-GCM 解密数据
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(iv) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid IV size: expected %d, got %d", gcm.NonceSize(), len(iv))
	}

	// 解密并验证认证标签
	plaintext, err := gcm.Open(nil, iv, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("authentication/decryption failed: %w", err)
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
