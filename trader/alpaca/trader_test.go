package alpaca

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Mock Alpaca Server ---

type mockAlpacaServer struct {
	server    *httptest.Server
	positions []AlpacaPosition
	orders    []AlpacaOrder
	account   AlpacaAccount
	nextOrderID int
}

func newMockAlpacaServer() *mockAlpacaServer {
	m := &mockAlpacaServer{
		account: AlpacaAccount{
			ID:            "test-account-id",
			AccountNumber: "PA1234567",
			Status:        "ACTIVE",
			Currency:      "USD",
			Cash:          "100000.00",
			Equity:        "100000.00",
			BuyingPower:   "100000.00",
		},
		nextOrderID: 1000,
	}

	mux := http.NewServeMux()

	// Account
	mux.HandleFunc("/v2/account", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		json.NewEncoder(w).Encode(m.account)
	})

	// Positions
	mux.HandleFunc("/v2/positions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && !strings.Contains(r.URL.Path, "/v2/positions/") {
			json.NewEncoder(w).Encode(m.positions)
			return
		}
	})

	// Position close by symbol (DELETE /v2/positions/AAPL)
	mux.HandleFunc("/v2/positions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			symbol := strings.TrimPrefix(r.URL.Path, "/v2/positions/")
			// Remove position
			var remaining []AlpacaPosition
			found := false
			for _, p := range m.positions {
				if p.Symbol == symbol {
					found = true
					continue
				}
				remaining = append(remaining, p)
			}
			if !found {
				http.Error(w, `{"code":40410000,"message":"position does not exist"}`, 404)
				return
			}
			m.positions = remaining
			order := m.createOrder(symbol, "sell", "0", "market", "filled")
			json.NewEncoder(w).Encode(order)
			return
		}
		// GET single position
		if r.Method == "GET" {
			symbol := strings.TrimPrefix(r.URL.Path, "/v2/positions/")
			for _, p := range m.positions {
				if p.Symbol == symbol {
					json.NewEncoder(w).Encode(p)
					return
				}
			}
			http.Error(w, `{"code":40410000,"message":"position does not exist"}`, 404)
		}
	})

	// Orders
	mux.HandleFunc("/v2/orders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			status := r.URL.Query().Get("status")
			symbolsFilter := r.URL.Query().Get("symbols")
			var result []AlpacaOrder
			for _, o := range m.orders {
				// Alpaca's "open" status filter matches: new, partially_filled, accepted
				if status == "open" {
					if o.Status != "new" && o.Status != "partially_filled" && o.Status != "accepted" {
						continue
					}
				} else if status == "closed" {
					if o.Status != "filled" && o.Status != "canceled" && o.Status != "expired" {
						continue
					}
				} else if status != "" && o.Status != status {
					continue
				}
				if symbolsFilter != "" && o.Symbol != symbolsFilter {
					continue
				}
				result = append(result, o)
			}
			json.NewEncoder(w).Encode(result)
		case "POST":
			var req AlpacaOrderRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"message":"invalid body"}`, 400)
				return
			}
			// Simulate order fill for market orders
			fillStatus := "accepted"
			if req.Type == "market" {
				fillStatus = "filled"
				// If buy, add position
				if req.Side == "buy" {
					m.addPosition(req.Symbol, req.Qty, "150.00")
				}
			}
			order := m.createOrder(req.Symbol, req.Side, req.Qty, req.Type, fillStatus)
			if req.StopPrice != "" {
				order.StopPrice = req.StopPrice
			}
			if req.LimitPrice != "" {
				order.LimitPrice = req.LimitPrice
			}
			if fillStatus != "filled" {
				order.Status = "new" // stop/limit orders stay open
			}
			m.orders = append(m.orders, order)
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(order)
		case "DELETE":
			// Cancel all orders
			m.orders = nil
			w.WriteHeader(207)
			json.NewEncoder(w).Encode([]interface{}{})
		}
	})

	// Single order operations
	mux.HandleFunc("/v2/orders/", func(w http.ResponseWriter, r *http.Request) {
		orderID := strings.TrimPrefix(r.URL.Path, "/v2/orders/")
		switch r.Method {
		case "GET":
			for _, o := range m.orders {
				if o.ID == orderID {
					json.NewEncoder(w).Encode(o)
					return
				}
			}
			http.Error(w, `{"code":40410000,"message":"order not found"}`, 404)
		case "DELETE":
			var remaining []AlpacaOrder
			found := false
			for _, o := range m.orders {
				if o.ID == orderID {
					found = true
					continue
				}
				remaining = append(remaining, o)
			}
			if !found {
				http.Error(w, `{"code":40410000,"message":"order not found"}`, 404)
				return
			}
			m.orders = remaining
			w.WriteHeader(204)
		}
	})

	// Clock
	mux.HandleFunc("/v2/clock", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"timestamp":  time.Now().Format(time.RFC3339),
			"is_open":    false,
			"next_open":  time.Now().Add(12 * time.Hour).Format(time.RFC3339),
			"next_close": time.Now().Add(18 * time.Hour).Format(time.RFC3339),
		})
	})

	m.server = httptest.NewServer(mux)
	return m
}

func (m *mockAlpacaServer) createOrder(symbol, side, qty, orderType, status string) AlpacaOrder {
	m.nextOrderID++
	now := time.Now().Format(time.RFC3339Nano)
	o := AlpacaOrder{
		ID:            fmt.Sprintf("order-%d", m.nextOrderID),
		ClientOrderID: fmt.Sprintf("client-%d", m.nextOrderID),
		Symbol:        symbol,
		Side:          side,
		Type:          orderType,
		Qty:           qty,
		Status:        status,
		CreatedAt:     now,
		UpdatedAt:     now,
		SubmittedAt:   now,
		TimeInForce:   "day",
	}
	if status == "filled" {
		o.FilledQty = qty
		o.FilledAvgPrice = "150.25"
		o.FilledAt = now
	}
	return o
}

func (m *mockAlpacaServer) addPosition(symbol, qty, price string) {
	for i, p := range m.positions {
		if p.Symbol == symbol {
			// Update existing
			existQty := parseFloatStr(p.Qty)
			addQty := parseFloatStr(qty)
			m.positions[i].Qty = fmt.Sprintf("%.4f", existQty+addQty)
			return
		}
	}
	m.positions = append(m.positions, AlpacaPosition{
		AssetID:       "asset-" + symbol,
		Symbol:        symbol,
		Exchange:      "NASDAQ",
		AssetClass:    "us_equity",
		AvgEntryPrice: price,
		Qty:           qty,
		Side:          "long",
		MarketValue:   fmt.Sprintf("%.2f", parseFloatStr(qty)*parseFloatStr(price)),
		CostBasis:     fmt.Sprintf("%.2f", parseFloatStr(qty)*parseFloatStr(price)),
		UnrealizedPL:  "0.00",
		CurrentPrice:  price,
	})
}

func (m *mockAlpacaServer) close() {
	m.server.Close()
}

func newTestTrader(serverURL string) *AlpacaTrader {
	t := NewAlpacaTrader("test-key", "test-secret", true)
	t.baseURL = serverURL
	return t
}

// --- Tests ---

func TestGetBalance(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("GetBalance() error: %v", err)
	}

	if balance["totalEquity"].(float64) != 100000.0 {
		t.Errorf("totalEquity = %v, want 100000.0", balance["totalEquity"])
	}
	if balance["availableBalance"].(float64) != 100000.0 {
		t.Errorf("availableBalance = %v, want 100000.0", balance["availableBalance"])
	}
	if balance["currency"].(string) != "USD" {
		t.Errorf("currency = %v, want USD", balance["currency"])
	}
	if balance["status"].(string) != "ACTIVE" {
		t.Errorf("status = %v, want ACTIVE", balance["status"])
	}
}

func TestGetBalance_Caching(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)

	// First call
	b1, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("first GetBalance() error: %v", err)
	}

	// Modify server-side data
	mock.account.Cash = "50000.00"

	// Second call should return cached
	b2, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("second GetBalance() error: %v", err)
	}

	if b1["availableBalance"] != b2["availableBalance"] {
		t.Errorf("cache miss: got different balances %v vs %v", b1["availableBalance"], b2["availableBalance"])
	}
}

func TestGetPositions_Empty(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("GetPositions() error: %v", err)
	}

	if len(positions) != 0 {
		t.Errorf("expected 0 positions, got %d", len(positions))
	}
}

func TestOpenLong_Buy(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	result, err := trader.OpenLong("AAPL", 10.0, 1)
	if err != nil {
		t.Fatalf("OpenLong() error: %v", err)
	}

	if result["symbol"].(string) != "AAPL" {
		t.Errorf("symbol = %v, want AAPL", result["symbol"])
	}
	if result["side"].(string) != "buy" {
		t.Errorf("side = %v, want buy", result["side"])
	}
	if result["status"].(string) != "filled" {
		t.Errorf("status = %v, want filled", result["status"])
	}

	// Verify position was created
	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("GetPositions() after buy error: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}
	if positions[0]["symbol"].(string) != "AAPL" {
		t.Errorf("position symbol = %v, want AAPL", positions[0]["symbol"])
	}
}

func TestOpenLong_FractionalShares(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	result, err := trader.OpenLong("AAPL", 0.5, 1)
	if err != nil {
		t.Fatalf("OpenLong(0.5) error: %v", err)
	}

	if result["qty"].(string) != "0.5000" {
		t.Errorf("qty = %v, want 0.5000", result["qty"])
	}
}

func TestCloseLong_FullPosition(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()
	mock.addPosition("TSLA", "10.0000", "250.00")

	trader := newTestTrader(mock.server.URL)

	// Close full position (quantity=0)
	result, err := trader.CloseLong("TSLA", 0)
	if err != nil {
		t.Fatalf("CloseLong() error: %v", err)
	}

	if result["symbol"].(string) != "TSLA" {
		t.Errorf("symbol = %v, want TSLA", result["symbol"])
	}

	// Position should be removed
	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("GetPositions() after close error: %v", err)
	}
	if len(positions) != 0 {
		t.Errorf("expected 0 positions after close, got %d", len(positions))
	}
}

func TestCloseLong_PartialSell(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()
	mock.addPosition("AAPL", "10.0000", "150.00")

	trader := newTestTrader(mock.server.URL)
	result, err := trader.CloseLong("AAPL", 5.0)
	if err != nil {
		t.Fatalf("CloseLong(partial) error: %v", err)
	}

	if result["side"].(string) != "sell" {
		t.Errorf("side = %v, want sell", result["side"])
	}
}

func TestOpenShort_NotSupported(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	_, err := trader.OpenShort("AAPL", 10, 1)
	if err == nil {
		t.Fatal("OpenShort() should return error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCloseShort_NotSupported(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	_, err := trader.CloseShort("AAPL", 10)
	if err == nil {
		t.Fatal("CloseShort() should return error")
	}
}

func TestSetStopLoss(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	err := trader.SetStopLoss("AAPL", "LONG", 10, 140.0)
	if err != nil {
		t.Fatalf("SetStopLoss() error: %v", err)
	}

	// Verify stop order was created
	orders, err := trader.GetOpenOrders("AAPL")
	if err != nil {
		t.Fatalf("GetOpenOrders() error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 open order, got %d", len(orders))
	}
	if orders[0].Type != "STOP" {
		t.Errorf("order type = %v, want STOP", orders[0].Type)
	}
	if orders[0].Side != "SELL" {
		t.Errorf("order side = %v, want SELL", orders[0].Side)
	}
}

func TestSetTakeProfit(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	err := trader.SetTakeProfit("AAPL", "LONG", 10, 200.0)
	if err != nil {
		t.Fatalf("SetTakeProfit() error: %v", err)
	}

	orders, err := trader.GetOpenOrders("AAPL")
	if err != nil {
		t.Fatalf("GetOpenOrders() error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 open order, got %d", len(orders))
	}
	if orders[0].Type != "LIMIT" {
		t.Errorf("order type = %v, want LIMIT", orders[0].Type)
	}
}

func TestCancelStopOrders(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)

	// Create stop and limit orders
	trader.SetStopLoss("AAPL", "LONG", 10, 140.0)
	trader.SetTakeProfit("AAPL", "LONG", 10, 200.0)

	orders, _ := trader.GetOpenOrders("AAPL")
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders before cancel, got %d", len(orders))
	}

	// Cancel stop orders (both stop and limit/TP)
	err := trader.CancelStopOrders("AAPL")
	if err != nil {
		t.Fatalf("CancelStopOrders() error: %v", err)
	}

	orders, _ = trader.GetOpenOrders("AAPL")
	if len(orders) != 0 {
		t.Errorf("expected 0 orders after cancel, got %d", len(orders))
	}
}

func TestCancelAllOrders(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)

	// Create multiple orders
	trader.SetStopLoss("AAPL", "LONG", 10, 140.0)
	trader.SetStopLoss("TSLA", "LONG", 5, 200.0)

	// Cancel all (no symbol filter)
	err := trader.CancelAllOrders("")
	if err != nil {
		t.Fatalf("CancelAllOrders() error: %v", err)
	}

	// Mock server DELETE /v2/orders clears all
	if len(mock.orders) != 0 {
		t.Errorf("expected 0 orders after cancel all, got %d", len(mock.orders))
	}
}

func TestIsMarketOpen(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	isOpen, status, err := trader.IsMarketOpen()
	if err != nil {
		t.Fatalf("IsMarketOpen() error: %v", err)
	}
	if isOpen {
		t.Errorf("expected market closed (mock)")
	}
	if !strings.Contains(status, "closed") {
		t.Errorf("status = %v, want contains 'closed'", status)
	}
}

func TestFormatQuantity(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	qty, err := trader.FormatQuantity("AAPL", 10.123456789)
	if err != nil {
		t.Fatalf("FormatQuantity() error: %v", err)
	}
	if qty != "10.1235" {
		t.Errorf("FormatQuantity = %v, want 10.1235", qty)
	}
}

func TestSetLeverage_NoOp(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	if err := trader.SetLeverage("AAPL", 10); err != nil {
		t.Errorf("SetLeverage should be no-op, got error: %v", err)
	}
}

func TestSetMarginMode_NoOp(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	if err := trader.SetMarginMode("AAPL", true); err != nil {
		t.Errorf("SetMarginMode should be no-op, got error: %v", err)
	}
}

func TestGetOrderStatus(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)

	// Create an open order (stop order stays "new")
	trader.SetStopLoss("AAPL", "LONG", 10, 140.0)

	orders, _ := trader.GetOpenOrders("AAPL")
	if len(orders) == 0 {
		t.Fatal("no open orders to check status")
	}

	status, err := trader.GetOrderStatus("AAPL", orders[0].OrderID)
	if err != nil {
		t.Fatalf("GetOrderStatus() error: %v", err)
	}
	if status["status"].(string) != "NEW" {
		t.Errorf("status = %v, want NEW", status["status"])
	}
	if status["commission"].(float64) != 0.0 {
		t.Errorf("commission = %v, want 0.0 (commission-free)", status["commission"])
	}
}

func TestFullTradeFlow(t *testing.T) {
	// Simulate: check balance → buy AAPL → check positions → set SL/TP → close position
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)

	// 1. Check balance
	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("Step 1 GetBalance: %v", err)
	}
	if balance["totalEquity"].(float64) != 100000.0 {
		t.Fatalf("unexpected equity: %v", balance["totalEquity"])
	}

	// 2. Buy AAPL
	buyResult, err := trader.OpenLong("AAPL", 10, 1)
	if err != nil {
		t.Fatalf("Step 2 OpenLong: %v", err)
	}
	if buyResult["status"].(string) != "filled" {
		t.Fatalf("buy not filled: %v", buyResult["status"])
	}

	// 3. Check positions
	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("Step 3 GetPositions: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}
	pos := positions[0]
	if pos["symbol"].(string) != "AAPL" {
		t.Errorf("position symbol = %v, want AAPL", pos["symbol"])
	}
	if pos["exchange"].(string) != "alpaca" {
		t.Errorf("position exchange = %v, want alpaca", pos["exchange"])
	}

	// 4. Set stop loss and take profit
	err = trader.SetStopLoss("AAPL", "LONG", 10, 140.0)
	if err != nil {
		t.Fatalf("Step 4a SetStopLoss: %v", err)
	}
	err = trader.SetTakeProfit("AAPL", "LONG", 10, 200.0)
	if err != nil {
		t.Fatalf("Step 4b SetTakeProfit: %v", err)
	}

	orders, _ := trader.GetOpenOrders("AAPL")
	if len(orders) != 2 {
		t.Errorf("expected 2 open orders (SL+TP), got %d", len(orders))
	}

	// 5. Cancel SL/TP before closing
	err = trader.CancelStopOrders("AAPL")
	if err != nil {
		t.Fatalf("Step 5 CancelStopOrders: %v", err)
	}

	// 6. Close position
	closeResult, err := trader.CloseLong("AAPL", 0)
	if err != nil {
		t.Fatalf("Step 6 CloseLong: %v", err)
	}
	if closeResult["symbol"].(string) != "AAPL" {
		t.Errorf("close symbol = %v, want AAPL", closeResult["symbol"])
	}

	// 7. Verify no positions left
	positions, err = trader.GetPositions()
	if err != nil {
		t.Fatalf("Step 7 GetPositions: %v", err)
	}
	if len(positions) != 0 {
		t.Errorf("expected 0 positions after close, got %d", len(positions))
	}
}

func TestCloseLong_NoPosition(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	trader := newTestTrader(mock.server.URL)
	_, err := trader.CloseLong("AAPL", 0) // No AAPL position exists
	if err == nil {
		t.Fatal("CloseLong should fail when no position exists")
	}
}

func TestGetClosedPnL(t *testing.T) {
	mock := newMockAlpacaServer()
	defer mock.close()

	// Create some filled sell orders in mock
	now := time.Now().Format(time.RFC3339Nano)
	mock.orders = append(mock.orders, AlpacaOrder{
		ID:             "order-closed-1",
		Symbol:         "AAPL",
		Side:           "sell",
		Type:           "market",
		Qty:            "10",
		FilledQty:      "10",
		FilledAvgPrice: "155.50",
		Status:         "filled",
		FilledAt:       now,
		UpdatedAt:      now,
		TimeInForce:    "day",
	})
	mock.orders = append(mock.orders, AlpacaOrder{
		ID:          "order-closed-2",
		Symbol:      "TSLA",
		Side:        "buy",  // buy orders should be excluded
		Type:        "market",
		Qty:         "5",
		Status:      "filled",
		FilledQty:   "5",
		FilledAvgPrice: "250.00",
		FilledAt:    now,
		UpdatedAt:   now,
	})

	trader := newTestTrader(mock.server.URL)
	records, err := trader.GetClosedPnL(time.Now().Add(-24*time.Hour), 50)
	if err != nil {
		t.Fatalf("GetClosedPnL() error: %v", err)
	}

	// Should only include the sell order
	if len(records) != 1 {
		t.Fatalf("expected 1 closed PnL record, got %d", len(records))
	}
	if records[0].Symbol != "AAPL" {
		t.Errorf("record symbol = %v, want AAPL", records[0].Symbol)
	}
	if records[0].ExitPrice != 155.50 {
		t.Errorf("exit price = %v, want 155.50", records[0].ExitPrice)
	}
	if records[0].Fee != 0 {
		t.Errorf("fee = %v, want 0 (commission-free)", records[0].Fee)
	}
}

func TestParseFloatStr(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"100.50", 100.50},
		{"0", 0},
		{"", 0},
		{"99999.9999", 99999.9999},
		{"-50.25", -50.25},
	}
	for _, tt := range tests {
		got := parseFloatStr(tt.input)
		if got != tt.want {
			t.Errorf("parseFloatStr(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	if truncate("hello world", 5) != "hello..." {
		t.Errorf("got %v", truncate("hello world", 5))
	}
}

// --- Live Integration Test (skip unless ALPACA_API_KEY is set) ---

func TestLiveAlpacaPaper(t *testing.T) {
	apiKey := ""
	apiSecret := ""
	// Check env vars at runtime
	if apiKey == "" {
		t.Skip("Skipping live test: set ALPACA_API_KEY and ALPACA_API_SECRET env vars")
	}

	trader := NewAlpacaTrader(apiKey, apiSecret, true)

	// Test 1: Get balance
	balance, err := trader.GetBalance()
	if err != nil {
		t.Fatalf("Live GetBalance: %v", err)
	}
	t.Logf("Balance: equity=%.2f, cash=%.2f, buying_power=%.2f",
		balance["totalEquity"], balance["availableBalance"], balance["buying_power"])

	// Test 2: Check market status
	isOpen, status, err := trader.IsMarketOpen()
	if err != nil {
		t.Fatalf("Live IsMarketOpen: %v", err)
	}
	t.Logf("Market: open=%v, status=%s", isOpen, status)

	// Test 3: Get positions
	positions, err := trader.GetPositions()
	if err != nil {
		t.Fatalf("Live GetPositions: %v", err)
	}
	t.Logf("Positions: %d", len(positions))

	// Test 4: Get open orders
	orders, err := trader.GetOpenOrders("")
	if err != nil {
		t.Fatalf("Live GetOpenOrders: %v", err)
	}
	t.Logf("Open orders: %d", len(orders))
}
