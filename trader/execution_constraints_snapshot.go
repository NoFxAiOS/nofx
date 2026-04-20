package trader

import (
	"strings"

	"nofx/kernel"
	"nofx/store"
)

// ExecutionConstraintsSnapshot is the compact, optional execution-data surface
// used for audit/review. It deliberately excludes orderbook ladders,
// liquidation data, OI, funding windows, and any fabricated exchange limits.
type ExecutionConstraintsSnapshot struct {
	TickSize             float64
	PricePrecision       int
	QtyStepSize          float64
	QtyPrecision         int
	MinQty               float64
	MinNotional          float64
	ContractValue        float64
	MarkPrice            float64
	LastPrice            float64
	BestBid              float64
	BestAsk              float64
	SpreadBps            float64
	TakerFeeRate         float64
	MakerFeeRate         float64
	EstimatedSlippageBps float64
	Source               map[string]string
}

type pricePrecisionProvider interface {
	GetSymbolPricePrecision(symbol string) (int, error)
}

type qtyPrecisionProvider interface {
	GetSymbolPrecision(symbol string) (int, error)
}

type instrumentProvider interface {
	GetInstrument(symbol string) (interface{}, error)
}

type executionConstraintsProvider interface {
	GetExecutionConstraints(symbol string) (map[string]float64, error)
}

// collectExecutionConstraintsSnapshot gathers only safe, compact execution fields.
// Missing/unavailable fields remain zero; errors are intentionally non-fatal.
func (at *AutoTrader) collectExecutionConstraintsSnapshot(symbol string) *ExecutionConstraintsSnapshot {
	if at == nil || at.trader == nil || strings.TrimSpace(symbol) == "" {
		return nil
	}
	caps := at.GetMarketDataCapabilities()
	if caps.DegradedProfile && !caps.QuoteLastPrice && !caps.QuoteMarkPrice {
		return nil
	}

	snap := &ExecutionConstraintsSnapshot{Source: map[string]string{}}

	if caps.InstrumentPricePrecision {
		if p, ok := at.trader.(pricePrecisionProvider); ok {
			if v, err := p.GetSymbolPricePrecision(symbol); err == nil && v > 0 {
				snap.PricePrecision = v
				snap.Source["price_precision"] = strings.ToLower(at.exchange) + ":instrument"
			}
		}
	}
	if caps.InstrumentQtyPrecision {
		if p, ok := at.trader.(qtyPrecisionProvider); ok {
			if v, err := p.GetSymbolPrecision(symbol); err == nil && v > 0 {
				snap.QtyPrecision = v
				snap.Source["qty_precision"] = strings.ToLower(at.exchange) + ":instrument"
			}
		}
	}
	if p, ok := at.trader.(executionConstraintsProvider); ok {
		if m, err := p.GetExecutionConstraints(symbol); err == nil {
			setSnapshotMapValue(m, "tick_size", &snap.TickSize, snap.Source, strings.ToLower(at.exchange)+":instrument")
			setSnapshotMapValue(m, "qty_step_size", &snap.QtyStepSize, snap.Source, strings.ToLower(at.exchange)+":instrument")
			setSnapshotMapValue(m, "min_qty", &snap.MinQty, snap.Source, strings.ToLower(at.exchange)+":instrument")
			setSnapshotMapValue(m, "min_notional", &snap.MinNotional, snap.Source, strings.ToLower(at.exchange)+":instrument")
		}
	}
	if strings.EqualFold(at.exchange, "okx") && (caps.InstrumentTickSize || caps.InstrumentQtyStep || caps.InstrumentMinQty || caps.InstrumentContractValue) {
		collectOKXInstrumentSnapshot(at.trader, symbol, snap)
	}

	if caps.QuoteLastPrice {
		if v, err := at.trader.GetMarketPrice(symbol); err == nil && isFinitePositive(v) {
			snap.LastPrice = v
			snap.Source["last_price"] = strings.ToLower(at.exchange) + ":ticker"
		}
	}
	if caps.QuoteBestBid || caps.QuoteBestAsk || caps.QuoteSpread {
		if g, ok := at.trader.(interface {
			GetOrderBook(string, int) ([][]float64, [][]float64, error)
		}); ok {
			bids, asks, err := g.GetOrderBook(symbol, 1)
			if err == nil {
				if len(bids) > 0 && len(bids[0]) > 0 && isFinitePositive(bids[0][0]) {
					snap.BestBid = bids[0][0]
					snap.Source["best_bid"] = strings.ToLower(at.exchange) + ":top_of_book"
				}
				if len(asks) > 0 && len(asks[0]) > 0 && isFinitePositive(asks[0][0]) {
					snap.BestAsk = asks[0][0]
					snap.Source["best_ask"] = strings.ToLower(at.exchange) + ":top_of_book"
				}
			}
		}
	}
	if caps.QuoteSpread && snap.BestBid > 0 && snap.BestAsk > 0 {
		mid := (snap.BestBid + snap.BestAsk) / 2
		if mid > 0 && snap.BestAsk >= snap.BestBid {
			snap.SpreadBps = ((snap.BestAsk - snap.BestBid) / mid) * 10000
			snap.Source["spread_bps"] = strings.ToLower(at.exchange) + ":top_of_book"
		}
	}

	if len(snap.Source) == 0 {
		return nil
	}
	return snap
}

