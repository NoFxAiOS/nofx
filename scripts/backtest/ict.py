"""Causal ICT detectors for UTC-aligned [open_ms, open, high, low, close] bars.

All decisions use closed bars. Market entries describe the next open; limit
orders describe a future order, without inspecting fills. ``Ctx.a5`` and
``Ctx.a1h`` contain the array dictionaries returned by ``load_arrays``.

Conventions where the model leaves a choice:
* A sweep uses the latest unconsumed pivot; every pierced pivot is consumed,
  including older pivots and penetrations that do not reclaim the level.
* Displacement uses nonzero bodies among the preceding 20 bars (at least one).
* SMT needs 80 prior bars; the reference's 80-bar baseline precedes BOTH of
  its test bars. Missing timestamp matches disable SMT.
* HTF dealing range is the last 24 complete hourly bars. "Latest" pivots and
  FVGs mean latest in time; ERL and P2's final choice mean nearest in price.
* A directional HTF FVG survives until a close crosses its far boundary;
  an opposing P2 FVG is fully filled by a wick reaching its far boundary.
* Previous-day levels require a complete UTC day. The day and FVG age are
  measured at decision close. Path flags require a valid directional target.
"""

from dataclasses import dataclass
from datetime import datetime, timezone
from zoneinfo import ZoneInfo

import numpy as np


M5 = 300_000
HOUR = 3_600_000
DAY = 24 * HOUR
NY = ZoneInfo("America/New_York")
DEFAULTS = dict(swing_w5=2, swing_w1h=3, sweep_lookback=80, window=12,
                body_mult=1.5, atr_mult=1.0, buffer_atr=0.1, lim_valid=24,
                dedupe_min=60)


@dataclass(frozen=True)
class Swing:
    idx: int
    kind: str
    price: float
    avail: int


@dataclass(frozen=True)
class FVG:
    start: int
    mid: int
    end: int
    direction: str
    lo: float
    hi: float
    ce: float

    @property
    def avail(self):
        return self.end


@dataclass
class Setup:
    sym: str
    side: str
    trigger: str
    s: int
    idx: int
    kind: str
    ref: float
    sl: float
    extreme: float
    tp_erl: float | None
    tp_p1: float | None
    tp_p2: float | None
    tp_p3: float | None
    flags: dict
    ny_hour: int
    atr5: float
    atr1h_pct: float | None
    t_close_ms: int
    erl_level: float | None = None
    valid_bars: int = 0


def load_arrays(bars) -> dict[str, np.ndarray]:
    """Return independent numeric arrays; reject unsorted/duplicate timestamps."""
    rows = np.asarray(bars, dtype=np.float64)
    if rows.size == 0:
        rows = np.empty((0, 5), dtype=np.float64)
    if rows.ndim != 2 or rows.shape[1] != 5 or not np.isfinite(rows).all():
        raise ValueError("bars must be finite [open_ms, o, h, l, c] rows")
    t = rows[:, 0].astype(np.int64)
    if np.any(t != rows[:, 0]) or np.any(np.diff(t) <= 0) or np.any(t % M5):
        raise ValueError("timestamps must be unique, increasing, and 5m aligned")
    out = {key: rows[:, i].copy() for i, key in enumerate(("o", "h", "l", "c"), 1)}
    out.update(t_open=t, t_close=t + M5)
    return out


def _resample_arrays(a, minutes):
    if not isinstance(minutes, (int, np.integer)) or minutes < 5 or minutes % 5:
        raise ValueError("minutes must be a positive multiple of 5")
    count, duration = minutes // 5, minutes * 60_000
    t = a["t_open"]
    if not len(t):
        return load_arrays([])
    bucket = t // duration * duration
    starts = np.r_[0, np.flatnonzero(np.diff(bucket)) + 1]
    ends = np.r_[starts[1:], len(t)]
    # Unique 5m-aligned timestamps + exact count and endpoints guarantee no holes.
    keep = ((ends - starts == count) & (t[starts] == bucket[starts])
            & (t[ends - 1] == bucket[starts] + duration - M5))
    out = dict(t_open=bucket[starts][keep].copy(),
               t_close=(bucket[starts][keep] + duration).copy(),
               o=a["o"][starts][keep].copy(), c=a["c"][ends - 1][keep].copy(),
               h=np.maximum.reduceat(a["h"], starts)[keep],
               l=np.minimum.reduceat(a["l"], starts)[keep])
    return out


