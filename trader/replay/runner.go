package replay

import (
	"encoding/json"
	"fmt"
	"os"

	"nofx/trader/paper"
)

type Scenario struct {
	Name         string           `json:"name"`
	Symbol       string           `json:"symbol"`
	InitialPrice float64          `json:"initial_price"`
	Prices       []float64        `json:"prices"`
	Actions      []ScenarioAction `json:"actions"`
	Expected     ScenarioExpected `json:"expected"`
}

type ScenarioAction struct {
	Type     string  `json:"type"`
	Quantity float64 `json:"quantity"`
	Leverage int     `json:"leverage"`
}

type ScenarioExpected struct {
	ProtectionOrders   int `json:"protection_orders"`
	FinalPositionCount int `json:"final_position_count"`
}

type Result struct {
	ScenarioName       string
	ProtectionOrders   int
	FinalPositionCount int
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
	if s.InitialPrice > 0 {
		pt.SetPrice(s.Symbol, s.InitialPrice)
	}
	for _, price := range s.Prices {
		if price > 0 {
			pt.SetPrice(s.Symbol, price)
		}
	}

	for _, action := range s.Actions {
		switch action.Type {
		case "open_long":
			if _, err := pt.OpenLong(s.Symbol, action.Quantity, action.Leverage); err != nil {
				return nil, err
			}
			entryPrice, _ := pt.GetMarketPrice(s.Symbol)
			_ = pt.SetStopLoss(s.Symbol, "LONG", action.Quantity, entryPrice*0.98)
			_ = pt.SetTakeProfit(s.Symbol, "LONG", action.Quantity, entryPrice*1.05)
		case "open_short":
			if _, err := pt.OpenShort(s.Symbol, action.Quantity, action.Leverage); err != nil {
				return nil, err
			}
			entryPrice, _ := pt.GetMarketPrice(s.Symbol)
			_ = pt.SetStopLoss(s.Symbol, "SHORT", action.Quantity, entryPrice*1.02)
			_ = pt.SetTakeProfit(s.Symbol, "SHORT", action.Quantity, entryPrice*0.95)
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
	}, nil
}

func ValidateResult(s *Scenario, result *Result) error {
	if s == nil || result == nil {
		return fmt.Errorf("scenario and result are required")
	}
	if s.Expected.ProtectionOrders > 0 && result.ProtectionOrders != s.Expected.ProtectionOrders {
		return fmt.Errorf("expected protection orders %d, got %d", s.Expected.ProtectionOrders, result.ProtectionOrders)
	}
	if result.FinalPositionCount != s.Expected.FinalPositionCount {
		return fmt.Errorf("expected final position count %d, got %d", s.Expected.FinalPositionCount, result.FinalPositionCount)
	}
	return nil
}
