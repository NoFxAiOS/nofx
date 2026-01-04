package store

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GormDB is the global GORM database connection
var gormDB *gorm.DB

// DB returns the GORM database connection
func DB() *gorm.DB {
	return gormDB
}

// InitGorm initializes GORM with SQLite
func InitGorm(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// Use UTC for all auto-generated timestamps (autoCreateTime, autoUpdateTime)
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Set connection pool for SQLite
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	// Enable foreign keys for SQLite
	db.Exec("PRAGMA foreign_keys = ON")
	db.Exec("PRAGMA journal_mode = DELETE")
	db.Exec("PRAGMA synchronous = FULL")
	db.Exec("PRAGMA busy_timeout = 5000")

	gormDB = db
	return db, nil
}

// InitGormPostgres initializes GORM with PostgreSQL
func InitGormPostgres(host string, port int, user, password, dbname, sslmode string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// Use UTC for all auto-generated timestamps (autoCreateTime, autoUpdateTime)
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL database: %w", err)
	}

	// Set connection pool for PostgreSQL
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)

	gormDB = db
	return db, nil
}

// InitGormWithConfig initializes GORM with provided configuration
// Uses DBConfig from driver.go
func InitGormWithConfig(cfg DBConfig) (*gorm.DB, error) {
	switch cfg.Type {
	case DBTypeSQLite:
		return InitGorm(cfg.Path)

	case DBTypePostgres:
		return InitGormPostgres(
			cfg.Host,
			cfg.Port,
			cfg.User,
			cfg.Password,
			cfg.DBName,
			cfg.SSLMode,
		)

	default:
		return nil, fmt.Errorf("unsupported DB_TYPE: %s (use 'sqlite' or 'postgres')", cfg.Type)
	}
}

// ============================================================================
// Query Scopes - Reusable query helpers
// ============================================================================

// ForUser returns a scope that filters by user_id
func ForUser(userID string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("user_id = ?", userID)
	}
}

// ForTrader returns a scope that filters by trader_id
func ForTrader(traderID string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("trader_id = ?", traderID)
	}
}

// OpenPositions returns a scope for open positions
func OpenPositions() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", "OPEN")
	}
}

// ClosedPositions returns a scope for closed positions
func ClosedPositions() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", "CLOSED")
	}
}

// OrderByCreatedDesc returns a scope that orders by created_at DESC
func OrderByCreatedDesc() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	}
}

// OrderByUpdatedDesc returns a scope that orders by updated_at DESC
func OrderByUpdatedDesc() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order("updated_at DESC")
	}
}

// Paginate returns a scope for pagination
func Paginate(limit, offset int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Limit(limit).Offset(offset)
	}
}
