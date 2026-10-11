package kernel

// ============================================================================
// Trading Data Schema
// ============================================================================
// Bilingual data dictionary supporting Chinese and English.
// Ensures AI can fully understand data formats regardless of language.
// ============================================================================

const (
	SchemaVersion = "1.0.0"
)

// Language represents the language type
type Language string

const (
	LangChinese Language = "zh-CN"
	LangEnglish Language = "en-US"
)

// ========== Bilingual Field Definitions ==========

// BilingualFieldDef defines a field with bilingual name, formula, and description
type BilingualFieldDef struct {
	NameZH    string // Chinese name
	NameEN    string // English name
	Unit      string // unit of measurement
	FormulaZH string // Chinese formula
	FormulaEN string // English formula
	DescZH    string // Chinese description
	DescEN    string // English description
}

// GetName returns the field name based on language
func (d BilingualFieldDef) GetName(lang Language) string {
	if lang == LangChinese {
		return d.NameZH
	}
	return d.NameEN
}

// GetFormula returns the formula based on language
func (d BilingualFieldDef) GetFormula(lang Language) string {
	if lang == LangChinese {
		return d.FormulaZH
	}
	return d.FormulaEN
}

// GetDesc returns the description based on language
func (d BilingualFieldDef) GetDesc(lang Language) string {
	if lang == LangChinese {
		return d.DescZH
	}
	return d.DescEN
}

// ========== Data Dictionary ==========

