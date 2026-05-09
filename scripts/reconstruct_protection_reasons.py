#!/usr/bin/env python3
import sqlite3
import sys
from math import fabs

DB_PATH = sys.argv[1] if len(sys.argv) > 1 else 'data/data.db'

conn = sqlite3.connect(DB_PATH)
conn.row_factory = sqlite3.Row
cur = conn.cursor()

query = """
select
  p.id as pos_id,
  p.symbol,
  p.side,
  p.entry_price,
  p.entry_quantity,
  p.close_reason as position_close_reason,
  datetime(p.entry_time/1000,'unixepoch','localtime') as entry_local,
  datetime(p.exit_time/1000,'unixepoch','localtime') as exit_local,
  e.id as event_id,
  e.exchange_order_id,
  e.close_reason as event_close_reason,
  e.execution_source,
  e.execution_type,
  e.close_quantity,
  e.close_ratio_pct,
  e.execution_price,
  datetime(e.event_time/1000,'unixepoch','localtime') as event_local,
  o.client_order_id,
  o.order_action,
  o.type as order_type
from trader_positions p
join position_close_events e on e.position_id = p.id
left join trader_orders o on o.exchange_id = p.exchange_id and o.exchange_order_id = e.exchange_order_id
where p.updated_at >= (strftime('%s','now','-1 day')*1000)
order by p.id, e.event_time, e.id
"""

rows = cur.execute(query).fetchall()


def pnl_pct(side, entry, price):
    if not entry:
        return None
    if str(side).upper() == 'LONG':
        return (price - entry) / entry * 100.0
    return (entry - price) / entry * 100.0


def infer_reason(row):
    client = (row['client_order_id'] or '').lower()
    action = (row['order_action'] or '').lower()
    otype = (row['order_type'] or row['execution_type'] or '').upper()
    pos_side = (row['side'] or '').upper()
    entry = row['entry_price'] or 0.0
    price = row['execution_price'] or 0.0
    ratio = row['close_ratio_pct'] or 0.0
    pnl = pnl_pct(pos_side, entry, price)
    evidence = []

    def mark(reason, why):
        evidence.append(why)
        return reason, '; '.join(evidence)

    if 'managed_drawdown' in client or 'managed_drawdown' in action:
        return mark('managed_drawdown', 'tag/action contains managed_drawdown')
    if 'break_even' in client or 'break_even' in action:
        return mark('break_even_stop', 'tag/action contains break_even')
    if 'native_trailing' in client or 'native_trailing' in action or 'TRAILING' in otype:
        return mark('native_trailing', 'tag/action/type indicates trailing')
    if 'ladder_tp' in client or 'ladder_tp' in action:
        return mark('ladder_tp', 'tag/action contains ladder_tp')
    if 'ladder_sl' in client or 'ladder_sl' in action:
        return mark('ladder_sl', 'tag/action contains ladder_sl')
    if 'full_tp' in client or 'full_tp' in action:
        return mark('full_tp', 'tag/action contains full_tp')
    if 'full_sl' in client or 'full_sl' in action or 'fallback_maxloss' in client or 'fallback_maxloss' in action:
        return mark('full_sl', 'tag/action contains full_sl/fallback_maxloss')

    if 'TAKE_PROFIT' in otype or otype.endswith('TP'):
        if ratio and ratio < 99.999:
            return mark('ladder_tp', 'order type indicates partial TP')
        return mark('full_tp', 'order type indicates TP')

    if 'STOP' in otype or otype.endswith('SL'):
        if entry > 0 and fabs(price - entry) / entry <= 0.003:
            return mark('break_even_stop', 'stop execution near entry price')
        if ratio and ratio < 99.999:
            return mark('ladder_sl', 'stop execution on partial size')
        return mark('full_sl', 'stop execution on near-full size')

    if pnl is not None:
        if pnl >= 0.6 and entry > 0 and fabs(price - entry) / entry <= 0.0125:
            return mark('break_even_stop?', 'profit-side protective exit consistent with BE regime')
        if pnl > 0 and pnl < 0.6:
            return mark('protective_profit_exit?', 'small-profit exit below BE/drawdown gates')
        if pnl <= 0:
            if ratio and ratio < 99.999:
                return mark('ladder_sl?', 'loss-side partial exit consistent with SL stack')
            return mark('full_sl?', 'loss-side full exit consistent with SL/fallback')

    raw = row['execution_source'] or row['event_close_reason'] or row['position_close_reason'] or 'unknown'
    return raw, 'fallback to raw stored reason'


print('\t'.join([
    'pos_id','symbol','side','event_id','event_local','entry_price','exec_price','close_qty','close_ratio_pct','pnl_pct',
    'stored_reason','stored_source','order_action','client_order_id','inferred_reason','evidence'
]))

for r in rows:
    inferred, evidence = infer_reason(r)
    pnl = pnl_pct(r['side'], r['entry_price'] or 0.0, r['execution_price'] or 0.0)
    print('\t'.join([
        str(r['pos_id']),
        str(r['symbol'] or ''),
        str(r['side'] or ''),
        str(r['event_id']),
        str(r['event_local'] or ''),
        f"{(r['entry_price'] or 0.0):.8f}",
        f"{(r['execution_price'] or 0.0):.8f}",
        f"{(r['close_quantity'] or 0.0):.8f}",
        f"{(r['close_ratio_pct'] or 0.0):.4f}",
        '' if pnl is None else f"{pnl:.4f}",
        str(r['event_close_reason'] or ''),
        str(r['execution_source'] or ''),
        str(r['order_action'] or ''),
        str(r['client_order_id'] or ''),
        inferred,
        evidence,
    ]))
