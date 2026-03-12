package gate

import (
	"fmt"
	"nofx/trader/types"
	"strconv"
	"time"

	"github.com/antihax/optional"
	"github.com/gateio/gateapi-go/v6"
)

// GetBalance retrieves account balance
func (t *GateTrader) GetBalance() (map[string]interface{}, error) {
	// Check cache
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cached := t.cachedBalance
		t.balanceCacheMutex.RUnlock()
		return cached, nil
	}
	t.balanceCacheMutex.RUnlock()

	// Fetch from API
	accounts, _, err := t.client.FuturesApi.ListFuturesAccounts(t.ctx, "usdt")
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	total, _ := strconv.ParseFloat(accounts.Total, 64)
	available, _ := strconv.ParseFloat(accounts.Available, 64)
	unrealizedPnl, _ := strconv.ParseFloat(accounts.UnrealisedPnl, 64)

	result := map[string]interface{}{
		"totalWalletBalance":    total,
		"availableBalance":      available,
		"totalUnrealizedProfit": unrealizedPnl,
	}

	// Update cache
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions retrieves all open positions
func (t *GateTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cached := t.cachedPositions
		t.positionsCacheMutex.RUnlock()
		return cached, nil
	}
	t.positionsCacheMutex.RUnlock()

	// Fetch from API
	positions, _, err := t.client.FuturesApi.ListPositions(t.ctx, "usdt", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		if pos.Size == 0 {
			continue // Skip empty positions
		}

		entryPrice, _ := strconv.ParseFloat(pos.EntryPrice, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiqPrice, 64)
		unrealizedPnl, _ := strconv.ParseFloat(pos.UnrealisedPnl, 64)
		leverage, _ := strconv.ParseFloat(pos.Leverage, 64)

		// Gate returns position size in contracts, need to convert to base currency
		// Each contract = quanto_multiplier base currency
		contractSize := float64(pos.Size)
		if pos.Size < 0 {
			contractSize = float64(-pos.Size)
		}

		// Get quanto_multiplier from contract info to convert contracts to actual quantity
		quantoMultiplier := 1.0
		contract, err := t.getContract(pos.Contract)
		if err == nil && contract != nil {
			qm, _ := strconv.ParseFloat(contract.QuantoMultiplier, 64)
			if qm > 0 {
				quantoMultiplier = qm
			}
		}

		// Convert contract count to actual token quantity
		positionAmt := contractSize * quantoMultiplier

		// Determine side based on position size
		side := "long"
		if pos.Size < 0 {
			side = "short"
		}

		result = append(result, map[string]interface{}{
			"symbol":           pos.Contract,
			"positionAmt":      positionAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealizedPnl,
			"leverage":         int(leverage),
			"liquidationPrice": liqPrice,
			"side":             side,
		})
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// GetClosedPnL retrieves closed position PnL records
func (t *GateTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	opts := &gateapi.ListPositionCloseOpts{
		Limit: optional.NewInt32(int32(limit)),
		From:  optional.NewInt64(startTime.Unix()),
	}

	closedPositions, _, err := t.client.FuturesApi.ListPositionClose(t.ctx, "usdt", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get closed positions: %w", err)
	}

	records := make([]types.ClosedPnLRecord, 0, len(closedPositions))
	for _, pos := range closedPositions {
		pnl, _ := strconv.ParseFloat(pos.Pnl, 64)

		record := types.ClosedPnLRecord{
			Symbol:      t.revertSymbol(pos.Contract),
			Side:        pos.Side,
			RealizedPnL: pnl,
			ExitTime:    time.Unix(int64(pos.Time), 0).UTC(),
			CloseType:   "unknown",
		}

		records = append(records, record)
	}

	return records, nil
}
