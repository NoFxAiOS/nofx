// Strategy Studio Types
export interface Strategy {
  id: string
  name: string
  description: string
  is_active: boolean
  is_default: boolean
  is_public: boolean // 是否在策略市场公开
  config_visible: boolean // 配置参数是否公开可见
  config: StrategyConfig
  created_at: string
  updated_at: string
}

// 策略使用统计
export interface StrategyStats {
  clone_count: number // 被克隆次数
  active_users: number // 当前使用人数
  top_performers?: StrategyPerformer[] // 收益排行
}

// 策略使用者收益排行
export interface StrategyPerformer {
  user_id: string
  user_name: string // 脱敏后的用户名
  total_pnl_pct: number // 总收益率
  total_pnl: number // 总收益金额
  win_rate: number // 胜率
  trade_count: number // 交易次数
  using_since: string // 使用开始时间
  rank: number // 排名
}

export interface PromptSectionsConfig {
  role_definition?: string
  trading_frequency?: string
  entry_standards?: string
  decision_process?: string
}

export interface StrategyConfig {
  // Strategy type: "ai_trading" (default), "grid_trading" or "us_stock"
  strategy_type?: 'ai_trading' | 'grid_trading' | 'us_stock'
  // Language setting: "zh" for Chinese, "en" for English
  // Determines the language used for data formatting and prompt generation
  language?: 'zh' | 'en'
  coin_source: CoinSourceConfig
  indicators: IndicatorConfig
  custom_prompt?: string
  // Rolling window (days) for the trading stats the AI sees (PF/win-rate).
  // 0/absent = 30 days default, negative = full history.
  stats_window_days?: number
  risk_control: RiskControlConfig
  prompt_sections?: PromptSectionsConfig
  // Grid trading configuration (only used when strategy_type is 'grid_trading')
  grid_config?: GridStrategyConfig
  // US-stock (Binance bStock spot) configuration (only used when strategy_type is 'us_stock')
  stock_config?: StockConfig
}

// US-stock strategy configuration; mirrors Go store.StockConfig JSON tags.
// Numeric 0 / omitted = use the preset default (see StockConfigEditor placeholders).
export interface StockSessions {
  // 09:30-16:00 ET; omitted = true
  regular?: boolean
  // 04:00-09:30 ET
  pre_market?: boolean
  // 16:00-20:00 ET
  after_hours?: boolean
}

export interface StockConfig {
  // Binance spot bStock pairs, e.g. "AAPLBUSDT" (no cap). Required (>=1) for
  // symbol_source "manual"/"hybrid"; ignored when "scan".
  symbols: string[]
  // Candidate selection: "manual" (default) = trade only the symbols above;
  // "scan" = every ~30 min rank all listed bStock pairs by daily
  // price-cycle score and feed the top N long setups to the AI (the symbols
  // above are ignored); "hybrid" = the manual symbols above are always fed
  // on top of the top N long setups (manual names don't consume Top-N
  // slots). Held positions are included in every mode.
  symbol_source?: 'manual' | 'scan' | 'hybrid'
  // scan/hybrid: how many long setups the AI sees (1-50; 0/omitted = 10)
  scan_top_n?: number
  // scan/hybrid: pairs the scanner must never pick
  scan_exclude?: string[]
  // "swing" (default) | "position"
  preset?: 'swing' | 'position'
  sessions: StockSessions
  // Use Yahoo underlying data when bStock klines are too short; omitted = true
  data_fallback_yahoo?: boolean
  // Divergence guard in percent; 0 = default 1.0, negative = disabled
  max_divergence_pct?: number
  max_position_pct?: number
  max_total_exposure_pct?: number
  max_positions?: number
  risk_per_trade_pct?: number
  stop_atr_min?: number
  stop_atr_max?: number
  // Paper trading (no orders); omitted = true
  paper_trading?: boolean
}

