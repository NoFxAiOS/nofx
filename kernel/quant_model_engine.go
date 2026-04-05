package kernel

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"strconv"
	"strings"
)

// ============================================================================
// Quant Model Engine
// ============================================================================

// QuantModelEngine executes quantitative models for trading decisions
type QuantModelEngine struct {
	config      *store.QuantModelConfig
	indicators  *market.IndicatorCalculator
}

// NewQuantModelEngine creates a new quant model engine
func NewQuantModelEngine(config *store.QuantModelConfig) *QuantModelEngine {
	return &QuantModelEngine{
		config:     config,
		indicators: market.NewIndicatorCalculator(),
	}
}

// Execute runs the quant model and returns trading decisions
func (e *QuantModelEngine) Execute(
	data *market.Data,
	position *PositionInfo,
	account AccountInfo,
) (*Decision, error) {
	switch e.config.Type {
	case "indicator_based":
		return e.executeIndicatorBased(data, position, account)
	case "rule_based":
		return e.executeRuleBased(data, position, account)
	case "ensemble":
		return e.executeEnsemble(data, position, account)
	default:
		return nil, fmt.Errorf("unsupported model type: %s", e.config.Type)
	}
}

// ExecuteBatch runs the model on multiple symbols and returns all decisions
func (e *QuantModelEngine) ExecuteBatch(
	marketDataMap map[string]*market.Data,
	positions []PositionInfo,
	account AccountInfo,
) ([]Decision, error) {
	var decisions []Decision

	// Build position lookup for quick access
	positionMap := make(map[string]PositionInfo)
	for _, pos := range positions {
		positionMap[pos.Symbol] = pos
	}

	// Process each symbol
	for symbol, data := range marketDataMap {
		position, hasPosition := positionMap[symbol]
		
		// Skip if we have a position and need to check exit
		if hasPosition {
			// Check exit conditions for existing positions
			exitDecision, err := e.checkExitConditions(data, position, account)
			if err != nil {
				logger.Warnf("⚠️ Failed to check exit for %s: %v", symbol, err)
				continue
			}
			if exitDecision != nil {
				decisions = append(decisions, *exitDecision)
				continue
			}
		}

		// Check entry conditions
		if !hasPosition {
			entryDecision, err := e.checkEntryConditions(data, account)
			if err != nil {
				logger.Warnf("⚠️ Failed to check entry for %s: %v", symbol, err)
				continue
			}
			if entryDecision != nil {
				decisions = append(decisions, *entryDecision)
			}
		}
	}

	return decisions, nil
}

// ============================================================================
// Indicator-Based Model Execution
// ============================================================================

func (e *QuantModelEngine) executeIndicatorBased(
	data *market.Data,
	position *PositionInfo,
	account AccountInfo,
) (*Decision, error) {
	if data == nil || len(data.Klines) == 0 {
		return nil, nil
	}

	// Calculate indicator scores
	scores := make(map[string]float64)
	
	for _, indicator := range e.config.Indicators {
		score := e.calculateIndicatorScore(data, indicator)
		scores[indicator.Name] = score * indicator.Weight
	}

	// Aggregate scores
	totalScore := 0.0
	for _, score := range scores {
		totalScore += score
	}

	// Check if we have an existing position
	if position != nil && position.Symbol == data.Symbol {
		// Check exit conditions
		if e.shouldExitPosition(position, totalScore, account) {
			action := "close_long"
			if position.Side == "short" {
				action = "close_short"
			}
			return &Decision{
				Symbol:     data.Symbol,
				Action:     action,
				Confidence: e.scoreToConfidence(totalScore),
				Reasoning:  fmt.Sprintf("Indicator score %.2f triggered exit threshold", totalScore),
			}, nil
		}
		return nil, nil // Hold position
	}

	// Check entry conditions
	if e.shouldEnterPosition(totalScore, account) {
		action := "open_long"
		if totalScore < 0 {
			action = "open_short"
		}
		
		// Get risk parameters from config
		riskUSD := account.TotalEquity * 0.02 // 2% risk per trade
		
		return &Decision{
			Symbol:     data.Symbol,
			Action:     action,
			Confidence: e.scoreToConfidence(totalScore),
			RiskUSD:    riskUSD,
			Reasoning:  fmt.Sprintf("Indicator score %.2f triggered entry threshold. Signals: %v", totalScore, scores),
		}, nil
	}

	return nil, nil // No action
}

