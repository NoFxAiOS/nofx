package stockengine

import (
	"fmt"
	"math"
	"nofx/market/usstock"
	"strings"
)

// MinEntryConfidence is the confidence gate (0–100): open_long / add_long below
// it are rejected with LOW_CONFIDENCE. Exits, adjusts, hold and wait are not gated.
const MinEntryConfidence = 80

// RepairableCodes are the rejection codes the model can fix by resubmitting the
// entry (a bad stop/limit/take-profit/entry type, or a missing thesis). A
// rejected open/add is repaired only when ALL its codes are in this set: a
// low-confidence idea, a full book, a closed session or missing data cannot be
// repaired by rewording the decision.
var RepairableCodes = map[string]bool{
	"STOP_INVALID":     true,
	"STOP_OUT_OF_BAND": true,
	"LIMIT_INVALID":    true,
	"TP_INVALID":       true,
	"ENTRY_INVALID":    true,
	"THESIS_REQUIRED":  true,
}

// Validate preserves input order, rejects later duplicate symbols, and reserves
// accepted buy notionals for sizing subsequent buys in this cycle. It never
// mutates the context or assumes a proposed close has already released capacity.
func Validate(ctx *Context, decisions []Decision) []Verdict {
	verdicts := make([]Verdict, 0, len(decisions))
	seen := make(map[string]bool)
	opens, cycleNotional := 0, 0.0
	held := 0
	if ctx != nil {
		for _, p := range ctx.Positions {
			if p.Quantity > 0 {
				held++
			}
		}
	}
	for _, d := range decisions {
		v := Verdict{Decision: d}
		code, note := validateOne(ctx, d, seen, held+opens)
		var extra []string
		if d.Action == ActionOpenLong || d.Action == ActionAddLong {
			if strings.TrimSpace(d.Thesis) == "" || strings.TrimSpace(d.Invalidation) == "" {
				switch {
				case code == "":
					code, note = "THESIS_REQUIRED", "thesis (why it works over weeks) and invalidation (what proves it wrong) are both required for open_long / add_long."
				case RepairableCodes[code]:
					extra = append(extra, "THESIS_REQUIRED")
					note = strings.TrimSpace(note + " Also: thesis and invalidation are both required for open_long / add_long.")
				}
			}
		}
		if code == "" && (d.Action == ActionOpenLong || d.Action == ActionAddLong) {
			size, err := sizeOrder(ctx, d, cycleNotional)
			if err != nil {
				code = err.Error()
			} else {
				cycleNotional += size.Notional
				if d.Action == ActionOpenLong {
					opens++
				}
			}
		}
		v.Accepted, v.Note = code == "", note
		if code != "" {
			v.Codes = append([]string{code}, extra...)
		}
		verdicts = append(verdicts, v)
	}
	return verdicts
}

func validateOne(ctx *Context, d Decision, seen map[string]bool, positionCount int) (string, string) {
	known := false
	if ctx != nil && ctx.Config != nil {
		for _, symbol := range ctx.Config.Symbols {
			if symbol == d.Symbol {
				known = true
				break
			}
		}
	}
	if !known {
		return "UNKNOWN_SYMBOL", ""
	}
	if seen[d.Symbol] {
		return "DUPLICATE", ""
	}
	seen[d.Symbol] = true
	if !validAction(d.Action) {
		return "UNKNOWN_ACTION", ""
	}
	p := positionFor(ctx, d.Symbol)
	if d.Action == ActionOpenLong && p != nil {
		return "ALREADY_HELD", "Use add_long for an existing position."
	}
	if p == nil {
		switch d.Action {
		case ActionAddLong, ActionReduceLong, ActionCloseLong, ActionAdjustStop, ActionHold:
			return "NOT_HELD", ""
		}
	}
	s := snapshotFor(ctx, d.Symbol)
	switch d.Action {
	case ActionOpenLong, ActionAddLong:
		if ctx.Session == usstock.SessionClosed {
			return "SESSION_CLOSED", ""
		}
		if !sessionAllowed(ctx.Config, ctx.Session) {
			return "SESSION_NOT_ALLOWED", ""
		}
		if d.Confidence < MinEntryConfidence {
			return "LOW_CONFIDENCE", fmt.Sprintf("confidence %d is below the entry gate %d.", d.Confidence, MinEntryConfidence)
		}
		preset := contextPreset(ctx)
		if s == nil || !positive(s.Price) {
			return "DATA_INSUFFICIENT", ""
		}
		for _, tf := range s.MissingTF {
			if tf == usstock.TF1d || tf == preset.TrendTF || tf == preset.EntryTF {
				return "DATA_INSUFFICIENT", ""
			}
		}
		if s.Quote == nil || !s.Quote.RefFresh {
			return "BSTOCK_REF_STALE", ""
		}
		limit := effectiveMaxDivergencePct(ctx.Config)
		if limit >= 0 && (!finite(s.Quote.DivergencePct) || math.Abs(s.Quote.DivergencePct) > limit) {
			return "BSTOCK_DIVERGENCE", ""
		}
		if d.EntryType != "" && d.EntryType != EntryMarket && d.EntryType != EntryLimit {
			return "ENTRY_INVALID", "entry_type must be market or limit."
		}
		entry := entryPrice(s, d)
		if d.EntryType == EntryLimit && (!positive(entry) || entry > s.Price*1.01) {
			return "LIMIT_INVALID", fmt.Sprintf("limit_price must be >0 and ≤ market price×1.01 = %.4f (yours %.4f).", s.Price*1.01, entry)
		}
		if !positive(d.StopLoss) || d.StopLoss >= entry {
			return "STOP_INVALID", fmt.Sprintf("stop_loss must be >0 and below the entry price %.4f (yours %.4f).", entry, d.StopLoss)
		}
		if !positive(s.ATR1d) {
			return "DATA_INSUFFICIENT", ""
		}
		multiple := (entry - d.StopLoss) / s.ATR1d
		if multiple < preset.StopATRMin || multiple > preset.StopATRMax {
			return "STOP_OUT_OF_BAND", fmt.Sprintf("stop must be within [%.2f, %.2f]×ATR(1d) = [%.4f, %.4f] below entry %.4f; yours %.4f is %.2f×ATR.",
				preset.StopATRMin, preset.StopATRMax, entry-preset.StopATRMax*s.ATR1d, entry-preset.StopATRMin*s.ATR1d, entry, d.StopLoss, multiple)
		}
		if !finite(d.TakeProfit) || (d.TakeProfit != 0 && d.TakeProfit <= entry) {
			return "TP_INVALID", fmt.Sprintf("take_profit must be 0 (none) or above the entry price %.4f (yours %.4f).", entry, d.TakeProfit)
		}
		if d.Action == ActionOpenLong && positionCount >= effectiveMaxPositions(ctx.Config) {
			return "MAX_POSITIONS", ""
		}
		if d.Action == ActionAddLong && (s.Price <= p.InitialStop || s.Price <= p.AvgPrice) {
			return "ADD_WHILE_LOSING", ""
		}
	case ActionReduceLong:
		if !finite(d.ReduceFraction) || d.ReduceFraction <= 0 || d.ReduceFraction >= 1 {
			return "REDUCE_INVALID", ""
		}
	case ActionAdjustStop:
		if s == nil || !positive(s.Price) {
			return "DATA_INSUFFICIENT", ""
		}
		if !positive(d.StopLoss) || d.StopLoss >= s.Price {
			return "STOP_INVALID", ""
		}
		if d.StopLoss < p.StopPrice {
			return "STOP_LOOSEN", ""
		}
	}
	return "", ""
}
