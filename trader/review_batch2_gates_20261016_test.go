package trader

import (
	"strings"
	"testing"

	"nofx/kernel"
	"nofx/store"
)

func batch2GatesContext() *kernel.Context {
	return &kernel.Context{
		CandidateCoins: []kernel.CandidateCoin{{Symbol: "BTCUSDT"}},
	}
}

func TestBatch2GatesCandidatePool(t *testing.T) {
	for _, limitEnabled := range []bool{false, true} {
		for _, action := range []string{"open_long", "open_short", "open_long_limit", "open_short_limit"} {
			t.Run(action+map[bool]string{false: "/market_mode", true: "/limit_mode"}[limitEnabled], func(t *testing.T) {
				at := riskTestTrader(store.RiskControlConfig{LimitEntryEnabled: limitEnabled})
				// A permissive direction state must not grant pool membership.
				at.cycleGateStates = map[string]*kernel.GateState{"SOLUSDT": {LongAllowed: true, ShortAllowed: true}}
				ctx := batch2GatesContext()
				// The pre-fetch list can contain candidates subsequently removed.
				ctx.CandidateOrder = []string{"BTCUSDT", "SOLUSDT"}
				d := kernel.Decision{Symbol: "sol_usdt", Action: action, Confidence: 95}
				if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 0 {
					t.Fatalf("out-of-pool open survived: %+v", got)
				}
				summaries := at.buildCoTSummaries([]kernel.Decision{d}, nil)
				if len(summaries) != 1 || !strings.Contains(summaries[0].ErrText, "NOT_CANDIDATE") {
					t.Fatalf("candidate rejection missing from reporting: %+v", summaries)
				}
			})
		}
	}
}

func TestBatch2GatesExcludedCoinOverridesPoolAndOwnership(t *testing.T) {
	for _, inPool := range []bool{false, true} {
		for _, action := range []string{"open_long", "open_short", "open_long_limit", "open_short_limit"} {
			at := riskTestTrader(store.RiskControlConfig{})
			at.config.StrategyConfig.CoinSource.ExcludedCoins = []string{"sol"}
			at.cycleGateStates = map[string]*kernel.GateState{"SOLUSDT": {LongAllowed: true, ShortAllowed: true}}
			ctx := batch2GatesContext()
			if inPool {
				ctx.CandidateCoins = append(ctx.CandidateCoins, kernel.CandidateCoin{Symbol: "SOLUSDT"})
				ctx.Positions = []kernel.PositionInfo{{Symbol: "SOLUSDT", Side: "long", Managed: true, Quantity: 1}}
			}
			d := kernel.Decision{Symbol: "SOL-USDT-SWAP", Action: action, Confidence: 95}
			if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 0 {
				t.Fatalf("excluded open survived (inPool=%v): %+v", inPool, got)
			}
			if reason := at.popFilterReason(d); !strings.Contains(reason, "EXCLUDED_COIN") {
				t.Fatalf("excluded open reported %q (inPool=%v)", reason, inPool)
			}
		}
	}
}

func TestBatch2GatesCurrentPoolAndAIPositionAdds(t *testing.T) {
	for _, tc := range []struct {
		name       string
		candidates []kernel.CandidateCoin
		positions  []kernel.PositionInfo
		wantCode   string
	}{
		{name: "current candidate without gate state", candidates: []kernel.CandidateCoin{{Symbol: "sol"}}},
		{name: "held AI position without gate state", positions: []kernel.PositionInfo{{Symbol: "SOL-USDT-SWAP", Side: "long", Managed: true, Quantity: 1}}},
		{name: "manual position", positions: []kernel.PositionInfo{{Symbol: "SOLUSDT", Side: "long", Quantity: 1}}, wantCode: "NOT_CANDIDATE"},
		{name: "stale AI ownership with no position", positions: []kernel.PositionInfo{{Symbol: "SOLUSDT", Side: "long", Managed: true}}, wantCode: "NOT_CANDIDATE"},
		{name: "empty current pool", wantCode: "NOT_CANDIDATE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := riskTestTrader(store.RiskControlConfig{})
			ctx := &kernel.Context{CandidateCoins: tc.candidates, Positions: tc.positions}
			d := kernel.Decision{Symbol: "sol_usdt", Action: "open_long", Price: 100, Confidence: 95}
			got := at.applyHardRiskGates([]kernel.Decision{d}, ctx)
			if tc.wantCode == "" {
				if len(got) != 1 || at.popFilterReason(d) != "" {
					t.Fatalf("legitimate market open/add rejected: %+v", got)
				}
				// Pool membership does not bypass the existing limit-state gate.
				d.Action = "open_long_limit"
				if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 0 {
					t.Fatalf("limit open without direction state survived: %+v", got)
				}
				if reason := at.popFilterReason(d); !strings.Contains(reason, "NO_GATE_STATE") {
					t.Fatalf("expected original limit-state rejection, got %q", reason)
				}
			} else if len(got) != 0 || !strings.Contains(at.popFilterReason(d), tc.wantCode) {
				t.Fatalf("expected %s, got %+v", tc.wantCode, got)
			}
		})
	}

	// Every invocation uses its own pool, even on the same trader instance.
	at := riskTestTrader(store.RiskControlConfig{})
	d := kernel.Decision{Symbol: "SOLUSDT", Action: "open_long", Price: 100}
	ctx := &kernel.Context{CandidateCoins: []kernel.CandidateCoin{{Symbol: "SOLUSDT"}}}
	if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); len(got) != 1 {
		t.Fatal("current candidate rejected")
	}
	if got := at.applyHardRiskGates([]kernel.Decision{d}, batch2GatesContext()); len(got) != 0 || !strings.Contains(at.popFilterReason(d), "NOT_CANDIDATE") {
		t.Fatal("previous cycle's candidate still authorized")
	}
}

