import { useEffect, useState } from 'react'
import { Badge, Card, EmptyState } from '../components/ui'
import {
  L,
  fmtInt,
  fmtNum,
  fmtText,
  isNum,
} from '../components/factorMining/format'
import { getAuthHeaders } from '../lib/api/helpers'
import type {
  PromotionComparison,
  PromotionPair,
  PromotionReport,
  PromotionStats,
  PromotionVerdict,
} from '../types/promotion'

const VERDICT_VARIANT: Record<PromotionVerdict, 'up' | 'warn' | 'neutral'> = {
  ready: 'up',
  hold: 'warn',
  insufficient: 'neutral',
}

const percent = (value: unknown) =>
  isNum(value) ? `${(value * 100).toFixed(2)}%` : '—'
const percentagePoints = (value: unknown) =>
  isNum(value) ? `${(value * 100).toFixed(2)} pp` : '—'

function metrics(language: string) {
  return [
    { key: 'n', label: L('笔数', 'Trades', language), format: fmtInt },
    {
      key: 'net_pnl_usdt',
      label: L('净盈亏 (USDT)', 'Net P&L (USDT)', language),
      format: (value: unknown) => fmtNum(value, 2),
    },
    {
      key: 'period_return',
      label: L('期间收益', 'Period return', language),
      format: percent,
    },
    { key: 'max_drawdown', label: 'MaxDD', format: percent },
    {
      key: 'win_rate',
      label: L('胜率', 'Win rate', language),
      format: percent,
    },
    {
      key: 'avg_r',
      label: 'avgR',
      format: (value: unknown) => fmtNum(value, 3),
    },
  ] satisfies {
    key: keyof PromotionStats
    label: string
    format: (value: unknown) => string
  }[]
}

function Progress({
  label,
  value,
  min,
  days = false,
}: {
  label: string
  value?: number | null
  min: number
  days?: boolean
}) {
  const current = isNum(value) ? Math.max(0, Math.min(value, min)) : 0
  return (
    <div className="min-w-0">
      <div className="mb-1 flex flex-wrap justify-between gap-x-2 text-xs text-fg-3">
        <span>{label}</span>
        <span className="tabular-nums text-fg-2">
          {days ? fmtNum(value, 2) : fmtInt(value)} /{' '}
          {fmtNum(min, days && !Number.isInteger(min) ? 2 : 0)}
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={min}
        aria-valuenow={current}
        className="h-1.5 overflow-hidden rounded-full bg-surface-2"
      >
        <div
          className="h-full rounded-full bg-brand"
          style={{ width: `${(current / min) * 100}%` }}
        />
      </div>
    </div>
  )
}

function Comparison({
  label,
  comparison,
  rates = false,
  sample,
  note,
}: {
  label: string
  comparison?: PromotionComparison | null
  rates?: boolean
  sample: string
  note?: string | null
}) {
  const format = rates ? percentagePoints : (v: unknown) => fmtNum(v, 3)
  return (
    <div className="min-w-0 rounded-md bg-surface-2 p-3 text-[13px]">
      <p className="font-medium text-fg-2">{label}</p>
      <p className="mt-1 break-words tabular-nums">
        {format(comparison?.mean_diff)} · {sample}
      </p>
      {comparison?.ci95 && (
        <p className="mt-1 break-words text-xs text-fg-2">
          95% CI [{comparison.ci95.map(format).join(', ')}]
        </p>
      )}
      {note && <p className="mt-1 break-words text-xs text-fg-3">{note}</p>}
    </div>
  )
}

