package config

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"os"
)

// LeverageConfig 杠杆配置
type LeverageConfig struct {
	BTCETHLeverage  int `json:"btc_eth_leverage"` // BTC和ETH的杠杆倍数（主账户建议5-50，子账户≤5）
	AltcoinLeverage int `json:"altcoin_leverage"` // 山寨币的杠杆倍数（主账户建议5-20，子账户≤5）
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `json:"level"` // 日志级别: debug, info, warn, error (默认: info)
}

// Config 总配置
type Config struct {
	BetaMode           bool           `json:"beta_mode"`
	APIServerPort      int            `json:"api_server_port"`
	UseDefaultCoins    bool           `json:"use_default_coins"`
	DefaultCoins       []string       `json:"default_coins"`
	CoinPoolAPIURL     string         `json:"coin_pool_api_url"`
	OITopAPIURL        string         `json:"oi_top_api_url"`
	MaxDailyLoss       float64        `json:"max_daily_loss"`
	MaxDrawdown        float64        `json:"max_drawdown"`
	StopTradingMinutes int            `json:"stop_trading_minutes"`
	Leverage           LeverageConfig `json:"leverage"`
	JWTSecret          string         `json:"jwt_secret"`
	DataKLineTime      string         `json:"data_k_line_time"`
	Log                *LogConfig     `json:"nofx/logger"` // 日志配置
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	// 检查filename是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		logger.Infof("📄 %s不存在，使用默认配置", filename)
		return &Config{}, nil
	}

	// 读取 filename
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取%s失败: %w", filename, err)
	}

	// 解析JSON
	var configFile Config
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("解析%s失败: %w", filename, err)
	}

	return &configFile, nil
}
