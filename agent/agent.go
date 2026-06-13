// Package agent implements the NOFXi Agent Core.
//
// Architecture: ALL user messages go to the LLM. The LLM understands intent
// and calls tools to execute actions. No regex routing, no pattern matching.
// The LLM IS the brain — just like how OpenClaw works.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"nofx/manager"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"nofx/wallet"
)

type Agent struct {
	traderManager *manager.TraderManager
	store         *store.Store
	aiClient      mcp.AIClient
	config        *Config
	sentinel      *Sentinel
	brain         *Brain
	scheduler     *Scheduler
	logger        *slog.Logger
	history       *chatHistory
	pending       *pendingTrades
	stopCh        chan struct{} // signals background goroutines to stop
	setupStates   sync.Map
	flowLocks     sync.Map
	NotifyFunc    func(userID int64, text string) error
}

type Config struct {
	Language            string   `json:"language"`
	WatchSymbols        []string `json:"watch_symbols"`
	EnableBriefs        bool     `json:"enable_briefs"`
	EnableNews          bool     `json:"enable_news"`
	EnableSentinel      bool     `json:"enable_sentinel"`
	AllowTradeExecution bool     `json:"allow_trade_execution"`
	BriefTimes          []int    `json:"brief_times"`
}

var (
	agentWalletAddressFromPrivateKey = walletAddressFromPrivateKey
	agentQueryUSDCBalanceCached      = wallet.QueryUSDCBalanceCached
)

func DefaultConfig() *Config {
	return &Config{
		Language:            "zh",
		WatchSymbols:        []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"},
		EnableBriefs:        true,
		EnableNews:          true,
		EnableSentinel:      true,
		AllowTradeExecution: true,
		BriefTimes:          []int{8, 20},
	}
}

func New(tm *manager.TraderManager, st *store.Store, cfg *Config, logger *slog.Logger) *Agent {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Agent{traderManager: tm, store: st, config: cfg, logger: logger, history: newChatHistory(chatHistoryMaxTurns), pending: newPendingTrades(), stopCh: make(chan struct{})}
}

func (a *Agent) SetAIClient(c mcp.AIClient) { a.aiClient = c }

func (a *Agent) ensureHistory() {
	if a.history == nil {
		a.history = newChatHistory(100)
	}
}

func (a *Agent) log() *slog.Logger {
	if a != nil && a.logger != nil {
		return a.logger
	}
	return slog.Default()
}

func (a *Agent) flowLock(userID int64) *sync.Mutex {
	if a == nil {
		return &sync.Mutex{}
	}
	lock, _ := a.flowLocks.LoadOrStore(userID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (a *Agent) EnsureAIClient() {
	a.ensureAIClientForStoreUser("default")
}

func (a *Agent) ensureAIClientForStoreUser(storeUserID string) {
	if storeUserID == "" {
		storeUserID = "default"
	}
	if a.store != nil {
		if client, modelName, ok := a.loadAIClientFromStoreUser(storeUserID); ok {
			a.aiClient = client
			a.log().Info("agent AI client ready", "store_user_id", storeUserID, "model", modelName)
			return
		}
	}
	if a.aiClient != nil {
		a.log().Warn("clearing stale AI client for store user", "store_user_id", storeUserID)
		a.aiClient = nil
	}
	a.log().Warn("no AI client — agent will have limited capabilities", "store_user_id", storeUserID)
}

func (a *Agent) loadAIClientFromStoreUser(storeUserID string) (mcp.AIClient, string, bool) {
	if a.store == nil {
		a.log().Warn("cannot load AI client: store unavailable", "store_user_id", storeUserID)
		return nil, "", false
	}

	if storeUserID == "" {
		storeUserID = "default"
	}
	candidateUserIDs := []string{storeUserID}
	if storeUserID != "default" {
		candidateUserIDs = append(candidateUserIDs, "default")
	}
	for _, candidateUserID := range candidateUserIDs {
		models, err := a.store.AIModel().List(candidateUserID)
		if err != nil {
			a.log().Warn("failed to list AI models for store user", "store_user_id", candidateUserID, "error", err)
			continue
		}
		candidates := rankAgentModelCandidates(models)
		for _, candidate := range candidates {
			model := candidate.model
			if model == nil || !model.Enabled || !agentModelHasUsableAPIKey(model) {
				continue
			}

			a.log().Info(
				"agent evaluating AI model config",
				"store_user_id", candidateUserID,
				"model_id", model.ID,
				"provider", model.Provider,
				"enabled", model.Enabled,
				"has_api_key", len(model.APIKey) > 0,
				"custom_api_url", strings.TrimSpace(model.CustomAPIURL),
				"custom_model_name", strings.TrimSpace(model.CustomModelName),
				"prefer_model_with_balance", candidate.preferModelWithBalance,
				"wallet_balance_usdc", candidate.balanceUSDC,
			)

			apiKey := strings.TrimSpace(string(model.APIKey))
			customAPIURL := strings.TrimSpace(model.CustomAPIURL)
			modelName := strings.TrimSpace(model.CustomModelName)
			provider := strings.ToLower(strings.TrimSpace(model.Provider))

			// Use the provider registry for providers like claw402 that have their own
			// client implementation (x402 payment, custom auth, etc.).
			if client := mcp.NewAIClientByProvider(provider); client != nil {
				if modelName == "" {
					modelName = model.ID
				}
				client.SetAPIKey(apiKey, customAPIURL, modelName)
				a.log().Info("agent AI client selected (provider registry)", "store_user_id", candidateUserID, "model_id", model.ID, "provider", provider, "model", modelName)
				return client, modelName, true
			}

			customAPIURL, modelName = resolveModelRuntimeConfig(provider, customAPIURL, modelName, model.ID)
			if apiKey == "" || customAPIURL == "" {
				a.log().Warn(
					"skipping incomplete enabled AI model",
					"store_user_id", candidateUserID,
					"model_id", model.ID,
					"provider", provider,
					"has_api_key", apiKey != "",
					"has_custom_api_url", customAPIURL != "",
				)
				continue
			}

			httpClient := &http.Client{Timeout: 60 * time.Second}
			client := mcp.NewClient(mcp.WithHTTPClient(httpClient))
			client.SetAPIKey(apiKey, customAPIURL, modelName)
			a.log().Info("agent AI client selected", "store_user_id", candidateUserID, "model_id", model.ID, "model", modelName)
			return client, modelName, true
		}
	}

	a.log().Warn("no enabled AI model found for store user", "store_user_id", storeUserID)
	return nil, "", false
}

type agentModelCandidate struct {
	model                  *store.AIModel
	preferModelWithBalance bool
	balanceUSDC            float64
}

func rankAgentModelCandidates(models []*store.AIModel) []agentModelCandidate {
	candidates := make([]agentModelCandidate, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		candidate := agentModelCandidate{model: model}
		if balance, ok := agentModelUSDCBalance(model); ok && balance > 0 {
			candidate.preferModelWithBalance = true
			candidate.balanceUSDC = balance
		}
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.preferModelWithBalance != right.preferModelWithBalance {
			return left.preferModelWithBalance
		}
		if left.balanceUSDC != right.balanceUSDC {
			return left.balanceUSDC > right.balanceUSDC
		}
		leftUpdatedAt := time.Time{}
		rightUpdatedAt := time.Time{}
		if left.model != nil {
			leftUpdatedAt = left.model.UpdatedAt
		}
		if right.model != nil {
			rightUpdatedAt = right.model.UpdatedAt
		}
		if !leftUpdatedAt.Equal(rightUpdatedAt) {
			return leftUpdatedAt.After(rightUpdatedAt)
		}
		leftID := ""
		rightID := ""
		if left.model != nil {
			leftID = left.model.ID
		}
		if right.model != nil {
			rightID = right.model.ID
		}
		return leftID < rightID
	})

	return candidates
}

func agentModelUSDCBalance(model *store.AIModel) (float64, bool) {
	if model == nil || !agentProviderSupportsUSDCBalance(model.Provider) {
		return 0, false
	}
	privateKey := strings.TrimSpace(string(model.APIKey))
	if privateKey == "" {
		return 0, false
	}
	walletAddress, err := agentWalletAddressFromPrivateKey(privateKey)
	if err != nil || strings.TrimSpace(walletAddress) == "" {
		return 0, false
	}
	balance, err := agentQueryUSDCBalanceCached(walletAddress)
	if err != nil || balance <= 0 {
		return 0, false
	}
	return balance, true
}

func agentProviderSupportsUSDCBalance(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claw402", "blockrun-base":
		return true
	default:
		return false
	}
}

