package trader

import (
	"fmt"
	"nofx/logger"
	"nofx/store"
	"sync"
	"time"
)

// PositionSyncManager 仓位状态同步管理器
// 负责定期同步交易所仓位，检测手动平仓等变化
type PositionSyncManager struct {
	store        *store.Store
	interval     time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	traderCache  map[string]Trader                    // trader_id -> Trader 实例缓存
	configCache  map[string]*store.TraderFullConfig   // trader_id -> 配置缓存
	cacheMutex   sync.RWMutex
}

// NewPositionSyncManager 创建仓位同步管理器
func NewPositionSyncManager(st *store.Store, interval time.Duration) *PositionSyncManager {
	if interval == 0 {
		interval = 10 * time.Second
	}
	return &PositionSyncManager{
		store:       st,
		interval:    interval,
		stopCh:      make(chan struct{}),
		traderCache: make(map[string]Trader),
		configCache: make(map[string]*store.TraderFullConfig),
	}
}

// Start 启动仓位同步服务
func (m *PositionSyncManager) Start() {
	m.wg.Add(1)
	go m.run()
	logger.Info("📊 仓位同步管理器已启动")
}

// Stop 停止仓位同步服务
func (m *PositionSyncManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	// 清理缓存
	m.cacheMutex.Lock()
	m.traderCache = make(map[string]Trader)
	m.configCache = make(map[string]*store.TraderFullConfig)
	m.cacheMutex.Unlock()

	logger.Info("📊 仓位同步管理器已停止")
}

// run 主循环
func (m *PositionSyncManager) run() {
	defer m.wg.Done()

	// 启动时立即执行一次
	m.syncPositions()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.syncPositions()
		}
	}
}

// syncPositions 同步所有仓位状态
func (m *PositionSyncManager) syncPositions() {
	// 获取所有 OPEN 状态的仓位
	localPositions, err := m.store.Position().GetAllOpenPositions()
	if err != nil {
		logger.Infof("⚠️  获取本地仓位失败: %v", err)
		return
	}

	if len(localPositions) == 0 {
		return
	}

	// 按 trader_id 分组
	positionsByTrader := make(map[string][]*store.TraderPosition)
	for _, pos := range localPositions {
		positionsByTrader[pos.TraderID] = append(positionsByTrader[pos.TraderID], pos)
	}

	// 逐个 trader 处理
	for traderID, traderPositions := range positionsByTrader {
		m.syncTraderPositions(traderID, traderPositions)
	}
}

// syncTraderPositions 同步单个 trader 的仓位
func (m *PositionSyncManager) syncTraderPositions(traderID string, localPositions []*store.TraderPosition) {
	// 获取或创建 trader 实例
	trader, err := m.getOrCreateTrader(traderID)
	if err != nil {
		logger.Infof("⚠️  获取 trader 实例失败 (ID: %s): %v", traderID, err)
		return
	}

	// 获取交易所当前仓位
	exchangePositions, err := trader.GetPositions()
	if err != nil {
		logger.Infof("⚠️  获取交易所仓位失败 (ID: %s): %v", traderID, err)
		return
	}

	// 构建交易所仓位 map: symbol_side -> position
	exchangeMap := make(map[string]map[string]interface{})
	for _, pos := range exchangePositions {
		symbol, _ := pos["symbol"].(string)
		side, _ := pos["positionSide"].(string)
		if symbol == "" || side == "" {
			continue
		}
		key := fmt.Sprintf("%s_%s", symbol, side)
		exchangeMap[key] = pos
	}

	// 对比本地和交易所仓位
	for _, localPos := range localPositions {
		key := fmt.Sprintf("%s_%s", localPos.Symbol, localPos.Side)
		exchangePos, exists := exchangeMap[key]

		if !exists {
			// 交易所没有这个仓位了 → 已被平仓
			m.closeLocalPosition(localPos, trader, "manual")
			continue
		}

		// 检查数量是否为0或很小
		qty := getFloatFromMap(exchangePos, "positionAmt")
		if qty < 0 {
			qty = -qty // 空仓数量是负的
		}

		if qty < 0.0000001 {
			// 数量为0，仓位已平
			m.closeLocalPosition(localPos, trader, "manual")
		}
	}
}

