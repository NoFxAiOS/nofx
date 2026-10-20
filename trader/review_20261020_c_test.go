package trader

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
	"nofx/trader/binance"
	"nofx/trader/types"

	"github.com/adshao/go-binance/v2/common"
)

// heldMock reports a configurable long XUSDT position (0 = flat) and records
// close calls. Fake account only.
type heldMock struct {
	review03Mock
	held   float64
	closes int
}

func (m *heldMock) GetPositions() ([]map[string]interface{}, error) {
	if m.positionsErr != nil {
		return nil, m.positionsErr
	}
	if m.held == 0 {
		return nil, nil
	}
	return []map[string]interface{}{{"symbol": "XUSDT", "side": "long", "positionAmt": m.held}}, nil
}
func (m *heldMock) CloseLong(string, float64) (map[string]interface{}, error) {
	m.closes++
	return map[string]interface{}{"orderId": int64(5)}, nil
}

func review1020Trader(t *testing.T, m Trader) *AutoTrader {
	t.Helper()
	at := riskTestTrader(store.RiskControlConfig{})
	at.trader = m
	at.positionInitialStopLoss = map[string]float64{}
	at.positionExitMode = map[string]string{}
	at.store = lifecycleFixture(t).store
	at.id, at.name, at.exchange = "t", "t", "binance"
	at.marketDataFn = func(string) (*market.Data, error) { return nil, errors.New("no market data") }
	return at
}

// area2 #2: a submitted full close must not clear protection state or AI
// ownership while the position is not confirmed flat.
func TestReview1020AICloseKeepsOwnershipUntilFlatConfirmed(t *testing.T) {
	prev := closeFlatRetryDelay
	closeFlatRetryDelay = 0
	t.Cleanup(func() { closeFlatRetryDelay = prev })
	for _, tc := range []struct {
		name     string
		held     float64
		posErr   error
		wantFlat bool
	}{
		{"flat", 0, nil, true},
		{"partial_remainder", 0.6, nil, false},
		{"snapshot_error", 0, errors.New("positions down"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &heldMock{held: 1}
			at := review1020Trader(t, m)
			at.markAIManaged("XUSDT", "long", "e1")
			at.SetRecordedStopLoss("XUSDT", "long", 95)
			// Fill the close: the exchange snapshot AFTER the order reflects tc.
			closeHook := &closeThenMock{heldMock: m, held: tc.held, posErr: tc.posErr}
			at.trader = closeHook
			if err := at.executeCloseLongWithRecord(&kernel.Decision{Symbol: "XUSDT"}, &store.DecisionAction{}); err != nil {
				t.Fatalf("close: %v", err)
			}
			marked := at.isAIManaged("XUSDT", "long")
			recorded := at.GetRecordedStopLoss("XUSDT", "long")
			if tc.wantFlat && (marked || recorded != 0) {
				t.Fatalf("confirmed flat must clear state: marked=%v recordedSL=%v", marked, recorded)
			}
			if !tc.wantFlat && (!marked || recorded != 95) {
				t.Fatalf("unconfirmed close must keep ownership/SL: marked=%v recordedSL=%v", marked, recorded)
			}
		})
	}
}

// closeThenMock switches the position snapshot once CloseLong was called.
type closeThenMock struct {
	*heldMock
	held   float64
	posErr error
	closed bool
}

func (m *closeThenMock) CloseLong(s string, q float64) (map[string]interface{}, error) {
	m.closed = true
	return m.heldMock.CloseLong(s, q)
}
func (m *closeThenMock) GetPositions() ([]map[string]interface{}, error) {
	if !m.closed {
		return m.heldMock.GetPositions()
	}
	if m.posErr != nil {
		return nil, m.posErr
	}
	if m.held == 0 {
		return nil, nil
	}
	return []map[string]interface{}{{"symbol": "XUSDT", "side": "long", "positionAmt": m.held}}, nil
}

