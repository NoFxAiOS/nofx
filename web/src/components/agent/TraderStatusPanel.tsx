import useSWR from 'swr'
import { useAuth } from '../../contexts/AuthContext'
import { api } from '../../lib/api'
import { Activity, CircleOff, Bot } from 'lucide-react'
import type { TraderInfo } from '../../types'

export function TraderStatusPanel() {
  const { user, token } = useAuth()

  const { data: traders } = useSWR<TraderInfo[]>(
    user && token ? 'agent-sidebar-traders' : null,
    api.getTraders,
    { refreshInterval: 30000, shouldRetryOnError: false }
  )

  if (!user || !token) {
    return (
      <div
        style={{
          padding: '20px 14px',
          textAlign: 'center',
          color: '#5c5c72',
          fontSize: 12,
        }}
      >
        <Bot size={20} style={{ margin: '0 auto 8px', opacity: 0.5 }} />
        <div>Login to view traders</div>
      </div>
    )
  }

  if (!traders || traders.length === 0) {
    return (
      <div
        style={{
          padding: '16px 14px',
          textAlign: 'center',
          color: '#5c5c72',
          fontSize: 12,
        }}
      >
        No traders configured
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      {traders.map((trader) => (
        <div
          key={trader.trader_id}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '10px 12px',
            background: '#0d0d15',
            borderRadius: 10,
            border: '1px solid #1a1a28',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <div
              style={{
                width: 28,
                height: 28,
                borderRadius: 7,
                background: trader.is_running
                  ? 'rgba(0,229,160,0.08)'
                  : 'rgba(92,92,114,0.08)',
                display: 'grid',
                placeItems: 'center',
              }}
            >
              {trader.is_running ? (
                <Activity size={14} color="#00e5a0" />
              ) : (
                <CircleOff size={14} color="#5c5c72" />
              )}
            </div>
            <div>
              <div
                style={{ fontSize: 13, fontWeight: 600, color: '#eaeaf0' }}
              >
                {trader.trader_name}
              </div>
              <div style={{ fontSize: 10, color: '#5c5c72' }}>
                {trader.trader_id.slice(0, 8)}...
              </div>
            </div>
          </div>
          <div
            style={{
              fontSize: 10,
              fontWeight: 600,
              padding: '3px 8px',
              borderRadius: 6,
              background: trader.is_running
                ? 'rgba(0,229,160,0.12)'
                : 'rgba(92,92,114,0.12)',
              color: trader.is_running ? '#00e5a0' : '#5c5c72',
            }}
          >
            {trader.is_running ? 'RUNNING' : 'STOPPED'}
          </div>
        </div>
      ))}
    </div>
  )
}
