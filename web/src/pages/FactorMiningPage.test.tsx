import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import FactorMiningPage from './FactorMiningPage'

// recharts 的 ResponsiveContainer 在 jsdom 里没有尺寸，给一个固定尺寸的替身
vi.mock('recharts', async () => {
  const actual = await vi.importActual<typeof import('recharts')>('recharts')
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactElement }) => (
      <div style={{ width: 600, height: 200 }}>{children}</div>
    ),
  }
})

const metrics = (ic: number, t: number) => ({
  ic,
  icir: 0.4,
  t,
  ls_ret: 0.18,
  ls_sharpe: 1.1,
  turnover: 0.62,
  n: 400,
})

const OVERVIEW = {
  status: {
    updated_at: '2026-10-18 02:41:00',
    running: true,
    current: { market: 'crypto', stage: 'mutation' },
    schedule: '每天 02:30',
    markets: {
      crypto: {
        symbols: 80,
        start: '2022-01-01',
        end: '2026-10-17',
        bars: 90000,
      },
      us: { symbols: 40, start: '2021-10-18', end: '2026-10-17', bars: 50000 },
    },
  },
  zoo: {
    crypto: {
      market: 'crypto',
      horizons: {
        '1': { trials: 500, t_req: 3.1234, accepted: 1, candidate: 1 },
        '3': { trials: 1234, t_req: 3.5512, accepted: 2, candidate: 1 },
        '7': { trials: 900, t_req: 3.3, accepted: 0, candidate: 0 },
      },
      factors: [
        {
          id: 'f_rej00001',
          expr: 'Std($ret,20)',
          source: 'seed',
          family: 'volatility',
          horizon: 3,
          direction: 1,
          status: 'rejected',
          train: metrics(0.001, 0.5),
          val: null,
        },
        {
          id: 'f_acc00001',
          expr: 'Mean($close,5)/$close-1',
          source: 'mutation',
          family: 'momentum',
          horizon: 3,
          direction: -1,
          status: 'accepted',
          first_seen: '2026-10-10',
          run_id: 'run-xyz',
          train: metrics(0.031, 3.9),
          val: metrics(0.02, 1.9),
          corr_max: 0.35,
          corr_with: 'f_other',
          ic_monthly: [
            ['2024-01', 0.03],
            ['2024-02', -0.01],
          ],
        },
        {
          id: 'f_acc00002',
          expr: 'Rank($volume)',
          source: 'llm',
          family: 'volume',
          horizon: 3,
          direction: 1,
          status: 'accepted',
          train: { ic: 0.05, t: 5.2, icir: null, ls_ret: null },
          val: { ic: null, t: null },
        },
        {
          id: 'f_cand0001',
          expr: 'Slope($close,10)',
          source: 'seed',
          family: 'momentum',
          horizon: 3,
          direction: 1,
          status: 'candidate',
          train: metrics(0.02, 3.7),
          val: metrics(0.004, 0.8),
        },
        {
          id: 'f_h1only01',
          expr: 'Ref($close,1)',
          source: 'seed',
          family: 'reversal',
          horizon: 1,
          direction: 1,
          status: 'accepted',
          train: metrics(0.04, 4),
          val: metrics(0.03, 2),
        },
      ],
    },
    us: {
      market: 'us',
      horizons: { '5': { trials: 77, t_req: 2.5, accepted: 0, candidate: 0 } },
      factors: [],
    },
  },
  ensemble: {
    crypto: {
      horizons: {
        '3': {
          factor_ids: ['f_acc00001', 'f_acc00002'],
          train: { ic: 0.04, icir: 0.5, ls_sharpe: 1.2 },
          val: { ic: 0.0234, icir: 0.3, ls_sharpe: 0.9 },
          curve: {
            val: [
              ['2025-06-01', 1.0],
              ['2025-06-02', 1.02],
            ],
          },
        },
      },
    },
    us: null,
  },
}

const RUNS = [
  {
    run_id: 'r1',
    market: 'crypto',
    started_at: '2026-10-18 02:30:00',
    seconds: 321.4,
    seed_evaluated: 150,
    mutation_evaluated: 300,
    llm_proposed: 20,
    llm_valid: 15,
    new_accepted: 2,
    new_candidate: 4,
    trials_total: 1234,
    error: null,
  },
  {
    run_id: 'r2',
    market: 'us',
    started_at: '2026-10-17 02:30:00',
    error: 'boom failure',
  },
]

