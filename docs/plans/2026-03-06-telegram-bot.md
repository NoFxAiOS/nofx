# Telegram Bot Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 NOFX 单进程内内置 Telegram Bot，用户通过自然语言（LLM 解析意图）在 Telegram 配置策略、交易所、大模型、交易员、查询持仓、控制交易。

**Architecture:** 新增 `telegram/` 包，单一 Facade 层（`service/nofx.go`）作为唯一接触 NOFX 内部的边界，借鉴 openclaw compaction 模式实现多轮对话记忆压缩，`main.go` 仅增加 3 行。

**Tech Stack:** Go, `github.com/go-telegram-bot-api/telegram-bot-api/v5`（已在 go.mod）, `nofx/mcp`（复用现有 LLM 客户端）

---

## 监工修正（Claude 开始前先读）

这份文档里的代码块只能当伪代码参考，**不能直接照抄**。当前仓库真实接口和文档示例存在多处偏差，首轮实现必须以编译通过的仓库接口为准。

### 真实接口约束

1. `manager.TraderManager` **没有** `StartTrader` / `StopTrader` 方法。
   - Telegram 启停交易员时，必须复用现有 API Server 的流程语义：
   - 启动：校验归属 -> 移除已停止的内存实例 -> `LoadUserTradersFromStore()` -> `GetTrader()` -> `go trader.Run()` -> `store.Trader().UpdateStatus(userID, traderID, true)`
   - 停止：`GetTrader()` -> 检查 `GetStatus()["is_running"]` -> `Stop()` -> `UpdateStatus(..., false)`

2. `store` 方法签名与文档示例不一致，必须按真实接口实现：
   - `store.Trader().List(userID)` 返回 `[]*store.Trader`
   - `store.Trader()` 没有 `Get(traderID)`，常用的是 `GetFullConfig(userID, traderID)`
   - `store.Strategy().Get(userID, id string)`，`Strategy.ID` 是 `string`，不是 `uint`
   - `store.AIModel().Create(...)` 返回 `error`，不是 `*store.AIModel`
   - `store.Exchange().Create(...)` 返回 `(string, error)`，不是 `*store.Exchange`
   - `store.Exchange()` 读单条配置用 `GetByID(userID, id)`
   - `store.Equity()` 没有 `Latest`，现有方法是 `GetLatest(traderID, limit)`
   - `store.Position()` 没有 `ListByTrader`

3. `mcp.New()` 在当前仓库中不存在。
   - 必须使用已有构造器，例如 `mcp.NewDeepSeekClient()`、`mcp.NewClient(...)`，或新增一个显式 helper。

4. 策略创建不能直接拼一个“猜测字段”的 JSON。
   - 当前真实结构是 `store.StrategyConfig`
   - 首选做法：从 `store.GetDefaultStrategyConfig("zh")` 起步，修改需要的字段，再 `json.Marshal`
   - `Strategy.ID` 需要像现有 API 一样使用 `uuid.New().String()`

5. “修改策略 Prompt” 不能按文档示例那样直接改 `Strategy.CustomPrompt`。
   - `store.Strategy` 没有这个顶层字段
   - 真实做法应是：读取 `strategy.Config` -> `ParseConfig()` -> 更新 `StrategyConfig.CustomPrompt` 或相关 prompt section -> 序列化回 `strategy.Config` -> `Update(strategy)`

6. `/start` 的“完全重置”与当前伪代码冲突。
   - 现在 `Memory.Reset()` 只清空短期历史，不清空长期摘要
   - 如果 `/start` 要“重置会话”，就必须新增 `ClearAll()` 或重建 `Memory`

7. 不要在 Telegram 回复里默认启用 `Markdown` parse mode。
   - 用户输入、策略名、API key、交易对等都可能包含 Markdown 特殊字符
   - 首版建议纯文本回复，稳定后再做 escape

8. 不要在日志、回复、错误信息中回显敏感字段。
   - `api_key`
   - `secret_key`
   - `passphrase`
   - 私钥或钱包密钥

### 首轮交付范围（必须收敛）

首个可交付版本只做“最小可用闭环”，不要一口气把所有写操作做满：

1. 必做：
   - Telegram Bot 启动
   - 管理员 chat ID 鉴权
   - `/start` 重置会话
   - 会话管理
   - LLM 意图解析
   - 只读查询：`list traders` / `query positions` / `query equity`
   - 控制：`start trader` / `stop trader`

2. 第二阶段再做：
   - `config_strategy`
   - `config_exchange`
   - `config_model`
   - `config_trader`
   - `update_prompt`

3. `control_close` 先不要做，除非先找到仓库里现成且安全的平仓入口。

### 硬性门禁

