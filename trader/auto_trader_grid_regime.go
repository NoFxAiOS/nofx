package trader

import (
	"fmt"
	"math"
	"nofx/logger"
	"nofx/market"
	"time"
)

// ============================================================================
// Regime Detection and Strategy Switching
// ============================================================================

// checkBoxBreakout checks for multi-period box breakouts and takes appropriate action
func (at *AutoTrader) checkBoxBreakout() error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	// Get box data
	box, err := market.GetBoxData(gridConfig.Symbol)
	if err != nil {
		logger.Infof("Failed to get box data: %v", err)
		return nil // Non-fatal, continue with other checks
	}

	// Update grid state with box values
	at.gridState.mu.Lock()
	at.gridState.ShortBoxUpper = box.ShortUpper
	at.gridState.ShortBoxLower = box.ShortLower
	at.gridState.MidBoxUpper = box.MidUpper
	at.gridState.MidBoxLower = box.MidLower
	at.gridState.LongBoxUpper = box.LongUpper
	at.gridState.LongBoxLower = box.LongLower
	at.gridState.mu.Unlock()

	// Detect breakout
	breakoutLevel, direction := detectBoxBreakout(box)

	// Get current breakout state
	state := &BreakoutState{
		Level:        market.BreakoutLevel(at.gridState.BreakoutLevel),
		Direction:    at.gridState.BreakoutDirection,
		ConfirmCount: at.gridState.BreakoutConfirmCount,
	}

	// Check if breakout is confirmed (3 candles)
	confirmed := confirmBreakout(state, breakoutLevel, direction)

	// Update grid state
	at.gridState.mu.Lock()
	at.gridState.BreakoutLevel = string(state.Level)
	at.gridState.BreakoutDirection = state.Direction
	at.gridState.BreakoutConfirmCount = state.ConfirmCount
	at.gridState.mu.Unlock()

	if !confirmed {
		return nil
	}

	// Take action based on breakout level
	// Use direction-aware action if enabled
	enableDirectionAdjust := gridConfig.EnableDirectionAdjust
	action := getBreakoutActionWithDirection(breakoutLevel, enableDirectionAdjust)

	// If direction adjustment action, determine the new direction
	if action == BreakoutActionAdjustDirection {
		box, _ := market.GetBoxData(gridConfig.Symbol)
		newDirection := determineGridDirection(box, at.gridState.CurrentDirection, breakoutLevel, direction)
		return at.executeDirectionAdjustment(newDirection)
	}

	return at.executeBreakoutAction(action)
}

// executeBreakoutAction executes the appropriate action for a breakout
func (at *AutoTrader) executeBreakoutAction(action BreakoutAction) error {
	switch action {
	case BreakoutActionReducePosition:
		// Short box breakout: reduce position to 50%
		logger.Infof("Short box breakout confirmed, reducing position to 50%%")
		at.gridState.mu.Lock()
		at.gridState.PositionReductionPct = 50
		at.gridState.mu.Unlock()
		return nil

	case BreakoutActionPauseGrid:
		// Mid box breakout: pause grid + cancel orders
		logger.Infof("Mid box breakout confirmed, pausing grid and canceling orders")
		at.gridState.mu.Lock()
		at.gridState.IsPaused = true
		at.gridState.mu.Unlock()
		return at.cancelAllGridOrders()

	case BreakoutActionCloseAll:
		// Long box breakout: pause + cancel + close all
		logger.Infof("Long box breakout confirmed, closing all positions")
		at.gridState.mu.Lock()
		at.gridState.IsPaused = true
		at.gridState.mu.Unlock()
		if err := at.cancelAllGridOrders(); err != nil {
			logger.Infof("Failed to cancel orders: %v", err)
		}
		return at.closeAllPositions()

	case BreakoutActionAdjustDirection:
		// Direction adjustment is handled separately via executeDirectionAdjustment
		// This case should not be reached, but handle gracefully
		logger.Infof("Direction adjustment action received via executeBreakoutAction")
		return nil
	}

	return nil
}

