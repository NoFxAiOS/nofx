package backtest

import (
	"fmt"
	"math"
	"strings"
)

// CalculateMetrics 读取已有日志并计算汇总指标。state 可选，用于补充尚未落盘的信息。
func CalculateMetrics(runID string, cfg *BacktestConfig, state *BacktestState) (*Metrics, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	points, err := LoadEquityPoints(runID)
	if err != nil {
		return nil, fmt.Errorf("load equity points: %w", err)
	}

	events, err := LoadTradeEvents(runID)
	if err != nil {
		return nil, fmt.Errorf("load trade events: %w", err)
	}

	metrics := &Metrics{
		SymbolStats: make(map[string]SymbolMetrics),
	}

	metrics.Liquidated = determineLiquidation(events, state)

	initialBalance := cfg.InitialBalance
	if initialBalance <= 0 {
		initialBalance = 1
	}

	lastEquity := initialBalance
	if len(points) > 0 && points[len(points)-1].Equity > 0 {
		lastEquity = points[len(points)-1].Equity
	} else if state != nil && state.Equity > 0 {
		lastEquity = state.Equity
	}
	metrics.TotalReturnPct = ((lastEquity - initialBalance) / initialBalance) * 100

	metrics.MaxDrawdownPct = maxDrawdown(points, state)
	metrics.SharpeRatio = sharpeRatio(points)

	fillTradeMetrics(metrics, events)

	return metrics, nil
}

func determineLiquidation(events []TradeEvent, state *BacktestState) bool {
	if state != nil && state.Liquidated {
		return true
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].LiquidationFlag {
			return true
		}
	}
	return false
}

func maxDrawdown(points []EquityPoint, state *BacktestState) float64 {
	if len(points) == 0 {
		if state != nil {
			return state.MaxDrawdownPct
		}
		return 0
	}
	peak := points[0].Equity
	if peak <= 0 {
		peak = 1
	}
	maxDD := 0.0
	for _, pt := range points {
		if pt.Equity > peak {
			peak = pt.Equity
		}
		if peak <= 0 {
			continue
		}
		dd := (peak - pt.Equity) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
	}
	if state != nil && state.MaxDrawdownPct > maxDD {
		maxDD = state.MaxDrawdownPct
	}
	return maxDD
}

func sharpeRatio(points []EquityPoint) float64 {
	if len(points) < 2 {
		return 0
	}

	returns := make([]float64, 0, len(points)-1)
	prev := points[0].Equity
	for i := 1; i < len(points); i++ {
		curr := points[i].Equity
		if prev <= 0 {
			prev = curr
			continue
		}
		ret := (curr - prev) / prev
		returns = append(returns, ret)
		prev = curr
	}
	if len(returns) == 0 {
		return 0
	}

	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(returns))

	std := math.Sqrt(variance)
	if std == 0 {
		if mean > 0 {
			return 999
		}
		if mean < 0 {
			return -999
		}
		return 0
	}
	return mean / std
}

func fillTradeMetrics(metrics *Metrics, events []TradeEvent) {
	if metrics == nil {
		return
	}

	totalTrades := 0
	winTrades := 0
	lossTrades := 0
	totalWinAmount := 0.0
	totalLossAmount := 0.0

	for _, evt := range events {
		include := evt.LiquidationFlag || strings.HasPrefix(evt.Action, "close")
		if evt.RealizedPnL != 0 {
			include = true
		}
		if !include {
			continue
		}
		totalTrades++

		stats := metrics.SymbolStats[evt.Symbol]
		stats.TotalTrades++
		stats.TotalPnL += evt.RealizedPnL

		if evt.RealizedPnL > 0 {
			winTrades++
			totalWinAmount += evt.RealizedPnL
			stats.WinningTrades++
		} else if evt.RealizedPnL < 0 {
			lossTrades++
			totalLossAmount += -evt.RealizedPnL
			stats.LosingTrades++
		}

		metrics.SymbolStats[evt.Symbol] = stats
	}

	metrics.Trades = totalTrades
	if totalTrades > 0 {
		metrics.WinRate = (float64(winTrades) / float64(totalTrades)) * 100
	}
	if winTrades > 0 {
		metrics.AvgWin = totalWinAmount / float64(winTrades)
	}
	if lossTrades > 0 {
		metrics.AvgLoss = -(totalLossAmount / float64(lossTrades))
	}
	if totalLossAmount > 0 {
		metrics.ProfitFactor = totalWinAmount / totalLossAmount
	} else if totalWinAmount > 0 {
		metrics.ProfitFactor = 999
	}

	bestSymbol := ""
	bestPnL := math.Inf(-1)
	worstSymbol := ""
	worstPnL := math.Inf(1)

	for symbol, stats := range metrics.SymbolStats {
		if stats.TotalTrades > 0 {
			if stats.TotalPnL > bestPnL {
				bestPnL = stats.TotalPnL
				bestSymbol = symbol
			}
			if stats.TotalPnL < worstPnL {
				worstPnL = stats.TotalPnL
				worstSymbol = symbol
			}

			stats.AvgPnL = stats.TotalPnL / float64(stats.TotalTrades)
			stats.WinRate = (float64(stats.WinningTrades) / float64(stats.TotalTrades)) * 100
		}
		metrics.SymbolStats[symbol] = stats
	}

	metrics.BestSymbol = bestSymbol
	if math.IsInf(bestPnL, -1) {
		metrics.BestSymbol = ""
	}
	metrics.WorstSymbol = worstSymbol
	if math.IsInf(worstPnL, 1) {
		metrics.WorstSymbol = ""
	}
}
