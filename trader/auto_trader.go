package trader

import (
        "encoding/json"
        "fmt"
        "log"
        "nofx/decision"
        "nofx/logger"
        "nofx/market"
        "nofx/mcp"
        "nofx/pool"
        "strings"
        "time"
)

// AutoTraderConfig 自动交易配置（简化版 - AI全权决策）
type AutoTraderConfig struct {
        // Trader标识
        ID      string // Trader唯一标识（用于日志目录等）
        Name    string // Trader显示名称
        AIModel string // AI模型: "qwen" 或 "deepseek"

        // 交易平台选择
        Exchange string // "binance", "hyperliquid" 或 "aster"

        // 币安API配置
        BinanceAPIKey    string
        BinanceSecretKey string

        // Hyperliquid配置
        HyperliquidPrivateKey string
        HyperliquidWalletAddr string
        HyperliquidTestnet    bool

        // Aster配置
        AsterUser       string // Aster主钱包地址
        AsterSigner     string // Aster API钱包地址
        AsterPrivateKey string // Aster API钱包私钥

        // OKX配置
        OKXAPIKey     string // OKX API密钥
        OKXSecretKey  string // OKX Secret密钥
        OKXPassphrase string // OKX Passphrase
        OKXTestnet    bool   // OKX是否使用测试网络

        CoinPoolAPIURL string

        // AI配置
        UseQwen     bool
        DeepSeekKey string
        QwenKey     string

        // 自定义AI API配置
        CustomAPIURL    string
        CustomAPIKey    string
        CustomModelName string

        // 扫描配置
        ScanInterval time.Duration // 扫描间隔（建议3分钟）

        // 账户配置
        InitialBalance float64 // 初始金额（用于计算盈亏，需手动设置）

        // 杠杆配置
        BTCETHLeverage  int // BTC和ETH的杠杆倍数
        AltcoinLeverage int // 山寨币的杠杆倍数

        // 风险控制（仅作为提示，AI可自主决定）
        MaxDailyLoss    float64       // 最大日亏损百分比（提示）
        MaxDrawdown     float64       // 最大回撤百分比（提示）
        StopTradingTime time.Duration // 触发风控后暂停时长

        // 仓位模式
        IsCrossMargin bool // true=全仓模式, false=逐仓模式

        // 币种配置
        DefaultCoins []string // 默认币种列表（从数据库获取）
        TradingCoins []string // 实际交易币种列表

        // 系统提示词模板
        SystemPromptTemplate string // 系统提示词模板名称（如 "default", "aggressive"）
}

// AutoTrader 自动交易器
type AutoTrader struct {
        id                    string // Trader唯一标识
        name                  string // Trader显示名称
        aiModel               string // AI模型名称
        exchange              string // 交易平台名称
        config                AutoTraderConfig
        trader                Trader // 使用Trader接口（支持多平台）
        mcpClient             *mcp.Client
        decisionLogger        *logger.DecisionLogger // 决策日志记录器
        kellyManager          *decision.KellyStopManager // 凯利公式止盈止损管理器
        initialBalance        float64
        dailyPnL              float64
        customPrompt          string   // 自定义交易策略prompt
        overrideBasePrompt    bool     // 是否覆盖基础prompt
        systemPromptTemplate  string   // 系统提示词模板名称
        defaultCoins          []string // 默认币种列表（从数据库获取）
        tradingCoins          []string // 实际交易币种列表
        lastResetTime         time.Time
        stopUntil             time.Time
        isRunning             bool
        startTime             time.Time        // 系统启动时间
        callCount             int              // AI调用次数
        positionFirstSeenTime map[string]int64 // 持仓首次出现时间 (symbol_side -> timestamp毫秒)
}

