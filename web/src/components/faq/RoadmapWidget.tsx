import { useEffect, useState } from 'react'

type RoadmapItem = {
  id: string
  type?: string
  title: string
  url?: string
  state?: string
  repo?: string
  status?: string
  assignees?: string[]
  iteration?: string
}

type RoadmapResponse = {
  title: string
  url: string
  items: RoadmapItem[]
  error?: string
  note?: string
}

export function RoadmapWidget() {
  const [data, setData] = useState<RoadmapResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [, setError] = useState<string | null>(null)

  useEffect(() => {
    const load = async () => {
      try {
        const res = await fetch('/api/roadmap')
        const json = (await res.json()) as RoadmapResponse
        setData(json)
      } catch (e) {
        setError((e as Error).message)
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  if (loading) {
    return (
      <div
        className="rounded p-4"
        style={{ background: '#111418', border: '1px solid #2B3139' }}
      >
        <div className="animate-pulse">
          <div className="h-4 w-48 rounded bg-[#2B3139] mb-3" />
          <div className="h-3 w-full rounded bg-[#2B3139] mb-2" />
          <div className="h-3 w-4/5 rounded bg-[#2B3139]" />
        </div>
      </div>
    )
  }

  if (!data) return null

  const items = (data.items || []).slice(0, 12)

  return (
    <div
      className="rounded p-4"
      style={{ background: '#0F1318', border: '1px solid #2B3139' }}
    >
      <div className="flex items-center justify-between mb-3">
        <a
          href={data.url}
          target="_blank"
          rel="noreferrer"
          className="text-sm font-bold hover:opacity-80"
          style={{ color: '#F0B90B' }}
        >
          {data.title || 'NOFX Roadmap'} →
        </a>
        {data.error && (
          <span className="text-xs" style={{ color: '#848E9C' }}>
            {data.error}
          </span>
        )}
      </div>

      {items.length === 0 ? (
        <div className="text-sm" style={{ color: '#848E9C' }}>
          无法加载任务，点击上方链接查看完整路线图。
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {items.map((it) => (
            <a
              key={it.id}
              href={it.url}
              target="_blank"
              rel="noreferrer"
              className="block rounded p-3 hover:opacity-90 transition"
              style={{ background: '#111418', border: '1px solid #2B3139' }}
            >
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs" style={{ color: '#848E9C' }}>
                  {it.repo || it.type || ''}
                </span>
                {it.status && (
                  <span
                    className="text-xs px-2 py-0.5 rounded"
                    style={{
                      background: 'rgba(240, 185, 11, 0.12)',
                      color: '#EAECEF',
                      border: '1px solid rgba(240, 185, 11, 0.25)',
                    }}
                  >
                    {it.status}
                  </span>
                )}
              </div>
              <div
                className="text-sm font-medium mb-2"
                style={{ color: '#EAECEF' }}
              >
                {it.title}
              </div>
              <div className="flex flex-wrap gap-2">
                {it.assignees && it.assignees.length > 0 && (
                  <span className="text-xs" style={{ color: '#848E9C' }}>
                    👤 {it.assignees.join(', ')}
                  </span>
                )}
                {it.iteration && (
                  <span className="text-xs" style={{ color: '#848E9C' }}>
                    🗓 {it.iteration}
                  </span>
                )}
              </div>
            </a>
          ))}
        </div>
      )}

      {!items.length && (
        <div className="mt-2 text-xs" style={{ color: '#5E6673' }}>
          {data.note || ''}
        </div>
      )}
    </div>
  )
}
