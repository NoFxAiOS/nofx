import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Card, CardHeader, EmptyState } from '../ui'
import { getChartTheme, useThemeVersion } from '../../lib/chartTheme'
import type { EnsembleHorizon, EnsembleMetrics } from '../../types/factorMining'
import { L, fmtIC, fmtT, isNum } from './format'

function MetricsLine({
  label,
  m,
}: {
  label: string
  m?: EnsembleMetrics | null
}) {
  return (
    <div className="flex items-baseline justify-between gap-3 text-xs">
      <span className="text-fg-3">{label}</span>
      <span className="num text-fg">
        IC {fmtIC(m?.ic)} · ICIR {fmtT(m?.icir)} · {fmtT(m?.ls_sharpe)}
      </span>
    </div>
  )
}

export default function EnsembleCard({
  ensemble,
  language,
}: {
  ensemble: EnsembleHorizon | null
  language: string
}) {
  useThemeVersion()
  const theme = getChartTheme()
  const curve = (ensemble?.curve?.val ?? [])
    .filter((p): p is [string, number] => Array.isArray(p) && isNum(p[1]))
    .map(([date, cum]) => ({ date, cum }))
  const ids = ensemble?.factor_ids ?? []

  return (
    <Card className="mb-4">
      <CardHeader
        title={L('组合曲线（val）', 'Ensemble curve (val)', language)}
        subtitle={L(
          'accepted 因子逐截面 z-score 后等权平均的多空净值',
          'Equal-weight z-score blend of accepted factors, long-short NAV',
          language
        )}
      />
      {curve.length === 0 ? (
        <EmptyState
          title={L(
            '暂无 accepted 因子组合',
            'No accepted-factor ensemble yet',
            language
          )}
        />
      ) : (
        <div className="grid grid-cols-1 gap-3 p-3 lg:grid-cols-[1fr_260px]">
          <div className="h-[220px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={curve}
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
                    L('净值', 'NAV', language),
                  ]}
                  contentStyle={{
                    background: theme.background,
                    border: `1px solid ${theme.border}`,
                    fontSize: 12,
                  }}
                />
                <ReferenceLine
                  y={1}
                  stroke={theme.border}
                  strokeDasharray="3 3"
                />
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
          <div className="space-y-2">
            <div className="text-xs text-fg-3">
              {L('组合因子数', 'Factors in ensemble', language)}
              <span className="num ml-1 text-sm font-semibold text-fg">
                {ids.length}
              </span>
            </div>
            <MetricsLine label="train" m={ensemble?.train} />
            <MetricsLine label="val" m={ensemble?.val} />
            <div className="text-[11px] text-fg-3">
              {L(
                '第三项为多空夏普',
                'Third value is the long-short Sharpe',
                language
              )}
            </div>
          </div>
        </div>
      )}
    </Card>
  )
}
