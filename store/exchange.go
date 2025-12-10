package store

import (
	"database/sql"
	"fmt"
	"nofx/logger"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ExchangeStore exchange storage
type ExchangeStore struct {
	db          *sql.DB
	encryptFunc func(string) string
	decryptFunc func(string) string
}

// Exchange exchange configuration
type Exchange struct {
	ID                      string    `json:"id"`            // UUID
	ExchangeType            string    `json:"exchange_type"` // "binance", "bybit", "okx", "hyperliquid", "aster", "lighter"
	AccountName             string    `json:"account_name"`  // User-defined account name
	UserID                  string    `json:"user_id"`
	Name                    string    `json:"name"` // Display name (auto-generated or user-defined)
	Type                    string    `json:"type"` // "cex" or "dex"
	Enabled                 bool      `json:"enabled"`
	APIKey                  string    `json:"apiKey"`
	SecretKey               string    `json:"secretKey"`
	Passphrase              string    `json:"passphrase"` // OKX-specific
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
	// Create new table structure with UUID as primary key
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS exchanges (
			id TEXT PRIMARY KEY,
			exchange_type TEXT NOT NULL DEFAULT '',
			account_name TEXT NOT NULL DEFAULT '',
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
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Migration: add new columns if not exists
	s.db.Exec(`ALTER TABLE exchanges ADD COLUMN passphrase TEXT DEFAULT ''`)
	s.db.Exec(`ALTER TABLE exchanges ADD COLUMN exchange_type TEXT NOT NULL DEFAULT ''`)
	s.db.Exec(`ALTER TABLE exchanges ADD COLUMN account_name TEXT NOT NULL DEFAULT ''`)

	// Run migration to multi-account if needed
	if err := s.migrateToMultiAccount(); err != nil {
		logger.Warnf("Multi-account migration warning: %v", err)
	}

	// Fix empty account_name for existing records
	s.db.Exec(`UPDATE exchanges SET account_name = 'Default' WHERE account_name = '' OR account_name IS NULL`)

	// Update trigger for new schema
	s.db.Exec(`DROP TRIGGER IF EXISTS update_exchanges_updated_at`)
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
		AFTER UPDATE ON exchanges
		BEGIN
			UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	return err
}

// migrateToMultiAccount migrates old schema (id=exchange_type) to new schema (id=UUID)
func (s *ExchangeStore) migrateToMultiAccount() error {
	// Check if migration is needed by looking for old-style IDs (non-UUID)
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM exchanges
		WHERE exchange_type = '' AND id IN ('binance', 'bybit', 'okx', 'hyperliquid', 'aster', 'lighter')
	`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// No migration needed
		return nil
	}

	logger.Infof("🔄 Migrating %d exchange records to multi-account schema...", count)

	// Get all old records
	rows, err := s.db.Query(`
		SELECT id, user_id, name, type, enabled, api_key, secret_key,
		       COALESCE(passphrase, '') as passphrase, testnet,
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       COALESCE(lighter_wallet_addr, '') as lighter_wallet_addr,
		       COALESCE(lighter_private_key, '') as lighter_private_key,
		       COALESCE(lighter_api_key_private_key, '') as lighter_api_key_private_key
		FROM exchanges
		WHERE exchange_type = '' AND id IN ('binance', 'bybit', 'okx', 'hyperliquid', 'aster', 'lighter')
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type oldRecord struct {
		id, userID, name, typ                                                                             string
		enabled, testnet                                                                                  bool
		apiKey, secretKey, passphrase                                                                     string
		hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey                                    string
		lighterWalletAddr, lighterPrivateKey, lighterApiKeyPrivateKey                                     string
	}

	var records []oldRecord
	for rows.Next() {
		var r oldRecord
		if err := rows.Scan(&r.id, &r.userID, &r.name, &r.typ, &r.enabled,
			&r.apiKey, &r.secretKey, &r.passphrase, &r.testnet,
			&r.hyperliquidWalletAddr, &r.asterUser, &r.asterSigner, &r.asterPrivateKey,
			&r.lighterWalletAddr, &r.lighterPrivateKey, &r.lighterApiKeyPrivateKey); err != nil {
			return err
		}
		records = append(records, r)
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Migrate each record
	for _, r := range records {
		newID := uuid.New().String()
		oldID := r.id // This is the exchange type (e.g., "binance")

		// Update traders table to use new UUID
		_, err = tx.Exec(`UPDATE traders SET exchange_id = ? WHERE exchange_id = ? AND user_id = ?`,
			newID, oldID, r.userID)
		if err != nil {
			logger.Errorf("Failed to update traders for exchange %s: %v", oldID, err)
			return err
		}

		// Update the exchange record
		_, err = tx.Exec(`
			UPDATE exchanges SET
				id = ?,
				exchange_type = ?,
				account_name = ?
			WHERE id = ? AND user_id = ?
		`, newID, oldID, "Default", oldID, r.userID)
		if err != nil {
			logger.Errorf("Failed to migrate exchange %s: %v", oldID, err)
			return err
		}

		logger.Infof("✅ Migrated exchange %s -> UUID %s for user %s", oldID, newID, r.userID)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	logger.Infof("✅ Multi-account migration completed successfully")
	return nil
}

func (s *ExchangeStore) initDefaultData() error {
	// No longer pre-populate exchanges - create on demand when user configures
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

// List gets user's exchange list
func (s *ExchangeStore) List(userID string) ([]*Exchange, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(exchange_type, '') as exchange_type, COALESCE(account_name, '') as account_name,
		       user_id, name, type, enabled, api_key, secret_key,
		       COALESCE(passphrase, '') as passphrase, testnet,
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       COALESCE(lighter_wallet_addr, '') as lighter_wallet_addr,
		       COALESCE(lighter_private_key, '') as lighter_private_key,
		       COALESCE(lighter_api_key_private_key, '') as lighter_api_key_private_key,
		       created_at, updated_at
		FROM exchanges WHERE user_id = ? ORDER BY exchange_type, account_name
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
			&e.ID, &e.ExchangeType, &e.AccountName,
			&e.UserID, &e.Name, &e.Type,
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

// GetByID gets a specific exchange by UUID
func (s *ExchangeStore) GetByID(userID, id string) (*Exchange, error) {
	var e Exchange
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, COALESCE(exchange_type, '') as exchange_type, COALESCE(account_name, '') as account_name,
		       user_id, name, type, enabled, api_key, secret_key,
		       COALESCE(passphrase, '') as passphrase, testnet,
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       COALESCE(lighter_wallet_addr, '') as lighter_wallet_addr,
		       COALESCE(lighter_private_key, '') as lighter_private_key,
		       COALESCE(lighter_api_key_private_key, '') as lighter_api_key_private_key,
		       created_at, updated_at
		FROM exchanges WHERE id = ? AND user_id = ?
	`, id, userID).Scan(
		&e.ID, &e.ExchangeType, &e.AccountName,
		&e.UserID, &e.Name, &e.Type,
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
	return &e, nil
}

// getExchangeNameAndType returns the display name and type for an exchange type
func getExchangeNameAndType(exchangeType string) (name string, typ string) {
	switch exchangeType {
	case "binance":
		return "Binance Futures", "cex"
	case "bybit":
		return "Bybit Futures", "cex"
	case "okx":
		return "OKX Futures", "cex"
	case "hyperliquid":
		return "Hyperliquid", "dex"
	case "aster":
		return "Aster DEX", "dex"
	case "lighter":
		return "LIGHTER DEX", "dex"
	default:
		return exchangeType + " Exchange", "cex"
	}
}

// Create creates a new exchange account with UUID
func (s *ExchangeStore) Create(userID, exchangeType, accountName string, enabled bool,
	apiKey, secretKey, passphrase string, testnet bool,
	hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey,
	lighterWalletAddr, lighterPrivateKey, lighterApiKeyPrivateKey string) (string, error) {

	id := uuid.New().String()
	name, typ := getExchangeNameAndType(exchangeType)

	// If account name is empty, use "Default"
	if accountName == "" {
		accountName = "Default"
	}

	logger.Debugf("🔧 ExchangeStore.Create: userID=%s, exchangeType=%s, accountName=%s, id=%s",
		userID, exchangeType, accountName, id)

	_, err := s.db.Exec(`
		INSERT INTO exchanges (id, exchange_type, account_name, user_id, name, type, enabled,
		                       api_key, secret_key, passphrase, testnet,
		                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
		                       lighter_wallet_addr, lighter_private_key, lighter_api_key_private_key,
		                       created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, exchangeType, accountName, userID, name, typ, enabled,
		s.encrypt(apiKey), s.encrypt(secretKey), s.encrypt(passphrase), testnet,
		hyperliquidWalletAddr, asterUser, asterSigner, s.encrypt(asterPrivateKey),
		lighterWalletAddr, s.encrypt(lighterPrivateKey), s.encrypt(lighterApiKeyPrivateKey))

	if err != nil {
		return "", err
	}
	return id, nil
}

// Update updates exchange configuration by UUID
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
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	return nil
}

// UpdateAccountName updates the account name for an exchange
func (s *ExchangeStore) UpdateAccountName(userID, id, accountName string) error {
	result, err := s.db.Exec(`UPDATE exchanges SET account_name = ?, updated_at = datetime('now') WHERE id = ? AND user_id = ?`,
		accountName, id, userID)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	return nil
}

// Delete deletes an exchange account
func (s *ExchangeStore) Delete(userID, id string) error {
	result, err := s.db.Exec(`DELETE FROM exchanges WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	logger.Infof("🗑️ Deleted exchange: id=%s, userID=%s", id, userID)
	return nil
}

// CreateLegacy creates exchange configuration (legacy API for backward compatibility)
// This method is deprecated, use Create instead
func (s *ExchangeStore) CreateLegacy(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool,
	hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {

	// Check if this is an old-style ID (exchange type as ID)
	if id == "binance" || id == "bybit" || id == "okx" || id == "hyperliquid" || id == "aster" || id == "lighter" {
		// Use new Create method with exchange type
		_, err := s.Create(userID, id, "Default", enabled, apiKey, secretKey, "", testnet,
			hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey, "", "", "")
		return err
	}

	// Otherwise assume it's already a UUID
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO exchanges (id, exchange_type, account_name, user_id, name, type, enabled,
		                                 api_key, secret_key, testnet,
		                                 hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
		                                 lighter_wallet_addr, lighter_private_key)
		VALUES (?, '', '', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '')
	`, id, userID, name, typ, enabled, s.encrypt(apiKey), s.encrypt(secretKey), testnet,
		hyperliquidWalletAddr, asterUser, asterSigner, s.encrypt(asterPrivateKey))
	return err
}
