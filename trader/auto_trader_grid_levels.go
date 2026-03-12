package trader

import (
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
)

// ============================================================================
// Grid Level Calculation and Rebalancing
// ============================================================================

// calculateDefaultBounds calculates default bounds based on price
func (at *AutoTrader) calculateDefaultBounds(price float64, config *store.GridStrategyConfig) {
	// Default: +/-3% from current price
	multiplier := 0.03 * float64(config.GridCount) / 10
	at.gridState.UpperPrice = price * (1 + multiplier)
	at.gridState.LowerPrice = price * (1 - multiplier)
}

// calculateATRBounds calculates bounds using ATR
func (at *AutoTrader) calculateATRBounds(price float64, mktData *market.Data, config *store.GridStrategyConfig) {
	atr := 0.0
	if mktData.LongerTermContext != nil {
		atr = mktData.LongerTermContext.ATR14
	}

	if atr <= 0 {
		at.calculateDefaultBounds(price, config)
		return
	}

	multiplier := config.ATRMultiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}

	halfRange := atr * multiplier
	at.gridState.UpperPrice = price + halfRange
	at.gridState.LowerPrice = price - halfRange
}

// initializeGridLevels creates the grid level structure
func (at *AutoTrader) initializeGridLevels(currentPrice float64, config *store.GridStrategyConfig) {
	levels := make([]kernel.GridLevelInfo, config.GridCount)
	totalWeight := 0.0
	weights := make([]float64, config.GridCount)

	// Calculate weights based on distribution
	for i := 0; i < config.GridCount; i++ {
		switch config.Distribution {
		case "gaussian":
			// Gaussian distribution - more weight in the middle
			center := float64(config.GridCount-1) / 2
			sigma := float64(config.GridCount) / 4
			weights[i] = math.Exp(-math.Pow(float64(i)-center, 2) / (2 * sigma * sigma))
		case "pyramid":
			// Pyramid - more weight at bottom
			weights[i] = float64(config.GridCount - i)
		default: // uniform
			weights[i] = 1.0
		}
		totalWeight += weights[i]
	}

	// Create levels
	for i := 0; i < config.GridCount; i++ {
		price := at.gridState.LowerPrice + float64(i)*at.gridState.GridSpacing
		allocatedUSD := config.TotalInvestment * weights[i] / totalWeight

		// Determine initial side (below current price = buy, above = sell)
		side := "buy"
		if price > currentPrice {
			side = "sell"
		}

		levels[i] = kernel.GridLevelInfo{
			Index:        i,
			Price:        price,
			State:        "empty",
			Side:         side,
			AllocatedUSD: allocatedUSD,
		}
	}

	at.gridState.Levels = levels

	// Apply direction-based side assignment if enabled
	if config.EnableDirectionAdjust {
		at.applyGridDirection(currentPrice)
	}
}

