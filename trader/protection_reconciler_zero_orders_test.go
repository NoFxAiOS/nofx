package trader

import (
	"errors"
	"testing"

	"nofx/store"
	tradertypes "nofx/trader/types"
)

func TestProtectionReconciler_DoesNotMarkVerifiedWhenZeroOrdersAfterReapplyFailure(t *testing.T) {
	ft := &fakeReconcileTrader{
		fakeOrderProtectionTrader: fakeOrderProtectionTrader{openOrders: []tradertypes.OpenOrder{}},
		positions: []map[string]interface{}{{
			"symbol":      "BTCUSDT",
			"side":        "short",
			"entryPrice":  100.0,
			"positionAmt": -1.0,
			"markPrice":   100.0,
		}},
	}
	ft.setStopLossErr = errors.New("set stop loss failed")

	at := &AutoTrader{
		exchange: "okx",
		trader:   ft,
		config: AutoTraderConfig{StrategyConfig: &store.StrategyConfig{Protection: store.ProtectionConfig{FullTPSL: store.FullTPSLConfig{
			Enabled: true,
			Mode:    store.ProtectionModeManual,
			StopLoss: store.ProtectionValueSource{
				Mode:  store.ProtectionValueModeManual,
				Value: 1,
			},
		}}}},
		protectionState: make(map[string]string),
		breakEvenState:  make(map[string]string),
	}

	at.reconcilePositionProtections()
	state := at.getProtectionState("BTCUSDT", "short")
	if state == "exchange_protection_verified" {
		t.Fatalf("must not mark exchange protection verified when zero orders cannot be repaired")
	}
	if state == "" || state == "native_trailing_armed" {
		t.Fatalf("expected explicit reconcile failure state, got %q", state)
	}
}
