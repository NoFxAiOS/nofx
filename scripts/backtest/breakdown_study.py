"""Historical breakdown-short score study, using the existing Backtrader engine.

Run with the backtest virtualenv (numpy/backtrader); --help needs only Python.
All timestamps are UTC, in milliseconds. No network or database access is used.
"""
import argparse
import collections
import datetime as dt
import hashlib
import json
import math
from pathlib import Path
import random
import shutil
import sys
import time
from dataclasses import dataclass

M5, HOUR, DAY = 300_000, 3_600_000, 86_400_000
HORIZON = 72
SCRATCH = Path("/private/tmp/claude-501/-Users-zhangyun-workspace-nofx--claude-worktrees-youthful-bassi-0f1445/8940c3f5-4aed-433b-a443-612abfb892af/scratchpad/ict")
NOTES = Path("/Users/zhangyun/Library/Application Support/Claude/side-session-notes/local_461050eb-0464-42a1-be93-dc653e1a6e70/breakdown_study_report.txt")


def ema(xs, period):
    """Go indicators.go: first-value seed, including on each fetched window."""
    if not len(xs):
        return []
    out, k = [float(xs[0])], 2.0 / (period + 1.0)
    for x in xs[1:]:
        out.append(float(x) * k + out[-1] * (1 - k))
    return out


def atr(h, l, c, period=14):
    """Go's expanding-mean seed followed by Wilder smoothing (not ict.atr)."""
    out = []
    for i in range(len(c)):
        tr = float(h[i] - l[i])
        if i:
            tr = max(tr, abs(float(h[i] - c[i-1])), abs(float(l[i] - c[i-1])))
        if not i:
            out.append(tr)
        elif i < period:
            out.append(out[-1] + (tr - out[-1]) / (i + 1))
        else:
            out.append((out[-1] * (period - 1) + tr) / period)
    return out


def rsi_series(c, period=14):
    """Go shortscan.rsiSeries; warmup entries deliberately remain zero."""
    if len(c) < period + 1:
        return []
    out = [0.0] * len(c)
    gains = sum(max(float(c[i] - c[i-1]), 0) for i in range(1, period + 1))
    losses = sum(max(float(c[i-1] - c[i]), 0) for i in range(1, period + 1))
    ag, al = gains / period, losses / period
    for i in range(period, len(c)):
        if i > period:
            d = float(c[i] - c[i-1])
            ag = (ag * (period - 1) + max(d, 0)) / period
            al = (al * (period - 1) + max(-d, 0)) / period
        out[i] = 100.0 if al == 0 else 100 - 100 / (1 + ag / al)
    return out


def sigmoid(x, center, width):
    z = (x - center) / (width if width > 0 else 1)
    if z >= 0:
        return 100 / (1 + math.exp(-z))
    e = math.exp(z)
    return 100 * e / (1 + e)


def round2(x):
    """math.Round in Go rounds halves away from zero, unlike Python round."""
    return math.copysign(math.floor(abs(x) * 100 + 0.5) / 100, x)


def continuation(k1, k4):
    """Line-for-line predicates and index bounds of breakdownContinuation."""
    c, n = k1['c'], len(k1['c'])
    if n < 30 or len(k4['c']) < 12:
        return 0.0, False
    e20, last = ema(c, 20), n - 1
    bounce = 0.0
    if c[last] < e20[last]:
        for i in range(n - 13, last):
            if k1['h'][i] >= e20[i] and c[i] < e20[i]:
                bounce = 100.0
                break
    prev_min = min(k1['l'][n-25:last])
    new_low = 100.0 if c[last] < prev_min else (60.0 if k1['l'][last] < prev_min else 0.0)
    lower_high = 100.0 if max(k4['h'][-6:]) < max(k4['h'][-12:-6]) else 0.0
    return 0.45 * bounce + 0.30 * new_low + 0.25 * lower_high, bounce == 100 or new_low == 100


def btc_regime(k4):
    c = k4['c']
    if len(c) < 60:
        return 'unknown'
    e20, e50, rsi = ema(c, 20)[-1], ema(c, 50)[-1], rsi_series(c)[-1]
    if e20 > e50 and c[-1] > e20 and rsi >= 60:
        return 'btc_bull'
    if e20 < e50 and c[-1] < e20:
        return 'btc_bear'
    return 'chop'


