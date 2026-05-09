# Daily Quantitative Observation Loop — nofxmax Strategy Governance

_Updated: 2026-04-23_

Goal: keep daily governance **lightweight but falsifiable**. The operator should be able to answer three questions in under 10 minutes:

1. **Blocked opportunity audit** — did the system block or downgrade potentially tradable opens, and why?
2. **Entry structure adherence audit** — when opens appeared, did they carry the minimum structure needed for trustworthy review?
3. **Post-strategy-change verification** — after changing policy/structure settings, can we see the new control path in persisted decisions?

This builds on:
- `scripts/observe_open_candidates.py`
- `docs/OPEN_CANDIDATE_OBSERVATION_RECIPE_2026-04-22.md`
- `docs/STRUCTURAL_ENTRY_CONTRACT_2026-04-21.md`
- `docs/SYSTEM_JUDGMENT_PRIORITY_2026-04-20.md`

---

## Daily operator flow

From repo root:

```bash
python3 scripts/daily_strategy_governance_audit.py data/data.db 24 200
```

Arguments:
- `24` = lookback hours
- `200` = max decision records to scan in that window

If there were no open attempts in the last 24h, widen the window:

```bash
python3 scripts/daily_strategy_governance_audit.py data/data.db 72 500
```

Use `scripts/observe_open_candidates.py` only as a drill-down tool after this daily summary tells you where to look.

---

## What “good enough” looks like

### 1) Blocked opportunity audit

Check the report section:
- `blocked/downgraded/rejected opens`
- `control decisions`
- `failed checks`

Interpretation:
- **Healthy**: few blocked opens, and reasons are understandable / intentional.
- **Needs attention**: the same failed check repeats (`runtime_rr_below_min`, future `protection_alignment_mismatch`, etc.) across many symbols/cycles.
- **Governance red flag**: many downgrades/rejections but no obvious improvement in strategy quality after edits.

Daily action:
- Save 1–3 representative record ids when a failure pattern repeats.
- Do **not** debate anecdotes; work from repeated failed-check patterns.

### 2) Entry structure adherence audit

The compact checklist is:
- `primary_tf`
- `adjacent_tf`
- `support`
- `resistance`
- `anchors`
- `rr`

Interpretation:
- **Healthy**: recent open actions contain `entry_protection_rationale` and pass the compact structure checklist.
- **Weak but observable**: open actions exist, but rationale or structure fields are missing.
- **Not testable yet**: no recent open actions in the window.

Governance rule:
- If an open thesis is missing structure fields, treat it as **not audit-clean**, even if the trade looked directionally correct.
- Repeated missing structure is usually a prompt/runtime persistence problem, not just an operator review problem.

### 3) Post-strategy-change verification

After any strategy edit affecting:
- `strategy_control_policy.mode`
- `entry_structure.*`
- RR thresholds
- protection alignment behavior

run the audit again and check whether fresh open actions now show:
- action-level `review_context.control`
- expected `control.decision`
- expected `failed_checks`
- `original_action -> final_action` when recommend-only downgrades are expected
- `no_order_placed=yes` when opens were blocked/downgraded

Interpretation:
- **Verified**: persisted decisions show the new control path.
- **Not yet verified**: no fresh annotated opens appeared.
- **Suspicious**: strategy was changed, but new records still look identical to pre-change behavior.

---

## Recommended daily threshold logic

Keep it simple. Use these operator thresholds:

- **Green**
  - blocked opens <= 2 in 24h
  - no repeated failed-check cluster
  - any observed opens are structurally complete

- **Yellow**
  - blocked opens 3–5 in 24h
  - one failed check repeats several times
  - some opens are missing structure fields

- **Red**
  - blocked opens > 5 in 24h
  - repeated downgrade/reject pattern across multiple cycles
  - post-change behavior cannot be verified within the next live open attempts

These are governance thresholds, not automatic trading thresholds.

---

## Fast drill-down when the daily report shows a problem

### A. See the exact recent open candidates

```bash
python3 scripts/observe_open_candidates.py data/data.db 50
```

Use this when you need raw samples of:
- `raw_response`
- `decision_json`
- parsed `decisions`
- `entry_protection_rationale`
- `review_context.control`

### B. Verify protection-event attribution after related changes

```bash
python3 scripts/verify_postfix_protection_reasons.py data/data.db 24
```

Use this only when the strategy/policy change is related to close/protection attribution.

---

## Minimal daily note template

Copy this into the operator journal:

```md
### nofxmax daily governance check
- window: last __h
- blocked opens: __
- repeated failed checks: __
- structurally incomplete opens: __
- post-change verification status: verified / waiting for sample / suspicious
- follow-up record ids: __
- operator decision: no action / tune prompt / tune structure rules / tune RR policy / inspect persistence path
```

---

## Practical rules

- Prefer **counts and repeated patterns** over one-off stories.
- If there are **no open candidates**, mark the day as **insufficient sample**, not “all good”.
- If a strategy change was shipped, verification is incomplete until a **fresh annotated open attempt** appears in persisted data.
- If an open is blocked for a good reason, that is still a **successful control outcome**, not necessarily a strategy failure.
- If good-looking opportunities are repeatedly blocked for the same reason, the system is telling you the live contract and intended strategy are drifting apart.

---

## Why this loop is intentionally small

This mechanism is meant to be used every day without operator fatigue:
- one summary command
- one drill-down command when needed
- one short journal note

That is enough to catch the three governance failure modes that matter most right now:
- missed opportunities caused by over-blocking,
- low-quality opens caused by weak structure adherence,
- and silent strategy edits that never actually show up in persisted runtime behavior.
