import { Fragment, useMemo, useState } from 'react'
import { Badge, Card, CardHeader, EmptyState, Input, Segmented } from '../ui'
import type { FactorEntry } from '../../types/factorMining'
import FactorDetail from './FactorDetail'
import { L, fmtIC, fmtPct, fmtT, fmtText, isNum } from './format'

const PAGE_SIZE = 25

type StatusFilter = 'all' | 'accepted' | 'candidate' | 'redundant' | 'rejected'

type SortKey =
  | 'train_ic'
  | 'train_icir'
  | 'train_t'
  | 'val_ic'
  | 'val_t'
  | 'ls_train'
  | 'ls_val'
  | 'turnover'

const STATUS_RANK: Record<string, number> = {
  accepted: 0,
  candidate: 1,
  redundant: 2,
  rejected: 3,
}

const STATUS_VARIANT: Record<string, 'up' | 'warn' | 'neutral' | 'down'> = {
  accepted: 'up',
  candidate: 'warn',
  redundant: 'neutral',
  rejected: 'down',
}

const SOURCE_LABEL: Record<string, { zh: string; en: string }> = {
  seed: { zh: '种子', en: 'Seed' },
  mutation: { zh: '变异', en: 'Mutation' },
  llm: { zh: 'LLM', en: 'LLM' },
}

const SORT_GETTERS: Record<SortKey, (f: FactorEntry) => number | null> = {
  train_ic: (f) => (isNum(f.train?.ic) ? f.train!.ic! : null),
  train_icir: (f) => (isNum(f.train?.icir) ? f.train!.icir! : null),
  train_t: (f) => (isNum(f.train?.t) ? f.train!.t! : null),
  val_ic: (f) => (isNum(f.val?.ic) ? f.val!.ic! : null),
  val_t: (f) => (isNum(f.val?.t) ? f.val!.t! : null),
  ls_train: (f) => (isNum(f.train?.ls_ret) ? f.train!.ls_ret! : null),
  ls_val: (f) => (isNum(f.val?.ls_ret) ? f.val!.ls_ret! : null),
  turnover: (f) => (isNum(f.train?.turnover) ? f.train!.turnover! : null),
}

const TH_CLS =
  'h-8 bg-surface-2 px-3 py-0 text-left text-xs font-medium text-fg-3 whitespace-nowrap'
const TD_CLS = 'px-3 py-1.5 whitespace-nowrap'

function statusLabel(s: string, language: string) {
  const m: Record<string, [string, string]> = {
    accepted: ['已接受', 'accepted'],
    candidate: ['候选', 'candidate'],
    redundant: ['冗余', 'redundant'],
    rejected: ['已拒绝', 'rejected'],
  }
  const p = m[s]
  return p ? L(p[0], p[1], language) : s
}

function defaultCompare(a: FactorEntry, b: FactorEntry) {
  const ra = STATUS_RANK[a.status ?? ''] ?? 9
  const rb = STATUS_RANK[b.status ?? ''] ?? 9
  if (ra !== rb) return ra - rb
  const ta = a.train?.t
  const tb = b.train?.t
  return (isNum(tb) ? tb : -Infinity) - (isNum(ta) ? ta : -Infinity)
}

