package web3

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/web3_auth"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ============ 请求/响应结构 ============

// GenerateNonceRequest 生成nonce请求
type GenerateNonceRequest struct {
	Address    string `json:"address" binding:"required"`
	WalletType string `json:"wallet_type" binding:"required"`
}

// GenerateNonceResponse 生成nonce响应
type GenerateNonceResponse struct {
	Nonce      string    `json:"nonce"`
	Timestamp  int64     `json:"timestamp"`
	Message    string    `json:"message"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// AuthRequest 钱包认证请求
type AuthRequest struct {
	Address   string `json:"address" binding:"required"`
	Signature string `json:"signature" binding:"required"`
	Nonce     string `json:"nonce" binding:"required"`
	WalletType string `json:"wallet_type" binding:"required"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Success      bool      `json:"success"`
	Message      string    `json:"message"`
	Token        string    `json:"token,omitempty"`
	WalletAddr   string    `json:"wallet_addr,omitempty"`
	BoundWallets []string  `json:"bound_wallets,omitempty"`
	NonceInfo    *NonceInfo `json:"nonce_info,omitempty"`
}

// LinkWalletRequest 绑定钱包请求
type LinkWalletRequest struct {
	Address    string `json:"address" binding:"required"`
	WalletType string `json:"wallet_type" binding:"required"`
	IsPrimary  bool   `json:"is_primary"`
}

// LinkWalletResponse 绑定钱包响应
type LinkWalletResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Address   string `json:"address"`
	IsPrimary bool   `json:"is_primary"`
}

// UnlinkWalletResponse 解绑钱包响应
type UnlinkWalletResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Address string `json:"address"`
}

// ListWalletsResponse 钱包列表响应
type ListWalletsResponse struct {
	Success bool         `json:"success"`
	Wallets []UserWalletInfo `json:"wallets"`
}

// UserWalletInfo 用户钱包信息
type UserWalletInfo struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"`
	WalletType  string    `json:"wallet_type"`
	Label       string    `json:"label"`
	IsPrimary   bool      `json:"is_primary"`
	BoundAt     time.Time `json:"bound_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
}

// NonceInfo nonce信息
type NonceInfo struct {
	Used     bool      `json:"used"`
	Expired  bool      `json:"expired"`
	UsedAt   time.Time `json:"used_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// ============ 错误码定义 ============

const (
	// Web3认证错误
	ErrCodeInvalidAddress     = "WEB3_001"
	ErrCodeInvalidSignature   = "WEB3_002"
	ErrCodeNonceExpired       = "WEB3_003"
	ErrCodeAddressMismatch    = "WEB3_004"
	ErrCodeWalletBound        = "WEB3_005"
	ErrCodeWalletNotBound     = "WEB3_006"
	ErrCodeCannotUnbind       = "WEB3_007"
	ErrCodeWalletTypeInvalid  = "WEB3_008"
	ErrCodeNonceNotFound      = "WEB3_009"
	ErrCodeNonceAlreadyUsed   = "WEB3_010"
	ErrCodeInvalidNonce       = "WEB3_011"
	ErrCodeRateLimited        = "WEB3_012"
)

// ============ 处理器 ============

// Handler Web3认证处理器
type Handler struct {
	walletRepo    web3.Repository
	nonceRepo     web3.NonceRepository
	rateLimitIP   map[string]int // 内存速率限制
	rateLimitAddr map[string]int // 地址速率限制
}

// NewHandler 创建处理器
func NewHandler(walletRepo web3.Repository, nonceRepo web3.NonceRepository) *Handler {
	return &Handler{
		walletRepo:    walletRepo,
		nonceRepo:     nonceRepo,
		rateLimitIP:   make(map[string]int),
		rateLimitAddr: make(map[string]int),
	}
}

// ============ 核心方法 ============

