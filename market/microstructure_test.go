package market

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchOrderBookDepth(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request parameters
		symbol := r.URL.Query().Get("symbol")

		if symbol == "" {
			http.Error(w, "Missing symbol", http.StatusBadRequest)
			return
		}

		// Mock response
		response := map[string]interface{}{
			"lastUpdateId": 123456789,
			"bids": [][]interface{}{
				{"50000.00", "1.5"},
				{"49990.00", "2.3"},
				{"49980.00", "3.1"},
			},
			"asks": [][]interface{}{
				{"50010.00", "1.8"},
				{"50020.00", "2.1"},
				{"50030.00", "2.9"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	analyzer := NewMarketMicrostructureAnalyzer()
	analyzer.baseURL = server.URL

	tests := []struct {
		name      string
		symbol    string
		limit     int
		wantError bool
	}{
		{
			name:      "Valid request with 20 levels",
			symbol:    "BTCUSDT",
			limit:     20,
			wantError: false,
		},
		{
			name:      "Valid request with 50 levels",
			symbol:    "ETHUSDT",
			limit:     50,
			wantError: false,
		},
		{
			name:      "Valid request with default limit",
			symbol:    "BTCUSDT",
			limit:     0,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depth, err := analyzer.FetchOrderBookDepth(tt.symbol, tt.limit)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if depth == nil {
				t.Fatal("Expected depth data but got nil")
			}

			if depth.Symbol != tt.symbol {
				t.Errorf("Expected symbol %s, got %s", tt.symbol, depth.Symbol)
			}

			if len(depth.Bids) != 3 {
				t.Errorf("Expected 3 bids, got %d", len(depth.Bids))
			}

			if len(depth.Asks) != 3 {
				t.Errorf("Expected 3 asks, got %d", len(depth.Asks))
			}

			// Verify bid/ask ordering and values
			if len(depth.Bids) > 0 && depth.Bids[0].Price != 50000.00 {
				t.Errorf("Expected best bid 50000.00, got %f", depth.Bids[0].Price)
			}

			if len(depth.Asks) > 0 && depth.Asks[0].Price != 50010.00 {
				t.Errorf("Expected best ask 50010.00, got %f", depth.Asks[0].Price)
			}
		})
	}
}

func TestAnalyzeMarketMicrostructure(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()

	// Create mock order book depth
	depth := &OrderBookDepth{
		Symbol:    "BTCUSDT",
		Timestamp: time.Now(),
		Bids: []PriceLevel{
			{Price: 50000, Quantity: 2.5},
			{Price: 49990, Quantity: 3.0},
			{Price: 49980, Quantity: 1.5},
			{Price: 49970, Quantity: 4.2},
			{Price: 49960, Quantity: 2.0},
		},
		Asks: []PriceLevel{
			{Price: 50010, Quantity: 2.0},
			{Price: 50020, Quantity: 2.5},
			{Price: 50030, Quantity: 1.8},
			{Price: 50040, Quantity: 3.5},
			{Price: 50050, Quantity: 2.2},
		},
	}

	// Create mock klines for VWAP calculation
	klines := []Kline{
		{High: 50100, Low: 49900, Close: 50000, Volume: 100},
		{High: 50200, Low: 50000, Close: 50100, Volume: 150},
		{High: 50150, Low: 49950, Close: 50050, Volume: 120},
	}

	currentPrice := 50005.0

	ms, err := analyzer.AnalyzeMarketMicrostructure("BTCUSDT", depth, currentPrice, klines)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if ms == nil {
		t.Fatal("Expected microstructure data but got nil")
	}

	// Test VWAP calculation
	if ms.VWAP == 0 {
		t.Error("Expected non-zero VWAP")
	}

	// Test bid-ask spread calculation
	if ms.BidAskSpread == 0 {
		t.Error("Expected non-zero bid-ask spread")
	}

	expectedSpread := ((50010.0 - 50000.0) / 50005.0) * 100
	if ms.BidAskSpread < expectedSpread*0.99 || ms.BidAskSpread > expectedSpread*1.01 {
		t.Errorf("Expected spread around %.4f%%, got %.4f%%", expectedSpread, ms.BidAskSpread)
	}

	// Test order book imbalance
	if ms.OrderBookImbalance < 0 || ms.OrderBookImbalance > 1 {
		t.Errorf("Order book imbalance should be between 0 and 1, got %.2f", ms.OrderBookImbalance)
	}

	// Test depth calculations
	if ms.BidDepth == 0 {
		t.Error("Expected non-zero bid depth")
	}

	if ms.AskDepth == 0 {
		t.Error("Expected non-zero ask depth")
	}

	// Total depth should match sum of quantities
	expectedBidDepth := 2.5 + 3.0 + 1.5 + 4.2 + 2.0
	if ms.BidDepth != expectedBidDepth {
		t.Errorf("Expected bid depth %.1f, got %.1f", expectedBidDepth, ms.BidDepth)
	}

	// Test cumulative volumes
	if len(ms.CumulativeBidVolume) == 0 {
		t.Error("Expected cumulative bid volume data")
	}

	if len(ms.CumulativeAskVolume) == 0 {
		t.Error("Expected cumulative ask volume data")
	}

	// Test details
	if ms.Details == nil {
		t.Error("Expected details map")
	}

	if _, exists := ms.Details["best_bid"]; !exists {
		t.Error("Expected best_bid in details")
	}

	if _, exists := ms.Details["imbalance_direction"]; !exists {
		t.Error("Expected imbalance_direction in details")
	}
}

func TestCalculateVWAP(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()

	tests := []struct {
		name     string
		klines   []Kline
		expected float64
	}{
		{
			name: "Single kline",
			klines: []Kline{
				{High: 100, Low: 90, Close: 95, Volume: 1000},
			},
			expected: 95.0, // (100+90+95)/3 = 95
		},
		{
			name: "Multiple klines - balanced",
			klines: []Kline{
				{High: 100, Low: 90, Close: 95, Volume: 1000},
				{High: 105, Low: 95, Close: 100, Volume: 1000},
			},
			expected: 97.5, // Average of typical prices weighted equally
		},
		{
			name: "Multiple klines - volume weighted",
			klines: []Kline{
				{High: 100, Low: 90, Close: 95, Volume: 1000},   // Typical: 95
				{High: 110, Low: 100, Close: 105, Volume: 3000}, // Typical: 105, 3x volume
			},
			expected: 102.5, // (95*1000 + 105*3000) / 4000 = 102.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vwap := analyzer.calculateVWAP("BTCUSDT", tt.klines)

			tolerance := 0.1
			if vwap < tt.expected-tolerance || vwap > tt.expected+tolerance {
				t.Errorf("Expected VWAP around %.2f, got %.2f", tt.expected, vwap)
			}
		})
	}
}

func TestDetectLargeOrders(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()
	analyzer.SetLargeOrderThreshold(50000) // $50k threshold

	depth := &OrderBookDepth{
		Symbol: "BTCUSDT",
		Bids: []PriceLevel{
			{Price: 50000, Quantity: 0.5}, // $25k - small
			{Price: 49990, Quantity: 2.0}, // $100k - large
			{Price: 49980, Quantity: 0.8}, // $40k - small
			{Price: 49970, Quantity: 5.0}, // $250k - large
		},
		Asks: []PriceLevel{
			{Price: 50010, Quantity: 0.6}, // $30k - small
			{Price: 50020, Quantity: 3.0}, // $150k - large
			{Price: 50030, Quantity: 0.9}, // $45k - small
		},
	}

	currentPrice := 50000.0

	count, volume := analyzer.detectLargeOrders(depth, currentPrice)

	// Should detect 3 large orders: 2.0 BTC, 5.0 BTC, 3.0 BTC
	if count < 3 {
		t.Errorf("Expected at least 3 large orders, got %d", count)
	}

	expectedMinVolume := 2.0 + 5.0 + 3.0
	if volume < expectedMinVolume {
		t.Errorf("Expected large order volume >= %.1f, got %.1f", expectedMinVolume, volume)
	}
}

func TestIdentifySupportLevels(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()

	bids := []PriceLevel{
		{Price: 50000, Quantity: 2.0},
		{Price: 49990, Quantity: 1.5},
		{Price: 49980, Quantity: 5.0}, // Strong support - local maximum
		{Price: 49970, Quantity: 2.0},
		{Price: 49960, Quantity: 1.8},
		{Price: 49950, Quantity: 4.5}, // Strong support - local maximum
		{Price: 49940, Quantity: 2.1},
	}

	midPrice := 50005.0

	supports := analyzer.identifySupportLevels(bids, midPrice)

	if len(supports) == 0 {
		t.Error("Expected to find support levels")
	}

	// Should identify 49980 and 49950 as significant support levels
	found49980 := false
	found49950 := false

	for _, support := range supports {
		if support == 49980 {
			found49980 = true
		}
		if support == 49950 {
			found49950 = true
		}
	}

	if !found49980 {
		t.Error("Expected to find support level at 49980")
	}

	if !found49950 {
		t.Error("Expected to find support level at 49950")
	}
}

func TestIdentifyResistanceLevels(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()

	asks := []PriceLevel{
		{Price: 50010, Quantity: 2.0},
		{Price: 50020, Quantity: 1.5},
		{Price: 50030, Quantity: 5.0}, // Strong resistance - local maximum
		{Price: 50040, Quantity: 2.0},
		{Price: 50050, Quantity: 1.8},
		{Price: 50060, Quantity: 4.5}, // Strong resistance - local maximum
		{Price: 50070, Quantity: 2.1},
	}

	midPrice := 50005.0

	resistances := analyzer.identifyResistanceLevels(asks, midPrice)

	if len(resistances) == 0 {
		t.Error("Expected to find resistance levels")
	}

	// Should identify 50030 and 50060 as significant resistance levels
	found50030 := false
	found50060 := false

	for _, resistance := range resistances {
		if resistance == 50030 {
			found50030 = true
		}
		if resistance == 50060 {
			found50060 = true
		}
	}

	if !found50030 {
		t.Error("Expected to find resistance level at 50030")
	}

	if !found50060 {
		t.Error("Expected to find resistance level at 50060")
	}
}

func TestGetImbalanceDirection(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()

	tests := []struct {
		name      string
		imbalance float64
		expected  string
	}{
		{name: "Strong buy", imbalance: 0.70, expected: "strong_buy"},
		{name: "Moderate buy", imbalance: 0.60, expected: "moderate_buy"},
		{name: "Balanced", imbalance: 0.50, expected: "balanced"},
		{name: "Moderate sell", imbalance: 0.40, expected: "moderate_sell"},
		{name: "Strong sell", imbalance: 0.30, expected: "strong_sell"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.getImbalanceDirection(tt.imbalance)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCumulativeVolume(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()

	levels := []PriceLevel{
		{Price: 50000, Quantity: 2.0},
		{Price: 49990, Quantity: 3.0},
		{Price: 49980, Quantity: 1.5},
	}

	midPrice := 50005.0

	cumulative := analyzer.calculateCumulativeVolume(levels, midPrice, false)

	if len(cumulative) != 3 {
		t.Errorf("Expected 3 cumulative levels, got %d", len(cumulative))
	}

	// Test cumulative sums
	if cumulative[0].CumulativeVolume != 2.0 {
		t.Errorf("Expected first cumulative volume 2.0, got %.1f", cumulative[0].CumulativeVolume)
	}

	if cumulative[1].CumulativeVolume != 5.0 {
		t.Errorf("Expected second cumulative volume 5.0, got %.1f", cumulative[1].CumulativeVolume)
	}

	if cumulative[2].CumulativeVolume != 6.5 {
		t.Errorf("Expected third cumulative volume 6.5, got %.1f", cumulative[2].CumulativeVolume)
	}

	// Test percentage from mid
	expectedPct := ((50000.0 - 50005.0) / 50005.0) * 100
	if cumulative[0].PercentageFromMid < expectedPct*0.99 || cumulative[0].PercentageFromMid > expectedPct*1.01 {
		t.Errorf("Expected percentage around %.4f%%, got %.4f%%", expectedPct, cumulative[0].PercentageFromMid)
	}
}

func TestVWAPHistory(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()

	klines := []Kline{
		{High: 100, Low: 90, Close: 95, Volume: 1000},
	}

	// Calculate VWAP multiple times
	for i := 0; i < 5; i++ {
		analyzer.calculateVWAP("BTCUSDT", klines)
		time.Sleep(10 * time.Millisecond)
	}

	history := analyzer.GetVWAPHistory("BTCUSDT")

	if len(history) != 5 {
		t.Errorf("Expected 5 VWAP history points, got %d", len(history))
	}

	// Verify all points have timestamps
	for i, point := range history {
		if point.Timestamp.IsZero() {
			t.Errorf("History point %d has zero timestamp", i)
		}
		if point.VWAP == 0 {
			t.Errorf("History point %d has zero VWAP", i)
		}
		if point.Volume == 0 {
			t.Errorf("History point %d has zero volume", i)
		}
	}
}

func TestSetLargeOrderThreshold(t *testing.T) {
	analyzer := NewMarketMicrostructureAnalyzer()

	// Default threshold
	if analyzer.largeOrderThreshold != 100000 {
		t.Errorf("Expected default threshold 100000, got %.0f", analyzer.largeOrderThreshold)
	}

	// Set custom threshold
	analyzer.SetLargeOrderThreshold(250000)

	if analyzer.largeOrderThreshold != 250000 {
		t.Errorf("Expected threshold 250000, got %.0f", analyzer.largeOrderThreshold)
	}
}

func BenchmarkAnalyzeMarketMicrostructure(b *testing.B) {
	analyzer := NewMarketMicrostructureAnalyzer()

	depth := &OrderBookDepth{
		Symbol: "BTCUSDT",
		Bids:   make([]PriceLevel, 100),
		Asks:   make([]PriceLevel, 100),
	}

	for i := 0; i < 100; i++ {
		depth.Bids[i] = PriceLevel{Price: 50000 - float64(i)*10, Quantity: 1.5}
		depth.Asks[i] = PriceLevel{Price: 50010 + float64(i)*10, Quantity: 1.5}
	}

	klines := []Kline{
		{High: 50100, Low: 49900, Close: 50000, Volume: 100},
		{High: 50200, Low: 50000, Close: 50100, Volume: 150},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeMarketMicrostructure("BTCUSDT", depth, 50005, klines)
	}
}
