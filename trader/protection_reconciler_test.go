package trader

import (
	"testing"

	"nofx/store"
	tradertypes "nofx/trader/types"
)

type fakeReconcileTrader struct {
	fakeOrderProtectionTrader
	positions      []map[string]interface{}
	cooldownBypass bool
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

func TestProtectionReconciler_RearmsBreakEvenOnQuantityChange(t *testing.T) {
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
				"markPrice":   106.0,
			},
		},
	}

	at := &AutoTrader{
		exchange: "okx",
		trader:   ft,
		config: AutoTraderConfig{
			StrategyConfig: &store.StrategyConfig{
				Protection: store.ProtectionConfig{
					BreakEvenStop: store.BreakEvenStopConfig{
						Enabled:      true,
						TriggerMode:  store.BreakEvenTriggerProfitPct,
						TriggerValue: 5,
						OffsetPct:    0,
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
		t.Fatalf("expected initial break-even stop placement, got %d", len(ft.stopLossOrders))
	}

	// Simulate resized position: quantity changes while break-even remained armed.
	ft.positions = []map[string]interface{}{
		{
			"symbol":      "BTCUSDT",
			"side":        "long",
			"entryPrice":  100.0,
			"positionAmt": 1.5,
			"markPrice":   106.0,
		},
	}
	before := len(ft.stopLossOrders)
	at.reconcilePositionProtections()
	if len(ft.stopLossOrders) != before+1 {
		t.Fatalf("expected break-even to re-arm on quantity change, got %d stop-loss orders", len(ft.stopLossOrders))
	}
	if ft.stopLossOrders[len(ft.stopLossOrders)-1].quantity != 1.5 {
		t.Fatalf("expected re-armed stop to use updated quantity 1.5, got %.2f", ft.stopLossOrders[len(ft.stopLossOrders)-1].quantity)
	}
}