// area2 #3: an offline fill is only marked AI-managed once a position is
// confirmed; a confirmed-flat recovery clears just this entry's own stale mark.
func TestReview1020OfflineFillMarksOnlyConfirmedPosition(t *testing.T) {
	row := &store.PendingEntryDB{TraderID: "t", Symbol: "XUSDT", Side: "long", Price: 100, Quantity: 1, StopLoss: 95, TakeProfit: 110, Leverage: 1, OrderID: "e1"}
	status := map[string]interface{}{"status": "FILLED", "executedQty": 1.0, "avgPrice": 100.0}

	t.Run("flat_no_mark_created", func(t *testing.T) {
		at := review1020Trader(t, &heldMock{held: 0})
		at.finalizePendingFill(row, status)
		if at.isAIManaged("XUSDT", "long") {
			t.Fatal("no position: nothing may be marked AI-managed")
		}
	})
	t.Run("flat_clears_own_stale_mark_only", func(t *testing.T) {
		at := review1020Trader(t, &heldMock{held: 0})
		at.markAIManaged("XUSDT", "long", "e1")
		at.finalizePendingFill(row, status)
		if at.isAIManaged("XUSDT", "long") {
			t.Fatal("stale mark of the same entry must be cleared")
		}
		at.markAIManaged("XUSDT", "long", "e2") // newer generation
		at.finalizePendingFill(row, status)
		if !at.isAIManaged("XUSDT", "long") {
			t.Fatal("a newer generation's mark must survive")
		}
	})
	t.Run("position_confirmed_marks", func(t *testing.T) {
		at := review1020Trader(t, &heldMock{held: 1})
		at.finalizePendingFill(row, status)
		if !at.isAIManaged("XUSDT", "long") {
			t.Fatal("confirmed position must be marked AI-managed")
		}
	})
	t.Run("check_failed_not_marked_plan_kept", func(t *testing.T) {
		hm := &heldMock{}
		hm.positionsErr = errors.New("down")
		at := review1020Trader(t, hm)
		at.finalizePendingFill(row, status)
		if at.isAIManaged("XUSDT", "long") {
			t.Fatal("unconfirmed position must not be marked")
		}
		if at.getPendingEntry("XUSDT", "long") == nil {
			t.Fatal("plan must stay live for retry")
		}
	})
}

// area2 #7: cumulative executed quantity must not re-arm a TP slice that
// already filled (that would sell the runner).
func TestReview1020PendingRepairDoesNotRearmFilledTP(t *testing.T) {
	status := map[string]interface{}{"status": "PARTIALLY_FILLED", "executedQty": 1.0, "avgPrice": 100.0}
	newCase := func(held float64, previouslyProtected float64) (*AutoTrader, *heldMock, *pendingEntry) {
		m := &heldMock{held: held}
		// Full-size SL still rests; the TP leg (5) already filled and is gone.
		m.orders = []types.OpenOrder{{OrderID: "sl", Type: "STOP_MARKET", PositionSide: "LONG", StopPrice: 95, Quantity: 1, ClosePosition: true}}
		at := riskTestTrader(store.RiskControlConfig{TrailingStopEnabled: true})
		at.trader = m
		at.positionInitialStopLoss = map[string]float64{}
		at.positionExitMode = map[string]string{}
		m.equity = 1000
		pe := &pendingEntry{Symbol: "XUSDT", Side: "long", Price: 100, Quantity: 2, StopLoss: 95, TakeProfit: 110, OrderID: "entry", PlacedAt: time.Now(),
			ExecutedQty: 1, ProtectedQty: previouslyProtected}
		return at, m, pe
	}
	t.Run("filled_tp_not_rearmed", func(t *testing.T) {
		at, m, pe := newCase(0.5, 1) // runner 0.5 left of 1 executed
		at.protectExecutedSlice(pe, status)
		if m.tpCalls != 0 {
			t.Fatalf("TP already filled, runner must not be re-armed: tpCalls=%d", m.tpCalls)
		}
	})
	t.Run("first_fill_still_places_tp", func(t *testing.T) {
		at, m, pe := newCase(1, 0)
		pe.ExecutedQty = 0
		at.protectExecutedSlice(pe, status)
		if m.tpCalls != 1 {
			t.Fatalf("fresh slice must get its TP: tpCalls=%d", m.tpCalls)
		}
	})
	t.Run("later_fill_tops_up_only_missing", func(t *testing.T) {
		at, m, pe := newCase(1, 1) // position 1 = 0.5 runner + 0.5 new; executed 1.5
		pe.ExecutedQty = 1
		at.protectExecutedSlice(pe, map[string]interface{}{"status": "PARTIALLY_FILLED", "executedQty": 1.5, "avgPrice": 100.0})
		if m.tpCalls != 1 || m.orders[len(m.orders)-1].Quantity != 0.25 {
			t.Fatalf("want one TP top-up of 0.25 (0.75 target - 0.5 realized), calls=%d orders=%+v", m.tpCalls, m.orders)
		}
	})
	t.Run("unknown_position_defers", func(t *testing.T) {
		at, m, pe := newCase(0.5, 1)
		m.positionsErr = errors.New("down")
		at.protectExecutedSlice(pe, status)
		if m.tpCalls != 0 || pe.ProtectedQty != 0 {
			t.Fatalf("unknown position must defer without advancing the watermark: tp=%d watermark=%v", m.tpCalls, pe.ProtectedQty)
		}
	})
}

