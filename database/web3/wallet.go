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
	WalletType  string    `json:"wallet_type"`
	Label       string    `json:"label"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserWallet 用户钱包关联
type UserWallet struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	WalletAddr  string    `json:"wallet_addr"`
	IsPrimary   bool      `json:"is_primary"`
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
		ON CONFLICT (wallet_addr) DO UPDATE SET
			label = EXCLUDED.label,
			updated_at = NOW()
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

// LinkWallet 关联钱包到用户（带事务安全）
func (r *PostgreSQLRepository) LinkWallet(userID, walletAddr string, isPrimary bool) error {
	// 开启事务
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 如果设置为主钱包，先取消其他主钱包
	if isPrimary {
		// 添加行级锁防止竞态条件（修复CVE-WS-018）
		_, err = tx.Exec(`
			UPDATE user_wallets
			SET is_primary = false
			WHERE user_id = $1 AND is_primary = true
			FOR UPDATE
		`, userID)
		if err != nil {
			return fmt.Errorf("取消其他主钱包失败: %w", err)
		}
	}

	// 检查是否已关联
	var exists bool
	err = tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM user_wallets
			WHERE user_id = $1 AND wallet_addr = $2
		)
	`, userID, walletAddr).Scan(&exists)
	if err != nil {
		return fmt.Errorf("检查关联状态失败: %w", err)
	}

	// 插入或更新关联
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
		return fmt.Errorf("关联钱包失败: %w", err)
	}

	return tx.Commit()
}

// UnlinkWallet 取消钱包关联
func (r *PostgreSQLRepository) UnlinkWallet(userID, walletAddr string) error {
	// 检查是否为最后一个钱包
	var walletCount int64
	err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM user_wallets
		WHERE user_id = $1
	`, userID).Scan(&walletCount)
	if err != nil {
		return fmt.Errorf("查询钱包数量失败: %w", err)
	}

	// 如果只有一个钱包且是主钱包，不允许解绑
	if walletCount == 1 {
		var isPrimary bool
		err := r.db.QueryRow(`
			SELECT is_primary
			FROM user_wallets
			WHERE user_id = $1 AND wallet_addr = $2
		`, userID, walletAddr).Scan(&isPrimary)
		if err == nil && isPrimary {
			return fmt.Errorf("无法解绑唯一的主钱包，请先设置其他钱包为主钱包")
		}
	}

	query := `
		DELETE FROM user_wallets
		WHERE user_id = $1 AND wallet_addr = $2
	`

	result, err := r.db.Exec(query, userID, walletAddr)
	if err != nil {
		return fmt.Errorf("解绑钱包失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("钱包未绑定或已被解绑")
	}

	return nil
}

// SetPrimaryWallet 设置主钱包（修复CVE-WS-018的竞态条件）
func (r *PostgreSQLRepository) SetPrimaryWallet(userID, walletAddr string) error {
	// 使用事务确保原子性
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 检查钱包是否属于用户
	var exists bool
	err = tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM user_wallets
			WHERE user_id = $1 AND wallet_addr = $2
		)
	`, userID, walletAddr).Scan(&exists)
	if err != nil {
		return fmt.Errorf("检查钱包归属失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("钱包不属于用户")
	}

	// 使用行级锁防止并发设置
	_, err = tx.Exec(`
		UPDATE user_wallets
		SET is_primary = false
		WHERE user_id = $1
		FOR UPDATE
	`, userID)
	if err != nil {
		return fmt.Errorf("取消其他主钱包失败: %w", err)
	}

	// 设置新的主钱包
	result, err := tx.Exec(`
		UPDATE user_wallets
		SET is_primary = true, last_used_at = NOW()
		WHERE user_id = $1 AND wallet_addr = $2
	`, userID, walletAddr)
	if err != nil {
		return fmt.Errorf("设置主钱包失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("设置主钱包失败，可能是并发操作导致")
	}

	return tx.Commit()
}

// GetUserWallets 获取用户的所有钱包
func (r *PostgreSQLRepository) GetUserWallets(userID string) ([]UserWallet, error) {
	query := `
		SELECT id, user_id, wallet_addr, is_primary, bound_at, last_used_at
		FROM user_wallets
		WHERE user_id = $1
		ORDER BY is_primary DESC, bound_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("查询钱包列表失败: %w", err)
	}
	defer rows.Close()

	var wallets []UserWallet
	for rows.Next() {
		var w UserWallet
		err := rows.Scan(
			&w.ID, &w.UserID, &w.WalletAddr,
			&w.IsPrimary, &w.BoundAt, &w.LastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描钱包记录失败: %w", err)
		}
		wallets = append(wallets, w)
	}

	return wallets, nil
}

// GetBoundUser 根据钱包地址获取绑定的用户
func (r *PostgreSQLRepository) GetBoundUser(walletAddr string) (*UserWallet, error) {
	query := `
		SELECT id, user_id, wallet_addr, is_primary, bound_at, last_used_at
		FROM user_wallets
		WHERE wallet_addr = $1
	`

	var uw UserWallet
	err := r.db.QueryRow(query, walletAddr).Scan(
		&uw.ID, &uw.UserID, &uw.WalletAddr,
		&uw.IsPrimary, &uw.BoundAt, &uw.LastUsedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &uw, err
}
