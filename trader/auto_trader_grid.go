package trader

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"sync"
	"time"
)

// ============================================================================
// Grid Trading State Management
// ============================================================================

// GridState holds the runtime state for grid trading
type GridState struct {
	mu sync.RWMutex

	// Configuration
	Config *store.GridStrategyConfig

	// Grid levels
	Levels []kernel.GridLevelInfo

	// Calculated bounds
	UpperPrice  float64
	LowerPrice  float64
	GridSpacing float64

	// State flags
	IsPaused    bool
	IsInitialized bool

	// Performance tracking
	TotalProfit   float64
	TotalTrades   int
	WinningTrades int
	MaxDrawdown   float64
	PeakEquity    float64
	DailyPnL      float64
	LastDailyReset time.Time

	// Order tracking
	OrderBook map[string]int // OrderID -> LevelIndex

	// Box state
	ShortBoxUpper float64
	ShortBoxLower float64
	MidBoxUpper   float64
	MidBoxLower   float64
	LongBoxUpper  float64
	LongBoxLower  float64

	// Breakout state
	BreakoutLevel        string
	BreakoutDirection    string
	BreakoutConfirmCount int

	// Position reduction (0 = normal, 50 = reduced after false breakout)
	PositionReductionPct float64

	// Current regime level
	CurrentRegimeLevel string

	// Grid direction adjustment
	CurrentDirection       market.GridDirection
	DirectionChangedAt     time.Time
	DirectionChangeCount   int
}

// NewGridState creates a new grid state
func NewGridState(config *store.GridStrategyConfig) *GridState {
	return &GridState{
		Config:           config,
		Levels:           make([]kernel.GridLevelInfo, 0),
		OrderBook:        make(map[string]int),
		CurrentDirection: market.GridDirectionNeutral,
	}
}

// ============================================================================
// Breakout Detection
// ============================================================================

// BreakoutType represents the type of price breakout
type BreakoutType string

const (
	BreakoutNone  BreakoutType = "none"
	BreakoutUpper BreakoutType = "upper"
	BreakoutLower BreakoutType = "lower"
)

// checkBreakout detects if price has broken out of grid range
// Returns breakout type and percentage beyond boundary
func (at *AutoTrader) checkBreakout() (BreakoutType, float64) {
	gridConfig := at.config.StrategyConfig.GridConfig

	currentPrice, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		return BreakoutNone, 0
	}

	at.gridState.mu.RLock()
	upper := at.gridState.UpperPrice
	lower := at.gridState.LowerPrice
	at.gridState.mu.RUnlock()

	if upper <= 0 || lower <= 0 {
		return BreakoutNone, 0
	}

	// Check upper breakout
	if currentPrice > upper {
		breakoutPct := (currentPrice - upper) / upper * 100
		return BreakoutUpper, breakoutPct
	}

	// Check lower breakout
	if currentPrice < lower {
		breakoutPct := (lower - currentPrice) / lower * 100
		return BreakoutLower, breakoutPct
	}

	return BreakoutNone, 0
}

// checkMaxDrawdown checks if current drawdown exceeds maximum allowed
// Returns: (exceeded bool, currentDrawdown float64)
func (at *AutoTrader) checkMaxDrawdown() (bool, float64) {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig.MaxDrawdownPct <= 0 {
		return false, 0
	}

	// Get current equity
	balance, err := at.trader.GetBalance()
	if err != nil {
		return false, 0
	}

	currentEquity := 0.0
	if equity, ok := balance["total_equity"].(float64); ok {
		currentEquity = equity
	} else if total, ok := balance["totalWalletBalance"].(float64); ok {
		if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
			currentEquity = total + unrealized
		}
	}

	if currentEquity <= 0 {
		return false, 0
	}

	// Update peak equity
	at.gridState.mu.Lock()
	if currentEquity > at.gridState.PeakEquity {
		at.gridState.PeakEquity = currentEquity
	}
	peakEquity := at.gridState.PeakEquity
	at.gridState.mu.Unlock()

	if peakEquity <= 0 {
		return false, 0
	}

	// Calculate current drawdown
	drawdown := (peakEquity - currentEquity) / peakEquity * 100

	// Update max drawdown tracking
	at.gridState.mu.Lock()
	if drawdown > at.gridState.MaxDrawdown {
		at.gridState.MaxDrawdown = drawdown
	}
	at.gridState.mu.Unlock()

	return drawdown >= gridConfig.MaxDrawdownPct, drawdown
}

