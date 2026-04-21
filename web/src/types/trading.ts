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
  callback_rate?: number
  quantity: number
  status: string
  client_order_id?: string
  protection_role?: string
  protection_status?: string
}

export interface ProtectionRuntimeTier {
  index: number
  min_profit_pct: number
  max_drawdown_pct: number
  close_ratio_pct: number
  activation_price: number
  planned_activation_price?: number
  activation_source?: string
  callback_rate: number
  callback_source?: string
  planned_quantity: number
  source: string
  execution_mode: string
  drawdown_stage?: string
  runner_mode_active?: boolean
  runner_keep_pct?: number
  runner_stop_mode?: string
  runner_stop_price?: number
  runner_stop_source?: string
  runner_target_mode?: string
  runner_target_price?: number
  runner_target_source?: string
  break_even_suppressed_by_runner?: boolean
  is_satisfied?: boolean
  is_triggered?: boolean
}

export interface ProtectionRuntimeRunnerState {
  active?: boolean
  stage?: string
  keep_pct?: number
  stop_mode?: string
  stop_price?: number
  stop_source?: string
  target_mode?: string
  target_price?: number
  target_source?: string
  break_even_suppressed?: boolean
}

export interface ProtectionRuntime {
  protection_state?: string
  break_even_state?: string
  drawdown_execution_mode?: string
  drawdown_config_source?: string
  break_even_execution_mode?: string
  current_pnl_pct?: number
  drawdown_peak_pnl_pct?: number
  current_drawdown_pct?: number
  current_break_even_trigger_pct?: number
  break_even_offset_pct?: number
  next_break_even_gap_pct?: number
  break_even_config_source?: string
  live_break_even_stop_price?: number
  break_even_order_detected?: boolean
  planned_ladder_stop_count?: number
  planned_ladder_take_profit_count?: number
  live_ladder_stop_count?: number
  live_ladder_take_profit_count?: number
  live_full_stop_count?: number
  live_full_take_profit_count?: number
  fallback_order_detected?: boolean
  live_fallback_stop_count?: number
  full_stop_planned?: boolean
  full_take_profit_planned?: boolean
  fallback_planned?: boolean
  ladder_stop_degraded?: boolean
  ladder_take_profit_degraded?: boolean
  ladder_stop_degraded_to_full?: boolean
  ladder_take_profit_degraded_to_full?: boolean
  current_drawdown_stage_min_profit_pct?: number
  current_drawdown_stage_rule_count?: number
  current_drawdown_stage?: string
  drawdown_structure_stage?: string
  drawdown_structure_stop_source?: string
  drawdown_structure_target_source?: string
  drawdown_structure_target_progress?: number
  drawdown_structure_primary_timeframe?: string
  drawdown_structure_evidence?: string[]
  drawdown_structure_trace?: string[]
  structure_protection_health?: string
  structure_protection_drift_reason?: string
  structure_protection_detached?: boolean
  runner_mode_active?: boolean
  runner_keep_pct?: number
  runner_stop_mode?: string
  runner_stop_price?: number
  runner_stop_source?: string
  runner_target_mode?: string
  runner_target_price?: number
  runner_target_source?: string
  break_even_suppressed_by_runner?: boolean
  runner_state?: ProtectionRuntimeRunnerState
  active_orders?: ProtectionRuntimeOrder[]
  active_trailing_orders?: ProtectionRuntimeOrder[]
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
  drawdown_config_source?: string
  break_even_execution_mode?: string
  entry_decision_cycle?: number
  entry_review_summary?: EntryReviewSummary
  entry_structure_audit?: EntryStructureAuditConfig
  protection_runtime?: ProtectionRuntime
}

export interface ProtectionSnapshotValueSource {
  mode?: string
  value?: number
}

export interface ProtectionSnapshotFullTPSL {
  enabled: boolean
  mode: string
  take_profit?: ProtectionSnapshotValueSource
  stop_loss?: ProtectionSnapshotValueSource
  fallback_max_loss?: ProtectionSnapshotValueSource
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
  take_profit_price?: ProtectionSnapshotValueSource
  take_profit_size?: ProtectionSnapshotValueSource
  stop_loss_price?: ProtectionSnapshotValueSource
  stop_loss_size?: ProtectionSnapshotValueSource
  fallback_max_loss?: ProtectionSnapshotValueSource
  rules: ProtectionSnapshotLadderRule[]
}

export interface ProtectionSnapshotDrawdown {
  mode?: string
  source?: string
  stage?: string
  runner_mode_active?: boolean
  runner_keep_pct?: number
  runner_stop_mode?: string
  runner_stop_price?: number
  runner_stop_source?: string
  runner_target_mode?: string
  runner_target_price?: number
  runner_target_source?: string
  break_even_suppressed_by_runner?: boolean
  min_profit_pct: number
  max_drawdown_pct: number
  close_ratio_pct: number
  poll_interval_s: number
}