// NewAutoTrader 创建自动交易器
func NewAutoTrader(config AutoTraderConfig) (*AutoTrader, error) {
        // 设置默认值
        if config.ID == "" {
                config.ID = "default_trader"
        }
        if config.Name == "" {
                config.Name = "Default Trader"
        }
        if config.AIModel == "" {
                if config.UseQwen {
                        config.AIModel = "qwen"
                } else {
                        config.AIModel = "deepseek"
                }
        }

        mcpClient := mcp.New()

        // 初始化AI
        if config.AIModel == "custom" {
                // 使用自定义API
                mcpClient.SetCustomAPI(config.CustomAPIURL, config.CustomAPIKey, config.CustomModelName)
                log.Printf("🤖 [%s] 使用自定义AI API: %s (模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
        } else if config.UseQwen || config.AIModel == "qwen" {
                // 使用Qwen (支持自定义URL和Model)
                mcpClient.SetQwenAPIKey(config.QwenKey, config.CustomAPIURL, config.CustomModelName)
                if config.CustomAPIURL != "" || config.CustomModelName != "" {
                        log.Printf("🤖 [%s] 使用阿里云Qwen AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
                } else {
                        log.Printf("🤖 [%s] 使用阿里云Qwen AI", config.Name)
                }
        } else {
                // 默认使用DeepSeek (支持自定义URL和Model)
                mcpClient.SetDeepSeekAPIKey(config.DeepSeekKey, config.CustomAPIURL, config.CustomModelName)
                if config.CustomAPIURL != "" || config.CustomModelName != "" {
                        log.Printf("🤖 [%s] 使用DeepSeek AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
                } else {
                        log.Printf("🤖 [%s] 使用DeepSeek AI", config.Name)
                }
        }

        // 初始化币种池API
        if config.CoinPoolAPIURL != "" {
                pool.SetCoinPoolAPI(config.CoinPoolAPIURL)
        }

        // 设置默认交易平台
        if config.Exchange == "" {
                config.Exchange = "binance"
        }

        // 根据配置创建对应的交易器
        var trader Trader
        var err error

        // 记录仓位模式（通用）
        marginModeStr := "全仓"
        if !config.IsCrossMargin {
                marginModeStr = "逐仓"
        }
        log.Printf("📊 [%s] 仓位模式: %s", config.Name, marginModeStr)

        switch config.Exchange {
        case "binance":
                log.Printf("🏦 [%s] 使用币安合约交易", config.Name)
                trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey)
        case "hyperliquid":
                log.Printf("🏦 [%s] 使用Hyperliquid交易", config.Name)
                trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
                if err != nil {
                        return nil, fmt.Errorf("初始化Hyperliquid交易器失败: %w", err)
                }
        case "aster":
                log.Printf("🏦 [%s] 使用Aster交易", config.Name)
                trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
                if err != nil {
                        return nil, fmt.Errorf("初始化Aster交易器失败: %w", err)
                }
        case "okx":
                log.Printf("🏦 [%s] 使用OKX交易", config.Name)
                trader, err = NewOKXTrader(config.OKXAPIKey, config.OKXSecretKey, config.OKXPassphrase, config.OKXTestnet)
                if err != nil {
                        return nil, fmt.Errorf("初始化OKX交易器失败: %w", err)
                }
        default:
                return nil, fmt.Errorf("不支持的交易平台: %s", config.Exchange)
        }

        // 验证初始金额配置
        if config.InitialBalance <= 0 {
                return nil, fmt.Errorf("初始金额必须大于0，请在配置中设置InitialBalance")
        }

        // 初始化决策日志记录器（使用trader ID创建独立目录）
        logDir := fmt.Sprintf("decision_logs/%s", config.ID)
        decisionLogger := logger.NewDecisionLogger(logDir)

        // 初始化凯利公式止盈止损管理器
        kellyManager := decision.NewKellyStopManager()

        // 设置默认系统提示词模板
        systemPromptTemplate := config.SystemPromptTemplate
        if systemPromptTemplate == "" {
                systemPromptTemplate = "default" // 默认使用 default 模板
        }

        return &AutoTrader{
                id:                    config.ID,
                name:                  config.Name,
                aiModel:               config.AIModel,
                exchange:              config.Exchange,
                config:                config,
                trader:                trader,
                mcpClient:             mcpClient,
                decisionLogger:        decisionLogger,
                kellyManager:          kellyManager,
                initialBalance:        config.InitialBalance,
                systemPromptTemplate:  systemPromptTemplate,
                defaultCoins:          config.DefaultCoins,
                tradingCoins:          config.TradingCoins,
                lastResetTime:         time.Now(),
                startTime:             time.Now(),
                callCount:             0,
                isRunning:             false,
                positionFirstSeenTime: make(map[string]int64),
        }, nil
}

// Run 运行自动交易主循环
func (at *AutoTrader) Run() error {
        at.isRunning = true
        log.Println("🚀 AI驱动自动交易系统启动")
        log.Printf("💰 初始余额: %.2f USDT", at.initialBalance)
        log.Printf("⚙️  扫描间隔: %v", at.config.ScanInterval)
        log.Println("🤖 AI将全权决定杠杆、仓位大小、止损止盈等参数")

        ticker := time.NewTicker(at.config.ScanInterval)
        defer ticker.Stop()

        // 首次立即执行
        if err := at.runCycle(); err != nil {
                log.Printf("❌ 执行失败: %v", err)
        }

        for at.isRunning {
                select {
                case <-ticker.C:
                        if err := at.runCycle(); err != nil {
                                log.Printf("❌ 执行失败: %v", err)
                        }
                }
        }

        return nil
}

// Stop 停止自动交易
func (at *AutoTrader) Stop() {
        at.isRunning = false
        log.Println("⏹ 自动交易系统停止")
}

// runCycle 运行一个交易周期（使用AI全权决策）
func (at *AutoTrader) runCycle() error {
        at.callCount++

        log.Println(strings.Repeat("=", 70))
        log.Printf("⏰ %s - AI决策周期 #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
        log.Println(strings.Repeat("=", 70))

        // 创建决策记录
        record := &logger.DecisionRecord{
                ExecutionLog: []string{},
                Success:      true,
        }

        // 1. 检查是否需要停止交易
        if time.Now().Before(at.stopUntil) {
                remaining := at.stopUntil.Sub(time.Now())
                log.Printf("⏸ 风险控制：暂停交易中，剩余 %.0f 分钟", remaining.Minutes())
                record.Success = false
                record.ErrorMessage = fmt.Sprintf("风险控制暂停中，剩余 %.0f 分钟", remaining.Minutes())
                at.decisionLogger.LogDecision(record)
                return nil
        }

        // 2. 重置日盈亏（每天重置）
        if time.Since(at.lastResetTime) > 24*time.Hour {
                at.dailyPnL = 0
                at.lastResetTime = time.Now()
                log.Println("📅 日盈亏已重置")
        }

        // 3. 收集交易上下文
        ctx, err := at.buildTradingContext()
        if err != nil {
                record.Success = false
                record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
                at.decisionLogger.LogDecision(record)
                return fmt.Errorf("构建交易上下文失败: %w", err)
        }

        // 保存账户状态快照
        record.AccountState = logger.AccountSnapshot{
                TotalBalance:          ctx.Account.TotalEquity,
                AvailableBalance:      ctx.Account.AvailableBalance,
                TotalUnrealizedProfit: ctx.Account.TotalPnL,
                PositionCount:         ctx.Account.PositionCount,
                MarginUsedPct:         ctx.Account.MarginUsedPct,
        }

        // 保存持仓快照
        for _, pos := range ctx.Positions {
                record.Positions = append(record.Positions, logger.PositionSnapshot{
                        Symbol:           pos.Symbol,
                        Side:             pos.Side,
                        PositionAmt:      pos.Quantity,
                        EntryPrice:       pos.EntryPrice,
                        MarkPrice:        pos.MarkPrice,
                        UnrealizedProfit: pos.UnrealizedPnL,
                        Leverage:         float64(pos.Leverage),
                        LiquidationPrice: pos.LiquidationPrice,
                })
        }

        // 保存候选币种列表
        for _, coin := range ctx.CandidateCoins {
                record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
        }

        log.Printf("📊 账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
                ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

        // 4. 调用AI获取完整决策
        log.Printf("🤖 正在请求AI分析并决策... [模板: %s]", at.systemPromptTemplate)
        decision, err := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, at.customPrompt, at.overrideBasePrompt, at.systemPromptTemplate)

        // 即使有错误，也保存思维链、决策和输入prompt（用于debug）
        if decision != nil {
                record.SystemPrompt = decision.SystemPrompt // 保存系统提示词
                record.InputPrompt = decision.UserPrompt
                record.CoTTrace = decision.CoTTrace
                if len(decision.Decisions) > 0 {
                        decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
                        record.DecisionJSON = string(decisionJSON)
                }
        }

        if err != nil {
                record.Success = false
                record.ErrorMessage = fmt.Sprintf("获取AI决策失败: %v", err)

                // 打印系统提示词和AI思维链（即使有错误，也要输出以便调试）
                if decision != nil {
                        if decision.SystemPrompt != "" {
                                log.Println(strings.Repeat("=", 70))
                                log.Printf("📋 系统提示词 [模板: %s] (错误情况)", at.systemPromptTemplate)
                                log.Println(strings.Repeat("=", 70))
                                log.Println(decision.SystemPrompt)
                                log.Println(strings.Repeat("=", 70))
                        }

                        if decision.CoTTrace != "" {
                                log.Println(strings.Repeat("-", 70))
                                log.Println("💭 AI思维链分析（错误情况）:")
                                log.Println(strings.Repeat("-", 70))
                                log.Println(decision.CoTTrace)
                                log.Println(strings.Repeat("-", 70))
                        }
                }

                at.decisionLogger.LogDecision(record)
                return fmt.Errorf("获取AI决策失败: %w", err)
        }

        // // 5. 打印系统提示词
        // log.Printf("\n" + strings.Repeat("=", 70))
        // log.Printf("📋 系统提示词 [模板: %s]", at.systemPromptTemplate)
        // log.Println(strings.Repeat("=", 70))
        // log.Println(decision.SystemPrompt)
        // log.Printf(strings.Repeat("=", 70) + "\n")

        // 6. 打印AI思维链
        // log.Printf("\n" + strings.Repeat("-", 70))
        // log.Println("💭 AI思维链分析:")
        // log.Println(strings.Repeat("-", 70))
        // log.Println(decision.CoTTrace)
        // log.Printf(strings.Repeat("-", 70) + "\n")

        // 7. 打印AI决策
        // log.Printf("📋 AI决策列表 (%d 个):\n", len(decision.Decisions))
        // for i, d := range decision.Decisions {
        //      log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
        //      if d.Action == "open_long" || d.Action == "open_short" {
        //              log.Printf("      杠杆: %dx | 仓位: %.2f USDT | 止损: %.4f | 止盈: %.4f",
        //                      d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
        //      }
        // }
        log.Println()

        // 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
        sortedDecisions := sortDecisionsByPriority(decision.Decisions)

        log.Println("🔄 执行顺序（已优化）: 先平仓→后开仓")
        for i, d := range sortedDecisions {
                log.Printf("  [%d] %s %s", i+1, d.Symbol, d.Action)
        }
        log.Println()

        // 执行决策并记录结果
        for _, d := range sortedDecisions {
                actionRecord := logger.DecisionAction{
                        Action:    d.Action,
                        Symbol:    d.Symbol,
                        Quantity:  0,
                        Leverage:  d.Leverage,
                        Price:     0,
                        Timestamp: time.Now(),
                        Success:   false,
                }

                if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
                        log.Printf("❌ 执行决策失败 (%s %s): %v", d.Symbol, d.Action, err)
                        actionRecord.Error = err.Error()
                        record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
                } else {
                        actionRecord.Success = true
                        record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
                        // 成功执行后短暂延迟
                        time.Sleep(1 * time.Second)
                }

                record.Decisions = append(record.Decisions, actionRecord)
        }

        // 9. 检查并更新现有持仓的止盈止损单（使用凯利公式优化）
        log.Println("🔄 开始执行凯利公式动态止盈止损检查...")
        if err := at.checkAndUpdateStopOrders(); err != nil {
                log.Printf("⚠ 更新止盈止损单失败: %v", err)
                record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("⚠ 止盈止损更新失败: %v", err))
        } else {
                log.Println("✅ 所有持仓的止盈止损单已更新（凯利公式优化）")
                record.ExecutionLog = append(record.ExecutionLog, "✅ 止盈止损单已更新")
        }

        // 10. 保存决策记录
        if err := at.decisionLogger.LogDecision(record); err != nil {
                log.Printf("⚠ 保存决策记录失败: %v", err)
        }

        return nil
}

// buildTradingContext 构建交易上下文
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
        // 1. 获取账户信息
        balance, err := at.trader.GetBalance()
        if err != nil {
                return nil, fmt.Errorf("获取账户余额失败: %w", err)
        }

        // 获取账户字段（使用OKXTrader返回的正确字段名）
        totalWalletBalance := 0.0
        totalUnrealizedProfit := 0.0
        availableBalance := 0.0

        // 从OKXTrader.GetBalance()返回的"total"字段获取总资产
        if total, ok := balance["total"].(float64); ok {
                totalWalletBalance = total
                log.Printf("✓ 从OKX获取总资产: %.8f", total)
        }
        // 从OKXTrader.GetBalance()返回的"free"字段获取可用余额
        if free, ok := balance["free"].(float64); ok {
                availableBalance = free
                log.Printf("✓ 从OKX获取可用余额: %.8f", free)
        }
        // 从OKXTrader.GetBalance()返回的"used"字段获取已用余额
        if used, ok := balance["used"].(float64); ok {
                log.Printf("✓ 从OKX获取已用余额: %.8f", used)
        }

        // Total Equity = 钱包余额 + 未实现盈亏
        totalEquity := totalWalletBalance + totalUnrealizedProfit

        // 2. 获取持仓信息
        positions, err := at.trader.GetPositions()
        if err != nil {
                return nil, fmt.Errorf("获取持仓失败: %w", err)
        }

        var positionInfos []decision.PositionInfo
        totalMarginUsed := 0.0

        // 当前持仓的key集合（用于清理已平仓的记录）
        currentPositionKeys := make(map[string]bool)

        for _, pos := range positions {
                // 安全类型断言，避免nil值导致panic
                symbol := ""
                if s, ok := pos["symbol"].(string); ok {
                        symbol = s
                }
                side := ""
                if s, ok := pos["side"].(string); ok {
                        side = s
                }
                entryPrice := 0.0
                if ep, ok := pos["entryPrice"].(float64); ok {
                        entryPrice = ep
                }
                markPrice := 0.0
                if mp, ok := pos["markPrice"].(float64); ok {
                        markPrice = mp
                }
                quantity := 0.0
                if qty, ok := pos["positionAmt"].(float64); ok {
                        quantity = qty
                }
                if quantity < 0 {
                        quantity = -quantity // 空仓数量为负，转为正数
                }
                unrealizedPnl := 0.0
                if upl, ok := pos["unRealizedProfit"].(float64); ok {
                        unrealizedPnl = upl
                }
                liquidationPrice := 0.0
                if lp, ok := pos["liquidationPrice"].(float64); ok {
                        liquidationPrice = lp
                }

                // 跳过无效持仓数据
                if symbol == "" || side == "" || markPrice == 0 {
                        continue
                }

                // 计算盈亏百分比
                pnlPct := 0.0
                if entryPrice > 0 {
                        if side == "long" {
                                pnlPct = ((markPrice - entryPrice) / entryPrice) * 100
                        } else {
                                pnlPct = ((entryPrice - markPrice) / entryPrice) * 100
                        }
                }

                // 计算占用保证金（估算）
                leverage := 10 // 默认值，实际应该从持仓信息获取
                if lev, ok := pos["leverage"].(float64); ok {
                        leverage = int(lev)
                }
                marginUsed := (quantity * markPrice) / float64(leverage)
                totalMarginUsed += marginUsed

                // 跟踪持仓首次出现时间
                posKey := symbol + "_" + side
                currentPositionKeys[posKey] = true
                if _, exists := at.positionFirstSeenTime[posKey]; !exists {
                        // 新持仓，记录当前时间
                        at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
                }
                updateTime := at.positionFirstSeenTime[posKey]

                positionInfos = append(positionInfos, decision.PositionInfo{
                        Symbol:           symbol,
                        Side:             side,
                        EntryPrice:       entryPrice,
                        MarkPrice:        markPrice,
                        Quantity:         quantity,
                        Leverage:         leverage,
                        UnrealizedPnL:    unrealizedPnl,
                        UnrealizedPnLPct: pnlPct,
                        LiquidationPrice: liquidationPrice,
                        MarginUsed:       marginUsed,
                        UpdateTime:       updateTime,
                })
        }

        // 清理已平仓的持仓记录
        for key := range at.positionFirstSeenTime {
                if !currentPositionKeys[key] {
                        delete(at.positionFirstSeenTime, key)
                }
        }

        // 3. 获取交易员的候选币种池
        candidateCoins, err := at.getCandidateCoins()
        if err != nil {
                return nil, fmt.Errorf("获取候选币种失败: %w", err)
        }

        // 4. 计算总盈亏
        totalPnL := totalEquity - at.initialBalance
        totalPnLPct := 0.0
        if at.initialBalance > 0 {
                totalPnLPct = (totalPnL / at.initialBalance) * 100
        }

        marginUsedPct := 0.0
        if totalEquity > 0 {
                marginUsedPct = (totalMarginUsed / totalEquity) * 100
        }

        // 5. 分析历史表现（最近100个周期，避免长期持仓的交易记录丢失）
        // 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
        performance, err := at.decisionLogger.AnalyzePerformance(100)
        if err != nil {
                log.Printf("⚠️  分析历史表现失败: %v", err)
                // 不影响主流程，继续执行（但设置performance为nil以避免传递错误数据）
                performance = nil
        }

        // 6. 构建上下文
        ctx := &decision.Context{
                CurrentTime:     time.Now().Format("2006-01-02 15:04:05"),
                RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
                CallCount:       at.callCount,
                BTCETHLeverage:  at.config.BTCETHLeverage,  // 使用配置的杠杆倍数
                AltcoinLeverage: at.config.AltcoinLeverage, // 使用配置的杠杆倍数
                Account: decision.AccountInfo{
                        TotalEquity:      totalEquity,
                        AvailableBalance: availableBalance,
                        TotalPnL:         totalPnL,
                        TotalPnLPct:      totalPnLPct,
                        MarginUsed:       totalMarginUsed,
                        MarginUsedPct:    marginUsedPct,
                        PositionCount:    len(positionInfos),
                },
                Positions:      positionInfos,
                CandidateCoins: candidateCoins,
                Performance:    performance, // 添加历史表现分析
        }

        return ctx, nil
}

