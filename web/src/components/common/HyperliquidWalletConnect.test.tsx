import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { HyperliquidWalletConnect } from './HyperliquidWalletConnect'

const mocks = vi.hoisted(() => ({
  getExchangeConfigs: vi.fn(),
  generateWallet: vi.fn(),
  submitHyperliquidApproval: vi.fn(),
  createExchangeEncrypted: vi.fn(),
  updateExchangeConfigsEncrypted: vi.fn(),
  getHyperliquidAgent: vi.fn(),
  getProvider: vi.fn(),
  getProviders: vi.fn(),
}))

vi.mock('../../lib/api', () => ({
  api: {
    getExchangeConfigs: mocks.getExchangeConfigs,
    generateWallet: mocks.generateWallet,
    submitHyperliquidApproval: mocks.submitHyperliquidApproval,
    createExchangeEncrypted: mocks.createExchangeEncrypted,
    updateExchangeConfigsEncrypted: mocks.updateExchangeConfigsEncrypted,
    getHyperliquidAgent: mocks.getHyperliquidAgent,
  },
}))

vi.mock('../../lib/hyperliquidWallet', async (importOriginal) => ({
  ensureWalletSigningAccount: (await importOriginal<typeof import('../../lib/hyperliquidWallet')>()).ensureWalletSigningAccount,
  formatUSDC: (value?: number) =>
    typeof value === 'number' ? value.toFixed(2) : '--',
  getPreferredWalletProvider: () => mocks.getProvider(),
  getWalletProviders: () => mocks.getProviders(),
  subscribeWalletProviders: (
    onChange: (providers: Array<{ label?: string }>) => void
  ) => {
    onChange(mocks.getProviders())
    return vi.fn()
  },
  getWalletProviderName: (provider: { label?: string }) =>
    provider.label || 'Browser wallet',
  getWalletProviderForAddress: () => Promise.resolve(mocks.getProvider()),
  getWalletErrorMessage: (error: unknown, fallback: string) => {
    if (error instanceof Error) return error.message
    if (
      error &&
      typeof error === 'object' &&
      typeof (error as { message?: unknown }).message === 'string'
    ) {
      return String((error as { message: string }).message)
    }
    return fallback
  },
  normalizeAddress: (value: string) => value.toLowerCase(),
  shortAddress: (value?: string) => value || '--',
  signHyperliquidUserAction: async (
    provider: {
      request: (args: {
        method: string
        params?: unknown[]
      }) => Promise<unknown>
    },
    signerAddress: string,
    action: Record<string, unknown>
  ) => {
    const raw = await provider.request({
      method: 'eth_signTypedData_v4',
      params: [signerAddress, '{}'],
    })
    if (typeof raw !== 'string') throw new Error('Invalid test signature')
    return {
      action: { ...action, signatureChainId: '0xa4b1' },
      signature: { r: '0x1', s: '0x2', v: 27 },
    }
  },
}))