// checkDailyLossLimit checks if daily loss exceeds limit
// Returns: (exceeded bool, dailyLossPct float64)
func (at *AutoTrader) checkDailyLossLimit() (bool, float64) {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig.DailyLossLimitPct <= 0 {
		return false, 0
	}

	at.gridState.mu.Lock()
	// Reset daily PnL if new day
	now := time.Now()
	if now.YearDay() != at.gridState.LastDailyReset.YearDay() ||
		now.Year() != at.gridState.LastDailyReset.Year() {
		at.gridState.DailyPnL = 0
		at.gridState.LastDailyReset = now
	}
	dailyPnL := at.gridState.DailyPnL
	at.gridState.mu.Unlock()

	// Calculate daily loss as percentage of total investment
	dailyLossPct := 0.0
	if gridConfig.TotalInvestment > 0 && dailyPnL < 0 {
		dailyLossPct = (-dailyPnL) / gridConfig.TotalInvestment * 100
	}

	return dailyLossPct >= gridConfig.DailyLossLimitPct, dailyLossPct
}

// updateDailyPnL updates the daily PnL tracking
func (at *AutoTrader) updateDailyPnL(realizedPnL float64) {
	at.gridState.mu.Lock()
	at.gridState.DailyPnL += realizedPnL
	at.gridState.TotalProfit += realizedPnL
	at.gridState.mu.Unlock()
}

// emergencyExit closes all positions and cancels all orders
func (at *AutoTrader) emergencyExit(reason string) error {
	gridConfig := at.config.StrategyConfig.GridConfig

	logger.Errorf("[Grid] EMERGENCY EXIT: %s", reason)

	// Cancel all orders
	if err := at.cancelAllGridOrders(); err != nil {
		logger.Errorf("[Grid] Failed to cancel orders in emergency: %v", err)
	}

	// Close all positions
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); ok && sym == gridConfig.Symbol {
				if size, ok := pos["positionAmt"].(float64); ok && size != 0 {
					if size > 0 {
						at.trader.CloseLong(gridConfig.Symbol, size)
					} else {
						at.trader.CloseShort(gridConfig.Symbol, -size)
					}
				}
			}
		}
	}

	// Pause grid
	at.gridState.mu.Lock()
	at.gridState.IsPaused = true
	at.gridState.mu.Unlock()

	return nil
}

// handleBreakout handles price breakout from grid range
func (at *AutoTrader) handleBreakout(breakoutType BreakoutType, breakoutPct float64) error {
	logger.Warnf("[Grid] BREAKOUT DETECTED: %s, %.2f%% beyond boundary", breakoutType, breakoutPct)

	// If breakout exceeds 2%, pause grid and cancel orders
	if breakoutPct >= 2.0 {
		logger.Warnf("[Grid] Significant breakout (%.2f%%), pausing grid and canceling orders", breakoutPct)

		// Cancel all pending orders to prevent further losses
		if err := at.cancelAllGridOrders(); err != nil {
			logger.Errorf("[Grid] Failed to cancel orders on breakout: %v", err)
		}

		// Pause grid trading
		at.gridState.mu.Lock()
		at.gridState.IsPaused = true
		at.gridState.mu.Unlock()

		return fmt.Errorf("grid paused due to %s breakout (%.2f%%)", breakoutType, breakoutPct)
	}

	// If breakout is minor (< 2%), consider adjusting grid
	if breakoutPct >= 1.0 {
		logger.Infof("[Grid] Minor breakout (%.2f%%), considering grid adjustment", breakoutPct)
		// Let AI decide whether to adjust
	}

	return nil
}

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

	logger.Infof("[Grid] Direction adjustment: %s → %s", oldDirection, newDirection)

	// Cancel existing orders before adjusting
	if err := at.cancelAllGridOrders(); err != nil {
		logger.Warnf("[Grid] Failed to cancel orders during direction adjustment: %v", err)
	}

	// Apply the new direction
	return at.adjustGridDirection(newDirection)
}

// closeAllPositions closes all open positions for the grid symbol
func (at *AutoTrader) closeAllPositions() error {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig == nil {
		return nil
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		if symbol != gridConfig.Symbol {
			continue
		}

		size, _ := pos["positionAmt"].(float64)
		if size == 0 {
			continue
		}

		if size > 0 {
			_, err = at.trader.CloseLong(symbol, size)
		} else {
			_, err = at.trader.CloseShort(symbol, -size)
		}
		if err != nil {
			logger.Infof("Failed to close position: %v", err)
		}
	}

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
				logger.Infof("[Grid] Direction recovery: %s → %s (price back in short box)",
					currentDirection, newDirection)
				at.adjustGridDirection(newDirection)
			}
		}
	}

	return nil
}

