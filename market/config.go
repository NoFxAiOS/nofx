package market

import (
	"fmt"
	"strings"
)

// MarketDataConfig 市场数据配置结构
type MarketDataConfig struct {
	Klines     []KlineConfig   `json:"klines"`     // K线级别配置
	Indicators IndicatorConfig `json:"indicators"` // 技术指标配置
}

// KlineConfig K线配置
type KlineConfig struct {
	Interval string `json:"interval"` // K线间隔: 1s, 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M
	Limit    int    `json:"limit"`    // 需要获取的K线数量
}

// IndicatorConfig 技术指标配置
type IndicatorConfig struct {
	EMA            []EMAConfig           `json:"ema,omitempty"`             // EMA配置列表
	MACD           *MACDConfig           `json:"macd,omitempty"`            // MACD配置
	RSI            []RSIConfig           `json:"rsi,omitempty"`             // RSI配置列表
	ATR            []ATRConfig           `json:"atr,omitempty"`             // ATR配置列表
	SMA            []SMAConfig           `json:"sma,omitempty"`             // SMA配置列表（可选）
	BollingerBands *BollingerBandsConfig `json:"bollinger_bands,omitempty"` // 布林带配置
}

// EMAConfig EMA指标配置
type EMAConfig struct {
	Period  int      `json:"period"`  // EMA周期
	Sources []string `json:"sources"` // 使用的K线级别列表
}

// MACDConfig MACD指标配置
type MACDConfig struct {
	Fast    int      `json:"fast"`    // 快线周期（默认12）
	Slow    int      `json:"slow"`    // 慢线周期（默认26）
	Signal  int      `json:"signal"`  // 信号线周期（默认9）
	Sources []string `json:"sources"` // 使用的K线级别列表
}

// RSIConfig RSI指标配置
type RSIConfig struct {
	Period  int      `json:"period"`  // RSI周期
	Sources []string `json:"sources"` // 使用的K线级别列表
}

// ATRConfig ATR指标配置
type ATRConfig struct {
	Period  int      `json:"period"`  // ATR周期
	Sources []string `json:"sources"` // 使用的K线级别列表
}

// SMAConfig SMA指标配置（可选）
type SMAConfig struct {
	Period  int      `json:"period"`  // SMA周期
	Sources []string `json:"sources"` // 使用的K线级别列表
}

// BollingerBandsConfig 布林带指标配置
type BollingerBandsConfig struct {
	Period  int      `json:"period"`  // 布林带周期（默认20）
	StdDev  float64  `json:"std_dev"` // 标准差倍数（默认2.0）
	Sources []string `json:"sources"` // 使用的K线级别列表
}

// ValidKlineIntervals 币安支持的有效K线间隔列表
var ValidKlineIntervals = map[string]bool{
	// 秒级
	"1s": true,
	// 分钟级
	"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
	// 小时级
	"1h": true, "2h": true, "4h": true, "6h": true, "8h": true, "12h": true,
	// 天级
	"1d": true, "3d": true,
	// 周级
	"1w": true,
	// 月级
	"1M": true,
}

// Validate 验证配置的有效性
func (c *MarketDataConfig) Validate() error {
	// 验证K线配置
	if len(c.Klines) == 0 {
		return fmt.Errorf("至少需要配置一个K线级别")
	}

	intervalSet := make(map[string]bool)
	for i, klineCfg := range c.Klines {
		// 验证间隔格式
		interval := strings.TrimSpace(klineCfg.Interval)
		if interval == "" {
			return fmt.Errorf("K线配置[%d]: interval不能为空", i)
		}

		// 验证是否为有效的K线间隔
		if !IsValidKlineInterval(interval) {
			return fmt.Errorf("K线配置[%d]: 无效的interval '%s'，支持的间隔: %v", i, interval, getValidIntervalsList())
		}

		// 检查重复
		if intervalSet[interval] {
			return fmt.Errorf("K线配置[%d]: interval '%s' 重复", i, interval)
		}
		intervalSet[interval] = true

		// 验证limit
		if klineCfg.Limit <= 0 {
			return fmt.Errorf("K线配置[%d]: limit必须大于0", i)
		}
		if klineCfg.Limit > 1000 {
			return fmt.Errorf("K线配置[%d]: limit不能超过1000（币安API限制）", i)
		}
	}

	// 验证指标配置中引用的K线级别
	if err := c.Indicators.Validate(intervalSet); err != nil {
		return fmt.Errorf("指标配置验证失败: %w", err)
	}

	return nil
}

