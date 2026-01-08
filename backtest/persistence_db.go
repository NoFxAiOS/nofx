package backtest

import (
	"database/sql"
	"fmt"
	"strings"
)

var persistenceDB *sql.DB
var dbIsPostgres bool

// UseDatabase enables database-backed persistence for all backtest storage operations.
// If isPostgres is true, queries will use $1, $2... placeholders instead of ?
func UseDatabase(db *sql.DB) {
	persistenceDB = db
}

// UseDatabaseWithType enables database-backed persistence with explicit type.
func UseDatabaseWithType(db *sql.DB, isPostgres bool) {
	persistenceDB = db
	dbIsPostgres = isPostgres
}

func usingDB() bool {
	return persistenceDB != nil
}

// convertQuery converts ? placeholders to $1, $2, etc. for PostgreSQL
func convertQuery(query string) string {
	if !dbIsPostgres {
		return query
	}
	result := query
	index := 1
	for strings.Contains(result, "?") {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", index), 1)
		index++
	}
	return result
}
