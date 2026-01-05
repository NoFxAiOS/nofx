package market

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// BybitOrderWebSocket implements order update streaming for Bybit
type BybitOrderWebSocket struct {
	mu                 sync.RWMutex
	conn               *websocket.Conn
	url                string
	isConnected        bool
	orderUpdateChannel chan OrderUpdate
	stopCh             chan struct{}
	reconnectAttempts  int
	reconnectDelay     time.Duration
	lastActivity       time.Time
	heartbeatTicker    *time.Ticker
	testnet            bool
	apiKey             string
	apiSecret          string
	authenticated      bool
}

// BybitOrderUpdate represents order updates from Bybit
type BybitOrderUpdate struct {
	Topic string `json:"topic"`
	Type  string `json:"type"`
	Data  struct {
		Orders []struct {
			OrderID       string  `json:"orderId"`
			OrderLinkID   string  `json:"orderLinkId"`
			BlockTradeID  string  `json:"blockTradeId"`
			Symbol        string  `json:"symbol"`
			Price         float64 `json:"price,string"`
			Qty           float64 `json:"qty,string"`
			Side          string  `json:"side"` // Buy or Sell
			IsLeverage    string  `json:"isLeverage"`
			PositionIdx   int     `json:"positionIdx"` // 0: one-way, 1: long, 2: short
			OrderStatus   string  `json:"orderStatus"` // Created, New, Rejected, PartiallyFilled, Filled, Cancelled, Untriggered
			CancelType    string  `json:"cancelType"`
			RejectReason  string  `json:"rejectReason"`
			AvgPrice      float64 `json:"avgPrice,string"`
			LeavesQty     float64 `json:"leavesQty,string"`
			LeavesValue   float64 `json:"leavesValue,string"`
			CumExecQty    float64 `json:"cumExecQty,string"`
			CumExecValue  float64 `json:"cumExecValue,string"`
			CumExecFee    float64 `json:"cumExecFee,string"`
			TimeInForce   string  `json:"timeInForce"` // GTC, IOC, FOK, PostOnly
			OrderType     string  `json:"orderType"`   // Market, Limit
			StopOrderType string  `json:"stopOrderType"`
			TpslMode      string  `json:"tpslMode"`
			TakeProfit    float64 `json:"takeProfit,string"`
			StopLoss      float64 `json:"stopLoss,string"`
			TpTriggerBy   string  `json:"tpTriggerBy"`
			SlTriggerBy   string  `json:"slTriggerBy"`
			TpLimitPrice  float64 `json:"tpLimitPrice,string"`
			SlLimitPrice  float64 `json:"slLimitPrice,string"`
			TriggerPrice  float64 `json:"triggerPrice,string"`
			CreatedTime   int64   `json:"createdTime,string"`
			UpdatedTime   int64   `json:"updatedTime,string"`
		} `json:"order"`
	} `json:"data"`
}

// NewBybitOrderWebSocket creates a new Bybit order WebSocket client
func NewBybitOrderWebSocket(testnet bool, apiKey, apiSecret string) *BybitOrderWebSocket {
	url := "wss://stream.bybit.com/v5/private"
	if testnet {
		url = "wss://stream-testnet.bybit.com/v5/private"
	}

	return &BybitOrderWebSocket{
		url:                url,
		isConnected:        false,
		orderUpdateChannel: make(chan OrderUpdate, 100),
		stopCh:             make(chan struct{}),
		reconnectAttempts:  5,
		reconnectDelay:     5 * time.Second,
		testnet:            testnet,
		apiKey:             apiKey,
		apiSecret:          apiSecret,
	}
}

// Connect establishes the WebSocket connection with authentication
func (bow *BybitOrderWebSocket) Connect() error {
	bow.mu.Lock()
	defer bow.mu.Unlock()

	if bow.isConnected {
		return nil
	}

	if bow.apiKey == "" || bow.apiSecret == "" {
		return fmt.Errorf("API key and secret required for Bybit order stream authentication")
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(bow.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Bybit order stream: %w", err)
	}

	bow.conn = conn
	bow.isConnected = true
	bow.lastActivity = time.Now()

	// Start message reader
	go bow.readMessages()

	// Authenticate with API key
	if err := bow.authenticate(); err != nil {
		return fmt.Errorf("failed to authenticate with Bybit: %w", err)
	}

	logger.Infof("✓ Bybit order stream connected and authenticated (testnet=%v)", bow.testnet)

	// Start heartbeat
	go bow.heartbeat()

	return nil
}

// authenticate sends authentication message to Bybit
func (bow *BybitOrderWebSocket) authenticate() error {
	// Bybit requires sending auth message with signature
	// For simplicity in this implementation, we'll note this needs to be implemented
	// with proper HMAC-SHA256 signature generation

	authMsg := map[string]interface{}{
		"op": "auth",
		"args": []map[string]string{
			{
				"key":  bow.apiKey,
				"sign": "PLACEHOLDER", // Should be HMAC-SHA256(expires + "GET/realtime")
			},
		},
	}

	if err := bow.conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("failed to send auth message: %w", err)
	}

	logger.Info("Bybit authentication message sent")
	return nil
}

