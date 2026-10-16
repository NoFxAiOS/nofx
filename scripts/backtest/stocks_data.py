"""Offline stock cache and point-in-time signals (no execution/accounting).

Thresholds and inclusive/exclusive windows port cmd/ma2060bt/rules.go.
Daily timestamps are UTC dates; hourly timestamps retain their UTC time.
"""
import bisect
import json
import math
from dataclasses import dataclass
from pathlib import Path

import pandas as pd

NAMES = ("MOM_TOP", "MA_CROSS", "MA2060_RULES", "MOM_X_MA", "SPY_BH", "EW_BH")
DEFAULT_EXCLUDE = "TQQQBUSDT,SQQQBUSDT,SOXLBUSDT,SOXSBUSDT,UVXYBUSDT,MUUBUSDT,KORUBUSDT"
SPY = "SPYBUSDT"


@dataclass(frozen=True)
class Indicator:
    fast: float = math.nan
    slow: float = math.nan
    trend: float = math.nan
    atr: float = math.nan
    new_high: bool = False


def indicators(bars):
    """Forward sums and Wilder's seed include the first bar's high-low."""
    sums, seed, atr, out = [0.0] * 3, 0.0, 0.0, []
    for i, b in enumerate(bars):
        ma = []
        for k, period in enumerate((20, 60, 200)):
            sums[k] += b['close']
            if i >= period:
                sums[k] -= bars[i-period]['close']
            ma.append(sums[k] / period if i+1 >= period else math.nan)
        tr = b['high'] - b['low']
        if i:
            tr = max(tr, abs(b['high']-bars[i-1]['close']), abs(b['low']-bars[i-1]['close']))
        if i < 14:
            seed += tr
            atr = seed / 14
        else:
            atr = (atr*13 + tr) / 14
        high = i >= 20 and all(b['close'] > p['close'] for p in bars[i-20:i])
        out.append(Indicator(*ma, atr if i >= 13 else math.nan, high))
    return out


def golden(v, i):
    return i > 0 and v[i-1].fast <= v[i-1].slow and v[i].fast > v[i].slow


def death(v, i):
    return i > 0 and v[i-1].fast >= v[i-1].slow and v[i].fast < v[i].slow


def ready(v, i):
    return 0 <= i < len(v) and all(math.isfinite(x) for x in (v[i].fast, v[i].slow, v[i].atr))


def bullish_key(bars, i):
    if i < 1:
        return False
    b, p = bars[i], bars[i-1]
    engulf = p['close'] < p['open'] and b['close'] > b['open'] and b['open'] <= p['close'] and b['close'] >= p['open']
    return b['close'] > p['high'] or engulf


def entry_rule(bars, v, i):
    if not ready(v, i):
        return ""
    b, a = bars[i], v[i]
    if golden(v, i):
        return "E1"
    rising = i >= 5 and a.slow > v[i-5].slow
    if a.fast > a.slow and rising:
        if b['low'] <= a.fast*1.005 and b['close'] >= a.slow and b['close'] > b['open']:
            return "E2"
        if b['close'] > a.fast:
            for j in range(max(0, i-2), i+1):
                if bars[j]['close'] < v[j].fast and bars[j]['close'] >= v[j].slow*0.98:
                    return "E3"
    if bullish_key(bars, i):
        for j in range(max(0, i-2), i+1):
            if math.isfinite(v[j].fast) and math.isfinite(v[j].atr) and bars[j]['close']/v[j].fast-1 <= -2.5*v[j].atr/bars[j]['close']:
                return "E4"
    return ""


def no_new_high(v, i, window):
    return i-window+1 >= 20 and not any(a.new_high for a in v[i-window+1:i+1])


def exit_rule(bars, v, i, entry_price, entry_atr, bounce=False):
    if not ready(v, i):
        return ""
    b, a = bars[i], v[i]
    if death(v, i):
        return "X1"
    extended = any(math.isfinite(v[j].fast) and math.isfinite(v[j].atr) and
                   bars[j]['close']/v[j].fast-1 >= 3*v[j].atr/bars[j]['close']
                   for j in range(max(0, i-19), i+1))
    if extended and no_new_high(v, i, 5) and i >= 5 and b['close'] < min(p['low'] for p in bars[i-5:i]):
        return "X2"
    if no_new_high(v, i, 10) and b['close'] < min(a.fast, a.slow):
        return "X3"
    if b['close'] < entry_price-2*entry_atr:
        return "X4"
    if bounce and b['close'] >= a.fast:
        return "TARGET"
    return ""


class History:
    def __init__(self, raw, tf='1d'):
        valid, parsed = {}, []
        for b in raw:
            try:
                key = pd.Timestamp(b['open_time'])
                if pd.isna(key):
                    continue
                key = key.tz_localize('UTC') if key.tzinfo is None else key.tz_convert('UTC')
                key = key.tz_localize(None)
                row = {k: float(b[k]) for k in ('open', 'high', 'low', 'close', 'volume')}
            except (ValueError, TypeError, KeyError):
                continue
            o, h, l, c, vol = (row[k] for k in ('open', 'high', 'low', 'close', 'volume'))
            if not all(math.isfinite(x) for x in row.values()) or min(o, l, c) <= 0 or h < max(o, c) or l > min(o, c) or vol < 0:
                continue
            parsed.append((key, row))
        for key, row in sorted(parsed, key=lambda item: item[0]):
            valid[key.normalize() if tf == '1d' else key] = row  # latest valid duplicate
        self.keys = sorted(valid)
        self.bars = [valid[k] for k in self.keys]
        self.index = {k: i for i, k in enumerate(self.keys)}
        self.ind = indicators(self.bars)
        self.entries = [entry_rule(self.bars, self.ind, i) for i in range(len(self.bars))]
        # Static exits preserve X1..X3 precedence; X4/TARGET need fill metadata.
        self.exits = [exit_rule(self.bars, self.ind, i, -math.inf, 0) for i in range(len(self.bars))]
        self.removed = len(raw)-len(self.keys)
        self.frame = pd.DataFrame(self.bars, index=pd.DatetimeIndex(self.keys),
                                  columns=['open', 'high', 'low', 'close', 'volume'])