// executeDecisionWithRecord 执行AI决策并记录详细信息
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
        switch decision.Action {
        case "open_long":
                return at.executeOpenLongWithRecord(decision, actionRecord)
        case "open_short":
                return at.executeOpenShortWithRecord(decision, actionRecord)
        case "close_long":
                return at.executeCloseLongWithRecord(decision, actionRecord)
        case "close_short":
                return at.executeCloseShortWithRecord(decision, actionRecord)
        case "hold", "wait":
                // 无需执行，仅记录
                return nil
        default:
                return fmt.Errorf("未知的action: %s", decision.Action)
        }
}

// executeOpenLongWithRecord 执行开多仓并记录详细信息
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
        log.Printf("  📈 开多仓: %s", decision.Symbol)

        // ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
        positions, err := at.trader.GetPositions()
        if err == nil {
                for _, pos := range positions {
                        if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
                                return fmt.Errorf("❌ %s 已有多仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_long 决策", decision.Symbol)
                        }
                }
        }

        // 获取当前价格
        marketData, err := market.Get(decision.Symbol)
        if err != nil {
                return err
        }

        // ===== 保证金检查与自动调整 =====
        adjustedPositionSizeUSD := decision.PositionSizeUSD
        balance, balanceErr := at.trader.GetBalance()
        if balanceErr == nil {
                availableBalance := 0.0
                if free, ok := balance["free"].(float64); ok {
                        availableBalance = free
                }
                
                // 计算最大可开仓价值 = 可用保证金 * 80% * 杠杆
                // 保留20%作为安全边际，防止价格波动导致保证金不足
                maxMarginToUse := availableBalance * 0.80
                maxPositionValue := maxMarginToUse * float64(decision.Leverage)
                
                if decision.PositionSizeUSD > maxPositionValue {
                        log.Printf("  ⚠️ 保证金检查: AI请求开仓 $%.2f，但可用保证金 $%.2f (80%% = $%.2f)，杠杆 %dx，最大可开仓 $%.2f",
                                decision.PositionSizeUSD, availableBalance, maxMarginToUse, decision.Leverage, maxPositionValue)
                        
                        if maxPositionValue < 10 {
                                // 如果最大可开仓金额太小（<$10），拒绝开仓
                                return fmt.Errorf("保证金不足: 可用 $%.2f, 需要至少 $%.2f 保证金才能开仓 (杠杆 %dx)", 
                                        availableBalance, decision.PositionSizeUSD/float64(decision.Leverage)/0.8, decision.Leverage)
                        }
                        
                        // 自动调整到最大可开仓值
                        adjustedPositionSizeUSD = maxPositionValue
                        log.Printf("  ✅ 自动调整开仓金额: $%.2f -> $%.2f (可用保证金的80%%)", decision.PositionSizeUSD, adjustedPositionSizeUSD)
                } else {
                        log.Printf("  ✅ 保证金检查通过: 开仓 $%.2f, 可用保证金 $%.2f, 杠杆 %dx", 
                                decision.PositionSizeUSD, availableBalance, decision.Leverage)
                }
        } else {
                log.Printf("  ⚠️ 无法获取账户余额进行保证金检查: %v, 继续使用AI决定的仓位", balanceErr)
        }
        
        // 最小开仓金额检查（无论是否调整过，都需要检查）
        const minPositionSizeUSD = 10.0
        if adjustedPositionSizeUSD < minPositionSizeUSD {
                return fmt.Errorf("开仓金额过小: $%.2f < 最小要求 $%.2f", adjustedPositionSizeUSD, minPositionSizeUSD)
        }
        // ===== 保证金检查结束 =====

        // 计算数量（使用调整后的仓位大小）
        quantity := adjustedPositionSizeUSD / marketData.CurrentPrice
        actionRecord.Quantity = quantity
        actionRecord.Price = marketData.CurrentPrice

        // 设置仓位模式
        if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
                log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
                // 继续执行，不影响交易
        }

        // 开仓
        order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
        if err != nil {
                return err
        }

        // 记录订单ID
        if orderID, ok := order["orderId"].(int64); ok {
                actionRecord.OrderID = orderID
        }

        log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

        // 记录开仓时间
        posKey := decision.Symbol + "_long"
        at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

        // 设置止损止盈
        if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
                log.Printf("  ⚠ 设置止损失败: %v", err)
        }
        if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
                log.Printf("  ⚠ 设置止盈失败: %v", err)
        }

        return nil
}

