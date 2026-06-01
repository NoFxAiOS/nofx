import { API_BASE, handleJSONResponse } from './helpers'

export interface GeneratedWallet {
  address: string
  private_key: string
}

export interface HyperliquidConnectConfig {
  builderAddress: string
  builderMaxFee: string
  chain: string
  signatureChainId: string
}

export interface HyperliquidSignature {
  r: string
  s: string
  v: number
}

export interface HyperliquidAccountSummary {
  address: string
  accountValue: number
  withdrawable: number
  totalMarginUsed: number
  unrealizedPnl: number
  openPositions: number
  updatedAt: number
}

export const walletApi = {
  async generateWallet(): Promise<GeneratedWallet> {
    const res = await fetch(`${API_BASE}/wallet/generate`, { method: 'POST' })
    return handleJSONResponse<GeneratedWallet>(res)
  },

  async getHyperliquidConnectConfig(): Promise<HyperliquidConnectConfig> {
    const res = await fetch(`${API_BASE}/hyperliquid/connect-config`)
    return handleJSONResponse<HyperliquidConnectConfig>(res)
  },

  async getHyperliquidAccount(address: string): Promise<HyperliquidAccountSummary> {
    const res = await fetch(`${API_BASE}/hyperliquid/account?address=${encodeURIComponent(address)}`)
    return handleJSONResponse<HyperliquidAccountSummary>(res)
  },

  async submitHyperliquidApproval(action: Record<string, unknown>, nonce: number, signature: HyperliquidSignature) {
    const res = await fetch(`${API_BASE}/hyperliquid/submit-exchange`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action, nonce, signature }),
    })
    return handleJSONResponse<{ success: boolean; response?: unknown }>(res)
  },
}
