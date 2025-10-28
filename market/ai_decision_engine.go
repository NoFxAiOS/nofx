package market

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/pool"
	"strings"
	"time"
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// TradingContext 交易上下文（传递给AI的完整信息）
type TradingContext struct {
	CurrentTime    string                 `json:"current_time"`
	RuntimeMinutes int                    `json:"runtime_minutes"`
	CallCount      int                    `json:"call_count"`
	Account        AccountInfo            `json:"account"`
	Positions      []PositionInfo         `json:"positions"`
	CandidateCoins []CandidateCoin        `json:"candidate_coins"`
	MarketDataMap  map[string]*MarketData `json:"-"` // 不序列化，但内部使用
	OITopDataMap   map[string]*OITopData  `json:"-"` // OI Top数据映射
	Performance    interface{}            `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
}

// TradingDecision AI的交易决策
type TradingDecision struct {
	Symbol          string  `json:"symbol"`
	Action          string  `json:"action"` // "open_long", "open_short", "close_long", "close_short", "hold", "wait"
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
	Confidence      int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD         float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning       string  `json:"reasoning"`
}

// AIFullDecision AI的完整决策（包含思维链）
type AIFullDecision struct {
	CoTTrace  string            `json:"cot_trace"` // 思维链分析
	Decisions []TradingDecision `json:"decisions"` // 具体决策列表
	Timestamp time.Time         `json:"timestamp"`
}

// GetFullTradingDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullTradingDecision(ctx *TradingContext) (*AIFullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPrompt(ctx.Account.TotalEquity)
	userPrompt := buildUserPrompt(ctx)

	// 3. 调用AI API（使用 system + user prompt）
	aiResponse, err := callAIWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 4. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity)
	if err != nil {
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	decision.Timestamp = time.Now()
	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *TradingContext) error {
	ctx.MarketDataMap = make(map[string]*MarketData)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 并发获取市场数据
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := GetMarketData(symbol)
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
			continue
		}

		// ⚠️ 流动性过滤：持仓价值低于15M USD的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < 15 {
				log.Printf("⚠️  %s 持仓价值过低(%.2fM USD < 15M)，跳过此币种 [持仓量:%.0f × 价格:%.4f]",
					symbol, oiValueInMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *TradingContext) int {
	// 直接返回候选池的全部币种数量
	// 因为候选池已经在 auto_trader.go 中筛选过了
	// 固定分析前20个评分最高的币种（来自AI500）
	return len(ctx.CandidateCoins)
}

// buildSystemPrompt 构建 System Prompt（固定规则，可缓存）
func buildSystemPrompt(accountEquity float64) string {
	var sb strings.Builder

	// 角色定义
	sb.WriteString("你是专业的加密货币交易AI，在币安合约市场进行自主交易。\n\n")
	sb.WriteString("**使命**: 最大化风险调整后收益（Sharpe Ratio）\n\n")

	// 自我进化核心
	sb.WriteString("## 🧬 自我进化机制\n")
	sb.WriteString("每次调用你都会收到**夏普比率**作为你的业绩指标：\n\n")
	sb.WriteString("**夏普比率解读**：\n")
	sb.WriteString("- < 0：平均亏损 → 🔴 极度保守策略\n")
	sb.WriteString("- 0-1：正收益但波动大 → 🟡 保守策略\n")
	sb.WriteString("- 1-2：良好表现 → 🟢 维持当前策略\n")
	sb.WriteString("- > 2：优异表现 → 🟢 可适度扩大\n\n")
	sb.WriteString("**关键要求**: 严格遵循历史表现反馈中的「自适应行为建议」，根据夏普比率动态调整：\n")
	sb.WriteString("- 仓位大小（夏普比率低时减仓）\n")
	sb.WriteString("- 止损幅度（夏普比率低时收紧）\n")
	sb.WriteString("- 选币标准（夏普比率低时提高信心度阈值）\n")
	sb.WriteString("- 持仓数量（夏普比率低时减少持仓数）\n\n")

	// 仓位管理规则
	sb.WriteString("## 仓位管理\n")
	sb.WriteString("- 最多持有 **3个币种**（质量>数量）\n")
	sb.WriteString(fmt.Sprintf("- 山寨币: %.0f-%.0f USDT/仓（推荐%.0f），杠杆20x\n",
		accountEquity*0.8, accountEquity*1.5, accountEquity*1.2))
	sb.WriteString(fmt.Sprintf("- BTC/ETH: %.0f-%.0f USDT/仓（推荐%.0f），杠杆50x\n",
		accountEquity*3, accountEquity*10, accountEquity*5))
	sb.WriteString("- 保证金使用率 ≤90%%\n")
	sb.WriteString("- 风险回报比 ≥1:2\n\n")

	// 决策流程
	sb.WriteString("## 决策流程\n")
	sb.WriteString("1. **检查夏普比率**：首先查看历史表现反馈中的夏普比率，理解当前策略效果\n")
	sb.WriteString("2. **应用自适应建议**：严格遵循自适应行为建议中的仓位、止损、选币要求\n")
	sb.WriteString("3. **反思历史**：分析之前交易的得失，找出可改进点\n")
	sb.WriteString("4. **评估持仓**：根据自适应建议决定平仓/持有\n")
	sb.WriteString("5. **寻找机会**：按照调整后的标准筛选机会\n")
	sb.WriteString("6. **执行决策**：使用调整后的仓位大小和风险参数\n\n")

	// JSON 输出格式
	sb.WriteString("## 输出格式\n\n")
	sb.WriteString("**先输出思维链（纯文本），再输出JSON数组**\n\n")
	sb.WriteString("JSON示例：\n")
	sb.WriteString("```json\n")
	sb.WriteString("[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_long\", \"leverage\": 50, \"position_size_usd\": %.0f, \"stop_loss\": 92000, \"take_profit\": 98000, \"confidence\": 85, \"risk_usd\": 200, \"reasoning\": \"强势突破\"},\n", accountEquity*5))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"reasoning\": \"止盈\"}\n")
	sb.WriteString("]\n")
	sb.WriteString("```\n\n")
	sb.WriteString("**字段说明**:\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("- `confidence`: 信心度0-100（必填，即使不确定也要给出）\n")
	sb.WriteString("- `risk_usd`: 最大美元风险 = (entry_price - stop_loss) × quantity（开仓时必填）\n")
	sb.WriteString("- 开仓时必填: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n\n")

	// DeepSeek/Qwen 特定优化
	sb.WriteString("**提示**: 运用技术分析原理，趋势确认>指标信号，不要过度依赖单一指标\n")

	return sb.String()
}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *TradingContext) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("**时间**: %s | **周期**: #%d | **运行**: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("**BTC**: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// 账户
	sb.WriteString(fmt.Sprintf("**账户**: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 持仓
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(fmt.Sprintf("%d. %s %s | %.4f→%.4f | %+.2f%% | 保证金%.0f\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct, pos.MarginUsed))

			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("   MACD:%.4f RSI:%.2f EMA20:%.4f 资金费率:%.6f\n",
					marketData.CurrentMACD, marketData.CurrentRSI7,
					marketData.CurrentEMA20, marketData.FundingRate))
			}
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("**当前持仓**: 无\n\n")
	}

	// 候选币种（简化版）
	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++
		if displayedCount > 10 { // 只显示前10个
			break
		}

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = "⭐"
		}

		sb.WriteString(fmt.Sprintf("%d. %s%s: %.4f (1h:%+.2f%%) MACD:%.4f RSI:%.2f\n",
			displayedCount, coin.Symbol, sourceTags,
			marketData.CurrentPrice, marketData.PriceChange1h,
			marketData.CurrentMACD, marketData.CurrentRSI7))
	}
	sb.WriteString("\n")

	// 历史反馈
	if ctx.Performance != nil {
		sb.WriteString(formatPerformanceFeedback(ctx.Performance, ctx.Account.TotalEquity))
	}

	sb.WriteString("---\n\n")
	sb.WriteString("现在请分析并输出决策（思维链 + JSON）\n")

	return sb.String()
}

