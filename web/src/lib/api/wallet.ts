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
  /** Spot ("main wallet") USDC balance on Hyperliquid */
  spotUsdc: number
  /** Spot USDC not locked by open orders, transferable to perp */
  spotUsdcAvailable: number
  updatedAt: number
}

export interface HyperliquidAgentInfo {
  name: string
  address: string
  validUntil: number // unix milliseconds
}

export interface HyperliquidAgentResponse {
  agent: HyperliquidAgentInfo | null // the NOFX-managed agent, null when none approved
  agents: HyperliquidAgentInfo[] // every approved agent for the wallet
  builderApproved: boolean // live maxBuilderFee state for the NOFX builder
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

  async getHyperliquidAccount(
    address: string
  ): Promise<HyperliquidAccountSummary> {
    const res = await fetch(
      `${API_BASE}/hyperliquid/account?address=${encodeURIComponent(address)}`
    )
    return handleJSONResponse<HyperliquidAccountSummary>(res)
  },

  async getHyperliquidAgent(
    address: string
  ): Promise<HyperliquidAgentResponse> {
    const res = await fetch(
      `${API_BASE}/hyperliquid/agent?address=${encodeURIComponent(address)}`
    )
    return handleJSONResponse<HyperliquidAgentResponse>(res)
  },

  async submitHyperliquidApproval(
    action: Record<string, unknown>,
    nonce: number,
    signature: HyperliquidSignature
  ) {
    const res = await fetch(`${API_BASE}/hyperliquid/submit-exchange`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action, nonce, signature }),
    })
    return handleJSONResponse<{ success: boolean; response?: unknown }>(res)
  },
}