export default function FactorTable({
  factors,
  language,
}: {
  factors: FactorEntry[]
  language: string
}) {
  const [filter, setFilter] = useState<StatusFilter>('all')
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean } | null>(null)
  const [page, setPage] = useState(0)
  const [expanded, setExpanded] = useState<string | null>(null)

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase()
    let list = factors.filter((f) => filter === 'all' || f.status === filter)
    if (q) {
      list = list.filter(
        (f) =>
          (f.expr ?? '').toLowerCase().includes(q) ||
          f.id.toLowerCase().includes(q)
      )
    }
    const sorted = [...list]
    if (sort) {
      const get = SORT_GETTERS[sort.key]
      sorted.sort((a, b) => {
        const va = get(a)
        const vb = get(b)
        // 空值始终排最后
        if (va === null && vb === null) return 0
        if (va === null) return 1
        if (vb === null) return -1
        return sort.desc ? vb - va : va - vb
      })
    } else {
      sorted.sort(defaultCompare)
    }
    return sorted
  }, [factors, filter, query, sort])

  const pageCount = Math.max(1, Math.ceil(rows.length / PAGE_SIZE))
  const curPage = Math.min(page, pageCount - 1)
  const pageRows = rows.slice(curPage * PAGE_SIZE, (curPage + 1) * PAGE_SIZE)

  const toggleSort = (key: SortKey) => {
    setPage(0)
    setSort((s) => {
      if (!s || s.key !== key) return { key, desc: true }
      if (s.desc) return { key, desc: false }
      return null
    })
  }

  const sortHeader = (key: SortKey, label: string) => {
    const active = sort?.key === key
    return (
      <th
        className={TH_CLS}
        aria-sort={
          active ? (sort!.desc ? 'descending' : 'ascending') : undefined
        }
      >
        <button
          type="button"
          onClick={() => toggleSort(key)}
          className="inline-flex items-center gap-0.5 hover:text-fg"
        >
          {label}
          <span className="w-2 text-[10px]">
            {active ? (sort!.desc ? '▼' : '▲') : ''}
          </span>
        </button>
      </th>
    )
  }

  const colCount = 14

  return (
    <Card className="mb-4">
      <CardHeader
        title={L('因子榜单', 'Factor leaderboard', language)}
        subtitle={L(
          `${rows.length} 个因子；点击行查看详情，点击数值列表头排序`,
          `${rows.length} factors; click a row for details, click numeric headers to sort`,
          language
        )}
      />
      <div className="flex flex-wrap items-center gap-2 border-b border-line p-3">
        <Segmented<StatusFilter>
          size="sm"
          value={filter}
          onChange={(v) => {
            setFilter(v)
            setPage(0)
          }}
          items={[
            { key: 'all', label: L('全部', 'All', language) },
            { key: 'accepted', label: 'accepted' },
            { key: 'candidate', label: 'candidate' },
            { key: 'redundant', label: 'redundant' },
            { key: 'rejected', label: 'rejected' },
          ]}
        />
        <Input
          className="w-full sm:w-64"
          value={query}
          placeholder={L(
            '搜索表达式 / 因子 id',
            'Search expression / id',
            language
          )}
          aria-label={L('搜索表达式', 'Search expression', language)}
          onChange={(e) => {
            setQuery(e.target.value)
            setPage(0)
          }}
        />
      </div>

      {rows.length === 0 ? (
        <EmptyState
          title={L('暂无符合条件的因子', 'No matching factors', language)}
        />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-[12.5px]">
            <thead>
              <tr>
                <th className={TH_CLS}>{L('状态', 'Status', language)}</th>
                <th className={TH_CLS}>{L('因子', 'Factor', language)}</th>
                <th className={TH_CLS}>
                  {L('表达式', 'Expression', language)}
                </th>
                <th className={TH_CLS}>{L('类别', 'Family', language)}</th>
                <th className={TH_CLS}>{L('来源', 'Source', language)}</th>
                <th className={TH_CLS}>{L('方向', 'Dir', language)}</th>
                {sortHeader('train_ic', 'train IC')}
                {sortHeader('train_icir', 'train ICIR')}
                {sortHeader('train_t', 'train t')}
                {sortHeader('val_ic', 'val IC')}
                {sortHeader('val_t', 'val t')}
                {sortHeader(
                  'ls_train',
                  L('多空年化 train', 'L/S ann. train', language)
                )}
                {sortHeader(
                  'ls_val',
                  L('多空年化 val', 'L/S ann. val', language)
                )}
                {sortHeader('turnover', L('换手', 'Turnover', language))}
              </tr>
            </thead>
            <tbody>
              {pageRows.map((f) => {
                const open = expanded === f.id
                const st = f.status ?? ''
                const src = SOURCE_LABEL[f.source ?? '']
                return (
                  <Fragment key={f.id}>
                    <tr
                      data-testid={`factor-row-${f.id}`}
                      onClick={() => setExpanded(open ? null : f.id)}
                      className="cursor-pointer border-b border-line hover:bg-surface-hover"
                      aria-expanded={open}
                    >
                      <td className={TD_CLS}>
                        <Badge variant={STATUS_VARIANT[st] ?? 'neutral'}>
                          {statusLabel(st, language)}
                        </Badge>
                      </td>
                      <td className={`${TD_CLS} font-mono`}>{f.id}</td>
                      <td
                        className={`${TD_CLS} max-w-[280px] truncate font-mono`}
                        title={f.expr ?? undefined}
                      >
                        {fmtText(f.expr)}
                      </td>
                      <td className={TD_CLS}>{fmtText(f.family)}</td>
                      <td className={TD_CLS}>
                        {src ? L(src.zh, src.en, language) : fmtText(f.source)}
                      </td>
                      <td className={`${TD_CLS} num`}>
                        {f.direction === 1
                          ? '+1'
                          : f.direction === -1
                            ? '-1'
                            : '—'}
                      </td>
                      <td className={`${TD_CLS} num`}>{fmtIC(f.train?.ic)}</td>
                      <td className={`${TD_CLS} num`}>{fmtT(f.train?.icir)}</td>
                      <td className={`${TD_CLS} num`}>{fmtT(f.train?.t)}</td>
                      <td className={`${TD_CLS} num`}>{fmtIC(f.val?.ic)}</td>
                      <td className={`${TD_CLS} num`}>{fmtT(f.val?.t)}</td>
                      <td className={`${TD_CLS} num`}>
                        {fmtPct(f.train?.ls_ret)}
                      </td>
                      <td className={`${TD_CLS} num`}>
                        {fmtPct(f.val?.ls_ret)}
                      </td>
                      <td className={`${TD_CLS} num`}>
                        {fmtT(f.train?.turnover)}
                      </td>
                    </tr>
                    {open && (
                      <tr className="border-b border-line bg-surface">
                        <td colSpan={colCount}>
                          <FactorDetail factor={f} language={language} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {rows.length > PAGE_SIZE && (
        <div className="flex items-center justify-between gap-2 border-t border-line p-3 text-xs text-fg-3">
          <button
            type="button"
            className="rounded border border-line px-2 py-1 hover:bg-surface-hover disabled:opacity-40"
            disabled={curPage === 0}
            onClick={() => setPage(curPage - 1)}
          >
            {L('上一页', 'Prev', language)}
          </button>
          <span data-testid="factor-page-info">
            {curPage + 1} / {pageCount}
          </span>
          <button
            type="button"
            className="rounded border border-line px-2 py-1 hover:bg-surface-hover disabled:opacity-40"
            disabled={curPage >= pageCount - 1}
            onClick={() => setPage(curPage + 1)}
          >
            {L('下一页', 'Next', language)}
          </button>
        </div>
      )}
    </Card>
  )
}
