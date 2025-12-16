package market

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WSClient struct {
	conn              *websocket.Conn
	mu                sync.RWMutex
	subscribers       map[string]chan []byte
	subscribedStreams []string // 已订阅的流列表，用于重连恢复
	reconnect         bool
	done              chan struct{}
}

type WSMessage struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

type KlineWSData struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Kline     struct {
		StartTime           int64  `json:"t"`
		CloseTime           int64  `json:"T"`
		Symbol              string `json:"s"`
		Interval            string `json:"i"`
		FirstTradeID        int64  `json:"f"`
		LastTradeID         int64  `json:"L"`
		OpenPrice           string `json:"o"`
		ClosePrice          string `json:"c"`
		HighPrice           string `json:"h"`
		LowPrice            string `json:"l"`
		Volume              string `json:"v"`
		NumberOfTrades      int    `json:"n"`
		IsFinal             bool   `json:"x"`
		QuoteVolume         string `json:"q"`
		TakerBuyBaseVolume  string `json:"V"`
		TakerBuyQuoteVolume string `json:"Q"`
	} `json:"k"`
}

type TickerWSData struct {
	EventType          string `json:"e"`
	EventTime          int64  `json:"E"`
	Symbol             string `json:"s"`
	PriceChange        string `json:"p"`
	PriceChangePercent string `json:"P"`
	WeightedAvgPrice   string `json:"w"`
	LastPrice          string `json:"c"`
	LastQty            string `json:"Q"`
	OpenPrice          string `json:"o"`
	HighPrice          string `json:"h"`
	LowPrice           string `json:"l"`
	Volume             string `json:"v"`
	QuoteVolume        string `json:"q"`
	OpenTime           int64  `json:"O"`
	CloseTime          int64  `json:"C"`
	FirstID            int64  `json:"F"`
	LastID             int64  `json:"L"`
	Count              int    `json:"n"`
}

func NewWSClient() *WSClient {
	return &WSClient{
		subscribers:       make(map[string]chan []byte),
		subscribedStreams: make([]string, 0),
		reconnect:         true,
		done:              make(chan struct{}),
	}
}

func (w *WSClient) Connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial("wss://ws-fapi.binance.com/ws-fapi/v1", nil)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %v", err)
	}

	w.mu.Lock()
	w.conn = conn
	w.mu.Unlock()

	log.Println("WebSocket连接成功")

	// 启动消息读取循环
	go w.readMessages()

	return nil
}

func (w *WSClient) SubscribeKline(symbol, interval string) error {
	stream := fmt.Sprintf("%s@kline_%s", symbol, interval)
	return w.subscribe(stream)
}

func (w *WSClient) SubscribeTicker(symbol string) error {
	stream := fmt.Sprintf("%s@ticker", symbol)
	return w.subscribe(stream)
}

func (w *WSClient) SubscribeMiniTicker(symbol string) error {
	stream := fmt.Sprintf("%s@miniTicker", symbol)
	return w.subscribe(stream)
}

func (w *WSClient) subscribe(stream string) error {
	subscribeMsg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": []string{stream},
		"id":     time.Now().Unix(),
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	err := w.conn.WriteJSON(subscribeMsg)
	if err != nil {
		return err
	}

	log.Printf("订阅流: %s", stream)
	return nil
}

func (w *WSClient) readMessages() {
	for {
		select {
		case <-w.done:
			return
		default:
			w.mu.RLock()
			conn := w.conn
			w.mu.RUnlock()

			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("读取WebSocket消息失败: %v", err)
				w.handleReconnect()
				return
			}

			w.handleMessage(message)
		}
	}
}

func (w *WSClient) handleMessage(message []byte) {
	var wsMsg WSMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		// 可能是其他格式的消息
		return
	}

	w.mu.RLock()
	ch, exists := w.subscribers[wsMsg.Stream]
	w.mu.RUnlock()

	if exists {
		select {
		case ch <- wsMsg.Data:
		default:
			log.Printf("订阅者通道已满: %s", wsMsg.Stream)
		}
	}
}

// handleReconnect 处理重连逻辑，使用退避重连策略
func (w *WSClient) handleReconnect() {
	if !w.reconnect {
		return
	}

	maxBackoff := 60 * time.Second
	backoff := 3 * time.Second
	retryCount := 0

	for {
		retryCount++
		log.Printf("WebSocket尝试重新连接 (第 %d 次)...", retryCount)

		err := w.Connect()
		if err == nil {
			log.Println("✅ WebSocket重连成功，开始恢复订阅...")
			w.resubscribeAll()
			return
		}

		log.Printf("❌ WebSocket重连失败: %v", err)
		log.Printf("⏳ 等待 %v 后重试...", backoff)
		time.Sleep(backoff)

		// 指数退避，但不超过最大值
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
			log.Println("⚠️ 达到最大退避时间，使用固定间隔重试")
		}

		// 检查是否应该退出重连循环
		select {
		case <-w.done:
			log.Println("🔚 收到退出信号，停止重连")
			return
		default:
			// 继续重试
		}
	}
}

// resubscribeAll 重新订阅所有已订阅的流
func (w *WSClient) resubscribeAll() {
	w.mu.RLock()
	streams := make([]string, len(w.subscribedStreams))
	copy(streams, w.subscribedStreams)
	w.mu.RUnlock()

	if len(streams) == 0 {
		log.Println("⚠️ 没有已订阅的流需要恢复")
		return
	}

	log.Printf("🔄 重新订阅 %d 个流...", len(streams))
	successCount := 0
	failCount := 0

	for _, stream := range streams {
		if err := w.subscribeStream(stream); err != nil {
			log.Printf("❌ 重新订阅流 %s 失败: %v", stream, err)
			failCount++
		} else {
			successCount++
			log.Printf("  ✅ 重新订阅成功: %s", stream)
			// 短暂延迟避免请求过快
			time.Sleep(50 * time.Millisecond)
		}
	}

	log.Printf("📊 重订阅完成: %d 成功, %d 失败", successCount, failCount)
	if failCount > 0 {
		log.Printf("⚠️ 部分流订阅失败，可能需要手动检查网络连接")
	}
}

// subscribeStream 订阅单个流
func (w *WSClient) subscribeStream(stream string) error {
	subscribeMsg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": []string{stream},
		"id":     time.Now().UnixNano(),
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	return w.conn.WriteJSON(subscribeMsg)
}

func (w *WSClient) AddSubscriber(stream string, bufferSize int) <-chan []byte {
	ch := make(chan []byte, bufferSize)
	w.mu.Lock()
	w.subscribers[stream] = ch
	// 检查是否已经订阅，避免重复
	exists := false
	for _, s := range w.subscribedStreams {
		if s == stream {
			exists = true
			break
		}
	}
	if !exists {
		w.subscribedStreams = append(w.subscribedStreams, stream)
	}
	w.mu.Unlock()
	return ch
}

func (w *WSClient) RemoveSubscriber(stream string) {
	w.mu.Lock()
	delete(w.subscribers, stream)
	w.mu.Unlock()
}

func (w *WSClient) Close() {
	w.reconnect = false
	close(w.done)

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}

	// 关闭所有订阅者通道
	for stream, ch := range w.subscribers {
		close(ch)
		delete(w.subscribers, stream)
	}
}
