"""T8: offline, causal state conditioning of observed/Backtrader trade outcomes.

No orders, matching, account simulation or P&L reconstruction occur here.
Run: python -m scripts.backtest.breaker_state_study
"""
import argparse
from bisect import bisect_left, bisect_right
import collections
import datetime as dt
import hashlib
import json
import math
from pathlib import Path
import random
import sqlite3
import statistics
import sys

from .breakdown_study import DAY, HOUR, SCRATCH, atomic_json, kept_indices, mean_difference
from .ict_bt import fmt, stats

WINDOWS = (6, 24)
PERIODS = ('full', 'first', 'second')
DEFAULT_DB = '/Users/zhangyun/workspace/nofx/data/data.db'


def stamp(t):
    return dt.datetime.fromtimestamp(t / 1000, dt.timezone.utc).isoformat(timespec='milliseconds')


def entry_key(t):
    return t['entry_ms'], t['id']


def exit_key(t):
    return t['exit_ms'], t['id']


def validate_trade(t):
    if (not math.isfinite(t['r']) or t['entry_ms'] <= 0
            or t['exit_ms'] <= t['entry_ms']):
        raise ValueError(f'Invalid completed trade: {t}')


def load_live(path):
    """Use valid initial-risk rows for BOTH outcomes and state; never invent R."""
    uri = Path(path).resolve().as_uri() + '?mode=ro'
    with sqlite3.connect(uri, uri=True) as con:
        con.execute('PRAGMA query_only=ON')
        con.row_factory = sqlite3.Row
        rows = [dict(r) for r in con.execute(
            "SELECT id,trader_id,symbol,side,entry_time,exit_time,entry_quantity,quantity,"
            "entry_price,initial_stop_loss,realized_pnl,fee,ai_managed,source,close_reason "
            "FROM trader_positions WHERE trader_id LIKE ? AND status='CLOSED' "
            "ORDER BY entry_time,id", ('%1787982996',))]
    traders = sorted({r['trader_id'] for r in rows})
    if len(traders) != 1:
        raise ValueError(f'Expected exactly one live account, got {traders}')
    trades, excluded, fallback = [], collections.Counter(), 0
    excluded_pnl = dict(manual=0.0, nonmanual_no_stop=0.0)
    for row in rows:
        stop = float(row['initial_stop_loss'] or 0)
        if not row['ai_managed'] and row['source'] == 'sync' and stop <= 0:
            excluded['manual_unmanaged'] += 1
            excluded_pnl['manual'] += float(row['realized_pnl'])
            continue
        if stop <= 0:
            excluded['no_initial_stop'] += 1
            excluded_pnl['nonmanual_no_stop'] += float(row['realized_pnl'])
            continue
        qty = float(row['entry_quantity'] or 0)
        if qty <= 0:
            qty = float(row['quantity'] or 0)
            fallback += 1
        risk = qty * abs(float(row['entry_price']) - stop)
        if not math.isfinite(risk) or risk <= 0:
            excluded['invalid_initial_risk'] += 1
            continue
        t = dict(id=row['id'], symbol=row['symbol'], side=row['side'],
                 entry_ms=row['entry_time'], exit_ms=row['exit_time'],
                 r=float(row['realized_pnl']) / risk, how=row['close_reason'],
                 initial_risk=risk, entry_quantity_used=qty,
                 realized_pnl=row['realized_pnl'], fee=row['fee'], source=row['source'],
                 ai_managed=bool(row['ai_managed']))
        validate_trade(t)
        trades.append(t)
    if not trades:
        raise ValueError('No valid live R observations')
    digest = hashlib.sha256(json.dumps(rows, sort_keys=True).encode()).hexdigest()
    return trades, dict(db_uri=uri, trader_id=traders[0], closed_rows=len(rows),
                        excluded=dict(excluded), quantity_fallback=fallback,
                        selected_rows_sha256=digest,
                        raw_entry_range=[min(r['entry_time'] for r in rows),
                                         max(r['entry_time'] for r in rows)],
                        excluded_realized_pnl=excluded_pnl)