def score_windows(k1, k4, btc4):
    """Inputs are already-closed 60/84-bar windows, oldest first."""
    c1, c4 = k1['c'], k4['c']
    if len(c1) < 30 or len(c4) < 12:
        return None
    e1, e4, e50 = ema(c1, 20), ema(c4, 20), ema(c4, 50)
    gate = e4[-1] < e50[-1] and c4[-1] < e4[-1]
    down_ext = max(0.0, (e4[-1] - c4[-1]) / e4[-1] * 100)
    rej = 0.0
    for i in range(len(c1) - 3, len(c1)):
        span = k1['h'][i] - k1['l'][i]
        if span > 0:
            rej += (k1['h'][i] - max(k1['o'][i], c1[i])) / span
    rejection = sigmoid(rej / 3, 0.35, 0.15)
    cont, printed = continuation(k1, k4)
    outperform = 0.0
    if len(c4) >= 61 and len(btc4['c']) >= 56:
        bc = btc4['c']
        recent5d = (c4[-1] / c4[-31] - 1) * 100
        btc5d = (bc[-1] / bc[-31] - 1) * 100
        outperform = round2(recent5d - btc5d)
    rel_weak = sigmoid(-outperform, 0, 8) if len(btc4['c']) >= 56 else 50.0
    fake = False
    if len(c1) >= 53:
        for i in range(len(c1) - 5, len(c1)):
            swing = max(k1['h'][i-48:i])
            if swing > 0 and k1['h'][i] > swing and c1[i] < swing and c1[-1] < swing:
                fake = True
                break
    ma_break = len(e1) >= 5 and c1[-1] < e1[-1] and c1[-5] > e1[-5]
    structure = 100.0 if fake else (80.0 if ma_break else 0.0)
    divergence = 0.0
    if len(c4) >= 30:
        rs = rsi_series(c4)
        ri = max(range(len(c4) - 10, len(c4)), key=lambda i: k4['h'][i])
        pi = max(range(len(c4) - 30, len(c4) - 10), key=lambda i: k4['h'][i])
        recent, prior = k4['h'][ri], k4['h'][pi]
        div = recent > prior and rs[ri] < rs[pi] - 2
        divergence = 100.0 if div else (40.0 if prior > 0 and recent >= prior * 0.995 else 0.0)
    new = 0.35*cont + 0.25*sigmoid(-down_ext, -3, 1.5) + 0.15*rejection + 0.10*rel_weak + 0.15*50
    old = 0.30*structure + 0.25*sigmoid(-down_ext, -1.5, 4) + 0.15*rejection + 0.15*divergence + 0.15*50
    return dict(new=round2(new), old=round2(old), gate=bool(gate), continuation=cont,
                printed=printed, down_ext=float(down_ext), rejection=rejection,
                rel_weak=rel_weak, btc_outperform=outperform, structure=structure,
                divergence=divergence, atr=atr(k1['h'], k1['l'], c1)[-1])


def closed_window(a, t, limit, duration, complete=True):
    import numpy as np
    n = int(np.searchsorted(a['t_close'], t, side='right'))
    if n < limit:
        return None
    s = n - limit
    if complete and (int(a['t_close'][n-1]) != t or
                     int(a['t_close'][s]) != t - (limit - 1) * duration):
        return None
    window = {k: v[s:n] for k, v in a.items()}
    assert all(int(x) <= t for x in window['t_close']), 'future bar in decision'
    return window


@dataclass
class Context:
    a5: dict
    a1: dict
    a4: dict

    @classmethod
    def from_bars(cls, bars):
        from . import ict
        return cls(ict.load_arrays(bars), ict.resample(bars, 60), ict.resample(bars, 240))


def snapshot(ctx, btc, t):
    k1 = closed_window(ctx.a1, t, 60, HOUR)
    k4 = closed_window(ctx.a4, t, 84, 4 * HOUR)
    bk = closed_window(btc.a4, t, 84, 4 * HOUR)
    if k1 is None or k4 is None or bk is None:
        return None
    return dict(score_windows(k1, k4, bk), regime=btc_regime(bk))


