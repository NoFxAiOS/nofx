"""Nightly factor search CLI. Fetching can be disabled for fully offline runs."""
import argparse
import datetime as dt
import json
from pathlib import Path
import sys
import time

from . import data_fetch, store
from .config import MARKETS
from .miner import Miner, frozen_hashes


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--market', choices=(*MARKETS, 'all'), required=True)
    parser.add_argument('--mutations', type=int, default=300)
    parser.add_argument('--llm', type=int, default=20)
    parser.add_argument('--no-llm', action='store_true')
    parser.add_argument('--no-fetch', action='store_true')
    parser.add_argument('--budget-min', type=float, default=25)
    parser.add_argument('--seed', type=int, default=int(dt.datetime.now().strftime('%Y%m%d')))
    parser.add_argument('--root', type=Path, default=data_fetch.ROOT)
    args = parser.parse_args(argv)
    if args.mutations < 0 or args.llm < 0 or args.budget_min < 0:
        parser.error('mutations, llm and budget-min must be nonnegative')
    failed = False
    try:
        with store.FileLock(args.root):
            for market in MARKETS if args.market == 'all' else (args.market,):
                miner = Miner(args.root)
                entered_miner = False
                started_dt, clock = dt.datetime.now(dt.timezone.utc), time.monotonic()
                started = started_dt.isoformat()
                error = None
                try:
                    if not args.no_fetch:
                        store.write_status(dict(running=True, current=dict(market=market, stage='fetch',
                                                   started_at=started), **frozen_hashes()), args.root)
                        try:
                            fetched = data_fetch.update(market, root=args.root)
                            failures = fetched.get('failed') or {}
                            # A few delisted/unknown tickers are routine; only a broad failure is a run error.
                            if failures and len(failures) > 0.25 * (len(failures) + fetched.get('ok', 0)):
                                miner.fetch_error = 'fetch: ' + json.dumps(failures, ensure_ascii=False)
                            elif failures:
                                print(f'{market}: fetch skipped {sorted(failures)}', file=sys.stderr)
                        except Exception as exc:
                            miner.fetch_error = f'fetch: {type(exc).__name__}: {exc}'
                    entered_miner = True
                    miner.fetch_seconds = time.monotonic() - clock
                    miner.run_market(market, n_mutations=args.mutations, n_llm=args.llm,
                                     budget_s=args.budget_min * 60, seed=args.seed, use_llm=not args.no_llm,
                                     now=started_dt)
                    if miner.fetch_error:
                        print(miner.fetch_error, file=sys.stderr)
                except Exception as exc:
                    error = f'{type(exc).__name__}: {exc}'
                    print(f'{market}: {error}', file=sys.stderr)
                    failed = True
                except BaseException as exc:
                    error = f'{type(exc).__name__}: {exc}'
                    raise
                finally:
                    # Miner owns its one run record. This covers a fetch interrupt.
                    if not entered_miner:
                        try:
                            store.write_status(dict(running=False, current=None), args.root)
                        finally:
                            store.append_run(dict(run_id=f'{market}-{started}', market=market,
                                started_at=started, finished_at=store.timestamp(),
                                seconds=time.monotonic()-clock, reevaluated=0, forward_updated=0,
                                seed_evaluated=0, mutation_evaluated=0,
                                llm_proposed=0, llm_valid=0, new_accepted=0, new_candidate=0,
                                trials_total=0, error=error, **frozen_hashes()), args.root)
    except store.AlreadyRunning as exc:
        print(str(exc), file=sys.stderr)
        return 2
    return int(failed)


if __name__ == '__main__':
    raise SystemExit(main())
