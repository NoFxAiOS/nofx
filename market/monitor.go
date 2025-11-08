package market

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	// MaxStreamsPerConnection Binance WebSocket 單連接最大訂閱流數限制
	MaxStreamsPerConnection = 1024
	// SafeMaxSymbols 安全的最大幣種數量（留 2.3% 緩衝空間）
	// 250 個幣種 × 4 時間週期 = 1000 流 < 1024
	SafeMaxSymbols = 250
)

type WSMonitor struct {
	wsClient        *WSClient
	combinedClient  *CombinedStreamsClient
	symbols         []string
	featuresMap     sync.Map
	alertsChan      chan Alert
	klineDataMap3m  sync.Map      // 存储每个交易对的K线历史数据
	klineDataMap15m sync.Map      // 存储每个交易对的15分钟K线历史数据
	klineDataMap1h  sync.Map      // 存储每个交易对的1小时K线历史数据
	klineDataMap4h  sync.Map      // 存储每个交易对的K线历史数据
	tickerDataMap   sync.Map      // 存储每个交易对的ticker数据
	oiHistoryMap    sync.Map      // P0修复：存储OI历史数据 map[symbol][]OISnapshot
	oiStopChan      chan struct{} // P0修复：OI监控停止信号通道
	batchSize       int
	filterSymbols   sync.Map // 使用sync.Map来存储需要监控的币种和其状态
	symbolStats     sync.Map // 存储币种统计信息
	FilterSymbol    []string //经过筛选的币种
}
type SymbolStats struct {
	LastActiveTime   time.Time
	AlertCount       int
	VolumeSpikeCount int
	LastAlertTime    time.Time
	Score            float64 // 综合评分
}

var WSMonitorCli *WSMonitor
var subKlineTime = []string{"3m", "15m", "1h", "4h"} // 管理订阅流的K线周期

func NewWSMonitor(batchSize int) *WSMonitor {
	WSMonitorCli = &WSMonitor{
		wsClient:       NewWSClient(),
		combinedClient: NewCombinedStreamsClient(batchSize),
		alertsChan:     make(chan Alert, 1000),
		batchSize:      batchSize,
	}
	return WSMonitorCli
}

func (m *WSMonitor) Initialize(coins []string) error {
	log.Println("初始化WebSocket监控器...")
	// 获取交易对信息
	apiClient := NewAPIClient()
	// 如果不指定交易对，则使用market市场的所有交易对币种
	if len(coins) == 0 {
		exchangeInfo, err := apiClient.GetExchangeInfo()
		if err != nil {
			return err
		}
		// 筛选永续合约交易对 --仅测试时使用
		//exchangeInfo.Symbols = exchangeInfo.Symbols[0:2]
		for _, symbol := range exchangeInfo.Symbols {
			if symbol.Status == "TRADING" && symbol.ContractType == "PERPETUAL" && strings.ToUpper(symbol.Symbol[len(symbol.Symbol)-4:]) == "USDT" {
				m.symbols = append(m.symbols, symbol.Symbol)
				m.filterSymbols.Store(symbol.Symbol, true)
			}
		}
	} else {
		m.symbols = coins
	}

	log.Printf("找到 %d 个交易对", len(m.symbols))

	// WebSocket 訂閱流數檢查與自動調整
	totalStreams := len(m.symbols) * len(subKlineTime)

	if len(m.symbols) > SafeMaxSymbols {
		log.Printf("⚠️  幣種數量過多，自動調整:")
		log.Printf("   - 原始數量: %d 個幣種 (%d 流)", len(m.symbols), totalStreams)
		log.Printf("   - Binance 限制: %d 流/連接", MaxStreamsPerConnection)
		log.Printf("   - 時間週期: %d (%v)", len(subKlineTime), subKlineTime)

		// 調整到安全上限
		m.symbols = m.symbols[:SafeMaxSymbols]
		totalStreams = len(m.symbols) * len(subKlineTime)

		log.Printf("   - 調整後: %d 個幣種 (%d 流)", len(m.symbols), totalStreams)
		log.Printf("   - 已過濾: 前 %d 個幣種保留，其餘忽略", SafeMaxSymbols)
	}

	// 顯示訂閱使用率
	usagePercent := float64(totalStreams) / float64(MaxStreamsPerConnection) * 100
	log.Printf("✓ WebSocket 訂閱: %d 個幣種 × %d 時間週期 = %d 流 (%.1f%% 用量)",
		len(m.symbols), len(subKlineTime), totalStreams, usagePercent)

	// 接近上限警告（>90%）
	if usagePercent > 90 {
		log.Printf("⚠️  警告: 訂閱流使用率較高 (%.1f%%)，建議減少幣種數量以確保穩定性", usagePercent)
	}

	// 初始化历史数据
	if err := m.initializeHistoricalData(); err != nil {
		log.Printf("初始化历史数据失败: %v", err)
	}

	return nil
}