// One row of GET /api/usstock/symbols
export interface USStockSymbol {
  symbol: string // "AAPLBUSDT"
  underlying: string // "AAPL"
  base_asset: string // "AAPLB"
}

// Oliver Kell cycle-of-price-action phases on daily bars (scan classification)
export type USStockCyclePhase =
  | 'reversal_extension'
  | 'wedge_pop'
  | 'ema_crossback'
  | 'base_n_break'
  | 'base_forming'
  | 'uptrend'
  | 'exhaustion_extension'
  | 'wedge_drop'
  | 'downtrend'
  | 'neutral'
  | 'insufficient_data'

// One ranked row of GET /api/usstock/scan (long setups first, score desc)
export interface USStockScanResult {
  symbol: string // "NVDABUSDT"
  underlying: string // "NVDA"
  source: string // data provider for the bars, e.g. "yahoo"
  cycle: {
    phase: USStockCyclePhase
    // true = long setup; score is only meaningful then
    long_setup: boolean
    score: number // 0-100 long-setup score
    ext10_atr?: number // (close − EMA10) / ATR14
    dist20_atr?: number // (close − EMA20) / ATR14
    rs63_pct?: number // 63-bar return minus SPY's, %
    vol_ratio?: number // signal-bar volume vs average
    above_sma50?: boolean
    sma50_above_200?: boolean
    bars?: number // daily bars available
    notes?: string[]
  }
}

// Response of GET /api/usstock/scan — the latest scanner ranking
export interface USStockScanReport {
  // true when no report exists yet and a scan was just started
  scanning: boolean
  // ISO timestamp of the last finished scan; null when no report yet
  generated_at: string | null
  results: USStockScanResult[]
  // pairs not scanned, keyed by symbol with the reason
  skipped: Record<string, string>
}

// Grid trading specific configuration
export interface GridStrategyConfig {
  // Trading pair (e.g., "BTCUSDT")
  symbol: string
  // Number of grid levels (5-50)
  grid_count: number
  // Total investment in USDT
  total_investment: number
  // Leverage (1-20)
  leverage: number
  // Upper price boundary (0 = auto-calculate from ATR)
  upper_price: number
  // Lower price boundary (0 = auto-calculate from ATR)
  lower_price: number
  // Use ATR to auto-calculate bounds
  use_atr_bounds: boolean
  // ATR multiplier for bound calculation (default 2.0)
  atr_multiplier: number
  // Position distribution: "uniform" | "gaussian" | "pyramid"
  distribution: 'uniform' | 'gaussian' | 'pyramid'
  // Maximum drawdown percentage before emergency exit
  max_drawdown_pct: number
  // Stop loss percentage per position
  stop_loss_pct: number
  // Daily loss limit percentage
  daily_loss_limit_pct: number
  // Use maker-only orders for lower fees
  use_maker_only: boolean
  // Enable automatic grid direction adjustment based on box breakouts
  enable_direction_adjust?: boolean
  // Direction bias ratio for long_bias/short_bias modes (default 0.7 = 70%/30%)
  direction_bias_ratio?: number
}

