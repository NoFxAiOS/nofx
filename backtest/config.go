package backtest

import (
	"fmt"
	"strings"
	"time"

	"nofx/market"
)

// AIConfig 定义回测中使用的 AI 客户端配置。
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

// BacktestConfig 描述一次回测运行的输入配置。
type BacktestConfig struct {
	RunID                string   `json:"run_id"`
	UserID               string   `json:"user_id,omitempty"`
	AIModelID            string   `json:"ai_model_id,omitempty"`
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
}

// Validate 对配置进行合法性检查并填充默认值。
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

// Duration 返回回测区间时长。
func (cfg *BacktestConfig) Duration() time.Duration {
	if cfg == nil {
		return 0
	}
	return time.Unix(cfg.EndTS, 0).Sub(time.Unix(cfg.StartTS, 0))
}

const (
	// FillPolicyNextOpen 使用下一根 K 线的开盘价成交。
	FillPolicyNextOpen = "next_open"
	// FillPolicyBarVWAP 采用当前 K 线的近似 VWAP 成交。
	FillPolicyBarVWAP = "bar_vwap"
	// FillPolicyMidPrice 采用 (high+low)/2 的中间价成交。
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
