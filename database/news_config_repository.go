package database

// NewsConfigRepository 定义新闻配置数据库操作接口
type NewsConfigRepository interface {
	GetByUserID(userID string) (*UserNewsConfig, error)
	Create(config *UserNewsConfig) error
	Update(config *UserNewsConfig) error
	Delete(userID string) error
	GetOrCreateDefault(userID string) (*UserNewsConfig, error)
	ListAllEnabled() ([]UserNewsConfig, error)
}