// applyGridDirection adjusts grid level sides based on the current direction
// This redistributes buy/sell levels according to the direction bias ratio
func (at *AutoTrader) applyGridDirection(currentPrice float64) {
	config := at.gridState.Config
	direction := at.gridState.CurrentDirection

	// Get bias ratio from config, default to 0.7 (70%/30%)
	biasRatio := config.DirectionBiasRatio
	if biasRatio <= 0 || biasRatio > 1 {
		biasRatio = 0.7
	}

	buyRatio, _ := direction.GetBuySellRatio(biasRatio)

	// Calculate how many levels should be buy vs sell based on direction
	totalLevels := len(at.gridState.Levels)
	targetBuyLevels := int(float64(totalLevels) * buyRatio)

	// For neutral: use price-based assignment (buy below, sell above)
	if direction == market.GridDirectionNeutral {
		for i := range at.gridState.Levels {
			if at.gridState.Levels[i].Price <= currentPrice {
				at.gridState.Levels[i].Side = "buy"
			} else {
				at.gridState.Levels[i].Side = "sell"
			}
		}
		return
	}

	// For long/long_bias: more buy levels
	// For short/short_bias: more sell levels
	switch direction {
	case market.GridDirectionLong:
		// 100% buy - all levels are buy
		for i := range at.gridState.Levels {
			at.gridState.Levels[i].Side = "buy"
		}

	case market.GridDirectionShort:
		// 100% sell - all levels are sell
		for i := range at.gridState.Levels {
			at.gridState.Levels[i].Side = "sell"
		}

	case market.GridDirectionLongBias, market.GridDirectionShortBias:
		// Assign sides based on position relative to current price
		// For long_bias: keep all below as buy, convert some above to buy
		// For short_bias: keep all above as sell, convert some below to sell
		buyCount := 0
		sellCount := 0

		for i := range at.gridState.Levels {
			needMoreBuys := buyCount < targetBuyLevels
			needMoreSells := sellCount < (totalLevels - targetBuyLevels)

			if at.gridState.Levels[i].Price <= currentPrice {
				// Level below or at current price
				if needMoreBuys {
					at.gridState.Levels[i].Side = "buy"
					buyCount++
				} else {
					at.gridState.Levels[i].Side = "sell"
					sellCount++
				}
			} else {
				// Level above current price
				if needMoreSells && direction == market.GridDirectionShortBias {
					at.gridState.Levels[i].Side = "sell"
					sellCount++
				} else if needMoreBuys && direction == market.GridDirectionLongBias {
					at.gridState.Levels[i].Side = "buy"
					buyCount++
				} else if needMoreSells {
					at.gridState.Levels[i].Side = "sell"
					sellCount++
				} else {
					at.gridState.Levels[i].Side = "buy"
					buyCount++
				}
			}
		}
	}

	logger.Infof("[Grid] Applied direction %s: buy_ratio=%.0f%%, levels reconfigured",
		direction, buyRatio*100)
}

// checkGridSkew checks if grid is heavily skewed (too many fills on one side)
// Returns: (skewed bool, buyFilledCount int, sellFilledCount int)
func (at *AutoTrader) checkGridSkew() (bool, int, int) {
	at.gridState.mu.RLock()
	defer at.gridState.mu.RUnlock()

	buyFilled := 0
	sellFilled := 0
	buyEmpty := 0
	sellEmpty := 0

	for _, level := range at.gridState.Levels {
		if level.Side == "buy" {
			if level.State == "filled" {
				buyFilled++
			} else if level.State == "empty" {
				buyEmpty++
			}
		} else {
			if level.State == "filled" {
				sellFilled++
			} else if level.State == "empty" {
				sellEmpty++
			}
		}
	}

	// Grid is skewed if one side has 3x more fills than the other
	// or if one side is completely empty
	skewed := false
	if buyFilled > 0 && sellFilled == 0 && sellEmpty > 5 {
		skewed = true // All buys filled, no sells
	} else if sellFilled > 0 && buyFilled == 0 && buyEmpty > 5 {
		skewed = true // All sells filled, no buys
	} else if buyFilled >= 3*sellFilled && buyFilled > 5 {
		skewed = true
	} else if sellFilled >= 3*buyFilled && sellFilled > 5 {
		skewed = true
	}

	return skewed, buyFilled, sellFilled
}

