// Package store provides database driver abstraction
package store

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"      // PostgreSQL driver
	_ "modernc.org/sqlite"     // SQLite driver
)

// DBType represents database type
type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

// DBConfig database configuration
type DBConfig struct {
	Type     DBType // sqlite or postgres
	Path     string // SQLite file path (for sqlite)
	Host     string // PostgreSQL host (for postgres)
	Port     int    // PostgreSQL port (for postgres)
	User     string // PostgreSQL user (for postgres)
	Password string // PostgreSQL password (for postgres)
	DBName   string // PostgreSQL database name (for postgres)
	SSLMode  string // PostgreSQL SSL mode (for postgres)
}

// DBDriver database driver abstraction
type DBDriver struct {
	Type DBType
	db   *sql.DB
}

// NewDBDriver creates database driver from config
func NewDBDriver(cfg DBConfig) (*DBDriver, error) {
	var db *sql.DB
	var err error

	switch cfg.Type {
	case DBTypeSQLite:
		db, err = openSQLite(cfg.Path)
	case DBTypePostgres:
		db, err = openPostgres(cfg)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	if err != nil {
		return nil, err
	}

	return &DBDriver{Type: cfg.Type, db: db}, nil
}

// NewDBDriverFromEnv creates database driver from environment variables
// DB_TYPE: sqlite (default) or postgres
// For SQLite: DB_PATH (default: data/data.db)
// For PostgreSQL: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
func NewDBDriverFromEnv() (*DBDriver, error) {
	dbType := DBType(strings.ToLower(getEnv("DB_TYPE", "sqlite")))

	switch dbType {
	case DBTypeSQLite:
		path := getEnv("DB_PATH", "data/data.db")
		return NewDBDriver(DBConfig{Type: DBTypeSQLite, Path: path})

	case DBTypePostgres:
		port := 5432
		if p := os.Getenv("DB_PORT"); p != "" {
			fmt.Sscanf(p, "%d", &port)
		}
		return NewDBDriver(DBConfig{
			Type:     DBTypePostgres,
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     port,
			User:     getEnv("DB_USER", "postgres"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   getEnv("DB_NAME", "nofx"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		})

	default:
		return nil, fmt.Errorf("unsupported DB_TYPE: %s (use 'sqlite' or 'postgres')", dbType)
	}
}

// DB returns underlying database connection
func (d *DBDriver) DB() *sql.DB {
	return d.db
}

// Close closes database connection
func (d *DBDriver) Close() error {
	return d.db.Close()
}

// AutoIncrement returns auto-increment syntax for current database
func (d *DBDriver) AutoIncrement() string {
	switch d.Type {
	case DBTypePostgres:
		return "SERIAL"
	default:
		return "INTEGER PRIMARY KEY AUTOINCREMENT"
	}
}

// Placeholder returns placeholder for parameterized queries
// SQLite uses ?, PostgreSQL uses $1, $2, etc.
func (d *DBDriver) Placeholder(index int) string {
	switch d.Type {
	case DBTypePostgres:
		return fmt.Sprintf("$%d", index)
	default:
		return "?"
	}
}

// ConvertPlaceholders converts ? placeholders to database-specific format
func (d *DBDriver) ConvertPlaceholders(query string) string {
	if d.Type != DBTypePostgres {
		return query
	}

	// Convert ? to $1, $2, etc. for PostgreSQL
	result := query
	index := 1
	for strings.Contains(result, "?") {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", index), 1)
		index++
	}
	return result
}

// TableExists checks if a table exists
func (d *DBDriver) TableExists(tableName string) (bool, error) {
	var exists bool
	var query string

	switch d.Type {
	case DBTypePostgres:
		query = `SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`
	default:
		query = `SELECT EXISTS (
			SELECT 1 FROM sqlite_master
			WHERE type = 'table' AND name = ?
		)`
	}

	query = d.ConvertPlaceholders(query)
	err := d.db.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// UpsertSyntax returns the upsert syntax for current database
// SQLite: INSERT ... ON CONFLICT(...) DO UPDATE SET ...
// PostgreSQL: INSERT ... ON CONFLICT(...) DO UPDATE SET ...
// Both use the same syntax in modern versions
func (d *DBDriver) UpsertSyntax() string {
	return "ON CONFLICT"
}

// openSQLite opens SQLite database
func openSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// SQLite configuration
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Enable foreign key constraints
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Use DELETE mode for Docker compatibility
	if _, err := db.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set journal_mode: %w", err)
	}

	// Set synchronous=FULL
	if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set synchronous: %w", err)
	}

	// Set busy_timeout
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	return db, nil
}

// openPostgres opens PostgreSQL database
func openPostgres(cfg DBConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL database: %w", err)
	}

	// PostgreSQL configuration
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	return db, nil
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// convertQuery converts ? placeholders to $1, $2 for PostgreSQL
// and handles other database-specific syntax differences
func convertQuery(query string, dbType DBType) string {
	if dbType != DBTypePostgres {
		return query
	}
	result := query

	// Convert ? to $1, $2, etc. for PostgreSQL
	index := 1
	for strings.Contains(result, "?") {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", index), 1)
		index++
	}

	// Convert datetime('now') to CURRENT_TIMESTAMP
	result = strings.ReplaceAll(result, "datetime('now')", "CURRENT_TIMESTAMP")

	// Remove datetime() wrapper for ORDER BY (PostgreSQL timestamps sort correctly)
	// This handles patterns like "ORDER BY datetime(column) DESC"
	result = strings.ReplaceAll(result, "datetime(updated_at)", "updated_at")
	result = strings.ReplaceAll(result, "datetime(created_at)", "created_at")

	return result
}

// boolDefault returns database-appropriate boolean default for COALESCE
// Use in queries like: COALESCE(column, %s)
func boolDefault(dbType DBType, value bool) string {
	if dbType == DBTypePostgres {
		if value {
			return "TRUE"
		}
		return "FALSE"
	}
	if value {
		return "1"
	}
	return "0"
}
