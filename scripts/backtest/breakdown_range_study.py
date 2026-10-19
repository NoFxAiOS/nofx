"""Offline range/chasing-low breakdown-short study; execution is Backtrader only.

Run as ``python -m scripts.backtest.breakdown_range_study`` in the project venv.
Reuses breakdown_study's causal windows, data loader, statistics and checkpoints.
"""
import argparse
import collections
import datetime as dt
import hashlib
import json
import math
from pathlib import Path
import sys
import time
from types import SimpleNamespace

from . import breakdown_study as bd

M5, HOUR, DAY = bd.M5, bd.HOUR, bd.DAY
VARIANTS = {'PB05': (0.5, 24), 'PB10': (1.0, 48)}


def decision_features(ctx, btc, job):
    """No new ATR estimate: use the original signal's ref and atr_pct."""
    t = job['t_ms']
    bk = bd.closed_window(btc.a4, t, 18, 4 * HOUR)
    k1 = bd.closed_window(ctx.a1, t, 24, HOUR)
    if bk is None or k1 is None:
        raise ValueError(f"Incomplete closed feature window: {job['sym']} @ {t}")
    assert all(int(x) <= t for a in (bk, k1) for x in a['t_close']), 'future feature bar'
    atr = job['atr_pct'] / 100 * job['ref']
    if not math.isfinite(atr) or atr <= 0:
        raise ValueError('ATR must be positive and finite')
    return dict(btc_range72=float((max(bk['h']) - min(bk['l'])) / bk['c'][-1] * 100),
                chase_low_atr=float((job['ref'] - min(k1['l'])) / atr))


def causal_check(ctx, btc, job):
    """Truncate raw 5m data, rebuild HTFs, and assert identical features."""
    def prefix(c):
        a = c.a5
        return [[int(t), *[float(a[k][i]) for k in ('o', 'h', 'l', 'c')]]
                for i, t in enumerate(a['t_open']) if int(a['t_close'][i]) <= job['t_ms']]
    clipped = [bd.Context.from_bars(prefix(c)) for c in (ctx, btc)]
    assert decision_features(ctx, btc, job) == decision_features(*clipped, job), 'future changed features'


def range_bucket(value, cuts):
    """Ties stay together: low < q1, middle [q1,q2), high >= q2."""
    return '低' if value < cuts[0] else '中' if value < cuts[1] else '高'


def chase_bucket(value):
    return '<0.5' if value < 0.5 else '0.5–1.5' if value < 1.5 else '>=1.5'


def paired_difference(jobs, variant, market, indices):
    """Same signal pairs; genuine unfilled orders contribute zero R.

    Feeding both arms to the shared-week influence function retains covariance
    and is algebraically the cluster SE of the within-signal differences.
    """
    def zero_unfilled(rows):
        out = []
        for r in rows:
            if r['r'] is None and r['how'] != 'unfilled':
                raise ValueError(f"Cannot zero an invalid result: {r}")
            out.append(dict(r, r=0.0 if r['r'] is None else r['r']))
        return out
    return bd.mean_difference(jobs, zero_unfilled(variant), indices,
                              jobs, zero_unfilled(market), indices)


def inference(d):
    if d is None or d['cluster_se'] is None:
        return '无法检验（空组或不足两周）'
    delta, se = d['diff'], d['cluster_se']
    p = math.erfc(abs(delta) / (math.sqrt(2) * se)) if se else (1.0 if delta == 0 else 0.0)
    verdict = '差异不显著'
    if abs(delta) > 1.96 * se:
        verdict = ('正' if delta > 0 else '负') + '差异名义显著（未校正多重比较）'
    return (f"ΔR={delta:+.5f} 周聚类SE={se:.5f} weeks={d['weeks']} "
            f"95%CI=[{delta-1.96*se:+.5f},{delta+1.96*se:+.5f}] p≈{p:.5g}；{verdict}")


