// Package store provides unified database storage layer
// All database operations should go through this package
package store

import (
	"database/sql"
	"fmt"
	"nofx/logger"
	"sync"
)

// Store unified data storage interface
type Store struct {
	db     *sql.DB
	driver *DBDriver // Database driver for abstraction

	// Sub-stores (lazy initialization)
	user     *UserStore
	aiModel  *AIModelStore
	exchange *ExchangeStore
	trader   *TraderStore
	decision *DecisionStore
	backtest *BacktestStore
	position *PositionStore
	strategy *StrategyStore
	equity   *EquityStore
	order    *OrderStore

	// Encryption functions
	encryptFunc func(string) string
	decryptFunc func(string) string

	mu sync.RWMutex
}

// New creates new Store instance (SQLite mode for backward compatibility)
func New(dbPath string) (*Store, error) {
	driver, err := NewDBDriver(DBConfig{Type: DBTypeSQLite, Path: dbPath})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &Store{db: driver.DB(), driver: driver}

	// Initialize all table structures
	if err := s.initTables(); err != nil {
		driver.Close()
		return nil, fmt.Errorf("failed to initialize table structure: %w", err)
	}

	// Initialize default data
	if err := s.initDefaultData(); err != nil {
		driver.Close()
		return nil, fmt.Errorf("failed to initialize default data: %w", err)
	}

	logger.Infof("✅ Database initialized (type: %s)", driver.Type)
	return s, nil
}

// NewFromEnv creates new Store instance from environment variables
// DB_TYPE: sqlite (default) or postgres
// For SQLite: DB_PATH (default: data/data.db)
// For PostgreSQL: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
func NewFromEnv() (*Store, error) {
	driver, err := NewDBDriverFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &Store{db: driver.DB(), driver: driver}

	// Initialize all table structures
	if err := s.initTables(); err != nil {
		driver.Close()
		return nil, fmt.Errorf("failed to initialize table structure: %w", err)
	}

	// Initialize default data
	if err := s.initDefaultData(); err != nil {
		driver.Close()
		return nil, fmt.Errorf("failed to initialize default data: %w", err)
	}

	logger.Infof("✅ Database initialized (type: %s)", driver.Type)
	return s, nil
}

// NewFromDB creates Store from existing database connection
func NewFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

// SetCryptoFuncs sets encryption/decryption functions
func (s *Store) SetCryptoFuncs(encrypt, decrypt func(string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.encryptFunc = encrypt
	s.decryptFunc = decrypt

	// Update already initialized sub-stores
	if s.aiModel != nil {
		s.aiModel.encryptFunc = encrypt
		s.aiModel.decryptFunc = decrypt
	}
	if s.exchange != nil {
		s.exchange.encryptFunc = encrypt
		s.exchange.decryptFunc = decrypt
	}
	if s.trader != nil {
		s.trader.decryptFunc = decrypt
	}
}

// initTables initializes all database tables
func (s *Store) initTables() error {
	// Initialize system config table first
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("failed to create system_config table: %w", err)
	}

	// Initialize in dependency order
	if err := s.User().initTables(); err != nil {
		return fmt.Errorf("failed to initialize user tables: %w", err)
	}
	if err := s.AIModel().initTables(); err != nil {
		return fmt.Errorf("failed to initialize AI model tables: %w", err)
	}
	if err := s.Exchange().initTables(); err != nil {
		return fmt.Errorf("failed to initialize exchange tables: %w", err)
	}
	if err := s.Trader().initTables(); err != nil {
		return fmt.Errorf("failed to initialize trader tables: %w", err)
	}
	if err := s.Decision().initTables(); err != nil {
		return fmt.Errorf("failed to initialize decision log tables: %w", err)
	}
	if err := s.Backtest().initTables(); err != nil {
		return fmt.Errorf("failed to initialize backtest tables: %w", err)
	}
	if err := s.Position().InitTables(); err != nil {
		return fmt.Errorf("failed to initialize position tables: %w", err)
	}
	if err := s.Strategy().initTables(); err != nil {
		return fmt.Errorf("failed to initialize strategy tables: %w", err)
	}
	if err := s.Equity().initTables(); err != nil {
		return fmt.Errorf("failed to initialize equity tables: %w", err)
	}
	if err := s.Order().InitTables(); err != nil {
		return fmt.Errorf("failed to initialize order tables: %w", err)
	}
	return nil
}

