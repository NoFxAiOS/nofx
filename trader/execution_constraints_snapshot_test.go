package trader

import (
	"testing"
	"time"
)

type stubExecutionConstraintsTrader struct {
	constraints map[string]float64
	price       float64
}

func (s *stubExecutionConstraintsTrader) GetBalance() (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) GetPositions() ([]map[string]interface{}, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) SetLeverage(symbol string, leverage int) error { return nil }
func (s *stubExecutionConstraintsTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return nil
}
func (s *stubExecutionConstraintsTrader) GetMarketPrice(symbol string) (float64, error) {
	return s.price, nil
}
func (s *stubExecutionConstraintsTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}
func (s *stubExecutionConstraintsTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}
func (s *stubExecutionConstraintsTrader) CancelStopLossOrders(symbol string) error   { return nil }
func (s *stubExecutionConstraintsTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (s *stubExecutionConstraintsTrader) CancelAllOrders(symbol string) error        { return nil }
func (s *stubExecutionConstraintsTrader) CancelStopOrders(symbol string) error       { return nil }
func (s *stubExecutionConstraintsTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return "", nil
}
func (s *stubExecutionConstraintsTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	return nil, nil
}
func (s *stubExecutionConstraintsTrader) GetExecutionConstraints(symbol string) (map[string]float64, error) {
	return s.constraints, nil
}
func (s *stubExecutionConstraintsTrader) GetSymbolPricePrecision(symbol string) (int, error) {
	return 2, nil
}
func (s *stubExecutionConstraintsTrader) GetSymbolPrecision(symbol string) (int, error) {
	return 3, nil
}

func TestCollectExecutionConstraintsSnapshotBinanceUsesCompactFieldsOnly(t *testing.T) {
	at := &AutoTrader{
		exchange: "binance",
		trader: &stubExecutionConstraintsTrader{
			price: 123.45,
			constraints: map[string]float64{
				"tick_size":     0.01,
				"qty_step_size": 0.001,
				"min_qty":       0.001,
				"min_notional":  5,
			},
		},
	}

	snap := at.collectExecutionConstraintsSnapshot("BTCUSDT")
	if snap == nil {
		t.Fatal("expected snapshot")
	}
	if snap.TickSize != 0.01 || snap.QtyStepSize != 0.001 || snap.MinQty != 0.001 || snap.MinNotional != 5 {
		t.Fatalf("unexpected instrument constraints: %+v", snap)
	}
	if snap.LastPrice != 123.45 {
		t.Fatalf("expected last price, got %+v", snap)
	}
	if snap.BestBid != 0 || snap.BestAsk != 0 || snap.SpreadBps != 0 {
		t.Fatalf("did not expect top-of-book fields for default Binance profile: %+v", snap)
	}
}

func TestBuildDecisionActionReviewContextIncludesExecutionConstraints(t *testing.T) {
	snap := &ExecutionConstraintsSnapshot{TickSize: 0.1, LastPrice: 100, MinNotional: 10}
	ctx := buildDecisionActionReviewContext(nil, 0, nil, snap)
	if ctx == nil || ctx.ExecutionConstraints == nil {
		t.Fatalf("expected execution constraints in review context, got %+v", ctx)
	}
	if ctx.ExecutionConstraints.TickSize != 0.1 || ctx.ExecutionConstraints.LastPrice != 100 || ctx.ExecutionConstraints.MinNotional != 10 {
		t.Fatalf("unexpected execution constraints: %+v", ctx.ExecutionConstraints)
	}
}
