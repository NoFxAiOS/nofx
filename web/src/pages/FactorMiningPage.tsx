import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Badge,
  Card,
  CardHeader,
  EmptyState,
  Segmented,
  Stat,
} from '../components/ui'
import EnsembleCard from '../components/factorMining/EnsembleCard'
import FactorTable from '../components/factorMining/FactorTable'
import RunsTable from '../components/factorMining/RunsTable'
import {
  L,
  fmtIC,
  fmtInt,
  fmtNum,
  fmtText,
  isNum,
} from '../components/factorMining/format'
import { getAuthHeaders } from '../lib/api/helpers'
import type {
  FactorMarket,
  FactorMiningOverview,
  FactorMiningRun,
} from '../types/factorMining'

const POLL_MS = 30_000

async function fetchJSON<T>(url: string): Promise<T> {
  const res = await fetch(url, { headers: getAuthHeaders() })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return (await res.json()) as T
}

function methodNotes(language: string): string[] {
  return [
    L(
      '目标：截面去均值的 h 日收益；t 日收盘出信号，t+1 开盘入场，t+1+h 开盘出场（crypto 去全宇宙等权均值，美股去 SPY 同期收益）。',
      'Target: cross-sectionally demeaned h-day return; signal at close t, enter at open t+1, exit at open t+1+h (crypto: demeaned by the equal-weight universe mean; US: by SPY).',
      language
    ),
    L(
      '切分：时间轴 60/20/20 = train/val/holdout，段间做 purge。holdout 尚未开启——页面与 JSON 中没有 holdout 数据，所有指标都只是 train/val 的选择后结果。',
      'Split: 60/20/20 train/val/holdout with purging between segments. The holdout is NOT opened yet: every metric shown is a post-selection train/val number.',
      language
    ),
    L(
      '指标：日 RankIC（每 h 天取一次的非重叠序列算 ICIR 与 t）、Q5−Q1 多空年化（已扣费用与滑点）、换手；训练集 IC 为负则取反方向。',
      'Metrics: daily RankIC (ICIR and t use a non-overlapping every-h-days series), Q5−Q1 long-short annualised return net of fees, and turnover; a negative train IC flips the direction.',
      language
    ),
    L(
      '多重检验：同一 (市场, 周期) 下已试表达式数 K 越大，接受门槛 t_req = max(2, Φ⁻¹(1−0.025/K)) 越高；变异和 LLM 提案都会让 K 增长，门槛只升不降。',
      'Multiple testing: with K expressions tried per (market, horizon), the bar t_req = max(2, Φ⁻¹(1−0.025/K)) rises; mutation and LLM proposals both grow K, so the bar only goes up.',
      language
    ),
    L(
      '状态：accepted = train 达标且 val 同号、t≥1.65、净收益>0；candidate = train 达标但 val 未过；redundant = 与已接受因子相关性≥0.8；rejected = train 不达标。',
      'Status: accepted = passes train, same-sign val with t≥1.65 and positive net return; candidate = passes train only; redundant = correlation ≥0.8 with an accepted factor; rejected = fails train.',
      language
    ),
    L(
      'accepted ≠ 可交易：只代表研究层通过 train/val，未建模资金费率、冲击成本与容量，也不会自动影响任何策略。',
      'accepted ≠ tradable: it only means the factor cleared train/val in research. Funding, impact cost and capacity are not modelled, and nothing is wired into live strategies.',
      language
    ),
    L(
      '数据局限：日线级别；币圈只有 2–5 年样本；美股使用当前在售标的的调整后日线，存在幸存者偏差。',
      'Data limits: daily bars only; crypto has just 2–5 years of history; US uses adjusted bars of currently listed tickers, so survivorship bias applies.',
      language
    ),
  ]
}

