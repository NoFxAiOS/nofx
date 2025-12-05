package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/store"
	"nofx/trader"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CompetitionCache 竞赛数据缓存
type CompetitionCache struct {
	data      map[string]interface{}
	timestamp time.Time
	mu        sync.RWMutex
}

// TraderManager 管理多个trader实例
type TraderManager struct {
	traders          map[string]*trader.AutoTrader // key: trader ID
	competitionCache *CompetitionCache
	mu               sync.RWMutex
}

// NewTraderManager 创建trader管理器
func NewTraderManager() *TraderManager {
	return &TraderManager{
		traders: make(map[string]*trader.AutoTrader),
		competitionCache: &CompetitionCache{
			data: make(map[string]interface{}),
		},
	}
}

// GetTrader 获取指定ID的trader
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.traders[id]
	if !exists {
		return nil, fmt.Errorf("trader ID '%s' 不存在", id)
	}
	return t, nil
}

// GetAllTraders 获取所有trader
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.traders {
		result[id] = t
	}
	return result
}

// GetTraderIDs 获取所有trader ID列表
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.traders))
	for id := range tm.traders {
		ids = append(ids, id)
	}
	return ids
}

// StartAll 启动所有trader
func (tm *TraderManager) StartAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	logger.Info("🚀 启动所有Trader...")
	for id, t := range tm.traders {
		go func(traderID string, at *trader.AutoTrader) {
			logger.Infof("▶️  启动 %s...", at.GetName())
			if err := at.Run(); err != nil {
				logger.Infof("❌ %s 运行错误: %v", at.GetName(), err)
			}
		}(id, t)
	}
}

// StopAll 停止所有trader
func (tm *TraderManager) StopAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	logger.Info("⏹  停止所有Trader...")
	for _, t := range tm.traders {
		t.Stop()
	}
}

// GetComparisonData 获取对比数据
func (tm *TraderManager) GetComparisonData() (map[string]interface{}, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	comparison := make(map[string]interface{})
	traders := make([]map[string]interface{}, 0, len(tm.traders))

	for _, t := range tm.traders {
		account, err := t.GetAccountInfo()
		if err != nil {
			continue
		}

		status := t.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":       t.GetID(),
			"trader_name":     t.GetName(),
			"ai_model":        t.GetAIModel(),
			"exchange":        t.GetExchange(),
			"total_equity":    account["total_equity"],
			"total_pnl":       account["total_pnl"],
			"total_pnl_pct":   account["total_pnl_pct"],
			"position_count":  account["position_count"],
			"margin_used_pct": account["margin_used_pct"],
			"call_count":      status["call_count"],
			"is_running":      status["is_running"],
		})
	}

	comparison["traders"] = traders
	comparison["count"] = len(traders)

	return comparison, nil
}

// GetCompetitionData 获取竞赛数据（全平台所有交易员）
func (tm *TraderManager) GetCompetitionData() (map[string]interface{}, error) {
	// 检查缓存是否有效（30秒内）
	tm.competitionCache.mu.RLock()
	if time.Since(tm.competitionCache.timestamp) < 30*time.Second && len(tm.competitionCache.data) > 0 {
		// 返回缓存数据
		cachedData := make(map[string]interface{})
		for k, v := range tm.competitionCache.data {
			cachedData[k] = v
		}
		tm.competitionCache.mu.RUnlock()
		logger.Infof("📋 返回竞赛数据缓存 (缓存时间: %.1fs)", time.Since(tm.competitionCache.timestamp).Seconds())
		return cachedData, nil
	}
	tm.competitionCache.mu.RUnlock()

	tm.mu.RLock()

	// 获取所有交易员列表
	allTraders := make([]*trader.AutoTrader, 0, len(tm.traders))
	for _, t := range tm.traders {
		allTraders = append(allTraders, t)
	}
	tm.mu.RUnlock()

	logger.Infof("🔄 重新获取竞赛数据，交易员数量: %d", len(allTraders))

	// 并发获取交易员数据
	traders := tm.getConcurrentTraderData(allTraders)

	// 按收益率排序（降序）
	sort.Slice(traders, func(i, j int) bool {
		pnlPctI, okI := traders[i]["total_pnl_pct"].(float64)
		pnlPctJ, okJ := traders[j]["total_pnl_pct"].(float64)
		if !okI {
			pnlPctI = 0
		}
		if !okJ {
			pnlPctJ = 0
		}
		return pnlPctI > pnlPctJ
	})

	// 限制返回前50名
	totalCount := len(traders)
	limit := 50
	if len(traders) > limit {
		traders = traders[:limit]
	}

	comparison := make(map[string]interface{})
	comparison["traders"] = traders
	comparison["count"] = len(traders)
	comparison["total_count"] = totalCount // 总交易员数量

	// 更新缓存
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = comparison
	tm.competitionCache.timestamp = time.Now()
	tm.competitionCache.mu.Unlock()

	return comparison, nil
}

