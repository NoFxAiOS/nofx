# Grid Market Regime Detection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement multi-period box indicators and 4-level ranging classification for grid trading with automatic parameter adjustment and breakout handling.

**Architecture:** Add Donchian channel calculation to market package, extend grid models with box/regime fields, implement breakout detection in auto_trader_grid, add risk control panel to frontend.

**Tech Stack:** Go (backend), React/TypeScript (frontend), GORM (database), 1-hour Kline data

---

## Task 1: Add Donchian Channel Calculation

**Files:**
- Modify: `market/data.go`
- Test: `market/data_test.go`

**Step 1: Write the failing test**

Add to `market/data_test.go`:

```go
func TestCalculateDonchian(t *testing.T) {
	// Create test klines with known high/low values
	klines := []Kline{
		{High: 100, Low: 90},
		{High: 105, Low: 88},
		{High: 102, Low: 92},
		{High: 108, Low: 85},
		{High: 103, Low: 91},
	}

	upper, lower := calculateDonchian(klines, 5)

	if upper != 108 {
		t.Errorf("Expected upper = 108, got %v", upper)
	}
	if lower != 85 {
		t.Errorf("Expected lower = 85, got %v", lower)
	}
}

func TestCalculateDonchian_PartialPeriod(t *testing.T) {
	klines := []Kline{
		{High: 100, Low: 90},
		{High: 105, Low: 88},
	}

	upper, lower := calculateDonchian(klines, 10)

	// Should use all available klines when period > len(klines)
	if upper != 105 {
		t.Errorf("Expected upper = 105, got %v", upper)
	}
	if lower != 88 {
		t.Errorf("Expected lower = 88, got %v", lower)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./market/... -run TestCalculateDonchian`
Expected: FAIL with "undefined: calculateDonchian"

**Step 3: Write minimal implementation**

Add to `market/data.go`:

```go
// calculateDonchian calculates Donchian channel (highest high, lowest low) for given period
func calculateDonchian(klines []Kline, period int) (upper, lower float64) {
	if len(klines) == 0 {
		return 0, 0
	}

	// Use all available klines if period > len(klines)
	start := len(klines) - period
	if start < 0 {
		start = 0
	}

	upper = klines[start].High
	lower = klines[start].Low

	for i := start + 1; i < len(klines); i++ {
		if klines[i].High > upper {
			upper = klines[i].High
		}
		if klines[i].Low < lower {
			lower = klines[i].Low
		}
	}

	return upper, lower
}

// ExportCalculateDonchian exports calculateDonchian for testing
func ExportCalculateDonchian(klines []Kline, period int) (float64, float64) {
	return calculateDonchian(klines, period)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./market/... -run TestCalculateDonchian`
Expected: PASS

**Step 5: Commit**

```bash
git add market/data.go market/data_test.go
git commit -m "feat(market): add Donchian channel calculation"
```

---

## Task 2: Add Box Data Types

**Files:**
- Modify: `market/types.go`

**Step 1: Add BoxData struct**

Add to `market/types.go`:

```go
// BoxData represents multi-period Donchian channel (box) data
type BoxData struct {
	// Short-term box (72 1h candles = 3 days)
	ShortUpper float64 `json:"short_upper"`
	ShortLower float64 `json:"short_lower"`

	// Mid-term box (240 1h candles = 10 days)
	MidUpper float64 `json:"mid_upper"`
	MidLower float64 `json:"mid_lower"`

	// Long-term box (500 1h candles = ~21 days)
	LongUpper float64 `json:"long_upper"`
	LongLower float64 `json:"long_lower"`

	// Current price position relative to boxes
	CurrentPrice float64 `json:"current_price"`
}

// RegimeLevel represents the ranging classification level
type RegimeLevel string

const (
	RegimeLevelNarrow   RegimeLevel = "narrow"   // 窄幅震荡
	RegimeLevelStandard RegimeLevel = "standard" // 标准震荡
	RegimeLevelWide     RegimeLevel = "wide"     // 宽幅震荡
	RegimeLevelVolatile RegimeLevel = "volatile" // 剧烈震荡
	RegimeLevelTrending RegimeLevel = "trending" // 趋势
)

// BreakoutLevel represents which box level has been broken
type BreakoutLevel string

const (
	BreakoutNone   BreakoutLevel = "none"
	BreakoutShort  BreakoutLevel = "short"
	BreakoutMid    BreakoutLevel = "mid"
	BreakoutLong   BreakoutLevel = "long"
)
```

**Step 2: Commit**

```bash
git add market/types.go
git commit -m "feat(market): add BoxData and RegimeLevel types"
```

---

## Task 3: Add GetBoxData Function

**Files:**
- Modify: `market/data.go`
- Test: `market/data_test.go`

**Step 1: Write the failing test**

Add to `market/data_test.go`:

```go
func TestGetBoxData(t *testing.T) {
	// This test requires mocking kline data source
	// For now, test the internal calculation logic
	klines := make([]Kline, 500)
	for i := 0; i < 500; i++ {
		// Create synthetic price data
		basePrice := 100.0
		klines[i] = Kline{
			High: basePrice + float64(i%10),
			Low:  basePrice - float64(i%10),
		}
	}

	box := calculateBoxData(klines, 100.0)

	if box.ShortUpper == 0 || box.ShortLower == 0 {
		t.Error("Short box should not be zero")
	}
	if box.MidUpper == 0 || box.MidLower == 0 {
		t.Error("Mid box should not be zero")
	}
	if box.LongUpper == 0 || box.LongLower == 0 {
		t.Error("Long box should not be zero")
	}
	if box.CurrentPrice != 100.0 {
		t.Errorf("Expected CurrentPrice = 100.0, got %v", box.CurrentPrice)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./market/... -run TestGetBoxData`
