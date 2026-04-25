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
	a.ensureHistory()

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
	if answer, ok := tryInstantDirectReply(lang, text); ok {
		return answer, nil
	}
	if answer, ok := a.tryReadFastPath(storeUserID, userID, lang, text); ok {
		return answer, nil
	}
	if answer, ok, err := a.tryWorkflowIntent(ctx, storeUserID, userID, lang, text, nil); ok || err != nil {
		return answer, err
	}
	if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, nil); ok {
		return answer, nil
	}
	// Check setup flow before falling back to noAI — handles "开始配置", "setup", etc.
	if reply, handled := a.handleSetupFlowForStoreUser(storeUserID, userID, text, lang); handled {
		return reply, nil
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
	if answer, ok := tryInstantDirectReply(lang, text); ok {
		if onEvent != nil {
			onEvent(StreamEventDelta, answer)
		}
		return answer, nil
	}
	if answer, ok := a.tryReadFastPath(storeUserID, userID, lang, text); ok {
		if onEvent != nil {
			onEvent(StreamEventTool, "read_fast_path")
			onEvent(StreamEventDelta, answer)
		}
		return answer, nil
	}
	if answer, ok, err := a.tryWorkflowIntent(ctx, storeUserID, userID, lang, text, onEvent); ok || err != nil {
		return answer, err
	}
	if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent); ok {
		return answer, nil
	}
	// Check setup flow before falling back to noAI — handles "开始配置", "setup", etc.
	if reply, handled := a.handleSetupFlowForStoreUser(storeUserID, userID, text, lang); handled {
		if onEvent != nil {
			onEvent(StreamEventDelta, reply)
		}
		return reply, nil
	}
	if a.aiClient == nil {
		return a.noAIFallback(lang, text)
	}
	return a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
}

func tryInstantDirectReply(lang, text string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return "", false
	}

	zhReplies := map[string]string{
		"hi":     "在，有什么我帮你看的？",
		"hello":  "在，有什么我帮你看的？",
		"hey":    "在，有什么我帮你看的？",
		"你好":     "在，有什么我帮你看的？",
		"嗨":      "在，有什么我帮你看的？",
		"在吗":     "在，有什么我帮你看的？",
		"谢谢":     "不客气。",
		"多谢":     "不客气。",
		"谢了":     "不客气。",
		"ok":     "好。",
		"好的":     "好。",
		"收到":     "好。",
	}
	enReplies := map[string]string{
		"hi":        "I'm here. What should we look at?",
		"hello":     "I'm here. What should we look at?",
		"hey":       "I'm here. What should we look at?",
		"thanks":    "You're welcome.",
		"thank you": "You're welcome.",
		"ok":        "Okay.",
		"okay":      "Okay.",
		"got it":    "Got it.",
	}

	if lang == "zh" {
		if reply, ok := zhReplies[lower]; ok {
			return reply, true
		}
		if reply, ok := enReplies[lower]; ok {
			return reply, true
		}
		return "", false
	}

	if reply, ok := enReplies[lower]; ok {
		return reply, true
	}
	return "", false
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
	if workflow := a.getWorkflowSession(userID); hasActiveWorkflowSession(workflow) {
		answer, handled, err := a.handleWorkflowSession(ctx, storeUserID, userID, lang, text, workflow, onEvent)
		if handled || err != nil {
			return answer, true, err
		}
	}
	if session := a.getSkillSession(userID); strings.TrimSpace(session.Name) != "" {
		switch a.classifySkillSessionInput(ctx, userID, lang, session, text) {
		case "cancel":
			a.clearSkillSession(userID)
			a.clearWorkflowSession(userID)
			if lang == "zh" {
				return "已取消当前流程。", true, nil
			}
			return "Cancelled the current flow.", true, nil
		case "interrupt":
			a.clearSkillSession(userID)
		default:
			if answer, ok := a.tryHardSkill(ctx, storeUserID, userID, lang, text, onEvent); ok {
				return answer, true, nil
			}
		}
	}

	state := a.getExecutionState(userID)
	if hasActiveExecutionState(state) {
		switch classifyExecutionStateInput(state, text) {
		case "cancel":
			a.clearExecutionState(userID)
			if lang == "zh" {
				return "已取消当前流程。", true, nil
			}
			return "Cancelled the current flow.", true, nil
		case "interrupt":
			a.clearExecutionState(userID)
		default:
			answer, err := a.runPlannedAgent(ctx, storeUserID, userID, lang, text, onEvent)
			return answer, true, err
		}
	}

	return "", false, nil
}