def causal_check(ctx, btc, t):
    """Rebuild from 5m prefixes, including resampling, then rescore at t."""
    def prefix(c):
        a = c.a5
        return [[int(a['t_open'][i]), *[float(a[k][i]) for k in ('o', 'h', 'l', 'c')]]
                for i in range(len(a['c'])) if int(a['t_close'][i]) <= t]
    full = snapshot(ctx, btc, t)
    clipped = snapshot(Context.from_bars(prefix(ctx)), Context.from_bars(prefix(btc)), t)
    assert full is not None and full == clipped, 'truncation changed decision score'


def make_job(sym, ctx, t, scores):
    import numpy as np
    a = ctx.a5
    i = int(np.searchsorted(a['t_close'], t, side='right')) - 1
    if i < 0 or i + 1 >= len(a['c']) or int(a['t_close'][i]) != t or int(a['t_open'][i+1]) != t:
        return None
    if int(a['t_close'][-1]) < t + (HORIZON + 2.5) * HOUR:
        return None
    # The next bar opens at t. Only this open supplies the order reference;
    # its H/L/C never enter the score or ATR. Backtrader determines the fill.
    ref, risk = float(a['o'][i+1]), 2 * scores['atr']
    if risk <= 0 or ref - 2 * risk <= 0:
        return None
    return dict(sym=sym, side='short', kind='mkt', idx=i, t_ms=int(t),
                ref=ref, sl=ref + risk, tp=ref - 2 * risk, atr_pct=scores['atr'] / ref * 100,
                valid_bars=24, key=[sym, 'short', int(t)], **scores)


def control_of(job, ctx, rng):
    """Same geometry, same coin; random signed integer shift of 2..14 days."""
    a = ctx.a5
    import numpy as np
    for _ in range(100):
        t = job['t_ms'] + rng.choice((-1, 1)) * rng.randint(2, 14) * DAY
        i = int(np.searchsorted(a['t_close'], t, side='right')) - 1
        if i < 300 or i+1 >= len(a['c']) or int(a['t_close'][i]) != t or int(a['t_open'][i+1]) != t:
            continue
        if int(a['t_close'][-1]) < t + (HORIZON + 2.5) * HOUR:
            continue
        ref = float(a['o'][i+1])
        slp, tpp = (job['sl'] - job['ref']) / job['ref'], (job['ref'] - job['tp']) / job['ref']
        return dict(job, idx=i, t_ms=t, ref=ref, sl=ref*(1+slp), tp=ref*(1-tpp),
                    control=True, origin_ms=job['t_ms'], key=job['key'] + ['ctl'])
    raise RuntimeError(f"Cannot time-shift control: {job['key']}")


def atomic_json(path, obj):
    tmp = path.with_suffix(path.suffix + '.tmp')
    tmp.write_text(json.dumps(obj, ensure_ascii=False, allow_nan=False), encoding='utf-8')
    tmp.replace(path)


