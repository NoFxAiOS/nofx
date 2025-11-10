package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/greatcloak/decimal"
)

// FundingRateCache 资金费率缓存结构
// Binance Funding Rate 每 8 小时才更新一次，使用 1 小时缓存可显著减少 API 调用
type FundingRateCache struct {
	Rate      decimal.Decimal
	UpdatedAt time.Time
}

var (
	fundingRateMap sync.Map // map[string]*FundingRateCache
	frCacheTTL     = 1 * time.Hour
)

// Get 获取指定代币的市场数据
func Get(symbol string) (*Data, error) {
	var klines3m, klines4h []Kline
	var err error
	// 标准化symbol
	symbol = Normalize(symbol)
	// 获取3分钟K线数据 (最近10个)
	klines3m, err = WSMonitorCli.GetCurrentKlines(symbol, "3m") // 多获取一些用于计算
	if err != nil {
		return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
	}

	// 获取4小时K线数据 (最近10个)
	klines4h, err = WSMonitorCli.GetCurrentKlines(symbol, "4h") // 多获取用于计算指标
	if err != nil {
		return nil, fmt.Errorf("获取4小时K线失败: %v", err)
	}

	// 检查数据是否为空
	if len(klines3m) == 0 {
		return nil, fmt.Errorf("3分钟K线数据为空")
	}
	if len(klines4h) == 0 {
		return nil, fmt.Errorf("4小时K线数据为空")
	}

	// 计算当前指标 (基于3分钟最新数据)
	currentPrice := klines3m[len(klines3m)-1].Close
	currentEMA20 := calculateEMA(klines3m, 20)
	currentMACD := calculateMACD(klines3m)
	currentRSI7 := calculateRSI(klines3m, 7)

	// 计算价格变化百分比
	// 1小时价格变化 = 20个3分钟K线前的价格
	priceChange1h := decimal.Zero
	if len(klines3m) >= 21 { // 至少需要21根K线 (当前 + 20根前)
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo.GreaterThan(decimal.Zero) {
			priceChange1h = currentPrice.Sub(price1hAgo).Div(price1hAgo).Mul(decimal.NewFromInt(100))
		}
	}

	// 4小时价格变化 = 1个4小时K线前的价格
	priceChange4h := decimal.Zero
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo.GreaterThan(decimal.Zero) {
			priceChange4h = currentPrice.Sub(price4hAgo).Div(price4hAgo).Mul(decimal.NewFromInt(100))
		}
	}

	// 获取OI数据
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		// OI失败不影响整体,使用默认值
		oiData = &OIData{Latest: decimal.Zero, Average: decimal.Zero}
	}

	// 获取Funding Rate
	fundingRate, _ := getFundingRate(symbol)

	// 计算日内系列数据
	intradayData := calculateIntradaySeries(klines3m)

	// 计算长期数据
	longerTermData := calculateLongerTermData(klines4h)

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
		IntradaySeries:    intradayData,
		LongerTermContext: longerTermData,
	}, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) decimal.Decimal {
	if len(klines) < period {
		return decimal.Zero
	}

	// 计算SMA作为初始EMA
	sum := decimal.Zero
	for i := 0; i < period; i++ {
		sum = sum.Add(klines[i].Close)
	}
	ema := sum.Div(decimal.NewFromInt(int64(period)))

	// 计算EMA
	multiplier := decimal.NewFromInt(2).Div(decimal.NewFromInt(int64(period + 1)))
	for i := period; i < len(klines); i++ {
		ema = klines[i].Close.Sub(ema).Mul(multiplier).Add(ema)
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(klines []Kline) decimal.Decimal {
	if len(klines) < 26 {
		return decimal.Zero
	}

	// 计算12期和26期EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12.Sub(ema26)
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) decimal.Decimal {
	if len(klines) <= period {
		return decimal.Zero
	}

	gains := decimal.Zero
	losses := decimal.Zero

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close.Sub(klines[i-1].Close)
		if change.GreaterThan(decimal.Zero) {
			gains = gains.Add(change)
		} else {
			losses = losses.Add(change.Neg())
		}
	}

	periodDec := decimal.NewFromInt(int64(period))
	avgGain := gains.Div(periodDec)
	avgLoss := losses.Div(periodDec)

	// 使用Wilder平滑方法计算后续RSI
	periodMinus1 := decimal.NewFromInt(int64(period - 1))
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close.Sub(klines[i-1].Close)
		if change.GreaterThan(decimal.Zero) {
			avgGain = avgGain.Mul(periodMinus1).Add(change).Div(periodDec)
			avgLoss = avgLoss.Mul(periodMinus1).Div(periodDec)
		} else {
			avgGain = avgGain.Mul(periodMinus1).Div(periodDec)
			avgLoss = avgLoss.Mul(periodMinus1).Add(change.Neg()).Div(periodDec)
		}
	}

	if avgLoss.IsZero() {
		return decimal.NewFromInt(100)
	}

	rs := avgGain.Div(avgLoss)
	rsi := decimal.NewFromInt(100).Sub(decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(rs)))

	return rsi
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) decimal.Decimal {
	if len(klines) <= period {
		return decimal.Zero
	}

	trs := make([]decimal.Decimal, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high.Sub(low)
		tr2 := high.Sub(prevClose).Abs()
		tr3 := low.Sub(prevClose).Abs()

		// 取最大值
		trs[i] = tr1
		if tr2.GreaterThan(trs[i]) {
			trs[i] = tr2
		}
		if tr3.GreaterThan(trs[i]) {
			trs[i] = tr3
		}
	}

	// 计算初始ATR
	sum := decimal.Zero
	for i := 1; i <= period; i++ {
		sum = sum.Add(trs[i])
	}
	periodDec := decimal.NewFromInt(int64(period))
	atr := sum.Div(periodDec)

	// Wilder平滑
	periodMinus1 := decimal.NewFromInt(int64(period - 1))
	for i := period + 1; i < len(klines); i++ {
		atr = atr.Mul(periodMinus1).Add(trs[i]).Div(periodDec)
	}

	return atr
}