func (a *Agent) classifySkillSessionInput(ctx context.Context, userID int64, lang string, session skillSession, text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return "continue"
	}
	if isYesReply(text) || isNoReply(text) {
		return "continue"
	}
	if isExplicitFlowAbort(text) {
		return "cancel"
	}
	if shouldContinueSkillSessionByExpectedSlot(session, text) {
		return "continue"
	}
	if decision := a.classifySkillSessionIntentWithLLM(ctx, userID, lang, session, text); decision != "" {
		return decision
	}
	if isNewSkillRootIntent(session, text) {
		return "interrupt"
	}
	if isSkillFlowDeflection(session, text) {
		return "interrupt"
	}
	if belongsToSkillDomain(session.Name, text) || !looksLikeNewTopLevelIntent(text) {
		return "continue"
	}
	return "interrupt"
}

type skillSessionIntentDecision struct {
	Decision string `json:"decision"`
}

func shouldUseLLMSkillSessionClassifier(session skillSession, text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if isExplicitFlowAbort(text) || isYesReply(text) || isNoReply(text) {
		return false
	}
	if shouldContinueSkillSessionByExpectedSlot(session, text) {
		return false
	}
	return true
}

func shouldContinueSkillSessionByExpectedSlot(session skillSession, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	currentStep, ok := currentSkillDAGStep(session)
	if !ok {
		return false
	}
	switch currentStep.ID {
	case "await_start_confirmation", "await_confirmation":
		return isYesReply(text) || isNoReply(text)
	case "resolve_config_value":
		if fieldValue(session, "config_field") == "selected_timeframes" {
			return timeframeTokenRE.MatchString(strings.ToLower(text))
		}
		return firstIntegerPattern.MatchString(text)
	case "collect_enabled":
		_, ok := parseEnabledValue(text)
		return ok
	case "collect_custom_api_url":
		return extractURL(text) != ""
	case "resolve_exchange_type":
		return exchangeTypeFromText(text) != ""
	case "resolve_provider":
		return providerFromText(text) != ""
	case "resolve_name", "collect_name", "collect_prompt", "collect_account_name", "collect_custom_model_name":
		return !looksLikeNewTopLevelIntent(text)
	}
	for _, field := range currentStep.RequiredFields {
		switch field {
		case "config_value":
			return firstIntegerPattern.MatchString(text)
		case "enabled":
			_, ok := parseEnabledValue(text)
			return ok
		case "custom_api_url":
			return extractURL(text) != ""
		}
	}
	return false
}

func (a *Agent) classifySkillSessionIntentWithLLM(ctx context.Context, userID int64, lang string, session skillSession, text string) string {
	if a == nil || a.aiClient == nil {
		return ""
	}
	if !shouldUseLLMSkillSessionClassifier(session, text) {
		return ""
	}
	currentStep, _ := currentSkillDAGStep(session)
	recentConversationCtx := a.buildRecentConversationContext(userID, text)
	systemPrompt := `You classify one user message while a NOFXi structured management flow is active.
Return JSON only. No markdown.

Possible decisions:
- "continue": the user is still answering the current flow
- "cancel": the user wants to stop the current flow
- "interrupt": the user changed topic, wants diagnosis/query/new task, or should leave the current flow

Be conservative:
- Prefer "continue" only when the message clearly answers the current slot/question.
- Use "cancel" for explicit abandonment like "算了", "不改了", "换话题", "别弄了".
- Use "interrupt" for diagnosis, query, new requests, or topic shifts.`
	userPrompt := fmt.Sprintf(
		"Language: %s\nActive skill: %s\nAction: %s\nCurrent DAG step: %s\nExpected required fields: %s\nUser message: %s\n\nRecent conversation:\n%s",
		lang,
		session.Name,
		session.Action,
		currentStep.ID,
		strings.Join(currentStep.RequiredFields, ", "),
		text,
		recentConversationCtx,
	)
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
		return ""
	}
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var decision skillSessionIntentDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start || json.Unmarshal([]byte(raw[start:end+1]), &decision) != nil {
			return ""
		}
	}
	switch strings.TrimSpace(decision.Decision) {
	case "continue", "cancel", "interrupt":
		return decision.Decision
	default:
		return ""
	}
}