func (m *WSMonitor) initializeHistoricalData() error {
	apiClient := NewAPIClient()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // 限制并发数

	for _, symbol := range m.symbols {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(s string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// 获取3分钟历史K线数据
			klines3m, err := apiClient.GetKlines(s, "3m", 100)
			if err != nil {
				log.Printf("获取 %s 3m历史数据失败: %v", s, err)
			} else if len(klines3m) > 0 {
				m.klineDataMap3m.Store(s, klines3m)
				log.Printf("已加载 %s 的历史K线数据-3m: %d 条", s, len(klines3m))
			}

			// 获取15分钟历史K线数据
			klines15m, err := apiClient.GetKlines(s, "15m", 100)
			if err != nil {
				log.Printf("获取 %s 15m历史数据失败: %v", s, err)
			} else if len(klines15m) > 0 {
				m.klineDataMap15m.Store(s, klines15m)
				log.Printf("已加载 %s 的历史K线数据-15m: %d 条", s, len(klines15m))
			}

			// 获取1小时历史K线数据
			klines1h, err := apiClient.GetKlines(s, "1h", 100)
			if err != nil {
				log.Printf("获取 %s 1h历史数据失败: %v", s, err)
			} else if len(klines1h) > 0 {
				m.klineDataMap1h.Store(s, klines1h)
				log.Printf("已加载 %s 的历史K线数据-1h: %d 条", s, len(klines1h))
			}

			// 获取4小时历史K线数据（P0修复：添加重试机制）
			var klines4h []Kline
			for retry := 0; retry < 3; retry++ {
				klines4h, err = apiClient.GetKlines(s, "4h", 100)
				if err == nil && len(klines4h) > 0 {
					break
				}
				if retry < 2 {
					log.Printf("获取 %s 4h历史数据失败 (尝试 %d/3): %v，1秒后重试...", s, retry+1, err)
					time.Sleep(1 * time.Second)
				}
			}
			if err != nil {
				log.Printf("❌ 获取 %s 4h历史数据失败（已重试3次）: %v", s, err)
			} else if len(klines4h) > 0 {
				m.klineDataMap4h.Store(s, klines4h)
				log.Printf("✅ 已加载 %s 的历史K线数据-4h: %d 条", s, len(klines4h))
			} else {
				log.Printf("⚠️  WARNING: %s 4h数据为空（API返回成功但无数据）", s)
			}

			// 🚀 优化：回填历史OI数据（15分钟粒度，最近20个数据点 = 5小时）
			oiHistory, err := apiClient.GetOpenInterestHistory(s, "15m", 20)
			normalizedSymbol := strings.ToUpper(s)

			if err != nil || len(oiHistory) == 0 {
				// ✅ 修复：无论是API错误还是返回空数组，都尝试降级方案
				if err != nil {
					log.Printf("⚠️  获取 %s OI历史数据失败: %v，尝试降级方案...", s, err)
				} else {
					log.Printf("⚠️  %s OI历史数据为空（API返回成功但无数据），尝试降级方案...", normalizedSymbol)
				}

				// ✅ 修复：降级方案 - 至少获取当前OI作为第一个数据点
				currentOI, currentErr := apiClient.GetOpenInterest(s)
				if currentErr != nil {
					log.Printf("❌ 获取 %s 当前OI也失败: %v，该币种将无OI数据", s, currentErr)
				} else {
					// 创建单个数据点作为起始
					oiHistory = []OISnapshot{{Value: currentOI.Latest, Timestamp: time.Now()}}
					m.oiHistoryMap.Store(normalizedSymbol, oiHistory)
					log.Printf("✅ %s 使用降级方案：仅1个OI数据点（%.0f），将在15分钟后开始累积历史数据", normalizedSymbol, currentOI.Latest)
				}
			} else {
				// ✅ 成功获取历史数据
				m.oiHistoryMap.Store(normalizedSymbol, oiHistory)

				// 🔍 診斷：顯示時間範圍
				oldest := oiHistory[0].Timestamp
				newest := oiHistory[len(oiHistory)-1].Timestamp
				timeSpan := newest.Sub(oldest)
				log.Printf("✅ 已回填 %s 的历史OI数据: %d 个快照（时间范围: %s ~ %s，跨度 %.1f 小时）",
					normalizedSymbol, len(oiHistory), oldest.Format("15:04"), newest.Format("15:04"), timeSpan.Hours())
			}
		}(symbol)
	}

	wg.Wait()
	return nil
}