Expected: FAIL with "undefined: calculateBoxData"

**Step 3: Write minimal implementation**

Add to `market/data.go`:

```go
const (
	ShortBoxPeriod = 72  // 3 days of 1h candles
	MidBoxPeriod   = 240 // 10 days of 1h candles
	LongBoxPeriod  = 500 // ~21 days of 1h candles
)

// calculateBoxData calculates multi-period box data from klines
func calculateBoxData(klines []Kline, currentPrice float64) *BoxData {
	box := &BoxData{
		CurrentPrice: currentPrice,
	}

	if len(klines) == 0 {
		return box
	}

	box.ShortUpper, box.ShortLower = calculateDonchian(klines, ShortBoxPeriod)
	box.MidUpper, box.MidLower = calculateDonchian(klines, MidBoxPeriod)
	box.LongUpper, box.LongLower = calculateDonchian(klines, LongBoxPeriod)

	return box
}

// GetBoxData fetches 1h klines and calculates box data for a symbol
func GetBoxData(symbol string) (*BoxData, error) {
	symbol = Normalize(symbol)

	// Fetch 500 1h klines
	var klines []Kline
	var err error

	if IsXyzDexAsset(symbol) {
		klines, err = getKlinesFromHyperliquid(symbol, "1h", LongBoxPeriod)
	} else {
		klines, err = getKlinesFromCoinAnk(symbol, "1h", LongBoxPeriod)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get 1h klines: %w", err)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("no kline data available")
	}

	currentPrice := klines[len(klines)-1].Close

	return calculateBoxData(klines, currentPrice), nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./market/... -run TestGetBoxData`
Expected: PASS

**Step 5: Commit**

```bash
git add market/data.go market/data_test.go
git commit -m "feat(market): add GetBoxData for multi-period box calculation"
```

---

## Task 4: Update GridConfigModel with Box Parameters

**Files:**
- Modify: `store/grid.go`

**Step 1: Add new fields to GridConfigModel**

Add fields after `TrendResumeThreshold` in `store/grid.go`:

```go
	// Box indicator periods (1h candles)
	ShortBoxPeriod int `json:"short_box_period" gorm:"default:72"`  // 3 days
	MidBoxPeriod   int `json:"mid_box_period" gorm:"default:240"`   // 10 days
	LongBoxPeriod  int `json:"long_box_period" gorm:"default:500"`  // 21 days

	// Effective leverage limits by regime level
	NarrowRegimeLeverage   int `json:"narrow_regime_leverage" gorm:"default:2"`
	StandardRegimeLeverage int `json:"standard_regime_leverage" gorm:"default:4"`
	WideRegimeLeverage     int `json:"wide_regime_leverage" gorm:"default:3"`
	VolatileRegimeLeverage int `json:"volatile_regime_leverage" gorm:"default:2"`

	// Position limits by regime level (percentage of total investment)
	NarrowRegimePositionPct   float64 `json:"narrow_regime_position_pct" gorm:"default:40"`
	StandardRegimePositionPct float64 `json:"standard_regime_position_pct" gorm:"default:70"`
	WideRegimePositionPct     float64 `json:"wide_regime_position_pct" gorm:"default:60"`
	VolatileRegimePositionPct float64 `json:"volatile_regime_position_pct" gorm:"default:40"`
```

**Step 2: Commit**

```bash
git add store/grid.go
git commit -m "feat(store): add box period and regime leverage fields to GridConfigModel"
```

---

## Task 5: Update GridInstanceModel with Box State

**Files:**
- Modify: `store/grid.go`

**Step 1: Add new fields to GridInstanceModel**

Add fields after `ConsecutiveTrending` in `store/grid.go`:

```go
	// Current regime level (narrow/standard/wide/volatile/trending)
	CurrentRegimeLevel string `json:"current_regime_level" gorm:"default:standard"`

	// Box state
	ShortBoxUpper float64 `json:"short_box_upper"`
	ShortBoxLower float64 `json:"short_box_lower"`
	MidBoxUpper   float64 `json:"mid_box_upper"`
	MidBoxLower   float64 `json:"mid_box_lower"`
	LongBoxUpper  float64 `json:"long_box_upper"`
	LongBoxLower  float64 `json:"long_box_lower"`

	// Breakout state
	BreakoutLevel        string    `json:"breakout_level" gorm:"default:none"` // none/short/mid/long
	BreakoutDirection    string    `json:"breakout_direction"`                 // up/down
	BreakoutConfirmCount int       `json:"breakout_confirm_count" gorm:"default:0"`
	BreakoutStartTime    time.Time `json:"breakout_start_time"`

	// Position adjustment due to breakout
	PositionReductionPct float64 `json:"position_reduction_pct" gorm:"default:0"` // 0 = normal, 50 = reduced
```

**Step 2: Commit**

```bash
git add store/grid.go
git commit -m "feat(store): add box state and breakout fields to GridInstanceModel"
```

---

## Task 6: Add Regime Level Classification

**Files:**
- Create: `trader/grid_regime.go`
- Test: `trader/grid_regime_test.go`