// GenerateNonce 生成nonce（修复CVE-WS-002）
func (h *Handler) GenerateNonce(c *gin.Context) {
	// 1. 绑定请求
	var req GenerateNonceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidRequest,
			Message: "无效的请求参数",
			Detail:  err.Error(),
		})
		return
	}

	// 2. 验证地址格式
	if err := web3_auth.ValidateAddress(req.Address); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidAddress,
			Message: "地址格式错误",
			Detail:  err.Error(),
		})
		return
	}

	// 3. 验证钱包类型
	if err := web3_auth.ValidateWalletType(req.WalletType); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeWalletTypeInvalid,
			Message: "钱包类型错误",
			Detail:  err.Error(),
		})
		return
	}

	// 4. 检查速率限制（每个IP和地址）
	clientIP := c.ClientIP()
	if h.checkRateLimit(clientIP, req.Address) {
		c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Code:    ErrCodeRateLimited,
			Message: "请求过于频繁，请稍后再试",
		})
		return
	}

	// 5. 生成nonce
	nonce, err := web3_auth.GenerateNonce()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrCodeInternalError,
			Message: "生成nonce失败",
			Detail:  err.Error(),
		})
		return
	}

	// 6. 设置过期时间
	expiresAt := time.Now().Add(web3_auth.NONCE_EXPIRY_MINUTES * time.Minute)

	// 7. 存储nonce到数据库
	err = h.nonceRepo.StoreNonce(req.Address, nonce, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrCodeInternalError,
			Message: "存储nonce失败",
			Detail:  err.Error(),
		})
		return
	}

	// 8. 生成签名消息
	message := web3_auth.GenerateSignatureMessage(req.Address, nonce, expiresAt)

	// 9. 返回响应
	c.JSON(http.StatusOK, GenerateNonceResponse{
		Nonce:      nonce,
		Timestamp:  expiresAt.Unix(),
		Message:    message,
		ExpiresAt:  expiresAt,
	})

	// 10. 记录审计日志（此处省略实际实现）
}

// Authenticate 钱包认证（修复CVE-WS-010 - 完整nonce验证）
func (h *Handler) Authenticate(c *gin.Context) {
	// 1. 绑定请求
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidRequest,
			Message: "无效的请求参数",
			Detail:  err.Error(),
		})
		return
	}

	// 2. 验证地址格式
	if err := web3_auth.ValidateAddress(req.Address); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidAddress,
			Message: "地址格式错误",
			Detail:  err.Error(),
		})
		return
	}

	// 3. 验证签名格式
	if err := web3_auth.ValidateSignature(req.Signature); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidSignature,
			Message: "签名格式错误",
			Detail:  err.Error(),
		})
		return
	}

	// 4. 验证nonce格式
	if err := web3_auth.ValidateNonce(req.Nonce); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidNonce,
			Message: "nonce格式错误",
			Detail:  err.Error(),
		})
		return
	}

	// 5. ✅ 关键修复：验证nonce有效性（修复CVE-WS-010）
	err := h.nonceRepo.ValidateNonce(req.Address, req.Nonce)
	if err != nil {
		var code, message string
		if strings.Contains(err.Error(), "nonce已过期") {
			code = ErrCodeNonceExpired
			message = "nonce已过期，请重新生成"
		} else if strings.Contains(err.Error(), "nonce已被使用") {
			code = ErrCodeNonceAlreadyUsed
			message = "nonce已被使用，不能重复使用"
		} else if strings.Contains(err.Error(), "nonce不存在") {
			code = ErrCodeNonceNotFound
			message = "nonce不存在，请先生成nonce"
		} else {
			code = ErrCodeInvalidNonce
			message = "nonce验证失败"
		}

		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    code,
			Message: message,
			Detail:  err.Error(),
		})
		return
	}

	// 6. 生成签名消息
	// 获取nonce的过期时间
	var expiresAt time.Time
	err = h.db.QueryRow(`
		SELECT expires_at FROM web3_wallet_nonces
		WHERE address = $1 AND nonce = $2
	`, req.Address, req.Nonce).Scan(&expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrCodeInternalError,
			Message: "查询nonce失败",
		})
		return
	}

	// 生成包含过期时间的签名消息
	message := web3_auth.GenerateSignatureMessage(req.Address, req.Nonce, expiresAt)

	// 7. 验证签名（使用修复后的secp256k1实现）
	recoveredAddr, err := web3_auth.RecoverAddressFromSignature(message, req.Signature, req.Address)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    ErrCodeInvalidSignature,
			Message: "签名验证失败",
			Detail:  err.Error(),
		})
		return
	}

	// 8. ✅ 关键修复：验证恢复的地址匹配
	if !strings.EqualFold(recoveredAddr, req.Address) {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    ErrCodeAddressMismatch,
			Message: "签名地址与请求地址不匹配",
		})
		return
	}

	// 9. ✅ 关键修复：标记nonce为已使用（防止重放攻击）
	err = h.nonceRepo.MarkNonceUsed(req.Address, req.Nonce)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrCodeInternalError,
			Message: "标记nonce失败",
			Detail:  err.Error(),
		})
		return
	}

	// 10. 检查该地址是否已绑定用户
	boundUser, err := h.walletRepo.GetBoundUser(req.Address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrCodeInternalError,
			Message: "查询绑定信息失败",
		})
		return
	}

	// 11. 构建响应
	response := AuthResponse{
		Success: true,
		Message: "钱包验证成功",
	}

	// 12. 如果已绑定用户，返回已绑定的钱包列表
	if boundUser != nil {
		wallets, err := h.walletRepo.GetUserWallets(boundUser.UserID)
		if err == nil {
			var walletAddrs []string
			for _, w := range wallets {
				walletAddrs = append(walletAddrs, w.WalletAddr)
			}
			response.BoundWallets = walletAddrs
		}

		// 如果需要，可以生成JWT token返回
		// response.Token = jwtToken
	}

	// 13. 记录审计日志
	// 此处应记录到audit_logs表

	c.JSON(http.StatusOK, response)
}

