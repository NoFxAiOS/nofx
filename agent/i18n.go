package agent

var i18nMessages = map[string]map[string]string{
	"help": {
		"zh": "🤖 *NOFXi — 你的 AI 交易 Agent*\n\n" +
			"*交易:* /buy /sell /long /short + 交易对 数量 杠杆\n" +
			"*查询:* /positions /balance /pnl /traders\n" +
			"*分析:* /analyze BTC\n" +
			"*监控:* /watch BTC · /unwatch BTC\n" +
			"*策略:* /strategy\n" +
			"*系统:* /status /help\n\n" +
			"直接跟我说话就行，中英文都可以 💬",
		"en": "🤖 *NOFXi — Your AI Trading Agent*\n\n" +
			"*Trade:* /buy /sell /long /short + symbol qty leverage\n" +
			"*Query:* /positions /balance /pnl /traders\n" +
			"*Analyze:* /analyze BTC\n" +
			"*Monitor:* /watch BTC · /unwatch BTC\n" +
			"*Strategy:* /strategy\n" +
			"*System:* /status /help\n\n" +
			"Just talk to me in any language 💬",
	},
	"status": {
		"zh": "📊 *NOFXi 状态*\n\n• Traders: %d/%d 运行中\n• 监控: %d 个交易对\n• AI: %s\n• 时间: %s",
		"en": "📊 *NOFXi Status*\n\n• Traders: %d/%d running\n• Watching: %d symbols\n• AI: %s\n• Time: %s",
	},
	"no_traders": {
		"zh": "📭 暂无 Trader。请在 Web UI 中创建和配置。",
		"en": "📭 No traders configured. Create one in Web UI.",
	},
	"no_running_trader": {
		"zh": "⚠️ 没有运行中的 Trader。请在 Web UI 中启动。",
		"en": "⚠️ No running trader. Start one in Web UI.",
	},
	"no_positions": {
		"zh": "📭 当前没有持仓。",
		"en": "📭 No open positions.",
	},
	"positions_header": {
		"zh": "📊 *当前持仓*\n\n",
		"en": "📊 *Open Positions*\n\n",
	},
	"total_pnl": {
		"zh": "💰 *总未实现盈亏: $%.2f*",
		"en": "💰 *Total Unrealized P/L: $%.2f*",
	},
	"balance_header": {
		"zh": "💰 *账户余额*\n\n",
		"en": "💰 *Account Balances*\n\n",
	},
	"traders_header": {
		"zh": "🤖 *Traders*\n\n",
		"en": "🤖 *Traders*\n\n",
	},
	"trade_usage": {
		"zh": "用法: `/buy BTC 0.01` 或 `/sell ETH 0.5 3x`",
		"en": "Usage: `/buy BTC 0.01` or `/sell ETH 0.5 3x`",
	},
	"invalid_qty": {
		"zh": "❓ 无效数量: %s",
		"en": "❓ Invalid quantity: %s",
	},
	"analysis_header": {
		"zh": "🔍 *%s 市场分析*",
		"en": "🔍 *%s Analysis*",
	},
	"sentinel_off": {
		"zh": "⚠️ Sentinel 未启用。",
		"en": "⚠️ Sentinel not enabled.",
	},
	"system_prompt": {
		"zh": "你是 NOFXi，一个专业的 AI 交易 Agent。简洁、专业、用中文回复。使用交易相关 emoji。",
		"en": "You are NOFXi, a professional AI trading agent. Be concise, professional. Use trading emojis.",
	},
}

func (a *Agent) msg(lang, key string) string {
	if m, ok := i18nMessages[key]; ok {
		if s, ok := m[lang]; ok {
			return s
		}
		if s, ok := m["en"]; ok {
			return s
		}
	}
	return key
}
