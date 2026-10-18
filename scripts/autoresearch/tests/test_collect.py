import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import urllib.error
from scripts.autoresearch import alt_collect as ac


class MockHTTP:
    def __init__(self):
        self.calls = []

    def get(self, endpoint, params):
        self.calls.append((endpoint, dict(params)))
        if 'alternative.me' in endpoint:
            return {'data': [{'timestamp': '86400', 'value': '42'}, {'timestamp': '0', 'value': '30'}]}
        start, end = params['startTime'], params['endTime']
        # Force a short page so advancing from raw timestamp is exercised.
        ts = list(range(start, end + 1, ac.HOUR))[:170]
        if 'premiumIndex' in endpoint:
            return [[t, '.01', '.02', '-.01', '.005', '0', t + ac.HOUR - 1] for t in ts]
        return [dict(timestamp=t, sumOpenInterest='10', sumOpenInterestValue='100') for t in ts]


class CollectTests(unittest.TestCase):
    def test_pagination_dedup_incremental(self):
        with tempfile.TemporaryDirectory() as tmp:
            http = MockHTTP()
            end = 40 * ac.DAY
            n = ac.collect_symbol(http, tmp, 'oi', 'TEST', 'backfill', end)
            self.assertGreater(len(http.calls), 2)
            self.assertTrue(all(p['limit'] <= 500 and p['endTime'] - p['startTime'] < 500 * ac.HOUR for _, p in http.calls))
            ac.collect_symbol(http, tmp, 'oi', 'TEST', 'backfill', end)
            rows = json.loads((Path(tmp) / 'oi/TEST.json').read_text())
            self.assertEqual(len(rows), n)
            ts = [r['timestamp'] for r in rows]
            self.assertEqual(ts, sorted(set(ts)))
            http.calls.clear()
            ac.collect_symbol(http, tmp, 'oi', 'TEST', 'update', end + 2 * ac.HOUR)
            self.assertEqual(http.calls[0][1]['startTime'], ts[-1] + ac.HOUR)
            self.assertEqual(len(json.loads((Path(tmp) / 'oi/TEST.json').read_text())), n + 2)

    def test_premium_history_and_no_open_bucket(self):
        with tempfile.TemporaryDirectory() as tmp:
            http = MockHTTP()
            end = ac.START + 1502 * ac.HOUR + 123
            ac.collect_symbol(http, tmp, 'premium', 'TEST', 'backfill', end)
            self.assertEqual(http.calls[0][1]['startTime'], ac.START)
            self.assertEqual(http.calls[0][1]['limit'], 1500)
            rows = json.loads((Path(tmp) / 'premium/TEST.json').read_text())
            self.assertEqual(len(rows), 1502)
            self.assertLessEqual(rows[-1]['timestamp'], end)

    def test_fng_and_failure_isolation_exclusions(self):
        with tempfile.TemporaryDirectory() as tmp:
            ac.atomic_json(Path(tmp) / 'universe_src.json', {'crypto': ['BAD', 'GOOD', 'MANAUSDT']})
            http = MockHTTP()
            original = http.get
            def fail(endpoint, params):
                if params.get('symbol') == 'BAD':
                    raise RuntimeError('mock failure')
                return original(endpoint, params)
            http.get = fail
            status = ac.collect(tmp, 'backfill', ['oi', 'fng'], client=http, now_ms=40 * ac.DAY)
            self.assertFalse(status['oi/BAD']['ok'])
            self.assertTrue(status['oi/GOOD']['ok'])
            self.assertNotIn('oi/MANAUSDT', status)
            self.assertEqual(json.loads((Path(tmp) / 'fng.json').read_text())[0]['value'], 30)
            self.assertTrue((Path(tmp) / 'collect.log').exists())

    def test_taker_waits_for_closed_bucket_and_partial_failure_preserves_pages(self):
        raw = [{'timestamp': 0, 'buySellRatio': '1.2', 'buyVol': '12', 'sellVol': '10'}]
        self.assertEqual(ac.normalize('taker', raw, ac.HOUR - 1), [])
        self.assertEqual(ac.normalize('taker', raw, ac.HOUR)[0]['timestamp'], ac.HOUR)
        class Partial(MockHTTP):
            def get(self, endpoint, params):
                if self.calls:
                    raise RuntimeError('page two failed')
                return super().get(endpoint, params)
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(RuntimeError):
                ac.collect_symbol(Partial(), tmp, 'oi', 'TEST', 'backfill', 40 * ac.DAY)
            saved = json.loads((Path(tmp) / 'oi/TEST.json').read_text())
            self.assertEqual(len(saved), 170)

    def test_pacing_proxy_and_429_418_retry(self):
        sleeps, clock = [], [0.]
        def sleep(seconds):
            sleeps.append(seconds)
            clock[0] += seconds
        c = ac.HTTPClient(sleep=sleep, clock=lambda: clock[0])
        starts = []
        def open_mock(*args, **kwargs):
            starts.append(clock[0])
            if len(starts) < 3:
                raise urllib.error.HTTPError('mock', 429 if len(starts) == 1 else 418, 'rate', {'Retry-After': '20'}, None)
            return io.StringIO('[]')
        with patch.object(c.opener, 'open', side_effect=open_mock):
            c.get('/mock', {})
            c.get('/mock', {})
        self.assertTrue(all(b - a >= .6 for a, b in zip(starts, starts[1:])))
        self.assertIn(20., sleeps)
        proxy = next(h for h in c.opener.handlers if isinstance(h, ac.urllib.request.ProxyHandler))
        self.assertEqual(proxy.proxies['https'], 'http://127.0.0.1:7890')


if __name__ == '__main__':
    unittest.main()
