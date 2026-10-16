"""Statistics from native analyzers and read-only Go-result cross-checks."""
import math
import statistics
from collections import defaultdict

import pandas as pd


def ratio(a, b):
    return a/b if b else None


def nested(d, *keys, default=0):
    for key in keys:
        if not isinstance(d, dict) or key not in d:
            return default
        d = d[key]
    return d


def return_stats(returns):
    avg = statistics.mean(returns) if returns else 0
    std = statistics.stdev(returns) if len(returns) > 1 else 0
    downside = math.sqrt(sum(min(r, 0)**2 for r in returns)/len(returns)) if returns else 0
    return dict(annualized_vol=std*math.sqrt(252), sharpe=ratio(avg*math.sqrt(252), std),
                sortino=ratio(avg*math.sqrt(252), downside))


def compute_metrics(curve, analyzers, commission):
    first, last = curve[0], curve[-1]
    years = (pd.Timestamp(last['time'])-pd.Timestamp(first['time'])).total_seconds()/(365.25*86400)
    returns = analyzers['time_return']
    # An initial zero return has no preceding evaluation interval.
    rs = list(returns.values())
    if rs and rs[0] == 0:
        rs = rs[1:]
    out = return_stats(rs)
    out['cagr'] = (last['equity']/first['equity'])**(1/years)-1 if years > 0 else None
    out['total_return'] = last['equity']/first['equity']-1
    out['final_equity'] = last['equity']
    out['max_drawdown'] = analyzers['drawdown']['max']['drawdown']/100
    peak, peak_date, maxdd = first['equity'], first['time'], 0
    out['drawdown_peak'] = out['drawdown_trough'] = first['time']
    for pt in curve:
        if pt['equity'] > peak:
            peak, peak_date = pt['equity'], pt['time']
        dd = 1-pt['equity']/peak
        if dd > maxdd:
            maxdd = dd
            out['drawdown_peak'], out['drawdown_trough'] = peak_date, pt['time']
    out['calmar'] = ratio(out['cagr'], out['max_drawdown']) if out['cagr'] is not None else None
    yearly = defaultdict(lambda: 1.0)
    for dt, r in returns.items():
        yearly[dt[:4]] *= 1+r
    out['calendar_year_returns'] = {y: g-1 for y, g in sorted(yearly.items())}
    cutoff = pd.Timestamp(last['time'])-pd.DateOffset(years=1)
    base = first
    for pt in curve:
        if pd.Timestamp(pt['time']) > cutoff:
            break
        base = pt
    out['last_12_months_return'] = last['equity']/base['equity']-1
    out['last_12_months_partial'] = pd.Timestamp(first['time']) > cutoff
    out['avg_invested_weight'] = statistics.mean(p['invested_weight'] for p in curve[1:]) if len(curve) > 1 else 0
    ta = analyzers['trade_analyzer']
    closed = nested(ta, 'total', 'closed')
    out.update(closed_trades=closed, win_rate=ratio(nested(ta, 'won', 'total'), closed),
        avg_win_pnl=nested(ta, 'won', 'pnl', 'average', default=None),
        avg_loss_pnl=nested(ta, 'lost', 'pnl', 'average', default=None),
        profit_factor=ratio(nested(ta, 'won', 'pnl', 'total'), abs(nested(ta, 'lost', 'pnl', 'total'))),
        avg_bars_held=nested(ta, 'len', 'average', default=None), total_commission=commission)
    return out


def rule_breakdown(trades, field, rules):
    out = {}
    for rule in rules:
        group = [t for t in trades if t[field] == rule]
        n = len(group)
        out[rule] = dict(closed_trades=n, win_rate=sum(t['net_pnl'] >= 0 for t in group)/n if n else None,
            avg_return=statistics.mean(t['net_return'] for t in group) if n else None,
            avg_bars_held=statistics.mean(t['bars_held'] for t in group) if n else None,
            net_pnl=sum(t['net_pnl'] for t in group))
    return out