func agentModelHasUsableAPIKey(model *store.AIModel) bool {
	if model == nil {
		return false
	}
	if strings.TrimSpace(string(model.APIKey)) != "" {
		return true
	}
	envKeyByProvider := map[string]string{
		"deepseek": "DEEPSEEK_API_KEY",
		"openai":   "OPENAI_API_KEY",
		"claude":   "ANTHROPIC_API_KEY",
		"gemini":   "GEMINI_API_KEY",
		"grok":     "XAI_API_KEY",
		"kimi":     "MOONSHOT_API_KEY",
		"minimax":  "MINIMAX_API_KEY",
		"qwen":     "DASHSCOPE_API_KEY",
	}
	envKey := envKeyByProvider[strings.ToLower(strings.TrimSpace(model.Provider))]
	return envKey != "" && strings.TrimSpace(os.Getenv(envKey)) != ""
}

func walletAddressFromPrivateKey(privateKey string) (string, error) {
	key := strings.TrimSpace(privateKey)
	if !strings.HasPrefix(key, "0x") {
		return "", fmt.Errorf("private key must start with 0x")
	}
	if len(key) != 66 {
		return "", fmt.Errorf("private key must be 66 characters")
	}

	privateKeyObj, err := gethcrypto.HexToECDSA(strings.TrimPrefix(key, "0x"))
	if err != nil {
		return "", err
	}

	return gethcrypto.PubkeyToAddress(privateKeyObj.PublicKey).Hex(), nil
}

func resolveModelRuntimeConfig(provider, customAPIURL, customModelName, fallbackModelID string) (string, string) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	customAPIURL = strings.TrimSpace(customAPIURL)
	customModelName = strings.TrimSpace(customModelName)
	fallbackModelID = strings.TrimSpace(fallbackModelID)

	type providerDefaults struct {
		url   string
		model string
	}
	defaults := map[string]providerDefaults{
		"deepseek": {url: "https://api.deepseek.com/v1", model: "deepseek-chat"},
		"qwen":     {url: "https://dashscope.aliyuncs.com/compatible-mode/v1", model: "qwen3-max"},
		"openai":   {url: "https://api.openai.com/v1", model: "gpt-5.5"},
		"claude":   {url: "https://api.anthropic.com/v1", model: "claude-opus-4-6"},
		"gemini":   {url: "https://generativelanguage.googleapis.com/v1beta/openai", model: "gemini-3-pro-preview"},
		"grok":     {url: "https://api.x.ai/v1", model: "grok-3-latest"},
		"kimi":     {url: "https://api.moonshot.ai/v1", model: "moonshot-v1-auto"},
		"minimax":  {url: "https://api.minimax.chat/v1", model: "MiniMax-M2.5"},
		"claw402":  {url: "https://claw402.ai", model: "deepseek"},
	}

	if customAPIURL == "" {
		if cfg, ok := defaults[provider]; ok {
			customAPIURL = cfg.url
		}
	}
	if customModelName == "" {
		if cfg, ok := defaults[provider]; ok {
			customModelName = cfg.model
		}
	}
	if customModelName == "" {
		customModelName = fallbackModelID
	}
	return customAPIURL, customModelName
}

func (a *Agent) Start() {
	a.logger.Info("starting NOFXi agent...")
	a.EnsureAIClient()

	if a.config.EnableSentinel {
		a.sentinel = NewSentinel(a.config.WatchSymbols, a.handleSignal, a.logger)
		a.sentinel.Start()
	}
	a.brain = NewBrain(a, a.logger)
	if a.config.EnableNews {
		a.brain.StartNewsScan(5 * time.Minute)
	}
	if a.config.EnableBriefs {
		a.brain.StartMarketBriefs(a.config.BriefTimes)
	}
	a.scheduler = NewScheduler(a, a.logger)
	a.scheduler.Start(context.Background())

	a.logger.Info("NOFXi agent is online 🚀")
}

func (a *Agent) Stop() {
	// Signal all background goroutines (e.g. chat-history-cleanup) to exit.
	select {
	case <-a.stopCh:
		// Already closed
	default:
		close(a.stopCh)
	}
	if a.sentinel != nil {
		a.sentinel.Stop()
	}
	if a.brain != nil {
		a.brain.Stop()
	}
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
}