def group_definitions(jobs, features):
    """(label, membership, comparison universe); cross rows compare within regime."""
    all_ix = set(range(len(jobs)))
    groups = []
    for bucket in ('低', '中', '高'):
        groups.append(('range三分位/' + bucket,
                       {i for i, f in enumerate(features) if f['btc_range72_bucket'] == bucket}, all_ix))
    for narrow in (True, False):
        groups.append(('range<3%' if narrow else 'range>=3%',
                       {i for i, f in enumerate(features) if (f['btc_range72'] < 3) == narrow}, all_ix))
    for bucket in ('<0.5', '0.5–1.5', '>=1.5'):
        groups.append(('chase/' + bucket,
                       {i for i, f in enumerate(features) if chase_bucket(f['chase_low_atr']) == bucket}, all_ix))
    for regime in sorted({j['regime'] for j in jobs}):
        parent = {i for i, j in enumerate(jobs) if j['regime'] == regime}
        for narrow in (True, False):
            groups.append((regime + ('×range<3%' if narrow else '×range>=3%'),
                           {i for i in parent if (features[i]['btc_range72'] < 3) == narrow}, parent))
    groups.append(('range<3%∧chase<0.5',
                   {i for i, f in enumerate(features) if f['btc_range72'] < 3 and f['chase_low_atr'] < 0.5}, all_ix))
    return groups


def analyse_a(jobs, net, features, primary_overlap=False):
    import numpy as np
    from .ict_bt import stats
    groups = group_definitions(jobs, features)
    rows, tests, samples = [], [], []
    for allow in (primary_overlap, not primary_overlap):
        for sample in ('BASELINE', 'NEW>=50'):
            eligible = [i for i, j in enumerate(jobs) if sample == 'BASELINE' or j['new'] >= 50]
            kept, blocked = bd.kept_indices(jobs, net, allow, eligible)
            midpoint = float(np.median([jobs[i]['t_ms'] for i in kept])) if kept else 0
            samples.append(dict(sample=sample, allow_overlap=allow, eligible=len(eligible),
                                kept=len(kept), blocked=blocked, midpoint_ms=midpoint))
            for name, members, parent in [('整体', set(kept), set(kept)), *groups]:
                ix = [i for i in kept if i in members]
                values = [net[i]['r'] for i in ix]
                halves = [[net[i]['r'] for i in ix if (jobs[i]['t_ms'] <= midpoint) == early]
                          for early in (True, False)]
                row = dict(sample=sample, allow_overlap=allow, group=name, stats=stats(values),
                           half_n=[len(h) for h in halves],
                           half_avg=[stats(h)['avg'] if h else None for h in halves],
                           trim5_avg=stats(sorted(values)[:-5])['avg'] if len(values) > 5 else None)
                if name != '整体':
                    complement = [i for i in kept if i in parent and i not in members]
                    row['difference'] = bd.mean_difference(jobs, net, ix, jobs, net, complement)
                    row['complement_n'] = len(complement)
                    tests.append(dict(part='A', sample=sample, allow_overlap=allow, group=name,
                                      difference=row['difference']))
                rows.append(row)
    return dict(rows=rows, tests=tests, samples=samples)


def pullback_job(job, name):
    mult, valid = VARIANTS[name]
    atr = job['atr_pct'] / 100 * job['ref']
    limit, risk = job['ref'] + mult * atr, 2 * atr
    return dict(job, kind='lim', ref=limit, sl=limit + risk, tp=limit - 2 * risk,
                valid_bars=valid, atr_pct=atr / limit * 100, key=[*job['key'], name])


def assert_limit_result(job, result):
    if result['r'] is None:
        assert result['how'] == 'unfilled', result
        assert result.get('entry_ms') is None, result
        return
    assert math.isfinite(result['r']), result
    # Broker's clock is bar-close time; entry_ms records that bar's open.
    assert job['t_ms'] <= result['entry_ms'], 'fill predates limit window'
    assert result['entry_ms'] + M5 <= job['t_ms'] + job['valid_bars'] * M5, 'fill after expiry'
    assert result['how'] not in ('open', 'rejected'), 'incomplete/invalid simulation'
    assert result['exit_ms'] is not None, result
    assert result['entry_ms'] < result['exit_ms'] <= result['entry_ms'] + bd.HORIZON * HOUR + 2*M5, result


