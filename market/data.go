package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// Data 市场数据结构
type Data struct {
	Symbol            string
	CurrentPrice      float64
	PriceChange1h     float64            // 1小时价格变化百分比
	PriceChange4h     float64            // 4小时价格变化百分比
	CurrentIndicators map[string]float64 // 当前指标值，key 格式: "EMA_20", "RSI_7", "MACD" 等
	OpenInterest      *OIData
	FundingRate       float64
	IntradaySeries    *IntradayData
	LongerTermContext *LongerTermData
}

// OIData Open Interest数据
type OIData struct {
	Latest  float64
	Average float64
}

// IntradayData 日内数据
type IntradayData struct {
	MidPrices       []float64            // 价格序列
	IndicatorSeries map[string][]float64 // 指标序列，key格式: "EMA_20", "RSI_7", "MACD" 等
}

// LongerTermData 长期数据
type LongerTermData struct {
	CurrentIndicators map[string]float64   // 当前指标值，key格式: "EMA_20", "ATR_3" 等
	IndicatorSeries   map[string][]float64 // 指标序列，key格式: "MACD", "RSI_14" 等
	CurrentVolume     float64
	AverageVolume     float64
}

// Kline K线数据
type Kline struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// Get 获取指定代币的市场数据（要求传入market config名称）
func Get(symbol string, marketConfigName string) (*Data, error) {
	// 如果marketConfigName为空，使用"default"
	if marketConfigName == "" {
		marketConfigName = "default"
	}

	// 加载指定的市场数据配置
	config, err := GetMarketDataConfig(marketConfigName)
	if err != nil {
		// 如果指定的配置不存在，尝试使用默认配置
		config, err = GetMarketDataConfig("default")
		if err != nil {
			// 如果默认配置也不存在，使用硬编码的默认配置
			defaultConfig := getDefaultMarketDataConfig()
			return GetIndicatorsWithConfig(symbol, defaultConfig)
		}
	}
	return GetIndicatorsWithConfig(symbol, config)
}

