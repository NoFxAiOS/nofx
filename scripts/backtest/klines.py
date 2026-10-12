"""K 线缓存路径与加载。缓存格式: data/backtest_cache/{kl1m,kl5m}/{SYMBOL}_{t0_ms}.json,行 [openTimeMs,o,h,l,c]。"""
import json
import os

MIN_MS = 60_000
ONE_M_LIMIT = 1500
FIVE_M_LIMIT = 1000


def default_cache_dir(repo_root):
    return os.path.join(repo_root, "data", "backtest_cache")


def floor_min(ms):
    return int(ms) // MIN_MS * MIN_MS


def cache_paths(cache, symbol, t0_ms):
    """t0_ms 会被取整到分钟,与参考回测的缓存文件名一致。"""
    t0 = floor_min(t0_ms)
    return (
        os.path.join(cache, "kl1m", f"{symbol}_{t0}.json"),
        os.path.join(cache, "kl5m", f"{symbol}_{t0}.json"),
    )


def _read(path):
    try:
        with open(path) as f:
            return json.load(f)
    except (OSError, ValueError):
        return None


def load_bars(cache, symbol, t0_ms):
    """返回 (bars, n_1m):1m 之后追加 5m 扩展(仅取晚于最后一根 1m 的 5m);无 1m 缓存返回 None。"""
    p1, p5 = cache_paths(cache, symbol, t0_ms)
    b1 = _read(p1)
    if b1 is None:
        return None
    bars = list(b1)
    b5 = _read(p5)
    if b5 and b1:
        last = b1[-1][0]
        bars += [b for b in b5 if b[0] > last]
    return bars, len(b1)
