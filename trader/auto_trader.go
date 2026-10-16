package trader

import (
	"context"
	"errors"
	"fmt"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	_ "nofx/mcp/provider"
	"nofx/store"
	"nofx/trader/aster"
	"nofx/trader/binance"
	binance_stocks "nofx/trader/binance_stocks"
	"nofx/trader/bitget"
	"nofx/trader/bybit"
	"nofx/trader/gate"
	"nofx/trader/hyperliquid"
	"nofx/trader/indodax"
	"nofx/trader/kucoin"
	"nofx/trader/lighter"
	"nofx/trader/okx"
	"nofx/trader/types"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	notify "nofx/telegram/notify"
)

// AutoTraderConfig auto trading configuration (simplified version - AI makes all decisions)
type AutoTraderConfig struct {
	// Trader identification
	ID      string // Trader unique identifier (for log directory, etc.)
	Name    string // Trader display name
	AIModel string // AI model: "qwen" or "deepseek"

	// Trading platform selection
	Exchange   string // Exchange type: "binance", "bybit", "okx", "bitget", "gate", "hyperliquid", "aster" or "lighter"
	ExchangeID string // Exchange account UUID (for multi-account support)

	// Binance API configuration
	BinanceAPIKey          string
	BinanceStocksAPIKey    string
	BinanceStocksSecretKey string
	BinanceSecretKey       string

	// Bybit API configuration
	BybitAPIKey    string
	BybitSecretKey string

	// OKX API configuration
	OKXAPIKey     string
	OKXSecretKey  string
	OKXPassphrase string

	// Bitget API configuration
	BitgetAPIKey     string
	BitgetSecretKey  string
	BitgetPassphrase string

	// Gate API configuration
	GateAPIKey    string
	GateSecretKey string

	// KuCoin API configuration
	KuCoinAPIKey     string
	KuCoinSecretKey  string
	KuCoinPassphrase string

	// Indodax API configuration
	IndodaxAPIKey    string
	IndodaxSecretKey string

	// Hyperliquid configuration
	HyperliquidPrivateKey  string
	HyperliquidWalletAddr  string
	HyperliquidTestnet     bool
	HyperliquidUnifiedAcct bool // Unified Account mode: Spot USDC as Perp collateral

	// Aster configuration
	AsterUser       string // Aster main wallet address
	AsterSigner     string // Aster API wallet address
	AsterPrivateKey string // Aster API wallet private key

	// LIGHTER configuration
	LighterWalletAddr       string // LIGHTER wallet address (L1 wallet)
	LighterPrivateKey       string // LIGHTER L1 private key (for account identification)
	LighterAPIKeyPrivateKey string // LIGHTER API Key private key (40 bytes, for transaction signing)
	LighterAPIKeyIndex      int    // LIGHTER API Key index (0-255)
	LighterTestnet          bool   // Whether to use testnet

	// AI configuration
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// Custom AI API configuration
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// Scan configuration
	ScanInterval time.Duration // Scan interval (recommended 3 minutes)

	// Account configuration
	InitialBalance float64 // Initial balance (for P&L calculation, must be set manually)

	// Risk control (only as hints, AI can make autonomous decisions)
	MaxDailyLoss    float64       // Maximum daily loss percentage (hint)
	MaxDrawdown     float64       // Maximum drawdown percentage (hint)
	StopTradingTime time.Duration // Pause duration after risk control triggers

	// Position mode
	IsCrossMargin bool // true=cross margin mode, false=isolated margin mode

	// Competition visibility
	ShowInCompetition bool // Whether to show in competition page

	// Strategy configuration (use complete strategy config)
	StrategyConfig *store.StrategyConfig // Strategy configuration (includes coin sources, indicators, risk control, prompts, etc.)
	StrategyID     string                // Strategy row ID in the store — lets the cycle self-check compare the DB config against this process's in-memory copy
}

