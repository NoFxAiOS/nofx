#!/usr/bin/env python3
import sqlite3
import sys
from collections import Counter

DB_PATH = sys.argv[1] if len(sys.argv) > 1 else 'data/data.db'
LOOKBACK_HOURS = int(sys.argv[2]) if len(sys.argv) > 2 else 24

conn = sqlite3.connect(DB_PATH)
conn.row_factory = sqlite3.Row
cur = conn.cursor()

rows = cur.execute(
    """
    select
      e.id,
      e.symbol,
      e.side,
      e.close_reason,
      e.execution_source,
      e.execution_type,
      e.exchange_order_id,
      datetime(e.event_time/1000,'unixepoch','localtime') as event_local,
      o.order_action,
      o.client_order_id,
      o.type as order_type
    from position_close_events e
    left join trader_orders o
      on o.exchange_id = e.exchange_id and o.exchange_order_id = e.exchange_order_id
    where e.event_time >= (strftime('%s','now', ?)*1000)
    order by e.event_time desc, e.id desc
    """,
    (f'-{LOOKBACK_HOURS} hours',)
).fetchall()

rich_reasons = {
    'break_even_stop',
    'managed_drawdown',
    'native_trailing',
    'ladder_tp',
    'ladder_sl',
    'full_tp',
    'full_sl',
}

source_counter = Counter()
rich_rows = []
flat_rows = []

for r in rows:
    source = (r['execution_source'] or r['close_reason'] or '').strip()
    source_counter[source or ''] += 1
    item = {
        'id': r['id'],
        'symbol': r['symbol'],
        'side': r['side'],
        'event_local': r['event_local'],
        'close_reason': r['close_reason'] or '',
        'execution_source': r['execution_source'] or '',
        'execution_type': r['execution_type'] or '',
        'order_action': r['order_action'] or '',
        'client_order_id': r['client_order_id'] or '',
        'order_type': r['order_type'] or '',
    }
    if source in rich_reasons:
        rich_rows.append(item)
    else:
        flat_rows.append(item)

print(f"# Post-fix live verification window: last {LOOKBACK_HOURS}h")
print(f"Total close events: {len(rows)}")
print(f"Rich-attributed events: {len(rich_rows)}")
print(f"Flat/generic events: {len(flat_rows)}")
print()
print("## Reason/source distribution")
for reason, count in source_counter.most_common():
    label = reason if reason else '<empty>'
    print(f"- {label}: {count}")

print()
print("## Rich-attributed samples")
if not rich_rows:
    print("(none yet)")
else:
    for item in rich_rows[:20]:
        print(
            f"- [{item['event_local']}] {item['symbol']} {item['side']} "
            f"reason={item['close_reason']} source={item['execution_source']} type={item['execution_type']} "
            f"order_action={item['order_action']} client_order_id={item['client_order_id']}"
        )

print()
print("## Flat/generic samples needing further follow-up")
for item in flat_rows[:20]:
    print(
        f"- [{item['event_local']}] {item['symbol']} {item['side']} "
        f"reason={item['close_reason']} source={item['execution_source']} type={item['execution_type']} "
        f"order_action={item['order_action']} client_order_id={item['client_order_id']}"
    )

print()
print("## Interpretation")
if rich_rows:
    print("- The attribution fix is now observable in live data: at least some close events carry specific protection reasons.")
else:
    print("- No rich protection reasons observed in this window yet.")
    print("- This does not mean the fix failed by itself: the window may contain only pre-fix events or generic close paths.")
    print("- If new post-fix fills continue to stay flat, inspect upstream tagging / order_action propagation.")
