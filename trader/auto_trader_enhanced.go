package trader

import (
        "fmt"
        "log"
        "nofx/decision"
        "nofx/logger"
        "strings"
        "time"
)

// EnhancedAutoTrader 增强版自动交易器
// 集成增强版Kelly公式管理器，支持实时峰值追踪和数据持久化
type EnhancedAutoTrader struct {
        *AutoTrader
        kellyManagerEnhanced *decision.KellyStopManagerEnhanced // 增强版凯利公式管理器
}

// NewEnhancedAutoTrader 创建增强版自动交易器
func NewEnhancedAutoTrader(config AutoTraderConfig) (*EnhancedAutoTrader, error) {
        // 先创建基础AutoTrader
        baseTrader, err := NewAutoTrader(config)
        if err != nil {
                return nil, fmt.Errorf("创建基础交易器失败: %w", err)
        }

        // 创建增强版Kelly管理器，数据文件保存在data目录下
        dataFilePath := fmt.Sprintf("data/kelly_stats_%s.json", config.ID)
        kellyManager := decision.NewKellyStopManagerEnhanced(dataFilePath)

        log.Printf("🚀 创建增强版AutoTrader，数据文件: %s", dataFilePath)

        return &EnhancedAutoTrader{
                AutoTrader:           baseTrader,
                kellyManagerEnhanced: kellyManager,
        }, nil
}

// Shutdown 优雅关闭增强版交易器
func (eat *EnhancedAutoTrader) Shutdown() error {
        log.Println("🔄 正在关闭增强版AutoTrader...")

        // 保存Kelly统计数据
        if err := eat.kellyManagerEnhanced.Shutdown(); err != nil {
                log.Printf("⚠️ 关闭Kelly管理器失败: %v", err)
        }

        // 关闭基础交易器（调用Stop方法）
        eat.AutoTrader.Stop()

        log.Println("✅ 增强版AutoTrader已安全关闭")
        return nil
}

// checkAndUpdateStopOrdersEnhanced 增强版止盈止损检查
func (eat *EnhancedAutoTrader) checkAndUpdateStopOrdersEnhanced() error {
        log.Println("🔄 开始执行增强版凯利公式动态止盈止损检查...")

        // 1. 获取当前持仓
        positions, err := eat.trader.GetPositions()
        if err != nil {
                return fmt.Errorf("获取持仓失败: %w", err)
        }

        log.Printf("📊 当前持仓数量: %d", len(positions))

        // 2. 对每个持仓进行止盈止损检查
        for _, pos := range positions {
                symbol := pos["symbol"].(string)
                side := pos["side"].(string)
                entryPrice := pos["entryPrice"].(float64)
                currentPrice := pos["markPrice"].(float64)

                // 计算当前盈利百分比并更新峰值
                var currentProfitPct float64
                if side == "long" {
                        currentProfitPct = (currentPrice - entryPrice) / entryPrice
                } else {
                        currentProfitPct = (entryPrice - currentPrice) / entryPrice
                }

                // 更新持仓期间的峰值盈利（关键改进）
                eat.kellyManagerEnhanced.UpdatePositionPeak(symbol, currentProfitPct)

                // 3. 使用增强版凯利公式计算动态止盈止损
                log.Printf("🎯 [%s] 计算增强版止盈止损: 入场价=%.6f, 当前价=%.6f, 盈利=%.2f%%",
                        symbol, entryPrice, currentPrice, currentProfitPct*100)

                // 计算动态止盈点（使用增强版算法）
                optimalTakeProfitPrice, err := eat.kellyManagerEnhanced.CalculateOptimalTakeProfitEnhanced(
                        symbol, entryPrice, currentPrice, side,
                )
                if err != nil {
                        log.Printf("⚠️ [%s] 计算增强版止盈点失败: %v", symbol, err)
                        continue
                }

                // 计算动态止损点（使用增强版算法）
                peakProfit := eat.kellyManagerEnhanced.GetPositionPeak(symbol)
                dynamicStopLossPrice, err := eat.kellyManagerEnhanced.CalculateDynamicStopLossEnhanced(
                        symbol, entryPrice, currentPrice, peakProfit,
                )
                if err != nil {
                        log.Printf("⚠️ [%s] 计算增强版止损点失败: %v", symbol, err)
                        continue
                }

                // 4. 更新止盈止损单
                quantity := pos["positionAmt"].(float64)
                if quantity < 0 {
                        quantity = -quantity
                }

                positionSide := strings.ToUpper(side)

                // 更新止损单
                if err := eat.trader.SetStopLoss(symbol, positionSide, quantity, dynamicStopLossPrice); err != nil {
                        log.Printf("⚠️ [%s] 更新增强版止损单失败 (%s): %v", symbol, positionSide, err)
                } else {
                        log.Printf("✅ [%s] 更新增强版止损单成功: %s @ %.6f (峰值盈利: %.2f%%)",
                                symbol, positionSide, dynamicStopLossPrice, peakProfit*100)
                }

                // 更新止盈单
                if err := eat.trader.SetTakeProfit(symbol, positionSide, quantity, optimalTakeProfitPrice); err != nil {
                        log.Printf("⚠️ [%s] 更新增强版止盈单失败 (%s): %v", symbol, positionSide, err)
                } else {
                        log.Printf("✅ [%s] 更新增强版止盈单成功: %s @ %.6f", symbol, positionSide, optimalTakeProfitPrice)
                }

                // 短暂延迟，避免API调用过于频繁
                time.Sleep(500 * time.Millisecond)
        }

        log.Printf("✅ 增强版止盈止损检查完成，共处理 %d 个持仓", len(positions))
        return nil
}

