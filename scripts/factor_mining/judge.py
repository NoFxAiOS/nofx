"""Explicitly spend the sealed period on the committed ensemble; never mine it."""
import argparse
import hashlib
import json
from pathlib import Path

import numpy as np

from .config import MARKETS
from .data_fetch import ROOT
from .ensemble import ensemble_scores
from .evaluate import cs_rank_ic, segment_metrics
from .panel import load_panels, make_splits
from . import store


def judge(market, h, root):
    cfg = MARKETS[market]
    if h not in cfg['horizons']:
        raise ValueError(f'unsupported {market} horizon: {h}')
    root = Path(root)
    with store.FileLock(root):
        ensemble = store.read_json(root / market / 'ensemble.json', {})
        committed = ensemble.get('horizons', {}).get(str(h))
        if not committed or not committed['factor_ids']:
            raise ValueError('no committed ensemble for this horizon')
        ids = committed['factor_ids']
        zoo = store.read_json(root / market / 'zoo.json', {})
        by_id = {f['id']: f for f in zoo.get('factors', []) if f['horizon'] == h}
        factors = [by_id[key] for key in ids]
        panels = load_panels(market, root)
        segment = make_splits(panels.mask.index, h)['holdout']
        score = ensemble_scores(factors, panels).loc[segment]
        # This is the sole path that constructs and reads holdout labels.
        opens = panels.fields['open']
        raw = (opens.shift(-1-h).loc[segment] / opens.shift(-1).loc[segment] - 1)
        raw = raw.replace([np.inf, -np.inf], np.nan)
        mask = panels.mask.loc[segment]
        if panels.benchmark_open is None:
            baseline = raw.where(mask).mean(axis=1)
        else:
            benchmark = panels.benchmark_open
            baseline = benchmark.shift(-1-h).loc[segment] / benchmark.shift(-1).loc[segment] - 1
        excess = raw.sub(baseline, axis=0)
        ic = cs_rank_ic(score, excess, mask, cfg['min_names'])
        result = segment_metrics(ic, score, raw, mask, segment.loc[segment], h, cfg)
        digest = hashlib.sha256(json.dumps(ids, separators=(',', ':')).encode()).hexdigest()
        row = dict(ts=store.timestamp(), market=market, horizon=h, factor_ids_hash=digest,
                   factor_ids=ids, result=result)
        store.append_jsonl(root / 'holdout_spent.jsonl', [row])
        return row


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--market', required=True, choices=MARKETS)
    parser.add_argument('--horizon', required=True, type=int)
    parser.add_argument('--i-understand-this-spends-the-holdout', action='store_true')
    parser.add_argument('--root', type=Path, default=ROOT)
    args = parser.parse_args(argv)
    if not args.i_understand_this_spends_the_holdout:
        parser.error('refusing to spend holdout without --i-understand-this-spends-the-holdout')
    print(json.dumps(store.finite(judge(args.market, args.horizon, args.root)), allow_nan=False,
                     ensure_ascii=False))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
