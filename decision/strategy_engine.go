package decision

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"nofx/pool"
	"nofx/store"
	"strings"
	"time"
)

// StrategyEngine 策略执行引擎
// 负责基于策略配置动态获取数据和组装 Prompt
type StrategyEngine struct {
	config *store.StrategyConfig
}

// NewStrategyEngine 创建策略执行引擎
func NewStrategyEngine(config *store.StrategyConfig) *StrategyEngine {
	return &StrategyEngine{config: config}
}

// GetCandidateCoins 根据策略配置获取候选币种
func (e *StrategyEngine) GetCandidateCoins() ([]CandidateCoin, error) {
	var candidates []CandidateCoin
	symbolSources := make(map[string][]string)

	coinSource := e.config.CoinSource

	// 设置自定义的 API URL（如果配置了）
	if coinSource.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(coinSource.CoinPoolAPIURL)
		logger.Infof("✓ 使用策略配置的 AI500 API URL: %s", coinSource.CoinPoolAPIURL)
	}
	if coinSource.OITopAPIURL != "" {
		pool.SetOITopAPI(coinSource.OITopAPIURL)
		logger.Infof("✓ 使用策略配置的 OI Top API URL: %s", coinSource.OITopAPIURL)
	}

	switch coinSource.SourceType {
	case "static":
		// 静态币种列表
		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"static"},
			})
		}
		return candidates, nil

	case "coinpool":
		// 仅使用 AI500 币种池
		return e.getCoinPoolCoins(coinSource.CoinPoolLimit)

	case "oi_top":
		// 仅使用 OI Top
		return e.getOITopCoins(coinSource.OITopLimit)

	case "mixed":
		// 混合模式：AI500 + OI Top
		if coinSource.UseCoinPool {
			poolCoins, err := e.getCoinPoolCoins(coinSource.CoinPoolLimit)
			if err != nil {
				logger.Infof("⚠️  获取 AI500 币种池失败: %v", err)
			} else {
				for _, coin := range poolCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "ai500")
				}
			}
		}

		if coinSource.UseOITop {
			oiCoins, err := e.getOITopCoins(coinSource.OITopLimit)
			if err != nil {
				logger.Infof("⚠️  获取 OI Top 失败: %v", err)
			} else {
				for _, coin := range oiCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "oi_top")
				}
			}
		}

		// 添加静态币种（如果有）
		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			if _, exists := symbolSources[symbol]; !exists {
				symbolSources[symbol] = []string{"static"}
			} else {
				symbolSources[symbol] = append(symbolSources[symbol], "static")
			}
		}

		// 转换为候选币种列表
		for symbol, sources := range symbolSources {
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: sources,
			})
		}
		return candidates, nil

	default:
		return nil, fmt.Errorf("未知的币种来源类型: %s", coinSource.SourceType)
	}
}

// getCoinPoolCoins 获取 AI500 币种池
func (e *StrategyEngine) getCoinPoolCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 30
	}

	symbols, err := pool.GetTopRatedCoins(limit)
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"ai500"},
		})
	}
	return candidates, nil
}

// getOITopCoins 获取 OI Top 币种
func (e *StrategyEngine) getOITopCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 20
	}

	positions, err := pool.GetOITopPositions()
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for i, pos := range positions {
		if i >= limit {
			break
		}
		symbol := market.Normalize(pos.Symbol)
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"oi_top"},
		})
	}
	return candidates, nil
}

// FetchMarketData 根据策略配置获取市场数据
func (e *StrategyEngine) FetchMarketData(symbol string) (*market.Data, error) {
	// 目前使用现有的 market.Get，后续可以根据策略配置自定义
	return market.Get(symbol)
}

// FetchExternalData 获取外部数据源
func (e *StrategyEngine) FetchExternalData() (map[string]interface{}, error) {
	externalData := make(map[string]interface{})

	for _, source := range e.config.Indicators.ExternalDataSources {
		data, err := e.fetchSingleExternalSource(source)
		if err != nil {
			logger.Infof("⚠️  获取外部数据源 [%s] 失败: %v", source.Name, err)
			continue
		}
		externalData[source.Name] = data
	}

	return externalData, nil
}

// QuantData 量化数据结构（资金流向、持仓变化、价格变化）
type QuantData struct {
	Symbol      string                 `json:"symbol"`
	Price       float64                `json:"price"`
	Netflow     *NetflowData           `json:"netflow,omitempty"`
	OI          map[string]*OIData     `json:"oi,omitempty"`
	PriceChange map[string]float64     `json:"price_change,omitempty"`
}

