package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"nofx/mcp"
)

const (
	plannerMaxSteps      = 8
	plannerMaxIterations = 12
	observationMaxLength = 400
)

var (
	plannerCreateTimeout = 36 * time.Second
	plannerReplanTimeout = 24 * time.Second
	plannerReasonTimeout = 30 * time.Second
	plannerFinalTimeout  = 36 * time.Second
	directReplyTimeout   = 8 * time.Second
)

type replannerDecision struct {
	Action      string     `json:"action"`
	Goal        string     `json:"goal,omitempty"`
	Steps       []PlanStep `json:"steps,omitempty"`
	Instruction string     `json:"instruction,omitempty"`
	Question    string     `json:"question,omitempty"`
}

type readFastPathRequest struct {
	Kind     string
	ArgsJSON string
}

type directReplyDecision struct {
	Action string `json:"action"`
	Answer string `json:"answer,omitempty"`
}

func latestAskedQuestion(state ExecutionState) string {
	if state.Waiting != nil && strings.TrimSpace(state.Waiting.Question) != "" {
		return strings.TrimSpace(state.Waiting.Question)
	}
	for i := len(state.Steps) - 1; i >= 0; i-- {
		step := state.Steps[i]
		if step.Type == planStepTypeAskUser {
			if q := strings.TrimSpace(step.Instruction); q != "" {
				return q
			}
			if q := strings.TrimSpace(step.OutputSummary); q != "" {
				return q
			}
		}
	}
	if state.Status == executionStatusWaitingUser {
		return strings.TrimSpace(state.FinalAnswer)
	}
	return ""
}

func buildWaitingState(state ExecutionState, step PlanStep, question string) *WaitingState {
	waiting := &WaitingState{
		Question:           strings.TrimSpace(question),
		Intent:             inferWaitingIntent(state.Goal, step, question),
		PendingFields:      inferPendingFields(step, question),
		ConfirmationTarget: inferConfirmationTarget(state.Goal, step, question),
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	return normalizeWaitingState(waiting)
}

func inferWaitingIntent(goal string, step PlanStep, question string) string {
	lowerGoal := strings.ToLower(strings.TrimSpace(goal))
	lowerQuestion := strings.ToLower(strings.TrimSpace(question))
	switch {
	case step.RequiresConfirmation || strings.Contains(lowerQuestion, "需要我") || strings.Contains(lowerQuestion, "confirm") || strings.Contains(lowerQuestion, "确认"):
		return "confirm_action"
	case strings.Contains(lowerGoal, "交易员") || strings.Contains(lowerGoal, "trader"):
		return "complete_trader_setup"
	case strings.Contains(lowerGoal, "交易所") || strings.Contains(lowerGoal, "exchange"):
		return "complete_exchange_config"
	case strings.Contains(lowerGoal, "模型") || strings.Contains(lowerGoal, "model"):
		return "complete_model_config"
	default:
		return "provide_missing_information"
	}
}

func inferPendingFields(step PlanStep, question string) []string {
	source := strings.ToLower(strings.TrimSpace(question))
	if source == "" {
		sourceBytes, _ := json.Marshal(step.ToolArgs)
		source = strings.ToLower(string(sourceBytes))
	}
	candidates := []struct {
		key      string
		patterns []string
	}{
		{key: "ai_model_id", patterns: []string{"ai_model_id", "model id", "模型id", "模型 id"}},
		{key: "exchange_id", patterns: []string{"exchange_id", "exchange id", "交易所id", "交易所 id"}},
		{key: "strategy_id", patterns: []string{"strategy_id", "strategy id", "策略id", "策略 id"}},
		{key: "name", patterns: []string{"trader name", "name", "名字", "名称"}},
		{key: "api_key", patterns: []string{"api key", "apikey", "api_key"}},
		{key: "secret_key", patterns: []string{"secret key", "secret_key", "密钥"}},
		{key: "passphrase", patterns: []string{"passphrase", "密码短语"}},
	}
	fields := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		for _, pattern := range candidate.patterns {
			if strings.Contains(source, pattern) {
				fields = append(fields, candidate.key)
				break
			}
		}
	}
	return cleanStringList(fields)
}

func inferConfirmationTarget(goal string, step PlanStep, question string) string {
	if step.RequiresConfirmation {
		if step.ToolName != "" {
			return step.ToolName
		}
	}
	lowerGoal := strings.ToLower(strings.TrimSpace(goal))
	lowerQuestion := strings.ToLower(strings.TrimSpace(question))
	switch {
	case strings.Contains(lowerGoal, "交易员") || strings.Contains(lowerQuestion, "交易员") || strings.Contains(lowerGoal, "trader"):
		return "trader"
	case strings.Contains(lowerGoal, "交易所") || strings.Contains(lowerQuestion, "交易所") || strings.Contains(lowerGoal, "exchange"):
		return "exchange_config"
	case strings.Contains(lowerGoal, "模型") || strings.Contains(lowerQuestion, "模型") || strings.Contains(lowerGoal, "model"):
		return "model_config"
	default:
		return ""
	}
}

func isConfigOrTraderIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	keywords := []string{
		"交易员", "trader", "exchange", "交易所", "模型", "model", "api key", "apikey",
		"绑定", "配置", "setup", "configure", "deepseek", "openai", "claude", "gemini",
		"okx", "binance", "bybit", "gate", "kucoin", "hyperliquid", "aster", "lighter",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func isStrategyIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	keywords := []string{
		"策略", "strategy", "template", "模板", "激进", "趋势跟踪", "网格策略",
		"量化策略", "策略模板", "strategy studio",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func isRealtimeAccountIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	keywords := []string{
		"余额", "balance", "equity", "净值", "available", "available balance",
		"持仓", "position", "positions", "仓位", "unrealized pnl", "浮盈", "浮亏",
		"交易历史", "trade history", "history", "closed trades", "recent trades",
		"订单", "order", "orders", "成交", "pnl", "profit", "loss",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func snapshotKindsForIntent(userText string) []string {
	kinds := make([]string, 0, 6)
	if isConfigOrTraderIntent(userText) {
		kinds = append(kinds,
			"current_model_configs",
			"current_exchange_configs",
			"current_traders",
		)
	}
	if isStrategyIntent(userText) {
		kinds = append(kinds, "current_strategies")
	}
	return uniqueStrings(kinds)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func withPlannerStageTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= timeout {
			return context.WithCancel(ctx)
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func isPlannerTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func plannerTimeoutMessage(lang string) string {
	if lang == "zh" {
		return "⏱️ 当前请求处理超时，请重试一次。若持续出现，请把问题拆小一点。"
	}
	return "⏱️ This request timed out. Please try again, or break it into a smaller request."
}

func shouldResetExecutionStateForNewAttempt(text string, state ExecutionState) bool {
	if state.SessionID == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	retrySignals := []string{
		"再试", "重试", "重新", "继续", "继续创建", "我已经配置好了", "已经配置好了", "我配好了",
		"我已经弄好了", "已经弄好了", "好了", "retry", "try again", "continue", "resume",
		"i configured it", "i've configured it", "i already configured", "configured already",
	}
	for _, signal := range retrySignals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	if isConfigOrTraderIntent(lower) && (state.Status == executionStatusFailed || state.Status == executionStatusCompleted) {
		return true
	}
	if isConfigOrTraderIntent(lower) && state.Status == executionStatusWaitingUser {
		return true
	}
	return false
}

func ensureCurrentReferences(state *ExecutionState) {
	if state.CurrentReferences == nil {
		state.CurrentReferences = &CurrentReferences{}
	}
}

func preferReference(current **EntityReference, id, name string) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" && name == "" {
		return
	}
	if *current == nil {
		*current = &EntityReference{}
	}
	if id != "" {
		(*current).ID = id
	}
	if name != "" {
		(*current).Name = name
	}
}

func matchEntityReference(text string, candidates []EntityReference) *EntityReference {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return nil
	}
	var matched *EntityReference
	for _, candidate := range candidates {
		id := strings.ToLower(strings.TrimSpace(candidate.ID))
		name := strings.ToLower(strings.TrimSpace(candidate.Name))
		if id == "" && name == "" {
			continue
		}
		if (id != "" && strings.Contains(lower, id)) || (name != "" && strings.Contains(lower, name)) {
			if matched != nil {
				return nil
			}
			copy := candidate
			matched = &copy
		}
	}
	return matched
}

func (a *Agent) refreshCurrentReferencesForUserText(storeUserID, text string, state *ExecutionState) {
	if a.store == nil || strings.TrimSpace(text) == "" {
		return
	}
	ensureCurrentReferences(state)

	if strategies, err := a.store.Strategy().List(storeUserID); err == nil {
		candidates := make([]EntityReference, 0, len(strategies))
		for _, strategy := range strategies {
			candidates = append(candidates, EntityReference{ID: strategy.ID, Name: strategy.Name})
		}
		if ref := matchEntityReference(text, candidates); ref != nil {
			preferReference(&state.CurrentReferences.Strategy, ref.ID, ref.Name)
		}
	}
	if traders, err := a.store.Trader().List(storeUserID); err == nil {
		candidates := make([]EntityReference, 0, len(traders))
		for _, trader := range traders {
			candidates = append(candidates, EntityReference{ID: trader.ID, Name: trader.Name})
		}
		if ref := matchEntityReference(text, candidates); ref != nil {
			preferReference(&state.CurrentReferences.Trader, ref.ID, ref.Name)
		}
	}
	if models, err := a.store.AIModel().List(storeUserID); err == nil {
		candidates := make([]EntityReference, 0, len(models))
		for _, model := range models {
			name := model.Name
			if name == "" {
				name = model.CustomModelName
			}
			if name == "" {
				name = model.Provider
			}
			candidates = append(candidates, EntityReference{ID: model.ID, Name: name})
		}
		if ref := matchEntityReference(text, candidates); ref != nil {
			preferReference(&state.CurrentReferences.Model, ref.ID, ref.Name)
		}
	}
	if exchanges, err := a.store.Exchange().List(storeUserID); err == nil {
		candidates := make([]EntityReference, 0, len(exchanges))
		for _, exchange := range exchanges {
			name := exchange.AccountName
			if name == "" {
				name = exchange.ExchangeType
			}
			candidates = append(candidates, EntityReference{ID: exchange.ID, Name: name})
		}
		if ref := matchEntityReference(text, candidates); ref != nil {
			preferReference(&state.CurrentReferences.Exchange, ref.ID, ref.Name)
		}
	}
}

func updateCurrentReferencesFromToolResult(state *ExecutionState, toolName, raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false
	}
	ensureCurrentReferences(state)
	before, _ := json.Marshal(state.CurrentReferences)

	switch toolName {
	case "manage_strategy":
		if item, ok := payload["strategy"].(map[string]any); ok {
			preferReference(&state.CurrentReferences.Strategy, asString(item["id"]), asString(item["name"]))
		}
	case "manage_trader":
		if item, ok := payload["trader"].(map[string]any); ok {
			preferReference(&state.CurrentReferences.Trader, asString(item["id"]), asString(item["name"]))
			preferReference(&state.CurrentReferences.Model, asString(item["ai_model_id"]), "")
			preferReference(&state.CurrentReferences.Exchange, asString(item["exchange_id"]), "")
			preferReference(&state.CurrentReferences.Strategy, asString(item["strategy_id"]), "")
		}
	case "manage_model_config":
		if item, ok := payload["model"].(map[string]any); ok {
			name := asString(item["name"])
			if name == "" {
				name = asString(item["provider"])
			}
			preferReference(&state.CurrentReferences.Model, asString(item["id"]), name)
		}
	case "manage_exchange_config":
		if item, ok := payload["exchange"].(map[string]any); ok {
			name := asString(item["account_name"])
			if name == "" {
				name = asString(item["exchange_type"])
			}
			preferReference(&state.CurrentReferences.Exchange, asString(item["id"]), name)
		}
	case "get_strategies":
		if items, ok := payload["strategies"].([]any); ok && len(items) == 1 {
			if item, ok := items[0].(map[string]any); ok {
				preferReference(&state.CurrentReferences.Strategy, asString(item["id"]), asString(item["name"]))
			}
		}
	}
	state.CurrentReferences = normalizeCurrentReferences(state.CurrentReferences)
	after, _ := json.Marshal(state.CurrentReferences)
	return string(before) != string(after)
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func containsAnyKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func detectReadFastPath(text string) *readFastPathRequest {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return nil
	}

	switch lower {
	case "/traders":
		return &readFastPathRequest{Kind: "list_traders"}
	case "/strategies":
		return &readFastPathRequest{Kind: "get_strategies"}
	case "/models":
		return &readFastPathRequest{Kind: "get_model_configs"}
	case "/exchanges":
		return &readFastPathRequest{Kind: "get_exchange_configs"}
	case "/balance":
		return &readFastPathRequest{Kind: "get_balance"}
	case "/positions":
		return &readFastPathRequest{Kind: "get_positions"}
	case "/history", "/trades":
		return &readFastPathRequest{Kind: "get_trade_history", ArgsJSON: `{"limit":10}`}
	default:
		return nil
	}
}

func (a *Agent) tryReadFastPath(storeUserID string, userID int64, lang, text string) (string, bool) {
	req := detectReadFastPath(text)
	if req == nil {
		return "", false
	}
	if a.history == nil {
		a.history = newChatHistory(100)
	}

	a.history.Add(userID, "user", text)
	raw := a.executeReadFastPath(storeUserID, userID, req)
	answer := formatReadFastPathResponse(lang, req.Kind, raw)
	a.history.Add(userID, "assistant", answer)
	if !isEphemeralReadFastPathKind(req.Kind) {
		a.maybeUpdateTaskStateIncrementally(context.Background(), userID)
		a.maybeCompressHistory(context.Background(), userID)
	}
	return answer, true
}

func isEphemeralReadFastPathKind(kind string) bool {
	switch kind {
	case "get_balance", "get_positions", "get_trade_history":
		return true
	default:
		return false
	}
}

func (a *Agent) executeReadFastPath(storeUserID string, _ int64, req *readFastPathRequest) string {
	switch req.Kind {
	case "get_balance":
		return a.toolGetBalance()
	case "get_positions":
		return a.toolGetPositions()
	case "get_trade_history":
		return a.toolGetTradeHistory(req.ArgsJSON)
	case "get_strategies":
		return a.toolGetStrategies(storeUserID)
	case "list_traders":
		return a.toolListTraders(storeUserID)
	case "get_model_configs":
		return a.toolGetModelConfigs(storeUserID)
	case "get_exchange_configs":
		return a.toolGetExchangeConfigs(storeUserID)
	default:
		return `{"error":"unsupported fast path"}`
	}
}

func formatReadFastPathResponse(lang, kind, raw string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return summarizeObservation(raw)
	}
	if errMsg, _ := payload["error"].(string); strings.TrimSpace(errMsg) != "" {
		return summarizeObservation(raw)
	}

	switch kind {
	case "get_strategies":
		items, _ := payload["strategies"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前还没有策略。"
			}
			return "There are no strategies yet."
		}
		lines := []string{"Current strategies:"}
		if lang == "zh" {
			lines[0] = "当前策略："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(entry["name"])
			if name == "" {
				name = asString(entry["id"])
			}
			meta := make([]string, 0, 2)
			if active, _ := entry["is_active"].(bool); active {
				meta = append(meta, "active")
			}
			if isDefault, _ := entry["is_default"].(bool); isDefault {
				meta = append(meta, "default")
			}
			if len(meta) > 0 {
				lines = append(lines, fmt.Sprintf("- %s (%s)", name, strings.Join(meta, ", ")))
			} else {
				lines = append(lines, fmt.Sprintf("- %s", name))
			}
		}
		return strings.Join(lines, "\n")
	case "list_traders":
		items, _ := payload["traders"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前还没有交易员。"
			}
			return "There are no traders yet."
		}
		lines := []string{"Current traders:"}
		if lang == "zh" {
			lines[0] = "当前交易员："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(entry["name"])
			line := fmt.Sprintf("- %s", name)
			meta := cleanStringList([]string{asString(entry["exchange_type"]), asString(entry["ai_model_id"])})
			if len(meta) > 0 {
				line += fmt.Sprintf(" (%s)", strings.Join(meta, ", "))
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	case "get_model_configs":
		items, _ := payload["model_configs"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前还没有模型配置。"
			}
			return "There are no model configs yet."
		}
		lines := []string{"Current model configs:"}
		if lang == "zh" {
			lines[0] = "当前模型配置："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(entry["name"])
			if name == "" {
				name = asString(entry["provider"])
			}
			meta := make([]string, 0, 2)
			if enabled, _ := entry["enabled"].(bool); enabled {
				meta = append(meta, "enabled")
			}
			if model := asString(entry["custom_model_name"]); model != "" {
				meta = append(meta, model)
			}
			if len(meta) > 0 {
				lines = append(lines, fmt.Sprintf("- %s (%s)", name, strings.Join(meta, ", ")))
			} else {
				lines = append(lines, fmt.Sprintf("- %s", name))
			}
		}
		return strings.Join(lines, "\n")
	case "get_exchange_configs":
		items, _ := payload["exchange_configs"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前还没有交易所配置。"
			}
			return "There are no exchange configs yet."
		}
		lines := []string{"Current exchange configs:"}
		if lang == "zh" {
			lines[0] = "当前交易所配置："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := asString(entry["account_name"])
			if name == "" {
				name = asString(entry["exchange_type"])
			}
			meta := cleanStringList([]string{asString(entry["exchange_type"])})
			if enabled, _ := entry["enabled"].(bool); enabled {
				meta = append(meta, "enabled")
			}
			if len(meta) > 0 {
				lines = append(lines, fmt.Sprintf("- %s (%s)", name, strings.Join(meta, ", ")))
			} else {
				lines = append(lines, fmt.Sprintf("- %s", name))
			}
		}
		return strings.Join(lines, "\n")
	case "get_balance":
		items, _ := payload["balances"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前没有可用的余额数据。"
			}
			return "No balance data is available right now."
		}
		lines := []string{"Current balance overview:"}
		if lang == "zh" {
			lines[0] = "当前余额概览："
		}
		var totalEquity float64
		var totalAvailable float64
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			equity := toFloat(entry["total_equity"])
			available := toFloat(entry["available"])
			totalEquity += equity
			totalAvailable += available
			lines = append(lines, fmt.Sprintf("- %s (%s): equity %.4f, available %.4f",
				asString(entry["name"]), asString(entry["exchange"]),
				equity, available))
		}
		if len(items) > 1 {
			if lang == "zh" {
				lines = append(lines, fmt.Sprintf("汇总：equity %.4f, available %.4f", totalEquity, totalAvailable))
			} else {
				lines = append(lines, fmt.Sprintf("Total: equity %.4f, available %.4f", totalEquity, totalAvailable))
			}
		}
		return strings.Join(lines, "\n")
	case "get_positions":
		items, _ := payload["positions"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前没有持仓。"
			}
			return "There are no open positions right now."
		}
		lines := []string{"Current positions:"}
		if lang == "zh" {
			lines[0] = "当前持仓："
		}
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s %s size %.4f, entry %.4f, pnl %.4f",
				asString(entry["symbol"]), asString(entry["side"]),
				toFloat(entry["size"]), toFloat(entry["entry_price"]), toFloat(entry["unrealized_pnl"])))
		}
		return strings.Join(lines, "\n")
	case "get_trade_history":
		items, _ := payload["trades"].([]any)
		if len(items) == 0 {
			if lang == "zh" {
				return "当前没有已平仓交易历史。"
			}
			return "There is no closed trade history yet."
		}
		summary, _ := payload["summary"].(map[string]any)
		head := fmt.Sprintf("Recent trades: %.0f total, win rate %s, total PnL %.4f",
			toFloat(summary["total_trades"]), asString(summary["win_rate"]), toFloat(summary["total_pnl"]))
		if lang == "zh" {
			head = fmt.Sprintf("最近交易：共 %.0f 笔，胜率 %s，总 PnL %.4f",
				toFloat(summary["total_trades"]), asString(summary["win_rate"]), toFloat(summary["total_pnl"]))
		}
		lines := []string{head}
		for idx, item := range items {
			if idx >= 5 {
				break
			}
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s %s pnl %.4f (%s -> %s)",
				asString(entry["symbol"]), asString(entry["side"]), toFloat(entry["pnl"]),
				asString(entry["entry_time"]), asString(entry["exit_time"])))
		}
		return strings.Join(lines, "\n")
	default:
		return summarizeObservation(raw)
	}
}

