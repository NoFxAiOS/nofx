package market

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"nofx/logger"
	"sync"
	"time"
)

// BinanceOrderWebSocket implements order update streaming for Binance
type BinanceOrderWebSocket struct {
	mu                 sync.RWMutex
	conn               *websocket.Conn
	listenKey          string
	baseURL            string
	isConnected        bool
	orderUpdateChannel chan OrderUpdate
	stopCh             chan struct{}
	reconnectAttempts  int
	reconnectDelay     time.Duration
	lastActivity       time.Time
	heartbeatTicker    *time.Ticker
	heartbeatInterval  time.Duration
	testnet            bool
	reconnectCh        chan struct{} // For triggering reconnects
}

// BinanceAccountUpdate represents account and order updates from Binance
type BinanceAccountUpdate struct {
	EventType            string  `json:"e"` // "ACCOUNT_UPDATE" or "ORDER_TRADE_UPDATE"
	EventTime            int64   `json:"E"`
	TransactionTime      int64   `json:"T"`
	BuyerCommissionRate  float64 `json:"b"`
	SellerCommissionRate float64 `json:"a"`
	CommissionAsset      string  `json:"m"`
	Balances             []struct {
		Asset  string  `json:"a"`
		Free   float64 `json:"f,string"`
		Locked float64 `json:"l,string"`
	} `json:"B"`
	Orders []struct {
		Symbol               string  `json:"s"`
		ClientOrderID        string  `json:"c"`
		Side                 string  `json:"S"` // BUY or SELL
		OrderType            string  `json:"o"` // LIMIT, MARKET, etc
		TimeInForce          string  `json:"f"` // GTC, IOC, FOK
		OriginalQuantity     float64 `json:"q,string"`
		ExecutedQuantity     float64 `json:"z,string"`
		CumulativeQuoteAsset float64 `json:"Z,string"`
		Status               string  `json:"X"` // NEW, PARTIALLY_FILLED, FILLED, CANCELED, etc
		OrderID              int64   `json:"i"`
		LastExecutedPrice    float64 `json:"L,string"`
		Commission           float64 `json:"n,string"`
		CommissionAsset      string  `json:"N"`
		OrderCreationTime    int64   `json:"O"`
		OrderUpdateTime      int64   `json:"T"`
		IsWorking            bool    `json:"w"`
		OriginalOrderType    string  `json:"ot"`
		PositionSide         string  `json:"ps"` // LONG, SHORT, BOTH
		StopPrice            float64 `json:"P,string"`
		TrailingDelta        int64   `json:"d"`
		TrailingTime         int64   `json:"dt"`
		Price                float64 `json:"p,string"`  // Order price
		AvgPrice             float64 `json:"ap,string"` // Average execution price
	} `json:"o"`
}

// NewBinanceOrderWebSocket creates a new Binance order WebSocket client
func NewBinanceOrderWebSocket(testnet bool) *BinanceOrderWebSocket {
	baseURL := "wss://fstream.binance.com/ws"
	if testnet {
		baseURL = "wss://stream.binancefuture.com/ws"
	}

	return &BinanceOrderWebSocket{
		baseURL:            baseURL,
		isConnected:        false,
		orderUpdateChannel: make(chan OrderUpdate, 100),
		stopCh:             make(chan struct{}),
		reconnectAttempts:  5,
		reconnectDelay:     5 * time.Second,
		heartbeatInterval:  30 * time.Second,
		testnet:            testnet,
	}
}

// SetListenKey sets the user data stream listen key (required for order updates)
// Must be called before Connect()
func (bos *BinanceOrderWebSocket) SetListenKey(listenKey string) error {
	if listenKey == "" {
		return fmt.Errorf("listen key cannot be empty")
	}

	bos.mu.Lock()
	defer bos.mu.Unlock()

	bos.listenKey = listenKey
	logger.Infof("✓ Binance listen key set (key length: %d)", len(listenKey))
	return nil
}

// Connect establishes the WebSocket connection using the listen key
func (bos *BinanceOrderWebSocket) Connect() error {
	bos.mu.Lock()
	defer bos.mu.Unlock()

	if bos.listenKey == "" {
		return fmt.Errorf("listen key not set - call SetListenKey() first")
	}

	if bos.isConnected {
		return nil
	}

	// User data stream URL format: ws://{baseurl}/ws/{listenKey}
	url := fmt.Sprintf("%s/%s", bos.baseURL, bos.listenKey)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Binance order stream: %w", err)
	}

	bos.conn = conn
	bos.isConnected = true
	bos.lastActivity = time.Now()

	logger.Infof("✓ Binance order stream connected (testnet=%v)", bos.testnet)

	// Start message reader
	go bos.readMessages()

	// Start heartbeat (keep-alive mechanism)
	go bos.heartbeat()

	return nil
}

