package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// FundingRateCache 资金费率缓存结构
// Binance Funding Rate 每 8 小时才更新一次，使用 1 小时缓存可显著减少 API 调用
type FundingRateCache struct {
	Rate      float64
	UpdatedAt time.Time
}

var (
	fundingRateMap sync.Map // map[string]*FundingRateCache
	frCacheTTL     = 1 * time.Hour
)

// GetTimeframeName 将时间框架代码转换为可读名称
func GetTimeframeName(tf string) string {
	names := map[string]string{
		"1m":  "1-minute",
		"3m":  "3-minute",
		"5m":  "5-minute",
		"15m": "15-minute",
		"30m": "30-minute",
		"1h":  "1-hour",
		"2h":  "2-hour",
		"4h":  "4-hour",
		"6h":  "6-hour",
		"12h": "12-hour",
		"1d":  "1-day",
	}
	if name, ok := names[tf]; ok {
		return name
	}
	return tf
}

// GetDefaultDataPoints 返回时间框架的默认数据点数
func GetDefaultDataPoints(tf string) int {
	defaults := map[string]int{
		"1m":  40, // ~40 分钟
		"3m":  40, // ~2 小时
		"5m":  40, // ~3.3 小时
		"15m": 40, // ~10 小时
		"30m": 30, // ~15 小时
		"1h":  30, // ~30 小时
		"2h":  25, // ~2 天
		"4h":  25, // ~4 天
		"6h":  20, // ~5 天
		"12h": 20, // ~10 天
		"1d":  15, // ~2 周
	}
	if points, ok := defaults[tf]; ok {
		return points
	}
	return 30 // 默认值
}

// ValidateTimeframe 验证时间框架是否支持
func ValidateTimeframe(tf string) bool {
	supported := map[string]bool{
		"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
		"1h": true, "2h": true, "4h": true, "6h": true, "12h": true, "1d": true,
	}
	return supported[tf]
}

// calculateTimeframeData 计算单个时间框架的所有指标数据
func calculateTimeframeData(klines []Kline, timeframe string, dataPoints int) *TimeframeData {
	if len(klines) == 0 {
		return nil
	}

	// 限制数据点数量不超过实际K线数量
	if dataPoints > len(klines) {
		dataPoints = len(klines)
	}
	if dataPoints <= 0 {
		dataPoints = GetDefaultDataPoints(timeframe)
	}

	// 计算所需的指标数组
	midPrices := make([]float64, 0, dataPoints)
	ema20Values := make([]float64, 0, dataPoints)
	macdValues := make([]float64, 0, dataPoints)
	rsi7Values := make([]float64, 0, dataPoints)
	rsi14Values := make([]float64, 0, dataPoints)
	bollingerUpper := make([]float64, 0, dataPoints)
	bollingerMid := make([]float64, 0, dataPoints)
	bollingerLower := make([]float64, 0, dataPoints)
	volumes := make([]float64, 0, dataPoints)

	// 从最后dataPoints个K线开始计算
	startIdx := len(klines) - dataPoints
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(klines); i++ {
		// Mid价格 (High + Low) / 2
		midPrice := (klines[i].High + klines[i].Low) / 2
		midPrices = append(midPrices, midPrice)

		// 成交量
		volumes = append(volumes, klines[i].Volume)

		// 计算EMA20 (需要前20个K线)
		ema20 := 0.0
		if i >= 19 {
			ema20 = calculateEMA(klines[:i+1], 20)
		}
		ema20Values = append(ema20Values, ema20)

		// 计算MACD (需要足够的历史数据)
		macd := 0.0
		if i >= 33 { // MACD需要至少34个数据点
			macd = calculateMACD(klines[:i+1])
		}
		macdValues = append(macdValues, macd)

		// 计算RSI7
		rsi7 := 0.0
		if i >= 7 {
			rsi7 = calculateRSI(klines[:i+1], 7)
		}
		rsi7Values = append(rsi7Values, rsi7)

		// 计算RSI14
		rsi14 := 0.0
		if i >= 14 {
			rsi14 = calculateRSI(klines[:i+1], 14)
		}
		rsi14Values = append(rsi14Values, rsi14)

		// 计算布林带
		upper, mid, lower := 0.0, 0.0, 0.0
		if i >= 19 { // 布林带使用20周期
			upper, mid, lower = calculateBollingerBands(klines[:i+1], 20, 2.0)
		}
		bollingerUpper = append(bollingerUpper, upper)
		bollingerMid = append(bollingerMid, mid)
		bollingerLower = append(bollingerLower, lower)
	}

	// 计算ATR14 (使用所有可用数据)
	atr14 := 0.0
	if len(klines) >= 14 {
		atr14 = calculateATR(klines, 14)
	}

	return &TimeframeData{
		Timeframe:      timeframe,
		DataPoints:     len(midPrices),
		MidPrices:      midPrices,
		EMA20Values:    ema20Values,
		MACDValues:     macdValues,
		RSI7Values:     rsi7Values,
		RSI14Values:    rsi14Values,
		BollingerUpper: bollingerUpper,
		BollingerMid:   bollingerMid,
		BollingerLower: bollingerLower,
		Volume:         volumes,
		ATR14:          atr14,
	}
}

