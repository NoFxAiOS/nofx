"""FROZEN EVALUATOR: modifying this file defines a different experiment.

Its SHA256 is recorded in status.json and each market run. Daily RankIC uses
Bartlett/Newey-West HAC with lag=h (the revised T2 evaluation convention).
All entry points used by mining restrict labels to train/val before statistics.
"""
import math
from statistics import NormalDist

import numpy as np
import pandas as pd

from .config import MIN_COVERAGE, VAL_MIN_T


def t_required(K: int) -> float:
    # Symmetry avoids rounding 1-small_p to 1 for large ledgers.
    return max(2.0, -NormalDist().inv_cdf(0.025 / max(K, 1)))


def _row_corr(a, b, min_names=2):
    valid = np.isfinite(a) & np.isfinite(b)
    n = valid.sum(axis=1)
    x, y = np.where(valid, a, 0.), np.where(valid, b, 0.)
    x -= np.divide(x.sum(axis=1), n, out=np.zeros(len(n)), where=n > 0)[:, None]
    y -= np.divide(y.sum(axis=1), n, out=np.zeros(len(n)), where=n > 0)[:, None]
    x, y = np.where(valid, x, 0.), np.where(valid, y, 0.)
    den = np.sqrt(np.einsum('ij,ij->i', x, x) * np.einsum('ij,ij->i', y, y))
    out = np.full(len(n), np.nan)
    np.divide(np.einsum('ij,ij->i', x, y), den, out=out, where=(n >= min_names) & (den > 0))
    return np.clip(out, -1, 1)


def cs_rank_ic(values, target, mask, min_names) -> pd.Series:
    valid = mask & np.isfinite(values) & np.isfinite(target)
    a = values.where(valid).rank(axis=1, method='average').to_numpy()
    b = target.where(valid).rank(axis=1, method='average').to_numpy()
    return pd.Series(_row_corr(a, b, min_names), index=values.index, name='ic')


def nw_t(series, lag) -> float:
    x = np.asarray(series, dtype=float)
    x = x[np.isfinite(x)]
    n = len(x)
    if n < 20:
        return 0.0
    mean = float(x.mean())
    centered = x - mean
    variance = float(np.dot(centered, centered) / n)
    for k in range(1, min(int(lag), n - 1) + 1):
        variance += 2 * (1 - k / (lag + 1)) * float(np.dot(centered[k:], centered[:-k]) / n)
    if variance <= 1e-30:
        return math.copysign(1e12, mean) if mean else 0.0
    return float(mean / math.sqrt(variance / n))


