package database

import (
	"os"
)

// GetConfigFromEnv 从环境变量获取数据库配置
func GetConfigFromEnv() DBConfig {
	return DBConfig{
		UseNeon:    os.Getenv("USE_NEON") == "true",
		NeonDSN:    os.Getenv("DATABASE_URL"),
		SQLitePath: os.Getenv("SQLITE_PATH"),
	}
}
