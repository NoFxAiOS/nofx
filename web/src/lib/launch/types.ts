export type LaunchCheckStatus = 'ok' | 'failed' | 'warning' | 'skipped'

export type LaunchCheckId =
  | 'ai_model'
  | 'ai_wallet'
  | 'ai_wallet_funds'
  | 'strategy'
  | 'exchange_config'
  | 'exchange_account'
  | 'exchange_funds'

export interface LaunchCheck {
  id: LaunchCheckId | string
  status: LaunchCheckStatus
  code?: string
  message?: string
  required?: number
  actual?: number
  asset?: string
  address?: string
}

export interface LaunchPreflightResult {
  ready: boolean
  checks: LaunchCheck[]
  min_ai_fee_usdc: number
  min_trading_usdc: number
  checked_at: string
}

/** Guided-setup anchor consumed by the traders page (`?setup=`). */
export type SetupTarget = 'claw402' | 'hyperliquid' | 'hyperliquid-funds'

export type LaunchOutcome =
  | { ok: true; traderId: string; warning?: string }
  | {
      ok: false
      kind: 'preflight'
      message: string
      preflight: LaunchPreflightResult
      setupTarget: SetupTarget | null
    }
  | { ok: false; kind: 'setup'; message: string; setupTarget: SetupTarget }
  | { ok: false; kind: 'error'; message: string }
