"""Manual-only holdout spending. Every acknowledged attempt is audited FIRST."""
import argparse
import datetime as dt
import json
from pathlib import Path
import sys

if __package__ in (None, ''):
    sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
import numpy as np
from scripts.autoresearch.common import RESEARCH, read_json, sha256
from scripts.autoresearch.evaluate import append_ledger, clean, evaluate_arrays


def judge(candidate, side, horizon, private_dir=RESEARCH / 'private', acknowledged=False):
    if not acknowledged:
        raise ValueError('requires --i-understand-this-spends-the-holdout')
    private = Path(private_dir)
    # Log failures too, before loading data or running any candidate code.
    append_ledger(private / 'judge_log.jsonl', dict(ts=dt.datetime.now(dt.timezone.utc).isoformat(),
                  side=side, horizon=horizon, candidate=str(candidate),
                  candidate_sha256=sha256(candidate) if Path(candidate).is_file() else None, acknowledged=True))
    meta = read_json(private / 'meta.json')
    if sha256(private / 'holdout.npz') != meta['data_sha256']['holdout']:
        raise ValueError('holdout sha256 mismatch')
    with np.load(private / 'holdout.npz', allow_pickle=False) as data:
        if set(data['split']) != {'holdout'}:
            raise ValueError('private dataset must contain holdout only')
        result = evaluate_arrays(candidate, data, meta, side, horizon, 'holdout')
    return clean(result)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--candidate', required=True, type=Path)
    ap.add_argument('--side', required=True, choices=['long', 'short'])
    ap.add_argument('--horizon', required=True, type=int, choices=[12, 24, 72])
    ap.add_argument('--private-dir', type=Path, default=RESEARCH / 'private')
    ap.add_argument('--i-understand-this-spends-the-holdout', required=True, action='store_true')
    args = ap.parse_args()
    try:
        result = judge(args.candidate, args.side, args.horizon, args.private_dir, args.i_understand_this_spends_the_holdout)
    except Exception as exc:
        result = dict(valid=False, reason=str(exc))
    print(json.dumps(result, ensure_ascii=False, allow_nan=False))
    return 0 if result['valid'] else 2


if __name__ == '__main__':
    raise SystemExit(main())