// DataDictionary defines the meaning of all fields
var DataDictionary = map[string]map[string]BilingualFieldDef{
	"AccountMetrics": {
		"Equity": {
			NameZH:    "总权益",
			NameEN:    "Total Equity",
			Unit:      "USDT",
			FormulaZH: "钱包余额 + 未实现盈亏（= Balance + 已用保证金）",
			FormulaEN: "Wallet Balance + Unrealized PnL (= Balance + Used Margin)",
			DescZH:    "账户的实际净值，包含所有持仓的浮动盈亏",
			DescEN:    "Actual account value including all unrealized P&L from positions",
		},
		"Balance": {
			NameZH:    "可用余额",
			NameEN:    "Available Balance",
			Unit:      "USDT",
			FormulaZH: "总权益 - 已用保证金（含未实现盈亏；本系统策略口径，≠ 交易所 availableBalance）",
			FormulaEN: "Total Equity - Used Margin (includes unrealized PnL; this system's strategy convention, NOT the exchange availableBalance)",
			DescZH:    "用于策略仓位与保证金校验的可用额（user prompt 的 Account 行显示为 Balance）。交易所 availableBalance 还预留了挂单保证金与维持保证金，数值不同",
			DescEN:    "Amount used by strategy position and margin checks (rendered as Balance in the Account line). The exchange availableBalance additionally reserves order and maintenance margin and differs",
		},
		"PnL": {
			NameZH:    "总盈亏百分比",
			NameEN:    "Total PnL Percentage",
			Unit:      "%",
			FormulaZH: "(总权益 - 初始资金) / 初始资金 × 100",
			FormulaEN: "(Total Equity - Initial Capital) / Initial Capital × 100",
			DescZH:    "自系统启动以来的总收益率，+15.87%表示盈利15.87%",
			DescEN:    "Total return since inception, +15.87% means 15.87% profit",
		},
		"MarginUsage": {
			NameZH:    "保证金使用率",
			NameEN:    "Margin Usage Rate",
			Unit:      "%",
			FormulaZH: "已用保证金合计 / 总权益 × 100",
			FormulaEN: "Total Used Margin / Total Equity × 100",
			DescZH:    "该值越高，账户风险越大。安全值<30%，危险值>70%",
			DescEN:    "Higher value = higher risk. Safe <30%, Dangerous >70%",
		},
	},

	"TradeMetrics": {
		"Entry": {
			NameZH: "进场价",
			NameEN: "Entry Price",
			Unit:   "USDT",
			DescZH: "开仓时的平均价格",
			DescEN: "Average price when opening position",
		},
		"Exit": {
			NameZH: "出场价",
			NameEN: "Exit Price",
			Unit:   "USDT",
			DescZH: "平仓时的平均价格",
			DescEN: "Average price when closing position",
		},
		"Profit": {
			NameZH:    "已实现盈亏",
			NameEN:    "Realized PnL",
			Unit:      "USDT",
			FormulaZH: "方向因子 × (出场价 - 进场价) × 数量 - 手续费（多=+1，空=-1；资金费未摊入单笔）",
			FormulaEN: "side_factor × (Exit - Entry) × Quantity - Fees (long=+1, short=-1; funding not attributed per trade)",
			DescZH:    "已平仓交易的净盈亏（已扣手续费；资金费不按单笔归集）。正值=盈利，负值=亏损",
			DescEN:    "Net profit/loss of closed trades after fees; funding is not attributed per trade. Positive=profit, Negative=loss",
		},
		"PnL%": {
			NameZH:    "盈亏百分比",
			NameEN:    "PnL Percentage",
			Unit:      "%",
			FormulaZH: "方向因子 × (出场价 - 进场价) / 进场价 × 100（纯价格回报，不含杠杆）",
			FormulaEN: "side_factor × (Exit - Entry) / Entry × 100 (price return, no leverage)",
			DescZH:    "已平仓交易的收益率，+6.71%表示盈利6.71%",
			DescEN:    "Return on closed trade, +6.71% means 6.71% profit",
		},
		"HoldDuration": {
			NameZH: "持仓时长",
			NameEN: "Holding Duration",
			Unit:   "minutes",
			DescZH: "从开仓到平仓的时间。<15分钟=超短线，15分钟-4小时=日内，>4小时=波段",
			DescEN: "Time from open to close. <15min=scalping, 15min-4h=intraday, >4h=swing",
		},
	},

	"PositionMetrics": {
		"UnrealizedPnL%": {
			NameZH:    "未实现盈亏百分比",
			NameEN:    "Unrealized PnL Percentage",
			Unit:      "%",
			FormulaZH: "方向因子 × (当前价 - 进场价) / 进场价 × 100（纯价格回报；保证金口径 ROI 约为该值 × 杠杆）",
			FormulaEN: "side_factor × (Current - Entry) / Entry × 100 (price return; margin-basis ROI is approximately this × leverage)",
			DescZH:    "当前持仓的浮动盈亏，未平仓前是浮动的",
			DescEN:    "Floating P&L of current position, not realized until closed",
		},
		"PeakPnL%": {
			NameZH: "峰值盈亏百分比",
			NameEN: "Peak PnL Percentage",
			Unit:   "%",
			DescZH: "该持仓曾经达到的最高未实现盈亏。口径为保证金（×杠杆），与价格口径的 UnrealizedPnL% 相差杠杆倍数，两者不可直接比较。用于判断是否需要止盈",
			DescEN: "Historical max unrealized PnL for this position. Margin-basis (leveraged) — differs from the price-basis UnrealizedPnL% by the leverage factor; do not compare directly. Used for take-profit decisions",
		},
		"Leverage": {
			NameZH: "杠杆倍数",
			NameEN: "Leverage",
			Unit:   "x",
			DescZH: "3x时价格变动1%，保证金口径ROI约变3%（未计费用）。杠杆越高，风险越大",
			DescEN: "At 3x, a 1% price move changes margin-basis ROI by about 3% before costs. Higher leverage increases risk",
		},
		"MarginUsed": {
			NameZH:    "占用保证金",
			NameEN:    "Margin Used",
			Unit:      "USDT",
			FormulaZH: "开仓名义价值 / 杠杆（开仓时锁定，不随现价变动）",
			FormulaEN: "Initial Notional / Leverage (locked at entry, does not track mark price)",
			DescZH:    "该仓位锁定的保证金金额",
			DescEN:    "Collateral locked for this position",
		},
		"LiqPrice": {
			NameZH: "强平价格",
			NameEN: "Liquidation Price",
			Unit:   "USDT",
			DescZH: "价格触及此值时可能被强制平仓。0 表示未知或不适用，不得解释为没有强平风险",
			DescEN: "Price at which the position may be force-closed. Zero means unknown or not applicable, not risk-free",
		},
		"AutomationStage": {
			NameZH: "自动管理阶段",
			NameEN: "Automation Stage",
			Unit:   "enum",
			DescZH: "程序当前已执行的仓位管理阶段：NONE、BREAKEVEN_ARMED、R_LOCK_TRIMMED、ROE_TRIMMED 或 REDUCED_PRIOR_OR_EXTERNAL；避免重复保本或减仓",
			DescEN: "Program-owned management stage: NONE, BREAKEVEN_ARMED, R_LOCK_TRIMMED, ROE_TRIMMED, or REDUCED_PRIOR_OR_EXTERNAL; prevents duplicate management actions",
		},
		"CumulativeReducedPct": {
			NameZH:    "累计减仓比例",
			NameEN:    "Cumulative Reduction",
			Unit:      "%",
			FormulaZH: "(初始数量 - 当前数量) / 初始数量 × 100",
			FormulaEN: "(Initial Quantity - Current Quantity) / Initial Quantity × 100",
			DescZH:    "相对初始仓位已经平掉的总比例，包含程序阶梯、结构止盈、LLM 部分平仓及外部减仓",
			DescEN:    "Total share of original size already closed, including program ladders, structural TP, LLM partial closes, and external reductions",
		},
	},

	"MarketData": {
		"OI": {
			NameZH: "持仓量",
			NameEN: "Open Interest",
			Unit:   "USDT",
			DescZH: "未平仓合约的总量或名义价值。增加表示未平仓合约扩张，减少表示合约收缩；不能单独证明多空方向",
			DescEN: "Total amount or notional value of open contracts. Rising OI means expansion and falling OI contraction; OI alone does not prove direction",
		},
	},
}

