import { useMemo, useState } from 'react'
import { Badge, Card, Segmented, Stat } from '../components/ui'
import {
  RESEARCH_LOG,
  RESEARCH_METHOD_NOTE,
  type ResearchVerdict,
} from '../constants/researchLog'

type Filter = 'all' | ResearchVerdict

const VERDICT_VARIANT: Record<
  ResearchVerdict,
  'down' | 'warn' | 'up' | 'info'
> = {
  none: 'down',
  partial: 'warn',
  adopted: 'up',
  running: 'info',
}

const L = (zh: string, en: string, language: string) =>
  language === 'zh' ? zh : en

export default function ResearchLogPage({ language }: { language: string }) {
  const [filter, setFilter] = useState<Filter>('all')

  const counts = useMemo(() => {
    const c: Record<ResearchVerdict, number> = {
      none: 0,
      partial: 0,
      adopted: 0,
      running: 0,
    }
    RESEARCH_LOG.forEach((e) => (c[e.verdict] += 1))
    return c
  }, [])

  const groups = useMemo(() => {
    const list = RESEARCH_LOG.filter(
      (e) => filter === 'all' || e.verdict === filter
    )
    const byDate = new Map<string, typeof list>()
    list.forEach((e) => {
      byDate.set(e.date, [...(byDate.get(e.date) ?? []), e])
    })
    return Array.from(byDate.entries())
  }, [filter])

  return (
    <div className="mx-auto max-w-[980px] px-4 py-6">
      <div className="mb-4">
        <h1 className="text-xl font-semibold text-fg">
          {L('量化研究记录', 'Quant Research Log', language)}
        </h1>
        <p className="mt-1 text-[13px] text-fg-3">
          {L(
            '已尝试 / 回测 / 验证过的量化思路，按时间倒序。内容为中文研究笔记。',
            'Ideas that were tried, backtested or validated, newest first. Notes are written in Chinese.',
            language
          )}
        </p>
      </div>

      <div className="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-5">
        <Card className="p-3">
          <Stat
            label={L('已记录', 'Total', language)}
            value={RESEARCH_LOG.length}
          />
        </Card>
        <Card className="p-3">
          <Stat label={L('无优势', 'No edge', language)} value={counts.none} />
        </Card>
        <Card className="p-3">
          <Stat
            label={L('有信号但不足', 'Partial', language)}
            value={counts.partial}
          />
        </Card>
        <Card className="p-3">
          <Stat
            label={L('已采用/有支持', 'Adopted', language)}
            value={counts.adopted}
          />
        </Card>
        <Card className="p-3">
          <Stat
            label={L('进行中', 'Running', language)}
            value={counts.running}
          />
        </Card>
      </div>

      <Card className="mb-4 border-l-2 border-l-brand p-3 text-[13px] leading-relaxed text-fg-2">
        <span className="font-semibold text-brand">
          {L('阅读口径：', 'How to read: ', language)}
        </span>
        {RESEARCH_METHOD_NOTE}
      </Card>

      <Segmented<Filter>
        className="mb-2"
        value={filter}
        onChange={setFilter}
        items={[
          { key: 'all', label: L('全部', 'All', language) },
          { key: 'none', label: L('无优势', 'No edge', language) },
          { key: 'partial', label: L('有信号不足以上线', 'Partial', language) },
          { key: 'adopted', label: L('已采用/有支持', 'Adopted', language) },
          { key: 'running', label: L('进行中', 'Running', language) },
        ]}
      />

      {groups.map(([date, entries]) => (
        <section key={date} className="mt-6">
          <div className="mb-2 border-b border-line pb-1 text-xs font-semibold tracking-wide text-fg-3">
            {date}
          </div>
          <div className="space-y-3">
            {entries.map((e) => (
              <Card key={e.id} className="p-4">
                <div className="flex flex-wrap items-center gap-2">
                  <h2 className="text-[15px] font-semibold text-fg">
                    {e.title}
                  </h2>
                  <Badge variant={VERDICT_VARIANT[e.verdict]}>
                    {e.verdictText}
                  </Badge>
                  <Badge variant="neutral" size="xs">
                    {e.domain}
                  </Badge>
                </div>
                <p className="mt-2 text-[13px] leading-relaxed text-fg-2">
                  {e.idea}
                </p>
                {e.paragraphs?.map((p) => (
                  <p
                    key={p}
                    className="mt-2 text-[13px] leading-relaxed text-fg-2"
                  >
                    {p}
                  </p>
                ))}
                {e.table && (
                  <div className="mt-3 overflow-x-auto">
                    <table className="w-full border-collapse text-[12.5px]">
                      <thead>
                        <tr>
                          {e.table.head.map((h, i) => (
                            <th
                              key={h}
                              className={`whitespace-nowrap border-b border-line px-2.5 py-1 font-medium text-fg-3 ${i === 0 ? 'text-left' : 'text-right'}`}
                            >
                              {h}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {e.table.rows.map((r) => (
                          <tr key={r.join('|')}>
                            {r.map((c, i) => (
                              <td
                                key={`${i}-${c}`}
                                className={`whitespace-nowrap border-b border-line px-2.5 py-1 text-fg ${i === 0 ? 'text-left' : 'text-right'}`}
                              >
                                {c}
                              </td>
                            ))}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                {e.bullets && (
                  <ul className="mt-2 list-disc space-y-1 pl-5 text-[13px] leading-relaxed text-fg-2">
                    {e.bullets.map((b) => (
                      <li key={b}>{b}</li>
                    ))}
                  </ul>
                )}
                {e.conclusion && (
                  <p className="mt-2 text-[13px] leading-relaxed text-fg">
                    <span className="font-semibold">
                      {L('结论：', 'Conclusion: ', language)}
                    </span>
                    {e.conclusion}
                  </p>
                )}
                <p className="mt-2 break-all text-xs text-fg-3">
                  {L('来源：', 'Source: ', language)}
                  {e.source}
                </p>
              </Card>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