// HandleMessage — the core. Everything goes through the LLM.
func (a *Agent) HandleMessage(ctx context.Context, userID int64, text string) (string, error) {
	a.EnsureAIClient()
	return a.handleMessageForStoreUser(ctx, "default", userID, text)
}

// HandleMessageForStoreUser is like HandleMessage but stores setup artifacts
// (exchange/model) under the provided authenticated store user ID.
func (a *Agent) HandleMessageForStoreUser(ctx context.Context, storeUserID string, userID int64, text string) (string, error) {
	return a.handleMessageForStoreUser(ctx, storeUserID, userID, text)
}

func (a *Agent) handleMessageForStoreUser(ctx context.Context, storeUserID string, userID int64, text string) (string, error) {
	a.ensureAIClientForStoreUser(storeUserID)

	lang := a.config.Language
	if strings.HasPrefix(text, "[lang:") {
		if end := strings.Index(text, "] "); end > 0 {
			lang = text[6:end]
			text = text[end+2:]
		}
	}

	a.logger.Info("message", "user_id", userID, "text", text)

	// Only keep a tiny command surface outside the planner.
	if text == "/status" {
		return a.handleStatus(lang), nil
	}
	if text == "/clear" {
		a.clearConversationState(userID)
		if lang == "zh" {
			return "🧹 对话记忆已清除。", nil
		}
		return "🧹 Conversation history cleared.", nil
	}
	if reply, handled := a.handleTradeConfirmation(ctx, userID, text, lang); handled {
		return reply, nil
	}
	if reply, handled := a.handleModelWalletBalanceQuestion(storeUserID, lang, text); handled {
		return reply, nil
	}

	// Everything else goes through the planner and tool system.
	return a.thinkAndAct(ctx, storeUserID, userID, lang, text)
}

// HandleMessageStream is like HandleMessage but streams the final LLM response via SSE.
// onEvent is called with (eventType, data) — see StreamEvent* constants.
// Non-streamable responses (commands, trade confirmations) return immediately without events.
func (a *Agent) HandleMessageStream(ctx context.Context, userID int64, text string, onEvent func(event, data string)) (string, error) {
	a.EnsureAIClient()
	return a.handleMessageStreamForStoreUser(ctx, "default", userID, text, onEvent)
}

// HandleMessageStreamForStoreUser mirrors HandleMessageForStoreUser for SSE responses.
func (a *Agent) HandleMessageStreamForStoreUser(ctx context.Context, storeUserID string, userID int64, text string, onEvent func(event, data string)) (string, error) {
	return a.handleMessageStreamForStoreUser(ctx, storeUserID, userID, text, onEvent)
}

func (a *Agent) handleMessageStreamForStoreUser(ctx context.Context, storeUserID string, userID int64, text string, onEvent func(event, data string)) (string, error) {
	a.ensureAIClientForStoreUser(storeUserID)

	lang := a.config.Language
	if strings.HasPrefix(text, "[lang:") {
		if end := strings.Index(text, "] "); end > 0 {
			lang = text[6:end]
			text = text[end+2:]
		}
	}

	a.logger.Info("message (stream)", "user_id", userID, "text", text)

	if text == "/status" {
		return a.handleStatus(lang), nil
	}
	if text == "/clear" {
		a.clearConversationState(userID)
		if lang == "zh" {
			return "🧹 对话记忆已清除。", nil
		}
		return "🧹 Conversation history cleared.", nil
	}
	if reply, handled := a.handleTradeConfirmation(ctx, userID, text, lang); handled {
		if onEvent != nil {
			emitStreamText(onEvent, reply)
		}
		return reply, nil
	}
	if reply, handled := a.handleModelWalletBalanceQuestion(storeUserID, lang, text); handled {
		if onEvent != nil {
			emitStreamText(onEvent, reply)
		}
		return reply, nil
	}
	return a.thinkAndActStream(ctx, storeUserID, userID, lang, text, onEvent)
}

func (a *Agent) clearConversationState(userID int64) {
	if a == nil {
		return
	}
	if a.history != nil {
		a.history.Clear(userID)
	}
	a.clearTaskState(userID)
	a.clearSkillSession(userID)
	a.clearActiveSkillSession(userID)
	a.clearPendingProposalSession(userID)
	a.clearWorkflowSession(userID)
	a.clearExecutionState(userID)
	a.clearReferenceMemory(userID)
	a.SnapshotManager(userID).Clear()
	a.clearSetupState(userID)
}

// StreamEvent types sent via SSE to the frontend.
const (
	StreamEventPlanning     = "planning"
	StreamEventPlan         = "plan"
	StreamEventStepStart    = "step_start"
	StreamEventStepComplete = "step_complete"
	StreamEventReplan       = "replan"
	StreamEventTool         = "tool"  // Tool is being called (shows status to user)
	StreamEventDelta        = "delta" // Text chunk from LLM streaming
	StreamEventDone         = "done"  // Stream complete
	StreamEventError        = "error" // Error occurred
)

// buildSystemPrompt creates the system prompt that makes NOFXi behave like a real agent.
func (a *Agent) buildSystemPrompt(lang string) string {
	return a.buildSystemPromptForStoreUser(lang, "default")
}