// GetIndicatorsWithConfig 使用指定配置获取市场数据
func GetIndicatorsWithConfig(symbol string, config *MarketDataConfig) (*Data, error) {
	// 标准化symbol
	symbol = Normalize(symbol)

	// 根据 market Data config 获取该策略 (trader) 所需的全部粒度的 K线
	klinesMap := make(map[string][]Kline)
	for _, klineCfg := range config.Klines {
		klines, err := getKlines(symbol, klineCfg.Interval, klineCfg.Limit)
		if err != nil {
			return nil, fmt.Errorf("获取%s K线失败: %v", klineCfg.Interval, err)
		}
		klinesMap[klineCfg.Interval] = klines
	}

	// 获取主要 K线数据用于计算当前价格和价格变化
	// 优先使用最短间隔的K线作为主要参考
	var primaryKlines []Kline
	var primaryInterval string
	for _, klineCfg := range config.Klines {
		if klines, ok := klinesMap[klineCfg.Interval]; ok && len(klines) > 0 {
			if len(primaryKlines) == 0 {
				primaryKlines = klines
				primaryInterval = klineCfg.Interval
				break
			}
		}
	}

	if len(primaryKlines) == 0 {
		return nil, fmt.Errorf("无法获取主要K线数据")
	}

	currentPrice := primaryKlines[len(primaryKlines)-1].Close

	// 计算价格变化百分比（兼容旧代码逻辑）
	priceChange1h := 0.0
	priceChange4h := 0.0

	// 尝试计算 1小时价格变化（如果主 K线是 3m，则 20 根前）
	if primaryInterval == "3m" && len(primaryKlines) >= 21 {
		price1hAgo := primaryKlines[len(primaryKlines)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 尝试计算4小时价格变化
	if klines4h, ok := klinesMap["4h"]; ok && len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 根据配置动态计算当前指标值
	currentIndicators := make(map[string]float64)

	// 计算 EMA 指标（根据配置中的所有 EMA）
	for _, emaCfg := range config.Indicators.EMA {
		for _, source := range emaCfg.Sources {
			if klines, ok := klinesMap[source]; ok && len(klines) >= emaCfg.Period {
				key := fmt.Sprintf("EMA_%d", emaCfg.Period)
				currentIndicators[key] = CalculateEMA(klines, emaCfg.Period)
				break
			}
		}
	}

	// 计算 MACD 指标
	if config.Indicators.MACD != nil {
		macdCfg := config.Indicators.MACD
		for _, source := range macdCfg.Sources {
			if klines, ok := klinesMap[source]; ok && len(klines) >= macdCfg.Slow+macdCfg.Signal {
				macd, _, _ := CalculateMACD(klines, macdCfg.Fast, macdCfg.Slow, macdCfg.Signal)
				currentIndicators["MACD"] = macd
				break
			}
		}
	}

	// 计算 RSI 指标（根据配置中的所有 RSI）
	for _, rsiCfg := range config.Indicators.RSI {
		for _, source := range rsiCfg.Sources {
			if klines, ok := klinesMap[source]; ok && len(klines) >= rsiCfg.Period {
				key := fmt.Sprintf("RSI_%d", rsiCfg.Period)
				currentIndicators[key] = CalculateRSI(klines, rsiCfg.Period)
				break
			}
		}
	}

	// 计算布林带指标
	if config.Indicators.BollingerBands != nil {
		bbCfg := config.Indicators.BollingerBands
		for _, source := range bbCfg.Sources {
			if klines, ok := klinesMap[source]; ok && len(klines) >= bbCfg.Period {
				upper, middle, lower := CalculateBollingerBands(klines, bbCfg.Period, bbCfg.StdDev)
				key := fmt.Sprintf("BB_%d", bbCfg.Period)
				currentIndicators[fmt.Sprintf("%s_Upper", key)] = upper
				currentIndicators[fmt.Sprintf("%s_Middle", key)] = middle
				currentIndicators[fmt.Sprintf("%s_Lower", key)] = lower
				break
			}
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

	// 计算日内系列数据（使用配置）
	intradayData := calculateIntradaySeriesWithConfig(klinesMap, config)

	// 计算长期数据（使用配置）
	longerTermData := calculateLongerTermDataWithConfig(klinesMap, config)

	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentIndicators: currentIndicators,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		IntradaySeries:    intradayData,
		LongerTermContext: longerTermData,
	}, nil
}

// getKlines 从 Binance API 获取 K线数据
func getKlines(symbol, interval string, limit int) ([]Kline, error) {
	// 验证 K线间隔
	if !IsValidKlineInterval(interval) {
		return nil, fmt.Errorf("无效的K线间隔: %s", interval)
	}

	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=%d",
		symbol, interval, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawData [][]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, err
	}

	klines := make([]Kline, len(rawData))
	for i, item := range rawData {
		openTime := int64(item[0].(float64))
		open, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		low, _ := parseFloat(item[3])
		close, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])
		closeTime := int64(item[6].(float64))

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		}
	}

	return klines, nil
}