func TestBatch2GatesNonOpenActionsUnaffected(t *testing.T) {
	at := riskTestTrader(store.RiskControlConfig{})
	at.config.StrategyConfig.CoinSource.ExcludedCoins = []string{"SOLUSDT"}
	ds := []kernel.Decision{
		{Symbol: "SOLUSDT", Action: "hold"},
		{Symbol: "SOLUSDT", Action: "wait"},
		{Symbol: "SOLUSDT", Action: "adjust_stop_loss"},
		{Symbol: "SOLUSDT", Action: "close_long"},
		{Symbol: "SOLUSDT", Action: "close_short"},
	}
	if got := at.applyHardRiskGates(ds, batch2GatesContext()); len(got) != len(ds) {
		t.Fatalf("candidate gate affected management actions: %+v", got)
	}
}

func TestBatch2GatesDowngradeRequiresLimitAuthorization(t *testing.T) {
	for _, side := range []string{"long", "short"} {
		for _, tc := range []struct {
			name     string
			allowed  bool
			failed   []string
			wantCode string
		}{
			{name: "direction blocked despite live anchor", failed: []string{"EXTENDED_PUMP_UNCONFIRMED"}, wantCode: "DIRECTION_DISALLOWED"},
			{name: "disallowed without codes", wantCode: "DIRECTION_DISALLOWED"},
			{name: "vendor divergence", allowed: true, failed: []string{"VENDOR_DIVERGENCE_UNKNOWN"}, wantCode: "VENDOR_DIVERGENCE_UNKNOWN"},
			{name: "authorized", allowed: true},
		} {
			t.Run(side+"/"+tc.name, func(t *testing.T) {
				at := riskTestTrader(store.RiskControlConfig{LimitEntryEnabled: true})
				gs := &kernel.GateState{LongAllowed: true, ShortAllowed: true}
				anchor := 99.5
				if side == "long" {
					gs.LongAllowed, gs.LongFailed = tc.allowed, tc.failed
					gs.LongLimitAllowed, gs.LongEntryPrice = true, anchor
				} else {
					anchor = 100.5
					gs.ShortAllowed, gs.ShortFailed = tc.allowed, tc.failed
					gs.ShortLimitAllowed, gs.ShortEntryPrice = true, anchor
				}
				at.cycleGateStates = map[string]*kernel.GateState{"SOLUSDT": gs}
				ctx := &kernel.Context{CandidateCoins: []kernel.CandidateCoin{{Symbol: "sol"}}}
				d := kernel.Decision{Symbol: "SOLUSDT", Action: "open_" + side, Confidence: 95}
				direct := d
				direct.Action += "_limit"
				direct.Price = anchor
				got := at.applyHardRiskGates([]kernel.Decision{direct}, ctx)
				if (len(got) == 0) != (tc.wantCode != "") {
					t.Fatalf("direct-limit verdict differs: %+v", got)
				}
				if tc.wantCode != "" {
					if reason := at.popFilterReason(direct); !strings.Contains(reason, tc.wantCode) {
						t.Fatalf("direct-limit rejection: %q", reason)
					}
					// Exercise the actual dispatcher: a nil exchange would panic if
					// an unauthorized conversion reached execution or market fallback.
					err := at.executeDecisionWithRecord(&d, &store.DecisionAction{})
					if err == nil || !strings.Contains(err.Error(), tc.wantCode) {
						t.Fatalf("unauthorized downgrade returned %v", err)
					}
					if d.Action != "open_"+side || d.Price != 0 || d.MarketDegraded {
						t.Fatalf("rejected decision was mutated: %+v", d)
					}
					if reason := at.popFilterReason(d); !strings.Contains(reason, tc.wantCode) {
						t.Fatalf("downgrade rejection missing from reporting: %q", reason)
					}
				} else {
					out, err := at.marketExceptionGate(&d, &store.DecisionAction{}, side == "long")
					if err != nil || out.Action != direct.Action || out.Price != anchor || !out.MarketDegraded {
						t.Fatalf("authorized downgrade changed: %+v, err=%v", out, err)
					}
				}
			})
		}
	}
}