def load_data(args):
    from . import ict_bt, ict_fetch
    import numpy as np
    names = list(dict.fromkeys(json.loads(Path(args.universe_json).read_text())[args.universe]))
    ctxs, dropped, nodata = {}, {}, []
    # Filter before deterministic coin sampling, just as ict_study does.
    eligible = []
    for sym in names:
        rows = ict_fetch.load(args.cache, sym)
        if len(rows) <= 3000:
            nodata.append(sym)
            continue
        ts = np.asarray([r[0] for r in rows], dtype=np.int64)
        gap = float(np.maximum(np.diff(ts) // M5 - 1, 0).sum()) / len(ts)
        if gap > 0.01:
            dropped[sym] = gap
        else:
            eligible.append(sym)
    chosen = eligible
    if args.sample and args.sample < len(eligible):
        chosen = sorted(random.Random(11).sample(eligible, args.sample))
    print(f'universe eligible={len(eligible)} selected={len(chosen)}; gaps>1%={dropped}; no_data={nodata}', flush=True)
    fingerprint = hashlib.sha256()
    for sym in list(dict.fromkeys(chosen + ['BTCUSDT'])):
        rows = ict_fetch.load(args.cache, sym)
        if len(rows) <= 3000:
            raise ValueError('BTCUSDT reference data is required')
        ctx = Context.from_bars(rows)
        if np.any(ctx.a5['l'] <= 0) or np.any(ctx.a5['l'] > np.minimum(ctx.a5['o'], ctx.a5['c'])) or np.any(ctx.a5['h'] < np.maximum(ctx.a5['o'], ctx.a5['c'])):
            raise ValueError(f'invalid OHLC: {sym}')
        fingerprint.update(sym.encode())
        for k in sorted(ctx.a5):
            fingerprint.update(ctx.a5[k].tobytes())
        ctxs[sym] = ctx
        ict_bt.DATA[sym] = ctx.a5
    return ctxs, chosen, dict(eligible=len(eligible), selected=chosen, dropped_gaps=dropped,
                               no_data=nodata, data_sha256=fingerprint.hexdigest())


def build_candidates(ctxs, syms):
    btc = ctxs['BTCUSDT']
    start = (int(btc.a5['t_open'][0]) + 84*4*HOUR + DAY-1) // DAY * DAY
    last = int(btc.a5['t_close'][-1]) - int((HORIZON + 2.5)*HOUR)
    last_day = (last - 20*HOUR) // DAY * DAY
    decisions = list(range(start, last_day + 20*HOUR + 1, 4*HOUR))
    if not decisions:
        raise ValueError('need 14d warmup + complete decision days + 72h exit coverage')
    candidates, jobs, breadth = [], [], []
    checked, unavailable, btc_missing = set(), 0, 0
    for t in decisions:
        if closed_window(btc.a4, t, 84, 4*HOUR) is None:
            btc_missing += 1
            breadth.append(0)
            continue
        board = []
        for sym in syms:
            k = closed_window(ctxs[sym].a1, t, 25, HOUR)
            if k is not None:
                board.append((float((k['c'][-1] / k['c'][-25] - 1) * 100), sym))
        breadth.append(len(board))
        for chg, sym in sorted(board)[:20]:
            s = snapshot(ctxs[sym], btc, t)
            if s is None:
                unavailable += 1
                continue
            if not s['gate']:
                continue
            s['chg24'] = chg
            candidates.append(dict(sym=sym, t_ms=t, **s))
            j = make_job(sym, ctxs[sym], t, s)
            if j:
                jobs.append(j)
            if sym not in checked:
                causal_check(ctxs[sym], btc, t)
                checked.add(sym)
    print(f'decisions={len(decisions)} candidates={len(candidates)} jobs={len(jobs)} causal_checks={len(checked)}', flush=True)
    return candidates, jobs, dict(decisions=decisions, rank_breadth_min=min(breadth),
                                  rank_breadth_max=max(breadth), unready_top20=unavailable,
                                  btc_missing_decisions=btc_missing, causal_checks=len(checked))


def run_checkpoint(jobs, args, label, signature, gross=False):
    """Bounded batches + run_jobs' fork Pool(maxtasksperchild=200)."""
    from . import ict_bt
    path = Path(args.out) / f'{label}.json'
    digest = hashlib.sha256((signature + json.dumps(jobs, sort_keys=True) + str(gross)).encode()).hexdigest()
    results = []
    if path.exists():
        saved = json.loads(path.read_text())
        if saved['signature'] == digest:
            results = saved['results']
    started = time.monotonic()
    for offset in range(len(results), len(jobs), 1000):
        batch = jobs[offset:offset+1000]
        kw = dict(comm=0, slip=0) if gross else {}
        res = ict_bt.run_jobs(batch, ict_bt.plain_rules(horizon_h=HORIZON),
                              horizon_h=HORIZON, procs=args.procs, **kw)
        for j, r in zip(batch, res):
            if r['r'] is not None:
                assert r['entry_ms'] == j['t_ms'], 'market entry was not the next 5m open'
                assert r['exit_ms'] is not None and r['exit_ms'] <= j['t_ms'] + (HORIZON + 2.5)*HOUR, 'insufficient exit coverage'
                assert r['how'] not in ('open', 'invalid_fill', 'rejected'), r
        results.extend(res)
        atomic_json(path, dict(signature=digest, results=results))
        print(f'{label}: {len(results)}/{len(jobs)}, elapsed={time.monotonic()-started:.1f}s', flush=True)
    if not path.exists():
        atomic_json(path, dict(signature=digest, results=results))
    return results


def kept_indices(jobs, res, allow_overlap, indices=None):
    from . import ict_bt
    indices = list(range(len(jobs))) if indices is None else list(indices)
    sub, rr = [jobs[i] for i in indices], [res[i] for i in indices]
    _, kept, blocked = ict_bt.summarize(sub, rr, allow_overlap)
    return [indices[i] for i in kept], blocked


def mean_difference(ja, ra, ia, jb, rb, ib):
    """IID reference SE and shared-calendar-week cluster influence-function SE."""
    from .ict_bt import stats
    sa, sb = stats([ra[i]['r'] for i in ia]), stats([rb[i]['r'] for i in ib])
    if not ia or not ib:
        return None
    influence = collections.defaultdict(float)
    # Monday-aligned UTC weeks (1970-01-01 was Thursday).
    week = lambda t: (t // DAY + 3) // 7
    for j, r, ix, s, sign in ((ja, ra, ia, sa, 1), (jb, rb, ib, sb, -1)):
        for i in ix:
            influence[week(j[i]['t_ms'])] += sign * (r[i]['r'] - s['avg']) / s['n']
    lo, hi = min(influence), max(influence)
    g = hi - lo + 1
    cluster_se = math.sqrt(g / (g - 1) * sum(v*v for v in influence.values())) if g > 1 else None
    return dict(diff=sa['avg'] - sb['avg'], iid_se=math.hypot(sa['se'], sb['se']),
                cluster_se=cluster_se, weeks=g)


def spearman(xs, ys):
    """Average ranks for ties, without scipy or an IID significance claim."""
    def ranks(v):
        order, out = sorted(range(len(v)), key=lambda i: v[i]), [0.0] * len(v)
        k = 0
        while k < len(v):
            end = k + 1
            while end < len(v) and v[order[end]] == v[order[k]]:
                end += 1
            for i in order[k:end]:
                out[i] = (k + end - 1) / 2
            k = end
        return out
    if len(xs) < 2:
        return None
    a, b = ranks(xs), ranks(ys)
    ma, mb = sum(a) / len(a), sum(b) / len(b)
    denom = math.sqrt(sum((x-ma)**2 for x in a) * sum((y-mb)**2 for y in b))
    return sum((x-ma)*(y-mb) for x, y in zip(a, b)) / denom if denom else None


def report(args, meta, candidates, jobs, net, gross, controls, cnet, cgross):
    import numpy as np
    from .ict_bt import stats, fmt
    lines = []
    def emit(s=''):
        print(s, flush=True)
        lines.append(s)
    def stamp(t):
        return dt.datetime.fromtimestamp(t/1000, dt.timezone.utc).isoformat()
    def row(name, jj, rr, gg, ix, blocked=0):
        s = stats([rr[i]['r'] for i in ix])
        gs = stats([gg[i]['r'] for i in ix if gg[i]['r'] is not None])
        emit(fmt(name, s, 24, f'blocked={blocked}') + f" gross_n={gs['n']} gross_avgR={gs['avg']:+.4f}±{gs['se']:.4f} gross_sumR={gs['sum']:+.2f}")
        return s
    def compare(name, ja, ra, ia, jb, rb, ib):
        d = mean_difference(ja, ra, ia, jb, rb, ib)
        if d is None:
            emit(f'{name}: n=0，无法比较')
            return
        cs, delta = d['cluster_se'], d['diff']
        ci = f'[{delta-1.96*cs:+.4f}, {delta+1.96*cs:+.4f}]' if cs is not None else 'NA'
        verdict = '样本不足以估计周聚类SE'
        if cs is not None:
            verdict = ('正差异名义显著（未校正多重比较）' if delta > 1.96*cs else
                       '负差异名义显著（未校正多重比较）' if delta < -1.96*cs else '差异不显著')
        emit(f"{name}: ΔavgR={delta:+.4f} ± IID独立SE {d['iid_se']:.4f}; 周聚类SE={cs}, weeks={d['weeks']}, 95%CI={ci}; {verdict}")
    emit('Breakdown continuation score historical study (Backtrader)')
    emit(f"UTC决策区间 {stamp(meta['decisions'][0])} .. {stamp(meta['decisions'][-1])}; {len(meta['decisions'])}次 / {len(meta['decisions'])/6:.0f}完整UTC日")
    emit(f"币种 {len(meta['selected'])}/{meta['eligible']}；样本币={meta['selected']}")
    emit(f"剔除 gaps>1%: {meta['dropped_gaps']}; 数据不足: {meta['no_data']}")
    emit(f"每次24h排序可用币数范围={meta['rank_breadth_min']}..{meta['rank_breadth_max']}，升序最差20（不足20取全部）；Top20历史不足/窗口有缺口={meta['unready_top20']}，BTC缺失决策={meta['btc_missing_decisions']}")
    emit(f"候选={len(candidates)}，可建交易={len(jobs)}；因果截断重算通过={meta['causal_checks']}币；所有输入 close_time<=t，决策前60根1h/84根4h必须连续且恰在t收盘。")
    emit('NEW/OLD、闸门及BTC状态的EMA/RSI在每次60根1h/84根4h窗口内按Go重新播种；BTC同期30个4h间隔收益，BtcOutperform先按Go round2。')
    emit('历史无资金费率：fundingHealth固定50（两者均贡献7.5分），忽略OLD funding_rollover；无资金费现金流。报告研究原始评分，不施加ScanShorts后续BTC-bull haircut或grade/confirmed过滤。')
    emit('入场=t后下一根5m开盘市价做空，参考价为该开盘；SL=ref+2×Go Wilder ATR14(最近60根1h)，TP=ref−4×ATR，72h初始SL/固定TP，无R-ladder。')
    emit('默认费用0.05%/边+滑点0.03%；gross为comm=0/slip=0重新跑Backtrader，沿用net保留的同一交易队列。滑点使实际成交风险及R略偏离参考2R。')
    emit(f"持仓重叠={'允许' if args.allow_overlap else '各策略独立调用ict_bt.overlap_filter，同币同向单仓'}；分桶/相关/BTC拆分共用该策略全体队列后切组。")
    emit('时间平移对照：seed=7，每个候选同币同方向±2..14整天，保留SL/TP百分比几何；平移后不要求跌幅榜/趋势闸门，分数只用于匹配原信号。每个策略的对照独立过滤重叠。')
    emit('固定事后universe，非历史成分/成交量/存活币重建；排名宽度会随上市历史变化。单一历史样本、单个随机种子，不能外推实盘收益。')
    emit(f"数据SHA256={meta['data_sha256']}; command={meta['command']}")
    emit('\n候选供给（持仓过滤前；每日=每决策×6，零供给时刻计入）')
    for key in ('new', 'old'):
        xs = [c[key] for c in candidates]
        q = np.percentile(xs, [50, 90, 100]) if xs else [float('nan')]*3
        emit(f'{key.upper()} p50={q[0]:.2f} p90={q[1]:.2f} max={q[2]:.2f}')
        for floor in (50, 55):
            n = sum(x >= floor for x in xs)
            rate = n / len(meta['decisions'])
            emit(f'{key.upper()}>={floor}: n={n}, 每决策={rate:.4f}, 每日={rate*6:.4f}')
    base, bb = kept_indices(jobs, net, args.allow_overlap)
    groups = {'NEW>=50': [i for i, j in enumerate(jobs) if j['new'] >= 50],
              'OLD>=50': [i for i, j in enumerate(jobs) if j['old'] >= 50],
              'BASELINE': list(range(len(jobs)))}
    selected, cselected = {}, {}
    emit('\n策略对比（avgR为扣费后；±为每交易IID SE）')
    for name, ix in groups.items():
        selected[name], blocked = kept_indices(jobs, net, args.allow_overlap, ix)
        row(name, jobs, net, gross, selected[name], blocked)
        cselected[name], cb = kept_indices(controls, cnet, args.allow_overlap, ix)
        row('RANDOM matched ' + name, controls, cnet, cgross, cselected[name], cb)
    for name in ('OLD>=50', 'BASELINE'):
        compare('NEW>=50 − ' + name, jobs, net, selected['NEW>=50'], jobs, net, selected[name])
    for name in groups:
        compare(name + ' − matched RANDOM', jobs, net, selected[name], controls, cnet, cselected[name])
    emit('\n分桶（先对BASELINE全体过滤重叠，再按评分切桶，避免每桶另开仓造成口径偏差）')
    for key in ('new', 'old'):
        for name, lo, hi in (('<30', -math.inf, 30), ('30-40', 30, 40), ('40-50', 40, 50), ('50-60', 50, 60), ('>=60', 60, math.inf)):
            ix = [i for i in base if lo <= jobs[i][key] < hi]
            row(key.upper() + ' ' + name, jobs, net, gross, ix)
    emit('\nBTC 4h状态（btc_bull需EMA20>EMA50、close>EMA20且RSI14>=60；btc_bear需EMA20<EMA50、close<EMA20；其余chop）')
    for regime in ('btc_bear', 'chop', 'btc_bull'):
        ni = [i for i in selected['NEW>=50'] if jobs[i]['regime'] == regime]
        bi = [i for i in base if jobs[i]['regime'] == regime]
        row(regime + ' NEW>=50', jobs, net, gross, ni)
        row(regime + ' BASELINE', jobs, net, gross, bi)
        compare(regime + ' NEW − BASELINE', jobs, net, ni, jobs, net, bi)
    emit('\nSpearman（BASELINE相同交易样本、平均秩处理并列，扣费后R）')
    for key in ('new', 'old'):
        rho = spearman([jobs[i][key] for i in base], [net[i]['r'] for i in base])
        emit(f'{key.upper()} vs R: rho={rho}, n={len(base)}')
    emit('\n关键警告：per-trade σ≈1R，se≈σ/√n（实际σ见各组SE×√n）。每交易IID SE及独立两组SE未处理相关性；重叠组用共同UTC周影响函数估算差值SE，考虑共享交易及周内市场相关，周际依赖仍可能存在。')
    emit('95%CI/名义显著仅为探索性正态近似，未经多重比较校正；分桶/阈值/BTC拆分存在多重比较和事后选择风险。不得把不显著差异称作优势；显著差异也需要独立样本验证。')
    emit(f"无效net={dict(collections.Counter(r['how'] for r in net if r['r'] is None))}; BASELINE blocked={bb}")
    path = Path(args.out) / 'report.txt'
    path.write_text('\n'.join(lines) + '\n', encoding='utf-8')
    try:
        shutil.copyfile(path, NOTES)
        emit(f'报告副本已写入 {NOTES}')
    except OSError as exc:
        emit(f'报告副本写入失败（报告已保存在 {path}）：{exc}')
    path.write_text('\n'.join(lines) + '\n', encoding='utf-8')


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--cache', default='/Users/zhangyun/workspace/nofx/data/backtest_cache', help='contains ict5m/{SYM}.json')
    ap.add_argument('--universe', default='crypto')
    ap.add_argument('--universe-json', default=str(SCRATCH / 'universe2.json'))
    ap.add_argument('--out', default=str(SCRATCH / 'outBD'))
    ap.add_argument('--procs', type=int, default=9)
    ap.add_argument('--allow-overlap', action='store_true')
    ap.add_argument('--sample', type=int, default=0, help='deterministic coin sample, 0=all')
    args = ap.parse_args()
    if args.procs < 1 or args.sample < 0:
        ap.error('--procs must be positive; --sample must be nonnegative')
    try:
        import numpy  # noqa: F401
        import backtrader  # noqa: F401
    except ImportError as exc:
        ap.error(f'{exc}; use /Users/zhangyun/workspace/nofx/.venv/bin/python')
    Path(args.out).mkdir(parents=True, exist_ok=True)
    ctxs, syms, meta = load_data(args)
    candidates, jobs, extra = build_candidates(ctxs, syms)
    meta.update(extra, command=' '.join([sys.executable, '-m', 'scripts.backtest.breakdown_study', *sys.argv[1:]]))
    rng = random.Random(7)
    controls = [control_of(j, ctxs[j['sym']], rng) for j in jobs]
    atomic_json(Path(args.out) / 'study.json', dict(meta=meta, candidates=candidates, jobs=jobs, controls=controls))
    signature = hashlib.sha256(meta['data_sha256'].encode())
    for name in ('breakdown_study.py', 'ict_bt.py', 'backtrader_adapter.py', 'params.py'):
        signature.update(Path(__file__).with_name(name).read_bytes())
    runs = {}
    for label, js, gross in (('net', jobs, False), ('gross', jobs, True),
                              ('control_net', controls, False), ('control_gross', controls, True)):
        runs[label] = run_checkpoint(js, args, label, signature.hexdigest(), gross)
    report(args, meta, candidates, jobs, runs['net'], runs['gross'], controls,
           runs['control_net'], runs['control_gross'])


if __name__ == '__main__':
    main()
