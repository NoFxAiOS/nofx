package config

import (
	"database/sql"
)

// GetNewsState 获取新闻推送状态
func (d *Database) GetNewsState(category string) (int64, int64, error) {
	var lastID, lastTimestamp int64
	err := d.queryRow(`
                SELECT last_id, last_timestamp FROM news_feed_state WHERE category = $1
        `, category).Scan(&lastID, &lastTimestamp)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return lastID, lastTimestamp, nil
}

// UpdateNewsState 更新新闻推送状态
func (d *Database) UpdateNewsState(category string, id int64, timestamp int64) error {
	_, err := d.exec(`
                INSERT INTO news_feed_state (category, last_id, last_timestamp, updated_at)
                VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
                ON CONFLICT (category) DO UPDATE SET
                        last_id = GREATEST(news_feed_state.last_id, EXCLUDED.last_id),
                        last_timestamp = GREATEST(news_feed_state.last_timestamp, EXCLUDED.last_timestamp),
                        updated_at = CURRENT_TIMESTAMP
        `, category, id, timestamp)
	return err
}
