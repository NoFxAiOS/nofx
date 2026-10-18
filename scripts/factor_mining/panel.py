"""Offline point-in-time panels. Public targets expose train/val only.

Splits contain boolean Series plus ``ranges`` with inclusive date endpoints.
``forward_target`` returns (excess, raw); sealed and purged rows are NaN.
Only the explicit forward_labels path and manual judge read later labels.
"""
from dataclasses import dataclass
import json
from pathlib import Path
import re

import numpy as np
import pandas as pd

from .config import MARKETS, SPLIT
from .data_fetch import ROOT

HOLDOUT_SEALED = True
FIELD_NAMES = ('open', 'high', 'low', 'close', 'volume', 'vwap')


@dataclass
class Panels:
    fields: dict[str, pd.DataFrame]
    mask: pd.DataFrame
    benchmark_close: pd.Series | None = None
    benchmark_open: pd.Series | None = None
    meta: dict | None = None
    # Calendar-bar window lengths; None falls back to proportional splits.
    windows: dict[str, int] | None = None


def _read_symbol(path):
    try:
        rows = json.loads(path.read_text(encoding='utf-8'))
    except (OSError, ValueError):
        rows = []
    valid = {}
    for row in rows if isinstance(rows, list) else []:
        try:
            if (not isinstance(row, list) or len(row) != 7 or
                    not isinstance(row[0], str) or not re.fullmatch(r'\d{4}-\d{2}-\d{2}', row[0])):
                continue
            date = pd.Timestamp(row[0])
            values = np.asarray(row[1:], dtype=float)
            o, high, low, close, volume, vwap = values
            if (pd.isna(date) or date.tzinfo is not None or
                    not np.isfinite(values).all() or
                    min(o, low, close, vwap) <= 0 or volume < 0 or
                    high < max(o, close) or low > min(o, close)):
                continue
            valid[date.normalize()] = values
        except (TypeError, ValueError, OverflowError):
            continue
    return pd.DataFrame.from_dict(valid, orient='index', columns=FIELD_NAMES).sort_index()


def load_panels(market, root=ROOT) -> Panels:
    cfg = MARKETS[market]
    base = Path(root) / market
    symbols = {p.stem: _read_symbol(p) for p in sorted((base / 'daily').glob('*.json'))}
    benchmark = symbols.pop(cfg['benchmark'], None) if cfg['benchmark'] else None
    if cfg['benchmark'] and (benchmark is None or benchmark.empty):
        raise ValueError('US calendar requires SPY daily data')
    symbols = {s: f for s, f in symbols.items() if not f.empty}
    if not symbols:
        raise ValueError(f'no usable daily data for {market}')
    if benchmark is not None:
        calendar = pd.DatetimeIndex(benchmark.index)
    else:
        calendar = pd.DatetimeIndex(sorted(set().union(*(f.index for f in symbols.values()))))
    calendar.name = 'date'
    fields = {name: pd.DataFrame({s: f[name].reindex(calendar) for s, f in symbols.items()},
                                 index=calendar, dtype=float) for name in FIELD_NAMES}
    observed = fields['close'].notna()
    mask = observed & observed.cumsum().ge(cfg['min_history'])
    if cfg['universe_n'] is not None:
        amount = (fields['volume'] * fields['vwap']).rolling(20, min_periods=20).mean()
        # Rank only eligible, currently observed names. Ties resolve by symbol.
        ranks = amount.where(mask).rank(axis=1, ascending=False, method='first')
        mask &= ranks.le(cfg['universe_n'])
    try:
        data_meta = json.loads((base / 'data_meta.json').read_text(encoding='utf-8'))
    except (OSError, ValueError):
        data_meta = {}
    meta = dict(market=market, symbols=len(symbols), start=calendar[0].strftime('%Y-%m-%d'),
                end=calendar[-1].strftime('%Y-%m-%d'), bars=int(observed.to_numpy().sum()),
                universe=cfg['universe_n'] or len(symbols),
                data_updated_at=data_meta.get('updated_at'))
    return Panels(fields, mask.astype(bool),
                  benchmark['close'].reindex(calendar) if benchmark is not None else None,
                  benchmark['open'].reindex(calendar) if benchmark is not None else None, meta,
                  dict(cfg['windows']))


def make_splits(index, h, windows=None) -> dict:
    """Trailing calendar-bar windows, with h+2 bars purged from train/val tails."""
    if h < 1:
        raise ValueError('h must be positive')
    index = pd.DatetimeIndex(index)
    n = len(index)
    rolling = windows is not None and n >= sum(windows.values()) + 2 * (h + 2)
    if rolling:
        a = n - windows['holdout'] - windows['val']
        b = n - windows['holdout']
        start = a - windows['train']
    else:
        a, b = int(n * SPLIT[0]), int(n * (SPLIT[0] + SPLIT[1]))
        start = 0
    bounds = {'train': (start, max(start, a - h - 2)),
              'val': (a, max(a, b - h - 2)), 'holdout': (b, n)}
    result = {'ranges': {}, 'holdout_sealed': HOLDOUT_SEALED, 'rolling': rolling}
    for name, (start, stop) in bounds.items():
        mask = pd.Series(False, index=index, dtype=bool)
        mask.iloc[start:stop] = True
        result[name] = mask
        result['ranges'][name] = (index[start].strftime('%Y-%m-%d'),
                                  index[stop - 1].strftime('%Y-%m-%d')) if stop > start else (None, None)
    return result


def forward_target(panels, h) -> tuple[pd.DataFrame, pd.DataFrame]:
    """After close t, buy open[t+1], sell open[t+1+h]; seal all other rows."""
    splits = make_splits(panels.mask.index, h, panels.windows)
    visible = splits['train'] | splits['val']
    # Restrict before computing ratios/benchmark means; never return sealed labels.
    entry = panels.fields['open'].shift(-1).loc[visible]
    exit_ = panels.fields['open'].shift(-1-h).loc[visible]
    raw = (exit_ / entry - 1).replace([np.inf, -np.inf], np.nan)
    if panels.benchmark_open is None:
        baseline = raw.where(panels.mask.loc[visible]).mean(axis=1)
    else:
        opens = panels.benchmark_open
        baseline = opens.shift(-1-h).loc[visible] / opens.shift(-1).loc[visible] - 1
    excess = raw.sub(baseline, axis=0)
    return excess.reindex(panels.mask.index), raw.reindex(panels.mask.index)


def forward_labels(panels, h, start) -> tuple[pd.DataFrame, pd.DataFrame]:
    """Display-only labels from start, requiring an observed t+1+h bar.

    Deliberately independent of rolling train/val masks: a registered interval
    stays a forward interval even after it has rolled into validation.
    """
    if h < 1:
        raise ValueError('h must be positive')
    index = panels.mask.index
    positions = np.flatnonzero((index >= pd.Timestamp(start)) &
                               (np.arange(len(index)) + 1 + h < len(index)))
    opens = panels.fields['open']
    entry = opens.iloc[positions + 1].set_axis(index[positions])
    exit_ = opens.iloc[positions + 1 + h].set_axis(index[positions])
    raw = (exit_ / entry - 1).replace([np.inf, -np.inf], np.nan)
    if panels.benchmark_open is None:
        baseline = raw.where(panels.mask.iloc[positions]).mean(axis=1)
    else:
        benchmark = panels.benchmark_open
        baseline = (benchmark.iloc[positions + 1 + h].set_axis(index[positions]) /
                    benchmark.iloc[positions + 1].set_axis(index[positions]) - 1)
    excess = raw.sub(baseline, axis=0)
    return excess.reindex(index), raw.reindex(index)