// Get 获取指定代币的市场数据
func Get(symbol string, config ...*IndicatorConfig) (*Data, error) {
	// 使用默认配置或传入的配置
	var cfg *IndicatorConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = GetDefaultIndicatorConfig()
	}

	// 标准化symbol
	symbol = Normalize(symbol)

	// 如果没有配置timeframes,使用默认值 [3m, 4h]
	timeframes := cfg.Timeframes
	if len(timeframes) == 0 {
		timeframes = []string{"3m", "4h"}
	}

	// 验证所有时间框架
	validTimeframes := make([]string, 0, len(timeframes))
	for _, tf := range timeframes {
		if ValidateTimeframe(tf) {
			validTimeframes = append(validTimeframes, tf)
		} else {
			// 记录警告但继续处理其他时间框架
			fmt.Printf("警告: 不支持的时间框架 %s,已跳过\n", tf)
		}
	}

	if len(validTimeframes) == 0 {
		return nil, fmt.Errorf("没有有效的时间框架配置")
	}

	// Data staleness detection: Get 3m klines first for staleness check
	// This prevents trading on frozen/outdated market data (e.g., DOGEUSDT issue)
	klines3m, err := WSMonitorCli.GetCurrentKlines(symbol, "3m")
	if err != nil {
		return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
	}

	if isStaleData(klines3m, symbol) {
		log.Printf("⚠️  WARNING: %s detected stale data (consecutive price freeze), skipping symbol", symbol)
		return nil, fmt.Errorf("%s data is stale, possible cache failure", symbol)
	}

	// 使用 errgroup 并发获取多个时间框架的数据
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// 创建结果存储 (线程安全)
	var mu sync.Mutex
	klinesMap := make(map[string][]Kline)

	// Pre-fill 3m klines since we already fetched it for staleness check
	klinesMap["3m"] = klines3m

	// 限制并发数为5
	semaphore := make(chan struct{}, 5)

	// 并发获取所有时间框架的K线数据 (except 3m which is already fetched)
	for _, tf := range validTimeframes {
		if tf == "3m" {
			continue // Skip 3m since we already have it
		}
		timeframe := tf // 捕获循环变量
		g.Go(func() error {
			// 获取信号量
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return ctx.Err()
			}

			// 获取K线数据,带超时控制
			klines, err := WSMonitorCli.GetCurrentKlines(symbol, timeframe)
			if err != nil {
				return fmt.Errorf("获取%s K线失败: %v", timeframe, err)
			}

			if len(klines) == 0 {
				return fmt.Errorf("%s K线数据为空", timeframe)
			}

			// 安全地存储结果
			mu.Lock()
			klinesMap[timeframe] = klines
			mu.Unlock()

			return nil
		})
	}

	// 等待所有goroutine完成
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %v", err)
	}

	// 计算每个时间框架的指标数据
	timeframeDataMap := make(map[string]*TimeframeData)
	for tf, klines := range klinesMap {
		dataPoints := cfg.DataPoints[tf]
		if dataPoints == 0 {
			dataPoints = GetDefaultDataPoints(tf)
		}

		tfData := calculateTimeframeData(klines, tf, dataPoints)
		if tfData != nil {
			timeframeDataMap[tf] = tfData
		}
	}

	// 计算当前价格和主要指标 (使用最小时间框架的最新数据)
	var currentPrice, currentEMA20, currentMACD, currentRSI7 float64
	var priceChange1h, priceChange4h float64

	// 使用3m数据计算当前指标 (如果有的话)
	if klines3m, ok := klinesMap["3m"]; ok && len(klines3m) > 0 {
		currentPrice = klines3m[len(klines3m)-1].Close
		currentEMA20 = calculateEMA(klines3m, 20)
		currentMACD = calculateMACD(klines3m)
		currentRSI7 = calculateRSI(klines3m, 7)

		// 1小时价格变化 = 20个3分钟K线前的价格
		if len(klines3m) >= 21 {
			price1hAgo := klines3m[len(klines3m)-21].Close
			if price1hAgo > 0 {
				priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
			}
		}
	} else if len(klinesMap) > 0 {
		// 如果没有3m数据,使用第一个可用时间框架的数据
		for _, klines := range klinesMap {
			if len(klines) > 0 {
				currentPrice = klines[len(klines)-1].Close
				currentEMA20 = calculateEMA(klines, 20)
				currentMACD = calculateMACD(klines)
				currentRSI7 = calculateRSI(klines, 7)
				break
			}
		}
	}

	// 使用4h数据计算4小时价格变化
	if klines4h, ok := klinesMap["4h"]; ok && len(klines4h) >= 2 {
		if currentPrice == 0 && len(klines4h) > 0 {
			currentPrice = klines4h[len(klines4h)-1].Close
		}
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 获取OI数据
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		// OI失败不影响整体,使用默认值
		oiData = &OIData{Latest: 0, Average: 0}
	}

	// 获取Funding Rate
	fundingRate, _ := getFundingRate(symbol)

	// 为向后兼容性保留旧字段
	var intradayData *IntradayData
	var longerTermData *LongerTermData

	if klines3m, ok := klinesMap["3m"]; ok && len(klines3m) > 0 {
		dataPoints3m := cfg.DataPoints["3m"]
		if dataPoints3m == 0 {
			dataPoints3m = 40
		}
		intradayData = calculateIntradaySeries(klines3m, dataPoints3m)
	}

	if klines4h, ok := klinesMap["4h"]; ok && len(klines4h) > 0 {
		dataPoints4h := cfg.DataPoints["4h"]
		if dataPoints4h == 0 {
			dataPoints4h = 25
		}
		longerTermData = calculateLongerTermData(klines4h, dataPoints4h)
	}

	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentRSI7:       currentRSI7,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		TimeframeData:     timeframeDataMap,
		IntradaySeries:    intradayData,
		LongerTermContext: longerTermData,
	}, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	// 计算12期和26期EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12 - ema26
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateBollingerBands 计算布林带
// 返回值顺序：upper, middle, lower
func calculateBollingerBands(klines []Kline, period int, numStdDev float64) (float64, float64, float64) {
	if len(klines) < period {
		return 0, 0, 0
	}

	// 计算中轨（SMA）
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	middle := sum / float64(period)

	// 计算标准差
	variance := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		diff := klines[i].Close - middle
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(period))

	// 计算上轨和下轨
	upper := middle + numStdDev*stdDev
	lower := middle - numStdDev*stdDev

	return upper, middle, lower
}

