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
	if len(fakeTrader.stopLossOrders) != 0 || len(fakeTrader.takeProfitOrders) != 0 {
		t.Fatalf("expected no duplicate ladder orders when equivalent open orders already exist, got sl=%d tp=%d", len(fakeTrader.stopLossOrders), len(fakeTrader.takeProfitOrders))
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

func TestPlaceAndVerifyProtectionWithRetryRecoversOnSecondAttempt(t *testing.T) {
	fakeTrader := &fakeOrderProtectionTrader{
		openOrders: []tradertypes.OpenOrder{{PositionSide: "LONG", Type: "STOP_MARKET", StopPrice: 98}},
	}
	at := &AutoTrader{trader: &retryOnceProtectionTrader{inner: fakeTrader, failFirstStopLoss: true}}

	err := at.placeAndVerifyProtectionWithRetry("BTCUSDT", "LONG", 1, true, 98, false, 0)
	if err != nil {
		t.Fatalf("expected retry recovery, got %v", err)
	}
	wrapped := at.trader.(*retryOnceProtectionTrader)
	if wrapped.stopLossCalls != 2 {
		t.Fatalf("expected 2 stop-loss attempts, got %d", wrapped.stopLossCalls)
	}
}

type retryOnceProtectionTrader struct {
	inner             *fakeOrderProtectionTrader
	failFirstStopLoss bool
	stopLossCalls     int
}

func (r *retryOnceProtectionTrader) GetBalance() (map[string]interface{}, error) { return r.inner.GetBalance() }
func (r *retryOnceProtectionTrader) GetPositions() ([]map[string]interface{}, error) { return r.inner.GetPositions() }
func (r *retryOnceProtectionTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return r.inner.OpenLong(symbol, quantity, leverage)
}
func (r *retryOnceProtectionTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return r.inner.OpenShort(symbol, quantity, leverage)
}
func (r *retryOnceProtectionTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return r.inner.CloseLong(symbol, quantity)
}
func (r *retryOnceProtectionTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return r.inner.CloseShort(symbol, quantity)
}
func (r *retryOnceProtectionTrader) SetLeverage(symbol string, leverage int) error { return r.inner.SetLeverage(symbol, leverage) }
func (r *retryOnceProtectionTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	return r.inner.SetMarginMode(symbol, isCrossMargin)
}
func (r *retryOnceProtectionTrader) GetMarketPrice(symbol string) (float64, error) { return r.inner.GetMarketPrice(symbol) }
func (r *retryOnceProtectionTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	r.stopLossCalls++
	if r.failFirstStopLoss && r.stopLossCalls == 1 {
		return fmt.Errorf("temporary stop loss error")
	}
	return r.inner.SetStopLoss(symbol, positionSide, quantity, stopPrice)
}
func (r *retryOnceProtectionTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return r.inner.SetTakeProfit(symbol, positionSide, quantity, takeProfitPrice)
}
func (r *retryOnceProtectionTrader) CancelStopLossOrders(symbol string) error { return r.inner.CancelStopLossOrders(symbol) }
func (r *retryOnceProtectionTrader) CancelTakeProfitOrders(symbol string) error { return r.inner.CancelTakeProfitOrders(symbol) }
func (r *retryOnceProtectionTrader) CancelAllOrders(symbol string) error { return r.inner.CancelAllOrders(symbol) }
func (r *retryOnceProtectionTrader) CancelStopOrders(symbol string) error { return r.inner.CancelStopOrders(symbol) }
func (r *retryOnceProtectionTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	return r.inner.FormatQuantity(symbol, quantity)
}
func (r *retryOnceProtectionTrader) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return r.inner.GetOrderStatus(symbol, orderID)
}
func (r *retryOnceProtectionTrader) GetClosedPnL(startTime time.Time, limit int) ([]tradertypes.ClosedPnLRecord, error) {
	return r.inner.GetClosedPnL(startTime, limit)
}
func (r *retryOnceProtectionTrader) GetOpenOrders(symbol string) ([]tradertypes.OpenOrder, error) {
	return r.inner.GetOpenOrders(symbol)
}
