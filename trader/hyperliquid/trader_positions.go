package hyperliquid

import (
	"fmt"
	"nofx/logger"
	"strconv"
	"strings"
)

// GetPositions gets all positions (including xyz dex positions)
func (t *HyperliquidTrader) GetPositions() ([]map[string]interface{}, error) {
	// Get account status
	accountState, err := t.exchange.Info().UserState(t.ctx, t.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}

	// Iterate through all perp positions
	for _, assetPos := range accountState.AssetPositions {
		position := assetPos.Position

		// Position amount (string type)
		posAmt, _ := strconv.ParseFloat(position.Szi, 64)

		if posAmt == 0 {
			continue // Skip positions with zero amount
		}

		posMap := make(map[string]interface{})

		// Normalize symbol format (Hyperliquid uses "BTC", we convert to "BTCUSDT")
		symbol := position.Coin + "USDT"
		posMap["symbol"] = symbol

		// Position amount and direction
		if posAmt > 0 {
			posMap["side"] = "long"
			posMap["positionAmt"] = posAmt
		} else {
			posMap["side"] = "short"
			posMap["positionAmt"] = -posAmt // Convert to positive number
		}

		// Price information (EntryPx and LiquidationPx are pointer types)
		var entryPrice, liquidationPx float64
		if position.EntryPx != nil {
			entryPrice, _ = strconv.ParseFloat(*position.EntryPx, 64)
		}
		if position.LiquidationPx != nil {
			liquidationPx, _ = strconv.ParseFloat(*position.LiquidationPx, 64)
		}

		positionValue, _ := strconv.ParseFloat(position.PositionValue, 64)
		unrealizedPnl, _ := strconv.ParseFloat(position.UnrealizedPnl, 64)

		// Calculate mark price (positionValue / abs(posAmt))
		var markPrice float64
		if posAmt != 0 {
			markPrice = positionValue / absFloat(posAmt)
		}

		posMap["entryPrice"] = entryPrice
		posMap["markPrice"] = markPrice
		posMap["unRealizedProfit"] = unrealizedPnl
		posMap["leverage"] = float64(position.Leverage.Value)
		posMap["liquidationPrice"] = liquidationPx

		result = append(result, posMap)
	}

	// Also get xyz dex positions (stocks, forex, commodities)
	_, _, xyzPositions, err := t.getXYZDexBalance()
	if err != nil {
		// xyz dex query failed - log warning but don't fail
		logger.Infof("⚠️  Failed to get xyz dex positions: %v", err)
	} else {
		for _, pos := range xyzPositions {
			posAmt, _ := strconv.ParseFloat(pos.Position.Szi, 64)
			if posAmt == 0 {
				continue
			}

			posMap := make(map[string]interface{})

			// xyz dex positions - the API returns coin names with xyz: prefix (e.g., "xyz:SILVER")
			// Only add prefix if not already present
			symbol := pos.Position.Coin
			if !strings.HasPrefix(symbol, "xyz:") {
				symbol = "xyz:" + symbol
			}
			posMap["symbol"] = symbol

			if posAmt > 0 {
				posMap["side"] = "long"
				posMap["positionAmt"] = posAmt
			} else {
				posMap["side"] = "short"
				posMap["positionAmt"] = -posAmt
			}

			// Parse price information
			var entryPrice, liquidationPx float64
			if pos.Position.EntryPx != nil {
				entryPrice, _ = strconv.ParseFloat(*pos.Position.EntryPx, 64)
			}
			if pos.Position.LiquidationPx != nil {
				liquidationPx, _ = strconv.ParseFloat(*pos.Position.LiquidationPx, 64)
			}

			positionValue, _ := strconv.ParseFloat(pos.Position.PositionValue, 64)
			unrealizedPnl, _ := strconv.ParseFloat(pos.Position.UnrealizedPnl, 64)

			// Calculate mark price from position value
			var markPrice float64
			if posAmt != 0 {
				markPrice = positionValue / absFloat(posAmt)
			}

			// Get leverage (default to 1 if not available)
			leverage := float64(pos.Position.Leverage.Value)
			if leverage == 0 {
				leverage = 1.0
			}

			posMap["entryPrice"] = entryPrice
			posMap["markPrice"] = markPrice
			posMap["unRealizedProfit"] = unrealizedPnl
			posMap["leverage"] = leverage
			posMap["liquidationPrice"] = liquidationPx
			posMap["isXyzDex"] = true // Mark as xyz dex position

			result = append(result, posMap)
		}
	}

	return result, nil
}

// SetMarginMode sets margin mode (set together with SetLeverage)
func (t *HyperliquidTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Hyperliquid's margin mode is set in SetLeverage, only record here
	t.isCrossMargin = isCrossMargin
	marginModeStr := "cross margin"
	if !isCrossMargin {
		marginModeStr = "isolated margin"
	}
	logger.Infof("  ✓ %s will use %s mode", symbol, marginModeStr)
	return nil
}

// SetLeverage sets leverage
func (t *HyperliquidTrader) SetLeverage(symbol string, leverage int) error {
	// Hyperliquid symbol format (remove USDT suffix)
	coin := convertSymbolToHyperliquid(symbol)

	// Call UpdateLeverage (leverage int, name string, isCross bool)
	// Third parameter: true=cross margin mode, false=isolated margin mode
	_, err := t.exchange.UpdateLeverage(t.ctx, leverage, coin, t.isCrossMargin)
	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	logger.Infof("  ✓ %s leverage switched to %dx", symbol, leverage)
	return nil
}
