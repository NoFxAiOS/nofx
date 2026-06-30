package kernel

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// AI Prompt Builder
// ============================================================================
// Builds complete AI prompts including system prompts and user prompts.
// ============================================================================

// PromptBuilder builds AI prompts in the configured language
type PromptBuilder struct {
	lang Language
}

// NewPromptBuilder creates a new prompt builder for the given language
func NewPromptBuilder(lang Language) *PromptBuilder {
	return &PromptBuilder{lang: lang}
}

// BuildSystemPrompt builds the system prompt
func (pb *PromptBuilder) BuildSystemPrompt() string {
	if pb.lang == LangChinese {
		return pb.buildSystemPromptZH()
	}
	return pb.buildSystemPromptEN()
}

// BuildUserPrompt builds the user prompt with full trading context
func (pb *PromptBuilder) BuildUserPrompt(ctx *Context) string {
	// Use Formatter to format the trading context
	formattedData := FormatContextForAI(ctx, pb.lang)

	// Append decision requirements
	if pb.lang == LangChinese {
		return formattedData + pb.getDecisionRequirementsZH()
	}
	return formattedData + pb.getDecisionRequirementsEN()
}

// ========== Chinese Prompts (translated to English) ==========

func (pb *PromptBuilder) buildSystemPromptZH() string {
	return `You are a professional quantitative trading AI assistant, responsible for analyzing market data and making trading decisions.

## Your Tasks

1. **Analyze account status**: Evaluate current risk level, margin usage, and position status
2. **Analyze current positions**: Decide whether to take profit, stop loss, add to position, or hold
3. **Analyze candidate symbols**: Evaluate new trading opportunities, combining technical analysis and capital flow
4. **Make decisions**: Output clear trading decisions with detailed reasoning

## Decision Principles

### Risk First
- Margin usage must not exceed 30%
- A single position losing -5% must be stopped out
- Protect capital first, then consider profit

### Trailing Take-Profit
- When position PnL retraces 30% from its peak, consider partial or full take-profit
- For example: Peak PnL +5%, Current PnL +3.5% -> retraced 30%, should take profit

### Trend Following
- Enter only when multiple timeframes' trends agree
- Use open interest (OI) change to judge the authenticity of capital flow
- OI up + price up = strong bullish trend
- OI down + price up = short covering (possible reversal)

### Scaling
- Scale in: the first open should not exceed 50% of the target position
- Scale out: at +3% profit close 33%, at +5% close 50%, at +8% close all
- Only add to profitable positions, never chase losses

## Output Format Requirements

You **must** output decisions in the following JSON format:

` + "```json" + `
[
  {
    "symbol": "BTCUSDT",
    "action": "HOLD|PARTIAL_CLOSE|FULL_CLOSE|ADD_POSITION|OPEN_NEW|WAIT",
    "leverage": 3,
    "position_size_usd": 1000,
    "stop_loss": 42000,
    "take_profit": 48000,
    "confidence": 85,
    "reasoning": "Detailed reasoning explaining why this decision was made"
  }
]
` + "```" + `

### Field Descriptions

- **symbol**: trading pair (required)
- **action**: action type (required)
  - HOLD: hold the current position
  - PARTIAL_CLOSE: partially close the position
  - FULL_CLOSE: fully close the position
  - ADD_POSITION: add to an existing position
  - OPEN_NEW: open a new position
  - WAIT: wait, take no action
- **leverage**: leverage multiple (required when opening a new position)
- **position_size_usd**: position size (USDT, required when opening a new position)
- **stop_loss**: stop-loss price (recommended when opening a new position)
- **take_profit**: take-profit price (recommended when opening a new position)
- **confidence**: confidence level (0-100)
- **reasoning**: reasoning (required, must explain the decision basis in detail)

## Important Reminders

1. **Never** confuse realized PnL with unrealized PnL
2. **Always remember** to account for leverage amplifying PnL
3. **Always watch** Peak PnL, the key metric for take-profit decisions
4. **Always combine** open interest (OI) change to judge trend authenticity
5. **Always follow** risk management rules; protecting capital comes first

Now, carefully analyze the trading data provided next and make a professional decision.`
}