func isSkillFlowDeflection(session skillSession, text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if containsAny(lower, []string{
		"看下报错", "看看报错", "帮我看下报错", "帮我看看报错", "报错怎么回事", "错误怎么回事",
		"换话题", "聊别的", "不是这个", "先说别的", "不聊这个",
	}) {
		return true
	}
	switch strings.TrimSpace(session.Name) {
	case "exchange_management":
		return detectModelDiagnosisSkill(text) || detectTraderDiagnosisSkill(text) || detectStrategyDiagnosisSkill(text)
	case "model_management":
		return detectExchangeDiagnosisSkill(text) || detectTraderDiagnosisSkill(text) || detectStrategyDiagnosisSkill(text)
	case "strategy_management":
		return detectExchangeDiagnosisSkill(text) || detectTraderDiagnosisSkill(text) || detectModelDiagnosisSkill(text)
	case "trader_management":
		return detectExchangeDiagnosisSkill(text) || detectModelDiagnosisSkill(text) || detectStrategyDiagnosisSkill(text)
	default:
		return false
	}
}

func isNewSkillRootIntent(session skillSession, text string) bool {
	currentSkill := strings.TrimSpace(session.Name)
	currentAction := strings.TrimSpace(session.Action)
	if currentSkill == "" {
		return false
	}
	switch currentSkill {
	case "trader_management":
		if detectCreateTraderSkill(text) && currentAction != "create" {
			return true
		}
		if action := normalizeAtomicSkillAction("trader_management", detectManagementAction(text, "trader")); action == "create" && currentAction != "create" {
			return true
		}
	case "strategy_management":
		if action := normalizeAtomicSkillAction("strategy_management", detectManagementAction(text, "strategy")); action == "create" && currentAction != "create" {
			return true
		}
	case "model_management":
		if action := normalizeAtomicSkillAction("model_management", detectManagementAction(text, "model")); action == "create" && currentAction != "create" {
			return true
		}
	case "exchange_management":
		if action := normalizeAtomicSkillAction("exchange_management", detectManagementAction(text, "exchange")); action == "create" && currentAction != "create" {
			return true
		}
	}
	return false
}

func classifyExecutionStateInput(state ExecutionState, text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return "continue"
	}
	if isExplicitFlowAbort(text) {
		return "cancel"
	}
	if isYesReply(text) || isNoReply(text) || shouldResetExecutionStateForNewAttempt(text, state) {
		return "continue"
	}
	if state.Waiting != nil && !looksLikeNewTopLevelIntent(text) {
		return "continue"
	}
	if looksLikeNewTopLevelIntent(text) {
		return "interrupt"
	}
	return "continue"
}

func isExplicitFlowAbort(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if isCancelSkillReply(text) {
		return true
	}
	return containsAny(lower, []string{
		"算了", "先不", "不配了", "别弄了", "不搞了", "先停", "换个话题", "换话题", "聊点别的", "聊别的",
		"stop this", "drop it", "never mind", "forget it", "skip this",
	})
}