def resample(bars5, minutes) -> dict[str, np.ndarray]:
    """Aggregate UTC buckets, discarding incomplete or gapped buckets."""
    return _resample_arrays(load_arrays(bars5), minutes)


def swings(h, l, w) -> list[Swing]:
    """Strict two-sided pivots, usable only after bar ``idx + w`` closes."""
    if not isinstance(w, (int, np.integer)) or w < 1:
        raise ValueError("w must be a positive integer")
    h, l = np.asarray(h, dtype=float), np.asarray(l, dtype=float)
    n = len(h)
    if len(l) != n:
        raise ValueError("high and low lengths differ")
    if n < 2 * w + 1:
        return []
    high = np.ones(n - 2 * w, dtype=bool)
    low = high.copy()
    for d in range(1, w + 1):
        high &= (h[w:n - w] > h[w - d:n - w - d]) & (h[w:n - w] > h[w + d:n - w + d])
        low &= (l[w:n - w] < l[w - d:n - w - d]) & (l[w:n - w] < l[w + d:n - w + d])
    out = []
    for offset in np.flatnonzero(high | low):
        i = int(offset + w)
        if high[offset]:
            out.append(Swing(i, "high", float(h[i]), i + w))
        if low[offset]:
            out.append(Swing(i, "low", float(l[i]), i + w))
    return out


def fvgs(o, h, l, c) -> list[FVG]:
    """Strict three-bar wick gaps; middle-bar displacement is a separate gate."""
    h, l = np.asarray(h, dtype=float), np.asarray(l, dtype=float)
    if not (len(o) == len(h) == len(l) == len(c)):
        raise ValueError("OHLC lengths differ")
    out = []
    for end in range(2, len(h)):
        if h[end - 2] < l[end]:
            lo, hi, direction = h[end - 2], l[end], "bull"
        elif l[end - 2] > h[end]:
            lo, hi, direction = h[end], l[end - 2], "bear"
        else:
            continue
        out.append(FVG(end - 2, end - 1, end, direction,
                       float(lo), float(hi), float((lo + hi) / 2)))
    return out


def atr(h, l, c, period=14) -> np.ndarray:
    """Simple rolling mean TR, using high-low for the first bar's TR."""
    if not isinstance(period, (int, np.integer)) or period < 1:
        raise ValueError("period must be a positive integer")
    h, l, c = (np.asarray(x, dtype=float) for x in (h, l, c))
    if not (len(h) == len(l) == len(c)):
        raise ValueError("HLC lengths differ")
    if not len(c):
        return np.empty(0, dtype=float)
    prev = np.r_[c[0], c[:-1]]
    tr = np.maximum(h - l, np.maximum(np.abs(h - prev), np.abs(l - prev)))
    total = np.r_[0.0, np.cumsum(tr)]
    end = np.arange(1, len(c) + 1)
    start = np.maximum(0, end - period)
    return (total[end] - total[start]) / (end - start)


