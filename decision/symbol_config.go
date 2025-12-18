package decision

import (
	"fmt"
	"strings"
)

// SymbolCategory 币种类别
type SymbolCategory string

const (
	BTCETH    SymbolCategory = "btceth"    // BTC/ETH主流币
	ALTCOIN   SymbolCategory = "altcoin"   // 山寨币
	UNKNOWN   SymbolCategory = "unknown"   // 未知
)

// SymbolSpecificConfig 币种特定配置
type SymbolSpecificConfig struct {
	// 杠杆配置
	MinLeverage       float64
	MaxLeverage       float64
	DefaultLeverage   float64

	// 仓位大小配置
	MinPositionSize   float64 // 最小仓位USD
	MaxPositionSize   float64 // 最大仓位USD
	DefaultFraction   float64 // 默认使用账户净值的百分比

	// 风险参数
	MaxDrawdownLimit  float64 // 最大回撤限制百分比
	StopLossPercent   float64 // 默认止损百分比
	TakeProfitPercent float64 // 默认止盈百分比

	// 交易限制
	MaxDailyLosses    int     // 每日最多连续亏损笔数
	MaxPositionsHeld  int     // 最多同时持仓数量
	CooldownMinutes   int     // 平仓后冷却期（分钟）

	// 交易时间限制
	AllowDayTrading   bool    // 是否允许日内交易
	PreferredHours    []int   // 优选交易小时数 (0-23)
}

// SymbolConfigManager 币种配置管理器
type SymbolConfigManager struct {
	configs map[string]*SymbolSpecificConfig
}

// NewSymbolConfigManager 创建配置管理器
func NewSymbolConfigManager() *SymbolConfigManager {
	scm := &SymbolConfigManager{
		configs: make(map[string]*SymbolSpecificConfig),
	}
	scm.initializeDefaults()
	return scm
}

// initializeDefaults 初始化默认配置
func (scm *SymbolConfigManager) initializeDefaults() {
	// BTC/ETH主流币配置（保守策略）
	btcEthConfig := &SymbolSpecificConfig{
		MinLeverage:       1,
		MaxLeverage:       20,
		DefaultLeverage:   5,
		MinPositionSize:   10,
		MaxPositionSize:   1000,
		DefaultFraction:   0.3, // 账户净值的30%
		MaxDrawdownLimit:  20,
		StopLossPercent:   -3,
		TakeProfitPercent: 8,
		MaxDailyLosses:    3,
		MaxPositionsHeld:  2,
		CooldownMinutes:   15,
		AllowDayTrading:   true,
		PreferredHours:    []int{8, 9, 13, 14, 15, 16, 20, 21, 22}, // UTC+0时间
	}

	// 山寨币配置（激进策略）
	altcoinConfig := &SymbolSpecificConfig{
		MinLeverage:       1,
		MaxLeverage:       15,
		DefaultLeverage:   3,
		MinPositionSize:   5,
		MaxPositionSize:   500,
		DefaultFraction:   0.2, // 账户净值的20%
		MaxDrawdownLimit:  25,
		StopLossPercent:   -5,
		TakeProfitPercent: 10,
		MaxDailyLosses:    5,
		MaxPositionsHeld:  3,
		CooldownMinutes:   10,
		AllowDayTrading:   true,
		PreferredHours:    []int{8, 9, 14, 15, 20, 21, 22, 23}, // UTC+0时间
	}

	// 注册配置
	scm.configs["BTCETH"] = btcEthConfig
	scm.configs["ALTCOIN"] = altcoinConfig
}

// GetSymbolCategory 获取币种类别
func GetSymbolCategory(symbol string) SymbolCategory {
	symbol = strings.ToUpper(symbol)

	if strings.Contains(symbol, "BTCUSDT") || strings.Contains(symbol, "ETHUSDT") {
		return BTCETH
	}

	if strings.HasSuffix(symbol, "USDT") {
		return ALTCOIN
	}

	return UNKNOWN
}

// GetConfig 获取币种配置
func (scm *SymbolConfigManager) GetConfig(symbol string) *SymbolSpecificConfig {
	category := GetSymbolCategory(symbol)

	switch category {
	case BTCETH:
		return scm.configs["BTCETH"]
	case ALTCOIN:
		return scm.configs["ALTCOIN"]
	default:
		// 返回默认的山寨币配置
		return scm.configs["ALTCOIN"]
	}
}

// SetCustomConfig 设置自定义配置
func (scm *SymbolConfigManager) SetCustomConfig(key string, config *SymbolSpecificConfig) {
	scm.configs[key] = config
}

// UpdateLeverage 更新杠杆设置
func (scm *SymbolConfigManager) UpdateLeverage(symbol string, leverage float64) error {
	config := scm.GetConfig(symbol)

	if leverage < config.MinLeverage || leverage > config.MaxLeverage {
		return fmt.Errorf("杠杆 %.1f 超出限制 [%.1f, %.1f]",
			leverage, config.MinLeverage, config.MaxLeverage)
	}

	return nil
}