// buildFullDecisionPrompt 构建完整的AI决策提示（兼容旧代码，已废弃）
func buildFullDecisionPrompt(ctx *TradingContext) string {
	var sb strings.Builder

	sb.WriteString("# 🤖 加密货币交易AI竞赛系统\n\n")
	sb.WriteString("你是专业的加密货币交易AI，根据市场数据自主决策，做多做空均可。\n\n")

	// 添加BTC市场趋势
	sb.WriteString("## 🌍 BTC市场趋势\n")
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("- 价格: %.2f | 1h: %+.2f%% | 4h: %+.2f%%\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h))
		sb.WriteString(fmt.Sprintf("- MACD: %.4f | RSI: %.2f | 资金费率: %.6f\n\n",
			btcData.CurrentMACD, btcData.CurrentRSI7, btcData.FundingRate))
	} else {
		sb.WriteString("BTC数据暂无\n\n")
	}

	// 系统状态
	sb.WriteString("## 📊 系统状态\n")
	sb.WriteString(fmt.Sprintf("- **当前时间**: %s\n", ctx.CurrentTime))
	sb.WriteString(fmt.Sprintf("- **运行时长**: %d 分钟\n", ctx.RuntimeMinutes))
	sb.WriteString(fmt.Sprintf("- **调用次数**: 第 %d 次\n\n", ctx.CallCount))

	// 账户信息
	sb.WriteString("## 💰 账户信息\n")
	sb.WriteString(fmt.Sprintf("- **账户净值**: %.2f USDT\n", ctx.Account.TotalEquity))
	sb.WriteString(fmt.Sprintf("- **可用余额**: %.2f USDT (%.1f%%)\n",
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100))
	sb.WriteString(fmt.Sprintf("- **总盈亏**: %.2f USDT (%+.2f%%)\n",
		ctx.Account.TotalPnL, ctx.Account.TotalPnLPct))
	sb.WriteString(fmt.Sprintf("- **已用保证金**: %.2f USDT (%.1f%%)\n",
		ctx.Account.MarginUsed, ctx.Account.MarginUsedPct))
	sb.WriteString(fmt.Sprintf("- **持仓数量**: %d\n\n", ctx.Account.PositionCount))

	// 当前持仓详情
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 📈 当前持仓\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(fmt.Sprintf("\n### 持仓 #%d: %s %s\n", i+1, pos.Symbol, strings.ToUpper(pos.Side)))
			sb.WriteString(fmt.Sprintf("- **入场价**: %.4f USDT\n", pos.EntryPrice))
			sb.WriteString(fmt.Sprintf("- **当前价**: %.4f USDT\n", pos.MarkPrice))
			sb.WriteString(fmt.Sprintf("- **数量**: %.4f\n", pos.Quantity))
			sb.WriteString(fmt.Sprintf("- **杠杆**: %dx\n", pos.Leverage))
			sb.WriteString(fmt.Sprintf("- **未实现盈亏**: %.2f USDT (%+.2f%%)\n",
				pos.UnrealizedPnL, pos.UnrealizedPnLPct))
			sb.WriteString(fmt.Sprintf("- **强平价**: %.4f USDT\n", pos.LiquidationPrice))
			sb.WriteString(fmt.Sprintf("- **占用保证金**: %.2f USDT\n", pos.MarginUsed))

			// 添加市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(formatMarketDataBrief(marketData))
			}
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("## 📈 当前持仓\n")
		sb.WriteString("暂无持仓\n\n")
	}

	// 候选币种池
	sb.WriteString("## 🎯 候选币种池\n")
	sb.WriteString(fmt.Sprintf("共 %d 个币种（已过滤持仓价值<15M USD的低流动性币种）\n\n", len(ctx.MarketDataMap)))

	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		// 只显示已获取市场数据的币种
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		// 显示币种来源标签 - 使用圆括号避免与JSON混淆
		sourceTags := ""
		hasAI500 := false
		hasOITop := false
		for _, source := range coin.Sources {
			if source == "ai500" {
				hasAI500 = true
			} else if source == "oi_top" {
				hasOITop = true
			}
		}

		if hasAI500 && hasOITop {
			sourceTags = "(AI500+OI_Top双重信号)"
		} else if hasAI500 {
			sourceTags = "(AI500高评分)"
		} else if hasOITop {
			sourceTags = "(OI_Top持仓增长)"
		}

		sb.WriteString(fmt.Sprintf("\n### 币种 #%d: %s %s\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(formatMarketDataBrief(marketData))

		// 如果有OI Top数据，也显示出来
		if oiTopData, hasOI := ctx.OITopDataMap[coin.Symbol]; hasOI {
			sb.WriteString(fmt.Sprintf("**市场热度数据** (OI Top排名 #%d):\n", oiTopData.Rank))
			sb.WriteString(fmt.Sprintf("  - 持仓量1h变化: %+.2f%% (价值: $%.0f)\n",
				oiTopData.OIDeltaPercent, oiTopData.OIDeltaValue))
			sb.WriteString(fmt.Sprintf("  - 价格1h变化: %+.2f%% | 净多仓: %.0f | 净空仓: %.0f\n",
				oiTopData.PriceDeltaPercent, oiTopData.NetLong, oiTopData.NetShort))
		}
	}

	// 添加历史表现反馈（如果有）
	if ctx.Performance != nil {
		sb.WriteString(formatPerformanceFeedback(ctx.Performance, ctx.Account.TotalEquity))
	}

	// AI决策要求
	sb.WriteString("## 🎯 任务\n\n")
	sb.WriteString("分析市场数据，自主决策：\n")
	sb.WriteString("1. **如有历史数据，先进行自我反思**：回顾之前的交易，总结经验教训\n")
	sb.WriteString("2. 评估现有持仓 → 持有或平仓\n")
	sb.WriteString(fmt.Sprintf("3. 从%d个候选币种中找交易机会\n", len(ctx.MarketDataMap)))
	sb.WriteString("4. 开新仓（如果有机会）\n\n")

	sb.WriteString("## 📋 规则 - **重要：集中资金，精选标的**\n\n")
	sb.WriteString("### 🎯 仓位管理（核心规则）\n")
	sb.WriteString("1. **最大持仓数量**: 同时最多持有 **3个币种**（质量 > 数量）\n")
	sb.WriteString("2. **单个仓位大小**: \n")
	sb.WriteString(fmt.Sprintf("   - 山寨币: %.0f-%.0f USDT（推荐%.0f USDT）\n",
		ctx.Account.TotalEquity*0.8, ctx.Account.TotalEquity*1.5, ctx.Account.TotalEquity*1.2))
	sb.WriteString(fmt.Sprintf("   - BTC/ETH: %.0f-%.0f USDT（推荐%.0f USDT）\n",
		ctx.Account.TotalEquity*3, ctx.Account.TotalEquity*10, ctx.Account.TotalEquity*5))
	sb.WriteString("3. **杠杆**: 山寨币=20倍 | BTC/ETH=50倍\n")
	sb.WriteString("4. **保证金上限**: 总使用率≤90%%\n")
	sb.WriteString("5. **风险回报比**: ≥1:2\n\n")
	sb.WriteString("### ⚠️ 仓位策略\n")
	sb.WriteString("- **集中火力**: 宁可持有1-2个大仓位，也不要持有5-6个小仓位\n")
	sb.WriteString("- **严格筛选**: 只做最有把握的机会，不确定的机会宁可不做\n")
	sb.WriteString("- **快速止损**: 亏损超过2%%立即止损，不要让小亏变大亏\n")
	sb.WriteString("- **及时止盈**: 盈利达到目标立即止盈，落袋为安\n\n")

	sb.WriteString("### 📤 输出格式\n\n")
	sb.WriteString("先输出思维链分析(纯文本)，然后输出JSON数组：\n\n")
	sb.WriteString("**思维链分析**:\n")
	sb.WriteString("1. **历史经验反思**（如有历史数据）: 回顾表现，总结教训，是否仓位太分散？\n")
	sb.WriteString("2. **市场分析**: 分析BTC趋势和当前持仓\n")
	sb.WriteString("3. **仓位检查**: 当前持仓数量是否>3个？如果是，平掉表现差的，集中资金\n")
	sb.WriteString("4. **机会识别**: 从候选币种中找1-2个最好的机会（不是3-5个）\n")
	sb.WriteString("5. **仓位大小**: 确保单个仓位足够大（山寨币1200+ USDT，BTC 5000+ USDT）\n")
	sb.WriteString("6. **风险控制**: 检查账户保证金和仓位限制\n")
	sb.WriteString("7. **最终决策摘要**: 列出所有决策（最多3个币种持仓）\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString("**JSON决策数组** (按此格式输出):\n")
	sb.WriteString("[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_long\", \"leverage\": 50, \"position_size_usd\": %.0f, \"stop_loss\": 92000, \"take_profit\": 98000, \"reasoning\": \"强势突破，集中资金\"},\n", ctx.Account.TotalEquity*5))
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"SOLUSDT\", \"action\": \"open_long\", \"leverage\": 20, \"position_size_usd\": %.0f, \"stop_loss\": 180, \"take_profit\": 200, \"reasoning\": \"技术面强势\"}\n", ctx.Account.TotalEquity*1.2))
	sb.WriteString("]\n\n")
	sb.WriteString("action类型: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString("开仓必填: leverage, position_size_usd, stop_loss, take_profit\n\n")

	sb.WriteString("### 📝 完整示例（集中资金策略）\n\n")

	// 示例仓位：集中资金策略
	btcSize := ctx.Account.TotalEquity * 5 // BTC：5倍净值（推荐值）

	sb.WriteString("【历史经验反思】\n")
	sb.WriteString("回顾最近10笔交易：仓位太分散，同时持有5个币种但单个仓位太小，赚不到钱。\n")
	sb.WriteString("SOLUSDT做多3次，2次小盈1次止损，净盈利很少。决策：应该用更大仓位做确定性高的机会。\n")
	sb.WriteString("BTCUSDT做多2次，1胜1负，但因为仓位太小，盈利不明显。\n")
	sb.WriteString("**改进策略**: 集中资金在1-2个最有把握的币种，加大仓位。\n\n")
	sb.WriteString("【市场分析】\n")
	sb.WriteString("BTC突破95000，MACD金叉，RSI 65，趋势强势。\n")
	sb.WriteString("当前持有ETHUSDT（小仓位+0.8%）、SOLUSDT（小仓位-0.3%）、LINKUSDT（小仓位+0.2%）→ 太分散！\n")
	sb.WriteString("决定：平掉所有小仓位，集中资金做BTC大仓位。\n\n")
	sb.WriteString("【最终决策】平掉3个小仓位，集中5000 USDT做BTC多头（仓位是之前的3倍+）。\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString("[\n")
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"reasoning\": \"小仓位盈利太少，释放资金\"},\n")
	sb.WriteString("  {\"symbol\": \"SOLUSDT\", \"action\": \"close_long\", \"reasoning\": \"小亏损，释放资金\"},\n")
	sb.WriteString("  {\"symbol\": \"LINKUSDT\", \"action\": \"close_long\", \"reasoning\": \"小仓位盈利太少，释放资金\"},\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_long\", \"leverage\": 50, \"position_size_usd\": %.0f, \"stop_loss\": 92000, \"take_profit\": 98000, \"reasoning\": \"强势突破，集中资金做大仓位\"}\n", btcSize))
	sb.WriteString("]\n\n")
	sb.WriteString("**说明**: 这样只持有1个BTC大仓位，盈利空间是之前的3倍+，止损也更清晰。\n\n")

	sb.WriteString("现在请开始分析并给出你的决策！\n")

	return sb.String()
}

