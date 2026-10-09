package trader

import (
	"errors"
	"testing"

	"nofx/kernel"
	"nofx/store"
	tradertypes "nofx/trader/types"
)

type emergencyCloseTestTrader struct {
	tradertypes.Trader
	closeCalls    int
	invalidations int
	cancelCalls   int
	positions     []map[string]interface{}
	openOrders    []tradertypes.OpenOrder
	cancelErr     error
}

func (t *emergencyCloseTestTrader) CloseLong(string, float64) (map[string]interface{}, error) {
	t.closeCalls++
	return map[string]interface{}{"status": "submitted"}, nil
}

func (t *emergencyCloseTestTrader) CloseShort(string, float64) (map[string]interface{}, error) {
	t.closeCalls++
	return map[string]interface{}{"status": "submitted"}, nil
}

func (t *emergencyCloseTestTrader) InvalidatePositionCache() { t.invalidations++ }
func (t *emergencyCloseTestTrader) GetPositions() ([]map[string]interface{}, error) {
	return t.positions, nil
}
func (t *emergencyCloseTestTrader) CancelAllOrders(string) error {
	t.cancelCalls++
	return t.cancelErr
}
func (t *emergencyCloseTestTrader) GetOpenOrders(string) ([]tradertypes.OpenOrder, error) {
	return t.openOrders, nil
}

func testVergexSignalTrader() *AutoTrader {
	cfg := store.GetDefaultStrategyConfig("en")
	cfg.CoinSource.SourceType = "vergex_signal"
	return &AutoTrader{
		config:         AutoTraderConfig{StrategyConfig: &cfg},
		strategyEngine: kernel.NewStrategyEngine(&cfg),
	}
}

func testSignalBias(values map[string]string) func(string) (string, bool) {
	return func(symbol string) (string, bool) {
		bias, ok := values[universeBaseKey(symbol)]
		return bias, ok
	}
}

func TestVergexSignalPolicyHoldsWhileDirectionIsUnchanged(t *testing.T) {
	decisions := []kernel.Decision{
		{Symbol: "xyz:NVDA", Action: "close_long"},
		{Symbol: "BTC", Action: "close_short"},
	}
	positions := []kernel.PositionInfo{
		{Symbol: "xyz:NVDA", Side: "long"},
		{Symbol: "BTC", Side: "short"},
	}

	got, blocked := applyVergexSignalPolicy(decisions, positions, testSignalBias(map[string]string{
		"NVDA": "bullish",
		"BTC":  "bearish",
	}))
	if len(got) != 2 || got[0].Action != "hold" || got[1].Action != "hold" ||
		got[0].Confidence != 100 || got[1].Confidence != 100 {
		t.Fatalf("unchanged long and short signals must force hold, got %+v", got)
	}
	if len(blocked) != 2 || blocked[0].Action != "close_long" || blocked[1].Action != "close_short" {
		t.Fatalf("AI closes must be blocked while signals remain unchanged, got %+v", blocked)
	}
}

func TestVergexSignalPolicyDoesNotFlagMatchingAIHoldAsConflict(t *testing.T) {
	decisions := []kernel.Decision{{Symbol: "xyz:NVDA", Action: "hold"}}
	positions := []kernel.PositionInfo{{Symbol: "xyz:NVDA", Side: "long"}}

	got, blocked := applyVergexSignalPolicy(decisions, positions, testSignalBias(map[string]string{"NVDA": "bullish"}))
	if len(got) != 1 || got[0].Action != "hold" {
		t.Fatalf("unchanged bullish signal must hold, got %+v", got)
	}
	if len(blocked) != 0 {
		t.Fatalf("matching AI hold is not a signal conflict, got %+v", blocked)
	}
}

func TestVergexSignalPolicyClosesWhenDirectionChangesOrDisappears(t *testing.T) {
	decisions := []kernel.Decision{
		{Symbol: "xyz:NVDA", Action: "open_short"},
		{Symbol: "BTC", Action: "open_long"},
	}
	positions := []kernel.PositionInfo{
		{Symbol: "xyz:NVDA", Side: "long"},
		{Symbol: "BTC", Side: "short"},
		{Symbol: "ETH", Side: "long"},
	}

	got, blocked := applyVergexSignalPolicy(decisions, positions, testSignalBias(map[string]string{
		"NVDA": "bearish",
		"ETH":  "neutral",
	}))
	if len(got) != 3 || got[0].Action != "close_long" || got[1].Action != "close_short" || got[2].Action != "close_long" {
		t.Fatalf("changed or absent signals must close positions, got %+v", got)
	}
	if len(blocked) != 2 {
		t.Fatalf("a reversed position must close without flipping in the same cycle, blocked=%+v", blocked)
	}
}

