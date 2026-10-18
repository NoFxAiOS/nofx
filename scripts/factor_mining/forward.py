"""Pre-registered, display-only checks; never feed these results into selection."""
from collections import Counter, defaultdict

import numpy as np
import pandas as pd

from . import evaluate, panel


VERDICTS = ('pending', 'confirmed', 'inconclusive', 'failed')


def register(entry, splits_ranges_for_h, accepted_at):
    """Freeze the first accepted direction and the then-unread holdout start."""
    if 'forward' not in entry and entry['status'] == 'accepted':
        entry['forward'] = dict(accepted_at=accepted_at,
                                start=splits_ranges_for_h['holdout'][0],
                                direction=entry['direction'], verdict='pending',
                                n_days=0, min_days=max(40, 4 * entry['horizon']),
                                last_date=None, ic=None, icir=None, t=None,
                                ls_ret=None, ls_sharpe=None, turnover=None, curve=[])
    return entry.get('forward')


def verdict(n_days, min_days, ic, t, ls_ret):
    if n_days < min_days:
        return 'pending'
    if ic is not None and ic <= 0:
        return 'failed'
    if ic is not None and ic > 0 and t is not None and t >= 1.65 and ls_ret is not None and ls_ret > 0:
        return 'confirmed'
    return 'inconclusive'


def evaluate_forward(entry, values, panels, h, cfg, *, labels=None):
    """Return a complete forward record using the frozen direction.

    A caller may share labels constructed for an earlier start in this horizon.
    Slice before any statistics, even when supplied labels contain other dates.
    """
    registered = entry['forward']
    start = registered['start']
    excess, raw = labels if labels is not None else panel.forward_labels(panels, h, start)
    dates = panels.mask.index >= pd.Timestamp(start)
    values, excess, raw, mask = (x.loc[dates] for x in (values, excess, raw, panels.mask))
    # Match evaluate_factor's treatment of machine-precision constant rows.
    eligible = values.where(mask)
    span = eligible.max(axis=1) - eligible.min(axis=1)
    scale = eligible.abs().max(axis=1).clip(lower=1.)
    values = values.where(span > 10 * np.finfo(float).eps * scale, axis=0)
    values = values * registered['direction']
    ic = evaluate.cs_rank_ic(values, excess, mask, cfg['min_names'])
    segment = pd.Series(True, index=values.index)
    # segment_metrics delegates to nw_t(lag=h) and long_short unchanged.
    metrics = evaluate.segment_metrics(ic, values, raw, mask, segment, h, cfg)
    sample = ic.dropna()
    curve = sample.cumsum()
    if len(curve) > 120:
        curve = curve.iloc[np.linspace(0, len(curve) - 1, 120, dtype=int)]
    n_days, min_days = metrics.pop('n'), max(40, 4 * h)
    return dict(accepted_at=registered['accepted_at'], start=start,
                direction=registered['direction'], n_days=n_days, min_days=min_days,
                last_date=sample.index[-1].strftime('%Y-%m-%d') if len(sample) else None,
                **metrics, verdict=verdict(n_days, min_days, metrics['ic'], metrics['t'], metrics['ls_ret']),
                curve=[[d.strftime('%Y-%m-%d'), float(v)] for d, v in curve.items()])


def summarize(factors):
    """Counts for horizons[h].forward, restricted to currently accepted rows."""
    counts = defaultdict(Counter)
    for factor in factors:
        h = str(factor['horizon'])
        counts[h]  # Include horizons containing only non-accepted rows.
        if factor['status'] == 'accepted' and 'forward' in factor:
            counts[h][factor['forward'].get('verdict', 'pending')] += 1
    return {h: {v: count[v] for v in VERDICTS} for h, count in counts.items()}
