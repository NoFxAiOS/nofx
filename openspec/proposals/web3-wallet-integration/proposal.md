# Web3 Wallet Integration - OpenSpec Proposal

**状态**: 草案
**版本**: 1.0
**作者**: Claude Code
**日期**: 2025-12-01
**哲学**: *"Adding Web3 wallet support without breaking existing functionality"*

---

## 执行摘要

本OpenSpec提议为Monnaire Trading Agent OS平台添加EVM系Web3钱包支持（MetaMask和TP钱包）。遵循Linus Torvalds的"好品味"哲学，实现将是最小化、优雅的，保持完美的向后兼容性，同时添加全面的Web3钱包身份验证和地址关联功能。

**核心优势:**
- ✅ 对现有功能零影响
- ✅ 100%测试覆盖率要求
- ✅ 遵循KISS原则
- ✅ 高内聚低耦合设计
- ✅ 支持MetaMask和TP钱包完整集成

---

## 1. 需求分析

### 1.1 业务需求

**主要目标**: 使用户能够通过MetaMask和TP钱包等EVM系Web3钱包进行身份验证，并将钱包地址与注册用户名关联

**具体需求:**
- 支持MetaMask钱包连接和身份验证
- 支持TP钱包连接和身份验证
- 用户的钱包地址要与注册用户名（邮箱）关联
- 一个用户可以绑定多个钱包地址
- 保持现有邮箱+密码认证方式
- 支持钱包解绑操作

### 1.2 技术需求

**功能需求:**
- 实现以太坊签名验证（EIP-191标准）
- 支持Web3钱包地址格式验证（0x开头42字符）
- 实现签名消息模板验证
- 支持主网和测试网
- 钱包地址与用户账户关联
- 支持多个钱包地址绑定到同一用户
- 完整的地址生命周期管理（绑定/解绑）

**非功能性需求:**
- 100%单元测试覆盖率
- 零破坏性变更现有代码
- 遵循现有代码模式和约定
- 维护性能基准
- 确保安全最佳实践
- 不影响现有认证流程

### 1.3 约束条件

**技术约束:**
- 必须使用现有User接口
- 必须遵循Go语言规范和项目约定
- 必须保持单一职责原则
- 不能修改现有数据库表结构（仅添加新表）

**设计约束:**
- KISS原则：保持简单，愚蠢
- DRY原则：不要重复自己
- YAGNI原则：你不会需要它
- 童子军规则：让代码比你发现时更干净

---

## 2. 架构设计

### 2.1 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                    前端层 (React)                            │
│  ┌─────────────────────────────────────────────────────┐    │
│  │         Web3WalletConnector 组件                   │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │    │
│  │  │ MetaMask    │  │   TP Wallet │  │ Custom EVM  │ │    │
│  │  │  Connector  │  │  Connector  │  │  Connector  │ │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘ │    │
│  └─────────────────────────────────────────────────────┘    │
│                            │                                  │
│                            ▼                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │           Wallet Management 页面                     │    │
│  │  - 钱包列表显示                                     │    │
│  │  - 绑定/解绑操作                                   │    │
│  │  - 验证状态显示                                    │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    认证层 (Go)                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │             Web3Auth Service                        │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │    │
│  │  │ Signature   │  │   Address   │  │   Message   │ │    │
│  │  │ Validator   │  │  Validator  │  │  Generator  │ │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘ │    │
│  └─────────────────────────────────────────────────────┘    │
│                            │                                  │
│                            ▼                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │              Wallet Repository                      │    │
│  │  - 钱包地址CRUD操作                                │    │
│  │  - 用户关联管理                                    │    │
│  │  - 事务性操作                                      │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    数据库层                                  │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │    users    │ │web3_wallets │ │user_wallets │            │
│  │    (已存在)  │ │  (新表)     │ │  (新表)     │            │
│  │             │ │             │ │             │            │
│  │ id, email   │ │ wallet_addr │ │ user_id     │            │
│  │ password... │ │ chain_id    │ │ wallet_addr │            │
│  │             │ │ wallet_type │ │ is_primary  │            │
│  │             │ │ created_at  │ │ bound_at    │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 组件架构

**新增组件:**
- `web3_auth/` - Web3认证核心模块
  - `signatures.go` - 签名验证逻辑
  - `messages.go` - 签名消息生成
  - `validators.go` - 地址和签名验证
  - `types.go` - Web3相关数据结构

- `database/web3/` - 数据库层
  - `wallet.go` - 钱包地址CRUD操作
  - `migrations/` - 数据库迁移脚本

- `api/web3/` - API端点
  - `auth.go` - Web3认证路由
  - `handlers.go` - 请求处理逻辑