func (a *Agent) thinkAndAct(ctx context.Context, storeUserID string, userID int64, lang, text string) (string, error) {
	if answer, ok, err := a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, nil); ok || err != nil {
		return answer, err
	}
	if answer, ok := a.tryDirectAnswer(ctx, userID, lang, text, nil); ok {
		return answer, nil
	}
	if answer, ok := a.tryLLMSkillRoute(ctx, storeUserID, userID, lang, text, nil); ok {
		return answer, nil
	}
	if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, nil); ok {
		return answer, nil
	}
	if answer, ok := a.tryReadFastPath(storeUserID, userID, lang, text); ok {
		return answer, nil
	}
	if a.aiClient == nil {
		return a.noAIFallback(lang, text)
	}
	return a.runPlannedAgent(ctx, storeUserID, userID, lang, text, nil)
}

func (a *Agent) thinkAndActStream(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, error) {
	if answer, ok, err := a.tryStatePriorityPath(ctx, storeUserID, userID, lang, text, onEvent); ok || err != nil {
		return answer, err
	}
	if answer, ok := a.tryDirectAnswer(ctx, userID, lang, text, onEvent); ok {
		return answer, nil
	}
	if answer, ok := a.tryLLMSkillRoute(ctx, storeUserID, userID, lang, text, onEvent); ok {
		return answer, nil
	}
	if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent); ok {
		return answer, nil
	}
	if answer, ok := a.tryReadFastPath(storeUserID, userID, lang, text); ok {
		if onEvent != nil {
			onEvent(StreamEventTool, "read_fast_path")
			onEvent(StreamEventDelta, answer)
		}
		return answer, nil
	}
	if a.aiClient == nil {
		return a.noAIFallback(lang, text)
	}
	return a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
}