// area2 #1: an UNKNOWN submit is tracked, blocks the same side, and is adopted
// (real order ID, normal fill protection) once the exchange answers.
func TestReview1020UnknownSubmitTrackedBlockedAndAdopted(t *testing.T) {
	unknownErr := fmt.Errorf("failed to open long position: %w", &binance.SubmitUnknownError{Symbol: "XUSDT", ClientOrderID: "x-abc123"})
	plan := pendingEntry{Symbol: "XUSDT", Price: 100, Quantity: 1, StopLoss: 95, TakeProfit: 110, Leverage: 2}

	t.Run("plain_error_untouched", func(t *testing.T) {
		at := review03Trader(&review03Mock{})
		orig := errors.New("margin insufficient")
		if got := at.trackUnknownSubmit(orig, "long", plan); got != orig || at.getPendingEntry("XUSDT", "long") != nil {
			t.Fatalf("non-UNKNOWN errors must pass through untouched: %v", got)
		}
	})
	t.Run("tracked_blocks_then_adopted", func(t *testing.T) {
		m := &review03Mock{statusErr: errors.New("still unknown")}
		at := review03Trader(m)
		if err := at.trackUnknownSubmit(unknownErr, "long", plan); !errors.Is(err, binance.ErrSubmitUnknown) {
			t.Fatalf("tracked error must still be UNKNOWN: %v", err)
		}
		pe := at.getPendingEntry("XUSDT", "long")
		if pe == nil || pe.OrderID != binance.ClientOrderRefPrefix+"x-abc123" {
			t.Fatalf("UNKNOWN submit not tracked: %+v", pe)
		}
		if err := at.unknownSubmitBlocks("XUSDT", "long"); err == nil {
			t.Fatal("same-side entry must be blocked while UNKNOWN")
		}
		if err := at.unknownSubmitBlocks("XUSDT", "short"); err != nil {
			t.Fatalf("opposite side is independent: %v", err)
		}
		if err := at.cancelPendingSide("XUSDT", "long"); err == nil {
			t.Fatal("replacing an UNKNOWN entry must be refused")
		}
		at.processPendingEntries(false)
		if at.getPendingEntry("XUSDT", "long") == nil {
			t.Fatal("unresolved UNKNOWN entry must stay tracked")
		}
		// Exchange answers: filled under the real order ID.
		m.statusErr = nil
		m.status = map[string]interface{}{"orderId": int64(42), "status": "FILLED", "executedQty": 1.0, "avgPrice": 100.0}
		at.processPendingEntries(false)
		if at.getPendingEntry("XUSDT", "long") != nil {
			t.Fatal("filled + protected entry must complete the lifecycle")
		}
		if m.slCalls == 0 {
			t.Fatal("adopted fill must receive protection")
		}
	})
	t.Run("never_landed_dropped", func(t *testing.T) {
		notFound := fmt.Errorf("failed to get order status: %w", &common.APIError{Code: -2013, Message: "Order does not exist."})
		m := &review03Mock{statusErr: notFound}
		at := review03Trader(m)
		_ = at.trackUnknownSubmit(unknownErr, "long", plan)
		pe := at.getPendingEntry("XUSDT", "long")
		pe.PlacedAt = time.Now().Add(-5 * time.Minute)
		at.processPendingEntries(false)
		if at.getPendingEntry("XUSDT", "long") == nil {
			t.Fatal("a single not-found must not drop the tracking yet")
		}
		at.processPendingEntries(false)
		if at.getPendingEntry("XUSDT", "long") != nil {
			t.Fatal("repeated not-found past the grace window must drop the tracking")
		}
	})
}