type NetflowData struct {
	Institution *FlowTypeData `json:"institution,omitempty"`
	Personal    *FlowTypeData `json:"personal,omitempty"`
}

type FlowTypeData struct {
	Future map[string]float64 `json:"future,omitempty"`
	Spot   map[string]float64 `json:"spot,omitempty"`
}

type OIData struct {
	CurrentOI float64                `json:"current_oi"`
	NetLong   float64                `json:"net_long"`
	NetShort  float64                `json:"net_short"`
	Delta     map[string]*OIDeltaData `json:"delta,omitempty"`
}

type OIDeltaData struct {
	OIDelta        float64 `json:"oi_delta"`
	OIDeltaValue   float64 `json:"oi_delta_value"`
	OIDeltaPercent float64 `json:"oi_delta_percent"`
}

// FetchQuantData 获取单个币种的量化数据
func (e *StrategyEngine) FetchQuantData(symbol string) (*QuantData, error) {
	if !e.config.Indicators.EnableQuantData || e.config.Indicators.QuantDataAPIURL == "" {
		return nil, nil
	}

	// 替换 {symbol} 占位符
	url := strings.Replace(e.config.Indicators.QuantDataAPIURL, "{symbol}", symbol, -1)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var apiResp struct {
		Code int       `json:"code"`
		Data *QuantData `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("API返回错误码: %d", apiResp.Code)
	}

	return apiResp.Data, nil
}

// FetchQuantDataBatch 批量获取量化数据
func (e *StrategyEngine) FetchQuantDataBatch(symbols []string) map[string]*QuantData {
	result := make(map[string]*QuantData)

	if !e.config.Indicators.EnableQuantData || e.config.Indicators.QuantDataAPIURL == "" {
		return result
	}

	for _, symbol := range symbols {
		data, err := e.FetchQuantData(symbol)
		if err != nil {
			logger.Infof("⚠️  获取 %s 量化数据失败: %v", symbol, err)
			continue
		}
		if data != nil {
			result[symbol] = data
		}
	}

	return result
}

// formatQuantData 格式化量化数据
func (e *StrategyEngine) formatQuantData(data *QuantData) string {
	if data == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("📊 量化数据:\n")

	// 价格变化
	if len(data.PriceChange) > 0 {
		sb.WriteString("价格变化: ")
		timeframes := []string{"5m", "15m", "1h", "4h", "24h"}
		parts := []string{}
		for _, tf := range timeframes {
			if v, ok := data.PriceChange[tf]; ok {
				parts = append(parts, fmt.Sprintf("%s: %+.2f%%", tf, v))
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
	}

	// 资金流向
	if data.Netflow != nil {
		sb.WriteString("资金流向(USDT):\n")

		// 机构资金
		if data.Netflow.Institution != nil {
			if data.Netflow.Institution.Future != nil {
				sb.WriteString("  机构合约: ")
				parts := []string{}
				for _, tf := range []string{"1h", "4h", "24h"} {
					if v, ok := data.Netflow.Institution.Future[tf]; ok {
						parts = append(parts, fmt.Sprintf("%s: %+.0f", tf, v))
					}
				}
				sb.WriteString(strings.Join(parts, " | "))
				sb.WriteString("\n")
			}
			if data.Netflow.Institution.Spot != nil {
				sb.WriteString("  机构现货: ")
				parts := []string{}
				for _, tf := range []string{"1h", "4h", "24h"} {
					if v, ok := data.Netflow.Institution.Spot[tf]; ok {
						parts = append(parts, fmt.Sprintf("%s: %+.0f", tf, v))
					}
				}
				sb.WriteString(strings.Join(parts, " | "))
				sb.WriteString("\n")
			}
		}

		// 散户资金
		if data.Netflow.Personal != nil {
			if data.Netflow.Personal.Future != nil {
				sb.WriteString("  散户合约: ")
				parts := []string{}
				for _, tf := range []string{"1h", "4h", "24h"} {
					if v, ok := data.Netflow.Personal.Future[tf]; ok {
						parts = append(parts, fmt.Sprintf("%s: %+.0f", tf, v))
					}
				}
				sb.WriteString(strings.Join(parts, " | "))
				sb.WriteString("\n")
			}
		}
	}

	// 持仓数据
	if len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			sb.WriteString(fmt.Sprintf("持仓(%s): 当前%.2f | 多%.2f 空%.2f\n",
				exchange, oiData.CurrentOI, oiData.NetLong, oiData.NetShort))
			if len(oiData.Delta) > 0 {
				sb.WriteString("  持仓变化: ")
				parts := []string{}
				for _, tf := range []string{"1h", "4h", "24h"} {
					if d, ok := oiData.Delta[tf]; ok {
						parts = append(parts, fmt.Sprintf("%s: %+.2f%%", tf, d.OIDeltaPercent))
					}
				}
				sb.WriteString(strings.Join(parts, " | "))
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// fetchSingleExternalSource 获取单个外部数据源
func (e *StrategyEngine) fetchSingleExternalSource(source store.ExternalDataSource) (interface{}, error) {
	client := &http.Client{
		Timeout: time.Duration(source.RefreshSecs) * time.Second,
	}

	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}

	req, err := http.NewRequest(source.Method, source.URL, nil)
	if err != nil {
		return nil, err
	}

	// 添加请求头
	for k, v := range source.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// 如果指定了数据路径，提取指定路径的数据
	if source.DataPath != "" {
		result = extractJSONPath(result, source.DataPath)
	}

	return result, nil
}

// extractJSONPath 提取 JSON 路径数据（简单实现）
func extractJSONPath(data interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

// BuildUserPrompt 根据策略配置构建 User Prompt
func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("时间: %s | 周期: #%d | 运行: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场（如果配置了）
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// 账户信息
	sb.WriteString(fmt.Sprintf("账户: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 持仓信息
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(e.formatPositionInfo(i+1, pos, ctx))
		}
	} else {
		sb.WriteString("当前持仓: 无\n\n")
	}

	// 交易统计
	if ctx.TradingStats != nil && ctx.TradingStats.TotalTrades > 0 {
		sb.WriteString("## 历史交易统计\n")
		sb.WriteString(fmt.Sprintf("总交易数: %d | 胜率: %.1f%% | 盈亏比: %.2f | 夏普比: %.2f\n",
			ctx.TradingStats.TotalTrades,
			ctx.TradingStats.WinRate,
			ctx.TradingStats.ProfitFactor,
			ctx.TradingStats.SharpeRatio))
		sb.WriteString(fmt.Sprintf("总盈亏: %.2f USDT | 平均盈利: %.2f | 平均亏损: %.2f | 最大回撤: %.1f%%\n\n",
			ctx.TradingStats.TotalPnL,
			ctx.TradingStats.AvgWin,
			ctx.TradingStats.AvgLoss,
			ctx.TradingStats.MaxDrawdownPct))
	}

	// 最近完成的订单
	if len(ctx.RecentOrders) > 0 {
		sb.WriteString("## 最近完成的交易\n")
		for i, order := range ctx.RecentOrders {
			resultStr := "盈利"
			if order.RealizedPnL < 0 {
				resultStr = "亏损"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s | 入场%.4f 出场%.4f | %s: %+.2f USDT (%+.2f%%) | %s\n",
				i+1, order.Symbol, order.Side,
				order.EntryPrice, order.ExitPrice,
				resultStr, order.RealizedPnL, order.PnLPct,
				order.FilledAt))
		}
		sb.WriteString("\n")
	}

	// 候选币种
	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := e.formatCoinSourceTag(coin.Sources)
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(e.formatMarketData(marketData))

		// 添加量化数据（如果有）
		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[coin.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("---\n\n")
	sb.WriteString("现在请分析并输出决策（思维链 + JSON）\n")

	return sb.String()
}

// formatPositionInfo 格式化持仓信息
func (e *StrategyEngine) formatPositionInfo(index int, pos PositionInfo, ctx *Context) string {
	var sb strings.Builder

	// 计算持仓时长
	holdingDuration := ""
	if pos.UpdateTime > 0 {
		durationMs := time.Now().UnixMilli() - pos.UpdateTime
		durationMin := durationMs / (1000 * 60)
		if durationMin < 60 {
			holdingDuration = fmt.Sprintf(" | 持仓时长%d分钟", durationMin)
		} else {
			durationHour := durationMin / 60
			durationMinRemainder := durationMin % 60
			holdingDuration = fmt.Sprintf(" | 持仓时长%d小时%d分钟", durationHour, durationMinRemainder)
		}
	}

	// 计算仓位价值
	positionValue := pos.Quantity * pos.MarkPrice
	if positionValue < 0 {
		positionValue = -positionValue
	}

	sb.WriteString(fmt.Sprintf("%d. %s %s | 入场价%.4f 当前价%.4f | 数量%.4f | 仓位价值%.2f USDT | 盈亏%+.2f%% | 盈亏金额%+.2f USDT | 最高收益率%.2f%% | 杠杆%dx | 保证金%.0f | 强平价%.4f%s\n\n",
		index, pos.Symbol, strings.ToUpper(pos.Side),
		pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
		pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

	// 使用策略配置的指标输出市场数据
	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
		sb.WriteString(e.formatMarketData(marketData))

		// 添加量化数据（如果有）
		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[pos.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatCoinSourceTag 格式化币种来源标签
func (e *StrategyEngine) formatCoinSourceTag(sources []string) string {
	if len(sources) > 1 {
		return " (AI500+OI_Top双重信号)"
	} else if len(sources) == 1 {
		switch sources[0] {
		case "ai500":
			return " (AI500)"
		case "oi_top":
			return " (OI_Top持仓增长)"
		case "static":
			return " (手动选择)"
		}
	}
	return ""
}

// formatMarketData 根据策略配置格式化市场数据
func (e *StrategyEngine) formatMarketData(data *market.Data) string {
	var sb strings.Builder
	indicators := e.config.Indicators

	// 当前价格（总是显示）
	sb.WriteString(fmt.Sprintf("current_price = %.4f", data.CurrentPrice))

	// EMA
	if indicators.EnableEMA {
		sb.WriteString(fmt.Sprintf(", current_ema20 = %.3f", data.CurrentEMA20))
	}

	// MACD
	if indicators.EnableMACD {
		sb.WriteString(fmt.Sprintf(", current_macd = %.3f", data.CurrentMACD))
	}

	// RSI
	if indicators.EnableRSI {
		sb.WriteString(fmt.Sprintf(", current_rsi7 = %.3f", data.CurrentRSI7))
	}

	sb.WriteString("\n\n")

	// OI 和 Funding Rate
	if indicators.EnableOI || indicators.EnableFundingRate {
		sb.WriteString(fmt.Sprintf("Additional data for %s:\n\n", data.Symbol))

		if indicators.EnableOI && data.OpenInterest != nil {
			sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n",
				data.OpenInterest.Latest, data.OpenInterest.Average))
		}

		if indicators.EnableFundingRate {
			sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
		}
	}

	// 优先使用多时间周期数据（新增）
	if len(data.TimeframeData) > 0 {
		// 按时间周期排序输出
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				sb.WriteString(fmt.Sprintf("=== %s Timeframe (oldest → latest) ===\n\n", strings.ToUpper(tf)))
				e.formatTimeframeSeriesData(&sb, tfData, indicators)
			}
		}
	} else {
		// 兼容旧的数据格式
		// 日内数据
		if data.IntradaySeries != nil {
			klineConfig := indicators.Klines
			sb.WriteString(fmt.Sprintf("Intraday series (%s intervals, oldest → latest):\n\n", klineConfig.PrimaryTimeframe))

			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}

			if indicators.EnableEMA && len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA indicators (20-period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}

			if indicators.EnableMACD && len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}

			if indicators.EnableRSI {
				if len(data.IntradaySeries.RSI7Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (7-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
				}
				if len(data.IntradaySeries.RSI14Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
				}
			}

			if indicators.EnableVolume && len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3m ATR (14-period): %.3f\n\n", data.IntradaySeries.ATR14))
			}
		}

		// 长周期数据
		if data.LongerTermContext != nil && indicators.Klines.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf("Longer-term context (%s timeframe):\n\n", indicators.Klines.LongerTimeframe))

			if indicators.EnableEMA {
				sb.WriteString(fmt.Sprintf("20-Period EMA: %.3f vs. 50-Period EMA: %.3f\n\n",
					data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3-Period ATR: %.3f vs. 14-Period ATR: %.3f\n\n",
					data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))
			}

			if indicators.EnableVolume {
				sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
					data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))
			}

			if indicators.EnableMACD && len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}

			if indicators.EnableRSI && len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}
		}
	}

	return sb.String()
}

// formatTimeframeSeriesData 格式化单个时间周期的序列数据
func (e *StrategyEngine) formatTimeframeSeriesData(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig) {
	if len(data.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.MidPrices)))
	}

	if indicators.EnableEMA {
		if len(data.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20-period): %s\n\n", formatFloatSlice(data.EMA20Values)))
		}
		if len(data.EMA50Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (50-period): %s\n\n", formatFloatSlice(data.EMA50Values)))
		}
	}

	if indicators.EnableMACD && len(data.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.MACDValues)))
	}

	if indicators.EnableRSI {
		if len(data.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7-Period): %s\n\n", formatFloatSlice(data.RSI7Values)))
		}
		if len(data.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.RSI14Values)))
		}
	}

	if indicators.EnableVolume && len(data.Volume) > 0 {
		sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.Volume)))
	}

	if indicators.EnableATR {
		sb.WriteString(fmt.Sprintf("ATR (14-period): %.3f\n\n", data.ATR14))
	}
}

// formatFloatSlice 格式化浮点数切片
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.4f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// BuildSystemPrompt 根据策略配置构建 System Prompt
func (e *StrategyEngine) BuildSystemPrompt(accountEquity float64, variant string) string {
	var sb strings.Builder
	riskControl := e.config.RiskControl
	promptSections := e.config.PromptSections

	// 1. 角色定义（可编辑）
	if promptSections.RoleDefinition != "" {
		sb.WriteString(promptSections.RoleDefinition)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# 你是专业的加密货币交易AI\n\n")
		sb.WriteString("你的任务是根据提供的市场数据做出交易决策。\n\n")
	}

	// 2. 交易模式变体
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		sb.WriteString("## 模式：Aggressive（进攻型）\n- 优先捕捉趋势突破，可在信心度≥70时分批建仓\n- 允许更高仓位，但须严格设置止损并说明盈亏比\n\n")
	case "conservative":
		sb.WriteString("## 模式：Conservative（稳健型）\n- 仅在多重信号共振时开仓\n- 优先保留现金，连续亏损必须暂停多个周期\n\n")
	case "scalping":
		sb.WriteString("## 模式：Scalping（剥头皮）\n- 聚焦短周期动量，目标收益较小但要求迅速\n- 若价格两根bar内未按预期运行，立即减仓或止损\n\n")
	}

	// 3. 硬约束（风险控制）- 来自策略配置（不可编辑，自动生成）
	sb.WriteString("# 硬约束（风险控制）\n\n")
	sb.WriteString(fmt.Sprintf("1. 风险回报比: 必须 ≥ 1:%.1f\n", riskControl.MinRiskRewardRatio))
	sb.WriteString(fmt.Sprintf("2. 最多持仓: %d个币种（质量>数量）\n", riskControl.MaxPositions))
	sb.WriteString(fmt.Sprintf("3. 单币仓位: 山寨%.0f-%.0f U | BTC/ETH %.0f-%.0f U\n",
		accountEquity*0.8, accountEquity*riskControl.MaxPositionRatio,
		accountEquity*5, accountEquity*10))
	sb.WriteString(fmt.Sprintf("4. 杠杆限制: **山寨币最大%dx杠杆** | **BTC/ETH最大%dx杠杆**\n",
		riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
	sb.WriteString(fmt.Sprintf("5. 保证金使用率 ≤ %.0f%%\n", riskControl.MaxMarginUsage*100))
	sb.WriteString(fmt.Sprintf("6. 开仓金额: 建议 ≥%.0f USDT\n", riskControl.MinPositionSize))
	sb.WriteString(fmt.Sprintf("7. 最小信心度: ≥%d\n\n", riskControl.MinConfidence))

	// 4. 交易频率与信号质量（可编辑）
	if promptSections.TradingFrequency != "" {
		sb.WriteString(promptSections.TradingFrequency)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# ⏱️ 交易频率认知\n\n")
		sb.WriteString("- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔\n")
		sb.WriteString("- 每小时>2笔 = 过度交易\n")
		sb.WriteString("- 单笔持仓时间≥30-60分钟\n")
		sb.WriteString("如果你发现自己每个周期都在交易 → 标准过低；若持仓<30分钟就平仓 → 过于急躁。\n\n")
	}

	// 5. 开仓标准（可编辑）
	if promptSections.EntryStandards != "" {
		sb.WriteString(promptSections.EntryStandards)
		sb.WriteString("\n\n你拥有以下指标数据：\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\n**信心度 ≥%d** 才能开仓。\n\n", riskControl.MinConfidence))
	} else {
		sb.WriteString("# 🎯 开仓标准（严格）\n\n")
		sb.WriteString("只在多重信号共振时开仓。你拥有：\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\n自由运用任何有效的分析方法，但**信心度 ≥%d** 才能开仓；避免单一指标、信号矛盾、横盘震荡、刚平仓即重启等低质量行为。\n\n", riskControl.MinConfidence))
	}

	// 6. 决策流程提示（可编辑）
	if promptSections.DecisionProcess != "" {
		sb.WriteString(promptSections.DecisionProcess)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# 📋 决策流程\n\n")
		sb.WriteString("1. 检查持仓 → 是否该止盈/止损\n")
		sb.WriteString("2. 扫描候选币 + 多时间框 → 是否存在强信号\n")
		sb.WriteString("3. 先写思维链，再输出结构化JSON\n\n")
	}

	// 7. 输出格式
	sb.WriteString("# 输出格式 (严格遵守)\n\n")
	sb.WriteString("**必须使用XML标签 <reasoning> 和 <decision> 标签分隔思维链和决策JSON，避免解析错误**\n\n")
	sb.WriteString("## 格式要求\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("你的思维链分析...\n")
	sb.WriteString("- 简洁分析你的思考过程 \n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("第二步: JSON决策数组\n\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300},\n",
		riskControl.BTCETHMaxLeverage, accountEquity*5))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\"}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## 字段说明\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString(fmt.Sprintf("- `confidence`: 0-100（开仓建议≥%d）\n", riskControl.MinConfidence))
	sb.WriteString("- 开仓时必填: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n\n")

	// 8. 自定义 Prompt
	if e.config.CustomPrompt != "" {
		sb.WriteString("# 📌 个性化交易策略\n\n")
		sb.WriteString(e.config.CustomPrompt)
		sb.WriteString("\n\n")
		sb.WriteString("注意: 以上个性化策略是对基础规则的补充，不能违背基础风险控制原则。\n")
	}

	return sb.String()
}

// writeAvailableIndicators 写入可用指标列表
func (e *StrategyEngine) writeAvailableIndicators(sb *strings.Builder) {
	indicators := e.config.Indicators
	kline := indicators.Klines

	sb.WriteString(fmt.Sprintf("- %s价格序列", kline.PrimaryTimeframe))
	if kline.EnableMultiTimeframe {
		sb.WriteString(fmt.Sprintf(" + %s K线序列\n", kline.LongerTimeframe))
	} else {
		sb.WriteString("\n")
	}

	if indicators.EnableEMA {
		sb.WriteString("- EMA 指标")
		if len(indicators.EMAPeriods) > 0 {
			sb.WriteString(fmt.Sprintf("（周期: %v）", indicators.EMAPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableMACD {
		sb.WriteString("- MACD 指标\n")
	}

	if indicators.EnableRSI {
		sb.WriteString("- RSI 指标")
		if len(indicators.RSIPeriods) > 0 {
			sb.WriteString(fmt.Sprintf("（周期: %v）", indicators.RSIPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableATR {
		sb.WriteString("- ATR 指标")
		if len(indicators.ATRPeriods) > 0 {
			sb.WriteString(fmt.Sprintf("（周期: %v）", indicators.ATRPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableVolume {
		sb.WriteString("- 成交量数据\n")
	}

	if indicators.EnableOI {
		sb.WriteString("- 持仓量(OI)数据\n")
	}

	if indicators.EnableFundingRate {
		sb.WriteString("- 资金费率\n")
	}

	if len(e.config.CoinSource.StaticCoins) > 0 || e.config.CoinSource.UseCoinPool || e.config.CoinSource.UseOITop {
		sb.WriteString("- AI500 / OI_Top 筛选标签（若有）\n")
	}

	if indicators.EnableQuantData {
		sb.WriteString("- 量化数据（机构/散户资金流向、持仓变化、多周期价格变化）\n")
	}
}

// GetRiskControlConfig 获取风险控制配置
func (e *StrategyEngine) GetRiskControlConfig() store.RiskControlConfig {
	return e.config.RiskControl
}

// GetConfig 获取完整策略配置
func (e *StrategyEngine) GetConfig() *store.StrategyConfig {
	return e.config
}
