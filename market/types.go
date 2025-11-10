package market

import (
	"time"

	"github.com/greatcloak/decimal"
)

// Data 市场数据结构
type Data struct {
	Symbol            string
	CurrentPrice      decimal.Decimal
	PriceChange1h     decimal.Decimal // 1小时价格变化百分比
	PriceChange4h     decimal.Decimal // 4小时价格变化百分比
	CurrentEMA20      decimal.Decimal
	CurrentMACD       decimal.Decimal
	CurrentRSI7       decimal.Decimal
	OpenInterest      *OIData
	FundingRate       decimal.Decimal
	IntradaySeries    *IntradayData
	LongerTermContext *LongerTermData
}

// OIData Open Interest数据
type OIData struct {
	Latest  decimal.Decimal
	Average decimal.Decimal
}

// IntradayData 日内数据(3分钟间隔)
type IntradayData struct {
	MidPrices   []decimal.Decimal
	EMA20Values []decimal.Decimal
	MACDValues  []decimal.Decimal
	RSI7Values  []decimal.Decimal
	RSI14Values []decimal.Decimal
}

// LongerTermData 长期数据(4小时时间框架)
type LongerTermData struct {
	EMA20         decimal.Decimal
	EMA50         decimal.Decimal
	ATR3          decimal.Decimal
	ATR14         decimal.Decimal
	CurrentVolume decimal.Decimal
	AverageVolume decimal.Decimal
	MACDValues    []decimal.Decimal
	RSI14Values   []decimal.Decimal
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
	OpenTime            int64           `json:"openTime"`
	Open                decimal.Decimal `json:"open"`
	High                decimal.Decimal `json:"high"`
	Low                 decimal.Decimal `json:"low"`
	Close               decimal.Decimal `json:"close"`
	Volume              decimal.Decimal `json:"volume"`
	CloseTime           int64           `json:"closeTime"`
	QuoteVolume         decimal.Decimal `json:"quoteVolume"`
	Trades              int             `json:"trades"`
	TakerBuyBaseVolume  decimal.Decimal `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume decimal.Decimal `json:"takerBuyQuoteVolume"`
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
	Symbol           string          `json:"symbol"`
	Timestamp        time.Time       `json:"timestamp"`
	Price            decimal.Decimal `json:"price"`
	PriceChange15Min decimal.Decimal `json:"price_change_15min"`
	PriceChange1H    decimal.Decimal `json:"price_change_1h"`
	PriceChange4H    decimal.Decimal `json:"price_change_4h"`
	Volume           decimal.Decimal `json:"volume"`
	VolumeRatio5     decimal.Decimal `json:"volume_ratio_5"`
	VolumeRatio20    decimal.Decimal `json:"volume_ratio_20"`
	VolumeTrend      decimal.Decimal `json:"volume_trend"`
	RSI14            decimal.Decimal `json:"rsi_14"`
	SMA5             decimal.Decimal `json:"sma_5"`
	SMA10            decimal.Decimal `json:"sma_10"`
	SMA20            decimal.Decimal `json:"sma_20"`
	HighLowRatio     decimal.Decimal `json:"high_low_ratio"`
	Volatility20     decimal.Decimal `json:"volatility_20"`
	PositionInRange  decimal.Decimal `json:"position_in_range"`
}

// 警报数据结构
type Alert struct {
	Type      string          `json:"type"`
	Symbol    string          `json:"symbol"`
	Value     decimal.Decimal `json:"value"`
	Threshold decimal.Decimal `json:"threshold"`
	Message   string          `json:"message"`
	Timestamp time.Time       `json:"timestamp"`
}

type Config struct {
	AlertThresholds AlertThresholds `json:"alert_thresholds"`
	UpdateInterval  int             `json:"update_interval"` // seconds
	CleanupConfig   CleanupConfig   `json:"cleanup_config"`
}

type AlertThresholds struct {
	VolumeSpike      decimal.Decimal `json:"volume_spike"`
	PriceChange15Min decimal.Decimal `json:"price_change_15min"`
	VolumeTrend      decimal.Decimal `json:"volume_trend"`
	RSIOverbought    decimal.Decimal `json:"rsi_overbought"`
	RSIOversold      decimal.Decimal `json:"rsi_oversold"`
}
type CleanupConfig struct {
	InactiveTimeout   time.Duration   `json:"inactive_timeout"`    // 不活跃超时时间
	MinScoreThreshold decimal.Decimal `json:"min_score_threshold"` // 最低评分阈值
	NoAlertTimeout    time.Duration   `json:"no_alert_timeout"`    // 无警报超时时间
	CheckInterval     time.Duration   `json:"check_interval"`      // 检查间隔
}

var config = Config{
	AlertThresholds: AlertThresholds{
		VolumeSpike:      decimal.NewFromFloat(3.0),
		PriceChange15Min: decimal.NewFromFloat(0.05),
		VolumeTrend:      decimal.NewFromFloat(2.0),
		RSIOverbought:    decimal.NewFromFloat(70),
		RSIOversold:      decimal.NewFromFloat(30),
	},
	CleanupConfig: CleanupConfig{
		InactiveTimeout:   30 * time.Minute,
		MinScoreThreshold: decimal.NewFromFloat(15.0),
		NoAlertTimeout:    20 * time.Minute,
		CheckInterval:     5 * time.Minute,
	},
	UpdateInterval: 60, // 1 minute
}