func (e *QuantModelEngine) calculateIndicatorScore(data *market.Data, indicator store.ModelIndicator) float64 {
	switch indicator.Name {
	case "RSI":
		return e.calculateRSIScore(data, indicator.Period)
	case "EMA":
		return e.calculateEMAScore(data, indicator.Period)
	case "MACD":
		return e.calculateMACDScore(data, indicator.Params)
	case "ATR":
		return e.calculateATRScore(data, indicator.Period)
	case "BOLL":
		return e.calculateBollingerScore(data, indicator.Period)
	case "SMA":
		return e.calculateSMAScore(data, indicator.Period)
	default:
		return 0
	}
}

// RSI Score: Oversold (<30) = positive (buy signal), Overbought (>70) = negative (sell signal)
func (e *QuantModelEngine) calculateRSIScore(data *market.Data, period int) float64 {
	klines := data.Klines
	if len(klines) < period+1 {
		return 0
	}

	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	rsi := e.indicators.RSI(closes, period)
	if len(rsi) == 0 {
		return 0
	}

	currentRSI := rsi[len(rsi)-1]
	
	// Normalize RSI (0-100) to score (-1 to 1)
	// RSI < 30: bullish signal, RSI > 70: bearish signal
	if currentRSI < 30 {
		return (30 - currentRSI) / 30 // Positive score for oversold
	} else if currentRSI > 70 {
		return (70 - currentRSI) / 30 // Negative score for overbought
	}
	return 0
}

// EMA Score: Price above EMA = positive (uptrend), below = negative (downtrend)
func (e *QuantModelEngine) calculateEMAScore(data *market.Data, period int) float64 {
	klines := data.Klines
	if len(klines) < period {
		return 0
	}

	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	ema := e.indicators.EMA(closes, period)
	if len(ema) == 0 {
		return 0
	}

	currentPrice := closes[len(closes)-1]
	currentEMA := ema[len(ema)-1]
	
	// Price above EMA = positive (bullish), below = negative (bearish)
	return (currentPrice - currentEMA) / currentEMA
}

// MACD Score: MACD > Signal = positive (bullish), < Signal = negative (bearish)
func (e *QuantModelEngine) calculateMACDScore(data *market.Data, params map[string]interface{}) float64 {
	klines := data.Klines
	if len(klines) < 26 {
		return 0
	}

	// Default MACD parameters
	fast := 12
	slow := 26
	signal := 9

	// Parse custom params
	if p, ok := params["fast"]; ok {
		if v, ok := p.(float64); ok {
			fast = int(v)
		}
	}
	if p, ok := params["slow"]; ok {
		if v, ok := p.(float64); ok {
			slow = int(v)
		}
	}
	if p, ok := params["signal"]; ok {
		if v, ok := p.(float64); ok {
			signal = int(v)
		}
	}

	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	macd, signalLine, _ := e.indicators.MACD(closes, fast, slow, signal)
	if len(macd) == 0 || len(signalLine) == 0 {
		return 0
	}

	currentMACD := macd[len(macd)-1]
	currentSignal := signalLine[len(signalLine)-1]
	
	// MACD above signal = bullish, below = bearish
	return (currentMACD - currentSignal) / currentSignal
}

// ATR Score: High ATR = volatile (reduce position size), Low ATR = stable
func (e *QuantModelEngine) calculateATRScore(data *market.Data, period int) float64 {
	klines := data.Klines
	if len(klines) < period {
		return 0
	}

	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	closes := make([]float64, len(klines))
	
	for i, k := range klines {
		highs[i] = k.High
		lows[i] = k.Low
		closes[i] = k.Close
	}

	atr := e.indicators.ATR(highs, lows, closes, period)
	if len(atr) < 2 {
		return 0
	}

	// Compare current ATR to previous
	currentATR := atr[len(atr)-1]
	previousATR := atr[len(atr)-2]
	
	// Rising ATR = increasing volatility (caution)
	// Falling ATR = decreasing volatility (more stable)
	if previousATR > 0 {
		return (previousATR - currentATR) / previousATR
	}
	return 0
}