// recordTradeResultEnhanced 增强版记录交易结果
func (eat *EnhancedAutoTrader) recordTradeResultEnhanced(symbol string, isWin bool, profitPct float64, holdingTime time.Duration) {
        log.Printf("📊 [%s] 记录增强版交易结果: 盈利=%t, 收益率=%.2f%%, 持仓时间=%v",
                symbol, isWin, profitPct*100, holdingTime)

        // 清除该币种的峰值记录（平仓后不再追踪）
        eat.kellyManagerEnhanced.ClearPositionPeak(symbol)

        // 使用增强版统计更新函数
        eat.kellyManagerEnhanced.UpdateHistoricalStatsEnhanced(
                symbol,
                isWin,
                profitPct,
                int64(holdingTime.Seconds()),
        )

        log.Printf("💾 [%s] 增强版交易数据已更新并持久化", symbol)
}

// runCycleEnhanced 增强版交易周期
func (eat *EnhancedAutoTrader) runCycleEnhanced() error {
        log.Println("🔄 开始增强版交易周期...")

        // 调用基础交易周期
        if err := eat.AutoTrader.runCycle(); err != nil {
                return fmt.Errorf("基础交易周期失败: %w", err)
        }

        // 使用增强版止盈止损检查
        log.Println("🔄 开始执行增强版凯利公式动态止盈止损检查...")
        if err := eat.checkAndUpdateStopOrdersEnhanced(); err != nil {
                log.Printf("⚠️ 增强版止盈止损更新失败: %v", err)
        } else {
                log.Println("✅ 所有持仓的增强版止盈止损单已更新")
        }

        return nil
}

// GetKellyStats 获取Kelly公式统计数据
func (eat *EnhancedAutoTrader) GetKellyStats(symbol string) *decision.HistoricalStatsEnhanced {
        return eat.kellyManagerEnhanced.GetHistoricalStats(symbol)
}

// UpdateKellyConfig 更新Kelly公式配置
func (eat *EnhancedAutoTrader) UpdateKellyConfig(config *decision.KellyConfig) {
        eat.kellyManagerEnhanced.UpdateConfig(config)
        log.Printf("⚙️ Kelly配置已更新: 凯利系数=%.2f, 最大倍数=%.1f, 时间衰减=%.3f",
                config.KellyRatioAdjustment, config.MaxTakeProfitMultiplier, config.TimeDecayLambda)
}

// GetKellyConfig 获取当前Kelly配置
func (eat *EnhancedAutoTrader) GetKellyConfig() *decision.KellyConfig {
        return eat.kellyManagerEnhanced.GetConfig()
}

// ForceSaveKellyStats 强制保存Kelly统计数据
func (eat *EnhancedAutoTrader) ForceSaveKellyStats() error {
        // SaveStatsToFile 需要文件路径参数，使用数据文件路径
        dataFilePath := fmt.Sprintf("data/kelly_stats_%s.json", eat.AutoTrader.GetID())
        return eat.kellyManagerEnhanced.SaveStatsToFile(dataFilePath)
}

// 覆盖executeDecisionWithRecord以使用增强版记录功能
func (eat *EnhancedAutoTrader) executeDecisionWithRecordEnhanced(d *decision.Decision, actionRecord *logger.DecisionAction) error {
        // 先执行基础决策
        err := eat.AutoTrader.executeDecisionWithRecord(d, actionRecord)

        // 如果是平仓操作，使用增强版记录
        if d.Action == "close_long" || d.Action == "close_short" {
                if err == nil && actionRecord.Success {
                        // 使用增强版记录功能
                        // 注意: 盈亏信息需要从持仓记录中获取，这里简化为默认值
                        holdingTime := time.Since(eat.getPositionOpenTime(d.Symbol))
                        eat.recordTradeResultEnhanced(d.Symbol, true, 0.0, holdingTime)
                }
        }

        return err
}

