import { cn } from '../../lib/cn'
import { Input } from '../ui'
import { useEffect, useMemo, useState } from 'react'
import {
  AlertTriangle,
  CandlestickChart,
  Clock,
  RefreshCw,
  Search,
  X,
} from 'lucide-react'
import type {
  StockConfig,
  USStockCyclePhase,
  USStockScanReport,
  USStockScanResult,
  USStockSymbol,
} from '../../types'
import { stockConfig as tx, ts } from '../../i18n/strategy-translations'
import { api } from '../../lib/api'

interface StockConfigEditorProps {
  config: StockConfig
  onChange: (config: StockConfig) => void
  disabled?: boolean
  language: string
}

// How many long setups the scan feeds to the AI when scan_top_n is unset/0
// (mirrors the Go resolver in store/strategy_stock.go).
const DEFAULT_SCAN_TOP_N = 10

// Scan-table phase colours: strong green = ready long setups, light green =
// bases building, grey = no clear trade, red = short/exhaustion phases.
const PHASE_TEXT: Partial<Record<USStockCyclePhase, string>> = {
  wedge_pop: 'text-up',
  ema_crossback: 'text-up',
  base_n_break: 'text-up',
  base_forming: 'text-up/70',
  reversal_extension: 'text-up/70',
  uptrend: 'text-fg-3',
  neutral: 'text-fg-3',
  insufficient_data: 'text-fg-3',
  exhaustion_extension: 'text-down',
  wedge_drop: 'text-down',
  downtrend: 'text-down',
}

const PHASE_LABEL: Partial<Record<USStockCyclePhase, keyof typeof tx>> = {
  reversal_extension: 'phaseReversalExtension',
  wedge_pop: 'phaseWedgePop',
  ema_crossback: 'phaseEmaCrossback',
  base_n_break: 'phaseBaseNBreak',
  base_forming: 'phaseBaseForming',
  uptrend: 'phaseUptrend',
  exhaustion_extension: 'phaseExhaustionExtension',
  wedge_drop: 'phaseWedgeDrop',
  downtrend: 'phaseDowntrend',
  neutral: 'phaseNeutral',
  insufficient_data: 'phaseInsufficientData',
}

// "+12.3%" / "-3.4%"; em dash when the field is absent
const fmtPct = (v: number | undefined) =>
  v === undefined ? '—' : `${v > 0 ? '+' : ''}${v.toFixed(1)}%`
// "1.8x"; em dash when absent
const fmtVolRatio = (v: number | undefined) =>
  v === undefined ? '—' : `${v.toFixed(1)}x`

// Momentum-rank eligible: full 12-1 window and above the 200-day SMA
// (mirrors the Go TopMomentum filter).
const momentumEligible = (r: USStockScanResult) =>
  !!r.momentum?.valid && r.momentum.above_sma200

// Defaults shown as placeholders (mirror the Go resolvers in
// store/strategy_stock.go; the backend stays the source of truth).
const DEFAULT_DIVERGENCE_PCT = 1.0
const DEFAULT_MAX_POSITION_PCT = 20
const DEFAULT_MAX_TOTAL_EXPOSURE_PCT = 80
const DEFAULT_MAX_POSITIONS = 5
const DEFAULT_RISK_PER_TRADE_PCT = 1.0
const STOP_ATR_DEFAULTS = {
  swing: { min: 1.5, max: 3.0 },
  position: { min: 2.0, max: 4.0 },
}

// Default config for a newly created us_stock strategy: no symbols yet,
// paper trading ON, regular session only. Values are explicit (not omitted)
// because the backend merges PUT bodies onto the stored config, so an omitted
// field would not override a previously stored value.
export const defaultStockConfig: StockConfig = {
  symbols: [],
  preset: 'swing',
  sessions: { regular: true, pre_market: false, after_hours: false },
  data_fallback_yahoo: true,
  max_divergence_pct: 0,
  max_position_pct: 0,
  max_total_exposure_pct: 0,
  max_positions: 0,
  risk_per_trade_pct: 0,
  stop_atr_min: 0,
  stop_atr_max: 0,
  paper_trading: true,
}

