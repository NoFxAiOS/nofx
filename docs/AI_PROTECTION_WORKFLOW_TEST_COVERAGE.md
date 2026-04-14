
## Route-aware Protection Hard Constraints (2026-04-15)

### What is now enforced
- Full and Ladder are no longer treated as AI preference choices in the acceptance layer.
- They are treated as strategy-selected protection routes.
- Under `full_tp_sl.mode=ai` with ladder disabled, open actions must carry `protection_plan.mode=full`.
- Under `ladder_tp_sl.mode=ai` with full disabled, open actions must carry `protection_plan.mode=ladder`.
- Ladder route currently enforces 2~3 tiers.

### What is not fully enforced yet
- Drawdown / Break-even are currently treated as analysis-contract constraints, not output-shape constraints.
- They are acknowledged in prompt guidance, but not yet promoted to full route-aware validator rules.

### Real validation status
- Full route: real model output validated successfully (`open_long + protection_plan.mode=full + pct fields`, parse_error empty).
- Ladder route: engineering path validated, but real model under tested market contexts still prefers `wait`; no real ladder output observed yet.