func (a *Agent) hasActiveSkillSession(userID int64) bool {
	session := a.getSkillSession(userID)
	return strings.TrimSpace(session.Name) != ""
}

func hasActiveExecutionState(state ExecutionState) bool {
	if strings.TrimSpace(state.SessionID) == "" {
		return false
	}
	switch strings.TrimSpace(state.Status) {
	case executionStatusPlanning, executionStatusRunning, executionStatusWaitingUser:
		return true
	default:
		return false
	}
}

func (a *Agent) tryStatePriorityPath(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, bool, error) {
	if a.hasActiveSkillSession(userID) {
		if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent); ok {
			return answer, true, nil
		}
	}

	state := a.getExecutionState(userID)
	if hasActiveExecutionState(state) {
		answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
		return answer, true, err
	}

	return "", false, nil
}

func (a *Agent) tryDirectAnswer(ctx context.Context, userID int64, lang, text string, onEvent func(event, data string)) (string, bool) {
	if a.aiClient == nil {
		return "", false
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}

	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	taskStateCtx := buildTaskStateContext(a.getTaskState(userID))
	executionState := normalizeExecutionState(a.getExecutionState(userID))
	executionJSON, _ := json.Marshal(executionState)
	systemPrompt := `You are the first-pass router for NOFXi.
Decide whether the assistant can answer the user's message directly without using skills, tools, or planning.
Return JSON only. Do not return markdown.

Use "direct_answer" only when a concise, self-contained answer is sufficient.
Examples that often fit direct_answer:
- greetings, thanks, small talk
- concept explanations
- open-ended advice that does not require current system state
- trading education or opinion questions that can be answered from general reasoning

Use "defer" when the message likely needs:
- a management or diagnosis skill
- tool reads
- multi-step planning
- continuation of an active execution flow that needs stateful follow-up

Rules:
- Consider Recent conversation, Task state, and Execution state JSON before deciding.
- Default to direct_answer for greetings, thanks, identity questions, and other lightweight conversational turns unless there is a clearly unfinished operational flow that the user is continuing.
- If the user is clearly continuing an unfinished operational flow, choose defer.
- If you choose direct_answer, provide the final user-facing answer in the same language as the user.
- Prefer defer when uncertain.

Return JSON with this exact shape:
{"action":"direct_answer|defer","answer":""}`
	userPrompt := fmt.Sprintf("Language: %s\nUser message: %s\n\nRecent conversation:\n%s\n\nTask state:\n%s\n\nExecution state JSON:\n%s", lang, text, recentConversationCtx, taskStateCtx, string(executionJSON))

	stageCtx, cancel := withPlannerStageTimeout(ctx, directReplyTimeout)
	defer cancel()

	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	if err != nil {
		return "", false
	}

	decision, err := parseDirectReplyDecision(raw)
	if err != nil {
		return "", false
	}
	if decision.Action != "direct_answer" {
		return "", false
	}

	answer := strings.TrimSpace(decision.Answer)
	if answer == "" {
		return "", false
	}

	if a.history == nil {
		a.history = newChatHistory(100)
	}
	a.history.Add(userID, "user", text)
	a.history.Add(userID, "assistant", answer)
	a.maybeUpdateTaskStateIncrementally(ctx, userID)
	a.maybeCompressHistory(ctx, userID)
	if onEvent != nil {
		onEvent(StreamEventDelta, answer)
	}
	return answer, true
}

func parseDirectReplyDecision(raw string) (directReplyDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision directReplyDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeDirectReplyDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeDirectReplyDecision(decision), nil
		}
	}
	return directReplyDecision{}, fmt.Errorf("invalid direct reply decision json")
}

func normalizeDirectReplyDecision(decision directReplyDecision) directReplyDecision {
	decision.Action = strings.TrimSpace(strings.ToLower(decision.Action))
	decision.Answer = strings.TrimSpace(decision.Answer)
	return decision
}

func (a *Agent) runPlannedAgent(ctx context.Context, storeUserID string, userID int64, lang, text string, onEvent func(event, data string)) (string, error) {
	a.history.Add(userID, "user", text)
	if onEvent != nil {
		onEvent(StreamEventPlanning, a.planningStatusText(lang))
	}

	state, err := a.prepareExecutionState(ctx, storeUserID, userID, lang, text)
	if err != nil {
		if isPlannerTimeoutError(err) {
			msg := plannerTimeoutMessage(lang)
			if onEvent != nil {
				onEvent(StreamEventError, msg)
				onEvent(StreamEventDelta, msg)
			}
			return msg, nil
		}
		a.logger.Warn("planner failed, falling back to legacy loop", "error", err, "user_id", userID)
		return a.thinkAndActLegacy(ctx, userID, lang, text, onEvent)
	}

	answer, err := a.executePlan(ctx, storeUserID, userID, lang, &state, onEvent)
	if err != nil {
		if isPlannerTimeoutError(err) {
			msg := plannerTimeoutMessage(lang)
			if onEvent != nil {
				onEvent(StreamEventError, msg)
				onEvent(StreamEventDelta, msg)
			}
			return msg, nil
		}
		a.logger.Warn("plan execution failed, falling back to legacy loop", "error", err, "user_id", userID)
		return a.thinkAndActLegacy(ctx, userID, lang, text, onEvent)
	}

	a.history.Add(userID, "assistant", answer)
	a.maybeUpdateTaskStateIncrementally(ctx, userID)
	a.maybeCompressHistory(ctx, userID)
	return answer, nil
}

