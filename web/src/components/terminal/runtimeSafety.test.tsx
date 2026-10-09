import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { SWRConfig } from 'swr'
import { KlineChart } from './KlineChart'
import { LiquidationMap } from './LiquidationMap'
import { ExecutionLog } from './ExecutionLog'
import type { DecisionRecord } from '../../types'

const mocks = vi.hoisted(() => ({ klines: vi.fn(), heatmap: vi.fn() }))
vi.mock('../../lib/api', () => ({ api: { getKlines: mocks.klines, getVergexCostLiquidationHeatmap: mocks.heatmap } }))
vi.mock('./Candles', () => ({ Candles: ({ data }: { data: {close:number}[] }) => <div data-testid="candles">{data.map((d) => d.close).join(',')}</div> }))
class Socket {
  static all: Socket[] = []
  onopen: null | (() => void) = null
  onmessage: null | ((event: { data: string }) => void) = null
  onclose: null | (() => void) = null
  onerror: null | (() => void) = null
  send = vi.fn()
  close = vi.fn()
  constructor() { Socket.all.push(this) }
}
beforeEach(() => { vi.clearAllMocks(); Socket.all = []; vi.stubGlobal('WebSocket', Socket) })
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
const bar = { openTime: 60_000, closeTime: 120_000, open:120, high:121, low:119, close:120, volume:10 }

it('keeps the HIP-3 namespace and never merges previous-market candles on failure', async () => {
  mocks.klines.mockImplementation((symbol: string) => symbol === 'SOL' ? Promise.resolve([bar]) : Promise.reject(new Error('unavailable')))
  const cache = new Map()
  const view = (symbol: string) => <SWRConfig value={{provider: () => cache}}><KlineChart symbol={symbol} /></SWRConfig>
  const ui = render(view('SOL'))
  await waitFor(() => expect(screen.getByTestId('candles')).toHaveTextContent('120'))
  const previousSocket = Socket.all[0]
  ui.rerender(view('xyz:SKHY'))
  await waitFor(() => expect(screen.getByText(/Candle history unavailable/)).toBeInTheDocument())
  expect(mocks.klines).toHaveBeenLastCalledWith('xyz:SKHY', '1m', 'hyperliquid', 90, true)
  act(() => previousSocket.onmessage?.({data:JSON.stringify({channel:'candle',data:{s:'SOL',i:'1m',t:120_000,T:180_000,o:'120',h:'120',l:'120',c:'120',v:'1'}})}))
  expect(screen.queryByTestId('candles')).not.toBeInTheDocument()
  expect(screen.queryByText('● live')).not.toBeInTheDocument()
  expect(previousSocket.close).toHaveBeenCalled()
})

it('does not try a second paid market when a valid heatmap response is empty', async () => {
  mocks.heatmap.mockResolvedValue({data:{bins:[]}})
  render(<SWRConfig value={{provider: () => new Map()}}><LiquidationMap symbol="SOL" marketType="core_perp" /></SWRConfig>)
  await screen.findByText('No heatmap data in the selected range.')
  expect(mocks.heatmap).toHaveBeenCalledOnce()
  expect(mocks.heatmap).toHaveBeenCalledWith({marketType:'core_perp',symbol:'SOL',chain:'mainnet',liqBand:'15'})
})

it('shows a real request failure without claiming all crypto heatmaps are unsupported', async () => {
  mocks.heatmap.mockRejectedValue(new Error('Snapshot being generated (503)'))
  render(<SWRConfig value={{provider: () => new Map()}}><LiquidationMap symbol="SOL" marketType="core_perp" /></SWRConfig>)
  await screen.findByText(/No automatic paid retry/)
  expect(screen.queryByText(/crypto.*have none/)).not.toBeInTheDocument()
  expect(screen.queryByText('● live')).not.toBeInTheDocument()
})

it('marks legacy success=true cycles as faults when their orders failed and shows the date', () => {
  const record = {cycle_number:2581,timestamp:'2026-08-15T12:00:00Z',success:true,decisions:[{action:'open_long',symbol:'SOL',success:false,timestamp:'2026-08-15T12:00:00Z',error:'denied'}]} as DecisionRecord
  render(<ExecutionLog decisions={[record]} />)
  expect(screen.getByText('FAULT · 1/1 trades failed/blocked')).toBeInTheDocument()
  expect(screen.getAllByText(/2026-08-15/).length).toBeGreaterThan(0)
})
