package market

import "time"

// TimeframeData 单个时间框架的数据
type TimeframeData struct {
	Timeframe      string    `json:"timeframe"`       // "3m", "1h", "4h" 等
	DataPoints     int       `json:"data_points"`     // 数据点数量
	MidPrices      []float64 `json:"mid_prices"`      // 中间价格序列
	EMA20Values    []float64 `json:"ema20_values"`    // EMA20 指标值
	MACDValues     []float64 `json:"macd_values"`     // MACD 指标值
	RSI7Values     []float64 `json:"rsi7_values"`     // RSI7 指标值
	RSI14Values    []float64 `json:"rsi14_values"`    // RSI14 指标值
	BollingerUpper []float64 `json:"bollinger_upper"` // 布林带上轨
	BollingerMid   []float64 `json:"bollinger_mid"`   // 布林带中轨
	BollingerLower []float64 `json:"bollinger_lower"` // 布林带下轨
	Volume         []float64 `json:"volume"`          // 成交量序列
	ATR14          float64   `json:"atr14"`           // ATR14 指标值
}

// Data 市场数据结构
type Data struct {
	Symbol        string                    `json:"symbol"`
	CurrentPrice  float64                   `json:"current_price"`
	PriceChange1h float64                   `json:"price_change_1h"` // 1小时价格变化百分比
	PriceChange4h float64                   `json:"price_change_4h"` // 4小时价格变化百分比
	CurrentEMA20  float64                   `json:"current_ema20"`
	CurrentMACD   float64                   `json:"current_macd"`
	CurrentRSI7   float64                   `json:"current_rsi7"`
	OpenInterest  *OIData                   `json:"open_interest"`
	FundingRate   float64                   `json:"funding_rate"`
	TimeframeData map[string]*TimeframeData `json:"timeframe_data"` // 动态时间框架数据

	// Deprecated: 保留用于向后兼容，将在未来版本移除
	// 使用 TimeframeData["3m"] 替代
	IntradaySeries *IntradayData `json:"intraday_series,omitempty"`

	// Deprecated: 保留用于向后兼容，将在未来版本移除
	// 使用 TimeframeData["4h"] 替代
	LongerTermContext *LongerTermData `json:"longer_term_context,omitempty"`
}

// OIData Open Interest数据
type OIData struct {
	Latest  float64
	Average float64
}

// IntradayData 日内数据(3分钟间隔)
type IntradayData struct {
	MidPrices      []float64
	EMA20Values    []float64
	MACDValues     []float64
	RSI7Values     []float64
	RSI14Values    []float64
	Volume         []float64
	ATR14          float64
	BollingerUpper []float64
	BollingerMid   []float64
	BollingerLower []float64
}

// LongerTermData 长期数据(4小时时间框架)
type LongerTermData struct {
	EMA20          float64
	EMA50          float64
	ATR3           float64
	ATR14          float64
	CurrentVolume  float64
	AverageVolume  float64
	MACDValues     []float64
	RSI14Values    []float64
	BollingerUpper []float64
	BollingerMid   []float64
	BollingerLower []float64
}

// Binance API 响应结构
type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

type SymbolInfo struct {
	Symbol            string `json:"symbol"`
	Status            string `json:"status"`
	BaseAsset         string `json:"baseAsset"`
	QuoteAsset        string `json:"quoteAsset"`
	ContractType      string `json:"contractType"`
	PricePrecision    int    `json:"pricePrecision"`
	QuantityPrecision int    `json:"quantityPrecision"`
}