// ========== OI Interpretation ==========

// OIInterpretation defines bilingual market interpretations for OI changes
type OIInterpretationType struct {
	OIUp_PriceUp struct {
		ZH string
		EN string
	}
	OIUp_PriceDown struct {
		ZH string
		EN string
	}
	OIDown_PriceUp struct {
		ZH string
		EN string
	}
	OIDown_PriceDown struct {
		ZH string
		EN string
	}
}

var OIInterpretation = OIInterpretationType{
	OIUp_PriceUp: struct {
		ZH string
		EN string
	}{
		ZH: "未平仓合约扩张且价格上涨，偏多推断；仍需结合成交、资金费率与结构确认",
		EN: "Open contracts expand while price rises: a bullish inference that still needs flow, funding, and structure confirmation",
	},
	OIUp_PriceDown: struct {
		ZH string
		EN string
	}{
		ZH: "未平仓合约扩张且价格下跌，偏空推断；仍需结合成交、资金费率与结构确认",
		EN: "Open contracts expand while price falls: a bearish inference that still needs flow, funding, and structure confirmation",
	},
	OIDown_PriceUp: struct {
		ZH string
		EN string
	}{
		ZH: "合约收缩且价格上涨，可能有空头回补；不是确定结论",
		EN: "Contracts shrink while price rises: possible short covering, not a definitive conclusion",
	},
	OIDown_PriceDown: struct {
		ZH string
		EN string
	}{
		ZH: "合约收缩且价格下跌，可能有多头平仓；不是确定结论",
		EN: "Contracts shrink while price falls: possible long liquidation, not a definitive conclusion",
	},
}

// ========== Prompt Generation Functions ==========

// GetSchemaPrompt generates schema description text for AI prompts
func GetSchemaPrompt(lang Language) string {
	if lang == LangChinese {
		return getSchemaPromptZH()
	}
	return getSchemaPromptEN()
}