// executeDirectionAdjustment handles grid direction changes based on box breakout
func (at *AutoTrader) executeDirectionAdjustment(newDirection market.GridDirection) error {
	at.gridState.mu.RLock()
	oldDirection := at.gridState.CurrentDirection
	at.gridState.mu.RUnlock()

	if oldDirection == newDirection {
		return nil // No change needed
	}

	logger.Infof("[Grid] Direction adjustment: %s -> %s", oldDirection, newDirection)

	// Cancel existing orders before adjusting
	if err := at.cancelAllGridOrders(); err != nil {
		logger.Warnf("[Grid] Failed to cancel orders during direction adjustment: %v", err)
	}

	// Apply the new direction
	return at.adjustGridDirection(newDirection)
}

// adjustGridDirection handles runtime direction adjustment when breakout is detected
func (at *AutoTrader) adjustGridDirection(newDirection market.GridDirection) error {
	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	oldDirection := at.gridState.CurrentDirection
	if oldDirection == newDirection {
		return nil // No change needed
	}

	at.gridState.CurrentDirection = newDirection
	at.gridState.DirectionChangedAt = time.Now()
	at.gridState.DirectionChangeCount++

	logger.Infof("[Grid] Direction changed: %s -> %s (change count: %d)",
		oldDirection, newDirection, at.gridState.DirectionChangeCount)

	// Get current price for recalculation
	currentPrice, err := at.trader.GetMarketPrice(at.gridState.Config.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	// Reapply direction to grid levels
	at.applyGridDirection(currentPrice)

	return nil
}

// checkFalseBreakoutRecovery checks if price has returned to box after breakout
func (at *AutoTrader) checkFalseBreakoutRecovery() error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	at.gridState.mu.RLock()
	breakoutLevel := at.gridState.BreakoutLevel
	isPaused := at.gridState.IsPaused
	positionReduction := at.gridState.PositionReductionPct
	currentDirection := at.gridState.CurrentDirection
	at.gridState.mu.RUnlock()

	// Only check if we had a breakout or non-neutral direction
	needsRecoveryCheck := breakoutLevel != string(market.BreakoutNone) ||
		positionReduction != 0 ||
		isPaused ||
		(gridConfig.EnableDirectionAdjust && currentDirection != market.GridDirectionNeutral)

	if !needsRecoveryCheck {
		return nil
	}

	// Get current box data
	box, err := market.GetBoxData(gridConfig.Symbol)
	if err != nil {
		return nil
	}

	// Check if price is back inside the long box
	if box.CurrentPrice >= box.LongLower && box.CurrentPrice <= box.LongUpper {
		logger.Infof("Price returned to box, recovering with 50%% position")

		at.gridState.mu.Lock()
		at.gridState.BreakoutLevel = string(market.BreakoutNone)
		at.gridState.BreakoutDirection = ""
		at.gridState.BreakoutConfirmCount = 0
		at.gridState.PositionReductionPct = 50 // Recover at 50%
		at.gridState.IsPaused = false
		at.gridState.mu.Unlock()
	}

	// Check for direction recovery toward neutral (if direction adjustment is enabled)
	if gridConfig.EnableDirectionAdjust && currentDirection != market.GridDirectionNeutral {
		if shouldRecoverDirection(box, currentDirection) {
			newDirection := determineRecoveryDirection(box.CurrentPrice, box, currentDirection)
			if newDirection != currentDirection {
				logger.Infof("[Grid] Direction recovery: %s -> %s (price back in short box)",
					currentDirection, newDirection)
				at.adjustGridDirection(newDirection)
			}
		}
	}

	return nil
}