def load_history(directory):
    directory = Path(directory)
    sb, nb = (directory / 'study.json').read_bytes(), (directory / 'net.json').read_bytes()
    study, saved = json.loads(sb), json.loads(nb)
    jobs, results = study['jobs'], saved['results']
    if not jobs or len(jobs) != len(results):
        raise ValueError('Incomplete/mismatched Backtrader checkpoint')
    # Source is the specified bd.run_checkpoint output. Fail on incomplete labels;
    # do not replace an open/unfilled trade by a fabricated zero-R outcome.
    for j, r in zip(jobs, results):
        if (r['r'] is None or r['how'] not in ('tp', 'sl', 'time')
                or r['entry_ms'] != j['t_ms']):
            raise ValueError(f'Unexpected source label: {r}')
    eligible = [i for i, j in enumerate(jobs) if j['new'] >= 50]
    kept, blocked = kept_indices(jobs, results, False, eligible)
    trades = [dict(id=i, symbol=jobs[i]['sym'], side=jobs[i]['side'],
                   entry_ms=results[i]['entry_ms'], exit_ms=results[i]['exit_ms'],
                   r=results[i]['r'], how=results[i]['how']) for i in kept]
    for t in trades:
        validate_trade(t)
    return trades, dict(directory=str(directory), source_jobs=len(jobs),
                        new_ge_50=len(eligible), overlap_blocked=blocked,
                        after_overlap=len(trades), net_signature=saved['signature'],
                        study_sha256=hashlib.sha256(sb).hexdigest(),
                        net_sha256=hashlib.sha256(nb).hexdigest(),
                        data_sha256=study['meta']['data_sha256'])


