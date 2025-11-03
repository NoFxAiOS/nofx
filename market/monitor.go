package market

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
)

type WSMonitor struct {
	wsClient      *WSClient
	symbols       []string
	klinesDataMap map[string]*klinesDataMap //分时管理K线 格式为:klinesDataMap[3m].kline.store(symbol,kline)
	tickerDataMap sync.Map                  // 存储每个交易对的ticker数据
	batchSize     int                       //分批订阅
	subKlineTime  []string                  //分时K线
}
type klinesDataMap struct {
	combinedClient *CombinedStreamsClient
	kline          sync.Map
}

var WSMonitorCli *WSMonitor

func NewWSMonitor(batchSize int, subKlineTime []string) *WSMonitor {
	WSMonitorCli = &WSMonitor{
		wsClient:      NewWSClient(),
		batchSize:     batchSize,
		subKlineTime:  subKlineTime,
		klinesDataMap: make(map[string]*klinesDataMap),
	}
	// 初始化分时订阅K线
	for _, timeframe := range WSMonitorCli.subKlineTime {
		client := NewCombinedStreamsClient(batchSize)
		conn := client.Connect()
		if conn != nil {
			log.Fatalf("[%v]周期Ws初始化失败:%s", timeframe, conn.Error())
		}
		WSMonitorCli.klinesDataMap[timeframe] = &klinesDataMap{
			combinedClient: client,
			kline:          sync.Map{},
		}
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
		//exchangeInfo.Symbols = exchangeInfo.Symbols[0:5]
		for _, symbol := range exchangeInfo.Symbols {
			if symbol.Status == "TRADING" && symbol.ContractType == "PERPETUAL" && strings.ToUpper(symbol.Symbol[len(symbol.Symbol)-4:]) == "USDT" {
				m.symbols = append(m.symbols, symbol.Symbol)
			}
		}
	} else {
		m.symbols = coins
	}

	log.Printf("找到 %d 个交易对", len(m.symbols))
	// 初始化历史数据
	if err := m.initializeHistoricalData(); err != nil {
		log.Printf("初始化历史数据失败: %v", err)
	}

	return nil
}

func (m *WSMonitor) initializeHistoricalData() error {
	apiClient := NewAPIClient()
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // 限制并发数
	for _, _time := range m.subKlineTime {
		log.Printf("加载 %v K线", _time)
		for _, symbol := range m.symbols {
			wg.Add(1)
			semaphore <- struct{}{}
			go func(t, s string) {
				defer wg.Done()
				defer func() { <-semaphore }()
				// 获取历史K线数据
				klines, err := apiClient.GetKlines(s, t, 100)
				if err != nil {
					log.Printf("获取 %s 历史数据失败: %v", s, err)
				}
				if len(klines) > 0 {
					m.klinesDataMap[t].kline.Store(s, klines)
					log.Printf("已加载 %s 的历史K线数据-%v: %d 条", s, t, len(klines))
				}
				// 订阅K线
				m.subscribeSymbol(s, t)
			}(_time, symbol)

		}
		wg.Wait()
		err := m.klinesDataMap[_time].combinedClient.BatchSubscribeKlines(m.symbols, _time)
		if err != nil {
			log.Fatalf("❌ 订阅%v K线: %v", _time, err)
		}
	}

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
}

// subscribeSymbol 注册监听
func (m *WSMonitor) subscribeSymbol(symbol, st string) []string {
	var streams []string
	stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), st)
	ch := m.klinesDataMap[st].combinedClient.AddSubscriber(stream, 1024*len(m.subKlineTime)) // 每个流最多订阅1024 然后 x 订阅时间数
	streams = append(streams, stream)
	go m.handleKlineData(symbol, ch, st)

	return streams
}

// handleKlineData 解析K线
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

// getKlineDataMap 根据时间周期获取K线
func (m *WSMonitor) getKlineDataMap(_time string) *sync.Map {
	return &m.klinesDataMap[_time].kline
}

// 获取所有时间周期的K线数据
func (m *WSMonitor) getAllPeriodKlines(symbol string) map[string][]Kline {
	klinesMap := make(map[string][]Kline)

	for _, period := range m.subKlineTime {
		if _klineDataMap, exists := m.klinesDataMap[period]; exists {
			if value, ok := _klineDataMap.kline.Load(symbol); ok {
				klinesMap[period] = value.([]Kline)
			}
		}
	}
	return klinesMap
}

// processKlineUpdate K线归类到时间
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

func (m *WSMonitor) processTickerUpdate(symbol string, tickerData TickerWSData) {
	// 存储ticker数据
	m.tickerDataMap.Store(symbol, tickerData)
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
		subErr := m.klinesDataMap[_time].combinedClient.subscribeStreams(subStr)
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
	for _, timeframe := range m.subKlineTime {
		if klineData, exists := m.klinesDataMap[timeframe]; exists {
			klineData.combinedClient.Close()
		}
	}
}
