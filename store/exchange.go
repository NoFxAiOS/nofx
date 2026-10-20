package store

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/crypto"
	"nofx/logger"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExchangeStore exchange storage
type ExchangeStore struct {
	db *gorm.DB
}

// Exchange exchange configuration
type Exchange struct {
	ID                      string                 `gorm:"primaryKey" json:"id"`
	ExchangeType            string                 `gorm:"column:exchange_type;not null;default:''" json:"exchange_type"`
	AccountName             string                 `gorm:"column:account_name;not null;default:''" json:"account_name"`
	UserID                  string                 `gorm:"column:user_id;not null;default:default;index" json:"user_id"`
	Name                    string                 `gorm:"not null" json:"name"`
	Type                    string                 `gorm:"not null" json:"type"` // "cex" or "dex"
	Enabled                 bool                   `gorm:"default:false" json:"enabled"`
	APIKey                  crypto.EncryptedString `gorm:"column:api_key;default:''" json:"apiKey"`
	SecretKey               crypto.EncryptedString `gorm:"column:secret_key;default:''" json:"secretKey"`
	Passphrase              crypto.EncryptedString `gorm:"column:passphrase;default:''" json:"passphrase"`
	Testnet                 bool                   `gorm:"default:false" json:"testnet"`
	HyperliquidWalletAddr   string                 `gorm:"column:hyperliquid_wallet_addr;default:''" json:"hyperliquidWalletAddr"`
	HyperliquidUnifiedAcct  bool                   `gorm:"column:hyperliquid_unified_account;default:true" json:"hyperliquidUnifiedAccount"` // Unified Account mode (Spot as collateral)
	AsterUser               string                 `gorm:"column:aster_user;default:''" json:"asterUser"`
	AsterSigner             string                 `gorm:"column:aster_signer;default:''" json:"asterSigner"`
	AsterPrivateKey         crypto.EncryptedString `gorm:"column:aster_private_key;default:''" json:"asterPrivateKey"`
	LighterWalletAddr       string                 `gorm:"column:lighter_wallet_addr;default:''" json:"lighterWalletAddr"`
	LighterPrivateKey       crypto.EncryptedString `gorm:"column:lighter_private_key;default:''" json:"lighterPrivateKey"`
	LighterAPIKeyPrivateKey crypto.EncryptedString `gorm:"column:lighter_api_key_private_key;default:''" json:"lighterAPIKeyPrivateKey"`
	LighterAPIKeyIndex      int                    `gorm:"column:lighter_api_key_index;default:0" json:"lighterAPIKeyIndex"`
	PaperConfig             string                 `gorm:"column:paper_config;type:text;not null;default:''" json:"paper_config,omitempty"`
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
}

func (Exchange) TableName() string { return "exchanges" }

// NewExchangeStore creates a new ExchangeStore
func NewExchangeStore(db *gorm.DB) *ExchangeStore {
	return &ExchangeStore{db: db}
}