def run_limit_checkpoint(jobs, args, label, signature):
    """Extend bd.run_checkpoint's atomic, signature-checked batch protocol to limits.

    Its market-only entry_ms==t assertion cannot be applied to deferred fills.
    Only the orchestration differs: all execution still calls ict_bt.run_jobs.
    """
    from . import ict_bt
    path = Path(args.out) / f'{label}.json'
    digest = hashlib.sha256((signature + json.dumps(jobs, sort_keys=True)).encode()).hexdigest()
    results = []
    if path.exists():
        saved = json.loads(path.read_text())
        if saved['signature'] == digest:
            results = saved['results']
    assert len(results) <= len(jobs), 'oversized checkpoint'
    for j, r in zip(jobs, results):
        assert_limit_result(j, r)
    started = time.monotonic()
    feed_hours = bd.HORIZON + max(j['valid_bars'] for j in jobs) / 12 if jobs else bd.HORIZON
    for offset in range(len(results), len(jobs), 500):
        batch = jobs[offset:offset+500]
        res = ict_bt.run_jobs(batch, ict_bt.plain_rules(horizon_h=bd.HORIZON),
                              horizon_h=feed_hours, procs=args.procs)
        assert len(res) == len(batch)
        for j, r in zip(batch, res):
            assert_limit_result(j, r)
        results.extend(res)
        bd.atomic_json(path, dict(signature=digest, results=results))
        print(f'{label}: {len(results)}/{len(jobs)}, elapsed={time.monotonic()-started:.1f}s', flush=True)
    if not path.exists():
        bd.atomic_json(path, dict(signature=digest, results=results))
    return results


def analyse_b(jobs, market, features, variants):
    from .ict_bt import stats
    scopes = [('全样本', list(range(len(jobs)))),
              ('range<3%', [i for i, f in enumerate(features) if f['btc_range72'] < 3]),
              ('range>=3%', [i for i, f in enumerate(features) if f['btc_range72'] >= 3])]
    rows, sensitivity, tests = [], [], []
    for name, results in {'MKT': market, **variants}.items():
        kept, blocked = bd.kept_indices(jobs, results, False)
        admitted = set(kept)
        for scope, ix in scopes:
            filled = [i for i in ix if results[i]['r'] is not None]
            signal_r = [results[i]['r'] if results[i]['r'] is not None else 0.0 for i in ix]
            row = dict(variant=name, scope=scope, signals=len(ix), fills=len(filled),
                       fill_rate=len(filled)/len(ix) if ix else None,
                       filled_stats=stats([results[i]['r'] for i in filled]), signal_stats=stats(signal_r))
            if name != 'MKT':
                row['paired'] = paired_difference(jobs, results, market, ix)
                missed = [i for i in ix if results[i]['r'] is None]
                row['unfilled_mkt_sum'] = sum(market[i]['r'] for i in missed)
                row['unfilled_mkt_avg'] = stats([market[i]['r'] for i in missed])['avg'] if missed else None
                tests.append(dict(part='B', variant=name, scope=scope, difference=row['paired']))
            rows.append(row)
            accepted = [i for i in ix if i in admitted]
            sensitivity.append(dict(variant=name, scope=scope, signals=len(ix), accepted=len(accepted),
                                    blocked=len(filled)-len(accepted),
                                    filled_stats=stats([results[i]['r'] for i in accepted]),
                                    signal_stats=stats([results[i]['r'] if i in admitted else 0.0 for i in ix])))
        assert sensitivity[-3]['blocked'] == sensitivity[-2]['blocked'] + sensitivity[-1]['blocked'] == blocked
    return dict(rows=rows, sensitivity=sensitivity, tests=tests,
                how={name: dict(collections.Counter(r['how'] for r in rr))
                     for name, rr in {'MKT': market, **variants}.items()})


def conclusions(a, b, allow):
    def lookup(sample, group):
        return next(r['difference'] for r in a['rows'] if r['sample'] == sample
                    and r['allow_overlap'] == allow and r['group'] == group)
    def worse(d):
        return d is not None and d['cluster_se'] is not None and d['diff'] < -1.96*d['cluster_se']
    lines = []
    for title, group in (('区间期是否显著更差', 'range<3%'), ('追下沿是否显著更差', 'chase/<0.5')):
        nd, base = lookup('NEW>=50', group), lookup('BASELINE', group)
        decision = ('NEW>=50名义显著更差（未校正多重比较）' if worse(nd)
                    else '无法证明NEW>=50显著更差')
        base_text = 'BASELINE名义显著更差' if worse(base) else 'BASELINE无法证明显著更差'
        deltas = '/'.join(f"{d['diff']:+.4f}" if d else 'NA' for d in (nd, base))
        lines.append(f'{title}？{decision}；{base_text}；ΔR(NEW/BASELINE)={deltas}。')
    if b is None:
        lines.append('回抽确认是否在按信号计的口径下显著更好？未运行B部分，无法证明。')
    else:
        significant = [f"{t['variant']}({t['scope']})" for t in b['tests']
                       if t['difference'] is not None and t['difference']['cluster_se'] is not None
                       and t['difference']['diff'] > 1.96*t['difference']['cluster_se']]
        answer = ('、'.join(significant) + '名义显著更好（未校正多重比较）；其他口径无法证明'
                  if significant else 'PB05/PB10在三个口径均无法证明显著更好')
        lines.append('回抽确认是否在按信号计的口径下显著更好？' + answer + '。')
    return lines


