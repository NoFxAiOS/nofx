package trader

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/store"
)

// review 2026-10-11: growing partial fills retain a tightened stop and the
// original opening-risk anchor, for both position directions.
func TestReview11PartialFillRetainsTightenedStop(t *testing.T) {
	for _, side := range []string{"long", "short"} {
		t.Run(side, func(t *testing.T) {
			m := &review03Mock{}
			at := review03Trader(m)
			pe := review03Plan()
			pe.Side = side
			tight := 101.0
			if side == "short" {
				pe.StopLoss, pe.TakeProfit, tight = 105, 90, 99
			}
			if got := at.protectExecutedSlice(pe, map[string]interface{}{"executedQty": 0.4, "avgPrice": 100.0}); got != pe.StopLoss {
				t.Fatalf("first slice stop=%g, want %g", got, pe.StopLoss)
			}
			at.SetRecordedStopLoss(pe.Symbol, side, tight)
			// Model the exchange's confirmed early-breakeven/AI stop move.
			for i := range m.orders {
				if m.orders[i].Type == "STOP_MARKET" {
					m.orders[i].StopPrice = tight
				}
			}
			if got := at.protectExecutedSlice(pe, map[string]interface{}{"executedQty": 0.8, "avgPrice": 100.0}); got != tight {
				t.Fatalf("later slice restored stop=%g, want %g", got, tight)
			}
			coverage := protectiveCoverage(m.orders, strings.ToUpper(side), "SL", tight)
			if math.Abs(coverage-0.8) > 1e-9 || pe.ProtectedQty != 0.8 {
				t.Fatalf("coverage=%g watermark=%g, want 0.8", coverage, pe.ProtectedQty)
			}
			for _, order := range m.orders {
				if order.Type == "STOP_MARKET" && order.StopPrice != tight {
					t.Fatalf("looser stop rearmed: %+v", order)
				}
			}
			if got := at.GetInitialStopLoss(pe.Symbol, side); got != pe.StopLoss {
				t.Fatalf("opening anchor=%g, want original plan %g", got, pe.StopLoss)
			}
		})
	}
}

func TestReview11PartialFillChoosesTightestValidStop(t *testing.T) {
	for _, tc := range []struct {
		name, side string
		recorded   float64
	}{
		{"long_plan_tighter", "long", 94}, {"short_plan_tighter", "short", 106},
		{"long_zero", "long", 0}, {"short_negative", "short", -1},
		{"long_nan", "long", math.NaN()}, {"short_infinite", "short", math.Inf(1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &review03Mock{}
			at := review03Trader(m)
			pe := review03Plan()
			pe.Side = tc.side
			if tc.side == "short" {
				pe.StopLoss, pe.TakeProfit = 105, 90
			}
			at.SetRecordedStopLoss(pe.Symbol, pe.Side, tc.recorded)
			if got := at.protectExecutedSlice(pe, map[string]interface{}{"executedQty": 0.8, "avgPrice": 100.0}); got != pe.StopLoss {
				t.Fatalf("stop=%g, want valid tighter plan %g", got, pe.StopLoss)
			}
		})
	}
}

// review 2026-10-11: the replacement decision uses the FINAL receipt, even
// when a full fill makes the exchange reject the cancellation itself.
func TestReview11ReplacementAbortsAfterCancelFill(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		executed     float64
		cancelErr    error
	}{
		{"unfilled", "CANCELED", 0, nil},
		{"partial_fill", "CANCELED", 0.4, nil},
		{"full_fill", "FILLED", 1, errors.New("order already filled")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &review03Mock{cancelErr: tc.cancelErr, status: map[string]interface{}{
				"status": tc.status, "executedQty": tc.executed, "avgPrice": 100.0,
			}}
			at := review03Trader(m)
			pe := review03Plan()
			at.setPendingEntry(pe)
			err := at.cancelPendingForReplacement(pe)
			newEntryAllowed := err == nil
			if newEntryAllowed != (tc.executed == 0) {
				t.Fatalf("executed=%g new entry allowed=%t err=%v", tc.executed, newEntryAllowed, err)
			}
			if tc.executed > 0 && !strings.Contains(err.Error(), "new entry aborted; position now owned by the filled slice") {
				t.Fatalf("missing replacement-abort reason: %v", err)
			}
			if pe.ProtectedQty != tc.executed || at.getPendingEntry(pe.Symbol, pe.Side) != nil {
				t.Fatalf("terminal slice ownership/protection: pending=%v protected=%g", at.getPendingEntry(pe.Symbol, pe.Side), pe.ProtectedQty)
			}
		})
	}
}

