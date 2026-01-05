package market

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

// OrderBookDepth represents order book depth data from exchange
type OrderBookDepth struct {
	Symbol       string       `json:"symbol"`
	Timestamp    time.Time    `json:"timestamp"`
	Bids         []PriceLevel `json:"bids"` // Buy orders: [price, quantity]
	Asks         []PriceLevel `json:"asks"` // Sell orders: [price, quantity]
	LastUpdateID int64        `json:"lastUpdateId"`
}

// PriceLevel represents a single price level in the order book
type PriceLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// MarketMicrostructure contains advanced market microstructure data
type MarketMicrostructure struct {
	Symbol              string                 `json:"symbol"`
	Timestamp           time.Time              `json:"timestamp"`
	CurrentPrice        float64                `json:"current_price"`
	VWAP                float64                `json:"vwap"`                 // Volume-weighted average price
	VWAPDeviation       float64                `json:"vwap_deviation"`       // Current price vs VWAP (%)
	BidAskSpread        float64                `json:"bid_ask_spread"`       // Spread in %
	BidAskSpreadBps     float64                `json:"bid_ask_spread_bps"`   // Spread in basis points
	OrderBookImbalance  float64                `json:"order_book_imbalance"` // 0-1, 0.5=balanced
	BidDepth            float64                `json:"bid_depth"`            // Total bid volume (top 10 levels)
	AskDepth            float64                `json:"ask_depth"`            // Total ask volume (top 10 levels)
	LargeOrderCount     int                    `json:"large_order_count"`    // Count of large orders detected
	LargeOrderVolume    float64                `json:"large_order_volume"`   // Total volume of large orders
	CumulativeBidVolume []CumulativeLevel      `json:"cumulative_bid_volume"`
	CumulativeAskVolume []CumulativeLevel      `json:"cumulative_ask_volume"`
	SupportLevels       []float64              `json:"support_levels"`    // Identified support price levels
	ResistanceLevels    []float64              `json:"resistance_levels"` // Identified resistance price levels
	Details             map[string]interface{} `json:"details"`
}

// CumulativeLevel represents cumulative volume at a price level
type CumulativeLevel struct {
	Price             float64 `json:"price"`
	CumulativeVolume  float64 `json:"cumulative_volume"`
	PercentageFromMid float64 `json:"percentage_from_mid"` // Distance from mid price in %
}

// MarketMicrostructureAnalyzer analyzes order book depth and calculates microstructure metrics
type MarketMicrostructureAnalyzer struct {
	client              *http.Client
	baseURL             string
	mu                  sync.RWMutex
	vwapHistory         map[string][]VWAPDataPoint // symbol -> VWAP history
	largeOrderThreshold float64                    // Threshold for large order detection (in base currency)
}

// VWAPDataPoint represents a single VWAP calculation point
type VWAPDataPoint struct {
	Timestamp time.Time
	VWAP      float64
	Volume    float64
}

// NewMarketMicrostructureAnalyzer creates a new market microstructure analyzer
func NewMarketMicrostructureAnalyzer() *MarketMicrostructureAnalyzer {
	return &MarketMicrostructureAnalyzer{
		client:              &http.Client{Timeout: 10 * time.Second},
		baseURL:             "https://fapi.binance.com",
		vwapHistory:         make(map[string][]VWAPDataPoint),
		largeOrderThreshold: 100000, // $100k USD equivalent
	}
}

// FetchOrderBookDepth fetches order book depth from Binance Futures API
func (m *MarketMicrostructureAnalyzer) FetchOrderBookDepth(symbol string, limit int) (*OrderBookDepth, error) {
	if limit == 0 {
		limit = 20 // Default to 20 levels
	}

	// Validate limit (Binance accepts: 5, 10, 20, 50, 100, 500, 1000)
	validLimits := []int{5, 10, 20, 50, 100, 500, 1000}
	valid := false
	for _, vl := range validLimits {
		if limit == vl {
			valid = true
			break
		}
	}
	if !valid {
		// Find closest valid limit
		for _, vl := range validLimits {
			if vl >= limit {
				limit = vl
				break
			}
		}
	}

	url := fmt.Sprintf("%s/fapi/v1/depth?symbol=%s&limit=%d", m.baseURL, symbol, limit)

	resp, err := m.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order book depth: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp struct {
		LastUpdateID int64           `json:"lastUpdateId"`
		Bids         [][]interface{} `json:"bids"` // [[price, quantity], ...]
		Asks         [][]interface{} `json:"asks"` // [[price, quantity], ...]
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse order book: %w", err)
	}

	// Convert to our format
	depth := &OrderBookDepth{
		Symbol:       symbol,
		Timestamp:    time.Now(),
		LastUpdateID: apiResp.LastUpdateID,
		Bids:         make([]PriceLevel, 0, len(apiResp.Bids)),
		Asks:         make([]PriceLevel, 0, len(apiResp.Asks)),
	}

	for _, bid := range apiResp.Bids {
		if len(bid) >= 2 {
			price, _ := parseFloat(bid[0])
			quantity, _ := parseFloat(bid[1])
			depth.Bids = append(depth.Bids, PriceLevel{Price: price, Quantity: quantity})
		}
	}

	for _, ask := range apiResp.Asks {
		if len(ask) >= 2 {
			price, _ := parseFloat(ask[0])
			quantity, _ := parseFloat(ask[1])
			depth.Asks = append(depth.Asks, PriceLevel{Price: price, Quantity: quantity})
		}
	}

	return depth, nil
}