// getConcurrentTraderData 并发获取多个交易员的数据
func (tm *TraderManager) getConcurrentTraderData(traders []*trader.AutoTrader) []map[string]interface{} {
	type traderResult struct {
		index int
		data  map[string]interface{}
	}

	// 创建结果通道
	resultChan := make(chan traderResult, len(traders))

	// 并发获取每个交易员的数据
	for i, t := range traders {
		go func(index int, trader *trader.AutoTrader) {
			// 设置单个交易员的超时时间为3秒
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			// 使用通道来实现超时控制
			accountChan := make(chan map[string]interface{}, 1)
			errorChan := make(chan error, 1)

			go func() {
				account, err := trader.GetAccountInfo()
				if err != nil {
					errorChan <- err
				} else {
					accountChan <- account
				}
			}()

			status := trader.GetStatus()
			var traderData map[string]interface{}

			select {
			case account := <-accountChan:
				// 成功获取账户信息
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           account["total_equity"],
					"total_pnl":              account["total_pnl"],
					"total_pnl_pct":          account["total_pnl_pct"],
					"position_count":         account["position_count"],
					"margin_used_pct":        account["margin_used_pct"],
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
				}
			case err := <-errorChan:
				// 获取账户信息失败
				logger.Infof("⚠️ 获取交易员 %s 账户信息失败: %v", trader.GetID(), err)
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           0.0,
					"total_pnl":              0.0,
					"total_pnl_pct":          0.0,
					"position_count":         0,
					"margin_used_pct":        0.0,
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
					"error":                  "账户数据获取失败",
				}
			case <-ctx.Done():
				// 超时
				logger.Infof("⏰ 获取交易员 %s 账户信息超时", trader.GetID())
				traderData = map[string]interface{}{
					"trader_id":              trader.GetID(),
					"trader_name":            trader.GetName(),
					"ai_model":               trader.GetAIModel(),
					"exchange":               trader.GetExchange(),
					"total_equity":           0.0,
					"total_pnl":              0.0,
					"total_pnl_pct":          0.0,
					"position_count":         0,
					"margin_used_pct":        0.0,
					"is_running":             status["is_running"],
					"system_prompt_template": trader.GetSystemPromptTemplate(),
					"error":                  "获取超时",
				}
			}

			resultChan <- traderResult{index: index, data: traderData}
		}(i, t)
	}

	// 收集所有结果
	results := make([]map[string]interface{}, len(traders))
	for i := 0; i < len(traders); i++ {
		result := <-resultChan
		results[result.index] = result.data
	}

	return results
}

// GetTopTradersData 获取前5名交易员数据（用于表现对比）
func (tm *TraderManager) GetTopTradersData() (map[string]interface{}, error) {
	// 复用竞赛数据缓存，因为前5名是从全部数据中筛选出来的
	competitionData, err := tm.GetCompetitionData()
	if err != nil {
		return nil, err
	}

	// 从竞赛数据中提取前5名
	allTraders, ok := competitionData["traders"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("竞赛数据格式错误")
	}

	// 限制返回前5名
	limit := 5
	topTraders := allTraders
	if len(allTraders) > limit {
		topTraders = allTraders[:limit]
	}

	result := map[string]interface{}{
		"traders": topTraders,
		"count":   len(topTraders),
	}

	return result, nil
}