// GetGridRiskInfo returns current risk information for frontend display
func (at *AutoTrader) GetGridRiskInfo() *GridRiskInfo {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return &GridRiskInfo{}
	}

	at.gridState.mu.RLock()
	defer at.gridState.mu.RUnlock()

	// Get current price
	currentPrice, _ := at.trader.GetMarketPrice(gridConfig.Symbol)

	// Calculate effective leverage
	totalInvestment := gridConfig.TotalInvestment
	leverage := gridConfig.Leverage

	// Get current position value
	positions, _ := at.trader.GetPositions()
	var currentPositionValue float64
	var currentPositionSize float64
	for _, pos := range positions {
		if sym, _ := pos["symbol"].(string); sym == gridConfig.Symbol {
			size, _ := pos["positionAmt"].(float64)
			entry, _ := pos["entryPrice"].(float64)
			currentPositionValue = math.Abs(size * entry)
			currentPositionSize = size
			break
		}
	}

	effectiveLeverage := 0.0
	if totalInvestment > 0 {
		effectiveLeverage = currentPositionValue / totalInvestment
	}

	// Calculate max position based on regime
	regimeLevel := market.RegimeLevel(at.gridState.CurrentRegimeLevel)
	if regimeLevel == "" {
		regimeLevel = market.RegimeLevelStandard
	}

	// Use default position limit since GridStrategyConfig doesn't have regime-specific limits
	// Default is 70% for standard regime
	maxPositionPct := 70.0
	switch regimeLevel {
	case market.RegimeLevelNarrow:
		maxPositionPct = 40.0
	case market.RegimeLevelStandard:
		maxPositionPct = 70.0
	case market.RegimeLevelWide:
		maxPositionPct = 60.0
	case market.RegimeLevelVolatile:
		maxPositionPct = 40.0
	}

	maxPosition := totalInvestment * maxPositionPct / 100 * float64(leverage)

	// Use default leverage limits since GridStrategyConfig doesn't have regime-specific limits
	recommendedLeverage := leverage
	switch regimeLevel {
	case market.RegimeLevelNarrow:
		recommendedLeverage = min(leverage, 2)
	case market.RegimeLevelStandard:
		recommendedLeverage = min(leverage, 4)
	case market.RegimeLevelWide:
		recommendedLeverage = min(leverage, 3)
	case market.RegimeLevelVolatile:
		recommendedLeverage = min(leverage, 2)
	}

	// Calculate liquidation distance and price only when there's a position
	var liquidationDistance float64
	var liquidationPrice float64
	if currentPositionSize != 0 && currentPrice > 0 {
		liquidationDistance = 100.0 / float64(leverage) * 0.9 // ~90% of theoretical max
		if currentPositionSize > 0 {
			// Long position: liquidation below entry
			liquidationPrice = currentPrice * (1 - liquidationDistance/100)
		} else {
			// Short position: liquidation above entry
			liquidationPrice = currentPrice * (1 + liquidationDistance/100)
		}
	}

	positionPercent := 0.0
	if maxPosition > 0 {
		positionPercent = currentPositionValue / maxPosition * 100
	}

	return &GridRiskInfo{
		CurrentLeverage:     leverage,
		EffectiveLeverage:   effectiveLeverage,
		RecommendedLeverage: recommendedLeverage,

		CurrentPosition: currentPositionValue,
		MaxPosition:     maxPosition,
		PositionPercent: positionPercent,

		LiquidationPrice:    liquidationPrice,
		LiquidationDistance: liquidationDistance,

		RegimeLevel: string(regimeLevel),

		ShortBoxUpper: at.gridState.ShortBoxUpper,
		ShortBoxLower: at.gridState.ShortBoxLower,
		MidBoxUpper:   at.gridState.MidBoxUpper,
		MidBoxLower:   at.gridState.MidBoxLower,
		LongBoxUpper:  at.gridState.LongBoxUpper,
		LongBoxLower:  at.gridState.LongBoxLower,
		CurrentPrice:  currentPrice,

		BreakoutLevel:     at.gridState.BreakoutLevel,
		BreakoutDirection: at.gridState.BreakoutDirection,

		CurrentGridDirection:  string(at.gridState.CurrentDirection),
		DirectionChangeCount:  at.gridState.DirectionChangeCount,
		EnableDirectionAdjust: gridConfig.EnableDirectionAdjust,
	}
}
