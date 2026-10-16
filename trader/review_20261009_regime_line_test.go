package trader

import (
	"math"
	"strings"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

func reviewRegimeTraderData(now time.Time, long bool, live, line float64) *market.Data {
	data := &market.Data{Symbol: "XRPUSDT", CurrentPrice: live, TimeframeData: map[string]*market.TimeframeSeriesData{}}
	for _, tf := range []string{"15m", "1h", "4h"} {
		td := &market.TimeframeSeriesData{Timeframe: tf}
		for i := 0; i < 60; i++ {
			p := 1.70 - 0.007*float64(i)
			if long {
				p = 1.08 + 0.007*float64(i)
			}
			if tf == "15m" && i == 59 {
				p = 1.3829
			}
			td.Klines = append(td.Klines, market.KlineBar{Time: now.Add(time.Duration(i-60) * market.TimeframeDuration(tf)).UnixMilli(), Open: p, High: p + 0.002, Low: p - 0.002, Close: p, Volume: 1000})
			td.EMA50Values = append(td.EMA50Values, line)
		}
		data.TimeframeData[tf] = td
	}
	return data
}

func TestReview20261009RegimeLineTraderParity(t *testing.T) {
	for _, tc := range []struct {
		name               string
		long               bool
		live, line, anchor float64
		blocked            bool
	}{
		{"XRP replay", false, 1.3829, 1.417, 1.39141, false},
		{"short anchor broken", false, 1.3829, 1.417, 1.43, true},
		{"short live broken", false, 1.43, 1.417, 1.44, true},
		{"long valid", true, 1.3829, 1.35, 1.37, false},
		{"long anchor broken", true, 1.3829, 1.35, 1.34, true},
		{"long live broken", true, 1.34, 1.35, 1.33, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			data := reviewRegimeTraderData(now, tc.long, tc.live, tc.line)
			sig, err := kernel.ComputeSymbolSignals(data.Symbol, data, kernel.SignalOptions{Now: now, PrimaryTF: "15m", EntryTimingGate: true, LimitEntryEnabled: true, LimitEntryOffsetMode: "fixed", LimitEntryOffsetPct: math.Abs(tc.anchor/tc.live-1) * 100})
			if err != nil {
				t.Fatal(err)
			}
			g, anchor, action := sig.HardGate.Short, sig.LimitSellPrice, "open_short_limit"
			if tc.long {
				g, anchor, action = sig.HardGate.Long, sig.LimitBuyPrice, "open_long_limit"
			}
			if g.Allowed == tc.blocked {
				t.Fatalf("kernel verdict: %+v", g)
			}
			if !tc.blocked && math.Abs(anchor-tc.anchor) > 1e-9 {
				t.Fatalf("kernel anchor changed: %g", anchor)
			}
			d := kernel.Decision{Symbol: data.Symbol, Action: action, Price: tc.anchor}
			ctx := &kernel.Context{MarketDataMap: map[string]*market.Data{data.Symbol: data}, CandidateCoins: []kernel.CandidateCoin{{Symbol: data.Symbol}}}
			at := riskTestTrader(store.RiskControlConfig{EntryTimingGate: true})
			at.cycleGateStates = map[string]*kernel.GateState{data.Symbol: {LongAllowed: sig.HardGate.Long.Allowed, ShortAllowed: sig.HardGate.Short.Allowed, LongFailed: sig.HardGate.Long.Failed, ShortFailed: sig.HardGate.Short.Failed}}
			if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); (len(got) == 0) != tc.blocked {
				t.Fatalf("kernel/trader disagreement: got %v, gate %+v", got, g)
			}
			// Independently exercise the backstop with a stale permissive cycle
			// verdict so an upstream kernel-code rejection cannot mask it.
			at = riskTestTrader(store.RiskControlConfig{EntryTimingGate: true})
			at.cycleGateStates = map[string]*kernel.GateState{data.Symbol: {LongAllowed: true, ShortAllowed: true}}
			if got := at.applyHardRiskGates([]kernel.Decision{d}, ctx); (len(got) == 0) != tc.blocked {
				t.Fatalf("backstop disagreement: got %v", got)
			}
			reason := at.popFilterReason(d)
			if tc.blocked && (!strings.Contains(reason, "1h regime line (EMA50)") || !strings.Contains(reason, "锚位")) {
				t.Fatalf("backstop omitted reason: %q", reason)
			}
			if !tc.blocked && reason != "" {
				t.Fatalf("allowed order recorded block: %q", reason)
			}
		})
	}
}

func TestReview20261009RegimeLineTraderFillSiteAndTrigger(t *testing.T) {
	now := time.Now()
	data := reviewRegimeTraderData(now, true, 1.34, 1.35)
	at := riskTestTrader(store.RiskControlConfig{EntryTimingGate: true})
	at.cycleGateStates = map[string]*kernel.GateState{data.Symbol: {LongAllowed: true}}
	ctx := &kernel.Context{MarketDataMap: map[string]*market.Data{data.Symbol: data}, CandidateCoins: []kernel.CandidateCoin{{Symbol: data.Symbol}}}
	limit := kernel.Decision{Symbol: data.Symbol, Action: "open_long_limit", Price: 1.37}
	if got := at.applyHardRiskGates([]kernel.Decision{limit}, ctx); len(got) != 1 {
		t.Fatal("backstop must use the limit fill site")
	}
	marketOrder := kernel.Decision{Symbol: data.Symbol, Action: "open_long", Price: 1.37}
	if got := at.applyHardRiskGates([]kernel.Decision{marketOrder}, ctx); len(got) != 0 {
		t.Fatal("market backstop must use live price")
	}
	if reason := at.popFilterReason(marketOrder); !strings.Contains(reason, "1h regime line") {
		t.Fatalf("market reason: %q", reason)
	}
	data = reviewRegimeTraderData(now, false, 1.43, 1.417)
	data.TimeframeData["15m"].Klines[59].Close = 1.287
	ctx.MarketDataMap[data.Symbol] = data
	at.cycleGateStates[data.Symbol].ShortAllowed = true
	if got := at.applyHardRiskGates([]kernel.Decision{{Symbol: data.Symbol, Action: "open_short_limit", Price: 1.44}}, ctx); len(got) != 1 {
		t.Fatal("down trend must not trigger rally guard")
	}
	if !absoluteBanCode("REGIME_LINE_BROKEN") {
		t.Fatal("live reversal must have no market exception")
	}
}
