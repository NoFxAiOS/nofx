package trader

import (
	"testing"

	"nofx/store"
	tradertypes "nofx/trader/types"
)

type fakeReconcileTrader struct {
	fakeOrderProtectionTrader
	positions []map[string]interface{}
}

func (f *fakeReconcileTrader) GetPositions() ([]map[string]interface{}, error) {
	return f.positions, nil
}

func (f *fakeReconcileTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	if err := f.fakeOrderProtectionTrader.SetStopLoss(symbol, positionSide, quantity, stopPrice); err != nil {
		return err
	}
	f.openOrders = append(f.openOrders, tradertypes.OpenOrder{
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "STOP_MARKET",
		StopPrice:    stopPrice,
		Quantity:     quantity,
	})
	return nil
}

func (f *fakeReconcileTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	if err := f.fakeOrderProtectionTrader.SetTakeProfit(symbol, positionSide, quantity, takeProfitPrice); err != nil {
		return err
	}
	f.openOrders = append(f.openOrders, tradertypes.OpenOrder{
		Symbol:       symbol,
		PositionSide: positionSide,
		Type:         "TAKE_PROFIT_MARKET",
		StopPrice:    takeProfitPrice,
		Quantity:     quantity,
	})
	return nil
}

func TestProtectionReconciler_ReappliesMissingManualOrders(t *testing.T) {
	ft := &fakeReconcileTrader{
		fakeOrderProtectionTrader: fakeOrderProtectionTrader{
			openOrders: []tradertypes.OpenOrder{},
		},
		positions: []map[string]interface{}{
			{
				"symbol":      "BTCUSDT",
				"side":        "long",
				"entryPrice":  100.0,
				"positionAmt": 1.0,
			},
		},
	}

	at := &AutoTrader{
		exchange: "okx",
		trader:   ft,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					FullTPSL: store.FullTPSLConfig{
						Enabled: true,
						Mode:    store.ProtectionModeManual,
						StopLoss: store.ProtectionThresholdRule{
							Enabled:      true,
							PriceMovePct: 2,
						},
						TakeProfit: store.ProtectionThresholdRule{
							Enabled:      true,
							PriceMovePct: 5,
						},
					},
				},
			},
		},
		protectionState:       make(map[string]string),
		breakEvenState:        make(map[string]string),
		breakEvenFingerprints: make(map[string]string),
	}

	at.reconcilePositionProtections()

	if len(ft.stopLossOrders) != 1 {
		t.Fatalf("expected reconciler to re-apply 1 stop loss, got %d", len(ft.stopLossOrders))
	}
	if len(ft.takeProfitOrders) != 1 {
		t.Fatalf("expected reconciler to re-apply 1 take profit, got %d", len(ft.takeProfitOrders))
	}
}