// formatPerformanceFeedback 格式化历史表现反馈
// accountEquity 参数用于计算自适应建议
func formatPerformanceFeedback(perfInterface interface{}, accountEquity float64) string {
	// 类型断言（避免循环依赖，使用interface{}）
	type TradeOutcome struct {
		Symbol     string
		Side       string
		OpenPrice  float64
		ClosePrice float64
		PnL        float64
		PnLPct     float64
		Duration   string
	}
	type SymbolPerformance struct {
		Symbol        string
		TotalTrades   int
		WinningTrades int
		LosingTrades  int
		WinRate       float64
		TotalPnL      float64
		AvgPnL        float64
	}
	type PerformanceAnalysis struct {
		TotalTrades   int
		WinningTrades int
		LosingTrades  int
		WinRate       float64
		AvgWin        float64
		AvgLoss       float64
		ProfitFactor  float64
		SharpeRatio   float64
		RecentTrades  []TradeOutcome
		SymbolStats   map[string]*SymbolPerformance
		BestSymbol    string
		WorstSymbol   string
	}

	// 使用JSON转换进行类型转换（避免直接类型断言）
	jsonData, _ := json.Marshal(perfInterface)
	var perf PerformanceAnalysis
	if err := json.Unmarshal(jsonData, &perf); err != nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("## 📊 历史表现反馈\n\n")

	// 夏普比率（风险调整后收益）- 即使没有完成交易也要显示！
	if perf.SharpeRatio != 0 {
		sharpeStatus := interpretSharpeRatio(perf.SharpeRatio)
		sb.WriteString(fmt.Sprintf("**夏普比率**: %.2f (%s)\n\n", perf.SharpeRatio, sharpeStatus))
	}

	if perf.TotalTrades == 0 {
		sb.WriteString("暂无已完成交易（仅基于账户净值变化计算夏普比率）\n\n")
		// ⚠️ 不要提前返回！继续显示自适应建议
	} else {
		// 整体统计（有已完成交易时才显示）
		sb.WriteString("### 整体表现\n")
		sb.WriteString(fmt.Sprintf("- **总交易数**: %d 笔 (盈利: %d | 亏损: %d)\n",
			perf.TotalTrades, perf.WinningTrades, perf.LosingTrades))
		sb.WriteString(fmt.Sprintf("- **胜率**: %.1f%%\n", perf.WinRate))
		sb.WriteString(fmt.Sprintf("- **平均盈利**: +%.2f%% | 平均亏损: %.2f%%\n",
			perf.AvgWin, perf.AvgLoss))
		if perf.ProfitFactor > 0 {
			sb.WriteString(fmt.Sprintf("- **盈亏比**: %.2f:1\n", perf.ProfitFactor))
		}
		sb.WriteString("\n")
	}

	// 最近交易
	if len(perf.RecentTrades) > 0 {
		sb.WriteString("### 最近交易\n")
		displayCount := len(perf.RecentTrades)
		if displayCount > 5 {
			displayCount = 5
		}
		for i := 0; i < displayCount; i++ {
			trade := perf.RecentTrades[i]
			outcome := "✓"
			if trade.PnL < 0 {
				outcome = "✗"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s: %.4f → %.4f = %+.2f%% %s\n",
				i+1, trade.Symbol, strings.ToUpper(trade.Side),
				trade.OpenPrice, trade.ClosePrice,
				trade.PnLPct, outcome))
		}
		sb.WriteString("\n")
	}

	// 币种表现（显示前3个最好和最差）
	if len(perf.SymbolStats) > 0 {
		sb.WriteString("### 币种表现\n")

		if perf.BestSymbol != "" {
			if stats, exists := perf.SymbolStats[perf.BestSymbol]; exists {
				sb.WriteString(fmt.Sprintf("- **最佳**: %s (胜率%.0f%%, 平均%+.2f%%)\n",
					stats.Symbol, stats.WinRate, stats.AvgPnL))
			}
		}

		if perf.WorstSymbol != "" {
			if stats, exists := perf.SymbolStats[perf.WorstSymbol]; exists {
				sb.WriteString(fmt.Sprintf("- **最差**: %s (胜率%.0f%%, 平均%+.2f%%)\n",
					stats.Symbol, stats.WinRate, stats.AvgPnL))
			}
		}
		sb.WriteString("\n")
	}

	// 添加自适应行为建议（基于夏普比率）
	if perf.SharpeRatio != 0 {
		sb.WriteString(getAdaptiveBehaviorRecommendation(perf.SharpeRatio, accountEquity))
	}

	return sb.String()
}