export interface CoinSourceConfig {
  source_type:
    | 'static'
    | 'ai500'
    | 'oi_top'
    | 'oi_low'
    | 'piggy_dash'
    | 'short_scan'
    | 'mixed'
  static_coins?: string[]
  excluded_coins?: string[] // 排除的币种列表
  // mixed mode: include the static coin list as one of the sources
  // (legacy configs without this flag always included it)
  use_static?: boolean
  use_ai500: boolean
  ai500_limit?: number
  use_oi_top: boolean
  oi_top_limit?: number
  use_oi_low: boolean
  oi_low_limit?: number
  use_piggy_dash: boolean
  piggy_dash_limit?: number
  piggy_dash_direction?: string // '', 'breakout', 'breakdown'
  // 猪猪冲刺最低信号等级：'weak' | 'medium' | 'strong'；缺省 = medium（低于此等级的信号不进候选池）
  piggy_dash_min_grade?: string
  // 做空扫描: top 24h gainers ranked by short-suitability score (Binance-derived)
  use_short_scan: boolean
  short_scan_limit?: number
  // 做空扫描资金费率拥挤阈值（%，8h 费率）；0/缺省 = 默认 0.03
  short_scan_funding_rate_pct?: number
  // 历史涨幅池窗口（天）：近 N 天每日涨幅 Top20 合并进做空扫描宇宙；0/缺省 = 默认 7，负数 = 关闭
  short_scan_history_days?: number
  // 历史涨幅池每轮额外分析上限（个）；0/缺省 = 默认 30
  short_scan_history_max?: number
  // 做空扫描入池最低分（0-100）；0/缺省 = 默认 55，低于此分的候选不进入候选池
  short_scan_min_score?: number
  // 候选币最低 OI 持仓价值（百万 USD），低于则跳过；0/缺省 = 默认 15M
  min_oi_value_millions?: number
  // Note: API URLs are now built automatically using nofxos_api_key from IndicatorConfig
}

export interface IndicatorConfig {
  klines: KlineConfig
  // Raw OHLCV kline data - required for AI analysis
  enable_raw_klines: boolean
  // Technical indicators (optional)
  enable_ema: boolean
  enable_macd: boolean
  enable_rsi: boolean
  enable_atr: boolean
  enable_boll: boolean
  enable_volume: boolean
  enable_oi: boolean
  enable_funding_rate: boolean
  ema_periods?: number[]
  rsi_periods?: number[]
  atr_periods?: number[]
  boll_periods?: number[]
  external_data_sources?: ExternalDataSource[]

  // ========== NofxOS 数据源统一配置 ==========
  // Unified NofxOS API Key - used for all NofxOS data sources
  nofxos_api_key?: string

  // 量化数据源（资金流向、持仓变化、价格变化）
  enable_quant_data?: boolean
  enable_quant_oi?: boolean
  enable_quant_netflow?: boolean

  // OI 排行数据（市场持仓量增减排行）
  enable_oi_ranking?: boolean
  oi_ranking_duration?: string // "1h", "4h", "24h"
  oi_ranking_limit?: number

  // NetFlow 排行数据（机构/散户资金流向排行）
  enable_netflow_ranking?: boolean
  netflow_ranking_duration?: string // "1h", "4h", "24h"
  netflow_ranking_limit?: number

  // Price 排行数据（涨跌幅排行）
  enable_price_ranking?: boolean
  price_ranking_duration?: string // "1h", "4h", "24h" or "1h,4h,24h"
  price_ranking_limit?: number
}

export interface KlineConfig {
  primary_timeframe: string
  primary_count: number
  longer_timeframe?: string
  longer_count?: number
  enable_multi_timeframe: boolean
  // 新增：支持选择多个时间周期
  selected_timeframes?: string[]
  // Prompt 侧每周期保留的最近已闭合K线根数(0=默认20,负数=完整 primary_count)
  prompt_kline_bars?: number
}

export interface ExternalDataSource {
  name: string
  type: 'api' | 'webhook'
  url: string
  method: string
  headers?: Record<string, string>
  data_path?: string
  refresh_secs?: number
}

export interface RiskControlConfig {
  // Max number of coins held simultaneously (CODE ENFORCED)
  max_positions: number

  // Trading Leverage - exchange leverage ceilings (CODE ENFORCED)
  btc_eth_max_leverage: number // BTC/ETH max exchange leverage
  altcoin_max_leverage: number // Altcoin max exchange leverage

  // Position Value Ratio - single position notional value / account equity (CODE ENFORCED)
  // Max position value = equity × this ratio
  btc_eth_max_position_value_ratio?: number // default: 5 (BTC/ETH max position = 5x equity)
  altcoin_max_position_value_ratio?: number // default: 1 (Altcoin max position = 1x equity)

