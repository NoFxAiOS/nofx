package store

import (
	"database/sql"
	"encoding/json"
	"nofx/logger"
	"nofx/market"
	"slices"
	"strings"
	"time"
)

// TraderStore 交易员存储
type TraderStore struct {
	db          *sql.DB
	decryptFunc func(string) string
}

// Trader 交易员配置
type Trader struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	Name                string    `json:"name"`
	AIModelID           string    `json:"ai_model_id"`
	ExchangeID          string    `json:"exchange_id"`
	StrategyID          string    `json:"strategy_id"`           // 关联策略ID
	InitialBalance      float64   `json:"initial_balance"`
	ScanIntervalMinutes int       `json:"scan_interval_minutes"`
	IsRunning           bool      `json:"is_running"`
	IsCrossMargin       bool      `json:"is_cross_margin"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`

	// 以下字段已废弃，保留用于向后兼容，新交易员应使用 StrategyID
	BTCETHLeverage       int    `json:"btc_eth_leverage,omitempty"`
	AltcoinLeverage      int    `json:"altcoin_leverage,omitempty"`
	TradingSymbols       string `json:"trading_symbols,omitempty"`
	UseCoinPool          bool   `json:"use_coin_pool,omitempty"`
	UseOITop             bool   `json:"use_oi_top,omitempty"`
	CustomPrompt         string `json:"custom_prompt,omitempty"`
	OverrideBasePrompt   bool   `json:"override_base_prompt,omitempty"`
	SystemPromptTemplate string `json:"system_prompt_template,omitempty"`
}

// TraderFullConfig 交易员完整配置（包含AI模型、交易所和策略）
type TraderFullConfig struct {
	Trader   *Trader
	AIModel  *AIModel
	Exchange *Exchange
	Strategy *Strategy // 关联的策略配置
}

func (s *TraderStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			custom_prompt TEXT DEFAULT '',
			override_base_prompt BOOLEAN DEFAULT 0,
			system_prompt_template TEXT DEFAULT 'default',
			is_cross_margin BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// 触发器
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_traders_updated_at
		AFTER UPDATE ON traders
		BEGIN
			UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return err
	}

	// 向后兼容
	alterQueries := []string{
		`ALTER TABLE traders ADD COLUMN custom_prompt TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN override_base_prompt BOOLEAN DEFAULT 0`,
		`ALTER TABLE traders ADD COLUMN is_cross_margin BOOLEAN DEFAULT 1`,
		`ALTER TABLE traders ADD COLUMN btc_eth_leverage INTEGER DEFAULT 5`,
		`ALTER TABLE traders ADD COLUMN altcoin_leverage INTEGER DEFAULT 5`,
		`ALTER TABLE traders ADD COLUMN trading_symbols TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN use_coin_pool BOOLEAN DEFAULT 0`,
		`ALTER TABLE traders ADD COLUMN use_oi_top BOOLEAN DEFAULT 0`,
		`ALTER TABLE traders ADD COLUMN system_prompt_template TEXT DEFAULT 'default'`,
		`ALTER TABLE traders ADD COLUMN strategy_id TEXT DEFAULT ''`,
	}
	for _, q := range alterQueries {
		s.db.Exec(q)
	}

	return nil
}

func (s *TraderStore) decrypt(encrypted string) string {
	if s.decryptFunc != nil {
		return s.decryptFunc(encrypted)
	}
	return encrypted
}