func (a *Agent) buildSystemPromptForStoreUser(lang, storeUserID string) string {
	// Gather live system state
	traderInfo := a.getTradersSummaryForStoreUser(storeUserID)
	watchlist := ""
	if a.sentinel != nil {
		watchlist = a.sentinel.FormatWatchlist(lang)
	}
	skillCatalog := skillCatalogPrompt(lang)

	if lang == "zh" {
		return fmt.Sprintf(`你是 NOFXi，一个专业的 AI 交易 Agent。你不是一个简单的聊天机器人——你是用户的交易伙伴。

## 你的核心能力
1. **市场分析** — 加密货币（BTC/ETH/SOL等）有实时数据，A股/港股/美股/外汇你可以基于知识分析
2. **交易管理** — 查看持仓、余额、交易历史、Trader 状态
3. **策略建议** — 根据用户需求制定交易策略
4. **策略模板管理** — 创建、查看、修改、删除、激活策略模板
5. **风险管理** — 评估风险、建议止损止盈
6. **配置引导** — 用户说"开始配置"时引导配置交易所和AI模型

## 当前系统状态
%s
%s

## 数据说明（极其重要，违反即失职！）
- 加密货币（BTC/ETH等）：交易所实时数据，标注 [Real-time]
- A股/港股/美股：**必须调用 search_stock 工具**获取实时行情。不调工具就没有数据。
- 美股盘前盘后：search_stock 返回的 quote 中 ext_price/ext_change_pct/ext_time
- 外汇/指数期货：当前没有数据源，如实告知

### 铁律：禁止编造任何价格！
- **你的训练数据中的价格全部过时，不可使用**
- **没有通过工具获取的价格 = 你不知道 = 不能说**
- 用户问多只股票的盘前数据？→ 对每只股票调用 search_stock 工具
- 用户问"盘前概览"？→ 调用 search_stock 查主要股票（AAPL、TSLA、NVDA、MSFT、GOOGL、AMZN、META等），用真实数据回答
- **绝对不允许**不调工具就给出具体价格数字（如 $421.85）
- 如果某只股票 search_stock 查不到数据，就说"暂时无法获取该股票数据"
- 指数期货（纳指、标普、道琼斯期货）我们目前没有数据源，直接说"暂不支持指数期货数据"

## 工具使用
你可以调用以下工具来执行操作：
- **search_stock** — 搜索股票（支持中文名、英文名、代码）。当用户提到你不认识的股票时，先用这个工具搜索。
- **execute_trade** — 下单交易（加密货币或美股）。常见写法："做多 BTC 0.01 x10"、"做空 ETH 0.1"、"平多 BTC"、"平空 ETH"；英文也支持 "long BTC 0.01 x10"、"short ETH 0.1"、"close long BTC"、"close short ETH"。美股：open_long=买入，close_long=卖出。调用后先创建待确认订单，不会立刻成交。若触发大额风控，用户必须回复"确认大额 trade_xxx"；待确认订单 5 分钟后自动失效。
- **get_positions** — 查看当前所有持仓（加密货币 + 股票）
- **get_balance** — 查看账户余额
- **get_market_price** — 获取实时价格（加密货币或股票代码）
- **get_kline** — 获取最近 K 线 / 蜡烛图数据（适合“看 15 分钟 K 线”“最近 50 根 1 小时 K 线”）
- **get_exchange_configs / manage_exchange_config** — 查看、新增、修改、删除交易所绑定配置
- **get_model_configs / manage_model_config** — 查看、新增、修改、删除 AI 模型配置
- **get_strategies / manage_strategy** — 查看、新增、修改、删除、激活、复制策略模板
- **manage_trader** — 查看、新增、修改、删除、启动、停止交易员
- **get_watchlist / manage_watchlist** — 查看、添加、移除运行时监控币对，适合“把 BTC 加入监控”“别再监控 SOL”这类请求
- **get_ai500_list** — 获取 AI500 指数榜单（AI 评分 0-100 + 入选以来涨幅，按评分排序）。**用户要选币、要推荐标的、或创建策略/交易员没指定币种时，默认先调这个工具，从评分高、表现好的标的里选**；用户问"AI500 里有什么"时也用它

### 配置、策略与交易员管理规则
- 当用户要求创建、修改、删除、激活、复制策略模板时，优先使用 get_strategies / manage_strategy
- **策略模板本身是独立资源，不默认依赖交易所或 AI 模型**
- **策略模板创建成功后应立即出现在策略列表/策略页**
- **策略模板不能直接启动或运行；只有交易员有运行态。**
- 如果用户说“启动策略 / 运行策略”，要明确说明：应先把策略绑定到交易员，再启动交易员
- 用户没问运行/部署/创建交易员时，不要主动延伸到交易员、模型或交易所绑定
- 当用户要求配置交易所、绑定 API Key、修改交易所账户时，优先使用 manage_exchange_config
- 当用户要求配置大模型、设置 API Key、切换模型、修改模型地址时，优先使用 manage_model_config
- 当用户要求创建、修改、删除、启动、停止交易员时，优先使用 manage_trader
- **缺字段时优先用合理默认值，不要逐项追问** — 创建策略/交易员时，凡是有行业常识默认值的字段（杠杆、周期、风控比例、名称等）直接预填，调用工具，然后把"我帮你默认了 X、Y、Z"放进回复让用户确认或修改。只有无法合理默认的字段（如 API Key、交易所选择）才需要追问，且一次问完所有缺失项，不要一轮问一个。
- **在这些工具存在时，不要说“系统没有这个能力”**
- 对敏感信息（API Key、Secret、Private Key）只保存，不要在最终回复中完整回显

%s

### 交易安全规则
- 用户明确要求交易时才调用 execute_trade
- 下单前先尊重风控：数量过大、仓位太小、杠杆过高、超过权益上限时，不要假装能下单，要直接用人话解释原因
- 分析和建议不需要调用工具，直接回复即可
- 交易确认信息要清晰展示：品种、方向、数量、杠杆
- 提醒用户确认命令格式；普通订单用“确认 trade_xxx”，大额订单用“确认大额 trade_xxx”

### 数据真实性规则（极其重要！）
- **持仓信息必须且只能通过 get_positions 工具获取**，绝对禁止编造持仓
- **余额信息必须且只能通过 get_balance 工具获取**，绝对禁止编造余额
- 如果用户问持仓但 get_positions 返回空，就说"当前没有持仓"，不要编造
- 如果工具返回 error（如未配置交易所），如实告知用户
- **你不知道用户持有什么股票/币种，除非工具返回了数据**
- 查股票行情 ≠ 用户持有该股票。不要混淆"查价格"和"有持仓"

## 行为准则（最高优先级）
- **先做完, 再汇报** — 一个回合内可以连续调用多个工具直到任务完成（查→建→配→启动）。所有工具调用都发生在当前回合, 用户看到的下一条消息就是最终结果。绝对不要说"稍等"、"我去处理"、"正在为你办理"——你没有后台任务, 说了就是撒谎。
- **先直接答, 再可选追加一条相关提醒** — 第一句永远是用户问的那个具体答案。然后只在以下三种情况追加一句话: (a) 用户当前仓位有暴露的风险, (b) 完成请求所需的配置缺失, (c) 下一步动作显而易见（比如"已创建 trader, 要我现在启动吗?"）。一次只追加一句, 不要列清单。
- **System Context 是参考资料, 不是输出模板** — 只用跟用户问题直接相关的那部分, 不要复述整个状态。
- **回复要短** — 能一句话说清就不要写一段。**聊天界面不渲染 markdown 表格, 绝对不要输出表格**; 排名/对比类数据用编号列表, 一行一条（如 "1. BEAT — 评分 84.2, +404.1%%"）。也不要用分隔线和标题。
- **会"做事"的 agent, 不是只会"答题"的查询机** — 用户说"创建并启动 X trader", 你应该一次链式调用 create + start, 不要先回"已创建, 请去面板手动启动"。用户已经表达意图, 你就去做。
- **遇到工具错误**: 用一句人话说出原因, 然后给一个最可能的修复建议或一个聚焦的追问。不要默默重试。不要说"稍等一下我去办" — 你没有后台任务。
- **不要重复自我介绍** — 除非用户首次问"你是谁/你能做什么"。
- 把用户当交易小白, 语言简单直接。先结论, 再原因。
- **诚实是第一原则** — 不确定就说不确定, 没数据就说没数据。绝不编造。
- 用中文回复。

当前时间: %s`, traderInfo, watchlist, skillCatalog, time.Now().Format("2006-01-02 15:04:05"))
	}

	return fmt.Sprintf(`You are NOFXi, a professional AI trading agent. Not a chatbot — a trading partner.

## Capabilities
1. Market analysis — crypto with real-time data, stocks/forex with knowledge
2. Trade management — positions, balance, history, trader status
3. Strategy — build trading strategies based on user needs
4. Strategy template management — create, inspect, update, delete, and activate strategy templates
5. Risk management — assess risk, suggest stop-loss/take-profit
6. Setup — guide exchange/AI configuration when user asks

## Current System State
%s
%s

## Data Notice (CRITICAL — violating this is unacceptable!)
- Crypto (BTC/ETH): Exchange real-time data, marked [Real-time]
- Stocks: You MUST call search_stock tool to get real-time quotes. No tool call = no data.
- US stocks pre/after-hours: ext_price/ext_change_pct/ext_time in search_stock results
- Forex/Index futures: No data source currently — tell user honestly

### ABSOLUTE RULE: NEVER fabricate any price!
- Your training data prices are ALL outdated and MUST NOT be used
- No tool result = you don't know = you cannot state a price
- User asks multiple stocks? → Call search_stock for EACH one
- User asks "pre-market overview"? → Call search_stock for major stocks (AAPL, TSLA, NVDA, MSFT, GOOGL, AMZN, META etc.) and use real data
- NEVER output a specific price number (like $421.85) without a tool having returned it
- If search_stock fails for a stock, say "unable to fetch data for this stock"
- Index futures (NDX, SPX, DJI futures) — we have no data source, say "index futures not supported yet"

## Tools
You can call these tools to take action:
- **search_stock** — Search for stocks by name, ticker, or code. Covers A-share, HK, and US markets. Use when the user mentions an unknown stock.
- **execute_trade** — Place a trade order (crypto or US stocks). Common phrasings include "long BTC 0.01 x10", "short ETH 0.1", "close long BTC", and "close short ETH". For stocks: open_long=buy, close_long=sell. This creates a pending trade first; it does not execute immediately. Large orders require "confirm large trade_xxx", and pending trades expire after 5 minutes.
- **get_positions** — View all current open positions (crypto + stocks)
- **get_balance** — View account balance and equity
- **get_market_price** — Get real-time price from the exchange (crypto or stock symbol)
- **get_kline** — Get recent candlestick / kline data for a crypto symbol
- **get_ai500_list** — AI500 index board (AI score 0-100 + gain since entry, sorted by score). **When the user wants coin picks, recommendations, or creates a strategy/trader without naming coins, call this first and choose from the high-scoring, well-performing entries**; also use it when asked what's in AI500
- **get_exchange_configs / manage_exchange_config** — View, create, update, and delete exchange bindings
- **get_model_configs / manage_model_config** — View, create, update, and delete AI model bindings
- **get_strategies / manage_strategy** — View, create, update, delete, activate, and duplicate strategy templates
- **manage_trader** — List, create, update, delete, start, and stop traders

### Configuration, Strategy, and Trader Rules
- When the user wants to create, edit, delete, activate, or duplicate a strategy template, prefer get_strategies / manage_strategy
- **A strategy template is an independent asset and does not require exchange or model bindings by default**
- **After creation, a strategy template should immediately appear in the strategy list/page**
- **A strategy template cannot be started or run directly; only traders have runtime state**
- If the user says "start the strategy" or "run this strategy", explain that the strategy must be attached to a trader first, then the trader can be started
- Do not proactively bring up traders, models, or exchange bindings unless the user asks to run, deploy, or create a trader
- When the user wants to bind or edit an exchange account, prefer manage_exchange_config
- When the user wants to bind or edit an AI model, prefer manage_model_config
- When the user wants to create, edit, delete, start, or stop a trader, prefer manage_trader
- When the user wants to add, remove, or inspect monitored coins, prefer get_watchlist / manage_watchlist
- **Prefer sensible defaults over field-by-field interrogation** — when creating strategies/traders, prefill any field with an industry-standard default (leverage, timeframes, risk ratios, names), call the tool, then list "I defaulted X, Y, Z" in the reply for the user to confirm or adjust. Only ask for fields that cannot be reasonably defaulted (API keys, exchange choice), and ask for ALL missing ones in a single question — never one per turn.
- **Do not claim the system lacks these capabilities when the tools exist**
- For secrets such as API keys, secrets, and private keys: store them, but never echo them back in full

%s

### Trade Safety Rules
- Only call execute_trade when user explicitly requests a trade
- Respect risk guardrails before placing a trade: if the quantity is too large, the notional is too small, leverage is too high, or the order exceeds equity limits, explain the reason plainly instead of pretending it can be placed
- Analysis and advice don't need tools — just reply directly
- Show trade details clearly: symbol, direction, quantity, leverage
- Remind user of the confirmation command format; normal orders use "confirm trade_xxx", large orders use "confirm large trade_xxx"

### Data Truthfulness Rules (CRITICAL!)
- **Position data MUST come from get_positions tool only** — NEVER fabricate positions
- **Balance data MUST come from get_balance tool only** — NEVER fabricate balances
- If get_positions returns empty, say "no open positions" — do NOT make up holdings
- If a tool returns an error (e.g. no exchange configured), tell the user honestly
- **You do NOT know what the user holds unless a tool tells you**
- Checking a stock price ≠ user owns that stock. Never confuse "quote lookup" with "holding"

## Behavior (HIGHEST PRIORITY)
- **Finish the work, then report** — You may chain as many tool calls as needed within this turn (look up → create → configure → start). Everything happens now; the next message the user sees is the final result. NEVER say "please wait", "I'm working on it", or "I'll handle this" — you have no background tasks, so saying it is lying.
- **Answer directly first, then optionally one relevant follow-up** — The first sentence is always the specific answer to what the user asked. After that, you may add at most one follow-up only when: (a) the user has open risk exposure, (b) a config required to fulfill the request is missing, or (c) the next step is obvious (e.g. "Trader created — want me to start it?"). One follow-up max, no checklists.
- **System Context is reference material, not output template** — Use only the part directly relevant to the user's question. Don't recap the whole state.
- **Keep it short** — One sentence beats a paragraph. **The chat UI does not render markdown tables — never output a table**; for rankings/comparisons use a numbered list, one item per line (e.g. "1. BEAT — score 84.2, +404.1%%"). No dividers or headers either.
- **You're an agent that DOES things, not a Q&A bot** — If the user says "create and start trader X", chain create + start in one go; don't reply "created, please start manually". They already expressed intent; execute it.
- **On tool errors**: name the error in plain language in one sentence, then propose the single most likely fix OR ask one focused clarifying question. Never silently retry. Never say "I'll get back to you" / "please wait" — you have no background job.
- **Don't repeat self-introduction** — unless user first asks "who are you / what can you do".
- Treat the user like a trading beginner. Use plain language. Conclusion first, reason after.
- **Honesty is rule #1** — uncertain = say uncertain, no data = say no data. Never fabricate.

Current time: %s`, traderInfo, watchlist, skillCatalog, time.Now().Format("2006-01-02 15:04:05"))
}

