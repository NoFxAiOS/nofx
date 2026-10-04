import { StrictMode, useEffect } from 'react'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { SWRConfig, useSWRConfig } from 'swr'
import type { Cache } from 'swr'
import type {
  VergexWinrateHoldersRequest,
  VergexWinrateMapResponse,
} from '../../lib/api/data'
import { WinrateMatrix } from './WinrateMatrix'

const mocks = vi.hoisted(() => ({ map: vi.fn(), holders: vi.fn() }))
vi.mock('../../lib/api', () => ({
  api: {
    getVergexHolderWinrateMap: mocks.map,
    getVergexHolderWinrateHolders: mocks.holders,
  },
}))
const snapshotA = 'AAAAAAAAAAAAAAAAAAAAAAAAAA'
const snapshotB = 'BBBBBBBBBBBBBBBBBBBBBBBBBB'
const key = ['winrate-map', 'hip3_perp', 'NVDA']
function matrix(snapshotId = snapshotA): VergexWinrateMapResponse {
  return {
    data: {
      snapshotId,
      markPrice: '100',
      coverage: 'complete',
      asOf: new Date().toISOString(),
      positionsAsOf: new Date().toISOString(),
      staleHistoryCount: 0,
      minRoundTrips: 3,
      viewport: { winMin: 0, winMax: 100, costMin: 92, costMax: 108 },
      winBins: 20,
      costBins: 16,
      cells: [
        {
          row: 0,
          column: 9,
          long: { count: 2, notional: '1000' },
          short: { count: 0, notional: '0' },
        },
      ],
    },
  }
}
function page(q: VergexWinrateHoldersRequest, address = '0x1234') {
  return {
    data: {
      snapshotId: q.snapshotId,
      total: 40,
      nextOffset: (q.offset ?? 0) === 0 ? 20 : null,
      items: [
        {
          address,
          side: 'long',
          size: '1',
          entryPrice: '100',
          notional: '100',
          costRatio: '100',
          winRate: '1',
          roundTrips: 2,
        },
      ],
    },
  }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((r) => {
    resolve = r
  })
  return { promise, resolve }
}
let mutate: ReturnType<typeof useSWRConfig>['mutate']
function Access() {
  const config = useSWRConfig()
  useEffect(() => {
    mutate = config.mutate
  }, [config.mutate])
  return null
}
function view(symbol: string, cache: Cache = new Map(), demo = false) {
  return (
    <StrictMode>
      <SWRConfig value={{ provider: () => cache, dedupingInterval: 0 }}>
        <Access />
        <WinrateMatrix symbol={symbol} demo={demo} />
      </SWRConfig>
    </StrictMode>
  )
}
function cell() {
  return screen
    .queryAllByRole('button')
    .find((b) => b.textContent?.startsWith('$1K'))!
}
async function selectCell() {
  await screen.findByText('● current snapshot')
  fireEvent.click(cell())
  await screen.findByText('0x1234')
}
beforeEach(() => {
  mocks.map.mockReset().mockResolvedValue(matrix())
  mocks.holders
    .mockReset()
    .mockImplementation((q: VergexWinrateHoldersRequest) =>
      Promise.resolve(page(q))
    )
})
afterEach(cleanup)

it('clears selection and old matrix on a market change, without paying for mismatched data', async () => {
  const next = deferred<VergexWinrateMapResponse>()
  mocks.map.mockImplementation(({ symbol }) =>
    symbol === 'NVDA' ? Promise.resolve(matrix()) : next.promise
  )
  const cache = new Map()
  const ui = render(view('NVDA', cache))
  await selectCell()
  ui.rerender(view('SNDK', cache))
  expect(screen.queryByText('0x1234')).not.toBeInTheDocument()
  expect(screen.queryByText(/Pinned data version/)).not.toBeInTheDocument()
  expect(cell()).toBeUndefined()
  await act(async () => {
    next.resolve(matrix(snapshotB))
  })
  await screen.findByText('● current snapshot')
  expect(mocks.holders).toHaveBeenCalledTimes(1)
  fireEvent.click(cell())
  await screen.findByText('0x1234')
  expect(mocks.holders).toHaveBeenCalledTimes(2)
  expect(mocks.holders.mock.calls[1][0]).toMatchObject({
    symbol: 'SNDK',
    snapshotId: snapshotB,
    offset: 0,
  })
})

it('pins snapshot, viewport and filters for pagination across background matrix refreshes', async () => {
  render(view('NVDA'))
  await selectCell()
  expect(mocks.holders.mock.calls[0][0]).toMatchObject({
    snapshotId: snapshotA,
    symbol: 'NVDA',
    minRoundTrips: 3,
    costMin: 92,
    costMax: 108,
    row: 0,
    rowEnd: 1,
    column: 9,
    columnEnd: 9,
  })
  fireEvent.click(screen.getByRole('button', { name: 'next →' }))
  await waitFor(() => expect(mocks.holders).toHaveBeenCalledTimes(2))
  const updated = matrix(snapshotB)
  updated.data!.markPrice = '200'
  updated.data!.minRoundTrips = 5
  updated.data!.viewport = { winMin: 0, winMax: 100, costMin: 60, costMax: 140 }
  await act(async () => {
    await mutate(key, updated, { revalidate: false })
  })
  expect(mocks.holders).toHaveBeenCalledTimes(2)
  expect(
    screen.getByText(`Pinned data version: ${snapshotA}`)
  ).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '← prev' }))
  await waitFor(() => expect(mocks.holders).toHaveBeenCalledTimes(3))
  expect(mocks.holders.mock.calls[2][0]).toMatchObject({
    snapshotId: snapshotA,
    offset: 0,
    minRoundTrips: 3,
    costMin: 92,
    costMax: 108,
  })
  expect(mocks.holders.mock.calls[1][0]).toMatchObject({
    snapshotId: snapshotA,
    offset: 20,
  })
})