// Create 创建交易员
func (s *TraderStore) Create(trader *Trader) error {
	_, err := s.db.Exec(`
		INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, strategy_id, initial_balance,
		                     scan_interval_minutes, is_running, is_cross_margin,
		                     btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool,
		                     use_oi_top, custom_prompt, override_base_prompt, system_prompt_template)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, trader.ID, trader.UserID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.StrategyID,
		trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.IsCrossMargin,
		trader.BTCETHLeverage, trader.AltcoinLeverage, trader.TradingSymbols, trader.UseCoinPool,
		trader.UseOITop, trader.CustomPrompt, trader.OverrideBasePrompt, trader.SystemPromptTemplate)
	return err
}

// List 获取用户的交易员列表
func (s *TraderStore) List(userID string) ([]*Trader, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, COALESCE(strategy_id, ''),
		       initial_balance, scan_interval_minutes, is_running, COALESCE(is_cross_margin, 1),
		       COALESCE(btc_eth_leverage, 5), COALESCE(altcoin_leverage, 5), COALESCE(trading_symbols, ''),
		       COALESCE(use_coin_pool, 0), COALESCE(use_oi_top, 0), COALESCE(custom_prompt, ''),
		       COALESCE(override_base_prompt, 0), COALESCE(system_prompt_template, 'default'),
		       created_at, updated_at
		FROM traders WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*Trader
	for rows.Next() {
		var t Trader
		var createdAt, updatedAt string
		err := rows.Scan(
			&t.ID, &t.UserID, &t.Name, &t.AIModelID, &t.ExchangeID, &t.StrategyID,
			&t.InitialBalance, &t.ScanIntervalMinutes, &t.IsRunning, &t.IsCrossMargin,
			&t.BTCETHLeverage, &t.AltcoinLeverage, &t.TradingSymbols,
			&t.UseCoinPool, &t.UseOITop, &t.CustomPrompt, &t.OverrideBasePrompt,
			&t.SystemPromptTemplate, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		t.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &t)
	}
	return traders, nil
}

// UpdateStatus 更新交易员运行状态
func (s *TraderStore) UpdateStatus(userID, id string, isRunning bool) error {
	_, err := s.db.Exec(`UPDATE traders SET is_running = ? WHERE id = ? AND user_id = ?`, isRunning, id, userID)
	return err
}

// Update 更新交易员配置
func (s *TraderStore) Update(trader *Trader) error {
	_, err := s.db.Exec(`
		UPDATE traders SET
			name = ?, ai_model_id = ?, exchange_id = ?, strategy_id = ?,
			scan_interval_minutes = ?, is_cross_margin = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, trader.Name, trader.AIModelID, trader.ExchangeID, trader.StrategyID,
		trader.ScanIntervalMinutes, trader.IsCrossMargin, trader.ID, trader.UserID)
	return err
}

// UpdateInitialBalance 更新初始余额
func (s *TraderStore) UpdateInitialBalance(userID, id string, newBalance float64) error {
	_, err := s.db.Exec(`UPDATE traders SET initial_balance = ? WHERE id = ? AND user_id = ?`, newBalance, id, userID)
	return err
}

// UpdateCustomPrompt 更新自定义提示词
func (s *TraderStore) UpdateCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
	_, err := s.db.Exec(`UPDATE traders SET custom_prompt = ?, override_base_prompt = ? WHERE id = ? AND user_id = ?`,
		customPrompt, overrideBase, id, userID)
	return err
}

