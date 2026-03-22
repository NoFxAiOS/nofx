package agent

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the complete NOFXi configuration.
type Config struct {
	Agent     AgentConfig     `yaml:"agent"`
	Telegram  TelegramConfig  `yaml:"telegram"`
	LLM       LLMConfig       `yaml:"llm"`
	Database  DatabaseConfig  `yaml:"database"`
	Exchanges []ExchangeConfig `yaml:"exchanges"`
}

type AgentConfig struct {
	Name     string `yaml:"name"`      // Agent display name
	Language string `yaml:"language"`   // "en" or "zh"
	LogLevel string `yaml:"log_level"`  // "debug", "info", "warn", "error"
	WebPort  int    `yaml:"web_port"`   // REST API port (0 = disabled)
}

type TelegramConfig struct {
	Token      string  `yaml:"token"`
	AllowedIDs []int64 `yaml:"allowed_ids"` // Allowed Telegram user IDs (empty = allow all)
}

type LLMConfig struct {
	Provider string `yaml:"provider"`  // "openai", "claw402"
	BaseURL  string `yaml:"base_url"`  // API base URL
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`     // e.g. "gpt-4o", "deepseek-chat"
}

type DatabaseConfig struct {
	Path string `yaml:"path"` // SQLite file path
}

type ExchangeConfig struct {
	Name      string `yaml:"name"`       // "binance", "okx", "bybit", etc.
	APIKey    string `yaml:"api_key"`
	APISecret string `yaml:"api_secret"`
	Passphrase string `yaml:"passphrase"` // OKX needs this
	Testnet   bool   `yaml:"testnet"`
}

// LoadConfig reads config from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		Agent: AgentConfig{
			Name:     "NOFXi",
			Language: "en",
			LogLevel: "info",
		},
		Database: DatabaseConfig{
			Path: "nofxi.db",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Override with env vars if set
	if v := os.Getenv("NOFXI_TELEGRAM_TOKEN"); v != "" {
		cfg.Telegram.Token = v
	}
	if v := os.Getenv("NOFXI_LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("NOFXI_LLM_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}

	return cfg, nil
}

// Validate checks required fields.
func (c *Config) Validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	if c.LLM.APIKey == "" {
		return fmt.Errorf("llm.api_key is required")
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("llm.model is required")
	}
	return nil
}