// gatherContext collects real-time market data relevant to the user's message.
func (a *Agent) gatherContext(storeUserID, text string) string {
	var parts []string
	upper := strings.ToUpper(text)

	// Crypto — detect symbols dynamically
	// 1. Check known popular symbols (fast path)
	// 2. Extract any "XXXUSDT" pattern from text (catches arbitrary pairs)
	knownSymbols := []string{
		"BTC", "ETH", "SOL", "BNB", "XRP", "DOGE", "ADA", "AVAX", "DOT", "LINK",
		"PEPE", "SHIB", "ARB", "OP", "SUI", "APT", "SEI", "TIA", "JUP", "WIF",
		"NEAR", "ATOM", "FTM", "MATIC", "INJ", "RENDER", "FET", "TAO", "WLD",
		"AAVE", "UNI", "LDO", "MKR", "CRV", "PENDLE", "ENA", "ONDO", "TRUMP",
	}
	matched := make(map[string]bool)
	for _, sym := range knownSymbols {
		if strings.Contains(upper, sym) {
			matched[sym] = true
		}
	}
	// Also extract "XXXUSDT" patterns for coins not in the known list
	for _, word := range strings.Fields(upper) {
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		if strings.HasSuffix(word, "USDT") && len(word) > 4 && len(word) <= 15 {
			sym := strings.TrimSuffix(word, "USDT")
			if len(sym) >= 2 && len(sym) <= 10 {
				matched[sym] = true
			}
		}
	}
	// Collect and sort matched symbols for deterministic selection
	sortedSymbols := make([]string, 0, len(matched))
	for sym := range matched {
		sortedSymbols = append(sortedSymbols, sym)
	}
	sort.Strings(sortedSymbols)

	// Cap at 5 symbols to avoid slow context gathering
	count := 0
	for _, sym := range sortedSymbols {
		if count >= 5 {
			break
		}
		md, err := market.Get(sym + "USDT")
		if err == nil && md.CurrentPrice > 0 {
			parts = append(parts, fmt.Sprintf("[%s/USDT Real-time]\nPrice: $%.4f | 1h: %+.2f%% | 4h: %+.2f%% | RSI7: %.1f | EMA20: %.4f | MACD: %.6f | Funding: %.4f%%",
				sym, md.CurrentPrice, md.PriceChange1h, md.PriceChange4h, md.CurrentRSI7, md.CurrentEMA20, md.CurrentMACD, md.FundingRate*100))
			count++
		}
	}

	// A-share / stocks — only call Sina API when text likely references stocks.
	// Skip for purely crypto conversations to avoid unnecessary external API calls.
	if looksLikeStockQuery(text) {
		stockCode, stockName := resolveStockCodeDynamic(text)
		if stockCode != "" {
			quote, err := fetchStockQuote(stockCode)
			if err == nil && quote.Price > 0 {
				parts = append(parts, fmt.Sprintf("[%s(%s) Real-time A-share Data]\n%s", quote.Name, quote.Code, formatStockQuote(quote)))
			} else if err != nil {
				a.logger.Error("fetch stock quote", "code", stockCode, "name", stockName, "error", err)
			}
		}
	}

	// Trader positions
	if a.traderManager != nil && a.store != nil {
		traderConfigs, _ := a.store.Trader().List(storeUserID)
		for _, traderCfg := range traderConfigs {
			if strings.TrimSpace(traderCfg.ID) == "" {
				continue
			}
			t, err := a.traderManager.GetTrader(traderCfg.ID)
			if err != nil {
				continue
			}
			positions, err := t.GetPositions()
			if err != nil {
				continue
			}
			for _, p := range positions {
				size := toFloat(p["size"])
				if size == 0 {
					continue
				}
				parts = append(parts, fmt.Sprintf("[Position] %s %s: size=%.4f entry=$%.4f mark=$%.4f pnl=$%.2f",
					p["symbol"], p["side"], size, toFloat(p["entryPrice"]), toFloat(p["markPrice"]), toFloat(p["unrealizedPnl"])))
			}
		}
	}

	return strings.Join(parts, "\n")
}

