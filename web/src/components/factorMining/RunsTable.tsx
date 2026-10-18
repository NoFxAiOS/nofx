import { Card, CardHeader, EmptyState } from '../ui'
import type { FactorMiningRun } from '../../types/factorMining'
import { L, fmtInt, fmtNum, fmtText } from './format'

const TH_CLS =
  'h-8 bg-surface-2 px-3 py-0 text-left text-xs font-medium text-fg-3 whitespace-nowrap'
const TD_CLS = 'px-3 py-1.5 whitespace-nowrap'

export default function RunsTable({
  runs,
  language,
}: {
  runs: FactorMiningRun[]
  language: string
}) {
  return (
    <Card className="mb-4">
      <CardHeader title={L('运行历史', 'Run history', language)} />
      {runs.length === 0 ? (
        <EmptyState title={L('暂无运行记录', 'No runs yet', language)} />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-[12.5px]">
            <thead>
              <tr>
                <th className={TH_CLS}>{L('开始时间', 'Started', language)}</th>
                <th className={TH_CLS}>{L('市场', 'Market', language)}</th>
                <th className={TH_CLS}>{L('耗时(秒)', 'Secs', language)}</th>
                <th className={TH_CLS}>{L('种子评估', 'Seed', language)}</th>
                <th className={TH_CLS}>
                  {L('变异评估', 'Mutation', language)}
                </th>
                <th className={TH_CLS}>
                  {L('LLM 提案/有效', 'LLM prop/valid', language)}
                </th>
                <th className={TH_CLS}>
                  {L('新增 accepted', 'New accepted', language)}
                </th>
                <th className={TH_CLS}>
                  {L('新增 candidate', 'New candidate', language)}
                </th>
                <th className={TH_CLS}>
                  {L('累计试验', 'Total trials', language)}
                </th>
                <th className={TH_CLS}>{L('错误', 'Error', language)}</th>
              </tr>
            </thead>
            <tbody>
              {runs.map((r, i) => (
                <tr
                  key={r.run_id ?? `${r.market}-${r.started_at}-${i}`}
                  className="border-b border-line last:border-b-0 hover:bg-surface-hover"
                >
                  <td className={TD_CLS}>{fmtText(r.started_at)}</td>
                  <td className={TD_CLS}>{fmtText(r.market)}</td>
                  <td className={`${TD_CLS} num`}>{fmtNum(r.seconds, 0)}</td>
                  <td className={`${TD_CLS} num`}>
                    {fmtInt(r.seed_evaluated)}
                  </td>
                  <td className={`${TD_CLS} num`}>
                    {fmtInt(r.mutation_evaluated)}
                  </td>
                  <td className={`${TD_CLS} num`}>
                    {fmtInt(r.llm_proposed)} / {fmtInt(r.llm_valid)}
                  </td>
                  <td className={`${TD_CLS} num`}>{fmtInt(r.new_accepted)}</td>
                  <td className={`${TD_CLS} num`}>{fmtInt(r.new_candidate)}</td>
                  <td className={`${TD_CLS} num`}>{fmtInt(r.trials_total)}</td>
                  <td
                    className={`${TD_CLS} max-w-[260px] truncate ${r.error ? 'text-down' : ''}`}
                    title={r.error ?? undefined}
                  >
                    {r.error ? r.error : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  )
}