func TestVergexSignalPolicyAllowsOnlyMatchingEntries(t *testing.T) {
	decisions := []kernel.Decision{
		{Symbol: "xyz:NVDA", Action: "open_long"},
		{Symbol: "BTC", Action: "open_short"},
		{Symbol: "ETH", Action: "open_long"},
		{Symbol: "SOL", Action: "open_short"},
	}
	biases := map[string]string{
		"NVDA": "bullish",
		"BTC":  "bearish",
		"ETH":  "bearish",
	}

	got, blocked := applyVergexSignalPolicy(decisions, nil, testSignalBias(biases))
	if len(got) != 2 || got[0].Symbol != "xyz:NVDA" || got[1].Symbol != "BTC" {
		t.Fatalf("only direction-matched entries should pass, got %+v", got)
	}
	if len(blocked) != 2 {
		t.Fatalf("expected mismatched and absent entries to be blocked, got %+v", blocked)
	}
}

func TestVergexSignalPolicyDoesNotTreatMissingSnapshotAsSignalExit(t *testing.T) {
	at := testVergexSignalTrader()
	decisions := []kernel.Decision{{Symbol: "xyz:NVDA", Action: "hold"}}
	ctx := &kernel.Context{Positions: []kernel.PositionInfo{{Symbol: "xyz:NVDA", Side: "long"}}}

	got := at.enforceVergexSignalPolicy(decisions, ctx)
	if len(got) != 1 || got[0].Action != "hold" {
		t.Fatalf("missing board snapshot must leave decisions untouched, got %+v", got)
	}
}

func TestVergexSignalPolicyUsesSignalManagedProfitExit(t *testing.T) {
	at := testVergexSignalTrader()
	if !at.usesSignalManagedExit() {
		t.Fatal("Vergex strategy must use signal-managed ordinary exits")
	}
	at.config.StrategyConfig.CoinSource.SourceType = "static"
	if at.usesSignalManagedExit() {
		t.Fatal("non-Vergex strategies must keep their configured take-profit behavior")
	}
}

func TestLegacyVergexFieldsDoNotChangeFixedExitMode(t *testing.T) {
	at := testVergexSignalTrader()
	at.config.StrategyConfig.CoinSource.SourceType = "claw402"
	at.config.StrategyConfig.CoinSource.VergexLimit = 5
	at.config.StrategyConfig.CoinSource.VergexMarketType = "all"
	if at.usesSignalManagedExit() {
		t.Fatal("legacy Vergex fields must not turn a claw402 strategy into signal-managed mode")
	}
}

func TestValidateProtectionPrices(t *testing.T) {
	tests := []struct {
		name, action           string
		market, sl, tp         float64
		signalManaged, wantErr bool
	}{
		{"fixed long valid", "open_long", 100, 95, 120, false, false},
		{"signal long valid", "open_long", 100, 95, 0, true, false},
		{"long stop above market", "open_long", 100, 101, 120, false, true},
		{"long target below market", "open_long", 100, 95, 99, false, true},
		{"fixed short valid", "open_short", 100, 105, 80, false, false},
		{"signal short valid", "open_short", 100, 105, 0, true, false},
		{"short stop below market", "open_short", 100, 99, 80, false, true},
		{"short target above market", "open_short", 100, 105, 101, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProtectionPrices(tt.action, tt.market, tt.sl, tt.tp, tt.signalManaged)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestEmergencyCloseUsesFreshPositionsAndVerifiesOrderCleanup(t *testing.T) {
	fake := &emergencyCloseTestTrader{}
	at := &AutoTrader{trader: fake}
	if err := at.emergencyClosePositionAndVerify("BTCUSDT", "long", 1); err != nil {
		t.Fatalf("emergency close failed: %v", err)
	}
	if fake.closeCalls != 1 || fake.invalidations != 1 || fake.cancelCalls != 1 {
		t.Fatalf("calls close=%d invalidate=%d cancel=%d, want 1 each", fake.closeCalls, fake.invalidations, fake.cancelCalls)
	}
}

func TestEmergencyCloseFailsIfProtectionOrdersRemain(t *testing.T) {
	fake := &emergencyCloseTestTrader{openOrders: []tradertypes.OpenOrder{{Symbol: "BTCUSDT"}}}
	at := &AutoTrader{trader: fake}
	if err := at.emergencyClosePositionAndVerify("BTCUSDT", "long", 1); err == nil {
		t.Fatal("expected remaining order to fail closed")
	}
}

func TestEmergencyCloseFailsIfOrderCleanupFails(t *testing.T) {
	fake := &emergencyCloseTestTrader{cancelErr: errors.New("cancel rejected")}
	at := &AutoTrader{trader: fake}
	if err := at.emergencyClosePositionAndVerify("BTCUSDT", "long", 1); err == nil {
		t.Fatal("expected cleanup error to fail closed")
	}
}
