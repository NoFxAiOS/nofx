package web3

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Nonce 钱包认证nonce
type Nonce struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// NonceRepository nonce存储接口
type NonceRepository interface {
	// 创建nonce
	StoreNonce(address, nonce string, expiresAt time.Time) error

	// 验证nonce
	ValidateNonce(address, nonce string) error

	// 标记nonce为已使用
	MarkNonceUsed(address, nonce string) error

	// 清理过期nonce
	CleanupExpired() (int64, error)
}

// PostgreSQLNonceRepository PostgreSQL实现
type PostgreSQLNonceRepository struct {
	db *sql.DB
}

// NewNonceRepository 创建nonce仓库实例
func NewNonceRepository(db *sql.DB) NonceRepository {
	return &PostgreSQLNonceRepository{db: db}
}

// StoreNonce 存储nonce（修复CVE-WS-002）
func (r *PostgreSQLNonceRepository) StoreNonce(address, nonce string, expiresAt time.Time) error {
	// 验证参数
	if address == "" {
		return fmt.Errorf("地址不能为空")
	}
	if nonce == "" {
		return fmt.Errorf("nonce不能为空")
	}
	if expiresAt.Before(time.Now()) {
		return fmt.Errorf("过期时间不能是过去时间")
	}

	query := `
		INSERT INTO web3_wallet_nonces (id, address, nonce, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err := r.db.Exec(query, uuid.New().String(), address, nonce, expiresAt)
	return err
}

// ValidateNonce 验证nonce有效性（修复CVE-WS-010）
func (r *PostgreSQLNonceRepository) ValidateNonce(address, nonce string) error {
	if address == "" || nonce == "" {
		return fmt.Errorf("地址和nonce不能为空")
	}

	query := `
		SELECT expires_at, used
		FROM web3_wallet_nonces
		WHERE address = $1 AND nonce = $2
	`

	var expiresAt time.Time
	var used bool

	err := r.db.QueryRow(query, address, nonce).Scan(&expiresAt, &used)
	if err == sql.ErrNoRows {
		return fmt.Errorf("nonce不存在")
	}
	if err != nil {
		return fmt.Errorf("查询nonce失败: %w", err)
	}

	// 检查是否已使用
	if used {
		return fmt.Errorf("nonce已被使用")
	}

	// 检查是否过期
	if time.Now().After(expiresAt) {
		return fmt.Errorf("nonce已过期")
	}

	return nil
}

// MarkNonceUsed 标记nonce为已使用（防止重放攻击）
func (r *PostgreSQLNonceRepository) MarkNonceUsed(address, nonce string) error {
	query := `
		UPDATE web3_wallet_nonces
		SET used = TRUE
		WHERE address = $1 AND nonce = $2 AND used = FALSE
	`

	result, err := r.db.Exec(query, address, nonce)
	if err != nil {
		return fmt.Errorf("标记nonce失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("nonce不存在或已被使用")
	}

	return nil
}

// CleanupExpired 清理过期的nonce
func (r *PostgreSQLNonceRepository) CleanupExpired() (int64, error) {
	query := `
		DELETE FROM web3_wallet_nonces
		WHERE expires_at < NOW() - INTERVAL '1 hour' AND used = TRUE
	`

	result, err := r.db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("清理过期nonce失败: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

// GetActiveNonceCount 获取地址的活跃nonce数量（用于速率限制）
func (r *PostgreSQLNonceRepository) GetActiveNonceCount(address string) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM web3_wallet_nonces
		WHERE address = $1 AND expires_at > NOW() AND used = FALSE
	`

	var count int64
	err := r.db.QueryRow(query, address).Scan(&count)
	return count, err
}