func TestReview11CancelReportsFinalExecutedQuantity(t *testing.T) {
	m := &review03Mock{status: map[string]interface{}{"status": "CANCELED", "executedQty": "0.6", "avgPrice": "100"}}
	at := review03Trader(m)
	pe := review03Plan()
	at.setPendingEntry(pe)
	executed, err := at.cancelPendingWithExecuted(pe)
	if err != nil || executed != 0.6 || pe.ProtectedQty != 0.6 {
		t.Fatalf("executed=%g protected=%g err=%v", executed, pe.ProtectedQty, err)
	}
}

type review11ProfitLockMock struct {
	*batch1RiskMock
	stopErr   error
	invisible bool
}

func (m *review11ProfitLockMock) SetStopLoss(symbol, side string, qty, price float64) error {
	if m.stopErr != nil || m.invisible {
		m.slCalls++
		return m.stopErr
	}
	return m.batch1RiskMock.SetStopLoss(symbol, side, qty, price)
}

// review 2026-10-11: exercise the full management cycle, including retries,
// verification failures, already-safe stops, early arms and the dust exemption.
func TestReview11ProfitLockTrimRequiresBreakeven(t *testing.T) {
	for _, side := range []string{"long", "short"} {
		for _, scenario := range []string{"rejected_then_retry", "unverified", "success", "already_at", "already_past", "early_arm", "dust_full_close"} {
			t.Run(side+"/"+scenario, func(t *testing.T) {
				initial, current, mark, qty := 95.0, 95.0, 108.0, 1.0
				if side == "short" {
					initial, current, mark, qty = 105, 105, 92, -1
				}
				rc := store.RiskControlConfig{ProfitLockAtR: 1, ProfitLockBEOffsetR: 0.2}
				if scenario == "early_arm" {
					rc.BreakevenArmR = 0.5
				}
				if scenario == "dust_full_close" {
					rc.MinPositionSize = 60
				}
				if scenario == "already_at" || scenario == "already_past" {
					current = 101
					if side == "short" {
						current = 99
					}
					if scenario == "already_past" {
						current = 102
						if side == "short" {
							current = 98
						}
					}
				}
				at, base := newBatch1RiskTrader(side, qty, mark, rc)
				m := &review11ProfitLockMock{batch1RiskMock: base, invisible: scenario == "unverified"}
				if scenario == "rejected_then_retry" || scenario == "dust_full_close" {
					m.stopErr = errors.New("simulated stop rejection")
				}
				at.trader = m
				at.marketDataFn = func(string) (*market.Data, error) { return &market.Data{Symbol: "XUSDT", CurrentPrice: mark}, nil }
				at.SetInitialStopLoss("XUSDT", side, initial)
				at.SetRecordedStopLoss("XUSDT", side, current)
				at.processVolTargetAndTrailing()
				key := "XUSDT_" + side
				if scenario == "rejected_then_retry" || scenario == "unverified" {
					if m.closeCalls+m.closeShorts != 0 || at.r1TrimDone[key] || at.tpTrimDone[key] {
						t.Fatalf("failed stop still trimmed: closes=%d/%d flags=%v/%v", m.closeCalls, m.closeShorts, at.r1TrimDone, at.tpTrimDone)
					}
					if at.GetRecordedStopLoss("XUSDT", side) != current {
						t.Fatal("failed stop move changed the recorded stop")
					}
					if scenario == "unverified" {
						return
					}
					m.stopErr = nil
					at.processVolTargetAndTrailing()
				}
				if m.closeCalls+m.closeShorts != 1 || !at.r1TrimDone[key] || !at.tpTrimDone[key] {
					t.Fatalf("safe trim/full close missing: closes=%d/%d flags=%v/%v", m.closeCalls, m.closeShorts, at.r1TrimDone, at.tpTrimDone)
				}
				if scenario != "dust_full_close" && m.closedQty != 0.5 {
					t.Fatalf("trim quantity=%g, want 0.5", m.closedQty)
				}
				if scenario == "dust_full_close" && m.closedQty != 0 {
					t.Fatalf("dust must use the full-close sentinel, got quantity=%g", m.closedQty)
				}
				if (scenario == "already_at" || scenario == "already_past") && m.slCalls != 0 {
					t.Fatal("already-safe stop should permit trim without another stop move")
				}
				// A full emergency close intentionally clears position caches.
				if scenario != "dust_full_close" && at.GetInitialStopLoss("XUSDT", side) != initial {
					t.Fatal("profit lock changed the opening-risk anchor")
				}
			})
		}
	}
}

