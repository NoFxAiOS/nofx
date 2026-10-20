import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { ExchangeConfigModal } from './ExchangeConfigModal'
import { PaperBadge } from './PaperBadge'
import type { Exchange } from '../../types'
import { toast } from 'sonner'
import { t } from '../../i18n/translations'

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }))
// Paper accounts remain selectable when browser credential encryption is unavailable.
vi.mock('../common/WebCryptoEnvironmentCheck', () => ({
  WebCryptoEnvironmentCheck: () => <div>Crypto unavailable</div>,
}))

const cfg = {
  initial_balance: 2500,
  taker_fee_bps: 7,
  maker_fee_bps: 1,
  slippage_bps: 4,
  default_leverage: 9,
}
const account: Exchange = {
  id: 'paper-candidate',
  exchange_type: 'paper',
  account_name: 'candidate',
  name: 'Paper Futures (模拟盘)',
  type: 'paper',
  enabled: true,
  paper_config: cfg,
}

function setup(editing = false) {
  const save = vi.fn().mockResolvedValue(undefined)
  render(
    <ExchangeConfigModal
      allExchanges={editing ? [account] : []}
      editingExchangeId={editing ? account.id : null}
      onSave={save}
      onClose={vi.fn()}
      onDelete={vi.fn()}
      language="zh"
    />
  )
  if (!editing)
    fireEvent.click(screen.getByRole('button', { name: '模拟盘 (Paper)' }))
  return save
}
function submit() {
  fireEvent.submit(
    screen
      .getByRole('button', { name: new RegExp(t('saveConfig', 'zh')) })
      .closest('form')!
  )
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('paper exchange configuration', () => {
  it('renders seed defaults without credentials and saves a new account', () => {
    const save = setup()
    expect(screen.getByLabelText('初始资金 (USDT)')).toHaveValue(1000)
    expect(screen.getByLabelText('Taker 手续费 (bps)')).toHaveValue(5)
    expect(screen.getByLabelText('Maker 手续费 (bps)')).toHaveValue(2)
    expect(screen.getByLabelText('滑点 (bps)')).toHaveValue(3)
    expect(screen.getByLabelText('默认杠杆')).toHaveValue(5)
    expect(screen.queryByText(/API Key/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Secret Key/)).not.toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    fireEvent.change(
      screen.getByLabelText(t('exchangeConfig.accountName', 'zh')),
      { target: { value: 'candidate' } }
    )
    submit()
    expect(save).toHaveBeenCalledWith(
      null,
      'paper',
      'candidate',
      '',
      '',
      '',
      false,
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
      undefined,
      {
        initial_balance: 1000,
        taker_fee_bps: 5,
        maker_fee_bps: 2,
        slippage_bps: 3,
        default_leverage: 5,
      }
    )
  })
  it('restores persisted settings and permits inclusive bps/leverage boundaries', () => {
    const save = setup(true)
    expect(screen.getByLabelText('初始资金 (USDT)')).toHaveValue(2500)
    expect(screen.getByLabelText('默认杠杆')).toHaveValue(9)
    fireEvent.change(screen.getByLabelText('Taker 手续费 (bps)'), {
      target: { value: '0' },
    })
    fireEvent.change(screen.getByLabelText('Maker 手续费 (bps)'), {
      target: { value: '100' },
    })
    fireEvent.change(screen.getByLabelText('默认杠杆'), {
      target: { value: '125' },
    })
    submit()
    expect(save).toHaveBeenCalledOnce()
    expect(save.mock.calls[0][15]).toEqual({
      ...cfg,
      taker_fee_bps: 0,
      maker_fee_bps: 100,
      default_leverage: 125,
    })
  })
  it.each([
    ['初始资金 (USDT)', '0'],
    ['初始资金 (USDT)', ''],
    ['Taker 手续费 (bps)', '-1'],
    ['Maker 手续费 (bps)', '101'],
    ['滑点 (bps)', '-0.1'],
    ['默认杠杆', '126'],
    ['默认杠杆', '1.5'],
  ])('rejects invalid %s = %s', (label, value) => {
    const save = setup(true)
    fireEvent.change(screen.getByLabelText(label), { target: { value } })
    submit()
    expect(save).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledWith(
      expect.stringContaining('初始资金必须大于 0')
    )
  })
  it('requires an account name', () => {
    const save = setup(true)
    fireEvent.change(
      screen.getByLabelText(t('exchangeConfig.accountName', 'zh')),
      { target: { value: ' ' } }
    )
    submit()
    expect(save).not.toHaveBeenCalled()
  })
  it('labels paper accounts in both languages and omits the badge for live accounts', () => {
    const { rerender } = render(
      <PaperBadge exchangeType="paper" language="zh" />
    )
    expect(screen.getByText('模拟盘')).toBeInTheDocument()
    rerender(<PaperBadge exchangeType="paper" language="en" />)
    expect(screen.getByText('Paper')).toBeInTheDocument()
    rerender(<PaperBadge exchangeType="binance" language="en" />)
    expect(screen.queryByText('Paper')).not.toBeInTheDocument()
  })
})