it('ignores late responses after closing and does not double charge on repeated clicks', async () => {
  const pending = deferred<ReturnType<typeof page>>()
  mocks.holders.mockImplementationOnce(() => pending.promise)
  render(view('NVDA'))
  await screen.findByText('● current snapshot')
  fireEvent.click(cell())
  fireEvent.click(cell())
  expect(mocks.holders).toHaveBeenCalledTimes(1)
  fireEvent.click(screen.getByRole('button', { name: 'close' }))
  await act(async () => {
    pending.resolve(page(mocks.holders.mock.calls[0][0], 'old-result'))
  })
  expect(screen.queryByText('old-result')).not.toBeInTheDocument()
  expect(screen.queryByText(/Pinned data version/)).not.toBeInTheDocument()
  fireEvent.click(cell())
  await screen.findByText('0x1234')
  expect(mocks.holders).toHaveBeenCalledTimes(2)
})

it('does not reuse a late response after changing stock', async () => {
  const pending = deferred<ReturnType<typeof page>>()
  mocks.holders.mockImplementationOnce(() => pending.promise)
  const cache = new Map()
  const ui = render(view('NVDA', cache))
  await screen.findByText('● current snapshot')
  fireEvent.click(cell())
  ui.rerender(view('SNDK', cache))
  await act(async () => {
    pending.resolve(page(mocks.holders.mock.calls[0][0], 'old-result'))
  })
  expect(screen.queryByText('old-result')).not.toBeInTheDocument()
  expect(mocks.holders).toHaveBeenCalledTimes(1)
})

it('shows expired snapshot errors without automatic retry or snapshot replacement', async () => {
  mocks.holders.mockRejectedValueOnce(new Error('snapshot expired'))
  render(view('NVDA'))
  await screen.findByText('● current snapshot')
  fireEvent.click(cell())
  await screen.findByText(/Drilldown failed: snapshot expired/)
  await act(async () => {
    await mutate(key, matrix(snapshotB), { revalidate: false })
  })
  expect(mocks.holders).toHaveBeenCalledTimes(1)
  fireEvent.click(
    screen.getByRole('button', { name: 'Retry this page ($0.002)' })
  )
  await screen.findByText('0x1234')
  expect(mocks.holders.mock.calls[1][0]).toMatchObject({
    snapshotId: snapshotA,
    offset: 0,
  })
})

it('rejects a holders response from another snapshot and invalid pagination', async () => {
  mocks.holders.mockImplementationOnce((q) =>
    Promise.resolve(page({ ...q, snapshotId: snapshotB }))
  )
  render(view('NVDA'))
  await screen.findByText('● current snapshot')
  fireEvent.click(cell())
  await screen.findByText(/Data version mismatch/)
  expect(screen.queryByText('0x1234')).not.toBeInTheDocument()
  mocks.holders.mockImplementationOnce((q) =>
    Promise.resolve({ data: { ...page(q).data, nextOffset: 0 } })
  )
  fireEvent.click(
    screen.getByRole('button', { name: 'Retry this page ($0.002)' })
  )
  await screen.findByText(/Invalid pagination response/)
  expect(mocks.holders).toHaveBeenCalledTimes(2)
})

it('labels partial stale data as unverified and exposes its quality', async () => {
  const partial = matrix()
  Object.assign(partial.data!, {
    coverage: 'partial',
    positionsAsOf: '2025-01-01T00:00:00Z',
    staleHistoryCount: 10,
  })
  mocks.map.mockResolvedValue(partial)
  render(view('NVDA'))
  await screen.findByText('○ cached / unverified')
  expect(screen.getByRole('status')).toHaveTextContent('coverage partial')
  expect(screen.getByRole('status')).toHaveTextContent(
    'positions older than 15 minutes'
  )
  expect(screen.getByRole('status')).toHaveTextContent('10 stale histories')
})

it('exposes refresh errors even when a previous matrix remains cached', async () => {
  render(view('NVDA'))
  await screen.findByText('● current snapshot')
  mocks.map.mockRejectedValue(new Error('upstream unavailable'))
  await act(async () => {
    try {
      await mutate(key)
    } catch {
      /* SWR preserves stale data */
    }
  })
  await screen.findByText('○ cached / unverified')
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Showing the previous snapshot, not live data'
  )
  expect(mocks.holders).not.toHaveBeenCalled()
})

it('keeps demo queries offline and stops pagination at the last page', () => {
  render(view('NVDA', new Map(), true))
  fireEvent.click(
    screen.getAllByRole('button').find((b) => !b.hasAttribute('disabled'))!
  )
  for (let i = 0; i < 6; i++)
    fireEvent.click(screen.getByRole('button', { name: 'next →' }))
  expect(screen.getByText('121–137 / 137')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'next →' })).toBeDisabled()
  expect(mocks.map).not.toHaveBeenCalled()
  expect(mocks.holders).not.toHaveBeenCalled()
})