func review11CloseGateData(side string) *market.Data {
	start := time.Now().Truncate(15 * time.Minute)
	bars := make([]market.KlineBar, 21)
	for i := range bars {
		bars[i] = market.KlineBar{Time: start.Add(time.Duration(i-20) * 15 * time.Minute).UnixMilli(), Open: 99.9, High: 100, Low: 99.8, Close: 99.9, Volume: 100}
	}
	bars[5].Low, bars[10].Low = 98, 99.5
	bars[19].Open, bars[19].High, bars[19].Low, bars[19].Close = 99.3, 99.4, 99.1, 99.2
	// A forming bar has recovered; it must not erase the closed-candle break.
	bars[20].Open, bars[20].High, bars[20].Low, bars[20].Close = 100, 100.3, 99.9, 100.2
	mark := 99.2
	if side == "short" {
		mark = 200 - mark
		for i := range bars {
			b := &bars[i]
			b.Open, b.High, b.Low, b.Close = 200-b.Open, 200-b.Low, 200-b.High, 200-b.Close
		}
	}
	return &market.Data{Symbol: "XUSDT", CurrentPrice: mark, TimeframeData: map[string]*market.TimeframeSeriesData{"15m": {Timeframe: "15m", Klines: bars}}}
}

// review 2026-10-11: the real view builder must retain pivots that the live
// S/R classification has discarded, then exempt the losing close at entry.
func TestReview11CloseGateClosedStructureBreak(t *testing.T) {
	for _, side := range []string{"long", "short"} {
		t.Run(side, func(t *testing.T) {
			data := review11CloseGateData(side)
			view := buildCloseGateView(data, side, data.CurrentPrice)
			if view == nil || view.structure15m == nil || !view.structure15m.UnclosedDropped {
				t.Fatalf("missing settled structure: %+v", view)
			}
			tf := view.structure15m
			if !closeGateStructureBroken(tf, side, 100) || tf.LastClose != data.CurrentPrice {
				t.Fatalf("closed candle did not break entry-relative pivot: %+v", tf)
			}
			// With the break removed, proximity and the losing entry DO block.
			intact := *view
			intact.structure15m = nil
			if blocked, _ := closeRejectBreakoutBlocks(side, 100, &intact, 1); !blocked {
				t.Fatal("fixture must otherwise be a blocked losing close near structure")
			}
			if blocked, reason := closeRejectBreakoutBlocks(side, 100, view, 1); blocked {
				t.Fatalf("confirmed 15m break was blocked: %s", reason)
			}
		})
	}
}

func TestReview11CloseGateEntryRelativePivot(t *testing.T) {
	for _, tc := range []struct {
		name, side string
		close      float64
		levels     []float64
		want       bool
	}{
		{"long_highest_eligible", "long", 98.5, []float64{97, 101, 99}, true},
		{"long_entry_equal", "long", 99.5, []float64{100}, true},
		{"long_close_equal", "long", 99, []float64{99}, false},
		{"long_no_entry_level", "long", 99, []float64{101}, false},
		{"short_lowest_eligible", "short", 101.5, []float64{103, 99, 101}, true},
		{"short_entry_equal", "short", 100.5, []float64{100}, true},
		{"short_close_equal", "short", 101, []float64{101}, false},
		{"short_no_entry_level", "short", 101, []float64{99}, false},
		{"missing_levels", "long", 98, nil, false},
		{"missing_close", "short", 0, []float64{101}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tf := &kernel.TFSignal{LastClose: tc.close}
			if tc.side == "long" {
				tf.StructuralSupport = tc.levels
			} else {
				tf.StructuralResistance = tc.levels
			}
			if got := closeGateStructureBroken(tf, tc.side, 100); got != tc.want {
				t.Fatalf("structure broken=%t, want %t", got, tc.want)
			}
		})
	}
}