// AutoTrader automatic trader
type AutoTrader struct {
	id                 string // Trader unique identifier
	name               string // Trader display name
	aiModel            string // AI model name
	exchange           string // Trading platform type (binance/bybit/etc)
	exchangeID         string // Exchange account UUID
	showInCompetition  bool   // Whether to show in competition page
	config             AutoTraderConfig
	trader             Trader                       // Use Trader interface (supports multiple platforms)
	cycleGateStates    map[string]*kernel.GateState // THIS cycle's hard-gate verdicts — the executor's plan-parity checks read them (09-19)
	protectionFailures map[string]int
	protectionFaults   map[string]string // guarded by runtimeMu; blocks new account risk until reconciled
	// candidateMeta snapshots this cycle's candidate pool (normalized symbol
	// → candidate): markAIManaged reads it at fill time to attribute each
	// position to its candidate sources. Rebuilt wholesale every cycle
	// (guarded by candidateMetaMu — fills can arrive off the cycle goroutine).
	candidateMetaMu sync.RWMutex
	candidateMeta   map[string]kernel.CandidateCoin
	// recheckDataFn, when set (tests), replaces the strategy-scoped fetch the
	// pending-direction recheck uses; nil = live getMarketTimeframes path.
	recheckDataFn func(symbol string) (*market.Data, error)
	// btcTrendClosesFn, when set (tests), replaces the BTC 4h closes fetch;
	// nil = live kernel.BTC4hTrendCloses.
	btcTrendClosesFn func(limit int) []float64
	// marketDataFn, when set (tests), replaces the generic execution-quote
	// fetch (getMarketData); nil = live GetMarketPrice + GetWithExchange.
	marketDataFn               func(symbol string) (*market.Data, error)
	mcpClient                  mcp.AIClient
	store                      *store.Store           // Data storage (decision records, etc.)
	strategyEngine             *kernel.StrategyEngine // Strategy engine (uses strategy configuration)
	strategyID                 string                 // Strategy row ID — drives the per-cycle config-drift self-check
	loadedConfigHash           string                 // risk_control hash this process LOADED at start (compare against the DB row each cycle)
	cycleNumber                int                    // Current cycle number
	initialBalance             float64
	dailyPnL                   float64
	dayStartDay                string             // UTC day anchor for the daily-loss halt (D2, QUANT_REVIEW 09-22)
	dayStartEquity             float64            // first equity seen on dayStartDay — the halt's baseline
	dailyHaltDay               string             // UTC day the daily-loss halt latched on (held until the next UTC day)
	dailyHaltReason            string             // halt message recorded at trigger time (loss % at that moment)
	cycleRiskReservedUSD       float64            // stop-risk booked by opens that PASSED the exposure gate this cycle (D2 reservation, 09-25 P1)
	cycleLongRiskReservedUSD   float64            // admitted long stop-risk in this cycle; used by net-direction concentration gate
	cycleShortRiskReservedUSD  float64            // admitted short stop-risk in this cycle; used by net-direction concentration gate
	cycleSymbolRiskReservedUSD map[string]float64 // same-cycle stop-risk reserved per symbol
	aiMarksSeededOnce          sync.Once
	customPrompt               string // Custom trading strategy prompt
	overrideBasePrompt         bool   // Whether to override base prompt
	lastResetTime              time.Time
	stopUntil                  time.Time
	isRunning                  bool
	lifecycleMu                sync.Mutex
	executionStateMu           sync.Mutex
	realtimeMu                 sync.RWMutex
	realtime                   *realtimeProtection // review 2026-10-11: Binance-only stream lifecycle
	runVersion                 uint64
	aiCancel                   context.CancelFunc
	runDone                    chan struct{}
	runtimeMu                  sync.RWMutex
	isRunningMutex             sync.RWMutex       // Mutex to protect isRunning flag
	startTime                  time.Time          // System start time
	callCount                  int                // AI call count
	positionFirstSeenTime      map[string]int64   // Position first seen time (symbol_side -> timestamp in milliseconds)
	positionStopLoss           map[string]float64 // Recorded stop-loss per open position (symbol_side -> price), set at open, drives the min-hold hard-exit bypass
	positionStopLossMutex      sync.RWMutex       // Mutex protecting positionStopLoss
	positionInitialStopLoss    map[string]float64 // Opening-risk stop per open position (symbol_side -> price) — write-once 1R anchor; stop adjustments move only positionStopLoss
	stopMonitorCh              chan struct{}      // Used to stop monitoring goroutine
	monitorWg                  sync.WaitGroup     // Used to wait for monitoring goroutine to finish
	peakPnLCache               map[string]float64 // Peak profit cache (symbol -> peak P&L percentage)
	peakPnLCacheMutex          sync.RWMutex       // Cache read-write lock
	tpTrimDone                 map[string]bool    // TP ladder: symbol_side -> 1/3 trim already taken
	r1TrimDone                 map[string]bool    // 1R profit lock: symbol_side -> 50% trim already taken
	partialTrimmed             map[string]float64 // fallback cumulative fraction of ORIGINAL size reduced by AI/automation (DB EntryQuantity is authoritative when available)
	tpTrimMutex                sync.Mutex
	// filterReasons records WHY an actionable proposal was dropped by the
	// executor gates this cycle (symbol|action -> reason) — the Telegram
	// CoT summary used to render a bare "被闸门过滤，未执行" with no cause,
	// forcing a manual log dig every time (LITUSDT 10-01 user report).
	filterReasons         map[string]string
	lastBalanceSyncTime   time.Time                   // Last balance sync time
	userID                string                      // User ID
	gridState             *GridState                  // Grid trading state (only used when StrategyType == "grid_trading")
	consecutiveAIFailures int                         // Consecutive AI call failures
	safeMode              bool                        // Safe mode: no new positions, protect existing ones
	safeModeReason        string                      // Why safe mode was activated
	authBlocked           bool                        // Binance auth/IP rejection (-2015/-2014): trading paused until operator fixes config
	authBlockedReason     string                      // Why auth blocking was activated
	gateNotify            map[string]*gateNotifyState // hard-gate push dedup (symbol → streak)
	gateNotifyMu          sync.Mutex
	pendingEntries        map[string]*pendingEntry // limit-entry state machine (symbol|side → order)
	pendingEntriesMu      sync.RWMutex
	volResizeLast         map[string]time.Time // per-position vol-resize cooldown
	volResizeMu           sync.Mutex
	tpRunnerDoneMap       map[string]bool    // TP-runner conversion done per position (GUARDED BY volResizeMu; cleared with the position lifecycle in ClearPeakPnLCache)
	openTP                map[string]float64 // recorded decision TP per open position (symbol_side)
	openTPMu              sync.RWMutex
	closeIntents          map[string]closeIntent // program close intents for exit classification (symbol|side → why)
	closeIntentsMu        sync.Mutex
	positionExitMode      map[string]string // exit template per open position (symbol_side → trend|range|quick), chosen at open; DB row is the restart authority

	// us_stock (design 2026-10-09): live spot executor (nil in paper mode),
	// injectable scheduler clock (nil = time.Now) and run state.
	stockTrader types.SpotStockTrader
	stockNow    func() time.Time
	stock       stockRuntime

	// keepRestingOnStop: set by StopForRestart — the trader is about to come
	// back (process restart / config reload), so us_stock resting limit entries
	// stay on the exchange and are restored + reconciled on the next start.
	keepRestingOnStop atomic.Bool
}