def cross_check(result, go):
    reference = {s['name']: s for s in go['strategies']}
    rows = []
    for s in result['strategies']:
        if s['name'] not in reference:
            continue
        g, b = reference[s['name']], s['metrics']
        m = g['metrics']
        row = dict(name=s['name'], backtrader={k: b[k] for k in ('cagr', 'sharpe', 'max_drawdown', 'closed_trades')},
                   go={k: m[k] for k in ('cagr', 'sharpe', 'max_drawdown')})
        row['go']['closed_trades'] = m['closed_lots']
        row['difference'] = {k: row['backtrader'][k]-row['go'][k]
                             if row['backtrader'][k] is not None and row['go'][k] is not None else None
                             for k in row['backtrader']}
        # Reaggregate the saved Go marks to daily returns to quantify the 1h
        # sampling difference without rerunning or importing its simulator.
        marks = {}
        for pt in g['equity_curve']:
            marks[pt['time'][:10]] = pt['equity']
        values = list(marks.values())
        row['go_daily_sharpe'] = return_stats([values[i]/values[i-1]-1 for i in range(1, len(values))])['sharpe']
        row['size_caps'] = len(s['cash_size_adjustments'])
        row['go_final_equity'] = g['equity_curve'][-1]['equity']
        row['backtrader_final_equity'] = b['final_equity']
        row['go_costs'] = m['total_costs']
        row['backtrader_costs'] = b['total_commission']
        rows.append(row)
    cfg = result['config']
    expected = dict(timeframe=cfg['timeframe'], max_positions=cfg['max_positions'],
        rebalance_trading_days=cfg['rebalance_trading_days'], cost_bps_per_side=cfg['commission']*10000)
    mismatches = [f'{k}: BT={v}, Go={go["config"].get(k)}' for k, v in expected.items() if go['config'].get(k) != v]
    if cfg['cash'] != 100000:
        mismatches.append('Go initial equity is 100000; Backtrader cash differs')
    for k in ('first_signal_close', 'last_mark_close'):
        if pd.Timestamp(result['coverage'][k]) != pd.Timestamp(go['coverage'][k]).tz_localize(None):
            mismatches.append(f'coverage {k} differs')
    bt_symbols = set(result['coverage']['symbols'])-{'SPYBUSDT'}
    if bt_symbols != set(go['coverage']['symbols']):
        mismatches.append('eligible symbol sets differ')
    return dict(rows=rows, configuration_mismatches=mismatches)


def pct(x):
    return 'n/a' if x is None else f'{x*100:.2f}%'


def num(x):
    return 'n/a' if x is None else f'{x:.3f}'


def comparison_table(check):
    lines = ['| Strategy | BT CAGR | Go CAGR | Δ pp | BT Sharpe | Go Sharpe | Δ | BT MaxDD | Go MaxDD | Δ pp | BT trades | Go lots | Δ |',
             '|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|']
    for r in check['rows']:
        b, g, d = r['backtrader'], r['go'], r['difference']
        delta = lambda k, scale=1: 'n/a' if d[k] is None else f'{d[k]*scale:+.2f}'
        lines.append(f"| {r['name']} | {pct(b['cagr'])} | {pct(g['cagr'])} | {delta('cagr', 100)} | "
            f"{num(b['sharpe'])} | {num(g['sharpe'])} | {delta('sharpe')} | {pct(b['max_drawdown'])} | "
            f"{pct(g['max_drawdown'])} | {delta('max_drawdown', 100)} | {b['closed_trades']} | {g['closed_trades']} | {d['closed_trades']:+d} |")
    return '\n'.join(lines)


def curve_risk_metrics(curve):
    """Read-only risk statistics on native broker marks, including split anchors."""
    peak, dd = curve[0]['equity'], 0.0
    returns = {curve[0]['time']: 0.0}
    for i, pt in enumerate(curve):
        peak = max(peak, pt['equity'])
        dd = max(dd, 1-pt['equity']/peak)
        if i:
            returns[pt['time']] = pt['equity']/curve[i-1]['equity']-1
    m = compute_metrics(curve, dict(time_return=returns,
        drawdown=dict(max=dict(drawdown=dd*100)), trade_analyzer={}), 0)
    return {k: m[k] for k in ('total_return', 'cagr', 'annualized_vol', 'sharpe', 'sortino',
                              'max_drawdown', 'calmar', 'avg_invested_weight')}


