#!/usr/bin/env python3
import json
import sqlite3
import sys
from collections import Counter
from typing import Any

OPEN_ACTIONS = {"open_long", "open_short"}


def safe_json_loads(text: Any, fallback: Any) -> Any:
    if text in (None, ""):
        return fallback
    if isinstance(text, (dict, list)):
        return text
    try:
        return json.loads(text)
    except Exception:
        return fallback


def yn(value: bool) -> str:
    return "yes" if value else "no"


def compact_list(values: list[str], limit: int = 3) -> str:
    items = [v for v in values if v]
    if not items:
        return "-"
    if len(items) <= limit:
        return ", ".join(items)
    return ", ".join(items[:limit]) + f" (+{len(items) - limit} more)"


def get_checklist_flags(epr: dict[str, Any]) -> dict[str, bool]:
    timeframe_context = epr.get("timeframe_context") if isinstance(epr.get("timeframe_context"), dict) else {}
    key_levels = epr.get("key_levels") if isinstance(epr.get("key_levels"), dict) else {}
    anchors = epr.get("anchors") if isinstance(epr.get("anchors"), list) else []
    risk_reward = epr.get("risk_reward") if isinstance(epr.get("risk_reward"), dict) else {}
    return {
        "primary_tf": bool(timeframe_context.get("primary")),
        "adjacent_tf": bool(timeframe_context.get("lower") or timeframe_context.get("higher")),
        "support": bool(key_levels.get("support")),
        "resistance": bool(key_levels.get("resistance")),
        "anchors": bool(anchors),
        "rr": all(risk_reward.get(k) not in (None, 0, 0.0, "") for k in ("entry", "invalidation", "first_target", "gross_estimated_rr")),
    }


