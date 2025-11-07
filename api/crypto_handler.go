package api

import (
	"encoding/json"
	"log"
	"net/http"
	"nofx/crypto"
)

// CryptoHandler 加密 API 處理器
type CryptoHandler struct {
	em *crypto.EncryptionManager
	ss *crypto.SecureStorage
}

// NewCryptoHandler 創建加密處理器
func NewCryptoHandler(ss *crypto.SecureStorage) (*CryptoHandler, error) {
	em, err := crypto.GetEncryptionManager()
	if err != nil {
		return nil, err
	}

	return &CryptoHandler{
		em: em,
		ss: ss,
	}, nil
}

// ==================== 公鑰端點 ====================

// HandleGetPublicKey 獲取伺服器公鑰
func (h *CryptoHandler) HandleGetPublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publicKey := h.em.GetPublicKeyPEM()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"public_key": publicKey,
		"algorithm":  "RSA-OAEP-4096",
	})
}

// ==================== 加密數據解密端點 ====================

// HandleDecryptPrivateKey 解密客戶端傳送的加密私鑰
func (h *CryptoHandler) HandleDecryptPrivateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		EncryptedKey string `json:"encrypted_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 解密
	decrypted, err := h.em.DecryptWithPrivateKey(req.EncryptedKey)
	if err != nil {
		log.Printf("❌ 解密失敗: %v", err)
		http.Error(w, "Decryption failed", http.StatusInternalServerError)
		return
	}

	// 驗證私鑰格式
	if !isValidPrivateKey(decrypted) {
		http.Error(w, "Invalid private key format", http.StatusBadRequest)
		return
	}

	// ⚠️ 注意：實際生產中，這裡不應該直接返回明文私鑰
	// 應該立即使用主密鑰加密後存入數據庫，然後返回成功狀態
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "私鑰已成功解密並驗證",
	})
}

// ==================== 審計日誌查詢端點 ====================

// HandleGetAuditLogs 查詢審計日誌
func (h *CryptoHandler) HandleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 從請求中獲取用戶 ID（應該從 JWT token 中提取）
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logs, err := h.ss.GetAuditLogs(userID, 100)
	if err != nil {
		http.Error(w, "Failed to fetch audit logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}

// ==================== 工具函數 ====================

// isValidPrivateKey 驗證私鑰格式
func isValidPrivateKey(key string) bool {
	// EVM 私鑰: 64 位十六進制 (可選 0x 前綴)
	if len(key) == 64 || (len(key) == 66 && key[:2] == "0x") {
		return true
	}
	// TODO: 添加其他鏈的驗證
	return false
}
