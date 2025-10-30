package config

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database 配置数据库
type Database struct {
	db *sql.DB
}

// NewDatabase 创建配置数据库
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	if err := database.initDefaultData(); err != nil {
		return nil, fmt.Errorf("初始化默认数据失败: %w", err)
	}

	return database, nil
}

// createTables 创建数据库表
func (d *Database) createTables() error {
	queries := []string{
		// AI模型配置表
		`CREATE TABLE IF NOT EXISTS ai_models (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 交易所配置表
		`CREATE TABLE IF NOT EXISTS exchanges (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- 'cex' or 'dex'
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 交易员配置表
		`CREATE TABLE IF NOT EXISTS traders (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
			FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
		)`,

		// 系统配置表
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 触发器：自动更新 updated_at
		`CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
				UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_traders_updated_at
			AFTER UPDATE ON traders
			BEGIN
				UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_system_config_updated_at
			AFTER UPDATE ON system_config
			BEGIN
				UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
			END`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败 [%s]: %w", query, err)
		}
	}

	return nil
}

// initDefaultData 初始化默认数据
func (d *Database) initDefaultData() error {
	// 初始化AI模型
	aiModels := []struct {
		id, name, provider string
	}{
		{"deepseek", "DeepSeek", "deepseek"},
		{"qwen", "Qwen", "qwen"},
	}

	for _, model := range aiModels {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO ai_models (id, name, provider, enabled) 
			VALUES (?, ?, ?, 0)
		`, model.id, model.name, model.provider)
		if err != nil {
			return fmt.Errorf("初始化AI模型失败: %w", err)
		}
	}

	// 初始化交易所
	exchanges := []struct {
		id, name, typ string
	}{
		{"binance", "Binance", "cex"},
		{"hyperliquid", "Hyperliquid", "dex"},
	}

	for _, exchange := range exchanges {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO exchanges (id, name, type, enabled) 
			VALUES (?, ?, ?, 0)
		`, exchange.id, exchange.name, exchange.typ)
		if err != nil {
			return fmt.Errorf("初始化交易所失败: %w", err)
		}
	}

	// 初始化系统配置
	systemConfigs := map[string]string{
		"api_server_port":       "8081",
		"use_default_coins":     "true",
		"coin_pool_api_url":     "",
		"oi_top_api_url":        "",
		"max_daily_loss":        "10.0",
		"max_drawdown":          "20.0",
		"stop_trading_minutes":  "60",
	}

	for key, value := range systemConfigs {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO system_config (key, value) 
			VALUES (?, ?)
		`, key, value)
		if err != nil {
			return fmt.Errorf("初始化系统配置失败: %w", err)
		}
	}

	return nil
}

// AIModelConfig AI模型配置
type AIModelConfig struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	Enabled   bool      `json:"enabled"`
	APIKey    string    `json:"apiKey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExchangeConfig 交易所配置
type ExchangeConfig struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Enabled   bool      `json:"enabled"`
	APIKey    string    `json:"apiKey"`
	SecretKey string    `json:"secretKey"`
	Testnet   bool      `json:"testnet"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TraderConfig 交易员配置
type TraderConfig struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	AIModelID          string    `json:"ai_model_id"`
	ExchangeID         string    `json:"exchange_id"`
	InitialBalance     float64   `json:"initial_balance"`
	ScanIntervalMinutes int      `json:"scan_interval_minutes"`
	IsRunning          bool      `json:"is_running"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// GetAIModels 获取所有AI模型配置
func (d *Database) GetAIModels() ([]*AIModelConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, name, provider, enabled, api_key, created_at, updated_at 
		FROM ai_models ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []*AIModelConfig
	for rows.Next() {
		var model AIModelConfig
		err := rows.Scan(
			&model.ID, &model.Name, &model.Provider, 
			&model.Enabled, &model.APIKey,
			&model.CreatedAt, &model.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		models = append(models, &model)
	}

	return models, nil
}

// UpdateAIModel 更新AI模型配置
func (d *Database) UpdateAIModel(id string, enabled bool, apiKey string) error {
	_, err := d.db.Exec(`
		UPDATE ai_models SET enabled = ?, api_key = ? WHERE id = ?
	`, enabled, apiKey, id)
	return err
}

// GetExchanges 获取所有交易所配置
func (d *Database) GetExchanges() ([]*ExchangeConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, name, type, enabled, api_key, secret_key, testnet, created_at, updated_at 
		FROM exchanges ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exchanges []*ExchangeConfig
	for rows.Next() {
		var exchange ExchangeConfig
		err := rows.Scan(
			&exchange.ID, &exchange.Name, &exchange.Type,
			&exchange.Enabled, &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
			&exchange.CreatedAt, &exchange.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		exchanges = append(exchanges, &exchange)
	}

	return exchanges, nil
}

// UpdateExchange 更新交易所配置
func (d *Database) UpdateExchange(id string, enabled bool, apiKey, secretKey string, testnet bool) error {
	_, err := d.db.Exec(`
		UPDATE exchanges SET enabled = ?, api_key = ?, secret_key = ?, testnet = ? WHERE id = ?
	`, enabled, apiKey, secretKey, testnet, id)
	return err
}

// CreateTrader 创建交易员
func (d *Database) CreateTrader(trader *TraderConfig) error {
	_, err := d.db.Exec(`
		INSERT INTO traders (id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, trader.ID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning)
	return err
}

// GetTraders 获取所有交易员
func (d *Database) GetTraders() ([]*TraderConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, created_at, updated_at
		FROM traders ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderConfig
	for rows.Next() {
		var trader TraderConfig
		err := rows.Scan(
			&trader.ID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// UpdateTraderStatus 更新交易员状态
func (d *Database) UpdateTraderStatus(id string, isRunning bool) error {
	_, err := d.db.Exec(`UPDATE traders SET is_running = ? WHERE id = ?`, isRunning, id)
	return err
}

// DeleteTrader 删除交易员
func (d *Database) DeleteTrader(id string) error {
	_, err := d.db.Exec(`DELETE FROM traders WHERE id = ?`, id)
	return err
}

// GetTraderConfig 获取交易员完整配置（包含AI模型和交易所信息）
func (d *Database) GetTraderConfig(traderID string) (*TraderConfig, *AIModelConfig, *ExchangeConfig, error) {
	var trader TraderConfig
	var aiModel AIModelConfig
	var exchange ExchangeConfig

	err := d.db.QueryRow(`
		SELECT 
			t.id, t.name, t.ai_model_id, t.exchange_id, t.initial_balance, t.scan_interval_minutes, t.is_running, t.created_at, t.updated_at,
			a.id, a.name, a.provider, a.enabled, a.api_key, a.created_at, a.updated_at,
			e.id, e.name, e.type, e.enabled, e.api_key, e.secret_key, e.testnet, e.created_at, e.updated_at
		FROM traders t
		JOIN ai_models a ON t.ai_model_id = a.id
		JOIN exchanges e ON t.exchange_id = e.id
		WHERE t.id = ?
	`, traderID).Scan(
		&trader.ID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.CreatedAt, &trader.UpdatedAt,
		&aiModel.ID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
		&aiModel.CreatedAt, &aiModel.UpdatedAt,
		&exchange.ID, &exchange.Name, &exchange.Type, &exchange.Enabled,
		&exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
		&exchange.CreatedAt, &exchange.UpdatedAt,
	)

	if err != nil {
		return nil, nil, nil, err
	}

	return &trader, &aiModel, &exchange, nil
}

// GetSystemConfig 获取系统配置
func (d *Database) GetSystemConfig(key string) (string, error) {
	var value string
	err := d.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	return value, err
}

// SetSystemConfig 设置系统配置
func (d *Database) SetSystemConfig(key, value string) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO system_config (key, value) VALUES (?, ?)
	`, key, value)
	return err
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	return d.db.Close()
}