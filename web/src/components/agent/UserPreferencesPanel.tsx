import { useEffect, useState } from 'react'

interface Preference {
  id: string
  text: string
  created_at?: string
}

interface Props {
  token: string | null
  language: string
}

export function UserPreferencesPanel({ token, language }: Props) {
  const [preferences, setPreferences] = useState<Preference[]>([])
  const [draft, setDraft] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const loadPreferences = async () => {
    if (!token) {
      setPreferences([])
      setLoading(false)
      return
    }

    setLoading(true)
    try {
      const res = await fetch('/api/agent/preferences', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (!res.ok) throw new Error('Failed to load preferences')
      const data = await res.json()
      setPreferences(Array.isArray(data.preferences) ? data.preferences : [])
      setError(null)
    } catch {
      setError(language === 'zh' ? '加载偏好失败' : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!token) {
      setPreferences([])
      setLoading(false)
      return
    }

    let cancelled = false
    void (async () => {
      await loadPreferences()
    })()

    const handleRefresh = () => {
      if (!cancelled) void loadPreferences()
    }
    window.addEventListener('agent-preferences-refresh', handleRefresh)

    return () => {
      cancelled = true
      window.removeEventListener('agent-preferences-refresh', handleRefresh)
    }
  }, [token, language])

  const addPreference = async () => {
    const text = draft.trim()
    if (!text || !token || saving) return

    setSaving(true)
    try {
      const res = await fetch('/api/agent/preferences', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ text }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'save failed')
      setPreferences(Array.isArray(data.preferences) ? data.preferences : [])
      setDraft('')
      setError(null)
    } catch {
      setError(language === 'zh' ? '保存偏好失败' : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  const removePreference = async (id: string) => {
    if (!token || saving) return
    setSaving(true)
    try {
      const res = await fetch(`/api/agent/preferences/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(data.error || 'delete failed')
      setPreferences(Array.isArray(data.preferences) ? data.preferences : [])
      setError(null)
    } catch {
      setError(language === 'zh' ? '删除偏好失败' : 'Failed to delete')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      id="agent-preferences-panel"
      style={{
        background: 'rgba(255,255,255,0.02)',
        border: '1px solid rgba(255,255,255,0.05)',
        borderRadius: 12,
        padding: 10,
      }}
    >
      <div style={{ marginBottom: 8 }}>
        <div style={{ color: '#d7d7e0', fontSize: 12, fontWeight: 600 }}>
          {language === 'zh' ? '长期偏好' : 'Persistent Preferences'}
        </div>
        <div style={{ color: '#77778d', fontSize: 11, lineHeight: 1.5, marginTop: 4 }}>
          {language === 'zh'
            ? '把长期偏好固定下来，比如“默认用中文回答”或“优先关注 BTC 和 ETH”。'
            : 'Pin durable preferences the agent should keep in mind, like answering in Chinese or focusing on BTC and ETH.'}
        </div>
      </div>

      <div style={{ display: 'flex', gap: 6, marginBottom: 8 }}>
        <input
          data-agent-preferences-input="true"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void addPreference()
          }}
          placeholder={language === 'zh' ? '例如：默认用中文回答，优先关注 BTC、ETH' : 'Example: Answer in Chinese and focus on BTC, ETH'}
          style={{
            flex: 1,
            background: 'rgba(255,255,255,0.03)',
            border: '1px solid rgba(255,255,255,0.08)',
            color: '#e8e8f0',
            borderRadius: 8,
            padding: '8px 10px',
            fontSize: 12,
            outline: 'none',
          }}
        />
        <button
          onClick={() => void addPreference()}
          disabled={!draft.trim() || saving}
          style={{
            background: draft.trim() && !saving ? 'rgba(240,185,11,0.12)' : 'rgba(255,255,255,0.04)',
            color: draft.trim() && !saving ? '#F0B90B' : '#6d6d82',
            border: '1px solid rgba(240,185,11,0.14)',
            borderRadius: 8,
            padding: '0 10px',
            fontSize: 12,
            cursor: draft.trim() && !saving ? 'pointer' : 'default',
          }}
        >
          {language === 'zh' ? '添加' : 'Add'}
        </button>
      </div>

      {error && (
        <div style={{ color: '#f08a8a', fontSize: 11, marginBottom: 8 }}>{error}</div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {loading ? (
          <div style={{ color: '#77778d', fontSize: 11 }}>
            {language === 'zh' ? '加载中...' : 'Loading...'}
          </div>
        ) : preferences.length === 0 ? (
          <div style={{ color: '#77778d', fontSize: 11, lineHeight: 1.5 }}>
            {language === 'zh'
              ? '还没有长期偏好。你可以把关注标的、风险倾向、回答习惯放在这里。'
              : 'No persistent preferences yet. Add watchlists, risk preferences, or response habits here.'}
          </div>
        ) : (
          preferences.map((pref) => (
            <div
              key={pref.id}
              style={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 8,
                padding: 8,
                borderRadius: 10,
                background: 'rgba(255,255,255,0.025)',
                border: '1px solid rgba(255,255,255,0.04)',
              }}
            >
              <div style={{ flex: 1, color: '#d7d7e0', fontSize: 12, lineHeight: 1.5 }}>
                {pref.text}
              </div>
              <button
                onClick={() => void removePreference(pref.id)}
                disabled={saving}
                style={{
                  background: 'transparent',
                  border: 'none',
                  color: '#8b8ba0',
                  fontSize: 11,
                  cursor: saving ? 'default' : 'pointer',
                  padding: 0,
                }}
              >
                {language === 'zh' ? '删除' : 'Delete'}
              </button>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