class Ctx:
    """Precompute one instrument; reference prices are aligned by open timestamp."""

    def __init__(self, bars5, ref_bars5=None):
        self.a5 = load_arrays(bars5)
        self.atr5 = atr(self.a5["h"], self.a5["l"], self.a5["c"])
        self.swings5 = swings(self.a5["h"], self.a5["l"], 2)
        self.fvgs5 = fvgs(*(self.a5[k] for k in ("o", "h", "l", "c")))
        self.a1h = _resample_arrays(self.a5, 60)
        self.t_close1h = self.a1h["t_close"]
        self.atr1h = atr(self.a1h["h"], self.a1h["l"], self.a1h["c"])
        self.swings1h = swings(self.a1h["h"], self.a1h["l"], 3)
        self.fvgs1h = fvgs(*(self.a1h[k] for k in ("o", "h", "l", "c")))
        daily = _resample_arrays(self.a5, 1440)
        self.prev_days = {int(t): (float(lo), float(hi)) for t, lo, hi in
                          zip(daily["t_close"], daily["l"], daily["h"])}
        self.ref5 = None
        if ref_bars5 is not None:
            ref = load_arrays(ref_bars5)
            self.ref5 = {k: np.full(len(self.a5["c"]), np.nan) for k in ("l", "h")}
            positions = np.searchsorted(ref["t_open"], self.a5["t_open"])
            valid = np.flatnonzero(positions < len(ref["t_open"]))
            valid = valid[ref["t_open"][positions[valid]] == self.a5["t_open"][valid]]
            for key in self.ref5:
                self.ref5[key][valid] = ref[key][positions[valid]]


def _books(points):
    return {kind: tuple(np.asarray([getattr(p, attr) for p in points if p.kind == kind],
                                  dtype=float if attr == "price" else np.int64)
                        for attr in ("idx", "price", "avail")) for kind in ("low", "high")}


def _known(book, kind, idx, min_idx=0):
    indices, prices, available = book[kind]
    end = np.searchsorted(available, idx, side="right")
    start = np.searchsorted(indices, min_idx)
    return indices[start:end], prices[start:end]


def _sweeps(a, points, lookback):
    """Yield (index, side, extreme); consume even non-selected pierced levels."""
    active = {"low": [], "high": []}
    next_point = 0
    for s in range(len(a["c"])):
        while next_point < len(points) and points[next_point].avail <= s - 1:
            p = points[next_point]
            active[p.kind].append(p)
            next_point += 1
        for kind, side in (("low", "long"), ("high", "short")):
            levels = [p for p in active[kind] if s - p.idx <= lookback]
            extreme = a["l" if side == "long" else "h"][s]
            sign = 1 if side == "long" else -1
            if levels:
                ref = levels[-1]
                if sign * (extreme - ref.price) < 0 < sign * (a["c"][s] - ref.price):
                    yield s, side, float(extreme)
            active[kind] = [p for p in levels if sign * (extreme - p.price) >= 0]


def _smt(ctx, s, sign):
    if ctx.ref5 is None or s < 81:
        return False
    key = "l" if sign == 1 else "h"
    own = sign * ctx.a5[key][s - 80:s + 1]
    segment = sign * ctx.ref5[key][s - 81:s + 1]
    return bool(np.isfinite(segment).all() and own[-1] < np.min(own[:-1])
                and np.min(segment[-2:]) >= np.min(segment[:-2]))


