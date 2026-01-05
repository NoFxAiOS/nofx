package backtest

import (
	"fmt"
	"strings"
	"time"

	"nofx/market"
	"nofx/store"
)

// AIConfig defines the AI client configuration used in backtesting.
type AIConfig struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	APIKey      string  `json:"key"`
	SecretKey   string  `json:"secret_key,omitempty"`
	BaseURL     string  `json:"base_url,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

type LeverageConfig struct {
	BTCETHLeverage  int `json:"btc_eth_leverage"`
	AltcoinLeverage int `json:"altcoin_leverage"`
}

// BacktestConfig describes the input configuration for a backtest run.
type BacktestConfig struct {
	RunID                string   `json:"run_id"`
	UserID               string   `json:"user_id,omitempty"`
	AIModelID            string   `json:"ai_model_id,omitempty"`
	StrategyID           string   `json:"strategy_id,omitempty"` // Optional: use saved strategy from Strategy Studio
	Symbols              []string `json:"symbols"`
	Timeframes           []string `json:"timeframes"`
	DecisionTimeframe    string   `json:"decision_timeframe"`
	DecisionCadenceNBars int      `json:"decision_cadence_nbars"`
	StartTS              int64    `json:"start_ts"`
	EndTS                int64    `json:"end_ts"`
	InitialBalance       float64  `json:"initial_balance"`
	FeeBps               float64  `json:"fee_bps"`
	SlippageBps          float64  `json:"slippage_bps"`
	FillPolicy           string   `json:"fill_policy"`
	PromptVariant        string   `json:"prompt_variant"`
	PromptTemplate       string   `json:"prompt_template"`
	CustomPrompt         string   `json:"custom_prompt"`
	OverrideBasePrompt   bool     `json:"override_prompt"`
	CacheAI              bool     `json:"cache_ai"`
	ReplayOnly           bool     `json:"replay_only"`

	AICfg    AIConfig       `json:"ai"`
	Leverage LeverageConfig `json:"leverage"`

	SharedAICachePath         string `json:"ai_cache_path,omitempty"`
	CheckpointIntervalBars    int    `json:"checkpoint_interval_bars,omitempty"`
	CheckpointIntervalSeconds int    `json:"checkpoint_interval_seconds,omitempty"`
	ReplayDecisionDir         string `json:"replay_decision_dir,omitempty"`

	// Internal: loaded strategy config (set by Manager when StrategyID is provided)
	loadedStrategy *store.StrategyConfig `json:"-"`
}

// Validate performs validity checks on the configuration and fills in default values.
func (cfg *BacktestConfig) Validate() error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	cfg.RunID = strings.TrimSpace(cfg.RunID)
	if cfg.RunID == "" {
		return fmt.Errorf("run_id cannot be empty")
	}
	cfg.UserID = strings.TrimSpace(cfg.UserID)
	if cfg.UserID == "" {
		cfg.UserID = "default"
	}
	cfg.AIModelID = strings.TrimSpace(cfg.AIModelID)

	if len(cfg.Symbols) == 0 {
		return fmt.Errorf("at least one symbol is required")
	}
	for i, sym := range cfg.Symbols {
		cfg.Symbols[i] = market.Normalize(sym)
	}

	if len(cfg.Timeframes) == 0 {
		cfg.Timeframes = []string{"3m", "15m", "4h"}
	}
	normTF := make([]string, 0, len(cfg.Timeframes))
	for _, tf := range cfg.Timeframes {
		normalized, err := market.NormalizeTimeframe(tf)
		if err != nil {
			return fmt.Errorf("invalid timeframe '%s': %w", tf, err)
		}
		normTF = append(normTF, normalized)
	}
	cfg.Timeframes = normTF

	if cfg.DecisionTimeframe == "" {
		cfg.DecisionTimeframe = cfg.Timeframes[0]
	}
	normalizedDecision, err := market.NormalizeTimeframe(cfg.DecisionTimeframe)
	if err != nil {
		return fmt.Errorf("invalid decision_timeframe: %w", err)
	}
	cfg.DecisionTimeframe = normalizedDecision

	if cfg.DecisionCadenceNBars <= 0 {
		cfg.DecisionCadenceNBars = 20
	}

	if cfg.StartTS <= 0 || cfg.EndTS <= 0 || cfg.EndTS <= cfg.StartTS {
		return fmt.Errorf("invalid start_ts/end_ts")
	}

	if cfg.InitialBalance <= 0 {
		cfg.InitialBalance = 1000
	}

	if cfg.FillPolicy == "" {
		cfg.FillPolicy = FillPolicyNextOpen
	}
	if err := validateFillPolicy(cfg.FillPolicy); err != nil {
		return err
	}

	if cfg.CheckpointIntervalBars <= 0 {
		cfg.CheckpointIntervalBars = 20
	}
	if cfg.CheckpointIntervalSeconds <= 0 {
		cfg.CheckpointIntervalSeconds = 2
	}

	cfg.PromptVariant = strings.TrimSpace(cfg.PromptVariant)
	if cfg.PromptVariant == "" {
		cfg.PromptVariant = "baseline"
	}
	cfg.PromptTemplate = strings.TrimSpace(cfg.PromptTemplate)
	if cfg.PromptTemplate == "" {
		cfg.PromptTemplate = "default"
	}
	cfg.CustomPrompt = strings.TrimSpace(cfg.CustomPrompt)

	if cfg.AICfg.Provider == "" {
		cfg.AICfg.Provider = "inherit"
	}
	if cfg.AICfg.Temperature == 0 {
		cfg.AICfg.Temperature = 0.4
	}

	if cfg.Leverage.BTCETHLeverage <= 0 {
		cfg.Leverage.BTCETHLeverage = 5
	}
	if cfg.Leverage.AltcoinLeverage <= 0 {
		cfg.Leverage.AltcoinLeverage = 5
	}

	return nil
}

// Duration returns the backtest interval duration.
func (cfg *BacktestConfig) Duration() time.Duration {
	if cfg == nil {
		return 0
	}
	return time.Unix(cfg.EndTS, 0).Sub(time.Unix(cfg.StartTS, 0))
}

const (
	// FillPolicyNextOpen uses the open price of the next bar for execution.
	FillPolicyNextOpen = "next_open"
	// FillPolicyBarVWAP uses the approximate VWAP of the current bar for execution.
	FillPolicyBarVWAP = "bar_vwap"
	// FillPolicyMidPrice uses the mid-price (high+low)/2 for execution.
	FillPolicyMidPrice = "mid"
)

func validateFillPolicy(policy string) error {
	switch policy {
	case FillPolicyNextOpen, FillPolicyBarVWAP, FillPolicyMidPrice:
		return nil
	default:
		return fmt.Errorf("unsupported fill_policy '%s'", policy)
	}
}

// SetLoadedStrategy sets the loaded strategy config from database.
func (cfg *BacktestConfig) SetLoadedStrategy(strategy *store.StrategyConfig) {
	cfg.loadedStrategy = strategy
}

// ToStrategyConfig converts BacktestConfig to StrategyConfig for unified prompt generation.
// This ensures backtest uses the same StrategyEngine logic as live trading.
// If a strategy was loaded from database (via StrategyID), it will be used with overrides.
func (cfg *BacktestConfig) ToStrategyConfig() *store.StrategyConfig {
	// If a strategy was loaded from database, use it with some overrides
	if cfg.loadedStrategy != nil {
		result := *cfg.loadedStrategy // Make a copy

		// Override coin source with backtest symbols (回测指定的币对优先)
		if len(cfg.Symbols) > 0 {
			result.CoinSource.SourceType = "static"
			result.CoinSource.StaticCoins = cfg.Symbols
			result.CoinSource.UseAI500 = false
			result.CoinSource.UseOITop = false
		}

		// Override timeframes with backtest config
		if len(cfg.Timeframes) > 0 {
			result.Indicators.Klines.SelectedTimeframes = cfg.Timeframes
			result.Indicators.Klines.PrimaryTimeframe = cfg.Timeframes[0]
			if len(cfg.Timeframes) > 1 {
				result.Indicators.Klines.LongerTimeframe = cfg.Timeframes[len(cfg.Timeframes)-1]
			}
			result.Indicators.Klines.EnableMultiTimeframe = len(cfg.Timeframes) > 1
		}

		// Override leverage with backtest config
		if cfg.Leverage.BTCETHLeverage > 0 {
			result.RiskControl.BTCETHMaxLeverage = cfg.Leverage.BTCETHLeverage
		}
		if cfg.Leverage.AltcoinLeverage > 0 {
			result.RiskControl.AltcoinMaxLeverage = cfg.Leverage.AltcoinLeverage
		}

		// Override custom prompt if provided in backtest config
		if cfg.CustomPrompt != "" {
			result.CustomPrompt = cfg.CustomPrompt
		}

		return &result
	}

	// Fallback: build strategy config from backtest config (original logic)
	primaryTF := "5m"
	longerTF := "4h"
	if len(cfg.Timeframes) > 0 {
		primaryTF = cfg.Timeframes[0]
	}
	if len(cfg.Timeframes) > 1 {
		longerTF = cfg.Timeframes[len(cfg.Timeframes)-1]
	}

	return &store.StrategyConfig{
		CoinSource: store.CoinSourceConfig{
			SourceType: "static",
			StaticCoins: cfg.Symbols,
			UseAI500:   false,
			AI500Limit: len(cfg.Symbols),
			UseOITop:   false,
			OITopLimit: 0,
		},
		Indicators: store.IndicatorConfig{
			Klines: store.KlineConfig{
				PrimaryTimeframe:     primaryTF,
				PrimaryCount:         30,
				LongerTimeframe:      longerTF,
				LongerCount:          10,
				EnableMultiTimeframe: len(cfg.Timeframes) > 1,
				SelectedTimeframes:   cfg.Timeframes,
			},
			EnableRawKlines:   true,
			EnableEMA:         true,
			EnableMACD:        true,
			EnableRSI:         true,
			EnableATR:         true,
			EnableVolume:      true,
			EnableOI:          true,
			EnableFundingRate: true,
			EMAPeriods:        []int{20, 50},
			RSIPeriods:        []int{7, 14},
			ATRPeriods:        []int{14},
		},
		CustomPrompt: cfg.CustomPrompt,
		RiskControl: store.RiskControlConfig{
			MaxPositions:                 3,
			BTCETHMaxLeverage:            cfg.Leverage.BTCETHLeverage,
			AltcoinMaxLeverage:           cfg.Leverage.AltcoinLeverage,
			BTCETHMaxPositionValueRatio:  5.0,
			AltcoinMaxPositionValueRatio: 1.0,
			MaxMarginUsage:               0.9,
			MinPositionSize:              12,
			MinRiskRewardRatio:           3.0,
			MinConfidence:                75,
		},
	}
}
