package agent

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"nofx/store"
)

const strategyCreateDraftIntentField = "strategy_draft"

var compactCoinPairRE = regexp.MustCompile(`(?i)\b([A-Z0-9]{2,10})\s*(?:和|与|/|,|，|、|\+)\s*([A-Z0-9]{2,10})\b`)

type strategyDraft struct {
	Name             string   `json:"name,omitempty"`
	StrategyKind     string   `json:"strategy_kind,omitempty"`
	CoinSourceIntent string   `json:"coin_source_intent,omitempty"`
	Symbols          []string `json:"symbols,omitempty"`
	Timeframe        string   `json:"timeframe,omitempty"`
	Leverage         int      `json:"leverage,omitempty"`
}

func normalizeStrategyDraft(d strategyDraft) strategyDraft {
	d.Name = strings.TrimSpace(d.Name)
	d.StrategyKind = strings.TrimSpace(d.StrategyKind)
	d.CoinSourceIntent = strings.TrimSpace(d.CoinSourceIntent)
	d.Timeframe = strings.ToLower(strings.TrimSpace(d.Timeframe))
	if d.Leverage < 0 {
		d.Leverage = 0
	}
	if len(d.Symbols) > 0 {
		normalized := make([]string, 0, len(d.Symbols))
		for _, symbol := range d.Symbols {
			symbol = normalizeCoinSymbol(symbol)
			if symbol != "" {
				normalized = append(normalized, symbol)
			}
		}
		d.Symbols = cleanStringList(normalized)
	}
	if len(d.Symbols) > 0 && d.CoinSourceIntent == "" {
		d.CoinSourceIntent = "static"
	}
	if d.CoinSourceIntent == "static" && len(d.Symbols) == 0 {
		d.CoinSourceIntent = ""
	}
	return d
}

func marshalStrategyDraft(d strategyDraft) string {
	d = normalizeStrategyDraft(d)
	raw, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return string(raw)
}

func unmarshalStrategyDraft(raw string) strategyDraft {
	if strings.TrimSpace(raw) == "" {
		return strategyDraft{}
	}
	var d strategyDraft
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return strategyDraft{}
	}
	return normalizeStrategyDraft(d)
}

func buildStrategyDraftFromActiveSession(session ActiveSkillSession) strategyDraft {
	d := strategyDraft{}
	if value, ok := session.CollectedFields[strategyCreateDraftIntentField]; ok {
		d = unmarshalStrategyDraft(activeFieldString(value))
	}
	if value, ok := session.CollectedFields["name"]; ok {
		d.Name = activeFieldString(value)
	}
	d = applyStrategyDraftText(d, session.Goal)
	for i, msg := range session.LocalHistory {
		if msg.Role != "user" {
			continue
		}
		d = applyStrategyDraftText(d, msg.Content)
		if d.Name == "" && i > 0 && activeHistoryMessageAsksStrategyName(session.LocalHistory[i-1].Content) {
			d.Name = inferStandaloneStrategyName(msg.Content)
		}
	}
	return normalizeStrategyDraft(d)
}

func applyStrategyDraftText(d strategyDraft, text string) strategyDraft {
	text = strings.TrimSpace(text)
	if text == "" {
		return normalizeStrategyDraft(d)
	}
	lower := strings.ToLower(text)
	if containsAny(lower, []string{"趋势", "trend"}) {
		d.StrategyKind = "trend"
	}
	if d.Name == "" {
		if value := extractDelimitedSegmentAfterKeywords(text, []string{"取名为", "取名叫", "命名为", "名称叫", "名字叫", "名为", "叫做", "取名", "名称", "名字是", "called"}); value != "" {
			d.Name = value
		}
	}
	if containsAny(lower, []string{"ai500"}) {
		d.CoinSourceIntent = "ai500"
	}
	if symbols := extractStrategyDraftSymbols(text); len(symbols) > 0 {
		d.Symbols = symbols
		d.CoinSourceIntent = "static"
	}
	if timeframes := extractTimeframes(text); len(timeframes) > 0 {
		d.Timeframe = timeframes[0]
	}
	if leverage, ok := extractLabeledInt(text, []string{"杠杆", "leverage"}); ok && leverage > 0 {
		d.Leverage = leverage
	} else if leverage := extractCompactLeverage(text); leverage > 0 {
		d.Leverage = leverage
	}
	return normalizeStrategyDraft(d)
}

