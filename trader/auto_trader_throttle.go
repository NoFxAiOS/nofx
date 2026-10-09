package trader

import (
	"fmt"
	"nofx/kernel"
	"nofx/market"
	"nofx/store"
	"strings"
	"time"
)

const (
	// Exit gates, validated by decision replay (2026-07-26, 4154 cycles,
	// 3-fold robustness): gates beat no-gates by 34 pts and the old rigid
	// 4h/8h by 16 pts of worst-fold score; the searched optimum sits at these
	// values. Thresholds are PRICE-move percentages (leverage-independent).
	autopilotMinHoldDuration        = 90 * time.Minute
	autopilotNoiseCloseHoldDuration = 3 * time.Hour
	// Re-entering a just-closed symbol was a consistent loss source: the
	// replay's top-20 configs cluster tightly at ~4h.
	autopilotReentryCooldown      = 4 * time.Hour
	earlyCloseStopLossBypassPct   = -3.0
	earlyCloseTakeProfitBypassPct = 8.0
	noiseCloseLossFloorPct        = -2.0
	noiseCloseProfitCeilingPct    = 3.0
)

// positionPricePnLPct converts the margin-based UnrealizedPnLPct reported for
// a position into the underlying price-move percentage.
func positionPricePnLPct(pos *kernel.PositionInfo) float64 {
	if pos == nil {
		return 0
	}
	if pos.Leverage > 1 {
		return pos.UnrealizedPnLPct / float64(pos.Leverage)
	}
	return pos.UnrealizedPnLPct
}

func isOpenAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long", "open_short":
		return true
	default:
		return false
	}
}

func isCloseAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "close_long", "close_short":
		return true
	default:
		return false
	}
}

func closeActionSide(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "close_long":
		return "long"
	case "close_short":
		return "short"
	default:
		return ""
	}
}

func openActionSide(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open_long":
		return "long"
	case "open_short":
		return "short"
	default:
		return ""
	}
}

func normalizedDecisionSymbol(symbol string) string {
	return market.Normalize(strings.TrimSpace(symbol))
}

func (at *AutoTrader) tradeThrottleReason(decision kernel.Decision, ctx *kernel.Context) string {
	if ctx == nil {
		return ""
	}

	switch {
	case isOpenAction(decision.Action):
		return at.openThrottleReason(decision, ctx)
	case isCloseAction(decision.Action):
		return at.closeThrottleReason(decision, ctx)
	default:
		return ""
	}
}

func (at *AutoTrader) openThrottleReason(decision kernel.Decision, ctx *kernel.Context) string {
	symbol := normalizedDecisionSymbol(decision.Symbol)
	if symbol == "" {
		return ""
	}

	if pos := findAnyContextPosition(ctx, symbol); pos != nil {
		return fmt.Sprintf("trade throttle: %s already has an open %s position; manage or close it before opening another side", symbol, pos.Side)
	}

	if !at.usesVergexSignalPolicy() {
		if order := at.findRecentCloseOrder(symbol, time.Now().Add(-autopilotReentryCooldown)); order != nil {
			age := time.Since(time.UnixMilli(order.CreatedAt))
			remaining := autopilotReentryCooldown - age
			if remaining < 0 {
				remaining = 0
			}
			return fmt.Sprintf("trade throttle: %s was closed %s ago; wait %s before re-entry", symbol, roundDuration(age), roundDuration(remaining))
		}
	}

	return ""
}

