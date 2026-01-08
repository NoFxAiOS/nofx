package backtest

import (
	"fmt"
	"math"
	"strings"
)

const epsilon = 1e-8

type position struct {
	Symbol           string
	Side             string
	Quantity         float64
	EntryPrice       float64
	Leverage         int
	Margin           float64
	Notional         float64
	LiquidationPrice float64
	OpenTime         int64
	AccumulatedFee   float64 // Total fees paid (opening + any additions)
}

type BacktestAccount struct {
	initialBalance float64
	cash           float64
	feeRate        float64
	slippageRate   float64
	positions      map[string]*position
	realizedPnL    float64
}

func NewBacktestAccount(initialBalance, feeBps, slippageBps float64) *BacktestAccount {
	return &BacktestAccount{
		initialBalance: initialBalance,
		cash:           initialBalance,
		feeRate:        feeBps / 10000.0,
		slippageRate:   slippageBps / 10000.0,
		positions:      make(map[string]*position),
	}
}

func positionKey(symbol, side string) string {
	return strings.ToUpper(symbol) + ":" + side
}

func (acc *BacktestAccount) ensurePosition(symbol, side string) *position {
	key := positionKey(symbol, side)
	if pos, ok := acc.positions[key]; ok {
		return pos
	}
	pos := &position{Symbol: strings.ToUpper(symbol), Side: side}
	acc.positions[key] = pos
	return pos
}

func (acc *BacktestAccount) removePosition(pos *position) {
	key := positionKey(pos.Symbol, pos.Side)
	delete(acc.positions, key)
}

func (acc *BacktestAccount) Open(symbol, side string, quantity float64, leverage int, price float64, ts int64) (*position, float64, float64, error) {
	if quantity <= 0 {
		return nil, 0, 0, fmt.Errorf("quantity must be positive")
	}
	if leverage <= 0 {
		return nil, 0, 0, fmt.Errorf("leverage must be positive")
	}

	execPrice := applySlippage(price, acc.slippageRate, side, true)
	notional := execPrice * quantity
	margin := notional / float64(leverage)
	fee := notional * acc.feeRate

	if margin+fee > acc.cash+epsilon {
		return nil, 0, 0, fmt.Errorf("insufficient cash: need %.2f", margin+fee)
	}

	acc.cash -= margin + fee

	pos := acc.ensurePosition(symbol, side)

	if pos.Quantity < epsilon {
		pos.Quantity = quantity
		pos.EntryPrice = execPrice
		pos.Leverage = leverage
		pos.Margin = margin
		pos.Notional = notional
		pos.OpenTime = ts
		pos.LiquidationPrice = computeLiquidation(execPrice, leverage, side)
		pos.AccumulatedFee = fee // Track opening fee
	} else {
		if leverage != pos.Leverage {
			// Use weighted average leverage (approximate)
			weightedMargin := pos.Margin + margin
			pos.Leverage = int(math.Round((pos.Notional + notional) / weightedMargin))
		}
		pos.Notional += notional
		pos.Margin += margin
		pos.EntryPrice = ((pos.EntryPrice * pos.Quantity) + execPrice*quantity) / (pos.Quantity + quantity)
		pos.Quantity += quantity
		pos.LiquidationPrice = computeLiquidation(pos.EntryPrice, pos.Leverage, side)
		pos.AccumulatedFee += fee // Add to accumulated fee for position additions
	}

	return pos, fee, execPrice, nil
}

