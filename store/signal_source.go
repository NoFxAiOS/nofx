package store

import (
	"database/sql"
	"time"
)

// SignalSourceStore 信号源存储
type SignalSourceStore struct {
	db *sql.DB
}

// SignalSource 用户信号源配置
type SignalSource struct {
	ID          int       `json:"id"`
	UserID      string    `json:"user_id"`
	CoinPoolURL string    `json:"coin_pool_url"`
	OITopURL    string    `json:"oi_top_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *SignalSourceStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_signal_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			coin_pool_url TEXT DEFAULT '',
			oi_top_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		)
	`)
	if err != nil {
		return err
	}

	// 触发器
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_user_signal_sources_updated_at
		AFTER UPDATE ON user_signal_sources
		BEGIN
			UPDATE user_signal_sources SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	return err
}

// Create 创建信号源配置
func (s *SignalSourceStore) Create(userID, coinPoolURL, oiTopURL string) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, userID, coinPoolURL, oiTopURL)
	return err
}

// Get 获取信号源配置
func (s *SignalSourceStore) Get(userID string) (*SignalSource, error) {
	var source SignalSource
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, coin_pool_url, oi_top_url, created_at, updated_at
		FROM user_signal_sources WHERE user_id = ?
	`, userID).Scan(
		&source.ID, &source.UserID, &source.CoinPoolURL, &source.OITopURL,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	source.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	source.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &source, nil
}

// Update 更新信号源配置
func (s *SignalSourceStore) Update(userID, coinPoolURL, oiTopURL string) error {
	_, err := s.db.Exec(`
		UPDATE user_signal_sources SET coin_pool_url = ?, oi_top_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, coinPoolURL, oiTopURL, userID)
	return err
}
