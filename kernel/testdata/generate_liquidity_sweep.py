"""Freeze the reference fixture; Go tests never execute Python.

From the repository root:
PYTHONDONTWRITEBYTECODE=1 /Users/zhangyun/workspace/nofx/.venv/bin/python kernel/testdata/generate_liquidity_sweep.py

Events come directly from ict.swings / ict._sweeps. The adapter recovers the
selected level (not returned by _sweeps), applies the requested reporting
window, and copies find_setups' MSS lookup through the current closed bar.
"""

import hashlib
import importlib.util
import json
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "scripts/backtest/ict.py"
spec = importlib.util.spec_from_file_location("ict", SOURCE)
ict = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = ict
spec.loader.exec_module(ict)

EPOCH = 1_792_022_400_000  # 2026-10-15 00:00 UTC, aligned to 15m and 1h


def candles(n):
    rows = [[100, 110, 99, 100] for _ in range(n)]
    rows[3][2] = 95
    rows[7][1] = 115
    rows[11][2] = 94
    if n > 12:
        rows[12][1], rows[12][3] = 117, 116
    return rows


def mirror(rows):
    return [[200-o, 200-l, 200-h, 200-c] for o, h, l, c in rows]


def expand_5m(rows, minutes):
    """Distinct 5m OHLC rows whose complete UTC buckets reproduce rows."""
    count = minutes // 5
    out = []
    for i, (o, h, l, c) in enumerate(rows):
        for j in range(count):
            start = o + (c-o)*j/count
            end = o + (c-o)*(j+1)/count
            high = h if j == 0 else max(start, end)
            low = l if j == 1 else min(start, end)
            out.append([EPOCH + (i*count+j)*ict.M5, start, high, low, end])
    return out


def expected(a, tf):
    shoulder, lookback, recent = (2, 40, 8) if tf == "15m" else (3, 48, 4)
    points = ict.swings(a["h"], a["l"], shoulder)
    book = ict._books(points)
    result = dict(sellside_swept=None, buyside_swept=None)
    for s, side, extreme in ict._sweeps(a, points, lookback):
        if len(a["c"])-1-s >= recent:
            continue
        sign = 1 if side == "long" else -1
        key = "l" if side == "long" else "h"
        # A pivot is active from avail+1, and every earlier piercing consumes
        # it. This recovers _sweeps' selected ref without modifying ict.py.
        eligible = [p for p in points
                    if p.kind == ("low" if side == "long" else "high")
                    and p.avail < s and s-p.idx <= lookback
                    and not (sign*(a[key][p.avail+1:s]-p.price) < 0).any()]
        ref = eligible[-1].price
        assert sign*(extreme-ref) < 0 < sign*(a["c"][s]-ref)
        indices, prices = ict._known(book, "high" if sign == 1 else "low",
                                     s, max(0, s-lookback))
        prices = prices[indices < s]
        mss = bool(len(prices) and (sign*(a["c"][s+1:]-prices[-1]) > 0).any())
        event = dict(tf=tf, bars_ago=len(a["c"])-1-s, level=ref,
                     extreme=extreme, depth_pct=abs(extreme-ref)/ref*100,
                     mss_confirmed=mss, reclaimed=True)
        result["sellside_swept" if side == "long" else "buyside_swept"] = event
    return result if any(result.values()) else None


def case(name, rows, tf="15m"):
    minutes = 15 if tf == "15m" else 60
    bars5 = expand_5m(rows, minutes)
    a = ict.resample(bars5, minutes)
    assert len(a["c"]) == len(rows)
    assert [[float(a[k][i]) for k in ("o", "h", "l", "c")]
            for i in range(len(rows))] == rows
    snapshots = [dict(closed_bars=n,
                      expected=expected({k: v[:n] for k, v in a.items()}, tf))
                 for n in range(1, len(rows)+1)]
    return dict(name=name, tf=tf, bars_5m=bars5, snapshots=snapshots)


def main():
    long = candles(21)
    long[15][2] = 93  # latest new pivot is swept, replacing the first event
    long[16][1], long[16][3] = 119, 118

    consumed = candles(24)
    consumed[11][3] = 94.5
    consumed[12] = [100, 110, 94, 100]  # equal low prevents a new pivot
    consumed[22][2] = 94.5  # old 95 was consumed outside the recent window

    older = candles(14)
    older[3][2], older[7][2] = 90, 95
    older[11][2], older[11][3] = 89, 94
    older[12] = [100, 110, 99, 100]
    older[13][2], older[13][3] = 89, 94

    at_close = candles(14)
    at_close[7][1], at_close[9][1] = 110, 115

    fixture = dict(
        reference="scripts/backtest/ict.py: resample, swings, _sweeps, find_setups MSS",
        reference_sha256=hashlib.sha256(SOURCE.read_bytes()).hexdigest(),
        generator="kernel/testdata/generate_liquidity_sweep.py",
        semantics="MSS uses _known at sweep close within lookback; reporting windows replace find_setups.window",
        cases=[case("15m_long_latest_and_expiry", long),
               case("15m_short_mirror", mirror(long)),
               case("15m_consumed_before_reporting_window", consumed),
               case("15m_non_selected_levels_consumed", older),
               case("15m_mss_pivot_confirmed_at_sweep_close", at_close),
               case("1h_long", candles(17), "1h"),
               case("1h_short", mirror(candles(17)), "1h")],
    )
    target = Path(__file__).with_name("liquidity_sweep_ict.json")
    # Keep each 5m OHLC row on one line so the frozen fixture remains small.
    text = json.dumps(fixture, indent=2, ensure_ascii=False, allow_nan=False)
    import re
    text = re.sub(r"\[\n\s+(-?\d[^\[\]{}]*?)\n\s+\]",
                  lambda m: "[" + " ".join(m[1].split()) + "]", text)
    target.write_text(text + "\n")
    print(f"Wrote {target.relative_to(ROOT)}: {len(fixture['cases'])} cases, "
          f"{sum(len(c['snapshots']) for c in fixture['cases'])} closed-bar snapshots")


if __name__ == "__main__":
    main()