// ============================================================================
// AutoTrader Grid Methods
// ============================================================================

// InitializeGrid initializes the grid state and calculates levels
func (at *AutoTrader) InitializeGrid() error {
	if at.config.StrategyConfig == nil || at.config.StrategyConfig.GridConfig == nil {
		return fmt.Errorf("grid configuration not found")
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	at.gridState = NewGridState(gridConfig)

	// Get current market price
	price, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	// Calculate grid bounds
	if gridConfig.UseATRBounds {
		// Get ATR for bound calculation
		mktData, err := market.GetWithTimeframes(gridConfig.Symbol, []string{"4h"}, "4h", 20)
		if err != nil {
			logger.Warnf("Failed to get market data for ATR: %v, using default bounds", err)
			at.calculateDefaultBounds(price, gridConfig)
		} else {
			at.calculateATRBounds(price, mktData, gridConfig)
		}
	} else {
		// Use manual bounds
		at.gridState.UpperPrice = gridConfig.UpperPrice
		at.gridState.LowerPrice = gridConfig.LowerPrice
	}

	// Calculate grid spacing
	at.gridState.GridSpacing = (at.gridState.UpperPrice - at.gridState.LowerPrice) / float64(gridConfig.GridCount-1)

	// Initialize grid levels
	at.initializeGridLevels(price, gridConfig)

	at.gridState.IsInitialized = true

	// CRITICAL: Set leverage on exchange before trading
	if err := at.trader.SetLeverage(gridConfig.Symbol, gridConfig.Leverage); err != nil {
		logger.Warnf("[Grid] Failed to set leverage %dx on exchange: %v", gridConfig.Leverage, err)
		// Not fatal - continue with default leverage
	} else {
		logger.Infof("[Grid] Leverage set to %dx for %s", gridConfig.Leverage, gridConfig.Symbol)
	}

	logger.Infof("📊 [Grid] Initialized: %d levels, $%.2f - $%.2f, spacing $%.2f",
		gridConfig.GridCount, at.gridState.LowerPrice, at.gridState.UpperPrice, at.gridState.GridSpacing)

	return nil
}

// calculateDefaultBounds calculates default bounds based on price
func (at *AutoTrader) calculateDefaultBounds(price float64, config *store.GridStrategyConfig) {
	// Default: ±3% from current price
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

	logger.Infof("[Grid] Direction changed: %s → %s (change count: %d)",
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

// RunGridCycle executes one grid trading cycle
func (at *AutoTrader) RunGridCycle() error {
	// Check if trader is stopped (early exit to prevent trades after Stop() is called)
	at.isRunningMutex.RLock()
	running := at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("[Grid] Trader is stopped, aborting grid cycle")
		return nil
	}

	if at.gridState == nil || !at.gridState.IsInitialized {
		if err := at.InitializeGrid(); err != nil {
			return fmt.Errorf("failed to initialize grid: %w", err)
		}
	}

	// CRITICAL: Check for breakout before executing any trades
	breakoutType, breakoutPct := at.checkBreakout()
	if breakoutType != BreakoutNone {
		if err := at.handleBreakout(breakoutType, breakoutPct); err != nil {
			return err // Grid paused due to breakout
		}
	}

	// CRITICAL: Check max drawdown
	exceeded, drawdown := at.checkMaxDrawdown()
	if exceeded {
		return at.emergencyExit(fmt.Sprintf("max drawdown exceeded: %.2f%%", drawdown))
	}

	// CRITICAL: Check daily loss limit
	dailyExceeded, dailyLossPct := at.checkDailyLossLimit()
	if dailyExceeded {
		logger.Errorf("[Grid] Daily loss limit exceeded: %.2f%%", dailyLossPct)
		at.gridState.mu.Lock()
		at.gridState.IsPaused = true
		at.gridState.mu.Unlock()
		return fmt.Errorf("daily loss limit exceeded: %.2f%%", dailyLossPct)
	}

	// Check multi-period box breakout
	if err := at.checkBoxBreakout(); err != nil {
		logger.Infof("Box breakout check error: %v", err)
	}

	// Check for false breakout recovery
	if err := at.checkFalseBreakoutRecovery(); err != nil {
		logger.Infof("False breakout recovery check error: %v", err)
	}

	// Check if grid is paused
	at.gridState.mu.RLock()
	isPaused := at.gridState.IsPaused
	at.gridState.mu.RUnlock()
	if isPaused {
		logger.Infof("[Grid] Grid is paused, skipping cycle")
		return nil
	}

	gridConfig := at.config.StrategyConfig.GridConfig
	lang := at.config.StrategyConfig.Language
	if lang == "" {
		lang = "en"
	}

	// Build grid context
	gridCtx, err := at.buildGridContext()
	if err != nil {
		return fmt.Errorf("failed to build grid context: %w", err)
	}

	// Get AI decisions
	decision, err := kernel.GetGridDecisions(gridCtx, at.mcpClient, gridConfig, lang)
	if err != nil {
		return fmt.Errorf("failed to get grid decisions: %w", err)
	}

	// Check if trader is stopped before executing any decisions (prevent trades after Stop())
	at.isRunningMutex.RLock()
	running = at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("[Grid] Trader stopped before decision execution, aborting grid cycle")
		return nil
	}

	// Execute decisions
	for _, d := range decision.Decisions {
		// Check if trader is still running before each decision
		at.isRunningMutex.RLock()
		running := at.isRunning
		at.isRunningMutex.RUnlock()
		if !running {
			logger.Infof("[Grid] Trader stopped, skipping remaining %d decisions", len(decision.Decisions))
			break
		}

		if err := at.executeGridDecision(&d); err != nil {
			logger.Warnf("[Grid] Failed to execute decision %s: %v", d.Action, err)
		}
	}

	// Sync state with exchange
	at.syncGridState()

	// Save decision record
	at.saveGridDecisionRecord(decision)

	return nil
}

// buildGridContext builds the context for AI grid decisions
func (at *AutoTrader) buildGridContext() (*kernel.GridContext, error) {
	gridConfig := at.config.StrategyConfig.GridConfig

	// Get market data
	mktData, err := market.GetWithTimeframes(gridConfig.Symbol, []string{"5m", "4h"}, "5m", 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get market data: %w", err)
	}

	// Build base context from market data
	ctx := kernel.BuildGridContextFromMarketData(mktData, gridConfig)

	// Add grid state
	at.gridState.mu.RLock()
	ctx.Levels = at.gridState.Levels
	ctx.UpperPrice = at.gridState.UpperPrice
	ctx.LowerPrice = at.gridState.LowerPrice
	ctx.GridSpacing = at.gridState.GridSpacing
	ctx.IsPaused = at.gridState.IsPaused
	ctx.TotalProfit = at.gridState.TotalProfit
	ctx.TotalTrades = at.gridState.TotalTrades
	ctx.WinningTrades = at.gridState.WinningTrades
	ctx.MaxDrawdown = at.gridState.MaxDrawdown
	ctx.DailyPnL = at.gridState.DailyPnL

	// Count active orders and filled levels
	for _, level := range at.gridState.Levels {
		if level.State == "pending" {
			ctx.ActiveOrderCount++
		} else if level.State == "filled" {
			ctx.FilledLevelCount++
		}
	}
	at.gridState.mu.RUnlock()

	// Get account info
	balance, err := at.trader.GetBalance()
	if err == nil {
		if equity, ok := balance["total_equity"].(float64); ok {
			ctx.TotalEquity = equity
		}
		if available, ok := balance["availableBalance"].(float64); ok {
			ctx.AvailableBalance = available
		}
		if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
			ctx.UnrealizedPnL = unrealized
		}
	}

	// Get current position
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); ok && sym == gridConfig.Symbol {
				if size, ok := pos["positionAmt"].(float64); ok {
					ctx.CurrentPosition = size
				}
			}
		}
	}

	return ctx, nil
}