1. 每个子任务至少过 `go build ./telegram/...`
2. 合并前必须过 `go build ./...`
3. `handler/` 不允许直接碰 `store/` 或 `manager/`
4. 所有跨层访问都只能从 `telegram/service/nofx.go` 进入
5. 任何伪代码字段名、方法名、返回值，在落地前都必须先对照真实仓库接口

---

## 文件结构

```
telegram/
├── bot.go                  # 新建：Bot 启动、消息收发路由
├── session/
│   ├── session.go          # 新建：会话状态（当前意图、进度）
│   └── memory.go           # 新建：对话记忆 + 自动压缩
├── intent/
│   └── parser.go           # 新建：LLM 意图解析
├── service/
│   └── nofx.go             # 新建：Facade（唯一接触 store/manager 的地方）
└── handler/
    └── handler.go          # 新建：业务路由，只调 service/ 和 intent/

config/config.go            # 修改：加 TelegramBotToken, TelegramAdminChatID
main.go                     # 修改：加 3 行启动 Telegram Bot
```

---

### Task 1: 扩展 Config

**Files:**
- Modify: `config/config.go`

**Step 1: 在 Config struct 末尾加两个字段**

```go
// Telegram Bot configuration
TelegramBotToken    string // TELEGRAM_BOT_TOKEN
TelegramAdminChatID int64  // TELEGRAM_ADMIN_CHAT_ID (only this user can operate)
```

**Step 2: 在 Init() 函数的解析段加读取逻辑**

找到 Init() 函数中 os.Getenv 的模式，加：

```go
cfg.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
if chatIDStr := os.Getenv("TELEGRAM_ADMIN_CHAT_ID"); chatIDStr != "" {
    if id, err := strconv.ParseInt(chatIDStr, 10, 64); err == nil {
        cfg.TelegramAdminChatID = id
    }
}
```

**监工补充：** `Init()` 函数里当前一直在填充局部变量 `cfg`，最后才赋值给 `global`，这里不能提前写 `global.TelegramBotToken`

**Step 3: 构建验证**

```bash
cd /Users/yida/gopro/open-nofx && go build ./...
```

Expected: 无错误

**Step 4: Commit**

```bash
git add config/config.go
git commit -m "feat(telegram): add TelegramBotToken and TelegramAdminChatID to config"
```

---

### Task 2: Facade 层 telegram/service/nofx.go

**Files:**
- Create: `telegram/service/nofx.go`

这是**唯一**接触 NOFX 内部（store、manager）的文件。handler 不直接碰 store/manager。

**Step 1: 创建文件**

```go
package service

import (
	"fmt"
	"nofx/manager"
	"nofx/store"
)

// NofxService is the single facade between Telegram bot and NOFX internals.
// All store/manager access MUST go through this layer.
type NofxService struct {
	store   *store.Store
	manager *manager.TraderManager
	userID  string // fixed user ID for single-user mode: "default"
}

func New(st *store.Store, tm *manager.TraderManager) *NofxService {
	return &NofxService{store: st, manager: tm, userID: "default"}
}

// --- Trader ---

func (s *NofxService) ListTraders() ([]store.Trader, error) {
	return s.store.Trader().List(s.userID)
}

func (s *NofxService) StartTrader(traderID string) error {
	t, err := s.store.Trader().Get(traderID)
	if err != nil {
		return fmt.Errorf("trader not found: %w", err)
	}
	return s.manager.StartTrader(t, s.store)
}

func (s *NofxService) StopTrader(traderID string) error {
	return s.manager.StopTrader(traderID)
}

// --- Strategy ---

func (s *NofxService) ListStrategies() ([]store.Strategy, error) {
	return s.store.Strategy().List(s.userID)
}

func (s *NofxService) CreateStrategy(name string, configJSON string) (*store.Strategy, error) {
	strategy := &store.Strategy{
		UserID: s.userID,
		Name:   name,
		Config: configJSON,
	}
	if err := s.store.Strategy().Create(strategy); err != nil {
		return nil, err
	}
	return strategy, nil
}

func (s *NofxService) UpdateStrategyPrompt(strategyID uint, prompt string) error {
	strategy, err := s.store.Strategy().Get(strategyID)
	if err != nil {
		return err
	}
	strategy.CustomPrompt = prompt
	return s.store.Strategy().Update(strategy)
}

// --- AI Model ---

func (s *NofxService) ListModels() ([]store.AIModel, error) {
	return s.store.AIModel().List(s.userID)
}

func (s *NofxService) CreateModel(provider, apiKey, model string) (*store.AIModel, error) {
	m := &store.AIModel{
		UserID:   s.userID,
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
	}
	if err := s.store.AIModel().Create(m); err != nil {
		return nil, err
	}
	return m, nil
}

// --- Exchange ---

func (s *NofxService) ListExchanges() ([]store.Exchange, error) {
	return s.store.Exchange().List(s.userID)
}

func (s *NofxService) CreateExchange(exchangeType, apiKey, secretKey string) (*store.Exchange, error) {
	ex := &store.Exchange{
		UserID:       s.userID,
		ExchangeType: exchangeType,
		APIKey:       apiKey,
		SecretKey:    secretKey,
	}
	if err := s.store.Exchange().Create(ex); err != nil {
		return nil, err
	}
	return ex, nil
}

// --- Positions / Query ---

func (s *NofxService) GetPositions(traderID string) ([]store.TraderPosition, error) {
	return s.store.Position().ListByTrader(traderID)
}

func (s *NofxService) GetEquitySummary(traderID string) (*store.EquitySnapshot, error) {
	return s.store.Equity().Latest(traderID)
}
```