func collectOKXInstrumentSnapshot(tr interface{}, symbol string, snap *ExecutionConstraintsSnapshot) {
	p, ok := tr.(interface {
		GetInstrument(string) (interface{}, error)
	})
	if !ok || snap == nil {
		return
	}
	inst, err := p.GetInstrument(symbol)
	if err != nil || inst == nil {
		return
	}
	// Avoid broad interface churn and keep adapter coupling narrow: use compact
	// structural access through known exported field names on the OKX instrument.
	v := reflectValue(inst)
	setFloatFromField(v, "TickSz", &snap.TickSize, snap.Source, "tick_size", "okx:instrument")
	setFloatFromField(v, "LotSz", &snap.QtyStepSize, snap.Source, "qty_step_size", "okx:instrument")
	setFloatFromField(v, "MinSz", &snap.MinQty, snap.Source, "min_qty", "okx:instrument")
	setFloatFromField(v, "CtVal", &snap.ContractValue, snap.Source, "contract_value", "okx:instrument")
}

func mapExecutionConstraintsToActionReview(s *ExecutionConstraintsSnapshot) *store.DecisionActionExecutionConstraints {
	if s == nil {
		return nil
	}
	out := &store.DecisionActionExecutionConstraints{
		TickSize:             s.TickSize,
		PricePrecision:       s.PricePrecision,
		QtyStepSize:          s.QtyStepSize,
		QtyPrecision:         s.QtyPrecision,
		MinQty:               s.MinQty,
		MinNotional:          s.MinNotional,
		ContractValue:        s.ContractValue,
		MarkPrice:            s.MarkPrice,
		LastPrice:            s.LastPrice,
		BestBid:              s.BestBid,
		BestAsk:              s.BestAsk,
		SpreadBps:            s.SpreadBps,
		TakerFeeRate:         s.TakerFeeRate,
		MakerFeeRate:         s.MakerFeeRate,
		EstimatedSlippageBps: s.EstimatedSlippageBps,
	}
	if out.TickSize == 0 && out.PricePrecision == 0 && out.QtyStepSize == 0 && out.QtyPrecision == 0 && out.MinQty == 0 && out.MinNotional == 0 && out.ContractValue == 0 && out.MarkPrice == 0 && out.LastPrice == 0 && out.BestBid == 0 && out.BestAsk == 0 && out.SpreadBps == 0 && out.TakerFeeRate == 0 && out.MakerFeeRate == 0 && out.EstimatedSlippageBps == 0 {
		return nil
	}
	return out
}

func mapExecutionConstraintsToKernel(s *ExecutionConstraintsSnapshot) kernel.AIEntryExecutionConstraints {
	if s == nil {
		return kernel.AIEntryExecutionConstraints{}
	}
	return kernel.AIEntryExecutionConstraints{
		TickSize:             s.TickSize,
		PricePrecision:       s.PricePrecision,
		QtyStepSize:          s.QtyStepSize,
		QtyPrecision:         s.QtyPrecision,
		MinQty:               s.MinQty,
		MinNotional:          s.MinNotional,
		MarkPrice:            s.MarkPrice,
		LastPrice:            s.LastPrice,
		BestBid:              s.BestBid,
		BestAsk:              s.BestAsk,
		SpreadBps:            s.SpreadBps,
		TakerFeeRate:         s.TakerFeeRate,
		MakerFeeRate:         s.MakerFeeRate,
		EstimatedSlippageBps: s.EstimatedSlippageBps,
	}
}

func executionConstraintsEmpty(c kernel.AIEntryExecutionConstraints) bool {
	return c.TickSize == 0 && c.PricePrecision == 0 && c.QtyStepSize == 0 && c.QtyPrecision == 0 && c.MinQty == 0 && c.MinNotional == 0 && c.MarkPrice == 0 && c.LastPrice == 0 && c.IndexPrice == 0 && c.BestBid == 0 && c.BestAsk == 0 && c.SpreadBps == 0 && c.TakerFeeRate == 0 && c.MakerFeeRate == 0 && c.EstimatedSlippageBps == 0
}

func mergeExecutionConstraints(decision *kernel.Decision, snap *ExecutionConstraintsSnapshot) bool {
	if decision == nil || decision.EntryProtection == nil || snap == nil {
		return false
	}
	if executionConstraintsEmpty(decision.EntryProtection.ExecutionConstraints) {
		decision.EntryProtection.ExecutionConstraints = mapExecutionConstraintsToKernel(snap)
		return !executionConstraintsEmpty(decision.EntryProtection.ExecutionConstraints)
	}
	return false
}