// calculateIntradaySeries 计算日内系列数据
func calculateIntradaySeries(klines []Kline, dataPoints ...int) *IntradayData {
	// 默认返回40个数据点
	numPoints := 40
	if len(dataPoints) > 0 && dataPoints[0] > 0 {
		numPoints = dataPoints[0]
	}

	data := &IntradayData{
		MidPrices:      make([]float64, 0, numPoints),
		EMA20Values:    make([]float64, 0, numPoints),
		MACDValues:     make([]float64, 0, numPoints),
		RSI7Values:     make([]float64, 0, numPoints),
		RSI14Values:    make([]float64, 0, numPoints),
		Volume:         make([]float64, 0, numPoints),
		BollingerUpper: make([]float64, 0, numPoints),
		BollingerMid:   make([]float64, 0, numPoints),
		BollingerLower: make([]float64, 0, numPoints),
	}

	// 获取最近N个数据点
	start := len(klines) - numPoints
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)
		data.Volume = append(data.Volume, klines[i].Volume)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的MACD
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// 计算每个点的RSI
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}

		// 计算布林带（20周期，2倍标准差）
		if i >= 19 {
			upper, middle, lower := calculateBollingerBands(klines[:i+1], 20, 2.0)
			data.BollingerUpper = append(data.BollingerUpper, upper)
			data.BollingerMid = append(data.BollingerMid, middle)
			data.BollingerLower = append(data.BollingerLower, lower)
		}
	}

	// 计算3m ATR14
	data.ATR14 = calculateATR(klines, 14)

	return data
}

