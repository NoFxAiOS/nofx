package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"nofx/market"
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
	ProtectionOrders   int  `json:"protection_orders"`
	FinalPositionCount int  `json:"final_position_count"`
	Blocked            bool `json:"blocked,omitempty"`
}

type Result struct {
	ScenarioName       string
	ProtectionOrders   int
	FinalPositionCount int
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
	latestPrice, _ := pt.GetMarketPrice(s.Symbol)
	latestFunding := 0.0
	if len(s.FundingRates) > 0 {
		latestFunding = s.FundingRates[len(s.FundingRates)-1]
	}
	marketData := &market.Data{
		CurrentPrice:   latestPrice,
		CurrentEMA20:   latestPrice,
		PriceChange4h:  computePriceChange4h(priceSeries),
		FundingRate:    latestFunding,
		IntradaySeries: &market.IntradayData{ATR14: latestPrice * 0.01},
	}

	for _, action := range s.Actions {
		if s.RegimeFilter != nil && s.RegimeFilter.Enabled {
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

	return &Result{
		ScenarioName:       s.Name,
		ProtectionOrders:   len(orders),
		FinalPositionCount: len(positions),
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
		regime = "trending"
	}
	if len(cfg.AllowedRegimes) > 0 {
		allowed := false
		for _, item := range cfg.AllowedRegimes {
			if strings.EqualFold(item, regime) {
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
		if strings.EqualFold(action, "open_long") && !(data.CurrentPrice >= data.CurrentEMA20 && data.PriceChange4h >= 0) {
			return true
		}
		if strings.EqualFold(action, "open_short") && !(data.CurrentPrice <= data.CurrentEMA20 && data.PriceChange4h <= 0) {
			return true
		}
	}
	return false
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
