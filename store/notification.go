package store

import (
	"nofx/crypto"
	"time"

	"gorm.io/gorm"
)

// NotificationConfig notification configuration for each trader
type NotificationConfig struct {
	ID             string                 `gorm:"primaryKey" json:"id"`
	UserID         string                 `gorm:"column:user_id;not null;index" json:"user_id"`
	TraderID       string                 `gorm:"column:trader_id;not null;index" json:"trader_id"`
	WxPusherToken  crypto.EncryptedString `gorm:"column:wx_pusher_token;type:text;default:''" json:"wx_pusher_token"`
	WxPusherUIDs   string                 `gorm:"column:wx_pusher_uids;type:text;default:''" json:"wx_pusher_uids"` // JSON array of UIDs
	IsEnabled      bool                   `gorm:"column:is_enabled;default:false" json:"is_enabled"`
	EnableDecision bool                   `gorm:"column:enable_decision;default:true" json:"enable_decision"`
	EnableTradeOpen bool                  `gorm:"column:enable_trade_open;default:true" json:"enable_trade_open"`
	EnableTradeClose bool                 `gorm:"column:enable_trade_close;default:true" json:"enable_trade_close"`
	CreatedAt      time.Time              `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time              `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName returns the table name for NotificationConfig
func (NotificationConfig) TableName() string {
	return "notification_configs"
}

// NotificationStore notification storage
type NotificationStore struct {
	db *gorm.DB
}

// NewNotificationStore creates a new notification store
func NewNotificationStore(db *gorm.DB) *NotificationStore {
	return &NotificationStore{db: db}
}

// InitTables initializes tables
func (s *NotificationStore) InitTables() error {
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'notification_configs'`).Scan(&tableExists)
		if tableExists > 0 {
			return nil
		}
	}
	return s.db.AutoMigrate(&NotificationConfig{})
}

// GetByTraderID gets notification config by trader ID
func (s *NotificationStore) GetByTraderID(userID, traderID string) (*NotificationConfig, error) {
	var config NotificationConfig
	err := s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &config, err
}

// CreateOrUpdate creates or updates notification config
func (s *NotificationStore) CreateOrUpdate(config *NotificationConfig) error {
	return s.db.Save(config).Error
}

// GetByUserID gets all notification configs for a user
func (s *NotificationStore) GetByUserID(userID string) ([]*NotificationConfig, error) {
	var configs []*NotificationConfig
	err := s.db.Where("user_id = ?", userID).Find(&configs).Error
	return configs, err
}

// Delete deletes a notification config
func (s *NotificationStore) Delete(userID, traderID string) error {
	return s.db.Where("user_id = ? AND trader_id = ?", userID, traderID).Delete(&NotificationConfig{}).Error
}

// GetEnabledForTrader gets enabled notification config for a trader
func (s *NotificationStore) GetEnabledForTrader(traderID string) (*NotificationConfig, error) {
	var config NotificationConfig
	err := s.db.Where("trader_id = ? AND is_enabled = ?", traderID, true).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &config, err
}