def _erl_sweep(ctx, book1h, s, sign):
    a, now = ctx.a5, int(ctx.a5["t_close"][s])
    k = int(np.searchsorted(ctx.t_close1h, now, side="right") - 1)
    _, levels = _known(book1h, "low" if sign == 1 else "high", k)
    choices = list(levels[-4:][::-1])
    day = ctx.prev_days.get(now // DAY * DAY)
    if day is not None:
        choices.append(day[0 if sign == 1 else 1])
    extreme = a["l" if sign == 1 else "h"][s]
    for level in choices:
        if sign * (extreme - level) < 0 < sign * (a["c"][s] - level):
            return float(level)
    return None


def _targets(ctx, book5, book1h, idx, ref, sign, erl_swept):
    """All slices stop at the decision's latest complete hourly bar."""
    now = int(ctx.a5["t_close"][idx])
    k = int(np.searchsorted(ctx.t_close1h, now, side="right") - 1)
    a = ctx.a1h
    own_kind, target_kind = (("low", "high") if sign == 1 else ("high", "low"))
    _, bias_points = _known(book1h, own_kind, k)
    bias = bool(len(bias_points) >= 2 and sign * (bias_points[-1] - bias_points[-2]) > 0)
    target_indices, levels1h = _known(book1h, target_kind, k)
    beyond = sign * (levels1h - ref) > 0
    levels1h, target_indices = levels1h[beyond], target_indices[beyond]
    _, levels5 = _known(book5, target_kind, idx, max(0, idx - 287))
    erl = [float(p) for p in levels5 if sign * (p - ref) > 0] + list(levels1h)
    day = ctx.prev_days.get(now // DAY * DAY)
    if day is not None and sign * (day[1 if sign == 1 else 0] - ref) > 0:
        erl.append(day[1 if sign == 1 else 0])
    nearest = lambda prices: float(min(prices, key=lambda p: sign * (p - ref))) if len(prices) else None
    tp_erl = nearest(erl)
    in_fvg = False
    opposing = None
    own_direction = "bull" if sign == 1 else "bear"
    for gap in reversed(ctx.fvgs1h):
        if gap.end > k:
            continue
        if now - int(a["t_close"][gap.end]) > 72 * HOUR:
            break
        if gap.direction == own_direction:
            boundary = gap.lo if sign == 1 else gap.hi
            survived = not np.any(sign * (a["c"][gap.end + 1:k + 1] - boundary) < 0)
            in_fvg |= bool(gap.lo <= ref <= gap.hi and survived)
        elif opposing is None:
            near = gap.lo if sign == 1 else gap.hi
            far = gap.hi if sign == 1 else gap.lo
            wick = a["h" if sign == 1 else "l"][gap.end + 1:k + 1]
            if sign * (near - ref) > 0 and not np.any(sign * (wick - far) >= 0):
                opposing = near
    p1 = float(levels1h[-1]) if in_fvg and bias and len(levels1h) else None
    internal = [] if opposing is None else [opposing]
    if k >= 0:
        start = max(0, k - 23)
        eq = (np.min(a["l"][start:k + 1]) + np.max(a["h"][start:k + 1])) / 2
        if sign * (eq - ref) > 0:
            internal.append(eq)
    p2 = nearest(internal) if erl_swept else None
    outer = levels1h[target_indices >= max(0, k - 23)]
    p3 = float(max(outer, key=lambda p: sign * p)) if erl_swept and len(outer) else None
    return tp_erl, p1, p2, p3, bias, bool(in_fvg), k


def find_setups(ctx: Ctx, params: dict | None = None) -> list[Setup]:
    """Return time-ordered Setups without mutating inputs, context, or parameters.

    ``sym`` is initially empty for callers to fill. Limit ``valid_bars`` counts
    bars AFTER the decision; market orders have zero. Missing HTF ATR is None.
    Custom swing widths rebuild local pivots while leaving Ctx defaults intact.
    """
    p = dict(DEFAULTS, **(params or {}))
    for key in ("swing_w5", "swing_w1h", "sweep_lookback", "window", "lim_valid"):
        if not isinstance(p[key], (int, np.integer)) or p[key] < 1:
            raise ValueError(f"{key} must be a positive integer")
    for key in ("body_mult", "atr_mult", "buffer_atr", "dedupe_min"):
        if not np.isfinite(p[key]) or p[key] < 0:
            raise ValueError(f"{key} must be finite and nonnegative")
    a, hourly = ctx.a5, ctx.a1h
    points5 = ctx.swings5 if p["swing_w5"] == 2 else swings(a["h"], a["l"], p["swing_w5"])
    points1h = (ctx.swings1h if p["swing_w1h"] == 3 else
                swings(hourly["h"], hourly["l"], p["swing_w1h"]))
    book5, book1h = _books(points5), _books(points1h)
    body = np.abs(a["c"] - a["o"])
    displacement = {"long": [], "short": []}
    for gap in ctx.fvgs5:
        m = gap.mid
        previous = body[max(0, m - 20):m]
        previous = previous[previous > 0]
        sign = 1 if gap.direction == "bull" else -1
        if (len(previous) and sign * (a["c"][m] - a["o"][m]) > 0
                and body[m] >= p["body_mult"] * np.median(previous)
                and a["h"][m] - a["l"][m] >= p["atr_mult"] * ctx.atr5[m]):
            displacement["long" if sign == 1 else "short"].append(gap)
    disp_ends = {side: [g.end for g in gaps] for side, gaps in displacement.items()}
    candidates = []
    for s, side, extreme in _sweeps(a, points5, p["sweep_lookback"]):
        sign = 1 if side == "long" else -1
        end = min(len(a["c"]) - 1, s + p["window"])
        indices, prices = _known(book5, "high" if sign == 1 else "low", s,
                                  max(0, s - p["sweep_lookback"]))
        prices = prices[indices < s]
        mss = None
        if len(prices):
            hits = np.flatnonzero(sign * (a["c"][s + 1:end + 1] - prices[-1]) > 0)
            if len(hits):
                mss = int(s + 1 + hits[0])
        delivery_end = s if sign * (a["c"][s] - a["o"][s]) < 0 else s - 1
        start = delivery_end
        while start >= 0 and sign * (a["c"][start] - a["o"][start]) < 0:
            start -= 1
        start += 1
        cisd = None
        if delivery_end - start + 1 >= 2:
            hits = np.flatnonzero(sign * (a["c"][s:end + 1] - a["o"][start]) > 0)
            if len(hits):
                cisd = int(s + hits[0])
        gaps = displacement[side]
        pos = int(np.searchsorted(disp_ends[side], s + 1))
        gap = gaps[pos] if pos < len(gaps) and gaps[pos].end <= end else None
        triggers = [("sweep", s, None)]
        if cisd is not None:
            triggers.append(("cisd", cisd, None))
        if mss is not None:
            triggers.append(("mss", mss, None))
        if gap is not None:
            triggers.append(("fvg", gap.end, gap))
            if mss is not None:
                triggers.append(("full", max(mss, gap.end), gap))
        erl_level = _erl_sweep(ctx, book1h, s, sign)
        smt = _smt(ctx, s, sign)
        for trigger, idx, entry_gap in triggers:
            low = float(np.min(a["l"][s:idx + 1]))
            high = float(np.max(a["h"][s:idx + 1]))
            ref = float(entry_gap.ce if entry_gap is not None else a["c"][idx])
            if trigger == "full" and sign * (ref - (low + high) / 2) > 0:
                continue
            sl = (low if sign == 1 else high) - sign * p["buffer_atr"] * ctx.atr5[idx]
            risk = sign * (ref - sl)
            if not np.isfinite(ref) or not np.isfinite(sl) or risk <= 0:
                continue
            tp_erl, p1, p2, p3, bias, in_fvg, k = _targets(
                ctx, book5, book1h, idx, ref, sign, erl_level is not None)
            if trigger == "full" and (tp_erl is None or sign * (tp_erl - ref) / risk < 2):
                continue
            now = int(a["t_close"][idx])
            atr_pct = float(ctx.atr1h[k] / hourly["c"][k] * 100) if k >= 0 and hourly["c"][k] != 0 else None
            candidates.append(Setup(
                sym="", side=side, trigger=trigger, s=s, idx=idx,
                kind="lim" if entry_gap is not None else "mkt", ref=ref,
                sl=float(sl), extreme=extreme, tp_erl=tp_erl, tp_p1=p1, tp_p2=p2, tp_p3=p3,
                flags=dict(erl_swept=erl_level is not None, smt=smt, htf_bias=bias,
                           in_htf_fvg=in_fvg, p1=p1 is not None, p2=p2 is not None, p3=p3 is not None),
                ny_hour=datetime.fromtimestamp(now / 1000, timezone.utc).astimezone(NY).hour,
                atr5=float(ctx.atr5[idx]), atr1h_pct=atr_pct, t_close_ms=now,
                erl_level=erl_level, valid_bars=p["lim_valid"] if entry_gap is not None else 0))
    order = {name: i for i, name in enumerate(("sweep", "cisd", "mss", "fvg", "full"))}
    candidates.sort(key=lambda setup: (setup.idx, setup.s, order[setup.trigger], setup.side))
    out, last = [], {}
    for setup in candidates:
        key = setup.side, setup.trigger
        if key not in last or setup.t_close_ms - last[key] >= p["dedupe_min"] * 60_000:
            out.append(setup)
            last[key] = setup.t_close_ms
    return out
