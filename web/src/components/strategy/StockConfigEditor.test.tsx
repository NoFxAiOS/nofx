import React, { useState } from 'react'
import { afterEach, expect, test, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import type { StockConfig } from '../../types'
import { StockConfigEditor, defaultStockConfig } from './StockConfigEditor'

const apiMock = vi.hoisted(() => ({
  getUSStockSymbols: vi.fn(),
  getUSStockScan: vi.fn(),
}))
vi.mock('../../lib/api', () => ({ api: apiMock }))

afterEach(() => {
  cleanup()
  apiMock.getUSStockSymbols.mockReset()
  apiMock.getUSStockScan.mockReset()
})

function Harness({ onChange }: { onChange: (c: StockConfig) => void }) {
  const [cfg, setCfg] = useState<StockConfig>(defaultStockConfig)
  return (
    <StockConfigEditor
      config={cfg}
      language="zh"
      onChange={(c) => {
        setCfg(c)
        onChange(c)
      }}
    />
  )
}

test('defaults: paper on, regular session on, no symbols, swing preset', () => {
  expect(defaultStockConfig.symbols).toEqual([])
  expect(defaultStockConfig.paper_trading).toBe(true)
  expect(defaultStockConfig.sessions.regular).toBe(true)
  expect(defaultStockConfig.preset).toBe('swing')
})

test('searchable multi-select shows "AAPL · AAPLBUSDT" and selects symbols', async () => {
  apiMock.getUSStockSymbols.mockResolvedValue([
    { symbol: 'AAPLBUSDT', underlying: 'AAPL', base_asset: 'AAPLB' },
    { symbol: 'NVDABUSDT', underlying: 'NVDA', base_asset: 'NVDAB' },
  ])
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)
  expect(await screen.findByText('AAPL · AAPLBUSDT')).toBeInTheDocument()

  fireEvent.change(screen.getByTestId('stock-symbol-search'), {
    target: { value: 'nvd' },
  })
  expect(screen.queryByText('AAPL · AAPLBUSDT')).not.toBeInTheDocument()
  fireEvent.click(screen.getByText('NVDA · NVDABUSDT'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ symbols: ['NVDABUSDT'] })
  )
})

test('falls back to free-text comma-separated input when the endpoint fails', async () => {
  apiMock.getUSStockSymbols.mockRejectedValue(new Error('404'))
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)
  const input = await screen.findByTestId('stock-symbol-manual')
  fireEvent.change(input, {
    target: { value: 'aaplbusdt, NVDABUSDT aaplbusdt' },
  })
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ symbols: ['AAPLBUSDT', 'NVDABUSDT'] })
  )
})

test('sessions, preset and paper toggle update config; live mode warns', async () => {
  apiMock.getUSStockSymbols.mockResolvedValue([])
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)

  fireEvent.click(screen.getByTestId('stock-session-pre'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({
      sessions: { regular: true, pre_market: true, after_hours: false },
    })
  )
  fireEvent.click(screen.getByTestId('stock-preset-position'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ preset: 'position' })
  )
  // position preset changes the stop ATR placeholder to 2 / 4
  expect(screen.getByTestId('stock-stop_atr_min')).toHaveAttribute(
    'placeholder',
    expect.stringContaining('2')
  )

  expect(screen.queryByTestId('stock-live-warning')).not.toBeInTheDocument()
  fireEvent.click(screen.getByTestId('stock-paper-trading'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ paper_trading: false })
  )
  expect(screen.getByTestId('stock-live-warning')).toBeInTheDocument()
})

test('numeric field: empty means 0 (default), value is stored', async () => {
  apiMock.getUSStockSymbols.mockResolvedValue([])
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)
  const field = screen.getByTestId('stock-max_positions')
  expect(field).toHaveAttribute('placeholder', expect.stringContaining('5'))
  fireEvent.change(field, { target: { value: '3' } })
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ max_positions: 3 })
  )
  fireEvent.change(field, { target: { value: '' } })
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ max_positions: 0 })
  )
})

test('no 20-symbol cap: a 25th symbol can still be selected', async () => {
  const list = Array.from({ length: 25 }, (_, i) => ({
    symbol: `S${i}BUSDT`,
    underlying: `S${i}`,
    base_asset: `S${i}B`,
  }))
  apiMock.getUSStockSymbols.mockResolvedValue(list)
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)
  await screen.findByText('S0 · S0BUSDT')
  for (const s of list)
    fireEvent.click(screen.getByText(`${s.underlying} · ${s.symbol}`))
  expect(onChange.mock.lastCall?.[0].symbols).toHaveLength(25)
})

