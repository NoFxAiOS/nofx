package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"time"
)

// EnvironmentAnalysis 市场环境分析
type EnvironmentAnalysis struct {
	MarketTrend      string  `json:"market_trend"`      // "bull", "bear", "sideways"
	VolatilityLevel  string  `json:"volatility_level"`  // "high", "medium", "low"
	BTCCorrelation   float64 `json:"btc_correlation"`   // 与BTC的相关性
	RiskLevel        string  `json:"risk_level"`        // "high", "medium", "low"
	TradingMode      string  `json:"trading_mode"`      // "aggressive", "normal", "conservative"
	FearGreedIndex   int     `json:"fear_greed_index"`  // 恐惧贪婪指数 (0-100)
	FearGreedLevel   string  `json:"fear_greed_level"`  // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
}

// FearGreedResponse 恐惧贪婪指数API响应
type FearGreedResponse struct {
	Name string `json:"name"`
	Data []struct {
		Value               string `json:"value"`
		ValueClassification string `json:"value_classification"`
		Timestamp           string `json:"timestamp"`
		TimeUntilUpdate     string `json:"time_until_update"`
	} `json:"data"`
	Metadata struct {
		Error interface{} `json:"error"`
	} `json:"metadata"`
}

// AnalyzeEnvironment 分析市场环境（基于现有数据）
func AnalyzeEnvironment(symbols []string) (*EnvironmentAnalysis, error) {
	// 1. 获取BTC数据作为市场基准
	btcData, err := Get("BTCUSDT")
	if err != nil {
		return nil, fmt.Errorf("获取BTC数据失败: %w", err)
	}

	// 2. 获取恐惧贪婪指数
	fearGreedIndex, fearGreedLevel := getFearGreedIndex()
	
	// 3. 分析市场趋势（基于BTC + 恐惧贪婪指数）
	marketTrend := analyzeMarketTrendWithSentiment(btcData, fearGreedIndex)
	
	// 4. 分析波动率环境
	volatilityLevel := analyzeVolatilityLevel(btcData)
	
	// 5. 计算平均相关性（简化版）
	avgCorrelation := calculateAverageCorrelation(symbols, btcData)
	
	// 6. 综合评估风险等级（包含情绪指标）
	riskLevel := assessRiskLevelWithSentiment(marketTrend, volatilityLevel, avgCorrelation, fearGreedIndex)
	
	// 7. 推荐交易模式（基于情绪调整）
	tradingMode := recommendTradingModeWithSentiment(riskLevel, volatilityLevel, fearGreedIndex)

	return &EnvironmentAnalysis{
		MarketTrend:     marketTrend,
		VolatilityLevel: volatilityLevel,
		BTCCorrelation:  avgCorrelation,
		RiskLevel:       riskLevel,
		TradingMode:     tradingMode,
		FearGreedIndex:  fearGreedIndex,
		FearGreedLevel:  fearGreedLevel,
	}, nil
}

// getFearGreedIndex 获取恐惧贪婪指数
func getFearGreedIndex() (int, string) {
	client := http.Client{
		Timeout: 5 * time.Second, // 5秒超时
	}
	
	resp, err := client.Get("https://api.alternative.me/fng/")
	if err != nil {
		// API失败时返回中性值
		return 50, "Neutral"
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 50, "Neutral"
	}

	var fgResp FearGreedResponse
	if err := json.Unmarshal(body, &fgResp); err != nil {
		return 50, "Neutral"
	}

	if len(fgResp.Data) == 0 {
		return 50, "Neutral"
	}

	value, err := strconv.Atoi(fgResp.Data[0].Value)
	if err != nil {
		return 50, "Neutral"
	}

	classification := fgResp.Data[0].ValueClassification
	return value, classification
}

// analyzeMarketTrendWithSentiment 分析市场趋势（基于BTC指标 + 恐惧贪婪指数）
func analyzeMarketTrendWithSentiment(btcData *Data, fearGreedIndex int) string {
	// 基于多个时间框架判断
	price1h := btcData.PriceChange1h
	price4h := btcData.PriceChange4h
	emaSignal := btcData.CurrentPrice > btcData.CurrentEMA20
	macdSignal := btcData.CurrentMACD > 0
	
	bullishSignals := 0
	if price1h > 1 { bullishSignals++ }      // 1小时涨幅>1%
	if price4h > 2 { bullishSignals++ }      // 4小时涨幅>2%  
	if emaSignal { bullishSignals++ }        // 价格在EMA20上方
	if macdSignal { bullishSignals++ }       // MACD为正
	
	// 恐惧贪婪指数调整 (强化情绪影响)
	if fearGreedIndex >= 75 { bullishSignals++ }      // 极度贪婪 → 牛市信号
	if fearGreedIndex <= 25 { bullishSignals-- }      // 极度恐惧 → 熊市信号
	
	// 防止负数
	if bullishSignals < 0 { bullishSignals = 0 }
	
	if bullishSignals >= 3 {
		return "bull"
	} else if bullishSignals <= 1 {
		return "bear"  
	} else {
		return "sideways"
	}
}