def drawdown_episodes(curve):
    """Non-overlapping high-water-mark episodes, retaining an unrecovered tail."""
    peak = curve[0]
    active, episodes = None, []
    for pt in curve[1:]:
        if pt['equity'] >= peak['equity']:
            if active:
                episodes.append(active)
                active = None
            peak = pt
        else:
            dd = 1-pt['equity']/peak['equity']
            if active is None or dd > active['max_drawdown']:
                active = dict(peak=peak['time'], trough=pt['time'], max_drawdown=dd)
    if active:
        episodes.append(active)
    return sorted(episodes, key=lambda e: e['max_drawdown'], reverse=True)[:3]


def regime_analysis(result):
    runs = [s for s in result['strategies'] if 'overlay_config' in s]
    base = next(s for s in runs if s['overlay_config']['scaler'] == 'NONE')
    mid = len(base['equity_curve'])//2
    split, windows = {}, drawdown_episodes(base['equity_curve'])
    for s in runs:
        curve = s['equity_curve']
        # The preceding mark anchors the second half's FIRST session return.
        split[s['name']] = dict(first_half=curve_risk_metrics(curve[:mid]),
                               second_half=curve_risk_metrics(curve[max(0, mid-1):]))
    for w in windows:
        w['returns'] = {}
        for s in runs:
            marks = {p['time']: p['equity'] for p in s['equity_curve']}
            w['returns'][s['name']] = marks[w['trough']]/marks[w['peak']]-1
    return dict(first_half_sessions=mid, second_half_sessions=len(base['equity_curve'])-mid,
                first_half_end=base['equity_curve'][mid-1]['time'],
                second_half_start=base['equity_curve'][mid]['time'],
                split=split, drawdown_windows=windows)


def overlay_table(result):
    lines = ['| Run | CAGR | Ann vol | Sharpe | Sortino | MaxDD | DD peak → trough | Calmar | Avg exposure | Avg s | s<1 sessions | Scale trades | Scale commission | Total commission | Last 12m |',
             '|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|']
    for s in result['strategies']:
        if 'overlay_config' not in s:
            continue
        m = s['metrics']
        lines.append(f"| {s['name']} | {pct(m['cagr'])} | {pct(m['annualized_vol'])} | {num(m['sharpe'])} | "
            f"{num(m['sortino'])} | {pct(m['max_drawdown'])} | {m['drawdown_peak'][:10]} → {m['drawdown_trough'][:10]} | "
            f"{num(m['calmar'])} | {pct(m['avg_invested_weight'])} | {num(m['avg_scale'])} | "
            f"{pct(m['scaled_sessions_fraction'])} | {m['scale_trades']} | {m['scale_commission']:,.2f} | "
            f"{m['total_commission']:,.2f} | {pct(m['last_12_months_return'])} |")
    return '\n'.join(lines)


def sensitivity_table(result):
    lines = ['| Scaler | Floor | Band | Target vol | CAGR | Sharpe | MaxDD | Calmar |',
             '|---|---:|---:|---:|---:|---:|---:|---:|']
    for s in result.get('sensitivity', []):
        c, m = s['overlay_config'], s['metrics']
        lines.append(f"| {c['scaler']} | {c['floor']:.2f} | {c['band']:.2f} | {pct(c['target_vol'])} | "
                     f"{pct(m['cagr'])} | {num(m['sharpe'])} | {pct(m['max_drawdown'])} | {num(m['calmar'])} |")
    return '\n'.join(lines)