**Step 2: 注意事项**

store 的方法名称（List、Get、Create、Update）需要根据实际 store 接口调整。运行 `go build ./telegram/...` 后根据编译错误逐一对齐方法名。

**监工补充：这一节不能照抄上面的示例实现，至少要修正以下事实**

- `ListTraders()` / `ListStrategies()` / `ListModels()` / `ListExchanges()` 的返回值都应与真实 store 一致，当前仓库大多是指针切片
- `StartTrader()` / `StopTrader()` 不能调用不存在的 `manager` 方法，必须镜像 `api/server.go` 的启动/停止流程
- `CreateStrategy()` 不能假设 `Strategy.ID` 是整数；请复用现有 API 的 `uuid.New().String()` 方案
- `CreateModel()` / `CreateExchange()` 不能假设 store 会返回新建对象；真实接口要么返回 `error`，要么返回 `(id, error)`
- `GetPositions()` / `GetEquitySummary()` 需要在 `service` 内封装真实查询逻辑，不能调用仓库中不存在的 `ListByTrader()` / `Latest()`

**Step 3: Build 验证**

```bash
cd /Users/yida/gopro/open-nofx && go build ./telegram/...
```

Expected: 只可能有 store 方法名不匹配的错误，逐一修正即可。

**Step 4: Commit**

```bash
git add telegram/service/nofx.go
git commit -m "feat(telegram): add NofxService facade layer"
```

---

### Task 3: 会话记忆 telegram/session/memory.go

**Files:**
- Create: `telegram/session/memory.go`

借鉴 openclaw compaction 模式：token 超阈值 → LLM 静默压缩 → 写入长期记忆 → 清空短期历史。

**Step 1: 创建文件**

```go
package session

import (
	"fmt"
	"nofx/mcp"
	"strings"
)

const (
	// When short-term history exceeds this token estimate, trigger compaction
	compactionThresholdTokens = 3000
	// Rough estimate: 1 token ≈ 4 chars (Chinese ~2 chars/token)
	charsPerToken = 3
)

// Message represents a single conversation turn
type Message struct {
	Role    string // "user" or "assistant"
	Content string
}

// Memory manages conversation history with automatic compaction.
// Inspired by openclaw's compaction pattern.
type Memory struct {
	LongTerm  string    // Durable summary (survives compaction)
	ShortTerm []Message // Recent conversation (cleared on compaction)
	llm       mcp.AIClient
}

func NewMemory(llm mcp.AIClient) *Memory {
	return &Memory{llm: llm}
}

// Add appends a message and triggers compaction if needed
func (m *Memory) Add(role, content string) {
	m.ShortTerm = append(m.ShortTerm, Message{Role: role, Content: content})
	if m.estimateTokens() > compactionThresholdTokens {
		m.compact()
	}
}

// BuildContext returns context string for LLM intent parsing
func (m *Memory) BuildContext() string {
	var sb strings.Builder
	if m.LongTerm != "" {
		sb.WriteString("【历史摘要】\n")
		sb.WriteString(m.LongTerm)
		sb.WriteString("\n\n")
	}
	if len(m.ShortTerm) > 0 {
		sb.WriteString("【近期对话】\n")
		for _, msg := range m.ShortTerm {
			sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
		}
	}
	return sb.String()
}

// Reset clears session (called on /start or new session)
func (m *Memory) Reset() {
	m.ShortTerm = []Message{}
	// LongTerm is preserved intentionally
}

func (m *Memory) estimateTokens() int {
	total := len(m.LongTerm)
	for _, msg := range m.ShortTerm {
		total += len(msg.Content)
	}
	return total / charsPerToken
}

// compact summarizes short-term history into long-term memory (silent, user doesn't see this)
func (m *Memory) compact() {
	if m.llm == nil || len(m.ShortTerm) == 0 {
		return
	}

	history := m.BuildContext()
	systemPrompt := `你是一个对话摘要助手。将以下交易配置对话压缩为简洁摘要。