// Bollinger Bands Score: Price near lower band = bullish, near upper = bearish
func (e *QuantModelEngine) calculateBollingerScore(data *market.Data, period int) float64 {
	klines := data.Klines
	if len(klines) < period {
		return 0
	}

	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	upper, middle, lower := e.indicators.BOLL(closes, period, 2.0)
	if len(upper) == 0 {
		return 0
	}

	currentPrice := closes[len(closes)-1]
	currentUpper := upper[len(upper)-1]
	currentLower := lower[len(lower)-1]
	
	if currentUpper <= currentLower {
		return 0
	}

	// Price position within bands (0 = lower, 1 = upper)
	position := (currentPrice - currentLower) / (currentUpper - currentLower)
	
	// Invert: near lower band = bullish (positive), near upper = bearish (negative)
	return (0.5 - position) * 2
}

// SMA Score: Price above SMA = bullish, below = bearish
func (e *QuantModelEngine) calculateSMAScore(data *market.Data, period int) float64 {
	klines := data.Klines
	if len(klines) < period {
		return 0
	}

	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	sma := e.indicators.SMA(closes, period)
	if len(sma) == 0 {
		return 0
	}

	currentPrice := closes[len(closes)-1]
	currentSMA := sma[len(sma)-1]
	
	return (currentPrice - currentSMA) / currentSMA
}

// ============================================================================
// Rule-Based Model Execution
// ============================================================================

func (e *QuantModelEngine) executeRuleBased(
	data *market.Data,
	position *PositionInfo,
	account AccountInfo,
) (*Decision, error) {
	if data == nil || len(data.Klines) == 0 {
		return nil, nil
	}

	// Sort rules by priority (highest first)
	rules := make([]store.ModelRule, len(e.config.Rules))
	copy(rules, e.config.Rules)
	
	// Execute rules in priority order
	for _, rule := range rules {
		match, err := e.evaluateRuleCondition(rule.Condition, data)
		if err != nil {
			logger.Debugf("Rule evaluation error for %s: %v", rule.Name, err)
			continue
		}

		if match {
			// Check if this action conflicts with current position
			if position != nil {
				if rule.Action == "buy" && position.Side == "long" {
					continue // Already long
				}
				if rule.Action == "sell" && position.Side == "short" {
					continue // Already short
				}
				// Exit conditions
				if rule.Action == "sell" && position.Side == "long" {
					return &Decision{
						Symbol:     data.Symbol,
						Action:     "close_long",
						Confidence: rule.Confidence,
						Reasoning:  fmt.Sprintf("Rule '%s' matched: %s", rule.Name, rule.Condition),
					}, nil
				}
				if rule.Action == "buy" && position.Side == "short" {
					return &Decision{
						Symbol:     data.Symbol,
						Action:     "close_short",
						Confidence: rule.Confidence,
						Reasoning:  fmt.Sprintf("Rule '%s' matched: %s", rule.Name, rule.Condition),
					}, nil
				}
			}

			// Entry conditions
			if position == nil {
				action := "open_long"
				if rule.Action == "sell" {
					action = "open_short"
				}
				
				return &Decision{
					Symbol:     data.Symbol,
					Action:     action,
					Confidence: rule.Confidence,
					StopLoss:   rule.StopLossPct,
					TakeProfit: rule.TakeProfitPct,
					Reasoning:  fmt.Sprintf("Rule '%s' matched: %s", rule.Name, rule.Condition),
				}, nil
			}
		}
	}

	return nil, nil
}

// evaluateRuleCondition parses and evaluates a simple rule condition
// Supported operators: <, >, <=, >=, =, AND, OR
func (e *QuantModelEngine) evaluateRuleCondition(condition string, data *market.Data) (bool, error) {
	// Simple condition parser - handles basic indicator comparisons
	condition = strings.ToUpper(strings.TrimSpace(condition))
	
	// Handle AND/OR
	if strings.Contains(condition, " AND ") {
		parts := strings.Split(condition, " AND ")
		for _, part := range parts {
			match, err := e.evaluateRuleCondition(strings.TrimSpace(part), data)
			if err != nil || !match {
				return false, err
			}
		}
		return true, nil
	}
	
	if strings.Contains(condition, " OR ") {
		parts := strings.Split(condition, " OR ")
		for _, part := range parts {
			match, err := e.evaluateRuleCondition(strings.TrimSpace(part), data)
			if err == nil && match {
				return true, nil
			}
		}
		return false, nil
	}

	// Extract indicator value and compare
	return e.evaluateSimpleCondition(condition, data)
}

