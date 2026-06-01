import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

// Strategy Market — embedded vergex.trade/explore.
//
// vergex.trade now lists the NOFX origins in its enforced
// `Content-Security-Policy: frame-ancestors 'self' https://nofxos.ai
// https://www.nofxos.ai http://127.0.0.1:3000 http://localhost:3000` for the
// /explore path, so cross-origin embedding works. The X-Frame-Options header
// is still SAMEORIGIN, but modern browsers prioritize the CSP
// `frame-ancestors` directive when both are present (per CSP Level 2).
//
// Mirrors the DataPage.tsx pattern (vergex.trade/trending).
export function StrategyMarketPage() {
  const { language } = useLanguage()

  return (
    <div className="h-[calc(100vh-64px)] w-full">
      <iframe
        src="https://vergex.trade/explore"
        title={t('strategyMarket', language) || 'Strategy Market'}
        className="h-full w-full border-0"
        // Permission policy: keep minimal. `fullscreen` matches the existing
        // DataPage iframe; `clipboard-write` was previously listed but is
        // not needed by the embedded view and would let the iframe silently
        // overwrite the user's clipboard (classic clipboard-hijack pattern,
        // e.g. swap a copied wallet address). Drop it.
        allow="fullscreen"
        // Sandbox grants vergex.trade only what it actually needs to render
        // the explore page: run scripts, talk to its own origin / wallet
        // providers (allow-same-origin), submit search forms, open external
        // links in new tabs. Notably absent:
        //   - allow-top-navigation: prevents the iframe from navigating the
        //     parent NOFX shell to an arbitrary URL.
        //   - allow-modals / allow-pointer-lock / allow-orientation-lock:
        //     not needed for a strategy list view.
        //   - allow-storage-access-by-user-activation: keeps any third-party
        //     storage access prompts out of the embedded surface.
        sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox"
        referrerPolicy="strict-origin-when-cross-origin"
      />
    </div>
  )
}