// executeGridDecision executes a single grid decision
func (at *AutoTrader) executeGridDecision(d *kernel.Decision) error {
	switch d.Action {
	case "place_buy_limit":
		return at.placeGridLimitOrder(d, "BUY")
	case "place_sell_limit":
		return at.placeGridLimitOrder(d, "SELL")
	case "cancel_order":
		return at.cancelGridOrder(d)
	case "cancel_all_orders":
		return at.cancelAllGridOrders()
	case "pause_grid":
		return at.pauseGrid(d.Reasoning)
	case "resume_grid":
		return at.resumeGrid()
	case "adjust_grid":
		return at.adjustGrid(d)
	case "hold":
		logger.Infof("[Grid] Holding current state: %s", d.Reasoning)
		return nil
	// Support standard actions for closing positions
	case "close_long":
		_, err := at.trader.CloseLong(d.Symbol, d.Quantity)
		return err
	case "close_short":
		_, err := at.trader.CloseShort(d.Symbol, d.Quantity)
		return err
	default:
		logger.Warnf("[Grid] Unknown action: %s", d.Action)
		return nil
	}
}

// checkTotalPositionLimit checks if adding a new position would exceed total limits
// Returns: (allowed bool, currentPositionValue float64, maxAllowed float64)
func (at *AutoTrader) checkTotalPositionLimit(symbol string, additionalValue float64) (bool, float64, float64) {
	gridConfig := at.config.StrategyConfig.GridConfig

	// Calculate max allowed total position value
	// Total position should not exceed: TotalInvestment × Leverage
	maxTotalPositionValue := gridConfig.TotalInvestment * float64(gridConfig.Leverage)

	// Get current position value from exchange
	currentPositionValue := 0.0
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); ok && sym == symbol {
				if size, ok := pos["positionAmt"].(float64); ok {
					if price, ok := pos["markPrice"].(float64); ok {
						currentPositionValue = math.Abs(size) * price
					} else if entryPrice, ok := pos["entryPrice"].(float64); ok {
						currentPositionValue = math.Abs(size) * entryPrice
					}
				}
			}
		}
	}

	// Also count pending orders as potential position
	at.gridState.mu.RLock()
	pendingValue := 0.0
	for _, level := range at.gridState.Levels {
		if level.State == "pending" {
			pendingValue += level.OrderQuantity * level.Price
		}
	}
	at.gridState.mu.RUnlock()

	totalAfterOrder := currentPositionValue + pendingValue + additionalValue
	allowed := totalAfterOrder <= maxTotalPositionValue

	return allowed, currentPositionValue + pendingValue, maxTotalPositionValue
}

