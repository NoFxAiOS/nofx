package store

import (
	"database/sql"
)

// SystemConfigStore 系统配置存储
type SystemConfigStore struct {
	db *sql.DB
}

func (s *SystemConfigStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 触发器
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_system_config_updated_at
		AFTER UPDATE ON system_config
		BEGIN
			UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
		END
	`)
	return err
}

func (s *SystemConfigStore) initDefaultData() error {
	configs := map[string]string{
		"beta_mode":            "false",
		"api_server_port":      "8080",
		"use_default_coins":    "true",
		"default_coins":        `["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]`,
		"max_daily_loss":       "10.0",
		"max_drawdown":         "20.0",
		"stop_trading_minutes": "60",
		"btc_eth_leverage":     "5",
		"altcoin_leverage":     "5",
		"jwt_secret":           "",
		"registration_enabled": "true",
	}

	for key, value := range configs {
		_, err := s.db.Exec(`INSERT OR IGNORE INTO system_config (key, value) VALUES (?, ?)`, key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

// Get 获取配置值
func (s *SystemConfigStore) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	return value, err
}

// Set 设置配置值
func (s *SystemConfigStore) Set(key, value string) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (key, value) VALUES (?, ?)`, key, value)
	return err
}