type Kline struct {
	OpenTime            int64   `json:"openTime"`
	Open                float64 `json:"open"`
	High                float64 `json:"high"`
	Low                 float64 `json:"low"`
	Close               float64 `json:"close"`
	Volume              float64 `json:"volume"`
	CloseTime           int64   `json:"closeTime"`
	QuoteVolume         float64 `json:"quoteVolume"`
	Trades              int     `json:"trades"`
	TakerBuyBaseVolume  float64 `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume float64 `json:"takerBuyQuoteVolume"`
}

type KlineResponse []interface{}

type PriceTicker struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Ticker24hr struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
}

// 特征数据结构
type SymbolFeatures struct {
	Symbol           string    `json:"symbol"`
	Timestamp        time.Time `json:"timestamp"`
	Price            float64   `json:"price"`
	PriceChange15Min float64   `json:"price_change_15min"`
	PriceChange1H    float64   `json:"price_change_1h"`
	PriceChange4H    float64   `json:"price_change_4h"`
	Volume           float64   `json:"volume"`
	VolumeRatio5     float64   `json:"volume_ratio_5"`
	VolumeRatio20    float64   `json:"volume_ratio_20"`
	VolumeTrend      float64   `json:"volume_trend"`
	RSI14            float64   `json:"rsi_14"`
	SMA5             float64   `json:"sma_5"`
	SMA10            float64   `json:"sma_10"`
	SMA20            float64   `json:"sma_20"`
	HighLowRatio     float64   `json:"high_low_ratio"`
	Volatility20     float64   `json:"volatility_20"`
	PositionInRange  float64   `json:"position_in_range"`
}

// 警报数据结构
type Alert struct {
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type Config struct {
	AlertThresholds AlertThresholds `json:"alert_thresholds"`
	UpdateInterval  int             `json:"update_interval"` // seconds
	CleanupConfig   CleanupConfig   `json:"cleanup_config"`
}

type AlertThresholds struct {
	VolumeSpike      float64 `json:"volume_spike"`
	PriceChange15Min float64 `json:"price_change_15min"`
	VolumeTrend      float64 `json:"volume_trend"`
	RSIOverbought    float64 `json:"rsi_overbought"`
	RSIOversold      float64 `json:"rsi_oversold"`
}
type CleanupConfig struct {
	InactiveTimeout   time.Duration `json:"inactive_timeout"`    // 不活跃超时时间
	MinScoreThreshold float64       `json:"min_score_threshold"` // 最低评分阈值
	NoAlertTimeout    time.Duration `json:"no_alert_timeout"`    // 无警报超时时间
	CheckInterval     time.Duration `json:"check_interval"`      // 检查间隔
}

var config = Config{
	AlertThresholds: AlertThresholds{
		VolumeSpike:      3.0,
		PriceChange15Min: 0.05,
		VolumeTrend:      2.0,
		RSIOverbought:    70,
		RSIOversold:      30,
	},
	CleanupConfig: CleanupConfig{
		InactiveTimeout:   30 * time.Minute,
		MinScoreThreshold: 15.0,
		NoAlertTimeout:    20 * time.Minute,
		CheckInterval:     5 * time.Minute,
	},
	UpdateInterval: 60, // 1 minute
}

// IndicatorConfig 指标配置结构
type IndicatorConfig struct {
	Indicators []string       `json:"indicators"`  // 启用的指标列表: ["ema", "macd", "rsi", "atr", "volume", "bollinger"]
	Timeframes []string       `json:"timeframes"`  // 启用的时间框架: ["3m", "15m", "1h", "4h", "1d"]
	DataPoints map[string]int `json:"data_points"` // 每个时间框架的数据点数量: {"3m": 40, "4h": 25}
	Parameters map[string]int `json:"parameters"`  // 指标参数: {"rsi_period": 14, "ema_period": 20}
}

// GetDefaultIndicatorConfig 返回默认的指标配置
func GetDefaultIndicatorConfig() *IndicatorConfig {
	return &IndicatorConfig{
		Indicators: []string{"ema", "macd", "rsi", "atr", "volume"},
		Timeframes: []string{"3m", "4h"},
		DataPoints: map[string]int{
			"3m": 40, // 40条3分钟K线 = 2小时
			"4h": 25, // 25条4小时K线 = 4.2天
		},
		Parameters: map[string]int{
			"rsi_period":  14,
			"ema_period":  20,
			"macd_fast":   12,
			"macd_slow":   26,
			"macd_signal": 9,
			"atr_period":  14,
		},
	}
}