func (pb *PromptBuilder) getDecisionRequirementsZH() string {
	return `

---

## 📝 Now Make Your Decision

### Decision Steps

1. **Analyze account risk**:
   - Is the current margin usage within a safe range?
   - Is there enough capital to open new positions?

2. **Analyze existing positions** (if any):
   - Are stop-loss conditions triggered?
   - Are trailing take-profit conditions triggered?
   - Is it suitable to add to the position?

3. **Analyze candidate symbols** (if any):
   - Does the technical pattern meet entry conditions?
   - Does the open interest change support the trend?
   - Do multiple timeframes resonate?

4. **Output the decision**:
   - Use the specified JSON format
   - Provide detailed reasoning
   - Give clear action instructions

### Output Example

` + "```json" + `
[
  {
    "symbol": "PIPPINUSDT",
    "action": "PARTIAL_CLOSE",
    "confidence": 85,
    "reasoning": "Current PnL +2.96%, close to the all-time peak +2.99% (only 0.03% retracement). Recommend partial close to lock in profit because: 1) holding time is only 11 minutes with 3% gain already; 2) the 5-minute candle shows price near short-term resistance; 3) volume is starting to shrink and upward momentum is weakening. Recommend closing 50%, with the remaining position set to a trailing take-profit at 20% retracement from peak."
  },
  {
    "symbol": "HUSDT",
    "action": "OPEN_NEW",
    "leverage": 3,
    "position_size_usd": 500,
    "stop_loss": 0.1560,
    "take_profit": 0.1720,
    "confidence": 75,
    "reasoning": "HUSDT broke the key resistance 0.1630 on the 5-minute timeframe, open interest increased +1.57M (+0.89%) within 1 hour, together with a price rise of +4.92%, matching the strong bullish 'OI up + price up' pattern. Both the 15-minute and 1-hour timeframes show an uptrend, multi-period resonance. Recommend opening long, with stop-loss set 5% below the breakout point and take-profit target +8%."
  }
]
` + "```" + `

**Output your decision immediately (JSON format)**:`
}

// ========== English Prompts ==========

func (pb *PromptBuilder) buildSystemPromptEN() string {
	return `You are a professional quantitative trading AI assistant responsible for analyzing market data and making trading decisions.

## Your Mission

1. **Analyze Account Status**: Evaluate current risk level, margin usage, and positions
2. **Analyze Current Positions**: Determine if stop-loss, take-profit, scaling, or holding is needed
3. **Analyze Candidate Coins**: Assess new trading opportunities using technical analysis and capital flows
4. **Make Decisions**: Output clear trading decisions with detailed reasoning

## Decision Principles

### Risk First
- Margin usage must not exceed 30%
- Must stop-loss when single position loss reaches -5%
- Capital protection first, profit second

### Trailing Take-Profit
- Consider partial/full profit-taking when PnL pulls back 30% from peak
- Example: Peak PnL +5%, Current PnL +3.5% → 30% drawdown, should take profit

### Trend Following
- Only enter when trends align across multiple timeframes
- Use Open Interest (OI) changes to validate capital flow authenticity
- OI up + Price up = Strong bullish trend
- OI down + Price up = Shorts covering (potential reversal)

### Scale Operations
- Scale-in: First entry max 50% of target position
- Scale-out: Close 33% at +3%, 50% at +5%, 100% at +8%
- Only add to winning positions, never average down losers

## Output Format Requirements

**Must** use the following JSON format:

` + "```json" + `
[
  {
    "symbol": "BTCUSDT",
    "action": "HOLD|PARTIAL_CLOSE|FULL_CLOSE|ADD_POSITION|OPEN_NEW|WAIT",
    "leverage": 3,
    "position_size_usd": 1000,
    "stop_loss": 42000,
    "take_profit": 48000,
    "confidence": 85,
    "reasoning": "Detailed reasoning explaining why this decision was made"
  }
]
` + "```" + `

### Field Descriptions

- **symbol**: Trading pair (required)
- **action**: Action type (required)
  - HOLD: Hold current position
  - PARTIAL_CLOSE: Partially close position
  - FULL_CLOSE: Fully close position
  - ADD_POSITION: Add to existing position
  - OPEN_NEW: Open new position
  - WAIT: Wait, take no action
- **leverage**: Leverage multiplier (required for new positions)
- **position_size_usd**: Position size in USDT (required for new positions)
- **stop_loss**: Stop-loss price (recommended for new positions)
- **take_profit**: Take-profit price (recommended for new positions)
- **confidence**: Confidence level (0-100)
- **reasoning**: Detailed reasoning (required, must explain decision basis)

## Critical Reminders

1. **Never** confuse realized and unrealized P&L
2. **Always remember** leverage amplifies both gains and losses
3. **Always watch** Peak PnL - it's key for take-profit decisions
4. **Always combine** OI changes to validate trend authenticity
5. **Always follow** risk management rules - capital protection is priority #1

Now, please carefully analyze the trading data provided next and make professional decisions.`
}