func (m *WSMonitor) Start(coins []string) {
	log.Printf("启动WebSocket实时监控...")
	// 初始化交易对
	err := m.Initialize(coins)
	if err != nil {
		log.Printf("❌ 初始化币种失败: %v", err)
		return
	}

	err = m.combinedClient.Connect()
	if err != nil {
		log.Printf("❌ 批量订阅流失败: %v", err)
		return
	}
	// 订阅所有交易对
	err = m.subscribeAll()
	if err != nil {
		log.Printf("❌ 订阅币种交易对失败: %v", err)
		return
	}

	// P0修复：启动OI定期监控（每15分钟采样，用于计算4小时变化率）
	m.StartOIMonitoring()
}

// subscribeSymbol 注册监听
func (m *WSMonitor) subscribeSymbol(symbol, st string) []string {
	var streams []string
	stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), st)
	ch := m.combinedClient.AddSubscriber(stream, 100)
	streams = append(streams, stream)
	go m.handleKlineData(symbol, ch, st)

	return streams
}
func (m *WSMonitor) subscribeAll() error {
	log.Println("开始订阅所有交易对...")

	for _, symbol := range m.symbols {
		for _, st := range subKlineTime {
			m.subscribeSymbol(symbol, st)
		}
	}

	// 执行批量订阅
	for _, st := range subKlineTime {
		err := m.combinedClient.BatchSubscribeKlines(m.symbols, st)
		if err != nil {
			log.Printf("❌ 订阅 %s K线失败: %v", st, err)
			return err
		}
	}
	log.Println("所有交易对订阅完成")
	return nil
}

func (m *WSMonitor) handleKlineData(symbol string, ch <-chan []byte, _time string) {
	for data := range ch {
		var klineData KlineWSData
		if err := json.Unmarshal(data, &klineData); err != nil {
			log.Printf("解析Kline数据失败: %v", err)
			continue
		}
		m.processKlineUpdate(symbol, klineData, _time)
	}
}