// AnalyzeMarketMicrostructure analyzes order book depth and calculates all microstructure metrics
func (m *MarketMicrostructureAnalyzer) AnalyzeMarketMicrostructure(symbol string, depth *OrderBookDepth, currentPrice float64, klines []Kline) (*MarketMicrostructure, error) {
	if depth == nil || len(depth.Bids) == 0 || len(depth.Asks) == 0 {
		return nil, fmt.Errorf("invalid order book depth data")
	}

	ms := &MarketMicrostructure{
		Symbol:       symbol,
		Timestamp:    time.Now(),
		CurrentPrice: currentPrice,
		Details:      make(map[string]interface{}),
	}

	// Calculate VWAP from recent klines
	if len(klines) > 0 {
		ms.VWAP = m.calculateVWAP(symbol, klines)
		ms.VWAPDeviation = ((currentPrice - ms.VWAP) / ms.VWAP) * 100
	}

	// Calculate bid-ask spread
	bestBid := depth.Bids[0].Price
	bestAsk := depth.Asks[0].Price
	midPrice := (bestBid + bestAsk) / 2
	ms.BidAskSpread = ((bestAsk - bestBid) / midPrice) * 100
	ms.BidAskSpreadBps = ms.BidAskSpread * 100 // Convert to basis points

	// Calculate order book depth (top 10 levels)
	ms.BidDepth = m.calculateTotalVolume(depth.Bids, 10)
	ms.AskDepth = m.calculateTotalVolume(depth.Asks, 10)

	// Calculate order book imbalance
	totalDepth := ms.BidDepth + ms.AskDepth
	if totalDepth > 0 {
		ms.OrderBookImbalance = ms.BidDepth / totalDepth
	} else {
		ms.OrderBookImbalance = 0.5
	}

	// Detect large orders
	ms.LargeOrderCount, ms.LargeOrderVolume = m.detectLargeOrders(depth, currentPrice)

	// Calculate cumulative volumes
	ms.CumulativeBidVolume = m.calculateCumulativeVolume(depth.Bids, midPrice, false)
	ms.CumulativeAskVolume = m.calculateCumulativeVolume(depth.Asks, midPrice, true)

	// Identify support and resistance levels
	ms.SupportLevels = m.identifySupportLevels(depth.Bids, midPrice)
	ms.ResistanceLevels = m.identifyResistanceLevels(depth.Asks, midPrice)

	// Add detailed metrics
	ms.Details["best_bid"] = bestBid
	ms.Details["best_ask"] = bestAsk
	ms.Details["mid_price"] = midPrice
	ms.Details["bid_levels"] = len(depth.Bids)
	ms.Details["ask_levels"] = len(depth.Asks)
	ms.Details["imbalance_direction"] = m.getImbalanceDirection(ms.OrderBookImbalance)

	return ms, nil
}

// calculateVWAP calculates Volume-Weighted Average Price from klines
func (m *MarketMicrostructureAnalyzer) calculateVWAP(symbol string, klines []Kline) float64 {
	if len(klines) == 0 {
		return 0
	}

	totalPV := 0.0 // Price * Volume
	totalVolume := 0.0

	for _, k := range klines {
		typicalPrice := (k.High + k.Low + k.Close) / 3
		totalPV += typicalPrice * k.Volume
		totalVolume += k.Volume
	}

	if totalVolume == 0 {
		return klines[len(klines)-1].Close
	}

	vwap := totalPV / totalVolume

	// Store in history
	m.mu.Lock()
	if _, exists := m.vwapHistory[symbol]; !exists {
		m.vwapHistory[symbol] = make([]VWAPDataPoint, 0, 100)
	}
	m.vwapHistory[symbol] = append(m.vwapHistory[symbol], VWAPDataPoint{
		Timestamp: time.Now(),
		VWAP:      vwap,
		Volume:    totalVolume,
	})
	// Keep last 100 points
	if len(m.vwapHistory[symbol]) > 100 {
		m.vwapHistory[symbol] = m.vwapHistory[symbol][1:]
	}
	m.mu.Unlock()

	return vwap
}

// calculateTotalVolume calculates total volume for top N levels
func (m *MarketMicrostructureAnalyzer) calculateTotalVolume(levels []PriceLevel, topN int) float64 {
	total := 0.0
	count := topN
	if count > len(levels) {
		count = len(levels)
	}
	for i := 0; i < count; i++ {
		total += levels[i].Quantity
	}
	return total
}

