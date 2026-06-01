import { ExternalLink } from 'lucide-react'
import { useLanguage } from '../contexts/LanguageContext'

const VERGEX_EXPLORE_URL = 'https://vergex.trade/explore'

// Strategy Market — proxied to vergex.trade/explore.
//
// vergex.trade currently sets `X-Frame-Options: SAMEORIGIN` on /explore which
// makes browsers refuse cross-origin embedding and render their own "refused
// to connect" page inside the iframe. There is no reliable way to detect
// this from JavaScript (the iframe's `load` event fires for the browser
// error page, and `contentWindow.location` always throws cross-origin
// regardless of success or failure), so we don't try to be clever — we
// surface a clean external-launch CTA instead.
//
// TO RE-ENABLE INLINE EMBEDDING:
//   1. Ask the vergex.trade team to add NOFX origins to the /explore
//      CSP `frame-ancestors` (same as /trending: 'self' https://nofxos.ai
//      https://www.nofxos.ai http://127.0.0.1:3000 http://localhost:3000)
//      AND drop the `X-Frame-Options: SAMEORIGIN` header on that path.
//   2. Replace this component with the same iframe pattern used by
//      DataPage.tsx (which already embeds vergex.trade/trending successfully).
export function StrategyMarketPage() {
  const { language } = useLanguage()

  const heading =
    language === 'zh'
      ? 'Vergex 策略市场'
      : language === 'id'
      ? 'Pasar Strategi Vergex'
      : 'Vergex Strategy Market'

  const description =
    language === 'zh'
      ? '在 Vergex 上探索由社区创建的交易策略,一键复制到您的 NOFX 账户。当前需要在新窗口打开。'
      : language === 'id'
      ? 'Jelajahi strategi trading komunitas di Vergex dan salin ke akun NOFX Anda. Saat ini terbuka di tab baru.'
      : 'Explore community-built trading strategies on Vergex and copy them to your NOFX account. Currently opens in a new tab.'

  const ctaLabel =
    language === 'zh'
      ? '在 Vergex 打开策略市场'
      : language === 'id'
      ? 'Buka Pasar Strategi di Vergex'
      : 'Open Strategy Market on Vergex'

  const subtitle =
    language === 'zh'
      ? 'POWERED BY VERGEX.TRADE'
      : language === 'id'
      ? 'DITENAGAI OLEH VERGEX.TRADE'
      : 'POWERED BY VERGEX.TRADE'

  const features =
    language === 'zh'
      ? [
          { label: '策略数量', value: '100+' },
          { label: '覆盖市场', value: 'CEX & DEX' },
          { label: '实时数据', value: '24/7' },
        ]
      : language === 'id'
      ? [
          { label: 'Total Strategi', value: '100+' },
          { label: 'Cakupan Pasar', value: 'CEX & DEX' },
          { label: 'Data Real-time', value: '24/7' },
        ]
      : [
          { label: 'Strategies', value: '100+' },
          { label: 'Markets', value: 'CEX & DEX' },
          { label: 'Live Data', value: '24/7' },
        ]

  return (
    <div className="relative flex h-[calc(100vh-64px)] w-full items-center justify-center overflow-hidden bg-nofx-bg px-6">
      {/* Ambient halos */}
      <div className="pointer-events-none absolute -left-20 top-1/4 h-96 w-96 rounded-full bg-nofx-gold/[0.06] blur-3xl" />
      <div className="pointer-events-none absolute -right-20 bottom-1/4 h-80 w-80 rounded-full bg-nofx-accent/[0.04] blur-3xl" />

      {/* Main card */}
      <div className="relative w-full max-w-xl">
        <div className="overflow-hidden rounded-2xl border border-white/[0.08] bg-zinc-950/60 shadow-2xl shadow-black/40 backdrop-blur-xl">
          {/* Top gold edge */}
          <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-nofx-gold/40 to-transparent" />

          <div className="p-8 sm:p-10 text-center">
            {/* Subtitle */}
            <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-nofx-gold/25 bg-nofx-gold/[0.06] px-3 py-1">
              <div className="h-1.5 w-1.5 animate-pulse rounded-full bg-nofx-gold" />
              <span className="text-[10.5px] font-mono uppercase tracking-[0.18em] text-nofx-gold">
                {subtitle}
              </span>
            </div>

            {/* Heading */}
            <h1 className="text-3xl sm:text-4xl font-bold tracking-tight text-white leading-tight">
              {heading}
            </h1>

            {/* Description */}
            <p className="mx-auto mt-4 max-w-md text-sm leading-relaxed text-zinc-400">
              {description}
            </p>

            {/* CTA */}
            <a
              href={VERGEX_EXPLORE_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="group mt-8 inline-flex items-center gap-2 rounded-lg bg-nofx-gold px-6 py-3 text-sm font-semibold text-black shadow-lg shadow-nofx-gold/20 transition-all hover:bg-yellow-400 active:scale-[0.98]"
            >
              {ctaLabel}
              <ExternalLink
                size={15}
                className="transition-transform group-hover:translate-x-0.5"
              />
            </a>

            {/* Stats row */}
            <div className="mt-10 grid grid-cols-3 gap-6 border-t border-white/[0.05] pt-8">
              {features.map((f) => (
                <div key={f.label}>
                  <div className="font-mono text-xl font-bold text-white">
                    {f.value}
                  </div>
                  <div className="mt-1 text-[10.5px] uppercase tracking-[0.14em] text-zinc-500">
                    {f.label}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Footer hint */}
        <p className="mt-4 text-center text-[11px] text-zinc-600">
          vergex.trade · {VERGEX_EXPLORE_URL}
        </p>
      </div>
    </div>
  )
}