test('scan mode: top N, exclude, preview marks fed rows', async () => {
  apiMock.getUSStockSymbols.mockResolvedValue([])
  const cycle = (phase: string, long: boolean, score: number) => ({
    phase,
    long_setup: long,
    score,
    rs63_pct: 5,
    vol_ratio: 1.5,
  })
  apiMock.getUSStockScan.mockResolvedValue({
    scanning: false,
    generated_at: '2026-10-14T13:30:00Z',
    results: [
      {
        symbol: 'NVDABUSDT',
        underlying: 'NVDA',
        source: 'yahoo',
        cycle: cycle('wedge_pop', true, 80),
      },
      {
        symbol: 'AAPLBUSDT',
        underlying: 'AAPL',
        source: 'yahoo',
        cycle: cycle('ema_crossback', true, 70),
      },
      {
        symbol: 'TSLABUSDT',
        underlying: 'TSLA',
        source: 'yahoo',
        cycle: cycle('base_n_break', true, 60),
      },
      {
        symbol: 'INTCBUSDT',
        underlying: 'INTC',
        source: 'yahoo',
        cycle: cycle('wedge_drop', false, 0),
      },
    ],
    skipped: {},
  })
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)

  fireEvent.click(screen.getByTestId('stock-source-scan'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ symbol_source: 'scan' })
  )
  // empty pin list is fine in scan mode
  expect(screen.queryByText(/至少/)).not.toBeInTheDocument()

  fireEvent.change(screen.getByTestId('stock-scan-top-n'), {
    target: { value: '2' },
  })
  fireEvent.change(screen.getByTestId('stock-scan-exclude-input'), {
    target: { value: 'nvdabusdt' },
  })
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ scan_top_n: 2, scan_exclude: ['NVDABUSDT'] })
  )

  const preview = await screen.findByTestId('stock-scan-preview')
  await screen.findByText('楔形突破')
  const rows = preview.querySelectorAll('tbody tr')
  const fed = Array.from(rows).map((r) => r.textContent?.includes('★'))
  // NVDA excluded → AAPL and TSLA fill the top 2; INTC is not a long setup
  expect(fed).toEqual([false, true, true, false])
  expect(preview).toHaveTextContent('已排除')
})

test('hybrid mode: manual symbols always fed and do not consume top-N slots', async () => {
  apiMock.getUSStockSymbols.mockResolvedValue([
    { symbol: 'AAPLBUSDT', underlying: 'AAPL', base_asset: 'AAPLB' },
  ])
  const cycle = (phase: string, long: boolean, score: number) => ({
    phase,
    long_setup: long,
    score,
  })
  apiMock.getUSStockScan.mockResolvedValue({
    scanning: false,
    generated_at: '2026-10-14T13:30:00Z',
    results: [
      {
        symbol: 'AAPLBUSDT',
        underlying: 'AAPL',
        source: 'yahoo',
        cycle: cycle('wedge_pop', true, 90),
      },
      {
        symbol: 'NVDABUSDT',
        underlying: 'NVDA',
        source: 'yahoo',
        cycle: cycle('ema_crossback', true, 80),
      },
      {
        symbol: 'TSLABUSDT',
        underlying: 'TSLA',
        source: 'yahoo',
        cycle: cycle('base_n_break', true, 70),
      },
    ],
    skipped: {},
  })
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)
  await screen.findByText('AAPL · AAPLBUSDT')
  fireEvent.click(screen.getByText('AAPL · AAPLBUSDT'))
  fireEvent.click(screen.getByTestId('stock-source-hybrid'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ symbol_source: 'hybrid', symbols: ['AAPLBUSDT'] })
  )
  fireEvent.change(screen.getByTestId('stock-scan-top-n'), {
    target: { value: '1' },
  })
  // manual picker stays visible in hybrid
  expect(screen.getByTestId('stock-symbol-search')).toBeInTheDocument()
  const preview = await screen.findByTestId('stock-scan-preview')
  await screen.findByText('楔形突破')
  const fed = Array.from(preview.querySelectorAll('tbody tr')).map((r) =>
    r.textContent?.includes('★')
  )
  // AAPL is manual (fed, no slot); top 1 slot goes to NVDA; TSLA not fed
  expect(fed).toEqual([true, true, false])
  expect(preview.querySelector('tbody tr')?.textContent).toContain('手动')
})

test('scan mode hides the manual picker and notes an ignored saved list', async () => {
  apiMock.getUSStockSymbols.mockResolvedValue([
    { symbol: 'AAPLBUSDT', underlying: 'AAPL', base_asset: 'AAPLB' },
  ])
  apiMock.getUSStockScan.mockResolvedValue({
    scanning: true,
    generated_at: null,
    results: [],
    skipped: {},
  })
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)
  await screen.findByText('AAPL · AAPLBUSDT')
  fireEvent.click(screen.getByText('AAPL · AAPLBUSDT'))
  fireEvent.click(screen.getByTestId('stock-source-scan'))
  expect(screen.queryByTestId('stock-symbol-search')).not.toBeInTheDocument()
  expect(screen.getByText(/自动扫描模式不使用手动列表/)).toBeInTheDocument()
  // the saved list is not cleared
  expect(onChange.mock.lastCall?.[0].symbols).toEqual(['AAPLBUSDT'])
})

