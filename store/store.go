// Package store provides unified database storage layer
// All database operations should go through this package
package store

import (
	"database/sql"
	"fmt"
	"nofx/logger"
	"sync"

	"gorm.io/gorm"
)

// Store unified data storage interface
type Store struct {
	gdb    *gorm.DB  // GORM database connection
	db     *sql.DB   // Legacy sql.DB for backward compatibility
	driver *DBDriver // Database driver for abstraction (legacy)

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
	grid     *GridStore

	mu sync.RWMutex
}

// New creates new Store instance (SQLite mode for backward compatibility)
func New(dbPath string) (*Store, error) {
	gdb, err := InitGorm(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying sql.DB for legacy compatibility
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	s := &Store{gdb: gdb, db: sqlDB}

	// Initialize all table structures
	if err := s.initTables(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize table structure: %w", err)
	}

	// Initialize default data
	if err := s.initDefaultData(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize default data: %w", err)
	}

	logger.Infof("✅ Database initialized (GORM, SQLite)")
	return s, nil
}

// NewWithConfig creates new Store instance with provided database configuration
func NewWithConfig(cfg DBConfig) (*Store, error) {
	gdb, err := InitGormWithConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying sql.DB for legacy compatibility
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	s := &Store{gdb: gdb, db: sqlDB}

	// Initialize all table structures
	if err := s.initTables(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize table structure: %w", err)
	}

	// Initialize default data
	if err := s.initDefaultData(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize default data: %w", err)
	}

	dbTypeStr := "SQLite"
	if cfg.Type == DBTypePostgres {
		dbTypeStr = "PostgreSQL"
	}
	logger.Infof("✅ Database initialized (GORM, %s)", dbTypeStr)
	return s, nil
}

// NewFromGorm creates Store from existing GORM connection
func NewFromGorm(gdb *gorm.DB) (*Store, error) {
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	return &Store{gdb: gdb, db: sqlDB}, nil
}

// NewFromDB creates Store from existing database connection (legacy)
// Deprecated: Use NewFromGorm instead
func NewFromDB(db *sql.DB) *Store {
	return &Store{db: db}
}

// initTables initializes all database tables using GORM AutoMigrate
func (s *Store) initTables() error {
	// Create system_config table (GORM handles this via raw SQL for simplicity)
	if err := s.gdb.Exec(`
		CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`).Error; err != nil {
		return fmt.Errorf("failed to create system_config table: %w", err)
	}

	// Initialize sub-store tables
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
	if err := s.Grid().InitTables(); err != nil {
		return fmt.Errorf("failed to initialize grid tables: %w", err)
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
		s.user = NewUserStore(s.gdb)
	}
	return s.user
}

// AIModel gets AI model storage
func (s *Store) AIModel() *AIModelStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aiModel == nil {
		s.aiModel = NewAIModelStore(s.gdb)
	}
	return s.aiModel
}

// Exchange gets exchange storage
func (s *Store) Exchange() *ExchangeStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exchange == nil {
		s.exchange = NewExchangeStore(s.gdb)
	}
	return s.exchange
}

// Trader gets trader storage
func (s *Store) Trader() *TraderStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trader == nil {
		s.trader = NewTraderStore(s.gdb)
	}
	return s.trader
}

// Decision gets decision log storage
func (s *Store) Decision() *DecisionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.decision == nil {
		s.decision = NewDecisionStore(s.gdb)
	}
	return s.decision
}

// Backtest gets backtest data storage
func (s *Store) Backtest() *BacktestStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backtest == nil {
		s.backtest = NewBacktestStore(s.gdb)
	}
	return s.backtest
}

// Position gets position storage
func (s *Store) Position() *PositionStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.position == nil {
		s.position = NewPositionStore(s.gdb)
	}
	return s.position
}

// Strategy gets strategy storage
func (s *Store) Strategy() *StrategyStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.strategy == nil {
		s.strategy = NewStrategyStore(s.gdb)
	}
	return s.strategy
}

// Equity gets equity storage
func (s *Store) Equity() *EquityStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.equity == nil {
		s.equity = NewEquityStore(s.gdb)
	}
	return s.equity
}

// Order gets order storage
func (s *Store) Order() *OrderStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.order == nil {
		s.order = NewOrderStore(s.gdb)
	}
	return s.order
}

// Grid gets grid trading storage
func (s *Store) Grid() *GridStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.grid == nil {
		s.grid = NewGridStore(s.gdb)
	}
	return s.grid
}

// Close closes database connection
func (s *Store) Close() error {
	if s.driver != nil {
		return s.driver.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GormDB returns the GORM database connection
func (s *Store) GormDB() *gorm.DB {
	return s.gdb
}

// Driver returns database driver for abstraction (legacy)
func (s *Store) Driver() *DBDriver {
	return s.driver
}

// DBType returns current database type
func (s *Store) DBType() DBType {
	if s.driver != nil {
		return s.driver.Type
	}
	// Detect from GORM dialector
	if s.gdb != nil {
		switch s.gdb.Dialector.Name() {
		case "postgres":
			return DBTypePostgres
		default:
			return DBTypeSQLite
		}
	}
	return DBTypeSQLite
}

// q converts query placeholders for current database type (legacy helper)
func (s *Store) q(query string) string {
	return convertQuery(query, s.DBType())
}

// DB gets underlying database connection (for legacy code compatibility)
// Deprecated: use GormDB() instead
func (s *Store) DB() *sql.DB {
	return s.db
}

// GetSystemConfig gets a system configuration value by key
func (s *Store) GetSystemConfig(key string) (string, error) {
	var value string
	result := s.gdb.Raw("SELECT value FROM system_config WHERE key = ?", key).Scan(&value)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", nil
	}
	return value, nil
}

// SetSystemConfig sets a system configuration value
func (s *Store) SetSystemConfig(key, value string) error {
	// Use GORM-compatible upsert
	return s.gdb.Exec(`
		INSERT INTO system_config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value).Error
}

// Transaction executes transaction with GORM
func (s *Store) Transaction(fn func(tx *gorm.DB) error) error {
	return s.gdb.Transaction(fn)
}

// TransactionSQL executes transaction with sql.Tx (legacy)
// Deprecated: Use Transaction() instead
func (s *Store) TransactionSQL(fn func(tx *sql.Tx) error) error {
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