// Binance hedge legs must use isolated margin for this strategy. Other
// exchanges keep their configured margin mode.
func (at *AutoTrader) entryUsesCrossMargin() bool {
	return at.exchange != "binance" && at.config.IsCrossMargin
}

// NewAutoTrader creates an automatic trader
// st parameter is used to store decision records to database
func NewAutoTrader(config AutoTraderConfig, st *store.Store, userID string) (*AutoTrader, error) {
	if err := config.StrategyConfig.Validate(); err != nil {
		return nil, err
	}
	if err := store.ValidateExchangeEnvironment(config.Exchange, config.HyperliquidTestnet || config.LighterTestnet); err != nil {
		return nil, err
	}
	// Set default values
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	// Initialize AI client based on provider
	var mcpClient mcp.AIClient
	aiModel := config.AIModel
	if config.UseQwen && aiModel == "" {
		aiModel = "qwen"
	}

	// Resolve API key (provider-specific overrides)
	apiKey := config.CustomAPIKey
	customURL := config.CustomAPIURL
	switch aiModel {
	case "qwen":
		if config.QwenKey != "" {
			apiKey = config.QwenKey
		}
	case "deepseek", "":
		if config.DeepSeekKey != "" {
			apiKey = config.DeepSeekKey
		}
	}

	// Create client via registry (covers all registered providers)
	if aiModel == "custom" {
		mcpClient = mcp.New()
	} else if aiModel == "" {
		aiModel = "deepseek"
		mcpClient = mcp.NewAIClientByProvider(aiModel)
	} else {
		mcpClient = mcp.NewAIClientByProvider(aiModel)
	}
	if mcpClient == nil {
		mcpClient = mcp.New()
	}

	mcpClient.SetAPIKey(apiKey, customURL, config.CustomModelName)
	logger.Infof("🤖 [%s] Using %s AI", config.Name, aiModel)

	if config.CustomAPIURL != "" || config.CustomModelName != "" {
		logger.Infof("🔧 [%s] Custom config - URL: %s, Model: %s", config.Name, config.CustomAPIURL, config.CustomModelName)
	}

	// Set default trading platform
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// Create corresponding trader based on configuration
	var trader Trader
	var err error

	// us_stock (design 2026-10-09): spot bStock pairs, no futures init at all
	// (no hedge/margin mode, leverage or futures trader construction).
	stockMode := config.StrategyConfig != nil && config.StrategyConfig.StrategyType == store.StrategyTypeUSStock && config.StrategyConfig.StockConfig != nil
	var stockExec types.SpotStockTrader
	if stockMode {
		if stockExec, err = newStockExecutor(&config); err != nil {
			return nil, err
		}
		if stockExec != nil {
			trader = stockExec
		}
		logger.Infof("🇺🇸 [%s] Using Binance spot bStock trading (paper=%v)", config.Name, config.StrategyConfig.StockConfig.IsPaper())
	} else {
		// Record position mode (general)
		marginModeStr := "Cross Margin"
		if !config.IsCrossMargin {
			marginModeStr = "Isolated Margin"
		}
		logger.Infof("📊 [%s] Position mode: %s", config.Name, marginModeStr)
	}

	switch config.Exchange {
	case "binance":
		if stockMode {
			break
		}
		logger.Infof("🏦 [%s] Using Binance Futures trading", config.Name)
		trader = binance.NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey, userID)
	case "binance_stocks":
		logger.Infof("🏦 [%s] Using Binance Stocks (US equity) trading", config.Name)
		stocksTrader := binance_stocks.NewStocksTrader(config.BinanceStocksAPIKey, config.BinanceStocksSecretKey)
		if err := stocksTrader.EnsureDisclaimer(); err != nil {
			logger.Warnf("⚠️ [%s] %v (the US equity disclaimer must be signed once before trading)", config.Name, err)
		}
		trader = stocksTrader
	case "bybit":
		logger.Infof("🏦 [%s] Using Bybit Futures trading", config.Name)
		trader = bybit.NewBybitTrader(config.BybitAPIKey, config.BybitSecretKey)
	case "okx":
		logger.Infof("🏦 [%s] Using OKX Futures trading", config.Name)
		trader = okx.NewOKXTrader(config.OKXAPIKey, config.OKXSecretKey, config.OKXPassphrase)
	case "bitget":
		logger.Infof("🏦 [%s] Using Bitget Futures trading", config.Name)
		trader = bitget.NewBitgetTrader(config.BitgetAPIKey, config.BitgetSecretKey, config.BitgetPassphrase)
	case "gate":
		logger.Infof("🏦 [%s] Using Gate.io Futures trading", config.Name)
		trader = gate.NewGateTrader(config.GateAPIKey, config.GateSecretKey)
	case "kucoin":
		logger.Infof("🏦 [%s] Using KuCoin Futures trading", config.Name)
		trader = kucoin.NewKuCoinTrader(config.KuCoinAPIKey, config.KuCoinSecretKey, config.KuCoinPassphrase)
	case "hyperliquid":
		logger.Infof("🏦 [%s] Using Hyperliquid trading", config.Name)
		trader, err = hyperliquid.NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet, config.HyperliquidUnifiedAcct)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Hyperliquid trader: %w", err)
		}
	case "aster":
		logger.Infof("🏦 [%s] Using Aster trading", config.Name)
		trader, err = aster.NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Aster trader: %w", err)
		}
	case "lighter":
		logger.Infof("🏦 [%s] Using LIGHTER trading", config.Name)

		if config.LighterWalletAddr == "" || config.LighterAPIKeyPrivateKey == "" {
			return nil, fmt.Errorf("Lighter requires wallet address and API Key private key")
		}

		// Preserve the selected Lighter environment.
		trader, err = lighter.NewLighterTraderV2(
			config.LighterWalletAddr,
			config.LighterAPIKeyPrivateKey,
			config.LighterAPIKeyIndex,
			config.LighterTestnet,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize LIGHTER trader: %w", err)
		}
		logger.Infof("✓ LIGHTER trader initialized successfully")
	case "indodax":
		logger.Infof("🏦 [%s] Using Indodax Spot trading", config.Name)
		trader = indodax.NewIndodaxTrader(config.IndodaxAPIKey, config.IndodaxSecretKey)
	default:
		return nil, fmt.Errorf("unsupported trading platform: %s", config.Exchange)
	}

	// Validate initial balance configuration, auto-fetch from exchange if 0
	if stockMode {
		if config.InitialBalance, err = resolveStockInitialBalance(&config, st, userID, stockExec); err != nil {
			return nil, err
		}
	} else if config.InitialBalance <= 0 {
		logger.Infof("📊 [%s] Initial balance not set, attempting to fetch current balance from exchange...", config.Name)
		account, err := trader.GetBalance()
		if err != nil {
			return nil, fmt.Errorf("initial balance not set and unable to fetch balance from exchange: %w", err)
		}
		// Try multiple balance field names (different exchanges return different formats)
		balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
		var foundBalance float64
		for _, key := range balanceKeys {
			if balance, ok := account[key].(float64); ok && balance > 0 {
				foundBalance = balance
				break
			}
		}
		if foundBalance > 0 {
			config.InitialBalance = foundBalance
			logger.Infof("✓ [%s] Auto-fetched initial balance: %.2f USDT", config.Name, foundBalance)
			// Save to database so it persists across restarts
			if st != nil {
				if err := st.Trader().UpdateInitialBalance(userID, config.ID, foundBalance); err != nil {
					logger.Infof("⚠️  [%s] Failed to save initial balance to database: %v", config.Name, err)
				} else {
					logger.Infof("✓ [%s] Initial balance saved to database", config.Name)
				}
			}
		} else {
			return nil, fmt.Errorf("initial balance must be greater than 0, please set InitialBalance in config or ensure exchange account has balance")
		}
	}

	// Get last cycle number (for recovery)
	var cycleNumber int
	if st != nil {
		cycleNumber, _ = st.Decision().GetLastCycleNumber(config.ID)
		logger.Infof("📊 [%s] Decision records will be stored to database", config.Name)
	}

	// Create strategy engine (must have strategy config)
	if config.StrategyConfig == nil {
		return nil, fmt.Errorf("[%s] strategy not configured", config.Name)
	}
	strategyEngine := kernel.NewStrategyEngine(config.StrategyConfig)
	logger.Infof("✓ [%s] Using strategy engine (strategy configuration loaded)", config.Name)

	at := &AutoTrader{
		id:                      config.ID,
		name:                    config.Name,
		aiModel:                 config.AIModel,
		exchange:                config.Exchange,
		exchangeID:              config.ExchangeID,
		showInCompetition:       config.ShowInCompetition,
		config:                  config,
		trader:                  trader,
		stockTrader:             stockExec,
		mcpClient:               mcpClient,
		store:                   st,
		strategyEngine:          strategyEngine,
		strategyID:              config.StrategyID,
		loadedConfigHash:        RiskControlHash(&config.StrategyConfig.RiskControl),
		cycleNumber:             cycleNumber,
		initialBalance:          config.InitialBalance,
		lastResetTime:           time.Now(),
		startTime:               time.Now(),
		callCount:               0,
		isRunning:               false,
		positionFirstSeenTime:   make(map[string]int64),
		positionStopLoss:        make(map[string]float64),
		positionInitialStopLoss: make(map[string]float64),
		gateNotify:              make(map[string]*gateNotifyState),
		pendingEntries:          make(map[string]*pendingEntry),
		tpRunnerDoneMap:         make(map[string]bool),
		openTP:                  make(map[string]float64),
		closeIntents:            make(map[string]closeIntent),
		positionExitMode:        make(map[string]string),
		stopMonitorCh:           make(chan struct{}),
		monitorWg:               sync.WaitGroup{},
		peakPnLCache:            make(map[string]float64),
		tpTrimDone:              make(map[string]bool),
		r1TrimDone:              make(map[string]bool),
		filterReasons:           make(map[string]string),
		partialTrimmed:          make(map[string]float64),
		peakPnLCacheMutex:       sync.RWMutex{},
		lastBalanceSyncTime:     time.Now(),
		userID:                  userID,
	}
	if stockMode {
		at.stock.cfg = config.StrategyConfig.StockConfig
		at.stock.stopTicks = map[string]int{}
		at.stock.lastProtect = map[string]stockProtect{}
		at.stock.pendings = map[string]*stockPending{}
	}
	registerAccountTrader(at)
	return at, nil
}

