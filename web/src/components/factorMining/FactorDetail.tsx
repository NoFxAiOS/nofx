import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Button } from '../ui'
import { copyWithToast } from '../../lib/clipboard'
import { getChartTheme, useThemeVersion } from '../../lib/chartTheme'
import type { FactorEntry } from '../../types/factorMining'
import { L, fmtIC, fmtText, isNum } from './format'

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
