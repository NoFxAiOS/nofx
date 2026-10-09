"""Extract a replayable dataset from data/data.db.

Reads decision_records and emits three CSVs under scripts/optimize/data/:
  cycles.csv    - one row per decision cycle (timestamp, recorded equity)
  decisions.csv - the AI's intended actions per cycle (including throttled ones)
  candles.csv   - deduplicated per-symbol 15m OHLCV parsed from input prompts
  prices.csv    - per-cycle current_price for every symbol present in the prompt

Run:  .venv/bin/python extract.py [--db ../../data/data.db]
"""

import argparse
import csv
import json
import os
import re
import sqlite3
from datetime import datetime, timedelta, timezone

SECTION_RE = re.compile(r"^=== (\S+) Market Data ===$")
CURRENT_PRICE_RE = re.compile(r"^current_price = ([\d.eE+-]+)$")
CANDLE_RE = re.compile(
    r"^(\d{2})-(\d{2}) (\d{2}):(\d{2})\s+"
    r"([\d.eE+-]+)\s+([\d.eE+-]+)\s+([\d.eE+-]+)\s+([\d.eE+-]+)\s+([\d.eE+-]+)\s*$"
)
EQUITY_RE = re.compile(r"Account: Equity ([\d.]+)")
TS_RE = re.compile(r"(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2}:\d{2})")


def parse_cycle_ts(raw):
    m = TS_RE.search(raw)
    if not m:
        return None
    return datetime.strptime(
        f"{m.group(1)} {m.group(2)}", "%Y-%m-%d %H:%M:%S"
    ).replace(tzinfo=timezone.utc)


def candle_ts(cycle_ts, month, day, hour, minute):
    """Candle rows carry no year; anchor to the cycle timestamp."""
    ts = datetime(cycle_ts.year, month, day, hour, minute, tzinfo=timezone.utc)
    if ts > cycle_ts + timedelta(days=2):
        ts = ts.replace(year=cycle_ts.year - 1)
    return ts


def parse_prompt(prompt, cycle_ts):
    equity = None
    m = EQUITY_RE.search(prompt)
    if m:
        equity = float(m.group(1))

    prices = {}
    candles = []
    symbol = None
    for line in prompt.splitlines():
        line = line.rstrip()
        sm = SECTION_RE.match(line)
        if sm:
            symbol = sm.group(1).upper()
            continue
        if symbol is None:
            continue
        pm = CURRENT_PRICE_RE.match(line)
        if pm:
            prices[symbol] = float(pm.group(1))
            continue
        cm = CANDLE_RE.match(line)
        if cm:
            ts = candle_ts(cycle_ts, *(int(cm.group(i)) for i in range(1, 5)))
            o, h, low, c, v = (float(cm.group(i)) for i in range(5, 10))
            candles.append((symbol, ts, o, h, low, c, v))
    return equity, prices, candles


def parse_decisions(raw):
    try:
        items = json.loads(raw or "[]")
    except json.JSONDecodeError:
        return []
    out = []
    for d in items if isinstance(items, list) else []:
        if not isinstance(d, dict):
            continue
        out.append(
            {
                "action": str(d.get("action", "")).strip().lower(),
                "symbol": str(d.get("symbol", "")).strip().upper(),
                "leverage": d.get("leverage") or 0,
                "price": d.get("price") or 0.0,
                "stop_loss": d.get("stop_loss") or 0.0,
                "take_profit": d.get("take_profit") or 0.0,
                "confidence": d.get("confidence") or 0,
                "recorded_success": bool(d.get("success")),
                "recorded_error": str(d.get("error", "")),
            }
        )
    return out


def main():
    ap = argparse.ArgumentParser()
    here = os.path.dirname(os.path.abspath(__file__))
    ap.add_argument("--db", default=os.path.join(here, "..", "..", "data", "data.db"))
    ap.add_argument("--out", default=os.path.join(here, "data"))
    args = ap.parse_args()

    os.makedirs(args.out, exist_ok=True)
    conn = sqlite3.connect(f"file:{args.db}?mode=ro", uri=True)
    rows = conn.execute(
        "SELECT id, timestamp, input_prompt, decisions FROM decision_records"
        " ORDER BY timestamp"
    )

    cycles, decisions = [], []
    candle_map = {}  # (symbol, ts) -> row, latest observation wins
    price_rows = []
    skipped = 0

    for rec_id, ts_raw, prompt, decisions_raw in rows:
        cycle_ts = parse_cycle_ts(str(ts_raw))
        if cycle_ts is None:
            skipped += 1
            continue
        equity, prices, candles = parse_prompt(prompt or "", cycle_ts)
        cycles.append((rec_id, cycle_ts.isoformat(), equity if equity is not None else ""))
        for sym, price in prices.items():
            price_rows.append((rec_id, cycle_ts.isoformat(), sym, price))
        for sym, cts, o, h, low, c, v in candles:
            candle_map[(sym, cts)] = (sym, cts.isoformat(), o, h, low, c, v)
        for d in parse_decisions(decisions_raw):
            decisions.append(
                (
                    rec_id,
                    cycle_ts.isoformat(),
                    d["action"],
                    d["symbol"],
                    d["leverage"],
                    d["price"],
                    d["stop_loss"],
                    d["take_profit"],
                    d["confidence"],
                    int(d["recorded_success"]),
                    d["recorded_error"],
                )
            )

    def write(name, header, data):
        path = os.path.join(args.out, name)
        with open(path, "w", newline="") as f:
            w = csv.writer(f)
            w.writerow(header)
            w.writerows(data)
        return path

    write("cycles.csv", ["cycle_id", "ts", "equity"], cycles)
    write(
        "decisions.csv",
        [
            "cycle_id", "ts", "action", "symbol", "leverage", "price",
            "stop_loss", "take_profit", "confidence", "recorded_success",
            "recorded_error",
        ],
        decisions,
    )
    write(
        "candles.csv",
        ["symbol", "ts", "open", "high", "low", "close", "volume"],
        sorted(candle_map.values(), key=lambda r: (r[0], r[1])),
    )
    write("prices.csv", ["cycle_id", "ts", "symbol", "price"], price_rows)

    print(
        f"cycles={len(cycles)} decisions={len(decisions)}"
        f" candles={len(candle_map)} prices={len(price_rows)} skipped={skipped}"
    )


if __name__ == "__main__":
    main()