def sample_daily(trades, seed, cap=8):
    """Uniform per-day subset, outcome-blind; filtering has already happened."""
    if cap < 1:
        raise ValueError('Daily cap must be positive')
    days = collections.defaultdict(list)
    for t in sorted(trades, key=entry_key):
        days[t['entry_ms'] // DAY].append(t)
    rng, out = random.Random(seed), []
    for day in sorted(days):
        group = days[day]
        out.extend(rng.sample(group, cap) if len(group) > cap else group)
    return sorted(out, key=entry_key)


def compute_states(trades, windows=WINDOWS):
    """Return input-aligned states using only exits in (entry-H, entry].

    UTC day uses [midnight, entry]. Zero R interrupts a losing streak. Tied
    exits sort by source id; all entries at the same time see the same exits.
    """
    closed = sorted(trades, key=exit_key)
    exits = [t['exit_ms'] for t in closed]
    out = []
    for trade in trades:
        t = trade['entry_ms']
        end = bisect_right(exits, t)
        day_start = t // DAY * DAY
        begin = bisect_left(exits, day_start, 0, end)
        state = dict(day_r=math.fsum(c['r'] for c in closed[begin:end]))
        for h in windows:
            losses = 0
            start = bisect_right(exits, t - h * HOUR, 0, end)
            for i in range(end - 1, start - 1, -1):
                if closed[i]['r'] >= 0:
                    break
                losses += 1
            state[f'consec_{h}h'] = losses
        out.append(state)
    return out


def day_bucket(r):
    return ('> -1' if r > -1 else '(-2,-1]' if r > -2 else
            '(-3,-2]' if r > -3 else '(-4,-3]' if r > -4 else '<= -4')


def state_groups(states):
    """Predeclared 25 contrasts; thresholds are inclusive and nested."""
    groups = []
    for h in WINDOWS:
        for k in range(5):
            groups.append(dict(name=f'{h}h/k={k if k < 4 else "4+"}',
                               family='loss', threshold=False,
                               members={i for i, s in enumerate(states)
                                        if min(s[f'consec_{h}h'], 4) == k}))
        for k in (2, 3, 4):
            groups.append(dict(name=f'{h}h/k>={k}', family='loss', threshold=True,
                               members={i for i, s in enumerate(states)
                                        if s[f'consec_{h}h'] >= k}))
    for bucket in ('> -1', '(-2,-1]', '(-3,-2]', '(-4,-3]', '<= -4'):
        groups.append(dict(name='day/桶 ' + bucket, family='day', threshold=False,
                           members={i for i, s in enumerate(states) if day_bucket(s['day_r']) == bucket}))
    for x in (2, 3, 4, 6):
        groups.append(dict(name=f'day/<= -{x}', family='day', threshold=True,
                           members={i for i, s in enumerate(states) if s['day_r'] <= -x}))
    return groups


def outcome_stats(trades, indices):
    """Reuse stats; its drawdown input is explicitly sorted by exit time."""
    ix = sorted(indices, key=lambda i: exit_key(trades[i]))
    return stats([trades[i]['r'] for i in ix])


def add_inference(d):
    if d is None or d['cluster_se'] is None:
        return d
    delta, se = d['diff'], d['cluster_se']
    d = dict(d)
    d['p_normal_two_sided'] = (math.erfc(abs(delta) / (math.sqrt(2) * se)) if se
                               else 1.0 if delta == 0 else 0.0)
    d['ci95_normal'] = [delta - 1.96 * se, delta + 1.96 * se]
    return d


def contrast(trades, jobs, results, inside, outside):
    d = mean_difference(jobs, results, inside, jobs, results, outside)
    if d is None:
        return None
    influence = collections.defaultdict(float)
    for indices, sign in ((inside, 1), (outside, -1)):
        avg = math.fsum(trades[i]['r'] for i in indices) / len(indices)
        for i in indices:
            week = (trades[i]['entry_ms'] // DAY + 3) // 7
            influence[str(week)] += sign * (trades[i]['r'] - avg) / len(indices)
    d['week_influence'] = dict(influence)
    return add_inference(d)


def analyse_stream(trades, midpoint, seed=None):
    trades = sorted(trades, key=entry_key)
    states = compute_states(trades)
    groups = state_groups(states)
    jobs = [dict(t_ms=t['entry_ms']) for t in trades]
    results = [dict(r=t['r']) for t in trades]
    periods = {}
    for period in PERIODS:
        parent = [i for i, t in enumerate(trades) if period == 'full'
                  or (t['entry_ms'] <= midpoint) == (period == 'first')]
        baseline = outcome_stats(trades, parent)
        rows = []
        for group in groups:
            inside = [i for i in parent if i in group['members']]
            outside = [i for i in parent if i not in group['members']]
            after = outcome_stats(trades, outside)
            rows.append(dict(name=group['name'], family=group['family'],
                             threshold=group['threshold'], stats=outcome_stats(trades, inside),
                             complement_n=len(outside),
                             difference=contrast(trades, jobs, results, inside, outside),
                             after=after, removed_n=len(inside),
                             sum_r_change=after['sum'] - baseline['sum'],
                             dd_r_change=after['dd'] - baseline['dd'],
                             proportional_sum_r=baseline['sum'] * len(outside) / len(parent)
                             if parent else None))
        periods[period] = dict(baseline=baseline, rows=rows)
    return dict(seed=seed, midpoint_ms=midpoint, periods=periods,
                observations=[dict(t, state=s) for t, s in zip(trades, states)])


def dispersion(values):
    values = [v for v in values if v is not None and math.isfinite(v)]
    return dict(mean=statistics.mean(values) if values else None,
                sd=statistics.stdev(values) if len(values) > 1 else 0.0 if values else None,
                min=min(values) if values else None, max=max(values) if values else None,
                seeds=len(values))


def aggregate_stats(items):
    return {k: dispersion([s[k] if s['n'] or k in ('n', 'sum', 'dd') else None for s in items])
            for k in ('n', 'sum', 'avg', 'se', 'win', 'dd')}


def ensemble_difference(differences):
    """SE of seed-mean contrast: average shared-week influences, not IID seeds.

    The random streams share source trades and market weeks. Averaging their
    weekly influences keeps this dependence instead of dividing SE by sqrt(20).
    An empty-arm seed makes the 20-seed contrast undefined, rather than zero.
    """
    valid = [d for d in differences if d is not None]
    desc = dispersion([d['diff'] for d in valid])
    out = dict(seed_diff=desc, valid_seeds=len(valid), total_seeds=len(differences),
               diff=None, cluster_se=None, weeks=None)
    if len(valid) != len(differences) or not valid:
        return out
    influence = collections.defaultdict(float)
    for d in valid:
        for week, v in d['week_influence'].items():
            influence[int(week)] += v / len(valid)
    weeks = max(influence) - min(influence) + 1
    se = math.sqrt(weeks / (weeks - 1) * math.fsum(v*v for v in influence.values())) if weeks > 1 else None
    out.update(diff=desc['mean'], cluster_se=se, weeks=weeks)
    return add_inference(out)


def aggregate_streams(streams):
    periods = {}
    for period in PERIODS:
        blocks = [s['periods'][period] for s in streams]
        rows = []
        for k, template in enumerate(blocks[0]['rows']):
            rr = [b['rows'][k] for b in blocks]
            assert all(r['name'] == template['name'] for r in rr)
            rows.append(dict(name=template['name'], family=template['family'],
                             threshold=template['threshold'],
                             stats=aggregate_stats([r['stats'] for r in rr]),
                             complement_n=dispersion([r['complement_n'] for r in rr]),
                             difference=ensemble_difference([r['difference'] for r in rr]),
                             seed_nominal_worse=sum(is_worse(r['difference']) for r in rr),
                             after=aggregate_stats([r['after'] for r in rr]),
                             sum_r_change=dispersion([r['sum_r_change'] for r in rr]),
                             dd_r_change=dispersion([r['dd_r_change'] for r in rr]),
                             proportional_sum_r=dispersion([r['proportional_sum_r'] for r in rr])))
        periods[period] = dict(baseline=aggregate_stats([b['baseline'] for b in blocks]), rows=rows)
    return dict(periods=periods)


def is_worse(d, alpha=0.05):
    return (d is not None and d.get('diff') is not None and d['diff'] < 0
            and d.get('p_normal_two_sided', 1) < alpha)


def all_differences(stream):
    return [r['difference'] for p in stream['periods'].values() for r in p['rows']]


def test_inventory(live, streams, ensemble):
    ds = all_differences(live) + [d for s in streams for d in all_differences(s)] + all_differences(ensemble)
    alpha = 0.05 / len(ds)
    corrected = []
    for label, stream in [('live', live), ('history_ensemble', ensemble),
                          *[(f'history_seed_{s["seed"]}', s) for s in streams]]:
        for period, block in stream['periods'].items():
            for row in block['rows']:
                if is_worse(row['difference'], alpha):
                    corrected.append(dict(dataset=label, period=period, state=row['name'],
                                          threshold=row['threshold'], difference=row['difference']))
    return dict(groups_per_period=25, periods=3, live_planned=75,
                historical_seed_planned=75 * len(streams), ensemble_planned=75,
                total_planned=len(ds), estimable=sum(d is not None and d['cluster_se'] is not None for d in ds),
                bonferroni_alpha=alpha,
                nominal_worse=sum(is_worse(d) for d in ds),
                bonferroni_worse=len(corrected), bonferroni_worse_cases=corrected)


def num(x, digits=4):
    return 'NA' if x is None else f'{x:+.{digits}f}'


def stats_text(name, s, width=34):
    if not s['n']:
        return f'{name:{width}s} n=0 avgR=NA±NA sumR=0 win=NA maxDD_R=0'
    return fmt(name, s, width)


def inference_text(d, alpha):
    if d is None or d['cluster_se'] is None:
        return '无法检验（空组、种子空组或不足两周）'
    verdict = ('负差异名义显著' if is_worse(d) else
               '正差异名义显著' if d['diff'] > 0 and d['p_normal_two_sided'] < .05 else '差异不显著')
    if is_worse(d, alpha):
        verdict += '，Bonferroni后仍为负'
    ci = ','.join(num(v) for v in d['ci95_normal'])
    return (f"ΔR={num(d['diff'])} 周SE={num(d['cluster_se'])} 周数={d['weeks']} "
            f"95%CI=[{ci}] p≈{d['p_normal_two_sided']:.4g}；{verdict}")


def get_row(stream, name, period='full'):
    return next(r for r in stream['periods'][period]['rows'] if r['name'] == name)


def conclusions(live, ensemble, alpha):
    lines = []
    for family, title in (('loss', '连亏状态之后入场是否显著更差'), ('day', '日亏状态之后入场是否显著更差')):
        descriptions = []
        for label, stream in (('实盘', live), ('历史20种子均值', ensemble)):
            rows = [r for r in stream['periods']['full']['rows'] if r['family'] == family and r['threshold']]
            nominal = [r['name'] for r in rows if is_worse(r['difference'])]
            adjusted = [r['name'] for r in rows if is_worse(r['difference'], alpha)]
            stable = [name for name in adjusted if all(is_worse(get_row(stream, name, p)['difference'], alpha)
                                                       for p in ('first', 'second'))]
            text = label + '：'
            if adjusted:
                text += '全样本校正后更差=' + '、'.join(adjusted)
                text += '；前后半均通过=' + ('、'.join(stable) if stable else '无，稳定性不足')
            else:
                text += '无法证明稳定且经多重校正后显著更差'
                if nominal:
                    text += '（仅名义显著：' + '、'.join(nominal) + '；多重校正未通过）'
            descriptions.append(text)
        lines.append(title + '？' + '；'.join(descriptions) + '。')
    details = []
    # Predeclared examples, not the best after-the-fact threshold.
    for label, stream, aggregated in (('实盘', live, False), ('历史均值', ensemble, True)):
        b = stream['periods']['full']['baseline']
        value = lambda s, k: s[k]['mean'] if aggregated else s[k]
        parts = []
        for name in ('24h/k>=3', 'day/<= -3'):
            a = get_row(stream, name)['after']
            parts.append(f"{name}剔除后sumR={num(value(a, 'sum'), 2)}、DD={value(a, 'dd'):.2f}R")
        details.append(f"{label}原sumR={num(value(b, 'sum'), 2)}、DD={value(b, 'dd'):.2f}R；" + '；'.join(parts))
    lines.append('筛除后最大回撤与sumR的权衡如何？' + '；'.join(details)
                 + '。交易量同步减少；仅事后静态筛除，不能据此证明熔断有价值。')
    return lines


def report_text(meta, live, streams, ensemble, inventory):
    lm, hm = meta['live'], meta['history']
    lo = live['observations']
    hs = streams[0]['observations']
    alpha = inventory['bonferroni_alpha']
    corrected = inventory['bonferroni_worse_cases']
    corrected_text = '；'.join(f"{c['dataset']}/{c['period']}/{c['state']}" for c in corrected) or '无'
    def live_weeks(period):
        times = [t['entry_ms'] for t in lo if period == 'full'
                 or (t['entry_ms'] <= live['midpoint_ms']) == (period == 'first')]
        return (max(times) // DAY + 3) // 7 - (min(times) // DAY + 3) // 7 + 1 if times else 0
    lines = [
        'T8 账户级连亏 / UTC日亏后的入场期望（离线条件统计；不执行新订单）',
        f"实盘R样本入场范围 {stamp(min(t['entry_ms'] for t in lo))} ～ {stamp(max(t['entry_ms'] for t in lo))}；"
        f"平仓范围 {stamp(min(t['exit_ms'] for t in lo))} ～ {stamp(max(t['exit_ms'] for t in lo))}。",
        f"实盘原查询入场 {stamp(lm['raw_entry_range'][0])} ～ {stamp(lm['raw_entry_range'][1])}，CLOSED={lm['closed_rows']}；"
        f"R样本/状态={len(lo)}；排除={lm['excluded']}；entry_quantity回退quantity={lm['quantity_fallback']}。",
        f"历史过滤后候选入场范围 {stamp(hm['entry_range'][0])} ～ {stamp(hm['entry_range'][1])}；"
        f"原信号={hm['source_jobs']}，NEW>=50={hm['new_ge_50']}，同币同向单仓后={hm['after_overlap']}，blocked={hm['overlap_blocked']}；"
        f"每种子入场n={len(hs)}，seed={meta['seeds']}，UTC每日最多8笔，20次重复均值另列。",
        '口径：实盘R=realized_pnl / [quantity_at_entry × |entry_price−initial_stop_loss|]；quantity_at_entry优先entry_quantity>0，否则quantity。只保留initial_stop_loss>0且风险有效的行。',
        '无初始止损的托管单也完全不进入状态/样本，不用权益快照估造R；缺失的亏损/盈利可能改变实际账户连亏和日亏状态。手动/未托管=ai_managed=0且source=sync且无初始止损，完全排除。',
        f"排除行realized_pnl合计={lm['excluded_realized_pnl']}（仅审计，不换算R、不用于状态）。",
        '实盘严格按题设使用realized_pnl，不再减fee；历史R沿用Backtrader净盈亏（含原费用/滑点）。两数据集分开，不合并估计，也不把R直接当权益收益。',
        '因此实盘微利但扣fee后亏损的行，在本题R口径仍会中断连亏；不能把这里的连亏序列等同完整费后权益亏损序列。',
        f"检验总数={inventory['total_planned']}：实盘75 + 历史20种子×75={inventory['historical_seed_planned']} + 种子均值75；"
        f"每流25状态×全/前/后3段；可估计周SE={inventory['estimable']}；包含空组与重复/嵌套方向。",
        f"多重比较警告：p为双侧正态近似；名义α=.05，全局保守Bonferroni α={alpha:.8g}；重复种子、互补桶及半段不是独立复验。"
        f"负差异名义显著={inventory['nominal_worse']}项，校正后={inventory['bonferroni_worse']}项。",
        f'校正后负差异所在位置={corrected_text}；个别种子的半段结果不能代替全样本与20种子均值的稳定证据。',
        '状态严格只用exit_ms<=entry_ms已平仓结果；连亏窗口为(entry−H小时, entry]（左开右闭），跨所有币种，倒序遇到R>=0即停止，R=0会中断。',
        '日亏为UTC[当日00:00, entry]的已平仓R之和，含昨天开仓今天平仓；无当日平仓则0。阈值包含等号；k=4+及k>=4相同，仍计重复比较。',
        '连续亏损按R<0，包含止损和其他亏损平仓；未另做仅止损检验（sync/external等实盘原因不完整）。同毫秒多笔平仓以源id排序，新入场同刻看相同状态。',
        '统计复用kept_indices / mean_difference / stats / fmt；各桶比较同一数据流、同一半段的补集。表内avgR±SE为IID SE；差异的SE为UTC周一起始的共享周影响函数聚类SE。',
        f"实盘全样本只有{live_weeks('full')}个日历周（前/后半{live_weeks('first')}/{live_weeks('second')}周），"
        '周聚类SE和正态CI不可靠；极少样本/单一周贡献时尤其不可据小p值作部署判断。周际依赖也未处理。',
        '历史先在全部NEW>=50上单仓过滤，再按UTC日均匀抽样（不看R），seed=7..26；下采样未重新接纳已blocked信号。只在抽中的组合流上构建状态。',
        '这只是独立信号对账户并发/资金的近似：不建共享账户、不做撮合、不改变Backtrader每笔R、不加入保证金/资金费率/动态风险权重。日内均匀抽样是事后研究设计，并非可直接上线的入场调度器。',
        '历史表的n/avgR/胜率等为各有效种子等权平均；同时给种子间样本SD（不是SE）。空桶avgR不计入平均且注明有效种子；空组使20种子均值差异不可检验。',
        '种子均值ΔR的周SE：先平均各种子的同日历周影响函数，再按mean_difference相同的G/(G−1)公式计算；保留共用交易与市场周依赖，绝不把20次重复当独立数据来除sqrt(20)。',
        f"前后半段：实盘入场中位时点={stamp(live['midpoint_ms'])}；历史统一采用下采样前单仓候选的中位时点={stamp(meta['history_midpoint_ms'])}；等于分界归前半。",
        '各半段共用完整流中已计算的因果状态（不在分界重置）；回撤在该半段选中交易上从0累计，按exit_ms、源id排序，跨分界未平仓不作盯市。状态历史在数据首笔之前不可见，初始0不代表真实账户此前没有亏损。',
        '反事实只静态剔除处于状态的已发生入场；报告剩余sumR/avgR/平仓序列最大回撤R。未重放剔除交易对后续状态的反馈（包括连亏解除时间、恢复开仓与已blocked信号）；不是可执行熔断回测。',
        '回撤是等权R序列的峰谷差，非权益百分比；日亏2/3/4/6R仅在每笔风险固定约3%时粗略对应6/9/12/18%，不能还原实际日内权益亏损。',
        f"实盘最大单笔R={max(t['r'] for t in lo):.5f}（id={max(lo, key=lambda t: t['r'])['id']}）；按题设保留，不截尾，平均值与sumR可能被该笔显著影响。",
        f"源数据库={lm['db_uri']}；trader={lm['trader_id']}；查询行SHA256={lm['selected_rows_sha256']}。",
        f"源历史={hm['directory']}；net签名={hm['net_signature']}；study SHA256={hm['study_sha256']}；net SHA256={hm['net_sha256']}。",
        f"运行命令={meta['command']}；生成UTC={meta['generated_utc']}。",
    ]
    labels = {'full': '全样本', 'first': '前半段', 'second': '后半段'}
    for period in PERIODS:
        block = live['periods'][period]
        lines.extend(['', f"实盘 / {labels[period]}", stats_text('基线', block['baseline'])])
        for r in block['rows']:
            lines.append(stats_text(r['name'], r['stats'], 18)
                         + f" 对补集n={r['complement_n']}：{inference_text(r['difference'], alpha)}")
        lines.append('静态筛除阈值（ΔsumR/ΔDD=筛除后−基线；比例sumR=随机等比例减交易的描述性参照，无显著性含义）')
        for r in block['rows']:
            if r['threshold']:
                lines.append(stats_text('剔除 ' + r['name'], r['after'], 21)
                             + f" 删除n={r['removed_n']} ΔsumR={num(r['sum_r_change'], 2)}"
                             f" ΔDD={num(r['dd_r_change'], 2)} 比例sumR={num(r['proportional_sum_r'], 2)}")
    for period in PERIODS:
        block = ensemble['periods'][period]
        b = block['baseline']
        lines.extend(['', f"历史20种子均值 / {labels[period]}",
                      '基线 ' + aggregate_stats_text(b),
                      '每行 avgR±平均IID SE、seedSD；ΔR±周SE为种子均值对补集的检验。'])
        for r in block['rows']:
            desc = r['difference']['seed_diff']
            lines.append(f"{r['name']:18s} {aggregate_stats_text(r['stats'])} "
                         f"对补集n均={r['complement_n']['mean']:.1f}；{inference_text(r['difference'], alpha)} "
                         f"ΔR种子SD={num(desc['sd'])}，负差异名义显著种子={r['seed_nominal_worse']}/20")
        lines.append('静态筛除阈值（均值±种子SD；每种子先算平仓序列DD再平均，不对平均R序列算DD）')
        for r in block['rows']:
            if r['threshold']:
                lines.append(f"剔除 {r['name']:18s} {aggregate_stats_text(r['after'])} "
                             f"ΔsumR={num(r['sum_r_change']['mean'], 2)}±{r['sum_r_change']['sd']:.2f} "
                             f"ΔDD={num(r['dd_r_change']['mean'], 2)}±{r['dd_r_change']['sd']:.2f} "
                             f"比例sumR均={num(r['proportional_sum_r']['mean'], 2)}")
    lines.extend(['', '历史固定seed=7明细 / 全样本（完整20种子、各半段、每笔状态见analysis.json）',
                  stats_text('seed=7基线', streams[0]['periods']['full']['baseline'])])
    for r in streams[0]['periods']['full']['rows']:
        lines.append(stats_text(r['name'], r['stats'], 18) + ' ' + inference_text(r['difference'], alpha))
    lines.extend(['', '每种子基线离散度与半段稳定性：'])
    for s in streams:
        pieces = []
        for p in PERIODS:
            b = s['periods'][p]['baseline']
            pieces.append(f"{labels[p]}n={b['n']} sumR={b['sum']:+.2f} avgR={b['avg']:+.4f} DD={b['dd']:.2f}R")
        lines.append(f"seed={s['seed']} " + '；'.join(pieces))
    lines.extend(['', *conclusions(live, ensemble, alpha)])
    return '\n'.join(lines) + '\n'


def aggregate_stats_text(s):
    win = f"{s['win']['mean']:.1%}" if s['win']['mean'] is not None else 'NA'
    return (f"n={s['n']['mean']:.1f}[{s['n']['min']},{s['n']['max']}] "
            f"avgR={num(s['avg']['mean'])}±{num(s['se']['mean']).lstrip('+')} "
            f"seedSD={num(s['avg']['sd'])}(有效{s['avg']['seeds']}/20) "
            f"win均={win} sumR={num(s['sum']['mean'], 2)}±{s['sum']['sd']:.2f} "
            f"DD={s['dd']['mean']:.2f}±{s['dd']['sd']:.2f}R")


def json_safe(obj):
    if isinstance(obj, float) and not math.isfinite(obj):
        return None
    if isinstance(obj, dict):
        return {k: json_safe(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [json_safe(v) for v in obj]
    return obj


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--db', default=DEFAULT_DB)
    ap.add_argument('--study-dir', default=str(SCRATCH / 'outBD'))
    ap.add_argument('--out', default=str(Path(__file__).parent / '_out_breaker'))
    args = ap.parse_args()
    live_trades, live_meta = load_live(args.db)
    candidates, history_meta = load_history(args.study_dir)
    history_meta['entry_range'] = [min(t['entry_ms'] for t in candidates), max(t['entry_ms'] for t in candidates)]
    live_midpoint = statistics.median(t['entry_ms'] for t in live_trades)
    history_midpoint = statistics.median(t['entry_ms'] for t in candidates)
    seeds = list(range(7, 27))
    live = analyse_stream(live_trades, live_midpoint)
    streams = [analyse_stream(sample_daily(candidates, seed), history_midpoint, seed) for seed in seeds]
    ensemble = aggregate_streams(streams)
    inventory = test_inventory(live, streams, ensemble)
    meta = dict(live=live_meta, history=history_meta, seeds=seeds, daily_entry_cap=8,
                history_midpoint_ms=history_midpoint,
                generated_utc=dt.datetime.now(dt.timezone.utc).isoformat(),
                command=' '.join([sys.executable, '-m', 'scripts.backtest.breaker_state_study', *sys.argv[1:]]),
                scope='observed_and_backtrader_labels_static_conditioning_only',
                state_missing_r_policy='exclude_from_both_state_and_outcomes',
                loss_window='(entry_ms-H*3600000, entry_ms]',
                day_window='[UTC_midnight, entry_ms]',
                live_r='realized_pnl / (entry_quantity_or_quantity * abs(entry_price-initial_stop_loss))',
                fee_policy='live_realized_pnl_as_given; historical_backtrader_net',
                drawdown_order='exit_ms,id', half_state_policy='carry_full_stream_history',
                counterfactual='static_exclusion_without_state_replay')
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    report = report_text(meta, live, streams, ensemble, inventory)
    (out / 'report.txt').write_text(report, encoding='utf-8')
    atomic_json(out / 'analysis.json', json_safe(dict(meta=meta, tests=inventory, live=live,
                history=dict(streams=streams, ensemble=ensemble),
                conclusions=conclusions(live, ensemble, inventory['bonferroni_alpha']))))
    print('\n'.join(report.splitlines()[:12] + ['...', *report.splitlines()[-3:]]))
    print(f'Written {out / "report.txt"} and {out / "analysis.json"}')


if __name__ == '__main__':
    main()