func (a *Agent) getTradersSummary() string {
	return a.getTradersSummaryForStoreUser("default")
}

func (a *Agent) getTradersSummaryForStoreUser(storeUserID string) string {
	if a.traderManager == nil {
		return "Traders: none configured"
	}
	if a.store == nil {
		return "Traders: none configured"
	}
	if strings.TrimSpace(storeUserID) == "" {
		storeUserID = "default"
	}
	traderConfigs, err := a.store.Trader().List(storeUserID)
	if err != nil || len(traderConfigs) == 0 {
		return "Traders: none configured"
	}

	var lines []string
	for _, traderCfg := range traderConfigs {
		if strings.TrimSpace(traderCfg.ID) == "" {
			continue
		}
		t, err := a.traderManager.GetTrader(traderCfg.ID)
		isRunning := traderCfg.IsRunning
		exchange := traderCfg.ExchangeID
		if err == nil && t != nil {
			s := t.GetStatus()
			if running, ok := s["is_running"].(bool); ok {
				isRunning = running
			}
			exchange = t.GetExchange()
		}
		status := "stopped"
		if isRunning {
			status = "running"
		}
		tid := traderCfg.ID
		if len(tid) > 8 {
			tid = tid[:8]
		}
		lines = append(lines, fmt.Sprintf("• %s [%s] %s | %s", traderCfg.Name, tid, status, exchange))
	}
	if len(lines) == 0 {
		return "Traders: none configured"
	}
	return "Traders:\n" + strings.Join(lines, "\n")
}

