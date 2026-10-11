import useSWR from 'swr'
import { api } from '../../lib/api'
import { t, type Language } from '../../i18n/translations'
import type { RiskStatus } from '../../types'

// RiskStatusPanel surfaces the program-enforced risk state the backend
// already computes (2026-10-07 review F1): daily-loss halt, account drawdown
// breaker, loss-streak bans, resting limit entries and open positions in R.

type Tone = 'up' | 'down' | 'brand' | 'muted'
const TEXT: Record<Tone, string> = {
  up: 'text-up',
  down: 'text-down',
  brand: 'text-brand',
  muted: 'text-fg-3',
}
const BG: Record<Tone, string> = {
  up: 'bg-up',
  down: 'bg-down',
  brand: 'bg-brand',
  muted: 'bg-fg-3',
}

function meterTone(value: number, cap: number): Tone {
  if (cap <= 0) return 'muted'
  const ratio = value / cap
  if (ratio >= 1) return 'down'
  if (ratio >= 0.6) return 'brand'
  return 'up'
}

function fmtR(r: number) {
  return `${r >= 0 ? '+' : ''}${r.toFixed(2)}R`
}

function fmtTime(iso: string) {
  const d = new Date(iso)
  return Number.isNaN(d.getTime())
    ? '--'
    : d.toLocaleString(undefined, {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })
}

function Meter({
  label,
  value,
  cap,
  language,
}: {
  label: string
  value: number
  cap: number
  language: Language
}) {
  const shown = Math.max(0, value)
  const tone = meterTone(shown, cap)
  const pct = cap > 0 ? Math.min(100, (shown / cap) * 100) : 0
  return (
    <div>
      <div className="mb-1 flex justify-between text-xs">
        <span className="text-fg-3">{label}</span>
        <span className={`num ${TEXT[tone]}`}>
          {shown.toFixed(2)}% /{' '}
          {cap > 0
            ? `${cap}% ${t('riskStatus.cap', language)}`
            : t('riskStatus.off', language)}
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded bg-line">
        <div
          className={`h-full rounded ${BG[tone]}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}

export function RiskStatusPanel({
  traderId,
  language,
}: {
  traderId: string
  language: Language
}) {
  const { data, error } = useSWR<RiskStatus>(
    traderId ? `risk-status-${traderId}` : null,
    () => api.getRiskStatus(traderId, true),
    { refreshInterval: 30000, dedupingInterval: 10000 }
  )

  if (error && !data) {
    return (
      <div className="rounded-lg border border-line bg-surface p-3 text-xs text-fg-3">
        {t('riskStatus.unavailable', language)}
      </div>
    )
  }
  if (!data) return null

  const alerts: { text: string; tone: Tone }[] = []
  if (data.daily_halted)
    alerts.push({
      text: `${t('riskStatus.dailyLoss', language)}: ${t('riskStatus.halted', language)}`,
      tone: 'down',
    })
  if (data.account_breaker_active)
    alerts.push({
      text: `${t('riskStatus.accountDrawdown', language)}: ${t('riskStatus.halted', language)}`,
      tone: 'down',
    })
  if (data.reduce_only)
    alerts.push({ text: t('riskStatus.reduceOnly', language), tone: 'brand' })
  if (data.protection_fault)
    alerts.push({
      text: `${t('riskStatus.protectionFault', language)}: ${data.protection_fault}`,
      tone: 'down',
    })

  return (
    <div className="space-y-3 rounded-lg border border-line bg-surface p-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-fg">
          {t('riskStatus.title', language)}
        </h2>
        <span
          className={`text-xs font-semibold ${
            alerts.length ? TEXT[alerts[0].tone] : 'text-up'
          }`}
        >
          {alerts.length ? alerts[0].text : t('riskStatus.ok', language)}
        </span>
      </div>
      {alerts.length > 1 && (
        <ul className="space-y-1 text-xs">
          {alerts.slice(1).map((a) => (
            <li key={a.text} className={TEXT[a.tone]}>
              {a.text}
            </li>
          ))}
        </ul>
      )}

      <div className="grid grid-cols-1 gap-x-6 gap-y-3 md:grid-cols-2">
        <Meter
          label={t('riskStatus.dailyLoss', language)}
          value={data.daily_loss_pct}
          cap={data.daily_loss_cap_pct}
          language={language}
        />
        <Meter
          label={t('riskStatus.accountDrawdown', language)}
          value={data.account_drawdown_pct}
          cap={data.account_drawdown_cap_pct}
          language={language}
        />
      </div>

      <div className="grid grid-cols-1 gap-x-6 gap-y-3 border-t border-line pt-3 text-xs md:grid-cols-3">
        <div>
          <div className="mb-1 text-fg-3">
            {t('riskStatus.lossStreak', language)}
          </div>
          {!data.loss_streak_enabled ? (
            <div className="text-fg-3">
              {t('riskStatus.lossStreakOff', language)}
            </div>
          ) : data.loss_streak_bans.length === 0 ? (
            <div className="text-up">{t('riskStatus.none', language)}</div>
          ) : (
            data.loss_streak_bans.map((b) => (
              <div key={b.symbol} className="num text-down">
                {b.symbol} · {t('riskStatus.until', language)}{' '}
                {fmtTime(b.until)}
              </div>
            ))
          )}
        </div>

        <div>
          <div className="mb-1 text-fg-3">
            {t('riskStatus.pending', language)}
          </div>
          {data.pending_entries.length === 0 ? (
            <div className="text-fg-3">{t('riskStatus.none', language)}</div>
          ) : (
            data.pending_entries.map((p) => (
              <div
                key={`${p.symbol}-${p.side}`}
                className="num"
                title={`SL ${p.stop_loss} · TP ${p.take_profit} · ${fmtTime(p.placed_at)}`}
              >
                <span className={p.side === 'long' ? 'text-up' : 'text-down'}>
                  {p.side.toUpperCase()}
                </span>{' '}
                {p.symbol} @ {p.price}
              </div>
            ))
          )}
        </div>

        <div>
          <div className="mb-1 text-fg-3">
            {t('riskStatus.positionsR', language)}
          </div>
          {data.positions.length === 0 ? (
            <div className="text-fg-3">{t('riskStatus.none', language)}</div>
          ) : (
            data.positions.map((p) => (
              <div
                key={`${p.symbol}-${p.side}`}
                className="num"
                title={
                  p.has_initial_stop
                    ? `SL0 ${p.initial_stop} · SL ${p.current_stop} · ${p.exit_mode}`
                    : p.ai_managed
                      ? t('riskStatus.noInitialStop', language)
                      : t('riskStatus.manualPositionHint', language)
                }
              >
                {p.symbol}{' '}
                {p.has_initial_stop ? (
                  <>
                    <span
                      className={p.current_r >= 0 ? 'text-up' : 'text-down'}
                    >
                      {t('riskStatus.currentR', language)} {fmtR(p.current_r)}
                    </span>
                    {p.current_stop > 0 && (
                      <span
                        className={
                          p.stop_locked_r >= 0 ? 'text-up' : 'text-fg-3'
                        }
                      >
                        {' '}
                        · {t('riskStatus.lockedR', language)}{' '}
                        {fmtR(p.stop_locked_r)}
                      </span>
                    )}
                  </>
                ) : p.ai_managed ? (
                  // An AI-opened position without an opening-stop anchor is a
                  // real protection gap — flag it, unlike hands-off manual ones.
                  <span className="text-warn">
                    {t('riskStatus.noInitialStop', language)}
                  </span>
                ) : (
                  <span className="text-fg-3">
                    {t('riskStatus.manualPosition', language)}
                  </span>
                )}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
