package kernel

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ============================================================================
// AI Prompt Builder
// ============================================================================
// Builds complete AI prompts including system prompts and user prompts.
// ============================================================================

// PromptBuilder 负责把 system prompt、用户上下文、输出约束拼装成最终给 AI 的输入。
// 它不直接决定交易动作，但强烈影响 AI 的分析边界、输出格式和可控性。
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

// ========== Chinese Prompts ==========

func (pb *PromptBuilder) buildSystemPromptZH() string {
	return `你是一个专业的量化交易AI助手，负责分析市场数据并做出交易决策。

## 你的任务

1. **分析账户状态**: 评估当前风险水平、保证金使用率、持仓情况
2. **分析当前持仓**: 判断是否需要止盈、止损、加仓或持有
3. **分析候选币种**: 评估新的交易机会，结合技术分析和资金流向
4. **做出决策**: 输出明确的交易决策，包含详细的推理过程

## 决策原则

### 风险优先
- 保证金使用率不得超过30%
- 单个持仓亏损达到-5%必须止损
- 优先保护资本，再考虑盈利

### 跟踪止盈
- 当持仓盈亏从峰值回撤30%时，考虑部分或全部止盈
- 例如：Peak PnL +5%，Current PnL +3.5% → 回撤了30%，应该止盈

### 顺势交易
- 只在多个时间框架趋势一致时进场
- 结合持仓量(OI)变化判断资金流向真实性
- OI增加+价格上涨 = 强多头趋势
- OI减少+价格上涨 = 空头平仓（可能反转）

### 分批操作
- 分批建仓：第一次开仓不超过目标仓位的50%
- 分批止盈：盈利3%平33%，盈利5%平50%，盈利8%全平
- 只在盈利仓位上加仓，永远不要追亏损

## 输出格式要求

**必须**使用以下JSON格式输出决策：

` + "```json" + `
[
  {
    "symbol": "BTCUSDT",
    "action": "open_long|open_short|close_long|close_short|hold|wait",
    "leverage": 3,
    "position_size_usd": 1000,
    "protection_plan": {
      "mode": "full|ladder",
      "take_profit_pct": 8,
      "stop_loss_pct": 3,
      "ladder_rules": [
        {
          "take_profit_pct": 3,
          "take_profit_close_ratio_pct": 40,
          "stop_loss_pct": 1.5,
          "stop_loss_close_ratio_pct": 25
        }
      ]
    },
    "confidence": 85,
    "reasoning": "详细的推理过程，说明为什么做出这个决策"
  }
]
` + "```" + `

### 字段说明

- **symbol**: 交易对（必需）
- **action**: 动作类型（必需）
  - open_long: 开新多仓
  - open_short: 开新空仓
  - close_long: 平已有多仓
  - close_short: 平已有空仓
  - hold: 保持当前仓位不变
  - wait: 该币种本轮不操作
- **leverage**: 杠杆倍数（开新仓时必需）
- **position_size_usd**: 仓位大小（USDT，开新仓时必需）
- **stop_loss**: 直接止损价（可选，仅当你不使用 protection_plan 时）
- **take_profit**: 直接止盈价（可选，仅当你不使用 protection_plan 时）
- **protection_plan**: 可选的结构化保护计划。mode=full 时只提供 take_profit_pct/stop_loss_pct（不要写价格字段）；mode=ladder 时提供 ladder_rules
- **confidence**: 信心度（0-100）
- **reasoning**: 推理过程（必需，必须详细说明决策依据）

## 重要提醒

1. **永远不要**混淆已实现盈亏和未实现盈亏
2. **永远记得**考虑杠杆对盈亏的放大作用
3. **永远关注**Peak PnL，这是判断止盈的关键指标
4. **永远结合**持仓量(OI)变化来判断趋势真实性
5. **永远遵守**风险管理规则，保护资本是第一位的

现在，请仔细分析接下来提供的交易数据，并做出专业的决策。`
}

