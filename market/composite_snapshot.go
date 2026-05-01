package market

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type CompositeMarketSource struct {
	Name      string    `json:"name"`
	Available bool      `json:"available"`
	Reason    string    `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type CompositeMarketLine struct {
	ID          string  `json:"id"`
	Price       float64 `json:"price"`
	Kind        string  `json:"kind"`
	Label       string  `json:"label"`
	Timeframe   string  `json:"timeframe,omitempty"`
	Strength    int     `json:"strength,omitempty"`
	Source      string  `json:"source,omitempty"`
	DistancePct float64 `json:"distance_pct,omitempty"`
}

type CompositeMarketTimeframe struct {
	Timeframe string                `json:"timeframe"`
	Klines    []KlineBar            `json:"klines,omitempty"`
	EMA20     []float64             `json:"ema20,omitempty"`
	EMA50     []float64             `json:"ema50,omitempty"`
	RSI14     []float64             `json:"rsi14,omitempty"`
	ATR14     float64               `json:"atr14,omitempty"`
	Lines     []CompositeMarketLine `json:"lines,omitempty"`
}

type CompositeMarketSnapshot struct {
	Symbol        string                              `json:"symbol"`
	Exchange      string                              `json:"exchange"`
	PrimaryTF     string                              `json:"primary_timeframe"`
	UpdatedAt     time.Time                           `json:"updated_at"`
	ExpiresAt     time.Time                           `json:"expires_at"`
	TTLSeconds    int                                 `json:"ttl_seconds"`
	Stale         bool                                `json:"stale"`
	Price         float64                             `json:"price"`
	PriceChange1h float64                             `json:"price_change_1h"`
	PriceChange4h float64                             `json:"price_change_4h"`
	DataQuality   string                              `json:"data_quality,omitempty"`
	Sources       []CompositeMarketSource             `json:"sources,omitempty"`
	Context       *MarketContextV2                    `json:"context,omitempty"`
	Timeframes    map[string]CompositeMarketTimeframe `json:"timeframes,omitempty"`
	Lines         []CompositeMarketLine               `json:"lines,omitempty"`
	AICompact     string                              `json:"ai_compact,omitempty"`
}

type compositeMarketCacheEntry struct {
	snapshot  *CompositeMarketSnapshot
	updatedAt time.Time
}

var compositeMarketCache sync.Map

func BuildCompositeMarketSnapshot(symbol, exchange string, timeframes []string, primaryTF string, count int, ttl time.Duration) (*CompositeMarketSnapshot, error) {
	if exchange == "" {
		exchange = "okx"
	}
	if primaryTF == "" {
		primaryTF = "15m"
	}
	if len(timeframes) == 0 {
		timeframes = []string{"3m", "5m", "15m", "1h", "4h", "1d"}
	}
	if count <= 0 {
		count = 120
	}
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	key := strings.ToUpper(Normalize(symbol)) + "|" + strings.ToLower(exchange) + "|" + primaryTF + "|" + strings.Join(timeframes, ",") + fmt.Sprintf("|%d", count)
	if cached, ok := compositeMarketCache.Load(key); ok {
		entry := cached.(*compositeMarketCacheEntry)
		if time.Since(entry.updatedAt) < ttl && entry.snapshot != nil {
			cp := *entry.snapshot
			cp.Stale = false
			return &cp, nil
		}
	}
	data, err := GetWithTimeframesExchange(symbol, timeframes, primaryTF, count, exchange)
	if err != nil {
		return nil, err
	}
	s := buildCompositeMarketSnapshotFromData(exchange, timeframes, primaryTF, ttl, data)
	compositeMarketCache.Store(key, &compositeMarketCacheEntry{snapshot: s, updatedAt: s.UpdatedAt})
	return s, nil
}

func BuildCompositeMarketSnapshotFromExistingData(exchange string, timeframes []string, primaryTF string, ttl time.Duration, data *Data) *CompositeMarketSnapshot {
	if data == nil {
		return nil
	}
	if exchange == "" {
		exchange = "okx"
	}
	if primaryTF == "" {
		primaryTF = "15m"
	}
	if len(timeframes) == 0 {
		timeframes = []string{"3m", "15m", "1h", "4h", "1d"}
	}
	if ttl <= 0 {
		ttl = 180 * time.Second
	}
	return buildCompositeMarketSnapshotFromData(exchange, timeframes, primaryTF, ttl, data)
}

func buildCompositeMarketSnapshotFromData(exchange string, timeframes []string, primaryTF string, ttl time.Duration, data *Data) *CompositeMarketSnapshot {
	now := time.Now().UTC()
	s := &CompositeMarketSnapshot{
		Symbol:        data.Symbol,
		Exchange:      exchange,
		PrimaryTF:     primaryTF,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(ttl),
		TTLSeconds:    int(ttl.Seconds()),
		Price:         data.CurrentPrice,
		PriceChange1h: data.PriceChange1h,
		PriceChange4h: data.PriceChange4h,
		Context:       BuildMarketContextV2(data.Symbol, data, timeframes, primaryTF),
		Timeframes:    map[string]CompositeMarketTimeframe{},
	}
	s.DataQuality = s.Context.DataQuality
	s.Sources = buildCompositeSources(data, s.Context)
	for _, tf := range sortedTimeframes(data.TimeframeData) {
		series := data.TimeframeData[tf]
		if series == nil {
			continue
		}
		lines := buildLinesForTimeframe(data.CurrentPrice, series.StructuralLevels, series.FibonacciLevels)
		s.Timeframes[tf] = CompositeMarketTimeframe{Timeframe: tf, Klines: series.Klines, EMA20: series.EMA20Values, EMA50: series.EMA50Values, RSI14: series.RSI14Values, ATR14: series.ATR14, Lines: lines}
		s.Lines = append(s.Lines, lines...)
	}
	if len(s.Lines) == 0 {
		s.Lines = buildLinesForTimeframe(data.CurrentPrice, data.StructuralLevels, data.FibonacciLevels)
	}
	s.AICompact = FormatCompositeMarketForAI(s)
	return s
}

func ProjectCompositeMarketSnapshot(s *CompositeMarketSnapshot, view string) *CompositeMarketSnapshot {
	if s == nil {
		return nil
	}
	cp := *s
	switch strings.ToLower(strings.TrimSpace(view)) {
	case "ai":
		cp.Context = nil
		cp.Timeframes = nil
		cp.Lines = compactNearestLines(cp.Lines, 12)
	case "summary":
		cp.Timeframes = nil
		cp.Lines = compactNearestLines(cp.Lines, 16)
	case "chart":
		cp.Context = nil
		cp.Timeframes = compactChartTimeframes(cp.Timeframes, 120)
		cp.Lines = compactNearestLines(cp.Lines, 24)
	case "full", "":
		return &cp
	default:
		cp.Timeframes = nil
		cp.Lines = compactNearestLines(cp.Lines, 16)
	}
	return &cp
}

func compactNearestLines(lines []CompositeMarketLine, limit int) []CompositeMarketLine {
	if limit <= 0 || len(lines) <= limit {
		out := append([]CompositeMarketLine(nil), lines...)
		return out
	}
	out := append([]CompositeMarketLine(nil), lines...)
	sort.Slice(out, func(i, j int) bool { return absFloat(out[i].DistancePct) < absFloat(out[j].DistancePct) })
	return out[:limit]
}

func compactChartTimeframes(in map[string]CompositeMarketTimeframe, maxBars int) map[string]CompositeMarketTimeframe {
	if in == nil {
		return nil
	}
	out := make(map[string]CompositeMarketTimeframe, len(in))
	for tf, series := range in {
		if maxBars > 0 && len(series.Klines) > maxBars {
			trim := len(series.Klines) - maxBars
			series.Klines = append([]KlineBar(nil), series.Klines[trim:]...)
			series.EMA20 = trimFloatSlice(series.EMA20, maxBars)
			series.EMA50 = trimFloatSlice(series.EMA50, maxBars)
			series.RSI14 = trimFloatSlice(series.RSI14, maxBars)
		} else {
			series.Klines = append([]KlineBar(nil), series.Klines...)
			series.EMA20 = append([]float64(nil), series.EMA20...)
			series.EMA50 = append([]float64(nil), series.EMA50...)
			series.RSI14 = append([]float64(nil), series.RSI14...)
		}
		series.Lines = compactNearestLines(series.Lines, 24)
		out[tf] = series
	}
	return out
}

func trimFloatSlice(in []float64, max int) []float64 {
	if max <= 0 || len(in) <= max {
		return append([]float64(nil), in...)
	}
	return append([]float64(nil), in[len(in)-max:]...)
}

func buildCompositeSources(data *Data, ctx *MarketContextV2) []CompositeMarketSource {
	now := time.Now().UTC()
	sources := []CompositeMarketSource{{Name: "klines", Available: data != nil && len(data.TimeframeData) > 0, UpdatedAt: now}}
	if data == nil {
		return sources
	}
	sources = append(sources,
		CompositeMarketSource{Name: "open_interest", Available: data.OpenInterest != nil, UpdatedAt: now, Reason: boolReason(data.OpenInterest != nil, "unavailable")},
		CompositeMarketSource{Name: "funding", Available: data.FundingRate != 0, UpdatedAt: now, Reason: boolReason(data.FundingRate != 0, "unavailable_or_zero")},
		CompositeMarketSource{Name: "exchange_flow", Available: ctx != nil && ctx.ExchangeFlow != nil && ctx.ExchangeFlow.DataQuality != "missing", UpdatedAt: now},
		CompositeMarketSource{Name: "structure", Available: len(data.StructuralLevels) > 0, UpdatedAt: now},
	)
	if data.QuantContext != nil {
		sources = append(sources, CompositeMarketSource{Name: "quant_optional", Available: data.QuantContext.DataQuality != "missing", UpdatedAt: now, Reason: data.QuantContext.DataQuality})
	}
	return sources
}

func buildLinesForTimeframe(current float64, levels []StructuralLevel, fib *FibonacciLevels) []CompositeMarketLine {
	out := make([]CompositeMarketLine, 0, len(levels)+8)
	for i, l := range levels {
		out = append(out, CompositeMarketLine{ID: fmt.Sprintf("%s-%s-%d", l.Timeframe, l.Type, i), Price: l.Price, Kind: l.Type, Label: l.Type, Timeframe: l.Timeframe, Strength: l.Strength, Source: l.Source, DistancePct: distancePct(current, l.Price)})
	}
	if fib != nil {
		keys := make([]string, 0, len(fib.Levels))
		for k := range fib.Levels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := fib.Levels[k]
			out = append(out, CompositeMarketLine{ID: "fib-" + fib.Timeframe + "-" + k, Price: p, Kind: "fibonacci", Label: "Fib " + k, Timeframe: fib.Timeframe, Source: fib.Direction, DistancePct: distancePct(current, p)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

func FormatCompositeMarketForAI(s *CompositeMarketSnapshot) string {
	if s == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Composite market snapshot %s %s price=%.8f 1h=%.2f%% 4h=%.2f%% quality=%s updated_at=%s ttl=%ds\n", s.Exchange, s.Symbol, s.Price, s.PriceChange1h, s.PriceChange4h, s.DataQuality, s.UpdatedAt.Format(time.RFC3339), s.TTLSeconds))
	b.WriteString("AI note: market snapshot is point-in-time, not realtime tick data; do not infer movement after updated_at without newer data.\n")
	if s.Context != nil && s.Context.RegimeRules != nil {
		b.WriteString(fmt.Sprintf("regime=%s allowed=%s structure=%s protection=%s\n", s.Context.RegimeRules.Regime, strings.Join(s.Context.RegimeRules.AllowedSetups, ","), s.Context.RegimeRules.StructureMode, s.Context.RegimeRules.ProtectionGuidance))
	}
	if s.Context != nil && s.Context.ExchangeFlow != nil {
		ef := s.Context.ExchangeFlow
		b.WriteString(fmt.Sprintf("exchange_flow funding=%s long_short=%s taker=%s depth=%s crowding=%s\n", ef.FundingBias, ef.LongShortSkew, ef.TakerFlowBias, ef.DepthBias, ef.CrowdingRisk))
	}
	near := append([]CompositeMarketLine(nil), s.Lines...)
	sort.Slice(near, func(i, j int) bool { return absFloat(near[i].DistancePct) < absFloat(near[j].DistancePct) })
	limit := 8
	if len(near) < limit {
		limit = len(near)
	}
	for i := 0; i < limit; i++ {
		l := near[i]
		b.WriteString(fmt.Sprintf("line %s %s %.8f dist=%.2f%% strength=%d src=%s\n", l.Timeframe, l.Kind, l.Price, l.DistancePct, l.Strength, l.Source))
	}
	return b.String()
}

func sortedTimeframes(m map[string]*TimeframeSeriesData) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func boolReason(ok bool, reason string) string {
	if ok {
		return ""
	}
	return reason
}
func distancePct(current, price float64) float64 {
	if current <= 0 {
		return 0
	}
	return ((price - current) / current) * 100
}
func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
