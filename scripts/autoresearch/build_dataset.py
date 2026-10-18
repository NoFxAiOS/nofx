"""Offline full-universe feature table. No screening, row sampling, or fetching.
Source: existing ict5m / ict1h / fund JSON caches plus alt_collect JSON objects.
All times UTC ms; returns are decimals. Fees=0.0016; excess is gross beta-one
coin minus BTC (short is its negation), not an executable hedged portfolio.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile

if __package__ in (None, ''):
    sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
import numpy as np
import pandas as pd
from scripts.backtest import short_features_study as sf, ict_fetch
from scripts.autoresearch.common import ALT, DATA, RESEARCH, atomic_json, read_json, sha256, universe

HOUR, DAY = sf.HOUR, sf.DAY
HORIZONS = (12, 24, 72)
TARGETS = [f'{side}_{kind}_{h}' for h in HORIZONS for side in ('long', 'short') for kind in ('ret', 'net', 'exc')]
NEW_FEATURES = ([f'oi_{kind}_chg_{h}h' for kind in ('qty', 'value') for h in (4, 24)] +
                [f'oi_price_sign_{h}h' for h in (4, 24)] +
                ['glsr', 'glsr_change_24h', 'toplsp', 'topls_acct', 'toplsp_global_gap', 'topls_acct_global_gap'] +
                [f'taker_ratio_{kind}_{h}h' for kind in ('mean', 'change') for h in (4, 24)] +
                ['premium', 'premium_mean_24h', 'premium_rank_7d', 'fng', 'fng_change_7d'])
BASE_FEATURES = sorted(set(sf.PRICE_KEYS + sf.MARKET + list(sf.volume_features({}, 0)) +
                           list(sf.funding_features(np.empty((0, 2)), 0))))
FEATURES = BASE_FEATURES + NEW_FEATURES


class Series:
    """Sorted unique published rows; every query is bounded by its availability."""
    def __init__(self, rows=()):
        dedup = {int(r['timestamp']): r for r in rows}
        self.rows = [dedup[t] for t in sorted(dedup)]
        self.times = np.array(sorted(dedup), dtype=np.int64)

    def at(self, t, key, max_age=3 * HOUR):
        i = int(np.searchsorted(self.times, t, side='right')) - 1
        if i < 0 or t - self.times[i] > max_age:
            return np.nan
        return float(self.rows[i].get(key, np.nan))

    def window(self, t, hours, key):
        lo, hi = np.searchsorted(self.times, [t - hours * HOUR, t], side='right')
        times = self.times[lo:hi]
        # No filling missing buckets, including inside a rolling window.
        if len(times) < hours or not len(times) or t - times[-1] > 3 * HOUR or np.any(np.diff(times) > HOUR):
            return np.array([], dtype=float)
        values = np.array([float(r.get(key, np.nan)) for r in self.rows[lo:hi]])
        return values if np.isfinite(values).all() else np.array([], dtype=float)

    def prefix(self, t):
        return Series(self.rows[:np.searchsorted(self.times, t, side='right')])


def load_alt(root, sym):
    return {k: Series(read_json(Path(root) / k / f'{sym}.json', []))
            for k in ('oi', 'glsr', 'toplsp', 'topls_acct', 'taker', 'premium')}


def change(series, t, key, h):
    now, past = series.at(t, key), series.at(t - h * HOUR, key)
    return now / past - 1 if np.isfinite(now) and np.isfinite(past) and past > 0 else np.nan


def alt_features(series, fng, t, price):
    out = dict.fromkeys(NEW_FEATURES, np.nan)
    for h in (4, 24):
        for kind, key in (('qty', 'sumOpenInterest'), ('value', 'sumOpenInterestValue')):
            out[f'oi_{kind}_chg_{h}h'] = change(series['oi'], t, key, h)
        a, b = out[f'oi_qty_chg_{h}h'], price.get(f'ret_{h}h', np.nan)
        if np.isfinite([a, b]).all():
            out[f'oi_price_sign_{h}h'] = float(np.sign(a) * np.sign(b))
        vals = series['taker'].window(t, h, 'buySellRatio')
        out[f'taker_ratio_mean_{h}h'] = float(vals.mean()) if len(vals) else np.nan
        out[f'taker_ratio_change_{h}h'] = change(series['taker'], t, 'buySellRatio', h)
    for kind in ('glsr', 'toplsp', 'topls_acct'):
        out[kind] = series[kind].at(t, 'longShortRatio')
    prior = series['glsr'].at(t - DAY, 'longShortRatio')
    out['glsr_change_24h'] = out['glsr'] - prior
    for kind in ('toplsp', 'topls_acct'):
        out[kind + '_global_gap'] = out[kind] - out['glsr']
    out['premium'] = series['premium'].at(t, 'close')
    vals = series['premium'].window(t, 24, 'close')
    out['premium_mean_24h'] = float(vals.mean()) if len(vals) else np.nan
    vals = series['premium'].window(t, 7 * 24, 'close')
    if len(vals) and np.isfinite(out['premium']):
        out['premium_rank_7d'] = float((sum(vals < out['premium']) + .5 * sum(vals == out['premium'])) / len(vals))
    out['fng'] = fng.at(t, 'value', 36 * HOUR)
    out['fng_change_7d'] = out['fng'] - fng.at(t - 7 * DAY, 'value', 36 * HOUR)
    return out


def features_at(ctxs, symbols, aux, alts, fng, t):
    btc = ctxs['BTCUSDT']
    prices = {s: sf.price_features(ctxs[s], btc, t, *aux[s]) if s in ctxs else None for s in symbols}
    active = {s: f for s, f in prices.items() if f is not None}
    market = sf.market_features(active, btc, t) if active and sf.bd.closed_window(btc.a4, t, 84, 4 * HOUR) is not None else {}
    rows = []
    for s in symbols:
        row = dict.fromkeys(BASE_FEATURES, np.nan)
        row.update(prices[s] or {})
        row.update({k: v for k, v in market.items() if k != 'regime'})
        row.update(alt_features(alts[s], fng, t, row))
        rows.append({k: row[k] for k in FEATURES})
    raw = pd.DataFrame(rows, index=symbols)
    raw = raw.astype(float).replace([np.inf, -np.inf], np.nan)
    xs = raw.rank(method='average', pct=True).add_suffix('_xs')
    return pd.concat([raw, xs], axis=1)


def causal_check(ctxs, symbols, aux, alts, fng, t):
    # Reuse the original all-member price/aux/market prefix self-check.
    available = [s for s in symbols if s in ctxs]
    if sf.bd.closed_window(ctxs['BTCUSDT'].a4, t, 84, 4 * HOUR) is not None:
        sf.causal_check(ctxs, available, aux, t)
    clipped = {}
    for s, ctx in ctxs.items():
        a = ctx.a5
        n = np.searchsorted(a['t_close'], t, side='right')
        if n:
            clipped[s] = sf.bd.Context.from_bars(np.column_stack([a['t_open'][:n], *[a[k][:n] for k in ('o', 'h', 'l', 'c')]]).tolist())
    ca = {s: ({k: v[a['t_close'] <= t] for k, v in a.items()} if a else {}, f[f[:, 0] <= t]) for s, (a, f) in aux.items()}
    cs = {s: {k: v.prefix(t) for k, v in a.items()} for s, a in alts.items()}
    a = features_at(ctxs, symbols, aux, alts, fng, t)
    b = features_at(clipped, symbols, ca, cs, fng.prefix(t), t)
    if not np.allclose(a.to_numpy(), b.to_numpy(), rtol=1e-12, atol=1e-12, equal_nan=True):
        raise AssertionError(f'feature prefix mismatch at {t}')
    return dict(t_ms=int(t), symbols=len(symbols), passed=True)


def split_times(times):
    times = np.unique(times)
    if len(times) < 5:
        raise ValueError('need >=5 decision times')
    b1, b2 = times[int(.6 * len(times))], times[int(.8 * len(times))]
    # Remove the trailing 76h from the earlier segment, inclusive boundary.
    return {'train': times[times + sf.PURGE < b1],
            'val': times[(times >= b1) & (times + sf.PURGE < b2)],
            'holdout': times[times >= b2]}


def atomic_npz(path, **arrays):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(prefix=path.name, suffix='.tmp', dir=path.parent)
    try:
        with os.fdopen(fd, 'wb') as stream:
            np.savez_compressed(stream, **arrays)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def save_dataset(frame, out_root, features, source_sha256='', checks=None):
    root = Path(out_root)
    if (root / 'work/freeze.json').exists():
        raise ValueError('dataset is frozen; use a new --out-root and preserve experiment history')
    parts = split_times(frame.t_ms)
    if any(not len(v) for v in parts.values()):
        raise ValueError('not enough history for three nonempty splits plus 76h purges')
    split = np.full(len(frame), 'purged', dtype='<U7')
    for name, times in parts.items():
        split[np.isin(frame.t_ms, times)] = name
    files = {}
    for name, mask, path in (('visible', np.isin(split, ['train', 'val']), root / 'work/visible.npz'),
                             ('holdout', split == 'holdout', root / 'private/holdout.npz')):
        z = frame.loc[mask]
        atomic_npz(path, X=z[features].to_numpy(float), y=z[TARGETS].to_numpy(float),
                   t_ms=z.t_ms.to_numpy(np.int64), sym=z.sym.to_numpy(dtype=str), split=split[mask],
                   feature_names=np.array(features), target_names=np.array(TARGETS))
        files[name] = sha256(path)
    visible = frame.loc[np.isin(split, ['train', 'val'])]
    coverage = {f: float(np.isfinite(visible[f]).mean()) for f in features}
    meta = dict(schema_version=1, features=features, targets=TARGETS, coverage=coverage,
                feature_status={f: 'immature' if coverage[f] < .3 else 'mature' for f in features},
                coverage_scope='train+val only; holdout never influences feature metadata',
                splits={k: dict(start=sf.stamp(v[0]), end=sf.stamp(v[-1]), decisions=len(v),
                                samples=int(np.sum(split == k))) for k, v in parts.items()},
                purge_hours=76, purged_samples=int(sum(split == 'purged')), fee_roundtrip=.0016,
                data_sha256=files, source_sha256=source_sha256, causal_checks=checks or [])
    atomic_json(root / 'work/meta.json', meta)
    atomic_json(root / 'private/meta.json', meta)
    return meta


def build(cache, aux_cache, alt_root, out_root, universe_json=None, sample=0):
    names = universe(universe_json or Path(alt_root) / 'universe_src.json')
    if sample:
        names = names[:sample]  # debug only, deterministic; production keeps every name
    ctxs, digest, missing = {}, hashlib.sha256(), []
    for s in dict.fromkeys([*names, 'BTCUSDT']):
        path = Path(cache) / 'ict5m' / f'{s}.json'
        rows = ict_fetch.load(cache, s)
        if rows:
            ctxs[s] = sf.bd.Context.from_bars(rows)
        else:
            missing.append(s)
        for p in [path, *[Path(aux_cache) / k / f'{s}.json' for k in ('ict1h', 'fund')],
                  *[Path(alt_root) / k / f'{s}.json' for k in ('oi', 'glsr', 'toplsp', 'topls_acct', 'taker', 'premium')]]:
            digest.update(str(p).encode())
            digest.update(sha256(p).encode() if p.exists() else b'missing')
    if 'BTCUSDT' not in ctxs:
        raise ValueError('BTCUSDT 5m reference cache required')
    digest.update(json.dumps(names).encode())
    fp = Path(alt_root) / 'fng.json'
    digest.update(sha256(fp).encode() if fp.exists() else b'missing:fng')
    aux = {s: sf.load_aux(aux_cache, s) for s in names}
    alts = {s: load_alt(alt_root, s) for s in names}
    fng = Series(read_json(fp, []))
    times = sf.decision_times(ctxs['BTCUSDT'])
    if not len(times):
        raise ValueError('no decisions after warmup and label maturity')
    checks = [causal_check(ctxs, names, aux, alts, fng, int(times[i]))
              for i in sorted({0, len(times) // 2, len(times) - 1})]
    chunks = []
    for i, t in enumerate(times):
        z = features_at(ctxs, names, aux, alts, fng, int(t))
        z['sym'], z['t_ms'] = names, int(t)
        for h in HORIZONS:
            btc_ret = sf.forward_return(ctxs['BTCUSDT'], int(t), h)
            r = np.array([sf.forward_return(ctxs[s], int(t), h) if s in ctxs else np.nan for s in names])
            for side, sign in (('long', 1), ('short', -1)):
                z[f'{side}_ret_{h}'], z[f'{side}_net_{h}'] = sign * r, sign * r - .0016
                z[f'{side}_exc_{h}'] = sign * (r - btc_ret)
        chunks.append(z)
        if i % 120 == 0:
            print(f'features {i + 1}/{len(times)} rows={(i + 1) * len(names)}', flush=True)
    features = FEATURES + [f + '_xs' for f in FEATURES]
    meta = save_dataset(pd.concat(chunks, ignore_index=True), out_root, features, digest.hexdigest(), checks)
    meta['missing_price_files'] = missing
    atomic_json(Path(out_root) / 'work/meta.json', meta)
    atomic_json(Path(out_root) / 'private/meta.json', meta)
    return meta


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--cache', type=Path, default=DATA / 'backtest_cache')
    ap.add_argument('--aux-cache', type=Path, help='ict1h/fund root; default --cache')
    ap.add_argument('--alt-root', type=Path, default=ALT)
    ap.add_argument('--out-root', type=Path, default=RESEARCH)
    ap.add_argument('--universe-json', type=Path)
    ap.add_argument('--sample', type=int, default=0)
    args = ap.parse_args()
    if args.sample < 0:
        ap.error('--sample must be nonnegative')
    meta = build(args.cache, args.aux_cache or args.cache, args.alt_root, args.out_root, args.universe_json, args.sample)
    print(json.dumps(meta['splits'], ensure_ascii=False))


if __name__ == '__main__':
    main()