必须保留：
- 用户正在配置什么（策略/交易所/大模型/交易员）
- 已确认的参数（交易对、杠杆、止损比例、指标等）
- 待确认或缺失的参数
- 用户表达的偏好和要求

输出格式：纯文本摘要，不超过200字。`

	summary, err := m.llm.CallWithMessages(systemPrompt, history)
	if err != nil {
		// Compaction failed: keep short-term as-is, don't lose data
		return
	}

	// Write summary to long-term, clear short-term
	if m.LongTerm != "" {
		m.LongTerm = m.LongTerm + "\n" + summary
	} else {
		m.LongTerm = summary
	}
	m.ShortTerm = []Message{}
}
```

**Step 2: Build 验证**

```bash
cd /Users/yida/gopro/open-nofx && go build ./telegram/...
```

**Step 3: Commit**

```bash
git add telegram/session/memory.go
git commit -m "feat(telegram): add conversation memory with openclaw-style compaction"
```

---

### Task 4: 会话状态 telegram/session/session.go

**Files:**
- Create: `telegram/session/session.go`

**Step 1: 创建文件**

```go
package session

import (
	"nofx/mcp"
	"sync"
	"time"
)

// Intent represents what the user is currently trying to do
type Intent string

const (
	IntentNone            Intent = ""
	IntentConfigStrategy  Intent = "config_strategy"
	IntentConfigExchange  Intent = "config_exchange"
	IntentConfigModel     Intent = "config_model"
	IntentConfigTrader    Intent = "config_trader"
	IntentQueryPositions  Intent = "query_positions"
	IntentControlTrader   Intent = "control_trader"
	IntentUpdatePrompt    Intent = "update_prompt"
)

// Session holds state for a single Telegram conversation
type Session struct {
	ChatID    int64
	Intent    Intent
	Params    map[string]string // collected parameters so far
	Memory    *Memory
	UpdatedAt time.Time
}

// Manager manages all active sessions (one per chat ID)
type Manager struct {
	mu       sync.RWMutex
	sessions map[int64]*Session
	llm      mcp.AIClient
}

func NewManager(llm mcp.AIClient) *Manager {
	return &Manager{
		sessions: make(map[int64]*Session),
		llm:      llm,
	}
}

// Get returns or creates a session for the given chat ID
func (m *Manager) Get(chatID int64) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[chatID]
	if !ok {
		s = &Session{
			ChatID:    chatID,
			Intent:    IntentNone,
			Params:    make(map[string]string),
			Memory:    NewMemory(m.llm),
			UpdatedAt: time.Now(),
		}
		m.sessions[chatID] = s
	}
	s.UpdatedAt = time.Now()
	return s
}

// Reset clears session intent and params (keeps memory)
func (s *Session) Reset() {
	s.Intent = IntentNone
	s.Params = make(map[string]string)
}

// ResetFull clears everything including memory (on /start command)
func (s *Session) ResetFull() {
	s.Reset()
	s.Memory.Reset()
}
```

**监工补充：这里的伪代码与注释不一致**

- 当前 `Memory.Reset()` 只清空短期历史，不会清空 `LongTerm`
- 如果 `/start` 的产品语义是“完全重置”，这里必须改成真正清空长期摘要，或者直接新建一个 `Memory`

**Step 2: Build 验证**

```bash
cd /Users/yida/gopro/open-nofx && go build ./telegram/...
```

**Step 3: Commit**

```bash
git add telegram/session/session.go
git commit -m "feat(telegram): add session state manager"
```

---

### Task 5: LLM 意图解析 telegram/intent/parser.go

**Files:**
- Create: `telegram/intent/parser.go`

复用 `nofx/mcp` 的现有 LLM 客户端，不引入新依赖。

**Step 1: 创建文件**

```go
package intent

import (
	"encoding/json"
	"nofx/mcp"
	"strings"
)

// ParsedIntent is the structured output from LLM intent parsing
type ParsedIntent struct {
	Action  string            `json:"action"`  // e.g. "config_strategy", "query_positions"
	Params  map[string]string `json:"params"`  // extracted parameters
	Missing []string          `json:"missing"` // params still needed
	Reply   string            `json:"reply"`   // what bot should say to user
}

const systemPrompt = `你是 NOFX 交易系统的对话助手。分析用户消息，提取交易配置意图和参数。

