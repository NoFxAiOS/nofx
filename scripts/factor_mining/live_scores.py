"""Latest closed-day ensemble scores for reference only; never used in selection."""
from collections import Counter
import datetime as dt
from pathlib import Path

import numpy as np
import pandas as pd

from . import store
from .config import MARKETS
from .data_fetch import ROOT
from .ensemble import ensemble_scores, select_ensemble_factors


def export_live_scores(market, factors, panels, *, root=ROOT, values_cache=None, now=None):
    """Atomically write a snapshot, or remove a stale file if no horizon is usable.

    Use the latest panel date strictly before today in UTC, without falling back
    to older dates for thin cross sections. ``universe_size`` counts the mask's
    eligible names on that date, before filtering each horizon's finite scores.
    Inputs, including the caller's expression cache, are never modified.
    """
    generated = pd.Timestamp(now if now is not None else dt.datetime.now(dt.timezone.utc))
    generated = (generated.tz_localize('UTC') if generated.tzinfo is None
                 else generated.tz_convert('UTC'))
    path = Path(root) / market / 'live_scores.json'
    dates = panels.mask.index[pd.to_datetime(panels.mask.index, utc=True) < generated.normalize()]
    if not len(dates):
        path.unlink(missing_ok=True)
        return None
    as_of = dates.max()
    mask = panels.mask.loc[as_of]
    output = dict(version=1, market=market, generated_at=generated.isoformat(),
                  as_of=as_of.strftime('%Y-%m-%d'), universe_size=int(mask.sum()), horizons={})
    for h in MARKETS[market]['horizons']:
        selected = select_ensemble_factors(factors, h)
        if not selected:
            continue
        row = ensemble_scores(selected, panels, values_cache=values_cache).loc[as_of]
        row = row[mask & np.isfinite(row)]
        if len(row) < 10:
            continue
        percentiles = row.rank(pct=True)
        output['horizons'][str(h)] = dict(
            factor_count=len(selected),
            forward=dict(Counter((f.get('forward') or {}).get('verdict', 'none') for f in selected)),
            scores={str(symbol): dict(z=round(float(z), 4), pct=round(float(percentiles[symbol]), 4))
                    for symbol, z in row.items()})
    if not output['horizons']:
        path.unlink(missing_ok=True)
        return None
    store.write_json(path, output)
    return output