// CalculatePositionSize 计算仓位大小
func (scm *SymbolConfigManager) CalculatePositionSize(symbol string, accountEquity float64) float64 {
	config := scm.GetConfig(symbol)

	// 计算推荐仓位 = 账户净值 * 默认百分比
	recommendedSize := accountEquity * config.DefaultFraction

	// 限制在最小和最大范围内
	if recommendedSize < config.MinPositionSize {
		recommendedSize = config.MinPositionSize
	}
	if recommendedSize > config.MaxPositionSize {
		recommendedSize = config.MaxPositionSize
	}

	return recommendedSize
}

// GetStopLoss 获取推荐止损点
func (scm *SymbolConfigManager) GetStopLoss(symbol string, entryPrice float64) float64 {
	config := scm.GetConfig(symbol)
	category := GetSymbolCategory(symbol)

	// 对于做多：止损价 = 入场价 * (1 + StopLossPercent/100)
	// 对于做空：止损价 = 入场价 * (1 - StopLossPercent/100)

	if category == BTCETH {
		return entryPrice * (1 + config.StopLossPercent/100)
	} else {
		// 山寨币可能更波动，稍微宽松止损
		return entryPrice * (1 + config.StopLossPercent/100)
	}
}

// GetTakeProfit 获取推荐止盈点
func (scm *SymbolConfigManager) GetTakeProfit(symbol string, entryPrice float64) float64 {
	config := scm.GetConfig(symbol)
	category := GetSymbolCategory(symbol)

	// 对于做多：止盈价 = 入场价 * (1 + TakeProfitPercent/100)
	// 对于做空：止盈价 = 入场价 * (1 - TakeProfitPercent/100)

	if category == BTCETH {
		return entryPrice * (1 + config.TakeProfitPercent/100)
	} else {
		// 山寨币可能更易波动，更激进的止盈
		return entryPrice * (1 + config.TakeProfitPercent/100)
	}
}

// GetMaxDrawdownLimit 获取最大回撤限制
func (scm *SymbolConfigManager) GetMaxDrawdownLimit(symbol string) float64 {
	config := scm.GetConfig(symbol)
	return config.MaxDrawdownLimit
}

// GetCooldownMinutes 获取冷却期（分钟）
func (scm *SymbolConfigManager) GetCooldownMinutes(symbol string) int {
	config := scm.GetConfig(symbol)
	return config.CooldownMinutes
}

// IsPreferredTradingTime 检查是否是优选交易时间
func (scm *SymbolConfigManager) IsPreferredTradingTime(symbol string, currentHour int) bool {
	config := scm.GetConfig(symbol)

	if !config.AllowDayTrading {
		return false
	}

	for _, hour := range config.PreferredHours {
		if hour == currentHour {
			return true
		}
	}

	return false
}

// GetMaxPositionsHeld 获取最多同时持仓数量
func (scm *SymbolConfigManager) GetMaxPositionsHeld(symbol string) int {
	config := scm.GetConfig(symbol)
	return config.MaxPositionsHeld
}

// GetMaxDailyLosses 获取每日最多连续亏损笔数
func (scm *SymbolConfigManager) GetMaxDailyLosses(symbol string) int {
	config := scm.GetConfig(symbol)
	return config.MaxDailyLosses
}

// ValidateTradeParameters 验证交易参数
func (scm *SymbolConfigManager) ValidateTradeParameters(symbol string, leverage float64, positionSize, stopLoss, takeProfit float64) error {
	config := scm.GetConfig(symbol)

	// 验证杠杆
	if leverage < config.MinLeverage || leverage > config.MaxLeverage {
		return fmt.Errorf("杠杆 %.1f 不在允许范围 [%.1f, %.1f]",
			leverage, config.MinLeverage, config.MaxLeverage)
	}

	// 验证仓位大小
	if positionSize < config.MinPositionSize || positionSize > config.MaxPositionSize {
		return fmt.Errorf("仓位 %.2f 不在允许范围 [%.2f, %.2f]",
			positionSize, config.MinPositionSize, config.MaxPositionSize)
	}

	// 验证止损止盈
	if stopLoss <= 0 || takeProfit <= 0 {
		return fmt.Errorf("止损和止盈必须大于0")
	}

	return nil
}

// GetConfigSummary 获取配置摘要（用于日志）
func (scm *SymbolConfigManager) GetConfigSummary(symbol string) string {
	config := scm.GetConfig(symbol)
	category := GetSymbolCategory(symbol)

	return fmt.Sprintf(
		"[%s] 杠杆: %.1f-%.1f | 仓位: %.0f-%.0f USD | 止损: %.1f%% | 止盈: %.1f%% | 回撤限: %.1f%% | 冷却: %d分钟",
		category,
		config.MinLeverage, config.MaxLeverage,
		config.MinPositionSize, config.MaxPositionSize,
		config.StopLossPercent, config.TakeProfitPercent,
		config.MaxDrawdownLimit, config.CooldownMinutes)
}
