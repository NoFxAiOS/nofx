package market

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type WSMonitor struct {
	wsClient       *WSClient
	combinedClient *CombinedStreamsClient
	symbols        []string
	featuresMap    sync.Map
	alertsChan     chan Alert
	klineDataMap3m sync.Map // Store K-line historical data for each trading pair
	klineDataMap4h sync.Map // Store K-line historical data for each trading pair
	tickerDataMap  sync.Map // Store ticker data for each trading pair
	batchSize      int
	filterSymbols  sync.Map // Use sync.Map to store monitored coins and their status
	symbolStats    sync.Map // Store symbol statistics
	FilterSymbol   []string // Filtered symbols
}
type SymbolStats struct {
	LastActiveTime   time.Time
	AlertCount       int
	VolumeSpikeCount int
	LastAlertTime    time.Time
	Score            float64 // Composite score
}

var WSMonitorCli *WSMonitor
var subKlineTime = []string{"3m", "4h"} // Manage K-line periods for subscription streams

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
	log.Println("Initializing WebSocket monitor...")
	// Get trading pair information
	apiClient := NewAPIClient()
	// If trading pairs are not specified, use all trading pairs from the market
	if len(coins) == 0 {
		exchangeInfo, err := apiClient.GetExchangeInfo()
		if err != nil {
			return err
		}
		// Filter perpetual contract trading pairs -- only use for testing
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

	log.Printf("Found %d trading pairs", len(m.symbols))
	// Initialize historical data
	if err := m.initializeHistoricalData(); err != nil {
		log.Printf("Failed to initialize historical data: %v", err)
	}

	return nil
}

func (m *WSMonitor) initializeHistoricalData() error {
	apiClient := NewAPIClient()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit concurrency

	for _, symbol := range m.symbols {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(s string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// Get historical K-line data
			klines, err := apiClient.GetKlines(s, "3m", 100)
			if err != nil {
				log.Printf("Failed to get %s historical data: %v", s, err)
				return
			}
			if len(klines) > 0 {
				m.klineDataMap3m.Store(s, klines)
				log.Printf("Loaded %s historical K-line data-3m: %d entries", s, len(klines))
			}
			// Get historical K-line data
			klines4h, err := apiClient.GetKlines(s, "4h", 100)
			if err != nil {
				log.Printf("Failed to get %s historical data: %v", s, err)
				return
			}
			if len(klines4h) > 0 {
				m.klineDataMap4h.Store(s, klines4h)
				log.Printf("Loaded %s historical K-line data-4h: %d entries", s, len(klines4h))
			}
		}(symbol)
	}

	wg.Wait()
	return nil
}

func (m *WSMonitor) Start(coins []string) {
	log.Printf("Starting WebSocket real-time monitoring...")
	// Initialize trading pairs
	err := m.Initialize(coins)
	if err != nil {
		log.Printf("❌ Failed to initialize coins: %v", err)
		return
	}

	err = m.combinedClient.Connect()
	if err != nil {
		log.Printf("❌ Failed to batch subscribe to streams: %v", err)
		return
	}
	// Subscribe to all trading pairs
	err = m.subscribeAll()
	if err != nil {
		log.Printf("❌ Failed to subscribe to coin trading pairs: %v", err)
		return
	}
}

// subscribeSymbol registers listener
func (m *WSMonitor) subscribeSymbol(symbol, st string) []string {
	var streams []string
	stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), st)
	ch := m.combinedClient.AddSubscriber(stream, 100)
	streams = append(streams, stream)
	go m.handleKlineData(symbol, ch, st)

	return streams
}
func (m *WSMonitor) subscribeAll() error {
	// Execute batch subscription
	log.Println("Starting to subscribe to all trading pairs...")
	for _, symbol := range m.symbols {
		for _, st := range subKlineTime {
			m.subscribeSymbol(symbol, st)
		}
	}
	for _, st := range subKlineTime {
		err := m.combinedClient.BatchSubscribeKlines(m.symbols, st)
		if err != nil {
			log.Printf("❌ Failed to subscribe to %s K-line: %v", st, err)
			return err
		}
	}
	log.Println("All trading pair subscriptions completed")
	return nil
}

func (m *WSMonitor) handleKlineData(symbol string, ch <-chan []byte, _time string) {
	for data := range ch {
		var klineData KlineWSData
		if err := json.Unmarshal(data, &klineData); err != nil {
			log.Printf("Failed to parse Kline data: %v", err)
			continue
		}
		m.processKlineUpdate(symbol, klineData, _time)
	}
}

func (m *WSMonitor) getKlineDataMap(_time string) *sync.Map {
	var klineDataMap *sync.Map
	if _time == "3m" {
		klineDataMap = &m.klineDataMap3m
	} else if _time == "4h" {
		klineDataMap = &m.klineDataMap4h
	} else {
		klineDataMap = &sync.Map{}
	}
	return klineDataMap
}
func (m *WSMonitor) processKlineUpdate(symbol string, wsData KlineWSData, _time string) {
	// Convert WebSocket data to Kline structure
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
	// Update K-line data
	var klineDataMap = m.getKlineDataMap(_time)
	value, exists := klineDataMap.Load(symbol)
	var klines []Kline
	if exists {
		klines = value.([]Kline)

		// Check if it's a new K-line
		if len(klines) > 0 && klines[len(klines)-1].OpenTime == kline.OpenTime {
			// Update current K-line
			klines[len(klines)-1] = kline
		} else {
			// Add new K-line
			klines = append(klines, kline)

			// Maintain data length
			if len(klines) > 100 {
				klines = klines[1:]
			}
		}
	} else {
		klines = []Kline{kline}
	}

	klineDataMap.Store(symbol, klines)
}

func (m *WSMonitor) GetCurrentKlines(symbol string, duration string) ([]Kline, error) {
	// Check if each incoming symbol exists internally, if not subscribe to it
	value, exists := m.getKlineDataMap(duration).Load(symbol)
	if !exists {
		// If WS data is not initialized, use API separately - compatibility code (prevents trader from running when not initialized)
		apiClient := NewAPIClient()
		klines, err := apiClient.GetKlines(symbol, duration, 100)
		if err != nil {
			return nil, fmt.Errorf("Failed to get %v-minute K-line: %v", duration, err)
		}

		// Dynamically cache into cache
		m.getKlineDataMap(duration).Store(strings.ToUpper(symbol), klines)

		// Subscribe to WebSocket stream
		subStr := m.subscribeSymbol(symbol, duration)
		subErr := m.combinedClient.subscribeStreams(subStr)
		log.Printf("Dynamic subscription to stream: %v", subStr)
		if subErr != nil {
			log.Printf("Warning: Failed to dynamically subscribe to %v-minute K-line: %v (using API data)", duration, subErr)
		}

		// ✅ FIX: Return deep copy instead of reference
		result := make([]Kline, len(klines))
		copy(result, klines)
		return result, nil
	}

	// ✅ FIX: Return deep copy instead of reference, avoid concurrent race conditions
	klines := value.([]Kline)
	result := make([]Kline, len(klines))
	copy(result, klines)
	return result, nil
}

func (m *WSMonitor) Close() {
	m.wsClient.Close()
	close(m.alertsChan)
}
