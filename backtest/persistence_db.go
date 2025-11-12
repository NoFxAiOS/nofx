package backtest

import (
	"database/sql"
)

var persistenceDB *sql.DB

// UseDatabase enables database-backed persistence for all backtest storage operations.
func UseDatabase(db *sql.DB) {
	persistenceDB = db
}

func usingDB() bool {
	return persistenceDB != nil
}