// getSchemaPromptZH generates the Chinese prompt
func getSchemaPromptZH() string {
	prompt := "# 📖 数据字典与交易规则\n\n"
	prompt += "## 📊 字段含义说明\n\n"

	// Account metrics
	prompt += "### 账户指标\n"
	for _, key := range []string{"Equity", "Balance", "PnL", "MarginUsage"} {
		prompt += formatFieldDefZH(key, DataDictionary["AccountMetrics"][key])
	}

	// Trade metrics
	prompt += "\n### 交易指标\n"
	for _, key := range []string{"Profit", "PnL%", "HoldDuration", "Entry", "Exit"} {
		prompt += formatFieldDefZH(key, DataDictionary["TradeMetrics"][key])
	}

	// Position metrics
	prompt += "\n### 持仓指标\n"
	for _, key := range []string{"UnrealizedPnL%", "PeakPnL%", "Leverage", "MarginUsed", "LiqPrice", "AutomationStage", "CumulativeReducedPct"} {
		prompt += formatFieldDefZH(key, DataDictionary["PositionMetrics"][key])
	}

	// Market data
	prompt += "\n### 市场数据\n"
	for _, key := range []string{"OI"} {
		prompt += formatFieldDefZH(key, DataDictionary["MarketData"][key])
	}

	// OI interpretation
	prompt += "\n## 💹 持仓量(OI)变化解读\n\n"
	prompt += "- **OI增加 + 价格上涨**: " + OIInterpretation.OIUp_PriceUp.ZH + "\n"
	prompt += "- **OI增加 + 价格下跌**: " + OIInterpretation.OIUp_PriceDown.ZH + "\n"
	prompt += "- **OI减少 + 价格上涨**: " + OIInterpretation.OIDown_PriceUp.ZH + "\n"
	prompt += "- **OI减少 + 价格下跌**: " + OIInterpretation.OIDown_PriceDown.ZH + "\n"

	// Ban-code supplement (user audit 2026-10-01: MIN_SIZE_DEAD_ZONE /
	// STOP_PLAN_OUT_OF_BAND / signed VENDOR_DIVERGENCE had no prompt
	// definition — the model guessed their meaning)
	prompt += "\n## 🚫 阻断码补充词表(hard_blockers/failed 中出现,程序预计算,直接引用)\n\n"
	prompt += "- **MIN_SIZE_DEAD_ZONE**: 按风险公式算出的仓位低于最小下单量,开仓无意义——wait\n"
	prompt += "- **STOP_PLAN_OUT_OF_BAND**: 程序止损计划距离超出 [1.5×ATR(1h), max(2×ATR(4h),8%)] 允许带(过紧或过宽均算,码不区分方向)——wait,不要自行改止损凑带\n"
	prompt += "- **STOP_PLAN_NO_STRUCTURE**: 对侧没有可用结构位生成止损计划——wait\n"
	prompt += "- **VENDOR_DIVERGENCE_x.xx**: x.xx 为带符号偏差,判级取绝对值(|偏差| > 1.0% 即拦)——wait\n"
	prompt += "- **EMA20_STRETCH_x_GT_y**: 现价高于 4h EMA20 超过 y%(当前 x%)——追强势被程序禁止;突破确认的币会给出 long_pullback 回踩锚,等回踩而非追\n"
	prompt += "- **CHASE_EMA20_1H_x_GT_y**: 入场价在交易方向上高于(多)/低于(空) 1h EMA20 超过 y%(当前 x%)——追涨杀跌被程序禁止,等回到 EMA20 附近的锚点\n"
	prompt += "- **COUNTER_EMA20_1H_x_GT_y**: 入场价在 1h EMA20 的逆向一侧超过 y%(多=低于,空=高于;当前 x%)——接飞刀/逆势抄顶被程序禁止\n"
	prompt += "- **RSI1H_EXHAUSTED_x**: 1h RSI(当前 x)在交易方向上已极端(多≥上限,空≤100-上限)——动能耗尽,不追\n"
	prompt += "- **CHASE_MOVE4H_x_GT_y**: 近 4h 价格已在交易方向上移动超过 y%(当前 x%)——不追已走完的一段\n"
	prompt += "- **REGIME_LINE_BROKEN**: pullback 做多 / rally 做空时,现价已跌/升破 trend_tf 的 EMA50——可能反转,wait\n"
	prompt += "- **BTC_4H_DOWNTREND**: BTC 4h 处于下跌趋势(EMA20<EMA50 且价在 EMA20 下)——山寨多头整体暂停\n"
	prompt += "- **BTC_WEAK_LONG_x_VS_y**: 该币 24h 涨幅(x%)弱于 BTC(y%)——不接弱势跟跌的多头\n"
	prompt += "- **BTC_4H_STRONGBULL**: BTC 4h 强势上行(EMA20>EMA50、价在 EMA20 上、RSI≥60)且策略开启了 btc_filter_short——山寨空头整体暂停(扫描层同源分类器已对空头 ×0.85,此为硬停开关)\n"
	prompt += "- **BTC_REGIME_UNKNOWN**: 策略开启了 btc_filter_short 但 BTC 4h 数据不足(无法证明「非强牛」)——按不满足处理,空头等待;数据恢复后自动重评\n"
	prompt += "- **SHORT_TOP_CONFIRM_MISSING**: short_scan 候选缺少顶部确认(顶背离/假突破/费率回落(derivatives.funding_rollover.detected);破EMA20不算确认)——策略默认姿态的程序强制;等确认打印或 15m 转向后 RECHECK;无 short_scan 证据的候选不受此码约束\n"
	prompt += "- **WIDE_STOP_x_GT_y**: 止损计划距离 x% 超过上限 y%——币的波动结构不配 2% 风险的仓位几何,直接放弃该标的\n"

	return prompt
}