**Step 1: Write the failing test**

Create `trader/grid_regime_test.go`:

```go
package trader

import (
	"nofx/market"
	"testing"
)

func TestClassifyRegimeLevel(t *testing.T) {
	tests := []struct {
		name           string
		bollingerWidth float64
		atr14Pct       float64
		expected       market.RegimeLevel
	}{
		{"narrow", 1.5, 0.8, market.RegimeLevelNarrow},
		{"standard", 2.5, 1.5, market.RegimeLevelStandard},
		{"wide", 3.5, 2.5, market.RegimeLevelWide},
		{"volatile", 5.0, 4.0, market.RegimeLevelVolatile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyRegimeLevel(tt.bollingerWidth, tt.atr14Pct)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./trader/... -run TestClassifyRegimeLevel`
Expected: FAIL with "undefined: classifyRegimeLevel"

**Step 3: Write minimal implementation**

Create `trader/grid_regime.go`:

```go
package trader

import "nofx/market"

// classifyRegimeLevel determines the regime level based on market indicators
// bollingerWidth: Bollinger band width as percentage
// atr14Pct: ATR14 as percentage of current price
func classifyRegimeLevel(bollingerWidth, atr14Pct float64) market.RegimeLevel {
	// Narrow: Bollinger < 2%, ATR < 1%
	if bollingerWidth < 2.0 && atr14Pct < 1.0 {
		return market.RegimeLevelNarrow
	}

	// Standard: Bollinger 2-3%, ATR 1-2%
	if bollingerWidth <= 3.0 && atr14Pct <= 2.0 {
		return market.RegimeLevelStandard
	}

	// Wide: Bollinger 3-4%, ATR 2-3%
	if bollingerWidth <= 4.0 && atr14Pct <= 3.0 {
		return market.RegimeLevelWide
	}

	// Volatile: Bollinger > 4%, ATR > 3%
	return market.RegimeLevelVolatile
}

// getRegimeLeverageLimit returns the effective leverage limit for a regime level
func getRegimeLeverageLimit(level market.RegimeLevel, config *store.GridStrategyConfig) int {
	switch level {
	case market.RegimeLevelNarrow:
		return config.NarrowRegimeLeverage
	case market.RegimeLevelStandard:
		return config.StandardRegimeLeverage
	case market.RegimeLevelWide:
		return config.WideRegimeLeverage
	case market.RegimeLevelVolatile:
		return config.VolatileRegimeLeverage
	default:
		return 2 // Conservative default
	}
}

// getRegimePositionLimit returns the position limit percentage for a regime level
func getRegimePositionLimit(level market.RegimeLevel, config *store.GridStrategyConfig) float64 {
	switch level {
	case market.RegimeLevelNarrow:
		return config.NarrowRegimePositionPct
	case market.RegimeLevelStandard:
		return config.StandardRegimePositionPct
	case market.RegimeLevelWide:
		return config.WideRegimePositionPct
	case market.RegimeLevelVolatile:
		return config.VolatileRegimePositionPct
	default:
		return 40.0 // Conservative default
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./trader/... -run TestClassifyRegimeLevel`
Expected: PASS

**Step 5: Commit**

```bash
git add trader/grid_regime.go trader/grid_regime_test.go
git commit -m "feat(trader): add regime level classification"
```

---

## Task 7: Add Breakout Detection

**Files:**
- Modify: `trader/grid_regime.go`
- Test: `trader/grid_regime_test.go`

**Step 1: Write the failing test**

Add to `trader/grid_regime_test.go`:

```go
func TestDetectBoxBreakout(t *testing.T) {
	box := &market.BoxData{
		ShortUpper:   100,
		ShortLower:   90,
		MidUpper:     105,
		MidLower:     85,
		LongUpper:    110,
		LongLower:    80,
		CurrentPrice: 95,
	}

	// No breakout
	level, direction := detectBoxBreakout(box)
	if level != market.BreakoutNone {
		t.Errorf("Expected no breakout, got %v", level)
	}

	// Short breakout up
	box.CurrentPrice = 101
	level, direction = detectBoxBreakout(box)
	if level != market.BreakoutShort || direction != "up" {
		t.Errorf("Expected short breakout up, got %v %v", level, direction)
	}

	// Mid breakout down
	box.CurrentPrice = 84
	level, direction = detectBoxBreakout(box)
	if level != market.BreakoutMid || direction != "down" {
		t.Errorf("Expected mid breakout down, got %v %v", level, direction)
	}

	// Long breakout up
	box.CurrentPrice = 112
	level, direction = detectBoxBreakout(box)
	if level != market.BreakoutLong || direction != "up" {
		t.Errorf("Expected long breakout up, got %v %v", level, direction)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./trader/... -run TestDetectBoxBreakout`
Expected: FAIL with "undefined: detectBoxBreakout"

**Step 3: Write minimal implementation**

Add to `trader/grid_regime.go`:

```go
// detectBoxBreakout checks if price has broken out of any box level
// Returns the highest breakout level and direction
func detectBoxBreakout(box *market.BoxData) (market.BreakoutLevel, string) {
	price := box.CurrentPrice

	// Check long box first (highest priority)
	if price > box.LongUpper {
		return market.BreakoutLong, "up"
	}
	if price < box.LongLower {
		return market.BreakoutLong, "down"
	}

	// Check mid box
	if price > box.MidUpper {
		return market.BreakoutMid, "up"
	}
	if price < box.MidLower {
		return market.BreakoutMid, "down"
	}

	// Check short box
	if price > box.ShortUpper {
		return market.BreakoutShort, "up"
	}
	if price < box.ShortLower {
		return market.BreakoutShort, "down"
	}

	return market.BreakoutNone, ""
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./trader/... -run TestDetectBoxBreakout`
Expected: PASS

