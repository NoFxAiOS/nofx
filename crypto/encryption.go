package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// EncryptionManager 加密管理器（單例模式）
type EncryptionManager struct {
	privateKey   *rsa.PrivateKey
	publicKeyPEM string
	masterKey    []byte // 用於數據庫加密的主密鑰
	mu           sync.RWMutex
}

var (
	instance *EncryptionManager
	once     sync.Once
)

// GetEncryptionManager 獲取加密管理器實例
func GetEncryptionManager() (*EncryptionManager, error) {
	var initErr error
	once.Do(func() {
		instance, initErr = newEncryptionManager()
	})
	return instance, initErr
}

// newEncryptionManager 初始化加密管理器
func newEncryptionManager() (*EncryptionManager, error) {
	em := &EncryptionManager{}

	// 1. 加載或生成 RSA 密鑰對
	if err := em.loadOrGenerateRSAKeyPair(); err != nil {
		return nil, fmt.Errorf("初始化 RSA 密鑰失敗: %w", err)
	}

	// 2. 加載或生成數據庫主密鑰
	if err := em.loadOrGenerateMasterKey(); err != nil {
		return nil, fmt.Errorf("初始化主密鑰失敗: %w", err)
	}

	log.Println("🔐 加密管理器初始化成功")
	return em, nil
}

// ==================== RSA 密鑰管理 ====================

const (
	rsaKeySize        = 4096
	rsaPrivateKeyFile = ".secrets/rsa_private.pem"
	rsaPublicKeyFile  = ".secrets/rsa_public.pem"
	masterKeyFile     = ".secrets/master.key"
)

// loadOrGenerateRSAKeyPair 加載或生成 RSA 密鑰對
func (em *EncryptionManager) loadOrGenerateRSAKeyPair() error {
	// 確保 .secrets 目錄存在
	if err := os.MkdirAll(".secrets", 0700); err != nil {
		return err
	}

	// 嘗試加載現有密鑰
	if _, err := os.Stat(rsaPrivateKeyFile); err == nil {
		return em.loadRSAKeyPair()
	}

	// 生成新密鑰對
	log.Println("🔑 生成新的 RSA-4096 密鑰對...")
	privateKey, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return err
	}

	em.privateKey = privateKey

	// 保存私鑰
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})
	if err := os.WriteFile(rsaPrivateKeyFile, privateKeyPEM, 0600); err != nil {
		return err
	}

	// 保存公鑰
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	if err := os.WriteFile(rsaPublicKeyFile, publicKeyPEM, 0644); err != nil {
		return err
	}

	em.publicKeyPEM = string(publicKeyPEM)
	log.Println("✅ RSA 密鑰對已生成並保存")
	return nil
}

// loadRSAKeyPair 加載 RSA 密鑰對
func (em *EncryptionManager) loadRSAKeyPair() error {
	// 加載私鑰
	privateKeyPEM, err := os.ReadFile(rsaPrivateKeyFile)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return errors.New("無效的私鑰 PEM 格式")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	em.privateKey = privateKey

	// 加載公鑰
	publicKeyPEM, err := os.ReadFile(rsaPublicKeyFile)
	if err != nil {
		return err
	}
	em.publicKeyPEM = string(publicKeyPEM)

	log.Println("✅ RSA 密鑰對已加載")
	return nil
}

// GetPublicKeyPEM 獲取公鑰 (PEM 格式)
func (em *EncryptionManager) GetPublicKeyPEM() string {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.publicKeyPEM
}

// ==================== 混合解密 (RSA + AES) ====================