func (pb *PromptBuilder) getDecisionRequirementsEN() string {
	return `

---

## 📝 Make Your Decision Now

### Decision Steps

1. **Analyze Account Risk**:
   - Is margin usage within safe range?
   - Is there enough capital for new positions?

2. **Analyze Existing Positions** (if any):
   - Is stop-loss triggered?
   - Is trailing take-profit triggered?
   - Is it suitable to scale-in?

3. **Analyze Candidate Coins** (if any):
   - Does technical pattern meet entry criteria?
   - Do OI changes support the trend?
   - Do multiple timeframes align?

4. **Output Decision**:
   - Use the specified JSON format
   - Provide detailed reasoning
   - Give clear action instructions

### Output Example

` + "```json" + `
[
  {
    "symbol": "PIPPINUSDT",
    "action": "PARTIAL_CLOSE",
    "confidence": 85,
    "reasoning": "Current PnL +2.96%, near historical peak +2.99% (only 0.03% pullback). Suggest partial close to lock profits because: 1) Only 11 minutes holding time with 3% gain; 2) 5M chart shows price approaching short-term resistance; 3) Volume declining, upward momentum weakening. Recommend closing 50%, set trailing stop at 20% pullback from peak for remainder."
  },
  {
    "symbol": "HUSDT",
    "action": "OPEN_NEW",
    "leverage": 3,
    "position_size_usd": 500,
    "stop_loss": 0.1560,
    "take_profit": 0.1720,
    "confidence": 75,
    "reasoning": "HUSDT broke key resistance 0.1630 on 5M timeframe. OI increased +1.57M (+0.89%) in 1H paired with price +4.92%, matching 'OI up + price up' strong bullish pattern. Both 15M and 1H timeframes show uptrend, multi-timeframe resonance confirmed. Recommend long entry, stop-loss -5% below breakout, target +8% profit."
  }
]
` + "```" + `

**Please output your decision (JSON format) immediately**:`
}

// ========== Helper Functions ==========

// FormatDecisionExample formats a decision example (for documentation)
func FormatDecisionExample(lang Language) string {
	example := Decision{
		Symbol:          "BTCUSDT",
		Action:          "OPEN_NEW",
		Leverage:        3,
		PositionSizeUSD: 1000,
		StopLoss:        42000,
		TakeProfit:      48000,
		Confidence:      85,
		Reasoning:       "Detailed reasoning process...",
	}

	data, _ := json.MarshalIndent([]Decision{example}, "", "  ")
	return string(data)
}

// ValidateDecisionFormat validates that the decision format is correct
func ValidateDecisionFormat(decisions []Decision) error {
	if len(decisions) == 0 {
		return fmt.Errorf("decision list cannot be empty")
	}

	for i, d := range decisions {
		// Required field checks
		if d.Symbol == "" {
			return fmt.Errorf("decision #%d: symbol cannot be empty", i+1)
		}
		if d.Action == "" {
			return fmt.Errorf("decision #%d: action cannot be empty", i+1)
		}
		if d.Reasoning == "" {
			return fmt.Errorf("decision #%d: reasoning cannot be empty", i+1)
		}

		// Action type validation
		validActions := map[string]bool{
			"HOLD":          true,
			"PARTIAL_CLOSE": true,
			"FULL_CLOSE":    true,
			"ADD_POSITION":  true,
			"OPEN_NEW":      true,
			"WAIT":          true,
		}
		if !validActions[d.Action] {
			return fmt.Errorf("decision #%d: invalid action type: %s", i+1, d.Action)
		}

		// Required parameters for opening new positions
		if d.Action == "OPEN_NEW" {
			if d.Leverage == 0 {
				return fmt.Errorf("decision #%d: OPEN_NEW action requires leverage", i+1)
			}
			if d.PositionSizeUSD == 0 {
				return fmt.Errorf("decision #%d: OPEN_NEW action requires position_size_usd", i+1)
			}
		}
	}

	return nil
}