func (m *WSMonitor) getKlineDataMap(_time string) *sync.Map {
	var klineDataMap *sync.Map
	switch _time {
	case "3m":
		klineDataMap = &m.klineDataMap3m
	case "15m":
		klineDataMap = &m.klineDataMap15m
	case "1h":
		klineDataMap = &m.klineDataMap1h
	case "4h":
		klineDataMap = &m.klineDataMap4h
	default:
		klineDataMap = &sync.Map{}
	}
	return klineDataMap
}
func (m *WSMonitor) processKlineUpdate(symbol string, wsData KlineWSData, _time string) {
	// 转换WebSocket数据为Kline结构
	kline := Kline{
		OpenTime:  wsData.Kline.StartTime,
		CloseTime: wsData.Kline.CloseTime,
		Trades:    wsData.Kline.NumberOfTrades,
	}
	kline.Open, _ = parseFloat(wsData.Kline.OpenPrice)
	kline.High, _ = parseFloat(wsData.Kline.HighPrice)
	kline.Low, _ = parseFloat(wsData.Kline.LowPrice)
	kline.Close, _ = parseFloat(wsData.Kline.ClosePrice)
	kline.Volume, _ = parseFloat(wsData.Kline.Volume)
	kline.High, _ = parseFloat(wsData.Kline.HighPrice)
	kline.QuoteVolume, _ = parseFloat(wsData.Kline.QuoteVolume)
	kline.TakerBuyBaseVolume, _ = parseFloat(wsData.Kline.TakerBuyBaseVolume)
	kline.TakerBuyQuoteVolume, _ = parseFloat(wsData.Kline.TakerBuyQuoteVolume)
	// 更新K线数据
	var klineDataMap = m.getKlineDataMap(_time)
	value, exists := klineDataMap.Load(symbol)
	var klines []Kline
	if exists {
		klines = value.([]Kline)

		// 检查是否是新的K线
		if len(klines) > 0 && klines[len(klines)-1].OpenTime == kline.OpenTime {
			// 更新当前K线
			klines[len(klines)-1] = kline
		} else {
			// 添加新K线
			klines = append(klines, kline)

			// 保持数据长度
			if len(klines) > 100 {
				klines = klines[1:]
			}
		}
	} else {
		klines = []Kline{kline}
	}

	klineDataMap.Store(symbol, klines)
}

func (m *WSMonitor) GetCurrentKlines(symbol string, _time string) ([]Kline, error) {
	// 对每一个进来的symbol检测是否存在内类 是否的话就订阅它
	value, exists := m.getKlineDataMap(_time).Load(symbol)
	if !exists {
		// 如果Ws数据未初始化完成时,单独使用api获取 - 兼容性代码 (防止在未初始化完成是,已经有交易员运行)
		apiClient := NewAPIClient()
		klines, err := apiClient.GetKlines(symbol, _time, 100)
		if err != nil {
			return nil, fmt.Errorf("获取%v分钟K线失败: %v", _time, err)
		}

		// 动态缓存进缓存
		m.getKlineDataMap(_time).Store(strings.ToUpper(symbol), klines)

		// 订阅 WebSocket 流
		subStr := m.subscribeSymbol(symbol, _time)
		subErr := m.combinedClient.subscribeStreams(subStr)
		log.Printf("动态订阅流: %v", subStr)
		if subErr != nil {
			log.Printf("警告: 动态订阅%v分钟K线失败: %v (使用API数据)", _time, subErr)
		}

		// ✅ FIX: 返回深拷贝而非引用
		result := make([]Kline, len(klines))
		copy(result, klines)
		return result, nil
	}

	// ✅ FIX: 返回深拷贝而非引用，避免并发竞态条件
	klines := value.([]Kline)
	result := make([]Kline, len(klines))
	copy(result, klines)
	return result, nil
}

func (m *WSMonitor) Close() {
	// P0修复：停止OI监控goroutine
	if m.oiStopChan != nil {
		close(m.oiStopChan)
	}

	m.wsClient.Close()
	close(m.alertsChan)
}

// P0修复：添加OI历史数据管理
const (
	OIHistoryMaxSize     = 20               // 最多保存20个OI快照（覆盖5小时，每15分钟采样）
	OIUpdateInterval     = 15 * time.Minute // OI采样间隔15分钟
	OIChange4hSampleSize = 16               // 4小时 = 16个15分钟样本
)

