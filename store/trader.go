package store

import (
	"database/sql"
	"strings"
	"time"
)

// TraderStore trader storage
type TraderStore struct {
	db          *sql.DB
	decryptFunc func(string) string
}

// Trader trader configuration
type Trader struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	Name                string    `json:"name"`
	AIModelID           string    `json:"ai_model_id"`
	ExchangeID          string    `json:"exchange_id"`
	StrategyID          string    `json:"strategy_id"`           // Associated strategy ID
	InitialBalance      float64   `json:"initial_balance"`
	ScanIntervalMinutes int       `json:"scan_interval_minutes"`
	IsRunning           bool      `json:"is_running"`
	IsCrossMargin       bool      `json:"is_cross_margin"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`

	// Following fields are deprecated, kept for backward compatibility, new traders should use StrategyID
	BTCETHLeverage       int    `json:"btc_eth_leverage,omitempty"`
	AltcoinLeverage      int    `json:"altcoin_leverage,omitempty"`
	TradingSymbols       string `json:"trading_symbols,omitempty"`
	UseCoinPool          bool   `json:"use_coin_pool,omitempty"`
	UseOITop             bool   `json:"use_oi_top,omitempty"`
	CustomPrompt         string `json:"custom_prompt,omitempty"`
	OverrideBasePrompt   bool   `json:"override_base_prompt,omitempty"`
	SystemPromptTemplate string `json:"system_prompt_template,omitempty"`
}

// TraderFullConfig trader full configuration (includes AI model, exchange and strategy)
type TraderFullConfig struct {
	Trader   *Trader
	AIModel  *AIModel
	Exchange *Exchange
	Strategy *Strategy // Associated strategy configuration
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
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Trigger
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

	// Backward compatibility
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

	// Migration: Remove FOREIGN KEY constraint from existing traders table
	// SQLite doesn't support ALTER TABLE DROP CONSTRAINT, so we need to recreate the table
	if err := s.migrateTradersRemoveFK(); err != nil {
		// Log but don't fail - this is a best-effort migration
		// The constraint may not exist in older databases
	}

	return nil
}

// migrateTradersRemoveFK removes FOREIGN KEY constraint from traders table if it exists
func (s *TraderStore) migrateTradersRemoveFK() error {
	// Check if the table has a foreign key constraint by examining the schema
	var sql string
	err := s.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='traders'`).Scan(&sql)
	if err != nil {
		return err
	}

	// If no FOREIGN KEY in schema, no migration needed
	if !strings.Contains(sql, "FOREIGN KEY") {
		return nil
	}

	// Recreate table without FOREIGN KEY constraint
	_, err = s.db.Exec(`
		-- Create new table without FOREIGN KEY
		CREATE TABLE IF NOT EXISTS traders_new (
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
			strategy_id TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- Copy data from old table
		INSERT OR IGNORE INTO traders_new
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance,
		       scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage,
		       trading_symbols, use_coin_pool, use_oi_top, custom_prompt,
		       override_base_prompt, system_prompt_template, is_cross_margin,
		       COALESCE(strategy_id, ''), created_at, updated_at
		FROM traders;

		-- Drop old table
		DROP TABLE traders;

		-- Rename new table
		ALTER TABLE traders_new RENAME TO traders;
	`)

	if err != nil {
		return err
	}

	// Recreate trigger
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_traders_updated_at
		AFTER UPDATE ON traders
		BEGIN
			UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)

	return err
}

func (s *TraderStore) decrypt(encrypted string) string {
	if s.decryptFunc != nil {
		return s.decryptFunc(encrypted)
	}
	return encrypted
}

// Create creates trader
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

// List gets user's trader list
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

// UpdateStatus updates trader running status
func (s *TraderStore) UpdateStatus(userID, id string, isRunning bool) error {
	_, err := s.db.Exec(`UPDATE traders SET is_running = ? WHERE id = ? AND user_id = ?`, isRunning, id, userID)
	return err
}

// Update updates trader configuration
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

// UpdateInitialBalance updates initial balance
func (s *TraderStore) UpdateInitialBalance(userID, id string, newBalance float64) error {
	_, err := s.db.Exec(`UPDATE traders SET initial_balance = ? WHERE id = ? AND user_id = ?`, newBalance, id, userID)
	return err
}

// UpdateCustomPrompt updates custom prompt
func (s *TraderStore) UpdateCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
	_, err := s.db.Exec(`UPDATE traders SET custom_prompt = ?, override_base_prompt = ? WHERE id = ? AND user_id = ?`,
		customPrompt, overrideBase, id, userID)
	return err
}

// Delete deletes trader and associated data
func (s *TraderStore) Delete(userID, id string) error {
	// Delete associated equity snapshots first
	_, _ = s.db.Exec(`DELETE FROM trader_equity_snapshots WHERE trader_id = ?`, id)

	// Delete the trader
	_, err := s.db.Exec(`DELETE FROM traders WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// GetFullConfig gets trader full configuration
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
			e.id, COALESCE(e.exchange_type, '') as exchange_type, COALESCE(e.account_name, '') as account_name,
			e.user_id, e.name, e.type, e.enabled, e.api_key, e.secret_key, COALESCE(e.passphrase, ''), e.testnet,
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
		&exchange.ID, &exchange.ExchangeType, &exchange.AccountName,
		&exchange.UserID, &exchange.Name, &exchange.Type, &exchange.Enabled,
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

	// Decrypt
	aiModel.APIKey = s.decrypt(aiModel.APIKey)
	exchange.APIKey = s.decrypt(exchange.APIKey)
	exchange.SecretKey = s.decrypt(exchange.SecretKey)
	exchange.Passphrase = s.decrypt(exchange.Passphrase)
	exchange.AsterPrivateKey = s.decrypt(exchange.AsterPrivateKey)
	exchange.LighterPrivateKey = s.decrypt(exchange.LighterPrivateKey)
	exchange.LighterAPIKeyPrivateKey = s.decrypt(exchange.LighterAPIKeyPrivateKey)

	// Load associated strategy
	var strategy *Strategy
	if trader.StrategyID != "" {
		strategy, _ = s.getStrategyByID(userID, trader.StrategyID)
	}
	// If no associated strategy, get user's active strategy or default strategy
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

// getStrategyByID internal method: gets strategy by ID
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

// getActiveOrDefaultStrategy internal method: gets user's active strategy or system default strategy
func (s *TraderStore) getActiveOrDefaultStrategy(userID string) (*Strategy, error) {
	var strategy Strategy
	var createdAt, updatedAt string

	// First try to get user's active strategy
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

	// Fallback to system default strategy
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

// ListAll gets all users' trader list
// GetByID gets a trader by ID without requiring userID (for public APIs)
func (s *TraderStore) GetByID(traderID string) (*Trader, error) {
	var t Trader
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, ai_model_id, exchange_id, COALESCE(strategy_id, ''),
		       initial_balance, scan_interval_minutes, is_running, COALESCE(is_cross_margin, 1),
		       COALESCE(btc_eth_leverage, 5), COALESCE(altcoin_leverage, 5), COALESCE(trading_symbols, ''),
		       COALESCE(use_coin_pool, 0), COALESCE(use_oi_top, 0), COALESCE(custom_prompt, ''),
		       COALESCE(override_base_prompt, 0), COALESCE(system_prompt_template, 'default'),
		       created_at, updated_at
		FROM traders WHERE id = ?
	`, traderID).Scan(
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
	return &t, nil
}

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