// InitialBalance exposes the configured starting capital for stats/reporting.
func (at *AutoTrader) InitialBalance() float64 {
	return at.initialBalance
}

// Run runs the automatic trading main loop
var ErrAlreadyRunning = errors.New("trader is already running")
var ErrRunIntentChanged = errors.New("trader running intent changed")

func (at *AutoTrader) beginRun() error { return at.beginRunWithIntent(nil) }

func (at *AutoTrader) beginRunWithIntent(expected *uint64) error {
	at.lifecycleMu.Lock()
	defer at.lifecycleMu.Unlock()
	if at.runDone != nil {
		select {
		case <-at.runDone:
		default:
			return ErrAlreadyRunning
		}
	}
	if at.store != nil {
		if expected == nil {
			if err := at.store.Trader().UpdateStatus(at.userID, at.id, true); err != nil {
				return err
			}
		}
		row, err := at.store.Trader().GetForUser(at.userID, at.id)
		if err != nil {
			return err
		}
		if expected != nil && (!row.IsRunning || row.RunVersion != *expected) {
			return ErrRunIntentChanged
		}
		at.runVersion = row.RunVersion
	}

	at.runtimeMu.Lock()
	at.startTime = time.Now()
	at.runtimeMu.Unlock()
	at.monitorWg.Add(1)
	at.isRunningMutex.Lock()
	at.stopMonitorCh = make(chan struct{})
	at.runDone = make(chan struct{})
	at.isRunning = true
	aiCtx, cancel := context.WithCancel(context.Background())
	at.aiCancel = cancel
	if client, ok := at.mcpClient.(interface{ SetRequestContext(context.Context) }); ok {
		client.SetRequestContext(aiCtx)
	}
	at.isRunningMutex.Unlock()
	return nil
}