test('momentum ranking: only above-SMA200 names, ordered by 12-1 momentum', async () => {
  apiMock.getUSStockSymbols.mockResolvedValue([])
  const row = (symbol: string, mom: number, above: boolean, long = false) => ({
    symbol,
    underlying: symbol.replace('BUSDT', ''),
    source: 'yahoo',
    cycle: {
      phase: long ? 'wedge_pop' : 'uptrend',
      long_setup: long,
      score: long ? 80 : 0,
    },
    momentum: { valid: true, mom12_1_pct: mom, above_sma200: above },
  })
  apiMock.getUSStockScan.mockResolvedValue({
    scanning: false,
    generated_at: '2026-10-15T13:30:00Z',
    results: [
      row('AAABUSDT', 5, true, true), // Kell top, weak momentum
      row('BBBBUSDT', 90, false), // highest momentum but below SMA200
      row('CCCBUSDT', 60, true),
      row('DDDBUSDT', 40, true),
      row('EEEBUSDT', 30, true),
    ],
    skipped: {},
  })
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)
  fireEvent.click(screen.getByTestId('stock-source-scan'))
  fireEvent.click(screen.getByTestId('stock-rank-momentum'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ scan_rank: 'momentum' })
  )
  fireEvent.change(screen.getByTestId('stock-scan-top-n'), {
    target: { value: '2' },
  })
  fireEvent.change(screen.getByTestId('stock-scan-exclude-input'), {
    target: { value: 'CCCBUSDT' },
  })
  const preview = await screen.findByTestId('stock-scan-preview')
  await screen.findByText(/\+90\.0%/)
  const rows = Array.from(preview.querySelectorAll('tbody tr')).map((r) => ({
    sym: r.textContent?.match(/[A-E]{3}BUSDT/)?.[0],
    fed: r.textContent?.includes('★'),
  }))
  // eligible rows first by momentum (CCC excluded keeps its place but no ★),
  // then the below-SMA200 row; top 2 slots → DDD, EEE
  expect(rows.map((r) => r.sym).slice(0, 4)).toEqual([
    'CCCBUSDT',
    'DDDBUSDT',
    'EEEBUSDT',
    'AAABUSDT',
  ])
  expect(rows.filter((r) => r.fed).map((r) => r.sym)).toEqual([
    'DDDBUSDT',
    'EEEBUSDT',
  ])
  // back to Kell: scan_rank omitted again
  fireEvent.click(screen.getByTestId('stock-rank-kell'))
  expect(onChange.mock.lastCall?.[0].scan_rank).toBeUndefined()
})

test('regime scaler: basket_vol reveals inputs, trim defaults on, off hides them', () => {
  apiMock.getUSStockSymbols.mockResolvedValue([])
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)

  // hidden while the scaler is off (the default)
  expect(
    screen.queryByTestId('stock-regime-target-vol')
  ).not.toBeInTheDocument()
  expect(screen.queryByTestId('stock-regime-floor')).not.toBeInTheDocument()
  expect(screen.queryByTestId('stock-regime-trim')).not.toBeInTheDocument()

  fireEvent.click(screen.getByTestId('stock-regime-basket_vol'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ regime_scaler: 'basket_vol' })
  )

  const target = screen.getByTestId('stock-regime-target-vol')
  expect(target).toHaveAttribute('placeholder', '40')
  const floor = screen.getByTestId('stock-regime-floor')
  expect(floor).toHaveAttribute('placeholder', '0.25')
  // trim defaults to on when regime_trim is unset
  expect(screen.getByTestId('stock-regime-trim')).toBeChecked()

  fireEvent.change(target, { target: { value: '30' } })
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ regime_target_vol_pct: 30 })
  )
  // clearing stores 0 = backend default (PUT bodies merge, so omitting would keep the old value)
  fireEvent.change(target, { target: { value: '' } })
  expect(onChange.mock.lastCall?.[0].regime_target_vol_pct).toBe(0)

  fireEvent.change(floor, { target: { value: '0.5' } })
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ regime_floor: 0.5 })
  )

  fireEvent.click(screen.getByTestId('stock-regime-trim'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ regime_trim: false })
  )

  // back to off: scaler stored explicitly and inputs disappear again
  fireEvent.click(screen.getByTestId('stock-regime-off'))
  expect(onChange).toHaveBeenLastCalledWith(
    expect.objectContaining({ regime_scaler: 'off' })
  )
  expect(screen.queryByTestId('stock-regime-trim')).not.toBeInTheDocument()
})