export default function FactorMiningPage({ language }: { language: string }) {
  const [overview, setOverview] = useState<FactorMiningOverview | null>(null)
  const [runs, setRuns] = useState<FactorMiningRun[]>([])
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)
  const [market, setMarket] = useState<FactorMarket>('crypto')
  const [horizonSel, setHorizonSel] = useState<string | null>(null)
  const [noteOpen, setNoteOpen] = useState(false)
  const aliveRef = useRef(true)

  const load = useCallback(async () => {
    const [ov, rn] = await Promise.allSettled([
      fetchJSON<FactorMiningOverview | null>('/api/factor-mining/overview'),
      fetchJSON<FactorMiningRun[] | null>('/api/factor-mining/runs?limit=50'),
    ])
    if (!aliveRef.current) return
    if (ov.status === 'fulfilled') {
      setOverview(ov.value ?? null)
      setFailed(false)
    } else {
      setFailed(true)
    }
    if (rn.status === 'fulfilled') {
      const v = rn.value as
        | FactorMiningRun[]
        | { runs?: FactorMiningRun[] }
        | null
      const list = Array.isArray(v) ? v : (v?.runs ?? [])
      setRuns(Array.isArray(list) ? list : [])
    }
    setLoading(false)
  }, [])

  // 每 30 秒轮询；页面不可见时不请求，重新可见时立即刷新。
  useEffect(() => {
    aliveRef.current = true
    let timer: number | undefined
    const schedule = () => {
      timer = window.setTimeout(tick, POLL_MS)
    }
    const tick = async () => {
      if (typeof document !== 'undefined' && document.hidden) {
        schedule()
        return
      }
      await load()
      if (aliveRef.current) schedule()
    }
    const onVisible = () => {
      if (!document.hidden) {
        if (timer) window.clearTimeout(timer)
        void tick()
      }
    }
    document.addEventListener('visibilitychange', onVisible)
    void tick()
    return () => {
      aliveRef.current = false
      if (timer) window.clearTimeout(timer)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [load])

  const status = overview?.status ?? null
  const zoo = overview?.zoo?.[market] ?? null
  const ensemble = overview?.ensemble?.[market] ?? null
  const coverage = status?.markets?.[market] ?? null

  const horizonKeys = useMemo(
    () =>
      Object.keys(zoo?.horizons ?? {}).sort((a, b) => Number(a) - Number(b)),
    [zoo]
  )
  const horizon =
    horizonSel && horizonKeys.includes(horizonSel)
      ? horizonSel
      : (horizonKeys[0] ?? null)
  const hSummary = horizon ? (zoo?.horizons?.[horizon] ?? null) : null
  const hEnsemble = horizon ? (ensemble?.horizons?.[horizon] ?? null) : null

  const factors = useMemo(
    () =>
      (zoo?.factors ?? []).filter(
        (f) => f && f.id && horizon !== null && String(f.horizon) === horizon
      ),
    [zoo, horizon]
  )

  const dayUnit = L('天', 'd', language)
  const notes = methodNotes(language)

  return (
    <div className="mx-auto max-w-[1440px] px-4 py-6 sm:px-6">
      <div className="mb-4">
        <h1 className="text-xl font-semibold text-fg">
          {L('因子挖掘', 'Factor Mining', language)}
        </h1>
        <p className="mt-1 text-[13px] text-fg-3">
          {L(
            '每晚自动在种子库、遗传变异与 LLM 提案上挖掘日线因子，统一经冻结评估器与多重检验门槛筛选。',
            'Nightly mining of daily-bar factors from seeds, mutations and LLM proposals, all filtered by a frozen evaluator and a multiple-testing bar.',
            language
          )}
        </p>
      </div>

      <Card className="mb-4 flex flex-wrap items-center gap-x-4 gap-y-1 p-3 text-[13px]">
        {status === null ? (
          <span className="text-fg-3">
            {loading
              ? L('加载中…', 'Loading…', language)
              : failed
                ? L(
                    '接口请求失败，稍后自动重试',
                    'Request failed, will retry automatically',
                    language
                  )
                : L(
                    '尚未运行过，等待首次定时任务',
                    'Never run yet, waiting for the first scheduled job',
                    language
                  )}
          </span>
        ) : (
          <>
            {status.running ? (
              <Badge variant="info">
                {L('运行中', 'Running', language)}：
                {fmtText(status.current?.market)} ·{' '}
                {fmtText(status.current?.stage)}
              </Badge>
            ) : (
              <Badge variant="neutral">{L('空闲', 'Idle', language)}</Badge>
            )}
            <span className="text-fg-3">
              {L('更新于', 'Updated', language)} {fmtText(status.updated_at)}
            </span>
            <span className="text-fg-3">
              {L('计划', 'Schedule', language)}：{fmtText(status.schedule)}
            </span>
          </>
        )}
        {failed && status !== null && (
          <span className="text-down">
            {L('最近一次刷新失败', 'Last refresh failed', language)}
          </span>
        )}
      </Card>

      <div className="mb-4 flex flex-wrap items-center gap-3">
        <Segmented<FactorMarket>
          value={market}
          onChange={(m) => {
            setMarket(m)
            setHorizonSel(null)
          }}
          items={[
            { key: 'crypto', label: L('虚拟币', 'Crypto', language) },
            { key: 'us', label: L('美股', 'US stocks', language) },
          ]}
        />
        {horizonKeys.length > 0 && (
          <Segmented<string>
            value={horizon ?? ''}
            onChange={setHorizonSel}
            items={horizonKeys.map((h) => ({
              key: h,
              label: `${h}${dayUnit}`,
            }))}
          />
        )}
      </div>

      <div className="mb-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Card className="p-3">
          <Stat
            label={L('样本覆盖', 'Coverage', language)}
            value={
              isNum(coverage?.symbols)
                ? `${fmtInt(coverage?.symbols)}${L(' 个标的', ' symbols', language)}`
                : '—'
            }
            hint={
              coverage
                ? `${fmtText(coverage.start)}~${fmtText(coverage.end)} · ${fmtInt(coverage.bars)} bars`
                : undefined
            }
          />
        </Card>
        <Card className="p-3">
          <Stat
            label={L('已试表达式 K', 'Trials K', language)}
            value={fmtInt(hSummary?.trials)}
            hint={`t_req ${fmtNum(hSummary?.t_req, 2)}`}
          />
        </Card>
        <Card className="p-3">
          <Stat
            label="accepted / candidate"
            value={`${fmtInt(hSummary?.accepted)} / ${fmtInt(hSummary?.candidate)}`}
          />
        </Card>
        <Card className="p-3">
          <Stat
            label={L('组合 val IC', 'Ensemble val IC', language)}
            value={fmtIC(hEnsemble?.val?.ic)}
          />
        </Card>
      </div>

      <Card className="mb-4">
        <button
          type="button"
          aria-expanded={noteOpen}
          onClick={() => setNoteOpen((o) => !o)}
          className="flex w-full items-center justify-between gap-2 px-4 py-2 text-left text-sm font-semibold text-fg"
        >
          <span>
            {L(
              '方法与局限（先读这里）',
              'Method & limitations (read first)',
              language
            )}
          </span>
          <span className="text-xs text-fg-3">
            {noteOpen
              ? L('收起', 'Collapse', language)
              : L('展开', 'Expand', language)}
          </span>
        </button>
        {noteOpen && (
          <ul className="list-disc space-y-1.5 border-t border-line px-8 py-3 text-[13px] leading-relaxed text-fg-2">
            {notes.map((n) => (
              <li key={n}>{n}</li>
            ))}
          </ul>
        )}
      </Card>

      {zoo === null || horizonKeys.length === 0 ? (
        <Card className="mb-4">
          <CardHeader title={L('因子榜单', 'Factor leaderboard', language)} />
          <EmptyState
            title={L('暂无因子数据', 'No factor data yet', language)}
            description={L(
              '该市场尚未产出结果，等待定时任务运行。',
              'This market has no results yet; waiting for the scheduled job.',
              language
            )}
          />
        </Card>
      ) : (
        <FactorTable
          key={`${market}-${horizon}`}
          factors={factors}
          language={language}
        />
      )}

      <EnsembleCard ensemble={hEnsemble} language={language} />
      <RunsTable runs={runs} language={language} />
    </div>
  )
}