// executeOpenShortWithRecord 执行开空仓并记录详细信息
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
        log.Printf("  📉 开空仓: %s", decision.Symbol)

        // ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
        positions, err := at.trader.GetPositions()
        if err == nil {
                for _, pos := range positions {
                        if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
                                return fmt.Errorf("❌ %s 已有空仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_short 决策", decision.Symbol)
                        }
                }
        }

        // 获取当前价格
        marketData, err := market.Get(decision.Symbol)
        if err != nil {
                return err
        }

        // ===== 保证金检查与自动调整 =====
        adjustedPositionSizeUSD := decision.PositionSizeUSD
        balance, balanceErr := at.trader.GetBalance()
        if balanceErr == nil {
                availableBalance := 0.0
                if free, ok := balance["free"].(float64); ok {
                        availableBalance = free
                }
                
                // 计算最大可开仓价值 = 可用保证金 * 80% * 杠杆
                // 保留20%作为安全边际，防止价格波动导致保证金不足
                maxMarginToUse := availableBalance * 0.80
                maxPositionValue := maxMarginToUse * float64(decision.Leverage)
                
                if decision.PositionSizeUSD > maxPositionValue {
                        log.Printf("  ⚠️ 保证金检查: AI请求开仓 $%.2f，但可用保证金 $%.2f (80%% = $%.2f)，杠杆 %dx，最大可开仓 $%.2f",
                                decision.PositionSizeUSD, availableBalance, maxMarginToUse, decision.Leverage, maxPositionValue)
                        
                        if maxPositionValue < 10 {
                                // 如果最大可开仓金额太小（<$10），拒绝开仓
                                return fmt.Errorf("保证金不足: 可用 $%.2f, 需要至少 $%.2f 保证金才能开仓 (杠杆 %dx)", 
                                        availableBalance, decision.PositionSizeUSD/float64(decision.Leverage)/0.8, decision.Leverage)
                        }
                        
                        // 自动调整到最大可开仓值
                        adjustedPositionSizeUSD = maxPositionValue
                        log.Printf("  ✅ 自动调整开仓金额: $%.2f -> $%.2f (可用保证金的80%%)", decision.PositionSizeUSD, adjustedPositionSizeUSD)
                } else {
                        log.Printf("  ✅ 保证金检查通过: 开仓 $%.2f, 可用保证金 $%.2f, 杠杆 %dx", 
                                decision.PositionSizeUSD, availableBalance, decision.Leverage)
                }
        } else {
                log.Printf("  ⚠️ 无法获取账户余额进行保证金检查: %v, 继续使用AI决定的仓位", balanceErr)
        }
        
        // 最小开仓金额检查（无论是否调整过，都需要检查）
        const minPositionSizeUSD = 10.0
        if adjustedPositionSizeUSD < minPositionSizeUSD {
                return fmt.Errorf("开仓金额过小: $%.2f < 最小要求 $%.2f", adjustedPositionSizeUSD, minPositionSizeUSD)
        }
        // ===== 保证金检查结束 =====

        // 计算数量（使用调整后的仓位大小）
        quantity := adjustedPositionSizeUSD / marketData.CurrentPrice
        actionRecord.Quantity = quantity
        actionRecord.Price = marketData.CurrentPrice

        // 设置仓位模式
        if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
                log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
                // 继续执行，不影响交易
        }

        // 开仓
        order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
        if err != nil {
                return err
        }

        // 记录订单ID
        if orderID, ok := order["orderId"].(int64); ok {
                actionRecord.OrderID = orderID
        }

        log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

        // 记录开仓时间
        posKey := decision.Symbol + "_short"
        at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

        // 设置止损止盈
        if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
                log.Printf("  ⚠ 设置止损失败: %v", err)
        }
        if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
                log.Printf("  ⚠ 设置止盈失败: %v", err)
        }

        return nil
}

