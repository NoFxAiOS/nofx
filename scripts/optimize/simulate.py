"""Replay recorded AI decisions under alternative risk/throttle parameters.

Mirrors the live semantics of trader/auto_trader_throttle.go:
  - throttle thresholds compare PRICE-basis PnL% (leverage-independent),
    matching the 2026-07-24 fix that converted the live thresholds from
    margin basis to price basis
  - stop-loss / take-profit are exchange trigger orders -> intra-candle fills
  - opens are gated by confidence, per-cycle/per-hour caps, re-entry cooldown,
    max positions and available margin

Known limitation: decisions are replayed as recorded. Parameters that would have
changed WHAT the AI decided (prompt wording, candidate pool) are out of scope;
parameters that gate/size/exit those decisions are faithfully simulated.
"""

import bisect
import csv
import os
from dataclasses import dataclass, field
from datetime import datetime, timedelta

HERE = os.path.dirname(os.path.abspath(__file__))
DATA = os.path.join(HERE, "data")

FEE_RATE = 0.000784  # measured from trader_fills: 7.84 bps per side
MIN_POSITION_USD = 12.0
LIQUIDATION_MARGIN_PNL = -0.90  # liquidate when margin PnL <= -90%
DATA_GAP_FORCE_CLOSE = timedelta(hours=48)


@dataclass(frozen=True)
class Params:
    min_confidence: float = 78.0
    min_hold_h: float = 4.0
    noise_hold_extra_h: float = 4.0   # noise window = min_hold + extra
    reentry_h: float = 3.0
    max_opens_per_hour: int = 3
    max_opens_per_cycle: int = 2
    max_positions: int = 2
    leverage: float = 10.0
    ratio: float = 5.0                # per-position notional = ratio x equity
    sl_bypass: float = -5.0           # price-PnL% allowing early AI close
    tp_bypass: float = 12.0
    noise_floor: float = -4.0         # price-PnL% band blocking flat closes
    noise_ceiling: float = 6.0
    sl_mult: float = 1.0              # scale AI stop distance from entry
    tp_mult: float = 1.0
    margin_cap: float = 1.0


@dataclass
class Position:
    symbol: str
    side: str        # "long" | "short"
    entry: float
    notional: float  # USD at entry
    qty: float
    margin: float
    entry_ts: datetime
    stop: float = 0.0
    take: float = 0.0
    last_price: float = 0.0
    last_seen: datetime = None


@dataclass
class Trade:
    symbol: str
    side: str
    entry_ts: datetime
    exit_ts: datetime
    entry: float
    exit: float
    notional: float
    pnl: float      # net of fees
    fees: float
    reason: str


def _ts(s):
    return datetime.fromisoformat(s)


def load_dataset():
    cycles = []
    with open(os.path.join(DATA, "cycles.csv")) as f:
        for r in csv.DictReader(f):
            cycles.append(
                (int(r["cycle_id"]), _ts(r["ts"]),
                 float(r["equity"]) if r["equity"] else None)
            )
    cycles.sort(key=lambda c: c[1])

    decisions = {}
    with open(os.path.join(DATA, "decisions.csv")) as f:
        for r in csv.DictReader(f):
            decisions.setdefault(int(r["cycle_id"]), []).append(
                {
                    "action": r["action"],
                    "symbol": r["symbol"],
                    "price": float(r["price"]),
                    "stop_loss": float(r["stop_loss"]),
                    "take_profit": float(r["take_profit"]),
                    "confidence": float(r["confidence"]),
                }
            )

    prices = {}
    with open(os.path.join(DATA, "prices.csv")) as f:
        for r in csv.DictReader(f):
            prices[(int(r["cycle_id"]), r["symbol"])] = float(r["price"])

    candles = {}
    with open(os.path.join(DATA, "candles.csv")) as f:
        for r in csv.DictReader(f):
            candles.setdefault(r["symbol"], []).append(
                (_ts(r["ts"]), float(r["open"]), float(r["high"]),
                 float(r["low"]), float(r["close"]))
            )
    for sym in candles:
        candles[sym].sort(key=lambda c: c[0])
    candle_times = {s: [c[0] for c in rows] for s, rows in candles.items()}
    return cycles, decisions, prices, candles, candle_times


