package stockengine

import (
	"errors"
	"math"
)

func finite(v float64) bool   { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func positive(v float64) bool { return finite(v) && v > 0 }
func snapshotFor(ctx *Context, symbol string) *SymbolSnapshot {
	for _, s := range ctx.Snapshots {
		if s != nil && s.Symbol == symbol {
			return s
		}
	}
	return nil
}
func positionFor(ctx *Context, symbol string) *Position {
	for i := range ctx.Positions {
		if ctx.Positions[i].Symbol == symbol && ctx.Positions[i].Quantity > 0 {
			return &ctx.Positions[i]
		}
	}
	return nil
}
func entryPrice(s *SymbolSnapshot, d Decision) float64 {
	if d.EntryType == EntryLimit {
		return d.LimitPrice
	}
	return s.Price
}

// SizeOrder sizes one order with no lot-size rounding. For several orders callers
// must reserve earlier sizes; Validate does so explicitly with cycleNotional.
func SizeOrder(ctx *Context, d Decision) (Sizing, error) { return sizeOrder(ctx, d, 0) }

func sizeOrder(ctx *Context, d Decision, cycleNotional float64) (Sizing, error) {
	if ctx == nil {
		return Sizing{}, errors.New("DATA_INSUFFICIENT")
	}
	s := snapshotFor(ctx, d.Symbol)
	if s == nil || !positive(s.Price) {
		return Sizing{}, errors.New("DATA_INSUFFICIENT")
	}
	if d.EntryType != "" && d.EntryType != EntryMarket && d.EntryType != EntryLimit {
		return Sizing{}, errors.New("ENTRY_INVALID")
	}
	entry := entryPrice(s, d)
	if !positive(entry) || (d.EntryType == EntryLimit && entry > s.Price*1.01) {
		return Sizing{}, errors.New("LIMIT_INVALID")
	}
	if !positive(d.StopLoss) || d.StopLoss >= entry {
		return Sizing{}, errors.New("STOP_INVALID")
	}
	risk := entry - d.StopLoss
	currentValue := 0.0
	if p := positionFor(ctx, d.Symbol); p != nil {
		currentValue = p.Quantity * s.Price
	}
	// The regime scale shrinks the risk budget and both exposure caps; the cash
	// bound is a fact, not a budget, and stays as is.
	regime := 1.0
	if ctx.Regime.Enabled {
		regime = ctx.Regime.Scale
	}
	// Reserve ALL accepted buys (opens and adds), both exposure and cash, in order.
	bounds := []struct {
		name string
		qty  float64
	}{
		{"risk", ctx.Account.Equity * effectiveRiskPerTradePct(ctx.Config) * regime / 100 / risk},
		{"position_cap", (ctx.Account.Equity*effectiveMaxPositionPct(ctx.Config)*regime/100 - currentValue) / entry},
		{"exposure_cap", (ctx.Account.Equity*effectiveMaxTotalExposurePct(ctx.Config)*regime/100 - ctx.Account.Exposure - cycleNotional) / entry},
		{"available", (ctx.Account.Available - cycleNotional) * 0.995 / entry},
	}
	qty, limited := bounds[0].qty, bounds[0].name
	for _, b := range bounds {
		if !finite(b.qty) {
			return Sizing{}, errors.New("NO_CAPACITY")
		}
		if b.qty < qty {
			qty, limited = b.qty, b.name
		}
	}
	qty = math.Max(0, qty)
	size := Sizing{Quantity: qty, Notional: qty * entry, RiskUSDT: qty * risk, LimitedBy: limited}
	if qty <= 0 {
		return size, errors.New("NO_CAPACITY")
	}
	if size.Notional < 5 {
		size.LimitedBy = "min_notional"
		return size, errors.New("MIN_NOTIONAL")
	}
	return size, nil
}