// placeGridLimitOrder places a limit order for grid trading
func (at *AutoTrader) placeGridLimitOrder(d *kernel.Decision, side string) error {
	// Check if trader supports GridTrader interface
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		// Fallback to adapter
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	gridConfig := at.config.StrategyConfig.GridConfig

	// CRITICAL: Validate and cap quantity to prevent excessive position sizes
	// This protects against AI miscalculations or leverage misconfigurations
	quantity := d.Quantity
	if d.Price > 0 && gridConfig.TotalInvestment > 0 {
		// Calculate max allowed position value per grid level
		// Each level gets proportional share of total investment
		maxMarginPerLevel := gridConfig.TotalInvestment / float64(gridConfig.GridCount)
		maxPositionValuePerLevel := maxMarginPerLevel * float64(gridConfig.Leverage)
		maxQuantityPerLevel := maxPositionValuePerLevel / d.Price

		// Also get the level's allocated USD for additional validation
		at.gridState.mu.RLock()
		var levelAllocatedUSD float64
		if d.LevelIndex >= 0 && d.LevelIndex < len(at.gridState.Levels) {
			levelAllocatedUSD = at.gridState.Levels[d.LevelIndex].AllocatedUSD
		}
		at.gridState.mu.RUnlock()

		// Use level-specific allocation if available
		if levelAllocatedUSD > 0 {
			levelMaxPositionValue := levelAllocatedUSD * float64(gridConfig.Leverage)
			levelMaxQuantity := levelMaxPositionValue / d.Price
			if levelMaxQuantity < maxQuantityPerLevel {
				maxQuantityPerLevel = levelMaxQuantity
			}
		}

		// Cap quantity if it exceeds the maximum allowed
		if quantity > maxQuantityPerLevel {
			logger.Warnf("[Grid] ⚠️ Quantity %.4f exceeds max allowed %.4f (position_value $%.2f > max $%.2f), capping",
				quantity, maxQuantityPerLevel, quantity*d.Price, maxPositionValuePerLevel)
			quantity = maxQuantityPerLevel
		}

		// Safety check: ensure position value is reasonable (within 2x of intended max as absolute limit)
		positionValue := quantity * d.Price
		absoluteMaxValue := gridConfig.TotalInvestment * float64(gridConfig.Leverage) * 2 // 2x safety margin
		if positionValue > absoluteMaxValue {
			logger.Errorf("[Grid] CRITICAL: Position value $%.2f exceeds absolute max $%.2f! Rejecting order.",
				positionValue, absoluteMaxValue)
			return fmt.Errorf("position value $%.2f exceeds safety limit $%.2f", positionValue, absoluteMaxValue)
		}
	}

	// CRITICAL: Check total position limit before placing order
	orderValue := quantity * d.Price
	allowed, currentValue, maxValue := at.checkTotalPositionLimit(d.Symbol, orderValue)
	if !allowed {
		logger.Errorf("[Grid] TOTAL POSITION LIMIT EXCEEDED: current=$%.2f + order=$%.2f > max=$%.2f. Rejecting order.",
			currentValue, orderValue, maxValue)
		return fmt.Errorf("total position value $%.2f would exceed limit $%.2f", currentValue+orderValue, maxValue)
	}

	req := &LimitOrderRequest{
		Symbol:     d.Symbol,
		Side:       side,
		Price:      d.Price,
		Quantity:   quantity, // Use validated/capped quantity
		Leverage:   gridConfig.Leverage,
		PostOnly:   gridConfig.UseMakerOnly,
		ReduceOnly: false,
		ClientID:   fmt.Sprintf("grid-%d-%d", d.LevelIndex, time.Now().UnixNano()%1000000),
	}

	result, err := gridTrader.PlaceLimitOrder(req)
	if err != nil {
		return fmt.Errorf("failed to place limit order: %w", err)
	}

	// Update grid level state
	at.gridState.mu.Lock()
	if d.LevelIndex >= 0 && d.LevelIndex < len(at.gridState.Levels) {
		at.gridState.Levels[d.LevelIndex].State = "pending"
		at.gridState.Levels[d.LevelIndex].OrderID = result.OrderID
		at.gridState.Levels[d.LevelIndex].OrderQuantity = d.Quantity
		at.gridState.OrderBook[result.OrderID] = d.LevelIndex
	}
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Placed %s limit order at $%.2f, qty=%.4f, level=%d, orderID=%s",
		side, d.Price, d.Quantity, d.LevelIndex, result.OrderID)

	return nil
}

