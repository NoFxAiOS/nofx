package config

import (
	"os"
	"strconv"
	"strings"
)

// 全局配置实例
var global *Config

// Config 全局配置（从 .env 加载）
// 只包含真正的全局配置，交易相关配置在 trader/策略 级别
type Config struct {
	// 服务配置
	APIServerPort       int
	JWTSecret           string
	RegistrationEnabled bool
}

// Init 初始化全局配置（从 .env 加载）
func Init() {
	cfg := &Config{
		APIServerPort:       8080,
		RegistrationEnabled: true,
	}

	// 从环境变量加载
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = strings.TrimSpace(v)
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "default-jwt-secret-change-in-production"
	}

	if v := os.Getenv("REGISTRATION_ENABLED"); v != "" {
		cfg.RegistrationEnabled = strings.ToLower(v) == "true"
	}

	if v := os.Getenv("API_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.APIServerPort = port
		}
	}

	global = cfg
}

// Get 获取全局配置
func Get() *Config {
	if global == nil {
		Init()
	}
	return global
}