// calculateIntradaySeries 计算日内系列数据
func calculateIntradaySeries(klines []Kline) *IntradayData {
	data := &IntradayData{
		MidPrices:   make([]decimal.Decimal, 0, 10),
		EMA20Values: make([]decimal.Decimal, 0, 10),
		MACDValues:  make([]decimal.Decimal, 0, 10),
		RSI7Values:  make([]decimal.Decimal, 0, 10),
		RSI14Values: make([]decimal.Decimal, 0, 10),
	}

	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)

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
	}

	return data
}

// calculateLongerTermData 计算长期数据
func calculateLongerTermData(klines []Kline) *LongerTermData {
	data := &LongerTermData{
		MACDValues:  make([]decimal.Decimal, 0, 10),
		RSI14Values: make([]decimal.Decimal, 0, 10),
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
		sum := decimal.Zero
		for _, k := range klines {
			sum = sum.Add(k.Volume)
		}
		data.AverageVolume = sum.Div(decimal.NewFromInt(int64(len(klines))))
	}

	// 计算MACD和RSI序列
	start := len(klines) - 10
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

	oi, _ := decimal.NewFromString(result.OpenInterest)

	return &OIData{
		Latest:  oi,
		Average: oi.Mul(decimal.NewFromFloat(0.999)), // 近似平均值
	}, nil
}