// calculateLongerTermData 计算长期数据
func calculateLongerTermData(klines []Kline, dataPoints ...int) *LongerTermData {
	// 默认返回25个数据点
	numPoints := 25
	if len(dataPoints) > 0 && dataPoints[0] > 0 {
		numPoints = dataPoints[0]
	}

	data := &LongerTermData{
		MACDValues:     make([]float64, 0, numPoints),
		RSI14Values:    make([]float64, 0, numPoints),
		BollingerUpper: make([]float64, 0, numPoints),
		BollingerMid:   make([]float64, 0, numPoints),
		BollingerLower: make([]float64, 0, numPoints),
	}

	// 计算EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// 计算ATR
	data.ATR3 = calculateATR(klines, 3)
	data.ATR14 = calculateATR(klines, 14)

	// 计算成交量
	if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		// 计算平均成交量
		sum := 0.0
		for _, k := range klines {
			sum += k.Volume
		}
		data.AverageVolume = sum / float64(len(klines))
	}

	// 计算MACD和RSI序列
	start := len(klines) - numPoints
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
		// 计算布林带（20周期，2倍标准差）
		if i >= 19 {
			upper, middle, lower := calculateBollingerBands(klines[:i+1], 20, 2.0)
			data.BollingerUpper = append(data.BollingerUpper, upper)
			data.BollingerMid = append(data.BollingerMid, middle)
			data.BollingerLower = append(data.BollingerLower, lower)
		}
	}

	return data
}

// getOpenInterestData 获取OI数据
func getOpenInterestData(symbol string) (*OIData, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	apiClient := NewAPIClient()
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // 近似平均值
	}, nil
}

// getFundingRate 获取资金费率（优化：使用 1 小时缓存）
func getFundingRate(symbol string) (float64, error) {
	// 检查缓存（有效期 1 小时）
	// Funding Rate 每 8 小时才更新，1 小时缓存非常合理
	if cached, ok := fundingRateMap.Load(symbol); ok {
		cache := cached.(*FundingRateCache)
		if time.Since(cache.UpdatedAt) < frCacheTTL {
			// 缓存命中，直接返回
			return cache.Rate, nil
		}
	}

	// 缓存过期或不存在，调用 API
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	apiClient := NewAPIClient()
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)

	// 更新缓存
	fundingRateMap.Store(symbol, &FundingRateCache{
		Rate:      rate,
		UpdatedAt: time.Now(),
	})

	return rate, nil
}

// getSortedTimeframes 返回按标准顺序排序的时间框架列表
func getSortedTimeframes(timeframeMap map[string]*TimeframeData) []string {
	// 定义标准顺序
	order := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "12h", "1d"}

	var result []string
	for _, tf := range order {
		if _, exists := timeframeMap[tf]; exists {
			result = append(result, tf)
		}
	}

	return result
}

