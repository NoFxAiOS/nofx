package trader

import (
	"fmt"
	"sort"
	"time"
)

// =============================================================================
// Unified Position Rebuild Algorithm
// All exchanges use this same algorithm to reconstruct position history from trades
// =============================================================================

// openTradeEntry represents an opening trade for position tracking
type openTradeEntry struct {
	Price    float64
	Quantity float64
	Fee      float64
	Time     time.Time
	TradeID  string
}

// positionState tracks open trades for a symbol+side combination
type positionState struct {
	OpenTrades []openTradeEntry
	TotalQty   float64
}

// RebuildPositionsFromTrades reconstructs complete position records from trade history
// This is the unified algorithm used by all exchanges
//
// Algorithm:
// 1. Sort trades by time
// 2. For each trade, determine if it's opening or closing based on RealizedPnL
// 3. Opening trade (RealizedPnL == 0): Add to open trades list
// 4. Closing trade (RealizedPnL != 0): Match with open trades using FIFO, generate position record
//
// The algorithm handles:
// - Partial opens (multiple trades to build a position)
// - Partial closes (multiple trades to close a position)
// - Both hedge mode (LONG/SHORT) and one-way mode (BOTH)
func RebuildPositionsFromTrades(trades []TradeRecord) []ClosedPnLRecord {
	if len(trades) == 0 {
		return nil
	}

	// Sort trades by time
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].Time.Before(trades[j].Time)
	})

	// Track positions by symbol_side
	positions := make(map[string]*positionState)
	var records []ClosedPnLRecord

	for _, trade := range trades {
		// Determine position side
		side := determinePositionSide(trade)
		if side == "" {
			continue // Skip invalid trades
		}

		key := fmt.Sprintf("%s_%s", trade.Symbol, side)
		if positions[key] == nil {
			positions[key] = &positionState{}
		}
		state := positions[key]

		if trade.RealizedPnL == 0 {
			// Opening trade: add to open trades list
			state.OpenTrades = append(state.OpenTrades, openTradeEntry{
				Price:    trade.Price,
				Quantity: trade.Quantity,
				Fee:      trade.Fee,
				Time:     trade.Time,
				TradeID:  trade.TradeID,
			})
			state.TotalQty += trade.Quantity
		} else {
			// Closing trade: generate position record
			record := buildClosedPosition(trade, side, state)
			if record != nil {
				records = append(records, *record)
			}
		}
	}

	return records
}

// determinePositionSide determines the position side from a trade
func determinePositionSide(trade TradeRecord) string {
	// Hedge mode: use PositionSide directly
	switch trade.PositionSide {
	case "LONG", "long":
		return "long"
	case "SHORT", "short":
		return "short"
	}

	// One-way mode (BOTH or empty): determine from trade direction and RealizedPnL
	if trade.RealizedPnL == 0 {
		// Opening trade
		if trade.Side == "BUY" || trade.Side == "Buy" {
			return "long"
		} else if trade.Side == "SELL" || trade.Side == "Sell" {
			return "short"
		}
	} else {
		// Closing trade
		if trade.Side == "BUY" || trade.Side == "Buy" {
			return "short" // Buy to close short
		} else if trade.Side == "SELL" || trade.Side == "Sell" {
			return "long" // Sell to close long
		}
	}

	return ""
}

// buildClosedPosition builds a closed position record from a closing trade
func buildClosedPosition(trade TradeRecord, side string, state *positionState) *ClosedPnLRecord {
	var entryPrice float64
	var entryTime time.Time
	var totalEntryFee float64

	if len(state.OpenTrades) > 0 {
		// Use FIFO to match open trades
		remainingQty := trade.Quantity
		var weightedSum float64
		var matchedQty float64

		for i := 0; i < len(state.OpenTrades) && remainingQty > 0.00000001; i++ {
			ot := &state.OpenTrades[i]
			matchQty := ot.Quantity
			if matchQty > remainingQty {
				matchQty = remainingQty
			}

			weightedSum += ot.Price * matchQty
			matchedQty += matchQty
			totalEntryFee += ot.Fee * (matchQty / ot.Quantity)

			if entryTime.IsZero() {
				entryTime = ot.Time
			}

			remainingQty -= matchQty
			ot.Quantity -= matchQty

			// Remove fully consumed open trade
			if ot.Quantity <= 0.00000001 {
				state.OpenTrades = append(state.OpenTrades[:i], state.OpenTrades[i+1:]...)
				i--
			}
		}

		if matchedQty > 0.00000001 {
			entryPrice = weightedSum / matchedQty
		}
		state.TotalQty -= trade.Quantity
	}

	// If no open trades found (history incomplete), calculate entry price from PnL
	if entryPrice == 0 && trade.Quantity > 0 {
		// PnL = (exitPrice - entryPrice) * qty for LONG
		// PnL = (entryPrice - exitPrice) * qty for SHORT
		if side == "long" {
			entryPrice = trade.Price - trade.RealizedPnL/trade.Quantity
		} else {
			entryPrice = trade.Price + trade.RealizedPnL/trade.Quantity
		}
		entryTime = trade.Time // Use exit time as fallback
	}

	// Validate data
	if entryPrice <= 0 || trade.Price <= 0 || trade.Quantity <= 0 {
		return nil
	}

	return &ClosedPnLRecord{
		Symbol:      trade.Symbol,
		Side:        side,
		EntryPrice:  entryPrice,
		ExitPrice:   trade.Price,
		Quantity:    trade.Quantity,
		RealizedPnL: trade.RealizedPnL,
		Fee:         trade.Fee + totalEntryFee,
		EntryTime:   entryTime,
		ExitTime:    trade.Time,
		OrderID:     trade.TradeID,
		ExchangeID:  trade.TradeID,
		CloseType:   "unknown",
	}
}