// getFundingRate 获取资金费率（优化：使用 1 小时缓存）
func getFundingRate(symbol string) (decimal.Decimal, error) {
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
		return decimal.Zero, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return decimal.Zero, err
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
		return decimal.Zero, err
	}

	rate, _ := decimal.NewFromString(result.LastFundingRate)

	// 更新缓存
	fundingRateMap.Store(symbol, &FundingRateCache{
		Rate:      rate,
		UpdatedAt: time.Now(),
	})

	return rate, nil
}

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	// 使用动态精度格式化价格
	priceStr := formatPriceWithDynamicPrecision(data.CurrentPrice)
	sb.WriteString(fmt.Sprintf("current_price = %s, current_ema20 = %s, current_macd = %s, current_rsi (7 period) = %s\n\n",
		priceStr, data.CurrentEMA20.StringFixed(3), data.CurrentMACD.StringFixed(3), data.CurrentRSI7.StringFixed(3)))

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		// 使用动态精度格式化 OI 数据
		oiLatestStr := formatPriceWithDynamicPrecision(data.OpenInterest.Latest)
		oiAverageStr := formatPriceWithDynamicPrecision(data.OpenInterest.Average)
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %s Average: %s\n\n",
			oiLatestStr, oiAverageStr))
	}

	fundingRateFloat, _ := data.FundingRate.Float64()
	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", fundingRateFloat))

	if data.IntradaySeries != nil {
		sb.WriteString("Intraday series (3‑minute intervals, oldest → latest):\n\n")

		if len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatDecimalSlice(data.IntradaySeries.MidPrices)))
		}

		if len(data.IntradaySeries.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatDecimalSlice(data.IntradaySeries.EMA20Values)))
		}

		if len(data.IntradaySeries.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatDecimalSlice(data.IntradaySeries.MACDValues)))
		}

		if len(data.IntradaySeries.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatDecimalSlice(data.IntradaySeries.RSI7Values)))
		}

		if len(data.IntradaySeries.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatDecimalSlice(data.IntradaySeries.RSI14Values)))
		}
	}

	if data.LongerTermContext != nil {
		sb.WriteString("Longer‑term context (4‑hour timeframe):\n\n")

		sb.WriteString(fmt.Sprintf("20‑Period EMA: %s vs. 50‑Period EMA: %s\n\n",
			data.LongerTermContext.EMA20.StringFixed(3), data.LongerTermContext.EMA50.StringFixed(3)))

		sb.WriteString(fmt.Sprintf("3‑Period ATR: %s vs. 14‑Period ATR: %s\n\n",
			data.LongerTermContext.ATR3.StringFixed(3), data.LongerTermContext.ATR14.StringFixed(3)))

		sb.WriteString(fmt.Sprintf("Current Volume: %s vs. Average Volume: %s\n\n",
			data.LongerTermContext.CurrentVolume.StringFixed(3), data.LongerTermContext.AverageVolume.StringFixed(3)))

		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatDecimalSlice(data.LongerTermContext.MACDValues)))
		}

		if len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatDecimalSlice(data.LongerTermContext.RSI14Values)))
		}
	}

	return sb.String()
}

// formatPriceWithDynamicPrecision 根据价格区间动态选择精度
// 这样可以完美支持从超低价 meme coin (< 0.0001) 到 BTC/ETH 的所有币种
func formatPriceWithDynamicPrecision(price decimal.Decimal) string {
	priceFloat, _ := price.Float64()
	switch {
	case priceFloat < 0.0001:
		// 超低价 meme coin: 1000SATS, 1000WHY, DOGS
		// 0.00002070 → "0.00002070" (8位小数)
		return price.StringFixed(8)
	case priceFloat < 0.001:
		// 低价 meme coin: NEIRO, HMSTR, HOT, NOT
		// 0.00015060 → "0.000151" (6位小数)
		return price.StringFixed(6)
	case priceFloat < 0.01:
		// 中低价币: PEPE, SHIB, MEME
		// 0.00556800 → "0.005568" (6位小数)
		return price.StringFixed(6)
	case priceFloat < 1.0:
		// 低价币: ASTER, DOGE, ADA, TRX
		// 0.9954 → "0.9954" (4位小数)
		return price.StringFixed(4)
	case priceFloat < 100:
		// 中价币: SOL, AVAX, LINK, MATIC
		// 23.4567 → "23.4567" (4位小数)
		return price.StringFixed(4)
	default:
		// 高价币: BTC, ETH (节省 Token)
		// 45678.9123 → "45678.91" (2位小数)
		return price.StringFixed(2)
	}
}

// formatDecimalSlice 格式化decimal切片为字符串（使用动态精度）
func formatDecimalSlice(values []decimal.Decimal) string {
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
