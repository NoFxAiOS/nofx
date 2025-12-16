package news

import "nofx/config"

// DBStateStore 实现 StateStore 接口，包装 config.Database
type DBStateStore struct {
	db *config.Database
}

// NewDBStateStore 创建 DBStateStore
func NewDBStateStore(db *config.Database) *DBStateStore {
	return &DBStateStore{db: db}
}

func (s *DBStateStore) GetNewsState(category string) (int64, int64, error) {
	return s.db.GetNewsState(category)
}

func (s *DBStateStore) UpdateNewsState(category string, id int64, timestamp int64) error {
	return s.db.UpdateNewsState(category, id, timestamp)
}

func (s *DBStateStore) GetSystemConfig(key string) (string, error) {
	return s.db.GetSystemConfig(key)
}