// executeCloseLongWithRecord 执行平多仓并记录详细信息
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
        log.Printf("  🔄 平多仓: %s", decision.Symbol)

        // 记录平仓前持仓信息（用于计算盈利）
        positions, err := at.trader.GetPositions()
        if err == nil {
                for _, pos := range positions {
                        if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
                                // 获取持仓详情
                                entryPrice, _ := pos["entryPrice"].(float64)
                                markPrice, _ := pos["markPrice"].(float64)
                                unrealizedPnl, _ := pos["unRealizedProfit"].(float64)

                                // 计算盈亏百分比
                                profitPct := 0.0
                                if entryPrice > 0 {
                                        profitPct = (markPrice - entryPrice) / entryPrice
                                }

                                // 记录交易结果（延迟到平仓成功后执行）
                                defer func(symbol string, isWin bool, profit float64) {
                                        at.recordTradeResult(symbol, isWin, profit)
                                }(decision.Symbol, profitPct >= 0, profitPct*100)

                                log.Printf("  📊 平仓前: 入场价=%.6f, 当前价=%.6f, 未实现盈亏=%.2f, 盈亏比例=%.2f%%",
                                        entryPrice, markPrice, unrealizedPnl, profitPct*100)
                                break
                        }
                }
        }

        // 获取当前价格
        marketData, err := market.Get(decision.Symbol)
        if err != nil {
                return err
        }
        actionRecord.Price = marketData.CurrentPrice

        // 平仓
        order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = 全部平仓
        if err != nil {
                return err
        }

        // 记录订单ID
        if orderID, ok := order["orderId"].(int64); ok {
                actionRecord.OrderID = orderID
        }

        log.Printf("  ✓ 平多仓成功")
        return nil
}