def main() -> int:
    db_path = sys.argv[1] if len(sys.argv) > 1 else "data/data.db"
    lookback_hours = int(sys.argv[2]) if len(sys.argv) > 2 else 24
    limit = int(sys.argv[3]) if len(sys.argv) > 3 else 200

    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    cur = conn.cursor()

    rows = cur.execute(
        """
        SELECT id, trader_id, cycle_number, timestamp, decisions, review_context, success, error_message
        FROM decision_records
        WHERE timestamp >= datetime('now', ?)
        ORDER BY timestamp DESC
        LIMIT ?
        """,
        (f'-{lookback_hours} hours', limit),
    ).fetchall()

    open_records = []
    blocked_rows = []
    structural_rows = []
    change_verify_rows = []
    failed_check_counter: Counter[str] = Counter()
    control_decision_counter: Counter[str] = Counter()

    for row in rows:
        decisions = safe_json_loads(row["decisions"], [])
        if not isinstance(decisions, list):
            continue

        for action in decisions:
            if not isinstance(action, dict):
                continue
            if action.get("action") not in OPEN_ACTIONS:
                continue

            open_records.append((row, action))
            review_context = action.get("review_context") if isinstance(action.get("review_context"), dict) else {}
            control = review_context.get("control") if isinstance(review_context.get("control"), dict) else {}
            epr = action.get("entry_protection_rationale") if isinstance(action.get("entry_protection_rationale"), dict) else {}
            flags = get_checklist_flags(epr)
            missing = [
                label for label, ok in [
                    ("primary_tf", flags["primary_tf"]),
                    ("adjacent_tf", flags["adjacent_tf"]),
                    ("support", flags["support"]),
                    ("resistance", flags["resistance"]),
                    ("anchors", flags["anchors"]),
                    ("rr", flags["rr"]),
                ] if not ok
            ]

            decision = str(control.get("decision") or "")
            failed_checks = control.get("failed_checks") if isinstance(control.get("failed_checks"), list) else []
            reasons = control.get("reasons") if isinstance(control.get("reasons"), list) else []

            if decision:
                control_decision_counter[decision] += 1
            for item in failed_checks:
                if item:
                    failed_check_counter[str(item)] += 1

            item = {
                "id": row["id"],
                "ts": row["timestamp"],
                "trader": row["trader_id"],
                "cycle": row["cycle_number"],
                "symbol": action.get("symbol") or "",
                "action": action.get("action") or "",
                "control_decision": decision or "-",
                "failed_checks": [str(x) for x in failed_checks if x],
                "reasons": [str(x) for x in reasons if x],
                "no_order_placed": bool(control.get("no_order_placed")),
                "original_action": control.get("original_action") or action.get("original_action") or "",
                "final_action": control.get("final_action") or action.get("final_action") or "",
                "epr_present": bool(epr),
                "missing": missing,
            }

            if decision in {"rejected", "downgraded", "downgraded_to_wait"} or item["no_order_placed"]:
                blocked_rows.append(item)
            if epr or missing:
                structural_rows.append(item)
            if decision not in ("", "-") or failed_checks or item["original_action"] or item["final_action"]:
                change_verify_rows.append(item)

    print(f"# nofxmax daily strategy-governance audit")
    print(f"window: last {lookback_hours}h | scanned decision_records: {len(rows)} | open actions found: {len(open_records)}")
    print()

    print("## 1) Blocked opportunity audit")
    print(f"blocked/downgraded/rejected opens: {len(blocked_rows)}")
    if control_decision_counter:
        print("control decisions:")
        for key, count in control_decision_counter.most_common():
            print(f"- {key}: {count}")
    if failed_check_counter:
        print("failed checks:")
        for key, count in failed_check_counter.most_common():
            print(f"- {key}: {count}")
    if not blocked_rows:
        print("- no blocked open candidates observed in this window")
    else:
        for item in blocked_rows[:12]:
            transition = ""
            if item["original_action"] or item["final_action"]:
                transition = f" transition={item['original_action'] or item['action']}->{item['final_action'] or item['action']}"
            print(
                f"- [{item['ts']}] id={item['id']} {item['symbol']} {item['action']} decision={item['control_decision']}"
                f" no_order_placed={yn(item['no_order_placed'])}{transition}"
                f" failed={compact_list(item['failed_checks'])} reasons={compact_list(item['reasons'])}"
            )
    print()

    print("## 2) Entry structure adherence audit")
    if not structural_rows:
        print("- no open actions found, so no structure adherence sample is available")
    else:
        complete = sum(1 for item in structural_rows if item["epr_present"] and not item["missing"])
        with_epr = sum(1 for item in structural_rows if item["epr_present"])
        print(f"open actions with entry_protection_rationale: {with_epr}/{len(structural_rows)}")
        print(f"open actions meeting compact structure checklist: {complete}/{len(structural_rows)}")
        print("checklist = primary_tf + adjacent_tf + support + resistance + anchors + rr")
        for item in structural_rows[:12]:
            print(
                f"- [{item['ts']}] id={item['id']} {item['symbol']} {item['action']}"
                f" epr_present={yn(item['epr_present'])} missing={compact_list(item['missing'], limit=6)}"
            )
    print()

    print("## 3) Post-strategy-change verification")
    if not change_verify_rows:
        print("- no action-level control annotations observed yet in this window")
        print("- if a strategy/policy change was just shipped, widen the lookback or wait for fresh open candidates")
    else:
        print(f"annotated open actions: {len(change_verify_rows)}")
        for item in change_verify_rows[:12]:
            transition = ""
            if item["original_action"] or item["final_action"]:
                transition = f" transition={item['original_action'] or item['action']}->{item['final_action'] or item['action']}"
            print(
                f"- [{item['ts']}] id={item['id']} {item['symbol']} decision={item['control_decision']}"
                f" failed={compact_list(item['failed_checks'])}{transition}"
            )
    print()

    print("## Operator interpretation")
    if blocked_rows:
        print("- Review repeated failed checks first. Repetition means the strategy contract is probably too strict, stale, or mismatched to live execution reality.")
    else:
        print("- No blocked opens is neutral, not automatically good. Confirm there were real open candidates in the window.")
    if structural_rows and sum(1 for item in structural_rows if item["epr_present"] and not item["missing"]) < len(structural_rows):
        print("- Missing structure fields means the open thesis is not audit-clean enough for governance, even if the trade idea looked attractive.")
    if change_verify_rows:
        print("- Fresh control annotations after a strategy edit are the fastest proof that the new policy path is alive in persisted decisions.")
    else:
        print("- No fresh control annotations means post-change verification is still incomplete; use a larger lookback or wait for the next open attempt.")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