  // Risk Parameters
  max_margin_usage: number // Max margin utilization, e.g. 0.9 = 90% (CODE ENFORCED)
  min_position_size: number // Min position size in USDT (CODE ENFORCED)
  min_risk_reward_ratio: number // Min take_profit / stop_loss ratio (CODE ENFORCED at open)
  min_confidence: number // Min AI confidence to open position (CODE ENFORCED)

  // Min holding period in minutes before AI-initiated closes are allowed (CODE ENFORCED)
  min_hold_minutes?: number
  // AI closes before this many hours need ≥2 against-direction closed 1h candles (0 = default 4h, negative = disabled) (CODE ENFORCED)
  early_close_min_hours?: number
  // TP menu + exit-mode contract (nil/true = on): the model picks a
  // program-precomputed TP plan (tp_option) and classifies the regime
  // (exit_mode trend|range|quick); false = legacy verbatim first_rr_ge_target
  tp_menu_enabled?: boolean
  // quick exit-mode's time stop in hours — a position still at a loss after
  // this long is closed by the program (0 = default 4h, negative = disabled)
  time_stop_hours?: number
  // 暴涨延伸做多确认门：4h 趋势窗口涨幅 ≥ N% 时做多需回踩确认；0 = 默认 20，负数 = 禁用
  pump_guard_4h_pct?: number
  // 防追高门：入场价顺交易方向偏离 1h EMA20 超过 N% 拒绝开仓；0 = 默认 6，负数 = 禁用
  chase_max_ema20_1h_pct?: number
  // 逆势入场门：入场价在 1h EMA20 错误一侧超过 N% 拒绝开仓；0 = 默认 3，负数 = 禁用
  counter_max_ema20_1h_pct?: number
  // RSI 极端门：1h RSI ≥ N 拒绝做多、≤ 100−N 拒绝做空；0 = 默认 78，负数 = 禁用
  entry_rsi1h_max?: number
  // 防追高门：最近 4h 顺交易方向涨跌超过 N% 拒绝开仓；0 = 默认 6，负数 = 禁用
  chase_max_move4h_pct?: number
  // TP ladder on leveraged PnL%: at ≥ this the program trims 1/3 (0 = default 10, negative = off) (CODE ENFORCED)
  tp_trim_profit_pct?: number
  // At ≥ this PnL% the program closes the rest (0 = default 25, negative = off) (CODE ENFORCED)
  tp_full_profit_pct?: number
  // R-based trim tier: trim 1/3 at ≥ N× the opening risk (>0 overrides the ROE
  // tier — leverage-independent; negative = trim off; 0 = legacy ROE field)
  tp_trim_at_r?: number
  // R-based full-close tier: close all at ≥ N× the opening risk (>0 overrides
  // the ROE tier; negative = tier off — exits owned by the structure TP algo +
  // trailing stop + drawdown protect; 0 = legacy ROE field)
  tp_full_at_r?: number
  // Block open_short when the 1d trend is up (CODE ENFORCED)
  block_short_1d_uptrend?: boolean
  // Entry timing gate: finest sub-hour TF trend must align with direction (CODE ENFORCED)
  entry_timing_gate?: boolean
  // Position value capped at equity × this % ÷ stop-distance% (default 1.5)
  risk_per_trade_pct?: number
  // Gross stop-risk across all positions and resting entries (0 = default 10%, negative = off)
  max_account_risk_pct?: number
  // Absolute long-vs-short stop-risk imbalance (0 = default 6%, negative = off)
  max_net_directional_risk_pct?: number
  // Stop-distance noise floor: reject stops closer than N × ATR(1h) (0 = off)
  sl_min_atr_mult?: number
  // Limit-entry state machine: AI can emit open_*_limit trigger orders (CODE ENFORCED)
  limit_entry_enabled?: boolean
  // Cancel a limit entry after this many cycles unfilled (default 3)
  limit_entry_max_cycles?: number
  // Limit-entry anchor offset from the snapshot price, % (buy below / sell above; default 0.5)
  limit_entry_offset_pct?: number
  // Convert a limit entry to market when the live price already crossed the
  // anchor (pullback arrived); rejected if price is beyond the SL. Default on.
  limit_entry_market_fallback?: boolean
  // Volatility-targeted sizing with 80/120 hysteresis band (CODE ENFORCED)
  vol_target_enabled?: boolean
  // Rule-based trailing stop: arm at 1.5× stop distance, trail 2×ATR (CODE ENFORCED)
  trailing_stop_enabled?: boolean
  // Drawdown-protect close: peak profit must reach this % before the
  // giveback guard arms (default 5)
  peak_drawdown_min_profit_pct?: number
  // Giveback (of peak) that triggers the protective close (default 55)
  peak_drawdown_max_dd_pct?: number
  // R-based drawdown protect: arm when the peak reaches N× the opening risk
  // (>0 switches the monitor to R units — leverage-independent; 0 = the ROE
  // fields above)
  peak_drawdown_arm_r?: number
  // R-mode giveback: fraction of the peak surrendered that triggers the close
  // (0 = default 0.5)
  peak_drawdown_giveback_r?: number
  // 1R profit lock: at N× the opening risk the SL moves to entry and 50%
  // is trimmed (0 = default 1R, negative = disabled). While ON it
  // supersedes the ROE trim tier (tp_trim_profit_pct).
  profit_lock_at_r?: number
  // 1R lock breakeven offset in R: the stop parks at entry ± N× the
  // opening-risk distance (0 = default 0.2R, negative = pure breakeven).
  profit_lock_be_offset_r?: number
  // Fraction the TP algo closes at the structure level; the remainder runs
  // as a trend-runner under the trailing stop (0 = default 0.5, negative =
  // full close; collapses to full close when trailing_stop_enabled is off).
  tp_close_fraction?: number
  // 止盈上限(R):计划止盈超过「入场价 ± N × 初始风险」时按此截断
  // (0 / 负数 = 不设上限;(0,1) 无效,按不设上限处理) (CODE ENFORCED)
  tp_max_r?: number
  // Hard-block opens when the vendor-vs-live price diverges beyond this %
  // (0 = default 1, negative = disabled) (CODE ENFORCED)
  max_vendor_divergence_pct?: number
}

