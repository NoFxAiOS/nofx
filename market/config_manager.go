package market

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// ConfigManager 市场数据配置管理器
type ConfigManager struct {
	configs   map[string]*MarketDataConfig // key: filename 或 "default"
	mu        sync.RWMutex
	configDir string // 配置文件目录
}

var (
	// globalConfigManager 全局配置管理器
	globalConfigManager *ConfigManager
	// marketConfigsDir 市场数据配置文件夹路径
	marketConfigsDir = "market_configs"
)

// init 包初始化时创建配置管理器
func init() {
	globalConfigManager = NewConfigManager()
}

// NewConfigManager 创建配置管理器
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		configs:   make(map[string]*MarketDataConfig),
		configDir: marketConfigsDir,
	}
}

// LoadConfig 加载指定名称的市场数据配置文件
// configName: 配置名称（对应market_configs文件夹下的JSON文件名，不含扩展名，如"default"）
// 如果配置文件不存在，返回默认配置（如果default.json存在）
func (cm *ConfigManager) LoadConfig(configName string) (*MarketDataConfig, error) {
	cm.mu.RLock()
	if config, exists := cm.configs[configName]; exists {
		cm.mu.RUnlock()
		return config, nil
	}
	cm.mu.RUnlock()

	// 尝试加载指定名称的配置文件
	configPath := filepath.Join(cm.configDir, configName+".json")
	config, err := cm.loadConfigFromFile(configPath)
	if err == nil {
		cm.mu.Lock()
		cm.configs[configName] = config
		cm.mu.Unlock()
		return config, nil
	}

	// 如果加载失败且不是文件不存在错误，返回错误
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("加载配置文件失败 %s: %w", configPath, err)
	}

	// 文件不存在，尝试加载默认配置
	defaultConfig, err := cm.LoadDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("市场数据配置文件 %s.json 不存在，且无法加载默认配置: %w", configName, err)
	}

	// 使用默认配置
	cm.mu.Lock()
	cm.configs[configName] = defaultConfig
	cm.mu.Unlock()

	log.Printf("📊 配置 %s 不存在，使用默认市场数据配置", configName)
	return defaultConfig, nil
}

// LoadDefaultConfig 加载默认配置
func (cm *ConfigManager) LoadDefaultConfig() (*MarketDataConfig, error) {
	cm.mu.RLock()
	if config, exists := cm.configs["default"]; exists {
		cm.mu.RUnlock()
		return config, nil
	}
	cm.mu.RUnlock()

	defaultPath := filepath.Join(cm.configDir, "default.json")
	config, err := cm.loadConfigFromFile(defaultPath)
	if err != nil {
		return nil, fmt.Errorf("加载默认配置文件失败: %w", err)
	}

	cm.mu.Lock()
	cm.configs["default"] = config
	cm.mu.Unlock()

	log.Printf("📊 已加载默认市场数据配置")
	return config, nil
}

// loadConfigFromFile 从文件加载配置
func (cm *ConfigManager) loadConfigFromFile(filePath string) (*MarketDataConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config MarketDataConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// SaveConfig 保存配置到文件
// configName: 配置名称（将保存为 market_configs/{configName}.json）
func (cm *ConfigManager) SaveConfig(configName string, config *MarketDataConfig) error {
	// 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(cm.configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 保存到文件
	filePath := filepath.Join(cm.configDir, configName+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}

	// 更新缓存
	cm.mu.Lock()
	cm.configs[configName] = config
	cm.mu.Unlock()

	log.Printf("✓ 已保存市场数据配置: %s", configName)
	return nil
}

// EnsureDefaultConfigExists 确保默认配置文件存在，如果不存在则创建
func (cm *ConfigManager) EnsureDefaultConfigExists() error {
	defaultPath := filepath.Join(cm.configDir, "default.json")

	// 检查文件是否存在
	if _, err := os.Stat(defaultPath); err == nil {
		// 文件已存在，尝试加载以验证
		_, err := cm.LoadDefaultConfig()
		return err
	}

	// 文件不存在，创建默认配置
	log.Printf("📊 创建默认市场数据配置文件...")
	defaultConfig := getDefaultMarketDataConfig()
	return cm.SaveConfig("default", defaultConfig)
}

// ReloadConfig 重新加载指定名称的配置
func (cm *ConfigManager) ReloadConfig(configName string) error {
	cm.mu.Lock()
	delete(cm.configs, configName)
	cm.mu.Unlock()

	_, err := cm.LoadConfig(configName)
	return err
}

// === 全局函数（供外部调用）===

// GetMarketDataConfig 获取指定名称的市场数据配置（全局函数）
// configName: 配置名称（对应 market_configs 文件夹下的JSON文件名，不含扩展名，如 "default"）
func GetMarketDataConfig(configName string) (*MarketDataConfig, error) {
	return globalConfigManager.LoadConfig(configName)
}

// SaveMarketDataConfig 保存指定名称的市场数据配置（全局函数）
// configName: 配置名称（将保存为 market_configs/{configName}.json）
func SaveMarketDataConfig(configName string, config *MarketDataConfig) error {
	return globalConfigManager.SaveConfig(configName, config)
}

// EnsureDefaultMarketDataConfigExists 确保默认配置文件存在（全局函数）
func EnsureDefaultMarketDataConfigExists() error {
	return globalConfigManager.EnsureDefaultConfigExists()
}

// ListMarketConfigs 列出所有可用的市场数据配置文件（全局函数）
func ListMarketConfigs() ([]string, error) {
	return globalConfigManager.ListConfigs()
}

// ListConfigs 列出所有可用的配置文件名称（不包含.json后缀）
func (cm *ConfigManager) ListConfigs() ([]string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 读取目录中的所有.json文件
	files, err := os.ReadDir(cm.configDir)
	if err != nil {
		// 如果目录不存在，返回空列表
		if os.IsNotExist(err) {
			return []string{"default"}, nil
		}
		return nil, fmt.Errorf("读取配置目录失败: %w", err)
	}

	var configs []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		// 只处理.json文件
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			// 移除.json后缀
			configName := name[:len(name)-5]
			configs = append(configs, configName)
		}
	}

	// 如果没有找到任何配置，至少返回default
	if len(configs) == 0 {
		return []string{"default"}, nil
	}

	return configs, nil
}

// getDefaultMarketDataConfig 获取默认市场数据配置（对应当前硬编码的3m/4h配置）
func getDefaultMarketDataConfig() *MarketDataConfig {
	return &MarketDataConfig{
		Klines: []KlineConfig{
			{Interval: "3m", Limit: 40}, // 对应原来的3分钟K线
			{Interval: "4h", Limit: 60}, // 对应原来的4小时K线
		},
		Indicators: IndicatorConfig{
			EMA: []EMAConfig{
				{Period: 20, Sources: []string{"3m"}}, // EMA20基于3分钟
				{Period: 50, Sources: []string{"4h"}}, // EMA50基于4小时（用于长期数据）
			},
			MACD: &MACDConfig{
				Fast:    12,
				Slow:    26,
				Signal:  9,
				Sources: []string{"3m"},
			},
			RSI: []RSIConfig{
				{Period: 7, Sources: []string{"3m"}},        // RSI7基于3分钟
				{Period: 14, Sources: []string{"3m", "4h"}}, // RSI14同时基于3分钟和4小时
			},
			ATR: []ATRConfig{
				{Period: 3, Sources: []string{"4h"}},  // ATR3基于4小时
				{Period: 14, Sources: []string{"4h"}}, // ATR14基于4小时
			},
		},
	}
}
