package market

import "time"

// Data market data structure
type Data struct {
	Symbol            string
	CurrentPrice      float64
	PriceChange1h     float64 // 1-hour price change percentage
	PriceChange4h     float64 // 4-hour price change percentage
	CurrentEMA20      float64
	CurrentMACD       float64
	CurrentRSI7       float64
	OpenInterest      *OIData
	FundingRate       float64
	IntradaySeries    *IntradayData
	LongerTermContext *LongerTermData
	// Multi-timeframe data (new)
	TimeframeData map[string]*TimeframeSeriesData `json:"timeframe_data,omitempty"`
}

// KlineBar single kline bar with OHLCV data
type KlineBar struct {
	Time   int64   `json:"time"`   // Unix timestamp in milliseconds
	Open   float64 `json:"open"`   // Open price
	High   float64 `json:"high"`   // High price
	Low    float64 `json:"low"`    // Low price
	Close  float64 `json:"close"`  // Close price
	Volume float64 `json:"volume"` // Volume
}

// TimeframeSeriesData series data for a single timeframe
type TimeframeSeriesData struct {
	Timeframe   string     `json:"timeframe"`    // Timeframe identifier, e.g. "5m", "15m", "1h"
	Klines      []KlineBar `json:"klines"`       // Full OHLCV kline data
	MidPrices   []float64  `json:"mid_prices"`   // Price series (deprecated, kept for compatibility)
	EMA20Values []float64  `json:"ema20_values"` // EMA20 series
	EMA50Values []float64  `json:"ema50_values"` // EMA50 series
	MACDValues  []float64  `json:"macd_values"`  // MACD series
	RSI7Values  []float64  `json:"rsi7_values"`  // RSI7 series
	RSI14Values []float64  `json:"rsi14_values"` // RSI14 series
	Volume      []float64  `json:"volume"`       // Volume series (deprecated, use Klines)
	ATR14       float64    `json:"atr14"`        // ATR14
	// Bollinger Bands (period 20, std dev multiplier 2)
	BOLLUpper  []float64 `json:"boll_upper"`  // Upper band
	BOLLMiddle []float64 `json:"boll_middle"` // Middle band (SMA)
	BOLLLower  []float64 `json:"boll_lower"`  // Lower band
}

// OIData Open Interest data
type OIData struct {
	Latest  float64
	Average float64
}

// IntradayData intraday data (3-minute interval)
type IntradayData struct {
	MidPrices   []float64
	EMA20Values []float64
	MACDValues  []float64
	RSI7Values  []float64
	RSI14Values []float64
	Volume      []float64
	ATR14       float64
}

// LongerTermData longer-term data (4-hour timeframe)
type LongerTermData struct {
	EMA20         float64
	EMA50         float64
	ATR3          float64
	ATR14         float64
	CurrentVolume float64
	AverageVolume float64
	MACDValues    []float64
	RSI14Values   []float64
}

// Binance API response structure
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

// SymbolFeatures feature data structure
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

// Alert alert data structure
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
	InactiveTimeout   time.Duration `json:"inactive_timeout"`    // Inactive timeout duration
	MinScoreThreshold float64       `json:"min_score_threshold"` // Minimum score threshold
	NoAlertTimeout    time.Duration `json:"no_alert_timeout"`    // No alert timeout duration
	CheckInterval     time.Duration `json:"check_interval"`      // Check interval
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

// BoxData represents multi-period Donchian channel (box) data
type BoxData struct {
	// Short-term box (72 1h candles = 3 days)
	ShortUpper float64 `json:"short_upper"`
	ShortLower float64 `json:"short_lower"`

	// Mid-term box (240 1h candles = 10 days)
	MidUpper float64 `json:"mid_upper"`
	MidLower float64 `json:"mid_lower"`

	// Long-term box (500 1h candles = ~21 days)
	LongUpper float64 `json:"long_upper"`
	LongLower float64 `json:"long_lower"`

	// Current price position relative to boxes
	CurrentPrice float64 `json:"current_price"`
}

// RegimeLevel represents the ranging classification level
type RegimeLevel string

const (
	RegimeLevelNarrow   RegimeLevel = "narrow"   // 窄幅震荡
	RegimeLevelStandard RegimeLevel = "standard" // 标准震荡
	RegimeLevelWide     RegimeLevel = "wide"     // 宽幅震荡
	RegimeLevelVolatile RegimeLevel = "volatile" // 剧烈震荡
	RegimeLevelTrending RegimeLevel = "trending" // 趋势
)

// BreakoutLevel represents which box level has been broken
type BreakoutLevel string

const (
	BreakoutNone  BreakoutLevel = "none"
	BreakoutShort BreakoutLevel = "short"
	BreakoutMid   BreakoutLevel = "mid"
	BreakoutLong  BreakoutLevel = "long"
)

// GridDirection represents the current grid trading direction bias
type GridDirection string

const (
	GridDirectionNeutral   GridDirection = "neutral"     // 50% buy + 50% sell
	GridDirectionLong      GridDirection = "long"        // 100% buy
	GridDirectionShort     GridDirection = "short"       // 100% sell
	GridDirectionLongBias  GridDirection = "long_bias"   // 70% buy + 30% sell (default)
	GridDirectionShortBias GridDirection = "short_bias"  // 30% buy + 70% sell (default)
)

// GetBuySellRatio returns the buy and sell ratio for this direction
// biasRatio is the ratio for biased directions (default 0.7 means 70%/30%)
func (d GridDirection) GetBuySellRatio(biasRatio float64) (buyRatio, sellRatio float64) {
	if biasRatio <= 0 || biasRatio > 1 {
		biasRatio = 0.7 // Default 70%/30%
	}

	switch d {
	case GridDirectionNeutral:
		return 0.5, 0.5
	case GridDirectionLong:
		return 1.0, 0.0
	case GridDirectionShort:
		return 0.0, 1.0
	case GridDirectionLongBias:
		return biasRatio, 1.0 - biasRatio
	case GridDirectionShortBias:
		return 1.0 - biasRatio, biasRatio
	default:
		return 0.5, 0.5
	}
}