func (a *Agent) prepareExecutionState(ctx context.Context, storeUserID string, userID int64, lang, text string) (ExecutionState, error) {
	existing := a.getExecutionState(userID)
	if shouldResetExecutionStateForNewAttempt(text, existing) {
		a.clearExecutionState(userID)
		existing = ExecutionState{}
	}
	if existing.Status == executionStatusWaitingUser && existing.SessionID != "" {
		a.refreshCurrentReferencesForUserText(storeUserID, text, &existing)
		askedQuestion := latestAskedQuestion(existing)
		replySummary := strings.TrimSpace(text)
		if askedQuestion != "" {
			replySummary = fmt.Sprintf("Answer to previous question [%s]: %s", askedQuestion, replySummary)
		}
		appendExecutionLog(&existing, Observation{
			Kind:      "user_reply",
			Summary:   replySummary,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
		existing.Status = executionStatusPlanning
		existing.Waiting = nil
		existing.FinalAnswer = ""
		existing.LastError = ""
		existing = a.refreshStateForDynamicRequests(storeUserID, text, existing)
		plan, err := a.createExecutionPlan(ctx, userID, lang, text, existing)
		if err != nil {
			return ExecutionState{}, err
		}
		existing.Goal = plan.Goal
		existing.Steps = plan.Steps
		existing.CurrentStepID = ""
		existing.Status = executionStatusRunning
		existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := a.saveExecutionState(existing); err != nil {
			return ExecutionState{}, err
		}
		return existing, nil
	}

	state := newExecutionState(userID, text)
	a.refreshCurrentReferencesForUserText(storeUserID, text, &state)
	state = a.refreshStateForDynamicRequests(storeUserID, text, state)
	plan, err := a.createExecutionPlan(ctx, userID, lang, text, state)
	if err != nil {
		return ExecutionState{}, err
	}
	state.Goal = plan.Goal
	state.Steps = plan.Steps
	state.Status = executionStatusRunning
	if err := a.saveExecutionState(state); err != nil {
		return ExecutionState{}, err
	}
	return state, nil
}

func (a *Agent) refreshStateForDynamicRequests(storeUserID, userText string, state ExecutionState) ExecutionState {
	kinds := snapshotKindsForIntent(userText)
	if len(kinds) == 0 {
		return state
	}
	kindsToRefresh := make(map[string]struct{}, len(kinds))
	for _, kind := range kinds {
		kindsToRefresh[kind] = struct{}{}
	}

	fresh := make([]Observation, 0, len(state.DynamicSnapshots)+3)
	for _, obs := range state.DynamicSnapshots {
		if _, ok := kindsToRefresh[obs.Kind]; ok {
			continue
		}
		fresh = append(fresh, obs)
	}

	appendSnapshot := func(kind, raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		fresh = append(fresh, Observation{
			Kind:      kind,
			Summary:   summarizeObservation(raw),
			RawJSON:   raw,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	for _, kind := range kinds {
		switch kind {
		case "current_model_configs":
			appendSnapshot(kind, a.toolGetModelConfigs(storeUserID))
		case "current_exchange_configs":
			appendSnapshot(kind, a.toolGetExchangeConfigs(storeUserID))
		case "current_traders":
			appendSnapshot(kind, a.toolListTraders(storeUserID))
		case "current_strategies":
			appendSnapshot(kind, a.toolGetStrategies(storeUserID))
		case "current_balances":
			appendSnapshot(kind, a.toolGetBalance())
		case "current_positions":
			appendSnapshot(kind, a.toolGetPositions())
		case "recent_trade_history":
			appendSnapshot(kind, a.toolGetTradeHistory(`{"limit":10}`))
		}
	}
	state.DynamicSnapshots = fresh
	return state
}

func (a *Agent) buildRecentConversationContext(userID int64, currentUserText string) string {
	if a.history == nil {
		return ""
	}

	msgs := a.history.Get(userID)
	if len(msgs) == 0 {
		return ""
	}

	currentUserText = strings.TrimSpace(currentUserText)
	if currentUserText != "" {
		last := msgs[len(msgs)-1]
		if last.Role == "user" && strings.TrimSpace(last.Content) == currentUserText {
			msgs = msgs[:len(msgs)-1]
		}
	}

	if len(msgs) == 0 {
		return ""
	}
	if len(msgs) > recentConversationMessages {
		msgs = msgs[len(msgs)-recentConversationMessages:]
	}

	transcript := formatChatMessagesForSummary(msgs)
	if transcript == "" {
		return ""
	}
	return transcript
}

func (a *Agent) createExecutionPlan(ctx context.Context, userID int64, lang, userText string, state ExecutionState) (executionPlan, error) {
	toolDefs, _ := json.Marshal(agentTools())
	stateJSON, _ := json.Marshal(normalizeExecutionState(state))
	taskStateCtx := buildTaskStateContext(a.getTaskState(userID))
	recentConversationCtx := a.buildRecentConversationContext(userID, userText)
	if isConfigOrTraderIntent(userText) {
		// Configuration and trader setup requests are especially sensitive to stale
		// summaries like "this capability does not exist". Prefer fresh tool checks.
		taskStateCtx = ""
	}

	systemPrompt := `You are the planning module for NOFXi.
Return JSON only. Do not return markdown.

Create a minimal safe execution plan using these step types only:
- tool
- reason
- ask_user
- respond

Rules:
- Use all available memory layers when planning: Execution state JSON, Recent conversation, and Task state.
- Memory priority order:
  1. Execution state JSON = current operational truth for the active task.
  2. Recent conversation = the best source for what was said in the last few turns.
  3. Task state = compressed durable background only.
- If these memory layers conflict, prefer execution state first, then recent conversation. Do not let task state override fresher evidence.
- Do not ask the user to repeat a fact that is already explicit in execution state or recent conversation unless the inputs are contradictory.
- Use tool steps whenever fresh external data is required.
- Use ask_user if required parameters are missing.
- Never place a trade unless the user intent is explicit.
- For exchange binding or exchange credential requests, prefer get_exchange_configs/manage_exchange_config.
- For AI model binding or model credential requests, prefer get_model_configs/manage_model_config.
- For strategy template creation or editing requests, prefer get_strategies/manage_strategy.
- For trader creation or trader lifecycle requests, prefer manage_trader.
- A strategy template is independent and does not require exchange/model bindings unless the user explicitly asks to run or deploy it through a trader.
- If these tools exist, never answer that the system lacks exchange/model/trader management capability.
- When configuration, strategy, or trader creation is requested, gather missing required fields via ask_user, then call the appropriate tool.
- Before concluding that exchange/model/trader/strategy setup is impossible or missing, first inspect current state with the relevant tools.
- For high-volatility state such as balances, positions, recent trade history, or current config availability, prefer fresh tool reads over old observations.
- Keep the plan short and practical.
- End with either ask_user or respond.
- At most 8 steps.
- For tool steps, set tool_name exactly to one of the available tool names and provide tool_args as JSON object.
- For reason steps, put the reasoning task in instruction.
- For ask_user steps, put the exact follow-up question in instruction.
- For respond steps, put either a short instruction or leave instruction empty.
- If resuming after a waiting_user state, incorporate the new user reply and return a fresh full plan.
- Never invent tools.`

	resumeContext := ""
	if state.SessionID != "" {
		if askedQuestion := latestAskedQuestion(state); askedQuestion != "" {
			resumeContext = fmt.Sprintf("\n\nResume context:\n- The assistant was waiting for the user's answer to this exact question: %s\n- Interpret the new user message as the answer to that question unless the message clearly starts a new topic.", askedQuestion)
			if state.Waiting != nil {
				waitingJSON, _ := json.Marshal(state.Waiting)
				resumeContext += fmt.Sprintf("\n- Structured waiting state JSON: %s", string(waitingJSON))
			}
		}
	}

	userPrompt := fmt.Sprintf("Language: %s\nUser request: %s%s\n\nRecent conversation:\n%s\n\nAvailable tools JSON:\n%s\n\nPersistent preferences:\n%s\n\nTask state:\n%s\n\nExecution state JSON:\n%s\n\nReturn JSON with this exact shape:\n{\"goal\":\"\",\"steps\":[{\"id\":\"step_1\",\"type\":\"tool|reason|ask_user|respond\",\"title\":\"\",\"tool_name\":\"\",\"tool_args\":{},\"instruction\":\"\",\"requires_confirmation\":false}]}", lang, userText, resumeContext, recentConversationCtx, string(toolDefs), a.buildPersistentPreferencesContext(userID), taskStateCtx, string(stateJSON))

	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerCreateTimeout)
	defer cancel()

	resp, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	if err != nil {
		return executionPlan{}, err
	}

	plan, err := parseExecutionPlanJSON(resp)
	if err != nil {
		return executionPlan{}, err
	}
	if len(plan.Steps) == 0 {
		return executionPlan{}, fmt.Errorf("empty execution plan")
	}
	if len(plan.Steps) > plannerMaxSteps {
		plan.Steps = plan.Steps[:plannerMaxSteps]
	}
	for i := range plan.Steps {
		if plan.Steps[i].ID == "" {
			plan.Steps[i].ID = fmt.Sprintf("step_%d", i+1)
		}
		if plan.Steps[i].Status == "" {
			plan.Steps[i].Status = planStepStatusPending
		}
		if plan.Steps[i].Title == "" {
			plan.Steps[i].Title = strings.ReplaceAll(plan.Steps[i].ID, "_", " ")
		}
	}
	if strings.TrimSpace(plan.Goal) == "" {
		plan.Goal = strings.TrimSpace(userText)
	}
	return plan, nil
}

func parseExecutionPlanJSON(raw string) (executionPlan, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var plan executionPlan
	if err := json.Unmarshal([]byte(raw), &plan); err == nil {
		return plan, nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &plan); err == nil {
			return plan, nil
		}
	}
	return executionPlan{}, fmt.Errorf("invalid execution plan json")
}

func (a *Agent) executePlan(ctx context.Context, storeUserID string, userID int64, lang string, state *ExecutionState, onEvent func(event, data string)) (string, error) {
	if onEvent != nil {
		onEvent(StreamEventPlan, formatPlanStatus(*state, lang))
	}

	for i := 0; i < plannerMaxIterations; i++ {
		stepIndex := nextPendingStepIndex(state.Steps)
		if stepIndex < 0 {
			finalText, err := a.generateFinalPlanResponse(ctx, userID, lang, *state, "")
			if err != nil {
				return "", err
			}
			state.Status = executionStatusCompleted
			state.FinalAnswer = finalText
			state.CurrentStepID = ""
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := a.saveExecutionState(*state); err != nil {
				return "", err
			}
			if onEvent != nil {
				onEvent(StreamEventDelta, finalText)
			}
			return finalText, nil
		}

		step := &state.Steps[stepIndex]
		step.Status = planStepStatusRunning
		state.Status = executionStatusRunning
		state.CurrentStepID = step.ID
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if onEvent != nil {
			onEvent(StreamEventStepStart, formatStepStatus(*step, stepIndex, len(state.Steps), lang))
		}
		if err := a.saveExecutionState(*state); err != nil {
			return "", err
		}

		switch step.Type {
		case planStepTypeTool:
			if onEvent != nil {
				onEvent(StreamEventTool, step.ToolName)
			}
			result := a.executePlanTool(ctx, storeUserID, userID, lang, *step)
			summary := summarizeObservation(result)
			referencesChanged := false
			step.Status = planStepStatusCompleted
			step.OutputSummary = summary
			appendExecutionLog(state, Observation{
				StepID:    step.ID,
				Kind:      "tool_result",
				Summary:   summary,
				RawJSON:   result,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			})
			referencesChanged = updateCurrentReferencesFromToolResult(state, step.ToolName, result)
			if shouldAttemptReplan(*state, *step, referencesChanged) {
				state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				if err := a.saveExecutionState(*state); err != nil {
					return "", err
				}
				if onEvent != nil {
					onEvent(StreamEventStepComplete, formatStepCompleteStatus(*step, lang))
				}
				decision, err := a.replanAfterStep(ctx, userID, lang, *state, *step)
				if err == nil && applyReplannerDecision(state, decision) {
					state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
					if err := a.saveExecutionState(*state); err != nil {
						return "", err
					}
					if onEvent != nil {
						onEvent(StreamEventReplan, formatReplanStatus(decision, lang))
						onEvent(StreamEventPlan, formatPlanStatus(*state, lang))
					}
				}
				continue
			}
		case planStepTypeReason:
			reasoning, err := a.executeReasonStep(ctx, userID, lang, state.Goal, *state, *step)
			if err != nil {
				step.Status = planStepStatusFailed
				step.Error = err.Error()
				state.Status = executionStatusFailed
				state.LastError = err.Error()
				_ = a.saveExecutionState(*state)
				return "", err
			}
			step.Status = planStepStatusCompleted
			step.OutputSummary = reasoning
			appendExecutionLog(state, Observation{
				StepID:    step.ID,
				Kind:      "reasoning",
				Summary:   reasoning,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			})
		case planStepTypeAskUser:
			question := strings.TrimSpace(step.Instruction)
			if question == "" {
				if lang == "zh" {
					question = "我还缺少一些信息，麻烦你补充一下。"
				} else {
					question = "I need a bit more information before I continue."
				}
			}
			step.Status = planStepStatusCompleted
			step.OutputSummary = question
			state.Status = executionStatusWaitingUser
			state.Waiting = buildWaitingState(*state, *step, question)
			state.FinalAnswer = question
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := a.saveExecutionState(*state); err != nil {
				return "", err
			}
			if onEvent != nil {
				onEvent(StreamEventStepComplete, formatStepCompleteStatus(*step, lang))
				onEvent(StreamEventDelta, question)
			}
			return question, nil
		case planStepTypeRespond:
			finalText, err := a.generateFinalPlanResponse(ctx, userID, lang, *state, step.Instruction)
			if err != nil {
				return "", err
			}
			step.Status = planStepStatusCompleted
			step.OutputSummary = finalText
			state.Status = executionStatusCompleted
			state.Waiting = nil
			state.FinalAnswer = finalText
			state.CurrentStepID = ""
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := a.saveExecutionState(*state); err != nil {
				return "", err
			}
			if onEvent != nil {
				onEvent(StreamEventStepComplete, formatStepCompleteStatus(*step, lang))
				onEvent(StreamEventDelta, finalText)
			}
			return finalText, nil
		default:
			return "", fmt.Errorf("unsupported step type: %s", step.Type)
		}

		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := a.saveExecutionState(*state); err != nil {
			return "", err
		}
		if onEvent != nil {
			onEvent(StreamEventStepComplete, formatStepCompleteStatus(*step, lang))
		}
	}

	return "", fmt.Errorf("plan execution exceeded iteration limit")
}

func (a *Agent) replanAfterStep(ctx context.Context, userID int64, lang string, state ExecutionState, completedStep PlanStep) (replannerDecision, error) {
	obsJSON, _ := json.Marshal(buildObservationContext(state))
	stepsJSON, _ := json.Marshal(state.Steps)
	systemPrompt := `You are the replanning module for NOFXi.
Return JSON only.

Decide what to do after a plan step completed.
Allowed actions:
- continue
- replace_remaining
- ask_user
- finish

Rules:
- Use continue when the current remaining steps still make sense.
- Use replace_remaining when the observations materially change the remaining plan.
- Use ask_user when execution is blocked on missing user input.
- Use finish when there is enough information to answer and remaining steps are unnecessary.
- If action=replace_remaining, return a fresh list of remaining steps only.
- Keep plans short and safe.
- Never invent tools.`

	userPrompt := fmt.Sprintf("Language: %s\nGoal: %s\nCompleted step: %s (%s)\nCompleted summary: %s\n\nCurrent steps JSON:\n%s\n\nObservations JSON:\n%s\n\nPersistent preferences:\n%s\n\nTask state:\n%s\n\nReturn JSON with this exact shape:\n{\"action\":\"continue|replace_remaining|ask_user|finish\",\"goal\":\"\",\"instruction\":\"\",\"question\":\"\",\"steps\":[{\"id\":\"step_x\",\"type\":\"tool|reason|ask_user|respond\",\"title\":\"\",\"tool_name\":\"\",\"tool_args\":{},\"instruction\":\"\",\"requires_confirmation\":false}]}", lang, state.Goal, completedStep.ID, completedStep.Type, completedStep.OutputSummary, string(stepsJSON), string(obsJSON), a.buildPersistentPreferencesContext(userID), buildTaskStateContext(a.getTaskState(userID)))

	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerReplanTimeout)
	defer cancel()

	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx:       stageCtx,
		MaxTokens: intPtr(500),
	})
	if err != nil {
		return replannerDecision{}, err
	}
	return parseReplannerDecisionJSON(raw)
}

