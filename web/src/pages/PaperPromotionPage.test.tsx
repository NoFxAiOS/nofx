import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import PaperPromotionPage from './PaperPromotionPage'
import type { PromotionPair, PromotionReport } from '../types/promotion'

const pair: PromotionPair = {
  id: 'ensemble',
  title: '因子组合实验',
  feature: 'enable_factor_ensemble',
  started_at: '2026-10-01T00:00:00Z',
  elapsed_days: 8.5,
  min_days: 7,
  min_trades: 20,
  candidate_trader_id: 'candidate-id',
  control_trader_id: 'control-id',
  candidate: {
    n: 24,
    net_pnl_usdt: 123.45,
    period_return: 0.12,
    max_drawdown: 0.03,
    win_rate: 0.6,
    avg_r: 0.345,
  },
  control: {
    n: 21,
    net_pnl_usdt: 60.12,
    period_return: 0.06,
    max_drawdown: 0.05,
    win_rate: 0.5,
    avg_r: 0.123,
  },
  live: { trader_id: 'live-id', n: 30, net_pnl_usdt: 90 },
  paired_daily: {
    n_days: 10,
    mean_diff: 0.002,
    ci95: [-0.001, 0.005],
    cumulative_return_diff: 0.06,
  },
  weekly_avg_r: {
    n_weeks: 3,
    mean_diff: 0.222,
    ci95: [-0.1, 0.544],
  },
  verdict: 'ready',
  reasons: ['实验判定依据'],
  warnings: ['candidate：1 笔未计入 R 统计'],
  caveats: '一周样本统计功效有限，请人工确认',
}