// getSchemaPromptEN generates the English prompt
func getSchemaPromptEN() string {
	prompt := "# 📖 Data Dictionary & Trading Rules\n\n"
	prompt += "## 📊 Field Definitions\n\n"

	// Account Metrics
	prompt += "### Account Metrics\n"
	for _, key := range []string{"Equity", "Balance", "PnL", "MarginUsage"} {
		prompt += formatFieldDefEN(key, DataDictionary["AccountMetrics"][key])
	}

	// Trade Metrics
	prompt += "\n### Trade Metrics\n"
	for _, key := range []string{"Profit", "PnL%", "HoldDuration", "Entry", "Exit"} {
		prompt += formatFieldDefEN(key, DataDictionary["TradeMetrics"][key])
	}

	// Position Metrics
	prompt += "\n### Position Metrics\n"
	for _, key := range []string{"UnrealizedPnL%", "PeakPnL%", "Leverage", "MarginUsed", "LiqPrice", "AutomationStage", "CumulativeReducedPct"} {
		prompt += formatFieldDefEN(key, DataDictionary["PositionMetrics"][key])
	}

	// Market Data
	prompt += "\n### Market Data\n"
	for _, key := range []string{"OI"} {
		prompt += formatFieldDefEN(key, DataDictionary["MarketData"][key])
	}

	// OI Interpretation
	prompt += "\n## 💹 Open Interest (OI) Change Interpretation\n\n"
	prompt += "- **OI Up + Price Up**: " + OIInterpretation.OIUp_PriceUp.EN + "\n"
	prompt += "- **OI Up + Price Down**: " + OIInterpretation.OIUp_PriceDown.EN + "\n"
	prompt += "- **OI Down + Price Up**: " + OIInterpretation.OIDown_PriceUp.EN + "\n"
	prompt += "- **OI Down + Price Down**: " + OIInterpretation.OIDown_PriceDown.EN + "\n"

	// Ban-code supplement — keep in sync with the ZH version above.
	prompt += "\n## 🚫 Ban-Code Supplement (in hard_blockers/failed; program-computed, cite verbatim)\n\n"
	prompt += "- **MIN_SIZE_DEAD_ZONE**: the risk-formula position size falls below the minimum order size — wait\n"
	prompt += "- **STOP_PLAN_OUT_OF_BAND**: the stop-plan distance sits outside [1.5×ATR(1h), max(2×ATR(4h),8%)] (either too tight or too wide; the code does not distinguish) — wait, never move the stop to fit the band\n"
	prompt += "- **STOP_PLAN_NO_STRUCTURE**: no usable opposite-side structure to build a stop plan — wait\n"
	prompt += "- **VENDOR_DIVERGENCE_x.xx**: x.xx is a SIGNED deviation; grading uses its absolute value (|deviation| > 1.0% blocks) — wait\n"
	prompt += "- **EMA20_STRETCH_x_GT_y**: price is x% above the 4h EMA20 (cap y%) — chasing strength is program-blocked; breakout-confirmed coins carry a long_pullback retest anchor instead of a chase price\n"
	prompt += "- **CHASE_EMA20_1H_x_GT_y**: entry price is x% beyond the 1h EMA20 in the trade direction (cap y%) — chasing is program-blocked; wait for an anchor near the EMA20\n"
	prompt += "- **COUNTER_EMA20_1H_x_GT_y**: entry price is x% on the WRONG side of the 1h EMA20 (cap y%; long = below, short = above) — knife-catching / fading a pump is program-blocked\n"
	prompt += "- **RSI1H_EXHAUSTED_x**: 1h RSI (x) is already extreme in the trade direction (long >= cap, short <= 100-cap) — momentum exhausted, do not chase\n"
	prompt += "- **CHASE_MOVE4H_x_GT_y**: price already moved x% over the last 4h in the trade direction (cap y%) — do not chase a finished leg\n"
	prompt += "- **REGIME_LINE_BROKEN**: pullback-long / rally-short live price has crossed the trend_tf EMA50 — possible reversal, wait\n"
	prompt += "- **BTC_4H_DOWNTREND**: BTC 4h is in a downtrend (EMA20<EMA50 and price below EMA20) — altcoin longs pause market-wide\n"
	prompt += "- **BTC_WEAK_LONG_x_VS_y**: the coin's 24h return (x%) lags BTC's (y%) — no longs on weak laggards\n"
	prompt += "- **BTC_4H_STRONGBULL**: BTC 4h is in a strong uptrend (EMA20>EMA50, price above, RSI≥60) AND btc_filter_short is enabled — altcoin shorts pause market-wide (the scan layer already haircuts shorts ×0.85 off the same shared classifier; this is the hard pause switch)\n"
	prompt += "- **BTC_REGIME_UNKNOWN**: btc_filter_short is enabled but BTC 4h data is insufficient (cannot demonstrate \"not strong bull\") — treated as not satisfied, shorts wait; re-evaluated automatically once data recovers\n"
	prompt += "- **SHORT_TOP_CONFIRM_MISSING**: a short_scan candidate lacks a topping confirmation (bearish divergence / fake breakout / live derivatives.funding_rollover.detected; an EMA20 break alone does NOT count) — the program-enforced default stance; wait for a confirmation to print or the 15m trend to turn, then RECHECK; candidates WITHOUT short_scan evidence are not subject to this code\n"
	prompt += "- **WIDE_STOP_x_GT_y**: the stop-plan distance (x%) exceeds the cap (y%) — the coin's volatility structure doesn't fit 2%-risk sizing geometry; skip the symbol entirely\n"

	return prompt
}

// formatFieldDefZH formats a field definition in Chinese
func formatFieldDefZH(key string, field BilingualFieldDef) string {
	result := "- **" + key + "**（" + field.NameZH + "）: " + field.DescZH
	if field.FormulaZH != "" {
		result += " | 公式: `" + field.FormulaZH + "`"
	}
	if field.Unit != "" {
		result += " | 单位: " + field.Unit
	}
	result += "\n"
	return result
}

// formatFieldDefEN formats a field definition in English
func formatFieldDefEN(key string, field BilingualFieldDef) string {
	result := "- **" + key + "** (" + field.NameEN + "): " + field.DescEN
	if field.FormulaEN != "" {
		result += " | Formula: `" + field.FormulaEN + "`"
	}
	if field.Unit != "" {
		result += " | Unit: " + field.Unit
	}
	result += "\n"
	return result
}