// closeLocalPosition 标记本地仓位为已平仓
func (m *PositionSyncManager) closeLocalPosition(pos *store.TraderPosition, trader Trader, reason string) {
	// 尝试获取最后成交价作为平仓价
	exitPrice := pos.EntryPrice // 默认用开仓价

	// 尝试从交易所获取最新价格
	if price, err := trader.GetMarketPrice(pos.Symbol); err == nil && price > 0 {
		exitPrice = price
	}

	// 计算盈亏
	var realizedPnL float64
	if pos.Side == "LONG" {
		realizedPnL = (exitPrice - pos.EntryPrice) * pos.Quantity
	} else {
		realizedPnL = (pos.EntryPrice - exitPrice) * pos.Quantity
	}

	// 更新数据库
	err := m.store.Position().ClosePosition(
		pos.ID,
		exitPrice,
		"", // 手动平仓没有订单ID
		realizedPnL,
		0,      // 手动平仓无法获取手续费
		reason,
	)

	if err != nil {
		logger.Infof("⚠️  更新仓位状态失败: %v", err)
	} else {
		logger.Infof("📊 仓位已平仓 [%s] %s %s @ %.4f → %.4f, PnL: %.2f (%s)",
			pos.TraderID[:8], pos.Symbol, pos.Side, pos.EntryPrice, exitPrice, realizedPnL, reason)
	}
}

// getOrCreateTrader 获取或创建 trader 实例
func (m *PositionSyncManager) getOrCreateTrader(traderID string) (Trader, error) {
	m.cacheMutex.RLock()
	trader, exists := m.traderCache[traderID]
	m.cacheMutex.RUnlock()

	if exists && trader != nil {
		return trader, nil
	}

	// 需要创建新的 trader 实例
	config, err := m.getTraderConfig(traderID)
	if err != nil {
		return nil, fmt.Errorf("获取 trader 配置失败: %w", err)
	}

	trader, err = m.createTrader(config)
	if err != nil {
		return nil, fmt.Errorf("创建 trader 实例失败: %w", err)
	}

	m.cacheMutex.Lock()
	m.traderCache[traderID] = trader
	m.cacheMutex.Unlock()

	return trader, nil
}

// getTraderConfig 获取 trader 配置
func (m *PositionSyncManager) getTraderConfig(traderID string) (*store.TraderFullConfig, error) {
	m.cacheMutex.RLock()
	config, exists := m.configCache[traderID]
	m.cacheMutex.RUnlock()

	if exists {
		return config, nil
	}

	// 从数据库获取
	traders, err := m.store.Trader().ListAll()
	if err != nil {
		return nil, fmt.Errorf("获取 trader 列表失败: %w", err)
	}

	var userID string
	for _, t := range traders {
		if t.ID == traderID {
			userID = t.UserID
			break
		}
	}

	if userID == "" {
		return nil, fmt.Errorf("找不到 trader: %s", traderID)
	}

	config, err = m.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		return nil, err
	}

	m.cacheMutex.Lock()
	m.configCache[traderID] = config
	m.cacheMutex.Unlock()

	return config, nil
}

// createTrader 根据配置创建 trader 实例
func (m *PositionSyncManager) createTrader(config *store.TraderFullConfig) (Trader, error) {
	exchange := config.Exchange

	// 使用 exchange.ID 判断具体的交易所，而不是 exchange.Type (cex/dex)
	switch exchange.ID {
	case "binance":
		return NewFuturesTrader(exchange.APIKey, exchange.SecretKey, config.Trader.UserID), nil

	case "bybit":
		return NewBybitTrader(exchange.APIKey, exchange.SecretKey), nil

	case "hyperliquid":
		return NewHyperliquidTrader(exchange.SecretKey, exchange.HyperliquidWalletAddr, exchange.Testnet)

	case "aster":
		return NewAsterTrader(exchange.AsterUser, exchange.AsterSigner, exchange.AsterPrivateKey)

	case "lighter":
		if exchange.LighterAPIKeyPrivateKey != "" {
			return NewLighterTraderV2(
				exchange.LighterPrivateKey,
				exchange.LighterWalletAddr,
				exchange.LighterAPIKeyPrivateKey,
				exchange.Testnet,
			)
		}
		return NewLighterTrader(exchange.LighterPrivateKey, exchange.LighterWalletAddr, exchange.Testnet)

	default:
		return nil, fmt.Errorf("不支持的交易所: %s", exchange.ID)
	}
}

// InvalidateCache 使缓存失效
func (m *PositionSyncManager) InvalidateCache(traderID string) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	delete(m.traderCache, traderID)
	delete(m.configCache, traderID)
}

// getFloatFromMap 从 map 中获取 float64 值
func getFloatFromMap(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int64:
			return float64(val)
		case int:
			return float64(val)
		case string:
			var f float64
			fmt.Sscanf(val, "%f", &f)
			return f
		}
	}
	return 0
}
