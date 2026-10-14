export interface SystemStatus {
  trader_id: string
  trader_name: string
  ai_model: string
  is_running: boolean
  start_time: string
  runtime_minutes: number
  call_count: number
  initial_balance: number
  scan_interval: string
  stop_until: string
  last_reset_time: string
  ai_provider: string
  strategy_type?: 'ai_trading' | 'grid_trading' | 'us_stock'
  grid_symbol?: string
}

export interface AccountInfo {
  total_equity: number
  wallet_balance: number
  unrealized_profit: number // 未实现盈亏（交易所API官方值）
  available_balance: number
  total_pnl: number
  total_pnl_pct: number
  initial_balance: number
  daily_pnl: number
  position_count: number
  margin_used: number
  margin_used_pct: number
}

export interface Position {
  symbol: string
  side: string
  entry_price: number
  mark_price: number
  quantity: number
  leverage: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  liquidation_price: number
  margin_used: number
  // 交易所实查的该仓挂单腿(09-27):SL/TP algo 触发价 + 未成交入场限价;查询失败时缺省
  protection?: {
    sl_price?: number
    tp_price?: number
    limit_price?: number
    // F17 (2026-10-01): 挂单查询失败 → 保护状态未知,前端显式展示「?」,
    // 不得与「无挂单」混同
    protection_error?: boolean
  } | null
}

export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  stop_loss?: number // Stop loss price
  take_profit?: number // Take profit price
  confidence?: number // AI confidence (0-100)
  reasoning?: string // Brief reasoning
  order_id: number
  timestamp: string
  success: boolean
  error?: string
}

export interface AccountSnapshot {
  total_balance: number
  available_balance: number
  total_unrealized_profit: number
  position_count: number
  margin_used_pct: number
}

export interface DecisionRecord {
  timestamp: string
  cycle_number: number
  system_prompt: string
  input_prompt: string
  cot_trace: string
  decision_json: string
  account_state: AccountSnapshot
  positions: any[]
  candidate_coins: string[]
  // 2026-10-10 per-candidate block reasons (absent on old records)
  candidate_verdicts?: Array<{
    symbol: string
    status: 'evaluated' | 'blocked' | 'filtered' | 'held' | string
    long_failed?: string[]
    short_failed?: string[]
    reason?: string
  }>
  decisions: DecisionAction[]
  execution_log: string[]
  success: boolean
  error_message?: string
}

export interface Statistics {
  total_cycles: number
  successful_cycles: number
  failed_cycles: number
  total_open_positions: number
  total_close_positions: number
}

// AI Trading相关类型
export interface TraderInfo {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange_id?: string
  is_running?: boolean
  cycles?: number
  runtime_minutes?: number
  startup_warning?: string
  show_in_competition?: boolean
  strategy_id?: string
  strategy_name?: string
  custom_prompt?: string
  use_ai500?: boolean
  use_oi_top?: boolean
  system_prompt_template?: string
}

// Competition related types
export interface CompetitionTraderData {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange: string
  total_equity: number
  total_pnl: number
  total_pnl_pct: number
  position_count: number
  margin_used_pct: number
  is_running: boolean
}

export interface CompetitionData {
  traders: CompetitionTraderData[]
  count: number
}

// Trader Configuration Data for View Modal
export interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id?: string // 策略ID
  strategy_name?: string // 策略名称
  is_cross_margin: boolean
  show_in_competition: boolean // 是否在竞技场显示
  scan_interval_minutes: number
  initial_balance: number
  is_running: boolean
  // 以下为旧版字段（向后兼容）
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  use_ai500?: boolean
  use_oi_top?: boolean
}

// Position History Types
export interface HistoricalPosition {
  id: number
  trader_id: string
  exchange_id: string
  exchange_type: string
  symbol: string
  side: string
  quantity: number
  entry_quantity: number
  entry_price: number
  entry_order_id: string
  entry_time: string
  exit_price: number
  exit_order_id: string
  exit_time: string
  realized_pnl: number
  fee: number
  leverage: number
  status: string
  close_reason: string
  created_at: string
  updated_at: string
  // Risk geometry (0 / absent on legacy or manual rows)
  initial_stop_loss?: number
  exit_mode?: string
  mae_r?: number
  mfe_r?: number
  ai_managed?: boolean
}

// Matches Go TraderStats struct exactly
export interface TraderStats {
  total_trades: number
  win_trades: number
  loss_trades: number
  win_rate: number
  profit_factor: number
  sharpe_ratio: number
  total_pnl: number
  total_fee: number
  total_win: number
  total_loss: number
  avg_win: number
  avg_loss: number
  max_drawdown_pct: number
  window_days?: number
}

// Matches Go SymbolStats struct exactly
export interface SymbolStats {
  symbol: string
  total_trades: number
  win_trades: number
  win_rate: number
  total_pnl: number
  avg_pnl: number
  avg_hold_mins: number
}

// Matches Go DirectionStats struct exactly
export interface DirectionStats {
  side: string
  trade_count: number
  win_rate: number
  total_pnl: number
  avg_pnl: number
}

export interface PositionHistoryResponse {
  positions: HistoricalPosition[]
  stats: TraderStats | null
  symbol_stats: SymbolStats[]
  direction_stats: DirectionStats[]
}

// Grid Risk Information for frontend display
export interface GridRiskInfo {
  // Leverage info
  current_leverage: number
  effective_leverage: number
  recommended_leverage: number

  // Position info
  current_position: number
  max_position: number
  position_percent: number

  // Liquidation info
  liquidation_price: number
  liquidation_distance: number

  // Market state
  regime_level: string

  // Box state
  short_box_upper: number
  short_box_lower: number
  mid_box_upper: number
  mid_box_lower: number
  long_box_upper: number
  long_box_lower: number
  current_price: number

  // Breakout state
  breakout_level: string
  breakout_direction: string
}

// Program-enforced risk state (GET /api/risk-status) — read-only.
export interface RiskStatus {
  generated_at: string
  equity: number
  day_start_equity: number
  daily_loss_pct: number
  daily_loss_cap_pct: number
  daily_halted: boolean
  initial_balance: number
  account_drawdown_pct: number
  account_drawdown_cap_pct: number
  account_breaker_active: boolean
  reduce_only: boolean
  protection_fault?: string
  loss_streak_enabled: boolean
  loss_streak_max: number
  loss_streak_bans: { symbol: string; until: string }[]
  pending_entries: {
    symbol: string
    side: string
    price: number
    quantity: number
    stop_loss: number
    take_profit: number
    placed_at: string
    filled_qty: number
  }[]
  positions: {
    symbol: string
    side: string
    entry_price: number
    mark_price: number
    initial_stop: number
    current_stop: number
    current_r: number
    stop_locked_r: number
    exit_mode: string
    ai_managed: boolean
    has_initial_stop: boolean
  }[]
}
