# Open Candidate Observation Recipe — Post Strategy-Control / Entry-Structure Changes

## Goal

When a fresh `open_long` / `open_short` candidate appears, capture the exact fields needed to verify whether the recent strategy-control and entry-structure changes are visible in persisted decision data:

- `raw_response`
- `decision_json`
- parsed `decisions`
- `review_context.control`
- whether `entry_protection_rationale` is present

This is a **minimal observation flow**, not a full audit.

## Fast path

From repo root:

```bash
python3 scripts/observe_open_candidates.py data/data.db 30
```

- `30` = inspect latest 30 decision records.
- Increase if no recent open candidate is found.

## What the script prints

For each recent record containing at least one `open_long` or `open_short` action, it prints:

- decision record id / trader / cycle / timestamp
- whether top-level `review_context.control` exists
- for each open action:
  - action + symbol
  - whether `entry_protection_rationale` exists
  - whether action-level `review_context` exists
  - whether action-level `review_context.control` exists
  - compact dump of the action itself
- compact sample of:
  - `decision_json`
  - `raw_response`

## Exact manual fallback

If you do not want to use the helper script, use this one-shot SQLite inspection:

```bash
python3 - <<'PY'
import sqlite3, json
conn = sqlite3.connect('data/data.db')
conn.row_factory = sqlite3.Row
cur = conn.cursor()
rows = cur.execute("""
SELECT id, trader_id, cycle_number, timestamp,
       decision_json, raw_response, decisions, review_context
FROM decision_records
ORDER BY timestamp DESC
LIMIT 30
""").fetchall()
for r in rows:
    try:
        decisions = json.loads(r['decisions'] or '[]')
    except Exception:
        continue
    opens = [d for d in decisions if isinstance(d, dict) and d.get('action') in ('open_long','open_short')]
    if not opens:
        continue
    print('\n===', r['id'], r['timestamp'], '===')
    print('root review_context =', (r['review_context'] or '')[:400])
    print('decision_json =', (r['decision_json'] or '')[:600])
    print('raw_response =', (r['raw_response'] or '')[:600])
    for d in opens:
        print('action =', d.get('action'), 'symbol =', d.get('symbol'))
        print('entry_protection_rationale present =', bool(d.get('entry_protection_rationale')))
        rc = d.get('review_context') or {}
        print('action review_context.control =', json.dumps(rc.get('control'), ensure_ascii=False)[:500])
        print('decision =', json.dumps(d, ensure_ascii=False)[:700])
PY
```

## Current baseline observed in live data

On the current DB snapshot (`data/data.db`) at the time this recipe was created:

- recent open candidates **do exist**;
- `raw_response` is populated;
- `decision_json` is populated;
- parsed `decisions` are populated;
- action-level `review_context` is currently absent on the sampled recent open actions;
- `review_context.control` was not present on sampled recent open actions;
- `entry_protection_rationale` was not present on sampled recent open actions.

A concrete recent example was `decision_records.id=7428` (`ZECUSDT open_long`, `2026-04-22 14:43:41Z`).

## Minimal operator checklist

When a new open candidate appears, check:

1. `raw_response` exists and still includes the full reasoning / decision block.
2. `decision_json` contains the candidate in structured form.
3. parsed `decisions` contains the matching `open_long` / `open_short` action.
4. action-level `review_context.control` exists if the new control path is expected to annotate it.
5. `entry_protection_rationale` exists on the open action if the new entry-structure contract is expected to persist it.
6. If 4 or 5 are missing, save the printed record id + timestamp as the exact follow-up anchor.

## Interpretation

- If fields 1–3 exist but 4–5 do not, the candidate is still observable, but the new structured audit payload may not yet be reaching persisted `decisions` rows.
- If 1–3 are missing, this is a broader decision-record persistence/parsing problem, not only a control/entry-structure issue.