支持的操作（action）：
- config_strategy: 创建/修改策略（需要：name, coins, indicators, max_position_pct, stop_loss_pct）
- config_exchange: 配置交易所（需要：exchange_type, api_key, secret_key）
- config_model: 配置大模型（需要：provider, api_key, model）
- config_trader: 配置交易员（需要：name, model_id, exchange_id, strategy_id）
- query_positions: 查询持仓（需要：trader_id 或 "all"）
- query_equity: 查询账户余额/盈亏
- control_start: 启动交易员（需要：trader_id 或 trader_name）
- control_stop: 停止交易员（需要：trader_id 或 trader_name）
- control_close: 紧急平仓（需要：trader_id, symbol）
- update_prompt: 修改策略 Prompt（需要：strategy_id 或 strategy_name, prompt）
- unknown: 无法识别

输出严格 JSON 格式：
{
  "action": "action_name",
  "params": {"key": "value"},
  "missing": ["param1", "param2"],
  "reply": "对用户的回复（询问缺失参数或确认操作）"
}

安全要求：API Key 等敏感信息原样保留在 params 中，不要截断或修改。`

// Parser uses LLM to parse user message into structured intent
type Parser struct {
	llm mcp.AIClient
}

func NewParser(llm mcp.AIClient) *Parser {
	return &Parser{llm: llm}
}

// Parse sends user message + conversation context to LLM, returns structured intent
func (p *Parser) Parse(userMessage, conversationContext string) (*ParsedIntent, error) {
	userPrompt := userMessage
	if conversationContext != "" {
		userPrompt = conversationContext + "\n\n【当前消息】\n" + userMessage
	}

	resp, err := p.llm.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Extract JSON from response (LLM may wrap in markdown code block)
	jsonStr := extractJSON(resp)

	var result ParsedIntent
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Fallback: return unknown intent with raw response as reply
		return &ParsedIntent{
			Action: "unknown",
			Reply:  "抱歉，我没有理解你的意思。请描述你想做什么，例如：「帮我创建一个 BTC 策略」",
		}, nil
	}
	return &result, nil
}

func extractJSON(s string) string {
	// Strip markdown code block if present
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "```json"); idx >= 0 {
		s = s[idx+7:]
	} else if idx := strings.Index(s, "```"); idx >= 0 {
		s = s[idx+3:]
	}
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	// Find first { to last }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
```

**Step 2: Build 验证**

```bash
cd /Users/yida/gopro/open-nofx && go build ./telegram/...
```

**Step 3: Commit**

```bash
git add telegram/intent/parser.go
git commit -m "feat(telegram): add LLM intent parser"
```

---

### Task 6: 业务处理 telegram/handler/handler.go

**Files:**
- Create: `telegram/handler/handler.go`

handler 只调 service/ 和 intent/，不直接碰 store/manager。

**Step 1: 创建文件**

```go
package handler

import (
	"fmt"
	"nofx/telegram/intent"
	"nofx/telegram/service"
	"nofx/telegram/session"
	"strings"
)

// Handler dispatches parsed intents to the right operation
type Handler struct {
	svc     *service.NofxService
	parser  *intent.Parser
	sessions *session.Manager
}

func New(svc *service.NofxService, parser *intent.Parser, sessions *session.Manager) *Handler {
	return &Handler{svc: svc, parser: parser, sessions: sessions}
}

// Handle processes a user message and returns the bot reply
func (h *Handler) Handle(chatID int64, userMessage string) string {
	sess := h.sessions.Get(chatID)

	// Record user message in memory
	sess.Memory.Add("user", userMessage)

	// Build conversation context for LLM
	ctx := sess.Memory.BuildContext()

	// Parse intent via LLM
	parsed, err := h.parser.Parse(userMessage, ctx)
	if err != nil {
		return "❌ 解析失败，请重试"
	}

	// Merge newly extracted params into session
	for k, v := range parsed.Params {
		sess.Params[k] = v
	}

	// If there are missing params, ask user
	if len(parsed.Missing) > 0 {
		sess.Intent = session.Intent(parsed.Action)
		reply := parsed.Reply
		sess.Memory.Add("assistant", reply)
		return reply
	}

	// Execute the action
	reply := h.execute(sess, parsed)
	sess.Memory.Add("assistant", reply)
	sess.Reset() // clear intent after successful execution
	return reply
}

func (h *Handler) execute(sess *session.Session, parsed *intent.ParsedIntent) string {
	params := sess.Params

	switch parsed.Action {
	case "config_strategy":
		return h.createStrategy(params)

	case "config_exchange":
		return h.createExchange(params)

	case "config_model":
		return h.createModel(params)

	case "query_positions":
		return h.queryPositions(params)

	case "query_equity":
		return h.queryEquity(params)

	case "control_start":
		return h.startTrader(params)

	case "control_stop":
		return h.stopTrader(params)

	case "update_prompt":
		return h.updatePrompt(params)

	default:
		return parsed.Reply
	}
}

