package trader

import (
	"fmt"
	"testing"
	"time"

	tradertypes "nofx/trader/types"
)

type fakeOrderProtectionTrader struct {
	stopLossOrders       []struct{ symbol, positionSide string; quantity, price float64 }
	takeProfitOrders     []struct{ symbol, positionSide string; quantity, price float64 }
	openOrders           []tradertypes.OpenOrder
	setStopLossErr       error
	setTakeProfitErr     error
	getOpenOrdersErr     error
}

func (f *fakeOrderProtectionTrader) GetBalance() (map[string]interface{}, error) { return nil, nil }
func (f *fakeOrderProtectionTrader) GetPositions() ([]map[string]interface{}, error) { return nil, nil }
func (f *fakeOrderProtectionTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) { return nil, nil }
func (f *fakeOrderProtectionTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) { return nil, nil }
func (f *fakeOrderProtectionTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) { return nil, nil }
func (f *fakeOrderProtectionTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) { return nil, nil }
func (f *fakeOrderProtectionTrader) SetLeverage(symbol string, leverage int) error { return nil }
func (f *fakeOrderProtectionTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (f *fakeOrderProtectionTrader) GetMarketPrice(symbol string) (float64, error) { return 0, nil }
func (f *fakeOrderProtectionTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	if f.setStopLossErr != nil {
		return f.setStopLossErr
	}
	f.stopLossOrders = append(f.stopLossOrders, struct{ symbol, positionSide string; quantity, price float64 }{symbol, positionSide, quantity, stopPrice})
	return nil
}
func (f *fakeOrderProtectionTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	if f.setTakeProfitErr != nil {
		return f.setTakeProfitErr
	}
	f.takeProfitOrders = append(f.takeProfitOrders, struct{ symbol, positionSide string; quantity, price float64 }{symbol, positionSide, quantity, takeProfitPrice})
	return nil
}
func (f *fakeOrderProtectionTrader) CancelStopLossOrders(symbol string) error { return nil }
func (f *fakeOrderProtectionTrader) CancelTakeProfitOrders(symbol string) error { return nil }
func (f *fakeOrderProtectionTrader) CancelAllOrders(symbol string) error { return nil }
func (f *fakeOrderProtectionTrader) CancelStopOrders(symbol string) error { return nil }
func (f *fakeOrderProtectionTrader) FormatQuantity(symbol string, quantity float64) (string, error) { return "", nil }
func (f *fakeOrderProtectionTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) { return nil, nil }
func (f *fakeOrderProtectionTrader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) { return nil, nil }
func (f *fakeOrderProtectionTrader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	if f.getOpenOrdersErr != nil {
		return nil, f.getOpenOrdersErr
	}
	return f.openOrders, nil
}

func TestPlaceAndVerifyLadderProtection(t *testing.T) {
	fakeTrader := &fakeOrderProtectionTrader{
		openOrders: []tradertypes.OpenOrder{
			{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98},
			{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 96},
			{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 105},
			{PositionSide: "LONG", Type: "TAKE_PROFIT_MARKET", StopPrice: 110},
		},
	}
	at := &AutoTrader{trader: fakeTrader}
	plan := &ProtectionPlan{
		StopLossOrders: []ProtectionOrder{{Price: 98, CloseRatioPct: 50}, {Price: 96, CloseRatioPct: 50}},
		TakeProfitOrders: []ProtectionOrder{{Price: 105, CloseRatioPct: 30}, {Price: 110, CloseRatioPct: 70}},
	}

	if err := at.placeAndVerifyLadderProtection("BTCUSDT", "LONG", 2, plan); err != nil {
		t.Fatalf("expected ladder protection success, got %v", err)
	}
	if len(fakeTrader.stopLossOrders) != 2 || len(fakeTrader.takeProfitOrders) != 2 {
		t.Fatalf("expected 2 stop-loss and 2 take-profit orders, got sl=%d tp=%d", len(fakeTrader.stopLossOrders), len(fakeTrader.takeProfitOrders))
	}
	if fakeTrader.stopLossOrders[0].quantity != 1 || fakeTrader.takeProfitOrders[0].quantity != 0.6 {
		t.Fatalf("unexpected ladder quantities, sl=%v tp=%v", fakeTrader.stopLossOrders[0].quantity, fakeTrader.takeProfitOrders[0].quantity)
	}
}

func TestPlaceAndVerifyProtectionPlanRejectsUnsupportedPartialClose(t *testing.T) {
	at := &AutoTrader{exchange: "lighter"}
	caps := at.GetProtectionCapabilities()
	if caps.NativePartialClose {
		t.Fatal("expected lighter partial close to be unsupported in safety matrix")
	}
}

func TestVerifyProtectionOrdersFailsOnMissingLadderLeg(t *testing.T) {
	orders := []tradertypes.OpenOrder{{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98}}
	err := verifyProtectionOrders(orders, "LONG", []ProtectionOrder{{Price: 98, CloseRatioPct: 50}, {Price: 96, CloseRatioPct: 50}}, false)
	if err == nil {
		t.Fatal("expected missing ladder leg verification error")
	}
	if err.Error() == "" {
		t.Fatal("expected non-empty verification error")
	}
}

func TestPlaceAndVerifyLadderProtectionReturnsSetError(t *testing.T) {
	fakeTrader := &fakeOrderProtectionTrader{setTakeProfitErr: fmt.Errorf("tp failed")}
	at := &AutoTrader{trader: fakeTrader}
	plan := &ProtectionPlan{TakeProfitOrders: []ProtectionOrder{{Price: 105, CloseRatioPct: 50}, {Price: 110, CloseRatioPct: 50}}}

	err := at.placeAndVerifyLadderProtection("BTCUSDT", "LONG", 2, plan)
	if err == nil {
		t.Fatal("expected ladder set error")
	}
}