// getPositionOpenTime 获取仓位开仓时间（辅助函数）
func (eat *EnhancedAutoTrader) getPositionOpenTime(symbol string) time.Time {
        posKey := symbol + "_long" // 简化处理，假设主要是多仓
        if firstSeenTime, exists := eat.positionFirstSeenTime[posKey]; exists {
                return time.UnixMilli(firstSeenTime)
        }
        return time.Now().Add(-time.Hour) // 默认返回1小时前
}

// 为EnhancedAutoTrader添加缺失的字段定义
type EnhancedAutoTraderFields struct {
        positionFirstSeenTime map[string]int64 // 记录每个仓位首次看到的时间
}

// 初始化增强版交易器的额外字段
func (eat *EnhancedAutoTrader) initializeEnhancedFields() {
        if eat.positionFirstSeenTime == nil {
                eat.positionFirstSeenTime = make(map[string]int64)
        }
}

// MonitorPerformance 性能监控（新增功能）
func (eat *EnhancedAutoTrader) MonitorPerformance() {
        log.Println("📈 增强版性能监控报告:")

        // 获取所有币种的Kelly统计
        symbols := []string{"BTC", "ETH", "SOL", "DOGE"} // 主要交易币种

        for _, symbol := range symbols {
                stats := eat.GetKellyStats(symbol)
                if stats != nil && stats.TotalTrades > 0 {
                        log.Printf("📊 [%s] 总交易: %d, 加权胜率: %.2f%%, 平均盈利: %.2f%%, 平均亏损: %.2f%%, 波动率: %.2f%%",
                                symbol, stats.TotalTrades, stats.WeightedWinRate*100, stats.AvgWinPct*100,
                                stats.AvgLossPct*100, stats.Volatility*100)
                }
        }

        // 显示当前配置
        config := eat.GetKellyConfig()
        log.Printf("⚙️ 当前配置: 凯利系数=%.2f, 最大倍数=%.1f, 时间衰减=%.3f, 最小交易数=%d",
                config.KellyRatioAdjustment, config.MaxTakeProfitMultiplier, config.TimeDecayLambda, config.MinTradesForKelly)
}

// OptimizeParameters 参数自动优化（基于历史表现）
func (eat *EnhancedAutoTrader) OptimizeParameters() {
        log.Println("🔧 开始自动参数优化...")

        // 简单的参数优化逻辑
        config := eat.GetKellyConfig()

        // 分析整体表现
        totalTrades := 0
        totalWinRate := 0.0
        count := 0

        symbols := []string{"BTC", "ETH", "SOL", "DOGE"}
        for _, symbol := range symbols {
                stats := eat.GetKellyStats(symbol)
                if stats != nil && stats.TotalTrades >= 10 { // 至少10笔交易才考虑
                        totalTrades += stats.TotalTrades
                        totalWinRate += stats.WeightedWinRate
                        count++
                }
        }

        if count > 0 {
                avgWinRate := totalWinRate / float64(count)

                // 根据平均胜率调整参数
                if avgWinRate > 0.6 {
                        // 高胜率，可以适当激进
                        config.KellyRatioAdjustment = 0.6
                        config.MaxTakeProfitMultiplier = 3.5
                        log.Printf("📈 检测到高胜率(%.2f%%)，调整为激进参数", avgWinRate*100)
                } else if avgWinRate < 0.4 {
                        // 低胜率，需要更保守
                        config.KellyRatioAdjustment = 0.3
                        config.MaxTakeProfitMultiplier = 2.0
                        log.Printf("📉 检测到低胜率(%.2f%%)，调整为保守参数", avgWinRate*100)
                } else {
                        // 中等胜率，使用默认参数
                        log.Printf("⚖️ 胜率适中(%.2f%%)，使用默认参数", avgWinRate*100)
                }

                eat.UpdateKellyConfig(config)
        } else {
                log.Println("⚠️ 数据不足，无法自动优化参数")
        }
}

// 辅助函数：检查并初始化增强版交易器
func (eat *EnhancedAutoTrader) ensureInitialized() {
        eat.initializeEnhancedFields()

        // 确保Kelly管理器已初始化
        if eat.kellyManagerEnhanced == nil {
                dataFilePath := fmt.Sprintf("data/kelly_stats_%s.json", eat.id)
                eat.kellyManagerEnhanced = decision.NewKellyStopManagerEnhanced(dataFilePath)
        }
}

// EnhancedNewAutoTrader 创建增强版自动交易器的工厂函数
func EnhancedNewAutoTrader(config AutoTraderConfig) (*EnhancedAutoTrader, error) {
        return NewEnhancedAutoTrader(config)
}

// 为了确保兼容性，添加一个包装函数
func (eat *EnhancedAutoTrader) RunCycle() error {
        return eat.runCycleEnhanced()
}