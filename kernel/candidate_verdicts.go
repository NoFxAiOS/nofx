package kernel

import (
	"nofx/market"
	"nofx/store"
)

// CandidateVerdict is persisted with each decision record (2026-10-10
// per-candidate block reasons); the type lives in store (kernel imports store).
type CandidateVerdict = store.CandidateVerdict

// markFiltered records why a candidate never reached the prompt.
func markFiltered(ctx *Context, symbol, reason string) {
	if ctx.FilteredCandidates == nil {
		ctx.FilteredCandidates = make(map[string]string)
	}
	ctx.FilteredCandidates[market.Normalize(symbol)] = reason
}

// BuildCandidateVerdicts returns one verdict per candidate, in pool order
// (pre-fetch order when known, so OI-dropped coins keep their slot).
// 2026-10-10 per-candidate block reasons.
func BuildCandidateVerdicts(ctx *Context) []CandidateVerdict {
	if ctx == nil {
		return nil
	}
	var order []string
	seen := map[string]bool{}
	add := func(sym string) {
		k := market.Normalize(sym)
		if seen[k] {
			return
		}
		seen[k] = true
		order = append(order, sym)
	}
	for _, s := range ctx.CandidateOrder {
		add(s)
	}
	for _, c := range ctx.CandidateCoins {
		add(c.Symbol)
	}
	// Held symbols are rendered in the positions section, never as candidate
	// blocks, so they carry no gate state — report them as held instead of
	// falling through to "no market data" (2026-10-14 BTWUSDT).
	held := map[string]bool{}
	for _, p := range ctx.Positions {
		held[market.Normalize(p.Symbol)] = true
	}
	out := make([]CandidateVerdict, 0, len(order))
	for _, sym := range order {
		k := market.Normalize(sym)
		v := CandidateVerdict{Symbol: sym}
		if gs := ctx.GateStates[k]; gs != nil {
			v.LongFailed = append([]string(nil), gs.LongFailed...)
			v.ShortFailed = append([]string(nil), gs.ShortFailed...)
			if !gs.LongAllowed && !gs.ShortAllowed {
				v.Status = "blocked"
			} else {
				v.Status = "evaluated"
			}
		} else if held[k] && ctx.FilteredCandidates[k] == "" {
			v.Status = "held"
			v.Reason = "held position — evaluated in the positions section"
		} else {
			v.Status = "filtered"
			if r := ctx.FilteredCandidates[k]; r != "" {
				v.Reason = r
			} else {
				v.Reason = "not rendered (no market data)"
			}
		}
		out = append(out, v)
	}
	return out
}
