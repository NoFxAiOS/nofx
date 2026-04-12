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
  strategy_type?: 'ai_trading' | 'grid_trading'
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

export interface ProtectionRuntimeOrder {
  order_id: string
  type: string
  side: string
  position_side: string
  trigger_price: number
  quantity: number
  status: string
}

export interface ProtectionRuntimeTier {
  index: number
  min_profit_pct: number
  max_drawdown_pct: number
  close_ratio_pct: number
  activation_price: number
  callback_rate: number
  planned_quantity: number
  source: string
  execution_mode: string
}

export interface ProtectionRuntime {
  protection_state?: string
  break_even_state?: string
  drawdown_execution_mode?: string
  break_even_execution_mode?: string
  active_orders?: ProtectionRuntimeOrder[]
  scheduled_tiers?: ProtectionRuntimeTier[]
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
  protection_state?: string
  break_even_state?: string
  // native_trailing_full | native_partial_trailing | managed_partial_drawdown | local_fallback ...
  drawdown_execution_mode?: string
  break_even_execution_mode?: string
  protection_runtime?: ProtectionRuntime
}

export interface ProtectionSnapshotFullTPSL {
  enabled: boolean
  mode: string
  take_profit_pct?: number
  stop_loss_pct?: number
}

export interface ProtectionSnapshotLadderRule {
  take_profit_pct?: number
  take_profit_close_ratio_pct?: number
  stop_loss_pct?: number
  stop_loss_close_ratio_pct?: number
}

export interface ProtectionSnapshotLadder {
  enabled: boolean
  mode: string
  take_profit_enabled: boolean
  stop_loss_enabled: boolean
  rules: ProtectionSnapshotLadderRule[]
}

export interface ProtectionSnapshotDrawdown {
  min_profit_pct: number
  max_drawdown_pct: number
  close_ratio_pct: number
  poll_interval_s: number
}

export interface ProtectionSnapshotBreakEven {
  enabled: boolean
  trigger_mode: string
  trigger_value: number
  offset_pct: number
}

export interface ProtectionSnapshot {
  full_tp_sl?: ProtectionSnapshotFullTPSL
  ladder_tp_sl?: ProtectionSnapshotLadder
  drawdown?: ProtectionSnapshotDrawdown[]
  break_even?: ProtectionSnapshotBreakEven
}

export interface OpenOrder {
  order_id: string
  symbol: string
  side: string
  position_side: string
  type: string
  price: number
  stop_price: number
  quantity: number
  status: string
}

export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  stop_loss?: number      // Stop loss price
  take_profit?: number    // Take profit price
  confidence?: number     // AI confidence (0-100)
  reasoning?: string      // Brief reasoning
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
  decisions: DecisionAction[]
  execution_log: string[]
  protection_snapshot?: ProtectionSnapshot
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
  strategy_id?: string  // 策略ID
  strategy_name?: string  // 策略名称
  is_cross_margin: boolean
  show_in_competition: boolean  // 是否在竞技场显示
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
  avg_win: number
  avg_loss: number
  max_drawdown_pct: number
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