function PairCard({
  pair,
  language,
}: {
  pair: PromotionPair
  language: string
}) {
  const rows = metrics(language)
  const minDays = isNum(pair.min_days) ? Math.max(7, pair.min_days) : 7
  const minTrades =
    isNum(pair.min_trades) && pair.min_trades > 0 ? pair.min_trades : 20
  return (
    <Card
      className="min-w-0 p-4"
      role="article"
      aria-label={pair.title || pair.id}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <h2 className="break-words text-[15px] font-semibold">
            {pair.title || pair.id}
          </h2>
          <p className="mt-1 break-all text-xs text-fg-3">
            feature: {fmtText(pair.feature)}
          </p>
          <p className="mt-1 break-words text-xs text-fg-3">
            {L('开始于', 'Started', language)} {fmtText(pair.started_at)}
          </p>
        </div>
        <Badge variant={VERDICT_VARIANT[pair.verdict ?? 'insufficient']}>
          {pair.verdict ?? 'insufficient'}
        </Badge>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-3">
        <Progress
          label={L('已运行天数 / 最少天数', 'Elapsed days / minimum', language)}
          value={pair.elapsed_days}
          min={minDays}
          days
        />
        <Progress
          label={L(
            '候选笔数 / 最少笔数',
            'Candidate trades / minimum',
            language
          )}
          value={pair.candidate?.n}
          min={minTrades}
        />
        <Progress
          label={L('对照笔数 / 最少笔数', 'Control trades / minimum', language)}
          value={pair.control?.n}
          min={minTrades}
        />
      </div>

      <table className="mt-4 w-full table-fixed border-collapse text-xs sm:text-[13px]">
        <caption className="sr-only">
          {L('候选与对照指标', 'Candidate and control metrics', language)}
        </caption>
        <thead>
          <tr className="border-b border-line text-fg-3">
            <th scope="col" className="w-[34%] py-2 text-left font-medium">
              {L('指标', 'Metric', language)}
            </th>
            {(['candidate', 'control'] as const).map((role) => (
              <th
                key={role}
                scope="col"
                className="px-1 py-2 text-right font-medium"
              >
                {role === 'candidate'
                  ? L('候选', 'Candidate', language)
                  : L('对照', 'Control', language)}
                <span className="mt-1 block break-all text-[10px] font-normal">
                  {fmtText(pair[role]?.trader_id || pair[`${role}_trader_id`])}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map(({ key, label, format }) => (
            <tr key={key} className="border-b border-line">
              <th
                scope="row"
                className="break-words py-2 text-left font-normal text-fg-3"
              >
                {label}
              </th>
              <td className="break-all px-1 py-2 text-right tabular-nums">
                {format(pair.candidate?.[key])}
              </td>
              <td className="break-all px-1 py-2 text-right tabular-nums">
                {format(pair.control?.[key])}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {pair.live && (
        <div className="mt-3 rounded-md border border-line p-3">
          <p className="break-all text-xs text-fg-3">
            {L(
              '正式参照（不参与判定）',
              'Live reference (excluded from verdict)',
              language
            )}
            {' · '}
            {fmtText(pair.live.trader_id || pair.live_trader_id)}
          </p>
          <dl className="mt-2 grid grid-cols-2 gap-2 text-xs sm:grid-cols-3 lg:grid-cols-6">
            {rows.map(({ key, label, format }) => (
              <div key={key} className="min-w-0">
                <dt className="text-fg-3">{label}</dt>
                <dd className="mt-1 break-all tabular-nums">
                  {format(pair.live?.[key])}
                </dd>
              </div>
            ))}
          </dl>
        </div>
      )}

      <p className="mt-4 text-xs text-fg-3">
        {L(
          '配对差：候选 − 对照（pp 为百分点）',
          'Paired differences: candidate − control (pp = percentage points)',
          language
        )}
      </p>
      <div className="mt-2 grid gap-3 sm:grid-cols-2">
        <Comparison
          label={L(
            '完整日收益差均值',
            'Mean completed-day return difference',
            language
          )}
          comparison={pair.paired_daily}
          rates
          sample={`${fmtInt(pair.paired_daily?.n_days)} ${L('配对日', 'paired days', language)}`}
          note={pair.paired_daily?.ci_note}
        />
        <Comparison
          label={L(
            'avgR 差（按周聚类）',
            'avgR difference (weekly clusters)',
            language
          )}
          comparison={pair.weekly_avg_r}
          sample={`${fmtInt(pair.weekly_avg_r?.n_weeks)} ${L('周', 'weeks', language)}`}
          note={pair.weekly_avg_r?.note}
        />
      </div>
      <p className="mt-2 text-xs text-fg-3">
        {L('期间累计收益差', 'Period cumulative return difference', language)}：{' '}
        {percentagePoints(pair.paired_daily?.cumulative_return_diff)}
      </p>
      {!!pair.reasons?.length && (
        <ul className="mt-3 list-disc space-y-1 break-words pl-5 text-[13px] text-fg-2">
          {pair.reasons.map((reason, index) => (
            <li key={index}>{reason}</li>
          ))}
        </ul>
      )}
      {!!pair.warnings?.length && (
        <ul className="mt-3 list-disc space-y-1 break-words pl-5 text-xs text-warn">
          {pair.warnings.map((warning, index) => (
            <li key={index}>{warning}</li>
          ))}
        </ul>
      )}
      {pair.caveats && (
        <p className="mt-3 break-words text-xs leading-relaxed text-fg-3">
          {pair.caveats}
        </p>
      )}
    </Card>
  )
}

export default function PaperPromotionPage({ language }: { language: string }) {
  const [report, setReport] = useState<PromotionReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)
  const [refresh, setRefresh] = useState(0)

  useEffect(() => {
    let active = true
    let timer: number | undefined
    const controller = new AbortController()
    const load = async () => {
      try {
        const response = await fetch('/api/promotion/report', {
          headers: getAuthHeaders(),
          signal: controller.signal,
        })
        if (!response.ok) throw new Error(`HTTP ${response.status}`)
        const data = (await response.json()) as PromotionReport | null
        if (active) {
          setReport(data)
          setFailed(false)
        }
      } catch {
        if (active) setFailed(true)
      } finally {
        if (active) setLoading(false)
      }
    }
    const tick = async () => {
      if (!document.hidden) await load()
      if (active) timer = window.setTimeout(tick, 30_000)
    }
    const onVisible = () => {
      if (!document.hidden) {
        window.clearTimeout(timer)
        void tick()
      }
    }
    document.addEventListener('visibilitychange', onVisible)
    void tick()
    return () => {
      active = false
      controller.abort()
      window.clearTimeout(timer)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [refresh])

  const pairs = Array.isArray(report?.pairs) ? report.pairs : []
  return (
    <div className="mx-auto min-w-0 max-w-[1100px] px-4 py-6 sm:px-6">
      <h1 className="text-xl font-semibold text-fg">
        {L('模拟盘晋升报告', 'Paper Promotion Report', language)}
      </h1>
      <p className="mt-2 text-[13px] leading-relaxed text-fg-3">
        {L(
          '候选 vs 对照，模拟盘 ≥7 天仍为正向，才可手动上线正式交易员；本页仅提示，不会修改任何配置',
          'Candidate vs control: at least 7 days of positive paper validation before manually applying to a live trader; this page is advisory and never changes configuration.',
          language
        )}
      </p>
      <Card className="my-4 flex flex-wrap items-center justify-between gap-2 p-3 text-[13px] text-fg-3">
        <span className="min-w-0 break-words" role="status">
          {loading
            ? L('加载中…', 'Loading…', language)
            : `${L('更新于', 'Updated', language)} ${fmtText(report?.generated_at)}`}
        </span>
        <button
          type="button"
          className="text-brand"
          onClick={() => setRefresh((value) => value + 1)}
        >
          {L('刷新', 'Refresh', language)}
        </button>
      </Card>
      {failed && (
        <p role="alert" className="mb-4 text-[13px] text-down">
          {L(
            '接口请求失败，稍后自动重试；也可点击刷新',
            'Request failed, will retry automatically; you can also refresh',
            language
          )}
        </p>
      )}
      {!loading && !failed && pairs.length === 0 && (
        <Card className="p-4">
          <EmptyState
            title={L(
              '暂无候选 / 对照实验',
              'No candidate / control experiments',
              language
            )}
          />
          <p className="text-center text-[13px] text-fg-3">
            {L('运行', 'Run', language)}{' '}
            <code className="break-all text-fg-2">
              scripts/promotion/create_pair.py …
            </code>
            {L(
              '，建立候选与对照模拟交易员，并登记到 data/promotion/pairs.json；夜间任务每日更新报告。',
              ' to create candidate and control paper traders and register data/promotion/pairs.json; the nightly job updates the report daily.',
              language
            )}
          </p>
        </Card>
      )}
      <div className="space-y-4">
        {pairs.map((pair) => (
          <PairCard key={pair.id} pair={pair} language={language} />
        ))}
      </div>
      {report?.caveats && (
        <p className="mt-4 break-words text-xs leading-relaxed text-fg-3">
          {report.caveats}
        </p>
      )}
    </div>
  )
}
