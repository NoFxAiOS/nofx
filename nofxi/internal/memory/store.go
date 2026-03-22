package memory

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the SQLite-backed memory store.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) the SQLite database.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			exchange TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'market',
			price REAL NOT NULL,
			quantity REAL NOT NULL,
			pnl REAL DEFAULT 0,
			fee REAL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'open',
			ai_model TEXT,
			ai_reason TEXT,
			strategy_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			closed_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_preferences (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, key)
		)`,
		`CREATE TABLE IF NOT EXISTS strategy_performance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			strategy_id TEXT NOT NULL UNIQUE,
			strategy_name TEXT,
			total_trades INTEGER DEFAULT 0,
			win_rate REAL DEFAULT 0,
			total_pnl REAL DEFAULT 0,
			max_drawdown REAL DEFAULT 0,
			sharpe_ratio REAL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS market_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL,
			price REAL NOT NULL,
			volume_24h REAL,
			change_24h REAL,
			note TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_status ON trades(status)`,
		`CREATE INDEX IF NOT EXISTS idx_conversations_user ON conversations(user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_symbol ON market_snapshots(symbol, created_at)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("exec %q: %w", q[:40], err)
		}
	}
	return nil
}

// --- Trade Operations ---

// SaveTrade inserts a new trade record.
func (s *Store) SaveTrade(t *TradeRecord) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO trades (exchange, symbol, side, type, price, quantity, pnl, fee, status, ai_model, ai_reason, strategy_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Exchange, t.Symbol, t.Side, t.Type, t.Price, t.Quantity,
		t.PnL, t.Fee, t.Status, t.AIModel, t.AIReason, t.StrategyID,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetRecentTrades returns the last N trades.
func (s *Store) GetRecentTrades(limit int) ([]TradeRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, exchange, symbol, side, type, price, quantity, pnl, fee, status, 
		        COALESCE(ai_model,''), COALESCE(ai_reason,''), COALESCE(strategy_id,''), created_at, closed_at
		 FROM trades ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []TradeRecord
	for rows.Next() {
		var t TradeRecord
		var closedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.Exchange, &t.Symbol, &t.Side, &t.Type,
			&t.Price, &t.Quantity, &t.PnL, &t.Fee, &t.Status,
			&t.AIModel, &t.AIReason, &t.StrategyID, &t.CreatedAt, &closedAt); err != nil {
			return nil, err
		}
		if closedAt.Valid {
			t.ClosedAt = &closedAt.Time
		}
		trades = append(trades, t)
	}
	return trades, nil
}

// --- Conversation Operations ---

// SaveMessage stores a conversation message.
func (s *Store) SaveMessage(userID int64, role, content string) error {
	_, err := s.db.Exec(
		`INSERT INTO conversations (user_id, role, content) VALUES (?, ?, ?)`,
		userID, role, content,
	)
	return err
}

// GetRecentMessages returns the last N messages for a user.
func (s *Store) GetRecentMessages(userID int64, limit int) ([]Conversation, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, role, content, created_at FROM conversations 
		 WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.UserID, &c.Role, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, c)
	}

	// Reverse to get chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// --- Preference Operations ---

// SetPreference upserts a user preference.
func (s *Store) SetPreference(userID int64, key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO user_preferences (user_id, key, value, updated_at) 
		 VALUES (?, ?, ?, ?) 
		 ON CONFLICT(user_id, key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		userID, key, value, time.Now(),
	)
	return err
}

// GetPreference retrieves a user preference.
func (s *Store) GetPreference(userID int64, key string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM user_preferences WHERE user_id = ? AND key = ?`,
		userID, key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}
