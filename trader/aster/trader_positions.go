package aster

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"strconv"
	"strings"
)

// GetPositions Get position information
func (t *AsterTrader) GetPositions() ([]map[string]interface{}, error) {
	params := make(map[string]interface{})
	body, err := t.request("GET", "/fapi/v3/positionRisk", params)
	if err != nil {
		return nil, err
	}

	var positions []map[string]interface{}
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, err
	}

	result := []map[string]interface{}{}
	for _, pos := range positions {
		posAmtStr, ok := pos["positionAmt"].(string)
		if !ok {
			continue
		}

		posAmt, _ := strconv.ParseFloat(posAmtStr, 64)
		if posAmt == 0 {
			continue // Skip empty positions
		}

		entryPrice, _ := strconv.ParseFloat(pos["entryPrice"].(string), 64)
		markPrice, _ := strconv.ParseFloat(pos["markPrice"].(string), 64)
		unRealizedProfit, _ := strconv.ParseFloat(pos["unRealizedProfit"].(string), 64)
		leverageVal, _ := strconv.ParseFloat(pos["leverage"].(string), 64)
		liquidationPrice, _ := strconv.ParseFloat(pos["liquidationPrice"].(string), 64)

		// Determine direction (consistent with Binance)
		side := "long"
		if posAmt < 0 {
			side = "short"
			posAmt = -posAmt
		}

		// Return same field names as Binance
		result = append(result, map[string]interface{}{
			"symbol":           pos["symbol"],
			"side":             side,
			"positionAmt":      posAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unRealizedProfit,
			"leverage":         leverageVal,
			"liquidationPrice": liquidationPrice,
		})
	}

	return result, nil
}

// SetMarginMode Set margin mode
func (t *AsterTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Aster supports margin mode settings
	// API format similar to Binance: CROSSED (cross margin) / ISOLATED (isolated margin)
	marginType := "CROSSED"
	if !isCrossMargin {
		marginType = "ISOLATED"
	}

	params := map[string]interface{}{
		"symbol":     symbol,
		"marginType": marginType,
	}

	// Use request method to call API
	_, err := t.request("POST", "/fapi/v3/marginType", params)
	if err != nil {
		// Ignore error if it indicates no need to change
		if strings.Contains(err.Error(), "No need to change") ||
			strings.Contains(err.Error(), "Margin type cannot be changed") {
			logger.Infof("  ✓ %s margin mode is already %s or cannot be changed due to existing positions", symbol, marginType)
			return nil
		}
		// Detect multi-assets mode (error code -4168)
		if strings.Contains(err.Error(), "Multi-Assets mode") ||
			strings.Contains(err.Error(), "-4168") ||
			strings.Contains(err.Error(), "4168") {
			logger.Infof("  ⚠️ %s detected multi-assets mode, forcing cross margin mode", symbol)
			logger.Infof("  💡 Tip: To use isolated margin mode, please disable multi-assets mode on the exchange")
			return nil
		}
		// Detect unified account API
		if strings.Contains(err.Error(), "unified") ||
			strings.Contains(err.Error(), "portfolio") ||
			strings.Contains(err.Error(), "Portfolio") {
			logger.Infof("  ❌ %s detected unified account API, cannot perform futures trading", symbol)
			return fmt.Errorf("please use 'Spot & Futures Trading' API permission, not 'Unified Account API'")
		}
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Don't return error, let trading continue
		return nil
	}

	logger.Infof("  ✓ %s margin mode has been set to %s", symbol, marginType)
	return nil
}

// SetLeverage Set leverage multiplier
func (t *AsterTrader) SetLeverage(symbol string, leverage int) error {
	params := map[string]interface{}{
		"symbol":   symbol,
		"leverage": leverage,
	}

	_, err := t.request("POST", "/fapi/v3/leverage", params)
	return err
}
