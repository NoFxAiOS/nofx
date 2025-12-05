package logger

// Config 日志配置（简化版）
type Config struct {
	Level string `json:"level"` // 日志级别: debug, info, warn, error (默认: info)
}

// SetDefaults 设置默认值
func (c *Config) SetDefaults() {
	if c.Level == "" {
		c.Level = "info"
	}
}
