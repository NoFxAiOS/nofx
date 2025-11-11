package api

import (
	"log"
	"net/http"
	"nofx/crypto"

	"github.com/gin-gonic/gin"
)

// CryptoHandler 加密 API 处理器
type CryptoHandler struct {
	cryptoService *crypto.CryptoService
}

// NewCryptoHandler 創建加密处理器
func NewCryptoHandler(cryptoService *crypto.CryptoService) *CryptoHandler {
	return &CryptoHandler{
		cryptoService: cryptoService,
	}
}

// ==================== 公鑰端点 ====================

// HandleGetPublicKey 获取伺服器公鑰
func (h *CryptoHandler) HandleGetPublicKey(c *gin.Context) {
	publicKey := h.cryptoService.GetPublicKeyPEM()

	c.JSON(http.StatusOK, map[string]string{
		"public_key": publicKey,
		"algorithm":  "RSA-OAEP-2048",
	})
}

// ==================== 加密数据解密端点 ====================

// HandleDecryptSensitiveData 解密客戶端传送的加密数据
func (h *CryptoHandler) HandleDecryptSensitiveData(c *gin.Context) {
	var payload crypto.EncryptedPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 解密
	decrypted, err := h.cryptoService.DecryptSensitiveData(&payload)
	if err != nil {
		log.Printf("❌ 解密失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Decryption failed"})
		return
	}

	c.JSON(http.StatusOK, map[string]string{
		"plaintext": decrypted,
	})
}

// ==================== 审计日誌查詢端点 ====================

// 删除审计日志相关功能，在当前简化的实现中不需要

// ==================== 工具函数 ====================

// isValidPrivateKey 验证私鑰格式
func isValidPrivateKey(key string) bool {
	// EVM 私鑰: 64 位十六进制 (可选 0x 前綴)
	if len(key) == 64 || (len(key) == 66 && key[:2] == "0x") {
		return true
	}
	// TODO: 添加其他鏈的验证
	return false
}