// StoreOISnapshot 存储OI快照到历史记录
func (m *WSMonitor) StoreOISnapshot(symbol string, oiValue float64) {
	// ✅ 修复：统一symbol格式（确保大小写一致）
	symbol = strings.ToUpper(symbol)

	snapshot := OISnapshot{
		Value:     oiValue,
		Timestamp: time.Now(),
	}

	cachedValue, exists := m.oiHistoryMap.Load(symbol)
	var history []OISnapshot
	if exists {
		history = cachedValue.([]OISnapshot)
	}

	// 添加新快照
	history = append(history, snapshot)

	// 保持最大长度
	if len(history) > OIHistoryMaxSize {
		history = history[1:]
	}

	m.oiHistoryMap.Store(symbol, history)

	// 診斷日誌（僅前3次採集時輸出）
	if len(history) <= 3 {
		log.Printf("📝 [OI存儲] Symbol: %s, OI: %.0f, 历史数据点数: %d", symbol, oiValue, len(history))
	}
}

// GetOIHistory 获取OI历史数据
func (m *WSMonitor) GetOIHistory(symbol string) []OISnapshot {
	// ✅ 修复：统一symbol格式（确保大小写一致）
	symbol = strings.ToUpper(symbol)

	value, exists := m.oiHistoryMap.Load(symbol)
	if !exists {
		return nil
	}
	return value.([]OISnapshot)
}

// CalculateOIChange4h 计算4小时OI变化率（如果数据不足，降级到最长可用时间）
// 返回：(变化率百分比, 实际时间段字符串)
func (m *WSMonitor) CalculateOIChange4h(symbol string, latestOI float64) (float64, string) {
	// ✅ 修复：统一symbol格式（确保大小写一致）
	symbol = strings.ToUpper(symbol)

	history := m.GetOIHistory(symbol)
	if len(history) == 0 {
		// ✅ P0修复：歷史數據為空時，嘗試從 API 回填（降級方案）
		log.Printf("⚠️  %s: OI历史数据为空，尝试从API回填历史数据...", symbol)
		apiClient := NewAPIClient()
		historyFromAPI, err := apiClient.GetOpenInterestHistory(symbol, "15m", 20) // 获取20个15分钟数据点（5小时）
		if err != nil {
			log.Printf("❌ %s: 从API回填OI历史数据失败: %v，无法计算变化率", symbol, err)
			return 0.0, "N/A" // API回填也失败，无法计算
		}

		if len(historyFromAPI) == 0 {
			log.Printf("⚠️  %s: API返回的OI历史数据为空，无法计算变化率", symbol)
			return 0.0, "N/A"
		}

		// 将回填的数据直接存储到缓存中（保留原始时间戳）
		m.oiHistoryMap.Store(symbol, historyFromAPI)
		log.Printf("✅ %s: 成功从API回填 %d 个OI历史数据点（时间跨度: %s 到 %s）",
			symbol, len(historyFromAPI),
			historyFromAPI[0].Timestamp.Format("15:04"),
			historyFromAPI[len(historyFromAPI)-1].Timestamp.Format("15:04"))

		// 重新获取历史数据（现在应该有数据了）
		history = m.GetOIHistory(symbol)
		if len(history) == 0 {
			log.Printf("❌ %s: 回填后历史数据仍为空，存储失败", symbol)
			return 0.0, "N/A"
		}
	}

	// ✅ 修复：只有 1 個數據點時，返回特殊標記而非 N/A
	// 這樣至少能顯示 Latest 值，只是無法計算變化率
	if len(history) == 1 {
		log.Printf("⚠️  %s: OI历史数据仅1个点（系统刚启动），变化率为0", symbol)
		return 0.0, "0m" // 特殊標記：剛啟動，無變化率數據
	}

	// 找到最早的数据点
	oldest := history[0]
	newest := history[len(history)-1]
	timeSpan := newest.Timestamp.Sub(oldest.Timestamp)

	// 計算實際可用的時間跨度
	actualHours := timeSpan.Hours()

	// 如果數據不足 4 小時，使用最早的數據點（降級策略）
	var oiOld float64
	var actualPeriod string

	if actualHours >= 3.5 { // 接近 4 小時（考慮採樣誤差）
		// 嘗試找 4 小時前的數據點
		now := time.Now()
		fourHoursAgo := now.Add(-4 * time.Hour)
		var closestTimeDiff time.Duration = 24 * time.Hour

		for _, snapshot := range history {
			timeDiff := snapshot.Timestamp.Sub(fourHoursAgo)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}
			if timeDiff < closestTimeDiff {
				closestTimeDiff = timeDiff
				oiOld = snapshot.Value
			}
		}

		// 如果找到的數據點時間差在 1 小時內，視為有效
		if closestTimeDiff <= 1*time.Hour {
			actualPeriod = "4h"
		} else {
			// 找不到 4h 前數據，降級使用最早數據點
			oiOld = oldest.Value
			actualPeriod = fmt.Sprintf("%.1fh", actualHours)
		}
	} else {
		// 數據不足 4 小時，使用最早的數據點
		oiOld = oldest.Value
		actualPeriod = fmt.Sprintf("%.1fh", actualHours)
	}

	// 计算变化率
	if oiOld == 0 {
		log.Printf("⚠️  %s: 历史OI值为0，无法计算变化率", symbol)
		return 0.0, "N/A"
	}

	change := ((latestOI - oiOld) / oiOld) * 100

	// 根據實際使用的時間段記錄日誌
	if actualPeriod == "4h" {
		log.Printf("✅ %s: OI 4h变化 %.3f%% (当前: %.0f, 4h前: %.0f)",
			symbol, change, latestOI, oiOld)
	} else {
		log.Printf("⚠️  %s: OI %s变化 %.3f%% (当前: %.0f, %s前: %.0f) [系统运行时间不足4h，使用降级计算]",
			symbol, actualPeriod, change, latestOI, actualPeriod, oiOld)
	}

	return change, actualPeriod
}

