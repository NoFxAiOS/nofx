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

export type StrategyControlPolicyMode = 'strict' | 'audit_only' | 'recommend_only';

export interface StrategyControlPolicyConfig {
  mode?: StrategyControlPolicyMode;
}

export interface EntryStructureConfig {
  enabled: boolean;
  require_primary_timeframe: boolean;
  require_adjacent_timeframes: boolean;
  require_support_resistance: boolean;
  require_structural_anchors: boolean;
  require_fibonacci: boolean;
  max_support_levels?: number;
  max_resistance_levels?: number;
  max_anchor_count?: number;
  audit_primary_timeframe?: boolean;
  audit_adjacent_timeframes?: boolean;
  audit_support_resistance?: boolean;
  audit_structural_anchors?: boolean;
  audit_fibonacci?: boolean;
  require_invalidation_target_linkage?: boolean;
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
  protection: ProtectionConfig;
  entry_structure?: EntryStructureConfig;
  prompt_sections?: PromptSectionsConfig;
  strategy_control_policy?: StrategyControlPolicyConfig;
  // Grid trading configuration (only used when strategy_type is 'grid_trading')
  grid_config?: GridStrategyConfig;
}

export type ProtectionMode = 'disabled' | 'manual' | 'ai';
export type ProtectionValueMode = 'disabled' | 'manual' | 'ai';
export type BreakEvenTriggerMode = 'profit_pct' | 'r_multiple';

export interface ProtectionValueSource {
  mode: ProtectionValueMode;
  value: number;
}

export interface FullTPSLConfig {
  enabled: boolean;
  mode: ProtectionMode;
  take_profit_enabled?: boolean;
  stop_loss_enabled?: boolean;
  fallback_max_loss_enabled?: boolean;
  take_profit: ProtectionValueSource;
  stop_loss: ProtectionValueSource;
  fallback_max_loss: ProtectionValueSource;
}

export interface LadderTPSLRule {
  take_profit_pct?: number;
  take_profit_close_ratio_pct?: number;
  stop_loss_pct?: number;
  stop_loss_close_ratio_pct?: number;
}

export interface LadderTPSLConfig {
  enabled: boolean;
  mode: ProtectionMode;
  take_profit_enabled: boolean;
  stop_loss_enabled: boolean;
  take_profit_price: ProtectionValueSource;
  take_profit_size: ProtectionValueSource;
  stop_loss_price: ProtectionValueSource;
  stop_loss_size: ProtectionValueSource;
  fallback_max_loss: ProtectionValueSource;
  rules: LadderTPSLRule[];
}

export type DrawdownEngineMode = 'manual' | 'ai';
export type BreakEvenRunnerPolicy = 'primary' | 'fallback_only' | 'disabled_for_runner';

export interface DrawdownTakeProfitRule {
  min_profit_pct: number;
  max_drawdown_pct: number;
  close_ratio_pct: number;
  poll_interval_seconds: number;
}

export interface DrawdownTakeProfitConfig {
  enabled: boolean;
  /**
   * disabled/manual/ai reflects protection ownership semantics.
   * engine_mode decides whether drawdown behavior stays fixed-rule or structure-driven.
   */
  mode: ProtectionMode;
  engine_mode?: DrawdownEngineMode;
  runner_enabled?: boolean;
  min_runner_keep_pct?: number;
  max_first_reduce_pct?: number;
  break_even_runner_policy?: BreakEvenRunnerPolicy;
  rules: DrawdownTakeProfitRule[];
}

export interface BreakEvenStopConfig {
  enabled: boolean;
  trigger_mode: BreakEvenTriggerMode;
  trigger_value: number;
  offset_pct: number;
}

export interface RegimeFilterConfig {
  enabled: boolean;
  allowed_regimes: string[];
  block_high_funding: boolean;
  max_funding_rate_abs: number;
  block_high_volatility: boolean;
  max_atr14_pct: number;
  require_trend_alignment: boolean;
}

export interface ProtectionConfig {
  full_tp_sl: FullTPSLConfig;
  ladder_tp_sl: LadderTPSLConfig;
  drawdown_take_profit: DrawdownTakeProfitConfig;
  break_even_stop: BreakEvenStopConfig;
  regime_filter: RegimeFilterConfig;
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
