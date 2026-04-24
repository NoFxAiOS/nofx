package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"nofx/market"
	tradertypes "nofx/trader/types"
	"nofx/trader/paper"
)

type Scenario struct {
	Name         string                `json:"name"`
	Symbol       string                `json:"symbol"`
	InitialPrice float64               `json:"initial_price"`
	Prices       []float64             `json:"prices"`
	FundingRates []float64             `json:"funding_rates,omitempty"`
	Actions      []ScenarioAction      `json:"actions"`
	Protection   *ScenarioProtection   `json:"protection,omitempty"`
	RegimeFilter *ScenarioRegimeFilter `json:"regime_filter,omitempty"`
	Expected     ScenarioExpected      `json:"expected"`
}

type ScenarioAction struct {
	Type     string  `json:"type"`
	Quantity float64 `json:"quantity"`
	Leverage int     `json:"leverage"`
	Price    float64 `json:"price,omitempty"`
}

type ScenarioProtection struct {
	Mode          string  `json:"mode,omitempty"`
	TakeProfitPct float64 `json:"take_profit_pct,omitempty"`
	StopLossPct   float64 `json:"stop_loss_pct,omitempty"`
}

type ScenarioRegimeFilter struct {
	Enabled           bool     `json:"enabled"`
	AllowedRegimes    []string `json:"allowed_regimes,omitempty"`
	BlockHighFunding  bool     `json:"block_high_funding"`
	MaxFundingRateAbs float64  `json:"max_funding_rate_abs,omitempty"`
	RequireTrendAlign bool     `json:"require_trend_alignment"`
}

type ScenarioExpected struct {
	ProtectionOrders   int     `json:"protection_orders"`
	FinalPositionCount int     `json:"final_position_count"`
	ClosedPnLCount     int     `json:"closed_pnl_count,omitempty"`
	RealizedPnL        float64 `json:"realized_pnl,omitempty"`
	Blocked            bool    `json:"blocked,omitempty"`
}

type Result struct {
	ScenarioName       string
	ProtectionOrders   int
	FinalPositionCount int
	ClosedPnLCount     int
	RealizedPnL        float64
	Blocked            bool
}

func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var scenario Scenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, err
	}
	return &scenario, nil
}

