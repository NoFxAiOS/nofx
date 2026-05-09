# Post-Fix Live Acceptance Checklist — Protection Attribution

## Purpose

Use this checklist after the tagged OKX market-close attribution fix to verify that
future protection-driven closes are now preserved with richer reasons instead of
flattening into generic `close_long` / `close_short`.

Relevant commits in this workline include:

- `e9935cf3` — store-side close reason recovery improvements
- `8474895a` — store tests for protection close reason recovery
- `171ed073` — heuristic reconstruction audit flow
- `73545d33` — OKX tagged protection-driven market closes

## Fast path

After a new real close event appears, run:

```bash
scripts/verify_postfix_protection_reasons.py data/data.db 6
scripts/reconstruct_protection_reasons.py data/data.db | sed -n '1,120p'
```

Adjust the lookback window as needed.

## Acceptance criteria

### PASS — direct success

At least one new close event in the post-fix window shows a rich reason/source such as:

- `break_even_stop`
- `managed_drawdown`
- `native_trailing`
- `ladder_tp`
- `ladder_sl`
- `full_tp`
- `full_sl`

This indicates the fix is observable in live data.

### PASS — indirect but plausible

No rich reason is shown yet, but all of the following are true:

- the lookback window includes only older pre-fix fills, or
- the new closes are not protection-driven market closes, or
- reconstruction output is consistent with expected protection behavior, and
- logs show expected break-even / protection reconcile behavior.

In this case, keep observing the next real protection-driven close.

### FAIL / needs follow-up

If new post-fix closes still appear only as generic `close_long` / `close_short` **and**
all of the following are true:

- the fills definitely happened after commit `73545d33` was deployed,
- they were protection-driven market closes,
- `order_action` and `client_order_id` are still generic,
- `verify_postfix_protection_reasons.py` shows zero rich-attributed events,

then continue investigation upstream.

## What to inspect if it still stays flat

### 1. Order metadata propagation

Check whether new `trader_orders` rows now contain non-generic metadata:

- `client_order_id`
- `order_action`
- `type`

If these remain generic, the reason is still being lost before or during exchange/order sync.

### 2. Runtime call path

Verify the protection path actually uses `closePositionByReason(...)` with a non-empty reason.

Important targets:

- `managed_drawdown`
- emergency / fallback protection closes
- any future protection-driven direct market close path

### 3. Exchange adapter behavior

For OKX specifically, verify that tagged close methods are being used:

- `CloseLongTagged`
- `CloseShortTagged`

and that the resulting order carries `okxReasonTag(reason)`.

### 4. Sync/storage recovery

If the order is tagged correctly on the exchange/order row but close events are still generic,
inspect:

- `trader/okx/order_sync.go`
- `store/position.go` `deriveCloseReason()`
- `PositionCloseEvent` persistence path

## Recommended audit sequence

1. `scripts/verify_postfix_protection_reasons.py data/data.db 6`
2. If still flat, run `scripts/reconstruct_protection_reasons.py data/data.db`
3. Inspect recent order rows for the relevant `exchange_order_id`
4. Check logs around the close timestamp for:
   - break-even trigger/apply/verify
   - drawdown trigger
   - protection reconciler verification
5. Decide:
   - direct live pass
   - indirect plausible pass / keep observing
   - real upstream regression

## Practical interpretation

Do not treat "no rich reasons yet" as an automatic failure.

This workline is specifically meant to improve **future** observability. A valid outcome is:

- protection continues behaving correctly,
- historical rows remain flat,
- and the first decisive confirmation only arrives when the next post-fix protection-driven close occurs.