// StartOIMonitoring 启动OI定期监控（每15分钟采样）
func (m *WSMonitor) StartOIMonitoring() {
	log.Println("✅ 启动 OI 定期监控（每15分钟采样）")

	// 初始化停止通道
	m.oiStopChan = make(chan struct{})

	// 立即执行一次
	m.collectOISnapshots()

	// 定期执行（可优雅退出）
	ticker := time.NewTicker(OIUpdateInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.collectOISnapshots()
			case <-m.oiStopChan:
				log.Println("🛑 停止 OI 定期监控")
				return
			}
		}
	}()
}

// collectOISnapshots 采集所有交易对的OI快照（✅ 优化3：并发采集，性能提升 6 倍）
func (m *WSMonitor) collectOISnapshots() {
	apiClient := NewAPIClient()
	successCount := 0
	var mu sync.Mutex

	// ✅ 优化3：添加并发控制（semaphore=10）
	// 好处：执行时间从 ~12 秒降低到 ~2 秒（快 6 倍）
	// 安全性：仍然限制并发数，避免瞬时负荷过大
	semaphore := make(chan struct{}, 10)
	var wg sync.WaitGroup

	startTime := time.Now()

	for _, symbol := range m.symbols {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(s string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// ✅ 修复：添加重试机制（最多3次）
			var oiData *OIData
			var err error
			for retry := 0; retry < 3; retry++ {
				oiData, err = apiClient.GetOpenInterest(s)
				if err == nil {
					break
				}
				if retry < 2 {
					log.Printf("⚠️  获取 %s OI失败 (尝试 %d/3): %v，1秒后重试...", s, retry+1, err)
					time.Sleep(1 * time.Second)
				}
			}

			if err != nil {
				log.Printf("❌ 获取 %s OI失败（已重试3次）: %v", s, err)
				return
			}

			// 存储快照
			m.StoreOISnapshot(s, oiData.Latest)

			mu.Lock()
			successCount++
			mu.Unlock()
		}(symbol)
	}

	wg.Wait()

	elapsed := time.Since(startTime)
	log.Printf("✅ OI快照采集完成（成功: %d/%d，耗时: %.1f秒，时间: %s）",
		successCount, len(m.symbols), elapsed.Seconds(), time.Now().Format("15:04:05"))
}