**Step 5: Commit**

```bash
git add trader/grid_regime.go trader/grid_regime_test.go
git commit -m "feat(trader): add box breakout detection"
```

---

## Task 8: Add Breakout Confirmation Logic

**Files:**
- Modify: `trader/grid_regime.go`
- Test: `trader/grid_regime_test.go`

**Step 1: Write the failing test**

Add to `trader/grid_regime_test.go`:

```go
func TestBreakoutConfirmation(t *testing.T) {
	state := &BreakoutState{
		Level:        market.BreakoutShort,
		Direction:    "up",
		ConfirmCount: 0,
	}

	// First confirmation
	confirmed := confirmBreakout(state, market.BreakoutShort, "up")
	if confirmed || state.ConfirmCount != 1 {
		t.Errorf("Expected not confirmed, count=1, got confirmed=%v count=%d", confirmed, state.ConfirmCount)
	}

	// Second confirmation
	confirmed = confirmBreakout(state, market.BreakoutShort, "up")
	if confirmed || state.ConfirmCount != 2 {
		t.Errorf("Expected not confirmed, count=2, got confirmed=%v count=%d", confirmed, state.ConfirmCount)
	}

	// Third confirmation - should confirm
	confirmed = confirmBreakout(state, market.BreakoutShort, "up")
	if !confirmed || state.ConfirmCount != 3 {
		t.Errorf("Expected confirmed, count=3, got confirmed=%v count=%d", confirmed, state.ConfirmCount)
	}

	// Reset on price return
	state.ConfirmCount = 2
	confirmed = confirmBreakout(state, market.BreakoutNone, "")
	if state.ConfirmCount != 0 {
		t.Errorf("Expected count reset to 0, got %d", state.ConfirmCount)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./trader/... -run TestBreakoutConfirmation`
Expected: FAIL with "undefined: BreakoutState"

**Step 3: Write minimal implementation**

Add to `trader/grid_regime.go`:

```go
const BreakoutConfirmRequired = 3 // 3 candles to confirm breakout

// BreakoutState tracks the current breakout state
type BreakoutState struct {
	Level        market.BreakoutLevel
	Direction    string
	ConfirmCount int
	StartTime    time.Time
}

// confirmBreakout updates breakout state and returns true if breakout is confirmed
func confirmBreakout(state *BreakoutState, currentLevel market.BreakoutLevel, direction string) bool {
	// If price returned to box, reset state
	if currentLevel == market.BreakoutNone {
		state.ConfirmCount = 0
		state.Level = market.BreakoutNone
		state.Direction = ""
		return false
	}

	// If same breakout continues, increment count
	if state.Level == currentLevel && state.Direction == direction {
		state.ConfirmCount++
	} else {
		// New breakout, reset count
		state.Level = currentLevel
		state.Direction = direction
		state.ConfirmCount = 1
		state.StartTime = time.Now()
	}

	return state.ConfirmCount >= BreakoutConfirmRequired
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./trader/... -run TestBreakoutConfirmation`
Expected: PASS

**Step 5: Commit**

```bash
git add trader/grid_regime.go trader/grid_regime_test.go
git commit -m "feat(trader): add breakout confirmation logic"
```

---

## Task 9: Add Breakout Handler

**Files:**
- Modify: `trader/grid_regime.go`
- Test: `trader/grid_regime_test.go`

**Step 1: Write the failing test**

Add to `trader/grid_regime_test.go`:

```go
func TestGetBreakoutAction(t *testing.T) {
	tests := []struct {
		level    market.BreakoutLevel
		expected BreakoutAction
	}{
		{market.BreakoutNone, BreakoutActionNone},
		{market.BreakoutShort, BreakoutActionReducePosition},
		{market.BreakoutMid, BreakoutActionPauseGrid},
		{market.BreakoutLong, BreakoutActionCloseAll},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			action := getBreakoutAction(tt.level)
			if action != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, action)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./trader/... -run TestGetBreakoutAction`
Expected: FAIL with "undefined: BreakoutAction"

**Step 3: Write minimal implementation**

Add to `trader/grid_regime.go`:

```go
// BreakoutAction represents the action to take on breakout
type BreakoutAction int

const (
	BreakoutActionNone BreakoutAction = iota
	BreakoutActionReducePosition // Short box breakout: reduce to 50%
	BreakoutActionPauseGrid      // Mid box breakout: pause grid + cancel orders
	BreakoutActionCloseAll       // Long box breakout: pause + cancel + close all
)

// getBreakoutAction returns the appropriate action for a breakout level
func getBreakoutAction(level market.BreakoutLevel) BreakoutAction {
	switch level {
	case market.BreakoutShort:
		return BreakoutActionReducePosition
	case market.BreakoutMid:
		return BreakoutActionPauseGrid
	case market.BreakoutLong:
		return BreakoutActionCloseAll
	default:
		return BreakoutActionNone
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yida/gopro/open-nofx && go test -v ./trader/... -run TestGetBreakoutAction`
Expected: PASS