func (pb *PromptBuilder) getDecisionRequirementsZH() string {
	return `

---

## 📝 现在请做出决策

### 决策步骤

1. **分析账户风险**:
   - 当前保证金使用率是否在安全范围？
   - 是否有足够资金开新仓？

2. **分析现有持仓**（如果有）:
   - 是否触发止损条件？
   - 是否触发跟踪止盈条件？
   - 是否适合加仓？

3. **分析候选币种**（如果有）:
   - 技术形态是否符合进场条件？
   - 持仓量变化是否支持趋势？
   - 多个时间框架是否共振？

4. **输出决策**:
   - 使用规定的JSON格式
   - 提供详细的推理过程
   - 给出明确的行动指令

### 输出示例

` + "```json" + `
[
  {
    "symbol": "PIPPINUSDT",
    "action": "close_long",
    "confidence": 85,
    "reasoning": "当前多仓已接近短期阻力且上涨动能减弱，优先锁定已有利润。本动作是平仓，不要附带 protection_plan。"
  },
  {
    "symbol": "BTCUSDT",
    "action": "open_long",
    "leverage": 3,
    "position_size_usd": 500,
    "confidence": 78,
    "protection_plan": {
      "mode": "full",
      "take_profit_pct": 8,
      "stop_loss_pct": 3
    },
    "reasoning": "BTCUSDT 在主趋势方向上完成回踩确认，适合统一 TP/SL 的单段管理，因此使用 full protection_plan，并且用百分比字段表达止盈止损，而不是直接价格。"
  },
  {
    "symbol": "HUSDT",
    "action": "open_long",
    "leverage": 3,
    "position_size_usd": 500,
    "confidence": 75,
    "protection_plan": {
      "mode": "ladder",
      "ladder_rules": [
        {"take_profit_pct": 3, "take_profit_close_ratio_pct": 40, "stop_loss_pct": 1.5, "stop_loss_close_ratio_pct": 25},
        {"take_profit_pct": 6, "take_profit_close_ratio_pct": 60, "stop_loss_pct": 3.0, "stop_loss_close_ratio_pct": 75}
      ]
    },
    "reasoning": "HUSDT 在低周期突破且多周期共振明显，更适合分段止盈止损管理，因此使用 ladder protection_plan。"
  }
]
` + "```" + `

**请立即输出你的决策（JSON格式）**:`
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
    "action": "open_long|open_short|close_long|close_short|hold|wait",
    "leverage": 3,
    "position_size_usd": 1000,
    "protection_plan": {
      "mode": "full|ladder",
      "take_profit_pct": 8,
      "stop_loss_pct": 3,
      "ladder_rules": [
        {
          "take_profit_pct": 3,
          "take_profit_close_ratio_pct": 40,
          "stop_loss_pct": 1.5,
          "stop_loss_close_ratio_pct": 25
        }
      ]
    },
    "confidence": 85,
    "reasoning": "Detailed reasoning explaining why this decision was made"
  }
]
` + "```" + `

### Field Descriptions

- **symbol**: Trading pair (required)
- **action**: Action type (required)
  - open_long: Open new long position
  - open_short: Open new short position
  - close_long: Close an existing long position
  - close_short: Close an existing short position
  - hold: Keep current position unchanged
  - wait: No action for this symbol
- **leverage**: Leverage multiplier (required for open_long/open_short)
- **position_size_usd**: Position size in USDT (required for open_long/open_short)
- **stop_loss**: Stop-loss price (optional direct price, only when you are not using protection_plan)
- **take_profit**: Take-profit price (optional direct price, only when you are not using protection_plan)
- **protection_plan**: Optional structured protection plan. Use mode=full with take_profit_pct/stop_loss_pct only (do not put price fields inside protection_plan), or mode=ladder with ladder_rules.
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
    "action": "close_long",
    "confidence": 85,
    "reasoning": "The existing long is close to short-term resistance and momentum is weakening. Close the long to lock profit. Because this is a close action, do not attach protection_plan."
  },
  {
    "symbol": "BTCUSDT",
    "action": "open_long",
    "leverage": 3,
    "position_size_usd": 500,
    "confidence": 78,
    "protection_plan": {
      "mode": "full",
      "take_profit_pct": 8,
      "stop_loss_pct": 3
    },
    "reasoning": "BTCUSDT completed a pullback confirmation in the primary trend direction, so a single unified TP/SL structure is sufficient and full protection_plan is appropriate. Express TP/SL as percentage fields rather than absolute prices."
  },
  {
    "symbol": "HUSDT",
    "action": "open_long",
    "leverage": 3,
    "position_size_usd": 500,
    "confidence": 75,
    "protection_plan": {
      "mode": "ladder",
      "ladder_rules": [
        {"take_profit_pct": 3, "take_profit_close_ratio_pct": 40, "stop_loss_pct": 1.5, "stop_loss_close_ratio_pct": 25},
        {"take_profit_pct": 6, "take_profit_close_ratio_pct": 60, "stop_loss_pct": 3.0, "stop_loss_close_ratio_pct": 75}
      ]
    },
    "reasoning": "HUSDT shows a low-timeframe breakout with multi-timeframe alignment, so staged TP/SL management is more suitable and ladder protection_plan is preferred."
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
		return nil
	}
	return validateDecisionFormatInternal(decisions, false)
}