// "aapl, NVDABUSDT nvda" -> ['AAPL', 'NVDABUSDT', 'NVDA'] (deduped, uppercased)
function parseSymbolText(text: string): string[] {
  const out: string[] = []
  for (const raw of text.split(/[\s,，;；]+/)) {
    const sym = raw.trim().toUpperCase()
    if (sym && !out.includes(sym)) out.push(sym)
  }
  return out
}

export function StockConfigEditor({
  config,
  onChange,
  disabled,
  language,
}: StockConfigEditorProps) {
  const updateField = <K extends keyof StockConfig>(
    key: K,
    value: StockConfig[K]
  ) => {
    if (!disabled) {
      onChange({ ...config, [key]: value })
    }
  }

  // ---- symbol universe (GET /api/usstock/symbols) -------------------------
  const [universe, setUniverse] = useState<USStockSymbol[] | null>(null)
  const [universeState, setUniverseState] = useState<
    'loading' | 'ready' | 'failed'
  >('loading')
  const [query, setQuery] = useState('')
  const [manualText, setManualText] = useState(() =>
    (config.symbols || []).join(', ')
  )
  const [excludeText, setExcludeText] = useState(() =>
    (config.scan_exclude || []).join(', ')
  )

  // Scan preview report (fetched only while scan mode is on; the fetch is
  // read-only, so it still runs when the editor is disabled).
  const [scanReport, setScanReport] = useState<USStockScanReport | null>(null)
  const [scanState, setScanState] = useState<
    'idle' | 'loading' | 'ready' | 'failed'
  >('idle')
  const [scanTick, setScanTick] = useState(0)

  useEffect(() => {
    let cancelled = false
    api
      .getUSStockSymbols()
      .then((list) => {
        if (cancelled) return
        if (list.length > 0) {
          setUniverse(list)
          setUniverseState('ready')
        } else {
          setUniverseState('failed')
        }
      })
      .catch(() => {
        if (!cancelled) setUniverseState('failed')
      })
    return () => {
      cancelled = true
    }
  }, [])

  const symbols = config.symbols || []
  const selected = useMemo(() => new Set(symbols), [symbols])

  const symbolSource =
    config.symbol_source === 'scan' || config.symbol_source === 'hybrid'
      ? config.symbol_source
      : 'manual'
  const usesScan = symbolSource === 'scan' || symbolSource === 'hybrid'
  const usesManual = symbolSource === 'manual' || symbolSource === 'hybrid'
  const scanExclude = config.scan_exclude || []
  const excludeSet = useMemo(
    () => new Set(config.scan_exclude || []),
    [config.scan_exclude]
  )
  const scanTopN =
    config.scan_top_n && config.scan_top_n > 0
      ? config.scan_top_n
      : DEFAULT_SCAN_TOP_N
  // Candidate ranking; omitted = kell (price-cycle score)
  const rankMode = config.scan_rank === 'momentum' ? 'momentum' : 'kell'

  useEffect(() => {
    if (!usesScan) return
    let cancelled = false
    setScanState('loading')
    api
      .getUSStockScan()
      .then((report) => {
        if (cancelled) return
        setScanReport(report)
        setScanState('ready')
      })
      .catch(() => {
        if (!cancelled) setScanState('failed')
      })
    return () => {
      cancelled = true
    }
  }, [usesScan, scanTick])

  // The first scan after a backend restart takes a minute or two: re-poll
  // while the backend reports it is still scanning.
  useEffect(() => {
    if (!usesScan || !scanReport?.scanning) return
    const id = setTimeout(() => setScanTick((t) => t + 1), 15000)
    return () => clearTimeout(id)
  }, [usesScan, scanReport])

  // Preview rows: first max(topN, 20) of the ranking, each annotated with the
  // feed marker. Kell ranking feeds the top N long setups; momentum ranking
  // feeds the top N momentum-eligible rows (valid 12-1 momentum, above the
  // 200-day SMA). In both modes excluded/manual pairs never consume a slot and
  // hybrid always feeds the manual symbols on top of those N.
  const scanRows = useMemo(() => {
    const results = usesScan && scanReport ? scanReport.results || [] : []
    const manualSet = symbolSource === 'hybrid' ? selected : null

    // Momentum ranking reorders the preview: eligible rows first, by
    // mom12_1_pct → rs63_pct → symbol, then the rest in report order. Kell
    // keeps the backend order as-is.
    let ordered = results
    if (rankMode === 'momentum') {
      const eligible = results.filter(momentumEligible)
      eligible.sort(
        (a, b) =>
          (b.momentum?.mom12_1_pct ?? 0) - (a.momentum?.mom12_1_pct ?? 0) ||
          (b.cycle.rs63_pct ?? 0) - (a.cycle.rs63_pct ?? 0) ||
          a.symbol.localeCompare(b.symbol)
      )
      const picked = new Set(eligible.map((r) => r.symbol))
      ordered = [...eligible, ...results.filter((r) => !picked.has(r.symbol))]
    }

    let fedCount = 0
    const fed = new Set<string>()
    for (const r of ordered) {
      const candidate =
        rankMode === 'momentum' ? momentumEligible(r) : r.cycle.long_setup
      if (!candidate || excludeSet.has(r.symbol)) continue
      if (manualSet?.has(r.symbol)) continue
      fedCount += 1
      if (fedCount <= scanTopN) fed.add(r.symbol)
    }
    return ordered.slice(0, Math.max(scanTopN, 20)).map((r, i) => ({
      ...r,
      rank: i + 1,
      fed: fed.has(r.symbol) || !!manualSet?.has(r.symbol),
      manual: !!manualSet?.has(r.symbol),
      excluded: excludeSet.has(r.symbol),
      phaseKey: PHASE_LABEL[r.cycle.phase],
      phaseCls: PHASE_TEXT[r.cycle.phase] ?? 'text-fg-3',
    }))
  }, [
    usesScan,
    rankMode,
    symbolSource,
    selected,
    scanReport,
    scanTopN,
    excludeSet,
  ])

  const filtered = useMemo(() => {
    if (!universe) return []
    const q = query.trim().toUpperCase()
    if (!q) return universe
    return universe.filter(
      (s) =>
        s.symbol.toUpperCase().includes(q) ||
        s.underlying.toUpperCase().includes(q)
    )
  }, [universe, query])

  const toggleSymbol = (symbol: string) => {
    if (disabled) return
    updateField(
      'symbols',
      selected.has(symbol)
        ? symbols.filter((s) => s !== symbol)
        : [...symbols, symbol]
    )
  }

  const removeExclude = (symbol: string) => {
    const next = scanExclude.filter((s) => s !== symbol)
    setExcludeText(next.join(', '))
    updateField('scan_exclude', next.length > 0 ? next : undefined)
  }

  const underlyingOf = (symbol: string) =>
    universe?.find((s) => s.symbol === symbol)?.underlying ||
    symbol.replace(/BUSDT$/, '')

  // ---- helpers -------------------------------------------------------------
  const sessions = config.sessions || {}
  const regularOn = sessions.regular !== false
  const updateSession = (patch: Partial<StockConfig['sessions']>) =>
    updateField('sessions', {
      regular: regularOn,
      pre_market: !!sessions.pre_market,
      after_hours: !!sessions.after_hours,
      ...patch,
    })
  const noSession = !regularOn && !sessions.pre_market && !sessions.after_hours

  const preset = config.preset === 'position' ? 'position' : 'swing'
  const atrDefaults = STOP_ATR_DEFAULTS[preset]
  const isPaper = config.paper_trading !== false

  // Numeric input: empty -> 0 ("use default"), otherwise the parsed number.
  const numField = (
    key:
      | 'max_divergence_pct'
      | 'max_position_pct'
      | 'max_total_exposure_pct'
      | 'max_positions'
      | 'risk_per_trade_pct'
      | 'stop_atr_min'
      | 'stop_atr_max',
    label: string,
    defaultValue: number,
    opts: {
      step?: number
      min?: number
      max?: number
      integer?: boolean
      desc?: string
    }
  ) => {
    const value = config[key]
    return (
      <div className="p-3 rounded-lg bg-surface-2 border border-line">
        <label className="block text-[13px] mb-1 text-fg-2">{label}</label>
        {opts.desc && <p className="text-xs mb-2 text-fg-3">{opts.desc}</p>}
        <Input
          type="number"
          data-testid={`stock-${key}`}
          value={value ? value : ''}
          placeholder={`${ts(tx.defaultHint, language)} ${defaultValue}`}
          onChange={(e) => {
            const raw = e.target.value
            if (raw === '' || raw === '-') {
              updateField(key, 0)
              return
            }
            const n = opts.integer ? parseInt(raw, 10) : parseFloat(raw)
            updateField(key, Number.isFinite(n) ? n : 0)
          }}
          disabled={disabled}
          min={opts.min}
          max={opts.max}
          step={opts.step ?? 1}
          className="w-full px-3 bg-surface-2 border border-line text-fg num text-right"
        />
      </div>
    )
  }

  return (
    <div className="space-y-4" data-testid="stock-config-editor">
      {/* Symbols */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <CandlestickChart className="w-5 h-5 text-brand" />
          <h3 className="font-medium text-fg">
            {ts(tx.symbolsSection, language)}
          </h3>
        </div>

        {/* Symbol source: manual list / auto scan / both */}
        <p className="text-[13px] mb-2 text-fg-2">
          {ts(tx.symbolSource, language)}
        </p>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
          {(
            [
              ['manual', tx.sourceManual],
              ['scan', tx.sourceScan],
              ['hybrid', tx.sourceHybrid],
            ] as const
          ).map(([value, label]) => (
            <label
              key={value}
              data-testid={`stock-source-${value}`}
              className={cn(
                'p-3 rounded-lg cursor-pointer',
                'bg-surface-2',
                symbolSource === value
                  ? 'border border-brand'
                  : 'border border-line',
                'text-fg-2'
              )}
            >
              <div className="flex items-center gap-2">
                <input
                  type="radio"
                  name="stock-symbol-source"
                  checked={symbolSource === value}
                  disabled={disabled}
                  onChange={() => updateField('symbol_source', value)}
                  className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
                />
                <span className="text-sm font-medium text-fg">
                  {ts(label, language)}
                </span>
              </div>
            </label>
          ))}
        </div>
        <p className="text-xs mb-4 text-fg-3">
          {ts(
            symbolSource === 'manual'
              ? tx.sourceManualDesc
              : symbolSource === 'scan'
                ? tx.sourceScanDesc
                : tx.sourceHybridDesc,
            language
          )}
          {usesScan && ` ${ts(tx.sourceHeldNote, language)}`}
        </p>

        {symbolSource === 'scan' && symbols.length > 0 && (
          <p className="text-xs mb-4 text-fg-3">
            {ts(tx.symbolsIgnoredNote, language).replace(
              '{n}',
              String(symbols.length)
            )}
          </p>
        )}

        {usesScan && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
            <div className="p-3 rounded-lg bg-surface-2 border border-line">
              <label className="block text-[13px] mb-1 text-fg-2">
                {ts(tx.scanTopN, language)}
              </label>
              <p className="text-xs mb-2 text-fg-3">
                {ts(tx.scanTopNDesc, language)}
              </p>
              <Input
                type="number"
                data-testid="stock-scan-top-n"
                value={config.scan_top_n ?? ''}
                placeholder="10"
                onChange={(e) => {
                  const n = parseInt(e.target.value, 10)
                  updateField(
                    'scan_top_n',
                    Number.isFinite(n) && n > 0 ? n : undefined
                  )
                }}
                disabled={disabled}
                min={1}
                max={50}
                step={1}
                className="w-full px-3 bg-surface-2 border border-line text-fg num text-right"
              />
            </div>
            <div className="p-3 rounded-lg bg-surface-2 border border-line">
              <label className="block text-[13px] mb-1 text-fg-2">
                {ts(tx.scanExclude, language)}
              </label>
              <p className="text-xs mb-2 text-fg-3">
                {ts(tx.scanExcludeDesc, language)}
              </p>
              {scanExclude.length > 0 && (
                <div className="flex flex-wrap gap-2 mb-2">
                  {scanExclude.map((sym) => (
                    <span
                      key={sym}
                      data-testid="stock-scan-exclude-chip"
                      className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs bg-surface-2 border border-down text-fg-2"
                    >
                      {sym}
                      {!disabled && (
                        <button
                          type="button"
                          onClick={() => removeExclude(sym)}
                          aria-label={`remove ${sym}`}
                          className="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
                        >
                          <X className="w-3 h-3" />
                        </button>
                      )}
                    </span>
                  ))}
                </div>
              )}
              <Input
                type="text"
                data-testid="stock-scan-exclude-input"
                value={excludeText}
                onChange={(e) => {
                  setExcludeText(e.target.value)
                  const list = parseSymbolText(e.target.value)
                  updateField(
                    'scan_exclude',
                    list.length > 0 ? list : undefined
                  )
                }}
                placeholder={ts(tx.scanExcludePlaceholder, language)}
                disabled={disabled}
                className="w-full px-3 num bg-surface-2 border border-line text-fg"
              />
            </div>
          </div>
        )}

        {usesScan && (
          <div className="mb-4">
            <p className="text-[13px] mb-2 text-fg-2">
              {ts(tx.scanRank, language)}
            </p>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {(
                [
                  ['kell', tx.scanRankKell, tx.scanRankKellDesc],
                  ['momentum', tx.scanRankMomentum, tx.scanRankMomentumDesc],
                ] as const
              ).map(([value, label, desc]) => (
                <label
                  key={value}
                  data-testid={`stock-rank-${value}`}
                  className={cn(
                    'p-3 rounded-lg cursor-pointer',
                    'bg-surface-2',
                    rankMode === value
                      ? 'border border-brand'
                      : 'border border-line',
                    'text-fg-2'
                  )}
                >
                  <div className="flex items-center gap-2 mb-1">
                    <input
                      type="radio"
                      name="stock-scan-rank"
                      checked={rankMode === value}
                      disabled={disabled}
                      onChange={() =>
                        updateField(
                          'scan_rank',
                          value === 'momentum' ? 'momentum' : undefined
                        )
                      }
                      className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
                    />
                    <span className="text-sm font-medium text-fg">
                      {ts(label, language)}
                    </span>
                  </div>
                  <p className="text-xs text-fg-3">{ts(desc, language)}</p>
                </label>
              ))}
            </div>
          </div>
        )}

        {usesManual && (
          <div className="p-3 rounded-lg bg-surface-2 border border-line">
            <label className="block text-[13px] mb-1 text-fg-2">
              {ts(tx.symbols, language)}
            </label>
            <p className="text-xs mb-3 text-fg-3">
              {ts(
                symbolSource === 'hybrid'
                  ? tx.symbolsHybridDesc
                  : tx.symbolsDesc,
                language
              )}
            </p>

            {/* Selected chips */}
            {symbols.length > 0 && (
              <div className="flex flex-wrap gap-2 mb-3">
                {symbols.map((sym) => (
                  <span
                    key={sym}
                    data-testid="stock-selected-symbol"
                    className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs bg-surface-2 border border-brand"
                  >
                    {underlyingOf(sym)} · {sym}
                    {!disabled && (
                      <button
                        type="button"
                        onClick={() => toggleSymbol(sym)}
                        aria-label={`remove ${sym}`}
                        className="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
                      >
                        <X className="w-3 h-3" />
                      </button>
                    )}
                  </span>
                ))}
              </div>
            )}

            {universeState === 'loading' && (
              <p className="text-xs text-fg-3">
                {ts(tx.symbolsLoading, language)}
              </p>
            )}

            {universeState === 'ready' && (
              <div>
                <div className="relative mb-2">
                  <Search className="w-4 h-4 absolute left-2 top-1/2 -translate-y-1/2 text-fg-3" />
                  <Input
                    type="text"
                    data-testid="stock-symbol-search"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder={ts(tx.symbolSearch, language)}
                    disabled={disabled}
                    className="w-full pl-8 pr-3 bg-surface-2 border border-line text-fg"
                  />
                </div>
                <div className="max-h-56 overflow-y-auto rounded border border-line">
                  {filtered.length === 0 && (
                    <p className="p-3 text-xs text-fg-3">
                      {ts(tx.symbolsNoMatch, language)}
                    </p>
                  )}
                  {filtered.map((s) => {
                    const checked = selected.has(s.symbol)
                    return (
                      <label
                        key={s.symbol}
                        className="flex items-center gap-2 min-h-[34px] px-3 py-1.5 text-[13px] cursor-pointer hover:bg-surface-hover text-fg-2"
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          disabled={disabled}
                          onChange={() => toggleSymbol(s.symbol)}
                          className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
                        />
                        <span>
                          {s.underlying} · {s.symbol}
                        </span>
                      </label>
                    )
                  })}
                </div>
                <p className="text-xs mt-2 text-fg-3">
                  {ts(tx.symbolsSelected, language)} {symbols.length}
                </p>
              </div>
            )}

            {universeState === 'failed' && (
              <div>
                <p className="text-xs mb-2 text-fg-3">
                  {ts(tx.symbolsFallback, language)}
                </p>
                <Input
                  type="text"
                  data-testid="stock-symbol-manual"
                  value={manualText}
                  onChange={(e) => {
                    setManualText(e.target.value)
                    updateField('symbols', parseSymbolText(e.target.value))
                  }}
                  placeholder={ts(tx.symbolsFallbackPlaceholder, language)}
                  disabled={disabled}
                  className="w-full px-3 num bg-surface-2 border border-line text-fg"
                />
              </div>
            )}

            {symbols.length === 0 && (
              <p className="text-xs mt-2 flex items-center gap-1 text-down">
                <AlertTriangle className="w-3 h-3" />
                {ts(
                  symbolSource === 'hybrid'
                    ? tx.symbolsHybridEmptyWarn
                    : tx.symbolsEmptyWarn,
                  language
                )}
              </p>
            )}
          </div>
        )}

        {usesScan && (
          <div
            className="p-3 rounded-lg bg-surface-2 border border-line mt-4"
            data-testid="stock-scan-preview"
          >
            <div className="flex items-center justify-between gap-2 mb-2">
              <span className="text-[13px] text-fg-2">
                {ts(tx.scanPreview, language)}
              </span>
              <button
                type="button"
                data-testid="stock-scan-refresh"
                onClick={() => setScanTick((t) => t + 1)}
                className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs border border-line text-fg-2 hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
              >
                <RefreshCw
                  className={cn(
                    'w-3 h-3',
                    scanState === 'loading' && 'animate-spin'
                  )}
                />
                {ts(tx.scanRefresh, language)}
              </button>
            </div>

            {scanState === 'loading' && (
              <p className="text-xs text-fg-3">
                {ts(tx.scanScanning, language)}
              </p>
            )}
            {scanState === 'failed' && (
              <p className="text-xs text-down">{ts(tx.scanError, language)}</p>
            )}
            {scanState === 'ready' && scanReport && (
              <>
                <p className="text-xs mb-2 text-fg-3">
                  {ts(tx.scanGeneratedAt, language)}:{' '}
                  {scanReport.generated_at
                    ? new Date(scanReport.generated_at).toLocaleString()
                    : '—'}
                </p>
                {scanReport.scanning && (
                  <p className="text-xs mb-2 text-fg-3">
                    {ts(tx.scanScanning, language)}
                  </p>
                )}
                {scanRows.length > 0 ? (
                  <div className="overflow-x-auto">
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="text-left text-fg-3 border-b border-line">
                          <th className="py-1.5 pr-2 font-medium">
                            {ts(tx.scanColRank, language)}
                          </th>
                          <th className="py-1.5 pr-2 font-medium">
                            {ts(tx.scanColSymbol, language)}
                          </th>
                          <th className="py-1.5 pr-2 font-medium">
                            {ts(tx.scanColPhase, language)}
                          </th>
                          <th className="py-1.5 pr-2 font-medium text-right">
                            {ts(tx.scanColScore, language)}
                          </th>
                          <th className="py-1.5 pr-2 font-medium text-right">
                            {ts(tx.scanColRs63, language)}
                          </th>
                          <th className="py-1.5 pr-2 font-medium text-right">
                            {ts(tx.scanColMom, language)}
                          </th>
                          <th className="py-1.5 pr-2 font-medium text-right">
                            {ts(tx.scanColVol, language)}
                          </th>
                          <th className="py-1.5 pr-2 font-medium text-center">
                            {ts(tx.scanColFeed, language)}
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {scanRows.map((row) => (
                          <tr
                            key={row.symbol}
                            className={cn(
                              'border-b border-line last:border-0',
                              row.excluded && 'opacity-50'
                            )}
                          >
                            <td className="py-1.5 pr-2 num text-fg-3">
                              {row.rank}
                            </td>
                            <td className="py-1.5 pr-2 whitespace-nowrap text-fg-2">
                              {row.underlying} · {row.symbol}
                              {row.excluded && (
                                <span className="ml-1.5 px-1 py-0.5 rounded text-[10px] bg-down-soft text-down">
                                  {ts(tx.scanExcludedTag, language)}
                                </span>
                              )}
                              {row.manual && (
                                <span className="ml-1.5 px-1 py-0.5 rounded text-[10px] bg-brand-soft text-brand">
                                  {ts(tx.scanManualTag, language)}
                                </span>
                              )}
                            </td>
                            <td
                              className={cn(
                                'py-1.5 pr-2 whitespace-nowrap',
                                row.phaseCls
                              )}
                            >
                              {row.phaseKey
                                ? ts(tx[row.phaseKey], language)
                                : row.cycle.phase}
                            </td>
                            <td className="py-1.5 pr-2 num text-right text-fg-2">
                              {row.cycle.long_setup
                                ? row.cycle.score.toFixed(1)
                                : '—'}
                            </td>
                            <td className="py-1.5 pr-2 num text-right text-fg-2">
                              {fmtPct(row.cycle.rs63_pct)}
                            </td>
                            <td className="py-1.5 pr-2 num text-right text-fg-2 whitespace-nowrap">
                              {row.momentum?.valid
                                ? fmtPct(row.momentum.mom12_1_pct)
                                : '—'}
                              {row.momentum?.above_sma200 && (
                                <span className="ml-1 px-1 py-0.5 rounded text-[10px] border border-line text-fg-3">
                                  {ts(tx.scanAboveSma200, language)}
                                </span>
                              )}
                            </td>
                            <td className="py-1.5 pr-2 num text-right text-fg-2">
                              {fmtVolRatio(row.cycle.vol_ratio)}
                            </td>
                            <td
                              className="py-1.5 pr-2 text-center text-brand"
                              title={ts(tx.scanFeedTip, language)}
                            >
                              {row.fed ? '★' : ''}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  !scanReport.scanning && (
                    <p className="text-xs text-fg-3">
                      {ts(tx.scanNoResults, language)}
                    </p>
                  )
                )}
              </>
            )}
          </div>
        )}
      </div>

      {/* Holding style preset */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Clock className="w-5 h-5 text-up" />
          <h3 className="font-medium text-fg">
            {ts(tx.presetSection, language)}
          </h3>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {(
            [
              ['swing', tx.presetSwing, tx.presetSwingDesc],
              ['position', tx.presetPosition, tx.presetPositionDesc],
            ] as const
          ).map(([value, label, desc]) => (
            <label
              key={value}
              className={cn(
                'p-3 rounded-lg cursor-pointer',
                'bg-surface-2',
                'border border-line',
                preset === value ? 'border border-brand' : 'border border-line',
                'text-fg-2'
              )}
            >
              <div className="flex items-center gap-2 mb-1">
                <input
                  type="radio"
                  name="stock-preset"
                  data-testid={`stock-preset-${value}`}
                  checked={preset === value}
                  disabled={disabled}
                  onChange={() => updateField('preset', value)}
                  className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
                />
                <span className="text-sm font-medium text-fg">
                  {ts(label, language)}
                </span>
              </div>
              <p className="text-xs text-fg-3">{ts(desc, language)}</p>
            </label>
          ))}
        </div>
      </div>

      {/* Sessions */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Clock className="w-5 h-5 text-brand" />
          <h3 className="font-medium text-fg">
            {ts(tx.sessionsSection, language)}
          </h3>
        </div>
        <div className="p-3 rounded-lg space-y-2 bg-surface-2 border border-line">
          <label className="flex items-center gap-2 text-[13px] text-fg-2">
            <input
              type="checkbox"
              data-testid="stock-session-regular"
              checked={regularOn}
              disabled={disabled}
              onChange={(e) => updateSession({ regular: e.target.checked })}
              className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
            />
            {ts(tx.sessionRegular, language)}
          </label>
          <label className="flex items-center gap-2 text-[13px] text-fg-2">
            <input
              type="checkbox"
              data-testid="stock-session-pre"
              checked={!!sessions.pre_market}
              disabled={disabled}
              onChange={(e) => updateSession({ pre_market: e.target.checked })}
              className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
            />
            {ts(tx.sessionPreMarket, language)}
          </label>
          <label className="flex items-center gap-2 text-[13px] text-fg-2">
            <input
              type="checkbox"
              data-testid="stock-session-after"
              checked={!!sessions.after_hours}
              disabled={disabled}
              onChange={(e) => updateSession({ after_hours: e.target.checked })}
              className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
            />
            {ts(tx.sessionAfterHours, language)}
          </label>
          <p className="text-xs pt-1 text-fg-3">
            {ts(tx.sessionsNote, language)}
          </p>
          {noSession && (
            <p className="text-xs text-down">
              {ts(tx.sessionsNoneWarn, language)}
            </p>
          )}
        </div>
      </div>

      {/* Data */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <h3 className="font-medium text-fg">
            {ts(tx.dataSection, language)}
          </h3>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="p-3 rounded-lg bg-surface-2 border border-line">
            <label className="flex items-center gap-2 text-[13px] mb-1 text-fg-2">
              <input
                type="checkbox"
                data-testid="stock-yahoo-fallback"
                checked={config.data_fallback_yahoo !== false}
                disabled={disabled}
                onChange={(e) =>
                  updateField('data_fallback_yahoo', e.target.checked)
                }
                className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
              />
              {ts(tx.yahooFallback, language)}
            </label>
            <p className="text-xs text-fg-3">
              {ts(tx.yahooFallbackDesc, language)}
            </p>
          </div>
          {numField(
            'max_divergence_pct',
            ts(tx.maxDivergence, language),
            DEFAULT_DIVERGENCE_PCT,
            { step: 0.1, max: 10, desc: ts(tx.maxDivergenceDesc, language) }
          )}
        </div>
      </div>

      {/* Risk */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <h3 className="font-medium text-fg">
            {ts(tx.riskSection, language)}
          </h3>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {numField(
            'max_position_pct',
            ts(tx.maxPositionPct, language),
            DEFAULT_MAX_POSITION_PCT,
            { step: 1, min: 0, max: 100 }
          )}
          {numField(
            'max_total_exposure_pct',
            ts(tx.maxTotalExposurePct, language),
            DEFAULT_MAX_TOTAL_EXPOSURE_PCT,
            { step: 1, min: 0, max: 100 }
          )}
          {numField(
            'max_positions',
            ts(tx.maxPositions, language),
            DEFAULT_MAX_POSITIONS,
            { step: 1, min: 0, max: 20, integer: true }
          )}
          {numField(
            'risk_per_trade_pct',
            ts(tx.riskPerTradePct, language),
            DEFAULT_RISK_PER_TRADE_PCT,
            { step: 0.1, min: 0, max: 5 }
          )}
          {numField(
            'stop_atr_min',
            ts(tx.stopAtrMin, language),
            atrDefaults.min,
            { step: 0.1, min: 0, max: 10 }
          )}
          {numField(
            'stop_atr_max',
            ts(tx.stopAtrMax, language),
            atrDefaults.max,
            { step: 0.1, min: 0, max: 10 }
          )}
        </div>
      </div>

      {/* Paper trading */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <h3 className="font-medium text-fg">
            {ts(tx.paperSection, language)}
          </h3>
        </div>
        <div
          className={`p-3 rounded-lg border ${isPaper ? 'bg-surface-2 border-line' : 'bg-down-soft border-down'}`}
          data-testid="stock-paper-box"
        >
          <label className="flex items-center gap-2 text-[13px] font-medium text-fg-2">
            <input
              type="checkbox"
              data-testid="stock-paper-trading"
              checked={isPaper}
              disabled={disabled}
              onChange={(e) => updateField('paper_trading', e.target.checked)}
              className="accent-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/50 disabled:opacity-50"
            />
            {ts(tx.paperTrading, language)}
          </label>
          <p className="text-xs mt-1 text-fg-3">
            {ts(tx.paperTradingDesc, language)}
          </p>
          {!isPaper && (
            <p
              className="text-xs mt-2 flex items-center gap-1 font-medium text-down"
              data-testid="stock-live-warning"
            >
              <AlertTriangle className="w-3 h-3" />
              {ts(tx.liveWarning, language)}
            </p>
          )}
        </div>
      </div>
    </div>
  )
}