// RemoveTrader 从内存中移除指定的trader（不影响数据库）
// 用于更新trader配置时强制重新加载
func (tm *TraderManager) RemoveTrader(traderID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[traderID]; exists {
		delete(tm.traders, traderID)
		logger.Infof("✓ Trader %s 已从内存中移除", traderID)
	}
}

// LoadUserTradersFromStore 为特定用户从store加载交易员到内存
func (tm *TraderManager) LoadUserTradersFromStore(st *store.Store, userID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取指定用户的所有交易员
	traders, err := st.Trader().List(userID)
	if err != nil {
		return fmt.Errorf("获取用户 %s 的交易员列表失败: %w", userID, err)
	}

	logger.Infof("📋 为用户 %s 加载交易员配置: %d 个", userID, len(traders))

	// 获取系统配置
	maxDailyLossStr, _ := st.SystemConfig().Get("max_daily_loss")
	maxDrawdownStr, _ := st.SystemConfig().Get("max_drawdown")
	stopTradingMinutesStr, _ := st.SystemConfig().Get("stop_trading_minutes")
	defaultCoinsStr, _ := st.SystemConfig().Get("default_coins")

	// 获取用户信号源配置
	var coinPoolURL, oiTopURL string
	if signalSource, err := st.SignalSource().Get(userID); err == nil {
		coinPoolURL = signalSource.CoinPoolURL
		oiTopURL = signalSource.OITopURL
		logger.Infof("📡 加载用户 %s 的信号源配置: COIN POOL=%s, OI TOP=%s", userID, coinPoolURL, oiTopURL)
	} else {
		logger.Infof("🔍 用户 %s 暂未配置信号源", userID)
	}

	// 解析配置
	maxDailyLoss := 10.0 // 默认值
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // 默认值
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // 默认值
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// 解析默认币种列表
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			logger.Infof("⚠️ 解析默认币种配置失败: %v，使用空列表", err)
			defaultCoins = []string{}
		}
	}

	// 获取AI模型和交易所列表（在循环外只查询一次）
	aiModels, err := st.AIModel().List(userID)
	if err != nil {
		logger.Infof("⚠️ 获取用户 %s 的AI模型配置失败: %v", userID, err)
		return fmt.Errorf("获取AI模型配置失败: %w", err)
	}

	exchanges, err := st.Exchange().List(userID)
	if err != nil {
		logger.Infof("⚠️ 获取用户 %s 的交易所配置失败: %v", userID, err)
		return fmt.Errorf("获取交易所配置失败: %w", err)
	}

	// 为每个交易员加载配置
	for _, traderCfg := range traders {
		// 检查是否已经加载过这个交易员
		if _, exists := tm.traders[traderCfg.ID]; exists {
			logger.Infof("⚠️ 交易员 %s 已经加载，跳过", traderCfg.Name)
			continue
		}

		// 从已查询的列表中查找AI模型配置
		var aiModelCfg *store.AIModel
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					break
				}
			}
		}

		if aiModelCfg == nil {
			logger.Infof("⚠️ 交易员 %s 的AI模型 %s 不存在，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			logger.Infof("⚠️ 交易员 %s 的AI模型 %s 未启用，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// 从已查询的列表中查找交易所配置
		var exchangeCfg *store.Exchange
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			logger.Infof("⚠️ 交易员 %s 的交易所 %s 不存在，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			logger.Infof("⚠️ 交易员 %s 的交易所 %s 未启用，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// 使用现有的方法加载交易员
		err = tm.addTraderFromStore(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, st)
		if err != nil {
			logger.Infof("⚠️ 加载交易员 %s 失败: %v", traderCfg.Name, err)
		}
	}

	return nil
}

// LoadTradersFromStore 从store加载所有交易员到内存（新版API）
func (tm *TraderManager) LoadTradersFromStore(st *store.Store) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取所有用户
	userIDs, err := st.User().GetAllIDs()
	if err != nil {
		return fmt.Errorf("获取用户列表失败: %w", err)
	}

	logger.Infof("📋 发现 %d 个用户，开始加载所有交易员配置...", len(userIDs))

	var allTraders []*store.Trader
	for _, userID := range userIDs {
		// 获取每个用户的交易员
		traders, err := st.Trader().List(userID)
		if err != nil {
			logger.Infof("⚠️ 获取用户 %s 的交易员失败: %v", userID, err)
			continue
		}
		logger.Infof("📋 用户 %s: %d 个交易员", userID, len(traders))
		allTraders = append(allTraders, traders...)
	}

	logger.Infof("📋 总共加载 %d 个交易员配置", len(allTraders))

	// 获取系统配置
	maxDailyLossStr, _ := st.SystemConfig().Get("max_daily_loss")
	maxDrawdownStr, _ := st.SystemConfig().Get("max_drawdown")
	stopTradingMinutesStr, _ := st.SystemConfig().Get("stop_trading_minutes")
	defaultCoinsStr, _ := st.SystemConfig().Get("default_coins")

	// 解析配置
	maxDailyLoss := 10.0 // 默认值
	if val, err := strconv.ParseFloat(maxDailyLossStr, 64); err == nil {
		maxDailyLoss = val
	}

	maxDrawdown := 20.0 // 默认值
	if val, err := strconv.ParseFloat(maxDrawdownStr, 64); err == nil {
		maxDrawdown = val
	}

	stopTradingMinutes := 60 // 默认值
	if val, err := strconv.Atoi(stopTradingMinutesStr); err == nil {
		stopTradingMinutes = val
	}

	// 解析默认币种列表
	var defaultCoins []string
	if defaultCoinsStr != "" {
		if err := json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins); err != nil {
			logger.Infof("⚠️ 解析默认币种配置失败: %v，使用空列表", err)
			defaultCoins = []string{}
		}
	}

	// 为每个交易员获取AI模型和交易所配置
	for _, traderCfg := range allTraders {
		// 获取AI模型配置
		aiModels, err := st.AIModel().List(traderCfg.UserID)
		if err != nil {
			logger.Infof("⚠️  获取AI模型配置失败: %v", err)
			continue
		}

		var aiModelCfg *store.AIModel
		// 优先精确匹配 model.ID
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}
		// 如果没有精确匹配，尝试匹配 provider（兼容旧数据）
		if aiModelCfg == nil {
			for _, model := range aiModels {
				if model.Provider == traderCfg.AIModelID {
					aiModelCfg = model
					logger.Infof("⚠️  交易员 %s 使用旧版 provider 匹配: %s -> %s", traderCfg.Name, traderCfg.AIModelID, model.ID)
					break
				}
			}
		}

		if aiModelCfg == nil {
			logger.Infof("⚠️  交易员 %s 的AI模型 %s 不存在，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			logger.Infof("⚠️  交易员 %s 的AI模型 %s 未启用，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// 获取交易所配置
		exchanges, err := st.Exchange().List(traderCfg.UserID)
		if err != nil {
			logger.Infof("⚠️  获取交易所配置失败: %v", err)
			continue
		}

		var exchangeCfg *store.Exchange
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			logger.Infof("⚠️  交易员 %s 的交易所 %s 不存在，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			logger.Infof("⚠️  交易员 %s 的交易所 %s 未启用，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// 获取用户信号源配置
		var coinPoolURL, oiTopURL string
		if signalSource, err := st.SignalSource().Get(traderCfg.UserID); err == nil {
			coinPoolURL = signalSource.CoinPoolURL
			oiTopURL = signalSource.OITopURL
		} else {
			logger.Infof("🔍 用户 %s 暂未配置信号源", traderCfg.UserID)
		}

		// 添加到TraderManager
		err = tm.addTraderFromStore(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, oiTopURL, maxDailyLoss, maxDrawdown, stopTradingMinutes, defaultCoins, st)
		if err != nil {
			logger.Infof("❌ 添加交易员 %s 失败: %v", traderCfg.Name, err)
			continue
		}
	}

	logger.Infof("✓ 成功加载 %d 个交易员到内存", len(tm.traders))
	return nil
}

// addTraderFromStore 内部方法：从store配置添加交易员
func (tm *TraderManager) addTraderFromStore(traderCfg *store.Trader, aiModelCfg *store.AIModel, exchangeCfg *store.Exchange, coinPoolURL, oiTopURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int, defaultCoins []string, st *store.Store) error {
	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", traderCfg.ID)
	}

	// 处理交易币种列表
	var tradingCoins []string
	if traderCfg.TradingSymbols != "" {
		symbols := strings.Split(traderCfg.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" {
				tradingCoins = append(tradingCoins, symbol)
			}
		}
	}

	// 如果没有指定交易币种，使用默认币种
	if len(tradingCoins) == 0 {
		tradingCoins = defaultCoins
	}

	// 根据交易员配置决定是否使用信号源
	var effectiveCoinPoolURL string
	if traderCfg.UseCoinPool && coinPoolURL != "" {
		effectiveCoinPoolURL = coinPoolURL
		logger.Infof("✓ 交易员 %s 启用 COIN POOL 信号源: %s", traderCfg.Name, coinPoolURL)
	}

	// 构建AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider,
		Exchange:              exchangeCfg.ID,
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        effectiveCoinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		CustomAPIURL:          aiModelCfg.CustomAPIURL,
		CustomModelName:       aiModelCfg.CustomModelName,
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		BTCETHLeverage:        traderCfg.BTCETHLeverage,
		AltcoinLeverage:       traderCfg.AltcoinLeverage,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
		IsCrossMargin:         traderCfg.IsCrossMargin,
		DefaultCoins:          defaultCoins,
		TradingCoins:          tradingCoins,
		SystemPromptTemplate:  traderCfg.SystemPromptTemplate,
	}

	// 根据交易所类型设置API密钥
	switch exchangeCfg.ID {
	case "binance":
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	case "bybit":
		traderConfig.BybitAPIKey = exchangeCfg.APIKey
		traderConfig.BybitSecretKey = exchangeCfg.SecretKey
	case "hyperliquid":
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey
		traderConfig.HyperliquidWalletAddr = exchangeCfg.HyperliquidWalletAddr
	case "aster":
		traderConfig.AsterUser = exchangeCfg.AsterUser
		traderConfig.AsterSigner = exchangeCfg.AsterSigner
		traderConfig.AsterPrivateKey = exchangeCfg.AsterPrivateKey
	case "lighter":
		traderConfig.LighterPrivateKey = exchangeCfg.LighterPrivateKey
		traderConfig.LighterWalletAddr = exchangeCfg.LighterWalletAddr
		traderConfig.LighterTestnet = exchangeCfg.Testnet
	}

	// 根据AI模型设置API密钥
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// 创建trader实例
	at, err := trader.NewAutoTrader(traderConfig, st, traderCfg.UserID)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	// 设置自定义prompt（如果有）
	if traderCfg.CustomPrompt != "" {
		at.SetCustomPrompt(traderCfg.CustomPrompt)
		at.SetOverrideBasePrompt(traderCfg.OverrideBasePrompt)
		if traderCfg.OverrideBasePrompt {
			logger.Infof("✓ 已设置自定义交易策略prompt (覆盖基础prompt)")
		} else {
			logger.Infof("✓ 已设置自定义交易策略prompt (补充基础prompt)")
		}
	}

	tm.traders[traderCfg.ID] = at
	logger.Infof("✓ Trader '%s' (%s + %s) 已加载到内存", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}