// DecryptWithPrivateKey 使用私鑰解密數據
// 數據格式: [加密的 AES 密鑰長度(4字節)] + [加密的 AES 密鑰] + [IV(12字節)] + [加密數據]
func (em *EncryptionManager) DecryptWithPrivateKey(encryptedBase64 string) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// Base64 解碼
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("Base64 解碼失敗: %w", err)
	}

	if len(encryptedData) < 4+256+12 { // 最小長度檢查
		return "", errors.New("加密數據長度不足")
	}

	// 1. 讀取加密的 AES 密鑰長度
	aesKeyLen := binary.BigEndian.Uint32(encryptedData[:4])
	if aesKeyLen > 1024 { // 防止過大的長度值
		return "", errors.New("無效的 AES 密鑰長度")
	}

	offset := 4
	// 2. 提取加密的 AES 密鑰
	encryptedAESKey := encryptedData[offset : offset+int(aesKeyLen)]
	offset += int(aesKeyLen)

	// 3. 使用 RSA 私鑰解密 AES 密鑰
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, em.privateKey, encryptedAESKey, nil)
	if err != nil {
		return "", fmt.Errorf("RSA 解密失敗: %w", err)
	}

	// 4. 提取 IV
	iv := encryptedData[offset : offset+12]
	offset += 12

	// 5. 提取加密數據
	ciphertext := encryptedData[offset:]

	// 6. 使用 AES-GCM 解密
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := aesGCM.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("AES 解密失敗: %w", err)
	}

	// 清除敏感數據
	for i := range aesKey {
		aesKey[i] = 0
	}

	return string(plaintext), nil
}

// ==================== 數據庫加密 (AES-256-GCM) ====================

// loadOrGenerateMasterKey 加載或生成數據庫主密鑰
func (em *EncryptionManager) loadOrGenerateMasterKey() error {
	// 優先從環境變數加載
	if envKey := os.Getenv("NOFX_MASTER_KEY"); envKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(envKey)
		if err == nil && len(decoded) == 32 {
			em.masterKey = decoded
			log.Println("✅ 從環境變數加載主密鑰")
			return nil
		}
		log.Println("⚠️ 環境變數中的主密鑰無效，使用文件密鑰")
	}

	// 嘗試從文件加載
	if _, err := os.Stat(masterKeyFile); err == nil {
		keyBytes, err := os.ReadFile(masterKeyFile)
		if err != nil {
			return err
		}
		decoded, err := base64.StdEncoding.DecodeString(string(keyBytes))
		if err != nil || len(decoded) != 32 {
			return errors.New("主密鑰文件損壞")
		}
		em.masterKey = decoded
		log.Println("✅ 從文件加載主密鑰")
		return nil
	}

	// 生成新主密鑰
	log.Println("🔑 生成新的數據庫主密鑰 (AES-256)...")
	masterKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, masterKey); err != nil {
		return err
	}

	em.masterKey = masterKey

	// 保存到文件
	encoded := base64.StdEncoding.EncodeToString(masterKey)
	if err := os.WriteFile(masterKeyFile, []byte(encoded), 0600); err != nil {
		return err
	}

	log.Println("✅ 主密鑰已生成並保存")
	log.Printf("📁 主密鑰文件位置: %s (權限: 0600)", masterKeyFile)
	log.Println("🔐 生產環境請設置環境變數: NOFX_MASTER_KEY=<從文件讀取>")
	log.Println("⚠️  請妥善保管 .secrets 目錄，切勿將密鑰提交到版本控制系統")
	return nil
}

// EncryptForDatabase 使用主密鑰加密數據（用於數據庫存儲）
func (em *EncryptionManager) EncryptForDatabase(plaintext string) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	block, err := aes.NewCipher(em.masterKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptFromDatabase 使用主密鑰解密數據（從數據庫讀取）
func (em *EncryptionManager) DecryptFromDatabase(encryptedBase64 string) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// 處理空字符串（未加密的舊數據）
	if encryptedBase64 == "" {
		return "", nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(em.masterKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("加密數據過短")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// ==================== 密鑰輪換 ====================

// RotateMasterKey 輪換主密鑰（需要重新加密所有數據）
func (em *EncryptionManager) RotateMasterKey() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	log.Println("🔄 開始輪換主密鑰...")

	// 生成新主密鑰
	newMasterKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newMasterKey); err != nil {
		return err
	}

	// 備份舊密鑰
	oldMasterKey := em.masterKey

	// 更新密鑰
	em.masterKey = newMasterKey

	// 保存新密鑰
	encoded := base64.StdEncoding.EncodeToString(newMasterKey)
	backupFile := fmt.Sprintf("%s.backup.%d", masterKeyFile, os.Getpid())
	if err := os.WriteFile(backupFile, []byte(base64.StdEncoding.EncodeToString(oldMasterKey)), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(masterKeyFile, []byte(encoded), 0600); err != nil {
		return err
	}

	log.Println("✅ 主密鑰已輪換")
	log.Printf("⚠️ 舊密鑰已備份到: %s", backupFile)
	log.Printf("🔐 新主密鑰: %s", encoded)

	return nil
}
