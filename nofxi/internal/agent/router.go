package agent

import (
	"regexp"
	"strings"
)

// Intent represents the parsed user intent.
type Intent struct {
	Type   IntentType
	Params map[string]string
	Raw    string
}

// IntentType identifies what the user wants to do.
type IntentType int

const (
	IntentChat        IntentType = iota // General conversation
	IntentTrade                         // Open/close a trade
	IntentQuery                         // Query positions, balance, P/L
	IntentAnalyze                       // Ask for market analysis
	IntentSettings                      // Change preferences
	IntentHelp                          // Help / command list
	IntentStatus                        // Check agent/system status
	IntentWatch                         // Watch symbols, price alerts
	IntentStrategy                      // Start/stop/list strategies
)

var (
	tradePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(buy|sell|long|short|open|close)\s+(.+)`),
		regexp.MustCompile(`(?i)(做多|做空|买入|卖出|开仓|平仓)\s*(.+)`),
	}
	queryPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(position|balance|pnl|profit|loss|持仓|余额|盈亏)`),
		regexp.MustCompile(`(?i)(show|list|查看)\s*(position|trade|order|持仓|订单)`),
	}
	analyzePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(analyze|analysis|分析|看看)\s+(.+)`),
		regexp.MustCompile(`(?i)(what.*think|怎么看|你觉得)\s*(.+)?`),
	}
	settingsPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(set|config|设置|配置)\s+(.+)`),
	}
)

// Route parses user input and determines intent.
func Route(text string) Intent {
	text = strings.TrimSpace(text)
	lower := strings.ToLower(text)

	// Commands
	if strings.HasPrefix(lower, "/") {
		return routeCommand(text)
	}

	// Trade intent
	for _, p := range tradePatterns {
		if m := p.FindStringSubmatch(text); m != nil {
			return Intent{
				Type: IntentTrade,
				Params: map[string]string{
					"action": strings.ToLower(m[1]),
					"detail": m[2],
				},
				Raw: text,
			}
		}
	}

	// Query intent
	for _, p := range queryPatterns {
		if p.MatchString(text) {
			return Intent{Type: IntentQuery, Raw: text}
		}
	}

	// Analyze intent
	for _, p := range analyzePatterns {
		if m := p.FindStringSubmatch(text); m != nil {
			detail := ""
			if len(m) > 2 {
				detail = m[2]
			}
			return Intent{
				Type: IntentAnalyze,
				Params: map[string]string{
					"detail": detail,
				},
				Raw: text,
			}
		}
	}

	// Settings intent
	for _, p := range settingsPatterns {
		if m := p.FindStringSubmatch(text); m != nil {
			return Intent{
				Type: IntentSettings,
				Params: map[string]string{
					"detail": m[2],
				},
				Raw: text,
			}
		}
	}

	// Default: chat
	return Intent{Type: IntentChat, Raw: text}
}

func routeCommand(text string) Intent {
	cmd := strings.ToLower(strings.Fields(text)[0])
	switch cmd {
	case "/start", "/help":
		return Intent{Type: IntentHelp, Raw: text}
	case "/status":
		return Intent{Type: IntentStatus, Raw: text}
	case "/buy", "/sell", "/long", "/short", "/open", "/close":
		parts := strings.SplitN(text, " ", 2)
		detail := ""
		if len(parts) > 1 {
			detail = parts[1]
		}
		return Intent{
			Type: IntentTrade,
			Params: map[string]string{
				"action": strings.TrimPrefix(cmd, "/"),
				"detail": detail,
			},
			Raw: text,
		}
	case "/positions", "/balance", "/pnl":
		return Intent{Type: IntentQuery, Raw: text}
	case "/watch", "/unwatch", "/alert", "/price":
		return Intent{Type: IntentWatch, Raw: text}
	case "/strategy":
		return Intent{Type: IntentStrategy, Raw: text}
	case "/analyze":
		parts := strings.SplitN(text, " ", 2)
		detail := ""
		if len(parts) > 1 {
			detail = parts[1]
		}
		return Intent{
			Type: IntentAnalyze,
			Params: map[string]string{"detail": detail},
			Raw: text,
		}
	case "/settings", "/config":
		return Intent{Type: IntentSettings, Raw: text}
	default:
		return Intent{Type: IntentChat, Raw: text}
	}
}