export interface ProtectionSnapshotBreakEven {
  enabled: boolean
  source?: string
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
  callback_rate?: number
  quantity: number
  status: string
  client_order_id?: string
  protection_role?: string
  protection_status?: string
}

export interface DecisionActionReasonAnchor {
  type?: string
  timeframe?: string
  price?: number
  reason?: string
}

export interface DecisionActionKeyLevels {
  support?: number[]
  resistance?: number[]
  swing_highs?: number[]
  swing_lows?: number[]
  fibonacci?: {
    swing_high?: number
    swing_low?: number
    levels?: number[]
  }
}

export interface DecisionActionRiskRewardSummary {
  entry?: number
  invalidation?: number
  first_target?: number
  gross_estimated_rr?: number
  net_estimated_rr?: number
  passed: boolean
}

export interface DecisionActionProtectionAlignment {
  stop_beyond_invalidation?: boolean
  target_aligned?: boolean
  break_even_before_target?: boolean
  fallback_within_envelope?: boolean
  policy_status?: string
  policy_override?: boolean
  policy_rejected?: boolean
  policy_reasons?: string[]
  notes?: string[]
}

export interface DecisionActionExecutionConstraints {
  tick_size?: number
  price_precision?: number
  qty_step_size?: number
  qty_precision?: number
  min_qty?: number
  min_notional?: number
  contract_value?: number
  mark_price?: number
  last_price?: number
  best_bid?: number
  best_ask?: number
  spread_bps?: number
  taker_fee_rate?: number
  maker_fee_rate?: number
  estimated_slippage_bps?: number
}

export interface DecisionActionControlOutcome {
  decision?: string
  original_action?: string
  final_action?: string
  reasons?: string[]
  failed_checks?: string[]
  constraints_merged?: boolean
  runtime_rr_recomputed?: boolean
  ai_gross_rr?: number
  ai_net_rr?: number
  runtime_gross_rr?: number
  runtime_net_rr?: number
  effective_rr?: number
  effective_rr_source?: string
  execution_constraint_sources?: string[]
  no_order_placed?: boolean
}

export interface DecisionActionReviewContext {
  primary_timeframe?: string
  timeframe_context?: {
    primary?: string
    lower?: string[]
    higher?: string[]
  }
  min_risk_reward?: number
  risk_reward?: DecisionActionRiskRewardSummary
  key_levels?: DecisionActionKeyLevels
  anchors?: DecisionActionReasonAnchor[]
  protection?: DecisionActionProtectionAlignment
  control?: DecisionActionControlOutcome
  execution_constraints?: DecisionActionExecutionConstraints
  alignment_notes?: string[]
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
  review_context?: DecisionActionReviewContext
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
  allow_ai_close?: boolean
  ai_decision_mode?: 'conservative' | 'balanced' | 'aggressive'
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
  allow_ai_close?: boolean
  ai_decision_mode?: 'conservative' | 'balanced' | 'aggressive'
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
  allow_ai_close?: boolean
  ai_decision_mode?: 'conservative' | 'balanced' | 'aggressive'
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
export interface DecisionReviewRef {
  decision_record_id: number
  cycle_number: number
  timestamp: string
  review_context?: Record<string, unknown>
  protection_snapshot?: ProtectionSnapshot
  decisions?: DecisionAction[]
  matched_decision?: DecisionAction
}

export interface EntryReviewSummary {
  timeframe_context?: Record<string, unknown>
  risk_reward?: Record<string, unknown>
  key_levels?: Record<string, unknown>
  anchors?: unknown[]
  alignment_notes?: string[]
}

export interface EntryStructureAuditConfig {
  audit_primary_timeframe?: boolean;
  audit_adjacent_timeframes?: boolean;
  audit_support_resistance?: boolean;
  audit_structural_anchors?: boolean;
  audit_fibonacci?: boolean;
  require_invalidation_target_linkage?: boolean;
}

export interface PositionCloseEvent {
  id: number
  position_id: number
  trader_id: string
  exchange_id: string
  symbol: string
  side: string
  close_reason: string
  execution_source: string
  execution_type: string
  protection_status?: string
  decision_cycle?: number
  decision_review?: DecisionReviewRef
  exchange_order_id: string
  close_quantity: number
  close_ratio_pct: number
  execution_price: number
  close_value_usdt: number
  realized_pnl_delta: number
  fee_delta: number
  event_time: string
}

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
  entry_decision_cycle?: number
  entry_decision_review?: DecisionReviewRef
  entry_review_summary?: EntryReviewSummary
  entry_structure_audit?: EntryStructureAuditConfig
  entry_time: string
  exit_price: number
  exit_order_id: string
  exit_decision_cycle?: number
  exit_decision_review?: DecisionReviewRef
  exit_time: string
  realized_pnl: number
  fee: number
  leverage: number
  status: string
  close_reason: string
  execution_source?: string
  execution_order_type?: string
  close_ratio_pct?: number
  close_value_usdt?: number
  close_events?: PositionCloseEvent[]
  protection_snapshot?: ProtectionSnapshot
  protection_runtime?: ProtectionRuntime
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