func parseReplannerDecisionJSON(raw string) (replannerDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision replannerDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeReplannerDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeReplannerDecision(decision), nil
		}
	}
	return replannerDecision{}, fmt.Errorf("invalid replanner decision json")
}

func normalizeReplannerDecision(decision replannerDecision) replannerDecision {
	decision.Action = strings.TrimSpace(decision.Action)
	decision.Goal = strings.TrimSpace(decision.Goal)
	decision.Instruction = strings.TrimSpace(decision.Instruction)
	decision.Question = strings.TrimSpace(decision.Question)
	for i := range decision.Steps {
		if decision.Steps[i].ID == "" {
			decision.Steps[i].ID = fmt.Sprintf("step_%d", i+1)
		}
		if decision.Steps[i].Status == "" {
			decision.Steps[i].Status = planStepStatusPending
		}
		decision.Steps[i].Type = strings.TrimSpace(decision.Steps[i].Type)
		decision.Steps[i].Title = strings.TrimSpace(decision.Steps[i].Title)
		decision.Steps[i].ToolName = strings.TrimSpace(decision.Steps[i].ToolName)
		decision.Steps[i].Instruction = strings.TrimSpace(decision.Steps[i].Instruction)
	}
	return decision
}

func applyReplannerDecision(state *ExecutionState, decision replannerDecision) bool {
	switch decision.Action {
	case "", "continue":
		return false
	case "finish":
		state.Steps = append(completedSteps(state.Steps), PlanStep{
			ID:          fmt.Sprintf("step_finish_%d", time.Now().UTC().UnixNano()),
			Type:        planStepTypeRespond,
			Title:       "final response",
			Status:      planStepStatusPending,
			Instruction: decision.Instruction,
		})
		state.CurrentStepID = ""
		if decision.Goal != "" {
			state.Goal = decision.Goal
		}
		state.Waiting = nil
		return true
	case "ask_user":
		question := decision.Question
		if question == "" {
			question = decision.Instruction
		}
		state.Steps = append(completedSteps(state.Steps), PlanStep{
			ID:          fmt.Sprintf("step_ask_%d", time.Now().UTC().UnixNano()),
			Type:        planStepTypeAskUser,
			Title:       "need user input",
			Status:      planStepStatusPending,
			Instruction: question,
		})
		state.CurrentStepID = ""
		if decision.Goal != "" {
			state.Goal = decision.Goal
		}
		state.Waiting = buildWaitingState(*state, state.Steps[len(state.Steps)-1], question)
		return true
	case "replace_remaining":
		if len(decision.Steps) == 0 {
			return false
		}
		state.Steps = append(completedSteps(state.Steps), decision.Steps...)
		state.CurrentStepID = ""
		if decision.Goal != "" {
			state.Goal = decision.Goal
		}
		state.Waiting = nil
		return true
	default:
		return false
	}
}