def stamp(t):
    return dt.datetime.fromtimestamp(t/1000, dt.timezone.utc).strftime('%Y-%m-%d %H:%MZ')


def number(x):
    return 'NA' if x is None else f'{x:+.5f}'


def write_report(args, study, jobs, features, cuts, data_meta, a, b, consistency):
    from .ict_bt import fmt
    tests = a['tests'] + (b['tests'] if b else [])
    estimable = sum(t['difference'] is not None and t['difference']['cluster_se'] is not None for t in tests)
    lines = [
        'T7 区间震荡期破位做空与回抽确认（离线，Backtrader）',
        f"数据：决策 {stamp(min(j['t_ms'] for j in jobs))} ～ {stamp(max(j['t_ms'] for j in jobs))}；"
        f"5m缓存覆盖 {stamp(data_meta['first_ms'])} ～ {stamp(data_meta['last_ms'])}（各币可用起点不同）。",
        f"样本：BASELINE={len(jobs)}，NEW>=50={sum(j['new'] >= 50 for j in jobs)}，币种={len(study['meta']['selected'])}；"
        + ('B使用全部NEW>=50，无抽样。' if b else '当前报告仅含A部分。'),
        '特征：btc_range72=(最近18根已收盘4h的最高high−最低low)/最后close×100%；固定先验<3%，并列报三个全jobs分位桶。',
        f'三分位边界：q1={cuts[0]:.6f}%，q2={cuts[1]:.6f}%；低<q1，中[q1,q2)，高>=q2；并列值不拆分，分位数仅作全样本描述，不是可部署训练阈值。',
        'chase_low_atr=(原ref−该币最近24根已收盘1h最低low)/(原atr_pct/100×原ref)；桶<0.5、[0.5,1.5)、>=1.5；负值归<0.5；down_ext/chg24沿用原job。',
        f"全部特征窗口close_time<=t；每币首个信号原始5m截断后重采样重算断言通过={data_meta['causal_checks']}；所有job决策idx/ref匹配缓存。",
        f'检验总数={len(tests)}（A={len(a["tests"])}，B={len(b["tests"]) if b else 0}；可估计周聚类SE={estimable}；含互为补集的重复方向检验）。',
        '多重比较警告：全部p值/95%CI采用正态近似，名义显著（未校正多重比较）；反向重复、相关分桶、样本切分均计入总数，不把这些表当独立验证。需独立样本复验。',
        '差异检验复用mean_difference，以UTC周（周一起始）聚类，保留共用交易/共用市场周的协方差；周际依赖仍可能存在。avgR±SE表内为每交易IID SE，显著性只用周聚类SE。',
        'A：每策略先对完整信号队列调用kept_indices做同币同向单仓过滤，再切特征；交叉组对比同regime内补集，其他组对比该策略整体补集。',
        '前/后半段以该策略保留队列t_ms中位数切分（等于中位数属前半），所有子桶用相同日历分界；去最大5笔只在n>5时计算。',
        f"主口径={'允许重叠 (--allow-overlap)' if args.allow_overlap else '单仓'}；另一口径为敏感性表。",
        f"源study={args.study_dir}；源net签名={data_meta['net_signature']}；数据SHA256={data_meta['data_sha256']}。",
        '费用0.05%/边；市价/止损滑点0.03%，限价无额外滑点；R沿用原生broker扣费净盈亏/实际初始SL风险。',
    ]
    for sample in a['samples']:
        lines.append(f"\nA {'允许重叠 (--allow-overlap)' if sample['allow_overlap'] else '单仓'} {sample['sample']}："
                     f"eligible={sample['eligible']} kept={sample['kept']} blocked={sample['blocked']}；时间分界={stamp(sample['midpoint_ms'])}")
        for row in a['rows']:
            if (row['sample'], row['allow_overlap']) != (sample['sample'], sample['allow_overlap']):
                continue
            half = '/'.join(number(x) for x in row['half_avg'])
            lines.append(fmt(row['group'], row['stats'], 26)
                         + f" 前/后avgR={half} n={row['half_n']} 去最大5笔avgR={number(row['trim5_avg'])}")
            if 'difference' in row:
                lines.append(f"  对补集n={row['complement_n']}：{inference(row['difference'])}")
    if b is None:
        lines.append('\nB：--skip-sim，未运行。')
    else:
        lines.extend([
            '\nB 回抽确认：全部NEW>=50信号，无重叠过滤，未成交=0R；同一job与MKT成对差异。',
            'PB05=原ref+0.5ATR、有效24根5m（2h）；PB10=原ref+1ATR、有效48根5m（4h）；idx保持决策K线。',
            '委托几何risk=2ATR，SL=limit+risk，TP=limit−2risk；跳空改善成交价时保留原生固定SL/TP并按实际初始风险归一，不伪造成交价或重写账务。',
            '成交时刻断言：t<=entry_ms且entry_ms+5m<=t+valid_bars×5m；数据窗口延长等待期，持仓仍为成交后72h；无open/rejected混入0R。',
            f"MKT适配器一致性：跨全时段等距取{consistency['n']}个NEW信号，复用bd.run_checkpoint重跑，结果所有字段与net.json完全一致={consistency['exact']}。",
            '未成交是要求回抽的代价，可能错过单边下跌；按信号sumR/avgR计入全部未成交的0R，另列未成交信号原MKT表现。',
        ])
        for row in b['rows']:
            s, f = row['signal_stats'], row['filled_stats']
            rate = f"{row['fill_rate']:.2%}" if row['fill_rate'] is not None else 'NA'
            lines.append(f"{row['scope']:12s} {row['variant']:5s} 信号n={row['signals']} 成交n={row['fills']} 成交率={rate} "
                         f"成交后avgR={f['avg']:+.5f}±{f['se']:.5f} 按信号sumR={s['sum']:+.4f} avgR={s['avg']:+.5f}±{s['se']:.5f}")
            if 'paired' in row:
                lines.append('  配对−MKT：' + inference(row['paired']))
                lines.append(f"  未成交信号的MKT sumR={row['unfilled_mkt_sum']:+.4f} avgR={number(row['unfilled_mkt_avg'])}")
        lines.append('\nB 单仓敏感性：每变体在全NEW队列独立kept_indices后切组；未成交不占持仓，待成交委托不另设互斥。')
        lines.append('按原始信号n计，blocked与unfilled均计0R；不同队列不作成对显著性检验，成交率见上方无过滤主表。')
        for row in b['sensitivity']:
            s = row['signal_stats']
            lines.append(fmt(row['scope'] + ' ' + row['variant'], row['filled_stats'], 26)
                         + f" 原信号n={row['signals']} blocked={row['blocked']} 按原信号sumR={s['sum']:+.4f} avgR={s['avg']:+.5f}")
        lines.append(f"Backtrader结果类型={b['how']}")
    lines.extend([
        '\n边界：固定事后币池存在存活偏差；独立等风险信号研究，非共享账户收益；无资金费率/保证金/组合仓位约束；5m OHLC无法还原根内路径，开仓当根的保护单不能回溯。',
        f"本数据决策结束于{stamp(max(j['t_ms'] for j in jobs))}；此处检验历史场景，不能解释10-19实盘案例的全部亏损。",
        f"命令：{sys.executable} -m scripts.backtest.breakdown_range_study {' '.join(sys.argv[1:])}",
        '',
        *conclusions(a, b, args.allow_overlap),
    ])
    (Path(args.out) / 'report.txt').write_text('\n'.join(lines) + '\n', encoding='utf-8')
    print('\n'.join(lines[:10] + ['...', *lines[-3:]]), flush=True)
    return tests


