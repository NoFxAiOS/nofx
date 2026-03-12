package trader

import (
	"encoding/json"
	"fmt"
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
	IsPaused      bool
	IsInitialized bool

	// Performance tracking
	TotalProfit    float64
	TotalTrades    int
	WinningTrades  int
	MaxDrawdown    float64
	PeakEquity     float64
	DailyPnL       float64
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
	CurrentDirection     market.GridDirection
	DirectionChangedAt   time.Time
	DirectionChangeCount int
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
// Breakout Detection (price vs grid boundary)
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

// ============================================================================
// AutoTrader Grid Lifecycle
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

	logger.Infof("[Grid] Initialized: %d levels, $%.2f - $%.2f, spacing $%.2f",
		gridConfig.GridCount, at.gridState.LowerPrice, at.gridState.UpperPrice, at.gridState.GridSpacing)

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

// IsGridStrategy returns true if current strategy is grid trading
func (at *AutoTrader) IsGridStrategy() bool {
	if at.config.StrategyConfig == nil {
		return false
	}
	return at.config.StrategyConfig.StrategyType == "grid_trading" && at.config.StrategyConfig.GridConfig != nil
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
	CurrentGridDirection  string `json:"current_grid_direction"`
	DirectionChangeCount  int    `json:"direction_change_count"`
	EnableDirectionAdjust bool   `json:"enable_direction_adjust"`
}