func (h *Handler) createStrategy(params map[string]string) string {
	name := params["name"]
	if name == "" {
		name = "我的策略"
	}
	// Build a minimal strategy config JSON from params
	// Full StrategyConfig is complex; we start with essential fields
	configJSON := buildStrategyConfigJSON(params)
	strategy, err := h.svc.CreateStrategy(name, configJSON)
	if err != nil {
		return fmt.Sprintf("❌ 创建策略失败: %v", err)
	}
	return fmt.Sprintf("✅ 策略「%s」已创建（ID: %d）\n\n配置摘要：\n%s", strategy.Name, strategy.ID, formatParams(params))
}

func (h *Handler) createExchange(params map[string]string) string {
	exType := params["exchange_type"]
	apiKey := params["api_key"]
	secretKey := params["secret_key"]
	ex, err := h.svc.CreateExchange(exType, apiKey, secretKey)
	if err != nil {
		return fmt.Sprintf("❌ 配置交易所失败: %v", err)
	}
	return fmt.Sprintf("✅ %s 交易所已配置（ID: %d）", ex.ExchangeType, ex.ID)
}

func (h *Handler) createModel(params map[string]string) string {
	provider := params["provider"]
	apiKey := params["api_key"]
	model := params["model"]
	m, err := h.svc.CreateModel(provider, apiKey, model)
	if err != nil {
		return fmt.Sprintf("❌ 配置大模型失败: %v", err)
	}
	return fmt.Sprintf("✅ %s (%s) 已配置（ID: %d）", m.Provider, m.Model, m.ID)
}

func (h *Handler) queryPositions(params map[string]string) string {
	traderID := params["trader_id"]
	if traderID == "" {
		traders, err := h.svc.ListTraders()
		if err != nil || len(traders) == 0 {
			return "❌ 没有找到交易员"
		}
		traderID = traders[0].ID
	}
	positions, err := h.svc.GetPositions(traderID)
	if err != nil {
		return fmt.Sprintf("❌ 查询持仓失败: %v", err)
	}
	if len(positions) == 0 {
		return "📭 当前无持仓"
	}
	var sb strings.Builder
	sb.WriteString("📊 当前持仓：\n")
	for _, p := range positions {
		sb.WriteString(fmt.Sprintf("• %s %s | 入场: %.4f | 未实现P&L: %.2f USDT\n",
			p.Symbol, p.Side, p.EntryPrice, p.UnrealizedPnl))
	}
	return sb.String()
}

func (h *Handler) queryEquity(params map[string]string) string {
	traders, err := h.svc.ListTraders()
	if err != nil || len(traders) == 0 {
		return "❌ 没有找到交易员"
	}
	traderID := params["trader_id"]
	if traderID == "" {
		traderID = traders[0].ID
	}
	eq, err := h.svc.GetEquitySummary(traderID)
	if err != nil {
		return fmt.Sprintf("❌ 查询余额失败: %v", err)
	}
	return fmt.Sprintf("💰 账户余额：%.2f USDT", eq.TotalBalance)
}

func (h *Handler) startTrader(params map[string]string) string {
	traderID := params["trader_id"]
	if err := h.svc.StartTrader(traderID); err != nil {
		return fmt.Sprintf("❌ 启动失败: %v", err)
	}
	return "✅ 交易员已启动"
}

func (h *Handler) stopTrader(params map[string]string) string {
	traderID := params["trader_id"]
	if err := h.svc.StopTrader(traderID); err != nil {
		return fmt.Sprintf("❌ 停止失败: %v", err)
	}
	return "✅ 交易员已停止"
}

func (h *Handler) updatePrompt(params map[string]string) string {
	// strategy_id must be numeric; convert from params
	strategyIDStr := params["strategy_id"]
	var strategyID uint
	fmt.Sscanf(strategyIDStr, "%d", &strategyID)
	prompt := params["prompt"]
	if err := h.svc.UpdateStrategyPrompt(strategyID, prompt); err != nil {
		return fmt.Sprintf("❌ 更新 Prompt 失败: %v", err)
	}
	return "✅ 策略 Prompt 已更新"
}