function response(pairs: PromotionPair[] = [pair]) {
  const report: PromotionReport = {
    generated_at: '2026-10-20T02:30:00Z',
    pairs,
  }
  return { ok: true, json: async () => report } as Response
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('PaperPromotionPage', () => {
  it.each([
    ['ready', 'text-up'],
    ['hold', 'text-warn'],
    ['insufficient', 'text-fg-2'],
  ] as const)(
    'renders %s with the corresponding badge and report metrics',
    async (verdict, color) => {
      localStorage.setItem('auth_token', 'test-token')
      const fetchMock = vi
        .fn()
        .mockResolvedValue(response([{ ...pair, verdict }]))
      vi.stubGlobal('fetch', fetchMock)
      render(<PaperPromotionPage language="zh" />)

      const card = await screen.findByRole('article', { name: '因子组合实验' })
      const content = within(card)
      expect(content.getByText(verdict)).toHaveClass(color)
      expect(screen.getByText(/模拟盘 ≥7 天仍为正向/)).toHaveTextContent(
        '本页仅提示，不会修改任何配置'
      )
      expect(
        content.getByText('feature: enable_factor_ensemble')
      ).toBeInTheDocument()
      expect(content.getByText('8.50 / 7')).toBeInTheDocument()
      expect(content.getByText('24 / 20')).toBeInTheDocument()
      expect(content.getByText('21 / 20')).toBeInTheDocument()
      expect(
        content
          .getAllByRole('progressbar')
          .every(
            (bar) =>
              bar.getAttribute('aria-valuenow') ===
              bar.getAttribute('aria-valuemax')
          )
      ).toBe(true)
      const table = content.getByRole('table', { name: '候选与对照指标' })
      expect(within(table).getByText('candidate-id')).toBeInTheDocument()
      expect(within(table).getByText('control-id')).toBeInTheDocument()
      for (const metric of [
        '123.45',
        '60.12',
        '12.00%',
        '6.00%',
        '3.00%',
        '5.00%',
        '60.00%',
        '50.00%',
        '0.345',
        '0.123',
      ]) {
        expect(within(table).getByText(metric)).toBeInTheDocument()
      }
      expect(content.getByText(/正式参照（不参与判定）/)).toHaveTextContent(
        'live-id'
      )
      expect(content.getByText('0.20 pp · 10 配对日')).toBeInTheDocument()
      expect(
        content.getByText('95% CI [-0.10 pp, 0.50 pp]')
      ).toBeInTheDocument()
      expect(content.getByText('95% CI [-0.100, 0.544]')).toBeInTheDocument()
      expect(content.getByText('实验判定依据')).toBeInTheDocument()
      expect(content.getByText(/未计入 R 统计/).closest('ul')).toHaveClass(
        'text-warn'
      )
      expect(content.getByText('一周样本统计功效有限，请人工确认')).toHaveClass(
        'text-fg-3'
      )
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/promotion/report',
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: 'Bearer test-token',
          }),
        })
      )
      expect(
        screen.queryByRole('button', { name: /上线|promote/i })
      ).not.toBeInTheDocument()
    }
  )

  it('shows loading, then pair creation guidance for an empty report', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ pairs: [], generated_at: null }),
      })
    )
    render(<PaperPromotionPage language="zh" />)
    expect(screen.getByRole('status')).toHaveTextContent('加载中…')
    expect(await screen.findByText('暂无候选 / 对照实验')).toBeInTheDocument()
    expect(
      screen.getByText('scripts/promotion/create_pair.py …')
    ).toBeInTheDocument()
    expect(screen.getByText(/data\/promotion\/pairs.json/)).toBeInTheDocument()
    expect(screen.queryByRole('article')).not.toBeInTheDocument()
  })

  it.each(['HTTP', 'network'])(
    'shows an error after an %s failure and supports retry',
    async (kind) => {
      const fetchMock = vi.fn()
      if (kind === 'HTTP')
        fetchMock.mockResolvedValueOnce({ ok: false, status: 500 })
      else fetchMock.mockRejectedValueOnce(new Error('offline'))
      fetchMock.mockResolvedValue(response())
      vi.stubGlobal('fetch', fetchMock)
      render(<PaperPromotionPage language="zh" />)
      expect(await screen.findByRole('alert')).toHaveTextContent('接口请求失败')
      expect(screen.queryByText('暂无候选 / 对照实验')).not.toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: '刷新' }))
      expect(await screen.findByRole('article')).toBeInTheDocument()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    }
  )

  it('retains the last successful report when a refresh fails', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(response())
      .mockRejectedValue(new Error('offline'))
    vi.stubGlobal('fetch', fetchMock)
    render(<PaperPromotionPage language="zh" />)
    await screen.findByRole('article')
    fireEvent.click(screen.getByRole('button', { name: '刷新' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('article')).toBeInTheDocument()
  })

  it('renders null statistics and unavailable confidence intervals without a live reference', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        response([
          {
            id: 'new-pair',
            verdict: 'insufficient',
            elapsed_days: 1.5,
            min_days: 7.5,
            min_trades: 5,
            candidate: { n: 2 },
            control: null,
            paired_daily: {
              mean_diff: null,
              ci95: null,
              ci_note: '完整配对日不足 10 日',
            },
            weekly_avg_r: { note: '退出周不足 3 周', ci95: null },
            reasons: ['候选笔数不足'],
          },
        ])
      )
    )
    render(<PaperPromotionPage language="en" />)
    const card = await screen.findByRole('article', { name: 'new-pair' })
    expect(within(card).getAllByText('—').length).toBeGreaterThan(0)
    expect(screen.getByText('完整配对日不足 10 日')).toBeInTheDocument()
    expect(screen.getByText('退出周不足 3 周')).toBeInTheDocument()
    expect(screen.queryByText(/95% CI/)).not.toBeInTheDocument()
    expect(screen.queryByText(/Live reference/)).not.toBeInTheDocument()
    expect(screen.getByText('1.50 / 7.50')).toBeInTheDocument()
    expect(screen.getByText('2 / 5')).toBeInTheDocument()
    expect(
      within(card).getByRole('progressbar', { name: 'Elapsed days / minimum' })
    ).toHaveAttribute('aria-valuenow', '1.5')
  })

  it('refreshes when the page becomes visible', async () => {
    const fetchMock = vi.fn().mockResolvedValue(response())
    vi.stubGlobal('fetch', fetchMock)
    render(<PaperPromotionPage language="zh" />)
    await screen.findByRole('article')
    fireEvent(document, new Event('visibilitychange'))
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2))
  })
})