def rank_at(symbol, daily, key, tf):
    day = key.normalize()
    i = (bisect.bisect_left(daily.keys, day) if tf == '1h' else bisect.bisect_right(daily.keys, day))-1
    if i < 252:
        return (symbol, False, False, 0.0)
    return (symbol, True, daily.bars[i]['close'] > daily.ind[i].trend,
            daily.bars[i-21]['close']/daily.bars[i-252]['close']-1)


def rank_key(r):
    s, valid, trend, momentum = r
    return (not valid, not trend if valid else False, -momentum if valid else 0, s)


class Study:
    def __init__(self, execution, daily, tf='1d'):
        self.tf, self.histories, self.daily = tf, execution, daily
        spy = execution[SPY]
        self.last_of_day = {k: i+1 == len(spy.keys) or spy.keys[i+1].date() != k.date()
                            for i, k in enumerate(spy.keys)}
        starts = [i for i, k in enumerate(spy.keys) if i >= 60 and self.last_of_day[k] and
                  rank_at(SPY, daily[SPY], k, tf)[1]]
        if not starts or starts[0]+1 >= len(spy.keys):
            raise ValueError('need 253 daily SPY bars, 61 execution bars, and a subsequent fill bar')
        self.calendar = spy.keys[starts[0]:]
        self.calendar_index = {k: i for i, k in enumerate(self.calendar)}
        self.days, day = {}, 0
        for i, k in enumerate(self.calendar):
            if i and k.date() != self.calendar[i-1].date():
                day += 1
            self.days[k] = day
        self.ew = sorted(s for s, h in execution.items() if s != SPY and self.calendar[0] in h.index)
        self.ranks = {k: sorted((rank_at(s, daily[s], k, tf) for s, h in execution.items()
                               if s != SPY and k in h.index), key=rank_key) for k in self.calendar}
        self.coverage = dict(universe_size=len(execution)-1, first_signal_close=str(self.calendar[0]),
                             last_mark_close=str(self.calendar[-1]), evaluation_bars=len(self.calendar),
                             spy_bars_including_warmup=len(spy.keys), symbols={}, skipped={}, warnings={}, sources={},
                             invalid_or_duplicate_bars_removed={s: h.removed for s, h in execution.items()})
        for s, h in execution.items():
            self.coverage['symbols'][s] = dict(execution_bars=len(h.keys), daily_bars=len(daily[s].keys),
                evaluation_bars_on_spy_calendar=sum(k in h.index for k in self.calendar),
                first_bar=str(h.keys[0]), last_bar=str(h.keys[-1]))


def load_study(cache, tf='1d', exclude=DEFAULT_EXCLUDE):
    root = Path(cache)
    drops = {s.strip().upper() for s in exclude.split(',') if s.strip()}
    drops = {s if s.endswith('BUSDT') else s+'BUSDT' for s in drops} | {'QQQBUSDT'}
    execution, daily, skipped, sources, warnings = {}, {}, {}, {}, {}
    folder = root/'1h' if tf == '1h' else root
    symbols = sorted({p.stem for p in folder.glob('*BUSDT.json')} | {p.stem for p in root.glob('*BUSDT.json')})
    if not symbols:
        raise ValueError(f'no cached stock bars in {folder}')
    for s in symbols:
        path = folder/(s+'.json')
        if s in drops and s != SPY:
            continue
        try:
            doc = json.loads(path.read_text())['series']
            if doc['symbol'] != s or doc['timeframe'] != tf:
                raise ValueError('symbol/timeframe mismatch')
            h = History(doc['bars'], tf)
            if not h.keys:
                raise ValueError('no valid execution bars')
            execution[s] = h
            sources[s+'/'+tf] = doc.get('source', 'unknown')
            if tf == '1d':
                daily[s] = h
            else:
                try:
                    d = json.loads((root/path.name).read_text())['series']
                    if d['symbol'] != s or d['timeframe'] != '1d':
                        raise ValueError('daily symbol/timeframe mismatch')
                    daily[s] = History(d['bars'])
                    sources[s+'/1d'] = d.get('source', 'unknown')
                except (OSError, ValueError, KeyError) as exc:
                    if s == SPY:
                        raise ValueError(f'SPY daily cache: {exc}') from exc
                    daily[s] = History([])
                    warnings[s] = 'daily momentum unavailable: '+str(exc)
        except (OSError, ValueError, KeyError) as exc:
            if s == SPY:
                raise ValueError(f'SPY cache: {exc}') from exc
            execution.pop(s, None)
            skipped[s] = str(exc)
    if SPY not in execution or not daily[SPY].keys:
        raise ValueError('SPYBUSDT execution and daily cache are required')
    if len(execution) < 2:
        raise ValueError('no eligible stocks loaded')
    study = Study(execution, daily, tf)
    study.coverage.update(skipped=skipped, warnings=warnings, sources=sources,
                          excluded=sorted(drops | {SPY}), cache=str(root.resolve()),
                          candidate_universe_size=len(set(symbols)-drops-{SPY}))
    return study