// Start reserves the running state synchronously before dispatching the loop.
func (at *AutoTrader) Start() error {
	if err := at.beginRun(); err != nil {
		return err
	}
	go func() {
		if err := at.run(); err != nil {
			logger.Errorf("Trader %s stopped: %v", at.id, err)
		}
	}()
	return nil
}

func (at *AutoTrader) Run() error {
	if err := at.beginRun(); err != nil {
		return err
	}
	return at.run()
}

func (at *AutoTrader) run() (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("trading loop panic: %v", p)
		}
		at.isRunningMutex.Lock()
		if at.isRunning {
			at.isRunning = false
			close(at.stopMonitorCh)
		}
		at.isRunningMutex.Unlock()
		at.stopRealtimeProtection()
		if syncer, ok := at.trader.(interface{ StopOrderSync() }); ok {
			syncer.StopOrderSync()
		}
		at.monitorWg.Done()
		at.monitorWg.Wait()
		if err != nil && at.store != nil {
			_ = at.store.Trader().StopIfVersion(at.userID, at.id, at.runVersion)
		}
		close(at.runDone)
	}()

	if at.store != nil {
		row, e := at.store.Trader().GetForUser(at.userID, at.id)
		if e != nil || !row.IsRunning || row.RunVersion != at.runVersion {
			return nil
		}
	}
	select {
	case <-at.stopMonitorCh:
		return nil
	default:
	}
	logger.Info("🚀 AI-driven automatic trading system started")
	logger.Infof("💰 Initial balance: %.2f USDT", at.initialBalance)
	logger.Infof("⚙️  Scan interval: %v", at.config.ScanInterval)
	logger.Info("🤖 AI will make full decisions on leverage, position size, stop loss/take profit, etc.")

	at.seedConfigVersionBaseline()

	// us_stock (design 2026-10-09): own scheduler and protection tick; none of
	// the futures-only monitors / reconcilers / order sync below apply.
	if at.IsStockStrategy() {
		return at.runStockLoop()
	}
	if onFuturesMonitorsStart != nil {
		onFuturesMonitorsStart(at)
	}

	// Start drawdown monitoring
	at.startDrawdownMonitor()

	// F6 (2026-10-01 review): fill finalization + protection watchdog on a
	// dedicated ticker — inside a decision cycle they used to wait for the
	// NEXT cycle boundary (ScanInterval plus AI latency) while a just-filled
	// limit entry sat naked.
	at.startProtectionMonitor()

	// Reconcile resting limit entries before the first decision cycle: a
	// restart orphans the in-memory pending state — shadow rows re-claim live
	// orders, offline fills get their protective orders placed, unowned
	// tagged orders get cancelled (see auto_trader_reconcile.go).
	withExecutionLock(at.executionMutex(), func() (bool, error) { at.ReconcilePendingEntries(); return true, nil })
	at.startRealtimeProtection()

	// All adapters, including wrappers such as Binance Stocks, share one
	// lifecycle contract instead of a per-exchange start/stop matrix.
	if syncer, ok := at.trader.(interface {
		StartOrderSync(string, string, string, *store.Store, time.Duration)
	}); ok && at.store != nil {
		if labeler, ok := at.trader.(interface{ SetDisplayName(string) }); ok {
			labeler.SetDisplayName(fmt.Sprintf("%s · %s", at.name, strings.ToUpper(at.aiModel)))
		}
		syncer.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
	}

	timer := time.NewTimer(nextAlignedWait(at.config.ScanInterval, time.Now()))
	defer timer.Stop()
	logger.Infof("🕒 [%s] 周期调度对齐整点: 每 %v,下次 %s",
		at.name, at.config.ScanInterval, time.Now().Add(nextAlignedWait(at.config.ScanInterval, time.Now())).Format("15:04:05"))

	// Check if this is a grid trading strategy
	isGridStrategy := at.IsGridStrategy()
	if isGridStrategy {
		logger.Infof("🔲 [%s] Grid trading strategy detected, initializing grid...", at.name)
		_, initErr := withExecutionLock(at.executionMutex(), func() (bool, error) { return true, at.InitializeGrid() })
		if err := initErr; err != nil {
			logger.Errorf("❌ [%s] Failed to initialize grid: %v", at.name, err)
			return fmt.Errorf("grid initialization failed: %w", err)
		}
	}

	select {
	case <-at.stopMonitorCh:
		return nil
	default:
	}
	// Execute immediately on first run
	if isGridStrategy {
		if err := at.RunGridCycle(); err != nil {
			logger.Infof("❌ Grid execution failed: %v", err)
		}
	} else {
		if err := at.runCycle(); err != nil {
			logger.Infof("❌ Execution failed: %v", err)
		}
	}

	for {
		at.isRunningMutex.RLock()
		running := at.isRunning
		at.isRunningMutex.RUnlock()

		if !running {
			break
		}

		select {
		case <-timer.C:
			if isGridStrategy {
				if err := at.RunGridCycle(); err != nil {
					logger.Infof("❌ Grid execution failed: %v", err)
				}
			} else {
				if err := at.runCycle(); err != nil {
					logger.Infof("❌ Execution failed: %v", err)
				}
			}
			// Re-arm on the next wall-clock boundary — a cycle that overshoots
			// a boundary skips it instead of bursting.
			timer.Reset(nextAlignedWait(at.config.ScanInterval, time.Now()))
		case <-at.stopMonitorCh:
			logger.Infof("[%s] ⏹ Stop signal received, exiting automatic trading main loop", at.name)
			return nil
		}
	}

	return nil
}