func (a *Agent) handleStatus(L string) string {
	tc, rc := 0, 0
	if a.traderManager != nil {
		all := a.traderManager.GetAllTraders()
		tc = len(all)
		for _, t := range all {
			if s := t.GetStatus(); s["is_running"] == true {
				rc++
			}
		}
	}
	wc := 0
	if a.sentinel != nil {
		wc = a.sentinel.SymbolCount()
	}
	ai := "❌"
	if a.aiClient != nil {
		ai = "✅"
	}
	return fmt.Sprintf(a.msg(L, "status"), rc, tc, wc, ai, time.Now().Format("2006-01-02 15:04:05"))
}

// noAIFallback — when no AI is available, still try to be useful.
func (a *Agent) noAIFallback(storeUserID, lang, text string) (string, error) {
	upper := strings.ToUpper(text)

	// Try to provide market data directly
	for _, sym := range []string{"BTC", "ETH", "SOL", "BNB", "XRP", "DOGE"} {
		if strings.Contains(upper, sym) {
			md, err := market.Get(sym + "USDT")
			if err == nil {
				return fmt.Sprintf("📊 *%s/USDT*\n\n%s\n\n💡 配置 AI 模型后我能给你更深度的分析。发送 *开始配置* 开始。", sym, market.Format(md)), nil
			}
		}
	}

	// Check if asking about positions/balance
	if strings.Contains(text, "持仓") || strings.Contains(upper, "POSITION") {
		return a.queryPositionsDirect(storeUserID, lang)
	}
	if strings.Contains(text, "余额") || strings.Contains(upper, "BALANCE") {
		return a.queryBalancesDirect(storeUserID, lang)
	}

	if lang == "zh" {
		return "🤖 我是 NOFXi。配置 AI 模型后我就能理解你的任何问题——分析股票、制定策略、管理交易。\n\n现在可用：\n• 加密货币实时行情（试试「BTC」）\n• `/status` 查看系统状态\n• `/clear` 清空当前对话记忆\n\n发送 *开始配置* 配置 AI 模型。", nil
	}
	return "🤖 I'm NOFXi. Configure an AI model and I can understand anything — analyze stocks, build strategies, manage trades.\n\nAvailable now:\n• Crypto real-time data (try 'BTC')\n• `/status` to check system status\n• `/clear` to clear the current conversation memory\n\nSend *setup* to configure AI.", nil
}

func (a *Agent) aiServiceFailure(lang string, err error) (string, error) {
	reason := "unknown error"
	if err != nil {
		reason = summarizeObservation(err.Error())
	}
	a.logger.Error("AI service call failed", "error", reason)
	if lang == "zh" {
		return fmt.Sprintf("当前 AI 服务调用失败：%s\n\n%s", reason, aiServiceFailureGuidance("zh", reason)), nil
	}
	return fmt.Sprintf("The AI service call failed: %s\n\n%s", reason, aiServiceFailureGuidance(lang, reason)), nil
}

func aiServiceFailureGuidance(lang, reason string) string {
	lower := strings.ToLower(strings.TrimSpace(reason))
	looksLikeHTMLGateway := strings.Contains(lower, "invalid character '<'") ||
		strings.Contains(lower, "unexpected character '<'") ||
		strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<!doctype html")
	looksLikeUpstreamEmptyOutput := strings.Contains(lower, "upstream_empty_output") ||
		(strings.Contains(lower, "empty output") && strings.Contains(lower, "rate_limit_error"))
	looksLikeRateLimit := strings.Contains(lower, "status 429") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "rate_limit_error")
	looksLikeBannedAccount := strings.Contains(lower, "user_is_banned") ||
		strings.Contains(lower, "account is banned") ||
		strings.Contains(lower, "account banned")
	looksLikeAuthFailure := strings.Contains(lower, "status 401") ||
		strings.Contains(lower, "authentication_failed") ||
		strings.Contains(lower, "authentication_error") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "invalid api key")

	if lang == "zh" {
		if looksLikeHTMLGateway {
			return "这不是“未配置模型”。这次更像是上游返回了 HTML 页面或网关/反代错误页，而不是标准 JSON 响应。更可能原因是模型服务地址配错、网关拦截、支付/鉴权页返回、或上游服务临时异常。请优先检查当前启用模型的 custom_api_url、反向代理/网关状态，以及对应 provider 的服务状态。"
		}
		if looksLikeBannedAccount {
			return "这不是“未配置模型”。当前启用模型已经连到了上游，但上游明确拒绝登录，原因是账号被禁用/封禁（USER_IS_BANNED）。请检查当前启用模型配置对应的账号或 API Key，换一个可用账号/API Key，或切换到另一个已启用模型后再试。"
		}
		if looksLikeAuthFailure {
			return "这不是“未配置模型”。当前启用模型已经连到了上游，但鉴权失败了。请检查当前启用模型的 API Key、钱包凭证、provider 账号状态和 custom_api_url 是否匹配；修复凭证或切换到另一个可用模型后再试。"
		}
		if looksLikeUpstreamEmptyOutput {
			return "这不是“未配置模型”。这次更像是上游模型没有返回有效内容，当前 provider 把它包装成了 429 / rate_limit_error。更可能原因是上游临时限流、服务拥塞、模型空响应，或 provider 网关没有拿到有效结果；不应优先归因成“余额不足”。请先重试一次；如果持续出现，再检查当前启用模型的 provider 状态、限流配额、网关日志，或先切换到另一个可用模型。"
		}
		if looksLikeRateLimit {
			return "这不是“未配置模型”。这次更像是当前模型 provider 触发了限流或网关节流。更可能原因是并发过高、调用频率超限、provider 临时拥塞，或上游配额限制。请先稍后重试；如果持续出现，再检查当前启用模型的 provider 配额、限流策略和网关状态。"
		}
		return "这不是“未配置模型”。更可能是模型服务余额不足、接口报错、鉴权失败或超时。请检查当前启用模型的 API 状态后再试。"
	}
	if looksLikeHTMLGateway {
		return "This is not a missing-model issue. It looks more like the upstream returned an HTML page or gateway/proxy error page instead of the expected JSON response. The likely causes are a wrong model endpoint URL, gateway interception, a payment/auth page being returned, or a temporary upstream outage. Check the active model's custom_api_url, proxy/gateway status, and the provider service health first."
	}
	if looksLikeBannedAccount {
		return "This is not a missing-model issue. The active model reached the upstream provider, but login was rejected because the account is banned (USER_IS_BANNED). Check the active model account/API key, replace it with a usable credential, or switch to another enabled model."
	}
	if looksLikeAuthFailure {
		return "This is not a missing-model issue. The active model reached the upstream provider, but authentication failed. Check the active model API key, wallet credential, provider account status, and custom_api_url, or switch to another enabled model."
	}
	if looksLikeUpstreamEmptyOutput {
		return "This is not a missing-model issue. The upstream model appears to have returned no usable output, and the provider wrapped it as a 429 / rate_limit_error. The more likely causes are temporary throttling, upstream congestion, an empty model response, or a gateway that did not receive a valid result. Do not treat this as an insufficient-balance issue first. Retry once, then check the active provider status, rate limits, gateway logs, or switch to another model."
	}
	if looksLikeRateLimit {
		return "This is not a missing-model issue. The active model provider more likely hit rate limiting or gateway throttling. Check the provider quota, rate-limit policy, and gateway status, then retry."
	}
	return "This is not a missing-model issue. The active model provider more likely returned an API error, authentication failure, timeout, or insufficient-balance response. Please check the active model API and try again."
}

