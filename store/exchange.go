package store

import (
	"database/sql"
	"fmt"
	"nofx/logger"
	"strings"
	"time"
)

// ExchangeStore 交易所存储
type ExchangeStore struct {
	db          *sql.DB
	encryptFunc func(string) string
	decryptFunc func(string) string
}

// Exchange 交易所配置
type Exchange struct {
	ID                      string    `json:"id"`
	UserID                  string    `json:"user_id"`
	Name                    string    `json:"name"`
	Type                    string    `json:"type"`
	Enabled                 bool      `json:"enabled"`
	APIKey                  string    `json:"apiKey"`
	SecretKey               string    `json:"secretKey"`
	Passphrase              string    `json:"passphrase"` // OKX专用
	Testnet                 bool      `json:"testnet"`
	HyperliquidWalletAddr   string    `json:"hyperliquidWalletAddr"`
	AsterUser               string    `json:"asterUser"`
	AsterSigner             string    `json:"asterSigner"`
	AsterPrivateKey         string    `json:"asterPrivateKey"`
	LighterWalletAddr       string    `json:"lighterWalletAddr"`
	LighterPrivateKey       string    `json:"lighterPrivateKey"`
	LighterAPIKeyPrivateKey string    `json:"lighterAPIKeyPrivateKey"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func (s *ExchangeStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS exchanges (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			passphrase TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			hyperliquid_wallet_addr TEXT DEFAULT '',
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			lighter_wallet_addr TEXT DEFAULT '',
			lighter_private_key TEXT DEFAULT '',
			lighter_api_key_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id)
		)
	`)
	if err != nil {
		return err
	}

	// 迁移：添加 passphrase 列（如果不存在）
	s.db.Exec(`ALTER TABLE exchanges ADD COLUMN passphrase TEXT DEFAULT ''`)

	// 触发器
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
		AFTER UPDATE ON exchanges
		BEGIN
			UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id AND user_id = NEW.user_id;
		END
	`)
	return err
}

func (s *ExchangeStore) initDefaultData() error {
	exchanges := []struct {
		id, name, typ string
	}{
		{"binance", "Binance Futures", "binance"},
		{"bybit", "Bybit Futures", "bybit"},
		{"okx", "OKX Futures", "okx"},
		{"hyperliquid", "Hyperliquid", "hyperliquid"},
		{"aster", "Aster DEX", "aster"},
		{"lighter", "LIGHTER DEX", "lighter"},
	}

	for _, exchange := range exchanges {
		_, err := s.db.Exec(`
			INSERT OR IGNORE INTO exchanges (id, user_id, name, type, enabled)
			VALUES (?, 'default', ?, ?, 0)
		`, exchange.id, exchange.name, exchange.typ)
		if err != nil {
			return fmt.Errorf("初始化交易所失败: %w", err)
		}
	}
	return nil
}

func (s *ExchangeStore) encrypt(plaintext string) string {
	if s.encryptFunc != nil {
		return s.encryptFunc(plaintext)
	}
	return plaintext
}

func (s *ExchangeStore) decrypt(encrypted string) string {
	if s.decryptFunc != nil {
		return s.decryptFunc(encrypted)
	}
	return encrypted
}

// EnsureUserExchanges 确保用户有所有支持的交易所记录
func (s *ExchangeStore) EnsureUserExchanges(userID string) error {
	exchanges := []struct {
		id, name, typ string
	}{
		{"binance", "Binance Futures", "binance"},
		{"bybit", "Bybit Futures", "bybit"},
		{"okx", "OKX Futures", "okx"},
		{"hyperliquid", "Hyperliquid", "hyperliquid"},
		{"aster", "Aster DEX", "aster"},
		{"lighter", "LIGHTER DEX", "lighter"},
	}

	for _, exchange := range exchanges {
		_, err := s.db.Exec(`
			INSERT OR IGNORE INTO exchanges (id, user_id, name, type, enabled)
			VALUES (?, ?, ?, ?, 0)
		`, exchange.id, userID, exchange.name, exchange.typ)
		if err != nil {
			return fmt.Errorf("确保用户交易所失败: %w", err)
		}
	}
	return nil
}

// List 获取用户的交易所列表
func (s *ExchangeStore) List(userID string) ([]*Exchange, error) {
	// 确保用户有所有支持的交易所记录
	if err := s.EnsureUserExchanges(userID); err != nil {
		logger.Debugf("⚠️ 确保用户交易所记录失败: %v", err)
	}

	rows, err := s.db.Query(`
		SELECT id, user_id, name, type, enabled, api_key, secret_key,
		       COALESCE(passphrase, '') as passphrase, testnet,
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       COALESCE(lighter_wallet_addr, '') as lighter_wallet_addr,
		       COALESCE(lighter_private_key, '') as lighter_private_key,
		       COALESCE(lighter_api_key_private_key, '') as lighter_api_key_private_key,
		       created_at, updated_at
		FROM exchanges WHERE user_id = ? ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exchanges := make([]*Exchange, 0)
	for rows.Next() {
		var e Exchange
		var createdAt, updatedAt string
		err := rows.Scan(
			&e.ID, &e.UserID, &e.Name, &e.Type,
			&e.Enabled, &e.APIKey, &e.SecretKey, &e.Passphrase, &e.Testnet,
			&e.HyperliquidWalletAddr, &e.AsterUser, &e.AsterSigner, &e.AsterPrivateKey,
			&e.LighterWalletAddr, &e.LighterPrivateKey, &e.LighterAPIKeyPrivateKey,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		e.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		e.APIKey = s.decrypt(e.APIKey)
		e.SecretKey = s.decrypt(e.SecretKey)
		e.Passphrase = s.decrypt(e.Passphrase)
		e.AsterPrivateKey = s.decrypt(e.AsterPrivateKey)
		e.LighterPrivateKey = s.decrypt(e.LighterPrivateKey)
		e.LighterAPIKeyPrivateKey = s.decrypt(e.LighterAPIKeyPrivateKey)
		exchanges = append(exchanges, &e)
	}
	return exchanges, nil
}