// nextAlignedWait returns how long to wait until the next wall-clock interval
// boundary: boundaries are counted from the top of the hour (5m →
// :00/:05/:10…, 7m → :00/:07/:14…), so trader restarts and UI starts land on
// the shared grid instead of drifting with the start moment. Falls back to a
// one-minute wait when the interval is non-positive.
func nextAlignedWait(interval time.Duration, now time.Time) time.Duration {
	if interval <= 0 {
		return time.Minute
	}
	hourStart := now.Truncate(time.Hour)
	next := hourStart.Add((now.Sub(hourStart)/interval + 1) * interval)
	if wait := next.Sub(now); wait > 0 {
		return wait
	}
	// now sits exactly on a boundary (the immediate first run covers it):
	// aim at the following one.
	return interval
}

// StopForRestart stops the trader like Stop but keeps the us_stock resting
// limit entries on the exchange: the caller guarantees the trader is restarted
// right away (process restart, config reload). The next start restores them
// and the live protection tick (every 60s) books any fill and places its stop.
func (at *AutoTrader) StopForRestart() {
	at.keepRestingOnStop.Store(true)
	at.Stop()
}

// Stop stops the automatic trading
func (at *AutoTrader) Stop() {
	at.lifecycleMu.Lock()
	defer at.lifecycleMu.Unlock()
	at.isRunningMutex.Lock()
	wasRunning := at.isRunning
	at.isRunning = false
	if at.aiCancel != nil {
		at.aiCancel()
	}
	if wasRunning {
		close(at.stopMonitorCh)
	}
	at.isRunningMutex.Unlock()

	at.stopRealtimeProtection()
	at.executionMutex().Lock()
	if at.trader != nil && at.gridState != nil && at.config.StrategyConfig != nil && at.config.StrategyConfig.GridConfig != nil {
		if err := at.pauseGrid("trader stopped"); err != nil {
			logger.Errorf("grid stop incomplete: %v", err)
		}
	}
	at.cancelAllPendingEntries("trader stopped")
	at.cleanupStockOnStop()
	at.executionMutex().Unlock()
	if at.runDone != nil {
		<-at.runDone
	} else if syncer, ok := at.trader.(interface{ StopOrderSync() }); ok {
		syncer.StopOrderSync()
	}
	unregisterAccountTrader(at)

	logger.Info("⏹ Automatic trading system stopped")
}