// autoAdjustGrid automatically adjusts grid when heavily skewed
func (at *AutoTrader) autoAdjustGrid() {
	skewed, buyFilled, sellFilled := at.checkGridSkew()
	if !skewed {
		return
	}

	logger.Warnf("[Grid] Grid heavily skewed: buy_filled=%d, sell_filled=%d. Auto-adjusting...",
		buyFilled, sellFilled)

	gridConfig := at.config.StrategyConfig.GridConfig

	// Get current price
	currentPrice, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		logger.Errorf("[Grid] Failed to get price for auto-adjust: %v", err)
		return
	}

	// Check if price is near grid boundary
	at.gridState.mu.RLock()
	upper := at.gridState.UpperPrice
	lower := at.gridState.LowerPrice
	at.gridState.mu.RUnlock()

	// Only adjust if price has moved significantly (>30% of grid range)
	gridRange := upper - lower
	midPrice := (upper + lower) / 2
	priceDeviation := math.Abs(currentPrice - midPrice)

	if priceDeviation < gridRange*0.3 {
		return // Price still near center, don't adjust
	}

	logger.Infof("[Grid] Adjusting grid around new price $%.2f", currentPrice)

	// Cancel existing orders first (before taking the lock for state modification)
	if err := at.cancelAllGridOrders(); err != nil {
		logger.Errorf("[Grid] Failed to cancel orders during auto-adjust: %v", err)
		// Continue with adjustment anyway
	}

	// CRITICAL FIX: Hold lock for the entire adjustment operation to ensure atomicity
	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	// Preserve filled positions before reinitializing
	filledPositions := make(map[int]kernel.GridLevelInfo)
	for i, level := range at.gridState.Levels {
		if level.State == "filled" {
			filledPositions[i] = level
		}
	}

	// CRITICAL FIX: Recalculate grid bounds centered on current price
	// Use the same logic as InitializeGrid() - either ATR-based or default percentage
	if gridConfig.UseATRBounds {
		// Try to get ATR for bound calculation
		mktData, err := market.GetWithTimeframes(gridConfig.Symbol, []string{"4h"}, "4h", 20)
		if err != nil {
			logger.Warnf("[Grid] Failed to get market data for ATR during adjust: %v, using default bounds", err)
			at.calculateDefaultBoundsLocked(currentPrice, gridConfig)
		} else {
			at.calculateATRBoundsLocked(currentPrice, mktData, gridConfig)
		}
	} else {
		// Use default bounds calculation (scaled by grid count)
		at.calculateDefaultBoundsLocked(currentPrice, gridConfig)
	}

	// Recalculate grid spacing based on new bounds
	at.gridState.GridSpacing = (at.gridState.UpperPrice - at.gridState.LowerPrice) / float64(gridConfig.GridCount-1)

	logger.Infof("[Grid] New bounds: $%.2f - $%.2f, spacing: $%.2f",
		at.gridState.LowerPrice, at.gridState.UpperPrice, at.gridState.GridSpacing)

	// Initialize new grid levels (without lock since we already hold it)
	at.initializeGridLevelsLocked(currentPrice, gridConfig)

	// CRITICAL FIX: Restore filled positions - find closest new level for each filled position
	for _, filledLevel := range filledPositions {
		closestIdx := -1
		closestDist := math.MaxFloat64

		for i, newLevel := range at.gridState.Levels {
			dist := math.Abs(newLevel.Price - filledLevel.PositionEntry)
			if dist < closestDist {
				closestDist = dist
				closestIdx = i
			}
		}

		if closestIdx >= 0 {
			// Restore the filled state to the closest level
			at.gridState.Levels[closestIdx].State = "filled"
			at.gridState.Levels[closestIdx].PositionEntry = filledLevel.PositionEntry
			at.gridState.Levels[closestIdx].PositionSize = filledLevel.PositionSize
			at.gridState.Levels[closestIdx].UnrealizedPnL = filledLevel.UnrealizedPnL
			at.gridState.Levels[closestIdx].OrderID = filledLevel.OrderID
			at.gridState.Levels[closestIdx].OrderQuantity = filledLevel.OrderQuantity
			logger.Infof("[Grid] Restored filled position at level %d (entry $%.2f)", closestIdx, filledLevel.PositionEntry)
		}
	}
}

// calculateDefaultBoundsLocked calculates default bounds (caller must hold lock)
func (at *AutoTrader) calculateDefaultBoundsLocked(price float64, config *store.GridStrategyConfig) {
	// Default: +/-3% from current price, scaled by grid count
	multiplier := 0.03 * float64(config.GridCount) / 10
	at.gridState.UpperPrice = price * (1 + multiplier)
	at.gridState.LowerPrice = price * (1 - multiplier)
}

// calculateATRBoundsLocked calculates bounds using ATR (caller must hold lock)
func (at *AutoTrader) calculateATRBoundsLocked(price float64, mktData *market.Data, config *store.GridStrategyConfig) {
	atr := 0.0
	if mktData.LongerTermContext != nil {
		atr = mktData.LongerTermContext.ATR14
	}

	if atr <= 0 {
		at.calculateDefaultBoundsLocked(price, config)
		return
	}

	multiplier := config.ATRMultiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}

	halfRange := atr * multiplier
	at.gridState.UpperPrice = price + halfRange
	at.gridState.LowerPrice = price - halfRange
}