// executeCloseShortWithRecord 执行平空仓并记录详细信息
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
        log.Printf("  🔄 平空仓: %s", decision.Symbol)

        // 记录平仓前持仓信息（用于计算盈利）
        positions, err := at.trader.GetPositions()
        if err == nil {
                for _, pos := range positions {
                        if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
                                // 获取持仓详情
                                entryPrice, _ := pos["entryPrice"].(float64)
                                markPrice, _ := pos["markPrice"].(float64)
                                unrealizedPnl, _ := pos["unRealizedProfit"].(float64)

                                // 计算盈亏百分比（空仓相反）
                                profitPct := 0.0
                                if entryPrice > 0 {
                                        profitPct = (entryPrice - markPrice) / entryPrice
                                }

                                // 记录交易结果（延迟到平仓成功后执行）
                                defer func(symbol string, isWin bool, profit float64) {
                                        at.recordTradeResult(symbol, isWin, profit)
                                }(decision.Symbol, profitPct >= 0, profitPct*100)

                                log.Printf("  📊 平仓前: 入场价=%.6f, 当前价=%.6f, 未实现盈亏=%.2f, 盈亏比例=%.2f%%",
                                        entryPrice, markPrice, unrealizedPnl, profitPct*100)
                                break
                        }
                }
        }

        // 获取当前价格
        marketData, err := market.Get(decision.Symbol)
        if err != nil {
                return err
        }
        actionRecord.Price = marketData.CurrentPrice

        // 平仓
        order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = 全部平仓
        if err != nil {
                return err
        }

        // 记录订单ID
        if orderID, ok := order["orderId"].(int64); ok {
                actionRecord.OrderID = orderID
        }

        log.Printf("  ✓ 平空仓成功")
        return nil
}

// GetID 获取trader ID
func (at *AutoTrader) GetID() string {
        return at.id
}

// GetName 获取trader名称
func (at *AutoTrader) GetName() string {
        return at.name
}

// GetAIModel 获取AI模型
func (at *AutoTrader) GetAIModel() string {
        return at.aiModel
}

// GetExchange 获取交易所
func (at *AutoTrader) GetExchange() string {
        return at.exchange
}

// SetCustomPrompt 设置自定义交易策略prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
        at.customPrompt = prompt
}

// SetOverrideBasePrompt 设置是否覆盖基础prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
        at.overrideBasePrompt = override
}

// SetSystemPromptTemplate 设置系统提示词模板
func (at *AutoTrader) SetSystemPromptTemplate(templateName string) {
        at.systemPromptTemplate = templateName
}

// GetSystemPromptTemplate 获取当前系统提示词模板名称
func (at *AutoTrader) GetSystemPromptTemplate() string {
        return at.systemPromptTemplate
}

// GetDecisionLogger 获取决策日志记录器
func (at *AutoTrader) GetDecisionLogger() *logger.DecisionLogger {
        return at.decisionLogger
}

// GetStatus 获取系统状态（用于API）
func (at *AutoTrader) GetStatus() map[string]interface{} {
        aiProvider := "DeepSeek"
        if at.config.UseQwen {
                aiProvider = "Qwen"
        }

        return map[string]interface{}{
                "trader_id":       at.id,
                "trader_name":     at.name,
                "ai_model":        at.aiModel,
                "exchange":        at.exchange,
                "is_running":      at.isRunning,
                "start_time":      at.startTime.Format(time.RFC3339),
                "runtime_minutes": int(time.Since(at.startTime).Minutes()),
                "call_count":      at.callCount,
                "initial_balance": at.initialBalance,
                "scan_interval":   at.config.ScanInterval.String(),
                "stop_until":      at.stopUntil.Format(time.RFC3339),
                "last_reset_time": at.lastResetTime.Format(time.RFC3339),
                "ai_provider":     aiProvider,
        }
}

// GetAccountInfo 获取账户信息（用于API）
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
        balance, err := at.trader.GetBalance()
        if err != nil {
                return nil, fmt.Errorf("获取余额失败: %w", err)
        }

        // 获取账户字段（使用OKXTrader返回的正确字段名）
        totalWalletBalance := 0.0
        totalUnrealizedProfit := 0.0
        availableBalance := 0.0

        // 从OKXTrader.GetBalance()返回的"total"字段获取总资产
        if total, ok := balance["total"].(float64); ok {
                totalWalletBalance = total
                log.Printf("✓ 从OKX获取总资产: %.8f", total)
        }
        // 从OKXTrader.GetBalance()返回的"free"字段获取可用余额
        if free, ok := balance["free"].(float64); ok {
                availableBalance = free
                log.Printf("✓ 从OKX获取可用余额: %.8f", free)
        }
        // 从OKXTrader.GetBalance()返回的"used"字段获取已用余额
        if used, ok := balance["used"].(float64); ok {
                log.Printf("✓ 从OKX获取已用余额: %.8f", used)
        }

        // Total Equity = 钱包余额 + 未实现盈亏
        totalEquity := totalWalletBalance + totalUnrealizedProfit

        if totalWalletBalance > 0 {
                log.Printf("✓ 账户余额映射成功: 总资产=%.2f, 可用=%.2f",
                        totalWalletBalance, availableBalance)
        }

        // 获取持仓计算总保证金
        positions, err := at.trader.GetPositions()
        if err != nil {
                return nil, fmt.Errorf("获取持仓失败: %w", err)
        }

        totalMarginUsed := 0.0
        totalUnrealizedPnL := 0.0
        for _, pos := range positions {
                // 安全类型断言，避免nil值导致panic
                markPrice := 0.0
                if mp, ok := pos["markPrice"].(float64); ok {
                        markPrice = mp
                }
                
                quantity := 0.0
                if qty, ok := pos["positionAmt"].(float64); ok {
                        quantity = qty
                }
                if quantity < 0 {
                        quantity = -quantity
                }
                
                unrealizedPnl := 0.0
                if upl, ok := pos["unRealizedProfit"].(float64); ok {
                        unrealizedPnl = upl
                }
                totalUnrealizedPnL += unrealizedPnl

                leverage := 10
                if lev, ok := pos["leverage"].(float64); ok {
                        leverage = int(lev)
                }
                
                // 跳过无效持仓数据
                if markPrice == 0 || quantity == 0 {
                        continue
                }
                
                marginUsed := (quantity * markPrice) / float64(leverage)
                totalMarginUsed += marginUsed
        }

        totalPnL := totalEquity - at.initialBalance
        totalPnLPct := 0.0
        if at.initialBalance > 0 {
                totalPnLPct = (totalPnL / at.initialBalance) * 100
        }

        marginUsedPct := 0.0
        if totalEquity > 0 {
                marginUsedPct = (totalMarginUsed / totalEquity) * 100
        }

        return map[string]interface{}{
                // 核心字段
                "total_equity":      totalEquity,           // 账户净值 = wallet + unrealized
                "wallet_balance":    totalWalletBalance,    // 钱包余额（不含未实现盈亏）
                "unrealized_profit": totalUnrealizedProfit, // 未实现盈亏（从API）
                "available_balance": availableBalance,      // 可用余额

                // 盈亏统计
                "total_pnl":            totalPnL,           // 总盈亏 = equity - initial
                "total_pnl_pct":        totalPnLPct,        // 总盈亏百分比
                "total_unrealized_pnl": totalUnrealizedPnL, // 未实现盈亏（从持仓计算）
                "initial_balance":      at.initialBalance,  // 初始余额
                "daily_pnl":            at.dailyPnL,        // 日盈亏

                // 持仓信息
                "position_count":  len(positions),  // 持仓数量
                "margin_used":     totalMarginUsed, // 保证金占用
                "margin_used_pct": marginUsedPct,   // 保证金使用率
        }, nil
}

