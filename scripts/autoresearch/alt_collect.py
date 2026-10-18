"""Hourly public alternative-data collector. No liquidation series:
allForceOrders has been retired (404). Backfill/update never contact a database.
Rows use availability timestamp in UTC milliseconds; taker buckets conservatively
become available one hour after their source timestamp; premium at closeTime+1.
Snapshot OI/ratios use their published timestamp. FNG uses UTC publication midnight.
"""
import argparse
import datetime as dt
import json
import logging
from logging.handlers import RotatingFileHandler
import math
from pathlib import Path
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

if __package__ in (None, ''):
    sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from scripts.autoresearch.common import ALT, atomic_json, read_json, universe
from scripts.backtest.short_features_study import Fetcher, HOUR, DAY

ENDPOINTS = {
    'oi': '/futures/data/openInterestHist',
    'glsr': '/futures/data/globalLongShortAccountRatio',
    'toplsp': '/futures/data/topLongShortPositionRatio',
    'topls_acct': '/futures/data/topLongShortAccountRatio',
    'taker': '/futures/data/takerlongshortRatio',
    'premium': '/fapi/v1/premiumIndexKlines',
}
START = int(dt.datetime(2025, 10, 1, tzinfo=dt.timezone.utc).timestamp() * 1000)


class HTTPClient(Fetcher):
    """Reuse the study's explicit localhost proxy; injectable clock/sleep for tests."""
    def __init__(self, sleep=time.sleep, clock=time.monotonic):
        super().__init__()
        self.sleep, self.clock = sleep, clock

    def get(self, endpoint, params):
        base = endpoint if endpoint.startswith('https://') else 'https://fapi.binance.com' + endpoint
        url = base + '?' + urllib.parse.urlencode(params)
        error = None
        for attempt in range(6):
            self.sleep(max(0, .65 - (self.clock() - self.last)))
            self.last = self.clock()
            try:
                req = urllib.request.Request(url, headers={'User-Agent': 'nofx-autoresearch/1'})
                with self.opener.open(req, timeout=25) as response:
                    result = json.load(response)
                if not isinstance(result, (list, dict)):
                    raise ValueError('unexpected response')
                return result
            except (OSError, ValueError) as exc:
                error = exc
                if isinstance(exc, urllib.error.HTTPError):
                    if exc.code in (400, 401, 403, 404):
                        break
                    if exc.code in (429, 418):
                        retry = exc.headers.get('Retry-After', '0') if exc.headers else '0'
                        try:
                            retry = float(retry)
                        except ValueError:
                            retry = 0
                        self.sleep(max(retry, min(300, 15 * 2**attempt)))
                        continue
                self.sleep(min(60, 2**(attempt + 1)))
        raise RuntimeError(f'{endpoint}: {error}')


def merge_save(path, old, got):
    old.update((int(r['timestamp']), r) for r in got)
    atomic_json(path, [old[t] for t in sorted(old)])


def normalize(kind, raw, end):
    out = []
    for row in raw:
        if kind == 'premium':
            t = int(row[6]) + 1
            r = dict(timestamp=t, source_timestamp=int(row[0]),
                     **dict(zip(('open', 'high', 'low', 'close'), map(float, row[1:5]))))
        else:
            t0 = int(row['timestamp'])
            t = t0 + (HOUR if kind == 'taker' else 0)
            r = {k: (float(v) if k not in ('symbol', 'timestamp') else v) for k, v in row.items()}
            r.update(timestamp=t, source_timestamp=t0)
        if t <= end and all(not isinstance(v, float) or math.isfinite(v) for v in r.values()):
            out.append(r)
    return out