**Step 5: Commit**

```bash
git add trader/grid_regime.go trader/grid_regime_test.go
git commit -m "feat(trader): add breakout action handler"
```

---

## Task 10: Integrate Breakout Detection into Grid Cycle

**Files:**
- Modify: `trader/auto_trader_grid.go`

**Step 1: Add checkBoxBreakout method**

Add to `trader/auto_trader_grid.go` after `checkBreakout` function:

```go
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

	// Update instance with box values
	at.gridState.mu.Lock()
	// Store box values in grid state for reference
	at.gridState.mu.Unlock()

	// Detect breakout
	breakoutLevel, direction := detectBoxBreakout(box)

	// Get current breakout state from instance
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
	action := getBreakoutAction(breakoutLevel)
	return at.executeBreakoutAction(action)
}

// executeBreakoutAction executes the appropriate action for a breakout
func (at *AutoTrader) executeBreakoutAction(action BreakoutAction) error {
	gridConfig := at.config.StrategyConfig.GridConfig

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
	}

	return nil
}

// closeAllPositions closes all open positions
func (at *AutoTrader) closeAllPositions() error {
	gridConfig := at.config.StrategyConfig.GridConfig

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
```

**Step 2: Add checkBoxBreakout call to RunGridCycle**

In `RunGridCycle`, add after existing breakout check:

```go
	// Check multi-period box breakout
	if err := at.checkBoxBreakout(); err != nil {
		logger.Infof("Box breakout check error: %v", err)
	}
```

**Step 3: Commit**

```bash
git add trader/auto_trader_grid.go
git commit -m "feat(trader): integrate box breakout detection into grid cycle"
```

---

## Task 11: Add False Breakout Recovery

**Files:**
- Modify: `trader/auto_trader_grid.go`

**Step 1: Add recovery logic**

Add to `trader/auto_trader_grid.go`:

```go
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
	at.gridState.mu.RUnlock()

	// Only check if we had a breakout
	if breakoutLevel == string(market.BreakoutNone) && positionReduction == 0 && !isPaused {
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

	return nil
}
```

**Step 2: Add call in RunGridCycle**

```go
	// Check for false breakout recovery
	if err := at.checkFalseBreakoutRecovery(); err != nil {
		logger.Infof("False breakout recovery check error: %v", err)
	}
```

**Step 3: Commit**

```bash
git add trader/auto_trader_grid.go
git commit -m "feat(trader): add false breakout recovery logic"
```

---

## Task 12: Update GridState with Box Fields

**Files:**
- Modify: `trader/auto_trader_grid.go`

**Step 1: Add box fields to GridState struct**

Add to `GridState` struct in `trader/auto_trader_grid.go`:

```go
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
```

**Step 2: Commit**

```bash
git add trader/auto_trader_grid.go
git commit -m "feat(trader): add box and regime fields to GridState"
```

---

## Task 13: Add Frontend Types

**Files:**
- Modify: `web/src/types.ts` (or equivalent types file)

**Step 1: Add grid risk info types**

Add to types file:

```typescript
export interface GridRiskInfo {
  // Leverage info
  currentLeverage: number
  effectiveLeverage: number
  recommendedLeverage: number

  // Position info
  currentPosition: number
  maxPosition: number
  positionPercent: number

  // Liquidation info
  liquidationPrice: number
  liquidationDistance: number // percentage

  // Market state
  regimeLevel: 'narrow' | 'standard' | 'wide' | 'volatile' | 'trending'

  // Box state
  shortBoxUpper: number
  shortBoxLower: number
  midBoxUpper: number
  midBoxLower: number
  longBoxUpper: number
  longBoxLower: number
  currentPrice: number

  // Breakout state
  breakoutLevel: 'none' | 'short' | 'mid' | 'long'
  breakoutDirection: 'up' | 'down' | ''
}
```

**Step 2: Commit**

```bash
git add web/src/types.ts
git commit -m "feat(web): add GridRiskInfo type"
```

---

## Task 14: Add API Endpoint for Risk Info

**Files:**
- Modify: `api/server.go`

**Step 1: Add handler function**

Add to `api/server.go`:

```go
// handleGetGridRiskInfo returns current risk information for a grid trader
func (s *Server) handleGetGridRiskInfo(c *gin.Context) {
	traderID := c.Param("id")

	trader, err := s.manager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trader not found"})
		return
	}

	autoTrader, ok := trader.(*trader.AutoTrader)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not an auto trader"})
		return
	}

	riskInfo := autoTrader.GetGridRiskInfo()
	c.JSON(http.StatusOK, riskInfo)
}
```

**Step 2: Add route**

Add route in `setupRoutes`:

```go
	api.GET("/traders/:id/grid-risk", s.handleGetGridRiskInfo)
```

**Step 3: Commit**

```bash
git add api/server.go
git commit -m "feat(api): add grid risk info endpoint"
```

---

## Task 15: Add GetGridRiskInfo Method to AutoTrader

**Files:**
- Modify: `trader/auto_trader_grid.go`

**Step 1: Add method**

Add to `trader/auto_trader_grid.go`:

```go
// GridRiskInfo contains risk information for frontend display
type GridRiskInfo struct {
	CurrentLeverage     int     `json:"current_leverage"`
	EffectiveLeverage   float64 `json:"effective_leverage"`
	RecommendedLeverage int     `json:"recommended_leverage"`

	CurrentPosition  float64 `json:"current_position"`
	MaxPosition      float64 `json:"max_position"`
	PositionPercent  float64 `json:"position_percent"`

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
}

// GetGridRiskInfo returns current risk information
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
	for _, pos := range positions {
		if sym, _ := pos["symbol"].(string); sym == gridConfig.Symbol {
			size, _ := pos["positionAmt"].(float64)
			entry, _ := pos["entryPrice"].(float64)
			currentPositionValue = math.Abs(size * entry)
			break
		}
	}

	effectiveLeverage := currentPositionValue / totalInvestment

	// Calculate max position based on regime
	regimeLevel := market.RegimeLevel(at.gridState.CurrentRegimeLevel)
	maxPositionPct := getRegimePositionLimit(regimeLevel, gridConfig)
	maxPosition := totalInvestment * maxPositionPct / 100 * float64(leverage)
	recommendedLeverage := getRegimeLeverageLimit(regimeLevel, gridConfig)

	// Calculate liquidation distance
	liquidationDistance := 100.0 / float64(leverage) * 0.9 // ~90% of theoretical max

	var liquidationPrice float64
	if currentPositionValue > 0 {
		liquidationPrice = currentPrice * (1 - liquidationDistance/100)
	}

	return &GridRiskInfo{
		CurrentLeverage:     leverage,
		EffectiveLeverage:   effectiveLeverage,
		RecommendedLeverage: recommendedLeverage,

		CurrentPosition:  currentPositionValue,
		MaxPosition:      maxPosition,
		PositionPercent:  currentPositionValue / maxPosition * 100,

		LiquidationPrice:    liquidationPrice,
		LiquidationDistance: liquidationDistance,

		RegimeLevel: at.gridState.CurrentRegimeLevel,

		ShortBoxUpper: at.gridState.ShortBoxUpper,
		ShortBoxLower: at.gridState.ShortBoxLower,
		MidBoxUpper:   at.gridState.MidBoxUpper,
		MidBoxLower:   at.gridState.MidBoxLower,
		LongBoxUpper:  at.gridState.LongBoxUpper,
		LongBoxLower:  at.gridState.LongBoxLower,
		CurrentPrice:  currentPrice,

		BreakoutLevel:     at.gridState.BreakoutLevel,
		BreakoutDirection: at.gridState.BreakoutDirection,
	}
}
```

**Step 2: Commit**

```bash
git add trader/auto_trader_grid.go
git commit -m "feat(trader): add GetGridRiskInfo method"
```

---

## Task 16: Create GridRiskPanel Component

**Files:**
- Create: `web/src/components/strategy/GridRiskPanel.tsx`

**Step 1: Create component**

Create `web/src/components/strategy/GridRiskPanel.tsx`:

```tsx
import { useState, useEffect } from 'react'
import { AlertTriangle, TrendingUp, Shield, Box } from 'lucide-react'

interface GridRiskInfo {
  currentLeverage: number
  effectiveLeverage: number
  recommendedLeverage: number
  currentPosition: number
  maxPosition: number
  positionPercent: number
  liquidationPrice: number
  liquidationDistance: number
  regimeLevel: string
  shortBoxUpper: number
  shortBoxLower: number
  midBoxUpper: number
  midBoxLower: number
  longBoxUpper: number
  longBoxLower: number
  currentPrice: number
  breakoutLevel: string
  breakoutDirection: string
}

interface GridRiskPanelProps {
  traderId: string
  language: string
}

export function GridRiskPanel({ traderId, language }: GridRiskPanelProps) {
  const [riskInfo, setRiskInfo] = useState<GridRiskInfo | null>(null)
  const [loading, setLoading] = useState(true)

  const t = (key: string) => {
    const translations: Record<string, Record<string, string>> = {
      leverageInfo: { zh: '杠杆信息', en: 'Leverage Info' },
      currentLeverage: { zh: '当前杠杆', en: 'Current Leverage' },
      effectiveLeverage: { zh: '有效杠杆', en: 'Effective Leverage' },
      recommendedLeverage: { zh: '推荐杠杆', en: 'Recommended Leverage' },
      positionInfo: { zh: '仓位信息', en: 'Position Info' },
      currentPosition: { zh: '当前仓位', en: 'Current Position' },
      maxPosition: { zh: '最大仓位', en: 'Max Position' },
      liquidationInfo: { zh: '爆仓信息', en: 'Liquidation Info' },
      liquidationPrice: { zh: '爆仓价格', en: 'Liquidation Price' },
      liquidationDistance: { zh: '爆仓距离', en: 'Distance' },
      marketState: { zh: '市场状态', en: 'Market State' },
      regimeLevel: { zh: '震荡级别', en: 'Regime Level' },
      boxState: { zh: '箱体状态', en: 'Box State' },
      shortBox: { zh: '短期箱体', en: 'Short Box' },
      midBox: { zh: '中期箱体', en: 'Mid Box' },
      longBox: { zh: '长期箱体', en: 'Long Box' },
      narrow: { zh: '窄幅震荡', en: 'Narrow' },
      standard: { zh: '标准震荡', en: 'Standard' },
      wide: { zh: '宽幅震荡', en: 'Wide' },
      volatile: { zh: '剧烈震荡', en: 'Volatile' },
      trending: { zh: '趋势', en: 'Trending' },
      breakout: { zh: '突破', en: 'Breakout' },
      none: { zh: '无', en: 'None' },
    }
    return translations[key]?.[language] || key
  }

  useEffect(() => {
    const fetchRiskInfo = async () => {
      try {
        const res = await fetch(`/api/traders/${traderId}/grid-risk`)
        if (res.ok) {
          const data = await res.json()
          setRiskInfo(data)
        }
      } catch (err) {
        console.error('Failed to fetch risk info:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchRiskInfo()
    const interval = setInterval(fetchRiskInfo, 10000) // Update every 10s
    return () => clearInterval(interval)
  }, [traderId])

  if (loading || !riskInfo) {
    return <div className="animate-pulse bg-gray-800 h-48 rounded" />
  }

  const getRegimeColor = (level: string) => {
    switch (level) {
      case 'narrow': return 'text-green-400'
      case 'standard': return 'text-blue-400'
      case 'wide': return 'text-yellow-400'
      case 'volatile': return 'text-orange-400'
      case 'trending': return 'text-red-400'
      default: return 'text-gray-400'
    }
  }

  return (
    <div className="bg-[#0B0E11] rounded-lg p-4 space-y-4">
      {/* Leverage Info */}
      <div className="border-b border-gray-700 pb-3">
        <h3 className="text-sm font-medium text-gray-400 flex items-center gap-2 mb-2">
          <TrendingUp size={14} />
          {t('leverageInfo')}
        </h3>
        <div className="grid grid-cols-3 gap-2 text-sm">
          <div>
            <div className="text-gray-500">{t('currentLeverage')}</div>
            <div className="text-white">{riskInfo.currentLeverage}x</div>
          </div>
          <div>
            <div className="text-gray-500">{t('effectiveLeverage')}</div>
            <div className="text-white">{riskInfo.effectiveLeverage.toFixed(2)}x</div>
          </div>
          <div>
            <div className="text-gray-500">{t('recommendedLeverage')}</div>
            <div className="text-green-400">{riskInfo.recommendedLeverage}x</div>
          </div>
        </div>
      </div>

      {/* Position Info */}
      <div className="border-b border-gray-700 pb-3">
        <h3 className="text-sm font-medium text-gray-400 flex items-center gap-2 mb-2">
          <Shield size={14} />
          {t('positionInfo')}
        </h3>
        <div className="grid grid-cols-2 gap-2 text-sm">
          <div>
            <div className="text-gray-500">{t('currentPosition')}</div>
            <div className="text-white">${riskInfo.currentPosition.toFixed(2)}</div>
          </div>
          <div>
            <div className="text-gray-500">{t('maxPosition')}</div>
            <div className="text-white">${riskInfo.maxPosition.toFixed(2)}</div>
          </div>
        </div>
        <div className="mt-2 bg-gray-800 rounded h-2">
          <div
            className="bg-blue-500 h-full rounded"
            style={{ width: `${Math.min(riskInfo.positionPercent, 100)}%` }}
          />
        </div>
      </div>

      {/* Liquidation Info */}
      <div className="border-b border-gray-700 pb-3">
        <h3 className="text-sm font-medium text-gray-400 flex items-center gap-2 mb-2">
          <AlertTriangle size={14} />
          {t('liquidationInfo')}
        </h3>
        <div className="grid grid-cols-2 gap-2 text-sm">
          <div>
            <div className="text-gray-500">{t('liquidationPrice')}</div>
            <div className="text-red-400">${riskInfo.liquidationPrice.toFixed(2)}</div>
          </div>
          <div>
            <div className="text-gray-500">{t('liquidationDistance')}</div>
            <div className="text-white">{riskInfo.liquidationDistance.toFixed(1)}%</div>
          </div>
        </div>
      </div>

      {/* Market State */}
      <div className="border-b border-gray-700 pb-3">
        <h3 className="text-sm font-medium text-gray-400 flex items-center gap-2 mb-2">
          <Box size={14} />
          {t('marketState')}
        </h3>
        <div className="flex items-center gap-4">
          <div>
            <div className="text-gray-500 text-sm">{t('regimeLevel')}</div>
            <div className={`font-medium ${getRegimeColor(riskInfo.regimeLevel)}`}>
              {t(riskInfo.regimeLevel)}
            </div>
          </div>
          {riskInfo.breakoutLevel !== 'none' && (
            <div className="text-red-400">
              {t('breakout')}: {riskInfo.breakoutLevel} ({riskInfo.breakoutDirection})
            </div>
          )}
        </div>
      </div>

      {/* Box State */}
      <div>
        <h3 className="text-sm font-medium text-gray-400 mb-2">{t('boxState')}</h3>
        <div className="text-xs space-y-1">
          <div className="flex justify-between">
            <span className="text-gray-500">{t('shortBox')}</span>
            <span className="text-white">{riskInfo.shortBoxLower.toFixed(2)} - {riskInfo.shortBoxUpper.toFixed(2)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-500">{t('midBox')}</span>
            <span className="text-white">{riskInfo.midBoxLower.toFixed(2)} - {riskInfo.midBoxUpper.toFixed(2)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-gray-500">{t('longBox')}</span>
            <span className="text-white">{riskInfo.longBoxLower.toFixed(2)} - {riskInfo.longBoxUpper.toFixed(2)}</span>
          </div>
          <div className="flex justify-between font-medium">
            <span className="text-gray-400">Current Price</span>
            <span className="text-yellow-400">${riskInfo.currentPrice.toFixed(2)}</span>
          </div>
        </div>
      </div>
    </div>
  )
}
```