// cancelGridOrder cancels a specific grid order
func (at *AutoTrader) cancelGridOrder(d *kernel.Decision) error {
	gridTrader, ok := at.trader.(GridTrader)
	if !ok {
		gridTrader = NewGridTraderAdapter(at.trader)
	}

	if err := gridTrader.CancelOrder(d.Symbol, d.OrderID); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	// Update state
	at.gridState.mu.Lock()
	if levelIdx, ok := at.gridState.OrderBook[d.OrderID]; ok {
		if levelIdx >= 0 && levelIdx < len(at.gridState.Levels) {
			at.gridState.Levels[levelIdx].State = "empty"
			at.gridState.Levels[levelIdx].OrderID = ""
			at.gridState.Levels[levelIdx].OrderQuantity = 0
		}
		delete(at.gridState.OrderBook, d.OrderID)
	}
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Cancelled order: %s", d.OrderID)
	return nil
}

// cancelAllGridOrders cancels all grid orders
func (at *AutoTrader) cancelAllGridOrders() error {
	gridConfig := at.config.StrategyConfig.GridConfig

	if err := at.trader.CancelAllOrders(gridConfig.Symbol); err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}

	// Reset all pending levels
	at.gridState.mu.Lock()
	for i := range at.gridState.Levels {
		if at.gridState.Levels[i].State == "pending" {
			at.gridState.Levels[i].State = "empty"
			at.gridState.Levels[i].OrderID = ""
			at.gridState.Levels[i].OrderQuantity = 0
		}
	}
	at.gridState.OrderBook = make(map[string]int)
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Cancelled all orders")
	return nil
}

// pauseGrid pauses grid trading
func (at *AutoTrader) pauseGrid(reason string) error {
	at.cancelAllGridOrders()

	at.gridState.mu.Lock()
	at.gridState.IsPaused = true
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Paused: %s", reason)
	return nil
}

// resumeGrid resumes grid trading
func (at *AutoTrader) resumeGrid() error {
	at.gridState.mu.Lock()
	at.gridState.IsPaused = false
	at.gridState.mu.Unlock()

	logger.Infof("[Grid] Resumed")
	return nil
}