describe('Hyperliquid guided connection', () => {
  beforeEach(() => {
    localStorage.clear()
    mocks.getExchangeConfigs.mockReset()
    mocks.getExchangeConfigs.mockResolvedValue([])
    mocks.generateWallet.mockReset()
    mocks.generateWallet.mockResolvedValue({
      address: '0xagent',
      private_key: 'test-agent-key',
    })
    mocks.submitHyperliquidApproval.mockReset()
    mocks.submitHyperliquidApproval.mockResolvedValue(undefined)
    mocks.createExchangeEncrypted.mockReset()
    mocks.createExchangeEncrypted.mockImplementation(async (payload) => {
      mocks.getExchangeConfigs.mockResolvedValue([
        {
          id: 'hyperliquid-1',
          exchange_type: 'hyperliquid',
          enabled: true,
          hyperliquidWalletAddr: payload.hyperliquid_wallet_addr,
          hyperliquidAgentAddress: '0xagent',
          hyperliquidBuilderApproved: true,
        },
      ])
      return { id: 'hyperliquid-1' }
    })
    mocks.updateExchangeConfigsEncrypted.mockReset()
    mocks.updateExchangeConfigsEncrypted.mockResolvedValue(undefined)
    mocks.getHyperliquidAgent.mockReset()
    mocks.getHyperliquidAgent.mockResolvedValue({
      agent: {
        name: 'NOFX Agent',
        address: '0xagent',
        validUntil: Date.now() + 60_000,
      },
      builderApproved: true,
      agents: [
        {
          name: 'NOFX Agent',
          address: '0xagent',
          validUntil: Date.now() + 60_000,
        },
      ],
    })
    mocks.getProvider.mockReset()
    mocks.getProvider.mockReturnValue(undefined)
    mocks.getProviders.mockReset()
    mocks.getProviders.mockImplementation(() => {
      const provider = mocks.getProvider()
      return provider ? [provider] : []
    })
  })

  it('shows one clear current action instead of internal implementation steps', () => {
    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )

    expect(
      screen.getByRole('heading', { name: 'Connect Hyperliquid' })
    ).toBeTruthy()
    expect(screen.getByText('Connect your wallet to begin')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Connect wallet' })).toBeTruthy()

    expect(screen.queryByText('Create a trading key for NOFX')).toBeNull()
    expect(
      screen.queryByText('Approve the small per-trade builder fee')
    ).toBeNull()
    expect(screen.queryByText('Save to NOFX — done')).toBeNull()
  })

  it('explains the authorization outcome before asking for a signature', () => {
    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )

    expect(
      screen.getByText(/trade-only access; it can never withdraw your funds/i)
    ).toBeTruthy()
    expect(screen.getByText(/two wallet approvals/i)).toBeTruthy()
  })

  it('prepares the agent after wallet connect and saves automatically after both approvals', async () => {
    const provider = {
      request: vi.fn(async ({ method }: { method: string }) => {
        if (method === 'eth_requestAccounts') return ['0xMAIN']
        if (method === 'eth_signTypedData_v4') return `0x${'11'.repeat(65)}`
        throw new Error(`Unexpected method: ${method}`)
      }),
      on: vi.fn(),
      removeListener: vi.fn(),
    }
    mocks.getProvider.mockReturnValue(provider)

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Browser wallet' }))
    fireEvent.click(screen.getByRole('button', { name: 'Connect wallet' }))
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Approve trade-only access' })
      ).toBeTruthy()
    )
    expect(mocks.generateWallet).toHaveBeenCalledOnce()

    fireEvent.click(
      screen.getByRole('button', { name: 'Approve trade-only access' })
    )
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Approve fee & finish' })
      ).toBeTruthy()
    )

    fireEvent.click(
      screen.getByRole('button', { name: 'Approve fee & finish' })
    )
    await waitFor(() =>
      expect(screen.getByText('Hyperliquid is ready')).toBeTruthy()
    )
    expect(mocks.createExchangeEncrypted).toHaveBeenCalledOnce()
    expect(screen.queryByRole('button', { name: 'Save connection' })).toBeNull()
  })

  it('clears wallet-bound authorization when the browser wallet account changes', async () => {
    localStorage.setItem(
      'nofx.hyperliquid.connection.v6',
      JSON.stringify({
        mainWallet: '0xmain',
        agentAddress: '0xagent',
        agentApproved: true,
        builderApproved: true,
        savedExchangeId: 'hyperliquid-1',
      })
    )
    mocks.getExchangeConfigs.mockResolvedValue([
      {
        id: 'hyperliquid-1',
        exchange_type: 'hyperliquid',
        enabled: true,
        hyperliquidWalletAddr: '0xmain',
        hyperliquidAgentAddress: '0xagent',
        hyperliquidBuilderApproved: true,
      },
    ])
    let accountsChanged: ((accounts: unknown) => void) | undefined
    mocks.getProvider.mockReturnValue({
      request: vi.fn(),
      on: vi.fn((event, handler) => {
        if (event === 'accountsChanged') accountsChanged = handler
      }),
      removeListener: vi.fn(),
    })

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )
    await screen.findByText('Hyperliquid is ready')

    await act(async () => {
      accountsChanged?.(['0xNEW'])
      await Promise.resolve()
    })

    expect(screen.queryByText('Hyperliquid is ready')).toBeNull()
    expect(screen.getByText('Prepare secure trading access')).toBeTruthy()
    expect(screen.getByText('0xnew')).toBeTruthy()
  })

  it('does not trust cached authorization when the saved agent is not approved on-chain', async () => {
    localStorage.setItem(
      'nofx.hyperliquid.connection.v6',
      JSON.stringify({
        mainWallet: '0xmain',
        agentAddress: '0xsaved-agent',
        agentApproved: true,
        builderApproved: true,
        savedExchangeId: 'hyperliquid-1',
      })
    )
    mocks.getExchangeConfigs.mockResolvedValue([
      {
        id: 'hyperliquid-1',
        exchange_type: 'hyperliquid',
        enabled: true,
        hyperliquidWalletAddr: '0xmain',
        hyperliquidAgentAddress: '0xsaved-agent',
        hyperliquidBuilderApproved: true,
      },
    ])
    mocks.getHyperliquidAgent.mockResolvedValue({
      agent: {
        name: 'NOFX Agent',
        address: '0xold-agent',
        validUntil: Date.now() + 60_000,
      },
      builderApproved: true,
      agents: [
        {
          name: 'NOFX Agent',
          address: '0xold-agent',
          validUntil: Date.now() + 60_000,
        },
      ],
    })

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )

    await waitFor(() =>
      expect(screen.queryByText('Hyperliquid is ready')).toBeNull()
    )
    expect(
      screen.getByRole('button', { name: 'Re-authorize trade-only access' })
    ).toBeTruthy()
  })

  it('does not report Ready after the on-chain builder fee is revoked', async () => {
    localStorage.setItem(
      'nofx.hyperliquid.connection.v6',
      JSON.stringify({
        mainWallet: '0xmain',
        agentAddress: '0xagent',
        agentApproved: true,
        builderApproved: true,
        savedExchangeId: 'hyperliquid-1',
      })
    )
    mocks.getExchangeConfigs.mockResolvedValue([
      {
        id: 'hyperliquid-1',
        exchange_type: 'hyperliquid',
        enabled: true,
        hyperliquidWalletAddr: '0xmain',
        hyperliquidAgentAddress: '0xagent',
        hyperliquidBuilderApproved: true,
      },
    ])
    mocks.getHyperliquidAgent.mockResolvedValue({
      agent: {
        name: 'NOFX Agent',
        address: '0xagent',
        validUntil: Date.now() + 60_000,
      },
      builderApproved: false,
      agents: [
        {
          name: 'NOFX Agent',
          address: '0xagent',
          validUntil: Date.now() + 60_000,
        },
      ],
    })

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )

    await waitFor(() =>
      expect(screen.queryByText('Hyperliquid is ready')).toBeNull()
    )
    expect(screen.getByText('Finish authorization · 2 of 2')).toBeTruthy()
  })

  it('shows the wallet provider error instead of a generic connection failure', async () => {
    mocks.getProvider.mockReturnValue({
      request: vi.fn().mockRejectedValue({
        code: 4001,
        message: 'Request rejected by Rabby',
      }),
      on: vi.fn(),
      removeListener: vi.fn(),
    })

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Browser wallet' }))
    fireEvent.click(screen.getByRole('button', { name: 'Connect wallet' }))

    expect(await screen.findByText('Request rejected by Rabby')).toBeTruthy()
    expect(screen.queryByText('Wallet connection failed')).toBeNull()
  })

  it('lets the user choose which injected wallet extension to connect', async () => {
    const rabby = {
      label: 'Rabby',
      request: vi.fn(),
      on: vi.fn(),
      removeListener: vi.fn(),
    }
    const metamask = {
      label: 'MetaMask',
      request: vi.fn(async ({ method }: { method: string }) => {
        if (method === 'eth_requestAccounts') return ['0xMAIN']
        throw new Error(`Unexpected method: ${method}`)
      }),
      on: vi.fn(),
      removeListener: vi.fn(),
    }
    mocks.getProvider.mockReturnValue(rabby)
    mocks.getProviders.mockReturnValue([rabby, metamask])

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )

    expect(screen.getByRole('button', { name: 'Rabby' })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'MetaMask' }))
    fireEvent.click(screen.getByRole('button', { name: 'Connect wallet' }))

    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Approve trade-only access' })
      ).toBeTruthy()
    )
    expect(metamask.request).toHaveBeenCalledWith({
      method: 'eth_requestAccounts',
    })
    expect(rabby.request).not.toHaveBeenCalled()
  })

  it('does not submit a signature if the wallet account changes while approval is open', async () => {
    let accountsChanged: ((accounts: unknown) => void) | undefined
    let resolveSignature: ((signature: string) => void) | undefined
    const signature = new Promise<string>((resolve) => {
      resolveSignature = resolve
    })
    const provider = {
      request: vi.fn(({ method }: { method: string }) => {
        if (method === 'eth_requestAccounts') return Promise.resolve(['0xMAIN'])
        if (method === 'eth_signTypedData_v4') return signature
        throw new Error(`Unexpected method: ${method}`)
      }),
      on: vi.fn((event, handler) => {
        if (event === 'accountsChanged') accountsChanged = handler
      }),
      removeListener: vi.fn(),
    }
    mocks.getProvider.mockReturnValue(provider)

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Browser wallet' }))
    fireEvent.click(screen.getByRole('button', { name: 'Connect wallet' }))
    await screen.findByRole('button', { name: 'Approve trade-only access' })
    fireEvent.click(
      screen.getByRole('button', { name: 'Approve trade-only access' })
    )
    act(() => accountsChanged?.(['0xNEW']))
    await act(async () => resolveSignature?.(`0x${'11'.repeat(65)}`))

    await waitFor(() =>
      expect(mocks.submitHyperliquidApproval).not.toHaveBeenCalled()
    )
    expect(screen.getByText(/wallet account changed/i)).toBeTruthy()
    expect(screen.queryByText('Hyperliquid is ready')).toBeNull()
  })

  it('keeps the successful save state when post-save refresh fails', async () => {
    const provider = {
      request: vi.fn(async ({ method }: { method: string }) => {
        if (method === 'eth_requestAccounts') return ['0xMAIN']
        if (method === 'eth_signTypedData_v4') return `0x${'11'.repeat(65)}`
        throw new Error(`Unexpected method: ${method}`)
      }),
      on: vi.fn(),
      removeListener: vi.fn(),
    }
    mocks.getProvider.mockReturnValue(provider)

    render(
      <HyperliquidWalletConnect
        language="en"
        isLoggedIn
        variant="inline"
        onSaved={async () => {
          throw new Error('refresh failed')
        }}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Browser wallet' }))
    fireEvent.click(screen.getByRole('button', { name: 'Connect wallet' }))
    await screen.findByRole('button', { name: 'Approve trade-only access' })
    fireEvent.click(
      screen.getByRole('button', { name: 'Approve trade-only access' })
    )
    await screen.findByRole('button', { name: 'Approve fee & finish' })
    fireEvent.click(
      screen.getByRole('button', { name: 'Approve fee & finish' })
    )

    await screen.findByText('Hyperliquid is ready')
    expect(screen.queryByRole('button', { name: 'Save connection' })).toBeNull()
  })

  it('binds readiness to the exact saved exchange when wallet configs are duplicated', async () => {
    localStorage.setItem(
      'nofx.hyperliquid.connection.v6',
      JSON.stringify({
        mainWallet: '0xmain',
        agentAddress: '0xexact-agent',
        agentApproved: true,
        builderApproved: true,
        savedExchangeId: 'exact',
      })
    )
    mocks.getExchangeConfigs.mockResolvedValue([
      {
        id: 'wrong',
        exchange_type: 'hyperliquid',
        enabled: true,
        hyperliquidWalletAddr: '0xmain',
        hyperliquidAgentAddress: '0xwrong-agent',
        hyperliquidBuilderApproved: true,
      },
      {
        id: 'exact',
        exchange_type: 'hyperliquid',
        enabled: true,
        hyperliquidWalletAddr: '0xmain',
        hyperliquidAgentAddress: '0xexact-agent',
        hyperliquidBuilderApproved: true,
      },
    ])
    mocks.getHyperliquidAgent.mockResolvedValue({
      agent: {
        name: 'NOFX Agent',
        address: '0xexact-agent',
        validUntil: Date.now() + 60_000,
      },
      builderApproved: true,
      agents: [
        {
          name: 'NOFX Agent',
          address: '0xwrong-agent',
          validUntil: Date.now() + 60_000,
        },
        {
          name: 'NOFX Agent',
          address: '0xexact-agent',
          validUntil: Date.now() + 60_000,
        },
      ],
    })

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )

    await screen.findByText('Hyperliquid is ready')
  })

  it('does not replace a disabled saved exchange with another wallet duplicate', async () => {
    localStorage.setItem(
      'nofx.hyperliquid.connection.v6',
      JSON.stringify({
        mainWallet: '0xmain',
        agentAddress: '0xdisabled-agent',
        agentApproved: true,
        builderApproved: true,
        savedExchangeId: 'disabled',
      })
    )
    mocks.getExchangeConfigs.mockResolvedValue([
      {
        id: 'enabled-other',
        exchange_type: 'hyperliquid',
        enabled: true,
        hyperliquidWalletAddr: '0xmain',
        hyperliquidAgentAddress: '0xother-agent',
        hyperliquidBuilderApproved: true,
      },
      {
        id: 'disabled',
        exchange_type: 'hyperliquid',
        enabled: false,
        hyperliquidWalletAddr: '0xmain',
        hyperliquidAgentAddress: '0xdisabled-agent',
        hyperliquidBuilderApproved: true,
      },
    ])

    render(
      <HyperliquidWalletConnect language="en" isLoggedIn variant="inline" />
    )

    await waitFor(() =>
      expect(screen.queryByText('Hyperliquid is ready')).toBeNull()
    )
    expect(
      screen.getByRole('button', { name: 'Re-authorize trade-only access' })
    ).toBeTruthy()
  })
})