func shouldAttemptReplan(state ExecutionState, step PlanStep, referencesChanged bool) bool {
	if step.Type != planStepTypeTool {
		return false
	}
	if toolResultIndicatesError(step.OutputSummary) || toolResultSignalsDependencyGap(step.OutputSummary) {
		return true
	}
	if referencesChanged {
		return true
	}
	if !hasPendingWorkAfterStep(state.Steps) {
		return false
	}
	switch step.ToolName {
	case "manage_trader", "manage_strategy", "manage_model_config", "manage_exchange_config", "execute_trade":
		return toolActionMayChangePlan(step.ToolArgs)
	default:
		return false
	}
}

func hasPendingWorkAfterStep(steps []PlanStep) bool {
	for _, step := range steps {
		if step.Status == planStepStatusPending {
			return true
		}
	}
	return false
}

func toolActionMayChangePlan(args map[string]any) bool {
	action, _ := args["action"].(string)
	switch strings.TrimSpace(action) {
	case "create", "update", "delete", "start", "stop", "activate", "duplicate":
		return true
	default:
		return false
	}
}

func toolResultIndicatesError(summary string) bool {
	lower := strings.ToLower(strings.TrimSpace(summary))
	return strings.Contains(lower, `"error"`) || strings.Contains(lower, `"status":"error"`) || strings.Contains(lower, "failed to ")
}

func toolResultSignalsDependencyGap(summary string) bool {
	lower := strings.ToLower(strings.TrimSpace(summary))
	patterns := []string{
		"is required", "invalid ai_model_id", "invalid exchange_id", "invalid strategy_id",
		"ai model is disabled", "exchange is disabled", "not found", "missing",
	}
	return containsAnyKeyword(lower, patterns)
}

func completedSteps(steps []PlanStep) []PlanStep {
	out := make([]PlanStep, 0, len(steps))
	for _, step := range steps {
		if step.Status == planStepStatusCompleted {
			out = append(out, step)
		}
	}
	return out
}

func (a *Agent) planningStatusText(lang string) string {
	if lang == "zh" {
		return "🧭 正在规划执行步骤..."
	}
	return "🧭 Planning the next execution steps..."
}

func formatPlanStatus(state ExecutionState, lang string) string {
	parts := make([]string, 0, len(state.Steps))
	for i, step := range state.Steps {
		label := step.Title
		if label == "" {
			label = step.Type
		}
		parts = append(parts, fmt.Sprintf("%d.%s", i+1, label))
	}
	if lang == "zh" {
		return fmt.Sprintf("🗺️ 计划: %s", strings.Join(parts, " -> "))
	}
	return fmt.Sprintf("🗺️ Plan: %s", strings.Join(parts, " -> "))
}

func formatStepStatus(step PlanStep, idx, total int, lang string) string {
	label := step.Title
	if label == "" {
		label = step.Type
	}
	if lang == "zh" {
		return fmt.Sprintf("▶️ 步骤 %d/%d: %s", idx+1, total, label)
	}
	return fmt.Sprintf("▶️ Step %d/%d: %s", idx+1, total, label)
}