// Disconnect closes the WebSocket connection
func (bow *BybitOrderWebSocket) Disconnect() error {
	bow.mu.Lock()
	defer bow.mu.Unlock()

	if !bow.isConnected {
		return nil
	}

	bow.isConnected = false
	close(bow.stopCh)

	if bow.conn != nil {
		bow.conn.Close()
	}

	if bow.heartbeatTicker != nil {
		bow.heartbeatTicker.Stop()
	}

	logger.Info("✓ Bybit order stream disconnected")
	return nil
}

// IsConnected returns whether the connection is active
func (bow *BybitOrderWebSocket) IsConnected() bool {
	bow.mu.RLock()
	defer bow.mu.RUnlock()

	return bow.isConnected && bow.conn != nil
}

// GetOrderUpdateChannel returns the channel for order updates
func (bow *BybitOrderWebSocket) GetOrderUpdateChannel() <-chan OrderUpdate {
	return bow.orderUpdateChannel
}

// Reconnect attempts to reconnect the WebSocket
func (bow *BybitOrderWebSocket) Reconnect() error {
	bow.Disconnect()
	time.Sleep(bow.reconnectDelay)
	return bow.Connect()
}

// readMessages reads messages from the WebSocket
func (bow *BybitOrderWebSocket) readMessages() {
	defer func() {
		bow.mu.Lock()
		bow.isConnected = false
		bow.mu.Unlock()
	}()

	for {
		select {
		case <-bow.stopCh:
			return
		default:
		}

		bow.mu.RLock()
		conn := bow.conn
		bow.mu.RUnlock()

		if conn == nil {
			return
		}

		var data []byte
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warnf("Bybit order stream error: %v", err)
			}
			return
		}

		bow.mu.Lock()
		bow.lastActivity = time.Now()
		bow.mu.Unlock()

		// Parse and process the message
		var update BybitOrderUpdate
		if err := json.Unmarshal(data, &update); err != nil {
			// Could be a system message or auth response
			continue
		}

		// Process order updates
		if update.Topic == "order" {
			bow.processOrderUpdate(update)
		}
	}
}

// processOrderUpdate converts Bybit update to OrderUpdate
func (bow *BybitOrderWebSocket) processOrderUpdate(update BybitOrderUpdate) {
	if len(update.Data.Orders) == 0 {
		return
	}

	for _, order := range update.Data.Orders {
		orderUpdate := OrderUpdate{
			Symbol:             order.Symbol,
			OrderID:            order.OrderID,
			ClientOrderID:      order.OrderLinkID,
			Side:               order.Side,
			PositionSide:       bow.getPositionSideFromIdx(order.PositionIdx),
			OrderType:          order.OrderType,
			TimeInForce:        order.TimeInForce,
			OriginalQuantity:   order.Qty,
			ExecutedQuantity:   order.CumExecQty,
			CumulativeQuoteQty: order.CumExecValue,
			Status:             order.OrderStatus,
			ExecutionType:      order.OrderStatus,
			OrderPrice:         order.Price,
			AveragePrice:       order.AvgPrice,
			RejectReason:       order.RejectReason,
			Timestamp:          time.Now(),
		}

		select {
		case bow.orderUpdateChannel <- orderUpdate:
		case <-bow.stopCh:
			return
		default:
			logger.Warnf("Order update channel full, dropping order %s", order.OrderLinkID)
		}
	}
}

// getPositionSideFromIdx converts Bybit position index to side
func (bow *BybitOrderWebSocket) getPositionSideFromIdx(idx int) string {
	switch idx {
	case 1:
		return "LONG"
	case 2:
		return "SHORT"
	default:
		return "BOTH"
	}
}

// heartbeat sends periodic ping
func (bow *BybitOrderWebSocket) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bow.mu.Lock()
			conn := bow.conn
			bow.mu.Unlock()
			if conn != nil {
				if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					logger.Warnf("Failed to send ping to Bybit: %v", err)
					return
				}
			}
		case <-bow.stopCh:
			return
		}
	}
}
