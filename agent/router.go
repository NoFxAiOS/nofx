package agent

import (
	"regexp"
	"strings"
)

type IntentType int

const (
	IntentChat     IntentType = iota
	IntentTrade
	IntentQuery
	IntentAnalyze
	IntentSettings
	IntentHelp
	IntentStatus
	IntentWatch
	IntentStrategy
)

type Intent struct {
	Type   IntentType
	Params map[string]string
	Raw    string
}

type Router struct {
	tradePatterns   []*regexp.Regexp
	queryPatterns   []*regexp.Regexp
	analyzePatterns []*regexp.Regexp
}

func NewRouter() *Router {
	return &Router{
		tradePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(buy|sell|long|short|open|close)\s+(.+)`),
			regexp.MustCompile(`(?i)(做多|做空|买入|卖出|开仓|平仓)\s*(.+)`),
		},
		queryPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(position|balance|pnl|profit|loss|持仓|余额|盈亏|trader|交易员|账户|仓位|资金)`),
			regexp.MustCompile(`(?i)(多少钱|赚了|亏了|收益|回报)`),
		},
		analyzePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(analyze|analysis|分析|看看|研究)\s+(.+)`),
			regexp.MustCompile(`(?i)(what.*think|怎么看|你觉得|走势|趋势|行情)\s*(.+)?`),
			regexp.MustCompile(`(?i)(该不该|适合|能不能|要不要).*(买|卖|做多|做空|入场|开仓).*`),
			regexp.MustCompile(`(?i)(should\s+i|is\s+it\s+good).*(buy|sell|long|short).*`),
		},
	}
}

func (r *Router) Route(text string) Intent {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "/") {
		return r.routeCommand(text)
	}

	for _, p := range r.tradePatterns {
		if m := p.FindStringSubmatch(text); m != nil {
			return Intent{Type: IntentTrade, Params: map[string]string{"action": strings.ToLower(m[1]), "detail": m[2]}, Raw: text}
		}
	}
	for _, p := range r.queryPatterns {
		if p.MatchString(text) {
			return Intent{Type: IntentQuery, Raw: text}
		}
	}
	for _, p := range r.analyzePatterns {
		if m := p.FindStringSubmatch(text); m != nil {
			d := ""
			if len(m) > 2 { d = m[2] }
			return Intent{Type: IntentAnalyze, Params: map[string]string{"detail": d}, Raw: text}
		}
	}

	return Intent{Type: IntentChat, Raw: text}
}

func (r *Router) routeCommand(text string) Intent {
	cmd := strings.ToLower(strings.Fields(text)[0])
	parts := strings.SplitN(text, " ", 2)
	detail := ""
	if len(parts) > 1 { detail = parts[1] }

	switch cmd {
	case "/start", "/help":
		return Intent{Type: IntentHelp, Raw: text}
	case "/status":
		return Intent{Type: IntentStatus, Raw: text}
	case "/buy", "/sell", "/long", "/short", "/open", "/close":
		return Intent{Type: IntentTrade, Params: map[string]string{"action": strings.TrimPrefix(cmd, "/"), "detail": detail}, Raw: text}
	case "/positions", "/balance", "/pnl", "/traders":
		return Intent{Type: IntentQuery, Raw: text}
	case "/analyze":
		return Intent{Type: IntentAnalyze, Params: map[string]string{"detail": detail}, Raw: text}
	case "/watch", "/unwatch":
		return Intent{Type: IntentWatch, Raw: text}
	case "/strategy":
		return Intent{Type: IntentStrategy, Raw: text}
	default:
		return Intent{Type: IntentChat, Raw: text}
	}
}