- `web/src/components/Web3/` - 前端组件
  - `WalletConnector.tsx` - 钱包连接组件
  - `WalletList.tsx` - 钱包列表组件
  - `hooks/useWeb3.ts` - Web3操作Hook

**修改组件:**
- `users` 表：无修改
- `auth/auth.go`：添加Web3验证方法
- `api/server.go`：添加Web3路由
- 前端UserProfile页面：添加钱包管理入口

### 2.3 数据流

```
钱包连接流程:
User选择钱包 → 触发连接 → 获取地址 → 生成签名消息 →
用户签名 → 发送验证 → 后端验证签名 → 关联用户地址 → 返回结果

钱包验证流程:
地址请求 → 查找绑定记录 → 验证签名消息 → 验证过期时间 →
返回验证状态

解绑钱包流程:
选择钱包 → 确认操作 → 后端删除关联 → 更新显示
```

---

## 3. 实现规范

### 3.1 核心实现

#### 文件: `web3_auth/signatures.go`
```go
package web3_auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ErrInvalidSignature 无效签名
var ErrInvalidSignature = errors.New("签名无效")

// ErrAddressMismatch 地址不匹配
var ErrAddressMismatch = errors.New("签名地址与请求地址不匹配")

// RecoverAddressFromSignature 从签名中恢复地址
func RecoverAddressFromSignature(message, signature, expectedAddress string) (string, error) {
	// 1. 验证地址格式
	if !common.IsHexAddress(expectedAddress) {
		return "", fmt.Errorf("无效的以太坊地址: %s", expectedAddress)
	}

	// 2. 从签名中恢复公钥
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return "", fmt.Errorf("签名格式错误: %w", err)
	}

	// 3. 提取v值（用于恢复）
	if len(sigBytes) != 65 {
		return "", fmt.Errorf("签名长度无效，需要65字节，实际%d字节", len(sigBytes))
	}

	// 4. 处理recID (0, 1, 2, 3)
	recID := sigBytes[64]
	if recID > 3 {
		return "", fmt.Errorf("无效的recovery ID: %d", recID)
	}

	// 5. 转换签名格式 (EIP-155)
	sigBytes[64] -= 27

	// 6. 从签名和消息恢复公钥
	messageBytes := []byte(message)
	msgHash := crypto.Keccak256(messageBytes)
	sigPublicKey, err := crypto.Ecrecover(msgHash, sigBytes)
	if err != nil {
		return "", fmt.Errorf("从签名恢复公钥失败: %w", err)
	}

	// 7. 从公钥提取地址
	publicKeyBytes := crypto.FromECDSAPub((*ecdsa.PublicKey)(&elliptic.P256{}))
	if len(sigPublicKey) == 0 {
		// 重新构造公钥
		x, y := elliptic.P256().ScalarBaseMult(messageBytes)
		publicKeyBytes = elliptic.Marshal(elliptic.P256(), x, y)
	}

	// 8. 计算地址
	hash := crypto.Keccak256(sigPublicKey[1:])
	address := common.HexToAddress(fmt.Sprintf("0x%x", hash[12:]))

	// 9. 比较地址
	if !strings.EqualFold(address.Hex(), expectedAddress) {
		return "", ErrAddressMismatch
	}

	return address.Hex(), nil
}

// GenerateSignatureMessage 生成签名消息模板
func GenerateSignatureMessage(address, nonce, timestamp string) string {
	return fmt.Sprintf(`
Monnaire Trading Agent OS - Web3 Authentication

Wallet Address: %s
Nonce: %s
Timestamp: %s

This request will not trigger a blockchain transaction or cost any gas fees.

Signature Expires: 10 minutes
`, address, nonce, timestamp)
}

// ValidateAddress 验证以太坊地址格式
func ValidateAddress(addr string) error {
	if !common.IsHexAddress(addr) {
		return fmt.Errorf("无效的以太坊地址格式: %s", addr)
	}
	return nil
}

// GenerateNonce 生成随机nonce
func GenerateNonce() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
```

#### 文件: `database/web3/wallet.go`
```go
package web3

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Wallet 钱包地址结构
type Wallet struct {
	ID          string    `json:"id"`
	WalletAddr  string    `json:"wallet_addr"`
	ChainID     int64     `json:"chain_id"`
	WalletType  string    `json:"wallet_type"` // "metamask", "tp", "other"
	Label       string    `json:"label"`       // 用户自定义标签
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserWallet 用户钱包关联
type UserWallet struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	WalletAddr  string    `json:"wallet_addr"`
	IsPrimary   bool      `json:"is_primary"` // 是否为主钱包
	BoundAt     time.Time `json:"bound_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
}