func (a *Agent) queryPositionsDirect(storeUserID, L string) (string, error) {
	if a.traderManager == nil {
		return a.msg(L, "no_traders"), nil
	}
	if a.store == nil {
		return a.msg(L, "no_traders"), nil
	}
	traderConfigs, err := a.store.Trader().List(storeUserID)
	if err != nil {
		return a.msg(L, "no_traders"), nil
	}
	var sb strings.Builder
	sb.WriteString("📊 *Positions*\n\n")
	hasAny := false
	for _, traderCfg := range traderConfigs {
		if strings.TrimSpace(traderCfg.ID) == "" {
			continue
		}
		t, err := a.traderManager.GetTrader(traderCfg.ID)
		if err != nil {
			continue
		}
		positions, err := t.GetPositions()
		if err != nil {
			continue
		}
		for _, p := range positions {
			size := toFloat(p["size"])
			if size == 0 {
				continue
			}
			hasAny = true
			pnl := toFloat(p["unrealizedPnl"])
			e := "🟢"
			if pnl < 0 {
				e = "🔴"
			}
			tid := traderCfg.ID
			if len(tid) > 8 {
				tid = tid[:8]
			}
			sb.WriteString(fmt.Sprintf("%s *%s* %s — $%.2f | Trader: %s\n", e, p["symbol"], p["side"], pnl, tid))
		}
	}
	if !hasAny {
		return a.msg(L, "no_positions"), nil
	}
	return sb.String(), nil
}

func (a *Agent) queryBalancesDirect(storeUserID, L string) (string, error) {
	if a.traderManager == nil {
		return a.msg(L, "no_traders"), nil
	}
	if a.store == nil {
		return a.msg(L, "no_traders"), nil
	}
	traderConfigs, err := a.store.Trader().List(storeUserID)
	if err != nil {
		return a.msg(L, "no_traders"), nil
	}
	var sb strings.Builder
	sb.WriteString("💰 *Balance*\n\n")
	for _, traderCfg := range traderConfigs {
		if strings.TrimSpace(traderCfg.ID) == "" {
			continue
		}
		t, err := a.traderManager.GetTrader(traderCfg.ID)
		if err != nil {
			continue
		}
		info, err := t.GetAccountInfo()
		if err != nil {
			continue
		}
		tid := traderCfg.ID
		if len(tid) > 8 {
			tid = tid[:8]
		}
		sb.WriteString(fmt.Sprintf("*%s* (%s): $%.2f\n", t.GetName(), tid, toFloat(info["total_equity"])))
	}
	return sb.String(), nil
}

func (a *Agent) handleSignal(sig Signal) {
	if a.brain != nil {
		a.brain.HandleSignal(sig)
	}
}

func (a *Agent) notifyAll(text string) {
	if a.NotifyFunc != nil {
		a.NotifyFunc(0, text)
	}
}

// looksLikeStockQuery returns true if the text likely references stocks rather
// than being a pure crypto/general query. This avoids hitting the Sina search
// API on every single message (saves ~200ms latency + external API call).
func looksLikeStockQuery(text string) bool {
	upper := strings.ToUpper(text)

	// Check for known stock-related Chinese keywords
	stockKeywords := []string{
		"股", "A股", "港股", "美股", "股票", "涨停", "跌停", "大盘",
		"沪指", "深指", "恒指", "纳指", "标普", "道琼斯",
		"茅台", "比亚迪", "宁德", "腾讯", "阿里", "美团", "小米",
		"京东", "百度", "苹果", "特斯拉", "英伟达", "微软", "谷歌",
		"盘前", "盘后", "开盘", "收盘", "涨幅", "跌幅",
	}
	for _, kw := range stockKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}

	// Check for US stock ticker patterns (1-5 uppercase letters not matching crypto)
	for _, word := range strings.Fields(upper) {
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		if len(word) >= 1 && len(word) <= 5 {
			allLetter := true
			for _, c := range word {
				if c < 'A' || c > 'Z' {
					allLetter = false
					break
				}
			}
			if allLetter {
				// Check if it's in the known US ticker map
				if _, ok := usTickerMap[word]; ok {
					return true
				}
			}
		}
	}

	// Check for 6-digit A-share codes or 5-digit HK codes
	for _, w := range strings.Fields(text) {
		w = strings.TrimSpace(w)
		if len(w) == 5 || len(w) == 6 {
			if _, err := strconv.Atoi(w); err == nil {
				return true
			}
		}
	}

	return false
}

func toFloat(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case int32:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}
