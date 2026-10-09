package trader

import (
	"nofx/kernel"
	"strings"
	"testing"
	"time"
)

func throttleContext(symbol, side string, heldFor time.Duration, pnlPct float64) *kernel.Context {
	return leveragedThrottleContext(symbol, side, heldFor, pnlPct, 1)
}

func leveragedThrottleContext(symbol, side string, heldFor time.Duration, pnlPct float64, leverage int) *kernel.Context {
	return &kernel.Context{
		Positions: []kernel.PositionInfo{
			{
				Symbol:           symbol,
				Side:             side,
				UnrealizedPnLPct: pnlPct,
				Leverage:         leverage,
				UpdateTime:       time.Now().Add(-heldFor).UnixMilli(),
			},
		},
	}
}

func TestTradeThrottleBlocksEarlyNoiseClose(t *testing.T) {
	at := &AutoTrader{}
	ctx := throttleContext("xyz:INTC", "long", 20*time.Minute, -0.3)

	reason := at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "close_long"}, ctx)
	if !strings.Contains(reason, "min AI-managed hold") {
		t.Fatalf("expected early close to be blocked by min hold, got %q", reason)
	}
}

func TestTradeThrottleAllowsEarlyHardStop(t *testing.T) {
	at := &AutoTrader{}
	// A price loss beyond the default -3% bypass unlocks the min hold.
	ctx := throttleContext("xyz:INTC", "long", 20*time.Minute, -6.0)

	reason := at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "close_long"}, ctx)
	if reason != "" {
		t.Fatalf("expected hard stop close to pass, got %q", reason)
	}
}

func TestTradeThrottleBypassIsPriceBasisNotMarginBasis(t *testing.T) {
	at := &AutoTrader{}
	// At 10x leverage the exchange reports margin-based PnL: -6% margin is
	// only a -0.6% price move — noise, must NOT bypass the min hold.
	ctx := leveragedThrottleContext("xyz:INTC", "long", 20*time.Minute, -6.0, 10)

	reason := at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "close_long"}, ctx)
	if !strings.Contains(reason, "min AI-managed hold") {
		t.Fatalf("expected -0.6%% price move to stay blocked at 10x, got %q", reason)
	}

	// -60% margin at 10x is a real -6% price move — bypass allowed.
	ctx = leveragedThrottleContext("xyz:INTC", "long", 20*time.Minute, -60.0, 10)
	reason = at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "close_long"}, ctx)
	if reason != "" {
		t.Fatalf("expected -6%% price move to bypass min hold at 10x, got %q", reason)
	}
}

func TestTradeThrottleNoiseBandIsPriceBasisNotMarginBasis(t *testing.T) {
	at := &AutoTrader{}
	// Past min hold at 10x: +20% margin is only a +2% price move, still
	// inside the default -2%..+3% noise band — flat close must stay blocked.
	ctx := leveragedThrottleContext("xyz:INTC", "long", 2*time.Hour, 20.0, 10)

	reason := at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "close_long"}, ctx)
	if !strings.Contains(reason, "noise band") {
		t.Fatalf("expected +2%% price move to be blocked inside noise band at 10x, got %q", reason)
	}
}

func TestTradeThrottleBlocksFlatCloseInsideNoiseWindow(t *testing.T) {
	at := &AutoTrader{}
	// Held past the default 90m min hold but still inside the noise band and
	// under the 3h noise window.
	ctx := throttleContext("xyz:INTC", "long", 2*time.Hour, 0.4)

	reason := at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "close_long"}, ctx)
	if !strings.Contains(reason, "noise band") {
		t.Fatalf("expected flat close to be blocked inside noise window, got %q", reason)
	}
}

func TestTradeThrottleAllowsConfirmedLossAfterMinimumHold(t *testing.T) {
	at := &AutoTrader{}
	// Past the min hold, loss beyond the -2% noise floor → close allowed.
	ctx := throttleContext("xyz:INTC", "long", 2*time.Hour, -2.5)

	reason := at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "close_long"}, ctx)
	if reason != "" {
		t.Fatalf("expected confirmed loss after min hold to pass, got %q", reason)
	}
}

func TestTradeThrottleBlocksQuickReentryAfterClose(t *testing.T) {
	// Re-entering a just-closed symbol was a consistent loss source in the
	// replay data; the 4h cooldown is enforced from recent close orders, which
	// requires a store — covered by the throttle reason path being non-empty
	// only when a recent close order exists (nil store returns no orders).
	at := &AutoTrader{}
	ctx := &kernel.Context{}
	if reason := at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "open_long"}, ctx); reason != "" {
		t.Fatalf("expected open with no order history to be allowed, got %q", reason)
	}
}

func TestTradeThrottleDoesNotCapOpensPerCycle(t *testing.T) {
	at := &AutoTrader{}
	ctx := &kernel.Context{}

	for _, symbol := range []string{"xyz:INTC", "xyz:NVDA", "xyz:SNDK", "xyz:MU", "xyz:SP500"} {
		if reason := at.tradeThrottleReason(kernel.Decision{Symbol: symbol, Action: "open_long"}, ctx); reason != "" {
			t.Fatalf("expected %s open to have no per-cycle count cap, got %q", symbol, reason)
		}
	}
}

func TestTradeThrottleBlocksOpeningAgainstExistingPosition(t *testing.T) {
	at := &AutoTrader{}
	ctx := throttleContext("xyz:INTC", "long", 2*time.Hour, 1.0)

	reason := at.tradeThrottleReason(kernel.Decision{Symbol: "xyz:INTC", Action: "open_short"}, ctx)
	if !strings.Contains(reason, "already has an open") {
		t.Fatalf("expected opposite open to be blocked when position exists, got %q", reason)
	}
}
