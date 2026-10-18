"""Causal short-selection research; all trading uses ict_bt.run_jobs -> Backtrader.

Run with /Users/zhangyun/workspace/nofx/.venv/bin/python. No scipy/sklearn.
Example: python -m scripts.backtest.short_features_study --skip-fetch --sample 12
Fetches use urllib, ONLY the HTTP proxy at 127.0.0.1:7890, >=0.65s/request.
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
import urllib.error
import urllib.parse
import urllib.request

if __package__ in (None, ''):
    sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from scripts.backtest import breakdown_study as bd

import numpy as np
import pandas as pd

HOUR, DAY, M5 = bd.HOUR, bd.DAY, bd.M5
PURGE = 76 * HOUR  # beyond both 24h labels and the 72h (+2.5h) trade deadline
NOTES = Path('/Users/zhangyun/Library/Application Support/Claude/side-session-notes/local_461050eb-0464-42a1-be93-dc653e1a6e70/short_features_report.txt')
MARKET = ['btc_24h', 'btc_5d', 'btc_bear', 'btc_bull', 'btc_chop', 'breadth_negative',
          'breadth_below_ema20', 'breadth_ema_bear', 'market_median', 'market_dispersion', 'utc_hour']
TARGETS = ['net_r', 'short_24h', 'excess_24h']
PRICE_KEYS = ('ret_1h ret_4h ret_24h ret_5d ema_gap ema_direction ema20_slope price_ema20 '
              'dist_low24 dist_low7 new_low close_new_low range7_position adx_1h adx_4h '
              'rsi_1h rsi_4h bounce_failure continuation_newlow lower_high4h upper_wick '
              'atr_pct realized_vol relative_5d beta24 vol30_ratio new old continuation '
              'gate atr pump_stretch pump_overbought pump_extension pump_parabolic '
              'pump_score pump_price_score').split()
EXCLUDE = {'sym', 't_ms', 'regime', 'atr', 'all', 'loser_gate', 'loser_nogate', 'gainer',
           'loser20', 'trade', 'gate', 'short_12h', 'short_24h', 'excess_12h', 'excess_24h',
           'net_r', 'gross_r', 'win', 'exit_ms', 'q', 'score', 'future_valid'}


def atomic_json(path, obj):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    bd.atomic_json(path, obj)


def stamp(t):
    return dt.datetime.fromtimestamp(int(t) / 1000, dt.timezone.utc).isoformat()


def read_json(path, default):
    try:
        return json.loads(Path(path).read_text())
    except (OSError, ValueError):
        return default


class Fetcher:
    """One serial, globally paced urllib client; no environment-proxy fallback."""
    def __init__(self):
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({
            'http': 'http://127.0.0.1:7890', 'https': 'http://127.0.0.1:7890'}))
        self.last = -math.inf

    def get(self, endpoint, params):
        url = 'https://fapi.binance.com' + endpoint + '?' + urllib.parse.urlencode(params)
        error = None
        for attempt in range(3):
            time.sleep(max(0, 0.65 - (time.monotonic() - self.last)))
            self.last = time.monotonic()
            try:
                req = urllib.request.Request(url, headers={'User-Agent': 'nofx-short-research/1'})
                with self.opener.open(req, timeout=25) as response:
                    result = json.load(response)
                if not isinstance(result, list):
                    raise ValueError(str(result)[:180])
                return result
            except (OSError, ValueError) as exc:
                error = exc
                if isinstance(exc, urllib.error.HTTPError) and exc.code in (400, 401, 403, 404):
                    break
                time.sleep(min(12, 2 ** (attempt + 1)))
        raise RuntimeError(f'{endpoint}: {type(error).__name__}: {error}')


def fetch_symbol(client, root, sym, start, end, kind):
    """Persist EACH page and cursor. Cursor is independent of missing/listing bars.

    ict1h rows: [open_ms,o,h,l,c,base_volume,quote_volume,taker_buy_base,
                available_close_ms]. The last field is Binance closeTime+1.
    fund rows: [fundingTime,settled_rate]. Historical settlements, not forward quotes.
    """
    path = Path(root) / kind / f'{sym}.json'
    state_path = path.with_suffix('.state.json')
    rows = {int(r[0]): r for r in read_json(path, [])}
    state = read_json(state_path, {})
    if path.exists() and state.get('start') == start and state.get('end') == end and state.get('complete'):
        return dict(n=len(rows), cached=True, complete=True)
    # Resume completed pages even if the previous process died mid-symbol.
    cur = max(start, int(state.get('cursor', start))) if state.get('start') == start else start
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        while cur < end:
            if kind == 'ict1h':
                stop = min(end, cur + 1500 * HOUR)
                raw = client.get('/fapi/v1/klines', dict(symbol=sym, interval='1h',
                                 startTime=cur, endTime=stop - 1, limit=1500))
                got = [[int(k[0]), *map(float, k[1:6]), float(k[7]), float(k[9]), int(k[6]) + 1]
                       for k in raw if int(k[0]) < end and int(k[6]) + 1 <= end]
                nxt = max(cur + HOUR, int(raw[-1][0]) + HOUR) if raw else stop
            else:
                raw = client.get('/fapi/v1/fundingRate', dict(symbol=sym, startTime=cur,
                                 endTime=end - 1, limit=1000))
                got = [[int(k['fundingTime']), float(k['fundingRate'])] for k in raw]
                nxt = max(cur + 1, got[-1][0] + 1) if len(raw) == 1000 else end
            rows.update((int(r[0]), r) for r in got)
            atomic_json(path, [rows[k] for k in sorted(rows)])
            cur = nxt
            atomic_json(state_path, dict(start=start, end=end, cursor=cur, complete=cur >= end))
        return dict(n=len(rows), complete=True)
    except (OSError, RuntimeError, ValueError, IndexError, KeyError) as exc:
        # Preserve successfully downloaded pages. Missing windows stay NaN.
        return dict(n=len(rows), complete=False, error=str(exc))


def fetch_all(args, ctxs, syms):
    root = Path(args.aux_cache or args.cache)
    start = min(int(ctxs[s].a5['t_open'][0]) for s in syms)
    end = max(int(ctxs[s].a5['t_close'][-1]) for s in syms)
    client, status = Fetcher(), {}
    for n, sym in enumerate(syms):
        status[sym] = {kind: fetch_symbol(client, root, sym, start, end, kind)
                       for kind in ('ict1h', 'fund')}
        print(f'fetch {n+1}/{len(syms)} {sym}: {status[sym]}', flush=True)
        atomic_json(Path(args.out) / 'fetch_status.json', status)
    return status


def load_aux(root, sym):
    rows = np.asarray(read_json(Path(root) / 'ict1h' / f'{sym}.json', []), dtype=float)
    a = {}
    if rows.ndim == 2 and rows.shape[1] == 9:
        good = np.isfinite(rows).all(axis=1) & (rows[:, 5:8] >= 0).all(axis=1)
        good &= (rows[:, 7] <= rows[:, 5] * 1.000001)
        rows = rows[good]
        if len(rows):
            rows = rows[np.argsort(rows[:, 0], kind='stable')]
            a = {k: rows[:, i] for i, k in enumerate(
                ('t_open', 'o', 'h', 'l', 'c', 'v', 'qv', 'tb', 't_close'))}
            a['t_open'], a['t_close'] = a['t_open'].astype(np.int64), a['t_close'].astype(np.int64)
    f = np.asarray(read_json(Path(root) / 'fund' / f'{sym}.json', []), dtype=float)
    if f.ndim != 2 or f.shape[1] != 2:
        f = np.empty((0, 2))
    f = f[np.isfinite(f).all(axis=1)]
    return a, f[np.argsort(f[:, 0], kind='stable')]


def divide(a, b):
    return float(a / b) if np.isfinite(b) and b > 0 else np.nan


def adx(k, period=14):
    """Wilder DMI/ADX, expanding seed; last value of the causal fetched window."""
    if len(k['c']) < 2 * period:
        return np.nan
    h, l, c = k['h'], k['l'], k['c']
    up, dn = np.diff(h), -np.diff(l)
    plus, minus = np.r_[0, np.where((up > dn) & (up > 0), up, 0)], np.r_[0, np.where((dn > up) & (dn > 0), dn, 0)]
    def smooth(xs):
        result = [float(xs[0])]
        for i, x in enumerate(xs[1:], 1):
            result.append(result[-1] + (float(x) - result[-1]) / min(i + 1, period))
        return np.asarray(result)
    av = np.asarray(bd.atr(h, l, c, period))
    p, m = smooth(plus) / np.maximum(av, 1e-20), smooth(minus) / np.maximum(av, 1e-20)
    dx = 100 * np.abs(p - m) / np.maximum(p + m, 1e-20)
    return float(smooth(dx[period-1:])[-1])


def continuation_parts(k1, k4):
    n, c = len(k1['c']), k1['c']
    e = bd.ema(c, 20)
    bounce = float(c[-1] < e[-1] and any(k1['h'][i] >= e[i] and c[i] < e[i] for i in range(n-13, n-1)))
    lo = min(k1['l'][n-25:n-1])
    newlow = 1.0 if c[-1] < lo else (0.6 if k1['l'][-1] < lo else 0.0)
    lower = float(max(k4['h'][-6:]) < max(k4['h'][-12:-6]))
    return bounce, newlow, lower


def volume_features(a, t):
    out = dict.fromkeys(['quote_volume_ratio', 'down_volume', 'taker_buy', 'sell_pressure',
                         'taker_change', 'pump_volume_fade'], np.nan)
    if not a:
        return out
    k = bd.closed_window(a, t, 24, HOUR)
    if k is None:
        return out
    q, v, tb = k['qv'], k['v'], k['tb']
    out['quote_volume_ratio'] = divide(np.mean(q[-3:]), np.mean(q))
    down = k['c'][-6:] < k['o'][-6:]
    out['down_volume'] = divide(np.mean(q[-6:][down]), np.mean(q)) if down.any() else 0.0
    out['taker_buy'] = divide(sum(tb[-3:]), sum(v[-3:]))
    out['sell_pressure'] = 1 - out['taker_buy']
    out['taker_change'] = out['taker_buy'] - divide(sum(tb[-6:-3]), sum(v[-6:-3]))
    ratio = divide(np.mean(q[-3:]), np.max(q))
    if np.isfinite(ratio):
        fade = bd.sigmoid(-ratio, -0.45, 0.2)
        if k['c'][-1] >= max(k['c']) and ratio < 0.7:
            fade = max(70, fade)
        out['pump_volume_fade'] = fade
    return out


def funding_features(f, t):
    out = dict.fromkeys(['funding_rate', 'funding_percentile', 'funding_change3'], np.nan)
    if not len(f):
        return out
    n = np.searchsorted(f[:, 0], t, side='right')
    if not n or t - f[n-1, 0] > 16 * HOUR:
        return out  # do not forward-fill a failed/missing funding history forever
    rate = f[n-1, 1]
    out['funding_rate'] = float(rate)
    start = np.searchsorted(f[:, 0], t - 30 * DAY, side='left')
    past = f[start:n, 1]
    # Require a nearly complete calendar month; variable settlement frequency is allowed.
    if len(past) >= 20 and f[start, 0] <= t - 28 * DAY:
        out['funding_percentile'] = float((sum(past < rate) + 0.5 * sum(past == rate)) / len(past))
    if n >= 4 and f[n-4, 0] >= t - 48 * HOUR:
        out['funding_change3'] = float(rate - f[n-4, 1])
    return out


def price_features(ctx, btc, t, aux, funding):
    k1 = bd.closed_window(ctx.a1, t, 60, HOUR)
    k4 = bd.closed_window(ctx.a4, t, 84, 4 * HOUR)
    bk = bd.closed_window(btc.a4, t, 84, 4 * HOUR)
    if k1 is None or k4 is None or bk is None:
        # Availability is not a short-selection gate: keep active coins with
        # incomplete feature windows. Never select the random baseline conditional
        # on 14 days of trend data (particularly relevant to newly listed gainers).
        i = np.searchsorted(ctx.a5['t_close'],t,side='right')-1
        if i < 0 or ctx.a5['t_close'][i] != t:
            return None  # no actual tradable/listed bar at the decision instant
        r = dict.fromkeys(PRICE_KEYS,np.nan)
        for h in (1,4,24):
            w = bd.closed_window(ctx.a1,t,h+1,HOUR)
            if w is not None:
                r[f'ret_{h}h'] = float(w['c'][-1]/w['c'][0]-1)
        if k1 is not None:
            r['atr'] = bd.atr(k1['h'],k1['l'],k1['c'])[-1]
            r['atr_pct'] = divide(r['atr'],k1['c'][-1])
        r.update(volume_features(aux,t),**funding_features(funding,t))
        return r
    s = bd.score_windows(k1, k4, bk)
    c, c4 = k1['c'], k4['c']
    a1, a4 = s['atr'], bd.atr(k4['h'], k4['l'], c4)[-1]
    e20, e50 = bd.ema(c4, 20), bd.ema(c4, 50)
    low24, low7, hi7 = min(k1['l'][-25:-1]), min(k4['l'][-42:]), max(k4['h'][-42:])
    bounce, newlow, lower = continuation_parts(k1, k4)
    ret5 = c4[-1] / c4[-31] - 1
    btc5 = bk['c'][-1] / bk['c'][-31] - 1
    r = dict(ret_1h=c[-1]/c[-2]-1, ret_4h=c[-1]/c[-5]-1,
             ret_24h=c[-1]/c[-25]-1, ret_5d=ret5,
             ema_gap=divide(e20[-1]-e50[-1], a4), ema_direction=float(e20[-1] > e50[-1]),
             ema20_slope=divide(e20[-1]-e20[-2], a4), price_ema20=divide(c4[-1]-e20[-1], a4),
             dist_low24=divide(c[-1]-low24, a1), dist_low7=divide(c[-1]-low7, a1),
             new_low=float(k1['l'][-1] < low24), close_new_low=float(c[-1] < low24),
             range7_position=divide(c[-1]-low7, hi7-low7), adx_1h=adx(k1), adx_4h=adx(k4),
             rsi_1h=bd.rsi_series(c)[-1], rsi_4h=bd.rsi_series(c4)[-1],
             bounce_failure=bounce, continuation_newlow=newlow, lower_high4h=lower,
             upper_wick=float(np.mean((k1['h'][-3:]-np.maximum(k1['o'][-3:], c[-3:])) /
                                     np.maximum(k1['h'][-3:]-k1['l'][-3:], 1e-20))),
             atr_pct=divide(a1, c[-1]), realized_vol=float(np.std(np.diff(np.log(c[-25:])), ddof=1)),
             relative_5d=ret5-btc5, beta24=np.nan, vol30_ratio=np.nan,
             new=s['new'], old=s['old'], continuation=s['continuation'], gate=s['gate'], atr=a1)
    b1 = bd.closed_window(btc.a1, t, 25, HOUR)
    if b1 is not None:
        x, y = np.diff(np.log(b1['c'])), np.diff(np.log(c[-25:]))
        r['beta24'] = divide(float(np.mean((x-x.mean())*(y-y.mean()))), float(np.var(x)))
    # Rolling 24h volatility relative to the median of PRIOR 30 calendar days.
    long = bd.closed_window(ctx.a1, t, 30*24+25, HOUR)
    if long is not None:
        rets = pd.Series(np.diff(np.log(long['c'])))
        vols = rets.rolling(24).std(ddof=1).to_numpy()
        r['vol30_ratio'] = divide(r['realized_vol'], float(np.median(vols[-721:-1])))
    rolls = c4[6:] / c4[:-6] - 1
    r['pump_stretch'] = bd.sigmoid(divide(r['ret_24h']-np.mean(rolls), np.std(rolls)), 2, 1) if np.std(rolls) else np.nan
    def fade(v, lo, hi):
        return max(0, min(1, 1-(lo-v)/10, 1-(v-hi)/10))
    r['pump_overbought'] = .6*bd.sigmoid(r['rsi_1h'],82,7)*fade(r['rsi_1h'],65,95) + .4*bd.sigmoid(r['rsi_4h'],78,8)*fade(r['rsi_4h'],60,95)
    ext = ((c[-1]/bd.ema(c,50)[-1]-1) + .5*(c[-1]/bd.ema(c,20)[-1]-1) + .8*(c4[-1]/e20[-1]-1))*100
    r['pump_extension'] = bd.sigmoid(ext, 8, 5)
    prior = c4[-31]/c4[-61]-1
    para = .6*bd.sigmoid(ret5*100,40,25) + .4*bd.sigmoid((ret5-prior)*100,15,12)
    r['pump_parabolic'] = .7*para + .3*bd.sigmoid(bd.round2((ret5-btc5)*100),20,15)
    r.update(volume_features(aux, t), **funding_features(funding, t))
    # Equal renormalization of the five specified components (not the live nine-part score).
    parts = [r['pump_'+p] for p in ('stretch','overbought','extension','parabolic','volume_fade')]
    r['pump_score'] = float(np.mean(parts)) if np.isfinite(parts).all() else np.nan
    r['pump_price_score'] = float(np.mean(parts[:4]))
    return r


def market_features(features, btc, t):
    """All contemporaneous active universe members; each breadth excludes its NaNs."""
    if not features:
        return {}
    vals = list(features.values())
    bk = bd.closed_window(btc.a4, t, 84, 4*HOUR)
    regime = bd.btc_regime(bk)
    ret = np.asarray([v['ret_24h'] for v in vals])
    def negative_fraction(key):
        a = np.asarray([v[key] for v in vals])
        a = a[np.isfinite(a)]
        return float(np.mean(a < 0)) if len(a) else np.nan
    return dict(regime=regime, btc_24h=float(bk['c'][-1]/bk['c'][-7]-1),
                btc_5d=float(bk['c'][-1]/bk['c'][-31]-1), btc_bear=float(regime=='btc_bear'),
                btc_bull=float(regime=='btc_bull'), btc_chop=float(regime=='chop'),
                breadth_negative=negative_fraction('ret_24h'),
                breadth_below_ema20=negative_fraction('price_ema20'),
                breadth_ema_bear=negative_fraction('ema_gap'),
                market_median=float(np.nanmedian(ret)), market_dispersion=float(np.nanstd(ret)),
                utc_hour=int((t//HOUR) % 24))


def forward_return(ctx, t, horizon):
    a = ctx.a5
    i = int(np.searchsorted(a['t_close'], t, side='right'))-1
    j = int(np.searchsorted(a['t_close'], t+horizon*HOUR, side='right'))-1
    if i < 0 or j <= i or j+1 > len(a['c']) or i+1 >= len(a['o']):
        return np.nan
    if a['t_close'][i] != t or a['t_open'][i+1] != t or a['t_close'][j] != t+horizon*HOUR or j-i != horizon*12:
        return np.nan
    return float(a['c'][j]/a['o'][i+1]-1)


def decision_times(btc):
    start = (int(btc.a5['t_open'][0]) + 84*4*HOUR + DAY-1)//DAY*DAY
    last = int(btc.a5['t_close'][-1]) - int((bd.HORIZON+2.5)*HOUR)
    end = (last-20*HOUR)//DAY*DAY + 20*HOUR
    return np.arange(start, end+1, 4*HOUR, dtype=np.int64)


def causal_check(ctxs, syms, auxs, t):
    """Rebuild every coin/BTC from 5m prefixes; truncate BOTH auxiliary sources too."""
    def prefix(ctx):
        a = ctx.a5
        n = np.searchsorted(a['t_close'], t, side='right')
        return bd.Context.from_bars(np.column_stack([a['t_open'][:n], *[a[k][:n] for k in ('o','h','l','c')]]).tolist())
    btc = ctxs['BTCUSDT']
    clipped = {s: prefix(ctxs[s]) for s in dict.fromkeys([*syms, 'BTCUSDT'])}
    full, cut = {}, {}
    for s in syms:
        a, f = auxs[s]
        ca = {k: v[a['t_close'] <= t] for k,v in a.items()} if a else {}
        cf = f[f[:,0] <= t]
        full[s] = price_features(ctxs[s],btc,t,a,f)
        cut[s] = price_features(clipped[s],clipped['BTCUSDT'],t,ca,cf)
        assert (full[s] is None) == (cut[s] is None)
        if full[s] is not None:
            for key in full[s]:
                assert np.isclose(full[s][key],cut[s][key],rtol=1e-12,atol=1e-12,equal_nan=True), (s,key,t)
    full, cut = {s:v for s,v in full.items() if v is not None}, {s:v for s,v in cut.items() if v is not None}
    fm,cm = market_features(full,btc,t),market_features(cut,clipped['BTCUSDT'],t)
    for k in fm:
        assert (fm[k] == cm[k] if isinstance(fm[k],str) else np.isclose(fm[k],cm[k],equal_nan=True)), ('breadth truncation mismatch',k)
    return len(full)


def build_samples(args, ctxs, syms):
    root = args.aux_cache or args.cache
    auxs = {s: load_aux(root,s) for s in syms}
    decisions, rng = decision_times(ctxs['BTCUSDT']), random.Random(args.seed)
    if args.max_decisions and len(decisions) > args.max_decisions:
        decisions = decisions[np.linspace(0,len(decisions)-1,args.max_decisions,dtype=int)]
    if not len(decisions):
        raise ValueError('not enough data for 14d warmup and 72h exit coverage')
    checks = []
    for j in sorted(set([0,len(decisions)//2,len(decisions)-1])):
        t = int(decisions[j])
        checks.append(dict(t=stamp(t),coins=causal_check(ctxs,syms,auxs,t)))
    print(f'causal prefix checks: {checks}',flush=True)
    rows, jobs, job_rows, counts = [], [], [], collections.Counter()
    btc = ctxs['BTCUSDT']
    started = time.monotonic()
    for ti, t0 in enumerate(decisions):
        t = int(t0)
        if bd.closed_window(btc.a4,t,84,4*HOUR) is None:
            counts['btc_missing_decisions'] += 1
            continue
        features = {s: price_features(ctxs[s],btc,t,*auxs[s]) for s in syms}
        features = {s:v for s,v in features.items() if v is not None}
        if not features:
            counts['empty_decisions'] += 1
            continue
        market = market_features(features,btc,t)
        # Board rank uses the same 25 closed 1h-bar eligibility as breakdown_study,
        # even where the longer feature/gate window is not ready yet.
        board = []
        for s in syms:
            w = bd.closed_window(ctxs[s].a1,t,25,HOUR)
            if w is not None:
                board.append((float(w['c'][-1]/w['c'][-25]-1),s))
        loser = {s for _,s in sorted(board)[:20]}
        gainer = {s for chg,s in sorted(board,reverse=True)[:20] if chg >= .05}
        # Sampling precedes availability of forward outcomes and trade-validity tests.
        sampled = set(rng.sample(sorted(features),min(12,len(features))))
        rank = pd.Series({s:v['ret_24h'] for s,v in features.items()}).rank(pct=True)
        breturns = {h: forward_return(btc,t,h) for h in (12,24)}
        for s, f in features.items():
            row = dict(sym=s,t_ms=t,**f,**market,ret24_rank=float(rank[s]),
                       all=s in sampled,loser20=s in loser,
                       loser_gate=bool(s in loser and np.isfinite(f['gate']) and f['gate']),
                       loser_nogate=bool(s in loser and np.isfinite(f['gate']) and not f['gate']),gainer=s in gainer,trade=False)
            for h in (12,24):
                ret = forward_return(ctxs[s],t,h)
                row[f'short_{h}h'] = -ret
                row[f'excess_{h}h'] = -ret+breturns[h]  # short coin + LONG BTC, beta=1
            ri = len(rows)
            if row['all'] or row['loser20'] or row['gainer']:
                j = bd.make_job(s,ctxs[s],t,dict(atr=f['atr'])) if np.isfinite(f['atr']) else None
                if j:
                    # Existing universe allows small gaps. Reject any local future
                    # gap before handing the job to Backtrader (no synthetic bars).
                    endi = np.searchsorted(ctxs[s].a5['t_close'],t+int(74.5*HOUR),side='right')-1
                    if int(ctxs[s].a5['t_close'][endi]) == t+int(74.5*HOUR) and endi-j['idx'] == int(74.5*12):
                        jobs.append(j)
                        job_rows.append(ri)
                        row['trade'] = True
                    else:
                        counts['future_gap'] += 1
                else:
                    counts['invalid_geometry_or_coverage'] += 1
            rows.append(row)
        if ti % 120 == 0 or ti == len(decisions)-1:
            print(f'features {ti+1}/{len(decisions)} rows={len(rows)} jobs={len(jobs)} elapsed={time.monotonic()-started:.1f}s',flush=True)
    df = pd.DataFrame(rows)
    return df, jobs, job_rows, dict(decisions=decisions.tolist(),causal_checks=checks,excluded=dict(counts))


def rank_average(x):
    return pd.Series(np.asarray(x, dtype=float)).rank(method='average').to_numpy()


def spearman(x, y):
    x, y = np.asarray(x,dtype=float), np.asarray(y,dtype=float)
    keep = np.isfinite(x) & np.isfinite(y)
    x, y = x[keep], y[keep]
    if len(x) < 3:
        return np.nan
    # Reuse the tie-exact implementation from the previous study.
    val = bd.spearman(x.tolist(),y.tolist())
    return float(val) if val is not None else np.nan


def week_id(t):
    return (np.asarray(t,dtype=np.int64)//DAY+3)//7


def cluster_estimate(t, values, weights=None):
    """Calendar-week CR1 mean SE (same influence function as bd.mean_difference)."""
    t, values = np.asarray(t), np.asarray(values,dtype=float)
    weights = np.ones(len(values)) if weights is None else np.asarray(weights,dtype=float)
    keep = np.isfinite(values) & np.isfinite(weights) & (weights > 0)
    t, v, w = t[keep], values[keep], weights[keep]
    if not len(v):
        return dict(n=0,mean=np.nan,se=np.nan,t=np.nan,weeks=0,lo=np.nan,hi=np.nan)
    mu = float(np.average(v,weights=w))
    weeks = week_id(t)
    g = int(weeks.max()-weeks.min()+1)
    inf = np.bincount(weeks-weeks.min(),weights=w*(v-mu)/w.sum())
    se = float(np.sqrt(g/(g-1)*sum(inf**2))) if g > 1 else np.nan
    tv = mu/se if se > 0 else (math.copysign(math.inf,mu) if se == 0 and mu else np.nan)
    return dict(n=len(v),mean=mu,se=se,t=tv,weeks=g,lo=mu-1.96*se,hi=mu+1.96*se)


def cluster_difference(a, b, metric):
    """Reuse shared-week difference tool, including covariance of overlapping groups."""
    a, b = a[np.isfinite(a[metric])], b[np.isfinite(b[metric])]
    if a.empty or b.empty:
        return dict(n_a=len(a),n_b=len(b),mean=np.nan,se=np.nan,lo=np.nan,hi=np.nan,weeks=0)
    # Lightweight adapters into the prior study's R-agnostic influence function.
    d = bd.mean_difference(a[['t_ms']].to_dict('records'),[{'r':v} for v in a[metric]],range(len(a)),
                           b[['t_ms']].to_dict('records'),[{'r':v} for v in b[metric]],range(len(b)))
    se = d['cluster_se'] if d['cluster_se'] is not None else np.nan
    return dict(n_a=len(a),n_b=len(b),mean=d['diff'],se=se,weeks=d['weeks'],
                lo=d['diff']-1.96*se,hi=d['diff']+1.96*se)


def ic_series(frame, feature, target):
    """Per-decision Spearman, pairwise missingness, at least 6 distinct coins."""
    z = frame[['t_ms',feature,target]].replace([np.inf,-np.inf],np.nan).dropna()
    if z.empty:
        return pd.DataFrame(columns=['t_ms','ic','n','regime'])
    ranks = z.groupby('t_ms')[[feature,target]].rank(method='average')
    centered = ranks-ranks.groupby(z['t_ms']).transform('mean')
    product = pd.DataFrame({'xy':centered[feature]*centered[target],
                            'xx':centered[feature]**2,'yy':centered[target]**2,'t_ms':z['t_ms']})
    sums = product.groupby('t_ms').sum()
    nums = z.groupby('t_ms').size()
    out = pd.DataFrame({'ic':sums['xy']/np.sqrt(sums['xx']*sums['yy']),'n':nums})
    out = out[(out['n'] >= 6) & np.isfinite(out['ic'])].reset_index()
    regimes = frame[['t_ms','regime']].drop_duplicates('t_ms').set_index('t_ms')['regime']
    out['regime'] = out['t_ms'].map(regimes)
    return out


def pooled_ic(frame, feature, target, reps=39, seed=17):
    """Actual calendar-week block bootstrap: rerank EACH resampled observation set."""
    z = frame[['t_ms',feature,target]].replace([np.inf,-np.inf],np.nan).dropna()
    val = spearman(z[feature],z[target])
    groups = list(z.groupby(week_id(z['t_ms'])).indices.values())
    if len(groups) < 2 or not np.isfinite(val):
        return dict(n=len(z),mean=val,se=np.nan,weeks=len(groups))
    x, y = z[feature].to_numpy(), z[target].to_numpy()
    rng, draws = np.random.default_rng(seed), []
    for _ in range(reps):
        ix = np.concatenate([groups[j] for j in rng.integers(0,len(groups),len(groups))])
        # Vectorized equivalent of bd.spearman, including exact average tie ranks.
        rx, ry = rank_average(x[ix]), rank_average(y[ix])
        if np.std(rx) > 0 and np.std(ry) > 0:
            draws.append(float(np.corrcoef(rx,ry)[0,1]))
    return dict(n=len(z),mean=val,se=float(np.std(draws,ddof=1)) if len(draws)>1 else np.nan,weeks=len(groups))


def stability(series, times):
    """Both time halves |cluster-t|>2, same sign; >=2/3 BTC states agree."""
    times = np.unique(times)
    if len(times) < 4:
        return False, np.nan, 'too_few_times', {}
    cut = times[len(times)//2]
    v = series[series['t_ms'].isin(times)]
    scopes = {'full':v,'first':v[v['t_ms']<cut],'second':v[v['t_ms']>=cut]}
    scopes.update({s:v[v['regime']==s] for s in ('btc_bear','chop','btc_bull')})
    stats = {s:cluster_estimate(g['t_ms'],g['ic']) for s,g in scopes.items()}
    f, a, b = stats['full'], stats['first'], stats['second']
    sign = np.sign(f['mean'])
    reasons = []
    if a['n'] < 20 or b['n'] < 20 or a['weeks'] < 3 or b['weeks'] < 3:
        reasons.append('insufficient_halves')
    if not (np.isfinite(a['mean']) and np.isfinite(b['mean']) and a['mean']*b['mean'] > 0):
        reasons.append('half_sign_unstable_or_constant')
    if not (abs(a['t']) > 2 and abs(b['t']) > 2 and abs(f['t']) > 2):
        reasons.append('half_or_full_abs_t<=2')
    if sum(stats[s]['mean']*sign > 0 for s in ('btc_bear','chop','btc_bull')) < 2:
        reasons.append('BTC_sign_agreement<2')
    return not reasons, float(sign), ','.join(reasons) or 'pass', stats


def quintiles(frame, scores):
    """Contemporaneous full-universe ranks; ties stay together, empty bins allowed."""
    s = pd.Series(np.asarray(scores,dtype=float),index=frame.index)
    ranks = s.groupby(frame['t_ms']).rank(pct=True,method='average')
    return np.ceil(ranks*5).clip(1,5).astype(float)


def cross_standardize(frame, features):
    x = frame[features].replace([np.inf,-np.inf],np.nan)
    group = x.groupby(frame['t_ms'])
    sd = group.transform('std').replace(0,np.nan)
    return ((x-group.transform('mean'))/sd).clip(-5,5).fillna(0).to_numpy(dtype=float)


def fit_ridge(x, y, alpha=1.0):
    """min mean squared error + alpha*||coef||^2; intercept unpenalized."""
    x, y = np.asarray(x,dtype=float),np.asarray(y,dtype=float)
    keep = np.isfinite(y) & np.isfinite(x).all(axis=1)
    x, y = x[keep],y[keep]
    if not len(y):
        raise ValueError('no finite training targets')
    center, bias = x.mean(axis=0),float(y.mean())
    a, b = x-center,y-bias
    coef = np.linalg.solve(a.T@a/max(1,len(y)) + alpha*np.eye(x.shape[1]),a.T@b/max(1,len(y)))
    return dict(center=center,coef=coef,bias=bias,n=len(y),alpha=alpha)


def predict_ridge(model, x):
    return (np.asarray(x)-model['center'])@model['coef']+model['bias']


def walk_forward(times, folds=1):
    """76h label-maturity purge; never train using a label crossing test start."""
    times = np.unique(times)
    boundaries = [len(times)//2,len(times)] if folds == 1 else [int(len(times)*i/5) for i in range(1,6)]
    for start,end in zip(boundaries[:-1],boundaries[1:]):
        if end <= start:
            continue
        test = times[start:end]
        train = times[times+PURGE < test[0]]
        if len(train):
            assert train[-1]+PURGE < test[0]
            yield train,test


def feature_names(df):
    return [c for c in df.columns if c not in EXCLUDE and pd.api.types.is_numeric_dtype(df[c])]


def strategy_indices(df, jobs, results, mask, allow_overlap=False):
    """Independently enforce same-coin short overlap for EACH actual strategy."""
    candidates = [i for i,j in enumerate(jobs) if mask[j['row_id']] and results[i]['r'] is not None]
    kept, blocked = bd.kept_indices(jobs,results,allow_overlap,candidates)
    return [jobs[i]['row_id'] for i in kept],blocked


def drawdown_stats(frame, reps=79, seed=19):
    """Equal initial R per trade, exit-time realized curve; weekly block-bootstrap SE."""
    z = frame[np.isfinite(frame['net_r'])].sort_values(['exit_ms','sym'])
    def dd(r):
        eq = np.r_[0,np.cumsum(r)]
        return float(np.max(np.maximum.accumulate(eq)-eq))
    if z.empty:
        return dict(n=0,dd=np.nan,se=np.nan)
    r = z['net_r'].to_numpy()
    groups = list(z.groupby(week_id(z['exit_ms'])).indices.values())
    rng = np.random.default_rng(seed)
    draws = [dd(np.concatenate([r[groups[j]] for j in rng.integers(0,len(groups),len(groups))])) for _ in range(reps)]
    return dict(n=len(z),dd=dd(r),se=float(np.std(draws,ddof=1)) if len(groups)>1 else np.nan)


class Report:
    def __init__(self, out):
        self.path = Path(out)/'report.txt'
        self.path.write_text('',encoding='utf-8')
        self.summary = {}

    def emit(self, line=''):
        print(line,flush=True)
        with self.path.open('a',encoding='utf-8') as f:
            f.write(line+'\n')

    def estimate(self, name, frame, metric):
        s = cluster_estimate(frame['t_ms'],frame[metric])
        self.emit(f"{name} {metric}: n={s['n']} mean={s['mean']:+.6f} SE_week={s['se']:.6f} 95%CI=[{s['lo']:+.6f},{s['hi']:+.6f}] weeks={s['weeks']}")
        return s

    def difference(self, name, a, b, metric):
        d = cluster_difference(a,b,metric)
        verdict = '差异不显著/不足' if not np.isfinite(d['se']) or d['lo'] <= 0 <= d['hi'] else '名义显著（未校正多重比较）'
        self.emit(f"{name} Δ{metric}: n={d['n_a']}/{d['n_b']} delta={d['mean']:+.6f} SE_week={d['se']:.6f} 95%CI=[{d['lo']:+.6f},{d['hi']:+.6f}] {verdict}")
        return d


def audit_pools(rep, df, jobs, net, args):
    rep.emit('\n1. 前置筛选审计：all=每时刻随机12币；每个策略独立过滤同币同向重叠')
    groups, output = {}, {}
    for name in ('all','loser_gate','gainer','loser_nogate'):
        raw = df[df[name]]
        ix,blocked = strategy_indices(df,jobs,net,df[name].to_numpy(),args.allow_overlap)
        trading = df.loc[ix]
        groups[name] = (raw,trading)
        rep.emit(f'{name}: candidates={len(raw)} retained_trades={len(ix)} blocked={blocked}')
        output[name] = {}
        for metric in ('short_24h','excess_24h'):
            output[name][metric] = rep.estimate(name,raw,metric)
        for metric in ('net_r','gross_r','win'):
            output[name][metric] = rep.estimate(name,trading,metric)
    for name in ('loser_gate','gainer','loser_nogate'):
        output[name]['differences'] = {}
        for metric in ('short_24h','excess_24h','net_r','win'):
            k = 1 if metric in ('net_r','win') else 0
            output[name]['differences'][metric] = rep.difference(name+' − all',groups[name][k],groups['all'][k],metric)
    rep.summary['gainer_minus_loser'] = rep.difference('gainer − loser_gate',groups['gainer'][1],groups['loser_gate'][1],'net_r')
    rep.summary['gate_increment'] = rep.difference('loser_gate − loser_nogate',groups['loser_gate'][1],groups['loser_nogate'][1],'net_r')
    rep.emit('池标签可重叠；共享样本及同周跨币相关纳入差值周聚类SE。连续目标包含所有候选，R只含实际可成交且经过重叠过滤的策略队列，二者样本量分别列出。')
    rep.summary['pools'] = output
    return groups


def analyze_ics(rep, df, features, args):
    base = df[df['all']]
    times = np.unique(base['t_ms'])
    cut = times[len(times)//2]
    scopes = {'full':base,'first_half':base[base['t_ms']<cut],'second_half':base[base['t_ms']>=cut]}
    scopes.update({s:base[base['regime']==s] for s in ('btc_bear','chop','btc_bull')})
    rep.emit(f'\n2. 单特征IC：共同随机all样本，未过滤交易重叠，R仅有Backtrader有效交易；切点={stamp(cut)}')
    rep.emit(f'多重比较：{len(features)}特征 × {len(TARGETS)}主要目标 = {len(features)*len(TARGETS)}，另有时期/状态分层和组合/阈值搜索；所有显著性均未校正，候选仅作探索。')
    rep.emit(f'FMB为每决策时刻内Spearman（配对有限值、至少6币）再平均，SE按UTC周聚类；池化IC的SE为{args.bootstrap}次整周块自助、每次重新计算平均并列秩。市场状态/UTC小时在截面内常数，不能通过截面IC筛选。')
    records, cache, stable, rejected = [], {}, collections.defaultdict(list), {}
    for fi,feature in enumerate(features):
        for target in TARGETS:
            series = ic_series(base,feature,target)
            cache[feature,target] = series
            ok,sign,reason,_ = stability(series,times)
            if ok:
                stable[target].append((feature,sign))
            rejected[feature,target] = reason
            for scope,z in scopes.items():
                ss = series[series['t_ms'].isin(z['t_ms'].unique())]
                s = cluster_estimate(ss['t_ms'],ss['ic'])
                p = pooled_ic(z,feature,target,args.bootstrap,args.seed+fi)
                records.append(dict(feature=feature,target=target,scope=scope,fmb_n=s['n'],fmb_ic=s['mean'],
                                    fmb_se=s['se'],fmb_t=s['t'],n_pairs=int(ss['n'].sum()),
                                    pooled_n=p['n'],pooled_ic=p['mean'],pooled_se=p['se'],stable=ok,reason=reason))
                rep.emit(f"IC {feature}/{target}/{scope}: decisions={s['n']} pairs={int(ss['n'].sum())} FMB={s['mean']:+.5f} SE_week={s['se']:.5f} t={s['t']:+.2f}; pooled_n={p['n']} rho={p['mean']:+.5f} SE_block={p['se']:.5f}")
        if fi % 10 == 0:
            print(f'IC progress {fi+1}/{len(features)}',flush=True)
    pd.DataFrame(records).to_csv(Path(args.out)/'ic.csv',index=False)
    rep.emit('稳定要求：前/后半期同号且两半及全期 |周聚类t|>2；至少两个BTC状态同号，每半期>=20个截面和>=3周。')
    for target in TARGETS:
        rep.emit(f'全样本描述性候选 {target}: {stable[target]}（不作为样本外训练的事后特征清单）')
    rep.summary['stable_descriptive'] = dict(stable)
    return cache,stable,rejected


def quantile_table(rep, label, df, q, jobs, net, args, mask=None):
    """For diagnostics: filter one shared all-strategy queue, then slice quintiles.

    Actual market/sensitivity strategies use independent overlap instead.
    """
    mask = df['all'].to_numpy().copy() if mask is None else np.asarray(mask,dtype=bool)
    raw = df.loc[mask].copy()
    raw['q'] = q.loc[raw.index]
    ix,blocked = strategy_indices(df,jobs,net,mask,args.allow_overlap)
    trade = df.loc[ix].copy()
    trade['q'] = q.loc[trade.index]
    means,means_r,output = [],[],{}
    for qi in range(1,6):
        rr,tt = raw[raw['q']==qi],trade[trade['q']==qi]
        a = rep.estimate(f'{label} Q{qi}',rr,'short_24h')
        b = rep.estimate(f'{label} Q{qi}',tt,'net_r')
        e = rep.estimate(f'{label} Q{qi}',rr,'excess_24h')
        means.append(a['mean'])
        means_r.append(b['mean'])
        output[qi] = dict(short_24h=a,net_r=b,excess_24h=e)
    d24 = rep.difference(label+' Q5−Q1',raw[raw['q']==5],raw[raw['q']==1],'short_24h')
    dr = rep.difference(label+' Q5−Q1',trade[trade['q']==5],trade[trade['q']==1],'net_r')
    def monotonicity(xs):
        finite = np.isfinite(xs).all()
        return 'increasing' if finite and np.all(np.diff(xs)>=0) else ('decreasing' if finite and np.all(np.diff(xs)<=0) else 'nonmonotone/empty_bins')
    mono,mono_r = monotonicity(means),monotonicity(means_r)
    rep.emit(f'{label}: raw-score monotonicity24={mono}, monotonicityR={mono_r}; common_queue_blocked={blocked}; ties are not split. 诊断分位共用队列，不能当作每个分位独立开仓组合。')
    return dict(bins=output,delta24=d24,deltaR=dr,monotonicity=mono,monotonicity_R=mono_r)


def fit_scores(df, features, ic_cache, train_times, test_times):
    train_period = df['t_ms'].isin(train_times)
    training = train_period & df['all']
    testing = df['t_ms'].isin(test_times)
    train = df.loc[training]
    eligible = [f for f in features if f not in MARKET and np.isfinite(train[f]).mean() >= .6
                and train[f].nunique() > 1]
    stable = []
    for f in eligible:
        ok,sign,_,_ = stability(ic_cache[f,'excess_24h'],train_times)
        if ok:
            stable.append((f,sign))
    pred = pd.DataFrame(index=df.index[testing])
    pred['equal'] = np.nan
    if stable:
        subset = df.loc[train_period | testing]
        z = cross_standardize(subset,[f for f,_ in stable])
        sign = np.asarray([s for _,s in stable])
        pred['equal'] = (z@sign/len(stable))[testing.loc[subset.index].to_numpy()]
    model = None
    pred['ridge'] = np.nan
    if eligible and len(train) >= 50:
        subset = df.loc[train_period | testing]
        x = cross_standardize(subset,eligible)
        tr = training.loc[subset.index].to_numpy()
        model = fit_ridge(x[tr],subset.loc[tr,'excess_24h'].to_numpy(),alpha=1.0)
        pred['ridge'] = predict_ridge(model,x[testing.loc[subset.index].to_numpy()])
    for f in ('new','old'):
        pred[f] = df.loc[testing,f]
    return pred,dict(stable=stable,ridge_features=eligible,ridge_coef={} if model is None else dict(zip(eligible,model['coef'].tolist())),
                     train_times=len(train_times),train_rows=len(train),train_last=int(max(train_times)),test_first=int(min(test_times)))


def report_score_ic(rep,label,frame,score):
    z = frame.copy()
    z['score'] = score.loc[z.index]
    out = {}
    for target in TARGETS:
        series = ic_series(z,'score',target)
        s = cluster_estimate(series['t_ms'],series['ic'])
        rep.emit(f"{label}/{target}: decisions={s['n']} pairs={int(series['n'].sum())} FMB_IC={s['mean']:+.5f} SE_week={s['se']:.5f} t={s['t']:+.2f}")
        out[target] = s
    return out


def audit_ranks(rep,label,frame,score):
    """Separate entry-direction evidence from fee/R effects in censored trade ranks."""
    z = frame.copy()
    z['score'] = score.loc[z.index]
    # Fixed-SL/TP gross outcomes are often exactly -1/+2. Suppress numerical
    # roundoff that would otherwise turn economic ties into arbitrary ranks.
    z['gross_rounded_r'] = z['gross_r'].round(8)
    result = {}
    for target in ('gross_rounded_r','win','atr_pct'):
        series = ic_series(z,'score',target)
        s = cluster_estimate(series['t_ms'],series['ic'])
        rep.emit(f"{label}/rank_audit/{target}: decisions={s['n']} pairs={int(series['n'].sum())} FMB_IC={s['mean']:+.5f} SE_week={s['se']:.5f} t={s['t']:+.2f}")
        result[target] = s
    rep.emit('秩诊断：gross R保留8位以合并浮点舍入造成的伪并列拆分；净R的SL/TP簇内秩会受费用/初始风险影响。若净R IC强、gross R及胜率IC弱而评分与ATR高度相关，只能提示风险/成本效应，不能据此宣称入场方向优势；仍须检查均值与分位差值CI。')
    return result


def walk_forward_study(rep,df,features,cache,jobs,net,args):
    rep.emit('\n4. 严格样本外组合：50/50留出 + 4折20%起步扩展；76小时训练标签清场，alpha=1固定强正则。')
    rep.emit('训练只用随机all样本；等权特征仅按该训练窗口的内部两半稳定IC选择和定方向，若无稳定特征则不生成等权预测。岭回归使用训练覆盖>=60%的非市场特征；截面z分数、缺失填0、截断±5，禁止用测试标签选特征/正则。')
    rep.emit('分位边界在每个测试决策时刻的完整可用universe上计算（只读取当时特征）；测试交易收益只用随机all样本已有的Backtrader R，避免额外候选池过采样。随机对照为随机all整体，NEW/OLD不事后反向。')
    rep.emit('某折无稳定特征时，等权组合不生成预测；该组合与随机/NEW/OLD的比较限于它实际有预测的时刻，避免不同时段的市场状态混淆。')
    predictions, info = {}, {}
    times = np.unique(df['t_ms'])
    for name,folds in (('holdout',1),('expanding4',4)):
        pieces, fits = [],[]
        for fold,(tr,te) in enumerate(walk_forward(times,folds),1):
            pred,fit = fit_scores(df,features,cache,tr,te)
            pred['fold'] = fold
            pred['t_ms'] = df.loc[pred.index,'t_ms']
            pieces.append(pred)
            fits.append(fit)
            rep.emit(f'{name} fold={fold} train_end={stamp(tr[-1])} test={stamp(te[0])}..{stamp(te[-1])} ntrain_times={len(tr)} test_times={len(te)} stable={fit["stable"]} ridge_features={len(fit["ridge_features"])}')
        pred = pd.concat(pieces).sort_index()
        predictions[name],info[name] = pred,fits
        pd.concat([df.loc[pred.index,['sym','t_ms','all']],pred.drop(columns='t_ms')],axis=1).to_csv(Path(args.out)/f'{name}_predictions.csv.gz',index=False)
        rep.summary[name] = {}
        mask = df['all'].to_numpy() & df.index.isin(pred.index)
        tops = {}
        for method in ('equal','ridge','new','old'):
            scores = pred[method].reindex(df.index)
            if not scores.notna().any():
                rep.emit(f'{name}/{method}: 无训练期稳定特征/无可用模型，未生成得分；不得解读为0收益。')
                continue
            q = quintiles(df,scores)
            method_mask = mask & scores.notna().to_numpy()
            common_ix,_ = strategy_indices(df,jobs,net,method_mask,args.allow_overlap)
            result = quantile_table(rep,f'{name}/{method}',df,q,jobs,net,args,method_mask)
            z = df.loc[method_mask]
            result['ic'] = report_score_ic(rep,f'{name}/{method}',z,scores)
            if name == 'holdout':
                result['rank_diagnostic'] = audit_ranks(rep,f'{name}/{method}',z,scores)
            rep.summary[name][method] = result
            top_trade = df.loc[common_ix].loc[q.loc[common_ix]==5]
            top_raw = df.loc[method_mask & (q==5).to_numpy()]
            tops[method] = (scores.notna().to_numpy(),q)
            result['top_minus_random_R'] = rep.difference(f'{name}/{method} Q5−RANDOM',top_trade,df.loc[common_ix],'net_r')
            rep.difference(f'{name}/{method} Q5−RANDOM',top_raw,df.loc[method_mask],'short_24h')
        for method in ('equal','ridge'):
            for benchmark in ('new','old'):
                if method in tops and benchmark in tops:
                    shared = mask & tops[method][0] & tops[benchmark][0]
                    shared_ix,_ = strategy_indices(df,jobs,net,shared,args.allow_overlap)
                    left = df.loc[shared_ix].loc[tops[method][1].loc[shared_ix]==5]
                    right = df.loc[shared_ix].loc[tops[benchmark][1].loc[shared_ix]==5]
                    rep.difference(f'{name}/{method} Q5−{benchmark} Q5 (matched times)',left,right,'net_r')
        ix,_ = strategy_indices(df,jobs,net,mask,args.allow_overlap)
        rep.estimate(name+'/RANDOM',df.loc[ix],'net_r')
        rep.estimate(name+'/RANDOM',df.loc[mask],'short_24h')
    atomic_json(Path(args.out)/'models.json',info)
    rep.emit('50/50与4折结果时间重叠，不是两份独立样本验证；每个预测严格来自它之前的训练数据。最终敏感性预先固定用holdout/ridge，避免事后挑测试收益最好的模型。')
    return predictions,info


def rule_masks(df,train_times):
    training = df[df['t_ms'].isin(train_times)].drop_duplicates('t_ms')
    masks = {'always':np.ones(len(df),dtype=bool),'btc_bear':(df['regime']=='btc_bear').to_numpy(),
             'not_btc_bull':(df['regime']!='btc_bull').to_numpy()}
    rules = dict(always='always',btc_bear='btc_regime == btc_bear',not_btc_bull='btc_regime != btc_bull')
    for f in ('breadth_negative','breadth_below_ema20','breadth_ema_bear'):
        for q in (.33,.5,.67):
            threshold = float(training[f].quantile(q))
            name = f'{f}>train_q{q}'
            masks[name] = (df[f] >= threshold).to_numpy()
            rules[name] = f'{f} >= {threshold:.6f}'
    return masks,rules


def market_switch(rep,df,predictions,jobs,net,args):
    rep.emit('\n5. 市场状态开关：只在50/50训练期选择，评价其留出测试期。')
    tr,te = next(walk_forward(np.unique(df['t_ms']),1))
    masks,rules = rule_masks(df,tr)
    trainmask = df['all'].to_numpy() & df['t_ms'].isin(tr).to_numpy()
    tests = df['all'].to_numpy() & df['t_ms'].isin(te).to_numpy()
    fitrows = []
    for name,m in masks.items():
        ix,_ = strategy_indices(df,jobs,net,trainmask&m,args.allow_overlap)
        s = cluster_estimate(df.loc[ix,'t_ms'],df.loc[ix,'net_r'])
        valid = s['n'] >= 100 and s['weeks'] >= 8 and np.isfinite(s['se'])
        utility = s['mean']-1.96*s['se'] if valid else -math.inf
        fitrows.append((utility,name,s))
        rep.emit(f"train switch {name} ({rules[name]}): n={s['n']} meanR={s['mean']:+.5f} SE_week={s['se']:.5f} selection_LCB={utility:.5f}")
    utility,chosen,_ = max(fitrows,key=lambda v:v[0])
    if not np.isfinite(utility):
        chosen = 'always'
    rep.emit(f'锁定规则={chosen}: {rules[chosen]}；从{len(masks)}个预先声明规则中选训练期均值95%下界最大者（>=100交易且>=8周）。')
    score = predictions['holdout']['ridge'].reindex(df.index)
    q = quintiles(df,score)
    top = (q==5).to_numpy()
    result,frames = {},{}
    for name,mask in (('all/no_switch',tests),('all/switch',tests&masks[chosen]),
                      ('top/no_switch',tests&top),('top/switch',tests&top&masks[chosen])):
        ix,blocked = strategy_indices(df,jobs,net,mask,args.allow_overlap)
        z = df.loc[ix]
        s = rep.estimate(name,z,'net_r')
        dd = drawdown_stats(z,args.bootstrap,args.seed)
        rep.emit(f"{name}: blocked={blocked} exit-time maxDD={dd['dd']:.3f}R block_SE={dd['se']:.3f} n={dd['n']} sumR={z['net_r'].sum():+.3f}; equal-initial-R/trade，非资金曲线百分比")
        frames[name] = z
        result[name] = dict(net_r=s,drawdown=dd)
    delta = rep.difference('switch all − no_switch all',frames['all/switch'],frames['all/no_switch'],'net_r')
    rep.difference('switch top − no_switch top',frames['top/switch'],frames['top/no_switch'],'net_r')
    rep.difference('top/no_switch − all/no_switch',frames['top/no_switch'],frames['all/no_switch'],'net_r')
    rep.summary['market_switch'] = dict(rule=chosen,definition=rules[chosen],results=result,delta=delta)
    # Descriptive state splits in test period, fixed definitions (no fitted thresholds).
    for regime in ('btc_bear','chop','btc_bull'):
        ix,_ = strategy_indices(df,jobs,net,tests&(df['regime']==regime).to_numpy(),args.allow_overlap)
        rep.estimate('test '+regime,df.loc[ix],'net_r')
    return tests,top


def sensitivity(rep,df,jobs,args,signature,tests,top):
    rep.emit('\n6. 敏感性：最终固定holdout/ridge，SL1.5ATR/3ATR各保留TP=2R；SL2ATR且TP=3R。仅复跑测试随机all（Top是其子集）。')
    results = {}
    js = [j for j in jobs if tests[j['row_id']]]
    for sl_mult,tp_r in ((1.5,2),(3,2),(2,3)):
        changed = []
        for j in js:
            risk = sl_mult*j['atr']
            if j['ref']-tp_r*risk <= 0:
                continue
            changed.append(dict(j,sl=j['ref']+risk,tp=j['ref']-tp_r*risk))
        label = f'sensitivity_sl{sl_mult}_tp{tp_r}'
        rr = bd.run_checkpoint(changed,args,label,signature)
        copy = df.copy()
        copy['net_r'] = np.nan
        copy['exit_ms'] = np.nan
        for j,r in zip(changed,rr):
            if r['r'] is not None:
                copy.loc[j['row_id'],['net_r','exit_ms']] = r['r'],r['exit_ms']
        tables = {}
        for name,m in (('all',tests),('top',tests&top)):
            ix,_ = strategy_indices(copy,changed,rr,m,args.allow_overlap)
            tables[name] = copy.loc[ix]
            rep.estimate(label+'/'+name,tables[name],'net_r')
        results[label] = rep.difference(label+' top−all',tables['top'],tables['all'],'net_r')
    rep.summary['sensitivity'] = results


def clean_json(x):
    if isinstance(x,dict):
        return {str(k):clean_json(v) for k,v in x.items()}
    if isinstance(x,(list,tuple)):
        return [clean_json(v) for v in x]
    if isinstance(x,np.ndarray):
        return clean_json(x.tolist())
    if isinstance(x,(np.integer,np.bool_)):
        return x.item()
    if isinstance(x,(float,np.floating)):
        return float(x) if np.isfinite(x) else None
    return x


def conclusion(rep, rejected, features, coverage):
    rep.emit('\n结论')
    pools = rep.summary['pools']
    for name in ('loser_gate','gainer','loser_nogate'):
        s,d = pools[name]['net_r'],pools[name]['differences']['net_r']
        significant = np.isfinite(d['lo']) and d['lo'] > 0
        verdict = '出现名义正向差异，仍待独立历史验证' if significant else ('出现名义负向差异' if np.isfinite(d['hi']) and d['hi'] < 0 else '没有证据证明比随机选币有增量')
        rep.emit(f"1. {name}: n={s['n']} mean_netR={s['mean']:+.4f}, 相对all Δ={d['mean']:+.4f} 95%CI=[{d['lo']:+.4f},{d['hi']:+.4f}]：{verdict}。")
    d = rep.summary['gainer_minus_loser']
    direction = ('追高方向名义较好' if d['lo'] > 0 else '破位方向名义较好' if d['hi'] < 0 else '两方向差异不显著，无法判断哪个更好')
    rep.emit(f"1. 追高−破位 netR差={d['mean']:+.4f}, 95%CI=[{d['lo']:+.4f},{d['hi']:+.4f}]；{direction}。")
    hold = rep.summary.get('holdout',{})
    for name in ('equal','ridge','new','old'):
        if name not in hold:
            rep.emit(f'2. {name}：训练期未筛出稳定特征，无法主张该组合有效。')
            continue
        d = hold[name]['deltaR']
        ic = hold[name]['ic']['excess_24h']
        good = (np.isfinite(d['lo']) and d['lo'] > 0 and ic['lo'] > 0
                and hold[name]['monotonicity']=='increasing' and hold[name]['monotonicity_R']=='increasing')
        rep.emit(f"2. holdout/{name}: Q5−Q1 netR={d['mean']:+.4f}, 95%CI=[{d['lo']:+.4f},{d['hi']:+.4f}], excess24 IC={ic['mean']:+.4f}±{ic['se']:.4f}；{'留出期满足正向IC、五分位单调和名义显著R差异，仍需独立验证' if good else '未同时满足正向IC、五分位单调和显著R差异，不能证明分数越高做空越好'}。")
    sw = rep.summary['market_switch']
    a,b = sw['results']['all/switch']['net_r'],sw['results']['all/no_switch']['net_r']
    rep.emit(f"3. 训练选择市场开关={sw['definition']}；测试all开关开 n={a['n']} mean_netR={a['mean']:+.4f}±{a['se']:.4f}，无开关 n={b['n']} mean_netR={b['mean']:+.4f}±{b['se']:.4f}；显著性以第5节共享周差值CI为准，不能凭点估计宣称开关优势。应先评估市场择时风险，再讨论选币；本研究不能直接授权上线。")
    rep.emit('被拒绝的特征及原因（对三个目标逐项列出；市场常数转由第5节研究）：')
    for feature in features:
        reasons = [f'{target}: {rejected[feature,target]}' for target in TARGETS if rejected[feature,target]!='pass']
        if reasons:
            rep.emit(f'  {feature} coverage={coverage[feature]:.1%}: '+ '; '.join(reasons))
    rep.emit('单一历史样本约2025-10~2026-10，主要偏空/震荡；固定事后universe存在存活/成分偏差，不能外推牛市、其他年份或实盘收益。重叠标签/周际相关、多重比较、有限周数会放大不确定性；报告不把不显著差异写成优势。')
    rep.emit('24h目标是价格收益而非交易现金流；资金费仅作特征，不计入Backtrader现金流。没有市值/成交额历史成分、借贷/强平和容量约束。任何特征缺失或简化复刻均不等同于真实扫描器的历史表现。')


def make_report(args,meta,df,jobs,net,gross,signature):
    rep = Report(args.out)
    features = feature_names(df)
    rep.emit('Short features study / Backtrader / causal, numpy+pandas only')
    rep.emit(f"决策UTC {stamp(meta['decisions'][0])}..{stamp(meta['decisions'][-1])}; times={len(meta['decisions'])}, universe={len(meta['selected'])}/{meta['eligible']}, rows={len(df)}, Backtrader_jobs={len(jobs)}")
    rep.emit(f"selected={meta['selected']}; dropped_gaps={meta['dropped_gaps']}; no_data={meta['no_data']}; excluded={meta['excluded']}")
    rep.emit(f"source_SHA256={meta['data_sha256']}; run_cache_signature={signature}; report_source_SHA256={hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}; command={meta['command']}")
    rep.emit(f"因果截断自检={meta['causal_checks']}; 只使用close_time<=t的完整连续窗口；Go EMA/ATR/RSI复用breakdown_study，1h60根/4h84根重播种。")
    rep.emit('入场=t后下一根5m开盘做空；SL=ref+2ATR14(1h)，TP=ref−4ATR，72h；ict_bt.plain_rules / run_jobs -> Backtrader，Pool(maxtasksperchild=200)。费用0.05%/边+0.03%滑点，gross用comm=slip=0独立重跑。')
    rep.emit('未来收益参考ref=下一根5m开盘；short_h=−(close(t+h)/ref−1)，excess_h=short_h+BTC同期上涨收益，即空币+多BTC(β=1)。12h/24h都保存，三类主要IC目标为netR/short24/excess24。')
    rep.emit('new_low用上一根之前24根1h低点比较当前low（close_new_low另列）；dist_low7/区间位置包括当前已收盘4h bar，所有特征均无未来窗口。ADX使用Wilder DMI扩展播种，属于明确简化定义。')
    rep.emit('涨幅评分：stretch/overbought/extension/parabolic公式复刻Go，输入统一60h/84×4h闭合窗口；volume_fade用历史1h成交额。pump_score为这五项等权均值，有缺项则NaN；pump_price_score单独用前四项。不复刻完整九项权重、OI/账户多空比、confirmed、BTC haircut、实时ticker和tuner。NEW/OLD沿用原score_windows，fundingHealth=50、OLD不含funding_rollover。')
    rep.emit('历史fundingRate是已结算费率，不是决策时刻可预见的下一期费率；只在fundingTime<=t后使用，超过16h不前填。1h volume/taker数据只在Binance closeTime+1<=t可用，整24h窗口缺失则量特征NaN。')
    rep.emit('所有均值/胜率/FMB及差值SE按Monday-aligned UTC周聚类；95%CI为1.96×SE探索性近似。池化Spearman与回撤用周块自助SE。IC保留全部有效随机交易标签，不先过滤重叠；交易策略表独立过滤重叠，分位诊断共用all队列后切分。')
    rep.emit('随机12币从决策时有实际5m已收盘价格的全universe抽取，不要求4h闸门/84根历史；历史不足的特征保留NaN，只有60根完整1h可得ATR时才建交易。广度逐指标剔除NaN分母，不用候选池算广度；趋势未知的跌幅前20不归入不过闸门组。')
    rep.emit(f"抽样seed={args.seed}；币种调试抽样沿用原研究seed=11；--max-decisions={args.max_decisions}（0=全部）。--aux-cache={args.aux_cache or args.cache}; --skip-fetch={args.skip_fetch}")
    coverage = {f:float(np.isfinite(df[f]).mean()) for f in features}
    rep.emit('\n特征覆盖率（全universe可计算样本，NaN不填0后做单特征IC）：')
    for f in features:
        rep.emit(f'{f}: n={int(np.isfinite(df[f]).sum())}/{len(df)} coverage={coverage[f]:.2%}')
    rep.emit('无效net结果='+str(dict(collections.Counter(r['how'] for r in net if r['r'] is None))))
    audit_pools(rep,df,jobs,net,args)
    cache,stable,rejected = analyze_ics(rep,df,features,args)
    rep.emit('\n3. 原始特征/NEW/OLD五分位诊断（使用全样本选出的特征，仅为描述性；禁止称为样本外验证）')
    chosen = sorted({f for target in TARGETS for f,_ in stable[target]} | {'new','old'})
    for f in chosen:
        quantile_table(rep,f,df,quintiles(df,df[f]),jobs,net,args)
    predictions,info = walk_forward_study(rep,df,features,cache,jobs,net,args)
    tests,top = market_switch(rep,df,predictions,jobs,net,args)
    sensitivity(rep,df,jobs,args,signature,tests,top)
    conclusion(rep,rejected,features,coverage)
    rep.summary['coverage'] = coverage
    atomic_json(Path(args.out)/'summary.json',clean_json(rep.summary))
    try:
        shutil.copyfile(rep.path,NOTES)
        rep.emit(f'报告副本已复制到 {NOTES}')
    except OSError as exc:
        rep.emit(f'报告副本写入被拒绝/失败：{exc}；请手工复制 {rep.path} 到 {NOTES}')
    return rep


def parser():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--cache',default='/Users/zhangyun/workspace/nofx/data/backtest_cache')
    ap.add_argument('--out',default=str(Path(__file__).resolve().parents[2]/'data/short_features_study'))
    ap.add_argument('--aux-cache',help='optional separate writable root for NEW ict1h/fund; 5m still read from --cache')
    ap.add_argument('--universe-json',default=str(bd.SCRATCH/'universe2.json'))
    ap.add_argument('--universe',default='crypto')
    ap.add_argument('--procs',type=int,default=9)
    ap.add_argument('--sample',type=int,default=0,help='debug coin sample; 0=all, original study seed=11')
    ap.add_argument('--seed',type=int,default=20261018,help='per-decision random12 and bootstrap seed')
    ap.add_argument('--fetch-only',action='store_true',help='resume auxiliary downloads and exit; no trades')
    ap.add_argument('--skip-fetch',action='store_true',help='offline: use only existing auxiliary cache, missing=NaN')
    ap.add_argument('--allow-overlap',action='store_true')
    ap.add_argument('--max-decisions',type=int,default=0,help='smoke only: evenly spaced decision sample, 0=all')
    ap.add_argument('--bootstrap',type=int,default=39,help='pooled IC/drawdown calendar-week block replicates')
    return ap


def main():
    ap = parser()
    args = ap.parse_args()
    if args.procs < 1 or args.sample < 0 or args.max_decisions < 0 or args.bootstrap < 9:
        ap.error('procs>=1, sample/max-decisions>=0, bootstrap>=9 required')
    if args.fetch_only and args.skip_fetch:
        ap.error('--fetch-only and --skip-fetch are mutually exclusive')
    Path(args.out).mkdir(parents=True,exist_ok=True)
    ctxs,syms,meta = bd.load_data(args)
    if not args.skip_fetch:
        status = fetch_all(args,ctxs,syms)
        failed = sum(not info[k].get('complete') for info in status.values() for k in ('ict1h','fund'))
        print(f'fetch complete: failed symbol/endpoints={failed}/{2*len(status)}; missing features remain NaN',flush=True)
    if args.fetch_only:
        return
    # Source + data + auxiliary file signatures make resumption unambiguous.
    digest = hashlib.sha256(meta['data_sha256'].encode())
    for name in ('short_features_study.py','breakdown_study.py','ict_bt.py','ict.py','backtrader_adapter.py','params.py'):
        digest.update(Path(__file__).with_name(name).read_bytes())
    root = Path(args.aux_cache or args.cache)
    for sym in syms:
        for kind in ('ict1h','fund'):
            p = root/kind/f'{sym}.json'
            digest.update(p.read_bytes() if p.exists() else f'missing:{kind}:{sym}'.encode())
    digest.update(json.dumps(dict(seed=args.seed,max_decisions=args.max_decisions),sort_keys=True).encode())
    signature = digest.hexdigest()
    prepared = Path(args.out)/'prepared.pkl'
    saved_meta = read_json(Path(args.out)/'meta.json',{})
    if prepared.exists() and saved_meta.get('signature') == signature:
        saved = pd.read_pickle(prepared)  # only this script's own local checkpoint
        df,jobs,job_rows,extra = saved
        print(f'resuming prepared features: rows={len(df)} jobs={len(jobs)}',flush=True)
    else:
        df,jobs,job_rows,extra = build_samples(args,ctxs,syms)
        for j,r in zip(jobs,job_rows):
            j['row_id'] = r
        pd.to_pickle((df,jobs,job_rows,extra),prepared)
    meta.update(extra,signature=signature,command=' '.join([sys.executable,'-m','scripts.backtest.short_features_study',*sys.argv[1:]]))
    atomic_json(Path(args.out)/'meta.json',meta)
    if not jobs:
        raise ValueError('No executable jobs')
    net = bd.run_checkpoint(jobs,args,'net',signature)
    gross = bd.run_checkpoint(jobs,args,'gross',signature,True)
    df['net_r'],df['gross_r'],df['win'],df['exit_ms'] = np.nan,np.nan,np.nan,np.nan
    for j,n,g in zip(jobs,net,gross):
        ri = j['row_id']
        if n['r'] is not None:
            df.loc[ri,['net_r','win','exit_ms']] = n['r'],float(n['r']>0),n['exit_ms']
        if g['r'] is not None:
            df.loc[ri,'gross_r'] = g['r']
    df.to_pickle(Path(args.out)/'samples.pkl')
    make_report(args,meta,df,jobs,net,gross,signature)


if __name__ == '__main__':
    main()
