#!/usr/bin/env python3
import json
import sqlite3
import sys
from typing import Any

OPEN_ACTIONS = {"open_long", "open_short"}


def truncate(value: Any, limit: int = 800) -> str:
    if value is None:
        return ""
    text = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False)
    text = text.strip()
    if len(text) <= limit:
        return text
    return text[:limit] + " ...[truncated]"


def safe_json_loads(text: str, fallback: Any) -> Any:
    if not text:
        return fallback
    try:
        return json.loads(text)
    except Exception:
        return fallback


def main() -> int:
    db_path = sys.argv[1] if len(sys.argv) > 1 else "data/data.db"
    limit = int(sys.argv[2]) if len(sys.argv) > 2 else 30

    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    cur = conn.cursor()
    rows = cur.execute(
        """
        SELECT id, trader_id, cycle_number, timestamp,
               decision_json, raw_response, decisions, review_context,
               success, error_message
        FROM decision_records
        ORDER BY timestamp DESC
        LIMIT ?
        """,
        (limit,),
    ).fetchall()

    found = 0
    for row in rows:
        decisions = safe_json_loads(row["decisions"], [])
        if not isinstance(decisions, list):
            continue

        open_actions = [d for d in decisions if isinstance(d, dict) and d.get("action") in OPEN_ACTIONS]
        if not open_actions:
            continue

        found += 1
        print(f"=== decision_record id={row['id']} trader={row['trader_id']} cycle={row['cycle_number']} ts={row['timestamp']} success={bool(row['success'])} ===")
        if row["error_message"]:
            print(f"error_message: {truncate(row['error_message'], 300)}")

        root_review = safe_json_loads(row["review_context"], {})
        print(f"root.review_context.control_present={isinstance(root_review, dict) and ('control' in root_review) and bool(root_review.get('control'))}")

        for idx, action in enumerate(open_actions, 1):
            action_review = action.get("review_context") if isinstance(action.get("review_context"), dict) else {}
            control = action_review.get("control") if isinstance(action_review, dict) else None
            epr = action.get("entry_protection_rationale")
            print(f"-- open_action[{idx}] action={action.get('action')} symbol={action.get('symbol')}")
            print(f"   entry_protection_rationale_present={bool(epr)}")
            print(f"   action.review_context_present={bool(action_review)}")
            print(f"   action.review_context.control_present={bool(control)}")
            if control:
                print(f"   action.review_context.control={truncate(control, 1200)}")
            if epr:
                print(f"   entry_protection_rationale={truncate(epr, 1200)}")
            print(f"   decision_action={truncate(action, 1200)}")

        decision_json = safe_json_loads(row["decision_json"], row["decision_json"])
        print("decision_json_sample:")
        print(truncate(decision_json, 1600))
        print("raw_response_sample:")
        print(truncate(row["raw_response"], 1600))
        print()

    if found == 0:
        print(f"No open_long/open_short candidates found in latest {limit} decision_records from {db_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