func belongsToSkillDomain(skillName, text string) bool {
	switch strings.TrimSpace(skillName) {
	case "trader_management":
		return detectCreateTraderSkill(text) || detectTraderManagementIntent(text) || detectTraderDiagnosisSkill(text)
	case "strategy_management":
		return detectStrategyManagementIntent(text) || detectStrategyDiagnosisSkill(text)
	case "model_management":
		return detectModelManagementIntent(text) || detectModelDiagnosisSkill(text)
	case "exchange_management":
		return detectExchangeManagementIntent(text) || detectExchangeDiagnosisSkill(text)
	default:
		return false
	}
}

func looksLikeNewTopLevelIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if strings.HasPrefix(lower, "/") {
		return true
	}
	if detectCreateTraderSkill(text) ||
		detectTraderManagementIntent(text) ||
		detectExchangeManagementIntent(text) ||
		detectModelManagementIntent(text) ||
		detectStrategyManagementIntent(text) ||
		detectTraderDiagnosisSkill(text) ||
		detectExchangeDiagnosisSkill(text) ||
		detectModelDiagnosisSkill(text) ||
		detectStrategyDiagnosisSkill(text) {
		return true
	}
	if detectReadFastPath(text) != nil {
		return true
	}
	return containsAny(lower, []string{
		"btc", "eth", "sol", "市场", "行情", "余额", "仓位", "持仓", "订单", "账户",
		"price", "market", "balance", "position", "portfolio", "account",
	})
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

	a.ensureHistory()
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
	a.ensureHistory()
	a.history.Add(userID, "user", text)
	if onEvent != nil {
		onEvent(StreamEventPlanning, a.planningStatusText(lang))
	}

	requestStartedAt := time.Now()
	state, err := a.prepareExecutionState(ctx, storeUserID, userID, lang, text)
	if err != nil {
		a.logPlannerTiming("", userID, "prepare_execution_state", requestStartedAt, err)
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
	a.logPlannerTiming(state.SessionID, userID, "prepare_execution_state", requestStartedAt, nil)

	executionStartedAt := time.Now()
	answer, err := a.executePlan(ctx, storeUserID, userID, lang, &state, onEvent)
	a.logPlannerTiming(state.SessionID, userID, "execute_plan", executionStartedAt, err)
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
	a.logPlannerTiming(state.SessionID, userID, "run_planned_agent_total", requestStartedAt, nil)
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
		existing.Steps = completedSteps(existing.Steps)
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
	state.Status = executionStatusRunning
	if err := a.saveExecutionState(state); err != nil {
		return ExecutionState{}, err
	}
	return state, nil
}

type nextStepDecision struct {
	Goal  string     `json:"goal"`
	Steps []PlanStep `json:"steps,omitempty"`
	Step  PlanStep   `json:"step"`
}

