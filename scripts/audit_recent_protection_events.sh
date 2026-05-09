#!/usr/bin/env bash
set -euo pipefail

DB_PATH="${1:-data/data.db}"

sqlite3 -header -column "$DB_PATH" <<'SQL'
select
  p.id as pos_id,
  p.symbol,
  p.side,
  printf('%.8f', p.entry_price) as entry_price,
  printf('%.8f', p.entry_quantity) as entry_qty,
  datetime(p.entry_time/1000,'unixepoch','localtime') as entry_local,
  datetime(p.exit_time/1000,'unixepoch','localtime') as exit_local,
  p.close_reason as position_close_reason,
  e.exchange_order_id,
  e.close_reason as event_close_reason,
  e.execution_source,
  e.execution_type,
  printf('%.8f', e.close_quantity) as close_qty,
  printf('%.8f', e.execution_price) as exec_price,
  round(((case when p.side='LONG' then (e.execution_price-p.entry_price) else (p.entry_price-e.execution_price) end)/p.entry_price)*100, 4) as pnl_pct,
  datetime(e.event_time/1000,'unixepoch','localtime') as event_local
from trader_positions p
join position_close_events e on e.position_id = p.id
where p.updated_at >= (strftime('%s','now','-1 day')*1000)
order by p.id, e.event_time;
SQL