// Validate 验证指标配置
func (ic *IndicatorConfig) Validate(availableIntervals map[string]bool) error {
	// 验证EMA配置
	for i, ema := range ic.EMA {
		if ema.Period <= 0 {
			return fmt.Errorf("EMA配置[%d]: period必须大于0", i)
		}
		if len(ema.Sources) == 0 {
			return fmt.Errorf("EMA配置[%d]: sources不能为空", i)
		}
		for _, source := range ema.Sources {
			if !availableIntervals[source] {
				return fmt.Errorf("EMA配置[%d]: 引用了不存在的K线级别 '%s'", i, source)
			}
		}
	}

	// 验证MACD配置
	if ic.MACD != nil {
		if ic.MACD.Fast <= 0 || ic.MACD.Slow <= 0 || ic.MACD.Signal <= 0 {
			return fmt.Errorf("MACD配置: fast/slow/signal必须大于0")
		}
		if ic.MACD.Fast >= ic.MACD.Slow {
			return fmt.Errorf("MACD配置: fast周期(%d)必须小于slow周期(%d)", ic.MACD.Fast, ic.MACD.Slow)
		}
		if len(ic.MACD.Sources) == 0 {
			return fmt.Errorf("MACD配置: sources不能为空")
		}
		for _, source := range ic.MACD.Sources {
			if !availableIntervals[source] {
				return fmt.Errorf("MACD配置: 引用了不存在的K线级别 '%s'", source)
			}
		}
	}

	// 验证RSI配置
	for i, rsi := range ic.RSI {
		if rsi.Period <= 0 {
			return fmt.Errorf("RSI配置[%d]: period必须大于0", i)
		}
		if len(rsi.Sources) == 0 {
			return fmt.Errorf("RSI配置[%d]: sources不能为空", i)
		}
		for _, source := range rsi.Sources {
			if !availableIntervals[source] {
				return fmt.Errorf("RSI配置[%d]: 引用了不存在的K线级别 '%s'", i, source)
			}
		}
	}

	// 验证ATR配置
	for i, atr := range ic.ATR {
		if atr.Period <= 0 {
			return fmt.Errorf("ATR配置[%d]: period必须大于0", i)
		}
		if len(atr.Sources) == 0 {
			return fmt.Errorf("ATR配置[%d]: sources不能为空", i)
		}
		for _, source := range atr.Sources {
			if !availableIntervals[source] {
				return fmt.Errorf("ATR配置[%d]: 引用了不存在的K线级别 '%s'", i, source)
			}
		}
	}

	// 验证SMA配置（如果有）
	for i, sma := range ic.SMA {
		if sma.Period <= 0 {
			return fmt.Errorf("SMA配置[%d]: period必须大于0", i)
		}
		if len(sma.Sources) == 0 {
			return fmt.Errorf("SMA配置[%d]: sources不能为空", i)
		}
		for _, source := range sma.Sources {
			if !availableIntervals[source] {
				return fmt.Errorf("SMA配置[%d]: 引用了不存在的K线级别 '%s'", i, source)
			}
		}
	}

	// 验证布林带配置（如果有）
	if ic.BollingerBands != nil {
		if ic.BollingerBands.Period <= 0 {
			return fmt.Errorf("布林带配置: period必须大于0")
		}
		if ic.BollingerBands.StdDev <= 0 {
			return fmt.Errorf("布林带配置: std_dev必须大于0")
		}
		if len(ic.BollingerBands.Sources) == 0 {
			return fmt.Errorf("布林带配置: sources不能为空")
		}
		for _, source := range ic.BollingerBands.Sources {
			if !availableIntervals[source] {
				return fmt.Errorf("布林带配置: 引用了不存在的K线级别 '%s'", source)
			}
		}
	}

	return nil
}

// IsValidKlineInterval 检查K线间隔是否有效
func IsValidKlineInterval(interval string) bool {
	return ValidKlineIntervals[interval]
}

// getValidIntervalsList 获取有效间隔列表（用于错误提示）
func getValidIntervalsList() []string {
	intervals := make([]string, 0, len(ValidKlineIntervals))
	for interval := range ValidKlineIntervals {
		intervals = append(intervals, interval)
	}
	return intervals
}