def regime_report(result):
    analysis = result['regime_analysis']
    lines = ['## 仓位缩放（regime overlay）', '', overlay_table(result), '',
        'Fixed a priori rules, not optimized; MOM_TOP only, daily signals. Ordinary rebalances apply s/maxpos; '
        'between rebalances a band breach or zero transition resizes existing holdings. '
        'A zero-to-positive restart with an empty portfolio selects the current rank basket. '
        'Scale trades count filled orders tagged SCALE on these extra resize days (not complete round trips); '
        'their actual native commission is shown separately. Rebalance fills retain REBALANCE tags. '
        'Avg s and s<1 use every evaluation signal close, including the final unfilled signal; '
        'exposure is the broker-observed holding weight after fills.', '',
        'Percentiles use inclusive rolling average rank/n over 252 observations, minimum 126. '
        'Unavailable inputs use neutral s=1; RV_RATIO uses the available trailing median after its first 21 returns. '
        'Breadth divides above-SMA200 rank-valid names by all rank-valid names, including names below SMA200. '
        'BASKET_VOL uses simple daily returns of a constant current top-maxpos basket; a constituent needs '
        'all 21 session returns (22 closes), with no stock-price forward fill. SPY RV uses log returns. '
        'All volatility uses sample standard deviation and sqrt(252).', '',
        'Breadth uses today’s listed universe → survivorship bias. Local VIX and VIX3M are independently '
        'forward filled from dates ≤ signal date, never backfilled. VIX closes at 16:15 ET versus stocks at '
        '16:00 ET; orders fill next open, after both closes. All calendar-year returns appear in the table above.', '',
        '### Calendar-half robustness', '',
        f"First half: {analysis['first_half_sessions']} sessions through {analysis['first_half_end'][:10]}; "
        f"second half: {analysis['second_half_sessions']} sessions from {analysis['second_half_start'][:10]}. "
        'Same equity curves, no refit; all thresholds fixed a priori. The second half includes its first session’s '
        'return using the preceding close as an anchor. Drawdown restarts at each half’s anchor; '
        'CAGR uses elapsed calendar time.', '',
        '| Run | Half | Return | CAGR | Ann vol | Sharpe | Sortino | MaxDD | Calmar | Exposure |',
        '|---|---|---:|---:|---:|---:|---:|---:|---:|---:|']
    for name, halves in analysis['split'].items():
        for half, m in halves.items():
            lines.append(f"| {name} | {half} | {pct(m['total_return'])} | {pct(m['cagr'])} | "
                f"{pct(m['annualized_vol'])} | {num(m['sharpe'])} | {num(m['sortino'])} | "
                f"{pct(m['max_drawdown'])} | {num(m['calmar'])} | {pct(m['avg_invested_weight'])} |")
    names = list(analysis['split'])
    lines += ['', '### NONE drawdown episodes: same-window returns', '',
        'Three worst non-overlapping high-water-mark episodes (including an unrecovered final episode), '
        'ranked by NONE peak-to-trough drawdown. Entries are close-to-close portfolio returns over '
        'the exact same dates, including costs.', '',
        '| NONE peak → trough | NONE MaxDD | '+' | '.join(names)+' |',
        '|---|---:|'+'---:|'*len(names)]
    for w in analysis['drawdown_windows']:
        lines.append(f"| {w['peak'][:10]} → {w['trough'][:10]} | {pct(w['max_drawdown'])} | "+
                     ' | '.join(pct(w['returns'][n]) for n in names)+' |')
    if result.get('sensitivity'):
        lines += ['', '### Sensitivity (fixed one-factor-at-a-time grid)', '',
                  'Defaults: floor=0.25, band=0.20, target-vol=0.40. Shared default cases run once; '
                  'the grid is descriptive, with no parameter selection or refitting.', '', sensitivity_table(result)]
    return '\n'.join(lines)