func (e *QuantModelEngine) evaluateSimpleCondition(condition string, data *market.Data) (bool, error) {
	// Parse patterns like "RSI_14 < 30" or "Close > EMA_20"
	
	// Find operator
	operators := []string{"<=", ">=", "<", ">", "="}
	var operator string
	var parts []string
	
	for _, op := range operators {
		if strings.Contains(condition, " "+op+" ") || strings.Contains(condition, op) {
			parts = strings.Split(condition, op)
			if len(parts) == 2 {
				operator = op
				break
			}
		}
	}
	
	if operator == "" || len(parts) != 2 {
		return false, fmt.Errorf("unsupported condition format: %s", condition)
	}

	leftSide := strings.TrimSpace(parts[0])
	rightSide := strings.TrimSpace(parts[1])

	// Get left value (indicator or special keyword)
	leftValue, err := e.getValueFromData(leftSide, data)
	if err != nil {
		return false, err
	}

	// Get right value (number or indicator)
	var rightValue float64
	if val, err := strconv.ParseFloat(rightSide, 64); err == nil {
		rightValue = val
	} else {
		rightValue, err = e.getValueFromData(rightSide, data)
		if err != nil {
			return false, err
		}
	}

	// Compare
	switch operator {
	case "<":
		return leftValue < rightValue, nil
	case ">":
		return leftValue > rightValue, nil
	case "<=":
		return leftValue <= rightValue, nil
	case ">=":
		return leftValue >= rightValue, nil
	case "=":
		return leftValue == rightValue, nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

func (e *QuantModelEngine) getValueFromData(name string, data *market.Data) (float64, error) {
	name = strings.ToUpper(name)
	
	// Special values
	if name == "CLOSE" || name == "PRICE" {
		if len(data.Klines) == 0 {
			return 0, fmt.Errorf("no data available")
		}
		return data.Klines[len(data.Klines)-1].Close, nil
	}
	if name == "VOLUME" {
		if len(data.Klines) == 0 {
			return 0, fmt.Errorf("no data available")
		}
		return data.Klines[len(data.Klines)-1].Volume, nil
	}

	// Indicator patterns: RSI_14, EMA_20, etc.
	if strings.HasPrefix(name, "RSI_") {
		period, _ := strconv.Atoi(strings.TrimPrefix(name, "RSI_"))
		score := e.calculateRSIScore(data, period)
		// Convert score back to RSI value for comparison
		return 50 + score*50, nil
	}
	if strings.HasPrefix(name, "EMA_") {
		period, _ := strconv.Atoi(strings.TrimPrefix(name, "EMA_"))
		klines := data.Klines
		closes := make([]float64, len(klines))
		for i, k := range klines {
			closes[i] = k.Close
		}
		ema := e.indicators.EMA(closes, period)
		if len(ema) > 0 {
			return ema[len(ema)-1], nil
		}
	}
	if strings.HasPrefix(name, "SMA_VOLUME_") {
		period, _ := strconv.Atoi(strings.TrimPrefix(name, "SMA_VOLUME_"))
		klines := data.Klines
		volumes := make([]float64, len(klines))
		for i, k := range klines {
			volumes[i] = k.Volume
		}
		sma := e.indicators.SMA(volumes, period)
		if len(sma) > 0 {
			return sma[len(sma)-1], nil
		}
	}

	return 0, fmt.Errorf("unknown value reference: %s", name)
}

// ============================================================================
// Ensemble Model Execution
// ============================================================================

func (e *QuantModelEngine) executeEnsemble(
	data *market.Data,
	position *PositionInfo,
	account AccountInfo,
) (*Decision, error) {
	// For now, ensemble uses weighted voting of configured indicators
	// In full implementation, this would integrate multiple sub-models
	return e.executeIndicatorBased(data, position, account)
}

// ============================================================================
// Helper Methods
// ============================================================================

func (e *QuantModelEngine) shouldEnterPosition(score float64, account AccountInfo) bool {
	params := e.config.Parameters
	
	// Check entry threshold
	threshold := params.EntryThreshold
	if threshold == 0 {
		threshold = 0.5 // Default
	}
	
	// Score must exceed threshold (positive for long, negative for short)
	if score > 0 && score >= threshold {
		return true
	}
	if score < 0 && -score >= threshold {
		return true
	}
	
	return false
}

func (e *QuantModelEngine) shouldExitPosition(position *PositionInfo, score float64, account AccountInfo) bool {
	params := e.config.Parameters
	
	threshold := params.ExitThreshold
	if threshold == 0 {
		threshold = 0.3 // Default
	}
	
	// Exit if score contradicts position direction
	if position.Side == "long" && score < -threshold {
		return true
	}
	if position.Side == "short" && score > threshold {
		return true
	}
	
	return false
}

func (e *QuantModelEngine) checkEntryConditions(data *market.Data, account AccountInfo) (*Decision, error) {
	return e.executeIndicatorBased(data, nil, account)
}

func (e *QuantModelEngine) checkExitConditions(data *market.Data, position PositionInfo, account AccountInfo) (*Decision, error) {
	decision, err := e.executeIndicatorBased(data, &position, account)
	if err != nil {
		return nil, err
	}
	
	// Only return if it's an exit action
	if decision != nil && (decision.Action == "close_long" || decision.Action == "close_short") {
		return decision, nil
	}
	
	return nil, nil
}

func (e *QuantModelEngine) scoreToConfidence(score float64) int {
	// Convert score (-1 to 1) to confidence (0-100)
	absScore := score
	if absScore < 0 {
		absScore = -absScore
	}
	
	confidence := int(absScore * 100)
	if confidence < e.config.SignalConfig.MinConfidence {
		confidence = e.config.SignalConfig.MinConfidence
	}
	if confidence > 100 {
		confidence = 100
	}
	
	return confidence
}

// BacktestResult represents the result of a backtest run
type BacktestResult struct {
	TotalTrades    int       `json:"total_trades"`
	WinningTrades  int       `json:"winning_trades"`
	LosingTrades   int       `json:"losing_trades"`
	WinRate        float64   `json:"win_rate"`
	TotalReturnPct float64   `json:"total_return_pct"`
	MaxDrawdownPct float64   `json:"max_drawdown_pct"`
	AvgProfitPct   float64   `json:"avg_profit_pct"`
	SharpeRatio    float64   `json:"sharpe_ratio"`
	Trades         []BacktestTrade `json:"trades"`
}

// BacktestTrade represents a single trade in backtest
type BacktestTrade struct {
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	EntryTime    string  `json:"entry_time"`
	ExitTime     string  `json:"exit_time"`
	EntryPrice   float64 `json:"entry_price"`
	ExitPrice    float64 `json:"exit_price"`
	PnLPct       float64 `json:"pnl_pct"`
	ExitReason   string  `json:"exit_reason"`
}

// RunBacktest runs a backtest simulation on historical data
func (e *QuantModelEngine) RunBacktest(
	historicalData map[string][]market.Kline,
	initialBalance float64,
) (*BacktestResult, error) {
	// Placeholder for backtest implementation
	// Full implementation would iterate through historical data, simulate trades,
	// and calculate performance metrics
	
	return &BacktestResult{
		TotalTrades:    0,
		WinningTrades:  0,
		LosingTrades:   0,
		WinRate:        0,
		TotalReturnPct: 0,
		MaxDrawdownPct: 0,
		AvgProfitPct:   0,
		SharpeRatio:    0,
		Trades:         []BacktestTrade{},
	}, nil
}

// QuantModelSignal represents a simplified signal output for strategy integration
type QuantModelSignal struct {
	Symbol      string  `json:"symbol"`
	Signal      string  `json:"signal"`       // "buy", "sell", "hold", "close"
	Confidence  int     `json:"confidence"`
	Score       float64 `json:"score"`
	Indicators  map[string]float64 `json:"indicators"`
}

// GetSignal returns a simplified signal for the strategy engine
func (e *QuantModelEngine) GetSignal(data *market.Data) (*QuantModelSignal, error) {
	decision, err := e.executeIndicatorBased(data, nil, AccountInfo{
		TotalEquity: 10000, // Default for signal generation
	})
	if err != nil {
		return nil, err
	}
	
	if decision == nil {
		return &QuantModelSignal{
			Symbol:     data.Symbol,
			Signal:     "hold",
			Confidence: 0,
			Score:      0,
		}, nil
	}
	
	signal := "hold"
	switch decision.Action {
	case "open_long":
		signal = "buy"
	case "open_short":
		signal = "sell"
	case "close_long", "close_short":
		signal = "close"
	}
	
	return &QuantModelSignal{
		Symbol:     decision.Symbol,
		Signal:     signal,
		Confidence: decision.Confidence,
		Score:      0, // Could be calculated from indicator scores
	}, nil
}

// ToJSON serializes the signal to JSON for prompt injection
func (s *QuantModelSignal) ToJSON() string {
	data, _ := json.Marshal(s)
	return string(data)
}