func formatStepCompleteStatus(step PlanStep, lang string) string {
	label := step.Title
	if label == "" {
		label = step.Type
	}
	if lang == "zh" {
		return fmt.Sprintf("✅ 已完成: %s", label)
	}
	return fmt.Sprintf("✅ Completed: %s", label)
}

func formatReplanStatus(decision replannerDecision, lang string) string {
	switch decision.Action {
	case "replace_remaining":
		if lang == "zh" {
			return "🔄 已根据新结果更新后续步骤"
		}
		return "🔄 Updated the remaining steps based on new results"
	case "ask_user":
		if lang == "zh" {
			return "📝 当前流程需要用户补充信息"
		}
		return "📝 This flow needs more user input"
	case "finish":
		if lang == "zh" {
			return "🏁 已提前收敛到最终回复"
		}
		return "🏁 Converged early to the final response"
	default:
		if lang == "zh" {
			return "🔄 已重新评估计划"
		}
		return "🔄 Re-evaluated the plan"
	}
}

func (a *Agent) executePlanTool(ctx context.Context, storeUserID string, userID int64, lang string, step PlanStep) string {
	argsJSON := "{}"
	if len(step.ToolArgs) > 0 {
		if data, err := json.Marshal(step.ToolArgs); err == nil {
			argsJSON = string(data)
		}
	}
	return a.handleToolCall(ctx, storeUserID, userID, lang, mcp.ToolCall{
		ID:   step.ID,
		Type: "function",
		Function: mcp.ToolCallFunction{
			Name:      step.ToolName,
			Arguments: argsJSON,
		},
	})
}

func (a *Agent) executeReasonStep(ctx context.Context, userID int64, lang, goal string, state ExecutionState, step PlanStep) (string, error) {
	obsJSON, _ := json.Marshal(buildObservationContext(state))
	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerReasonTimeout)
	defer cancel()

	resp, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage("You are the reasoning module for NOFXi. Return one short paragraph only. No markdown, no bullet list."),
			mcp.NewUserMessage(fmt.Sprintf("Language: %s\nGoal: %s\nReasoning task: %s\nObservations JSON: %s\nPersistent preferences: %s\nTask state: %s", lang, goal, step.Instruction, string(obsJSON), a.buildPersistentPreferencesContext(userID), buildTaskStateContext(a.getTaskState(userID)))),
		},
		Ctx: stageCtx,
	})
	if err != nil {
		return "", err
	}
	return summarizeObservation(resp), nil
}

func (a *Agent) generateFinalPlanResponse(ctx context.Context, userID int64, lang string, state ExecutionState, instruction string) (string, error) {
	obsJSON, _ := json.Marshal(buildObservationContext(state))
	systemPrompt := a.buildSystemPrompt(lang)
	if instruction == "" {
		instruction = "Provide the best possible final response to the user based on the finished execution."
	}
	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerFinalTimeout)
	defer cancel()
	return a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewSystemMessage("You are responding after a completed execution plan. Use the observations as the source of truth. Be concise and actionable."),
			mcp.NewUserMessage(fmt.Sprintf("Goal: %s\nResponse instruction: %s\nObservations JSON: %s\nPersistent preferences: %s\nTask state: %s", state.Goal, instruction, string(obsJSON), a.buildPersistentPreferencesContext(userID), buildTaskStateContext(a.getTaskState(userID)))),
		},
		Ctx: stageCtx,
	})
}

func nextPendingStepIndex(steps []PlanStep) int {
	for i := range steps {
		if steps[i].Status == "" || steps[i].Status == planStepStatusPending {
			return i
		}
	}
	return -1
}

func summarizeObservation(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= observationMaxLength {
		return value
	}
	return strings.TrimSpace(value[:observationMaxLength]) + "..."
}

func (a *Agent) thinkAndActLegacy(ctx context.Context, userID int64, lang, text string, onEvent func(event, data string)) (string, error) {
	systemPrompt := a.buildSystemPrompt(lang)
	enrichment := a.gatherContext(text)
	preferencesCtx := a.buildPersistentPreferencesContext(userID)

	userPrompt := text
	if preferencesCtx != "" {
		userPrompt = preferencesCtx + "\n\n---\n" + userPrompt
	}
	if enrichment != "" {
		userPrompt = text + "\n\n---\n[NOFXi System Context - real-time data for reference]\n" + enrichment
		if preferencesCtx != "" {
			userPrompt = preferencesCtx + "\n\n---\n" + userPrompt
		}
	}

	messages := []mcp.Message{mcp.NewSystemMessage(systemPrompt)}
	taskStateCtx := buildTaskStateContext(a.getTaskState(userID))
	if isConfigOrTraderIntent(text) {
		taskStateCtx = ""
	}
	if taskStateCtx != "" {
		messages = append(messages, mcp.NewSystemMessage(taskStateCtx))
	}
	history := a.history.Get(userID)
	if len(history) > 0 {
		history = history[:len(history)-1]
	}
	for _, msg := range history {
		messages = append(messages, mcp.NewMessage(msg.Role, msg.Content))
	}
	messages = append(messages, mcp.NewUserMessage(userPrompt))

	tools := agentTools()

	const maxToolRounds = 5
	for round := 0; round < maxToolRounds; round++ {
		req := &mcp.Request{
			Messages:   messages,
			Tools:      tools,
			ToolChoice: "auto",
			Ctx:        ctx,
		}

		resp, err := a.aiClient.CallWithRequestFull(req)
		if err != nil {
			if round == 0 {
				plainResp, plainErr := a.aiClient.CallWithRequest(&mcp.Request{Messages: messages, Ctx: ctx})
				if plainErr != nil {
					a.logger.Warn("legacy AI plain fallback failed", "error", plainErr, "user_id", userID)
					return a.aiServiceFailure(lang, plainErr)
				}
				if onEvent != nil {
					onEvent(StreamEventDelta, plainResp)
				}
				return plainResp, nil
			}
			a.logger.Warn("legacy AI tool round failed", "error", err, "user_id", userID, "round", round)
			return a.aiServiceFailure(lang, err)
		}

		if len(resp.ToolCalls) == 0 {
			if onEvent != nil {
				onEvent(StreamEventDelta, resp.Content)
			}
			return resp.Content, nil
		}

		assistantMsg := mcp.Message{Role: "assistant", ToolCalls: resp.ToolCalls}
		if resp.Content != "" {
			assistantMsg.Content = resp.Content
		}
		messages = append(messages, assistantMsg)

		for _, tc := range resp.ToolCalls {
			if onEvent != nil {
				onEvent(StreamEventTool, tc.Function.Name)
			}
			result := a.handleToolCall(ctx, storeUserIDFromContext(ctx), userID, lang, tc)
			messages = append(messages, mcp.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	finalResp, err := a.aiClient.CallWithRequest(&mcp.Request{Messages: messages, Ctx: ctx})
	if err != nil {
		a.logger.Warn("legacy AI final response failed", "error", err, "user_id", userID)
		return a.aiServiceFailure(lang, err)
	}
	if onEvent != nil {
		onEvent(StreamEventDelta, finalResp)
	}
	return finalResp, nil
}