func (s *ExchangeStore) initTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		if err := s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'exchanges'`).Scan(&tableExists).Error; err != nil {
			return err
		}
		if tableExists > 0 {
			if err := ensureColumns(s.db, &Exchange{}); err != nil {
				return err
			}
			if err := s.migrateToMultiAccount(); err != nil {
				return err
			}
			return s.db.Model(&Exchange{}).Where("account_name = '' OR account_name IS NULL").Update("account_name", "Default").Error
		}
	}

	if err := s.db.AutoMigrate(&Exchange{}); err != nil {
		return err
	}

	// Run migration to multi-account if needed
	if err := s.migrateToMultiAccount(); err != nil {
		logger.Warnf("Multi-account migration warning: %v", err)
	}

	// Fix empty account_name for existing records
	s.db.Model(&Exchange{}).Where("account_name = '' OR account_name IS NULL").Update("account_name", "Default")

	return nil
}

// migrateToMultiAccount migrates old schema (id=exchange_type) to new schema (id=UUID)
func (s *ExchangeStore) migrateToMultiAccount() error {
	// Check if migration is needed by looking for old-style IDs (non-UUID)
	var count int64
	err := s.db.Model(&Exchange{}).
		Where("exchange_type = '' AND id IN ?", []string{"binance", "bybit", "okx", "bitget", "hyperliquid", "aster", "lighter"}).
		Count(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	logger.Infof("🔄 Migrating %d exchange records to multi-account schema...", count)

	// Get all old records
	var records []Exchange
	err = s.db.Where("exchange_type = '' AND id IN ?", []string{"binance", "bybit", "okx", "bitget", "hyperliquid", "aster", "lighter"}).
		Find(&records).Error
	if err != nil {
		return err
	}

	// Begin transaction
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, r := range records {
			newID := uuid.New().String()
			oldID := r.ID // This is the exchange type (e.g., "binance")

			// Update traders table to use new UUID
			if err := tx.Exec("UPDATE traders SET exchange_id = ? WHERE exchange_id = ? AND user_id = ?",
				newID, oldID, r.UserID).Error; err != nil {
				logger.Errorf("Failed to update traders for exchange %s: %v", oldID, err)
				return err
			}

			// Update the exchange record
			if err := tx.Model(&Exchange{}).
				Where("id = ? AND user_id = ?", oldID, r.UserID).
				Updates(map[string]interface{}{
					"id":            newID,
					"exchange_type": oldID,
					"account_name":  "Default",
				}).Error; err != nil {
				logger.Errorf("Failed to migrate exchange %s: %v", oldID, err)
				return err
			}

			logger.Infof("✅ Migrated exchange %s -> UUID %s for user %s", oldID, newID, r.UserID)
		}
		return nil
	})
}

func (s *ExchangeStore) initDefaultData() error {
	// No longer pre-populate exchanges - create on demand when user configures
	return nil
}

// List gets user's exchange list
func (s *ExchangeStore) List(userID string) ([]*Exchange, error) {
	var exchanges []*Exchange
	err := s.db.Where("user_id = ?", userID).Order("exchange_type, account_name").Find(&exchanges).Error
	if err != nil {
		return nil, err
	}
	return exchanges, nil
}

// GetByID gets a specific exchange by UUID
func (s *ExchangeStore) GetByID(userID, id string) (*Exchange, error) {
	var exchange Exchange
	err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&exchange).Error
	if err != nil {
		return nil, err
	}
	return &exchange, nil
}

// getExchangeNameAndType returns the display name and type for an exchange type
func getExchangeNameAndType(exchangeType string) (name string, typ string) {
	switch exchangeType {
	case "binance", "binance_stocks":
		return "Binance Futures", "cex"
	case "paper":
		return "Paper Futures (模拟盘)", "paper"
	case "bybit":
		return "Bybit Futures", "cex"
	case "okx":
		return "OKX Futures", "cex"
	case "bitget":
		return "Bitget Futures", "cex"
	case "hyperliquid":
		return "Hyperliquid", "dex"
	case "aster":
		return "Aster DEX", "dex"
	case "lighter":
		return "LIGHTER DEX", "dex"
	case "indodax":
		return "Indodax", "cex"
	default:
		return exchangeType + " Exchange", "cex"
	}
}

// Create creates a new exchange account with UUID
func (s *ExchangeStore) Create(userID, exchangeType, accountName string, enabled bool,
	apiKey, secretKey, passphrase string, testnet bool,
	hyperliquidWalletAddr string, hyperliquidUnifiedAcct bool,
	asterUser, asterSigner, asterPrivateKey,
	lighterWalletAddr, lighterPrivateKey, lighterApiKeyPrivateKey string, lighterApiKeyIndex int) (string, error) {

	id := uuid.New().String()
	name, typ := getExchangeNameAndType(exchangeType)

	if accountName == "" {
		accountName = "Default"
	}

	logger.Debugf("🔧 ExchangeStore.Create: userID=%s, exchangeType=%s, accountName=%s, id=%s",
		userID, exchangeType, accountName, id)

	exchange := &Exchange{
		ID:                      id,
		ExchangeType:            exchangeType,
		AccountName:             accountName,
		UserID:                  userID,
		Name:                    name,
		Type:                    typ,
		Enabled:                 enabled,
		APIKey:                  crypto.EncryptedString(apiKey),
		SecretKey:               crypto.EncryptedString(secretKey),
		Passphrase:              crypto.EncryptedString(passphrase),
		Testnet:                 testnet,
		HyperliquidWalletAddr:   hyperliquidWalletAddr,
		HyperliquidUnifiedAcct:  hyperliquidUnifiedAcct,
		AsterUser:               asterUser,
		AsterSigner:             asterSigner,
		AsterPrivateKey:         crypto.EncryptedString(asterPrivateKey),
		LighterWalletAddr:       lighterWalletAddr,
		LighterPrivateKey:       crypto.EncryptedString(lighterPrivateKey),
		LighterAPIKeyPrivateKey: crypto.EncryptedString(lighterApiKeyPrivateKey),
		LighterAPIKeyIndex:      lighterApiKeyIndex,
	}

	if err := s.db.Create(exchange).Error; err != nil {
		return "", err
	}
	return id, nil
}

// Update updates exchange configuration by UUID
func (s *ExchangeStore) Update(userID, id string, enabled bool, apiKey, secretKey, passphrase string, testnet bool,
	hyperliquidWalletAddr string, hyperliquidUnifiedAcct bool,
	asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterApiKeyPrivateKey string, lighterApiKeyIndex int) error {

	logger.Debugf("🔧 ExchangeStore.Update: userID=%s, id=%s, enabled=%v", userID, id, enabled)

	updates := map[string]interface{}{
		"enabled":                     enabled,
		"testnet":                     testnet,
		"hyperliquid_wallet_addr":     hyperliquidWalletAddr,
		"hyperliquid_unified_account": hyperliquidUnifiedAcct,
		"aster_user":                  asterUser,
		"aster_signer":                asterSigner,
		"lighter_wallet_addr":         lighterWalletAddr,
		"lighter_api_key_index":       lighterApiKeyIndex,
		"updated_at":                  time.Now().UTC(),
	}

	// Only update encrypted fields if not empty
	if apiKey != "" {
		updates["api_key"] = crypto.EncryptedString(apiKey)
	}
	if secretKey != "" {
		updates["secret_key"] = crypto.EncryptedString(secretKey)
	}
	if passphrase != "" {
		updates["passphrase"] = crypto.EncryptedString(passphrase)
	}
	if asterPrivateKey != "" {
		updates["aster_private_key"] = crypto.EncryptedString(asterPrivateKey)
	}
	if lighterPrivateKey != "" {
		updates["lighter_private_key"] = crypto.EncryptedString(lighterPrivateKey)
	}
	if lighterApiKeyPrivateKey != "" {
		updates["lighter_api_key_private_key"] = crypto.EncryptedString(lighterApiKeyPrivateKey)
	}

	result := s.db.Model(&Exchange{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	return nil
}

// UpdateAccountName updates the account name for an exchange
func (s *ExchangeStore) UpdateAccountName(userID, id, accountName string) error {
	result := s.db.Model(&Exchange{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"account_name": accountName,
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	return nil
}

// Delete deletes an exchange account
func (s *ExchangeStore) Delete(userID, id string) error {
	result := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&Exchange{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
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
	if id == "binance" || id == "bybit" || id == "okx" || id == "bitget" || id == "hyperliquid" || id == "aster" || id == "lighter" {
		_, err := s.Create(userID, id, "Default", enabled, apiKey, secretKey, "", testnet,
			hyperliquidWalletAddr, true, // Default to Unified Account mode
			asterUser, asterSigner, asterPrivateKey, "", "", "", 0)
		return err
	}

	// Otherwise assume it's already a UUID
	exchange := &Exchange{
		ID:                    id,
		UserID:                userID,
		Name:                  name,
		Type:                  typ,
		Enabled:               enabled,
		APIKey:                crypto.EncryptedString(apiKey),
		SecretKey:             crypto.EncryptedString(secretKey),
		Testnet:               testnet,
		HyperliquidWalletAddr: hyperliquidWalletAddr,
		AsterUser:             asterUser,
		AsterSigner:           asterSigner,
		AsterPrivateKey:       crypto.EncryptedString(asterPrivateKey),
	}
	return s.db.Where("id = ?", id).FirstOrCreate(exchange).Error
}

// PaperConfig is the public, persistable configuration for one virtual account.
// Execution-only dependencies and the account UUID are never supplied by clients.
type PaperConfig struct {
	InitialBalance  float64 `json:"initial_balance"`
	TakerFeeBps     float64 `json:"taker_fee_bps"`
	MakerFeeBps     float64 `json:"maker_fee_bps"`
	SlippageBps     float64 `json:"slippage_bps"`
	DefaultLeverage int     `json:"default_leverage"`
}

func DefaultPaperConfig() PaperConfig {
	return PaperConfig{InitialBalance: 1000, TakerFeeBps: 5, MakerFeeBps: 2, SlippageBps: 3, DefaultLeverage: 5}
}

func (c PaperConfig) Validate() error {
	if math.IsNaN(c.InitialBalance) || math.IsInf(c.InitialBalance, 0) || c.InitialBalance <= 0 {
		return fmt.Errorf("模拟盘初始资金必须大于 0")
	}
	for _, field := range []struct {
		name  string
		value float64
	}{
		{"Taker 手续费", c.TakerFeeBps}, {"Maker 手续费", c.MakerFeeBps}, {"滑点", c.SlippageBps},
	} {
		if math.IsNaN(field.value) || math.IsInf(field.value, 0) || field.value < 0 || field.value > 100 {
			return fmt.Errorf("模拟盘%s必须在 0 到 100 bps 之间", field.name)
		}
	}
	if c.DefaultLeverage < 1 || c.DefaultLeverage > 125 {
		return fmt.Errorf("模拟盘默认杠杆必须在 1 到 125 之间")
	}
	return nil
}

func (e *Exchange) ParsePaperConfig() (PaperConfig, error) {
	cfg := DefaultPaperConfig()
	if e.PaperConfig != "" {
		if err := json.Unmarshal([]byte(e.PaperConfig), &cfg); err != nil {
			return cfg, fmt.Errorf("模拟盘配置格式无效：%w", err)
		}
	}
	return cfg, cfg.Validate()
}

// CreatePaper keeps the existing credential-based Create signature unchanged.
func (s *ExchangeStore) CreatePaper(userID, accountName string, enabled bool, cfg PaperConfig) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if accountName == "" {
		accountName = "Default"
	}
	name, typ := getExchangeNameAndType("paper")
	ex := &Exchange{ID: uuid.New().String(), UserID: userID, ExchangeType: "paper", AccountName: accountName,
		Name: name, Type: typ, Enabled: enabled, PaperConfig: string(data)}
	if err := s.db.Create(ex).Error; err != nil {
		return "", err
	}
	return ex.ID, nil
}

// UpdatePaperConfig atomically updates the paper form; it never resets a ledger.
func (s *ExchangeStore) UpdatePaperConfig(userID, id, accountName string, enabled bool, cfg PaperConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	updates := map[string]interface{}{"paper_config": string(data), "enabled": enabled, "updated_at": time.Now().UTC()}
	if accountName != "" {
		updates["account_name"] = accountName
	}
	result := s.db.Model(&Exchange{}).Where("id = ? AND user_id = ? AND exchange_type = ?", id, userID, "paper").Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	return nil
}