// Delete 删除交易员
func (s *TraderStore) Delete(userID, id string) error {
	_, err := s.db.Exec(`DELETE FROM traders WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// GetFullConfig 获取交易员完整配置
func (s *TraderStore) GetFullConfig(userID, traderID string) (*TraderFullConfig, error) {
	var trader Trader
	var aiModel AIModel
	var exchange Exchange
	var traderCreatedAt, traderUpdatedAt string
	var aiModelCreatedAt, aiModelUpdatedAt string
	var exchangeCreatedAt, exchangeUpdatedAt string

	err := s.db.QueryRow(`
		SELECT
			t.id, t.user_id, t.name, t.ai_model_id, t.exchange_id, COALESCE(t.strategy_id, ''),
			t.initial_balance, t.scan_interval_minutes, t.is_running, COALESCE(t.is_cross_margin, 1),
			COALESCE(t.btc_eth_leverage, 5), COALESCE(t.altcoin_leverage, 5), COALESCE(t.trading_symbols, ''),
			COALESCE(t.use_coin_pool, 0), COALESCE(t.use_oi_top, 0), COALESCE(t.custom_prompt, ''),
			COALESCE(t.override_base_prompt, 0), COALESCE(t.system_prompt_template, 'default'),
			t.created_at, t.updated_at,
			a.id, a.user_id, a.name, a.provider, a.enabled, a.api_key,
			COALESCE(a.custom_api_url, ''), COALESCE(a.custom_model_name, ''), a.created_at, a.updated_at,
			e.id, e.user_id, e.name, e.type, e.enabled, e.api_key, e.secret_key, COALESCE(e.passphrase, ''), e.testnet,
			COALESCE(e.hyperliquid_wallet_addr, ''), COALESCE(e.aster_user, ''), COALESCE(e.aster_signer, ''),
			COALESCE(e.aster_private_key, ''), COALESCE(e.lighter_wallet_addr, ''), COALESCE(e.lighter_private_key, ''),
			COALESCE(e.lighter_api_key_private_key, ''), e.created_at, e.updated_at
		FROM traders t
		JOIN ai_models a ON t.ai_model_id = a.id AND t.user_id = a.user_id
		JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id
		WHERE t.id = ? AND t.user_id = ?
	`, traderID, userID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID, &trader.StrategyID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning, &trader.IsCrossMargin,
		&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
		&trader.UseCoinPool, &trader.UseOITop, &trader.CustomPrompt, &trader.OverrideBasePrompt,
		&trader.SystemPromptTemplate, &traderCreatedAt, &traderUpdatedAt,
		&aiModel.ID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
		&aiModel.CustomAPIURL, &aiModel.CustomModelName, &aiModelCreatedAt, &aiModelUpdatedAt,
		&exchange.ID, &exchange.UserID, &exchange.Name, &exchange.Type, &exchange.Enabled,
		&exchange.APIKey, &exchange.SecretKey, &exchange.Passphrase, &exchange.Testnet, &exchange.HyperliquidWalletAddr,
		&exchange.AsterUser, &exchange.AsterSigner, &exchange.AsterPrivateKey,
		&exchange.LighterWalletAddr, &exchange.LighterPrivateKey, &exchange.LighterAPIKeyPrivateKey,
		&exchangeCreatedAt, &exchangeUpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	trader.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", traderCreatedAt)
	trader.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", traderUpdatedAt)
	aiModel.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", aiModelCreatedAt)
	aiModel.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", aiModelUpdatedAt)
	exchange.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", exchangeCreatedAt)
	exchange.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", exchangeUpdatedAt)

	// 解密
	aiModel.APIKey = s.decrypt(aiModel.APIKey)
	exchange.APIKey = s.decrypt(exchange.APIKey)
	exchange.SecretKey = s.decrypt(exchange.SecretKey)
	exchange.Passphrase = s.decrypt(exchange.Passphrase)
	exchange.AsterPrivateKey = s.decrypt(exchange.AsterPrivateKey)
	exchange.LighterPrivateKey = s.decrypt(exchange.LighterPrivateKey)
	exchange.LighterAPIKeyPrivateKey = s.decrypt(exchange.LighterAPIKeyPrivateKey)

	// 加载关联的策略
	var strategy *Strategy
	if trader.StrategyID != "" {
		strategy, _ = s.getStrategyByID(userID, trader.StrategyID)
	}
	// 如果没有关联策略，获取用户的激活策略或默认策略
	if strategy == nil {
		strategy, _ = s.getActiveOrDefaultStrategy(userID)
	}

	return &TraderFullConfig{
		Trader:   &trader,
		AIModel:  &aiModel,
		Exchange: &exchange,
		Strategy: strategy,
	}, nil
}

// getStrategyByID 内部方法：根据ID获取策略
func (s *TraderStore) getStrategyByID(userID, strategyID string) (*Strategy, error) {
	var strategy Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies WHERE id = ? AND (user_id = ? OR is_default = 1)
	`, strategyID, userID).Scan(
		&strategy.ID, &strategy.UserID, &strategy.Name, &strategy.Description,
		&strategy.IsActive, &strategy.IsDefault, &strategy.Config, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	strategy.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	strategy.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &strategy, nil
}

// getActiveOrDefaultStrategy 内部方法：获取用户激活的策略或系统默认策略
func (s *TraderStore) getActiveOrDefaultStrategy(userID string) (*Strategy, error) {
	var strategy Strategy
	var createdAt, updatedAt string

	// 先尝试获取用户激活的策略
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies WHERE user_id = ? AND is_active = 1
	`, userID).Scan(
		&strategy.ID, &strategy.UserID, &strategy.Name, &strategy.Description,
		&strategy.IsActive, &strategy.IsDefault, &strategy.Config, &createdAt, &updatedAt,
	)
	if err == nil {
		strategy.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		strategy.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		return &strategy, nil
	}

	// 回退到系统默认策略
	err = s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies WHERE is_default = 1 LIMIT 1
	`).Scan(
		&strategy.ID, &strategy.UserID, &strategy.Name, &strategy.Description,
		&strategy.IsActive, &strategy.IsDefault, &strategy.Config, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	strategy.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	strategy.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &strategy, nil
}

// GetCustomCoins 获取所有交易员自定义币种
func (s *TraderStore) GetCustomCoins() []string {
	var symbol string
	var symbols []string
	_ = s.db.QueryRow(`
		SELECT GROUP_CONCAT(trading_symbols, ',') as symbol
		FROM traders WHERE trading_symbols != ''
	`).Scan(&symbol)

	// 如果没有自定义币种，返回默认币种
	if symbol == "" {
		var symbolJSON string
		_ = s.db.QueryRow(`SELECT value FROM system_config WHERE key = 'default_coins'`).Scan(&symbolJSON)
		if symbolJSON != "" {
			if err := json.Unmarshal([]byte(symbolJSON), &symbols); err != nil {
				logger.Warnf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
				symbols = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
			}
		} else {
			symbols = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
		}
		return symbols
	}

	// 处理并去重币种列表
	for _, s := range strings.Split(symbol, ",") {
		if s == "" {
			continue
		}
		coin := market.Normalize(s)
		if !slices.Contains(symbols, coin) {
			symbols = append(symbols, coin)
		}
	}
	return symbols
}

// ListAll 获取所有用户的交易员列表
func (s *TraderStore) ListAll() ([]*Trader, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, COALESCE(strategy_id, ''),
		       initial_balance, scan_interval_minutes, is_running, COALESCE(is_cross_margin, 1),
		       COALESCE(btc_eth_leverage, 5), COALESCE(altcoin_leverage, 5), COALESCE(trading_symbols, ''),
		       COALESCE(use_coin_pool, 0), COALESCE(use_oi_top, 0), COALESCE(custom_prompt, ''),
		       COALESCE(override_base_prompt, 0), COALESCE(system_prompt_template, 'default'),
		       created_at, updated_at
		FROM traders ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*Trader
	for rows.Next() {
		var t Trader
		var createdAt, updatedAt string
		err := rows.Scan(
			&t.ID, &t.UserID, &t.Name, &t.AIModelID, &t.ExchangeID, &t.StrategyID,
			&t.InitialBalance, &t.ScanIntervalMinutes, &t.IsRunning, &t.IsCrossMargin,
			&t.BTCETHLeverage, &t.AltcoinLeverage, &t.TradingSymbols,
			&t.UseCoinPool, &t.UseOITop, &t.CustomPrompt, &t.OverrideBasePrompt,
			&t.SystemPromptTemplate, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		t.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		traders = append(traders, &t)
	}
	return traders, nil
}