def render_report(result):
    cfg, cov = result['config'], result['coverage']
    lines = [f"# US-stock portfolios — Backtrader ({cfg['timeframe']})", '',
        f"Engine: Backtrader {result['engine_version']}; initial cash {cfg['cash']:,.2f}; "
        f"commission {pct(cfg['commission'])} per side; maxpos {cfg['max_positions']}; "
        f"momentum rebalance every {cfg['rebalance_trading_days']} SPY sessions.", '',
        'Native Cerebro / BackBroker / Market orders; no cheat-on-close/open; **no slippage**. '
        'Signals at close t execute at next SPY bar open t+1. Fractional shares; no leverage. '
        'Shares are sized at the signal close and capped to actual cash at the fill open; sells precede buys. '
        'A missing next-SPY open cancels the order without promoting another candidate. '
        'Open positions remain marked to market at the end; no terminal liquidation.', '',
        '## Strategy comparison', '',
        '| Strategy | CAGR | Ann vol | Sharpe | Sortino | MaxDD | Exposure | Trades | Win rate | Avg win $ | Avg loss $ | PF | Avg bars | Commission | Last 12m |',
        '|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|']
    for s in result['strategies']:
        m = s['metrics']
        lines.append(f"| {s['name']} | {pct(m['cagr'])} | {pct(m['annualized_vol'])} | {num(m['sharpe'])} | "
            f"{num(m['sortino'])} | {pct(m['max_drawdown'])} | {pct(m['avg_invested_weight'])} | {m['closed_trades']} | "
            f"{pct(m['win_rate'])} | {num(m['avg_win_pnl'])} | {num(m['avg_loss_pnl'])} | {num(m['profit_factor'])} | "
            f"{num(m['avg_bars_held'])} | {m['total_commission']:,.2f} | {pct(m['last_12_months_return'])} |")
    lines += ['', 'Daily TimeReturn equity returns: 252 sessions/year, sample volatility, Sharpe rf=0; '
        'Sortino uses RMS negative returns over all sessions. CAGR uses elapsed calendar time (365.25 days/year). '
        'DrawDown and TradeAnalyzer are native analyzers; trades mean fully closed Backtrader trades, including breakeven as wins. '
        'Average win/loss and profit factor use commission-net dollar P&L. Exposure is averaged over execution bars after the initial mark.', '',
        '| Strategy | DD peak | DD trough | Open positions | Pending terminal orders | Missing opens / Cash-slot zero fills | Cash/slot size caps |',
        '|---|---|---|---:|---:|---:|---:|']
    for s in result['strategies']:
        m = s['metrics']
        lines.append(f"| {s['name']} | {m['drawdown_peak']} | {m['drawdown_trough']} | {s['open_positions_at_end']} | "
                     f"{s['pending_orders_at_end']} | {s['missing_open_orders']} / {s['cash_or_slot_zero_fill_orders']} | {len(s['cash_size_adjustments'])} |")
    lines += ['', '## Calendar-year returns', '', '| Year | '+' | '.join(s['name'] for s in result['strategies'])+' |',
              '|---|'+'---:|'*len(result['strategies'])]
    for year in sorted({y for s in result['strategies'] for y in s['metrics']['calendar_year_returns']}):
        lines.append('| '+year+' | '+' | '.join(pct(s['metrics']['calendar_year_returns'].get(year)) for s in result['strategies'])+' |')
    lines += ['', 'First/last calendar years may be partial. Last 12m uses the last mark on/before the prior anniversary; '
              'a shorter history is flagged in JSON as last_12_months_partial.', '', '## MA2060 rule breakdown', '']
    for s in result['strategies']:
        if s['name'] != 'MA2060_RULES':
            continue
        lines += ['| Rule | Closed trades | Win rate | Avg net return | Avg bars | Net P&L | Filled entry/exit orders |',
                  '|---|---:|---:|---:|---:|---:|---:|']
        for field, counts in (('entry_breakdown', 'entry_order_counts'), ('exit_breakdown', 'exit_order_counts')):
            for rule, b in s[field].items():
                lines.append(f"| {rule} | {b['closed_trades']} | {pct(b['win_rate'])} | {pct(b['avg_return'])} | "
                             f"{num(b['avg_bars_held'])} | {b['net_pnl']:,.2f} | {s[counts].get(rule, 0)} |")
    if 'cross_check' in result:
        check = result['cross_check']
        lines += ['', '## Cross-check vs Go engine', '', comparison_table(check), '',
            'Δ = Backtrader minus Go; CAGR/MaxDD differences are percentage points. '
            'Reference: '+result.get('compare_path', '')+'.', '',
            '- Backtrader sizes fixed shares using the observable signal close; next-open gaps change weights. '
            'The Go reference recalculates its notional/target weights using next opens and post-cost equity. '
            'Backtrader cash size caps can reduce the last ranked purchase; this can change subsequent slot availability.',
            '- Commission is charged by Backtrader on actual fill price × quantity, both sides; '
            'different quantities/turnover produce different fee totals.',
            '- Backtrader counts fully closed trades; the Go reference counts FIFO lot portions, including partial rebalance trims. '
            'Retained positions with trims/adds can stay one open Backtrader trade.',
            '- For 1h, Backtrader Sharpe/volatility/Sortino use daily equity returns; Go uses hourly marks '
            'annualized with observed bars/session. MaxDD still uses every execution-bar mark in both.', '']
        if check['configuration_mismatches']:
            lines += ['Configuration/coverage mismatches: '+ '; '.join(check['configuration_mismatches']), '']
        else:
            lines += ['Configuration, evaluation dates and eligible symbol sets match the Go reference.', '']
        lines += ['| Strategy | Go Sharpe reaggregated daily | BT final equity | Go final equity | BT commission | Go costs |',
                  '|---|---:|---:|---:|---:|---:|']
        for r in check['rows']:
            lines.append(f"| {r['name']} | {num(r['go_daily_sharpe'])} | {r['backtrader_final_equity']:,.2f} | "
                f"{r['go_final_equity']:,.2f} | {r['backtrader_costs']:,.2f} | {r['go_costs']:,.2f} |")
    if 'regime_analysis' in result:
        lines += ['', regime_report(result), '']
    lines += ['', '## Coverage', '', f"{cov['universe_size']} loaded eligible stocks "
              f"out of {cov.get('candidate_universe_size', cov['universe_size'])} cached candidates; SPY calendar "
              f"{cov['first_signal_close']} → {cov['last_mark_close']}; {cov['evaluation_bars']} evaluation bars "
              f"({cov['spy_bars_including_warmup']} SPY bars including warmup).", '',
              'Excluded (SPY/QQQ benchmark-only): '+', '.join(cov.get('excluded', []))+'.', '',
              '| Symbol | Execution bars | Daily bars | Evaluation bars matched | First | Last | Source |',
              '|---|---:|---:|---:|---|---|---|']
    for s, c in sorted(cov['symbols'].items()):
        lines.append(f"| {s} | {c['execution_bars']} | {c['daily_bars']} | {c['evaluation_bars_on_spy_calendar']} | "
            f"{c['first_bar']} | {c['last_bar']} | {cov.get('sources', {}).get(s+'/'+cfg['timeframe'], 'synthetic')} |")
    lines += ['', 'Skipped: '+str(cov.get('skipped', {})), '', 'Warnings: '+str(cov.get('warnings', {})), '',
        'Removed invalid/duplicate bars: '+str({s: n for s, n in cov['invalid_or_duplicate_bars_removed'].items() if n}), '',
        '## Caveats', '',
        '- Survivorship bias: a current cached universe, without historical listing/delisting membership or delisting proceeds.',
        '- Yahoo adjusted underlying regular-session OHLCV differs from executable bStock tokens (hours, spreads, adjustments, liquidity). '
        'No bStock trading, volume constraint, taxes, borrow, dividends beyond the vendor adjustment, or market impact is modeled.',
        '- No LLM layer: these are mechanical rules. 1h Yahoo history covers only ~2 years (about 730 days), with 60m/last half-hour bars.',
        '- 1h MA/ATR windows count hourly bars; ranking uses the previous completed daily session. '
        'The common warmup is SPY rank-valid (253 daily bars), at least 61 execution bars, and the session’s last bar. '
        'New listings use prenext and do not postpone that start; signals only trade current, synchronized feeds.',
        '- MOM_X_MA selects top N before timing veto, without backfill; EW freezes the first-date universe and rebalances '
        'every 63 execution bars, with an explicit all-name exception to maxpos. X1–X4 precede E4 TARGET; X4 uses actual entry fill and signal ATR.', '']
    return '\n'.join(lines)
