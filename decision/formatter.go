package decision

import (
	"fmt"
	"nofx/market"
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

	// 4. 最近交易记录
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
		if lang == LangChinese {
			sb.WriteString(formatOIRankingZH(ctx.OIRankingData))
		} else {
			sb.WriteString(formatOIRankingEN(ctx.OIRankingData))
		}
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

// formatOIRankingZH 格式化OI排名数据（中文）
func formatOIRankingZH(oiData interface{}) string {
	// TODO: 根据实际OIRankingData结构实现
	return "## 市场持仓量排名\n\n(数据加载中...)\n\n"
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

// formatOIRankingEN 格式化OI排名数据（英文）
func formatOIRankingEN(oiData interface{}) string {
	return "## Market-wide OI Ranking\n\n(Loading data...)\n\n"
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