// cancelAllPendingEntries cancels every resting entry order this trader owns.
// Persisted shadow rows are kept for restart reconciliation (dropPendingEntry
// only removes them after a confirmed cancel).
func (at *AutoTrader) cancelAllPendingEntries(cause string) {
	keys := at.snapshotPendingKeys()
	for _, key := range keys {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		pe := at.getPendingEntry(parts[0], parts[1])
		if pe == nil {
			continue
		}
		if err := at.cancelPending(pe); err != nil {
			logger.Warnf("⚠️ [%s] Resting entry %s %s NOT cancelled on %s: %v — the order may still fill unmanaged, manual check advised", at.name, pe.Symbol, pe.Side, cause, err)
			notify.Notify("ALERT", at.name, fmt.Sprintf(
				"<b>⚠️ 停止后入场单仍在 %s</b>\n<i>%s %s 撤单失败(%v)——交易器已停止,该挂单成交将无人保护,请立即人工处理</i>",
				notify.Escape(pe.Symbol), pe.Symbol, pe.Side, notify.Escape(err.Error())))
		} else {
			logger.Infof("📌 [%s] Resting entry %s %s cancelled on %s", at.name, pe.Symbol, pe.Side, cause)
		}
	}
}

// GetID gets trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetUnderlyingTrader returns the underlying Trader interface implementation
// This is used by grid trading and other components that need direct exchange access
func (at *AutoTrader) GetUnderlyingTrader() Trader {
	return at.trader
}

// GetName gets trader name
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel gets AI model
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange gets exchange
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// GetShowInCompetition returns whether trader should be shown in competition
func (at *AutoTrader) GetShowInCompetition() bool {
	at.runtimeMu.RLock()
	defer at.runtimeMu.RUnlock()
	return at.showInCompetition
}

// SetShowInCompetition sets whether trader should be shown in competition
func (at *AutoTrader) SetShowInCompetition(show bool) {
	at.runtimeMu.Lock()
	defer at.runtimeMu.Unlock()
	at.showInCompetition = show
}

// SetCustomPrompt sets custom trading strategy prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.runtimeMu.Lock()
	defer at.runtimeMu.Unlock()
	at.customPrompt = prompt
}

// SetOverrideBasePrompt sets whether to override base prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.runtimeMu.Lock()
	defer at.runtimeMu.Unlock()
	at.overrideBasePrompt = override
}

// GetSystemPromptTemplate gets current system prompt template name (from strategy config)
func (at *AutoTrader) GetSystemPromptTemplate() string {
	if at.strategyEngine != nil {
		config := at.strategyEngine.GetConfig()
		if config.CustomPrompt != "" {
			return "custom"
		}
	}
	return "strategy"
}

// GetStore gets data store (for external access to decision records, etc.)
func (at *AutoTrader) GetStore() *store.Store {
	return at.store
}

// calculatePnLPercentage calculates P&L percentage (based on margin, automatically considers leverage)
// Return rate = Unrealized P&L / Margin x 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// ============================================================================
// AI-managed ownership (user directive 2026-09-25): positions NOT opened by
// the AI are hands-off for the automation — the protection watchdog, the
// vol-target/trailing/1R-lock/breakeven loop and AI close/adjust decisions
// all skip them. Ownership lives in the durable ai_managed_positions table:
// every AI open path marks at fill time; everything unmarked is manual.
// ============================================================================