func RunScenario(s *Scenario) (*Result, error) {
	if s == nil {
		return nil, fmt.Errorf("nil scenario")
	}
	if s.Symbol == "" {
		return nil, fmt.Errorf("scenario symbol is required")
	}

	pt := paper.NewTrader()
	priceSeries := append([]float64{}, s.Prices...)
	if len(priceSeries) == 0 && s.InitialPrice > 0 {
		priceSeries = []float64{s.InitialPrice}
	}
	if s.InitialPrice > 0 {
		pt.SetPrice(s.Symbol, s.InitialPrice)
	}
	for _, price := range priceSeries {
		if price > 0 {
			pt.SetPrice(s.Symbol, price)
		}
	}

	blocked := false
	latestFunding := 0.0
	if len(s.FundingRates) > 0 {
		latestFunding = s.FundingRates[len(s.FundingRates)-1]
	}
	baselinePrice := s.InitialPrice
	if baselinePrice <= 0 && len(priceSeries) > 0 {
		baselinePrice = priceSeries[0]
	}
	currentPrice := baselinePrice
	if currentPrice <= 0 {
		currentPrice, _ = pt.GetMarketPrice(s.Symbol)
	}
	marketData := &market.Data{
		CurrentPrice:   currentPrice,
		CurrentEMA20:   baselinePrice, // EMA20 tracks the baseline, not the current price
		PriceChange4h:  computeChangeFromBaseline(baselinePrice, currentPrice),
		PriceChange1h:  computeChangeFromBaseline(baselinePrice, currentPrice) * 0.5, // approximate 1h as half of 4h
		FundingRate:    latestFunding,
		IntradaySeries: &market.IntradayData{ATR14: currentPrice * 0.01},
	}

	for _, action := range s.Actions {
		if action.Price > 0 {
			pt.SetPrice(s.Symbol, action.Price)
			marketData.CurrentPrice = action.Price
			change4h := computeChangeFromBaseline(baselinePrice, action.Price)
			marketData.PriceChange4h = change4h
			marketData.PriceChange1h = change4h * 0.5
			marketData.IntradaySeries = &market.IntradayData{ATR14: action.Price * 0.01}
		}
		if shouldCheckRegimeFilter(action.Type) && s.RegimeFilter != nil && s.RegimeFilter.Enabled {
			if blockedByScenarioRegimeFilter(action.Type, marketData, s.RegimeFilter) {
				blocked = true
				continue
			}
		}
		switch action.Type {
		case "open_long":
			if _, err := pt.OpenLong(s.Symbol, action.Quantity, action.Leverage); err != nil {
				return nil, err
			}
			entryPrice, _ := pt.GetMarketPrice(s.Symbol)
			if err := applyScenarioProtection(pt, s.Symbol, "LONG", action.Quantity, entryPrice, s.Protection, true); err != nil {
				return nil, err
			}
		case "open_short":
			if _, err := pt.OpenShort(s.Symbol, action.Quantity, action.Leverage); err != nil {
				return nil, err
			}
			entryPrice, _ := pt.GetMarketPrice(s.Symbol)
			if err := applyScenarioProtection(pt, s.Symbol, "SHORT", action.Quantity, entryPrice, s.Protection, false); err != nil {
				return nil, err
			}
		case "close_long":
			if _, err := pt.CloseLong(s.Symbol, action.Quantity); err != nil {
				return nil, err
			}
		case "close_short":
			if _, err := pt.CloseShort(s.Symbol, action.Quantity); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported scenario action: %s", action.Type)
		}
	}

	orders, err := pt.GetOpenOrders(s.Symbol)
	if err != nil {
		return nil, err
	}
	positions, err := pt.GetPositions()
	if err != nil {
		return nil, err
	}

	closedPnL, err := pt.GetClosedPnL(time.Time{}, 1000)
	if err != nil {
		return nil, err
	}

	return &Result{
		ScenarioName:       s.Name,
		ProtectionOrders:   len(orders),
		FinalPositionCount: len(positions),
		ClosedPnLCount:     len(closedPnL),
		RealizedPnL:        sumRealizedPnL(closedPnL),
		Blocked:            blocked,
	}, nil
}

func ValidateResult(s *Scenario, result *Result) error {
	if s == nil || result == nil {
		return fmt.Errorf("scenario and result are required")
	}
	if s.Expected.Blocked != result.Blocked {
		return fmt.Errorf("expected blocked=%v, got %v", s.Expected.Blocked, result.Blocked)
	}
	if s.Expected.ProtectionOrders >= 0 && result.ProtectionOrders != s.Expected.ProtectionOrders {
		return fmt.Errorf("expected protection orders %d, got %d", s.Expected.ProtectionOrders, result.ProtectionOrders)
	}
	if s.Expected.ClosedPnLCount >= 0 && result.ClosedPnLCount != s.Expected.ClosedPnLCount {
		return fmt.Errorf("expected closed pnl count %d, got %d", s.Expected.ClosedPnLCount, result.ClosedPnLCount)
	}
	if s.Expected.RealizedPnL != 0 && abs(result.RealizedPnL-s.Expected.RealizedPnL) > 1e-9 {
		return fmt.Errorf("expected realized pnl %.8f, got %.8f", s.Expected.RealizedPnL, result.RealizedPnL)
	}
	if result.FinalPositionCount != s.Expected.FinalPositionCount {
		return fmt.Errorf("expected final position count %d, got %d", s.Expected.FinalPositionCount, result.FinalPositionCount)
	}
	return nil
}

