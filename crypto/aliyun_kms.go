package crypto

import (
	"encoding/base64"
	"fmt"
	"os"

	kms "github.com/aliyun/alibaba-cloud-sdk-go/services/kms"
)

// AliyunKMSManager 阿里雲 KMS 管理器
type AliyunKMSManager struct {
	client *kms.Client
	keyID  string // 主密鑰 ID
}

// NewAliyunKMSManager 創建阿里雲 KMS 管理器
func NewAliyunKMSManager() (*AliyunKMSManager, error) {
	// 從環境變數讀取配置
	accessKeyID := os.Getenv("ALIYUN_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("ALIYUN_ACCESS_KEY_SECRET")
	regionID := os.Getenv("ALIYUN_REGION_ID") // 如 cn-hangzhou
	keyID := os.Getenv("ALIYUN_KMS_KEY_ID")   // 主密鑰 ID

	if accessKeyID == "" || accessKeySecret == "" {
		return nil, fmt.Errorf("阿里雲憑證未配置，請設置環境變數 ALIYUN_ACCESS_KEY_ID 和 ALIYUN_ACCESS_KEY_SECRET")
	}

	if keyID == "" {
		return nil, fmt.Errorf("KMS 密鑰 ID 未配置，請設置環境變數 ALIYUN_KMS_KEY_ID")
	}

	// 創建 KMS 客戶端
	client, err := kms.NewClientWithAccessKey(regionID, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("創建 KMS 客戶端失敗: %w", err)
	}

	return &AliyunKMSManager{
		client: client,
		keyID:  keyID,
	}, nil
}

// Encrypt 使用 KMS 加密數據
func (m *AliyunKMSManager) Encrypt(plaintext string) (string, error) {
	request := kms.CreateEncryptRequest()
	request.Scheme = "https"
	request.KeyId = m.keyID
	request.Plaintext = plaintext

	response, err := m.client.Encrypt(request)
	if err != nil {
		return "", fmt.Errorf("KMS 加密失敗: %w", err)
	}

	return response.CiphertextBlob, nil
}

// Decrypt 使用 KMS 解密數據
func (m *AliyunKMSManager) Decrypt(ciphertext string) (string, error) {
	request := kms.CreateDecryptRequest()
	request.Scheme = "https"
	request.CiphertextBlob = ciphertext

	response, err := m.client.Decrypt(request)
	if err != nil {
		return "", fmt.Errorf("KMS 解密失敗: %w", err)
	}

	// Base64 解碼
	plaintext, err := base64.StdEncoding.DecodeString(response.Plaintext)
	if err != nil {
		return "", fmt.Errorf("解碼失敗: %w", err)
	}

	return string(plaintext), nil
}

// GenerateDataKey 生成數據密鑰（用於本地加密，KMS 僅管理主密鑰）
func (m *AliyunKMSManager) GenerateDataKey() (plaintext, ciphertext string, err error) {
	request := kms.CreateGenerateDataKeyRequest()
	request.Scheme = "https"
	request.KeyId = m.keyID
	request.KeySpec = "AES_256" // 256-bit AES 密鑰

	response, err := m.client.GenerateDataKey(request)
	if err != nil {
		return "", "", fmt.Errorf("生成數據密鑰失敗: %w", err)
	}

	// 明文密鑰（用於加密數據）
	plaintextBytes, _ := base64.StdEncoding.DecodeString(response.Plaintext)
	plaintext = string(plaintextBytes)

	// 密文密鑰（保存到數據庫，用於後續解密）
	ciphertext = response.CiphertextBlob

	return plaintext, ciphertext, nil
}

// CreateKey 創建新的 KMS 主密鑰（僅管理員操作）
func (m *AliyunKMSManager) CreateKey(description string) (string, error) {
	request := kms.CreateCreateKeyRequest()
	request.Scheme = "https"
	request.Description = description
	request.KeyUsage = "ENCRYPT/DECRYPT"
	request.Origin = "Aliyun_KMS" // 阿里雲託管

	response, err := m.client.CreateKey(request)
	if err != nil {
		return "", fmt.Errorf("創建 KMS 密鑰失敗: %w", err)
	}

	return response.KeyMetadata.KeyId, nil
}

// EnableKeyRotation 啟用自動密鑰輪換（每年自動輪換）
func (m *AliyunKMSManager) EnableKeyRotation() error {
	request := kms.CreateEnableKeyRotationRequest()
	request.Scheme = "https"
	request.KeyId = m.keyID

	_, err := m.client.EnableKeyRotation(request)
	if err != nil {
		return fmt.Errorf("啟用密鑰輪換失敗: %w", err)
	}

	return nil
}

// ==================== 與現有加密系統集成 ====================

// EncryptionManagerWithKMS 混合加密管理器（本地 + KMS）
type EncryptionManagerWithKMS struct {
	localEM *EncryptionManager
	kmsEM   *AliyunKMSManager
	useKMS  bool // 是否使用 KMS
}

// NewEncryptionManagerWithKMS 創建混合加密管理器
func NewEncryptionManagerWithKMS() (*EncryptionManagerWithKMS, error) {
	// 初始化本地加密
	localEM, err := GetEncryptionManager()
	if err != nil {
		return nil, err
	}

	// 嘗試初始化 KMS（如果配置了環境變數）
	kmsEM, err := NewAliyunKMSManager()
	useKMS := err == nil

	if useKMS {
		fmt.Println("✅ 阿里雲 KMS 已啟用")
	} else {
		fmt.Println("⚠️  阿里雲 KMS 未配置，使用本地加密")
	}

	return &EncryptionManagerWithKMS{
		localEM: localEM,
		kmsEM:   kmsEM,
		useKMS:  useKMS,
	}, nil
}

// EncryptForDatabase 加密數據（自動選擇 KMS 或本地）
func (m *EncryptionManagerWithKMS) EncryptForDatabase(plaintext string) (string, error) {
	if m.useKMS {
		// 使用 KMS 加密
		encrypted, err := m.kmsEM.Encrypt(plaintext)
		if err != nil {
			// KMS 失敗時降級到本地加密
			fmt.Printf("⚠️  KMS 加密失敗，降級到本地加密: %v\n", err)
			return m.localEM.EncryptForDatabase(plaintext)
		}
		return "kms:" + encrypted, nil // 添加前綴標識
	}

	// 使用本地加密
	return m.localEM.EncryptForDatabase(plaintext)
}

// DecryptFromDatabase 解密數據（自動檢測 KMS 或本地）
func (m *EncryptionManagerWithKMS) DecryptFromDatabase(ciphertext string) (string, error) {
	// 檢測是否為 KMS 加密
	if len(ciphertext) > 4 && ciphertext[:4] == "kms:" {
		if !m.useKMS {
			return "", fmt.Errorf("數據使用 KMS 加密，但 KMS 未配置")
		}
		return m.kmsEM.Decrypt(ciphertext[4:])
	}

	// 本地解密
	return m.localEM.DecryptFromDatabase(ciphertext)
}

// MigrateToKMS 將現有本地加密數據遷移到 KMS
func (m *EncryptionManagerWithKMS) MigrateToKMS(localEncrypted string) (string, error) {
	if !m.useKMS {
		return "", fmt.Errorf("KMS 未啟用")
	}

	// 1. 本地解密
	plaintext, err := m.localEM.DecryptFromDatabase(localEncrypted)
	if err != nil {
		return "", err
	}

	// 2. KMS 加密
	kmsEncrypted, err := m.kmsEM.Encrypt(plaintext)
	if err != nil {
		return "", err
	}

	return "kms:" + kmsEncrypted, nil
}
