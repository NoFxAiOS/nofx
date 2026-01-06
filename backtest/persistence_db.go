package backtest

import (
	"database/sql"
	"fmt"
	"strings"
)

var (
	persistenceDB     *sql.DB
	persistenceDBType string // "postgres" or "sqlite"
)

// UseDatabase enables database-backed persistence for all backtest storage operations.
func UseDatabase(db *sql.DB) {
	persistenceDB = db
}

// SetDatabaseType sets the database type for query conversion
func SetDatabaseType(dbType string) {
	persistenceDBType = dbType
}

func usingDB() bool {
	return persistenceDB != nil
}

// convertQuery converts ? placeholders to $1, $2, etc. for PostgreSQL
func convertQuery(query string) string {
	if persistenceDBType != "postgres" {
		return query
	}

	result := query
	index := 1
	for strings.Contains(result, "?") {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", index), 1)
		index++
	}

	// Convert CURRENT_TIMESTAMP for PostgreSQL compatibility
	result = strings.ReplaceAll(result, "datetime('now')", "CURRENT_TIMESTAMP")

	return result
}
