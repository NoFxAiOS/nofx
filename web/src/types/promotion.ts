// Read-only scripts/promotion/report.py output. Rates are fractions (0.01 = 1%).
export type PromotionVerdict = 'insufficient' | 'ready' | 'hold'

export interface PromotionStats {
  trader_id?: string | null
  n?: number | null
  net_pnl_usdt?: number | null
  period_return?: number | null
  max_drawdown?: number | null
  win_rate?: number | null
  avg_r?: number | null
}

export interface PromotionComparison {
  mean_diff?: number | null
  cluster_se?: number | null
  ci95?: [number | null, number | null] | null
}

export interface PromotionPair {
  id: string
  title?: string | null
  feature?: string | null
  candidate_trader_id?: string | null
  control_trader_id?: string | null
  live_trader_id?: string | null
  started_at?: string | null
  elapsed_days?: number | null
  min_days?: number | null
  min_trades?: number | null
  candidate?: PromotionStats | null
  control?: PromotionStats | null
  live?: PromotionStats | null
  paired_daily?:
    | (PromotionComparison & {
        n_days?: number | null
        cumulative_return_diff?: number | null
        ci_note?: string | null
      })
    | null
  weekly_avg_r?:
    | (PromotionComparison & {
        n_weeks?: number | null
        note?: string | null
      })
    | null
  verdict?: PromotionVerdict | null
  reasons?: string[] | null
  warnings?: string[] | null
  caveats?: string | null
}

export interface PromotionReport {
  generated_at: string | null
  pairs: PromotionPair[]
  caveats?: string | null
}