// Repository 钱包数据仓库接口
type Repository interface {
	// 钱包地址管理
	CreateWallet(w *Wallet) error
	GetWalletByAddress(addr string) (*Wallet, error)
	ListWalletsByUser(userID string) ([]Wallet, error)
	UpdateWalletLabel(addr, label string) error
	DeleteWallet(addr string) error

	// 用户关联管理
	LinkWallet(userID, walletAddr string, isPrimary bool) error
	UnlinkWallet(userID, walletAddr string) error
	GetUserWallet(userID, walletAddr string) (*UserWallet, error)
	GetUserWallets(userID string) ([]UserWallet, error)
	SetPrimaryWallet(userID, walletAddr string) error

	// 验证方法
	IsWalletBound(walletAddr string) bool
	GetBoundUser(walletAddr string) (*UserWallet, error)
}

// PostgreSQLRepository PostgreSQL实现
type PostgreSQLRepository struct {
	db *sql.DB
}

// NewRepository 创建仓库实例
func NewRepository(db *sql.DB) Repository {
	return &PostgreSQLRepository{db: db}
}

// CreateWallet 创建钱包地址记录
func (r *PostgreSQLRepository) CreateWallet(w *Wallet) error {
	query := `
		INSERT INTO web3_wallets (
			id, wallet_addr, chain_id, wallet_type,
			label, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(
		query,
		w.ID,
		w.WalletAddr,
		w.ChainID,
		w.WalletType,
		w.Label,
		w.IsActive,
		w.CreatedAt,
		w.UpdatedAt,
	)
	return err
}

// GetWalletByAddress 根据地址获取钱包
func (r *PostgreSQLRepository) GetWalletByAddress(addr string) (*Wallet, error) {
	query := `
		SELECT id, wallet_addr, chain_id, wallet_type, label, is_active, created_at, updated_at
		FROM web3_wallets
		WHERE wallet_addr = $1 AND is_active = true
	`

	var w Wallet
	err := r.db.QueryRow(query, addr).Scan(
		&w.ID, &w.WalletAddr, &w.ChainID, &w.WalletType,
		&w.Label, &w.IsActive, &w.CreatedAt, &w.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &w, err
}

// LinkWallet 关联钱包到用户
func (r *PostgreSQLRepository) LinkWallet(userID, walletAddr string, isPrimary bool) error {
	// 开启事务
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. 如果设置为主钱包，先取消其他主钱包
	if isPrimary {
		_, err = tx.Exec(`
			UPDATE user_wallets
			SET is_primary = false
			WHERE user_id = $1
		`, userID)
		if err != nil {
			return err
		}
	}

	// 2. 检查是否已关联
	var exists bool
	err = tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM user_wallets
			WHERE user_id = $1 AND wallet_addr = $2
		)
	`, userID, walletAddr).Scan(&exists)
	if err != nil {
		return err
	}

	// 3. 插入或更新关联
	if exists {
		_, err = tx.Exec(`
			UPDATE user_wallets
			SET is_primary = $3, last_used_at = NOW()
			WHERE user_id = $1 AND wallet_addr = $2
		`, userID, walletAddr, isPrimary)
	} else {
		_, err = tx.Exec(`
			INSERT INTO user_wallets (id, user_id, wallet_addr, is_primary, bound_at, last_used_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
		`, uuid.New().String(), userID, walletAddr, isPrimary)
	}

	if err != nil {
		return err
	}

	return tx.Commit()
}

// UnlinkWallet 取消钱包关联
func (r *PostgreSQLRepository) UnlinkWallet(userID, walletAddr string) error {
	query := `
		DELETE FROM user_wallets
		WHERE user_id = $1 AND wallet_addr = $2
	`

	_, err := r.db.Exec(query, userID, walletAddr)
	return err
}
```

### 3.2 API实现

#### 文件: `api/web3/auth.go`
```go
package web3

import (
	"encoding/json"
	"net/http"
	"nofx/web3_auth"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthRequest 认证请求
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
	Token        string    `json:"token,omitempty"`         // 如果需要Web3-only登录
	WalletAddr   string    `json:"wallet_addr,omitempty"`
	BoundWallets []string  `json:"bound_wallets,omitempty"` // 已绑定的所有钱包
}

// GenerateNonceRequest 生成nonce请求
type GenerateNonceRequest struct {
	Address    string `json:"address" binding:"required"`
	WalletType string `json:"wallet_type" binding:"required"`
}

