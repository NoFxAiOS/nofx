import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Badge, Button } from '../ui'
import { copyWithToast } from '../../lib/clipboard'
import { getChartTheme, useThemeVersion } from '../../lib/chartTheme'
import type { FactorEntry } from '../../types/factorMining'
import { L, fmtIC, fmtPct, fmtT, fmtText, forwardBadge, isNum } from './format'

export default function FactorDetail({
  factor,
  language,
}: {
  factor: FactorEntry
  language: string
}) {
  useThemeVersion()
  const theme = getChartTheme()
  const monthly = (factor.ic_monthly ?? [])
    .filter((p): p is [string, number] => Array.isArray(p) && isNum(p[1]))
    .map(([month, ic]) => ({ month, ic }))
  const fw = factor.forward ?? null
  const fwBadge = forwardBadge(fw, language)
  const fwCurve = (fw?.curve ?? [])
    .filter((p): p is [string, number] => Array.isArray(p) && isNum(p[1]))
    .map(([date, cum]) => ({ date, cum }))
  const fwRemain =
    fw?.verdict === 'pending' && isNum(fw.min_days)
      ? Math.max(
          0,
          Math.round(fw.min_days - (isNum(fw.n_days) ? fw.n_days : 0))
        )
      : null

  return (
    <div className="space-y-3 p-3 text-[13px]">
      <div>
        <div className="mb-1 flex items-center gap-2 text-xs text-fg-3">
          {L('完整表达式', 'Expression', language)}
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              copyWithToast(
                factor.expr ?? '',
                L('已复制表达式', 'Expression copied', language)
              )
            }
          >
            {L('复制', 'Copy', language)}
          </Button>
        </div>
        <pre
          data-testid="factor-expr-full"
          className="overflow-x-auto whitespace-pre-wrap break-all rounded border border-line bg-surface-2 p-2 font-mono text-[12.5px] text-fg"
        >
          {fmtText(factor.expr)}
        </pre>
      </div>

      <div className="grid grid-cols-1 gap-x-6 gap-y-1 text-xs text-fg-2 sm:grid-cols-2">
        <div>
          {L('冗余对象', 'Correlated with', language)}：
          <span className="font-mono text-fg">
            {factor.corr_with ? factor.corr_with : '—'}
          </span>
          {isNum(factor.corr_max) && (
            <span className="num ml-1">
              (corr {factor.corr_max.toFixed(2)})
            </span>
          )}
        </div>
        <div>
          {L('首次发现', 'First seen', language)}：{fmtText(factor.first_seen)}
        </div>
        <div>
          run_id：<span className="font-mono">{fmtText(factor.run_id)}</span>
        </div>
      </div>

      {fw && (
        <div data-testid="factor-forward">
          <div className="mb-1 flex items-center gap-2 text-xs text-fg-3">
            {L('前向校验', 'Forward test', language)}
            {fwBadge && <Badge variant={fwBadge.variant}>{fwBadge.text}</Badge>}
          </div>
          <div className="grid grid-cols-2 gap-x-6 gap-y-1 text-xs text-fg-2 sm:grid-cols-4">
            <div>
              {L('起始日', 'Start', language)}：{fmtText(fw.start)}
            </div>
            <div>
              {L('已累计天数', 'Days so far', language)}：
              <span className="num text-fg">
                {isNum(fw.n_days) ? Math.round(fw.n_days) : '—'}
              </span>
            </div>
            <div>
              {L('最后信号日', 'Last signal day', language)}：
              {fmtText(fw.last_date)}
            </div>
            <div>
              {L('冻结方向', 'Frozen direction', language)}：
              <span className="num text-fg">
                {fw.direction === 1 ? '+1' : fw.direction === -1 ? '-1' : '—'}
              </span>
            </div>
            <div>
              {L('前向 IC', 'Forward IC', language)}：
              <span className="num text-fg">{fmtIC(fw.ic)}</span>
            </div>
            <div>
              ICIR：<span className="num text-fg">{fmtT(fw.icir)}</span>
            </div>
            <div>
              t：<span className="num text-fg">{fmtT(fw.t)}</span>
            </div>
            <div>
              {L('多空年化', 'L/S ann.', language)}：
              <span className="num text-fg">{fmtPct(fw.ls_ret)}</span>
            </div>
            <div>
              {L('换手', 'Turnover', language)}：
              <span className="num text-fg">{fmtT(fw.turnover)}</span>
            </div>
          </div>
          {fwRemain !== null && (
            <div
              data-testid="factor-forward-pending"
              className="mt-1 text-xs text-warn"
            >
              {L(
                `样本不足，还差 ${fwRemain} 天才出结论`,
                `Not enough data yet: ${fwRemain} more days needed for a verdict`,
                language
              )}
            </div>
          )}
          {fwCurve.length === 0 ? (
            <div className="py-6 text-center text-xs text-fg-3">
              {L(
                '暂无前向累计 IC 曲线',
                'No forward cumulative IC curve',
                language
              )}
            </div>
          ) : (
            <div className="mt-2 h-[180px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart
                  data={fwCurve}
                  margin={{ top: 4, right: 8, bottom: 0, left: 0 }}
                >
                  <CartesianGrid stroke={theme.grid} vertical={false} />
                  <XAxis
                    dataKey="date"
                    tick={{ fill: theme.text, fontSize: 11 }}
                    stroke={theme.border}
                    minTickGap={32}
                  />
                  <YAxis
                    domain={['auto', 'auto']}
                    tick={{ fill: theme.text, fontSize: 11 }}
                    stroke={theme.border}
                    width={44}
                    tickFormatter={(v: number) => v.toFixed(2)}
                  />
                  <Tooltip
                    formatter={(v: number) => [
                      v.toFixed(3),
                      L('累计 IC', 'Cumulative IC', language),
                    ]}
                    contentStyle={{
                      background: theme.background,
                      border: `1px solid ${theme.border}`,
                      fontSize: 12,
                    }}
                  />
                  <ReferenceLine y={0} stroke={theme.border} />
                  <Line
                    type="monotone"
                    dataKey="cum"
                    stroke={theme.brand}
                    strokeWidth={1.5}
                    dot={false}
                    isAnimationActive={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}
          <div className="mt-1 text-[11px] text-fg-3">
            {L(
              '因子首次入选时冻结方向并登记；仅统计入选前选择过程从未见过的新数据；该指标不参与任何筛选，实盘参考以此为准。',
              'Direction is frozen and the factor registered when first accepted; only new data never seen by the selection process is counted; this metric is not used in any filtering, and is the reference for live use.',
              language
            )}
          </div>
        </div>
      )}

      <div>
        <div className="mb-1 text-xs text-fg-3">
          {L('IC 月度均值', 'Monthly mean IC', language)}
        </div>
        {monthly.length === 0 ? (
          <div className="py-6 text-center text-xs text-fg-3">
            {L('暂无月度 IC 数据', 'No monthly IC data', language)}
          </div>
        ) : (
          <div className="h-[180px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={monthly}
                margin={{ top: 4, right: 8, bottom: 0, left: 0 }}
              >
                <CartesianGrid stroke={theme.grid} vertical={false} />
                <XAxis
                  dataKey="month"
                  tick={{ fill: theme.text, fontSize: 11 }}
                  stroke={theme.border}
                  minTickGap={16}
                />
                <YAxis
                  tick={{ fill: theme.text, fontSize: 11 }}
                  stroke={theme.border}
                  width={44}
                  tickFormatter={(v: number) => v.toFixed(2)}
                />
                <Tooltip
                  formatter={(v: number) => [fmtIC(v), 'IC']}
                  contentStyle={{
                    background: theme.background,
                    border: `1px solid ${theme.border}`,
                    fontSize: 12,
                  }}
                />
                <ReferenceLine y={0} stroke={theme.border} />
                <Bar dataKey="ic" isAnimationActive={false}>
                  {monthly.map((d) => (
                    <Cell
                      key={d.month}
                      fill={d.ic >= 0 ? theme.up : theme.down}
                    />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </div>
  )
}
