"""ICT 研究用 5m K 线缓存: <cache>/ict5m/{SYMBOL}.json,行 [openMs,o,h,l,c]。urllib 自动遵循 HTTPS_PROXY。"""
import json
import os
import time
import urllib.error
import urllib.request

URL = "https://fapi.binance.com/fapi/v1/klines"
M5 = 300_000


def path(cache, symbol):
    return os.path.join(cache, "ict5m", f"{symbol}.json")


def load(cache, symbol):
    try:
        with open(path(cache, symbol)) as f:
            return json.load(f)
    except (OSError, ValueError):
        return []


def _get(symbol, start, end, retries=3):
    u = f"{URL}?symbol={symbol}&interval=5m&startTime={start}&endTime={end}&limit=1500"
    for _ in range(retries):
        try:
            with urllib.request.urlopen(u, timeout=20) as r:
                data = json.load(r)
            time.sleep(0.6)
            return [[k[0], float(k[1]), float(k[2]), float(k[3]), float(k[4])] for k in data]
        except urllib.error.HTTPError as e:
            if e.code == 400:
                return None
        except Exception:  # noqa: BLE001
            pass
        time.sleep(4)
    return None


def fetch(cache, symbol, start_ms, end_ms, log=print):
    """补齐 [start_ms, end_ms] 的 5m 数据并合并落盘,返回根数。"""
    rows = {r[0]: r for r in load(cache, symbol)}
    cur = start_ms // M5 * M5
    while cur < end_ms:
        have_to = cur + 1500 * M5
        if all(t in rows for t in range(cur, min(have_to, end_ms), M5 * 50)):  # 抽样判断已缓存
            cur = have_to
            continue
        got = _get(symbol, cur, min(have_to, end_ms))
        if got is None:
            log(f"  fail {symbol} @ {cur}")
            break
        for r in got:
            rows[r[0]] = r
        cur = got[-1][0] + M5 if got else have_to
    out = [rows[k] for k in sorted(rows)]
    os.makedirs(os.path.dirname(path(cache, symbol)), exist_ok=True)
    tmp = path(cache, symbol) + ".tmp"
    with open(tmp, "w") as f:
        json.dump(out, f)
    os.replace(tmp, path(cache, symbol))
    return len(out)
