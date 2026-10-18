// 因子挖掘接口数据契约，见 docs/research/factor-mining-design.md「落盘文件」。
// 后端直接透传 data/factor_mining/ 下的 JSON，任何字段都可能缺失或为 null。

export type FactorMarket = 'crypto' | 'us'

export type FactorStatus = 'accepted' | 'candidate' | 'redundant' | 'rejected'

export type FactorSource = 'seed' | 'mutation' | 'llm'

export interface FactorMetrics {
  ic?: number | null
  icir?: number | null
  t?: number | null
  ls_ret?: number | null
  ls_sharpe?: number | null
  turnover?: number | null
  n?: number | null
}

export interface FactorEntry {
  id: string
  expr?: string | null
  source?: FactorSource | string | null
  family?: string | null
  horizon?: number | null
  direction?: number | null
  status?: FactorStatus | string | null
  first_seen?: string | null
  run_id?: string | null
  train?: FactorMetrics | null
  val?: FactorMetrics | null
  corr_max?: number | null
  corr_with?: string | null
  ic_monthly?: Array<[string, number | null]> | null
}

export interface HorizonSummary {
  trials?: number | null
  t_req?: number | null
  accepted?: number | null
  candidate?: number | null
  rejected?: number | null
  redundant?: number | null
}

export interface FactorZoo {
  market?: string | null
  generated_at?: string | null
  split?: {
    train?: [string, string] | null
    val?: [string, string] | null
    holdout_sealed?: boolean | null
  } | null
  horizons?: Record<string, HorizonSummary | null> | null
  factors?: FactorEntry[] | null
}

export interface EnsembleMetrics {
  ic?: number | null
  icir?: number | null
  ls_sharpe?: number | null
}

export interface EnsembleHorizon {
  factor_ids?: string[] | null
  train?: EnsembleMetrics | null
  val?: EnsembleMetrics | null
  curve?: { val?: Array<[string, number | null]> | null } | null
}

export interface FactorEnsemble {
  horizons?: Record<string, EnsembleHorizon | null> | null
}

export interface MarketCoverage {
  symbols?: number | null
  start?: string | null
  end?: string | null
  bars?: number | null
  universe?: unknown
  data_updated_at?: string | null
}

export interface FactorMiningStatus {
  updated_at?: string | null
  running?: boolean | null
  current?: {
    market?: string | null
    stage?: string | null
    started_at?: string | null
  } | null
  schedule?: string | null
  markets?: Record<string, MarketCoverage | null> | null
}

export interface FactorMiningOverview {
  status?: FactorMiningStatus | null
  zoo?: Partial<Record<FactorMarket, FactorZoo | null>> | null
  ensemble?: Partial<Record<FactorMarket, FactorEnsemble | null>> | null
}

export interface FactorMiningRun {
  run_id?: string | null
  market?: string | null
  started_at?: string | null
  finished_at?: string | null
  seconds?: number | null
  seed_evaluated?: number | null
  mutation_evaluated?: number | null
  llm_proposed?: number | null
  llm_valid?: number | null
  new_accepted?: number | null
  new_candidate?: number | null
  trials_total?: number | null
  error?: string | null
}
