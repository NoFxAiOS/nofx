package bitget

import (
	"encoding/json"
	"fmt"
	"nofx/trader/types"
	"strconv"
	"time"
)

// GetPositions gets all positions
func (t *BitgetTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
	}

	data, err := t.doRequest("GET", bitgetPositionPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positions []struct {
		Symbol           string `json:"symbol"`
		HoldSide         string `json:"holdSide"`         // long, short
		OpenPriceAvg     string `json:"openPriceAvg"`     // Average entry price
		MarkPrice        string `json:"markPrice"`        // Mark price
		Total            string `json:"total"`            // Total position size
		Available        string `json:"available"`        // Available to close
		UnrealizedPL     string `json:"unrealizedPL"`     // Unrealized P&L
		Leverage         string `json:"leverage"`         // Leverage
		LiquidationPrice string `json:"liquidationPrice"` // Liquidation price
		MarginSize       string `json:"marginSize"`       // Position margin
		CTime            string `json:"cTime"`            // Create time
		UTime            string `json:"uTime"`            // Update time
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse position data: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		total, _ := strconv.ParseFloat(pos.Total, 64)
		if total == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(pos.OpenPriceAvg, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64)
		unrealizedPnL, _ := strconv.ParseFloat(pos.UnrealizedPL, 64)
		leverage, _ := strconv.ParseFloat(pos.Leverage, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiquidationPrice, 64)
		cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)

		// Normalize side
		side := "long"
		if pos.HoldSide == "short" {
			side = "short"
		}

		posMap := map[string]interface{}{
			"symbol":           pos.Symbol,
			"positionAmt":      total,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealizedPnL,
			"leverage":         leverage,
			"liquidationPrice": liqPrice,
			"side":             side,
			"createdTime":      cTime,
			"updatedTime":      uTime,
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

// GetClosedPnL retrieves closed position PnL records
func (t *BitgetTrader) GetClosedPnL(startTime time.Time, limit int) ([]types.ClosedPnLRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	params := map[string]interface{}{
		"productType": "USDT-FUTURES",
		"startTime":   fmt.Sprintf("%d", startTime.UnixMilli()),
		"limit":       fmt.Sprintf("%d", limit),
	}

	data, err := t.doRequest("GET", "/api/v2/mix/position/history-position", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions history: %w", err)
	}

	var resp struct {
		List []struct {
			Symbol          string `json:"symbol"`
			HoldSide        string `json:"holdSide"`
			OpenPriceAvg    string `json:"openPriceAvg"`
			ClosePriceAvg   string `json:"closePriceAvg"`
			CloseVol        string `json:"closeVol"`
			AchievedProfits string `json:"achievedProfits"`
			TotalFee        string `json:"totalFee"`
			Leverage        string `json:"leverage"`
			CTime           string `json:"cTime"`
			UTime           string `json:"uTime"`
		} `json:"list"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	records := make([]types.ClosedPnLRecord, 0, len(resp.List))
	for _, pos := range resp.List {
		record := types.ClosedPnLRecord{
			Symbol: pos.Symbol,
			Side:   pos.HoldSide,
		}

		record.EntryPrice, _ = strconv.ParseFloat(pos.OpenPriceAvg, 64)
		record.ExitPrice, _ = strconv.ParseFloat(pos.ClosePriceAvg, 64)
		record.Quantity, _ = strconv.ParseFloat(pos.CloseVol, 64)
		record.RealizedPnL, _ = strconv.ParseFloat(pos.AchievedProfits, 64)
		fee, _ := strconv.ParseFloat(pos.TotalFee, 64)
		record.Fee = -fee
		lev, _ := strconv.ParseFloat(pos.Leverage, 64)
		record.Leverage = int(lev)

		cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)
		record.EntryTime = time.UnixMilli(cTime).UTC()
		record.ExitTime = time.UnixMilli(uTime).UTC()

		record.CloseType = "unknown"
		records = append(records, record)
	}

	return records, nil
}