// ============ 辅助方法 ============

// checkRateLimit 检查速率限制
func (h *Handler) checkRateLimit(ip, address string) bool {
	// 这里应该使用Redis等分布式存储
	// 简化实现使用内存map
	clientIP := h.rateLimitIP[ip]
	if clientIP > 10 { // 每分钟最多10次
		return true
	}
	h.rateLimitIP[ip]++

	addrCount := h.rateLimitAddr[address]
	if addrCount > 5 { // 每个地址每分钟最多5次
		return true
	}
	h.rateLimitAddr[address]++

	return false
}

// ============ 钱包管理方法 ============

// LinkWallet 绑定钱包到用户（JWT保护）
func (h *Handler) LinkWallet(c *gin.Context) {
	// 从JWT中获取用户ID
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    ErrCodeUnauthorized,
			Message: "未认证",
		})
		return
	}

	// 绑定请求
	var req LinkWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidRequest,
			Message: "无效的请求参数",
			Detail:  err.Error(),
		})
		return
	}

	// 验证地址
	if err := web3_auth.ValidateAddress(req.Address); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidAddress,
			Message: "地址格式错误",
			Detail:  err.Error(),
		})
		return
	}

	// 验证钱包类型
	if err := web3_auth.ValidateWalletType(req.WalletType); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeWalletTypeInvalid,
			Message: "钱包类型错误",
			Detail:  err.Error(),
		})
		return
	}

	// 绑定钱包
	err := h.walletRepo.LinkWallet(userID, req.Address, req.IsPrimary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrCodeInternalError,
			Message: "绑定钱包失败",
			Detail:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, LinkWalletResponse{
		Success:   true,
		Message:   "钱包绑定成功",
		Address:   req.Address,
		IsPrimary: req.IsPrimary,
	})
}

// UnlinkWallet 解绑钱包（JWT保护）
func (h *Handler) UnlinkWallet(c *gin.Context) {
	// 从JWT中获取用户ID
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    ErrCodeUnauthorized,
			Message: "未认证",
		})
		return
	}

	address := c.Param("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidAddress,
			Message: "地址不能为空",
		})
		return
	}

	// 验证地址格式
	if err := web3_auth.ValidateAddress(address); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrCodeInvalidAddress,
			Message: "地址格式错误",
			Detail:  err.Error(),
		})
		return
	}

	// 解绑钱包
	err := h.walletRepo.UnlinkWallet(userID, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrCodeInternalError,
			Message: "解绑钱包失败",
			Detail:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, UnlinkWalletResponse{
		Success: true,
		Message: "钱包解绑成功",
		Address: address,
	})
}

// ListWallets 列出用户的所有钱包（JWT保护）
func (h *Handler) ListWallets(c *gin.Context) {
	// 从JWT中获取用户ID
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    ErrCodeUnauthorized,
			Message: "未认证",
		})
		return
	}

	wallets, err := h.walletRepo.GetUserWallets(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrCodeInternalError,
			Message: "查询钱包列表失败",
			Detail:  err.Error(),
		})
		return
	}

	// 构建响应
	var walletInfos []UserWalletInfo
	for _, w := range wallets {
		walletInfo := UserWalletInfo{
			ID:         w.ID,
			Address:    w.WalletAddr,
			IsPrimary:  w.IsPrimary,
			BoundAt:    w.BoundAt,
			LastUsedAt: w.LastUsedAt,
		}
		walletInfos = append(walletInfos, walletInfo)
	}

	c.JSON(http.StatusOK, ListWalletsResponse{
		Success: true,
		Wallets: walletInfos,
	})
}
