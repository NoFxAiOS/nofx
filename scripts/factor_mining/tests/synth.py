"""Synthetic fixtures only; never reads a real data directory or uses a network."""
import json
from pathlib import Path

import numpy as np
import pandas as pd

from scripts.factor_mining.panel import Panels

SIGNAL_EXPR = '$close/$open-1'
SIGNAL_SEED = {'expr': SIGNAL_EXPR, 'family': 'momentum', 'name': 'PLANTED'}


def planted(rows=650, columns=25, seed=17, market='crypto'):
    rng = np.random.default_rng(seed)
    index = pd.date_range('2020-01-01', periods=rows, freq='B' if market == 'us' else 'D')
    signal = rng.normal(size=(rows, columns))
    # Decision t's close/open encodes a known predictor of open[t+2]/open[t+1].
    increments = rng.normal(0, .002, (rows, columns))
    increments[2:] += .012 * signal[:-2]
    opens = 100 * np.exp(np.cumsum(increments, axis=0))
    close = opens * np.exp(.02 * signal)
    high = np.maximum(opens, close) * 1.005
    low = np.minimum(opens, close) * .995
    volume = rng.lognormal(10, .3, (rows, columns))
    fields = {name: pd.DataFrame(x, index=index, columns=[f'S{i:03}' for i in range(columns)])
              for name, x in zip(('open', 'high', 'low', 'close', 'volume', 'vwap'),
                                 (opens, high, low, close, volume, close))}
    mask = pd.DataFrame(True, index=index, columns=fields['close'].columns)
    benchmark = pd.Series(100 * np.exp(np.cumsum(rng.normal(0, .001, rows))), index=index)
    return Panels(fields, mask, benchmark.copy() if market == 'us' else None,
                  benchmark.copy() if market == 'us' else None, {'market': market})


def write_market(root, market='crypto', rows=650, columns=25, seed=17):
    panels = planted(rows, columns, seed, market)
    base = Path(root) / market / 'daily'
    base.mkdir(parents=True, exist_ok=True)
    for symbol in panels.fields['close'].columns:
        data = [[d.strftime('%Y-%m-%d'), *(float(panels.fields[f].at[d, symbol])
                    for f in ('open', 'high', 'low', 'close', 'volume', 'vwap'))] for d in panels.mask.index]
        (base / f'{symbol}.json').write_text(json.dumps(data), encoding='utf-8')
    if market == 'us':
        data = [[d.strftime('%Y-%m-%d'), p, p * 1.01, p * .99, p, 1e6, p]
                for d, p in panels.benchmark_open.items()]
        (base / 'SPY.json').write_text(json.dumps(data), encoding='utf-8')
    return panels
