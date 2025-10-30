package manager

import (
	"fmt"
	"log"
	"nofx/config"
	"nofx/trader"
	"strconv"
	"sync"
	"time"
)

// TraderManager 管理多个trader实例
type TraderManager struct {
	traders map[string]*trader.AutoTrader // key: trader ID
	mu      sync.RWMutex
}

// NewTraderManager 创建trader管理器
func NewTraderManager() *TraderManager {
	return &TraderManager{
		traders: make(map[string]*trader.AutoTrader),
	}
}

// LoadTradersFromDatabase 从数据库加载所有交易员到内存
func (tm *TraderManager) LoadTradersFromDatabase(database *config.Database) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 获取数据库中的所有交易员
	traders, err := database.GetTraders()
	if err != nil {
		return fmt.Errorf("获取交易员列表失败: %w", err)
	}

	log.Printf("📋 加载数据库中的交易员配置: %d 个", len(traders))

	// 获取系统配置
	coinPoolURL, _ := database.GetSystemConfig("coin_pool_api_url")
	maxDailyLossStr, _ := database.GetSystemConfig("max_daily_loss")
	maxDrawdownStr, _ := database.GetSystemConfig("max_drawdown")
	stopTradingMinutesStr, _ := database.GetSystemConfig("stop_trading_minutes")

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

	// 为每个交易员获取AI模型和交易所配置
	for _, traderCfg := range traders {
		// 获取AI模型配置
		aiModels, err := database.GetAIModels()
		if err != nil {
			log.Printf("⚠️  获取AI模型配置失败: %v", err)
			continue
		}

		var aiModelCfg *config.AIModelConfig
		for _, model := range aiModels {
			if model.ID == traderCfg.AIModelID {
				aiModelCfg = model
				break
			}
		}

		if aiModelCfg == nil {
			log.Printf("⚠️  交易员 %s 的AI模型 %s 不存在，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		if !aiModelCfg.Enabled {
			log.Printf("⚠️  交易员 %s 的AI模型 %s 未启用，跳过", traderCfg.Name, traderCfg.AIModelID)
			continue
		}

		// 获取交易所配置
		exchanges, err := database.GetExchanges()
		if err != nil {
			log.Printf("⚠️  获取交易所配置失败: %v", err)
			continue
		}

		var exchangeCfg *config.ExchangeConfig
		for _, exchange := range exchanges {
			if exchange.ID == traderCfg.ExchangeID {
				exchangeCfg = exchange
				break
			}
		}

		if exchangeCfg == nil {
			log.Printf("⚠️  交易员 %s 的交易所 %s 不存在，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		if !exchangeCfg.Enabled {
			log.Printf("⚠️  交易员 %s 的交易所 %s 未启用，跳过", traderCfg.Name, traderCfg.ExchangeID)
			continue
		}

		// 添加到TraderManager
		err = tm.addTraderFromConfig(traderCfg, aiModelCfg, exchangeCfg, coinPoolURL, maxDailyLoss, maxDrawdown, stopTradingMinutes)
		if err != nil {
			log.Printf("❌ 添加交易员 %s 失败: %v", traderCfg.Name, err)
			continue
		}
	}

	log.Printf("✓ 成功加载 %d 个交易员到内存", len(tm.traders))
	return nil
}

// addTraderFromConfig 内部方法：从配置添加交易员（不加锁，因为调用方已加锁）
func (tm *TraderManager) addTraderFromConfig(traderCfg *config.TraderConfig, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int) error {
	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", traderCfg.ID)
	}

	// 构建AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider, // 使用provider作为模型标识
		Exchange:              exchangeCfg.ID,      // 使用exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        coinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
	}

	// 根据交易所类型设置API密钥
	if exchangeCfg.ID == "binance" {
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid用APIKey存储private key
	}

	// 根据AI模型设置API密钥
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// 创建trader实例
	at, err := trader.NewAutoTrader(traderConfig)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) 已加载到内存", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
}

// AddTrader 从数据库配置添加trader (移除旧版兼容性)

// AddTraderFromDB 从数据库配置添加trader
func (tm *TraderManager) AddTraderFromDB(traderCfg *config.TraderConfig, aiModelCfg *config.AIModelConfig, exchangeCfg *config.ExchangeConfig, coinPoolURL string, maxDailyLoss, maxDrawdown float64, stopTradingMinutes int) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.traders[traderCfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", traderCfg.ID)
	}

	// 构建AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    traderCfg.ID,
		Name:                  traderCfg.Name,
		AIModel:               aiModelCfg.Provider, // 使用provider作为模型标识
		Exchange:              exchangeCfg.ID,      // 使用exchange ID
		BinanceAPIKey:         "",
		BinanceSecretKey:      "",
		HyperliquidPrivateKey: "",
		HyperliquidTestnet:    exchangeCfg.Testnet,
		CoinPoolAPIURL:        coinPoolURL,
		UseQwen:               aiModelCfg.Provider == "qwen",
		DeepSeekKey:           "",
		QwenKey:               "",
		ScanInterval:          time.Duration(traderCfg.ScanIntervalMinutes) * time.Minute,
		InitialBalance:        traderCfg.InitialBalance,
		MaxDailyLoss:          maxDailyLoss,
		MaxDrawdown:           maxDrawdown,
		StopTradingTime:       time.Duration(stopTradingMinutes) * time.Minute,
	}

	// 根据交易所类型设置API密钥
	if exchangeCfg.ID == "binance" {
		traderConfig.BinanceAPIKey = exchangeCfg.APIKey
		traderConfig.BinanceSecretKey = exchangeCfg.SecretKey
	} else if exchangeCfg.ID == "hyperliquid" {
		traderConfig.HyperliquidPrivateKey = exchangeCfg.APIKey // hyperliquid用APIKey存储private key
	}

	// 根据AI模型设置API密钥
	if aiModelCfg.Provider == "qwen" {
		traderConfig.QwenKey = aiModelCfg.APIKey
	} else if aiModelCfg.Provider == "deepseek" {
		traderConfig.DeepSeekKey = aiModelCfg.APIKey
	}

	// 创建trader实例
	at, err := trader.NewAutoTrader(traderConfig)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	tm.traders[traderCfg.ID] = at
	log.Printf("✓ Trader '%s' (%s + %s) 已添加", traderCfg.Name, aiModelCfg.Provider, exchangeCfg.ID)
	return nil
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

	log.Println("🚀 启动所有Trader...")
	for id, t := range tm.traders {
		go func(traderID string, at *trader.AutoTrader) {
			log.Printf("▶️  启动 %s...", at.GetName())
			if err := at.Run(); err != nil {
				log.Printf("❌ %s 运行错误: %v", at.GetName(), err)
			}
		}(id, t)
	}
}

// StopAll 停止所有trader
func (tm *TraderManager) StopAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("⏹  停止所有Trader...")
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
