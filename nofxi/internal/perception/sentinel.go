package perception

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Signal types for proactive notifications.
type SignalType string

const (
	SignalPriceBreakout  SignalType = "price_breakout"   // Sudden price move
	SignalVolumeSpike    SignalType = "volume_spike"      // Abnormal volume
	SignalFundingRate    SignalType = "funding_rate"      // Extreme funding rate
	SignalLiquidation    SignalType = "liquidation_wave"  // Mass liquidations
	SignalTrendReversal  SignalType = "trend_reversal"    // Potential reversal
	SignalPositionRisk   SignalType = "position_risk"     // User's position at risk
)

// Signal is a proactive market event detected by the sentinel.
type Signal struct {
	Type      SignalType `json:"type"`
	Symbol    string     `json:"symbol"`
	Severity  string     `json:"severity"` // "info", "warning", "critical"
	Title     string     `json:"title"`
	Detail    string     `json:"detail"`
	Price     float64    `json:"price"`
	Change    float64    `json:"change"` // Percentage
	Timestamp time.Time  `json:"timestamp"`
}

// SignalCallback is called when the sentinel detects something.
type SignalCallback func(signal Signal)

// Sentinel continuously monitors markets and detects anomalies.
// This is the "eyes" of NOFXi — always watching, always analyzing.
type Sentinel struct {
	mu          sync.RWMutex
	symbols     []string
	history     map[string][]pricePoint // symbol → recent prices
	onSignal    SignalCallback
	httpClient  *http.Client
	logger      *slog.Logger
	stopCh      chan struct{}

	// Thresholds
	priceBreakoutPct float64 // Price move % to trigger alert (default 3%)
	volumeSpikeMult  float64 // Volume multiplier vs average (default 3x)
	fundingThreshold float64 // Extreme funding rate threshold (default 0.1%)
}

type pricePoint struct {
	Price     float64
	Volume    float64
	Timestamp time.Time
}

// NewSentinel creates a new market sentinel.
func NewSentinel(symbols []string, onSignal SignalCallback, logger *slog.Logger) *Sentinel {
	return &Sentinel{
		symbols:          symbols,
		history:          make(map[string][]pricePoint),
		onSignal:         onSignal,
		httpClient:       &http.Client{Timeout: 10 * time.Second},
		logger:           logger,
		stopCh:           make(chan struct{}),
		priceBreakoutPct: 3.0,
		volumeSpikeMult:  3.0,
		fundingThreshold: 0.1,
	}
}

// Start begins the sentinel loop. Checks every 60 seconds.
func (s *Sentinel) Start() {
	go s.loop()
	s.logger.Info("sentinel started", "symbols", s.symbols)
}

// Stop stops the sentinel.
func (s *Sentinel) Stop() {
	close(s.stopCh)
}

// AddSymbol adds a symbol to watch.
func (s *Sentinel) AddSymbol(symbol string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sym := range s.symbols {
		if sym == symbol {
			return
		}
	}
	s.symbols = append(s.symbols, symbol)
}

func (s *Sentinel) loop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Initial scan
	s.scan()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.scan()
		}
	}
}

func (s *Sentinel) scan() {
	s.mu.RLock()
	symbols := make([]string, len(s.symbols))
	copy(symbols, s.symbols)
	s.mu.RUnlock()

	for _, sym := range symbols {
		s.checkSymbol(sym)
	}
	s.checkFundingRates()
}