// adjustGrid adjusts grid parameters
func (at *AutoTrader) adjustGrid(d *kernel.Decision) error {
	// Cancel existing orders first
	at.cancelAllGridOrders()

	gridConfig := at.config.StrategyConfig.GridConfig

	// Get current price
	price, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get market price: %w", err)
	}

	// Reinitialize grid levels
	at.initializeGridLevels(price, gridConfig)

	logger.Infof("[Grid] Adjusted grid bounds around price $%.2f", price)
	return nil
}

// syncGridState syncs grid state with exchange
func (at *AutoTrader) syncGridState() {
	gridConfig := at.config.StrategyConfig.GridConfig

	// Get open orders from exchange
	openOrders, err := at.trader.GetOpenOrders(gridConfig.Symbol)
	if err != nil {
		logger.Warnf("[Grid] Failed to get open orders: %v", err)
		return
	}

	// Build set of active order IDs
	activeOrderIDs := make(map[string]bool)
	for _, order := range openOrders {
		activeOrderIDs[order.OrderID] = true
	}

	// Get current positions to verify fills
	positions, err := at.trader.GetPositions()
	currentPositionSize := 0.0
	if err != nil {
		logger.Warnf("[Grid] Failed to get positions for state sync: %v", err)
	} else {
		for _, pos := range positions {
			if sym, ok := pos["symbol"].(string); ok && sym == gridConfig.Symbol {
				if size, ok := pos["positionAmt"].(float64); ok {
					currentPositionSize = size
				}
			}
		}
	}

	// Update levels based on order status
	at.gridState.mu.Lock()
	expectedPositionSize := 0.0
	for _, level := range at.gridState.Levels {
		if level.State == "filled" {
			expectedPositionSize += level.PositionSize
		}
	}

	for i := range at.gridState.Levels {
		level := &at.gridState.Levels[i]
		if level.State == "pending" && level.OrderID != "" {
			if !activeOrderIDs[level.OrderID] {
				// Order no longer exists - check if position changed to determine fill vs cancel
				// This is a heuristic - ideally we'd query order history
				// If current position is larger than expected filled positions, this order was likely filled
				if math.Abs(currentPositionSize) > math.Abs(expectedPositionSize) {
					// Position increased, likely filled
					level.State = "filled"
					level.PositionEntry = level.Price
					level.PositionSize = level.OrderQuantity
					at.gridState.TotalTrades++
					logger.Infof("[Grid] Level %d order filled at $%.2f", i, level.Price)
				} else {
					// Position didn't increase as expected, likely cancelled
					level.State = "empty"
					level.OrderID = ""
					level.OrderQuantity = 0
					logger.Infof("[Grid] Level %d order cancelled/expired", i)
				}
				delete(at.gridState.OrderBook, level.OrderID)
			}
		}
	}
	at.gridState.mu.Unlock()

	logger.Debugf("[Grid] Synced state: position=%.4f, orders=%d", currentPositionSize, len(openOrders))

	// Check stop loss
	at.checkAndExecuteStopLoss()

	// Check grid skew
	at.autoAdjustGrid()
}

// saveGridDecisionRecord saves the grid decision to database
func (at *AutoTrader) saveGridDecisionRecord(decision *kernel.FullDecision) {
	if at.store == nil {
		return
	}

	at.cycleNumber++

	record := &store.DecisionRecord{
		TraderID:            at.id,
		CycleNumber:         at.cycleNumber,
		Timestamp:           time.Now().UTC(),
		SystemPrompt:        decision.SystemPrompt,
		InputPrompt:         decision.UserPrompt,
		CoTTrace:            decision.CoTTrace,
		RawResponse:         decision.RawResponse,
		AIRequestDurationMs: decision.AIRequestDurationMs,
		Success:             true,
	}

	if len(decision.Decisions) > 0 {
		decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
		record.DecisionJSON = string(decisionJSON)

		// Convert kernel.Decision to store.DecisionAction for frontend display
		for _, d := range decision.Decisions {
			actionRecord := store.DecisionAction{
				Action:     d.Action,
				Symbol:     d.Symbol,
				Quantity:   d.Quantity,
				Leverage:   d.Leverage,
				Price:      d.Price,
				StopLoss:   d.StopLoss,
				TakeProfit: d.TakeProfit,
				Confidence: d.Confidence,
				Reasoning:  d.Reasoning,
				Timestamp:  time.Now().UTC(),
				Success:    true, // Grid decisions are executed inline
			}
			record.Decisions = append(record.Decisions, actionRecord)
		}
	}

	record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("Grid cycle completed with %d decisions", len(decision.Decisions)))

	if err := at.store.Decision().LogDecision(record); err != nil {
		logger.Warnf("[Grid] Failed to save decision record: %v", err)
	}
}