func (acc *BacktestAccount) Close(symbol, side string, quantity float64, price float64) (float64, float64, float64, error) {
	key := positionKey(symbol, side)
	pos, ok := acc.positions[key]
	if !ok || pos.Quantity <= epsilon {
		return 0, 0, 0, fmt.Errorf("no active %s position for %s", side, symbol)
	}

	if quantity <= 0 || quantity > pos.Quantity+epsilon {
		if math.Abs(quantity) <= epsilon {
			quantity = pos.Quantity
		} else {
			return 0, 0, 0, fmt.Errorf("invalid close quantity")
		}
	}

	execPrice := applySlippage(price, acc.slippageRate, side, false)
	closeNotional := execPrice * quantity // Notional at close price (for fee calculation)
	closingFee := closeNotional * acc.feeRate

	// Calculate proportional values based on the portion being closed
	closePortion := quantity / pos.Quantity
	openingFeePortion := pos.AccumulatedFee * closePortion
	totalFee := closingFee + openingFeePortion

	realized := realizedPnL(pos, quantity, execPrice)

	marginPortion := pos.Margin * closePortion
	// BUG FIX: Calculate notional portion based on ENTRY price, not close price
	// pos.Notional tracks the total notional at entry, so we must subtract proportionally
	entryNotionalPortion := pos.Notional * closePortion

	// Note: Opening fee was already deducted from cash when opening, so we only deduct closing fee here
	acc.cash += marginPortion + realized - closingFee
	// But for realized P&L tracking, we include both fees
	acc.realizedPnL += realized - totalFee

	pos.Quantity -= quantity
	pos.Notional -= entryNotionalPortion // FIX: Use entry notional portion, not close notional
	pos.Margin -= marginPortion
	pos.AccumulatedFee -= openingFeePortion // Reduce tracked opening fee

	if pos.Quantity <= epsilon {
		acc.removePosition(pos)
	}

	// Return total fee (opening + closing) so caller can calculate accurate P&L
	return realized, totalFee, execPrice, nil
}

func (acc *BacktestAccount) TotalEquity(priceMap map[string]float64) (float64, float64, map[string]float64) {
	unrealized := 0.0
	margin := 0.0
	perSymbol := make(map[string]float64)
	for _, pos := range acc.positions {
		price := priceMap[pos.Symbol]
		pnl := unrealizedPnL(pos, price)
		unrealized += pnl
		margin += pos.Margin
		perSymbol[pos.Symbol+":"+pos.Side] = pnl
	}
	return acc.cash + margin + unrealized, unrealized, perSymbol
}

func applySlippage(price float64, rate float64, side string, isOpen bool) float64 {
	if rate <= 0 {
		return price
	}
	adjust := 1.0
	if side == "long" {
		if isOpen {
			adjust += rate
		} else {
			adjust -= rate
		}
	} else {
		if isOpen {
			adjust -= rate
		} else {
			adjust += rate
		}
	}
	return price * adjust
}

func computeLiquidation(entry float64, leverage int, side string) float64 {
	if leverage <= 0 {
		return 0
	}
	lev := float64(leverage)
	if side == "long" {
		return entry * (1.0 - 1.0/lev)
	}
	return entry * (1.0 + 1.0/lev)
}

func realizedPnL(pos *position, qty, price float64) float64 {
	if pos.Side == "long" {
		return (price - pos.EntryPrice) * qty
	}
	return (pos.EntryPrice - price) * qty
}

func unrealizedPnL(pos *position, price float64) float64 {
	if pos.Side == "long" {
		return (price - pos.EntryPrice) * pos.Quantity
	}
	return (pos.EntryPrice - price) * pos.Quantity
}

func (acc *BacktestAccount) Positions() []*position {
	list := make([]*position, 0, len(acc.positions))
	for _, pos := range acc.positions {
		list = append(list, pos)
	}
	return list
}

func (acc *BacktestAccount) positionLeverage(symbol, side string) int {
	key := positionKey(symbol, side)
	if pos, ok := acc.positions[key]; ok && pos.Quantity > epsilon {
		return pos.Leverage
	}
	return 0
}

func (acc *BacktestAccount) Cash() float64 {
	return acc.cash
}

func (acc *BacktestAccount) InitialBalance() float64 {
	return acc.initialBalance
}

func (acc *BacktestAccount) RealizedPnL() float64 {
	return acc.realizedPnL
}

// RestoreFromSnapshots restores account state from checkpoint.
func (acc *BacktestAccount) RestoreFromSnapshots(cash float64, realized float64, snaps []PositionSnapshot) {
	acc.cash = cash
	acc.realizedPnL = realized
	acc.positions = make(map[string]*position)
	for _, snap := range snaps {
		pos := &position{
			Symbol:           snap.Symbol,
			Side:             snap.Side,
			Quantity:         snap.Quantity,
			EntryPrice:       snap.AvgPrice,
			Leverage:         snap.Leverage,
			Margin:           snap.MarginUsed,
			Notional:         snap.Quantity * snap.AvgPrice,
			LiquidationPrice: snap.LiquidationPrice,
			OpenTime:         snap.OpenTime,
			AccumulatedFee:   snap.AccumulatedFee,
		}
		key := positionKey(pos.Symbol, pos.Side)
		acc.positions[key] = pos
	}
}