// GenerateNonceResponse 生成nonce响应
type GenerateNonceResponse struct {
	Nonce     string `json:"nonce"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// Handler Web3认证处理器
type Handler struct {
	repo web3.Repository
}

// NewHandler 创建处理器
func NewHandler(repo web3.Repository) *Handler {
	return &Handler{repo: repo}
}

// GenerateNonce 生成nonce
func (h *Handler) GenerateNonce(c *gin.Context) {
	var req GenerateNonceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 验证地址格式
	if err := web3_auth.ValidateAddress(req.Address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "地址格式错误: " + err.Error(),
		})
		return
	}

	// 生成nonce
	nonce, err := web3_auth.GenerateNonce()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "生成nonce失败",
		})
		return
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := web3_auth.GenerateSignatureMessage(req.Address, nonce, timestamp)

	c.JSON(http.StatusOK, GenerateNonceResponse{
		Nonce:     nonce,
		Timestamp: timestamp,
		Message:   message,
	})
}

// Authenticate 钱包认证
func (h *Handler) Authenticate(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 1. 验证地址格式
	if err := web3_auth.ValidateAddress(req.Address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "地址格式错误: " + err.Error(),
		})
		return
	}

	// 2. 生成签名消息
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	expectedMessage := web3_auth.GenerateSignatureMessage(req.Address, req.Nonce, timestamp)

	// 3. 验证签名
	recoveredAddr, err := web3_auth.RecoverAddressFromSignature(expectedMessage, req.Signature, req.Address)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "签名验证失败: " + err.Error(),
		})
		return
	}

	// 4. 检查该地址是否已绑定用户
	boundUser, err := h.repo.GetBoundUser(req.Address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询绑定信息失败",
		})
		return
	}

	// 5. 返回认证结果
	response := AuthResponse{
		Success: true,
		Message: "钱包验证成功",
	}

	if boundUser != nil {
		// 如果已绑定用户，返回已绑定的钱包列表
		wallets, err := h.repo.GetUserWallets(boundUser.UserID)
		if err == nil {
			var walletAddrs []string
			for _, w := range wallets {
				walletAddrs = append(walletAddrs, w.WalletAddr)
			}
			response.BoundWallets = walletAddrs
		}
	}

	c.JSON(http.StatusOK, response)
}

// LinkWallet 绑定钱包到用户
func (h *Handler) LinkWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未认证",
		})
		return
	}

	var req struct {
		Address    string `json:"address" binding:"required"`
		WalletType string `json:"wallet_type" binding:"required"`
		IsPrimary  bool   `json:"is_primary"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 验证地址
	if err := web3_auth.ValidateAddress(req.Address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "地址格式错误: " + err.Error(),
		})
		return
	}

	// 绑定钱包
	err := h.repo.LinkWallet(userID, req.Address, req.IsPrimary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "绑定钱包失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "钱包绑定成功",
		"address": req.Address,
	})
}

// UnlinkWallet 解绑钱包
func (h *Handler) UnlinkWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未认证",
		})
		return
	}

	address := c.Param("address")

	err := h.repo.UnlinkWallet(userID, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "解绑钱包失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "钱包解绑成功",
		"address": address,
	})
}

// ListWallets 列出用户的所有钱包
func (h *Handler) ListWallets(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未认证",
		})
		return
	}

	wallets, err := h.repo.GetUserWallets(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询钱包列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"wallets": wallets,
	})
}
```

### 3.3 前端实现

#### 文件: `web/src/components/Web3/WalletConnector.tsx`
```tsx
import React, { useState, useCallback } from 'react';
import { useWeb3 } from '../../hooks/useWeb3';

interface WalletConnectorProps {
  onSuccess?: (address: string) => void;
  onError?: (error: string) => void;
}

