// Strategy Studio Types
export interface Strategy {
  id: string;
  name: string;
  description: string;
  is_active: boolean;
  is_default: boolean;
  is_public: boolean;           // 是否在策略市场公开
  config_visible: boolean;      // 配置参数是否公开可见
  config: StrategyConfig;
  created_at: string;
  updated_at: string;
}

// 策略使用统计
export interface StrategyStats {
  clone_count: number;          // 被克隆次数
  active_users: number;         // 当前使用人数
  top_performers?: StrategyPerformer[];  // 收益排行
}

// 策略使用者收益排行
export interface StrategyPerformer {
  user_id: string;
  user_name: string;            // 脱敏后的用户名
  total_pnl_pct: number;        // 总收益率
  total_pnl: number;            // 总收益金额
  win_rate: number;             // 胜率
  trade_count: number;          // 交易次数
  using_since: string;          // 使用开始时间
  rank: number;                 // 排名
}

export interface PromptSectionsConfig {
  role_definition?: string;
  trading_frequency?: string;
  entry_standards?: string;
  decision_process?: string;
}

export interface StrategyConfig {
  // Strategy type: "ai_trading" (default) or "grid_trading"
  strategy_type?: 'ai_trading' | 'grid_trading';
  // Language setting: "zh" for Chinese, "en" for English
  // Determines the language used for data formatting and prompt generation
  language?: 'zh' | 'en';
  coin_source: CoinSourceConfig;
  indicators: IndicatorConfig;
  custom_prompt?: string;
  risk_control: RiskControlConfig;
  prompt_sections?: PromptSectionsConfig;
  // Grid trading configuration (only used when strategy_type is 'grid_trading')
  grid_config?: GridStrategyConfig;
  // Quant model integration (for custom models alongside or instead of AI)
  quant_model_integration?: QuantModelIntegration;
}

// Grid trading specific configuration
export interface GridStrategyConfig {
  // Trading pair (e.g., "BTCUSDT")
  symbol: string;
  // Number of grid levels (5-50)
  grid_count: number;
  // Total investment in USDT
  total_investment: number;
  // Leverage (1-20)
  leverage: number;
  // Upper price boundary (0 = auto-calculate from ATR)
  upper_price: number;
  // Lower price boundary (0 = auto-calculate from ATR)
  lower_price: number;
  // Use ATR to auto-calculate bounds
  use_atr_bounds: boolean;
  // ATR multiplier for bound calculation (default 2.0)
  atr_multiplier: number;
  // Position distribution: "uniform" | "gaussian" | "pyramid"
  distribution: 'uniform' | 'gaussian' | 'pyramid';
  // Maximum drawdown percentage before emergency exit
  max_drawdown_pct: number;
  // Stop loss percentage per position
  stop_loss_pct: number;
  // Daily loss limit percentage
  daily_loss_limit_pct: number;
  // Use maker-only orders for lower fees
  use_maker_only: boolean;
  // Enable automatic grid direction adjustment based on box breakouts
  enable_direction_adjust?: boolean;
  // Direction bias ratio for long_bias/short_bias modes (default 0.7 = 70%/30%)
  direction_bias_ratio?: number;
}

export interface CoinSourceConfig {
  source_type: 'static' | 'ai500' | 'oi_top' | 'oi_low' | 'mixed';
  static_coins?: string[];
  excluded_coins?: string[];   // 排除的币种列表
  use_ai500: boolean;
  ai500_limit?: number;
  use_oi_top: boolean;
  oi_top_limit?: number;
  use_oi_low: boolean;
  oi_low_limit?: number;
  // Note: API URLs are now built automatically using nofxos_api_key from IndicatorConfig
}

export interface IndicatorConfig {
  klines: KlineConfig;
  // Raw OHLCV kline data - required for AI analysis
  enable_raw_klines: boolean;
  // Technical indicators (optional)
  enable_ema: boolean;
  enable_macd: boolean;
  enable_rsi: boolean;
  enable_atr: boolean;
  enable_boll: boolean;
  enable_volume: boolean;
  enable_oi: boolean;
  enable_funding_rate: boolean;
  ema_periods?: number[];
  rsi_periods?: number[];
  atr_periods?: number[];
  boll_periods?: number[];
  external_data_sources?: ExternalDataSource[];

  // ========== NofxOS 数据源统一配置 ==========
  // Unified NofxOS API Key - used for all NofxOS data sources
  nofxos_api_key?: string;

  // 量化数据源（资金流向、持仓变化、价格变化）
  enable_quant_data?: boolean;
  enable_quant_oi?: boolean;
  enable_quant_netflow?: boolean;

  // OI 排行数据（市场持仓量增减排行）
  enable_oi_ranking?: boolean;
  oi_ranking_duration?: string;  // "1h", "4h", "24h"
  oi_ranking_limit?: number;

  // NetFlow 排行数据（机构/散户资金流向排行）
  enable_netflow_ranking?: boolean;
  netflow_ranking_duration?: string;  // "1h", "4h", "24h"
  netflow_ranking_limit?: number;

  // Price 排行数据（涨跌幅排行）
  enable_price_ranking?: boolean;
  price_ranking_duration?: string;  // "1h", "4h", "24h" or "1h,4h,24h"
  price_ranking_limit?: number;
}