func (s *Sentinel) checkSymbol(symbol string) {
	// Fetch current ticker
	ticker, err := s.fetchTicker(symbol)
	if err != nil {
		return
	}

	price, _ := strconv.ParseFloat(ticker["lastPrice"].(string), 64)
	volume, _ := strconv.ParseFloat(ticker["quoteVolume"].(string), 64)
	changePct, _ := strconv.ParseFloat(ticker["priceChangePercent"].(string), 64)

	now := time.Now()
	point := pricePoint{Price: price, Volume: volume, Timestamp: now}

	s.mu.Lock()
	hist := s.history[symbol]
	hist = append(hist, point)
	// Keep last 60 points (1 hour at 1min intervals)
	if len(hist) > 60 {
		hist = hist[len(hist)-60:]
	}
	s.history[symbol] = hist
	s.mu.Unlock()

	// Need at least 5 data points to detect anomalies
	if len(hist) < 5 {
		return
	}

	// === Detect Price Breakout ===
	// Compare current price to 5-minute-ago price
	fiveAgo := hist[len(hist)-5]
	pctMove := ((price - fiveAgo.Price) / fiveAgo.Price) * 100
	if math.Abs(pctMove) >= s.priceBreakoutPct {
		direction := "📈 上涨"
		severity := "warning"
		if pctMove < 0 {
			direction = "📉 下跌"
		}
		if math.Abs(pctMove) >= s.priceBreakoutPct*2 {
			severity = "critical"
		}
		s.emit(Signal{
			Type:     SignalPriceBreakout,
			Symbol:   symbol,
			Severity: severity,
			Title:    fmt.Sprintf("%s %s 急速%s %.1f%%", symbol, direction, map[bool]string{true: "拉升", false: "下跌"}[pctMove > 0], math.Abs(pctMove)),
			Detail:   fmt.Sprintf("5分钟内从 $%.2f → $%.2f，变动 %.1f%%\n24h 涨跌: %.1f%%", fiveAgo.Price, price, pctMove, changePct),
			Price:    price,
			Change:   pctMove,
		})
	}

	// === Detect Volume Spike ===
	if len(hist) >= 10 {
		var avgVol float64
		for i := 0; i < len(hist)-1; i++ {
			avgVol += hist[i].Volume
		}
		avgVol /= float64(len(hist) - 1)
		if avgVol > 0 && volume > avgVol*s.volumeSpikeMult {
			mult := volume / avgVol
			s.emit(Signal{
				Type:     SignalVolumeSpike,
				Symbol:   symbol,
				Severity: "warning",
				Title:    fmt.Sprintf("%s 成交量异常放大 %.1fx", symbol, mult),
				Detail:   fmt.Sprintf("当前成交量是平均值的 %.1f 倍\n价格: $%.2f (24h: %.1f%%)", mult, price, changePct),
				Price:    price,
				Change:   changePct,
			})
		}
	}
}

func (s *Sentinel) checkFundingRates() {
	url := "https://fapi.binance.com/fapi/v1/premiumIndex"
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var indexes []map[string]interface{}
	if err := json.Unmarshal(body, &indexes); err != nil {
		return
	}

	s.mu.RLock()
	watchSet := make(map[string]bool)
	for _, sym := range s.symbols {
		watchSet[sym] = true
	}
	s.mu.RUnlock()

	for _, idx := range indexes {
		symbol, _ := idx["symbol"].(string)
		if !watchSet[symbol] {
			continue
		}
		rateStr, _ := idx["lastFundingRate"].(string)
		rate, _ := strconv.ParseFloat(rateStr, 64)
		ratePct := rate * 100

		if math.Abs(ratePct) >= s.fundingThreshold {
			direction := "多头主导"
			if ratePct < 0 {
				direction = "空头主导"
			}
			s.emit(Signal{
				Type:     SignalFundingRate,
				Symbol:   symbol,
				Severity: "info",
				Title:    fmt.Sprintf("%s 资金费率异常: %.4f%%", symbol, ratePct),
				Detail:   fmt.Sprintf("当前资金费率 %.4f%% (%s)\n极端费率可能预示反转", ratePct, direction),
				Change:   ratePct,
			})
		}
	}
}

func (s *Sentinel) fetchTicker(symbol string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", symbol)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

func (s *Sentinel) emit(sig Signal) {
	sig.Timestamp = time.Now()
	s.logger.Info("signal detected",
		"type", sig.Type,
		"symbol", sig.Symbol,
		"severity", sig.Severity,
		"title", sig.Title,
	)
	if s.onSignal != nil {
		s.onSignal(sig)
	}
}