func applyScenarioProtection(pt *paper.Trader, symbol, positionSide string, quantity, entryPrice float64, protection *ScenarioProtection, isLong bool) error {
	if protection == nil {
		return nil
	}
	mode := strings.ToLower(protection.Mode)
	if mode == "" || mode == "full" || mode == "ai" || mode == "manual" {
		if protection.StopLossPct > 0 {
			stop := entryPrice
			if isLong {
				stop = entryPrice * (1 - protection.StopLossPct/100)
			} else {
				stop = entryPrice * (1 + protection.StopLossPct/100)
			}
			if err := pt.SetStopLoss(symbol, positionSide, quantity, stop); err != nil {
				return err
			}
		}
		if protection.TakeProfitPct > 0 {
			tp := entryPrice
			if isLong {
				tp = entryPrice * (1 + protection.TakeProfitPct/100)
			} else {
				tp = entryPrice * (1 - protection.TakeProfitPct/100)
			}
			if err := pt.SetTakeProfit(symbol, positionSide, quantity, tp); err != nil {
				return err
			}
		}
	}
	return nil
}

func blockedByScenarioRegimeFilter(action string, data *market.Data, cfg *ScenarioRegimeFilter) bool {
	if cfg == nil || !cfg.Enabled {
		return false
	}
	regime := "standard"
	if data != nil && abs(data.PriceChange4h) >= 5 {
		if data.PriceChange4h >= 0 {
			regime = "trending_up"
		} else {
			regime = "trending_down"
		}
	}
	if len(cfg.AllowedRegimes) > 0 {
		allowed := false
		for _, item := range cfg.AllowedRegimes {
			if strings.EqualFold(item, regime) {
				allowed = true
				break
			}
			// "trending" matches trending_up and trending_down
			if strings.EqualFold(item, "trending") &&
				(strings.EqualFold(regime, "trending_up") || strings.EqualFold(regime, "trending_down")) {
				allowed = true
				break
			}
		}
		if !allowed {
			return true
		}
	}
	if cfg.BlockHighFunding && data != nil && cfg.MaxFundingRateAbs > 0 && abs(data.FundingRate) > cfg.MaxFundingRateAbs {
		return true
	}
	if cfg.RequireTrendAlign && data != nil {
		// Directional regime check: block counter-trend trades
		if regime == "trending_up" && strings.EqualFold(action, "open_short") {
			return true
		}
		if regime == "trending_down" && strings.EqualFold(action, "open_long") {
			return true
		}
		// In range regimes, block if 3+ factors oppose the direction
		if regime == "standard" {
			counterScore := 0
			if strings.EqualFold(action, "open_long") {
				if data.CurrentPrice < data.CurrentEMA20 { counterScore++ }
				if data.PriceChange4h < 0 { counterScore++ }
				if data.PriceChange1h < 0 { counterScore++ }
				if data.CurrentMACD < 0 { counterScore++ }
			} else if strings.EqualFold(action, "open_short") {
				if data.CurrentPrice > data.CurrentEMA20 { counterScore++ }
				if data.PriceChange4h > 0 { counterScore++ }
				if data.PriceChange1h > 0 { counterScore++ }
				if data.CurrentMACD > 0 { counterScore++ }
			}
			if counterScore >= 3 {
				return true
			}
		}
	}
	return false
}

func shouldCheckRegimeFilter(action string) bool {
	switch strings.ToLower(action) {
	case "open_long", "open_short":
		return true
	default:
		return false
	}
}

func computePriceChange4h(prices []float64) float64 {
	if len(prices) < 2 || prices[0] <= 0 {
		return 0
	}
	return (prices[len(prices)-1] - prices[0]) / prices[0] * 100
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func computeChangeFromBaseline(baseline, current float64) float64 {
	if baseline <= 0 || current <= 0 {
		return 0
	}
	return (current - baseline) / baseline * 100
}

func sumRealizedPnL(records []tradertypes.ClosedPnLRecord) float64 {
	total := 0.0
	for _, record := range records {
		total += record.RealizedPnL
	}
	return total
}