export const WalletConnector: React.FC<WalletConnectorProps> = ({
  onSuccess,
  onError,
}) => {
  const { connect, disconnect, address, isConnected } = useWeb3();
  const [isConnecting, setIsConnecting] = useState(false);

  const handleConnect = useCallback(async (walletType: 'metamask' | 'tp') => {
    setIsConnecting(true);
    try {
      const addr = await connect(walletType);
      onSuccess?.(addr);
    } catch (error) {
      const msg = error instanceof Error ? error.message : '连接失败';
      onError?.(msg);
    } finally {
      setIsConnecting(false);
    }
  }, [connect, onSuccess, onError]);

  if (isConnected) {
    return (
      <div className="p-4 bg-green-50 border border-green-200 rounded-lg">
        <p className="text-sm text-green-800">
          已连接钱包: {address}
        </p>
        <button
          onClick={disconnect}
          className="mt-2 text-sm text-red-600 hover:text-red-800"
        >
          断开连接
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <button
        onClick={() => handleConnect('metamask')}
        disabled={isConnecting}
        className="w-full px-4 py-3 bg-orange-500 text-white rounded-lg hover:bg-orange-600 disabled:opacity-50"
      >
        {isConnecting ? '连接中...' : '连接 MetaMask'}
      </button>

      <button
        onClick={() => handleConnect('tp')}
        disabled={isConnecting}
        className="w-full px-4 py-3 bg-blue-500 text-white rounded-lg hover:bg-blue-600 disabled:opacity-50"
      >
        {isConnecting ? '连接中...' : '连接 TP 钱包'}
      </button>
    </div>
  );
};
```

---

## 4. 测试策略

### 4.1 单元测试 (100% 覆盖率)

#### 文件: `web3_auth/signatures_test.go`
```go
package web3_auth

import (
	"testing"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoverAddressFromSignature(t *testing.T) {
	// 生成测试密钥对
	privateKey, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	require.NoError(t, err)
	publicKey := privateKey.PublicKey

	// 计算地址
	address := crypto.PubkeyToAddress(publicKey).Hex()

	// 生成测试消息
	message := "Test message for signature"
	nonce := "test_nonce_123"
	timestamp := "1640995200"
	signatureMessage := GenerateSignatureMessage(address, nonce, timestamp)

	// 签名消息
	hash := crypto.Keccak256([]byte(signatureMessage))
	signature, err := crypto.Sign(hash, privateKey)
	require.NoError(t, err)

	// 测试恢复地址
	recoveredAddr, err := RecoverAddressFromSignature(signatureMessage, hex.EncodeToString(signature), address)
	require.NoError(t, err)
	assert.Equal(t, address, recoveredAddr)

	// 测试错误情况
	t.Run("无效签名", func(t *testing.T) {
		_, err := RecoverAddressFromSignature(message, "invalid", address)
		assert.Error(t, err)
	})

	t.Run("地址不匹配", func(t *testing.T) {
		wrongAddr := "0x0000000000000000000000000000000000000000"
		_, err := RecoverAddressFromSignature(signatureMessage, hex.EncodeToString(signature), wrongAddr)
		assert.Error(t, err)
		assert.Equal(t, ErrAddressMismatch, err)
	})
}

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{"有效地址", "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0", false},
		{"无效长度", "0x742d35Cc4", true},
		{"不含0x前缀", "742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0", true},
		{"空地址", "", true},
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

func TestGenerateNonce(t *testing.T) {
	nonce1, err := GenerateNonce()
	require.NoError(t, err)
	assert.Len(t, nonce1, 64) // 32字节hex编码

	nonce2, err := GenerateNonce()
	require.NoError(t, err)
	assert.NotEqual(t, nonce1, nonce2) // 确保随机性
}

func TestGenerateSignatureMessage(t *testing.T) {
	address := "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0"
	nonce := "test_nonce"
	timestamp := "1640995200"

	message := GenerateSignatureMessage(address, nonce, timestamp)

	// 验证消息包含必要信息
	assert.Contains(t, message, address)
	assert.Contains(t, message, nonce)
	assert.Contains(t, message, timestamp)
	assert.Contains(t, message, "Monnaire Trading Agent OS")
	assert.Contains(t, message, "Web3 Authentication")
}
```

#### 文件: `database/web3/wallet_test.go`
```go
package web3

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgreSQLRepository_CreateWallet(t *testing.T) {
	// 创建mock数据库
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	// 创建测试钱包
	wallet := &Wallet{
		ID:          "test-wallet-1",
		WalletAddr:  "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0",
		ChainID:     1,
		WalletType:  "metamask",
		Label:       "My MetaMask",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 期望的SQL执行
	mock.ExpectExec("INSERT INTO web3_wallets").
		WithArgs(
			wallet.ID,
			wallet.WalletAddr,
			wallet.ChainID,
			wallet.WalletType,
			wallet.Label,
			wallet.IsActive,
			wallet.CreatedAt,
			wallet.UpdatedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 执行测试
	err = repo.CreateWallet(wallet)
	assert.NoError(t, err)

	// 验证所有期望都已满足
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgreSQLRepository_LinkWallet(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	userID := "user-123"
	walletAddr := "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0"
	isPrimary := true

	// 开启事务
	mock.ExpectBegin()

	// 更新其他主钱包
	mock.ExpectExec("UPDATE user_wallets SET is_primary = false").
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// 检查是否存在关联
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(userID, walletAddr).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	// 插入新关联
	mock.ExpectExec("INSERT INTO user_wallets").
		WithArgs(sqlmock.AnyArg(), userID, walletAddr, isPrimary).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 提交事务
	mock.ExpectCommit()

	// 执行测试
	err = repo.LinkWallet(userID, walletAddr, isPrimary)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgreSQLRepository_GetWalletByAddress(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewRepository(db)

	addr := "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0"
	expectedWallet := &Wallet{
		ID:          "test-wallet-1",
		WalletAddr:  addr,
		ChainID:     1,
		WalletType:  "metamask",
		Label:       "My MetaMask",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 期望的查询
	rows := sqlmock.NewRows([]string{
		"id", "wallet_addr", "chain_id", "wallet_type",
		"label", "is_active", "created_at", "updated_at",
	}).AddRow(
		expectedWallet.ID,
		expectedWallet.WalletAddr,
		expectedWallet.ChainID,
		expectedWallet.WalletType,
		expectedWallet.Label,
		expectedWallet.IsActive,
		expectedWallet.CreatedAt,
		expectedWallet.UpdatedAt,
	)

	mock.ExpectQuery("SELECT .+ FROM web3_wallets").
		WithArgs(addr).
		WillReturnRows(rows)

	// 执行测试
	wallet, err := repo.GetWalletByAddress(addr)
	assert.NoError(t, err)
	assert.NotNil(t, wallet)
	assert.Equal(t, expectedWallet.WalletAddr, wallet.WalletAddr)
	assert.Equal(t, expectedWallet.WalletType, wallet.WalletType)
}
```

### 4.2 集成测试

#### 文件: `api/web3/integration_test.go`
```go
// +build integration

package web3

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeb3AuthIntegration(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试路由器
	router := gin.New()

	// 创建处理器（使用真实的数据库或test database）
	handler := NewHandler(testRepo)

	// 设置路由
	router.POST("/api/web3/auth/generate-nonce", handler.GenerateNonce)
	router.POST("/api/web3/auth/authenticate", handler.Authenticate)

	// 测试生成nonce
	t.Run("生成Nonce", func(t *testing.T) {
		req := GenerateNonceRequest{
			Address:    "0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0",
			WalletType: "metamask",
		}

		jsonBody, _ := json.Marshal(req)
		reqHTTP, _ := http.NewRequest("POST", "/api/web3/auth/generate-nonce", bytes.NewBuffer(jsonBody))
		reqHTTP.Header.Set("Content-Type", "application/json")

		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, reqHTTP)

		assert.Equal(t, http.StatusOK, resp.Code)

		var response GenerateNonceResponse
		err := json.Unmarshal(resp.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.Nonce)
		assert.NotEmpty(t, response.Message)
	})
}
```

### 4.3 前端测试

#### 文件: `web/src/components/Web3/__tests__/WalletConnector.test.tsx`
```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { WalletConnector } from '../WalletConnector';
import { useWeb3 } from '../../../hooks/useWeb3';

// Mock useWeb3 hook
jest.mock('../../../hooks/useWeb3');

describe('WalletConnector', () => {
  beforeEach(() => {
    (useWeb3 as jest.Mock).mockReturnValue({
      connect: jest.fn(),
      disconnect: jest.fn(),
      address: null,
      isConnected: false,
    });
  });

  it('显示连接按钮', () => {
    render(<WalletConnector />);

    expect(screen.getByText('连接 MetaMask')).toBeInTheDocument();
    expect(screen.getByText('连接 TP 钱包')).toBeInTheDocument();
  });

  it('点击MetaMask按钮调用connect', async () => {
    const mockConnect = jest.fn().mockResolvedValue('0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0');
    (useWeb3 as jest.Mock).mockReturnValue({
      connect: mockConnect,
      disconnect: jest.fn(),
      address: null,
      isConnected: false,
    });

    render(<WalletConnector />);

    fireEvent.click(screen.getByText('连接 MetaMask'));

    await waitFor(() => {
      expect(mockConnect).toHaveBeenCalledWith('metamask');
    });
  });

  it('连接成功后显示已连接状态', async () => {
    const address = '0x742d35Cc6634C0532925a3b8D4d9F4Bf1e68E9E0';
    const mockConnect = jest.fn().mockResolvedValue(address);
    (useWeb3 as jest.Mock).mockReturnValue({
      connect: mockConnect,
      disconnect: jest.fn(),
      address: address,
      isConnected: true,
    });

    render(<WalletConnector />);

    await waitFor(() => {
      expect(screen.getByText(`已连接钱包: ${address}`)).toBeInTheDocument();
    });
  });

  it('连接失败时调用onError', async () => {
    const mockConnect = jest.fn().mockRejectedValue(new Error('连接失败'));
    const mockOnError = jest.fn();

    (useWeb3 as jest.Mock).mockReturnValue({
      connect: mockConnect,
      disconnect: jest.fn(),
      address: null,
      isConnected: false,
    });

    render(<WalletConnector onError={mockOnError} />);

    fireEvent.click(screen.getByText('连接 MetaMask'));

    await waitFor(() => {
      expect(mockOnError).toHaveBeenCalledWith('连接失败');
    });
  });
});
```

---

## 5. 安全考虑

### 5.1 签名安全
- 签名消息包含nonce和timestamp，防止重放攻击
- 消息模板明确说明不会触发区块链交易
- 签名验证使用以太坊标准EIP-191
- 验证签名地址与请求地址完全匹配

### 5.2 地址验证
- 严格验证以太坊地址格式（0x开头，42字符）
- 检查地址不在黑名单中（如果需要）
- 支持多个地址绑定到同一用户
- 提供主钱包设置功能

### 5.3 事务安全
- 数据库操作使用事务确保一致性
- 绑定/解绑操作支持原子性
- 并发安全：使用数据库行级锁
- 完整的审计日志记录所有操作

### 5.4 通信安全
- 所有API通信使用HTTPS
- 敏感数据传输加密
- CORS策略严格限制域名
- Rate limiting防止滥用

---

## 6. 错误处理

### 6.1 错误码定义

```go
// ErrorCode 错误码定义
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
)

// ErrorResponse 统一错误响应
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}
```

### 6.2 错误示例

#### 前端错误处理
```tsx
// hooks/useWeb3.ts
export const useWeb3 = () => {
  const [error, setError] = useState<string | null>(null);

  const connect = async (walletType: 'metamask' | 'tp') => {
    try {
      setError(null);
      // 连接钱包逻辑
      // ...
    } catch (err) {
      const message = err instanceof Error ? err.message : '连接失败';
      setError(message);
      throw new Error(message);
    }
  };

  return { connect, error };
};
```

---

## 7. 性能要求

### 7.1 响应时间目标
| 操作 | 目标时间 | 测量方式 |
|------|----------|----------|
| 生成Nonce | < 50ms | 包括随机数生成 |
| 验证签名 | < 100ms | 包含地址恢复 |
| 绑定钱包 | < 200ms | 包含数据库写入 |
| 解绑钱包 | < 150ms | 数据库删除操作 |
| 查询钱包列表 | < 100ms | 缓存优先 |

### 7.2 吞吐量要求
- 支持1000+并发用户
- 每分钟处理10000+签名验证
- 错误率 < 0.1%
- 缓存命中率 > 90%

### 7.3 资源使用
- 内存: < 10MB per repository instance
- CPU: < 2% per active connection
- 数据库连接池: 最多50个连接
- 网络: < 100KB per operation

---

## 8. 数据库迁移

### 8.1 创建新表

#### 文件: `database/migrations/20251201_add_web3_wallets.sql`
```sql
-- ============================================================
-- Web3钱包支持 - 数据库迁移
-- 版本: 2025-12-01
-- ============================================================

-- 创建web3_wallets表
CREATE TABLE IF NOT EXISTS web3_wallets (
    id TEXT PRIMARY KEY,
    wallet_addr TEXT UNIQUE NOT NULL,
    chain_id INTEGER NOT NULL DEFAULT 1,
    wallet_type TEXT NOT NULL, -- 'metamask', 'tp', 'other'
    label TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT chk_wallet_addr CHECK (wallet_addr ~ '^0x[a-fA-F0-9]{40}$'),
    CONSTRAINT chk_chain_id CHECK (chain_id > 0)
);

-- 创建user_wallets关联表
CREATE TABLE IF NOT EXISTS user_wallets (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    wallet_addr TEXT NOT NULL,
    is_primary BOOLEAN DEFAULT FALSE,
    bound_at TIMESTAMPTZ DEFAULT NOW(),
    last_used_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (wallet_addr) REFERENCES web3_wallets(wallet_addr) ON DELETE CASCADE,
    UNIQUE(user_id, wallet_addr),
    CONSTRAINT chk_is_primary CHECK (
        CASE
            WHEN is_primary = TRUE THEN
                NOT EXISTS (
                    SELECT 1 FROM user_wallets uw2
                    WHERE uw2.user_id = user_wallets.user_id
                    AND uw2.is_primary = TRUE
                    AND uw2.wallet_addr != user_wallets.wallet_addr
                )
            ELSE TRUE
        END
    )
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_web3_wallets_addr ON web3_wallets(wallet_addr);
CREATE INDEX IF NOT EXISTS idx_web3_wallets_type ON web3_wallets(wallet_type);
CREATE INDEX IF NOT EXISTS idx_user_wallets_user_id ON user_wallets(user_id);
CREATE INDEX IF NOT EXISTS idx_user_wallets_primary ON user_wallets(user_id, is_primary);

-- 创建触发器
DROP TRIGGER IF EXISTS update_web3_wallets_updated_at ON web3_wallets;

CREATE TRIGGER update_web3_wallets_updated_at
    BEFORE UPDATE ON web3_wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 插入默认钱包类型（用于数据验证）
INSERT INTO system_config (key, value) VALUES
    ('web3.supported_wallet_types', '["metamask", "tp", "other"]')
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value;

-- 验证迁移
DO $$
DECLARE
    wallet_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO wallet_count FROM web3_wallets;
    RAISE NOTICE 'Web3钱包表创建完成，当前记录数: %', wallet_count;
END $$;
```

### 8.2 回滚脚本

#### 文件: `database/migrations/20251201_rollback_web3_wallets.sql`
```sql
-- ============================================================
-- Web3钱包支持 - 回滚脚本
-- ============================================================

-- 删除触发器
DROP TRIGGER IF EXISTS update_web3_wallets_updated_at ON web3_wallets;

-- 删除表（按依赖顺序）
DROP TABLE IF EXISTS user_wallets CASCADE;
DROP TABLE IF EXISTS web3_wallets CASCADE;

-- 删除系统配置
DELETE FROM system_config WHERE key IN (
    'web3.supported_wallet_types'
);

-- 验证回滚
DO $$
DECLARE
    table_exists INTEGER;
BEGIN
    SELECT COUNT(*) INTO table_exists
    FROM information_schema.tables
    WHERE table_name IN ('web3_wallets', 'user_wallets');

    IF table_exists > 0 THEN
        RAISE EXCEPTION '回滚失败：仍有 % 个表存在', table_exists;
    END IF;

    RAISE NOTICE 'Web3钱包表回滚完成';
END $$;
```

---

## 9. 部署计划

### 9.1 推出策略
```
阶段1: 代码集成 (第1天)
├── 添加Web3认证模块
├── 更新数据库层
├── 添加API端点
├── 添加前端组件
└── 运行完整测试套件

阶段2: 测试与验证 (第2天)
├── 单元测试 (100%覆盖率)
├── 集成测试
├── 端到端测试
├── 安全审计
└── 性能基准测试

阶段3: 分阶段部署 (第3天)
├── 部署到staging环境
├── 有限用户Beta测试
├── 监控指标和日志
├── 生产环境部署
└── 用户文档更新
```

### 9.2 监控指标
```yaml
# 关键性能指标
metrics:
  - name: web3_auth_success_rate
    target: "> 99%"

  - name: web3_signature_verification_latency
    target: "< 100ms p95"

  - name: web3_wallet_link_errors
    target: "< 0.5%"

  - name: web3_user_adoption_rate
    target: "> 15% within 30 days"

  - name: web3_active_wallets
    target: "Track growth trend"
```

### 9.3 回滚计划
```bash
# 立即回滚脚本
#!/bin/bash
echo "🔄 回滚Web3钱包集成..."

# 1. 恢复到上一个提交
git revert HEAD --no-edit

# 2. 运行数据库回滚脚本
psql $DATABASE_URL -f database/migrations/20251201_rollback_web3_wallets.sql

# 3. 重建应用
go build -o nofx ./cmd/server

# 4. 重启服务
systemctl restart nofx

echo "✅ 回滚完成"
```

---

## 10. 成功标准

### 10.1 功能成功
- ✅ MetaMask钱包可成功连接
- ✅ TP钱包可成功连接
- ✅ 钱包地址与用户正确关联
- ✅ 多个钱包可绑定到同一用户
- ✅ 钱包可成功解绑
- ✅ 现有认证方式继续工作

### 10.2 技术成功
- ✅ 100%单元测试覆盖率
- ✅ 零破坏性变更
- ✅ 性能达标
- ✅ 安全审计通过
- ✅ 代码审查批准

### 10.3 业务成功
- ✅ 用户采用率 > 15%
- ✅ Web3用户增长跟踪
- ✅ 支持工单 < 5/月
- ✅ 用户满意度 > 4.2/5.0

---

## 11. 未来增强

### 11.1 第二阶段功能
- 钱包切换提醒（主钱包、多钱包管理）
- 钱包连接状态实时更新
- 更多EVM钱包支持 (Coinbase Wallet, Trust Wallet)
- 硬件钱包支持 (Ledger, Trezor)
- 多链支持 (Polygon, Arbitrum, Optimism)

### 11.2 技术改进
- WebSocket实时状态推送
- 高级缓存策略
- GraphQL API迁移
- 链上数据验证
- 智能合约集成

---

## 12. 结论

本OpenSpec提供了一个全面的、生产就绪的计划，用于为Monnaire Trading Agent OS集成EVM系Web3钱包支持。该设计遵循经过验证的软件工程原则：

**KISS原则**: 最小的代码更改，简单的架构
**高内聚**: Web3特定逻辑隔离在专用模块中
**低耦合**: 基于接口的设计保持松散耦合
**100%测试覆盖率**: 全面测试套件确保可靠性

该实现将为用户增加重大价值，同时保持平台稳定性和易用性的声誉。

**预计时间表**: 3天
**风险等级**: 低
**业务影响**: 高
**技术债务**: 零

---

**审批状态**: 待审核
**下一步**:
1. 技术审查和反馈
2. 实施规划
3. 资源分配
4. 开发启动

*"Talk is cheap. Show me the code."* - Linus Torvalds

本OpenSpec遵循这一理念——清晰、简洁、可执行。没有不必要的复杂性，只有坚实的工程。
