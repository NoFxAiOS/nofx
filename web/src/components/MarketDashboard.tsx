import { useState, useEffect, useCallback } from 'react';
import useSWR from 'swr';
import { api } from '../lib/api';
import { KlineChart } from './KlineChart';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';
import type { KlineData } from '../types';

const DEFAULT_COINS = ['BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT', 'XRPUSDT', 'DOGEUSDT'];

type Interval = '3m' | '4h';

interface CoinCardProps {
  symbol: string;
  interval: Interval;
  expanded: boolean;
  onToggleExpand: () => void;
  onRemove: () => void;
}

function CoinCard({ symbol, interval, expanded, onToggleExpand, onRemove }: CoinCardProps) {
  const { language } = useLanguage();

  const { data: klines } = useSWR<KlineData[]>(
    `klines-${symbol}-${interval}`,
    () => api.getKlines(symbol, interval, 100),
    { refreshInterval: interval === '3m' ? 15000 : 60000, revalidateOnFocus: false }
  );

  const lastKline = klines && klines.length > 0 ? klines[klines.length - 1] : null;
  const prevKline = klines && klines.length > 1 ? klines[klines.length - 2] : null;
  const priceChange = lastKline && prevKline
    ? ((lastKline.close - prevKline.close) / prevKline.close) * 100
    : 0;
  const isPositive = priceChange >= 0;

  return (
    <div
      className={`binance-card overflow-hidden transition-all duration-300 ${expanded ? 'col-span-2' : ''}`}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: '1px solid #2B3139' }}>
        <div className="flex items-center gap-3">
          <span className="font-bold font-mono text-base" style={{ color: '#EAECEF' }}>
            {symbol.replace('USDT', '')}
          </span>
          <span className="text-xs px-1.5 py-0.5 rounded" style={{ background: '#1E2329', color: '#848E9C' }}>
            /USDT
          </span>
        </div>
        <div className="flex items-center gap-3">
          {lastKline && (
            <div className="flex items-center gap-3 text-sm">
              <span className="font-mono font-bold" style={{ color: '#EAECEF' }}>
                {lastKline.close.toFixed(lastKline.close >= 100 ? 2 : lastKline.close >= 1 ? 4 : 6)}
              </span>
              <span
                className="font-mono font-bold px-2 py-0.5 rounded text-xs"
                style={{
                  color: isPositive ? '#0ECB81' : '#F6465D',
                  background: isPositive ? 'rgba(14, 203, 129, 0.1)' : 'rgba(246, 70, 93, 0.1)',
                }}
              >
                {isPositive ? '+' : ''}{priceChange.toFixed(2)}%
              </span>
            </div>
          )}
          <button
            onClick={onToggleExpand}
            className="text-xs px-2 py-1 rounded transition-colors"
            style={{ background: '#1E2329', color: '#848E9C', border: '1px solid #2B3139' }}
          >
            {expanded ? t('collapseChart', language) : t('expandChart', language)}
          </button>
          <button
            onClick={onRemove}
            className="text-xs px-2 py-1 rounded transition-colors hover:bg-red-900/30"
            style={{ color: '#F6465D', border: '1px solid rgba(246, 70, 93, 0.2)' }}
          >
            ×
          </button>
        </div>
      </div>

      {/* Price details bar */}
      {lastKline && (
        <div className="flex items-center gap-4 px-4 py-2 text-xs" style={{ background: '#181A20', color: '#848E9C' }}>
          <span>{t('open', language)}: <span className="font-mono" style={{ color: '#EAECEF' }}>{lastKline.open.toFixed(lastKline.open >= 100 ? 2 : 4)}</span></span>
          <span>{t('high', language)}: <span className="font-mono" style={{ color: '#0ECB81' }}>{lastKline.high.toFixed(lastKline.high >= 100 ? 2 : 4)}</span></span>
          <span>{t('low', language)}: <span className="font-mono" style={{ color: '#F6465D' }}>{lastKline.low.toFixed(lastKline.low >= 100 ? 2 : 4)}</span></span>
          <span>{t('vol', language)}: <span className="font-mono" style={{ color: '#EAECEF' }}>{formatVolume(lastKline.volume)}</span></span>
        </div>
      )}

      {/* Chart */}
      <div className="px-1">
        {klines && klines.length > 0 ? (
          <KlineChart klines={klines} symbol={symbol} height={expanded ? 480 : 280} />
        ) : (
          <div className="flex items-center justify-center" style={{ height: expanded ? 480 : 280 }}>
            <div className="text-center" style={{ color: '#848E9C' }}>
              <div className="text-3xl mb-2 opacity-30">📊</div>
              <div className="text-sm">{t('loading', language)}</div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function formatVolume(vol: number): string {
  if (vol >= 1e9) return (vol / 1e9).toFixed(2) + 'B';
  if (vol >= 1e6) return (vol / 1e6).toFixed(2) + 'M';
  if (vol >= 1e3) return (vol / 1e3).toFixed(2) + 'K';
  return vol.toFixed(2);
}

export function MarketDashboard() {
  const { language } = useLanguage();
  const [interval, setInterval_] = useState<Interval>('3m');
  const [selectedCoins, setSelectedCoins] = useState<string[]>(DEFAULT_COINS);
  const [expandedCoin, setExpandedCoin] = useState<string | null>(null);
  const [showCoinPicker, setShowCoinPicker] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  // Fetch available symbols
  const { data: symbolsData } = useSWR('market-symbols', () => api.getMarketSymbols(), {
    revalidateOnFocus: false,
    dedupingInterval: 60000,
  });

  const availableSymbols = symbolsData?.symbols || [];

  const filteredSymbols = availableSymbols.filter(
    (s) =>
      !selectedCoins.includes(s) &&
      s.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const addCoin = useCallback((symbol: string) => {
    setSelectedCoins((prev) => [...prev, symbol]);
    setSearchQuery('');
  }, []);

  const removeCoin = useCallback((symbol: string) => {
    setSelectedCoins((prev) => prev.filter((s) => s !== symbol));
    if (expandedCoin === symbol) setExpandedCoin(null);
  }, [expandedCoin]);

  const toggleExpand = useCallback((symbol: string) => {
    setExpandedCoin((prev) => (prev === symbol ? null : symbol));
  }, []);

  // Close coin picker on outside click
  useEffect(() => {
    if (!showCoinPicker) return;
    const handler = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (!target.closest('.coin-picker-container')) {
        setShowCoinPicker(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showCoinPicker]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold" style={{ color: '#EAECEF' }}>
            {t('marketDashboard', language)}
          </h2>
          <p className="text-sm mt-1" style={{ color: '#848E9C' }}>
            {selectedCoins.length} {t('symbol', language)} · {interval === '3m' ? t('interval3m', language) : t('interval4h', language)} {t('klineChart', language)}
          </p>
        </div>

        <div className="flex items-center gap-3">
          {/* Interval toggle */}
          <div className="flex gap-1 rounded p-1" style={{ background: '#1E2329' }}>
            <button
              onClick={() => setInterval_('3m')}
              className="px-3 py-1.5 rounded text-xs font-semibold transition-all"
              style={interval === '3m'
                ? { background: '#F0B90B', color: '#000' }
                : { background: 'transparent', color: '#848E9C' }
              }
            >
              {t('interval3m', language)}
            </button>
            <button
              onClick={() => setInterval_('4h')}
              className="px-3 py-1.5 rounded text-xs font-semibold transition-all"
              style={interval === '4h'
                ? { background: '#F0B90B', color: '#000' }
                : { background: 'transparent', color: '#848E9C' }
              }
            >
              {t('interval4h', language)}
            </button>
          </div>

          {/* Add coin button */}
          <div className="relative coin-picker-container">
            <button
              onClick={() => setShowCoinPicker(!showCoinPicker)}
              className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
              style={{ background: 'rgba(240, 185, 11, 0.1)', color: '#F0B90B', border: '1px solid rgba(240, 185, 11, 0.2)' }}
            >
              + {t('addCoin', language)}
            </button>

            {/* Coin picker dropdown */}
            {showCoinPicker && (
              <div
                className="absolute right-0 top-full mt-2 w-64 rounded-lg shadow-xl z-50 overflow-hidden"
                style={{ background: '#1E2329', border: '1px solid #2B3139' }}
              >
                <div className="p-3" style={{ borderBottom: '1px solid #2B3139' }}>
                  <input
                    type="text"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    placeholder={t('selectCoins', language)}
                    className="w-full px-3 py-2 rounded text-sm"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF', outline: 'none' }}
                    autoFocus
                  />
                </div>
                <div className="max-h-60 overflow-y-auto">
                  {filteredSymbols.slice(0, 50).map((sym) => (
                    <button
                      key={sym}
                      onClick={() => {
                        addCoin(sym);
                        setShowCoinPicker(false);
                      }}
                      className="w-full text-left px-4 py-2 text-sm font-mono transition-colors hover:bg-gray-700/30"
                      style={{ color: '#EAECEF' }}
                    >
                      {sym.replace('USDT', '')}
                      <span style={{ color: '#848E9C' }}>/USDT</span>
                    </button>
                  ))}
                  {filteredSymbols.length === 0 && (
                    <div className="px-4 py-3 text-sm text-center" style={{ color: '#848E9C' }}>
                      {availableSymbols.length === 0 ? t('noMarketData', language) : 'No matches'}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Chart Grid */}
      {selectedCoins.length > 0 ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {selectedCoins.map((symbol) => (
            <CoinCard
              key={`${symbol}-${interval}`}
              symbol={symbol}
              interval={interval}
              expanded={expandedCoin === symbol}
              onToggleExpand={() => toggleExpand(symbol)}
              onRemove={() => removeCoin(symbol)}
            />
          ))}
        </div>
      ) : (
        <div className="binance-card p-16 text-center">
          <div className="text-6xl mb-4 opacity-30">📈</div>
          <div className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
            {t('noMarketData', language)}
          </div>
          <div className="text-sm" style={{ color: '#848E9C' }}>
            {t('marketDataWillAppear', language)}
          </div>
        </div>
      )}
    </div>
  );
}
