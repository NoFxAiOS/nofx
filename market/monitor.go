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
	klineDataMap3m  sync.Map // 存储每个交易对的K线历史数据
	klineDataMap15m sync.Map // 存储每个交易对的15分钟K线历史数据
	klineDataMap1h  sync.Map // 存储每个交易对的1小时K线历史数据
	klineDataMap4h  sync.Map // 存储每个交易对的K线历史数据
	tickerDataMap   sync.Map // 存储每个交易对的ticker数据
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

			// 获取4小时历史K线数据
			klines4h, err := apiClient.GetKlines(s, "4h", 100)
			if err != nil {
				log.Printf("获取 %s 4h历史数据失败: %v", s, err)
			} else if len(klines4h) > 0 {
				m.klineDataMap4h.Store(s, klines4h)
				log.Printf("已加载 %s 的历史K线数据-4h: %d 条", s, len(klines4h))
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
		log.Fatalf("❌ 初始化币种: %v", err)
		return
	}

	err = m.combinedClient.Connect()
	if err != nil {
		log.Fatalf("❌ 批量订阅流: %v", err)
		return
	}
	// 订阅所有交易对
	err = m.subscribeAll()
	if err != nil {
		log.Fatalf("❌ 订阅币种交易对: %v", err)
		return
	}
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
	// 执行批量订阅
	log.Println("开始订阅所有交易对...")
	for _, symbol := range m.symbols {
		for _, st := range subKlineTime {
			m.subscribeSymbol(symbol, st)
		}
	}
	for _, st := range subKlineTime {
		err := m.combinedClient.BatchSubscribeKlines(m.symbols, st)
		if err != nil {
			log.Fatalf("❌ 订阅3m K线: %v", err)
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
		m.getKlineDataMap(_time).Store(strings.ToUpper(symbol), klines) //动态缓存进缓存
		subStr := m.subscribeSymbol(symbol, _time)
		subErr := m.combinedClient.subscribeStreams(subStr)
		log.Printf("动态订阅流: %v", subStr)
		if subErr != nil {
			return nil, fmt.Errorf("动态订阅%v分钟K线失败: %v", _time, subErr)
		}
		if err != nil {
			return nil, fmt.Errorf("获取%v分钟K线失败: %v", _time, err)
		}
		return klines, fmt.Errorf("symbol不存在")
	}
	return value.([]Kline), nil
}

func (m *WSMonitor) Close() {
	m.wsClient.Close()
	close(m.alertsChan)
}