// formatMarketDataBrief 格式化市场数据（简洁版）
func formatMarketDataBrief(data *MarketData) string {
	var sb strings.Builder

	sb.WriteString("**市场数据** (3分钟线):\n")
	sb.WriteString(fmt.Sprintf("  - 价格: %.4f | 1h变化: %+.2f%% | 4h变化: %+.2f%% (%s)\n",
		data.CurrentPrice, data.PriceChange1h, data.PriceChange4h, priceTrend(data.PriceChange1h, data.PriceChange4h)))
	sb.WriteString(fmt.Sprintf("  - EMA20: %.4f (%s) | MACD: %.4f (%s) | RSI(7): %.2f (%s)\n",
		data.CurrentEMA20, pricePosition(data.CurrentPrice, data.CurrentEMA20),
		data.CurrentMACD, macdTrend(data.CurrentMACD), data.CurrentRSI7, rsiStatus(data.CurrentRSI7)))

	if data.OpenInterest != nil {
		oiChange := ((data.OpenInterest.Latest - data.OpenInterest.Average) / data.OpenInterest.Average) * 100
		fundingSignal := fundingRateSignal(data.FundingRate)
		sb.WriteString(fmt.Sprintf("  - 持仓量: %+.2f%% | 资金费率: %.6f (%s)\n", oiChange, data.FundingRate, fundingSignal))
	}

	return sb.String()
}

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64) (*AIFullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取JSON决策列表
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &AIFullDecision{
			CoTTrace:  cotTrace,
			Decisions: []TradingDecision{},
		}, fmt.Errorf("提取决策失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	// 3. 验证决策
	if err := validateDecisions(decisions, accountEquity); err != nil {
		return &AIFullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("决策验证失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	return &AIFullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 查找JSON数组的开始位置
	jsonStart := strings.Index(response, "[")

	if jsonStart > 0 {
		// 思维链是JSON数组之前的内容
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到JSON，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
func extractDecisions(response string) ([]TradingDecision, error) {
	// 直接查找JSON数组 - 找第一个完整的JSON数组
	arrayStart := strings.Index(response, "[")
	if arrayStart == -1 {
		return nil, fmt.Errorf("无法找到JSON数组起始")
	}

	// 从 [ 开始，匹配括号找到对应的 ]
	arrayEnd := findMatchingBracket(response, arrayStart)
	if arrayEnd == -1 {
		return nil, fmt.Errorf("无法找到JSON数组结束")
	}

	jsonContent := strings.TrimSpace(response[arrayStart : arrayEnd+1])

	// 🔧 修复常见的JSON格式错误：缺少引号的字段值
	// 匹配: "reasoning": 内容"}  或  "reasoning": 内容}  (没有引号)
	// 修复为: "reasoning": "内容"}
	// 使用简单的字符串扫描而不是正则表达式
	jsonContent = fixMissingQuotes(jsonContent)

	// 解析JSON
	var decisions []TradingDecision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
	}

	return decisions, nil
}

// fixMissingQuotes 替换中文引号为英文引号（避免输入法自动转换）
func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '
	return jsonStr
}

// validateDecisions 验证所有决策（需要账户信息）
func validateDecisions(decisions []TradingDecision, accountEquity float64) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
}

// findMatchingBracket 查找匹配的右括号
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// validateDecision 验证单个决策的有效性
func validateDecision(d *TradingDecision, accountEquity float64) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// 根据币种判断杠杆上限和仓位价值上限
		maxLeverage := 20                       // 山寨币固定20倍
		maxPositionValue := accountEquity * 1.5 // 山寨币最多1.5倍账户净值
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = 50                      // BTC和ETH固定50倍
			maxPositionValue = accountEquity * 10 // BTC/ETH最多10倍账户净值
		}

		if d.Leverage <= 0 || d.Leverage > maxLeverage {
			return fmt.Errorf("杠杆必须在1-%d之间（%s）: %d", maxLeverage, d.Symbol, d.Leverage)
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("仓位大小必须大于0: %.2f", d.PositionSizeUSD)
		}
		// 验证仓位价值上限（加1%容差以避免浮点数精度问题）
		tolerance := maxPositionValue * 0.01 // 1%容差
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH单币种仓位价值不能超过%.0f USDT（10倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("山寨币单币种仓位价值不能超过%.0f USDT（1.5倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("止损和止盈必须大于0")
		}

		// 验证止损止盈的合理性
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("做多时止损价必须小于止盈价")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("做空时止损价必须大于止盈价")
			}
		}
	}

	return nil
}