func (at *AutoTrader) markAIManaged(symbol, side string, orders ...string) {
	if at.store == nil {
		return
	}
	orderID := ""
	if len(orders) > 0 {
		orderID = orders[0]
	}
	if err := at.store.AIManaged().MarkEntry(at.id, symbol, strings.ToLower(side), orderID); err != nil {
		// 2026-10-03 review: a swallowed mark failure classifies the position
		// MANUAL = automation hands-off — exactly what this registry exists
		// to prevent. Surface it.
		logger.Errorf("🤖 [%s] AI-managed MARK FAILED %s %s: %v — position treated as manual (hands-off) until marked", at.name, symbol, side, err)
	} else {
		logger.Infof("🤖 [%s] position marked AI-managed: %s %s — automation owns its lifecycle", at.name, symbol, side)
		at.recordEntryAttribution(symbol, side, orderID)
	}
}

// recordEntryAttribution persists which candidate-pool source(s) produced
// this open (for per-source win-rate stats). Read-only over the cycle's
// candidateMeta snapshot; a symbol absent from it (fill racing the pool
// rebuild, restart in between) records "unknown". Best-effort: a failed
// record is logged and swallowed — it must never affect the open itself.
func (at *AutoTrader) recordEntryAttribution(symbol, side, orderID string) {
	// No order ID = the one-time adoption of pre-existing positions, not a
	// new AI entry — nothing to attribute (and no key to dedupe on).
	if at.store == nil || orderID == "" {
		return
	}
	attr := &store.EntryAttribution{
		TraderID: at.id,
		Symbol:   symbol,
		Side:     strings.ToLower(side),
		OrderID:  orderID,
	}
	at.candidateMetaMu.RLock()
	if coin, ok := at.candidateMeta[market.Normalize(symbol)]; ok {
		attr.Sources = strings.Join(coin.Sources, ",")
		attr.ScannerDirection = coin.ScannerDirection
		attr.ShortScore = coin.ShortScore
		attr.ShortGrade = coin.ShortGrade
		attr.ShortUniverse = coin.ShortUniverse
		attr.ShortConfirmed = coin.ShortConfirmed
	}
	at.candidateMetaMu.RUnlock()
	if attr.Sources == "" {
		attr.Sources = "unknown"
	}
	if err := at.store.EntryAttribution().Record(attr); err != nil {
		logger.Warnf("⚠️ [%s] entry attribution record failed (%s %s): %v", at.name, symbol, side, err)
	}
}

func (at *AutoTrader) unmarkAIManaged(symbol, side string) {
	if at.store == nil {
		return
	}
	// review 2026-10-11: Unmark is a DELETE that succeeds even when no mark
	// existed, so manual positions closing logged a false "AI-managed mark
	// removed" (STRKUSDT 10-11). Only report a mark that was really there.
	wasMarked := at.store.AIManaged().IsMarked(at.id, symbol, strings.ToLower(side))
	if err := at.store.AIManaged().Unmark(at.id, symbol, strings.ToLower(side)); err == nil && wasMarked {
		// Warn-level: an unexpected unmark = automation silently stops
		// managing a live position (the BEATUSDT 09-27 vanish was invisible
		// because this had no log).
		logger.Warnf("🤖 [%s] AI-managed mark removed: %s %s — automation hands-off from here", at.name, symbol, strings.ToLower(side))
	}
}

// isAIManaged reports whether the position belongs to the automation. A
// missing store reads as AI-managed (degrade to old behavior rather than
// dropping protection on everything during store outages).
func (at *AutoTrader) isAIManaged(symbol, side string) bool {
	if at.store == nil {
		return true
	}
	return at.store.AIManaged().IsMarked(at.id, symbol, strings.ToLower(side))
}

// setFilterReason records why an actionable proposal was dropped by an
// executor gate this cycle (consumed by the Telegram CoT summary —
// "被闸门过滤" without a cause forced a manual log dig, LITUSDT 10-01).
func (at *AutoTrader) setFilterReason(d kernel.Decision, reason string) {
	if at.filterReasons == nil {
		at.filterReasons = make(map[string]string)
	}
	at.filterReasons[d.Symbol+"|"+d.Action] = reason
}

// popFilterReason returns and clears the recorded filter reason for one
// proposal ("" when it was not gated).
func (at *AutoTrader) popFilterReason(d kernel.Decision) string {
	r := at.filterReasons[d.Symbol+"|"+d.Action]
	delete(at.filterReasons, d.Symbol+"|"+d.Action)
	return r
}

// StartIfDesired never rewrites a newer stop intent while a reload is finishing.
func (at *AutoTrader) StartIfDesired(version uint64) error {
	if err := at.beginRunWithIntent(&version); err != nil {
		if errors.Is(err, ErrRunIntentChanged) {
			return nil
		}
		return err
	}
	go func() {
		if err := at.run(); err != nil {
			logger.Errorf("trader loop: %v", err)
		}
	}()
	return nil
}

func (at *AutoTrader) SignalStop() {
	at.isRunningMutex.Lock()
	defer at.isRunningMutex.Unlock()
	if at.isRunning {
		if at.aiCancel != nil {
			at.aiCancel()
		}
		at.isRunning = false
		close(at.stopMonitorCh)
	}
}
