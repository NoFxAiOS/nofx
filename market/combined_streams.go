package market

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type CombinedStreamsClient struct {
	conn        *websocket.Conn
	mu          sync.RWMutex
	subscribers map[string]chan []byte
	reconnect   bool
	done        chan struct{}
	batchSize   int // 每批订阅的流数量
}

func NewCombinedStreamsClient(batchSize int) *CombinedStreamsClient {
	return &CombinedStreamsClient{
		subscribers: make(map[string]chan []byte),
		reconnect:   true,
		done:        make(chan struct{}),
		batchSize:   batchSize,
	}
}

func (c *CombinedStreamsClient) Connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// 组合流使用不同的端点
	// 尝试多个端点以提高连接成功率
	endpoints := []string{
		"wss://fstream.binance.com/stream",
		"wss://stream.binance.com:9443/stream", // 备用端点
	}

	var lastErr error
	for _, endpoint := range endpoints {
		conn, _, err := dialer.Dial(endpoint, nil)
		if err != nil {
			log.Printf("⚠️ WebSocket连接失败 (%s): %v", endpoint, err)
			lastErr = err
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()

		log.Printf("✅ 组合流WebSocket连接成功: %s", endpoint)
		go c.readMessages()
		return nil
	}

	// 所有端点都失败
	log.Printf("⚠️ WebSocket实时数据流暂时不可用（网络问题）")
	log.Printf("💡 提示：系统将使用历史数据继续运行，AI决策不受影响")
	return fmt.Errorf("组合流WebSocket连接失败: %v", lastErr)
}

// BatchSubscribeKlines 批量订阅K线
func (c *CombinedStreamsClient) BatchSubscribeKlines(symbols []string, interval string) error {
	// 将symbols分批处理
	batches := c.splitIntoBatches(symbols, c.batchSize)

	for i, batch := range batches {
		log.Printf("订阅第 %d 批, 数量: %d", i+1, len(batch))

		streams := make([]string, len(batch))
		for j, symbol := range batch {
			streams[j] = fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
		}

		if err := c.subscribeStreams(streams); err != nil {
			return fmt.Errorf("第 %d 批订阅失败: %v", i+1, err)
		}

		// 批次间延迟，避免被限制
		if i < len(batches)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// splitIntoBatches 将切片分成指定大小的批次
func (c *CombinedStreamsClient) splitIntoBatches(symbols []string, batchSize int) [][]string {
	var batches [][]string

	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batches = append(batches, symbols[i:end])
	}

	return batches
}

// subscribeStreams 订阅多个流
func (c *CombinedStreamsClient) subscribeStreams(streams []string) error {
	subscribeMsg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": streams,
		"id":     time.Now().UnixNano(),
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	log.Printf("订阅流: %v", streams)
	return c.conn.WriteJSON(subscribeMsg)
}

func (c *CombinedStreamsClient) readMessages() {
	for {
		select {
		case <-c.done:
			return
		default:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("读取组合流消息失败: %v", err)
				c.handleReconnect()
				return
			}

			c.handleCombinedMessage(message)
		}
	}
}

func (c *CombinedStreamsClient) handleCombinedMessage(message []byte) {
	var combinedMsg struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &combinedMsg); err != nil {
		log.Printf("解析组合消息失败: %v", err)
		return
	}

	c.mu.RLock()
	ch, exists := c.subscribers[combinedMsg.Stream]
	c.mu.RUnlock()

	if exists {
		select {
		case ch <- combinedMsg.Data:
		default:
			log.Printf("订阅者通道已满: %s", combinedMsg.Stream)
		}
	}
}

func (c *CombinedStreamsClient) AddSubscriber(stream string, bufferSize int) <-chan []byte {
	ch := make(chan []byte, bufferSize)
	c.mu.Lock()
	c.subscribers[stream] = ch
	c.mu.Unlock()
	return ch
}

func (c *CombinedStreamsClient) handleReconnect() {
	if !c.reconnect {
		return
	}

	log.Println("组合流尝试重新连接...")
	time.Sleep(3 * time.Second)

	if err := c.Connect(); err != nil {
		log.Printf("组合流重新连接失败: %v", err)
		go c.handleReconnect()
	}
}

func (c *CombinedStreamsClient) Close() {
	c.reconnect = false
	close(c.done)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	for stream, ch := range c.subscribers {
		close(ch)
		delete(c.subscribers, stream)
	}
}