// ── Config version history (策略版本迭代) ──

// One changed leaf path in a version diff. Old/New are JSON-encoded values
// ("" = field absent on that side); long strings are truncated server-side.
export interface StrategyConfigDiffEntry {
  path: string
  old: string
  new: string
}

// Realized performance of trades entered during a version's active window
// (net of fees — same caliber as the journal stats).
export interface StrategyVersionStats {
  trades: number
  wins: number
  losses: number
  win_rate: number
  net_pnl: number
  avg_win: number
  avg_loss: number
  profit_factor: number
  avg_r: number
  r_count: number
  // review 2026-10-09 I (optional: older servers omit them)
  excluded_manual?: number
  excluded_unattributed?: number
  low_sample?: boolean
  win_rate_lo?: number
  win_rate_hi?: number
  crossed_version?: number
}

export interface StrategyConfigVersion {
  id: number
  config_hash: string
  risk_hash: string
  // create | duplicate | baseline | save | external
  source: string
  changed_at: number // Unix ms UTC
  summary: StrategyConfigDiffEntry[]
  stats: StrategyVersionStats
  risk_control?: Partial<RiskControlConfig>
  coin_source?: Partial<CoinSourceConfig>
}

export interface StrategyVersionsResponse {
  strategy_id: string
  trader_ids: string[]
  current_hash: string
  updated_at: string
  versions: StrategyConfigVersion[]
  version_count: number
}
