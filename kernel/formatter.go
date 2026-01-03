package kernel

import (
	"fmt"
	"nofx/market"
	"nofx/provider/nofxos"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// AI Data Formatter - AI数据格式化器
// ============================================================================
// 将交易上下文转换为AI友好的格式，确保AI能够100%理解数据
// ============================================================================

// FormatContextForAI 将交易上下文格式化为AI可理解的文本（包含Schema）
func FormatContextForAI(ctx *Context, lang Language) string {
	var sb strings.Builder

	// 1. 添加Schema说明（让AI理解数据格式）
	sb.WriteString(GetSchemaPrompt(lang))
	sb.WriteString("\n---\n\n")

	// 2. 当前状态概览
	sb.WriteString(formatContextData(ctx, lang))

	return sb.String()
}

// FormatContextDataOnly 仅格式化上下文数据，不包含Schema（用于已有Schema的场景）
func FormatContextDataOnly(ctx *Context, lang Language) string {
	return formatContextData(ctx, lang)
}

// formatContextData 格式化核心数据部分
func formatContextData(ctx *Context, lang Language) string {
	var sb strings.Builder

	// 1. 当前状态概览
	if lang == LangChinese {
		sb.WriteString(formatHeaderZH(ctx))
	} else {
		sb.WriteString(formatHeaderEN(ctx))
	}

	// 3. 账户信息
	if lang == LangChinese {
		sb.WriteString(formatAccountZH(ctx))
	} else {
		sb.WriteString(formatAccountEN(ctx))
	}

	// 4. 历史交易统计
	if ctx.TradingStats != nil && ctx.TradingStats.TotalTrades > 0 {
		if lang == LangChinese {
			sb.WriteString(formatTradingStatsZH(ctx.TradingStats))
		} else {
			sb.WriteString(formatTradingStatsEN(ctx.TradingStats))
		}
	}

	// 5. 最近交易记录
	if len(ctx.RecentOrders) > 0 {
		if lang == LangChinese {
			sb.WriteString(formatRecentTradesZH(ctx.RecentOrders))
		} else {
			sb.WriteString(formatRecentTradesEN(ctx.RecentOrders))
		}
	}

	// 5. 当前持仓
	if len(ctx.Positions) > 0 {
		if lang == LangChinese {
			sb.WriteString(formatCurrentPositionsZH(ctx))
		} else {
			sb.WriteString(formatCurrentPositionsEN(ctx))
		}
	}

	// 6. 候选币种（带市场数据）
	if len(ctx.CandidateCoins) > 0 {
		if lang == LangChinese {
			sb.WriteString(formatCandidateCoinsZH(ctx))
		} else {
			sb.WriteString(formatCandidateCoinsEN(ctx))
		}
	}

	// 7. OI排名数据（如果有）
	if ctx.OIRankingData != nil {
		nofxosLang := nofxos.LangEnglish
		if lang == LangChinese {
			nofxosLang = nofxos.LangChinese
		}
		sb.WriteString(nofxos.FormatOIRankingForAI(ctx.OIRankingData, nofxosLang))
	}

	return sb.String()
}

// ========== 中文格式化函数 ==========

// formatHeaderZH 格式化头部信息（中文）
func formatHeaderZH(ctx *Context) string {
	return fmt.Sprintf("# 📊 交易决策请求\n\n时间: %s | 周期: #%d | 运行时长: %d 分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes)
}

// formatAccountZH 格式化账户信息（中文）
func formatAccountZH(ctx *Context) string {
	acc := ctx.Account
	var sb strings.Builder

	sb.WriteString("## 账户状态\n\n")
	sb.WriteString(fmt.Sprintf("总权益: %.2f USDT | ", acc.TotalEquity))
	sb.WriteString(fmt.Sprintf("可用余额: %.2f USDT (%.1f%%) | ", acc.AvailableBalance, (acc.AvailableBalance/acc.TotalEquity)*100))
	sb.WriteString(fmt.Sprintf("总盈亏: %+.2f%% | ", acc.TotalPnLPct))
	sb.WriteString(fmt.Sprintf("保证金使用率: %.1f%% | ", acc.MarginUsedPct))
	sb.WriteString(fmt.Sprintf("持仓数: %d\n\n", acc.PositionCount))

	// 添加风险提示
	if acc.MarginUsedPct > 70 {
		sb.WriteString("⚠️ **风险警告**: 保证金使用率 > 70%，处于高风险状态！\n\n")
	} else if acc.MarginUsedPct > 50 {
		sb.WriteString("⚠️ **风险提示**: 保证金使用率 > 50%，建议谨慎开仓\n\n")
	}

	return sb.String()
}

// formatTradingStatsZH 格式化历史交易统计（中文）
func formatTradingStatsZH(stats *TradingStats) string {
	var sb strings.Builder
	sb.WriteString("## 历史交易统计\n\n")

	// 盈亏比计算
	var winLossRatio float64
	if stats.AvgLoss > 0 {
		winLossRatio = stats.AvgWin / stats.AvgLoss
	}

	// 指标定义说明（去掉胜率，聚焦核心指标）
	sb.WriteString("**指标说明**:\n")
	sb.WriteString("- 盈利因子: 总盈利 ÷ 总亏损（>1表示盈利，>1.5为良好，>2为优秀）\n")
	sb.WriteString("- 夏普比率: (平均收益 - 无风险收益) ÷ 收益标准差（>1良好，>2优秀）\n")
	sb.WriteString("- 盈亏比: 平均盈利 ÷ 平均亏损（>1.5为良好，>2为优秀）\n")
	sb.WriteString("- 最大回撤: 资金曲线从峰值到谷底的最大跌幅（<20%为低风险）\n\n")

	// 数据值
	sb.WriteString("**当前数据**:\n")
	sb.WriteString(fmt.Sprintf("- 总交易: %d 笔\n", stats.TotalTrades))
	sb.WriteString(fmt.Sprintf("- 盈利因子: %.2f\n", stats.ProfitFactor))
	sb.WriteString(fmt.Sprintf("- 夏普比率: %.2f\n", stats.SharpeRatio))
	sb.WriteString(fmt.Sprintf("- 盈亏比: %.2f\n", winLossRatio))
	sb.WriteString(fmt.Sprintf("- 总盈亏: %+.2f USDT\n", stats.TotalPnL))
	sb.WriteString(fmt.Sprintf("- 平均盈利: +%.2f USDT\n", stats.AvgWin))
	sb.WriteString(fmt.Sprintf("- 平均亏损: -%.2f USDT\n", stats.AvgLoss))
	sb.WriteString(fmt.Sprintf("- 最大回撤: %.1f%%\n\n", stats.MaxDrawdownPct))

	// 综合分析和决策建议
	sb.WriteString("**决策参考**:\n")

	// 根据统计数据给出具体建议
	if stats.TotalTrades < 10 {
		sb.WriteString("- 样本量较小（<10笔），统计结果参考意义有限\n")
	}

	if stats.ProfitFactor >= 1.5 && stats.SharpeRatio >= 1 {
		sb.WriteString("- 📈 表现良好: 可以维持当前策略风格\n")
	} else if stats.ProfitFactor >= 1.0 {
		sb.WriteString("- 📊 表现正常: 策略可行但有优化空间\n")
	}

	if stats.ProfitFactor < 1.0 {
		sb.WriteString("- ⚠️ 盈利因子<1: 亏损大于盈利，需要提高盈亏比，优化止盈止损\n")
	}

	if winLossRatio > 0 && winLossRatio < 1.5 {
		sb.WriteString("- ⚠️ 盈亏比偏低: 建议让利润奔跑，提高止盈目标\n")
	}

	if stats.MaxDrawdownPct > 30 {
		sb.WriteString("- ⚠️ 最大回撤过高: 建议降低仓位大小控制风险\n")
	} else if stats.MaxDrawdownPct < 10 {
		sb.WriteString("- ✅ 回撤控制良好: 风险管理有效\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatRecentTradesZH 格式化最近交易（中文）
func formatRecentTradesZH(orders []RecentOrder) string {
	var sb strings.Builder
	sb.WriteString("## 最近完成的交易\n\n")

	for i, order := range orders {
		// 判断盈亏
		profitOrLoss := "盈利"
		if order.RealizedPnL < 0 {
			profitOrLoss = "亏损"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s | 进场 %.4f 出场 %.4f | %s: %+.2f USDT (%+.2f%%) | %s → %s (%s)\n",
			i+1,
			order.Symbol,
			order.Side,
			order.EntryPrice,
			order.ExitPrice,
			profitOrLoss,
			order.RealizedPnL,
			order.PnLPct,
			order.EntryTime,
			order.ExitTime,
			order.HoldDuration,
		))
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatCurrentPositionsZH 格式化当前持仓（中文）
func formatCurrentPositionsZH(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## 当前持仓\n\n")

	for i, pos := range ctx.Positions {
		// 计算回撤
		drawdown := pos.UnrealizedPnLPct - pos.PeakPnLPct

		sb.WriteString(fmt.Sprintf("%d. %s %s | ", i+1, pos.Symbol, strings.ToUpper(pos.Side)))
		sb.WriteString(fmt.Sprintf("进场 %.4f 当前 %.4f | ", pos.EntryPrice, pos.MarkPrice))
		sb.WriteString(fmt.Sprintf("数量 %.4f | ", pos.Quantity))
		sb.WriteString(fmt.Sprintf("仓位价值 %.2f USDT | ", pos.Quantity*pos.MarkPrice))
		sb.WriteString(fmt.Sprintf("盈亏 %+.2f%% | ", pos.UnrealizedPnLPct))
		sb.WriteString(fmt.Sprintf("盈亏金额 %+.2f USDT | ", pos.UnrealizedPnL))
		sb.WriteString(fmt.Sprintf("峰值盈亏 %.2f%% | ", pos.PeakPnLPct))
		sb.WriteString(fmt.Sprintf("杠杆 %dx | ", pos.Leverage))
		sb.WriteString(fmt.Sprintf("保证金 %.0f USDT | ", pos.MarginUsed))
		sb.WriteString(fmt.Sprintf("强平价 %.4f\n", pos.LiquidationPrice))

		// 添加分析提示
		if drawdown < -0.30*pos.PeakPnLPct && pos.PeakPnLPct > 0.02 {
			sb.WriteString(fmt.Sprintf("   ⚠️ **止盈提示**: 当前盈亏从峰值 %.2f%% 回撤到 %.2f%%，回撤幅度 %.2f%%，建议考虑止盈\n",
				pos.PeakPnLPct, pos.UnrealizedPnLPct, (drawdown/pos.PeakPnLPct)*100))
		}

		if pos.UnrealizedPnLPct < -4.0 {
			sb.WriteString("   ⚠️ **止损提示**: 亏损接近-5%止损线，建议考虑止损\n")
		}

		// 显示当前价格（如果有市场数据）
		if ctx.MarketDataMap != nil {
			if mdata, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("   📈 当前价格: %.4f\n", mdata.CurrentPrice))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatCandidateCoinsZH 格式化候选币种（中文）
func formatCandidateCoinsZH(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## 候选币种\n\n")

	for i, coin := range ctx.CandidateCoins {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, coin.Symbol))

		// 当前价格
		if ctx.MarketDataMap != nil {
			if mdata, ok := ctx.MarketDataMap[coin.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("当前价格: %.4f\n\n", mdata.CurrentPrice))

				// K线数据（多时间框架）
				if mdata.TimeframeData != nil {
					sb.WriteString(formatKlineDataZH(coin.Symbol, mdata.TimeframeData, ctx.Timeframes))
				}
			}
		}

		// OI数据（如果有）
		if ctx.OITopDataMap != nil {
			if oiData, ok := ctx.OITopDataMap[coin.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("**持仓量变化**: OI排名 #%d | 变化 %+.2f%% (%+.2fM USDT) | 价格变化 %+.2f%%\n\n",
					oiData.Rank,
					oiData.OIDeltaPercent,
					oiData.OIDeltaValue/1_000_000,
					oiData.PriceDeltaPercent,
				))

				// OI解读
				oiChange := "增加"
				if oiData.OIDeltaPercent < 0 {
					oiChange = "减少"
				}
				priceChange := "上涨"
				if oiData.PriceDeltaPercent < 0 {
					priceChange = "下跌"
				}

				interpretation := getOIInterpretationZH(oiChange, priceChange)
				sb.WriteString(fmt.Sprintf("**市场解读**: %s\n\n", interpretation))
			}
		}
	}

	return sb.String()
}

// formatKlineDataZH 格式化K线数据（中文）
func formatKlineDataZH(symbol string, tfData map[string]*market.TimeframeSeriesData, timeframes []string) string {
	var sb strings.Builder

	for _, tf := range timeframes {
		if data, ok := tfData[tf]; ok && len(data.Klines) > 0 {
			sb.WriteString(fmt.Sprintf("#### %s 时间框架 (从旧到新)\n\n", tf))
			sb.WriteString("```\n")
			sb.WriteString("时间(UTC)      开盘      最高      最低      收盘      成交量\n")

			// 只显示最近30根K线
			startIdx := 0
			if len(data.Klines) > 30 {
				startIdx = len(data.Klines) - 30
			}

			for i := startIdx; i < len(data.Klines); i++ {
				k := data.Klines[i]
				t := time.UnixMilli(k.Time).UTC()
				sb.WriteString(fmt.Sprintf("%s    %.4f    %.4f    %.4f    %.4f    %.2f\n",
					t.Format("01-02 15:04"),
					k.Open,
					k.High,
					k.Low,
					k.Close,
					k.Volume,
				))
			}

			// 标记最后一根K线
			if len(data.Klines) > 0 {
				sb.WriteString("    <- 当前\n")
			}

			sb.WriteString("```\n\n")
		}
	}

	return sb.String()
}


// getOIInterpretationZH 获取OI变化解读（中文）
func getOIInterpretationZH(oiChange, priceChange string) string {
	if oiChange == "增加" && priceChange == "上涨" {
		return OIInterpretation.OIUp_PriceUp.ZH
	} else if oiChange == "增加" && priceChange == "下跌" {
		return OIInterpretation.OIUp_PriceDown.ZH
	} else if oiChange == "减少" && priceChange == "上涨" {
		return OIInterpretation.OIDown_PriceUp.ZH
	} else {
		return OIInterpretation.OIDown_PriceDown.ZH
	}
}

// ========== 英文格式化函数 ==========

// formatHeaderEN 格式化头部信息（英文）
func formatHeaderEN(ctx *Context) string {
	return fmt.Sprintf("# 📊 Trading Decision Request\n\nTime: %s | Period: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes)
}

// formatAccountEN 格式化账户信息（英文）
func formatAccountEN(ctx *Context) string {
	acc := ctx.Account
	var sb strings.Builder

	sb.WriteString("## Account Status\n\n")
	sb.WriteString(fmt.Sprintf("Total Equity: %.2f USDT | ", acc.TotalEquity))
	sb.WriteString(fmt.Sprintf("Available Balance: %.2f USDT (%.1f%%) | ", acc.AvailableBalance, (acc.AvailableBalance/acc.TotalEquity)*100))
	sb.WriteString(fmt.Sprintf("Total PnL: %+.2f%% | ", acc.TotalPnLPct))
	sb.WriteString(fmt.Sprintf("Margin Usage: %.1f%% | ", acc.MarginUsedPct))
	sb.WriteString(fmt.Sprintf("Positions: %d\n\n", acc.PositionCount))

	// Risk warning
	if acc.MarginUsedPct > 70 {
		sb.WriteString("⚠️ **Risk Alert**: Margin usage > 70%, high risk!\n\n")
	} else if acc.MarginUsedPct > 50 {
		sb.WriteString("⚠️ **Risk Notice**: Margin usage > 50%, be cautious with new positions\n\n")
	}

	return sb.String()
}

// formatTradingStatsEN 格式化历史交易统计（英文）
func formatTradingStatsEN(stats *TradingStats) string {
	var sb strings.Builder
	sb.WriteString("## Historical Trading Statistics\n\n")

	// Win/Loss ratio calculation
	var winLossRatio float64
	if stats.AvgLoss > 0 {
		winLossRatio = stats.AvgWin / stats.AvgLoss
	}

	// Metric definitions (focus on core metrics, remove win rate)
	sb.WriteString("**Metric Definitions**:\n")
	sb.WriteString("- Profit Factor: Total profits ÷ Total losses (>1 = profitable, >1.5 = good, >2 = excellent)\n")
	sb.WriteString("- Sharpe Ratio: (Avg return - Risk-free rate) ÷ Std dev of returns (>1 = good, >2 = excellent)\n")
	sb.WriteString("- Win/Loss Ratio: Avg win ÷ Avg loss (>1.5 = good, >2 = excellent)\n")
	sb.WriteString("- Max Drawdown: Largest peak-to-trough decline in equity curve (<20% = low risk)\n\n")

	// Data values
	sb.WriteString("**Current Data**:\n")
	sb.WriteString(fmt.Sprintf("- Total Trades: %d\n", stats.TotalTrades))
	sb.WriteString(fmt.Sprintf("- Profit Factor: %.2f\n", stats.ProfitFactor))
	sb.WriteString(fmt.Sprintf("- Sharpe Ratio: %.2f\n", stats.SharpeRatio))
	sb.WriteString(fmt.Sprintf("- Win/Loss Ratio: %.2f\n", winLossRatio))
	sb.WriteString(fmt.Sprintf("- Total PnL: %+.2f USDT\n", stats.TotalPnL))
	sb.WriteString(fmt.Sprintf("- Avg Win: +%.2f USDT\n", stats.AvgWin))
	sb.WriteString(fmt.Sprintf("- Avg Loss: -%.2f USDT\n", stats.AvgLoss))
	sb.WriteString(fmt.Sprintf("- Max Drawdown: %.1f%%\n\n", stats.MaxDrawdownPct))

	// Analysis and decision guidance
	sb.WriteString("**Decision Guidance**:\n")

	// Specific recommendations based on stats
	if stats.TotalTrades < 10 {
		sb.WriteString("- Small sample size (<10 trades), statistics have limited significance\n")
	}

	if stats.ProfitFactor >= 1.5 && stats.SharpeRatio >= 1 {
		sb.WriteString("- 📈 Good performance: Maintain current strategy approach\n")
	} else if stats.ProfitFactor >= 1.0 {
		sb.WriteString("- 📊 Normal performance: Strategy viable but has room for optimization\n")
	}

	if stats.ProfitFactor < 1.0 {
		sb.WriteString("- ⚠️ Profit factor <1: Losses exceed profits, improve win/loss ratio, optimize TP/SL\n")
	}

	if winLossRatio > 0 && winLossRatio < 1.5 {
		sb.WriteString("- ⚠️ Low win/loss ratio: Let profits run, increase take-profit targets\n")
	}

	if stats.MaxDrawdownPct > 30 {
		sb.WriteString("- ⚠️ High max drawdown: Consider reducing position sizes to control risk\n")
	} else if stats.MaxDrawdownPct < 10 {
		sb.WriteString("- ✅ Good drawdown control: Risk management is effective\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatRecentTradesEN 格式化最近交易（英文）
func formatRecentTradesEN(orders []RecentOrder) string {
	var sb strings.Builder
	sb.WriteString("## Recent Completed Trades\n\n")

	for i, order := range orders {
		profitOrLoss := "Profit"
		if order.RealizedPnL < 0 {
			profitOrLoss = "Loss"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | %s: %+.2f USDT (%+.2f%%) | %s → %s (%s)\n",
			i+1,
			order.Symbol,
			order.Side,
			order.EntryPrice,
			order.ExitPrice,
			profitOrLoss,
			order.RealizedPnL,
			order.PnLPct,
			order.EntryTime,
			order.ExitTime,
			order.HoldDuration,
		))
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatCurrentPositionsEN 格式化当前持仓（英文）
func formatCurrentPositionsEN(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## Current Positions\n\n")

	for i, pos := range ctx.Positions {
		drawdown := pos.UnrealizedPnLPct - pos.PeakPnLPct

		sb.WriteString(fmt.Sprintf("%d. %s %s | ", i+1, pos.Symbol, strings.ToUpper(pos.Side)))
		sb.WriteString(fmt.Sprintf("Entry %.4f Current %.4f | ", pos.EntryPrice, pos.MarkPrice))
		sb.WriteString(fmt.Sprintf("Qty %.4f | ", pos.Quantity))
		sb.WriteString(fmt.Sprintf("Value %.2f USDT | ", pos.Quantity*pos.MarkPrice))
		sb.WriteString(fmt.Sprintf("PnL %+.2f%% | ", pos.UnrealizedPnLPct))
		sb.WriteString(fmt.Sprintf("PnL Amount %+.2f USDT | ", pos.UnrealizedPnL))
		sb.WriteString(fmt.Sprintf("Peak PnL %.2f%% | ", pos.PeakPnLPct))
		sb.WriteString(fmt.Sprintf("Leverage %dx | ", pos.Leverage))
		sb.WriteString(fmt.Sprintf("Margin %.0f USDT | ", pos.MarginUsed))
		sb.WriteString(fmt.Sprintf("Liq Price %.4f\n", pos.LiquidationPrice))

		// Analysis hints
		if drawdown < -0.30*pos.PeakPnLPct && pos.PeakPnLPct > 0.02 {
			sb.WriteString(fmt.Sprintf("   ⚠️ **Take Profit Alert**: PnL dropped from peak %.2f%% to %.2f%%, drawdown %.2f%%, consider taking profit\n",
				pos.PeakPnLPct, pos.UnrealizedPnLPct, (drawdown/pos.PeakPnLPct)*100))
		}

		if pos.UnrealizedPnLPct < -4.0 {
			sb.WriteString("   ⚠️ **Stop Loss Alert**: Loss approaching -5% threshold, consider cutting loss\n")
		}

		if ctx.MarketDataMap != nil {
			if mdata, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("   📈 Current Price: %.4f\n", mdata.CurrentPrice))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatCandidateCoinsEN 格式化候选币种（英文）
func formatCandidateCoinsEN(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## Candidate Coins\n\n")

	for i, coin := range ctx.CandidateCoins {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, coin.Symbol))

		if ctx.MarketDataMap != nil {
			if mdata, ok := ctx.MarketDataMap[coin.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("Current Price: %.4f\n\n", mdata.CurrentPrice))

				if mdata.TimeframeData != nil {
					sb.WriteString(formatKlineDataEN(coin.Symbol, mdata.TimeframeData, ctx.Timeframes))
				}
			}
		}

		if ctx.OITopDataMap != nil {
			if oiData, ok := ctx.OITopDataMap[coin.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("**OI Change**: Rank #%d | Change %+.2f%% (%+.2fM USDT) | Price Change %+.2f%%\n\n",
					oiData.Rank,
					oiData.OIDeltaPercent,
					oiData.OIDeltaValue/1_000_000,
					oiData.PriceDeltaPercent,
				))

				oiChange := "increase"
				if oiData.OIDeltaPercent < 0 {
					oiChange = "decrease"
				}
				priceChange := "up"
				if oiData.PriceDeltaPercent < 0 {
					priceChange = "down"
				}

				interpretation := getOIInterpretationEN(oiChange, priceChange)
				sb.WriteString(fmt.Sprintf("**Market Interpretation**: %s\n\n", interpretation))
			}
		}
	}

	return sb.String()
}

// formatKlineDataEN 格式化K线数据（英文）
func formatKlineDataEN(symbol string, tfData map[string]*market.TimeframeSeriesData, timeframes []string) string {
	var sb strings.Builder

	// Sort timeframes for consistent output
	sortedTF := make([]string, len(timeframes))
	copy(sortedTF, timeframes)
	sort.Strings(sortedTF)

	for _, tf := range sortedTF {
		if data, ok := tfData[tf]; ok && len(data.Klines) > 0 {
			sb.WriteString(fmt.Sprintf("#### %s Timeframe (oldest → latest)\n\n", tf))
			sb.WriteString("```\n")
			sb.WriteString("Time(UTC)      Open      High      Low       Close     Volume\n")

			startIdx := 0
			if len(data.Klines) > 30 {
				startIdx = len(data.Klines) - 30
			}

			for i := startIdx; i < len(data.Klines); i++ {
				k := data.Klines[i]
				t := time.UnixMilli(k.Time).UTC()
				sb.WriteString(fmt.Sprintf("%s    %.4f    %.4f    %.4f    %.4f    %.2f\n",
					t.Format("01-02 15:04"),
					k.Open,
					k.High,
					k.Low,
					k.Close,
					k.Volume,
				))
			}

			if len(data.Klines) > 0 {
				sb.WriteString("    <- current\n")
			}

			sb.WriteString("```\n\n")
		}
	}

	return sb.String()
}


// getOIInterpretationEN 获取OI变化解读（英文）
func getOIInterpretationEN(oiChange, priceChange string) string {
	if oiChange == "increase" && priceChange == "up" {
		return OIInterpretation.OIUp_PriceUp.EN
	} else if oiChange == "increase" && priceChange == "down" {
		return OIInterpretation.OIUp_PriceDown.EN
	} else if oiChange == "decrease" && priceChange == "up" {
		return OIInterpretation.OIDown_PriceUp.EN
	} else {
		return OIInterpretation.OIDown_PriceDown.EN
	}
}