// formatTimeframeData 格式化单个时间框架的数据
func formatTimeframeData(tf *TimeframeData) string {
	if tf == nil {
		return ""
	}

	var sb strings.Builder

	// 获取时间框架的友好名称
	tfName := GetTimeframeName(tf.Timeframe)
	sb.WriteString(fmt.Sprintf("Time series data (%s timeframe, oldest → latest):\n\n", tfName))

	// Mid prices
	if len(tf.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(tf.MidPrices)))
	}

	// EMA20
	if len(tf.EMA20Values) > 0 {
		sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(tf.EMA20Values)))
	}

	// MACD
	if len(tf.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(tf.MACDValues)))
	}

	// RSI7
	if len(tf.RSI7Values) > 0 {
		sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(tf.RSI7Values)))
	}

	// RSI14
	if len(tf.RSI14Values) > 0 {
		sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(tf.RSI14Values)))
	}

	// Bollinger Bands
	if len(tf.BollingerUpper) > 0 {
		sb.WriteString(fmt.Sprintf("Bollinger Bands (20‑period, 2σ):\n"))
		sb.WriteString(fmt.Sprintf("  Upper: %s\n", formatFloatSlice(tf.BollingerUpper)))
		sb.WriteString(fmt.Sprintf("  Middle: %s\n", formatFloatSlice(tf.BollingerMid)))
		sb.WriteString(fmt.Sprintf("  Lower: %s\n\n", formatFloatSlice(tf.BollingerLower)))
	}

	// Volume
	if len(tf.Volume) > 0 {
		sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(tf.Volume)))
	}

	// ATR14
	if tf.ATR14 > 0 {
		sb.WriteString(fmt.Sprintf("%s ATR (14‑period): %.3f\n\n", tf.Timeframe, tf.ATR14))
	}

	return sb.String()
}

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	// 使用动态精度格式化价格
	priceStr := formatPriceWithDynamicPrecision(data.CurrentPrice)
	sb.WriteString(fmt.Sprintf("current_price = %s, current_ema20 = %.3f, current_macd = %.3f, current_rsi (7 period) = %.3f\n\n",
		priceStr, data.CurrentEMA20, data.CurrentMACD, data.CurrentRSI7))

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		// 使用动态精度格式化 OI 数据
		oiLatestStr := formatPriceWithDynamicPrecision(data.OpenInterest.Latest)
		oiAverageStr := formatPriceWithDynamicPrecision(data.OpenInterest.Average)
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %s Average: %s\n\n",
			oiLatestStr, oiAverageStr))
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))

	// 优先使用新的 TimeframeData（动态多时间框架）
	if data.TimeframeData != nil && len(data.TimeframeData) > 0 {
		// 按标准顺序输出时间框架数据
		sortedTimeframes := getSortedTimeframes(data.TimeframeData)
		for _, tf := range sortedTimeframes {
			tfData := data.TimeframeData[tf]
			sb.WriteString(formatTimeframeData(tfData))
		}
	} else {
		// 向后兼容：如果没有 TimeframeData，使用旧字段
		if data.IntradaySeries != nil {
			sb.WriteString("Intraday series (3‑minute intervals, oldest → latest):\n\n")

			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}

			if len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}

			if len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}

			if len(data.IntradaySeries.RSI7Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
			}

			if len(data.IntradaySeries.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
			}

			if len(data.IntradaySeries.BollingerUpper) > 0 {
				sb.WriteString(fmt.Sprintf("Bollinger Bands (20‑period, 2σ):\n"))
				sb.WriteString(fmt.Sprintf("  Upper: %s\n", formatFloatSlice(data.IntradaySeries.BollingerUpper)))
				sb.WriteString(fmt.Sprintf("  Middle: %s\n", formatFloatSlice(data.IntradaySeries.BollingerMid)))
				sb.WriteString(fmt.Sprintf("  Lower: %s\n\n", formatFloatSlice(data.IntradaySeries.BollingerLower)))
			}

			if len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}

			sb.WriteString(fmt.Sprintf("3m ATR (14‑period): %.3f\n\n", data.IntradaySeries.ATR14))
		}

		if data.LongerTermContext != nil {
			sb.WriteString("Longer‑term context (4‑hour timeframe):\n\n")

			sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
				data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))

			sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
				data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))

			sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
				data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))

			if len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}

			if len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}

			if len(data.LongerTermContext.BollingerUpper) > 0 {
				sb.WriteString(fmt.Sprintf("Bollinger Bands (20‑period, 2σ):\n"))
				sb.WriteString(fmt.Sprintf("  Upper: %s\n", formatFloatSlice(data.LongerTermContext.BollingerUpper)))
				sb.WriteString(fmt.Sprintf("  Middle: %s\n", formatFloatSlice(data.LongerTermContext.BollingerMid)))
				sb.WriteString(fmt.Sprintf("  Lower: %s\n\n", formatFloatSlice(data.LongerTermContext.BollingerLower)))
			}
		}
	}

	return sb.String()
}