// Disconnect closes the WebSocket connection
func (bos *BinanceOrderWebSocket) Disconnect() error {
	bos.mu.Lock()
	defer bos.mu.Unlock()

	if !bos.isConnected {
		return nil
	}

	bos.isConnected = false
	close(bos.stopCh)

	if bos.conn != nil {
		bos.conn.Close()
	}

	if bos.heartbeatTicker != nil {
		bos.heartbeatTicker.Stop()
	}

	logger.Info("✓ Binance order stream disconnected")
	return nil
}

// IsConnected returns whether the connection is active
func (bos *BinanceOrderWebSocket) IsConnected() bool {
	bos.mu.RLock()
	defer bos.mu.RUnlock()

	return bos.isConnected && bos.conn != nil
}

// GetOrderUpdateChannel returns the channel for order updates
func (bos *BinanceOrderWebSocket) GetOrderUpdateChannel() <-chan OrderUpdate {
	return bos.orderUpdateChannel
}

// Reconnect attempts to reconnect the WebSocket
func (bos *BinanceOrderWebSocket) Reconnect() error {
	bos.Disconnect()
	time.Sleep(bos.reconnectDelay)
	return bos.Connect()
}

// readMessages reads messages from the WebSocket and processes them
func (bos *BinanceOrderWebSocket) readMessages() {
	defer func() {
		bos.mu.Lock()
		bos.isConnected = false
		bos.mu.Unlock()
	}()

	for {
		select {
		case <-bos.stopCh:
			return
		default:
		}

		bos.mu.RLock()
		conn := bos.conn
		bos.mu.RUnlock()

		if conn == nil {
			return
		}

		var data []byte
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warnf("Binance order stream error: %v", err)
			}
			return
		}

		bos.mu.Lock()
		bos.lastActivity = time.Now()
		bos.mu.Unlock()

		// Parse the message
		var update BinanceAccountUpdate
		if err := json.Unmarshal(data, &update); err != nil {
			logger.Warnf("Failed to parse Binance order update: %v", err)
			continue
		}

		// Process order updates
		if update.EventType == "ORDER_TRADE_UPDATE" {
			bos.processOrderUpdate(update)
		}
	}
}

// processOrderUpdate converts Binance update to OrderUpdate and sends it
func (bos *BinanceOrderWebSocket) processOrderUpdate(update BinanceAccountUpdate) {
	if len(update.Orders) == 0 {
		return
	}

	// Process each order in the update
	for _, order := range update.Orders {
		orderUpdate := OrderUpdate{
			Symbol:             order.Symbol,
			OrderID:            fmt.Sprintf("%d", order.OrderID),
			ClientOrderID:      order.ClientOrderID,
			Side:               order.Side,
			PositionSide:       order.PositionSide,
			OrderType:          order.OrderType,
			TimeInForce:        order.TimeInForce,
			OriginalQuantity:   order.OriginalQuantity,
			ExecutedQuantity:   order.ExecutedQuantity,
			CumulativeQuoteQty: order.CumulativeQuoteAsset,
			Status:             order.Status,
			ExecutionType:      order.Status, // In Binance, status is used for execution type
			OrderPrice:         order.Price,
			AveragePrice:       order.AvgPrice,
			Timestamp:          time.Now(),
		}

		// Send to channel
		select {
		case bos.orderUpdateChannel <- orderUpdate:
		case <-bos.stopCh:
			return
		default:
			// Channel full, drop message
			logger.Warnf("Order update channel full, dropping order %s", order.ClientOrderID)
		}
	}
}

// heartbeat sends periodic ping to keep connection alive
func (bos *BinanceOrderWebSocket) heartbeat() {
	ticker := time.NewTicker(bos.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bos.mu.Lock()
			conn := bos.conn
			bos.mu.Unlock()

			if conn != nil {
				if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					logger.Warnf("Failed to send ping to Binance order stream: %v", err)
					return
				}
			}
		case <-bos.stopCh:
			return
		}
	}
}

// RefreshListenKey extends the life of the listen key (required every 60 minutes)
// The REST API call to refresh the listen key must be made separately
func (bos *BinanceOrderWebSocket) RefreshListenKey() error {
	bos.mu.RLock()
	key := bos.listenKey
	bos.mu.RUnlock()

	if key == "" {
		return fmt.Errorf("no listen key to refresh")
	}

	// Note: The actual REST API call to refresh the listen key should be made
	// by the caller (e.g., AutoTrader) using the Binance REST API
	// POST /fapi/v1/listenKey with the current listen key
	logger.Infof("⏱️ Binance listen key refresh needed (refresh in background)")
	return nil
}

// GetListenKey returns the current listen key
func (bos *BinanceOrderWebSocket) GetListenKey() string {
	bos.mu.RLock()
	defer bos.mu.RUnlock()

	return bos.listenKey
}