def collect_symbol(client, out_root, kind, sym, mode, now_ms):
    """Persist each page. Empty windows advance; partial pages don't end backfill."""
    path = Path(out_root) / kind / (sym + '.json')
    rows = {int(r['timestamp']): r for r in read_json(path, [])}
    earliest = START if kind == 'premium' else now_ms - 30 * DAY + HOUR
    # Continue after the last CLOSED/published source bucket; deduplicate by availability time.
    cur = earliest
    if mode == 'update' and rows:
        last = rows[max(rows)]
        cur = max(earliest, int(last.get('source_timestamp', last['timestamp'])) + HOUR)
    limit = 1500 if kind == 'premium' else 500
    while cur < now_ms:
        stop = min(now_ms, cur + limit * HOUR)
        params = dict(symbol=sym, startTime=cur, endTime=stop - 1, limit=limit)
        params['interval' if kind == 'premium' else 'period'] = '1h'
        raw = client.get(ENDPOINTS[kind], params)
        if not isinstance(raw, list):
            raise ValueError(f'{kind}/{sym}: expected array')
        got = normalize(kind, raw, now_ms)
        merge_save(path, rows, got)
        # Use the raw bucket cursor, not availability time; handle capped pages.
        last = max((int(r[0]) if kind == 'premium' else int(r['timestamp']) for r in raw), default=stop - HOUR)
        cur = max(cur + HOUR, min(stop, last + HOUR)) if raw else stop
    return len(rows)


def collect_fng(client, out_root):
    path = Path(out_root) / 'fng.json'
    rows = {int(r['timestamp']): r for r in read_json(path, [])}
    data = client.get('https://api.alternative.me/fng/', {'limit': 0})
    got = [dict(timestamp=int(r['timestamp']) * 1000 // DAY * DAY, value=float(r['value']))
           for r in data['data']]
    merge_save(path, rows, got)
    return len(rows)


def collect(out_root=ALT, mode='update', series=None, symbols=None, client=None, now_ms=None):
    out_root = Path(out_root)
    out_root.mkdir(parents=True, exist_ok=True)
    logger = logging.getLogger('autoresearch.collect.' + str(out_root))
    logger.setLevel(logging.INFO)
    logger.propagate = False
    handler = RotatingFileHandler(out_root / 'collect.log', maxBytes=1024 * 1024, backupCount=3)
    handler.setFormatter(logging.Formatter('%(asctime)s %(levelname)s %(message)s'))
    logger.addHandler(handler)
    try:
        selected = universe(out_root / 'universe_src.json')
        if symbols is not None:
            unknown = set(symbols) - set(selected)
            if unknown:
                raise ValueError(f'symbols outside eligible universe: {sorted(unknown)}')
            selected = [s for s in selected if s in symbols]
        client = HTTPClient() if client is None else client
        now_ms = int(time.time() * 1000) if now_ms is None else now_ms
        status = {}
        for kind in series or [*ENDPOINTS, 'fng']:
            for sym in (['GLOBAL'] if kind == 'fng' else selected):
                key = kind + '/' + sym
                try:
                    n = collect_fng(client, out_root) if kind == 'fng' else collect_symbol(client, out_root, kind, sym, mode, now_ms)
                    status[key] = dict(ok=True, rows=n)
                    logger.info('%s rows=%s', key, n)
                except Exception as exc:  # one failed coin/endpoint must not stop the run
                    status[key] = dict(ok=False, error=str(exc))
                    logger.exception('%s failed', key)
        return status
    finally:
        logger.removeHandler(handler)
        handler.close()


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    modes = ap.add_mutually_exclusive_group(required=True)
    modes.add_argument('--backfill', action='store_true')
    modes.add_argument('--update', action='store_true')
    ap.add_argument('--series', nargs='+', choices=[*ENDPOINTS, 'fng'])
    ap.add_argument('--symbols', nargs='+')
    ap.add_argument('--out-root', type=Path, default=ALT)
    args = ap.parse_args()
    print(json.dumps(collect(args.out_root, 'backfill' if args.backfill else 'update', args.series, args.symbols), ensure_ascii=False))


if __name__ == '__main__':
    main()