class Simulator:
    def __init__(self, dataset, start_equity=None):
        self.cycles, self.decisions, self.prices, self.candles, self.candle_times = dataset
        recorded = next((e for _, _, e in self.cycles if e), 100.0)
        self.start_equity = start_equity if start_equity is not None else recorded

    def price_at(self, symbol, cycle_id, ts):
        p = self.prices.get((cycle_id, symbol))
        if p:
            return p
        times = self.candle_times.get(symbol)
        if not times:
            return None
        i = bisect.bisect_right(times, ts) - 1
        return self.candles[symbol][i][4] if i >= 0 else None

    def price_pnl_pct(self, pos, price):
        move = (price - pos.entry) / pos.entry
        if pos.side == "short":
            move = -move
        return move * 100.0

    def run(self, p: Params, start=None, end=None):
        equity = self.start_equity
        positions = {}
        open_times = []          # for the opens-per-hour cap
        last_close = {}          # symbol -> ts
        trades = []
        curve = []
        min_hold = timedelta(hours=p.min_hold_h)
        noise_hold = timedelta(hours=p.min_hold_h + p.noise_hold_extra_h)
        reentry = timedelta(hours=p.reentry_h)

        cycles = [c for c in self.cycles
                  if (start is None or c[1] >= start) and (end is None or c[1] < end)]
        if not cycles:
            return None

        def close_position(pos, price, ts, reason):
            nonlocal equity
            move = (price - pos.entry) / pos.entry
            if pos.side == "short":
                move = -move
            gross = pos.notional * move
            fees = (pos.notional + pos.qty * price) * FEE_RATE
            equity += gross - fees
            trades.append(Trade(pos.symbol, pos.side, pos.entry_ts, ts,
                                pos.entry, price, pos.notional, gross - fees,
                                fees, reason))
            del positions[pos.symbol]
            last_close[pos.symbol] = ts

        for idx, (cycle_id, ts, _) in enumerate(cycles):
            next_ts = cycles[idx + 1][1] if idx + 1 < len(cycles) else ts

            # 1. Candle window since previous cycle: trigger orders + liquidation
            prev_ts = cycles[idx - 1][1] if idx > 0 else ts - timedelta(minutes=30)
            for pos in list(positions.values()):
                times = self.candle_times.get(pos.symbol, [])
                lo = bisect.bisect_right(times, prev_ts)
                hi = bisect.bisect_right(times, ts)
                for cts, o, h, low, c in self.candles.get(pos.symbol, [])[lo:hi]:
                    pos.last_price, pos.last_seen = c, cts
                    liq_move = -(1.0 / p.leverage) * (-LIQUIDATION_MARGIN_PNL)
                    if pos.side == "long":
                        liq_px = pos.entry * (1 + liq_move)
                        if low <= liq_px:
                            close_position(pos, liq_px, cts, "liquidation"); break
                        if pos.stop and low <= pos.stop:
                            close_position(pos, pos.stop, cts, "stop_loss"); break
                        if pos.take and h >= pos.take:
                            close_position(pos, pos.take, cts, "take_profit"); break
                    else:
                        liq_px = pos.entry * (1 - liq_move)
                        if h >= liq_px:
                            close_position(pos, liq_px, cts, "liquidation"); break
                        if pos.stop and h >= pos.stop:
                            close_position(pos, pos.stop, cts, "stop_loss"); break
                        if pos.take and low <= pos.take:
                            close_position(pos, pos.take, cts, "take_profit"); break

            # Stale market data: force-close what we can no longer price
            for pos in list(positions.values()):
                if pos.last_seen and ts - pos.last_seen > DATA_GAP_FORCE_CLOSE:
                    close_position(pos, pos.last_price, ts, "data_gap")

            if equity <= 0:
                return self._metrics(equity, trades, curve, bankrupt=True)

            # 2. Replay this cycle's AI intents
            opens_this_cycle = 0
            for d in self.decisions.get(cycle_id, []):
                sym, act = d["symbol"], d["action"]
                if act in ("close_long", "close_short"):
                    pos = positions.get(sym)
                    side = "long" if act == "close_long" else "short"
                    if not pos or pos.side != side:
                        continue
                    price = self.price_at(sym, cycle_id, ts) or pos.last_price
                    if not price:
                        continue
                    pnl_pct = self.price_pnl_pct(pos, price)
                    held = ts - pos.entry_ts
                    if held >= min_hold:
                        allowed = (held >= noise_hold
                                   or pnl_pct <= p.noise_floor
                                   or pnl_pct >= p.noise_ceiling)
                    else:
                        allowed = pnl_pct <= p.sl_bypass or pnl_pct >= p.tp_bypass
                    if allowed:
                        close_position(pos, price, ts, "ai_close")

                elif act in ("open_long", "open_short"):
                    if d["confidence"] < p.min_confidence:
                        continue
                    if opens_this_cycle >= p.max_opens_per_cycle:
                        continue
                    if sym in positions or len(positions) >= p.max_positions:
                        continue
                    hour_ago = ts - timedelta(hours=1)
                    open_times[:] = [t for t in open_times if t >= hour_ago]
                    if len(open_times) >= p.max_opens_per_hour:
                        continue
                    if sym in last_close and ts - last_close[sym] < reentry:
                        continue
                    price = self.price_at(sym, cycle_id, ts) or d["price"]
                    if not price or price <= 0:
                        continue
                    notional = p.ratio * equity
                    margin_used = sum(x.margin for x in positions.values())
                    margin_free = p.margin_cap * equity - margin_used
                    notional = min(notional, max(0.0, margin_free) * p.leverage)
                    if notional < MIN_POSITION_USD:
                        continue
                    side = "long" if act == "open_long" else "short"
                    stop = take = 0.0
                    if d["stop_loss"] > 0:
                        stop = price - (price - d["stop_loss"]) * p.sl_mult \
                            if side == "long" else \
                            price + (d["stop_loss"] - price) * p.sl_mult
                        if (side == "long") != (stop < price):
                            stop = 0.0
                    if d["take_profit"] > 0:
                        take = price + (d["take_profit"] - price) * p.tp_mult \
                            if side == "long" else \
                            price - (price - d["take_profit"]) * p.tp_mult
                        if (side == "long") != (take > price):
                            take = 0.0
                    equity -= notional * FEE_RATE
                    positions[sym] = Position(
                        symbol=sym, side=side, entry=price, notional=notional,
                        qty=notional / price, margin=notional / p.leverage,
                        entry_ts=ts, stop=stop, take=take,
                        last_price=price, last_seen=ts,
                    )
                    open_times.append(ts)
                    opens_this_cycle += 1

            # 3. Mark to market
            mtm = equity
            for pos in positions.values():
                price = self.price_at(pos.symbol, cycle_id, ts) or pos.last_price
                move = (price - pos.entry) / pos.entry
                if pos.side == "short":
                    move = -move
                mtm += pos.notional * move
            curve.append((ts, mtm))
            if mtm <= 0:
                return self._metrics(equity, trades, curve, bankrupt=True)

        for pos in list(positions.values()):
            close_position(pos, pos.last_price or pos.entry,
                           cycles[-1][1], "end_of_data")
        curve.append((cycles[-1][1], equity))
        return self._metrics(equity, trades, curve, bankrupt=False)

    def _metrics(self, equity, trades, curve, bankrupt):
        peak, max_dd = -1e18, 0.0
        for _, v in curve:
            peak = max(peak, v)
            if peak > 0:
                max_dd = max(max_dd, (peak - v) / peak)
        daily = {}
        for ts, v in curve:
            daily[ts.date()] = v
        vals = list(daily.values())
        rets = [(b - a) / a for a, b in zip(vals, vals[1:]) if a > 0]
        sharpe = 0.0
        if len(rets) > 1:
            mean = sum(rets) / len(rets)
            var = sum((r - mean) ** 2 for r in rets) / (len(rets) - 1)
            if var > 0:
                sharpe = mean / var ** 0.5 * (365 ** 0.5)
        wins = sum(1 for t in trades if t.pnl > 0)
        return {
            "final_equity": equity,
            "net_pnl": equity - self.start_equity,
            "ret_pct": (equity / self.start_equity - 1) * 100,
            "max_dd_pct": max_dd * 100,
            "sharpe": sharpe,
            "trades": len(trades),
            "win_rate": wins / len(trades) * 100 if trades else 0.0,
            "fees": sum(t.fees for t in trades),
            "liquidations": sum(1 for t in trades if t.reason == "liquidation"),
            "bankrupt": bankrupt,
        }


if __name__ == "__main__":
    sim = Simulator(load_dataset())
    live = Params()
    m = sim.run(live)
    print("live-config replay:", {k: round(v, 2) if isinstance(v, float) else v
                                  for k, v in m.items()})