function mockFetch(overview: unknown, runs: unknown) {
  const fn = vi.fn((url: string) => {
    const body = String(url).includes('/runs') ? runs : overview
    return Promise.resolve({
      ok: true,
      status: 200,
      json: () => Promise.resolve(body),
    } as Response)
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

function bodyRows() {
  return screen
    .getAllByTestId(/^factor-row-/)
    .map((r) => r.getAttribute('data-testid')!.replace('factor-row-', ''))
}

describe('FactorMiningPage', () => {
  beforeEach(() => {
    vi.useRealTimers()
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders leaderboard, switches market/horizon, filters, sorts and expands detail', async () => {
    mockFetch(OVERVIEW, RUNS)
    render(<FactorMiningPage language="zh" />)

    // status bar + stats for default crypto / first horizon (1d)
    expect(await screen.findByText(/运行中/)).toBeInTheDocument()
    expect(screen.getByText(/crypto · mutation/)).toBeInTheDocument()
    expect(screen.getByText(/每天 02:30/)).toBeInTheDocument()
    expect(bodyRows()).toEqual(['f_h1only01'])
    expect(screen.getByText('t_req 3.12')).toBeInTheDocument()

    // switch horizon to 3d
    fireEvent.click(screen.getByRole('button', { name: '3天' }))
    expect(screen.getByText('t_req 3.55')).toBeInTheDocument()
    expect(screen.getByText('0.023')).toBeInTheDocument()
    // default order: accepted (t desc) > candidate > rejected
    expect(bodyRows()).toEqual([
      'f_acc00002',
      'f_acc00001',
      'f_cand0001',
      'f_rej00001',
    ])
    // null metrics render as dashes without crashing
    const nullRow = screen.getByTestId('factor-row-f_acc00002')
    expect(within(nullRow).getAllByText('—').length).toBeGreaterThan(0)

    // status filter
    fireEvent.click(screen.getByRole('button', { name: 'candidate' }))
    expect(bodyRows()).toEqual(['f_cand0001'])
    fireEvent.click(screen.getByRole('button', { name: '全部' }))

    // sort by train IC ascending (click twice: desc then asc)
    fireEvent.click(screen.getByRole('button', { name: 'train IC' }))
    expect(bodyRows()[0]).toBe('f_acc00002')
    fireEvent.click(screen.getByRole('button', { name: /^train IC\s*[▼▲]?$/ }))
    expect(bodyRows()[0]).toBe('f_rej00001')

    // search
    fireEvent.change(screen.getByLabelText('搜索表达式'), {
      target: { value: 'slope' },
    })
    expect(bodyRows()).toEqual(['f_cand0001'])
    fireEvent.change(screen.getByLabelText('搜索表达式'), {
      target: { value: '' },
    })

    // expand detail
    fireEvent.click(screen.getByTestId('factor-row-f_acc00001'))
    expect(screen.getByTestId('factor-expr-full')).toHaveTextContent(
      'Mean($close,5)/$close-1'
    )
    expect(screen.getByText('f_other')).toBeInTheDocument()
    expect(screen.getByText(/2026-10-10/)).toBeInTheDocument()
    expect(screen.getByText('run-xyz')).toBeInTheDocument()

    // ensemble card shows factor count
    expect(screen.getByText('组合曲线（val）')).toBeInTheDocument()

    // runs table
    expect(screen.getByText('2026-10-18 02:30:00')).toBeInTheDocument()
    expect(screen.getByText('boom failure')).toHaveClass('text-down')

    // switch to US market: no factors / no ensemble
    fireEvent.click(screen.getByRole('button', { name: '美股' }))
    expect(
      await screen.findByText('暂无 accepted 因子组合')
    ).toBeInTheDocument()
  })

  it('does not crash when every field is null', async () => {
    mockFetch(
      {
        status: null,
        zoo: { crypto: null, us: null },
        ensemble: { crypto: null, us: null },
      },
      []
    )
    render(<FactorMiningPage language="zh" />)
    await waitFor(() =>
      expect(
        screen.getByText('尚未运行过，等待首次定时任务')
      ).toBeInTheDocument()
    )
    expect(screen.getByText('暂无因子数据')).toBeInTheDocument()
    expect(screen.getByText('暂无 accepted 因子组合')).toBeInTheDocument()
    expect(screen.getByText('暂无运行记录')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '美股' }))
    expect(screen.getByText('暂无因子数据')).toBeInTheDocument()
  })

  it('does not crash when overview is entirely null', async () => {
    mockFetch(null, null)
    render(<FactorMiningPage language="en" />)
    await waitFor(() =>
      expect(screen.getByText(/Never run yet/)).toBeInTheDocument()
    )
  })

  describe('forward test', () => {
    const fwBase = {
      accepted_at: '2026-10-01',
      start: '2026-04-20',
      direction: -1,
      n_days: 87,
      min_days: 40,
      last_date: '2026-10-10',
      ic: 0.0312,
      icir: 0.25,
      t: 1.9,
      ls_ret: 0.12,
      ls_sharpe: 0.8,
      turnover: 0.6,
      curve: [
        ['2026-04-20', 0.01],
        ['2026-04-21', 0.02],
      ],
    }

    function overviewWithForward(forwards: Record<string, unknown>) {
      const ov = structuredClone(OVERVIEW) as typeof OVERVIEW & {
        zoo: { crypto: { factors: Array<Record<string, unknown>> } }
      }
      for (const f of ov.zoo.crypto.factors) {
        if (f.id in forwards) f.forward = forwards[f.id as string]
      }
      return ov
    }

    it('shows verdict badges, forward IC and summary line', async () => {
      const ov = overviewWithForward({
        f_acc00001: { ...fwBase, verdict: 'confirmed' },
        f_acc00002: { ...fwBase, verdict: 'failed', ic: -0.011 },
        f_cand0001: { ...fwBase, verdict: 'inconclusive', ic: 0.004 },
        f_rej00001: {
          ...fwBase,
          verdict: 'pending',
          n_days: 12,
          min_days: 40,
          ic: null,
        },
      })
      ;(ov.zoo.crypto.horizons['3'] as Record<string, unknown>).forward = {
        pending: 4,
        confirmed: 5,
        inconclusive: 6,
        failed: 7,
      }
      mockFetch(ov, RUNS)
      render(<FactorMiningPage language="zh" />)
      await screen.findByText(/运行中/)
      fireEvent.click(screen.getByRole('button', { name: '3天' }))

      expect(
        within(screen.getByTestId('factor-fwd-f_acc00001')).getByText('已确认')
      ).toBeInTheDocument()
      expect(
        within(screen.getByTestId('factor-fwd-f_acc00001')).getByText(
          'IC 0.031'
        )
      ).toBeInTheDocument()
      expect(
        within(screen.getByTestId('factor-fwd-f_acc00002')).getByText('已失效')
      ).toBeInTheDocument()
      expect(
        within(screen.getByTestId('factor-fwd-f_cand0001')).getByText('不确定')
      ).toBeInTheDocument()
      expect(
        within(screen.getByTestId('factor-fwd-f_rej00001')).getByText(
          '观察中 12/40 天'
        )
      ).toBeInTheDocument()

      const summary = screen.getByTestId('forward-summary')
      expect(summary).toHaveTextContent(
        '已确认 5 · 观察中 4 · 不确定 6 · 已失效 7'
      )
    })

    it('shows forward detail block with note and pending hint', async () => {
      mockFetch(
        overviewWithForward({
          f_acc00001: { ...fwBase, verdict: 'confirmed' },
          f_rej00001: {
            ...fwBase,
            verdict: 'pending',
            n_days: 12,
            min_days: 40,
            ic: null,
            t: null,
            curve: [],
          },
        }),
        RUNS
      )
      render(<FactorMiningPage language="zh" />)
      await screen.findByText(/运行中/)
      fireEvent.click(screen.getByRole('button', { name: '3天' }))

      fireEvent.click(screen.getByTestId('factor-row-f_acc00001'))
      let block = screen.getByTestId('factor-forward')
      expect(block).toHaveTextContent('起始日：2026-04-20')
      expect(block).toHaveTextContent(
        '该指标不参与任何筛选，实盘参考以此为准。'
      )
      expect(
        within(block).queryByTestId('factor-forward-pending')
      ).not.toBeInTheDocument()

      // pending: hint with remaining days; empty curve does not crash
      fireEvent.click(screen.getByTestId('factor-row-f_rej00001'))
      block = screen.getByTestId('factor-forward')
      expect(
        within(block).getByTestId('factor-forward-pending')
      ).toHaveTextContent('还差 28 天')
      expect(block).toHaveTextContent('暂无前向累计 IC 曲线')
    })

    it('does not crash when forward is missing or null', async () => {
      mockFetch(
        overviewWithForward({
          f_acc00001: null,
          f_acc00002: { verdict: null, curve: null },
          f_cand0001: { verdict: 'pending' },
        }),
        RUNS
      )
      render(<FactorMiningPage language="zh" />)
      await screen.findByText(/运行中/)
      fireEvent.click(screen.getByRole('button', { name: '3天' }))
      expect(screen.queryByTestId('forward-summary')).not.toBeInTheDocument()
      expect(screen.getByTestId('factor-fwd-f_acc00001')).toHaveTextContent('—')
      expect(screen.getByTestId('factor-fwd-f_rej00001')).toHaveTextContent('—')
      expect(screen.getByTestId('factor-fwd-f_acc00002')).toHaveTextContent('—')
      expect(screen.getByTestId('factor-fwd-f_cand0001')).toHaveTextContent(
        '观察中 0/? 天'
      )
      for (const id of ['f_acc00001', 'f_acc00002', 'f_cand0001']) {
        fireEvent.click(screen.getByTestId(`factor-row-${id}`))
      }
      expect(screen.getAllByTestId('factor-forward').length).toBe(1)
    })
  })
})