// initializeGridLevelsLocked creates the grid level structure (caller must hold lock)
func (at *AutoTrader) initializeGridLevelsLocked(currentPrice float64, config *store.GridStrategyConfig) {
	levels := make([]kernel.GridLevelInfo, config.GridCount)
	totalWeight := 0.0
	weights := make([]float64, config.GridCount)

	// Calculate weights based on distribution
	for i := 0; i < config.GridCount; i++ {
		switch config.Distribution {
		case "gaussian":
			// Gaussian distribution - more weight in the middle
			center := float64(config.GridCount-1) / 2
			sigma := float64(config.GridCount) / 4
			weights[i] = math.Exp(-math.Pow(float64(i)-center, 2) / (2 * sigma * sigma))
		case "pyramid":
			// Pyramid - more weight at bottom
			weights[i] = float64(config.GridCount - i)
		default: // uniform
			weights[i] = 1.0
		}
		totalWeight += weights[i]
	}

	// Create levels
	for i := 0; i < config.GridCount; i++ {
		price := at.gridState.LowerPrice + float64(i)*at.gridState.GridSpacing
		allocatedUSD := config.TotalInvestment * weights[i] / totalWeight

		// Determine initial side (below current price = buy, above = sell)
		side := "buy"
		if price > currentPrice {
			side = "sell"
		}

		levels[i] = kernel.GridLevelInfo{
			Index:        i,
			Price:        price,
			State:        "empty",
			Side:         side,
			AllocatedUSD: allocatedUSD,
		}
	}

	at.gridState.Levels = levels

	// Apply direction-based side assignment if enabled (note: caller holds lock)
	if config.EnableDirectionAdjust {
		at.applyGridDirectionLocked(currentPrice)
	}
}

// applyGridDirectionLocked adjusts grid level sides based on the current direction (caller must hold lock)
func (at *AutoTrader) applyGridDirectionLocked(currentPrice float64) {
	config := at.gridState.Config
	direction := at.gridState.CurrentDirection

	// Get bias ratio from config, default to 0.7 (70%/30%)
	biasRatio := config.DirectionBiasRatio
	if biasRatio <= 0 || biasRatio > 1 {
		biasRatio = 0.7
	}

	buyRatio, _ := direction.GetBuySellRatio(biasRatio)

	// For neutral: use price-based assignment (buy below, sell above)
	if direction == market.GridDirectionNeutral {
		for i := range at.gridState.Levels {
			if at.gridState.Levels[i].Price <= currentPrice {
				at.gridState.Levels[i].Side = "buy"
			} else {
				at.gridState.Levels[i].Side = "sell"
			}
		}
		return
	}

	totalLevels := len(at.gridState.Levels)
	targetBuyLevels := int(float64(totalLevels) * buyRatio)

	switch direction {
	case market.GridDirectionLong:
		for i := range at.gridState.Levels {
			at.gridState.Levels[i].Side = "buy"
		}

	case market.GridDirectionShort:
		for i := range at.gridState.Levels {
			at.gridState.Levels[i].Side = "sell"
		}

	case market.GridDirectionLongBias, market.GridDirectionShortBias:
		buyCount := 0
		sellCount := 0

		for i := range at.gridState.Levels {
			needMoreBuys := buyCount < targetBuyLevels
			needMoreSells := sellCount < (totalLevels - targetBuyLevels)

			if at.gridState.Levels[i].Price <= currentPrice {
				if needMoreBuys {
					at.gridState.Levels[i].Side = "buy"
					buyCount++
				} else {
					at.gridState.Levels[i].Side = "sell"
					sellCount++
				}
			} else {
				if needMoreSells && direction == market.GridDirectionShortBias {
					at.gridState.Levels[i].Side = "sell"
					sellCount++
				} else if needMoreBuys && direction == market.GridDirectionLongBias {
					at.gridState.Levels[i].Side = "buy"
					buyCount++
				} else if needMoreSells {
					at.gridState.Levels[i].Side = "sell"
					sellCount++
				} else {
					at.gridState.Levels[i].Side = "buy"
					buyCount++
				}
			}
		}
	}
}