func activeHistoryMessageAsksStrategyName(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return containsAny(lower, []string{"策略名", "名称", "名字", "叫什么", "name"})
}

func inferStandaloneStrategyName(text string) string {
	value := strings.TrimSpace(text)
	if value == "" || len([]rune(value)) > 50 {
		return ""
	}
	if strategyCreateConfirmationReply(value) || strategyCreateDefaultConfigReply(value) || isCancelSkillReply(value) {
		return ""
	}
	if parseStrategyTypeValue(value) != "" {
		return ""
	}
	if containsAny(strings.ToLower(value), []string{"创建", "grid_trading", "ai_trading"}) {
		return ""
	}
	return value
}

func extractStrategyDraftSymbols(text string) []string {
	upper := strings.ToUpper(text)
	candidates := []string{
		"BTC", "ETH", "SOL", "BNB", "XRP", "DOGE", "ADA", "AVAX", "DOT", "LINK",
		"PEPE", "SHIB", "ARB", "OP", "SUI", "APT", "SEI", "TIA", "JUP", "WIF",
		"NEAR", "ATOM", "MATIC", "INJ", "AAVE", "UNI", "LDO", "MKR", "CRV",
	}
	found := make([]string, 0, 4)
	for _, match := range compactCoinPairRE.FindAllStringSubmatch(upper, -1) {
		if len(match) >= 3 {
			found = append(found, match[1], match[2])
		}
	}
	for _, symbol := range candidates {
		if strings.Contains(upper, symbol+"USDT") || strings.Contains(upper, symbol+"USD") || strings.Contains(upper, symbol) {
			found = append(found, symbol)
		}
	}
	return cleanStringList(symbolsToUSDT(found))
}

func symbolsToUSDT(symbols []string) []string {
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = normalizeCoinSymbol(symbol)
		if symbol != "" {
			out = append(out, symbol)
		}
	}
	return out
}

func extractCompactLeverage(text string) int {
	lower := strings.ToLower(text)
	for _, marker := range []string{"x", "倍"} {
		idx := strings.Index(lower, marker)
		if idx <= 0 {
			continue
		}
		prefix := lower[:idx]
		matches := firstIntegerPattern.FindAllString(prefix, -1)
		if len(matches) == 0 {
			continue
		}
		value, err := strconv.Atoi(matches[len(matches)-1])
		if err == nil {
			return value
		}
	}
	return 0
}

func applyStrategyDraftToConfig(cfg *store.StrategyConfig, draft strategyDraft) []string {
	if cfg == nil {
		return nil
	}
	draft = normalizeStrategyDraft(draft)
	changed := make([]string, 0, 4)
	if draft.StrategyKind != "" {
		cfg.StrategyType = "ai_trading"
		changed = append(changed, "strategy_kind")
	}
	switch draft.CoinSourceIntent {
	case "static":
		if len(draft.Symbols) > 0 {
			cfg.CoinSource.SourceType = "static"
			cfg.CoinSource.StaticCoins = append([]string(nil), draft.Symbols...)
			cfg.CoinSource.UseAI500 = false
			cfg.CoinSource.UseOITop = false
			cfg.CoinSource.UseOILow = false
			changed = append(changed, "symbols")
		}
	case "ai500":
		cfg.CoinSource.SourceType = "ai500"
		cfg.CoinSource.UseAI500 = true
		if cfg.CoinSource.AI500Limit <= 0 {
			cfg.CoinSource.AI500Limit = 3
		}
		changed = append(changed, "coin_source")
	}
	if draft.Timeframe != "" {
		cfg.Indicators.Klines.PrimaryTimeframe = draft.Timeframe
		if len(cfg.Indicators.Klines.SelectedTimeframes) == 0 {
			cfg.Indicators.Klines.SelectedTimeframes = []string{draft.Timeframe}
		} else if !containsString(cfg.Indicators.Klines.SelectedTimeframes, draft.Timeframe) {
			cfg.Indicators.Klines.SelectedTimeframes = append([]string{draft.Timeframe}, cfg.Indicators.Klines.SelectedTimeframes...)
		}
		changed = append(changed, "timeframe")
	}
	if draft.Leverage > 0 {
		cfg.RiskControl.BTCETHMaxLeverage = draft.Leverage
		cfg.RiskControl.AltcoinMaxLeverage = draft.Leverage
		changed = append(changed, "leverage")
	}
	return cleanStringList(changed)
}