def long_short(values, raw, mask, segment, h, cfg):
    """Return dated net holding-period returns and mean two-leg turnovers.

    Schedule is anchored at segment position zero, even if that day is skipped.
    Ties break by stable column order. Each of five equal-sized buckets has >=3.
    """
    positions = np.flatnonzero(np.asarray(segment, dtype=bool))[::h]
    v = values.to_numpy()[positions]
    r = raw.to_numpy()[positions]
    eligible = mask.to_numpy()[positions] & np.isfinite(v) & np.isfinite(r)
    count = eligible.sum(axis=1)
    order = np.argsort(np.where(eligible, v, np.inf), axis=1, kind='stable')
    dates, net, turnovers = [], [], []
    prev_long, prev_short = set(), set()
    for j in np.flatnonzero(count >= max(15, cfg['min_names'])):
        # array_split convention: first remainder buckets receive one extra name.
        sizes = np.full(5, count[j] // 5)
        sizes[:count[j] % 5] += 1
        short = order[j, :sizes[0]]
        long = order[j, count[j] - sizes[-1]:count[j]]
        current_long, current_short = set(long), set(short)
        tl = 1 - len(current_long & prev_long) / len(long)
        ts = 1 - len(current_short & prev_short) / len(short)
        ret = r[j, long].mean() - r[j, short].mean() - cfg['one_way_cost'] * 2 * (tl + ts)
        dates.append(values.index[positions[j]])
        net.append(float(ret))
        turnovers.append((tl + ts) / 2)
        prev_long, prev_short = current_long, current_short
    return pd.Series(net, index=pd.DatetimeIndex(dates), dtype=float), np.asarray(turnovers)


def segment_metrics(ic, values, raw, mask, segment, h, cfg):
    sample = ic.loc[segment].dropna()
    mean = float(sample.mean()) if len(sample) else 0.0
    std = float(sample.std(ddof=1)) if len(sample) > 1 else 0.0
    net, turnover = long_short(values, raw, mask, segment, h, cfg)
    annual = cfg['periods_per_year'] / h
    rmean = float(net.mean()) if len(net) else 0.0
    rstd = float(net.std(ddof=1)) if len(net) > 1 else 0.0
    return dict(ic=mean, icir=mean / std if std > 1e-15 else 0.0,
                t=nw_t(sample, h), ls_ret=rmean * annual,
                ls_sharpe=rmean / rstd * math.sqrt(annual) if rstd > 1e-15 else 0.0,
                turnover=float(turnover.mean()) if len(turnover) else 0.0, n=len(sample))


def evaluate_factor(values, tgt_excess, tgt_raw, mask, split_masks, h, cfg) -> dict | None:
    visible = split_masks['train'] | split_masks['val']
    # Slice immediately: even poisoned labels elsewhere cannot affect validity.
    values, excess, raw, universe = (x.loc[visible] for x in (values, tgt_excess, tgt_raw, mask))
    segments = {k: split_masks[k].loc[visible] for k in ('train', 'val')}
    eligible = universe.to_numpy().sum()
    covered = (universe & np.isfinite(values)).to_numpy().sum()
    if not eligible or covered / eligible < MIN_COVERAGE:
        return None
    # Ignore machine-precision cross-sectional jitter around a constant.
    eligible_values = values.where(universe)
    span = eligible_values.max(axis=1) - eligible_values.min(axis=1)
    scale = eligible_values.abs().max(axis=1).clip(lower=1.)
    varying = span > 10 * np.finfo(float).eps * scale
    values = values.where(varying, axis=0)
    ic = cs_rank_ic(values, excess, universe, cfg['min_names'])
    if ic.notna().sum() < 60:
        return None
    train_mean = ic.loc[segments['train']].mean()
    direction = -1 if train_mean < 0 else 1
    # Ensemble calls this same evaluator with a precommitted positive direction.
    direction = cfg.get('direction', direction)
    ic, values = ic * direction, values * direction
    train = segment_metrics(ic, values, raw, universe, segments['train'], h, cfg)
    val = segment_metrics(ic, values, raw, universe, segments['val'], h, cfg)
    train_days = ic.loc[segments['train']]
    halves = (train_days.iloc[:len(train_days)//2], train_days.iloc[len(train_days)//2:])
    train['ic_halves'] = [float(x.mean()) if x.notna().any() else 0.0 for x in halves]
    monthly = ic.resample('MS').mean().dropna()
    return dict(direction=direction, train=train, val=val,
                ic_monthly=[[d.strftime('%Y-%m'), float(v)] for d, v in monthly.items()])


def classify(train, val, K, t_req):
    halves = train['ic_halves']
    if train['t'] < t_req or np.sign(halves[0]) != np.sign(halves[1]) or train['ic'] <= 0:
        return 'rejected'
    if val['ic'] > 0 and val['t'] >= VAL_MIN_T and val['ls_ret'] > 0 and train['ls_ret'] > 0:
        return 'accepted'
    return 'candidate'


def cross_corr(rankA, rankB, dates_sample):
    a, b = rankA.reindex(dates_sample), rankB.reindex(dates_sample)
    correlations = _row_corr(a.to_numpy(), b.to_numpy())
    valid = correlations[np.isfinite(correlations)]
    return float(valid.mean()) if len(valid) else 0.0