def json_safe(obj):
    if isinstance(obj, float) and not math.isfinite(obj):
        return None
    if isinstance(obj, dict):
        return {k: json_safe(v) for k, v in obj.items()}
    if isinstance(obj, (tuple, list)):
        return [json_safe(v) for v in obj]
    return obj


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--study-dir', default=str(bd.SCRATCH / 'outBD'))
    ap.add_argument('--cache', default='/Users/zhangyun/workspace/nofx/data/backtest_cache')
    ap.add_argument('--out', default=str(Path(__file__).parent / '_out_range'))
    ap.add_argument('--procs', type=int, default=9)
    ap.add_argument('--skip-sim', action='store_true', help='only part A; do not execute new orders')
    ap.add_argument('--allow-overlap', action='store_true', help='make overlap-allowed table primary; always show both')
    args = ap.parse_args()
    if args.procs < 1:
        ap.error('--procs must be positive')
    import numpy as np
    from . import ict_bt
    out, source = Path(args.out), Path(args.study_dir)
    out.mkdir(parents=True, exist_ok=True)
    study_bytes, net_bytes = (source / 'study.json').read_bytes(), (source / 'net.json').read_bytes()
    study, saved = json.loads(study_bytes), json.loads(net_bytes)
    jobs, net = study['jobs'], saved['results']
    assert len(jobs) == len(net) and jobs, 'source net/study mismatch'
    assert all(r['r'] is not None and math.isfinite(r['r']) and r['entry_ms'] == j['t_ms']
               and r['how'] not in ('open', 'invalid_fill', 'rejected') for j, r in zip(jobs, net))
    universe_file = out / 'universe_input.json'
    bd.atomic_json(universe_file, {'source': study['meta']['selected']})
    ctxs, syms, data_meta = bd.load_data(SimpleNamespace(cache=args.cache, universe='source',
                                        universe_json=universe_file, sample=0))
    assert syms == study['meta']['selected'], 'cache universe changed'
    assert data_meta['data_sha256'] == study['meta']['data_sha256'], 'cache changed since source simulation'
    features, checked = [], set()
    for job in jobs:
        ctx = ctxs[job['sym']]
        assert int(ctx.a5['t_close'][job['idx']]) == job['t_ms'], 'idx/cache mismatch'
        assert float(ctx.a5['o'][job['idx']+1]) == job['ref'], 'ref/cache mismatch'
        features.append(decision_features(ctx, ctxs['BTCUSDT'], job))
        if job['sym'] not in checked:
            causal_check(ctx, ctxs['BTCUSDT'], job)
            checked.add(job['sym'])
    cuts = np.quantile([f['btc_range72'] for f in features], [1/3, 2/3]).tolist()
    for f, job in zip(features, jobs):
        f.update(sym=job['sym'], t_ms=job['t_ms'], btc_range72_bucket=range_bucket(f['btc_range72'], cuts),
                 down_ext=job['down_ext'], chg24=job['chg24'])
    data_meta.update(causal_checks=len(checked), net_signature=saved['signature'],
                     first_ms=min(int(c.a5['t_open'][0]) for c in ctxs.values()),
                     last_ms=max(int(c.a5['t_close'][-1]) for c in ctxs.values()))
    bd.atomic_json(out / 'features.json', dict(cuts=cuts, meta=data_meta, features=features))
    a = analyse_a(jobs, net, features, args.allow_overlap)
    write_report(args, study, jobs, features, cuts, data_meta, a, None, None)
    b, consistency = None, None
    if not args.skip_sim:
        signature = hashlib.sha256(study_bytes + net_bytes)
        for name in ('breakdown_range_study.py', 'breakdown_study.py', 'ict_bt.py', 'backtrader_adapter.py', 'params.py'):
            signature.update(Path(__file__).with_name(name).read_bytes())
        signature.update(data_meta['data_sha256'].encode())
        digest = signature.hexdigest()
        ix = [i for i, j in enumerate(jobs) if j['new'] >= 50]
        bj, bm, bf = [jobs[i] for i in ix], [net[i] for i in ix], [features[i] for i in ix]
        assert bj, 'no NEW>=50 signals'
        check_ix = np.linspace(0, len(bj)-1, min(50, len(bj)), dtype=int).tolist()
        rr = bd.run_checkpoint([bj[i] for i in check_ix], args, 'mkt_check50', digest)
        assert rr == [bm[i] for i in check_ix], 'MKT rerun differs from source net.json'
        consistency = dict(n=len(check_ix), exact=True, source_indices=[ix[i] for i in check_ix])
        bd.atomic_json(out / 'adapter_consistency.json', consistency)
        variants = {}
        for name in VARIANTS:
            js = [pullback_job(j, name) for j in bj]
            for j in js:
                assert int(ict_bt.DATA[j['sym']]['t_close'][-1]) >= j['t_ms'] + bd.HORIZON*HOUR + j['valid_bars']*M5, 'insufficient 72h+waiting coverage'
            variants[name] = run_limit_checkpoint(js, args, name, digest)
        b = analyse_b(bj, bm, bf, variants)
    tests = write_report(args, study, jobs, features, cuts, data_meta, a, b, consistency)
    bd.atomic_json(out / 'analysis.json', json_safe(dict(meta=data_meta, cuts=cuts, a=a, b=b,
                   consistency=consistency, tests=tests, conclusions=conclusions(a, b, args.allow_overlap))))


if __name__ == '__main__':
    main()