// Update 更新交易所配置
func (s *ExchangeStore) Update(userID, id string, enabled bool, apiKey, secretKey, passphrase string, testnet bool,
	hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterApiKeyPrivateKey string) error {

	logger.Debugf("🔧 ExchangeStore.Update: userID=%s, id=%s, enabled=%v", userID, id, enabled)

	setClauses := []string{
		"enabled = ?",
		"testnet = ?",
		"hyperliquid_wallet_addr = ?",
		"aster_user = ?",
		"aster_signer = ?",
		"lighter_wallet_addr = ?",
		"updated_at = datetime('now')",
	}
	args := []interface{}{enabled, testnet, hyperliquidWalletAddr, asterUser, asterSigner, lighterWalletAddr}

	if apiKey != "" {
		setClauses = append(setClauses, "api_key = ?")
		args = append(args, s.encrypt(apiKey))
	}
	if secretKey != "" {
		setClauses = append(setClauses, "secret_key = ?")
		args = append(args, s.encrypt(secretKey))
	}
	if passphrase != "" {
		setClauses = append(setClauses, "passphrase = ?")
		args = append(args, s.encrypt(passphrase))
	}
	if asterPrivateKey != "" {
		setClauses = append(setClauses, "aster_private_key = ?")
		args = append(args, s.encrypt(asterPrivateKey))
	}
	if lighterPrivateKey != "" {
		setClauses = append(setClauses, "lighter_private_key = ?")
		args = append(args, s.encrypt(lighterPrivateKey))
	}
	if lighterApiKeyPrivateKey != "" {
		setClauses = append(setClauses, "lighter_api_key_private_key = ?")
		args = append(args, s.encrypt(lighterApiKeyPrivateKey))
	}

	args = append(args, id, userID)
	query := fmt.Sprintf(`UPDATE exchanges SET %s WHERE id = ? AND user_id = ?`, strings.Join(setClauses, ", "))

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// 创建新记录，type 使用交易所 ID 以便后续正确识别
		var name, typ string
		switch id {
		case "binance":
			name, typ = "Binance Futures", "binance"
		case "bybit":
			name, typ = "Bybit Futures", "bybit"
		case "okx":
			name, typ = "OKX Futures", "okx"
		case "hyperliquid":
			name, typ = "Hyperliquid", "hyperliquid"
		case "aster":
			name, typ = "Aster DEX", "aster"
		case "lighter":
			name, typ = "LIGHTER DEX", "lighter"
		default:
			name, typ = id+" Exchange", id
		}

		_, err = s.db.Exec(`
			INSERT INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, passphrase, testnet,
			                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
			                       lighter_wallet_addr, lighter_private_key, lighter_api_key_private_key, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, id, userID, name, typ, enabled, s.encrypt(apiKey), s.encrypt(secretKey), s.encrypt(passphrase), testnet,
			hyperliquidWalletAddr, asterUser, asterSigner, s.encrypt(asterPrivateKey),
			lighterWalletAddr, s.encrypt(lighterPrivateKey), s.encrypt(lighterApiKeyPrivateKey))
		return err
	}
	return nil
}

// Create 创建交易所配置
func (s *ExchangeStore) Create(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool,
	hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO exchanges (id, user_id, name, type, enabled, api_key, secret_key, testnet,
		                                 hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
		                                 lighter_wallet_addr, lighter_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '')
	`, id, userID, name, typ, enabled, s.encrypt(apiKey), s.encrypt(secretKey), testnet,
		hyperliquidWalletAddr, asterUser, asterSigner, s.encrypt(asterPrivateKey))
	return err
}
