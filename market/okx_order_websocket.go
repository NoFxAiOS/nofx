package market

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"nofx/logger"
	"sync"
	"time"
)

// OKXOrderWebSocket implements order update streaming for OKX
type OKXOrderWebSocket struct {
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
	passphrase         string
	authenticated      bool
}

// OKXOrderUpdate represents order updates from OKX
type OKXOrderUpdate struct {
	Arg struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Data []struct {
		InstID    string  `json:"instId"`
		OrdID     string  `json:"ordId"`
		ClOrdID   string  `json:"clOrdId"`
		Tag       string  `json:"tag"`
		Price     float64 `json:"px,string"`
		Sz        float64 `json:"sz,string"`
		OrdType   string  `json:"ordType"` // market limit post_only fok ioc
		Side      string  `json:"side"`    // buy sell
		PosSide   string  `json:"posSide"` // long short net
		Status    string  `json:"state"`   // live partially_filled filled canceled
		LeavesSz  float64 `json:"leavesSz,string"`
		AccFillSz float64 `json:"accFillSz,string"`
		AvgPx     float64 `json:"avgPx,string"`
		CumEx     float64 `json:"cumEx,string"`
		Fee       float64 `json:"fee,string"`
		FeeCcy    string  `json:"feeCcy"`
		RebateCcy string  `json:"rebateCcy"`
		CtType    string  `json:"ctType"` // linear inverse
		CTime     int64   `json:"cTime,string"`
		UTime     int64   `json:"uTime,string"`
		TpSz      float64 `json:"tpSz,string"`
		SlSz      float64 `json:"slSz,string"`
		TpPx      float64 `json:"tpPx,string"`
		SlPx      float64 `json:"slPx,string"`
		TriggerPx float64 `json:"triggerPx,string"`
		OrdPx     float64 `json:"ordPx,string"`
	} `json:"data"`
}

// NewOKXOrderWebSocket creates a new OKX order WebSocket client
func NewOKXOrderWebSocket(testnet bool, apiKey, apiSecret, passphrase string) *OKXOrderWebSocket {
	url := "wss://ws.okx.com:8443/ws/v5/private"
	if testnet {
		url = "wss://wspap.okx.com:8443/ws/v5/private"
	}

	return &OKXOrderWebSocket{
		url:                url,
		isConnected:        false,
		orderUpdateChannel: make(chan OrderUpdate, 100),
		stopCh:             make(chan struct{}),
		reconnectAttempts:  5,
		reconnectDelay:     5 * time.Second,
		testnet:            testnet,
		apiKey:             apiKey,
		apiSecret:          apiSecret,
		passphrase:         passphrase,
	}
}

// Connect establishes the WebSocket connection with authentication
func (oow *OKXOrderWebSocket) Connect() error {
	oow.mu.Lock()
	defer oow.mu.Unlock()

	if oow.isConnected {
		return nil
	}

	if oow.apiKey == "" || oow.apiSecret == "" || oow.passphrase == "" {
		return fmt.Errorf("API key, secret, and passphrase required for OKX order stream")
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(oow.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to OKX order stream: %w", err)
	}

	oow.conn = conn
	oow.isConnected = true
	oow.lastActivity = time.Now()

	// Start message reader
	go oow.readMessages()

	// Authenticate
	if err := oow.authenticate(); err != nil {
		return fmt.Errorf("failed to authenticate with OKX: %w", err)
	}

	logger.Infof("✓ OKX order stream connected and authenticated (testnet=%v)", oow.testnet)

	// Start heartbeat
	go oow.heartbeat()

	return nil
}

// authenticate sends authentication message to OKX
func (oow *OKXOrderWebSocket) authenticate() error {
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// Create signature: HmacSHA256(timestamp + "GET" + "/users/self/verify", secretKey)
	message := timestamp + "GET" + "/users/self/verify"
	h := hmac.New(sha256.New, []byte(oow.apiSecret))
	h.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	authMsg := map[string]interface{}{
		"op": "login",
		"args": []map[string]string{
			{
				"apiKey":     oow.apiKey,
				"passphrase": oow.passphrase,
				"timestamp":  timestamp,
				"sign":       signature,
			},
		},
	}

	if err := oow.conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("failed to send auth message: %w", err)
	}

	logger.Info("OKX authentication message sent")
	return nil
}