// GetPositions 获取持仓列表（用于API）
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
        positions, err := at.trader.GetPositions()
        if err != nil {
                return nil, fmt.Errorf("获取持仓失败: %w", err)
        }

        var result []map[string]interface{}
        for _, pos := range positions {
                // 安全类型断言，避免nil值导致panic
                symbol := ""
                if s, ok := pos["symbol"].(string); ok {
                        symbol = s
                }
                side := ""
                if s, ok := pos["side"].(string); ok {
                        side = s
                }
                entryPrice := 0.0
                if ep, ok := pos["entryPrice"].(float64); ok {
                        entryPrice = ep
                }
                markPrice := 0.0
                if mp, ok := pos["markPrice"].(float64); ok {
                        markPrice = mp
                }
                quantity := 0.0
                if qty, ok := pos["positionAmt"].(float64); ok {
                        quantity = qty
                }
                if quantity < 0 {
                        quantity = -quantity
                }
                unrealizedPnl := 0.0
                if upl, ok := pos["unRealizedProfit"].(float64); ok {
                        unrealizedPnl = upl
                }
                liquidationPrice := 0.0
                if lp, ok := pos["liquidationPrice"].(float64); ok {
                        liquidationPrice = lp
                }

                // 跳过无效持仓数据
                if symbol == "" || side == "" || markPrice == 0 {
                        continue
                }

                leverage := 10
                if lev, ok := pos["leverage"].(float64); ok {
                        leverage = int(lev)
                }

                // 计算占用保证金
                marginUsed := (quantity * markPrice) / float64(leverage)

                // 计算盈亏百分比（基于保证金）
                // 收益率 = 未实现盈亏 / 保证金 × 100%
                pnlPct := 0.0
                if marginUsed > 0 {
                        pnlPct = (unrealizedPnl / marginUsed) * 100
                }

                result = append(result, map[string]interface{}{
                        "symbol":             symbol,
                        "side":               side,
                        "entry_price":        entryPrice,
                        "mark_price":         markPrice,
                        "quantity":           quantity,
                        "leverage":           leverage,
                        "unrealized_pnl":     unrealizedPnl,
                        "unrealized_pnl_pct": pnlPct,
                        "liquidation_price":  liquidationPrice,
                        "margin_used":        marginUsed,
                })
        }

        return result, nil
}

// sortDecisionsByPriority 对决策排序：先平仓，再开仓，最后hold/wait
// 这样可以避免换仓时仓位叠加超限
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
        if len(decisions) <= 1 {
                return decisions
        }

        // 定义优先级
        getActionPriority := func(action string) int {
                switch action {
                case "close_long", "close_short":
                        return 1 // 最高优先级：先平仓
                case "open_long", "open_short":
                        return 2 // 次优先级：后开仓
                case "hold", "wait":
                        return 3 // 最低优先级：观望
                default:
                        return 999 // 未知动作放最后
                }
        }

        // 复制决策列表
        sorted := make([]decision.Decision, len(decisions))
        copy(sorted, decisions)

        // 按优先级排序
        for i := 0; i < len(sorted)-1; i++ {
                for j := i + 1; j < len(sorted); j++ {
                        if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
                                sorted[i], sorted[j] = sorted[j], sorted[i]
                        }
                }
        }

        return sorted
}

// getCandidateCoins 获取交易员的候选币种列表
func (at *AutoTrader) getCandidateCoins() ([]decision.CandidateCoin, error) {
        if len(at.tradingCoins) == 0 {
                // 使用数据库配置的默认币种列表
                var candidateCoins []decision.CandidateCoin

                if len(at.defaultCoins) > 0 {
                        // 使用数据库中配置的默认币种
                        for _, coin := range at.defaultCoins {
                                symbol := normalizeSymbol(coin)
                                candidateCoins = append(candidateCoins, decision.CandidateCoin{
                                        Symbol:  symbol,
                                        Sources: []string{"default"}, // 标记为数据库默认币种
                                })
                        }
                        log.Printf("📋 [%s] 使用数据库默认币种: %d个币种 %v",
                                at.name, len(candidateCoins), at.defaultCoins)
                        return candidateCoins, nil
                } else {
                        // 如果数据库中没有配置默认币种，则使用AI500+OI Top作为fallback
                        const ai500Limit = 20 // AI500取前20个评分最高的币种

                        mergedPool, err := pool.GetMergedCoinPool(ai500Limit)
                        if err != nil {
                                return nil, fmt.Errorf("获取合并币种池失败: %w", err)
                        }

                        // 构建候选币种列表（包含来源信息）
                        for _, symbol := range mergedPool.AllSymbols {
                                sources := mergedPool.SymbolSources[symbol]
                                candidateCoins = append(candidateCoins, decision.CandidateCoin{
                                        Symbol:  symbol,
                                        Sources: sources, // "ai500" 和/或 "oi_top"
                                })
                        }

                        log.Printf("📋 [%s] 数据库无默认币种配置，使用AI500+OI Top: AI500前%d + OI_Top20 = 总计%d个候选币种",
                                at.name, ai500Limit, len(candidateCoins))
                        return candidateCoins, nil
                }
        } else {
                // 使用自定义币种列表
                var candidateCoins []decision.CandidateCoin
                for _, coin := range at.tradingCoins {
                        // 确保币种格式正确（转为大写USDT交易对）
                        symbol := normalizeSymbol(coin)
                        candidateCoins = append(candidateCoins, decision.CandidateCoin{
                                Symbol:  symbol,
                                Sources: []string{"custom"}, // 标记为自定义来源
                        })
                }

                log.Printf("📋 [%s] 使用自定义币种: %d个币种 %v",
                        at.name, len(candidateCoins), at.tradingCoins)
                return candidateCoins, nil
        }
}