// initDefaultData initializes default data
func (s *Store) initDefaultData() error {
	if err := s.AIModel().initDefaultData(); err != nil {
		return err
	}
	if err := s.Exchange().initDefaultData(); err != nil {
		return err
	}
	if err := s.Strategy().initDefaultData(); err != nil {
		return err
	}
	// Migrate old decision_account_snapshots data to new trader_equity_snapshots table
	if migrated, err := s.Equity().MigrateFromDecision(); err != nil {
		logger.Warnf("failed to migrate equity data: %v", err)
	} else if migrated > 0 {
		logger.Infof("✅ Migrated %d equity records to new table", migrated)
	}
	return nil
}

// User gets user storage
func (s *Store) User() *UserStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.user == nil {
		s.user = &UserStore{db: s.db}
	}
	return s.user
}

// AIModel gets AI model storage
func (s *Store) AIModel() *AIModelStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aiModel == nil {
		s.aiModel = &AIModelStore{
			db:          s.db,
			encryptFunc: s.encryptFunc,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.aiModel
}

// Exchange gets exchange storage
func (s *Store) Exchange() *ExchangeStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exchange == nil {
		s.exchange = &ExchangeStore{
			db:          s.db,
			encryptFunc: s.encryptFunc,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.exchange
}

// Trader gets trader storage
func (s *Store) Trader() *TraderStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trader == nil {
		s.trader = &TraderStore{
			db:          s.db,
			decryptFunc: s.decryptFunc,
		}
	}
	return s.trader
}

// Decision gets decision log storage
func (s *Store) Decision() *DecisionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.decision == nil {
		s.decision = &DecisionStore{db: s.db}
	}
	return s.decision
}

// Backtest gets backtest data storage
func (s *Store) Backtest() *BacktestStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backtest == nil {
		s.backtest = &BacktestStore{db: s.db}
	}
	return s.backtest
}

// Position gets position storage
func (s *Store) Position() *PositionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.position == nil {
		s.position = NewPositionStore(s.db)
	}
	return s.position
}

// Strategy gets strategy storage
func (s *Store) Strategy() *StrategyStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.strategy == nil {
		s.strategy = &StrategyStore{db: s.db}
	}
	return s.strategy
}

// Equity gets equity storage
func (s *Store) Equity() *EquityStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.equity == nil {
		s.equity = &EquityStore{db: s.db}
	}
	return s.equity
}

// Order gets order storage
func (s *Store) Order() *OrderStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.order == nil {
		s.order = NewOrderStore(s.db)
	}
	return s.order
}

// Close closes database connection
func (s *Store) Close() error {
	if s.driver != nil {
		return s.driver.Close()
	}
	return s.db.Close()
}

// Driver returns database driver for abstraction
func (s *Store) Driver() *DBDriver {
	return s.driver
}

// DBType returns current database type
func (s *Store) DBType() DBType {
	if s.driver != nil {
		return s.driver.Type
	}
	return DBTypeSQLite
}

// DB gets underlying database connection (for legacy code compatibility, gradually deprecated)
// Deprecated: use Store methods instead
func (s *Store) DB() *sql.DB {
	return s.db
}

// GetSystemConfig gets a system configuration value by key
func (s *Store) GetSystemConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSystemConfig sets a system configuration value
func (s *Store) SetSystemConfig(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO system_config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

// Transaction executes transaction
func (s *Store) Transaction(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
