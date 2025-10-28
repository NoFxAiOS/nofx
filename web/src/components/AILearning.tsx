import useSWR from 'swr';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';

interface TradeOutcome {
  symbol: string;
  side: string;
  open_price: number;
  close_price: number;
  pn_l: number;
  pn_l_pct: number;
  duration: string;
  open_time: string;
  close_time: string;
  was_stop_loss: boolean;
}

interface SymbolPerformance {
  symbol: string;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  total_pn_l: number;
  avg_pn_l: number;
}

interface PerformanceAnalysis {
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  avg_win: number;
  avg_loss: number;
  profit_factor: number;
  sharpe_ratio: number; // 夏普比率（风险调整后收益）
  recent_trades: TradeOutcome[];
  symbol_stats: { [key: string]: SymbolPerformance };
  best_symbol: string;
  worst_symbol: string;
}

interface AILearningProps {
  traderId: string;
}

interface DecisionRecord {
  timestamp: string;
  cycle_number: number;
  input_prompt: string;
  cot_trace: string;
  success: boolean;
}

const fetcher = (url: string) => fetch(url).then(res => res.json());

export default function AILearning({ traderId }: AILearningProps) {
  const { language } = useLanguage();
  const { data: performance, error } = useSWR<PerformanceAnalysis>(
    `http://localhost:8080/api/performance?trader_id=${traderId}`,
    fetcher,
    { refreshInterval: 10000 }
  );

  // 获取最新的决策记录，查看AI的思考过程
  const { data: latestDecisions } = useSWR<DecisionRecord[]>(
    `http://localhost:8080/api/decisions/latest?trader_id=${traderId}`,
    fetcher,
    { refreshInterval: 10000 }
  );

  if (error) {
    return (
      <div className="rounded p-6" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
        <div style={{ color: '#F6465D' }}>{t('loadingError', language)}</div>
      </div>
    );
  }

  if (!performance) {
    return (
      <div className="rounded p-6" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
        <div style={{ color: '#848E9C' }}>📊 {t('loading', language)}</div>
      </div>
    );
  }

  if (!performance || performance.total_trades === 0) {
    return (
      <div className="rounded p-6" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
        <div className="flex items-center gap-2 mb-2">
          <span className="text-xl">🧠</span>
          <h2 className="text-lg font-bold" style={{ color: '#EAECEF' }}>{t('aiLearning', language)}</h2>
        </div>
        <div style={{ color: '#848E9C' }}>
          {t('noCompleteData', language)}
        </div>
      </div>
    );
  }

  // 安全地获取symbol_stats
  const symbolStats = performance.symbol_stats || {};
  const symbolStatsList = Object.values(symbolStats).filter(stat => stat != null).sort(
    (a, b) => (b.total_pn_l || 0) - (a.total_pn_l || 0)
  );

  return (
    <div className="space-y-6">
      {/* 标题区 - 更简洁 */}
      <div className="flex items-center gap-4">
        <div className="w-12 h-12 rounded-xl flex items-center justify-center text-2xl" style={{
          background: 'linear-gradient(135deg, #8B5CF6 0%, #6366F1 100%)',
          boxShadow: '0 4px 14px rgba(139, 92, 246, 0.4)'
        }}>
          🧠
        </div>
        <div>
          <h2 className="text-2xl font-bold" style={{ color: '#EAECEF' }}>{t('aiLearning', language)}</h2>
          <p className="text-sm" style={{ color: '#848E9C' }}>
            {t('tradesAnalyzed', language, { count: performance.total_trades })}
          </p>
        </div>
      </div>

      {/* 主要内容：现代化网格布局 */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">

        {/* 左侧大区域：核心指标 (5列) */}
        <div className="lg:col-span-5 space-y-6">
          {/* 核心指标网格 - 玻璃态设计 */}
          <div className="grid grid-cols-2 gap-4">
            {/* 总交易数 */}
            <div className="rounded-xl p-4 backdrop-blur-sm" style={{
              background: 'rgba(30, 35, 41, 0.6)',
              border: '1px solid rgba(99, 102, 241, 0.2)',
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.1)'
            }}>
              <div className="text-xs font-semibold mb-2" style={{ color: '#94A3B8' }}>{t('totalTrades', language)}</div>
              <div className="text-3xl font-bold mono" style={{ color: '#E0E7FF' }}>
                {performance.total_trades}
              </div>
            </div>

            {/* 胜率 */}
            <div className="rounded-xl p-4 backdrop-blur-sm" style={{
              background: (performance.win_rate || 0) >= 50
                ? 'rgba(14, 203, 129, 0.1)'
                : 'rgba(246, 70, 93, 0.1)',
              border: `1px solid ${(performance.win_rate || 0) >= 50 ? 'rgba(14, 203, 129, 0.3)' : 'rgba(246, 70, 93, 0.3)'}`,
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.1)'
            }}>
              <div className="text-xs font-semibold mb-2" style={{ color: '#94A3B8' }}>{t('winRate', language)}</div>
              <div className="text-3xl font-bold mono" style={{
                color: (performance.win_rate || 0) >= 50 ? '#10B981' : '#F87171'
              }}>
                {(performance.win_rate || 0).toFixed(1)}%
              </div>
              <div className="text-xs mt-1" style={{ color: '#94A3B8' }}>
                {performance.winning_trades || 0}W / {performance.losing_trades || 0}L
              </div>
            </div>

            {/* 平均盈利 */}
            <div className="rounded-xl p-4 backdrop-blur-sm" style={{
              background: 'rgba(14, 203, 129, 0.08)',
              border: '1px solid rgba(14, 203, 129, 0.2)',
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.1)'
            }}>
              <div className="text-xs font-semibold mb-2" style={{ color: '#94A3B8' }}>{t('avgWin', language)}</div>
              <div className="text-3xl font-bold mono" style={{ color: '#10B981' }}>
                +{(performance.avg_win || 0).toFixed(2)}%
              </div>
            </div>

            {/* 平均亏损 */}
            <div className="rounded-xl p-4 backdrop-blur-sm" style={{
              background: 'rgba(246, 70, 93, 0.08)',
              border: '1px solid rgba(246, 70, 93, 0.2)',
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.1)'
            }}>
              <div className="text-xs font-semibold mb-2" style={{ color: '#94A3B8' }}>{t('avgLoss', language)}</div>
              <div className="text-3xl font-bold mono" style={{ color: '#F87171' }}>
                {(performance.avg_loss || 0).toFixed(2)}%
              </div>
            </div>
          </div>

          {/* 夏普比率 - AI自我进化核心指标 */}
          <div className="rounded-2xl p-6 relative overflow-hidden" style={{
            background: 'linear-gradient(135deg, rgba(139, 92, 246, 0.2) 0%, rgba(99, 102, 241, 0.1) 100%)',
            border: '2px solid rgba(139, 92, 246, 0.4)',
            boxShadow: '0 10px 30px rgba(139, 92, 246, 0.3)'
          }}>
            <div className="absolute top-0 right-0 w-40 h-40 rounded-full opacity-20" style={{
              background: 'radial-gradient(circle, #8B5CF6 0%, transparent 70%)',
              filter: 'blur(30px)'
            }} />

            <div className="relative">
              <div className="flex items-center gap-2 mb-3">
                <span className="text-2xl">🧬</span>
                <div>
                  <div className="text-sm font-bold" style={{ color: '#C4B5FD' }}>夏普比率</div>
                  <div className="text-xs" style={{ color: '#94A3B8' }}>风险调整后收益 · AI自我进化指标</div>
                </div>
              </div>

              <div className="flex items-end justify-between">
                <div className="text-6xl font-bold mono" style={{
                  color: (performance.sharpe_ratio || 0) >= 2 ? '#10B981' :
                         (performance.sharpe_ratio || 0) >= 1 ? '#22D3EE' :
                         (performance.sharpe_ratio || 0) >= 0 ? '#F0B90B' : '#F87171'
                }}>
                  {performance.sharpe_ratio ? performance.sharpe_ratio.toFixed(2) : 'N/A'}
                </div>

                {performance.sharpe_ratio !== undefined && (
                  <div className="text-right mb-2">
                    <div className="text-xs font-bold mb-1" style={{
                      color: (performance.sharpe_ratio || 0) >= 2 ? '#10B981' :
                             (performance.sharpe_ratio || 0) >= 1 ? '#22D3EE' :
                             (performance.sharpe_ratio || 0) >= 0 ? '#F0B90B' : '#F87171'
                    }}>
                      {performance.sharpe_ratio >= 2 ? '🟢 卓越表现' :
                       performance.sharpe_ratio >= 1 ? '🟢 良好表现' :
                       performance.sharpe_ratio >= 0 ? '🟡 波动较大' : '🔴 需要调整'}
                    </div>
                  </div>
                )}
              </div>

              {performance.sharpe_ratio !== undefined && (
                <div className="mt-4 p-3 rounded-xl" style={{
                  background: 'rgba(0, 0, 0, 0.3)',
                  border: '1px solid rgba(139, 92, 246, 0.2)'
                }}>
                  <div className="text-xs leading-relaxed" style={{ color: '#C4B5FD' }}>
                    {performance.sharpe_ratio >= 2 && '✨ AI策略非常有效！风险调整后收益优异，可适度扩大仓位但保持纪律。'}
                    {performance.sharpe_ratio >= 1 && performance.sharpe_ratio < 2 && '✅ 策略表现稳健，风险收益平衡良好，继续保持当前策略。'}
                    {performance.sharpe_ratio >= 0 && performance.sharpe_ratio < 1 && '⚠️ 收益为正但波动较大，AI正在优化策略，降低风险。'}
                    {performance.sharpe_ratio < 0 && '🚨 当前策略需要调整！AI已自动进入保守模式，减少仓位和交易频率。'}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* 盈亏比 - 突出显示 */}
          <div className="rounded-2xl p-6 relative overflow-hidden" style={{
            background: 'linear-gradient(135deg, rgba(240, 185, 11, 0.15) 0%, rgba(252, 213, 53, 0.05) 100%)',
            border: '1px solid rgba(240, 185, 11, 0.3)',
            boxShadow: '0 8px 24px rgba(240, 185, 11, 0.2)'
          }}>
            <div className="absolute top-0 right-0 w-40 h-40 rounded-full opacity-20" style={{
              background: 'radial-gradient(circle, #F0B90B 0%, transparent 70%)',
              filter: 'blur(30px)'
            }} />

            <div className="relative flex items-center justify-between">
              <div>
                <div className="text-sm font-semibold mb-1" style={{ color: '#FCD34D' }}>{t('profitFactor', language)}</div>
                <div className="text-xs" style={{ color: '#94A3B8' }}>
                  {t('avgWinDivLoss', language)}
                </div>
              </div>
              <div className="text-5xl font-bold mono" style={{
                color: (performance.profit_factor || 0) >= 2.0 ? '#10B981' :
                       (performance.profit_factor || 0) >= 1.5 ? '#F0B90B' :
                       (performance.profit_factor || 0) >= 1.0 ? '#FB923C' : '#F87171'
              }}>
                {(performance.profit_factor || 0) > 0 ? (performance.profit_factor || 0).toFixed(2) : 'N/A'}
              </div>
            </div>

            <div className="mt-3 text-xs font-semibold" style={{
              color: (performance.profit_factor || 0) >= 2.0 ? '#10B981' :
                     (performance.profit_factor || 0) >= 1.5 ? '#F0B90B' : '#94A3B8'
            }}>
              {(performance.profit_factor || 0) >= 2.0 && t('excellent', language)}
              {(performance.profit_factor || 0) >= 1.5 && (performance.profit_factor || 0) < 2.0 && t('good', language)}
              {(performance.profit_factor || 0) >= 1.0 && (performance.profit_factor || 0) < 1.5 && t('fair', language)}
              {(performance.profit_factor || 0) > 0 && (performance.profit_factor || 0) < 1.0 && t('poor', language)}
            </div>
          </div>
        </div>
        {/* 左侧结束 */}

        {/* 中间列：数据表格 (4列) */}
        <div className="lg:col-span-4 space-y-6">
          {/* 最佳/最差币种 */}
          {(performance.best_symbol || performance.worst_symbol) && (
            <div className="grid grid-cols-2 gap-4">
              {performance.best_symbol && (
                <div className="rounded-xl p-5 backdrop-blur-sm" style={{
                  background: 'linear-gradient(135deg, rgba(16, 185, 129, 0.15) 0%, rgba(14, 203, 129, 0.05) 100%)',
                  border: '1px solid rgba(16, 185, 129, 0.3)',
                  boxShadow: '0 4px 16px rgba(16, 185, 129, 0.1)'
                }}>
                  <div className="flex items-center gap-2 mb-3">
                    <span className="text-xl">🏆</span>
                    <span className="text-xs font-semibold" style={{ color: '#6EE7B7' }}>{t('bestPerformer', language)}</span>
                  </div>
                  <div className="text-2xl font-bold mono mb-1" style={{ color: '#10B981' }}>
                    {performance.best_symbol}
                  </div>
                  {symbolStats[performance.best_symbol] && (
                    <div className="text-sm font-semibold" style={{ color: '#6EE7B7' }}>
                      {symbolStats[performance.best_symbol].total_pn_l > 0 ? '+' : ''}
                      {symbolStats[performance.best_symbol].total_pn_l.toFixed(2)}% {t('pnl', language)}
                    </div>
                  )}
                </div>
              )}

              {performance.worst_symbol && (
                <div className="rounded-xl p-5 backdrop-blur-sm" style={{
                  background: 'linear-gradient(135deg, rgba(248, 113, 113, 0.15) 0%, rgba(246, 70, 93, 0.05) 100%)',
                  border: '1px solid rgba(248, 113, 113, 0.3)',
                  boxShadow: '0 4px 16px rgba(248, 113, 113, 0.1)'
                }}>
                  <div className="flex items-center gap-2 mb-3">
                    <span className="text-xl">📉</span>
                    <span className="text-xs font-semibold" style={{ color: '#FCA5A5' }}>{t('worstPerformer', language)}</span>
                  </div>
                  <div className="text-2xl font-bold mono mb-1" style={{ color: '#F87171' }}>
                    {performance.worst_symbol}
                  </div>
                  {symbolStats[performance.worst_symbol] && (
                    <div className="text-sm font-semibold" style={{ color: '#FCA5A5' }}>
                      {symbolStats[performance.worst_symbol].total_pn_l > 0 ? '+' : ''}
                      {symbolStats[performance.worst_symbol].total_pn_l.toFixed(2)}% {t('pnl', language)}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {/* 币种表现统计 - 现代化表格 */}
          {symbolStatsList.length > 0 && (
            <div className="rounded-2xl overflow-hidden" style={{
              background: 'rgba(30, 35, 41, 0.4)',
              border: '1px solid rgba(99, 102, 241, 0.2)',
              boxShadow: '0 4px 16px rgba(0, 0, 0, 0.2)'
            }}>
              <div className="p-5 border-b" style={{ borderColor: 'rgba(99, 102, 241, 0.2)', background: 'rgba(30, 35, 41, 0.6)' }}>
                <h3 className="font-bold flex items-center gap-2" style={{ color: '#E0E7FF' }}>
                  {t('symbolPerformance', language)}
                </h3>
              </div>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr style={{ background: 'rgba(15, 23, 42, 0.6)' }}>
                      <th className="text-left px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Symbol</th>
                      <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Trades</th>
                      <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Win Rate</th>
                      <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Total P&L</th>
                      <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Avg P&L</th>
                    </tr>
                  </thead>
                  <tbody>
                    {symbolStatsList.map((stat, idx) => (
                      <tr key={stat.symbol} className="transition-colors hover:bg-white/5" style={{
                        borderTop: idx > 0 ? '1px solid rgba(99, 102, 241, 0.1)' : 'none'
                      }}>
                        <td className="px-4 py-3">
                          <span className="font-bold mono text-sm" style={{ color: '#E0E7FF' }}>{stat.symbol}</span>
                        </td>
                        <td className="px-4 py-3 text-right mono text-sm" style={{ color: '#CBD5E1' }}>
                          {stat.total_trades}
                        </td>
                        <td className="px-4 py-3 text-right mono text-sm font-semibold" style={{
                          color: (stat.win_rate || 0) >= 50 ? '#10B981' : '#F87171'
                        }}>
                          {(stat.win_rate || 0).toFixed(1)}%
                        </td>
                        <td className="px-4 py-3 text-right mono text-sm font-bold" style={{
                          color: (stat.total_pn_l || 0) > 0 ? '#10B981' : '#F87171'
                        }}>
                          {(stat.total_pn_l || 0) > 0 ? '+' : ''}{(stat.total_pn_l || 0).toFixed(2)}%
                        </td>
                        <td className="px-4 py-3 text-right mono text-sm" style={{
                          color: (stat.avg_pn_l || 0) > 0 ? '#10B981' : '#F87171'
                        }}>
                          {(stat.avg_pn_l || 0) > 0 ? '+' : ''}{(stat.avg_pn_l || 0).toFixed(2)}%
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

        </div>
        {/* 中间列结束 */}

        {/* 右侧列：历史成交记录 (3列) */}
        <div className="lg:col-span-3">
          <div className="rounded-2xl overflow-hidden sticky top-24" style={{
            background: 'rgba(30, 35, 41, 0.4)',
            border: '1px solid rgba(240, 185, 11, 0.2)',
            maxHeight: 'calc(100vh - 200px)'
          }}>
            {/* 标题 - 固定在顶部 */}
            <div className="p-4 border-b backdrop-blur-sm" style={{
              background: 'rgba(240, 185, 11, 0.1)',
              borderColor: 'rgba(240, 185, 11, 0.3)'
            }}>
              <div className="flex items-center gap-2">
                <span className="text-xl">📜</span>
                <div>
                  <h3 className="font-bold text-sm" style={{ color: '#FCD34D' }}>{t('tradeHistory', language)}</h3>
                  <p className="text-xs" style={{ color: '#94A3B8' }}>
                    {performance?.recent_trades && performance.recent_trades.length > 0
                      ? t('completedTrades', language, { count: performance.recent_trades.length })
                      : t('completedTradesWillAppear', language)}
                  </p>
                </div>
              </div>
            </div>

            {/* 滚动内容区域 */}
            <div className="overflow-y-auto p-4 space-y-3" style={{ maxHeight: 'calc(100vh - 300px)' }}>
              {performance?.recent_trades && performance.recent_trades.length > 0 ? (
                performance.recent_trades.map((trade: TradeOutcome, idx: number) => {
                  const isProfitable = trade.pn_l >= 0;
                  const isRecent = idx === 0;

                  return (
                    <div key={idx} className="rounded-xl p-4 backdrop-blur-sm transition-all hover:scale-[1.02]" style={{
                      background: isRecent
                        ? isProfitable
                          ? 'linear-gradient(135deg, rgba(16, 185, 129, 0.15) 0%, rgba(14, 203, 129, 0.05) 100%)'
                          : 'linear-gradient(135deg, rgba(248, 113, 113, 0.15) 0%, rgba(246, 70, 93, 0.05) 100%)'
                        : 'rgba(30, 35, 41, 0.4)',
                      border: isRecent
                        ? isProfitable ? '1px solid rgba(16, 185, 129, 0.4)' : '1px solid rgba(248, 113, 113, 0.4)'
                        : '1px solid rgba(71, 85, 105, 0.3)',
                      boxShadow: isRecent
                        ? '0 4px 16px rgba(139, 92, 246, 0.2)'
                        : '0 2px 8px rgba(0, 0, 0, 0.1)'
                    }}>
                      {/* 头部：币种和方向 */}
                      <div className="flex items-center justify-between mb-3">
                        <div className="flex items-center gap-2">
                          <span className="text-base font-bold mono" style={{ color: '#E0E7FF' }}>
                            {trade.symbol}
                          </span>
                          <span className="text-xs px-2 py-1 rounded font-bold" style={{
                            background: trade.side === 'long' ? 'rgba(14, 203, 129, 0.2)' : 'rgba(246, 70, 93, 0.2)',
                            color: trade.side === 'long' ? '#10B981' : '#F87171'
                          }}>
                            {trade.side.toUpperCase()}
                          </span>
                          {isRecent && (
                            <span className="text-xs px-2 py-0.5 rounded font-semibold" style={{
                              background: 'rgba(240, 185, 11, 0.2)',
                              color: '#FCD34D'
                            }}>
                              {t('latest', language)}
                            </span>
                          )}
                        </div>
                        <div className="text-lg font-bold mono" style={{
                          color: isProfitable ? '#10B981' : '#F87171'
                        }}>
                          {isProfitable ? '+' : ''}{trade.pn_l_pct.toFixed(2)}%
                        </div>
                      </div>

                      {/* 价格信息 */}
                      <div className="grid grid-cols-2 gap-2 mb-3 text-xs">
                        <div>
                          <div style={{ color: '#94A3B8' }}>{t('entry', language)}</div>
                          <div className="font-mono font-semibold" style={{ color: '#CBD5E1' }}>
                            {trade.open_price.toFixed(4)}
                          </div>
                        </div>
                        <div className="text-right">
                          <div style={{ color: '#94A3B8' }}>{t('exit', language)}</div>
                          <div className="font-mono font-semibold" style={{ color: '#CBD5E1' }}>
                            {trade.close_price.toFixed(4)}
                          </div>
                        </div>
                      </div>

                      {/* 盈亏详情 */}
                      <div className="rounded-lg p-2 mb-2" style={{
                        background: isProfitable ? 'rgba(16, 185, 129, 0.1)' : 'rgba(248, 113, 113, 0.1)'
                      }}>
                        <div className="flex items-center justify-between text-xs">
                          <span style={{ color: '#94A3B8' }}>P&L</span>
                          <span className="font-bold mono" style={{
                            color: isProfitable ? '#10B981' : '#F87171'
                          }}>
                            {isProfitable ? '+' : ''}{trade.pn_l.toFixed(2)} USDT
                          </span>
                        </div>
                      </div>

                      {/* 时间和持仓时长 */}
                      <div className="flex items-center justify-between text-xs" style={{ color: '#94A3B8' }}>
                        <span>⏱️ {formatDuration(trade.duration)}</span>
                        {trade.was_stop_loss && (
                          <span className="px-2 py-0.5 rounded font-semibold" style={{
                            background: 'rgba(248, 113, 113, 0.2)',
                            color: '#FCA5A5'
                          }}>
                            {t('stopLoss', language)}
                          </span>
                        )}
                      </div>

                      {/* 交易时间 */}
                      <div className="text-xs mt-2 pt-2 border-t" style={{
                        color: '#64748B',
                        borderColor: 'rgba(71, 85, 105, 0.3)'
                      }}>
                        {new Date(trade.close_time).toLocaleString('en-US', {
                          month: 'short',
                          day: '2-digit',
                          hour: '2-digit',
                          minute: '2-digit'
                        })}
                      </div>
                    </div>
                  );
                })
              ) : (
                <div className="p-6 text-center">
                  <div className="text-4xl mb-2 opacity-50">📜</div>
                  <div style={{ color: '#94A3B8' }}>{t('noCompletedTrades', language)}</div>
                </div>
              )}
            </div>
          </div>
        </div>
        {/* 右侧列结束 */}

      </div>
      {/* 3列布局结束 */}

      {/* AI学习说明 - 现代化设计 */}
      <div className="rounded-2xl p-6 backdrop-blur-sm" style={{
        background: 'linear-gradient(135deg, rgba(240, 185, 11, 0.1) 0%, rgba(252, 213, 53, 0.05) 100%)',
        border: '1px solid rgba(240, 185, 11, 0.2)',
        boxShadow: '0 4px 16px rgba(240, 185, 11, 0.1)'
      }}>
        <div className="flex items-start gap-4">
          <div className="w-10 h-10 rounded-lg flex items-center justify-center text-xl flex-shrink-0" style={{
            background: 'rgba(240, 185, 11, 0.2)',
            border: '1px solid rgba(240, 185, 11, 0.3)'
          }}>
            💡
          </div>
          <div>
            <h3 className="font-bold mb-3 text-base" style={{ color: '#FCD34D' }}>{t('howAILearns', language)}</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
              <div className="flex items-start gap-2">
                <span style={{ color: '#F0B90B' }}>•</span>
                <span style={{ color: '#CBD5E1' }}>{t('aiLearningPoint1', language)}</span>
              </div>
              <div className="flex items-start gap-2">
                <span style={{ color: '#F0B90B' }}>•</span>
                <span style={{ color: '#CBD5E1' }}>{t('aiLearningPoint2', language)}</span>
              </div>
              <div className="flex items-start gap-2">
                <span style={{ color: '#F0B90B' }}>•</span>
                <span style={{ color: '#CBD5E1' }}>{t('aiLearningPoint3', language)}</span>
              </div>
              <div className="flex items-start gap-2">
                <span style={{ color: '#F0B90B' }}>•</span>
                <span style={{ color: '#CBD5E1' }}>{t('aiLearningPoint4', language)}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// 格式化持仓时长
function formatDuration(duration: string | undefined): string {
  if (!duration) return '-';

  // duration格式: "1h23m45s" or "23m45.123s"
  const match = duration.match(/(\d+h)?(\d+m)?(\d+\.?\d*s)?/);
  if (!match) return duration;

  const hours = match[1] || '';
  const minutes = match[2] || '';
  const seconds = match[3] || '';

  let result = '';
  if (hours) result += hours.replace('h', '小时');
  if (minutes) result += minutes.replace('m', '分');
  if (!hours && seconds) result += seconds.replace(/(\d+)\.?\d*s/, '$1秒');

  return result || duration;
}

// 从CoT trace中提取AI的历史表现分析和反思
function extractReflectionFromCoT(cotTrace: string | undefined): string | null {
  if (!cotTrace) return null;

  // 优先提取【历史经验反思】部分（新格式）
  const reflectionMatch = cotTrace.match(/【历史经验反思】\s*([\s\S]*?)(?=【|$)/);
  if (reflectionMatch) {
    const reflection = reflectionMatch[1].trim();
    if (reflection.length > 50) {
      return `🎯 AI历史经验总结\n\n${reflection}`;
    }
  }

  // 尝试提取"历史表现反馈"部分（兼容旧格式）
  const performanceSectionMatch = cotTrace.match(/## 📊 历史表现反馈[\s\S]*?(?=##|$)/);
  if (performanceSectionMatch) {
    const performanceSection = performanceSectionMatch[0];

    // 提取关键学习点
    const lines: string[] = [];

    // 提取总体统计
    const statsMatch = performanceSection.match(/总交易数：(\d+).*?胜率：([\d.]+)%.*?盈亏比：([\d.]+)/s);
    if (statsMatch) {
      const [, totalTrades, winRate, profitFactor] = statsMatch;
      lines.push(`📈 历史表现回顾：`);
      lines.push(`   • 完成了 ${totalTrades} 笔交易，胜率 ${winRate}%`);
      lines.push(`   • 盈亏比 ${profitFactor}（平均盈利/平均亏损）`);
      lines.push('');
    }

    // 提取最近交易
    const recentTradesMatch = performanceSection.match(/最近5笔交易[\s\S]*?(?=##|表现最好|$)/);
    if (recentTradesMatch) {
      const tradesText = recentTradesMatch[0];
      const tradeLines = tradesText.split('\n').filter(line => line.trim().startsWith('-'));

      if (tradeLines.length > 0) {
        lines.push(`🔍 最近交易分析：`);
        tradeLines.slice(0, 3).forEach(line => {
          lines.push(`   ${line.trim()}`);
        });
        lines.push('');
      }
    }

    // 提取最佳/最差币种
    const bestWorstMatch = performanceSection.match(/表现最好：([A-Z]+).*?\((.*?)\).*?表现最差：([A-Z]+).*?\((.*?)\)/s);
    if (bestWorstMatch) {
      const [, bestSymbol, bestPnl, worstSymbol, worstPnl] = bestWorstMatch;
      lines.push(`💡 币种表现洞察：`);
      lines.push(`   🏆 ${bestSymbol} 表现最佳 ${bestPnl}`);
      lines.push(`   💔 ${worstSymbol} 表现较差 ${worstPnl}`);
      lines.push('');
    }

    // 尝试提取AI的分析或决策理由
    const analysisMatch = cotTrace.match(/(?:分析|策略|决策)[:：]([\s\S]*?)(?:\n\n|##|$)/);
    if (analysisMatch) {
      const analysis = analysisMatch[1].trim();
      if (analysis.length > 20 && analysis.length < 500) {
        lines.push(`🎯 AI策略调整：`);
        lines.push(`   ${analysis.substring(0, 300)}${analysis.length > 300 ? '...' : ''}`);
      }
    }

    if (lines.length > 0) {
      return lines.join('\n');
    }
  }

  // 如果没有找到历史表现反馈，尝试提取整体思路
  const thinkingMatch = cotTrace.match(/(?:思考|分析|策略)[:：]([\s\S]{50,500}?)(?:\n\n|##|决策|$)/);
  if (thinkingMatch) {
    return `🤔 AI思考过程：\n\n${thinkingMatch[1].trim().substring(0, 400)}...`;
  }

  // 如果都没有，返回CoT的前面部分
  if (cotTrace.length > 100) {
    return `💭 AI分析摘要：\n\n${cotTrace.substring(0, 400).trim()}...`;
  }

  return null;
}