// normalizeSymbol 标准化币种符号（确保以USDT结尾）
func normalizeSymbol(symbol string) string {
        // 转为大写
        symbol = strings.ToUpper(strings.TrimSpace(symbol))

        // 确保以USDT结尾
        if !strings.HasSuffix(symbol, "USDT") {
                symbol = symbol + "USDT"
        }

        return symbol
}

// checkAndUpdateStopOrders 检查并更新止盈止损单（使用凯利公式优化）
// 该方法在每个交易周期末尾调用，确保现有持仓的止盈止损点是最优的
func (at *AutoTrader) checkAndUpdateStopOrders() error {
        // 1. 获取当前持仓
        positions, err := at.trader.GetPositions()
        if err != nil {
                return fmt.Errorf("获取持仓失败: %w", err)
        }

        if len(positions) == 0 {
                log.Println("ℹ️ 当前无持仓，跳过止盈止损更新")
                return nil
        }

        log.Printf("📊 检查 %d 个持仓的止盈止损单...", len(positions))

        // 2. 对每个持仓进行止盈止损检查
        for _, pos := range positions {
                symbol := ""
                if s, ok := pos["symbol"].(string); ok {
                        symbol = s
                }
                side := ""
                if s, ok := pos["side"].(string); ok {
                        side = s
                }

                // 安全检查
                if symbol == "" || side == "" {
                        log.Printf("⚠️ 跳过无效持仓数据: %v", pos)
                        continue
                }

                // 3. 使用凯利公式计算动态止盈止损
                entryPrice := 0.0
                if ep, ok := pos["entryPrice"].(float64); ok {
                        entryPrice = ep
                }
                currentPrice := 0.0
                if mp, ok := pos["markPrice"].(float64); ok {
                        currentPrice = mp
                }

                if entryPrice <= 0 || currentPrice <= 0 {
                        log.Printf("⚠️ %s 无效价格: entry=%.6f, current=%.6f", symbol, entryPrice, currentPrice)
                        continue
                }

                // 计算当前盈利百分比
                currentProfitPct := 0.0
                if side == "long" {
                        currentProfitPct = (currentPrice - entryPrice) / entryPrice
                } else {
                        currentProfitPct = (entryPrice - currentPrice) / entryPrice
                }

                // 获取历史统计数据
                stats := at.kellyManager.GetHistoricalStats(symbol)

                log.Printf("🔄 [%s %s] 当前盈利: %.2f%%, 历史胜率: %.1f%%",
                        symbol, side, currentProfitPct*100, func() float64 {
                                if stats != nil {
                                        return stats.WinRate * 100
                                }
                                return 0
                        }())

                // 计算最优止盈点
                takeProfitPrice, err := at.kellyManager.CalculateOptimalTakeProfit(
                        symbol, entryPrice, currentPrice, side,
                )
                if err != nil {
                        log.Printf("⚠️ 计算止盈点失败 (%s): %v", symbol, err)
                        continue
                }

                // 计算动态止损点（保护已获利润）
                stopLossPrice, err := at.kellyManager.CalculateDynamicStopLoss(
                        symbol, entryPrice, currentPrice, 0, // maxProfitPct需要从历史数据计算
                )
                if err != nil {
                        log.Printf("⚠️ 计算止损点失败 (%s): %v", symbol, err)
                        continue
                }

                // 4. 更新止盈止损单
                quantity := 0.0
                if qty, ok := pos["positionAmt"].(float64); ok {
                        quantity = qty
                }
                if quantity < 0 {
                        quantity = -quantity
                }

                positionSide := strings.ToUpper(side)

                // 更新止损单
                if err := at.trader.SetStopLoss(symbol, positionSide, quantity, stopLossPrice); err != nil {
                        log.Printf("⚠️ 更新止损单失败 (%s %s @ %.6f): %v", symbol, positionSide, stopLossPrice, err)
                } else {
                        log.Printf("✅ 更新止损单成功: %s %s @ %.6f (保护%.1f%%利润)",
                                symbol, positionSide, stopLossPrice, currentProfitPct*100)
                }

                // 更新止盈单
                if err := at.trader.SetTakeProfit(symbol, positionSide, quantity, takeProfitPrice); err != nil {
                        log.Printf("⚠️ 更新止盈单失败 (%s %s @ %.6f): %v", symbol, positionSide, takeProfitPrice, err)
                } else {
                        log.Printf("✅ 更新止盈单成功: %s %s @ %.6f (目标%.1f%%收益)",
                                symbol, positionSide, takeProfitPrice,
                                func() float64 {
                                        if side == "long" {
                                                return (takeProfitPrice-entryPrice)/entryPrice*100
                                        }
                                        return (entryPrice-takeProfitPrice)/entryPrice*100
                                }())
                }

                // 更新历史统计数据（如果这次交易完成）
                // 注意：这里仅在持仓盈利或亏损时更新，实际平仓时会在平仓逻辑中更新
        }

        return nil
}

// recordTradeResult 记录交易结果到凯利公式管理器
// isWin: 是否盈利
// profitPct: 盈利百分比（正数为盈利，负数为亏损）
func (at *AutoTrader) recordTradeResult(symbol string, isWin bool, profitPct float64) {
        at.kellyManager.UpdateHistoricalStats(symbol, isWin, profitPct)
        log.Printf("📊 记录交易结果: %s %s, 盈利%.2f%%",
                symbol, func() string {
                        if isWin {
                                return "盈利"
                        }
                        return "亏损"
                }(), profitPct)
}
