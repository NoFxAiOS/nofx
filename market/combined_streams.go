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
	conn              *websocket.Conn
	mu                sync.RWMutex
	subscribers       map[string]chan []byte
	subscribedStreams []string // 已订阅的流列表，用于重连恢复
	reconnect         bool
	done              chan struct{}
	batchSize         int // 每批订阅的流数量
}

func NewCombinedStreamsClient(batchSize int) *CombinedStreamsClient {
	return &CombinedStreamsClient{
		subscribers:       make(map[string]chan []byte),
		subscribedStreams: make([]string, 0),
		reconnect:         true,
		done:              make(chan struct{}),
		batchSize:         batchSize,
	}
}

func (c *CombinedStreamsClient) Connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// 组合流使用不同的端点
	conn, _, err := dialer.Dial("wss://fstream.binance.com/stream", nil)
	if err != nil {
		return fmt.Errorf("组合流WebSocket连接失败: %v", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	log.Println("组合流WebSocket连接成功")
	go c.readMessages()

	return nil
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
	// 检查是否已经订阅，避免重复
	exists := false
	for _, s := range c.subscribedStreams {
		if s == stream {
			exists = true
			break
		}
	}
	if !exists {
		c.subscribedStreams = append(c.subscribedStreams, stream)
	}
	c.mu.Unlock()
	return ch
}

// handleReconnect 处理重连逻辑，使用退避重连策略
func (c *CombinedStreamsClient) handleReconnect() {
	if !c.reconnect {
		return
	}

	maxBackoff := 60 * time.Second
	backoff := 3 * time.Second
	retryCount := 0

	for {
		retryCount++
		log.Printf("组合流尝试重新连接 (第 %d 次)...", retryCount)

		err := c.Connect()
		if err == nil {
			log.Println("✅ 组合流重连成功，开始恢复订阅...")
			c.resubscribeAll()
			return
		}

		log.Printf("❌ 组合流重连失败: %v", err)
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
		case <-c.done:
			log.Println("🔚 收到退出信号，停止重连")
			return
		default:
			// 继续重试
		}
	}
}

// resubscribeAll 重新订阅所有已订阅的流
func (c *CombinedStreamsClient) resubscribeAll() {
	c.mu.RLock()
	streams := make([]string, len(c.subscribedStreams))
	copy(streams, c.subscribedStreams)
	c.mu.RUnlock()

	if len(streams) == 0 {
		log.Println("⚠️ 没有已订阅的流需要恢复")
		return
	}

	log.Printf("🔄 重新订阅 %d 个流...", len(streams))
	successCount := 0
	failCount := 0

	for _, stream := range streams {
		if err := c.subscribeStream(stream); err != nil {
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
func (c *CombinedStreamsClient) subscribeStream(stream string) error {
	subscribeMsg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": []string{stream},
		"id":     time.Now().UnixNano(),
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	return c.conn.WriteJSON(subscribeMsg)
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