// IsGridStrategy returns true if current strategy is grid trading
func (at *AutoTrader) IsGridStrategy() bool {
	if at.config.StrategyConfig == nil {
		return false
	}
	return at.config.StrategyConfig.StrategyType == "grid_trading" && at.config.StrategyConfig.GridConfig != nil
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
	// Default: ±3% from current price, scaled by grid count
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

// GridRiskInfo contains risk information for frontend display
type GridRiskInfo struct {
	CurrentLeverage     int     `json:"current_leverage"`
	EffectiveLeverage   float64 `json:"effective_leverage"`
	RecommendedLeverage int     `json:"recommended_leverage"`

	CurrentPosition float64 `json:"current_position"`
	MaxPosition     float64 `json:"max_position"`
	PositionPercent float64 `json:"position_percent"`

	LiquidationPrice    float64 `json:"liquidation_price"`
	LiquidationDistance float64 `json:"liquidation_distance"`

	RegimeLevel string `json:"regime_level"`

	ShortBoxUpper float64 `json:"short_box_upper"`
	ShortBoxLower float64 `json:"short_box_lower"`
	MidBoxUpper   float64 `json:"mid_box_upper"`
	MidBoxLower   float64 `json:"mid_box_lower"`
	LongBoxUpper  float64 `json:"long_box_upper"`
	LongBoxLower  float64 `json:"long_box_lower"`
	CurrentPrice  float64 `json:"current_price"`

	BreakoutLevel     string `json:"breakout_level"`
	BreakoutDirection string `json:"breakout_direction"`

	// Grid direction
	CurrentGridDirection    string `json:"current_grid_direction"`
	DirectionChangeCount    int    `json:"direction_change_count"`
	EnableDirectionAdjust   bool   `json:"enable_direction_adjust"`
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

// checkAndExecuteStopLoss checks if any filled level has exceeded stop loss and closes it
func (at *AutoTrader) checkAndExecuteStopLoss() {
	gridConfig := at.config.StrategyConfig.GridConfig
	if gridConfig.StopLossPct <= 0 {
		return // Stop loss not configured
	}

	currentPrice, err := at.trader.GetMarketPrice(gridConfig.Symbol)
	if err != nil {
		logger.Warnf("[Grid] Failed to get market price for stop loss check: %v", err)
		return
	}

	at.gridState.mu.Lock()
	defer at.gridState.mu.Unlock()

	for i := range at.gridState.Levels {
		level := &at.gridState.Levels[i]
		if level.State != "filled" || level.PositionEntry <= 0 {
			continue
		}

		// Calculate loss percentage
		var lossPct float64
		if level.Side == "buy" {
			// Long position: loss when price drops
			lossPct = (level.PositionEntry - currentPrice) / level.PositionEntry * 100
		} else {
			// Short position: loss when price rises
			lossPct = (currentPrice - level.PositionEntry) / level.PositionEntry * 100
		}

		// Check if stop loss triggered
		if lossPct >= gridConfig.StopLossPct {
			logger.Warnf("[Grid] STOP LOSS TRIGGERED: Level %d, entry=$%.2f, current=$%.2f, loss=%.2f%%",
				i, level.PositionEntry, currentPrice, lossPct)

			// Close the position
			var closeErr error
			if level.Side == "buy" {
				_, closeErr = at.trader.CloseLong(gridConfig.Symbol, level.PositionSize)
			} else {
				_, closeErr = at.trader.CloseShort(gridConfig.Symbol, level.PositionSize)
			}

			if closeErr != nil {
				logger.Errorf("[Grid] Failed to execute stop loss for level %d: %v", i, closeErr)
			} else {
				level.State = "stopped"
				realizedLoss := -lossPct * level.AllocatedUSD / 100
				level.UnrealizedPnL = realizedLoss
				at.gridState.TotalTrades++
				// Update daily PnL tracking (lock already held, update directly)
				at.gridState.DailyPnL += realizedLoss
				at.gridState.TotalProfit += realizedLoss
				logger.Infof("[Grid] Stop loss executed: Level %d closed at $%.2f (loss %.2f%%)",
					i, currentPrice, lossPct)
			}
		}
	}
}