// formatPriceWithDynamicPrecision 根据价格区间动态选择精度
// 这样可以完美支持从超低价 meme coin (< 0.0001) 到 BTC/ETH 的所有币种
func formatPriceWithDynamicPrecision(price float64) string {
	switch {
	case price < 0.0001:
		// 超低价 meme coin: 1000SATS, 1000WHY, DOGS
		// 0.00002070 → "0.00002070" (8位小数)
		return fmt.Sprintf("%.8f", price)
	case price < 0.001:
		// 低价 meme coin: NEIRO, HMSTR, HOT, NOT
		// 0.00015060 → "0.000151" (6位小数)
		return fmt.Sprintf("%.6f", price)
	case price < 0.01:
		// 中低价币: PEPE, SHIB, MEME
		// 0.00556800 → "0.005568" (6位小数)
		return fmt.Sprintf("%.6f", price)
	case price < 1.0:
		// 低价币: ASTER, DOGE, ADA, TRX
		// 0.9954 → "0.9954" (4位小数)
		return fmt.Sprintf("%.4f", price)
	case price < 100:
		// 中价币: SOL, AVAX, LINK, MATIC
		// 23.4567 → "23.4567" (4位小数)
		return fmt.Sprintf("%.4f", price)
	default:
		// 高价币: BTC, ETH (节省 Token)
		// 45678.9123 → "45678.91" (2位小数)
		return fmt.Sprintf("%.2f", price)
	}
}

// formatFloatSlice 格式化float64切片为字符串（使用动态精度）
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = formatPriceWithDynamicPrecision(v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// isStaleData detects stale data (consecutive price freeze)
// Fix DOGEUSDT-style issue: consecutive N periods with completely unchanged prices indicate data source anomaly
func isStaleData(klines []Kline, symbol string) bool {
	if len(klines) < 5 {
		return false // Insufficient data to determine
	}

	// Detection threshold: 5 consecutive 3-minute periods with unchanged price (15 minutes without fluctuation)
	const stalePriceThreshold = 5
	const priceTolerancePct = 0.0001 // 0.01% fluctuation tolerance (avoid false positives)

	// Take the last stalePriceThreshold K-lines
	recentKlines := klines[len(klines)-stalePriceThreshold:]
	firstPrice := recentKlines[0].Close

	// Check if all prices are within tolerance
	for i := 1; i < len(recentKlines); i++ {
		priceDiff := math.Abs(recentKlines[i].Close-firstPrice) / firstPrice
		if priceDiff > priceTolerancePct {
			return false // Price fluctuation exists, data is normal
		}
	}

	// Additional check: MACD and volume
	// If price is unchanged but MACD/volume shows normal fluctuation, it might be a real market situation (extremely low volatility)
	// Check if volume is also 0 (data completely frozen)
	allVolumeZero := true
	for _, k := range recentKlines {
		if k.Volume > 0 {
			allVolumeZero = false
			break
		}
	}

	if allVolumeZero {
		log.Printf("⚠️  %s stale data confirmed: price freeze + zero volume", symbol)
		return true
	}

	// Price frozen but has volume: might be extremely low volatility market, allow but log warning
	log.Printf("⚠️  %s detected extreme price stability (no fluctuation for %d consecutive periods), but volume is normal", symbol, stalePriceThreshold)
	return false
}
