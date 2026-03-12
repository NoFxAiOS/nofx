package kucoin

import (
	"encoding/json"
	"fmt"
	"time"
)

// GetPositions gets all positions
func (t *KuCoinTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	data, err := t.doRequest("GET", kucoinPositionPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positions []struct {
		Symbol           string  `json:"symbol"`
		CurrentQty       int64   `json:"currentQty"`      // Position quantity (in lots, integer)
		AvgEntryPrice    float64 `json:"avgEntryPrice"`   // Average entry price
		MarkPrice        float64 `json:"markPrice"`       // Mark price
		UnrealisedPnl    float64 `json:"unrealisedPnl"`   // Unrealized PnL
		Leverage         float64 `json:"leverage"`        // Leverage setting
		RealLeverage     float64 `json:"realLeverage"`    // Effective leverage (may be nil in cross mode)
		LiquidationPrice float64 `json:"liquidationPrice"`// Liquidation price
		Multiplier       float64 `json:"multiplier"`      // Contract multiplier
		IsOpen           bool    `json:"isOpen"`
		CrossMode        bool    `json:"crossMode"`
		OpeningTimestamp int64   `json:"openingTimestamp"`
		SettleCurrency   string  `json:"settleCurrency"`
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse position data: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		if !pos.IsOpen || pos.CurrentQty == 0 {
			continue
		}

		// Convert symbol format
		symbol := t.convertSymbolBack(pos.Symbol)

		// Determine side based on position quantity
		// KuCoin: positive qty = long, negative qty = short
		side := "long"
		qty := pos.CurrentQty
		if qty < 0 {
			side = "short"
			qty = -qty
		}

		// Convert lots to actual quantity using multiplier
		// Position quantity = lots * multiplier
		multiplier := pos.Multiplier
		if multiplier == 0 {
			multiplier = 0.001 // Default for BTC
		}
		positionAmt := float64(qty) * multiplier

		// Determine margin mode
		mgnMode := "isolated"
		if pos.CrossMode {
			mgnMode = "cross"
		}

		// Use Leverage field (setting), fallback to RealLeverage (effective), default to 10
		leverage := pos.Leverage
		if leverage == 0 {
			leverage = pos.RealLeverage
		}
		if leverage == 0 {
			leverage = 10 // Default leverage
		}

		posMap := map[string]interface{}{
			"symbol":           symbol,
			"positionAmt":      positionAmt,
			"entryPrice":       pos.AvgEntryPrice,
			"markPrice":        pos.MarkPrice,
			"unRealizedProfit": pos.UnrealisedPnl,
			"leverage":         leverage,
			"liquidationPrice": pos.LiquidationPrice,
			"side":             side,
			"mgnMode":          mgnMode,
			"createdTime":      pos.OpeningTimestamp,
		}
		result = append(result, posMap)
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// InvalidatePositionCache clears the position cache
func (t *KuCoinTrader) InvalidatePositionCache() {
	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()
}