**Step 2: Commit**

```bash
git add web/src/components/strategy/GridRiskPanel.tsx
git commit -m "feat(web): add GridRiskPanel component"
```

---

## Task 17: Update AI Prompt with Box Indicators

**Files:**
- Modify: `kernel/grid_engine.go`

**Step 1: Update BuildGridUserPrompt to include box data**

Add box data section to the prompt in `kernel/grid_engine.go`:

```go
// In BuildGridUserPrompt function, add after market data section:

	// Box Indicator Section
	if gridCtx.BoxData != nil {
		sb.WriteString("\n## Box Indicators (Donchian Channels)\n\n")
		sb.WriteString("| Box Level | Upper | Lower | Width |\n")
		sb.WriteString("|-----------|-------|-------|-------|\n")

		shortWidth := (gridCtx.BoxData.ShortUpper - gridCtx.BoxData.ShortLower) / gridCtx.BoxData.CurrentPrice * 100
		midWidth := (gridCtx.BoxData.MidUpper - gridCtx.BoxData.MidLower) / gridCtx.BoxData.CurrentPrice * 100
		longWidth := (gridCtx.BoxData.LongUpper - gridCtx.BoxData.LongLower) / gridCtx.BoxData.CurrentPrice * 100

		sb.WriteString(fmt.Sprintf("| Short (3d) | %.2f | %.2f | %.2f%% |\n",
			gridCtx.BoxData.ShortUpper, gridCtx.BoxData.ShortLower, shortWidth))
		sb.WriteString(fmt.Sprintf("| Mid (10d) | %.2f | %.2f | %.2f%% |\n",
			gridCtx.BoxData.MidUpper, gridCtx.BoxData.MidLower, midWidth))
		sb.WriteString(fmt.Sprintf("| Long (21d) | %.2f | %.2f | %.2f%% |\n",
			gridCtx.BoxData.LongUpper, gridCtx.BoxData.LongLower, longWidth))

		// Price position
		sb.WriteString(fmt.Sprintf("\nCurrent Price: %.2f\n", gridCtx.BoxData.CurrentPrice))

		// Check position relative to boxes
		price := gridCtx.BoxData.CurrentPrice
		if price > gridCtx.BoxData.LongUpper || price < gridCtx.BoxData.LongLower {
			sb.WriteString("⚠️ BREAKOUT: Price outside long-term box!\n")
		} else if price > gridCtx.BoxData.MidUpper || price < gridCtx.BoxData.MidLower {
			sb.WriteString("⚠️ WARNING: Price approaching long-term box boundary\n")
		}
	}
```

**Step 2: Update GridContext struct**

Add BoxData field to GridContext:

```go
type GridContext struct {
	// ... existing fields ...

	// Box data
	BoxData *market.BoxData
}
```

**Step 3: Commit**

```bash
git add kernel/grid_engine.go
git commit -m "feat(kernel): add box indicators to AI prompt"
```

---

## Task 18: Database Migration

**Files:**
- Modify: `store/grid.go`

**Step 1: Update InitGridSchema to migrate new fields**

The GORM AutoMigrate will handle adding new columns. Verify by running:

```bash
cd /Users/yida/gopro/open-nofx && go run . migrate
```

**Step 2: Commit**

```bash
git add store/grid.go
git commit -m "chore(store): ensure new grid fields are migrated"
```

---

## Task 19: Run All Tests

**Step 1: Run backend tests**

```bash
cd /Users/yida/gopro/open-nofx && go test -v ./...
```

**Step 2: Run frontend tests (if available)**

```bash
cd /Users/yida/gopro/open-nofx/web && npm test
```

**Step 3: Fix any failing tests and commit**

```bash
git add .
git commit -m "test: fix tests for grid regime implementation"
```

---

## Task 20: Final Integration Test

**Step 1: Start the server**

```bash
cd /Users/yida/gopro/open-nofx && go run .
```

**Step 2: Verify API endpoint**

```bash
curl http://localhost:8080/api/traders/<trader-id>/grid-risk
```

**Step 3: Verify frontend displays risk panel**

Open browser and check grid trading page shows risk panel.

**Step 4: Final commit**

```bash
git add .
git commit -m "feat: complete grid market regime detection implementation"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Donchian calculation | market/data.go |
| 2 | Box data types | market/types.go |
| 3 | GetBoxData function | market/data.go |
| 4 | GridConfigModel fields | store/grid.go |
| 5 | GridInstanceModel fields | store/grid.go |
| 6 | Regime classification | trader/grid_regime.go |
| 7 | Breakout detection | trader/grid_regime.go |
| 8 | Breakout confirmation | trader/grid_regime.go |
| 9 | Breakout handler | trader/grid_regime.go |
| 10 | Grid cycle integration | trader/auto_trader_grid.go |
| 11 | False breakout recovery | trader/auto_trader_grid.go |
| 12 | GridState fields | trader/auto_trader_grid.go |
| 13 | Frontend types | web/src/types.ts |
| 14 | API endpoint | api/server.go |
| 15 | GetGridRiskInfo method | trader/auto_trader_grid.go |
| 16 | GridRiskPanel component | web/src/components/ |
| 17 | AI prompt update | kernel/grid_engine.go |
| 18 | Database migration | store/grid.go |
| 19 | Run all tests | - |
| 20 | Integration test | - |