// buildStrategyConfigJSON builds a minimal valid StrategyConfig JSON from params
func buildStrategyConfigJSON(params map[string]string) string {
	coins := params["coins"]
	if coins == "" {
		coins = "BTC"
	}
	stopLoss := params["stop_loss_pct"]
	if stopLoss == "" {
		stopLoss = "5"
	}
	maxPos := params["max_position_pct"]
	if maxPos == "" {
		maxPos = "20"
	}
	indicators := params["indicators"]

	return fmt.Sprintf(`{
		"strategy_type": "ai_trading",
		"coin_source": {"source_type": "static", "static_coins": [%q]},
		"indicators": {"enable_rsi": %v, "enable_macd": %v},
		"risk_control": {"stop_loss_pct": %s, "max_position_pct": %s}
	}`,
		coins,
		strings.Contains(indicators, "RSI"),
		strings.Contains(indicators, "MACD"),
		stopLoss,
		maxPos,
	)
}

func formatParams(params map[string]string) string {
	var sb strings.Builder
	for k, v := range params {
		if k == "api_key" || k == "secret_key" {
			v = "***"
		}
		sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
	}
	return sb.String()
}
```

**监工补充：这里至少有 6 个会直接出错或行为错误的点**

1. 当前写法会把“当前消息”重复注入 LLM 上下文。
   - `sess.Memory.Add("user", userMessage)` 已经把本轮消息写进历史
   - `parser.Parse(userMessage, ctx)` 又会把 `userMessage` 拼到 `conversationContext` 后面
   - 二选一修正：要么先 parse 再写 memory，要么 `Parse()` 不再重复追加当前消息

2. `store.TraderPosition` 没有 `UnrealizedPnl` 字段。
   - 首版查询持仓只能返回仓位基础信息，或另找真实未实现盈亏来源

3. `store.EquitySnapshot` 没有 `TotalBalance` 字段，真实字段是 `TotalEquity`

4. `strategy.ID` 不是 `%d`，`AIModel` 也没有示例中的 `Model` 字段

5. `buildStrategyConfigJSON()` 示例不符合当前仓库真实 `StrategyConfig`
   - `risk_control.stop_loss_pct`
   - `risk_control.max_position_pct`
   这些都不是当前结构里的真实字段名
   - 首版如果做策略写入，必须基于 `store.GetDefaultStrategyConfig("zh")` 组装

6. `updatePrompt()` 不能直接调用“按数值 strategyID 更新顶层 prompt”的假接口
   - 真实实现应该更新 `Strategy.Config` 里的 `CustomPrompt` 或 prompt sections
   - 或者先把首版 prompt 修改目标收缩为 `Trader().UpdateCustomPrompt(...)`

**Step 2: Build 验证**

```bash
cd /Users/yida/gopro/open-nofx && go build ./telegram/...
```

**Step 3: Commit**

```bash
git add telegram/handler/handler.go
git commit -m "feat(telegram): add intent handler with 6 feature areas"
```

---

### Task 7: Bot 入口 telegram/bot.go

**Files:**
- Create: `telegram/bot.go`

**Step 1: 创建文件**

```go
package telegram

import (
	"nofx/config"
	"nofx/logger"
	"nofx/manager"
	"nofx/mcp"
	"nofx/store"
	"nofx/telegram/handler"
	"nofx/telegram/intent"
	"nofx/telegram/service"
	"nofx/telegram/session"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Start initializes and runs the Telegram bot.
// Called from main.go as a goroutine.
func Start(cfg *config.Config, st *store.Store, tm *manager.TraderManager) {
	if cfg.TelegramBotToken == "" {
		logger.Info("📵 Telegram bot not configured (TELEGRAM_BOT_TOKEN not set), skipping")
		return
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		logger.Errorf("❌ Failed to start Telegram bot: %v", err)
		return
	}

	logger.Infof("🤖 Telegram bot started: @%s", bot.Self.UserName)

	// Build the LLM client for intent parsing (use DeepSeek by default)
	llmClient := mcp.New()
	// Configure with whatever key is available in env (intent parsing is lightweight)
	// The service layer will use store to get user-configured models for actual trading

	svc := service.New(st, tm)
	parser := intent.NewParser(llmClient)
	sessions := session.NewManager(llmClient)
	h := handler.New(svc, parser, sessions)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID

		// Access control: only allow configured admin chat ID
		if cfg.TelegramAdminChatID != 0 && chatID != cfg.TelegramAdminChatID {
			msg := tgbotapi.NewMessage(chatID, "⛔ 未授权访问")
			bot.Send(msg)
			continue
		}

		text := update.Message.Text
		if text == "" {
			continue
		}

		// Handle /start command
		if text == "/start" {
			sessions.Get(chatID).ResetFull()
			reply := tgbotapi.NewMessage(chatID, welcomeMessage())
			bot.Send(reply)
			continue
		}

		// Process message
		reply := h.Handle(chatID, text)
		msg := tgbotapi.NewMessage(chatID, reply)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
	}
}