// Disconnect closes the WebSocket connection
func (oow *OKXOrderWebSocket) Disconnect() error {
	oow.mu.Lock()
	defer oow.mu.Unlock()

	if !oow.isConnected {
		return nil
	}

	oow.isConnected = false
	close(oow.stopCh)

	if oow.conn != nil {
		oow.conn.Close()
	}

	if oow.heartbeatTicker != nil {
		oow.heartbeatTicker.Stop()
	}

	logger.Info("✓ OKX order stream disconnected")
	return nil
}

// IsConnected returns whether the connection is active
func (oow *OKXOrderWebSocket) IsConnected() bool {
	oow.mu.RLock()
	defer oow.mu.RUnlock()

	return oow.isConnected && oow.conn != nil
}

// GetOrderUpdateChannel returns the channel for order updates
func (oow *OKXOrderWebSocket) GetOrderUpdateChannel() <-chan OrderUpdate {
	return oow.orderUpdateChannel
}

// Reconnect attempts to reconnect the WebSocket
func (oow *OKXOrderWebSocket) Reconnect() error {
	oow.Disconnect()
	time.Sleep(oow.reconnectDelay)
	return oow.Connect()
}

// readMessages reads messages from the WebSocket
func (oow *OKXOrderWebSocket) readMessages() {
	defer func() {
		oow.mu.Lock()
		oow.isConnected = false
		oow.mu.Unlock()
	}()

	for {
		select {
		case <-oow.stopCh:
			return
		default:
		}

		oow.mu.RLock()
		conn := oow.conn
		oow.mu.RUnlock()

		if conn == nil {
			return
		}

		var data []byte
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warnf("OKX order stream error: %v", err)
			}
			return
		}

		oow.mu.Lock()
		oow.lastActivity = time.Now()
		oow.mu.Unlock()

		// Parse the message
		var update OKXOrderUpdate
		if err := json.Unmarshal(data, &update); err != nil {
			// Could be a system message or auth response
			continue
		}

		// Process order updates
		if update.Arg.Channel == "orders" {
			oow.processOrderUpdate(update)
		}
	}
}

// processOrderUpdate converts OKX update to OrderUpdate
func (oow *OKXOrderWebSocket) processOrderUpdate(update OKXOrderUpdate) {
	if len(update.Data) == 0 {
		return
	}

	for _, order := range update.Data {
		orderUpdate := OrderUpdate{
			Symbol:             order.InstID,
			OrderID:            order.OrdID,
			ClientOrderID:      order.ClOrdID,
			Side:               order.Side,
			PositionSide:       order.PosSide,
			OrderType:          order.OrdType,
			OriginalQuantity:   order.Sz,
			ExecutedQuantity:   order.AccFillSz,
			CumulativeQuoteQty: order.CumEx,
			Status:             order.Status,
			ExecutionType:      order.Status,
			OrderPrice:         order.Price,
			AveragePrice:       order.AvgPx,
			Timestamp:          time.Now(),
		}

		select {
		case oow.orderUpdateChannel <- orderUpdate:
		case <-oow.stopCh:
			return
		default:
			logger.Warnf("Order update channel full, dropping order %s", order.ClOrdID)
		}
	}
}

// heartbeat sends periodic ping
func (oow *OKXOrderWebSocket) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			oow.mu.Lock()
			conn := oow.conn
			oow.mu.Unlock()

			if conn != nil {
				if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					logger.Warnf("Failed to send ping to OKX: %v", err)
					return
				}
			}
		case <-oow.stopCh:
			return
		}
	}
}

// SubscribeOrders subscribes to order updates for a specific instrument
func (oow *OKXOrderWebSocket) SubscribeOrders(instID string) error {
	oow.mu.Lock()
	defer oow.mu.Unlock()

	if !oow.isConnected {
		return fmt.Errorf("not connected")
	}

	subMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{
				"channel": "orders",
				"instId":  instID,
			},
		},
	}

	if err := oow.conn.WriteJSON(subMsg); err != nil {
		return fmt.Errorf("failed to subscribe to orders: %w", err)
	}

	logger.Infof("✓ Subscribed to OKX orders for %s", instID)
	return nil
}