func (at *AutoTrader) closeThrottleReason(decision kernel.Decision, ctx *kernel.Context) string {
	// Vergex positions are closed by the direction state machine. A changed or
	// vanished board signal must exit immediately, independent of hold duration.
	if at.usesVergexSignalPolicy() {
		return ""
	}
	symbol := normalizedDecisionSymbol(decision.Symbol)
	side := closeActionSide(decision.Action)
	if symbol == "" || side == "" {
		return ""
	}

	pos := findContextPosition(ctx, symbol, side)
	pnlPct := 0.0
	entryTime := int64(0)
	if pos != nil {
		pnlPct = positionPricePnLPct(pos)
		entryTime = pos.UpdateTime
	}

	if order := at.findRecentOpenOrder(symbol, side, time.Now().Add(-autopilotNoiseCloseHoldDuration)); order != nil && order.CreatedAt > entryTime {
		entryTime = order.CreatedAt
	}
	if entryTime <= 0 {
		return ""
	}

	heldFor := time.Since(time.UnixMilli(entryTime))
	if heldFor < 0 {
		heldFor = 0
	}
	if heldFor >= autopilotMinHoldDuration {
		if heldFor >= autopilotNoiseCloseHoldDuration ||
			pnlPct <= noiseCloseLossFloorPct ||
			pnlPct >= noiseCloseProfitCeilingPct {
			return ""
		}

		remaining := autopilotNoiseCloseHoldDuration - heldFor
		return fmt.Sprintf(
			"trade throttle: %s %s has been held for %s with price PnL %.2f%%; it is still inside the noise band %.1f%% to %.1f%%, so wait about %s before a flat/small close",
			symbol,
			side,
			roundDuration(heldFor),
			pnlPct,
			noiseCloseLossFloorPct,
			noiseCloseProfitCeilingPct,
			roundDuration(remaining),
		)
	}

	// Do not block true risk exits or unusually strong take-profit exits.
	if pnlPct <= earlyCloseStopLossBypassPct || pnlPct >= earlyCloseTakeProfitBypassPct {
		return ""
	}

	remaining := autopilotMinHoldDuration - heldFor
	return fmt.Sprintf(
		"trade throttle: %s %s has only been held for %s with price PnL %.2f%%; min AI-managed hold is %s unless price loss <= %.1f%% or price profit >= %.1f%%",
		symbol,
		side,
		roundDuration(heldFor),
		pnlPct,
		roundDuration(autopilotMinHoldDuration),
		earlyCloseStopLossBypassPct,
		earlyCloseTakeProfitBypassPct,
	) + fmt.Sprintf("; wait about %s", roundDuration(remaining))
}

func findContextPosition(ctx *kernel.Context, symbol string, side string) *kernel.PositionInfo {
	if ctx == nil {
		return nil
	}
	for i := range ctx.Positions {
		pos := &ctx.Positions[i]
		if normalizedDecisionSymbol(pos.Symbol) == symbol && strings.EqualFold(pos.Side, side) {
			return pos
		}
	}
	return nil
}

func findAnyContextPosition(ctx *kernel.Context, symbol string) *kernel.PositionInfo {
	if ctx == nil {
		return nil
	}
	for i := range ctx.Positions {
		pos := &ctx.Positions[i]
		if normalizedDecisionSymbol(pos.Symbol) == symbol {
			return pos
		}
	}
	return nil
}

func (at *AutoTrader) recentOrders(limit int) ([]*store.TraderOrder, error) {
	if at == nil || at.store == nil {
		return nil, nil
	}
	return at.store.Order().GetTraderOrders(at.id, limit)
}

func (at *AutoTrader) findRecentCloseOrder(symbol string, since time.Time) *store.TraderOrder {
	orders, err := at.recentOrders(100)
	if err != nil {
		at.logWarnf("⚠️ Trade throttle could not read recent close orders: %v", err)
		return nil
	}
	sinceMs := since.UTC().UnixMilli()
	for _, order := range orders {
		if order == nil || order.CreatedAt < sinceMs || isCanceledOrder(order) {
			continue
		}
		if normalizedDecisionSymbol(order.Symbol) == symbol && isCloseAction(order.OrderAction) {
			return order
		}
	}
	return nil
}

func (at *AutoTrader) findRecentOpenOrder(symbol string, side string, since time.Time) *store.TraderOrder {
	orders, err := at.recentOrders(100)
	if err != nil {
		at.logWarnf("⚠️ Trade throttle could not read recent open orders: %v", err)
		return nil
	}
	sinceMs := since.UTC().UnixMilli()
	for _, order := range orders {
		if order == nil || order.CreatedAt < sinceMs || isCanceledOrder(order) {
			continue
		}
		if normalizedDecisionSymbol(order.Symbol) == symbol &&
			strings.EqualFold(openActionSide(order.OrderAction), side) {
			return order
		}
	}
	return nil
}

func isCanceledOrder(order *store.TraderOrder) bool {
	status := strings.ToUpper(strings.TrimSpace(order.Status))
	return status == "CANCELED" || status == "CANCELLED" || status == "REJECTED" || status == "EXPIRED"
}

func roundDuration(d time.Duration) string {
	if d < time.Minute {
		return "0m"
	}
	return d.Round(time.Minute).String()
}