func validateDecisionFormatInternal(decisions []Decision, allowEmptyReasoning bool) error {
	for i, d := range decisions {
		if d.Symbol == "" {
			return fmt.Errorf("decision #%d: symbol cannot be empty", i+1)
		}
		if d.Action == "" {
			return fmt.Errorf("decision #%d: action cannot be empty", i+1)
		}
		if d.Reasoning == "" && !allowEmptyReasoning {
			return fmt.Errorf("decision #%d: reasoning cannot be empty", i+1)
		}

		validActions := map[string]bool{
			"open_long":     true,
			"open_short":    true,
			"close_long":    true,
			"close_short":   true,
			"hold":          true,
			"wait":          true,
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

		isOpenAction := d.Action == "open_long" || d.Action == "open_short" || d.Action == "OPEN_NEW"
		if isOpenAction {
			if d.Leverage == 0 {
				return fmt.Errorf("decision #%d: open action requires leverage", i+1)
			}
			if d.PositionSizeUSD == 0 {
				return fmt.Errorf("decision #%d: open action requires position_size_usd", i+1)
			}
		}

		if d.ProtectionPlan != nil {
			if !isOpenAction {
				return fmt.Errorf("decision #%d: protection_plan is only allowed for open actions", i+1)
			}
			switch d.ProtectionPlan.Mode {
			case "full":
				if len(d.ProtectionPlan.LadderRules) > 0 {
					return fmt.Errorf("decision #%d: full protection_plan must not include ladder_rules", i+1)
				}
				if len(d.ProtectionPlan.DrawdownRules) > 0 {
					return fmt.Errorf("decision #%d: full protection_plan must not include drawdown_rules", i+1)
				}
				if d.ProtectionPlan.TakeProfitPct <= 0 && d.ProtectionPlan.StopLossPct <= 0 {
					return fmt.Errorf("decision #%d: full protection_plan requires take_profit_pct or stop_loss_pct", i+1)
				}
			case "ladder":
				if len(d.ProtectionPlan.DrawdownRules) > 0 {
					return fmt.Errorf("decision #%d: ladder protection_plan must not include drawdown_rules", i+1)
				}
				if len(d.ProtectionPlan.LadderRules) == 0 {
					return fmt.Errorf("decision #%d: ladder protection_plan requires ladder_rules", i+1)
				}
				for j, rule := range d.ProtectionPlan.LadderRules {
					if rule.TakeProfitPct <= 0 && rule.StopLossPct <= 0 {
						return fmt.Errorf("decision #%d: ladder_rules[%d] requires take_profit_pct or stop_loss_pct", i+1, j)
					}
					if rule.TakeProfitCloseRatioPct < 0 || rule.TakeProfitCloseRatioPct > 100 {
						return fmt.Errorf("decision #%d: ladder_rules[%d] has invalid take_profit_close_ratio_pct", i+1, j)
					}
					if rule.StopLossCloseRatioPct < 0 || rule.StopLossCloseRatioPct > 100 {
						return fmt.Errorf("decision #%d: ladder_rules[%d] has invalid stop_loss_close_ratio_pct", i+1, j)
					}
					if rule.TakeProfitPct > 0 && rule.TakeProfitCloseRatioPct <= 0 {
						return fmt.Errorf("decision #%d: ladder_rules[%d] take_profit_pct requires positive take_profit_close_ratio_pct", i+1, j)
					}
					if rule.StopLossPct > 0 && rule.StopLossCloseRatioPct <= 0 {
						return fmt.Errorf("decision #%d: ladder_rules[%d] stop_loss_pct requires positive stop_loss_close_ratio_pct", i+1, j)
					}
				}
			case "drawdown":
				if len(d.ProtectionPlan.LadderRules) > 0 || d.ProtectionPlan.TakeProfitPct > 0 || d.ProtectionPlan.StopLossPct > 0 {
					return fmt.Errorf("decision #%d: drawdown protection_plan must only include drawdown_rules", i+1)
				}
				if len(d.ProtectionPlan.DrawdownRules) == 0 {
					return fmt.Errorf("decision #%d: drawdown protection_plan requires drawdown_rules", i+1)
				}
				for j, rule := range d.ProtectionPlan.DrawdownRules {
					if rule.MinProfitPct <= 0 {
						return fmt.Errorf("decision #%d: drawdown_rules[%d] requires positive min_profit_pct", i+1, j)
					}
					if rule.MaxDrawdownPct <= 0 || rule.MaxDrawdownPct > 100 {
						return fmt.Errorf("decision #%d: drawdown_rules[%d] has invalid max_drawdown_pct", i+1, j)
					}
					if rule.CloseRatioPct <= 0 || rule.CloseRatioPct > 100 {
						return fmt.Errorf("decision #%d: drawdown_rules[%d] has invalid close_ratio_pct", i+1, j)
					}
					if rule.PollIntervalSeconds > 0 && rule.PollIntervalSeconds < 5 {
						return fmt.Errorf("decision #%d: drawdown_rules[%d] poll_interval_seconds must be >= 5", i+1, j)
					}
				}
			case "":
				return fmt.Errorf("decision #%d: protection_plan.mode cannot be empty", i+1)
			default:
				return fmt.Errorf("decision #%d: invalid protection_plan.mode: %s", i+1, d.ProtectionPlan.Mode)
			}
		}
	}
	return nil
}

func ValidateDecisionFormatWithCoT(decisions []Decision, cotTrace string) error {
	return validateDecisionFormatInternal(decisions, strings.TrimSpace(cotTrace) != "")
}
