package agent

// i18n message templates
var messages = map[string]map[string]string{
	"help": {
		"zh": `🤖 *NOFXi — 你的 AI 交易 Agent*

*交易：*
/buy BTC 0.01 — 做多（市价单）
/sell BTC 0.01 — 做空
/close BTC — 平仓
/positions — 查看持仓
/balance — 查看余额
/pnl — 盈亏记录

*分析：*
/analyze BTC — AI 市场分析
/watch BTC — 监控价格
/alert BTC above 100000 — 价格提醒
/price BTC — 实时价格

*策略：*
/strategy start BTC 1h — 启动 AI 自动策略
/strategy list — 查看运行中的策略
/strategy stop <id> — 停止策略

*系统：*
/status — Agent 状态
/help — 帮助菜单

直接跟我说话就行，中英文都可以 💬`,

		"en": `🤖 *NOFXi — Your AI Trading Agent*

*Trading:*
/buy BTC 0.01 — Open long (market order)
/sell BTC 0.01 — Open short
/close BTC — Close position
/positions — View open positions
/balance — Check balance
/pnl — P/L history

*Analysis:*
/analyze BTC — AI market analysis
/watch BTC — Monitor price
/alert BTC above 100000 — Price alert
/price BTC — Real-time price

*Strategy:*
/strategy start BTC 1h — Start AI auto strategy
/strategy list — View active strategies
/strategy stop <id> — Stop strategy

*System:*
/status — Agent status
/help — This menu

Just talk to me in any language 💬`,
	},

	"no_trades": {
		"zh": "📭 暂无交易记录。试试 `/buy BTC 0.01` 或 `/analyze BTC` 开始吧。",
		"en": "📭 No trades yet. Start with `/buy BTC 0.01` or `/analyze BTC`.",
	},
	"no_positions": {
		"zh": "📭 当前没有持仓。",
		"en": "📭 No open positions.",
	},
	"recent_trades": {
		"zh": "📋 *最近交易*\n\n",
		"en": "📋 *Recent Trades*\n\n",
	},
	"total_pnl": {
		"zh": "\n💰 总盈亏: $%.2f",
		"en": "\n💰 Total P/L: $%.2f",
	},
	"open_positions": {
		"zh": "📊 *当前持仓*\n\n",
		"en": "📊 *Open Positions*\n\n",
	},
	"total_unrealized": {
		"zh": "💰 *未实现总盈亏: $%.2f*",
		"en": "💰 *Total Unrealized P/L: $%.2f*",
	},
	"account_balance": {
		"zh": "💰 *账户余额*\n\n",
		"en": "💰 *Account Balance*\n\n",
	},
	"balance_total": {
		"zh": "   总额: $%.2f\n",
		"en": "   Total: $%.2f\n",
	},
	"balance_available": {
		"zh": "   可用: $%.2f\n",
		"en": "   Available: $%.2f\n",
	},
	"balance_in_position": {
		"zh": "   持仓占用: $%.2f\n\n",
		"en": "   In Position: $%.2f\n\n",
	},
	"no_exchange": {
		"zh": "⚠️ 还没有配置交易所。请在 config.yaml 的 exchanges 中添加交易所 API Key。",
		"en": "⚠️ No exchange configured. Add exchange API keys in config.yaml.",
	},
	"trade_usage": {
		"zh": "❓ 用法: `/buy BTC 0.01` 或 `/sell ETH 0.5 3x`",
		"en": "❓ Usage: `/buy BTC 0.01` or `/sell ETH 0.5 3x`",
	},
	"invalid_quantity": {
		"zh": "❓ 无效数量: %s",
		"en": "❓ Invalid quantity: %s",
	},
	"specify_quantity": {
		"zh": "❓ 请指定数量: `/buy BTC 0.01`",
		"en": "❓ Please specify quantity: `/buy BTC 0.01`",
	},
	"confirm_trade": {
		"zh": "⚡ *确认交易*\n\n• 操作: %s\n• 交易对: %s\n• 数量: %.6f\n• 杠杆: %dx\n• 交易所: %s\n\n回复 '确认' 执行交易。",
		"en": "⚡ *Confirm Trade*\n\n• Action: %s\n• Symbol: %s\n• Quantity: %.6f\n• Leverage: %dx\n• Exchange: %s\n\nReply 'yes' to execute.",
	},
	"trade_executed": {
		"zh": "✅ *交易已执行！*\n\n• %s %s\n• 数量: %.6f\n• 杠杆: %dx\n• 交易所: %s\n• 结果: %v",
		"en": "✅ *Trade Executed!*\n\n• %s %s\n• Qty: %.6f\n• Leverage: %dx\n• Exchange: %s\n• Result: %v",
	},
	"no_pending": {
		"zh": "没有待确认的交易",
		"en": "no pending trade",
	},
	"analysis_signal": {
		"zh": "🔍 *%s/USDT 分析*\n\n信号: %s\n置信度: %.0f%%\n\n%s",
		"en": "🔍 *%s/USDT Analysis*\n\nSignal: %s\nConfidence: %.0f%%\n\n%s",
	},
	"stop_loss": {
		"zh": "\n\n🛑 止损: $%.2f",
		"en": "\n\n🛑 Stop Loss: $%.2f",
	},
	"take_profit": {
		"zh": "\n🎯 止盈: $%.2f",
		"en": "\n🎯 Take Profit: $%.2f",
	},
	"settings": {
		"zh": "⚙️ *设置*\n\n• 语言: %s\n• 模型: %s\n• 提供商: %s\n• 交易所: %d 个已配置",
		"en": "⚙️ *Settings*\n\n• Language: %s\n• Model: %s\n• Provider: %s\n• Exchanges: %d configured",
	},
	"status_title": {
		"zh": "📊 *NOFXi 状态*\n\n• Agent: %s\n• 模型: %s\n• 提供商: %s\n• 记忆: ✅ 在线\n• 执行: %s\n• 监控: %d 个交易对\n• 时间: %s",
		"en": "📊 *NOFXi Status*\n\n• Agent: %s\n• Model: %s\n• Provider: %s\n• Memory: ✅ Online\n• Execution: %s\n• Watching: %d symbols\n• Time: %s",
	},
	"bridge_connected": {
		"zh": "✅ 已连接",
		"en": "✅ Connected",
	},
	"bridge_disconnected": {
		"zh": "❌ 未连接",
		"en": "❌ Not connected",
	},
	"ai_timeout": {
		"zh": "⏱️ AI 响应超时，请稍后再试。",
		"en": "⏱️ AI response timed out, please try again.",
	},
	"system_prompt": {
		"zh": `你是 NOFXi，一个基于 NOFX 构建的 AI 交易 Agent。

你的能力：
- 市场分析和交易建议
- 实时持仓和余额监控
- 交易执行（开仓/平仓）
- 价格提醒和行情监控
- 风险管理建议

支持多家交易所：Binance、OKX、Bybit、Bitget、KuCoin、Gate、Hyperliquid 等。

简洁、自信、专注交易。使用交易相关的 emoji。
用中文回复。
当前时间: %s`,
		"en": `You are NOFXi, an AI trading agent built on NOFX.

Your capabilities:
- Market analysis and trading recommendations
- Real-time position and balance monitoring
- Trade execution (open/close positions)
- Price alerts and market monitoring
- Risk management advice

Supports multiple exchanges: Binance, OKX, Bybit, Bitget, KuCoin, Gate, Hyperliquid, etc.

Be concise, confident, and action-oriented. Use trading emojis.
Respond in English.
Current time: %s`,
	},
}

// msg returns the localized message for the given key and language.
func msg(lang, key string) string {
	if m, ok := messages[key]; ok {
		if s, ok := m[lang]; ok {
			return s
		}
		if s, ok := m["en"]; ok {
			return s
		}
	}
	return key
}