// analyzeMarketTrend 兼容旧版本（已弃用，使用上面的新函数）
func analyzeMarketTrend(btcData *Data) string {
	return analyzeMarketTrendWithSentiment(btcData, 50) // 默认中性情绪
}

// analyzeVolatilityLevel 分析波动率水平
func analyzeVolatilityLevel(btcData *Data) string {
	// 基于ATR和价格变化
	atr := btcData.LongerTermContext.ATR14
	priceVolatility := math.Abs(btcData.PriceChange4h)
	
	// 计算相对波动率
	relativeVolatility := (atr / btcData.CurrentPrice) * 100
	
	if relativeVolatility > 3 || priceVolatility > 5 {
		return "high"
	} else if relativeVolatility < 1.5 && priceVolatility < 2 {
		return "low"
	} else {
		return "medium"
	}
}

// calculateAverageCorrelation 计算与BTC的平均相关性（简化版）
func calculateAverageCorrelation(symbols []string, btcData *Data) float64 {
	// 简化实现：基于价格变化方向的相关性
	totalCorrelation := 0.0
	validCount := 0
	
	for _, symbol := range symbols {
		if symbol == "BTCUSDT" {
			continue
		}
		
		data, err := Get(symbol)
		if err != nil {
			continue
		}
		
		// 简单相关性：同向变化为正相关
		correlation := 0.0
		if (btcData.PriceChange1h > 0 && data.PriceChange1h > 0) ||
		   (btcData.PriceChange1h < 0 && data.PriceChange1h < 0) {
			correlation += 0.5
		}
		if (btcData.PriceChange4h > 0 && data.PriceChange4h > 0) ||
		   (btcData.PriceChange4h < 0 && data.PriceChange4h < 0) {
			correlation += 0.5
		}
		
		totalCorrelation += correlation
		validCount++
	}
	
	if validCount == 0 {
		return 0.5 // 默认中等相关性
	}
	
	return totalCorrelation / float64(validCount)
}

// assessRiskLevelWithSentiment 评估整体风险等级（包含情绪指标）
func assessRiskLevelWithSentiment(trend, volatility string, correlation float64, fearGreedIndex int) string {
	riskScore := 0
	
	// 市场趋势风险
	if trend == "bear" {
		riskScore += 2
	} else if trend == "sideways" {
		riskScore += 1
	}
	
	// 波动率风险
	if volatility == "high" {
		riskScore += 2
	} else if volatility == "medium" {
		riskScore += 1
	}
	
	// 相关性风险
	if correlation > 0.8 {
		riskScore += 1 // 高相关性增加系统风险
	}
	
	// 恐惧贪婪指数风险调整
	if fearGreedIndex >= 80 {
		riskScore += 2 // 极度贪婪 = 高风险（市场过热）
	} else if fearGreedIndex <= 20 {
		riskScore += 1 // 极度恐惧 = 中等风险（恐慌性抛售）
	}
	
	if riskScore >= 5 {
		return "high"
	} else if riskScore >= 3 {
		return "medium"
	} else {
		return "low"
	}
}

// recommendTradingModeWithSentiment 推荐交易模式（基于情绪调整）
func recommendTradingModeWithSentiment(riskLevel, volatilityLevel string, fearGreedIndex int) string {
	// 基础模式判断
	baseMode := "normal"
	if riskLevel == "high" {
		baseMode = "conservative"
	} else if riskLevel == "low" && volatilityLevel == "high" {
		baseMode = "aggressive"
	}
	
	// 恐惧贪婪指数微调
	if fearGreedIndex <= 15 {
		// 极度恐惧时，反向机会出现
		if baseMode == "conservative" {
			return "normal" // 从保守调整为正常
		}
		return "aggressive" // 其他情况更积极
	} else if fearGreedIndex >= 85 {
		// 极度贪婪时，需要更谨慎
		if baseMode == "aggressive" {
			return "normal" // 从激进调整为正常
		}
		return "conservative" // 其他情况更保守
	}
	
	return baseMode
}

// 兼容旧版本函数
func assessRiskLevel(trend, volatility string, correlation float64) string {
	return assessRiskLevelWithSentiment(trend, volatility, correlation, 50)
}

func recommendTradingMode(riskLevel, volatilityLevel string) string {
	return recommendTradingModeWithSentiment(riskLevel, volatilityLevel, 50)
}