// detectLargeOrders detects large orders in the order book
func (m *MarketMicrostructureAnalyzer) detectLargeOrders(depth *OrderBookDepth, currentPrice float64) (int, float64) {
	count := 0
	totalVolume := 0.0

	// Calculate average order size
	avgBidSize := m.calculateAverageOrderSize(depth.Bids)
	avgAskSize := m.calculateAverageOrderSize(depth.Asks)
	avgOrderSize := (avgBidSize + avgAskSize) / 2

	// Large order = 5x average or $100k+ equivalent
	largeThreshold := math.Max(avgOrderSize*5, m.largeOrderThreshold/currentPrice)

	// Check bids
	for _, bid := range depth.Bids {
		orderValue := bid.Quantity * bid.Price
		if bid.Quantity > largeThreshold || orderValue > m.largeOrderThreshold {
			count++
			totalVolume += bid.Quantity
		}
	}

	// Check asks
	for _, ask := range depth.Asks {
		orderValue := ask.Quantity * ask.Price
		if ask.Quantity > largeThreshold || orderValue > m.largeOrderThreshold {
			count++
			totalVolume += ask.Quantity
		}
	}

	return count, totalVolume
}

// calculateAverageOrderSize calculates average order size for a side
func (m *MarketMicrostructureAnalyzer) calculateAverageOrderSize(levels []PriceLevel) float64 {
	if len(levels) == 0 {
		return 0
	}
	total := 0.0
	for _, level := range levels {
		total += level.Quantity
	}
	return total / float64(len(levels))
}

// calculateCumulativeVolume calculates cumulative volume at each price level
func (m *MarketMicrostructureAnalyzer) calculateCumulativeVolume(levels []PriceLevel, midPrice float64, isAsk bool) []CumulativeLevel {
	result := make([]CumulativeLevel, 0, len(levels))
	cumulative := 0.0

	for _, level := range levels {
		cumulative += level.Quantity
		percentageFromMid := ((level.Price - midPrice) / midPrice) * 100
		result = append(result, CumulativeLevel{
			Price:             level.Price,
			CumulativeVolume:  cumulative,
			PercentageFromMid: percentageFromMid,
		})
	}

	return result
}

// identifySupportLevels identifies significant support levels from bid side
func (m *MarketMicrostructureAnalyzer) identifySupportLevels(bids []PriceLevel, midPrice float64) []float64 {
	if len(bids) == 0 {
		return []float64{}
	}

	// Find levels with significantly higher volume (local maxima)
	avgVolume := m.calculateAverageOrderSize(bids)
	threshold := avgVolume * 3 // 3x average = significant level

	supports := []float64{}
	for i, bid := range bids {
		if bid.Quantity > threshold {
			// Check if it's a local maximum
			isLocalMax := true
			if i > 0 && bids[i-1].Quantity > bid.Quantity {
				isLocalMax = false
			}
			if i < len(bids)-1 && bids[i+1].Quantity > bid.Quantity {
				isLocalMax = false
			}
			if isLocalMax {
				supports = append(supports, bid.Price)
			}
		}
	}

	// Limit to top 5 support levels
	if len(supports) > 5 {
		supports = supports[:5]
	}

	return supports
}

// identifyResistanceLevels identifies significant resistance levels from ask side
func (m *MarketMicrostructureAnalyzer) identifyResistanceLevels(asks []PriceLevel, midPrice float64) []float64 {
	if len(asks) == 0 {
		return []float64{}
	}

	// Find levels with significantly higher volume (local maxima)
	avgVolume := m.calculateAverageOrderSize(asks)
	threshold := avgVolume * 3 // 3x average = significant level

	resistances := []float64{}
	for i, ask := range asks {
		if ask.Quantity > threshold {
			// Check if it's a local maximum
			isLocalMax := true
			if i > 0 && asks[i-1].Quantity > ask.Quantity {
				isLocalMax = false
			}
			if i < len(asks)-1 && asks[i+1].Quantity > ask.Quantity {
				isLocalMax = false
			}
			if isLocalMax {
				resistances = append(resistances, ask.Price)
			}
		}
	}

	// Limit to top 5 resistance levels
	if len(resistances) > 5 {
		resistances = resistances[:5]
	}

	return resistances
}

// getImbalanceDirection returns human-readable imbalance direction
func (m *MarketMicrostructureAnalyzer) getImbalanceDirection(imbalance float64) string {
	if imbalance > 0.65 {
		return "strong_buy"
	} else if imbalance > 0.55 {
		return "moderate_buy"
	} else if imbalance < 0.35 {
		return "strong_sell"
	} else if imbalance < 0.45 {
		return "moderate_sell"
	}
	return "balanced"
}

// SetLargeOrderThreshold sets the threshold for detecting large orders (in USD)
func (m *MarketMicrostructureAnalyzer) SetLargeOrderThreshold(thresholdUSD float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.largeOrderThreshold = thresholdUSD
}

// GetVWAPHistory returns VWAP history for a symbol
func (m *MarketMicrostructureAnalyzer) GetVWAPHistory(symbol string) []VWAPDataPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if history, exists := m.vwapHistory[symbol]; exists {
		// Return a copy
		result := make([]VWAPDataPoint, len(history))
		copy(result, history)
		return result
	}
	return []VWAPDataPoint{}
}
