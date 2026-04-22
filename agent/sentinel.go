package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"nofx/safe"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SignalType string

const (
	SignalPriceBreakout SignalType = "price_breakout"
	SignalVolumeSpike   SignalType = "volume_spike"
	SignalFundingRate   SignalType = "funding_rate"
)

type Signal struct {
	Type     SignalType
	Symbol   string
	Severity string
	Title    string
	Detail   string
	Price    float64
	Change   float64
}

type SignalCallback func(Signal)

type Sentinel struct {
	mu       sync.RWMutex
	symbols  []string
	history  map[string][]pricePt
	onSignal SignalCallback
	http     *http.Client
	logger   *slog.Logger
	stopCh   chan struct{}
	stopOnce sync.Once
}

type pricePt struct {
	Price  float64
	Volume float64
	Time   time.Time
}

func NewSentinel(symbols []string, cb SignalCallback, logger *slog.Logger) *Sentinel {
	return &Sentinel{
		symbols:  symbols,
		history:  make(map[string][]pricePt),
		onSignal: cb,
		http:     &http.Client{Timeout: 10 * time.Second},
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

func (s *Sentinel) Start() {
	safe.GoNamed("sentinel", func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		s.scan()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.scan()
			}
		}
	})
}

func (s *Sentinel) Stop()                            { s.stopOnce.Do(func() { close(s.stopCh) }) }
func (s *Sentinel) SymbolCount() int                  { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.symbols) }
func (s *Sentinel) AddSymbol(sym string)              { s.mu.Lock(); defer s.mu.Unlock(); for _, x := range s.symbols { if x == sym { return } }; s.symbols = append(s.symbols, sym) }
func (s *Sentinel) RemoveSymbol(sym string)           { s.mu.Lock(); defer s.mu.Unlock(); for i, x := range s.symbols { if x == sym { s.symbols = append(s.symbols[:i], s.symbols[i+1:]...); return } } }

func (s *Sentinel) FormatWatchlist(L string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.symbols) == 0 {
		if L == "zh" { return "📭 监控列表为空。用 `/watch BTC` 添加。" }
		return "📭 Watchlist empty. Use `/watch BTC` to add."
	}
	var sb strings.Builder
	if L == "zh" { sb.WriteString("👁️ *监控列表*\n\n") } else { sb.WriteString("👁️ *Watchlist*\n\n") }
	for _, sym := range s.symbols {
		if pts, ok := s.history[sym]; ok && len(pts) > 0 {
			last := pts[len(pts)-1]
			sb.WriteString(fmt.Sprintf("• *%s*: $%.4f (%s)\n", sym, last.Price, last.Time.Format("15:04")))
		} else {
			sb.WriteString(fmt.Sprintf("• *%s*: waiting...\n", sym))
		}
	}
	return sb.String()
}

func (s *Sentinel) scan() {
	s.mu.RLock()
	syms := make([]string, len(s.symbols))
	copy(syms, s.symbols)
	s.mu.RUnlock()
	for _, sym := range syms {
		s.check(sym)
	}
}

func (s *Sentinel) check(symbol string) {
	resp, err := s.http.Get(fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/24hr?symbol=%s", symbol))
	if err != nil { return }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.logger.Debug("sentinel ticker non-200", "symbol", symbol, "status", resp.StatusCode)
		return
	}
	body, err := safe.ReadAllLimited(resp.Body, 256*1024) // 256KB limit
	if err != nil { return }
	var t map[string]interface{}
	if err := json.Unmarshal(body, &t); err != nil { return }

	price, _ := strconv.ParseFloat(fmt.Sprint(t["lastPrice"]), 64)
	vol, _ := strconv.ParseFloat(fmt.Sprint(t["quoteVolume"]), 64)
	chg, _ := strconv.ParseFloat(fmt.Sprint(t["priceChangePercent"]), 64)

	pt := pricePt{Price: price, Volume: vol, Time: time.Now()}
	s.mu.Lock()
	h := s.history[symbol]
	h = append(h, pt)
	if len(h) > 60 { h = h[len(h)-60:] }
	s.history[symbol] = h
	s.mu.Unlock()

	if len(h) < 5 { return }

	// Price breakout (>3% in 5 min)
	old := h[len(h)-5]
	pct := ((price - old.Price) / old.Price) * 100
	if math.Abs(pct) >= 3.0 {
		sev := "warning"
		if math.Abs(pct) >= 6.0 { sev = "critical" }
		dir := "📈 拉升"
		if pct < 0 { dir = "📉 下跌" }
		s.emit(Signal{Type: SignalPriceBreakout, Symbol: symbol, Severity: sev,
			Title: fmt.Sprintf("%s %s %.1f%%", symbol, dir, math.Abs(pct)),
			Detail: fmt.Sprintf("5min: $%.2f → $%.2f (24h: %.1f%%)", old.Price, price, chg),
			Price: price, Change: pct})
	}

	// Volume spike (>3x avg)
	if len(h) >= 10 {
		var avg float64
		for i := 0; i < len(h)-1; i++ { avg += h[i].Volume }
		avg /= float64(len(h) - 1)
		if avg > 0 && vol > avg*3 {
			s.emit(Signal{Type: SignalVolumeSpike, Symbol: symbol, Severity: "warning",
				Title: fmt.Sprintf("%s 成交量异常 %.1fx", symbol, vol/avg),
				Detail: fmt.Sprintf("Price: $%.2f (24h: %.1f%%)", price, chg),
				Price: price, Change: chg})
		}
	}
}

func (s *Sentinel) emit(sig Signal) {
	s.logger.Info("signal", "type", sig.Type, "symbol", sig.Symbol, "title", sig.Title)
	if s.onSignal != nil { s.onSignal(sig) }
}
