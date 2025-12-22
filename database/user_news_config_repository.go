package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// UserNewsConfig 用户新闻源配置结构体
type UserNewsConfig struct {
	ID                      int
	UserID                  string
	Enabled                 bool
	NewsSources             string    // 逗号分隔的新闻源列表
	AutoFetchIntervalMinutes int
	MaxArticlesPerFetch     int
	SentimentThreshold      float64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// UserNewsConfigRepository 用户新闻配置数据库操作
type UserNewsConfigRepository struct {
	db *sql.DB
}

// NewUserNewsConfigRepository 创建用户新闻配置repository
func NewUserNewsConfigRepository(db *sql.DB) *UserNewsConfigRepository {
	return &UserNewsConfigRepository{db: db}
}

// GetByUserID 获取用户的新闻配置
func (r *UserNewsConfigRepository) GetByUserID(userID string) (*UserNewsConfig, error) {
	query := `
		SELECT id, user_id, enabled, news_sources, auto_fetch_interval_minutes,
		       max_articles_per_fetch, sentiment_threshold, created_at, updated_at
		FROM user_news_config
		WHERE user_id = $1
	`

	config := &UserNewsConfig{}
	err := r.db.QueryRow(query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.Enabled,
		&config.NewsSources,
		&config.AutoFetchIntervalMinutes,
		&config.MaxArticlesPerFetch,
		&config.SentimentThreshold,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("⚠️  用户%s的新闻配置不存在", userID)
			return nil, fmt.Errorf("用户新闻配置不存在: %w", err)
		}
		log.Printf("❌ 查询用户新闻配置失败: %v", err)
		return nil, fmt.Errorf("查询用户新闻配置失败: %w", err)
	}

	return config, nil
}

// Create 创建用户新闻配置
func (r *UserNewsConfigRepository) Create(config *UserNewsConfig) error {
	query := `
		INSERT INTO user_news_config
		(user_id, enabled, news_sources, auto_fetch_interval_minutes, max_articles_per_fetch, sentiment_threshold, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	now := time.Now()
	err := r.db.QueryRow(
		query,
		config.UserID,
		config.Enabled,
		config.NewsSources,
		config.AutoFetchIntervalMinutes,
		config.MaxArticlesPerFetch,
		config.SentimentThreshold,
		now,
		now,
	).Scan(&config.ID)

	if err != nil {
		log.Printf("❌ 创建用户新闻配置失败: %v", err)
		return fmt.Errorf("创建用户新闻配置失败: %w", err)
	}

	log.Printf("✅ 为用户%s创建新闻配置成功", config.UserID)
	return nil
}

// Update 更新用户新闻配置
func (r *UserNewsConfigRepository) Update(config *UserNewsConfig) error {
	query := `
		UPDATE user_news_config
		SET enabled = $1, news_sources = $2, auto_fetch_interval_minutes = $3,
		    max_articles_per_fetch = $4, sentiment_threshold = $5, updated_at = $6
		WHERE user_id = $7
	`

	result, err := r.db.Exec(
		query,
		config.Enabled,
		config.NewsSources,
		config.AutoFetchIntervalMinutes,
		config.MaxArticlesPerFetch,
		config.SentimentThreshold,
		time.Now(),
		config.UserID,
	)

	if err != nil {
		log.Printf("❌ 更新用户新闻配置失败: %v", err)
		return fmt.Errorf("更新用户新闻配置失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		log.Printf("⚠️  用户%s的新闻配置不存在，尝试创建", config.UserID)
		return r.Create(config)
	}

	log.Printf("✅ 更新用户%s的新闻配置成功", config.UserID)
	return nil
}

// Delete 删除用户新闻配置
func (r *UserNewsConfigRepository) Delete(userID string) error {
	query := `DELETE FROM user_news_config WHERE user_id = $1`

	result, err := r.db.Exec(query, userID)
	if err != nil {
		log.Printf("❌ 删除用户新闻配置失败: %v", err)
		return fmt.Errorf("删除用户新闻配置失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		log.Printf("⚠️  用户%s的新闻配置不存在", userID)
		return fmt.Errorf("用户新闻配置不存在")
	}

	log.Printf("✅ 删除用户%s的新闻配置成功", userID)
	return nil
}

// GetEnabledNewsSources 获取启用的新闻源列表
func (config *UserNewsConfig) GetEnabledNewsSources() []string {
	if config.NewsSources == "" {
		return []string{"mlion"} // 默认新闻源
	}
	sources := strings.Split(config.NewsSources, ",")
	result := make([]string, 0, len(sources))
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source != "" {
			result = append(result, source)
		}
	}
	return result
}

// SetNewsSources 设置新闻源列表
func (config *UserNewsConfig) SetNewsSources(sources []string) {
	config.NewsSources = strings.Join(sources, ",")
}

// GetOrCreateDefault 获取或创建默认配置
func (r *UserNewsConfigRepository) GetOrCreateDefault(userID string) (*UserNewsConfig, error) {
	config, err := r.GetByUserID(userID)
	if err == nil {
		return config, nil
	}

	// 如果不存在，创建默认配置
	defaultConfig := &UserNewsConfig{
		UserID:                  userID,
		Enabled:                 true,
		NewsSources:             "mlion",
		AutoFetchIntervalMinutes: 5,
		MaxArticlesPerFetch:     10,
		SentimentThreshold:      0.0,
	}

	err = r.Create(defaultConfig)
	if err != nil {
		return nil, err
	}

	return defaultConfig, nil
}

// ListAllEnabled 列出所有启用的用户配置
func (r *UserNewsConfigRepository) ListAllEnabled() ([]UserNewsConfig, error) {
	query := `
		SELECT id, user_id, enabled, news_sources, auto_fetch_interval_minutes,
		       max_articles_per_fetch, sentiment_threshold, created_at, updated_at
		FROM user_news_config
		WHERE enabled = TRUE
		ORDER BY updated_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		log.Printf("❌ 查询启用的新闻配置列表失败: %v", err)
		return nil, fmt.Errorf("查询启用的新闻配置列表失败: %w", err)
	}
	defer rows.Close()

	var configs []UserNewsConfig
	for rows.Next() {
		var config UserNewsConfig
		err := rows.Scan(
			&config.ID,
			&config.UserID,
			&config.Enabled,
			&config.NewsSources,
			&config.AutoFetchIntervalMinutes,
			&config.MaxArticlesPerFetch,
			&config.SentimentThreshold,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			log.Printf("❌ 扫描新闻配置失败: %v", err)
			return nil, fmt.Errorf("扫描新闻配置失败: %w", err)
		}
		configs = append(configs, config)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果失败: %w", err)
	}

	return configs, nil
}

// ToAPIResponse 将用户新闻配置转换为API响应格式
func (c *UserNewsConfig) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":                           c.ID,
		"user_id":                      c.UserID,
		"enabled":                      c.Enabled,
		"news_sources":                 c.NewsSources,
		"news_sources_list":            c.GetEnabledNewsSources(),
		"auto_fetch_interval_minutes":  c.AutoFetchIntervalMinutes,
		"max_articles_per_fetch":       c.MaxArticlesPerFetch,
		"sentiment_threshold":          c.SentimentThreshold,
		"created_at":                   c.CreatedAt.Unix(),
		"updated_at":                   c.UpdatedAt.Unix(),
	}
}