// calculateIntradaySeriesWithConfig 使用配置计算日内系列数据
func calculateIntradaySeriesWithConfig(klinesMap map[string][]Kline, config *MarketDataConfig) *IntradayData {
	data := &IntradayData{
		MidPrices:       make([]float64, 0, 10),
		IndicatorSeries: make(map[string][]float64),
	}

	// 找到最短间隔的K线用于日内数据
	var intradayKlines []Kline
	var intradayInterval string
	for _, klineCfg := range config.Klines {
		if klines, ok := klinesMap[klineCfg.Interval]; ok && len(klines) > 0 {
			if len(intradayKlines) == 0 {
				intradayKlines = klines
				intradayInterval = klineCfg.Interval
				break
			}
		}
	}

	if len(intradayKlines) == 0 {
		return data
	}

	// 获取最近10个数据点
	start := len(intradayKlines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(intradayKlines); i++ {
		data.MidPrices = append(data.MidPrices, intradayKlines[i].Close)

		// 计算每个点的EMA指标（根据配置中的所有EMA）
		for _, emaCfg := range config.Indicators.EMA {
			for _, source := range emaCfg.Sources {
				if source == intradayInterval && i >= emaCfg.Period-1 {
					key := fmt.Sprintf("EMA_%d", emaCfg.Period)
					if data.IndicatorSeries[key] == nil {
						data.IndicatorSeries[key] = make([]float64, 0, 10)
					}
					emaSeries := CalculateEMASeries(intradayKlines[:i+1], emaCfg.Period)
					if len(emaSeries) > 0 {
						data.IndicatorSeries[key] = append(data.IndicatorSeries[key], emaSeries[len(emaSeries)-1])
					}
					break
				}
			}
		}

		// 计算每个点的MACD指标（如果配置中有）
		if config.Indicators.MACD != nil {
			macdCfg := config.Indicators.MACD
			for _, source := range macdCfg.Sources {
				if source == intradayInterval && i >= macdCfg.Slow+macdCfg.Signal-1 {
					if data.IndicatorSeries["MACD"] == nil {
						data.IndicatorSeries["MACD"] = make([]float64, 0, 10)
					}
					macdSeries, _, _ := CalculateMACDSeries(intradayKlines[:i+1], macdCfg.Fast, macdCfg.Slow, macdCfg.Signal)
					if len(macdSeries) > 0 {
						data.IndicatorSeries["MACD"] = append(data.IndicatorSeries["MACD"], macdSeries[len(macdSeries)-1])
					}
					break
				}
			}
		}

		// 计算每个点的RSI指标（根据配置中的所有RSI）
		for _, rsiCfg := range config.Indicators.RSI {
			for _, source := range rsiCfg.Sources {
				if source == intradayInterval && i >= rsiCfg.Period-1 {
					key := fmt.Sprintf("RSI_%d", rsiCfg.Period)
					if data.IndicatorSeries[key] == nil {
						data.IndicatorSeries[key] = make([]float64, 0, 10)
					}
					rsiSeries := CalculateRSISeries(intradayKlines[:i+1], rsiCfg.Period)
					if len(rsiSeries) > 0 {
						data.IndicatorSeries[key] = append(data.IndicatorSeries[key], rsiSeries[len(rsiSeries)-1])
					}
					break
				}
			}
		}

		// 计算每个点的布林带指标（如果配置中有）
		if config.Indicators.BollingerBands != nil {
			bbCfg := config.Indicators.BollingerBands
			for _, source := range bbCfg.Sources {
				if source == intradayInterval && i >= bbCfg.Period-1 {
					key := fmt.Sprintf("BB_%d", bbCfg.Period)
					upperKey := fmt.Sprintf("%s_Upper", key)
					middleKey := fmt.Sprintf("%s_Middle", key)
					lowerKey := fmt.Sprintf("%s_Lower", key)

					if data.IndicatorSeries[upperKey] == nil {
						data.IndicatorSeries[upperKey] = make([]float64, 0, 10)
						data.IndicatorSeries[middleKey] = make([]float64, 0, 10)
						data.IndicatorSeries[lowerKey] = make([]float64, 0, 10)
					}

					upperSeries, middleSeries, lowerSeries := CalculateBollingerBandsSeries(intradayKlines[:i+1], bbCfg.Period, bbCfg.StdDev)
					if len(upperSeries) > 0 {
						data.IndicatorSeries[upperKey] = append(data.IndicatorSeries[upperKey], upperSeries[len(upperSeries)-1])
						data.IndicatorSeries[middleKey] = append(data.IndicatorSeries[middleKey], middleSeries[len(middleSeries)-1])
						data.IndicatorSeries[lowerKey] = append(data.IndicatorSeries[lowerKey], lowerSeries[len(lowerSeries)-1])
					}
					break
				}
			}
		}
	}

	return data
}

// calculateLongerTermDataWithConfig 使用配置计算长期数据
func calculateLongerTermDataWithConfig(klinesMap map[string][]Kline, config *MarketDataConfig) *LongerTermData {
	data := &LongerTermData{
		CurrentIndicators: make(map[string]float64),
		IndicatorSeries:   make(map[string][]float64),
	}

	// 找到最长间隔的K线用于长期数据（优先4h，否则使用最长的）
	var longerTermKlines []Kline
	var longerTermInterval string
	for _, klineCfg := range config.Klines {
		if klines, ok := klinesMap[klineCfg.Interval]; ok {
			if longerTermKlines == nil || klineCfg.Interval == "4h" {
				longerTermKlines = klines
				longerTermInterval = klineCfg.Interval
				if klineCfg.Interval == "4h" {
					break
				}
			}
		}
	}

	if len(longerTermKlines) == 0 {
		return data
	}

	// 计算EMA指标（根据配置中的所有EMA）
	for _, emaCfg := range config.Indicators.EMA {
		for _, source := range emaCfg.Sources {
			if source == longerTermInterval && len(longerTermKlines) >= emaCfg.Period {
				key := fmt.Sprintf("EMA_%d", emaCfg.Period)
				data.CurrentIndicators[key] = CalculateEMA(longerTermKlines, emaCfg.Period)
				break
			}
		}
	}

	// 计算ATR指标（根据配置中的所有ATR）
	for _, atrCfg := range config.Indicators.ATR {
		for _, source := range atrCfg.Sources {
			if source == longerTermInterval && len(longerTermKlines) >= atrCfg.Period {
				key := fmt.Sprintf("ATR_%d", atrCfg.Period)
				data.CurrentIndicators[key] = CalculateATR(longerTermKlines, atrCfg.Period)
				break
			}
		}
	}

	// 计算布林带指标（如果配置中有）
	if config.Indicators.BollingerBands != nil {
		bbCfg := config.Indicators.BollingerBands
		for _, source := range bbCfg.Sources {
			if source == longerTermInterval && len(longerTermKlines) >= bbCfg.Period {
				upper, middle, lower := CalculateBollingerBands(longerTermKlines, bbCfg.Period, bbCfg.StdDev)
				key := fmt.Sprintf("BB_%d", bbCfg.Period)
				data.CurrentIndicators[fmt.Sprintf("%s_Upper", key)] = upper
				data.CurrentIndicators[fmt.Sprintf("%s_Middle", key)] = middle
				data.CurrentIndicators[fmt.Sprintf("%s_Lower", key)] = lower
				break
			}
		}
	}

	// 计算成交量
	if len(longerTermKlines) > 0 {
		data.CurrentVolume = longerTermKlines[len(longerTermKlines)-1].Volume
		sum := 0.0
		for _, k := range longerTermKlines {
			sum += k.Volume
		}
		data.AverageVolume = sum / float64(len(longerTermKlines))
	}

	// 计算MACD和RSI序列
	start := len(longerTermKlines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(longerTermKlines); i++ {
		// MACD序列
		if config.Indicators.MACD != nil {
			macdCfg := config.Indicators.MACD
			for _, source := range macdCfg.Sources {
				if source == longerTermInterval && i >= macdCfg.Slow+macdCfg.Signal-1 {
					if data.IndicatorSeries["MACD"] == nil {
						data.IndicatorSeries["MACD"] = make([]float64, 0, 10)
					}
					macdSeries, _, _ := CalculateMACDSeries(longerTermKlines[:i+1], macdCfg.Fast, macdCfg.Slow, macdCfg.Signal)
					if len(macdSeries) > 0 {
						data.IndicatorSeries["MACD"] = append(data.IndicatorSeries["MACD"], macdSeries[len(macdSeries)-1])
					}
					break
				}
			}
		}

		// RSI序列（根据配置中的所有RSI）
		for _, rsiCfg := range config.Indicators.RSI {
			for _, source := range rsiCfg.Sources {
				if source == longerTermInterval && i >= rsiCfg.Period-1 {
					key := fmt.Sprintf("RSI_%d", rsiCfg.Period)
					if data.IndicatorSeries[key] == nil {
						data.IndicatorSeries[key] = make([]float64, 0, 10)
					}
					rsiSeries := CalculateRSISeries(longerTermKlines[:i+1], rsiCfg.Period)
					if len(rsiSeries) > 0 {
						data.IndicatorSeries[key] = append(data.IndicatorSeries[key], rsiSeries[len(rsiSeries)-1])
					}
					break
				}
			}
		}

		// 布林带序列（如果配置中有）
		if config.Indicators.BollingerBands != nil {
			bbCfg := config.Indicators.BollingerBands
			for _, source := range bbCfg.Sources {
				if source == longerTermInterval && i >= bbCfg.Period-1 {
					key := fmt.Sprintf("BB_%d", bbCfg.Period)
					upperKey := fmt.Sprintf("%s_Upper", key)
					middleKey := fmt.Sprintf("%s_Middle", key)
					lowerKey := fmt.Sprintf("%s_Lower", key)

					if data.IndicatorSeries[upperKey] == nil {
						data.IndicatorSeries[upperKey] = make([]float64, 0, 10)
						data.IndicatorSeries[middleKey] = make([]float64, 0, 10)
						data.IndicatorSeries[lowerKey] = make([]float64, 0, 10)
					}

					upperSeries, middleSeries, lowerSeries := CalculateBollingerBandsSeries(longerTermKlines[:i+1], bbCfg.Period, bbCfg.StdDev)
					if len(upperSeries) > 0 {
						data.IndicatorSeries[upperKey] = append(data.IndicatorSeries[upperKey], upperSeries[len(upperSeries)-1])
						data.IndicatorSeries[middleKey] = append(data.IndicatorSeries[middleKey], middleSeries[len(middleSeries)-1])
						data.IndicatorSeries[lowerKey] = append(data.IndicatorSeries[lowerKey], lowerSeries[len(lowerSeries)-1])
					}
					break
				}
			}
		}
	}

	return data
}

// getOpenInterestData 获取OI数据
func getOpenInterestData(symbol string) (*OIData, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := http.Get(url)
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

// getFundingRate 获取资金费率
func getFundingRate(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	resp, err := http.Get(url)
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
	return rate, nil
}

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	// 输出当前价格和当前指标值（动态）
	indicatorStrs := make([]string, 0)
	for key, value := range data.CurrentIndicators {
		indicatorStrs = append(indicatorStrs, fmt.Sprintf("%s = %.3f", key, value))
	}

	if len(indicatorStrs) > 0 {
		sb.WriteString(fmt.Sprintf("current_price = %.2f, %s\n\n",
			data.CurrentPrice, strings.Join(indicatorStrs, ", ")))
	} else {
		sb.WriteString(fmt.Sprintf("current_price = %.2f\n\n", data.CurrentPrice))
	}

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n",
			data.OpenInterest.Latest, data.OpenInterest.Average))
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))

	if data.IntradaySeries != nil {
		sb.WriteString("Intraday series (oldest → latest):\n\n")

		if len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
		}

		// 动态输出所有指标序列（按key排序以保证输出一致性）
		keys := make([]string, 0, len(data.IntradaySeries.IndicatorSeries))
		for key := range data.IntradaySeries.IndicatorSeries {
			keys = append(keys, key)
		}
		// 排序以保持输出一致性
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}

		for _, key := range keys {
			if values := data.IntradaySeries.IndicatorSeries[key]; len(values) > 0 {
				sb.WriteString(fmt.Sprintf("%s indicators: %s\n\n", key, formatFloatSlice(values)))
			}
		}
	}

	if data.LongerTermContext != nil {
		sb.WriteString("Longer‑term context:\n\n")

		// 动态输出当前指标值
		if len(data.LongerTermContext.CurrentIndicators) > 0 {
			indicatorStrs := make([]string, 0)
			for key, value := range data.LongerTermContext.CurrentIndicators {
				indicatorStrs = append(indicatorStrs, fmt.Sprintf("%s = %.3f", key, value))
			}
			// 排序以保持输出一致性
			for i := 0; i < len(indicatorStrs); i++ {
				for j := i + 1; j < len(indicatorStrs); j++ {
					if indicatorStrs[i] > indicatorStrs[j] {
						indicatorStrs[i], indicatorStrs[j] = indicatorStrs[j], indicatorStrs[i]
					}
				}
			}
			if len(indicatorStrs) > 0 {
				sb.WriteString(strings.Join(indicatorStrs, " | "))
				sb.WriteString("\n\n")
			}
		}

		sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
			data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))

		// 动态输出指标序列
		if len(data.LongerTermContext.IndicatorSeries) > 0 {
			keys := make([]string, 0, len(data.LongerTermContext.IndicatorSeries))
			for key := range data.LongerTermContext.IndicatorSeries {
				keys = append(keys, key)
			}
			// 排序以保持输出一致性
			for i := 0; i < len(keys); i++ {
				for j := i + 1; j < len(keys); j++ {
					if keys[i] > keys[j] {
						keys[i], keys[j] = keys[j], keys[i]
					}
				}
			}

			for _, key := range keys {
				if values := data.LongerTermContext.IndicatorSeries[key]; len(values) > 0 {
					sb.WriteString(fmt.Sprintf("%s indicators: %s\n\n", key, formatFloatSlice(values)))
				}
			}
		}
	}

	return sb.String()
}

// formatFloatSlice 格式化float64切片为字符串
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.3f", v)
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