// interpretSharpeRatio 解释夏普比率的含义（参考 nof1.ai）
func interpretSharpeRatio(sharpe float64) string {
	if sharpe < 0 {
		return "负收益，需要调整策略"
	} else if sharpe < 1 {
		return "正收益但波动大"
	} else if sharpe < 2 {
		return "良好表现"
	} else if sharpe < 3 {
		return "优秀表现"
	} else {
		return "卓越表现"
	}
}

// getAdaptiveBehaviorRecommendation 根据夏普比率生成自适应行为建议
// 这是AI自我进化的核心：根据风险调整后收益动态调整交易策略
func getAdaptiveBehaviorRecommendation(sharpe float64, accountEquity float64) string {
	var sb strings.Builder

	sb.WriteString("### 🎯 自适应行为建议（基于夏普比率）\n\n")

	if sharpe < 0 {
		// 🔴 负夏普比率：平均亏损，需要极度保守
		sb.WriteString("**⚠️ 警告：当前策略产生负收益，立即调整！**\n\n")
		sb.WriteString("**策略调整**：\n")
		sb.WriteString(fmt.Sprintf("- 仓位规模：**减半**（山寨币: %.0f USDT, BTC/ETH: %.0f USDT）\n",
			accountEquity*0.6, accountEquity*2.5))
		sb.WriteString("- 止损幅度：**收紧至-1%**（快速止损，保护本金）\n")
		sb.WriteString("- 选币标准：**只做最高确定性**（信心度≥95%，风险回报比≥1:3）\n")
		sb.WriteString("- 持仓数量：**最多1个**（极度精选）\n")
		sb.WriteString("- 决策频率：**减少交易**（宁可不做，也不要乱做）\n\n")
		sb.WriteString("**反思要点**：\n")
		sb.WriteString("- 为什么之前的交易亏损？是选币问题还是时机问题？\n")
		sb.WriteString("- 是否追涨杀跌？是否逆势交易？\n")
		sb.WriteString("- 止损是否执行到位？\n\n")

	} else if sharpe < 1 {
		// 🟡 0-1：正收益但波动性较大
		sb.WriteString("**状态：正收益但风险较高，需要优化**\n\n")
		sb.WriteString("**策略调整**：\n")
		sb.WriteString(fmt.Sprintf("- 仓位规模：**保守**（山寨币: %.0f USDT, BTC/ETH: %.0f USDT）\n",
			accountEquity*0.8, accountEquity*3.5))
		sb.WriteString("- 止损幅度：**收紧至-1.5%**\n")
		sb.WriteString("- 选币标准：**提高阈值**（信心度≥80%，风险回报比≥1:2.5）\n")
		sb.WriteString("- 持仓数量：**最多2个**\n")
		sb.WriteString("- 重点改进：**减少亏损幅度**，提高止损执行力\n\n")
		sb.WriteString("**优化方向**：\n")
		sb.WriteString("- 避免冲动交易，等待更好的入场时机\n")
		sb.WriteString("- 减少交易频率，提高单笔交易质量\n")
		sb.WriteString("- 盈利时及时止盈，不要贪多\n\n")

	} else if sharpe < 2 {
		// 🟢 1-2：风险调整后表现良好
		sb.WriteString("**状态：表现良好，继续保持当前策略**\n\n")
		sb.WriteString("**策略调整**：\n")
		sb.WriteString(fmt.Sprintf("- 仓位规模：**标准**（山寨币: %.0f USDT, BTC/ETH: %.0f USDT）\n",
			accountEquity*1.2, accountEquity*5))
		sb.WriteString("- 止损幅度：**-2%**（标准设置）\n")
		sb.WriteString("- 选币标准：**正常**（信心度≥75%，风险回报比≥1:2）\n")
		sb.WriteString("- 持仓数量：**最多3个**\n")
		sb.WriteString("- 保持纪律：**严格执行止损止盈**\n\n")
		sb.WriteString("**持续改进**：\n")
		sb.WriteString("- 总结盈利交易的共性特征，复制成功模式\n")
		sb.WriteString("- 分析亏损交易，避免重复错误\n")
		sb.WriteString("- 保持冷静客观，不要因为短期盈利而冒进\n\n")

	} else {
		// 🟢 >2：风险调整后表现优异
		sb.WriteString("**状态：卓越表现，策略非常有效！**\n\n")
		sb.WriteString("**策略调整**：\n")
		sb.WriteString(fmt.Sprintf("- 仓位规模：**可适度放大**（山寨币: %.0f USDT, BTC/ETH: %.0f USDT）\n",
			accountEquity*1.5, accountEquity*6))
		sb.WriteString("- 止损幅度：**-2%**（保持纪律，不要因为盈利而放松）\n")
		sb.WriteString("- 选币标准：**正常**（信心度≥75%）\n")
		sb.WriteString("- 持仓数量：**最多3个**\n")
		sb.WriteString("- **核心原则：保持纪律，不要过度自信**\n\n")
		sb.WriteString("**风险提示**：\n")
		sb.WriteString("- 即使表现优异，也要保持风险管理纪律\n")
		sb.WriteString("- 市场环境会变化，不要因短期成功而冒进\n")
		sb.WriteString("- 继续严格执行止损，保护已有收益\n\n")
	}

	return sb.String()
}