func welcomeMessage() string {
	return `👋 欢迎使用 NOFX 交易助手！

你可以用自然语言配置和管理你的交易系统：

📋 *配置功能*
• 「帮我创建一个 BTC 策略，RSI+MACD，止损 8%」
• 「配置 Binance 交易所」
• 「添加 DeepSeek 大模型」
• 「创建一个交易员」

📊 *查询功能*
• 「查看当前持仓」
• 「查看账户余额」

⚙️ *控制功能*
• 「启动交易员」
• 「停止交易员」
• 「修改策略 Prompt」

输入 /start 重置会话`
}
```

**监工补充：本节伪代码需要先修正两个问题**

1. `mcp.New()` 在当前仓库里不存在，必须改成真实可用的构造器
2. `msg.ParseMode = "Markdown"` 首版不要开，先用纯文本，避免用户内容触发格式错误或意外转义

**Step 2: Build 验证**

```bash
cd /Users/yida/gopro/open-nofx && go build ./telegram/...
```

**Step 3: Commit**

```bash
git add telegram/bot.go
git commit -m "feat(telegram): add Telegram bot entry point with access control"
```

---

### Task 8: 接入 main.go（3 行改动）

**Files:**
- Modify: `main.go`

**Step 1: 加 import**

在 main.go 的 import 块加：

```go
"nofx/telegram"
```

**Step 2: 在 API Server 启动之后加 3 行**

找到这段代码：
```go
// Start API server
server := api.NewServer(...)
go func() { ... }()
```

在其后加：

```go
// Start Telegram bot (if configured)
go telegram.Start(cfg, st, traderManager)
logger.Info("🤖 Telegram bot goroutine started")
```

**Step 3: 完整构建**

```bash
cd /Users/yida/gopro/open-nofx && go build -o nofx .
```

Expected: 成功编译，无错误

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat(telegram): wire Telegram bot into main startup (3 lines)"
```

---

### Task 9: .env.example 文档更新

**Files:**
- Modify: `.env.example` 或 `.env`（若存在）

**Step 1: 在 .env.example 末尾加**

```env
# Telegram Bot Configuration
# Get token from @BotFather on Telegram
TELEGRAM_BOT_TOKEN=
# Get your chat ID from @userinfobot on Telegram
TELEGRAM_ADMIN_CHAT_ID=
```

**Step 2: Commit**

```bash
git add .env.example
git commit -m "docs: add Telegram bot configuration to .env.example"
```

---

### Task 10: 手动集成测试

**Step 1: 配置环境变量**

```bash
export TELEGRAM_BOT_TOKEN=你的bot_token
export TELEGRAM_ADMIN_CHAT_ID=你的chat_id
```

**Step 2: 启动 NOFX**

```bash
cd /Users/yida/gopro/open-nofx && ./nofx
```

Expected 日志：
```
✅ Configuration loaded
🤖 Telegram bot started: @your_bot_name
✅ System started successfully
```

**Step 3: 测试对话流程**

在 Telegram 发送：
1. `/start` → 收到欢迎消息
2. `查看当前持仓` → 返回持仓信息或「无持仓」
3. `帮我创建一个 BTC 策略，RSI+MACD，止损 8%` → Bot 追问策略名
4. `叫"主力BTC"` → 策略创建成功

**Step 4: 验证访问控制**

用其他账号发送消息 → 收到「⛔ 未授权访问」

---

## 关键约束备忘

1. **`service/nofx.go` 是唯一接触 store/manager 的文件**，handler 不能绕过它
2. **compaction 静默发生**，用户看不到压缩过程
3. **LLM 客户端必须使用真实存在的构造器**，不能写 `mcp.New()`
4. **当前仓库的 `store` / `manager` 接口与本文示例存在偏差**，实现时必须以源码为准
5. **首轮目标是“最小可用闭环”而不是功能铺满**，先交付查询与启停，再扩到配置写入

## 监工验收清单

1. `go build ./telegram/...` 成功
2. `go build ./...` 成功
3. 未授权 chat 收到拒绝消息，且不会进入业务逻辑
4. `/start` 后会话状态确实被清空，且重置语义与代码一致
5. 启动/停止交易员的行为与现有 HTTP API 一致
6. 没有任何日志或回复泄露密钥、私钥、passphrase
7. 查询接口用到的字段名全部来自真实 struct，而不是文档猜测

## 后续可扩展

- 主动推送：NOFX 交易决策 → 推送到 Telegram
- 多语言：intent parser 的 systemPrompt 支持英文
- 图表：发送持仓/权益曲线截图（需 TradingView Lightweight Charts 截图服务）