func (a *Agent) decideNextStep(ctx context.Context, userID int64, lang string, state ExecutionState) (nextStepDecision, error) {
	toolDefs, _ := json.Marshal(agentTools())
	stateJSON, _ := json.Marshal(normalizeExecutionState(state))
	obsJSON, _ := json.Marshal(buildObservationContext(state))
	recentlyFetchedJSON, _ := json.Marshal(buildRecentlyFetchedData(state, time.Now().UTC()))
	taskStateCtx := buildTaskStateContext(a.getTaskState(userID))
	recentConversationCtx := a.buildRecentConversationContext(userID, state.Goal)

	systemPrompt := `You are the step selector for NOFXi.
Return JSON only. Do not return markdown.

You are operating in ReAct mode: Thought -> Action -> Observation.
Choose the immediate next action batch. Do not generate a long multi-step execution plan.

Allowed step types:
- tool
- reason
- ask_user
- respond

Rules:
- Use all available memory layers: Execution state JSON, Observations JSON, Recent conversation, and Task state.
- Use Recently fetched data JSON as the deduplication source of truth for fresh tool results.
- Prefer the freshest evidence in this order: execution state, observations, recent conversation, then task state.
- If fresh external or system data is needed, choose a tool step.
- If the user is blocked on a missing parameter, choose ask_user.
- If there is enough information to answer now, choose respond.
- Use reason only when a short intermediate synthesis is necessary before the next action.
- Prefer tool or respond over reason whenever possible.
- Never emit the same reason step twice in a row.
- After a reason step, the next batch should usually be tool, ask_user, or respond. Do not stay in analysis loops.
- Never invent tools.
- If the task needs multiple independent tool reads, emit ALL of them together in one response.
- Parallelism rule: when multiple tool reads are mutually independent, do not split them across turns. Return them together in steps.
- Never mix ask_user/respond with additional steps in the same batch.
- Only emit multiple steps when every emitted step is a tool step.
- Avoid repeated tool calls. If a matching tool call already exists in Recently fetched data and age_seconds <= 60, do not call it again unless the user explicitly asks to refresh.
- For tool steps, set tool_name exactly to one available tool and provide tool_args as a JSON object.
- For ask_user or respond steps, put the user-facing question/response instruction in instruction.
- If the latest observation already answers the goal, prefer respond over another tool call.
- Never place a trade unless the user intent is explicit.

Return JSON with this exact shape:
{"goal":"","steps":[{"id":"step_1","type":"tool|reason|ask_user|respond","title":"","tool_name":"","tool_args":{},"instruction":"","requires_confirmation":false}]}`

	userPrompt := fmt.Sprintf("Language: %s\nGoal: %s\n\nRecent conversation:\n%s\n\nAvailable tools JSON:\n%s\n\nPersistent preferences:\n%s\n\nTask state:\n%s\n\nExecution state JSON:\n%s\n\nObservations JSON:\n%s\n\nRecently fetched data JSON:\n%s", lang, state.Goal, recentConversationCtx, string(toolDefs), a.buildPersistentPreferencesContext(userID), taskStateCtx, string(stateJSON), string(obsJSON), string(recentlyFetchedJSON))

	stageCtx, cancel := withPlannerStageTimeout(ctx, plannerCreateTimeout)
	defer cancel()

	startedAt := time.Now()
	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	a.logPlannerTiming(state.SessionID, userID, "decide_next_step_llm", startedAt, err)
	if err != nil {
		return nextStepDecision{}, err
	}
	return parseNextStepDecisionJSON(raw)
}

func parseNextStepDecisionJSON(raw string) (nextStepDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision nextStepDecision
	if err := json.Unmarshal([]byte(raw), &decision); err == nil {
		return normalizeNextStepDecision(decision), nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &decision); err == nil {
			return normalizeNextStepDecision(decision), nil
		}
	}
	return nextStepDecision{}, fmt.Errorf("invalid next step decision json")
}