export interface KlineConfig {
  primary_timeframe: string;
  primary_count: number;
  longer_timeframe?: string;
  longer_count?: number;
  enable_multi_timeframe: boolean;
  // 新增：支持选择多个时间周期
  selected_timeframes?: string[];
}

export interface ExternalDataSource {
  name: string;
  type: 'api' | 'webhook';
  url: string;
  method: string;
  headers?: Record<string, string>;
  data_path?: string;
  refresh_secs?: number;
}

export interface RiskControlConfig {
  // Max number of coins held simultaneously (CODE ENFORCED)
  max_positions: number;

  // Trading Leverage - exchange leverage for opening positions (AI guided)
  btc_eth_max_leverage: number;    // BTC/ETH max exchange leverage
  altcoin_max_leverage: number;    // Altcoin max exchange leverage

  // Position Value Ratio - single position notional value / account equity (CODE ENFORCED)
  // Max position value = equity × this ratio
  btc_eth_max_position_value_ratio?: number;     // default: 5 (BTC/ETH max position = 5x equity)
  altcoin_max_position_value_ratio?: number;     // default: 1 (Altcoin max position = 1x equity)

  // Risk Parameters
  max_margin_usage: number;        // Max margin utilization, e.g. 0.9 = 90% (CODE ENFORCED)
  min_position_size: number;       // Min position size in USDT (CODE ENFORCED)
  min_risk_reward_ratio: number;   // Min take_profit / stop_loss ratio (AI guided)
  min_confidence: number;          // Min AI confidence to open position (AI guided)
}

// ==================== Quant Model Types ====================

export interface QuantModel {
  id: string;
  user_id: string;
  name: string;
  description: string;
  model_type: 'indicator_based' | 'rule_based' | 'ml_classifier' | 'ensemble';
  version: string;
  is_public: boolean;
  is_active: boolean;
  config: QuantModelConfig;
  // Backtest statistics
  backtest_count: number;
  win_rate: number;
  avg_profit_pct: number;
  max_drawdown_pct: number;
  sharpe_ratio: number;
  // Usage tracking
  usage_count: number;
  last_used_at?: string;
  created_at: string;
  updated_at: string;
}

export interface QuantModelConfig {
  type: 'indicator_based' | 'rule_based' | 'ml_classifier' | 'ensemble';
  indicators?: ModelIndicator[];
  rules?: ModelRule[];
  ml_config?: MLModelConfig;
  ensemble?: EnsembleConfig;
  parameters: ModelParameters;
  signal_config: SignalGenerationConfig;
}

export interface ModelIndicator {
  name: string;      // e.g., "RSI", "MACD", "EMA", "ATR", "BOLL"
  period: number;    // e.g., 14 for RSI
  timeframe: string; // e.g., "1h", "4h", "1d"
  params?: Record<string, number | string | boolean>; // Additional parameters
  weight: number;    // Weight in multi-indicator models
}

export interface ModelRule {
  name: string;
  condition: string;   // e.g., "RSI_14 < 30 AND Close > EMA_20"
  action: 'buy' | 'sell' | 'hold';
  confidence: number;  // 0-100
  priority: number;    // Higher = evaluated first
  stop_loss_pct?: number;
  take_profit_pct?: number;
}

export interface MLModelConfig {
  algorithm: string;      // e.g., "random_forest", "xgboost", "neural_net"
  features: string[];     // Feature names
  class_labels: string[]; // e.g., ["buy", "sell", "hold"]
  model_weights?: Record<string, number>;
  thresholds?: Record<string, number>;   // Decision thresholds
  trained_at?: string;
  training_data?: TrainingDataInfo;
}

export interface TrainingDataInfo {
  start_date: string;
  end_date: string;
  symbols: string[];
  timeframes: string[];
}

export interface EnsembleConfig {
  method: 'weighted_vote' | 'stacking' | 'average';
  model_ids: string[];    // IDs of sub-models
  weights: Record<string, number>; // Weights for each sub-model
  voting_threshold: number; // Min consensus for action
}

export interface ModelParameters {
  lookback_periods: number;       // Bars to look back
  entry_threshold: number;        // Signal threshold for entry
  exit_threshold: number;         // Signal threshold for exit
  max_position_hold_time: number;  // Max bars to hold
  min_position_hold_time: number;  // Min bars before exit
  max_daily_trades: number;       // Daily trade limit
}

export interface SignalGenerationConfig {
  signal_type: 'discrete' | 'continuous' | 'probabilistic';
  min_confidence: number;    // Minimum confidence threshold
  require_confirmation: boolean; // Wait for confirmation candle
  confirmation_delay: number;    // Candles to wait for confirmation
}

// Strategy integration with quant models
export interface StrategyQuantModelLink {
  model_id: string;
  model_name: string;
  mode: 'primary' | 'secondary' | 'ensemble';
  weight: number; // For ensemble mode
  override_params?: Record<string, number | string | boolean>;
}

export interface QuantModelIntegration {
  enabled: boolean;
  primary_model_id?: string;
  secondary_models?: StrategyQuantModelLink[];
  fallback_to_ai: boolean;      // Use AI if model fails
  model_confidence_threshold: number; // Min confidence from model
  backtest_before_live: boolean;
}