func normalizeNextStepDecision(decision nextStepDecision) nextStepDecision {
	decision.Goal = strings.TrimSpace(decision.Goal)
	steps := decision.Steps
	if len(steps) == 0 && decision.Step.Type != "" {
		steps = []PlanStep{decision.Step}
	}
	if len(steps) > 0 {
		steps = normalizeExecutionState(ExecutionState{Steps: steps}).Steps
	}
	decision.Steps = steps
	if len(steps) > 0 {
		decision.Step = steps[0]
	}
	return decision
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

	startedAt := time.Now()
	resp, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx: stageCtx,
	})
	a.logPlannerTiming(state.SessionID, userID, "create_execution_plan_llm", startedAt, err)
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
	if onEvent != nil && len(state.Steps) > 0 {
		onEvent(StreamEventPlan, formatPlanStatus(*state, lang))
	}

	for i := 0; i < plannerMaxIterations; i++ {
		stepIndex := nextPendingStepIndex(state.Steps)
		if stepIndex < 0 {
			decisionStartedAt := time.Now()
			decision, err := a.decideNextStep(ctx, userID, lang, *state)
			a.logPlannerTiming(state.SessionID, userID, "decide_next_step", decisionStartedAt, err)
			if err != nil {
				return "", err
			}
			steps := filterFreshDuplicateToolSteps(decision.Steps, *state, time.Now().UTC())
			if len(steps) == 0 {
				appendExecutionLog(state, Observation{
					Kind:      "decision_note",
					Summary:   "Skipped duplicate fresh tool calls from next-step decision",
					CreatedAt: time.Now().UTC().Format(time.RFC3339),
				})
				state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				if err := a.saveExecutionState(*state); err != nil {
					return "", err
				}
				continue
			}
			if hasRepeatedReasonLoop(*state, steps) {
				return "", fmt.Errorf("repeated reasoning loop detected")
			}
			if decision.Goal != "" {
				state.Goal = decision.Goal
			}
			base := len(completedSteps(state.Steps))
			for idx := range steps {
				if steps[idx].Type == "" {
					return "", fmt.Errorf("next step decision missing step type")
				}
				if steps[idx].ID == "" {
					steps[idx].ID = fmt.Sprintf("step_%d", base+idx+1)
				}
				if steps[idx].Title == "" {
					steps[idx].Title = strings.ReplaceAll(steps[idx].ID, "_", " ")
				}
				if steps[idx].Status == "" {
					steps[idx].Status = planStepStatusPending
				}
			}
			state.Steps = append(completedSteps(state.Steps), steps...)
			state.Status = executionStatusRunning
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := a.saveExecutionState(*state); err != nil {
				return "", err
			}
			if onEvent != nil {
				onEvent(StreamEventPlan, formatPlanStatus(*state, lang))
			}
			continue
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
			stepStartedAt := time.Now()
			result := a.executePlanTool(ctx, storeUserID, userID, lang, *step)
			a.logPlannerTiming(state.SessionID, userID, "tool:"+step.ToolName, stepStartedAt, nil)
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
			if referencesChanged {
				a.log().Info("tool step updated references", "tool", step.ToolName, "session", state.SessionID)
			}
		case planStepTypeReason:
			reasonStartedAt := time.Now()
			reasoning, err := a.executeReasonStep(ctx, userID, lang, state.Goal, *state, *step)
			a.logPlannerTiming(state.SessionID, userID, "reason_step", reasonStartedAt, err)
			if err != nil {
				step.Status = planStepStatusFailed
				step.Error = err.Error()
				state.Status = executionStatusFailed
				state.LastError = err.Error()
				if saveErr := a.saveExecutionState(*state); saveErr != nil {
					a.log().Warn("failed to save execution state after reason step error", "error", saveErr)
				}
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
			respondStartedAt := time.Now()
			finalText, err := a.generateFinalPlanResponse(ctx, userID, lang, *state, step.Instruction)
			a.logPlannerTiming(state.SessionID, userID, "respond_step", respondStartedAt, err)
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

type fetchedToolRecord struct {
	ToolName     string `json:"tool_name"`
	ToolArgsJSON string `json:"tool_args_json"`
	FetchedAt    string `json:"fetched_at"`
	AgeSeconds   int64  `json:"age_seconds"`
}

func buildRecentlyFetchedData(state ExecutionState, now time.Time) []fetchedToolRecord {
	state = normalizeExecutionState(state)
	stepByID := make(map[string]PlanStep, len(state.Steps))
	for _, step := range state.Steps {
		stepByID[step.ID] = step
	}
	latest := map[string]fetchedToolRecord{}
	for _, obs := range state.ExecutionLog {
		if obs.Kind != "tool_result" {
			continue
		}
		step, ok := stepByID[obs.StepID]
		if !ok || step.ToolName == "" {
			continue
		}
		sig := toolCallSignature(step.ToolName, step.ToolArgs)
		createdAt := parseRFC3339(obs.CreatedAt)
		record := fetchedToolRecord{
			ToolName:     step.ToolName,
			ToolArgsJSON: toolArgsJSONString(step.ToolArgs),
			FetchedAt:    obs.CreatedAt,
			AgeSeconds:   int64(now.Sub(createdAt).Seconds()),
		}
		prev, exists := latest[sig]
		if !exists || prev.FetchedAt < record.FetchedAt {
			latest[sig] = record
		}
	}
	out := make([]fetchedToolRecord, 0, len(latest))
	for _, record := range latest {
		if record.AgeSeconds < 0 {
			record.AgeSeconds = 0
		}
		out = append(out, record)
	}
	return out
}

func filterFreshDuplicateToolSteps(steps []PlanStep, state ExecutionState, now time.Time) []PlanStep {
	if len(steps) == 0 {
		return nil
	}
	fresh := make(map[string]struct{})
	for _, item := range buildRecentlyFetchedData(state, now) {
		if item.AgeSeconds <= 60 {
			fresh[item.ToolName+"|"+item.ToolArgsJSON] = struct{}{}
		}
	}
	out := make([]PlanStep, 0, len(steps))
	for _, step := range steps {
		if step.Type != planStepTypeTool {
			out = append(out, step)
			continue
		}
		sig := toolCallSignature(step.ToolName, step.ToolArgs)
		if _, ok := fresh[sig]; ok {
			continue
		}
		fresh[sig] = struct{}{}
		out = append(out, step)
	}
	return out
}

func hasRepeatedReasonLoop(state ExecutionState, steps []PlanStep) bool {
	if len(steps) == 0 {
		return false
	}
	last := lastCompletedStep(state.Steps)
	if last == nil || last.Type != planStepTypeReason {
		return false
	}
	for _, step := range steps {
		if step.Type != planStepTypeReason {
			return false
		}
		if stepSemanticKey(*last) != stepSemanticKey(step) {
			return false
		}
	}
	return true
}

func lastCompletedStep(steps []PlanStep) *PlanStep {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Status == planStepStatusCompleted {
			return &steps[i]
		}
	}
	return nil
}

func stepSemanticKey(step PlanStep) string {
	return strings.ToLower(strings.TrimSpace(
		step.Type + "|" + step.ToolName + "|" + step.Title + "|" + step.Instruction,
	))
}

func toolCallSignature(toolName string, args map[string]any) string {
	return strings.TrimSpace(toolName) + "|" + toolArgsJSONString(args)
}

func toolArgsJSONString(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	data, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func parseRFC3339(value string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return t
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

	startedAt := time.Now()
	raw, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewUserMessage(userPrompt),
		},
		Ctx:       stageCtx,
		MaxTokens: intPtr(500),
	})
	a.logPlannerTiming(state.SessionID, userID, "replan_after_step_llm", startedAt, err)
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

	startedAt := time.Now()
	resp, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage("You are the reasoning module for NOFXi. Return one short paragraph only. No markdown, no bullet list."),
			mcp.NewUserMessage(fmt.Sprintf("Language: %s\nGoal: %s\nReasoning task: %s\nObservations JSON: %s\nPersistent preferences: %s\nTask state: %s", lang, goal, step.Instruction, string(obsJSON), a.buildPersistentPreferencesContext(userID), buildTaskStateContext(a.getTaskState(userID)))),
		},
		Ctx: stageCtx,
	})
	a.logPlannerTiming(state.SessionID, userID, "reason_step_llm", startedAt, err)
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
	startedAt := time.Now()
	resp, err := a.aiClient.CallWithRequest(&mcp.Request{
		Messages: []mcp.Message{
			mcp.NewSystemMessage(systemPrompt),
			mcp.NewSystemMessage("You are responding after a completed execution plan. Use the observations as the source of truth. Be concise and actionable."),
			mcp.NewUserMessage(fmt.Sprintf("Goal: %s\nResponse instruction: %s\nObservations JSON: %s\nPersistent preferences: %s\nTask state: %s", state.Goal, instruction, string(obsJSON), a.buildPersistentPreferencesContext(userID), buildTaskStateContext(a.getTaskState(userID)))),
		},
		Ctx: stageCtx,
	})
	a.logPlannerTiming(state.SessionID, userID, "generate_final_response_llm", startedAt, err)
	return resp, err
}

func (a *Agent) logPlannerTiming(sessionID string, userID int64, stage string, startedAt time.Time, err error) {
	if stage == "" || startedAt.IsZero() {
		return
	}
	attrs := []any{
		"session_id", sessionID,
		"user_id", userID,
		"stage", stage,
		"elapsed_ms", time.Since(startedAt).Milliseconds(),
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	a.log().Info("planner timing", attrs...)
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
